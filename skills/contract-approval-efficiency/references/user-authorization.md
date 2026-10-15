# 业务 Skill 固定授权规范（USER_ONLY_V1）

本文件随业务 Skill 一起交付；执行前与 `api-contract.yaml` 一起读取。只处理当前业务的用户授权，不提供 APP 登录或身份切换能力。

## 身份、环境与凭证边界

- CLI、MCP、所有前置/业务/回查接口及 `permissions.transport_identity` 均只允许 USER。APP 已登录不代表用户已授权；不能用 APP 执行业务、绕过权限或作为失败兜底，也不修改全局默认身份。
- 先核对契约的执行模式、目标环境、通道实际版本和能力。CLI 全程显式沿用同一个已确认 profile；支持身份参数的业务命令显式传 `--as user`。通道不能保证用户身份时停止调用。
- 正式 CLI 的标准环境为 prod、profile 为 contract；本规范不授予生产操作权限。Builder 的 test 验证必须使用明确获准的测试交付通道和独立 test profile。契约、实际构建能力或交付环境边界不一致时停止并报告缺口，不改域名、不混用凭据、不自动切换环境。先核实实际命令帮助，不因包名、历史 profile 或测试构建支持 test 就推断其 Agent 流程获准使用 test。
- 不索取、接收、构造、展示或保存 Token、Refresh Token、Cookie、Session、AK/SK、App Secret、device_code、密码或身份覆盖 Header。不得读取凭证文件代替授权，不借用他人身份。用户主动发送凭证时不复述、不拼入命令，停止相关调用并提示撤销或轮换。
- 业务目标、接口范围和读写类型应明确；禁止为了检查授权而枚举全部接口。授权成功不等于拥有业务权限，也不代替写入确认。

## CLI Device Grant 固定流程

以下 `<profile>` 指同一个已确认 profile，不是可直接执行的字面值。仅在允许 CLI 的模式且命令帮助已确认支持时执行。

1. 新任务或授权状态未知时执行 `contract-cli auth status --profile <profile> --as user`。该命令不支持 `--output`，也不会刷新 Token。本任务已确认有效的同环境、同 profile、同身份授权直接复用。
2. 需要建立或恢复 Device 授权时执行一次 `contract-cli auth init --profile <profile> --output json`。profile 不存在时，先按获准环境及实际帮助完成配置，不覆盖已有异环境 profile。
3. 按真实返回状态处理：

| 返回 | 必须执行的动作 |
|---|---|
| `authorized` | 已复用有效或刷新后的凭据，直接续办原请求；不展示授权链接/二维码，不调用 complete。 |
| `pending` | 按下节展示官方授权入口和过期时间，立即结束本轮；不执行 complete、再次 init、业务请求或轮询。 |
| init 失败 | 如实说明失败，询问是否重新发起并结束本轮；不追加 status、curl、网络探测或重试。 |

4. 只有收到新的用户消息明确“已授权”后，执行一次 `contract-cli auth complete --profile <profile> --output json`：

| 返回 | 必须执行的动作 |
|---|---|
| `succeeded` | 续办授权前尚未发送的原请求；已有范围内的写入确认继续有效，未取得业务确认则先确认。 |
| `pending` | 告知尚未完成并结束本轮，不轮询。 |
| `uncertain` / `busy` | 告知结果不确定并结束本轮，不重试 complete，不重新 init。 |
| `denied` / `expired` / `restart_required` | 先询问是否重新授权；收到新消息明确同意后才执行一次带 `--restart` 的 init，再按真实状态处理。 |

Refresh Token 的 `invalid_grant` 也先询问是否重新授权；收到新消息明确同意后，先查 user 授权状态，再选择复用会话、普通 init 或需要重启时的 init --restart。不能直接重放原业务写请求。

仅当实际 profile 明确使用历史 Authorization Code + PKCE 模式时，按已核实的 `auth login --profile <profile> --as user` 官方流程恢复；不得自动迁移 Device 凭据或把该流程作为不确定状态的重试通道。

## pending 的授权展示

正文必须展示 CLI 返回的完整 HTTPS `verification_uri_complete`，格式为 `[打开授权页面](<verification_uri_complete>)`，以及 **<expires_at_display>**；不猜链接、不手工拼接用户码、不要求用户抄授权码。不展示 RFC3339 原始时间、内部状态或 CLI 命令。将“原业务操作”替换为用户实际请求，不能把写任务一律说成查询。

- WorkBuddy：仅调用一次 `present_files` 交付 `qr_code_path` 对应的原始 PNG；正文同时保留授权链接和过期时间。不读取、复制或重新编码 `qr_code_data_uri`，不重新生成二维码。工具失败时说明“二维码附件展示失败，请直接使用授权链接”，不重试或换工具。
- 豆包 AgentKit：按平台能力仅展示一次 `qr_code_path` 原始 PNG，并给出正文链接和过期时间；展示失败后只保留链接，不换工具或重试。
- 豆包普通工作任务：只展示链接和过期时间，不读取或处理二维码字段，不调用代码执行、图片处理或图片交付工具。
- Codex：正文展示链接和过期时间；若 CLI 返回可展示的本地 PNG，可按宿主能力直接展示原图，不自行生成或解码二维码。

固定文案（有原图附件时第一步可补充“或扫描本消息中的二维码图片附件”）：

> 继续原业务操作前，需要先完成智书合同授权。
> 1. 点击蓝色链接「[打开授权页面](<verification_uri_complete>)」
> 2. 在 **<expires_at_display>** 前完成：手机号验证 + 企业确认授权
> 3. 授权全部完成后，回复消息：**已授权**，我将继续为你执行原业务操作

展示完成后立即结束当前轮次，等待新的用户消息。

## 凭证范围与 MCP

- 以 CLI 返回的 `credential_scope` / `Credential Scope` 为准：`user` 可在当前系统用户及同 profile 下跨任务复用，`task` 仅限当前任务。不能仅凭操作系统、任务 ID 或本地目录推断共享资格。
- 不修改环境标记伪装本地客户端，不复制、解密或迁移凭据。云端/沙箱保持任务隔离；隔离模式的豆包普通工作任务在初始工作目录持续执行，不通过换目录获取其他授权。缺少安全存储或任务隔离必需配置时停止，按通道文档修复，不降级明文存储。
- MCP_ONLY 不执行 CLI 授权命令。MCP 使用连接器自身的当前用户 OAuth 授权及真实 Tool Schema；不能把 CLI 的 Device 状态或命令套到 MCP。缺少用户授权时引导连接器授权，完成后续办原请求；不能确认 USER 上下文时停止，不索取凭证或传入身份覆盖字段。
- 业务权限拒绝时提示联系管理员，不换身份、环境或通道重放请求。

## 重试与原任务续办

- Agent 不重试 OAuth 命令。CLI 内部仅在能确认请求未发出的 TCP dial 失败时对 init 做一次安全重试；这不授权 Agent 再执行一次命令。
- Token 刷新由 CLI 内部处理。仅 HTTP 401 同时带 `X-Qfei-Open-Platform-Auth-Error: token_expired` 和 `data.error_type=token_expired` 时，CLI 才可按自身协议确认未转发并刷新重发一次；Agent 不手动实现这一重试，其他 401/5xx/网络错误不能当作已确认过期。
- 写请求已发送但超时、断网、5xx 或结果不确定时，先只读核对，不自动重试、不换 CLI/MCP 重放。恢复授权只允许续办尚未发送且仍在原授权范围内的请求；无法判断是否已发送时保留待核实。
