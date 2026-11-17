import {scalar,sha256,STRICT_COOPERATION_PROFILE} from './runtime-core.mjs';
import {verifyUserContext,assertSameEndpointContext} from './runtime-context.mjs';

const WEB='/open-web/web/clm';
export const QUERY_PATHS=Object.freeze({
  cooperationDetail:'/open-web/web/clm-openapi/v1/mcp/cooperations/',
  cooperationCheck:WEB+'/cooperation/checkCooperation',
  taskList:WEB+'/task/list/v2',taskInfo:WEB+'/task/info',draftRouteConfig:WEB+'/sso/getUser',
});
const object=v=>v!==null&&typeof v==='object'&&!Array.isArray(v)&&!JSON.isRawJSON(v);
const fail=code=>{const error=new Error(code);error.code=code;throw error;};
const need=(v,code)=>{if(!v)fail(code);};
const string=v=>{const x=scalar(v);need(typeof x==='string'||(typeof x==='number'&&Number.isSafeInteger(x)),'EXACT_SCALAR_REQUIRED');return String(x);};
const id=v=>{const s=string(v);need(/^[1-9][0-9]*$/u.test(s),'INVALID_OBJECT_ID');return s;};
const taskId=v=>{need(typeof v==='string'&&/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$/u.test(v),'INVALID_TASK_ID');return v;};
const count=v=>{const s=string(v);need(/^(0|[1-9][0-9]*)$/u.test(s)&&Number.isSafeInteger(Number(s)),'INVALID_INTEGER');return Number(s);};
const optional=(v,fn)=>v===undefined||v===null?null:fn(v);
const label=v=>typeof v==='string'?v:null;
function options(value,allowed){need(object(value),'QUERY_OPTIONS_REQUIRED');for(const key of Object.keys(value))need(allowed.includes(key),'QUERY_OPTION_NOT_ADAPTED');}
function issue(error){return {code:typeof error?.code==='string'&&/^[A-Z0-9_]+$/u.test(error.code)?error.code:'QUERY_FAILED',...(error?.businessCode!==undefined?{businessCode:error.businessCode}:{})};}
function response(raw,protocol){
  need(object(raw),'RESPONSE_OBJECT_REQUIRED');
  const code=optional(raw.code,string);
  if(code!==(protocol==='mcp'?'0':'200')||(protocol==='mcp'?raw.success===false:raw.success!==true)){
    const error=new Error('BUSINESS_FAILURE');error.code=protocol==='mcp'?'MCP_BUSINESS_FAILURE':'WEB_BUSINESS_FAILURE';
    if(code!==null&&/^-?[0-9]+$/u.test(code))error.businessCode=code;
    throw error;
  }
  return raw.data;
}
const noApproval=Object.freeze({approvalSubmitted:false,approvalPermission:'UNCONFIRMED'});
async function begin(client,deps){
  need(client?.context?.identity==='user','CURRENT_USER_REQUIRED');
  const verify=deps.verifyContext??verifyUserContext;
  const identity=await verify(client);
  need(object(identity),'CURRENT_USER_NOT_VERIFIED');
  const tenantId=id(identity.tenantId),userId=id(identity.userId);
  if(identity.profile!==undefined)need(identity.profile===client.context.profile,'CURRENT_USER_CONTEXT_MISMATCH');
  if(identity.environment!==undefined)need(identity.environment===client.context.environment,'CURRENT_USER_CONTEXT_MISMATCH');
  const scope={profile:client.context.profile,environment:client.context.environment,tenantId,userId};
  return {scope,async end(){
    const end=await verify(client);
    assertSameEndpointContext(identity,end);
    need(object(end)&&id(end.tenantId)===tenantId&&id(end.userId)===userId,'CURRENT_USER_CHANGED_RELOAD_REQUIRED');
    need(client.context.identity==='user'&&client.context.profile===scope.profile&&client.context.environment===scope.environment,'CURRENT_USER_CHANGED_RELOAD_REQUIRED');
    if(end.profile!==undefined)need(end.profile===scope.profile,'CURRENT_USER_CHANGED_RELOAD_REQUIRED');
    if(end.environment!==undefined)need(end.environment===scope.environment,'CURRENT_USER_CHANGED_RELOAD_REQUIRED');
  }};
}
function target(input){return {businessId:id(input.businessId),businessTypeCode:count(input.businessTypeCode)};}
function sameBusiness(actual,wanted){return actual.businessId===wanted.businessId&&actual.businessTypeCode===wanted.businessTypeCode;}
function cooperationExpected(input){
  return {cooperationId:id(input.cooperationId),businessId:optional(input.businessId,id),businessTypeCode:optional(input.businessTypeCode,count),cooperationRoundId:optional(input.cooperationRoundId,id)};
}
function cooperation(data,expected){
  need(object(data),'COOPERATION_DETAIL_NOT_RETURNED');
  const value={cooperationId:id(data.cooperation_id),cooperationRoundId:id(data.cooperation_round_id),
    businessId:optional(data.business_id,id),businessTypeCode:optional(data.business_type_code,count),
    cooperationStatusCode:count(data.cooperation_status_code),cooperationStageCode:optional(data.cooperation_stage_code,count),
    cooperationName:label(data.cooperation_name),contractNumber:label(data.contract_number),
    currentRoundCreatorUserId:optional(data.create_user?.user_id,id)};
  for(const key of ['cooperationId','cooperationRoundId','businessId','businessTypeCode'])if(expected[key]!==null&&expected[key]!==undefined)need(value[key]===expected[key],'COOPERATION_OBJECT_OR_ROUND_CHANGED');
  need(Array.isArray(data.current_round_files)&&Array.isArray(data.cooperation_users),'COOPERATION_DETAIL_COLLECTIONS_MISSING');
  value.currentRoundFiles=data.current_round_files.map(file=>{
    need(object(file),'COOPERATION_FILE_SHAPE');
    return {cooperationFileId:id(file.cooperation_file_id),fileId:optional(file.file_id,id),latest:typeof file.latest==='boolean'?file.latest:null,
      cooperationFileTypeCode:optional(file.cooperation_file_type_code,count),createTime:optional(file.create_time,string)};
  });
  need(new Set(value.currentRoundFiles.map(file=>file.cooperationFileId)).size===value.currentRoundFiles.length,'COOPERATION_FILE_DUPLICATE');
  value.cooperationUsers=data.cooperation_users.map(user=>{
    need(object(user),'COOPERATION_USER_SHAPE');
    return {cooperationUserId:optional(user.cooperation_user_id,id),userId:optional(user.employee?.user_id,id),
      cooperationUserTypeCode:optional(user.cooperation_user_type_code,count),cooperationUserStatusCode:optional(user.cooperation_user_status_code,count)};
  });
  // Employee IDs in this projection lack tenant IDs: never infer the caller's membership here.
  value.currentUserMembership='UNCONFIRMED';value.enableFormCooperation=null;
  value.missingRouteFacts=['OBJECT_TWO_IN_ONE_FLAG'];
  return value;
}
async function fetchCooperation(client,expected){
  return cooperation(response(await client.web({method:'GET',path:QUERY_PATHS.cooperationDetail+expected.cooperationId,
    headers:{'X-MCP-Response-Profile':STRICT_COOPERATION_PROFILE},effect:'read'}),'mcp'),expected);
}
/** Explicit strict profile; no legacy fallback, full detail, raw extra or action permission inference. */
export async function readCooperationDetail(client,input={},deps={}){
  options(input,['cooperationId','businessId','businessTypeCode','cooperationRoundId']);
  const expected=cooperationExpected(input),session=await begin(client,deps);
  let result;
  try{result={state:'FOUND',data:await fetchCooperation(client,expected),issues:[]};}
  catch(error){result={state:'UNKNOWN',data:null,issues:[issue(error)]};}
  await session.end();
  return {...result,currentUser:session.scope,observedAt:new Date().toISOString(),profile:STRICT_COOPERATION_PROFILE,...noApproval};
}
/** Read only when current-round creator evidence is needed; the server enforces current membership. */
export async function checkCooperation(client,input={},deps={}){
  options(input,['cooperationId','cooperationFileId','businessId','businessTypeCode','cooperationRoundId']);
  const expected=cooperationExpected(input),fileId=id(input.cooperationFileId),session=await begin(client,deps);
  let result;
  try{
    const before=await fetchCooperation(client,expected);
    need(before.currentRoundFiles.some(file=>file.cooperationFileId===fileId),'CHECK_FILE_NOT_IN_AUTHORIZED_CURRENT_ROUND');
    const data=response(await client.web({method:'GET',path:QUERY_PATHS.cooperationCheck,
      query:{cooperationId:expected.cooperationId,cooperationFileId:fileId},effect:'read'}),'web');
    need(object(data),'COOPERATION_CHECK_NOT_RETURNED');
    const status=count(data.cooperationCheckStatusCode);need(status<=4,'COOPERATION_CHECK_STATUS_UNKNOWN');
    const creator={tenantId:id(data.createEmployeeVO?.tenantId),userId:id(data.createEmployeeVO?.employeeId)};
    const after=await fetchCooperation(client,{...expected,cooperationRoundId:before.cooperationRoundId});
    need(after.cooperationStatusCode===before.cooperationStatusCode&&sameBusiness(after,before),'COOPERATION_OBJECT_OR_ROUND_CHANGED');
    need(sha256(after.currentRoundFiles)===sha256(before.currentRoundFiles),'COOPERATION_CURRENT_FILE_CHANGED');
    if(status===2)need(after.cooperationStatusCode===3,'COOPERATION_CHECK_STATE_MISMATCH');
    if(status===3)need(after.cooperationStatusCode===4,'COOPERATION_CHECK_STATE_MISMATCH');
    const labels=['OK','HAS_NEW_VERSION','COMPLETED','CANCELED','NO_USER_CONFIRM'];
    const refreshRequired=status===1;
    // A stale file check is not reusable role evidence; no automated retry or workflow action.
    result={state:refreshRequired?'REFRESH_REQUIRED':'CHECKED',checkStatusCode:status,checkStatus:labels[status],
      cooperationId:after.cooperationId,cooperationRoundId:after.cooperationRoundId,cooperationFileId:fileId,
      currentUserMembership:refreshRequired?'UNCONFIRMED':'VERIFIED_BY_CHECK',
      creator:refreshRequired?null:creator,isCurrentUserCreator:refreshRequired?null:creator.tenantId===session.scope.tenantId&&creator.userId===session.scope.userId,
      requiresFreshRead:refreshRequired,issues:[]};
  }catch(error){result={state:'UNKNOWN',creator:null,currentUserMembership:'UNCONFIRMED',issues:[issue(error)]};}
  await session.end();return {...result,currentUser:session.scope,observedAt:new Date().toISOString(),...noApproval};
}
function task(data){
  need(object(data),'TASK_NOT_RETURNED');
  return {businessId:id(data.businessId),businessTypeCode:count(data.businessTypeCode),
    taskInstanceId:taskId(data.processTaskInstanceId),processInstanceId:optional(data.processInstanceId,taskId),
    taskTypeCode:count(data.taskTypeCode),taskNodeId:optional(data.taskNodeId,taskId),taskNodeTypeCode:optional(data.taskNodeTypeCode,count),
    taskNodeName:label(data.taskNodeDisplayName)??label(data.taskNodeName),contractName:label(data.contractName),
    contractNumber:label(data.contractNumber),contractStatusCode:optional(data.contractStatusCode,count)};
}
async function fetchTask(client,taskInstanceId,wanted){
  const data=response(await client.web({method:'GET',path:QUERY_PATHS.taskInfo,query:{taskInstanceId},effect:'read'}),'web');
  if(data===null)return {state:'NOT_RETURNED',data:null,issues:[{code:'TASK_NOT_RETURNED'}]};
  const value=task(data);
  need(value.taskInstanceId===taskInstanceId&&sameBusiness(value,wanted),'TASK_OBJECT_MISMATCH');
  return {state:value.taskTypeCode===0?'CURRENT_TODO':'NOT_CURRENT_TODO',data:value,issues:[]};
}
/** No noticeId: this endpoint verifies the current authenticated task assignee. TODO is not RUNNING. */
export async function readCurrentTask(client,input={},deps={}){
  options(input,['taskInstanceId','businessId','businessTypeCode']);
  const wanted=target(input),candidate=taskId(input.taskInstanceId),session=await begin(client,deps);
  let result;try{result=await fetchTask(client,candidate,wanted);}catch(error){result={state:'UNKNOWN',data:null,issues:[issue(error)]};}
  await session.end();return {...result,currentUser:session.scope,observedAt:new Date().toISOString(),serverRunningState:'UNCONFIRMED',routeResolution:'TASK_CARRIER_ONLY',...noApproval};
}
/** Enumerate current-user TODO without old server filters; match exact business ID + type locally. */
export async function findCurrentTasks(client,input={},deps={}){
  options(input,['businessId','businessTypeCode','pageSize','maxPages']);
  const wanted=target(input),requestedSize=input.pageSize??100,maxPages=input.maxPages??100;
  need(Number.isInteger(requestedSize)&&requestedSize>=1&&requestedSize<=100&&Number.isInteger(maxPages)&&maxPages>=1&&maxPages<=10000,'INVALID_TASK_SCAN_LIMIT');
  const session=await begin(client,deps),issues=[],seen=new Set(),pageHashes=new Set(),matches=new Map();
  let total=null,pageSize=requestedSize,pages=0,complete=false;
  for(let pageIndex=0;pageIndex<maxPages;pageIndex++){
    try{
      const data=response(await client.web({method:'GET',path:QUERY_PATHS.taskList,
        query:{taskTypeCode:0,pageIndex,pageSize,desc:true,limitBefore:0,limitAfter:pageSize-1},effect:'read'}),'web');
      need(object(data)&&Array.isArray(data.content),'TASK_LIST_SHAPE');
      const actualSize=count(data.pageSize),reportedTotal=count(data.totalElements);
      need(actualSize>0&&actualSize<=pageSize,'TASK_PAGE_SIZE_INVALID');
      if(pages>0)need(actualSize===pageSize,'TASK_PAGE_SIZE_CHANGED');
      if(data.pageIndex!==undefined&&data.pageIndex!==null)need(count(data.pageIndex)===pageIndex,'TASK_PAGE_INDEX_CHANGED');
      if(total!==null)need(total===reportedTotal,'TASK_TOTAL_CHANGED');
      total=reportedTotal;pageSize=actualSize;pages++;
      need(data.content.length<=pageSize,'TASK_PAGE_EXCEEDS_LIMIT');
      const rows=data.content.map(task),hash=sha256(rows.map(row=>row.taskInstanceId));
      if(rows.length)need(!pageHashes.has(hash),'TASK_PAGE_REPEATED');pageHashes.add(hash);
      for(const row of rows){
        need(row.taskTypeCode===0,'TASK_LIST_NOT_TODO');
        if(seen.has(row.taskInstanceId)){issues.push({code:'TASK_DUPLICATE_OR_SHIFT'});continue;}
        seen.add(row.taskInstanceId);if(sameBusiness(row,wanted))matches.set(row.taskInstanceId,row);
      }
      need(seen.size<=total,'TASK_OBSERVED_COUNT_EXCEEDS_TOTAL');
      // The underlying page can be short or empty after resource joins. Total counts raw BPM tasks.
      if((pageIndex+1)*pageSize>=total){complete=issues.length===0;break;}
    }catch(error){issues.push(issue(error));break;}
  }
  if(!complete&&issues.length===0)issues.push({code:'TASK_SCAN_LIMIT_REACHED'});
  const items=[],unresolved=[];
  for(const [candidate] of matches){
    try{
      const verified=await fetchTask(client,candidate,wanted);
      if(verified.state==='CURRENT_TODO')items.push(verified.data);
      else unresolved.push({taskInstanceId:candidate,...verified});
    }catch(error){unresolved.push({taskInstanceId:candidate,state:'UNKNOWN',data:null,issues:[issue(error)]});}
  }
  await session.end();
  const resolution=!complete?'INCOMPLETE':unresolved.length?'REFRESH_REQUIRED':items.length===0?'NO_MATCHING_CURRENT_TODO':items.length===1?'ONE_CURRENT_TASK':'MULTIPLE_CURRENT_TASKS';
  return {scope:'CURRENT_USER_TODO',target:wanted,currentUser:session.scope,observedAt:new Date().toISOString(),complete,pages,reportedTotal:total,observedTasks:seen.size,
    actualPageSize:pageSize,items,unresolved,issues,resolution,consistency:'PAGINATED_OBSERVATION_NOT_SNAPSHOT',
    serverRunningState:'UNCONFIRMED',routeResolution:'TASK_CARRIER_ONLY',...noApproval};
}
/** Tenant metadata for a known own template draft only; this is not the object's two-in-one flag. */
export async function readDraftRouteConfig(client,deps={}){
  const session=await begin(client,deps);let result;
  try{
    const data=response(await client.web({method:'GET',path:QUERY_PATHS.draftRouteConfig,effect:'read'}),'web');
    need(object(data),'DRAFT_ROUTE_CONFIG_NOT_RETURNED');
    if(data.employeeId!==undefined&&data.employeeId!==null)need(id(data.employeeId)===session.scope.userId,'SSO_IDENTITY_MISMATCH');
    if(data.tenantId!==undefined&&data.tenantId!==null)need(id(data.tenantId)===session.scope.tenantId,'SSO_IDENTITY_MISMATCH');
    need(typeof data.systemConfigVO?.isOneStepTemplate==='boolean','ONE_STEP_TEMPLATE_CONFIG_UNKNOWN');
    result={state:'FOUND',isOneStepTemplate:data.systemConfigVO.isOneStepTemplate,issues:[]};
  }catch(error){result={state:'UNKNOWN',isOneStepTemplate:null,issues:[issue(error)]};}
  await session.end();return {...result,currentUser:session.scope,observedAt:new Date().toISOString(),scope:'CURRENT_TENANT_DRAFT_ROUTE_CONFIG',...noApproval};
}
