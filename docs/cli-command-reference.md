# contract-cli 命令文档

审批矩阵 `pre-release/release` 已内置全量分页规则行基准检查：预发布前保存内容，正式发布前按行 ID、列 ID 比较，忽略返回顺序并保留数字精度。基准绑定环境、身份、矩阵及预发布版本；异常或变化时停止，需重新预发布并确认。本地基准丢失但服务端 `status=0` 时，pre-release 双读版本与规则行后只恢复基准并返回 `baseline_recovered=true`，不重复预发布或改写规则行。仅核对规则行，不提供原子并发保护，不依赖 snapshot。

本文档汇总当前代码里已经实际支持的 `contract-cli` 命令，作为后续继续扩展 app 接口和新业务命令的基线。

## 当前状态

- 所有当前构建只支持 `prod` 环境预设；历史 test/blue/dev profile 和对应网络端点会在请求前被拒绝，不自动迁移凭据。
- `contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`contract upload-file`、`mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get`、`mdm fields list` 是当前仅有的十五个同时支持 `user` 与 `app` 的结构化业务命令
- `contract search-v2`、`contract field update`、`contract sign switch-to-paper`、`contract sign-url get`、`contract form attribute list`、`contract authorization grant`、`contract esign *`、`contract submit/resubmit/patch/download-file/delete/print-file`、`contract share get/batch-create`、`contract cooperation link/record/search/file`、`contract approval start/get`、`payment *`、`mdm vendor create/update/list-all/query-by-cert`、`mdm legal get --code/create/update`、`mdm fixed-exchange-rate get/update`、`mdm file download`、`event outbound-ip list` 当前仅支持 `--as app`
- `rule *`（包括 `approval-matrix` 别名）支持 `--as user` / `--as app`；user 当前限 contract 产品且需要合同规则管理权限。
- 除上述双身份和 app-only 能力外，当前其他结构化业务命令仍只支持 `--as user`
- `app` 目前已经支持登录、状态查看、登出、默认身份切换
- 推荐使用 `npx skills add qfeius/contract-cli -y -g` 安装跨 Agent 平台 skills；`contract-cli skills install` 保留为 CLI 内置兜底
- `update check` 支持手动检查 npm 远端版本；默认输出文本，带 `--json` 时返回飞书式 JSON；CLI 会为符合条件的普通命令按 24 小时缓存检查远端版本，并在 JSON object 输出中注入 `_notice.update`
- 当前全部已支持命令都可以通过 `--help` 查看本地帮助，例如 `contract-cli --help`、`contract-cli contract search --help`、`contract-cli help contract upload-file`
- `app` 业务接口后续继续新增时，优先在本文件补充命令矩阵

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
- `Notes` 只放身份限制、user/app 路由差异、请求体或文件上传关键约束
- 不兼容旧顶层别名，例如 `contract-cli help vendor` 会返回未知 help topic

### 通用身份规则

- `config` 和 `version` 不需要登录态
- `skills list/install` 不需要登录态；通用 `npx skills add qfeius/contract-cli -y -g` 也不依赖 contract-cli 登录态
- `update check` 不需要登录态
- `auth login --as user` 走 OAuth 用户授权
- `auth login --as app` 走 `appId + appSecret -> tenant_access_token/internal`
- 为兼容老用户脚本，旧身份值 `--as bot` 仍可使用，运行时等价于 `--as app`；新文档和示例统一使用 `app`
- `contract ...`、`mdm ...` 结构化命令大多默认只支持 `--as user`
- `/open-apis/contract/v1/mcp/...` 路径大多仍只支持 `--as user`
- `contract search-v2`、`contract field update`、`contract sign switch-to-paper`、`contract sign-url get`、`contract form attribute list`、`contract authorization grant`、`contract esign *`、`contract submit`、`contract resubmit`、`contract patch`、`contract download-file`、`contract delete`、`contract print-file`、`contract share get/batch-create`、`contract cooperation link/record/search/file`、`contract approval start/get`、`payment *`、新增写入和扩展查询型 `mdm *`、`event outbound-ip list` 当前仅支持 `--as app`
- `contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`contract upload-file`、`mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get`、`mdm fields list` 是例外：
  - `contract get --as user` 走 MCP 路径 `/open-apis/contract/v1/mcp/contracts/{contract_id}`
  - `contract get --as app` 走开放平台路径 `/open-apis/contract/v1/contracts/{contract_id}`
  - `--as user` 走 MCP 路径 `/open-apis/contract/v1/mcp/contracts/search`
  - `--as app` 走开放平台路径 `/open-apis/contract/v1/contracts/search`
  - `contract create --as user` 走 MCP 路径 `/open-apis/contract/v1/mcp/contracts`
  - `contract create --as app` 走开放平台路径 `POST /open-apis/contract/v1/contracts`
  - `contract sync-user-groups --as user` 走 `/open-apis/contract/v1/mcp/contracts/user-groups/sync?user_id_type=user_id`
  - `contract sync-user-groups --as app` 走 `/open-apis/contract/v1/contracts/user-groups/sync`
  - `contract text --as user` 走 `/open-apis/contract/v1/mcp/contracts/{contract_id}/text?user_id_type=user_id&...`
  - `contract text --as app` 走 `GET /open-apis/contract/v1/contracts/{contract_id}/text?...`
  - `contract category list --as user` 走 `/open-apis/contract/v1/mcp/contract_categorys`
  - `contract category list --as app` 走 `/open-apis/contract/v1/contract_categorys`
  - `contract template list --as user` 走 `/open-apis/contract/v1/mcp/templates`
  - `contract template list --as app` 走 `/open-apis/contract/v1/templates`
  - `contract template get --as user` 走 `/open-apis/contract/v1/mcp/templates/{template_id}`
  - `contract template get --as app` 走 `/open-apis/contract/v1/templates/{template_id}`
  - `contract template instantiate --as user` 走 `/open-apis/contract/v1/mcp/template_instances`
  - `contract template instantiate --as app` 走 `POST /open-apis/contract/v1/template_instances`
  - `contract upload-file --as user` 与 `contract upload-file --as app` 均走 `POST /open-apis/contract/v1/files/upload`
  - `mdm vendor list --as user` 走 `/open-apis/contract/v1/mcp/vendors`
  - `mdm vendor list --as app` 走 `/open-apis/mdm/v1/vendors`
  - `mdm vendor get --as user` 走 `/open-apis/contract/v1/mcp/vendors/{vendor_id}`
  - `mdm vendor get --as app` 走 `/open-apis/mdm/v1/vendors/{vendor_id}`
  - `mdm legal list --as user` 走 `/open-apis/contract/v1/mcp/legal_entities`
  - `mdm legal list --as app` 走 `/open-apis/mdm/v1/legal_entities/list_all`
  - `mdm legal get --as user` 走 `/open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}`
  - `mdm legal get --as app` 走 `/open-apis/mdm/v1/legal_entities/{legal_entity_id}`，并额外透传同名 query `legal_entity_id`
  - `mdm fields list --as user` 走 `/open-apis/contract/v1/mcp/config/config_list`
  - `mdm fields list --as app` 走 `/open-apis/mdm/v1/config/config_list`
