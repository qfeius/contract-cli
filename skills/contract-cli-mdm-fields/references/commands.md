# Schema Commands Reference

```bash
contract-cli mdm fields list --profile contract-group --biz-line vendor
contract-cli mdm fields list --profile contract-group --biz-line legal_entity
contract-cli mdm fields list --profile contract-group --as user --biz-line vendor_risk
contract-cli mdm fields list --profile contract-group --as bot --biz-line vendor --user-id-type employee_id
contract-cli mdm fields list --profile contract-group --as bot --biz-line legal_entity
```

说明：

- `mdm fields list` 会按身份路由：
  - `user` -> `contract/v1/mcp/config/config_list`
  - `bot` -> `mdm/v1/config/config_list`
- bot 后端当前只接受 `vendor` / `legalEntity`；CLI 会把 bot 下的 `legal_entity` 映射为 `legalEntity`
- `vendor_risk` 仅适用于 user/MCP 路径
- 常用于后续写操作前确认字段结构；`api call` 当前暂未开放，不作为兜底入口
