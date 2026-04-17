# Contract Create Fields

这份文档是 `contract-cli contract create` 的主入口，优先解决两个问题：

- 我要创建哪一类合同，请直接抄哪种最小 JSON
- 某个复杂对象或枚举值到底怎么传，应该去看哪份附录

这套 skill 文档以后不依赖旧的 YAML 接口清单。`contract create` 的说明顺序固定为：

1. 先看本文件，选场景和最小请求体
2. 需要精确查嵌套对象时，看 [create-contract-field-tree.md](create-contract-field-tree.md)
3. 需要确认 code 取值时，看 [create-contract-enums.md](create-contract-enums.md)

## 1. CLI 命令面与硬约束

当前命令面：

```bash
contract-cli contract create \
  --profile contract-group \
  --input-file contract-create.json
```

或：

```bash
contract-cli contract create \
  --profile contract-group \
  --data '{"contract_name":"示例合同", ... }'
```

bot 示例：

```bash
contract-cli contract create \
  --profile contract-group \
  --as bot \
  --data '{"contract_name":"示例合同","create_user_id":"ou_xxx", ... }'
```

硬约束：

- `--as user` 走 `/open-apis/contract/v1/mcp/contracts`
- `--as bot` 走 `POST /open-apis/contract/v1/contracts`
- `--as bot` 时，请求体必须自己带 `create_user_id`
- `--input-file` 与 `--data` 互斥
- `--file` 不是请求体参数，它预留给后续真实文件上传命令
- CLI 只透传请求体，不做字段级本地校验，不自动补默认值
- `text_file_id`、`contract_cause_file_id_list`、`attachment_file_id_list`、`scan_file_id` 都必须传平台文件 id，不能传本地路径
- CLI 不自动创建模板实例；模板模式需要你先拿到 `template_instance_id`

## 2. 场景配方

### 2.1 文件正文创建

适用场景：

- 已经有正文文件 id
- 不走模板实例
- 常见状态是草稿或协商中

最小必填字段集：

- `create_user_id`（仅 `--as bot` 时必填）
- `contract_category_abbreviation`
- `contract_name`
- `our_party_list`
- `counter_party_list`
- `pay_type_code`
- `fixed_validity_code`
- `text_file_id`
- 如果 `pay_type_code=1/2`，通常还要给：
  - `amount`
  - `currency_code`
  - `property_type_code`
- 如果 `fixed_validity_code=1`，通常还要给：
  - `start_date`
  - `end_date`
- 如果正文处于草稿/协商中，通常还要给：
  - `contract_status_code`

常见不要传：

- `template_instance_id`
- 业务类型不是变更/终止时，不要传：
  - `previous_contract_id`
  - `change_remark`
  - `termination_remark`
  - `termination_date`
  - `termination_date_type_code`

最小示例：

```json
{
  "create_user_id": "ou_xxx",
  "contract_category_abbreviation": "PROCUREMENT",
  "contract_name": "示例采购合同",
  "our_party_list": [
    {
      "our_party_code": "OUR001"
    }
  ],
  "counter_party_list": [
    {
      "counter_party_code": "VENDOR001"
    }
  ],
  "pay_type_code": 2,
  "amount": 100000,
  "currency_code": "CNY",
  "property_type_code": 0,
  "fixed_validity_code": 1,
  "start_date": "2026-04-01",
  "end_date": "2027-03-31",
  "contract_status_code": "0",
  "text_file_id": "file_xxx"
}
```

### 2.2 模板实例创建

适用场景：

- 正文不是直接传文件，而是先在平台侧生成模板实例
- 想用模板变量和模板正文生成合同

最小必填字段集：

- `create_user_id`（仅 `--as bot` 时必填）
- `contract_category_abbreviation`
- `contract_name`
- `our_party_list`
- `counter_party_list`
- `pay_type_code`
- `fixed_validity_code`
- `template_instance_id`
- 如果 `pay_type_code=1/2`，通常还要给：
  - `amount`
  - `currency_code`
  - `property_type_code`
- 如果 `fixed_validity_code=1`，通常还要给：
  - `start_date`
  - `end_date`

常见不要传：

- `text_file_id`
- 一般不需要显式传 `contract_status_code`
- 业务类型不是变更/终止时，不要传：
  - `previous_contract_id`
  - `change_remark`
  - `termination_remark`
  - `termination_date`
  - `termination_date_type_code`

最小示例：

