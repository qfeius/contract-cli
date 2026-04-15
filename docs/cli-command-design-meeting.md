# contract-cli CLI 命令设计会议版

## 1. 文档目的

本文档用于内部会议讨论 `contract-cli` 的 CLI 命令设计。

使用方式：

- 以 `docs/cli-command-design.md` 作为主方案
- 以 `docs/contract-create-command-options.md` 作为“合同创建命令封装方式”的备选讨论材料
- 以 `docs/cli-command-design-mvp.md` 作为“极简首发方案”的收敛参考

本文件不替代原文档，而是用于会议快速对齐背景、主推荐方案、备选分支和决策点。

## 2. 会议建议阅读顺序

建议按以下顺序阅读：

1. 主文档：`docs/cli-command-design.md`
2. 合同创建分支方案：`docs/contract-create-command-options.md`
3. 极简 MVP 收敛方案：`docs/cli-command-design-mvp.md`

对应角色建议：

- 产品、业务、平台：先看主文档和会议结论
- 架构、后端、CLI 实现同学：重点看合同创建分支方案
- 首发推进、交付、实施：重点看 MVP 文档

## 3. 当前推荐基线

当前推荐把 `docs/cli-command-design.md` 作为主方案基线。

主文档的核心观点是：

- CLI 采用“资源 + 动作”风格
- 基础接入沿用本地空间已有授权体系
- 交易方、合同、事件、规则表等都预留命令空间
- 平台复杂对象优先走结构化输入
- 保留 `api call` 作为长尾能力兜底

主文档适合回答以下问题：

- 这套 CLI 最终希望长成什么样
- 命令层级和业务对象怎么分
- 哪些能力应当纳入长期命令树

## 4. 主文档中的重点内容

会议建议优先确认主文档里的四个主问题。

### 4.1 命令命名风格

是否接受以下整体风格：

```bash
contract-cli contract get
contract-cli contract create
contract-cli mdm vendor get
contract-cli mdm vendor create
contract-cli event serve
```

目标：

- 语义清晰
- 后续扩展统一
- 和平台对象模型一致

### 4.2 统一输入策略

是否接受复杂对象统一走文件输入，例如：

```bash
contract-cli contract create --input-file contract.json
contract-cli mdm vendor create --operator 123123123123 --input-file vendor.json
```

目标：

- 避免 flags 爆炸
- 保留自动化和脚本稳定性
- 便于智能体协助生成参数

### 4.3 一级命令树是否过重

主文档目前包含较完整的长期命令树，覆盖：

- `contract`
- `contract template`
- `contract file`
- `payment`
- `mdm vendor`
- `mdm legal`
- `mdm fields`
- `event`
- `rule table`
- `api call`

会议要判断的是：

- 这是“最终形态”还是“首发形态”
- 哪些命令现在就实现
- 哪些只保留在设计层

### 4.4 交易方命令设计是否认可

主文档里对交易方已经形成了相对清晰的设计：

```bash
contract-cli mdm vendor get 1063197165850985296
contract-cli mdm vendor create --operator 123123123123 --input-file vendor.json
contract-cli mdm fields list --biz-line vendor
```

这部分可作为首批落地命令的样板。

## 5. 合同创建的分支讨论

合同创建是目前最需要单独决策的部分，因此不建议只看主文档。

建议把 `docs/contract-create-command-options.md` 作为会议中的分支讨论材料。

它主要解决的问题是：

- 合同创建到底是一个命令还是拆成多个命令
- `mode` 是否显式
- `spec` 由谁提供
- 智能体应该负责到什么程度

### 5.1 分支 A：单命令 + `mode`

示例：

```bash
contract-cli contract create --mode file --spec contract-create.file.json
contract-cli contract create --mode template --spec contract-create.template.json
```

适合：

- 希望对外保持单一入口
- 希望智能体协同时交互更自然

风险：

- help 文案复杂
- 模式校验和排障成本更高

### 5.2 分支 B：拆命令

示例：

```bash
contract-cli contract create from-file
contract-cli contract create from-template
```

适合：

- 工程稳定性优先
- 一线用户高频手动操作

风险：

- 命令树更重
- 用户需要理解更多入口

