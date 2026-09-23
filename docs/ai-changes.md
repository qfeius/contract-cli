# AI 变更记录

- 2026-09-23
  变更摘要：当前分支移除 test/blue 环境运行能力，源码与本地预发布包统一只支持 prod。
  涉及文件/模块：环境预设与传输拦截、构建脚本、回归测试、README、命令文档及 auth/shared Skill。
  关键逻辑/决策：非 prod 配置和历史 profile 在请求前拒绝；不迁移、不复用旧凭据。历史记录中的 test/blue 联调方案保留为当时背景，不代表当前可用能力。

- 2026-09-21
  变更摘要：将合同搜索 Skill 调整为“页面默认关键词＋实体/字段引导”的混合路由。
  涉及文件/模块：搜索主 Skill、openai.yaml、全部关键词与引导参考、路由回归测试。
  关键逻辑/决策：显式关键词和无实体依据的普通词直接查页面默认十二字段；明确角色走精确筛选，强推测先查候选或字段元数据再引导。
  关键逻辑/决策：“搜索毛鹏”“查华东”默认关键词；“毛鹏的合同”保留人员引导；“项目区域为华东”先发现字段。
  页面映射：区分 UI 业务分类与十二个实际请求单元，不把默认人员关键词字段交付为申请人精确筛选。
  验证：新增路由回归先失败后通过；Skill 官方格式校验通过；internal/cli 测试通过；golangci-lint 0 issues。
  交付：重建未发布的 1.9.2-beta.1 本地包，release-check、六平台资产、双层校验和及隔离安装通过。
  本机同步：从重建包向 Codex/agents 两处安装 14 个 1.9.2-beta.1 Skill；搜索 Skill 含最新混合路由。

- 2026-09-21
  变更摘要：生成 contract-cli 1.9.2-beta.1 本地交付包，未发布 GitHub Release 或 npm。
  涉及文件/模块：package.json、CHANGELOG.md、六个平台二进制资产、npm 离线包与交付校验信息。
  关键逻辑/决策：保留当前工作区全部改动，通过隔离暂存快照构建，排除既有忽略文件 mcp.yaml 对发布检查的干扰。
  产物：dist/local-package-1.9.2-beta.1-20260921，包含 npm tgz、release-assets、SHA256SUMS 和 BUILD-INFO.json。
  验证：release-check 全量通过；golangci-lint 0 issues；六个平台资产校验通过；隔离安装版本正确且 14 个 Skill 全部安装成功。

- 2026-09-21
  变更摘要：优化合同搜索 Skill 的自动发现入口，使客户自然语言能稳定路由到搜索能力。
  涉及文件/模块：contract-cli-contract-search 的 SKILL.md、agents/openai.yaml 及发现性回归测试。
  关键逻辑/决策：description 覆盖查合同、文本、日期状态、人员部门、交易方/我方主体和自定义字段；保留详情 Skill 边界。
  关键逻辑/决策：默认提示先由 Agent 查询候选和字段元数据，仅剩业务歧义时询问，并完整分页区分合同组与条目。
  对外对齐：采用“智书合同 CLI”产品名和客户文档中的自然语言入口；保持隐式调用开启。
  验证：发现性回归、搜索 Skill 回归和官方格式校验通过；golangci-lint 0 issues；全库仅剩既有 mcp.yaml 缺 get-employees 对齐失败。
  本机同步：以 1.9.1-beta.1 内嵌格式同步 Codex/agents 两处搜索 Skill；文件需在刷新或新建会话后加载。

- 2026-09-21
  变更摘要：按用户指定将本地测试包版本更新为 1.9.1-beta.1，从当前工作区重新打包安装。
  涉及文件/模块：package.json、CHANGELOG.md、本机 npm 安装与内嵌 Skills；未发布远端版本。
  关键逻辑/决策：旧 1.9.0-beta.1 产物缺少人员/部门和最新搜索总纲，使用干净暂存目录构建本机 darwin/arm64 离线包，避免旧内置二进制优先安装。
  验证：包元数据/Skill回归及安装器测试通过；临时前缀离线安装、6项命令检查与内嵌14个Skill/106文件校验通过。
  本机交付：~/.local/bin/contract-cli 已为1.9.1-beta.1；Codex/agents两处14个Skill均同步该版本，逐文件哈希一致，原授权配置未变。
  产物与备份：dist/local-install-1.9.1-beta.1-20260921；~/.codex/backups/contract-cli-1.9.1-beta.1-20260921-095609。

- 2026-09-20
  变更摘要：搜索 Skill 重整五步总纲，先授权/身份分流，再按明确对象、字段和纯关键词选择路径。
  涉及文件/模块：搜索主文件、shared 授权分流、搜索参考、mdm-legal 查询参考、总纲接口缺口复核文档。
  关键逻辑/决策：指定申请人优先候选消歧与ID筛选，姓名关键词仅显式需求保留；自定义先发现、歧义才问，明确文本/编号/日期不降级为全部关键词。
  关键逻辑/决策：user/app契约隔离；我方及归属人/创建人不虚构精确ID字段；授权状态不等于当前业务人员ID。
  接口分析：基础候选命令已齐；优先补我方精确契约与交易方ID元数据，页面全部语义/复杂AND/本人身份按范围补齐，不重复新增列表。
  验证范围：总纲复用既有对照；用户追加确认后实测生产 user 法人主体列表1候选及同ID详情均成功，证据 /tmp/contract-legal-user-sa0s0h1y；未修改运行代码或服务端接口。
  验证：Skill/引用/场景及安全文案回归、32段JSON、搜索Skill格式和11项离线路由复核通过；shared/legal通用格式校验在临时副本去除既有CLI version字段后通过。
  本机同步：重建1.9.0-search.local，搜索/shared/mdm-legal共25文件同步Codex/agents并校验；备份 ~/.codex/backups/contract-search-guideline-20260920-202253。

- 2026-09-20
  变更摘要：生产验证需求人张洋＋指定交易方，新增交易方组合配方并区分精确主体 ID、名称关键词与 legacy label。
  涉及文件/模块：搜索 Skill 主文件、trading-party.md、需求人/字段/组合/引导参考、contract-search-validation.md。
  关键逻辑/决策：保留需求人外部 user_id，交易方精确筛选使用 CONTRACT_TRADING_PARTY_ID；现有 mdm vendor list 按名称获取真实候选，无需用户找 ID。
  验证：两个真实 ID 完整命中截图1条1组，更换真实交易方为零；名称文本正反1/0；另一需求人加已知名称限制为零。
  验证：页面枚举ID数组无label为零；带正确label命中，更换错误主体ID仍命中，证明该分支不能用来验证精确ID；交易方单查未查完不报总量。
  关键逻辑/决策：记录元数据未暴露_ID的缺口、camelCase候选分页与合同组边界；保留并行新增的人员/部门查询路由。
  验证：独立 Agent 只读事实边界复核通过；本轮未修改 CLI 运行代码。
  验证：搜索场景/引用/安装回归、32 段 JSON、Skill 格式通过；两份新增模板与真实成功请求脱敏后完全一致。
  本机同步：重建内嵌并校验15个文件至Codex/agents，版本1.9.0-search.local；备份 ~/.codex/backups/contract-search-trading-20260920-195233。

- 2026-09-20
  变更摘要：确认 1.9.0-beta.1 在豆包 Linux 云端仍按任务隔离，记录跨任务登录复用缺口与平台接入要求。
  涉及文件/模块：docs/doubao-cloud-auth-reuse.md；未修改运行代码、安装包或真实凭证。
  关键逻辑/决策：现场 cloud-runtime + Credential Scope=task；任务目录和加密密钥均依赖 SESSION_ID，既有本机共享不覆盖该环境。
  关键逻辑/决策：需要平台账号级持久化或可验证身份衔接后再接凭证托管；不能仅移除任务 ID 或扩大 Linux 共享范围。
  验证：现有任务加密隔离、云端标记优先和弱证据拒绝共享用例通过；尚未实现或验收云端跨任务复用。

- 2026-09-20
  变更摘要：新增顶层 employee list、department list 及两个独立 Skill，补齐姓名/部门名到合同筛选 ID 的查询流程。
  涉及文件/模块：directory 命令/帮助/合同 Service/MCP ToolSpec、日志脱敏、独立 Skill、shared/search 路由、命令文档与打包验收。
  关键逻辑/决策：只读 user-only；部门单值映射 department_collection；人员固定 user_id；原样保留状态、ID、未知字段及分页，不自动选择候选。
  关键逻辑/决策：名称与部门 ID 互斥，人员必须有条件，部门允许一页目录；限制 page-size 1–200；业务失败错误退出。
  验证：请求/响应、身份、校验、安装和目录→合同回归先红后绿；独立复核补充网络/HTTP 失败最终日志保护。
  验证：目录定向测试覆盖100%，全库race通过（跳过既有旧mcp.yaml对齐）；lint 0 issues、npm包清单、CLI smoke和Skill格式通过。
  生产：新CLI查询人员/部门候选各1项，部门人员10项，需求人合同1条、需求部门合同35条，全部查完；证据 /tmp/contract-directory-live-1u5wa88f。
  本地交付：bin/contract-cli-directory（1.9.0-directory.local）；4个相关Skill同步Codex/agents，备份 ~/.codex/backups/contract-directory-20260920-193417，未发布。

- 2026-09-20
  变更摘要：实测合同需求人张洋，新增需求人查询三段式配方并同步搜索路由、字段及引导说明。
  涉及文件/模块：搜索 Skill 主文件、demand-person.md、user 参数/字段发现/引导参考、contract-search-validation.md。
  关键逻辑/决策：需求人 filter 使用外部 user_id；从已知合同详情取得候选并核对身份、回填验证，不用姓名或 PC 内部 ID，不默认要求用户找技术 ID。
  验证：同名样本消歧后，真实需求人筛选完整 1 条/1 组与截图对应；姓名、内部 ID 和另一有效用户加已知名称限制均为零。
  关键逻辑/决策：详情 ID 转换受配置影响，多需求人需消歧；历史 combine 参数存在契约冲突，采用当前元数据指定的 filter 路径。
  验证：独立 Agent 只读复核元数据、真实请求/响应及 Skill 事实边界通过；本轮未修改 CLI 运行代码。
  验证：搜索场景/引用/安装回归、30 段 JSON 及 Skill 格式通过；新增示例与真实请求脱敏后一致。
  本机同步：重建 1.9.0-search.local 内嵌并逐文件校验14个文件，同步Codex/agents；备份 ~/.codex/backups/contract-search-demand-20260920-192642。

- 2026-09-20
  变更摘要：获姓名确认后完成第 3、4 项生产对照，补全申请人＋关键词 ID 配方与姓名＋状态直接查询配方。
  涉及文件/模块：搜索 Skill 主文件、applicant-status.md、组合/字段/全部关键词/引导参考、contract-search-validation.md。
  关键逻辑/决策：姓名与多字段 OR 组合会放宽，改用返回的外部 ID；只有一个姓名关键词加独立状态可直接查询，不统一要求先找 ID。
  验证：第 3 项 ID 组合完整 3 组/5 条与基线目标子集一致、无匹配词为零；真姓名 MUST 首批就扩大至 50 组，不视为总量。
  验证：第 4 项姓名/ID＋状态10均完整 3 组/5 条且集合相同；无匹配姓名零、已知合同状态10/11正反1/0；PC状态filter业务失败。
  关键逻辑/决策：区分组展开中的3条变更中与2条审批中；报告各条真实状态，完整取回后再核对是否同一条目满足所有条件。
  验证：搜索场景/引用/安装回归、29 段 JSON 与 Skill 格式通过；3 份新增示例与真实请求脱敏后完全一致，独立复核通过。
  本机同步：重建 1.9.0-search.local 内嵌并校验同步13个文件至Codex/agents；备份 ~/.codex/backups/contract-search-status-20260920-190205。

- 2026-09-20
  变更摘要：补充全部关键词与申请人姓名 AND 的失败反例，以及 Agent 自动获取申请人外部 ID 的来源规则。
  涉及文件/模块：搜索 Skill 主文件、组合/全部关键词/引导参考及逐项验证记录。
  关键逻辑/决策：无候选申请人姓名 MUST 被服务端移除，不能交付为 AND 结果；复用返回的 submitter_user_id，区别 PC 内部 ID，不默认向用户索要 ID。
  验证：十二字段毛鹏追加无匹配申请人姓名 MUST 仍返回同一完整 30 条/27 组；具体人员正例待用户确认“范学东”与截图“范学冬（Peter）”的差异。
  验证：搜索场景/引用/安装回归、26 段 JSON 和独立 Agent 证据/路由检查通过；未将待确认的具体人员对照写为通过。
  本机同步：重建内嵌 Skill 并逐文件核验同步 Codex/agents；备份 ~/.codex/backups/contract-search-applicant-20260920-185253。

- 2026-09-20
  变更摘要：实测页面“全部”关键词毛鹏，新增三段式搜索配方并同步主入口、字段、文本及引导说明。
  涉及文件/模块：搜索 Skill、agents/openai.yaml、all-keyword-search.md 及相关参考、contract-search-validation.md。
  关键逻辑/决策：保留十二字段同词 SHOULD；区别归属人、申请人与文本范围；不传 MCP 未声明的 allConditionUnits，记录候选差异及分号编号限制。
  验证：生产完整 30 条/27 组与页面计数及可见样本对应，无匹配词 0 条；申请人单查 25 条/24 组，不能代替全部关键词。
  验证：搜索场景/引用/安装回归、26 段 JSON 与 Skill 格式通过；新 JSON 与实测请求一致，独立 Agent 路由及证据复核通过。
  本机同步：重建 1.9.0-search.local 内嵌 Skill，12 个文件校验同步 Codex/agents；备份 ~/.codex/backups/contract-search-keyword-20260920-184534。

