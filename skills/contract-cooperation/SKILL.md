---
name: contract-cooperation
description: "通过 contract-cli 以 user 身份查询合同协商列表、协商详情和历史版本文件。用户要查我的协商、我发起的协商、待我确认或定稿、已完成协商、一个或多个发起人的协商、确认人数及历史文件时使用。不负责发起或修改协商、下载文件、智能审查或 app 身份协商管理。"
---

# 合同协商查询

使用 `contract-cli contract cooperation` 查询当前用户有权访问的协商。环境取自当前已确认的 profile，不硬编码域名；所有业务命令显式传同一个 `--profile <profile> --as user --output json`。

## 上下文与入口

- 保留用户已确认的环境、租户及 profile。先查看 `contract-cli contract cooperation --help` 和所需子命令帮助，核对运行中的 CLI 支持本次命令。
- 首次缺少 profile、环境不明或未授权时，按 [认证技能](../auth/SKILL.md) 和 [共享约定](../contract-cli-shared/SKILL.md) 获取对应环境的 user 上下文；当前 CLI 不支持指定环境时报告能力缺口，不改域名绕过限制，也不改用其他环境。
- 不索取 Token、Cookie 或 App Secret，不把 app-only 搜索、链接、记录或文件接口当作个人协商接口。
- 用户明确给出协商 ID 时直接查询详情；只提供合同 ID 时，不把它直接填为协商 ID。可搜索取得真实协商候选，存在多个候选且需要选定对象时让用户选择。

## 查询流程

按用户目的选择最少必要查询，不自动遍历所有协商详情或历史文件。

### 搜索协商

页面筛选先核对 [字段映射与能力缺口](references/filter-coverage.md)，请求体字段、分页及日期约束见 [搜索参数](references/search-parameters.md)。本技能的搜索、详情和历史文件均使用 user MCP 接口，前端代码和页面仅作为筛选与展示规则的核对依据。“全部节点已确认”和抽屉“我是否可确认”尚缺 MCP 筛选参数；用户要求这些条件时说明缺口，不能忽略条件、换用页面接口或将其他视图冒充该条件。将已支持的筛选写入 JSON 文件：

```bash
contract-cli contract cooperation search --input-file search.json --profile <profile> --as user --output json
```

- “我的/与我有关的协商”：`search_tab_enum_code=1`。
- “我发起的协商”：同时传 `search_tab_enum_code=1`、`cooperation_display_tab_code=1`，不猜当前人员 ID。
- “待我确认/定稿”顶部视图：使用我的列表，并传 `cooperation_display_tab_code=2`；“我已确认/定稿”对应 3。抽屉“我是否可确认”是另一条件，不用顶部视图替代。
- “已完成协商”：`cooperation_query_status_codes=[4]`，可与上述范围组合。它是搜索分组，包含返回状态“协商完成（3）”和“已结束（6）”；面向用户均展示“协商完成”，与页面一致；原始码保留，不把返回码套用搜索枚举。
- 按姓名或部门精确筛选时，先使用已安装的 [人员技能](../contract-cli-employee/SKILL.md) 或 [部门技能](../contract-cli-department/SKILL.md) 解析真实 ID；多个候选不能擅自选择。`create_user_id` 为单个 user_id，`department_collection` 为 open_department_id 列表。
- 用户指定多个发起人时，按 [多发起人查询与合并](references/multi-creator-search.md) 逐人调用现有搜索，保留共同条件、分别翻页，再按协商 ID 合并去重；不发送人员数组。报告去重后的已获取数量及查询完整性。
- 首次省略 `page_token`；仅在 `has_more=true` 时原样传回 token 并保留其他条件；`has_more=false` 即停止，即使 token 非空。需要继续但 token 缺失或重复时停止该人的分页并说明未查完；用户要求全部时逐页读取，不把单页说成全部结果。

### 查询详情

```bash
contract-cli contract cooperation get --cooperation-id <协商ID> --profile <profile> --as user --output json
```

只展示响应实际包含的基本信息、当前轮次文件、协商人、轮次记录和背景。合同 ID、协商 ID、协商文件 ID 和底层文件 ID 分别保留，长整数 ID 按字符串处理，不经浮点数转换。只查详情不自动发起审查。

### 查询历史文件

```bash
contract-cli contract cooperation history-files list --cooperation-id <协商ID> --profile <profile> --as user --output json
```

默认包含当前未完成协商轮次的版本；`--include-current-version=false` 仅在原始协商状态为未发起（0）或协商中（2）时排除当前轮次。协商完成（3）、已结束（6）等其他状态仍可能返回最终版，不能仅凭 `latest_version=true` 或返回同一文件判定参数未生效。按实际版本和文件信息整理结果，不根据列表顺序推断最新版本，不把历史版本当作当前主合同；本命令不下载文件。

整理结果前读取 [响应字段、确认人数与页面状态映射](references/response-fields.md)。协商总体状态按页面映射展示，优先于接口原始名称，返回码 3、6 均展示“协商完成”；未知码回退原始名称。确认人数使用返回的已确认人数与总人数展示“0/1 人已确认”，不据此推断全部节点已确认或当前用户能否确认。CLI JSON 保留原始字段。历史文件的嵌套 `history`、确认留痕列表与顶层文件项分别处理，不用顶层项数代替版本总数。

## 错误与结果

- HTTP 403 或业务码为数字/字符串 403：展示本次接口权限原因，立即停止后续详情、文件、审查、轮询和报告调用。不重试、不切换身份或环境、不换正文来源，不引导下载或另传同一合同绕过权限；不保存持久权限状态。
- 未授权时进入当前 profile 的官方授权流程；参数错误先修正已确认条件，不通过更换身份解决。
- 空结果说明本次筛选未命中；不编造合同、协商、人员、文件或下载链接。
- 列表展示实际返回的名称、协商 ID、发起人及相关时间；协商总体状态使用上述页面映射；缺失字段留空或说明未返回。查询部分页面、失败或无权限时明确说明范围，不输出“全部已查完”。
