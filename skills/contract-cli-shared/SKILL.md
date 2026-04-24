---
name: contract-cli-shared
version: 1.0.0
description: "contract-cli 开放平台共享约定技能：在 `contract` 和 `mdm` 模块间做选择，并遵守 `contract/v1/mcp` user-only 限制、`--input-file` 请求体输入、输出格式和 profile 选择规则。当用户要操作开放平台 CLI 但尚未明确命令模块时触发。"
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
- 用户给了精确的开放平台路径，或结构化命令还没覆盖：不要推荐 `api call`；它是预留能力，当前暂未开放使用

## 当前已实现模块

- `contract get/search/create/sync-user-groups/text`
- `contract upload-file`
- `contract category list`
- `contract template list/get/instantiate`
- `contract enum list`
- `mdm vendor list/get`
- `mdm legal list/get`
- `mdm fields list`

## 共享约束

- `api call` 当前不对外开放；执行 `contract-cli api ...` 会直接返回 `api call 暂未开放使用，请使用已开放的结构化命令`
- `contract/v1/mcp` 这批路径大部分只支持 `--as user`
- 当前结构化命令里只有 `contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`contract upload-file`、`mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get`、`mdm fields list` 支持 bot；其中合同命令的 bot 路由走 `/open-apis/contract/v1/...`，`contract upload-file` 走 `/open-apis/contract/v1/files/upload` 且仅支持 bot，`mdm vendor list/get` 的 bot 路由走 `/open-apis/mdm/v1/vendors...`，`mdm legal list` 的 bot 路由走 `/open-apis/mdm/v1/legal_entities/list_all`，`mdm legal get` 的 bot 路由走 `/open-apis/mdm/v1/legal_entities/{legal_entity_id}`，`mdm fields list` 的 bot 路由走 `/open-apis/mdm/v1/config/config_list`
- 若命中 `/open-apis/contract/v1/mcp/` 且未传 `--as`，CLI 会默认按 `user` 解析，不看 `default_identity`
- 这批命令不暴露 `--operator`
- 请求体文件输入统一使用 `--input-file`
- `--user-id-type` / `--user-id` 是开放平台通用 query 参数：结构化命令支持；`--user-id-type` 不传时默认拼接 `user_id_type=user_id`，显式传值会覆盖默认值；`--user-id` 传了就透传，不传就不带；不做命令级校验
- `--file` 现在只用于真实二进制文件上传，例如 `contract upload-file`
- JSON 请求体文件输入始终使用 `--input-file`，不要把 `--file` 当 JSON 请求体参数
- 默认输出建议用 `json`；需要排障时可加 `--raw`

## 实现来源

- [internal/cli/command_support.go](../../internal/cli/command_support.go)
- [internal/openplatform/client.go](../../internal/openplatform/client.go)
- [internal/openplatform/mcp_specs.go](../../internal/openplatform/mcp_specs.go)

## 排障要点

- 命令报 `only supports --as user`：当前命中的是 user-only `contract/v1/mcp` 路径，切到 `--as user`
- 命令报 `profile "<name>" not found`：先执行 `contract-cli config add --env dev --name <profile>`
- 命令报 `user identity is not authorized`：先执行 `contract-cli auth login --profile <profile> --as user`
- 用户想做文件上传：当前只支持 `contract upload-file --as bot --file <path> --file-type <type>`
