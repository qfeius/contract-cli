---
name: contract-cli-contract
description: "contract-cli 合同命令技能：支持 user/app 双身份下的合同详情、创建、文本、分类、模板、上传、下载和审批详情；支持 user 身份查询或生成合同摘要、查询/创建审批评论、查询个人任务、通过/拒绝任务；并支持 app 身份下的合同与审批管理。当用户要读取已知合同、查询摘要、创建、更新、下载或办理合同时触发；搜索和筛选合同使用 contract-cli-contract-search，智能审查及审查配置请求分派至独立审查 Skill。"
---

# contract-cli Contract

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

## 适用命令

- `contract-cli contract get <contract-id>`
- `contract-cli contract create`
- `contract-cli contract sync-user-groups`
- `contract-cli contract text <contract-id>`
- `contract-cli contract summary query`
- `contract-cli contract summary generate`
- `contract-cli contract field update`
- `contract-cli contract sign switch-to-paper`
- `contract-cli contract sign-url get <contract-id>`
- `contract-cli contract form attribute list`
- `contract-cli contract authorization grant`
- `contract-cli contract esign personal-auth-url`
- `contract-cli contract esign org-auth-url`
- `contract-cli contract category list`
- `contract-cli contract template list`
- `contract-cli contract template get <template-id>`
- `contract-cli contract template instantiate` 
- `contract-cli contract upload-file`
- `contract-cli contract submit <contract-id>`
- `contract-cli contract resubmit <contract-id>`
- `contract-cli contract patch <contract-id>`
- `contract-cli contract download-file <file-id>`
- `contract-cli contract delete <contract-id>`
- `contract-cli contract print-file`
- `contract-cli contract share get <contract-id>`
- `contract-cli contract share batch-create`
- `contract-cli contract cooperation link get <contract-id>`
- `contract-cli contract cooperation record get <contract-id>`
- `contract-cli contract cooperation search`
- `contract-cli contract cooperation file get <contract-id>`
- `contract-cli contract cooperation file download <file-id>`
- `contract-cli contract approval start <process-instance-id>`
- `contract-cli contract approval get <process-instance-id>`
- `contract-cli contract approval comment list <process-instance-id>`
- `contract-cli contract approval comment create <process-instance-id>`
- `contract-cli contract approval task list`
- `contract-cli contract approval task approve <task-instance-id>`
- `contract-cli contract approval task reject <task-instance-id>`
- `contract-cli contract enum list --type <enum_type>`

## 快速决策

- 想审查外部文件、系统内已有合同或继续 AI 审查任务：读 [../contract-cli-review/SKILL.md](../contract-cli-review/SKILL.md)。
- 想查询/维护审查清单、规则或规则分组：读 [../contract-cli-review-rule/SKILL.md](../contract-cli-review-rule/SKILL.md)；仅选择已有清单仍属于审查流程。
- 想直接拿合同详情：用 `contract get`
- 想搜索或筛选合同（包括合同文本关键词）：读 [合同搜索 Skill](../contract-cli-contract-search/SKILL.md)，由它选择 user、app V1 或 app V2。
- 想按合同编号/完整名称查询摘要，或为本地文件生成摘要：读 [智书合同摘要 Skill](../zhishu-contract-summary/SKILL.md)
- 想更新字段选项、拿签署链接、查询流程字段、合同授权、电子签链接或协商列表：读 [references/openapi-gap-commands.md](references/openapi-gap-commands.md)
- 想直接透传创建合同请求体：用 `contract create`
- 想拿正文文本：用 `contract text`
- 想看分类树：用 `contract category list`
- 想看模板或创建模板实例：用 `contract template ...`
- 想上传合同正文或附件文件：用 `contract upload-file --as user|app`
- 想提交、重新提交、更新或删除草稿合同：用 `contract submit|resubmit|patch|delete --as app`
- 想下载合同相关文件：用 `contract download-file --as user|app`；user 必须加 `--contract`
- 想生成合同打印文件：用 `contract print-file --as app`
- 想查分享、批量分享、协商链接/记录：用 `contract share ...`、`contract share batch-create`、`contract cooperation link get` 或 `contract cooperation record get --as app`
- 想查协商列表或协商文件：用 `contract cooperation search`、`contract cooperation file get` 或 `contract cooperation file download --as app`
- 想授予合同权限：用 `contract authorization grant --as app`
- 想发起旧版流程审批：用 `contract approval start --as app`
- 想查询审批实例：用 `contract approval get --as user|app`
- 想查/写审批评论或查/办个人任务：先读 [references/approval-mcp-fields.md](references/approval-mcp-fields.md)，再用 `contract approval comment|task --as user`
- 想查创建合同相关枚举：用 `contract enum list`
- 若需求是付款：读 [../contract-cli-payment/SKILL.md](../contract-cli-payment/SKILL.md)

