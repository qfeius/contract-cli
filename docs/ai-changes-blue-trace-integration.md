# blue 验收分支的 Trace、JSON 与取消处理集成

## 2026-10-27

- 基于 `blue@f71e507` 创建 `codex/dev-trace-blue-20261027`，合并统一维护分支 `codex/dev-trace-verification@3cfe097`；范围为命令共享 Trace ID、日志开关、显式 JSON、Ctrl+C 与升级提示。
- 保留 blue 的 prod/test 地址预设、自定义 HTTPS 业务/授权地址、同源守卫及 CredentialTargetBinding/Device 快照隔离；同名换目标时清理原认证，取消发现请求时保留旧状态。
- 配置及认证冲突同时保留真实提交时的 binding 更新与取消保护；签名文件下载继续使用独立 fileHTTPClient，Open Web 纳入命令 Trace，协商和审查输入沿原调用链传递 ctx。
- 不整合 test-agent 后续业务 Skill 更新，不修改 blue 的 Skill、package.json、配置存储及 OAuth 实现；维护分支及 master 的源码不受本次集成影响。
- TDD：原 blue 的自定义端点 JSON 两项、发现完成后取消一项及 Open Web 默认日志/调试两项均先红后绿；新增针对性回归并保留原端点、认证隔离、协商和下载测试。
- 原 blue 快照复现环境探测测试夹具越界，仅注入空的内存凭据库以隔离运行时探测；JSON/Device 夹具补齐目标 binding 和快照，不放宽运行时安全校验。
- 验证：全量 Go 测试与 race、vet、CLI 冒烟、旧链接参数及 dev 标签门禁通过，整体覆盖率 81.6%；Node 安装/setup/Skill/打包测试 23 通过、1 项平台跳过，npm 预检 146 文件通过，macOS ARM64/Linux AMD64/Windows AMD64 构建通过。
- 验证使用临时配置、模拟 HTTP 和隔离源码快照；本次不发布安装包、不操作真实业务或后端，不替换本机已安装 CLI。MR 供 blue 环境继续验收。
