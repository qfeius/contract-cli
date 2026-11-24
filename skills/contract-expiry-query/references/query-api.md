# 查询协议

## 入口与身份

只读业务操作采用 POST `/open-web/web/clm/es/search/contractList`，CLM family。请求无 query，无需额外业务 header。下游按当前组织 USER 设置 tenantId、employeeId 并填入当前用户数据权限；不使用 `/admin/contractList`、内部接口或租户级提醒任务数据。合同组及其记录的可见性沿用平台搜索权限，不扩大到管理员或租户视角。

CLI 执行前检查 `contract-cli api call --help` 是否实际支持 `--channel web`、`--profile`、`--as user`、`--input-file`、`--raw`。将 `PROFILE` 设为已核实的生产配置名；`query.json` 须按下表生成完整真实值：

```sh
contract-cli api call --channel web POST /open-web/web/clm/es/search/contractList --profile "$PROFILE" --as user --input-file query.json --raw
```

`--input-file` 发送 UTF-8 JSON 原文，不能与 `--data` 同用；不把完整 URL 或 query 拼进 path。CLI/MCP 管理认证，不传 Token、Cookie、可信身份或转发 header。身份状态及续期沿用宿主已有授权能力；不要猜登录命令。

MCP 仅在实际已连接且 schema 兼容时使用 `call-open-web-api` 或宿主等价工具；先读取真实 schema，映射本节的 method、相对 path 和 JSON body。不能把 CLI flags 当 MCP 参数。不因命令缺失擅自安装软件，不因业务拒绝切换身份或通道。

通道缺失、目标环境未确认、授权无效时，停止真实请求并说明具体缺项。源码核实不证明目标环境已部署接口和权限实现；需在目标环境已有可信发布依据或技术确认的适用范围内调用。

## 请求 JSON

以下各字段均位于 body；时间值为 JSON 整数毫秒，不是 ISO 字符串。

| 字段 | 本流程传值和依据 |
|---|---|
| `pageIndex` | 首次 0；游标续页保持 0。不用 1 作为首页。 |
| `pageSize` | 固定 50，续页不变。若服务拒绝该大小，明确错误，由已核实能力确定合法大小后重启本轮，不能跳页。 |
| `searchTabCode` | 0，全部合同下当前 USER 可见范围。 |
| `sortType` / `order` | `SUBMITTED_TIME` / `DESC`，续页保持一致；服务端追加 ID 排序。 |
| `conditionUnits` | 空数组；本次不增加关键词搜索。 |
| `filterUnits` | 一个对象组成的数组：`searchField`=`CONTRACT_END_DATE_IN`；`searchValue`=`[起始毫秒,结束毫秒]`，恰好两个整数且起始不晚于结束；`unionType`=`MUST`。不传 filterUniqueKey 或 searchLabel。 |
| `withStatistics` | false。避免无关标签统计；分页不依赖 totalElements。 |
| `enableHighlight` | false。无需高亮合同正文。 |
| `searchAfter` | 首页省略；续页携带前页 `data.searchAfter` 完整数值/字符串数组，不得自行构造、转浮点或截断。 |

不发送 `tenantId`、`employeeId`、`searchIndex`、`userAuthority`、`userAuthorityVO`、`userAuthorityAdvancedVO`；组织、身份、索引和权限来自当前上下文。其余字段全部省略，不发送 null 替代省略。不开启 scroll，不传 scrollId 或 scrollAliveTime，不依赖它们续页。

必须用 `CONTRACT_END_DATE_IN`。它按到期字段执行 gte/lte 闭区间；不要换成近似的 `CONTRACT_END_DATE`，不同搜索实现的含义不一致，组搜索未注册字段可能被忽略。范围查询不匹配没有到期日期的记录。响应组内可能带回不满足时间条件的其他合同，所以仍须逐条复核 `endDate`。

## 成功判据和字段

