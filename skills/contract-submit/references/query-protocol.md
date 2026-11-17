# 链接办理所需的窄查询

这里维护新增查询的请求、响应及数据衔接；操作资格只在[权限规则](permissions.md)维护，页面地址只在[页面入口](native-page-handoff.md)和[场景覆盖](scenario-actions.md)维护。先沿用已有新鲜事实，再按缺少的信息选最少查询；仅给协商或任务承载页，不必取得全部提交条件。

所有调用均使用当前已核 USER 的 `createClient(context)`；环境、tenantId、userId 及官方授权复核见[运行上下文](runtime.md)。不传身份覆盖，不静默改APP。业务ID保留完整字符串，任务ID保留服务端原字符串。下面路径已是public路径，不能再增加API前缀或转换为完整URL交给调用器。

## 1. 使用顺序与确定性助手

`scripts/native-queries.mjs` 导出以下函数，`client`来自 `scripts/runtime-core.mjs`；以安装目录为基准导入。函数的最后一个依赖参数只供离线测试，正常运行不覆盖认证/传输。

| 已有事实/需要补充的事实 | 调用 | 输出接下一步 |
|---|---|---|
| 已知协商ID，需要真实当前轮状态、业务关联或文件 | `readCooperationDetail(client,{cooperationId,businessId?,businessTypeCode?,cooperationRoundId?})` | FOUND后用data里的当前轮、底层业务关联；已给的预期ID必须一致 |
| 已知本人模板草稿，带协商关联，仍按“我的草稿”选择器办理 | `readDraftRouteConfig(client)` | FOUND的明确isOneStepTemplate布尔值，只用于相应草稿路由 |
| 需要补本轮创建者的完整tenant+employee证据 | `checkCooperation(client,{cooperationId,cooperationFileId,businessId?,businessTypeCode?,cooperationRoundId?})` | CHECKED的creator及同轮证据；不是提交授权 |
| 已有真实todoTaskId或当前任务候选 | `readCurrentTask(client,{taskInstanceId,businessId,businessTypeCode})` | CURRENT_TODO才可作为当前本人任务承载入口；业务对象必须一致 |
| 已定位业务对象，需要自己寻找当前待办 | `findCurrentTasks(client,{businessId,businessTypeCode,pageSize:100,maxPages:100})` | 同对象items为已再次鉴权的任务；多任务保留，不能挑最新冒充重提任务 |

助手每次调用前后核对当前用户。身份漂移抛错，整次结果不能混合展示；单个业务查询失败通常返回UNKNOWN、issues和业务错误码。`approvalSubmitted:false`、`approvalPermission:UNCONFIRMED`是所有助手的固定边界，不能据FOUND/CHECKED/CURRENT_TODO改成审批已提交。

## 2. 统一读取规则

- Web ApiResult：必须 `success=true` 且 `code=200`，再处理 `data`。
- 本文严格MCP详情：必须 `code=0`，若带success不得为false，再处理 `data`。不能用Web的200规则解析它。
- HTTP成功、CLI正常退出不是业务成功。失败保留code及所属步骤；缺字段、未知协议、接口未部署、无权和不存在不能互相替代。不自动去掉profile头、换完整详情或换身份。
- null/空数组是具体字段的值，不是全系统不存在。未知状态可原样显示，不参与未定义动作判断。ID、角色或轮次冲突时重新定位，不混用旧轮状态与新轮人员。
- 运行脚本不依赖外部审计文件；已适配的接口不支持时，使用同一对象已有可靠承载入口并说明无法进一步核实。原系统缺口不作为本期研发步骤。

## 3. 协商严格详情

```js
await client.web({
  method: 'GET',
  path: '/open-web/web/clm-openapi/v1/mcp/cooperations/' + cooperationId,
  headers: {'X-MCP-Response-Profile': 'clm-web-status-v2.0'},
  effect: 'read'
});
```

