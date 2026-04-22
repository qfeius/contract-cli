# contract-cli 测试文档

本文档面向 QA、开发自测和发布前验收，覆盖 `contract-cli` 当前已支持能力的端到端测试流程。

## 1. 测试目标

- 验证 CLI 可以通过 npm/npx 或源码方式安装并启动。
- 验证 profile 初始化、user OAuth、bot token、登出、默认身份切换等鉴权流程。
- 验证 bot 身份下当前已接入的全部结构化业务命令。
- 验证 user 身份下当前已接入的全部结构化业务命令。
- 验证 `api call`、输出格式、通用 Agent skills 安装、CLI 内置 skills 兜底安装、异常拦截等补充能力。

## 2. 测试环境准备

### 2.1 基础依赖

执行测试前确认本机具备：

```bash
node --version
npm --version
go version
tar --version || true
```

最低要求：

```text
Node.js >= 16
Go >= 1.24.3
macOS/Linux 使用 tar，Windows 使用 PowerShell Expand-Archive
```

### 2.2 推荐隔离配置目录

为避免污染本机已有登录态，建议每轮测试使用独立配置目录：

```bash
export CONTRACT_CLI_CONFIG_DIR="$(mktemp -d)"
export PROFILE="contract-group"
```

后续命令都显式带：

```bash
--profile "$PROFILE"
```

测试结束后可清理：

```bash
rm -rf "$CONTRACT_CLI_CONFIG_DIR"
```

### 2.3 测试数据清单

业务命令需要准备一组 dev 环境可访问的数据：

| 变量 | 含义 | 示例 |
| --- | --- | --- |
| `CONTRACT_ID` | 已存在合同 ID | `7023...` |
| `TEMPLATE_ID` | 已存在模板 ID | `TMP...` |
| `VENDOR_ID` | 已存在交易方 ID | `1063...` |
| `LEGAL_ENTITY_ID` | 已存在法人主体 ID | `7023...` |
| `BOT_APP_ID` | bot appId | 不写入文档 |
| `BOT_APP_SECRET` | bot appSecret | 不写入文档 |
| `USER_ID` | bot 接口可选透传 user_id | `ou_xxx` |
| `USER_ID_TYPE` | bot 接口可选透传 user_id_type | `user_id` / `employee_id` |

建议在 shell 中设置：

```bash
export CONTRACT_ID="<contract-id>"
export TEMPLATE_ID="<template-id>"
export VENDOR_ID="<vendor-id>"
export LEGAL_ENTITY_ID="<legal-entity-id>"
export USER_ID="<user-id>"
export USER_ID_TYPE="user_id"
```

bot 凭证建议用环境变量传入，不要写入命令历史：

```bash
export CONTRACT_CLI_BOT_APP_ID="<app-id>"
export CONTRACT_CLI_BOT_APP_SECRET="<app-secret>"
```

## 3. CLI 安装流程测试

### 3.1 npm 安装 beta 包

用途：验证真实 npm 分发、GitHub Release 二进制下载、`postinstall`、bin wrapper 全链路。

```bash
npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org
npx skills add qfeius/contract-cli -y -g
contract-cli --version
contract-cli skills list
```

预期结果：

```text
contract-cli version 0.1.0-beta.1
Built-in skills:
```

检查点：

| 检查项 | 预期 |
| --- | --- |
| npm install | 成功，无 404、无 postinstall 失败 |
| `npx skills add qfeius/contract-cli -y -g` | 成功安装 7 个 skills，并输出对应 Agent 平台适配信息 |
| `contract-cli --version` | 输出版本号、commit、build date |
| `contract-cli skills list` | 输出 `auth`、`contract-cli-contract`、`contract-cli-mdm-vendor` 等内置 skill |

### 3.2 npx 临时运行

用途：验证不全局安装时也可以拉起 CLI。

```bash
npx @qfeius/contract-cli@beta --version
npx @qfeius/contract-cli@beta skills list
```

预期结果：

```text
contract-cli version 0.1.0-beta.1
```

### 3.3 源码构建安装

用途：验证源码用户或开发者可直接编译。

```bash
git clone https://github.com/qfeius/contract-cli.git
cd contract-cli
make test
make build
./contract-cli --version
```

可选安装到本机 PATH：

```bash
make install
contract-cli --version
```

### 3.4 release assets 本地生成

用途：验证 GitHub Release 附件生成脚本与 npm installer 的文件名规则一致。

```bash
make release-assets
find dist/release-assets -maxdepth 1 -type f -print | sort
```

预期文件：

```text
checksums.txt
contract-cli-<version>-darwin-amd64.tar.gz
contract-cli-<version>-darwin-arm64.tar.gz
contract-cli-<version>-linux-amd64.tar.gz
contract-cli-<version>-linux-arm64.tar.gz
contract-cli-<version>-windows-amd64.zip
contract-cli-<version>-windows-arm64.zip
```

