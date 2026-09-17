# npm Trusted Publishing 配置

`contract-cli` 使用 GitHub Actions OIDC 发布 `@qfeius/contract-cli`，不需要在 GitHub Secrets 中保存 `NPM_TOKEN`。

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

- Tag `v1.2.3-beta.1` 发布到 npm `beta`。
- `dev-p1` 的 OIDC job 只处理带预发布标识的 Tag；正式版本仍由 `release` 分支发布。
- GitHub Release 成功后才执行 npm 发布。
- 发布前会检查 npm 当前 dist-tag，禁止把 `beta` 指向更旧或相同的版本。
- npm 发布 job 只允许在 `qfeius/contract-cli` 运行；fork 仓库打 Tag 时会跳过该 job。
- `package.json` 中的 `repository.url` 必须继续保持为 `git+https://github.com/qfeius/contract-cli.git`。
