# 审批矩阵 CLI 集成技术方案

## 2026-09-14 配置闭环更新（当前实现）

三个项目均在 `20260901-zss-approval` 工作树补齐：符号/能力开关/循环函数、人员回读、部门与角色查询，列配置及行号/优先级回读、列 PATCH 与影响预检、精确小数、原有预发布和发布。

已按用户要求移除幂等回执表及相关接口，导入继续使用既有列头计划和行 CRUD；移除 --guarded，旧保护计划拒绝执行并要求核验后重建。NUMBER 已升级为整数 30 位、小数 8 位，显式 null 表示清空单元格。具体协议、兼容边界、上线顺序见 [配套接口与版本保护](../skills/contract-cli-rule/references/configuration-completion.md)。

2026-09-15 已按用户截图澄清：所需枚举为五种条件类型及对应运算符，不是业务字段选项；不再将业务枚举数据源列为缺口。BPM 的普通查询与列配置共用单元格类型映射，补齐集合类判空映射；CLI 内置稳定的 symbol 映射，查询命令本地输出并原样提交 symbol，列更新由开平接口做最终校验。真实 MySQL 并发锁和发布外部副作用仍需集成验收；本轮无需新增 SQL，未执行任何数据库变更。此批不扩展测试用例、系统 Excel、版本历史、审计、行列重新排序等页面能力。

### 移除回执能力后的验证

CLI 全量测试及矩阵专项 race 测试通过，包含已移除命令在 HTTP 前报错、旧保护计划拒绝降级写入的回归。BPM 使用本地缓存依赖重新编译主代码和目标测试，JUnit 72/72 通过；开平目标路由测试 50/50 和 spotlessCheck 通过。Skill 校验与三个项目 diff 检查通过。未操作数据库、未打包、未提交或部署。

### 移除回执能力前的验证记录（历史）

- CLI：`go test ./...` 通过；`go test -race ./internal/cli -run 'Matrix|Rule'` 通过。全量 race 检查仍发现既有 `TestLegacyMCPCommandNamesAreRejected` 并行共用 App，竞争写 `App.Run` 的 updateNotice；未扩展修改无关逻辑。
- BPM：离线 Maven 依赖解析未通过；使用本地缓存依赖重新 javac 编译本次主代码及目标测试，JUnit 75/75 通过。覆盖作用域、BigDecimal 精度、旧接口回归、回执重放、冲突阻断、待生效元数据、纯改名预检和 PATCH 保留循环策略；mock 数据库测试不代表已完成真实并发验收。
- 开平：离线 Gradle `GatewayFilterConfigurationTest` 53/53 通过，spotlessCheck 通过；覆盖 user/app 身份与精确字节透传。
- Skill：quick_validate.py 通过；使用 skill-creator 将新查询、版本保护和恢复流程加入原 Skill，保留旧命令兼容说明。
- 三个工作树 diff 检查通过。未提交、打包、部署或执行数据库迁移；没有写入测试/生产矩阵。

## 首轮代码对齐记录（历史，以下限制已由上节部分替代）

基于 `bpm-rule-configuration` 分支 `20260901-zss-approval@c77a6c6` 及新交互文档第 3 版，新增 9 个 HTTP 接口：规则组 get/create、矩阵 get/create/update/delete、列 add/update/delete。条件/结果更新共享 PUT，提供语义别名。

新增结构接口字段见 `skills/contract-cli-rule/references/structure-parameters.md`。api call 保持项目原有关闭状态；所有目标结构接口均通过结构化命令调用。

修正后端类型：人员/部门为正 int64 外部 ID，NUMBER 为 int32；新增列通过基准列与方向插入，再更新配置；已有值的类型变更先由服务端阻止。

导入计划升级 v2：列头摘要、应用/环境指纹、24 小时有效期；apply 前重读列头；请求前记录 running，进程退出后先核验；uncertain 立即停批；HTTP 401/403/429 停批，404/409/412 失效。增加 --rows、--batch-size 及 get/cancel，保留成功行跳过。

尚未实现完整矩阵/相关行版本锁、跨计划去重、运行中异步暂停、uncertain 自动核验闭环及发布确认令牌；这些交互由 Agent 查询、确认和分批调用承接。后端详情当前仅含基础列头，缺少操作符/关联元素/默认值的完整回读与原子 revision 条件写接口，不能把列头摘要等同于完整配置快照。未完成的 v1 计划需重建，已完成计划可读取原结果。

以下保留原始设计背景。

## 1. 目标与依据

