# Changelog

## Unreleased

- 审批矩阵 `rule symbol query` 改为使用 CLI 内置运算符映射，不再请求 `/symbols/query`。
- 审批矩阵部门值统一使用 `open_department_id`（`od-...`）；行操作和批量导入会在本地拒绝 `sys_department.id` 数字，避免下游 500。
- 版本输出和 `rule` 帮助增加审批矩阵功能基线；`1.8.4` 及以上构建校验新增矩阵命令，避免高版本产物缺少低版本已具备的能力。
- 新增 `internal/build`，支持 `contract-cli version` 与 `--version`
- 新增 `build.sh`、`Makefile`、`.goreleaser.yml`
- 新增 npm/npx 薄包装：`package.json`、`scripts/install.js`、`scripts/run.js`
- 新增 `tests/cli_e2e/smoke.sh` 作为发布前冒烟脚本

## 0.1.0

- 初始化 `contract-cli` 构建、发布与分发脚手架
