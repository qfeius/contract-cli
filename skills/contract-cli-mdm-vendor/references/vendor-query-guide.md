# Vendor Query Guide

这份文档是 `contract-cli mdm vendor ...` 的主入口，优先解决两个问题：

- 我现在是要找候选交易方，还是已经拿到了交易方 id
- 这个命令面到底会把哪些 CLI 参数映射到哪个开放平台查询参数

推荐阅读顺序：

1. 先看本文件，选查询场景
2. 再看 [vendor-query-parameters.md](vendor-query-parameters.md) 查精确参数映射
3. 最后看 [commands.md](commands.md) 直接抄命令示例

## 1. 命令面与硬约束

当前结构化命令只有两类：

```bash
contract-cli mdm vendor list --profile contract-group --name "供应商A"
contract-cli mdm vendor get 1063197165850985296 --profile contract-group
```

硬约束：

- 不暴露 `--operator`
- 默认输出就是开放平台原始 envelope；如果要脚本消费，建议加 `--output json`
- 如果要排障或确认原始响应，建议加 `--raw`
- `mdm vendor list` 同时支持 `user` 和 `bot`
- `mdm vendor get` 也同时支持 `user` 和 `bot`
- `--user-id-type` / `--user-id` 继续按共享约定透传，不做本地校验

## 2. 场景配方

### 2.1 按名称找候选交易方

适用场景：

- 创建合同前只知道供应商名称
- 需要拿候选列表，再从结果里挑 id

最小命令：

```bash
contract-cli mdm vendor list --profile contract-group --name "供应商A"
```

常见追加参数：

- `--page-size 20`
- `--page-token <next-token>`
- `--as bot --user-id-type employee_id`

补充说明：

- user 路由走 `/open-apis/contract/v1/mcp/vendors`
- bot 路由走 `/open-apis/mdm/v1/vendors`
- 生产文档里 bot 侧把 query `vendor` 描述成“供应商编码”，CLI 仍保持 `--name -> vendor` 的透传映射

### 2.2 分页扫交易方列表

适用场景：

- 不按名字过滤，直接分页遍历
- 或者已经有上一页返回的 `page_token`

最小命令：

```bash
contract-cli mdm vendor list --profile contract-group --page-size 20
```

翻页示例：

```bash
contract-cli mdm vendor list --profile contract-group --page-size 20 --page-token next
```

bot 示例：

```bash
contract-cli mdm vendor list --profile contract-group --as bot --name "V00000001" --page-size 20 --user-id-type employee_id
```

### 2.3 已知 id 直接查详情

适用场景：

- 已经从搜索结果、外部系统或历史合同里拿到了交易方 id

最小命令：

```bash
contract-cli mdm vendor get 1063197165850985296 --profile contract-group
```

bot 示例：

```bash
contract-cli mdm vendor get 7003410079584092448 --profile contract-group --as bot --user-id-type employee_id
```

补充说明：

- user 路由走 `/open-apis/contract/v1/mcp/vendors/{vendor_id}`
- bot 路由走 `/open-apis/mdm/v1/vendors/{vendor_id}`
- 生产文档里 bot 详情接口只显式列出了 `user_id_type` 查询参数，没看到 `user_id`
- CLI 仍按共享约定统一透传 `--user-id-type` / `--user-id`，不做本地校验

## 3. 什么时候不要走这里

- 想创建或更新交易方：当前结构化命令未实现，明确说明暂未覆盖；不要退回 `api call`
- 想先确认交易方字段定义：改看 [../../contract-cli-mdm-fields/SKILL.md](../../contract-cli-mdm-fields/SKILL.md)
- 想查合同主体选择逻辑：回到 [../../contract-cli-contract/SKILL.md](../../contract-cli-contract/SKILL.md)
