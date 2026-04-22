# contract-cli 命令文档

本文档汇总当前代码里已经实际支持的 `contract-cli` 命令，作为后续继续扩展 bot 接口和新业务命令的基线。

## 当前状态

- 当前只预置 `dev` 环境：`contract-cli config add --env dev`
- `contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get`、`mdm fields list` 是当前仅有的十四个同时支持 `user` 与 `bot` 的结构化业务命令
- `contract upload-file` 当前仅支持 `--as bot`
- 除上述 bot 能力外，当前其他结构化业务命令仍只支持 `--as user`
- `bot` 目前已经支持登录、状态查看、登出、默认身份切换
- 推荐使用 `npx skills add qfeius/contract-cli -y -g` 安装跨 Agent 平台 skills；`contract-cli skills install` 保留为 CLI 内置兜底
- `update check` 支持手动检查 npm 远端版本；CLI 在交互终端下会每 30 分钟最多自动检查一次并提示升级
- 当前全部已支持命令都可以通过 `--help` 查看本地帮助，例如 `contract-cli --help`、`contract-cli contract search --help`、`contract-cli help contract upload-file`
- `bot` 业务接口后续继续新增时，优先在本文件补充命令矩阵

## 通用约定

### 通用帮助入口

CLI 内置帮助只做本地渲染，不读取 profile、不发 HTTP、不触发自动版本检查。

常用入口：

```bash
contract-cli --help
contract-cli -h
contract-cli help
contract-cli help contract upload-file
contract-cli contract search --help
contract-cli contract get <contract-id> --help
```

帮助内容按命令层级展示：

- 命令组展示 `Commands`
- 叶子命令展示 `Flags`、`Examples`、`Notes`
- `Notes` 只放身份限制、user/bot 路由差异、请求体或文件上传关键约束
- 不兼容旧顶层别名，例如 `contract-cli help vendor` 会返回未知 help topic

### 通用身份规则

- `config` 和 `version` 不需要登录态
- `skills list/install` 不需要登录态；通用 `npx skills add qfeius/contract-cli -y -g` 也不依赖 contract-cli 登录态
- `update check` 不需要登录态
- `auth login --as user` 走 OAuth 用户授权
- `auth login --as bot` 走 `appId + appSecret -> tenant_access_token/internal`
- `contract ...`、`mdm ...` 结构化命令大多默认只支持 `--as user`
- `/open-apis/contract/v1/mcp/...` 路径大多仍只支持 `--as user`
- `contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get`、`mdm fields list` 是例外：
  - `contract get --as user` 走 MCP 路径 `/open-apis/contract/v1/mcp/contracts/{contract_id}`
  - `contract get --as bot` 走开放平台路径 `/open-apis/contract/v1/contracts/{contract_id}`
  - `--as user` 走 MCP 路径 `/open-apis/contract/v1/mcp/contracts/search`
  - `--as bot` 走开放平台路径 `/open-apis/contract/v1/contracts/search`
  - `contract create --as user` 走 MCP 路径 `/open-apis/contract/v1/mcp/contracts`
  - `contract create --as bot` 走开放平台路径 `POST /open-apis/contract/v1/contracts`
  - `contract sync-user-groups --as user` 走 `/open-apis/contract/v1/mcp/contracts/user-groups/sync?user_id_type=user_id`
  - `contract sync-user-groups --as bot` 走 `/open-apis/contract/v1/contracts/user-groups/sync`
  - `contract text --as user` 走 `/open-apis/contract/v1/mcp/contracts/{contract_id}/text?user_id_type=user_id&...`
  - `contract text --as bot` 走 `POST /open-apis/contract/v1/contracts/{contract_id}/text?...`
  - `contract category list --as user` 走 `/open-apis/contract/v1/mcp/contract_categorys`
  - `contract category list --as bot` 走 `/open-apis/contract/v1/contract_categorys`
  - `contract template list --as user` 走 `/open-apis/contract/v1/mcp/templates`
  - `contract template list --as bot` 走 `/open-apis/contract/v1/templates`
  - `contract template get --as user` 走 `/open-apis/contract/v1/mcp/templates/{template_id}`
  - `contract template get --as bot` 走 `/open-apis/contract/v1/templates/{template_id}`
  - `contract template instantiate --as user` 走 `/open-apis/contract/v1/mcp/template_instances`
  - `contract template instantiate --as bot` 走 `POST /open-apis/contract/v1/template_instances`
  - `mdm vendor list --as user` 走 `/open-apis/contract/v1/mcp/vendors`
  - `mdm vendor list --as bot` 走 `/open-apis/mdm/v1/vendors`
  - `mdm vendor get --as user` 走 `/open-apis/contract/v1/mcp/vendors/{vendor_id}`
  - `mdm vendor get --as bot` 走 `/open-apis/mdm/v1/vendors/{vendor_id}`
  - `mdm legal list --as user` 走 `/open-apis/contract/v1/mcp/legal_entities`
  - `mdm legal list --as bot` 走 `/open-apis/mdm/v1/legal_entities/list_all`
  - `mdm legal get --as user` 走 `/open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}`
  - `mdm legal get --as bot` 走 `/open-apis/mdm/v1/legal_entities/{legal_entity_id}`，并额外透传同名 query `legal_entity_id`
  - `mdm fields list --as user` 走 `/open-apis/contract/v1/mcp/config/config_list`
  - `mdm fields list --as bot` 走 `/open-apis/mdm/v1/config/config_list`
