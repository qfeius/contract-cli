# 审批矩阵结构接口参数

代码依据：`bpm-rule-configuration` 分支 `20260901-zss-approval` 的 2026-09-14 工作树。依据 Controller 与 OpenApi DTO 核对，接口部署状态需由对应环境确认。

全部支持 `--as user` / `--as app`，显式选择 `--profile`；JSON 通过 `--input-file` 或 `--data`，GET/DELETE 不带请求体。定位使用对外编码，避免传数据库主键。

| 命令 | HTTP | 路径（前缀 `/open-apis/rule_engine/v1/products/{product_id}`） | 必填定位 |
| --- | --- | --- | --- |
| `rule group get` | GET | `/groups/{group_id}` | product-id, group-id |
| `rule table create` | POST | `/groups/{group_id}/rule_tables` | product-id, group-id |
| `rule table get/update/delete` | GET/PUT/DELETE | `/groups/{group_id}/rule_tables/{rule_table_id}` | product-id, group-id, table-id |
| `rule table column add` | POST | `/groups/{group_id}/rule_tables/{rule_table_id}/table_columns` | product-id, group-id, table-id |
| `rule table column update/update-condition/update-result/delete` | PUT/PUT/PUT/DELETE | `/groups/{group_id}/rule_tables/{rule_table_id}/table_columns/{table_column_id}` | product-id, group-id, table-id, column-id |

## 规则组

contract 产品页面只展示固定规则组 `approve_matrix`。本技能仅查询该组，不创建新组；`rule group create` 对 contract 产品在 CLI 执行层被阻断。已知其他组的只读查询仍可用于排查。

## 矩阵定义

contract 产品新建矩阵必须使用 `--group-id approve_matrix`；CLI 会在发送请求前拒绝其他组，避免出现页面不可见的矩阵。

创建请求不传 `rule_table_id`：`{"name":"采购矩阵","description":"采购规则","match_policy":0,"node_repetition_policy":0}`。

更新请求通过路径中的矩阵 ID 定位，可省略请求体中的 `rule_table_id`：`{"name":"采购矩阵","description":"采购规则","match_policy":0,"node_repetition_policy":0}`。

- name 必填且最多 100 字符；description 可选，最多 300 字符。
- `rule_table_id` 是服务端生成的对外矩阵 ID。创建前不向用户索取或根据名称生成；创建返回后内部保存。普通创建确认和成功回复不主动展示，遇到同名/多状态定位、排障、组合操作部分完成或用户明确查询时可以展示。更新时省略或与路径一致，已有 ID 不可修改。
- match_policy、node_repetition_policy 接受 0..2；具体命中策略含义按已确认后端枚举选择，勿根据中文标签猜数字。
- 节点去重策略：0 不去重、1 前去重、2 后去重。更新省略策略保留原值；description 省略不保证保留，应先 get 后带回。
- 创建初始化默认列及一条空白行。创建后先 `row list --page-size 10`；首条规则优先用 `row update <blank-row-id>` 填充这条默认行，后续规则再用 `row create` 追加，避免留下无业务数据的空白行。get 返回 id/name/description/status/prepared_version/release_version/match_policy/node_repetition_policy/column_headers。
- column_headers 返回 id/name/type/table_cell_content_type；其中 id 是列编码，用于后续列更新和规则行单元格定位。
- 修改列不调用 column preview；读取当前配置和规则行，确认后通过原有更新接口提交并回读核验。PUT 带齐已确认的保留字段，不自动清空数据。规则行对比发布流程见 [配套查询及版本保护](configuration-completion.md)。
- 额外回读 loop_function_id/name、extra_config、process_repetition_policy/node_name，用于展示已有循环和流程去重设置；当前 PATCH 不接受这些字段且保留旧策略。非元素绑定仅支持纯改名，不会隐式转成元素绑定。

## 条件列和结果列

