# 合同创建命令方案对比

## 1. 文档目的

本文档用于内部会议讨论 `contract-cli contract create` 的命令设计方案。

当前已明确：

- CLI 鉴权沿用现有本地空间授权体系，不直接暴露开放平台原始 token 获取流程
- 合同创建存在两条业务路径
  - 文件上传 + 合同分类模式
  - 模板创建模式
- 命令层希望兼顾人工使用、智能体辅助、脚本自动化和后续可维护性

本文对比两种用户可见命令设计：

- 方案 A：单命令 + `mode`
- 方案 B：拆命令模式

同时补充一个折中建议，供会议时做落地决策。

## 2. 业务前提

### 2.1 文件上传 + 合同分类模式

该模式的典型内部流程：

1. 上传合同文件，获取 `fileId`
2. 根据用户输入的合同分类，转换为创建合同所需分类参数
3. 根据用户输入的交易方、金额、日期、备注等，拼装创建合同参数
4. 调用创建合同接口

### 2.2 模板创建模式

该模式的典型内部流程：

1. 调用模板相关接口生成模板实例
2. 根据模板字段和用户输入补齐 `form`
3. 补充模板之外的公共合同参数
4. 调用创建合同接口

### 2.3 两种模式的共同点

- 最终都要落到“创建合同”的统一能力
- 都需要用户提供交易方、金额、有效期、备注等公共合同信息
- 都适合由智能体帮助做参数补齐
- 都需要保留结构化中间表示，便于调试、审计和回归测试

## 3. 统一的结构化执行格式

无论采用哪一种命令设计，都建议在 CLI 内部统一为一个结构化对象，例如 `ContractCreateSpec`。

其职责是：

- 作为 CLI 的标准执行契约
- 作为智能体生成参数后的承载格式
- 作为脚本和自动化调用的稳定输入格式

建议最小结构：

```json
{
  "mode": "file",
  "operator": "ae721f86",
  "contractName": "2026 年度设备采购合同",
  "remark": "内部评审示例",
  "ourPartyList": [],
  "counterPartyList": [],
  "amount": 100000,
  "currencyCode": "CNY"
}
```

文件模式扩展字段：

```json
{
  "mode": "file",
  "contractFile": "./采购合同.pdf",
  "uploadedFileId": "",
  "categoryCode": "CBG",
  "categoryName": "采购类合同"
}
```

模板模式扩展字段：

```json
{
  "mode": "template",
  "templateNumber": "202203160002",
  "templateId": "",
  "form": [
    {
      "attributeCode": "amount",
      "value": "100000"
    }
  ]
}
```

关于 `spec` 的提供方式，建议统一约定为：

- 普通人工使用时：默认可由智能体生成
- 自动化、批量导入、CI 场景：允许用户或上游系统直接提供
- 无论谁提供，都落到同一份结构化 `spec`

## 4. 方案 A：单命令 + mode

### 4.1 设计思路

用户始终使用一个入口命令：

```bash
contract-cli contract create
```

具体走哪条路径，由显式 `mode` 决定，而不是完全靠智能体猜测。

推荐形式：

```bash
contract-cli contract create --mode file --spec contract-create.file.json
contract-cli contract create --mode template --spec contract-create.template.json
```

不建议的形式：

```bash
contract-cli contract create --data '{"contractName":"示例"}'
```

如果命令层完全不区分模式，而是全部交给智能体隐式推断，会带来较高的不确定性和排障成本。

### 4.2 用户输入示例

#### 文件上传 + 合同分类模式

```bash
contract-cli contract create --mode file --spec contract-create.file.json
```

`contract-create.file.json`：

```json
{
  "mode": "file",
  "operator": "ae721f86",
  "contractFile": "./采购合同.pdf",
  "categoryCode": "CBG",
  "contractName": "2026 年度设备采购合同",
  "remark": "扫描件导入",
  "ourPartyList": [
    {
      "ourPartyCode": "XYZ123"
    }
  ],
  "counterPartyList": [
    {
      "counterPartyCode": "ABC123"
    }
  ],
  "amount": 100000,
  "currencyCode": "CNY",
  "startDate": "2026-04-01",
  "endDate": "2027-03-31"
}
```

