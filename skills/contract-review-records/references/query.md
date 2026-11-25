# 查询协议

本参考维护本 Skill 实际使用的请求、结果和停止条件。全部请求复用同一当前 profile 和个人 USER。授权、续期及传输错误沿用 auth／contract-cli-shared，不叠加重试或切换通道。

## 审查历史的授权方式

沿用当前环境、租户和个人 USER 的现有授权。以本人 history 的实际业务响应判断这一条 REVIEW 路由是否可用。`auth status` 显示 `authorized` 或 CLM 合同接口可访问，只说明各自的授权或接口状态。

已验证的登录方式是 CLI 的官方用户 OAuth 动态授权流程。若当前会话收到 HTTP401／业务码99100401“Token验证失败”，停止本次查询，说明下游没有接受当前授权。支持动态 OAuth 的环境中，用户同意重新授权后，通过宿主 `auth` 流程对原 profile 发起官方登录并由用户完成授权。在实际 CLI 帮助确认支持时，登录入口为 `contract-cli auth login --profile <profile> --as user`。完成后复查同一 profile、环境和 USER，再对同一路径重试一次。设备授权与动态 OAuth 分别核验，不根据这个汇总错误猜测过期原因。

授权失败的回报只保留环境、HTTP／业务码、可安全转述的消息及 trace。原样错误正文可能含认证附加字段，不能转发。保留 CLI 自身可信过期信号的内部刷新机制，无需另行刷新、读取浏览器会话、注入凭证或更换身份。

## 本人有效历史列表

REVIEW family，GET `/open-web/web/review/v3/smartAudit/history`。无 body，无自建授权或可信身份头。

```sh
contract-cli api call --channel web --profile <profile> --as user --output json \
  GET /open-web/web/review/v3/smartAudit/history \
  --query 'page=1' --query 'pageSize=100' \
  --query 'appType=CLM' --query 'reviewMode=all'
```

`<profile>` 来自本次已确认上下文；后续只将 page 替换为下一页，不改变身份及业务条件。

| query | wire 类型与约束 | 本流程取值 |
| --- | --- | --- |
| page | 十进制整数，至少1，默认1 | 首次1，之后逐页加1 |
| pageSize | 整数1～100，默认10 | 100 |
| reviewMode | review_rule／playbook／all；默认或空串为review_rule | all，包含两类审查模式 |
| appType | 字符串，最长32 | CLM，显式限定合同产品 |
| businessId | 可选字符串 | 始终省略，用于跨业务发现 |

`reviewMode=all` 由 V3 历史列表服务端请求定义支持。服务端在 `all` 时不限定审查模式，省略该参数则回到 `review_rule` 默认值。前端调用方的请求类型可能省略此字段，查询本人跨模式历史时使用服务端定义。

本接口没有起止时间、includeInvalid 或调用人参数，不发送杜撰字段。服务端从认证上下文取当前租户和用户，并过滤当前有效、web、无渠道标记的主任务。

成功须同时满足传输成功、业务 `code=200`、data 为有效分页对象。主要结果路径：

| 路径 | 类型与含义 |
| --- | --- |
| data.items | 当前页数组；空数组不能脱离 total／页信息单独解释 |
| data.total | 同一服务端过滤条件的任务总数，不是期间内合同数 |
| data.page、data.pageSize | 实际页码、页大小 |
| items[].taskId、fileId | 字符串形式的审查任务 ID、AI 审查文件 ID；不得互换 |
| items[].fileName | 文件名；关联文件缺失时可为空字符串 |
| items[].createTime | 带时区的创建时间；正常为北京时间 ISO 字符串 |
| items[].taskStatus | 4准备、0运行、1完成、2失败、3跳过 |
| items[].businessId、appType | 可空的业务定位；仅 CLM 来源适用本流程 |
| items[].smartAuditGroupId | 可空审查组 ID；不等于合同 ID |
| items[].version | 审查结果协议版本，不是合同文件版本 |
| items[].reviewMode、playbooks | 审查模式及可选 Playbook 展示信息 |

记录 ID 已由 Web 响应转为字符串，后续仍须无损保留。创建时间缺失、格式无效或没有可解释时区时，不能将该条归入某月；保留为时间待确认项。

