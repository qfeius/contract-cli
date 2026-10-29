# 已选合同父组定位修复与验证

> 历史验证记录：2026-10-22 同步 GitHub 1.9.9 后，运行规则以 `skills/contract-handover/references/assembly-lookup.md` 为准；下文保留旧配方的研发证据。

日期：2026-10-21。基线：contract-cli `codex/contract-handover-business-skill@ca186c01dbd50ce5fdc10b19cfc2aef3eb9c23bf`。

## 问题与修复边界

`b8cc339` 将合同定位委托已有搜索 Skill，并删除页面搜索配方；候选缺少 assemblyId 时没有等价的父组映射来源，通用详情步骤又可能把 type=11 成员送入普通 contract/info。旧契约已明确成员可从搜索取得 assemblyId，随后通过父组详情和 childrenInfo 读取，无需先调普通详情。

本次保留通用搜索复用，只补齐选中对象的父组关系。优先复用语义明确的现有映射，包括实际返回的 v2.1 Presentation；缺少时通过原有页面搜索按已选编号／名称作只读补充查询，按同一内部合同 ID 绑定，随后从父组详情和完整成员列表重新核实。查询不扩大原筛选和选择，不构成整组写入授权。

没有新增专用 CLI 命令，没有修改后端、通用调用器、其他 Skill 或当前分支的环境和交接写入规则。白名单需求由 8 个端点增加为 9 个，新增项为原有 `POST /open-web/web/clm/es/search/contractList`；本仓修改不代表网关已放行该路径。

## 源码证据

本地 CLM：`git@git.qtech.cn:limengchao/clm.git`，ref `20261020-lyy-cli`，commit `c651c0621faa6d1a102cdfdfccd64a6efb968c3d`。只读核对，没有修改该仓库。

- `clm-api/.../controller/SearchController.java:54-68`：页面搜索 Controller，从当前登录上下文取得租户和员工，按既有权限路由查询。
- `clm-common/.../vo/es/SearchContractListItemVO.java:170-172,403`：搜索条目含 assemblyId，按字符串序列化并从合同实体映射。
- `clm-openapi/.../response/vo/McpContractSearchPresentationV2_1VO.java:275-323`：实体分别携带 groupId、assemblyId、contractId、childContractId、businessTypeCode；展示 PARENT 不等于合同组父组类型。
- 历史 `e51d449` 的 API 契约 search_contracts.read_only_number_lookup.execution 明确按业务类型分流，不强制组成员走普通详情；现保留在研发归档。

以上为静态代码证据，不代表目标环境已部署对应版本、已启用 v2.1 或已放行页面查询。

## TDD 与自动检查

1. 先新增安装分发、受限查询示例和成员映射传输回归；缺少 assembly-lookup.md 时三项按预期失败。
2. 补齐运行参考、分流与白名单后，交接回归通过。查询示例经实际 CLI 通用调用器和本地 HTTP 替身验证，保留仅成员响应中的两个不同 Long ID 字符串；不伪装为 Agent 编排或真实业务验收。
3. `TMPDIR=/private/tmp go test ./...` 全量通过。默认 macOS 临时目录经符号链接解析后，既有安装测试的路径比较失败；使用实际规范目录重跑，不修改或跳过这些测试。
4. `golangci-lint run --enable=exhaustive ./...`：0 issues。
5. `node --test scripts/install.test.js scripts/setup.test.js tests/release/test-package.test.js`：17 通过，1 个已有平台相关用例跳过，0 失败。
6. 官方 Skill 格式检查通过；系统 Python 缺少 PyYAML，使用 uv 隔离依赖运行检查，不改变业务运行依赖。

## 独立离线行为验证

独立 Agent 仅收到现有 Skill、四项用户任务、初始搜索响应及只读调用替身；不提供预期答案，不读取替身源码，不使用真实网络或凭证。替身记录请求，主 Agent 再核对路径、次数和父组 ID。所有业务响应均为合成数据，不属于真实环境成功证据。

| 场景 | 实际调用 | 结果 |
|---|---|---|
| 旧搜索只返回 type=11 成员，无父组映射；补充响应有同编号无关合同且无父组行 | 补充查询 1 次、父组详情 2 次、成员页 0/1 各 1 次 | 精确绑定原选成员，忽略无关同编号命中，取得真实父组和两名成员；未调用普通详情 |
| v2.1 选中展示父行实际为 type=11，已有 assembly_id | 父组详情 2 次、成员页 0/1 各 1 次 | 按 row_ref、内部 ID 与类型绑定；没有额外补充查询，没有将展示 PARENT 改成 type=10 |
| 同一已选内部 ID 出现两个不同 assemblyId | 补充查询 1 次 | 停止，不尝试任一父组，不发交接 |
| 普通独立合同仅查询当前角色与历史 | 普通详情 1 次 | 沿用普通路径，未增加父组定位；空历史仅解释为接口未返回记录 |

合计 11 次只读调用；无交接写入。两个整组场景均分页至空页并回读父组计数，前后成员总数一致。未验证真实环境网关放行、权限、交接写入或通知等副作用；现有交接确认及逐项回查要求继续生效。
