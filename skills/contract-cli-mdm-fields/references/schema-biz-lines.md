# Schema Biz Lines Reference

这份附录专门解决 `mdm fields list` 的参数和值域问题。

## 1. 请求面导航

```text
mdm fields list
├── --biz-line -> $query.biz_line
├── --profile  -> profile selector
├── --as       -> identity selector (must be user)
├── --output   -> output format
└── --raw      -> raw response switch
```

请求形态：

- 方法：`GET`
- 路径：`/open-apis/contract/v1/mcp/config/config_list`

## 2. 参数说明

| CLI 参数 | 请求位置 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- | --- |
| `--biz-line` | `$query.biz_line` | `string` | 必填 | 主数据业务线。 | 当前支持值见下表。 |
| `--profile` | 本地上下文 | `string` | 可选 | 选择 profile。 | 未传时走默认 profile。 |
| `--as` | 本地上下文 | `string` | 可选 | 选择身份。 | 当前接口只支持 `user`。 |
| `--output` | CLI 输出 | `string` | 可选 | 输出格式。 | 常用 `json` / `yaml` / `table`。 |
| `--raw` | CLI 输出 | `boolean` | 可选 | 返回原始 envelope。 | 排障时常用。 |

## 3. `--biz-line` 支持值

| 值 | 含义 | 适用场景 |
| --- | --- | --- |
| `vendor` | 交易方 | 写交易方或核对交易方字段结构前使用。 |
| `legal_entity` | 法人实体 | 写法人实体或核对我方主体字段结构前使用。 |
| `vendor_risk` | 交易方风险 | 核对交易方风险相关字段结构前使用。 |

## 4. 使用建议

- 在调用未封装写接口前，先用这条命令确认字段名，避免硬猜
- 如果字段结构复杂，建议配合 `contract-cli api call` 一起使用
