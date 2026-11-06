# 页面标准读取的两步查询

运行入口统一为 `contract-cli api call --channel web`，保持同一已确认环境、租户、profile 和个人 user 授权。第一步复用已有页面标准 `contract/info`，直接返回当前轮次；第二步读取群状态，不依赖 briefInfo 增加字段。

## 共用规则

- 两步均要求 CLI 执行成功、响应是有效对象、业务 `code` 为数字/字符串 `200` 且 `success=true`。HTTP 200 或 CLI 退出码 0 不能代替业务判断。
- 合同、协商、轮次 ID 保留完整十进制原文。Long 不经过 JavaScript `Number` 或浮点数；轮次请求数字从可靠响应原文无损复制。
- 本流程用 `--raw` 作为后续步骤的管道输入，以保留 Long ID 原文，不是为常规展示绕过共享约定。Agent 解析及整理后按输出清单展示业务结果，不把原始详情直接交给用户；宿主解析工具会使长 ID 失真时，必须保留原文字串，不能采用舍入值。
- 请求体写入 UTF-8 JSON 文件，通过 `--input-file` 原文发送。本流程没有额外业务 header，不构造授权或可信身份覆盖头。
- `<profile>`、`<contract-id>` 等是需用当前真实上下文替换的位置，不按示例字面值执行。

## 1. 合同关联及当前轮次

```sh
contract-cli api call --channel web GET /open-web/web/clm/contract/info \
  --query 'contractId=<contract-id>' \
  --profile <profile> --as user --raw
```

| 参数/字段 | 类型、来源、校验及用途 |
| --- | --- |
| query `contractId` | 必传，用户确认的合同 ID，正 Long 原文 |
| `data.contractId` | 响应合同 ID，必须与输入相同 |
| `data.cooperationId` | 该合同当前关联的协商 ID，须为有效正 Long；不接受其他对象的关联 |
| `data.cooperationRoundId` | 合同详情中关联协商的当前轮次，序列化为字符串；须为有效正 Long，供步骤 2 使用 |
| `data.contractName`、`data.contractNumber` | 仅供展示，可为空，不作为搜索词或运行前提 |
| `data.contractStatusCode` | 排障时核对，默认不展示；不限制审批阶段或协商状态 |

字段直接位于 `data.contractId/cooperationId/cooperationRoundId`，不是 `data.contractInfo.*`。

只有 contractId 必填；本流程省略 taskInstanceId/noticeId，不开启自动生成合同名（generateContractName 默认 false），其他参数沿用页面接口默认。无 body、无额外业务 header。不自行增加 text=false/fullSwitch 之类没有公开的 query。

`data` 为空、类型不对、合同 ID 不一致或关联缺失时停止，不推导合同不存在/无权限/未建群。

### 页面读取行为与字段缺失

此接口按合同 AUTHORIZED 与页面功能权限读取，草稿另有页面原有的查看校验。当前轮次来自合同已关联 Cooperation 的当前指针，不按审批/归档阶段排除，不由客户端添加 COOPERATION_DATA 或参与者条件。

这是页面标准详情读取，不是纯轮次元数据接口：服务端可能初始化/导入正文、回填编辑器 ID，或生成编辑器访问凭证。只提取三个 ID 不会关闭这些页面行为；用户明确禁止这些行为时停止本链，不能宣称零副作用。

合同详情中的轮次字段缺失/null，或为 0、负数、非整数、超出 Long 范围时，输出“合同详情未返回可用的当前协商轮次，本次群查询未完成”，保留实际可用 trace。不能判断为未建群，也不临时合成字段。

不以 briefInfo、`contract cooperation get`、searchV2 或页面 cooperation/info 回退，不引入未上线的新字段契约。不保存/回显完整详情、正文、附件 URL 或编辑器访问凭证，只提取本业务需要的白名单字段。

## 2. 既有群绑定与状态

将下例写入 `cooperation-chat-group.json`。数字是虚构演示，必须替换为步骤 1 的真实 `cooperationRoundId` 完整原文；数组只有一个 JSON Long 数字，不加引号、不经过浮点数转换：

