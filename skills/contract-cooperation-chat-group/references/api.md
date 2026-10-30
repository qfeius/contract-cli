# CLI 查询接口与结果

本参考给出完整调用配方。三个请求均使用当前已确认的 profile、组织 user 和 CLI 自身授权，路径保持相对 `/open-web/web/**`。`<profile>`、`<contract-id>` 等是待替换位置，不能按示例字面值执行。

## 共用规则

- ID 全程保留原文字串。合同、协商、轮次 ID 是正整数 Long；生成请求时不经过 JavaScript `Number` 或其他浮点数。轮次请求的数组元素是 JSON 数字，将可靠响应中的完整十进制数字原文复制到数字位置，不加引号、不取整或截断。
- JSON 请求写入 UTF-8 文件并用 `--input-file` 发送。搜索词来自响应，按 JSON 规则转义；不直接拼进 shell 命令。
- 使用 `--raw` 保留响应原文。若宿主解析工具会损失长整数精度，从原文保留 ID 的完整字符，不采用已舍入的解析值；无法保真时停止该步骤。
- 成功需同时满足 CLI 执行成功、响应为有效对象、`code` 为数字/字符串 `200`、`success=true`。Open Web 业务响应与结构化开放平台命令的 `code=0` 协议不同，不能混用。HTTP 200 本身不代表业务成功。
- 所需普通业务头只有下述搜索接口的 `Timezone-Offset`。不构造授权、Cookie 或可信身份覆盖头。

## 1. 合同关联：briefInfo

```sh
contract-cli api call --channel web GET /open-web/web/clm/contract/briefInfo \
  --query 'contractId=<contract-id>' \
  --profile <profile> --as user --raw
```

| 参数/字段 | 类型、来源及用途 |
| --- | --- |
| query `contractId` | 必传，用户确认的合同 ID 完整十进制原文 |
| `data.contractId` | 应与请求 ID 相同；不相同时停止 |
| `data.cooperationId` | 关联协商 ID，供下一步精确匹配；缺失时停止定位 |
| `data.contractNumber` | 非空时作为搜索词；不转换成数字 |
| `data.contractName` | 编号为空时作为搜索词 |
| `data.contractStatusCode` | 仅用于如实展示阶段；不作为是否查询群 ID 的开关 |

本接口没有可靠的当前轮次字段。`data=null` 只表示本次未取得关联，不推导无群或权限原因。

## 2. 当前轮次：searchV2

将以下对象写入 `cooperation-search.json`，把 `keyword` 换成上一步取得的编号或名称：

```json
{"searchTabEnumCode":0,"cooperationDisplayTabCode":0,"keyword":"<上一步取得的搜索词>"}
```

```sh
contract-cli api call --channel web POST /open-web/web/clm/cooperation/searchV2 \
  --query 'pageIndex=0' --query 'pageSize=20' --query 'order=DESC' \
  --header 'Timezone-Offset: -480' \
  --input-file cooperation-search.json \
  --profile <profile> --as user --raw
```

| 参数 | 位置、类型与策略 |
| --- | --- |
| `pageIndex` | query，零起始整数，首次 0，续页加 1 |
| `pageSize` | query，本流程固定 20；若响应提供实际页容量则核对，不在翻页中改变 |
| `order` | query，本流程固定 `DESC` |
| `Timezone-Offset` | 普通 header，必传，本配方固定 `-480`；本查询不发送日期筛选 |
| `searchTabEnumCode` | JSON body 数字，固定 0，查询当前用户权限下的可见协商；不代表租户全量权限 |
| `cooperationDisplayTabCode` | JSON body 数字，固定 0，全部视图，不限待确认/本人发起 |
| `keyword` | JSON body 字符串，来自 briefInfo 的编号，编号为空才用名称；始终发送非空值 |

不发送 `cooperationQueryStatusCodes` 等状态条件，避免排除已完成协商和已进入审批的合同。

从 `data.content[]` 同时匹配 `businessId=合同ID` 与 `cooperationId=关联协商ID`，读取匹配项的 `cooperationRoundId`。这些 ID 按原文比较，不把其他同名合同、`groupId` 或历史轮次当作匹配项。

### 分页

