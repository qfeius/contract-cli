# 原生接口与字段衔接

## 通用入口和响应

所有页面请求使用同一已确认 profile、实际环境、当前租户及 USER，通过现有 `contract-cli api call --channel web --profile PROFILE --as user --output json` 调用。实际 CLI 帮助必须支持这些参数；请求路径只用下表相对 path，不拼域名/query，不构造身份覆盖头。

CLM 页面业务成功要求根 `code` 为整数 200、`success` 严格为 true，再核本步 data。CLI 退出 0、HTTP 200、消息文字或开放平台其他 code 都不代替它。业务错误保留 msg/requestId 和本阶段状态。

GET/query 参数使用独立 `--query key=value`，POST JSON 使用宿主普通 JSON 文件的 `--input-file`。publish 是 POST 但参数仍是 query，无 body。JSON 字符串、布尔、数值类型按本页保留，不使用显示编号或姓名代替内部标识。

## 身份

| 方法/path | 参数 | 消费结果 |
|---|---|---|
| GET `/open-web/web/clm/employee/info` | 无 | `data.employeeId`、`data.tenantId` |
| GET `/open-web/web/clm/sso/getUser` | 无 | `data.employeeId`、`data.tenantId`、`data.tenant.tenantId`、`data.systemConfigVO` |

成功后交叉核对两个 employeeId、三个 tenantId 均为一致的十进制字符串。标识缺失或冲突时停止受影响步骤，不猜别名。当前配置用于说明变更类型及后台开关，不请求其他用户资料来确定调用者。

## 当前本人功能权限

GET `/open-web/web/clm/permission/function/list`，无 query、无 body。该接口从当前 USER 上下文取得租户和人员，不接受 `tenantId/employeeId` 参数来查询另一身份。成功 `data` 为 `FunctionAuthorityVO[]`，本步消费 `data[].code`；其中 `authorityId` 是字符串，另有 `name/description/content/authorityTypeCode/authorityTypeName`，不以名称或 content 子串判断权限。

在流程枚举前，按通用响应规则核实 envelope 和列表结构，要求存在 `code` 严格等于 `admin` 的项。响应失败、结构未知或不含该 code 时停止本业务流程枚举及后续步骤，保留权限核实结果；不凭角色名称、管理员标记或其他页面操作项放行。

| 页面权限项 | 实际 code 与本业务关系 |
|---|---|
| 系统管理 | `admin`，本业务 form 列表/详情/edit/update/publish 及审批人表达式生成 API 所要求的后台功能权限 |
| 合同流程管理 | `contract_flow_manage`，角色树中的另一项，不代替 `admin` |
| 对可查看合同发起补充协议 | `operation_by_other_for_contract_change`，合同实例操作权限，不代替 `admin` |
| 对可查看合同发起终止协议 | `operation_by_other_for_contract_terminate`，合同实例操作权限，不代替 `admin` |
| 对可查看合同发起修改表单 | `operation_by_other_for_contract_change_info`，合同实例操作权限，不代替 `admin` |
| 为可查看父合同创建子合同 | `operation_by_other_for_contract_framework`，合同实例操作权限，不代替 `admin` |

本人功能权限由系统按当前身份计算，可能合并角色或采用原系统降级规则；本步只核实实际返回的有效 code，不反推出单个角色的完整授权。之后每个业务 API 的实际鉴权仍须成功，不能以查询过 `admin` 绕过拒绝。

## 流程枚举与详情

GET `/open-web/web/clm/form/listAllProcessInfo`：

| query | 本业务发送值 |
|---|---|
| `type` | 分别显式查询 0、2、16、3、10、12、15，对应后台六类业务，变更含两种编码 |
| `published` | 每个类型分别 false、true，两视图按交互规则去重 |
| `editFormBusinessTypeCode` | 12，保持原生管理列表计算口径，不当目标分类 |

消费 `data.businessTypeModuleList[]` 中所请求类型的 `fromDefinitions`。它不是分页接口；不制造 page/token。完整成功响应中的空数组/null 可以是无结果，缺失模块或结构不能一概当空；已知 DTO 可能重复返回同类型模块，只有关键字段一致才去重。