#### 模板创建模式

```bash
contract-cli contract create --mode template --spec contract-create.template.json
```

`contract-create.template.json`：

```json
{
  "mode": "template",
  "operator": "ae721f86",
  "templateNumber": "202203160002",
  "contractName": "2026 年度设备采购合同",
  "ourPartyList": [
    {
      "ourPartyCode": "XYZ123"
    }
  ],
  "counterPartyList": [
    {
      "counterPartyCode": "ABC123"
    }
  ],
  "form": [
    {
      "attributeCode": "amount",
      "value": "100000"
    },
    {
      "attributeCode": "currency",
      "value": "CNY"
    }
  ]
}
```

### 4.3 智能体辅助输入示例

如果未来 CLI 支持 agent 驱动输入，可以扩展为：

```bash
contract-cli contract create --mode file --prompt "上传这份采购合同，分类选采购类，对方是 ABC123，我方是 XYZ123，金额 10 万"
```

或：

```bash
contract-cli contract create --mode template --prompt "用模板 202203160002 发起采购合同，对方是 ABC123，金额 10 万"
```

CLI 的内部行为建议是：

1. 智能体生成临时 `ContractCreateSpec`
2. CLI 做结构化校验
3. 可选输出执行计划或打印 `spec`
4. 再执行实际 API 调用

建议同时提供：

```bash
--dry-run
--print-spec
--output json
```

### 4.4 优点

- 用户对外只有一个创建入口，学习成本低
- 对话式、智能体辅助场景更自然
- 文档主入口清晰，产品心智统一
- 后续新增第三种模式时，可沿用 `--mode`

### 4.5 缺点

- help 文案会更复杂，需要解释模式差异和参数约束
- 如果模式、参数和校验处理不好，命令容易变成黑盒
- 单测和回归测试需要覆盖更多模式分支
- 如果 `mode` 允许隐式推断，行为容易不稳定

### 4.6 适用场景

- 主要面向人工使用和智能体协同使用
- 希望对外维持单一入口
- 愿意在内部实现中投入更多结构化校验和执行计划展示能力

## 5. 方案 B：拆命令模式

### 5.1 设计思路

把两条业务路径在命令层显式拆开：

```bash
contract-cli contract create from-file
contract-cli contract create from-template
```

这样用户在命令入口层就完成模式选择。

### 5.2 用户输入示例

#### 文件上传 + 合同分类模式

```bash
contract-cli contract create from-file \
  --contract-file ./采购合同.pdf \
  --category-code CBG \
  --spec contract-create.file.json
```

#### 模板创建模式

```bash
contract-cli contract create from-template \
  --template-number 202203160002 \
  --spec contract-create.template.json
```

### 5.3 智能体辅助输入示例

同样可以支持智能体帮助生成 `spec`：

```bash
contract-cli contract create from-file \
  --contract-file ./采购合同.pdf \
  --category-code CBG \
  --prompt "对方是 ABC123，我方是 XYZ123，金额 10 万"
```

```bash
contract-cli contract create from-template \
  --template-number 202203160002 \
  --prompt "对方是 ABC123，我方是 XYZ123，金额 10 万"
```

### 5.4 优点

- 命令语义非常清楚，用户一眼知道走哪条链路
- help 文案更短，更容易写清楚
- 参数互斥关系简单，校验更直接
- 测试粒度更清晰，排障更容易
- 适合工程团队做长期维护

### 5.5 缺点

- 用户对外需要理解两个入口
- 如果未来模式继续增多，命令树会膨胀
- 对话式智能体场景下，入口略显啰嗦
- 产品展示上不如“单一创建命令”简洁

### 5.6 适用场景

- 工程稳定性和可测试性优先
- 以开发、实施、运维、脚本用户为主
- 团队更偏好显式命令而不是智能编排入口

## 6. 两种方案对比

