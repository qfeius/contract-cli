# 合同交接参数说明核对

本轮仅修改 contract-handover 的说明与对应测试，没有修改其他 Skill 或 CLI 业务逻辑。

发现并修订：类型码分散且无完整对照；参数类型缺失；“必填”混用接口必填与配方策略；conditionUnits 缺少子字段；filterUnits 无已证实的结构；各 ID 与分页体系易混淆；接收人数组绑定、空值处理、默认值和隐式参数解释不足。

运行参考统一使用“参数、位置、类型、本流程要求、值/来源/限制”；明确这些表是当前工作流的完整请求说明，不是底层接口全部可选字段清单。filterUnits 因结构证据不足明确省略，不能据此退化为无条件查询。

核对来源为本地 CLM c651c0621faa6d1a102cdfdfccd64a6efb968c3d 的 BusinessType、BusinessResourceDTO、BusinessResourceHandoverDTO、EmployeeController、ContractAssemblyController、ContractHandover、ContractAssemblyHandover，以及归档契约中已验证的网关映射、搜索参数和响应字段。此静态核对不代表部署环境已使用该版本或新增业务类型通过运行验收。

枚举完整对照包含 0～26 和 99。后端父组分支按 10/17 选择、普通分支排除组成员 11，仍按实际实体与权限、状态判断；不从“枚举存在”推导“可执行”。Skill 的受控验证范围保持独立说明，未验证类型不新增写入资格。

验证：文档分发/引用、类型表、完整请求示例与搜索作用域测试先红后绿；实际业务请求未执行。
