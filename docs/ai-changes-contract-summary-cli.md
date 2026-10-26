# AI 变更记录：合同摘要查询 CLI

## 2026-10-09

- 改动内容：早期新增 `contract-cli contract summary query` 命令时曾按 app 身份和普通 OpenAPI 摘要路径设计；该口径已被后续“接口契约更正”和“Device 路由修复”覆盖。
- 影响范围：合同命令分发、app-only OpenAPI 命令处理、本地帮助、命令参考文档、README、内置 contract/shared skills 及相关契约测试。
- 关键决策：未访问私有飞书接口文档；仅基于用户提供的开放平台路径和仓库既有 app-only 查询命令约定实现。摘要筛选条件不拆成本地 flag，保持 JSON body 透传；通用 `--user-id-type` / `--user-id` 仍按共享规则拼到 query string。
- 验证结果：先补端点和帮助测试复现未知命令失败，再实现命令；目标测试 `go test ./internal/cli -run 'TestOpenAPIGapCommandsUseExpectedEndpointsAsBot|TestHelpRequestsRenderExpectedTopics|TestAllCurrentHelpTopicsRender'` 通过。
- 未处理项或风险项：未进行真实后端带 token 调用；如果接口文档实际要求非 POST 方法或固定请求体字段，需要按文档再调整命令契约。

## 2026-10-09 接口契约更正

- 改动内容：根据开放平台与 CLM 后端实现，将 `contract summary query` 调整为用户身份命令；新增 `--contract-number`、`--contract-name`，同时兼容 `--input-file` / `--data`，并统一生成后端要求的 camelCase 请求字段。
- 影响范围：命令身份策略、请求体校验、帮助与参考文档、内置 skills 和契约测试。
- 关键决策：请求必须且只能包含合同编号或合同名称之一；查询服务在没有可用摘要快照时可能触发摘要生成，因此按写操作处理，不自动重试或轮询。
- 验证结果：测试覆盖用户身份、精确请求体、互斥筛选条件和帮助契约；`go test ./internal/cli` 通过目标用例。
- 未处理项或风险项：尚未使用真实用户 token 对本地后端进行端到端联调；调用方仍需检查响应中的业务码、`status`、`errorCode` 和 `errorMsg`。

## 2026-10-09 Device 路由修复

- 改动内容：将 `contract summary query` 的用户态调用路径从普通 OpenAPI 路径调整为 `POST /open-apis/contract/v1/mcp/summary/query`，避免 Device Token 被网关判定为路由不匹配。
- 影响范围：摘要查询命令、帮助文案、命令参考文档、内置 skills 和契约测试。
- 关键决策：摘要查询仍保持 user-only；请求体和“不确定结果不自动重试”的语义不变。
- 验证结果：先改测试复现旧路径失败，再切换到 MCP 路径；目标测试覆盖路径、帮助和 Skill 文档。

## 2026-10-09 普通开平路径恢复

- 改动内容：按用户明确要求，将 `contract summary query` 的用户态调用路径从 MCP 摘要路径恢复为 `POST /open-apis/contract/v1/summary/query`。
- 影响范围：摘要查询命令、帮助文案、命令参考文档、内置 skills 和契约测试。
- 关键决策：只移除 `/mcp` 路由前缀，仍保持 user-only、精确匹配请求体和“不确定结果不自动重试”的语义。
- 验证结果：同步更新路径断言和文档契约测试，确保当前 CLI 不再调用 `/mcp/summary/query`。

## 2026-10-09 不确定结果错误原因透出

- 改动内容：增强 OpenPlatform 写类请求的不确定结果错误文案，在保留“执行结果不确定，请先查询确认”的前提下追加底层 HTTP/网络错误原因。
- 影响范围：所有按写操作处理的开放平台请求错误展示；合同摘要查询在网络或 5xx 异常时可直接看到具体原因。
- 关键决策：不改变不确定请求的重试策略，仍避免对可能触发生成的摘要查询进行自动重试；只提升可诊断性。
- 验证结果：`go test ./internal/openplatform ./internal/cli` 通过；本地安装包在 Codex 沙箱内复现为 DNS 权限拦截，错误已可明确显示 `lookup dev-open.qtech.cn` 失败。
