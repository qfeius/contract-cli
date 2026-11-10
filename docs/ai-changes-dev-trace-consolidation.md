# 统一以 dev-trace-verification 管理本次更改

## 2026-10-23

- 按用户要求，后续本项目改动统一以 `codex/dev-trace-verification` 为维护分支，主工作目录为 `/Users/liguang/Documents/ChatGPT/contract-cli`；本次基于原提交 `394405f` 回移功能，不合并 test-agent 整条历史。
- 显式 JSON 改造从 `35a8ef6` 回移为 `ddb45b4`；Skill Ctrl+C 取消修复从 `95cd6c9` 回移为 `80ac774`，提交信息保留 cherry-pick 来源。
- 保留原 Trace、日志开关、登录及输入取消、升级提示和严格 prod 门禁；环境守卫、原始 API 开关、业务 Skill、版本号及 main 入口与原分支不变。
- 仅适配必要冲突：README/命令参考保留 prod 与原始 API 未开放说明；安装暂存仍使用原 0644 权限策略；JSON 配置测试改为 prod 初始化/重配与 dev/test/blue 零请求拒绝，历史非prod恢复prod时清理旧认证。
- 全量 Go 测试、race、vet、CLI 冒烟、`-tags dev` 与旧联调链接参数门禁检查通过，整体语句覆盖率 81.0%；JSON、真实 SIGINT 和安装恢复回归通过。测试仅使用临时目录和模拟 HTTP。
- 原基线保留于 `codex/backup-dev-trace-before-consolidation-20261023`；原 `codex/unified-json-output-20261023` 保留为测试版来源和回滚依据，后续不在该临时分支继续实现功能。
- 本次统一源码分支，不替换已安装的 test-agent 测试二进制；本机仍为 `1.9.2-beta.1-test.20261023.95cd6c9.1`，避免将 prod 源码管理与 test 环境测试包混淆。
