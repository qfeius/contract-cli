# 非业务命令统一显式 JSON 输出

## 2026-10-23

- 基于最新 `test-agent`（`96c8164a621a7e87b9231183892a2e74d96051a5`），修复 config add、auth login/status/logout/use、version、skills list/install、update check 对显式 `--output json` 支持不一致的问题。
- 默认文本、业务输出及 Trace 保持原样；新成功结果在 stdout 输出 `{ok:true,data}`，失败与取消在 stderr 输出 JSON，退出码仍为 1/130；update check 保留旧 `--json` 成功字段并统一 JSON 错误。
- OAuth 人工授权链接作为 stderr JSON 事件；Skill 安装失败的 CLI、登记、恢复状态和建议写入 `error.details`，保留原错误链，不输出 Token/App Secret。
- 延续既有认证、安装、目录保护及环境门禁调用链；提前拒绝非法位置参数，未增加锁、后端修改或 dev 支持。
- 按 TDD 先复现参数拒绝、混入文本、取消、非法位置参数及 update 别名覆盖问题，再完成实现；新增命令帮助与 macOS 临时目录规范化回归测试。
- 全量 Go 测试、race、vet、CLI 冒烟、14 个实际二进制 JSON 场景、macOS/Linux/Windows 构建、旧构建参数环境门禁及 Node 安装/测试包校验通过；整体覆盖率 81.4%，CLI 包 82.8%。验证使用临时目录与模拟 HTTP，未操作真实业务或用户授权。
- README、命令参考及命令帮助已同步；独立复审发现的 update 空 output 与旧 JSON 别名不一致问题已修复并复核通过。
