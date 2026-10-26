# 运行能力与采集规则

## 通用调用协议

CLI 调用形式：

```text
contract-cli api call --channel web --profile <当前任务所用 profile> --as user --output json <METHOD> <open_web_path>
```

按实际帮助传递 query、header 和 JSON body。不得把服务地址、认证令牌或 Relay 地址作为业务参数。所有时间相关请求携带 `Timezone-Offset: -480`；若用户业务时区不是 Asia/Shanghai，必须先确认接口是否支持相应偏移，未确认时停止而不是套用默认值。

同时检查调用器退出状态、HTTP 状态和业务 envelope：

- CLM 目录、合同、流程和表单接口：`success=true` 且 `code=200`。
- CLM OpenAPI 任务和历史实例接口：`success=true` 且 `code=0`。

退出成功或 HTTP 200 不能单独代表业务成功。

## 必需配方

| 配方 | 方法和 Open Web 路径 | 关键输入 | 用途 |
|---|---|---|---|
| 人员候选 | `GET /open-web/web/clm/employee/findEmployeesByKeyword` | `keywords`、`filterCurrentEmployee=false`、`noFilterDeletedEmployee=true` | 返回真实 `employeeId`、`larkId`、姓名、状态和当前部门 |
| 部门候选 | `GET /open-web/web/clm/department/findDepartmentsByKeyword` | `keyword`、`requireAll=false`、`showAllStatus=true` | 返回真实部门 ID 和已发现子部门 |
| 部门成员 | `GET /open-web/web/clm/department/organization/list` | `parentDepartmentId`、`employeePageIndex`、`employeePageSize=50` | 从第 1 页读取每个部门直属成员和子部门 |
| 人员原始任务 | `GET /open-web/web/clm-openapi/v1/tasks/getTasks` | `assignee_user_id=<真实 larkId>`、`task_status=0或1`、`sort_type=DESC`、游标参数 | 发现任意目标人员当前权限可见的已办和待办 |
| 流程记录 | `GET /open-web/web/clm/workflow/runtime/process/records` | 必传真实 `processInstanceId`；已知时附 `businessId`、`businessTypeCode` | 取得 `done`、`todo`、任务动作、人员和时间 |
| 普通合同详情 | `GET /open-web/web/clm/contract/info` | `contractId`、`tianyancha=false`、`generateContractName=false` | 补全普通合同实例和元数据 |
| 轻量合同详情 | `GET /open-web/web/clm/contract/briefInfo` | `contractId` | 仅供普通详情 110002 的一次回退 |
| 历史节点人员 | `GET /open-web/web/clm/workflow/runtime/listAllApprovalEmployees` | `businessId`、`businessTypeCode`、`processInstanceId`、`filterRobot=false` | 审计历史节点人员；不能单独证明唯一归因 |
| 历史流程实例 | `GET /open-web/web/clm-openapi/v1/process_instances/{process_instance_id}` | 路径中的真实实例 ID | 核对历史 definition、任务和动作 |
| 历史表单定义 | `GET /open-web/web/clm/form/getFormDefinition` | `formId` | 在 definition 严格一致时核对固定人员配置 |

普通合同类型接受集合为 `0/1/2/3/9/11/12/16`。实际运行时把“已经真实出现并成功读取的类型”和“接口规则允许但本次没有样本的类型”分开记录。合同组类型 `10/17` 只有在宿主已有经过验证的合同组详情配方时处理；缺少该能力时把相应对象标为未覆盖，不能当作普通合同读取。

## 任务分页

对每名人员和每个任务状态分别建立分页流：

1. 首次请求不传 `last_sort_value` 和 `last_cursor`；`cursor_size` 只控制单页大小，不限制总量。
2. 已办使用上一页最后任务的 `end_time` 作为 `last_sort_value`；待办使用 `create_time`。
3. 同时原样传回最后任务的整数 `cursor`，保持长整数精度。
4. 继续读取到空页或协议明确的终点。短页不是结束条件。
5. 页内容重复、最后游标不前进、缺少续页字段或业务失败时停止该流并记录续读位置。

部门成员按每个部门独立从 `employeePageIndex=1` 开始，固定 `employeePageSize=50`。根部门直属成员为空时仍继续已发现的子部门。部门 ID 和人员 ID 去重并防止层级循环。

## 统一有界重试

所有只读配方共用同一规则，每个请求最多三次。第二次和第三次前采用短暂递增退避，例如约 1.5 秒、3 秒。

可重试：

- 超时、连接中断等传输错误；
- 429、503；
- 明显截断或无法解析的 JSON；
- 只读调用非零退出且无法判断为确定性业务错误。

不可重试：

- 未授权或授权过期；先走调用器授权恢复，再续办原请求；
- 110004 权限不足；
- 参数错误、对象不存在、schema 不兼容；
- 已知确定性业务拒绝。

诊断至少记录配方、对象标识、人员和任务状态分页流、失败阶段、尝试次数、最后安全错误码及续读位置。重试成功的请求视为成功，不把整份报告标为 `partial`。

## 110002 元数据回退

无论对象来自明确合同范围还是人员原始任务，只要需要普通合同元数据，都适用同一规则：

1. 调用 `contract/info`。
2. 仅当业务码明确为 `110002` 时调用一次 `contract/briefInfo`。
3. 核对返回 `contractId` 与请求一致，再采用真实名称、编号、类型和实例 ID。
4. 回退为空、ID 不一致或失败时保留缺口。

110004 不得回退，不得切换身份或接口绕过权限。元数据补齐成功后，原 110002 不应让核心记录采集状态变成 `partial`。

## 多人合并

每个人都要先完成独立的目录解析和两类任务分页，再在内存或宿主的结构化数据能力中合并：

- `employees`、`approver_ids` 按 `employeeId` 取并集；同 ID 显示名冲突进入诊断。
- 任务唯一键是 `(process_instance_id, task_instance_id)`；`discovered_for_employee_ids` 取并集。
- 同一任务的状态、时间、业务 ID、业务类型、节点 ID 冲突时，不静默挑选，标记 `task_conflict` 并排除汇总。
- 实例按 `processInstanceId` 合并，`source_task_ids`、`matched_employee_ids` 取并集；业务归属冲突时标记 `instance_conflict`。
- 所有目标人员的目录清单、已办和待办分页均完成，人员发现才是 `complete`。

## 三类完整性状态

结构化诊断分别维护：

- `record_collection_status`：任务分页和流程记录是否完整。
- `business_metadata_status`：合同名称、编号和业务键是否完整。
- `attribution_evidence_status`：任务中心或历史单人证据是否足以归因。

任一状态使用 `complete`、`partial`、`not_applicable`。报告总状态由会影响本次统计的状态决定；可选名称缺失只影响明细可读性，不应覆盖核心统计状态。

## 数据最小化

只保存统计所需的人员 ID/姓名/当前部门、合同名称/编号/ID/类型、流程和任务 ID、节点、动作、状态及起止时间。不要保存合同正文、附件、联系方式、认证信息或与统计无关的表单内容。临时原始响应不进入 Skill 包。
