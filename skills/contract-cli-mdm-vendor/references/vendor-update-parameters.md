# mdm vendor update Parameters

本页专用于 `contract-cli mdm vendor update`。

- 接口：`PUT /open-apis/mdm/v1/vendors/{vendor_id}`
- 身份：仅 `app`
- 请求体：JSON 必填，`--input-file` 与 `--data` 二选一
- 官方 OpenAPI：[更新交易方](https://docs.qfei.cn/373499262e0.md)
- 校验基线：`contract-cli` 当前分支；CLM `master@334c18fc2e` 存在对应实现时，以 Controller、DTO 和业务校验补充官方规格。
- 自定义字段类型和值属性统一按 [交易方字段配置解释规则](../../contract-cli-mdm-fields/references/vendor-field-config-semantics.md) 解释；本页保留旧 PUT 的字段清单和全量更新行为。

## 目录

- [CLI 参数映射](#cli-参数映射)
- [请求体字段](#请求体字段)
- [枚举与约束](#枚举与约束)
- [示例](#示例)
- [来源差异说明](#来源差异说明)

## CLI 参数映射

| 参数 | 请求位置 | 类型 | 必填性 | 说明 |
| --- | --- | --- | --- | --- |
| --user-id | $query.user_id | string | 必填（CLI 本地校验） | 用户id，如果使用的是 tenant_access_token，那么需要填入user_id<br>示例值："123123123123" |
| --user-id-type | $query.user_id_type | string | 可选，默认 `user_id` | 用户 ID 类型，参考 用户身份体系 |
| <vendor-id> | $path.vendor_id | string | 必填 | 交易方 ID；CLI 将其拼入请求路径。 |
| --input-file | $body | JSON file | 二选一必填 | 从文件读取 JSON；与 `--data` 互斥。 |
| --data | $body | JSON string | 二选一必填 | 内联 JSON；与 `--input-file` 互斥。 |
| --profile | 本地上下文 | string | 可选 | 不传时使用当前 profile。 |
| --as | 本地上下文 | enum | 可选 | 仅支持 `app`；不传时使用 profile 默认身份。 |
| --output | CLI 输出 | enum | 可选 | `json`、`yaml` 或 `table`；默认 `json`。 |
| --raw | CLI 输出 | boolean | 可选 | 原样输出服务端响应 body。 |

## 请求体字段

字段名、类型和服务端必填性来自官方 OpenAPI；“CLI 必填/禁止”是结构化命令的额外本地校验。父对象可选时，其内部必填字段标记为“父对象存在时必填”。

| JSON 路径 | 类型 | 必填性 | 说明 |
| --- | --- | --- | --- |
| id | string | CLI 必填 | 交易方id，创建交易方时不填，修改交易方时必填<br>示例值："7023646046559404327" |
| adCountry | string | 可选 | 交易方注册国家<br>示例值："CN"<br>数据校验规则：<br>最大长度：50 字符 |
| adProvince | string | 可选 | 交易方注册省份<br>示例值："MDPS00000001"<br>数据校验规则：<br>最大长度：50 字符 |
| adCity | string | 可选 | 交易方注册城市<br>示例值："MDCY00001226"<br>数据校验规则：<br>- 最大长度：50 字符 |
| address | string | 可选 | 详细地址<br>示例值："上海市浦东新区世纪大道1000号"<br>数据校验规则：<br>- 最大长度：300 字符 |
| adPostcode | string | 可选 | 交易方注册地址邮编<br>示例值："100100"<br>数据校验规则：<br>- 最大长度：50 字符 |
| legalPerson | string | 可选 | 法人名称<br>示例值："张三"<br>数据校验规则：<br>- 最大长度：50 字符 |
| certificationType | string | 可选 | 证件类型<br>示例值："0"<br>可选值有：<br>- 0：统一社会信用代码(中国大陆)<br>- 1：中国大陆居民身份证(中国大陆)<br>- 2：注册号(海外)<br>- 3：税号(海外)<br>- 4：驾驶证(海外)<br>- 5：身份证(海外)<br>- 6：护照<br>- 8：港澳居民往来大陆通行证<br>- 9：台湾居民往来大陆通行征<br>- 10：香港永久性居民身份证<br>- 11：澳门特别行政区永久性居民身份证<br>- 12：台湾身份证<br>- 13：外国人永久居留证 |
| certificationId | string | 可选 | 证件ID<br>示例值："913100xxxxx555781R"<br>数据校验规则：<br>最大长度：300 字符 |
| contactPerson | string | 可选 | 联系人<br>示例值："李四"<br>数据校验规则：<br>最大长度：100 字符 |
| isRisked | boolean | 可选 | 是否标记风险<br>示例值：false |
| contactTelephone | string | 可选 | 联系电话<br>示例值："021-87853200"<br>数据校验规则：<br>最大长度：50 字符 |
| contactMobilePhone | string | 可选 | 联系移动电话<br>示例值："+8617621685955"<br>数据校验规则：<br>最大长度：50 字符 |
| fax | string | 可选 | 传真<br>示例值："021-87853200"<br>数据校验规则：<br>最大长度：50 字符 |
| email | string | 可选 | 邮箱<br>示例值："shunxing@xxx.com"<br>数据校验规则：<br>最大长度：100 字符 |
| status | integer | 必填 | 状态<br>示例值：1<br>可选值有：<br>- 1：有效（目前仅支持创建【有效】交易方，不支持创建【无效】交易方）<br>- 0：无效（目前仅支持创建【有效】交易方，不支持创建【无效】交易方） |
| vendor | string | CLI 必填 | 交易方编码<br>示例值："V00108006"<br>数据校验规则：<br>最大长度：64 字符 |
| vendorText | string | 可选 | 交易方名称<br>示例值："张三样例"<br>数据校验规则：<br>最大长度：240 字符 |
| shortText | string | 可选 | 交易方简称<br>示例值："王五"<br>数据校验规则：<br>最大长度：240 字符 |
| vendorType | string | 可选 | 交易方类型（多个枚举时，采用逗号分隔）<br>示例值："1"<br>可选值有：<br>- 1：客户<br>- 2：供应商<br>数据校验规则：<br>- 最大长度：3 字符 |
| vendorCategory | string | 可选 | 交易方类别<br>示例值："11"<br>可选值有：<br>- 11：内部客户(当vendor_category = 1 时的枚举值)<br>- 12：外部客户(当vendor_category = 1 时的枚举值)<br>- 21：内部供应商(当vendor_category = 2 时的枚举值)<br>- 22：外部供应商(当vendor_category = 2 时的枚举值)<br>- 23：员工供应商(当vendor_category = 2 时的枚举值) |
| vendorNature | string | 可选 | 交易方性质<br>示例值："0"<br>可选值有：<br>- 0：企业<br>- 1：自然人<br>- 2：非营利性组织 |
| linkedEmployee | string | 可选 | 关联员工<br>示例值："6959513973725069601"<br>数据校验规则：<br>最大长度：20 字符 |
| linkedCustomer | string | 可选 | 关联客户<br>示例值："客户"<br>数据校验规则：<br>- 最大长度：50 字符 |
| associatedWithLegalEntity | boolean | 可选 | 是否关联法人主体<br>示例值：true |
| appendix | array<object> | 可选 | 附件列表<br>数据校验规则：<br>最大长度：10 |
| appendix[].tenantId | string | 可选 | — |
| appendix[].fileId | string | 可选 | 文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4" |
| appendix[].fileName | string | 可选 | 文件名称<br>示例值："附件" |
| appendix[].fileType | string | 可选 | 文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR |
| appendix[].fileSize | integer | 可选 | 文件大小<br>示例值：1024 |
| appendix[].downloadUrl | string | 可选 | 文件下载地址<br>示例值："http://download.com/xxxxx" |
| extendInfo | array<object> | 可选 | 扩展字段相关信息列表,每个扩展字段需要填入【field_code】、【field_type】、【field_value】三个信息，其中【field_code】和【field_type】需要与用户【字段配置】(获取配置字段的开放平台接口：https://open.qfei.cn/open-apis/mdm/v1/config/config_list)中扩展字段（sys = 1）相关联（目前不支持附件类型的扩展信息）<br>数据校验规则：<br>- 最大长度：100 |
| extendInfo[].fieldType | integer | 父对象存在时必填 | 字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。 |
| extendInfo[].fieldValue | string | 可选 | 字段类型为 单行文本框(0)、多行文本框(1)、单选框(3)、下拉单选框(5) 时的值<br>示例值："文本值" |
| extendInfo[].options | array<string> | 可选 | 字段类型为 多选框(4) 下拉多选(6) 时的值<br>示例值：["字段名称"]<br>数据校验规则：最大长度：100 |
| extendInfo[].num | number | 可选 | 字段类型为 数字(2) 时的值<br>示例值：1.11 |
| extendInfo[].date | string | 可选 | 字段类型是 日期(7)时候的值<br>示例值："2021-10-14" |
| extendInfo[].rangeDate | array<string> | 可选 | 字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：["字段编码"]<br>数据校验规则：长度范围：2 ～ 2 |
| extendInfo[].fieldCode | string | 父对象存在时必填 | 字段编码<br>示例值："X00000001" |
| extendInfo[].appendix | array<object> | 可选 | 附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：<br>最大长度：10 |
| extendInfo[].appendix[].tenantId | string | 可选 | — |
| extendInfo[].appendix[].fileId | string | 可选 | 文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4" |
| extendInfo[].appendix[].fileName | string | 可选 | 文件名称示例值："附件" |
| extendInfo[].appendix[].fileType | string | 可选 | 文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR |
| extendInfo[].appendix[].fileSize | integer | 可选 | 文件大小示例值：1024 |
| extendInfo[].appendix[].downloadUrl | string | 可选 | 文件下载地址<br>示例值："http://download.com/xxxxx" |
| vendorAccounts | array<object> | 可选 | 银行账户列表<br>数据校验规则：最大长度：100 |
| vendorAccounts[].id | string | 可选 | 交易方银行账户id，修改信息时当前的账户id均必填，新增的账户id不填<br>示例值："1453263653228318721" |
| vendorAccounts[].account | string | 可选 | 账号<br>示例值："62448345986564434"<br>数据校验规则：最大长度：50 字符 |
| vendorAccounts[].iban | string | 可选 | 国际银行账号<br>示例值："46677"<br>数据校验规则：最大长度：34 字符 |
| vendorAccounts[].accountName | string | 可选 | 账户名<br>示例值："上海xxx技术有限（上海）分公司"<br>数据校验规则：<br>最大长度：1000 字符 |
| vendorAccounts[].bankId | string | 可选 | 银行内部Id<br>示例值："MDBK00061195"<br>数据校验规则：<br>最大长度：100 字符 |
| vendorAccounts[].bankCode | string | 可选 | 银联号<br>示例值："308290003732"<br>数据校验规则：<br>最大长度：100 字符 |
| vendorAccounts[].swiftCode | string | 可选 | 银行Swift编码<br>示例值："BOFAUS3NINQ"<br>数据校验规则：最大长度：100 字符 |
| vendorAccounts[].vendorSiteCode | string | 可选 | 交易方siteCode<br>示例值："99999999"<br>数据校验规则：<br>最大长度：100 字符 |
| vendorAccounts[].bankName | string | 可选 | 银行名称<br>示例值："xx银行股份有限公司苏州支行"<br>数据校验规则：<br>最大长度：100 字符 |
| vendorAccounts[].bankAcronym | string | 可选 | 银行简码<br>示例值："ZJTLCB"<br>数据校验规则：最大长度：100 字符 |
| vendorAccounts[].country | string | 可选 | 国家<br>示例值："CN"<br>数据校验规则：<br>最大长度：100 字符 |
| vendorAccounts[].bankControlCode | string | 可选 | 银行控制码<br>示例值："99999999"<br>数据校验规则：<br>最大长度：10 字符 |
| vendorAccounts[].extendInfo | array<object> | 可选 | 扩展字段相关信息列表<br>数据校验规则：<br>最大长度：100 |
| vendorAccounts[].extendInfo[].fieldType | integer | 父对象存在时必填 | 字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。 |
| vendorAccounts[].extendInfo[].fieldValue | string | 可选 | 字段类型为 单行文本框(0)、多行文本框(1)、单选框(3)、下拉单选框(5) 时的值<br>示例值："文本值" |
| vendorAccounts[].extendInfo[].options | array<string> | 可选 | 字段类型为 多选框(4) 下拉多选(6) 时的值<br>示例值：[""""]<br>数据校验规则：<br>最大长度：100 |
| vendorAccounts[].extendInfo[].num | number | 可选 | 字段类型为 数字(2) 时的值<br>示例值：1.11 |
| vendorAccounts[].extendInfo[].date | string | 可选 | 字段类型是 日期(7)时候的值<br>示例值："2021-10-14" |
| vendorAccounts[].extendInfo[].rangeDate | array<string> | 可选 | 字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：[""""]<br>数据校验规则：长度范围：2 ～ 2 |
| vendorAccounts[].extendInfo[].fieldCode | string | 父对象存在时必填 | 字段编码示例值："X00000001" |
| vendorAccounts[].extendInfo[].appendix | array<object> | 可选 | 附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：最大长度：10 |
| vendorAccounts[].extendInfo[].appendix[].tenantId | string | 可选 | — |
| vendorAccounts[].extendInfo[].appendix[].fileId | string | 可选 | 文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4" |
| vendorAccounts[].extendInfo[].appendix[].fileName | string | 可选 | 文件名称示例值："附件" |
| vendorAccounts[].extendInfo[].appendix[].fileType | string | 可选 | 文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR |
| vendorAccounts[].extendInfo[].appendix[].fileSize | integer | 可选 | 文件大小<br>示例值：1024 |
| vendorAccounts[].extendInfo[].appendix[].downloadUrl | string | 可选 | 文件下载地址<br>示例值："http://download.com/xxxxx" |
| vendorAddresses | array<object> | 可选 | 地址列表<br>数据校验规则：最大长度：100 |
| vendorAddresses[].id | string | 可选 | 交易方地址id<br>示例值："1433488030078558209" |
| vendorAddresses[].country | string | 可选 | 国家<br>示例值："CN"数据校验规则：最大长度：64 字符 |
| vendorAddresses[].province | string | 可选 | 省份<br>示例值："MDPS00000001"<br>数据校验规则：<br>最大长度：64 字符 |
| vendorAddresses[].city | string | 可选 | 城市<br>示例值："MDCY00000001"<br>数据校验规则：<br>最大长度：64 字符 |
| vendorAddresses[].county | string | 可选 | 县<br>示例值："MDCA00002746"<br>数据校验规则：<br>最大长度：64 字符 |
| vendorAddresses[].address | string | 可选 | 详细地址<br>示例值："北京市海淀区苏州街"<br>数据校验规则：<br>最大长度：64 字符 |
| vendorAddresses[].extendInfo | array<object> | 可选 | 扩展字段相关信息列表<br>数据校验规则：<br>最大长度：100 |
| vendorAddresses[].extendInfo[].fieldType | integer | 父对象存在时必填 | 字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。 |
| vendorAddresses[].extendInfo[].fieldValue | string | 可选 | 字段类型为 单行文本框(0)、多行文本框(1)、单选框(3)、下拉单选框(5) 时的值<br>示例值："文本值" |
| vendorAddresses[].extendInfo[].options | array<string> | 可选 | 字段类型为 多选框(4) 下拉多选(6) 时的值<br>示例值：[""""]<br>数据校验规则：<br>最大长度：100 |
| vendorAddresses[].extendInfo[].num | number | 可选 | 字段类型为 数字(2) 时的值<br>示例值：1.11 |
| vendorAddresses[].extendInfo[].date | string | 可选 | 字段类型是 日期(7)时候的值<br>示例值："2021-10-14" |
| vendorAddresses[].extendInfo[].rangeDate | array<string> | 可选 | 字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：["字段名称"]<br>数据校验规则：<br>长度范围：2 ～ 2 |
| vendorAddresses[].extendInfo[].fieldCode | string | 父对象存在时必填 | 字段编码<br>示例值："X00000001" |
| vendorAddresses[].extendInfo[].appendix | array<object> | 可选 | 附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：<br>最大长度：10 |
| vendorAddresses[].extendInfo[].appendix[].tenantId | string | 可选 | — |
| vendorAddresses[].extendInfo[].appendix[].fileId | string | 可选 | 文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4" |
| vendorAddresses[].extendInfo[].appendix[].fileName | string | 可选 | 文件名称<br>示例值："附件" |
| vendorAddresses[].extendInfo[].appendix[].fileType | string | 可选 | 文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR |
| vendorAddresses[].extendInfo[].appendix[].fileSize | integer | 可选 | 文件大小<br>示例值：1024 |
| vendorAddresses[].extendInfo[].appendix[].downloadUrl | string | 可选 | 文件下载地址<br>示例值："http://download.com/xxxxx" |
| vendorCompanyViews | array<object> | 可选 | 公司视图列表<br>数据校验规则：<br>最大长度：100 |
| vendorCompanyViews[].id | string | 可选 | 交易方公司视图id<br>示例值："1453263653228318721" |
| vendorCompanyViews[].companyCode | string | 可选 | 公司代码<br>示例值："1001"<br>数据校验规则：<br>最大长度：50 字符 |
| vendorCompanyViews[].glAccount | string | 可选 | 总账科目<br>示例值："22020101"<br>数据校验规则：<br>最大长度：15 字符 |
| vendorCompanyViews[].vendorSiteCode | string | 可选 | 交易方sitecode<br>示例值："999999"<br>数据校验规则：最大长度：100 字符 |
| vendorCompanyViews[].paymentTerm | string | 可选 | 付款条件信息<br>示例值："PT09"<br>数据校验规则：最大长度：255 字符 |
| vendorCompanyViews[].downPaymentTerm | string | 可选 | 预付条件<br>示例值："PT08"<br>数据校验规则：最大长度：100 字符 |
| vendorCompanyViews[].extendInfo | array<object> | 可选 | 扩展字段相关信息列表<br>数据校验规则：最大长度：100 |
| vendorCompanyViews[].extendInfo[].fieldType | integer | 父对象存在时必填 | 字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。 |
| vendorCompanyViews[].extendInfo[].fieldValue | string | 可选 | 字段类型为 单行文本框(0)、多行文本框(1)、单选框(3)、下拉单选框(5) 时的值<br>示例值："文本值" |
| vendorCompanyViews[].extendInfo[].options | array<string> | 可选 | 字段类型为 多选框(4) 下拉多选(6) 时的值示例值：[""""]<br>数据校验规则：最大长度：100 |
| vendorCompanyViews[].extendInfo[].num | number | 可选 | 字段类型为 数字(2) 时的值<br>示例值：1.11 |
| vendorCompanyViews[].extendInfo[].date | string | 可选 | 字段类型是 日期(7)时候的值<br>示例值："2021-10-14" |
| vendorCompanyViews[].extendInfo[].rangeDate | array<string> | 可选 | 字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：["字段名称"]数据校验规则：长度范围：2 ～ 2 |
| vendorCompanyViews[].extendInfo[].fieldCode | string | 父对象存在时必填 | 字段编码<br>示例值："X00000001" |
| vendorCompanyViews[].extendInfo[].appendix | array<object> | 可选 | 附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：最大长度：10 |
| vendorCompanyViews[].extendInfo[].appendix[].tenantId | string | 可选 | — |
| vendorCompanyViews[].extendInfo[].appendix[].fileId | string | 可选 | 文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4" |
| vendorCompanyViews[].extendInfo[].appendix[].fileName | string | 可选 | 文件名称<br>示例值："附件" |
| vendorCompanyViews[].extendInfo[].appendix[].fileType | string | 可选 | 文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR |
| vendorCompanyViews[].extendInfo[].appendix[].fileSize | integer | 可选 | 文件大小<br>示例值：1024 |
| vendorCompanyViews[].extendInfo[].appendix[].downloadUrl | string | 可选 | 文件下载地址<br>示例值："http://download.com/xxxxx" |
| vendorContacts | array<object> | 可选 | 联系人列表<br>数据校验规则：最大长度：100 |
| vendorContacts[].id | string | 可选 | 交易方联系人id<br>示例值："1433488091906793474" |
| vendorContacts[].name | string | 可选 | 姓名<br>示例值："张三"<br>数据校验规则：<br>最大长度：50 字符 |
| vendorContacts[].position | string | 可选 | 职位<br>示例值："董事长"<br>数据校验规则：<br>最大长度：50 字符 |
| vendorContacts[].email | string | 可选 | 邮箱<br>示例值："haha@xxx.com"<br>数据校验规则：<br>最大长度：50 字符 |
| vendorContacts[].phone | string | 可选 | 手机号<br>示例值："13333323333"<br>数据校验规则：<br>最大长度：50 字符 |
| vendorContacts[].remark | string | 可选 | 备注<br>示例值："备注"<br>数据校验规则：<br>最大长度：200 字符 |
| vendorContacts[].extendInfo | array<object> | 可选 | 扩展字段相关信息列表数据<br>校验规则：<br>最大长度：100 |
| vendorContacts[].extendInfo[].fieldType | integer | 父对象存在时必填 | 字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。 |
| vendorContacts[].extendInfo[].fieldValue | string | 可选 | 字段类型为 单行文本框(0)、多行文本框(1)、单选框(3)、下拉单选框(5) 时的值<br>示例值："文本值" |
| vendorContacts[].extendInfo[].options | array<string> | 可选 | 字段类型为 多选框(4) 下拉多选(6) 时的值示例值：["字段名称"]<br>数据校验规则：<br>最大长度：100 |
| vendorContacts[].extendInfo[].num | number | 可选 | 字段类型为 数字(2) 时的值<br>示例值：1.11 |
| vendorContacts[].extendInfo[].date | string | 可选 | 字段类型是 日期(7)时候的值<br>示例值："2021-10-14" |
| vendorContacts[].extendInfo[].rangeDate | array<string> | 可选 | 字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：["字段名称"]数据校验规则：长度范围：2 ～ 2 |
| vendorContacts[].extendInfo[].fieldCode | string | 父对象存在时必填 | 字段编码示例值："X00000001" |
| vendorContacts[].extendInfo[].appendix | array<object> | 可选 | 附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：<br>最大长度：10 |
| vendorContacts[].extendInfo[].appendix[].tenantId | string | 可选 | — |
| vendorContacts[].extendInfo[].appendix[].fileId | string | 可选 | 文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4" |
| vendorContacts[].extendInfo[].appendix[].fileName | string | 可选 | 文件名称示例值："附件" |
| vendorContacts[].extendInfo[].appendix[].fileType | string | 可选 | 文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR |
| vendorContacts[].extendInfo[].appendix[].fileSize | integer | 可选 | 文件大小<br>示例值：1024 |
| vendorContacts[].extendInfo[].appendix[].downloadUrl | string | 可选 | 文件下载地址<br>示例值："http://download.com/xxxxx" |
| glAccount | string | 可选 | 总账科目<br>示例值："22020101"<br>数据校验规则：<br>最大长度：15 字符 |
| downPaymentTerm | string | 可选 | 预付条件<br>示例值："PT09"<br>数据校验规则：<br>最大长度：100 字符 |
| paymentTerm | string | 可选 | 付款条件<br>示例值："PT08"<br>数据校验规则：<br>最大长度：255 字符 |
| vendorSiteCode | string | 可选 | 交易方site code<br>示例值："999999"<br>数据校验规则：<br>最大长度：100 字符 |

## 枚举与约束

- 使用该接口根据 id 更新交易方的全部字段，字段是否必填是根据后台动态配置的，如想获取配置，可从主数据的配置开放文档里获取，参数采用驼峰式。
- 字段必填性受租户动态配置影响；调用前使用 `mdm fields list --biz-line vendor`。
- CLI 额外要求非空 `id` 和 `vendor`。
- `id`（string，CLI 必填）：交易方id，创建交易方时不填，修改交易方时必填<br>示例值："7023646046559404327"
- `adCountry`（string，可选）：交易方注册国家<br>示例值："CN"<br>数据校验规则：<br>最大长度：50 字符
- `adProvince`（string，可选）：交易方注册省份<br>示例值："MDPS00000001"<br>数据校验规则：<br>最大长度：50 字符
- `adCity`（string，可选）：交易方注册城市<br>示例值："MDCY00001226"<br>数据校验规则：<br>- 最大长度：50 字符
- `address`（string，可选）：详细地址<br>示例值："上海市浦东新区世纪大道1000号"<br>数据校验规则：<br>- 最大长度：300 字符
- `adPostcode`（string，可选）：交易方注册地址邮编<br>示例值："100100"<br>数据校验规则：<br>- 最大长度：50 字符
- `legalPerson`（string，可选）：法人名称<br>示例值："张三"<br>数据校验规则：<br>- 最大长度：50 字符
- `certificationType`（string，可选）：证件类型<br>示例值："0"<br>可选值有：<br>- 0：统一社会信用代码(中国大陆)<br>- 1：中国大陆居民身份证(中国大陆)<br>- 2：注册号(海外)<br>- 3：税号(海外)<br>- 4：驾驶证(海外)<br>- 5：身份证(海外)<br>- 6：护照<br>- 8：港澳居民往来大陆通行证<br>- 9：台湾居民往来大陆通行征<br>- 10：香港永久性居民身份证<br>- 11：澳门特别行政区永久性居民身份证<br>- 12：台湾身份证<br>- 13：外国人永久居留证
- `certificationId`（string，可选）：证件ID<br>示例值："913100xxxxx555781R"<br>数据校验规则：<br>最大长度：300 字符
- `contactPerson`（string，可选）：联系人<br>示例值："李四"<br>数据校验规则：<br>最大长度：100 字符
- `contactTelephone`（string，可选）：联系电话<br>示例值："021-87853200"<br>数据校验规则：<br>最大长度：50 字符
- `contactMobilePhone`（string，可选）：联系移动电话<br>示例值："+8617621685955"<br>数据校验规则：<br>最大长度：50 字符
- `fax`（string，可选）：传真<br>示例值："021-87853200"<br>数据校验规则：<br>最大长度：50 字符
- `email`（string，可选）：邮箱<br>示例值："shunxing@xxx.com"<br>数据校验规则：<br>最大长度：100 字符
- `status`（integer，必填）：状态<br>示例值：1<br>可选值有：<br>- 1：有效（目前仅支持创建【有效】交易方，不支持创建【无效】交易方）<br>- 0：无效（目前仅支持创建【有效】交易方，不支持创建【无效】交易方）
- `vendor`（string，CLI 必填）：交易方编码<br>示例值："V00108006"<br>数据校验规则：<br>最大长度：64 字符
- `vendorText`（string，可选）：交易方名称<br>示例值："张三样例"<br>数据校验规则：<br>最大长度：240 字符
- `shortText`（string，可选）：交易方简称<br>示例值："王五"<br>数据校验规则：<br>最大长度：240 字符
- `vendorType`（string，可选）：交易方类型（多个枚举时，采用逗号分隔）<br>示例值："1"<br>可选值有：<br>- 1：客户<br>- 2：供应商<br>数据校验规则：<br>- 最大长度：3 字符
- `vendorCategory`（string，可选）：交易方类别<br>示例值："11"<br>可选值有：<br>- 11：内部客户(当vendor_category = 1 时的枚举值)<br>- 12：外部客户(当vendor_category = 1 时的枚举值)<br>- 21：内部供应商(当vendor_category = 2 时的枚举值)<br>- 22：外部供应商(当vendor_category = 2 时的枚举值)<br>- 23：员工供应商(当vendor_category = 2 时的枚举值)
- `vendorNature`（string，可选）：交易方性质<br>示例值："0"<br>可选值有：<br>- 0：企业<br>- 1：自然人<br>- 2：非营利性组织
- `linkedEmployee`（string，可选）：关联员工<br>示例值："6959513973725069601"<br>数据校验规则：<br>最大长度：20 字符
- `linkedCustomer`（string，可选）：关联客户<br>示例值："客户"<br>数据校验规则：<br>- 最大长度：50 字符
- `appendix`（array<object>，可选）：附件列表<br>数据校验规则：<br>最大长度：10
- `appendix[].fileId`（string，可选）：文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4"
- `appendix[].fileType`（string，可选）：文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR
- `extendInfo`（array<object>，可选）：扩展字段相关信息列表,每个扩展字段需要填入【field_code】、【field_type】、【field_value】三个信息，其中【field_code】和【field_type】需要与用户【字段配置】(获取配置字段的开放平台接口：https://open.qfei.cn/open-apis/mdm/v1/config/config_list)中扩展字段（sys = 1）相关联（目前不支持附件类型的扩展信息）<br>数据校验规则：<br>- 最大长度：100
- `extendInfo[].fieldType`（integer，父对象存在时必填）：字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。
- `extendInfo[].options`（array<string>，可选）：字段类型为 多选框(4) 下拉多选(6) 时的值<br>示例值：["字段名称"]<br>数据校验规则：最大长度：100
- `extendInfo[].rangeDate`（array<string>，可选）：字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：["字段编码"]<br>数据校验规则：长度范围：2 ～ 2
- `extendInfo[].appendix`（array<object>，可选）：附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：<br>最大长度：10
- `extendInfo[].appendix[].fileId`（string，可选）：文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4"
- `extendInfo[].appendix[].fileType`（string，可选）：文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR
- `vendorAccounts`（array<object>，可选）：银行账户列表<br>数据校验规则：最大长度：100
- `vendorAccounts[].account`（string，可选）：账号<br>示例值："62448345986564434"<br>数据校验规则：最大长度：50 字符
- `vendorAccounts[].iban`（string，可选）：国际银行账号<br>示例值："46677"<br>数据校验规则：最大长度：34 字符
- `vendorAccounts[].accountName`（string，可选）：账户名<br>示例值："上海xxx技术有限（上海）分公司"<br>数据校验规则：<br>最大长度：1000 字符
- `vendorAccounts[].bankId`（string，可选）：银行内部Id<br>示例值："MDBK00061195"<br>数据校验规则：<br>最大长度：100 字符
- `vendorAccounts[].bankCode`（string，可选）：银联号<br>示例值："308290003732"<br>数据校验规则：<br>最大长度：100 字符
- `vendorAccounts[].swiftCode`（string，可选）：银行Swift编码<br>示例值："BOFAUS3NINQ"<br>数据校验规则：最大长度：100 字符
- `vendorAccounts[].vendorSiteCode`（string，可选）：交易方siteCode<br>示例值："99999999"<br>数据校验规则：<br>最大长度：100 字符
- `vendorAccounts[].bankName`（string，可选）：银行名称<br>示例值："xx银行股份有限公司苏州支行"<br>数据校验规则：<br>最大长度：100 字符
- `vendorAccounts[].bankAcronym`（string，可选）：银行简码<br>示例值："ZJTLCB"<br>数据校验规则：最大长度：100 字符
- `vendorAccounts[].country`（string，可选）：国家<br>示例值："CN"<br>数据校验规则：<br>最大长度：100 字符
- `vendorAccounts[].bankControlCode`（string，可选）：银行控制码<br>示例值："99999999"<br>数据校验规则：<br>最大长度：10 字符
- `vendorAccounts[].extendInfo`（array<object>，可选）：扩展字段相关信息列表<br>数据校验规则：<br>最大长度：100
- `vendorAccounts[].extendInfo[].fieldType`（integer，父对象存在时必填）：字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。
- `vendorAccounts[].extendInfo[].options`（array<string>，可选）：字段类型为 多选框(4) 下拉多选(6) 时的值<br>示例值：[""""]<br>数据校验规则：<br>最大长度：100
- `vendorAccounts[].extendInfo[].rangeDate`（array<string>，可选）：字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：[""""]<br>数据校验规则：长度范围：2 ～ 2
- `vendorAccounts[].extendInfo[].appendix`（array<object>，可选）：附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：最大长度：10
- `vendorAccounts[].extendInfo[].appendix[].fileId`（string，可选）：文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4"
- `vendorAccounts[].extendInfo[].appendix[].fileType`（string，可选）：文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR
- `vendorAddresses`（array<object>，可选）：地址列表<br>数据校验规则：最大长度：100
- `vendorAddresses[].country`（string，可选）：国家<br>示例值："CN"数据校验规则：最大长度：64 字符
- `vendorAddresses[].province`（string，可选）：省份<br>示例值："MDPS00000001"<br>数据校验规则：<br>最大长度：64 字符
- `vendorAddresses[].city`（string，可选）：城市<br>示例值："MDCY00000001"<br>数据校验规则：<br>最大长度：64 字符
- `vendorAddresses[].county`（string，可选）：县<br>示例值："MDCA00002746"<br>数据校验规则：<br>最大长度：64 字符
- `vendorAddresses[].address`（string，可选）：详细地址<br>示例值："北京市海淀区苏州街"<br>数据校验规则：<br>最大长度：64 字符
- `vendorAddresses[].extendInfo`（array<object>，可选）：扩展字段相关信息列表<br>数据校验规则：<br>最大长度：100
- `vendorAddresses[].extendInfo[].fieldType`（integer，父对象存在时必填）：字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。
- `vendorAddresses[].extendInfo[].options`（array<string>，可选）：字段类型为 多选框(4) 下拉多选(6) 时的值<br>示例值：[""""]<br>数据校验规则：<br>最大长度：100
- `vendorAddresses[].extendInfo[].rangeDate`（array<string>，可选）：字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：["字段名称"]<br>数据校验规则：<br>长度范围：2 ～ 2
- `vendorAddresses[].extendInfo[].appendix`（array<object>，可选）：附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：<br>最大长度：10
- `vendorAddresses[].extendInfo[].appendix[].fileId`（string，可选）：文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4"
- `vendorAddresses[].extendInfo[].appendix[].fileType`（string，可选）：文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR
- `vendorCompanyViews`（array<object>，可选）：公司视图列表<br>数据校验规则：<br>最大长度：100
- `vendorCompanyViews[].companyCode`（string，可选）：公司代码<br>示例值："1001"<br>数据校验规则：<br>最大长度：50 字符
- `vendorCompanyViews[].glAccount`（string，可选）：总账科目<br>示例值："22020101"<br>数据校验规则：<br>最大长度：15 字符
- `vendorCompanyViews[].vendorSiteCode`（string，可选）：交易方sitecode<br>示例值："999999"<br>数据校验规则：最大长度：100 字符
- `vendorCompanyViews[].paymentTerm`（string，可选）：付款条件信息<br>示例值："PT09"<br>数据校验规则：最大长度：255 字符
- `vendorCompanyViews[].downPaymentTerm`（string，可选）：预付条件<br>示例值："PT08"<br>数据校验规则：最大长度：100 字符
- `vendorCompanyViews[].extendInfo`（array<object>，可选）：扩展字段相关信息列表<br>数据校验规则：最大长度：100
- `vendorCompanyViews[].extendInfo[].fieldType`（integer，父对象存在时必填）：字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。
- `vendorCompanyViews[].extendInfo[].options`（array<string>，可选）：字段类型为 多选框(4) 下拉多选(6) 时的值示例值：[""""]<br>数据校验规则：最大长度：100
- `vendorCompanyViews[].extendInfo[].rangeDate`（array<string>，可选）：字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：["字段名称"]数据校验规则：长度范围：2 ～ 2
- `vendorCompanyViews[].extendInfo[].appendix`（array<object>，可选）：附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：最大长度：10
- `vendorCompanyViews[].extendInfo[].appendix[].fileId`（string，可选）：文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4"
- `vendorCompanyViews[].extendInfo[].appendix[].fileType`（string，可选）：文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR
- `vendorContacts`（array<object>，可选）：联系人列表<br>数据校验规则：最大长度：100
- `vendorContacts[].name`（string，可选）：姓名<br>示例值："张三"<br>数据校验规则：<br>最大长度：50 字符
- `vendorContacts[].position`（string，可选）：职位<br>示例值："董事长"<br>数据校验规则：<br>最大长度：50 字符
- `vendorContacts[].email`（string，可选）：邮箱<br>示例值："haha@xxx.com"<br>数据校验规则：<br>最大长度：50 字符
- `vendorContacts[].phone`（string，可选）：手机号<br>示例值："13333323333"<br>数据校验规则：<br>最大长度：50 字符
- `vendorContacts[].remark`（string，可选）：备注<br>示例值："备注"<br>数据校验规则：<br>最大长度：200 字符
- `vendorContacts[].extendInfo`（array<object>，可选）：扩展字段相关信息列表数据<br>校验规则：<br>最大长度：100
- `vendorContacts[].extendInfo[].fieldType`（integer，父对象存在时必填）：字段类型；允许值 0、1、2、3、4、5、6、7、8、12、14。具体值属性以交易方字段配置解释规则为准。
- `vendorContacts[].extendInfo[].options`（array<string>，可选）：字段类型为 多选框(4) 下拉多选(6) 时的值示例值：["字段名称"]<br>数据校验规则：<br>最大长度：100
- `vendorContacts[].extendInfo[].rangeDate`（array<string>，可选）：字段类型是 日期区间(8) 时候的值 数组长度为2 0-startTime 1-endTime<br>示例值：["字段名称"]数据校验规则：长度范围：2 ～ 2
- `vendorContacts[].extendInfo[].appendix`（array<object>，可选）：附件列表 字段类型是 附件(12) 时候的值<br>数据校验规则：<br>最大长度：10
- `vendorContacts[].extendInfo[].appendix[].fileId`（string，可选）：文件id(文件的唯一标识)<br>示例值："5c7237e821a8409d9b8b2e1041cdc9a4"
- `vendorContacts[].extendInfo[].appendix[].fileType`（string，可选）：文件类型<br>示例值："DOX"<br>可选值有：<br>- DOC：DOC<br>- DOCX：DOCX<br>- XLS：XLS<br>- XLSX：XLSX<br>- PNG：PNG<br>- JPG：JPG<br>- JPEG：JPEG<br>- PDF：PDF<br>- ZIP：ZIP<br>- RAR：RAR
- `glAccount`（string，可选）：总账科目<br>示例值："22020101"<br>数据校验规则：<br>最大长度：15 字符
- `downPaymentTerm`（string，可选）：预付条件<br>示例值："PT09"<br>数据校验规则：<br>最大长度：100 字符
- `paymentTerm`（string，可选）：付款条件<br>示例值："PT08"<br>数据校验规则：<br>最大长度：255 字符
- `vendorSiteCode`（string，可选）：交易方site code<br>示例值："999999"<br>数据校验规则：<br>最大长度：100 字符
- 官方参数 `vendor_id`（header，可选）当前没有对应 CLI flag。

## 示例

```bash
contract-cli mdm vendor update --user-id <operator-user-id> <vendor-id> --input-file request.json --profile contract --as app
```

官方请求体示例（动态字段接口仍须以当前租户配置为准）：

```json
{
  "id": "7023646046559404327",
  "adCountry": "CN",
  "adProvince": "MDPS00000001",
  "adCity": "MDCY00001226",
  "address": "上海市浦东新区世纪大道1000号",
  "adPostcode": "100100",
  "legalPerson": "张三",
  "certificationType": "0",
  "certificationId": "913100xxxxx555781R",
  "contactPerson": "李四",
  "isRisked": false,
  "contactTelephone": "021-87853200",
  "contactMobilePhone": "+8617621685955",
  "fax": "021-87853200",
  "email": "shunxing@xxx.com",
  "status": 1,
  "vendor": "V00108006",
  "vendorText": "张三样例",
  "shortText": "王五",
  "vendorType": "",
  "vendorCategory": "11",
  "vendorNature": "0",
  "linkedEmployee": "6959513973725069601",
  "linkedCustomer": "客户",
  "associatedWithLegalEntity": true,
  "appendix": [
    {
      "tenantId": "6977354570259330000",
      "fileId": "609a128628ad4eaebd3063c59928a103",
      "fileName": "xxxx.xlsx",
      "fileType": "XLSX",
      "fileSize": 13367,
      "downloadUrl": "https://xxxx.qfei.cn/downloadxxxxxxxxxx"
    }
  ],
  "extendInfo": [
    {
      "fieldType": 0,
      "fieldValue": "文本值",
      "options": [
        ""
      ],
      "num": 1.11,
      "date": "2021-10-14",
      "rangeDate": [
        ""
      ],
      "fieldCode": "VXX000001",
      "appendix": [
        {
          "tenantId": "6977354570259330000",
          "fileId": "609a128628ad4eaebd3063c59928a103",
          "fileName": "xxxx.xlsx",
          "fileType": "XLSX",
          "fileSize": 13367,
          "downloadUrl": "https://xxxx.qfei.cn/downloadxxxxxxxxxx"
        }
      ]
    }
  ],
  "vendorAccounts": [
    {
      "id": "1453263653228318721",
      "account": "62448345986564434",
      "iban": "46677",
      "accountName": "上海xxx技术有限（上海）分公司",
      "bankId": "MDBK00061195",
      "bankCode": "308290003732",
      "swiftCode": "BOFAUS3NINQ",
      "vendorSiteCode": "99999999",
      "bankName": "xx银行股份有限公司苏州支行",
      "bankAcronym": "ZJTLCB",
      "country": "CN",
      "bankControlCode": "99999999",
      "extendInfo": [
        {
          "fieldType": 0,
          "fieldValue": "文本值",
          "options": [
            ""
          ],
          "num": 1.11,
          "date": "2021-10-14",
          "rangeDate": [
            ""
          ],
          "fieldCode": "VXX000001",
          "appendix": [
            {
              "tenantId": "6977354570259330000",
              "fileId": "609a128628ad4eaebd3063c59928a103",
              "fileName": "xxxx.xlsx",
              "fileType": "XLSX",
              "fileSize": 13367,
              "downloadUrl": "https://xxxx.qfei.cn/downloadxxxxxxxxxx"
            }
          ]
        }
      ]
    }
  ],
  "vendorAddresses": [
    {
      "id": "1433488030078558209",
      "country": "CN",
      "province": "MDPS00000001",
      "city": "MDCY00000001",
      "county": "MDCA00002746",
      "address": "北京市海淀区苏州街",
      "extendInfo": [
        {
          "fieldType": 0,
          "fieldValue": "文本值",
          "options": [
            ""
          ],
          "num": 1.11,
          "date": "2021-10-14",
          "rangeDate": [
            ""
          ],
          "fieldCode": "VXX000001",
          "appendix": [
            {
              "tenantId": "6977354570259330000",
              "fileId": "609a128628ad4eaebd3063c59928a103",
              "fileName": "xxxx.xlsx",
              "fileType": "XLSX",
              "fileSize": 13367,
              "downloadUrl": "https://xxxx.qfei.cn/downloadxxxxxxxxxx"
            }
          ]
        }
      ]
    }
  ],
  "vendorCompanyViews": [
    {
      "id": "1453263653228318721",
      "companyCode": "1001",
      "glAccount": "22020101",
      "vendorSiteCode": "999999",
      "paymentTerm": "PT09",
      "downPaymentTerm": "PT08",
      "extendInfo": [
        {
          "fieldType": 0,
          "fieldValue": "文本值",
          "options": [
            ""
          ],
          "num": 1.11,
          "date": "2021-10-14",
          "rangeDate": [
            ""
          ],
          "fieldCode": "VXX000001",
          "appendix": [
            {
              "tenantId": "6977354570259330000",
              "fileId": "609a128628ad4eaebd3063c59928a103",
              "fileName": "xxxx.xlsx",
              "fileType": "XLSX",
              "fileSize": 13367,
              "downloadUrl": "https://xxxx.qfei.cn/downloadxxxxxxxxxx"
            }
          ]
        }
      ]
    }
  ],
  "vendorContacts": [
    {
      "id": "1433488091906793474",
      "name": "张三",
      "position": "董事长",
      "email": "haha@xxx.com",
      "phone": "13333323333",
      "remark": "备注",
      "extendInfo": [
        {
          "fieldType": 0,
          "fieldValue": "文本值",
          "options": [
            ""
          ],
          "num": 1.11,
          "date": "2021-10-14",
          "rangeDate": [
            ""
          ],
          "fieldCode": "VXX000001",
          "appendix": [
            {
              "tenantId": "6977354570259330000",
              "fileId": "609a128628ad4eaebd3063c59928a103",
              "fileName": "xxxx.xlsx",
              "fileType": "XLSX",
              "fileSize": 13367,
              "downloadUrl": "https://xxxx.qfei.cn/downloadxxxxxxxxxx"
            }
          ]
        }
      ]
    }
  ],
  "glAccount": "22020101",
  "downPaymentTerm": "PT09",
  "paymentTerm": "PT08",
  "vendorSiteCode": "999999"
}
```

## 来源差异说明

- 官方规格路径：`PUT /open-apis/mdm/v1/vendors/7003410079584092448`
- CLI 路径表达：`PUT /open-apis/mdm/v1/vendors/{vendor_id}`；示例 ID 或占位名已统一为 CLI 名称。
- 本页描述请求参数；响应 envelope 和输出格式遵循共享 Skill。
