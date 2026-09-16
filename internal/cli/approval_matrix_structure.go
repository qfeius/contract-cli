package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

/*
normalizeApprovalMatrixArgs 将交互文档中的 approval-matrix 命令映射到原有 rule 命令。
入参 args（[]string）为完整参数，返回新的 []string；非审批矩阵参数保持原值。
*/
func normalizeApprovalMatrixArgs(args []string) []string {
	offset := 0
	if len(args) > 0 && args[0] == "help" {
		offset = 1
	}
	if len(args) <= offset || args[offset] != "approval-matrix" {
		return args
	}
	tail := args[offset+1:]
	if len(tail) == 0 {
		return append(append([]string{}, args[:offset]...), "rule")
	}
	prefix := []string{"rule", "table"}
	switch tail[0] {
	case "group", "employee", "department", "role", "symbol", "loop-function":
		prefix = []string{"rule"}
	case "table":
		tail = tail[1:]
		if len(tail) > 0 && tail[0] == "publish" {
			tail[0] = "release"
		}
	case "publish":
		tail[0] = "release"
	}
	result := append([]string{}, args[:offset]...)
	result = append(result, prefix...)
	return append(result, tail...)
}

/*
runRuleStructure 调用规则组、矩阵、列结构和只读人员搜索接口。
入参 ctx（context.Context）为上下文，resource（string）为资源类型，args（[]string）为动作及参数。
返回 error 表示参数或开放平台调用失败；成功使用项目统一响应输出。
*/
func (a *App) runRuleStructure(ctx context.Context, resource string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing rule %s subcommand", resource)
	}
	action := args[0]
	method := ""
	switch resource + "/" + action {
	case "group/get", "table/get":
		method = http.MethodGet
	case "group/create", "table/create", "column/add", "employee/search":
		method = http.MethodPost
	case "table/update", "column/update", "column/update-condition", "column/update-result":
		method = http.MethodPut
	case "table/delete", "column/delete":
		method = http.MethodDelete
	default:
		return fmt.Errorf("unknown rule %s subcommand %q", resource, action)
	}
	parsed, err := parseArgs(args[1:], structuredValueFlags("--product-id", "--group-id", "--table-id", "--column-id"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("use --product-id, --group-id, --table-id and --column-id to locate the resource")
	}
	product, err := requiredParsedValue(parsed, "--product-id")
	if err != nil {
		return err
	}
	path := "/open-apis/rule_engine/v1/products/" + escapePathSegment(product) + "/groups"
	if resource != "group" || action != "create" {
		group, err := requiredParsedValue(parsed, "--group-id")
		if err != nil {
			return err
		}
		path += "/" + escapePathSegment(group)
	}
	if resource == "employee" {
		path += "/employees/search"
	} else if resource != "group" {
		path += "/rule_tables"
		if resource != "table" || action != "create" {
			table, err := requiredParsedValue(parsed, "--table-id")
			if err != nil {
				return err
			}
			path += "/" + escapePathSegment(table)
		}
	}
	if resource == "column" {
		path += "/table_columns"
		if action != "add" {
			column, err := requiredParsedValue(parsed, "--column-id")
			if err != nil {
				return err
			}
			path += "/" + escapePathSegment(column)
		}
	}
	options := parseCommandOptions(parsed)
	if resource == "employee" && parsed.String("--user-id-type") != "" && parsed.String("--user-id-type") != "user_id" {
		return fmt.Errorf("employee search only supports --user-id-type user_id")
	}
	var body []byte
	if method == http.MethodPost || method == http.MethodPut {
		body, err = resolveRequiredRawBody(options)
		if err != nil {
			return err
		}
		if err = validateRuleStructureBody(resource, action, parsed.String("--group-id"), parsed.String("--table-id"), body); err != nil {
			return err
		}
	} else if err = rejectRawBody(options, "rule "+resource+" "+action); err != nil {
		return err
	}
	return a.executeRuleOpenPlatformRequest(ctx, options, method, path, nil, body)
}