- 2026-09-20
  变更摘要：将全部搜索实测逐项映射回 Skill，补合同申请日期、计数与时间精度边界。
  涉及文件/模块：搜索 Skill 主文件与申请日期/字段/组合/引导参考、场景回归、contract-search-validation.md。
  关键逻辑/决策：申请日期走 combine 的 submited 字符串路径；页面毫秒 filter 不可照搬；区分 103 组/110 条与展平排序，不将返回毫秒当作筛选精度保证。
  关键逻辑/决策：名称＋状态及续页统一到实测 combine 路径；补数值保真证据范围，收窄未验证能力的表述。
  验证：生产两日日期完整分页、7 个页面样本、38 条已归档子集及日期反例通过；末秒毫秒边界与自定义日期仍未宣称验证通过。
  验证：场景回归先红后绿，搜索/引用/安装回归及 25 段 JSON、Skill 格式通过；独立复核通过，lint 0 issues。
  本机同步：重建 1.9.0-search.local 内嵌 Skill，11 个文件逐一校验并同步 Codex/agents；备份 ~/.codex/backups/contract-search-date-20260920-184013。

- 2026-09-20
  变更摘要：逐项实测搜索组合，修复 JSON 数值保真与 user ID 类型静默忽略，统一搜索 Skill 字段及结果说明。
  涉及文件/模块：共享 JSON 解析、search 参数/帮助与回归、MCP 端点回归；搜索 Skill 主文件及字段/组合/引导参考、逐项验证记录。
  关键逻辑/决策：UseNumber 保留数字且拒绝多文档；user 显式非法 ID 类型提前报错，app 保持透传；新搜索日志只记错误类型。
  关键逻辑/决策：按实测支持名称、真实申请人 ID、顶层批量编号与正文组合；固定金额闭区间和单边 null；多词 AND 不可靠，部门/自定义日期缺样本不宣称验证通过。
  验证：新增回归先红后绿；实际结果集正反对照通过，数值修复后真实金额结果一致；自定义币种整数类型仅由生产元数据确认，零结果不作为阳性证明。
  验证：CLI 与其余包 race 通过，旧 ignored mcp.yaml 独立对齐检查缺新工具仍失败；lint 0 issues，Skill 格式/安装/示例及 6 项独立行为验证通过。
  本机同步：生成 bin/contract-cli-search-optimization（1.9.0-search.local），仅搜索 Skill 已备份同步 Codex/agents；正式 PATH CLI 仍为 1.8.6，未发布。

- 2026-09-20
  变更摘要：生产复现并修复 user 合同搜索将业务失败报告为进程成功的问题。
  涉及文件/模块：contract_command.go、contract_search_response_test.go、搜索 Skill、contract-search-validation.md。
  关键逻辑/决策：user 复用 MCP envelope 校验并保留响应，app 契约不变；日志不记录搜索值或合同内容。
  验证：回归先红后绿；同一生产 code=110000 请求退出码从 0 变为 1，正常正文查询返回 18 条且 has_more=false。

- 2026-09-20
  变更摘要：按用户指定将本地 Device 共享安装包版本调整为 1.9.0-beta.1，重新构建交付。
  涉及文件/模块：package.json、CHANGELOG.md、六平台二进制、npm 离线包与内嵌 Skills。
  关键逻辑/决策：沿用已通过 race / lint 的源码快照，仅调整发布版本；保持其他任务的并行改动原样。
  验证：六平台重建及 SHA-256 校验通过；1.9.0-beta.1 全新目录离线安装成功，CLI 与 12 个内嵌 Skills 版本一致。

- 2026-09-20
  变更摘要：搜索 Skill 增加引导式澄清，明确需求直接查询，有业务歧义时用字段、匹配方式和值反问。
  涉及文件/模块：查询 Skill 主文件、agents/openai.yaml、references/guided-search.md、搜索字段发现参考。
  关键逻辑/决策：元数据候选有来源；多候选中立选择；保留已确认条件，修改条件清除分页 token；用户确认不能代替接口能力验证。
  验证：现有搜索场景、引用及安装回归通过，Skill 格式校验通过；独立 Agent 离线验证 6 类对话行为，无生产调用。
  本机同步：仅本次查询 Skill 文件已备份并同步至 Codex/agents，保留安装版本；备份目录为 ~/.codex/backups/contract-search-guidance-20260920-174653。

- 2026-09-20
  变更摘要：Device 授权在确认的本机桌面环境按系统用户与 profile 跨任务复用，生成 1.9.0-beta.1 本地安装包。
  涉及文件/模块：internal/credential、internal/cli 的 Device 授权/存储/锁/状态/测试，auth/shared/contract Skills、README、package.json；清理全库既有 lint 问题。
  关键逻辑/决策：复用来源证据选择存储范围；本地用独立系统安全存储命名空间，云端/未知保持任务隔离，不迁移旧任务凭证；真实 OS 用户目录承载共享锁。
  关键逻辑/决策：init 复用有效授权并按需刷新；刷新、授权和退出共用锁；空 profile 可恢复共享身份；旧授权码显式模式保留。
  验证：新增回归先红后绿；发布源码快照 go test -race ./... 通过，原工作树 lint 0 issues；本地旧 ignored mcp.yaml 保留且不打包，独立对齐检查按既有逻辑跳过。

- 2026-09-20
  变更摘要：从场景引导、字段契约、组合逻辑到结果恢复，完成搜索能力 Agent 友好性评估并记录改进优先级。
  涉及文件/模块：`docs/contract-search-agent-review.md`；未修改搜索实现或 Skill。
  关键逻辑/决策：保留三段式场景，优先修正组合语义和执行一致性，再完善元数据转换、字段角色/边界与追加查询流程。
  验证：三个独立 Agent 评估；生产小范围只读复现无匹配姓名 MUST + 正文返回正文结果；临时 mock 复现业务错误信号、数值及 ID 类型问题，诊断测试已清理。

- 2026-09-20
  变更摘要：新增 user-only `contract search-fields`，按 MCP `list-contract-search-filter-fields` 契约发现搜索字段元数据。
  涉及文件/模块：CLI 路由/命令/帮助、MCP ToolSpec、合同 Service、契约/命令测试、查询及共享 Skill、命令文档与测试计划。
  关键逻辑/决策：仅透传可选 keyword，清除无关 query；完整保留响应及数值，业务失败保留输出并返回错误；人员/部门列表及既有 search 行为不变。
  验证：TDD 红→绿，CLI/合同 Service 全包通过；生产真实字段发现→元数据构造搜索及无匹配字段成功；全量受旧 ignored mcp.yaml 阻断，lint 仍有既有 42 项，本次无新增。
  本机同步：独立开发版输出至 bin/contract-cli-search-fields（1.8.6-search-fields.local）；查询及共享 Skill 已备份并同步 Codex/agents，正式 CLI 1.8.6 保留。

- 2026-09-20
  变更摘要：生产实测申请人姓名关键词搜索，将查询 Skill 场景 6 从仅 ID 模板改为直接按姓名查询。
  涉及文件/模块：查询 Skill 主文件、user 参数参考、搜索对齐记录、场景 JSON 回归测试。
  关键逻辑/决策：姓名使用 CONTRACT_SUBMIT_NAME/string/SHOULD，无需人员列表；精确人员和“我申请的”仍需可靠 ID，保留重名与历史人员边界。
  验证：生产完整姓名与部分姓名各首批 20 条且有后续页，无匹配姓名 0 条，部门名称对照成功；回归测试先失败后通过，Skill 结构及安装校验通过；lint 仍为既有 42 项。

- 2026-09-20
  变更摘要：将查询 Skill 改为 9 个独立场景，每个场景固定“用户怎么说、应该怎么查、示例 JSON”三段结构。
  涉及文件/模块：查询 Skill 主文件、Agent 提示、场景 JSON 回归测试。
  关键逻辑/决策：说明与完整请求就近排列，拆开人员和部门；人员、自定义字段、续页例子明确是前置条件满足后才可执行的模板。
  验证：先以 3 个示例不满足 9 场景复现失败，再校验文本范围、组合条件、日期角色和续页完整保留原请求；无生产调用。

- 2026-09-20
  变更摘要：在独立查询 Skill 主文件内置 8 类 user 搜索场景提示与 3 个可复用 JSON 请求。
  涉及文件/模块：`skills/contract-cli-contract-search/SKILL.md`、Agent 提示、搜索示例回归测试。
  关键逻辑/决策：自然语言映射到文本范围、页签、状态、编号、日期、候选发现和分页；示例保持正文与金额币种的 AND 关系，不将“我的合同”误作仅本人申请。
  验证：先失败后通过；CLI 全包测试、Skill 格式校验、4 个独立 Agent 场景通过；全量仅本地 ignored mcp.yaml 缺少 list-process-comments 的既有对齐测试失败，lint 仍为已有 42 项。
  本机同步：更新 Codex 与 agents 中的查询 Skill 并备份原内容；未执行生产组合查询。

- 2026-09-20
  变更摘要：将合同搜索独立为 `contract-cli-contract-search` Skill，迁移 user/app V1/app V2 参数并补齐合同文本搜索配方。
  涉及文件/模块：`skills/contract-cli-contract-search`、原合同/共享 Skill、搜索及安装测试、README、CLI 参考与测试计划。
  关键逻辑/决策：五类文本 SHOULD 与仅正文明确分离，业务失败不当空结果，按 has_more 翻页；旧参考保留跳转，搜索说明只有一个维护位置。
  验证：先失败后通过；全量 Go 测试、新 Skill 校验、内嵌安装/链接、JSON 配方及 7 个 Agent 场景通过；lint 与 HEAD 基线一致仍报 42 项，无新增。
  本机同步：从工作树安装查询/合同/共享三个 Skill 至 Codex 与 agents 目录并校验内容，安装版本标为 `1.8.6-search-skill.local`；保留备份和正式 CLI 1.8.6。

- 2026-09-20
  变更摘要：补充页面“合同文本”与 CLI 的生产查询对照，纠正此前空结果解释。
  涉及文件/模块：`docs/contract-search-mcp-alignment.md`、`docs/ai-changes.md`。
  关键逻辑/决策：正文 MUST 返回 0、SHOULD/默认返回 18；页面五文本字段 SHOULD 返回 19；将字段范围和条件语义分别纳入 Skill/搜索验收。
  验证：同 profile、关键词、页签和 page_size 控制变量只读实测；未修改业务代码或生产数据。

- 2026-09-20
  变更摘要：记录 CLI 1.8.6 与生产 MCP 合同搜索的对齐范围及只读验证结果，尚未实现新命令。
  涉及文件/模块：`docs/contract-search-mcp-alignment.md`、`docs/ai-changes.md`。
  关键逻辑/决策：补齐字段/人员/部门发现，纳入 user_id_type、业务错误退出、JSON 保真和 Skill 编排；以生产真实响应为基线，不将本地 V2 契约视为已上线。
  验证：搜索及元数据小范围只读实测、正常部门到人员链路验证通过；复现业务错误退出码为 0；现有搜索与 MCP 契约定向测试通过。

- 2026-09-20
  变更摘要：按用户要求将 dev001 的 search-contracts 从 Unbox semantic 契约恢复为原版 contract-group 搜索。
  涉及文件/模块：`mcp-config-dev001-optimized.yaml`、`docs/mcp-dev001-optimization.md`、`docs/ai-changes.md`。
  关键逻辑/决策：以 dev-contract-charts 提交 c306fb2f8 的完整搜索定义为准，补回依赖的 list-contract-search-filter-fields；共 31 个工具，其他 29 个及 server 完全不变；仅本地修改、未部署。
  验证：先红后绿；8 项回归测试、31 个输入 Schema 的 Higress 编译校验通过，原版搜索/字段发现元数据与当天 group 快照一致。

- 2026-09-20
  变更摘要：记录用户更新优化版后的在线握手与元数据一致性验证。
  涉及文件/模块：`docs/mcp-dev001-optimization.md`。
  关键逻辑/决策：2025-03-26 与 2025-06-18 均成功协商；线上 30 个工具的描述和输入/输出 Schema 与交付版一致；日志收录滞后，未声称完成业务端到端验收。

- 2026-09-20
  变更摘要：结合 group 和后端/网关源码生成 dev001 工具描述优化版，恢复正文重试规则、统一部门 ID 和人员匹配语义、补查询回执定义。
  涉及文件/模块：`mcp-config-dev001-optimized.yaml`、`docs/mcp-dev001-optimization.md`、`docs/mcp-tool-description-comparison.md`。
  关键逻辑/决策：仅移除定义不完整的 sync-user-groups（31→30）；保留工具的调用模板、鉴权、输入类型/默认值/必填约束全部不变；回执字段可选、value 保留任意 JSON 类型；未下发 Higress。
  验证：先红后绿；30 个输入 Schema、23 个搜索输入用例、6 项输出 Schema 测试、调用契约差异检查及独立静态复核通过。

- 2026-09-20
  变更摘要：读取两个 MCP 服务的工具元数据，完成描述清晰度、Agent 可用性和契约兼容性评审。
  涉及文件/模块：`docs/mcp-tool-description-comparison.md`、`docs/mcp-metadata/2026-09-20/`。
  关键逻辑/决策：比较 31 个共同工具并单列 group 独有 25 个；建议以 dev 业务化契约为基础补回关键语义，标出搜索接口结构变化；仅保存元数据，不保存 token，未修改远端或调用业务工具。

