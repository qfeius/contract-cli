# 标品页面接口契约

本页是本 Skill 唯一的技术事实来源。所有调用均通过：

```bash
contract-cli api call --channel web --profile <profile> --as user --output json <METHOD> <PATH>
```

只允许下表路径。禁止接收用户给出的 URL/path，禁止自行发现、猜测或扩展接口，禁止 curl、直接 HTTP 和随包/临时/内联业务运行器。每次直接执行一条 CLI 命令，不附加循环、脚本、管道、重定向或自建业务中转文件；完整结果的宿主只读展开方式见[可移植运行契约](portable-runtime.md)。所有 ID 按原始字符串传递；CLI 非零退出、非法 JSON、HTTP 失败、`success=false` 或非成功业务码都不是空结果或成功。

## 允许的读取接口

| 用途 | Method 与 path | 参数 | 完成条件 |
| --- | --- | --- | --- |
| 任务列表 | `GET /open-web/web/clm/task/list/v3` | query：`taskTypeCode`（todo=0、done=1、notice=2）、`pageIndex`、`pageSize`、`desc=true` | 返回本页任务与 `totalElements` 等分页信息 |
| 任务详情 | `GET /open-web/web/clm/composition/contractAndTask` | 优先 query：`taskInstanceId`；另传 `batchContractChildrenPageIndex=0`、`batchContractChildrenPageSize=10`、`tianyancha=false`、`smart=false` | `data.task.taskMeta`、`permissions`、`processRecords` 均可读取 |
| 人员搜索 | `GET /open-web/web/clm/employee/findEmployeesByKeyword` | query：`keywords`、`filterCurrentEmployee=true` | 返回全部当前可见候选 |
| 人员复核 | `GET /open-web/web/clm/employee/getEmployeesByIds` | query：`employeeIds=<逗号分隔>`、`extraDepartmentInfo=true` | 所有目标 ID 均存在且可选择 |
| 分类树 | `GET /open-web/web/clm/category/findAllWithinAuthority` | 无 | 用于名称、编号、缩写、父子关系和启用状态 |

任务列表示例：

```bash
contract-cli api call --channel web --profile <profile> --as user --output json \
  GET /open-web/web/clm/task/list/v3 \
  --query taskTypeCode=0 --query pageIndex=0 --query pageSize=100 --query desc=true
```

任务详情示例：

```bash
contract-cli api call --channel web --profile <profile> --as user --output json \
  GET /open-web/web/clm/composition/contractAndTask \
  --query taskInstanceId=<任务实例ID> \
  --query batchContractChildrenPageIndex=0 \
  --query batchContractChildrenPageSize=10 \
  --query tianyancha=false --query smart=false
```

通知列表同时有 `processTaskInstanceId` 和 `processNoticeTaskInstanceId` 时，读取详情仍用前者作为 `taskInstanceId`；仅在没有底层任务实例且页面明确要求时才用 `noticeId`。详情查询返回“任务不存在”等业务错误时，只表示该标识未命中；在允许的候选范围内继续匹配，不能吞掉鉴权、登录和网络错误。

## 任务完整分页

从 `pageIndex=0`、`pageSize=100` 开始。查询本人全部任务时不传业务类型过滤；v3 列表覆盖当前页面可见的全部业务类型，包含普通合同和合同组。

1. 每页按 `processTaskInstanceId` 去重；知会仅在没有该值时用 `processNoticeTaskInstanceId`。
2. 累计唯一任务数达到 `totalElements` 时结束；没有总数时，仅当本页少于 `pageSize` 时结束。
3. 下一页页码必须严格加一。页码不前进、任务集合整页重复、关键分页字段缺失或累计数量与总数不一致时，标记查询不完整，不得说“全部”。
4. 旧的 `workflow/runtime/task/todo|done|getNotices` 存在类型过滤和分页语义缺陷，不用于“全部待办”或批量定位。

列表至少保留：业务对象 ID、业务类型、合同/合同组名称与编号、`contractCategoryId`、`contractCategoryName`、`contractCategoryNumber`、流程实例、任务实例、知会实例、节点 ID/类型/名称、任务类型和时间。`businessTypeCode=10` 或业务类型明确为合同组时展示“合同组”。分类筛选直接使用这些任务字段，不能先用合同搜索求交集。

分类接口的 `data` 可能是根节点对象，不得假定为数组。从 `data` 开始递归遍历每个节点的 `subCategoryNodeList`；每个节点保留 `id`、`name`、`categoryDisplayName`、`categoryNumber`、`abbreviation`、`parentId`、`depth` 和 `enabled`。用户要求启用分类时只保留 `enabled=true` 的节点；`enabled` 缺失表示状态未知。

