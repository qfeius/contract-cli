# rule table pre-release Parameters

本页专用于 `contract-cli rule table pre-release`。

- 接口：`PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/pre_release`
- 身份：`user` / `app`（user 用于 contract 产品，需规则管理权限）
- 请求体：官方接口不要求 JSON 请求体
- 官方 OpenAPI：[矩阵-预发布规则表配置](https://docs.qfei.cn/375736093e0.md)
- 校验基线：`contract-cli` 当前分支；CLM `master@334c18fc2e` 存在对应实现时，以 Controller、DTO 和业务校验补充官方规格。

## 目录

- [CLI 参数映射](#cli-参数映射)
- [枚举与约束](#枚举与约束)
- [示例](#示例)
- [来源差异说明](#来源差异说明)

## CLI 参数映射

| 参数 | 请求位置 | 类型 | 必填性 | 说明 |
| --- | --- | --- | --- | --- |
| --product-id | $path.product_id | string | 必填（服务端） | 租户下的产品id 固定值:"contract" |
| --group-id | $path.group_id | string | 必填（服务端） | 产品下的规则组id 固定值:"approve_matrix" |
| --table-id | $path.table_id | string | 必填（服务端） | 规则组下的规则表id 示例值:"rule_table_test" |
| --input-file | $body | JSON file | 可选，不建议 | CLI 兼容；官方接口未定义请求体。 |
| --data | $body | JSON string | 可选，不建议 | 与 `--input-file` 互斥；官方接口未定义请求体。 |
| --profile | 本地上下文 | string | 可选 | 不传时使用当前 profile。 |
| --as | 本地上下文 | enum | 可选 | 支持 `user` / `app`；不传时使用 profile 默认身份。 |
| --user-id-type | $query.user_id_type | string | 可选 | 不传时 CLI 默认发送 `user_id`。 |
| --user-id | $query.user_id | string | 可选 | 矩阵路径会忽略该参数；编辑人由当前用户令牌身份确定。 |
| --output | CLI 输出 | enum | 可选 | `json`、`yaml` 或 `table`；默认 `json`。 |
| --raw | CLI 输出 | boolean | 可选 | 原样输出服务端响应 body。 |

## 枚举与约束

- CLI 在预发布前读取全部规则行分页，保存按行 ID、列 ID 对齐的基准；预发布后回读版本并再次核对规则行，再持久化到当前配置目录的 `approval-matrix-publish`。文件权限 0600，不保存 Token。
- 重新预发布先使旧基准失效；查询失败、分页缺失/循环、重复 ID、预发布失败或期间数据变化时停止，不保存可发布基准。

- 当对某规则表完成全部的新增、修改和删除操作后，需调用此接口对规则表的配置进行预发布，<br>完成后再调用发布规则表配置进行正式发布，成功后规则表更改的内容才算正式生效。
- 官方接口未定义请求体；当前 CLI 虽兼容可选 body，正常调用不要传。

## 示例

```bash
contract-cli rule table pre-release --product-id contract --group-id approve_matrix --table-id <table-id> --profile contract --as app
```

## 来源差异说明

- 官方规格路径：`PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/pre_release`
- CLI 路径表达：`PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/pre_release`；示例 ID 或占位名已统一为 CLI 名称。
- 本页描述请求参数；响应 envelope 和输出格式遵循共享 Skill。