- 2026-09-20
  变更摘要：补充用户更新 dev001 配置后的在线验证结果。
  涉及文件/模块：`docs/mcp-dev001-schema-fix.md`。
  关键逻辑/决策：握手正常、31 个工具元数据可读取且修复字段已生效；OpenObserve 确认请求由 MCP Wasm 处理，观察窗口无新增 Schema 加载错误；业务授权查询未执行。

- 2026-09-20
  变更摘要：生成 dev001 MCP 配置修正版，修复 `status_not_in.items` 被错误保存为字符串导致的 Schema 加载失败。
  涉及文件/模块：`mcp-config-dev001-fixed.yaml`、`docs/mcp-dev001-schema-fix.md`。
  关键逻辑/决策：仅展开为与 `status_in` 相同的 14 状态枚举；保留 `value_selectors` 原定义及 null 语义；未下发 Higress。
  验证：先复现原配置错误，再以官方校验器验证 31 个工具通过；23 个输入用例、nullable 兼容性与唯一字段差异检查通过，并核对本地后端状态枚举。

- 2026-09-16
  变更摘要：审批矩阵运算符改由 CLI 内置映射提供，`rule symbol query` 全程本地执行。
  涉及文件/模块：`internal/cli/approval_matrix_extensions.go`、矩阵回归测试、`contract-cli-rule` Skill、审批矩阵命令文档和测试场景。
  关键逻辑/决策：移除 `/symbols/query` 网络路由，固定维护 STRING、NUMBER、集合类及 BOOLEAN 兼容映射；列配置仍把 `symbol` 交给开平接口做最终校验。已完成全量 Go 测试、Skill 校验，并打包安装 `1.8.3-test.8`。

- 2026-09-14
  变更摘要：移除 dev 联调环境，替换为 test；默认仍为 prod，正式构建仍仅允许 prod。
  涉及文件/模块：环境预设、端点白名单、构建开关和脚本、联调测试、README 和双身份接入说明。
  关键逻辑/决策：test 域名使用 test-open.qtech.cn 与 test-myaccount.qtech.cn，公开账号元数据已确认；不依赖开放平台未提供的资源发现路径。新增 testBuild / ENABLE_TEST 开关，包版本使用 -test.N；所有构建继续拦截已移除的 dev 地址，避免旧凭证被继续使用。新建独立 contract-test 配置，不迁移旧凭证。

- 2026-09-14
  变更摘要：审批矩阵补齐 user/app 双身份，用户登录复用现有 OAuth 流程，其他模块身份策略不变。
  涉及文件/模块：规则命令、导入身份绑定、OpenAPI user 选择头、帮助、Skill 和回归测试；配套规则后端增加用户验签、管理员权限与实际操作人审计。
  关键逻辑/决策：user 不自动降级 app，导入计划持久化凭证摘要而非 Token，app 旧指纹保留；网关需透传 Authorization 并保留/重建身份选择头，禁止将 user 当 app 放行。未打包、未部署、未执行真实业务写入，详见 docs/approval-matrix-user-app-auth.md。
  验证：CLI 全量 go test、go vet 与相关竞态测试通过；后端以缓存依赖定向编译当前源码，68 个 JUnit 回归测试通过（非完整 Maven 构建）；Skill YAML 使用 ruamel.yaml 校验，自带脚本所需 PyYAML 在当前环境缺失。

- 2026-09-14
  变更摘要：增加独立 dev 联调构建与打包入口，正式构建仍仅支持 prod，所有构建默认环境仍为 prod。
  涉及文件/模块：环境预设、配置及 Device 凭据校验、授权链接校验、网络拦截、命令帮助、正式回归测试、构建脚本和 README。
  关键逻辑/决策：通过 ldflags 显式开启 developmentBuild；dev/prod 配置须分别匹配对应 API 与账号域名；跨环境更新同名 profile 清理旧认证。联调包版本使用 -dev.N，在暂存区构建六平台产物，不覆盖正式 npm 包或 release assets；内置生产 Skills 不变。

- 2026-09-14
  变更摘要：按 bpm-rule-configuration 的 20260901-zss-approval@c77a6c6 和交互文档第 3 版补齐审批矩阵结构接口。
  涉及文件/模块：internal/cli 审批矩阵结构路由、兼容别名、导入执行、帮助和正式回归测试；审批矩阵 Skill、参数文档、技术方案。
  关键逻辑/决策：新增规则组 get/create、矩阵 get/create/update/delete、列 add/update/delete 共 9 个 HTTP 能力；保留原有原子命令和 api call 关闭策略。导入校准 int32/Long DTO，绑定列头和应用环境摘要，写前落盘 running，uncertain 停批，支持局部确认、分批续跑、get/cancel。完整矩阵/行版本锁和跨计划去重仍需后续补齐。

- 2026-09-10
  变更摘要：新增 contract-skill-builder 元技能包（初版），供产品同学基于前端代码 / OpenObserve test 日志 / test MySQL 快速识别接口并生成业务 skill 骨架。
  涉及文件/模块：`contract-skill-builder/`（SKILL.md、config.example.yaml、scripts/mysql_readonly.sh、scripts/MysqlTestQuery.java、examples/合同到期提醒.skill.md、README.md，均为新增）。
  关键逻辑/决策：确立「代码候选 + 页面操作 + 日志验证 + 数据库校准」四源闭环；git clone 前端仓 master 拿候选接口，OpenObserve test SearchSQL 按用户+时间窗验证真实时序/参数，只读 MySQL 校验字段与写生效；敏感信息（git token / 日志账号密码 / db 密码）全部走 env 不落盘；生成骨架强制写操作人工确认节点；MysqlTestQuery.java 随包分发并封装只读 SQL 白名单。


- 2026-09-10
  变更摘要：新增 Skill 能力交付全链路信息图，用一张图展示 Web 接口底座、Skill 场景封装及评测发布平台。
  涉及文件/模块：`docs/assets/skill-delivery-architecture-overview.png`（新增）。
  关键逻辑/决策：采用三层递进结构；底层展示 1316+ Web 接口开放链路及安全底座，中层展示产品编排与 Agent 自由选型，上层展示五步评测发布流程；右侧独立呈现 30/30/25/15 评分权重和总分/安全双门槛。
- 2026-09-10
  变更摘要：新增 Skill 验证平台技术方案文档（含总体架构/运行时序/部署/对话微调等多张 mermaid 图例）。
  涉及文件/模块：`docs/skill-eval-platform-architecture.md`（新增）。
  关键逻辑/决策：确立「薄平台 + headless-agent 通用运行时 + 两个评测 skill（eval-case-gen / eval-run-report）+ 执行器工具 + 评分器」五层架构；明确智能（LLM）与确定（执行器/评分器）分离，执行走工具、评分走平台、对话复用 turn/steer 并加阶段限权；复用 agentCodex 内核 7 模块、替换 5 接线端；给出执行器工具与评分器接口草案、部署图（headless 实例池/每任务 dataDir/无 Electron）、关键数据契约、M1-M6 里程碑。
- 2026-09-10
  变更摘要：源码级重新分析 agentCodex 与 Skill 验证平台的结合方式，确认复用纯 Node 内核、无需桥接 Electron。
  涉及文件/模块：`agentCodex/docs/skill-eval-integration.md`（新增）、`docs/skill-eval-agent-integration.md`（v2 修订）、`docs/skill-eval-platform-effort.md`（M4 由 8→4 人日，总量 50→46 人日）。
  关键逻辑/决策：逐层确认 agent-session-service/codex-stdio-connection/codex-app-server-runtime/json-rpc-client/shared 均为纯 Node、依赖注入、无 electron 引用；main.ts 只是组装器；方案由「桥接 Electron」改为「复用内核抽 headless-agent」，替换 5 个 Electron 依赖（视觉/视频 stub、附件空实现、emit 接平台、models 配置化、logger），暴露 HTTP 接口；无需 Linux Electron 构建与 Xvfb；每评测任务 dataDir 解决单实例锁；改造量 3-5 人日。
- 2026-09-10
  变更摘要：新增 Skill 验证平台与 agentCodex 的集成架构文档，明确两个集成点与职责边界。
  涉及文件/模块：`docs/skill-eval-agent-integration.md`（新增）。
  关键逻辑/决策：平台=调度展示层，agentCodex=评测大脑；集成点①（M4）经桥接（推荐抽取 Main session-service 为 headless 服务）调用约 50 个无 UI IPC 能力驱动用例生成与审批应答；集成点②（M6）用例执行不走 Agent 对话而由平台执行器直调 B' 链路（确定性/可断言/可重放），Agent 仅做智能环节（生成场景、失败归因建议）；给出 skill 生命周期七步数据流与里程碑对应关系。
- 2026-09-10
  变更摘要：新增 Skill 验证平台工作量评估（基于 PRD 与 16 屏 UI 稿核对）。
  涉及文件/模块：`docs/skill-eval-platform-effort.md`（新增）。
  关键逻辑/决策：P0 全量 50 人日（单人全栈口径）≈ 2.5 人月，双人并行日历 3-4 周，含缓冲建议承诺 6.5-7 人周；模块拆解 M1 基座 5 / M2 上传解析 6 / M3 归档 4 / M4 Agent 桥接 8（成败点：CDP vs Main 服务抽取需先 PoC） / M5 确认工作台 6 / M6 沙箱执行 10（安全集中点） / M7 SSE 3 / M8 报告评分 5 / M9 导出通知 3；UI 稿 16 屏与 PRD 覆盖一致无大缺口（新增用例表单与版本时间线需确认口径）；列 4 项量化假设与 4 项前置依赖（B' 链路、清单 v2、linux 构建、评测租户）。
- 2026-09-10
  变更摘要：补齐 Skill 验证平台全流程 UI 设计，共 17 张页面与状态图，并新增业务流程索引。
  涉及文件/模块：`docs/assets/skill-eval-platform-ui/*.png`、`docs/skill-eval-platform-ui-design.md`。
  关键逻辑/决策：覆盖登录、列表/空态、上传成功/失败、详情、生成中/失败、用例确认/编辑、排队/执行/中止、通过/未通过报告；所有页面复用统一蓝白企业级设计系统，异常状态提供明确证据、恢复入口与安全确认。
- 2026-09-10
  变更摘要：新增 Skill 验证平台「用例确认」高保真 UI 图，用于产品评审与前端实现参考。
  涉及文件/模块：`docs/assets/skill-eval-platform-case-confirmation-ui.png`（新增）。
  关键逻辑/决策：围绕 P5 核心工作台出图；采用左侧导航、四步评测进度、五类用例表格、Agent 可解释侧栏与底部常驻执行栏；类别颜色、覆盖统计与写操作风险提示均按 PRD 表达。
- 2026-09-10
  变更摘要：新增 Skill 验证平台 PRD（供 UI 出图），覆盖 8 个页面、任务状态机、功能/非功能需求与数据契约。
  涉及文件/模块：`docs/skill-eval-platform-prd.md`（新增）。
  关键逻辑/决策：页面清单 P0-P8（登录/列表/上传/详情/生成中/用例确认/执行中/报告），核心交互页为用例确认（五类徽章配色 + 可解释侧栏 + 新增用例）与报告页（四维评分条 + 门禁徽章 + 一票否决提示 + 改进建议）；任务状态机 created→generating→awaiting_confirm→running→finished（含 failed/aborted 分支）；断线恢复用持久任务+推送；明确本期不做在线编辑/多角色/移动端/线上指标；附开发数据契约速查表。
- 2026-09-10
  变更摘要：澄清评测 Web 交互方案——agentCodex 服务器端无本地 UI，平台自建轻量 Web 经 Main IPC 协议桥接驱动。
  涉及文件/模块：`docs/delivery-platform-design.md`（5.4 MVP 第 3 条扩充）。
  关键逻辑/决策：基于代码证据（desktop-api.ts 约 50 个 IPC channel：send-message/decide-approval/answer-user-input/list-skills 等均为无 UI 纯协议调用），确定 Renderer 只是 Main 服务的消费者；推荐路线 i：平台 Web 后端经本地桥接调 Main 服务，Web 呈现任务时间线（生成用例→确认→执行→报告）而非聊天窗；路线 ii（remote-debugging 投屏）仅作应急兜底。
- 2026-09-10
  变更摘要：交付平台设计确认评测执行环境——agentCodex 服务端单机部署 + Web 页面使用，新增风险清单与 MVP 范围。
  涉及文件/模块：`docs/delivery-platform-design.md`（第 5 节新增）。
  关键逻辑/决策：部署形态为评测平台轻量 Web 服务 + 服务器上 agentCodex 实例（AI_ASSISTANT_USER_DATA_PATH 按会话隔离，代码已支持）；九大风险点：Linux 无头显示（建议 MVP 禁用 Computer Use/Git 页）、单实例锁（多 userData 实例解并发）、评测租户 token 托管（KMS+短时，禁用个人 token 跑写用例）、写副作用（测试租户+数据打标）、审批弹窗（协议自动应答或最高权限沙箱）、版本锁定、资源 8C32G/3-5 并发、缺 linux 构建目标需补；MVP 五步范围（Linux 构建、特性禁用、四页面 Web、串行起步、租户打标）。
