# Contract Commands Reference

## 查询与创建

```bash
contract-cli contract get 7023646046559404327 --profile contract --as user
contract-cli contract get 7023646046559404327 --profile contract --as app
contract-cli contract get 7023646046559404327 --profile contract --as app --user-id ou_xxx --user-id-type employee_id
contract-cli contract search --profile contract --as user --input-file contract-search.json
contract-cli contract search --profile contract --as user --data '{"contract_number":"CN-001"}'
contract-cli contract search --profile contract --as app --input-file contract-search.json
contract-cli contract search --profile contract --as app --input-file contract-search.json --user-id ou_xxx --user-id-type employee_id
contract-cli contract search-v2 --profile contract --as app --input-file search-v2.json
contract-cli contract summary query --profile contract --as user --contract-number CN-001
contract-cli contract summary generate --profile contract --as user --file ./SMOKE-001.docx
contract-cli contract create --profile contract --input-file contract-create.json
contract-cli contract create --profile contract --data '{"title":"示例合同"}'
contract-cli contract create --profile contract --as app --data '{"contract_name":"示例合同","create_user_id":"ou_xxx"}'
contract-cli contract field update --profile contract --as app --input-file field-update.json
contract-cli contract sign switch-to-paper --profile contract --as app --business-id 7023646046559404327 --business-type-code 0
contract-cli contract sign-url get 7023646046559404327 --profile contract --as app
contract-cli contract form attribute list --profile contract --as app --category-id cat_123 --business-type-code 0
contract-cli contract authorization grant --profile contract --as app --input-file authorization.json
contract-cli contract esign personal-auth-url --profile contract --as app --input-file psn-auth-url.json
contract-cli contract esign org-auth-url --profile contract --as app --input-file org-auth-url.json
contract-cli contract upload-file --profile contract --as user --file ./合同正文.docx --file-type text
contract-cli contract upload-file --profile contract --as app --file ./附件.pdf --file-type attachment --file-name 附件.pdf
contract-cli contract submit 7023646046559404327 --profile contract --as app
contract-cli contract resubmit 7023646046559404327 --profile contract --as app
contract-cli contract patch 7023646046559404327 --profile contract --as app --input-file contract-patch.json
contract-cli contract download-file file_123 --profile contract --as app --output-file ./contract.pdf
contract-cli contract download-file <file-id> --contract <contract-id> --profile contract --as user --output-file ./contract.pdf
contract-cli contract delete 7023646046559404327 --profile contract --as app
contract-cli contract print-file --profile contract --as app --input-file print-file.json
contract-cli contract share get 7023646046559404327 --profile contract --as app
contract-cli contract share batch-create --profile contract --as app --input-file batch-share.json
contract-cli contract cooperation link get 7023646046559404327 --profile contract --as app
contract-cli contract cooperation record get 7023646046559404327 --profile contract --as app
contract-cli contract cooperation search --profile contract --as app --input-file cooperation-search.json
contract-cli contract cooperation file get <contract-id> --profile contract --as app
contract-cli contract cooperation file download <file-id> --profile contract --as app --output-file ./cooperation.docx
contract-cli contract approval start process_123 --profile contract --as app --input-file approval.json
contract-cli contract approval get process_123 --profile contract --as app
contract-cli contract approval get <process-instance-id> --profile contract --as user
contract-cli contract approval comment list <process-instance-id> --profile contract --as user
contract-cli contract approval comment create <process-instance-id> --profile contract --as user --data '{"content":"请确认"}'
contract-cli contract approval task list --profile contract --as user --task-type todo --page-size 20
contract-cli contract approval task approve <task-instance-id> --profile contract --as user --comment "同意"
contract-cli contract approval task reject <task-instance-id> --profile contract --as user --comment "条款风险未解决"
contract-cli contract category list --profile contract --as app --lang zh-CN
contract-cli contract template list --profile contract --as app --category-number CAT-1 --page-size 20 --user-id ou_xxx --user-id-type employee_id
contract-cli contract template get tpl_123 --profile contract --as app --user-id ou_xxx --user-id-type employee_id
contract-cli contract template instantiate --profile contract --as app --data '{"template_number":"TMP001","create_user_id":"ou_xxx"}' --user-id-type employee_id
contract-cli contract sync-user-groups --profile contract --as user
contract-cli contract sync-user-groups --profile contract --as app
contract-cli contract sync-user-groups --profile contract --as app --user-id ou_xxx
contract-cli contract text 7023646046559404327 --profile contract --as user --full-text
contract-cli contract text 7023646046559404327 --profile contract --as app --full-text
contract-cli contract text 7023646046559404327 --profile contract --as app --full-text --user-id-type employee_id
```

## 分类、模板、枚举

