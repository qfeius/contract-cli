# 中断取消与结构化错误出口

## 2026-10-20 修复 MR !20 的取消后继续写入

- 复审发现自动更新请求取消后，auth logout、auth use、skills install 仍执行本地写入并返回 0；三条正式回归先复现失败，再在统一 Run 入口和更新检查后的分发边界检查命令 context，非业务与内部命令返回也保留取消错误链。
- 取消测试要求退出 130、stdout 无成功提示、完整配置字节不变，Skill 目标与登记文件均不创建；额外覆盖预取消的三个命令和内部同步、复用 App 不残留旧更新通知。
- 更新服务网络失败、返回超时错误、真实 1.5 秒子检查超时但父 context 仍有效时，原本的降级执行保持；没有修改更新策略、业务 Trace、锁、重试或已开始同步安装的事务/回滚语义。
- 默认/dev 全量 Go、Node、race、两种构建 vet、Linux/Windows 交叉编译通过，CLI 覆盖率 81.8%；原 review 的三个临时反例也全部转绿。macOS 临时候选使用仅监听本机的阻塞更新代理，三个真实进程在更新阶段接收 SIGINT 后均退出 130，配置、Skill 目录和登记均保持未改。未接触真实 Token、Skill 目录或业务写接口。
- 证据目录 `/tmp/contract-cli-cancel-dispatch-fix-20261020/`；验证通过后提交、推送同一 MR，并以更新后的实际 merge result 重新审查。只更新 MR，不执行合并或发布。

## 2026-10-20 创建 MR 前合入最新 master

- 用户授权推送当前分支并创建目标为 master 的 MR；fetch 发现 origin/master 已更新至 `ef9c980`，本轮合入并解决 9 个冲突文件，保留摘要、智审、矩阵原生 ID 和 Skill 自动同步的上游逻辑，默认构建仍仅支持 prod。
- 合并适配先补红例：review 纳入业务命令级 Trace，识别 stdin/dry-run/print-input/yes/enabled 布尔参数；新摘要和智审请求体继续沿 context 读取，智审 JSON/上传保留 2MiB/50MiB 上限及最多多读 1 字节，借用的 stdin 复制自有 fd 后取消，不关闭全局 stdin。
- Skill pending 提示在显式 JSON 时使用 JSONL，安装参数错误不混入重复文本摘要；默认提示和成功 stdout 保持，损坏登记的提示不丢失。没有新增锁、写入重试、真实智审写入或矩阵 apply。
- 合并后默认/dev 全量、Go/Node、vet、受影响模块 race、Linux/Windows 交叉编译及独立复核通过；CLI 语句覆盖率 81.7%。macOS 测试使用 canonical TMPDIR，避免 /var 与 /private/var 的路径断言和故障注入别名差异；Node 17 项通过、1 项按平台跳过。
- 临时候选 `1.9.5-local.trace-debug-interrupt.dev` 的真实 dev import plan 三种模式均退出 0、needs_confirmation，计划随后已取消；debug 文本/JSON Trace 分别为 `d929deff820626b973fed1fc4533e188`、`b0500b5823dd2cd2f6503a1d8fc1ca69`。登录等待发送 SIGINT 退出 130，并打印已有模拟 99.0.0 升级提示，profile 不变；JSON/文本 stdin 取消同样保留通知。
- 证据目录 `/tmp/contract-cli-mr-20261020/`。该候选未覆盖本机已安装的 `745a642` 版本；原中断前检查点与二进制备份保留。本轮只创建 MR，不执行合并或发布。

## 2026-10-20 补齐文本取消的升级提示

- 用户复现 `auth login --profile trace-dev-app --as user` 中断后缺少升级提示；模拟缓存只准备通知，旧 Execute 文本错误分支未显示它。
- TDD：先用真实 OAuth 回调等待取消链路、模拟 registration Transport 复现默认/debug 两个红例；修复仅在文本错误摘要后输出本次已有的非空 `notice.update.message`，没有新网络请求，也没有新增锁。
- 无新版或关闭自动检查时不追加提示；JSON 错误继续只输出 JSON，成功输出、登录参数、取消退出码 130 与原错误链保持。
- 同步 README 和命令参考说明；验证与安装证据保存到 `/tmp/contract-cli-text-update-notice-20261020/`，仍保留原回滚检查点。
- 验证：默认/dev 全量、Go/Node、race、vet 均通过，CLI 覆盖率 80.3%。候选版真实 dev `auth login --profile trace-dev-app --as user --no-open-browser` 到授权等待后发送 SIGINT，退出 130，stderr 先取消摘要、后单条模拟 99.0.0 升级提示；配置文件保持不变，显式 JSON 仍可解析且保留 notice。99.0.0 仅为用户授权设置的测试缓存，不代表真实发布版本。

