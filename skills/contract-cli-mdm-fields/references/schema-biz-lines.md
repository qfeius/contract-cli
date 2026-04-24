# Schema Biz Lines Reference

这份附录专门解决 `mdm fields list` 的参数和值域问题。

## 1. 请求面导航

```text
mdm fields list
├── --biz-line -> $query.biz_line
├── --profile  -> profile selector
├── --as       -> identity selector
├── --user-id  -> common query passthrough
├── --user-id-type -> common query passthrough
├── --output   -> output format
└── --raw      -> raw response switch
```

请求形态：

- 方法：`GET`
- `--as user` 路径：`/open-apis/contract/v1/mcp/config/config_list`
- `--as bot` 路径：`/open-apis/mdm/v1/config/config_list`
- bot 后端当前只接受 `vendor` / `legalEntity`；CLI 会把 bot 下的 `legal_entity` 映射为 `legalEntity`

## 2. 参数说明

| CLI 参数 | 请求位置 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- | --- |
| `--biz-line` | `$query.biz_line` | `string` | 必填 | 主数据业务线。 | 当前支持值见下表；bot 身份下 `legal_entity` 会映射为 `legalEntity`，`vendor_risk` 不支持。 |
| `--profile` | 本地上下文 | `string` | 可选 | 选择 profile。 | 未传时走默认 profile。 |
| `--as` | 本地上下文 | `string` | 可选 | 选择身份。 | `user` 走 MCP 路径，`bot` 走开放平台 `mdm/v1/config/config_list` 路径。 |
| `--user-id` | `$query.user_id` | `string` | 可选 | 通用用户标识参数。 | 文档里未显式列出，但 CLI 仍按共享约定透传。 |
| `--user-id-type` | `$query.user_id_type` | `string` | 可选 | 通用用户标识类型。 | CLI 透传，不做本地校验。 |
| `--output` | CLI 输出 | `string` | 可选 | 输出格式。 | 常用 `json` / `yaml` / `table`。 |
| `--raw` | CLI 输出 | `boolean` | 可选 | 返回原始 envelope。 | 排障时常用。 |

## 3. `--biz-line` 支持值

| 值 | 含义 | 适用场景 |
| --- | --- | --- |
| `vendor` | 交易方 | 写交易方或核对交易方字段结构前使用。 |
| `legal_entity` | 法人实体 | 写法人实体或核对我方主体字段结构前使用；bot 下会映射为后端实际值 `legalEntity`。 |
| `legalEntity` | 法人实体 | bot 后端实际取值；CLI 也可直接传这个值。 |
| `vendor_risk` | 交易方风险 | 仅 user/MCP 路径可用；bot 后端当前不支持。 |

## 4. 使用建议

- 在调用未封装写接口前，先用这条命令确认字段名，避免硬猜
- 如果字段结构复杂，先用结构化命令确认字段；`contract-cli api call` 当前暂未开放，不要作为兜底入口
