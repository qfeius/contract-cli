# 执行与采集

先读取 api-contract.yaml 的 `runtime.execution_mode`。本包为 `HYBRID`：CLI 可用且满足版本时，业务调用统一用 `contract-cli api call --channel web --profile contract-test --as user --output json`；只有 CLI 不可用或不兼容时，才检查符合 `universal_open_web_call` 统一契约的 MCP `call-open-web-api` Tool。两种通道都按 recipe 映射方法、Open Web 路径、query、普通业务 header 与 body。本版只读，不办理审批或改变流程。ID 始终保持字符串；需要保真的游标使用原始 JSON，不经浮点或 JavaScript Number 转换。

CLI 是本版 test 真实运行验证通道。MCP 仅在目标环境取得真实 Schema，并对本次必需 recipe 同时验证传输、HTTP 和业务成功后才能用于正式统计；业务专用 MCP Tool 不能替代通用 Open Web 调用器。MCP 采集时由 Agent 按 api-contract.yaml 生成与脚本输入相同的 discovery/bundle JSON，再运行本地补证、对账与分析脚本；不得让只支持 CLI 的采集脚本暗中切换通道。

客户体验环境只允许使用上述 CLI user 接口及本 Skill 的本地分析脚本。禁止调用 MySQL、PostgreSQL、只读业务库、内部数据库 MCP、日志查询平台或后台直连接口补齐数据；当前工具列表即使出现这些能力也不构成授权。CLI 返回权限、分页或读取缺口时保留 `partial` 并简短披露，不切换到内部数据源。

## 授权与窗口

`contract-cli auth status --profile contract-test --as user` 检查授权。profile 不存在时，在用户已授权 test 试用的范围内执行 `config add --env test --name contract-test`，再走 CLI 的 auth init / 用户操作后 auth complete。不要改生产 profile，不索要凭证。

默认最近 30 个完整自然日：报告时区今天 00:00 为 end，向前 30 天为 start。用户说“9月1日至20日”，end 为21日00:00，纳入20日全天。“今年”取本年1月1日00:00至实际采集时刻；参与合同发现不加合同创建年份过滤。

## 业务发现与记录

优先用户明确合同/合同组，或已知业务对应的流程实例。有姓名或部门条件时，先按 [人员与原始任务发现](discovery.md) 解析人员；已有业务清单用`--resolve-only`，否则按真实目录larkId完整分页读取已办和待办。不得以APPROVE_AUTHORITY作为统计必需条件。

```bash
python3 <skill-directory>/scripts/collect_records.py \
  --discovery <discovery.json> --start '<带时区开始>' --end '<带时区结束>' \
  --output-dir <采集目录> --analysis-subject people --with-definition-metadata

python3 <skill-directory>/scripts/collect_participant_evidence.py \
  --input <采集目录/bundle.json> --output <补证bundle.json> \
  --cache-dir <人数证据缓存目录>

python3 <skill-directory>/scripts/reconcile_records.py \
  --discovery <discovery.json> --bundle <补证bundle.json> \
  --output <任务记录对账.json>
```

collector默认刷新，失败缓存从不复用；可用显式--cache-max-age短期复用成功结果。仅有原始任务实例ID时，records只传processInstanceId，由服务端校验当前用户流程读权限，从真实record取得业务ID与类型，再补合同名/编号。metadata保留人数证据所需最小formId、实例/定义标识、提交人和当前动态配置状态；当前配置不能冒充历史快照，补证必须额外匹配真实历史实例和已发布固定表达式。原始source_task_ids必须与取得的records对账，缺失列缺口。分析器输入使用补证后的bundle。

对账脚本按实例和任务ID核对窗口内已办及全部发现待办，记录缺失、时间差异和窗口标记冲突。它不读取分析报告，不把提交等非审批任务计入审批效率；统计结果还须按JSON明细的计入标记复核。仅原始任务发现模式使用上述对账；显式业务或ES备用模式缺少原始任务清单时，不能把零条对账宣称为任务来源完整。