## 字段文档导航

### 智能审查入口边界

审查使用 `contract-cli review`，清单/规则/分组分别使用 `review checklist`、`review rule`、`review rule group`；历史 `rule table` 审批矩阵不变。审查仅复用个人 user 上下文，以下历史合同命令的身份矩阵及引用保持不变；不以 app-only 协商能力补个人审查缺口。

外部独立文件为预审，无须先创建合同或协商，结果由 Agent 展示，不返回详情 URL。系统内合同可先复用个人搜索、详情及分类查询；必须核实真实当前正文、业务场景、状态、成员权限和文件映射，缺失时停在对应步骤，不擅自上传普通附件代替。

所有按合同 ID 发起的审查（协商及任务中心各阶段）统一在引用上传后执行 `review subject extract`，让用户从同一文件提取的名称/角色候选选择立场；不使用合同登记交易方或我方/对方替代。旧交易方选择的恢复、候选异常和已有任务查询边界按审查 Skill 处理；6+16 状态及例外也见该 Skill。清单默认按真实分类筛选，显式跨类型选择仍需可见权限，不自动改配置。审批实例、审批任务实例与 AI 审查任务不能混用。

系统合同审查成功后，由审查 Skill 从本次合同详情 `multi_url.pc_url` 设置 `targetType=ai-review`，返回“查看审查”链接，保留原业务定位，退出参数按审查 Skill 的协商/任务中心导航规则处理。审查详情的 URL 字段为空或尚未验证网页展示，都不阻断返回链接；合同地址缺失/无效时按审查 Skill 重查或报告缺口，不编造地址。审查请求不授权创建合同、修改正文、办理审批或接受修订。

### 历史合同字段参考

