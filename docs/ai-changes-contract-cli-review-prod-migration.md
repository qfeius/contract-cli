# AI 变更摘要：contract-cli review prod 迁移

## 范围

把源分支 `origin/blue-feature-contract-cli-add-review` 在起始提交 `2dcc0cf9cd16fe0ae42490299fa784e2b2708731` 中的合同审查功能迁移到目标分支 `origin-github/feature-contract-cli-add-review`，目标起始基线为 `b3a86ad10b3a12d1eb188be239d7e58da38079ac`。仅迁移审查功能及其直接需要的 CLI、Skill、元数据和测试支持。

## 功能变化

- 增加 `contract-cli review` 命令，覆盖本地/URL 文件上传、合同主体提取、协商信息读取、审查任务分步操作与组合式执行，以及审查清单、规则分组和规则管理。
- 复用现有 OpenPlatform、user 授权和输出路径；保留输入校验、预览/干运行、写入保护、上传大小限制与不自动重复发起等行为。
- 增加审查和审查规则 Skills，并更新合同命令的审查路由、Agent 元数据和公开 Skill 结构说明。

## prod 配置保护与 Blue 清理

用户确认目标生产构建只支持 prod，Blue profile 与 Blue host 均拒绝。源分支的以下 Blue 环境支持没有迁入：

- `internal/cli/app.go` 中将默认环境设为 Blue、添加 Blue preset 与 endpoint 参数的变更。
- 新增的 `internal/cli/environment_validation.go` 中识别并放行 Blue 环境/host 的通用逻辑。
- `internal/cli/device_auth_command.go`、`device_profile.go`、`device_runtime.go`、`device_token.go` 等将生产专用验证切换为多环境验证的改动；Blue profile 恢复提示也未迁入。
- README、CLI 命令参考、auth/shared/contract Skills 中推荐 Blue profile 或 Blue 授权的说明。目标现有 prod 说明保留；shared/contract Skills 只增加审查能力路由。
- 源分支删除 prod 专用验证器并改成允许 Blue 的相关测试/fixture 变更。目标的 prod 验证器、prod 测试和测试 fixture 均保留。

目标起始 `internal/cli/prod_environment.go` blob 为 `b254a3399e6416147c14e9fa601d8bb59399e990`，迁移后该文件内容未变；`app.go` 中 `resolveEnvironment` 整个函数未变，默认环境仍为 prod。prod preset 字段和值及生产网络守卫语义沿用目标起始版本。

## 验证

- `npm test`：Skill 元数据/路由 Node 检查 6 项通过；`go test ./...` 全部通过。
- 另行运行的审查 CLI 定向测试、目标 prod profile/host 拒绝测试和 `git diff --check` 均通过。
- 未调用生产业务 API；未执行 GitLab MR、部署或发布。
