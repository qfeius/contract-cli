# API Call Guide

这份文档是 `contract-cli api call` 的主入口，优先解决两个问题：

- 什么时候该用结构化命令，什么时候该退回 `api call`
- 调开放平台原始接口时，方法、路径、请求体、身份该怎么组合

推荐阅读顺序：

1. 先看本文件，判断是否真的需要 `api call`
2. 再看 [api-call-rules.md](api-call-rules.md) 确认路径、身份、请求体规则
3. 最后看 [commands.md](commands.md) 直接抄命令示例

## 1. 什么时候该用 `api call`

优先用结构化命令的场景：

- 合同搜索、详情、创建、模板、分类、枚举：优先看 [../../contract-cli-contract/SKILL.md](../../contract-cli-contract/SKILL.md)
- 交易方查询：优先看 [../../contract-cli-mdm-vendor/SKILL.md](../../contract-cli-mdm-vendor/SKILL.md)
- 法人实体查询：优先看 [../../contract-cli-mdm-legal/SKILL.md](../../contract-cli-mdm-legal/SKILL.md)
- 字段配置查询：优先看 [../../contract-cli-mdm-fields/SKILL.md](../../contract-cli-mdm-fields/SKILL.md)

适合退回 `api call` 的场景：

- 结构化命令还没覆盖
- 用户已经给出了精确的开放平台路径
- 需要原样透传请求体或自定义 header

## 2. 常见配方

### 2.1 调普通开放平台 GET 接口

```bash
contract-cli api call GET /open-apis/mdm/v1/vendors/1063197165850985296 --profile contract-group
```

### 2.2 调普通开放平台 POST 接口并传 JSON 文件

```bash
contract-cli api call POST /open-apis/mdm/v1/vendors --profile contract-group --input-file vendor.json
```

### 2.3 调 user-only `contract/v1/mcp` 接口

```bash
contract-cli api call GET /open-apis/contract/v1/mcp/vendors/1063197165850985296 --profile contract-group
```

这类路径如果不传 `--as`，CLI 会默认按 `user` 身份解析。

## 3. 什么时候不要走这里

- 已有结构化命令覆盖时，不要退回 `api call`
- 想传绝对 URL：当前命令不支持
- 想把本地文件路径直接当上传文件传给开放平台：当前 `api call` 只处理 JSON/body，不处理真实文件上传