- `api call` 是预留能力，当前暂未开放使用；请优先使用已开放的结构化命令

### 通用输出

结构化业务命令共享这些输出参数：

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
- `--user-id-type` 不传时默认拼接 `user_id_type=user_id`
- 显式传 `--user-id-type <type>` 时会覆盖默认值
- `--user-id` 传了就拼接到 query string，不传就不带
- 不区分 `user` / `app`
- 除 `mdm vendor create/update` 与 `mdm legal create/update` 外不做命令级校验；这四个 MDM 写接口会本地要求 `--user-id`

## 命令矩阵

### 1. 配置与版本

#### `contract-cli config add`

用途：初始化或更新 profile，并写入 user OAuth 与 app token 的基础配置。

命令：

```bash
contract-cli config add --env prod --name contract
```

支持参数：

- `--env`：环境预设，当前仅支持 `prod`，默认 `prod`
- `--name`：profile 名称，默认 `contract`
- `--resource-metadata-url`：覆盖 protected resource metadata 地址
- `--redirect-url`：覆盖 OAuth callback 地址
- `--scope`：覆盖默认 scope 列表

执行结果：

- 写入 `open_platform_base_url`
- 写入 user OAuth metadata
- 写入 app `app_token_endpoint`
- 将 profile 设为当前 profile

#### `contract-cli version`

用途：查看当前 CLI 版本、commit、构建时间和功能基线。审批矩阵扩展基线为 `approval-matrix-extensions`，自 `1.8.3-test.13`（提交 `61d8aa6`）起包含新增矩阵命令。

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
contract-cli update check --channel latest
contract-cli update check --channel latest --json
```

支持参数：

- `--channel`：npm dist-tag；不传时根据当前版本推断，预发布版本默认检查 `beta`，稳定版本默认检查 `latest`
- `--json`：输出飞书式结构化 JSON；默认输出文本提示

执行结果：

- 当前版本是 `dev`、`unknown` 或非语义化版本（例如源码 git hash）时跳过远端检查
- 默认输出文本提示，和飞书 `lark-cli update --check` 的手动校验体验保持一致
- 带 `--json` 时输出顶层 `ok`、`previous_version`、`current_version`、`latest_version`、`action`、`message` 等字段
- 有新版本时 `action=update_available`，并额外包含 `command`，值为 `npm install -g @qfeius/contract-cli@<channel> --registry https://registry.npmjs.org`
- 无新版本时 `action=already_up_to_date`
- 手动 `update check --json` 不注入 `_notice.update`；`_notice.update` 只用于普通 JSON 业务命令的自动提示
- 手动执行 `update check` 会直接访问 npm registry，并把结果写入本机 update cache

自动提示：

