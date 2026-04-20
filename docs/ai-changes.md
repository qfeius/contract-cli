# AI 变更记录

- 2026-04-17
  变更摘要：修复 user-only MCP 请求中固定 query 被通用 `--user-id/--user-id-type` 覆盖的问题。
  涉及文件/模块：`internal/openplatform/client.go`、`internal/openplatform/client_test.go`、`internal/cli/mcp_command_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：`Client.Do()` 先解析生效的 `IdentityPolicy` 再合并 query；对 `IdentityPolicyUserOnly` 保留 `request.Query` 现有 key，仅补入 `CommonQuery` 中缺失的参数，从而保护 MCP 固定的 `user_id_type=user_id`；对 `IdentityPolicyAny` 继续保持通用 query 可覆盖同名请求参数的既有语义，并补充对应回归测试。

- 2026-04-17
  变更摘要：为 `contract-cli mdm fields list` 增加按身份自动分流的 bot 查询字段配置能力。
  涉及文件/模块：`internal/openplatform/schema/service.go`、`internal/openplatform/schema/service_test.go`、`internal/cli/schema_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-fields/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm fields list --biz-line <...>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/config/config_list`，`bot` 改走开放平台标准接口 `GET /open-apis/mdm/v1/config/config_list`；参考生产文档按显示文本采用 `config/config_list` 路径，同时记录超链接误指到 `vendors` 的瑕疵，并继续沿用 `biz_line` 的 query 透传映射，不做本地校验。

- 2026-04-17
  变更摘要：为 `contract-cli mdm legal get` 增加按身份自动分流的 bot 查询法人主体详情能力。
  涉及文件/模块：`internal/openplatform/entity/service.go`、`internal/openplatform/entity/service_test.go`、`internal/cli/vendor_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-legal/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm legal get <legal-entity-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}`，`bot` 改走开放平台标准接口 `GET /open-apis/mdm/v1/legal_entities/{legal_entity_id}`；由于生产文档同时把 `legal_entity_id` 写在查询参数表里，这次按确认方案采用“path + query 双带 `legal_entity_id`”的保守实现，并继续按共享约定透传 `--user-id-type` / `--user-id`，不做本地校验。

- 2026-04-17
  变更摘要：为 `contract-cli mdm legal list` 增加按身份自动分流的 bot 查询法人主体列表能力。
  涉及文件/模块：`internal/openplatform/entity/service.go`、`internal/openplatform/entity/service_test.go`、`internal/cli/vendor_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-legal/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm legal list` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/legal_entities`，`bot` 改走开放平台标准接口 `GET /open-apis/mdm/v1/legal_entities/list_all`；参考生产文档按显示文本采用 `legal_entities/list_all` 路径，同时记录文档超链接误指到 `vendors` 的瑕疵，并继续沿用当前 `legalEntity/page_size/page_token` 的 query 透传映射，不做本地校验。

- 2026-04-17
  变更摘要：为 `contract-cli mdm vendor get` 增加按身份自动分流的 bot 查询交易方详情能力。
  涉及文件/模块：`internal/openplatform/mdmvendor/service.go`、`internal/openplatform/mdmvendor/service_test.go`、`internal/cli/vendor_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-vendor/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm vendor get <vendor-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/vendors/{vendor_id}`，`bot` 改走开放平台标准接口 `GET /open-apis/mdm/v1/vendors/{vendor_id}`；参考生产文档仅把 `user_id_type` 视为 bot 文档显式列出的查询参数，但 CLI 继续按共享约定透传 `--user-id-type` / `--user-id`，不做本地校验。

