# 列出、筛选及定位合同

用户可以说“列出我的草稿”“找编号为…的草稿”“找名称含…的软件采购草稿”。不要求用户知道内部 ID，也不替用户新建合同。

需要批量页面办理时，发现结果交给 [系统页面入口](native-page-handoff.md) 逐项核实并生成链接。若本轮批量上传已返回精确合同 ID，直接复用该清单，不重新全量搜索；上传文件 ID 不能代替实际合同 ID。本人草稿列表不覆盖所有协商，不能将未命中的协商报告为不存在。

## USER 本人草稿

使用 `scripts/draft-discovery.mjs`。当前官方页面的草稿页按授权上下文的 tenant + owner 查询，不传用户或租户过滤值。读取范围是**当前用户本人负责的编辑中草稿**，不是其全部可见合同，也不是全部可提交业务。

扫描开始和结束均通过当前官方授权状态及 `employee/info` 核对具体租户/员工；中途换账号时拒绝把混合分页结果作为一份“我的草稿”列表输出，需重新只读查询。

```sh
node scripts/draft-discovery.mjs list /absolute/private/context.json '{"name":"软件采购"}'
```

这里的 `context.json` 由 Agent 在核对实际授权后写入私有工作目录，格式见[运行上下文](runtime.md)，不是让用户编写。脚本只读，没有提交或自动选择动作。

函数入口 `discoverDrafts(client, options)` 支持 `filters`、`pageSize`（1–30，默认 20）、`maxPages`（每个草稿页扫描上限，默认 100）、`displayOffset` 和 `displayLimit`。展示分页不影响已扫描范围。

命令行第四个参数可传扫描选项，例如 `'{"maxPages":200,"displayOffset":20,"displayLimit":20}'`。默认100页是本次Skill的资源预算，不是已证实的草稿服务端上限；用户要求更多结果时可在同一已授权范围增加预算重新完整扫描，并保留并发变化/不完整说明。不假造草稿续页token，也不把普通ES搜索的10000窗口套给缺少此元数据的草稿列表。

| 筛选键 | 用户含义和匹配方式 |
|---|---|
| `number` | 合同或合同组编号，完整字符串精确匹配 |
| `name` | 名称包含输入文本，区分大小写 |
| `kind` | 单合同 `single`、合同组 `group`、模板批量 `template_batch` |
| `businessTypeCode` | 用户选择的业务类型，由返回的类型名称和真实代码映射；不让用户填代码 |
| `sourceTypeCode` | 用户选择的发起来源，由真实来源名称和代码映射 |
| `templateId` | 已解析的模板内部 ID；名称重名须先澄清，不能当成草稿名称 |
| `categoryId` | 已解析的合同分类 ID |
| `id` | 已有授权详情/列表产生的内部 ID，按字符串传入 |

多条件采用 AND。若用户提出金额、日期、对方主体、负责人或自定义字段等当前适配未提供的条件，明确本次不能据此精确筛选；可以展示已知候选并保留该条件待页面核对，不能悄悄丢掉条件后宣称匹配，也不为补条件调用完整详情。本脚本拒绝未支持的键。查询结果缺少参与筛选的字段时列入 `unresolved`，不当作不匹配。

## 请求和结果

向 `/open-web/web/clm/es/search/contractList` 发 POST：

```json
{"pageIndex":0,"pageSize":20,"searchTabCode":2,"conditionUnits":[],"filterUnits":[]}
```

`searchTabCode=2` 为单合同草稿，`3` 为合同组/模板批量草稿。两者分别从第 0 页扫描。该页面分支不应用普通筛选条件，所以脚本仅在**已经按当前用户权限返回的本人草稿**内筛选，没有先取全租户再过滤。

只有业务响应 `success=true`、`code=200` 才读 `data`；每个 `data.content[]` 含一个 `searchContractListItems[]` 摘要。当前 tab 的完整计数是 `filteredTotalElements`，不是 `totalElements` 中其他 tab 的统计。父对象的 `contractId` 是 assembly ID，不能作为单合同 ID 使用；输出用 `contract:<id>` 和 `assembly:<id>` 区分。

计数缺失或变化、重复页/重复对象、页码异常、未到总数就短页、读取失败、扫描达到上限都会返回 `complete=false`、`matchedTotal=null`、`resolution=INCOMPLETE`；不得从部分结果得出“没有”或“唯一”，也不能自动选中仅有的一条候选。可以列出已知结果供用户选择；目标确定后，仍按已有协议校验精确对象、状态和权限。所有页完整、所有筛选已解析后：

