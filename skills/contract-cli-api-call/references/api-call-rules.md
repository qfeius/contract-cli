# Open Web API Call Rules

`contract-cli api call --channel web` 已开放为业务 Skill 的底层传输能力，但本目录不发布独立 `SKILL.md`，避免把传输通道误用成接口发现服务。

当前 CLI 边界：

- 必须显式传 `--channel web`，且只接受相对 `/open-web/web/**` 路径
- 只使用 user 身份；授权来自 CLI profile / Device Grant
- query 通过可重复的 `--query key=value` 传入，不自动追加开放平台通用 query
- body 最大 1 MiB；响应业务语义由上层业务 Skill 判断
- 禁止传入 Authorization、Cookie、租户/用户、网关和来源身份头

接口发现、参数映射、读写确认和业务编排必须留在业务 Skill / Agent 层。