- 2026-04-17
  变更摘要：为 `contract-cli mdm vendor list` 增加按身份自动分流的 bot 查询交易方列表能力。
  涉及文件/模块：`internal/openplatform/mdmvendor/service.go`、`internal/openplatform/mdmvendor/service_test.go`、`internal/cli/vendor_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-vendor/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm vendor list` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/vendors`，`bot` 改走开放平台标准接口 `GET /open-apis/mdm/v1/vendors`；依据生产文档保留 `vendor` 查询参数名，CLI 继续沿用 `--name -> vendor` 的透传映射，不在本地改名或做额外校验，`mdm vendor get` 仍保持 user-only。

- 2026-04-17
  变更摘要：为 `contract-cli contract template instantiate` 增加按身份自动分流的 bot 创建模板实例能力。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract template instantiate` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/template_instances`，`bot` 改走开放平台标准接口 `POST /open-apis/contract/v1/template_instances`；按照生产文档仅保留 `user_id_type` 作为 query 参数语义，并要求由调用方自行在 body 中提供 `create_user_id`，CLI 不做本地必填校验。

- 2026-04-17
  变更摘要：为 `contract-cli contract template get` 增加按身份自动分流的 bot 查看模板详情能力。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract template get <template-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/templates/{template_id}`，`bot` 改走开放平台标准接口 `GET /open-apis/contract/v1/templates/{template_id}`；查询参数继续沿用现有透传约定，不对生产文档中标注的 `user_id/user_id_type` 做本地必填校验，`template instantiate` 继续保持 user-only。

- 2026-04-17
  变更摘要：为 `contract-cli contract template list` 增加按身份自动分流的 bot 列出模板能力。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract template list` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/templates`，`bot` 改走开放平台标准接口 `GET /open-apis/contract/v1/templates`；查询参数仍沿用现有透传约定，不对生产文档中标注的 `category_number/user_id/user_id_type` 做本地必填校验，`template get/instantiate` 继续保持 user-only。

- 2026-04-17
  变更摘要：为 `contract-cli contract category list` 增加按身份自动分流的 bot 查询合同分类能力。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract category list` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/contract_categorys`，`bot` 改走开放平台标准接口 `GET /open-apis/contract/v1/contract_categorys`；`lang` 仍作为 query 参数透传，未放开其余模板/枚举等 user-only 合同命令。

- 2026-04-17
  变更摘要：为 `contract-cli contract create` 增加按身份自动分流的 bot 创建合同能力，并补齐 `create_user_id` 的命令/skill 说明。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract create` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/contracts`，`bot` 改走开放平台标准接口 `POST /open-apis/contract/v1/contracts`；CLI 仍旧透传原始 JSON body，不替用户补 `create_user_id`，只在命令文档和字段主档里明确它是 bot 创建时必须自行携带的请求体字段。

- 2026-04-17
  变更摘要：将 `--user-id-type` / `--user-id` 收敛为开放平台通用 query 参数，对结构化命令与 `api call` 统一透传，并同步纠正 `sync-user-groups` / `text` 的 bot 底层路径。
  涉及文件/模块：`internal/openplatform/client.go`、`internal/openplatform/client_test.go`、`internal/cli/command_support.go`、`internal/cli/api_command.go`、`internal/cli/api_command_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：通过 `RequestContext.CommonQuery` 在 `Client.Do()` 统一合并通用 query，显式传入的 `user_id_type/user_id` 会覆盖命令自身已有同名 query；结构化命令和 `api call` 只负责解析参数，不再对 `user` / `bot` 做必填、默认值或禁用校验；同时把 `contract sync-user-groups` 的 bot 路由纠正为 `POST /open-apis/contract/v1/contracts/user-groups/sync`，把 `contract text` 的 bot 路由纠正为 `POST /open-apis/contract/v1/contracts/{contract_id}/text`。

- 2026-04-17
  变更摘要：为 `contract-cli contract text` 增加按身份自动分流的 bot 获取合同文本能力。
  涉及文件/模块：`internal/openplatform/contract`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract text <contract-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 `/open-apis/contract/v1/mcp/contracts/{contract_id}/text?user_id_type=user_id&...`，`bot` 使用同一路径但不再追加 `user_id_type`，仅透传 `full_text/offset/limit` 查询参数并使用 `tenant_access_token` 调用；仅这条命令对 `/open-apis/contract/v1/mcp/contracts/{contract_id}/text` 单独放开 bot 访问，其余未改造的 `/contract/v1/mcp/` 结构化命令仍保持既有约束。

- 2026-04-17
  变更摘要：为 `contract-cli contract sync-user-groups` 增加按身份自动分流的 bot 同步用户组能力。
  涉及文件/模块：`internal/openplatform/contract`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract sync-user-groups` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 `/open-apis/contract/v1/mcp/contracts/user-groups/sync?user_id_type=user_id`，`bot` 仍使用同一路径但不再追加 user 侧查询参数，只使用 `tenant_access_token` 完成调用；仅这条命令对 `/open-apis/contract/v1/mcp/contracts/user-groups/sync` 单独放开 bot 访问，其余未改造的 `/contract/v1/mcp/` 结构化命令仍保持 user-only。

