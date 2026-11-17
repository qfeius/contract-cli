import {pathToFileURL} from 'node:url';
import {createClient,parseExact,stringifyExact,scalar,sha256,readPrivateJson} from './runtime-core.mjs';
import {verifyUserContext,assertSameEndpointContext} from './runtime-context.mjs';

export const DRAFT_ROUTE='/open-web/web/clm/es/search/contractList';
const FILTERS=new Set(['id','number','name','kind','businessTypeCode','sourceTypeCode','templateId','categoryId']);
const FIELDS=['contractNumber','contractName','businessTypeCode','businessTypeName','sourceTypeCode','sourceTypeName','templateId','contractCategoryId','contractCategoryName','contractCategoryDisplayName','createTime','assemblyId','cooperationId','editModeEnabled'];
const owns=(x,k)=>Object.hasOwn(x,k);
const id=value=>{const text=String(scalar(value));if(!/^[1-9][0-9]*$/u.test(text))throw Error('INVALID_OBJECT_ID');return text;};
const integer=value=>{const text=String(scalar(value));if(!/^[0-9]+$/u.test(text)||!Number.isSafeInteger(Number(text)))throw Error('INVALID_COUNT');return Number(text);};
const canonical=value=>value===null||value===undefined?null:String(scalar(value));
function paginationCapability(value,pageSize){
  if(value==null)return {state:'legacy'};
  const positive=n=>typeof n==='number'&&Number.isSafeInteger(n)&&n>0;
  if(typeof value!=='object'||Array.isArray(value)||value.strategy!=='OFFSET_WINDOW'||value.unit!=='CONTRACT_GROUP'||!positive(value.offsetWindow)||!positive(value.pageSize)||value.pageSize!==pageSize||!positive(value.reachablePageCount)||value.reachablePageCount!==Math.floor(value.offsetWindow/pageSize)||!Number.isSafeInteger(value.reachablePageCount*pageSize))throw Error('DRAFT_PAGINATION_CAPABILITY_INVALID');
  return {state:'valid',strategy:value.strategy,unit:value.unit,offsetWindow:value.offsetWindow,pageSize:value.pageSize,reachablePageCount:value.reachablePageCount};
}

