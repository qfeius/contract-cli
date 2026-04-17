# Contract Create Field Tree Reference

这份附录专门解决“某个字段在对象树里的什么位置、什么时候需要传、和哪些字段联动”。

阅读方式：

- 先看总览树，定位路径
- 再跳到对应 `Node: ...` 小节看详细字段说明
- 这里保留真实字段名，不做重命名

## 1. 总览树

```text
$
├── create_user_id
├── contract_category_abbreviation
├── contract_category_id
├── contract_name
├── contract_number
├── remark
├── source_id
├── generate_contract_name_by_rule
├── sign_type_code
├── pay_type_code
├── amount
├── currency_code
├── property_type_code
├── in_amount
├── in_currency_code
├── out_amount
├── out_currency_code
├── fixed_validity_code
├── start_date
├── end_date
├── contract_status_code
├── submitted_time
├── archive_number
├── archived_time
├── text_file_id
├── template_instance_id
├── contract_cause_file_id_list[]
├── attachment_file_id_list[]
├── scan_file_id
├── form
├── business_type_code
├── previous_contract_id
├── change_remark
├── termination_date
├── termination_date_type_code
├── termination_remark
├── our_party_list[]
│   ├── our_party_code
│   ├── docusign_signer_email
│   └── our_party_sign_info_resource
│       ├── auto_sign
│       ├── enable
│       ├── cross_page_seal_enabled
│       ├── date_seal_enabled
│       ├── keyword
│       ├── personal_seal_enabled
│       ├── personal_seal_keyword
│       ├── seal_auth_platform_code
│       ├── seal_auth_seal_id
│       ├── seal_position_type_code
│       ├── seal_type_codes
│       ├── signer_email
│       ├── signer_mobile
│       └── signer_name
├── counter_party_list[]
│   ├── counter_party_code
│   ├── bank_account_info.id
│   ├── bank_account_info.value
│   ├── business_address_info.id
│   ├── business_address_info.value
│   ├── contact_info.id
│   ├── contact_info.value
│   └── counter_party_sign_info_resource
│       ├── enable
│       ├── cross_page_seal_enabled
│       ├── date_seal_enabled
│       ├── keyword
│       ├── personal_seal_enabled
│       ├── personal_seal_keyword
│       ├── seal_position_type_code
│       ├── seal_type_codes
│       ├── signer_email
│       ├── signer_mobile
│       └── signer_name
├── counter_party_sign_short_url_list[]
│   ├── identityId
│   ├── name
│   └── shortUrl
├── payment_plan_list[]
│   ├── payment_amount
│   ├── payment_rate
│   ├── payment_condition
│   ├── payment_counter_party.counter_party_code
│   ├── payment_custom_attributes
│   ├── payment_date
│   ├── payment_date_type_code
│   ├── payment_desc
│   ├── payment_interval_days
│   ├── payment_reminder_matter
│   ├── payment_reminder_type_code
│   ├── payment_with_invoice
│   ├── prepaid
│   ├── need_check
│   ├── currency_code
│   ├── interval_day_type_code
│   └── source_id
├── collection_plan_list[]
│   ├── collection_amount
│   ├── collection_counter_party.counter_party_code
│   ├── collection_custom_attributes
│   ├── collection_date
│   ├── collection_date_type_code
│   ├── collection_desc
│   ├── collection_interval_days
│   ├── collection_reminder_matter
│   ├── collection_reminder_type_code
│   ├── currency_code
│   ├── interval_day_type_code
│   └── source_id
├── contract_budget_list[]
│   ├── budget_code
│   ├── budget_code_name
│   ├── budget_department_id
│   ├── budget_department_info
│   ├── budget_department_name
│   ├── budget_occupied_amount
│   ├── budget_occupied_amount_currency
│   ├── budget_subject_code
│   ├── budget_subject_info
│   ├── budget_subject_name
│   ├── budget_taxed_amount
│   ├── budget_taxed_amount_currency
│   ├── budget_year
│   ├── cost_center_code
│   ├── cost_center_group_code
│   ├── cost_center_group_info
│   ├── cost_center_group_name
│   ├── cost_center_info
│   ├── cost_center_name
│   ├── extra_info
│   ├── remark
│   ├── source_id
│   ├── tax_amount
│   ├── tax_amount_currency
│   ├── tax_rate
│   └── uuid
├── relation_list[]
│   ├── relation_name
│   └── contract_numbers[]
└── attribute_permission_list[]
    ├── attribute_key
    └── permission_type_code
```

