# mdm vendor patch Parameters

本页专用于 `contract-cli mdm vendor patch`。

- App 接口：`PATCH /open-apis/mdm/v1/vendors/{vendor_id}`
- 个人接口：`PATCH /open-apis/contract/v1/mcp/vendors/{vendor_id}`
- 身份：`app` 或 `user`
- 请求体：JSON object 必填，`--input-file` 与 `--data` 二选一
- 官方 OpenAPI：本次新增 V1 交易方局部更新接口；V2 不在本期范围
- 动态必填和自定义字段值属性统一按 [交易方字段配置解释规则](../../contract-cli-mdm-fields/references/vendor-field-config-semantics.md) 处理。

## 目录

- [CLI 参数映射](#cli-参数映射)
- [请求体字段](#请求体字段)
- [枚举与约束](#枚举与约束)
- [示例](#示例)

## CLI 参数映射

| 参数 | 请求位置 | 类型 | 必填性 | 说明 |
| --- | --- | --- | --- | --- |
| vendor-id | path.vendor_id | string | 必填 | 正十进制 64 位 ID；只放 path。 |
| --input-file | body | JSON file | 二选一必填 | 与 `--data` 互斥。 |
| --data | body | JSON string | 二选一必填 | 与 `--input-file` 互斥。 |
| --as | 本地上下文 | enum | 可选 | `user` 或 `app`；不传时使用 profile 默认身份。 |
| --user-id | query.user_id | string | app 必填，user 禁止 | App 当前操作人；user 操作人来自认证。 |
| --user-id-type | query.user_id_type | string | 可选 | 默认 `user_id`。 |
| --department-id-type | query.department_id_type | enum | user 可选，app 禁止 | `department_id` 或 `open_department_id`；`ownerDepts` 传 `od-...` 时必须传 `open_department_id`。 |
| --profile | 本地上下文 | string | 可选 | 不传时使用当前 profile。 |
| --output | CLI 输出 | enum | 可选 | `json`、`yaml` 或 `table`。 |
| --raw | CLI 输出 | boolean | 可选 | 原样输出响应。 |

## 请求体字段

PATCH 复用旧 V1 PUT 的字段名称和类型，完整字段定义见 [vendor-update-parameters.md](vendor-update-parameters.md)。以下是 PATCH 特有契约：

| JSON 路径 | 类型 | 必填性 | 处理方式 |
| --- | --- | --- | --- |
| id | string | 禁止 | ID 只允许放 path。 |
| 普通可编辑字段 | 原字段类型或 null | 可选 | 未传不改；`null` 请求清空；有值更新。 |
| status | integer | 仅 app 可选 | App 可传 `0` 或 `1`；个人禁止。 |
| vendor | string | 个人禁止 | 编码不可由个人修改。 |
| vendorContacts | array<object> | 可选 | 按 ID 局部合并；无 ID 新增。 |
| vendorAccounts | array<object> | 可选 | 按 ID 局部合并；无 ID 新增。 |
| vendorAccounts[].bankId | string | app 可选，个人禁止 | App 沿用 V1 PUT 字段能力；个人维护中该字段只读。 |
| vendorAddresses | array<object> | 可选 | 按 ID 局部合并；无 ID 新增。 |
| vendorCompanyViews | array<object> | 可选 | 按 ID 局部合并；无 ID 新增。 |
| 子项._delete | boolean | 可选 | 删除时必须为 `_delete: true` 且携带现有 ID。 |
| extendInfo | array<object> | 可选 | 按 `fieldCode` 合并。 |
| 附件、人员、部门、多选字段 | array | 可选 | 字段出现时整体替换；未出现保持不变。 |

## 枚举与约束

- 请求体必须是非空 JSON object；未知字段、重复 JSON key 和别名冲突会被拒绝。
- 未传字段不修改，`null` 请求清空；能否清空以最终快照的动态必填和字段规则为准。
- 空字符串是字符串值，按该字段原规则校验，不作为其他类型的通用清空标记。
- `0`、`false` 和空数组都是已传值，不得按“未传”处理。
- 四类子项未列出的记录保留；要删除必须显式传现有 ID 和 `_delete: true`。
- 子项字段为 `null` 时拒绝；`[]` 表示零条子项操作，不会删除现有记录。
- 自定义日期区间 `rangeDate` 非空时必须恰好包含开始、结束两个 `yyyy-MM-dd` 字符串；清空时传 `null`，`rangeDate: []` 非法。多选、附件和人员值仍使用 `[]` 请求清空。
- 个人 PATCH 不允许编码、状态、风险和系统字段；可编辑停用交易方，更新后仍停用。
- 个人 PATCH 不允许传 `vendorAccounts[].bankId`；不要为了写入该字段切换为 App 身份。
- 写入不发起审批，但个人请求与既有在途审批冲突时会失败。
- 返回 `APPLIED` 表示生效，`NO_CHANGE` 表示业务值未变化；`UNKNOWN` 时先查询，不直接重试。

## 示例

自定义日期区间设置、清空和不修改：

```json
{
  "extendInfo": [
    {
      "fieldCode": "VBI00110005",
      "fieldType": 8,
      "rangeDate": ["2026-09-22", "2027-09-22"]
    }
  ]
}
```

```json
{
  "extendInfo": [
    {
      "fieldCode": "VBI00110005",
      "fieldType": 8,
      "rangeDate": null
    }
  ]
}
```

不修改日期区间时，不提交该 `fieldCode`。`rangeDate: []` 是非法日期区间，不能用于清空。

个人修改简称、清空电话并更新一个联系人：

```bash
contract-cli mdm vendor patch 7003410079584092448 --profile contract --as user --data '{"shortText":"新简称","contactTelephone":null,"vendorContacts":[{"id":"7003410079584092450","phone":"13800000000"}]}'
```

个人使用部门查询返回的 `open_department_id`：

```bash
contract-cli mdm vendor patch 7003410079584092448 --profile contract --as user --department-id-type open_department_id --data '{"ownerDepts":["od-xxx"]}'
```

App 修改状态和简称：

```bash
contract-cli mdm vendor patch 7003410079584092448 --profile contract --as app --user-id <operator-user-id> --data '{"status":0,"shortText":"新简称"}'
```

显式删除联系人：

```bash
contract-cli mdm vendor patch 7003410079584092448 --profile contract --as user --data '{"vendorContacts":[{"id":"7003410079584092450","_delete":true}]}'
```
