# 审批矩阵 user / app 双身份接入

## 已实现

- CLI 的规则组、矩阵、列、行、导入及发布命令支持 `--as user` / `--as app`，不传使用 profile 默认身份。其他模块身份策略不变。
- 用户登录复用现有 OAuth：旧 Authorization Code profile 使用 `auth login --as user`；Device profile 使用 `auth init`、浏览器授权、`auth complete`。不新增登录协议，不在会话中粘贴凭证。
- user 请求仍走 `/open-apis/rule_engine/v1/...`，CLI 携带用户 Bearer 和 `X-Qfei-Identity: user`。该标记仅选择验证分支，不授予权限。
- 矩阵编辑人的 ID 不由 query 参数提供：规则服务端从 Bearer 验签结果写入当前用户和审计字段。CLI 在规则路径移除 `--user-id`，所以审批人 ID 只出现在规则行/列的业务值中，不会改变编辑人。
- 后端 OpenApiInterceptor 的 user 分支仅支持 `product_id=contract`，调用页面已有的 OrgV2 验签方法，但不调用包含 tempAuth 的页面拦截入口。租户/用户来自验签结果，并始终要求合同规则管理员权限。无权限或空权限返回 403；凭证缺失/无效返回 401；权限或员工查询依赖异常返回 503。user 验证失败不降级 app。
- 审计记录使用真实员工；用户完成和鉴权失败都会清理线程上下文。app 无标记或显式 app 继续使用原 X-Gate-Auth、36 进制 TenantID 和合同租户映射、系统审计逻辑。页面规则组审计行为不变。
- user 导入计划额外绑定凭证的 SHA-256 摘要，不存原始 Token。切换身份、重新登录或 Token 轮换后旧计划失效，需要重新生成和确认。app 计划保持原 v2 指纹，正常 app Token 轮换不影响计划。

## 网关部署契约（尚待确认）

本地 CLI / bpm-rule-configuration 修改不等于网关已支持 user。部署前需在开平网关确认：

1. 为规则接口配置 user / app 双 Token 类型及对应接口权限，不要只放开路由而跳过鉴权。
2. user 路径必须保留原始 `Authorization`，并保留或由网关从已验证 Token 类型重建 `X-Qfei-Identity: user`，到规则后端执行二次验签、业务权限检查。
3. `X-Gate-Auth` 只能由可信网关注入，剥离外部同名头；走 app 兼容分支的请求必须已验证为合法 app Token。不得把 user Token 当 app 放行，也不得因缺失 user 标记把用户请求降级为 app。
4. 不对外直连仅信任 X-Gate-Auth 的 app 后端入口；保留原有网络隔离。透传 401/403/503，不伪装为成功。
5. 若网关剥离 Authorization、自定义头或仅接受 tenant_access_token，需要同步修改网关配置/代码。本次尚未取得网关项目路径，因此未修改或验证该层。

## test 联调步骤

先部署后端及上述网关配置，再构建安装包含本次修改的 test CLI。旧 dev 包不包含本次环境替换，需安装 test 联调包；不复用 dev 凭证。

```bash
export CONTRACT_CLI_CONFIG_DIR="$HOME/.contract-cli-test"
# 适用于已有 Authorization Code 类型 contract-test profile
contract-cli auth login --profile contract-test --as user
contract-cli rule table list --profile contract-test --as user --product-id contract --group-id approve_matrix
# 使用原 app 凭证回归同一查询
contract-cli rule table list --profile contract-test --as app --product-id contract --group-id approve_matrix
```

以上查询不会创建组或矩阵。管理员 user、无权限 user、过期 Token、app、跨租户资源，以及 user 标记被剥离/冒充 app 的负向用例都需要网关联调验证。新增/修改/删除和发布必须先确认具体目标与变更，不通过写业务数据测试登录。