## 2. Node: $

根对象是创建合同请求体本身。下面按分组列出全部顶层字段。

### 2.1 基础与分类

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `create_user_id` | `string` | 条件必填 | 合同申请人 id。 | `--as bot` 时必填；这是请求体字段，不是 query 参数。 |
| `contract_category_abbreviation` | `string` | 必填 | 合同分类缩写。 | 创建合同最核心的分类定位字段。 |
| `contract_category_id` | `string` | 可选 | 合同分类 id。 | 分类缩写有歧义时，用它进一步唯一定位。 |
| `contract_name` | `string` | 必填 | 合同名称。 | 若 `generate_contract_name_by_rule=true`，名称可能由平台规则生成。 |
| `contract_number` | `string` | 条件必填 | 合同编号。 | `contract_status_code=9` 时必填；`business_type_code=2/3` 时通常不填。 |
| `remark` | `string` | 可选 | 合同说明。 | 纯备注，不影响模式分支。 |
| `source_id` | `string` | 可选 | 外部系统合同 id。 | 适合做外部系统与平台的映射。 |
| `generate_contract_name_by_rule` | `boolean` | 可选 | 是否按规则自动生成合同名称。 | 默认不自动生成。 |
| `sign_type_code` | `integer` | 可选 | 签约类型 code。 | 当前默认语义是 `1`；更细值见枚举附录。 |

### 2.2 金额与计价

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `pay_type_code` | `integer` | 必填 | 收支类型编码。 | 直接决定金额字段该怎么传；见枚举附录。 |
| `amount` | `number` | 条件必填 | 合同总金额。 | `pay_type_code=1/2` 时通常必填。 |
| `currency_code` | `string` | 条件必填 | 合同总金额币种。 | `pay_type_code=1/2` 时通常必填。 |
| `property_type_code` | `integer` | 条件必填 | 计价方式编码。 | `pay_type_code=1/2` 时通常必填。 |
| `in_amount` | `string` | 条件必填 | 收入金额。 | `pay_type_code=3` 时使用。 |
| `in_currency_code` | `string` | 条件必填 | 收入币种。 | `pay_type_code=3` 时使用。 |
| `out_amount` | `string` | 条件必填 | 支出金额。 | `pay_type_code=3` 时使用。 |
| `out_currency_code` | `string` | 条件必填 | 支出币种。 | `pay_type_code=3` 时使用。 |

### 2.3 期限与状态

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `fixed_validity_code` | `integer` | 必填 | 期限模式编码。 | `fixed_validity_code=1` 时通常要同时传 `start_date` 和 `end_date`。 |
| `start_date` | `string` | 条件必填 | 合同开始日期。 | 固定期限模式常用，格式 `yyyy-MM-dd`。 |
| `end_date` | `string` | 条件必填 | 合同结束日期。 | 固定期限模式常用，格式 `yyyy-MM-dd`。 |
| `contract_status_code` | `string` | 条件必填 | 合同状态编码。 | `business_type_code=2/3` 时必填，且只允许 `0` 或 `30`。 |
| `submitted_time` | `string` | 可选 | 提交时间。 | 毫秒级时间戳字符串。 |
| `archive_number` | `string` | 可选 | 归档编号。 | 归档场景相关。 |
| `archived_time` | `string` | 可选 | 归档时间。 | 毫秒级时间戳字符串。 |

### 2.4 正文、模板、附件与表单

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `text_file_id` | `string` | 条件必填 | 正文文件 id。 | `contract_status_code=0/30` 时通常必填；传 `template_instance_id` 时通常不再传。 |
| `template_instance_id` | `string` | 条件必填 | 模板实例 id。 | 模板模式使用；`business_type_code=2/3` 时不允许填写。 |
| `contract_cause_file_id_list` | `array<string>` | 可选 | 合同附件文件 id 列表。 | 只能传平台文件 id。 |
| `attachment_file_id_list` | `array<string>` | 可选 | 其他附件文件 id 列表。 | 只能传平台文件 id。 |
| `scan_file_id` | `string` | 可选 | 归档扫描件文件 id。 | 只能传平台文件 id。 |
| `form` | `string` | 可选 | 合同表单。 | 类型是 JSON 字符串，不是对象。 |

