# contract-cli 测试文档

本文档面向 QA、开发自测和发布前验收，覆盖 `contract-cli` 当前已支持能力的端到端测试流程。

## 1. 测试目标

- 验证 CLI 可以通过 npm/npx 或源码方式安装并启动。
- 验证 profile 初始化、user OAuth、app token、登出、默认身份切换等鉴权流程。
- 验证 app 身份下当前已接入的全部结构化业务命令。
- 验证 user 身份下当前已接入的全部结构化业务命令。
- 验证输出格式、通用 Agent skills 安装、CLI 内置 skills 兜底安装、`api call --channel web` 通道及其调用边界。
- 验证每次实际业务 HTTP 请求发送前都会重新探测调用环境，并正确区分 Doubao、WorkBuddy、Codex 或 `unknown`。

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
export PROFILE="contract"
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

业务命令需要准备一组目标租户可访问的数据：

| 变量 | 含义 | 示例 |
| --- | --- | --- |
| `CONTRACT_ID` | 已存在合同 ID | `7023...` |
| `TEMPLATE_ID` | 已存在模板 ID | `TMP...` |
| `VENDOR_ID` | 已存在交易方 ID | `1063...` |
| `LEGAL_ENTITY_ID` | 已存在法人主体 ID | `7023...` |
| `APP_ID` | appId | 不写入文档 |
| `APP_SECRET` | appSecret | 不写入文档 |
| `USER_ID` | app 接口可选透传 user_id | `ou_xxx` |
| `USER_ID_TYPE` | app 接口可选透传 user_id_type | `user_id` / `employee_id` |

建议在 shell 中设置：

```bash
export CONTRACT_ID="<contract-id>"
export TEMPLATE_ID="<template-id>"
export VENDOR_ID="<vendor-id>"
export LEGAL_ENTITY_ID="<legal-entity-id>"
export USER_ID="<user-id>"
export USER_ID_TYPE="user_id"
```

app 凭证建议用环境变量传入，不要写入命令历史：

```bash
export CONTRACT_CLI_APP_ID="<app-id>"
export CONTRACT_CLI_APP_SECRET="<app-secret>"
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
| `npx skills add qfeius/contract-cli -y -g` | 成功安装当前版本的已开放 skills（包含 contract-cli-contract-search），并输出对应 Agent 平台适配信息 |
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
- 输出安装成功及实际数量，并确认包含 `contract-cli-contract-search`；数量以本次发布清单为准。
- 至少包含 `auth`、`contract-cli-contract`、`contract-cli-mdm-fields`、`contract-cli-mdm-legal`、`contract-cli-mdm-vendor`、`contract-cli-shared`。
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
Installed skill: contract-cli-contract-search
Installed skill: contract-cli-mdm-vendor
Installed skill: contract-cli-mdm-legal
Installed skill: contract-cli-mdm-fields
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
contract-cli update check --channel beta --json
```

预期结果：

| 场景 | 预期 |
| --- | --- |
| 远端 beta 比本地新 | 默认输出文本；带 `--json` 时 `action=update_available`，并带 `command` |
| 远端 beta 与本地一致 | 默认输出文本；带 `--json` 时 `action=already_up_to_date`，不带 `_notice.update` |
| 本地是源码 `dev` 或 git hash 构建 | 默认输出跳过文本；带 `--json` 时 `action=skipped`，不访问 npm registry |

自动提示检查：

```bash
contract-cli contract get <contract-id> --profile "$PROFILE" --output json
contract-cli contract get <contract-id> --profile "$PROFILE" --output json
```

预期结果：

- 普通命令先同步读取 `update-check.json`，缓存中有可升级版本时，JSON object 输出包含 `_notice.update`，stderr 不输出旧版升级文本。
- fresh cache 无可升级版本时，当前命令不注入 `_notice.update`，也不请求远端。
- cache 缺失、channel 不匹配或超过 24 小时时，当前命令会在短超时内同步刷新并写入 `update-check.json`；远端返回新版本时当前命令可直接注入 `_notice.update`。
- 网络失败时原命令仍然继续执行，不应因为版本刷新失败而退出；失败结果不写入缓存。

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
contract-cli config add --env prod --name "$PROFILE"
```

预期结果：

- 写入 prod 环境配置。
- 写入开放平台基址 `https://open.qfei.cn`。
- 写入 user OAuth 配置。
- 写入 app token endpoint。
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

### 4.3 app 登录

推荐使用环境变量：

```bash
contract-cli auth login --profile "$PROFILE" --as app
```

也可以显式传参：

```bash
contract-cli auth login --profile "$PROFILE" --as app --app-id "$CONTRACT_CLI_APP_ID" --app-secret "$CONTRACT_CLI_APP_SECRET"
```

预期结果：

- CLI 调用 `https://open.qfei.cn/open-apis/auth/v3/tenant_access_token/internal`。
- 请求体使用 `appId/appSecret`。
- 成功后保存 app token。
- 默认身份切换为 `app`。
- 输出 token 过期时间。

验证状态：

```bash
contract-cli auth status --profile "$PROFILE" --as app
```

预期包含：

```text
Identity: app
Authorization: authorized
Token Protocol: tenant_access_token/internal
Expires At:
```

状态输出不展示开放平台地址、授权端点或 app token endpoint。

### 4.4 默认身份切换

切换到 user：

```bash
contract-cli auth use --profile "$PROFILE" --as user
contract-cli auth status --profile "$PROFILE"
```

切换到 app：

```bash
contract-cli auth use --profile "$PROFILE" --as app
contract-cli auth status --profile "$PROFILE"
```

预期结果：

- `auth use --as user` 后不显式传 `--as` 的业务命令默认走 user。
- `auth use --as app` 后不显式传 `--as` 的业务命令默认走 app。

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

- user logout 不应删除 app token。
- user logout 不应删除 appId/appSecret。

### 4.6 app 登出

```bash
contract-cli auth logout --profile "$PROFILE" --as app
contract-cli auth status --profile "$PROFILE" --as app
```

预期结果：

```text
Authorization: configured
```

回归点：

- 只清空 app token。
- 保留 `appId/appSecret` 和 secret 引用。
- 如果默认身份原本是 app，登出后默认身份仍保持 app。
- 再次执行 `auth login --as app` 时，可以复用已保存凭证重新换 token。

