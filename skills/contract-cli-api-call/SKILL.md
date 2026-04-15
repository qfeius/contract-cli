---
name: contract-cli-api-call
version: 1.0.0
description: "contract-cli 原始接口兜底技能：按方法和相对路径调用开放平台接口，适合结构化命令未覆盖的长尾接口或需要精确透传请求的场景。当用户要使用 `contract-cli api call` 或给出明确 `/open-apis/...` 路径时触发。"
---

# contract-cli API Call

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

## 适用命令

- `contract-cli api call <METHOD> <PATH>`

## 快速决策

- 结构化命令已覆盖：优先用对应 skill，不要退回 `api call`
- 用户给了精确开放平台路径或结构化命令未实现：用 `api call`
- 用户要调 `/open-apis/contract/v1/mcp/...`：必须走 `--as user`

## 关键规则

- `PATH` 必须是相对路径，且以 `/open-apis/` 开头
- 请求体文件输入使用 `--input-file`
- `--input-file` 与 `--data` 互斥
- 可重复使用 `--header "Key: Value"`
- 命中 `/open-apis/contract/v1/mcp/` 时：
  - `--as bot` 会被直接拒绝
  - 不传 `--as` 时默认按 `user` 身份解析
- 推荐阅读顺序是：
  - 先读 [references/api-call-guide.md](references/api-call-guide.md) 判断是否该退回 `api call`
  - 再读 [references/api-call-rules.md](references/api-call-rules.md) 查路径、身份、请求体规则
  - 最后读 [references/commands.md](references/commands.md) 抄命令示例

## 实现来源

- [internal/cli/api_command.go](../../internal/cli/api_command.go)
- [internal/cli/command_support.go](../../internal/cli/command_support.go)
- [internal/openplatform/client.go](../../internal/openplatform/client.go)
- [references/api-call-guide.md](references/api-call-guide.md)
- [references/api-call-rules.md](references/api-call-rules.md)
- [references/commands.md](references/commands.md)

## 操作建议

- 已有结构化命令时，优先用结构化命令
- 原始调试请求优先用 `--input-file`
- 命中 `contract/v1/mcp` 时，默认按 `user` 思维去看问题

## 不要这样做

- 不要传绝对 URL
- 不要把 `--file` 当成请求体输入
- 不要把真实文件上传需求伪装成普通 JSON body 调用
