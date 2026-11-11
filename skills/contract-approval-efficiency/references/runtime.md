# 运行能力与采集规则

## 通用调用协议

CLI 调用形式：

```text
contract-cli api call --channel web --profile <当前任务所用 profile> --as user --output json <METHOD> <open_web_path>
```

按实际帮助传递 query、header 和 JSON body。不得把服务地址、认证令牌或 Relay 地址作为业务参数。所有时间相关请求携带 `Timezone-Offset: -480`；若用户业务时区不是 Asia/Shanghai，必须先确认接口是否支持相应偏移，未确认时停止而不是套用默认值。

同时检查调用器退出状态、HTTP 状态和业务 envelope：

- CLM 目录、合同、流程和表单接口：`success=true` 且 `code=200`。
- `getVisibleTasks`：按下文任务配方校验 `code=0` 和 `data.task_list`，独立于 CLM 页面接口的成功码。

退出成功或 HTTP 200 不能单独代表业务成功。

页面 `--channel web` 能力只覆盖其帮助或协议明确允许的页面业务接口。不得自行添加 `X-Gate-Auth` 等调用器禁止的身份头，也不得假设网关会把页面路径重写为内部 Open Platform 路径。

## 人员部门和角色模式的能力与权限判定

用本次首个真实目标人员的首屏请求预检新路由，可复用成功响应作为该分页流首屏；首屏非空时确认可继续分页。按实际结果分流：

- **通道或路由缺失**（例如调用器禁用通用入口，或明确返回路由未启用/404）、响应协议不兼容或 ID 无法保真：返回 `CAPABILITY_MISSING`。通用调用帮助只能证明 CLI 有入口；当前环境仍须核实新路由。
- **登录未授权或已失效**（如任务接口业务码 `110001`）：按当前 USER 调用器的授权流程恢复，使用同一环境、租户和用户续办。
- **明确拒绝查询某员工**（`getVisibleTasks` 业务码 `110004`）：将该员工标记为未覆盖，停止其两类分页；丢弃该员工在拒绝前已取得的任务及归因关系。其他目标员工仍用新接口独立校验，首个员工被拒绝不代表接口不可用。混合结果为 `partial`，该员工及受影响的汇总指标显示 `—`。全部目标都被拒绝时返回 `permission_denied`，不生成零值报表。
- **网关拒绝当前 USER 使用路由**（如有效授权下明确被准入或 scope 拒绝）：返回 `permission_denied`，提示相应授权范围，不归咎于某员工的部门归属。无法辨明失败原因时记录覆盖缺口，不猜测为角色拒绝或空结果。
- **目标目录已变化**（`111714`）：按原业务输入重新解析该员工一次，仍无法确定时保留缺口；不得把该码当作空任务。参数错误（`111706`）核对映射后处理。权限来源或查询服务异常（如 `110002`、`110500`）只记录覆盖缺口，不当作权限拒绝或空页。

禁止调用旧 `getTasks` 补数或预检，禁止使用旧接口的任务缓存和续页游标。仅查本人的结构化命令或 MCP Tool 不能代替此配方，也不得切换 APP、管理员或其他用户。

完整空名单例外：若[发现流程](discovery.md)已确认部门或角色名单确实为空，可以输出对应的空名单结果，不需要任务发现探针。

## 数据范围与权限闸门

接口依次约束以下范围：

- [人员、部门和角色目录](discovery.md)按当前 USER 的目录浏览或后台角色管理权限解析目标。目录命中、查询人所属部门、合同所属部门均不作为任务查询权限依据。
- `getVisibleTasks` 根据当前 USER 的启用角色成员配置判定负责部门，多角色取并集，指定部门包含子部门；显式“全部”才覆盖租户全部部门。目标员工按当前部门校验，多部门任一命中即可。无有效负责范围、目标无部门、目标超出范围或权限源失败时以服务端拒绝为准。校验通过后可查询该人员的待办和历史已办（含调岗前任务），不按任务或合同的部门再筛一次。
- `contract/info` 和 `process/records` 按当前 USER 校验合同读取权限。最终统计、在途、个人表、部门总览和用户明细只接纳当前 USER 成功读取 `process/records` 的实例。

对候选实例调用 `process/records` 返回 110004、无权读取或业务失败时：

1. 不计完成任务、在途任务、合同数或用时；
2. 不把失败解释成零；
3. 报告标记 `record_collection_status=partial`；
4. 不导出该实例的任务元数据、人员候选或合同 ID，只在内部诊断保留脱敏对象标识、失败阶段和安全错误码。

报告范围按[输出规范](output.md)说明任务权限和流程读取范围。

## 必需配方