### 4.7 授权异常场景

| 场景 | 命令 | 预期 |
| --- | --- | --- |
| 旧 profile 缺少 app endpoint | `auth login --as app` | 报错并提示重跑 `config add --env prod --name <profile>` |
| appSecret 错误 | `auth login --as app` | 保留新凭证，app token 为空，默认身份不切到 app |
| user 未登录调用 user-only 命令 | `contract enum list --as user` | 报未授权或 token 不可用 |
| app token 已清空后调用 app 命令 | `contract get --as app` | 报未授权或 token 不可用 |

## 5. app 身份业务命令测试

执行本节前确保：

```bash
contract-cli auth login --profile "$PROFILE" --as app
contract-cli auth status --profile "$PROFILE" --as app
```

兼容旧身份值：

```bash
contract-cli auth login --profile "$PROFILE" --as bot
contract-cli contract upload-file --profile "$PROFILE" --as bot --file /tmp/contract-upload.pdf --file-type attachment --output json
```

预期：旧 `--as bot` 不报身份不支持错误，运行时按 app 身份执行；后续配置保存为 `default_identity=app` 和 `identities.app`。

公共检查点：

| 检查项 | 预期 |
| --- | --- |
| 身份 | 显式 `--as app` 或默认身份为 app |
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

contract-cli contract search --profile "$PROFILE" --as app --input-file /tmp/contract-search.json --output json
contract-cli contract search --profile "$PROFILE" --as app --input-file /tmp/contract-search.json --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
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
contract-cli contract get "$CONTRACT_ID" --profile "$PROFILE" --as app --output json
contract-cli contract get "$CONTRACT_ID" --profile "$PROFILE" --as app --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/contracts/{contract_id}
```

### 5.3 同步用户分组

```bash
contract-cli contract sync-user-groups --profile "$PROFILE" --as app --output json
contract-cli contract sync-user-groups --profile "$PROFILE" --as app --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/contracts/user-groups/sync
```

### 5.4 获取合同文本

```bash
contract-cli contract text "$CONTRACT_ID" --profile "$PROFILE" --as app --output json
contract-cli contract text "$CONTRACT_ID" --profile "$PROFILE" --as app --full-text --output json
contract-cli contract text "$CONTRACT_ID" --profile "$PROFILE" --as app --offset 0 --limit 1000 --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/contracts/{contract_id}/text
```

### 5.5 创建合同

创建合同是写操作，建议优先在可回收的测试租户中使用明确可回收的测试数据。

```bash
cat > /tmp/contract-create-app.json <<'JSON'
{
  "contract_name": "contract-cli app create smoke",
  "create_user_id": "REPLACE_WITH_USER_ID"
}
JSON
```

将 `REPLACE_WITH_USER_ID` 替换为真实创建人，再执行：

```bash
contract-cli contract create --profile "$PROFILE" --as app --input-file /tmp/contract-create-app.json --output json
```

也可直接传 JSON：

```bash
contract-cli contract create --profile "$PROFILE" --as app --data '{"contract_name":"contract-cli app create smoke","create_user_id":"'"$USER_ID"'"}' --output json
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
contract-cli contract category list --profile "$PROFILE" --as app --output json
contract-cli contract category list --profile "$PROFILE" --as app --lang zh-CN --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/contract_categorys
```

### 5.7 模板列表

```bash
contract-cli contract template list --profile "$PROFILE" --as app --page-size 10 --output json
contract-cli contract template list --profile "$PROFILE" --as app --category-number "<category-number>" --page-size 10 --page-token "<page-token>" --output json
contract-cli contract template list --profile "$PROFILE" --as app --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/templates
```

### 5.8 模板详情

```bash
contract-cli contract template get "$TEMPLATE_ID" --profile "$PROFILE" --as app --output json
contract-cli contract template get "$TEMPLATE_ID" --profile "$PROFILE" --as app --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/templates/{template_id}
```

### 5.9 创建模板实例

```bash
cat > /tmp/template-instance-app.json <<'JSON'
{
  "template_number": "REPLACE_WITH_TEMPLATE_NUMBER",
  "create_user_id": "REPLACE_WITH_USER_ID"
}
JSON
```

将占位符替换后执行：

```bash
contract-cli contract template instantiate --profile "$PROFILE" --as app --input-file /tmp/template-instance-app.json --output json
contract-cli contract template instantiate --profile "$PROFILE" --as app --input-file /tmp/template-instance-app.json --user-id-type "$USER_ID_TYPE" --output json
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
contract-cli contract upload-file --profile "$PROFILE" --as app --file /tmp/contract-upload.pdf --file-type attachment --file-name contract-upload.pdf --output json
contract-cli contract upload-file --profile "$PROFILE" --as user --file /tmp/contract-upload.pdf --file-type attachment --file-name contract-upload.pdf --output json
contract-cli contract upload-file --profile "$PROFILE" --as app --file /tmp/contract-upload.pdf --file-type attachment --raw
```

预期底层接口：

```text
POST /open-apis/contract/v1/files/upload
```

检查点：

- 请求是 `multipart/form-data`。
- 表单字段包含 `file_name`、`file_type`、`file`。
- 响应 JSON 中应包含后端返回的 `data.file_id`。
- `--as user` 与 `--as app` 均使用同一个上传接口，Authorization 使用对应身份的 token。
- 超过 `200MB`、目录路径、文件不存在、缺少 `--file`、缺少 `--file-type` 都应报明确错误。

### 5.10.1 app-only 合同提交、重提、更新与删除

```bash
contract-cli contract submit "$CONTRACT_ID" --profile "$PROFILE" --as app --output json
contract-cli contract submit "$CONTRACT_ID" --profile "$PROFILE" --as app --data '{"comment":"contract-cli smoke submit"}' --output json
contract-cli contract resubmit "$CONTRACT_ID" --profile "$PROFILE" --as app --output json
contract-cli contract resubmit "$CONTRACT_ID" --profile "$PROFILE" --as app --data '{"comment":"contract-cli smoke resubmit"}' --output json
```

更新合同需要准备 JSON 请求体：

```bash
cat > /tmp/contract-patch-app.json <<'JSON'
{
  "contract_name": "contract-cli app patch smoke"
}
JSON

