---
name: contract-cli-contract-search
description: "使用 contract-cli 查找、搜索和筛选合同。用户说“查合同”“搜合同”“关键词搜索”“按条件找合同”“按编号快速定位”“合同文本包含某词”“按日期或状态筛选”“某人申请或需求的合同”“某部门、交易方或我方主体的合同”“按自定义字段筛选”，或使用 contract search/search-fields/search-v2 时触发。普通搜索词复现页面默认“全部”关键词；明确或可合理推测人员、部门、主体及自定义字段时保留候选发现和引导。已知合同 ID 的详情、正文读取和文件下载使用 contract-cli-contract。"
---

# contract-cli 合同搜索

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。本任务已读取且配置未变时不重复读取；复用已确认有效的授权、profile、身份和 `CONTRACT_CLI_CONFIG_DIR`，同一次查询与翻页使用相同配置。

## 搜索总纲

按固定顺序执行：**确认授权与身份 → 识别角色和字段 → 取得真实值并构造全部条件 → 查询 → 核对响应与分页**。先读本总纲，再按分支读取必要参考；下方场景是示例库，不按顺序逐个执行。

### 1. 确认授权，先选 user 或 app

复用本任务已确认有效的授权、profile 和配置目录。尚未确认当前身份的授权状态时，执行 `contract-cli auth status --profile contract --as user` 或已选定的 `--as app`；该命令不支持 `--output`，也不负责刷新 Token。缺失、过期或权限错误按 [授权 Skill](../auth/SKILL.md) 处理，不读取或展示原始 Token，不要求用户粘贴凭证。

普通用户查合同优先使用已授权 user；用户明确要求 app 或已建立 app 调用上下文时使用 app。两种身份都存在时也不依赖 profile 默认值，业务命令显式写 `--as`。已确定的身份不可因零结果、权限错误或能力不足而静默切换。

| 身份和能力 | 命令与必须按需阅读的文档 |
| --- | --- |
| user：当前用户可见合同；关键词、结构化条件、自定义字段、分页 | `contract search --as user`；[user 参数](references/search-user-parameters.md)，再读对应场景参考 |
| user：搜索字段及人员/部门候选 | `contract search-fields`、`employee list`、`department list`，均为 user-only；按后续分支读取对应 Skill |
| app V1：编号精确查询及旧组合/逻辑条件 | `contract search --as app`；[app V1 参数](references/search-app-parameters.md) |
| app V2：编号模糊或分号批量查询 | `contract search-v2 --as app`；[app V2 参数](references/search-v2-parameters.md) |

下述 `condition_units/filter_units` 配方仅用于 user MCP。app 不支持这套请求，也没有同等的 user 字段发现能力；app V2 的顶层编号还会覆盖其他条件。只有 app 授权时，先按 app 文档判断能否表达全部需求，不能套用 user JSON。接口差异详见 [搜索接口选择](references/search-contract-fields.md)。

### 2. 识别查询意图，按以下优先级分流