- 2026-09-10
  变更摘要：交付平台设计 v2，吸收产品边界调整（skill 平台仅输出不绑定运行时；评测改为上传→Agent 生成用例→确认→执行→报告）。
  涉及文件/模块：`docs/delivery-platform-design.md`（重写为 v2）。
  关键逻辑/决策：交付物 2 明确「做输出/校验/组装助手，不做执行/绑定运行时/托管凭证」，skill 包以声明式 YAML + SKILL.md 公共子集保证 Codex/WorkBuddy/豆包均可装载；交付物 3 新增智能评测链路（五类场景：正常/边界/异常/权限/幂等，用户确认后用例集随版本归档复用）；评分维持 30/30/25/15 与 ≥80 且安全 ≥90，新增写用例失败一票否决，「运行时表现」本版以试跑耗时/稳定性折算；MCP 建议保留 SSOT+派生+治理复用；列出 4 项待确认（评测 Agent 载体、沙箱写策略、两 CLI 格式一致性、折算规则）。
- 2026-09-10
  变更摘要：新增客户端 Agent 云端化可行性分析（本地终端模式 → 手机号登录多租户 Web 模式）。
  涉及文件/模块：`docs/client-agent-web-remoting.md`（新增）。
  关键逻辑/决策：盘点本地模式五类依赖（文件/凭证/会话/skill 安装/终端交互）；结论是 CLI 内核约 70% 可平移（通道/清单/声明式 skill/JSON 输出），但多租户 SaaS 设施（登录/沙箱隔离/文件代理/前端/审计）需从零建且为工作量大头，粗估 4-6 人月；列出 14 项问题（高风险 5：token 金库、租户隔离、授权合规、确认真实性、文件生命周期）；推荐三阶段演进——先走 B' 链路+平台 worker，再用飞书做交互壳跳过自建前端与手机号登录，最后独立 Web 端；提出关键疑问：目标用户若无飞书账号手机号登录才有必要。
- 2026-09-10
  变更摘要：新增交付平台设计文档，覆盖基础建设/Skill 生成平台/Skill 归档评分 CI-CD 发布三大交付物，含 MCP 发布专项建议。
  涉及文件/模块：`docs/delivery-platform-design.md`（新增）。
  关键逻辑/决策：定义 skill.yaml 包格式（deps 为生产开白依据、自动插入人工确认节点、PII 声明、写接口不自动重试）；设计 registry 状态机与四维评分门禁（静态 30/试跑 30/安全 25/运行时 15）；CI 七步流水线（schema→静态→清单核对→沙箱试跑→安全扫描→评分→产物）；发布多端走 MR 机制（contract-cli skills/ + embed + 现有 release-beta.sh 升级）；MCP 建议以 web-api-catalog 为 SSOT、skill 发布自动派生 MCP tools、读类直接暴露/写类默认由 skill 承接、复用开平治理与 resource_metadata 发现规范；给出 M1–M5 里程碑与 5 项待确认（everyline 关系、MCP 维护方式、沙箱写策略、评分阈值、运行时载体）。
- 2026-09-09
  变更摘要：技术架构总览补充「CLI 定位澄清」与「安全设计」两个章节，并同步飞书。
  涉及文件/模块：`docs/web-api-exposure-architecture.md`、飞书《技术架构总览》。
  关键逻辑/决策：明确定位转变——CLI 不再承载业务命令（结构化命令冻结只维护），只提供标准 HTTP 调用通道 + 接口清单，业务闭环完全由 skill 承接（编排/参数映射/状态/确认节点）；安全设计覆盖四层：令牌（scope 收敛、Agent 短时 token）、开平（userTokenPaths/userId 强制非空/限流/ID转换）、Web 接口层（写操作确认、幂等、PII、IDOR）、治理（灰度只读→低危写→高危写、白名单回收、清单 diff 告警）；列出 4 项安全待确认（UserID 强制性、网络路径、PII 清单、Agent token 生命周期）。
- 2026-09-09
  变更摘要：技术架构总览补充部署架构章节，推荐 B 通道走 open-platform 复用限流与敏感信息 ID 转换能力。
  涉及文件/模块：`docs/web-api-exposure-architecture.md`（更新 1.4 节）、飞书《技术架构总览》同步。
  关键逻辑/决策：基于代码证据确认 open-platform 已有 RateLimiterGatewayFilterFactory（Nacos 动态限流）与 RewriteUserId/DeptId 过滤器（user_id/dept_id 内外 ID 转换）；对比 B'（open-gateway→open-platform→clm-api，推荐）与 B（直连 common-gateway，fallback）两条路线，采纳 B' 理由：限流/ID 转换是 Agent 硬需求、生产白名单复用 ACL、CLI 单出口改动最小、A/B 统一网关治理；分发设计相应简化为同网关前缀区分。
- 2026-09-09
  变更摘要：新增技术架构与业务流程方案文档（评审用），含现状/变动架构 mermaid 图、产品使用流程图、按模块接口附表。
  涉及文件/模块：`docs/web-api-exposure-architecture.md`（新增）。
  关键逻辑/决策：架构图区分「现状 CLI→open-apis 单通道」与「改造后双通道 + Agent 平台」；业务流程按内部验证/产品生成 skill/用户执行 skill/生产名单控制四个 sequence/flowchart 表达；接口附表基于 doc001 清单按模块汇总（contract 1203 / office 33 / _custom 26 / retrieval 12 / dreamcar 10 / 散落 32 / systemConfig 1，共 1316），方法分布 POST 589/GET 576/PUT 86/DELETE 47/其他 18，未识别 12 待 S1.1 补齐。
- 2026-09-09
  变更摘要：新增前端 API 静态解析脚本，完成 S1 接口清单与接口文档的初版生成。
  涉及文件/模块：`tools/gen_web_api_catalog.py`（新增）。
  关键逻辑/决策：扫描 clm-monorepo-fe 的 `features/common/src/api/**`（函数级高精度，free-swagger 格式）+ `apps/*/src`、`features/pc/src`（行级兜底），提取 url/method/params/data/返回泛型/注释，按 path+method 去重，输出机器清单 JSON 与人读 Markdown。实测去重后 1316 个接口（common 共享层 1288 + apps 自有 clm 19/admin 9），counterparty/clm-h5 无自有接口复用 common。已知不足：12 个 method 未识别（上传/下载/preload 边缘）、61 个 path 含模板变量待解析常量。
