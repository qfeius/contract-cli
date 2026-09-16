---
name: auth
version: 1.1.8
description: "contract-cli 登录与身份切换技能：初始化 profile、通过 Device Grant 展示手机号授权链接/二维码并单次查询结果、保留旧 user OAuth 授权码模式、登录 app 身份、查看状态与退出。当用户需要 `auth init/complete/login/status/logout/use` 时触发。"
---

# contract-cli Auth

本技能指导你如何在本仓库中使用 `contract-cli` 的登录与身份切换能力，并保持和当前实现一致。

## 环境与构建能力

- 正式构建仅支持 `prod`（默认 profile 为 `contract`）；test 联调构建支持 `prod/test`。先运行 `contract-cli version` 和 `contract-cli config add --help` 确认实际可执行程序的能力，不仅凭安装包名称判断。
- 用户明确选择 test 且 CLI 帮助支持 `--env test` 时，允许初始化和调用 `contract-test`。正式构建不支持 test 时提示安装联调包，不改域名绕过校验。所有构建均不使用 dev。
- 未指定环境沿用当前已确认配置；默认新配置为 prod。不自动切换环境，不用生产 Token 访问 test，也不因旧 profile 存在就复用它。
- Skill 更新后必须完全退出 WorkBuddy 并新建任务。已有任务不会热加载新 Skill，因此不能用旧任务验证升级后的规则。

## 适用范围

- 首次初始化本地 `profile`
- 在豆包或 WorkBuddy 中以 Device Grant 完成手机号授权
- 保留 `user` 身份的 OAuth 授权码 + PKCE 登录
- 以 `app` 身份录入 `app_id/app_secret`
- 查看或清理本地身份状态
- 切换默认业务身份
- 排查 `config.json` 和 `secrets.json` 的本地持久化问题

## 实现来源

- [internal/cli/app.go](../../internal/cli/app.go)
- [internal/cli/auth_provider.go](../../internal/cli/auth_provider.go)
- [internal/config/store.go](../../internal/config/store.go)
- [internal/config/secrets.go](../../internal/config/secrets.go)

## 对话凭证安全

- Agent 不得主动询问或接收原始凭证，包括 Token、Access Token、Refresh Token、AK/SK、Cookie、Session、App Secret、device code 和密码；不得让用户把这些值粘贴、上传或发送到对话中。
- user 身份只通过现有 Device Grant 或保留的 Authorization Code 官方页面完成授权；不得要求用户提供 Token 代替授权。
- 下方 `auth login --as app` 命令示例仅供本地操作者使用。Agent 只有在用户明确要求配置 app 身份时才能说明配置方式，不得要求用户把 App Secret 发到对话中，也不得把对话内容拼入 `--app-secret`。
- Agent 执行 app 登录时，只能使用用户已在本机安全配置好的环境变量或 CredentialStore；缺少安全配置时停止操作并提示用户在对话外完成配置，不得索要原始值。
- 用户在对话中主动发送敏感凭证时，不复述、不写入命令、不继续调用；提示凭证已经暴露，应立即撤销或轮换。

## 配置初始化

首次使用前，必须先执行：

```bash
contract-cli config add --env prod --name contract
```

正式构建仅内置 `prod`；test 联调构建额外支持 `test`。默认环境为 `prod`，默认 profile 名为 `contract`。上面的命令是生产示例；用户明确选择 test 时，改用以下配置，并在后续每条命令中显式传入相同的配置目录和 profile：

```bash
CONTRACT_CLI_CONFIG_DIR="$HOME/.contract-cli-test" contract-cli config add --env test --name contract-test
CONTRACT_CLI_CONFIG_DIR="$HOME/.contract-cli-test" contract-cli auth init --profile contract-test --output json
# 展示授权信息后结束本轮，用户回复已授权后再执行：
CONTRACT_CLI_CONFIG_DIR="$HOME/.contract-cli-test" contract-cli auth complete --profile contract-test --output json
```

WorkBuddy 执行时须保持同一 CODEBUDDY_SESSION_ID 和上述配置目录；Device 授权的展示、等待确认及错误处理规则不变。已有明确选定的 test Authorization Code 登录态可按对应模式使用。安装包不自动创建 profile，也不携带其他机器的登录态。以下生产示例不是要求 test 用户重新初始化 prod。

配置命令会：

- 发现 well-known 元数据
- 保存 MCP server / resource / OAuth server 配置
- 将 `default_identity` 初始化为 `user`

## 身份模型

同一个 profile 下维护两种身份：