只有未指定业务、姓名或部门时，才考虑 `efficiency_contracts` 发现看板样本，并明确其盖章日期窗口。默认取完当前权限可见分页，不设业务数或页数上限；仅用户明确要求试跑或限量时设置正数边界。人员分页与断点恢复见 discovery.md；异常中止仍披露缺口，不保证租户全量任务。

效率列表从 pageIndex=0、pageSize=50 开始，按 `(businessTypeCode,businessId)` 去重。hasNext可能为空、totalPages可能多一页，应结合实际content、累计唯一数量和totalElements判断。空页结束；重复整页或数量不一致时标缺口，防止无界循环。startDate/endDate 为明确时区日界的epoch毫秒；这一步的日期口径不能替代任务结束时间筛选。

业务类型10/17走 contractAssembly/info，并保留 assemblyId；该路由尚未运行验证。普通详情支持0/1/2/3/9/11（12虽代码支持但常规搜索排除）；目前运行样本验证0/2/3。其他类型先核对契约，不因“不是10”就当普通合同。详情中 processInstanceId 为空的草稿记为无流程，不记0耗时。

详情关闭额外企业查询、名称生成和组子项读取，参数见契约。使用返回的真实processInstanceId读取process_records，不能用合同/分类ID替代。详情已包含完整approveRecord时可提取复用，记录来源，无需重复请求。历史任务逐条使用自身定义/实例ID。普通详情遇110002系统错误且缺流程实例时，可用 `contract_brief_info` 取得真实 processInstanceId，再调用记录接口；110004无权限不走替代接口绕过。brief返回空值时标记未取得流程，不能说无任务。共享流程实例需全局按任务ID去重。

## 请求确认

时间相关请求加 `--header 'Timezone-Offset: -480'`，首次非空样本按metrics.md验证时区透传。

```bash
contract-cli api call --channel web --profile contract-test --as user --output json \
  GET /open-web/web/clm/workflow/runtime/process/records \
  --query 'businessId=<业务ID>' --query 'businessTypeCode=<真实类型>' \
  --query 'processInstanceId=<详情中的ID>' --header 'Timezone-Offset: -480'
```

同时检查CLI退出状态及下游success/code。CLM目录、详情、records、历史表单接口成功为success=true、code=200；CLM_OPENAPI原始人员任务及实例详情为success=true、code=0。CLI退出0本身不足以证明业务成功；分别按配方核验，不把失败改成成功envelope。

仅提取报表必需的任务字段及姓名、部门。不要保存合同正文、ticket、联系方式等无关内容；临时响应放任务工作目录，不打入Skill包。

## 分析 bundle

结构示例没有真实数据；ID与businessTypeCode均需来自真实响应，不能照抄示例作为默认。

```json
{
  "meta": {
    "start": "2026-09-01T00:00:00+08:00",
    "end": "2026-09-21T00:00:00+08:00",
    "timezone": "Asia/Shanghai",
    "timezone_confirmed": true,
    "timezone_source": "Timezone-Offset=-480；与0偏移对照已确认",
    "analysis_subject": "people",
    "coverage": "用户选定合同及服务返回历史流程；不是租户全量",
    "environment": "test",
    "collected_at": "实际采集时间"
  },
  "employees": [],
  "businesses": [],
  "node_participant_evidence": [],
  "instances": [{
    "businessId": "真实业务ID",
    "businessTypeCode": 0,
    "processInstanceId": "真实实例ID",
    "response": {
      "success": true, "code": 200,
      "data": {"done": [], "todo": [], "forecast": []}
    }
  }]
}
```

示例employees=[]只是结构占位，实际必须写入完整目标人员清单；目标确实无人时保留空数组。businesses保存真实业务ID/类型及合同名/编号。node_participant_evidence由补证脚本核验后填写：历史实例、定义、节点严格匹配，固定单提交人/固定指定单人配置，加全部原始任务参与人和变更动作检查；缺条件仍标待确认，不能不执行补证就输出全零统计。