- `api call` 对非 `contract/v1/mcp` 路径可以继续按 profile 默认身份或显式 `--as` 发起请求，后续 bot 接口优先从这里开始验证

### 通用输出

结构化业务命令和 `api call` 共享这些输出参数：

- `--output json|yaml|table`
- `--raw`

默认输出格式是 `json`。

### 通用请求体输入

需要请求体的命令统一使用：

- `--input-file <json-file>`
- `--data '<json-string>'`

约束：

- `--input-file` 与 `--data` 互斥
- `contract create`、`contract template instantiate` 至少需要其一
- `contract search` 可以只传查询 flag，也可以显式传空对象 `{}`，不强制要求 body 输入
- `--file` 只用于真实二进制文件上传，例如 `contract upload-file`
- 不要把 `--file` 当 JSON 请求体输入；JSON 请求体始终用 `--input-file`

### 通用用户标识参数

开放平台命令统一预留了两组通用 query 参数：

- `--user-id-type`
- `--user-id`

当前行为：

- `contract ...`、`mdm ...` 结构化命令会透传到对应底层接口
- `api call` 也支持这两个参数
- `--user-id-type` 不传时默认拼接 `user_id_type=user_id`
- 显式传 `--user-id-type <type>` 时会覆盖默认值
- `--user-id` 传了就拼接到 query string，不传就不带
- 不区分 `user` / `bot`
- 不做命令级校验

## 命令矩阵

### 1. 配置与版本

#### `contract-cli config add`

用途：初始化或更新 profile，并写入 user OAuth 与 bot token 的基础配置。

命令：

```bash
contract-cli config add --env dev --name contract-group
```

支持参数：

- `--env`：当前仅支持 `dev`
- `--name`：profile 名称，默认 `contract-group`
- `--resource-metadata-url`：覆盖 protected resource metadata 地址
- `--redirect-url`：覆盖 OAuth callback 地址
- `--scope`：覆盖默认 scope 列表

执行结果：

- 写入 `open_platform_base_url`
- 写入 user OAuth metadata
- 写入 bot `bot_token_endpoint`
- 将 profile 设为当前 profile

#### `contract-cli version`

用途：查看当前 CLI 版本、commit 和构建时间。

命令：

```bash
contract-cli version
contract-cli --version
```

#### `contract-cli update check`

用途：检查 npm 远端是否存在可升级版本。

命令：

```bash
contract-cli update check
contract-cli update check --channel beta
contract-cli update check --channel latest
```

支持参数：

- `--channel`：npm dist-tag；不传时根据当前版本推断，预发布版本默认检查 `beta`，稳定版本默认检查 `latest`

执行结果：