分页按 `page × pageSize >= total` 结束；total=0 且首个成功页为空才是服务端范围空结果。返回页码不符、未到结束条件却出现异常空页、结构缺失或分页总数变化造成不稳定时，标部分结果，不凭空补页或宣称固定快照。服务端按请求时间排序，所以不能按创建日期提前结束。

缺当前用户上下文为 HTTP400／业务码400001013；参数问题422；认证401／403及其他业务错误均按真实失败处理。业务报错不是零条结果。

若首个 history 请求收到 HTTP401／业务码99100401，按[审查历史的授权方式](#审查历史的授权方式)停止本次业务链并说明当前授权情况。

## 业务对象识别

只读识别的候选 ID 必须来自上文真实 CLM 业务定位，不能来自文件名、groupId、时间排序或用户随意输入的其他 ID。正式合同及预审接口使用正数十进制长整型 ID；返回非此格式的业务标识时保留为未定位，不截取数字或强行转换。

### 合同

复用 contract-cli-contract 的查询与成功判断：

```sh
contract-cli contract get <businessId> --profile <profile> --as user --raw
```

合同 `contract_id` 可能在原始 JSON 中是超过 JavaScript 安全整数范围的数字。保留原始输出，在解析阶段从原始数字词元无损取得十进制字符串，再与候选 ID 比较。可直接读取原始 JSON 字段，或使用宿主提供的无损 JSON 解析器；支持 `JSON.parse` reviver `context.source` 的宿主也可用该原始词元。**禁止先普通 JSON.parse 再 String(number)**；已经失真的数字不能用于比较、去重、URL 或后续请求，须从原始输出重新无损读取。不能只因输出模式叫 JSON 就假定它一定返回字符串 ID。无损读取能力不足时保留为未定位，并说明解析限制，不猜末位数字。

成功后核对 `data.contract.contract_id` 与候选 ID 精确相等，保存：

- `contract_name`：真实合同名。
- `contract_status_code`、`contract_status_name`：当前状态，优先响应中的中文展示值，否则按下表映射；未知码保留真实返回值并说明未识别，不根据英文词、URL 或文件名猜阶段。
- `multi_url.pc_url`：服务端生成的当前原生入口。

状态名缺失时使用下表兜底。它按业务状态含义区分正在编辑、干预中止和协商草稿。`clm-web-status-v2.0` 可能给出不同的展示词，响应中有状态名时保留服务端名称。

| 状态码 | 展示 | 状态码 | 展示 |
| --- | --- | --- | --- |
| 0 | 正编辑 | 1 | 已作废 |
| 2 | 已撤回 | 3 | 审批中 |
| 4 | 已拒绝 | 5 | 审批已通过 |
| 6 | 签订中 | 7 | 已签订 |
| 8 | 归档中 | 9 | 已归档 |
| 10 | 变更中 | 11 | 已变更 |
| 12 | 我方已签约 | 13 | 对方已签约 |
| 16 | 终止中 | 17 | 已终止 |
| 19 | 审批流程被干预中止 | 20 | 预审中 |
| 21 | 预审成功 | 22 | 预审失败 |
| 23 | 已完成 | 30 | 协商中 |
| 31 | 协商草稿 | — | — |

只有明确的“合同不存在”才继续查询预审。`110004` 是无权限；`110125` 也可能在严格策略下表示 `CONTRACT_NOT_FOUND_OR_NOT_VISIBLE`，不能只看数字码就当成不存在。错误含义合并、上下文或策略无法判断、网络/认证失败时停止该对象识别，保留为未定位，不使用其他对象接口绕过拒绝。

### 独立预审

CLM_OPENAPI family，GET `/open-web/web/clm-openapi/v1/pre_audits/<businessId>`；候选沿用上一步明确未命中的业务 ID，不传 query 或 body。

```sh
contract-cli api call --channel web --profile <profile> --as user --output json \
  GET /open-web/web/clm-openapi/v1/pre_audits/<businessId>
```

此接口成功业务码为 **0**，与 REVIEW history 的200不同。要求 data 有效、`data.pre_audit_id` 与候选精确一致，且 `data.business_type_code=24`。

| data 字段 | 解释 |
| --- | --- |
| pre_audit_id | 字符串预审 ID，不是正式合同 ID |
| pre_audit_name | 预审名称 |
| business_type_code | 24，独立预审 |
| status_code | 0初始化／未保存正式合同；1已进入协商 |
| page_url | 服务端返回的真实预审详情页地址 |

页面地址直接使用 page_url，不把它误读为 reviewDetailUrl，也不从域名或 ID 自行拼造。该查询按当前租户及预审创建人控制读取。

110004无权限；110903用户上下文缺失；111583预审不存在；111584当前文件未初始化；110000参数错误；110002系统错误。初始化、参数、系统或权限错误都不是对象不存在。

预审响应没有进入协商后的正式合同关联。status_code=1时只能展示“已进入协商，正式合同待定位”；不能凭同名、同文件名或数值 ID 把它与正式合同去重。

### 旧记录只有组 ID

groupId不能作为合同 ID；GET `/v2/smartAudit/group` 是创建组，也不得用于反查。可以对 history 已返回且缺业务定位的 taskId，做一次纯读元数据补充：

```sh
contract-cli api call --channel web --profile <profile> --as user --output json \
  GET /open-web/web/review/v3/smartAudit/task/status --query 'taskId=<history.taskId>'
```

仅传该条真实 taskId；businessId、appType、visibilityScope均省略，尤其不传 contractResult。该入口先校验当前租户及默认 owner，读取已有任务、子任务和文件身份，不执行 task/info 的历史结果项物化。

必须同时满足 HTTP成功、业务code=200、data有效，以及下列全部条件，才能采用补读的业务定位：

- data.status非空，data.taskId与当前history.taskId精确一致。
- data.fileId与当前history.fileId精确一致。
- **响应** data.appType=CLM，data.businessId非空，且符合后续业务接口的ID格式。

返回业务字段来自已有任务，不用请求参数补值。满足时回到合同／预审只读识别；缺字段、ID不符或appType不同则保留为“旧审查记录未能关联业务对象”。状态查询可能以code200和status=null返回不存在或不可见，不能只凭200认为定位成功，也不能推断该条失效。

补充查询失败只影响该条定位，不代表没有审查记录。不要改用task/info、contractResult、重复试ID或扫描全部合同来兜底；没有定位产物时保留文件名和记录，最终标部分结果。

## 链接

仅使用已核实合同详情的 pc_url 或预审接口的 page_url，且必须是合法 HTTP／HTTPS 页面地址。保留服务端提供的域名、业务定位、签名参数及 fragment；不使用正文链接、文件下载地址或其他用户提供的 URL 替代。

- 已核实的直达形式是：路径为 `/cooperation/detail` 且原URL的cooperationId非空、非0；或原路径匹配 `/task-center/(todo|done|notice)/<非空原生任务定位>`。只识别服务返回的这些形态，不自行构造业务定位。
- 对上述已确认的协商或任务中心入口，用宿主 URL API设置 `targetType=ai-review`，替换已有同名参数而非重复追加；不加 AI taskId。仅协商详情的 from=openPlatform 改为 allCoopList；其他 from 不变。
- 原生分享跳转：仅当服务返回的原路径为 `/bot`、`redirectType=share`、原 businessId 与已核实合同 ID 精确一致，且原 businessTypeCode 非空时，可同样用 URL API设置唯一 `targetType=ai-review`。`businessTypeCode=0` 是有效值；taskInstanceId／noticeId 可缺省或为空，由页面服务选择本人可访问任务，不能自行补造。保留全部原参数和任务标识，不重写 taskInstanceId。标“查看当前审查（按页面权限跳转）”：原页面按当前用户权限选择真实待办／已办／通知任务；非参与者可能回退合同详情，不能承诺一定直达审查面板。条件不满足则保留原地址并标“合同入口”。其他未核实路径同样降级，不自行造任务中心地址。
- 链接用途为“查看当前审查”，页面展示当前最新结果，不保证固定打开本月某条任务，也可能需要网页自身登录与权限。
- 草稿编辑页未核实审查直达能力，仅保留服务真实返回的“合同入口”，说明未提供审查直达，不把追加参数成功当成结果页已验证。
- 预审直接使用 page_url，标“预审详情”；已转协商但正式合同未定位时，不声称该页是新合同的最新审查结果页。
- URL 缺失、协议异常或入口用途不明时写“未提供”。不能为补链接上传、创建对象、重审、生成报告或签发其他产品预览 Token。

链接生成不等于网页联调验收。能够打开的当前原生入口也不能证明期间内审查记录与当前文件版本完全一致。
