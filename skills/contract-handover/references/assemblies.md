# 合同组：识别、完整读取与交接

合同组须完整读取父组与有效成员，并满足当前权限、状态和影响范围条件，不能套用普通合同角色规则。有后继、关联范围不明或缺失回读路径时不写入；只有本次状态确实依赖未核实灰度配置时，才保留该准入缺口，不把其他场景存在灰度开关当作所有合同组的额外前置。父组类型与当前通道权限不能可靠确认时停止，不能把其他组类型的成功结果作为本次可执行依据。

参数表约定见 [调用参数约定](transport.md)；业务类型码与 ID 选择见 [资源类型](resource-types.md)。

## 父组定位与读取入口

父组及成员可以使用下面的两个组接口完成交接预检和结果回查，无需逐份读取成员 `contract/info`。按本次已核实的信息选择入口：

| 已知信息 | 后续读取 |
|---|---|
| 已有同环境、同租户上下文中的可信父组 ID | 直接读取父组 info，再用 childrenInfo 读取完整成员 |
| 用户只给合同编号，或已有搜索成员但缺父组 ID | 按 [编号自动定位父组](assembly-lookup.md) 补查原始分组，再读取父组及完整成员；不要求手动提供 ID |
| 成员响应实际提供 assemblyId，且来源与 ID 语义已核实 | 将其作为父组候选，用父组及成员接口核对关系 |
| 可信父组页面链接 | 仅按已确认的链接格式解析父组候选，再读取组接口；链接格式不明时不猜参数含义 |
| 搜索仅提供成员 contract_id、business_type_code=11 与 group_id | 使用原用户编号或该成员响应中的编号自动补查；group_id 是关联链标识，不重命名为 contractAssemblyId |

父组 ID 必须经过接口关系核对：父组 info 返回的 contractAssemblyId 与候选一致；完整成员的 assemblyId 均指向该父组；用户原先选择的成员 contractId 必须在成员集合中。父组编号输入须与 contractAssemblyNumber 一致；成员编号输入核对原选成员，不要求父编号与成员编号相同。还需核对名称与用户目标是否一致。诊断材料中的 ID 只能作为候选，不直接据此构造写请求。

组成员可能没有独立流程表单。成员 `contract/info` 出现非权限读取异常时，不能据此断言父组接口不可用，也不把该异常本身当作父组 ID 或表单状态的证据。已有可信父组候选时，继续上述组接口核对；没有父组 ID 时先执行编号自动定位。只有零命中、歧义、权限或关系核对失败等实际缺口才停止，不把提供父组页面链接／ID 当作固定前置，不重复请求已失败的成员详情，也不猜测查询参数或枚举其他接口。

HTTP 或业务 401/403、明确权限拒绝不适用上述分流，不以切换父组入口绕过，按当前授权与权限错误处理。写请求已发送后的失败或不确定结果，仍按交接结果规则只读核对，不重放写请求。

成员读取固定 `needApproveRecord=false`，从组列表获取当前角色和成员关系。它不代表成员详情异常已经修复，也不免除完整成员、权限、接收人、整组影响和确认条件。写后仍使用相同两个组接口核对父组及每份成员角色、需求人和本次父组授权／新增历史；缺字段或回读矛盾时保留待核实结果，不改用成员完整详情强行补齐。

## 父合同组详情与回查

`GET /open-web/web/clm/contractAssembly/info`

请求形态示例（姓名和 ID 仅为占位，执行前按本次实际值替换）：

```http
GET /open-web/web/clm/contractAssembly/info?contractAssemblyId=30001&batchContractChildrenPageIndex=0&batchContractChildrenPageSize=100&needChildrenInfo=false
```
| 参数 | 位置 | 类型 | 本流程要求 | 值、来源与限制 |
|---|---|---|---|---|
| `contractAssemblyId` | query | string（内部 ID） | 必须传 | 已确认的父合同组 ID，不能填搜索结果外层 groupId。 |
| `batchContractChildrenPageIndex` | query | integer | 必须传 | 父组详情固定 `0`，不把详情接口当成员翻页接口。 |
| `batchContractChildrenPageSize` | query | integer | 必须传 | 固定 `100`。 |
| `needChildrenInfo` | query | boolean | 必须传 | 固定 `false`，完整成员通过 childrenInfo 分页读取，不依赖内嵌 contracts。 |

关键响应字段（路径相对完整响应）：

| 含义 | 路径 | 类型 | 可为空 |
|---|---|---|---|
| 父组 ID | `$.data.contractAssemblyId` | string | 否 |
| 父组编号 | `$.data.contractAssemblyNumber` | string | 是；以父组编号定位时必须返回并一致 |
| 名称 | `$.data.contractAssemblyName` | string | 是 |
| 业务类型码 | `$.data.businessTypeCode` | integer | 否 |
| 状态码 | `$.data.contractStatusCode` | integer | 否 |
| 关联 groupId | `$.data.groupId` | string | 是 |
| 前序 ID | `$.data.previousId` | string | 是 |
| 后继 ID | `$.data.nextId` | string | 是 |
| 当前归属人 ID | `$.data.ownerEmployee.employeeId` | string | 是 |
| 当前申请人 ID | `$.data.submitterEmployee.employeeId` | string | 是 |
| 合同部门 ID | `$.data.department.departmentId` | string | 是 |
| 有效成员总数 | `$.data.childrenCount` | integer | 是 |
| 成员列表 | `$.data.contracts` | array | 是 |
| 交接历史 | `$.data.handover.records` | array | 是 |

## 合同组成员与回查

`GET /open-web/web/clm/contractAssembly/childrenInfo`

请求形态示例（姓名和 ID 仅为占位，执行前按本次实际值替换）：

