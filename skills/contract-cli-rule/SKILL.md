---
name: contract-cli-rule
description: "contract-cli 审批矩阵技能：用 user/app 双身份查询、新建、编辑、导入、复制规则行和发布矩阵，并识别完整矩阵复制的能力边界；支持 rule 和 approval-matrix 命令入口，以及来自对话、文档、表格或 Excel 的规则维护、审批路由和协商自动邀请场景。"
---

# contract-cli Rule

接口前缀：`/open-apis/rule_engine/v1`。

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)，沿用其中的 profile、环境、身份、授权和不确定结果处理约定。矩阵操作只使用当前 CLI 已开放的结构化命令，不回退 `api call`。

## 适用命令

- `contract-cli rule group get`
- `contract-cli rule employee search`
- `contract-cli rule employee batch-get`
- `contract-cli rule department search|batch-get`
- `contract-cli rule role search|batch-get`
- `contract-cli rule symbol query`
- `contract-cli rule loop-function query`
- `contract-cli rule table get|create|update|delete`
- `contract-cli rule table column add|update|update-condition|update-result|delete`
- `contract-cli rule table column patch|preview`
- `contract-cli rule table list`
- `contract-cli rule table pre-release`
- `contract-cli rule table release`
- `contract-cli rule table column-headers list`
- `contract-cli rule table row create`
- `contract-cli rule table row get <row-id>`
- `contract-cli rule table row list`
- `contract-cli rule table row search`
- `contract-cli rule table row update <row-id>`
- `contract-cli rule table row delete <row-id>`
- `contract-cli rule table import plan`
- `contract-cli rule table import apply`
- `contract-cli rule table import get|pause|resume|verify|cancel`

文档中的 `approval-matrix group/table/column/row/import/publish` 已映射到上述命令；其中 `approval-matrix publish` 和 `approval-matrix table publish` 都映射为 `rule table release`，帮助显示实际 `rule` 路径。新增结构接口的字段和操作顺序先读 [references/structure-parameters.md](references/structure-parameters.md)。

## 快速决策

