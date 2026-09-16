# Changelog

## Unreleased

- 审批矩阵 `rule symbol query` 改为使用 CLI 内置运算符映射，不再请求 `/symbols/query`。
- 新增 `internal/build`，支持 `contract-cli version` 与 `--version`
- 新增 `build.sh`、`Makefile`、`.goreleaser.yml`
- 新增 npm/npx 薄包装：`package.json`、`scripts/install.js`、`scripts/run.js`
- 新增 `tests/cli_e2e/smoke.sh` 作为发布前冒烟脚本

## 0.1.0

- 初始化 `contract-cli` 构建、发布与分发脚手架