列表字段：`formId`、`formName`、`businessTypeCode`、`categoryScope`、范围显示名、`published/preset/publishTime/updateTime`。每行“流程分类”以实际 `businessTypeCode` 为依据，显示映射、分组顺序与全表编号只在[交互规则](interactions.md#列表格式)维护；`categoryScope` 及其范围名称不是这个分类。列表与定义的类型编码冲突时保留待核实，不按名称补猜。节点查找不能只靠列表；对每个真实 ID 调 GET `/open-web/web/clm/form/getFormDefinition`，query 仅 `formId`，无 body。

定义必需消费：`formId/formName/businessTypeCode/published`、原范围、`processDefinitionKey`（版本候选分组用）、`processDefinitionJson`、保存阶段所需完整字段。标识为字符串，类型为明确整数；null/false/空类型不当 0。

只读 GET 不创建编辑副本。枚举未覆盖历史全集、14/17 等非本业务入口，不能把这些排除范围说成已扫描无命中。当前草稿关系规则只在 interactions.md 维护。

## 人员定位

### 当前页面姓名候选

GET `/open-web/web/clm/employee/findEmployeesByKeywords`，这是当前流程选人组件仍调用的原生入口；虽有旧接口标记，不能改走未核实契约。query：

| 字段 | 发送策略 |
|---|---|
| `keywords` | 用户提供的非空姓名关键词，原样通过独立 query 承载 |
| `includeInvalid` | 新人员明确 false；旧人员不以此参数承诺能搜到离职员工，默认 false |
| `filterCurrentEmployee/filterEmployeeIds/filterVisibleEmployee/needQueryRoleInfo` | 本任务不主动附加，沿用流程选人组件的普通姓名搜索分支，不请求额外角色信息 |

成功 `data` 为 `EmployeeVO[]`。这是关键词候选，不是全租户穷举；接口无已核分页协议，不制造分页参数，也不凭返回一条就宣称租户内全局唯一。显示部门/状态，取得与用户业务信息一致的准确身份；必要时让用户选择。

当前响应的 `employeeId` 是后台内部 ID 字符串。`userId` 是飞书用户 ID；`openId/accountId/larkId/memberCode` 语义也不同。可选复用已有 `employee list` 得到的 `user_id`，但须在同一人员 VO 上精确匹配 `userId` 后才能取 `employeeId`，不可转换或猜测；该可选入口的候选与分页仍按其已安装 Skill 执行。

### 已知流程人员 ID 回读

GET `/open-web/web/clm/employee/getEmployeesByIds`：`employeeIds` 是从已获准流程收集的真实内部 ID 英文逗号串，`extraDepartmentInfo=true` 用于部门路径，无 body。返回成功 `data: EmployeeVO[]`；按 employeeId 对照请求集合，不能按数组位置关联。按可承载查询大小分批，同一 ID 结果冲突停止；未返回的 ID 保留未解析，不补造人员。

该读取没有正常/离职状态过滤，可用于旧人已离职时从配置 ID 回读身份。普通姓名查询为空不能证明旧人未配置；`noFilterDeletedEmployee=true` 的另一姓名接口只支持内部 ID 精确补查，不能拿它承诺离职人员姓名搜索。

旧人可以非在职，但必须确认就是要替换的身份。新人员必须 `status=1`，且 `canSelect` 若明确 false 则拒绝；0/2/-1、缺失或未知状态不当在职。姓名用 `displayName` 非空时优先，否则 name；部门从 departments 展示名称/已有祖先路径。写前按实际内部 ID 重新核实新人。

仅按 ID 回读旧流程中的人员，不能借此扫描无关租户目录。旧人仍无法解析或同名未消歧时，保留输入，不能向用户索要技术 ID 来规避定位。

## 取得编辑目标

POST `/open-web/web/clm/form/editFormDefinition`，无 query；JSON 仅 `formId`，其值为本批已确认源定义的内部 ID 字符串。本业务不发送 reminderRuleId。

成功 `data` 是完整可编辑定义 VO。未发布对象可能原地规范化并保存；已发布对象可能新建后继，也可能返回既有未发布后继。必须重新取实际返回 `formId`，核其未发布状态、类型、节点映射和当前配置。后续保存、按需发布和回读都用该目标。返回未发布草稿只说明写入对象，执行方式继续沿用编辑前核实的源版本状态和用户选择。

edit 会规范化模块/适用流程；新副本保留图、节点配置及版本组，部分 extraInfo 按原系统规则重建。不能对陌生草稿、额外业务差异或未核实的继承行为作“完全无变化”保证。异常按主流程停在实际阶段，不自动清理。

## 生成变更节点审批人表达式

POST `/open-web/web/clm/expression/approval/generate`，无 query；body 为目标节点修改后完整 `approver` 对象。其原 snake_case 字段通过 DTO 别名支持，可直接使用：`personnel_type_code`、`employee_ids`、递归 `approval_rule_list` 等结构及未修改字段。只改 code 0 叶子人员，不从头重建多种审批人规则。

这是当前页面依赖的只计算接口，服务端需后台功能权限；不创建流程记录。成功 `data` 须为非空字符串，将其写入同一 node_id 的 `approvers_bo`；失败、非字符串或空值停止该流程保存，不能照前端吞空分支继续提交。一次计算不等于 update/publish 已成功。

所需别名与保留边界见 node-rules.md；不添加自造 formId 字段作为定位协议，也不把返回表达式当完整节点配置。

## 全量保存

POST `/open-web/web/clm/form/updateFormDefinition`，无 query；body 以 edit 返回的完整当前可编辑快照为基线。保存不是局部 patch，尤其 moduleInfos 为空会重建空模块，不能只给命中节点或处理 JSON。

| DTO 字段 | 来源与处理 |
|---|---|
| `formId` | edit 返回实际目标 ID，十进制字符串 |
| `formName` | 同目标 `.formName`，不借换人改名 |
| `categoryScope` | 同目标原字符串，保留 0/-1/空值的既有业务含义，不从范围名重建 |
| `processScope` | 同目标的完整字符串 ID 列表；DTO 语义为 Long 列表，但不经 Number，保留原 `-1` 等兼容值 |
| `outbounds` | 同目标对应完整嵌套结构 |
| `validations` | 完整保留，含 processNodes |
| `outboundValidations` | 完整保留，含 processNodes |
| `approverOutbounds` | 完整保留 |
| `moduleInfos` | 同目标完整后端模块树，不能替成前端 fieldNodes，也不能省成空数组 |
| `processDefinitionJson` | 目标完整原 JSON 内仅修改已确认人员、必要缓存及表达式，再序列化为 JSON 字符串，不是对象 |
| `approvalEmptyStrategy` | 同目标 `approvalEmptyStrategyCode`，不从同名不存在字段读取，不用真值默认掉合法 0 |
| `approvalDuplicateStrategy` | 同目标 `approvalDuplicateStrategyCode`，保留实际值 |
| `assignApprovalEmployeeIds` | 同目标 `assignApprovalEmployees[*].employeeId` 完整列表，ID 保持字符串；该全局兜底列表不属于本次替人范围 |
| `nodeConfigMap` | 同目标完整 map，保留权限、自选和排序 |
| `formDescription` | 同目标原字符串，不重复 stringify |
| `aiSummaryConfig` | 同目标对应配置，存在时完整保留；原 null 时保持 null，后端此时不更新 |

这些字段必须有实际来源，不能因字段缺失而猜 null、0、空数组。全局转交策略需要的人员无法完整取得时停止该对象。原响应明确为 null/空列表的可选项按原值保留；不能用缺字段冒充明确空值。

`layoutInfo/extraInfo/assignApprovalEmployeeId` 在 DTO 中虽有声明，当前实现未消费，不能靠发送它们承诺保留一切服务器状态。本业务不以整个 VO 透传替代上表，也不丢弃原快照；edit 已发生的系统变动与未知字段保留/复核边界照主流程处理。不能假定修改 `extraInfo` 能覆盖 AI 配置，实际使用上述 `aiSummaryConfig` 原生字段。

发送前确认 JSON 转义、真实 ID、集合、布尔/整数类型及实际 UTF-8 字节符合运行方式。保存成功要求 envelope 成功且 `data=true`，之后 GET 实际目标并按节点不变量及其他保存字段做业务回读。仅保存分支回读通过即完成，响应成功但回读失败时保留“保存响应成功，回读待核实”。

当前实现可能在保存后的模块回读中，将属性 `attributeOrder` 显示为数组内连续序号或 null；原生模型保存的是 `attributeIndex`，差异检查按原数组补序号且可能提前结束。仅当本轮已核实这是同一实现行为，并逐项确认模块、字段、组的完整数组顺序及其余所有配置均一致时，才能将仅这一序号表示差异记录为已解释的系统规范化。不能一律忽略排序字段、对数组重排后比较或跳过其他模块差异。发布会合并当前字段元数据，须重新比较发布后的完整配置，不提前承诺这些序号一定恢复。

## 发布与结果

只有[保存与发布选择](interactions.md#保存与发布选择)确定为保存后发布，并且保存回读通过，才调用 POST `/open-web/web/clm/form/publishFormDefinition`。仅 query `formId=实际编辑目标ID`，无 JSON body。发布前再次 GET 对照已保存且已核验快照，漂移则停止该对象。

成功 envelope 后还需 `data.result=true`。data 可能包含 `changed/multiLangExist/errorNodeIds/errorFields/previousId`，不含发布 taskId。`changed=false` 或 result=true 都不能代替实际目标已发布的回查；方法名含 Async 也不能据此制造轮询任务接口。

`code=110355` 表示发布校验失败时，读取实际 `data.errorNodeIds/errorFields` 和消息，映射到节点或字段，保留已保存草稿。其他失败保留真实业务结果，不只凭 code 推导更细原因。

发布完成必须同时：GET 目标得到 `published=true`；目标选中节点/规则按预期替换且其他配置不发生未解释变化；同类型 `published=true` 完整列表包含该实际目标为当前版本并范围一致。若失败或出现不一致，不自动重复 publish。

`multiLangExist=true` 需提示原系统多语言处理要求，不自动重置译文。模板绑定更新为异步，当前接口无逐模板完成证据；报告流程定义发布与回查结果，不宣称模板已全部同步或存量/在途合同人员已迁移。