| 身份 | 命令 | 本地存储 | 当前实现 |
|------|------|----------|----------|
| `user` | `contract-cli auth login --as user` | `profiles.<name>.identities.user.token` | 已实现 OAuth 登录 |
| `app` | `contract-cli auth login --as app` | `profiles.<name>.identities.app` + `secrets.json` | 已实现凭据录入和 `tenant_access_token` 兑换 |

`user` 身份有两种互不迁移的模式：

- `auth login --as user`：保留的 Authorization Code + PKCE 模式，token 继续使用旧 profile 存储。
- `auth init` + `auth complete`：豆包/WorkBuddy 使用的 Device Grant 模式，token 只进入 CredentialStore，不写入 profile。

Device Grant 一次授权同时包含合同与智审平台访问范围；本 Skill 只负责授权，不提供智审业务命令或智审业务操作说明。

额外还有一个默认身份指针：

- `profiles.<name>.default_identity`
- 由 `contract-cli auth use --as user|app` 修改
- `auth login --as ...` 成功后也会自动切换到对应身份

## 快速流程

### 豆包 / WorkBuddy Device 授权

```bash
contract-cli config add --env prod --name contract
contract-cli auth init --profile contract --output json
# 按当前 Agent 的展示契约呈现授权信息，等用户完成手机号、企业确认和同意授权
contract-cli auth complete --profile contract --output json
```

完成授权后的每次合同业务调用也必须显式传入同一个 profile，例如：

```bash
contract-cli contract get <contract-id> --profile contract --output json
```

固定规则：

- `auth init` 只发起一次请求并立即退出。WorkBuddy 与豆包 AgentKit 的最终回复必须同时包含完整 HTTPS 授权链接、二维码和过期时间；豆包普通工作任务只展示授权链接和过期时间。完成下述展示后必须立即结束当前轮次。
- 将 `verification_uri_complete` 替换到 Markdown `[打开授权页面](<verification_uri_complete>)` 中，确保最终回复正文有可点击链接；不得只把 URL 留在工具输出或思考过程中。
- WorkBuddy 的 `auth init` 返回 `pending` 后，只使用 `qr_code_path` 调用一次 `present_files(files: ["<qr_code_path>"])`，将 CLI 生成的原始 PNG 作为二维码图片附件/产物卡片交付。
- WorkBuddy 主路径只调用一次 `present_files`，不得调用其他图片处理或展示工具。禁止读取、复制或重新编码 `qr_code_data_uri`，也不得自行重新生成二维码。
- WorkBuddy 的最终授权提示使用下方固定模板，不展示 `user 身份未授权`、命令名或内部状态：

  ```markdown
继续查询前，需要先完成智书合同授权。
  1. 点击蓝色链接「[打开授权页面](<verification_uri_complete>)」，或扫描本消息中的二维码图片附件
  2. 在 **<expires_at_display>** 前完成：手机号验证 + 企业确认授权
  3. 授权全部完成后，回复消息：**已授权**，我将立刻为你执行合同查询
  ```

  必须直接使用 CLI 返回的 `expires_at_display`，并按模板加粗显示。不得向用户展示 `expires_at` 的 RFC3339 原值，不得出现 `T` 或 `+08:00`，不得使用反引号或代码样式展示时间。回复关键词“已授权”必须使用 Markdown `**已授权**` 加粗，逗号不放入加粗范围。二维码附件与正文中的链接和过期时间必须同时交付，不得只返回授权链接或只返回二维码。
- `present_files` 失败时，原样告知“二维码附件展示失败，请直接使用授权链接”，并仍在正文中展示可点击链接和过期时间。附件展示失败后禁止重试 `present_files`，禁止调用代码执行、图片处理或其他图片展示工具，且不得声称二维码已经展示。
- 豆包 AgentKit 继续按平台能力展示 `qr_code_path` 对应的 PNG，并在正文中同时提供可点击链接和过期时间。
- 豆包普通工作任务只展示可点击的完整 HTTPS 授权链接和 `expires_at_display`，不展示二维码。禁止读取、复制、修改或交付 `qr_code_path`，也不得处理 `qr_code_data_uri`；禁止调用代码执行、图片处理或图片交付工具处理二维码。
- 豆包普通工作任务使用下方固定模板，不展示 `user 身份未授权`、命令名、内部状态或二维码失败信息：

  ```markdown
继续查询前，需要先完成智书合同授权。
  1. 点击蓝色链接「[打开授权页面](<verification_uri_complete>)」
  2. 在 **<expires_at_display>** 前完成：手机号验证 + 企业确认授权
  3. 授权全部完成后，回复消息：**已授权**，我将立刻为你执行合同查询
  ```

  必须直接使用 CLI 返回的 `expires_at_display`，不得展示 `expires_at` 的 RFC3339 原值。返回授权链接和过期时间后立即结束当前轮次，不再调用任何授权展示、代码执行、图片处理或业务工具。