### 3.5 Agent skills 安装测试

用途：验证推荐的通用 `skills` installer 可以从 GitHub 仓库安装 Agent skills，并覆盖 Codex、Cursor、Trae、Claude Code 等多类环境。

推荐安装：

```bash
npx skills add qfeius/contract-cli -y -g
```

预期结果：

- 输出 `Installation complete`。
- 输出 `Installed 7 skills`。
- 至少包含 `auth`、`contract-cli-api-call`、`contract-cli-contract`、`contract-cli-mdm-fields`、`contract-cli-mdm-legal`、`contract-cli-mdm-vendor`、`contract-cli-shared`。
- 输出中能看到 `universal` 或 `symlinked` 的平台适配信息；具体平台列表以 installer 实际输出为准。

注意事项：

- 该命令读取远程 `https://github.com/qfeius/contract-cli` 仓库，必须确认本轮 skill 改动已经 push。
- 测试机需要能访问 npm 和 GitHub。

### 3.6 CLI 内置 skills 兜底安装测试

用途：验证通用 installer 不可用时，CLI 随包分发的 skill 仍能安装到指定目录。

```bash
SKILLS_TARGET="$(mktemp -d)"
contract-cli skills install --target "$SKILLS_TARGET"
find "$SKILLS_TARGET" -maxdepth 2 -type f | sort
```

预期结果：

```text
Installed skill: auth
Installed skill: contract-cli-contract
Installed skill: contract-cli-mdm-vendor
Installed skill: contract-cli-mdm-legal
Installed skill: contract-cli-mdm-fields
Installed skill: contract-cli-api-call
Installed skill: contract-cli-shared
```

覆盖已有目录：

```bash
contract-cli skills install --target "$SKILLS_TARGET" --force
```

### 3.7 版本检查与升级提示测试

用途：验证 CLI 可以发现 npm 远端新版本，并且自动提示不会频繁打扰用户。

手动检查 beta 渠道：

```bash
contract-cli update check --channel beta
```

预期结果：

| 场景 | 预期 |
| --- | --- |
| 远端 beta 比本地新 | 输出 `A new contract-cli version is available` 和 `npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org` |
| 远端 beta 与本地一致 | 输出 `contract-cli is up to date` |
| 本地是源码 `dev` 或 git hash 构建 | 输出 `Update check skipped` |

自动提示检查：

```bash
contract-cli skills list
contract-cli skills list
```

预期结果：

- 在交互终端下，第一次普通命令最多触发一次远端版本检查。
- 第二次普通命令如果距离上次检查未超过 30 分钟，不应再次访问远端，也不应重复提示。
- 网络失败时原命令仍然继续执行，不应因为版本检查失败而退出；失败检查也应按 30 分钟缓存，避免每条命令都重试。

关闭自动检查：

```bash
CONTRACT_CLI_NO_UPDATE_CHECK=1 contract-cli skills list
```

预期结果：

- 不发起自动版本检查。
- `skills list` 本身正常输出。

## 4. 配置与授权流程测试

### 4.1 初始化 profile

```bash
contract-cli config add --env dev --name "$PROFILE"
```

预期结果：

- 写入 dev 环境配置。
- 写入开放平台基址 `https://dev-open.qtech.cn`。
- 写入 user OAuth 配置。
- 写入 bot token endpoint。
- 当前 profile 被设置为 `$PROFILE`。

### 4.2 user 登录

```bash
contract-cli auth login --profile "$PROFILE" --as user --timeout 5m
```

如测试机无法自动打开浏览器：

```bash
contract-cli auth login --profile "$PROFILE" --as user --timeout 5m --no-open-browser
```

预期结果：

- 浏览器完成 OAuth 授权。
- CLI 收到授权回调并保存 user token。
- 默认身份切换为 `user`。

验证状态：

```bash
contract-cli auth status --profile "$PROFILE" --as user
```

预期包含：

```text
Identity: user
Authorization: authorized
```

### 4.3 bot 登录

推荐使用环境变量：

```bash
contract-cli auth login --profile "$PROFILE" --as bot
```

也可以显式传参：

```bash
contract-cli auth login --profile "$PROFILE" --as bot --app-id "$CONTRACT_CLI_BOT_APP_ID" --app-secret "$CONTRACT_CLI_BOT_APP_SECRET"
```

预期结果：

- CLI 调用 `https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal`。
- 请求体使用 `appId/appSecret`。
- 成功后保存 bot token。
- 默认身份切换为 `bot`。
- 输出 token 过期时间。

验证状态：

```bash
contract-cli auth status --profile "$PROFILE" --as bot
```

预期包含：

