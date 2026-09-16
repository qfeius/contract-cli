# rule table row search Parameters

本页专用于 `contract-cli rule table row search`。

- 接口：`POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/search`
- 身份：`user` / `app`（user 用于 contract 产品，需规则管理权限）
- 请求体：JSON 必填，`--input-file` 与 `--data` 二选一
- 官方 OpenAPI：[矩阵-根据筛选条件查询规则表行信息列表](https://docs.qfei.cn/375948395e0.md)
- 校验基线：`contract-cli` 当前分支；CLM `master@334c18fc2e` 存在对应实现时，以 Controller、DTO 和业务校验补充官方规格。

## 目录

- [CLI 参数映射](#cli-参数映射)
- [请求体字段](#请求体字段)
- [枚举与约束](#枚举与约束)
- [示例](#示例)
- [来源差异说明](#来源差异说明)

## CLI 参数映射

| 参数 | 请求位置 | 类型 | 必填性 | 说明 |
| --- | --- | --- | --- | --- |
| --product-id | $path.product_id | string | 必填（服务端） | 租户下的产品 id 固定值:"contract" |
| --group-id | $path.group_id | string | 必填（服务端） | 产品下的规则组 id 固定值:"approve_matrix" |
| --table-id | $path.table_id | string | 必填（服务端） | 规则组下的规则表 id |
| --page-size | $query.page_size | integer | 必填（服务端） | 分页大小，范围 1-100，示例值：10 |
| --page-token | $query.page_token | string | 可选 | 分页标记，第一次请求不填，表示从头开始遍历；分页查询结果还有更多项时会同时返回新的 page_token，下次遍历可采用该 page_token 获取查询结果示例值："7112034393270534188" |
| --user-id-type | $query.user_id_type | string | 可选，默认 `user_id` | 用户 ID 类型，参考 用户身份体系 |
| --input-file | $body | JSON file | 二选一必填 | 从文件读取 JSON；与 `--data` 互斥。 |
| --data | $body | JSON string | 二选一必填 | 内联 JSON；与 `--input-file` 互斥。 |
| --profile | 本地上下文 | string | 可选 | 不传时使用当前 profile。 |
| --as | 本地上下文 | enum | 可选 | 支持 `user` / `app`；不传时使用 profile 默认身份。 |
| --user-id | $query.user_id | string | 可选 | 矩阵路径会忽略该参数；编辑人由当前用户令牌身份确定。 |
| --output | CLI 输出 | enum | 可选 | `json`、`yaml` 或 `table`；默认 `json`。 |
| --raw | CLI 输出 | boolean | 可选 | 原样输出服务端响应 body。 |

## 请求体字段

字段名、类型和服务端必填性来自官方 OpenAPI；“CLI 必填/禁止”是结构化命令的额外本地校验。父对象可选时，其内部必填字段标记为“父对象存在时必填”。

| JSON 路径 | 类型 | 必填性 | 说明 |
| --- | --- | --- | --- |
| table_cell | object | 必填 | 单元格列表 |
| table_cell.table_cell_content | object | 必填 | 单元格内容，六个字段与六种类型进行映射，每个单元格只能填充与单元格内容类型对应的字段；注意其他字段应为 null |
| table_cell.table_cell_content.bool | boolean | 可选 | 布尔值示例值：true |
| table_cell.table_cell_content.collection | array<string> | 可选 | 集合示例值：["element1"] |
| table_cell.table_cell_content.department_collection | array<string> | 可选 | 部门集合示例值：["od-sdwerdfvdc"] |
| table_cell.table_cell_content.employee_collection | array<string> | 可选 | 人员集合示例值：["ou-sdwerdfvdc"] |
| table_cell.table_cell_content.role_collection | array<string> | 可选 | 角色集合示例值：["123520234"] |
| table_cell.table_cell_content.number | number | 可选 | 数值示例值：1 |
| table_cell.table_cell_content.string | string | 可选 | 字符串值示例值："strDemo" |
| table_cell.table_cell_content_type | string | 必填 | 单元格内容类型，需与规则表列头中的单元格内容类型保持一致示例值："EMPLOYEE_COLLECTION"可选值有：STRING：字符串NUMBER：数值BOOLEAN：布尔COLLECTION：集合EMPLOYEE_COLLECTION：人员集合 |
| table_cell.table_column_id | string | 必填 | 规则表列 id示例值："7113921335113285631" |

## 枚举与约束

- 可调用此接口搜索规则表中的某些行信息，若单元格内容类型为字符串、数值和布尔时，支持精确查找；当类型为集合、人员集合和部门集合时，支持模糊搜索，当集合包含入参中的值时即返回。<br>- 不支持以行 ID 为条件进行查找，如果需要，请调用根据行ID查询规则表单行信息。<br>- 不支持空值查找，即单元格内容不能为空。<br>- 搜索时需指定列，系统会按指定列去搜索对应的行信息。
- `table_cell.table_cell_content`（object，必填）：单元格内容，六个字段与六种类型进行映射，每个单元格只能填充与单元格内容类型对应的字段；注意其他字段应为 null
- `table_cell.table_cell_content_type`（string，必填）：单元格内容类型，需与规则表列头中的单元格内容类型保持一致示例值："EMPLOYEE_COLLECTION"可选值有：STRING：字符串NUMBER：数值BOOLEAN：布尔COLLECTION：集合EMPLOYEE_COLLECTION：人员集合
- 官方参数 `department_id_type`（query，可选）当前没有对应 CLI flag。

## 示例

```bash
contract-cli rule table row search --product-id contract --group-id approve_matrix --table-id <table-id> --page-size 10 --input-file request.json --profile contract --as app
```

官方请求体示例（动态字段接口仍须以当前租户配置为准）：

```json
{
  "table_cell": {
    "table_cell_content": {
      "bool": null,
      "collection": null,
      "department_collection": null,
      "employee_collection": [
        "ou-sdwerdfvdc"
      ],
      "role_collection": null,
      "number": null,
      "string": null
    },
    "table_cell_content_type": "EMPLOYEE_COLLECTION",
    "table_column_id": "7111905213878894636"
  }
}
```

## 来源差异说明

- 官方规格路径：`POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/search`
- CLI 路径表达：`POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/search`；示例 ID 或占位名已统一为 CLI 名称。
- 本页描述请求参数；响应 envelope 和输出格式遵循共享 Skill。
