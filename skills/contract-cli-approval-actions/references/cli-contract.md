# CLI 能力契约

本 Skill 只使用 `contract-cli` 结构化命令。禁止使用 `api call`、用户给出的 URL、随包脚本、curl 或直接 HTTP 代替缺失能力。

## 能力发现

首次查询前读取以下真实帮助，不凭文档假设命令存在：

```bash
contract-cli contract approval task list --help
contract-cli contract approval get --help
contract-cli contract category list --help
contract-cli employee list --help
```

执行动作前还必须分别确认：

```bash
contract-cli contract approval task endorse --help
contract-cli contract approval task transfer --help
contract-cli contract approval task notice --help
```

当前已核验的 CLI 版本具备任务列表、流程详情、合同分类和人员查询，但不保证具备上述三个写命令。任一写命令缺失、参数或结果契约不满足本页要求时，该动作不可执行；说明 CLI 能力缺失并停止，不退回页面接口。

## 身份和上下文

- 任务查询、人员查询、预览、执行和回查固定使用同一个 `--profile` 和 `--as user`。
- 不接受 App/Bot 代操作，不传调用人字段，不复制其他会话或环境的凭据。
- 默认生产 profile 为 `contract`；只有用户明确测试或验收时使用 `contract-test`。
- CLI 非零退出、业务失败、非法 JSON 或关键字段缺失都不是空结果或成功。

## 任务完整分页

待办首请求：

```bash
contract-cli contract approval task list \
  --profile <profile> --as user --task-type todo \
  --page-index 0 --page-size 100 --output json
```

已办和知会分别使用 `done`、`notice`。用户查询本人全部任务时，不传 `business_type_codes`，避免遗漏续签、补充协议、终止、协商、合同组、表单修改、批量申请和模板实例等业务类型。

完整分页必须同时满足：

1. 从 `page_index=0` 开始；只有响应明确 `has_more=true` 才请求下一页。
2. 下一页使用返回页码加一，其他筛选、profile、身份和页大小保持不变。
3. 页码不前进、同一页任务集合重复、关键分页字段缺失或请求失败时停止，说明结果不完整；不得把当前累计结果称为全部。
4. 按 `task_instance_id` 去重；同一任务字段矛盾时停止，不用新值静默覆盖旧值。
5. 有 `total` 时，累计唯一任务数应与总数一致；不一致时标记不完整。
6. 只展示首批不等于只查询首批。需要回答“全部”“今天所有”“某类型全部”时必须先完成底层全量分页，再按输出契约折叠展示。

`business_type_codes` 表示流程业务类型，不是合同一二级分类。只有用户明确要求某种流程业务类型时才使用；合同分类按[目标定位](target-resolution.md)处理。

## 任务详情和当前处理人

对进入展示或动作候选的任务，使用：

```bash
contract-cli contract approval get <process-instance-id> \
  --task-instance-filter <task-instance-id> \
  --profile <profile> --as user --output json
```

必须在同一流程中精确找到目标任务。详情部分成功、任务状态缺失、标识不一致或目标任务不存在时，不写入。

“当前处理人”只取目标节点尚未完成记录中的人员：

- 当前用户显示为“你”；
- 不使用创建人、负责人或其他并行节点人员补位；
- 一至三人显示姓名，超过三人显示前两人和总人数；
- 能可靠取得处理策略时可补充“任一人处理”或“需全部处理”，无法取得时不猜。

## 合同分类

```bash
contract-cli contract category list --profile <profile> --as user --output json
```

按返回的分类 ID、编号、名称、父级和层级建立分类树。二级分类只匹配自身；一级分类在用户说“这个一级类型下全部”时展开全部启用后代。分类名称不唯一时用编号、父级和层级消歧。

## 人员查询

```bash
contract-cli employee list \
  --name <姓名或邮箱关键词> --page-size 200 \
  --profile <profile> --as user --output json
```

只有 `has_more=true` 时才使用返回的 `page_token` 继续，并保持原查询条件。保留所有候选的 `user_id`、姓名、部门、邮箱、工号和状态：

- 姓名搜索不保证唯一；不得默认第一条。
- 优先按部门，再按邮箱、工号消歧；仍相同则停止。
- `status=1` 可作为当前可选人员；其他状态按接口原义说明，不把“未找到”推断为离职。
- 用户重复说出同一人时，Skill 按首次出现去重并告知；CLI 仍须拒绝重复 ID。

## 写命令的最低能力要求

CLI 团队补齐的三个结构化命令必须使用以下业务语义；具体帮助是运行时最终依据：

| 命令 | 必要输入 | 必须内置的保护 |
| --- | --- | --- |
| `task endorse` | 任务、人员、五种模式、原因、条件性编辑权限 | 当前 User 待办、节点与模式权限、人员可选性、顺序、写后加签记录核验 |
| `task transfer` | 任务、唯一接收人、可选说明 | 当前 User 责任、接收人不是本人、写后转办记录及新责任人核验 |
| `task notice` | 任务、接收人列表、可选备注、条件性重复授权 | 抄送权限、本人排除、历史重复检查、写后新增抄送记录核验 |

这些命令必须默认只生成预览，返回绑定任务、节点、接收人顺序、模式和备注的短期 `plan_token`；实际写入必须同时使用 `--execute` 和同一次预览的 `--confirm-plan-token`。计划变化、令牌过期或上下文变化必须拒绝执行。令牌仅供内部调用，不向用户展示。

三个写命令必须返回动作特定回查结果：

| 状态 | 含义 | Skill 行为 |
| --- | --- | --- |
| `verified_applied` | 已由业务记录证明目标效果 | 报告完成 |
| `verified_not_applied` | 已证明写入未生效 | 报告未完成；只有用户重新授权新计划才可重试 |
| `unknown` | 无法证明是否生效 | 报告待核验并停止后续写入，禁止重试 |

传输异常必须区分“确定未发送”和“可能已发送”。可能已发送时先只读回查，不得自动重放。转办只有核对目标接收人才可返回 `verified_applied`；本人待办消失不充分。

## 明确排除

本 Skill 不调用 `contract approval task approve|reject`，也不执行盖章或归档完成。遇到这些动作按主文件的能力路由处理，由其他 Skill 独立承担其校验和授权。