- 当前版本是 `dev`、`unknown` 或非语义化版本（例如源码 git hash）时跳过远端检查
- 有新版本时输出当前版本、远端版本和 `npm install -g @qfeius/contract-cli@<channel> --registry https://registry.npmjs.org`
- 无新版本时输出当前版本已是最新
- 手动执行 `update check` 会直接访问 npm registry，并把结果写入本机 update cache

自动提示：

- 普通命令在交互终端下会自动检查远端版本
- 自动检查最多每 30 分钟触发一次，缓存文件位于当前配置目录的 `update-check.json`
- 网络失败、registry 失败或当前是 dev 构建时不会阻断原命令；自动检查失败也会按 30 分钟间隔抑制重复探测
- 设置 `CONTRACT_CLI_NO_UPDATE_CHECK=1` 可以关闭自动检查

#### `contract-cli skills list`

用途：列出当前二进制内置的 Codex skills。

命令：

```bash
contract-cli skills list
```

输出内容：

- skill 名称
- skill 版本
- skill 描述

#### `npx skills add qfeius/contract-cli -y -g`

用途：使用通用 `skills` installer 从 GitHub 仓库安装 contract-cli 的 Agent skills。

推荐命令：

```bash
npx skills add qfeius/contract-cli -y -g
```

适用场景：

- 推荐给 Codex、Cursor、Trae、Claude Code 等多类 Agent 环境使用
- 从 GitHub 仓库的 `skills/` 目录安装，适合快速获得最新 skill 文档
- `-g` 表示全局安装，安装位置和平台适配由通用 `skills` installer 决定

注意事项：

- 该命令依赖 npm、npx 和 GitHub 网络访问
- 安装内容来自远程 `qfeius/contract-cli` 仓库，不读取本地未 push 的改动
- 若通用 installer 不可用，使用 `contract-cli skills install` 作为兜底

#### `contract-cli skills install`

用途：将当前二进制内置的 Codex skills 安装到本机 Codex skills 目录，作为通用 `npx skills add ...` 不可用时的兜底方案。

命令：

```bash
contract-cli skills install
contract-cli skills install --target ~/.codex/skills
contract-cli skills install --force
```

支持参数：

- `--target`：安装目标目录；默认优先使用 `$CODEX_HOME/skills`，否则使用 `~/.codex/skills`
- `--force`：覆盖已存在的同名 skill；默认不覆盖，会跳过已有目录

执行结果：

- 复制内置 `auth`、`contract-cli-shared`、`contract-cli-contract`、`contract-cli-mdm-vendor`、`contract-cli-mdm-legal`、`contract-cli-mdm-fields`、`contract-cli-api-call` 等 skill
- 保留 `SKILL.md`、`agents/openai.yaml` 和 `references/*.md`

### 2. 鉴权

#### `contract-cli auth login`

##### `contract-cli auth login --as user`

用途：发起 OAuth 用户授权。

命令：

```bash
contract-cli auth login --profile contract-group --as user
```

支持参数：

- `--profile`
- `--as user`
- `--timeout`
- `--no-open-browser`

##### `contract-cli auth login --as bot`

用途：使用 bot `appId/appSecret` 直接换取 tenant access token。

命令：

```bash
contract-cli auth login --profile contract-group --as bot --app-id <id> --app-secret <secret>
```

支持参数：

- `--profile`
- `--as bot`
- `--app-id`
- `--app-secret`

补充说明：

- bot 凭证优先级：flag > env > 已保存 secrets
- 登录成功后会保存 bot token，并将默认身份切到 `bot`
- `auth logout --as bot` 只清 token，不删除 `appId/appSecret`

#### `contract-cli auth status`

用途：查看某个 profile 的 user 或 bot 身份状态。

命令：

```bash
contract-cli auth status --profile contract-group --as user
contract-cli auth status --profile contract-group --as bot
```

支持参数：

- `--profile`
- `--as user|bot`

当前状态语义：

- user：`authorized` / `unauthorized`
- bot：`authorized` / `expired` / `configured` / `unconfigured`

#### `contract-cli auth logout`

用途：清理指定身份的 token。

命令：

```bash
contract-cli auth logout --profile contract-group --as user
contract-cli auth logout --profile contract-group --as bot
```

支持参数：

- `--profile`
- `--as user|bot`

补充说明：