contract-cli contract patch "$CONTRACT_ID" --profile "$PROFILE" --as app --input-file /tmp/contract-patch-app.json --output json
```

删除草稿合同是破坏性操作，只能对明确可回收的测试草稿合同执行：

```bash
contract-cli contract delete "$DRAFT_CONTRACT_ID" --profile "$PROFILE" --as app --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/contracts/{contract_id}/submit
POST /open-apis/contract/v1/contracts/{contract_id}/resubmit
PATCH /open-apis/contract/v1/contracts/{contract_id}
DELETE /open-apis/contract/v1/contracts/{contract_id}
```

检查点：

- 四个命令当前都仅支持 app 身份，显式 `--as user` 应在发 HTTP 前失败。
- `submit` / `resubmit` 的 `--input-file` / `--data` 可选，不传时不发送请求体。
- `patch` 的 `--input-file` / `--data` 必须传一个且互斥。
- `delete` 不要求 `--yes`，执行前由测试者自行确认目标是草稿合同。

### 5.10.2 双身份下载与 app-only 生成打印文件

下载文件建议显式指定保存路径，避免默认保存弹窗在 Agent、SSH 或 CI 环境不可用：

```bash
contract-cli contract download-file "$FILE_ID" --profile "$PROFILE" --as app --output-file /tmp/contract-download.pdf
contract-cli contract download-file "$FILE_ID" --profile "$PROFILE" --as app --output-file /tmp/contract-download.pdf --force
contract-cli contract download-file "$FILE_ID" --profile "$PROFILE" --as app --raw > /tmp/contract-download.raw
contract-cli contract download-file "$FILE_ID" --contract "$CONTRACT_ID" --profile "$PROFILE" --as user --output-file /tmp/contract-download-user.pdf
```

生成打印文件需要准备 JSON 请求体：

```bash
cat > /tmp/contract-print-file-app.json <<'JSON'
{
  "contract_id": "REPLACE_WITH_CONTRACT_ID"
}
JSON

contract-cli contract print-file --profile "$PROFILE" --as app --input-file /tmp/contract-print-file-app.json --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/files/{file_id}
GET /open-apis/contract/v1/mcp/contracts/{contract_id}/files/{file_id}/download
POST /open-apis/contract/v1/files
```

检查点：

- `download-file` 支持 user/app，不支持 `dowload-file` 拼写；user 身份必须传 `--contract`。
- user 身份先获取 JSON 元数据，再无 Authorization 访问 300 秒TOS 临时预签名 URL；日志和输出不得出现该地址。
- user 文件保存失败时不得破坏已有目标文件或遗留未完成的新文件；完整下载并优先按实际 `Content-Length` 校验长度（缺失时回退到 `file_size`）后才提交，且需覆盖硬链接不可用时的独占创建降级路径。
- 不传 `--output-file` 且不传 `--raw` 时，CLI 默认拉起保存文件弹窗。
- 无 GUI、远程、CI、Agent 环境下，保存弹窗失败时不应发 HTTP，并提示改用 `--output-file`。
- `--output-file` 文件已存在时默认失败，加 `--force` 才覆盖。
- `print-file` 的 `--input-file` / `--data` 必须传一个且互斥。

### 5.10.3 app-only 分享记录与协商信息

```bash
contract-cli contract share get "$CONTRACT_ID" --profile "$PROFILE" --as app --output json
contract-cli contract cooperation link get "$CONTRACT_ID" --profile "$PROFILE" --as app --output json
contract-cli contract cooperation record get "$CONTRACT_ID" --profile "$PROFILE" --as app --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/contracts/{contract_id}/share_records
GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_link
GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_record_info
```

检查点：

- 三个命令当前都仅支持 app 身份。
- 响应保持后端 JSON envelope，不做 CLI 侧结构归一化。

### 5.11 交易方列表

```bash
contract-cli mdm vendor list --profile "$PROFILE" --as app --page-size 10 --output json
contract-cli mdm vendor list --profile "$PROFILE" --as app --name "供应商" --page-size 10 --output json
contract-cli mdm vendor list --profile "$PROFILE" --as app --name "供应商" --page-token "<page-token>" --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/vendors
```

### 5.12 交易方详情

```bash
contract-cli mdm vendor get "$VENDOR_ID" --profile "$PROFILE" --as app --output json
contract-cli mdm vendor get "$VENDOR_ID" --profile "$PROFILE" --as app --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/vendors/{vendor_id}
```

### 5.13 法人主体列表

```bash
contract-cli mdm legal list --profile "$PROFILE" --as app --page-size 10 --output json
contract-cli mdm legal list --profile "$PROFILE" --as app --name "主体A" --page-size 10 --output json
contract-cli mdm legal list --profile "$PROFILE" --as app --name "主体A" --page-token "<page-token>" --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/legal_entities/list_all
```

### 5.14 法人主体详情

```bash
contract-cli mdm legal get "$LEGAL_ENTITY_ID" --profile "$PROFILE" --as app --output json
contract-cli mdm legal get "$LEGAL_ENTITY_ID" --profile "$PROFILE" --as app --user-id "$USER_ID" --user-id-type "$USER_ID_TYPE" --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/legal_entities/{legal_entity_id}
```

检查点：

- app 路由会同时带 path 参数和同名 query `legal_entity_id`。

### 5.14.1 法人主体按编码查询

```bash
contract-cli mdm legal get --profile "$PROFILE" --as app --code "$LEGAL_ENTITY_CODE" --page-size 10 --output json
contract-cli mdm legal get --profile "$PROFILE" --as app --code "$LEGAL_ENTITY_CODE" --page-token "<page-token>" --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/legal_entities
```

检查点：

- 当前仅支持 app 身份。
- `--code` 映射为 query `legalEntity`。
- 该接口不同于法人主体列表的 `/open-apis/mdm/v1/legal_entities/list_all`。

### 5.15 字段配置

```bash
contract-cli mdm fields list --profile "$PROFILE" --as app --biz-line vendor --output json
contract-cli mdm fields list --profile "$PROFILE" --as app --biz-line legal_entity --output json
```

预期底层接口：

```text
GET /open-apis/mdm/v1/config/config_list
```

检查点：

- `legal_entity` 会在 app 路由下映射为 query `biz_line=legalEntity`。
- `vendor_risk` 当前不支持 app，执行 `--as app --biz-line vendor_risk` 应在本地报错且不发 HTTP。

### 5.16 MDM 写接口

```bash
contract-cli mdm vendor create --profile "$PROFILE" --as app --user-id "$USER_ID" --input-file /tmp/vendor-create.json --output json
contract-cli mdm vendor update "$VENDOR_ID" --profile "$PROFILE" --as app --user-id "$USER_ID" --input-file /tmp/vendor-update.json --output json
contract-cli mdm legal create --profile "$PROFILE" --as app --user-id "$USER_ID" --input-file /tmp/legal-create.json --output json
contract-cli mdm legal update "$LEGAL_ENTITY_ID" --profile "$PROFILE" --as app --user-id "$USER_ID" --input-file /tmp/legal-update.json --output json
```

检查点：

- 四个写接口当前仅支持 app 身份，且必须传 `--user-id`。
- `vendor create` 请求体不要带后端生成的 `vendor` 编码；`vendor update` 请求体必须带后端返回的 `id` 和 `vendor` 编码。
- `legal create` 请求体不要带后端生成的 `legalEntity` / `legal_entity` 编码；`legal update` 请求体必须带后端返回的 `id` 和 camelCase `legalEntity` 编码。
- 不传 `--user-id`、create 传生成编码、update 缺少生成 `id` / 编码时应在本地报错且不发 HTTP。

### 5.17 固定汇率

```bash
contract-cli mdm fixed-exchange-rate get --profile "$PROFILE" --as app --source-currency CNY --target-currency USD --effective-date 2026-06-01 --output json
contract-cli mdm fixed-exchange-rate update --profile "$PROFILE" --as app --input-file /tmp/fixed-exchange-rate.json --output json
```

检查点：

- `get` 走 `GET /open-apis/mdm/v1/fixed_exchange_rate`，CLI flag `--effective-date` 映射到底层 query `date`。
- `update` 走 `PUT /open-apis/mdm/v1/fixed_exchange_rate`，请求体仍使用字段 `effective_date`。

### 5.18 事件出口 IP

```bash
contract-cli event outbound-ip list --profile "$PROFILE" --as app --page-size 10 --output json
contract-cli event outbound-ip list --profile "$PROFILE" --as app --page-size 3 --output json
```

检查点：

- 正常分页走 `GET /open-apis/event/v1/outbound_ip`。
- `--page-size` 传入时必须在 `10` 到 `50` 之间；`--page-size 3` 应本地报错且不发 HTTP。

### 5.19 app 不支持命令的负向验证

```bash
contract-cli contract enum list --profile "$PROFILE" --as app --type contract_status
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
- 显式传 `--user-id-type employee_id`、`union_id` 或空值应本地报错，搜索 HTTP 次数为 0；省略及 `user_id` 正常执行，app 仍透传。
- `--user-id` 这种 MCP 固定 query 不存在的通用参数可以被补充。
- HTTP 200 下业务 code 非零、success=false 或缺失 code 时，user 搜索必须失败退出并保留 JSON/raw 响应；成功空结果仍正常退出。
- 在 mock transport 截获并校验大整数、高精度小数、嵌套数组和科学计数原值；多文档、尾部垃圾和非对象输入应被拒绝。真实金额查询在修复前后结果集保持一致。

