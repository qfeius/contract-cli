# contract-cli

`contract-cli` 是合同开放平台的命令行工具，支持：

- profile 配置与 OAuth / bot 双身份登录
- 开放平台 `/open-apis/...` 原始调用
- 合同与 MDM 结构化命令
- 源码构建、预编译二进制发布、npm/npx 薄包装分发

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
- 发布前只需要把 `package.json` 里的 `config.downloadBaseURLTemplate` 改成真实发布地址模板，或在安装时通过环境变量覆盖

示例：

```bash
CONTRACT_CLI_DOWNLOAD_BASE_URL_TEMPLATE="https://downloads.example.com/contract-cli/v{version}" npm install
npx contract-cli --version
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

### 正式发版

- 打 tag，例如 `v0.1.0`
- 运行 `goreleaser release --clean`
- 将生成的压缩包上传到你的发布源
- 更新 npm 包版本并发布薄包装

仓库里额外提供了一个可选的 GitHub Actions workflow：`/.github/workflows/release.yml`。如果后续继续使用 GitLab CI，可以直接复用相同的 `goreleaser release --clean` 命令。

## 常用命令

```bash
contract-cli config add --env dev --name contract-group
contract-cli auth login --profile contract-group --as user
contract-cli auth login --profile contract-group --as bot --app-id <id> --app-secret <secret>
contract-cli api call GET /open-apis/contract/v1/mcp/config/config_list --as user
contract-cli contract get <contract-id> --profile contract-group --as user
contract-cli mdm vendor list --profile contract-group --as user
contract-cli mdm legal get <legal-entity-id> --profile contract-group --as user
contract-cli mdm fields list --biz-line vendor --profile contract-group --as user
```

## 测试

```bash
make test
tests/cli_e2e/smoke.sh
```

## 目录说明

- `cmd/contract-cli`：CLI 入口
- `internal/cli`：命令解析与交互
- `internal/openplatform`：开放平台统一 client 和领域 service
- `internal/oauth`：user / bot 鉴权逻辑
- `internal/build`：版本与构建元信息
- `scripts`：npm 安装与运行脚本
- `tests/cli_e2e`：CLI 端到端冒烟脚本

## License

当前仓库以 `UNLICENSED` 方式提供，后续若需要对外发布，请在首发前补齐正式许可证与发布源配置。
