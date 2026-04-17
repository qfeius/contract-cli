---
name: contract-cli-mdm-vendor
version: 1.0.0
description: "contract-cli 交易方查询技能：列出交易方候选列表或按 ID 获取交易方详情。当用户要使用 `contract-cli mdm vendor list|get` 查询合同域交易方数据时触发。"
---

# contract-cli MDM Vendor

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

## 适用命令

- `contract-cli mdm vendor list`
- `contract-cli mdm vendor get <vendor-id>`

## 快速决策

- 已知交易方 ID：直接用 `mdm vendor get`
- 只知道名称或想拿候选列表：用 `mdm vendor list`
- 如果用户想在创建前确认字段定义：切到 [../contract-cli-mdm-fields/SKILL.md](../contract-cli-mdm-fields/SKILL.md)
- 如果用户想创建或更新交易方：当前结构化命令未实现，改走 [../contract-cli-api-call/SKILL.md](../contract-cli-api-call/SKILL.md) 或明确说明未覆盖

## 关键规则

- `mdm vendor list` 支持 `--name`、`--page-size`、`--page-token`
- `mdm vendor list --as user` 走 `/open-apis/contract/v1/mcp/vendors`
- `mdm vendor list --as bot` 走 `/open-apis/mdm/v1/vendors`
- `mdm vendor get --as user` 走 `/open-apis/contract/v1/mcp/vendors/{vendor_id}`
- `mdm vendor get --as bot` 走 `/open-apis/mdm/v1/vendors/{vendor_id}`
- 不暴露 `--operator`
- `--user-id-type` / `--user-id` 仍按共享规则透传，不做本地校验
- 推荐阅读顺序是：
  - 先读 [references/vendor-query-guide.md](references/vendor-query-guide.md) 选查询场景
  - 再读 [references/vendor-query-parameters.md](references/vendor-query-parameters.md) 查请求参数映射
  - 最后读 [references/commands.md](references/commands.md) 抄命令示例

## 实现来源

- [internal/cli/vendor_command.go](../../internal/cli/vendor_command.go)
- [internal/openplatform/mdmvendor/service.go](../../internal/openplatform/mdmvendor/service.go)
- [references/vendor-query-guide.md](references/vendor-query-guide.md)
- [references/vendor-query-parameters.md](references/vendor-query-parameters.md)
- [references/commands.md](references/commands.md)

## 操作建议

- 只知道名称时，先 `mdm vendor list` 拿候选，再 `mdm vendor get` 查详情
- 创建合同前如果只是要选对方主体，优先记住交易方 id
- 想确认写接口字段定义时，切到 `mdm fields list`

## 不要这样做

- 不要把 `mdm vendor list/get` 当成字段配置查询
- 不要把未实现的 `mdm vendor create/update` 当成已有结构化命令
