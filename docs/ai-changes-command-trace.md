# 命令级 Trace ID 改造

## 2026-10-20 本地实现

- 需求：按既定方案让一次业务命令的多个接口共用 Trace ID，后端不改，并使用 dev 的 import plan 复验。
- TDD：先新增模拟 import plan（列头及两页规则行）测试，旧实现因同命令 ID 不同失败；上传 content 缺头、签名下载日志及安全错误缺 Trace 的用例也先确认失败。
- 实现：私有 context key 保存每次命令新建的 Trace；Do/DoStream 复用命令 ID，无上下文保留请求级兼容；每次显式发送继续生成独立 parent-id，不增加锁、SDK 或 Span 导出。
- 入口：仅业务命令建立作用域，分发前设置 commandContext，所有返回路径恢复原值；日志采用局部 logger，不修改全局 App logger，不给独立认证/更新通道加头。
- 错误：在既有脱敏完成后复用 TraceError；遍历普通包装与 Join 分支避免同 ID 重复，保留 errors.Is/As、取消语义和原有业务输出。
- 上传下载：附件 prepare/content/commit 共用 ID，保留凭据隔离和禁重定向；签名下载仅关联本地日志及安全错误，不注入 Trace 头。
- 验证：make test（含 Go 全量和 Node 测试）、go test -tags dev ./...、默认/dev 的 go vet、受影响三个模块的全量 race 均通过；独立审查未发现需修复问题，git diff --check 通过。
- 覆盖率：按默认/dev 两种构建合并语句块，CLI 80.23%、OpenPlatform 80.21%、tracecontext 86.84%；日志每行不重复 trace_id，目录/签名 URL 脱敏及取消错误链回归通过。

## 2026-10-20 安装及 dev 客户端复验

- 已安装本地版本 `1.9.2-beta.1-local.command-trace.dev`（基于 e7bb2bc，包含工作区改动），旧二进制已备份，未提交、推送或发布。
- 10:51:37.882 至 10:51:41.240（北京时间），在 dev 使用 trace-dev-app 对已有矩阵 1178332302485226101 执行一次只读 import plan；退出码 0，生成 needs_confirmation 的本地计划。
- 列头查询和规则行查询各一次 GET、均 HTTP 200；命令开始、两次请求及命令完成日志共用 `f8b74b5419e878a91236da64356ab7db`，客户端已确认从原先两个 ID 改为一个 ID。
- 测试计划 `2b12dc66a1994142e06b89367d4ff262` 已在本地取消，取消过程无业务请求；未执行 apply，未修改矩阵数据或后端配置。
- OpenObserve 精确窗口 UTC 02:51:30 至 02:52:00 已检索到同一 Trace 下两个 HTTP 200 的 open-platform Server Span：列头 `78967b5bdc6ef8f2`、规则行 `1797282028bb9126`，证明改造后两个请求在后端仍共用命令 ID。
- 本次查询观察到 open-platform 29 个、Higress 3 个 Span，共 32 个，其中 Server Span 为上述 2 个；不宣称已核对日志 MDC 或全部下游服务，CLI 本身仍不导出 Span。
- 默认 profile 保持 trace-dev；测试、覆盖率、备份、客户端及后端证据位于本机 `/tmp/contract-cli-command-trace-implementation-20261020/`，复验完成，未改后端。

## 2026-10-20 多接口部分失败模拟

- 需求：模拟一次命令内前一个接口成功、后一个接口异常，核对错误反馈及日志关联。
- 方法：使用临时运行器调用现有 `cli.App.Run`，注入仅返回本地响应的 HTTP Transport；列头查询返回 200，规则行查询返回 500，响应为 `{"code":50001,"msg":"simulated table_rows failure"}`。运行器沿用主程序的错误日志及退出码处理。
- 范围：运行器以 dev 标签构建，使用隔离 profile 和虚构 token；未调用已安装二进制，未访问真实 dev 后端，未改 CLI 功能代码、端点校验、TLS、认证配置或业务数据。
- 结果：11:37:20（北京时间）两次 GET 共用 `c90444bdff2bcf51a4404ee78d8d45b6`；各请求的 `traceparent` Trace ID 与 `X-Log-Id` 一致，parent-id 分别为 `b60c33d3cb90794e`、`2d62a7b8fe772a53`。
- 错误反馈：第二次请求 HTTP 500 后立即失败，保留 HTTPStatusError 错误链，最终错误含一次 Trace ID，进程退出码 1、stdout 为空；本次没有重试、没有 command completed 日志，也没有生成本地导入计划。
- 验证：核对请求顺序、成功/失败日志、请求头、错误链、退出码及隔离目录，12 项检查通过；虚构 token 未进入日志。临时源码已从仓库清理，运行器、完整日志、请求记录及 verification.json 位于 `/tmp/contract-cli-command-trace-fault-20261020-113712/`。本次为本地模拟，因此没有对应的 OpenObserve 后端记录。

## 2026-10-20 已安装 CLI 的真实 dev 错误请求

- 用户澄清需要直接执行 `contract-cli`，而非模拟运行器。当前命令未提供指定某次请求返回 500 的故障注入参数；不能将本地运行器结果称为已安装 CLI 的 dev 异常验证。
- 使用已安装 CLI、trace-dev-app 身份和无效矩阵 ID `0` 执行 `rule table import plan`。测试 profile 的 app token 已过期，先使用本地已授权 dev App 凭据重新登录，随后执行只读请求。
- 11:42:59 至 11:43:00，真实列头查询 HTTP 200，业务码 20003，错误为 `rule table does not exist`；最终命令退出码 1，错误携带 `b3cb9d03c8d837b989b4954c602bfc2e`。
- 这是第一个接口的业务失败，未进入规则行查询，不能用来证明“第一个成功、第二个接口失败”。HTTP 成功日志与命令失败日志可同时出现，需结合业务码判断。未执行 apply，未改业务数据。
- 日志及结果位于 `/tmp/contract-cli-real-error-20261020-114242/retry.stderr.log`、`retry.result.json`，未修改 CLI 功能代码。

## 2026-10-20 真实业务错误 JSON 输出验证

- 用户要求直接执行 `contract-cli`，验证能够读取的业务错误结果。使用只读命令 `rule table column-headers list --profile trace-dev-app --as app --product-id contract --group-id approve_matrix --table-id 0 --output json`，没有注入模拟响应。
- 11:53:38 至 11:53:39，后端返回 HTTP 200，stdout 为合法 JSON：`{"code":20003,"data":null,"msg":"rule table does not exist"}`，命令退出码 1；stderr 的请求与最终错误共用 `195217c642d05f41bc0a843db4f55cbc`。
- 输出路径区别：普通规则接口命令先输出后端业务 JSON，再将非零业务码转为命令错误；`import plan` 在内部读取/校验失败后提前返回，没有输出原始业务响应。不能笼统地说所有业务错误都没有 JSON 输出；尚未统一的是命令级错误的结构化输出。
- 未改 CLI 功能代码或业务数据。原始 JSON、stderr 和核对结果位于 `/tmp/contract-cli-business-error-20261020-115337/`。
