# 页面接口运行契约

运行器：`python3 scripts/page_api.py [--profile contract-test] <command>`。

运行器随 Skill 完整目录分发，不依赖业务代码仓。Agent 只调用本文件记录的公开子命令；不得导入运行器内部函数、读取 CLI 凭证或自行拼装页面接口。安装与跨平台要求见 [可移植运行契约](portable-runtime.md)。

所有命令输出 JSON。`ok: true` 表示运行器完成了该次查询或请求；写命令还必须检查 `executed`、页面业务响应和 `postcheck`，不能只看 `ok`。错误时默认只输出 `error.type` 与业务化 `error.message`；只有显式 `--diagnostic` 时才包含最少必要详情。

## 展示边界

本文件描述的是 Skill 与运行器之间的技术契约，不是面向普通用户的回复格式。运行器返回的 ID、profile、environment、请求标识、命令编码、权限枚举、HTTP 信息和原始 JSON 默认只在内部使用；对用户输出必须遵循 [输出契约](output-contract.md)，优先展示合同、节点、人员、数量、影响和业务原因。

`--confirm-plan-fingerprint` 是内部防漂移硬拦截。Skill 保存预览返回的 `planFingerprint`，确认有效后原样填入，绝不要求用户查看、提供或复述。它绑定动作、任务、节点、接收人顺序和关键参数。`--confirm-task-id` 仅用于兼容性二次校验。

## 公共参数与输出

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `--profile` | 请求 | 否 | contract-cli profile，默认 `contract-test`；profile 的环境必须是 `test`，身份必须是 `user`。 |
| `--diagnostic` | 请求 | 否 | 仅在用户授权排障时使用；返回最少必要请求、权限和错误详情。日常调用不传。 |
| `profile` | 响应 | 是 | 本次读取登录态所用 profile，不包含凭证。 |
| `environment` | 响应 | 是 | 固定为 `test`；其他环境在发请求前被拦截。 |
| `ok` | 响应 | 是 | 运行器是否完成本次调用；不等同于写动作已经执行。 |
| `executed` | 写响应 | 是 | `false` 为预览，`true` 才表示已经发起页面写请求。 |
| `error.type` | 失败响应 | 是 | 错误类别，如 `auth`、`target`、`validation`、`permission`、`confirmation`、`http` 或 `business`。 |
| `error.message` | 失败响应 | 是 | 不含任务 ID、请求标识、本机路径和 HTTP 状态的业务化摘要；Skill 仍需结合已知合同与节点重新组织，不能原样粘贴整段运行器输出。 |

运行器只读取本机登录配置中的 User 凭证，输出前递归移除凭证、密钥和票据类字段。

### `runtime-check`

首次业务调用使用。它检查 contract-cli、Python、curl、目标 profile、test 环境和 User 身份是否满足随包运行条件，只返回可用状态和版本摘要，不返回命令路径、配置路径或凭证内容。失败时不得改用原始 HTTP 请求。

## 查询命令

### `tasks`