- 普通命令会先同步读取当前配置目录的 `update-check.json`；缓存里有可升级版本时，仅在 JSON object 输出中注入 `_notice.update`
- 命中 fresh cache 时不访问 npm registry，因此不会立即发现刚发布的新包
- cache 缺失、channel 不匹配或过期时，当前命令会在短超时内同步刷新远端版本；成功结果会写入当前配置目录的 `update-check.json`
- 网络失败、registry 失败或当前是 dev 构建时不会阻断原命令；刷新失败不会写入失败缓存
- `--raw`、yaml、table、纯文本命令不注入 `_notice.update`
- CI 环境会跳过自动远端检查
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
contract-cli skills install --name contract-cli-rule --force
contract-cli skills install --force
```

支持参数：

- `--target`：安装目标目录；默认优先使用 `$CODEX_HOME/skills`，否则使用 `~/.codex/skills`
- `--name`：只安装指定的内置 Skill，例如 `contract-cli-rule`；不传时处理全部
- `--force`：覆盖已存在的同名 skill；默认不覆盖，会跳过已有目录
- 更新本地规则 Skill 时建议组合 `--name contract-cli-rule --force`，仅替换该目录；若有本地自定义内容，先备份。单独使用 `--force` 会覆盖所有同名内置 Skill

执行结果：

- 复制内置 `auth`、`contract-cli-shared`、`contract-cli-contract`、`contract-cli-payment`、`contract-cli-mdm-vendor`、`contract-cli-mdm-legal`、`contract-cli-mdm-fields` 等 skill
- 保留 `SKILL.md`、`agents/openai.yaml` 和 `references/*.md`

### 2. 鉴权

#### `contract-cli auth login`

##### `contract-cli auth login --as user`

用途：发起 OAuth 用户授权。

命令：

```bash
contract-cli auth login --profile contract --as user
```

支持参数：

- `--profile`
- `--as user`
- `--timeout`
- `--no-open-browser`

##### `contract-cli auth login --as app`

用途：使用 app `appId/appSecret` 直接换取 tenant access token。

命令：

```bash
contract-cli auth login --profile contract --as app --app-id <id> --app-secret <secret>
```

支持参数：

- `--profile`
- `--as app`
- `--app-id`
- `--app-secret`

补充说明：

- app 凭证优先级：flag > env > 已保存 secrets
- 登录成功后会保存 app token，并将默认身份切到 `app`
- `auth logout --as app` 只清 token，不删除 `appId/appSecret`
- 兼容旧命令 `auth login --as bot`，实际按 app 身份登录并写入 `identities.app`

#### `contract-cli auth status`

用途：查看某个 profile 的 user 或 app 身份状态。

命令：

```bash
contract-cli auth status --profile contract --as user
contract-cli auth status --profile contract --as app
```

支持参数：

- `--profile`
- `--as user|app`

当前状态语义：

- user：`authorized` / `unauthorized`
- app：`authorized` / `expired` / `configured` / `unconfigured`

#### `contract-cli auth logout`

用途：清理指定身份的 token。

命令：

```bash
contract-cli auth logout --profile contract --as user
contract-cli auth logout --profile contract --as app
```

支持参数：

- `--profile`
- `--as user|app`

补充说明：

- user logout：清空 user token
- app logout：只清空 app token，保留 app 凭证

#### `contract-cli auth use`

用途：切换 profile 默认业务身份。

命令：

```bash
contract-cli auth use --profile contract --as user
contract-cli auth use --profile contract --as app
```

支持参数：

- `--profile`
- `--as user|app`

### 3. 原始开放平台调用（暂未开放）

`contract-cli api call` 是预留调试入口，当前不对外开放。

当前行为：

- 执行 `contract-cli api ...` 会直接返回：`api call 暂未开放使用，请使用已开放的结构化命令`
- 不读取 profile，不发 HTTP 请求
- 不出现在 `contract-cli --help`、`contract-cli help` 或内置 skills 安装列表中
- 需要开放平台能力时，请优先使用 `contract ...`、`mdm ...` 等结构化命令
- 显式 `--as app` 调用 `contract/v1/mcp` 路径会直接报错

### 4. 合同命令

共享参数：

- `--profile`
- `--as`
- `--output`
- `--raw`
- 需要请求体的命令额外支持 `--input-file` / `--data`
- `contract upload-file` 额外支持 `--file` / `--file-type` / `--file-name`
- `contract download-file` 额外支持 `--output-file` / `--force`

#### `contract-cli contract search`

用途：搜索合同。

命令：

```bash
contract-cli contract search --profile contract --as user --input-file search.json
contract-cli contract search --profile contract --as app --input-file search.json
contract-cli contract search --profile contract --as app --input-file search.json --user-id ou_xxx --user-id-type employee_id
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
- `--as app`：
  - 走 `/open-apis/contract/v1/contracts/search`
- 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string
- 未显式传 `--as` 时：
  - 若 profile 默认身份是 `app`，则会直接走 app 搜索路由
  - 若 profile 默认身份是 `user`，则走 user 搜索路由

#### `contract-cli contract search-v2`

用途：app 身份搜索合同 V2，复杂条件直接透传 JSON body。

命令：

```bash
contract-cli contract search-v2 --profile contract --as app --input-file search-v2.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/contracts/searchV2`。
- `--input-file` / `--data` 必须传一个且互斥。

#### `contract-cli contract get`

用途：获取合同详情。

命令：

```bash
contract-cli contract get <contract-id> --profile contract --as user
contract-cli contract get <contract-id> --profile contract --as app
contract-cli contract get <contract-id> --profile contract --as app --user-id ou_xxx --user-id-type employee_id
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
- `--as app`
  - 走 `/open-apis/contract/v1/contracts/{contract_id}`
- 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string

#### `contract-cli contract sync-user-groups`

用途：同步用户分组。

命令：

```bash
contract-cli contract sync-user-groups --profile contract --as user
contract-cli contract sync-user-groups --profile contract --as app
contract-cli contract sync-user-groups --profile contract --as app --user-id ou_xxx
```

支持参数：

- `--profile`
- `--as`
- `--output`
- `--raw`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/contracts/user-groups/sync?user_id_type=user_id`
- `--as app`
  - 走 `/open-apis/contract/v1/contracts/user-groups/sync`
  - 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string

#### `contract-cli contract text`

用途：获取合同文本。

命令：

```bash
contract-cli contract text <contract-id> --profile contract --as user
contract-cli contract text <contract-id> --profile contract --as app
contract-cli contract text <contract-id> --profile contract --as app --user-id-type employee_id
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
- `--as app`
  - 走 `GET /open-apis/contract/v1/contracts/{contract_id}/text?...`
  - 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string

#### `contract-cli contract create`

用途：创建合同。

命令：

```bash
contract-cli contract create --profile contract --input-file create.json
contract-cli contract create --profile contract --data '{"title":"demo"}'
contract-cli contract create --profile contract --as app --data '{"contract_name":"demo","create_user_id":"ou_xxx"}'
```

支持参数：

- `--input-file`
- `--data`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/contracts`
- `--as app`
  - 走 `POST /open-apis/contract/v1/contracts`
  - 请求体需要自己带上 `create_user_id`
  - 额外传入 `--user-id-type` / `--user-id` 时，会原样拼到 query string

字段参考：

- [create-contract-fields.md](../skills/contract-cli-contract/references/create-contract-fields.md)
- [create-contract-field-tree.md](../skills/contract-cli-contract/references/create-contract-field-tree.md)
- [create-contract-enums.md](../skills/contract-cli-contract/references/create-contract-enums.md)

#### `contract-cli contract field update`

用途：app 身份更新合同字段信息，目前主要用于修改下拉列表选项范围。

命令：

```bash
contract-cli contract field update --profile contract --as app --input-file field-update.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `PUT /open-apis/contract/v1/attribute_definition`。
- 最小 body 通常包含 `module_name`、`attribute_name`、`value_scopes`。

#### `contract-cli contract sign switch-to-paper`

用途：app 身份将电子签合同转为纸质签。

命令：

```bash
contract-cli contract sign switch-to-paper --profile contract --as app --business-id <contract-id> --business-type-code 0
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/contracts/signType/switchToPaper`。
- `--business-id` 和 `--business-type-code` 必填，不接受 `--input-file` / `--data`。

#### `contract-cli contract sign-url get`

用途：app 身份获取合同签署链接。

命令：

```bash
contract-cli contract sign-url get <contract-id> --profile contract --as app
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/{contract_id}/sign_url`。
- 不接受 `--input-file` / `--data`。

#### `contract-cli contract form attribute list`

用途：app 身份按合同类型和流程类型获取合同流程字段。

命令：

```bash
contract-cli contract form attribute list --profile contract --as app --category-id <category-id> --business-type-code 0
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/form_definition/attribute`。
- `--business-type-code`：0 申请、1 变更、2 终止、3 合同组申请。

#### `contract-cli contract authorization grant`

用途：app 身份授予合同权限。

命令：

```bash
contract-cli contract authorization grant --profile contract --as app --input-file authorization.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/authorizations`。
- 最小 body 通常包含 `business_id`、`authorized_user_id`、`start_time`、`end_time`。

#### `contract-cli contract esign personal-auth-url`

用途：app 身份获取个人认证和授权页面链接。

命令：

```bash
contract-cli contract esign personal-auth-url --profile contract --as app --input-file psn-auth-url.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/esign/auth/psnAuthUrl`。
- 最小 body 需要包含 `psnAuthConfig`。

#### `contract-cli contract esign org-auth-url`

用途：app 身份获取机构认证和授权页面链接。

命令：

```bash
contract-cli contract esign org-auth-url --profile contract --as app --input-file org-auth-url.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/esign/auth/orgAuthUrl`。
- 最小 body 需要包含 `orgAuthConfig`。

#### `contract-cli contract upload-file`

用途：上传合同相关文件，返回后端原始 JSON，重点关注 `data.file_id`。

命令：

```bash
contract-cli contract upload-file --profile contract --as user --file ./合同正文.docx --file-type text
contract-cli contract upload-file --profile contract --as app --file ./附件.pdf --file-type attachment --file-name 附件.pdf
```

支持参数：

- `--file`：必填，本地待上传文件路径。
- `--file-type`：必填，透传后端文件类型。
- `--file-name`：可选；不传时默认使用本地文件名。
- `--user-id-type`
- `--user-id`

身份规则：

- `--as user` 和 `--as app` 均支持。
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

#### `contract-cli contract submit`

用途：app 身份提交合同。

命令：

```bash
contract-cli contract submit <contract-id> --profile contract --as app
contract-cli contract submit <contract-id> --profile contract --as app --data '{"comment":"ok"}'
```

支持参数：

- `--input-file`：可选，透传 JSON 请求体。
- `--data`：可选，透传 JSON 请求体。
- `--user-id-type`
- `--user-id`

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/contracts/{contract_id}/submit`。
- 不传 `--input-file` / `--data` 时不发送请求体。

#### `contract-cli contract resubmit`

用途：app 身份重新提交合同。

命令：

```bash
contract-cli contract resubmit <contract-id> --profile contract --as app
contract-cli contract resubmit <contract-id> --profile contract --as app --input-file resubmit.json
```

支持参数：

- `--input-file`：可选，透传 JSON 请求体。
- `--data`：可选，透传 JSON 请求体。
- `--user-id-type`
- `--user-id`

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/contracts/{contract_id}/resubmit`。
- 不传 `--input-file` / `--data` 时不发送请求体。

#### `contract-cli contract patch`

用途：app 身份更新合同。

命令：

```bash
contract-cli contract patch <contract-id> --profile contract --as app --input-file patch.json
contract-cli contract patch <contract-id> --profile contract --as app --data '{"title":"demo"}'
```

支持参数：

- `--input-file`
- `--data`
- `--user-id-type`
- `--user-id`

身份规则：

- 当前仅支持 `--as app`。
- 走 `PATCH /open-apis/contract/v1/contracts/{contract_id}`。
- `--input-file` / `--data` 必须传一个且互斥。

#### `contract-cli contract download-file`

用途：app 身份下载合同相关文件。

命令：

```bash
contract-cli contract download-file <file-id> --profile contract --as app
contract-cli contract download-file <file-id> --profile contract --as app --output-file ./contract.pdf
contract-cli contract download-file <file-id> --profile contract --as app --raw > contract.pdf
```

支持参数：

- `--output-file`：保存到指定文件；不传时默认拉起保存文件弹窗。
- `--force`：覆盖已存在的 `--output-file`。
- `--raw`：把文件内容写到 stdout，不打印额外提示。
- `--user-id-type`
- `--user-id`

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/files/{file_id}`。
- 不实现 `dowload-file` 拼写别名。
- 无 GUI、远程、CI、Agent 环境推荐显式传 `--output-file`。

#### `contract-cli contract delete`

用途：app 身份删除草稿合同。

命令：

```bash
contract-cli contract delete <contract-id> --profile contract --as app
```

支持参数：

- `--user-id-type`
- `--user-id`

身份规则：

- 当前仅支持 `--as app`。
- 走 `DELETE /open-apis/contract/v1/contracts/{contract_id}`。
- 命令直接删除，不额外要求 `--yes`。

#### `contract-cli contract print-file`

用途：app 身份生成合同打印文件。

命令：

```bash
contract-cli contract print-file --profile contract --as app --input-file print-file.json
contract-cli contract print-file --profile contract --as app --data '{"contract_id":"<contract-id>"}'
```

支持参数：

- `--input-file`
- `--data`
- `--user-id-type`
- `--user-id`

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/files`。
- `--input-file` / `--data` 必须传一个且互斥。

#### `contract-cli contract share get`

用途：app 身份查询合同分享记录。

命令：

```bash
contract-cli contract share get <contract-id> --profile contract --as app
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/{contract_id}/share_records`。

#### `contract-cli contract share batch-create`

用途：app 身份批量分享合同。

命令：

```bash
contract-cli contract share batch-create --profile contract --as app --input-file batch-share.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/contracts/contract/batch_share`。
- 最小 body 通常包含 `contract_id` 和 `user_ids`。

#### `contract-cli contract cooperation link get`

用途：app 身份查询合同协商邀请链接。

命令：

```bash
contract-cli contract cooperation link get <contract-id> --profile contract --as app
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_link`。

#### `contract-cli contract cooperation record get`

用途：app 身份查询合同协商操作记录信息。

命令：

```bash
contract-cli contract cooperation record get <contract-id> --profile contract --as app
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_record_info`。

#### `contract-cli contract cooperation search`

用途：app 身份查询协商列表。

命令：

```bash
contract-cli contract cooperation search --profile contract --as app --input-file cooperation-search.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/cooperation/search`。
- 最小 body 需要包含 `user_id`，分页可放在 body 的 `page_size` / `page_token`。

#### `contract-cli contract cooperation file get`

用途：app 身份查询合同协商文件信息。

命令：

```bash
contract-cli contract cooperation file get <contract-id> --profile contract --as app
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/{contract_id}/cooperation/file_info`。

#### `contract-cli contract cooperation file download`

用途：app 身份下载合同协商文件。

命令：

```bash
contract-cli contract cooperation file download <file-id> --profile contract --as app --output-file ./cooperation.docx
contract-cli contract cooperation file download <file-id> --profile contract --as app --raw > cooperation.docx
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/cooperation/{file_id}/download_file`。
- `--raw` 会把二进制内容写到 stdout，不打印额外提示。

#### `contract-cli contract approval start`

用途：app 身份发起流程审批。

命令：

```bash
contract-cli contract approval start <process-instance-id> --profile contract --as app --input-file approval.json
```

支持参数：

- `--input-file`
- `--data`
- `--user-id-type`
- `--user-id`

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/process_instances/{process_instance_id}/task_approval`。
- `--input-file` / `--data` 必须传一个且互斥。

#### `contract-cli contract approval get`

用途：app 身份查询审批实例详情。

命令：

```bash
contract-cli contract approval get <process-instance-id> --profile contract --as app
contract-cli contract approval get <process-instance-id> --profile contract --as app --notice-filter notice_filter --task-instance-filter task_instance_filter
```

支持参数：

- `--notice-filter`
- `--task-instance-filter`
- `--user-id-type`
- `--user-id`

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/process_instances/{process_instance_id}`。
- 不接受 `--input-file` / `--data`。

#### `contract-cli contract category list`

用途：列出合同分类。

命令：

```bash
contract-cli contract category list --profile contract
contract-cli contract category list --profile contract --as app --lang zh-CN
```

支持参数：

- `--lang`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/contract_categorys`
- `--as app`
  - 走 `/open-apis/contract/v1/contract_categorys`

#### `contract-cli contract template list`

用途：列出模板。

命令：

```bash
contract-cli contract template list --profile contract
contract-cli contract template list --profile contract --as app --category-number CAT-1 --page-size 20 --user-id ou_xxx --user-id-type employee_id
```

支持参数：

- `--category-number`
- `--page-size`
- `--page-token`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/templates`
- `--as app`
  - 走 `/open-apis/contract/v1/templates`
  - 按生产文档，`category_number`、`user_id`、`user_id_type` 都属于 app 接口查询参数
  - CLI 继续按现有约定只透传，不做本地必填校验

#### `contract-cli contract template get`

用途：获取模板详情。

命令：

```bash
contract-cli contract template get <template-id> --profile contract
contract-cli contract template get <template-id> --profile contract --as app --user-id ou_xxx --user-id-type employee_id
```

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/templates/{template_id}`
- `--as app`
  - 走 `/open-apis/contract/v1/templates/{template_id}`
  - 按生产文档，`user_id`、`user_id_type` 都属于 app 接口查询参数
  - CLI 继续按现有约定只透传，不做本地必填校验

#### `contract-cli contract template instantiate`

用途：创建模板实例。

命令：

```bash
contract-cli contract template instantiate --profile contract --input-file template-instance.json
contract-cli contract template instantiate --profile contract --as app --data '{"template_number":"TMP001","create_user_id":"ou_xxx"}' --user-id-type employee_id
```

支持参数：

- `--input-file`
- `--data`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/template_instances`
- `--as app`
  - 走 `POST /open-apis/contract/v1/template_instances`
  - 按生产文档，query 里只有 `user_id_type`，请求体里需要 `create_user_id`
  - CLI 继续按现有约定只透传，不做本地必填校验

#### `contract-cli contract enum list`

用途：查询枚举值。

命令：

```bash
contract-cli contract enum list --profile contract --type contract_status
```

支持参数：

- `--type`

### 5. 付款命令

`payment` 这一组命令当前全部仅支持 `--as app`。命令参数采用“主操作对象 ID 用位置参数，父资源 ID 用 flag”的方式。

#### `contract-cli payment create`

用途：创建付款申请。

命令：

```bash
contract-cli payment create --contract <contract-id> --profile contract --as app --input-file payment.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/contracts/{contract_id}/payments`。
- `--contract` 必填。
- `--input-file` / `--data` 必填且互斥。

#### `contract-cli payment update`

用途：更新付款信息。

命令：

```bash
contract-cli payment update <payment-id> --contract <contract-id> --profile contract --as app --input-file payment-update.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `PATCH /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}`。
- `--contract` 必填。
- `--input-file` / `--data` 必填且互斥。

#### `contract-cli payment get`

用途：查看付款信息。

命令：

```bash
contract-cli payment get <payment-id> --contract <contract-id> --profile contract --as app
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}`。
- `--contract` 必填。
- 不接受 `--input-file` / `--data`。

#### `contract-cli payment list`

用途：查询付款申请列表。

命令：

```bash
contract-cli payment list --contract <contract-id> --profile contract --as app
contract-cli payment list --contract <contract-id> --profile contract --as app --page-size 10 --page-token next
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/{contract_id}/payments`。
- `--contract` 必填。
- `--page-size` / `--page-token` 可选。
- 不接受 `--input-file` / `--data`。

#### `contract-cli payment plan notify`

用途：同步付款记录。

命令：

```bash
contract-cli payment plan notify --profile contract --as app --input-file notify.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/payment/notify`。
- `--input-file` / `--data` 必填且互斥。

#### `contract-cli payment plan search`

用途：搜索付款计划。

命令：

```bash
contract-cli payment plan search --profile contract --as app --input-file payment-plan-search.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/payments/search`。
- `--input-file` / `--data` 必填且互斥。

#### `contract-cli payment record create`

用途：创建付款记录。

命令：

```bash
contract-cli payment record create --contract <contract-id> --payment <payment-id> --profile contract --as app --input-file payment-record.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `POST /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}/payment_records`。
- `--contract` / `--payment` 必填。
- `--input-file` / `--data` 必填且互斥。

#### `contract-cli payment record update`

用途：更新付款记录。

命令：

```bash
contract-cli payment record update <payment-record-id> --contract <contract-id> --payment <payment-id> --profile contract --as app --input-file payment-record-update.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `PATCH /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}/payment_records/{payment_record_id}`。
- `--contract` / `--payment` 必填。
- `--input-file` / `--data` 必填且互斥。

#### `contract-cli payment record get`

用途：查询付款记录详情。

命令：

```bash
contract-cli payment record get <payment-record-id> --contract <contract-id> --payment <payment-id> --profile contract --as app
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}/payment_records/{payment_record_id}`。
- `--contract` / `--payment` 必填。
- 不接受 `--input-file` / `--data`。

#### `contract-cli payment record list`

用途：根据付款计划 ID 查询付款记录。

命令：

```bash
contract-cli payment record list --plan <payment-plan-uuid> --profile contract --as app
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/contract/v1/contracts/payments/{payment_plan_uuid}/payment_records`。
- `--plan` 必填。
- 不接受 `--input-file` / `--data`。

### 6. MDM 命令

这一组命令里，当前 `mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get` 和 `mdm fields list` 同时支持 `user` 与 `app`。

共享参数：

- `--profile`
- `--as`
- `--output`
- `--raw`

#### `contract-cli mdm vendor list`

用途：查询交易方列表。

命令：

```bash
contract-cli mdm vendor list --profile contract --name 供应商 --page-size 10
```

支持参数：

- `--name`
- `--page-size`
- `--page-token`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/vendors`
  - 当前仍保留既有 MCP 查询行为
- `--as app`
  - 走 `/open-apis/mdm/v1/vendors`
  - 生产文档把 query `vendor` 描述成“供应商编码”
  - CLI 继续沿用现有 `--name -> vendor` 的透传映射，不在本地改名，也不做额外校验

#### `contract-cli mdm vendor get`

用途：查询交易方详情。

命令：

```bash
contract-cli mdm vendor get <vendor-id> --profile contract
```

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/vendors/{vendor_id}`
- `--as app`
  - 走 `/open-apis/mdm/v1/vendors/{vendor_id}`
  - 生产文档里 query 只看到 `user_id_type`
  - CLI 继续按共享约定透传 `--user-id-type` / `--user-id`，不做本地校验

#### `contract-cli mdm legal list`

用途：查询法人主体列表。

命令：

```bash
contract-cli mdm legal list --profile contract --name 主体A --page-size 10
```

支持参数：

- `--name`
- `--page-size`
- `--page-token`

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/legal_entities`
- `--as app`
  - 走 `/open-apis/mdm/v1/legal_entities/list_all`
  - 生产文档显示文本使用 `legal_entities/list_all`，但超链接目标误指到了 `vendors`
  - 文档还写了“查询参数采用驼峰式”，但当前 CLI 继续沿用既有 `legalEntity/page_size/page_token` 透传映射，不在本地改名

#### `contract-cli mdm legal get`

用途：查询法人主体详情。

命令：

```bash
contract-cli mdm legal get <legal-entity-id> --profile contract
contract-cli mdm legal get --profile contract --as app --code L0001 --page-size 10
```

支持参数：

- `<legal-entity-id>`：按 ID 查询详情
- `--code`：按法人实体编码查询，映射到底层 query `legalEntity`
- `--page-size`：仅 `--code` 模式可用
- `--page-token`：仅 `--code` 模式可用

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}`
- `--as app`
  - 按 ID 查询走 `/open-apis/mdm/v1/legal_entities/{legal_entity_id}`
  - 按这次确认方案，除了 path 参数外，还会额外拼接同名 query `legal_entity_id`
  - 文档里把 `legal_entity_id` 放在查询参数表里，因此 CLI 按“path + query 双带”的方式实现
  - 传 `--code` 时走 `GET /open-apis/mdm/v1/legal_entities`
  - `--code` 模式不同于 `mdm legal list --as app` 的 `/open-apis/mdm/v1/legal_entities/list_all`

#### `contract-cli mdm fields list`

用途：查询字段配置。

命令：

```bash
contract-cli mdm fields list --profile contract --biz-line vendor
```

支持参数：

- `--biz-line`

当前支持的典型值：

- `vendor`
- `legal_entity`：user 原样透传；app 会自动映射为 `legalEntity`
- `vendor_risk`：仅 user/MCP 路径可用，app 当前不支持

身份规则：

- `--as user`
  - 走 `/open-apis/contract/v1/mcp/config/config_list`
  - `--biz-line` 可传 `vendor`、`legal_entity`、`vendor_risk`
- `--as app`
  - 走 `/open-apis/mdm/v1/config/config_list`
  - 文档显示文本就是这条路径，但超链接目标误指到了 `vendors`
  - 后端当前只接受 `vendor` 或 `legalEntity`
  - CLI 允许继续传 `legal_entity`，并在 app 路由下自动映射为 `legalEntity`
  - `vendor_risk` 在 app 身份下会被本地拒绝，不再发送请求

#### `contract-cli mdm vendor create`

用途：app 身份创建交易方。

命令：

```bash
contract-cli mdm vendor create --profile contract --as app --user-id <operator-user-id> --input-file vendor-create.json
```

身份规则：

- 当前仅支持 `--as app`。
- 必须传 `--user-id`，用于提供当前操作人上下文。
- 走 `POST /open-apis/mdm/v1/vendors`。
- 创建请求体不要包含后端生成的 `vendor` 编码。
- 字段是否必填受后台动态配置影响，可先查 `mdm fields list --biz-line vendor`。

#### `contract-cli mdm vendor update`

用途：app 身份按 ID 更新交易方。

命令：

```bash
contract-cli mdm vendor update <vendor-id> --profile contract --as app --user-id <operator-user-id> --input-file vendor-update.json
```

身份规则：

- 当前仅支持 `--as app`。
- 必须传 `--user-id`，用于提供当前操作人上下文。
- 走 `PUT /open-apis/mdm/v1/vendors/{vendor_id}`。
- 请求体必须包含后端返回的 `id` 和 `vendor` 编码；CLI 不在本地补动态字段。

#### `contract-cli mdm vendor list-all`

用途：app 身份分页查询交易方全量数据。

命令：

```bash
contract-cli mdm vendor list-all --profile contract --as app --page-size 10 --page-token next
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/mdm/v1/vendors/list_all`。
- 不接受 `--input-file` / `--data`。

#### `contract-cli mdm vendor query-by-cert`

用途：app 身份根据证件 ID 和国家地区精确查询交易方。

命令：

```bash
contract-cli mdm vendor query-by-cert --profile contract --as app --certification-id 91110105 --ad-country CN
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/mdm/v1/vendors/query_vendors`。
- `--certification-id` 和 `--ad-country` 必填。

#### `contract-cli mdm legal create`

用途：app 身份创建法人主体。

命令：

```bash
contract-cli mdm legal create --profile contract --as app --user-id <operator-user-id> --input-file legal-create.json
```

身份规则：

- 当前仅支持 `--as app`。
- 必须传 `--user-id`，用于提供当前操作人上下文。
- 走 `POST /open-apis/mdm/v1/legal_entities`。
- 创建请求体不要包含后端生成的 `legalEntity` / `legal_entity` 编码。
- 字段是否必填受后台动态配置影响，可先查 `mdm fields list --biz-line legal_entity`。

#### `contract-cli mdm legal update`

用途：app 身份按 ID 更新法人主体。

命令：

```bash
contract-cli mdm legal update <legal-entity-id> --profile contract --as app --user-id <operator-user-id> --input-file legal-update.json
```

身份规则：

- 当前仅支持 `--as app`。
- 必须传 `--user-id`，用于提供当前操作人上下文。
- 走 `PUT /open-apis/mdm/v1/legal_entities/{legal_entity_id}`。
- 请求体必须包含后端返回的 `id` 和 camelCase `legalEntity` 编码，不要写成 `legal_entity`；CLI 不在本地补动态字段。

#### `contract-cli mdm fixed-exchange-rate get`

用途：app 身份查询固定汇率。

命令：

```bash
contract-cli mdm fixed-exchange-rate get --profile contract --as app --source-currency CNY --target-currency USD --effective-date 2026-06-01
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/mdm/v1/fixed_exchange_rate`。
- CLI flag `--effective-date` 会映射到底层 query 参数 `date`。
- 不接受 `--input-file` / `--data`。

#### `contract-cli mdm fixed-exchange-rate update`

用途：app 身份新增或更新固定汇率。

命令：

```bash
contract-cli mdm fixed-exchange-rate update --profile contract --as app --input-file fixed-exchange-rate.json
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `PUT /open-apis/mdm/v1/fixed_exchange_rate`。
- 请求体直接透传。

#### `contract-cli mdm file download`

用途：app 身份下载主数据附件。

命令：

```bash
contract-cli mdm file download <file-id> --profile contract --as app --output-file ./attachment.bin
contract-cli mdm file download <file-id> --profile contract --as app --raw > attachment.bin
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/mdm/v1/file/download/{file_id}`。
- `--raw` 会把二进制内容写到 stdout，不打印额外提示。

### 7. 事件命令

#### `contract-cli event outbound-ip list`

用途：app 身份分页查询开放平台事件出口 IP。

命令：

```bash
contract-cli event outbound-ip list --profile contract --as app --page-size 10 --page-token next
```

身份规则：

- 当前仅支持 `--as app`。
- 走 `GET /open-apis/event/v1/outbound_ip`。
- `--page-size` 可选，传入时必须在 `10` 到 `50` 之间。
- 不接受 `--input-file` / `--data`。

### 8. 审批矩阵规则表命令

矩阵接口返回 HTTP 200 但 JSON 业务 `code` 非 `0` 时，CLI 保留服务端响应并返回非零退出码；`code: 0` 时返回成功。

完整的人工验收步骤见 [审批矩阵 CLI 业务场景测试清单](approval-matrix-cli-test-scenarios.md)。

这一组命令支持 `--as user` / `--as app`；user 当前用于 contract 产品，须具备合同规则管理权限，不会自动降级为 app。公共定位参数是 `--product-id`、`--group-id`，涉及单表时再传 `--table-id`。contract 产品页面只展示 `approve_matrix`：CLI 阻断该产品的 `rule group create`（含 `approval-matrix` 别名），新建矩阵时也要求 `--group-id approve_matrix`；其他产品的原有创建能力不变。`approval-matrix publish` 和 `approval-matrix table publish` 是 `rule table release` 的兼容入口。

| 命令 | 方法与路径 | 请求体 |
| --- | --- | --- |
| `rule group get` | `GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}` | 不接受 |
| `rule employee search` | `POST .../products/{product_id}/groups/{group_id}/employees/search` | 只读搜索，必填：param |
| `rule table create` | `POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables` | 必填：name；编码和策略可选 |
| `rule table get/update/delete` | `GET/PUT/DELETE .../rule_tables/{rule_table_id}` | 仅 update 必填 JSON；name 必填，编码不变 |
| `rule table column add` | `POST .../rule_tables/{rule_table_id}/table_columns` | 必填：base_table_column_id/direction |
| `rule table column update/update-condition/update-result` | `PUT .../table_columns/{table_column_id}` | 必填；条件/结果共用接口 |
| `rule table column delete` | `DELETE .../table_columns/{table_column_id}` | 不接受 |
| `rule table import get/pause/resume/verify/cancel` | 本地读取、暂停、恢复、只读核验或取消计划 | 不接受，使用 --plan-id |
| `rule table list` | `GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables` | 不接受；`--page-size` 必填，范围 1-100 |
| `rule table pre-release` | `PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/pre_release` | 可选 |
| `rule table release` | `PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/release` | 可选 |
| `rule table column-headers list` | `GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_columns/column_headers` | 不接受 |
| `rule table row create` | `POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows` | 必填 |
| `rule table row get` | `GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/{table_row_id}` | 不接受 |
| `rule table row list` | `GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows` | 不接受 |
| `rule table row search` | `POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/search` | JSON 必填；`--page-size` 必填，范围 1-100 |
| `rule table row update` | `PUT /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/{table_row_id}` | 必填 |
| `rule table row delete` | `DELETE /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/{table_row_id}` | 不接受 |
| `rule table import plan` | 读取列头和全部分页规则行后本地生成计划 | 必填，多行 `rows` 输入 |
| `rule table import apply` | 对比行快照，逐行调用 create/update 并回读核验 | 不接受；读取本地 `plan_id` |

示例：

```bash
contract-cli rule table list --profile contract --as app --product-id <product-id> --group-id <group-id> --page-size 10
contract-cli rule employee search --profile contract --as user --product-id contract --group-id approve_matrix --input-file employee-search.json
contract-cli rule table row create --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --input-file row.json
contract-cli rule table row delete <row-id> --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>
contract-cli rule table import plan --profile contract --as app --product-id contract --group-id approve_matrix --table-id <table-id> --input-file import.json
contract-cli rule table import apply --profile contract --as app --plan-id <plan-id>
```

批量导入说明：

- Agent 接收业务 Excel、表格或字段资料时，先识别矩阵用于审批路由、协商自动邀请还是其他场景；用途不明确先询问，明确前不创建矩阵、不改列、不生成或执行导入计划。业务资料只是行数据来源，不视为完整结构定义。协商自动邀请必须先确认 `合同类型` 条件列存在且为 `value_type=COLLECTION`；缺失或类型不符时，结构变更单独确认并在变更后重读列头。

- 矩阵和列 ID 通过 `rule table get` 或 `rule table column-headers list` 获取。若响应中包含既有 `value_id/value_code/value_name/value_type`，更新时原样复用；新增或未绑定列省略 `value_*`，不根据列名推测业务元素 ID。

- 人员姓名先通过 `rule employee search` 解析；请求 `{"param":"赵少帅","group_code":"approve_matrix"}`，可选 `group_code` 必须与 `--group-id` 一致，省略或 null 兼容旧请求；返回候选 `employee_id/name/email/department_name/status/selectable`。仅采用 `selectable=true`，重名先确认。ID 是十进制字符串形式的矩阵外部人员 ID，固定 `user_id_type=user_id`；不直接复用页面内部编号。详见 [人员搜索参数](../skills/contract-cli-rule/references/employee-search-parameters.md)。

- 新增结构命令字段见 [结构接口参数](../skills/contract-cli-rule/references/structure-parameters.md)。`approval-matrix` 是对应 `rule` 命令的兼容入口，支持 user/app。
- 规则表总列数上限为 12，优先级列和备注列固定占 2 列，条件列与结果列合计最多 10 列。规划结构时先校验 `目标条件列数 + 目标结果列数 + 2 <= 12`；`rule table column add` 会先查询当前列头，在当前总数达到 12 时本地阻断，不发送新增写请求。服务端错误码 20303 仍是并发变化场景的最终保护。
- 人员批量值使用正 int64 外部 ID（数字或十进制字符串输入，发送为数字）；部门规则行与 import plan 统一使用 `open_department_id` 字符串（形如 `od-...`）并原样发送。`department search` 的可选候选直接返回该字段；兼容的数字 `department_id` 仅作为 `department batch-get` 查询键，用于回查 `open_department_id`，不可直接写入规则行。NUMBER 支持整数 30 位、小数 8 位，导入显式 null 清空单元格，省略字段保持不变。
- `apply --rows 1,3 --batch-size 10` 支持局部确认和分批执行；get 查询本地进度，pause 在当前行结束后停批，resume 清除暂停标记，verify 只读重试回读，cancel 取消后续操作。列头或全量行快照变化、24 小时过期返回 invalidated，结果不确定立即停止后续行。
- `import plan` 的输入顶层为 `rows`；每行包含可选 `operation`、更新所需的 `row_id`，以及以列名或列 ID 为键的 `cells`。未传 `--table-id` 时返回已有矩阵候选项和 `status=needs_input`。
- `plan` 查询列头和全部规则行，在本地做映射、目标行和 2000 行容量检查，不写矩阵。校验通过返回 `status=needs_confirmation` 与 `plan_id`；校验失败返回 `status=needs_input`。
- `import apply` 将 `plan_id` 作为确认令牌，串行逐行写入并回读；回读吻合才记成功，失败时进入 `needs_verification` 并停批，再次执行同一计划时跳过已验证成功行。`partial_success`、`failed` 和 `invalidated` 会在保留结构化结果后返回非零退出码。
- 矩阵写请求的编辑人由开平服务端从当前 Bearer 用户身份解析。使用 `--as user` 时，CLI 发送 `X-Qfei-Identity: user` 并移除 `--user-id` 对应的 query，避免把审批人 ID 或其他人员 ID 作为编辑人；`--as app` 记录应用/系统身份。
- 写入结果不确定先用 `row get/list/search` 核对，停止后续写入，不自动重试。
- 两个批量命令固定返回结构化状态，不支持 `--raw`。
- 完整输入契约和兼容性设计见 [审批矩阵 CLI 集成技术方案](approval-matrix-cli-technical-design.md)。

2026-09-14 配套命令（均支持 user/app）：

| 命令 | 用途 |
| --- | --- |
| `rule employee batch-get` | 外部人员 ID 回读 |
| `rule department search` / `rule department batch-get` | 部门搜索与回读 |
| `rule role search` / `rule role batch-get` | 角色搜索与回读 |
| `rule symbol query` / `rule loop-function query` | CLI 内置的当前类型符号、后端循环函数 |
| `rule table column preview` / `rule table column patch` | 变更预检和局部更新 |

方法、路径和请求示例见 [配套查询及版本保护](../skills/contract-cli-rule/references/configuration-completion.md)。无需新增 SQL；行写入仍调用原 CRUD。旧 guarded 计划已停用，应先核验已成功行再重新计划。条件类型与运算符由 BPM 定义，`rule symbol query` 使用 CLI 内置映射并提交其中的真实 symbol；该查询不调用 `/symbols/query`，无需 `product/group/profile` 或额外业务枚举数据源。

## 后续扩展 app 接口时的建议落点

- 新增 app 业务接口时，优先直接沉淀成结构化命令，避免把预留的 `api call` 暴露给最终用户
- 如需临时验证开放平台路径和鉴权，建议在本地测试或开发工具里完成，不把验证入口写入公开文档
- 一旦新增结构化 app 命令，先更新本文档的“命令矩阵”和“身份规则”，再补实现与测试
- 如果未来同一命令同时支持 user 和 app，需要在文档里明确写出路径差异、参数差异和默认身份规则