本地参数拒绝负例（预期报错，不发送搜索请求）：

```bash
contract-cli contract search --profile "$PROFILE" --as user --input-file /tmp/contract-search-user.json --user-id "$USER_ID" --user-id-type employee_id --output json
```

### 6.1.1 搜索字段发现

本次开发版新增 `contract search-fields`，旧 CLI 1.8.6 不支持。使用当前已授权的配置和 user 身份验证，保持 `CONTRACT_CLI_CONFIG_DIR` 不变。

```bash
contract-cli contract search-fields --help
contract-cli contract search-fields --keyword "项目区域" --profile "$PROFILE" --as user --output json
```

预期底层接口：

```text
GET /open-apis/contract/v1/mcp/contracts/search/filter_fields?keyword=<URL 编码后的字段展示名>
```

自动化回归点：

- 本地帮助无 HTTP 调用，列出可选 `--keyword` 和 user-only 限制。
- 关键词按原值进行 query 编码，覆盖中文、空格、`&`、`+` 等字符；只传 `keyword`，不注入 `user_id_type`、`lang` 或分页参数。
- 省略 `--keyword` 时不发送该 query 参数；完整目录场景使用 mock 验证，实际全量查询需用户明确要求。
- 省略 `--as` 时仍使用 user，显式 app 在发请求前失败；不接受请求体和未支持的语言、身份 ID 类型或分页参数。
- 基础字段与自定义字段完整保留，尤其是 `request_location`、`request_paths`、`search_field`、`filter_unique_key`、`search_value_type`、`value_description`、`usage_hint`、`value_scopes` 和 `examples`；不把所有字段改写成 `filter_units`。
- HTTP 200 下非零业务 `code` 或 `success=false` 仍返回失败退出码，并保留错误信息；成功空匹配与业务错误明确区分。

获授权后的只读联调：

- 使用租户真实展示名查询，核对字段 key、搜索路径和值类型；不以示例“项目区域”必然存在作为通过条件。
- 从真实元数据中选择字段并构造搜索请求，检查选项/日期/数字/文本等类型约束；没有候选时不猜 key，不删原条件。
- 固定申请人姓名和部门名称按主 Skill 场景 6、7 直接查；不以缺少人员/部门列表为失败条件。
- 安装后的 `contract-cli-contract-search` Skill 能发现新命令，并保留主文件 9 个场景的三段结构。

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

创建合同是写操作，建议在可回收的测试租户中使用可回收测试数据。

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
- app 身份调用应失败。

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

## 7. api call Open Web 通道测试

### 7.1 已验证配方的只读调用

在已确认的 prod profile 和授权范围内，使用业务 Skill 已验证的只读接口配方；以下示例来自合同交接的当前用户查询：

```bash
contract-cli api call --channel web GET /open-web/web/clm/employee/info --profile "$PROFILE" --as user --output json
```

预期结果：

- 复用 CLI 自身 profile 的 user 授权，响应按业务 Skill 的映射判断，不以 HTTP 2xx 直接认定业务成功。
- query 使用可重复的 `--query key=value`；JSON 使用 `--input-file` / `--data`，请求体不超过 1 MiB，不自动补 `user_id_type`。
- 不依赖独立的 `contract-cli-api-call` Skill，不临时猜测接口、参数或业务结果。

