# rule table row list Parameters

本页专用于 `contract-cli rule table row list`。

- 接口：`GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows`
- 身份：`user` / `app`（user 用于 contract 产品，需规则管理权限）
- 请求体：官方接口不要求 JSON 请求体
- 官方 OpenAPI：[矩阵-查询规则表行信息列表](https://docs.qfei.cn/375950808e0.md)
- 校验基线：`contract-cli` 当前分支；CLM `master@334c18fc2e` 存在对应实现时，以 Controller、DTO 和业务校验补充官方规格。

## 目录

- [CLI 参数映射](#cli-参数映射)
- [枚举与约束](#枚举与约束)
- [示例](#示例)
- [来源差异说明](#来源差异说明)

## CLI 参数映射

| 参数 | 请求位置 | 类型 | 必填性 | 说明 |
| --- | --- | --- | --- | --- |
| --product-id | $path.product_id | string | 必填（服务端） | 租户下的产品 id 固定值:"contract" |
| --group-id | $path.group_id | string | 必填（服务端） | 产品下的规则组 id 固定值:"approve_matrix" |
| --table-id | $path.table_id | string | 必填（服务端） | 规则组下的规则表 id |
| --page-size | $query.page_size | integer | 必填（服务端实测） | 必须显式传入，示例值：10；省略时服务端返回 `code 100000 parameter error`。 |
| --page-token | $query.page_token | string | 可选 | CLI 支持；官方规格未列出该 query。 |
| --profile | 本地上下文 | string | 可选 | 不传时使用当前 profile。 |
| --as | 本地上下文 | enum | 可选 | 支持 `user` / `app`；不传时使用 profile 默认身份。 |
| --user-id-type | $query.user_id_type | string | 可选 | 不传时 CLI 默认发送 `user_id`。 |
| --user-id | $query.user_id | string | 可选 | 矩阵路径会忽略该参数；编辑人由当前用户令牌身份确定。 |
| --output | CLI 输出 | enum | 可选 | `json`、`yaml` 或 `table`；默认 `json`。 |
| --raw | CLI 输出 | boolean | 可选 | 原样输出服务端响应 body。 |

## 枚举与约束

- 可调用此接口分页查询规则表中的所有行信息，每行的数据结构与根据行ID查询规则表单行信息相同
- 每次请求必须显式传入 `--page-size`，建议使用已实测的 `10`；后续分页继续携带该参数，并通过 `--page-token` 传入服务端返回的下一页游标。

## 示例

```bash
contract-cli rule table row list --product-id contract --group-id approve_matrix --table-id <table-id> --page-size 10 --profile contract --as app
```

## 来源差异说明

- 官方规格未列出分页 query；当前 CLI 支持 `page_size` / `page_token`。根据用户实测反馈，省略 `--page-size` 时服务端返回 `code 100000 parameter error`，显式传入 `--page-size 10` 后成功，因此本页将其标注为服务端必填；这是服务端实测约束，不代表 CLI 已在本地强制校验。
- 官方规格路径：`GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows`
- CLI 路径表达：`GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows`；示例 ID 或占位名已统一为 CLI 名称。
- 本页描述请求参数；响应 envelope 和输出格式遵循共享 Skill。