```bash
contract-cli contract category list --profile contract --lang zh-CN
contract-cli contract template list --profile contract --category-number CAT-1 --page-size 20
contract-cli contract template get tpl_123 --profile contract
contract-cli contract template instantiate --profile contract --input-file template-instance.json
contract-cli contract enum list --profile contract --type contract_status
```

## 已知限制

- `contract upload-file` 支持 user/app；user 上传 `reviewAttachment` / `approveAttachment` 自动完成 MCP prepare → content → commit，返回最终 `file_id`；app 和其他 user 类型继续走 `/open-apis/contract/v1/files/upload`。新附件须同一 user 在 commit 后 30 分钟内用于评论/审批，详见 [附件规则](approval-mcp-fields.md#新附件上传与复用)
- `contract search-v2`、`contract field update`、`contract sign switch-to-paper`、`contract sign-url get`、`contract form attribute list`、`contract authorization grant`、`contract esign *` 当前仅支持 app 身份
- `contract summary query/generate` 当前仅支持 user 身份；完整工作流见 [智书合同摘要 Skill](../../zhishu-contract-summary/SKILL.md)
- `contract submit`、`contract resubmit`、`contract patch`、`contract delete`、`contract print-file`、`contract approval start` 当前仅支持 app 身份
- `contract download-file`、`contract approval get` 支持 user/app；`contract approval comment list/create`、`contract approval task list/approve/reject` 仅支持 user
- `contract share get`、`contract share batch-create`、`contract cooperation link get`、`contract cooperation record get`、`contract cooperation search`、`contract cooperation file get/download` 当前仅支持 app 身份
- `contract template fields` 尚未实现
- `contract create` 不自动帮你补模板信息；当前就是透传请求体
- `contract create --as app` 时，`create_user_id` 需要你自己写进 JSON body
- `contract template list --as app` 按生产文档通常需要 `category_number`、`user_id`、`user_id_type`，但 CLI 目前只负责透传，不做本地必填校验
- `contract template get --as app` 按生产文档通常需要 `user_id`、`user_id_type`，但 CLI 目前只负责透传，不做本地必填校验
- `contract template instantiate --as app` 按生产文档会用到 query `user_id_type` 和 body `create_user_id`，但 CLI 目前只负责透传，不做本地必填校验

## 字段参考入口

- 搜索合同请求体与文本搜索：看 [合同搜索 Skill](../../contract-cli-contract-search/SKILL.md)
- 合同详情和搜索响应：看 [contract-response-fields.md](contract-response-fields.md)
- 创建合同请求体：先看 [category-fields.md](category-fields.md) 获取 `contract_category_abbreviation`，再看 [create-contract-fields.md](create-contract-fields.md)、[create-contract-field-tree.md](create-contract-field-tree.md)、[create-contract-enums.md](create-contract-enums.md)
- 更新合同文件/归档字段：看 [patch-contract-fields.md](patch-contract-fields.md)
- 模板列表和模板详情：看 [template-fields.md](template-fields.md)
- 创建模板实例：看 [template-instance-fields.md](template-instance-fields.md)
- 生成打印文件：看 [print-file-fields.md](print-file-fields.md)
- 合同分类树：看 [category-fields.md](category-fields.md)
- 分享和协商响应：看 [share-cooperation-fields.md](share-cooperation-fields.md)
- 上传、下载、提交、重提、删除：看 [contract-actions-fields.md](contract-actions-fields.md)
- 审批发起请求体：看 [approval-fields.md](approval-fields.md)
- 新增开放平台补齐接口命令：看 [openapi-gap-commands.md](openapi-gap-commands.md)

## 新增开放平台补齐命令

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
```

接口路径：

- `search-v2`：`POST /open-apis/contract/v1/contracts/searchV2`
- `summary query`：`POST /open-apis/contract/v1/summary/query`
- `summary generate`：`POST /open-apis/contract/v1/summary/generate`
- `field update`：`PUT /open-apis/contract/v1/attribute_definition`
- `sign switch-to-paper`：`POST /open-apis/contract/v1/contracts/signType/switchToPaper`
- `sign-url get`：`GET /open-apis/contract/v1/contracts/{contract_id}/sign_url`
- `form attribute list`：`GET /open-apis/contract/v1/form_definition/attribute`
- `authorization grant`：`POST /open-apis/contract/v1/authorizations`
- `esign personal-auth-url`：`POST /open-apis/esign/auth/psnAuthUrl`
- `esign org-auth-url`：`POST /open-apis/esign/auth/orgAuthUrl`

## app-only 合同操作

```bash
# 提交/重新提交：请求体可选
contract-cli contract submit 7023646046559404327 --profile contract --as app
contract-cli contract resubmit 7023646046559404327 --profile contract --as app --data '{"comment":"修正后重新提交"}'

# 更新合同文件/归档字段：请求体必填
contract-cli contract patch 7023646046559404327 --profile contract --as app --input-file contract-patch.json

# 删除草稿合同：直接删除，不额外要求 --yes
contract-cli contract delete 7023646046559404327 --profile contract --as app
```

接口路径：

- `submit`：`POST /open-apis/contract/v1/contracts/{contract_id}/submit`
- `resubmit`：`POST /open-apis/contract/v1/contracts/{contract_id}/resubmit`
- `patch`：`PATCH /open-apis/contract/v1/contracts/{contract_id}`
- `delete`：`DELETE /open-apis/contract/v1/contracts/{contract_id}`

`contract-patch.json` 示例：

```json
{
  "scan_file_id": "file_scan_xxx"
}
```

不要把 `contract patch` 当成任意基础字段更新；官方文档当前只确认了 `ocr_file_id`、`scan_file_id`、`archive_attachment_map`、`archive_attachment_file_ids` 等文件/归档字段。

## 文件命令

```bash
# 默认拉起保存弹窗；Agent/CI/远程环境建议显式传 --output-file
contract-cli contract download-file file_123 --profile contract --as app --output-file ./contract.pdf

# 管道场景使用 --raw
contract-cli contract download-file file_123 --profile contract --as app --raw > contract.pdf

# user 身份必须同时提供合同 ID；CLI 会立即消费 300 秒TOS 临时预签名 URL
contract-cli contract download-file <file-id> --contract <contract-id> --profile contract --as user --output-file ./contract.pdf

# 生成合同打印文件，请求体必填
contract-cli contract print-file --profile contract --as app --input-file print-file.json
```

接口路径：

- `download-file`：`GET /open-apis/contract/v1/files/{file_id}`
- `download-file --as user`：`GET /open-apis/contract/v1/mcp/contracts/{contract_id}/files/{file_id}/download`
- `print-file`：`POST /open-apis/contract/v1/files`

注意：

- 正式命令是 `download-file`，不支持 `dowload-file` 拼写。
- `download-file --output-file` 遇到已存在文件会失败；需要覆盖时加 `--force`。
- user 身份的临时下载地址不会输出或写入日志；保存文件先写临时文件，完整成功后再替换目标。

## app-only 分享与协商查询

```bash
contract-cli contract share get 7023646046559404327 --profile contract --as app
contract-cli contract share batch-create --profile contract --as app --input-file batch-share.json
contract-cli contract cooperation link get 7023646046559404327 --profile contract --as app
contract-cli contract cooperation record get 7023646046559404327 --profile contract --as app
contract-cli contract cooperation search --profile contract --as app --input-file cooperation-search.json
contract-cli contract cooperation file get <contract-id> --profile contract --as app
contract-cli contract cooperation file download <file-id> --profile contract --as app --output-file ./cooperation.docx
contract-cli contract approval start process_123 --profile contract --as app --data '{"task_instance_id":"task-1","command_type":"general"}'
contract-cli contract approval get process_123 --profile contract --as app --notice-filter notice_filter --task-instance-filter task_instance_filter
contract-cli contract approval get <process-instance-id> --profile contract --as user
```

接口路径：

- `share get`：`GET /open-apis/contract/v1/contracts/{contract_id}/share_records`
- `share batch-create`：`POST /open-apis/contract/v1/contracts/contract/batch_share`
- `cooperation link get`：`GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_link`
- `cooperation record get`：`GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_record_info`
- `cooperation search`：`POST /open-apis/contract/v1/cooperation/search`
- `cooperation file get`：`GET /open-apis/contract/v1/contracts/{contract_id}/cooperation/file_info`
- `cooperation file download`：`GET /open-apis/contract/v1/contracts/cooperation/{file_id}/download_file`
- `approval start`：`POST /open-apis/contract/v1/process_instances/{process_instance_id}/task_approval`
- `approval get`：`GET /open-apis/contract/v1/process_instances/{process_instance_id}`
- `approval get --as user`：`GET /open-apis/contract/v1/mcp/process_instances/{process_instance_id}`

审批发起请求体字段：看 [approval-fields.md](approval-fields.md)

## user MCP 审批评论与个人任务

```bash
contract-cli contract approval comment list <process-instance-id> --profile contract --as user
contract-cli contract approval comment create <process-instance-id> --profile contract --as user --data '{"content":"请确认"}'
contract-cli contract approval task list --profile contract --as user --task-type todo --page-size 20
contract-cli contract approval task approve <task-instance-id> --profile contract --as user --comment "同意" --file-id <file-id>
contract-cli contract approval task reject <task-instance-id> --profile contract --as user --comment "条款风险未解决"
```

完整字段、重试和附件规则：看 [approval-mcp-fields.md](approval-mcp-fields.md)。
