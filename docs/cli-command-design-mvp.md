# contract-cli 极简 MVP 命令设计

## 1. 文档目的

本文档用于在现有完整方案基础上，进一步收敛出一版适合首发落地的极简命令设计。

目标不是定义最终形态，而是回答三个更现实的问题：

1. 第一版 CLI 最少需要哪些命令
2. 如何降低用户学习成本
3. 如何在不牺牲后续扩展性的前提下，尽快让命令可用

## 2. 收敛原则

相比完整方案，MVP 版本明确做以下收敛：

- 少设计命令，多设计统一输入模型
- 复杂业务分支先收进 `input`
- 智能体负责帮助用户生成 `input`
- CLI 负责校验、编排、执行和输出
- 首发不追求“命令树完整”，优先保证“高频路径可跑通”

换句话说：

- 完整版偏“命令即产品能力边界”
- MVP 版偏“命令先尽量薄，能力沉到输入模型和执行引擎里”

## 3. MVP 推荐命令集

### 3.1 必选命令

```bash
contract-cli auth login
contract-cli auth status

contract-cli contract create --input contract-create.json
contract-cli contract get <contract-id>

contract-cli vendor create --input vendor.json
contract-cli vendor get <vendor-id>
```

### 3.2 可选补充命令

如果需要再增加一点点可观测性，可以加：

```bash
contract-cli contract create --input contract-create.json --dry-run
contract-cli contract create --input contract-create.json --print-input
```

说明：

- `--dry-run`：只展示执行计划，不调用实际接口
- `--print-input`：打印规范化后的输入结构，便于排障

### 3.3 首发不建议做的命令

以下命令在 MVP 阶段建议暂缓：

- `contract search`
- `contract template list/get/fields/instantiate`
- `contract create from-file`
- `contract create from-template`
- `vendor fields`
- `entity *`
- `event *`
- `payment *`
- `rule table *`
- `api call`

这些能力后续可以逐步补，但不建议一开始全部暴露给用户。

## 4. 为什么推荐 `--input`

MVP 阶段不建议一上来就设计太多参数。

例如，如果合同创建直接暴露：

```bash
contract-cli contract create --mode template --template-number 202203160002 --counter-party ABC123 --amount 100000
```

短期看似清楚，长期会遇到两个问题：

1. 参数会越来越多
2. 文件模式和模板模式的参数约束会越来越复杂

因此，MVP 更建议把复杂度收敛到：

```bash
contract-cli contract create --input contract-create.json
```

对用户来说，只需要记住：

- 创建合同：`contract create --input`
- 查询合同：`contract get`
- 创建交易方：`vendor create --input`
- 查询交易方：`vendor get`

这样学习成本最低。

## 5. 统一输入模型

### 5.1 设计目标

`input` 是 CLI 的统一结构化输入。

它的作用是：

- 承载用户明确输入
- 承载智能体生成的补全结果
- 作为 CLI 的稳定执行契约
- 作为后续自动化和回归测试的输入基础

### 5.2 合同创建输入

建议统一定义为 `ContractCreateInput`。

#### 文件模式示例

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

执行命令：

```bash
contract-cli contract create --input contract-create.file.json
```

#### 模板模式示例

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

执行命令：

```bash
contract-cli contract create --input contract-create.template.json
```

### 5.3 交易方创建输入

建议统一定义为 `VendorCreateInput`。

```json
{
  "operator": "123123123123",
  "vendor": "V00108006",
  "vendorText": "张三样例",
  "shortText": "王五",
  "vendorCategory": "11",
  "vendorNature": "0",
  "certificationType": "0",
  "certificationId": "913100xxxxx555781R",
  "legalPerson": "张三",
  "contactPerson": "李四",
  "contactMobilePhone": "+8617621685955",
  "email": "shunxing@xxx.com",
  "status": 1,
  "adCountry": "CN",
  "adProvince": "MDPS00000001",
  "adCity": "MDCY00001226",
  "address": "上海市浦东新区世纪大道1000号",
  "associatedWithLegalEntity": true
}
```

执行命令：

