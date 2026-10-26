# OpenAPI Gap Commands

这份文档覆盖新增的合同开放平台补齐命令。开始前仍要先读共享 Skill：摘要命令使用 user OAuth，其余本页命令使用 app token。

合同搜索的身份选择、条件与分页统一见 [合同搜索 Skill](../../contract-cli-contract-search/SKILL.md)；本页仅保留 app V2 命令速查。

## 命令速查

```bash
contract-cli contract search-v2 --profile contract --as app --input-file search-v2.json
contract-cli contract summary query --profile contract --as user --contract-number CN-001
contract-cli contract summary generate --profile contract --as user --file ./SMOKE-001.docx
contract-cli contract field update --profile contract --as app --input-file field-update.json
contract-cli contract sign switch-to-paper --profile contract --as app --business-id <contract-id> --business-type-code 0
contract-cli contract sign-url get <contract-id> --profile contract --as app
contract-cli contract form attribute list --profile contract --as app --category-id <category-id> --business-type-code 0
contract-cli contract authorization grant --profile contract --as app --input-file authorization.json
contract-cli contract esign personal-auth-url --profile contract --as app --input-file psn-auth-url.json
contract-cli contract esign org-auth-url --profile contract --as app --input-file org-auth-url.json
contract-cli contract share batch-create --profile contract --as app --input-file batch-share.json
contract-cli contract cooperation search --profile contract --as app --input-file cooperation-search.json
contract-cli contract cooperation file get <contract-id> --profile contract --as app
contract-cli contract cooperation file download <file-id> --profile contract --as app --output-file ./cooperation.docx
```

## 最小 JSON

`search-v2.json`：

```json
{
  "contract_number": "CN-001"
}
```

`summary-query.json`：

```json
{
  "contractNumber": "CN-001"
}
```

`contract summary query` 按合同编号或完整名称精确匹配，JSON 中 `contractNumber` 与 `contractName` 二选一。完整工作流见 [智书合同摘要 Skill](../../zhishu-contract-summary/SKILL.md)。

`contract summary generate` 直接调用生成接口，不经过独立 upload 接口。请求体是 `multipart/form-data`，只发送字段名为 `file` 的非空 doc/docx/pdf 文件；最大 200MB，`--file-name` 可覆盖 form-data 文件名。

`field-update.json`：

```json
{
  "module_name": "签约信息",
  "attribute_name": "城市",
  "value_scopes": [
    {
      "label": "深圳",
      "value": "sz"
    }
  ]
}
```

`authorization.json`：

```json
{
  "business_id": "6965467645105668385",
  "business_type_code": 0,
  "authorized_user_id": "ae721f86",
  "start_time": "1626850544000",
  "end_time": "1626850999000",
  "permanent": true,
  "source_system": "ZhishuOpenPlatform"
}
```

`batch-share.json`：

```json
{
  "contract_id": "6965467645105668385",
  "user_ids": ["kg8689t2"]
}
```

`psn-auth-url.json`：

```json
{
  "psnAuthConfig": {
    "psnAccount": "18500000000"
  }
}
```

`org-auth-url.json`：

```json
{
  "orgAuthConfig": {
    "orgName": "北京优矩"
  }
}
```

`cooperation-search.json`：

```json
{
  "user_id": "user_1",
  "keyword": "合同名称或编号",
  "page_size": 10
}
```

## 路径映射

| 命令 | 方法与路径 | 请求体 |
| --- | --- | --- |
| `contract search-v2` | `POST /open-apis/contract/v1/contracts/searchV2` | 必填 |
| `contract summary query` | `POST /open-apis/contract/v1/summary/query` | 必填 |
| `contract summary generate` | `POST /open-apis/contract/v1/summary/generate` | 必填，multipart/form-data 字段 `file` |
| `contract field update` | `PUT /open-apis/contract/v1/attribute_definition` | 必填 |
| `contract sign switch-to-paper` | `POST /open-apis/contract/v1/contracts/signType/switchToPaper` | 不接受 |
| `contract sign-url get` | `GET /open-apis/contract/v1/contracts/{contract_id}/sign_url` | 不接受 |
| `contract form attribute list` | `GET /open-apis/contract/v1/form_definition/attribute` | 不接受 |
| `contract authorization grant` | `POST /open-apis/contract/v1/authorizations` | 必填 |
| `contract esign personal-auth-url` | `POST /open-apis/esign/auth/psnAuthUrl` | 必填 |
| `contract esign org-auth-url` | `POST /open-apis/esign/auth/orgAuthUrl` | 必填 |
| `contract share batch-create` | `POST /open-apis/contract/v1/contracts/contract/batch_share` | 必填 |
| `contract cooperation search` | `POST /open-apis/contract/v1/cooperation/search` | 必填 |
| `contract cooperation file get` | `GET /open-apis/contract/v1/contracts/{contract_id}/cooperation/file_info` | 不接受 |
| `contract cooperation file download` | `GET /open-apis/contract/v1/contracts/cooperation/{file_id}/download_file` | 不接受 |

## 注意

- `contract summary query/generate` 仅支持 `--as user`；本页其余命令是 app-only。
- `contract summary query` 的 `--contract-number` / `--contract-name` 二选一；也可传 JSON body，CLI 会校验并规范字段。查询可能触发生成，结果不确定时不自动重试。
- `contract summary generate` 只发送一次生成请求；传本地 `--file`，CLI 会作为 multipart/form-data 的 `file` 字段直传生成接口，结果不确定时不自动重试。
- `contract sign switch-to-paper` 的 `business_type_code` 常用值：`0` 合同申请、`2` 合同变更、`3` 合同终止。
- `contract form attribute list` 的 `business_type_code` 常用值：`0` 申请、`1` 变更、`2` 终止、`3` 合同组申请。
- 下载类命令在 Agent/CI/远程环境优先传 `--output-file`；目标文件已存在时显式传 `--force`，管道场景用 `--raw`。