| 用户表达 | 优先处理方式 | 阅读入口 |
| --- | --- | --- |
| 有显式关键词意图，例如“关键词搜索毛鹏”“在页面搜索框搜里斯”“按全部搜索华东”，或“搜索/搜一下 X”只给了普通搜索词 | 直接使用 user 页面默认“全部”的十二字段同词 OR；即使词语像姓名、部门或公司，也不改成精确对象筛选 | [全部关键词](references/all-keyword-search.md) |
| 明确指定业务角色，例如“申请人是张三”“需求人是张洋”“申请部门是法务”“交易方为湖南安克” | 取得并消歧真实候选；优先该角色已支持的精确筛选，见下表 | [人员](../contract-cli-employee/SKILL.md)、[部门](../contract-cli-department/SKILL.md)、[交易方](../contract-cli-mdm-vendor/SKILL.md)、[我方主体](../contract-cli-mdm-legal/SKILL.md)，只读相关项 |
| 明确自定义字段，或给出了完整字段名和值，例如“自定义字段项目区域为华东”“项目区域为华东” | 先按展示名 `search-fields`，结合元数据与上下文确认字段，再解析值；唯一且含义明确时直接查 | [字段发现](references/search-filter-fields.md) |
| 明确名称、编号、合同文本、申请日期、状态、金额等固定字段 | 保留指定字段，不扩大为全部关键词 | 下方对应场景；[文本](references/text-search.md)、[申请日期](references/submission-date.md)、[组合](references/search-combinations.md) |
| 没有显式关键词意图，但表达可合理推测是人员、部门、交易方、我方主体或自定义字段，例如“张三的合同”“法务的合同”“区域为华东” | Agent 先查真实候选或字段元数据；仍有角色、字段或同名歧义时，用业务语言引导确认，不让用户找技术 ID | [引导式澄清](references/guided-search.md) |
| 其他带搜索词的请求，且没有足够依据识别业务角色或字段 | 先使用 user 页面默认“全部”关键词并说明范围；用户随后明确字段时再重构查询 | [全部关键词](references/all-keyword-search.md) |
| 只有“帮我查合同”而没有搜索词、字段或条件 | 只询问缺失的查询条件，不无条件读取全部合同 | [引导式澄清](references/guided-search.md) |

**显式关键词意图优先于实体外观；明确业务角色或引导确认后的对象优先精确筛选。** 不能仅因搜索词像姓名、部门或公司就改变用户选择的页面默认范围。取得 ID 也不等于任意字段都支持 ID。以下映射仅用于 user：

| 对象/角色 | 候选来源 → 合同筛选 | 当前边界 |
| --- | --- | --- |
| 申请人 | `employee list --name` → `filter_units.CONTRACT_SUBMIT_ID`＋user_id 数组 | “张三申请的”优先解析具体人员；重名只澄清身份。用户明确要求姓名关键词时才用场景 6 |
| 合同需求人 | `employee list --name` 或已核验详情候选 → `filter_units.CONTRACT_DEMAND_PERSON`＋user_id 数组 | 不用申请人或 PC 内部 employeeId 替代；[完整配方](references/demand-person.md) |
| 申请部门 / 需求人部门 | `department list --name` → 按角色使用 `DEPARTMENT_AUTHORITY` / `CONTRACT_DEMAND_PERSON_DEPARTMENT`＋部门 OPENID 数组 | “所属部门”名称关键词与这两个角色不能默认互换；不承诺排除子部门。自定义部门先发现字段 |
| 交易方 | `mdm vendor list --as user --name` → `filter_units.CONTRACT_TRADING_PARTY_ID`＋主体 ID 数组 | 不用名称/编码/label 冒充 ID 精确筛选；[需求人＋交易方配方](references/trading-party.md) |
| 我方主体 | `mdm legal list/get --as user` 取得候选并消歧；列表和详情已通过当前生产 user 授权实测 | 候选接口已具备；合同搜索当前公开 filter 是名称 TEXT，尚无已验证通用精确 ID 配方，不得虚构 `_ID` 或把名称匹配交付为主体 ID 精确筛选 |
| 归属人 / 创建人 | 有名称关键词路径，见 user 参数 | 不在当前普通人员精确 ID filter 白名单；不借用申请人、需求人或授权人员字段替代 |

用户要求精确我方主体等尚无可靠配方的条件时，保留条件并说明具体能力边界；可以核实接口契约或向用户确认是否接受名称匹配，但不能静默降级。`auth status` 也不提供可靠的当前业务 user_id；“我申请的”不能猜本人 ID 或用“我的合同”页签替代。推测对象的引导只用于补齐业务语义；候选不存在、证据不足或只是一个普通搜索词时，回到页面默认关键词，不虚构实体类型。

### 3. 自定义字段先发现，只有业务歧义才反问