export function validateFilters(filters={}) {
  if(!filters||typeof filters!=='object'||Array.isArray(filters))throw Error('FILTER_OBJECT_REQUIRED');
  for(const [key,value] of Object.entries(filters)){
    if(!FILTERS.has(key))throw Error('UNSUPPORTED_FILTER:'+key);
    if(typeof value!=='string'||!value.trim()||value.trim()!==value)throw Error('FILTER_VALUE_REQUIRED:'+key);
    if(['id','templateId','categoryId'].includes(key))id(value);
    if(['businessTypeCode','sourceTypeCode'].includes(key))integer(value);
    if(key==='kind'&&!['single','group','template_batch'].includes(value))throw Error('INVALID_DRAFT_KIND');
  }
  return {...filters};
}
function summary(row,tab){
  if(!row||!Array.isArray(row.searchContractListItems)||row.searchContractListItems.length!==1)throw Error('UNEXPECTED_DRAFT_GROUP_SHAPE');
  const item=row.searchContractListItems[0]; const objectId=id(item.contractId);
  const type=canonical(item.businessTypeCode);
  const kind=tab===2?'single':type==='10'?'group':type==='17'?'template_batch':'unknown_assembly';
  if(tab===2&&item.assemblyId!=null&&canonical(item.assemblyId)!=='0')throw Error('SINGLE_DRAFT_HAS_PARENT');
  if(tab===3&&item.assemblyId!=null&&id(item.assemblyId)!==objectId)throw Error('ASSEMBLY_ID_MISMATCH');
  const result={objectKey:(tab===2?'contract:':'assembly:')+objectId,id:objectId,kind,tab,ownerScope:'current_authenticated_user',draftState:'DRAFT_EDITING',provenance:'OWN_DRAFT_LIST',metadata:{}};
  for(const key of FIELDS)if(owns(item,key))result.metadata[key]=item[key]===null?null:scalar(item[key]);
  return result;
}
export function matchesDraft(item,filters){
  const mapping={id:item.id,kind:item.kind,number:item.metadata.contractNumber,name:item.metadata.contractName,businessTypeCode:item.metadata.businessTypeCode,sourceTypeCode:item.metadata.sourceTypeCode,templateId:item.metadata.templateId,categoryId:item.metadata.contractCategoryId};
  const missing=[];
  for(const [key,wanted]of Object.entries(filters)){
    const actual=canonical(mapping[key]);
    if(actual===null){missing.push(key);continue;}
    if(!(key==='name'?actual.includes(wanted):actual===wanted))return {match:false,unresolved:[]};
  }
  return {match:missing.length?null:true,unresolved:missing};
}
/** Reads only the current user's own drafts. Never submits, selects or expands group children. */
export async function discoverDrafts(client,{filters={},pageSize=20,maxPages=100,displayOffset=0,displayLimit=20}={},dependencies={}){
  validateFilters(filters);
  if(client.context?.identity!=='user')throw Error('OWN_DRAFT_DISCOVERY_REQUIRES_USER');
  if(!Number.isInteger(pageSize)||pageSize<1||pageSize>30||!Number.isInteger(maxPages)||maxPages<1||maxPages>10000)throw Error('INVALID_SCAN_LIMIT');
  if(!Number.isInteger(displayOffset)||displayOffset<0||!Number.isInteger(displayLimit)||displayLimit<1||displayLimit>100)throw Error('INVALID_DISPLAY_PAGE');
  const verify=dependencies.verifyContext??verifyUserContext;
  const currentUser=await verify(client);
  if(!currentUser||![currentUser.tenantId,currentUser.userId].every(v=>typeof v==='string'&&/^[1-9]\d*$/.test(v)))throw Error('DRAFT_CURRENT_USER_NOT_VERIFIED');
  const rows=[],issues=[],scans=[];
  for(const tab of [2,3]){
    let total=null,pages=0,successfulPages=0,complete=false,capability=null; const seen=new Set(),pageHashes=new Set();
    for(let pageIndex=0;pageIndex<maxPages;pageIndex++){
      let data;
      try{
        if(capability?.state==='valid'&&pageIndex>=capability.reachablePageCount)throw Error('DRAFT_OFFSET_WINDOW_REACHED');
        const result=await client.web({method:'POST',path:DRAFT_ROUTE,body:{pageIndex,pageSize,searchTabCode:tab,conditionUnits:[],filterUnits:[]},effect:'read'});
        if(result?.success!==true||canonical(result?.code)!=='200')throw Error('DRAFT_LIST_BUSINESS_FAILURE');
        successfulPages++;
        data=result.data;
        if(!data||!Array.isArray(data.content))throw Error('DRAFT_LIST_SHAPE');
        // Missing totals do not erase valid candidates or prove an empty scope.
        const missingTotal=data.filteredTotalElements==null;
        let reported=null;
        if(!missingTotal){
          try{
            if(!['number','string'].includes(typeof scalar(data.filteredTotalElements)))throw Error('INVALID_COUNT');
            reported=integer(data.filteredTotalElements);
          }catch{throw Error('DRAFT_TOTAL_INVALID');}
          if(total!==null&&reported!==total)throw Error('DRAFT_TOTAL_CHANGED');
        }
        const currentCapability=paginationCapability(data.paginationCapability,pageSize);
        if(capability!==null&&stringifyExact(currentCapability)!==stringifyExact(capability))throw Error('DRAFT_PAGINATION_CAPABILITY_CHANGED');
        if(data.pageIndex!=null&&integer(data.pageIndex)!==pageIndex)throw Error('DRAFT_PAGE_INDEX_CHANGED');
        if(data.content.length>pageSize)throw Error('DRAFT_PAGE_EXCEEDS_LIMIT');
        const summaries=data.content.map(row=>summary(row,tab));
        const digest=sha256(summaries.map(row=>row.objectKey));
        if(summaries.length&&pageHashes.has(digest))throw Error('DRAFT_PAGE_REPEATED');
        const pageKeys=new Set();
        for(const row of summaries){
          if(seen.has(row.objectKey)||pageKeys.has(row.objectKey))throw Error('DRAFT_DUPLICATE_OR_SHIFT');
          pageKeys.add(row.objectKey);
        }
        if(reported!==null&&seen.size+summaries.length>reported)throw Error('DRAFT_COUNT_EXCEEDS_TOTAL');
        // Accept the whole validated page together, never half a corrupt page.
        capability=currentCapability;pages++;
        if(reported!==null)total=reported;
        pageHashes.add(digest);
        for(const row of summaries){seen.add(row.objectKey);rows.push(row);}
        if(missingTotal){
          issues.push({tab,code:'DRAFT_TOTAL_MISSING',pageIndex,returnedCount:summaries.length});
          break;
        }
        if(seen.size===total){complete=true;break;}
        if(summaries.length<pageSize)throw Error('DRAFT_PAGE_ENDED_BEFORE_TOTAL');
      }catch(error){issues.push({tab,code:typeof error.code==='string'?error.code:(/^[A-Z0-9_]+$/u.test(error.message)?error.message:'DRAFT_READ_FAILED')});break;}
    }
    if(!complete&&!issues.some(issue=>issue.tab===tab))issues.push({tab,code:capability?.state==='valid'&&pages>=capability.reachablePageCount?'DRAFT_OFFSET_WINDOW_REACHED':'DRAFT_SCAN_LIMIT_REACHED'});
    scans.push({tab,pages,successfulPages,reportedTotal:total,observedCount:seen.size,complete,paginationCapability:capability});
  }
  const finalUser=await verify(client);
  assertSameEndpointContext(currentUser,finalUser);
  if(!finalUser||finalUser.tenantId!==currentUser.tenantId||finalUser.userId!==currentUser.userId)throw Error('DRAFT_CURRENT_USER_CHANGED_RELOAD_REQUIRED');
  const matches=[],unresolved=[];
  for(const row of rows){const match=matchesDraft(row,filters);if(match.match===true)matches.push(row);else if(match.match===null)unresolved.push({...row,missingFilterMetadata:match.unresolved});}
  const complete=scans.every(scan=>scan.complete)&&unresolved.length===0;
  return {scope:'CURRENT_USER_OWN_DRAFTS',currentUser:{tenantId:currentUser.tenantId,userId:currentUser.userId},filters,complete,scans,issues,unresolved,
    observedMatches:matches.length,matchedTotal:complete?matches.length:null,
    resolution:!complete?'INCOMPLETE':matches.length===0?'NO_OWN_DRAFT_MATCH':matches.length===1?'UNIQUE_CANDIDATE':'MULTIPLE_CANDIDATES',
    items:matches.slice(displayOffset,displayOffset+displayLimit),displayOffset,displayLimit,observedAt:new Date().toISOString(),
    hasMoreObserved:displayOffset+displayLimit<matches.length,
    requiresFreshObjectAndPermissionRead:true,approvalSubmitted:false};
}
if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href){
  const [operation,contextFile,filterJSON='{}',optionsJSON='{}']=process.argv.slice(2);
  if(operation!=='list'||!contextFile){process.stderr.write('Usage: node draft-discovery.mjs list PRIVATE_CONTEXT.json FILTER_JSON [SCAN_OPTIONS_JSON]\n');process.exitCode=2;}
  else try{const context=await readPrivateJson(contextFile);const options=parseExact(optionsJSON);if(!options||Array.isArray(options)||typeof options!=='object'||Object.keys(options).some(k=>!['pageSize','maxPages','displayOffset','displayLimit'].includes(k)))throw Error('DRAFT_SCAN_OPTIONS_INVALID');const result=await discoverDrafts(createClient(context),{...options,filters:parseExact(filterJSON)});process.stdout.write(stringifyExact(result)+'\n');}
  catch(error){process.stderr.write((typeof error.code==='string'?error.code:/^[A-Z0-9_]+(?::[A-Za-z]+)?$/u.test(error.message)?error.message:'DRAFT_DISCOVERY_FAILED')+'\n');process.exitCode=1;}
}