```bash
contract-cli vendor create --input vendor.json
```

## 6. 谁来提供 input

MVP 阶段建议明确区分两种使用方式。

### 6.1 普通用户

普通用户不应该被默认要求手写完整 JSON。

更合适的方式是：

- 用户给出高层意图
- 智能体生成 `input`
- CLI 负责执行

例如：

```bash
contract-cli contract create --input /tmp/generated-contract-create.json
```

这里的 `/tmp/generated-contract-create.json` 可以由智能体在本地临时生成。

### 6.2 自动化或脚本场景

在脚本、批量导入、CI 场景下，允许调用方直接提供 `input` 文件：

```bash
contract-cli contract create --input ./fixtures/contract-create.template.json
contract-cli vendor create --input ./fixtures/vendor.json
```

### 6.3 结论

所以，`input` 的定位不是“必须由人手写的配置文件”，而是：

- CLI 的统一执行格式
- 可由智能体生成
- 也可由用户或系统直接提供

## 7. 合同创建在 CLI 内部如何执行

即使命令表面只有一个：

```bash
contract-cli contract create --input contract-create.json
```

CLI 内部仍然应按 `mode` 分流。

### 7.1 文件模式

内部执行步骤：

1. 读取并校验 `input`
2. 上传合同文件，获取 `fileId`
3. 根据 `categoryCode` 转换出创建合同所需分类参数
4. 合并交易方、金额、日期等公共字段
5. 调用创建合同接口

### 7.2 模板模式

内部执行步骤：

1. 读取并校验 `input`
2. 根据 `templateNumber` 创建模板实例
3. 校验并补齐 `form`
4. 合并模板外公共字段
5. 调用创建合同接口

这意味着：

- 用户看到的是单命令
- 工程实现仍然是两套清晰 handler

## 8. MVP 方案的优点

- 命令数量少，学习成本低
- 文档更短，更容易讲清楚
- 适合智能体辅助生成输入
- 复杂性收敛在统一输入模型里
- 首发实现范围明确，更适合快速落地
- 后续可以平滑演进到更细分命令

## 9. MVP 方案的代价

- 命令本身表达力较弱
- 更多语义转移到了 `input schema`
- help 文案对纯手动用户帮助有限
- 如果没有 `--dry-run` 和 `--print-input`，排障体验会偏弱

因此建议 MVP 至少补两个调试能力：

```bash
--dry-run
--print-input
```

## 10. MVP 到完整版的演进路径

建议按三个阶段演进。

### 10.1 第一阶段：极简命令

```bash
contract-cli auth login
contract-cli auth status
contract-cli contract create --input contract-create.json
contract-cli contract get <id>
contract-cli vendor create --input vendor.json
contract-cli vendor get <id>
```

### 10.2 第二阶段：补调试和查询能力

```bash
contract-cli contract search
contract-cli vendor fields
contract-cli contract create --dry-run
contract-cli contract create --print-input
```

### 10.3 第三阶段：长成细分命令

如果后续发现：

- 文件模式和模板模式都足够高频
- 用户经常手动敲命令而不是依赖智能体
- `input` 结构对一线用户仍然太重

再考虑长成：

```bash
contract-cli contract create from-file
contract-cli contract create from-template
```

这会比一开始就设计得很重更稳妥。

## 11. 适合会议讨论的结论

如果当前更重视“尽快上线”和“减少首发学习成本”，建议优先采用本 MVP 方案：

```bash
contract-cli contract create --input contract-create.json
contract-cli vendor create --input vendor.json
```

核心理由：

- 用户只需要记住极少量命令
- 智能体可以承担大部分参数拼装工作
- CLI 仍然保有结构化输入、校验和可观测性
- 后续还有明确的演进路径，不会把未来锁死

## 12. 当前推荐

我的当前建议是：

1. 会议先不要争论过细的命令树
2. 先统一 `ContractCreateInput` 和 `VendorCreateInput`
3. 首发只上线极少数高频命令
4. 通过智能体生成 `input`
5. CLI 内部保留按 `mode` 分流的实现结构

这会比一开始把命令做得很细更容易落地。
