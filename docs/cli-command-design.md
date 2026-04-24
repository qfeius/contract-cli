# contract-cli 命令设计文档

> 维护说明：本文是阶段性设计记录。`api call` 目前仅作为代码中的预留能力保留，当前不对外开放；公开命令请以 [cli-command-reference.md](/Users/lyy/contract-cli/docs/cli-command-reference.md) 为准。

## 1. 背景

`contract-cli` 面向开放平台业务能力封装 CLI 命令，但鉴权不直接暴露开放平台原始 `token` 获取流程，而是复用当前本地空间已有的授权体系：

- `contract-cli config add`
- `contract-cli auth login`
- `contract-cli auth status`
- `contract-cli auth logout`

因此，业务命令只表达“我要做什么”，不要求用户理解开放平台底层接入细节。

## 1.1 当前实现状态

截至 2026-04-14，仓库内已经实现的开放平台结构化命令主要覆盖 `mcp.yaml` 中的 `contract/v1/mcp` 用户态能力：

- `contract get/search/create/sync-user-groups/text`
- `contract category list`
- `contract template list/get/instantiate`
- `contract upload-file`
- `contract enum list`
- `mdm vendor list/get`
- `mdm legal list/get`
- `mdm fields list`

当前实现约定：

- `contract/v1/mcp` 这批结构化命令只支持 `--as user`
- 这批命令不暴露 `--operator`
- 请求体文件输入统一使用 `--input-file`
- `--file` 仅用于真实二进制文件上传，不再表示 JSON 请求体
- 文件上传当前已支持 bot 身份下的 `contract upload-file`

## 2. 设计目标

- 采用“资源 + 动作”命令风格，降低学习成本
- 高价值业务动作优先，避免首版命令树过重
- 复杂请求体统一走 `--input-file` 或 `--data`
- 位置参数优先承载主键 ID，避免过度 flag 化
- 平台底层参数尽量映射为业务语义
- 保留 `api call` 作为长尾接口兜底能力

## 3. 核心约定

### 3.1 命名规则

- 一级命令代表业务域，如 `contract`、`mdm`、`event`
- `mdm` 域下再细分资源，如 `vendor`、`legal`、`fields`
- 二级命令代表动作，如 `get`、`create`、`search`
- 若存在强从属关系，则放在资源下一级，如 `contract template get`

### 3.2 输入规则

- `get` 类命令的主键采用位置参数
- 简单查询条件使用 flags
- 复杂 JSON 请求体使用 `--input-file`
- 仅在需要传内联 JSON 时使用 `--data`

示例：

```bash
contract-cli contract get 7023646046559404327
contract-cli contract create --input-file contract.json
contract-cli api call POST /open-apis/contract/v1/mcp/contracts/search --input-file contract-search.json
```

### 3.3 输出规则

建议所有命令统一支持：

- `--output json|yaml|table`
- `--raw`
- `--verbose`

约定：

- 默认输出面向人阅读
- `--output json` 输出稳定结构，便于脚本消费
- `--raw` 输出原始 API envelope，便于调试

### 3.4 用户标识规则

开放平台部分接口要求 `user_id` 或 `user_id_type`。

CLI 层建议：

- 对 `contract/v1/mcp` 这批已实现命令，不对外暴露 `--operator`
- 当前登录的 `--as user` 身份就是实际操作者
- 默认内部固定 `user_id_type=user_id`
- 首版不对外暴露 `open_id`、`lark_open_id`

这样可以减少用户理解成本，也避免把平台差异直接泄漏到命令层。

## 4. 全局参数建议

所有业务命令建议统一支持以下全局参数：

```bash
--profile string
--output string
--raw
--verbose
--timeout duration
--yes
```

复杂入参命令统一支持以下输入参数：

```bash
--input-file string
--data string
```

参数语义：

- `--profile`：指定认证上下文
- `--output`：指定输出格式
- `--raw`：输出原始响应
- `--verbose`：输出调试信息
- `--timeout`：设置请求超时
- `--yes`：跳过确认
- `--input-file`：从文件读取 JSON 请求体
- `--data`：直接传递 JSON 字符串

说明：

- 当前 `--file` 不再用于请求体输入，仅用于二进制文件上传命令

