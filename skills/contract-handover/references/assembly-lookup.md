# 按合同编号自动定位父组

仅在用户提供合同编号，或已有搜索成员携带编号但缺少真实父组 ID 时使用。一般合同搜索仍委托 [合同搜索 Skill](../../contract-cli-contract-search/SKILL.md)；此处只补充分组关系，不替代原查询、不扩大用户选择，也不提交交接。已有可信父组 ID 时直接读取组接口。

## 请求

`POST /open-web/web/clm/es/search/contractList`

沿用本次交接的环境、租户和 user 授权。使用 [通道约定](transport.md) 的输入文件调用，不要求用户提供父组 ID、页面链接、Cookie 或 Token。

下面只替换 `searchValue` 为用户完整合同编号或已选成员的原始编号；不要把编号转为数字。

```json
{
  "conditionUnits": [],
  "filterUnits": [
    {
      "searchField": "CONTRACT_NUMBER_FIELD",
      "searchValue": "CG-DEMO-01",
      "unionType": "MUST"
    }
  ],
  "order": "DESC",
  "pageIndex": 0,
  "pageSize": 10,
  "searchTabCode": 0,
  "sortType": "SUBMITTED_TIME",
  "withStatistics": false
}
```

| 参数 | 位置 | 类型 | 本流程要求 | 值、来源与限制 |
|---|---|---|---|---|
| `conditionUnits` | body | array | 必须传 | 固定空数组。 |
| `filterUnits` | body | array | 必须传 | 固定一个编号条件。 |
| `filterUnits[0].searchField` | body | string | 必须传 | 固定 `CONTRACT_NUMBER_FIELD`。 |
| `filterUnits[0].searchValue` | body | string | 必须传 | 用户提供的完整编号；已有成员候选时可用该成员响应中的编号。不得填 groupId 或父组 ID。 |
| `filterUnits[0].unionType` | body | string | 必须传 | 固定 `MUST`。 |
| `order` | body | string | 必须传 | 固定 `DESC`。 |
| `pageIndex` | body | integer | 必须传 | 固定 `0`，仅执行这一页关系定位。 |
| `pageSize` | body | integer | 必须传 | 固定 `10`，计数单位为搜索分组，不是展开成员数。 |
| `searchTabCode` | body | integer | 必须传 | 固定 `0`（全部合同中的当前用户可见范围）。不是管理员接口，不扩大权限。 |
| `sortType` | body | string | 必须传 | 固定 `SUBMITTED_TIME`。 |
| `withStatistics` | body | boolean | 必须传 | 固定 `false`；不能据此忽略本次返回的分组完整性。 |

不补传 tenantId、employeeId、userAuthority 或其他身份、权限字段。不调用管理员搜索，不关闭权限检查。

## 解析与消歧

按 Web 的 `code=200` 且 `success=true` 判断业务成功，再读取以下字段。保留 ID 的全部十进制位；响应 ID 即使编码为 JSON 整数，也不能经过 JavaScript Number。

| 含义 | 路径 | 类型 | 用法 |
|---|---|---|---|
| 分组列表 | `$.data.content` | array | 保留父行与成员的分组关系，不能先扁平化丢失父行。 |
| 命中分组总数 | `$.data.filteredTotalElements` | integer | 本配方只处理 `0..10` 且等于 content 长度的完整一页。 |
| 分组类型 | `$.data.content[*].groupType` | integer | `1` 表示真实合同组；`0` 的普通合同／版本链不能当成父组。 |
| 关联标识 | `$.data.content[*].groupId` | string | 关联链标识，不是父组 ID。 |
| 组内父行与成员 | `$.data.content[*].searchContractListItems` | array | 逐行读取类型、编号和标识。 |
| 行业务类型 | `…searchContractListItems[*].businessTypeCode` | integer | 父组候选为 `10/17`；不能将 `groupType=1` 当作业务类型。 |
| 行实体 ID | `…searchContractListItems[*].contractId` | string | 父组行的正 ID 才是父组候选。 |
| 行父组 ID | `…searchContractListItems[*].assemblyId` | string | 成员应指向父行 contractId；父行可为负哨兵，不读取它作为父组 ID。 |
| 行编号／名称 | `…searchContractListItems[*].contractNumber` / `contractName` | string | 精确核对编号，展示名称用于业务消歧。 |

1. 搜索可能按编号部分匹配并展开整组，必须逐行比较 `contractNumber` 与本次编号完全相等，不能把同组的其他行当命中。已有用户选定成员时，还须核对其 contractId 与来源上下文一致。
2. 在同一个 `groupType=1` 分组内确认只有一个类型 `10/17` 的父行，取其 `contractId` 为候选。父编号命中时由该父行定位；成员编号命中时，还要核对命中成员的正 `assemblyId` 等于父行 contractId。多个父行或标识矛盾时停止核对，不取数组第一项，也不以名称或 groupId 推算 ID。
3. 当前页必须完整：filteredTotalElements 为非负整数、不超过 10，并与 content 长度一致。缺计数、计数矛盾或多于一页时不认定唯一父组，不自行展开深分页或枚举接口；说明需要更精确的完整编号。多个不同父组精确命中时，展示编号、名称等业务候选让用户选择，不要求用户填写内部 ID。
4. 无精确命中时报告“未找到当前用户可见的对应合同”，不能用部分匹配项继续。只命中普通合同则按普通详情流程处理，不能制造父组。父行缺失、类型未知或成员关系矛盾时保留定位缺口，不用 groupId 或负哨兵补值。
5. 得到唯一父组候选后，继续 [父组及完整成员核对](assemblies.md#父组定位与读取入口)：父组 info 的 contractAssemblyId、类型和可用编号与候选一致；命中成员必须存在于完整成员集合，所有成员 assemblyId 指向该父组。父编号还要与 info 的 contractAssemblyNumber 一致。

搜索分组中的成员不是完整有效成员清单，不能据此省略 childrenInfo 分页与父组前后 childrenCount 核对，也不直接从搜索旧角色生成写请求。

成员完整详情发生非权限错误（含 110002、110302）时，可走本条已验证的关系定位，不重复请求失败的成员详情。HTTP／业务 401、403 或明确权限拒绝时停止，不用分组定位绕过权限。查询失败不换身份或环境重试；交接请求已发送后的未决结果，只能按原清单回查，不能重新搜索后再提交。
