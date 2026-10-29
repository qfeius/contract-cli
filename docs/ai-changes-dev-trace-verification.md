# 本地 dev 链路验证（历史记录）

## 记录归属与当前交付边界

本文保留 2026-10-19 至 2026-10-20 的本地只读验证证据，用于追溯 Trace 请求头、后端日志及 Span 的结论。完整原始文档和 dev 实现已保存在未推送的本地分支 `codex/local-dev-verification`（`6de2b57`）；该分支也包含当时的 Trace、debug 和取消实现。本文中的旧版本、profile、环境与测试结果均属于该历史验证范围。

当前 MR !20 已剥离 dev 支持，源码所有构建和正式安装包均仅支持 prod，误传 `-tags dev` 也不会放行 dev。本记录不提供当前源码开启 dev 的入口，也不将历史 dev 验证作为当前 MR 的环境能力。当前交付保留命令级 Trace、调试日志与取消修复，见[技术方案](command-trace-technical-design.md)。

## 2026-10-19 历史本地实现

- 历史需求：从 master 建立本地分支，支持使用 dev 验证现有 `traceparent` / `X-Log-Id` 链路。
- 历史方案：当时的本地验证分支通过构建标签增加 dev 预设；这部分环境实现已从当前 MR 移除。
- 当时环境约束：固定开发 API 与账号 HTTPS origin；继续拒绝 test/blue、尾点域名、额外端口、用户信息及混用环境的 profile / Device 凭据。
- 调用链：复用原有配置发现、认证、HTTP Transport 和业务客户端；Trace 生成、请求重试和业务行为不变，不新增锁。
- 当时登录：dev 通过浏览器 OAuth 授权码流程动态注册，不复用 prod Device Client ID。
- TDD：先补 dev 配置、只读请求及 Trace 头测试并确认失败，再增加最小实现；默认构建单独验证 dev 仍被阻断。
- 当时验证：默认/dev 构建的全量测试与 go vet 均通过；补测同环境保留登录、OAuth 退出及凭据不可读时停止重配；CLI 默认 / dev 合并语句覆盖率 80.05%，新增开发环境辅助逻辑合并覆盖率 100%。

## 本地安装与真实 dev 验证

- 当时分支：`codex/dev-trace-verification`，基于 master `e7bb2bc`；该节记录的是当时尚未提交或推送的本地状态。完整历史现由 `codex/local-dev-verification`（`6de2b57`）保留。
- 历史本地安装版本为 `1.9.2-beta.1-local.dev`；原 prod 二进制保存在用户本地备份目录。安装与构建命令仅保留在上述本地分支的完整历史中。
- 当时使用独立 dev profile `trace-dev` 完成配置及浏览器 OAuth 用户登录，不记录任何凭证；该配置流程不属于当前 MR。
- 2026-10-19 14:11:31 至 14:11:33（北京时间）完成一次只读 `contract search`（历史 profile `trace-dev`、user 身份、page-size=1、JSON 输出），退出码 0、业务 code=0，返回 1 条结果。
- CLI Trace ID：`6758775a7e738053d169d84e6a8b5702`。dev 的 open-platform 请求日志包含同一 traceparent；clm-openapi 日志 MDC 的 X-Log-Id 与该值一致。
- OpenObserve 已检索到同一 Trace ID 的 Span，覆盖 open-gateway、open-platform、clm-openapi、Higress、组织、鉴权、MDM 等服务；本次无需修改后端即可证明已测只读路径可关联。
- 验证范围：仅此只读合同查询成功路径；未据此宣称全部命令、异常、重试及异步场景均已完成验收。未修改后端和现有请求级 Trace 语义。

## 客户端不传 Trace 头的 dev 补充验证

