# 审批矩阵人员搜索

命令：`contract-cli rule employee search`，兼容入口：`contract-cli approval-matrix employee search`。

接口：`POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/employees/search`。这是只读搜索，不创建人员、不修改矩阵。

```bash
contract-cli rule employee search --profile contract --as user --product-id contract --group-id approve_matrix --input-file employee-search.json --output json
```

请求文件：`{"param":"赵少帅","group_code":"approve_matrix"}`。`param` 必填，非空字符串，最多 100 字符；`group_code` 可选，传入非 null 值时须为 1–30 字符的非空白字符串且与 `--group-id` 一致，CLI 和后端均校验。省略或 null 兼容旧请求，作用域仍取路径；不发送 `tenant_id` 或 Cookie。支持 user/app，user 沿用合同规则管理权限；使用当前明确选定的 profile 和租户，不自动换身份。

返回 `code/msg/data`，`data` 是候选数组：

```json
{"employee_id":"7113921696628736004","name":"赵少帅","email":"example@example.com","department_name":"研发部","status":1,"selectable":true}
```

- `employee_id` 是正整数外部 ID 的十进制字符串，对应规则行 `employee_collection` 和结果列 `default_value`，不是页面内部主键。批量计划可接收字符串，发往行接口时必须保持 int64 精度。
- `user_id_type` 仅支持 `user_id`；不支持 `open_id`。此接口不进行网关人员 ID 二次转换。
- `status`：1 正常、0 删除、2 停用、-1/null 未知。仅状态为 1 且 ID 映射有效时 `selectable=true`。缺少映射时 `employee_id=null`，不可用内部 ID 代替。
- 0 个候选：告知未找到，追问更准确姓名或其他识别信息；多个同名候选：展示姓名、部门、邮箱、ID 后选择，不自动取第一人。
- 搜索复用当前产品的目录源，不做额外人员创建；关键词匹配范围/返回数量沿用底层目录服务，不承诺全量分页枚举。
- 遇到权限失败、接口未部署或服务异常时停在人员解析阶段，说明实际错误；不要转用 SaaS Cookie 接口。
- 人员明确后回到矩阵列配置和行写入计划。指定规则的审批人应写在该规则行中，不擅自设为所有未命中情况的默认审批人；默认不发布。
