---
name: contract-cli-mdm-vendor
version: 1.1.3
description: "contract-cli 交易方主数据技能：支持 user/app 身份查询交易方，按身份创建或局部更新交易方，使用 user 身份启停交易方，以及 app 身份执行旧版全量更新、全量分页和证件查询。当用户要使用 `contract-cli mdm vendor ...` 操作交易方时触发。"
---

# contract-cli MDM Vendor

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

## 能力边界

- 本 Skill 只指导 Agent 执行 `contract-cli`，不会直接调用 Higress MCP 工具。
- 本 Skill 不会改变或增强远程 MCP Server；MCP Agent 的自然语言能力由远程 Tool description 和 InputSchema 独立提供。
- CLI 与 MCP 共用交易方业务口径，但两条执行链路相互独立。不要因为当前环境同时安装了 MCP，就跳过 CLI 命令或混用两边参数。

## 适用命令

- `contract-cli mdm vendor list`
- `contract-cli mdm vendor get <vendor-id>`
- `contract-cli mdm vendor create`
- `contract-cli mdm vendor update <vendor-id>`
- `contract-cli mdm vendor patch <vendor-id>`
- `contract-cli mdm vendor enable <vendor-id>`
- `contract-cli mdm vendor disable <vendor-id>`
- `contract-cli mdm vendor list-all`
- `contract-cli mdm vendor query-by-cert`

## 快速决策

- 查询候选或详情：使用 `list|get`，二者都支持 `user` 与 `app`。user 身份只支持交易方名称模糊查询，app 身份按交易方编码查询；详情始终使用内部交易方 ID。
- 创建交易方：使用 `create`。个人聊天场景用 `--as user`；开放平台应用场景用 `--as app --user-id <operator-user-id>`。
- 只改提交的字段：使用 `patch`。不要用旧 `update` 模拟局部更新。
- 启用或停用：个人聊天场景使用 `enable|disable --as user`，不要用个人 PATCH 改 `status`。
- 老客户需要保持原 PUT 全量更新行为：继续使用 `update --as app --user-id <operator-user-id>`。
- 按证件号或全量分页：使用 `query-by-cert|list-all --as app`。
- 写入前不确定动态字段：先读取 [../contract-cli-mdm-fields/SKILL.md](../contract-cli-mdm-fields/SKILL.md)。

## 自然语言场景决策

- 简单创建：字段明确且已满足当前租户 module 0 的全局必填规则时，使用 `create`，只提交用户确认的字段。
- 复杂创建：用户明确要求包含自定义字段、联系人、地址、账户、公司视图、附件或部门时，先查字段配置和相关 ID，再组织一次创建请求；不要因为子项内部存在必填字段而主动添加该子项。
- 普通字段 PATCH：个人身份先用 `list` 按名称定位交易方，确认唯一目标后用 `patch` 只提交需要修改的字段。用户只提供编码时，不得把个人查询空结果解释为不存在，也不得据此重复创建；应询问名称或内部交易方 ID，不自动切换 App 身份。
- 子项增改删：先用 `get` 获取联系人、账户、地址或公司视图的现有 ID；有 ID 修改、无 ID 新增、`id + _delete:true` 删除，未列出的子项保留。
- 附件替换：只接受用户从 MDM 页面取得的已有 `fileId`；不传 `appendix` 保持不变，传 `[]` 删除全部附件引用，传 fileId 列表整体替换。
- 部门查询后写入：只允许选择 `status=1` 的启用部门；`status=0` 的停用部门不得用于创建或修改。部门查询返回 `od-...` 时使用 `--department-id-type open_department_id`；内部数字 ID 省略该参数或传 `department_id`。
- 启停：用户明确要求启用或停用时使用 `enable|disable`，不要通过个人 PATCH 修改 `status`。
- 结果未知后的查询确认：返回 `UNKNOWN` 或网络中断导致结果不确定时，用 `get` 核对最终状态，不重复发起写请求。

