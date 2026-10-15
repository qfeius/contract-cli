# 按姓名／部门发现原始任务

适用于用户以人员姓名或部门名称提出效率分析。默认从BPM原始任务发现目标人员已办和待办，再由流程记录按节点、动作和完成时间重算。不得将APPROVE_AUTHORITY异步参与人字段作为统计必需条件：已验证真实审批任务存在而该字段漏项。

## 运行入口

脚本只复用 CLI 用户授权，不负责登录。每次选择一个姓名或一个部门；多对象请求分别解析后按真实 ID 合并。本版脚本仅接受 `contract-test`，当前已验证环境为 test。

```bash
python3 <skill-directory>/scripts/discover_businesses.py \
  --employee-name '<姓名>' --start '<带时区开始>' --end '<带时区结束>' \
  --profile contract-test --output <discovery.json>

python3 <skill-directory>/scripts/discover_businesses.py \
  --department-name '<部门名称>' --start '<带时区开始>' --end '<带时区结束>' \
  --profile contract-test --output <discovery.json>
```

CLI 不在 PATH 时加 `--cli <contract-cli可执行文件路径>`。用户已指定合同/合同组时加 `--resolve-only`，只取得人员范围，不扩大合同范围。

输出包含 `status`、`scope`、`employees`、`approver_ids`、`process_instances`、`task_records`、`businesses`、`coverage`、`diagnostics`、`continuation`。默认raw-tasks模式中采集清单是process_instances，businesses可为空，不猜业务类型。complete表示目标人员已办/待办分页完成，流程读取和人数核验另行对账；resolved仅人员解析，partial保留结果和缺口，not_found不等于零审批。仅complete/resolved退出0，其他退出2，始终读取输出文件。

## 默认：原始任务分页

`GET /open-web/web/clm-openapi/v1/tasks/getTasks`，同一`contract-test --as user`，成功为`success=true,code=0`，不同于CLM记录的code=200。

- `assignee_user_id`使用目录真实数字larkId。lookup/成员响应缺失时用真实employeeId调用getEmployeesByIds补查，不假定employeeId=larkId，不用openId/userId，也不能用只返回当前登录人的employee/info映射别人。
- 每人分别读取task_status=1已办、0待办，sort_type=DESC。人员和状态各自独立游标；已办last_sort_value=上页最后一条end_time，待办=create_time，配合其last_cursor。不得用update_time，保持长整数精度。
- 第一页可能只返回10条，后续才采用cursor_size。必须翻到空页，短页不代表结束；重复页、无进展、游标缺失和业务失败标partial。
- 完整保留task_records，再按已办end_time `[start,end)`选择实例；发现阶段保留全部待办，分析记录时再限定创建早于窗口截止时间。不用合同创建/提交日期预筛。BPM任务ID用于发现去重，下游按实例+任务ID核验；process_instances保留source_task_ids与真实businessId，类型从records读取。
- task_status=1可能含提交、取消或自动动作，不能全算审批通过。最后用records的节点、command、状态和时间判断。
- 所有max参数默认0不限量。--page-size只控制单页，不截断总量。
- --checkpoint-contract-id只验证正常查询是否发现指定合同，缺失标覆盖异常；不能人工插入伪装修复。
- --resume-file只接受同版本、同profile/source/窗口和分页设置，旧参与索引结果不能续跑成新原始任务完整结果。更换登录人/租户或刷新目录时重新查询。
- --candidate-source visible-contracts仅用于显式合同列表可见范围，全员只扫一遍且不加参与人过滤。participant-index只作为用户明确选择的快速候选，始终不保证历史审批完整。raw-tasks失败不能静默降级。

## 人员识别与部门范围

1. `employee_lookup` 用 keywords 查候选，保存 employeeId、姓名、状态、当前部门。使用 CLM 内部 employeeId，不使用 userId/openId。候选仅一人可直接采用并展示实际姓名（可能含括号昵称）；重名时展示候选及部门，得到用户选择后加 `--selected-employee-id <候选中的ID>` 重跑。不能拼接、推算或按姓名关联流程记录。
2. `department_lookup` 用 keyword 找部门，响应可能嵌套 childDepartments。只有名称匹配的节点是选择候选，子节点不能仅因随父节点返回就视为同名候选。多个候选时用 `--selected-department-id <候选中的ID>`。使用 departmentId，不从组织层级编码解析 ID，不替换为 openDepartmentId。
3. 默认包含所选部门及全部可见子部门。`department_members` 只返回该部门直属成员，根部门0人时仍遍历子部门；可加 `--direct-department-only` 仅直属。成员页码从1开始，每页50；对部门ID、人员ID去重并防循环。hasMore 可能多报，空页可结束；失败、重复页或达到边界保留已取得成员并标为部分。
4. `--max-member-pages`（每部门成员页数）和 `--max-departments`（部门数）默认均为0，表示不限量，自动遍历至结束。只有用户明确要求限制或试跑时才设置正数，负数拒绝。达到显式上限后，可取消或提高上限并重新执行原姓名/部门命令；成员范围的部分失败不能通过合同分页 resume 自动补齐。

