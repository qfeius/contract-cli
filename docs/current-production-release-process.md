# Contract CLI 发布流程

正式发布和预发布统一由 `.github/workflows/release.yml` 执行。本地不再上传 GitHub Release 或执行 npm 发布。

## 本地准备

1. 在开发分支完成修改，运行 `make release-check` 和 `golangci-lint run --enable=exhaustive ./...`。
2. 通过 PR 合入 GitHub `qfeius/contract-cli` 的 `release` 分支。
3. 确认目标提交后，创建并推送版本标签，例如 `v1.10.0`、`v1.10.0-beta.1` 或 `v1.10.0-rc.1`。

也可在 GitHub Release 页面选择 `release` 的目标提交，创建新的版本 Tag 并发布 Release；新 Tag 的 push 会触发 Actions。提前存在的可修改 Release 由 GoReleaser 复用并上传附件，不需要先删除空 Release。对已有 Tag 单独发布 Release 不会产生新的 Tag push。

`make release-assets`、`scripts/build-test-package.sh` 和本地验证包构建能力继续保留，它们不发布正式版本。

## Actions 检查与发布

- PR 和 release 分支推送运行 Linux/macOS 全量测试、安装/打包检查和固定版本 lint；Windows 运行无弹窗安全回归及含特殊字符路径的原生解压、npm 安装检查。
- 只有主仓库的标签推送可以进入发布任务，且必须等待所有检查成功。
- 标签严格采用 `v` 前缀的语义版本；正式版本发布到 npm `latest`，所有合法预发布版本发布到 `beta`，GitHub Release 同时标记 prerelease。
- 从标签推导版本号并更新暂存包元信息，不依赖源码 package.json 的开发版本号。
- 恢复旧版 Tag 发布顺序：先由 GoReleaser 构建并发布 GitHub Release，再准备 npm 包、执行六平台资产构建并发布 npm；GoReleaser 在修改 package.json 前运行。
- GoReleaser 固定为上一次成功发布使用的 v2.20.1，保留已有 Release 说明，不覆盖同名附件；不可修改的 Release 或附件冲突仍会报错，npm 不会继续发布。
- 发布权限仅授予 publish job，PR 检查没有发布权限。
- 部分发布失败应先核对 GitHub 附件及 npm 状态，不盲目重跑覆盖，也不移动已有标签。旧 Tag 的失败任务重跑仍使用旧提交中的 workflow；修复合入 release 不会自动更新旧任务。

## 验证边界

安装测试自行隔离 HOME、CODEX_HOME、CONTRACT_CLI_CONFIG_DIR，不修改真实用户 Skill 或登录配置。
本地检查通过不代表远端 Actions 已运行；真正发布以 Actions 结果、GitHub 附件和 npm dist-tag 为准。