```http
GET /open-web/web/clm/contractAssembly/childrenInfo?contractAssemblyId=30001&batchContractChildrenPageIndex=0&batchContractChildrenPageSize=100&needApproveRecord=false
```
| 参数 | 位置 | 类型 | 本流程要求 | 值、来源与限制 |
|---|---|---|---|---|
| `contractAssemblyId` | query | string（内部 ID） | 必须传 | 已确认的父合同组 ID，不能填搜索结果外层 groupId。 |
| `batchContractChildrenPageIndex` | query | integer | 必须传 | 从 0 开始，每页加 1。 |
| `batchContractChildrenPageSize` | query | integer | 必须传 | 固定 `100`。 |
| `needApproveRecord` | query | boolean | 必须传 | 固定 `false`，不请求成员审批记录；仍读取当前角色。 |

关键响应字段（路径相对完整响应）：

| 含义 | 路径 | 类型 | 可为空 |
|---|---|---|---|
| 成员列表 | `$.data.contracts` | array | 否 |
| 成员合同 ID | `$.data.contracts[*].contractId` | array | 否 |
| 租户 ID | `$.data.contracts[*].tenantId` | array | 是 |
| 父组 ID | `$.data.contracts[*].assemblyId` | array | 是 |
| 关联 groupId | `$.data.contracts[*].groupId` | array | 是 |
| 业务类型码 | `$.data.contracts[*].businessTypeCode` | array | 是 |
| 合同编号 | `$.data.contracts[*].contractNumber` | array | 是 |
| 合同名称 | `$.data.contracts[*].contractName` | array | 是 |
| 状态码 | `$.data.contracts[*].contractStatusCode` | array | 是 |
| 前序 ID | `$.data.contracts[*].previousId` | array | 是 |
| 后继 ID | `$.data.contracts[*].nextId` | array | 是 |
| 归属人 ID | `$.data.contracts[*].ownerEmployee.employeeId` | array | 是 |
| 申请人 ID | `$.data.contracts[*].submitterEmployee.employeeId` | array | 是 |
| 创建人 ID | `$.data.contracts[*].createEmployee.employeeId` | array | 是 |
| 需求人信息 | `$.data.contracts[*].demandEmployees` | array | 是 |
| 需求人 ID 列表 | `$.data.contracts[*].demandEmployees[*].employeeId` | array | 是 |

## 组范围与结果规则

- 类型统一按 [资源类型表](resource-types.md) 解释。父组类型 10/17 与成员类型 11/0 不可互换；核对真实详情与成员关系，不能用搜索外层 groupId 代替父组 ID。
- 成员从 pageIndex=0、pageSize=100 逐页读取到空页，按 contractId 去重，与父组前后 childrenCount 核对。父组 contracts 可能只有首批，不能直接当全部。数量不符或漂移时重新核实，不能宣布成员完整。
- 只选择组内一份合同但用户明确其他成员不得改变时，说明整组影响并停止；不能静默把父组及其他成员加入已授权范围。
- 写入以父组真实 ID、类型和 groupId 形成一个 resourceList 入口。组整体确认，不能拆成成员来规避范围限制。
- 功能权限已核实包含 `manage_contract_handover_all` 时，现有交接接口在服务端设置管理员交接标记，按旧父组归属人计算下面的角色变化；Agent 不再索取额外管理员标记，也不在请求中添加该字段。仅个人权限仍按本次实际角色核对；功能权限缺失或资源读取拒绝不由此放行。
- 合同组申请（类型 10）处于审批中（状态 3）时，不属于草稿／协商的灰度准入分支；在真实父组、完整成员、无后继、权限、接收人和整组范围均已核实时，可整理并确认具体清单，不额外要求未知灰度开关证明。其他类型或状态仍按其实际规则核对，最终由交接接口校验。
- 父组角色分别计算：实际操作人等于旧归属人且接收人不同，改变归属人；实际操作人等于旧申请人且接收人不同，改变申请人。服务端允许的第三方管理员分支以旧归属人计算。两个标记都不成立是无新增变化。只更换申请人的请求不借合同组路径变相执行。
- 所有有效成员跟随父组的两个角色标记，不能按每个成员自身角色关系重新计算。父组有效交接后按接收人首部门更新，无部门保留。子合同部门由后端处理，不作为读取或结果判据，不用父组部门或员工部门补值。
- 本次父组授权／新增历史 + 完整成员关系 + 每份成员角色和需求人回读，可以证明成员结果；不要求每份成员独立授权记录。成员缺失、角色不符或缺回读的对象待核实。
- 流程归属、旧授权停用、通知发送与送达分别核验。没有通知证据不自动判角色交接失败，也不能声称副作用全通过。没有合法回查方法时明确保留未核实项，不能先假定它们已发生。

## 参数与分页边界

两个接口的 contractAssemblyId 都是正十进制父组 ID；batchContractChildrenPageIndex 为非负整数，batchContractChildrenPageSize 为正整数，本流程固定 100。父组 info 只传 pageIndex=0；childrenInfo 才逐页加 1，不使用合同搜索的分页令牌。

needChildrenInfo=false 表示不请求内嵌完整成员，不保证响应中 contracts 一定为空；成员完整性以 childrenInfo 全分页加父组 childrenCount 核对。needApproveRecord=false 表示不要求成员审批记录，不是关闭当前角色回读。

childrenCount 应为非负整数；缺失、成员缺 ID、重复与计数不一致、前后总数变化都不能判断成员完整。nextId<=0/null 仅用于无后继父组条件，不是全部关联链已查全的证明。taskInstanceId、noticeId、mobile 等其他入口参数不在本流程中传入，也不借它们替换权限上下文。