## 5. 一级命令树

以下列表包含“已实现 + 规划中”的总命令树；已实现能力以 1.1 为准。

```bash
contract-cli config add
contract-cli auth login
contract-cli auth status
contract-cli auth logout

contract-cli contract get
contract-cli contract search
contract-cli contract create
contract-cli contract update
contract-cli contract submit
contract-cli contract resubmit
contract-cli contract delete-draft

contract-cli contract template list
contract-cli contract template get
contract-cli contract template fields
contract-cli contract template instantiate

contract-cli contract upload-file
contract-cli contract file download
contract-cli contract file print

contract-cli contract approval start
contract-cli contract approval get
contract-cli contract authorization grant
contract-cli contract share list
contract-cli contract esign personal-url
contract-cli contract esign org-url

contract-cli payment create
contract-cli payment get
contract-cli payment list
contract-cli payment plan search
contract-cli payment record create
contract-cli payment record update

contract-cli mdm fields list --biz-line vendor
contract-cli mdm vendor get
contract-cli mdm vendor list
contract-cli mdm vendor create
contract-cli mdm vendor update

contract-cli mdm legal get
contract-cli mdm legal list
contract-cli mdm legal create
contract-cli mdm legal update

contract-cli event list
contract-cli event serve
contract-cli event verify
contract-cli event decrypt
contract-cli event ip-list

contract-cli rule table list
contract-cli rule table headers
contract-cli rule table rows
contract-cli rule table query
contract-cli rule table create-row
contract-cli rule table update-row
contract-cli rule table delete-row
contract-cli rule table publish

contract-cli api call
```

## 6. 按领域展开的命令设计

### 6.1 基础接入命令

#### 添加 profile

```bash
contract-cli config add --env dev --name contract-group
```

#### 登录授权

```bash
contract-cli auth login --profile contract-group
```

#### 查看授权状态

```bash
contract-cli auth status --profile contract-group
```

#### 登出

```bash
contract-cli auth logout --profile contract-group
```

### 6.2 合同命令

#### 创建合同

命令示例：

```bash
contract-cli contract create --input-file contract.json
```

也支持内联 JSON：

```bash
contract-cli contract create --template TMP001 --data '{"title":"示例合同"}'
```

建议的 `contract.json`：