日期边界示例：业务时区 UTC，执行时当地日期 2026-11-19，只查今天，`searchValue` 为 `[1795017600000, 1795103999999]`。平台 GMT+8 的当日首毫秒和末毫秒都命中；前一日末毫秒和后一日首毫秒都不命中。逐条按平台返回的到期日期复核，不再换时区；完整遍历后若只有相邻日记录且无待核实项，则报告该范围无匹配。

传输成功与业务成功都要满足。下游业务 envelope 为 `code`、`data` 等，`code` 数字 200 才是成功；MCP/CLI 外层成功不代替它。保留错误信息供说明，但脱敏。返回无 data、data.content 为 null/缺失或类型异常不是可信空列表，即使 code=200 也停止并说明服务结果不完整。

| 响应路径 | 含义和处理 |
|---|---|
| `data.content` | 合同组数组；明确 `[]` 才是空页。 |
| `data.content[].groupId` | 合同组 ID，以字符串无损保留。不是合同 ID。 |
| `data.content[].searchContractListItems` | 组内合同数组。逐条取记录，不只取第一条，不把整组当一份合同。缺失/类型异常要记为数据缺口。 |
| `...searchContractListItems[].contractId` | 字符串合同 ID，用于去重；缺失则该记录待核实，不能按名称去重。 |
| `...contractName` / `...contractNumber` | 显示名称/编号；为空显示“未提供”。 |
| `...endDate` | `yyyy-MM-dd` 日期或 null。按业务日期比较范围，不把日期字符串当 UTC 时点。null 不满足到期查询；异常格式记待核实。 |
| `...archivedTime` | 归档完成时间；当前协议为时间字符串或 JSON null。按主文档判定，仅组合查询依赖它。缺失与 null 不同。 |
| `...contractStatusCode` / `...contractStatusName` | 展示状态；code 9 表示已归档。不能只排除 code 9 来筛未归档。未知 code 保留原值。 |
| `data.searchAfter` | 下一页请求 body.searchAfter 的来源；数组元素可能含长整型。 |
| `data.filteredTotalElements` | 候选合同组数，仅作进度参考；不是最终合同数，不作为唯一结束依据。 |
| `data.paginationCapability` | 可选，可能声明 OFFSET_WINDOW 和 CONTRACT_GROUP；本流程优先游标，不凭它推断查询已完整。 |

用 `--raw` 保存响应，使用宿主可用的无损 JSON 能力处理 ID/游标；若工具只能把长整数转为不精确浮点，则说明保真缺口并停止续页，不发失真游标。不得新增或交付 Python 运行脚本。只处理任务所需字段，不将原始合同响应上传到其他服务。

## 续页、计数与异常

1. 固定本轮时区、日期、身份、环境、筛选、排序和 pageSize；完成当前页逐条判断后再续页。
2. 当前响应含非空 `data.searchAfter` 数组时，以其原值续页、pageIndex=0，哪怕过滤后零命中、content 为空或 content 少于 50 也继续。展示列表的长度不是 ES 原始命中长度。
3. 返回明确 `data.content=[]` 且 searchAfter 缺失/null/空数组，才能认为游标遍历结束；content 非空但没有游标时，不能证明遍历完整，保留部分结果并报告分页协议异常。空/缺失数据对象也不能充当结束标记。
4. 游标与上一页相同或形成循环、响应格式异常、任一页失败、用户停止时，中止遍历；保留已确认结果与最后成功进度，明确“部分结果”。权限/授权失败沿用原通道恢复规则，不覆盖身份或绕过限流。
5. 按每份合同的字符串 contractId 去重。字段冲突的重复记录列为待核实，不挑有利值凑命中；数据查询期间可能变化，不承诺快照一致性。没有稳定完整遍历，不给确定总数。
6. 对满足日期但归档未知的记录单列，不能混入确定未归档数，也不能悄悄遗漏后声称完整。计数基于唯一合同记录，候选组数单独标注。

搜索无权可能按平台策略返回空数据；只表述“当前可见范围未找到”，不据空结果断言租户没有合同或用户必然有权限。