- `NO_OWN_DRAFT_MATCH`：只代表当前本人口径内无匹配；用户给的是退回/撤回申请、协商或其他业务时转[场景协议](scenario-actions.md)，不要求用户另找一个草稿。
- `UNIQUE_CANDIDATE`：展示合同名称、编号、类型、环境和当前身份。已有请求已精确授权该对象时继续读取，模糊请求仍需目标确认；不因唯一结果自动发起审批。
- `MULTIPLE_CANDIDATES`：展示并让用户选择；不挑第一条或最近一条。

每个候选都须在生成链接前获取新鲜且安全的状态与相应入口依据。列表不等于编辑/提交权限。未知类型可显示原值，不能猜测动作路线。

### 空结果与不完整结果的说明

草稿列表业务成功、当前页结构和摘要校验通过，但 `filteredTotalElements` 缺失或为 null 时，记录 `DRAFT_TOTAL_MISSING`，其 `pageIndex` 和 `returnedCount` 说明缺计数发生在哪一页、该页实际返回几条。保留该页已验证、去重的候选，然后停止当前 tab，继续读取另一 tab；不盲目追加页、不扩展授权查询范围，也不以 `totalElements` 或分页窗口容量补造总数。数值格式非法单独记录 `DRAFT_TOTAL_INVALID`，按总数异常说明，不能当作零条。

`scans` 按单合同草稿（tab 2）和合同组/模板批量草稿（tab 3）分别保留：`successfulPages` 是收到业务成功响应的页数，`pages` 是通过列表页校验并保留候选的页数，`reportedTotal` 是最后已核的总数、从未取得时为 null，`observedCount` 是已验证去重的候选数。收到成功响应不等于该页已校验通过；后续页缺计数时，先前总数也不能证明这次已查全。`returnedCount=0` 只说明该页为空，不能代表整个 tab 或两个 tab 都为空。

对用户用业务语言说明，技术诊断保留在执行结果中。按实际证据选择以下口径：

| 已取得的事实 | 对用户的说明及下一步 |
| --- | --- |
| 对应范围本次已校验页面均为空、存在总数缺失，且没有其他请求失败 | “本次单合同草稿查询未返回记录，但系统没有提供完整计数，暂时不能确认是否已查全。”查询的是组草稿时替换范围名称；可建议提供合同编号继续定位，或到系统对应草稿页面核对 |
| 已保留符合筛选条件的候选，但总数缺失 | “本次已找到 X 份符合条件的草稿；系统未提供完整计数，结果可能不完整。”X 取实际已观察匹配数；只展示其中一部分时另说明展示数量，不把 X 当作全部匹配数 |
| 已返回合同，但当前条件没有匹配，或者筛选字段缺失 | 分别说明“在已取得结果中暂未找到符合条件的草稿，查询尚未完整”或“部分合同缺少筛选所需信息，暂不能确认是否匹配”；保留待核实候选，不能说接口未返回合同或用户名下无草稿 |
| 另一 tab 成功、本 tab 请求失败或其他校验异常 | 分别说明各范围已取得的结果与失败原因，继续展示可靠候选；不把本 tab 的失败解释为空列表，也不将整个查询统一描述为失败 |
| 两个 tab 均完整、所有筛选已解析且无匹配 | “在本次本人负责的编辑中草稿范围内，没有找到符合条件的合同。”不能扩大为全系统不存在；其他业务对象按原场景协议继续定位 |