```text
Identity: bot
Authorization: authorized
Token Endpoint: https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal
Token Protocol: tenant_access_token/internal
Expires At:
```

### 4.4 默认身份切换

切换到 user：

```bash
contract-cli auth use --profile "$PROFILE" --as user
contract-cli auth status --profile "$PROFILE"
```

切换到 bot：

```bash
contract-cli auth use --profile "$PROFILE" --as bot
contract-cli auth status --profile "$PROFILE"
```

预期结果：

- `auth use --as user` 后不显式传 `--as` 的业务命令默认走 user。
- `auth use --as bot` 后不显式传 `--as` 的业务命令默认走 bot。

### 4.5 user 登出

```bash
contract-cli auth logout --profile "$PROFILE" --as user
contract-cli auth status --profile "$PROFILE" --as user
```

预期结果：

```text
Authorization: unauthorized
```

回归点：

- user logout 不应删除 bot token。
- user logout 不应删除 bot appId/appSecret。

### 4.6 bot 登出

```bash
contract-cli auth logout --profile "$PROFILE" --as bot
contract-cli auth status --profile "$PROFILE" --as bot
```

预期结果：

```text
Authorization: configured
```

回归点：

- 只清空 bot token。
- 保留 `appId/appSecret` 和 secret 引用。
- 如果默认身份原本是 bot，登出后默认身份仍保持 bot。
- 再次执行 `auth login --as bot` 时，可以复用已保存凭证重新换 token。

### 4.7 授权异常场景

| 场景 | 命令 | 预期 |
| --- | --- | --- |
| 旧 profile 缺少 bot endpoint | `auth login --as bot` | 报错并提示重跑 `config add --env dev --name <profile>` |
| bot appSecret 错误 | `auth login --as bot` | 保留新凭证，bot token 为空，默认身份不切到 bot |
| user 未登录调用 user-only 命令 | `contract enum list --as user` | 报未授权或 token 不可用 |
| bot token 已清空后调用 bot 命令 | `contract get --as bot` | 报未授权或 token 不可用 |

## 5. bot 身份业务命令测试

执行本节前确保：

```bash
contract-cli auth login --profile "$PROFILE" --as bot
contract-cli auth status --profile "$PROFILE" --as bot
```

公共检查点：

| 检查项 | 预期 |
| --- | --- |
| 身份 | 显式 `--as bot` 或默认身份为 bot |
| 路径 | 不走 `/open-apis/contract/v1/mcp/...`，除非命令特殊说明 |
| 输出 | 默认 JSON，可使用 `--output json` 显式校验 |
| 通用 query | `--user-id-type` 不传默认 `user_id`，显式传值则覆盖；`--user-id` 传了就透传，不传就不带 |
| token | 使用 tenant access token |

### 5.1 合同搜索

```bash
cat > /tmp/contract-search.json <<'JSON'
{
  "page_size": 10
}
JSON

contract-cli contract search --profile "$PROFILE" --as bot --input-file /tmp/contract-search.json --output json
contract-cli contract search --profile "$PROFILE" --as bot --input-file /tmp/contract-search.json --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/contracts/search
```

预期响应：

- HTTP 成功。
- JSON 中包含后端返回的搜索数据。
- 如果有数据，通常可观察到 `data.items`、`data.page_token`、`data.has_more` 等结构。

### 5.2 合同详情

```bash
contract-cli contract get "$CONTRACT_ID" --profile "$PROFILE" --as bot --output json
contract-cli contract get "$CONTRACT_ID" --profile "$PROFILE" --as bot --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/contracts/{contract_id}
```

### 5.3 同步用户分组

```bash
contract-cli contract sync-user-groups --profile "$PROFILE" --as bot --output json
contract-cli contract sync-user-groups --profile "$PROFILE" --as bot --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/contracts/user-groups/sync
```

### 5.4 获取合同文本

```bash
contract-cli contract text "$CONTRACT_ID" --profile "$PROFILE" --as bot --output json
contract-cli contract text "$CONTRACT_ID" --profile "$PROFILE" --as bot --full-text --output json
contract-cli contract text "$CONTRACT_ID" --profile "$PROFILE" --as bot --offset 0 --limit 1000 --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/contracts/{contract_id}/text
```

### 5.5 创建合同

创建合同是写操作，建议优先在 dev 环境使用明确可回收的测试数据。

```bash
cat > /tmp/contract-create-bot.json <<'JSON'
{
  "contract_name": "contract-cli bot create smoke",
  "create_user_id": "REPLACE_WITH_USER_ID"
}
JSON
```

将 `REPLACE_WITH_USER_ID` 替换为真实创建人，再执行：

```bash
contract-cli contract create --profile "$PROFILE" --as bot --input-file /tmp/contract-create-bot.json --output json
```

也可直接传 JSON：

