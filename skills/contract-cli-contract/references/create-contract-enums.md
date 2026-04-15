# Contract Create Enums Reference

这份附录专门解决 `contract create` 里的 code 型字段和值域问题。

使用建议：

- 写请求体前，先来这里确认值域
- 如果目标环境后续扩展了更多枚举，也可以再用 `contract-cli contract enum list --type <enum-type>` 做运行时核对
- 这里优先记录当前已明确可直接使用的值

## 1. `pay_type_code`

| 值 | 含义 | 联动/注意 |
| --- | --- | --- |
| `1` | 收入类 | 通常需要 `amount`、`currency_code`、`property_type_code`，可配 `collection_plan_list`。 |
| `2` | 支出类 | 通常需要 `amount`、`currency_code`、`property_type_code`，可配 `payment_plan_list`。 |
| `3` | 既收又支 | 通常改传 `in_amount/in_currency_code/out_amount/out_currency_code`。 |
| `4` | 无金额 | 通常不再传金额字段。 |

## 2. `fixed_validity_code`

| 值 | 含义 | 联动/注意 |
| --- | --- | --- |
| `0` | 无固定期限 | 一般不强制要求同时给 `start_date` 和 `end_date`。 |
| `1` | 固定期限 | 通常要同时传 `start_date` 和 `end_date`。 |
| `2` | 未选择 | 不建议在最终请求体里长期保留这种中间态。 |
| `3` | 不固定生效日期 | 适合起始日期暂不确定的场景。 |
| `4` | 不固定终止日期 | 适合终止日期暂不确定的场景。 |

## 3. `business_type_code`

| 值 | 含义 | 联动/注意 |
| --- | --- | --- |
| `0` | 合同申请 | 普通创建场景，默认值。 |
| `2` | 合同变更 | 必填 `previous_contract_id`、`change_remark`、`contract_status_code`；不允许传 `template_instance_id`。 |
| `3` | 合同终止 | 必填 `previous_contract_id`、`termination_remark`、`termination_date`、`termination_date_type_code`、`contract_status_code`；不允许传 `template_instance_id`。 |

## 4. `contract_status_code`

| 值 | 含义 | 联动/注意 |
| --- | --- | --- |
| `0` | 草稿 | 文件正文模式常见；通常需要 `text_file_id`。 |
| `9` | 已归档 | 这时 `contract_number` 通常必填。 |
| `30` | 协商中 | 文件正文模式常见；通常需要 `text_file_id`。 |

## 5. `termination_date_type_code`

| 值 | 含义 | 联动/注意 |
| --- | --- | --- |
| `1` | 固定日期 | 通常与明确的 `termination_date` 一起使用。 |
| `2` | 签订即终止 | 终止日期语义跟“签订动作”绑定。 |

## 6. `seal_position_type_code`

| 值 | 含义 | 联动/注意 |
| --- | --- | --- |
| `0` | 关键字 | 常与 `keyword` 搭配，按关键字定位印章位置。 |
| `1` | 拖拽 | 说明位置来自拖拽定位。 |
| `255` | 未指定 | 当前资料里的保留值；如果要精确定位，通常不要停留在这个值。 |

## 7. `seal_type_codes`

字段类型说明：

- 它的类型是 `string`
- 单选时常见写法是 `"3"`
- 多选时常见写法是 `"5,1,3"`

当前已明确可直接使用的值：

| 值 | 含义 | 联动/注意 |
| --- | --- | --- |
| `0` | 无 | 可理解为不指定具体印章类型。 |
| `1` | 公章 | 常见通用印章。 |
| `2` | 合同专用章 | 合同签署场景常见。 |
| `3` | 财务专用章 | 财务相关场景常见。 |
| `5` | 人事专用章 | 人事相关场景常见。 |
| `12` | 法定代表人章 | 法定代表人签章场景。 |
| `20` | 自定义章 | 需要结合实际印章配置使用。 |

如果你需要确认当前环境是否还有额外章型，可以再跑：

```bash
contract-cli contract enum list --profile contract-group --type seal_type_codes
```