- 使用返回的 `data.pageIndex`、`data.pageSize`、`data.totalPages` 及存在时的 `data.paginationCapability` 校验本页。`totalElements` 仅是搜索结果总数，不是精确目标命中数。
- 有唯一精确匹配即结束。没有匹配且本页不是末页时，下一次仅增加 query `pageIndex`，保留 body、页容量、排序和 header。
- `paginationCapability.strategy=OFFSET_WINDOW` 时，下一页须同时小于 `totalPages` 和 `reachablePageCount`；读取实际返回的窗口上限，不硬编码可达页数。
- 正常结束条件是已读到 `totalPages` 对应末页，或返回 `totalPages=0` 且内容为空。不能仅因某页内容为空就断言已到末页。
- 达到窗口边界仍无匹配时，说明本次可访问窗口内未能定位；响应页码重复/倒退、分页字段缺失或矛盾、返回未知分页策略时，停止续页并报告定位未完成，不无界扫描或猜游标。
- 唯一匹配的轮次为空或非法时停止；多个匹配项出现不同轮次或关联时停止，不能把列表顺序当作选择规则。

## 3. 群 ID：chatGroupStatus

将以下对象写入 `cooperation-chat-group.json`。下例数字是虚构示例，执行前必须替换为步骤 2 返回的完整轮次 ID 原文：

```json
{"cooperationRoundIds":[9007199254741999]}
```

写入文件后的内容必须是合法 JSON，且数组仅有一个实际轮次的正整数。不要直接执行虚构示例，也不要把合同 ID 或协商 ID 填入轮次字段。

```sh
contract-cli api call --channel web POST /open-web/web/clm/cooperation/chatGroupStatus \
  --input-file cooperation-chat-group.json \
  --profile <profile> --as user --raw
```

无额外 query 或业务 header。请求体字段 `cooperationRoundIds` 必传，类型为 Long 数组，本流程固定一个真实当前轮次。空数组、null、0 和负数均不发送。

成功返回的相关字段：

| 字段 | 类型与解释 |
| --- | --- |
| `data.cooperationChatGroupStatusList` | 群状态数组，枚举字符串；取唯一轮次对应的第 0 项 |
| `data.openChatIds` | 飞书群 ID 字符串数组；第 0 项为非空时保留原文 |
| `data.currentUserInChatGroups` | 布尔数组；`false` 不得当作字段缺失，仅表述为“接口报告未在群”，不保证上游成员查询成功 |
| `data.chatGroupIds` | 内部群记录 ID 数组，不作为飞书群 ID 输出 |

迁移群分支可使数组长度不同。由于请求只有一个轮次，分别读取各数组的第 0 项，缺项保持未知，不要求所有数组等长，不用空字符串或 false 补齐。存在的字段必须是数组，且元素符合表中类型；否则报告结果格式异常，不按字符串索引取字符或强制转换类型。相关数组如果出现多于 1 项，不擅自配对，报告响应无法可靠对应本次唯一轮次。

### 状态解释

| 原始状态 | 输出与处理 |
| --- | --- |
| `EXIST_WITH_ROBOT` | 群存在且机器人在群；有非空 `openChatIds[0]` 时输出群 ID，缺失时说明接口未返回群 ID |
| `EXIST_WITHOUT_ROBOT` | 群存在但机器人不在群；有返回 ID 时输出，不声称机器人可读取消息 |
| `UNEXIST` | 当前轮次未绑定群；不自动建群。若同时出现非空群 ID，标记字段矛盾，不解释为有效当前群 |
| `DISMISSED` | 群已解散；有 ID 时仅标为历史绑定 ID |
| `UNKNOWN` | 当前无法确认群状态；可能是上下文或上游状态读取问题，空 ID 不能解释为未建群；有返回 ID 时连同“状态未知”展示 |
| 缺失或其他状态 | 保留原值及已返回 ID，说明状态未核实；不猜成存在或未建群 |

群状态是本次所查询轮次的系统快照；多个查询之间可能产生新轮次，不承诺跨请求原子快照。即使取得 ID，也不保证用户具有飞书群消息读取权限。本流程不额外调用飞书查询作验证。

## 错误与停止

- HTTP 403、业务码数字/字符串 `403` 或 `111519`（无权限访问）：说明无权限并停止本对象后续调用。不换身份、环境、入口或传入邀请凭据绕过。
- 授权缺失/失效：沿用当前 profile 的认证技能处理，不在本 Skill 自行循环登录或刷新。
- `WEB_OPERATION_NOT_REGISTERED`：报告失败的 method/path 与可用 trace，说明目标环境未开放该操作；由维护人按接口清单配置后再验收，不解释为合同没有群。
- `WEB_OPERATION_BLOCKED`、限流、超时、5xx、解析失败及其他非成功业务码：保留真实错误，停止当前步骤，不把已取得的关联数据包装为群查询成功，不叠加自动重试。
- 关联查询之间合同/协商对象发生变化、精确匹配失败、轮次不存在或返回矛盾时，说明当前关联未能稳定确认；保留已取得标识，不猜历史轮次，也不自动重跑整条链。

不输出请求中的敏感授权信息。trace 仅用于定位实际错误，不用虚构的 trace 或示例响应代替真实调用结果。