/*
validateRuleStructureBody 校验后端 DTO 明确要求的关键字段，防止混用页面和开平参数。
入参 resource/action/groupID/tableID（string）定位操作，body（[]byte）为原始 JSON；返回 error 为字段问题。
*/
func validateRuleStructureBody(resource, action, groupID, tableID string, body []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return fmt.Errorf("request body must be a JSON object")
	}
	if resource == "employee" {
		// 可选规则组编码必须与路径一致；租户仍由服务端鉴权解析。
		if raw, ok := fields["group_code"]; ok {
			var groupCode *string
			if json.Unmarshal(raw, &groupCode) != nil || (groupCode != nil && (strings.TrimSpace(*groupCode) == "" || utf8.RuneCountInString(*groupCode) > 30 || *groupCode != groupID)) {
				return fmt.Errorf("group_code must match --group-id and be a non-empty string of at most 30 characters")
			}
		}
		var keyword string
		if json.Unmarshal(fields["param"], &keyword) != nil || strings.TrimSpace(keyword) == "" || utf8.RuneCountInString(keyword) > 100 {
			return fmt.Errorf("param must be a non-empty string of at most 100 characters")
		}
		for field := range fields {
			if field != "param" && field != "group_code" {
				return fmt.Errorf("employee search only accepts param and group_code; use --product-id and --group-id for scope")
			}
		}
		return nil
	}
	required := []string{"name"}
	if resource == "group" {
		required = append(required, "group_id")
	}
	if resource == "column" {
		required = []string{"table_column_name"}
		if action == "add" {
			required = []string{"base_table_column_id"}
		}
		if action == "update-condition" {
			required = append(required, "value_type", "symbol")
		}
		if action == "update-result" {
			required = append(required, "result_type")
		}
	}
	for _, key := range required {
		var value string
		if json.Unmarshal(fields[key], &value) != nil || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must be a non-empty string", key)
		}
	}
	if resource == "column" && action == "add" {
		var direction int
		if json.Unmarshal(fields["direction"], &direction) != nil || (direction != -1 && direction != 1) {
			return fmt.Errorf("direction must be -1 (left) or 1 (right)")
		}
	}
	if resource == "table" {
		for _, key := range []string{"match_policy", "node_repetition_policy"} {
			if raw, ok := fields[key]; ok && string(raw) != "null" {
				var value int
				if json.Unmarshal(raw, &value) != nil || value < 0 || value > 2 {
					return fmt.Errorf("%s must be an integer from 0 to 2", key)
				}
			}
		}
		if action == "update" {
			var code string
			if raw, ok := fields["rule_table_id"]; ok {
				if json.Unmarshal(raw, &code) != nil || (code != "" && code != tableID) {
					return fmt.Errorf("rule_table_id is immutable; it must match --table-id")
				}
			}
		}
	}
	return nil
}