- user logout：清空 user token
- bot logout：只清空 bot token，保留 app 凭证

#### `contract-cli auth use`

用途：切换 profile 默认业务身份。

命令：

```bash
contract-cli auth use --profile contract-group --as user
contract-cli auth use --profile contract-group --as bot
```

支持参数：

- `--profile`
- `--as user|bot`

### 3. 原始开放平台调用

#### `contract-cli api call`

用途：对开放平台任意相对路径发起原始调用，是后续验证 bot 业务接口的首选入口。

命令：

```bash
contract-cli api call GET /open-apis/contract/v1/mcp/config/config_list --profile contract-group --as user
contract-cli api call POST /open-apis/xxx --profile contract-group --as bot --data '{"foo":"bar"}'
```

支持参数：

- `--profile`
- `--as`
- `--output`
- `--raw`
- `--input-file`
- `--data`
- `--header Key: Value`，可重复传入

约束：

- 路径必须是相对路径，且以 `/open-apis/` 开头
- `/open-apis/contract/v1/mcp/...` 会被强制视为 user-only
- 显式 `--as bot` 调用 `contract/v1/mcp` 路径会直接报错

### 4. 合同命令

共享参数：

- `--profile`
- `--as`
- `--output`
- `--raw`
- 需要请求体的命令额外支持 `--input-file` / `--data`
- `contract upload-file` 额外支持 `--file` / `--file-type` / `--file-name`

#### `contract-cli contract search`

用途：搜索合同。

命令：

```bash
contract-cli contract search --profile contract-group --as user --input-file search.json
contract-cli contract search --profile contract-group --as bot --input-file search.json
contract-cli contract search --profile contract-group --as bot --input-file search.json --user-id ou_xxx --user-id-type employee_id
```

支持参数：

- `--contract-number`
- `--page-size`
- `--page-token`
- `--input-file`
- `--data`
- `--user-id-type`
- `--user-id`

按身份路由：

- `--as user`：
  - 走 `/open-apis/contract/v1/mcp/contracts/search`
- `--as bot`：
  - 走 `/open-apis/contract/v1/contracts/search`
- 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string
- 未显式传 `--as` 时：
  - 若 profile 默认身份是 `bot`，则会直接走 bot 搜索路由
  - 若 profile 默认身份是 `user`，则走 user 搜索路由

#### `contract-cli contract get`

用途：获取合同详情。

命令：

```bash
contract-cli contract get <contract-id> --profile contract-group --as user
contract-cli contract get <contract-id> --profile contract-group --as bot
contract-cli contract get <contract-id> --profile contract-group --as bot --user-id ou_xxx --user-id-type employee_id
```

支持参数：

- `--profile`
- `--as`
- `--output`
- `--raw`
- `--user-id-type`
- `--user-id`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/contracts/{contract_id}`
- `--as bot`
  - 走 `/open-apis/contract/v1/contracts/{contract_id}`
- 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string

#### `contract-cli contract sync-user-groups`

用途：同步用户分组。

命令：

```bash
contract-cli contract sync-user-groups --profile contract-group --as user
contract-cli contract sync-user-groups --profile contract-group --as bot
contract-cli contract sync-user-groups --profile contract-group --as bot --user-id ou_xxx
```

支持参数：

- `--profile`
- `--as`
- `--output`
- `--raw`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/contracts/user-groups/sync?user_id_type=user_id`
- `--as bot`
  - 走 `/open-apis/contract/v1/contracts/user-groups/sync`
  - 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string

#### `contract-cli contract text`

用途：获取合同文本。

命令：

```bash
contract-cli contract text <contract-id> --profile contract-group --as user
contract-cli contract text <contract-id> --profile contract-group --as bot
contract-cli contract text <contract-id> --profile contract-group --as bot --user-id-type employee_id
```

支持参数：

