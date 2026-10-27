# 运行能力与采集规则

## 通用调用协议

CLI 调用形式：

```text
contract-cli api call --channel web --profile <当前任务所用 profile> --as user --output json <METHOD> <open_web_path>
```

按实际帮助传递 query、header 和 JSON body。不得把服务地址、认证令牌或 Relay 地址作为业务参数。所有时间相关请求携带 `Timezone-Offset: -480`；若用户业务时区不是 Asia/Shanghai，必须先确认接口是否支持相应偏移，未确认时停止而不是套用默认值。

同时检查调用器退出状态、HTTP 状态和业务 envelope：

- CLM 目录、合同、流程和表单接口：`success=true` 且 `code=200`。
- 只有另行核实的 Open Platform 任务提供方才按其 schema 检查成功条件；不能从页面通道规则推导。

退出成功或 HTTP 200 不能单独代表业务成功。

页面 `--channel web` 能力只覆盖其帮助或协议明确允许的页面业务接口。不得自行添加 `X-Gate-Auth` 等调用器禁止的身份头，也不得假设网关会把页面路径重写为内部 Open Platform 路径。

## 人员和部门模式的能力预检

人员/部门模式需要一个能按真实目标人员标识查询其已办、待办并完整分页的 USER 能力。以下入口均不能自动满足：

- 只返回当前个人令牌对应人员任务的 MCP Tool；
- `contract approval task list` 等只查询当前登录人的结构化命令；
- 仅因路径前缀符合 `/open-web/web/**` 就假定可达的 Open Platform 路由；
- 需要显式传递 `X-Gate-Auth` 或其他调用器禁止身份头的请求。

解析出首个真实目标人员后，在批量采集前执行一次只读探针。历史候选地址 `/open-web/web/clm-openapi/v1/tasks/getTasks` 仅作为兼容性探针，不是已承诺的页面配方。能力判定必须同时满足：

1. 实际帮助、Tool schema 或可信提供方协议明确支持按任意目标人员过滤，而非只支持当前用户。
2. 使用本次真实目标人员和一个任务状态发起探针，路由与业务 envelope 成功；不添加禁止的身份头。
3. 提供方明确给出完整分页字段、排序键和终止条件；若探针返回续页信息，至少续读一页并确认游标前进且无整页重复。空首屏本身不能证明分页协议完整。
4. 响应中的合同、人员、任务和游标 ID 能无损保存。

任一条件不满足，或返回 404、401、110004、路由不支持、只允许当前用户，立即返回 `CAPABILITY_MISSING`。不得进入全量分页、不得生成个人或部门统计，也不得改用当前用户任务命令、数据库、日志、APP 或管理员身份。授权过期可按调用器协议恢复后重试探针；恢复后仍失败则停止。

完整空部门例外：若部门及子部门成员分页已按下文完整结束，且人员清单确实为空，可以输出空部门结果，不需要任务发现探针。

## 数据范围与权限闸门

接口的实际范围分两层：

- 人员候选、部门候选、部门成员、`getTasks` 任务元数据和历史流程实例可能是租户级数据，它们只用于发现候选、分页和诊断，不能证明当前调用人有权读取相关合同。
- `contract/info` 和 `process/records` 按当前 USER 校验合同读取权限。最终统计、在途、个人表、部门总览和用户明细只接纳当前 USER 成功读取 `process/records` 的实例。

对候选实例调用 `process/records` 返回 110004、无权读取或业务失败时：

1. 不计完成任务、在途任务、合同数或用时；
2. 不把失败解释成零；
3. 报告标记 `record_collection_status=partial`；
4. 不导出该实例的租户级任务元数据、人员候选或合同 ID，只在内部诊断保留脱敏对象标识、失败阶段和安全错误码。

因此报告范围必须写成“目录及任务候选来自当前租户；指标仅包含当前 USER 成功读取流程记录的合同”，不能笼统写成所有发现数据均为当前 USER 可见。

## 必需配方

