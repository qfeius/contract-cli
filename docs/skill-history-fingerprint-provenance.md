# 历史 Skill 指纹来源与维护

本次只支持自动识别官方 1.9.3、1.9.4 安装出的未修改内容。不把目录名、版本字段或客户端目录存在当作来源证明，也不承诺覆盖所有历史版本。

## 来源核验（2026-10-10）

使用 npm 官方源 `https://registry.npmjs.org` 查询指定版本的 `dist.integrity`，与本地原始 tgz 的 SHA-512 一致：

- 1.9.3：`sha512-A796+NtAvTN3Uz2V+snVR1du+3lwWc5kMtu2jpYJIVrWlwx7PJNAPJbcTEWHYL4GAPhyEr84B3g0IHP+NON8cQ==`
- 1.9.4：`sha512-wLCdMTOS1NN19cKmawA86yE4us/rMA6hcpHptmjYk5MUBKyxX0xyIujPgvgXFlkx4iqZbRVybo/xH62Sz6pOvg==`

从这些包解出 Windows amd64 程序，分别执行 `skills install --target <独立取样目录> --force`，取得每版 14 个 Skill。未执行 npm 全局安装，未接触日常客户端目录。Linux 原始发布程序另作端到端安装迁移验收。

## 生成与检查

`node scripts/skill-history-fingerprints.js <1.9.3 安装出的 skills 目录> <1.9.4 安装出的 skills 目录>` 只读输入并输出 JSON。核验输出后更新 `internal/skillsync/history_fingerprints.json`，不在用户安装时联网下载指纹，也不从用户现场文件生成可信指纹。

每个 Skill 的指纹包含全部相对文件路径、文件内容 SHA-256 和目录项；不忽略 references、额外文件、空目录，也不归一化版本字段或换行符。符号链接和非普通文件不作为可自动接管的证据。文件权限不参与内容指纹。

一个位置中，本轮所有即将覆盖的已有官方同名目录均须匹配可信指纹；有任一冲突则整个位置不自动覆盖、不新增登记。未知旧版本、被修改的无登记内容、非候选自定义位置，继续使用原有显式完整安装入口确认。已登记且标记匹配的位置仍覆盖用户修改。

指纹证明内容一致，不证明安装者身份；复制自正式包且内容完全一致的副本会匹配。旧版独有且不在当前内置名单中的 Skill，不因本次指纹匹配而猜测删除。
