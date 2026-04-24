# contract-cli

`contract-cli` 是合同开放平台的命令行工具，支持：

- profile 配置与 OAuth / bot 双身份登录
- 合同与 MDM 结构化命令
- Agent skills 通用安装与 CLI 内置兜底安装
- 版本检查与升级提示
- 源码构建、预编译二进制发布、npm/npx 薄包装分发

命令清单请看：[docs/cli-command-reference.md](/Users/lyy/contract-cli/docs/cli-command-reference.md)

## 环境要求

- Go `1.24.3+`
- Node.js `16+`
- `tar` 或 PowerShell `Expand-Archive`

## 快速开始

### 源码构建

```bash
make test
make build
./contract-cli --version
```

也可以直接使用：

```bash
./build.sh
go build ./cmd/contract-cli
```

### 安装到本机 PATH

```bash
make install
contract-cli --version
```

### npm / npx 薄包装

仓库已经提供 `package.json + scripts/install.js + scripts/run.js`：

- 本地源码仓库内执行 `npm install` 时，如果检测到 Go 源码，会回退到本地 `go build`
- 以后发布到 npm 后，安装脚本会优先下载预编译二进制
- npm 发布配置固定为 `https://registry.npmjs.org/` 和 public access
- 预编译二进制默认从 GitHub Releases 下载：`https://github.com/qfeius/contract-cli/releases/download/v{version}`

示例：

```bash
NPM_CONFIG_REGISTRY=https://registry.npmjs.org npm install -g @qfeius/contract-cli@beta
npx skills add qfeius/contract-cli -y -g
npx @qfeius/contract-cli --version
```

`npx skills add qfeius/contract-cli -y -g` 是推荐的 Agent skills 安装方式，会从 GitHub 仓库安装 `skills/` 目录，适配 Codex、Cursor、Trae、Claude Code 等多类 Agent 环境。若该通用安装器不可用，可使用 CLI 内置兜底：

```bash
contract-cli skills install
contract-cli skills install --target ~/.codex/skills
```

## 构建与发布

### 本地构建

```bash
make build
```

默认会把版本、commit、构建时间注入到二进制里。

### 本地快照发版

```bash
make release-snapshot
```

该命令依赖 `goreleaser`，会在 `dist/` 下产出多平台压缩包和 `checksums.txt`。

### 生成 GitHub Release 附件

```bash
make release-assets
```

默认会读取 `package.json` 的版本号，生成 `dist/release-assets/contract-cli-<version>-<os>-<arch>` 系列文件和 `checksums.txt`。这些文件需要上传到同名 GitHub Release，例如 `v0.1.0-beta.1`。

### 正式发版

- 打 tag，例如 `v0.1.0`
- 运行 `make release-assets`
- 将 `dist/release-assets/` 下的压缩包上传到 GitHub Release
- 发布 npm 薄包装

### Beta 一键发布脚本

仓库提供安全模式发布脚本：

```bash
scripts/release-beta.sh --version 0.1.1-beta.1 --dry-run
scripts/release-beta.sh --version 0.1.1-beta.1
scripts/release-beta.sh --version 0.1.1-beta.1 --publish --yes
```

默认模式只更新本地 `package.json`、执行 `make release-check`、生成 `dist/release-assets/`，不会推送 GitHub 或发布 npm。真正发布需要显式传入 `--publish --yes`，脚本会按顺序提交版本、打 `v<version>` tag、推送代码和 tag、创建 GitHub pre-release 并上传附件，最后执行 `npm publish --tag beta`。

远端发布需要满足其中一种授权方式：

- GitHub：本机已执行 `gh auth login`，或设置有仓库 `contents:write` 权限的 `GITHUB_TOKEN`
- npm：本机已执行 `npm login`，或设置有发布 `@qfeius/contract-cli` 权限的 `NPM_TOKEN`

仓库里额外提供了一个可选的 GitHub Actions workflow：`/.github/workflows/release.yml`。如果后续继续使用 GitLab CI，可以直接复用相同的 `goreleaser release --clean` 命令。

## 常用命令

```bash
contract-cli --help
contract-cli help contract upload-file
contract-cli config add --env dev --name contract-group
contract-cli auth login --profile contract-group --as user
contract-cli auth login --profile contract-group --as bot --app-id <id> --app-secret <secret>
contract-cli update check --channel beta
npx skills add qfeius/contract-cli -y -g
contract-cli skills list
contract-cli skills install --target ~/.codex/skills
contract-cli contract get <contract-id> --profile contract-group --as user
contract-cli contract upload-file --file ./合同正文.docx --file-type text --profile contract-group --as bot
contract-cli mdm vendor list --profile contract-group --as user
contract-cli mdm legal get <legal-entity-id> --profile contract-group --as user
contract-cli mdm fields list --biz-line vendor --profile contract-group --as user
```

所有已支持命令都可以通过 `--help` 查看本地帮助，例如 `contract-cli contract search --help`。帮助只渲染本地命令说明，不读取 profile、不发 HTTP，也不会触发自动版本检查。

## 测试

完整手工测试流程请看：[docs/cli-test-plan.md](/Users/lyy/contract-cli/docs/cli-test-plan.md)

```bash
make test
tests/cli_e2e/smoke.sh
make release-check
```

`make release-check` 会额外验证 npm 包 dry-run、本地 tgz 安装、安装后 `contract-cli --version`、`skills list` 和 `skills install`。

CLI 会在交互终端下最多每 30 分钟自动检查一次 npm 远端版本，并在发现新版本时提示升级命令。也可以手动执行 `contract-cli update check --channel beta`；如需关闭自动检查，可设置 `CONTRACT_CLI_NO_UPDATE_CHECK=1`。

## 目录说明

- `cmd/contract-cli`：CLI 入口
- `internal/cli`：命令解析与交互
- `internal/openplatform`：开放平台统一 client 和领域 service
- `internal/oauth`：user / bot 鉴权逻辑
- `internal/build`：版本与构建元信息
- `skills`：随 CLI 分发并可由通用 installer 安装的 Agent skills
- `scripts`：npm 安装与运行脚本
- `tests/cli_e2e`：CLI 端到端冒烟脚本

## License

当前仓库以 `UNLICENSED` 方式提供，后续若需要对外发布，请在首发前补齐正式许可证与发布源配置。
