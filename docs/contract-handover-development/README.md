# 合同交接旧版研发与验收资料

这里保留重整前的 API 契约、源码追溯、历史验收记录和 Ruby 离线检查器，用于维护追溯，不作为 Agent 日常执行说明，也不随 Skill 分发。

当前运行入口为 skills/contract-handover/SKILL.md，必要接口说明已按场景拆分到其 references。旧契约的成熟度、时间和包外 evidence_ref 均是历史记录，不代表当前环境已通过验证；引用的包外原始证据未随本仓提交。

可在此目录运行 `ruby tests/validate_contract.rb --self-test` 检查旧验证器自身；`ruby tests/validate_contract.rb` 仍按旧契约报告未闭合项，不是新文档的运行前置条件。原文件内容保留，便于核对请求参数和业务规则没有在精简中被改写。
