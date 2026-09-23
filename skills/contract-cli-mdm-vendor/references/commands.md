# Vendor Commands Reference

```bash
contract-cli mdm vendor list --profile contract --as user --name "供应商A" --page-size 20
contract-cli mdm vendor list --profile contract --as app --name "V00000001" --page-size 20 --user-id-type employee_id
contract-cli mdm vendor get 7003410079584092448 --profile contract --as user
contract-cli mdm vendor get 7003410079584092448 --profile contract --as app --user-id-type employee_id

contract-cli mdm vendor create --profile contract --as user --input-file vendor-create.json
contract-cli mdm vendor create --profile contract --as user --department-id-type open_department_id --data '{"vendorText":"交易方A","ownerDepts":["od-xxx"]}'
contract-cli mdm vendor create --profile contract --as app --user-id <operator-user-id> --input-file vendor-create.json

contract-cli mdm vendor patch 7003410079584092448 --profile contract --as user --input-file vendor-patch.json
contract-cli mdm vendor patch 7003410079584092448 --profile contract --as user --department-id-type open_department_id --data '{"ownerDepts":["od-xxx"]}'
contract-cli mdm vendor patch 7003410079584092448 --profile contract --as app --user-id <operator-user-id> --input-file vendor-patch.json

contract-cli mdm vendor enable 7003410079584092448 --profile contract --as user
contract-cli mdm vendor disable 7003410079584092448 --profile contract --as user

contract-cli mdm vendor update 7003410079584092448 --profile contract --as app --user-id <operator-user-id> --input-file vendor-update.json
contract-cli mdm vendor list-all --profile contract --as app --page-size 20
contract-cli mdm vendor query-by-cert --profile contract --as app --certification-id 91110105 --ad-country CN
```

说明：

- `list|get|create|patch` 按 `--as user|app` 路由到对应接口。
- `enable|disable` 仅支持 user；`update|list-all|query-by-cert` 仅支持 app。
- user 操作人来自登录态，不传 `--user-id`；app 写入必须传当前操作人的 `--user-id`。
- create 不传服务端生成的 `vendor`；patch 的 ID 只在 path，body 不传 `id`。
- user 创建/PATCH 传 `ownerDepts` 中的 `od-...` 时必须加 `--department-id-type open_department_id`；内部数字 ID 不需要该参数。
- 写请求返回 `UNKNOWN` 或执行结果不确定时，先执行 `get` 查询，不直接重试。
