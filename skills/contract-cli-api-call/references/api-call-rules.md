# API Call Rules

这份附录专门解决 `contract-cli api call` 的方法、路径、身份、请求体和 header 规则。

## 1. 请求面导航

```text
api call
├── <METHOD>
├── <PATH>        -> relative /open-apis/... path
├── --input-file  -> raw body from file
├── --data        -> raw body from inline string
├── --header      -> repeated request headers
├── --profile     -> profile selector
├── --as          -> identity selector
├── --output      -> output format
└── --raw         -> raw response switch
```

## 2. 参数与规则

| 参数/位置 | 类型 | 必填性 | 业务含义 | 联动/注意 |
| --- | --- | --- | --- | --- |
| `<METHOD>` | `string` | 必填 | HTTP 方法。 | 例如 `GET`、`POST`、`PUT`。 |
| `<PATH>` | `string` | 必填 | 开放平台相对路径。 | 必须以 `/open-apis/` 开头；不能是绝对 URL。 |
| `--input-file` | `string` | 可选 | 从文件读取请求体。 | 与 `--data` 互斥。 |
| `--data` | `string` | 可选 | 直接内联请求体。 | 与 `--input-file` 互斥。 |
| `--header` | `string` | 可选，可重复 | 追加请求头。 | 格式必须是 `Key: Value`。 |
| `--profile` | `string` | 可选 | 选择 profile。 | 未传时走默认 profile。 |
| `--as` | `string` | 可选 | 选择身份。 | 某些路径会被身份策略硬拦截。 |
| `--output` | `string` | 可选 | 输出格式。 | 常用 `json` / `yaml` / `table`。 |
| `--raw` | `boolean` | 可选 | 返回原始 envelope。 | 排障时常用。 |

## 3. user-only 路径规则

命中下列前缀时：

- `/open-apis/contract/v1/mcp/`

规则固定为：

- `--as bot` 会在本地直接报错
- 不传 `--as` 时，CLI 默认按 `user` 身份解析
- 这类路径适合和 `contract` / `vendor` / `entity` / `schema` 结构化命令一起对照使用

## 4. 请求体建议

- JSON 请求优先用 `--input-file`
- 小型调试请求可以用 `--data`
- 如果只是排障响应，不一定要加 `--raw`
- 如果要和结构化命令对照，尽量保持同一 `--profile` 和同一身份
