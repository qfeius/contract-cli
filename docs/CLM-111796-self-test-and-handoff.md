# CLM-111796：开发回归与交付检查

## 执行环境

需要 Go、Node.js 和 npm。真实 npm 安装测试会触发 Skill 覆盖，使用专用系统账户、虚拟机或测试电脑；只设置配置目录、CODEX_HOME 或 npm prefix 不能隔离 Windows WorkBuddy 默认历史目录。不要在日常账户无备份运行安装测试。

## 常规回归

```text
go test ./internal/skillsync
go test ./internal/cli -run "^(TestSkills|TestSkillSync|TestCompleteInstall|TestSelectedInstall|TestUpgradeCandidates|TestSkillInstallGuidance|TestSkillResultSummary)"
npm run test:installer
```

全仓库检查使用 go test ./...。失败须记录原因，并用独立原始基线区分已有问题和新增回归；不能把专项通过写成全仓库通过。

## 真实安装与升级

在专用环境运行 npm run test:skill-sync-install。设置 CONTRACT_CLI_BASELINE_ROOT 指向独立原始代码副本，可增加旧代码迁移验证。测试因环境保护而跳过时，不算通过。

通用包构建见 scripts/build-local-test-package.js；版本、输出位置及平台矩阵由调用者指定。历史来源核验与指纹维护见 skill-history-fingerprint-provenance.md，包级 Linux 验收见 scripts/verify-skill-guidance-packages.js 和 scripts/verify-history-provenance.js。

## 提交与交付

- 保留功能源码、回归测试、通用构建/验收工具、指纹来源说明及需求变更记录。
- 本机固定账户、固定旧包的阶段工具与日志在仓库外归档，不纳入提交。归档位置见需求变更记录。
- dist 中的包、校验文件和原始证据不提交。个人 profile、Token、App Secret 不进入包或仓库。
- 整合目标分支最新代码后回归，从确定提交构建最终验收包，核对版本、提交号、平台、校验值与手册附件一致。
- CLI 安装、各位置 Skill 同步、客户端加载及业务授权分别验证；测试结果应标明通过、失败、跳过和未验证项。