员工查找中的 `noFilterDeletedEmployee=true` 不能保证按姓名搜出离职人员：当前实现只在 keywords 为真实数字 employeeId 时额外精确补离职员工。部门关键词查找中的 `showAllStatus=true` 也不能据此承诺历史停用部门全覆盖。保留接口实际返回的状态，不擅自排除；姓名未找到时说明查找限制，可用有来源的内部ID继续核对。部门成员是当前组织快照，不能还原历史在岗成员或过去部门归属。

## 显式快速候选：参与索引（不完整）

仅用户明确指定`--candidate-source participant-index`时，`participant_contracts`对每位employeeId单独POST；此节的分页完成只指索引分页，不代表实际任务查全：

```json
{
  "searchTabCode": 0,
  "pageIndex": 0,
  "pageSize": 50,
  "withStatistics": false,
  "enableHighlight": false,
  "viewTypeEnum": "PLAIN_LIST_VIEW",
  "conditionUnits": [],
  "filterUnits": [{
    "searchField": "APPROVE_AUTHORITY",
    "searchValue": "<真实employeeId>",
    "unionType": "MUST"
  }]
}
```

- searchValue 是单个 ID 字符串，后端仅取首值；不能塞多个员工数组后声称已查全部人员。跨人查询后按 `(businessTypeCode,businessId)` 合并，并保留 `matched_employee_ids`。
- 不附加“今年创建/提交/盖章”日期过滤：往年发起、今年完成的任务也应进入分析。“今年”最终按任务 endTime 筛选，范围为本年1月1日00:00至采集时刻，明确时区。
- 每页 `data.content` 是合同组，组内 `searchContractListItems` 是业务条目。以 `filteredTotalElements` 对照累计唯一 groupId；不能用条目数或 `totalElements`（各页签统计）代替组数。一个组内不同生命周期业务必须分别保留。
- 第一页 pageIndex=0；后续递增并原样回传 `data.searchAfter` 数组。Python JSON 保留大整数精度，不先用 JavaScript Number 解析、舍入或把游标值全部改成字符串。
- 组数达到总数才声明完成；总数0且空页也可完成。总数未达却空页、重复页、总数变化或缺少必要游标均标记不完整，不用短页单独判断完成。
- 默认自动取完每位人员的可见候选分页，不设200份业务或20页等隐含上限。`--max-businesses`（每人累计唯一业务数）和 `--max-pages`（每人累计合同页数）均默认0，表示不限量；只有用户明确要求限制或试跑时才设置正数，负数拒绝。异常、重复页或游标缺失仍会中止并标记部分结果。显式业务限制发生在页中时，未消费条目保存在 continuation，不丢弃；已请求页数与游标也保存。报告仍须披露覆盖数量和缺口。

例如上次因显式上限截断，保留旧文件并使用默认不限量继续：

```bash
python3 <skill-directory>/scripts/discover_businesses.py \
  --resume-file <discovery.json> --candidate-source participant-index \
  --page-size <与原请求相同> --profile contract-test \
  --output <discovery-continued.json>
```

resume 复用上次人员快照及已取合同；原请求若提供start/end，续跑也必须原样提供。上述示例专用于显式participant-index；raw-tasks或visible-contracts续跑应保持对应来源。省略限额参数或显式传0会取消旧限额；若再次指定正数，限制是累计值而非每次新增量，例如 `--max-businesses 500 --max-pages 40`。上次因授权/网络失败可在恢复同一授权后重试；游标缺失、索引总数变化等结构问题须重新执行原查询并披露快照变化，不拼造游标。只用同一 profile 的本地结果恢复；切换登录用户/租户或要刷新组织快照时重新查询。

## 接入原有效率统计

1. 默认将process_instances交给collect_records.py，只有显式合同模式用businesses。仅有真实实例ID时records接口可省略businessId/type，由服务端校验流程读权限并返回record真实类型，再补名称/编号。保留权限失败和缺页，将source_task_ids与records对账；不能把未取得记录当零。
2. 将 employees 作为当前人员/部门快照，按 employeeId 与任务处理人精确匹配。对所有 approver_ids 重复传分析器 `--approver-id`；部门含子部门时也以这份人员集合限定范围，不额外只传父部门 `--department-id` 导致子部门成员被漏掉。
3. 业务详情的记录可能包含其他参与人，必须继续执行人员过滤。`matched_employee_ids` 仅说明搜索命中来源，不能证明某一任务的操作者。
4. 先判断任务是否实际完成及有效处理命令，再按指定年份/时间段筛选 endTime；审批、盖章、归档分列。输出平均自然用时及逐任务明细，不把自动跳过、待处理或缺时间记成零耗时。
5. 将本次查询范围、当前部门口径、候选数量、详情可读数量、流程记录覆盖、分页状态写入 JSON 诊断。面向用户的 Markdown 和 Excel 只展示统计周期内跨节点去重的合同数；任一层失败都不能称全量，缺权限保持简短缺口提示，不替换身份绕过。
