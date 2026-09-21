# rule table row update Parameters

本页专用于 `contract-cli rule table row update`。

- 接口：`PUT /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/{row_id}`
- 身份：`user` / `app`（user 用于 contract 产品，需规则管理权限）
- 请求体：JSON 必填，`--input-file` 与 `--data` 二选一
- 官方 OpenAPI：[矩阵-修改规则表行](https://docs.qfei.cn/375933532e0.md)
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
| <row-id> | $path.row_id | string | 必填（服务端） | 规则表行 id |
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
| table_cells | array<object> | 必填 | 单元格列表 |
| table_cells[].table_cell_content | object | 必填 | 单元格内容，六个字段与六种类型进行映射，每个单元格只能填充与单元格内容类型对应的字段；注意其他字段应为 null |
| table_cells[].table_cell_content.bool | boolean | 可选 | 布尔值示例值：true |
| table_cells[].table_cell_content.collection | array<string> | 可选 | 集合示例值：["element1"] |
| table_cells[].table_cell_content.department_collection | array<string> | 可选 | 部门 `open_department_id`，例如 ["od-1b1b803a7df98989bf457d9ba203c350"]；不要传 `sys_department.id` 数字 |
| table_cells[].table_cell_content.employee_collection | array<int64> | 可选 | 人员正整数外部 ID，例如 [7113921696628736004] |
| table_cells[].table_cell_content.role_collection | array<string> | 可选 | 角色集合示例值：["123520234"] |
| table_cells[].table_cell_content.number | decimal | 可选 | 后端 BigDecimal，整数最多 30 位、小数最多 8 位，示例：1000.50 |
| table_cells[].table_cell_content.string | string | 可选 | 字符串值示例值："strDemo" |
| table_cells[].table_cell_content_type | string | 必填 | 单元格内容类型，需与规则表列头中的单元格内容类型保持一致示例值："EMPLOYEE_COLLECTION"可选值有：STRING：字符串NUMBER：数值BOOLEAN：布尔COLLECTION：集合EMPLOYEE_COLLECTION：人员集合DEPARTMENT_COLLECTION：部门集合ROLE_COLLECTION：角色集合类型 |
| table_cells[].table_column_id | string | 必填 | 规则表列 id示例值："7113921335113285631" |

## 枚举与约束

- 可调用此接口修改目标行信息，请求体与创建规则表行 相同，接口只会修改入参中指定的单元格，对其它单元格无效。单元格对应的列 id，单元格内容类型需与对应的规则表列头数据保持一致，列头数据可调用 查询规则表列头信息得到。
- `table_cells[].table_cell_content`（object，必填）：单元格内容，六个字段与六种类型进行映射，每个单元格只能填充与单元格内容类型对应的字段；注意其他字段应为 null。部门集合使用 `open_department_id` 字符串，人员集合使用正整数外部 ID。
- `table_cells[].table_cell_content_type`（string，必填）：单元格内容类型，需与规则表列头中的单元格内容类型保持一致示例值："EMPLOYEE_COLLECTION"可选值有：STRING：字符串NUMBER：数值BOOLEAN：布尔COLLECTION：集合EMPLOYEE_COLLECTION：人员集合DEPARTMENT_COLLECTION：部门集合ROLE_COLLECTION：角色集合类型
- 官方参数 `department_id_type`（query，可选）当前没有对应 CLI flag。

## 示例

```bash
contract-cli rule table row update --product-id contract --group-id approve_matrix --table-id <table-id> <row-id> --input-file request.json --profile contract --as app
```

官方请求体示例（动态字段接口仍须以当前租户配置为准）：

```json
{
  "table_cells": [
    {
      "table_cell_content": {
        "bool": false,
        "collection": null,
        "department_collection": null,
        "employee_collection": [
          7113921696628736004,
          7113921696628736005
        ],
        "role_collection": null,
        "number": null,
        "string": null
      },
      "table_cell_content_type": "EMPLOYEE_COLLECTION",
      "table_column_id": "6113921696628736003"
    }
  ]
}
```

## 来源差异说明

- 官方规格路径：`PUT /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/{table_row_id}`
- CLI 路径表达：`PUT /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/{row_id}`；示例 ID 或占位名已统一为 CLI 名称。
- 本页描述请求参数；响应 envelope 和输出格式遵循共享 Skill。

本次代码校准：人员字段按正整数外部 ID 处理；部门字段按 `open_department_id` 字符串（`od-...`）处理；NUMBER 为 `BigDecimal`（30 位整数、8 位小数）；需部署对应后端。原子命令继续透传 JSON，调用前按后端类型构造数据。