- WorkBuddy 和豆包 AgentKit 在 `auth init` 返回后只能调用一次授权展示工具；工具失败也不得改用其他展示工具或重试。豆包普通工作任务不得调用授权展示工具。除此之外，禁止执行 `auth complete`、再次执行 `auth init`、业务命令、轮询或网络重试。
- CLI 内部的单次安全重试不算第二次 `auth init` 命令；该重试只允许发生在明确的 TCP `dial` 失败、能够确认请求尚未发出时。
- `auth init` 最终失败后，禁止额外执行 `curl`、`auth status` 或其他探测命令；如实告知失败原因并明确询问用户是否重新发起授权。
- 链接已包含一次性用户码，不要再要求用户手工输入授权码。
- 只有收到新的用户消息明确说“已授权”后，才执行一次 `auth complete`；不得在展示链接的同一轮调用。返回 `pending` 时只告知尚未完成并结束当前轮次，禁止持续轮询。
- `succeeded` 后可继续执行授权前尚未发送的原业务请求一次，不再追加二次确认。
- `uncertain` / `busy` 必须告知当前结果不确定并结束当前轮次；禁止重试 `auth complete`，禁止重新执行 `auth init`。
- `denied` / `expired` / `restart_required` 为终态；先询问用户是否重新授权。只有收到新的用户消息明确同意后，才执行 `auth init --profile <profile> --output json --restart`，随后再次立即结束当前轮次。
- `auth status` 不支持 `--output`，禁止自动附加该参数。
- Refresh Token 返回 `invalid_grant` 时也必须先询问用户；CLI 会清理失效 Token，但会保留可能存在的 pending 会话。只有收到新的用户消息明确同意后，才先执行 `auth status --profile <profile> --as user`，再根据真实状态选择复用现有会话、普通 `auth init` 或带 `--restart` 的 `auth init`；不要直接重复未确认结果的写请求。
- 豆包 AgentKit / Skills Sandbox 运行在云端 Skill 环境，必须提供 `SKILL_SESSION_WORKSPACE` 和格式正确的 `CONTRACT_CLI_CREDENTIAL_KEY_V1`。
- 豆包普通工作任务使用 `SESSION_ID` 做任务级隔离；所有 CLI 命令必须从任务初始工作目录执行，不得在授权前后切换到其他目录。凭证以 AES-256-GCM 密文保存到当前任务目录，只在同一任务内复用，新建任务必须重新授权。
- `auth init` 成功后，CLI 会把 Device 运行所需的非敏感 profile 快照与 pending transaction 一起加密保存。AgentKit 同一会话工作区或豆包普通工作任务的任务目录仍存在、但临时 HOME 中没有本地 profile 时，CLI 只会在命令显式携带 `--profile contract` 且快照完整匹配时恢复 profile。
- 恢复只写入当前沙箱临时 HOME；不会把 `config.json`、`secrets.json`、App Secret 或明文 profile 写进会话工作区。快照缺失或损坏时，按错误提示重新执行 `config add` 和 `auth init`，禁止猜测环境、scope、client 或 endpoint。
- WorkBuddy 运行在客户本机，必须提供 `CODEBUDDY_SESSION_ID`；macOS 使用 macOS Keychain，Windows 使用 Credential Manager，Linux 使用 Secret Service。任一条件缺失都直接失败，不降级成明文文件。

### `user` 登录

```bash
contract-cli config add --env prod --name contract
contract-cli auth login --as user
contract-cli auth status --as user
```

行为约束：

- `user` 登录会自动注册 `client_id`
- 使用授权码模式 + PKCE
- 会启动本地回调服务，回调地址来自 profile 中的 `redirect_url`
- 默认自动打开浏览器；如需仅打印链接，使用 `--no-open-browser`
- 登录成功后写入 `identities.user.token`

### `app` 登录

```bash
contract-cli auth login --as app --app-id "<app_id>" --app-secret "<app_secret>"
contract-cli auth status --as app
```

也可以通过环境变量提供凭据：

```bash
export CONTRACT_CLI_APP_ID="<app_id>"
export CONTRACT_CLI_APP_SECRET="<app_secret>"
contract-cli auth login --as app
```

运行时也兼容旧变量 `CONTRACT_CLI_BOT_APP_ID` / `CONTRACT_CLI_BOT_APP_SECRET` 和 `DEMOCLI_BOT_APP_ID` / `DEMOCLI_BOT_APP_SECRET`，但后续新增配置统一使用 `CONTRACT_CLI_APP_ID` / `CONTRACT_CLI_APP_SECRET`。