- 新建矩阵或批量导入且用途不明确时，先确认用于审批路由、协商自动邀请还是仅维护或测试；已有矩阵的普通维护不重复追问用途。资料只代表业务数据来源，不视为完整矩阵结构定义。
- 用户只有创建或导入意图而没有规则内容时，引导其直接说明条件、结果和备注；不强制要求系统模板。用户已提供对话文本、文档、表格或 Excel 时直接解析，只追问阻断项。
- 用途是协商自动邀请时，先读取矩阵和列头，确保存在 `合同类型` 条件列且 `value_type=COLLECTION`。缺列或类型不符时，先展示结构差异并单独确认新增或修改；完成后重新读取列头，再生成导入计划。不得因为 Excel 单元格显示为文本就配置成 STRING；操作符和行值仍按已确认的匹配语义从 `rule symbol query` 结果中选择，不猜测 `value_*` 绑定信息。
- 新建矩阵时省略 `rule_table_id`，由服务端生成；不向用户索取或按名称推导 Code。普通创建确认和成功回复不主动展示；定位歧义、排障、复制部分完成或用户明确查询时，可展示返回的矩阵 ID。`product_id=contract` 的 `group_id` 固定为 `approve_matrix`；其他产品的目标范围无法确定时先澄清。
- 先确认规则表：`rule table list --product-id --group-id`
- 主流程与页面一致：新矩阵 `table create` → `table get` → `row list --page-size 10` → 配置条件列/结果列 → 重新读取列头 → 写入规则行并回读。创建接口会生成一条后端默认空白行；首条规则优先使用 `row update <blank-row-id>` 填充该行，只有追加额外规则时才使用 `row create`。已有矩阵从 get/row list 开始，不重复创建。接口映射见 [结构参数](references/structure-parameters.md)。
- 通过 `table get` 或 `column-headers list` 获取列 ID 和现有列配置。已有列若返回 `value_id/value_code/value_name/value_type`，更新时原样复用；新增或未绑定列按名称、类型和操作符配置并省略 `value_*`，不猜测业务元素 ID。
- 已有当前租户已确认且可选的人员外部 ID 时直接用于规则行；只有姓名或候选不明确时才用 `rule employee search`，读取 [人员搜索参数](references/employee-search-parameters.md)。不要为了跳过搜索而猜 ID，也不要直接要求用户手工找 ID。
- 查列头：`rule table column-headers list --product-id --group-id --table-id`
- 配置条件列时使用 CLI 内置的 `symbol` 映射；`rule symbol query` 仅输出本地候选，不访问 `symbols/query`。部门/角色先搜索并核对候选，已有 ID 用 batch-get 回读名称。参见 [配套查询及版本保护](references/configuration-completion.md)。
- 新增或修改规则行：`row create|update --input-file row.json`
- 用户一次给出多行：先 `import plan --input-file import.json`，展示摘要并确认后再 `import apply --plan-id ...`。
- 向新建矩阵批量写入或复制规则时，先确认唯一的默认空白行；导入计划第一条使用带该 `row_id` 的 `update`，其余才用 `create`。不能唯一确认空白行时停止生成计划，避免残留空白行。
- 按条件分页查规则行：`row search --page-size 10 --input-file row-search.json`
- 删除行：先 `row get` 展示行摘要，用户确认后再执行 `row delete`
- 完整复制矩阵目前无法保证一致：`table get`、`column-headers list` 和 `row list` 无法完整回读列绑定、操作符、多选和循环配置、结果语义及默认值。用户要求“复制矩阵”“其余一模一样”时，先说明缺口；不以可见字段一致宣称完整复制，也不在用户接受替代方案前创建目标矩阵。用户明确接受“按可见信息重建”后，补齐并确认缺失配置，再按 [交互与组合操作契约](references/interaction-contract.md) 的矩阵编排步骤执行，结果称为重建。复制规则行仍可按该文档执行。
- 迁移等其他没有一键命令的目标，先判断能否组合现有结构化命令；按 [交互与组合操作契约](references/interaction-contract.md) 分步确认、核验并报告已完成部分。
- 发布使用原有 `pre-release/release`。预发布前读取全部分页规则行保留基准；正式发布前再次全量读取，按行 ID、列 ID 比较内容，忽略返回顺序并保留 ID、金额精度。发现新增、删除或值变化时停止发布，重新预发布并确认；查询失败、分页不完整也停止。基准绑定环境、身份、矩阵 ID 和预发布版本。仅核对规则行，不提供原子并发保护。本地基准因 profile 重建等原因丢失、但服务端详情为 `status=0` 且有 `prepared_version` 时，再调用 `pre-release` 会双读版本和规则行并只恢复本地基准；看到 `baseline_recovered=true` 后重新展示版本、行数并确认，再调用 `release`。恢复过程禁止通过原值重写、清空或新增规则行改变服务端状态。

### 豆包首条规则处理（固定）

创建矩阵后，豆包必须先执行 `row list --page-size 10`。当返回唯一一条后端默认空白行且当前要写入首条规则时，固定调用 `row update <blank-row-id>` 填充该行；`row create` 仅用于追加第二条及后续规则。写入后再执行 `row list --page-size 10` 核验，确保矩阵中没有遗留的默认空白行。

## 关键规则