- 用户明确说自定义字段且元数据唯一对应时，直接查询，不重复询问“是否自定义”。尚无同配置、同身份可复用元数据时，先执行 `contract search-fields --keyword "字段展示名" --profile contract --as user --output json`；不是 `search-fileds`。
- 用 `field_type`、展示名和业务含义判断系统字段还是自定义字段，再读取 `request_location/request_paths`、`search_field`、`filter_unique_key`、类型与候选。不能先猜自定义 key，再让用户确认技术值。多个合理匹配仍无法消歧时，可问：“你说的是系统合同编号，还是自定义字段里的合同编号？”
- 只有值、没有字段线索时，例如“区域那项是华东”但无法确定字段，先补齐业务字段；不把“华东”作为字段名枚举查询。已确认的字段元数据可以复用，字段失效或契约变化时刷新；仅用户要完整字段清单时才省略 keyword。
- 人员、部门、交易方及枚举候选由 Agent 查找并消歧，不默认要求用户找 ID。名称关键词、编号、金额、日期等系统字段不因“也叫字段”而一律先走自定义发现。

### 4. 最常用路径：页面默认关键词

用户只提供普通搜索词，或明确说“关键词搜索”“页面搜索框”“全部搜索”时，直接使用 [页面“全部”关键词配方](references/all-keyword-search.md)。该配方把同一个词放入页面实际请求的十二个 `condition_units` 并以 `SHOULD` 组合；无需先查询人员、部门、主体或自定义字段候选。

如果表达可合理推测是人员、部门、交易方、我方主体或自定义字段，但没有显式关键词意图，则先按 [引导式澄清](references/guided-search.md) 查询候选或字段元数据。明确业务角色后转为精确筛选；候选证据不足且用户仍只给了搜索词时，使用页面默认关键词并说明范围。

### 5. 构造请求，保留全部条件

默认 user 页签为全部可见合同（`search_tab_code=0`）；“我的合同”不等于“我申请的”。业务字段、值、匹配方式、AND/OR 与用户已确认范围全部保留。

结构化对象通常使用 `filter_units/MUST`；自定义字段按元数据指定位置，状态与申请日期走已验证的 `combine_condition` 路径，不统一塞入 filter。全部关键词是十二字段同词 SHOULD，页面合同文本是五类文本同词 SHOULD。生产已复现关键词单项 MUST 和姓名并列 MUST 的错误语义，不机械地把“必须包含”翻成 MUST；组合先读 [已验证配方](references/search-combinations.md)。姓名关键词＋独立状态的能力仍保留在 [姓名与状态](references/applicant-status.md)，只用于用户要求姓名关键词的情况。

“再加条件”保留旧条件，“改成”只替换目标条件；修改条件、范围或排序后清除旧 `page_token`，从第一页查。同查询续页仅换 token，见场景 9。复杂请求用 `--input-file`，简单请求可用 `--data`；不要用片段覆盖完整请求，CLI 编号/分页 flag 会覆盖 body 同名字段。

### 6. 核对结果后回复

按末尾“判断响应与翻页”同时检查退出码、业务 code/success、分页和组/条目口径。失败不等于零命中，未查完不报总量，组展开成员不保证逐条满足所有条件；不因零结果删条件、换身份或擅自扩大范围。

`contract get <id>`、`contract text <id>` 和文件下载交给 [合同 Skill](../contract-cli-contract/SKILL.md)；`contract text` 读取已知合同正文，不负责内容搜索。

## 用户身份搜索场景

以下场景均使用当前已授权用户的可见范围。选择与用户意图对应的场景，保留其全部条件；不要逐例执行。每个场景按“用户怎么说 → 应该怎么查 → 示例 JSON”就近展开。

完整请求示例需替换为用户实际值；标注为“模板”的示例必须先满足该场景的前置条件，不能原样执行。准备好请求后保存为 `search-user.json`，保留当前 `CONTRACT_CLI_CONFIG_DIR` 并执行：

```bash
contract-cli contract search --profile contract --as user --input-file search-user.json --output json
```

### 场景 1：按页面范围搜索合同文本

**1. 用户怎么说**

