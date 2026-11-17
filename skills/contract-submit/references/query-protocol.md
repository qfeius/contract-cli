# 链接办理所需的窄查询

这里维护已核查询的请求、响应及数据衔接；操作资格只在[权限规则](permissions.md)维护，页面地址只在[页面入口](native-page-handoff.md)和[场景覆盖](scenario-actions.md)维护。先沿用已有新鲜事实，再按缺少的信息选最少查询；仅给协商或任务承载页，不必取得全部提交条件。

Agent 按以下配方直接调用现有 CLI。环境、当前个人授权、租户与员工身份、查询前后复核及长 ID 保真统一按[运行上下文](runtime.md)执行。不要切为 APP 或传入身份覆盖；本文路径已经是公共路径，不能再补前缀或改为完整 URL。命令中的尖括号内容由 Agent 用前序真实结果替换，并按实际命令环境安全传参，不让用户提供内部 ID，也不原样执行占位文本。

## 1. 按缺少的事实选择查询

| 已有事实与需要补充的事实 | 执行步骤 | 取得结果后继续 |
| --- | --- | --- |
| 已知协商 ID，需要当前轮、底层业务关联或文件 | 第 3 节：协商严格详情 | 核对协商 ID，以及本轮已有的业务 ID、业务类型和轮次；使用相符的实际响应 |
| 已知本人模板草稿带协商关联，需要草稿选择器配置 | 第 7 节：草稿路由配置 | 使用明确的 `isOneStepTemplate` 布尔值；缺失时不作默认值 |
| 需要本轮创建者的完整租户与员工身份 | 第 3 节详情 → 第 4 节检查 → 再读第 3 节详情 | 文件、轮次、状态及业务关联均未变化后，才解释创建者身份；不是提交许可 |
| 已有真实 `todoTaskId` 或任务候选 | 第 6 节：任务详情 | 服务端鉴权成功、对象关联一致且仍为当前 TODO，才给当前任务入口 |
| 已定位业务对象，需要查找自己的当前待办 | 第 6 节：本人 TODO 列表 → 对匹配项逐一读任务详情 | 展示复核通过的任务；多个任务保留节点区分，不挑最新项冒充重提任务 |
| 已有真实合同 ID，需要合同基础状态或负责人 | 第 5 节：合同基础信息 | 核对实际返回的对象及关联，再按权限和页面规则判断 |

每段查询开始与结束均按运行上下文核对身份和环境；中途发生变化时，不能把前后结果混合展示。单个业务查询失败保留具体步骤与错误，可说明“尚未核实”（UNKNOWN）；取得详情或本人任务不等于已获得提交权限，更不等于审批已提交。

## 2. 统一读取规则

- Web ApiResult 必须同时满足 `success=true`、`code=200`，再使用 `data`。
- 第 3 节严格协商详情必须满足 `code=0`，若带 `success` 则不得为 false，再使用 `data`。不能套用 Web 的 200 规则。
- HTTP 成功、CLI 正常退出不是业务成功。失败保留业务错误码及所属步骤；缺字段、未知协议、未部署、无权限和不存在分别解释。不自动去掉指定响应头、改查完整详情或换身份。
- `null`、空数组、缺失字段与明确的 false 各按原含义处理；未知状态保留原值，不进入未核实的操作分支。协商、业务或任务 ID 按完整字符核对，不转成普通浮点数。
- 已有预期对象、类型或轮次必须与响应一致；冲突时停止采用该次结果并重新定位，不能混用旧轮状态与新轮人员。所需集合缺失、元素结构异常或重复 ID，按数据未核实处理。
- 本文所有命令均为已核查询。接口不可用时保留同一对象已有的可靠页面入口，并说明不能进一步核实；不为取得链接调用 `cooperation/info`、`composition/contractAndTask`、完整合同详情、正文导出、预提交或审批启动，也不把原系统缺口变成本期研发步骤。

## 3. 协商严格详情

```sh
contract-cli api call --channel web --profile '<本次已核profile>' --as user --raw GET '/open-web/web/clm-openapi/v1/mcp/cooperations/<已核cooperationId>' --header 'X-MCP-Response-Profile: clm-web-status-v2.0'
```