- 全部命令支持 `--as user` / `--as app`；不传时使用 profile 默认身份。user 当前用于 `--product-id contract`，需具备合同规则管理权限，失败时禁止自动切换 app。用户登录复用共享 Skill 的授权流程。
- 矩阵写操作的编辑人和审计人始终取当前登录身份：`--as user` 时由开平服务端从 Bearer 用户令牌解析当前用户 ID，并结合 `X-Qfei-Identity: user` 完成鉴权。规则命令不会把 `--user-id` 当作编辑人覆盖值；该参数在矩阵路径会被移除。`--as app` 仍按应用/系统身份记录，要求以当前用户留痕时使用 `--as user`。
- 公共定位参数：`--product-id`、`--group-id`
- contract 产品页面只展示 `approve_matrix` 规则组；不调用 `rule group create`，新建矩阵必须使用 `--group-id approve_matrix`。已存在的其他组仅在排查时按已知 ID 查询，不继续创建矩阵。
- 涉及单表时还需要 `--table-id`；列操作使用表详情或列头查询返回的列 ID。
- `row create`、`row search`、`row update` 必须传 JSON 请求体
- `row list` 必须显式传入 `--page-size`，例如 `--page-size 10`；包括首次读取、后续分页及写后核验。服务端实测约束见 [规则行分页参数](references/row-list-parameters.md)。
- `row search` 支持 `--page-size` / `--page-token` 作为 query 参数，请求体保持筛选条件 JSON
- `pre-release`、`release` 支持可选 JSON body；当前自动化样例不发送请求体
- GET/DELETE 类命令不接受 `--input-file` / `--data`
- `import plan` 读取列头和全部分页规则行，校验类型、update 目标行及现有行加计划新增行不超过 2000；`import apply` 才逐行调用既有创建/更新接口。
- 生成 `import plan` 前必须已确认业务用途并完成必需列检查。协商自动邀请缺少 `合同类型` 的 COLLECTION 条件列时，不得用现有资料直接生成计划或写入规则行。
- 批量计划输入中的人员、部门和角色集合必须已经解析成对应 ID 数组
- 批量执行只跳过已回读验证的成功行；写入响应成功但回读失败时先 `import verify`，结果不确定时用 row get/list/search 核验，不自动重试写请求。
- 所有新增、更新、删除、结构变更及发布先展示目标、变更内容和影响，用户确认后再调用。结构变更单独确认，完成后重新读列头再生成导入计划。
- 矩阵或列名称匹配多个候选时展示 ID 和摘要；不凭姓名、行号或近似列名猜测目标。
- 更新矩阵时 name 必填、rule_table_id 不变；保留描述需要把当前 description 一起提交。
- 新增列先 add（base_table_column_id + direction），取得新列 ID 后再 update-condition/update-result；新增类型与基准列一致。
- 任何列结构写入前先按完整目标方案计算容量：规则表总列数上限为 12，优先级列和备注列固定占 2 列，因此条件列与结果列合计最多 10 列。必须满足 `目标条件列数 + 目标结果列数 + 2 <= 12`；例如 10 个条件列加 2～3 个结果列会得到 14～15 列，应在首个 add 前说明超限并停止。直接调用 `column add` 时 CLI 也会读取当前列头，在总数达到 12 后本地阻断写入。
- 修改已有列不调用 `column preview`：先读取当前配置和规则行，展示变更并确认，再调用已有 update-condition/update-result 更新接口，回读核验。PUT 需要带齐已确认的配置字段，避免遗漏覆盖；缺少保留字段时先补齐，不猜测。类型或运算符变更仍由服务端校验，不自动清空数据。
- 有值的列改变类型或导致单元格类型变化的操作符时，服务端阻止更新。先展示影响，另行确认清空，再配置列；确认改类型不等于自动授权清空。
- 人员集合使用正整数外部 ID，批量输入接受数字或十进制字符串数组，发往后端为整数数组；部门集合统一使用 `open_department_id` 字符串（形如 `od-...`），不得把目录返回的数字 `department_id` 直接写入规则行。角色仍为字符串数组。
- 部门搜索结果直接取 `selectable=true` 候选的 `open_department_id`；兼容结果中的数字 `department_id` 只用于 `department batch-get` 回查，batch-get 返回的 `open_department_id` 才可用于行创建、更新、搜索和 import plan。若候选缺少 `open_department_id`，停止写入并提示部署目录接口修复，不使用数字 ID 兜底。
- 人员搜索返回的 `employee_id` 是矩阵写入使用的外部 ID；只采用 `selectable=true` 的候选。重名时展示姓名、部门、邮箱和 ID 让用户选；无结果或不可选时说明原因，不使用内部员工编号兜底。查询不需要写入确认，写入仍需先展示计划。
- 列 ID 只取 `table get` 或 `column-headers list` 的返回值；已有字段绑定信息原样保留，未绑定列不根据列名推测 `value_id`。金额字段仍需核对业务口径及单位。
- 此处枚举指五种条件类型及其运算符，不需要额外业务选项数据源。用 `rule symbol query` 查询 CLI 内置映射，提交输出中的 `symbol`；切换类型后重新查询，不把“包含”和“在…之内”混用。列更新时由开平接口做最终合法性校验。详细映射见 [配套查询](references/configuration-completion.md)。金额支持整数 30 位、小数 8 位，保持原单位和精度。
- 重复导入先搜索/读取已有行，由 Agent 明确追加、跳过或转 update；新建计划没有跨计划自动去重。
- `import apply --rows 1,3 --batch-size 10` 仅执行所选计划行和本次上限。交互执行可用 `--batch-size 1`。运行中收到暂停要求时调用 `import pause --plan-id`，当前行写入与回读结束后停批；`resume` 清除暂停标记，并将可续跑计划更新为 `status=ready`、清空旧 reason，再次 apply 继续。`needs_verification` 仍须先调用 `verify`，`resume` 不会放行未核验行。get 查询进度，cancel 取消尚未执行的后续操作。
- 不确定结果立即停批。用 get/list/search 核验后重新整理未完成输入并确认新计划。不直接编辑本地计划；已停用的旧 guarded 计划先核验再重建。
- plan 绑定列头和全量规则行摘要、身份、应用与环境指纹，24 小时过期；user 额外绑定凭证摘要，重新登录或 Token 轮换后重新生成并确认（不保存原始 Token）。每次 apply 前行快照变化使计划失效，本计划经回读验证的写入会推进快照；仍没有服务端原子 revision 条件写。结构变更后重新计划。
- 新接口已提供结构化命令，继续遵守项目 api call 关闭约定。
- 缺信息、歧义、重复导入、部分确认、暂停恢复、结果不确定或目标切换时，参见 [交互与组合操作契约](references/interaction-contract.md)。