“查合同文本包含‘里斯’的合同，和页面一样。”

**2. 应该怎么查**

对正文、归档扫描件、其他附件、归档附件、合同附件使用同一关键词，五字段以 SHOULD 组合，任一来源命中即可。不要只查正文，也不额外搜索合同名称；返回结果时说明使用了页面文本范围。

页面选中“全部”时使用 [全部关键词配方](references/all-keyword-search.md)，范围还包含名称、编号、归属人、表单等，不能套用本场景的五字段 JSON。

**3. 示例 JSON**

```json
{
  "search_tab_code": 0,
  "page_size": 20,
  "condition_units": [
    {"search_field": "CONTRACT_SCAN_TEXT", "search_value": "里斯", "union_type": "SHOULD"},
    {"search_field": "CONTRACT_TEXT_FIELD", "search_value": "里斯", "union_type": "SHOULD"},
    {"search_field": "CONTRACT_ATTACHMENT_TEXT", "search_value": "里斯", "union_type": "SHOULD"},
    {"search_field": "CONTRACT_ARCHIVE_ATTACHMENT", "search_value": "里斯", "union_type": "SHOULD"},
    {"search_field": "CONTRACT_CAUSE_FIELD", "search_value": "里斯", "union_type": "SHOULD"}
  ]
}
```

### 场景 2：仅正文关键词与金额组合

**1. 用户怎么说**

“只查正文包含‘里斯’，金额人民币 1000～5000 元的合同。”

**2. 应该怎么查**

只搜索 `CONTRACT_TEXT_FIELD`；正文关键词用 SHOULD，金额范围和币种放在 `filter_units`，各自使用 MUST，三个条件同时满足。用户只要求正文关键词时，省略整个 `filter_units`，不擅自添加金额限制。

**3. 示例 JSON**

```json
{
  "search_tab_code": 0,
  "page_size": 20,
  "condition_units": [
    {"search_field": "CONTRACT_TEXT_FIELD", "search_value": "里斯", "union_type": "SHOULD"}
  ],
  "filter_units": [
    {"search_field": "CONTRACT_AMOUNT", "search_value": [1000, 5000], "union_type": "MUST"},
    {"search_field": "CONTRACT_CURRENCY", "search_value": ["CNY"], "union_type": "MUST"}
  ]
}
```

### 场景 3：我的合同、名称与审批状态组合

**1. 用户怎么说**

“在‘我的合同’中，查名称含‘采购’、正在审批的合同。”

**2. 应该怎么查**

使用“我的合同”页签 `search_tab_code=1`；名称和状态分别放入 `combine_condition.contract_name` 与 `contract_status_in`，同时限定。该组合路径已在全部可见页签做过生产正反对照，不将此扩大为“我的合同”样本验证。该页签不等于“仅我申请的”；用户说“我申请的”时，应按场景 6 限定申请人。

**3. 示例 JSON**

```json
{
  "search_tab_code": 1,
  "page_size": 20,
  "combine_condition": {"contract_name": "采购", "contract_status_in": "3"}
}
```

### 场景 4：按合同编号关键词搜索

**1. 用户怎么说**

“查编号包含 HT2026 的合同。”

**2. 应该怎么查**

在当前用户可见范围内使用顶层 `contract_number` 关键词条件。不切换到 app，也不把关键词结果声称为编号精确匹配。用户另有其他关键词条件时，先确认组合逻辑，不能机械并列后把 AND 变成 OR。

