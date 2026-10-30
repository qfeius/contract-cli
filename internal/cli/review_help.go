package cli

import "strings"

func reviewHelpFlags(extra ...helpFlag) []helpFlag {
	return append([]helpFlag{
		{"--profile <name>", "profile 名称；不传使用当前 profile"},
		{"--as <user>", "仅支持 user；不传时自动使用 user"},
		{"--output <json|yaml|table>", "输出格式，默认 json"},
		{"--raw", "原样输出响应 body"},
	}, extra...)
}

func addReviewHelp(registry map[string]helpTopic) {
	dry := []helpFlag{{"--dry-run", "校验并输出请求，不调用远端"}, {"--print-input", "输出规范化输入，不调用远端"}}
	input := []helpFlag{{"--input-file <path>", "从 JSON 文件读取请求体；与 --data 互斥"}, {"--input <path>", "--input-file 的兼容别名"}, {"--data <json>", "内联 JSON 请求体；与文件输入互斥"}}
	wait := []helpFlag{{"--interval <duration>", "轮询间隔，默认 2s"}, {"--deadline <duration>", "最长等待时间，默认 10m；run 覆盖整个工作流"}}
	query := []helpFlag{{"--task-id <id>", "必填，大于 0 的审查任务 ID"}, {"--business-id <id>", "业务对象 ID"}, {"--visibility-scope <scope>", "可选 contractResult；指定时必须提供 --business-id"}}
	paging := []helpFlag{{"--sort <csv>", "排序，可重复或 CSV；保留逗号示例：--sort '\"name,desc\"'"}, {"--page-index <n>", "页码从 1 开始；0 或省略不传"}, {"--page-size <n>", "每页数量；0 或省略不传"}}
	configNotes := []string{
		"config.selectedPosition、selectedAuditRole、reviewStrength 必填。",
		"reviewStrength 支持弱势、中立、强势，兼容 0、1、2；HTTP 请求统一为数字。",
		"至少提供一种规则来源：非空 selectedCheckListIds，或 matchContractTypeRulePackage=true；两者可组合。",
		"拒绝未知 JSON 字段；fileHash 使用上传响应中的 SHA-256；HTTP fileId 为字符串。",
		"发起请求固定注入 usageReportContext.reportBusinessCode=clm_100_openApi_cli；不接受用户输入覆盖。",
	}
	registry["review"] = helpTopic{Name: "review", Summary: "执行合同智能审查。外部本地文件通过 CLM 预审流程准备，系统合同复用文件引用。", Usage: []string{"contract-cli review <subcommand> [flags]"}, Commands: []helpCommand{
		{"contract-cli review file <subcommand> [flags]", "准备审查输入文件"}, {"contract-cli review subject extract [flags]", "提取合同主体"}, {"contract-cli review task <subcommand> [flags]", "管理审查任务生命周期"}, {"contract-cli review run [flags]", "上传本地文件到 CLM 并输出待选分类阶段"}, {"contract-cli review checklist <subcommand> [flags]", "管理审查清单"}, {"contract-cli review rule <subcommand> [flags]", "管理审查规则"},
	}, Flags: reviewHelpFlags(), Examples: []string{"contract-cli review run --input review-run.json", "contract-cli review task result --task-id 123"}, Notes: []string{"review 下所有远端请求仅支持 user 身份。"}}
	registry["review file"] = helpTopic{Name: "review file", Summary: "准备审查输入文件；不会自动发起审查。", Usage: []string{"contract-cli review file <subcommand> [flags]"}, Commands: []helpCommand{{"contract-cli review file upload [flags]", "引用 CLM 已有文件"}}}
	registry["review file upload"] = helpTopic{Name: "review file upload", Summary: "引用 CLM 已有文件，准备 AI 审查输入。", Usage: []string{"contract-cli review file upload [flags]"}, Flags: reviewHelpFlags(append([]helpFlag{{"--name <name>", "必填，业务文件名（含扩展名）"}}, dry...)...), Notes: []string{"文件名仅支持 .doc/.docx/.pdf；只准备文件引用，不发起审查。", "不接受 --file/--stdin；本地文件请使用 contract-cli review pre-audit prepare。"}}
	registry["review subject"] = helpTopic{Name: "review subject", Usage: []string{"contract-cli review subject extract [flags]"}, Commands: []helpCommand{{"contract-cli review subject extract [flags]", "提取已上传合同参与方"}}}
	registry["review subject extract"] = helpTopic{Name: "review subject extract", Summary: "提取已上传合同参与方，不执行完整审查。", Usage: []string{"contract-cli review subject extract --business-id <id> --file-id <id> [flags]"}, Flags: reviewHelpFlags(append([]helpFlag{{"--business-id <id>", "必填，上传返回的业务对象 ID"}, {"--file-id <id>", "必填，大于 0 的平台文件 ID"}, {"--file-hash <hash>", "可选，服务端文件指纹"}}, dry...)...), Notes: []string{"fileId 自动转换为字符串。"}}
	registry["review task"] = helpTopic{Name: "review task", Usage: []string{"contract-cli review task <subcommand> [flags]"}, Commands: []helpCommand{{"contract-cli review task start [flags]", "发起任务，不上传文件"}, {"contract-cli review task status [flags]", "查询一次当前状态"}, {"contract-cli review task info [flags]", "查询一次任务详情"}, {"contract-cli review task result [flags]", "轮询状态，成功后获取详情"}}}
	registry["review task start"] = helpTopic{Name: "review task start", Summary: "通过 JSON 对象发起审查，不上传文件。", Usage: []string{"contract-cli review task start --input <path>|--data <json> [flags]"}, Flags: reviewHelpFlags(append(append([]helpFlag{}, input...), dry...)...), Examples: []string{`contract-cli review task start --data '{"businessId":"biz-001","fileId":123,"fileHash":"<upload.fileHash>","config":{"selectedPosition":"甲公司","selectedAuditRole":"甲方","reviewStrength":"中立","selectedCheckListIds":["2001"]}}' --dry-run`}, Notes: append([]string{"JSON 必填字段：businessId、fileId（正整数）、fileHash、config。"}, configNotes...)}
	for _, action := range []string{"status", "info", "result"} {
		flags := append([]helpFlag{}, query...)
		summary := "查询一次审查任务状态"
		var notes []string
		if action == "info" {
			summary = "查询一次任务详情，不等待"
		}
		if action == "result" {
			flags = append(flags, wait...)
			summary = "等待任务完成并获取最终结果"
			notes = []string{"内部轮询 status，成功后调用一次 info；失败、取消或超时返回错误和最后可用快照。", "详情中存在有效预览链接时补充 reviewDetailUrl，完整保留签名参数；缺少链接不影响成功。"}
		}
		registry["review task "+action] = helpTopic{Name: "review task " + action, Summary: summary, Usage: []string{"contract-cli review task " + action + " --task-id <id> [flags]"}, Flags: reviewHelpFlags(flags...), Notes: notes}
	}
	registry["review run"] = helpTopic{Name: "review run", Summary: "上传本地文件到 CLM，随后选择分类并继续预审。", Usage: []string{"contract-cli review run --input <path>|--data <json> [flags]"}, Flags: reviewHelpFlags(append(append(append([]helpFlag{}, input...), wait...), dry...)...), Examples: []string{`contract-cli review run --data '{"source":{"type":"file","path":"./合同.pdf","name":"合同.pdf"}}' --dry-run`}}
	registry["review checklist"] = helpTopic{Name: "review checklist", Summary: "管理审查清单。", Usage: []string{"contract-cli review checklist <subcommand> [flags]"}, Notes: []string{"仅支持 user；删除必须 --yes；批量创建/更新以一次数组请求提交。"}}
	registry["review rule"] = helpTopic{Name: "review rule", Summary: "管理审查规则及分组。", Usage: []string{"contract-cli review rule <subcommand> [flags]"}, Commands: []helpCommand{{"contract-cli review rule group <subcommand> [flags]", "管理规则分组"}}, Notes: []string{"仅支持 user；规则操作必须指定 --group-id；分组删除会级联删除规则。"}}
	registry["review rule group"] = helpTopic{Name: "review rule group", Usage: []string{"contract-cli review rule group <subcommand> [flags]"}}
	for _, group := range []string{"review checklist", "review rule", "review rule group"} {
		labels := map[string]string{"list": "查询", "create": "创建", "batch-create": "批量创建", "update": "更新", "batch-update": "批量更新", "delete": "删除", "batch-delete": "批量删除"}
		actions := []string{"list", "create", "batch-create", "update", "batch-update", "delete", "batch-delete"}
		if group == "review rule group" {
			actions = []string{"list", "create", "update", "delete"}
		}
		parent := registry[group]
		for _, action := range actions {
			key := group + " " + action
			parent.Commands = append(parent.Commands, helpCommand{"contract-cli " + key + " [flags]", labels[action]})
			registry[key] = helpTopic{Name: key, Usage: []string{"contract-cli " + key + " [flags]"}, Notes: []string{"仅支持 user 身份。"}}
		}
		registry[group] = parent
	}
	checklistFlags := []helpFlag{{"--name <name>", "清单名称过滤"}, {"--review-stage <csv>", "审查阶段，可重复或 CSV"}, {"--contract-category <csv>", "合同分类，可重复或 CSV"}, {"--start-time <ms>", "更新时间开始，Unix 毫秒"}, {"--end-time <ms>", "更新时间结束，Unix 毫秒"}, {"--enabled[=true|false]", "启用状态，不传不筛选"}, {"--create-employee-id <csv>", "创建员工 ID，可重复或 CSV"}, {"--update-employee-id <csv>", "更新员工 ID，可重复或 CSV"}}
	for _, key := range []string{"review checklist list", "review rule group list", "review rule list"} {
		topic := registry[key]
		flags := append([]helpFlag{}, paging...)
		if key == "review checklist list" {
			flags = append(checklistFlags, flags...)
		}
		if key == "review rule list" {
			flags = append([]helpFlag{{"--group-id <id>", "必填，规则所属分组"}}, flags...)
		}
		topic.Flags = reviewHelpFlags(flags...)
		registry[key] = topic
	}
	completeReviewHelp(registry, input, dry)
	completeReviewReferenceHelp(registry)
	addPreAuditHelp(registry, input, dry, wait, configNotes)
	run := registry["review run"]
	run.Notes = append(run.Notes, "本地文件仅支持 .doc/.docx/.pdf，最大 50 MiB；JSON 参数输入上限仍为 2 MiB。")
	registry["review run"] = run
}