`cooperationId` 来自 MY 列表或本轮已获权上下文，必须是完整的正整数 ID。没有 query 或 body。该固定响应头必须保留；缺少它会改变权限语义。响应未要求回显该头，不制造回显校验。

成功后直接使用实际 `data` 字段，无需另建转换协议：

| 原始字段 | 用途与衔接 |
| --- | --- |
| `cooperation_id`、`cooperation_round_id` | 协商 ID 与当前轮次；必须为有效 ID，且与请求及已有轮次相符。前者用于后续查询和页面的 `cooperationId` 参数，后者用于同轮核对 |
| `business_id`、`business_type_code` | 底层业务 ID 与类型；已有预期值时须相符，可空。先核真实对象族和父子关系，独立合同才将前者用作 `contractId`，真实父组才按场景规则用作 `contractAssemblyId`；成员 ID 不映射成父组 ID。空时不构造合同或组的页面 |
| `cooperation_status_code`、`cooperation_stage_code` | 当前轮状态与阶段；前者交给页面入口文档的协商子状态分支，不代替合同总状态；状态缺失或非法时不判定操作资格 |
| `cooperation_name`、`contract_number` | 名称及编号，仅供展示和核对，不能替代内部 ID |
| `create_user.user_id` | 当前轮创建者的员工 ID；未包含租户，不能独自证明是当前用户。完整身份补证使用第 4 节的 `createEmployeeVO.tenantId/employeeId`，不将此字段误当已包含租户的身份 |
| `current_round_files[].cooperation_file_id` | 第 4 节所需的协商文件 ID；应为有效且不重复的 ID |
| `current_round_files[].file_id`、`latest`、`cooperation_file_type_code`、`create_time` | 文件与版本事实，保留实际值；`file_id` 不能作为 `cooperationFileId` |
| `cooperation_users[].employee.user_id`、`cooperation_user_type_code`、`cooperation_user_status_code` | 本轮人员及角色信息；缺少人员租户，不能独自证明当前用户成员身份 |

`current_round_files` 和 `cooperation_users` 应实际返回数组；缺失不能当作已确认空数组。核对其元素及协商文件 ID 后再用于后续检查。

MY 列表同轮 `isCooperationUser=true` 才提供当前 USER 的本轮成员事实；列表可能含历史参与对象，默认 `userType` 不是成员证据。MY 列表的 `createEmployeeVO` 是最初创建者，这里的 `create_user` 指本轮创建者，不能互换。

严格详情执行对象可见性校验，且限定创建租户匹配；跨租户 MY 命中可能无法补详情。此时保留 MY 已核定位事实，不把详情失败解释为整个列表无权限，也不换租户重试。

此响应不提供对象二合一标记、父组关系或完整按钮权限。`enableFormCooperation` 应保持“未确认”，不能把未返回当 false，或用 SSO 同名租户开关替代。只有本轮已有可信对象快照实际提供该值，才用于相应分流；否则用已核原协商页接续。

## 4. 本轮创建者的部分检查

只在需要精确角色说明时使用；普通列表链接不调用。先执行第 3 节，确认当前轮文件集合中确有选定的 `cooperation_file_id`，记录该轮次、状态、底层业务关联及文件集合，再执行：

```sh
contract-cli api call --channel web --profile '<本次已核profile>' --as user --raw GET '/open-web/web/clm/cooperation/checkCooperation' --query 'cooperationId=<已核cooperationId>' --query 'cooperationFileId=<该轮已核cooperation_file_id>'
```

两个 query 均必填，取前序实际正整数 ID；不增加 `readOnly`、`checkComplete` 等参数。服务端要求当前租户与员工是当前轮有效成员；历史参与者或纯可见者失败时保留已有页面，不改查会自动加入的完整详情。

Web 业务成功后：

