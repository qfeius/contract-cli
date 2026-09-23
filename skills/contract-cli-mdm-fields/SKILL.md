---
name: contract-cli-mdm-fields
version: 1.0.2
description: "contract-cli 字段配置查询技能：查询 vendor、legal_entity、vendor_risk 的字段配置定义。当用户要使用 `contract-cli mdm fields list` 确认主数据字段结构时触发。app 身份当前只支持 vendor/legalEntity。"
---

# contract-cli MDM Fields

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

## 适用命令

- `contract-cli mdm fields list --biz-line <vendor|legal_entity|vendor_risk>`

## 快速决策

- 需要在写入交易方前确认字段定义：`--biz-line vendor`
- 需要在写入法人实体前确认字段定义：`--biz-line legal_entity`
- 需要确认交易方风险字段：`--biz-line vendor_risk`，仅适用于 user/MCP 路径
- 如果用户想查合同创建枚举，不要走这里，改用 `contract enum list`

## 关键规则

- `--biz-line` 必填
- `mdm fields list --as user` 走 `/open-apis/contract/v1/mcp/config/config_list`
- `mdm fields list --as app` 走 `/open-apis/mdm/v1/config/config_list`
- app 后端当前只接受 `vendor` / `legalEntity`；CLI 会把 app 下的 `legal_entity` 自动映射为 `legalEntity`
- `vendor_risk` 在 app 身份下不支持，会在本地报错
- `--user-id-type` / `--user-id` 仍按共享规则透传，不做本地校验
- 当前只封装字段配置查询，不负责本地校验和字段转换
- 查询 `vendor` 后必须按模块解释必填性：module 0 的必填字段是创建时的全局必填；module 1～4 的子项集合本身可选，内部必填字段只在提交该子项时生效；module 5 不属于交易方创建或 PATCH 请求
- 交易方自定义字段必须按 `fieldType` 选择唯一值属性：0/1/3/5 用 `fieldValue`，2 用 `num`，4/6 用 `options`，7 用 `date`，8 用 `rangeDate`，12 用 `appendix`，14 用 `employee`
- 推荐阅读顺序是：
  - 交易方写入先读 [references/vendor-field-config-semantics.md](references/vendor-field-config-semantics.md) 解释模块与必填性
  - 先读 [references/schema-fields-guide.md](references/schema-fields-guide.md) 选业务线和查询场景
  - 再读 [references/schema-biz-lines.md](references/schema-biz-lines.md) 查 `--biz-line` 精确值
  - 最后读 [references/commands.md](references/commands.md) 抄命令示例

## 实现来源

- [internal/cli/schema_command.go](../../internal/cli/schema_command.go)
- [internal/openplatform/schema/service.go](../../internal/openplatform/schema/service.go)
- [references/schema-fields-guide.md](references/schema-fields-guide.md)
- [references/schema-biz-lines.md](references/schema-biz-lines.md)
- [references/commands.md](references/commands.md)

## 操作建议

- 写入前，先用这条命令确认字段结构，再按 [交易方字段配置解释规则](references/vendor-field-config-semantics.md) 解释必填状态和自定义字段值属性；不得把子项内部必填误判为创建时必须提交整个子项
- 需要写交易方时，改读 [../contract-cli-mdm-vendor/SKILL.md](../contract-cli-mdm-vendor/SKILL.md)，按身份和意图选择 `mdm vendor create|patch|update`
- 需要写法人实体时，改读 [../contract-cli-mdm-legal/SKILL.md](../contract-cli-mdm-legal/SKILL.md)，使用 `mdm legal create|update --as app --user-id <operator-user-id>`
- 只想查合同枚举时，不要走这里

## 不要这样做

- 不要把 `mdm fields list` 当成数据查询命令
- 不要假设它会自动帮你校验写请求
- 不要因为 module 1～4 内部存在必填字段，就要求用户补齐未请求的地址、联系人、账户或公司视图
- 不要把 module 5 的签约信息写入交易方维护请求
- 不要根据示例值替用户生成国家、性质、地址、账户或公司代码等默认业务值
