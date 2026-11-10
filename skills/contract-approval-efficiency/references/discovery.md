# 按姓名、部门路径或角色确定统计人员

沿用[通用调用协议](runtime.md#通用调用协议)和当前 USER。以下都是 Open Web 只读 GET，参数放 query。目录接口只用于匹配名称、路径、成员及真实 ID。沿用当前账号可浏览的组织目录范围；若组织树被禁止、只能关键词搜索或无法确认更大范围的目录权限，不遍历更大范围。目录可见和角色成员身份均不授予任务或流程读取权限。

## 接口与数据衔接

| 用途 | Open Web 路径 | 查询参数与响应 |
|---|---|---|
| 人员关键词 | `/open-web/web/clm/employee/findEmployeesByKeyword` | `keywords` 为输入姓名或可验证的别名，`filterCurrentEmployee=false`、`filterVisibleEmployee=true`、`noFilterDeletedEmployee=false`。成功读取 `data[]` 的 `employeeId`、数字 `larkId`、`name`、`displayName`、`enName`、`departments`；仅用目录准许显示的候选。 |
| 部门关键词 | `/open-web/web/clm/department/findDepartmentsByKeyword` | `keyword` 为部门名，`requireAll=false`、`showAllStatus=false`。成功读取 `data[]`，以真实 `departmentId`、`name/displayName` 及其上下文识别候选。 |
| 组织浏览配置 | `/open-web/web/clm/systemConfig/security/departmentSelect` | 无参数。成功响应的 `data.selectionType` 是数字枚举：`1` 表示 ALL（默认展示部门及组织架构），`2` 表示 KEYWORD（不展示全部组织架构），`0` 表示未定义。仅当 `selectionType === 1` 时可使用组织树，仍以树接口实际授权为准。其他值、字符串、字段缺失或配置不可读均不放行。 |
| 部门目录种子 | `/open-web/web/clm/department/findDepartmentsByKeywordIncludeValid` | 仅在目录允许浏览组织树、且指定名称未能定位时使用。省略 `keyword`，传 `onlyStruct=false`、`requireAll=false`；不传 `showAllStatus`。成功读取 `data[]`，递归检查其 `childDepartments`。可能含停用/失效或只覆盖部分层级，不能据空结果或仅一条近似结果断定目标不存在或唯一。 |
| 组织树与直属成员 | `/open-web/web/clm/department/organization/list` | `parentDepartmentId` 为真实部门数字 ID，组织虚拟根用 `0`；`employeePageIndex` 从 `1` 开始，`employeePageSize=50`。省略 `querySeat`。成功读取 `data.subDepartments[]`、`data.employees[]`、`data.ancestralDepartments[]` 和布尔 `data.hasMore`。 |
| 角色基本信息 | `/open-web/web/clm/permission/role/base/list` | `filterDisabled=true`、`filterDefault=false`。读取未分页的 `data[]`，按 `name/displayName`（或用户明确给出的角色 `code`）匹配，使用返回的真实 `roleId`。 |
| 角色成员 | `/open-web/web/clm/permission/role/employee/list` | `roleId` 必须是上一步 `role/base/list` 返回的真实数字 ID，按原始十进制文本发送。无效 ID 导致的 `110002` 按参数错误处理，不重试、不视为空名单。成功响应返回未分页的 `data[]`；从每项 `employeeVO` 提取 `employeeId`、数字 `larkId`、姓名和当前部门。 |

这些目录响应沿用 CLM 成功判据。缺字段、非成功响应和权限拒绝都不能按空数组处理。以文本保存 ID；`employeeId` 用于去重和记录归因，`larkId` 传给任务接口。两者从同一条人员记录取得，不能互相替代。多来源同 ID 数据冲突时先按可靠目录核实。

## 部门名称和路径

1. 用户明确给路径时，直接按路径段处理；其他输入先按完整名字查部门关键词，保留含连字符的精确名称。完整名字无精确匹配且含 `/`、`／` 或 `-` 时，再按路径段处理。去掉段首尾空白，校验完整的父子关系。
2. 需要使用组织树定位路径或读取成员时，先核对浏览配置。仅数字 `selectionType === 1`（ALL）可进入组织树；`2`（KEYWORD）、`0` 或配置不可读时不发起组织树浏览，保留目录覆盖缺口。首段关键词无法定位时，在可浏览范围内取目录种子，按 `name/displayName` 匹配，包含匹配只用于产生候选。首段找不到、仅部分层级可见或路径无法核实时，从可浏览组织树的根或已确认的上级逐层定位。遇到同名父节点可沿后续段逐支核对，只有完整路径唯一才采用。
3. 对路径每一段，使用上一级实际 `departmentId` 请求组织树首屏，从 `subDepartments[]` 精确选下一层。最终使用叶子部门 ID。祖先的 `employees[]` 不计入叶子统计；只在定位结束后，读取目标部门及用户范围内子部门的成员。组织根无直属成员，不能据此结束。
4. 单个部门名在关键词和种子中仍未找到时，遍历当前账号可浏览的部门树，沿 `subDepartments[]` 查找并记录完整路径。定位阶段只用首屏组织结构，不读取无关部门的员工分页。重名或仅有包含匹配时显示允许查看的候选路径请用户选择。完整遍历无匹配才可返回目录范围内 `not_found`；不能展开或存在未查完的相关分支时，请用户给上级或完整路径，并保留覆盖缺口。

## 人员姓名

1. 按原名查关键词，优先用真实 `name/displayName` 精确匹配。比较时可去掉首尾空白；目录显示形如“中文姓名（英文名）”时，也用括号前的完整中文姓名匹配，保留同名候选。重名按候选的当前部门区分；明确提供了部门范围时，候选也必须符合该部门范围。
2. 中文关键词无有效命中时，有部门信息则按上文定位部门，遍历请求范围内的成员，对返回的 `name/displayName` 本地精确匹配。无部门信息时可先用由输入推导的拼音（全拼或首字母）补查，也可用用户已提供或已确认的英文名/工号。拼音及别名只用于找到候选，候选仍需匹配原中文姓名或经用户确认；不凭拉丁关键词的唯一命中推断身份。
3. 仍未命中时，按表中的浏览配置确认可浏览范围，再分页查成员并按第 1 步匹配。无部门线索时，只有 `selectionType` 为数字 `1`（ALL）才可从虚拟根 `0` 遍历；任何分支拒绝时保留缺口，不绕行。保留同名候选，完成相关范围内的搜索再判定唯一。无需保存无关人员资料。当前权限只允许关键词查询、搜索未完成或仍无匹配时，状态为 `needs_input`，请用户提供所在部门、拼音、英文名或工号，不要求内部 ID。

旧复数 `findEmployeesByKeywords` 已弃用且复用同一关键词查询，不能作为独立兜底；组织架构的关键词搜索也不替代上述成员查找。多个姓名分别解析；有未确认的目标时明确列出待确认对象，未经确认不合并其记录。

## 组织成员分页与完整性

按部门缓存首屏的 `subDepartments[]`，成员每次从第 1 页依页码递增，直到成功返回 `hasMore=false`；短页不足以结束。若上一页 `hasMore=true`，下一页仍需读取，合法空页且 `hasMore=false` 可以结束。部门 ID 去重防循环，成员按 `employeeId` 去重，保留范围内直属/子部门关系。页重复、缺字段、非成功响应或 `hasMore=true` 但无新成员时，标记对应分支未完成。所有所需部门及成员页均完整，才标记 `roster_complete=true`。允许显示的空部门和目录被拒/未查完是不同结果。

## 角色成员

“某个角色的审批效率”按**当前启用的系统角色成员**统计。成员是采集时快照；统计周期应用在成员自己的任务记录上，不能推断历史角色成员或任务当时以该角色办理。若用户问的是流程节点配置的角色或历史角色归因，先确认其口径，不能将其静默改成当前角色成员。

1. 先调用 `role/base/list`，匹配角色后使用返回的真实 `roleId` 查询成员。若重名，请用户按角色名称及可见角色编码选择。两条配方均要求当前 USER 已有后台页面角色管理所需的 `ADMIN_PAGE_FUNCTION_AUTHORITY`。失败时区分参数、权限和角色不存在：权限被拒时说明缺少读取权限；角色列表成功读取且目标不存在时返回 `not_found`。成员接口返回 `110002` 时，不重发同一个请求，可重新读取角色列表一次以确认 ID 是否失效；若角色仍存在且 ID 有效，报告成员读取失败。任何 `110002` 都不能作为角色无成员的依据。
2. 成员只取该角色响应中真实绑定的 `employeeVO`；`dimensionList` 是各成员负责的部门范围，不是成员清单。缺失身份字段或映射冲突保留覆盖缺口，不按零。完整且真正空的名单才可输出“该角色暂无成员”。如用户另限定部门，只与已核实的当前部门成员清单取交集。
3. 每位成员都按[任务接口](runtime.md#人员任务-getvisibletasks)独立查已办、待办，再校验流程读取权限。能查看角色成员不代表能查看其任务。角色任务权限不足按个人/部门相同规则处理。
