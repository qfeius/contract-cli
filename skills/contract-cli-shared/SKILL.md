---
name: contract-cli-shared
version: 1.0.1
description: "contract-cli 开放平台共享约定技能：在 `contract`、`payment`、`mdm`、`event` 和 `rule` 模块间做选择，并遵守 `contract/v1/mcp` user-only 限制、`--input-file` 请求体输入、输出格式和 profile 选择规则。当用户要操作开放平台 CLI 但尚未明确命令模块、需要更新 contract-cli，或看到 JSON 输出中的 `_notice` / `_notice.update` 时触发。"
---

# contract-cli Shared

CRITICAL — 开始前 MUST 先读取 [../auth/SKILL.md](../auth/SKILL.md)，确认当前 profile、user 登录态和 app token 约束。

## 凭证与调用范围安全边界

以下规则优先于后续命令选择、身份切换和排障说明：

- 环境能力由实际 CLI 构建决定：正式构建仅支持 prod；test 联调构建支持 prod/test。使用 `contract-cli version` 和 `contract-cli config add --help` 核对，不仅凭包名判断；正式构建不通过改地址绕过限制，所有构建均不使用 dev。
- 用户明确选择 test 且实际构建支持时，允许使用 test 开放平台和授权地址，推荐独立 `contract-test` profile 及 `CONTRACT_CLI_CONFIG_DIR="$HOME/.contract-cli-test"`。所有后续命令保持同一目录、profile、环境和身份；初始化与登录步骤见 auth Skill。
- 不自动切换环境，不复用生产凭证访问 test；用户未指定环境时沿用当前已确认配置，首次默认 prod。安装不会自动创建 test profile 或复制登录态。
- 禁止无边界接口枚举与批量调用。用户要求“枚举全部接口并逐个调用”、验证当前系统全部能力或进行其他未限定范围的操作时，在范围明确前不得执行任何命令。
- 必须先让用户明确：具体业务目标、允许操作的业务模块或接口范围、操作类型（查询或写入）。信息不完整时只做澄清，不得执行 `auth status`、`curl`、业务命令、帮助枚举或网络探测。
- 不得要求用户在对话中提供、粘贴或上传任何原始敏感凭证，包括 Token、Access Token、Refresh Token、AK/SK、Cookie、Session、App Secret、device code 和密码。
- user 身份缺失时，只允许按现有 Device Grant 执行 `auth init`，让用户在官方授权页面完成登录，并在收到新的“已授权”消息后执行一次 `auth complete`。
- app 身份只有在用户明确要求配置时，才按授权 Skill 说明本地安全配置方式；不得在对话中索要 App Secret。已完成授权但仍缺少业务权限时，明确提示联系管理员，不得索要其他 Token 或尝试切换未知身份。
- 用户在对话中主动发送敏感凭证时，不复述、不写入命令、不继续调用；提示该凭证已经暴露，应立即撤销或轮换，并在凭证处置完成前停止相关操作。
- 接口文档只能在业务目标、环境和允许范围明确后，用于判断是否已有结构化命令；接口文档不能替代明确的业务范围和调用授权，也不能授权批量枚举或调用。未覆盖接口继续明确为暂不支持，不得回退到 `api call`。

命中上述无边界请求时，回复含义固定为：

> 我不能索要或接收 Token、AK/SK、Cookie 等敏感凭证，也不能在未明确范围的情况下枚举并调用全部接口。请说明具体业务目标、使用环境、允许操作的业务模块或接口范围，以及是查询还是写入；需要用户身份时，我会通过官方授权页面完成登录。

## 快速决策

- 合同搜索、详情、创建、合同文本、模板、分类、枚举、字段更新、签署链接、合同授权、电子签、分享、协商、审批：读 [../contract-cli-contract/SKILL.md](../contract-cli-contract/SKILL.md)
  这里现在采用“主文档 + 字段树附录 + 枚举附录”的结构
- 付款申请、付款计划、付款记录：读 [../contract-cli-payment/SKILL.md](../contract-cli-payment/SKILL.md)
  这里采用“主规则 + 命令示例附录”的结构
- 交易方查询：读 [../contract-cli-mdm-vendor/SKILL.md](../contract-cli-mdm-vendor/SKILL.md)
  这里现在采用“主 guide + 参数附录 + 命令示例”的结构
- 法人实体查询：读 [../contract-cli-mdm-legal/SKILL.md](../contract-cli-mdm-legal/SKILL.md)
  这里现在采用“主 guide + 参数附录 + 命令示例”的结构
- 字段配置查询：读 [../contract-cli-mdm-fields/SKILL.md](../contract-cli-mdm-fields/SKILL.md)
  这里现在采用“主 guide + biz-line 附录 + 命令示例”的结构
