# AI 变更记录：CLI test 环境恢复

> 历史记录：此阶段已被 2026-10-15 再次切回 prod-only 的需求覆盖，当前能力见 [CLI 仅保留 prod 环境](ai-changes-cli-prod-only.md)。

## 2026-10-15 恢复 test 环境支持

- 改动内容：恢复 `config add --env test` 预设及 test 开放平台、账号端点；允许与 test 端点一致的已保存 profile、Device 快照和 test 域名请求。同步恢复帮助、文档、内置 Skills 和发布检查中的 prod/test 说明。
- 影响范围：环境预设、配置与凭据校验、传输层拦截规则、用户指引及相关回归测试；保留摘要查询 `status=1` 时隐藏旧摘要的未提交改动。
- 关键决策：仍拒绝 dev/blue；同名 profile 在 prod/test 间切换时继续清理旧认证，不跨环境复用凭据。
- 验证结果：先把 test 配置、已保存 profile、Device 快照与业务上下文测试改为期望支持，确认旧实现下失败；实现后定向测试与 `make release-check` 均通过。新增 prod→test 同名 profile 切换回归，确认旧 user/app token、App Secret 和 Device 凭据被清理。
