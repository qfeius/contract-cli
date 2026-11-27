# WorkBuddy 设备码授权适配

## 结论与范围

服务端继续使用现有 Device Grant：申请设备授权、用户在账号页面确认、CLI 主动查询 token endpoint。无需浏览器回调，不新增服务端接口或修改 grant/client/resource/scope。新增的是 CLI 中供宿主调度的等待入口，不替换原有 Agent 授权流程。

2026-11-27 核对生产公开元数据 `https://myaccount.qfei.cn/.well-known/oauth-authorization-server/contract`：已经声明 `urn:ietf:params:oauth:grant-type:device_code`、设备授权端点、token 端点、refresh_token 和撤销端点。随后在用户确认下使用独立测试 profile 完成 WorkBuddy 任务内真实授权；没有读取或输出 Token 原文。连接器按钮的完整上架链路尚未验收。

## 调用

```sh
contract-cli auth device-login --profile contract --timeout 5m
```

- 申请或复用待授权会话，输出无引号的完整 HTTPS 链接。WorkBuddy 负责展示链接；CLI 不打开浏览器、不启动 HTTP 回调监听。
- 等待不超过命令超时和设备码有效期；已授权则复用或刷新现有凭据，直接成功退出。
- 申请阶段有 9 秒上下文超时；依赖系统安全存储及时返回，仍需在目标客户端验证平台的 10 秒 URL 检测限制。
- 使用服务端 `interval`，缺失时 5 秒。只有明确 `authorization_pending` 继续；`slow_down` 增加 5 秒并保存到待授权状态，供续接时遵循。
- 拒绝、过期、无效 grant、busy、未知网络结果或持久化失败均退出非零；不重发未知结果，不自动替换旧授权。保留既有 init 的仅 TCP 拨号未发送请求时重试一次的规则。
- 等待期间不持续持有凭据锁；每次查询复用已有授权操作锁，并校验授权链接未被另一操作替换。
- 成功结果仍写入原 Device CredentialStore。`auth status` 只读，`auth logout` 仍走原有撤销和清理流程。

原 `auth init/complete` 的 JSON 输出及单次调用契约不变；新增等待模式不会在普通 Agent 对话中自动启用。旧 OAuth `auth login` 不修改，也不作为本连接器方案。

## 上架配置

将 `connectors/workbuddy/auth.fragment.json` 中的字段合入动态安装 Skill 包的 cli.json，不要把片段单独当作完整配置上传。connector-meta.json 的 `minWorkbuddyVersion` 至少为 `5.0.0`。

保留原 init、status、unAuth、statusMatch 及配置目录。使用 `authDeviceFlow` 展示设备码授权，并显式设置 `authWaitForExit: true` 保留 CLI 进程；不配置回调 URL，也不要在 status 中循环兑换 Token。

本机 WorkBuddy 5.3.14 的实际调度方法只在同时存在 `authDeviceFlow` 和界面 `onDeviceCode` 回调时自动保留进程，因此额外的 `authWaitForExit` 用于覆盖缺少该界面回调的入口。这里的界面回调不是 OAuth 服务端回调。两个入口均等待 CLI 保存凭据后成功退出，不因提取到链接立即杀进程；用户取消、5 分钟超时或客户端崩溃仍可能结束进程，不能保证任意中断后都拿到结果。

本功能尚未包含在 npm 1.9.16 中。需要先发布包含本改动的正式 CLI，再将连接器最低 CLI 版本设为该实际发布版本；不能仅更新 ZIP 仍安装旧 CLI，就宣称授权可用。本次代码不改 CLI 版本号、Tag、GitHub Actions 或 npm 发布。上架资料独立更新为 1.0.1，按用户要求同时附带当前 28 个 Skill 并保留动态安装；安装前增加 device-login 能力检查，不强制覆盖已有同名目录。双来源 Skill 的加载去重仍需客户端验收。

动态安装 Skill 的方案保持不变。安装阶段、连接器 auth/status/unAuth 和 Agent 的业务命令必须使用同一 CLI 配置目录及 profile，不能只给连接器进程注入独立目录却让业务命令使用默认目录。

## 验证边界

- 回归包含链接输出后才查询、凭据保存/复用、不泄漏 Token、pending/slow_down、拒绝/过期/不确定状态、取消、超时、并发替换防护和旧授权 API 兼容。
- `scripts/workbuddy-auth-lifecycle.test.js` 提取已安装客户端的实际授权调度方法，使用无网络的真实子进程验证默认 URL 模式提前终止、配置后两类入口保留进程直到结果、取消及非零退出；没有安装 WorkBuddy 时跳过客户端专项测试，配置断言始终执行。这不是账号授权端到端测试。
- macOS/Windows 仍复用已有桌面识别与系统安全存储；未知宿主及云端不放宽隔离。
- Linux 当前依旧是任务隔离/Secret Service 路径。如果连接器进程没有任务标识，或重启后的凭据范围改变，不能保证连接器跨重启可用；这是客户端存储适配边界，不是服务端需要回调。本轮不伪造固定任务 ID、不扩大共享权限，Linux 上架前需专项验证。
- 上架验收需在真实 WorkBuddy 中验证首次授权、拒绝/取消、重启、过期刷新、登出及业务命令复用。本轮模拟回归和公开元数据检查不替代该验收。

## 本机真实授权记录

2026-11-27，测试版 `1.9.16-test.workbuddy.20261127.1` 独立安装于本机 macOS arm64，通过 WorkBuddy 5.3.14 新任务运行 `auth device-login`。用户浏览器确认后，CLI 输出 `Authorization: authorized`，退出 0；WorkBuddy 新进程查询和 Codex 独立只读查询均返回 `authorized`、凭据范围 `user`。等待期间核对到 CLI 进程属于 WorkBuddy Electron 子进程树，未使用 nohup 或另起后台。

开放平台接受本地测试 ZIP，但“连接自测”页面仅测试远程服务端认证，CLI 客户端认证直接跳过。没有提交审核或发布，也没有修改客户端市场缓存来模拟上架。本次任务工具调用不是连接器服务按钮调用，因此仍需后续验证按钮全链路和客户端整体重启。详细记录及测试包位于本机输出目录 `workbuddy-device-auth-test-20261127`。

官方规范：https://open.workbuddy.cn/docs/connector