只有未完整的查询才提示结果不完整；不要对完整空结果附加“系统缺少总数”，也不要仅凭 `items=[]` 判断无合同，因为它还可能来自展示分页、筛选未匹配或字段待核实。批量链接输出格式统一见[主流程](../SKILL.md#3-在聊天中交付链接)。

## USER 本人协商定位

本人草稿或已核普通查询未命中，或者已知输入来自协商时，在用户请求范围内继续查 MY 协商；不是要求用户换一个合同。已知精确合同 ID 则优先安全的对象读取，不重复列表扫描。MY 仅当前参与范围，不等于所有可见协商；无匹配也不能宣称全系统不存在。

使用 `scripts/cooperation-discovery.mjs`，查询前后复核同一 tenantId/userId。可执行入口：

```sh
node scripts/cooperation-discovery.mjs list /absolute/private/context.json '{"keyword":"合同编号","filters":{"number":"合同编号"},"pageSize":20,"maxPages":100}'
```

函数 `discoverCooperations(client, options)` 的 `keyword` 是服务器关键词；`filters` 在当前USER已获权结果中以AND进一步筛选：`id`=协商ID、`businessId`=真实底层业务ID、`number`=完整合同编号、`name`=协商名称包含文本、`businessTypeCode`=业务类型、`cooperationStatusCode`=实际协商状态。技术ID/代码由Agent从真实结果解析，不让用户填写。名称匹配区分大小写，其他键精确匹配；未知字段列入unresolved。不使用列表筛选枚举代替实际协商状态。

还可传 `pageSize`（1–100）、`maxPages`（默认100）、`displayOffset`、`displayLimit`（默认20，最多100）和实际 `timezoneOffset`。额外筛选不能静默忽略。该脚本固定 MY，不开放身份、租户或ALL范围覆盖。

底层请求为：

```js
const result = await client.web({
  method: "POST", path: "/open-web/web/clm/cooperation/searchV2",
  query: {pageIndex: 0, pageSize: 20},
  headers: {"Timezone-Offset": new Date().getTimezoneOffset()},
  body: {keyword: userKeyword, searchTabEnumCode: 1, cooperationDisplayTabCode: 0},
  effect: "read"
});
```

userKeyword 是用户原编号或名称条件，不是拼接指令。Timezone-Offset 是当前业务时区偏移分钟，中国时区为 -480，不写成固定租户属性。searchTabEnumCode=1 是 MY，cooperationDisplayTabCode=0 为该范围全部展示，不静默改为 ALL，不传调用人/租户覆盖。响应检查 success=true、code=200 后读取 data；失败保留具体失败，不能解释为空。

- 从第0页按实际 content、totalElements、pageIndex/pageSize 逐页读取；默认20条/页，默认预算100页。短页或空页可能由关联对象失效产生，脚本在原总数页范围内继续，最终数量不足保留不完整；不因短页单独停止并宣称全部。总数变化、解析失败、重复对象或上限都标为部分结果，不能断言唯一/不存在。
- 关键词不是精确匹配保证。合同编号按完整字符串再匹配；名称按用户条件匹配。缺必要字段时复用候选的安全获权详情再核对；无安全读取则保留待核实，不能用不完整字段排除候选。其他筛选仍须满足，不能悄悄丢弃。
- 保留实际 businessId（底层业务ID）、cooperationId、cooperationRoundId、cooperationTypeCode、合同编号/实际名称、状态及真实 todoTaskId。先核底层对象族；外部协商工具的 businessId 不当 CLM 合同 ID。MY可含历史参与者及跨租户对象；仅 `isCooperationUser=true` 才是该返回时刻当前轮有效成员事实。默认userType不能替代它，createEmployeeVO为原始创建者，不当本轮创建者。
- 按当前获权快照核 ID、编号、名称和协商关联；关系、轮次不一致时刷新，不能混用旧轮角色。精确编号也有多对象时展示候选让用户选择。列表角色、readPermission 或 isCooperationUser 不能代替提交资格。
- 不为列链接调用 cooperation/info。缺少按钮规则时给当前协商的查看/核对入口，不编造 canComplete。路由与子状态见[系统页面入口](native-page-handoff.md)。

输出 `complete/resolution/items/unresolved/issues` 与草稿同义；`NO_MY_COOPERATION_MATCH`只表示本次MY条件内无匹配。`scans`保留页数、返回总数和实收数量，`observedAt`是本次观测时间。需要当前轮及合同负责人等事实时，按[窄查询协议](query-protocol.md)衔接；精确底层ID或任务上下文已有时不重新按名称挑对象。

## 服务端分页窗口

两个列表都兼容 paginationCapability 缺失/null 的旧响应；缺失不等于零条。若存在，必须校验 strategy=OFFSET_WINDOW，unit 为草稿 CONTRACT_GROUP / 协商 COOPERATION，offsetWindow、pageSize、reachablePageCount 是正安全整数，pageSize 与当前请求一致，reachablePageCount 等于 floor(offsetWindow/pageSize)。元数据无效或变化则停止继续翻页并声明不完整。MY协商另遵守已核10000偏移窗口，即使旧响应没有元数据，也不发送超过该窗口的页；草稿缺失元数据时不套用此限制。

可达条数为 reachablePageCount × pageSize，不能把它替代实际总数。只在页窗口内读取；达到窗口但仍有实际未读项时报告“服务端分页窗口限制，结果不完整”。用户条件可缩小协商检索范围时保留原条件重新查询；不能自行改搜索范围或把窗口内唯一当全局唯一。源码有该字段不证明当前部署返回它，以实际响应分支处理。

## APP 入口

本人的草稿和MY协商口径需要 USER 身份，不能在APP查询中伪造当前用户。只有APP上下文时，复用本轮已有、当前应用获权且对象关联明确的只读结果或真实页面入口，核实环境后提供查看链接，并注明浏览器个人办理权限仍由页面判断。本包不执行未适配为纯读的APP合同完整详情或通用搜索；没有可靠对象/入口时说明当前APP范围无法继续定位，不切USER、不索取Token，也不安排原系统改造。APP结果不能命名为“你的全部草稿”。
