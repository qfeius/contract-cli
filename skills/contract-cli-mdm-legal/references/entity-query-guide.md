# Entity Query Guide

这份文档是 `contract-cli mdm legal ...` 的主入口，优先解决两个问题：

- 我现在是要找候选法人实体，还是已经拿到了法人实体 id
- 这个命令面的 `--name` 到底如何映射到底层查询参数

推荐阅读顺序：

1. 先看本文件，选查询场景
2. 再看 [entity-query-parameters.md](entity-query-parameters.md) 查精确参数映射
3. 最后看 [commands.md](commands.md) 直接抄命令示例

## 1. 命令面与硬约束

当前结构化命令只有两类：

```bash
contract-cli mdm legal list --profile contract-group --name "上海主体"
contract-cli mdm legal get 7023646046559404327 --profile contract-group
```

硬约束：

- 只支持 `--as user`
- 不暴露 `--operator`
- 内部固定 `user_id_type=user_id`
- 默认输出就是开放平台原始 envelope；如果要脚本消费，建议加 `--output json`
- 如果要排障或确认原始响应，建议加 `--raw`

## 2. 场景配方

### 2.1 按名称找我方法人主体

适用场景：

- 创建合同前只知道主体名称
- 需要拿候选列表，再从结果里挑法人实体 id

最小命令：

```bash
contract-cli mdm legal list --profile contract-group --name "上海主体"
```

常见追加参数：

- `--page-size 20`
- `--page-token <next-token>`

### 2.2 分页扫法人实体列表

适用场景：

- 不按名字过滤，直接分页遍历
- 或者已经有上一页返回的 `page_token`

最小命令：

```bash
contract-cli mdm legal list --profile contract-group --page-size 20
```

### 2.3 已知 id 直接查详情

适用场景：

- 已经从搜索结果、外部系统或历史合同里拿到了法人实体 id

最小命令：

```bash
contract-cli mdm legal get 7023646046559404327 --profile contract-group
```

## 3. 什么时候不要走这里

- 想创建或更新法人实体：当前结构化命令未实现，改走 [../../contract-cli-api-call/SKILL.md](../../contract-cli-api-call/SKILL.md)
- 想先确认法人实体字段定义：改看 [../../contract-cli-mdm-fields/SKILL.md](../../contract-cli-mdm-fields/SKILL.md)
- 想查合同我方主体选择逻辑：回到 [../../contract-cli-contract/SKILL.md](../../contract-cli-contract/SKILL.md)