- 2026-09-09
  变更摘要：Web 接口暴露方案 v2 修订，吸收评审结论（common-gateway 纯转发、1400+ 全量+文档同步、生产按 skill 依赖动态开白、清单覆盖 apps/* 全部、命令命名暂不设计）。
  涉及文件/模块：`docs/web-api-exposure-design.md`（更新为 v2）。
  关键逻辑/决策：在 v1 基础上新增「生产白名单 + skill 发布联动」章节（`WebPathAllowlist`/`WebAllowAll`，skill deps 驱动 GitOps 开白）；接口清单从 apps/clm 扩为 apps/* 全部并同步生成人读文档；common-gateway 校验项降级为无需考虑；命令命名改为 TBD。未改动任何代码。

- 2026-09-09
  变更摘要：新增 Web 接口暴露方案设计文档，梳理开平与 Web 两条请求链路并给出 CLI 改造方案。
  涉及文件/模块：`docs/web-api-exposure-design.md`（新增）、`docs/skill-capability-schemes.md`（前序方案对比）。
  关键逻辑/决策：实测确认 user OAuth token 以 `Authorization: Bearer` 可直连 `contract.qfei.cn/clm/api/**`，含真实业务读接口 `POST /clm/api/es/search/contractList`；`tenant_id` 非强制，租户取自 `data.employee.tenantId`。方案定为传输层引入 Channel（`open_platform`/`web`）解耦 `buildURL` 对 `/open-apis/` 的硬编码、Profile 增加 `WebBaseURL` 并同步放行 `prod_environment.go` 的生产 origin 校验、新增 `web call` 通用命令（强制 user 身份 + 路径白名单）、按 path 前缀在 `openPlatformClientAndContext` 单点分发；接口清单从前端 `features/common/src/api/**` 静态提取（约 1400+ 条）作为 skill 工具注册表来源。未改动任何代码。

- 2026-09-02
  变更摘要：为 CLI 的所有 OpenPlatform 业务请求增加请求级 Trace 关联。
  涉及文件/模块：`internal/tracecontext`、`internal/openplatform` 统一客户端、README、命令参考与测试计划。
  关键逻辑/决策：每个逻辑请求使用加密安全随机数生成 W3C `trace_id`，每个实际 HTTP attempt 生成独立 `span_id`；统一覆盖发送 `traceparent` 和与其同值的 `X-Log-Id`，网络重试与 Token 刷新重放保持同一 Trace；响应对象和最终错误保留 `trace_id`，且不改变已有错误类型的 `errors.As` 判断。Trace ID 仅用于可观测性，不用于鉴权、幂等或客户端来源证明。

- 2026-09-01
  变更摘要：基于审批矩阵 Agent CLI 范围文档，为现有 `rule table` 命令组新增批量导入计划与执行链路。
  涉及文件/模块：`internal/cli` 审批矩阵批量导入、帮助与回归测试，`skills/contract-cli-rule`、CLI 命令文档和技术方案。
  关键逻辑/决策：保留原有 10 个审批矩阵原子命令的参数和路由；`import plan` 只读列头并做列映射/类型校验，`import apply` 串行逐行写入且持久化每行状态；重试跳过成功行，结果不确定的写入不自动重试。

- 2026-09-01
  变更摘要：将 Doubao Work 纳入运行环境识别并作为独立渠道透传。
  涉及文件/模块：`internal/invocation` 客户端规则与测试、运行环境识别 README 和专项测试文档。
  关键逻辑/决策：macOS 基于官方应用通过系统验签得到的 Bundle ID `com.work.pc.doubao` 与 Team ID `96L78H6LMH`；Windows 基于官方 2.27.10 x64、ARM64 发行包中 `DoubaoWork.exe` 的有效 Authenticode 签名登记叶证书 SHA-256，并要求证书与可执行文件路径/名称同时命中。新增智能体来源值 `doubaoWork`、`client.doubao_work.signed-bundle` 与 `client.doubao_work.authenticode`；即使 Doubao 与 Doubao Work 当前共享发布者证书，也通过各自路径规则独立分类。
- 2026-08-12
  变更摘要：将下一版 contract-cli 版本更新为 `1.7.0`。
  涉及文件/模块：`package.json`、发布版本元数据。
  关键逻辑/决策：仅更新源码包版本，为后续 `v1.7.0` 发布做准备；本次不创建 tag、不触发 GitHub Release 或 npm 发布。

- 2026-08-12
  变更摘要：补齐创建合同 `contract_category_abbreviation` 的动态来源与错误排查说明。
  涉及文件/模块：`skills/contract-cli-contract` 创建、分类 references、主 Skill 导航及文档契约测试。
  关键逻辑/决策：要求用与创建相同的 profile/身份查询分类树并取可用末级 `abbreviation`；明确名称、编号和分类 id 不能替代，用显式占位值替换易误抄的 `PROCUREMENT` 示例，并移除不被 Skill 校验器接受的自定义 frontmatter `version`。

- 2026-08-10
  变更摘要：修正 user MCP 合同搜索结构化筛选值的类型与约束说明。
  涉及文件/模块：`skills/contract-cli-contract` user 搜索参数参考及 CLI 文档契约测试。
  关键逻辑/决策：按当前 CLI 默认 legacy profile 和 CLM processor 区分实际可用值与推荐数组形态；明确外部 user/department ID、0/1 integer、盖章份数、枚举及自定义字段 JSON 类型，保留 legacy 单边范围和币种单值兼容。

- 2026-08-10
  变更摘要：将合同搜索 Skill 升级为 user MCP、app V1、app V2 三套独立参数契约。
  涉及文件/模块：`skills/contract-cli-contract` 搜索 references、响应字段、agent 元数据，shared 约束与 CLI 文档契约测试。
  关键逻辑/决策：按实际身份路由拆分字段类型、必填性、枚举、分页和请求示例；明确 app V1 精确查询、V2 ES 模糊查询及 MCP 结构化筛选边界，并记录当前 CLI user_id_type/字段发现限制。

- 2026-08-10
  变更摘要：补齐 P3 33 个新接口的独立参数参考。
  涉及文件/模块：合同、MDM、事件、审批矩阵 Skills 的 `references/*-parameters.md`、Skill 导航与元数据、文档契约测试。
  关键逻辑/决策：以官方 OpenAPI 为字段主档，补充 CLI/CLM 的本地校验和差异；逐字段记录类型、必填性、枚举与约束，动态主数据字段明确要求查询租户配置。

- 2026-08-10
  变更摘要：修复审批矩阵规则行搜索无法传递分页参数的问题。
  涉及文件/模块：`internal/cli` 的 rule table row search、帮助、请求与 Skill 回归测试。
  关键逻辑/决策：新增 `--page-size` / `--page-token`，复用分页 Query 构造并保持筛选 JSON body 不变。

- 2026-08-10
  变更摘要：修复 CLI 命令日志明文暴露 app secret 等敏感参数的问题。
  涉及文件/模块：`internal/cli` 命令入口、skills 子命令日志及回归测试。
  关键逻辑/决策：统一脱敏 secret/token/password/authorization、header 和 `--data`，兼容 `--flag value`、`--flag=value` 及下划线参数名。

- 2026-08-10
  变更摘要：修正 `release` 合入 `cli-p3` 后的文档与 Skill 冲突拼接问题。
  涉及文件/模块：`docs/cli-command-reference.md`、`skills/contract-cli-contract`、`internal/cli/command_reference_doc_test.go`。
  关键逻辑/决策：保留 release 的付款/审批字段资料与 cli-p3 的补齐命令；审批章节和能力清单只保留一份，授权继续由 `contract authorization grant` 处理，新增冲突回归测试。

- 2026-07-06
  变更摘要：补齐 P2 付款和审批命令的 `--input-file` 请求体字段参考，并以 CLM 后端代码口径覆盖飞书文档差异。
  涉及文件/模块：`skills/contract-cli-payment/references/*-fields.md`、`skills/contract-cli-contract/references/approval-fields.md`、`skills/contract-cli-* /SKILL.md`、`internal/cli/contract_skill_reference_test.go`
  关键逻辑/决策：付款申请、付款计划、付款记录和审批字段按 DTO/Service/Swagger 校验整理；明确 `has_invoice` 为 boolean、付款状态无 `9`、付款记录部门字段使用 `department_lark_id`；移除 skill frontmatter 中不符合校验脚本的 `version`。

- 2026-07-06
  变更摘要：统一 P2 付款与审批文档、skill 的身份命名，用户可见新示例从 `bot` 收敛为 `app`。
  涉及文件/模块：`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`docs/cli-p2-提示词.md`、`skills/contract-cli-payment`、`skills/contract-cli-contract`、`skills/contract-cli-shared`、`internal/cli/command_reference_doc_test.go`
  关键逻辑/决策：保留 README/auth/旧配置迁移中的 `--as bot` 兼容说明；新增文档契约测试只约束 P2 新命令和 skill 不再出现 `bot-only` / `--as bot` 主推文案。

- 2026-06-03
  变更摘要：修复 P2 分支在 app 身份改名后的编译失败。
  涉及文件/模块：`internal/cli/payment_command.go`、`internal/cli/contract_command.go`、`internal/openplatform/payment/service.go`、`internal/openplatform/contract/service.go`、相关 CLI/service 测试
  关键逻辑/决策：将付款和审批命令残留的 `IdentityPolicyBotOnly` / `IdentityBot` fixture 迁移为现有 `IdentityPolicyAppOnly` / `IdentityApp`，保持 app-only 身份限制语义并恢复生产入口编译。

- 2026-06-05
  变更摘要：修复生产验证发现的输出精度、授权状态和开放平台命令契约问题。
  涉及文件/模块：`internal/output`、`internal/cli`、`README.md`、`docs/*`、`skills/contract-cli-*`
  关键逻辑/决策：JSON 渲染改用 `UseNumber` 保留大整数精度；user token 过期时 `auth status` 显示 `expired`；固定汇率 get 改传 query `date`，event 分页限制 `10-50`，MDM 写接口强制 `--user-id` 并校验 create/update 生成编码规则；同步 help、skill 和验收文档。

- 2026-06-05
  变更摘要：修正 release 合并前发现的 skill 口径不一致问题。
  涉及文件/模块：`skills/contract-cli-contract`、`skills/contract-cli-shared`、`skills/contract-cli-api-call`、`internal/cli/openapi_gap_skill_test.go`
  关键逻辑/决策：补齐合同 commands reference 的新增 app-only 命令清单，拆分协商 search/file/download 的快速路由，扩展 shared 合同入口，并将禁用 api-call agent 元数据改为不可隐式触发；新增契约测试防止回退。

- 2026-06-05
  变更摘要：收敛 P3 新增命令的 README、命令文档、Agent skills 和 help 测试口径。
  涉及文件/模块：`README.md`、`docs/cli-command-reference.md`、`skills/contract-cli-*`、`internal/cli/*_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：补充回归测试覆盖新增 skill 清单、app-only 摘要、shared/contract/mdm-fields skill 旧口径和新增 help topic；修正审批/授权、MDM 写入、event/rule/payment 等已实现命令的引导，避免 Agent 误判未覆盖。

- 2026-06-03
  变更摘要：将新增开放平台结构化命令的应用身份口径从 bot 收敛为 app。
  涉及文件/模块：`internal/cli`、`internal/openplatform/payment`、`internal/openplatform/contract`、`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`skills/contract-cli-*`
  关键逻辑/决策：把新增命令的身份策略、help、命令文档和 skill 示例统一改为 `--as app` / app-only；仅保留旧 `--as bot`、旧环境变量和旧配置字段作为兼容入口。

- 2026-06-03
  变更摘要：补齐开放平台法人实体按编码查询命令。
  涉及文件/模块：`internal/cli`、`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`skills/contract-cli-mdm-legal`、`skills/contract-cli-shared`
  关键逻辑/决策：新增 app-only `contract-cli mdm legal get --code <code>`，走 `GET /open-apis/mdm/v1/legal_entities`，`--code` 映射 query `legalEntity`；删除旧独立子命令形态，按 TDD 覆盖 endpoint、身份拒绝、参数互斥、help 和 skill 文档。

- 2026-06-02
  变更摘要：补齐一批 app-only 开放平台结构化命令，并同步 help、命令文档和 Agent skills。
  涉及文件/模块：`internal/cli`、`docs/cli-command-reference.md`、`skills/contract-cli-*`、CLI 测试、`docs/ai-changes.md`
  关键逻辑/决策：新增 `contract search-v2/field/sign/sign-url/form/authorization/esign/share batch/cooperation search/cooperation file`、`mdm vendor/legal` 写入与扩展查询（含法人实体按编码查询）、固定汇率、主数据文件下载、事件出口 IP、审批矩阵规则表命令；全部按 TDD 验证方法/路径/query/body/下载和 app-only 身份，新增 skill 覆盖测试。

- 2026-05-27
  变更摘要：收敛 profile 和授权状态输出，避免展示开放平台与授权 endpoint 地址。
  涉及文件/模块：`internal/cli/app.go`、`internal/cli/auth_provider.go`、CLI 测试、`docs/cli-test-plan.md`、`skills/auth`
  关键逻辑/决策：`config add` 只输出保存成功；`auth status` 不再输出 Open Platform URL 或 app Token Endpoint；自动打开浏览器的 user 登录成功消息不再回显授权 URL，`--no-open-browser` 仍会在等待回调前打印必要授权链接。

- 2026-05-27
  变更摘要：移除 CLI 内置 dev 环境预设，收敛正式包初始化入口到 prod。
  涉及文件/模块：`internal/cli/app.go`、`internal/cli/help.go`、`internal/cli/auth_provider.go`、`internal/openplatform/client.go`、CLI 测试、`docs/*`、`skills/auth`
  关键逻辑/决策：`config add --env dev` 现在本地拒绝并提示仅支持 `prod`；help、错误提示、命令文档、测试计划和 auth skill 不再引导新建 dev profile；保留既有 profile 按已保存 URL 运行的兼容性。

- 2026-05-12
  变更摘要：实现 P2 付款、付款计划、付款记录和审批管理 CLI 命令。
  涉及文件/模块：`internal/cli`、`internal/openplatform/payment`、`internal/openplatform/contract`、`docs/*`、`skills/contract-cli-*`
  关键逻辑/决策：新增顶层 `payment` app-only 命令和 `contract approval start/get`，按“主 ID 位置参数、父资源 ID 用 flag”解析；POST/PATCH 强制 JSON body，GET/list 拒绝 body，并同步 help、命令文档、测试计划和 payment skill。

- 2026-05-12
  变更摘要：新增 P2 CLI 开发提示词文档，固化付款、付款计划、付款记录和审批命令规划。
  涉及文件/模块：`docs/cli-p2-提示词.md`、`docs/ai-changes.md`
  关键逻辑/决策：按“主操作对象 ID 用位置参数、父资源 ID 用 flag”的命令参数约定记录 P2 开发范围，并明确接口疑问停下询问、skill 同步、TDD 和测试验收要求。

- 2026-05-12
  变更摘要：新增正式发版现状整理文档，汇总生产发版流程、所需信息和当前达成结果。
  涉及文件/模块：`docs/current-production-release-process.md`、`docs/ai-changes.md`
  关键逻辑/决策：以 `scripts/release.sh`、README 和现有 release 测试为基线，明确正式版当前走 GitHub 正式 Release + npm latest、需准备的授权与环境、默认 remote/branch 行为以及后续值得讨论的边界问题。

- 2026-05-12
  变更摘要：全仓收敛旧 profile 示例名，统一使用 `--profile contract`。
  涉及文件/模块：`internal/cli/help.go`、CLI 测试、`docs/*`、`skills/contract-cli-contract`、`skills/contract-cli-shared`
  关键逻辑/决策：用户可见示例、测试命令参数和 profile fixture 从旧 `contract-group` 迁移到 `contract`；命令参考测试保留防回退断言，避免后续文档重新出现旧 profile 名。

- 2026-05-12
  变更摘要：支持 `contract upload-file` 在 user 身份下上传文件。
  涉及文件/模块：`internal/cli/contract_command.go`、`internal/openplatform/contract/service.go`、上传命令测试、帮助与命令文档、contract skills
  关键逻辑/决策：上传接口仍复用 `POST /open-apis/contract/v1/files/upload` 和 multipart 字段，将身份策略从 app-only 调整为 user/app 通用，并补充显式 user 与默认 user 上传测试。

- 2026-04-23
  变更摘要：新增 9 个 app-only 合同命令，覆盖提交、重提、更新、下载、删除、打印、分享记录和协商信息查询。
  涉及文件/模块：`internal/openplatform`、`internal/openplatform/contract`、`internal/cli/contract_command.go`、`internal/cli/help.go`、`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`skills/contract-cli-contract`、`skills/contract-cli-shared`
  关键逻辑/决策：所有新增结构化命令统一 `IdentityPolicyAppOnly`，通用 query 仍由 `CommonQuery` 注入；`download-file` 使用流式下载，默认保存弹窗，脚本环境推荐 `--output-file`，`--raw` 直接写 stdout；`delete` 直接执行不加 `--yes`。

- 2026-04-23
  变更摘要：新增 app 命令开发指南，沉淀后续 app 功能和 skill 文档开发约定。
  涉及文件/模块：`docs/app-command-development-guide.md`、`docs/ai-changes.md`
  关键逻辑/决策：按功能开发、身份路由、通用 query、测试要求、skill 编写和 Definition of Done 组织；补充 skill 版本、业务错误、`user_id` 必填、输出归一化等后续需要团队确认的问题。

- 2026-04-29
  变更摘要：发布前忽略所有层级的 macOS `.DS_Store` 本机文件。
  涉及文件/模块：`.gitignore`、`docs/ai-changes.md`
  关键逻辑/决策：将仅忽略仓库根目录 `/.DS_Store` 调整为全局 `.DS_Store`，避免子目录 Finder 元数据让正式发版脚本误判工作区不干净。

- 2026-04-29
  变更摘要：收敛正式版更新检查示例，并为构建链路开启 `-trimpath`。
  涉及文件/模块：`build.sh`、`Makefile`、`scripts/build-release-assets.sh`、`tests/cli_e2e/smoke.sh`、`tests/release/local-install.sh`、`tests/release/build-flags.sh`、`internal/cli/help.go`、`docs/cli-command-reference.md`
  关键逻辑/决策：正式帮助与命令参考只展示 `update check --channel latest` 示例，避免正式包把 beta 作为默认引导；本地构建、安装和 release assets 构建统一加 `go build/install -trimpath`，降低二进制中泄漏本机源码绝对路径的风险。

- 2026-04-29
  变更摘要：收敛正式包可见的默认环境、profile 与安装文档口径。
  涉及文件/模块：`internal/cli/help.go`、`internal/cli/app.go`、`internal/cli/help_command_test.go`、`internal/cli/command_reference_doc_test.go`、`README.md`、`docs/cli-command-reference.md`、`skills/*`
  关键逻辑/决策：`config add --help` 和命令参考统一改为默认 `prod`、默认 profile `contract`；README 移除 beta 安装入口和本机绝对路径；skills 示例从旧 `contract-group` 收敛到 `contract`，并补测试防止正式包文案回退。

- 2026-04-29
  变更摘要：新增正式版一键发版脚本，并把发布说明补齐到 README。
  涉及文件/模块：`scripts/release.sh`、`tests/release/release-script.sh`、`Makefile`、`README.md`、`docs/ai-changes.md`
  关键逻辑/决策：正式脚本要求稳定语义版本 `x.y.z`，默认执行 `make release-check` 和 `make release-assets`，远端发布时创建 GitHub latest release 并执行 `npm publish --tag latest`；release 脚本检查现在同时覆盖 beta 与正式包 dry-run。

- 2026-04-27
  变更摘要：修复 `contract text` 标准开放平台路由与文本参数默认值。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/cli/contract_command.go`、`internal/cli/command_support.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/mcp_command_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：app 身份下 `contract text` 改为 `GET /open-apis/contract/v1/contracts/{contract_id}/text`；命令可区分参数是否显式传入，默认请求完整文本 `full_text=true`，传 `--offset/--limit` 时自动使用分页模式 `full_text=false` 并保留 `offset=0`。

- 2026-04-24
  变更摘要：新增本地 `contract-cli-beta-release` skill，沉淀 beta 发版流程。
  涉及文件/模块：`~/.codex/skills/contract-cli-beta-release/SKILL.md`、`~/.codex/skills/contract-cli-beta-release/agents/openai.yaml`、`docs/ai-changes.md`
  关键逻辑/决策：skill 固化 `REMOTE=github BRANCH=main scripts/release-beta.sh --version <version> --publish --yes` 流程、npm token 临时注入、半发布恢复和 GitHub/npm 最终校验要求，后续只需提供版本号与 npm key 即可执行。

- 2026-04-24
  变更摘要：修复 beta 发布前 npm 打包检查仍要求禁用 `api call` skill 的问题。
  涉及文件/模块：`package.json`、`tests/release/package-dry-run.sh`、`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`docs/ai-changes.md`
  关键逻辑/决策：npm 包显式排除 `skills/contract-cli-api-call/**`，release dry-run 测试改为禁止禁用 skill 相关文件进入包内；文档同步移除内置安装会安装 `contract-cli-api-call` 的过期描述。

- 2026-04-24
  变更摘要：暂时封住预留的 `api call` 入口，保留实现代码但不对外暴露。
  涉及文件/模块：`internal/cli/app.go`、`internal/cli/help.go`、`internal/cli/api_command_test.go`、`internal/cli/skills_command.go`、`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`skills/contract-cli-*`、`docs/ai-changes.md`
  关键逻辑/决策：`contract-cli api ...` 在 profile、HTTP、update check 前直接返回暂未开放错误；help registry 不再注册 `api` 主题；内置 skills 跳过 `contract-cli-api-call`，并移除该目录的 `SKILL.md`，仅保留禁用说明和历史参考。

- 2026-04-24
  变更摘要：修复 app 身份下 `mdm fields list` 的 `biz_line` 取值与 help/文档不一致问题。
  涉及文件/模块：`internal/openplatform/schema`、`internal/cli/mcp_command_test.go`、`internal/cli/help.go`、`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`skills/contract-cli-mdm-fields/*`、`docs/ai-changes.md`
  关键逻辑/决策：app 路由下允许继续传 `legal_entity` 并映射为后端实际值 `legalEntity`；`vendor_risk` 当前仅 user/MCP 支持，app 下改为本地明确报错且不发 HTTP；同步更新 help、测试计划和 skill 示例。

- 2026-04-24
  变更摘要：修复 `auth login --as user --no-open-browser` 超时前不输出授权 URL 的问题。
  涉及文件/模块：`internal/cli/auth_provider.go`、`internal/cli/app.go`、`internal/cli/auth_provider_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：user OAuth 登录在构造授权 URL 后、等待本地 callback 前立即将 URL 写入 stdout；正常自动打开浏览器的路径保持原有成功输出；新增可注入 callback 等待接口，测试无需真实监听端口即可覆盖超时场景。

- 2026-04-22
  变更摘要：将 user OAuth 的 `resource` 调整为可选，dev 预设不再写入旧 Higress 内网 resource。
  涉及文件/模块：`internal/cli/app.go`、`internal/cli/app_test.go`、`internal/oauth/login.go`、`internal/oauth/login_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：`config add --env dev` 现在只依赖公开的 authorization server metadata URL 初始化 user OAuth；`BuildAuthorizationURL` 和 `ExchangeAuthorizationCode` 在 resource 为空时不再发送 `resource=` 参数，保留非空 resource 的兼容行为。

- 2026-04-22
  变更摘要：为开放平台通用 query 参数补齐 `user_id_type=user_id` 默认值。
  涉及文件/模块：`internal/cli/command_support.go`、`internal/cli/*_test.go`、`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`skills/contract-cli-*`、`docs/ai-changes.md`
  关键逻辑/决策：结构化命令和 `api call` 在构造 `RequestContext.CommonQuery` 时默认追加 `user_id_type=user_id`；显式传 `--user-id-type` 时覆盖默认值，`--user-id` 仍保持传了才带；MCP user-only 固定 query 仍由 `IdentityPolicyUserOnly` 保护，不会被通用参数覆盖。

- 2026-04-21
  变更摘要：增强 beta 发布脚本对已存在 GitHub Release 的幂等修正能力。
  涉及文件/模块：`scripts/release-beta.sh`、`tests/release/release-beta-script.sh`、`docs/ai-changes.md`
  关键逻辑/决策：发布脚本在创建或覆盖上传 GitHub Release assets 后，统一执行 `gh release edit --prerelease --latest=false`，确保 beta release 即使被重跑或手动创建过也会保持预发布状态；dry-run 测试新增该命令断言。

- 2026-04-21
  变更摘要：修复 npm 发布元信息测试锁死历史版本号导致新 beta 版本无法发布的问题。
  涉及文件/模块：`internal/cli/package_json_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：`package.json` 的包名、registry、下载地址模板和仓库地址仍保持精确断言；版本号从固定 `0.1.0-beta.1` 改为校验合法 semver，避免每次发版都需要同步修改测试常量。

- 2026-04-21
  变更摘要：新增 beta 版本一键发布脚本和 dry-run 发布脚本测试。
  涉及文件/模块：`scripts/release-beta.sh`、`tests/release/release-beta-script.sh`、`Makefile`、`.gitignore`、`README.md`、`docs/ai-changes.md`
  关键逻辑/决策：发布脚本要求显式 `--version <x.y.z-beta.n>`，默认只做本地准备，`--dry-run` 不改文件，真正远端发布必须传 `--publish --yes`；远端链路按 GitHub Release 附件先于 `npm publish --tag beta` 的顺序执行；默认推送当前分支，且本地 tag 已存在并指向 HEAD 时允许恢复重跑。

- 2026-04-21
  变更摘要：为当前全部已支持命令补齐统一 `--help` / `help <command>` 本地帮助系统。
  涉及文件/模块：`internal/cli/help.go`、`internal/cli/app.go`、`internal/cli/update_command.go`、`internal/cli/help_command_test.go`、`README.md`、`docs/cli-command-reference.md`、`docs/ai-changes.md`
  关键逻辑/决策：新增静态 help registry，不引入 Cobra、不改现有业务 parser；`App.Run` 在日志、版本检查和命令分发前拦截 help，支持顶层、命令组、叶子命令和带位置参数的 leaf help；help 只本地渲染，不读取 profile、不发 HTTP、不写 update cache，并保留旧命令别名拒绝行为。

- 2026-04-21
  变更摘要：补充 CLI 测试文档中的版本升级、Agent skills 单独安装和 app 文件上传专项验收内容。
  涉及文件/模块：`docs/cli-test-plan.md`、`docs/ai-changes.md`
  关键逻辑/决策：新增三个独立专项模块，分别覆盖 `update check` 手动/自动升级提示、`npx skills add qfeius/contract-cli -y -g` 通用安装与 CLI 内置兜底安装、`contract upload-file --as app` 的 multipart 上传和负向参数校验；同步修正 app 验收标准不再按旧十四条表述。

- 2026-04-21
  变更摘要：新增 app 身份下的 `contract-cli contract upload-file` 文件上传命令。
  涉及文件/模块：`internal/openplatform`、`internal/openplatform/contract`、`internal/cli/contract_command.go`、`docs/cli-command-reference.md`、`docs/cli-command-design.md`、`docs/cli-test-plan.md`、`README.md`、`skills/contract-cli-*`、`docs/ai-changes.md`
  关键逻辑/决策：新增 `IdentityPolicyAppOnly` 并扩展 `openplatform.Request.BodyReader` 支持流式上传；合同 service 使用 `multipart/form-data` 发送 `file_name/file_type/file`，CLI 只做本地文件存在、普通文件、`<=200MB` 和必填参数校验；`--file` 正式用于真实二进制上传，JSON 请求体继续使用 `--input-file`。

- 2026-04-21
  变更摘要：将 Agent skills 推荐安装方式调整为通用 `npx skills add qfeius/contract-cli -y -g`。
  涉及文件/模块：`README.md`、`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`internal/cli/command_reference_doc_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：文档主推通用 `skills` installer 从 GitHub 仓库安装 `skills/` 目录，以覆盖 Codex、Cursor、Trae、Claude Code 等多类 Agent 环境；`contract-cli skills install` 保留为随 npm/二进制分发的离线兜底方式，并补文档契约测试防止推荐安装命令漂移。

- 2026-04-20
  变更摘要：新增 CLI 版本检查与交互终端升级提示。
  涉及文件/模块：`internal/update`、`internal/cli/app.go`、`internal/cli/update_command.go`、`README.md`、`docs/cli-command-reference.md`、`docs/cli-test-plan.md`、`docs/ai-changes.md`
  关键逻辑/决策：通过 npm registry packument 检查 `@qfeius/contract-cli` 的 dist-tag，预发布版本默认看 `beta`、稳定版本默认看 `latest`；新增 `contract-cli update check` 手动入口，普通命令在交互终端下最多每 30 分钟自动检查一次，失败会缓存检查时间且不阻断原命令；`dev`、`unknown` 和 git hash 版本会跳过检查，并支持 `CONTRACT_CLI_NO_UPDATE_CHECK=1` 关闭自动检查。

- 2026-04-20
  变更摘要：新增面向 QA 和发布验收的 CLI 测试文档。
  涉及文件/模块：`docs/cli-test-plan.md`、`README.md`、`docs/ai-changes.md`
  关键逻辑/决策：测试文档按安装、profile 初始化、user/app 授权与登出切换、app 全量结构化命令、user 全量结构化命令、`api call` 兜底、输出格式、发布安装回归和常见问题组织；明确 app 当前十四条结构化业务命令、user-only 枚举命令、MCP 固定 query 保护和 npm beta 安装验收路径。

- 2026-04-20
  变更摘要：切换 npm beta 发布配置，并新增 GitHub Release 附件构建脚本。
  涉及文件/模块：`package.json`、`scripts/build-release-assets.sh`、`Makefile`、`README.md`、`internal/cli/package_json_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：版本固定为 `0.1.0-beta.1`，`publishConfig` 指向 `https://registry.npmjs.org/` 且 public；`downloadBaseURLTemplate` 指向 GitHub Releases 的 `v{version}`；新增 `make release-assets` 生成 npm 安装脚本所需的多平台压缩包和 `checksums.txt`，便于直接上传到 `v0.1.0-beta.1` Release。

- 2026-04-20
  变更摘要：同步 npm 发布元信息到正式 GitHub 仓库，并校正 scoped 包的 npx 示例。
  涉及文件/模块：`package.json`、`README.md`、`internal/cli/package_json_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：`repository.url` 改为 `git+https://github.com/qfeius/contract-cli.git`；新增发布元信息测试，固定包名 `@qfeius/contract-cli` 与正式仓库地址；README 的 npx 示例改为 `npx @qfeius/contract-cli --version`，避免用户按旧非 scoped 包名安装。

- 2026-04-20
  变更摘要：新增发布链路测试脚本，并接入 `make release-check`。
  涉及文件/模块：`Makefile`、`tests/release/package-dry-run.sh`、`tests/release/local-install.sh`、`tests/cli_e2e/README.md`、`README.md`、`docs/ai-changes.md`
  关键逻辑/决策：`package-dry-run.sh` 校验 `node --check`、`npm pack --dry-run` 和包内必须/禁止文件；`local-install.sh` 先构造本地 release archive，再通过 `file://` 模拟 npm postinstall 下载二进制，随后验证 `--version`、`skills list`、`skills install`；脚本统一使用临时 npm/Go cache，避免本机缓存权限污染发布前检查。

- 2026-04-20
  变更摘要：新增 `contract-cli skills list/install`，支持列出并安装随 CLI 分发的 Codex skills。
  涉及文件/模块：`cmd/contract-cli`、`internal/cli`、`skills/embed.go`、`package.json`、`README.md`、`docs/cli-command-reference.md`、`tests/cli_e2e/smoke.sh`、`docs/ai-changes.md`
  关键逻辑/决策：通过 Go embed 将 `skills/*` 打进二进制，`skills list` 读取内置 skill 元数据，`skills install` 默认安装到 `$CODEX_HOME/skills` 或 `~/.codex/skills`，支持 `--target` 和 `--force`；npm 打包清单只包含 skill 文档资源，二进制 smoke 改为捕获输出后匹配，避免 `pipefail + grep -q` 的 SIGPIPE 误失败。

- 2026-04-17
  变更摘要：修复 user-only MCP 请求中固定 query 被通用 `--user-id/--user-id-type` 覆盖的问题。
  涉及文件/模块：`internal/openplatform/client.go`、`internal/openplatform/client_test.go`、`internal/cli/mcp_command_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：`Client.Do()` 先解析生效的 `IdentityPolicy` 再合并 query；对 `IdentityPolicyUserOnly` 保留 `request.Query` 现有 key，仅补入 `CommonQuery` 中缺失的参数，从而保护 MCP 固定的 `user_id_type=user_id`；对 `IdentityPolicyAny` 继续保持通用 query 可覆盖同名请求参数的既有语义，并补充对应回归测试。

- 2026-04-17
  变更摘要：为 `contract-cli mdm fields list` 增加按身份自动分流的 app 查询字段配置能力。
  涉及文件/模块：`internal/openplatform/schema/service.go`、`internal/openplatform/schema/service_test.go`、`internal/cli/schema_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-fields/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm fields list --biz-line <...>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/config/config_list`，`app` 改走开放平台标准接口 `GET /open-apis/mdm/v1/config/config_list`；参考生产文档按显示文本采用 `config/config_list` 路径，同时记录超链接误指到 `vendors` 的瑕疵，并继续沿用 `biz_line` 的 query 透传映射，不做本地校验。

- 2026-04-17
  变更摘要：为 `contract-cli mdm legal get` 增加按身份自动分流的 app 查询法人主体详情能力。
  涉及文件/模块：`internal/openplatform/entity/service.go`、`internal/openplatform/entity/service_test.go`、`internal/cli/vendor_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-legal/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm legal get <legal-entity-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}`，`app` 改走开放平台标准接口 `GET /open-apis/mdm/v1/legal_entities/{legal_entity_id}`；由于生产文档同时把 `legal_entity_id` 写在查询参数表里，这次按确认方案采用“path + query 双带 `legal_entity_id`”的保守实现，并继续按共享约定透传 `--user-id-type` / `--user-id`，不做本地校验。

- 2026-04-17
  变更摘要：为 `contract-cli mdm legal list` 增加按身份自动分流的 app 查询法人主体列表能力。
  涉及文件/模块：`internal/openplatform/entity/service.go`、`internal/openplatform/entity/service_test.go`、`internal/cli/vendor_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-legal/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm legal list` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/legal_entities`，`app` 改走开放平台标准接口 `GET /open-apis/mdm/v1/legal_entities/list_all`；参考生产文档按显示文本采用 `legal_entities/list_all` 路径，同时记录文档超链接误指到 `vendors` 的瑕疵，并继续沿用当前 `legalEntity/page_size/page_token` 的 query 透传映射，不做本地校验。

- 2026-04-17
  变更摘要：为 `contract-cli mdm vendor get` 增加按身份自动分流的 app 查询交易方详情能力。
  涉及文件/模块：`internal/openplatform/mdmvendor/service.go`、`internal/openplatform/mdmvendor/service_test.go`、`internal/cli/vendor_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-vendor/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm vendor get <vendor-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/vendors/{vendor_id}`，`app` 改走开放平台标准接口 `GET /open-apis/mdm/v1/vendors/{vendor_id}`；参考生产文档仅把 `user_id_type` 视为 app 文档显式列出的查询参数，但 CLI 继续按共享约定透传 `--user-id-type` / `--user-id`，不做本地校验。

- 2026-04-17
  变更摘要：为 `contract-cli mdm vendor list` 增加按身份自动分流的 app 查询交易方列表能力。
  涉及文件/模块：`internal/openplatform/mdmvendor/service.go`、`internal/openplatform/mdmvendor/service_test.go`、`internal/cli/vendor_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-shared/SKILL.md`、`skills/contract-cli-mdm-vendor/*`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract-cli mdm vendor list` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/vendors`，`app` 改走开放平台标准接口 `GET /open-apis/mdm/v1/vendors`；依据生产文档保留 `vendor` 查询参数名，CLI 继续沿用 `--name -> vendor` 的透传映射，不在本地改名或做额外校验，`mdm vendor get` 仍保持 user-only。

- 2026-04-17
  变更摘要：为 `contract-cli contract template instantiate` 增加按身份自动分流的 app 创建模板实例能力。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract template instantiate` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/template_instances`，`app` 改走开放平台标准接口 `POST /open-apis/contract/v1/template_instances`；按照生产文档仅保留 `user_id_type` 作为 query 参数语义，并要求由调用方自行在 body 中提供 `create_user_id`，CLI 不做本地必填校验。

- 2026-04-17
  变更摘要：为 `contract-cli contract template get` 增加按身份自动分流的 app 查看模板详情能力。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract template get <template-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/templates/{template_id}`，`app` 改走开放平台标准接口 `GET /open-apis/contract/v1/templates/{template_id}`；查询参数继续沿用现有透传约定，不对生产文档中标注的 `user_id/user_id_type` 做本地必填校验，`template instantiate` 继续保持 user-only。

- 2026-04-17
  变更摘要：为 `contract-cli contract template list` 增加按身份自动分流的 app 列出模板能力。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract template list` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/templates`，`app` 改走开放平台标准接口 `GET /open-apis/contract/v1/templates`；查询参数仍沿用现有透传约定，不对生产文档中标注的 `category_number/user_id/user_id_type` 做本地必填校验，`template get/instantiate` 继续保持 user-only。

- 2026-04-17
  变更摘要：为 `contract-cli contract category list` 增加按身份自动分流的 app 查询合同分类能力。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract category list` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/contract_categorys`，`app` 改走开放平台标准接口 `GET /open-apis/contract/v1/contract_categorys`；`lang` 仍作为 query 参数透传，未放开其余模板/枚举等 user-only 合同命令。

- 2026-04-17
  变更摘要：为 `contract-cli contract create` 增加按身份自动分流的 app 创建合同能力，并补齐 `create_user_id` 的命令/skill 说明。
  涉及文件/模块：`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/cli/command_reference_doc_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract create` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/contracts`，`app` 改走开放平台标准接口 `POST /open-apis/contract/v1/contracts`；CLI 仍旧透传原始 JSON body，不替用户补 `create_user_id`，只在命令文档和字段主档里明确它是 app 创建时必须自行携带的请求体字段。

- 2026-04-17
  变更摘要：将 `--user-id-type` / `--user-id` 收敛为开放平台通用 query 参数，对结构化命令与 `api call` 统一透传，并同步纠正 `sync-user-groups` / `text` 的 app 底层路径。
  涉及文件/模块：`internal/openplatform/client.go`、`internal/openplatform/client_test.go`、`internal/cli/command_support.go`、`internal/cli/api_command.go`、`internal/cli/api_command_test.go`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：通过 `RequestContext.CommonQuery` 在 `Client.Do()` 统一合并通用 query，显式传入的 `user_id_type/user_id` 会覆盖命令自身已有同名 query；结构化命令和 `api call` 只负责解析参数，不再对 `user` / `app` 做必填、默认值或禁用校验；同时把 `contract sync-user-groups` 的 app 路由纠正为 `POST /open-apis/contract/v1/contracts/user-groups/sync`，把 `contract text` 的 app 路由纠正为 `POST /open-apis/contract/v1/contracts/{contract_id}/text`。

- 2026-04-17
  变更摘要：为 `contract-cli contract text` 增加按身份自动分流的 app 获取合同文本能力。
  涉及文件/模块：`internal/openplatform/contract`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract text <contract-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 `/open-apis/contract/v1/mcp/contracts/{contract_id}/text?user_id_type=user_id&...`，`app` 使用同一路径但不再追加 `user_id_type`，仅透传 `full_text/offset/limit` 查询参数并使用 `tenant_access_token` 调用；仅这条命令对 `/open-apis/contract/v1/mcp/contracts/{contract_id}/text` 单独放开 app 访问，其余未改造的 `/contract/v1/mcp/` 结构化命令仍保持既有约束。

- 2026-04-17
  变更摘要：为 `contract-cli contract sync-user-groups` 增加按身份自动分流的 app 同步用户组能力。
  涉及文件/模块：`internal/openplatform/contract`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract sync-user-groups` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 `/open-apis/contract/v1/mcp/contracts/user-groups/sync?user_id_type=user_id`，`app` 仍使用同一路径但不再追加 user 侧查询参数，只使用 `tenant_access_token` 完成调用；仅这条命令对 `/open-apis/contract/v1/mcp/contracts/user-groups/sync` 单独放开 app 访问，其余未改造的 `/contract/v1/mcp/` 结构化命令仍保持 user-only。

- 2026-04-17
  变更摘要：为 `contract-cli contract get` 增加按身份自动分流的 app 合同详情能力，并把统一的 `--user-id-type` / `--user-id` 约定扩展到详情命令。
  涉及文件/模块：`internal/openplatform/contract`、`internal/cli/contract_command.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract get <contract-id>` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/contracts/{contract_id}`，`app` 改走开放平台标准接口 `/open-apis/contract/v1/contracts/{contract_id}`，并同样按“第二种方式”追加 `user_id_type/user_id` 查询参数；仅 `contract get --as app` 真正消费 `--user-id-type` / `--user-id`，其中 `--user-id` 必填、`--user-id-type` 默认 `user_id`，而 `--as user` 传入这两个参数会直接报错；其余结构化命令仍保持 user-only。

- 2026-04-16
  变更摘要：为 `contract-cli contract search` 增加按身份自动分流的 app 搜索能力，并引入统一的 `--user-id-type` / `--user-id` 参数约定。
  涉及文件/模块：`internal/openplatform/contract`、`internal/cli/contract_command.go`、`internal/cli/command_support.go`、`internal/cli/mcp_command_test.go`、`internal/openplatform/contract/service_test.go`、`docs/cli-command-reference.md`、`skills/contract-cli-contract/*`、`skills/contract-cli-shared/SKILL.md`、`docs/ai-changes.md`
  关键逻辑/决策：保持 `contract search` 命令面不变，运行时按当前 token 身份路由；`user` 继续走 MCP `/open-apis/contract/v1/mcp/contracts/search`，`app` 改走开放平台标准接口 `/open-apis/contract/v1/contracts/search`，并按“第二种方式”追加 `user_id_type/user_id` 查询参数；仅 `contract search --as app` 真正消费 `--user-id-type` / `--user-id`，其中 `--user-id` 必填、`--user-id-type` 默认 `user_id`，而 `--as user` 传入这两个参数会直接报错；响应继续原样透传，不做 user/app 结果归一化。

- 2026-04-16
  变更摘要：新增一份与当前代码实现对齐的 CLI 命令总览文档，并补轻量契约测试防止文档漂移。
  涉及文件/模块：`docs/cli-command-reference.md`、`README.md`、`internal/cli/command_reference_doc_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：从 `internal/cli` 当前真实命令树反向整理 `config/auth/version/api/contract/mdm` 的命令矩阵、参数约定、输出约定和身份支持现状；明确当前结构化业务命令全部只支持 `--as user`，而 app 业务接口后续优先通过结构化命令验证；新增文档契约测试要求新文档必须覆盖所有已支持命令和 app/user 边界，避免后续扩展时清单失真。

- 2026-04-15
  变更摘要：将内部主数据交易方 service 包从 `internal/openplatform/vendor` 重命名为 `internal/openplatform/mdmvendor`，规避 JetBrains 对 `vendor` 包路径的错误识别。
  涉及文件/模块：`internal/cli/vendor_command.go`、`internal/openplatform/mdmvendor/*`、`docs/ai-changes.md`
  关键逻辑/决策：Go 工具链可正常编译，但 IDE 对终止于 `/vendor` 的内部包索引不稳定，导致仅该导入路径持续爆红；外部 CLI 命令保持 `mdm vendor` 不变，只调整内部实现目录与测试导入路径，绕开 vendoring 语义歧义。

- 2026-04-15
  变更摘要：修正本地 `.idea` 模块内容根目录错误指向 `.idea/` 本身，导致 Go 源码在 IDE 中整体爆红的问题。
  涉及文件/模块：`.idea/contract-cli.iml`、`docs/ai-changes.md`
  关键逻辑/决策：排查发现本地 IntelliJ 对 `.iml` 里的 `$MODULE_DIR$` 宏实际按项目根目录解析，而不是按 `.idea/` 目录解析；因此上一版将 content root 调整为 `file://$MODULE_DIR$/..` 会把项目根错误扩大到 `/Users/lyy`。现已改回 `file://$MODULE_DIR$`，并显式排除 `.idea/bin/dist`，让模块根稳定回到 `/Users/lyy/contract-cli`。

- 2026-04-15
  变更摘要：补齐源码构建、版本注入、GoReleaser 和 npm/npx 薄包装发布脚手架。
  涉及文件/模块：`internal/build`、`internal/cli/app.go`、`internal/cli/app_test.go`、`build.sh`、`Makefile`、`package.json`、`scripts/*`、`.goreleaser.yml`、`.github/workflows/release.yml`、`README.md`、`CHANGELOG.md`、`LICENSE`、`tests/cli_e2e/*`、`.gitignore`、`docs/ai-changes.md`
  关键逻辑/决策：新增 `contract-cli version` / `--version` 并统一从 `internal/build` 读取 `Version/Commit/Date`；`build.sh`、`Makefile`、GoReleaser 和 npm 本地源码回退构建全部复用同一套 ldflags；npm 包采用 thin wrapper 设计，优先从可配置的 `downloadBaseURLTemplate` 下载预编译产物，若当前是源码仓库则回退到本地 `go build`；同时补齐 README/CHANGELOG/UNLICENSED 许可证与 e2e smoke 脚本，形成最小可用的发布骨架。

- 2026-04-15
  变更摘要：移除未参与运行时链路的 `ServerURL` 配置，以及 `config add` 的 `--server-url` 入口。
  涉及文件/模块：`internal/config/store.go`、`internal/config/store_test.go`、`internal/cli/app.go`、`internal/cli/app_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：确认当前 `user` 登录走 `resource + authorization/token/registration endpoint`，`app` 登录走 app token endpoint，开放平台业务请求走 `open_platform_base_url`，`ServerURL` 仅剩保存和展示作用；因此删除 profile 中的 `server_url` 字段、`config add --server-url` flag，以及相关 stdout/status 输出；新增回归测试确保配置文件不再持久化该字段且旧 flag 被拒绝。

- 2026-04-15
  变更摘要：纠正 `config add --env dev` 的默认鉴权预设，把 user OAuth 与 app 直调 token 链路彻底分开。
  涉及文件/模块：`internal/cli/app.go`、`internal/cli/app_test.go`、`internal/oauth/discovery.go`、`internal/oauth/discovery_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：确认 `--as app` 使用 `https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal`；`--as user` 不再错误地复用 `dev-open` 的 `.well-known/oauth-protected-resource`，而是默认通过公开的 `https://dev-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract` 加载 OAuth server metadata，并配合固定 `resource=http://higress-gateway.higress-system/mcp-servers` 生成 user 登录配置；新增回归测试覆盖该默认链路。

- 2026-04-15
  变更摘要：修复 `config add --env dev` 默认预设使用集群内 Higress 地址导致本机发现链路易出现 502 的问题。
  涉及文件/模块：`internal/cli/app.go`、`internal/cli/app_test.go`、`docs/ai-changes.md`
  关键逻辑/决策：新增回归测试，要求默认 dev 预设走公共 `https://dev-open.qtech.cn` 的 `mcp-servers/contract-group` 和 `/.well-known/oauth-protected-resource`；将 `resolveEnvironment("dev")` 中原本的 `http://higress-gateway.higress-system/...` 集群内地址替换为公共 HTTPS 地址，避免本机 `config add` 默认链路命中 502。

- 2026-04-15
  变更摘要：将主数据命令树进一步统一为 `contract-cli mdm <vendor|legal|fields> ...`，其中字段配置收口为 `mdm fields list`。
  涉及文件/模块：`internal/cli`、`internal/cli/*_test.go`、`docs/cli-command-design*.md`、`skills/contract-cli-shared`、`skills/contract-cli-mdm-vendor`、`skills/contract-cli-mdm-legal`、`skills/contract-cli-mdm-fields`、`docs/ai-changes.md`
  关键逻辑/决策：新增 `mdm` 一级命令并作为主数据唯一入口，`vendor`、`legal`、`fields` 下降为二级资源；`fields` 再显式使用 `list` 子命令，统一成“一级领域 + 二级资源 + 三级动作”的用户心智；移除 `mdm-vendor`、`mdm-legal`、`mdm-fields` 旧入口；同步更新结构化命令测试、skill 示例、参数附录与设计文档中的命令写法。

- 2026-04-15
  变更摘要：将主数据命令统一重命名为 `mdm-vendor`、`mdm-legal`、`mdm-fields`，并同步把对应 skill 目录改成新命名。
  涉及文件/模块：`internal/cli`、`internal/openplatform`、`docs/cli-command-design*.md`、`skills/contract-cli-shared`、`skills/contract-cli-mdm-vendor`、`skills/contract-cli-mdm-legal`、`skills/contract-cli-mdm-fields`、`docs/ai-changes.md`
  关键逻辑/决策：移除旧的 `vendor`、`entity`、`schema fields` 顶层命令名，不保留别名；CLI 用法、结构化命令测试、skill 元数据和设计文档统一改为 `mdm-vendor`、`mdm-legal`、`mdm-fields`；同时将 skill 目录从 `contract-cli-vendor|entity|schema` 重命名为 `contract-cli-mdm-vendor|mdm-legal|mdm-fields`，并修正相对链接；主文档和会议版中仍作为请求体输入的 `--file` 示例同步改为 `--input-file`。

- 2026-04-14
  变更摘要：将 `vendor`、`entity`、`schema`、`api call` 四组 skill 统一重构为“主 guide + 规则/参数附录 + 命令示例”的阅读结构。
  涉及文件/模块：`skills/contract-cli-mdm-vendor`、`skills/contract-cli-mdm-legal`、`skills/contract-cli-mdm-fields`、`skills/contract-cli-api-call`、`skills/contract-cli-shared`、`docs/ai-changes.md`
  关键逻辑/决策：参考合同模块的阅读路径，把其他接口 skill 也拆成“先选场景，再查参数/规则，最后抄示例”的结构；新增 vendor/entity 参数映射附录、schema biz-line 附录和 api call 规则附录，并在 shared skill 中统一说明各模块的新阅读方式。

- 2026-04-14
  变更摘要：将 `contract create` skill 字段文档重构为“主文档 + 字段树附录 + 枚举附录”三段式结构。
  涉及文件/模块：`skills/contract-cli-contract/SKILL.md`、`skills/contract-cli-contract/references/create-contract-fields.md`、`skills/contract-cli-contract/references/create-contract-field-tree.md`、`skills/contract-cli-contract/references/create-contract-enums.md`、`docs/ai-changes.md`
  关键逻辑/决策：主文档改为场景配方与阅读导航，不再用单一平铺大表；新增 JSON Path 字段树附录承接全部顶层与嵌套字段；新增枚举附录承接 code 型字段和值域说明；合同总 skill 明确阅读顺序为“场景 -> 字段树 -> 枚举值”，整体不再依赖 `mcp.yaml` 作为说明来源。

- 2026-04-14
  变更摘要：把 `contract create` 字段参考补成独立主档，明确列出全部顶层字段和嵌套字段，不再依赖 `mcp.yaml` 兜底说明。
  涉及文件/模块：`skills/contract-cli-contract/SKILL.md`、`skills/contract-cli-contract/references/create-contract-fields.md`、`docs/ai-changes.md`
  关键逻辑/决策：重写 `create-contract-fields.md`，补齐 `create-contracts` 的全部字段、条件必填、文件 id 约束、变更/终止规则、嵌套对象说明和示例；合同总 skill 明确该 reference 已是 `contract create` 的完整参数来源。

- 2026-04-14
  变更摘要：补充 `contract-cli contract create` 的 skill 字段参考，并在合同总 skill 中增加明确入口。
  涉及文件/模块：`skills/contract-cli-contract/SKILL.md`、`skills/contract-cli-contract/references/create-contract-fields.md`、`docs/ai-changes.md`
  关键逻辑/决策：为 `contract create` 新增专门的字段参考，说明 CLI 参数、文件正文/模板实例两种常见路径、顶层核心字段、条件必填和最小 JSON 示例；总 skill 明确引导在需要具体字段说明时优先读取该 reference，并强调当前实现只是透传请求体、不做本地字段校验。

- 2026-04-14
  变更摘要：新增按命令模块拆分的 contract-cli skill 文档，并修正现有 auth skill 以匹配当前 app token 实现。
  涉及文件/模块：`skills/auth`、`skills/contract-cli-shared`、`skills/contract-cli-contract`、`skills/contract-cli-mdm-vendor`、`skills/contract-cli-mdm-legal`、`skills/contract-cli-mdm-fields`、`skills/contract-cli-api-call`、`docs/ai-changes.md`
  关键逻辑/决策：参考 `lark-sheets` 风格把 skill 拆成共享约定 + 业务模块；每个模块补 `SKILL.md`、`agents/openai.yaml` 和按需 `references/commands.md`；共享强调 `contract/v1/mcp` 只支持 `--as user`、请求体统一走 `--input-file`；同步修正 `auth` skill 中 app 已支持 `tenant_access_token` 兑换、状态枚举和 logout 仅清 token 的真实语义。

- 2026-04-14
  变更摘要：实现 `mcp.yaml` 驱动的 user-only 结构化 CLI 命令，并将请求体文件参数统一改为 `--input-file`。
  涉及文件/模块：`internal/openplatform`、`internal/openplatform/contract|vendor|entity|schema`、`internal/cli`、`docs/cli-command-design.md`、`docs/ai-changes.md`
  关键逻辑/决策：新增 `contract/v1/mcp` 静态工具映射与契约测试；`openplatform` 增加 `IdentityPolicy` 并对 `/open-apis/contract/v1/mcp/` 执行 `--as user` 硬拦截；新增 `contract/vendor/entity/schema` 结构化命令并统一复用 service 层；`api call` 对该前缀默认走 user 身份；所有请求体文件输入从 `--file` 迁移到 `--input-file`，`--file` 保留给后续真实文件上传。

- 2026-04-14
  变更摘要：新增开放平台统一 client、输出渲染层和 `api call` 命令，为后续业务域命令封装打底。
  涉及文件/模块：`internal/openplatform`、`internal/output`、`internal/cli`、`internal/config`、`docs/ai-changes.md`
  关键逻辑/决策：profile 新增 `open_platform_base_url` 并由 `config add --env dev` 写入 `https://dev-open.qtech.cn`；新增统一的相对路径 `/open-apis/...` 校验、token 解析与 HTTP 请求包装；`contract-cli api call` 支持 `--profile/--as/--file/--data/--output/--raw/--header`；新增 `vendor` 域 service 样板和 CLI 禁止直接发 HTTP 的架构约束测试。

- 2026-04-14
  变更摘要：实现 app 身份 `tenant_access_token` 登录、状态展示与保留凭证登出语义。
  涉及文件/模块：`internal/cli`、`internal/oauth`、`internal/config`、`docs/ai-changes.md`
  关键逻辑/决策：`config add` 为 `dev` profile 写入 app token endpoint；`auth login --as app` 立即调用 `https://dev-open.qtech.cn/open-apis/auth/v3/tenant_access_token/internal` 换 token 并保存过期时间；token 兑换失败时保留新 `appId/appSecret` 但不切默认身份；`auth status --as app` 区分 `authorized/expired/configured/unconfigured`；`auth logout --as app` 仅清 token、保留 app 凭证。

- 2026-04-14
  变更摘要：清理旧的根目录二进制产物，并新增 Git 忽略规则避免本地产物再次出现在仓库根目录变更中。
  涉及文件/模块：`.gitignore`、`docs/ai-changes.md`
  关键逻辑/决策：删除历史 `democli` 二进制；将根目录 `contract-cli` 与 `democli` 纳入忽略规则，保留本地可执行文件使用能力，同时避免构建产物污染版本管理视图。

- 2026-04-14
  变更摘要：将 Go module path 调整为公司规范 `cn.qfei/contract-cli`。
  涉及文件/模块：`go.mod`、`cmd/contract-cli`、`internal/cli`、`internal/config`、`internal/oauth`、`docs/ai-changes.md`
  关键逻辑/决策：统一替换仓库内 Go import 前缀，避免继续使用临时的 `github.com/lyy/contract-cli`；本次仅调整模块标识与编译路径，不改变 CLI 运行时行为。

- 2026-04-14
  变更摘要：将 CLI 对外名称从 `democli` 统一重命名为 `contract-cli`，并保留旧配置兼容读取。
  涉及文件/模块：`go.mod`、`cmd/contract-cli`、`internal/cli`、`internal/config`、`internal/oauth`、`skills/auth`、`docs/*.md`
  关键逻辑/决策：帮助文案、module path、默认 `client_name`、技能与设计文档统一切换到 `contract-cli`；运行时优先使用 `CONTRACT_CLI_*` 与 `~/.contract-cli`，同时兼容旧的 `DEMOCLI_*` 与 `~/.democli`，避免现有本地登录态失效。

- 2026-04-14
  变更摘要：新增本地 `skills/auth`，将 `democli` 的登录与身份切换逻辑整理成可复用 skill。
  涉及文件/模块：`skills/auth/SKILL.md`、`skills/auth/agents/openai.yaml`、`docs/ai-changes.md`
  关键逻辑/决策：按 `lark-shared` 风格沉淀 `config add`、`auth login --as user|app`、`auth status/logout/use` 的使用规则；明确 `app` 保存 `app_id/app_secret`；补充 `config.json`/`secrets.json` 的存储约束与排障说明。

- 2026-04-14
  变更摘要：实现 `user` / `app` 双身份鉴权与默认身份切换，新增 app 凭据独立存储。
  涉及文件/模块：`internal/cli`、`internal/config`、`docs/ai-changes.md`
  关键逻辑/决策：`auth login/status/logout` 支持 `--as user|app`；新增 `auth use` 切换默认业务身份；profile 下分离保存 `user.token` 和 `app.token/credentials`；`appsecret` 独立落 `secrets.json`，旧平铺 OAuth 配置自动迁移到 `identities.user`。

- 2026-04-13
  变更摘要：新增一份供会议使用的 CLI 命令设计汇总文档，以主文档为基线并引用另外两份辅助方案。
  涉及文件/模块：`docs/cli-command-design-meeting.md`、`docs/ai-changes.md`
  关键逻辑/决策：不改动既有三份设计文档；新增会议版文档用于统一阅读顺序、角色分工、决策顺序和会议结论模板。

- 2026-04-13
  变更摘要：撤回上一轮 CLI 文档整合，恢复为三份独立的设计文档。
  涉及文件/模块：`docs/cli-command-design.md`、`docs/contract-create-command-options.md`、`docs/cli-command-design-mvp.md`、`docs/ai-changes.md`
  关键逻辑/决策：取消“单一主文档 + 迁移说明”的整理方式；恢复完整方案、合同创建对比方案和 MVP 方案分别独立维护。

- 2026-04-13
  变更摘要：将分散的 CLI 设计文档整合为单一主文档，并将其他文档改为迁移说明。
  涉及文件/模块：`docs/cli-command-design.md`、`docs/contract-create-command-options.md`、`docs/cli-command-design-mvp.md`、`docs/ai-changes.md`
  关键逻辑/决策：统一以 `docs/cli-command-design.md` 作为唯一设计来源；保留极简 MVP、合同创建备选方案和演进路径；避免后续文档分叉。

- 2026-04-13
  变更摘要：新增极简 MVP 命令设计文档，收敛首发命令为统一 `--input` 形式。
  涉及文件/模块：`docs/cli-command-design-mvp.md`、`docs/ai-changes.md`
  关键逻辑/决策：首发优先统一输入模型而非细分命令树；合同与交易方创建统一走 `--input`；复杂模式差异先沉到 `input.mode` 和内部 handler。

- 2026-04-13
  变更摘要：新增合同创建命令方案对比文档，整理“单命令 + mode”与“拆命令”两种设计及折中建议。
  涉及文件/模块：`docs/contract-create-command-options.md`、`docs/ai-changes.md`
  关键逻辑/决策：统一以 `ContractCreateSpec` 作为执行格式；明确 `spec` 可由智能体或用户提供；不建议由智能体隐式猜测模式。

- 2026-04-13
  变更摘要：新增 `democli` CLI 命令设计文档，统一合同、交易方、事件等领域命令风格。
  涉及文件/模块：`docs/cli-command-design.md`、`docs/ai-changes.md`
  关键逻辑/决策：复用现有 `config/auth` 授权体系；业务命令采用“资源 + 动作”；复杂请求统一走 `--file`/`--data`；`vendor`/`entity` 使用 `--operator` 映射底层 `user_id`。

- 2026-04-08
  变更摘要：初始化 `democli` Go 项目并接入最小 OAuth 授权 CLI 骨架。
  涉及文件/模块：`cmd/democli`、`internal/cli`、`internal/config`、`internal/oauth`、`go.mod`
  关键逻辑/决策：按 `dev` 预设实现 Higress-MCP 授权流程；只使用 Go 标准库；通过 `config add` 做 well-known 发现，`auth login/status/logout` 处理客户端注册、PKCE 授权码换 token 和本地落盘。
