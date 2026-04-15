# Vendor Commands Reference

```bash
contract-cli mdm vendor list --profile contract-group --name "供应商A"
contract-cli mdm vendor list --profile contract-group --name "供应商A" --page-size 20 --page-token next
contract-cli mdm vendor get 1063197165850985296 --profile contract-group
```

说明：

- 这些命令面向 `contract/v1/mcp/vendors`
- 默认输出可直接用于查候选方或确认详情
- 需要查看原始响应时可加 `--raw`