- 2026-10-19 17:59:19.966 至 17:59:20.885（北京时间），直接 HTTPS 调用只读合同查询接口，携带既有 dev 用户认证，绕过 CLI 自动注入逻辑；未修改业务代码、网关或后端配置。
- 实际出站头仅含 Host、Accept-Encoding、Content-Length、Authorization、Accept、Content-Type、User-Agent；确认未发送 X-Log-Id、traceparent 及其他 Trace 关联头，认证值不记录。
- 请求：`POST /open-apis/contract/v1/mcp/contracts/search?user_id_type=user_id`，请求体 `{"page_size":1}`；HTTP 200、code=0、success=true，返回 1 条。
- 唯一 User-Agent 标记为 `contract-cli-no-trace-probe-2b96063edf7946f69bd3b3afe2f25b1c`；在 Higress 访问日志中定位同一请求，trace_id 为 `e2b5002926834398643f11e81d0d8c0b`。
- 已采集根 Span 位于 higress-gateway.higress-system（span_id=e3b58d4943f2c2be，无父 Span 引用）；后续 Higress、open-gateway、open-platform、clm-openapi 的入口 Span 父子关系连续，Trace ID 一致。
- open-platform 请求日志中的 traceparent 和 clm-openapi 日志 MDC 的 X-Log-Id 均对应此 Trace ID，证实该入口可自动建立 Trace 上下文并关联已测后端链路。
- 结论边界：MDC 的 X-Log-Id 不等于同名 HTTP 请求头；尚未证明网关另外注入 X-Log-Id HTTP 头。本次响应未回传 X-Log-Id 或 traceparent；不扩展到其他接口、环境及异常/异步场景。
- 首次探测漏带默认 user_id_type 参数，HTTP 200 但业务 code=110000，已补齐后复测成功；两次请求的 Trace ID 不同。脱敏摘要及链路证据保存在本机 `/tmp/contract-cli-no-trace-verification-20261019/`，不记录业务正文或凭据到仓库。

## 只传 X-Log-Id、不传 traceparent 的 dev 补充验证

- 2026-10-19 18:13:15.546 至 18:13:16.240（北京时间），沿用只读合同查询及 dev 用户认证，直接 HTTPS 请求；仅设置 X-Log-Id，确认未发 traceparent、tracestate、B3 等其他 Trace 上下文头。
- 传入 X-Log-Id：`54fe43a5fd5e56d10a81bd323dfbf187`；唯一 User-Agent：`contract-cli-x-log-id-only-probe-aca6dfd08190426981bc2b0bb9d601b3`。
- HTTP 200、code=0、success=true，返回 1 条；Higress 访问日志及根 Span 的 Trace ID 为新值 `cadc5d6b039b72b9f83592e30a41f908`，根 Span 无父引用。
- Higress → open-gateway → open-platform → clm-openapi 的核心 Span 父子关系连续，全部使用新 Trace ID；同一时间窗按两个 ID 查询 Trace，仅新 ID 检索到 Span。
- open-platform 请求头日志保留原 `x-log-id`，同时已有包含新 Trace ID 的 traceparent；clm-openapi 同一条请求日志里，HTTP 头 `x-log-id=54fe43a5fd5e56d10a81bd323dfbf187` 与 MDC `X-Log-Id:cadc5d6b039b72b9f83592e30a41f908` 并存。
- 结论：此已测 dev 入口未将单独传入的 X-Log-Id 用作 Trace ID；原 HTTP 头继续透传，链路与后端 MDC 使用网关新建的 Trace ID，两种 ID 分离。CLI 应继续同时设置一致的 traceparent 和 X-Log-Id，以实现命令级关联。
- 本次响应未回传两个关联头；未修改业务代码、网关或后端配置。脱敏摘要与证据位于本机 `/tmp/contract-cli-x-log-id-only-verification-20261019/`，结论不扩展至其他入口、环境或请求头冲突场景。

## 只传 traceparent、不传 X-Log-Id 的 dev 补充验证