```bash
contract-cli contract create --profile "$PROFILE" --as bot --data '{"contract_name":"contract-cli bot create smoke","create_user_id":"'"$USER_ID"'"}' --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/contracts
```

检查点：

- 请求体需要由测试者自行提供 `create_user_id`。
- CLI 不替调用方补 `create_user_id`。
- 如果后端提示字段缺失，应根据合同创建字段文档补齐必填字段。

字段参考：

```text
skills/contract-cli-contract/references/create-contract-fields.md
skills/contract-cli-contract/references/create-contract-field-tree.md
skills/contract-cli-contract/references/create-contract-enums.md
```

### 5.6 合同分类列表

```bash
contract-cli contract category list --profile "$PROFILE" --as bot --output json
contract-cli contract category list --profile "$PROFILE" --as bot --lang zh-CN --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/contract_categorys
```

### 5.7 模板列表

```bash
contract-cli contract template list --profile "$PROFILE" --as bot --page-size 10 --output json
contract-cli contract template list --profile "$PROFILE" --as bot --category-number "<category-number>" --page-size 10 --page-token "<page-token>" --output json
contract-cli contract template list --profile "$PROFILE" --as bot --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/templates
```

### 5.8 模板详情

```bash
contract-cli contract template get "$TEMPLATE_ID" --profile "$PROFILE" --as bot --output json
contract-cli contract template get "$TEMPLATE_ID" --profile "$PROFILE" --as bot --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/templates/{template_id}
```

### 5.9 创建模板实例

```bash
cat > /tmp/template-instance-bot.json <<'JSON'
{
  "template_number": "REPLACE_WITH_TEMPLATE_NUMBER",
  "create_user_id": "REPLACE_WITH_USER_ID"
}
JSON
```

将占位符替换后执行：

```bash
contract-cli contract template instantiate --profile "$PROFILE" --as bot --input-file /tmp/template-instance-bot.json --output json
contract-cli contract template instantiate --profile "$PROFILE" --as bot --input-file /tmp/template-instance-bot.json --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/template_instances
```

检查点：

- 请求体需要由测试者自行提供 `create_user_id`。
- CLI 只透传 JSON，不补字段。

### 5.10 上传合同相关文件

准备一个小于等于 `200MB` 的测试文件：

```bash
printf '%s\n' '%PDF-1.4 contract-cli upload smoke' > /tmp/contract-upload.pdf
```

执行：

```bash
contract-cli contract upload-file --profile "$PROFILE" --as bot --file /tmp/contract-upload.pdf --file-type attachment --file-name contract-upload.pdf --output json
contract-cli contract upload-file --profile "$PROFILE" --as bot --file /tmp/contract-upload.pdf --file-type attachment --raw
```

预期底层接口：

```text
POST /open-apis/contract/v1/files/upload
```

检查点：

- 请求是 `multipart/form-data`。
- 表单字段包含 `file_name`、`file_type`、`file`。
- 响应 JSON 中应包含后端返回的 `data.file_id`。
- profile 默认身份是 user 或显式 `--as user` 时，CLI 应在发 HTTP 前报错。
- 超过 `200MB`、目录路径、文件不存在、缺少 `--file`、缺少 `--file-type` 都应报明确错误。

### 5.11 交易方列表

```bash
contract-cli mdm vendor list --profile "$PROFILE" --as bot --page-size 10 --output json
contract-cli mdm vendor list --profile "$PROFILE" --as bot --name "供应商" --page-size 10 --output json
contract-cli mdm vendor list --profile "$PROFILE" --as bot --name "供应商" --page-token "<page-token>" --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/vendors
```

### 5.12 交易方详情

```bash
contract-cli mdm vendor get "$VENDOR_ID" --profile "$PROFILE" --as bot --output json
contract-cli mdm vendor get "$VENDOR_ID" --profile "$PROFILE" --as bot --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/vendors/{vendor_id}
```

### 5.13 法人主体列表

```bash
contract-cli mdm legal list --profile "$PROFILE" --as bot --page-size 10 --output json
contract-cli mdm legal list --profile "$PROFILE" --as bot --name "主体A" --page-size 10 --output json
contract-cli mdm legal list --profile "$PROFILE" --as bot --name "主体A" --page-token "<page-token>" --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/legal_entities/list_all
```

### 5.14 法人主体详情

```bash
contract-cli mdm legal get "$LEGAL_ENTITY_ID" --profile "$PROFILE" --as bot --output json
contract-cli mdm legal get "$LEGAL_ENTITY_ID" --profile "$PROFILE" --as bot --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/legal_entities/{legal_entity_id}
```

检查点：

- bot 路由会同时带 path 参数和同名 query `legal_entity_id`。

### 5.15 字段配置

