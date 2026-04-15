---
name: contract-cli-mdm-fields
version: 1.0.0
description: "contract-cli 字段配置查询技能：查询 vendor、legal_entity、vendor_risk 的字段配置定义。当用户要使用 `contract-cli mdm fields list` 确认主数据字段结构时触发。"
---

# contract-cli MDM Fields

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

## 适用命令

- `contract-cli mdm fields list --biz-line <vendor|legal_entity|vendor_risk>`

## 快速决策

- 需要在写入交易方前确认字段定义：`--biz-line vendor`
- 需要在写入法人实体前确认字段定义：`--biz-line legal_entity`
- 需要确认交易方风险字段：`--biz-line vendor_risk`
- 如果用户想查合同创建枚举，不要走这里，改用 `contract enum list`

## 关键规则

- 只支持 `--as user`
- `--biz-line` 必填
- 当前只封装字段配置查询，不负责本地校验和字段转换
- 推荐阅读顺序是：
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

- 未封装写接口前，先用这条命令确认字段结构
- 需要写交易方或法人实体时，可和 `api call` 搭配使用
- 只想查合同枚举时，不要走这里

## 不要这样做

- 不要对这批命令传 `--as bot`
- 不要把 `mdm fields list` 当成数据查询命令
- 不要假设它会自动帮你校验写请求
