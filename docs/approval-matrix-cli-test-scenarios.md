# 审批矩阵 CLI 业务场景测试清单

本清单根据[原始审批矩阵交互方案](https://ysi13ckdb9.feishu.cn/docx/CaMkdHRL5ovfOAxXE5ScHFtMnoe)整理，并按当前 `contract-cli` 的 OpenPlatform 命令树落地。测试时逐项填写“结果”和“记录”，把原始响应中的 `code`、`msg`、资源 ID、版本号和分页标记保存下来。

## 0. 测试约定

### 0.1 占位参数

```bash
export PROFILE=contract       # 按实际测试 profile 替换
export AS=app                 # 另测 user
export PRODUCT_ID=contract
export GROUP_ID=approve_matrix
export TABLE_ID="<table-id>"
export COLUMN_ID="<column-id>"
export ROW_ID="<row-id>"
COMMON="--profile $PROFILE --as $AS --product-id $PRODUCT_ID --group-id $GROUP_ID"
```

公共参数（以下命令中的 `$COMMON` 代表这组参数）：

```bash
--profile "$PROFILE" --as "$AS" \
--product-id "$PRODUCT_ID" --group-id "$GROUP_ID"
```

下文命令列中的 `rule ...` 均需加上 `contract-cli` 前缀；例如 `rule table list ...` 的完整形式是 `contract-cli rule table list ...`。

### 0.2 记录格式

每个场景记录：

```text
场景编号：
执行命令 / 输入文件：
身份：user 或 app
HTTP 方法和路径：
响应 code / msg：
关键 data：
结果：通过 / 失败 / 当前范围外
问题描述：
```

写入场景使用专用测试矩阵或测试规则组。删除、预发布和发布只在确认记录后执行。报告中保留 ID 和版本号，隐藏 Token、Cookie 和个人敏感信息。

## 1. 接入与命令入口

| 编号 | 场景 | 命令 | 预期 |
| --- | --- | --- | --- |
| A-01 | 查看总帮助 | `contract-cli help rule` | 能看到 group、employee、department、role、symbol、loop-function、table |
| A-02 | `approval-matrix` 兼容入口 | `contract-cli approval-matrix table list ... --page-size 10` | 请求与 `rule table list` 相同 |
| A-03 | 发布兼容入口 | `contract-cli approval-matrix publish ...` | 实际进入 `rule table release` |
| A-04 | 表级发布兼容入口 | `contract-cli approval-matrix table publish ...` | 实际进入 `rule table release` |
| A-05 | user 身份 | 所有只读场景加 `--as user` | 使用 user Bearer 和规则权限校验 |
| A-06 | app 身份 | 所有只读、写入和发布场景加 `--as app` | 使用 app token |
| A-07 | profile 与身份缺失 | 省略或填写错误的 `--profile`、`--as` | 本地给出明确错误，不发送错误目标请求 |

## 2. 规则组与矩阵定位

### 2.1 规则组

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| B-01 | 查询已有规则组 | `rule group get $COMMON` | 返回组编码、名称、描述和矩阵范围 |
| B-02 | 创建测试规则组 | `rule group create $COMMON --data '{"group_id":"<new-group>","name":"CLI 测试组"}'` | 创建成功，返回可继续使用的 group ID |
| B-03 | 重复创建规则组 | 重复执行 B-02 | 返回业务错误，已有组保持不变 |
| B-04 | 组编码与 `--group-id` 不一致 | body 中传另一个 `group_code` | 本地或服务端拒绝，记录 code/msg |

### 2.2 矩阵列表和候选选择

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| B-05 | 查询矩阵列表 | `rule table list $COMMON --page-size 10` | 返回矩阵 ID、名称、状态、预发布/发布版本等字段 |
| B-06 | 第一页和下一页 | 使用返回的 `page_token` 再查 | 页间无重复，能遍历完整列表 |
| B-07 | 最小分页值 | `--page-size 1` | 请求成功或返回后端明确结果 |
| B-08 | 最大分页值 | `--page-size 100` | 请求成功或返回后端明确结果 |
| B-09 | 缺少分页大小 | 省略 `--page-size` | 本地提示必填 |
| B-10 | 越界分页大小 | `--page-size 0`、`--page-size 101` | 本地拒绝，不发送 HTTP |
| B-11 | 导入未指定矩阵 | `rule table import plan $COMMON --input-file import.json` | 返回 `needs_input` 和候选矩阵，不创建计划、不写数据 |
| B-12 | 0 个候选矩阵 | 在空规则组执行 B-11 | 返回空候选并要求补充矩阵定位 |
| B-13 | 多个候选矩阵 | 在有多个矩阵的规则组执行 B-11 | 展示候选 ID、名称、状态、版本，等待选择 |
| B-14 | 指定矩阵后读取列头 | B-11 增加 `--table-id "$TABLE_ID"` | 只读取目标矩阵列头并进入本地校验 |

### 2.3 矩阵定义生命周期

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| B-15 | 创建矩阵 | `rule table create $COMMON --data '{"name":"CLI 测试矩阵","description":"test","match_policy":0,"node_repetition_policy":0}'` | 创建成功，返回矩阵 ID、初始列头和默认空白行；首条规则使用该行的 `row update`，追加规则才用 `row create` |
| B-16 | 查询矩阵详情 | `rule table get $COMMON --table-id "$TABLE_ID"` | 返回 name、description、status、prepared_version、release_version、策略和列头 |
| B-17 | 修改矩阵名称和描述 | `rule table update $COMMON --table-id "$TABLE_ID" --data '{"name":"CLI 测试矩阵 V2","description":"updated"}'` | 更新成功，编码保持不变 |
| B-18 | 修改命中策略 | 更新 `match_policy` 为 0、1、2 | 返回值与后端枚举一致，记录实际含义 |
| B-19 | 修改节点去重策略 | 更新 `node_repetition_policy` 为 0、1、2 | 返回值与后端枚举一致 |
| B-20 | 修改时带回原描述 | 先 get，再只改 name 并带回 description | 原描述保持 |
| B-21 | 修改矩阵编码 | body 传与路径不同的 `rule_table_id` | 本地拒绝，矩阵编码保持不变 |
| B-22 | 删除测试矩阵 | `rule table delete $COMMON --table-id "$TABLE_ID"` | 删除成功；已有矩阵先确认影响范围 |

## 3. 人员、部门、角色和元数据查询

当前目录查询使用 OpenPlatform 路径：

- `employee search`：`/employees/search`
- `department search`：`/departments/search`
- `role search`：`/roles/search`

### 3.1 人员

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| C-01 | 精确人员搜索 | `rule employee search $COMMON --data '{"param":"<姓名>"}'` | 返回 employee_id、姓名、部门、状态、selectable |
| C-02 | 无结果 | 使用不存在的姓名 | 返回空候选并提示补充信息 |
| C-03 | 重名 | 使用会返回多个候选的姓名 | 展示姓名、部门、邮箱、ID，等待选择 |
| C-04 | 不可选人员 | 返回 `selectable=false` 的候选 | Agent 不把该候选用于规则行 |
| C-05 | group_code 一致 | body 加 `"group_code":"$GROUP_ID"` | 请求成功 |
| C-06 | group_code 不一致 | body 加其他 group_code | 本地拒绝或后端返回明确错误 |
| C-07 | 人员批量回读 | `rule employee batch-get $COMMON --data '{"ids":["<employee-id>"]}'` | 返回名称和缺失 ID 列表 |
| C-08 | 人员 ID 格式 | batch-get 传空 ID、非数字 ID、超过 100 个 ID | 本地校验并停止请求 |

### 3.2 部门和角色

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| C-09 | 部门搜索 | `rule department search $COMMON --data '{"param":"<部门关键词>"}'` | 返回部门候选和可用于规则行的 `open_department_id`（`od-...`）；若只有数字 `sys_department.id`，记录为目录接口字段缺失 |
| C-10 | 部门批量回读 | `rule department batch-get $COMMON --data '{"ids":["<department-id>"]}'` | 返回部门名称和缺失 ID |
| C-11 | 角色搜索 | `rule role search $COMMON --data '{"param":"<角色关键词>"}'` | 返回角色候选和外部 ID |
| C-12 | 角色批量回读 | `rule role batch-get $COMMON --data '{"ids":["<role-id>"]}'` | 返回角色名称和缺失 ID |
| C-13 | 空搜索词 | `param` 为空或全是空格 | 本地拒绝，不发送请求 |

### 3.3 操作符和循环函数

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| C-14 | STRING 操作符 | `rule symbol query $COMMON --data '{"value_type":"STRING"}'` | 返回 CLI 内置的 `=`,`!=`,`in`,`notIn`，全程不发网络请求 |
| C-15 | NUMBER 操作符 | `value_type=NUMBER` | 返回 CLI 内置的比较、集合操作符及 `valueTypes` |
| C-16 | 集合类操作符 | 依次查询 COLLECTION、EMPLOYEE_COLLECTION、DEPARTMENT_COLLECTION | 返回 CLI 内置的 contain、in、判空等候选，全程不发网络请求 |
| C-17 | BOOLEAN 兼容查询 | `value_type=BOOLEAN` | 返回 CLI 内置的 `=`,`!=` 兼容映射 |
| C-18 | 部门循环函数 | `rule loop-function query $COMMON --data '{"value_type":"DEPARTMENT_COLLECTION"}'` | 返回可选循环函数 |
| C-19 | 未知类型 | 传入未知 `value_type` | 本地拒绝 |

## 4. 列配置

### 4.1 读取与新增

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| D-01 | 查询列头 | `rule table column-headers list $COMMON --table-id "$TABLE_ID"` | 返回列 ID、名称、列类型、单元格内容类型及已绑定配置 |
| D-02 | 新增左侧列 | `rule table column add $COMMON --table-id "$TABLE_ID" --data '{"base_table_column_id":"<base-column>","direction":-1}'` | 返回新列 ID，位置在基准列左侧 |
| D-03 | 新增右侧列 | `direction=1` | 返回新列 ID，位置在基准列右侧 |
| D-04 | 非法插入方向 | `direction=0` | 本地拒绝 |
| D-05 | 基准列不存在 | 使用错误 base column ID | 返回明确业务错误 |

### 4.2 条件列

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| D-06 | 配置 STRING 条件列 | `rule table column update-condition $COMMON --table-id "$TABLE_ID" --column-id "$COLUMN_ID" --data '{"table_column_name":"采购类型","value_type":"STRING","symbol":"<symbol>"}'` | 更新成功，symbol 使用 CLI 内置映射值 |
| D-07 | 配置 NUMBER 条件列 | `value_type=NUMBER`，覆盖 `=`,`>`,`in` 等符号 | 更新成功，记录 valueTypes |
| D-08 | 配置部门集合条件列 | `value_type=DEPARTMENT_COLLECTION`，带 `is_department_loop` | 更新成功或由服务端按循环函数校验 |
| D-09 | 复用关联元素 | 带回 `value_id/value_code/value_name/value_type` | 既有绑定信息保持 |
| D-10 | 关联元素类型不匹配 | value_id 与 value_type 不匹配 | 服务端拒绝，原列配置保持 |
| D-11 | 有数据时改变类型或操作符 | 对已有值列改变内容类型 | 服务端阻止，先进入清空流程 |
| D-12 | 修改列名称 | 只改 `table_column_name` | 名称更新，其他配置保持 |

### 4.3 结果列

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| D-13 | 配置人员结果列 | `result_type=EMPLOYEE_COLLECTION`，带真实人员外部 ID | 更新成功 |
| D-14 | 配置部门结果列 | `result_type=DEPARTMENT_COLLECTION` | 更新成功或按后端映射校验 |
| D-15 | 配置角色结果列 | `result_type=ROLE_COLLECTION` | 更新成功或按角色 ID 契约校验 |
| D-16 | 设置默认审批人 | 传 `default_value` | 只影响结果列默认值，规则行中的指定审批人保持独立 |
| D-17 | 缺少结果类型 | 省略 `result_type` | 本地拒绝 |

### 4.4 局部修改和预检

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| D-18 | 纯改名 patch | `rule table column patch $COMMON --table-id "$TABLE_ID" --column-id "$COLUMN_ID" --data '{"table_column_name":"新名称"}'` | 只改名称，既有绑定配置保留 |
| D-19 | patch 空对象 | `--data '{}'` | 本地拒绝 |
| D-20 | preview 无数据列 | 提交拟议列配置 | 返回影响行数、受影响行 ID、是否需要清空和拟议配置 |
| D-21 | preview 有数据列类型变化 | 对有值列改变类型 | 返回清空影响，写入前停留在确认阶段 |
| D-22 | 受保护字段 | patch 传循环、流程去重等受保护字段 | 记录服务端或本地字段校验结果 |
| D-23 | 删除普通列 | `rule table column delete ...` | 删除成功，回读列头 |
| D-24 | 删除最后一个同类列 | 删除条件列或结果列中的最后一列 | 服务端保护并返回明确错误 |
| D-25 | 调整已有列顺序 | 尝试寻找对应 CLI 命令 | 当前 CLI 记录为范围外场景，不作为已实现能力验收 |

## 5. 规则行和单元格

### 5.1 查询

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| E-01 | 分页查询规则行 | `rule table row list $COMMON --table-id "$TABLE_ID" --page-size 10` | 返回行 ID、列 ID 和单元格内容 |
| E-02 | 行详情 | `rule table row get <row-id> $COMMON --table-id "$TABLE_ID"` | 返回完整单行 |
| E-03 | STRING 精确搜索 | row-search JSON 使用 STRING 单元格 | 返回匹配行 |
| E-04 | NUMBER 精确搜索 | 使用 NUMBER 单元格 | 返回匹配行并保留精度 |
| E-05 | BOOLEAN 精确搜索 | 使用 BOOLEAN 单元格 | 返回匹配行 |
| E-06 | 集合模糊搜索 | 使用 COLLECTION、人员、部门或角色集合 | 包含搜索值的行返回 |
| E-07 | 空值搜索 | 搜索单元格为空 | 返回本地或后端明确校验结果 |
| E-08 | row search 分页大小 | 缺少、传 0、传 101、传 1/100 | 必填和 1-100 校验生效 |
| E-09 | 多页搜索 | 使用返回的 page_token | 无重复、无漏行 |
| E-10 | 备注列 STRING 精确搜索 | 备注列存在多条不同值，搜索不存在的字符串 | 返回空行列表，不返回其他非空备注行 |

### 5.2 创建和更新

最小行请求示例：

```json
{
  "table_cells": [
    {
      "table_column_id": "<column-id>",
      "table_cell_content_type": "STRING",
      "table_cell_content": {"string": "测试值"}
    }
  ]
}
```

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| E-10 | 创建字符串行 | `rule table row create $COMMON --table-id "$TABLE_ID" --input-file row.json` | 创建成功，返回 row ID |
| E-11 | 创建数字行 | NUMBER 使用整数、小数和最大精度边界 | 精度保持；超 30 位整数或 8 位小数返回校验错误 |
| E-12 | 创建布尔行 | BOOLEAN 使用 true/false | 创建成功 |
| E-13 | 创建集合行 | COLLECTION 使用字符串数组 | 创建成功 |
| E-14 | 创建人员/部门行 | 人员使用正 int64 外部 ID，部门使用 `open_department_id`（`od-...`）数组 | 创建成功；部门数字 `sys_department.id` 进入错误分支 |
| E-15 | 创建角色行 | ROLE_COLLECTION 使用角色 ID 数组 | 按后端角色 ID 契约处理 |
| E-16 | 局部更新行 | `rule table row update <row-id> $COMMON --table-id "$TABLE_ID" --input-file row.json` | 只更新请求中出现的列 |
| E-17 | null 清空 | cells 中对应列值为 `null` | 单元格清空 |
| E-18 | 空数组清空集合 | 集合列使用 `[]` | 集合值清空 |
| E-19 | 列名映射导入 | import plan 的 cells 使用唯一列名 | 映射到真实列 ID |
| E-20 | 重名列映射 | cells 使用重名列名 | 返回候选，要求改用列 ID |
| E-21 | 删除规则行 | `rule table row delete <row-id> $COMMON --table-id "$TABLE_ID"` | 确认后删除，回读应查不到 |

## 6. 批量导入计划

`import.json` 示例：

```json
{
  "rows": [
    {"operation":"create","cells":{"合同金额":1000000,"审批人":["<employee-id>"]}},
    {"operation":"update","row_id":"<row-id>","cells":{"<column-id>":null}}
  ]
}
```

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| F-01 | 生成创建计划 | `rule table import plan $COMMON --table-id "$TABLE_ID" --input-file import.json` | 只读列头，返回 `needs_confirmation` 和 plan_id |
| F-02 | 生成更新计划 | 输入带 `operation=update` 和 row_id | 生成 PUT 请求计划，不发送写请求 |
| F-03 | 未知列 | cells 使用不存在列名或列 ID | `needs_input`，列出问题，不保存计划 |
| F-04 | 类型错误 | 字符串列传 number、数字列传 string 等 | `needs_input`，返回 expected_type |
| F-05 | 非法 operation | 传 delete 或其他值 | `needs_input` |
| F-06 | update 缺少 row_id | 省略 row_id | `needs_input` |
| F-07 | 顶层未知字段 | rows 外增加字段 | 本地 JSON 校验错误 |
| F-08 | null 和空数组 | 同时测试 null 清空和 [] 清空 | 计划中的 cell content 为空，执行语义为清空 |
| F-09 | 执行全量计划 | `rule table import apply --profile "$PROFILE" --as "$AS" --plan-id <plan-id>` | 串行执行，返回逐行结果 |
| F-10 | 局部行确认 | 增加 `--rows 1,3` | 只执行选中行 |
| F-11 | 分批执行 | 增加 `--batch-size 1` | 已执行行完成，剩余状态为 paused |
| F-12 | 查看计划 | `rule table import get --profile "$PROFILE" --plan-id <plan-id>` | 返回当前状态、摘要和逐行结果 |
| F-13 | 取消计划 | `rule table import cancel --profile "$PROFILE" --plan-id <plan-id>` | 后续行取消，已成功写入保留 |
| F-14 | 重试成功计划 | 再次 apply 同一 plan_id | 只读返回原结果，不重复写成功行 |
| F-15 | 明确失败后重试 | 修正外部数据后再次 apply | 只重试明确失败行 |
| F-16 | 结果不确定 | 模拟网络中断或响应缺少 code | 状态转 needs_input/uncertain，停止后续行，先查询核验 |
| F-17 | 列头变化 | 计划生成后修改列，再 apply | 状态 invalidated，要求重建计划 |
| F-18 | 计划过期 | 使用超过 24 小时的计划 | 状态 invalidated |
| F-19 | 身份或环境变化 | 用其他 profile、身份或环境 apply | 状态 invalidated 或本地拒绝 |
| F-20 | raw 输出限制 | plan/apply 增加 `--raw` | 本地提示使用结构化状态 |

## 7. 预发布与正式发布

| 编号 | 场景 | 命令 / 输入 | 预期 |
| --- | --- | --- | --- |
| G-01 | 预发布前建立基准 | `rule table pre-release $COMMON --table-id "$TABLE_ID"` | 读取完整规则行、保存基准并调用 pre_release |
| G-02 | 预发布响应核验 | 回读矩阵详情 | 保存 prepared_version |
| G-03 | 数据未变化正式发布 | `rule table release $COMMON --table-id "$TABLE_ID"` | 行基准、版本一致后调用 release，回读 release_version |
| G-04 | 新增行后发布 | 预发布后新增一行再 release | 阻断发布，要求重新预发布 |
| G-05 | 删除行后发布 | 预发布后删除一行再 release | 阻断发布 |
| G-06 | 修改单元格后发布 | 预发布后修改金额、人员或集合值 | 阻断发布 |
| G-07 | 返回顺序变化 | 两次查询行顺序不同但内容相同 | 按行 ID 比较，发布继续 |
| G-08 | 金额精度变化 | 测试 1000 与 1000.0、30 位整数、8 位小数 | 精确比较，不因 float64 丢失而误判 |
| G-09 | 重复行 ID | 查询返回重复 row ID | 阻断并报告分页数据异常 |
| G-10 | 分页 token 异常 | has_more=true 但 token 缺失或循环 | 阻断并报告分页异常 |
| G-11 | 查询失败 | 预发布或正式发布前读取行失败 | 不发送 release |
| G-12 | 版本变化 | prepared_version 与基准不一致 | 阻断并重新预发布 |
| G-13 | 矩阵或身份变化 | release 使用其他 table_id/profile/identity | 阻断发布 |
| G-14 | release 结果不确定 | release 请求网络中断 | 先查询版本和状态，不直接重试 |
| G-15 | 发布别名 | G-03 改用 `approval-matrix publish` 或 `approval-matrix table publish` | 与 `rule table release` 行为一致 |

## 8. 原始方案中的页面能力边界

以下场景属于原始方案第二档的标品页面接口能力。当前 CLI 以 OpenPlatform 接口为主，这些项目单独记录为“当前范围外”，避免和已实现命令混在一起：

| 编号 | 页面业务场景 | 当前记录 |
| --- | --- | --- |
| H-01 | 测试用例创建、执行、结果查看 | 当前范围外 |
| H-02 | 系统模板或 Excel 导入 | 当前范围外 |
| H-03 | 导入历史查看与恢复 | 当前范围外 |
| H-04 | 更新记录、审计记录 | 当前范围外 |
| H-05 | 版本历史和回滚 | 当前范围外 |
| H-06 | 已有列移动和排序 | 当前范围外 |
| H-07 | 流程设计器节点绑定 | 原方案要求单独确认 |
| H-08 | 审批流程节点配置 | 原方案要求单独确认 |
| H-09 | 协作角色自动邀请 | 原方案要求单独确认 |

## 9. 测试完成判定

一轮测试完成后至少应有：

1. B-05、B-11、B-14、B-16 完成矩阵定位闭环；
2. C-01、C-09、C-11、C-14 完成目录和元数据闭环；
3. D-01、D-02、D-06、D-13、D-18 完成列配置闭环；
4. E-01、E-02、E-03、E-10、E-16、E-17 完成行数据闭环；
5. F-01、F-09、F-12、F-13、F-16、F-17 完成批量导入状态闭环；
6. G-01、G-03、G-04、G-06、G-11、G-15 完成发布保护闭环；
7. H 组逐项标记为当前范围外或另行验收。

相关参数和请求体：

- [结构接口参数](../skills/contract-cli-rule/references/structure-parameters.md)
- [配套查询与版本保护](../skills/contract-cli-rule/references/configuration-completion.md)
- [批量导入参数](../skills/contract-cli-rule/references/import-parameters.md)
- [CLI 命令参考](cli-command-reference.md)
