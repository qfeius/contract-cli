# Vendor Commands Reference

```bash
contract-cli mdm vendor list --profile contract-group --name "供应商A"
contract-cli mdm vendor list --profile contract-group --name "供应商A" --page-size 20 --page-token next
contract-cli mdm vendor list --profile contract-group --as bot --name "V00000001" --page-size 20 --user-id-type employee_id
contract-cli mdm vendor get 1063197165850985296 --profile contract-group
contract-cli mdm vendor get 7003410079584092448 --profile contract-group --as bot --user-id-type employee_id
```

说明：

- `mdm vendor list` 会按身份路由：
  - `user` -> `contract/v1/mcp/vendors`
  - `bot` -> `mdm/v1/vendors`
- `mdm vendor get` 也会按身份路由：
  - `user` -> `contract/v1/mcp/vendors/{vendor_id}`
  - `bot` -> `mdm/v1/vendors/{vendor_id}`
- 默认输出可直接用于查候选方或确认详情
- 需要查看原始响应时可加 `--raw`
