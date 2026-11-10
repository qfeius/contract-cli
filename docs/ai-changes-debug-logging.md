# CLI 调试日志开关与输出格式

## 2026-10-20 本地实现

- 需求：对齐 makecli 的默认安静行为，增加全局 `--debug`；显式 `--output json` 时调试记录也使用 JSON，保留命令级 Trace 传播，并用 dev import plan 复验。
- TDD：新增测试先确认默认模式仍打印日志、`--debug` 尚不识别；追加布尔简写及帮助用例先失败，再完成实现。浏览器打不开时的手动授权指引也先测后改。
- 实现：每个 App 使用稳定的 slog handler 代理，认证、网络 guard 和业务 Client 共享同一策略；策略通过原子指针切换并在每次 Run 后恢复，不增加显式锁。
- 参数：独立解析并移除 `--debug[=bool]`，不改写其他参数；跳过带值参数、支持单横线标准 flag 的值保护，遇 `--` 后停止识别；重复开关最后生效。
- 输出：默认关闭内部诊断；debug 使用现有安全字段，显式 JSON 模式为 JSONL，其他为文本；stdout 业务结构保持，必要错误摘要仍为 stderr 文本，Trace 传播独立于日志开关。
- 兼容：显式 `Options.Logger` 保留调用方 observer 控制；默认 CLI 不注入。浏览器打开失败时在原授权 URL 通道保留手动操作指引。
- 验证：make test（含 Go 全量和 Node）、go test -tags dev ./...、默认/dev 的 go vet、CLI/OpenPlatform/tracecontext 的 race 均通过；独立审查发现的单横线业务 ID 参数边界已按红绿测试修复，复核通过。CLI 默认/dev 合并语句覆盖率 80.46%，新增日志模块 98.21%。

## 2026-10-20 安装与 dev import plan 复验

- 已安装 `1.9.2-beta.1-local.command-trace.debug.dev`，路径为 `~/.local/bin/contract-cli`，基于 e7bb2bc 的本地未提交改动；与候选二进制 SHA-256 一致。旧版本备份在 `~/.local/share/contract-cli/backups/contract-cli-before-debug-logging-20261020T143715`。
- 使用既有 trace-dev-app 身份和矩阵 1178332302485226101，关闭自动更新检查，分别执行默认、文本 debug、JSON debug 三次只读 import plan；均退出 0，返回 needs_confirmation，本地计划均已取消，未执行 apply，默认 profile 保持 trace-dev。
- 默认模式（北京时间 14:37:32.402 至 14:37:35.519）：stderr 为 0 字节，stdout 保持业务 JSON；对应后端 Trace 为 `e89421cc7059e9efe297470777ac5184`。
- 文本模式（14:37:35.527 至 14:37:38.645）：命令前加 --debug，未显式指定输出格式，stderr 为文本；两个请求共用 `0d17c6b821b54206a02df34759cafb68`，均 HTTP 200。
- JSON 模式（14:37:38.655 至 14:37:41.711）：--debug --output json，stderr 14 条记录均为合法 JSON，两个请求共用 `f13c91d1cf4ed719f17f5bec46ae7095`，均 HTTP 200，stdout 没有增加 trace_id。
- OpenObserve 窗口 UTC 06:37:25 至 06:37:50 查到 6 个目标 open-platform Server Span，每个 ID 下恰有列头和规则行两个 HTTP 200 Span。静默模式未从本地采集 ID：按目标矩阵、三次调用的时间顺序，并排除两组日志直接记录的 debug ID，匹配唯一剩余组；不把该匹配说成已抓取静默请求头。未另行核对日志 MDC。
- 进程级失败验证确认默认只输出 error 文本摘要；debug JSON 的诊断记录可解析，末尾用户错误摘要仍为文本，退出码和错误中的 Trace ID 保留。auth app 登录成功也没有内部日志输出；未改后端代码或配置。
- 测试、安装、stdout/stderr、计划取消及后端证据位于 `/tmp/contract-cli-debug-logging-20261020/`，尚未提交、推送或发布。

### 本次 OpenObserve 链接

- [默认静默模式](https://openobserve.qtech.cn/web/traces/trace-details?org_identifier=default&stream=default&trace_id=e89421cc7059e9efe297470777ac5184&from=1792478245000000&to=1792478270000000)
- [文本调试模式](https://openobserve.qtech.cn/web/traces/trace-details?org_identifier=default&stream=default&trace_id=0d17c6b821b54206a02df34759cafb68&from=1792478245000000&to=1792478270000000)
- [JSON 调试模式](https://openobserve.qtech.cn/web/traces/trace-details?org_identifier=default&stream=default&trace_id=f13c91d1cf4ed719f17f5bec46ae7095&from=1792478245000000&to=1792478270000000)

## 2026-10-20 中断改造前的版本检查点

- 用户授权将当前 dev 支持、命令级 Trace 和日志开关改动先提交，作为尚未加入统一中断处理的可回滚版本；该次检查点的 make test 和 git diff --check 均通过。
- 随后同步远程 master，再单独实现中断及结构化错误；同步后的无中断版本也保留独立检查点，避免回滚时丢失 master 更新。提交号与检查点以 Git 记录为准。
