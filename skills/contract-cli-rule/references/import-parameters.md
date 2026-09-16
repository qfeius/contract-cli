# rule table import Parameters

本页用于审批矩阵批量行导入的计划与执行命令。

## 命令

```bash
contract-cli rule table import plan --profile contract --as app --product-id contract --group-id approve_matrix --table-id <table-id> --input-file import.json
contract-cli rule table import apply --profile contract --as app --plan-id <plan-id>
```

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
- 人员、部门、角色集合必须先解析成 ID 数组。
- 计划阶段只查询列头，不写矩阵；成功返回 `needs_confirmation` 和 `plan_id`，校验问题返回 `needs_input`。

## apply 参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `--plan-id` | 是 | plan 输出的确认令牌 |
| `--profile` | 否 | 默认使用计划创建时的 profile；显式传入时必须一致 |
| `--as` | 否 | 支持 `user` / `app` |

- `apply` 串行调用现有创建或更新行接口，不依赖新增表。
- 每行结果落盘；重试同一计划会跳过成功行。
- 结果不确定立即停批，先用 `row get/list/search` 查询确认，不自动重试。
- 输出状态为 `success`、`partial_success`、`failed` 或 `needs_input`。
- `plan` 和 `apply` 都不支持 `--raw`。

## 2026-09-14 实现更新

- 人员/部门接收正 int64 数字或十进制字符串数组，输出数字数组；角色、普通集合仍用字符串数组。NUMBER 支持整数 30 位、小数 8 位的精确 JSON 数字，不经 float64；保留金额单位，不自动换算。显式 null 清空该单元格，省略单元格不修改；集合也可用 [] 清空。
- apply 执行前重读列头，计划 24 小时有效；身份/应用/环境指纹或列头变化返回 invalidated。旧版未完成计划需重新生成。user 计划还绑定凭证摘要，重新授权或 Token 轮换后需重新生成并确认；app Token 轮换仍保持兼容。
- --rows 1,3 只执行指定计划行；--batch-size 10 限定本次写入数。剩余进度为 paused，再次 apply 继续。
- import get --plan-id 查看进度；import cancel --plan-id 取消后续操作，已成功数据保留。
- 每次写请求前保存 running；进程退出后按 uncertain 恢复并阻止继续执行，先人工查询核验。
- 401/403/429 停批；404/409/412 使计划 invalidated，原 code/msg 保留。
- 计划只校验列头摘要，不具备全表原子版本保护或跨计划去重。历史 guarded 计划已停用，执行返回 invalidated；先核验已成功行，再生成并确认新计划。
- 配套查询和版本条件发布详见 [配套查询及版本保护](configuration-completion.md)。