- `--profile`
- `--as`
- `--output`
- `--raw`
- `--full-text`
- `--offset`
- `--limit`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/contracts/{contract_id}/text?user_id_type=user_id&...`
- `--as bot`
  - 走 `POST /open-apis/contract/v1/contracts/{contract_id}/text?...`
  - 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string

#### `contract-cli contract create`

用途：创建合同。

命令：

```bash
contract-cli contract create --profile contract-group --input-file create.json
contract-cli contract create --profile contract-group --data '{"title":"demo"}'
contract-cli contract create --profile contract-group --as bot --data '{"contract_name":"demo","create_user_id":"ou_xxx"}'
```

支持参数：

- `--input-file`
- `--data`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/contracts`
- `--as bot`
  - 走 `POST /open-apis/contract/v1/contracts`
  - 请求体需要自己带上 `create_user_id`
  - 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string

字段参考：

- [create-contract-fields.md](/Users/lyy/contract-cli/skills/contract-cli-contract/references/create-contract-fields.md)
- [create-contract-field-tree.md](/Users/lyy/contract-cli/skills/contract-cli-contract/references/create-contract-field-tree.md)
- [create-contract-enums.md](/Users/lyy/contract-cli/skills/contract-cli-contract/references/create-contract-enums.md)

#### `contract-cli contract upload-file`

用途：上传合同相关文件，返回后端原始 JSON，重点关注 `data.file_id`。

命令：

```bash
contract-cli contract upload-file --profile contract-group --as bot --file ./合同正文.docx --file-type text
contract-cli contract upload-file --profile contract-group --as bot --file ./附件.pdf --file-type attachment --file-name 附件.pdf
```

支持参数：

- `--file`：必填，本地待上传文件路径。
- `--file-type`：必填，透传后端文件类型。
- `--file-name`：可选；不传时默认使用本地文件名。
- `--user-id-type`
- `--user-id`

身份规则：

- 当前仅支持 `--as bot`。
- 显式 `--as user` 或 profile 默认身份为 user 时会在发 HTTP 前失败。
- 走 `POST /open-apis/contract/v1/files/upload`。
- 请求是 `multipart/form-data`，字段为 `file_name`、`file_type`、`file`。
- 不接受 `--input-file` / `--data`；这两个参数只用于 JSON 请求体。

本地校验：

- `--file` 必须存在且是普通文件。
- 文件大小必须小于等于 `200MB`。
- CLI 不在本地校验扩展名白名单，扩展名和 `file_type` 合法性由后端最终校验。

常用 `file_type`：

- `text`：合同文本。
- `attachment`：其他附件。
- `scan`：归档扫描件。
- `cause`：合同附件。
- `archiveAttachment`：归档附件。
- `customPictureAttachment` / `customTableAttachment` / `customFileAttachment`：自定义附件。

#### `contract-cli contract category list`

用途：列出合同分类。

命令：

```bash
contract-cli contract category list --profile contract-group
contract-cli contract category list --profile contract-group --as bot --lang zh-CN
```

支持参数：

- `--lang`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/contract_categorys`
- `--as bot`
  - 走 `/open-apis/contract/v1/contract_categorys`

#### `contract-cli contract template list`

用途：列出模板。

命令：

```bash
contract-cli contract template list --profile contract-group
contract-cli contract template list --profile contract-group --as bot --category-number CAT-1 --page-size 20 --user-id ou_xxx --user-id-type employee_id
```

支持参数：

- `--category-number`
- `--page-size`
- `--page-token`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/templates`
- `--as bot`
  - 走 `/open-apis/contract/v1/templates`
  - 按生产文档，`category_number`、`user_id`、`user_id_type` 都属于 bot 接口查询参数
  - CLI 继续按现有约定只透传，不做本地必填校验

#### `contract-cli contract template get`

用途：获取模板详情。

命令：

```bash
contract-cli contract template get <template-id> --profile contract-group
contract-cli contract template get <template-id> --profile contract-group --as bot --user-id ou_xxx --user-id-type employee_id
```

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/templates/{template_id}`
- `--as bot`
  - 走 `/open-apis/contract/v1/templates/{template_id}`
  - 按生产文档，`user_id`、`user_id_type` 都属于 bot 接口查询参数
  - CLI 继续按现有约定只透传，不做本地必填校验

#### `contract-cli contract template instantiate`

用途：创建模板实例。

命令：

```bash
contract-cli contract template instantiate --profile contract-group --input-file template-instance.json
contract-cli contract template instantiate --profile contract-group --as bot --data '{"template_number":"TMP001","create_user_id":"ou_xxx"}' --user-id-type employee_id
```

支持参数：

- `--input-file`
- `--data`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/template_instances`
- `--as bot`
  - 走 `POST /open-apis/contract/v1/template_instances`
  - 按生产文档，query 里只有 `user_id_type`，请求体里需要 `create_user_id`
  - CLI 继续按现有约定只透传，不做本地必填校验

