# Story6-3 CLI 创建合同时填写所属项目

## 2026-11-20

- 从 GitHub 主空间 `main-github/release` 的 `2eabbbc` 建立本地 `feature-contract-project-admin`。先完善创建指南、字段树，再接入项目查询。
- `contract project search` 支持 `--as app` 和 `--as user`。应用走既有 `POST /open-apis/contract/v1/projects/search`，用户走 `POST /open-apis/contract/v1/mcp/projects/search`。请求只使用项目名称、零基页码和页大小，服务器按当前凭证过滤可见项目。CLI 保留原分页响应和字符串项目 ID。
- 指南说明使用与创建相同的身份取得 `data.content[].project_id`，填入普通合同或补充协议的 `project_changes.add_project_ids`。合同创建命令继续原样透传 JSON。
- 两种身份的命令测试先实际失败于未知 `project` 子命令。实现后验证方法、路由、Bearer、分页、长字符串 ID 以及错误参数不发 HTTP；最后 `go test ./...` 通过。Mac 默认临时目录的符号链接会影响既有安装测试的路径断言，验证时将 `TMPDIR` 指向规范物理路径。
- 发布时先确保网关个人凭证检查和 CLM 查询接口已部署，再发布使用新接口的 CLI。本地测试使用模拟响应，真实网关联调和安装包发布待执行。

### CLI 所属项目候选列表

- `contract project search --output table` 参考关联合同候选列表，按服务端顺序逐项展示项目名称、状态、所属部门、项目周期和项目 ID。表格展示当前页与总数，空页显示“本页无项目”，缺失的展示字段用横线表示。
- 仅将成功项目页投影为表格。默认 JSON、YAML、原始响应及两种身份各自的业务失败输出继续沿用既有处理。创建指南补充按人可读的候选信息选择项目，使用同一身份回填真实字符串 ID。