`cooperationId`是MY列表/本轮获权上下文的真实正整数ID。无query/body。profile头始终发送固定已核值；无头会改变权限语义，不能省略。返回未要求回显profile字段，不制造不存在的回显校验。

响应数据为snake_case，助手转换为下列camelCase：

| 原始data字段 | 助手data字段/用途 |
|---|---|
| cooperation_id、cooperation_round_id | cooperationId、cooperationRoundId；同对象与当前轮校验 |
| business_id、business_type_code | businessId、businessTypeCode；可空，空时不能构造合同/组页面 |
| cooperation_status_code、cooperation_stage_code | cooperationStatusCode、cooperationStageCode；以实际当前轮为准 |
| cooperation_name、contract_number | cooperationName、contractNumber；展示或核对，不能替代内部ID |
| create_user.user_id | currentRoundCreatorUserId；这里是employeeId语义，但没有tenantId，不能独自判同一人 |
| current_round_files[].cooperation_file_id | currentRoundFiles[].cooperationFileId；check所需协商文件ID |
| current_round_files[].file_id、latest、cooperation_file_type_code、create_time | fileId、latest、cooperationFileTypeCode、createTime；不要把fileId传成cooperationFileId |
| cooperation_users[].employee.user_id及cooperation_user_type_code/status_code | cooperationUsers的userId及角色/状态；仍缺人员tenant，不能独自证明当前用户成员身份 |

MY列表同轮 `isCooperationUser=true` 才能提供当前USER本轮成员事实；列表可含历史参与对象，默认userType不是成员证据。MY列表createEmployeeVO是最初创建者，strict create_user才指本轮创建者，二者不能互换。

严格详情执行对象可见性校验，且限定创建租户匹配；因此跨租户MY命中可能无法补详情。此时保留MY已核定位事实，不把详情失败解释为整个列表无权，也不换租户重试。

它**不返回对象二合一标记、父组关系或完整按钮权限**。助手明确保留enableFormCooperation=null；不能用SSO同名租户开关代替。只有本轮已有可信的对象快照实际含该值，才可用于相应分流；否则用已核原协商页接续。

## 4. 本轮创建者的部分检查

只在需要精确角色说明时使用；普通列表链接不调用。助手先读取strict详情，核当前轮中确有指定的协商文件，再执行：

```js
await client.web({method:'GET', path:'/open-web/web/clm/cooperation/checkCooperation',
  query:{cooperationId, cooperationFileId}, effect:'read'});
```

两个query始终必填，均是前序实际正整数ID；没有readOnly、checkComplete等额外参数。服务端要求当前tenant+employee为当前轮有效成员；历史参与者或纯可见者失败时保留已有承载页，不调用会自动加入的完整详情。

Web成功响应：

- `data.createEmployeeVO.tenantId/employeeId`：当前轮创建者完整身份，与已核当前USER的两个ID同时比较。
- `data.cooperationCheckStatusCode`：0=OK、1=HAS_NEW_VERSION、2=COMPLETED、3=CANCELED、4=NO_USER_CONFIRM。1表示所选文件版本已变化，助手返回REFRESH_REQUIRED，不复用该次角色结果；未知值为UNKNOWN。
- `data.hasReadFile`只是检查响应，不能说已读记录已持久化；本接口没有替用户标已读。

助手随后复读strict详情，确认协商ID/轮次/状态/业务关联及文件仍一致。CHECKED时可以用creator说明角色；2虽已完成仍可取得creator。0/2均不是complete、skip或startProcess授权，也没有验证所有必填和强确认。不执行checkComplete或任何完成动作来获得链接。

## 5. 已知合同的基础信息

`GET /open-web/web/clm/contract/briefInfo`，唯一query `contractId=<真实合同ID>`，无body；走Web成功规则。

`data`中的 `tenantId/contractId/businessTypeCode/contractStatusCode/ownerEmployeeId/sourceTypeCode/templateId/cooperationId/cooperationStatusCode` 按实际返回保留；`processInstanceId`及`submitterEmployee.tenantId/employeeId`如实际返回，也保留供关联复核，首次草稿申请人可为空。核请求对象与返回关联再使用。当前USER的tenant+owner、状态0/31等按权限规则判断；协商创建者不替代owner。流程ID或申请人本身仍不能证明当前任务具有重提动作。

