# 页面筛选与 CLI/MCP 覆盖

本技能仅调用 user MCP 接口。多个发起人由 Skill 逐人查询后合并去重；MCP 单次请求仍只接收一个发起人。确认流程进度和我是否可确认的筛选尚未进入当前 MCP 契约。

2026-10-21 对照生产 Firefox“合同管理 → 我的协商”，并读取前端 `apps/clm/src/features/management/containers/CooperationList/components/FilterForm/useFilterList.tsx`。测试使用当前 user 授权；不使用网页 Cookie，不写业务数据。

| 页面筛选 | MCP/CLI 请求字段 | 规则与覆盖 |
|---|---|---|
| 协商状态 | cooperation_query_status_codes | 协商中 2、待发起 1、已取消 5、完成 4、已拒绝 6；与返回状态枚举分离 |
| 合同类型 | contract_category_ids | 字符串 ID 数组，CLI 单选/多选已验证 |
| 协商发起日期 | search_cooperation_by_create_time | 两个 YYYY-MM-DD，包含起止日；前端手工输入格式为 YYYY/MM/DD |
| 最新版本上传时间 | search_cooperation_by_latest_version_upload_time | 同上，不能用合同更新时间替代 |
| 协商完成时间 | search_cooperation_by_cooperation_complete_time | 同上；前端待我确认/定稿视图隐藏该条件 |
| 协商发起人 | create_user_id | 每次传单个 user_id；多个发起人按 [逐人查询与合并](multi-creator-search.md) 取并集。生产页面当前是单选，可分别查询对照 |
| 协商发起人部门 | department_collection | open_department_id 字符串数组，不传页面内部 createDepartmentIds |
| 我确认的时间 | search_cooperation_by_last_confirm_time | 页面仅在“我的”范围的全部/我已确认视图展示；本次没有已确认正例 |
| 已确认角色 | confirmed_role_codes | 页面法务 legal、财务 finance；个人范围及全部可见正例已核对 |
| 确认人数展示 | cooperation_participant_confirm_count / cooperation_participant_total_count | 只作人数展示，缺失不补 0，异常不修正；不据此推断节点确认状态 |
| 确认流程进度：全部节点已确认 | 尚无 MCP 字段 | 前端 confirmProcessStatusCodes=[4]；须扩展 MCP 服务端后验证 |
| 我是否可确认 | 尚无 MCP 字段 | 前端 confirmableStatusCodes：1 待我确认、2 待前序节点确认、3 待我定稿、4 我已确认；须扩展 MCP 服务端后验证 |

## 不能混淆的视图与筛选

“我是否可确认 → 待我确认”不是顶部“待我确认/定稿”视图。本次同一账号同一“我的协商”范围下，前者页面返回 4 条，后者返回 3 条。用户明确要求确认流程进度或我是否可确认时，应说明当前 MCP 能力缺口；不改传 cooperation_display_tab_code=2 或其他近似条件，不忽略条件，也不改用页面接口。

## 待补齐的 MCP 能力

本次暂不改后端和 CLI。两组确认筛选仍需 MCP 接口接收、传递对应条件后才能验证；展示确认人数不等于支持这些筛选。多人逐人查询已复用单值 create_user_id 及现有 ID 转换；只有后续要求单次请求原生多选和服务端统一分页时，才需另行核对数组参数与转换支持。不能直接把页面内部人员 ID 当作 user_id，也不能只让 CLI 接受字段便宣称服务端筛选生效。

切换视图导致时间字段在页面隐藏时，不静默删改用户条件；用户要求严格复现页面时先说明条件与视图不兼容，确认查询范围后再执行。

## 展示字段与过滤命中

已确认角色筛选以服务端查询结果为准，不能用返回列表项的 confirmed_roles 再过滤。生产全部可见范围的财务筛选中，首条记录该字段为空；Firefox 同样返回该记录，角色列展示“-”。两端前两条记录一致。不要编造角色、将空数组展示成“财务已确认”，或只因展示字段为空就断言筛选无效。

## 验证范围

“我的协商”4 条为可穷尽基准：状态、类型、日期、发起人、部门、角色分别对照。法务/财务额外在全部可见范围抽样前两条，与 Firefox 一致。空结果只能证明当前样本结果一致；本次未覆盖的取消、拒绝、待发起和本人已确认正例不宣称已验证。
