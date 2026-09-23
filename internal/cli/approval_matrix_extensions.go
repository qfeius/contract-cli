package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"cn.qfei/contract-cli/internal/openplatform"
)

// matrixSymbol is the CLI-owned operator contract.  The rule-engine backend
// validates the selected symbol when a column is updated; symbol query itself
// is deliberately local so configuration does not depend on a discovery API.
type matrixSymbol struct {
	Symbol     string   `json:"symbol"`
	SymbolName string   `json:"symbolName"`
	ValueTypes []string `json:"valueTypes"`
}

var builtinMatrixSymbols = map[string][]matrixSymbol{
	"STRING": {
		{Symbol: "=", SymbolName: "等于", ValueTypes: []string{"STRING"}},
		{Symbol: "!=", SymbolName: "不等于", ValueTypes: []string{"STRING"}},
		{Symbol: "in", SymbolName: "在…之内", ValueTypes: []string{"COLLECTION"}},
		{Symbol: "notIn", SymbolName: "不在…之内", ValueTypes: []string{"COLLECTION"}},
	},
	"NUMBER": {
		{Symbol: "=", SymbolName: "等于", ValueTypes: []string{"NUMBER"}},
		{Symbol: ">", SymbolName: "大于", ValueTypes: []string{"NUMBER"}},
		{Symbol: ">=", SymbolName: "大于等于", ValueTypes: []string{"NUMBER"}},
		{Symbol: "<", SymbolName: "小于", ValueTypes: []string{"NUMBER"}},
		{Symbol: "<=", SymbolName: "小于等于", ValueTypes: []string{"NUMBER"}},
		{Symbol: "!=", SymbolName: "不等于", ValueTypes: []string{"NUMBER"}},
		{Symbol: "in", SymbolName: "在…之内", ValueTypes: []string{"COLLECTION"}},
		{Symbol: "notIn", SymbolName: "不在…之内", ValueTypes: []string{"COLLECTION"}},
	},
	"COLLECTION": {
		{Symbol: "contain", SymbolName: "包含", ValueTypes: []string{"COLLECTION"}},
		{Symbol: "notContain", SymbolName: "不包含", ValueTypes: []string{"COLLECTION"}},
		{Symbol: "=", SymbolName: "等于", ValueTypes: []string{"COLLECTION"}},
		{Symbol: "in", SymbolName: "在…之内", ValueTypes: []string{"COLLECTION"}},
		{Symbol: "notIn", SymbolName: "不在…之内", ValueTypes: []string{"COLLECTION"}},
		{Symbol: "isNull", SymbolName: "为空", ValueTypes: []string{"COLLECTION"}},
		{Symbol: "isNotNull", SymbolName: "不为空", ValueTypes: []string{"COLLECTION"}},
	},
	"EMPLOYEE_COLLECTION": {
		{Symbol: "contain", SymbolName: "包含", ValueTypes: []string{"EMPLOYEE_COLLECTION"}},
		{Symbol: "notContain", SymbolName: "不包含", ValueTypes: []string{"EMPLOYEE_COLLECTION"}},
		{Symbol: "=", SymbolName: "等于", ValueTypes: []string{"EMPLOYEE_COLLECTION"}},
		{Symbol: "in", SymbolName: "在…之内", ValueTypes: []string{"EMPLOYEE_COLLECTION"}},
		{Symbol: "notIn", SymbolName: "不在…之内", ValueTypes: []string{"EMPLOYEE_COLLECTION"}},
		{Symbol: "isNull", SymbolName: "为空", ValueTypes: []string{"EMPLOYEE_COLLECTION"}},
		{Symbol: "isNotNull", SymbolName: "不为空", ValueTypes: []string{"EMPLOYEE_COLLECTION"}},
	},
	"DEPARTMENT_COLLECTION": {
		{Symbol: "contain", SymbolName: "包含", ValueTypes: []string{"DEPARTMENT_COLLECTION"}},
		{Symbol: "notContain", SymbolName: "不包含", ValueTypes: []string{"DEPARTMENT_COLLECTION"}},
		{Symbol: "=", SymbolName: "等于", ValueTypes: []string{"DEPARTMENT_COLLECTION"}},
		{Symbol: "in", SymbolName: "在…之内", ValueTypes: []string{"DEPARTMENT_COLLECTION"}},
		{Symbol: "notIn", SymbolName: "不在…之内", ValueTypes: []string{"DEPARTMENT_COLLECTION"}},
		{Symbol: "isNull", SymbolName: "为空", ValueTypes: []string{"DEPARTMENT_COLLECTION"}},
		{Symbol: "isNotNull", SymbolName: "不为空", ValueTypes: []string{"DEPARTMENT_COLLECTION"}},
	},
	"BOOLEAN": {
		{Symbol: "=", SymbolName: "等于", ValueTypes: []string{"BOOLEAN"}},
		{Symbol: "!=", SymbolName: "不等于", ValueTypes: []string{"BOOLEAN"}},
	},
}