```json
{
  "create_user_id": "ou_xxx",
  "contract_category_abbreviation": "PROCUREMENT",
  "contract_name": "示例模板合同",
  "our_party_list": [
    {
      "our_party_code": "OUR001"
    }
  ],
  "counter_party_list": [
    {
      "counter_party_code": "VENDOR001"
    }
  ],
  "pay_type_code": 2,
  "amount": 100000,
  "currency_code": "CNY",
  "property_type_code": 0,
  "fixed_validity_code": 1,
  "start_date": "2026-04-01",
  "end_date": "2027-03-31",
  "template_instance_id": "tmpl_inst_xxx"
}
```

### 2.3 合同变更

适用场景：

- 创建补充协议
- 需要引用原合同

最小必填字段集：

- `create_user_id`（仅 `--as bot` 时必填）
- `business_type_code=2`
- `previous_contract_id`
- `change_remark`
- `contract_category_abbreviation`
- `contract_name`
- `our_party_list`
- `counter_party_list`
- `contract_status_code`
- 如果状态为草稿或协商中，通常还要给：
  - `text_file_id`

常见不要传：

- `template_instance_id`
- 通常不要传 `contract_number`

最小示例：

```json
{
  "create_user_id": "ou_xxx",
  "business_type_code": "2",
  "previous_contract_id": "contract_prev_xxx",
  "change_remark": "金额条款更新",
  "contract_category_abbreviation": "PROCUREMENT",
  "contract_name": "示例采购合同补充协议",
  "our_party_list": [
    {
      "our_party_code": "OUR001"
    }
  ],
  "counter_party_list": [
    {
      "counter_party_code": "VENDOR001"
    }
  ],
  "contract_status_code": "0",
  "text_file_id": "file_change_xxx"
}
```

### 2.4 合同终止

适用场景：

- 终止已存在的原合同
- 需要保留终止原因和终止日期语义

最小必填字段集：

- `create_user_id`（仅 `--as bot` 时必填）
- `business_type_code=3`
- `previous_contract_id`
- `termination_remark`
- `termination_date`
- `termination_date_type_code`
- `contract_category_abbreviation`
- `contract_name`
- `our_party_list`
- `counter_party_list`
- `contract_status_code`
- 如果状态为草稿或协商中，通常还要给：
  - `text_file_id`

常见不要传：

- `template_instance_id`
- 通常不要传 `contract_number`
- 不是变更场景时，不要传 `change_remark`

最小示例：

```json
{
  "create_user_id": "ou_xxx",
  "business_type_code": "3",
  "previous_contract_id": "contract_prev_xxx",
  "termination_remark": "双方协商一致终止",
  "termination_date": "2026-06-30",
  "termination_date_type_code": 1,
  "contract_category_abbreviation": "PROCUREMENT",
  "contract_name": "示例采购合同终止协议",
  "our_party_list": [
    {
      "our_party_code": "OUR001"
    }
  ],
  "counter_party_list": [
    {
      "counter_party_code": "VENDOR001"
    }
  ],
  "contract_status_code": "0",
  "text_file_id": "file_termination_xxx"
}
```

## 3. 字段树导航

下列复杂对象不要在这里硬背，直接去字段树附录按 JSON Path 查：

- 根对象 `$`
  看 [create-contract-field-tree.md](create-contract-field-tree.md) 里的 `Node: $`
- 我方主体与我方签章配置：
  - `$.our_party_list[]`
  - `$.our_party_list[].our_party_sign_info_resource`
- 对方主体与对方签章配置：
  - `$.counter_party_list[]`
  - `$.counter_party_list[].counter_party_sign_info_resource`
- 对方签约短链：
  - `$.counter_party_sign_short_url_list[]`
- 付款计划与付款对象：
  - `$.payment_plan_list[]`
  - `$.payment_plan_list[].payment_counter_party`
- 收款计划与收款对象：
  - `$.collection_plan_list[]`
  - `$.collection_plan_list[].collection_counter_party`
- 预算、关联合同、字段权限：
  - `$.contract_budget_list[]`
  - `$.relation_list[]`
  - `$.attribute_permission_list[]`

## 4. 枚举值导航

以下 code 型字段不要凭印象写，直接查枚举附录：

- `pay_type_code`
- `fixed_validity_code`
- `business_type_code`
- `contract_status_code`
- `termination_date_type_code`
- `seal_position_type_code`
- `seal_type_codes`

附录见：[create-contract-enums.md](create-contract-enums.md)

如果你想用 CLI 实时核对平台返回的枚举，也可以直接跑：

```bash
contract-cli contract enum list --profile contract-group --type contract_status_code
```

把 `contract_status_code` 换成目标枚举类型即可；当前接口支持的枚举类型列表见命令帮助。

## 5. 推荐阅读顺序

- 先按场景挑一套最小 JSON
- 再去字段树附录补齐复杂对象
- 最后去枚举附录确认 code 值
- 如果请求体很长，优先用 `--input-file`