它只在有效合同关联有效协商时返回基础VO：`data=null`不能区分无协商、无对象等原因，不能报告合同不存在。它不补assemblyId、editModeEnabled或正文上传标记；groupId是版本分组，不是合同组父ID。仅有成员ID不能因此构造父组链接。

## 6. 当前USER任务发现与复核

起点必须有实际 `businessId` 和 `businessTypeCode`。普通父组10、模板父组17使用父组ID；专用单据用自身业务ID；不拿名称或成员ID代替。

本人TODO请求：

```js
await client.web({method:'GET', path:'/open-web/web/clm/task/list/v2',
  query:{taskTypeCode:0,pageIndex:0,pageSize:100,desc:true,limitBefore:0,limitAfter:99},
  effect:'read'});
```

- taskTypeCode始终0；pageIndex从0递增；pageSize默认100，助手支持1–100；limitAfter始终本次pageSize减1、limitBefore=0；desc=true。
- 不传query关键词、businessTypeCodes、nodeTypes、taskInstanceId、noticeId、onlyUrge、read、租户或人员覆盖字段。旧服务端筛选会触发另一种截断链，不能以此证明完整发现。
- 响应 `data.content[]`按businessId+businessTypeCode精确匹配；保留processTaskInstanceId、processInstanceId、taskNodeId/type/name和任务/合同状态。
- 以实际 `data.pageSize`（服务端可能缩小）继续分页；终止为 `(pageIndex+1)*实际pageSize >= totalElements`。totalElements是本人原始任务总数，不是匹配合同数。主表关联可能使页面短或空，不能据此提前结束；不要求hasNext/totalPages存在。
- 总数变化、重复页/任务、错误或maxPages预算用尽保留INCOMPLETE；不能宣称无任务。分页是当前观察，不是并发一致快照。

每个匹配项、或实际协商返回的todoTaskId候选，再核：

```js
await client.web({method:'GET', path:'/open-web/web/clm/task/info',
  query:{taskInstanceId}, effect:'read'});
```

**始终不传noticeId。**该分支按当前用户实际assignee鉴权；先前列表命中不能替代本次鉴权。Web成功后，核 `data.processTaskInstanceId/businessId/businessTypeCode` 与候选/目标全部一致。`taskTypeCode=0`表示未结束的TODO，不等于原始RUNNING、原合同最新流程或已具备RESUBMIT权限。流程ID也只是该任务的关联流程。

输出状态：CURRENT_TODO可给真实任务承载页；NOT_CURRENT_TODO表示已不在当前待办；NOT_RETURNED表示未返回对象；UNKNOWN保留错误。列表 `items`只放复核通过的当前任务，其他放unresolved。多个有效任务保留节点区分；不自动选最新一个。无匹配仅说明本次本人待办范围内没有发现，不能宣称用户没有任何合同权限。

没有已核完整重提条件时文案为“打开当前任务”；不使用permission/business/list替代任务鉴权。此链已足以供页面继续办理，不额外读取审批记录、完整合同或composition来推测重提资格。

## 7. 本人模板草稿的路由配置

`GET /open-web/web/clm/sso/getUser`，无query/body，走Web成功规则。助手读取 `data.systemConfigVO.isOneStepTemplate` 的明确布尔值；employeeId/tenantId如返回必须与当前USER一致。缺失/null不是false，返回UNKNOWN。

只在本人模板草稿关联协商且草稿选择器需要此配置时调用。配置包含当前用户/租户元信息及缓存预热，不修改合同；不读取或输出凭据，不使用更宽的collectUserDetail。

这是租户草稿路由配置，不能替代协商对象的二合一标记，也不是非负责人的提交许可。完成态“进入审批”分支不需要这个开关，不为模板来源额外调用。