查询当前用户在标品任务中心的分页列表。未指定业务类型时，运行器会合并普通合同与合同组任务、按任务身份去重后再分页；不会把合同组静默漏掉。

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--scope todo\|done\|notice` | 否 | 列表范围，默认 `todo`；分别对应待办、已办和抄送。 |
| `--query` | 否 | 可传合同/合同组编号或名称、类型和节点等业务文字。运行器在当前用户完整可见任务集上做本地包含匹配；不是最终精确定位条件。 |
| `--business-type-code` | 否 | 页面业务类型编码，只在已知真实编码时传入。 |
| `--page-index` | 否 | 页码，从 1 开始，默认 1。 |
| `--page-size` | 否 | 每页数量，默认 20。 |

响应 `page.content[]` 只保留任务定位所需摘要，不再返回创建人、负责人、头像、租户、角色等无关人员数据。`businessTypeCode=10` 表示合同组；`contractNumber` 可能为空；`taskNodeTypeName`、`taskNodeDisplayName` 和 `timestamp` 用于区分节点与时间。内部 ID 仍只用于后续精确调用，不面向用户展示。

`todo` 还会对当前页逐条读取任务详情并补充以下展示字段。详情读取并发数受限，单条详情失败不影响其他待办返回：

| 字段 | 说明 |
| --- | --- |
| `sequence` | 当前查询页内对应的全局序号，从 1 开始；只用于支持“第 N 条”的自然指代。 |
| `objectType` | 业务对象类型，普通合同显示“合同”，合同组显示“合同组”，用于直接生成待办表格。 |
| `currentHandlerNames` | 该行任务对应当前节点的未办人员名称列表；当前登录用户写为“你”，不返回员工 ID。 |
| `currentHandlerCount` | 当前节点未办人员数量；详情无法可靠读取时为 0，不能据此认定节点无人处理。 |
| `currentHandlerDisplay` | 直接用于表格的业务化文本；一至三人展示姓名，超过三人压缩为“前两人等 N 人”，无法读取时为“暂未取得”。 |
| `currentHandlerStatus` | `available` 表示已从该任务对应节点取得人员，`unavailable` 表示未取得；只用于 Skill 判断，普通用户无需看到原始值。 |

人员来源是任务详情中与该待办 `taskNodeId` 匹配的 `todo.records[].assigneeEmployees`，并排除已经带完成命令的记录；不会用 `ownerEmployee` 或跨节点的全流程人员列表猜测。

### `task-list`

面向普通“查一下我的待办/继续看”场景，一次完成完整待办计数、当前批次的处理人读取和固定六列表格生成；Agent 不需要先调用 `tasks` 再拼接全部任务 ID。

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--query` | 否 | 按当前用户完整可见待办中的合同/合同组业务文字筛选。 |
| `--business-type-code` | 否 | 已知真实业务类型编码时使用。 |
| `--page-index` | 否 | 展示批次，从 1 开始；“继续看”依次递增。 |
| `--page-size` | 否 | 每批 1–20 条，默认 10。总数不超过 20 时忽略分批并完整展示。 |

响应 `presentation` 固定返回六列、完整总数、本批起止序号、尚未展开数量和可直接展示的 Markdown。总数超过 20 时，第二批从 11 连续编号，不能重新从 1 编号。`page.content[]` 保留本批序号和内部任务绑定，供同一会话理解“第 N 条”；不得面向用户展示其中的技术 ID。

### `task-table`

通过可重复传入的 `--task-id` 接收一个或多个任务；任务值本身不可重复。它仅处理已经完成动作、节点、权限和重复操作筛选后的当前待办集合，并按传入顺序返回固定六字段业务表格。任务 ID 只在命令参数内部使用，不进入 `presentation.rows` 或 Markdown。普通待办查询不用本命令，改用 `task-list`，避免重复读取整批任务。

| 字段 | 说明 |
| --- | --- |
| `columns` | 固定为序号、对象、名称、编号、当前节点、当前处理人。 |
| `candidateCount` | 最终可执行任务数；阈值只使用这个值。 |
| `displayMode` | 1–20 条为 `full`，超过 20 条为 `summary`。 |
| `displayedCount` | `full` 时等于全部任务数，`summary` 时为 10。 |
| `remainingCount` | 尚未展开的数量。 |
| `rows` | 业务字段行，不包含任务、流程、员工等内部 ID。 |
| `markdown` | 与 `rows` 一致的固定六列表格，可直接用于用户回复。 |

传入任务重复、任务已经不在当前待办或无法可靠读取时直接失败，不靠模型删除、替换或猜测。

### `resolve`