response是已验证成功的流程记录API envelope。如源于详情，明确提取data.approveRecord，沿用已验证成功字段。node/record映射见契约。人员部门优先复用发现结果的 employees 当前快照，按 employeeId 补齐。仍缺失时可用样本中的真实 ID 调用 employees 配方（该配方尚未运行验证），不按姓名关联。报告注明当前组织口径。

```bash
python3 <skill-directory>/scripts/analyze_records.py \
  --input <补证bundle.json> --markdown <report.md> --json <report.json> \
  --details <审批效率明细.xlsx>
```

过滤可重复传 `--approver-id <ID>`、`--department-id <ID>`、`--process-definition-id <ID>`、`--node-type 1|2|3`。同维度多ID为并集，跨维度共同约束。输出仍只反映已取得样本。姓名／部门分析需把发现输出的每个 approver_ids 传为 --approver-id；部门包含子部门时，以解析出的人员集合限制，不再只传父部门ID。将 discovery.coverage 与详情/记录读取缺口一起写入 bundle.meta.coverage。

明确询问一人或多人时，设置 bundle.meta.analysis_subject=people，并传 `--analysis-subject people`；按部门传 `department`，按合同、流程或其他范围传 `other`。命令行值优先于bundle。具体人员模式不渲染总览、效率洞察或编号章节；目标人员只有一人时三个节点合并为一张表，目标人员两人及以上时按节点渲染三张表。按部门提问时，即使部门当前仅一名成员，也不能改成people模式。

## 交付与证据

固定交付统计周期与范围、统计周期内跨节点去重的合同数、个人效率表、Excel明细（失败CSV）。面向用户的 Markdown 和 Excel 不展示原始任务量、候选合同量、流程记录读取量；这些采集计数仅留在 JSON 诊断中。存在接口、权限或对账缺口时只做不带过程计数的简短提示。单人查询将审批/盖章/归档合在一张表并显示节点类型，多人查询按三个节点分表。部门或其他范围额外展示总览表及洞察；具体人员模式不展示总览、洞察和编号章节。个人表不含人员ID，完整清单逐人补零；必须附实际生成的明细文件链接。JSON仍保留overview等完整结构用于对账，Markdown按analysis_subject路由展示。

每条真实验证记录保存recipe、环境、脱敏trace/request ID及空/非空状态。只有实际运行后才能更新API契约readiness；单个端点成功不意味着其他端点成功。合成测试只能证明计算行为，不能冒充审批结果或线上验收。

## 采集字段与导出

组装bundle不得先筛掉其他人员、窗口外任务或自动记录，并须把发现结果的task_records原样带入分析bundle。父节点保留nodeId/nodeName/nodeDisplayName/nodeTypeCode/processInstanceId/taskModeCode/assigneeEmployees/records；record保留processDefinitionId/processInstanceId/processTaskInstanceId/parentProcessTaskInstanceId/parentCommandTypeCode/businessId/businessTypeCode/nodeId/nodeName/nodeDisplayName/assigneeEmployees/agentEmployees/endorsementEmployees/commandTypeCode/taskStatusCode/createTime/endTime。只移除无关正文、附件、联系方式和认证材料，不能删除业务归属、任务中心人员归因或人数证据字段。

原始任务中心按流程实例ID+任务ID连接流程记录，`discovered_for_employee_ids`用于个人归因；多人节点中可明确归因的有效人工任务正常计入实际处理人。补证使用listAllApprovalEmployees、CLM_OPENAPI历史实例以及与实例definition精准匹配的form定义，作为没有原始个人任务时的回退和审计。父模式不证明人数。历史任务用record业务归属优先；名称/编号按真实业务键关联。

分析器自动调用export_details.py，优先已有Codex artifact-tool生成XLSX；也可单独执行：

```bash
python3 <skill-directory>/scripts/export_details.py \
  --input <report.json> --output <审批效率明细.xlsx>
```

可传--node/--node-modules指向已有运行依赖。缺依赖或生成失败自动回退同名CSV并返回fallback_reason，不静默下载依赖。按当前环境的表格技能要求执行导出标记及验证。分发包不携带本机node_modules，Python CSV兜底无需外部依赖。字段、时间与单位见SKILL.md。
