# AI 变更记录：智书合同摘要 Skill 适配

## 2026-10-09

- 改动内容：将产品维护的 `zhishu-contract-summary` Skill 适配为当前项目的内置 Skill，配合 `contract summary query` 和 `contract summary generate` 两个 CLI 接口使用。
- 影响范围：新增 Skill 主说明、代理元数据、接口契约、展示交互、摘要卡片和验收参考文档；更新 shared/contract Skill 路由、README 与命令参考文档。
- 关键决策：当前 CLI 支持 `prod`、`dev` 环境，两个摘要接口均只使用用户 OAuth；合同摘要查询与本地文件摘要是互相独立的流程，不互相降级；查询可能触发生成，两个接口都不自动重试或轮询；当前 CLI 不支持把摘要卡片发送到群聊，Skill 必须明确告知限制。
- 验证结果：新增 Skill 内嵌与安全策略测试，并校验 Skill 文档必须包含身份、环境、请求字段、文件限制及非重试约束；`quick_validate.py skills/zhishu-contract-summary` 与 `go test ./...` 均通过。
- 未处理项或风险项：未使用真实用户 token 对本地服务执行端到端调用；产品原 Skill 中的通用 connector/MCP、自动轮询和群发卡片能力没有映射到当前 CLI。
