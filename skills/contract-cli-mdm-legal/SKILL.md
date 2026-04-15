---
name: contract-cli-mdm-legal
version: 1.0.0
description: "contract-cli 法人实体查询技能：列出法人实体候选列表或按 ID 获取法人实体详情。当用户要使用 `contract-cli mdm legal list|get` 查询合同域法人实体数据时触发。"
---

# contract-cli MDM Legal

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

## 适用命令

- `contract-cli mdm legal list`
- `contract-cli mdm legal get <legal-entity-id>`

## 快速决策

- 已知法人实体 ID：直接用 `mdm legal get`
- 只知道名称、需要候选列表：用 `mdm legal list`
- 用户是想查字段配置：切到 [../contract-cli-mdm-fields/SKILL.md](../contract-cli-mdm-fields/SKILL.md)
- 用户想创建或更新法人实体：当前结构化命令未实现，不要伪造

## 关键规则

- 只支持 `--as user`
- `mdm legal list` 支持 `--name`、`--page-size`、`--page-token`
- 不暴露 `--operator`
- 默认内部固定 `user_id_type=user_id`
- 推荐阅读顺序是：
  - 先读 [references/entity-query-guide.md](references/entity-query-guide.md) 选查询场景
  - 再读 [references/entity-query-parameters.md](references/entity-query-parameters.md) 查请求参数映射
  - 最后读 [references/commands.md](references/commands.md) 抄命令示例

## 实现来源

- [internal/cli/vendor_command.go](../../internal/cli/vendor_command.go)
- [internal/openplatform/entity/service.go](../../internal/openplatform/entity/service.go)
- [references/entity-query-guide.md](references/entity-query-guide.md)
- [references/entity-query-parameters.md](references/entity-query-parameters.md)
- [references/commands.md](references/commands.md)

## 操作建议

- 只知道主体名称时，先 `mdm legal list` 拿候选，再 `mdm legal get` 查详情
- 创建合同前如果只是要选我方主体，优先记住法人实体 id
- 想确认写接口字段定义时，切到 `mdm fields list`

## 不要这样做

- 不要对这批命令传 `--as bot`
- 不要把 `mdm legal list` 当成字段配置查询
- 不要把未实现的 `mdm legal create/update` 当成已有结构化命令