### 2.5 业务模式与终止/变更

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `business_type_code` | `string` | 可选 | 业务类型编码。 | `2` 变更，`3` 终止；这两种模式都不允许传 `template_instance_id`。 |
| `previous_contract_id` | `string` | 条件必填 | 原合同 id。 | `business_type_code=2/3` 时必填。 |
| `change_remark` | `string` | 条件必填 | 变更原因。 | 只有 `business_type_code=2` 时必填。 |
| `termination_date` | `string` | 条件必填 | 终止日期。 | 终止场景使用，格式 `yyyy-MM-dd`。 |
| `termination_date_type_code` | `integer` | 条件必填 | 终止日期类型。 | 只有 `business_type_code=3` 时使用。 |
| `termination_remark` | `string` | 条件必填 | 终止原因。 | 只有 `business_type_code=3` 时必填。 |

### 2.6 复杂对象入口

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `our_party_list` | `array<object>` | 必填 | 我方主体列表。 | 详情看 `Node: $.our_party_list[]`。 |
| `counter_party_list` | `array<object>` | 必填 | 对方主体列表。 | 详情看 `Node: $.counter_party_list[]`。 |
| `counter_party_sign_short_url_list` | `array<object>` | 可选 | 对方签约短链列表。 | 详情看 `Node: $.counter_party_sign_short_url_list[]`。 |
| `payment_plan_list` | `array<object>` | 可选 | 付款计划列表。 | 主要用于支出类合同。 |
| `collection_plan_list` | `array<object>` | 可选 | 收款计划列表。 | 主要用于收入类合同。 |
| `contract_budget_list` | `array<object>` | 可选 | 预算明细列表。 | 详情看 `Node: $.contract_budget_list[]`。 |
| `relation_list` | `array<object>` | 可选 | 关联合同列表。 | 详情看 `Node: $.relation_list[]`。 |
| `attribute_permission_list` | `array<object>` | 可选 | 字段权限列表。 | 详情看 `Node: $.attribute_permission_list[]`。 |

## 3. Node: $.our_party_list[]

什么时候需要传：

- 任何创建合同请求都需要至少一个我方主体
- 如果需要电子签、自动签章、指定签字人，再继续展开 `our_party_sign_info_resource`

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `our_party_code` | `string` | 必填 | 我方主体编码。 | 用来定位我方主体。 |
| `docusign_signer_email` | `string` | 可选 | Docusign 我方签字人邮箱。 | 仅 Docusign 签约方式时有效。 |
| `our_party_sign_info_resource` | `object` | 可选 | 我方签章/签约配置对象。 | 详情看 `Node: $.our_party_list[].our_party_sign_info_resource`。 |

## 4. Node: $.our_party_list[].our_party_sign_info_resource

什么时候需要传：

- 需要我方电子签、自动签章、骑缝章、指定章型、指定签字人时
- 如果只是普通纸质合同且没有我方电子签配置，可以不传

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `auto_sign` | `boolean` | 可选 | 是否自动签署。 | `true` 自动签署，`false` 手动签署。 |
| `enable` | `boolean` | 条件必填 | 是否开启盖章。 | 进入签章配置对象后，通常先明确它。 |
| `cross_page_seal_enabled` | `boolean` | 可选 | 是否盖骑缝章。 | 常与电子签章场景一起出现。 |
| `date_seal_enabled` | `boolean` | 可选 | 是否支持日期印章。 | 日期章能力开关。 |
| `keyword` | `string` | 可选 | 签约关键字。 | `seal_position_type_code=0` 时常用。 |
| `personal_seal_enabled` | `boolean` | 可选 | 是否支持人名章。 | 与 `personal_seal_keyword` 配合使用。 |
| `personal_seal_keyword` | `string` | 可选 | 人名章关键字。 | 开启人名章时常用。 |
| `seal_auth_platform_code` | `integer` | 可选 | 签署服务。 | 已知值：`4` e签宝，`5` 法大大。 |
| `seal_auth_seal_id` | `string` | 可选 | 授权印章 id。 | 需要从电子签后台获取。 |
| `seal_position_type_code` | `integer` | 可选 | 印章位置类型。 | 具体值见枚举附录。 |
| `seal_type_codes` | `string` | 可选 | 印章类型编码串。 | 单选如 `3`，多选如 `5,1,3`。 |
| `signer_email` | `string` | 可选 | 签约人邮箱。 | 具体使用取决于签约渠道。 |
| `signer_mobile` | `string` | 可选 | 签约人手机号。 | 具体使用取决于签约渠道。 |
| `signer_name` | `string` | 可选 | 签约人姓名。 | 需要指定签署人时使用。 |

