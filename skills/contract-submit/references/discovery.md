# 本人草稿、协商与送审对象定位

按[会话与调用上下文](runtime.md)固定 USER、环境、租户和员工。优先复用本轮准确已获权的合同、本人草稿或协商事实，避免重新扫描。只有上传文件 ID 时不能代替合同 ID。编号可按完整字符串核对，名称筛选可按原文字包含，用户要求全名时逐字匹配。多个条件采用 AND；字段取不到则显示待核实，不忽略条件。未知或重名请用户从真实候选中选择。

## 本人编辑中草稿

```sh
contract-cli api call --channel web --profile '<已核 profile>' --as user --raw POST /open-web/web/clm/es/search/contractList --data '{"pageIndex":0,"pageSize":20,"searchTabCode":2,"conditionUnits":[],"filterUnits":[]}'
```

`searchTabCode=2` 是本 USER 单合同草稿。用户要求组清单时才另扫 `3`，父组与单合同分开记录。每次只增加 `pageIndex`；`pageSize` 默认 20，合法 1–30。此路径按上下文过滤本人 owner；不传租户或用户覆盖，两个条件数组保持空，本轮筛选在已获权摘要中进行。

业务成功需 `code=200`、`success=true`，`data.content[]` 各项恰有一条 `searchContractListItems[]` 摘要。保留 `contractId`、`contractNumber`、`contractName`、`sourceTypeCode`、`sourceTypeName`、`templateId`、`contractCategoryId`、`assemblyId`、`cooperationId`、`businessTypeCode` 等实际值。父组或组成员不能走独立业务路径；`assemblyId` 缺失不能推广为所有来源都是独立合同。

以 `data.filteredTotalElements` 作当前 tab 的已核总数，不能用 `totalElements` 替代。在页号、页大小、总数、实际可达窗口及去重都一致时，累计达到总数才宣称该 tab 已查全。缺计数、重复、变化、短页/空页但未达总数、失败或预算耗尽时保留已验证候选并标记结果不完整。仅选择已唯一匹配且可核实的目标；用户可以从部分结果中明确选一份继续。

## 我的协商

当前请求来自协商，或草稿未命中且用户范围包括协商时，读取 MY 范围。请求体固定 `searchTabEnumCode=1`、`cooperationDisplayTabCode=0`，`keyword` 可为空或用户原关键词。以 JSON 文件承载动态文本。每页保持同一认证、时区和条件：

```sh
contract-cli api call --channel web --profile '<已核 profile>' --as user --raw POST /open-web/web/clm/cooperation/searchV2 --query 'pageIndex=0' --query 'pageSize=20' --header 'Timezone-Offset: <业务时区偏移分钟>' --input-file '<本任务私有请求 JSON 文件>'
```

时区为 `-840` 至 `840` 的实值，中国标准时区为 `-480`；页从 `0` 递增，页大小默认 20、允许 1–100。检查 Web 成功、`data.content[]`、`data.totalElements` 以及页索引和分页窗口，按实际 `cooperationId` 原文去重。短页可能来自失效数据，不能单凭短页宣称已读完；读取到总数对应的末页且实收等于该总数才算完整。MY 最多读取前 10000 偏移窗口内可达整页，响应提供更小窗口时遵守实际窗口。默认两类查询各 100 页为本任务预算，可在受权范围内按实际需要增加；达到窗口或预算均说明不完整。

存在 `paginationCapability` 时核对 `strategy=OFFSET_WINDOW`、请求页大小与返回一致、`reachablePageCount=floor(offsetWindow/pageSize)`。草稿 `unit=CONTRACT_GROUP`，协商 `unit=COOPERATION`；未知或变化时停止该范围续页。名称关键词检索结果仍要逐项保护精确匹配。

正文存于底层合同。保留 `cooperationId`、`cooperationRoundId`、`businessId`、`businessTypeCode`、`cooperationStatusCode`、`cooperationStageCode`、`isCooperationUser`、`todoTaskId`、合同名和编号。先证明确实为本系统合同，再把 `businessId` 接到 `contractId`。当前轮成员由同轮 `isCooperationUser=true` 支持；原创建者、历史参与者、能查看协商均不能推成当前可提交。

## 当前状态、角色和任务补证

按需选用现有窄接口，继续沿用当前 profile、USER 与 Web 通道。除严格详情外，读取 `code=200`、`success=true` 的 `data`；失败保留具体对象、步骤和原因。

| 所需事实 | 已核查询和结果衔接 |
| --- | --- |
| 当前轮、文件与底层业务 | GET `/open-web/web/clm-openapi/v1/mcp/cooperations/<已核协商ID>`，header `X-MCP-Response-Profile: clm-web-status-v2.0`，无 query/body。使用 `code=0` 且 `success` 非 false 的 `data`。核对 `cooperation_id`、`cooperation_round_id`、`business_id/business_type_code`、`cooperation_status_code` 及 `cooperation_stage_code`。使用 `current_round_files[].cooperation_file_id`，不误用 `file_id`。 |
| 需要当前轮创建者的完整身份 | 严格详情 → GET `/open-web/web/clm/cooperation/checkCooperation`，query `cooperationId=<协商ID>`、`cooperationFileId=<当前轮有效文件ID>` → 再读严格详情。同对象、轮次、状态、文件版本稳定且 `cooperationCheckStatusCode=2` 对应协商完成，才用 `data.createEmployeeVO.tenantId/employeeId` 比较当前 USER。检查码 `1` 是文件变化，不采用旧结论。 |
| 有协商合同的当前基础状态 | GET `/open-web/web/clm/contract/briefInfo`，query `contractId=<真实合同ID>`。比对 `tenantId`、`contractId`、`cooperationId`、`contractStatusCode` 与 `ownerEmployeeId`；实际 `processInstanceId` 可用于流程衔接。`data=null` 不能断言无合同，`groupId` 是版本分组，不作父合同组 ID。 |
| 有真实 `todoTaskId` 的本人待办 | GET `/open-web/web/clm/task/info`，仅 query `taskInstanceId=<实际任务ID>`，不传 `noticeId`。`data.processTaskInstanceId`、`businessId/businessTypeCode` 必须同对象，`taskTypeCode=0` 只支持「有当前本人待办」；仍需实际重提资格和协议。 |

草稿列表、窄详情和权限候选只供定位与前置核对。预检满足的对象交给[表单协议](form.md)补核实际正文、对象类型、授权和对应页面。协商完成的首次提交核当前合同 owner、轮次身份和实际提交动作。有真实当前任务的重提另按[提交协议](submit.md)确认。

协商中按[协商完成前置](cooperation-submit.md#1-核适用动作和清洁方式)核本对象实际模式、当前轮、对应动作和服务端清洁生效值。状态码、参与者身份、列表确认码和类型配置都不足以独立证明资格。待发起、取消、回滚或被拒绝时说明相应前置。不从普通送审意图推导发起、跳过、单独完成或恢复协商的授权。查询结束核验上下文。已进入审批或提交中的对象先回查，避免重复。
