---
name: contract-cli-contract
version: 1.0.0
description: "contract-cli 合同命令技能：支持 user/bot 双身份下的合同详情、合同搜索、合同创建、同步用户组、读取合同文本、查询合同分类、列出模板、查看模板详情、创建模板实例，bot 身份下的文件上传，以及 user 身份下的枚举查询。当用户要使用 `contract-cli contract ...` 操作合同能力时触发。"
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
- `contract-cli contract upload-file`
- `contract-cli contract enum list --type <enum_type>`

## 快速决策

- 想直接拿合同详情：用 `contract get`
- 想按条件查合同列表：用 `contract search`
- 想直接透传创建合同请求体：用 `contract create`
- 想拿正文文本：用 `contract text`
- 想看分类树：用 `contract category list`
- 想看模板或创建模板实例：用 `contract template ...`
- 想上传合同正文或附件文件：用 `contract upload-file --as bot`
- 想查创建合同相关枚举：用 `contract enum list`
- 若需求是审批、授权、付款：当前 skill 不覆盖，别伪造命令

## 关键规则

- `contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate` 同时支持 `--as user` 和 `--as bot`
- `contract upload-file` 当前仅支持 `--as bot`
- 除这九条双身份命令和 `contract upload-file` 外，其余命令仍然只支持 `--as user`
- `contract create` 当前直接接收原始创建请求体，不额外暴露 `--template`
- `contract create --as bot` 走 `POST /open-apis/contract/v1/contracts`
- `contract create --as bot` 的请求体必须自己带 `create_user_id`
- `contract create` 的推荐阅读顺序是：
  - 先读 [references/create-contract-fields.md](references/create-contract-fields.md) 选场景和最小请求体
  - 再读 [references/create-contract-field-tree.md](references/create-contract-field-tree.md) 查嵌套对象和 JSON Path
  - 最后读 [references/create-contract-enums.md](references/create-contract-enums.md) 确认 code 取值
- 这三份文档一起构成 `contract create` 的完整参数主档，不需要再回查旧接口清单
- `contract get --as bot` 走开放平台标准接口 `/open-apis/contract/v1/contracts/{contract_id}`
- `--user-id-type` / `--user-id` 是通用 query 参数：
  - 传了就原样拼到底层接口
  - 不传就不带
  - 不区分 `user` / `bot`
  - 不做默认值补齐
  - 不做命令级校验
- `contract search` 会把 `--contract-number`、`--page-size`、`--page-token` 合并进 `--input-file/--data` 里的 JSON 对象
- `contract search --as bot` 走开放平台标准接口 `/open-apis/contract/v1/contracts/search`
- `contract sync-user-groups --as bot` 走 `/open-apis/contract/v1/contracts/user-groups/sync`
- `contract text --as bot` 走 `POST /open-apis/contract/v1/contracts/{contract_id}/text`
- `contract category list --as bot` 走 `/open-apis/contract/v1/contract_categorys`
- `contract template list --as bot` 走 `/open-apis/contract/v1/templates`
- 按生产文档，`contract template list --as bot` 的 `category_number`、`user_id`、`user_id_type` 都属于 query 参数；CLI 仍只透传，不做本地必填校验
- `contract template get --as bot` 走 `/open-apis/contract/v1/templates/{template_id}`
- 按生产文档，`contract template get --as bot` 的 `user_id`、`user_id_type` 都属于 query 参数；CLI 仍只透传，不做本地必填校验
- `contract template instantiate --as bot` 走 `POST /open-apis/contract/v1/template_instances`
- 按生产文档，`contract template instantiate --as bot` 的 query 只有 `user_id_type`，请求体里需要 `create_user_id`；CLI 仍只透传，不做本地必填校验
- `contract upload-file --as bot` 走 `POST /open-apis/contract/v1/files/upload`
- `contract upload-file` 使用 `multipart/form-data`，字段是 `file_name`、`file_type`、`file`
- `contract upload-file` 的 `--file` 是本地真实文件路径，不是 JSON 请求体文件
- `contract upload-file` 不接受 `--input-file` / `--data`
- `contract upload-file` 本地限制文件大小小于等于 `200MB`
- 常用 `file_type`：`text` 合同文本、`attachment` 其他附件、`scan` 归档扫描件、`cause` 合同附件、`archiveAttachment` 归档附件、`customPictureAttachment` 图片附件、`customTableAttachment` 表格附件、`customFileAttachment` 文件附件
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

- 先确认 profile 已完成目标身份的登录：
  - user 详情、user 搜索、user 创建、user 同步用户组、user 合同文本、user 分类查询、user 模板列表、user 模板详情、user 模板实例和其他 user-only 命令：`auth login --as user`
  - bot 详情、bot 搜索、bot 创建、bot 同步用户组、bot 合同文本、bot 分类查询、bot 模板列表、bot 模板详情、bot 模板实例、bot 文件上传：`auth login --as bot`
- 复杂请求体优先用 `--input-file`
- 需要脚本消费时加 `--output json`
- 需要对照后端原始 envelope 时加 `--raw`
- 创建合同前，先根据是“文件正文模式”“模板实例模式”“合同变更”还是“合同终止”选主文档里的场景配方
- 复杂对象不要平铺查表，直接去字段树附录按 JSON Path 找
- 遇到 code 型字段，不要凭印象写值，直接看枚举附录
- 交易方/我方主体、金额、期限、合同分类这几个字段最容易缺，优先核对

## 不要这样做

- 不要对 `contract enum` 传 `--as bot`
- 不要继续写 `--file contract.json`；JSON 请求体用 `--input-file`
- 不要对 `contract upload-file` 传 `--as user`
- 不要把 `contract template fields` 当成已实现能力
