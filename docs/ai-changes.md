# AI 变更记录

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