/* isMatrixLookup 判断矩阵只读资源；入参 resource 为 string，返回 bool，不改变其他资源分发。 */
func isMatrixLookup(resource string) bool {
	switch resource {
	case "department", "role", "symbol", "loop-function":
		return true
	}
	return false
}

/* runMatrixExtension 分发配套查询和列操作；ctx 为上下文，resource/args 为命令参数；返回参数或请求错误。 */
func (a *App) runMatrixExtension(ctx context.Context, resource string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing %s subcommand", resource)
	}
	action, args := args[0], args[1:]
	routes := map[string]string{
		"employee/batch-get": "employees/batch_get", "department/search": "departments/search",
		"department/batch-get": "departments/batch_get", "role/search": "roles/search", "role/batch-get": "roles/batch_get",
		"loop-function/query": "loop_functions/query",
		"column/patch":        "", "column/preview": "",
	}
	localSymbols := resource == "symbol" && action == "query"
	suffix, ok := routes[resource+"/"+action]
	if !ok && !localSymbols {
		return fmt.Errorf("unknown matrix command %s %s", resource, action)
	}
	parsed, err := parseArgs(args, structuredValueFlags("--product-id", "--group-id", "--table-id", "--column-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("matrix command uses named flags")
	}
	group := parsed.String("--group-id")
	options := parseCommandOptions(parsed)
	if parsed.String("--user-id-type") != "" && parsed.String("--user-id-type") != "user_id" {
		return fmt.Errorf("matrix extensions only support --user-id-type user_id")
	}
	var body []byte
	body, err = resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	if err = validateMatrixExtensionBody(resource, action, group, body); err != nil {
		return err
	}
	if localSymbols {
		if parsed.String("--table-id") != "" || parsed.String("--column-id") != "" {
			return fmt.Errorf("symbol query does not accept --table-id or --column-id")
		}
		return a.renderBuiltinMatrixSymbols(options, body)
	}
	product, requiredGroup, err := requiredRuleProductGroup(parsed)
	if err != nil {
		return err
	}
	path := "/open-apis/rule_engine/v1/products/" + escapePathSegment(product) + "/groups/" + escapePathSegment(requiredGroup)
	scoped := resource == "column"
	if scoped {
		table, e := requiredParsedValue(parsed, "--table-id")
		if e != nil {
			return e
		}
		path += "/rule_tables/" + escapePathSegment(table)
	} else if parsed.String("--table-id") != "" || parsed.String("--column-id") != "" {
		return fmt.Errorf("directory and metadata queries do not accept --table-id or --column-id")
	}
	method := http.MethodPost
	if resource == "column" {
		column, e := requiredParsedValue(parsed, "--column-id")
		if e != nil {
			return e
		}
		suffix = "table_columns/" + escapePathSegment(column)
		if action == "preview" {
			suffix += "/preview"
		} else {
			method = http.MethodPatch
		}
	}
	path += "/" + suffix
	return a.executeRuleOpenPlatformRequest(ctx, options, method, path, nil, body)
}