- 2026-10-19 19:00:28.837 至 19:00:29.444（北京时间），直接 HTTPS 调用同一只读合同查询接口；仅发送 `traceparent: 00-c25f5dcaa9b792bab994a0c24de53479-c1aa0885814bced8-01`，实际出站无 X-Log-Id 及其他 Trace 关联头。
- HTTP 200、code=0、success=true，返回 1 条；唯一 User-Agent 为 `contract-cli-traceparent-only-probe-1f191555dac840c292e9ea62bf1f4e49`，Higress 访问日志中的 trace_id 与传入的 `c25f5dcaa9b792bab994a0c24de53479` 一致。
- Higress 首个已观测 Span 为 `19dcc7feec0d81f6`，父 Span ID 恰为传入的 `c1aa0885814bced8`；至 open-gateway、open-platform、clm-openapi 的 9 个核心 Span 父子关系连续、Trace ID 不变。该网关 Span 具有远端父引用，不称为完整 Trace 的根 Span。
- open-platform 与 clm-openapi 请求日志的 traceparent 均包含相同 Trace ID；clm-openapi 请求日志及另外 8 条日志的 MDC `X-Log-Id` 均为传入的 Trace ID。
- 结论：此已测 dev 路径仅传 traceparent 即可继承 Trace 并与日志 MDC 关联，客户端无需另发 X-Log-Id 来获得本次关联效果；CLI 同时发送两个同 ID 的头仍作为现有兼容行为保留，不据此扩展到所有消费者。
- 边界：两个服务已检索请求日志未见 x-log-id 头名，不等于证明线上绝对没有该 HTTP 头，也未定位具体 MDC 转换组件；本次响应未回传两个关联头。未修改业务代码、网关或后端配置。
- 首轮 18:56:32 的 Trace ID 为 `5dc9e0f798d7eb626f7e757a7abd7251`，已确认网关及下游继承 Trace 和远端 parent-id，但对应入口及 clm-openapi 日志未检索到，原因未确认；上述日志结论取自第二轮完整证据。
- 本机脱敏摘要及证据位于 `/tmp/contract-cli-traceparent-only-verification-20261019/`；认证值与业务正文不写入仓库。

## traceparent 与 X-Log-Id 不一致的 dev 补充验证

- 2026-10-19 19:07:00.002 至 19:07:00.694（北京时间），直接 HTTPS 调用同一只读合同查询接口，同时发送两个不同 ID；实际出站头已审计，未携带其他 Trace 上下文头。
- 传入 `traceparent: 00-f041a5d8d2a755ef859be387bb517b5f-780f87a47f5e656a-01` 与 `X-Log-Id: 7819c0b5419bf8ee2fc86040f9fca0a3`；唯一 User-Agent 为 `contract-cli-conflicting-trace-probe-cc909115eac244eabde260db9c7d9f6b`。
- HTTP 200、code=0、success=true，返回 1 条；Higress 访问日志的 Trace ID 为 traceparent 中的 `f041a5d8d2a755ef859be387bb517b5f`。
- Higress 首个已观测 Span `d555d22de6c40811` 的父 ID 为传入的 `780f87a47f5e656a`；至 open-gateway、open-platform、clm-openapi 的 9 个核心 Span 父子关系连续，全部使用 traceparent 中的 Trace ID。
- open-platform 与 clm-openapi 请求头日志均保留原 `x-log-id=7819c0b5419bf8ee2fc86040f9fca0a3`；clm-openapi 同一条请求日志中的 MDC `X-Log-Id:f041a5d8d2a755ef859be387bb517b5f` 则与 traceparent 一致，另外 8 条日志的 MDC 也一致。
- 结论：此已测 dev 路径的 Trace 与日志 MDC 以 traceparent 中的 Trace ID 为准，冲突的 X-Log-Id HTTP 头继续透传，不覆盖实际 Trace ID；不能把 HTTP 头值与同名 MDC 值视为始终相同。CLI 应保持两个头的 Trace ID 一致。
- 本次响应未回传两个关联头；未修改业务代码、网关或后端配置。脱敏摘要及证据位于本机 `/tmp/contract-cli-conflicting-trace-headers-verification-20261019/`，结论不扩展至其他入口、环境或无效 traceparent 场景。

## 2026-10-20 import plan 多接口验证的授权阻塞

