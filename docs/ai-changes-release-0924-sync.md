# release-0924 同步 release 变更记录

## 2026-10-15

- 将 `upstream/release` 合入 `release-0924`，处理 README、CLI 测试及合同相关 Skill 的 9 处文件冲突。
- 保留 `release-0924` 的合同摘要命令与 `zhishu-contract-summary` Skill，同时接入 `release` 的智能审查命令及两个审查 Skill；在 README 能力地图、身份速查和命令示例中补充合同摘要。
- 合并生产环境与旧凭证隔离说明及其测试断言；统一 Skill 文案，并补齐摘要 Skill 的 agent metadata。同步后的内置 Skill 数为 17，已更新对应测试断言。
- 验证：`go test ./...` 全部通过；Node 安装器、Skill 合并、打包相关测试通过；本地 npm 测试包升级及 Skill 同步集成测试通过。测试使用隔离临时目录和缓存，未操作真实客户端的 Skill 目录。
