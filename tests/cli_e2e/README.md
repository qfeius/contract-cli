# CLI E2E

这里放发布前的端到端冒烟脚本。

当前提供：

- `smoke.sh`：源码构建 + `--version` + 基础 usage 验证
- `../release/package-dry-run.sh`：npm 包 dry-run + 包内容校验
- `../release/local-install.sh`：npm tgz 本地全局安装 + skills 安装校验

建议在本地发版前或 CI 的 release job 中执行一次。