- 当时目标：使用本地历史 dev CLI 执行一次只读 `rule table import plan`，比较列头查询与规则行查询的实际 Trace ID；当时尚未实施命令级 Trace 改造。
- 已确认本地版本仍为 `1.9.2-beta.1-local.dev`，commit `e7bb2bc`，分支为 `codex/dev-trace-verification`。
- 09:36:10 至 09:36:11（北京时间）先执行只读 `rule table list`（历史 profile `trace-dev`、user 身份、contract/approve_matrix、page-size=10、JSON 输出），在发现可读矩阵阶段返回 HTTP 403、退出码 1。
- 后端错误明确为当前 OAuth 用户令牌不允许访问 `/open-apis/rule_engine/v1/...`；此次请求 Trace ID 为 `23dae4d837aa40d67792dc2f5ab71da4`，不能将其当作 import plan 的执行证据。
- 本地 trace-dev 当时只有 OAuth 授权码身份，App ID/Secret/Token 及 Device Client 均未配置，CLI 默认识别的 App 凭据环境变量也未设置；后续已按用户提供的自定义变量名完成 dev App 登录，阻塞在下节解除。
- 该阶段尚未执行目标 import plan，也未取得同一命令多接口的实际 ID 对比结果；仅静态代码确认列头与分页读行分别调用 Client.Do，不能替代当时的 dev 实测结论。
- 未执行 apply、发布或业务写操作。脱敏阻塞摘要位于本机 `/tmp/contract-cli-import-plan-trace-verification-20261020/blocked.summary.json`。

## 2026-10-20 import plan 多接口成功复测

- 按用户提供的变量名在本地读取 `clm_dev_app_id` / `clm_dev_app_secret`，仅在子进程中映射给 CLI 的标准 App 凭据变量，未输出任何凭据值；建立独立 dev App profile `trace-dev-app`，保留原 trace-dev 用户配置并恢复其默认选中状态。
- 09:42:34.652 至 09:42:38.054（北京时间），使用未改 Trace 逻辑的 `1.9.2-beta.1-local.dev`（commit e7bb2bc）执行一次只读 `rule table import plan`（历史 profile `trace-dev-app`、app 身份、contract/approve_matrix、矩阵 `1178332302485226101`、本地 JSON 测试输入）。
- 使用已有矩阵及一个空值单元格作为纯本地计划输入；命令退出码 0，生成 `status=needs_confirmation` 的计划，未向矩阵写数据。列头读取和规则行读取均返回 HTTP 200。
- 同一次命令的列头查询 `/table_columns/column_headers` 使用 Trace ID `db99f5f3526ba88dccdac87a437b22e4`，规则行查询 `/table_rows` 使用 `bbcce9bd4c6c20a407ee8d951d1d3bb8`；两个业务请求对应两个不同 ID，发现矩阵和预读列头的独立命令不计入此次请求数。
- 客户端日志证明该历史版本仍按每次 Client.Do 生成请求级 Trace ID，当时尚未实施命令级共用 ID。首次 OpenObserve 精确查询 UTC 01:42:30 至 01:42:50 的 default/open_platform 日志及入口 Span 时均未命中；下节后续复查已在同一窗口查到两个 ID 的后端 Server Span。
- 测试计划 `713bbab07bdb336d4af0eba89faf5ab1` 已用本地 import cancel 标记为 cancelled；取消过程业务请求数为 0，未执行 apply、发布或其他业务写操作。
- 第一张候选矩阵的两个接口虽均为 HTTP 200，但现有行缺少单元格类型，导致本地快照校验失败；未改代码或业务数据，换用数据结构完整的既有矩阵取得上述成功复测，不将失败样本当作计划生成成功证据。
- 客户端安全摘要、清理结果和独立后端证据保存在本机 `/tmp/contract-cli-import-plan-trace-verification-20261020/`；原始响应及认证日志仅存本地受限文件，不写入仓库。

## 2026-10-20 OpenObserve 复查确认及未命中原因

