# Open Web API Call Guide

本目录只保留通用传输层的维护参考，不是可安装、可隐式触发的独立 Skill。

调用链固定为：

业务 Skill → Agent 编排 → `contract-cli api call --channel web` → `/open-web/web/**` → 页面业务接口。

业务 Skill 必须先给出已验证的 method、path、query/body、响应映射和权限要求。CLI 只复用自身授权并执行 HTTP，不接受外部 Token，也不负责发现或猜测接口。