| 配方 | 方法和 Open Web 路径 | 关键输入 | 用途 |
|---|---|---|---|
| 人员任务 | `GET /open-web/web/clm-openapi/v1/tasks/getVisibleTasks` | 见[人员任务配方](#人员任务-getvisibletasks) | 按当前 USER 的角色负责部门权限发现已办、待办 |
| 流程记录 | `GET /open-web/web/clm/workflow/runtime/process/records` | 必传真实 `processInstanceId`；已知时附 `businessId`、`businessTypeCode` | 取得 `done`、`todo`、任务动作、人员和时间 |
| 普通合同详情 | `GET /open-web/web/clm/contract/info` | `contractId`、`tianyancha=false`、`generateContractName=false` | 补全普通合同实例和元数据 |
| 轻量合同详情 | `GET /open-web/web/clm/contract/briefInfo` | `contractId` | 普通详情 110002 重试耗尽后的可选元数据恢复；可能返回空 |
| 历史节点人员 | `GET /open-web/web/clm/workflow/runtime/listAllApprovalEmployees` | `businessId`、`businessTypeCode`、`processInstanceId`、`filterRobot=false` | 审计历史节点人员；不能单独证明唯一归因 |
| 历史表单定义 | `GET /open-web/web/clm/form/getFormDefinition` | `formId` | 仅当前 USER 本身具备管理员权限时用于可选审计 |

普通合同类型接受集合 `0/1/2/3/9/11/12/16` 仅适用于响应字段 `businessTypeCode`。`contractTypeCode` 是另一套枚举，不得拿来选择合同详情路由或映射到上述集合。实际运行时把“已经真实出现并成功读取的类型”和“接口规则允许但本次没有样本的类型”分开记录。合同组 `businessTypeCode=10/17` 只有在宿主已有经过验证的合同组详情配方时处理；缺少该能力时把相应对象标为未覆盖，不能当作普通合同读取。

## 人员任务 getVisibleTasks

本 Skill 通过上表的 Open Web 路径以当前 USER 调用。使用 `Timezone-Offset: -480`；CLI 用 `--raw` 保留 JSON 原文，再无损解析。目标 `assignee_user_id` 仅用于筛选，调用人由授权上下文确定。平台的另一个入口 `GET /open-apis/contract/v1/tasks/getVisibleTasks` 有独立个人授权准入（Device 需允许该 GET 路由并具备 `contract:full`）；本 Skill 的 Open Web 调用器不将该路径作为替代入口。

| Query 参数 | 来源与发送规则 |
|---|---|
| `assignee_user_id` | 必填，目录返回的目标员工数字 `larkId`，按原始十进制文本发送。不能使用飞书 `open_id` 或短 ID。 |
| `task_status` | 必填，`0` 待办、`1` 已办。每人每个状态独立分页。 |
| `sort_type` | 必填，接口支持 `ASC`/`DESC`；本 Skill 固定 `DESC`。 |
| `last_cursor`、`last_sort_value` | 首屏省略；续页成对携带。取上一页最后一条的 `cursor` 和原始毫秒排序值（待办 `create_time`、已办 `end_time`）。 |
| `cursor_size` | 可选正整数，仅续页双游标齐备时生效；首屏省略，使用服务端默认大小。 |

已办首屏 CLI 示例：

```sh
contract-cli api call --channel web --profile '<当前 profile>' --as user --raw \
  GET /open-web/web/clm-openapi/v1/tasks/getVisibleTasks \
  --header 'Timezone-Offset: -480' --query 'assignee_user_id=<目标 larkId>' \
  --query 'task_status=1' --query 'sort_type=DESC'
```

传输成功且业务 `code=0`、`data.task_list` 确为数组才接受；若 `success` 明确为 false，按失败处理。响应沿用 `contract_id`、`assignee_ids`、`task_instance_id`、`process_instance_id`、`task_status`、`create_time`、`end_time`、`update_time`、`latest_process_event_sequence_id` 和 `cursor`。可空字段可能省略；没有 `next_cursor` 或 `task_detail`。用流程 ID 和任务 ID 连接办理记录。

继续读取直到成功返回 `task_list: []`；短页继续。游标停滞、页重复、缺少必需续页值或业务失败时记录该流缺口，不当作结束。可恢复的分页只使用同一新接口、同一 USER 和筛选条件下保存的游标。历史已办与在途按[统计口径](metrics.md)筛选。

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
- 权限不足（包括明确的员工范围拒绝和流程详情 110004）；
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
- 此模式不调用人员任务接口反查人员，不构造 `discovered_for_employee_ids`，也不使用部门零行规则。

`process/records` 即使返回 `success=true, code=200`，`data=null` 也不是空流程成功：

- 明确合同或流程查询时，若对象没有其他有效存在证据，返回 `not_found`；若合同详情已证明对象存在，则标记 `record_collection_status=partial`。
- 人员/部门/角色候选实例出现 `data=null` 时标记 `records_data_null` 和 `partial`，排除该实例的完成、在途、合同数和用时。
- 所有场景都不得把 `data=null` 转成空数组或真实零。

## 多人合并

用新接口查询每个目标员工的两类任务，再在内存或宿主的结构化数据能力中合并：

- `employees`、`approver_ids` 按 `employeeId` 取并集；同 ID 显示名冲突进入诊断。
- 任务唯一键是 `(process_instance_id, task_instance_id)`；`discovered_for_employee_ids` 只合并各目标人员通过权限校验的独立查询流所命中的人员，不合并 `getVisibleTasks.assignee_ids`。共享任务经其他目标员工授权命中时，可保留合格的任务明细，但去掉被拒员工的姓名和 ID，不把其记录计入个人统计或总览。拒绝人员的个人表行仅按已请求的人员范围显示 `—`。
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

`getVisibleTasks.contract_id`、`assignee_ids`、游标以及其他数值 ID 可能超过 JavaScript 安全整数范围。调用器必须保留原始 JSON 数字词法，或使用任意精度整数/字符串解析；不得先经过普通浮点 JSON 解析再转字符串。

业务归属优先采用 `process/records.businessId` 的字符串值。若当前调用器无法保证长 ID 无损，相关分页流返回 `CAPABILITY_MISSING` 或 `partial`，不能继续用已舍入 ID 查询。报告和 Excel/CSV 一律把合同、人员、流程、任务和游标 ID 当文本保存。

## 数据最小化

只保存统计所需的人员 ID/姓名/当前部门、合同名称/编号/ID/类型、流程和任务 ID、节点、动作、状态及起止时间。任务候选在通过 `process/records` 权限校验前只暂存完成下一步所需的最少字段；读取失败后从用户明细移除，仅保留脱敏诊断。不要保存合同正文、附件、联系方式、认证信息或与统计无关的表单内容。临时原始响应不进入 Skill 包。