- 重新查询原 UTC 01:42:30 至 01:42:50（北京时间 09:42:30 至 09:42:50）窗口，去掉服务过滤及恢复原三个入口服务过滤均查到两个请求；扩大至 UTC 01:40 至 01:52 得到相同数量，排除时间换算、窗口和该服务过滤差异导致本次结果变化。
- 列头 Trace `db99f5f3526ba88dccdac87a437b22e4` 在 open-platform 有 14 个 Span；Server Span 操作为 `GET fetch-rule-table-column-headers`，路径与目标矩阵的 `/table_columns/column_headers` 一致。
- 规则行 Trace `bbcce9bd4c6c20a407ee8d951d1d3bb8` 在 open-platform 有 15 个 Span；Server Span 操作为 `GET retrieve-list-of-rule-table-rows`，路径与目标矩阵的 `/table_rows?page_size=100` 一致，确认两个 CLI ID 分别传到后端、未合并。
- 两个 Server Span 的事件时间分别为北京时间 09:42:37.083699、09:42:38.232638；记录中的 `_o2_ingest_ts` 分别为 09:45:18.083396、09:45:18.083578，时间戳差为 160.999697 秒、159.850940 秒。
- 同一查询由 0 变为 29 条，证实数据查询可见性发生变化；结合摄取处理时间及此前查询时序，支持首次查询偏早的解释。原精确 queried_at 未保存，不能还原首次可查询的精确时刻，也未定位 SDK、Collector、平台摄取或存储中的具体延迟环节。
- [OpenObserve 官方源码](https://github.com/openobserve/openobserve/blob/a70b3830163a41283138b43b3811bb451b9df8ea/src/core/src/traces/mod.rs#L198-L203)将该字段设置为摄取处理时的 now_micros，部分路径可再次赋值；不将它等同于持久化完成或首次可查询时刻，未核实该源码版本与部署版本一致，也未单独校准跨服务时钟。
- 本次后端确认依据是 Server Span；open_platform 日志同时匹配 body/body_str 仍未命中，default 日志扩大窗口查询超时，均不用于断言 HTTP 头或日志 MDC 未透传。复查未发业务请求、未改任何配置。
- 脱敏复查证据保存为本机 `/tmp/contract-cli-import-plan-trace-verification-20261020/recheck-backend-evidence.json`，保留此前未命中记录作为历史，不覆盖或伪装原查询结果。

## 当时的后续命令级 Trace 方案

- 已整理[命令级 Trace 修改计划与技术方案](command-trace-technical-design.md)，包含两阶段状态、上下文生命周期、上传下载、错误脱敏、后端兼容、测试验收及回退范围。
- 该节编写时命令级 ID 复用尚未实施；当时计划使用每次 App.Run 的上下文管理 ID，保留每次 CLI 显式发送尝试独立 parent-id 及底层 Client 的请求级兼容处理，维持底层自动重定向和透明重试行为。当前 MR 已实现命令级 Trace，最新交付边界以技术方案和对应变更记录为准。
- 已按最新范围以当前用户身份创建[飞书技术方案](https://ysi13ckdb9.feishu.cn/docx/ExrBdFJLGoFNTjxIeBCcsMKan7e)，明确本次仅修改 contract-cli，不安排后端改造；已回读核对范围、验证基线、实施计划、测试验收和回退章节。
- 已在飞书方案开头新增“改造后的数据流”画板，展示命令 T1 的创建与复用、显式发送尝试 Sn 的生成、请求头注入及后端日志 / Span 关联；蓝色为 CLI 改造，灰色为现有后端。已回读确认画板源码及 17 个节点，导出 SVG，并对本地 PNG 做视觉检查；图源保存在 `docs/diagrams/command-trace-data-flow.mmd`，本地方案同步补充流程图。
- 已按用户要求在[飞书方案的请求头组合验证对比](https://ysi13ckdb9.feishu.cn/docx/ExrBdFJLGoFNTjxIeBCcsMKan7e#doxcnbmTotG17cJD45r7DLp9cSf)新增五种组合的对比表、实际时间与 ID 明细及结论边界，并同步本地技术方案；回读确认版本 9，两个原生表格分别为 6 行 × 6 列和 6 行 × 4 列（均含表头），原有内容完整保留，命令级方案及不改后端的范围不变。同值组 HTTP 头补充回查的脱敏证据保存在本机 `/tmp/contract-cli-feishu-header-comparison-20261019/same-id-header-evidence.json`。
- 2026-10-20 已按用户要求新增[飞书方案的 import plan 多接口验证与 OpenObserve 时延](https://ysi13ckdb9.feishu.cn/docx/ExrBdFJLGoFNTjxIeBCcsMKan7e#doxcn7SGG3OoysRFY9o3p4dpiR4)，同步本地技术方案；回读确认版本 10、两张原生表格分别为 3 行 × 5 列与 3 行 × 4 列（含表头），两个 ID、14/15 个 Span、精确时间及约 160 秒的边界均正确，原有内容完整保留，未改变仅修改 CLI 的范围。更新与核对证据位于本机 `/tmp/contract-cli-import-plan-trace-verification-20261020/feishu-import-plan-update/`。