/* renderBuiltinMatrixSymbols 输出 CLI 内置的条件运算符映射；body 为 value_type 查询体。 */
func (a *App) renderBuiltinMatrixSymbols(options commandOptions, body []byte) error {
	var request struct {
		ValueType string `json:"value_type"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return fmt.Errorf("invalid symbol query body: %w", err)
	}
	symbols, ok := builtinMatrixSymbols[request.ValueType]
	if !ok {
		return fmt.Errorf("invalid value_type %q", request.ValueType)
	}
	responseBody, err := json.Marshal(struct {
		Code int            `json:"code"`
		Data []matrixSymbol `json:"data"`
		Msg  string         `json:"msg"`
	}{Code: 0, Data: symbols, Msg: "success"})
	if err != nil {
		return fmt.Errorf("encode symbol query response: %w", err)
	}
	return a.renderOpenPlatformResponse(options, openplatform.Response{StatusCode: http.StatusOK, Body: responseBody})
}

/* validateMatrixExtensionBody 校验查询白名单、范围及受保护写入契约；resource/action/group 为 string，body 为字节；返回 error。 */
func validateMatrixExtensionBody(resource, action, group string, body []byte) error {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || fields == nil {
		return fmt.Errorf("request body must be an object")
	}
	if resource == "column" {
		if len(fields) == 0 {
			return fmt.Errorf("column changes must not be empty")
		}
		return nil
	}
	for k, v := range fields {
		switch k {
		case "group_code":
			var code string
			if json.Unmarshal(v, &code) != nil || (group != "" && code != group) {
				return fmt.Errorf("group_code must match --group-id")
			}
		case "param":
			var s string
			if action != "search" || json.Unmarshal(v, &s) != nil || strings.TrimSpace(s) == "" || utf8.RuneCountInString(s) > 100 {
				return fmt.Errorf("invalid param")
			}
		case "ids":
			var ids []string
			if action != "batch-get" || json.Unmarshal(v, &ids) != nil || len(ids) < 1 || len(ids) > 100 {
				return fmt.Errorf("ids must contain 1..100 external ID strings")
			}
			for _, id := range ids {
				if resource == "role" {
					if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`).MatchString(id) {
						return fmt.Errorf("invalid role ID")
					}
				} else {
					if n, e := strconv.ParseInt(id, 10, 64); e != nil || n <= 0 || strconv.FormatInt(n, 10) != id {
						if resource == "department" {
							return fmt.Errorf("department batch-get ID %q must be a positive numeric directory department_id; use the returned open_department_id for rule rows and imports", id)
						}
						return fmt.Errorf("invalid external ID")
					}
				}
			}
		case "value_type":
			var s string
			if action != "query" || json.Unmarshal(v, &s) != nil || !regexp.MustCompile(`^(NUMBER|STRING|COLLECTION|BOOLEAN|EMPLOYEE_COLLECTION|DEPARTMENT_COLLECTION)$`).MatchString(s) {
				return fmt.Errorf("invalid value_type")
			}
		default:
			return fmt.Errorf("unknown matrix query field %s", k)
		}
	}
	required := map[string]string{"search": "param", "batch-get": "ids", "query": "value_type"}[action]
	if len(fields[required]) == 0 {
		return fmt.Errorf("%s is required", required)
	}
	return nil
}