#### `contract-cli contract enum list`

用途：查询枚举值。

命令：

```bash
contract-cli contract enum list --profile contract-group --type contract_status
```

支持参数：

- `--type`

### 5. MDM 命令

这一组命令里，当前 `mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get` 和 `mdm fields list` 同时支持 `user` 与 `bot`。

共享参数：

- `--profile`
- `--as`
- `--output`
- `--raw`

#### `contract-cli mdm vendor list`

用途：查询交易方列表。

命令：

```bash
contract-cli mdm vendor list --profile contract-group --name 供应商 --page-size 10
```

支持参数：

- `--name`
- `--page-size`
- `--page-token`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/vendors`
  - 当前仍保留既有 MCP 查询行为
- `--as bot`
  - 走 `/open-apis/mdm/v1/vendors`
  - 生产文档把 query `vendor` 描述成“供应商编码”
  - CLI 继续沿用现有 `--name -> vendor` 的透传映射，不在本地改名，也不做额外校验

#### `contract-cli mdm vendor get`

用途：查询交易方详情。

命令：

```bash
contract-cli mdm vendor get <vendor-id> --profile contract-group
```

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/vendors/{vendor_id}`
- `--as bot`
  - 走 `/open-apis/mdm/v1/vendors/{vendor_id}`
  - 生产文档里 query 只看到 `user_id_type`
  - CLI 继续按共享约定透传 `--user-id-type` / `--user-id`，不做本地校验

#### `contract-cli mdm legal list`

用途：查询法人主体列表。

命令：

```bash
contract-cli mdm legal list --profile contract-group --name 主体A --page-size 10
```

支持参数：

- `--name`
- `--page-size`
- `--page-token`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/legal_entities`
- `--as bot`
  - 走 `/open-apis/mdm/v1/legal_entities/list_all`
  - 生产文档显示文本使用 `legal_entities/list_all`，但超链接目标误指到了 `vendors`
  - 文档还写了“查询参数采用驼峰式”，但当前 CLI 继续沿用既有 `legalEntity/page_size/page_token` 透传映射，不在本地改名

#### `contract-cli mdm legal get`

用途：查询法人主体详情。

命令：

```bash
contract-cli mdm legal get <legal-entity-id> --profile contract-group
```

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}`
- `--as bot`
  - 走 `/open-apis/mdm/v1/legal_entities/{legal_entity_id}`
  - 按这次确认方案，除了 path 参数外，还会额外拼接同名 query `legal_entity_id`
  - 文档里把 `legal_entity_id` 放在查询参数表里，因此 CLI 按“path + query 双带”的方式实现

#### `contract-cli mdm fields list`

用途：查询字段配置。

命令：

```bash
contract-cli mdm fields list --profile contract-group --biz-line vendor
```

支持参数：

- `--biz-line`

当前支持的典型值：

- `vendor`
- `legal_entity`
- `vendor_risk`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/config/config_list`
- `--as bot`
  - 走 `/open-apis/mdm/v1/config/config_list`
  - 文档显示文本就是这条路径，但超链接目标误指到了 `vendors`
  - query 表使用的是 `biz_line`，CLI 继续沿用 `--biz-line -> biz_line` 的透传映射

## 后续扩展 bot 接口时的建议落点

- 新增 bot 业务接口时，先用 `contract-cli api call ... --as bot` 验证开放平台路径和鉴权是否可用
- 验证稳定后，再决定是否沉淀成新的结构化命令
- 一旦新增结构化 bot 命令，先更新本文档的“命令矩阵”和“身份规则”，再补实现与测试
- 如果未来同一命令同时支持 user 和 bot，需要在文档里明确写出路径差异、参数差异和默认身份规则
