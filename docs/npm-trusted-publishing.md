# npm Trusted Publishing 配置

`contract-cli` 的 GitHub Actions 使用 npm Trusted Publishing OIDC 发布 `@qfeius/contract-cli`，不需要在 GitHub Secrets 中保存 `NPM_TOKEN`。

## npm 配置

在 npm 包 `@qfeius/contract-cli` 的 **Settings -> Trusted publishing** 中添加 GitHub Actions publisher：

| 配置项 | 值 |
| --- | --- |
| Organization or user | `qfeius` |
| Repository | `contract-cli` |
| Workflow filename | `release.yml` |
| Environment name | 留空 |
| Allowed actions | 允许 `npm publish` |

也可以使用 npm CLI 配置：

```bash
npm trust github @qfeius/contract-cli \
  --repo qfeius/contract-cli \
  --file release.yml \
  --allow-publish
```

配置操作需要 npm 包管理权限，并可能要求完成二次验证。

## 发布规则

- 正式 Tag，例如 `v1.8.6`，发布到 npm `latest`。
- 所有合法预发布 Tag，例如 `v1.8.6-beta.1`、`v1.8.6-rc.1`，发布到 npm `beta`，不修改 `latest`。
- Tag 必须是 `v` 前缀的合法语义版本；发布前须通过 Linux/macOS 检查和 Windows 安全回归。
- GitHub Release 和发布资产构建成功后，才会执行 npm 发布。
- 发布任务只允许在 `qfeius/contract-cli` 运行；fork 仓库打 Tag 时会跳过发布。
- `package.json` 中的 `repository.url` 必须保持为 `git+https://github.com/qfeius/contract-cli.git`，以通过 provenance 校验。
- GitHub Actions 是唯一发布入口；本地只做验证和构建，具体步骤见[发布流程](current-production-release-process.md)。
