---
name: zhishu-contract-summary
description: "使用 contract-cli 按合同编号或完整名称查询智书合同保存的总结和结构化摘要，或为单个本地 doc/docx/pdf 文件生成独立摘要。当用户要查合同摘要、结构化摘要，或上传本地合同文件生成摘要时触发。"
---

# 智书合同智能摘要

CRITICAL — 开始前 MUST 先读取 [../contract-cli-shared/SKILL.md](../contract-cli-shared/SKILL.md)。

本 Skill 是产品维护规则在当前 `contract-cli` 能力上的适配。只使用以下两个结构化命令，不调用源码、数据库、作者本机脚本或未开放的通用 API：

- `contract-cli contract summary query`
- `contract-cli contract summary generate`

## 环境与授权

- 当前 CLI 仅支持 `prod`；用户明确要求非生产环境时，说明当前版本不支持，不自动改用 prod。
- 两个命令都只支持 `--as user`，调用人来自用户 OAuth Token；不得用 app 身份或 `--user-id` 代替当前用户。
- 按共享 Skill 复用有效 Device 授权；未授权时执行规定的 `auth init` / `auth complete` 流程，不索取 Token、Cookie 或 App Secret。
- 摘要接口还会校验当前用户的智书席位、摘要功能权限和合同读取权限。权限失败不等同于合同不存在。

## 选择命令

- 用户给出合同编号或完整合同名称，要查询已有摘要：使用 `summary query`。
- 用户给出本地合同文件，要直接生成独立摘要：使用 `summary generate`。
- 合同查询与文件直出是两条不同链路。不要因为合同查询失败就下载正文再调用文件生成，也不要把文件生成结果说成已关联到某份智书合同。

## 按合同查询

编号与完整名称必须二选一，接口执行精确匹配。优先复用用户原始输入，不把 `*` 当通配符，不自行改写名称。

```bash
contract-cli contract summary query --profile contract --as user --contract-number '<合同编号>' --output json
contract-cli contract summary query --profile contract --as user --contract-name '<完整合同名称>' --output json
```

也可通过 `--input-file` 或 `--data` 传 JSON；只允许 `contractNumber` / `contract_number` 与 `contractName` / `contract_name` 两组字段中的一组。字段契约见 [连接器与命令契约](references/connector-contract.md)。

查询接口可能在没有可用摘要时触发生成，因此一次查询请求同时授权这一可能的生成动作。每个用户请求最多调用一次：超时、断网、5xx、结果不确定或返回生成中时，不要自动轮询或重复调用；如需刷新，等待用户新的明确请求。

- `code != 0` 或 `success=false`：按真实 `msg`、错误码和 trace 信息说明失败。
- 合同不存在：请用户核对编号或完整名称。
- 同名合同不唯一：请用户改用合同编号；不要猜选。
- `status=2`：展示服务端返回的总结和结构化摘要。
- `status=1`：只说明“摘要正在生成中，请稍后再试”，不展示响应中可能残留的 `overview`、`summary` 或 `structuredSummary`，本轮不重试。
- `status=3`：展示 `errorCode` / `errorMsg`，不自动重新生成。

## 本地文件生成

只接受一个非空 `doc`、`docx` 或 `pdf` 文件，最大 200MB。文件名后缀必须与允许类型一致。

```bash
contract-cli contract summary generate --profile contract --as user --file '<本地路径>' --output json
```

需要覆盖 multipart 文件名时使用 `--file-name`。命令直接调用生成接口，只发送一个 `file` 字段，不经过独立上传接口，不接受 `--input-file` 或 `--data`。生成会消耗服务端资源；请求结果不确定时禁止自动重试。

## 展示结果

按 [摘要响应与展示规则](references/summary-and-card.md) 解释字段，按 [展示与交互规则](references/display-and-interaction.md) 输出：

- 合同查询优先展示 `contractName`、`contractNumber`、状态和是否命中缓存。
- 合同查询仅在 `status=2` 时展示摘要；文件直出成功时按实际返回字段展示。分别保留 `overview`、`summary` 与 `structuredSummary` 的原文和顺序，不自行补写合同正文或法律结论。
- 部分字段缺失时交付已有内容并准确说明，不把 `summaryId` 或 `summaryVersionId` 当成正文版本证明。
- 文件直出接口没有合同 ID、编号或名称时是正常契约，不推断关联合同。

## 当前能力边界

当前 CLI 不提供发送到关联协商群的命令，也不提供卡片发送回执查询。用户要求外发时，先交付摘要结果，再说明当前 CLI 的具体缺口；不得改用机器人、普通文本或其他合同群代替。

验收场景见 [验收场景](references/acceptance.md)。
