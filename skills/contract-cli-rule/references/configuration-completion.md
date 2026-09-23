# 配套查询与版本保护（2026-09-14）

本页对应三个项目 `20260901-zss-approval` 工作树中的新增实现；部署 BPM、开平与新版 CLI，无新增数据库迁移。全部命令支持 user/app，权限仍由当前身份校验；查不到或权限失败不自动切换身份。

矩阵写入时，编辑人/审计人由开平服务端从当前 Bearer 身份解析。规则路径会移除通用 `--user-id` query；审批人、部门和角色 ID 只作为规则业务值提交。需要当前登录用户留痕时使用 `--as user`，`--as app` 沿用应用/系统身份。

## 只读配套查询

目录和循环函数命令使用公共参数 `--product-id contract --group-id approve_matrix --profile <profile> --as user|app`，POST 用 `--input-file` 或 `--data`。`symbol query` 只读取 CLI 内置映射，`--product-id`、`--group-id`、`--profile` 和 `--as` 均可省略。

| CLI（前缀 `rule`） | HTTP（前缀 `/open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}`） | JSON |
| --- | --- | --- |
| `employee batch-get` | POST `/employees/batch_get` | `{"ids":["123"],"group_code":"approve_matrix"}` |
| `department search` | POST `/departments/search` | `{"param":"部门关键词","group_code":"approve_matrix"}` |
| `department batch-get` | POST `/departments/batch_get` | `{"ids":["123"]}`（数字 `department_id`，用于回查 `open_department_id`） |
| `role search` | POST `/roles/search` | `{"param":"角色关键词"}` |
| `role batch-get` | POST `/roles/batch_get` | `{"ids":["123"]}` |
| `symbol query` | CLI 本地映射 | `{"value_type":"NUMBER"}` |
| `loop-function query` | POST `/loop_functions/query` | `{"value_type":"DEPARTMENT_COLLECTION"}` |

- group_code 可省略，传入时与路径组编码一致。batch-get 接受 1..100 个字符串 ID：人员使用正 int64 外部 ID；部门 batch-get 使用目录返回的数字 `department_id`，用于回查可写入的 `open_department_id`；角色依租户来源使用对应 ID。
- 目录回包为 `data.items`（含专用 employee_id/department_id/open_department_id/role_id、name、selectable）及 `missing_ids`。部门搜索可直接使用 `selectable=true` 候选的 `open_department_id`（`od-...`）；数字 `department_id` 只作为 batch-get 查询键，不进入规则行。batch-get 结果同样取 `open_department_id`，用于行创建、更新、搜索和 import plan。候选缺少 `open_department_id` 时应为 `selectable=false`；此时提示部署目录接口修复，不使用数字 ID 兜底。
- 操作符取 CLI 内置映射中的 `symbol`；列更新时仍由开平接口校验合法性。循环配置只使用 `loop-function query` 返回的可用候选。
- 本页枚举是条件类型及运算符，不是业务字段可选值，不需要另接业务枚举接口。

### 条件类型与运算符

BPM 复用规则引擎依赖的 DataType/Symbol，普通 SaaS 查询由 CellValueTypeMappingEnum 提供支持范围及单元格类型；CLI 将这份稳定映射内置在 `rule symbol query` 中。

| 类型 | CLI 内置 symbol（按页面顺序） |
| --- | --- |
| STRING | `=`、`!=`、`in`、`notIn` |
| NUMBER | `=`、`>`、`>=`、`<`、`<=`、`!=`、`in`、`notIn` |
| COLLECTION、EMPLOYEE_COLLECTION、DEPARTMENT_COLLECTION | `contain`、`notContain`、`=`、`in`、`notIn`、`isNull`、`isNotNull` |

例如 `rule symbol query --data '{"value_type":"NUMBER"}'`。该命令只读取 CLI 本地映射，输出候选的中文名称、真实 `symbol` 和 `valueTypes`，整个过程不读取 profile、不需要身份授权、不发起 `/symbols/query` 请求。等于/相等是 `=`，包含是 `contain`，在…之内是 `in`。`valueTypes` 描述右侧单元格类型，例如 NUMBER 的 in/notIn 对应 COLLECTION，而不是 NUMBER。

BOOLEAN 旧查询保持兼容；内部决策校验继续保留 anyIn 和部门灰度逻辑，但普通五类型选择列表不公开内部 anyIn。判空运算符是左值判空条件，与导入 null 清空已保存单元格是不同操作。

## 配置回读

普通 table get/column-headers 用于读取矩阵和列配置，实际字段以已部署接口为准。

table 返回详情而非全量行，展示规则还需 row list/get。

## 列预检、局部修改

`rule table column preview --table-id <id> --column-id <id> --input-file changes.json` → POST `/rule_tables/{id}/table_columns/{id}/preview`，返回 populated_rows、requires_clear、affected_row_ids、proposed。

`column patch` → PATCH 同一列路径，只改传入字段；纯改名 `{"table_column_name":"金额"}` 保留未绑定配置。非空列类型不兼容仍拒绝，预检不是清空授权。确认需要清空时先逐行计划显式 null，再重新读取矩阵和列配置。原 PUT 保持原语义。

## 批量与发布

原有 `pre-release/release` 命令已内置以下规则行基准检查，不再只是 Agent 的建议步骤。基准保存在配置目录 `approval-matrix-publish`，不含原始凭证；正式发布前仍由 Agent 展示版本与摘要并取得用户确认。旧安装包需更新后才具备此检查。

- `import plan` 查询列头，apply 串行调用既有行接口。成功行跳过，结果不确定时停止并先查询核验；没有服务端幂等回执或行写入原子 revision 条件。参见 [导入参数](import-parameters.md)。
- 预发布前全量分页读取规则行作为基准，调用原有 pre-release 后绑定 prepared_version。
- 本地基准丢失但矩阵详情为 `status=0`（待发布）且有 `prepared_version` 时，重新执行 `pre-release` 会双读版本和完整规则行；两次结果稳定后仅恢复本地基准，返回 `baseline_recovered=true`，不重复请求服务端预发布。Agent 重新展示版本和行数并取得正式发布确认后再执行 `release`。
- 基准恢复不改写规则行。不得用“原值重写”、清空、增删行或修改列把矩阵重置为草稿；这些业务写操作不属于原发布确认范围，也会改变已预发布内容。
- 正式发布前再次全量读取，按行 ID、列 ID 对比内容，忽略返回顺序并保留 ID、金额精度；有新增、删除或值变化时停止发布，重新预发布并确认。读取失败或分页不完整时停止。
- 基准绑定 profile、环境、身份、矩阵 ID 和预发布版本，切换目标或重新预发布后更新基准。
- 数据一致且用户确认后调用原有 release，再回读 release_version；结果不确定先查询，不自动重试。

本流程只核对规则行，不检查 symbol 等列配置，也不消除查询与发布之间的并发时间差。无需新增 SQL。
