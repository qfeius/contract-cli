# API Call Rules Disabled

`contract-cli api call` 是预留调试入口，当前不对外开放。

当前 CLI 行为：

- 执行 `contract-cli api ...` 会直接返回 `api call 暂未开放使用，请使用已开放的结构化命令`
- 不读取 profile
- 不发 HTTP 请求
- 不出现在 help 或 skills 安装列表中

恢复开放前，需要重新补齐 help、测试、文档和 skill，并确认安全边界。
