# Entity Commands Reference

```bash
contract-cli mdm legal list --profile contract-group --name "上海主体"
contract-cli mdm legal list --profile contract-group --name "上海主体" --page-size 20 --page-token next
contract-cli mdm legal list --profile contract-group --as bot --name "主体A" --page-size 20 --user-id-type employee_id
contract-cli mdm legal get 7023646046559404327 --profile contract-group
contract-cli mdm legal get 7003410079584092448 --profile contract-group --as bot --user-id-type employee_id
```

说明：

- `mdm legal list` 会按身份路由：
  - `user` -> `contract/v1/mcp/legal_entities`
  - `bot` -> `mdm/v1/legal_entities/list_all`
- `mdm legal get` 也会按身份路由：
  - `user` -> `contract/v1/mcp/legal_entities/{legal_entity_id}`
  - `bot` -> `mdm/v1/legal_entities/{legal_entity_id}`，并额外带上 query `legal_entity_id`
- 常用于合同创建前选择我方法人主体
