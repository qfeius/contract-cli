# AI 变更记录：CLI 仅保留 prod 环境

> 当前状态：2026-10-20 按用户要求从 MR !20 剥离 dev 能力，当前源码所有构建严格仅支持 prod，包括误传 dev 构建标签。中途恢复 test 的历史见 [CLI test 环境恢复](ai-changes-cli-test-environment.md)。

## 2026-10-20 从 MR !20 剥离 dev 支持

- 本地验证代码已保存为未推送的 `codex/local-dev-verification@6de2b57`，独立二进制位于 `~/.local/share/contract-cli/dev-verification/6de2b57/contract-cli`；本机配置和凭据未修改。
- 删除五个 development 构建开关、预设和专用测试文件；环境解析、profile/Device 凭据保护、HTTP guard、帮助和原有非生产拒绝断言恢复为最新 master 的严格 prod 规则，清除所有 dev 例外与测试跳过。Trace 错误链断言及命令 Trace、日志开关、中断/取消后停止分发修复保留。
- TDD：新增配置、域名变体、生产重定向到 dev 的负向测试，以及发布检查中显式 dev 标签的验证，先确认旧 dev 构建下失败，再移除入口。README/命令参考不再提供 dev 启用流程，历史联调证据标明仅属于本地验证分支。
- 验证：默认和误传 `-tags dev` 的全量、race、两种构建 vet、发布构建标志检查和 Linux/Windows 交叉编译均通过；两个实际二进制仅显示 prod，并拒绝 dev/test/blue 配置及历史 dev profile。两种构建的 Go 源文件列表相同，环境保护文件与 origin/master 无差异。
- prod-only 二进制的三条真实更新阶段 SIGINT 用例均退出 130，未更改配置、Skill 文件和登记；未访问真实 dev/prod 业务接口。证据目录 `/tmp/contract-cli-prod-only-mr20-20261020/`，本次仅构建独立候选，未替换本机已安装 CLI；只更新 MR，不合并或发布。

## 2026-10-15 移除 test 环境支持

- 改动内容：`config add` 仅接受 `prod`；已保存的非生产 profile、Device 快照和待授权状态在使用前被拒绝，传输层阻断已知非生产域名。同步更新错误提示、帮助、命令参考、README、内置 Skills 和发布检查。
- 影响范围：环境预设、配置及凭据校验、HTTP 传输保护、用户指引和相关契约测试。原有未提交的摘要输出格式修复保持独立。
- 关键决策：保留现有 profile 与凭据校验调用链；旧配置不会自动迁移或覆盖，重新配置 prod 时需重新授权。
- 验证结果：先把配置、已保存 profile、Device 快照及传输层测试改为要求拒绝 test，确认旧实现下失败；修改后定向测试、`go test ./...` 和 `make release-check` 均通过。发布检查使用符合项目要求的 Node 23。

## 2026-10-15 再次仅保留 prod

- 改动内容：撤销本日临时恢复的 test 环境预设及端点支持，配置、已保存 profile、Device 快照和传输层重新只接受 prod；同步撤销帮助、命令文档和 Skill 中的 test 环境说明。
- 影响范围：CLI 环境与认证边界、用户文档及本地分享包；合同摘要 `status=1` 的隐藏内容与提示语保持不变。
- 关键决策：保留历史记录文件，不把中途的 test 恢复描述为当前能力；本地新分享包另取 `-local` 版本，与曾经部署的 `-test` 包区分。
- 验证结果：先恢复 prod-only 回归断言并确认 test 能力使其失败；实现后定向测试与完整 `make release-check` 均通过。