- `data.createEmployeeVO.tenantId/employeeId` 是本轮创建者完整身份；两个值都实际取得并有效时，才与当前 USER 的租户和员工 ID 同时比较。
- `data.cooperationCheckStatusCode`：0=OK、1=HAS_NEW_VERSION、2=COMPLETED、3=CANCELED、4=NO_USER_CONFIRM。缺失或未知值按未核实处理；1 表示文件版本发生变化，需要重新取得事实（REFRESH_REQUIRED），不沿用该次创建者或成员结论。
- `data.hasReadFile` 只是检查响应，不能说明已持久化已读记录；此查询没有替用户标已读。

随后使用同一协商 ID 再执行第 3 节。协商 ID、轮次、状态、底层业务 ID 与类型，以及当前轮文件集合及版本字段应与前读一致。检查结果为 2 时，严格详情状态应为 3（协商完成）；检查结果为 3 时，应为 4（协商已取消）。任一关系矛盾或字段无法核实，停止采用本次角色结论。

前后核对通过且没有版本变化时，可以说明“创建者身份已核对”（CHECKED）；检查码 2 虽表示完成，仍可取得创建者。检查码 0 或 2 均不是完成协商、跳过协商或启动审批的授权，也没有证明必填项、强确认等已通过。不调用 `checkComplete` 或任何完成动作来验证链接。

## 5. 已知合同的基础信息

```sh
contract-cli api call --channel web --profile '<本次已核profile>' --as user --raw GET '/open-web/web/clm/contract/briefInfo' --query 'contractId=<已核真实合同ID>'
```

唯一 query 是合同 ID，没有 body。按 Web 成功规则读取实际 `data`；核对请求合同与返回 `contractId` 及已有业务关联一致，再保留以下事实：

- `tenantId`、`businessTypeCode`、`contractStatusCode`、`ownerEmployeeId`：用于所属租户、类型、状态和负责人判断。
- `sourceTypeCode`、`templateId`、`cooperationId`、`cooperationStatusCode`：用于已有来源、模板和协商关系；字段未返回不能推默认值。
- `processInstanceId`、`submitterEmployee.tenantId/employeeId`：实际存在时用于流程及申请人关联；首次草稿的申请人可以为空。

当前 USER 的租户与负责人、合同状态 0/31 等仍按权限规则判断；协商创建者不能替代负责人。流程 ID 或申请人本身不能证明当前任务具备重提动作。

该接口只在有效合同关联有效协商时返回基础对象；`data=null` 不能区分无协商、无对象等原因，不能报告合同不存在。它不补 `assemblyId`、`editModeEnabled` 或正文上传标记；`groupId` 是版本分组，不是合同组父 ID。仅有成员 ID 不能构造父组链接。

## 6. 当前 USER 任务发现与复核

起点必须有前序已核实际 `businessId` 和 `businessTypeCode`。普通父组 10、模板父组 17 使用父组 ID；专用单据用自身业务 ID；不拿名称或成员 ID 代替。已获真实任务候选时，直接做本节的任务详情复核，不重复扫描列表。

没有任务候选时，读取本人 TODO 首页：

```sh
contract-cli api call --channel web --profile '<本次已核profile>' --as user --raw GET '/open-web/web/clm/task/list/v2' --query 'taskTypeCode=0' --query 'pageIndex=0' --query 'pageSize=100' --query 'desc=true' --query 'limitBefore=0' --query 'limitAfter=99'
```

Agent 依次读取后续页面，每次仍是同一条 CLI 查询，按以下规则更新参数和保存判断依据：

