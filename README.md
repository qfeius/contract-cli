# 智书合同 CLI：让 Agent 真的会处理合同

你的 AI Agent 能读需求、写方案、整理材料，但如果它进不了合同系统，就只能停在“帮你想一想”。合同查询、交易方确认、法人主体匹配、合同创建参数整理，最后还是要人打开系统逐项操作。`contract-cli` 就是给 Agent 接入智书合同开放平台的一套操作能力。安装后，Agent 可以在本地终端里直接查询合同、读取正文、检索交易方和法人主体，也可以根据你确认过的参数创建或更新合同。以前是你在系统里找字段、查编号、拼参数；现在是 Agent 先把合同相关信息查清楚、整理好，你只负责确认关键判断。

## 它是什么

`contract-cli` 是面向合同开放平台的命令行工具，适合在本地终端、自动化脚本和 AI Agent 中使用。

| 组成部分 | 作用 | 适用场景 |
| --- | --- | --- |
| `@qfeius/contract-cli` | 合同开放平台 CLI，用于发起合同、主数据和配置类操作 | 终端执行、脚本调用、Agent 自动调用 |
| CLI 内置 Agent skills | 给 AI Agent 使用的操作说明，让 Agent 能自动选择合适命令 | Codex、Cursor、Claude Code、Trae 等 Agent 工具 |

适用读者：

- 需要在本地电脑或 AI Agent 中使用合同开放平台能力的客户
- 负责配置、交付、培训和排障的实施同学
- 需要帮助客户完成合同数据查询与操作闭环的客户成功同学

## GitHub 开源

