# AI 变更记录

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
