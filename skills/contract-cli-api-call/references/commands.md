# API Call Reference

## 通用开放平台调用

```bash
contract-cli api call GET /open-apis/mdm/v1/vendors/1063197165850985296 --profile contract-group
contract-cli api call POST /open-apis/mdm/v1/vendors --profile contract-group --input-file vendor.json
contract-cli api call POST /open-apis/mdm/v1/vendors --profile contract-group --data '{"vendor":"V001"}'
```

## `contract/v1/mcp` user-only 调用

```bash
contract-cli api call GET /open-apis/contract/v1/mcp/vendors/1063197165850985296 --profile contract-group
contract-cli api call POST /open-apis/contract/v1/mcp/contracts/search --profile contract-group --input-file contract-search.json
```

说明：

- 第二组命令默认按 `user` 身份解析
- 若显式写 `--as bot`，CLI 会在本地直接报错
