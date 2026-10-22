# app 命令开发指南

## 1. 目标

这份文档用于后续继续开发 `contract-cli` 的 app 身份业务命令，覆盖两部分工作：

- 功能开发：CLI handler、domain service、开放平台请求、测试。
- Skill 编写：`skills/` 下的模块文档、参数说明、命令示例和排障说明。

默认原则是：命令名保持业务语义不变，代码层根据当前身份 `user` / `app` 自动路由到底层不同接口。

示例：

```bash
contract-cli contract search --profile contract --as user ...
contract-cli contract search --profile contract --as app ...
```

同一个 `contract search` 命令，`user` 和 `app` 可以走不同后端路径。

## 2. 开发前必须确认的信息

每个新 app 命令开始前，先把下面信息补齐：

| 项目 | 必填 | 说明 |
| --- | --- | --- |
| CLI 命令名 | 是 | 例如 `contract submit <contract-id>`、`mdm vendor create` |
| user 是否已有实现 | 是 | 如果已有 user/MCP 命令，优先保持命令面不变 |
| app 接口方法 | 是 | `GET` / `POST` / `PUT` / `DELETE` |
| app 接口路径 | 是 | 必须是相对路径 `/open-apis/...` |
| query 参数 | 是 | 区分 CLI 显式 flag、通用 query、固定 query |
| body 参数 | 视接口 | 简单参数用 flag，复杂结构用 `--input-file` / `--data` |
| path 参数 | 视接口 | ID 类参数优先使用位置参数，并做 `url.PathEscape` |
| 身份限制 | 是 | `any`、`user_only`、`app_only` |
| 示例响应 | 是 | 至少覆盖成功响应，最好补一个业务失败响应 |
| 生产文档疑点 | 视接口 | 如果文档路径、方法、字段名互相矛盾，先和接口方确认 |

不要根据命令名猜接口路径。生产文档有疑点时，先与接口方确认契约，再用最小 service 测试验证。`api call --channel web` 仅支持 user 身份与 `/open-web/web/**` 页面接口，不能用于验证 app 身份的 `/open-apis/**` 接口。

## 3. 代码分层约定

### 3.1 CLI 层只做轻逻辑

`internal/cli` 只负责：

- 解析位置参数和 flags。
- 读取 `--input-file` / `--data`。
- 做本地必要校验，例如缺少 ID、缺少文件路径、JSON 非对象。
- 构造 request context。
- 调用 domain service。
- 交给统一 renderer 输出。

不要在 CLI handler 里直接写：

```go
http.NewRequest(...)
client.Do(...)
```

HTTP 细节必须放到 `internal/openplatform` 或 domain service。

### 3.2 domain service 负责身份路由

有 user/app 双身份的命令，在 domain service 中按 `requestContext.Identity` 分流。

推荐模式：

```go
switch requestContext.Identity {
case config.IdentityUser:
    return s.do(ctx, requestContext, "mcp-tool-name", replacements, query, body)
case config.IdentityApp:
    return s.client.Do(ctx, requestContext, openplatform.Request{
        Method:         http.MethodPost,
        Path:           "/open-apis/contract/v1/...",
        Query:          query,
        Body:           body,
        IdentityPolicy: openplatform.IdentityPolicyAny,
    })
default:
    return openplatform.Response{}, fmt.Errorf("unsupported identity %q for ...", requestContext.Identity)
}
```

如果命令只支持 app，例如合同提交、删除等操作，使用：

```go
IdentityPolicy: openplatform.IdentityPolicyAppOnly
```

如果命令仍然只支持 user/MCP，继续使用 MCP spec 的 `IdentityPolicyUserOnly`。

### 3.3 openplatform client 统一处理 HTTP

`internal/openplatform.Client` 负责：

- 拼接 `OpenPlatformBaseURL + Path`。
- 校验 `Path` 必须是 `/open-apis/...` 相对路径。
- 注入 `Authorization: Bearer <token>`。
- 注入 `Accept: application/json`。
- JSON body 默认 `Content-Type: application/json`。
- multipart / stream body 保留调用方设置的 `Content-Type`。
- 非 2xx 包装错误和响应摘要。
- 执行身份策略拦截，避免命令层散落判断。

业务命令不要自己拼 host，也不要直接使用绝对 URL。

## 4. 身份与路由规则

### 4.1 当前身份来源

命令身份解析优先级：