## 5. Node: $.counter_party_list[]

什么时候需要传：

- 任何创建合同请求都需要至少一个对方主体
- 如果需要对方签章/签约配置，再继续展开 `counter_party_sign_info_resource`

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `counter_party_code` | `string` | 必填 | 对方主体编码。 | 用来定位对方主体。 |
| `bank_account_info.id` | `number` | 可选 | 通过银行账户 id 选择账号。 | 与 `bank_account_info.value` 二选一使用更清晰。 |
| `bank_account_info.value` | `string` | 可选 | 通过银行账号文本选择账号。 | 适合拿不到账户 id 的场景。 |
| `business_address_info.id` | `number` | 可选 | 通过经营地址 id 选择地址。 | 与 `business_address_info.value` 二选一使用更清晰。 |
| `business_address_info.value` | `string` | 可选 | 通过经营地址文本选择地址。 | 适合拿不到地址 id 的场景。 |
| `contact_info.id` | `number` | 可选 | 通过联系人 id 选择联系人。 | 与 `contact_info.value` 二选一使用更清晰。 |
| `contact_info.value` | `string` | 可选 | 通过联系人姓名选择联系人。 | 适合拿不到联系人 id 的场景。 |
| `counter_party_sign_info_resource` | `object` | 可选 | 对方签章/签约配置对象。 | 详情看 `Node: $.counter_party_list[].counter_party_sign_info_resource`。 |

## 6. Node: $.counter_party_list[].counter_party_sign_info_resource

什么时候需要传：

- 需要配置对方签章位置、章型、签署人或关键字时
- 这是“对方签章/签约配置对象”，不是普通联系人字段

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `enable` | `boolean` | 条件必填 | 是否开启盖章。 | 进入签章配置对象后，通常先明确它。 |
| `cross_page_seal_enabled` | `boolean` | 可选 | 是否盖骑缝章。 | 对方需要骑缝章时开启。 |
| `date_seal_enabled` | `boolean` | 可选 | 是否支持日期印章。 | 日期章能力开关。 |
| `keyword` | `string` | 可选 | 签约关键字。 | `seal_position_type_code=0` 时常用。 |
| `personal_seal_enabled` | `boolean` | 可选 | 是否支持人名章。 | 与 `personal_seal_keyword` 配合。 |
| `personal_seal_keyword` | `string` | 可选 | 人名章关键字。 | 开启人名章时常用。 |
| `seal_position_type_code` | `integer` | 可选 | 印章位置类型。 | 具体值见枚举附录。 |
| `seal_type_codes` | `string` | 可选 | 印章类型编码串。 | 单选如 `3`，多选如 `5,1,3`。 |
| `signer_email` | `string` | 可选 | 签约人邮箱。 | 需要指定对方签署人时使用。 |
| `signer_mobile` | `string` | 可选 | 签约人手机号。 | 需要指定对方签署人时使用。 |
| `signer_name` | `string` | 可选 | 签约人姓名。 | 需要指定对方签署人时使用。 |

## 7. Node: $.counter_party_sign_short_url_list[]

什么时候需要传：

- 平台已经为对方签约生成了短链，需要把短链信息一起带入时

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `identityId` | `string` | 可选 | 相对交易方实体 id。 | 字段名是 camelCase，不要改成下划线风格。 |
| `name` | `string` | 可选 | 相对交易方实体名称。 | 纯展示/识别信息。 |
| `shortUrl` | `string` | 可选 | 签约链接。 | 字段名是 camelCase，不要改成下划线风格。 |

