---
name: contract-cli-shared
version: 1.0.0
description: "contract-cli 开放平台共享约定技能：在 `contract`、`mdm` 和 `api call` 模块间做选择，并遵守 `contract/v1/mcp` user-only 限制、`--input-file` 请求体输入、输出格式和 profile 选择规则。当用户要操作开放平台 CLI 但尚未明确命令模块，或需要判断该走结构化命令还是 `api call` 时触发。"
---

# contract-cli Shared

CRITICAL — 开始前 MUST 先读取 [../auth/SKILL.md](../auth/SKILL.md)，确认当前 profile、user 登录态和 bot/token 约束。

## 快速决策

- 合同搜索、详情、创建、合同文本、模板、分类、枚举：读 [../contract-cli-contract/SKILL.md](../contract-cli-contract/SKILL.md)
  这里现在采用“主文档 + 字段树附录 + 枚举附录”的结构
- 交易方查询：读 [../contract-cli-mdm-vendor/SKILL.md](../contract-cli-mdm-vendor/SKILL.md)
  这里现在采用“主 guide + 参数附录 + 命令示例”的结构
- 法人实体查询：读 [../contract-cli-mdm-legal/SKILL.md](../contract-cli-mdm-legal/SKILL.md)
  这里现在采用“主 guide + 参数附录 + 命令示例”的结构
- 字段配置查询：读 [../contract-cli-mdm-fields/SKILL.md](../contract-cli-mdm-fields/SKILL.md)
  这里现在采用“主 guide + biz-line 附录 + 命令示例”的结构
- 用户给了精确的开放平台路径，或结构化命令还没覆盖：读 [../contract-cli-api-call/SKILL.md](../contract-cli-api-call/SKILL.md)
  这里现在采用“主 guide + 调用规则附录 + 命令示例”的结构

## 当前已实现模块

- `contract get/search/create/sync-user-groups/text`
- `contract upload-file`
- `contract category list`
- `contract template list/get/instantiate`
- `contract enum list`
- `mdm vendor list/get`
- `mdm legal list/get`
- `mdm fields list`
- `api call`

## 共享约束

- `contract/v1/mcp` 这批路径大部分只支持 `--as user`
- 当前结构化命令里只有 `contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`contract upload-file`、`mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get`、`mdm fields list` 支持 bot；其中合同命令的 bot 路由走 `/open-apis/contract/v1/...`，`contract upload-file` 走 `/open-apis/contract/v1/files/upload` 且仅支持 bot，`mdm vendor list/get` 的 bot 路由走 `/open-apis/mdm/v1/vendors...`，`mdm legal list` 的 bot 路由走 `/open-apis/mdm/v1/legal_entities/list_all`，`mdm legal get` 的 bot 路由走 `/open-apis/mdm/v1/legal_entities/{legal_entity_id}`，`mdm fields list` 的 bot 路由走 `/open-apis/mdm/v1/config/config_list`
- 若命中 `/open-apis/contract/v1/mcp/` 且未传 `--as`，CLI 会默认按 `user` 解析，不看 `default_identity`
- 这批命令不暴露 `--operator`
- 请求体文件输入统一使用 `--input-file`
- `--user-id-type` / `--user-id` 是开放平台通用 query 参数：结构化命令和 `api call` 都支持，传了就透传，不传就不带，不做默认值和命令级校验
- `--file` 现在只用于真实二进制文件上传，例如 `contract upload-file`
- JSON 请求体文件输入始终使用 `--input-file`，不要把 `--file` 当 JSON 请求体参数
- 默认输出建议用 `json`；需要排障时可加 `--raw`

## 实现来源

- [internal/cli/command_support.go](../../internal/cli/command_support.go)
- [internal/cli/api_command.go](../../internal/cli/api_command.go)
- [internal/openplatform/client.go](../../internal/openplatform/client.go)
- [internal/openplatform/mcp_specs.go](../../internal/openplatform/mcp_specs.go)

## 排障要点

- 命令报 `only supports --as user`：当前命中的是 user-only `contract/v1/mcp` 路径，切到 `--as user`
- 命令报 `profile "<name>" not found`：先执行 `contract-cli config add --env dev --name <profile>`
- 命令报 `user identity is not authorized`：先执行 `contract-cli auth login --profile <profile> --as user`
- 用户想做文件上传：当前只支持 `contract upload-file --as bot --file <path> --file-type <type>`
