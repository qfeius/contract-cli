import fs from 'node:fs';
import fsp from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import {spawn} from 'node:child_process';
import {parseExact} from './exact-json.mjs';
export {parseExact};

export class TransportError extends Error {
  constructor(code, outcome='not_sent') {
    super(code); this.name='TransportError'; this.code=code; this.outcome=outcome;
  }
}
export const scalar = value => JSON.isRawJSON(value) ? value.rawJSON : value;
export function stringifyExact(value) {
  const seen=new WeakSet();
  const check = item => {
    if (JSON.isRawJSON(item)) return;
    if (typeof item==='number' && !Number.isSafeInteger(item)) throw new TransportError('EXACT_NUMBER_REQUIRED');
    if (typeof item==='bigint') throw new TransportError('EXPLICIT_RAW_JSON_OR_ID_STRING_REQUIRED');
    if (item && typeof item==='object') {
      if (seen.has(item)) throw new TransportError('CYCLIC_JSON');
      seen.add(item);
      for (const [key,v] of Object.entries(item)) {
        if (['__proto__','prototype','constructor'].includes(key)) throw new TransportError('UNSAFE_JSON_KEY');
        check(v);
      }
      seen.delete(item);
    }
    if (typeof item==='undefined' || typeof item==='function' || typeof item==='symbol') throw new TransportError('NON_JSON_INPUT');
  };
  check(value); return JSON.stringify(value);
}
export const sha256 = value => crypto.createHash('sha256').update(Buffer.isBuffer(value) || typeof value==='string' ? value : stringifyExact(value)).digest('hex');
export async function privateWrite(filename, value, {text=false}={}) {
  const parent=path.dirname(path.resolve(filename));
  await fsp.mkdir(parent,{recursive:true,mode:0o700});
  const resolved=await fsp.realpath(parent);
  if (resolved!==parent) throw new TransportError('SYMLINK_OUTPUT_DIRECTORY');
  const handle=await fsp.open(filename, fs.constants.O_WRONLY|fs.constants.O_CREAT|fs.constants.O_EXCL|fs.constants.O_NOFOLLOW,0o600);
  try {await handle.writeFile(text ? value : stringifyExact(value)); await handle.sync();}
  finally {await handle.close();}
  return path.resolve(filename);
}
export async function readPrivateJson(filename) {
  const stat=await fsp.lstat(filename);
  if (!stat.isFile() || stat.isSymbolicLink() || (stat.mode & 0o077)) throw new TransportError('PRIVATE_REGULAR_FILE_REQUIRED');
  const handle=await fsp.open(filename,fs.constants.O_RDONLY|fs.constants.O_NOFOLLOW);
  try {return parseExact(await handle.readFile('utf8'));} finally {await handle.close();}
}
const cleanText = value => typeof value==='string' && value.length>0 && value.trim()===value && !/[\x00-\x1f\x7f]/u.test(value);
const readPaths=new Set([
  'GET /open-web/web/clm/employee/info',
  'GET /open-web/web/clm/sso/getUser',
  'GET /open-web/web/clm/contract/briefInfo',
  'GET /open-web/web/clm/cooperation/checkCooperation',
  'GET /open-web/web/clm/task/list/v2',
  'GET /open-web/web/clm/task/info',
  'POST /open-web/web/clm/es/search/contractList',
  'POST /open-web/web/clm/cooperation/searchV2',
]);
export const STRICT_COOPERATION_PROFILE='clm-web-status-v2.0';
const strictCooperationPath=/^\/open-web\/web\/clm-openapi\/v1\/mcp\/cooperations\/[1-9][0-9]*$/u;
function businessHeaders(headers,strictCooperation=false) {
  if(headers===undefined) return [];
  if(!headers || typeof headers!=='object' || Array.isArray(headers)) throw new TransportError('BUSINESS_HEADERS_OBJECT_REQUIRED');
  const args=[];
  for(const [key,value] of Object.entries(headers)){
    if(key==='X-MCP-Response-Profile'&&strictCooperation){
      if(value!==STRICT_COOPERATION_PROFILE)throw new TransportError('STRICT_COOPERATION_PROFILE_REQUIRED');
      args.push('--header',key+': '+value);continue;
    }
    if(key!=='Timezone-Offset') throw new TransportError('BUSINESS_HEADER_NOT_ALLOWED');
    const text=String(scalar(value));
    if(!/^-?\d{1,3}$/u.test(text) || Number(text)<-840 || Number(text)>840) throw new TransportError('INVALID_TIMEZONE_OFFSET');
    args.push('--header',key+': '+text);
  }
  return args;
}
function queryObject(query){
  if(query===undefined)return {};
  if(!query||typeof query!=='object')throw new TransportError('QUERY_PAIRS_REQUIRED');
  const result=Object.create(null),pairs=Array.isArray(query)?query:Object.entries(query);
  for(const pair of pairs){
    if(!Array.isArray(pair)||pair.length!==2||typeof pair[0]!=='string'||Object.hasOwn(result,pair[0]))throw new TransportError('DUPLICATE_OR_INVALID_QUERY_KEY');
    result[pair[0]]=scalar(pair[1]);
  }
  return result;
}
const positiveId=value=>(typeof value==='string'||(typeof value==='number'&&Number.isSafeInteger(value)))&&/^[1-9][0-9]*$/u.test(String(value));
const opaqueId=value=>typeof value==='string'&&/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$/u.test(value);
function validateReadRequest(publicPath,query,headers,strictCooperation,body){
  const q=queryObject(query),keys=Object.keys(q);
  const exact=expected=>{if(keys.length!==expected.length||keys.some(k=>!expected.includes(k)))throw new TransportError('READ_QUERY_SCOPE_NOT_ADAPTED');};
  if(strictCooperation){
    exact([]);
    if(!headers||Object.keys(headers).length!==1||headers['X-MCP-Response-Profile']!==STRICT_COOPERATION_PROFILE)throw new TransportError('STRICT_COOPERATION_PROFILE_REQUIRED');
  }else if(publicPath.endsWith('/sso/getUser')||publicPath.endsWith('/employee/info')){
    exact([]);
  }else if(publicPath.endsWith('/contract/briefInfo')){
    exact(['contractId']);if(!positiveId(q.contractId))throw new TransportError('INVALID_OBJECT_ID');
  }else if(publicPath.endsWith('/task/info')){
    exact(['taskInstanceId']);if(!opaqueId(q.taskInstanceId))throw new TransportError('INVALID_TASK_ID');
  }else if(publicPath.endsWith('/task/list/v2')){
    exact(['taskTypeCode','pageIndex','pageSize','desc','limitBefore','limitAfter']);
    if(q.taskTypeCode!==0||q.desc!==true||q.limitBefore!==0||!Number.isSafeInteger(q.pageIndex)||q.pageIndex<0||!Number.isSafeInteger(q.pageSize)||q.pageSize<1||q.pageSize>100||q.limitAfter!==q.pageSize-1)throw new TransportError('TODO_PAGINATION_SCOPE_REQUIRED');
  }else if(publicPath.endsWith('/cooperation/checkCooperation')){
    exact(['cooperationId','cooperationFileId']);if(!positiveId(q.cooperationId)||!positiveId(q.cooperationFileId))throw new TransportError('INVALID_COOPERATION_ID');
  }else if(publicPath.endsWith('/es/search/contractList')||publicPath.endsWith('/cooperation/searchV2')){
    const b=typeof body==='string'?parseExact(body):body;
    if(!b||typeof b!=='object'||Array.isArray(b)||JSON.isRawJSON(b))throw new TransportError('READ_BODY_OBJECT_REQUIRED');
    const bodyKeys=Object.keys(b),bodyExact=expected=>{if(bodyKeys.length!==expected.length||bodyKeys.some(k=>!expected.includes(k)))throw new TransportError('READ_BODY_SCOPE_NOT_ADAPTED');};
    if(publicPath.endsWith('/es/search/contractList')){
      exact([]);bodyExact(['pageIndex','pageSize','searchTabCode','conditionUnits','filterUnits']);
      if(![2,3].includes(b.searchTabCode)||!Number.isSafeInteger(b.pageIndex)||b.pageIndex<0||!Number.isSafeInteger(b.pageSize)||b.pageSize<1||b.pageSize>30||!Array.isArray(b.conditionUnits)||b.conditionUnits.length||!Array.isArray(b.filterUnits)||b.filterUnits.length)throw new TransportError('OWN_DRAFT_SCOPE_REQUIRED');
    }else{
      exact(['pageIndex','pageSize']);bodyExact(['keyword','searchTabEnumCode','cooperationDisplayTabCode']);
      if(!Number.isSafeInteger(q.pageIndex)||q.pageIndex<0||!Number.isSafeInteger(q.pageSize)||q.pageSize<1||q.pageSize>100||q.pageIndex*q.pageSize>=10000||b.searchTabEnumCode!==1||b.cooperationDisplayTabCode!==0||typeof b.keyword!=='string'||b.keyword!==b.keyword.trim()||b.keyword.length>500||/[\x00-\x1f\x7f]/u.test(b.keyword))throw new TransportError('MY_COOPERATION_SCOPE_REQUIRED');
      if(!headers||!Object.hasOwn(headers,'Timezone-Offset'))throw new TransportError('TIMEZONE_OFFSET_REQUIRED');
    }
  }
  return q;
}
function validateContext(context) {
  if (!context || !cleanText(context.profile) || context.profile.startsWith('-')) throw new TransportError('EXPLICIT_PROFILE_REQUIRED');
  if (!['user','app'].includes(context.identity)) throw new TransportError('EXPLICIT_IDENTITY_REQUIRED');
  if (!cleanText(context.environment)) throw new TransportError('EXPLICIT_ENVIRONMENT_REQUIRED');
  const cli=context.cli || 'contract-cli';
  if (!cleanText(cli) || cli.startsWith('-') || (!path.isAbsolute(cli) && cli!=='contract-cli')) throw new TransportError('CLI_EXECUTABLE_INVALID');
  return cli;
}
function queries(query) {
  if (query===undefined) return [];
  const pairs=Object.entries(queryObject(query));
  const result=[];
  for (const pair of pairs) {
    if (!Array.isArray(pair) || pair.length!==2) throw new TransportError('QUERY_PAIRS_REQUIRED');
    const [key,raw]=pair; const value=scalar(raw);
    if(typeof value==='number'&&!Number.isSafeInteger(value))throw new TransportError('EXACT_NUMBER_REQUIRED');
    if (!/^[A-Za-z][A-Za-z0-9_.-]*$/u.test(key) || !['string','number','boolean'].includes(typeof value) || !cleanText(String(value))) throw new TransportError('QUERY_SCALAR_REQUIRED');
    if (/authorization|cookie|token|secret/iu.test(key)) throw new TransportError('QUERY_AUTH_OVERRIDE_FORBIDDEN');
    result.push('--query',key+'='+String(value));
  }
  return result;
}
async function invoke(executable,args,{timeoutMs=60000,maxBytes=32*1024*1024,effect='read'}={}) {
  return await new Promise((resolve,reject)=>{
    let child;
    try {child=spawn(executable,args,{shell:false,stdio:['ignore','pipe','pipe'],env:{...process.env}});}
    catch {reject(new TransportError('CLI_NOT_STARTED'));return;}
    let stdout=[],size=0,settled=false;
    const fail=(code,outcome)=>{if(settled)return;settled=true;clearTimeout(timer);reject(new TransportError(code,outcome));};
    const unknown=effect==='write'?'unknown':'not_sent';
    const timer=setTimeout(()=>{child.kill('SIGTERM');fail('CLI_TIMEOUT',unknown);},timeoutMs);
    child.stdout.on('data',chunk=>{
      size+=chunk.length;
      if(size>maxBytes){child.kill('SIGTERM');fail('CLI_RESPONSE_TOO_LARGE',unknown);}
      else stdout.push(chunk);
    });
    // Stderr may contain private server output. Drain without retaining or printing it.
    child.stderr.resume();
    child.on('error',()=>fail('CLI_NOT_STARTED','not_sent'));
    child.on('close',code=>{
      if(settled)return;settled=true;clearTimeout(timer);
      if(code!==0){reject(new TransportError('CLI_FAILED',unknown));return;}
      try {resolve(parseExact(Buffer.concat(stdout).toString('utf8')));}
      catch {reject(new TransportError('CLI_RESPONSE_INVALID_JSON',unknown));}
    });
  });
}
export function createClient(context,{invokeImpl=invoke}={}) {
  const cli=validateContext(context);
  context=Object.freeze({...context,cli});
  return Object.freeze({
    context,
    async web({method,path:publicPath,query,body,headers,effect,timeoutMs=60000}) {
      if(context.identity!=='user') throw new TransportError('WEB_IS_USER_ONLY');
      if(effect!=='read') throw new TransportError('LINK_ONLY_READ_REQUIRED');
      if(typeof publicPath!=='string' || !publicPath.startsWith('/open-web/web/') || /[%?#;\\\s]/u.test(publicPath) || publicPath.includes('//') || publicPath.split('/').some(x=>['.','..','internal','test','actuator','armorsClientTest'].includes(x))) throw new TransportError('PUBLIC_RELATIVE_PATH_REQUIRED');
      const strictCooperation=method==='GET'&&strictCooperationPath.test(publicPath);
      if(!strictCooperation&&!readPaths.has(method+' '+publicPath)) throw new TransportError('LINK_READ_PATH_NOT_ADAPTED');
      if(method==='GET' && body!==undefined) throw new TransportError('READ_QUERY_ONLY');
      validateReadRequest(publicPath,query,headers,strictCooperation,body);
      if(!Number.isInteger(timeoutMs) || timeoutMs<1 || timeoutMs>300000) throw new TransportError('INVALID_TIMEOUT');
      const args=['api','call','--channel','web','--profile',context.profile,'--as','user','--raw',method,publicPath,...queries(query),...businessHeaders(headers,strictCooperation)];
      let directory;
      try {
        if(body!==undefined){
          const encoded=typeof body==='string' ? body : stringifyExact(body);
          parseExact(encoded);
          if(Buffer.byteLength(encoded)>1024*1024) throw new TransportError('REQUEST_BODY_EXCEEDS_1_MIB');
          directory=await fsp.mkdtemp(path.join(await fsp.realpath(os.tmpdir()),'contract-skill-'));
          await fsp.chmod(directory,0o700);
          const file=path.join(directory,'request.json');await privateWrite(file,encoded,{text:true});
          args.push('--input-file',file);
        }
        return await invokeImpl(cli,args,{timeoutMs,effect});
      } finally {if(directory) await fsp.rm(directory,{recursive:true,force:true});}
    },
    // Only the existing read commands used to locate and refresh native links.
    async structured(argv,{effect='read',timeoutMs=60000}={}) {
      if(!Array.isArray(argv) || !argv.length || !argv.every(cleanText) || argv.some(x=>/^(--profile|--as|--debug)(=|$)/u.test(x))) throw new TransportError('STRUCTURED_ARGUMENTS_INVALID');
      if(effect!=='read') throw new TransportError('LINK_ONLY_READ_REQUIRED');
      const command=argv.slice(0,2).join(' ');
      if(command!=='auth status'||argv.length!==2) throw new TransportError('LINK_READ_COMMAND_NOT_ADAPTED');
      return await invokeImpl(cli,[...argv,'--profile',context.profile,'--as',context.identity,'--output','json'],{timeoutMs,effect});
    }
  });
}