为兼容老用户脚本，旧命令 `--as bot` 仍可执行，运行时等价于 `--as app`；新命令示例和解释统一使用 `app`。

凭据优先级固定为：

- 命令行参数
- 环境变量
- 本地已保存凭据

行为约束：

- `app` 登录会先保存 `app_id/app_secret`，再调用 `tenant_access_token/internal` 兑换 token
- `app_secret` 不写入 `config.json`
- token 成功后写入 `identities.app.token`
- token 兑换失败时，会保留新凭据，但不会切换默认身份到 `app`
- 登录成功后会切换 `default_identity=app`

## 状态、退出与切换

### 查看状态

```bash
contract-cli auth status --as user
contract-cli auth status --as app
```

规则：

- 不传 `--as` 时，`auth status` 默认查看 `user`
- `user` 显示 `authorized`、`expired` 或 `unauthorized`
- `app` 显示 `authorized`、`expired`、`configured` 或 `unconfigured`
- `app` 状态会显示 `Token Protocol: tenant_access_token/internal` 和过期时间（若有），不展示 endpoint 地址

### 退出登录

```bash
contract-cli auth logout --as user
contract-cli auth logout --as app
```

规则：

- Authorization Code 模式的 `logout --as user` 只清理 `user.token`
- Device 模式的 `logout --as user` 先撤销 Refresh Token family，成功后再清理 CredentialStore；撤销失败时保留本地凭据并明确报错
- `logout --as app` 只清理 `app.token`，保留 `app_id/app_secret` 和对应 secret
- 不传 `--as` 时，`auth logout` 默认处理 `user`

### 切换默认身份

```bash
contract-cli auth use --as user
contract-cli auth use --as app
```

规则：

- 该命令只修改 `default_identity`
- 不会重新登录
- 不会校验目标身份一定已拿到 token

## 本地文件

默认路径如下，若设置了 `CONTRACT_CLI_CONFIG_DIR`，则改为该目录：

- `~/.contract-cli/config.json`
- `~/.contract-cli/secrets.json`

运行时也兼容旧的 `DEMOCLI_CONFIG_DIR` 以及历史默认目录 `~/.democli`，用于平滑读取已有本地登录态。

存储约束：

- `config.json` 保存 profile、identity 元数据；旧授权码模式仍保持原有 token 存储行为
- Device Token 不写入 `config.json`：AgentKit 写入会话工作区的 AES-256-GCM 密文，豆包普通工作任务写入任务目录的 AES-256-GCM 密文，WorkBuddy 写入操作系统安全存储
- 豆包加密 Device 凭证可包含恢复当前 Device profile 所需的非敏感快照；不包含 App 身份、旧 OAuth Token、手机号、企业 ID 或业务参数
- 豆包普通工作任务没有平台 CredentialStore；任务级加密用于避免明文落盘和正常对话泄露，但不能抵御同一沙箱内具有文件和进程访问能力的 Shell，禁止宣称存在进程级 Secret 隔离
- `secrets.json` 只保存 app 的 `app_secret`
- `user.token` 与 `app.token` 分离存储，不共享
- 旧版平铺 OAuth 字段会自动迁移到 `identities.user`

## 安全规则

- 禁止在终端或文档中明文输出 `app_secret`、`access_token`、`refresh_token`
- 不要把 `app` 登出描述成“删除凭据”，当前实现只清 token、不删 `app_id/app_secret`
- 不要让用户误以为 `default_identity` 会影响 `auth status` 或 `auth logout` 的默认目标，这两个命令未传 `--as` 时仍按 `user`
- 涉及写入、清理本地凭据时，先确认是在当前 profile 上操作
- 禁止在 stdout、stderr、日志或 Skill 回复中输出 `device_code`、Access Token、Refresh Token 和加密密钥

## 故障排查

- `user identity is not configured`：按已确认环境初始化对应 profile；test 用上面的独立目录及 `--env test --name contract-test`，prod 用 `--env prod --name contract`。
- 浏览器未自动打开：改用 `--no-open-browser`，手动访问输出的授权链接
- 回调超时：检查 `redirect_url` 对应端口是否可监听，必要时调大 `--timeout`
- app 凭据不完整：补齐 `--app-id/--app-secret` 或设置 `CONTRACT_CLI_APP_ID/CONTRACT_CLI_APP_SECRET`
- app 登录提示缺少 `app_token_endpoint`：按当前已确认的环境和配置目录重跑 config add，不把 test profile 改成 prod。
- app 状态显示 `expired`：重新执行 `contract-cli auth login --as app`
- user 状态显示 `expired`：重新执行 `contract-cli auth login --as user`
- 旧脚本仍传 `--as bot`：可以继续执行；后续新脚本请改写为 `--as app`