| 配方 | 方法和 Open Web 路径 | 关键输入 | 用途 |
|---|---|---|---|
| 人员候选 | `GET /open-web/web/clm/employee/findEmployeesByKeyword` | `keywords`、`filterCurrentEmployee=false`、`noFilterDeletedEmployee=true` | 返回真实 `employeeId`、`larkId`、姓名、状态和当前部门 |
| 部门候选 | `GET /open-web/web/clm/department/findDepartmentsByKeyword` | `keyword`、`requireAll=false`、`showAllStatus=true` | 返回真实部门 ID 和已发现子部门 |
| 部门成员 | `GET /open-web/web/clm/department/organization/list` | `parentDepartmentId`、`employeePageIndex`、`employeePageSize=50` | 从第 1 页读取每个部门直属成员和子部门 |
| 流程记录 | `GET /open-web/web/clm/workflow/runtime/process/records` | 必传真实 `processInstanceId`；已知时附 `businessId`、`businessTypeCode` | 取得 `done`、`todo`、任务动作、人员和时间 |
| 普通合同详情 | `GET /open-web/web/clm/contract/info` | `contractId`、`tianyancha=false`、`generateContractName=false` | 补全普通合同实例和元数据 |
| 轻量合同详情 | `GET /open-web/web/clm/contract/briefInfo` | `contractId` | 普通详情 110002 重试耗尽后的可选元数据恢复；可能返回空 |
| 历史节点人员 | `GET /open-web/web/clm/workflow/runtime/listAllApprovalEmployees` | `businessId`、`businessTypeCode`、`processInstanceId`、`filterRobot=false` | 审计历史节点人员；不能单独证明唯一归因 |
| 历史表单定义 | `GET /open-web/web/clm/form/getFormDefinition` | `formId` | 仅当前 USER 本身具备管理员权限时用于可选审计 |

普通合同类型接受集合 `0/1/2/3/9/11/12/16` 仅适用于响应字段 `businessTypeCode`。`contractTypeCode` 是另一套枚举，不得拿来选择合同详情路由或映射到上述集合。实际运行时把“已经真实出现并成功读取的类型”和“接口规则允许但本次没有样本的类型”分开记录。合同组 `businessTypeCode=10/17` 只有在宿主已有经过验证的合同组详情配方时处理；缺少该能力时把相应对象标为未覆盖，不能当作普通合同读取。

## 任务分页

任意人员任务能力预检通过后，对每名人员和每个任务状态分别建立分页流：

1. 首次请求按提供方协议省略续页字段；首屏大小由提供方决定，不能把后续页大小参数解释为首屏上限。
2. 后续请求原样使用提供方确认的续页字段和排序值。只有 schema、帮助或真实两页验证明确说明时，才能采用 done=`end_time`、todo=`create_time`；否则返回 `CAPABILITY_MISSING`，不能猜测。
3. 保持游标和排序值的长整数精度，验证游标前进且无整页重复。
4. 仅以提供方确认的终止条件结束。短页不是结束条件；空页与 `hasMore` 等状态冲突时标记分页异常。
5. 页内容重复、游标不前进、缺少必需续页字段或业务失败时停止该流并记录续读位置。

部门成员按每个部门独立从 `employeePageIndex=1` 开始，固定 `employeePageSize=50`。每个部门仅在成功响应明确返回 `hasMore=false` 时完成；`hasMore=true` 时页码加一继续。`hasMore=true` 但员工页为空、成员集合无进展、页重复或缺少下一页所需字段时标记分页异常，`roster_complete=false`。根部门直属成员为空时仍继续已发现的子部门；部门 ID 和人员 ID 去重并防止层级循环。只有所有已发现部门均以 `hasMore=false` 结束、子部门队列耗尽且无分页异常时，才设置 `roster_complete=true`。

## 统一有界重试

所有只读配方共用同一规则，每个请求最多三次。第二次和第三次前采用短暂递增退避，例如约 1.5 秒、3 秒。

可重试：

- 超时、连接中断等传输错误；
- 429、503；
- 只读详情或元数据请求返回通用系统错误 110002；重试耗尽后的恢复规则见下文；
- 明显截断或无法解析的 JSON；
- 只读调用非零退出且无法判断为确定性业务错误。

不可重试：

- 未授权或授权过期；先走调用器授权恢复，再续办原请求；
- 110004 权限不足；
- 参数错误、对象不存在、schema 不兼容；
- 已知确定性业务拒绝。

诊断至少记录配方、对象标识、人员和任务状态分页流、失败阶段、尝试次数、最后安全错误码及续读位置。重试成功的请求视为成功，不把整份报告标为 `partial`。

## 110002 与元数据恢复

110002 表示通用系统错误，不是合同类型或对象不存在信号。需要普通合同元数据时适用：

