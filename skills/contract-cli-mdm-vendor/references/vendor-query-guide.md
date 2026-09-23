# Vendor Query Guide

这份文档是 `contract-cli mdm vendor ...` 的主入口，优先解决两个问题：

- 我现在是要找候选交易方，还是已经拿到了交易方 id
- 这个命令面到底会把哪些 CLI 参数映射到哪个开放平台查询参数

推荐阅读顺序：

1. 先看本文件，选查询场景
2. 再看 [vendor-query-parameters.md](vendor-query-parameters.md) 查精确参数映射
3. 最后看 [commands.md](commands.md) 直接抄命令示例

## 1. 命令面与硬约束

当前结构化命令分为查询和 app-only 写入：

```bash
contract-cli mdm vendor list --profile contract --name "供应商A"
contract-cli mdm vendor get 1063197165850985296 --profile contract
contract-cli mdm vendor create --profile contract --as app --user-id <operator-user-id> --input-file vendor-create.json
contract-cli mdm vendor update 7003410079584092448 --profile contract --as app --user-id <operator-user-id> --input-file vendor-update.json
contract-cli mdm vendor list-all --profile contract --as app --page-size 20
contract-cli mdm vendor query-by-cert --profile contract --as app --certification-id 91110105 --ad-country CN
```

硬约束：

- 不暴露 `--operator`
- 默认输出就是开放平台原始 envelope；如果要脚本消费，建议加 `--output json`
- 如果要排障或确认原始响应，建议加 `--raw`
- `mdm vendor list` 同时支持 `user` 和 `app`
- `mdm vendor get` 也同时支持 `user` 和 `app`
- `mdm vendor list --as user` 只支持交易方名称模糊查询；`--as app` 按交易方编码查询
- `mdm vendor create/update/list-all/query-by-cert` 当前仅支持 `app`
- `mdm vendor create/update` 必须传 `--user-id`，用于提供当前操作人上下文
- `create` 请求体不要包含后端生成的 `vendor` 编码；`update` 请求体必须包含后端返回的 `id` 和 `vendor` 编码
- 创建/更新请求体除上述规则外仍直接透传，其他字段是否必填以 `mdm fields list --biz-line vendor` 和后端配置为准

## 2. 场景配方

### 2.1 按身份查找候选交易方

适用场景：

- 创建合同前只知道供应商名称
- 需要拿候选列表，再从结果里挑 id

最小命令：

```bash
contract-cli mdm vendor list --profile contract --name "供应商A"
```

常见追加参数：

- `--page-size 20`
- `--page-token <next-token>`
- `--as app --user-id-type employee_id`

补充说明：

- user 路由走 `/open-apis/contract/v1/mcp/vendors`
- app 路由走 `/open-apis/mdm/v1/vendors`
- user 身份只支持交易方名称模糊查询，不支持交易方编码；只拿到编码时应请用户补充名称或内部交易方 ID
- app 身份按交易方编码查询，CLI 为兼容现有命令仍使用 `--name` 参数名并透传到 query `vendor`
- 不得自动切换身份，也不得把个人按编码查询得到的空结果解释为交易方不存在或据此重复创建

### 2.2 分页扫交易方列表

适用场景：

- 不按名字过滤，直接分页遍历
- 或者已经有上一页返回的 `page_token`

最小命令：

```bash
contract-cli mdm vendor list --profile contract --page-size 20
```

翻页示例：

```bash
contract-cli mdm vendor list --profile contract --page-size 20 --page-token next
```

app 示例：

```bash
contract-cli mdm vendor list --profile contract --as app --name "V00000001" --page-size 20 --user-id-type employee_id
```

### 2.3 已知 id 直接查详情

适用场景：

- 已经从搜索结果、外部系统或历史合同里拿到了交易方 id

最小命令：

```bash
contract-cli mdm vendor get 1063197165850985296 --profile contract
```

app 示例：

```bash
contract-cli mdm vendor get 7003410079584092448 --profile contract --as app --user-id-type employee_id
```

补充说明：

- user 路由走 `/open-apis/contract/v1/mcp/vendors/{vendor_id}`
- app 路由走 `/open-apis/mdm/v1/vendors/{vendor_id}`
- 生产文档里 app 详情接口只显式列出了 `user_id_type` 查询参数，没看到 `user_id`
- CLI 仍按共享约定统一透传 `--user-id-type` / `--user-id`，不做本地校验

### 2.4 创建或更新交易方

适用场景：

- 需要把外部供应商同步到合同主数据
- 已知交易方 id，需要修改交易方字段

最小命令：

```bash
contract-cli mdm vendor create --profile contract --as app --user-id <operator-user-id> --input-file vendor-create.json
contract-cli mdm vendor update 7003410079584092448 --profile contract --as app --user-id <operator-user-id> --input-file vendor-update.json
```

`vendor-create.json` 示例不要带后端生成的 `vendor` 编码：

```json
{
  "vendor_text": "供应商A",
  "certification_type": "统一社会信用代码",
  "status": 1,
  "ad_country": "CN",
  "address": "北京市朝阳区"
}
```

`vendor-update.json` 示例必须带后端返回的 `id` 和 `vendor` 编码：

```json
{
  "id": "7003410079584092448",
  "vendor": "V00000001",
  "vendor_text": "供应商A",
  "certification_type": "统一社会信用代码",
  "status": 1
}
```

补充说明：

- `create` 走 `POST /open-apis/mdm/v1/vendors`
- `update` 走 `PUT /open-apis/mdm/v1/vendors/{vendor_id}`
- 写接口必须传 `--user-id`
- 字段配置是动态的，不要只凭样例判断必填；先查 `mdm fields list --biz-line vendor`

### 2.5 全量分页或按证件查询

```bash
contract-cli mdm vendor list-all --profile contract --as app --page-size 20 --page-token next
contract-cli mdm vendor query-by-cert --profile contract --as app --certification-id 91110105 --ad-country CN
```

补充说明：

- `list-all` 走 `GET /open-apis/mdm/v1/vendors/list_all`
- `query-by-cert` 走 `GET /open-apis/mdm/v1/vendors/query_vendors`
- `query-by-cert` 必须传 `--certification-id` 和 `--ad-country`

## 3. 什么时候不要走这里

- 想先确认交易方字段定义：改看 [../../contract-cli-mdm-fields/SKILL.md](../../contract-cli-mdm-fields/SKILL.md)
- 想查合同主体选择逻辑：回到 [../../contract-cli-contract/SKILL.md](../../contract-cli-contract/SKILL.md)