## 详情完整性硬门槛

写入前必须能在同一详情中精确核对目标任务，并取得：

- `task.taskMeta` 中的业务对象、业务类型、流程实例、任务实例、节点和状态；
- `task.permissions.permittedFunctions`；
- `task.processRecords` 中目标节点的待办/已办记录；
- 抄送时的 `task.noticeRecords`；加签时的加签配置和可回读审批记录。

如果宿主或适配层另外返回 `complete` 与 `limitations`：`complete=false`、`limitations` 非空且影响上述字段，或任何必需字段缺失时，一律停止写入。查询可以降级展示已确认字段，但必须说明“详情不完整”，不得把缺失字段展示为无人、无权限、无记录或未重复。

`commandTypeCode` 与 `assigneeEmployees` 只能说明动作编码和任务处理人，不能单独证明本次加签的目标人员或串签顺序；必须读取本次新增加签动作记录的 `endorsementEmployees`。

## 当前处理人

详情中的同一节点可能同时包含已完成和未完成记录；“当前处理人”不能按节点内查到的全部人员直接展示，必须按以下顺序解析：

1. 将每次详情绑定到所查询的业务对象、流程、任务实例和节点；确认与列表当前行一致。不能按文件序号、数组位置或上一条详情填另一行。目标节点 ID 来自列表/`task.taskMeta`，在 `processRecords.todo[]` 的节点条目上用 `nodeId` 匹配，再读取该条目的 `records[]`；不要误以为每条内层记录必有 `taskNodeId`。
2. 汇总该流程该节点**全部** `records[]` 中 `taskStatusCode=0` 的记录，不仅取第一条、不仅取本人的一条任务。精确任务实例用于确认行与节点归属，不能用于把同节点其他人的未办记录删掉。`taskStatusCode=1` 等已完成记录不计入。
3. 收集保留记录的全部 `assigneeEmployees[]`，按 `employeeId` 去重，保存完整员工 ID 集合、显示姓名和人数。不能按姓名去重，也不能用候选池代替这些实际未办记录。
4. 用 `task.permissions.employeeId` 精确匹配该集合中的本人，仅把其显示姓名改成“你”；保留其他所有人员。不得先把整行赋值为“你”。
5. 一至三人完整展示；超过三人，含本人时展示“你、某某等 N 人”，不含本人时展示两名实际人员及总人数。显示前复核文本中的人数、人员与完整集合一致，不使用其他节点、创建人、负责人或历史人员补位。

批量表格对每一行独立执行上述解析并保存结果后再汇总。禁止把当前登录用户默认填入每一行、用第一条任务详情复用到其他行，或用人员选择器候选推断当前处理人。单行无法可靠解析时仅该行显示“暂未取得”；依赖该详情的写动作停止。

目标任务仍在运行，但记录缺少 `taskStatusCode`、目标节点无法唯一匹配或未完成记录无法取得人员时，当前处理人显示“暂未取得”，并把详情判为不完整：查询可以继续展示其他已确认字段，任何写操作必须停止。不能把历史处理人降级当作当前处理人，也不能写成“无人处理”。

## 写入责任门槛

**加签、转办、当前待办抄送**共用同一责任校验。写入前必须在目标节点当前记录中同时证明：

1. 记录属于已锁定流程和目标节点（节点匹配方式见上节）；
2. `taskStatusCode=0`；
3. `assigneeEmployees[].employeeId` 包含 `task.permissions.employeeId`。

完整证据证明任一条件不满足时，说明该任务已不在当前登录用户的办理责任内；字段缺失或归属不明时只说明详情不完整、无法核实。两者均不按当前待办生成写入方案，不调用写接口，也不自动改选同合同的其他任务。

加签还要额外检查已确认由**当前用户承担的目标责任记录**的 `parentCommandTypeCode`。值为 `18/19/20/21` 时，当前任务是前/后串签或会签产生的被加签子任务；当前用户作为被加签人不得再次加签。不能用同节点其他人的子任务类型推断本人身份，也不靠 `permittedFunctions` 是否仍返回 `endorse` 放行。

**历史任务抄送**不适用上述 `taskStatusCode=0` 门槛：用户明确指定历史审批任务时，从已办记录（`processRecords.done`）中筛选真实审批节点，展示时间和选取依据并确认；仅当详情完整且该节点仍开放抄送时才生成方案。不能把任意最新任务当作用户指定任务，也不能用 `notice/send` 对未确认的历史任务写入。

