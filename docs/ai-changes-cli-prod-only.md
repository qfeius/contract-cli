# AI 变更记录：CLI 仅保留 prod 环境

> 当前状态：2026-10-15 再次按用户要求切回 prod-only。中途恢复 test 的记录见 [CLI test 环境恢复](ai-changes-cli-test-environment.md)。

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
