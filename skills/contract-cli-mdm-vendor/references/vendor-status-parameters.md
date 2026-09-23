# mdm vendor enable / disable Parameters

本页专用于 `contract-cli mdm vendor enable` 和 `contract-cli mdm vendor disable`。

- 接口：`PUT /open-apis/contract/v1/mcp/vendors/{vendor_id}/status`
- 身份：仅 `user`
- 请求体：CLI 不接收请求体；根据命令生成 `target_status`
- 官方 OpenAPI：本次新增个人交易方启停接口

## CLI 参数映射

| 参数 | 请求位置 | 类型 | 必填性 | 说明 |
| --- | --- | --- | --- | --- |
| vendor-id | path.vendor_id | string | 必填 | 正十进制 64 位 ID。 |
| --as | 本地上下文 | enum | 可选 | 仅支持 `user`。 |
| --profile | 本地上下文 | string | 可选 | 不传时使用当前 profile。 |
| --user-id | query.user_id | string | 禁止 | 操作人来自个人认证。 |
| --output | CLI 输出 | enum | 可选 | `json`、`yaml` 或 `table`。 |
| --raw | CLI 输出 | boolean | 可选 | 原样输出响应。 |

## 枚举与约束

- `enable` 固定生成 `target_status=1`；`disable` 固定生成 `target_status=0`。
- 仅执行正常启用或停用，并联动联系人、账户、地址和公司视图状态。
- 启停不发起审批；与既有在途审批冲突时由服务端拒绝。
- 已是目标状态返回 `NO_CHANGE`，不会重复写入；实际改变返回 `APPLIED`。
- 返回中的 `previous_status` 是操作前状态，`status` 是操作后状态；`NO_CHANGE` 时二者相同。
- 返回 `UNKNOWN` 或执行结果不确定时，先用 `mdm vendor get` 查询，不直接重试。

## 示例

```bash
contract-cli mdm vendor enable 7003410079584092448 --profile contract --as user
contract-cli mdm vendor disable 7003410079584092448 --profile contract --as user
```