容量约束：规则表总列数上限为 12，服务端同时统计条件列、结果列、优先级列和备注列；优先级、备注固定占 2 列，所以条件列与结果列合计最多 10 列。开始结构变更前必须先读取当前列头，并按完整目标方案验证 `目标条件列数 + 目标结果列数 + 2 <= 12`。例如 10 个条件列加 2～3 个结果列总计 14～15 列，需在首次写入前停止并调整方案。`column add` 会额外读取一次列头，在当前总列数达到 12 时本地阻断，不发送新增请求；服务端仍保留错误码 20303 作为并发变化等场景的最终保护。

新增分两步：

1. `column add` 请求 `{"base_table_column_id":"123","direction":1}`；-1 左侧、1 右侧。新列继承同类基准列的条件/结果类别。
2. 根据返回的 `table_column_id` 执行配置。条件列：`{"table_column_name":"金额","value_type":"NUMBER","symbol":"已确认的操作符"}`。结果列：`{"table_column_name":"审批人","result_type":"EMPLOYEE_COLLECTION","result_code":"approver","default_value":"7113921696628736004"}`。

条件列可选字段：value_id、value_code、value_name、is_department_loop、multi_select_match_mode。value_id 指向同租户同规则组业务元素，且类型必须匹配；is_department_loop=true 仅适用 DEPARTMENT_COLLECTION。

协商自动邀请场景把 `合同类型` 作为必需条件列，并固定使用 `value_type=COLLECTION`。收到业务 Excel 时先确认用途，再检查现有结构；不得把资料中的文本展示形式直接推导为 STRING。缺列或类型不符属于结构变更，必须先展示差异并单独确认，变更完成后重新读取列头。具体操作符按业务匹配语义从 `rule symbol query` 候选中选择；新增未绑定列仍省略 `value_*`，不根据列名猜测元素绑定。

配置已有列时先调用 `table get` 或 `column-headers list`，以返回的列 `id` 定位更新目标；若返回 `value_id/value_code/value_name/value_type`，更新时原样复用。新增或未绑定列按列名、条件类型和操作符配置并省略 `value_*`，不根据名称猜测业务元素 ID。

结果列 default_value 为字符串，人员为逗号分隔的正整数外部 ID，部门为逗号分隔的 `open_department_id`（`od-...`）；角色沿用角色 ID 契约。条件/结果更新共用 PUT，update-condition/update-result 提供对应必填字段检查。

已有数据且改变类型或操作符导致内容类型变化时服务端阻止更新；先单独确认清空数据，再修改。优先级列、备注列不参与这些结构操作，删除至少保留同类一列。新增左右插入已有代码依据，移动已有列排序及指定位置插行仍无此接口。

## 交互顺序

按页面主流程执行，使用开平身份和对外编码，不使用页面 Cookie 或内部矩阵/列 ID：

| 页面操作 | CLI 对应操作 |
| --- | --- |
| `/decision_table/put` 创建 | `rule table create`（已有矩阵跳过） |
| `/decision_table/get_by_id` | `rule table get` |
| `/decision_table/row/page` | `rule table row list --page-size 10` |
| `/decision_table/column/condition/update` | `rule table column update-condition` |
| 填写条件阈值和审批人 | `rule table row update <row-id>` 或 `row create` |

条件更新字段：页面 `column_name` 对应开平 `table_column_name`；`symbol/value_type/value_id/value_code/value_name/is_department_loop` 沿用已确认值，组编码通过 `--group-id` 定位。当前 BPM 的 ConditionColumnUpdateRequest 接收 `value_id`，不接收示例中的 `value` 或 `type`；转换器固定元素类型为 2，不能把 `value:"4"` 自动认定为元素 ID 4。开平更新复用该页面转换器。

已有列从表详情或列头回读 ID 和配置，人员名称仍需解析为真实人员 ID。列结构变更单独确认，完成后重新读列头。单行配置可直接 row update/create；多行才生成导入计划。阈值 1000 和人员 ID 写入规则行，不放在条件列定义或默认审批人中。

确认使用页面可见的 `approve_matrix` 规则组 → 读取/创建矩阵并读取行 → 单独确认列结构变更 → 重新读列头 → 确认并写入规则行 → 查询核验。预发布和发布仅在用户要求后另行确认执行。

保留原子命令直接调用习惯，写前确认和结构变更影响摘要由 Agent 承接。原子命令不会自动生成计划或自动发布。
