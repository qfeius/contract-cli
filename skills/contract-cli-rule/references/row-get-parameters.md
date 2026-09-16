# rule table row get Parameters

本页专用于 `contract-cli rule table row get`。

- 接口：`GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/{row_id}`
- 身份：`user` / `app`（user 用于 contract 产品，需规则管理权限）
- 请求体：官方接口不要求 JSON 请求体
- 官方 OpenAPI：[矩阵-根据行ID查询规则表单行信息](https://docs.qfei.cn/375940239e0.md)
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
| <row-id> | $path.row_id | string | 必填（服务端） | 规则表行 id |
| --user-id-type | $query.user_id_type | string | 可选，默认 `user_id` | 用户 ID 类型，参考 用户身份体系 |
| --profile | 本地上下文 | string | 可选 | 不传时使用当前 profile。 |
| --as | 本地上下文 | enum | 可选 | 支持 `user` / `app`；不传时使用 profile 默认身份。 |
| --user-id | $query.user_id | string | 可选 | 矩阵路径会忽略该参数；编辑人由当前用户令牌身份确定。 |
| --output | CLI 输出 | enum | 可选 | `json`、`yaml` 或 `table`；默认 `json`。 |
| --raw | CLI 输出 | boolean | 可选 | 原样输出服务端响应 body。 |

## 枚举与约束

- 可调用此接口获取目标规则表目标行的数据，返回值为单元格组成的列表，可以通过单元格的列 id 与查询规则表列头信息中的列 id 进行对应
- 官方参数 `department_id_type`（query，可选）当前没有对应 CLI flag。

## 示例

```bash
contract-cli rule table row get --product-id contract --group-id approve_matrix --table-id <table-id> <row-id> --profile contract --as app
```

## 来源差异说明

- 官方规格路径：`GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/{table_row_id}`
- CLI 路径表达：`GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/{row_id}`；示例 ID 或占位名已统一为 CLI 名称。
- 本页描述请求参数；响应 envelope 和输出格式遵循共享 Skill。
