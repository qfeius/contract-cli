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

- `--biz-line` 必填
- 这条命令只查字段配置，不负责本地校验、字段清洗或自动组装写请求
- `mdm fields list` 同时支持 `user` 和 `bot`
- bot 后端当前只接受 `vendor` / `legalEntity`；CLI 会把 bot 下的 `legal_entity` 自动映射为 `legalEntity`
- `vendor_risk` 仅适用于 user/MCP 路径，bot 身份下会本地报错
- `--user-id-type` / `--user-id` 继续按共享约定透传，不做本地校验

## 2. 场景配方

### 2.1 查交易方字段定义

适用场景：

- 想调用交易方相关写接口前，先确认可写字段和字段结构

最小命令：

```bash
contract-cli mdm fields list --profile contract-group --biz-line vendor
```

bot 示例：

```bash
contract-cli mdm fields list --profile contract-group --as bot --biz-line vendor --user-id-type employee_id
```

### 2.2 查法人实体字段定义

适用场景：

- 想调用法人实体相关写接口前，先确认可写字段和字段结构

最小命令：

```bash
contract-cli mdm fields list --profile contract-group --biz-line legal_entity
```

bot 示例：

```bash
contract-cli mdm fields list --profile contract-group --as bot --biz-line legal_entity
```

### 2.3 查交易方风险字段定义

适用场景：

- 需要查看交易方风险业务线的字段配置

最小命令：

```bash
contract-cli mdm fields list --profile contract-group --as user --biz-line vendor_risk
```

注意：`vendor_risk` 当前不支持 bot 身份。

补充说明：

- user 路由走 `/open-apis/contract/v1/mcp/config/config_list`
- bot 路由走 `/open-apis/mdm/v1/config/config_list`
- 文档显示文本使用这条 bot 路径，但超链接目标误指到了 `vendors`
- bot 下 `legal_entity` 会映射为后端实际取值 `legalEntity`

## 3. 什么时候不要走这里

- 想查合同创建相关枚举：改走 [../../contract-cli-contract/SKILL.md](../../contract-cli-contract/SKILL.md)
- 想直接调未封装写接口：当前暂未开放；不要退回 `api call`
- 想查具体交易方/法人实体数据：改走对应 `mdm vendor` / `mdm legal` skill