## 人员数据

搜索结果必须保留 `employeeId`、姓名以及 `departments[].displayName/name`；如有邮箱、工号、`canSelect`、`expired`、`locked`、状态也保留。`departmentNames` 为空时不能丢弃 `departments[]`。

同名时按部门、邮箱、工号消歧；仍不能唯一就停止。执行前再用人员复核接口按 ID 校验全部接收人。搜索不到只表示“没有找到”，不能推断离职。

历史记录或当前处理人集合只能辅助理解业务，不能代替人员选择器产生可选接收人。当前选择器没有返回、按 ID 复核不可选或选择状态无法核实时，不把该记录加入确认方案；近音匹配需要用户明确纠正对象，不能直接沿用历史 ID。

## 允许的写接口

| 动作 | Method 与 path | 请求体 |
| --- | --- | --- |
| 加签 | `PUT /open-web/web/clm/workflow/runtime/task/run` | `businessId`、`businessTypeCode`、`commandTypeCode`、`processTaskInstanceId`、`endorseEmployeeIds[]`、`endorseInfo.endorseEmployeeIdList[]`、条件性的 `endorseeAttributeEditable`、必填 `remark` |
| 转办 | `PUT /open-web/web/clm/workflow/runtime/task/run` | `businessId`、`businessTypeCode`、`commandTypeCode=3`、`processTaskInstanceId`、`transferEmployeeId`、可选 `remark` |
| 抄送 | `POST /open-web/web/clm/workflow/runtime/notice/send` | `noticeEmployeeIds[]`、`processInstanceId`、`processTaskInstanceId`、可选 `remark` |

字段来源和用途：

| 字段 | 来源 | 说明 |
| --- | --- | --- |
| `businessId` | 目标任务详情 | 合同或合同组的业务对象 ID，必须与当前任务一致 |
| `businessTypeCode` | 目标任务详情 | 流程业务类型编码，不是合同分类 ID |
| `processInstanceId` | 目标任务详情 | 当前审批流程实例 ID，仅内部衔接 |
| `processTaskInstanceId` | 目标任务详情 | 实际办理节点的任务实例 ID；不能用合同 ID、知会 ID 或节点 ID 代替 |
| `employeeId` 及人员数组 | 人员搜索后再按 ID 复核 | 标品员工 ID；姓名不能直接写入动作请求 |
| `remark` | 用户为该动作明确提供的原话 | 加签时必填；转办、抄送可选。按步骤独立赋值，未给该步说明就删字段，不能把加签原因自动复制到转办或抄送 |
| `endorseeAttributeEditable` | 详情中的加签配置或用户选择 | 仅页面配置要求选择时追问；固定配置原样沿用 |

加签请求体形态：

```json
{
  "businessId": "<业务对象ID>",
  "businessTypeCode": 0,
  "commandTypeCode": 6,
  "processTaskInstanceId": "<任务实例ID>",
  "endorseEmployeeIds": ["<员工ID-1>", "<员工ID-2>"],
  "endorseInfo": {
    "endorseEmployeeIdList": ["<员工ID-1>", "<员工ID-2>"],
    "endorseeAttributeEditable": "ENDORSEE_EDITABLE"
  },
  "remark": "<用户给出的加签原因>"
}
```

转办请求体形态：

```json
{
  "businessId": "<业务对象ID>",
  "businessTypeCode": 0,
  "commandTypeCode": 3,
  "processTaskInstanceId": "<任务实例ID>",
  "transferEmployeeId": "<接收人员工ID>",
  "remark": "<用户给出的可选说明>"
}
```

抄送请求体形态：

```json
{
  "noticeEmployeeIds": ["<员工ID-1>", "<员工ID-2>"],
  "processInstanceId": "<流程实例ID>",
  "processTaskInstanceId": "<任务实例ID>",
  "remark": "<用户给出的可选说明>"
}
```

把对应 JSON 作为 `--data '<JSON>'` 传给表中的 method/path。可选 `remark` 没有值时删除字段，不发送空的伪造说明。普通回复不得展示上述 ID、命令码或原始请求体。

加签命令码与权限：

| 模式 | commandTypeCode | 必需权限 |
| --- | ---: | --- |
| 并加签 | 6 | `endorse` |
| 后串签 | 18 | `endorse` + `post_serial_endorsement` |
| 后会签 | 19 | `endorse` + `post_parallel_endorsement` |
| 前串签 | 20 | `endorse` + `pre_serial_endorsement` |
| 前会签 | 21 | `endorse` + `pre_parallel_endorsement` |

