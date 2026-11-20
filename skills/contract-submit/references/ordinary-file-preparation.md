# 协商完成后独立送审的文件准备

## 适用动作与开关

本分支适用于个人身份在独立申请页**首次提交**的单份原生文件上传合同。按[定位和权限](discovery.md)及[当前表单](form.md#取得当前表单)确认实际动作和编辑权，核同一合同与当前已完成协商轮次绑定、当前实际模式 `enableFormCooperation=false`、未有审批实例。合同申请 `businessTypeCode=0`、原生上传 `sourceTypeCode=0`、普通单合同 `contractTypeCode=0`，无模板、父组或组成员、上版和关联草稿。外部来源、重提、跳过协商、模板和二合一仍走各自已核协议。

缺少实际模式时，先按[定位协议](discovery.md)用严格详情和合同基础信息确认当前轮原始状态为 `3`、当前 USER 是当前轮创建者及合同负责人。确需补读时，按[原协商详情的读取影响与同轮核对](cooperation-submit.md#1-核适用动作和清洁方式)调用一次 `GET /open-web/web/clm/cooperation/info`，从 `data.enableFormCooperation` 取该协商对象的确切布尔值。比较实际合同、协商、轮次、状态和身份。模式无法核实则暂停受影响分支，租户级开关不能代替对象模式。

每条路径都要等当前上传、替换等文件操作完成。若实际对象仍需页面额外生成或同步正文（如非 `isUploadInProcess` 的只读魔方正文），按现有已核协议完成该前置，协议缺失时转原页办理。清洁调用仅处理已有文件。

两个清洁灰度可以各自开关。按用户正在办理的动作读取实际值。

| 场景 | 生效开关 | 分流 |
| --- | --- | --- |
| 协商已经完成，当前从独立申请页首次送审 | `normalContractFileCleanEnabled`。按[当前个人上下文](runtime.md)读同租户 `GET /open-web/web/clm/sso/getUser`，无业务 query 或 body，取 `code=200`、`success=true` 的 `data.systemConfigVO.normalContractFileCleanEnabled`。 | `true` 时按当前文件及变更证据判断是否准备清洁版。`false` 时沿用普通送审校验。字段缺失、返回非布尔值或读取失败时补核这一项，或交付已核原申请页。 |
| 当前仍在协商，动作会结束本轮 | `serverCleanFile`。从同一轮 [协商详情](cooperation-submit.md#1-核适用动作和清洁方式)读取有效值，来源为 `serverGenerateCleanCooperationFile` 配置。 | 适用于单独完成协商及「完成协商并提交审批」。按[协商完成协议](cooperation-submit.md#1-核适用动作和清洁方式)选择接口办理或原协商页。 |

普通送审灰度开启、协商服务端清洁灰度关闭时，本分支仍可准备已登记的变更文件。普通清洁失败是否阻断，由清洁接口结合当前租户的旧清洁策略判定。依照返回的 `status` 办理。协商详情的 `shouldCleanFile` 表示该轮的清洁要求。

## 判断当前文件

从同一合同、同一轮的最终文件角色与绑定、当前文件列表及可信的历史文件记录确认变更。原页 `GET /open-web/web/clm/contract/attachments` 传 `contractId=<已核合同ID>`，无请求体。该读取在特定条件下可初始化编辑器或文件封面；只在授权覆盖相应原页面行为时调用，优先复用当前已核、同对象的文件绑定。匹配 `code=200`、`success=true`，核 `data.contractBaseVO` 的合同与租户，记录 `data.contractText` 的 `textId` 和 `fileId`、`data.contractCauses` 的 `contractCauseId` 与 `fileId` 对应关系。此处「合同附件」是签订依据 `contractCauses`。其他附件和表单附件沿用原规则。保留类型及原始标识，忽略编辑器凭证和临时下载信息。

- 确认正文仍绑定本轮协商最终版，且签订依据完整集合与可信基线一致时，直接走当前文件的送审校验。[严格协商详情](discovery.md)的 `current_round_files[]` 中，`cooperation_file_status_code=4` 标识最终文件，按 `cooperation_file_type_code` 区分正文 `0` 与签订依据 `1`。完成协商时，原页面会复制最终文件到正式合同，产生新的 `fileId`。按角色和槽位比对，要求当前正式正文及各签订依据均有 `afterCooperation=true`，并以 `cooperationFileKey` 和 `cooperationVersion` 唯一匹配本轮最新最终文件的 `cooperation_file_key` 和 `version`。也可使用可信的本轮完成回写快照，按当前正式文件 ID 与全部附属绑定比对。附件集合不完整、最终角色不明、源文件映射不唯一或存在后来替换的证据时，不能凭协商来源判断文件仍未变化。服务实际返回空签订依据集合才能按空处理。
- 已分段完成普通清洁时，先回查合同当前可办理动作、审批实例和文件绑定。已有同一合同与轮次的清洁结果，且当前正文 `textId/fileId` 和完整签订依据绑定均与已核清洁产物一致，表单确认也使用这版文件时，继续送审校验。正文或附件又有变更时重新判断当前文件和确认依据。仅有 `PASSED` 或文件名相同不足以建立这一步的续办依据。
- 换过正文或新增、换过签订依据时，由原页面接口识别当前登记、支持的文件并准备签订版本，包括其支持的 PDF、DOC、DOCX。文件名、版本号、`afterCooperation`、上传时间或主文件相同都不足以证明整个文件集合未变化。页面读取不提供服务端清洁登记标记。若无法证明文件未变化，只有已核当前写入权限及清洁影响且用户确认覆盖本次文件准备时，才按原生接口让服务端判断当前是否有候选；否则转原页面。分清各文件当前绑定与协商最终文件，不能把任何同轮历史版本当最终版。用户明确请求替换文件时，先核当前对象的原生替换能力和权限，替换并完成回读后再做清洁准备。

## 原页面清洁和回查

采用[送审最终确认](submit.md#1-锁定用户最终确认的内容)，明确本次可能清洁并替换当前正文或签订依据文件绑定。先核表单已按页面流程保存并回查，适用的自定义/外部校验已完成。清洁前复核当前身份、同一合同和文件绑定，清洁后按上节取得同对象文件列表。读取或回查权限不能覆盖时转原页办理。

复用 `contract-cli api call --channel web --profile '<已核 profile>' --as user --raw`，按照[调用纪律](runtime.md#数据保真与调用纪律)发 `POST /open-web/web/clm/contract/cleanChangedContractFilesBeforeSubmit`，无 query，JSON 仅 `{ "contractId": "<已核合同ID>" }`。首次申请没有 `taskInstanceId`。调用可能实际执行接受修订与清除批注，绑定清洁文件并使既有电子签章位置需要重新核对。依照 `code=200`、`success=true`、明确的 `data.status` 办理。

| 页面结果 | Skill 处理 |
| --- | --- |
| `PASSED` | 取得 `data.contractText` 与 `data.contractCauses`，再读当前合同的附件列表，核主文 `textId/fileId` 与签订依据完整集合 `contractCauseId → fileId` 一致后采用最新绑定；响应字段缺失或绑定无法比较时保留待核实。后续正文一致性、电子签章位置及确认内容以最新文件为依据。若提交依据已变，重呈受影响项及完整提交表单，再确认。`PASSED` 表示页面允许继续，也包括无候选或存在允许跳过的清洁失败；从该状态本身无法认定文件已清洁。 |
| `NOT_APPLICABLE` | 按原页面回到现行普通送审校验，保留已核当前文件。保留任何仍适用的原页面前置条件。 |
| `BLOCKED` | 显示「合同自动接受修订失败」，按 `data.failedFiles` 给出实际失败文件名称，提供原申请页的查看入口。停止一致性确认与提交动作，保留先前实际保存的项目。 |
| `FILE_CHANGED` | 提示「文件已发生变化，请刷新后重试」。重新读取当前合同和关联文件；若正文或表单的确认依据改变，重新展示并让用户确认后才继续。 |
| 失败、超时、未知或文件回查不符 | 说明清洁结果或绑定待核实，按[未知写入回查](submit.md#5-结果及未知写入)先查当前文件、合同及审批状态，暂停后续流程，禁止自动重发清洁。通道不可用时可交付已核原生页面。 |

当用户需要验证实际修订清洁效果，按当前 USER 权限调用已有 `contract-cli contract download-file '<回查已绑定的fileId>' --contract '<已核合同ID>' --profile '<已核 profile>' --as user --output-file '<本任务私有文件路径>'`。对有修订的 DOCX 样本，在替换后的文件和清洁输出上分别核对正文中的实际插入、删除及修订标记。仅回查到新的文件绑定时，按绑定结果报告。下载前核可用性及必要权限，保护正文和临时地址。

原页入口优先用已取得的当前独立申请地址。当前来自协商页且确认满足以上资格时，使用[已核原页面站点](runtime.md#固定当前个人身份)的 `/apply/edit`，参数 `contractId=实际合同ID`、`cooperationId=实际协商ID`、`from=coop`、`showFeelGood=false`。当前属于其他入口时用该入口已核的来源值。用户限定仅测试或校验时，在文件回查与已授权的校验后停止，按[送审协议的动作限制](submit.md)回查实际审批状态。