func addPreAuditHelp(registry map[string]helpTopic, input, dry, wait []helpFlag, configNotes []string) {
	root := registry["review"]
	root.Summary = "执行合同智能审查。本地外部文件使用review pre-audit：上传CLM后选择分类、创建预审、确认配置，等待审查完成后返回预审链接。合同ID使用既有分步引用流程。"
	root.Commands = append(root.Commands, helpCommand{"contract-cli review pre-audit <subcommand> [flags]", "本地文件预审及链接恢复"})
	registry["review"] = root
	registry["review pre-audit"] = helpTopic{Name: "review pre-audit", Summary: "将本地外部文件接入CLM预审，完整成功后返回结果和页面地址。", Usage: []string{"contract-cli review pre-audit <prepare|create|start|resume> [flags]"}, Commands: []helpCommand{
		{"contract-cli review pre-audit prepare --file <path> --name <name>", "上传CLM，保存runId并等待选择分类"},
		{"contract-cli review pre-audit create --run-id <id> --category-id <id>", "创建预审并准备AI引用，等待选择配置"},
		{"contract-cli review pre-audit start --run-id <id> --input-file <config.json>", "使用绑定文件和配置发起，等待结果及链接"},
		{"contract-cli review pre-audit resume --run-id <id>", "续查原任务或仅重取原预审链接"},
	}, Notes: []string{"仅支持user身份；分类复用contract category list，在上传后选择。", "不自动打开浏览器；恢复记录位于配置目录preaudit-runs，由用户管理，无后台清理。"}}
	registry["review pre-audit prepare"] = helpTopic{Name: "review pre-audit prepare", Summary: "上传CLM文件后输出runId、sourceFileId及awaiting_category；不发起AI、不返回页面地址。", Usage: []string{"contract-cli review pre-audit prepare --file <path> --name <name> [flags]"}, Flags: reviewHelpFlags(append([]helpFlag{{"--file <path>", "必填，本地普通文件，DOC/DOCX/PDF最大50 MiB"}, {"--name <name>", "必填，包含扩展名的文件名"}}, dry...)...), Examples: []string{"contract-cli review pre-audit prepare --profile contract --file contract.pdf --name 合同.pdf --output json"}}
	registry["review pre-audit create"] = helpTopic{Name: "review pre-audit create", Summary: "按所选分类创建预审并准备AI引用，输出awaiting_config和字符串文件身份；不返回页面地址。", Usage: []string{"contract-cli review pre-audit create --run-id <id> --category-id <id> [flags]"}, Flags: reviewHelpFlags(append([]helpFlag{{"--run-id <id>", "必填，prepare返回的32位小写十六进制runId"}, {"--category-id <id>", "必填，上传后选择的真实分类ID"}}, dry...)...), Notes: []string{"首次创建明确拒绝并退回awaiting_category时，保留已上传文件；用户重新选择分类并执行create后才再次单次创建。", "创建结果未知时不支持重试恢复，保留原run及分类并停止；请人工核实，create和resume均不再次创建。"}}
	startFlags := append([]helpFlag{{"--run-id <id>", "必填，create返回的runId"}}, input...)
	startFlags = append(startFlags, wait...)
	startFlags = append(startFlags, dry...)
	registry["review pre-audit start"] = helpTopic{Name: "review pre-audit start", Summary: "以绑定预审/AI文件发起审查，等待终态和合法CLM链接；完整成功才返回0。", Usage: []string{"contract-cli review pre-audit start --run-id <id> --input-file <config.json>|--data <json> [flags]"}, Flags: reviewHelpFlags(startFlags...), Notes: append([]string{"JSON只接受config对象，禁止覆盖businessId/fileId/fileHash/usageReportContext。", "CLI内部注入usageReportContext.biz1=SMART_PRE_AUDIT；无公开aiFunction字段。", "结果含upload/start/final、runId/preAuditId/taskId；导航只取final.reviewDetailUrl。", "AI成功但链接失败返回错误并保留明细，使用resume仅重新获取链接。"}, configNotes...)}
	resumeFlags := append([]helpFlag{{"--run-id <id>", "必填，原runId"}}, wait...)
	resumeFlags = append(resumeFlags, dry...)
	registry["review pre-audit resume"] = helpTopic{Name: "review pre-audit resume", Summary: "续查已知taskId；审查成功后只重取原详情链接。", Usage: []string{"contract-cli review pre-audit resume --run-id <id> [flags]"}, Flags: reviewHelpFlags(resumeFlags...), Notes: []string{"创建结果未知时不支持重试恢复，请人工核实；未知启动结果且无taskId时停止，不自动重发；上传/引用响应未知也不自动重传。", "账号、租户、profile/API origin必须与原run一致；损坏记录不自动重建。", "未完成选择时返回错误并保留runId；成功明细及合法链接齐备后才返回0。"}}
	run := registry["review run"]
	run.Summary = "本地文件上传CLM后停在awaiting_category，由pre-audit create/start继续。"
	run.Notes = []string{"source.type仅支持file，提供path/name，config可稍后选择；上传后必须选择分类，输出runId/awaiting_category并返回未完成错误，不直接创建AI任务。", "不因wait=false或已有配置跳过分类选择；链接由后续pre-audit start等待成功后交付。"}
	registry["review run"] = run
}