1. 显式 `--as user|app`
2. profile 的 `default_identity`
3. user-only 路径默认强制 user

app 命令使用 `tenant_access_token`，user 命令使用 OAuth token。

为兼容老用户脚本，旧身份值 `--as bot` 仍按 app 身份解析；新增文档、测试命名和示例统一使用 `app`。

### 4.2 配置与鉴权上下文

app 命令依赖 profile 中的开放平台配置：

- `OpenPlatformBaseURL`：业务 API 基址，prod 默认 `https://open.qfei.cn`。
- `AppTokenEndpoint`：app 获取 `tenant_access_token` 的接口，prod 默认 `https://open.qfei.cn/open-apis/auth/v3/tenant_access_token/internal`。
- `default_identity`：未传 `--as` 时的默认身份。
- `identities.app.token`：app 命令实际使用的 token。

user OAuth 与 app token 是两套不同流程：

- `auth login --as app` 使用 `appId/appSecret` 直接换 `tenant_access_token`。
- `auth login --as user` 使用 OAuth 授权码流程。
- 当前 user OAuth 不要求旧 Higress `resource`，resource 为空时授权 URL 和 token 请求都不发送 `resource` 参数。

开发 app 命令时不要复用 user OAuth 的 resource 概念，也不要把 `OpenPlatformBaseURL` 和 OAuth metadata URL 混用。

### 4.3 MCP 路径约束

`/open-apis/contract/v1/mcp/` 默认是 user-only。

除非明确已经为该命令做了 app 标准接口路由，否则 app 不应该继续走 MCP 路径。

user 侧已有 MCP 工具时，优先保留原有 `s.do(...)` 实现，app 侧新增标准开放平台路径。

### 4.4 通用 query 参数

结构化命令支持：

```bash
--user-id-type <type>
--user-id <id>
```

当前统一规则：

- 不传 `--user-id-type` 时，底层默认拼 `user_id_type=user_id`。
- 显式传 `--user-id-type employee_id` 时，覆盖默认值。
- `--user-id` 传了就拼 `user_id=<id>`，不传就不带。
- 不区分 `user` / `app`。
- 不做命令级必填校验，除非后续产品明确要求。

MCP user-only 请求的固定 query 优先级更高。即使用户传 `--user-id-type employee_id`，MCP spec 固定的 `user_id_type=user_id` 也不能被覆盖。

`api call --channel web` 使用重复的 `--query key=value` 显式提供业务 query，不自动补 `user_id_type` 或调用人参数；身份只来自 CLI 自身的 user 授权，不能以 query 或 header 覆盖可信身份。

### 4.5 日志与安全

新增边界函数要补安全日志：

- 命令入口：记录 profile、identity、命令关键参数。
- 请求开始：记录 method、path、identity。
- 请求失败：记录 method、path、status、错误摘要。
- 请求成功：记录 method、path、status。

严禁记录：

- `appSecret`
- `tenant_access_token`
- user access token
- refresh token
- 完整文件内容
- 大体积 JSON body

如果必须排查请求体，只输出字段名或截断后的摘要，并确认不包含密钥、token 或个人敏感信息。

## 5. 参数设计规则

### 5.1 JSON 请求体

复杂结构统一使用：

```bash
--input-file <json-file>
--data '<json-string>'
```

规则：

- 两者互斥。
- body 必须是 JSON object，除非接口明确接受数组或原始文本。
- `contract search` 这类命令可以把简单 flags 合并进 JSON body。
- `contract create`、`contract template instantiate` 等复杂接口优先要求完整 JSON。

不要再用 `--file` 表示 JSON 请求体。

### 5.2 文件上传

`--file` 只用于真实二进制文件上传。

例如：

```bash
contract-cli contract upload-file --as user --file ./合同.docx --file-type text
contract-cli contract upload-file --as app --file ./合同.docx --file-type text
```

上传命令必须：

- 校验文件存在。
- 校验是普通文件。
- 校验大小上限，目前是 `200MB`。
- 使用 streaming / `BodyReader`，不要一次性把大文件读入内存。
- 使用 `multipart/form-data`。

### 5.3 ID 与路径参数

ID 类参数优先使用位置参数：

```bash
contract-cli contract get <contract-id>
contract-cli mdm vendor get <vendor-id>
```

写入 path 前必须做：

```go
url.PathEscape(id)
```