### 7.2 通道、路径、身份与请求头边界

```bash
contract-cli api call GET /open-web/web/clm/employee/info --profile "$PROFILE"
contract-cli api call --channel web GET /open-apis/contract/v1/mcp/config/config_list --profile "$PROFILE"
contract-cli api call --channel web GET /open-web/web/clm/employee/info --profile "$PROFILE" --as app
contract-cli api call --channel web GET /open-web/web/clm/employee/info --profile "$PROFILE" --header 'Authorization: blocked'
```

预期结果：上述请求均在发送业务 HTTP 请求前失败，分别提示缺少 `--channel web`、路径必须是相对 `/open-web/web/`、仅支持 user、Authorization 由 CLI 管理。其他通道、绝对 URL、路径中嵌入 query、路径穿越、Cookie 和可信身份头也应被拒绝。

### 7.3 帮助入口公开

```bash
contract-cli --help
contract-cli help api call
contract-cli api call --help
```

预期结果：

- `contract-cli --help` 展示 `contract-cli api call`。
- 两种调用帮助均展示必填 `--channel web`、仅支持 user 与 `/open-web/web/` 路径的限制。
- `rule` 及 `approval-matrix` 别名的结构操作帮助不再声称通用调用被关闭；矩阵操作仍使用结构化命令。

### 7.4 不发布独立传输 Skill

```bash
contract-cli skills list
contract-cli skills install --target "$SKILLS_TARGET"
```

预期结果：

- `skills list` 不展示 `contract-cli-api-call`。
- `skills install` 不安装 `contract-cli-api-call`。

## 8. 输出格式与参数解析测试

### 8.1 JSON 输出

```bash
contract-cli mdm fields list --profile "$PROFILE" --as app --biz-line vendor --output json
```

预期结果：

- 输出合法 JSON。

### 8.2 YAML 输出

```bash
contract-cli mdm fields list --profile "$PROFILE" --as app --biz-line vendor --output yaml
```

预期结果：

- 输出合法 YAML。

### 8.3 raw 输出

```bash
contract-cli mdm fields list --profile "$PROFILE" --as app --biz-line vendor --output json --raw
```

预期结果：

- 输出后端原始 envelope。

### 8.4 `--input-file` 与 `--data` 互斥

```bash
contract-cli contract create --profile "$PROFILE" --as app --input-file /tmp/contract-create-app.json --data '{}'
```

预期结果：

- CLI 报错。
- 不发 HTTP 请求。

### 8.5 `--file` 仅用于真实文件上传

```bash
contract-cli contract create --profile "$PROFILE" --as app --file /tmp/contract-create-app.json
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
- `auth login --as app`
- `auth status --as user`
- `auth status --as app`
- `auth logout --as user`
- `auth logout --as app`
- `auth use --as user`
- `auth use --as app`
- user logout 不影响 app。
- app logout 保留 app 凭证。
- app token 过期状态显示 `expired`。

### 10.3 每次改 openplatform client 后必须覆盖

- `IdentityPolicyUserOnly` 下 MCP 固定 query 不被通用 query 覆盖。
- `IdentityPolicyAny` 下 app/open API 仍允许通用 query 覆盖同名参数。
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
| app 登录失败 | 检查 `CONTRACT_CLI_APP_ID`、`CONTRACT_CLI_APP_SECRET`、`app_token_endpoint` |
| user 登录卡住 | 检查浏览器 OAuth 回调、`--timeout`、是否需要 `--no-open-browser` 手动打开 URL |
| MCP 命令 app 调用失败 | 预期行为，`contract/v1/mcp` 默认 user-only |
| 创建合同字段缺失 | 对照 `skills/contract-cli-contract/references/create-contract-fields.md` 补齐场景必填字段 |
| IDE 或命令爆红 | 先确认 `go.mod` 已 reload，`go test ./...` 是否通过 |

## 12. 测试验收标准

一轮完整验收应满足：

- CLI 可以通过 npm beta 包安装并执行 `contract-cli --version`。
- `contract-cli skills list` 和 `contract-cli skills install` 成功。
- `config add --env prod` 成功。
- user 登录、状态、切换、登出成功。
- app 登录、状态、切换、登出成功，且 app logout 保留凭证。
- app 身份下第 5 节结构化业务命令完成正向验证，`contract upload-file` 需覆盖 user/app 两种身份；写操作至少在可回收测试租户完成一次可回收数据验证。
- user 身份下第 6 节十五条结构化业务命令完成正向验证。
- `api call --channel web` 已验证只读配方、调用边界、公开帮助及非独立 Skill 安装限制完成验证。
- `make release-check` 通过。

## 13. 版本升级专项测试

本模块覆盖今天新增的版本管理与升级提示能力。它和安装测试分开验收，避免把“包能安装”和“客户端能发现远端新版本”混在一起。

### 13.1 手动版本检查

```bash
contract-cli update check
contract-cli update check --channel beta
contract-cli update check --channel latest
contract-cli update check --channel latest --json
```

预期结果：

- 当前版本是预发布版本时，不传 `--channel` 默认检查 npm `beta` dist-tag。
- 当前版本是稳定版本时，不传 `--channel` 默认检查 npm `latest` dist-tag。
- 不带 `--json` 时输出文本提示，和飞书 `lark-cli update --check` 保持一致。
- 带 `--json` 时输出飞书式顶层字段：`ok`、`previous_version`、`current_version`、`latest_version`、`action`、`message`。
- 远端版本更新时，`action=update_available`，并带 `command`。
- `command` 为 `npm install -g @qfeius/contract-cli@<channel> --registry https://registry.npmjs.org`。
- 远端版本未更新时，`action=already_up_to_date`。
- 本地是 `dev`、`unknown` 或非语义化版本时，`action=skipped`。
- 手动 `update check --json` 不注入 `_notice.update`。

### 13.2 自动升级提示

执行会返回 JSON object 的普通命令：

```bash
contract-cli contract get <contract-id> --profile "$PROFILE" --output json
contract-cli contract search --profile "$PROFILE" --data '{}'
```

预期结果：