本方案基于[《审批矩阵 Agent CLI 页面对齐方案》](https://ysi13ckdb9.feishu.cn/docx/CaMkdHRL5ovfOAxXE5ScHFtMnoe)和当前仓库实现，目标是在不改变既有 CLI 行为的前提下，让 Agent 能完成“读取已有矩阵结构 → 生成批量导入计划 → 用户确认 → 逐行创建或更新 → 返回可继续追问的结构化结果”。

开放平台依据：

- [审批矩阵接口目录](https://docs.qfei.cn/7646727m0)
- [查询规则表列头](https://docs.qfei.cn/375907146e0)
- [创建规则表行](https://docs.qfei.cn/375916617e0)
- [修改规则表行](https://docs.qfei.cn/375933532e0)

## 2. 当前能力与差距

项目已有 `rule table` 命令组，并已覆盖列表、列头、行列表、行详情、行搜索、行创建、行更新、行删除、预发布和发布 10 个开放平台原子能力。现有命令直接透传开放平台请求体，适合精确调用，但缺少：

- 按现有列头把“列名 → 值”转换成开放平台 `table_cells` 的本地映射；
- 在写入前集中校验列存在性、重名列和单元格类型；
- 多行创建/更新的计划、确认和逐行结果；
- 部分失败后可安全续跑的本地状态。

本次保留全部既有原子命令，只增加两个旁路子命令：

```text
contract-cli rule table import plan
contract-cli rule table import apply
```

命令名称延续 `rule table`；2026-09-14 增加 `approval-matrix` 别名，其中 `approval-matrix publish`（含 `approval-matrix table publish`）映射到现有 `rule table release` 处理函数。

## 3. 总体设计

```text
Agent 结构化 rows
       │
       ▼
import plan ──GET 列头──► 本地列映射/类型校验
       │                         │
       │ needs_input             │ needs_confirmation + plan_id
       ▼                         ▼
Agent 追问补齐                  用户确认
                                 │
                                 ▼
                           import apply
                                 │
                         串行 POST / PUT
                                 │
                  success / partial_success /
                     failed / needs_input
```

### 3.1 计划阶段

`import plan` 执行以下步骤：

1. 解析 `rows` 输入，不接受未知字段；
2. 未提供 `--table-id` 时调用列表接口并返回 `needs_input` 和候选矩阵；已经选定目标时调用一次列头接口；
3. 列键优先按列 ID 解析，其次按列名精确匹配；
4. 按列头的 `table_cell_content_type` 校验值；
5. 生成开放平台逐行请求，但不发起写请求；
6. 校验通过后，用随机 `plan_id` 保存本地计划，返回 `status=needs_confirmation`；
7. 校验失败时返回 `status=needs_input`、全部问题和当前可用列，不保存计划。

### 3.2 执行阶段

`import apply --plan-id ...` 把 `plan_id` 同时作为计划定位符和确认令牌：

- `create` 使用现有创建行路径；
- `update` 使用现有更新行路径，只提交计划中的单元格；
- 按输入顺序串行执行，避免并发放大开放平台限流；
- 每一行执行后立即原子落盘；
- 同一主机用计划级文件锁阻止两个 `apply` 进程同时执行；
- 再次执行同一计划时跳过成功行，仅重试明确失败行；
- 网络中断、5xx 等结果不确定的写入标为 `uncertain`，后续自动跳过并返回 `needs_input`，要求先查询矩阵确认，防止重复创建；
- 已整体成功的计划再次执行只返回原结果，不再发 HTTP 写请求。

开放平台行写接口存在频率限制。当前实现不做自动等待或自动重放；服务端明确拒绝的行进入 `failed`，可在修正原因后用同一计划重试。写入结果不确定时不重试。

## 4. 命令与数据契约

### 4.1 生成计划

```bash
contract-cli rule table import plan \
  --profile contract --as app \
  --product-id contract \
  --group-id approve_matrix \
  --table-id <table-id> \
  --input-file import.json
```

`import.json`：

```json
{
  "rows": [
    {
      "operation": "create",
      "cells": {
        "合同金额": 1000000,
        "采购类型": ["软件"],
        "审批人": ["7113921696628736004"]
      }
    },
    {
      "operation": "update",
      "row_id": "7113921696628736004",
      "cells": {
        "6113921696628736003": ["7113921696628736005"]
      }
    }
  ]
}
```

约束：

- `operation` 可选，默认 `create`；只允许 `create`、`update`；
- `--table-id` 未传时不写数据、不保存计划，并返回已有矩阵候选项供 Agent 追问；
- `update` 必须带 `row_id`；
- `cells` 的 key 可以是列名或列 ID；列名重名时必须改用列 ID；
- `STRING` 接收 JSON string；`NUMBER` 接收 JSON number；`BOOLEAN` 接收 JSON boolean；
- `COLLECTION`、`EMPLOYEE_COLLECTION`、`DEPARTMENT_COLLECTION`、`ROLE_COLLECTION` 接收 JSON array；人员值使用正整数外部 ID，部门值使用 `open_department_id`（`od-...`），角色值使用对应角色 ID，集合可用空数组清空；
- 显式 `null` 清空单元格，省略单元格保持不变。该语义由导入计划转换为行更新请求，并沿用开放平台的局部更新语义。

计划成功输出核心字段：

```json
{
  "status": "needs_confirmation",
  "plan_id": "<32位十六进制ID>",
  "summary": {
    "total": 2,
    "pending": 2,
    "succeeded": 0,
    "failed": 0,
    "uncertain": 0
  },
  "operations": []
}
```

### 4.2 执行计划

```bash
contract-cli rule table import apply \
  --profile contract --as app \
  --plan-id <plan-id>
```

输出状态：

| 状态 | 含义 | Agent 后续动作 |
| --- | --- | --- |
| `needs_confirmation` | 计划已生成、尚未写入 | 展示摘要并向用户确认 |
| `needs_input` | 计划校验失败，或存在结果不确定行 | 按 `issues` 追问，或先查询矩阵核对 |
| `success` | 全部行成功 | 可读取行或进入预发布 |
| `partial_success` | 部分成功、部分明确失败 | 展示失败行，修正外部原因后重试同一计划 |
| `failed` | 所有待执行行均明确失败 | 展示逐行错误，处理后重试 |

`plan` / `apply` 固定输出结构化状态，因此不支持 `--raw`。

## 5. 状态存储与幂等边界

计划保存在当前 CLI 配置目录的 `approval-matrix-plans/<plan-id>.json`：

- 目录权限为 `0700`，文件权限为 `0600`；
- 使用临时文件加同目录 rename 原子替换；
- 计划记录创建时实际使用的 profile，`apply` 显式传入其他 profile 时本地拒绝；
- 对创建行接口没有服务端幂等键的假设；本地仅通过“成功行不重放”降低重复写入风险；
- 同一主机的并发执行由计划锁保护；本地计划文件被复制、删除或跨机器执行时，不提供分布式幂等保证。

## 6. 兼容性设计

- 顶层命令分发只新增 `rule table import` 分支；
- 原有 `list/pre-release/release/column-headers/row` 分支顺序、参数和路径保持不变；
- 原有 `row create/update` 仍接受开放平台原始 JSON，不经过本地计划转换；
- 批量执行复用现有 profile、app 身份、token 刷新、通用 query、HTTP 错误和输出渲染组件；
- 未引入第三方依赖，`go.mod` 不变；
- 2026-09-14 已接入后端补齐的规则组、矩阵定义和列结构接口；系统模板导入、版本回滚、流程绑定继续在当前范围外。

## 7. 验证与验收映射

逐场景测试清单见 [审批矩阵 CLI 业务场景测试清单](approval-matrix-cli-test-scenarios.md)。

自动化回归覆盖：

- 列名和列 ID 映射；
- `NUMBER`、`EMPLOYEE_COLLECTION` 转换；
- 计划阶段只读列头、不写行；
- 类型不匹配和不存在列返回 `needs_input`，且不保存计划；
- user/app 双身份仅在规则模块开放；user 经 OrgV2 验签及合同规则管理员权限校验，不影响其他模块身份策略；
- 矩阵写操作的 `update_user`/编辑人由开平服务端从当前 Bearer 用户令牌解析；CLI 不将 `--user-id` 透传到规则接口，避免覆盖真实登录用户。需要当前个人留痕时使用 `--as user`，app 请求沿用系统身份审计；
- 创建成功、更新业务失败返回 `partial_success`；
- 重试跳过成功的创建行，只重试失败更新行；
- 完成计划再次执行不发写请求；
- 计划文件权限为 `0600`；
- 新帮助主题可离线渲染；
- 全仓既有测试继续通过。

人工验收按以下顺序执行：

1. `rule table list` 选择已有矩阵；
2. `column-headers list` 查看可写列；
3. `import plan` 生成计划并检查摘要；
4. 用户确认后执行 `import apply`；
5. 用 `row get/list/search` 核对结果；
6. 执行 `pre-release`，确认后执行 `release`。
