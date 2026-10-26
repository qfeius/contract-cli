# 交接请求与结果核对

仅在 [主流程](../SKILL.md) 的环境、范围和确认条件满足后调用。参数表中的“本流程要求”与接口本身是否有默认值不同，见 [调用参数约定](transport.md)。

## 执行交接

`POST /open-web/web/clm/permission/handover`

| 参数 | 位置 | 类型 | 本流程要求 | 值、来源与限制 |
|---|---|---|---|---|
| `manual` | body | boolean | 必须传 | 固定 `true`。 |
| `reason` | body | string | 必须传 | 固定 `""`。 |
| `targetEmployeeId` | body | string（内部 ID） | 必须传 | 已确认接收人的内部 employeeId，写前重新读取。 |
| `targetTenantId` | body | string（内部 ID） | 必须传 | 接收人详情 tenantId，必须与当前操作者 tenantId 相同。 |
| `resourceList` | body | array&lt;object&gt; | 必须传 | 非空数组，1～100 个去重入口；每项字段见下表。数量是 Skill 策略，不是接口已测容量；不传 null、空数组或动态条件。 |

关键响应字段（路径相对完整响应）：

| 含义 | 路径 | 类型 | 可为空 |
|---|---|---|---|
| 本次返回授权 | `$.data.authorizations` | array | 是 |
| 授权 ID | `$.data.authorizations[*].authorizationId` | array | 是 |
| 业务对象 ID | `$.data.authorizations[*].businessId` | array | 是 |
| 业务类型码 | `$.data.authorizations[*].businessTypeCode` | array | 是 |
| 接收人 ID | `$.data.authorizations[*].authorizedEmployeeId` | array | 是 |
| 接收租户 ID | `$.data.authorizations[*].authorizedTenantId` | array | 是 |
| 操作人 ID | `$.data.authorizations[*].approvedEmployeeId` | array | 是 |
| 创建时间 | `$.data.authorizations[*].createTime` | array | 是 |

## resourceList 与请求示例

| 字段 | 类型 | 本流程要求 | 值、来源与限制 |
|---|---|---|---|
| businessId | string（内部 ID） | 必须传 | 普通入口取 data.contractId；父组入口取 data.contractAssemblyId，必须与类型和对象对应 |
| businessTypeCode | integer | 必须传 | 取同一入口详情 data.businessTypeCode；枚举与可提交范围见 [资源类型表](resource-types.md)，不由用户随意选码 |
| parentId | string（内部 ID） | 必须传 | 同一入口详情 data.groupId；不是 assemblyId 或搜索聚合组 ID。缺失时停止，不依赖后端纠正 |

以下只展示 JSON 结构，所有示例 ID 必须替换为本次实际读取且已确认的数据；大整数保持字符串：

```json
{"manual":true,"reason":"","targetEmployeeId":"9223372036854775804","targetTenantId":"9223372036854775803","resourceList":[{"businessId":"9223372036854775806","businessTypeCode":0,"parentId":"9223372036854775805"}]}
```

不发送 `handoverByAdmin`、operatorId、onlyOwner、onlySubmitter 等身份或角色选择参数；也不把内部核对标记、搜索条件写进 body。100 是本 Skill 的单批入口策略，不是服务端容量承诺。组不拆成成员逐个写，重叠影响不跨批。

## 普通合同的预期变化

| 交接前 | 交接后 |
|---|---|
| 当前归属人与申请人相同 | 两者均变更为接收人 |
| 当前归属人与申请人不同 | 归属人变更，申请人保持原值 |

此规则适用于普通归属人交接路径，不是权限判定。管理员第三方交接分支由服务端按当前权限与角色决定，不提交管理员标记。有全部权限不代表一定进入第三方分支；仅申请人操作的未验证角色分支不得套用归属人规则执行。仅要求修改申请人或需求人时不调用交接。

普通合同成功时部门按接收人第一部门更新，无部门则保留；需求人不应被交接修改。这些字段有现成结果可另行说明，不把其缺证当作指定合同的角色交接失败。关联记录、草稿、流程、旧授权及通知由后端既有规则处理，不能承诺全部审批待办迁移、原人丧失所有查看权或所有旧提醒停止。

## 逐份判定

保存写前角色和历史记录。调用一次后，以本次授权中的 businessId、businessTypeCode、authorizedEmployeeId、authorizedTenantId，与写后新增交接记录及角色前后变化核对。回读接口见 [合同详情](contracts.md) 或 [合同组](assemblies.md)。历史记录中的 preAuthorizedEmployee、authorizedEmployee、approvedEmployee 分别是交接前人员、接收人、操作人，不能互换。

| 输出状态 | 判定 |
|---|---|
| 成功 | 本次授权／新增交接记录能与对象和接收人对应，角色回读符合预期 |
| 无新增变化 | 写前已经是目标状态，未证明本次新增交接；不计为本次成功 |
| 未执行 | 确认前排除、取消，或后续批次没有发送 |
| 跳过 | 有可靠证据证明服务端未执行，说明已知原因 |
| 失败 | 接口或可靠回查明确该对象失败 |
| 待核实 | 已发送但超时、缺响应、缺记录、字段矛盾或回查不可用 |

只有整体 authorizations，没有完整逐项失败／跳过清单。HTTP 200、success=true、空授权或数量不足不能判全部成功；403/404 或列表消失也不能独自判成功或失败。已有目标角色或旧历史不能当本次成功证据。

请求不是整批原子事务，超时或失败可能已经部分完成。出现不确定结果时停止后续批次，只读回查；不自动重试、不切换 CLI/MCP 重放、不交回原人当回滚。即使后来核实成功也不自动续发被停止批次。用户明确续办时，重新预检剩余范围；未决对象及与其有共同影响的对象不得再次提交。

指定普通合同只报告指定入口的结果；关联影响未核实应单独列出，不声称全量副作用已通过。

## 默认值、附加字段与记录关联

- manual 固定 true，不依赖后端默认值；reason 固定空字符串是已有调用形态。服务端若明确拒绝则报告业务错误，不猜原因或自动重发。
- targetTenantId 在后端可省略并默认当前租户，本 Skill 仍显式传已核实同租户值；targetEmployeeId 必须是接收人的内部 ID，不能以操作者、开放 user_id 或姓名替代。
- resourceList 每项不传 tenantId，沿用登录租户。后端模型还有 handoverByAdmin 等字段，不属于本 Skill 允许构造的参数；也不能依赖“未知字段被忽略”来添加参数。
- 本次 authorizations 的 ID、业务类型、接收人和租户要对应当前清单；approvedEmployeeId 是操作人，不是接收人。不能只用返回条数等于入口数判成功，组成员可能只有父组授权。
- nullable 不等于可据空值宣布成功：data/authorizations 缺失、空数组或关键关联字段为空均不能形成完整成功证据。状态由本次授权／新增记录与角色回读共同决定。