将自然语言中的合同、流程、节点或类型条件解析为当前用户真实可见任务。支持的精确筛选参数为：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--scope` | 否 | 默认 `todo`。加签、转办只能使用 `todo`；历史抄送可以查询 `done`。 |
| `--task-id` | 否 | 精确匹配 `processTaskInstanceId`。 |
| `--process-id` | 否 | 精确匹配流程实例 ID。 |
| `--contract-id` | 否 | 精确匹配合同业务 ID。 |
| `--contract-number` | 否 | 精确匹配合同编号，不做模糊等同。 |
| `--node-id` | 否 | 精确匹配任务节点 ID；节点名称需先从候选任务中解析。 |
| `--category-id` | 否 | 精确匹配合同类型 ID。 |
| `--category-number` | 否 | 精确匹配合同类型编号。 |
| `--include-descendants` | 否 | 与类型 ID 或编号一起使用；把一级类型展开为自身及全部子类型。 |
| `--node-type approve\|submit\|archive` | 否 | 按真实人工节点类型筛选；“最新审批/历史审批”必须传 `approve`，避免选到更晚的提交或归档任务。 |

响应 `tasks[]` 给出全部匹配任务，`matchCount` 给出数量，`latestTaskId` 和 `latestSelection` 只在已经完成动作/节点语义过滤后使用。Skill 必须向用户说明“最新”的节点、时间和选择依据；不得把未筛选的任意最新任务当成最新审批。

### `employees`

`--keyword` 必填，可传姓名、邮箱或员工编码。响应中的 `employeeId` 是加签、转办和抄送的人员参数来源；同名多条时用部门、邮箱等信息让用户选择。查询默认排除当前登录用户。审批通过不需要接收人参数。

该命令只反映标品人员选择器当前返回的候选。返回 0 条只能解释为“未找到人员”，不能据此判断人员已经离职；返回唯一可选人员也不能解释为已经识别并排除了历史离职档案。只有页面明确返回锁定或不可选标记时，才能按该明确状态拦截。

### `categories`

`--keyword` 可选，按类型名称、展示名称、类型编号、缩写或 ID 过滤。响应包含 `id`、`categoryNumber`、`parentId`、`depth` 和启用状态，可用于区分一二级类型。

### `task-detail`

`--task-id` 必填；`--scope` 默认为 `auto`。任务详情按标品任务中心当前调用方式只传任务或抄送实例标识，从而同时支持普通合同与合同组。响应 `detail.permissions.permittedFunctions` 是页面对当前任务实际开放的动作列表；`detail.permissions.endorsePermissionInfoVo` 决定加签人编辑权限是否固定或需要发起人选择。

任务详情还返回业务安全的 `detail.noticeSummary`：`count` 为可见抄送记录数，`latest` 为按创建时间排序后的最新记录，`records[]` 只包含接收人姓名、是否已读和创建时间。它不返回抄送记录 ID、员工 ID、请求标识或其他内部字段，用于回答“对方看到了吗”等后续追问。

## 审批通过命令 `approve`

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--task-id` | 是 | 当前用户待办中的真实审批任务实例 ID；运行器会重新反查合同、节点和页面动作权限，普通用户无需提供或看到。 |
| `--remark` | 否 | 用户明确给出的审批意见；不传时不自动生成。页面或租户要求必填但用户未提供时，业务校验失败后应追问，不得擅自补写。 |
| `--execute` | 否 | 不传时只生成预览；传入时准备实际写入。 |
| `--confirm-plan-fingerprint` | 执行时必填 | 内部硬拦截；必须原样使用本次预览返回的 `planFingerprint`。目标或意见变化后旧值失效。 |
| `--confirm-task-id` | 否 | 兼容性校验；如传入必须与 `--task-id` 完全相同。 |

审批通过只接受页面实时开放 `agree` 的当前真实审批节点。节点允许审批时编辑合同内容时运行器直接拦截，避免跳过页面表单校验。写后只有同时核验原待办移除与本节点通过记录时，`postcheck.status` 才为 `verified`。

