# GitHub release 同步内部 GitLab master

## 2026-10-29

- 需求：将远程 GitHub release 以独立同步分支 MR 合入内部 GitLab master，保留 GitHub PR #28 及双方提交历史。
- 来源：`qfeius/contract-cli:release@ae39acb93fd659636af6a0017231cda68e1dc942`；目标基线：`master@28b41cccaaf77aa65c9c94826213e457eaa49281`。
- 分支：`codex/sync-github-release-master-20261029`；功能文件与 release 一致，不新增功能修补，不改版本号或发布工作流。
- 冲突：仅 legacy `docs/ai-changes.md`，保留 master 独有的旧同步记录及 release 全部原记录；本次记录独立存放。
- 范围：命令共享 Trace、日志开关、显式 JSON、Ctrl+C 与取消边界、合同文本业务失败退出码，以及 release 原有 Open Web、协商、交接和 Skill 能力。
- 环境：随完整 release 同步 prod/test 预设和自定义 HTTPS 端点，保留凭据目标绑定、Device 快照与同源守卫；没有新增 dev/blue 环境预设。
- 验证：全量 Go 测试、go vet 通过；Node 安装、setup、Skill 合并和测试包回归 23 项通过、1 项 Windows 平台跳过；除两份变更记录外文件与来源逐项一致。
- 已知问题：来源版本的部分布尔参数与全局日志参数排列解析问题、npm 自动 Skill 同步取消未贯通仍保留；显式 skills install 取消修复有效。既有 MR !20、!33 保持原样。