转办要求 `transfer`，抄送要求 `notice`。权限判断只认 `permittedFunctions[].value`。加签配置为 `ENDORSER_SELECT` 时必须取得用户对“可编辑/只读”的选择；固定为 `ENDORSEE_EDITABLE` 或 `ENDORSEE_NOT_EDITABLE` 时沿用页面配置。人员数组至少一人、不可重复；串签按数组原顺序执行。

发写请求前必须重新读取任务列表、详情和人员：业务对象、任务、节点、动作权限、人员、模式、顺序、原因与用户确认计划完全一致才发送。不得把审批通过、拒绝、完成盖章或完成归档的命令码加入本契约。

读取时点必须在本次授权/确认之后、该步写入之前，具体顺序见[执行契约](execution-contract.md#每次写入的证据顺序)。保存过的旧响应、预览前快照和上一轮文件都不能直接充当最新写前证据。

## 动作特定写后核验

写前保存最小业务快照，写后重新读取详情并比较新增记录。接口 2xx、`success=true` 或本人待办消失都不能单独作为成功。

- **加签：** 先锁定写后新增且属于目标节点的加签记录/子任务。实际子任务记录用 `parentCommandTypeCode` 表示产生它的加签模式；按 `6/18/19/20/21` 与本次模式比对。只有当同一新增动作记录同时带有本次 `endorsementEmployees` 时，才可用该记录自身的 `commandTypeCode` 辅助核验；不得读取无关外层记录的 `commandTypeCode` 代替。通过新记录的 `endorsementEmployees[].employeeId` 核对人员。串签以该数组顺序作为顺序证据，逐项与请求中的 `endorseEmployeeIds[]` 比较；不得使用外层 `records` 数组位置或 `createTime` 推断顺序。并加签记录可能同时包含原处理人，先排除写前目标任务已有处理人，再按集合核对新增人员；前/后会签按目标人员集合和并行运行状态核对，不宣称存在业务顺序。关联子任务通过 `parentProcessTaskInstanceId` 与 `parentCommandTypeCode` 对应本次加签。模式、人员以及串签顺序任一无法证明就标记“结果待核验”。
- **转办：** 必须读到本次转办记录，并在后续活动记录中确认新责任人是目标接收人；仅原待办移除不充分。
- **抄送：** 比较写前、写后的 `noticeRecords.notice[]`，接收人优先读取记录顶层 `noticeEmployeeId`；若该字段缺失，仅在同一记录明确存在 `noticeEmployee.employeeId` 时使用嵌套值。不得把记录顶层的通用 `employeeId` 当作抄送接收人。必须出现属于本次发送的新记录且覆盖全部目标接收人；`readStatus=true/false` 只转换为“已读/未读”，普通回复不输出原始布尔值。记录存在表示已发送，不表示已读。

上述字段路径是核验契约，不允许调用方凭相似字段名猜测。任一字段缺失、类型不符或层级变化时停止判定，返回“结果待核验”，不生成临时代码探测结构，不自动重复写入。

动作记录的创建时间与当前操作时间不一致时，不能仅按旧创建日期推断本次动作未生效、也不能未经核实宣称系统时间异常。按同一目标的写前后记录增量、模式/人员及关联活动子任务核验；需要的业务证据不足时保持待核验。

写请求超时、断连或 CLI 返回传输失败时禁止重发，先执行同一动作的只读回查：已证明生效则报告完成，已证明未生效则报告未完成，仍无法证明则报告“结果待核验”并停止该任务的后续步骤。

## 动态状态追问

- 用户问“他读了吗/现在谁处理/还有我的责任吗”时，在该轮重新 GET 已锁定目标的详情；只读查询不请求写入确认，也不增加发送、转办等动作。
- 已读判断从这份新响应的 `noticeRecords.notice[]` 中，按目标任务与接收人员工 ID 匹配本次发送的记录。优先用已保存的记录标识关联；否则结合本次发送时间与写前后增量唯一确定，不能拿同人更早的通知状态代替。
- 本次匹配记录 `readStatus=true` 才说已读，`false` 才说未读；字段缺失、记录不唯一或本轮读取失败时说“暂时无法确认最新阅读状态”。可补充最近一次已知状态和查询时间，但不能把缓存值说成现状。
- 候选快照可以保留序号、任务集合与展示顺序，写前快照可以用于对比；这两种用途都不意味着可以复用旧数据回答动态状态。新的状态使任务不可办时，说明变化并停止相应写入，不因旧序号仍在就继续。