### 5.3 会议建议

对合同创建，不建议在会议里只讨论“命令长什么样”，而应同时讨论：

- 用户输入来源
- 智能体参与程度
- shell 手工调用和自动化调用的比例
- 调试和排障能力是否必须首发就具备

## 6. MVP 收敛分支

如果团队判断主文档过重，建议引入 `docs/cli-command-design-mvp.md` 作为收敛分支。

MVP 文档的核心观点是：

- 首发先减少命令数量
- 统一采用 `--input`
- 复杂模式差异先沉到输入模型中
- 智能体负责生成输入
- CLI 内部再按模式分流

### 6.1 MVP 命令示例

```bash
contract-cli auth login
contract-cli auth status

contract-cli contract create --input contract-create.json
contract-cli contract get <contract-id>

contract-cli mdm vendor create --input vendor.json
contract-cli mdm vendor get <vendor-id>
```

### 6.2 MVP 适合的场景

- 首发时间紧
- 希望优先打通高频路径
- 用户会大量依赖智能体，而不是手写复杂命令
- 团队不希望首发阶段就把命令树做满

### 6.3 MVP 的代价

- 命令本身表达力较弱
- 更多语义转移到了输入模型
- 对纯手工使用者不够直观

## 7. 三份文档的角色分工

为了避免会议中多份文档互相打架，建议明确分工：

### 7.1 `cli-command-design.md`

定位：

- 主方案
- 长期命令树
- 默认设计基线

适合回答：

- CLI 长期长什么样
- 命令命名怎么统一
- 各业务域怎么组织

### 7.2 `contract-create-command-options.md`

定位：

- 合同创建专题
- 分支方案对比
- 适合单独决策

适合回答：

- 合同创建应该单命令还是拆命令
- `mode` 和 `spec` 要不要显式

### 7.3 `cli-command-design-mvp.md`

定位：

- 首发收敛方案
- 范围控制参考

适合回答：

- 第一版最少做什么
- 哪些复杂命令先不做

## 8. 会议建议决策顺序

建议会议不要同时讨论所有命令，而是按以下顺序做决策：

1. 是否认可以 `cli-command-design.md` 作为长期主方案
2. 首发是否采用 MVP 收敛思路
3. 合同创建采用哪一种封装方式
4. 首发是否保留 `mdm fields list --biz-line vendor`
5. 首发是否需要 `api call` 兜底

这样可以避免会议陷入细枝末节。

## 9. 建议的会议结论模板

会议可以直接从下面三种结论里选一种。

### 9.1 结论 A：按主方案推进

- 长期设计按 `cli-command-design.md`
- 首发实现 `contract`、`mdm`、`event`、`api call` 等主要能力
- 合同创建后续再补充分支设计

适合：

- 资源充足
- 希望一次性把架构边界定清楚

### 9.2 结论 B：主方案不变，首发走 MVP

- 长期设计仍以 `cli-command-design.md` 为主
- 首发实现按 `cli-command-design-mvp.md` 收敛
- 合同创建暂时统一走单命令输入模型

适合：

- 希望既不丢长期方向，又能尽快上线

### 9.3 结论 C：主方案 + 合同创建专项决策

- 主体设计以 `cli-command-design.md` 为主
- 合同创建专题单独采用 `contract-create-command-options.md` 中的一种封装
- 其他业务域先不展开

适合：

- 目前真正卡点主要是合同创建

## 10. 我的建议

如果目标是“这次会议先定方向，不追求定完全部细节”，我建议：

1. 以 `cli-command-design.md` 作为主文档
2. 以 `cli-command-design-mvp.md` 作为首发收敛参考
3. 以 `contract-create-command-options.md` 作为合同创建专项讨论材料
4. 会议结论优先选“主方案不变，首发走 MVP”

对应的实际含义是：

- 长期方向不丢
- 首发复杂度可控
- 合同创建保留进一步讨论空间

## 11. 会议后建议产出

会议结束后，建议形成一份简短结论，至少明确：

- 长期主方案是否确认
- 首发范围最终包含哪些命令
- 合同创建采用哪种封装
- 智能体在参数拼装中承担到什么程度
- 哪些命令进入下一轮实现
