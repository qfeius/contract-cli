# rule table list Parameters

本页专用于 `contract-cli rule table list`。

- 接口：`GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables`
- 身份：`user` / `app`（user 用于 contract 产品，需规则管理权限）
- 请求体：官方接口不要求 JSON 请求体
- 官方 OpenAPI：[矩阵-查询规则表列表](https://docs.qfei.cn/375900397e0.md)
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
| --page-size | $query.page_size | integer | 必填（服务端） | 分页大小，范围 1-100，示例值:10 |
| --page-token | $query.page_token | string | 可选 | 分页标记,第一次请求不填,表示从头开始遍历;分页查询结果是还<br>有更多项时会同时返回新的page_token,下次遍历可采用该<br>page_token获取查询结果 示例值:"10" |
| --profile | 本地上下文 | string | 可选 | 不传时使用当前 profile。 |
| --as | 本地上下文 | enum | 可选 | 支持 `user` / `app`；不传时使用 profile 默认身份。 |
| --user-id-type | $query.user_id_type | string | 可选 | 不传时 CLI 默认发送 `user_id`。 |
| --user-id | $query.user_id | string | 可选 | 矩阵路径会忽略该参数；编辑人由当前用户令牌身份确定。 |
| --output | CLI 输出 | enum | 可选 | `json`、`yaml` 或 `table`；默认 `json`。 |
| --raw | CLI 输出 | boolean | 可选 | 原样输出服务端响应 body。 |

## 枚举与约束

- 此接口可用来分页查询规则表列表。

## 示例

```bash
contract-cli rule table list --product-id contract --group-id approve_matrix --page-size 10 --profile contract --as app
```

## 来源差异说明

- 官方规格路径：`GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables`
- 本页描述请求参数；响应 envelope 和输出格式遵循共享 Skill。