## 8. Node: $.payment_plan_list[]

什么时候需要传：

- 支出类合同需要拆付款计划时
- 付款日期、比例、验收、是否预付等都放在这里

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `payment_amount` | `number` | 条件必填 | 付款金额。 | `payment_reminder_type_code=0` 时必填。 |
| `payment_rate` | `number` | 可选 | 付款比例。 | 常与 `payment_amount` 一起使用。 |
| `payment_condition` | `string` | 可选 | 付款条件。 | 适合表达节点条件。 |
| `payment_counter_party.counter_party_code` | `string` | 可选 | 付款对象主体编码。 | 对象详情见下一节。 |
| `payment_custom_attributes` | `string` | 可选 | 付款计划自定义字段。 | 建议传 JSON 字符串。 |
| `payment_date` | `string` | 条件必填 | 付款日期。 | `payment_date_type_code=0` 时必填。 |
| `payment_date_type_code` | `integer` | 可选 | 付款时间类型。 | `0` 固定时间，`1` 不固定时间，`2` 履约条件完成后。 |
| `payment_desc` | `string` | 可选 | 付款描述。 | 适合写付款节点说明。 |
| `payment_interval_days` | `integer` | 条件必填 | 履约后间隔天数。 | `payment_date_type_code=2` 时必填。 |
| `payment_reminder_matter` | `string` | 可选 | 履约事项。 | 用于描述付款提醒触发点。 |
| `payment_reminder_type_code` | `integer` | 可选 | 履约类型。 | `0` 需要付款，`1` 无需付款。 |
| `payment_with_invoice` | `boolean` | 可选 | 是否有发票付款。 | 发票前置条件开关。 |
| `prepaid` | `boolean` | 可选 | 是否预付。 | 预付款场景使用。 |
| `need_check` | `boolean` | 可选 | 是否需要验收。 | 验收驱动付款时使用。 |
| `currency_code` | `string` | 可选 | 付款币种。 | 默认常见为 `CNY`。 |
| `interval_day_type_code` | `integer` | 可选 | 间隔日单位。 | `1` 工作日，`2` 自然日。 |
| `source_id` | `string` | 可选 | 原付款计划 id。 | 适合做来源映射。 |

## 9. Node: $.payment_plan_list[].payment_counter_party

什么时候需要传：

- 付款计划需要明确指向某个对方主体时

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `counter_party_code` | `string` | 可选 | 付款对象主体编码。 | 应与 `counter_party_list[].counter_party_code` 语义一致。 |

## 10. Node: $.collection_plan_list[]

什么时候需要传：

- 收入类合同需要拆收款计划时
- 收款日期、履约事项、收款币种等都放在这里

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `collection_amount` | `number` | 条件必填 | 收款金额。 | `collection_reminder_type_code=0` 时必填。 |
| `collection_counter_party.counter_party_code` | `string` | 可选 | 收款对象主体编码。 | 对象详情见下一节。 |
| `collection_custom_attributes` | `string` | 可选 | 收款计划自定义字段。 | 建议传 JSON 字符串。 |
| `collection_date` | `string` | 条件必填 | 收款日期。 | `collection_date_type_code=0` 时必填。 |
| `collection_date_type_code` | `integer` | 可选 | 收款时间类型。 | `0` 固定时间，`1` 不固定时间，`2` 履约条件完成后。 |
| `collection_desc` | `string` | 可选 | 收款描述。 | 适合写收款节点说明。 |
| `collection_interval_days` | `integer` | 条件必填 | 履约后间隔天数。 | `collection_date_type_code=2` 时必填。 |
| `collection_reminder_matter` | `string` | 可选 | 履约事项。 | 用于描述收款提醒触发点。 |
| `collection_reminder_type_code` | `integer` | 可选 | 履约类型。 | `0` 需要收款，`1` 无需收款。 |
| `currency_code` | `string` | 可选 | 收款币种。 | 默认常见为 `CNY`。 |
| `interval_day_type_code` | `integer` | 可选 | 间隔日单位。 | `1` 工作日，`2` 自然日。 |
| `source_id` | `string` | 可选 | 原收款计划 id。 | 适合做来源映射。 |