- 业务命令先同步读取本地 cache；fresh cache 24 小时内不再请求 npm registry，因此刚发布的新包可能要等缓存过期后才提示。
- cache 缺失、channel 不匹配或过期时，当前命令会在短超时内同步刷新 cache。
- 发现缓存或远端结果中有新版本时，JSON object 输出注入 `_notice.update`；stderr 不输出旧版升级文本。
- `--raw`、yaml、table、纯文本命令不注入 `_notice.update`。
- 刷新失败不阻断原命令；失败结果不写入缓存。
- `contract-cli version`、`contract-cli update check` 自身不触发自动检查。
- CI 环境跳过自动远端检查。

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
- 输出安装成功及实际数量，并确认包含 `contract-cli-contract-search`；数量以本次发布清单为准。
- 安装内容至少包含：
  - `auth`
  - `contract-cli-contract-search`
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

## 15. 文件上传命令专项测试

本模块覆盖 `contract-cli contract upload-file`。当前 user/app 身份均支持。app 和 user 普通类型继续调用原上传接口；user 的 `reviewAttachment` / `approveAttachment` 自动执行 MCP prepare → content → commit。

### 15.1 正向上传

准备小于等于 `200MB` 的测试文件：

```bash
printf '%s\n' '%PDF-1.4 contract-cli upload smoke' > /tmp/contract-upload.pdf
```

执行：

```bash
contract-cli contract upload-file --profile "$PROFILE" --as app --file /tmp/contract-upload.pdf --file-type attachment --file-name contract-upload.pdf --output json
contract-cli contract upload-file --profile "$PROFILE" --as user --file /tmp/contract-upload.pdf --file-type attachment --file-name contract-upload.pdf --output json
contract-cli contract upload-file --profile "$PROFILE" --as app --file /tmp/contract-upload.pdf --file-type attachment --raw
```

预期底层接口：

```text
POST /open-apis/contract/v1/files/upload
```

预期请求：

- 使用 `Authorization: Bearer <user-token>` 或 `Authorization: Bearer <app-token>`，取决于 `--as` 或 profile 默认身份。
- 使用 `multipart/form-data`。
- 表单字段包含 `file_name`、`file_type`、`file`。
- `--file-name` 不传时默认使用 `filepath.Base(--file)`。
- `--user-id` / `--user-id-type` 如传入，会继续按通用 query 参数透传。

预期响应：

- 后端返回原始 JSON envelope。
- 成功时重点检查 `data.file_id`。

### 15.2 参数负向测试

```bash
contract-cli contract upload-file --profile "$PROFILE" --as app --file /tmp/contract-upload.pdf
contract-cli contract upload-file --profile "$PROFILE" --as app --file-type attachment
contract-cli contract upload-file --profile "$PROFILE" --as app --file /tmp --file-type attachment
contract-cli contract upload-file --profile "$PROFILE" --as app --file /tmp/contract-upload.pdf --file-type attachment --input-file body.json
```

预期结果：

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

## 16. P2 付款与审批命令专项测试

本模块覆盖 `payment *` 和 `contract approval *`。付款与 `approval start` 为 app-only；MCP 评论和个人任务命令为 user-only；`approval get` 支持双身份。

### 16.1 付款申请

```bash
contract-cli payment create --contract "$CONTRACT_ID" --profile "$PROFILE" --as app --input-file payment.json --output json
contract-cli payment update "$PAYMENT_ID" --contract "$CONTRACT_ID" --profile "$PROFILE" --as app --input-file payment-update.json --output json
contract-cli payment get "$PAYMENT_ID" --contract "$CONTRACT_ID" --profile "$PROFILE" --as app --output json
contract-cli payment list --contract "$CONTRACT_ID" --profile "$PROFILE" --as app --page-size 10 --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/contracts/{contract_id}/payments
PATCH /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}
GET /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}
GET /open-apis/contract/v1/contracts/{contract_id}/payments
```

### 16.2 付款计划与付款记录

```bash
contract-cli payment plan notify --profile "$PROFILE" --as app --input-file notify.json --output json
contract-cli payment plan search --profile "$PROFILE" --as app --input-file payment-plan-search.json --output json
contract-cli payment record create --contract "$CONTRACT_ID" --payment "$PAYMENT_ID" --profile "$PROFILE" --as app --input-file payment-record.json --output json
contract-cli payment record update "$PAYMENT_RECORD_ID" --contract "$CONTRACT_ID" --payment "$PAYMENT_ID" --profile "$PROFILE" --as app --input-file payment-record-update.json --output json
contract-cli payment record get "$PAYMENT_RECORD_ID" --contract "$CONTRACT_ID" --payment "$PAYMENT_ID" --profile "$PROFILE" --as app --output json
contract-cli payment record list --plan "$PAYMENT_PLAN_UUID" --profile "$PROFILE" --as app --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/payment/notify
POST /open-apis/contract/v1/payments/search
POST /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}/payment_records
PATCH /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}/payment_records/{payment_record_id}
GET /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}/payment_records/{payment_record_id}
GET /open-apis/contract/v1/contracts/payments/{payment_plan_uuid}/payment_records
```

### 16.3 审批管理

```bash
contract-cli contract approval start "$PROCESS_INSTANCE_ID" --profile "$PROFILE" --as app --input-file approval.json --output json
contract-cli contract approval get "$PROCESS_INSTANCE_ID" --profile "$PROFILE" --as app --notice-filter notice_filter --task-instance-filter task_instance_filter --output json
contract-cli contract approval get "$PROCESS_INSTANCE_ID" --profile "$PROFILE" --as user --output json
contract-cli contract approval comment list "$PROCESS_INSTANCE_ID" --profile "$PROFILE" --as user --output json
contract-cli contract approval comment create "$PROCESS_INSTANCE_ID" --profile "$PROFILE" --as user --data '{"content":"请确认"}' --output json
contract-cli contract approval task list --profile "$PROFILE" --as user --task-type todo --page-size 20 --output json
contract-cli contract approval task approve "$TASK_INSTANCE_ID" --profile "$PROFILE" --as user --comment "同意" --output json
contract-cli contract approval task reject "$TASK_INSTANCE_ID" --profile "$PROFILE" --as user --comment "条款风险未解决" --output json
```

预期底层接口：