## 2026-10-20 回滚检查点与 master 同步

- 用户授权先提交既有 dev、命令级 Trace、日志开关及验证文档；检查点提交为 `e2ab4ebb504ebd023bf7694fb5cc3478e06379df`，本地标记为 `codex/pre-interrupt-20261020`，不含本次统一中断处理。
- git fetch、ls-remote 与 merge 均已核对：origin/master 仍为 `e7bb2bc7d084dec4d9f5dcfb2f6785e66f68e1fd`，已被检查点包含，merge 返回 Already up to date，没有遗漏新的 master 提交，也没有制造空合并。
- 以后可要求“回滚到中断改造前的 codex/pre-interrupt-20261020”；本需求与该检查点分开保存，不推送或发布。

## 2026-10-20 实现与 TDD

- 用户明确选择只有显式 `--output json` 时错误输出 JSON；其他模式继续文本，成功 stdout 业务结构保持。JSON 错误按错误链给出类型、Trace、HTTP 状态和取消标志，附本次已有更新通知，错误渲染不发新网络请求。
- 主入口统一 SIGINT：首次取消并清理后退出 130，第二次恢复系统默认强退；Inspect 删除内部硬退出，沿 parent context 回收私有进程组，内部 inspection-helper 仍在 CLI 初始化前分发。
- 沿既有 resolveRawBody/resolveJSONObjectBody 调用链透传 context；Unix 由单个读取方拥有非阻塞 fd，Darwin 使用 Select、其他 Unix 使用 Poll，每次等待最多 100ms 并检查取消；退出前恢复原 O_NONBLOCK，仅关闭自己打开的 fd。其他平台保留 AfterFunc 关闭自有文件的取消方式，不关闭全局 stdin。
- import apply 保留原有状态保存、输出和锁范围；发送中取消保留 UncertainWriteError、uncertain 行，回读取消保留 unverified 行，返回取消链并停止后续写入。没有新增锁、重试或补偿机制。
- TDD 先确认导入取消被吞掉、真实进程缺取消 JSON 的红例；标准 FlagSet 文本诊断污染 JSON 也先失败，再统一交由 Execute 输出。覆盖值内的开关、无效 debug 参数、重复执行 notice 隔离、超时/HTTP/不确定写入分类，以及正常/取消 FIFO。
- macOS 半段 stdin 取消、原 stdin 标志恢复、空 writer 和空闲 writer 关闭后的正常 EOF 都补了红例后修复；Select 避开 [Go 已记录的 Darwin FIFO kqueue EOF 问题](https://github.com/golang/go/issues/24164)。Darwin 自有 fd 达到 FD_SETSIZE（1024）时明确报错，避免 fd_set 越界；Linux/Windows 仅完成交叉编译，未做实机验证。
- 验证：默认/dev 全量、Go/Node、vet、受影响模块 race 均通过；Darwin 真实进程 stdin/FIFO/探测回收及第二 SIGINT 通过，Linux/Windows 交叉编译通过。平台仅编译的结果不等同于实机运行。

## 2026-10-20 验证证据与交付

- 本地证据目录：`/tmp/contract-cli-interrupt-error-20261020/`，保留红绿日志、真实进程、交叉编译及实测记录。最终代码的 make test、dev 全量、两种构建 vet、受影响模块 race 全通过，CLI 单包覆盖率 80.3%；独立复审通过。
- 功能改动仅在 contract-cli，后端代码、配置和业务数据不改；测试中的写入取消仅使用本地 HTTP Transport 模拟。
- 候选版本 `1.9.2-beta.1-local.interrupt.dev`：真实进程读取半段 stdin 后 SIGINT，文本/JSON/debug JSON 三种模式均退出 130（本轮约 4–131ms）；显式 JSON 保留 Trace 和已有更新通知。更新通知使用合成缓存验证，不代表真实存在 99.0.0 新版本，也没有取消后联网检查。
- 2026-10-20 16:36 dev import plan：默认、debug、debug JSON 都成功且本地计划已取消，未执行 apply；默认 stderr 0 字节。两种 debug 的列头/规则行接口各自共享同一 Trace，分别为 `83ad395b4077f617c5d715f3471eae9a`、`4628f44348ec6ef2df2e3defb186ecb9`，OpenObserve 查到每条 Trace 的两个 open-platform server span 均 HTTP 200（仅核对 Trace，不声称核对 MDC 日志）。
- 交付时从本次功能提交重建并安装同一版本；保留原已安装二进制备份及 `codex/pre-interrupt-20261020`，中断功能与旧检查点分开提交，方便按检查点回滚。