1. `taskTypeCode` 始终 0，`pageIndex` 从 0 递增，`desc=true`、`limitBefore=0`；首次 `pageSize` 默认 100，可在 1–100 内取本次明确值，`limitAfter` 始终为本次请求 `pageSize` 减 1。
2. 不增加关键词、`businessTypeCodes`、`nodeTypes`、`taskInstanceId`、`noticeId`、`onlyUrge`、`read`、租户或人员覆盖参数。旧筛选会进入另一种截断链，不能用其结果证明已查全。
3. Web 业务成功后，`data.content` 应为数组；`data.pageSize` 必须是正整数且不大于请求值，`data.totalElements` 必须是有效非负整数。如果返回 `pageIndex`，应等于请求页码。当页条数不得大于实际页大小，已观察的去重任务数不得超过已核总数。列表项应为 TODO，含有效任务及业务 ID；字段缺失不能猜测。
4. 服务端可能在首页缩小 `data.pageSize`；后续请求采用该实际值，并同步修改 `limitAfter`。后续页大小或总数再次变化时停止扫描，说明结果不完整，不拼接为完整清单。
5. 在已获权 `content[]` 内，按目标 `businessId` 与 `businessTypeCode` 同时精确匹配；保留 `processTaskInstanceId`、`processInstanceId`、任务节点 ID/类型/名称及合同状态。以完整任务 ID 去重，检测重复页面和重复任务；异常时不宣称查全。
6. 当前页读完后，以“已读页数 × 实际页大小已达到 `totalElements`”判断结束；`totalElements` 是本人原始任务总数，不是匹配合同数。关联数据可能使某页短或空，不能据此提前结束，也不要求 `hasNext` 或 `totalPages` 存在。
7. 默认本次最多读 100 页，这是扫描预算，不是系统总量。达到预算、请求失败、计数缺失/变化、重复或页结构异常时，保留已取得候选并说明不完整（INCOMPLETE），不能说没有任务。用户需要更多结果时在同一已授权范围确定新的扫描预算；不改查其他用户或全租户。

对每个匹配项，或前序实际协商返回的 `todoTaskId`，再分别执行：

```sh
contract-cli api call --channel web --profile '<本次已核profile>' --as user --raw GET '/open-web/web/clm/task/info' --query 'taskInstanceId=<已核processTaskInstanceId或todoTaskId>'
```

唯一 query 为 `taskInstanceId`，保留原完整字符串；**始终不传 `noticeId`**。该分支按当前用户实际任务处理人鉴权，列表命中不能替代本次鉴权。

Web 业务成功后，核对 `data.processTaskInstanceId` 与请求任务 ID 相同，`businessId`、`businessTypeCode` 与目标都一致。`taskTypeCode=0` 只表示当前未结束的 TODO，不等于原始 RUNNING 状态、原合同最新流程或 RESUBMIT 权限；流程 ID 也只是任务的关联流程。

| 实际结果 | 可作的解释与处理 |
| --- | --- |
| 对象关联一致，仍为 TODO | 当前待办已核实（CURRENT_TODO），可给真实任务承载页 |
| 对象关联一致，已不为 TODO | 已不在当前待办（NOT_CURRENT_TODO），不继续提供旧待办资格 |
| `data=null` | 本次未返回任务（NOT_RETURNED），不扩展为合同不存在或没有权限 |
| 请求失败、对象关联矛盾或必要字段缺失 | 待核实（UNKNOWN），保留具体原因，不放入已核当前任务清单 |

复核通过的当前任务与其他待核实候选分别展示。多个有效任务保留节点区分，不自动选最新一个。只有本人 TODO 分页完整且相应候选均已复核，才可说明本次未发现匹配的当前任务；这仍不代表用户没有任何合同权限。分页查询是本次观察，不是跨页并发一致快照。

没有已核完整重提条件时，链接文字为“打开当前任务”。不使用 `permission/business/list` 替代任务鉴权，也不额外读取审批记录、完整合同或 `composition` 推测重提资格。

## 7. 本人模板草稿的路由配置

```sh
contract-cli api call --channel web --profile '<本次已核profile>' --as user --raw GET '/open-web/web/clm/sso/getUser'
```

没有 query 或 body。Web 业务成功后，读取 `data.systemConfigVO.isOneStepTemplate` 的明确布尔值；`employeeId`、`tenantId` 如实际返回非空值，必须与当前 USER 一致。开关缺失、null、类型错误或返回的身份冲突，均按“未核实”（UNKNOWN）处理，不能当 false；不把响应未提供身份字段本身当成身份冲突。

只在本人模板草稿关联协商、且草稿选择器确实需要该配置时调用。此查询包含用户/租户元信息和缓存预热，不修改合同；不读取或输出凭据，不改用更宽的 `collectUserDetail`。

这是租户的草稿路由配置，不能替代协商对象的二合一标记，也不是非负责人的提交许可。协商完成后的“进入审批”分支不需要这个开关，不能仅因为合同来自模板就额外调用。
