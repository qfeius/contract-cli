# Schema Commands Reference

```bash
contract-cli mdm fields list --profile contract-group --biz-line vendor
contract-cli mdm fields list --profile contract-group --biz-line legal_entity
contract-cli mdm fields list --profile contract-group --biz-line vendor_risk
contract-cli mdm fields list --profile contract-group --as bot --biz-line vendor --user-id-type employee_id
```

说明：

- `mdm fields list` 会按身份路由：
  - `user` -> `contract/v1/mcp/config/config_list`
  - `bot` -> `mdm/v1/config/config_list`
- 常用于调用 `api call` 或后续未封装写操作前先确认字段结构