## 参数文档

- 规则表查询：[references/table-list-parameters.md](references/table-list-parameters.md)
- 规则表预发布：[references/table-pre-release-parameters.md](references/table-pre-release-parameters.md)
- 规则表发布：[references/table-release-parameters.md](references/table-release-parameters.md)
- 列头查询：[references/column-headers-list-parameters.md](references/column-headers-list-parameters.md)
- 规则行创建：[references/row-create-parameters.md](references/row-create-parameters.md)
- 规则行详情：[references/row-get-parameters.md](references/row-get-parameters.md)
- 规则行分页：[references/row-list-parameters.md](references/row-list-parameters.md)
- 规则行搜索：[references/row-search-parameters.md](references/row-search-parameters.md)
- 规则行更新：[references/row-update-parameters.md](references/row-update-parameters.md)
- 规则行删除：[references/row-delete-parameters.md](references/row-delete-parameters.md)
- 批量导入：[references/import-parameters.md](references/import-parameters.md)
- 配套查询与发布核验：[references/configuration-completion.md](references/configuration-completion.md)
- 连续对话、导入引导、异常分支与组合操作：[references/interaction-contract.md](references/interaction-contract.md)

## 示例

```bash
contract-cli rule table list --profile contract --as user --product-id contract --group-id approve_matrix
```

```bash
contract-cli rule table list --profile contract --as app --product-id <product-id> --group-id <group-id> --page-size 10
contract-cli rule table column-headers list --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>
contract-cli rule table row list --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --page-size 10
contract-cli rule table row create --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --input-file row.json
contract-cli rule table row search --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --page-size 10 --input-file row-search.json
contract-cli rule table row update <row-id> --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --input-file row.json
contract-cli rule table row delete <row-id> --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>
contract-cli rule table import plan --profile contract --as app --product-id contract --group-id approve_matrix --table-id <table-id> --input-file import.json
contract-cli rule table import apply --profile contract --as app --plan-id <plan-id>
```

`row.json` 最小形态：

```json
{
  "table_cells": [
    {
      "table_column_id": "column-1",
      "table_cell_content_type": "STRING",
      "table_cell_content": {
        "string": "示例值"
      }
    }
  ]
}
```