```json
{"cooperationRoundIds":[9007199254741999]}
```

```sh
contract-cli api call --channel web POST /open-web/web/clm/cooperation/chatGroupStatus \
  --input-file cooperation-chat-group.json \
  --profile <profile> --as user --raw
```

`cooperationRoundIds` 必传，每次一个真实当前轮次；不发送空数组/null/0/负数，不填合同 ID 或协商 ID。无额外 query 或业务 header。

服务端沿用页面群信息读取权限：同租户下，有效协商参与记录或关联合同可读；客户端不增加协商数据角色门槛，不自行放宽权限。服务端若明确拒绝，立即停止。

| 字段 | 类型和含义 |
| --- | --- |
| `data.cooperationChatGroupStatusList` | 群状态数组，第 0 项对应本次唯一轮次；缺失/null 为未核实 |
| `data.openChatIds` | 飞书群 ID 数组；字符串或 null 项，缺项/null/空字符串表示没有返回可用 ID |
| `data.currentUserInChatGroups` | 布尔或 null 数组；false 是接口报告值，不等于缺失；仅适用时展示 |
| `data.chatGroupIds` | 内部群记录 ID，不输出，也不据其值判断是否建群 |

各数组可长短不齐，允许空数组、字段为 null、相应项为 null；独立读取第 0 项，不补空字符串/false。非数组或元素类型异常时报告格式错误，不按字符串索引取字符；相关数组多于 1 项时不任意配对，停止确认唯一群绑定。

## 状态与页面权限的区别

页面的群状态决定“打开”“加入”、置灰或隐藏，这不等于额外数据角色权限。CLI 只返回有权读取的已有群信息，不执行打开/加入/创建动作。

| 状态 | 查询输出 |
| --- | --- |
| `EXIST_WITH_ROBOT` | 群存在且机器人在群；有非空 ID 时输出，缺 ID 则报告结果不完整 |
| `EXIST_WITHOUT_ROBOT` | 返回已有 ID，并说明机器人不在群、页面可能无法打开；不把页面不可操作解释为无数据读权限 |
| `UNEXIST` | 当前轮次未建群；不显示用户在群状态，不自动建群；非空群 ID 与状态矛盾时报告异常 |
| `DISMISSED` | 群已解散；有 ID 时只标历史绑定 ID，不展示用户在群状态 |
| `UNKNOWN` | 状态无法确认，包含权限不足、查询不到或上游状态读取失败等情形；不单凭该枚举断言某一种原因。空 ID 不等于未建群，有 ID 时与“状态未知”同时展示 |
| 缺失/其他状态 | 保留原始状态及实际 ID，说明未核实，不猜成存在或未建群 |

只有两种 `EXIST_*`、可靠群 ID 且返回布尔值时，补充“接口报告当前用户在群/未在群”。其他情况隐藏该字段。取得 ID 不保证用户具有飞书群消息权限，也不保证能直接打开群。

输出为本次所取得轮次的快照，不承诺两次请求原子一致。发现关联或轮次变化、数据矛盾时停止确认，不猜历史轮次，不自动重跑。

## 错误与完成边界

- HTTP 403 或明确权限类业务码（包括数字/字符串 403、110004、111519）立即停止，不换身份、租户、环境、路径或邀请凭据绕过。
- 未授权/过期沿用宿主公共认证 Skill，不自行循环登录、刷新或叠加重试。
- `WEB_OPERATION_NOT_REGISTERED`：说明目标环境尚未开放失败的精确 method/path，附真实可用 trace；不是未建群。
- 封禁、限流、超时、5xx、解析失败、未知非成功码：停止该步骤，不能把已取得的关联包装成群查询成功。
- 只有合同/关联/轮次均可靠、群状态响应业务成功且可解释时，报告本次群查询结果；缺字段、权限拒绝和状态未知保持各自含义。

不输出敏感授权信息，不虚构 trace 或结果，不主动调用建群、邀请、加入、办理审批、已读/参与状态写入或群消息接口；第一步页面标准详情本身的正常读取行为按上述口径处理。
