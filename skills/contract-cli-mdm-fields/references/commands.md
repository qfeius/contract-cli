# Schema Commands Reference

```bash
contract-cli mdm fields list --profile contract --biz-line vendor
contract-cli mdm fields list --profile contract --biz-line legal_entity
contract-cli mdm fields list --profile contract --as user --biz-line vendor_risk
contract-cli mdm fields list --profile contract --as app --biz-line vendor --user-id-type employee_id
contract-cli mdm fields list --profile contract --as app --biz-line legal_entity
```

说明：

- `mdm fields list` 会按身份路由：
  - `user` -> `contract/v1/mcp/config/config_list`
  - `app` -> `mdm/v1/config/config_list`
- app 后端当前只接受 `vendor` / `legalEntity`；CLI 会把 app 下的 `legal_entity` 映射为 `legalEntity`
- `vendor_risk` 仅适用于 user/MCP 路径
- 常用于后续写操作前确认字段结构；`api call --channel web` 只执行已验证业务配方的 Open Web 接口，本字段查询 Skill 不提供通用写入配方
