# test 环境 CLI 支持变更记录

## 2026-10-10

- 变更摘要：将当前分支的环境预设调整为 prod/test，默认仍为 prod。
- 关键逻辑/决策：`config add --env test` 写入 `https://test-open.qtech.cn`、`https://test-myaccount.qtech.cn` 和 test app token endpoint；profile 校验按声明环境做同源校验，避免 prod/test 跨环境复用端点或凭证；传输层继续阻断历史 dev/blue 环境域名。
- 影响范围：配置初始化、profile/Device 快照校验、app 登录提示、Open Platform 请求上下文、帮助文案、README、命令文档和相关 Skill 环境说明。
- 验证：新增/更新 test profile、test 请求上下文、test Device 快照恢复、dev/blue 拦截、帮助文案和 Skill 安全边界测试。