func completeReviewReferenceHelp(registry map[string]helpTopic) {
	topic := registry["review file upload"]
	topic.Summary = "引用 CLM 已有文件，准备 AI 审查输入。"
	topic.Usage = []string{"contract-cli review file upload --business-id <id> --business-file-id <id> --business-file-version <version> --source-locator <json> --name <name> [flags]"}
	topic.Flags = append(topic.Flags,
		helpFlag{"--business-id <id>", "必填，引用文件所属的真实业务对象 ID"},
		helpFlag{"--business-file-id <id>", "必填，实际源文件记录 ID"},
		helpFlag{"--business-file-version <version>", "引用模式必填，真实文件版本，最多 64 字符"},
		helpFlag{"--source-locator <json>", "引用模式必填，JSON 对象字符串；sourceFileType 为 0/1/2；最多 4096 UTF-8 字节"},
		helpFlag{"--source-revision-id <id>", "引用模式可选，内容修订 ID，最多 128 字符"},
		helpFlag{"--source-content-md5 <md5>", "引用模式可选，源 MD5 原值透传，不替代 fileHash"},
		helpFlag{"--channel-type <type>", "可选渠道参数；不传使用服务端默认值，不改变请求头"},
	)
	topic.Examples = append(topic.Examples, `contract-cli review file upload --business-id 123 --business-file-id 456 --name 合同.docx --business-file-version V1.0 --source-locator '{"provider":"clm","sourceFileType":1,"cooperationId":"789","cooperationFileId":"456"}' --dry-run`)
	topic.Notes = append(topic.Notes, "sourceFileType：0 普通文件，1 协商文件，2 合同正文；协商关联 ID 仅放在 sourceLocator。", "引用模式由服务端获取原文件；不得用合同对象 ID 代替业务文件记录 ID。")
	registry["review file upload"] = topic
	fileTopic := registry["review file"]
	fileTopic.Commands[0].Description = "引用 CLM 已有文件"
	registry["review file"] = fileTopic
	registry["review cooperation"] = helpTopic{Name: "review cooperation", Summary: "以个人身份读取协商信息，准备审查引用参数。", Usage: []string{"contract-cli review cooperation <subcommand> [flags]"}, Commands: []helpCommand{{"contract-cli review cooperation get [flags]", "查询个人可见协商详情及文件"}}}
	registry["review cooperation get"] = helpTopic{Name: "review cooperation get", Summary: "查询个人可见协商详情，保留文件类型、版本和关联 ID；不上传或发起审查。", Usage: []string{"contract-cli review cooperation get --cooperation-id <id> [flags]"}, Flags: reviewHelpFlags(helpFlag{"--cooperation-id <id>", "必填，正 int64 协商 ID，不是合同 ID"}), Examples: []string{"contract-cli review cooperation get --profile contract --cooperation-id 123"}, Notes: []string{"仅支持 user 身份，不改变历史 contract cooperation 命令。", "协商引用上传使用 cooperation_file_id，不是底层 file_id；当前主合同需结合文件类型与 latest 标识确定。"}}
	root := registry["review"]
	commands := append([]helpCommand{}, root.Commands[:3]...)
	commands = append(commands, helpCommand{"contract-cli review cooperation get [flags]", "读取个人协商及文件信息"})
	root.Commands = append(commands, root.Commands[3:]...)
	registry["review"] = root
}

