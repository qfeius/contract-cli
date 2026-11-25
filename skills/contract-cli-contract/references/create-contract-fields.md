# Contract Create Fields

这份文档是 `contract-cli contract create` 的主入口，优先解决两个问题：

- 我要创建哪一类合同，请直接抄哪种最小 JSON
- 某个复杂对象或枚举值到底怎么传，应该去看哪份附录

这套 skill 文档以后不依赖旧的 YAML 接口清单。`contract create` 的说明顺序固定为：

1. 先看 [category-fields.md](category-fields.md)，查询当前身份可用的合同分类简称
2. 再看本文件，选场景和最小请求体
3. 需要精确查嵌套对象时，看 [create-contract-field-tree.md](create-contract-field-tree.md)
4. 需要确认 code 取值时，看 [create-contract-enums.md](create-contract-enums.md)

## 1. CLI 命令面与硬约束

当前命令面：

```bash
contract-cli contract create \
  --profile contract \
  --input-file contract-create.json
```

或：

```bash
contract-cli contract create \
  --profile contract \
  --data '{"contract_name":"示例合同", ... }'
```

app 示例：

```bash
contract-cli contract create \
  --profile contract \
  --as app \
  --data '{"contract_name":"示例合同","create_user_id":"ou_xxx", ... }'
```

硬约束：

- `--as user` 走 `/open-apis/contract/v1/mcp/contracts`
- `--as app` 走 `POST /open-apis/contract/v1/contracts`
- `--as app` 时，请求体必须自己带 `create_user_id`
- `--input-file` 与 `--data` 互斥
- `--file` 不是 JSON 请求体参数；它只用于 `contract upload-file` 真实二进制上传
- CLI 只透传请求体，不做字段级本地校验，不自动补默认值
- `text_file_id`、`contract_cause_file_id_list`、`attachment_file_id_list`、`scan_file_id` 都必须传平台文件 id，不能传本地路径
- CLI 不自动创建模板实例；模板模式需要你先拿到 `template_instance_id`

### 1.1 `contract_category_abbreviation` 的来源

`contract_category_abbreviation` 不是固定枚举，也不能根据分类名称猜测。创建合同前，必须用与创建命令相同的 `--profile` 和 `--as` 查询合同分类：

```bash
# 后续使用 --as user 创建
contract-cli contract category list --profile contract --as user --lang zh-CN

# 后续使用 --as app 创建
contract-cli contract category list --profile contract --as app --lang zh-CN
```

从返回的分类树中选择当前身份可用的末级分类。二级分类的常见读取路径是：

```text
data.contract_category_resource_vo.category_resources[].children[].abbreviation
```

将返回的 `abbreviation` 原样写入创建请求体：

```json
{
  "contract_category_abbreviation": "CATEGORY_ABBREVIATION_FROM_LIST"
}
```

`CATEGORY_ABBREVIATION_FROM_LIST` 是文档占位值，执行前必须替换成分类查询的真实返回值。不要使用以下字段代替：

- `name`：仅用于展示分类名称
- `number`：常用于模板列表的 `category_number`
- `id` / `contract_category_id`：可以作为辅助信息，但当前创建链路仍要求 `contract_category_abbreviation`

若返回 `code=111717`、`msg="合同分类简称参数非法"`，优先检查：

- 是否遗漏了 `contract_category_abbreviation`
- 是否误用了示例占位值、分类名称、分类编号或分类 id
- 是否选择了非末级分类
- 该分类是否属于当前租户，并且对创建命令使用的身份可见

### 1.2 填写所属项目

创建普通合同（业务类型 `0`）或补充协议（业务类型 `2`）时，可以在创建请求体根对象中加入 `project_changes`。使用与创建命令相同的 `--profile` 和 `--as` 查询可选项目，把选中项目的 `project_id` 原样填入 `add_project_ids`。

```bash
contract-cli contract project search --profile contract --as user --keyword 采购 --output table
contract-cli contract project search --profile contract --as app --keyword 采购 --output table
```

终端表格每行展示项目名称、状态、所属部门、项目周期和项目 ID。由 Agent 协助填写时，读取默认 JSON，将名称、状态和用于区分的部门、周期列给用户，可用序号确认一个或多个项目，再从对应的 `data.content[].project_id` 取值。

```json
{
  "project_changes": {
    "add_project_ids": ["9007199254740993"]
  }
}
```

将这段字段合并到本次创建的完整 JSON，并用实际查询返回的项目 ID 替换示例值。数组支持多个项目，ID 使用字符串。文件和模板场景均在最终创建合同时填写此字段。

- 应用身份（`--as app`）可选择该应用获准访问的租户项目，创建请求还需提供 `create_user_id`。
- 用户身份（`--as user`）可选择当前用户有查看权限的项目。
- 选中流程需启用所属项目，且字段在当前表单可见、可填写。字段不可填写时，含项目 ID 的请求会被拒绝。
- 省略 `project_changes` 或提供空增量表示此次创建没有新增项目。草稿可以先留空，流程规定必填时，提交或直接归档需要有效的项目关联。

## 2. 场景配方

### 2.1 文件正文创建

适用场景：

- 已经有正文文件 id
- 不走模板实例
- 常见状态是草稿或协商中

最小必填字段集：

- `create_user_id`（仅 `--as app` 时必填）
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
  "contract_category_abbreviation": "CATEGORY_ABBREVIATION_FROM_LIST",
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

- `create_user_id`（仅 `--as app` 时必填）
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
  "contract_category_abbreviation": "CATEGORY_ABBREVIATION_FROM_LIST",
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

- `create_user_id`（仅 `--as app` 时必填）
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
  "contract_category_abbreviation": "CATEGORY_ABBREVIATION_FROM_LIST",
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

- `create_user_id`（仅 `--as app` 时必填）
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
  "contract_category_abbreviation": "CATEGORY_ABBREVIATION_FROM_LIST",
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
- 预算、关联合同、所属项目、字段权限：
  - `$.contract_budget_list[]`
  - `$.relation_list[]`
  - `$.project_changes`
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
contract-cli contract enum list --profile contract --type contract_status_code
```

把 `contract_status_code` 换成目标枚举类型即可；当前接口支持的枚举类型列表见命令帮助。

## 5. 推荐阅读顺序

- 先查询当前身份可用的合同分类，取得末级分类 `abbreviation`
- 再按场景挑一套最小 JSON，并替换 `CATEGORY_ABBREVIATION_FROM_LIST`
- 再去字段树附录补齐复杂对象
- 最后去枚举附录确认 code 值
- 如果请求体很长，优先用 `--input-file`