多个真实编号与正文同时筛选时，读 [批量编号组合](references/search-combinations.md#分号批量编号和正文同时满足)，不要把分号编号放入关键词数组后追加正文。

**3. 示例 JSON**

```json
{
  "search_tab_code": 0,
  "page_size": 20,
  "contract_number": "HT2026"
}
```

### 场景 5：合同申请日期与归档状态组合

**1. 用户怎么说**

“查合同申请日期在 2026 年 9 月 1 日到 15 日、现在已归档的合同。”

**2. 应该怎么查**

页面“合同申请日期”对应系统提交时间。按已确认业务时区，将范围设为 9 月 1 日 00:00:00 至 9 月 15 日 23:59:59，并同时限定已归档状态；只要求日期时省略状态。保留接口的 `submited` 拼写，不改用创建时间、归档时间或自定义日期；相对日期先换算并说明具体区间。

按日查询的生产对照及时间精度限制见 [申请日期说明](references/submission-date.md)。不要照抄页面毫秒 `filter_units`，也不因响应时间含毫秒就承诺毫秒级筛选。

**3. 示例 JSON**

```json
{
  "search_tab_code": 0,
  "page_size": 20,
  "combine_condition": {
    "submited_time_start": "2026-09-01 00:00:00",
    "submited_time_end": "2026-09-15 23:59:59",
    "contract_status_in": "9"
  }
}
```

### 场景 6：按申请人姓名关键词搜索

**1. 用户怎么说**

“按申请人姓名关键词‘张三’搜索合同。”

**2. 应该怎么查**

本场景仅用于用户明确要求姓名关键词匹配；“张三申请的”这类指定人员需求默认按总纲先解析候选、使用 ID filter。关键词方式使用 `condition_units.CONTRACT_SUBMIT_NAME`，将姓名作为字符串关键词，使用 SHOULD。服务端会按姓名关键词解析人员，CLI 无需先取得人员列表或 user_id；申请人不能替换成创建人、归属人或需求人。

用户明确说“合同需求人”时，改用 [需求人专用配方](references/demand-person.md)。当前没有需求人姓名专用查询参数，不能把本场景的申请人姓名结果当成需求人结果。

这是姓名关键词搜索，完整姓名也不能保证唯一人员，部分姓名可能匹配多人。返回时说明按姓名关键词查询；用户要求精确限定某个人时，先取得同租户、当前身份兼容的真实人员候选并消歧，再用 `filter_units.CONTRACT_SUBMIT_ID`、真实 `user_id` 字符串数组和 MUST 筛选，不能把姓名填入 ID 字段。用户说“我申请的”时，也需要可靠的当前用户 ID，不能只用“我的合同”页签代替；查历史或停用人员时，不把姓名无命中解释为没有历史合同。

申请人还需与正文或全部关键词同时满足时，先读 [申请人组合与 ID 来源](references/search-combinations.md#申请人和正文同时满足)，不直接并列姓名关键词和其他关键词。优先复用同租户、同身份已确认人员的 `submitter_user_id`；尚无可靠 ID 时按 [人员 Skill](../contract-cli-employee/SKILL.md) 调用 `employee list --name`，无需默认让用户提供 ID；姓名有歧义时只澄清业务身份。[全部关键词加申请人姓名](references/search-combinations.md#申请人姓名与全部关键词直接并列不能保证-and) 已实测出现无匹配姓名 MUST 被忽略，不能因请求成功就称为 AND。

只有申请人姓名关键词，再加独立状态筛选时，可以保留姓名 SHOULD，将状态写入 `combine_condition.contract_status_in`，无需先找 ID；该组合已有生产正反对照，见 [姓名与状态](references/applicant-status.md)。不要把这一结论用于姓名和其他关键词的并列 OR；用户限定具体重名人员时仍须消歧。

**3. 示例 JSON**

将“张三”替换为用户提供的姓名关键词即可查询，无需人员 ID。

```json
{
  "search_tab_code": 0,
  "page_size": 20,
  "condition_units": [
    {"search_field": "CONTRACT_SUBMIT_NAME", "search_value": "张三", "union_type": "SHOULD"}
  ]
}
```

### 场景 7：按部门名称关键词搜索

**1. 用户怎么说**

“按部门名称关键词‘法务’查合同。”

**2. 应该怎么查**

使用 `CONTRACT_DEPARTMENT_NAME` 查询部门名称关键词，说明这是名称关键词匹配，不能称为唯一部门的精确筛选。如果用户指定某个具体部门，先确认部门角色与真实候选，再按字段元数据构造 ID 筛选，不直接套用本例。

**3. 示例 JSON**

```json
{
  "search_tab_code": 0,
  "page_size": 20,
  "condition_units": [
    {"search_field": "CONTRACT_DEPARTMENT_NAME", "search_value": "法务", "union_type": "SHOULD"}
  ]
}
```

### 场景 8：按自定义选项字段搜索

**1. 用户怎么说**

“查项目区域为华东的合同。”

**2. 应该怎么查**

尚无可复用元数据时，先按字段展示名“项目区域”查询，再解析“华东”的真实选项值；同配置同身份下已确认的字段可直接复用：

```bash
contract-cli contract search-fields --keyword "项目区域" --profile contract --as user --output json
```

读取返回的 `request_location`、`request_paths`、`search_field`、`filter_unique_key`、`search_value_type`、`value_description`、`usage_hint`、`value_scopes` 和 `examples`。下例仅适用于元数据明确要求 `filter_units`、`CONTRACT_FORM_FIELDS_OPTION`，且“华东”的实际请求值为字符串“华东”的情况；其他字段类型、值类型或写入位置必须按元数据另行构造。多个匹配字段结合上下文仍无法消歧时才向用户确认；未找到时不能猜 key。完整规则见 [搜索字段发现](references/search-filter-fields.md)。

**3. 示例 JSON**

以下是条件模板，**不能原样执行**。只有上述元数据均已确认，并将 `REPLACE_WITH_FILTER_UNIQUE_KEY` 替换为该字段真实唯一 key 后才能使用。不要把显示名称当 key，也不要猜选项值。

```json
{
  "search_tab_code": 0,
  "page_size": 20,
  "filter_units": [
    {
      "search_field": "CONTRACT_FORM_FIELDS_OPTION",
      "filter_unique_key": "REPLACE_WITH_FILTER_UNIQUE_KEY",
      "search_value": ["华东"],
      "union_type": "MUST"
    }
  ]
}
```

### 场景 9：继续上一轮查询的分页

**1. 用户怎么说**

“刚才那些合同，继续下一页。”或“把剩余结果都列出来。”

**2. 应该怎么查**

恢复上一轮完整请求和最后一次成功响应。只有业务成功、`has_more=true` 且返回 token 有效未重复时继续；身份、配置目录、页签、页大小、关键词、筛选与排序均保持不变，只更新 `page_token`。用户要求列全时按同样规则继续到 `has_more=false`，无法恢复原查询时先确认，不另起一个示例查询。

**3. 示例 JSON**

以下是续页模板，**不能原样执行**：假设上一轮执行的是场景 3，并且响应表示还有下一页。将占位 token 替换为响应原值；若原查询不是场景 3，应复制实际原请求，不能使用这里的“采购”和审批状态。

```json
{
  "search_tab_code": 1,
  "page_size": 20,
  "combine_condition": {"contract_name": "采购", "contract_status_in": "3"},
  "page_token": "REPLACE_WITH_RETURNED_PAGE_TOKEN"
}
```

以上是请求构造示例，组合场景不等于已在当前账号实测成功；数量、业务成功和分页状态均以实际响应为准。用户要求文本“同时包含两个词”时，不把场景 1 复制两遍并列为 SHOULD；先确认可表达 AND 的服务端契约。

## 自定义字段与候选值

- 字段名、枚举值、人员和部门 ID 必须有来源。已知字段展示名、尚无同配置同身份可复用的元数据时，先执行 `contract search-fields --as user --keyword <字段展示名>`；字段失效或契约变化时刷新。只有用户明确要求完整字段清单时才省略 `--keyword`；保持与后续搜索相同的 profile、身份和配置目录。
- 自定义字段需要 `search_field` 与 `filter_unique_key`。按返回的 `request_location/request_paths` 放入 `combine_condition`、`condition_units` 或 `filter_units`，结合值类型、说明、候选范围和示例构造请求，不能统一塞入 `filter_units`。详见 [搜索字段发现](references/search-filter-fields.md)。
- 申请人姓名和部门名称关键词可直接用场景 6、7 的 `condition_units` 搜索；精确 ID 筛选使用 [人员 Skill](../contract-cli-employee/SKILL.md) 或 [部门 Skill](../contract-cli-department/SKILL.md) 取得并确认真实候选。
- `contract search-fields` 是本次开发版新增的 user-only 命令，对应 MCP 的 `list-contract-search-filter-fields`；旧 CLI 1.8.6 尚不支持，使用前确认本机版本或 `contract search-fields --help`。本次开发版另有顶层 `employee list` 与 `department list`，分别由独立 Skill 指导；旧版本先用对应 `--help` 确认可用性。`api call --channel web` 已开放为业务 Skill 的 Open Web 执行通道；本 Skill 的合同搜索和字段发现仍使用上述结构化命令，不自行改用页面接口。
- 自定义人员/部门字段要求 ID 时，分别调用 `employee list --name` 或 `department list --name`，按独立 Skill 处理分页和重名，再按字段元数据使用真实 ID。保留同配置、同身份及原有全部条件；查询失败不猜 ID、不删条件，也不默认让用户提供技术 ID。
- 保留人员/部门 ID 原值，不把数字字符串转成数值；部门候选也不能自行转换为内部 ID。本次开发版的 user 搜索固定 `user_id_type=user_id`，其他显式类型在发送搜索前报错；旧 CLI 1.8.6 会静默忽略覆盖值，不能声称 flag 已切换到 union_id。

## 判断响应与翻页

返回时概括实际范围、全部筛选条件、本次返回条目数及是否有后续页；总数只能来自服务端统计，或完整分页后的去重计数，并说明口径。

- 同时检查请求是否成功和业务响应的 `code` / `success`。本次开发版的 user 搜索对非零 code、`success=false` 或缺失业务 code 返回失败退出码，并保留响应；正式 CLI 1.8.6 的业务错误仍可能以退出码 0 返回。不能将业务失败解释为没有合同；app 响应继续按对应接口契约检查。
- 成功且 `data.items=[]` 才是该身份、范围和条件下无命中。说明实际查询范围，不直接断言系统里不存在相关合同。字段发现零匹配表示字段未找到；搜索零命中表示该条件下没有结果，两者分开解释。不得因零结果自动删除条件或放宽范围。
- 鉴权/权限错误按真实错误处理，保留原查询意图，不切换身份重试；组合能力不支持时解释具体限制，不显示“未找到合同”。
- 只在 `data.has_more=true` 时继续分页，原样传回 `page_token`，保持其他条件和身份不变。`has_more=false` 时即使 token 非空也停止；缺少或重复 token 时说明分页异常，避免循环。
- 没有查完时说明“已返回 N 条，还有后续页”；未返回总数时不从页大小、token 或本页条目数推算总量。
- 当前生产默认响应可只有 `items/has_more/page_token`。区分合同组数与合同条目数，条目数可能超过 `page_size`。有 `pagination.unit=CONTRACT_GROUP` 时按其口径读取统计；没有时，只能在完整分页且每条都有有效 `group_id` 后按唯一组 ID 统计，不能把空 ID 算作一组。具体实测和排序边界见 [申请日期说明](references/submission-date.md#结果计数与排序)。
- 命中合同组会展开相关成员，不能认定每个条目都同时满足全部条件。申请人＋变更中实测返回 3 组、5 条，其中 3 条变更中、2 条为同组审批中成员；按 [组与条目说明](references/applicant-status.md#结果按合同组解释) 展示，不给所有条目统一贴筛选状态。
- 默认展示合同名称、编号和必要状态。需要命中原文时再按用户要求读取相应合同文本/文件；搜索结果未提供命中片段时不编造引用。

响应字段按需见 [合同响应参考](../contract-cli-contract/references/contract-response-fields.md)，扩展字段是否可用以实际响应为准。