如果生产文档同时把 ID 写在 path 和 query 中，先按保守方案双带，并在测试和文档里明确原因。

### 5.4 本地校验边界

CLI 只做稳定、不容易和后端漂移的校验：

- 必填位置参数。
- 必填文件路径。
- JSON 格式。
- 文件大小。
- `--input-file` 与 `--data` 互斥。

不要轻易在 CLI 强校验后端枚举、字段组合、业务权限，除非接口非常稳定且产品明确要求。

## 6. 输出规则

默认继续输出后端 JSON envelope。

统一支持：

```bash
--output json|yaml|table
--raw
```

当前不强制把 user/app 响应归一化。原因是 app 标准接口和 user MCP 接口响应结构可能不同，贸然归一化容易隐藏后端真实差异。

如果某个接口 HTTP 200 但 body 中 `code != 0`，当前多数命令仍按响应 body 输出，不在 client 层统一转 error。后续是否统一业务错误处理，需要单独决策。

## 7. 测试要求

坚持 TDD：先写失败测试，再做最小实现，再重构。

### 7.1 domain service 测试

每个新增 app 路由至少覆盖：

- `user` 仍命中原 MCP 路径。
- `app` 命中新开放平台标准路径。
- method 正确。
- path 参数转义正确。
- query 参数正确。
- body 正确。
- Authorization header 注入正确。
- 身份不支持时返回明确错误。

### 7.2 CLI 测试

每个命令至少覆盖：

- 显式 `--as app`。
- profile 默认身份为 app，未传 `--as`。
- 显式 `--as user` 仍走 user 路由。
- `--user-id-type` 默认 `user_id`。
- 显式 `--user-id-type employee_id` 覆盖默认值。
- `--user-id` 传入时被拼到 query。
- `--input-file` / `--data` 生效且互斥。
- 缺少位置参数或必填 flag 时返回 usage 或明确错误。
- `--raw` 和 `--output json` 可用。

### 7.3 openplatform client 回归

涉及通用能力时补 client 测试：

- `IdentityPolicyAppOnly` 在 user 身份下失败且不发 HTTP。
- `IdentityPolicyUserOnly` 保护 request 固定 query，不被 CommonQuery 覆盖。
- `IdentityPolicyAny` 保持 CommonQuery 可覆盖同名 request query。
- `BodyReader` 不被自动设置为 JSON。

### 7.4 文档契约测试

如果更新 `docs/cli-command-reference.md`，同步检查 `internal/cli/command_reference_doc_test.go` 是否需要补断言。

如果新增命令 help topic，补 `internal/cli/help_command_test.go`。

### 7.5 推荐回归命令

```bash
GOCACHE=/tmp/contract-cli-gocache go test ./internal/openplatform ./internal/openplatform/contract ./internal/cli
GOCACHE=/tmp/contract-cli-gocache go test ./...
```

## 8. Skill 编写规则

### 8.1 文件组织

继续按模块拆分：

```text
skills/contract-cli-shared/
skills/contract-cli-contract/
skills/contract-cli-mdm-vendor/
skills/contract-cli-mdm-legal/
skills/contract-cli-mdm-fields/
```

新增业务域时，新建独立 skill 目录，不要把所有内容塞到 shared。

`contract-cli-api-call` 仅保留传输参考，不发布独立 `SKILL.md`，不随公开 skills 安装；Open Web 请求配方应放在对应业务 Skill 中。

### 8.2 每个 skill 的基本结构

`SKILL.md` 头部必须有 front matter：

```md
---
name: contract-cli-xxx
version: 1.0.0
description: "..."
---
```

公开内置 skill（目录含 `SKILL.md` 并随 CLI 分发）还必须提供宿主元数据：

```text
<skill>/
├── SKILL.md
├── agents/openai.yaml
└── references/（可选）
```

`agents/openai.yaml` 用于 Codex/OpenAI 宿主的展示名称、简短描述、默认调用提示和隐式调用策略。它不承载 CLI 业务实现；禁用、未公开且不含 `SKILL.md` 的内部描述可以不提供该文件。

正文建议包含：

- 适用命令。
- 身份限制。
- user/app 路由差异。
- 关键参数规则。
- 常用示例。
- 什么时候转去读 shared 或其他提供已验证接口配方的业务 Skill。
- 排障要点。

### 8.3 references 结构

复杂模块不要把所有字段塞进 `SKILL.md`。

