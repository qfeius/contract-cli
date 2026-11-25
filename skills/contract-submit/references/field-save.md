# 独立保存合同表单

## 保存范围

接收用户指定的上传合同、目标字段和本次要保存的值。依次使用[身份与通道](runtime.md)、[对象定位](discovery.md)和[当前表单](form.md#取得当前表单)。用户已明确授权保存且给定了唯一对象和明确值时，沿用其授权。只给出要验证的字段、未给具体值的业务请求，先展示拟填值和依据。受控测试若明确授权选择临时值，记录原值和恢复计划。

- 表单二合一处于协商中时，用真实合同 ID 读正式表单。以合同基础信息核当前 USER 属于本租户且是合同负责人，以协商详情核对应协商 ID、当前轮、可编辑状态 `formEditable=true` 及同一轮正式表单 `enableFormCooperation=true`。按[协商详情读取边界](cooperation-submit.md#1-核适用动作和清洁方式)使用本轮获权的页面读取。非二合一的独立前置表单走[前置表单协议](cooperation-preform.md)。文件信息、模板正文绑定字段及合同组成员另按各自协议。
- 从当前 `moduleInfos[].attributeInfos[]` 以真实 `attributeKey` 定位，核对所在模块和组、启用状态、显隐联动以及实际写权限。当前 `processAttributePermission.attributePermissions[]` 中对应的 `permissionTypeCode=3` 是已核写权限；实际 `editDisabled=true` 或条件隐藏时停止该字段。权限结果或表单对应关系缺失时保持待核实。显示名仅供用户识别，同名时先用分组或实际业务含义消歧。
- 只保存按字段本身的实际规则验证。送审必填项尚未补齐或当前轮服务端清洁关闭，均可继续符合当前编辑规则的保存。此分支在读回字段和状态后返回。用户同时要求送审时再按[送审协议](submit.md)判断分流并取得对应动作确认。

## 字段取值

下面是主表单及合同金额组内自定义控件的已核映射。通过实际 `attributeTypeCode`、`groupKey`、当前 `valueScope` 及联动规则识别控件。扩展配置采用有效的 `extraInfoJson` 对象，或解析 `extraInfo` 的 JSON 字符串。两处均存在时核对影响取值的设置，冲突或解析失败时先补证。

| 当前控件 | `schemaValueMap[attributeKey]` 中的值 |
| --- | --- |
| 部门选择 `1002` | 选中的部门内部 `departmentId` 字符串数组。单选也是单元素数组。按实际限制核生效层级、候选范围和当前关联人员。 |
| 人员选择 `1001` | 选中的员工内部 `employeeId` 字符串数组。单选也是单元素数组。两个代理人字段各用自己的键保存。 |
| 单选项 `13` | 选中的一个实际 `valueScope[].value` 字符串。按选项和值域配置核对可用性和联动选项。 |
| 下拉多选 `16` | 各选项实际 `valueScope[].value` 字符串数组，保持独立选项和有效集合。 |
| 数值 `5` | JSON 数值。先按字段精度、范围和单位判断合法性。扩展配置 `showPercent=true` 时，界面 6% 存为数值 `0.06`。普通数值 1 存为数值 `1`。 |

用 [运行上下文](runtime.md) 的相同 CLI 和 WEB USER 读取候选。结果须满足当前租户、实际权限和字段生效范围，取得可选实体的精确内部 ID。无候选、同名、离职或不可选择，分别反馈用户。按页面行为保留仍有效的旧选项。字段擦除只有在用户明确授权且已核当前控件的实际清空值时执行。未改字段继续带原值。

- 部门层级场景用 `GET /open-web/web/clm/department/findListByKeyword`。query `keyword` 用用户输入，`lowRange/highRange` 取当前启用的层级上下限 `rangeStart/rangeEnd`。字段范围按实际规则需要时传 `attributeId`，与人员联动时使用页面实际 `ownerEmployeeId`。采用成功响应 `data[]` 的 `departmentId` 与显示名，按实际状态及 `level` 核候选。例如一级范围只采用有效一级部门。额外返回的全部部门根节点不符合该范围。限定部门或其他联动规则无法核实时停止该字段。
- 无额外范围限制的人员场景可用 `GET /open-web/web/clm/employee/findEmployeesByKeyword`。query 为 `keywords`，页面允许本人作为候选时可传 `filterCurrentEmployee=false`。按当前生效的人员可见规则使用 `filterVisibleEmployee`，以成功响应 `data[]` 中当前租户的在职、可选 `employeeId` 和显示名映射。存在指定部门、角色、人员等范围时，仅沿当前页面的受限查询验证候选及 `canSelect`，保留所需实际 `attributeId/attributeKey`，无法确认时暂停对应字段。人员库的公开用户 ID 不用于合同表单值。

`groupKey=amount_group` 的自定义数字子字段仍分别平铺在 `schemaValueMap`，各用自己的叶子键。无收无支时按页面实际规则判断这些子字段的显隐和可写状态。金额组中系统金额、币种、预算税率、固定汇率折算结果，以及签约模块中的系统签章、签约人或交易方主体，按各自组件和存储协议识别。它们仅靠同名无法套用这里的自定义字段映射。明细表行保留实际 `tableValueMap` 或专用数据结构。

## 组装请求

仅保存正式合同字段使用 `PUT /open-web/web/clm/contract/update`，无 query，使用同一合同最新详情按 [运行上下文](runtime.md) 发送 JSON。当前对象、轮次、业务状态、表单、字段规则或用户权限发生变化时，重新核对变更后再保存。上传正文及交易方、签约、预算等独立存储动作按已有专项协议，字段保存无需改变它们。

| 更新体 | 来源与保留方式 |
| --- | --- |
| `contractId`、`formId` | 当前正式详情的合同 ID 和表单 ID，核对本轮有效绑定。 |
| `departmentId` | 同详情 `department.departmentId` 或实际返回的同义标准字段，保留合同归属部门。 |
| 合同命名及说明 | 保留 `contractName`、`contractNameVO`、`contractDesc`。涉及已存在的 `billingDesc`、`settlementTerm`、`contractChangeRemark`、`contractDemandPerson`、`sealNumber` 时沿用详情有效值。 |
| 收支和金额 | 保留 `payType`、`contractPropertyTypeCode`。详情的标量 `totalAmount`、`estimatedAmount` 分别与当前 `currency` 组成 `{amount,currency}`。双向金额 `totalInAmount`、`totalOutAmount`、`estimatedInAmount`、`estimatedOutAmount` 已是对象时保留其形状。标量按相应 `inCurrency` 或 `outCurrency` 转为金额对象。 |
| 期限、签约相关普通字段 | 保留 `fixedValidity`、`fixedValidityDetail`、`dateRange`、`validityDateDesc`、`contractDuration`、`contractDurationUnit`、`contractEffectiveStatus`、`firstSigner` 的当前映射。详情仅有 `startDate/endDate` 时按实际期限类型和页面格式转换。适用的长期备注、签订日期等已有数据按当前页面映射保留。 |
| `schemaValueMap`、`presetSchemaValueMap`、`tableValueMap`、`nodeConfigMap` | 采用当前对象原值。仅把本次已验证、授权写入的自定义字段合并到 `schemaValueMap`，其余照原页面形状保留。 |

服务端会给缺失的收支、计价和期限类型设置默认值。保留当前实际语义和本轮授权下的真实联动，发现无法保留的差异先停在写入前。按页面映射构造更新体，不透传整份详情。需修改标准字段时，用上表同一协议和该字段真实联动；原封写回时保留合法 `0`、空值及精确金额。可核目标字段与原值一致时标「已一致」。若任务要求验证实际保存，采用用户授权的有效差异值并执行同样的写后回读。

### 二合一纸质单合同的标准字段

下面适用于已核用户权限、同轮表单可编辑、从空值填写的文件上传类普通单合同。复用相同 USER 通道。标准字段没有单独权限项时，沿用原页申请节点规则。当前节点与实际 `submitProcessNodeId` 相符（未配置时页面默认 `UserTask_submit_0:0`），且字段已启用、页面允许编辑时，可按该分支的默认写权限办理。显式字段权限优先。按每次写后的正式合同详情建立下一步基线。

| 已确认的填写动作 | 原页面保存与回查 |
| --- | --- |
| 从未选签约形式设为纸质签约，双方主体尚为空 | `POST /open-web/web/clm/contract/sign/signType/update`，无 query。JSON 传实际申请的 `businessId`（正式合同 ID）、`businessTypeCode`（普通申请为 `0`）、`signType=1` 和当前详情的 `sealPositionTypeCode`（已核纸签分支为 `0`）。业务成功后回查详情 `signType=1`。已有签约人或盖章配置时先核变更影响和适用协议。 |
| 从空值选择我方 | 用已核主数据候选，按名称及可用性匹配 `legalEntityId`、`legalEntityName`。`POST /open-web/web/clm/contractLegalEntity/createContractLegalEntity`，无 query。JSON 传实际 `businessId`、`businessTypeCode`、所选 `legalEntityId` 和 `legalEntityName`。要求 `code=200`、`success=true` 及返回绑定记录 ID，再核详情 `ourSide` 中同一绑定及主体 ID。已绑定同一主体时复用。 |
| 从空值选择对方 | 走[对方绑定和供方配置](cooperation-submit.md#保存收支类型和交易方)。 |
| 选择合同需求人、固定期限、对方先盖章 | 按上节 `PUT /contract/update` 的完整映射保留原值。根字段 `contractDemandPerson` 用获权员工 ID 的字符串数组。固定起止日期使用 `fixedValidity=1`、`fixedValidityDetail=1`、`dateRange` 为两个 `YYYY-MM-DD`，`firstSigner=1` 为对方先盖章。保留已回查的纸签和实际金额组语义。回查 `contractDemandPerson`、`fixedValidity`、`fixedValidityDetail`、`startDate`、`endDate`、`signType` 和 `firstSigner`。`signTypeCode` 可随先盖章方派生变化，纸签形式仍以 `signType` 比对。 |

## 按项回查

1. 用户要求逐项保存时，每个业务项独立执行。金额组一项可包含多个已确认子键，同一次保存后分别查验两个值。每次使用上一次成功回查的同对象新值作为下一次的基线。调用前固定对象、写权限、原值、拟保存值及关联规则，防止覆盖并发改动。
2. 要求写响应 `code=200` 且 `success=true`，再读取同一份正式表单。按字段实际类型比较回读值，并核对同一合同、表单及轮次、非目标字段和关联的状态与实例。多选确认完整集合，人员和部门用真实候选显示名反馈。成功响应与落库不一致时指出实际结果。
3. 写入响应丢失、超时或合同在保存前后变化时，先回查，区分已保存、待核实及失败。任何未知写入结果先解决现状再办理依赖的后续动作。受控测试需要恢复原值时，恢复同样逐项执行、回查；若并发变化已改变基线，则保留现场并报告。
4. 输出实际保存项、已一致项、失败或待核实项和合同当前状态。只保存的协商合同核同一轮仍处于原状态、审批实例与原值相同，完成字段回查后结束任务。
