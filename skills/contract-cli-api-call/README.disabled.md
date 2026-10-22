# contract-cli API Call Is Not a Standalone Skill

`contract-cli api call --channel web` 已开放为业务 Skill 的通用执行通道，但它本身不负责接口发现、参数映射或业务编排。

本目录不包含 `SKILL.md`，也继续从 npm skills 包和 `contract-cli skills install` 中排除。业务 Agent 应先加载具体业务 Skill，再由其中的已验证配方调用该通道，不能隐式调用本目录猜测接口。

CLI 传输边界：

- 仅 `--channel web` 和 `/open-web/web/**`
- 仅 CLI 自身 user profile / Device Grant 授权
- 不接受外部 Token、Cookie 或可信身份头
- HTTP 响应仍由业务 Skill 按业务码和数据语义解释
