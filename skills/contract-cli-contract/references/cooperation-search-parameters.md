# contract cooperation search Parameters

本页专用于 `contract-cli contract cooperation search --as app`。

`--as user` 使用另一套个人权限接口和请求体，见 [个人协商搜索参数](../../contract-cooperation/references/search-parameters.md)。未传 `--as` 时沿用 profile 默认身份，不自动切换。

- 接口：`POST /open-apis/contract/v1/cooperation/search`
- 身份：仅 `app`
- 请求体：JSON 必填，`--input-file` 与 `--data` 二选一
- 官方 OpenAPI：[查询协商列表](https://docs.qfei.cn/465847310e0.md)
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
| --user-id-type | $query.user_id_type | string | 可选，默认 `user_id` | 用户 ID 类型 详情参考：用户身份体系<br>示例值："user_id"<br>可选值有：<br>- user_id：标识一个用户在某个租户内的身份。同一个用户在租户 A 和租户 B 内的 User ID 是不同的。在同一个租户内，一个用户的 User ID 在所有应用（包括商店应用）中都保持一致。User ID 主要用于在不同的应用间打通用户数据。<br>- union_id：标识一个用户在某个应用开发商下的身份。同一用户在同一开发商下的应用中的 Union ID 是相同的，在不同开发商下的应用中的 Union ID 是不同的。通过 Union ID，应用开发商可以把同个用户在多个应用中的身份关联起来。 |
| --input-file | $body | JSON file | 二选一必填 | 从文件读取 JSON；与 `--data` 互斥。 |
| --data | $body | JSON string | 二选一必填 | 内联 JSON；与 `--input-file` 互斥。 |
| --profile | 本地上下文 | string | 可选 | 不传时使用当前 profile。 |
| --as | 本地上下文 | enum | 可选 | 仅支持 `app`；不传时使用 profile 默认身份。 |
| --user-id | $query.user_id | string | 可选 | 传入时透传；MDM create/update 除外。 |
| --output | CLI 输出 | enum | 可选 | `json`、`yaml` 或 `table`；默认 `json`。 |
| --raw | CLI 输出 | boolean | 可选 | 原样输出服务端响应 body。 |

## 请求体字段

字段名、类型和服务端必填性来自官方 OpenAPI；“CLI 必填/禁止”是结构化命令的额外本地校验。父对象可选时，其内部必填字段标记为“父对象存在时必填”。

| JSON 路径 | 类型 | 必填性 | 说明 |
| --- | --- | --- | --- |
| user_id | string | 必填 | 协商参与者用户id |
| keyword | string | 可选 | 搜索合同名称、协商发起人、合同编号 |
| cooperation_query_status_codes | array<integer> | 可选 | 协商状态，示例值：[2, 1]<br>2:协商中<br>1:待发起协商<br>5:协商已取消<br>4:协商完成<br>6:已拒绝 |
| contract_category_ids | array<string> | 可选 | 合同类型id，<br>示例值：["993477335229399382","993477335229399383"] |
| search_cooperation_by_create_time | array<string> | 可选 | 协商发起日期，日期区间，日期格式为 YYYY-mm-dd。<br>示例值：["2026-02-06", "2026-04-08"] |
| create_user_ids | array<string> | 可选 | 协商发起人，示例值：["qazwsx","ollmijnk"] |
| search_cooperation_by_last_confirm_time | array<string> | 可选 | 我最后确认合同日期，日期区间，日期格式为 YYYY-mm-dd。<br>示例值：["2026-02-06", "2026-04-08"] |
| confirmed_role_codes | array<string> | 可选 | 已确认角色，示例值：["legal", "finance"]<br>legal：法务<br>finance：财务 |
| confirm_process_status_codes | array<integer> | 可选 | 确认流程进度，示例值：[4]<br>4：全部节点已确认 |
| confirmable_status_codes | array<integer> | 可选 | 我是否可确认，示例值：[1, 2, 3, 4]<br>1：待我确认<br>2：待前序节点确认<br>3：待我定稿<br>4：我已确认 |
| page_size | integer | 可选 | 分页大小，默认10 |
| page_token | string | 可选 | 分页标记，第一次请求不填，表示从头开始遍历；分页查询结果还有更多项时会同时返回新的 page_token，下次遍历可采用该 page_token 获取查询结果 |

## 枚举与约束

- `search_cooperation_by_create_time`（array<string>，可选）：协商发起日期，日期区间，日期格式为 YYYY-mm-dd。<br>示例值：["2026-02-06", "2026-04-08"]
- `search_cooperation_by_last_confirm_time`（array<string>，可选）：我最后确认合同日期，日期区间，日期格式为 YYYY-mm-dd。<br>示例值：["2026-02-06", "2026-04-08"]
- `page_size`（integer，可选）：分页大小，默认10

## 示例

```bash
contract-cli contract cooperation search --input-file request.json --profile contract --as app
```

官方请求体示例（动态字段接口仍须以当前租户配置为准）：

```json
"{\n  \"user_id\": \"wersdfxc\",\n  \"keyword\": \"123\",\n  \"cooperation_query_status_codes\": [\n    2\n  ],\n  \"contract_category_ids\": [\n    \"993477335229399383\"\n  ],\n  \"search_cooperation_by_create_time\": [\n    \"2026-02-06\",\n    \"2026-04-08\"\n  ],\n  \"create_user_ids\": [\n    \"wersdfxc\"\n  ],\n  \"search_cooperation_by_last_confirm_time\": [\n    \"2026-03-13\",\n    \"2026-04-16\"\n  ],\n  \"confirmed_role_codes\": [\n    \"legal\",\n    \"finance\"\n  ],\n  \"confirm_process_status_codes\": [\n    4\n  ],\n  \"confirmable_status_codes\": [\n    1\n  ]\n  \"page_size\": 10,\n  \"page_token\": \"1\"\n}"
```

## 来源差异说明

- 官方规格路径：`POST /open-apis/contract/v1/cooperation/search`
- 本页描述请求参数；响应 envelope 和输出格式遵循共享 Skill。