```text
POST /open-apis/contract/v1/process_instances/{process_instance_id}/task_approval
GET /open-apis/contract/v1/process_instances/{process_instance_id}
GET /open-apis/contract/v1/mcp/process_instances/{process_instance_id}
GET /open-apis/contract/v1/mcp/process_instances/{process_instance_id}/comments
POST /open-apis/contract/v1/mcp/process_instances/{process_instance_id}/comments
POST /open-apis/contract/v1/mcp/tasks
POST /open-apis/contract/v1/mcp/tasks/{task_instance_id}/approval
```

### 16.4 参数负向测试

```bash
contract-cli payment create --profile "$PROFILE" --as app --input-file payment.json
contract-cli payment update "$PAYMENT_ID" --contract "$CONTRACT_ID" --profile "$PROFILE" --as app
contract-cli payment record create --contract "$CONTRACT_ID" --profile "$PROFILE" --as app --input-file payment-record.json
contract-cli payment record list --profile "$PROFILE" --as app
contract-cli contract approval start "$PROCESS_INSTANCE_ID" --profile "$PROFILE" --as app
contract-cli contract approval comment list "$PROCESS_INSTANCE_ID" --profile "$PROFILE" --as app
contract-cli contract approval task reject "$TASK_INSTANCE_ID" --profile "$PROFILE" --as user --comment "   "
contract-cli contract download-file "$FILE_ID" --profile "$PROFILE" --as user --output-file /tmp/missing-contract.pdf
contract-cli payment get "$PAYMENT_ID" --contract "$CONTRACT_ID" --profile "$PROFILE" --as user
```

预期结果：

- 缺少 `--contract`、`--payment`、`--plan` 或位置 ID 时返回明确 usage 或必填错误，且不发送 HTTP。
- POST/PATCH 缺少 `--input-file` / `--data` 时报错，且不发送 HTTP。
- GET/list 命令传入 `--input-file` / `--data` 时报错，且不发送 HTTP。
- 显式 `--as user` 调用 app-only 命令时报 `only supports --as app`，显式 `--as app` 调用 user-only 命令时报 `only supports --as user`，且不发送 HTTP。
- reject 空白意见、user 下载缺少 `--contract` 均在发 HTTP 前失败。

### 16.5 User MCP 业务状态与审批意见日志回归

- 审批详情、评论查询/创建、任务查询/通过/拒绝收到 HTTP 200 且业务码非零时返回失败；stdout 保留响应，业务错误不得自动重试。
- `code=0` 且 `success=false` 仍失败；`code=0` 且未提供 `success` 兼容成功响应。
- 空响应、非法 JSON、缺少或类型错误的 `code` 不得按成功处理；JSON/YAML/raw 三种输出均验证。
- JSON/YAML 输出保留 Long ID 精度，raw 保留原始响应；输出写入失败必须向上传递。
- `--comment <text>` 和 `--comment=<text>` 在 approve/reject 命令的日志中均被脱敏，HTTP 请求中的意见保持完整。
- app 下载、app 审批详情及既有 user 合同详情的响应行为保持不变；原有 Token 刷新和网络重试测试继续通过。

### 16.6 新附件上传授权与下载长度回归

- user 显式/默认身份分别上传 `reviewAttachment`、`approveAttachment`，确认只有 prepare/commit 携带 user Token，content 仅有 multipart `file`；最终输出为 commit 的 `data.file_id`，JSON/YAML/raw 不丢失长 ID。
- 用同一用户将新文件 ID 分别传入评论 `file_ids` 和审批 `--file-id`；验证其他用户、普通 app 上传、commit 后超过 30 分钟且未关联业务的文件不能借此绕过权限。
- 验证已有当前合同可见文件仍可复用于评论，已有当前审批业务的 `approveAttachment` 仍可复用于审批。
- prepare 非成功、参数缺失、非 HTTPS、非 POST、非空上传 headers、过期及大小超限均不得发送文件；content 失败不得 commit，commit 失败不得输出文件 ID。
- 三步分别覆盖业务失败、网络中断、5xx、取消；确认不自动重试、保留取消原因、不泄露上传会话 URL/ID/Token；临时 content 地址重定向应失败。
- 保持 app 上传全部类型、user 普通类型的旧接口行为；空 MCP 附件在发送前失败，multipart 仍采用流式处理。
- 实际下载 `Content-Length` 优先于历史 `file_size`，覆盖实际长度为 0、元数据为 0、历史加密大小不同；无 Content-Length 时回退到元数据。流截断、404/503 均不得替换旧文件，临时文件必须清理。
- `complete=false` / `limitations` 部分成功与长字符串 ID 在 JSON/YAML/raw 下保留；`110002` 返回非零退出且不按业务错误自动重试。

## 17. 调用环境识别专项测试

本模块不要求先向 npm 发布版本。可以直接验证本地二进制，也可以生成带本地 release assets 的 tgz 并安装到隔离目录。

### 17.1 本地二进制

```bash
mkdir -p dist/local-test
go build -trimpath -o dist/local-test/contract-cli ./cmd/contract-cli
./dist/local-test/contract-cli environment inspect --output json
./dist/local-test/contract-cli environment inspect --output json --include-processes
```

在普通终端直接执行时通常应得到 `channel_type=cli`、`agent_source_type=unknown`。这不是失败，因为父进程链里只有 Terminal、Shell 等进程。

### 17.2 从真实客户端调用

分别让 Doubao、Doubao Work、WorkBuddy 和 Codex 直接执行测试二进制的绝对路径：

```text
<仓库绝对路径>/dist/local-test/contract-cli environment inspect --output json --include-processes
```

预期结果：

- Doubao 调用返回 `channel_type=cli`、`agent_source_type=doubao`。
- Doubao Work 调用返回 `channel_type=cli`、`agent_source_type=doubaoWork`。
- WorkBuddy 调用返回 `channel_type=cli`、`agent_source_type=workbuddy`。
- Codex 调用返回 `channel_type=cli`、`agent_source_type=codex`。
- macOS 官方签名匹配时返回 `evidence_type=macos_code_signature`、`confidence=high`。
- Windows 商店包身份匹配时返回 `evidence_type=windows_package_identity`、`confidence=high`。
- Windows 普通桌面程序通过 WinVerifyTrust 且证书指纹与路径均匹配时，返回 `evidence_type=windows_authenticode`、`confidence=high`。
- 两个客户端交替执行时，每个新 CLI 进程都按自己的父进程链判断，不读取上一次结果。

