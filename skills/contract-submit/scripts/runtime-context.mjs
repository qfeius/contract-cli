import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {parseExact,scalar,sha256,TransportError} from './runtime-core.mjs';
const mappings=Object.freeze({
 test:{api:'https://test-open.qtech.cn',account:'https://test-myaccount.qtech.cn',editor:'https://test-contract.qtech.cn'},
 prod:{api:'https://open.qfei.cn',account:'https://myaccount.qfei.cn',editor:'https://contract.qfei.cn'},
});
const need=(value,code)=>{if(!value)throw new TransportError(code);};
function endpoint(value){try{need(typeof value==='string'&&value.trim()===value,'HTTPS_PROFILE_ENDPOINT_REQUIRED');const u=new URL(value);need(u.protocol==='https:'&&!u.username&&!u.password&&!u.search&&!u.hash,'HTTPS_PROFILE_ENDPOINT_REQUIRED');return u.origin+u.pathname.replace(/\/$/,'');}catch{throw new TransportError('HTTPS_PROFILE_ENDPOINT_REQUIRED');}}
/** Resolve a known endpoint tuple, not the CLI credential namespace label. Never switch profile or endpoints. */
export function resolveEndpointContext(endpoints,{expectedEnvironment,expectedOrigin}={}){
  need(endpoints&&typeof endpoints.environment==='string'&&endpoints.environment.length>0,'PROFILE_ENVIRONMENT_LABEL_REQUIRED');
  const tuple={apiBase:endpoint(endpoints.apiBase),resource:endpoint(endpoints.resource),accountMetadata:endpoint(endpoints.accountMetadata)};
  const matches=Object.entries(mappings).filter(([,m])=>tuple.apiBase===m.api&&tuple.resource===m.api&&tuple.accountMetadata===m.account+'/.well-known/oauth-authorization-server/contract');
  need(matches.length===1,'PROFILE_ENDPOINT_TUPLE_NOT_ADAPTED');
  const [environment,mapping]=matches[0];
  need(expectedEnvironment===undefined||environment===expectedEnvironment,'REQUESTED_ENVIRONMENT_ENDPOINT_MISMATCH');
  need(expectedOrigin===undefined||expectedOrigin===mapping.editor,'EDITOR_ORIGIN_MISMATCH');
  return Object.freeze({environment,profileEnvironment:endpoints.environment,apiOrigin:mapping.api,origin:mapping.editor,endpointFingerprint:sha256(tuple)});
}
/** Freeze both the resolved destination and the profile label across paginated / multi-step reads. */
export function assertSameEndpointContext(before,after){
  for(const key of ['environment','profileEnvironment','apiOrigin','origin','endpointFingerprint']){
    if(before?.[key]!==undefined||after?.[key]!==undefined)need(before?.[key]===after?.[key],'ENDPOINT_CONTEXT_CHANGED_RELOAD_REQUIRED');
  }
}
async function verifyProfile(client,readEndpoints){
  const context=client.context;need(Object.hasOwn(mappings,context.environment),'BUSINESS_ENVIRONMENT_REQUIRED');
  const result=await client.structured(['auth','status'],{effect:'read'});
  need(result?.ok===true&&result?.data,'AUTH_STATUS_PROTOCOL_UNSUPPORTED');const status=result.data;
  need(status.profile===context.profile&&status.identity===context.identity,'AUTH_CONTEXT_MISMATCH');
  const endpoints=await readEndpoints(context.profile);
  need(endpoints.profile===context.profile&&status.environment===endpoints.environment,'PROFILE_AUTH_LABEL_MISMATCH');
  const resolved=resolveEndpointContext(endpoints,{expectedEnvironment:context.environment,expectedOrigin:context.origin});
  if(context.profileEnvironment!==undefined)need(context.profileEnvironment===resolved.profileEnvironment,'PROFILE_AUTH_LABEL_MISMATCH');
  if(context.endpointFingerprint!==undefined)need(context.endpointFingerprint===resolved.endpointFingerprint,'ENDPOINT_CONTEXT_CHANGED_RELOAD_REQUIRED');
  return {status,resolved,async recheck(){
    const after=await readEndpoints(context.profile);need(after.profile===context.profile,'PROFILE_CONFIGURATION_CHANGED');
    assertSameEndpointContext(resolved,resolveEndpointContext(after,{expectedEnvironment:context.environment,expectedOrigin:context.origin}));
  }};
}
/** Reads only whitelisted endpoint metadata; never returns identities, token, secret or raw configuration. */
export async function readProfileEndpoints(profile,{configDirectory}={}){
  need(typeof profile==='string'&&profile.trim()===profile&&profile.length>0,'EXPLICIT_PROFILE_REQUIRED');
  let directory=configDirectory||process.env.CONTRACT_CLI_CONFIG_DIR||process.env.DEMOCLI_CONFIG_DIR;
  if(!directory){
    const current=path.join(os.homedir(),'.contract-cli'),legacy=path.join(os.homedir(),'.democli');
    const exists=async dir=>{try{await fs.access(path.join(dir,'config.json'));return true;}catch{return false;}};
    const [hasCurrent,hasLegacy]=await Promise.all([exists(current),exists(legacy)]);
    need(!(hasCurrent&&hasLegacy),'AMBIGUOUS_CONFIG_DIRECTORY_USE_EXPLICIT_SCOPE');directory=hasCurrent?current:legacy;
  }
  const file=path.join(path.resolve(directory),'config.json');const stat=await fs.lstat(file);
  need(stat.isFile()&&!stat.isSymbolicLink()&&stat.size<=2*1024*1024,'REGULAR_PROFILE_CONFIG_REQUIRED');
  let config= parseExact(await fs.readFile(file,'utf8'));const p=config?.profiles?.[profile];
  need(p&&p.name===profile,'PROFILE_CONFIGURATION_NOT_FOUND');
  const result={profile,environment:p.environment,apiBase:p.open_platform_base_url,resource:p.resource,accountMetadata:p.authorization_server_metadata_url};
  config=null;
  need(Object.values(result).every(v=>typeof v==='string'&&v.length>0),'PROFILE_ENDPOINT_METADATA_REQUIRED');return Object.freeze(result);
}
/** The intended business environment is checked against endpoints; auth labels are checked against profile configuration. */
export async function verifyUserContext(client,{readEndpoints=readProfileEndpoints}={}){
  const context=client.context;need(context.identity==='user','USER_CONTEXT_REQUIRED');
  const profile=await verifyProfile(client,readEndpoints);need(profile.status.authorization==='authorized','CURRENT_USER_AUTHORIZATION_REQUIRED');
  const response=await client.web({method:'GET',path:'/open-web/web/clm/employee/info',effect:'read'});
  need(response?.success===true&&String(scalar(response.code))==='200'&&response.data,'CURRENT_EMPLOYEE_RESPONSE_REQUIRED');
  const userId=String(scalar(response.data.employeeId)),tenantId=String(scalar(response.data.tenantId));
  need(/^[1-9]\d*$/.test(userId)&&/^[1-9]\d*$/.test(tenantId),'CURRENT_EMPLOYEE_IDENTITY_REQUIRED');
  need((context.userId===undefined||context.userId===userId)&&(context.tenantId===undefined||context.tenantId===tenantId),'CURRENT_EMPLOYEE_IDENTITY_CHANGED');
  await profile.recheck();
  return Object.freeze({profile:context.profile,...profile.resolved,identity:'user',tenantId,userId,observedAt:new Date().toISOString()});
}
/** APP configured means credentials are configured, not that a business permission is granted. */
export async function verifyAppContext(client,{readEndpoints=readProfileEndpoints}={}) {
  const context=client.context;need(context.identity==='app','APP_CONTEXT_REQUIRED');
  const profile=await verifyProfile(client,readEndpoints),s=profile.status;
  need(['authorized','configured'].includes(s.authorization),'CURRENT_APP_CONFIGURATION_REQUIRED');
  const appIds=(s.fields||[]).filter(f=>f.label==='App ID').map(f=>f.value);
  need(appIds.length===1&&typeof appIds[0]==='string'&&appIds[0].trim()===appIds[0]&&appIds[0].length>0&&!appIds[0].startsWith('<'),'CURRENT_APP_ID_REQUIRED');
  const appId=appIds[0];need(context.appId===undefined||context.appId===appId,'CURRENT_APP_IDENTITY_CHANGED');
  await profile.recheck();
  return Object.freeze({profile:context.profile,...profile.resolved,identity:'app',appId,authorization:s.authorization,requiresAuthorizedBusinessRead:true,observedAt:new Date().toISOString()});
}