`contract-cli` 的代码和命令文档在 GitHub 维护。安装、排障或二次集成时，请选择与已安装 CLI 版本对应的发布标签，查看实现和命令参考。👉 **获取代码**：[qfeius/contract-cli](https://github.com/qfeius/contract-cli)

## 🚀 快速开始

### 1. 安装 CLI 与 Agent skills

推荐让 Agent 自动完成安装。把下面这段话发给你的 AI 工具即可：

```text
请帮我安装或升级合同开放平台 contract-cli，并同步更新当前客户端使用的全部配套 Agent skills。

本次任务包含 CLI 升级、skills 更新和结果验证，请完整执行：

1. 确认当前运行环境
   - 确认命令实际运行在本机还是云端沙箱。
   - 根据当前客户端的配置或已加载 skill 的真实路径，确认它实际读取且允许安装的 skills 目录。
   - 不要猜测目录，也不要默认使用 Codex 的目录。
   - 如果无法确认加载目录，请说明缺少的信息，不要宣称安装完成。
   - 执行 npm 安装或升级前，向用户说明自动同步会更新全部有效的已记录 Skill 目录，不只当前客户端；需要保留的官方 Skill 修改应先备份。自动同步会覆盖这些修改，成功后的事务备份不会长期保留，不能替代用户备份。

2. 安装或升级 CLI
   执行：
   npm install -g @qfeius/contract-cli@latest --registry https://registry.npmjs.org --foreground-scripts

   安装完成后，检查当前实际执行的 contract-cli 路径和版本，并查看安装输出中的 Skill 同步结果。
   如果 PATH 仍指向旧版本，请先解决路径问题。
   后续安装 skills 和验证版本必须使用同一个已确认的新版本 CLI。
   不要只凭 npx 输出的版本判断当前 CLI 已升级。

3. 同步更新全部配套 skills
   先检查自动同步结果。新版 CLI 会同步全部有效的已记录位置；当前客户端实际目录已同步成功时，不重复安装。
   首次安装、当前客户端没有配套 Skill、历史位置未被识别或当前目录未被覆盖时，确认真实目录、覆盖范围和备份后，使用新版本 CLI 内置 skills 完整安装：
   contract-cli skills install --target "<当前客户端实际读取的 skills 目录>" --force

   请先将占位内容替换为已确认的真实目录。WorkBuddy 等客户端也以实际读取目录为准，不将默认路径当作已确认配置。
   必须更新全部内置 skills，包括已存在的旧技能及其 references 等附属文件，不能只补装缺少的技能。完整安装成功后，CLI 会记住该位置，供后续全局升级同步。
   不要使用 npx skills add qfeius/contract-cli 代替本步骤，避免从 GitHub 默认分支安装与 CLI 不配套的版本。
   出现同步失败、权限或内容冲突时，先保留并说明原始结果，核实原因与备份后再处理；不要直接强制修复，也不要把手工补装成功说成自动同步成功。


4. 验证安装结果
   - 用 contract-cli skills list 获取当前 CLI 的内置技能清单，作为预期清单。
   - 检查目标目录中的技能名称、数量、SKILL.md 版本和附属文件。
   - 确认没有因跳过已有目录而保留旧版本。
   - 注意：skills list 只显示内置清单，不能单独证明客户端已安装或加载成功。
   - 按客户端要求刷新技能、重载或新建会话，再核实实际加载情况。

5. 汇报结果
   请分别列出：
   - CLI 实际执行路径及版本；
   - skills 实际安装目录、自动同步结果及是否后续手动安装；
   - CLI 已记录的 Skill 位置及各位置的失败或跳过项；
   - 预期技能数量和实际已更新数量；
   - 文件验证结果；
   - 当前客户端是否已加载新版。

   只有全部验证通过，才报告“CLI 与 skills 已同步升级完成”。
   如果文件已更新但客户端加载尚未验证，请明确报告“文件已更新，客户端加载待验证”。
```

也可以在终端中手动安装或升级。以下按首次使用和已有 Skill 升级分别执行：

```text
执行以下 npm 指令前，先确认自动同步的覆盖范围：全部有效的已记录 Skill 目录，不只当前客户端。需要保留的官方 Skill 修改请先备份；成功后的事务备份不会长期保留。

首次安装，或当前客户端尚无配套 Skill：
npm install -g @qfeius/contract-cli@latest --registry https://registry.npmjs.org --foreground-scripts
contract-cli --version
contract-cli skills list

当前目录未被自动覆盖时，确认真实目录和备份后再执行：
contract-cli skills install --target "<当前客户端实际读取的 skills 目录>" --force

已有配套 Skill，升级 CLI：
npm install -g @qfeius/contract-cli@latest --registry https://registry.npmjs.org --foreground-scripts
contract-cli --version

先查看自动同步结果。当前客户端目录已同步成功，无需重复安装；未识别或未覆盖时，再按上方指定目录完整安装。
```

执行 npm 安装或升级前，应先确认自动同步会更新全部有效的已记录 Skill 目录，并备份需要保留的官方 Skill 修改；成功后的事务备份不会长期保留，不能替代用户备份。手动安装 Skill 前，再将占位目录替换为当前客户端实际读取的完整路径，并确认该目录的覆盖范围和备份。`--force` 会更新官方配套 Skill（包括用户对其内容的修改）；不要默认使用 Codex 目录。完整安装成功后 CLI 会记住该位置，同一位置再次安装更新原记录。历史自定义目录未被发现、或换了客户端但新目录尚未覆盖时，也需要指定真实目录安装一次。

`--foreground-scripts` 仅用于显示安装脚本输出，不是同步开关；项目局部安装、使用 `--ignore-scripts` 或安装脚本被阻止时，不能认为已自动同步。CLI 安装成功不代表全部 Skill 位置同步成功。更新后仍须对照内置清单核对文件并刷新或新建会话；无法确认加载时说明“文件已更新，客户端加载待验证”。

历史自动接管仅覆盖支持探查的位置，且要求待覆盖的同名 Skill 完整内容匹配已核验的官方 1.9.3 / 1.9.4 指纹；未知来源、修改或额外文件不会仅凭名称自动接管。其他历史版本及未发现的自定义位置应确认后完整安装一次。新增官方名称与未管理的同名内容冲突时，该位置停止自动更新，其他位置继续处理。新目录完整安装不会取消旧目录登记，客户端换目录时不能据此认为旧位置已停止同步。`--name` 单技能安装或有跳过项时不新增整套同步登记。

### 2. 初始化本地 profile

当前源码构建仅支持 `prod`，默认 profile 为 `contract`；其他环境会在发请求前被拒绝，旧环境的同名 profile 重新配置时会清理旧认证，避免复用凭据。prod API 地址为 `https://open.qfei.cn`、账号地址为 `https://myaccount.qfei.cn`。

若需验证 npm 包构建流程，可生成独立的本地预发布包；版本中的 `-test` 只是包标记，运行时仅支持 `prod`，也不发布到 npm：

```bash
VERSION=1.8.4-test.20260923.6 bash scripts/build-test-package.sh
```

历史联调包可能仍含 test/blue 等旧环境能力，请通过新构建的 `config add --help` 核对当前能力。审批矩阵的 user/app 双身份和规则 OpenAPI 命令保持不变；真实环境验证仍需对应后端及网关配置，见 [双身份接入说明](docs/approval-matrix-user-app-auth.md)。不要在对话中发送 Token 或 App Secret。

首次使用前，需要在本机初始化一个 profile。本文统一使用 `contract` 作为示例名称，客户可以按项目名称自行调整。

```text
contract-cli config add --env prod --name contract
contract-cli auth status --profile contract --as user
```

| 参数 | 含义 | 示例值 |
| --- | --- | --- |
| `--env` | 目标环境 | `prod` |
| `--name` | 本机 profile 名称 | `contract` |
| `--profile` | 后续命令指定使用的 profile | `contract` |

### 3. 完成授权

`contract-cli` 支持个人授权和应用授权。你可以按使用场景选择。

| 授权方式 | 适用场景 | 身份参数 |
| --- | --- | --- |
| 个人授权 | 以当前用户身份查询或操作合同数据 | `--as user` |
| 应用授权 | 服务端脚本、自动化流程或客户已有应用凭证 | `--as app` |

两种身份的选择口径：

| 身份 | 含义 | 典型适用场景 |
| --- | --- | --- |
| `user` | 个人授权，以当前登录用户身份访问合同开放平台能力 | 用户本人查询、用户权限内的合同操作、Agent 代用户处理合同数据 |
| `app` | 应用授权，使用客户应用凭证换取 tenant access token | 服务端脚本、自动化流程、系统集成、批处理 |
| `user/app` | 同一条 CLI 命令两种身份都支持 | 查询、创建、模板、主数据等通用能力 |

豆包 / WorkBuddy / Codex 的个人授权使用 Device Grant，先检查当前授权：

```bash
contract-cli auth status --profile contract --as user
# 无有效授权时发起或恢复授权；已有有效授权则直接继续任务
contract-cli auth init --profile contract --output json
```

`auth init` 返回 `status=authorized` 时，表示已有有效或成功刷新的授权，直接继续原任务，不展示授权链接或二维码，也不执行 `auth complete`。返回 `status=pending` 时，才展示授权信息并等待用户完成授权。

WorkBuddy 使用 `qr_code_path` 交付原始 PNG 附件，AgentKit 使用 `qr_code_path`。豆包普通工作任务只展示 `verification_uri_complete` 和 `expires_at_display`，不展示二维码，也不读取或交付二维码文件。授权链接必须可点击；展示后结束当前轮次，不轮询、不继续业务操作。

仅在用户回复“已授权”后，查询一次授权结果：

```bash
contract-cli auth complete --profile contract --output json
```

结果仍为 `pending` 时继续等待；结果不确定时不自动重试。授权被拒绝、过期或要求重新开始时，先说明状态，经用户同意后再重新授权。登录态能否跨任务复用，以 CLI 返回的凭证范围为准。

本地终端仍支持个人授权码登录：

```text
contract-cli auth login --profile contract --as user
contract-cli auth status --profile contract --as user
```

如果当前环境不能自动打开浏览器：

```text
contract-cli auth login --profile contract --as user --no-open-browser
```

应用授权：

```text
contract-cli auth login --profile contract --as app --app-id <app_id> --app-secret <app_secret>
contract-cli auth status --profile contract --as app
```

正式包默认使用 `contract` profile 和 `prod` 环境；CLI 不会使用历史非生产 profile 发起授权或业务请求，也不会把旧凭据迁入 prod。更新 Skills 后必须完全退出并重新启动，然后新建任务；已有任务不会热加载新 Skill。

也可以通过环境变量提供应用凭证：

```text
export CONTRACT_CLI_APP_ID="<app_id>"
export CONTRACT_CLI_APP_SECRET="<app_secret>"
contract-cli auth login --profile contract --as app
```

> 兼容说明：旧 `--as bot` 以及旧环境变量 `CONTRACT_CLI_BOT_APP_ID` / `CONTRACT_CLI_BOT_APP_SECRET` 仍可被 CLI 识别，但新文档和新脚本统一使用 `app`。

> ⚠️ 请妥善保管 `app_secret`，不要粘贴到公开群聊、截图或代码仓库中。

需要把后续命令的默认身份切到 app 时，可以执行：

```text
contract-cli auth use --profile contract --as app
```

查看授权状态时，user 和 app 都可以检查：

```text
contract-cli auth status --profile contract --as user
contract-cli auth status --profile contract --as app
```

### 🎉 4. 开始第一个任务

打开你的 AI Agent 工具，输入一句自然语言需求：

```text
帮我用 contract-cli 查一下这个合同的详情：<contract-id>。profile 用 contract，以我的身份查询。
```

Agent 会根据已安装的 skills 选择对应命令，并在执行后把合同详情整理成可读结果。

## 🎬 用户场景

以下按 v1.9.5 面向用户的 15 个 skills 分组，按各 Skill 的实际能力展开代表场景；合同管理覆盖创建、审批、文件、签署与协作等常用业务。中文能力名称与 Skill 的对应关系统一见下方能力地图；涉及其他能力时，Agent 再调用对应 Skill 协作。示例中的名称、编号和文件名请替换为实际对象。右侧展示关键命令；JSON 文件由 Agent 按对应 skill 的真实字段与已确认参数生成。多步骤场景仍需逐步查询、确认并核验，不能将整列命令直接连续执行。

### 1. 授权与身份管理

#### 场景 1.1：完成授权后继续处理合同

**授权支持：**个人

Agent 检查当前授权，必要时引导你在官方页面完成手机号验证和企业授权；收到“已授权”后查询一次结果，再交给合同搜索 skill 继续原任务。应用身份只使用已在本机安全配置好的凭据，不在对话中索取 App Secret。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请帮我完成智书合同的个人授权，然后继续查询合同“HT2026-001”。<br>如果已有有效授权就直接复用；需要重新授权时，给我官方授权链接。 | 先检查授权：<br>`contract-cli auth status --profile contract --as user`<br><br>仅在需要授权时：<br>`contract-cli auth init --profile contract --output json`<br><br>仅在用户完成授权后：<br>`contract-cli auth complete --profile contract --output json` |

### 2. 合同搜索

#### 场景 2.1：按关键词快速找到合同

**授权支持：**个人

Agent 在当前用户可见范围内按默认关键词范围查询，不擅自缩小为仅合同名称。你明确要求“合同文本”或“仅正文”时，按相应范围查询。结果说明分页情况，区分合同组与合同条目，不把同组成员全部当作独立命中。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 帮我搜索“采购”，范围与合同页面默认“全部”一致。<br>列出合同名称、编号和状态，并说明是否还有下一页。 | `contract-cli contract search --profile contract --as user --input-file keyword-search.json` |

#### 场景 2.2：按人员和自定义字段组合筛选

**授权支持：**个人

Agent 借助人员 skill 查候选，通过字段发现获取真实字段及选项，再按已支持的组合表达查询。“我的合同”页签不等于“我申请的合同”；我方主体精确 ID 筛选目前没有已验证的通用配方，复杂条件也不能承诺任意 AND/OR 都可表达。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 查张三申请的、自定义字段“项目区域”为“华东”的合同。<br>先查真实人员和字段选项；有重名或字段歧义时让我确认，查询时保留这两个条件。 | 查询人员候选：<br>`contract-cli employee list --name "张三" --profile contract --as user`<br><br>发现真实字段：<br>`contract-cli contract search-fields --keyword "项目区域" --profile contract --as user`<br><br>保留两个条件查询：<br>`contract-cli contract search --profile contract --as user --input-file combined-search.json` |

### 3. 合同创建、审批与全流程业务操作

#### 场景 3.1：查看合同并下载归档材料

**授权支持：**个人 / 应用

Agent 读取已知合同，按指定字段收集文件并下载。明确要求“全部文件”时，还会收集可访问的流程和评论附件，去重后逐项处理。该流程由 skill 编排，没有独立的全部下载命令；归属不明、字段不可见或读取失败时会说明材料缺口。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请查看合同 ID 为“&lt;contract-id&gt;”的详情和正文，整理交易双方、金额及状态。<br>把归档主文件和归档附件下载到我指定的目录，列出文件名、来源和下载结果。 | 读取合同：<br>`contract-cli contract get <contract-id> --profile contract --as user`<br><br>读取正文：<br>`contract-cli contract text <contract-id> --profile contract --as user --full-text`<br><br>按已确认文件逐个下载：<br>`contract-cli contract download-file <file-id> --contract <contract-id> --profile contract --as user --output-file ./归档文件.pdf` |

#### 场景 3.2：用已有文件创建合同

**授权支持：**个人 / 应用

把合同正文和业务信息交给 Agent，它会查询可用分类、核对签约主体、金额与期限，先列出缺项，再整理创建资料。正文和附件先上传取得平台文件 ID；分类简称使用与创建身份一致的查询结果。创建支持个人和应用身份，应用身份还需提供真实创建人。创建成功后返回合同 ID 并核对详情；提交审批另行处理。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请以我的个人身份，用我提供的“采购合同.docx”创建采购合同。<br>先核对分类、我方主体、交易方、金额和有效期，缺少的信息先问我；展示创建摘要，确认后创建，暂不提交审批。 | 查询可用分类：<br>`contract-cli contract category list --profile contract --as user --lang zh-CN`<br><br>资料确认后上传正文：<br>`contract-cli contract upload-file --profile contract --as user --file ./采购合同.docx --file-type text`<br><br>将真实分类、主体和返回的文件 ID 写入请求体后创建：<br>`contract-cli contract create --profile contract --as user --input-file contract-create.json`<br><br>用返回的合同 ID 核验：<br>`contract-cli contract get <contract-id> --profile contract --as user` |

#### 场景 3.3：选择标准模板并创建合同

**授权支持：**个人 / 应用

Agent 查询分类下的模板、读取模板字段，补齐变量后先生成模板实例，再使用返回的实例 ID 创建合同。模板实例与合同是两个对象；生成实例并不代表合同已经创建。支持个人和应用身份，分类、模板与创建保持同一身份。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请用系统里的标准采购模板创建一份合同。<br>先列出可用分类和模板让我选择，再按模板字段核对双方、金额和日期；展示填写结果，确认后创建合同并返回详情。 | 查询分类并取得真实分类编号：<br>`contract-cli contract category list --profile contract --as user --lang zh-CN`<br><br>列出模板并读取所选模板：<br>`contract-cli contract template list --profile contract --as user --category-number <category-number> --page-size 20`<br>`contract-cli contract template get <template-id> --profile contract --as user`<br><br>补齐变量后生成实例：<br>`contract-cli contract template instantiate --profile contract --as user --input-file template-instance.json`<br><br>将返回的实例 ID 写入创建资料：<br>`contract-cli contract create --profile contract --as user --input-file template-contract-create.json` |

#### 场景 3.4：基于原合同创建补充协议或终止申请

**授权支持：**个人 / 应用

Agent 先读取原合同，核对关联关系，再按合同变更或终止的创建配方整理资料。变更需要变更说明；终止需要终止原因、日期及日期类型，分类、签约主体和正文也按要求补齐。这里创建的是关联原合同的业务单据，创建成功不代表原合同已经完成变更或终止。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请基于合同“&lt;contract-id&gt;”和我提供的补充协议文件，创建一份合同变更申请。<br>先展示原合同、变更原因和新协议资料，确认后创建，暂不提交。 | 读取原合同：<br>`contract-cli contract get <contract-id> --profile contract --as user`<br><br>按场景 3.2 查询分类、上传协议，补齐变更资料后：<br>`contract-cli contract create --profile contract --as user --input-file contract-change-create.json` |
| 请基于这份合同准备终止申请，使用我提供的终止原因和生效日期。<br>缺项先问我，确认资料后创建申请。 | 读取原合同并按相同身份核对分类、正文和终止资料后：<br>`contract-cli contract get <contract-id> --profile contract --as user`<br>`contract-cli contract create --profile contract --as user --input-file contract-termination-create.json` |

#### 场景 3.5：提交合同审批，或处理退回后的流程

**授权支持：**应用

合同提交和重新提交目前仅支持应用身份。首次提交限开放平台创建的草稿合同；重新提交要求合同已撤回或被拒绝，且当前处于提交节点。应用流程接口还支持按权限处理同意、拒绝、撤回、重新提交及作废；其中作废仅限提交节点。Agent 会核对合同、流程、任务和操作人，按明确选择执行一项操作，再查询结果。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请用已授权的应用身份检查这份开放平台创建的草稿合同。<br>资料和状态满足要求后，按我的确认提交审批，并返回流程信息。 | 核对合同：<br>`contract-cli contract get <contract-id> --profile contract --as app`<br><br>确认提交后：<br>`contract-cli contract submit <contract-id> --profile contract --as app`<br><br>用返回的流程 ID 查询：<br>`contract-cli contract approval get <process-instance-id> --profile contract --as app` |
| 这份合同被退回了，请检查是否已回到提交节点。<br>资料已按意见处理完成，满足条件后重新提交。 | 核对合同与流程后执行重新提交：<br>`contract-cli contract resubmit <contract-id> --profile contract --as app` |
| 请用应用身份检查这个已有流程能否撤回，并展示将要操作的合同和任务。<br>我确认后再执行撤回；如果节点不允许，就说明原因。 | 读取已有流程：<br>`contract-cli contract approval get <process-instance-id> --profile contract --as app`<br><br>核对操作人、任务及动作后执行：<br>`contract-cli contract approval start <process-instance-id> --profile contract --as app --input-file approval-action.json`<br><br>该命令操作已有流程；不是创建新审批流程的入口。 |

#### 场景 3.6：查看我的审批待办、已办和知会，追踪流程与评论

**授权支持：**个人

Agent 按个人身份查询任务，结合流程详情和评论说明当前节点、处理进度及待补材料。流程实例 ID、任务实例 ID 与合同 ID 分别使用。评论可以新增、回复、@ 人员或关联附件；提交前按真实对象整理内容。流程数据不完整时会明确说明，不能将缺失字段判断为流程结束。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 列出我的合同审批待办；找到合同“CN-001”对应的任务，告诉我当前在哪个节点、最近有哪些评论。 | 查询待办：<br>`contract-cli contract approval task list --profile contract --as user --task-type todo --query "CN-001" --page-size 20`<br><br>读取所选流程与评论：<br>`contract-cli contract approval get <process-instance-id> --profile contract --as user`<br>`contract-cli contract approval comment list <process-instance-id> --profile contract --as user` |
| 我想回顾已经处理过的合同，以及抄送给我的合同。请分别列出结果，并说明是否还有下一页。 | 分别查询已办与知会：<br>`contract-cli contract approval task list --profile contract --as user --task-type done --page-size 20`<br>`contract-cli contract approval task list --profile contract --as user --task-type notice --page-size 20` |
| 请在我选定的流程中添加评论：“请补充付款条件说明”，并附上我提供的说明文件。 | 上传同用户的评论附件：<br>`contract-cli contract upload-file --profile contract --as user --file ./付款条件说明.pdf --file-type reviewAttachment`<br><br>将返回文件 ID 与已确认评论写入请求体后提交：<br>`contract-cli contract approval comment create <process-instance-id> --profile contract --as user --input-file approval-comment.json`<br><br>回查评论确认结果：<br>`contract-cli contract approval comment list <process-instance-id> --profile contract --as user` |

#### 场景 3.7：按我的意见通过或拒绝审批待办

**授权支持：**个人

Agent 先读取个人待办和流程，核对可办理任务，再按你明确选择的意见通过或拒绝；拒绝必须填写原因。需要审批附件时，以同一用户上传专用审批附件，并在授权有效期内使用。盖章、归档节点需回合同系统处理。办理结果不确定时先查状态，不能直接重复提交。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请打开我选择的待办，核对合同和当前节点。<br>我确认通过后，以“同意”办理；如需附上我提供的审批意见文件，先按审批附件上传。 | 核对任务与流程后，按需上传新附件：<br>`contract-cli contract upload-file --profile contract --as user --file ./审批意见.pdf --file-type approveAttachment`<br><br>明确通过后执行；仅在有有效新附件时加文件参数：<br>`contract-cli contract approval task approve <task-instance-id> --profile contract --as user --comment "同意" --file-id <file-id>`<br><br>无附件时省略 `--file-id`，完成后回查流程。 |
| 请拒绝我选定的这条审批待办，意见是“付款条件尚未确认，请补充后重新提交”。 | 核对选定任务可办理后执行：<br>`contract-cli contract approval task reject <task-instance-id> --profile contract --as user --comment "付款条件尚未确认，请补充后重新提交"`<br><br>回查处理结果：<br>`contract-cli contract approval get <process-instance-id> --profile contract --as user` |

#### 场景 3.8：补齐归档材料，或生成合同信息打印文件

**授权支持：**应用

应用身份可按合同状态补充规定的扫描件、归档主文件和归档附件。Agent 先读取合同，上传对应用途文件，再更新允许的文件字段；这不等于可任意修改合同名称、金额等基础字段，也不代替办理归档节点。生成打印文件会先返回平台文件 ID，随后下载到指定目录。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 这份合同正在归档，请把我提供的签署扫描件补为归档主文件。<br>先核对合同状态及文件用途，确认后更新，并告诉我结果。 | 读取合同并核对归档状态：<br>`contract-cli contract get <contract-id> --profile contract --as app`<br><br>上传扫描件：<br>`contract-cli contract upload-file --profile contract --as app --file ./签署扫描件.pdf --file-type scan`<br><br>将返回文件 ID 写入对应归档字段后更新：<br>`contract-cli contract patch <contract-id> --profile contract --as app --input-file contract-archive-patch.json` |
| 请生成这份合同的合同信息打印文件，并下载到我指定的目录。 | 生成打印文件：<br>`contract-cli contract print-file --profile contract --as app --input-file contract-print.json`<br><br>使用返回的文件 ID 下载：<br>`contract-cli contract download-file <file-id> --profile contract --as app --output-file ./合同信息.pdf` |

#### 场景 3.9：获取签署与实名认证链接，或调整签署方式

**授权支持：**应用

应用身份可获取合同签署链接，以及个人、企业实名认证链接，由相关人员在页面完成认证和签署。需要改为纸质签署时，先核对真实业务对象、业务类型和状态，再执行切换。获取链接不代表已经完成认证或签署。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请获取这份合同的签署链接，并根据我提供的签署主体信息，准备所需的个人或企业实名认证链接。 | 获取签署链接：<br>`contract-cli contract sign-url get <contract-id> --profile contract --as app`<br><br>按实际主体选择一种认证入口：<br>`contract-cli contract esign personal-auth-url --profile contract --as app --input-file personal-auth.json`<br>或：<br>`contract-cli contract esign org-auth-url --profile contract --as app --input-file org-auth.json` |
| 这份合同申请确定改为线下纸质签署。请先核对目标合同和当前签署方式，确认后执行切换。 | 核对合同申请业务类型及状态后：<br>`contract-cli contract sign switch-to-paper --profile contract --as app --business-id <contract-id> --business-type-code 0` |

#### 场景 3.10：查询分享记录，并按指定范围分享或授权

**授权支持：**应用

Agent 使用应用身份查询分享记录，按明确的合同、接收人员及授权期限执行分享或授予权限。姓名须先解析为真实人员，存在歧义时先确认；分享和授权是不同操作，按业务目标分别选择，不自动合并执行。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 查一下这份合同已经分享给谁；再按我确认的接收人名单分享，完成后核对记录。 | 查询现有分享：<br>`contract-cli contract share get <contract-id> --profile contract --as app`<br><br>接收人确认后分享：<br>`contract-cli contract share batch-create --profile contract --as app --input-file contract-share.json`<br><br>再次查询分享记录核验。 |
| 请给指定人员授予这份合同的访问权限，期限按我提供的开始和结束时间设置。执行前先展示对象和期限。 | 核对合同、人员及授权期限后：<br>`contract-cli contract authorization grant --profile contract --as app --input-file contract-authorization.json` |

#### 场景 3.11：查看协商进展，并取得协商链接和文件

**授权支持：**应用

应用身份可搜索协商合同、查询协商操作记录与入口链接，并取得协商文件信息后下载。Agent 按返回记录整理参与人、操作时间和进展；查询记录不代表能够代替参与人确认合同、定稿或接受修订。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请查找与“采购”相关的协商合同，打开我选定的一份，整理最近的协商记录并给我协商入口。 | 查询协商列表：<br>`contract-cli contract cooperation search --profile contract --as app --input-file cooperation-search.json`<br><br>查询所选合同的记录与入口：<br>`contract-cli contract cooperation record get <contract-id> --profile contract --as app`<br>`contract-cli contract cooperation link get <contract-id> --profile contract --as app` |
| 请查看这份合同可取得的协商文件，并把我选定的文件下载到指定目录。 | 取得文件信息：<br>`contract-cli contract cooperation file get <contract-id> --profile contract --as app`<br><br>使用返回的文件 ID 下载：<br>`contract-cli contract cooperation file download <file-id> --profile contract --as app --output-file ./协商文件.docx` |

#### 场景 3.12：核对合同字段配置，并维护下拉选项

**授权支持：**个人 / 应用

实施人员可通过应用身份读取指定分类和业务类型的流程字段，并维护已支持的下拉选项范围。个人身份另可查询合同枚举。字段选项配置与某份合同上的字段值不同；该能力不包含任意新增字段或任意修改合同数据。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请查出这个合同分类在合同申请流程中使用的字段，便于我核对录入要求。 | 使用真实分类 ID 查询合同申请的流程字段：<br>`contract-cli contract form attribute list --profile contract --as app --category-id <category-id> --business-type-code 0` |
| 请查询合同状态枚举；另外，把指定字段组下这个下拉字段的选项按我确认的完整清单更新。 | 个人身份查询枚举：<br>`contract-cli contract enum list --profile contract --as user --type contract_status`<br><br>应用身份按已确认字段和完整选项清单更新：<br>`contract-cli contract field update --profile contract --as app --input-file contract-field-options.json` |

#### 场景 3.13：同步合同用户分组

**授权支持：**个人 / 应用

Agent 可在已授权的个人或应用身份下触发合同用户分组同步，报告实际执行结果。该能力用于同步已有分组关系，不提供任意新建分组、修改通讯录或自行扩大合同权限的入口。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请以我的个人身份执行合同用户分组同步，并告诉我同步是否成功；如果返回失败，列出原因。 | `contract-cli contract sync-user-groups --profile contract --as user` |

#### 场景 3.14：清理确认不再使用的合同草稿

**授权支持：**应用

应用身份可删除同一应用下创建的草稿合同。Agent 先核对名称、ID、状态及应用归属，展示待删除对象；只有明确确认删除后才执行。此入口不适用于任意状态的合同。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 这份测试合同不再使用。请先检查它是不是本应用创建的草稿，展示名称和 ID，等我确认后再删除。 | 检查合同：<br>`contract-cli contract get <contract-id> --profile contract --as app`<br><br>满足限制且明确确认删除后：<br>`contract-cli contract delete <contract-id> --profile contract --as app` |

### 4. 人员查询

#### 场景 4.1：确认同名人员，避免选错申请人

**授权支持：**个人

Agent 使用个人身份查询真实人员候选，保留对应人员标识。确认后的候选可交给合同搜索 skill，用于申请人、需求人或自定义人员字段筛选。人员查询只读，不自动同步或修改通讯录。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 帮我查找姓名为“张三”的人员，列出所属部门和人员状态。<br>如果有多个同名人员，先让我选择，不要默认取第一位。 | `contract-cli employee list --name "张三" --profile contract --as user` |

### 5. 部门查询

#### 场景 5.1：定位实际业务部门

**授权支持：**个人

Agent 使用个人身份查询部门候选，保留真实部门标识，供后续人员查询或合同部门筛选使用。申请部门与需求人部门是不同业务角色，后续筛选需按你指定的角色处理；本 skill 不修改组织架构。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 帮我查找名称包含“华东销售”的部门，列出部门层级和状态。<br>如果有多个候选，先让我确认要使用哪个部门。 | `contract-cli department list --name "华东销售" --profile contract --as user` |

### 6. 合同智能审查

#### 场景 6.1：审查一份外部合同文件

**授权支持：**个人

Agent 告知文件将提交至智书合同审查服务，再按分类、主体、强度和清单推进。当前支持单份 DOC、DOCX、PDF，本地文件或原始文件流最大 50 MiB；主体来自同一文件的提取结果。外部预审无须创建合同或协商，结果在 Agent 中展示，不提供系统合同详情链接。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请审查我提供的《采购协议.docx》。<br>上传后先让我选择合同分类，再从文件提取的主体中确认保护立场。<br>审查强度用中立，清单从我有权限使用的清单中选择，最后展示风险和建议。 | 上传文件：<br>`contract-cli review file upload --profile contract --as user --file ./采购协议.docx --name 采购协议.docx`<br><br>提取同一文件的主体：<br>`contract-cli review subject extract --profile contract --as user --business-id <business-id> --file-id <file-id>`<br><br>分类、立场、强度和清单确定后：<br>`contract-cli review task start --profile contract --as user --input-file review-start.json` |

#### 场景 6.2：审查系统合同，或继续已有任务

**授权支持：**个人

Agent 必要时先调用搜索 skill 定位合同，再核实来源、权限及状态。已识别状态下，新发起范围为协商的待发起协商、协商中，以及任务中心的审批中、签订中、归档中、已归档；未知状态由服务端校验。多个正文来源需先选择；已有任务沿用原任务及真实业务关联，符合导航条件时返回“查看审查”链接。审查不自动修改正文、接受修订或通过审批。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请审查合同“HT2026-001”的当前正文，先核实合同和正文来源。<br>让我从该文件提取的主体中选择立场；如果本次审查已有任务，就继续查询原任务。 | 新审查先读取目标合同：<br>`contract-cli contract get <contract-id> --profile contract --as user`<br><br>已有任务只续查原任务：<br>`contract-cli review task result --task-id <task-id> --business-id <business-id> --profile contract --as user` |

### 7. 审查清单与规则维护

#### 场景 7.1：了解团队已有的审查标准

**授权支持：**个人

Agent 查询当前租户可见配置，按所需范围处理分页；信息不完整时说明缺口。仅在审查时选择已有清单，仍由智能审查 skill 处理，不需要进入配置维护流程。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请查询我有权限查看的采购合同审查清单。<br>展示适用分类、包含的规则及规则所属分组，区分内置清单和自定义清单。 | 按已确认分类查询清单：<br>`contract-cli review checklist list --profile contract --as user --contract-category <category-id> --page-index 1 --page-size 100`<br><br>查询规则分组：<br>`contract-cli review rule group list --profile contract --as user --page-index 1 --page-size 100`<br><br>读取相关分组规则：<br>`contract-cli review rule list --profile contract --as user --group-id <group-id> --page-index 1 --page-size 100` |

#### 场景 7.2：补充一条团队审查规则

**授权支持：**个人

Agent 按已支持的单项流程维护自定义配置。内置清单只读；删除须确认完整影响，规则或分组删除还需证明完整引用范围及服务端单次原子级联能力。当前不支持规则跨分组迁移，不承诺整套清单复制；审查规则与审批矩阵由不同 skill 负责。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 我想新增一条检查付款前置条件的审查规则。<br>请先整理规则内容、所属分组及需要引用它的自定义清单，逐项展示变更，确认后保存并回读。 | 确认内容并通过预检后创建：<br>`contract-cli review rule create --profile contract --as user --group-id <group-id> --input-file review-rule.json`<br><br>取得新规则 ID 并确认完整引用差异后：<br>`contract-cli review checklist update --profile contract --as user --id <checklist-id> --input-file checklist-update.json` |

### 8. 交易方管理

#### 场景 8.1：查询交易方资料并确认目标

**授权支持：**个人 / 应用

Agent 可用个人或应用身份查询列表与详情。按证件查询和全量分页为应用专属能力，不能因为个人查询无结果就自动改用应用身份。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 帮我查找“示例供应商”的交易方资料和状态。<br>有同名记录时列出候选供我选择，确认后再查看详情。 | 查询候选：<br>`contract-cli mdm vendor list --profile contract --as user --name "示例供应商"`<br><br>读取已确认交易方：<br>`contract-cli mdm vendor get <vendor-id> --profile contract --as user` |

#### 场景 8.2：局部更新交易方信息

**授权支持：**个人 / 应用

Agent 使用对应身份的局部更新能力。个人创建和局部更新不发起审批，不能写入编码、风险和系统字段；启用、停用使用独立的个人身份操作，附件仅引用已有主数据文件。旧版全量更新仍为应用身份能力。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请按我提供的新资料更新这家已确认交易方的联系人信息。<br>先展示修改前后的差异，确认后只更新这些字段，其他信息保持不变。 | 确认差异后局部更新：<br>`contract-cli mdm vendor patch <vendor-id> --profile contract --as user --input-file vendor-patch.json`<br><br>回读核验：<br>`contract-cli mdm vendor get <vendor-id> --profile contract --as user` |

### 9. 法人主体管理

#### 场景 9.1：确认本次签约的我方主体

**授权支持：**个人 / 应用

Agent 可用个人或应用身份查询列表及按 ID 查看详情，避免直接把名称当成唯一主体。该查询可为合同创建准备主体信息，但不代表合同搜索已经支持主体 ID 精确筛选。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请查询我可以使用的法人主体，帮我找到“示例科技有限公司”。<br>展示候选并让我确认，再读取所选主体的详情。 | 查询主体候选：<br>`contract-cli mdm legal list --profile contract --as user --name "示例科技有限公司" --page-size 20`<br><br>读取已确认主体：<br>`contract-cli mdm legal get <legal-entity-id> --profile contract --as user` |

#### 场景 9.2：通过企业应用维护法人主体

**授权支持：**应用

法人主体按编码查询、创建和更新使用应用身份。字段要求通过主数据字段 skill 核对，操作人按接口要求提供，不从个人查询权限推断具有应用写权限。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请使用已配置的应用身份，根据我提供的资料准备新法人主体的创建参数。<br>先核对字段要求和操作人，展示完整内容，确认后创建并查询结果。 | 核对字段：<br>`contract-cli mdm fields list --profile contract --as app --biz-line legal_entity`<br><br>确认参数后创建：<br>`contract-cli mdm legal create --profile contract --as app --user-id <operator-user-id> --input-file legal-create.json`<br><br>按创建结果回读：<br>`contract-cli mdm legal get <legal-entity-id> --profile contract --as app` |

### 10. 主数据字段配置

#### 场景 10.1：先看字段要求，再准备主数据

**授权支持：**个人 / 应用

Agent 查询配置并整理填写要求，不自行修改字段定义。个人身份支持交易方、法人主体和交易方风险字段配置；应用身份支持交易方和法人主体。合同搜索字段发现属于合同搜索 skill，不使用此入口代替。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请查看当前交易方的字段配置，整理字段名称、类型和必填要求。<br>对照我提供的资料列出缺少的信息，先不要创建交易方。 | `contract-cli mdm fields list --profile contract --as user --biz-line vendor` |

### 11. 固定汇率维护

#### 场景 11.1：查询指定生效日的固定汇率

**授权支持：**应用

Agent 按明确的源币种、目标币种和生效日期查询系统固定汇率，不将结果解释为实时市场行情。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请使用已配置的应用身份，查询 2026 年 10 月 1 日人民币到美元的固定汇率。<br>展示币种方向、生效日期和查询结果。 | `contract-cli mdm fixed-exchange-rate get --profile contract --as app --source-currency CNY --target-currency USD --effective-date 2026-10-01` |

#### 场景 11.2：更新已确认的汇率配置

**授权支持：**应用

Agent 在必要字段完整后执行配置更新，不自行推算或选择汇率值。该能力仅支持应用身份。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请按我提供的币种方向、生效日期和汇率值准备更新内容。<br>先展示完整配置，确认后用应用身份更新，并重新查询核验。 | 确认后更新：<br>`contract-cli mdm fixed-exchange-rate update --profile contract --as app --input-file fixed-exchange-rate.json`<br><br>按同一组参数回读：<br>`contract-cli mdm fixed-exchange-rate get --profile contract --as app --source-currency <source-currency> --target-currency <target-currency> --effective-date <effective-date>` |

### 12. 主数据文件下载

#### 场景 12.1：下载主数据记录中的附件

**授权支持：**应用

Agent 使用真实主数据文件 ID 下载，显式指定输出位置。该 skill 仅支持应用身份；合同正文和合同附件走合同 skill 的文件能力，不混用文件归属和入口。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请使用已配置的应用身份，把这条主数据记录中已确认文件 ID 的附件下载到我指定的目录。<br>完成后给出本地文件位置和下载结果。 | `contract-cli mdm file download <file-id> --profile contract --as app --output-file ./主数据附件.pdf` |

### 13. 付款申请、计划与记录

#### 场景 13.1：查询合同的付款申请

**授权支持：**应用

Agent 使用指定合同查询付款申请列表与详情；付款计划可按另行明确的条件搜索。结果以真实返回字段为准，不用申请状态推断已经完成银行划款。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请使用已配置的应用身份，查询合同“&lt;contract-id&gt;”的付款申请。<br>整理申请列表和返回的状态，继续读取我选中的申请详情。 | 查询申请列表：<br>`contract-cli payment list --contract <contract-id> --profile contract --as app --page-size 20`<br><br>读取所选申请：<br>`contract-cli payment get <payment-id> --contract <contract-id> --profile contract --as app` |

#### 场景 13.2：登记已确认的付款记录

**授权支持：**应用

该 skill 还支持付款申请和记录更新、付款计划同步及按计划查询记录，全部使用应用身份。它管理申请、计划和记录，不执行银行打款或退款。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请根据我提供的实际付款资料，准备指定合同和付款申请的付款记录。<br>先展示关联对象及填写内容，确认后创建，再查询核验结果。 | 确认后创建记录：<br>`contract-cli payment record create --contract <contract-id> --payment <payment-id> --profile contract --as app --input-file payment-record.json`<br><br>按创建结果回读：<br>`contract-cli payment record get <record-id> --contract <contract-id> --payment <payment-id> --profile contract --as app` |

### 14. 审批矩阵维护

#### 场景 14.1：将业务规则整理为矩阵导入计划

**授权支持：**个人 / 应用

Agent 支持个人或应用身份维护矩阵。文档、表格和 Excel 内容先由 Agent 转换为 CLI 支持的输入，再生成计划；不是直接导入任意 Excel。矩阵最多 2000 行，总列最多 12，其中优先级和备注占 2 列，条件与结果列合计最多 10 列。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请根据我提供的采购审批规则，整理目标矩阵的条件列、结果列和规则行。<br>先核实人员、部门及现有结构，展示导入计划；确认后写入并核验，暂不发布。 | 先生成不写业务数据的计划：<br>`contract-cli rule table import plan --profile contract --as user --product-id contract --group-id approve_matrix --table-id <table-id> --input-file matrix-import.json`<br><br>计划获确认后执行：<br>`contract-cli rule table import apply --profile contract --as user --plan-id <plan-id>` |

#### 场景 14.2：核验导入进度，再发布矩阵

**授权支持：**个人 / 应用

导入支持暂停、恢复、核验和取消后续执行，逐行写入，不保证整批原子成功，取消不回滚已有数据。计划有效期 24 小时，身份、结构或数据变化后可能需重新生成。发布是独立步骤；当前不能保证完整矩阵原样复制，按可见信息重建需补齐并确认缺失配置。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请查看刚才的矩阵导入进度，先核验结果不确定的行。<br>展示剩余内容，按我确认的范围继续；全部核验后展示发布影响，确认后再发布。 | 查看进度：<br>`contract-cli rule table import get --profile contract --as user --plan-id <plan-id>`<br><br>先核验未确认的行：<br>`contract-cli rule table import verify --profile contract --as user --plan-id <plan-id>`<br><br>剩余导入完成且获授权后预发布：<br>`contract-cli rule table pre-release --profile contract --as user --product-id contract --group-id approve_matrix --table-id <table-id>`<br><br>确认发布影响后正式发布：<br>`contract-cli rule table release --profile contract --as user --product-id contract --group-id approve_matrix --table-id <table-id>` |

### 15. 事件出口 IP 查询

#### 场景 15.1：为事件通知准备网络白名单资料

**授权支持：**应用

Agent 查询并汇总出口 IP；分页未完成时说明清单不完整。该 skill 不自动修改防火墙或其他网络配置。

| 用户输入提示词 | Agent 会使用的命令 |
| --- | --- |
| 请使用已配置的应用身份，查询智书合同事件通知的出口 IP 清单。<br>读取完整分页，整理给网络管理员配置白名单使用。 | 读取第一页：<br>`contract-cli event outbound-ip list --profile contract --as app --page-size 20`<br><br>仅在仍有下一页时：<br>`contract-cli event outbound-ip list --profile contract --as app --page-size 20 --page-token <page-token>` |

## 能力地图

下表按用户场景的顺序，统一列出中文简称、Skill 和授权支持。“仅个人授权”或“仅应用授权”表示当前业务命令支持的身份；“两种授权”表示两种身份均有可用操作，标注“按操作区分”时，具体能力不能互相替代。“授权与身份管理”负责授权本身，其他 Skill 复用对应身份。操作差异见下方速查表。

| 中文简称 / Skill | 能力范围 | 对应用户场景 | 授权支持与限制 |
| --- | --- | --- | --- |
| **授权与身份管理**<br>`auth` | 授权、状态查询、退出及默认身份切换 | 1.1 完成授权后继续任务 | **统一授权入口**<br>分别管理个人授权与应用授权；完成一种不会自动获得另一种 |
| **合同搜索**<br>`contract-cli-contract-search` | 关键词、编号、文本、日期、状态、金额、对象及自定义字段筛选，分页 | 2.1 关键词查找；2.2 组合筛选 | **两种授权，按操作区分**<br>搜索均支持；字段发现仅个人，V2 搜索仅应用；查询参数不同 |
| **合同管理**<br>`contract-cli-contract` | 详情与文件下载；文件/模板创建及变更、终止申请；提交/重提、流程操作、个人任务与评论；归档材料更新与打印；签署及认证链接、纸质签切换；分享授权、协商、字段选项、枚举、用户组同步与草稿删除 | 3.1 详情；3.2～3.4 创建与变更/终止；3.5～3.7 审批；3.8 归档与打印；3.9 签署；3.10 分享与授权；3.11 协商；3.12～3.14 配置、分组与草稿清理 | **两种授权，按操作区分**<br>详情、创建等均支持；个人任务/评论和枚举仅个人；提交/重提、归档更新、签署等仅应用 |
| **智书合同摘要**<br>`zhishu-contract-summary` | 按合同编号或完整名称查询摘要，或为单个本地合同文件生成独立摘要 | 合同摘要查询与生成 | **仅个人授权** |
| **人员查询**<br>`contract-cli-employee` | 按姓名或部门查询人员及状态，取得真实候选 | 4.1 同名人员消歧 | **仅个人授权** |
| **部门查询**<br>`contract-cli-department` | 查询部门候选、层级和状态 | 5.1 定位业务部门 | **仅个人授权** |
| **合同智能审查**<br>`contract-cli-review` | 单份外部文件与系统合同审查，主体提取、任务续查、风险展示 | 6.1 外部文件审查；6.2 系统合同审查 | **仅个人授权**<br>还需具备相应合同、正文和审查权限 |
| **审查规则维护**<br>`contract-cli-review-rule` | 查询及单项维护审查清单、规则和分组；内置清单只读，删除受限制 | 7.1 查询审查标准；7.2 补充审查规则 | **仅个人授权**<br>维护操作还受清单/规则权限及内置配置限制 |
| **交易方管理**<br>`contract-cli-mdm-vendor` | 交易方查询、创建、局部更新、启停；另有应用身份全量更新/分页和证件查询 | 8.1 查询交易方；8.2 局部更新资料 | **两种授权，按操作区分**<br>查询、创建和局部更新均支持；启停仅个人；全量更新/分页及证件查询仅应用 |
| **法人主体管理**<br>`contract-cli-mdm-legal` | 法人主体列表及详情、按编码查询、创建与更新 | 9.1 确认签约主体；9.2 创建法人主体 | **两种授权，按操作区分**<br>列表和按 ID 查询均支持；按编码查询、创建和更新仅应用 |
| **主数据字段配置**<br>`contract-cli-mdm-fields` | 查询主数据字段定义，为填写资料提供依据 | 10.1 核对字段要求 | **两种授权，按操作区分**<br>交易方/法人字段均支持；交易方风险字段仅个人 |
| **固定汇率维护**<br>`contract-cli-mdm-exchange` | 固定汇率查询与更新 | 11.1 查汇率；11.2 更新配置 | **仅应用授权** |
| **主数据文件下载**<br>`contract-cli-mdm-file` | 根据主数据文件 ID 下载附件 | 12.1 下载主数据附件 | **仅应用授权** |
| **付款管理**<br>`contract-cli-payment` | 付款申请、付款计划和付款记录的查询与维护 | 13.1 查询付款申请；13.2 登记付款记录 | **仅应用授权** |
| **审批矩阵维护**<br>`contract-cli-rule` | 矩阵及列结构维护、规则行查询/维护/复制、导入计划与执行、预发布和发布 | 14.1 整理并导入规则；14.2 核验和发布 | **两种授权**<br>个人身份用于合同产品，并需规则管理权限；操作按所选身份留痕 |
| **事件出口 IP 查询**<br>`contract-cli-event` | 分页查询事件出口 IP | 15.1 准备白名单资料 | **仅应用授权** |

## 身份支持速查

| 命令 | 支持身份 | 备注 |
| --- | --- | --- |
| `auth login/status/use/logout` | user/app | 用于登录、检查授权、切换默认身份和退出登录 |
| `contract get/search` | user/app | user 搜索按当前用户权限查询；app 搜索支持合同编号精确查询和组合条件 |
| `contract search-v2` | app-only | 按合同编号做 ES 模糊或分号批量搜索 |
| `contract summary query/generate` | user-only | 按合同编号或完整名称查询摘要，或上传单个 doc/docx/pdf 文件生成独立摘要 |
| `contract create/sync-user-groups/text` | user/app | app 创建时请求体需要自行包含 `create_user_id` |
| `contract upload-file/category list` | user/app | 创建合同前可查询分类树，取当前身份可用末级分类的 `abbreviation` |
| `contract template list/get/instantiate` | user/app | 模板实例命令只接收请求体，不接模板 ID 位置参数 |
| `contract enum list` | user-only | 当前不支持 app |
| `contract field update/form attribute list` | app-only | 更新字段选项或查询合同流程字段 |
| `contract sign switch-to-paper/sign-url get` | app-only | 电子签转纸质签，或获取合同签署链接 |
| `contract authorization grant`、`contract esign *` | app-only | 合同授权，以及个人/企业实名认证链接 |
| `contract submit/resubmit/patch/delete` | app-only | 写操作，需确认合同状态满足后端要求；删除前确认目标合同 |
| `contract download-file` | user/app | 个人身份还需传 `--contract`；下载建议显式传 `--output-file` |
| `contract print-file` | app-only | 生成平台文件 ID 后，再调用下载命令保存文件 |
| `contract share get/batch-create` | app-only | 查询分享记录或批量创建分享 |
| `contract cooperation link/record/search` | app-only | 查询协商邀请链接、操作记录或协商列表 |
| `contract cooperation file get/download` | app-only | 查询或下载协商文件，下载建议传 `--output-file` |
| `contract approval start` | app-only | 按权限处理已有流程；不是创建新审批流程的入口 |
| `contract approval get` | user/app | 查询审批实例详情 |
| `contract approval comment list/create`、`contract approval task list/approve/reject` | user-only | 查看或添加评论、查询个人任务、通过或拒绝待办；盖章和归档节点需回合同系统处理 |
| `payment create/update/get/list` | app-only | 付款申请创建、更新、详情和列表 |
| `payment plan notify/search` | app-only | 同步付款计划或搜索付款计划 |
| `payment record create/update/get/list` | app-only | 付款记录创建、更新、详情和按计划查询 |
| `mdm vendor list/get` | user/app | 查询交易方列表或详情 |
| `mdm vendor create/update/list-all/query-by-cert` | 按操作区分 | create 支持 user/app；update、list-all、query-by-cert 仅 app；应用写操作要求 `--user-id` |
| `mdm vendor patch` | user/app | 局部更新交易方；应用身份要求 `--user-id`，个人身份从授权中取得操作人 |
| `mdm vendor enable/disable` | user-only | 启用或停用交易方 |
| `mdm legal list/get <id>` | user/app | 查询法人实体列表或按 ID 获取详情 |
| `mdm legal get --code/create/update` | app-only | 按编码查询、创建或更新法人实体；写操作要求 `--user-id` |
| `mdm fields list` | user/app | app 当前支持 `vendor` 和 `legal_entity`；user 还支持 `vendor_risk` |
| `mdm fixed-exchange-rate get/update` | app-only | 查询或维护固定汇率 |
| `mdm file download` | app-only | 下载主数据附件，Agent/脚本场景建议传 `--output-file` |
| `event outbound-ip list` | app-only | 分页查询事件出口 IP 白名单 |
| `rule table list/pre-release/release/column-headers list` | user/app | 查询、预发布、发布规则表或查询列头；个人身份需合同规则管理权限 |
| `rule table row create/get/list/search/update/delete` | user/app | 规则行完整生命周期；个人身份需合同规则管理权限；搜索支持 `--page-size` 和 `--page-token` |

应用身份下，`mdm vendor create/update` 和 `mdm legal create/update` 会要求 `--user-id`，用于提供真实操作人。`mdm vendor patch --as app` 同样要求该参数；个人身份的交易方创建和局部更新从当前授权中取得操作人，不允许传 `--user-id`。

## 安装后验证

| 验证目标 | 验证方式 | 成功标志 |
| --- | --- | --- |
| CLI 已安装或升级 | `contract-cli --version` | 当前实际执行的 CLI 版本与本次安装版本一致 |
| CLI 路径正确 | macOS / Linux：`command -v contract-cli`；Windows PowerShell：`Get-Command contract-cli -All` | 确认实际调用路径；skills 安装和业务调用使用同一个新版 CLI |
| CLI 帮助可查看 | `contract-cli --help` | 输出命令帮助 |
| skills 目录正确 | 查看当前客户端配置或已加载 skill 的真实路径，并与安装输出的目标目录核对 | 安装到当前客户端实际读取的目录 |
| skills 文件已同步 | 用 `contract-cli skills list` 获取预期清单，逐项检查目标目录中的 `SKILL.md` 与附属文件 | 名称和数量与内置清单一致；版本与 CLI 一致；已有技能也已更新，附属文件完整 |
| 客户端已加载新版 | 按客户端要求刷新技能或新建会话，检查实际加载的技能清单、来源路径和版本 | 确认加载新版后才通过；无法确认时标记“文件已更新，客户端加载待验证” |
| profile 已创建 | `contract-cli auth status --profile contract --as user` | 不再提示 profile 不存在 |
| user 授权正常 | `contract-cli auth status --profile contract --as user` | 显示 user 授权状态正常 |
| app 授权正常，可选 | `contract-cli auth status --profile contract --as app` | 显示 app 授权状态正常 |
| Agent 使用验证 | 授权完成后，向 Agent 发送合同详情查询等只读示例 | Agent 能选择并执行对应命令；本项不能替代技能文件和加载验证 |

CLI 版本正确、命令执行成功或 `skills list` 显示完整，都不能单独作为同步升级完成的依据。技能数量以当前 CLI 内置清单为准，不固定为某个数量。

## 常用命令速查

查看命令总览：

```text
contract-cli --help
```

查看具体命令参数：

```text
contract-cli contract search --help
```

常用命令：

| 操作目标 | 对应命令 |
| --- | --- |
| 初始化与查看版本 | `contract-cli config add --env prod --name contract`<br>`contract-cli version` |
| 登录/检查 user 身份 | `contract-cli auth login --profile contract --as user`<br>`contract-cli auth status --profile contract --as user` |
| 登录/检查 app 身份 | `contract-cli auth login --profile contract --as app --app-id <app_id> --app-secret <app_secret>`<br>`contract-cli auth status --profile contract --as app` |
| 切换默认身份 | `contract-cli auth use --profile contract --as app` |
| 查询合同详情 | `contract-cli contract get <contract-id> --profile contract --as user` |
| user 权限内搜索合同 | `contract-cli contract search --profile contract --as user --input-file contract-search.json` |
| app 精确/组合搜索合同 | `contract-cli contract search --profile contract --as app --input-file contract-search.json` |
| app 模糊/批量搜索合同编号 | `contract-cli contract search-v2 --profile contract --as app --input-file contract-search-v2.json` |
| 查询/生成合同摘要 | `contract-cli contract summary query --profile contract --as user --contract-number <number>`<br>`contract-cli contract summary generate --profile contract --as user --file ./合同.pdf` |
| 查询分类并创建合同 | `contract-cli contract category list --profile contract --as app --lang zh-CN`<br>`contract-cli contract create --profile contract --as app --input-file contract-create.json` |
| 提交/重新提交合同 | `contract-cli contract submit <contract-id> --profile contract --as app`<br>`contract-cli contract resubmit <contract-id> --profile contract --as app --input-file resubmit.json` |
| 更新/删除草稿合同 | `contract-cli contract patch <contract-id> --profile contract --as app --input-file patch.json`<br>`contract-cli contract delete <contract-id> --profile contract --as app` |
| 读取正文/同步用户分组 | `contract-cli contract text <contract-id> --profile contract --as user --full-text`<br>`contract-cli contract sync-user-groups --profile contract --as app` |
| 上传/下载/生成合同文件 | `contract-cli contract upload-file --profile contract --as app --file ./附件.pdf --file-type attachment`<br>`contract-cli contract download-file <file-id> --profile contract --as app --output-file ./contract.pdf`<br>`contract-cli contract print-file --profile contract --as app --input-file print-file.json` |
| 模板与枚举 | `contract-cli contract template list --profile contract --as app --page-size 20`<br>`contract-cli contract template get <template-id> --profile contract --as app`<br>`contract-cli contract template instantiate --profile contract --as app --input-file template-instance.json`<br>`contract-cli contract enum list --profile contract --as user --type contract_status` |
| 字段选项与流程字段 | `contract-cli contract field update --profile contract --as app --input-file field-update.json`<br>`contract-cli contract form attribute list --profile contract --as app --category-id <category-id> --business-type-code 0` |
| 签署能力 | `contract-cli contract sign switch-to-paper --profile contract --as app --business-id <contract-id> --business-type-code 0`<br>`contract-cli contract sign-url get <contract-id> --profile contract --as app` |
| 合同授权与实名认证链接 | `contract-cli contract authorization grant --profile contract --as app --input-file authorization.json`<br>`contract-cli contract esign personal-auth-url --profile contract --as app --input-file personal-auth-url.json`<br>`contract-cli contract esign org-auth-url --profile contract --as app --input-file org-auth-url.json` |
| 合同分享 | `contract-cli contract share get <contract-id> --profile contract --as app`<br>`contract-cli contract share batch-create --profile contract --as app --input-file batch-share.json` |
| 协商列表、链接与记录 | `contract-cli contract cooperation search --profile contract --as app --input-file cooperation-search.json`<br>`contract-cli contract cooperation link get <contract-id> --profile contract --as app`<br>`contract-cli contract cooperation record get <contract-id> --profile contract --as app` |
| 协商文件 | `contract-cli contract cooperation file get <contract-id> --profile contract --as app`<br>`contract-cli contract cooperation file download <file-id> --profile contract --as app --output-file ./cooperation.docx` |
| 流程审批 | `contract-cli contract approval start <process-instance-id> --profile contract --as app --input-file approval.json`<br>`contract-cli contract approval get <process-instance-id> --profile contract --as app` |
| 查询交易方 | `contract-cli mdm vendor list --profile contract --as user --name "示例交易方"`<br>`contract-cli mdm vendor get <vendor-id> --profile contract --as user` |
| 维护/全量查询交易方 | `contract-cli mdm vendor create --profile contract --as app --user-id <operator-user-id> --input-file vendor-create.json`<br>`contract-cli mdm vendor update <vendor-id> --profile contract --as app --user-id <operator-user-id> --input-file vendor-update.json`<br>`contract-cli mdm vendor list-all --profile contract --as app --page-size 10`<br>`contract-cli mdm vendor query-by-cert --profile contract --as app --certification-id <certification-id> --ad-country CN` |
| 查询法人实体 | `contract-cli mdm legal list --profile contract --as user --page-size 20`<br>`contract-cli mdm legal get <legal-entity-id> --profile contract --as user`<br>`contract-cli mdm legal get --profile contract --as app --code <legal-entity-code>` |
| 维护法人实体 | `contract-cli mdm legal create --profile contract --as app --user-id <operator-user-id> --input-file legal-create.json`<br>`contract-cli mdm legal update <legal-entity-id> --profile contract --as app --user-id <operator-user-id> --input-file legal-update.json` |
| 字段配置、汇率与主数据文件 | `contract-cli mdm fields list --profile contract --as user --biz-line vendor`<br>`contract-cli mdm fixed-exchange-rate get --profile contract --as app --source-currency CNY --target-currency USD --effective-date 2026-08-12`<br>`contract-cli mdm fixed-exchange-rate update --profile contract --as app --input-file fixed-exchange-rate.json`<br>`contract-cli mdm file download <file-id> --profile contract --as app --output-file ./attachment.bin` |
| 付款申请 | `contract-cli payment create --contract <contract-id> --profile contract --as app --input-file payment.json`<br>`contract-cli payment update <payment-id> --contract <contract-id> --profile contract --as app --input-file payment-update.json`<br>`contract-cli payment get <payment-id> --contract <contract-id> --profile contract --as app`<br>`contract-cli payment list --contract <contract-id> --profile contract --as app --page-size 10` |
| 付款计划与付款记录 | `contract-cli payment plan notify --profile contract --as app --input-file notify.json`<br>`contract-cli payment plan search --profile contract --as app --input-file payment-plan-search.json`<br>`contract-cli payment record create --contract <contract-id> --payment <payment-id> --profile contract --as app --input-file payment-record.json`<br>`contract-cli payment record list --plan <payment-plan-uuid> --profile contract --as app` |
| 查询事件出口 IP | `contract-cli event outbound-ip list --profile contract --as app --page-size 10` |
| 查询规则表与列头 | `contract-cli rule table list --profile contract --as app --product-id <product-id> --group-id <group-id> --page-size 10`<br>`contract-cli rule table column-headers list --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>` |
| 维护和搜索规则行 | `contract-cli rule table row create --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --input-file row.json`<br>`contract-cli rule table row search --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --page-size 10 --input-file row-search.json`<br>`contract-cli rule table row update <row-id> --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --input-file row.json`<br>`contract-cli rule table row delete <row-id> --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>` |
| 预发布/发布规则表 | `contract-cli rule table pre-release --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>`<br>`contract-cli rule table release --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>` |

## 推荐 Agent 提示词

第一次让 Agent 处理合同开放平台任务时，可以直接复制下面这段：

```text
请使用 contract-cli 帮我处理合同开放平台任务。

执行前请先确认：
1. 当前 profile 名称；
2. 这个场景应该使用 user 还是 app；
3. 写操作需要的 JSON 参数是否已经齐全；
4. 删除、提交、更新等操作是否确认可以执行。

如果缺少合同 ID、交易方 ID、法人主体 ID、模板 ID、create_user_id 或必填 JSON 字段，请先问我，不要自行猜值。



```

## 常见问题

### 安装后提示 `contract-cli: command not found` 怎么办？

通常是 CLI 没有安装成功，或安装目录不在 PATH 中。先安装或升级 CLI：

```text
npm install -g @qfeius/contract-cli@latest --registry https://registry.npmjs.org --foreground-scripts
```

随后检查实际执行路径：macOS / Linux 使用 `command -v contract-cli`，Windows PowerShell 使用 `Get-Command contract-cli -All`。若找不到命令或仍指向旧版本，先修正 PATH 并重新打开终端，再检查：

```text
contract-cli --version
```

不要仅凭 `npx` 输出版本判断 PATH 中的 CLI 已可用。CLI 恢复后，请按前文“安装 CLI 与 Agent skills”同步更新当前客户端的配套 skills。

### 提示 `profile "contract" not found` 怎么办？

说明还没有初始化本地 profile。执行：

```text
contract-cli config add --env prod --name contract



```

### 提示未授权怎么办？

根据当前使用身份分别处理：

```text
contract-cli auth login --profile contract --as user



```

或：

```text
contract-cli auth login --profile contract --as app --app-id <app_id> --app-secret <app_secret>



```

### 在飞书 Aily 智能体中，如何使用个人身份完成授权？

当用户要求在飞书 Aily 智能体中以个人身份使用 `contract-cli`，或常规 `contract-cli auth login --as user` 因本地回调不可用而无法完成时，智能体应先读取并严格参照 [《contract-cli Manual User Auth》](https://ysi13ckdb9.feishu.cn/wiki/C40RwhyUliMhtzkIPrGcXZJgnzb)，自主引导用户完成授权，给用户发送的授权链接要直接可点击。**如果用户消息本身是包含 `/callback?`、`code=` 和 `state=` 的完整 callback URL，必须立即判定为“完成授权”阶段；此时禁止再次执行 `prepare`、禁止生成新授权链接。**

**智能体应按以下流程执行：**

1. 检查并初始化 `contract` profile；按参考文档一次性创建授权 helper 脚本。
2. 仅执行一次 `prepare`，生成本轮专属的 `AUTH_URL` 和 `PENDING`。在一对一会话的同一条回复中，同时发送授权链接，并把原始 `PENDING` 标记为 `AUTH_SESSION` 保留在对话上下文中。若当前是公开群聊，先引导用户转到私聊，再发起授权。
3. 提示用户打开授权链接并完成授权；即使跳转后的 `127.0.0.1` 页面打不开，也要复制浏览器地址栏中的完整 callback URL，并回复包含 `AUTH_SESSION` 的那条授权消息。
4. 收到 callback URL 后，从上一条授权回复中取回同一轮的 `AUTH_SESSION`，作为 `--pending` 执行 `complete`。如果新一轮 sandbox 中 helper 脚本不存在，只重新创建 helper 脚本，不能再次执行 `prepare`。
5. 如果当前上下文找不到 `AUTH_SESSION`，先请用户引用上一条授权消息或粘贴其中的授权会话码，仍然不能直接生成新链接。只有确认会话码缺失、过期或与 callback 的 `state` 不匹配时，才重新发起一轮授权；新一轮也必须跨轮保留新的 `AUTH_SESSION`。完成后执行 `contract-cli auth status --profile contract --as user` 验证并告知结果。

状态流转必须是 `未授权 → 等待回调 → 完成授权 → 已授权`，禁止从“等待回调”重新跳回“发起授权”。如果参考指南中的通用步骤未明确说明跨轮状态保存，以本常见问题中的 callback 优先级和 `AUTH_SESSION` 规则为准。`AUTH_SESSION` 属于临时凭据，只能在一对一会话中保留，不要发送到公开群聊；任何时候都不要输出 `access_token`、`refresh_token` 或 `app_secret`。

### Agent 不会使用 `contract-cli` 怎么办？

可能是 skills 未安装、自动同步失败、当前客户端目录未被覆盖，或当前会话尚未加载新版。先查看安装输出、确认实际读取目录和文件版本；仅在该目录需要完整安装或更新时，核实备份后使用当前新版 CLI 执行：

```text
contract-cli --version
contract-cli skills list
contract-cli skills install --target "<当前客户端实际读取的 skills 目录>" --force
```

请将占位目录替换为已确认的真实路径，不能直接照抄占位内容。省略 `--target` 默认使用 Codex 目录；省略 `--force` 会跳过已有技能。

`skills list` 仅显示 CLI 内置清单，不能单独证明安装或加载成功。请对照清单核对目标目录中的技能版本和附属文件，再刷新技能或新建会话，确认客户端已加载新版。未确认加载前，报告“文件已更新，客户端加载待验证”。

首次安装或需要同时升级 CLI 时，请按前文“安装 CLI 与 Agent skills”执行完整流程。

### 不清楚某条命令的参数怎么办？

使用对应命令的 `--help`：

```text
contract-cli contract search --help
```

优先查看已安装 skills 中的 `SKILL.md` 和 `references/`；查阅 GitHub 在线文档时，请选择与 CLI 版本对应的发布标签。

## 参考

| 类型 | 信息 |
| --- | --- |
| npm 包 | `@qfeius/contract-cli` |
| Agent skills | 随当前 CLI 内置分发；全局升级时同步有效的已记录位置，首次或当前目录未覆盖时，使用 `contract-cli skills install --target "<当前客户端实际读取的 skills 目录>" --force` 完整安装。 |
| GitHub 仓库 | [qfeius/contract-cli](https://github.com/qfeius/contract-cli)，用于查看源码和历史版本 |
| 命令帮助 | `contract-cli --help` |
| 命令文档 | 已安装 skills 中的 `SKILL.md` 和 `references/`；在线文档请选择与 CLI 版本对应的发布标签 |

安装目录以当前客户端实际读取位置为准。CLI 升级时先检查自动同步结果；未覆盖该位置时再指定目录完整安装，并刷新或新建会话验证加载。同步失败须保留原始结果，不能将后续手工补装说成自动同步成功。

## 版本记录

| 序号 | 版本 | 发布内容 | 发布日期 |
| --- | --- | --- | --- |
| 1 | v1.0 | 新增客户使用指南 | 2026-04-29 |
| 2 | v1.1 | 补充典型使用场景，适用于 `contract-cli@1.2.0` 及后续版本 | 2026-05-15 |
| 3 | v1.2 | 按开放平台 CLI 文章风格重排结构与表达，原文档保持不变 | 2026-05-19 |
| 4 | v1.3 | 合并身份支持、模板、分类、上传、枚举、字段配置和推荐 Agent 提示词 | 2026-05-19 |
| 5 | v1.4 | 补充 GitHub 开源仓库入口 | 2026-05-19 |
| 6 | v1.5 | 优化 Skill 能力 | 2026-06-03 |
| 7 | v1.6 | 发布付款申请、付款计划、付款记录与流程审批能力 | 2026-07-06 |
| 8 | v1.7 | 新增cli命令，覆盖合同搜索 V2、字段/签署/授权/协商，MDM 写入与汇率/文件，事件出口 IP 和审批矩阵；同步完善 Agent Skills 参数约束，并修复敏感日志与规则行搜索分页问题 | 2026-08-12 |
| 9 | v1.9.5 | 按 15 个 Skill 重构用户场景和能力地图，补齐合同创建、审批等 36 个场景；增加中文简称及个人 / 应用授权标记。 | 2026-10-13 |