- 2026-04-17
  变更摘要：为 `contract-cli contract get` 增加按身份自动分流的 bot 合同详情能力，并把统一的 `--user-id-type` / `--user-id` 约定扩展到详情命令。
  涉及文件/模块：`internal/openplatform/contract`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract get <contract-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/contracts/{contract_id}`，`bot` 改走开放平台标准接口 `/open-apis/contract/v1/contracts/{contract_id}`，并同样按“第二种方式”追加 `user_id_type/user_id` 查询参数；仅 `contract get --as bot` 真正消费 `--user-id-type` / `--user-id`，其中 `--user-id` 必填、`--user-id-type` 默认 `user_id`，而 `--as user` 传入这两个参数会直接报错；其余结构化命令仍保持 user-only。

- 2026-04-16
  变更摘要：为 `contract-cli contract search` 增加按身份自动分流的 bot 搜索能力，并引入统一的 `--user-id-type` / `--user-id` 参数约定。
  涉及文件/模块：`internal/openplatform/contract`、`internal/cli/contract_command.go`、`internal/cli/command_support.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract search` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/contracts/search`，`bot` 改走开放平台标准接口 `/open-apis/contract/v1/contracts/search`，并按“第二种方式”追加 `user_id_type/user_id` 查询参数；仅 `contract search --as bot` 真正消费 `--user-id-type` / `--user-id`，其中 `--user-id` 必填、`--user-id-type` 默认 `user_id`，而 `--as user` 传入这两个参数会直接报错；响应继续原样透传，不做 user/bot 结果归一化。

