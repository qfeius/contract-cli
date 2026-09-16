---
name: contract-cli-rule
description: "contract-cli 审批矩阵技能：用 user/app 双身份管理规则组、矩阵定义、条件列和结果列、规则行、批量导入计划及发布；支持 rule 和 approval-matrix 命令入口。"
---

# contract-cli Rule

接口前缀：`/open-apis/rule_engine/v1`。

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

## 适用命令

- `contract-cli rule group get|create`
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
- `contract-cli rule table import get|cancel`

文档中的 `approval-matrix group/table/column/row/import/publish` 已映射到上述命令；其中 `approval-matrix publish` 和 `approval-matrix table publish` 都映射为 `rule table release`，帮助显示实际 `rule` 路径。新增结构接口的字段和操作顺序先读 [references/structure-parameters.md](references/structure-parameters.md)。

## 快速决策

- 先确认规则表：`rule table list --product-id --group-id`
- 主流程与页面一致：新矩阵 `table create` → `table get` → `row list --page-size 10` → 配置条件列/结果列 → 重新读取列头 → 写入规则行并回读。创建接口会生成一条后端默认空白行；首条规则优先使用 `row update <blank-row-id>` 填充该行，只有追加额外规则时才使用 `row create`。已有矩阵从 get/row list 开始，不重复创建。接口映射见 [结构参数](references/structure-parameters.md)。
- 通过 `table get` 或 `column-headers list` 获取列 ID 和现有列配置。已有列若返回 `value_id/value_code/value_name/value_type`，更新时原样复用；新增或未绑定列按名称、类型和操作符配置并省略 `value_*`，不猜测业务元素 ID。
- 已有当前租户已确认且可选的人员外部 ID 时直接用于规则行；只有姓名或候选不明确时才用 `rule employee search`，读取 [人员搜索参数](references/employee-search-parameters.md)。不要为了跳过搜索而猜 ID，也不要直接要求用户手工找 ID。
- 查列头：`rule table column-headers list --product-id --group-id --table-id`
- 配置条件列时使用 CLI 内置的 `symbol` 映射；`rule symbol query` 仅输出本地候选，不访问 `symbols/query`。部门/角色先搜索并核对候选，已有 ID 用 batch-get 回读名称。参见 [配套查询及版本保护](references/configuration-completion.md)。
- 新增或修改规则行：`row create|update --input-file row.json`
- 用户一次给出多行：先 `import plan --input-file import.json`，展示摘要并确认后再 `import apply --plan-id ...`。
- 按条件分页查规则行：`row search --page-size 10 --input-file row-search.json`
- 删除行：先 `row get` 展示行摘要，用户确认后再执行 `row delete`
- 发布使用原有 `pre-release/release`。预发布前读取全部分页规则行保留基准；正式发布前再次全量读取，按行 ID、列 ID 比较内容，忽略返回顺序并保留 ID、金额精度。发现新增、删除或值变化时停止发布，重新预发布并确认；查询失败、分页不完整也停止。基准绑定环境、身份、矩阵 ID 和预发布版本。仅核对规则行，不提供原子并发保护。

### 豆包首条规则处理（固定）

创建矩阵后，豆包必须先执行 `row list --page-size 10`。当返回唯一一条后端默认空白行且当前要写入首条规则时，固定调用 `row update <blank-row-id>` 填充该行；`row create` 仅用于追加第二条及后续规则。写入后再执行 `row list --page-size 10` 核验，确保矩阵中没有遗留的默认空白行。

## 关键规则