```bash
contract-cli mdm fields list --profile "$PROFILE" --as bot --biz-line vendor --output json
contract-cli mdm fields list --profile "$PROFILE" --as bot --biz-line legal_entity --output json
contract-cli mdm fields list --profile "$PROFILE" --as bot --biz-line vendor_risk --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/config/config_list
```

### 5.16 bot 不支持命令的负向验证

```bash
contract-cli contract enum list --profile "$PROFILE" --as bot --type contract_status
```

预期结果：

- 直接报错，不发 HTTP。
- 错误语义应说明该请求需要 user 身份或 user-only。

## 6. user 身份业务命令测试

执行本节前确保：

```bash
contract-cli auth login --profile "$PROFILE" --as user --timeout 5m
contract-cli auth status --profile "$PROFILE" --as user
```

公共检查点：

| 检查项 | 预期 |
| --- | --- |
| 身份 | 显式 `--as user` 或默认身份为 user |
| 路径 | 结构化 user 命令大多走 `/open-apis/contract/v1/mcp/...` |
| 固定 query | MCP user-only 固定 query 不被 `--user-id-type` 覆盖 |
| 输出 | 默认 JSON，可使用 `--output json` 显式校验 |

### 6.1 合同搜索

```bash
cat > /tmp/contract-search-user.json <<'JSON'
{
  "page_size": 10
}
JSON

contract-cli contract search --profile "$PROFILE" --as user --input-file /tmp/contract-search-user.json --output json
contract-cli contract search --profile "$PROFILE" --as user --contract-number "<contract-number>" --page-size 10 --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/mcp/contracts/search
```

回归点：

- MCP 固定 query `user_id_type=user_id` 应保留。
- 即使显式传 `--user-id-type employee_id`，MCP 固定参数也不应被覆盖。
- `--user-id` 这种 MCP 固定 query 不存在的通用参数可以被补充。

建议额外执行：

```bash
contract-cli contract search --profile "$PROFILE" --as user --input-file /tmp/contract-search-user.json --user-id "$USER_ID" --user-id-type employee_id --output json
```

### 6.2 合同详情

```bash
contract-cli contract get "$CONTRACT_ID" --profile "$PROFILE" --as user --output json
contract-cli contract get "$CONTRACT_ID" --profile "$PROFILE" --as user --user-id "$USER_ID" --user-id-type employee_id --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/contracts/{contract_id}
```

### 6.3 同步用户分组

```bash
contract-cli contract sync-user-groups --profile "$PROFILE" --as user --output json
contract-cli contract sync-user-groups --profile "$PROFILE" --as user --user-id "$USER_ID" --user-id-type employee_id --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/mcp/contracts/user-groups/sync
```

回归点：

- 固定 query `user_id_type=user_id` 不应被通用 `--user-id-type` 覆盖。

### 6.4 获取合同文本

```bash
contract-cli contract text "$CONTRACT_ID" --profile "$PROFILE" --as user --output json
contract-cli contract text "$CONTRACT_ID" --profile "$PROFILE" --as user --full-text --output json
contract-cli contract text "$CONTRACT_ID" --profile "$PROFILE" --as user --offset 0 --limit 1000 --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/contracts/{contract_id}/text
```

### 6.5 创建合同

创建合同是写操作，建议在 dev 环境使用可回收测试数据。

```bash
cat > /tmp/contract-create-user.json <<'JSON'
{
  "contract_name": "contract-cli user create smoke"
}
JSON

contract-cli contract create --profile "$PROFILE" --as user --input-file /tmp/contract-create-user.json --output json
```

也可以直接传 JSON：

```bash
contract-cli contract create --profile "$PROFILE" --as user --data '{"contract_name":"contract-cli user create smoke"}' --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/mcp/contracts
```

### 6.6 合同分类列表

```bash
contract-cli contract category list --profile "$PROFILE" --as user --output json
contract-cli contract category list --profile "$PROFILE" --as user --lang zh-CN --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/contract_categorys
```

### 6.7 模板列表

```bash
contract-cli contract template list --profile "$PROFILE" --as user --page-size 10 --output json
contract-cli contract template list --profile "$PROFILE" --as user --category-number "<category-number>" --page-size 10 --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/templates
```

### 6.8 模板详情

```bash
contract-cli contract template get "$TEMPLATE_ID" --profile "$PROFILE" --as user --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/templates/{template_id}
```

### 6.9 创建模板实例

```bash
cat > /tmp/template-instance-user.json <<'JSON'
{
  "template_number": "REPLACE_WITH_TEMPLATE_NUMBER"
}
JSON

contract-cli contract template instantiate --profile "$PROFILE" --as user --input-file /tmp/template-instance-user.json --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/mcp/template_instances
```

### 6.10 枚举值列表

```bash
contract-cli contract enum list --profile "$PROFILE" --as user --type contract_status --output json
contract-cli contract enum list --profile "$PROFILE" --as user --type business_type --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/enum_values
```

