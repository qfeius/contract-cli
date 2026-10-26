# 生产响应结构核对

2026-10-21 使用生产环境当前已授权 user 做只读抽样核对。以下是实际出现的结构，不表示所有字段在所有记录中必有；展示时以本次响应为准。

## 搜索

分页位置为 `data.items`、`data.has_more`、`data.page_token`。一次查询只返回一页；继续时原样回传 `page_token`。末页观察到 `has_more=false` 且 token 为空。

响应状态码与请求搜索分组码不同：请求完成分组 `[4]` 对应返回“协商完成（3）”或“已结束（6）”，详见 [搜索参数](search-parameters.md)。不把返回码 6 标为“已拒绝”。

列表项包含 `cooperation_id`、`business_id`、`cooperation_name`、`contract_number`、`cooperation_status_code`、`cooperation_status_name`、`user_name`、`create_time`、`update_time` 等字段。ID 按字符串保留。

## 确认人数展示

列表返回 `cooperation_participant_confirm_count`（已确认人数）和 `cooperation_participant_total_count`（参与总人数）。两个值均为非负整数且已确认人数不大于总人数时，展示“已确认人数/总人数 人已确认”，例如“0/1 人已确认”。数值为 0 时保留 0；合法的 0/0 原样展示，不解释为全部节点已确认。

任一人数缺失或为空时，展示“确认人数未返回”，不补成 0；负数、非整数或已确认人数大于总人数时，展示“确认人数异常”，保留原始值供排查，不修正数据。若收到数字字符串，先无损校验为整数再展示。

确认人数是独立展示字段。即使总体状态显示“协商完成”，返回 0/1 也照常展示“0/1 人已确认”。人数相等不能代替“全部节点已确认”筛选；人数差值也不能判断当前用户是否待确认、是否待前序节点、能否定稿。

## 详情

`data` 是协商对象，包含 `cooperation_id`、`business_id`、`current_round_files`、`cooperation_users`、`cooperation_rounds` 等字段。用返回的 ID 核对关联，不把合同 ID 与协商 ID 混用。

## 历史文件

顶层文件数组为 `data.history_files`。抽样项包含 `cooperation_file_id`、`cooperation_file_name`、`cooperation_file_type_code`、`version`、`latest_version`、`history`、`confirmed_file_list` 等字段。

### 当前轮次筛选

`--include-current-version` 透传为 `include_current_version`，默认 true。它控制是否包含当前未完成协商轮次的版本，不是无条件剔除每组文件的最高版本。

| 详情返回的原始协商状态码 | 传 false 时的行为 |
|---|---|
| 0 未发起、2 协商中 | 排除当前轮次的文件版本，保留符合条件的旧轮次版本 |
| 1 无需协商、3 协商完成、4 协商已取消、5 协商回滚、6 已结束、7 已拒绝 | 不执行上述当前轮次过滤，仍可能返回最终版 |

需要解释筛选差异时，以协商详情的原始 `cooperation_status_code` 核对状态；不从页面合并后的状态标签、`latest_version` 或文件名推断。状态缺失或未知时，不猜测适用哪一行。

2026-10-21 的协商中（2）样本在默认参数下返回 1 个文件组，传 false 后为空；另一已结束（6）样本的 true/false 均返回同一 V1.0 最终版，生产日志确认后端收到 false。两者均符合上述规则。空结果只表示该协商在本次过滤后没有返回文件，不推断其他协商没有历史记录。

### 文件分组与版本计数

- 顶层每项代表一个文件版本分组，`history` 包含该组返回的版本，也包含顶层代表项自身。同一 `cooperation_file_id` 在顶层与 `history` 出现时，统计为一个版本，不重复相加。
- `latest_version=true` 标记该文件组返回版本中的最高版本，不保证属于整个协商的当前轮次。即使当前轮次被排除，旧轮次的最高版本仍可带 true；不能据此判定筛选失败。
- `confirmed_file_list` 是确认留痕列表，与主文件版本分别展示，不计为额外的主文件版本。不要把顶层项数当作版本总数；嵌套版本信息不完整时，说明本次可确认的范围。
- 版本号取 `version` 字段，文件名中的版本文字不作为系统版本号。只返回 V1.0 最终版时，如实展示本次返回内容，不仅凭筛选前后相同就报告历史缺失，也不补造更早版本。

HTTP/业务权限拒绝场景由模拟测试验证，本次没有主动访问未授权对象或读取文件正文。

## 面向用户的状态展示：与合同管理协商列表一致

列表和协商基本信息的状态展示，以响应 `cooperation_status_code` 查下表，优先于响应 `cooperation_status_name`。本映射来自前端 `features/pc/src/component-biz/CoopContractStatusTag/utils.tsx` 的 `LEGAL_COOP_INFO`，数值由 `features/common/src/typings/cooperation-enum.ts` 的 `CooperationStatus` 确认；返回码 6 的展示已与用户提供的生产“我的协商”截图核对。

| 返回状态码 | 页面展示名称 |
|---|---|
| 0 | 待发起协商 |
| 1 | 无需协商 |
| 2 | 协商中 |
| 3 | 协商完成 |
| 4 | 协商已取消 |
| 5 | 待发起协商 |
| 6 | 协商完成 |
| 7 | 已拒绝 |

- 正常列表、统计和查询结论统一使用页面展示名称；例如接口 code=6/name=已结束，用户看到的状态应为“协商完成”。code=3 与 code=6 均归入页面的协商完成，不拆成两个面向用户的完成类别。
- 原始状态码和名称仍在 CLI JSON 中完整保留，不修改接口响应，不据此改变搜索条件。仅用户排障或明确需要原始状态时额外标注原始字段。
- 用户问“已完成协商”时仍使用搜索分组 `[4]`，包含返回码 3/6；不能为了显示“协商完成”而排除返回码 6。
- 未知、缺失或不合法状态码不猜测：保留服务端原始名称；也没有名称时显示“状态未知”。字符串数字先无损识别为对应数值，再应用同一表。
- 映射只用于协商总体状态，不套用 `cooperation_user_status_code`、轮次进度、确认人数、角色或文件状态。对方视角“已确认/已定稿”等展示另有当前用户上下文，不能仅凭总体状态猜测。