/*
addRuleStructureHelp 注册结构命令、只读人员搜索及请求字段提示。
入参 registry（map[string]helpTopic）为帮助注册表；返回值为空，就地新增主题。
*/
func addRuleStructureHelp(registry map[string]helpTopic) {
	defer addMatrixExtensionHelp(registry)
	for _, action := range []string{"get", "cancel"} {
		name := "rule table import " + action
		registry[name] = helpTopic{Name: name, Usage: []string{"contract-cli " + name + " --plan-id <id> [flags]"}, Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{{"--plan-id <id>", "本地计划 ID"}}), Notes: []string{"get 只读本地进度；cancel 在没有执行中的批次时取消后续操作，不撤销已成功写入。"}}
		parent := registry["rule table import"]
		parent.Commands = append(parent.Commands, helpCommand{"contract-cli " + name + " [flags]", "读取或取消导入计划"})
		registry["rule table import"] = parent
	}
	apply := registry["rule table import apply"]
	apply.Flags = append(apply.Flags, helpFlag{"--rows <1,2>", "仅执行已确认的计划行序号"}, helpFlag{"--batch-size <n>", "本次最多执行 n 行，剩余进度保留为 paused"})
	apply.Notes = append(apply.Notes, "执行前重读列头，24 小时过期或应用/环境变化时返回 invalidated；不确定结果立即停止后续行。")
	registry["rule table import apply"] = apply
	groups := []struct {
		parent  string
		actions []string
	}{
		{"rule employee", []string{"search"}},
		{"rule group", []string{"get", "create"}},
		{"rule table", []string{"get", "create", "update", "delete"}},
		{"rule table column", []string{"add", "update", "update-condition", "update-result", "delete"}},
	}
	for _, group := range groups {
		parent := registry[group.parent]
		parent.Name = group.parent
		if len(parent.Usage) == 0 {
			parent.Usage = []string{"contract-cli " + group.parent + " <subcommand> [flags]"}
		}
		for _, action := range group.actions {
			name := group.parent + " " + action
			flags := []helpFlag{{"--product-id <id>", "产品编码"}}
			if group.parent != "rule group" || action != "create" {
				flags = append(flags, helpFlag{"--group-id <id>", "规则组对外编码"})
			}
			if group.parent == "rule table column" || (group.parent == "rule table" && action != "create") {
				flags = append(flags, helpFlag{"--table-id <id>", "规则表对外编码（不是数据库主键）"})
			}
			if group.parent == "rule table column" && action != "add" {
				flags = append(flags, helpFlag{"--column-id <id>", "列头返回的对外列 ID"})
			}
			notes := []string{"支持 --as user 或 --as app。写操作先由 Agent 展示计划、确认，再执行并查询核验。", "approval-matrix table/group/column 等别名映射到对应 rule 命令；api call 保持关闭。"}
			if action != "get" && action != "delete" {
				flags = append(flags, jsonBodyFlags()...)
			}
			switch group.parent {
			case "rule employee":
				notes = []string{"只读 POST 搜索：JSON 为 {\"param\":\"姓名关键词\",\"group_code\":\"approve_matrix\"}；param 最长 100 字符，可选 group_code 必须与 --group-id 一致；支持 user/app。", "返回 data 数组，employee_id 为矩阵外部人员 ID（十进制字符串）；仅 selectable=true 可用于写入，重名须选择，禁止自动取第一人。", "user_id_type 固定使用默认 user_id 语义；不把内部员工编号或 ou_ ID 写入规则。"}
			case "rule group":
				notes = append(notes, "创建 JSON：group_id、name 必填，description 可选。")
			case "rule table":
				notes = append(notes, "写入 JSON：name 必填；rule_table_id、description、match_policy、node_repetition_policy 可选。策略值为 0..2；更新时 rule_table_id 不可改变。", "更新 description 省略会按后端空值语义处理；保留原描述时先 get 并带回。")
			case "rule table column":
				notes = append(notes, "add JSON：base_table_column_id 必填，direction 为 -1 左侧或 1 右侧；新列类型继承基准列，随后用 update 配置。", "条件列更新：table_column_name、value_type、symbol；可选 value_id/value_code/value_name/is_department_loop/multi_select_match_mode。", "结果列更新：table_column_name、result_type；可选 result_code/default_value。default_value 是字符串，人员/部门为逗号分隔的数字外部 ID。", "已有值且类型或操作符改变单元格类型时，服务端阻止更新；需要先单独确认清空。优先级列、备注列及最后必要列受服务端保护。")
			}
			registry[name] = helpTopic{Name: name, Usage: []string{"contract-cli " + name + " [flags]"}, Flags: concatHelpFlags(openPlatformCommonFlags(), flags), Notes: notes}
			parent.Commands = append(parent.Commands, helpCommand{"contract-cli " + name + " [flags]", "审批矩阵结构操作"})
		}
		registry[group.parent] = parent
	}
	for _, pair := range [][2]string{{"rule", "rule group"}, {"rule", "rule employee"}, {"rule table", "rule table column"}} {
		topic := registry[pair[0]]
		topic.Commands = append(topic.Commands, helpCommand{"contract-cli " + pair[1] + " <subcommand> [flags]", "审批矩阵结构操作"})
		registry[pair[0]] = topic
	}
}
