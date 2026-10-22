# 调用通道与参数约定

## 环境和授权

使用用户本次已确认的目标环境；是否支持由当前 CLI/MCP 的实际配置、命令帮助和服务能力决定，不在 Skill 中绑定环境、版本或安装包。搜索到回查保持同一环境和用户身份；不支持目标环境时停止，不修改地址、复用其他环境凭证或静默切环境。

沿用同一环境、同一租户、同一用户授权。CLI 授权步骤按 [auth](../../auth/SKILL.md) 中实际支持的流程执行；授权文档不构成扩大环境范围的依据。无需用户在对话里提供 Token、Cookie、Session 或身份头。没有可用通道时说明缺少的能力及环境，不自行安装来源不明的包。

## 选择入口

先检查现有 MCP 的实际 schema 是否支持通用 Open Web 请求；否则检查 CLI `api call --help` 是否支持 `--channel web`、query 与输入文件。通道必须覆盖本场景的读取、写入与结果回查。能力检查不固定 CLI 版本，不因同名命令存在就认定兼容。

只有入口缺失或参数能力不兼容时才选择另一通道。业务拒绝、已发送请求或结果不确定时不换通道重放。[父组定位](assemblies.md#父组定位与读取入口) 是在同一通道选择已确认的分组关系、父组与成员读取配方，不是跨通道重试；其权限错误与写入停止条件仍然适用。交接定位补充、预检、写入和回查限本 Skill 列出的 9 个 Open Web 端点；一般合同定位委托已有搜索 Skill，父组缺失时仅增加 [编号定位配方](assembly-lookup.md)，不枚举其他接口；MCP 未提供的 raw_body 等参数不能自行添加。

## CLI 调用形式

使用已确认的 profile 和 user 身份；以下调用形式中的占位符须替换为本次实际值：

```sh
contract-cli api call --channel web --profile <已确认profile> --as user GET /open-web/web/clm/employee/info
contract-cli api call --channel web --profile <已确认profile> --as user --query 'keywords=接收人姓名' --query 'includeInvalid=false' GET /open-web/web/clm/employee/findEmployeesByKeywords
contract-cli api call --channel web --profile <已确认profile> --as user --input-file <请求文件.json> POST /open-web/web/clm/permission/handover
```

- GET 参数使用 query，不拼接未知 URL；重复 query 的实际编码按命令帮助处理。query 中的布尔值使用 true/false。
- POST 使用 UTF-8 JSON 文件，避免 shell 拼接用户文本。body 中整数类型码保持整数、布尔保持布尔，Long ID 从响应到请求保留十进制字符串；不要经 JavaScript Number 转换。MCP 如需 raw_body 保真，必须确认实际 schema 支持。
- 内部 employeeId、contractId、groupId、assemblyId 与开放 user_id 含义不同，不混用；不发送 tenantId、employeeId 等额外身份覆盖字段，业务请求中的 targetTenantId 仅是已核实接收人的租户。
- 所有接口预期下游 body 为 code=200 且 success=true，再读取 data。HTTP/通道失败与业务失败分别判断；CLI 只返回 body 时不编造外层 HTTP 状态。data 为空不是交接成功。
- 只保存必要的本次对象、写前快照和核对结果，不记录凭证或无关个人信息。

## 参数表统一口径

- “类型”表示业务／JSON 类型；query 在 URL 中最终编码成文本，例如 boolean false 编码为 `false`、integer 0 编码为 `0`。body 中布尔、整数不能改成带引号字符串。
- “本流程要求”是本 Skill 的请求约定，不等于后端 Controller/DTO 的必填注解。即使后端可省略或存在默认值，固定参数仍按表发送；未列出的可选参数不自行补传。
- 内部 ID 统一以正十进制字符串保存和发送（后端 Long，最大 9223372036854775807），不含空格、符号或小数。不混用 contractNumber、开放 user_id 和内部 employeeId。仅查询响应中的 assemblyId、previousId、nextId 可出现 0/null 等无关联标记，不能把这些值当实际目标 ID。
- 缺失、null、空数组和 0 分别判断，不靠真假值合并。必须形成确认清单／请求的 ID 或角色缺失时停止该对象写入；备注为“可为空”的响应字段仅表示可能缺失，不承诺有完整执行证据。
- 响应表中 `[*]` 路径表示从集合投影的结果；表中 array 是投影后的集合类型。例如 data[*].status 是整数状态集合，data[*].employeeId 是内部 ID 字符串集合。实际单条字段按其对象和语义读取，不把集合当一个 ID。
- 固定值只在文档确认的场景使用；未知枚举、未核实筛选结构或新的服务端校验错误不猜测补齐。文档列出所有本流程请求参数，不表示列出底层接口的全部可选参数。

## 搜索与交接的上下文一致性

复用搜索结果前先核对其环境、配置目录、profile、租户与 user 身份，与交接通道保持一致。profile 名相同不足以证明服务来源相同；MCP 与 CLI 混用时也要有可靠的同环境和同用户依据，不能靠相同姓名或编号推断。

只检查每个调用层的实际响应协议：9 个 Open Web 端点按本文件的 code=200/success=true 判断；已有搜索命令的业务码、响应包装和分页语义由搜索 Skill 说明，不能把 Web 成功规则套到开放平台搜索，也不能把搜索 page_token 用于合同组成员分页。上下文或 ID 映射无法确认时只说明缺口，不执行写入。