/* validMatrixDecimal 精确检查十进制位数，不经 float64；value 为 JSON number 字符串，返回是否符合后端 Digits(30,8)。 */
func validMatrixDecimal(value string) bool {
	if len(value) > 100 || !regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`).MatchString(value) {
		return false
	}
	parts := strings.Split(strings.ToLower(strings.TrimPrefix(value, "-")), "e")
	exponent := 0
	if len(parts) == 2 {
		var err error
		exponent, err = strconv.Atoi(parts[1])
		if err != nil || exponent > 100 || exponent < -100 {
			return false
		}
	}
	mantissa := strings.Split(parts[0], ".")
	fraction := 0
	digits := mantissa[0]
	if len(mantissa) == 2 {
		fraction = len(mantissa[1])
		digits += mantissa[1]
	}
	precision := len(strings.TrimLeft(digits, "0"))
	if precision == 0 {
		precision = 1
	}
	scale := fraction - exponent
	return scale <= 8 && precision-scale <= 30
}

/* addMatrixExtensionHelp 注册新增能力帮助；registry 为帮助索引，返回 void，旧命令说明保留。 */
func addMatrixExtensionHelp(registry map[string]helpTopic) {
	commands := []string{"rule employee batch-get", "rule department search", "rule department batch-get", "rule role search", "rule role batch-get", "rule symbol query", "rule loop-function query", "rule table column patch", "rule table column preview"}
	for _, name := range commands {
		flags := []helpFlag{{"--product-id <id>", "产品编码"}, {"--group-id <id>", "规则组编码"}}
		if strings.HasPrefix(name, "rule table ") {
			flags = append(flags, helpFlag{"--table-id <id>", "矩阵外部编码"})
		}
		if strings.Contains(name, " column ") {
			flags = append(flags, helpFlag{"--column-id <id>", "列外部编码"})
		}
		flags = append(flags, jsonBodyFlags()...)
		registry[name] = helpTopic{Name: name, Usage: []string{"contract-cli " + name + " [flags]"}, Flags: concatHelpFlags(openPlatformCommonFlags(), flags), Notes: []string{"支持 user/app，保留 product/group/tenant 隔离。查询详见矩阵扩展参数文档；写入前展示变更并确认。", "column patch 只更新显式提供字段，preview 只返回影响行数。"}}
		if name == "rule symbol query" {
			topic := registry[name]
			topic.Notes = []string{"使用 JSON {\"value_type\":\"NUMBER\"} 查询 CLI 内置映射。页面条件类型为 STRING、COLLECTION、NUMBER、EMPLOYEE_COLLECTION、DEPARTMENT_COLLECTION；保留 BOOLEAN 旧接口兼容。", "此命令只读取 CLI 内置的 symbol、symbolName、valueTypes，不读取 profile、不需要授权、不调用 symbols/query；列更新时仍由后端校验 symbol。"}
			registry[name] = topic
		}
		if name == "rule department search" {
			topic := registry[name]
			topic.Notes = []string{"使用 JSON {\"param\":\"部门关键词\"} 搜索部门；只采用 selectable=true 的候选。", "结果中的 department_id 是数字目录查询 ID；open_department_id（od-...）才是规则行创建、更新、搜索和批量导入可直接使用的部门 ID。"}
			registry[name] = topic
		}
		if name == "rule department batch-get" {
			topic := registry[name]
			topic.Notes = []string{"使用 JSON {\"ids\":[\"<数字 department_id>\"]} 回读部门目录；支持 user/app。", "该命令把目录数字 ID 回查为包含 open_department_id 的候选；规则行和批量导入只使用返回的 od-... 值。"}
			registry[name] = topic
		}
		parts := strings.Split(name, " ")
		parentName := strings.Join(parts[:len(parts)-1], " ")
		parent := registry[parentName]
		parent.Name = parentName
		if len(parent.Usage) == 0 {
			parent.Usage = []string{"contract-cli " + parentName + " <subcommand> [flags]"}
		}
		parent.Commands = append(parent.Commands, helpCommand{"contract-cli " + name + " [flags]", "矩阵查询或受保护操作"})
		registry[parentName] = parent
	}
	for _, name := range []string{"rule department", "rule role", "rule symbol", "rule loop-function"} {
		parts := strings.Split(name, " ")
		parentName := strings.Join(parts[:len(parts)-1], " ")
		parent := registry[parentName]
		parent.Commands = append(parent.Commands, helpCommand{"contract-cli " + name + " <subcommand> [flags]", "矩阵配置配套能力"})
		registry[parentName] = parent
	}
	plan := registry["rule table import plan"]
	plan.Notes = append(plan.Notes, "导入复用现有行创建/更新接口，结果不确定时停止并先查询核验。NUMBER 支持最多 30 位整数、8 位小数，不自动换算单位；显式 null 清空单元格。", "DEPARTMENT_COLLECTION 只接受目录返回的 open_department_id 字符串（od-...）；数字 department_id 先通过 rule department batch-get 回查转换。")
	registry["rule table import plan"] = plan
}
