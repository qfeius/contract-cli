# AI 变更记录：合同摘要生成 CLI

## 2026-10-09

- 改动内容：早期新增并调整 `contract-cli contract summary generate` 命令时曾按 app 身份和普通 OpenAPI 摘要路径设计；该口径已被后续“接口契约更正”和“Device 路由修复”覆盖，仍保留不经过独立 upload 接口的直传文件语义。
- 影响范围：合同 summary 命令分发、合同开放平台 service、帮助文案、README、命令参考文档、内置 contract/shared skills 及相关回归测试。
- 关键决策：按最新口径取消独立上传阶段，但保留 `--file` / `--file-name` 用于直传 generate；移除 `--file-id-field` 语义，并拒绝 `--input-file` / `--data`，避免误用 JSON body。
- 验证结果：补充并调整测试覆盖直调 generate、单次请求、multipart `file` 字段、缺少 `--file` 报错、拒绝 JSON body 和帮助输出；实现后目标测试通过。
- 未处理项或风险项：未使用真实 app token 联调；若 generate 接口后续要求额外 form-data 字段，需要再按接口文档补充参数映射。

## 2026-10-09 接口契约更正

- 改动内容：根据开放平台与 CLM 后端实现，将 `contract summary generate` 调整为用户身份命令；限制上传文件为非空的 `.doc`、`.docx` 或 `.pdf`，大小不超过 200 MB。
- 影响范围：命令身份策略、文件前置校验、帮助与参考文档、内置 skills 和契约测试。
- 关键决策：接口只接受 multipart/form-data 的 `file` 字段，文件摘要与合同数据不建立关联；该生成操作不可安全重放，因此不自动重试。
- 验证结果：测试覆盖用户 token、multipart 字段、扩展名限制、大小限制、缺少文件及帮助契约；`go test ./internal/cli` 通过目标用例。
- 未处理项或风险项：尚未使用真实用户 token 对本地后端进行端到端联调；自定义 `--file-name` 也必须保留受支持的扩展名。

## 2026-10-09 Device 路由修复

- 改动内容：将 `contract summary generate` 的用户态调用路径从普通 OpenAPI 路径调整为 `POST /open-apis/contract/v1/mcp/summary/generate`，与 Device Token 的 MCP 路由授权保持一致。
- 影响范围：摘要生成命令、帮助文案、命令参考文档、内置 skills 和契约测试。
- 关键决策：multipart `file` 字段、文件校验和不可重试语义不变。
- 验证结果：先改测试复现旧路径失败，再切换到 MCP 路径；目标测试覆盖路径、multipart 请求和文档契约。

## 2026-10-09 普通开平路径恢复

- 改动内容：按用户明确要求，将 `contract summary generate` 的用户态调用路径从 MCP 摘要路径恢复为 `POST /open-apis/contract/v1/summary/generate`。
- 影响范围：摘要生成命令、帮助文案、命令参考文档、内置 skills 和契约测试。
- 关键决策：只移除 `/mcp` 路由前缀，仍保持 user-only、multipart `file` 字段、文件校验和不可重试语义。
- 验证结果：同步更新路径断言和文档契约测试，确保当前 CLI 不再调用 `/mcp/summary/generate`。

## 2026-10-14 不支持的输出格式请求前拦截

- 改动内容：修复 B08。`contract summary generate` 过去在服务端完成生成后才由渲染器拒绝不支持的 `--output`，可能已经实扣点数却没有输出摘要。现在共用参数解析阶段先校验输出格式，错误格式直接报错并退出，不发起 HTTP 请求；同样覆盖可能触发生成的 `contract summary query`。
- 影响范围：CLI 参数解析、输出渲染、合同摘要命令、帮助和命令参考文档；将已有下载命令的格式校验复用同一规则。
- 关键决策：保留渲染器自身的格式校验作为防线；接受原有的默认值、`json`、`yaml` 和 `table`，不改变有效格式的行为。
- 验证结果：先新增 generate/query 的“错误格式不得发送 HTTP 请求”测试并确认两例在旧实现下失败，修复后通过；`make test` 和 `make release-check` 均通过。隔离配置下对新构建的 macOS arm64 二进制执行错误格式命令，退出码为 1、标准输出为空，日志提示格式错误。
- 未处理项或风险项：未再次调用真实计费接口，避免额外扣费；2026-10-14 的既有 40 点扣费流水 `TX_CLM_1190561584045359221` 不会由 CLI 代码修复自动冲回。