## 加签命令 `endorse`

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--task-id` | 是 | 当前用户待办中的 `processTaskInstanceId`；运行器会实时反查合同、流程和页面动作权限。 |
| `--employee-id` | 是 | 加签人的 `employeeId`；每人传一次，至少一人，不可重复；串签顺序按参数出现顺序保留。 |
| `--mode` | 是 | `parallel`、`before-serial`、`before-parallel`、`after-serial`、`after-parallel` 之一；前/后会签至少两人。 |
| `--remark` | 是 | 加签原因，原样作为审批评论；与标品页面的必填校验一致。 |
| `--endorsee-editable` | 条件必填 | 当页面返回由发起人选择时传 `editable` 或 `read-only`；节点已固定时运行器使用固定配置。 |
| `--execute` | 否 | 不传时只预览；传入时准备实际写入。 |
| `--confirm-plan-fingerprint` | 执行时必填 | 内部硬拦截；必须原样使用本次预览返回的 `planFingerprint`。人员、模式、原因或目标变化后旧值失效。 |
| `--confirm-task-id` | 否 | 兼容性校验；如传入必须与 `--task-id` 完全相同。 |

预览响应的 `request.body` 包含：合同业务 ID、业务类型编码、加签命令编码、任务实例 ID、加签人 ID 数组、加签人编辑权限和可选评论。`preflight.permissions.permittedFunctions` 不包含加签及对应模式时直接拦截。

## 转办命令 `transfer`

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--task-id` | 是 | 当前用户待办中的任务实例 ID。 |
| `--employee-id` | 是 | 唯一接收人的 `employeeId`；不能是当前用户本人。 |
| `--remark` | 否 | 转办业务说明。 |
| `--execute` | 否 | 不传时只生成请求预览。 |
| `--confirm-plan-fingerprint` | 执行时必填 | 必须原样使用本次预览返回的 `planFingerprint`。 |
| `--confirm-task-id` | 否 | 兼容性校验；如传入必须与本次任务一致。 |

转办只移动当前登录用户在该任务上的审批责任。页面没有 `transfer` 权限、任务已不在待办或接收人是本人时不发写请求。

## 抄送命令 `notice`

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--task-id` | 是 | 当前用户可见任务的 `processTaskInstanceId`。 |
| `--employee-id` | 是 | 抄送人的 `employeeId`；每人传一次，至少一人，不可重复，也不能包含当前用户。 |
| `--remark` | 否 | 抄送原因或提示文字。 |
| `--allow-repeat` | 否 | 仅当用户明确要求再次抄送给已有记录的同一人员时传入；默认发现已有记录就拦截。该授权参与计划指纹，但不会作为未知字段发送给页面接口。 |
| `--execute` | 否 | 不传时只生成请求预览。 |
| `--confirm-plan-fingerprint` | 执行时必填 | 必须原样使用本次预览返回的 `planFingerprint`。 |
| `--confirm-task-id` | 否 | 兼容性校验；如传入必须与本次任务一致。 |

抄送请求使用任务详情中的流程实例 ID 和任务实例 ID，不改变审批责任。页面没有 `notice` 权限时不发写请求。

写命令默认不返回接口路径、请求方法、原始请求体或完整权限；需要排障时在主命令前增加 `--diagnostic`。实际写入后响应包含 `postcheck`：

| 字段 | 说明 |
| --- | --- |
| `postcheck.status` | `verified` 表示已从写后业务记录核验本次关键效果；`not_verified` 表示页面已接受请求，但记录暂未覆盖全部预期，不能报告完整成功。 |
| `postcheck.businessCheck` | 供 Skill 理解复查结论的业务摘要，仍需结合合同、节点和人员组织用户回复。 |
| `postcheck.endorsement` | 加签复查结果，包含业务模式、接收人显示名和串签顺序是否核验；内部不向用户展示人员 ID。 |
| `postcheck.notice` | 抄送复查结果，包含接收人显示名和是否全部已读；有新增发送记录不等于对方已阅读。 |

复合动作没有额外的后端“批量组合接口”。Skill 先对每一步完成业务预检，再把步骤合并成一份业务预览供用户一次确认；确认后按顺序逐步调用。每一步真正写入前重新调用对应命令生成当前技术预览和指纹，在业务对象、步骤、人员与参数仍等于已确认计划时直接使用该指纹执行；业务计划变化则停止并重新确认。这样可以适应前一步造成的预期待办状态变化，同时避免绕过实时校验或伪装成可回滚事务。

加签复查会比较写前、写后的审批记录，核对命令模式、新增加签人员、子任务及串签顺序。抄送复查会比较新增抄送记录和目标人员。任一复查不能完整证明预期时返回 `not_verified`，且运行器不会自动重复写入。