复杂写入遵循固定顺序：字段不确定时先查询字段配置；子项 ID 不确定时先查询交易方详情；存在歧义时先询问用户，不猜字段值、目标记录或删除范围。

## 动态字段配置解释

- module 0 是始终存在的交易方主对象；当前租户在 module 0 配置的必填字段属于全局必填。创建时只询问仍缺失的全局必填字段。
- module 1～4 分别对应经营地址、联系人、银行账户和公司视图。子项集合本身可选；只有用户明确要求提交某类子项时，才检查该子项每条记录内部的必填字段。
- module 5 是智书合同签约信息，不属于交易方创建请求或 PATCH 请求；即使字段配置返回该模块，也不得要求用户补充或写入请求。
- 不得推荐或生成默认业务值。国家、交易方性质、地址、联系人、银行账户、公司代码等缺失时，应询问用户或展示合法候选值，不得代替用户选择。
- 静态参数表中的 App V1 必填标记不能覆盖个人接口的租户动态配置。完整解释规则见 [../contract-cli-mdm-fields/references/vendor-field-config-semantics.md](../contract-cli-mdm-fields/references/vendor-field-config-semantics.md)。
- 主对象和四类子项的 `extendInfo` 必须按同一参考文档映射值属性：0/1/3/5 用 `fieldValue`，2 用 `num`，4/6 用 `options`，7 用 `date`，8 用 `rangeDate`，12 用 `appendix`，14 用 `employee`。每个自定义字段只提交一个对应值属性。
- 日期区间是由开始、结束两个日期组成的复合值：设置或更新时传两个 `yyyy-MM-dd` 字符串，清空时传 `rangeDate: null`，不修改时不提交对应 `fieldCode`；`rangeDate: []` 非法。多选、附件和人员字段仍使用空数组请求清空。

## 身份与路由

- `list --as user` -> `GET /open-apis/contract/v1/mcp/vendors`
- `list --as app` -> `GET /open-apis/mdm/v1/vendors`
- `get --as user` -> `GET /open-apis/contract/v1/mcp/vendors/{vendor_id}`
- `get --as app` -> `GET /open-apis/mdm/v1/vendors/{vendor_id}`
- `create --as user` -> `POST /open-apis/contract/v1/mcp/vendors`
- `create --as app` -> `POST /open-apis/mdm/v1/vendors`
- `patch --as user` -> `PATCH /open-apis/contract/v1/mcp/vendors/{vendor_id}`
- `patch --as app` -> `PATCH /open-apis/mdm/v1/vendors/{vendor_id}`
- `enable|disable --as user` -> `PUT /open-apis/contract/v1/mcp/vendors/{vendor_id}/status`
- `update --as app` -> `PUT /open-apis/mdm/v1/vendors/{vendor_id}`
- `list-all|query-by-cert` 仍只支持 `app`。

app 身份执行 `create/update/patch` 必须传 `--user-id`。user 身份的操作人来自登录态，不允许传 `--user-id`。

## 创建规则

- create 请求体不要包含后端生成的 `vendor` 编码。
- 个人创建不允许传 `status`、风险字段或系统维护字段（如 ID、编码、审计字段），服务端生成编码并直接创建为启用，不发起审批。
- 个人创建不允许传 `vendorAccounts[].bankId`；填写账户时使用当前租户允许维护的账户字段。App 创建保持原接口契约。
- 手工编码租户不支持个人创建；遇到该业务错误时应说明限制，不改用 App 或旧接口绕过。
- 外部主数据租户是否允许写入由服务端权威配置判断，CLI 不猜测。
- 个人创建的 `ownerDepts` 如果来自部门查询返回的 `od-...`，必须加 `--department-id-type open_department_id`；传 MDM 内部数字部门 ID 时省略该参数或传 `department_id`。该参数仅用于个人创建/PATCH。