如果客户端只是要求用户复制命令到外部 Terminal 执行，父进程将是 Terminal，而不是该客户端；这类方式不能作为真实客户端调用测试。

### 17.3 真实业务请求 Hook

使用已经授权的 profile，从 Doubao、Doubao Work、WorkBuddy 和 Codex 分别调用同一条只读业务命令，并在 Higress/OpenPlatform 接收端检查：

```text
X-Qfei-Channel-Type: cli
X-Qfei-Agent-Source-Type: doubao | doubaoWork | workbuddy | codex | unknown
X-Qfei-Product-Code: contract
X-Qfei-Evidence-Type: ...
X-Qfei-Channel-Confidence: ...
X-Qfei-Detector-Version: process-ancestry-v6
X-Qfei-Rule-Id: ...
traceparent: 00-<32 位 trace_id>-<16 位 span_id>-01
X-Log-Id: <与 traceparent 相同的 trace_id>
```

检查点：

- 每个实际 HTTP attempt 前探测函数调用一次。
- 请求重试或 Token 刷新后的业务请求重放会再次探测。
- 单次完整探测共享 5 秒预算，卡住时终止并回收探测 helper；macOS/Linux 的签名子进程也应终止。预算之外只允许短暂的进程回收开销。
- 探测自身超时或失败：保留本次已收到的完整报告；没有有效报告时，该次请求携带 `agent-source-type=unknown`、`evidence-type=none`、`confidence=unknown`，保留 `channel-type=cli`、`product-code=contract` 和 detector version，清除旧 Rule ID；业务照常发送，不增加业务重试。
- 超时之后下一次请求必须重新探测；用户主动取消则不发业务请求。普通请求和流式请求均覆盖。
- 调用环境没有写入 profile、OAuth Token 或其他持久化配置。
- Header 不包含 PID、进程路径或完整命令行。
- `traceparent` 符合 W3C 格式，且其中的 `trace_id` 与 `X-Log-Id` 完全相同。
- 同一逻辑请求发生网络重试或 Token 刷新重放时，所有 attempt 的 `trace_id` 相同、`span_id` 不同。
- 两次独立业务命令产生不同的 `trace_id`。
- 服务端返回错误时，CLI 最终错误信息包含 `trace_id=<值>`，可复制该值检索日志或 Trace。

CLI 单元测试只能保证 Header 已写入出站请求。完整链路验收还需在部署环境中用该 `trace_id` 检查：OpenPlatform 入口日志的 `X-Log-Id` 等于该值，并且 Trace 查询能返回相同 `trace_id` 的下游 spans；若不一致，应检查 Higress 和服务间 HTTP 客户端是否保留 `traceparent`。

### 17.4 不发布 npm 的本地安装包

```bash
make release-assets
mkdir -p dist/local-test
npm pack --pack-destination dist/local-test
npm install -g "$(pwd)/dist/local-test/qfeius-contract-cli-1.8.3.tgz" \
  --prefix "$(pwd)/dist/local-prefix"
"$(pwd)/dist/local-prefix/bin/contract-cli" --version
"$(pwd)/dist/local-prefix/bin/contract-cli" environment inspect --output json
```

该方式安装的是当前工作区编译出的本地二进制，不访问或发布新的 npm 版本，也不会覆盖系统全局安装。真实客户端验证时使用 `dist/local-prefix/bin/contract-cli` 的绝对路径。

### 17.5 Windows 真机/虚拟机验证

在 Windows PowerShell 中拉取本分支后构建：

```powershell
go build -trimpath -o dist/local-test/contract-cli.exe ./cmd/contract-cli
& (Resolve-Path ./dist/local-test/contract-cli.exe) environment inspect --output json --include-processes
```

先在普通 PowerShell 运行一次，通常应为 `channel_type=cli`、`agent_source_type=unknown`。然后分别让 Doubao、Doubao Work、WorkBuddy 或 Codex 直接调用同一个 `contract-cli.exe` 绝对路径。诊断重点：

- `matched_process` 应落在真实客户端祖先进程，而不是 PowerShell、cmd 或 Terminal。
- Codex 商店包应输出 `application.package_family_name=OpenAI.Codex_2p2nqsd0c76g0`。
- Doubao/Doubao Work/WorkBuddy 命中已登记的有效签名时，应输出 `application.signature_valid=true`、非空 `publisher` 与 `certificate_sha256`，置信度为 high；路径或进程名兜底为 medium/low。
- 如出现 `windows_authenticode_mismatch`，保留完整 JSON 并用以下命令核对实际父进程文件；客户端换证书后仍可按明确路径或进程名降级归因，核验并登记新身份后才恢复签名 high 置信度。

```powershell
Get-AuthenticodeSignature "<matched_process.executable>" | Format-List Status,StatusMessage,SignerCertificate
(Get-Item "<matched_process.executable>").VersionInfo | Format-List CompanyName,ProductName,FileVersion,OriginalFilename
```

最后从真实客户端执行一条已授权的只读业务命令，在 Higress/OpenPlatform 接收端核对 17.3 的七个来源 Header 和两个 Trace Header；只运行 `environment inspect` 不能证明业务 Hook 已透传。

## 人员与部门独立命令验收（2026-09-20）

- 顶层 `employee list` / `department list`：中文与保留字符编码、部门单值映射、分页 token 原样传递；人员必须有条件，部门允许读取目录一页；默认 user、显式 app 拒绝。
- page-size 1/200 成功，0/201/负数/小数/空值失败；名称与部门 ID 互斥，重复和空条件拒绝；不暴露 JSON body 或身份覆盖。
- 响应保持 user_id/department_id 字符串、状态、未知大整数和分页字段；错误 code/success 返回失败，正常空列表成功。
- mock 串联人员→需求人、部门→需求部门合同筛选；部署验证重名消歧、完整分页及保留原查询条件。
- 日志不包含姓名、部门 ID、分页 token 或 token；网络/HTTP 失败的最终错误保留状态和 trace，避免完整 URL 与服务端正文泄露。
- `skills list/install`、npm 包检查和本地安装应包含 `contract-cli-employee`、`contract-cli-department` 的主文件及 agents 元数据；搜索跨 Skill 引用可解析。