1. 调用 `contract/info`。
2. 110002 先执行适用的只读有界重试，不能立即改走其他接口。
3. 重试耗尽且对象已由可信业务类型字段确认为普通合同时，才调用一次 `contract/briefInfo` 尝试恢复可选元数据。
4. 核对返回 `contractId` 与请求一致，再采用真实名称、编号、类型和实例 ID；返回 `null`、ID 不一致或失败时保留缺口。

110004 不得回退，不得切换身份或接口绕过权限。`briefInfo` 成功不能替代 `process/records` 的当前 USER 权限校验，也不能把无权读取的对象升级为可统计。

## 合同和流程实例范围

- 普通合同 ID：用 `contract/info` 取得真实实例，再以 `process/records` 为任务事实源。
- 流程实例 ID：直接调用 `process/records`；实例不匹配或无权读取时停止该对象。
- 合同组 `10/17`：只有宿主存在已验证合同组详情配方时处理，否则返回范围未覆盖。
- 处理人清单：对可读流程的 `done[].records[].assigneeEmployees` 和 `todo[].records[].assigneeEmployees` 按 `employeeId` 去重。done 唯一人员视为实际处理人；todo 人员视为当前候选/经办人。
- 此模式不调用 `getTasks` 反查人员，不构造 `discovered_for_employee_ids`，也不使用部门零行规则。

`process/records` 即使返回 `success=true, code=200`，`data=null` 也不是空流程成功：

- 明确合同或流程查询时，若对象没有其他有效存在证据，返回 `not_found`；若合同详情已证明对象存在，则标记 `record_collection_status=partial`。
- 人员/部门候选实例出现 `data=null` 时标记 `records_data_null` 和 `partial`，排除该实例的完成、在途、合同数和用时。
- 所有场景都不得把 `data=null` 转成空数组或真实零。

## 多人合并

只有任意人员任务能力预检通过时，每个人才执行独立目录解析和两类任务分页，再在内存或宿主的结构化数据能力中合并：

- `employees`、`approver_ids` 按 `employeeId` 取并集；同 ID 显示名冲突进入诊断。
- 任务唯一键是 `(process_instance_id, task_instance_id)`；`discovered_for_employee_ids` 只合并各目标人员独立查询流的命中人员，不合并 `getTasks.assignee_ids`。
- 同一任务的状态、时间、业务 ID、业务类型、节点 ID 冲突时，不静默挑选，标记 `task_conflict` 并排除汇总。
- 实例按 `processInstanceId` 合并，`source_task_ids`、`matched_employee_ids` 取并集；业务归属冲突时标记 `instance_conflict`。
- 所有目标人员的目录清单、已办和待办分页均完成，人员发现才是 `complete`。

## 三类完整性状态

结构化诊断分别维护：

- `record_collection_status`：任务分页和流程记录是否完整。
- `business_metadata_status`：合同名称、编号和业务键是否完整。
- `attribution_evidence_status`：任务中心或历史单人证据是否足以归因。

任一状态使用 `complete`、`partial`、`not_applicable`。报告总状态由会影响本次统计的状态决定；可选名称缺失只影响明细可读性，不应覆盖核心统计状态。

历史表单定义属于管理员能力。普通 USER 不调用该配方；任务中心和可读流程记录已能唯一归因时状态仍可为 `complete`，只有确实依赖该历史证据且无法取得时才把 `attribution_evidence_status` 标为 `partial`。

## 长 ID

`getTasks.contract_id`、`assignee_ids`、游标以及其他数值 ID 可能超过 JavaScript 安全整数范围。调用器必须保留原始 JSON 数字词法，或使用任意精度整数/字符串解析；不得先经过普通浮点 JSON 解析再转字符串。

业务归属优先采用 `process/records.businessId` 的字符串值。若当前调用器无法保证长 ID 无损，相关分页流返回 `CAPABILITY_MISSING` 或 `partial`，不能继续用已舍入 ID 查询。报告和 Excel/CSV 一律把合同、人员、流程、任务和游标 ID 当文本保存。

## 数据最小化

只保存统计所需的人员 ID/姓名/当前部门、合同名称/编号/ID/类型、流程和任务 ID、节点、动作、状态及起止时间。租户级候选在通过 `process/records` 权限闸门前只暂存完成下一步所需的最小 ID；读取失败后立即从用户明细移除，仅保留脱敏诊断。不要保存合同正文、附件、联系方式、认证信息或与统计无关的表单内容。临时原始响应不进入 Skill 包。