完整参数见 [references/vendor-create-parameters.md](references/vendor-create-parameters.md)。

## PATCH 规则

- 交易方 ID 只放在 path，body 不允许出现 `id`。
- 未传字段不修改，`null` 请求清空；有值则更新。最终结果仍须通过字段类型、必填和动态配置校验。
- 自定义日期区间清空必须提交对应 `fieldCode` 和 `rangeDate: null`；不要用空数组表示清空。
- 联系人、账户、地址、公司视图按 ID 修改；无 ID 表示新增；删除必须携带现有 ID 和 `_delete: true`；未列出的记录保留。
- `extendInfo` 按 `fieldCode` 合并。附件、人员、部门、多选字段一旦出现就整体替换；未出现则保持不变。
- App PATCH 覆盖旧 V1 PUT 实际允许修改的字段，包括 `status`。
- 个人 PATCH 不允许传 `vendor`、`status`、风险字段或系统维护字段（如 ID、审计字段）；允许修改已停用交易方的可编辑资料，修改后仍保持停用。
- 个人 PATCH 不允许传 `vendorAccounts[].bankId`；App PATCH 仍按原 V1 PUT 的字段能力处理。
- 个人 PATCH 不发起审批，但存在冲突的在途审批时由服务端拒绝。
- 个人 PATCH 的 `ownerDepts` 如果传 `od-...`，必须加 `--department-id-type open_department_id`；传 MDM 内部数字 ID 时省略该参数或传 `department_id`。

完整参数见 [references/vendor-patch-parameters.md](references/vendor-patch-parameters.md)。

## 启停规则

- `enable|disable` 仅支持 user 身份，不接收 JSON 请求体。
- 启停联动联系人、账户、地址和公司视图；不发起审批。
- 已是目标状态时返回 `NO_CHANGE`，不是失败，也不会重复写入。

完整参数见 [references/vendor-status-parameters.md](references/vendor-status-parameters.md)。

## 旧 PUT 兼容

- `update` 仍是原 App V1 PUT，行为和必填项不变。
- update 请求体必须包含后端返回的 `id` 和 `vendor` 编码。
- 不要把 PATCH 语义套到 `update`，也不要为了 PATCH 失败而回退调用 `update`。

完整参数见 [references/vendor-update-parameters.md](references/vendor-update-parameters.md)。

## 结果与重试

- 个人写接口以 `outcome` 判断结果：`APPLIED` 表示已生效，`NO_CHANGE` 表示状态或业务值没有变化。
- `UNKNOWN` 或 CLI 提示“执行结果不确定，请先查询确认”时，必须使用 `get` 查询实际结果，不得直接重试写请求。
- 明确业务失败应原样向用户说明，不切换身份、不回退旧接口、不补默认字段。

## 参考资料

- 查询决策：[references/vendor-query-guide.md](references/vendor-query-guide.md)
- 查询参数：[references/vendor-query-parameters.md](references/vendor-query-parameters.md)
- 命令示例：[references/commands.md](references/commands.md)
- 创建参数：[references/vendor-create-parameters.md](references/vendor-create-parameters.md)
- PATCH 参数：[references/vendor-patch-parameters.md](references/vendor-patch-parameters.md)
- 启停参数：[references/vendor-status-parameters.md](references/vendor-status-parameters.md)
- 旧 PUT 参数：[references/vendor-update-parameters.md](references/vendor-update-parameters.md)
- 全量分页：[references/vendor-list-all-parameters.md](references/vendor-list-all-parameters.md)
- 按证件查询：[references/vendor-query-by-cert-parameters.md](references/vendor-query-by-cert-parameters.md)

## 不要这样做

- 不要把 `list|get` 当字段配置查询。
- 不要在日志、输入文件示例或回复中暴露 token、密钥及无关个人资料。
- 不要用 `--as app` 绕过个人权限错误。
- 不要在写请求结果未知时直接重复创建、PATCH 或启停。
