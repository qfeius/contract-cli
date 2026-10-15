# 验收场景

1. 输入唯一合同编号：只发送一次 query 请求，使用 user Token，JSON 为 `contractNumber`，完整展示成功摘要。
2. 输入完整合同名称：只发送一次 query 请求，JSON 为 `contractName`；同名不唯一时要求改用编号。
3. 同时输入编号和名称：在本地报错，不发 HTTP。
4. 摘要不存在但允许生成：query 由服务端触发生成；Agent 不再调用文件 generate。
5. query 返回 `status=1`：只说明“摘要正在生成中，请稍后再试”，即使响应携带旧摘要字段也不展示；不自动轮询或重发。超时、断网或 5xx 时说明结果不确定，不自动重试。
6. 上传单个 doc/docx/pdf：generate 只发送一个 multipart `file` 字段。
7. 上传空文件、不支持后缀或超过 200MB：在本地拒绝，不发 HTTP。
8. 使用 app 身份：在本地拒绝，并引导 user OAuth。
9. 用户要求非生产环境：说明当前 CLI 不支持该环境，不改用 prod 执行业务请求。
10. 用户要求发送协商群：先交付摘要，再说明当前 CLI 没有群卡片发送能力，不伪造发送结果。
