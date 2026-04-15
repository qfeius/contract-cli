# MDM Fields Guide

这份文档是 `contract-cli mdm fields list` 的主入口，优先解决两个问题：

- 我现在到底应该查 `vendor`、`legal_entity` 还是 `vendor_risk`
- 这条命令只是查“字段定义”，并不会帮我做写入校验或字段转换

推荐阅读顺序：

1. 先看本文件，确定要查哪条业务线
2. 再看 [schema-biz-lines.md](schema-biz-lines.md) 确认精确参数和值
3. 最后看 [commands.md](commands.md) 直接抄命令示例

## 1. 命令面与硬约束

当前结构化命令只有一条：

```bash
contract-cli mdm fields list --profile contract-group --biz-line vendor
```

硬约束：

- 只支持 `--as user`
- `--biz-line` 必填
- 内部固定 `user_id_type=user_id`
- 这条命令只查字段配置，不负责本地校验、字段清洗或自动组装写请求

## 2. 场景配方

### 2.1 查交易方字段定义

适用场景：

- 想调用交易方相关写接口前，先确认可写字段和字段结构

最小命令：

```bash
contract-cli mdm fields list --profile contract-group --biz-line vendor
```

### 2.2 查法人实体字段定义

适用场景：

- 想调用法人实体相关写接口前，先确认可写字段和字段结构

最小命令：

```bash
contract-cli mdm fields list --profile contract-group --biz-line legal_entity
```

### 2.3 查交易方风险字段定义

适用场景：

- 需要查看交易方风险业务线的字段配置

最小命令：

```bash
contract-cli mdm fields list --profile contract-group --biz-line vendor_risk
```

## 3. 什么时候不要走这里

- 想查合同创建相关枚举：改走 [../../contract-cli-contract/SKILL.md](../../contract-cli-contract/SKILL.md)
- 想直接调未封装写接口：改走 [../../contract-cli-api-call/SKILL.md](../../contract-cli-api-call/SKILL.md)
- 想查具体交易方/法人实体数据：改走对应 `mdm vendor` / `mdm legal` skill
