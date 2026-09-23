# rule table release Parameters

本页专用于 `contract-cli rule table release`。

- 接口：`PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/release`
- 身份：`user` / `app`（user 用于 contract 产品，需规则管理权限）
- 请求体：官方接口不要求 JSON 请求体
- 官方 OpenAPI：[矩阵-发布规则表配置](https://docs.qfei.cn/375738193e0.md)
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

- CLI 强制要求本机同一配置目录内的有效预发布基准。基准丢失但服务端仍为 `status=0` 待发布状态时，先执行 `pre-release` 只读恢复；看到 `baseline_recovered=true` 后重新展示版本和行数并确认，再执行本命令。服务端是草稿状态时仍按正常预发布流程处理。
- 基准绑定 profile、环境、身份、矩阵路径和预发布版本；用户凭证轮换后需重新预发布。发布前检查状态/版本并读取全部分页，按行 ID、列 ID 比较，忽略返回顺序；大整数和金额不经 float64 转换。
- 新增、删除、值变化、读取异常或分页不完整时停止；基准消费后，失败或结果不确定时也不直接重试，先查询，再重新预发布并确认。
- 恢复基准时不改写业务数据；原值重写、清空、增删行或改列均不属于发布恢复步骤。
- 仅核对规则行，不涵盖 symbol 等列配置；查询和发布仍非原子操作。发布成功后回读 release_version 核验。

- 规则表修改完成后需预发布规则表配置，成功后才能调用此接口对规则表配置进行正式发布，发布成功后修改的配置才会生效。
- 官方接口未定义请求体；当前 CLI 虽兼容可选 body，正常调用不要传。

## 示例

```bash
contract-cli rule table release --product-id contract --group-id approve_matrix --table-id <table-id> --profile contract --as app
```

## 来源差异说明

- 官方规格路径：`PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/release`
- CLI 路径表达：`PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/release`；示例 ID 或占位名已统一为 CLI 名称。
- 本页描述请求参数；响应 envelope 和输出格式遵循共享 Skill。