```json
{
  "title": "示例采购合同",
  "ourParty": {
    "vendorId": "7003410079584092448"
  },
  "counterParty": {
    "vendorId": "7023646046559404327"
  },
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

设计说明：

- 当前已实现版本直接透传创建合同请求体，不再额外暴露 `--template`
- 合同 form 字段动态性强，请求体默认走 `--input-file`
- 首版只提供一个统一的 `create`，内部自行处理模板实例和创建合同的编排

#### 获取合同详情

```bash
contract-cli contract get 7023646046559404327
```

#### 搜索合同

```bash
contract-cli contract search --name "采购合同" --status approved
```

复杂搜索建议：

```bash
contract-cli contract search --input-file contract-search.json
```

#### 更新合同

```bash
contract-cli contract update 7023646046559404327 --input-file contract-update.json
```

#### 提交合同

```bash
contract-cli contract submit 7023646046559404327
```

#### 重新提交合同

```bash
contract-cli contract resubmit 7023646046559404327
```

#### 删除草稿合同

```bash
contract-cli contract delete-draft 7023646046559404327
```

### 6.3 合同模板命令

#### 查看模板列表

```bash
contract-cli contract template list
```

#### 查看模板详情

```bash
contract-cli contract template get TMP001
```

#### 查看模板字段

```bash
contract-cli contract template fields TMP001
```

#### 创建模板实例

```bash
contract-cli contract template instantiate --input-file template-instance.json
```

设计说明：

- `template fields` 很重要，用于提前感知动态字段
- 如果 `contract create` 内部已经自动实例化模板，`instantiate` 仍然保留给高级用户或调试场景

### 6.4 合同文件命令

#### 上传合同相关文件

```bash
contract-cli contract upload-file --profile contract-group --as bot --file ./附件.pdf --file-type attachment
```

#### 下载合同相关文件

```bash
contract-cli contract file download --file-id 609a128628ad4eaebd3063c59928a103 --out ./downloaded.pdf
```

#### 生成打印文件

```bash
contract-cli contract file print 7023646046559404327
```

### 6.5 审批与权限命令

#### 发起审批

```bash
contract-cli contract approval start 7023646046559404327 --input-file approval.json
```

#### 查询审批实例

```bash
contract-cli contract approval get 7499999999999999999
```

#### 授予合同权限

```bash
contract-cli contract authorization grant 7023646046559404327 --input-file grant.json
```

#### 查询合同分享记录

```bash
contract-cli contract share list --contract 7023646046559404327
```

#### 获取电子签链接

```bash
contract-cli contract esign personal-url --contract 7023646046559404327
contract-cli contract esign org-url --contract 7023646046559404327
```

### 6.6 付款命令

#### 创建付款申请

```bash
contract-cli payment create --input-file payment.json
```

#### 查询付款详情

```bash
contract-cli payment get 7033333333333333333
```

#### 查询付款列表

```bash
contract-cli payment list --contract 7023646046559404327
```

#### 搜索付款计划

```bash
contract-cli payment plan search --contract 7023646046559404327
```

#### 创建付款记录

```bash
contract-cli payment record create --input-file payment-record.json
```

#### 更新付款记录

```bash
contract-cli payment record update 7044444444444444444 --input-file payment-record-update.json
```

### 6.7 交易方命令

交易方是首批应重点支持的主数据能力。

#### 查询交易方字段配置

```bash
contract-cli mdm fields list --biz-line vendor
```

设计说明：

- 该命令对应字段配置查询接口
- 用于创建前拉取后台配置的必填字段和字段定义
- 有助于在本地做预校验，减少请求失败

#### 获取单个交易方

```bash
contract-cli mdm vendor get 1063197165850985296
```

带调试输出：

```bash
contract-cli mdm vendor get 1063197165850985296 --output json --raw
```

设计说明：

- 交易方 ID 采用位置参数，不暴露底层 `vendor_id` header 形式
- 默认输出归一化后的交易方对象

#### 创建交易方

推荐主路径：

```bash
contract-cli mdm vendor create --operator 123123123123 --input-file vendor.json
```

`vendor.json` 示例：

```json
{
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

如果需要轻量录入，可提供少量高频 flag：

```bash
contract-cli mdm vendor create \
  --operator 123123123123 \
  --code V00108006 \
  --name "张三样例" \
  --short-name "王五" \
  --cert-type 0 \
  --cert-id 913100xxxxx555781R \
  --category 11 \
  --nature 0 \
  --country CN \
  --province MDPS00000001 \
  --city MDCY00001226 \
  --address "上海市浦东新区世纪大道1000号"
```

设计说明：

- 对外使用 `--operator`，内部映射到底层 `user_id`
- 首版不暴露 `user_id_type`，内部固定为 `user_id`
- 不建议把所有嵌套字段全部做成 flags
- `vendorAccounts`、`vendorAddresses`、`vendorCompanyViews`、`extendInfo` 等复杂结构建议一律放入 `--input-file`

#### 更新交易方

```bash
contract-cli mdm vendor update 1063197165850985296 --operator 123123123123 --input-file vendor-update.json
```

#### 查询交易方列表

```bash
contract-cli mdm vendor list --name "张三样例"
```

### 6.8 法人实体命令

#### 获取法人实体

```bash
contract-cli mdm legal get 7023646046559404327
```

#### 查询法人实体列表

```bash
contract-cli mdm legal list --name "上海主体"
```

#### 创建法人实体

```bash
contract-cli mdm legal create --operator 123123123123 --input-file entity.json
```

#### 更新法人实体

```bash
contract-cli mdm legal update 7023646046559404327 --operator 123123123123 --input-file entity-update.json
```

设计说明：

- `mdm legal` 的参数风格应与 `mdm vendor` 保持一致
- 若底层接口也要求用户标识，继续沿用 `--operator`

### 6.9 事件命令

#### 启动本地事件回调服务

```bash
contract-cli event serve --listen :8080 --verification-token xxx --encrypt-key yyy
```

设计说明：

- 用于本地联调 challenge、验签、解密和事件解析
- 推荐作为首批高价值调试命令

#### 验证事件签名

```bash
contract-cli event verify --headers-file headers.json --body-file event.json --verification-token xxx
```

#### 解密事件内容

```bash
contract-cli event decrypt --encrypt-key yyy --data-file encrypted-event.json
```

#### 查询事件列表

```bash
contract-cli event list
```

#### 获取事件出口 IP

```bash
contract-cli event ip-list
```

### 6.10 规则表命令

规则表能力首版可不优先实现，但建议预留命令空间。

```bash
contract-cli rule table list
contract-cli rule table headers TABLE001
contract-cli rule table rows TABLE001
contract-cli rule table query TABLE001 --input-file query.json
contract-cli rule table create-row TABLE001 --input-file row.json
contract-cli rule table update-row TABLE001 ROW001 --input-file row-update.json
contract-cli rule table delete-row TABLE001 ROW001
contract-cli rule table publish TABLE001
```

### 6.11 原始接口兜底命令

首版必须保留：

```bash
contract-cli api call GET /open-apis/mdm/v1/vendors/1063197165850985296
contract-cli api call POST /open-apis/mdm/v1/vendors --input-file vendor.json
```

设计说明：

- 为低频接口和新接口提供兜底能力
- 避免因封装不完整阻塞接入

## 7. 首发范围建议

### 7.1 v1

- `config`
- `auth`
- `contract`
- `contract template`
- `contract file`
- `mdm vendor`
- `mdm legal`
- `mdm fields`
- `event`
- `api call`

### 7.2 v1.1

- `payment`
- `contract approval`
- `contract authorization`
- `contract esign`

### 7.3 v2

- `rule table`
- 其他低频平台型能力

## 8. 设计上的几项明确取舍

### 8.1 不直接暴露开放平台 token 命令

原因：

- 当前 CLI 已有独立授权体系
- 用户无需理解底层 token 生命周期
- 能降低命令树噪音

### 8.2 主路径优先 `--input-file`

原因：

- 合同和主数据接口都存在大量动态字段和嵌套结构
- 如果把所有字段都平铺为 flags，命令会非常脆弱
- `--file` 只用于真实文件上传命令，避免和 JSON 请求体输入混淆

### 8.3 主数据命令统一收口到 `mdm`

原因：

- 与当前接口领域词汇一致
- 更便于从 API 文档映射到 CLI

如果后续更强调业务友好性，可以补充 `party` 作为别名。

### 8.4 `contract create` 保持编排式命令

约定：

- `contract-cli contract create --input-file contract.json`
- 命令面向业务动作，而非暴露多步底层流程

## 9. 帮助文案风格建议

示例：

```bash
contract-cli mdm vendor create --help
```

建议输出：

```text
Create a vendor using the current authorized profile.

Usage:
  contract-cli mdm vendor create --operator <user-id> --input-file <path>

Examples:
  contract-cli mdm vendor create --operator 123123123123 --input-file vendor.json
  contract-cli mdm vendor create --operator 123123123123 --data '{"vendor":"V001"}'
```

## 10. 实现时的测试建议

虽然本文档当前只定义命令，不涉及实现代码，但后续开发建议至少覆盖以下测试：

- 命令解析测试：验证子命令、位置参数、flag 默认值和错误提示
- 请求映射测试：验证 `--operator` 到底层 `user_id` 的映射
- 文件输入测试：验证 `--input-file` 与 `--data` 的互斥或优先级规则
- 输出测试：验证默认输出、`--output json` 与 `--raw`
- 交易方预校验测试：验证 `mdm fields list --biz-line vendor` 拉取字段后本地校验的行为
- 工作流测试：验证“模板查询 -> 合同创建 -> 合同提交”等典型链路
- 事件测试：验证 challenge、验签、解密和回调解析

## 11. 当前推荐的用户使用路径

```bash
contract-cli config add --env dev --name contract-group
contract-cli auth login --profile contract-group
contract-cli contract template fields TMP001
contract-cli contract create --input-file contract.json
contract-cli mdm fields list --biz-line vendor
contract-cli mdm vendor create --operator 123123123123 --input-file vendor.json
contract-cli mdm vendor get 1063197165850985296
```

以上流程兼顾了：

- 授权前置
- 动态字段先探测
- 复杂结构走文件
- 查询与创建命令风格统一
