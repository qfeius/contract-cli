# Changelog

## Unreleased

- 审批矩阵导入 `resume` 在解除暂停后持久化 `ready` 状态并清除旧 reason，核验中计划仍保持阻断；规则行搜索的成功空结果稳定输出空行列表。
- `skills install` 支持 `--name` 定向更新单个内置 Skill，帮助与规则 Skill 同步说明 `pause/resume/verify` 流程，避免本地旧 Skill 滞留。
- contract 产品审批矩阵在发请求前阻断新建规则组及向非 `approve_matrix` 组新建矩阵；保留其他产品的建组能力及已有矩阵的操作入口。
- 环境能力收敛为仅 `prod`：移除 test/blue 配置入口与构建开关，旧非生产 profile 和域名在请求前阻断；本地预发布包的 `-test` 仅为版本标记。
- 审批矩阵 Skill 增加业务 Excel 导入前的用途识别门禁；用途不明确时先询问，协商自动邀请场景先补齐 COLLECTION 类型的 `合同类型` 条件列再生成导入计划。
- 审批矩阵 `rule symbol query` 改为使用 CLI 内置运算符映射，不再请求 `/symbols/query`。
- 审批矩阵部门值统一使用 `open_department_id`（`od-...`）；行操作和批量导入会在本地拒绝 `sys_department.id` 数字，避免下游 500。
- 审批矩阵部门目录明确返回并使用 `open_department_id`：搜索结果可直接写入，数字 `department_id` 仅用于 batch-get 回查转换；命令帮助、Skill 和测试场景统一该约定。
- 审批矩阵发布基准按无序集合比较 `department_collection`，部门成员仅返回顺序变化时继续发布，成员真实增删仍阻断；读取时兼容旧版本未排序的本地基准。
- 审批矩阵修复 profile 重建后的发布恢复死锁：服务端已为待发布状态时，`pre-release` 双读版本与规则行后只恢复本地基准，不再要求通过原值重写业务行重置状态。
- 审批矩阵新增列增加总列数预检：总上限 12，优先级与备注占 2 列，条件列与结果列共用 10 个名额；满额时在 POST 前本地阻断并输出分类计数，Skill 会在完整方案首次写入前识别超限。
- 版本输出和 `rule` 帮助增加审批矩阵功能基线；`1.8.4` 及以上构建校验新增矩阵命令，避免高版本产物缺少低版本已具备的能力。
- 新增 `internal/build`，支持 `contract-cli version` 与 `--version`
- 新增 `build.sh`、`Makefile`、`.goreleaser.yml`
- 新增 npm/npx 薄包装：`package.json`、`scripts/install.js`、`scripts/run.js`
- 新增 `tests/cli_e2e/smoke.sh` 作为发布前冒烟脚本

## 0.1.0

- 初始化 `contract-cli` 构建、发布与分发脚手架
