# 连接器与命令契约

本文件记录当前 `contract-cli` 对两个合同摘要开放接口的稳定映射。两个命令均只支持 user OAuth，当前构建仅支持 prod。

## 合同摘要查询

- 命令：`contract-cli contract summary query`
- 接口：`POST /open-apis/contract/v1/summary/query`
- Content-Type：`application/json`
- 身份：仅 `--as user`
- 操作语义：可能在没有可用摘要时触发生成，按写请求处理，不自动重试

请求体必须精确选择一个字段：

| CLI 参数 | JSON 字段 | 说明 |
|---|---|---|
| `--contract-number <number>` | `contractNumber` | 合同编号，精确匹配；接口也接受 `contract_number` |
| `--contract-name <full-name>` | `contractName` | 合同完整名称，精确匹配；接口也接受 `contract_name` |

CLI 同时保留 `--input-file` / `--data` 输入，但会解析并校验 JSON object，再规范为上述 camelCase 请求体。选择参数与 JSON body 不得同时使用。

## 文件摘要生成

- 命令：`contract-cli contract summary generate`
- 接口：`POST /open-apis/contract/v1/summary/generate`
- Content-Type：`multipart/form-data`
- 身份：仅 `--as user`
- 请求体：仅包含一个 `file` 字段
- 文件限制：单个非空 `doc`、`docx` 或 `pdf`，最大 200MB

`--file` 是本地二进制文件路径；`--file-name` 只覆盖 multipart 文件名。该命令不接受 JSON body，也不先调用其他上传接口。

## 统一响应

服务端 envelope 为 `code`、`msg`、`data`、`success`。必须先判断业务结果，再读取 `data`。HTTP 2xx 不代表业务成功。

`data` 可能包含：`contractId`、`contractNumber`、`contractName`、`summaryId`、`summaryVersionId`、`status`、`errorCode`、`errorMsg`、`overview`、`summary`、`structuredSummary`、`traceId`、`fileType`、`cacheData`、`generated`。

查询返回 `status=1` 时，CLI 输出提示“摘要正在生成中，请稍后再试”，保留 `data.status`；即使服务端同时返回旧的摘要字段，也不会输出。`--raw` 同样遵循此规则。

文件直出响应中的合同 ID、编号、名称和摘要快照 ID 可以为空；不能据此回填或猜测合同关系。