- 合同搜索统一入口：[合同搜索 Skill](../contract-cli-contract-search/SKILL.md)，包含文本搜索配方、user/app 参数和分页规则。
- 合同详情和搜索响应字段：读 [references/contract-response-fields.md](references/contract-response-fields.md)
- 按字段下载（归档文件、归档主文件、归档附件、合同主文件、合同附件、其他附件、自定义字段附件、盖章节点比对文件）或下载合同全部文件到目录：读 [references/download-all-files.md](references/download-all-files.md)，先确定字段范围和合同归属，再收集、去重并逐个下载；归档文件包含当前合同归档主文件和归档附件；页面“合同附件”对应 `contract_causes`，“其他附件”对应 `contract_attachments`；任务附件来源标明节点类型和名称，评论来源统一为“评论附件”；下载结果默认用表格展示文件名链接、来源、大小（B）三列；多个合同按合同分子目录、分表展示，最上方先写“本次共处理 X 个合同，Y 个文件，成功 A 个，失败 B 个，跳过 C 个”，再给各合同统计，表格前一行展示“**实际合同名称**（合同编号：…）”，不展示“合同名称：”标签，名称加粗，编号紧接其后置于括号内，多个合同的标题前按展示顺序添加从 1 开始的连续序号，相邻合同之间用上下留空行的水平分隔线隔开
- 合同创建请求体与分类来源：先读 [references/category-fields.md](references/category-fields.md) 获取 `contract_category_abbreviation`，再读 [references/create-contract-fields.md](references/create-contract-fields.md)、[references/create-contract-field-tree.md](references/create-contract-field-tree.md)、[references/create-contract-enums.md](references/create-contract-enums.md)
- 合同更新文件/归档字段：读 [references/patch-contract-fields.md](references/patch-contract-fields.md)
- 模板列表和模板详情字段：读 [references/template-fields.md](references/template-fields.md)
- 模板实例请求体：读 [references/template-instance-fields.md](references/template-instance-fields.md)
- 生成打印文件：读 [references/print-file-fields.md](references/print-file-fields.md)
- 合同分类树：读 [references/category-fields.md](references/category-fields.md)
- 分享和协商响应：读 [references/share-cooperation-fields.md](references/share-cooperation-fields.md)
- 上传、下载、提交、重提、删除等轻量动作：读 [references/contract-actions-fields.md](references/contract-actions-fields.md)
- 审批发起请求体：读 [references/approval-fields.md](references/approval-fields.md)
- user MCP 审批详情、评论、任务和统一文件下载：读 [references/approval-mcp-fields.md](references/approval-mcp-fields.md)
- 新增开放平台补齐接口命令：读 [references/openapi-gap-commands.md](references/openapi-gap-commands.md)
- `contract sync-user-groups`、`contract text`、`contract enum list` 当前只补命令级约束；本技能未找到可补到 `contract create` 级别的官方字段页

### P3 接口参数参考

- 搜索 V2：[references/search-v2-parameters.md](../contract-cli-contract-search/references/search-v2-parameters.md)
- 字段更新：[references/field-update-parameters.md](references/field-update-parameters.md)
- 电子签转纸质签：[references/sign-switch-to-paper-parameters.md](references/sign-switch-to-paper-parameters.md)
- 签署链接：[references/sign-url-get-parameters.md](references/sign-url-get-parameters.md)
- 流程字段：[references/form-attribute-list-parameters.md](references/form-attribute-list-parameters.md)
- 合同授权：[references/authorization-grant-parameters.md](references/authorization-grant-parameters.md)
- 个人实名认证链接：[references/esign-personal-auth-url-parameters.md](references/esign-personal-auth-url-parameters.md)
- 企业实名认证链接：[references/esign-org-auth-url-parameters.md](references/esign-org-auth-url-parameters.md)
- 批量分享：[references/share-batch-create-parameters.md](references/share-batch-create-parameters.md)
- 协商搜索：[references/cooperation-search-parameters.md](references/cooperation-search-parameters.md)
- 协商文件信息：[references/cooperation-file-get-parameters.md](references/cooperation-file-get-parameters.md)
- 协商文件下载：[references/cooperation-file-download-parameters.md](references/cooperation-file-download-parameters.md)

## 关键规则