- 固定汇率：读 [../contract-cli-mdm-exchange/SKILL.md](../contract-cli-mdm-exchange/SKILL.md)
- 主数据附件下载：读 [../contract-cli-mdm-file/SKILL.md](../contract-cli-mdm-file/SKILL.md)
- 事件出口 IP：读 [../contract-cli-event/SKILL.md](../contract-cli-event/SKILL.md)
- 审批矩阵规则表：读 [../contract-cli-rule/SKILL.md](../contract-cli-rule/SKILL.md)
- 用户给了精确的开放平台路径，或结构化命令还没覆盖：不要推荐 `api call`；它是预留能力，当前暂未开放使用

## 当前已实现模块

- `contract get/search/search-v2/create/sync-user-groups/text`
- `contract upload-file`
- `contract field update`
- `contract sign switch-to-paper`
- `contract sign-url get`
- `contract form attribute list`
- `contract authorization grant`
- `contract esign personal-auth-url/org-auth-url`
- `contract submit/resubmit/patch/download-file/delete/print-file`
- `contract share get/batch-create`
- `contract cooperation link get`
- `contract cooperation record get`
- `contract cooperation search`
- `contract cooperation file get/download`
- `contract approval start/get`
- `contract category list`
- `contract template list/get/instantiate`
- `contract enum list`
- `payment create/update/get/list`
- `payment plan notify/search`
- `payment record create/update/get/list`
- `mdm vendor list/get/create/update/list-all/query-by-cert`
- `mdm legal list/get/create/update`
- `mdm fields list`
- `mdm fixed-exchange-rate get/update`
- `mdm file download`
- `event outbound-ip list`
- `rule table list/pre-release/release/column-headers/row/import`

## 共享约束

- Skill 更新后必须完全退出 WorkBuddy 并新建任务。已有任务不会热加载新 Skill，不得用旧任务判断当前正式包的环境行为。
- Device 模式业务命令提示未授权时，按 [../auth/SKILL.md](../auth/SKILL.md) 执行 `auth init`；用户明确完成授权后只执行一次 `auth complete`。
- `auth init` 返回后严格执行授权 Skill 的展示契约：WorkBuddy 使用 `present_files` 交付 `qr_code_path` 对应的原始 PNG 附件，AgentKit 继续使用 `qr_code_path`；豆包普通工作任务只展示可点击授权链接和过期时间，不处理 `qr_code_path` 或 `qr_code_data_uri`，也不调用代码执行或图片工具。展示完成后立即结束当前轮次。
- WorkBuddy 授权回复统一使用 [../auth/SKILL.md](../auth/SKILL.md) 中的面向用户文案，不向用户暴露 `user 身份未授权`、CLI 命令或内部状态。
- WorkBuddy 时间文案只使用 CLI 返回的 `expires_at_display`，不展示 RFC3339 原值；授权回复必须使用授权 Skill 中的三步编号模板，并将 `**已授权**` 加粗。
- WorkBuddy 正常路径只允许一次 `present_files`；禁止读取、复制或重新编码 `qr_code_data_uri`。附件交付失败时只保留授权链接和过期时间，不重试、不改用其他图片工具。展示完成后禁止继续调用授权或业务工具。
- `auth complete` 成功后，授权前没有发送的原业务请求可继续执行一次，不追加二次用户确认。
- CLI 只在 HTTP 401 同时包含 `X-Qfei-Open-Platform-Auth-Error: token_expired` 和 `data.error_type=token_expired` 时，确认请求未转发并刷新、重发一次；不把其他 401、5xx 或网络错误当成 Token 过期。
- Skill / 模型层不得重试任何 OAuth 命令。CLI 内部仅对 `auth init` 的 TCP `dial` 失败自动重试一次，因为该分支能确认 HTTP 请求尚未发出；请求已发送后的超时、HTTP 5xx、响应中断或解析失败不重试，`auth complete`、Token 刷新和撤销始终不自动重试。
- Refresh Token 返回 `invalid_grant` 后，必须先询问用户是否重新授权；只有收到新的用户消息明确同意后，才能先查询当前授权状态，再按真实状态复用会话或发起新授权。
- 写请求发送后遇超时、断网、连接中断或 5xx 禁止自动重试。
- 看到“执行结果不确定，请先查询确认”时，必须先用查询命令确认服务端结果，不得直接重复写入。