说明：

- 当前 `contract enum list` 是 user-only。
- bot 身份调用应失败。

### 6.11 交易方列表

```bash
contract-cli mdm vendor list --profile "$PROFILE" --as user --page-size 10 --output json
contract-cli mdm vendor list --profile "$PROFILE" --as user --name "供应商" --page-size 10 --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/vendors
```

### 6.12 交易方详情

```bash
contract-cli mdm vendor get "$VENDOR_ID" --profile "$PROFILE" --as user --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/vendors/{vendor_id}
```

### 6.13 法人主体列表

```bash
contract-cli mdm legal list --profile "$PROFILE" --as user --page-size 10 --output json
contract-cli mdm legal list --profile "$PROFILE" --as user --name "主体A" --page-size 10 --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/legal_entities
```

### 6.14 法人主体详情

```bash
contract-cli mdm legal get "$LEGAL_ENTITY_ID" --profile "$PROFILE" --as user --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}
```

### 6.15 字段配置

```bash
contract-cli mdm fields list --profile "$PROFILE" --as user --biz-line vendor --output json
contract-cli mdm fields list --profile "$PROFILE" --as user --biz-line legal_entity --output json
contract-cli mdm fields list --profile "$PROFILE" --as user --biz-line vendor_risk --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/config/config_list
```

## 7. api call 兜底测试

### 7.1 user MCP 路径

```bash
contract-cli api call GET /open-apis/contract/v1/mcp/config/config_list --profile "$PROFILE" --as user --output json --raw
```

预期结果：

- user 身份成功。
- 返回原始响应 envelope。

### 7.2 bot 调用非 MCP 路径

```bash
contract-cli api call GET /open-apis/mdm/v1/config/config_list --profile "$PROFILE" --as bot --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json --raw
```

预期结果：

- bot 身份成功。
- `--user-id` 与 `--user-id-type` 被拼到 query；不传 `--user-id-type` 时应默认拼接 `user_id_type=user_id`。

### 7.3 bot 禁止调用 MCP 路径

```bash
contract-cli api call GET /open-apis/contract/v1/mcp/config/config_list --profile "$PROFILE" --as bot
```

预期结果：

- CLI 本地直接报错。
- 不应发出 HTTP 请求。

### 7.4 非法路径

```bash
contract-cli api call GET https://dev-open.qtech.cn/open-apis/mdm/v1/config/config_list --profile "$PROFILE" --as bot
```

预期结果：

- 报错：开放平台路径必须是相对 `/open-apis/...` 路径。

## 8. 输出格式与参数解析测试

### 8.1 JSON 输出

```bash
contract-cli mdm fields list --profile "$PROFILE" --as bot --biz-line vendor --output json
```

预期结果：

- 输出合法 JSON。

### 8.2 YAML 输出

```bash
contract-cli mdm fields list --profile "$PROFILE" --as bot --biz-line vendor --output yaml
```

预期结果：

- 输出合法 YAML。

### 8.3 raw 输出

```bash
contract-cli mdm fields list --profile "$PROFILE" --as bot --biz-line vendor --output json --raw
```

预期结果：

- 输出后端原始 envelope。

### 8.4 `--input-file` 与 `--data` 互斥

```bash
contract-cli contract create --profile "$PROFILE" --as bot --input-file /tmp/contract-create-bot.json --data '{}'
```

预期结果：

- CLI 报错。
- 不发 HTTP 请求。

### 8.5 `--file` 仅用于真实文件上传

```bash
contract-cli contract create --profile "$PROFILE" --as bot --file /tmp/contract-create-bot.json
```

预期结果：

- CLI 报未知参数或用法错误，因为 `contract create` 只接受 `--input-file` / `--data` 作为 JSON 请求体。
- `--file` 只在 `contract upload-file` 中表示真实二进制文件上传。

## 9. 发布与安装回归测试

### 9.1 本地发布检查

```bash
make release-check
```

覆盖内容：

- `go test ./...`
- CLI smoke。
- `npm pack --dry-run`。
- 本地 tgz 安装。
- postinstall 下载/解压模拟。
- `skills list` 和 `skills install`。

### 9.2 npm beta 包安装检查

```bash
tmpdir="$(mktemp -d)"
cache_dir="$(mktemp -d)"
npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org --prefix "$tmpdir/prefix" --cache "$cache_dir" --prefer-online
"$tmpdir/prefix/bin/contract-cli" --version
"$tmpdir/prefix/bin/contract-cli" skills list
rm -rf "$tmpdir" "$cache_dir"
```

预期结果：

```text
contract-cli version 0.1.0-beta.1
Built-in skills:
```

## 10. 回归测试建议

### 10.1 每次发版前必须执行

```bash
make test
make release-check
npm publish --dry-run --tag beta
```

