# rule table import Parameters

本页用于审批矩阵批量行导入的计划与执行命令。

## 命令

```bash
contract-cli rule table import plan --profile contract --as app --product-id contract --group-id approve_matrix --table-id <table-id> --input-file import.json
contract-cli rule table import apply --profile contract --as app --plan-id <plan-id>
contract-cli rule table import pause --plan-id <plan-id>
contract-cli rule table import resume --plan-id <plan-id>
contract-cli rule table import verify --profile contract --as app --plan-id <plan-id>
```

## 导入前用途识别

- Excel、表格或字段资料是批量行的数据来源，不等于完整的矩阵结构定义。Agent 在转换为本命令接收的 JSON 前，必须先确认矩阵用于审批路由、协商自动邀请还是其他场景。
- 用户只要求“批量导入矩阵”而用途不明确时，先询问业务用途；用途明确前不创建矩阵、不修改列，也不调用 `import plan` 或 `import apply`。
- 用途为协商自动邀请时，先用 `table get` 或 `column-headers list` 检查 `合同类型` 条件列。该列必须配置为 `value_type=COLLECTION`；缺失或类型不符时，先展示结构差异并单独确认列变更，变更后重新读取列头，才能生成导入计划。
- Excel 中合同类型以文本展示，不改变上述集合类型约束。条件操作符和导入值按已确认的匹配语义选择，不根据列名猜测 `value_id/value_code/value_name`。

## plan 参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--product-id` | 是 | 审批矩阵固定使用 `contract` |
| `--group-id` | 是 | 审批矩阵固定使用 `approve_matrix` |
| `--table-id` | 条件必填 | 已存在的规则表 ID；未传时返回 `needs_input` 和候选矩阵，不生成计划 |
| `--input-file` / `--data` | 二选一 | 批量行 JSON |
| `--profile` | 否 | 不传时使用当前 profile |
| `--as` | 否 | 支持 `user` / `app`；不传时使用 profile 默认身份 |

输入：

```json
{
  "rows": [
    {
      "operation": "create",
      "cells": {
        "合同金额": 1000000,
        "审批人": ["7113921696628736004"]
      }
    },
    {
      "operation": "update",
      "row_id": "<row-id>",
      "cells": {
        "<column-id>": ["7113921696628736005"]
      }
    }
  ]
}
```

- `operation` 默认 `create`，也可为 `update`；`update` 必须有 `row_id`。
- `cells` 的 key 可以是列名或列 ID；重名列必须使用列 ID。
- 值类型必须与列头 `table_cell_content_type` 一致。
- 人员集合使用正整数外部 ID；部门集合使用 `open_department_id` 字符串数组（形如 `od-...`）；角色集合使用角色 ID 字符串数组。目录搜索候选可直接取 `open_department_id`；只有数字 `department_id` 时，先用 `rule department batch-get` 回查并取返回的 `open_department_id`，数字值不可直接导入。
- 计划阶段查询列头和全部分页规则行，不写矩阵；按现有行数加计划 `create` 数校验 2000 行上限，`update` 不增加行数。成功返回 `needs_confirmation` 和 `plan_id`，超限、目标行不存在或其他校验问题返回 `needs_input`。

### 新建矩阵批量写入的首行

新建矩阵会自带一条默认空白行，但 `import plan` 不会自动识别或复用该行。向新建矩阵批量写入已确认规则时，Agent 必须先执行 `row list --page-size 10`：

1. 仅当查询结果能唯一确认一条默认空白行时，取得其 `row_id`。
2. 输入规则的第一条设置为 `operation: "update"`，并传入该 `row_id`。
3. 第二条及后续规则设置为 `operation: "create"`。
4. 没有空白行、存在多个候选或候选行已有业务值时，停止生成计划并重新核对，不能把全部规则按 create 执行。
5. apply 完成后重新分页查询，确认没有遗留默认空白行，目标业务规则行数和单元格值与计划一致。

示例：

```json
{
  "rows": [
    {
      "operation": "update",
      "row_id": "<blank-row-id>",
      "cells": {"合同金额": 1000000}
    },
    {
      "operation": "create",
      "cells": {"合同金额": 2000000}
    }
  ]
}
```

## apply 参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--plan-id` | 是 | plan 输出的确认令牌 |
| `--profile` | 否 | 默认使用计划创建时的 profile；显式传入时必须一致 |
| `--as` | 否 | 支持 `user` / `app` |

- `apply` 串行调用现有创建或更新行接口，不依赖新增表。
- 每行结果落盘；写入成功后按行 ID 回读，值吻合才记为 `success`；重试同一计划会跳过已验证的成功行。
- `partial_success`、`failed` 和 `invalidated` 会在输出完整逐行结果后返回非零退出码。
- 结果不确定立即停批，先用 `row get/list/search` 查询确认，不自动重试。
- 输出状态包括 `ready`、`success`、`paused`、`needs_verification`、`partial_success`、`failed`、`invalidated` 或 `needs_input`。
- 回读失败或内容不符时保留该行 `unverified` 和已知行 ID，停止后续写入；`import verify --plan-id` 只重试回读，不重发写请求。修正差异后再核验或重新生成计划。
- `import pause --plan-id` 可在执行中提交暂停意图，当前行写入和回读结束后不再启动下一行；`import resume --plan-id` 清除标记，若计划有待执行行且无需核验，返回 `status=ready`、空 reason，再次调用 `apply` 续跑。`needs_verification` 必须先 `verify`；`get` 返回持久化状态和 `pause_requested`。
- `plan` 和 `apply` 都不支持 `--raw`。

## 2026-09-21 部门 ID 约定

- `DEPARTMENT_COLLECTION` 在 `import plan` 中按 `open_department_id` 字符串校验并原样发往行接口；例如 `["od-1b1b803a7df98989bf457d9ba203c350"]`。数字部门主键会返回明确的本地校验错误，不再提示使用正整数 ID。

## 2026-09-14 实现更新

- 人员接收正 int64 数字或十进制字符串数组并输出数字数组；部门接收 `open_department_id` 字符串数组并原样输出；角色、普通集合仍用字符串数组。NUMBER 支持整数 30 位、小数 8 位的精确 JSON 数字，不经 float64；保留金额单位，不自动换算。显式 null 清空该单元格，省略单元格不修改；集合也可用 [] 清空。
- apply 执行前重读列头和全部规则行，计划 24 小时有效；身份/应用/环境指纹、列头或行快照变化返回 invalidated。每条本计划写入经回读确认后推进预期快照。旧版未完成计划需重新生成。user 计划还绑定凭证摘要，重新授权或 Token 轮换后需重新生成并确认；app Token 轮换仍保持兼容。
- --rows 1,3 只执行指定计划行；--batch-size 10 限定本次写入数。剩余进度为 paused，再次 apply 继续。
- import get --plan-id 查看进度；import cancel --plan-id 取消后续操作，已成功数据保留。
- 每次写请求前保存 running；进程退出后按 uncertain 恢复并阻止继续执行，先人工查询核验。
- 401/403/429 停批；404/409/412 使计划 invalidated，原 code/msg 保留。
- 行快照在 apply 启动或恢复时比较，写入前还会核对 update 目标行；服务端仍无原子 revision 条件写，也没有跨计划去重。历史 guarded 计划已停用，执行返回 invalidated；先核验已成功行，再生成并确认新计划。
- 配套查询和版本条件发布详见 [配套查询及版本保护](configuration-completion.md)。
