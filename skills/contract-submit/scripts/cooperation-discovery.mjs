import {pathToFileURL} from 'node:url';
import {createClient,parseExact,stringifyExact,scalar,sha256,readPrivateJson} from './runtime-core.mjs';
import {verifyUserContext,assertSameEndpointContext} from './runtime-context.mjs';

export const COOPERATION_ROUTE='/open-web/web/clm/cooperation/searchV2';
const fields=['cooperationRoundId','cooperationTypeCode','cooperationName','contractNumber','businessId','businessTypeCode','cooperationStatusCode','cooperationStageCode','cooperationRoundSourceCode','isCooperationUser','cooperationUserTypeCode','cooperationUserStatusCode','createEmployeeVO','todoTaskId','createTime','updateTime','confirmProcessStatusCode','cooperationFinalizeStatusCode'];
const filterMap={id:'cooperationId',number:'contractNumber',name:'cooperationName',businessId:'businessId',businessTypeCode:'businessTypeCode',cooperationStatusCode:'cooperationStatusCode'};
const canonical=x=>x==null?null:String(scalar(x));
const id=x=>{const s=canonical(x);if(!s||! /^[1-9]\d*$/.test(s))throw Error('COOPERATION_ID_REQUIRED');return s;};
const integer=x=>{const s=canonical(x),n=Number(s);if(s===null||! /^\d+$/.test(s)||!Number.isSafeInteger(n))throw Error('COOPERATION_COUNT_INVALID');return n;};
const errorCode=e=>/^[A-Z0-9_]+$/.test(e.code||e.message)?e.code||e.message:'COOPERATION_READ_FAILED';
export function validateCooperationFilters(filters={}){
  if(!filters||Array.isArray(filters)||typeof filters!=='object')throw Error('COOPERATION_FILTER_OBJECT_REQUIRED');
  for(const [key,value] of Object.entries(filters)){
    if(!Object.hasOwn(filterMap,key))throw Error('UNSUPPORTED_COOPERATION_FILTER');
    if(typeof value!=='string'||!value.trim()||value!==value.trim())throw Error('COOPERATION_FILTER_VALUE_REQUIRED');
    if(['id','businessId'].includes(key))id(value);
    if(['businessTypeCode','cooperationStatusCode'].includes(key))integer(value);
  }
  return filters;
}
function match(item,filters){
  const missing=[];
  for(const [key,wanted] of Object.entries(filters)){
    const actual=canonical(item[filterMap[key]]);
    if(actual===null){missing.push(key);continue;}
    if(!(key==='name'?actual.includes(wanted):actual===wanted))return {match:false,missing:[]};
  }
  return {match:missing.length?null:true,missing};
}
function capability(raw,pageSize){
  if(raw==null)return {state:'legacy',reachablePageCount:Math.floor(10000/pageSize)};
  if(raw.strategy!=='OFFSET_WINDOW'||raw.unit!=='COOPERATION')throw Error('COOPERATION_PAGINATION_CAPABILITY_INVALID');
  for(const key of ['offsetWindow','pageSize','reachablePageCount'])if(typeof raw[key]!=='number'||!Number.isSafeInteger(raw[key])||raw[key]<1)throw Error('COOPERATION_PAGINATION_CAPABILITY_INVALID');
  if(raw.pageSize!==pageSize||raw.reachablePageCount!==Math.floor(raw.offsetWindow/pageSize))throw Error('COOPERATION_PAGINATION_CAPABILITY_INVALID');
  return {state:'valid',offsetWindow:raw.offsetWindow,pageSize,reachablePageCount:Math.min(raw.reachablePageCount,Math.floor(10000/pageSize))};
}
/** MY scope includes historical participation; a returned item never alone proves current-round authority. */
export async function discoverCooperations(client,{keyword='',filters={},pageSize=20,maxPages=100,displayOffset=0,displayLimit=20,timezoneOffset=new Date().getTimezoneOffset()}={},deps={}){
  validateCooperationFilters(filters);
  if(client.context?.identity!=='user')throw Error('COOPERATION_DISCOVERY_REQUIRES_USER');
  if(typeof keyword!=='string'||keyword!==keyword.trim()||keyword.length>500||/[\x00-\x1f\x7f]/u.test(keyword))throw Error('COOPERATION_KEYWORD_INVALID');
  if(!Number.isInteger(pageSize)||pageSize<1||pageSize>100||!Number.isInteger(maxPages)||maxPages<1||maxPages>10000)throw Error('COOPERATION_SCAN_LIMIT_INVALID');
  if(!Number.isInteger(displayOffset)||displayOffset<0||!Number.isInteger(displayLimit)||displayLimit<1||displayLimit>100)throw Error('COOPERATION_DISPLAY_LIMIT_INVALID');
  if(!Number.isInteger(timezoneOffset)||timezoneOffset< -840||timezoneOffset>840)throw Error('INVALID_TIMEZONE_OFFSET');
  const verify=deps.verifyContext??verifyUserContext,current=await verify(client);
  if(!current||![current.tenantId,current.userId].every(v=>typeof v==='string'&&/^[1-9]\d*$/.test(v)))throw Error('COOPERATION_CURRENT_USER_NOT_VERIFIED');
  const items=[],issues=[],seen=new Set(),hashes=new Set();
  let total=null,pages=0,complete=false,cap=null;
  for(let pageIndex=0;pageIndex<maxPages;pageIndex++){
    try{
      if(pageIndex>=(cap?.reachablePageCount??Math.floor(10000/pageSize)))throw Error('COOPERATION_OFFSET_WINDOW_REACHED');
      const response=await client.web({method:'POST',path:COOPERATION_ROUTE,query:{pageIndex,pageSize},headers:{'Timezone-Offset':timezoneOffset},body:{keyword,searchTabEnumCode:1,cooperationDisplayTabCode:0},effect:'read'});
      if(response?.success!==true||canonical(response.code)!=='200')throw Error('COOPERATION_LIST_BUSINESS_FAILURE');
      const data=response.data;
      if(!data||!Array.isArray(data.content))throw Error('COOPERATION_LIST_SHAPE');
      const count=integer(data.totalElements);
      if(total!==null&&count!==total)throw Error('COOPERATION_TOTAL_CHANGED');
      total=count;pages++;
      if(data.pageIndex!=null&&integer(data.pageIndex)!==pageIndex)throw Error('COOPERATION_PAGE_INDEX_CHANGED');
      if(data.pageSize!=null&&integer(data.pageSize)!==pageSize)throw Error('COOPERATION_PAGE_SIZE_CHANGED');
      if(data.content.length>pageSize)throw Error('COOPERATION_PAGE_EXCEEDS_LIMIT');
      const nextCap=capability(data.paginationCapability,pageSize);
      if(cap&&stringifyExact(cap)!==stringifyExact(nextCap))throw Error('COOPERATION_PAGINATION_CAPABILITY_CHANGED');
      cap=nextCap;
      const page=data.content.map(raw=>{
        const row={cooperationId:id(raw?.cooperationId),provenance:'MY_COOPERATION_LIST'};
        for(const field of fields)if(Object.hasOwn(raw,field))row[field]=raw[field]===null?null:scalar(raw[field]);
        return row;
      });
      const digest=sha256(page.map(r=>r.cooperationId));
      if(page.length&&hashes.has(digest))throw Error('COOPERATION_PAGE_REPEATED');
      hashes.add(digest);
      for(const row of page){if(seen.has(row.cooperationId))throw Error('COOPERATION_DUPLICATE_OR_SHIFT');seen.add(row.cooperationId);items.push(row);}
      if(seen.size>total)throw Error('COOPERATION_COUNT_EXCEEDS_TOTAL');
      if((pageIndex+1)*pageSize>=total){
        complete=seen.size===total;
        if(!complete)issues.push({code:'COOPERATION_CONTENT_SHORT_OF_TOTAL'});
        break;
      }
    }catch(e){issues.push({code:errorCode(e)});break;}
  }
  if(!complete&&!issues.length)issues.push({code:pages>=(cap?.reachablePageCount??Infinity)?'COOPERATION_OFFSET_WINDOW_REACHED':'COOPERATION_SCAN_LIMIT_REACHED'});
  const after=await verify(client);
  assertSameEndpointContext(current,after);
  if(after?.tenantId!==current.tenantId||after?.userId!==current.userId)throw Error('COOPERATION_CURRENT_USER_CHANGED_RELOAD_REQUIRED');
  const matches=[],unresolved=[];
  for(const item of items){const m=match(item,filters);if(m.match===true)matches.push(item);else if(m.match===null)unresolved.push({...item,missingFilterMetadata:m.missing});}
  complete=complete&&!unresolved.length;
  return {scope:'CURRENT_USER_MY_COOPERATIONS',currentUser:{tenantId:current.tenantId,userId:current.userId},observedAt:new Date().toISOString(),keyword,filters,complete,scans:{pages,reportedTotal:total,observedCount:seen.size,paginationCapability:cap},issues,unresolved,observedMatches:matches.length,matchedTotal:complete?matches.length:null,resolution:!complete?'INCOMPLETE':matches.length===0?'NO_MY_COOPERATION_MATCH':matches.length===1?'UNIQUE_CANDIDATE':'MULTIPLE_CANDIDATES',items:matches.slice(displayOffset,displayOffset+displayLimit),displayOffset,displayLimit,hasMoreObserved:displayOffset+displayLimit<matches.length,requiresFreshObjectAndPermissionRead:true,approvalSubmitted:false};
}
if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href){
  const [operation,contextFile,optionsJSON='{}']=process.argv.slice(2);
  if(operation!=='list'||!contextFile){process.stderr.write('Usage: node cooperation-discovery.mjs list PRIVATE_CONTEXT.json OPTIONS_JSON\n');process.exitCode=2;}
  else try{
    const context=await readPrivateJson(contextFile),options=parseExact(optionsJSON);
    const allowed=['keyword','filters','pageSize','maxPages','displayOffset','displayLimit','timezoneOffset'];
    if(!options||Array.isArray(options)||typeof options!=='object'||Object.keys(options).some(k=>!allowed.includes(k)))throw Error('COOPERATION_OPTIONS_INVALID');
    process.stdout.write(stringifyExact(await discoverCooperations(createClient(context),options))+'\n');
  }catch(e){process.stderr.write(errorCode(e)+'\n');process.exitCode=1;}
}