- 全部命令支持 `--as user` / `--as app`；不传时使用 profile 默认身份。user 当前用于 `--product-id contract`，需具备合同规则管理权限，失败时禁止自动切换 app。用户登录复用共享 Skill 的授权流程。
- 矩阵写操作的编辑人和审计人始终取当前登录身份：`--as user` 时由开平服务端从 Bearer 用户令牌解析当前用户 ID，并结合 `X-Qfei-Identity: user` 完成鉴权。规则命令不会把 `--user-id` 当作编辑人覆盖值；该参数在矩阵路径会被移除。`--as app` 仍按应用/系统身份记录，要求以当前用户留痕时使用 `--as user`。
- 公共定位参数：`--product-id`、`--group-id`
- 涉及单表时还需要 `--table-id`；列操作使用表详情或列头查询返回的列 ID。
- `row create`、`row search`、`row update` 必须传 JSON 请求体
- `row list` 必须显式传入 `--page-size`，例如 `--page-size 10`；包括首次读取、后续分页及写后核验。服务端实测约束见 [规则行分页参数](references/row-list-parameters.md)。
- `row search` 支持 `--page-size` / `--page-token` 作为 query 参数，请求体保持筛选条件 JSON
- `pre-release`、`release` 支持可选 JSON body；当前自动化样例不发送请求体
- GET/DELETE 类命令不接受 `--input-file` / `--data`
- `import plan` 只读取列头并做本地映射、类型校验；`import apply` 才逐行调用既有创建/更新接口。
- 批量计划输入中的人员、部门和角色集合必须已经解析成对应 ID 数组
- 批量执行跳过成功行；结果不确定先用 row get/list/search 核验，不自动重试。
- 所有新增、更新、删除、结构变更及发布先展示目标、变更内容和影响，用户确认后再调用。结构变更单独确认，完成后重新读列头再生成导入计划。
- 矩阵或列名称匹配多个候选时展示 ID 和摘要；不凭姓名、行号或近似列名猜测目标。
- 更新矩阵时 name 必填、rule_table_id 不变；保留描述需要把当前 description 一起提交。
- 新增列先 add（base_table_column_id + direction），取得新列 ID 后再 update-condition/update-result；新增类型与基准列一致。
- 修改已有列不调用 `column preview`：先读取当前配置和规则行，展示变更并确认，再调用已有 update-condition/update-result 更新接口，回读核验。PUT 需要带齐已确认的配置字段，避免遗漏覆盖；缺少保留字段时先补齐，不猜测。类型或运算符变更仍由服务端校验，不自动清空数据。
- 有值的列改变类型或导致单元格类型变化的操作符时，服务端阻止更新。先展示影响，另行确认清空，再配置列；确认改类型不等于自动授权清空。
- 人员/部门使用正整数外部 ID，批量输入接受数字或十进制字符串数组，发往后端为整数数组；禁止用 `ou_`/`od_` 字符串代替该分支 DTO 的 Long ID。角色仍为字符串数组。
- 人员搜索返回的 `employee_id` 是矩阵写入使用的外部 ID；只采用 `selectable=true` 的候选。重名时展示姓名、部门、邮箱和 ID 让用户选；无结果或不可选时说明原因，不使用内部员工编号兜底。查询不需要写入确认，写入仍需先展示计划。
- 列 ID 只取 `table get` 或 `column-headers list` 的返回值；已有字段绑定信息原样保留，未绑定列不根据列名推测 `value_id`。金额字段仍需核对业务口径及单位。
- 此处枚举指五种条件类型及其运算符，不需要额外业务选项数据源。用 `rule symbol query` 查询 CLI 内置映射，提交输出中的 `symbol`；切换类型后重新查询，不把“包含”和“在…之内”混用。列更新时由开平接口做最终合法性校验。详细映射见 [配套查询](references/configuration-completion.md)。金额支持整数 30 位、小数 8 位，保持原单位和精度。
- 重复导入先搜索/读取已有行，由 Agent 明确追加、跳过或转 update；新建计划没有跨计划自动去重。
- `import apply --rows 1,3 --batch-size 10` 仅执行所选计划行和本次上限。剩余行为 paused；再次 apply 继续前先核对用户确认范围。get 查询进度，cancel 取消尚未执行的后续操作。
- 不确定结果立即停批。用 get/list/search 核验后重新整理未完成输入并确认新计划。不直接编辑本地计划；已停用的旧 guarded 计划先核验再重建。
- plan 绑定列头摘要、身份、应用与环境指纹，24 小时过期；user 额外绑定凭证摘要，重新登录或 Token 轮换后重新生成并确认（不保存原始 Token）。行写入没有全表原子版本锁，写前和写后需要查询核验；结构变更后重新计划。
- 新接口已提供结构化命令，继续遵守项目 api call 关闭约定。

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
