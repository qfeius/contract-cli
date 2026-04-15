---
name: contract-cli-contract
version: 1.0.0
description: "contract-cli 合同 user-only 命令技能：搜索合同、查看详情、创建合同、同步用户组、读取合同文本，以及查询合同分类、模板和枚举。当用户要使用 `contract-cli contract ...` 操作 `contract/v1/mcp` 合同能力时触发。"
---

# contract-cli Contract

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

## 适用命令

- `contract-cli contract get <contract-id>`
- `contract-cli contract search`
- `contract-cli contract create`
- `contract-cli contract sync-user-groups`
- `contract-cli contract text <contract-id>`
- `contract-cli contract category list`
- `contract-cli contract template list`
- `contract-cli contract template get <template-id>`
- `contract-cli contract template instantiate`
- `contract-cli contract enum list --type <enum_type>`

## 快速决策

- 想直接拿合同详情：用 `contract get`
- 想按条件查合同列表：用 `contract search`
- 想直接透传创建合同请求体：用 `contract create`
- 想拿正文文本：用 `contract text`
- 想看分类树：用 `contract category list`
- 想看模板或创建模板实例：用 `contract template ...`
- 想查创建合同相关枚举：用 `contract enum list`
- 若需求是文件上传、审批、授权、付款：当前 skill 不覆盖，别伪造命令

## 关键规则

- 所有命令都只支持 `--as user`
- `contract create` 当前直接接收原始创建请求体，不额外暴露 `--template`
- `contract create` 的推荐阅读顺序是：
  - 先读 [references/create-contract-fields.md](references/create-contract-fields.md) 选场景和最小请求体
  - 再读 [references/create-contract-field-tree.md](references/create-contract-field-tree.md) 查嵌套对象和 JSON Path
  - 最后读 [references/create-contract-enums.md](references/create-contract-enums.md) 确认 code 取值
- 这三份文档一起构成 `contract create` 的完整参数主档，不需要再回查旧接口清单
- `contract search` 会把 `--contract-number`、`--page-size`、`--page-token` 合并进 `--input-file/--data` 里的 JSON 对象
- `contract text` 支持 `--full-text`、`--offset`、`--limit`
- `contract template instantiate` 只接收请求体，不再接模板 ID 位置参数

## 实现来源

- [internal/cli/contract_command.go](../../internal/cli/contract_command.go)
- [internal/openplatform/contract/service.go](../../internal/openplatform/contract/service.go)
- [references/commands.md](references/commands.md)
- [references/create-contract-fields.md](references/create-contract-fields.md)
- [references/create-contract-field-tree.md](references/create-contract-field-tree.md)
- [references/create-contract-enums.md](references/create-contract-enums.md)

## 操作建议

- 先确认 profile 已完成 `auth login --as user`
- 复杂请求体优先用 `--input-file`
- 需要脚本消费时加 `--output json`
- 需要对照后端原始 envelope 时加 `--raw`
- 创建合同前，先根据是“文件正文模式”“模板实例模式”“合同变更”还是“合同终止”选主文档里的场景配方
- 复杂对象不要平铺查表，直接去字段树附录按 JSON Path 找
- 遇到 code 型字段，不要凭印象写值，直接看枚举附录
- 交易方/我方主体、金额、期限、合同分类这几个字段最容易缺，优先核对

## 不要这样做

- 不要对这批命令传 `--as bot`
- 不要继续写 `--file contract.json`
- 不要把 `contract template fields`、`contract file upload` 当成已实现能力