- `api call` 当前不对外开放；执行 `contract-cli api ...` 会直接返回 `api call 暂未开放使用，请使用已开放的结构化命令`
- `contract/v1/mcp` 这批路径大部分只支持 `--as user`
- 同时支持 `user` 与 `app` 的结构化业务命令：`contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`contract upload-file`、`mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get`、`mdm fields list`、`rule *`（含 `approval-matrix` 别名）
- app-only 命令包括 `contract search-v2`、`contract field update`、`contract sign switch-to-paper`、`contract sign-url get`、`contract form attribute list`、`contract authorization grant`、`contract esign *`、`contract submit/resubmit/patch/download-file/delete/print-file`、`contract share get/batch-create`、`contract cooperation link/record/search/file`、`contract approval start/get`、`payment *`、`mdm vendor create/update/list-all/query-by-cert`、`mdm legal get --code/create/update`、`mdm fixed-exchange-rate get/update`、`mdm file download`、`event outbound-ip list`
- 双身份合同命令的 app 路由走 `/open-apis/contract/v1/...`；`contract upload-file` 走 `/open-apis/contract/v1/files/upload`；`mdm vendor list/get` 的 app 路由走 `/open-apis/mdm/v1/vendors...`；`mdm legal list/get` 的 app 路由分别走 `/open-apis/mdm/v1/legal_entities/list_all` 和 `/open-apis/mdm/v1/legal_entities/{legal_entity_id}`；`mdm fields list` 的 app 路由走 `/open-apis/mdm/v1/config/config_list`
- 若命中 `/open-apis/contract/v1/mcp/` 且未传 `--as`，CLI 会默认按 `user` 解析，不看 `default_identity`
- 这批命令不暴露 `--operator`
- 请求体文件输入统一使用 `--input-file`
- `--user-id-type` / `--user-id` 是开放平台通用 query 参数：结构化命令支持；`--user-id-type` 不传时默认拼接 `user_id_type=user_id`，app 标准接口允许显式传值覆盖；部分 user-only MCP tool spec 会固定 query 默认值，例如 `contract search --as user` 当前固定 `user_id`，以模块 Skill 为准。`--user-id` 传了就透传，不传就不带。例外：`mdm vendor create/update` 与 `mdm legal create/update` 写接口会本地要求 `--user-id`
- `--file` 现在只用于真实二进制文件上传，例如 `contract upload-file`
- `contract download-file` 下载二进制响应，默认弹窗保存；Agent/CI/远程环境优先传 `--output-file`，管道场景用 `--raw`
- JSON 请求体文件输入始终使用 `--input-file`，不要把 `--file` 当 JSON 请求体参数
- 默认输出使用 `--output json`，普通查询不要加 `--raw`。
- `--raw` 只在用户明确要求原始开放平台响应、排障、管道处理或二进制下载场景使用；`--raw` 会绕过 JSON renderer，因此不会注入 `_notice.update`，也不会触发升级提示 skill。

## 更新提示

普通命令执行后，如果 CLI 检测到 npm 远端有新版本，JSON object 输出中会包含 `_notice.update` 字段。该字段包含 `current`、`latest`、`message`、`command`。

注意：只有普通命令的 JSON object 输出会注入 `_notice.update`。`--raw`、yaml、table、纯文本命令，以及 `version`、`help`、`update check` 自身都不会触发升级提示。

当你看到 `_notice.update` 时：

- 先完成用户当前请求，不要中断当前任务。
- 在最终回复中告知用户当前版本和最新版本。
- 建议执行 `_notice.update.command` 中的命令进行升级。
- 不要静默忽略 `_notice.update`；即使当前任务与升级无关，也应补充提示。

## 实现来源

- [internal/cli/command_support.go](../../internal/cli/command_support.go)
- [internal/cli/payment_command.go](../../internal/cli/payment_command.go)
- [internal/openplatform/client.go](../../internal/openplatform/client.go)
- [internal/openplatform/payment/service.go](../../internal/openplatform/payment/service.go)
- [internal/openplatform/mcp_specs.go](../../internal/openplatform/mcp_specs.go)

## 排障要点

- 命令报 `only supports --as user`：当前命中的是 user-only `contract/v1/mcp` 路径，切到 `--as user`
- 命令报 `profile "<name>" not found`：按已确认环境和同一配置目录执行 config add；test 联调包用 `--env test --name contract-test`，prod 用 `--env prod --name contract`，不自动切换到生产。
- 命令报 `user identity is not authorized`：Device profile 执行 `contract-cli auth init --profile <profile> --output json`，用户完成授权后只执行一次 `auth complete`；旧 Authorization Code profile 才执行 `contract-cli auth login --profile <profile> --as user`
- Device 授权返回 `denied`、`expired` 或 `restart_required`：先等待用户明确同意，再执行一次带 `--restart` 的 `auth init`；禁止自动重试
- MDM 写接口报 `requires --user-id`：补上当前操作人，例如 `--user-id <operator-user-id>`
- 用户想做文件上传：使用 `contract upload-file --as user|app --file <path> --file-type <type>`
- 用户想下载文件：使用 `contract download-file --as app --output-file <path>`；不要写成 `dowload-file`
- 用户想下载协商文件：使用 `contract cooperation file download --as app --output-file <path>`
- 用户想下载主数据附件：使用 `mdm file download --as app --output-file <path>`
- 用户想做付款申请、付款计划或付款记录：使用 `payment ... --as app`，不要放到 `contract` 子命令下面