| 维度 | 方案 A：单命令 + mode | 方案 B：拆命令模式 |
| --- | --- | --- |
| 用户心智 | 单一入口，表面更简洁 | 入口更显式，用户要先选路径 |
| help 文案 | 更复杂 | 更清楚 |
| 参数校验 | 需要处理模式分支 | 更直接 |
| 测试成本 | 更高 | 更低 |
| 排障体验 | 依赖执行计划和日志 | 更容易定位 |
| 智能体协同 | 更自然 | 也可支持，但略显分散 |
| 脚本自动化 | 依赖 `mode/spec` 稳定性 | 更稳定 |
| 扩展性 | 可继续加 `mode` | 命令树会继续增长 |
| 实现复杂度 | 较高 | 中等 |

## 7. 关键讨论点

### 7.1 是否允许智能体隐式猜测模式

不建议。

原因：

- 会降低命令行为可预测性
- 会增加回归风险
- 会降低自动化场景稳定性

建议：

- 单命令方案中，必须有显式 `--mode` 或 `spec.mode`
- 拆命令方案中，模式由命令本身表达

### 7.2 `spec` 文件应由谁提供

建议统一约定：

- 人工交互场景：优先由智能体生成
- 自动化场景：允许用户或上游系统直接提供
- CLI 内部：始终收敛为同一份 `ContractCreateSpec`

因此，`spec` 不是要求普通用户必须手写的文件，而是 CLI 的标准执行格式。

### 7.3 是否继续支持 `--data '{...}'`

建议支持，但不建议作为主路径。

原因：

- 长 JSON 在 shell 中可读性差
- 转义成本高
- 对复杂对象和数组不友好

推荐优先级：

1. `--spec <file>`
2. 智能体生成 `spec`
3. `--data '<json>'`

## 8. 折中建议

如果团队希望同时兼顾产品体验和工程可维护性，推荐采用折中方案：

- 对外文档主入口使用单命令：

```bash
contract-cli contract create --mode file --spec contract-create.file.json
contract-cli contract create --mode template --spec contract-create.template.json
```

- CLI 内部实现拆分为两个独立处理器：

```text
runContractCreateFile(...)
runContractCreateTemplate(...)
```

折中方案的好处：

- 用户视角是统一入口
- 工程实现仍可保持清晰分层
- 测试和日志仍能按模式拆分
- 后续如果会议决定改成拆命令，对内部实现改动较小

## 9. 建议的会议结论模板

会议可以围绕以下结论模板做决策：

### 9.1 若偏产品体验

- 采用方案 A：单命令 + `mode`
- 要求：
  - `mode` 必须显式
  - 必须支持 `--dry-run`
  - 必须支持 `--print-spec`
  - 必须有稳定的 `ContractCreateSpec`

### 9.2 若偏工程稳定性

- 采用方案 B：拆命令
- 要求：
  - 两个子命令分别维护 help、校验和测试
  - `spec` 结构尽量统一，避免内部重复定义

### 9.3 若希望先快后稳

- 第一阶段：对外单命令 + `mode`
- 内部实现按两条 handler 分开
- 第二阶段：根据用户反馈决定是否外显为拆命令

## 10. 实现与测试建议

无论最终选择哪种方案，建议至少覆盖以下测试：

- 命令解析测试
  - `mode` 必填或 `spec.mode` 必填
  - `from-file` 和 `from-template` 的参数校验
- `spec` 解析测试
  - 文件模式字段校验
  - 模板模式字段校验
- 执行计划测试
  - 文件模式必须先上传文件再创建合同
  - 模板模式必须先创建模板实例再创建合同
- 请求映射测试
  - 公共字段映射
  - `categoryCode`、`templateNumber`、`form` 等模式特有字段映射
- 错误提示测试
  - 模式缺失
  - 必填字段缺失
  - 文件不存在
  - 模板不存在
- 可观测性测试
  - `--dry-run`
  - `--print-spec`
  - 日志中是否输出关键步骤

## 11. 当前建议

基于目前讨论，我建议会议优先讨论以下两个落点：

1. 对外是否坚持单一入口
2. 智能体是否只负责生成 `spec`，而不是隐式决定模式

如果没有额外强约束，我更倾向于：

- 对外采用方案 A：单命令 + 显式 `mode`
- 对内采用拆 handler 的实现方式

原因：

- 更适合后续引入智能体辅助
- 不会把模式选择完全交给模型猜
- 能兼顾产品简洁度和工程可维护性