- `contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`contract upload-file`、`contract download-file`、`contract approval get` 同时支持 `--as user` 和 `--as app`
- `contract search-v2`、`contract field update`、`contract sign switch-to-paper`、`contract sign-url get`、`contract form attribute list`、`contract authorization grant`、`contract esign *`、`contract submit`、`contract resubmit`、`contract patch`、`contract delete`、`contract print-file`、`contract share get`、`contract share batch-create`、`contract cooperation link get`、`contract cooperation record get`、`contract cooperation search`、`contract cooperation file get/download`、`contract approval start` 当前仅支持 `--as app`
- `contract summary query/generate`、`contract approval comment list/create`、`contract approval task list/approve/reject` 当前仅支持 `--as user`
- `contract create` 当前直接接收原始创建请求体，不额外暴露 `--template`
- `contract create --as app` 走 `POST /open-apis/contract/v1/contracts`
- `contract create --as app` 的请求体必须自己带 `create_user_id`
- `contract create` 的推荐阅读顺序是：
  - 创建合同前，先读 [references/category-fields.md](references/category-fields.md)，用与创建命令相同的 `--profile` 和 `--as` 查询分类树，取当前身份可用末级分类的 `abbreviation` 作为 `contract_category_abbreviation`
  - 再读 [references/create-contract-fields.md](references/create-contract-fields.md) 选场景和最小请求体
  - 然后读 [references/create-contract-field-tree.md](references/create-contract-field-tree.md) 查嵌套对象和 JSON Path
  - 最后读 [references/create-contract-enums.md](references/create-contract-enums.md) 确认 code 取值
- 这三份文档一起构成 `contract create` 的完整参数主档，不需要再回查旧接口清单
- `contract get --as app` 走开放平台标准接口 `/open-apis/contract/v1/contracts/{contract_id}`
- `--user-id-type` / `--user-id` 是通用 query 参数：
  - `--user-id-type` 不传时默认拼接 `user_id_type=user_id`
  - app 标准接口中，显式传 `--user-id-type <type>` 会覆盖默认值
  - user MCP 工具当前将 `user_id_type` 固定为 `user_id`，显式 flag 暂不能覆盖；不要在 user 搜索中声称已支持 `union_id`
  - `--user-id` 传了就原样拼到底层接口，不传就不带
  - 不做命令级校验
- `contract sync-user-groups --as app` 走 `/open-apis/contract/v1/contracts/user-groups/sync`
- `contract text --as app` 走 `GET /open-apis/contract/v1/contracts/{contract_id}/text`
- `contract category list --as app` 走 `/open-apis/contract/v1/contract_categorys`
- `contract template list --as app` 走 `/open-apis/contract/v1/templates`
- 按生产文档，`contract template list --as app` 的 `category_number`、`user_id`、`user_id_type` 都属于 query 参数；CLI 仍只透传，不做本地必填校验
- `contract template get --as app` 走 `/open-apis/contract/v1/templates/{template_id}`
- 按生产文档，`contract template get --as app` 的 `user_id`、`user_id_type` 都属于 query 参数；CLI 仍只透传，不做本地必填校验
- `contract template instantiate --as app` 走 `POST /open-apis/contract/v1/template_instances`
- 按生产文档，`contract template instantiate --as app` 的 query 只有 `user_id_type`，请求体里需要 `create_user_id`；CLI 仍只透传，不做本地必填校验
- `contract upload-file --as user|app` 走 `POST /open-apis/contract/v1/files/upload`
- `contract upload-file` 使用 `multipart/form-data`，字段是 `file_name`、`file_type`、`file`
- `contract upload-file` 的 `--file` 是本地真实文件路径，不是 JSON 请求体文件
- `contract upload-file` 不接受 `--input-file` / `--data`
- `contract upload-file` 本地限制文件大小小于等于 `200MB`
- `contract submit --as app` 走 `POST /open-apis/contract/v1/contracts/{contract_id}/submit`，`--input-file` / `--data` 可选
- `contract summary query --as user` 走 `POST /open-apis/contract/v1/summary/query`，按合同编号或完整名称精确匹配；查询可能触发生成，结果不确定时不自动重试
- `contract summary generate --as user` 直接走 `POST /open-apis/contract/v1/summary/generate`，使用单个非空 doc/docx/pdf `--file <path>` 作为 multipart/form-data 字段 `file`；最大 200MB，不接受 `--input-file` / `--data`
- `contract field update --as app` 走 `PUT /open-apis/contract/v1/attribute_definition`，请求体直接透传
- `contract sign switch-to-paper --as app` 走 `POST /open-apis/contract/v1/contracts/signType/switchToPaper`，用 query `business_id` 和 `business_type_code`
- `contract sign-url get --as app` 走 `GET /open-apis/contract/v1/contracts/{contract_id}/sign_url`
- `contract form attribute list --as app` 走 `GET /open-apis/contract/v1/form_definition/attribute`，用 query `category_id` 和 `business_type_code`
- `contract authorization grant --as app` 走 `POST /open-apis/contract/v1/authorizations`
- `contract esign personal-auth-url/org-auth-url --as app` 分别走 `/open-apis/esign/auth/psnAuthUrl` 和 `/open-apis/esign/auth/orgAuthUrl`
- `contract resubmit --as app` 走 `POST /open-apis/contract/v1/contracts/{contract_id}/resubmit`，`--input-file` / `--data` 可选
- `contract patch --as app` 走 `PATCH /open-apis/contract/v1/contracts/{contract_id}`，`--input-file` / `--data` 必填且互斥
- `contract download-file --as app` 走 `GET /open-apis/contract/v1/files/{file_id}`；默认拉起保存弹窗，Agent/CI/远程环境推荐传 `--output-file`
- `contract download-file --as user` 必须传 `--contract`，CLI 获取 300 秒预签名地址后立即下载二进制；不输出或记录该地址，文件先写入同目录临时文件并校验，完整下载后才替换或创建目标文件
- `contract download-file --raw` 会把二进制内容写到 stdout，不打印额外提示
- `contract delete --as app` 走 `DELETE /open-apis/contract/v1/contracts/{contract_id}`，命令直接删除，不额外要求 `--yes`
- `contract print-file --as app` 走 `POST /open-apis/contract/v1/files`，`--input-file` / `--data` 必填且互斥
- `contract share get --as app` 走 `GET /open-apis/contract/v1/contracts/{contract_id}/share_records`
- `contract share batch-create --as app` 走 `POST /open-apis/contract/v1/contracts/contract/batch_share`
- `contract cooperation link get --as app` 走 `GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_link`
- `contract cooperation record get --as app` 走 `GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_record_info`
- `contract cooperation search --as app` 走 `POST /open-apis/contract/v1/cooperation/search`
- `contract cooperation file get --as app` 走 `GET /open-apis/contract/v1/contracts/{contract_id}/cooperation/file_info`
- `contract cooperation file download --as app` 走 `GET /open-apis/contract/v1/contracts/cooperation/{file_id}/download_file`
- `contract approval start --as app` 走 `POST /open-apis/contract/v1/process_instances/{process_instance_id}/task_approval`，`--input-file` / `--data` 必填且互斥
- `contract approval get --as app` 走 `GET /open-apis/contract/v1/process_instances/{process_instance_id}`，可选 `--notice-filter` / `--task-instance-filter`
- `contract approval get --as user` 走 `GET /open-apis/contract/v1/mcp/process_instances/{process_instance_id}`，不发送调用人 query，也不主动发送 `X-MCP-Response-Profile`
- user 评论/任务命令的调用人由个人 Token 唯一确定；`--mention-id-type` 仅解释评论请求体中的 `user_id` 数组
- `contract approval task list` 虽然使用 POST，但属于读操作；评论创建和任务通过/拒绝是非幂等写操作，不能自动重试
- 不要使用 `dowload-file` 拼写；正式命令是 `download-file`
- 常用 `file_type`：`text` 合同文本、`attachment` 其他附件、`scan` 归档扫描件、`cause` 合同附件、`archiveAttachment` 归档附件、`customPictureAttachment` 图片附件、`customTableAttachment` 表格附件、`customFileAttachment` 文件附件
- `contract text` 支持 `--full-text`、`--offset`、`--limit`
- `contract template instantiate` 只接收请求体，不再接模板 ID 位置参数

## 实现来源

- [internal/cli/contract_command.go](../../internal/cli/contract_command.go)
- [internal/cli/openapi_gap_command.go](../../internal/cli/openapi_gap_command.go)
- [internal/openplatform/contract/service.go](../../internal/openplatform/contract/service.go)
- [references/commands.md](references/commands.md)
- [references/search-contract-fields.md](../contract-cli-contract-search/references/search-contract-fields.md)
- [references/search-user-parameters.md](../contract-cli-contract-search/references/search-user-parameters.md)
- [references/search-app-parameters.md](../contract-cli-contract-search/references/search-app-parameters.md)
- [references/search-v2-parameters.md](../contract-cli-contract-search/references/search-v2-parameters.md)
- [references/contract-response-fields.md](references/contract-response-fields.md)
- [references/create-contract-fields.md](references/create-contract-fields.md)
- [references/create-contract-field-tree.md](references/create-contract-field-tree.md)
- [references/create-contract-enums.md](references/create-contract-enums.md)
- [references/patch-contract-fields.md](references/patch-contract-fields.md)
- [references/template-fields.md](references/template-fields.md)
- [references/template-instance-fields.md](references/template-instance-fields.md)
- [references/print-file-fields.md](references/print-file-fields.md)
- [references/category-fields.md](references/category-fields.md)
- [references/share-cooperation-fields.md](references/share-cooperation-fields.md)
- [references/contract-actions-fields.md](references/contract-actions-fields.md)
- [references/openapi-gap-commands.md](references/openapi-gap-commands.md)
- [references/approval-fields.md](references/approval-fields.md)
- [references/approval-mcp-fields.md](references/approval-mcp-fields.md)
- P3 接口的完整类型、必填项、枚举与约束：按“P3 接口参数参考”选择对应文档

## 操作建议

- 先确认 profile 已完成目标身份的登录：
  - 新任务先执行 `auth status --profile contract --as user`；profile 的 `user.auth_mode=device` 且已有有效授权时直接复用。需要授权时按授权 Skill 执行一次 `auth init`：`status=authorized` 直接继续业务请求，不展示授权链接、不执行 `auth complete`；仅 `status=pending` 时展示授权信息，用户确认完成后再执行一次 `auth complete`。不要启动本机回调登录或持续轮询。
  - 仅保留的 Authorization Code 模式使用 `auth login --as user`。
  - user 身份覆盖详情、搜索、创建、同步用户组、合同文本、分类、模板、文件上传和其他 user-only 命令。
  - app 详情、app 搜索、app 创建、app 同步用户组、app 合同文本、app 分类查询、app 模板列表、app 模板详情、app 模板实例、app 文件上传、app 提交/重提/更新/删除/下载/打印/分享/协商查询/审批管理：`auth login --as app`
- 复杂请求体优先用 `--input-file`
- 需要脚本消费时加 `--output json`
- 需要对照后端原始 envelope 时加 `--raw`
- 创建合同前，先根据是“文件正文模式”“模板实例模式”“合同变更”还是“合同终止”选主文档里的场景配方
- 创建合同前，必须先查询合同分类；`contract_category_abbreviation` 不是固定枚举，不要使用分类 `name`、`number` 或只传 `contract_category_id` 代替
- 复杂对象不要平铺查表，直接去字段树附录按 JSON Path 找
- 遇到 code 型字段，不要凭印象写值，直接看枚举附录
- 交易方/我方主体、金额、期限、合同分类这几个字段最容易缺，优先核对

## 不要这样做

- 写命令返回“执行结果不确定”时，不要立即重试创建、模板实例化、用户组同步、文件上传、评论创建或任务处理；先用对应查询命令确认结果。

- 不要对 `contract enum` 传 `--as app`
- 不要继续写 `--file contract.json`；JSON 请求体用 `--input-file`
- 不要对新增 app-only 命令传 `--as user`
- 不要把付款命令写成 `contract payment ...`；付款申请、付款计划、付款记录走顶层 `payment`
- 不要把 `contract download-file` 的二进制响应交给 JSON 输出；保存文件用默认弹窗或 `--output-file`，管道场景用 `--raw`
- 不要把 `contract template fields` 当成已实现能力