### 10.2 每次改鉴权逻辑后必须覆盖

- `auth login --as user`
- `auth login --as bot`
- `auth status --as user`
- `auth status --as bot`
- `auth logout --as user`
- `auth logout --as bot`
- `auth use --as user`
- `auth use --as bot`
- user logout 不影响 bot。
- bot logout 保留 app 凭证。
- bot token 过期状态显示 `expired`。

### 10.3 每次改 openplatform client 后必须覆盖

- `IdentityPolicyUserOnly` 下 MCP 固定 query 不被通用 query 覆盖。
- `IdentityPolicyAny` 下 bot/open API 仍允许通用 query 覆盖同名参数。
- 非 2xx 错误摘要不泄露 token。
- 响应 raw body 截断逻辑正常。
- 相对路径校验正常。
- Authorization header 正常注入。

### 10.4 每次改命令参数后必须覆盖

- `--input-file` 与 `--data` 互斥。
- `--user-id-type` 默认 `user_id` 且可被显式传值覆盖；`--user-id` 透传到 query。
- `--output json|yaml|table` 可用。
- `--raw` 可用。
- 未知参数报错。
- 位置参数缺失时报 usage。

## 11. 常见问题排查

| 问题 | 排查方向 |
| --- | --- |
| npm 安装报 GitHub Release 404 | 确认 GitHub Release tag 是 `v<package.version>`，附件名是 `contract-cli-<version>-<os>-<arch>.<ext>` |
| npm 首次发布后短暂 404 | npm registry/CDN 可能有缓存，使用新 cache 或等待后重试 |
| bot 登录失败 | 检查 `CONTRACT_CLI_BOT_APP_ID`、`CONTRACT_CLI_BOT_APP_SECRET`、`bot_token_endpoint` |
| user 登录卡住 | 检查浏览器 OAuth 回调、`--timeout`、是否需要 `--no-open-browser` 手动打开 URL |
| MCP 命令 bot 调用失败 | 预期行为，`contract/v1/mcp` 默认 user-only |
| 创建合同字段缺失 | 对照 `skills/contract-cli-contract/references/create-contract-fields.md` 补齐场景必填字段 |
| IDE 或命令爆红 | 先确认 `go.mod` 已 reload，`go test ./...` 是否通过 |

## 12. 测试验收标准

一轮完整验收应满足：

- CLI 可以通过 npm beta 包安装并执行 `contract-cli --version`。
- `contract-cli skills list` 和 `contract-cli skills install` 成功。
- `config add --env dev` 成功。
- user 登录、状态、切换、登出成功。
- bot 登录、状态、切换、登出成功，且 bot logout 保留凭证。
- bot 身份下第 5 节结构化业务命令和 `contract upload-file` 完成正向验证，写操作至少在 dev 环境完成一次可回收数据验证。
- user 身份下第 6 节十五条结构化业务命令完成正向验证。
- `api call` 的 user MCP、bot open API、bot MCP 拦截、非法路径四类场景完成验证。
- `make release-check` 通过。

## 13. 版本升级专项测试

本模块覆盖今天新增的版本管理与升级提示能力。它和安装测试分开验收，避免把“包能安装”和“客户端能发现远端新版本”混在一起。

### 13.1 手动版本检查

```bash
contract-cli update check
contract-cli update check --channel beta
contract-cli update check --channel latest
```

预期结果：

- 当前版本是预发布版本时，不传 `--channel` 默认检查 npm `beta` dist-tag。
- 当前版本是稳定版本时，不传 `--channel` 默认检查 npm `latest` dist-tag。
- 远端版本更新时，输出 `A new contract-cli version is available`。
- 输出升级命令：`npm install -g @qfeius/contract-cli@<channel> --registry https://registry.npmjs.org`。
- 远端版本未更新时，输出 `contract-cli is up to date`。
- 本地是 `dev`、`unknown` 或非语义化版本时，输出 `Update check skipped`。

### 13.2 自动升级提示

在交互终端下执行任意普通命令：

```bash
contract-cli skills list
contract-cli auth status --profile "$PROFILE"
```

预期结果：

- 普通命令执行前最多触发一次自动版本检查。
- 自动检查间隔为 `30` 分钟，同版本同 channel 在缓存有效期内不重复请求 npm registry。
- 检查失败不阻断原命令；失败结果也会缓存，避免每条命令都重试。
- `contract-cli version`、`contract-cli update check` 自身不触发自动检查。

关闭自动检查：

```bash
CONTRACT_CLI_NO_UPDATE_CHECK=1 contract-cli skills list
```

预期结果：

- 不访问 npm registry。
- 原命令正常输出。

### 13.3 发包后升级链路验收

