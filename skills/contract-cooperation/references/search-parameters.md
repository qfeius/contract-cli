# 个人协商搜索参数

命令：`contract-cli contract cooperation search --input-file search.json --profile <profile> --as user --output json`。
接口：`POST /open-apis/contract/v1/mcp/cooperations/search`，仅 user；固定 `user_id_type=user_id`、`department_id_type=open_department_id`。本参数表来自本次提供的 MCP `search-cooperations` 定义，不适用于 app 的旧搜索接口。多个发起人使用 [逐人查询后合并去重](multi-creator-search.md)，每个请求仍只传一个 create_user_id。两组确认类筛选仍待服务端支持，见 [筛选覆盖与缺口](filter-coverage.md)。

请求体是一个 JSON 对象；所有字段可选，至少提供一个空对象 `{}`。CLI 不接受未知字段、调用人覆盖字段或错误字段类型。

| 字段 | 类型 | 默认值或含义 |
|---|---|---|
| `page_size` | integer | 默认 10，范围 1–20 |
| `page_token` | string | 首次省略，翻页原样传响应值 |
| `keyword` | string | 固定搜索合同名称、协商发起人、合同编号 |
| `search_tab_enum_code` | integer | 默认 0 全部；1 我的/与我有关 |
| `cooperation_display_tab_code` | integer | 默认 0 全部；1 我申请/发起；2 待我确认/定稿；3 我已确认/定稿；4 业务；5 法务 |
| `cooperation_query_status_codes` | array<integer> | 省略表示全部；常用值 0 全部、1 待发起、2 协商中、4 完成、5 已取消、6 已拒绝 |
| `contract_category_ids` | array<string> | 合同类型 ID |
| `search_cooperation_by_create_time` | array<string> | 协商发起日期范围 |
| `search_cooperation_by_last_confirm_time` | array<string> | 我最后确认合同日期范围 |
| `search_cooperation_by_latest_version_upload_time` | array<string> | 最新版本上传日期范围 |
| `search_cooperation_by_cooperation_complete_time` | array<string> | 协商完成日期范围 |
| `create_user_id` | string | 已知发起人的 user_id，单值；多个人分别查询，不传 `create_user_ids` |
| `department_collection` | array<string> | 发起人的 open_department_id 列表 |
| `confirmed_role_codes` | array<string> | 已确认/定稿角色编码，使用真实编码 |
| `order` | string | 默认 `DESC`；可选 `ASC` |

日期筛选使用 `["YYYY-MM-DD", "YYYY-MM-DD"]`，必须是有效日期且开始不晚于结束；省略或空数组表示不限定该日期。人员和分类等 ID 始终使用字符串。

“我发起且已完成”的请求体：

```json
{
  "page_size": 10,
  "search_tab_enum_code": 1,
  "cooperation_display_tab_code": 1,
  "cooperation_query_status_codes": [4],
  "order": "DESC"
}
```

CLI 每次只查询一页，原样保留响应的分页信息和服务端原因。app 搜索仍使用原接口和原请求体，其参数参考 [app 搜索参数](../../contract-cli-contract/references/cooperation-search-parameters.md)。不要混用两套字段，也不要把个人搜索失败后切为 app。

## 搜索状态与返回状态不可混用

`cooperation_query_status_codes` 是**搜索分组枚举**，不是响应 `cooperation_status_code` 的枚举。后端 `CooperationQueryStatus.getCooperationStatusUnionByQueryStatuses` 将搜索条件转换为实际协商状态；2026-10-21 生产只读抽样与该映射一致。

| 搜索状态码 | 搜索含义 | 对应的返回状态码 |
|---|---|---|
| 1 | 协商待发起 | 0 未发起、5 协商回滚 |
| 2 | 协商中 | 2 协商中 |
| 4 | 协商完成分组 | 3 协商完成、6 已结束 |
| 5 | 协商已取消 | 4 协商取消 |
| 6 | 协商已拒绝 | 7 已拒绝 |

因此，“已完成”按本 MCP 契约传 `[4]`；返回“已结束（6）”不表示筛选未生效，更不能将其标成“已拒绝”。面向用户时返回码 3/6 均展示为页面的“协商完成”，参见 [页面状态映射](response-fields.md)。用户按页面查询“协商完成”时不得排除返回码 6。仅用户明确要求按底层原始状态区分时才细分，并说明分页范围；不猜另一搜索状态码，不把局部结果宣称为全量。

“待我确认/定稿”是当前用户的列表视角，不能只凭全局状态“协商中”断言某个人待确认，也不能从参与人数减确认人数推断当前用户身份。应由 `search_tab_enum_code=1` 与 `cooperation_display_tab_code=2` 限定该视角。