// Complete leaf help against the actual parser, retaining Contract CLI's local renderer.
func completeReviewHelp(registry map[string]helpTopic, input, dry []helpFlag) {
	const identityNote = "仅支持 --as user；不传时自动使用 user。"
	examples := map[string][]string{
		"review file upload":     {"contract-cli review file upload --profile contract --file contract.pdf --name 合同.pdf", "contract-cli review file upload --stdin --name 合同.pdf < contract.pdf"},
		"review subject extract": {"contract-cli review subject extract --profile contract --business-id biz-001 --file-id 123", "contract-cli review subject extract --business-id biz-001 --file-id 123 --dry-run"},
		"review task start":      {"contract-cli review task start --profile contract --input-file review-start.json", "contract-cli review task start --input-file review-start.json --dry-run"},
		"review task status":     {"contract-cli review task status --profile contract --task-id 123"},
		"review task info":       {"contract-cli review task info --profile contract --task-id 123", "contract-cli review task info --task-id 123 --business-id biz-001 --visibility-scope contractResult"},
		"review task result":     {"contract-cli review task result --profile contract --task-id 123 --interval 2s --deadline 10m"},
		"review run":             {"contract-cli review run --profile contract --input-file review-run.json", "contract-cli review run --input-file review-run.json --dry-run"},
		"review checklist list":  {"contract-cli review checklist list --profile contract --name 采购 --enabled=true --page-index 1 --page-size 20", "contract-cli review checklist list --review-stage 1,2 --sort '\"name,desc\"'"},
		"review rule list":       {"contract-cli review rule list --profile contract --group-id group-001 --page-index 1 --page-size 20"},
		"review rule group list": {"contract-cli review rule group list --profile contract --page-index 1 --page-size 20"},
	}
	listSummaries := map[string]string{"review checklist list": "查询审查清单，支持名称、分类、阶段、启用状态、时间和分页过滤。", "review rule list": "查询指定分组内的审查规则，支持排序和分页。", "review rule group list": "查询审查规则分组，支持排序和分页。"}
	for key, summary := range listSummaries {
		topic := registry[key]
		topic.Summary = summary
		topic.Notes = []string{"分页参数必须为非负整数；0 或省略时不传分页值。", "多值参数可重复或使用逗号分隔；排序字段与方向需要作为同一项时使用 --sort '\"name,desc\"'。"}
		if key == "review checklist list" {
			topic.Notes = append(topic.Notes, "start-time/end-time 使用 Unix 毫秒；开始时间不能晚于结束时间；--enabled=false 筛选禁用清单。")
		}
		registry[key] = topic
	}
	for _, group := range []string{"review checklist", "review rule group", "review rule"} {
		resource := "审查清单"
		stem := "checklist"
		if group == "review rule group" {
			resource = "规则分组"
			stem = "rule-group"
		}
		if group == "review rule" {
			resource = "审查规则"
			stem = "rule"
		}
		for _, action := range []string{"create", "batch-create", "update", "batch-update", "delete", "batch-delete"} {
			key := group + " " + action
			topic, exists := registry[key]
			if !exists {
				continue
			}
			verb := map[string]string{"create": "创建", "batch-create": "批量创建", "update": "更新", "batch-update": "批量更新", "delete": "删除", "batch-delete": "批量删除"}[action]
			topic.Summary = verb + resource + "。"
			topic.Usage = []string{"contract-cli " + key + " [flags]"}
			topic.Notes = nil
			flags := []helpFlag{}
			prefix := "contract-cli " + key + " --profile contract"
			if group == "review rule" {
				flags = append(flags, helpFlag{"--group-id <id>", "必填，规则所属分组 ID"})
				prefix += " --group-id group-001"
			}
			switch action {
			case "create", "batch-create", "update", "batch-update":
				if action == "update" {
					idFlag := "--id"
					if group == "review rule" {
						idFlag = "--rule-id"
					}
					flags = append(flags, helpFlag{idFlag + " <id>", "必填，目标" + resource + " ID"})
					prefix += " " + idFlag + " " + stem + "-001"
				}
				flags = append(flags, input...)
				flags = append(flags, dry...)
				topic.Examples = []string{prefix + " --input-file " + stem + ".json", prefix + " --input-file " + stem + ".json --dry-run"}
				shape := "JSON 对象"
				if action == "batch-create" || action == "batch-update" {
					shape = "非空 JSON 对象数组"
				}
				topic.Notes = append(topic.Notes, "通过 --input-file（兼容 --input）或 --data 提供"+shape+"。")
				if group == "review checklist" {
					topic.Notes = append(topic.Notes, "清单字段：name、reviewRuleIds；可选 contractCategory、reviewStage、enabled。")
				}
				if group == "review rule group" {
					topic.Notes = append(topic.Notes, "规则分组字段：name；可选 sourceType。")
				}
				if group == "review rule" {
					topic.Notes = append(topic.Notes, "规则字段：name、content、riskLevel；可选 sourceType。")
				}
				if action == "update" {
					topic.Notes = append(topic.Notes, "请求体若包含 id，必须是与目标 ID 一致的字符串。")
				}
				if action == "batch-update" {
					topic.Notes = append(topic.Notes, "数组中的每项需要包含 id。")
				}
			case "delete":
				idFlag := "--id"
				if group == "review rule" {
					idFlag = "--rule-id"
				}
				flags = append(flags, helpFlag{idFlag + " <id>", "必填，目标" + resource + " ID"}, helpFlag{"--yes", "确认删除"})
				flags = append(flags, dry...)
				topic.Examples = []string{prefix + " " + idFlag + " " + stem + "-001 --dry-run", prefix + " " + idFlag + " " + stem + "-001 --yes"}
				topic.Notes = append(topic.Notes, "仅实际删除需要 --yes；预览不需要确认，也不调用远端。")
				if group == "review rule group" {
					topic.Notes = append(topic.Notes, "删除分组会级联删除其所属规则。")
				}
			case "batch-delete":
				flags = append(flags, helpFlag{"--id <id>", "待删除 ID，可重复或使用逗号分隔；与 JSON 输入互斥"})
				flags = append(flags, input...)
				flags = append(flags, helpFlag{"--yes", "确认批量删除"})
				flags = append(flags, dry...)
				topic.Examples = []string{prefix + ` --id ` + stem + `-001,` + stem + `-002 --dry-run`, prefix + ` --data '["` + stem + `-001","` + stem + `-002"]' --yes`}
				topic.Notes = append(topic.Notes, "ID 参数与 JSON 字符串数组只能选择一种；去除首尾空白并去重，拒绝空 ID。", "仅实际删除需要 --yes；预览不需要确认，也不调用远端。")
			}
			topic.Flags = reviewHelpFlags(flags...)
			registry[key] = topic
		}
	}
	// Use the established Name -> description -> Usage -> Flags -> Examples -> Notes layout.
	for key, topic := range registry {
		if key != "review" && !strings.HasPrefix(key, "review ") {
			continue
		}
		topic.Name = key
		if values, exists := examples[key]; exists {
			topic.Examples = values
		}
		if len(topic.Commands) == 0 {
			topic.Usage = []string{"contract-cli " + key + " [flags]"}
			clean := []string{identityNote, "单次远端请求默认超时 30s；run/result 还受整体 deadline 限制。"}
			for _, note := range topic.Notes {
				if note == "仅支持 user 身份。" {
					continue
				}
				clean = append(clean, note)
			}
			topic.Notes = clean
		}
		registry[key] = topic
	}
	create := registry["review checklist create"]
	create.Examples = append(create.Examples, `contract-cli review checklist create --profile contract --as user --data '{"name":"采购合同审查","reviewRuleIds":["rule-001"]}' --dry-run`)
	registry["review checklist create"] = create
}
