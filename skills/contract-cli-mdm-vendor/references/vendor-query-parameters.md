# Vendor Query Parameters

这份附录专门解决 `mdm vendor list` / `mdm vendor get` 的参数映射问题。

## 1. 请求面导航

```text
mdm vendor list
├── --name       -> $query.vendor
├── --page-size  -> $query.page_size
├── --page-token -> $query.page_token
├── --profile    -> profile selector
├── --as         -> identity selector
├── --user-id    -> common query passthrough
├── --user-id-type -> common query passthrough
├── --output     -> output format
└── --raw        -> raw response switch

mdm vendor get
├── <vendor-id>  -> $path.vendor_id
├── --profile    -> profile selector
├── --as         -> identity selector
├── --user-id    -> common query passthrough
├── --user-id-type -> common query passthrough
├── --output     -> output format
└── --raw        -> raw response switch
```

## 2. `mdm vendor list`

请求形态：

- 方法：`GET`
- `--as user` 路径：`/open-apis/contract/v1/mcp/vendors`
- `--as bot` 路径：`/open-apis/mdm/v1/vendors`

| CLI 参数 | 请求位置 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- | --- |
| `--name` | `$query.vendor` | `string` | 可选 | 交易方查询关键字。 | user 侧通常按名称筛选；bot 生产文档把它描述成供应商编码，CLI 继续统一透传到 `vendor`。 |
| `--page-size` | `$query.page_size` | `integer` | 可选 | 每页条数。 | 当前接口默认 `10`，建议不超过 `50`。 |
| `--page-token` | `$query.page_token` | `string` | 可选 | 分页令牌。 | 首次请求通常不传，翻页时使用上一页返回的 `page_token`。 |
| `--profile` | 本地上下文 | `string` | 可选 | 选择 profile。 | 未传时走默认 profile。 |
| `--as` | 本地上下文 | `string` | 可选 | 选择身份。 | `user` 走 MCP 路径，`bot` 走开放平台 `mdm/v1/vendors` 路径。 |
| `--user-id` | `$query.user_id` | `string` | 可选 | 通用用户标识参数。 | 传了就透传，不传就不带。 |
| `--user-id-type` | `$query.user_id_type` | `string` | 可选 | 通用用户标识类型。 | 传了就透传，不传就不带；bot 文档里常见 `employee_id`。 |
| `--output` | CLI 输出 | `string` | 可选 | 输出格式。 | 常用 `json` / `yaml` / `table`。 |
| `--raw` | CLI 输出 | `boolean` | 可选 | 返回原始 envelope。 | 排障时常用。 |

## 3. `mdm vendor get`

请求形态：

- 方法：`GET`
- `--as user` 路径：`/open-apis/contract/v1/mcp/vendors/{vendor_id}`
- `--as bot` 路径：`/open-apis/mdm/v1/vendors/{vendor_id}`

| CLI 参数 | 请求位置 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- | --- |
| `<vendor-id>` | `$path.vendor_id` | `string` | 必填 | 交易方 id。 | `user` 走 MCP 路径，`bot` 走开放平台 `mdm/v1/vendors/{vendor_id}` 路径。 |
| `--profile` | 本地上下文 | `string` | 可选 | 选择 profile。 | 未传时走默认 profile。 |
| `--as` | 本地上下文 | `string` | 可选 | 选择身份。 | `user` 走 MCP 路径，`bot` 走开放平台 `mdm/v1/vendors` 路径。 |
| `--user-id` | `$query.user_id` | `string` | 可选 | 通用用户标识参数。 | 文档里未显式列出，但 CLI 仍按共享约定透传。 |
| `--user-id-type` | `$query.user_id_type` | `string` | 可选 | 通用用户标识类型。 | bot 文档里显式列出；CLI 不做必填校验。 |
| `--output` | CLI 输出 | `string` | 可选 | 输出格式。 | 常用 `json` / `yaml` / `table`。 |
| `--raw` | CLI 输出 | `boolean` | 可选 | 返回原始 envelope。 | 排障时常用。 |

## 4. 使用建议

- 先 `list` 拿候选，再 `get` 看详情，是最稳的两步走方式
- 如果只是创建合同前选对方主体，优先保存交易方 id，后面再传给合同请求体
- 这组命令只做查询，不做本地字段裁剪或结构转换
- `mdm vendor list/get` 现在都支持按身份自动路由；`mdm legal` 和 `mdm fields` 仍保持 user-only