## 11. Node: $.collection_plan_list[].collection_counter_party

什么时候需要传：

- 收款计划需要明确指向某个对方主体时

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `counter_party_code` | `string` | 可选 | 收款对象主体编码。 | 应与 `counter_party_list[].counter_party_code` 语义一致。 |

## 12. Node: $.contract_budget_list[]

什么时候需要传：

- 合同需要挂预算、成本中心、税额、预算科目等信息时
- 补充协议场景也可能通过 `uuid` 回写同一预算项

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `budget_code` | `string` | 可选 | 预算编码。 | 预算定位字段。 |
| `budget_code_name` | `string` | 可选 | 预算编码名称。 | 预算展示字段。 |
| `budget_department_id` | `string` | 可选 | 预算部门 id。 | 与部门名称可同时传。 |
| `budget_department_info` | `string` | 可选 | 预算部门扩展信息。 | 常见为序列化 kv 字符串。 |
| `budget_department_name` | `string` | 可选 | 预算部门名称。 | 预算展示字段。 |
| `budget_occupied_amount` | `number` | 可选 | 预算占用金额。 | 不含税金额。 |
| `budget_occupied_amount_currency` | `string` | 可选 | 预算占用金额币种。 | 默认常见为 `CNY`。 |
| `budget_subject_code` | `string` | 可选 | 预算科目编码。 | 科目定位字段。 |
| `budget_subject_info` | `string` | 可选 | 预算科目信息。 | 常见为序列化 kv 字符串。 |
| `budget_subject_name` | `string` | 可选 | 预算科目名称。 | 科目展示字段。 |
| `budget_taxed_amount` | `number` | 可选 | 预算含税金额。 | 与税额相关。 |
| `budget_taxed_amount_currency` | `string` | 可选 | 预算含税金额币种。 | 默认常见为 `CNY`。 |
| `budget_year` | `string` | 可选 | 预算年份。 | 年度预算切片字段。 |
| `cost_center_code` | `string` | 可选 | 成本中心编码。 | 成本中心定位字段。 |
| `cost_center_group_code` | `string` | 可选 | 成本中心组编码。 | 成本中心组定位字段。 |
| `cost_center_group_info` | `string` | 可选 | 成本中心组信息。 | 常见为序列化 kv 字符串。 |
| `cost_center_group_name` | `string` | 可选 | 成本中心组名称。 | 展示字段。 |
| `cost_center_info` | `string` | 可选 | 成本中心信息。 | 常见为序列化 kv 字符串。 |
| `cost_center_name` | `string` | 可选 | 成本中心名称。 | 展示字段。 |
| `extra_info` | `string` | 可选 | 额外信息。 | 常见为序列化 kv 字符串，例如是否冻结。 |
| `remark` | `string` | 可选 | 预算备注。 | 不影响模式判断。 |
| `source_id` | `string` | 可选 | 原始预算明细 id。 | 适合做来源映射。 |
| `tax_amount` | `number` | 可选 | 税额。 | 税额字段。 |
| `tax_amount_currency` | `string` | 可选 | 税额币种。 | 默认常见为 `CNY`。 |
| `tax_rate` | `number` | 可选 | 税率。 | 单位是百分比。 |
| `uuid` | `string` | 可选 | 同一预算项唯一 id。 | 创建补充协议时可用于更新相同 `uuid` 的预算明细。 |

## 13. Node: $.relation_list[]

什么时候需要传：

- 需要把当前合同和其他合同建立关联时

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `relation_name` | `string` | 可选 | 关联合同字段。 | 说明这类关系是什么。 |
| `contract_numbers` | `array<string>` | 可选 | 关联合同编号列表。 | 这里传的是合同编号，不是合同 id。 |

## 14. Node: $.attribute_permission_list[]

什么时候需要传：

- 需要控制某些合同字段只读或可编辑时

| 字段 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `attribute_key` | `string` | 可选 | 字段属性 key。 | 用来定位具体字段。 |
| `permission_type_code` | `integer` | 可选 | 权限类型 code。 | 当前版本只支持 `2`（读权限，不可编辑）；`3` 表示写权限，可编辑。 |