- 2026-04-16
  变更摘要：新增一份与当前代码实现对齐的 CLI 命令总览文档，并补轻量契约测试防止文档漂移。
  涉及文件/模块：`docs/cli-command-reference.md`、`README.md`、`internal/cli/command_reference_doc_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：从 `internal/cli` 当前真实命令树反向整理 `config/auth/version/api/contract/mdm` 的命令矩阵、参数约定、输出约定和身份支持现状；明确当前结构化业务命令全部只支持 `--as user`，而 bot 业务接口后续优先通过 `api call --as bot` 验证；新增文档契约测试要求新文档必须覆盖所有已支持命令和 bot/user 边界，避免后续扩展时清单失真。

- 2026-04-15
  变更摘要：将内部主数据交易方 service 包从 `internal/openplatform/vendor` 重命名为 `internal/openplatform/mdmvendor`，规避 JetBrains 对 `vendor` 包路径的错误识别。
  涉及文件/模块：`internal/cli/vendor_command.go`、`internal/openplatform/mdmvendor/*`、`docs/ai-changes.md`
  关键逻辑/决策：Go 工具链可正常编译，但 IDE 对终止于 `/vendor` 的内部包索引不稳定，导致仅该导入路径持续爆红；外部 CLI 命令保持 `mdm vendor` 不变，只调整内部实现目录与测试导入路径，绕开 vendoring 语义歧义。

- 2026-04-15
  变更摘要：修正本地 `.idea` 模块内容根目录错误指向 `.idea/` 本身，导致 Go 源码在 IDE 中整体爆红的问题。
  涉及文件/模块：`.idea/contract-cli.iml`、`docs/ai-changes.md`
  关键逻辑/决策：排查发现本地 IntelliJ 对 `.iml` 里的 `$MODULE_DIR$` 宏实际按项目根目录解析，而不是按 `.idea/` 目录解析；因此上一版将 content root 调整为 `file://$MODULE_DIR$/..` 会把项目根错误扩大到 `/Users/lyy`。现已改回 `file://$MODULE_DIR$`，并显式排除 `.idea/bin/dist`，让模块根稳定回到 `/Users/lyy/contract-cli`。

- 2026-04-15
  变更摘要：补齐源码构建、版本注入、GoReleaser 和 npm/npx 薄包装发布脚手架。
  涉及文件/模块：`internal/build`、`internal/cli/app.go`、`internal/cli/app_test.go`、`build.sh`、`Makefile`、`package.json`、`scripts/*`、`.goreleaser.yml`、`.github/workflows/release.yml`、`README.md`、`CHANGELOG.md`、`LICENSE`、`tests/cli_e2e/*`、`.gitignore`、`docs/ai-changes.md`
  关键逻辑/决策：新增 `contract-cli version` / `--version` 并统一从 `internal/build` 读取 `Version/Commit/Date`；`build.sh`、`Makefile`、GoReleaser 和 npm 本地源码回退构建全部复用同一套 ldflags；npm 包采用 thin wrapper 设计，优先从可配置的 `downloadBaseURLTemplate` 下载预编译产物，若当前是源码仓库则回退到本地 `go build`；同时补齐 README/CHANGELOG/UNLICENSED 许可证与 e2e smoke 脚本，形成最小可用的发布骨架。

- 2026-04-15
  变更摘要：移除未参与运行时链路的 `ServerURL` 配置，以及 `config add` 的 `--server-url` 入口。
  涉及文件/模块：`internal/config/store.go`、`internal/config/store_test.go`、`internal/cli/app.go`、`internal/cli/app_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：确认当前 `user` 登录走 `resource + authorization/token/registration endpoint`，`bot` 登录走 `bot_token_endpoint`，开放平台业务请求走 `open_platform_base_url`，`ServerURL` 仅剩保存和展示作用；因此删除 profile 中的 `server_url` 字段、`config add --server-url` flag，以及相关 stdout/status 输出；新增回归测试确保配置文件不再持久化该字段且旧 flag 被拒绝。

- 2026-04-15
  变更摘要：纠正 `config add --env dev` 的默认鉴权预设，把 user OAuth 与 bot 直调 token 链路彻底分开。
  涉及文件/模块：`internal/cli/app.go`、`internal/cli/app_test.go`、`internal/oauth/discovery.go`、`internal/oauth/discovery_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：确认 `--as bot` 只使用 `https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal`；`--as user` 不再错误地复用 `dev-open` 的 `.well-known/oauth-protected-resource`，而是默认通过公开的 `https://dev-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract` 加载 OAuth server metadata，并配合固定 `resource=http://higress-gateway.higress-system/mcp-servers` 生成 user 登录配置；新增回归测试覆盖该默认链路。

- 2026-04-15
  变更摘要：修复 `config add --env dev` 默认预设使用集群内 Higress 地址导致本机发现链路易出现 502 的问题。
  涉及文件/模块：`internal/cli/app.go`、`internal/cli/app_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：新增回归测试，要求默认 dev 预设走公共 `https://dev-open.qtech.cn` 的 `mcp-servers/contract-group` 和 `/.well-known/oauth-protected-resource`；将 `resolveEnvironment("dev")` 中原本的 `http://higress-gateway.higress-system/...` 集群内地址替换为公共 HTTPS 地址，避免本机 `config add` 默认链路命中 502。

- 2026-04-15
  变更摘要：将主数据命令树进一步统一为 `contract-cli mdm <vendor|legal|fields> ...`，其中字段配置收口为 `mdm fields list`。
  涉及文件/模块：`internal/cli`、`internal/cli/*_test.go`、`docs/cli-command-design*.md`、`skills/contract-cli-shared`、`skills/contract-cli-mdm-vendor`、`skills/contract-cli-mdm-legal`、`skills/contract-cli-mdm-fields`、`docs/ai-changes.md`
  关键逻辑/决策：新增 `mdm` 一级命令并作为主数据唯一入口，`vendor`、`legal`、`fields` 下降为二级资源；`fields` 再显式使用 `list` 子命令，统一成“一级领域 + 二级资源 + 三级动作”的用户心智；移除 `mdm-vendor`、`mdm-legal`、`mdm-fields` 旧入口；同步更新结构化命令测试、skill 示例、参数附录与设计文档中的命令写法。

- 2026-04-15
  变更摘要：将主数据命令统一重命名为 `mdm-vendor`、`mdm-legal`、`mdm-fields`，并同步把对应 skill 目录改成新命名。
  涉及文件/模块：`internal/cli`、`internal/openplatform`、`docs/cli-command-design*.md`、`skills/contract-cli-shared`、`skills/contract-cli-mdm-vendor`、`skills/contract-cli-mdm-legal`、`skills/contract-cli-mdm-fields`、`docs/ai-changes.md`
  关键逻辑/决策：移除旧的 `vendor`、`entity`、`schema fields` 顶层命令名，不保留别名；CLI 用法、结构化命令测试、skill 元数据和设计文档统一改为 `mdm-vendor`、`mdm-legal`、`mdm-fields`；同时将 skill 目录从 `contract-cli-vendor|entity|schema` 重命名为 `contract-cli-mdm-vendor|mdm-legal|mdm-fields`，并修正相对链接；主文档和会议版中仍作为请求体输入的 `--file` 示例同步改为 `--input-file`。

- 2026-04-14
  变更摘要：将 `vendor`、`entity`、`schema`、`api call` 四组 skill 统一重构为“主 guide + 规则/参数附录 + 命令示例”的阅读结构。
  涉及文件/模块：`skills/contract-cli-mdm-vendor`、`skills/contract-cli-mdm-legal`、`skills/contract-cli-mdm-fields`、`skills/contract-cli-api-call`、`skills/contract-cli-shared`、`docs/ai-changes.md`
  关键逻辑/决策：参考合同模块的阅读路径，把其他接口 skill 也拆成“先选场景，再查参数/规则，最后抄示例”的结构；新增 vendor/entity 参数映射附录、schema biz-line 附录和 api call 规则附录，并在 shared skill 中统一说明各模块的新阅读方式。

- 2026-04-14
  变更摘要：将 `contract create` skill 字段文档重构为“主文档 + 字段树附录 + 枚举附录”三段式结构。
  涉及文件/模块：`skills/contract-cli-contract/SKILL.md`、`skills/contract-cli-contract/references/create-contract-fields.md`、`skills/contract-cli-contract/references/create-contract-field-tree.md`、`skills/contract-cli-contract/references/create-contract-enums.md`、`docs/ai-changes.md`
  关键逻辑/决策：主文档改为场景配方与阅读导航，不再用单一平铺大表；新增 JSON Path 字段树附录承接全部顶层与嵌套字段；新增枚举附录承接 code 型字段和值域说明；合同总 skill 明确阅读顺序为“场景 -> 字段树 -> 枚举值”，整体不再依赖 `mcp.yaml` 作为说明来源。

- 2026-04-14
  变更摘要：把 `contract create` 字段参考补成独立主档，明确列出全部顶层字段和嵌套字段，不再依赖 `mcp.yaml` 兜底说明。
  涉及文件/模块：`skills/contract-cli-contract/SKILL.md`、`skills/contract-cli-contract/references/create-contract-fields.md`、`docs/ai-changes.md`
  关键逻辑/决策：重写 `create-contract-fields.md`，补齐 `create-contracts` 的全部字段、条件必填、文件 id 约束、变更/终止规则、嵌套对象说明和示例；合同总 skill 明确该 reference 已是 `contract create` 的完整参数来源。

- 2026-04-14
  变更摘要：补充 `contract-cli contract create` 的 skill 字段参考，并在合同总 skill 中增加明确入口。
  涉及文件/模块：`skills/contract-cli-contract/SKILL.md`、`skills/contract-cli-contract/references/create-contract-fields.md`、`docs/ai-changes.md`
  关键逻辑/决策：为 `contract create` 新增专门的字段参考，说明 CLI 参数、文件正文/模板实例两种常见路径、顶层核心字段、条件必填和最小 JSON 示例；总 skill 明确引导在需要具体字段说明时优先读取该 reference，并强调当前实现只是透传请求体、不做本地字段校验。

- 2026-04-14
  变更摘要：新增按命令模块拆分的 contract-cli skill 文档，并修正现有 auth skill 以匹配当前 bot token 实现。
  涉及文件/模块：`skills/auth`、`skills/contract-cli-shared`、`skills/contract-cli-contract`、`skills/contract-cli-mdm-vendor`、`skills/contract-cli-mdm-legal`、`skills/contract-cli-mdm-fields`、`skills/contract-cli-api-call`、`docs/ai-changes.md`
  关键逻辑/决策：参考 `lark-sheets` 风格把 skill 拆成共享约定 + 业务模块；每个模块补 `SKILL.md`、`agents/openai.yaml` 和按需 `references/commands.md`；共享强调 `contract/v1/mcp` 只支持 `--as user`、请求体统一走 `--input-file`；同步修正 `auth` skill 中 bot 已支持 `tenant_access_token` 兑换、状态枚举和 logout 仅清 token 的真实语义。

- 2026-04-14
  变更摘要：实现 `mcp.yaml` 驱动的 user-only 结构化 CLI 命令，并将请求体文件参数统一改为 `--input-file`。
  涉及文件/模块：`internal/openplatform`、`internal/openplatform/contract|vendor|entity|schema`、`internal/cli`、`docs/cli-command-design.md`、`docs/ai-changes.md`
  关键逻辑/决策：新增 `contract/v1/mcp` 静态工具映射与契约测试；`openplatform` 增加 `IdentityPolicy` 并对 `/open-apis/contract/v1/mcp/` 执行 `--as user` 硬拦截；新增 `contract/vendor/entity/schema` 结构化命令并统一复用 service 层；`api call` 对该前缀默认走 user 身份；所有请求体文件输入从 `--file` 迁移到 `--input-file`，`--file` 保留给后续真实文件上传。

- 2026-04-14
  变更摘要：新增开放平台统一 client、输出渲染层和 `api call` 命令，为后续业务域命令封装打底。
  涉及文件/模块：`internal/openplatform`、`internal/output`、`internal/cli`、`internal/config`、`docs/ai-changes.md`
  关键逻辑/决策：profile 新增 `open_platform_base_url` 并由 `config add --env dev` 写入 `https://dev-open.qtech.cn`；新增统一的相对路径 `/open-apis/...` 校验、token 解析与 HTTP 请求包装；`contract-cli api call` 支持 `--profile/--as/--file/--data/--output/--raw/--header`；新增 `vendor` 域 service 样板和 CLI 禁止直接发 HTTP 的架构约束测试。

- 2026-04-14
  变更摘要：实现 bot 身份 `tenant_access_token` 登录、状态展示与保留凭证登出语义。
  涉及文件/模块：`internal/cli`、`internal/oauth`、`internal/config`、`docs/ai-changes.md`
  关键逻辑/决策：`config add` 为 `dev` profile 写入 `bot_token_endpoint`；`auth login --as bot` 立即调用 `https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal` 换 token 并保存过期时间；token 兑换失败时保留新 `appId/appSecret` 但不切默认身份；`auth status --as bot` 区分 `authorized/expired/configured/unconfigured`；`auth logout --as bot` 仅清 token、保留 bot 凭证。

- 2026-04-14
  变更摘要：清理旧的根目录二进制产物，并新增 Git 忽略规则避免本地产物再次出现在仓库根目录变更中。
  涉及文件/模块：`.gitignore`、`docs/ai-changes.md`
  关键逻辑/决策：删除历史 `democli` 二进制；将根目录 `contract-cli` 与 `democli` 纳入忽略规则，保留本地可执行文件使用能力，同时避免构建产物污染版本管理视图。

- 2026-04-14
  变更摘要：将 Go module path 调整为公司规范 `cn.qfei/contract-cli`。
  涉及文件/模块：`go.mod`、`cmd/contract-cli`、`internal/cli`、`internal/config`、`internal/oauth`、`docs/ai-changes.md`
  关键逻辑/决策：统一替换仓库内 Go import 前缀，避免继续使用临时的 `github.com/lyy/contract-cli`；本次仅调整模块标识与编译路径，不改变 CLI 运行时行为。

- 2026-04-14
  变更摘要：将 CLI 对外名称从 `democli` 统一重命名为 `contract-cli`，并保留旧配置兼容读取。
  涉及文件/模块：`go.mod`、`cmd/contract-cli`、`internal/cli`、`internal/config`、`internal/oauth`、`skills/auth`、`docs/*.md`
  关键逻辑/决策：帮助文案、module path、默认 `client_name`、技能与设计文档统一切换到 `contract-cli`；运行时优先使用 `CONTRACT_CLI_*` 与 `~/.contract-cli`，同时兼容旧的 `DEMOCLI_*` 与 `~/.democli`，避免现有本地登录态失效。

- 2026-04-14
  变更摘要：新增本地 `skills/auth`，将 `democli` 的登录与身份切换逻辑整理成可复用 skill。
  涉及文件/模块：`skills/auth/SKILL.md`、`skills/auth/agents/openai.yaml`、`docs/ai-changes.md`
  关键逻辑/决策：按 `lark-shared` 风格沉淀 `config add`、`auth login --as user|bot`、`auth status/logout/use` 的使用规则；明确 `bot` 仅保存 `app_id/app_secret`，真实 token 兑换尚未实现；补充 `config.json`/`secrets.json` 的存储约束与排障说明。

- 2026-04-14
  变更摘要：实现 `user` / `bot` 双身份鉴权与默认身份切换，新增 bot 凭据独立存储。
  涉及文件/模块：`internal/cli`、`internal/config`、`docs/ai-changes.md`
  关键逻辑/决策：`auth login/status/logout` 新增 `--as user|bot`；新增 `auth use` 切换默认业务身份；profile 下分离保存 `user.token` 和 `bot.token/credentials`；`appsecret` 独立落 `secrets.json`，旧平铺 OAuth 配置自动迁移到 `identities.user`。

- 2026-04-13
  变更摘要：新增一份供会议使用的 CLI 命令设计汇总文档，以主文档为基线并引用另外两份辅助方案。
  涉及文件/模块：`docs/cli-command-design-meeting.md`、`docs/ai-changes.md`
  关键逻辑/决策：不改动既有三份设计文档；新增会议版文档用于统一阅读顺序、角色分工、决策顺序和会议结论模板。

- 2026-04-13
  变更摘要：撤回上一轮 CLI 文档整合，恢复为三份独立的设计文档。
  涉及文件/模块：`docs/cli-command-design.md`、`docs/contract-create-command-options.md`、`docs/cli-command-design-mvp.md`、`docs/ai-changes.md`
  关键逻辑/决策：取消“单一主文档 + 迁移说明”的整理方式；恢复完整方案、合同创建对比方案和 MVP 方案分别独立维护。

- 2026-04-13
  变更摘要：将分散的 CLI 设计文档整合为单一主文档，并将其他文档改为迁移说明。
  涉及文件/模块：`docs/cli-command-design.md`、`docs/contract-create-command-options.md`、`docs/cli-command-design-mvp.md`、`docs/ai-changes.md`
  关键逻辑/决策：统一以 `docs/cli-command-design.md` 作为唯一设计来源；保留极简 MVP、合同创建备选方案和演进路径；避免后续文档分叉。

- 2026-04-13
  变更摘要：新增极简 MVP 命令设计文档，收敛首发命令为统一 `--input` 形式。
  涉及文件/模块：`docs/cli-command-design-mvp.md`、`docs/ai-changes.md`
  关键逻辑/决策：首发优先统一输入模型而非细分命令树；合同与交易方创建统一走 `--input`；复杂模式差异先沉到 `input.mode` 和内部 handler。

- 2026-04-13
  变更摘要：新增合同创建命令方案对比文档，整理“单命令 + mode”与“拆命令”两种设计及折中建议。
  涉及文件/模块：`docs/contract-create-command-options.md`、`docs/ai-changes.md`
  关键逻辑/决策：统一以 `ContractCreateSpec` 作为执行格式；明确 `spec` 可由智能体或用户提供；不建议由智能体隐式猜测模式。

- 2026-04-13
  变更摘要：新增 `democli` CLI 命令设计文档，统一合同、交易方、事件等领域命令风格。
  涉及文件/模块：`docs/cli-command-design.md`、`docs/ai-changes.md`
  关键逻辑/决策：复用现有 `config/auth` 授权体系；业务命令采用“资源 + 动作”；复杂请求统一走 `--file`/`--data`；`vendor`/`entity` 使用 `--operator` 映射底层 `user_id`。

- 2026-04-08
  变更摘要：初始化 `democli` Go 项目并接入最小 OAuth 授权 CLI 骨架。
  涉及文件/模块：`cmd/democli`、`internal/cli`、`internal/config`、`internal/oauth`、`go.mod`
  关键逻辑/决策：按 `dev` 预设实现 Higress-MCP 授权流程；只使用 Go 标准库；通过 `config add` 做 well-known 发现，`auth login/status/logout` 处理客户端注册、PKCE 授权码换 token 和本地落盘。