```bash
npm view @qfeius/contract-cli dist-tags --registry https://registry.npmjs.org
npm install -g @qfeius/contract-cli@beta --registry https://registry.npmjs.org
contract-cli --version
contract-cli update check --channel beta
```

检查点：

- npm `beta` dist-tag 指向预期版本。
- GitHub Release 中存在对应版本的多平台二进制附件。
- 已安装版本落后于远端 beta 时，CLI 能提示升级。
- 已安装版本等于远端 beta 时，CLI 显示已是最新。

## 14. Agent skills 单独安装专项测试

本模块覆盖今天确认的独立 skill 安装方式。推荐优先使用通用 installer，从 GitHub 仓库安装 `skills/`，适配 Codex、Cursor、Trae、Claude Code 等多类 Agent 环境。

### 14.1 通用 installer 安装

```bash
npx skills add qfeius/contract-cli -y -g
```

预期结果：

- 输出 `Installation complete`。
- 输出 `Installed 7 skills`。
- 安装内容至少包含：
  - `auth`
  - `contract-cli-api-call`
  - `contract-cli-contract`
  - `contract-cli-mdm-fields`
  - `contract-cli-mdm-legal`
  - `contract-cli-mdm-vendor`
  - `contract-cli-shared`
- 输出中能看到 `universal` 或 `symlinked` 的平台适配信息。

### 14.2 CLI 内置兜底安装

当通用 installer 不可用，或需要验证 npm 包内嵌 skills 时，执行：

```bash
contract-cli skills list
contract-cli skills install
contract-cli skills install --target ~/.codex/skills
contract-cli skills install --force
```

预期结果：

- `skills list` 能列出内置 skills。
- `skills install` 默认安装到 `$CODEX_HOME/skills` 或 `~/.codex/skills`。
- `--target` 可以覆盖安装目标目录。
- `--force` 可以覆盖已有 skill 目录。

验收注意：

- 通用 installer 依赖 GitHub 仓库内容，因此发版前要确认 skill 文档已经 push。
- CLI 内置安装依赖 npm 包或二进制内嵌内容，因此发布前要跑 `make release-check`。

## 15. bot 文件上传命令专项测试

本模块覆盖今天新增的 `contract-cli contract upload-file`。当前仅支持 bot 身份，user/MCP 三段式上传不在本期范围内。

### 15.1 正向上传

准备小于等于 `200MB` 的测试文件：

```bash
printf '%s\n' '%PDF-1.4 contract-cli upload smoke' > /tmp/contract-upload.pdf
```

执行：

```bash
contract-cli contract upload-file --profile "$PROFILE" --as bot --file /tmp/contract-upload.pdf --file-type attachment --file-name contract-upload.pdf --output json
contract-cli contract upload-file --profile "$PROFILE" --as bot --file /tmp/contract-upload.pdf --file-type attachment --raw
```

预期底层接口：

```text
POST /open-apis/contract/v1/files/upload
```

预期请求：

- 使用 `Authorization: Bearer <bot-token>`。
- 使用 `multipart/form-data`。
- 表单字段包含 `file_name`、`file_type`、`file`。
- `--file-name` 不传时默认使用 `filepath.Base(--file)`。
- `--user-id` / `--user-id-type` 如传入，会继续按通用 query 参数透传。

预期响应：

- 后端返回原始 JSON envelope。
- 成功时重点检查 `data.file_id`。

### 15.2 参数与身份负向测试

```bash
contract-cli contract upload-file --profile "$PROFILE" --as user --file /tmp/contract-upload.pdf --file-type attachment
contract-cli contract upload-file --profile "$PROFILE" --as bot --file /tmp/contract-upload.pdf
contract-cli contract upload-file --profile "$PROFILE" --as bot --file-type attachment
contract-cli contract upload-file --profile "$PROFILE" --as bot --file /tmp --file-type attachment
contract-cli contract upload-file --profile "$PROFILE" --as bot --file /tmp/contract-upload.pdf --file-type attachment --input-file body.json
```

预期结果：

- `--as user` 或 profile 默认身份为 user 时，在发 HTTP 前失败。
- 缺少 `--file` 报 `--file is required`。
- 缺少 `--file-type` 报 `--file-type is required`。
- 文件不存在、目录路径、超过 `200MB` 均报明确错误。
- `contract upload-file` 不接受 `--input-file` / `--data`，这两个参数只用于 JSON 请求体。

### 15.3 file_type 取值参考

常用取值：

- `text`：合同文本。
- `attachment`：其他附件。
- `scan`：归档扫描件。
- `cause`：合同附件。
- `archiveAttachment`：归档附件。
- `customPictureAttachment`：自定义图片附件。
- `customTableAttachment`：自定义表格附件。
- `customFileAttachment`：自定义文件附件。

CLI 不在本地强校验扩展名白名单，扩展名与 `file_type` 的最终合法性以后端校验为准。