推荐结构：

```text
references/commands.md
references/<domain>-guide.md
references/<domain>-parameters.md
references/<domain>-fields.md
references/<domain>-enums.md
```

合同创建这类复杂接口继续使用：

- 场景配方。
- JSON Path 字段树。
- 枚举/取值附录。

### 8.4 skill 内容必须和实现同步

每次新增或调整 app 命令，至少检查：

- `skills/contract-cli-shared/SKILL.md`
- 对应模块 `SKILL.md`
- 对应模块 `references/commands.md`
- 参数说明文件
- `docs/cli-command-reference.md`
- `docs/cli-test-plan.md`

重点同步：

- 是否支持 app。
- app 路径。
- 是否仍支持 user。
- `--user-id-type` 默认 `user_id`。
- `--user-id` 是否只是透传。
- 请求体走 `--input-file` / `--data`。
- 文件上传走 `--file`。

### 8.5 skill 版本号

`contract-cli skills list` 面向用户展示的版本必须和 `contract-cli --version` 保持一致，统一使用当前 CLI 运行时版本。

`SKILL.md` 头部的 `version` 字段保留为 skill 文档自身的内部元数据：

- `skills list` 不再把该字段作为展示版本。
- `skills install` 会在安装到目标目录时，把 `SKILL.md` front matter 的 `version` 改写为当前 CLI 运行时版本。
- 新增 skill 可继续在仓库源文件中使用 `1.0.0` 作为内部文档版本；用户和 agent 在 `skills list` 及已安装文件中看到的版本由 CLI 发布版本决定。

## 9. 开发步骤模板

### 9.1 功能开发

1. 阅读生产接口文档，整理 method/path/query/body/response。
2. 如果命令已有 user 实现，确认命令名不变，只新增 app 路由。
3. 先写 domain service 测试，锁定 app method/path/query/body。
4. 写 CLI 测试，覆盖身份解析、默认 query、输入、输出。
5. 在 domain service 中按 `requestContext.Identity` 分流。
6. 在 CLI handler 中补必要 flag 和参数解析。
7. 更新 help registry。
8. 更新命令参考文档和测试计划。
9. 更新对应 skill 和 references。
10. 追加 `docs/ai-changes.md`。
11. 跑局部测试和 `go test ./...`。

### 9.2 Skill 编写

1. 先更新 shared 的能力清单和共享约束。
2. 更新模块 `SKILL.md` 的适用命令和身份说明。
3. 更新 `references/commands.md` 的 user/app 示例。
4. 如果新增复杂请求体，新增参数或字段说明文档。
5. 检查所有示例命令是否和 `--help` 一致。
6. 检查是否还保留“未实现”“user-only”等过期描述。

## 10. Definition of Done

一个 app 命令完成必须满足：

- 命令名与 user 侧保持一致，除非产品明确要求新命名。
- app 路由不走 MCP 路径，除非接口明确要求。
- `--as app` 和默认 app 身份都可用。
- user 侧旧行为不回归。
- `--user-id-type` 默认 `user_id`，显式传值可覆盖。
- `--user-id` 仅在传入时拼接。
- 路径、query、body、method 有测试覆盖。
- `--help` 有对应说明。
- `docs/cli-command-reference.md` 有命令说明。
- `docs/cli-test-plan.md` 有验收步骤。
- 对应 skill 文档已同步。
- `docs/ai-changes.md` 已追加变更记录。
- `go test ./...` 通过。

## 11. 需要提前对齐的补充项

下面这些点建议在继续大规模扩 app 命令前统一确认：

- skill 版本策略：继续独立版本，还是跟随 npm/CLI 版本。
- 业务错误策略：HTTP 200 但 `code != 0` 时，CLI 是否仍原样输出，还是统一返回 error exit code。
- `user_id` 必填策略：默认按通用 query 透传，不做命令级校验；已实测强依赖操作人上下文的 `mdm vendor create/update` 与 `mdm legal create/update` 会本地要求 `--user-id`。
- 输出归一化策略：user/MCP 和 app/open API 响应结构是否需要统一为领域 DTO。
- table 输出策略：哪些命令需要稳定 table，哪些只保证 JSON/raw。
- 文档来源策略：生产文档和实际接口不一致时，以接口实测、后端确认还是文档为准。
- 发版策略：app 命令开发完成后是否必须同步发布 beta 包和更新 skill 安装验证。
