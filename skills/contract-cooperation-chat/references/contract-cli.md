# 通过合同 CLI 办理协商群聊

下列调用已在测试环境以用户身份走通。使用本次目标环境对应的已授权 profile。先检查 `contract-cli api call --help` 是否支持 `--channel web` 和 `--as user`；部分版本只提供应用身份协商搜索，不能代替当前用户校验。需要补查协商详情时，复用同环境、同组织、同一用户的合同 MCP `get_cooperation_detail(cooperation_id)`。不使用页面详情接口作为只读回退。

每次请求使用 `contract-cli api call --channel web --profile <目标profile> --as user --output json <METHOD> <PATH>`，按表追加参数。将占位内容替换为实际值，通过 CLI 的参数传值或 JSON 输入文件传递用户输入。检查 HTTP 和业务结果（测试环境返回 `success: true`、`code: 200`），再读取 `data`。ID 全程保留为字符串。

| 用途 | 方法和路径 | 参数与结果 |
|---|---|---|
| 按编号或名称找协商 | POST `/open-web/web/clm/cooperation/query` | `--query pageIndex=0 --query pageSize=100`，JSON 请求体示意 `{"keyword":"合同编号或名称","cooperationQueryStatusCodes":[0],"searchTabEnumCode":0}`，通过 `--data` 传入。按实际页大小及 `data.totalPages` 从 0 逐页递增，到最后一页为止。按 `content[].contractNumber` 或 `cooperationName` 和原文精确匹配，按 `cooperationId` 去重，核对分页完整。宽泛匹配、同名和不完整结果按主文档消歧。保留唯一行的协商 ID、当前轮次 ID、本人参与标志、角色和状态。 |
| 核实轮次与本人资格 | 复用上一步最新的同一协商行；按需补 MCP 详情 | 用 `cooperationRoundId` 定位本轮。写前要求 `isCooperationUser=true`，且 `cooperationUserTypeCode` 为 0（申请人）或 1（参与人），`cooperationUserStatusCode` 为 0、1、2、3 或 6。4 为失效，5 为退出。非成员行可能将角色默认填为 1，不能据此放行。只有 ID、关键信息不足或结果冲突时，通过只读 MCP 详情核对 `cooperation_id`、`cooperation_round_id` 和当前用户在 `cooperation_users[]` 中的有效记录。无法取得可信详情或最新匹配行时说明缺口并暂停。 |
| 新建前核实人数（仅在缺少人数依据时） | GET `/open-web/web/clm/cooperation/employee/list` | 已有群时跳过此步。若当前轮次的 MCP 详情已能确认有效人数，直接复用；否则用 `--query cooperationId=<协商ID>` 查 `data.content`。确认至少两名不同的有效人员即可。仅在信息完整时判定只有一人，人数不明时暂停新建。 |
| 读取群状态与成员状态 | POST `/open-web/web/clm/cooperation/chatGroupStatus` | `--data '{"cooperationRoundIds":["<当前轮次ID>"]}'`。每次只查目标轮次，列表的 `cooperationChatGroupStatus` 仅作线索。按[群状态判读](../SKILL.md#读取当前轮次的群状态)核对最新状态、成员布尔值和飞书群 ID。已有群时先查 `data.membershipQueryFailedRoundIds`：若包含当前轮次 ID（按字符串比较），即使成员标志为 false 或 true，也按“成员身份未知”处理——仅提示“协商群已存在，暂时无法确认您是否在群内”，不询问、不提交入群。排除该失败后，仅接受 `data.currentUserInChatGroups[0]` 的布尔值；缺失、非布尔值或冲突按未知处理。若目标环境未返回 `membershipQueryFailedRoundIds`，再按“false 表示接口报告未找到本人”处理，并说明该环境缺少失败标志。 |
| 新建或已获同意后加入 | POST `/open-web/web/clm/cooperation/openOrCreateChatGroup` | 仅在主文档对应分支的写前检查通过后执行：`--query cooperationId=<协商ID> --query cooperationRoundId=<当前轮次ID> --data '{}'`。两个 ID 是查询参数。业务成功的 `data` 是飞书群 ID。对于加入，须与用户同意的群 ID 一致。完成后用上一行只读接口回查同一轮次，按实际群与成员状态给出回执。 |

同一次写入选择一个通道即可。CLI 返回超时、空群 ID、意外群 ID 或不确定结果时，只读核对，避免切换到页面再次提交。建群、入群共用该接口，服务端按提交瞬间的状态决定动作。轮次或群变化时按[核对执行结果](../SKILL.md#核对执行结果)处理。
