# contract-cli

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24.3-blue.svg)](https://go.dev/)
[![Node.js](https://img.shields.io/badge/node-%3E%3D16-green.svg)](https://nodejs.org/)
[![License](https://img.shields.io/badge/license-UNLICENSED-lightgrey.svg)](./LICENSE)

`contract-cli` 是合同开放平台的命令行工具，面向人类使用者和 AI Agent 共同设计。它覆盖 profile 配置、user OAuth、app 身份、合同结构化命令、MDM 主数据命令、Agent skills 安装、版本检查、源码构建和 npm/npx 薄包装分发。

[快速开始](#installation--quick-start) · [AI Agent](#quick-start-ai-agent) · [Agent Skills](#agent-skills) · [鉴权](#authentication) · [命令体系](#command-system) · [高级用法](#advanced-usage) · [安全提示](#security--risk-warnings) · [完整命令文档](docs/cli-command-reference.md)

## Why contract-cli?

- **面向合同域收敛**：把合同、模板、文件、分享、协商、交易方、法人主体和字段配置整理成稳定的结构化命令。
- **人和 Agent 都好用**：命令参数保持明确、短路径和可复制示例，AI Agent 可以通过内置 skills 获取业务字段、身份边界和调用约束。
- **双身份支持**：支持 `user` OAuth 与 `app` 应用身份，旧脚本中的 `bot` 身份值继续兼容并按 `app` 处理。
- **输出稳定**：业务命令默认 JSON 输出，支持 `yaml`、`table`、`--raw`，便于人工查看和自动化解析。
- **发布闭环**：提供源码构建、预编译二进制、GitHub Release assets、npm/npx 薄包装和发布前检查脚本。
- **可控默认值**：默认 profile 为 `contract`，默认环境为 `prod`，帮助命令只读本地说明，不读取 profile、不发 HTTP。

## Features

| Category | Capabilities |
| --- | --- |
| 配置与版本 | 初始化 profile、查看版本、检查 npm 远端版本、自动注入 `_notice.update` |
| 鉴权 | `user` OAuth 登录、`appId/appSecret` 应用登录、状态查看、登出、默认身份切换 |
| 合同 | 搜索、详情、创建、提交、重提、更新、删除、文本读取、用户组同步 |
| 合同文件 | 正文/附件上传、文件下载、打印文件生成 |
| 模板与分类 | 分类列表、模板列表、模板详情、模板实例创建、枚举查询 |
| 分享与协商 | 合同分享记录、协商邀请链接、协商操作记录 |
| MDM 主数据 | 交易方列表/详情、法人主体列表/详情、字段配置查询 |
| Agent Skills | `auth`、共享约定、合同命令、交易方、法人主体、字段配置等内置 skills |
| 构建发布 | Go 构建、npm 包、release assets、发布前检查、本地安装验证 |

## Installation & Quick Start

### Test 联调包

正式构建仍仅支持 prod。独立 `1.8.3-test.1` 联调包通过构建开关增加 test，默认环境仍为 prod。

dev 环境支持已移除；新包拒绝旧 dev profile 和 dev 网络地址。test 使用独立配置并重新授权，不迁移 dev Token 或 App Secret。

```bash
# 从源码生成联调包，不发布、不覆盖正式包
bash scripts/build-test-package.sh
npm install -g ./dist/qfeius-contract-cli-1.8.3-test.1.tgz

# 建议使用独立配置目录及 profile，避免改变现有生产默认配置
export CONTRACT_CLI_CONFIG_DIR="$HOME/.contract-cli-test"
contract-cli config add --env test --name contract-test
```

审批矩阵支持 `--profile contract-test --as user` 或 `--as app`。user 复用浏览器授权登录并需具备合同规则管理权限；app 使用本机安全配置的 test 应用凭据。不要在对话中发送 Token 或 App Secret。此包包含双身份变更，调用仍需部署配套后端及网关配置（见下方说明）。

旧 Authorization Code profile 的用户登录命令：`contract-cli auth login --profile contract-test --as user`；Device profile 使用已有 `auth init` → 浏览器授权 → `auth complete`。审批矩阵接口仍走同一规则 OpenAPI 路径，具体网关要求见 [双身份接入说明](docs/approval-matrix-user-app-auth.md)。

API 地址为 `https://test-open.qtech.cn`，账号地址为 `https://test-myaccount.qtech.cn`。同名 profile 切换环境会清理原有认证，需要重新登录；独立 profile 不受影响。

联调包沿用正式包的内置 Agent Skills（其中仍保留生产使用约束），本节 test 操作面向本地 CLI 操作者。安装联调包会替换全局同名命令；需要回到正式版时可重新安装 `1.8.3` 正式包。结束联调后 `unset CONTRACT_CLI_CONFIG_DIR` 恢复默认配置目录。

### Requirements

- Go `1.24.3+`
- Node.js `16+`
- npm / npx
- `tar`，Windows 下可使用 PowerShell `Expand-Archive`
- 从源码构建 release snapshot 时需要 `goreleaser`

### Quick Start (Human Users)

#### Install

选择一种安装方式。

**方式 1：从 npm 安装发布包**

```bash
NPM_CONFIG_REGISTRY=https://registry.npmjs.org npm install -g @qfeius/contract-cli@latest
contract-cli --version
```

预发布版本可以安装 beta tag：

```bash
NPM_CONFIG_REGISTRY=https://registry.npmjs.org npm install -g @qfeius/contract-cli@beta
contract-cli --version
```

**方式 2：从源码构建**

```bash
git clone https://github.com/qfeius/contract-cli.git
cd contract-cli
make test
make build
./contract-cli --version
```

也可以安装到本机 PATH：

```bash
make install
contract-cli --version
```

#### Configure & Use

```bash
# 1. 初始化或更新 profile
contract-cli config add --env prod --name contract

# 2. 本地环境可继续使用旧 user 授权码模式
contract-cli auth login --profile contract --as user

# 3. 豆包 / WorkBuddy 使用 Device Grant：init 立即返回授权信息
contract-cli auth init --profile contract --output json
# 用户完成手机号和企业授权后，只查询一次
contract-cli auth complete --profile contract --output json

# 4. 或登录 app 身份
contract-cli auth login --profile contract --as app --app-id <id> --app-secret <secret>

# 5. 查看授权状态
contract-cli auth status --profile contract --as user
contract-cli auth status --profile contract --as app

# 6. 开始查询
contract-cli contract get <contract-id> --profile contract --as user
contract-cli mdm vendor list --profile contract --as user --page-size 10
```

新脚本请使用 `--as app` 表示应用身份。旧脚本中的 `--as bot` 会继续兼容并按 app 身份执行。

## Quick Start (AI Agent)

以下步骤适合 AI Agent 辅助用户完成安装和验证。涉及浏览器授权时，应把 CLI 输出的授权链接交给用户完成。

**Step 1：安装 CLI**

```bash
NPM_CONFIG_REGISTRY=https://registry.npmjs.org npm install -g @qfeius/contract-cli@latest
contract-cli --version
```

**Step 2：安装 Agent skills**

推荐使用通用 installer：

```bash
npx skills add qfeius/contract-cli -y -g
```

如果通用 installer 不可用，使用 CLI 内置兜底：

```bash
contract-cli skills install
contract-cli skills install --target ~/.codex/skills
contract-cli skills list
```

**Step 3：初始化 profile**

```bash
contract-cli config add --env prod --name contract
```

**Step 4：登录并验证**

```bash
contract-cli auth init --profile contract --output json
# Agent 展示授权信息，用户明确完成授权后：
contract-cli auth complete --profile contract --output json
contract-cli auth status --profile contract --as user
```

WorkBuddy 使用 `qr_code_path` 交付原始 PNG 附件，AgentKit 使用 `qr_code_path`。豆包普通工作任务只展示 `verification_uri_complete` 和 `expires_at_display`，不展示二维码，也不读取或交付二维码文件。

正式包固定使用 `contract` profile 和 `prod` 环境，不会使用历史非生产 profile 发起授权或业务请求。WorkBuddy 更新 Skills 后必须完全退出并重新启动，然后新建任务；已有任务不会热加载新 Skill。

`auth init` 和 `auth complete` 都只请求一次。`complete` 返回 `pending` 时不持续轮询；请用户完成授权后再主动查询。返回 `uncertain`、`denied`、`expired` 或 `restart_required` 时禁止自动重试；用户明确同意重新授权后，才执行 `auth init --profile contract --output json --restart`。

如用户提供应用凭证，也可以配置 app 身份：

```bash
contract-cli auth login --profile contract --as app --app-id <id> --app-secret <secret>
contract-cli auth status --profile contract --as app
```

**Step 5：用只读命令做 smoke check**

```bash
contract-cli --help
contract-cli contract category list --profile contract --as user
contract-cli mdm fields list --profile contract --as user --biz-line vendor
```

## Agent Skills

| Skill | Description |
| --- | --- |
| `auth` | 初始化 profile、user/app 登录、状态查看、登出、身份切换和本地配置排障 |
| `contract-cli-shared` | 在 contract、payment、mdm、event 和 rule 模块间做选择，说明身份边界、请求体输入、输出格式和 profile 规则 |
| `contract-cli-contract` | 合同详情、搜索、创建、字段、签署、授权、电子签、提交、更新、文件、分享、协商和审批命令 |
| `contract-cli-payment` | 付款申请、付款计划和付款记录命令 |
| `contract-cli-mdm-vendor` | 交易方列表、详情、创建、更新、全量分页和按证件查询 |
| `contract-cli-mdm-legal` | 法人主体列表、详情、按编码查询、创建和更新 |
| `contract-cli-mdm-fields` | vendor、legal_entity、vendor_risk 等字段配置查询 |
| `contract-cli-mdm-exchange` | 固定汇率查询和更新 |
| `contract-cli-mdm-file` | 主数据附件下载 |
| `contract-cli-event` | 事件出口 IP 查询 |
| `contract-cli-rule` | 审批矩阵规则表查询、行操作、批量导入计划与执行、预发布和发布 |

推荐安装方式：

```bash
npx skills add qfeius/contract-cli -y -g
```

内置兜底安装：

```bash
contract-cli skills install --target ~/.codex/skills
```

## Authentication

| Command | Description |
| --- | --- |
| `config add` | 初始化或更新 profile，写入开放平台地址、OAuth metadata 和 app token endpoint |
| `auth init` | 发起或恢复 Device Grant；`--restart` 仅在用户明确同意后替换旧授权会话 |
| `auth complete` | 单次查询 Device 授权结果，成功后安全保存 token |
| `auth login --as user` | 走 OAuth 用户授权 |
| `auth login --as app` | 使用 `appId + appSecret` 兑换 app token |
| `auth status` | 查看 user 或 app 授权状态 |
| `auth logout` | 清理指定身份 token；app 登出只清 token，不删除 appId/appSecret |
| `auth use` | 切换 profile 的默认业务身份 |

常用命令：

```bash
contract-cli config add --env prod --name contract
contract-cli auth init --profile contract --output json
contract-cli auth complete --profile contract --output json
contract-cli auth login --profile contract --as user
contract-cli auth login --profile contract --as app --app-id <id> --app-secret <secret>
contract-cli auth status --profile contract --as user
contract-cli auth status --profile contract --as app
contract-cli auth use --profile contract --as app
contract-cli auth logout --profile contract --as user
contract-cli auth logout --profile contract --as app
```

身份规则：

- `config`、`version`、`update check`、`skills list/install` 不需要登录态。
- `contract ...`、`mdm ...` 结构化命令会根据 `--as user|app` 选择对应底层路径。
- 当前大部分 MCP 路径仍是 user-only；显式用 app 调用 user-only 路径会直接报错。
- `contract search-v2`、`contract field update`、`contract sign switch-to-paper`、`contract sign-url get`、`contract form attribute list`、`contract authorization grant`、`contract esign *`、`contract submit/resubmit/patch/download-file/delete/print-file`、`contract share get/batch-create`、`contract cooperation link/record/search/file`、`contract approval start/get`、`payment *`、`mdm vendor create/update/list-all/query-by-cert`、`mdm legal get --code/create/update`、`mdm fixed-exchange-rate get/update`、`mdm file download`、`event outbound-ip list` 当前仅支持 app 身份。
- 兼容旧身份值 `bot`，但新文档和新脚本统一使用 `app`。

## Command System

`contract-cli` 不暴露未开放的 raw API 入口。当前命令体系分三层，优先使用结构化命令和本地帮助。

### 1. 本地帮助与基础命令

帮助只渲染本地命令说明，不读取 profile、不发 HTTP，也不会触发自动版本检查。

```bash
contract-cli --help
contract-cli -h
contract-cli help
contract-cli help contract upload-file
contract-cli contract search --help
contract-cli version
contract-cli update check --channel latest --json
```

### 2. 合同结构化命令

合同域命令覆盖合同、文本、文件、模板、分类、分享与协商。完整矩阵请看 [命令文档](docs/cli-command-reference.md)。

```bash
contract-cli contract search --profile contract --as user --input-file search.json
contract-cli contract search --profile contract --as app --data '{"contract_number":"CN-001"}'
contract-cli contract get <contract-id> --profile contract --as user
contract-cli contract create --profile contract --as app --data '{"contract_name":"demo","create_user_id":"ou_xxx"}'
contract-cli contract upload-file --profile contract --as user --file ./合同正文.docx --file-type text
contract-cli contract submit <contract-id> --profile contract --as app --data '{"comment":"ok"}'
contract-cli contract template list --profile contract --as app --category-number CAT-1 --page-size 20
```

### 3. MDM 主数据命令

MDM 命令覆盖交易方、法人主体和字段配置。

```bash
contract-cli mdm vendor list --profile contract --as user --name 供应商 --page-size 10
contract-cli mdm vendor get <vendor-id> --profile contract --as app --user-id-type employee_id
contract-cli mdm vendor create --profile contract --as app --user-id <operator-user-id> --input-file vendor-create.json
contract-cli mdm legal list --profile contract --as user --name 主体A --page-size 10
contract-cli mdm legal get <legal-entity-id> --profile contract --as app --user-id-type employee_id
contract-cli mdm legal update <legal-entity-id> --profile contract --as app --user-id <operator-user-id> --input-file legal-update.json
contract-cli mdm fields list --profile contract --as user --biz-line vendor
contract-cli mdm fields list --profile contract --as app --biz-line legal_entity
```

## Advanced Usage

### Output Formats

业务命令默认输出 JSON，可通过 `--output` 切换格式。

```bash
contract-cli contract get <contract-id> --profile contract --as user --output json
contract-cli contract get <contract-id> --profile contract --as user --output yaml
contract-cli mdm vendor list --profile contract --as user --output table
```

需要原样响应体时使用 `--raw`：

```bash
contract-cli contract get <contract-id> --profile contract --as user --raw
contract-cli contract download-file <file-id> --profile contract --as app --raw > contract.pdf
```

### Request Body

JSON 请求体统一使用 `--input-file` 或 `--data`，两者互斥。

```bash
contract-cli contract create --profile contract --as app --input-file create.json
contract-cli contract patch <contract-id> --profile contract --as app --data '{"title":"demo"}'
contract-cli contract print-file --profile contract --as app --input-file print-file.json
```

真实二进制文件上传只使用 `--file`，不要把 `--file` 当 JSON 请求体输入。

```bash
contract-cli contract upload-file --profile contract --as user --file ./合同正文.docx --file-type text
contract-cli contract upload-file --profile contract --as app --file ./附件.pdf --file-type attachment --file-name 附件.pdf
```

### Pagination

分页命令通常支持 `--page-size` 和 `--page-token`。

```bash
contract-cli contract search --profile contract --as user --page-size 20
contract-cli contract template list --profile contract --as app --page-size 20 --page-token <token>
contract-cli mdm vendor list --profile contract --as user --page-size 10 --page-token <token>
contract-cli event outbound-ip list --profile contract --as app --page-size 10
```

`event outbound-ip list` 的 `--page-size` 必须在 `10` 到 `50` 之间。

### User Query Parameters

开放平台命令统一预留 `--user-id-type` 和 `--user-id`。

```bash
contract-cli contract get <contract-id> --profile contract --as app --user-id ou_xxx --user-id-type employee_id
contract-cli mdm legal get <legal-entity-id> --profile contract --as app --user-id-type employee_id
```

`mdm vendor create/update` 和 `mdm legal create/update` 会要求 `--user-id`，用于提供当前操作人上下文。

### Update Check

手动检查：

```bash
contract-cli update check
contract-cli update check --channel latest
contract-cli update check --channel beta --json
```

自动提示：

- 普通命令会同步读取本地 `update-check.json`，有可升级缓存时在 JSON object 输出中注入 `_notice.update`。
- cache fresh 时不访问远端；cache 缺失、channel 不匹配或过期时，CLI 会在当前命令内用短超时刷新远端版本缓存。
- 有新版本时，仅在 JSON object 输出中注入 `_notice.update`。
- `--raw`、yaml、table、纯文本命令不注入 `_notice.update`。
- 设置 `CONTRACT_CLI_NO_UPDATE_CHECK=1` 可以关闭自动检查。

## Build, Test & Release

### Build

```bash
make build
./contract-cli --version
```

也可以直接使用：

```bash
./build.sh
go build ./cmd/contract-cli
```

默认会把版本、commit、构建时间和审批矩阵功能基线注入到二进制里。`1.8.4` 及以上版本的构建会检查矩阵扩展命令是否存在，避免版本号升高但命令集回退。

### Test

完整手工测试流程请看 [docs/cli-test-plan.md](docs/cli-test-plan.md)。

```bash
make test
tests/cli_e2e/smoke.sh
make release-check
```

`make release-check` 会额外验证 npm 包 dry-run、本地 tgz 安装、安装后 `contract-cli --version`、`skills list` 和 `skills install`。

### Release Assets

```bash
make release-assets
```

默认读取 `package.json` 的版本号，生成 macOS Intel/Apple Silicon、Windows amd64/arm64、Linux amd64/arm64 六类制品和 `checksums.txt`。npm 安装器必须校验对应 SHA-256，下载、校验或平台不匹配时直接失败，不回退到本地源码编译。这些文件需要上传到同名 GitHub Release，例如 `v1.0.0`。

### Stable Release

```bash
scripts/release.sh --version 1.0.0 --dry-run
scripts/release.sh --version 1.0.0
scripts/release.sh --version 1.0.0 --publish --yes
```

默认模式只更新本地 `package.json`、执行 `make release-check`、生成 `dist/release-assets/`，不会推送 GitHub 或发布 npm。真正发布需要显式传入 `--publish --yes`，脚本会按顺序提交版本、打 `v<version>` tag、推送代码和 tag、创建 GitHub 正式 Release 并上传附件，最后执行 `npm publish --tag latest`。

远端发布需要满足其中一种授权方式：

- GitHub：本机已执行 `gh auth login`，或设置有仓库 `contents:write` 权限的 `GITHUB_TOKEN`
- npm：本机已执行 `npm login`，或设置有发布 `@qfeius/contract-cli` 权限的 `NPM_TOKEN`

发布配置沿用 npmjs 历史口径：`https://registry.npmjs.org/` 与 public access。

仓库里额外提供了一个可选的 GitHub Actions workflow：`.github/workflows/release.yml`。如果后续继续使用 GitLab CI，可以直接复用相同的 `goreleaser release --clean` 命令。

## Security & Risk Warnings

`contract-cli` 可以被 AI Agent 调用来操作合同开放平台。授权后，Agent 可能在你的 user 或 app 身份下读取、创建、更新、提交、下载合同相关数据。请在使用前确认命令、profile、身份和输入文件来源。

安全建议：

- 不要把 app secret、OAuth token、`.npmrc`、合同正文、附件或包含个人信息的 JSON 请求体提交到 Git。
- 不要让未知 Agent 自动执行写操作；提交、更新、删除、下载、打印文件等命令应先人工确认。
- 使用 `--input-file` 时先检查 JSON 文件内容，避免 prompt injection 或误传生产数据。
- 在 CI、远程主机或无 GUI 环境下载文件时，优先显式传 `--output-file`，避免保存位置不明确。
- Agent 输出中如包含合同编号、交易方名称、法人主体或用户标识，应按内部数据安全要求处理。
- 默认帮助命令和 `skills list/install` 不需要登录态，适合作为低风险检查入口。

使用本工具即表示你理解并接受相关开放平台权限和数据操作风险。

## Project Layout

- `cmd/contract-cli`：CLI 入口
- `internal/cli`：命令解析与交互
- `internal/openplatform`：开放平台统一 client 和领域 service
- `internal/oauth`：user / app 鉴权逻辑
- `internal/build`：版本与构建元信息
- `skills`：随 CLI 分发并可由通用 installer 安装的 Agent skills
- `scripts`：npm 安装、运行和发布脚本
- `tests/cli_e2e`：CLI 端到端冒烟脚本
- `docs`：命令参考、测试计划、设计记录和 AI 变更记录

## License

当前仓库以 `UNLICENSED` 方式提供，后续若需要对外发布，请在首发前补齐正式许可证与发布源配置。
