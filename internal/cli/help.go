package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"cn.qfei/contract-cli/internal/build"
)

type helpTopic struct {
	Name     string
	Summary  string
	Usage    []string
	Commands []helpCommand
	Flags    []helpFlag
	Examples []string
	Notes    []string
}

type helpCommand struct {
	Name        string
	Description string
}

type helpFlag struct {
	Name        string
	Description string
}

func isHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "help" {
		return true
	}
	for _, arg := range args {
		if isHelpFlag(arg) {
			return true
		}
	}
	return false
}

func isHelpFlag(arg string) bool {
	switch arg {
	case "--help", "-h", "-help":
		return true
	default:
		return false
	}
}

func resolveHelpTopic(args []string) (helpTopic, error) {
	registry := helpRegistry()
	path := normalizeHelpPath(args)
	if len(path) == 0 {
		return registry["contract-cli"], nil
	}

	key := strings.Join(path, " ")
	if topic, ok := registry[key]; ok {
		return topic, nil
	}

	bestKey := ""
	bestLen := 0
	for i := len(path); i > 0; i-- {
		candidate := strings.Join(path[:i], " ")
		if _, ok := registry[candidate]; ok {
			bestKey = candidate
			bestLen = i
			break
		}
	}

	if bestKey != "" {
		topic := registry[bestKey]
		if len(topic.Commands) == 0 {
			return topic, nil
		}
		if bestLen < len(path) {
			unknown := strings.Join(path[:bestLen+1], " ")
			return helpTopic{}, unknownHelpTopicError(unknown)
		}
	}

	return helpTopic{}, unknownHelpTopicError(key)
}

func normalizeHelpPath(args []string) []string {
	start := 0
	if len(args) > 0 && args[0] == "help" {
		start = 1
	}

	path := make([]string, 0, len(args)-start)
	for _, arg := range args[start:] {
		if isHelpFlag(arg) {
			continue
		}
		path = append(path, arg)
	}
	return path
}

func unknownHelpTopicError(topic string) error {
	return fmt.Errorf("unknown help topic %q; run `contract-cli help` to list available commands", topic)
}

func renderHelp(writer io.Writer, topic helpTopic) error {
	if _, err := fmt.Fprintf(writer, "Name:\n  %s\n", topic.Name); err != nil {
		return err
	}
	if topic.Summary != "" {
		if _, err := fmt.Fprintf(writer, "\n%s\n", topic.Summary); err != nil {
			return err
		}
	}
	if len(topic.Usage) > 0 {
		if _, err := fmt.Fprintln(writer, "\nUsage:"); err != nil {
			return err
		}
		for _, usage := range topic.Usage {
			if _, err := fmt.Fprintf(writer, "  %s\n", usage); err != nil {
				return err
			}
		}
	}
	if len(topic.Commands) > 0 {
		if _, err := fmt.Fprintln(writer, "\nCommands:"); err != nil {
			return err
		}
		if err := renderHelpCommands(writer, topic.Commands); err != nil {
			return err
		}
	}
	if len(topic.Flags) > 0 {
		if _, err := fmt.Fprintln(writer, "\nFlags:"); err != nil {
			return err
		}
		if err := renderHelpFlags(writer, topic.Flags); err != nil {
			return err
		}
	}
	if len(topic.Examples) > 0 {
		if _, err := fmt.Fprintln(writer, "\nExamples:"); err != nil {
			return err
		}
		for _, example := range topic.Examples {
			if _, err := fmt.Fprintf(writer, "  %s\n", example); err != nil {
				return err
			}
		}
	}
	if len(topic.Notes) > 0 {
		if _, err := fmt.Fprintln(writer, "\nNotes:"); err != nil {
			return err
		}
		for _, note := range topic.Notes {
			if _, err := fmt.Fprintf(writer, "  - %s\n", note); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintln(writer)
	return err
}

func renderHelpCommands(writer io.Writer, commands []helpCommand) error {
	table := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	for _, command := range commands {
		if _, err := fmt.Fprintf(table, "  %s\t%s\n", command.Name, command.Description); err != nil {
			return err
		}
	}
	return table.Flush()
}

func renderHelpFlags(writer io.Writer, flags []helpFlag) error {
	table := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	for _, flag := range flags {
		if _, err := fmt.Fprintf(table, "  %s\t%s\n", flag.Name, flag.Description); err != nil {
			return err
		}
	}
	return table.Flush()
}

/*
helpRegistry 构造本地命令帮助，环境入口始终只展示 prod。
无入参；返回 map[string]helpTopic 为命令路径到帮助主题的映射。
*/
func helpRegistry() map[string]helpTopic {
	topCommands := []helpCommand{
		{"contract-cli config add [flags]", "初始化或更新 profile"},
		{"contract-cli auth init [flags]", "发起 Device 用户授权"},
		{"contract-cli auth complete [flags]", "单次查询 Device 授权结果"},
		{"contract-cli auth login [flags]", "登录 user 或 app 身份"},
		{"contract-cli auth status [flags]", "查看授权状态"},
		{"contract-cli auth logout [flags]", "登出指定身份"},
		{"contract-cli auth use [flags]", "切换默认业务身份"},
		{"contract-cli version", "查看版本信息"},
		{"contract-cli skills list", "列出内置 Agent skills"},
		{"contract-cli skills install [flags]", "安装内置 Agent skills"},
		{"contract-cli update check [flags]", "检查 npm 远端版本"},
		{"contract-cli contract <subcommand> [flags]", "合同结构化命令"},
		{"contract-cli payment <subcommand> [flags]", "付款结构化命令"},
		{"contract-cli mdm vendor <subcommand> [flags]", "交易方主数据命令"},
		{"contract-cli mdm legal <subcommand> [flags]", "法人主体主数据命令"},
		{"contract-cli mdm fields list [flags]", "字段配置查询"},
		{"contract-cli mdm fixed-exchange-rate <subcommand> [flags]", "固定汇率命令"},
		{"contract-cli mdm file download <file-id> [flags]", "主数据附件下载"},
		{"contract-cli event outbound-ip list [flags]", "事件出口 IP 查询"},
		{"contract-cli rule table <subcommand> [flags]", "审批矩阵规则表命令"},
	}

	registry := map[string]helpTopic{
		"contract-cli": {
			Name:    "contract-cli",
			Summary: "合同开放平台命令行工具。",
			Usage: []string{
				"contract-cli <command> [flags]",
				"contract-cli help <command path>",
				"contract-cli <command path> --help",
			},
			Commands: topCommands,
			Notes: []string{
				"使用 `contract-cli <command> --help` 查看具体命令参数。",
				"JSON 请求体统一使用 --input-file 或 --data；真实文件上传使用 --file。",
			},
		},
		"version": {
			Name:    "version",
			Summary: "查看当前 CLI 版本、commit 和构建时间。",
			Usage:   []string{"contract-cli version", "contract-cli --version"},
			Examples: []string{
				"contract-cli version",
				"contract-cli --version",
			},
			Notes: []string{build.FeatureBaseline()},
		},
	}

	addConfigHelp(registry)
	addAuthHelp(registry)
	addSkillsHelp(registry)
	addUpdateHelp(registry)
	addContractHelp(registry)
	addPaymentHelp(registry)
	addMDMHelp(registry)
	addEventHelp(registry)
	addRuleHelp(registry)
	addRuleStructureHelp(registry)
	return registry
}

func addConfigHelp(registry map[string]helpTopic) {
	registry["config"] = helpTopic{
		Name:  "config",
		Usage: []string{"contract-cli config <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli config add [flags]", "初始化或更新 profile"},
		},
	}
	registry["config add"] = helpTopic{
		Name:    "config add",
		Summary: "初始化或更新 profile，并写入开放平台、user OAuth 和 app token endpoint 配置。",
		Usage:   []string{"contract-cli config add [flags]"},
		Flags: []helpFlag{
			{"--env <prod>", "环境预设；当前仅支持 prod，默认 prod"},
			{"--name <profile>", "profile 名称，默认 contract"},
			{"--resource-metadata-url <url>", "覆盖 protected resource metadata 地址"},
			{"--redirect-url <url>", "覆盖 OAuth callback 地址"},
			{"--scope <scopes>", "覆盖默认 OAuth scopes，多个 scope 用空格分隔"},
		},
		Examples: []string{
			"contract-cli config add --env prod --name contract",
		},
	}
}

func addAuthHelp(registry map[string]helpTopic) {
	registry["auth"] = helpTopic{
		Name:  "auth",
		Usage: []string{"contract-cli auth <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli auth init [flags]", "发起 Device 用户授权"},
			{"contract-cli auth complete [flags]", "单次查询 Device 授权结果"},
			{"contract-cli auth login [flags]", "登录 user 或 app 身份"},
			{"contract-cli auth status [flags]", "查看授权状态"},
			{"contract-cli auth logout [flags]", "登出指定身份"},
			{"contract-cli auth use [flags]", "切换默认业务身份"},
		},
	}
	registry["auth init"] = helpTopic{
		Name:    "auth init",
		Summary: "发起 Device Grant，输出已包含一次性用户码的完整 HTTPS 链接、二维码路径、qr_code_data_uri、机器时间和北京时间展示值。",
		Usage:   []string{"contract-cli auth init [flags]"},
		Flags: []helpFlag{
			{"--profile <name>", "profile 名称；不传使用当前 profile"},
			{"--output <json>", "固定为 json"},
			{"--restart", "仅在用户明确同意重新授权后替换旧 Device 会话"},
		},
		Examples: []string{
			"contract-cli auth init --profile contract --output json",
			"contract-cli auth init --profile contract --output json --restart",
		},
		Notes: []string{"Device 授权不要求用户手工输入授权码。"},
	}
	registry["auth complete"] = helpTopic{
		Name:    "auth complete",
		Summary: "单次查询 Device 授权结果，不在 CLI 内持续轮询。",
		Usage:   []string{"contract-cli auth complete [flags]"},
		Flags: []helpFlag{
			{"--profile <name>", "profile 名称；不传使用当前 profile"},
			{"--output <json>", "固定为 json"},
		},
		Examples: []string{"contract-cli auth complete --profile contract --output json"},
	}
	registry["auth login"] = helpTopic{
		Name:    "auth login",
		Summary: "登录 user 或 app 身份。user 走 OAuth 授权，app 使用 appId/appSecret 换取 tenant_access_token。",
		Usage:   []string{"contract-cli auth login [flags]"},
		Flags: []helpFlag{
			{"--profile <name>", "profile 名称；不传使用当前 profile"},
			{"--as <user|app>", "登录身份，默认 user"},
			{"--timeout <duration>", "user OAuth 等待时间，默认 3m"},
			{"--no-open-browser", "只打印授权 URL，不自动打开浏览器"},
			{"--app-id <id>", "app id；app 登录时可通过 flag/env/secrets 提供"},
			{"--app-secret <secret>", "app secret；不会输出到日志"},
		},
		Examples: []string{
			"contract-cli auth login --profile contract --as user",
			"contract-cli auth login --profile contract --as app --app-id <id> --app-secret <secret>",
		},
		Notes: []string{
			"app 凭证优先级：flag > env > 已保存 secrets。",
			"app 登录成功后保存 token，并把 default_identity 切到 app。",
			"兼容旧身份值 --as bot，运行时按 app 处理；新脚本建议使用 --as app。",
		},
	}
	registry["auth status"] = helpTopic{
		Name:    "auth status",
		Summary: "查看某个 profile 的 user 或 app 身份状态。",
		Usage:   []string{"contract-cli auth status [flags]"},
		Flags: []helpFlag{
			{"--profile <name>", "profile 名称；不传使用当前 profile"},
			{"--as <user|app>", "查看身份，默认 user"},
		},
		Examples: []string{
			"contract-cli auth status --profile contract --as user",
			"contract-cli auth status --profile contract --as app",
		},
	}
	registry["auth logout"] = helpTopic{
		Name:    "auth logout",
		Summary: "清理指定身份的 token。",
		Usage:   []string{"contract-cli auth logout [flags]"},
		Flags: []helpFlag{
			{"--profile <name>", "profile 名称；不传使用当前 profile"},
			{"--as <user|app>", "登出身份，默认 user"},
		},
		Examples: []string{
			"contract-cli auth logout --profile contract --as user",
			"contract-cli auth logout --profile contract --as app",
		},
		Notes: []string{
			"app logout 只清空 app token，保留 appId/appSecret。",
		},
	}
	registry["auth use"] = helpTopic{
		Name:    "auth use",
		Summary: "切换 profile 默认业务身份。",
		Usage:   []string{"contract-cli auth use [flags]"},
		Flags: []helpFlag{
			{"--profile <name>", "profile 名称；不传使用当前 profile"},
			{"--as <user|app>", "默认业务身份，默认 user"},
		},
		Examples: []string{
			"contract-cli auth use --profile contract --as app",
		},
	}
}

/*
addSkillsHelp 注册内置 Skill 命令及安装参数的帮助主题。
入参 registry（map[string]helpTopic）为待填充的帮助索引；返回值为空。
*/
func addSkillsHelp(registry map[string]helpTopic) {
	registry["skills"] = helpTopic{
		Name:  "skills",
		Usage: []string{"contract-cli skills <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli skills list", "列出内置 Agent skills"},
			{"contract-cli skills install [flags]", "安装内置 Agent skills"},
		},
		Notes: []string{
			"推荐跨平台安装方式：npx skills add qfeius/contract-cli -y -g。",
		},
	}
	registry["skills list"] = helpTopic{
		Name:    "skills list",
		Summary: "列出当前二进制内置的 Agent skills。",
		Usage:   []string{"contract-cli skills list"},
		Examples: []string{
			"contract-cli skills list",
		},
	}
	registry["skills install"] = helpTopic{
		Name:    "skills install",
		Summary: "把当前二进制内置 skills 安装到本机 Codex skills 目录。",
		Usage:   []string{"contract-cli skills install [flags]"},
		Flags: []helpFlag{
			{"--target <dir>", "安装目标目录；默认 $CODEX_HOME/skills 或 ~/.codex/skills"},
			{"--name <skill>", "只安装指定的内置 skill；默认安装全部"},
			{"--force", "覆盖已存在的同名 skill；默认跳过"},
		},
		Examples: []string{
			"contract-cli skills install",
			"contract-cli skills install --target ~/.codex/skills",
			"contract-cli skills install --name contract-cli-rule --force",
			"contract-cli skills install --force",
		},
	}
}

func addUpdateHelp(registry map[string]helpTopic) {
	registry["update"] = helpTopic{
		Name:  "update",
		Usage: []string{"contract-cli update <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli update check [flags]", "检查 npm 远端版本"},
		},
	}
	registry["update check"] = helpTopic{
		Name:    "update check",
		Summary: "检查 npm 远端是否存在可升级版本。",
		Usage:   []string{"contract-cli update check [flags]"},
		Flags: []helpFlag{
			{"--channel <latest|beta>", "npm dist-tag；正式版通常使用 latest，不传时根据当前版本推断"},
			{"--json", "输出飞书式结构化 JSON；默认输出文本提示"},
		},
		Examples: []string{
			"contract-cli update check",
			"contract-cli update check --channel latest --json",
		},
		Notes: []string{
			"普通命令按 24 小时缓存检查远端版本，并在 JSON object 输出中注入 _notice.update。",
			"可设置 CONTRACT_CLI_NO_UPDATE_CHECK=1 关闭自动检查。",
		},
	}
}

func addAPIHelp(registry map[string]helpTopic) {
	registry["api"] = helpTopic{
		Name:  "api",
		Usage: []string{"contract-cli api <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli api call <METHOD> <PATH> [flags]", "原始开放平台接口调用"},
		},
	}
	registry["api call"] = helpTopic{
		Name:    "api call",
		Summary: "对开放平台任意相对路径发起原始调用。",
		Usage:   []string{"contract-cli api call <METHOD> <PATH> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags(), []helpFlag{
			{"--header \"Key: Value\"", "追加 HTTP header，可重复传入"},
		}),
		Examples: []string{
			"contract-cli api call GET /open-apis/contract/v1/mcp/config/config_list --profile contract --as user",
			"contract-cli api call POST /open-apis/mdm/v1/vendors --profile contract --as app --data '{\"foo\":\"bar\"}'",
		},
		Notes: []string{
			"PATH 必须是相对路径，且以 /open-apis/ 开头。",
			"/open-apis/contract/v1/mcp/... 会被视为 user-only，显式 --as app 会报错。",
		},
	}
}

func addContractHelp(registry map[string]helpTopic) {
	registry["contract"] = helpTopic{
		Name:  "contract",
		Usage: []string{"contract-cli contract <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract search [flags]", "搜索合同"},
			{"contract-cli contract search-v2 [flags]", "app 身份搜索合同 V2"},
			{"contract-cli contract get <contract-id> [flags]", "获取合同详情"},
			{"contract-cli contract sync-user-groups [flags]", "同步用户分组"},
			{"contract-cli contract text <contract-id> [flags]", "获取合同文本"},
			{"contract-cli contract create [flags]", "创建合同"},
			{"contract-cli contract field update [flags]", "app 身份更新合同字段信息"},
			{"contract-cli contract sign switch-to-paper [flags]", "app 身份将电子签合同转纸质签"},
			{"contract-cli contract sign-url get <contract-id> [flags]", "app 身份获取签署链接"},
			{"contract-cli contract form attribute list [flags]", "app 身份获取合同流程字段"},
			{"contract-cli contract authorization grant [flags]", "app 身份授予合同权限"},
			{"contract-cli contract esign <subcommand> [flags]", "app 身份获取电子签认证授权链接"},
			{"contract-cli contract upload-file [flags]", "上传合同文件"},
			{"contract-cli contract submit <contract-id> [flags]", "app 身份提交合同"},
			{"contract-cli contract resubmit <contract-id> [flags]", "app 身份重新提交合同"},
			{"contract-cli contract patch <contract-id> [flags]", "app 身份更新合同"},
			{"contract-cli contract download-file <file-id> [flags]", "app 身份下载合同相关文件"},
			{"contract-cli contract delete <contract-id> [flags]", "app 身份删除草稿合同"},
			{"contract-cli contract print-file [flags]", "app 身份生成合同打印文件"},
			{"contract-cli contract share <subcommand> [flags]", "app 身份查询合同分享记录"},
			{"contract-cli contract cooperation <resource> <subcommand> [flags]", "app 身份查询合同协商信息"},
			{"contract-cli contract approval <subcommand> [flags]", "app 身份操作审批流程"},
			{"contract-cli contract category list [flags]", "列出合同分类"},
			{"contract-cli contract template <subcommand> [flags]", "模板相关命令"},
			{"contract-cli contract enum list [flags]", "查询枚举值"},
		},
	}
	registry["contract search"] = helpTopic{
		Name:    "contract search",
		Summary: "搜索合同，按当前身份自动路由 user MCP 或 app 开放平台接口。",
		Usage:   []string{"contract-cli contract search [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags(), pageFlags(), []helpFlag{
			{"--contract-number <number>", "按合同编号搜索，会合并进 JSON 请求体"},
		}),
		Examples: []string{
			"contract-cli contract search --profile contract --as user --input-file search.json",
			"contract-cli contract search --profile contract --as app --data '{\"contract_number\":\"CN-001\"}'",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/contracts/search",
			"app: /open-apis/contract/v1/contracts/search",
			"--input-file / --data 可选；查询 flag 会合并进 JSON body。",
		},
	}
	registry["contract search-v2"] = helpTopic{
		Name:    "contract search-v2",
		Summary: "app 身份搜索合同 V2，请求体必须是 JSON。",
		Usage:   []string{"contract-cli contract search-v2 --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract search-v2 --profile contract --as app --input-file search-v2.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/contracts/searchV2。",
			"复杂条件如 combine_condition、logic_search 直接放入 JSON body。",
		},
	}
	registry["contract get"] = helpTopic{
		Name:    "contract get",
		Summary: "获取合同详情，按当前身份自动路由。",
		Usage:   []string{"contract-cli contract get <contract-id> [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli contract get <contract-id> --profile contract --as user",
			"contract-cli contract get <contract-id> --profile contract --as app --user-id-type employee_id",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/contracts/{contract_id}",
			"app: /open-apis/contract/v1/contracts/{contract_id}",
		},
	}
	registry["contract sync-user-groups"] = helpTopic{
		Name:    "contract sync-user-groups",
		Summary: "同步合同用户分组。",
		Usage:   []string{"contract-cli contract sync-user-groups [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli contract sync-user-groups --profile contract --as user",
			"contract-cli contract sync-user-groups --profile contract --as app --user-id ou_xxx",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/contracts/user-groups/sync",
			"app: /open-apis/contract/v1/contracts/user-groups/sync",
		},
	}
	registry["contract text"] = helpTopic{
		Name:    "contract text",
		Summary: "获取合同文本。",
		Usage:   []string{"contract-cli contract text <contract-id> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--full-text", "获取完整文本"},
			{"--offset <n>", "文本偏移量"},
			{"--limit <n>", "文本长度限制"},
		}),
		Examples: []string{
			"contract-cli contract text <contract-id> --profile contract --as user --full-text",
			"contract-cli contract text <contract-id> --profile contract --as app --offset 0 --limit 1000",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/contracts/{contract_id}/text",
			"app: /open-apis/contract/v1/contracts/{contract_id}/text",
		},
	}
	registry["contract create"] = helpTopic{
		Name:    "contract create",
		Summary: "创建合同，请求体必须是 JSON object。",
		Usage:   []string{"contract-cli contract create --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract create --profile contract --input-file create.json",
			"contract-cli contract create --profile contract --as app --data '{\"title\":\"demo\",\"create_user_id\":\"ou_xxx\"}'",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/contracts",
			"app: /open-apis/contract/v1/contracts",
			"app 创建合同时，请调用方自行在 JSON body 中提供 create_user_id。",
		},
	}
	registry["contract upload-file"] = helpTopic{
		Name:    "contract upload-file",
		Summary: "上传合同相关文件，返回后端原始 JSON，重点关注 data.file_id。",
		Usage:   []string{"contract-cli contract upload-file --file <path> --file-type <type> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--file <path>", "必填，本地待上传文件路径"},
			{"--file-type <type>", "必填，文件类型，例如 text、attachment、scan"},
			{"--file-name <name>", "可选，上传给后端的文件名；默认使用本地文件名"},
		}),
		Examples: []string{
			"contract-cli contract upload-file --profile contract --as user --file ./合同正文.docx --file-type text",
			"contract-cli contract upload-file --profile contract --as app --file ./附件.pdf --file-type attachment --file-name 附件.pdf",
		},
		Notes: []string{
			"user/app 均走 POST /open-apis/contract/v1/files/upload。",
			"请求使用 multipart/form-data，字段为 file_name、file_type、file。",
			"本地文件必须存在、是普通文件，大小 <= 200MB。",
			"不接受 --input-file / --data；这两个参数只用于 JSON 请求体。",
		},
	}
	registry["contract submit"] = helpTopic{
		Name:    "contract submit",
		Summary: "app 身份提交合同。",
		Usage:   []string{"contract-cli contract submit <contract-id> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract submit <contract-id> --profile contract --as app",
			"contract-cli contract submit <contract-id> --profile contract --as app --data '{\"comment\":\"ok\"}'",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/contracts/{contract_id}/submit。",
			"--input-file / --data 可选；不传时不发送请求体。",
		},
	}
	registry["contract resubmit"] = helpTopic{
		Name:    "contract resubmit",
		Summary: "app 身份重新提交合同。",
		Usage:   []string{"contract-cli contract resubmit <contract-id> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract resubmit <contract-id> --profile contract --as app",
			"contract-cli contract resubmit <contract-id> --profile contract --as app --input-file resubmit.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/contracts/{contract_id}/resubmit。",
			"--input-file / --data 可选；不传时不发送请求体。",
		},
	}
	registry["contract patch"] = helpTopic{
		Name:    "contract patch",
		Summary: "app 身份更新合同，请求体必须是 JSON。",
		Usage:   []string{"contract-cli contract patch <contract-id> --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract patch <contract-id> --profile contract --as app --input-file patch.json",
			"contract-cli contract patch <contract-id> --profile contract --as app --data '{\"title\":\"demo\"}'",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 PATCH /open-apis/contract/v1/contracts/{contract_id}。",
			"--input-file / --data 必填且互斥。",
		},
	}
	registry["contract download-file"] = helpTopic{
		Name:    "contract download-file",
		Summary: "app 身份下载合同相关文件。",
		Usage:   []string{"contract-cli contract download-file <file-id> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--output-file <path>", "保存到指定文件；不传时默认拉起保存文件弹窗"},
			{"--force", "覆盖已存在的 --output-file"},
		}),
		Examples: []string{
			"contract-cli contract download-file <file-id> --profile contract --as app",
			"contract-cli contract download-file <file-id> --profile contract --as app --output-file ./contract.pdf",
			"contract-cli contract download-file <file-id> --profile contract --as app --raw > contract.pdf",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/files/{file_id}。",
			"默认拉起保存文件弹窗；无 GUI/远程/CI 环境推荐显式传 --output-file。",
			"--raw 会把文件内容写到 stdout，不打印额外提示。",
			"不实现 dowload-file 拼写别名。",
		},
	}
	registry["contract delete"] = helpTopic{
		Name:    "contract delete",
		Summary: "app 身份删除草稿合同。",
		Usage:   []string{"contract-cli contract delete <contract-id> [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli contract delete <contract-id> --profile contract --as app",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 DELETE /open-apis/contract/v1/contracts/{contract_id}。",
			"命令直接删除，不额外要求 --yes。",
		},
	}
	registry["contract print-file"] = helpTopic{
		Name:    "contract print-file",
		Summary: "app 身份生成合同打印文件，请求体必须是 JSON。",
		Usage:   []string{"contract-cli contract print-file --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract print-file --profile contract --as app --input-file print-file.json",
			"contract-cli contract print-file --profile contract --as app --data '{\"contract_id\":\"<contract-id>\"}'",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/files。",
			"--input-file / --data 必填且互斥。",
		},
	}
	registry["contract field"] = helpTopic{
		Name:  "contract field",
		Usage: []string{"contract-cli contract field <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract field update [flags]", "app 身份更新合同字段信息"},
		},
	}
	registry["contract field update"] = helpTopic{
		Name:    "contract field update",
		Summary: "app 身份更新合同字段信息，目前主要用于修改下拉列表选项范围。",
		Usage:   []string{"contract-cli contract field update --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract field update --profile contract --as app --input-file field-update.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 PUT /open-apis/contract/v1/attribute_definition。",
			"最小 body 通常包含 module_name、attribute_name 和 value_scopes。",
		},
	}
	registry["contract sign"] = helpTopic{
		Name:  "contract sign",
		Usage: []string{"contract-cli contract sign <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract sign switch-to-paper [flags]", "app 身份将电子签合同转纸质签"},
		},
	}
	registry["contract sign switch-to-paper"] = helpTopic{
		Name:    "contract sign switch-to-paper",
		Summary: "app 身份将电子签合同转为纸质签。",
		Usage:   []string{"contract-cli contract sign switch-to-paper --business-id <contract-id> --business-type-code <code> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--business-id <contract-id>", "必填，合同 ID"},
			{"--business-type-code <code>", "必填，业务类型编码：0 合同申请、2 合同变更、3 合同终止"},
		}),
		Examples: []string{
			"contract-cli contract sign switch-to-paper --profile contract --as app --business-id <contract-id> --business-type-code 0",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/contracts/signType/switchToPaper。",
			"不接受 --input-file / --data。",
		},
	}
	registry["contract sign-url"] = helpTopic{
		Name:  "contract sign-url",
		Usage: []string{"contract-cli contract sign-url <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract sign-url get <contract-id> [flags]", "app 身份获取签署链接"},
		},
	}
	registry["contract sign-url get"] = helpTopic{
		Name:    "contract sign-url get",
		Summary: "app 身份获取合同签署链接。",
		Usage:   []string{"contract-cli contract sign-url get <contract-id> [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli contract sign-url get <contract-id> --profile contract --as app",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/{contract_id}/sign_url。",
			"不接受 --input-file / --data。",
		},
	}
	registry["contract form"] = helpTopic{
		Name:  "contract form",
		Usage: []string{"contract-cli contract form <resource> <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract form attribute list [flags]", "app 身份获取合同流程字段"},
		},
	}
	registry["contract form attribute"] = helpTopic{
		Name:  "contract form attribute",
		Usage: []string{"contract-cli contract form attribute <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract form attribute list [flags]", "app 身份获取合同流程字段"},
		},
	}
	registry["contract form attribute list"] = helpTopic{
		Name:    "contract form attribute list",
		Summary: "app 身份按合同类型和流程类型获取流程字段。",
		Usage:   []string{"contract-cli contract form attribute list --category-id <category-id> --business-type-code <code> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--category-id <category-id>", "必填，合同类型 ID"},
			{"--business-type-code <code>", "必填，流程类型：0 申请、1 变更、2 终止、3 合同组申请"},
		}),
		Examples: []string{
			"contract-cli contract form attribute list --profile contract --as app --category-id <category-id> --business-type-code 0",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/form_definition/attribute。",
			"不接受 --input-file / --data。",
		},
	}
	registry["contract authorization"] = helpTopic{
		Name:  "contract authorization",
		Usage: []string{"contract-cli contract authorization <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract authorization grant [flags]", "app 身份授予合同权限"},
		},
	}
	registry["contract authorization grant"] = helpTopic{
		Name:    "contract authorization grant",
		Summary: "app 身份授予合同权限，请求体必须是 JSON。",
		Usage:   []string{"contract-cli contract authorization grant --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract authorization grant --profile contract --as app --input-file authorization.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/authorizations。",
			"最小 body 通常包含 business_id、authorized_user_id、start_time、end_time。",
		},
	}
	registry["contract esign"] = helpTopic{
		Name:  "contract esign",
		Usage: []string{"contract-cli contract esign <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract esign personal-auth-url [flags]", "app 身份获取个人认证授权页面链接"},
			{"contract-cli contract esign org-auth-url [flags]", "app 身份获取机构认证授权页面链接"},
		},
	}
	registry["contract esign personal-auth-url"] = helpTopic{
		Name:    "contract esign personal-auth-url",
		Summary: "app 身份获取个人认证和授权页面链接。",
		Usage:   []string{"contract-cli contract esign personal-auth-url --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract esign personal-auth-url --profile contract --as app --input-file psn-auth-url.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/esign/auth/psnAuthUrl。",
		},
	}
	registry["contract esign org-auth-url"] = helpTopic{
		Name:    "contract esign org-auth-url",
		Summary: "app 身份获取机构认证和授权页面链接。",
		Usage:   []string{"contract-cli contract esign org-auth-url --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract esign org-auth-url --profile contract --as app --input-file org-auth-url.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/esign/auth/orgAuthUrl。",
		},
	}
	registry["contract share"] = helpTopic{
		Name:  "contract share",
		Usage: []string{"contract-cli contract share <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract share get <contract-id> [flags]", "app 身份查询合同分享记录"},
			{"contract-cli contract share batch-create [flags]", "app 身份批量分享合同"},
		},
	}
	registry["contract share batch-create"] = helpTopic{
		Name:    "contract share batch-create",
		Summary: "app 身份批量分享合同，请求体必须是 JSON。",
		Usage:   []string{"contract-cli contract share batch-create --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract share batch-create --profile contract --as app --input-file batch-share.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/contracts/contract/batch_share。",
			"最小 body 通常包含 contract_id 和 user_ids。",
		},
	}
	registry["contract share get"] = helpTopic{
		Name:    "contract share get",
		Summary: "app 身份查询合同分享记录。",
		Usage:   []string{"contract-cli contract share get <contract-id> [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli contract share get <contract-id> --profile contract --as app",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/{contract_id}/share_records。",
		},
	}
	registry["contract cooperation"] = helpTopic{
		Name:  "contract cooperation",
		Usage: []string{"contract-cli contract cooperation <resource> <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract cooperation link get <contract-id> [flags]", "app 身份查询合同协商邀请链接"},
			{"contract-cli contract cooperation record get <contract-id> [flags]", "app 身份查询合同协商操作记录"},
			{"contract-cli contract cooperation search [flags]", "app 身份查询协商列表"},
			{"contract-cli contract cooperation file get <contract-id> [flags]", "app 身份查询合同协商文件信息"},
			{"contract-cli contract cooperation file download <file-id> [flags]", "app 身份下载合同协商文件"},
		},
	}
	registry["contract cooperation link get"] = helpTopic{
		Name:    "contract cooperation link get",
		Summary: "app 身份查询合同协商邀请链接。",
		Usage:   []string{"contract-cli contract cooperation link get <contract-id> [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli contract cooperation link get <contract-id> --profile contract --as app",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_link。",
		},
	}
	registry["contract cooperation record get"] = helpTopic{
		Name:    "contract cooperation record get",
		Summary: "app 身份查询合同协商操作记录信息。",
		Usage:   []string{"contract-cli contract cooperation record get <contract-id> [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli contract cooperation record get <contract-id> --profile contract --as app",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/{contract_id}/cooperation_record_info。",
		},
	}
	registry["contract cooperation search"] = helpTopic{
		Name:    "contract cooperation search",
		Summary: "app 身份查询协商列表，请求体必须是 JSON。",
		Usage:   []string{"contract-cli contract cooperation search --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract cooperation search --profile contract --as app --input-file cooperation-search.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/cooperation/search。",
			"最小 body 需要包含 user_id。",
		},
	}
	registry["contract cooperation file"] = helpTopic{
		Name:  "contract cooperation file",
		Usage: []string{"contract-cli contract cooperation file <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract cooperation file get <contract-id> [flags]", "app 身份查询合同协商文件信息"},
			{"contract-cli contract cooperation file download <file-id> [flags]", "app 身份下载合同协商文件"},
		},
	}
	registry["contract cooperation file get"] = helpTopic{
		Name:    "contract cooperation file get",
		Summary: "app 身份查询合同协商文件信息。",
		Usage:   []string{"contract-cli contract cooperation file get <contract-id> [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli contract cooperation file get <contract-id> --profile contract --as app",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/{contract_id}/cooperation/file_info。",
			"不接受 --input-file / --data。",
		},
	}
	registry["contract cooperation file download"] = helpTopic{
		Name:    "contract cooperation file download",
		Summary: "app 身份下载合同协商文件。",
		Usage:   []string{"contract-cli contract cooperation file download <file-id> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--output-file <path>", "保存到指定文件；不传时默认拉起保存文件弹窗"},
			{"--force", "覆盖已存在的 --output-file"},
		}),
		Examples: []string{
			"contract-cli contract cooperation file download <file-id> --profile contract --as app --output-file ./cooperation.docx",
			"contract-cli contract cooperation file download <file-id> --profile contract --as app --raw > cooperation.docx",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/cooperation/{file_id}/download_file。",
			"--raw 会把文件内容写到 stdout，不打印额外提示。",
		},
	}
	registry["contract approval"] = helpTopic{
		Name:  "contract approval",
		Usage: []string{"contract-cli contract approval <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract approval start <process-instance-id> [flags]", "app 身份发起流程审批"},
			{"contract-cli contract approval get <process-instance-id> [flags]", "app 身份查询审批实例详情"},
		},
	}
	registry["contract approval start"] = helpTopic{
		Name:    "contract approval start",
		Summary: "app 身份发起流程审批，请求体必须是 JSON。",
		Usage:   []string{"contract-cli contract approval start <process-instance-id> --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract approval start <process-instance-id> --profile contract --as app --input-file approval.json",
			"contract-cli contract approval start <process-instance-id> --profile contract --as app --data '{\"task_instance_id\":\"task-1\",\"command_type\":\"general\"}'",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/process_instances/{process_instance_id}/task_approval。",
			"--input-file / --data 必填且互斥。",
		},
	}
	registry["contract approval get"] = helpTopic{
		Name:    "contract approval get",
		Summary: "app 身份查询审批实例详情。",
		Usage:   []string{"contract-cli contract approval get <process-instance-id> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--notice-filter <filter>", "可选，审批实例详情查询参数 notice_filter"},
			{"--task-instance-filter <filter>", "可选，审批实例详情查询参数 task_instance_filter"},
		}),
		Examples: []string{
			"contract-cli contract approval get <process-instance-id> --profile contract --as app",
			"contract-cli contract approval get <process-instance-id> --profile contract --as app --notice-filter notice_filter --task-instance-filter task_instance_filter",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/process_instances/{process_instance_id}。",
			"不接受 --input-file / --data。",
		},
	}
	addContractNestedHelp(registry)
}

func addPaymentHelp(registry map[string]helpTopic) {
	registry["payment"] = helpTopic{
		Name:  "payment",
		Usage: []string{"contract-cli payment <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli payment create [flags]", "app 身份创建付款申请"},
			{"contract-cli payment update <payment-id> [flags]", "app 身份更新付款信息"},
			{"contract-cli payment get <payment-id> [flags]", "app 身份查看付款信息"},
			{"contract-cli payment list [flags]", "app 身份查询付款申请列表"},
			{"contract-cli payment plan <subcommand> [flags]", "app 身份操作付款计划"},
			{"contract-cli payment record <subcommand> [flags]", "app 身份操作付款记录"},
		},
	}
	registry["payment create"] = helpTopic{
		Name:    "payment create",
		Summary: "app 身份创建付款申请，请求体必须是 JSON。",
		Usage:   []string{"contract-cli payment create --contract <contract-id> --input-file <path>|--data <json> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags(), []helpFlag{
			{"--contract <contract-id>", "必填，父级合同 ID"},
		}),
		Examples: []string{
			"contract-cli payment create --contract <contract-id> --profile contract --as app --input-file payment.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/contracts/{contract_id}/payments。",
			"--input-file / --data 必填且互斥。",
		},
	}
	registry["payment update"] = helpTopic{
		Name:    "payment update",
		Summary: "app 身份更新付款信息，请求体必须是 JSON。",
		Usage:   []string{"contract-cli payment update <payment-id> --contract <contract-id> --input-file <path>|--data <json> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags(), []helpFlag{
			{"--contract <contract-id>", "必填，父级合同 ID"},
		}),
		Examples: []string{
			"contract-cli payment update <payment-id> --contract <contract-id> --profile contract --as app --input-file payment-update.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 PATCH /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}。",
			"--input-file / --data 必填且互斥。",
		},
	}
	registry["payment get"] = helpTopic{
		Name:    "payment get",
		Summary: "app 身份查看付款信息。",
		Usage:   []string{"contract-cli payment get <payment-id> --contract <contract-id> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--contract <contract-id>", "必填，父级合同 ID"},
		}),
		Examples: []string{
			"contract-cli payment get <payment-id> --contract <contract-id> --profile contract --as app",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}。",
			"不接受 --input-file / --data。",
		},
	}
	registry["payment list"] = helpTopic{
		Name:    "payment list",
		Summary: "app 身份查询付款申请列表。",
		Usage:   []string{"contract-cli payment list --contract <contract-id> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), pageFlags(), []helpFlag{
			{"--contract <contract-id>", "必填，父级合同 ID"},
		}),
		Examples: []string{
			"contract-cli payment list --contract <contract-id> --profile contract --as app",
			"contract-cli payment list --contract <contract-id> --profile contract --as app --page-size 10 --page-token next",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/{contract_id}/payments。",
			"不接受 --input-file / --data。",
		},
	}
	registry["payment plan"] = helpTopic{
		Name:  "payment plan",
		Usage: []string{"contract-cli payment plan <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli payment plan notify [flags]", "app 身份同步付款记录"},
			{"contract-cli payment plan search [flags]", "app 身份搜索付款计划"},
		},
	}
	registry["payment plan notify"] = helpTopic{
		Name:    "payment plan notify",
		Summary: "app 身份同步付款记录，请求体必须是 JSON。",
		Usage:   []string{"contract-cli payment plan notify --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli payment plan notify --profile contract --as app --input-file notify.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/payment/notify。",
			"--input-file / --data 必填且互斥。",
		},
	}
	registry["payment plan search"] = helpTopic{
		Name:    "payment plan search",
		Summary: "app 身份搜索付款计划，请求体必须是 JSON。",
		Usage:   []string{"contract-cli payment plan search --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli payment plan search --profile contract --as app --input-file payment-plan-search.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/payments/search。",
			"--input-file / --data 必填且互斥。",
		},
	}
	registry["payment record"] = helpTopic{
		Name:  "payment record",
		Usage: []string{"contract-cli payment record <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli payment record create [flags]", "app 身份创建付款记录"},
			{"contract-cli payment record update <payment-record-id> [flags]", "app 身份更新付款记录"},
			{"contract-cli payment record get <payment-record-id> [flags]", "app 身份查询付款记录详情"},
			{"contract-cli payment record list [flags]", "app 身份按付款计划查询付款记录"},
		},
	}
	registry["payment record create"] = helpTopic{
		Name:    "payment record create",
		Summary: "app 身份创建付款记录，请求体必须是 JSON。",
		Usage:   []string{"contract-cli payment record create --contract <contract-id> --payment <payment-id> --input-file <path>|--data <json> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags(), []helpFlag{
			{"--contract <contract-id>", "必填，父级合同 ID"},
			{"--payment <payment-id>", "必填，父级付款 ID"},
		}),
		Examples: []string{
			"contract-cli payment record create --contract <contract-id> --payment <payment-id> --profile contract --as app --input-file payment-record.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 POST /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}/payment_records。",
			"--input-file / --data 必填且互斥。",
		},
	}
	registry["payment record update"] = helpTopic{
		Name:    "payment record update",
		Summary: "app 身份更新付款记录，请求体必须是 JSON。",
		Usage:   []string{"contract-cli payment record update <payment-record-id> --contract <contract-id> --payment <payment-id> --input-file <path>|--data <json> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags(), []helpFlag{
			{"--contract <contract-id>", "必填，父级合同 ID"},
			{"--payment <payment-id>", "必填，父级付款 ID"},
		}),
		Examples: []string{
			"contract-cli payment record update <payment-record-id> --contract <contract-id> --payment <payment-id> --profile contract --as app --input-file payment-record-update.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 PATCH /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}/payment_records/{payment_record_id}。",
			"--input-file / --data 必填且互斥。",
		},
	}
	registry["payment record get"] = helpTopic{
		Name:    "payment record get",
		Summary: "app 身份查询付款记录详情。",
		Usage:   []string{"contract-cli payment record get <payment-record-id> --contract <contract-id> --payment <payment-id> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--contract <contract-id>", "必填，父级合同 ID"},
			{"--payment <payment-id>", "必填，父级付款 ID"},
		}),
		Examples: []string{
			"contract-cli payment record get <payment-record-id> --contract <contract-id> --payment <payment-id> --profile contract --as app",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/{contract_id}/payments/{payment_id}/payment_records/{payment_record_id}。",
			"不接受 --input-file / --data。",
		},
	}
	registry["payment record list"] = helpTopic{
		Name:    "payment record list",
		Summary: "app 身份根据付款计划 ID 查询付款记录。",
		Usage:   []string{"contract-cli payment record list --plan <payment-plan-uuid> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--plan <payment-plan-uuid>", "必填，付款计划 UUID"},
		}),
		Examples: []string{
			"contract-cli payment record list --plan <payment-plan-uuid> --profile contract --as app",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/contract/v1/contracts/payments/{payment_plan_uuid}/payment_records。",
			"不接受 --input-file / --data。",
		},
	}
}

func addContractNestedHelp(registry map[string]helpTopic) {
	registry["contract category"] = helpTopic{
		Name:  "contract category",
		Usage: []string{"contract-cli contract category <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract category list [flags]", "列出合同分类"},
		},
	}
	registry["contract category list"] = helpTopic{
		Name:    "contract category list",
		Summary: "列出合同分类。",
		Usage:   []string{"contract-cli contract category list [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--lang <lang>", "语言，例如 zh-CN"},
		}),
		Examples: []string{
			"contract-cli contract category list --profile contract",
			"contract-cli contract category list --profile contract --as app --lang zh-CN",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/contract_categorys",
			"app: /open-apis/contract/v1/contract_categorys",
		},
	}
	registry["contract template"] = helpTopic{
		Name:  "contract template",
		Usage: []string{"contract-cli contract template <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract template list [flags]", "列出模板"},
			{"contract-cli contract template get <template-id> [flags]", "获取模板详情"},
			{"contract-cli contract template instantiate [flags]", "创建模板实例"},
		},
	}
	registry["contract template list"] = helpTopic{
		Name:    "contract template list",
		Summary: "列出模板。",
		Usage:   []string{"contract-cli contract template list [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), pageFlags(), []helpFlag{
			{"--category-number <number>", "合同分类编号"},
		}),
		Examples: []string{
			"contract-cli contract template list --profile contract",
			"contract-cli contract template list --profile contract --as app --category-number CAT-1 --page-size 20",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/templates",
			"app: /open-apis/contract/v1/templates",
		},
	}
	registry["contract template get"] = helpTopic{
		Name:    "contract template get",
		Summary: "获取模板详情。",
		Usage:   []string{"contract-cli contract template get <template-id> [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli contract template get <template-id> --profile contract",
			"contract-cli contract template get <template-id> --profile contract --as app --user-id ou_xxx",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/templates/{template_id}",
			"app: /open-apis/contract/v1/templates/{template_id}",
		},
	}
	registry["contract template instantiate"] = helpTopic{
		Name:    "contract template instantiate",
		Summary: "创建模板实例，请求体必须是 JSON object。",
		Usage:   []string{"contract-cli contract template instantiate --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli contract template instantiate --profile contract --input-file template-instance.json",
			"contract-cli contract template instantiate --profile contract --as app --data '{\"template_number\":\"TMP001\",\"create_user_id\":\"ou_xxx\"}'",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/template_instances",
			"app: /open-apis/contract/v1/template_instances",
			"app 创建模板实例时，请调用方自行在 JSON body 中提供 create_user_id。",
		},
	}
	registry["contract enum"] = helpTopic{
		Name:  "contract enum",
		Usage: []string{"contract-cli contract enum <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli contract enum list --type <enum-type> [flags]", "查询枚举值"},
		},
	}
	registry["contract enum list"] = helpTopic{
		Name:    "contract enum list",
		Summary: "查询枚举值。",
		Usage:   []string{"contract-cli contract enum list --type <enum-type> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--type <enum-type>", "必填，枚举类型"},
		}),
		Examples: []string{
			"contract-cli contract enum list --profile contract --type contract_status",
		},
		Notes: []string{
			"仅支持 --as user；走 /open-apis/contract/v1/mcp/enum_values。",
		},
	}
}

func addMDMHelp(registry map[string]helpTopic) {
	registry["mdm"] = helpTopic{
		Name:  "mdm",
		Usage: []string{"contract-cli mdm <resource> <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli mdm vendor <subcommand> [flags]", "交易方主数据"},
			{"contract-cli mdm legal <subcommand> [flags]", "法人主体主数据"},
			{"contract-cli mdm fields list [flags]", "字段配置查询"},
			{"contract-cli mdm fixed-exchange-rate <subcommand> [flags]", "固定汇率"},
			{"contract-cli mdm file download <file-id> [flags]", "主数据附件下载"},
		},
	}
	registry["mdm vendor"] = helpTopic{
		Name:  "mdm vendor",
		Usage: []string{"contract-cli mdm vendor <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli mdm vendor create [flags]", "app 身份创建交易方"},
			{"contract-cli mdm vendor update <vendor-id> [flags]", "app 身份更新交易方"},
			{"contract-cli mdm vendor list [flags]", "查询交易方列表"},
			{"contract-cli mdm vendor get <vendor-id> [flags]", "查询交易方详情"},
			{"contract-cli mdm vendor list-all [flags]", "app 身份查询交易方全量数据"},
			{"contract-cli mdm vendor query-by-cert [flags]", "app 身份根据证件 ID 精确查询交易方"},
		},
	}
	registry["mdm vendor create"] = helpTopic{
		Name:    "mdm vendor create",
		Summary: "app 身份创建交易方，请求体必须是 JSON。",
		Usage:   []string{"contract-cli mdm vendor create --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli mdm vendor create --profile contract --as app --user-id <operator-user-id> --input-file vendor-create.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"写接口必传 --user-id，用于提供当前操作人上下文。",
			"走 POST /open-apis/mdm/v1/vendors。",
			"创建请求体不要传后端生成的 vendor 编码。",
			"交易方字段是否必填受后台动态配置影响，可先查 mdm fields list --biz-line vendor。",
		},
	}
	registry["mdm vendor update"] = helpTopic{
		Name:    "mdm vendor update",
		Summary: "app 身份按 ID 更新交易方，请求体必须是 JSON。",
		Usage:   []string{"contract-cli mdm vendor update <vendor-id> --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli mdm vendor update <vendor-id> --profile contract --as app --user-id <operator-user-id> --input-file vendor-update.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"写接口必传 --user-id，用于提供当前操作人上下文。",
			"走 PUT /open-apis/mdm/v1/vendors/{vendor_id}。",
			"更新请求体必须包含后端返回的 id 和 vendor 编码。",
			"其他字段是否必填受后台动态配置影响。",
		},
	}
	registry["mdm vendor list"] = helpTopic{
		Name:    "mdm vendor list",
		Summary: "查询交易方列表。",
		Usage:   []string{"contract-cli mdm vendor list [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), listQueryFlags()),
		Examples: []string{
			"contract-cli mdm vendor list --profile contract --name 供应商 --page-size 10",
			"contract-cli mdm vendor list --profile contract --as app --name V00000001 --user-id-type employee_id",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/vendors",
			"app: /open-apis/mdm/v1/vendors",
			"--name 会映射到底层 query vendor。",
		},
	}
	registry["mdm vendor get"] = helpTopic{
		Name:    "mdm vendor get",
		Summary: "查询交易方详情。",
		Usage:   []string{"contract-cli mdm vendor get <vendor-id> [flags]"},
		Flags:   openPlatformCommonFlags(),
		Examples: []string{
			"contract-cli mdm vendor get <vendor-id> --profile contract",
			"contract-cli mdm vendor get <vendor-id> --profile contract --as app --user-id-type employee_id",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/vendors/{vendor_id}",
			"app: /open-apis/mdm/v1/vendors/{vendor_id}",
		},
	}
	registry["mdm vendor list-all"] = helpTopic{
		Name:    "mdm vendor list-all",
		Summary: "app 身份分页查询交易方全量数据。",
		Usage:   []string{"contract-cli mdm vendor list-all [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), pageFlags()),
		Examples: []string{
			"contract-cli mdm vendor list-all --profile contract --as app --page-size 10",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/mdm/v1/vendors/list_all。",
			"不接受 --input-file / --data。",
		},
	}
	registry["mdm vendor query-by-cert"] = helpTopic{
		Name:    "mdm vendor query-by-cert",
		Summary: "app 身份根据证件 ID 和国家地区精确查询交易方。",
		Usage:   []string{"contract-cli mdm vendor query-by-cert --certification-id <id> --ad-country <country> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--certification-id <id>", "必填，证件 ID"},
			{"--ad-country <country>", "必填，国家地区编码"},
		}),
		Examples: []string{
			"contract-cli mdm vendor query-by-cert --profile contract --as app --certification-id 91110105 --ad-country CN",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/mdm/v1/vendors/query_vendors。",
			"不接受 --input-file / --data。",
		},
	}
	registry["mdm legal"] = helpTopic{
		Name:  "mdm legal",
		Usage: []string{"contract-cli mdm legal <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli mdm legal create [flags]", "app 身份创建法人主体"},
			{"contract-cli mdm legal update <legal-entity-id> [flags]", "app 身份更新法人主体"},
			{"contract-cli mdm legal list [flags]", "查询法人主体列表"},
			{"contract-cli mdm legal get <legal-entity-id>|--code <code> [flags]", "查询法人主体详情或按编码查询"},
		},
	}
	registry["mdm legal create"] = helpTopic{
		Name:    "mdm legal create",
		Summary: "app 身份创建法人主体，请求体必须是 JSON。",
		Usage:   []string{"contract-cli mdm legal create --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli mdm legal create --profile contract --as app --user-id <operator-user-id> --input-file legal-create.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"写接口必传 --user-id，用于提供当前操作人上下文。",
			"走 POST /open-apis/mdm/v1/legal_entities。",
			"创建请求体不要传后端生成的 legalEntity / legal_entity 编码。",
			"法人字段是否必填受后台动态配置影响，可先查 mdm fields list --biz-line legal_entity。",
		},
	}
	registry["mdm legal update"] = helpTopic{
		Name:    "mdm legal update",
		Summary: "app 身份按 ID 更新法人主体，请求体必须是 JSON。",
		Usage:   []string{"contract-cli mdm legal update <legal-entity-id> --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli mdm legal update <legal-entity-id> --profile contract --as app --user-id <operator-user-id> --input-file legal-update.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"写接口必传 --user-id，用于提供当前操作人上下文。",
			"走 PUT /open-apis/mdm/v1/legal_entities/{legal_entity_id}。",
			"更新请求体必须包含后端返回的 id 和 legalEntity 编码；字段名使用 camelCase legalEntity，不要用 legal_entity。",
			"其他字段是否必填受后台动态配置影响。",
		},
	}
	registry["mdm legal list"] = helpTopic{
		Name:    "mdm legal list",
		Summary: "查询法人主体列表。",
		Usage:   []string{"contract-cli mdm legal list [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), listQueryFlags()),
		Examples: []string{
			"contract-cli mdm legal list --profile contract --name 主体A --page-size 10",
			"contract-cli mdm legal list --profile contract --as app --user-id-type employee_id",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/legal_entities",
			"app: /open-apis/mdm/v1/legal_entities/list_all",
			"--name 会映射到底层 query legalEntity。",
		},
	}
	registry["mdm legal get"] = helpTopic{
		Name:    "mdm legal get",
		Summary: "查询法人主体详情；传 --code 时按法人实体编码查询。",
		Usage: []string{
			"contract-cli mdm legal get <legal-entity-id> [flags]",
			"contract-cli mdm legal get --code <code> [flags]",
		},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--code <code>", "法人实体编码；传入后走编码查询接口"},
			{"--page-size <n>", "仅 --code 模式可用，分页大小"},
			{"--page-token <token>", "仅 --code 模式可用，分页 token"},
		}),
		Examples: []string{
			"contract-cli mdm legal get <legal-entity-id> --profile contract",
			"contract-cli mdm legal get <legal-entity-id> --profile contract --as app --user-id-type employee_id",
			"contract-cli mdm legal get --profile contract --as app --code L0001 --page-size 10",
		},
		Notes: []string{
			"按 ID 查询时 user: /open-apis/contract/v1/mcp/legal_entities/{legal_entity_id}",
			"按 ID 查询时 app: /open-apis/mdm/v1/legal_entities/{legal_entity_id}，并额外透传同名 query legal_entity_id",
			"按编码查询仅支持 app，走 GET /open-apis/mdm/v1/legal_entities，--code 映射 query legalEntity。",
			"按编码查询不接受 --input-file / --data。",
		},
	}
	registry["mdm fields"] = helpTopic{
		Name:  "mdm fields",
		Usage: []string{"contract-cli mdm fields <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli mdm fields list --biz-line <biz-line> [flags]", "查询字段配置"},
		},
	}
	registry["mdm fields list"] = helpTopic{
		Name:    "mdm fields list",
		Summary: "查询字段配置。",
		Usage:   []string{"contract-cli mdm fields list --biz-line <biz-line> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--biz-line <biz-line>", "必填；user 支持 vendor、legal_entity、vendor_risk；app 支持 vendor、legalEntity，legal_entity 会自动映射为 legalEntity"},
		}),
		Examples: []string{
			"contract-cli mdm fields list --profile contract --biz-line vendor",
			"contract-cli mdm fields list --profile contract --as app --biz-line legal_entity",
			"contract-cli mdm fields list --profile contract --as app --biz-line vendor --user-id-type employee_id",
		},
		Notes: []string{
			"user: /open-apis/contract/v1/mcp/config/config_list",
			"app: /open-apis/mdm/v1/config/config_list",
			"app 后端当前只接受 vendor 或 legalEntity；CLI 会把 app 下的 legal_entity 映射为 legalEntity，vendor_risk 不支持 app。",
		},
	}
	registry["mdm fixed-exchange-rate"] = helpTopic{
		Name:  "mdm fixed-exchange-rate",
		Usage: []string{"contract-cli mdm fixed-exchange-rate <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli mdm fixed-exchange-rate get [flags]", "app 身份查询固定汇率"},
			{"contract-cli mdm fixed-exchange-rate update [flags]", "app 身份更新固定汇率"},
		},
	}
	registry["mdm fixed-exchange-rate get"] = helpTopic{
		Name:    "mdm fixed-exchange-rate get",
		Summary: "app 身份按原始币种、目标币种、生效日期查询固定汇率。",
		Usage:   []string{"contract-cli mdm fixed-exchange-rate get --source-currency <code> --target-currency <code> --effective-date <date> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--source-currency <code>", "必填，原始币种"},
			{"--target-currency <code>", "必填，目标币种"},
			{"--effective-date <date>", "必填，生效日期"},
		}),
		Examples: []string{
			"contract-cli mdm fixed-exchange-rate get --profile contract --as app --source-currency CNY --target-currency USD --effective-date 2026-06-01",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/mdm/v1/fixed_exchange_rate。",
			"CLI flag --effective-date 会映射到底层 query 参数 date。",
			"不接受 --input-file / --data。",
		},
	}
	registry["mdm fixed-exchange-rate update"] = helpTopic{
		Name:    "mdm fixed-exchange-rate update",
		Summary: "app 身份新增或更新固定汇率，请求体必须是 JSON。",
		Usage:   []string{"contract-cli mdm fixed-exchange-rate update --input-file <path>|--data <json> [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), jsonBodyFlags()),
		Examples: []string{
			"contract-cli mdm fixed-exchange-rate update --profile contract --as app --input-file fixed-exchange-rate.json",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 PUT /open-apis/mdm/v1/fixed_exchange_rate。",
		},
	}
	registry["mdm file"] = helpTopic{
		Name:  "mdm file",
		Usage: []string{"contract-cli mdm file <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli mdm file download <file-id> [flags]", "app 身份下载主数据附件"},
		},
	}
	registry["mdm file download"] = helpTopic{
		Name:    "mdm file download",
		Summary: "app 身份下载主数据附件。",
		Usage:   []string{"contract-cli mdm file download <file-id> [flags]"},
		Flags: concatHelpFlags(openPlatformCommonFlags(), []helpFlag{
			{"--output-file <path>", "保存到指定文件；不传时默认拉起保存文件弹窗"},
			{"--force", "覆盖已存在的 --output-file"},
		}),
		Examples: []string{
			"contract-cli mdm file download <file-id> --profile contract --as app --output-file ./attachment.bin",
			"contract-cli mdm file download <file-id> --profile contract --as app --raw > attachment.bin",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/mdm/v1/file/download/{file_id}。",
			"--raw 会把文件内容写到 stdout，不打印额外提示。",
		},
	}
}

func addEventHelp(registry map[string]helpTopic) {
	registry["event"] = helpTopic{
		Name:  "event",
		Usage: []string{"contract-cli event <resource> <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli event outbound-ip list [flags]", "app 身份查询事件出口 IP"},
		},
	}
	registry["event outbound-ip"] = helpTopic{
		Name:  "event outbound-ip",
		Usage: []string{"contract-cli event outbound-ip <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli event outbound-ip list [flags]", "app 身份查询事件出口 IP"},
		},
	}
	registry["event outbound-ip list"] = helpTopic{
		Name:    "event outbound-ip list",
		Summary: "app 身份分页查询开放平台事件出口 IP。",
		Usage:   []string{"contract-cli event outbound-ip list [flags]"},
		Flags:   concatHelpFlags(openPlatformCommonFlags(), eventOutboundIPPageFlags()),
		Examples: []string{
			"contract-cli event outbound-ip list --profile contract --as app --page-size 10",
		},
		Notes: []string{
			"app-only: 当前仅支持 --as app。",
			"走 GET /open-apis/event/v1/outbound_ip。",
			"不接受 --input-file / --data。",
		},
	}
}

/*
addRuleHelp 注册审批矩阵原子命令与批量导入命令的纯本地帮助主题。
入参 registry（map[string]helpTopic）为帮助主题注册表，函数会就地追加 rule table 相关条目。
返回值为空。
*/
func addRuleHelp(registry map[string]helpTopic) {
	registry["rule"] = helpTopic{
		Name:  "rule",
		Usage: []string{"contract-cli rule <resource> <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli rule table <subcommand> [flags]", "user/app 身份操作审批矩阵规则表"},
		},
		Notes: []string{build.FeatureBaseline()},
	}
	registry["rule table"] = helpTopic{
		Name:  "rule table",
		Usage: []string{"contract-cli rule table <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli rule table list [flags]", "查询规则表列表"},
			{"contract-cli rule table pre-release [flags]", "预发布规则表配置"},
			{"contract-cli rule table release [flags]", "发布规则表配置"},
			{"contract-cli rule table column-headers list [flags]", "查询规则表列头"},
			{"contract-cli rule table row <subcommand> [flags]", "操作规则表行"},
			{"contract-cli rule table import <subcommand> [flags]", "生成并执行批量行导入计划"},
		},
	}
	registry["rule table list"] = ruleTableHelpTopic(
		"rule table list",
		"分页查询规则表列表。",
		"contract-cli rule table list --product-id <id> --group-id <id> [flags]",
		pageFlags(),
		[]string{"contract-cli rule table list --profile contract --as app --product-id <product-id> --group-id <group-id> --page-size 10"},
		[]string{"走 GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables。", "--page-size 必填，范围 1-100；不接受 --input-file / --data。"},
	)
	registry["rule table pre-release"] = ruleTableHelpTopic(
		"rule table pre-release",
		"全量读取规则行建立本地基准，再预发布并回读核验。",
		"contract-cli rule table pre-release --product-id <id> --group-id <id> --table-id <id> [flags]",
		concatHelpFlags(tableIDHelpFlags(), jsonBodyFlags()),
		[]string{"contract-cli rule table pre-release --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>"},
		[]string{"草稿状态走 PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/pre_release。", "服务端已为 status=0 待发布状态时，双读版本与规则行后只恢复本地基准并返回 baseline_recovered=true，不重复预发布、不改写规则行。", "--input-file / --data 可选；自动化样例不发送请求体。"},
	)
	registry["rule table release"] = ruleTableHelpTopic(
		"rule table release",
		"要求本地预发布基准；核对版本和全量规则行无变化后发布，异常时停止并要求重新预发布确认。",
		"contract-cli rule table release --product-id <id> --group-id <id> --table-id <id> [flags]",
		concatHelpFlags(tableIDHelpFlags(), jsonBodyFlags()),
		[]string{"contract-cli rule table release --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>"},
		[]string{"走 PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/release。", "本地基准丢失时先执行 pre-release；服务端仍待发布会只读恢复基准并返回 baseline_recovered=true，不改写规则行。", "--input-file / --data 可选；自动化样例不发送请求体。"},
	)
	registry["rule table column-headers"] = helpTopic{
		Name:  "rule table column-headers",
		Usage: []string{"contract-cli rule table column-headers <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli rule table column-headers list [flags]", "查询规则表列头"},
		},
	}
	registry["rule table column-headers list"] = ruleTableHelpTopic(
		"rule table column-headers list",
		"查询规则表列头信息。",
		"contract-cli rule table column-headers list --product-id <id> --group-id <id> --table-id <id> [flags]",
		tableIDHelpFlags(),
		[]string{"contract-cli rule table column-headers list --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>"},
		[]string{"走 GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_columns/column_headers。", "不接受 --input-file / --data。"},
	)
	registry["rule table row"] = helpTopic{
		Name:  "rule table row",
		Usage: []string{"contract-cli rule table row <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli rule table row create [flags]", "创建规则表行"},
			{"contract-cli rule table row get <row-id> [flags]", "查询规则表单行"},
			{"contract-cli rule table row list [flags]", "分页查询规则表行"},
			{"contract-cli rule table row search [flags]", "按筛选条件查询规则表行"},
			{"contract-cli rule table row update <row-id> [flags]", "修改规则表行"},
			{"contract-cli rule table row delete <row-id> [flags]", "删除规则表行"},
		},
	}
	registry["rule table row create"] = ruleTableHelpTopic("rule table row create", "创建规则表行，请求体必须是 JSON。", "contract-cli rule table row create --product-id <id> --group-id <id> --table-id <id> --input-file <path>|--data <json> [flags]", concatHelpFlags(tableIDHelpFlags(), jsonBodyFlags()), []string{"contract-cli rule table row create --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --input-file row.json"}, []string{"走 POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows。", "DEPARTMENT_COLLECTION 使用 open_department_id 字符串（od-...）；sys_department.id 数字会在本地拒绝。"})
	registry["rule table row get"] = ruleTableHelpTopic("rule table row get", "查询规则表单行信息。", "contract-cli rule table row get <row-id> --product-id <id> --group-id <id> --table-id <id> [flags]", tableIDHelpFlags(), []string{"contract-cli rule table row get <row-id> --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>"}, []string{"走 GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/{table_row_id}。", "不接受 --input-file / --data。"})
	registry["rule table row list"] = ruleTableHelpTopic("rule table row list", "分页查询规则表行。", "contract-cli rule table row list --product-id <id> --group-id <id> --table-id <id> [flags]", concatHelpFlags(tableIDHelpFlags(), pageFlags()), []string{"contract-cli rule table row list --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --page-size 10"}, []string{"走 GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows。", "不接受 --input-file / --data。"})
	registry["rule table row search"] = ruleTableHelpTopic("rule table row search", "按筛选条件分页查询规则表行，请求体必须是 JSON。", "contract-cli rule table row search --product-id <id> --group-id <id> --table-id <id> --input-file <path>|--data <json> [flags]", concatHelpFlags(tableIDHelpFlags(), pageFlags(), jsonBodyFlags()), []string{"contract-cli rule table row search --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --page-size 10 --input-file row-search.json"}, []string{"走 POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/search。", "--page-size 必填且范围为 1-100；--page-size / --page-token 作为 query 参数传递，请求体保持不变。", "DEPARTMENT_COLLECTION 使用 open_department_id 字符串（od-...）；sys_department.id 数字会在本地拒绝。"})
	registry["rule table row update"] = ruleTableHelpTopic("rule table row update", "修改规则表行，请求体必须是 JSON。", "contract-cli rule table row update <row-id> --product-id <id> --group-id <id> --table-id <id> --input-file <path>|--data <json> [flags]", concatHelpFlags(tableIDHelpFlags(), jsonBodyFlags()), []string{"contract-cli rule table row update <row-id> --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id> --input-file row.json"}, []string{"走 PUT /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/{table_row_id}。", "DEPARTMENT_COLLECTION 使用 open_department_id 字符串（od-...）；sys_department.id 数字会在本地拒绝。"})
	registry["rule table row delete"] = ruleTableHelpTopic("rule table row delete", "删除规则表行。", "contract-cli rule table row delete <row-id> --product-id <id> --group-id <id> --table-id <id> [flags]", tableIDHelpFlags(), []string{"contract-cli rule table row delete <row-id> --profile contract --as app --product-id <product-id> --group-id <group-id> --table-id <table-id>"}, []string{"走 DELETE /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{rule_table_id}/table_rows/{table_row_id}。", "不接受 --input-file / --data。"})
	registry["rule table import"] = helpTopic{
		Name:  "rule table import",
		Usage: []string{"contract-cli rule table import <subcommand> [flags]"},
		Commands: []helpCommand{
			{"contract-cli rule table import plan [flags]", "校验输入并生成不写数据的导入计划"},
			{"contract-cli rule table import apply [flags]", "按计划逐行创建或更新矩阵行"},
		},
	}
	registry["rule table import plan"] = ruleTableHelpTopic(
		"rule table import plan",
		"读取当前列头和全量规则行，将多行输入转换为待确认计划并校验 2000 行上限；本命令不写矩阵数据。",
		"contract-cli rule table import plan --product-id <id> --group-id <id> [--table-id <id>] --input-file <path>|--data <json> [flags]",
		concatHelpFlags(approvalMatrixImportPlanHelpFlags(), jsonBodyFlags()),
		[]string{"contract-cli rule table import plan --profile contract --as app --product-id contract --group-id approve_matrix --table-id <table-id> --input-file import.json"},
		[]string{"输入顶层为 rows；每行包含 operation(create|update)、update 所需的 row_id，以及以列名或列 ID 为键的 cells。", "未传 --table-id 时返回 status=needs_input 和候选矩阵；成功返回 status=needs_confirmation 和 plan_id。", "缺列、重名列、类型错误、目标行不存在或现有行加新增行超过 2000 时返回 status=needs_input。", "不支持 --raw。"},
	)
	registry["rule table import apply"] = helpTopic{
		Name:    "rule table import apply",
		Summary: "使用 plan_id 串行执行；写入成功后逐行回读，重试跳过已验证成功的行。",
		Usage:   []string{"contract-cli rule table import apply --plan-id <id> [flags]"},
		Flags:   concatHelpFlags(ruleOpenPlatformCommonFlags(), []helpFlag{{"--plan-id <id>", "必填，plan 命令返回的确认令牌"}}),
		Examples: []string{
			"contract-cli rule table import apply --profile contract --as app --plan-id <plan-id>",
		},
		Notes: []string{
			"支持 --as user / --as app；user 需具备合同规则管理权限。",
			"返回 success、paused、needs_verification、partial_success、failed、invalidated 或 needs_input，并附逐行结果。",
			"结果不确定的写入不会自动重试，请先用行查询命令核对。",
			"不接受 --input-file / --data，也不支持 --raw。",
		},
	}
}

func ruleTableHelpTopic(name string, summary string, usage string, extraFlags []helpFlag, examples []string, notes []string) helpTopic {
	return helpTopic{
		Name:    name,
		Summary: summary,
		Usage:   []string{usage},
		Flags: concatHelpFlags(ruleOpenPlatformCommonFlags(), []helpFlag{
			{"--product-id <id>", "必填，规则引擎产品 ID"},
			{"--group-id <id>", "必填，规则组 ID"},
		}, extraFlags),
		Examples: examples,
		Notes: append([]string{
			"支持 --as user / --as app；user 需具备合同规则管理权限。",
			"矩阵路径由开平服务端从当前用户令牌确定编辑人；--user-id 不会覆盖编辑人。",
		}, notes...),
	}
}

/* ruleOpenPlatformCommonFlags 返回矩阵命令的通用参数；user_id 只保留兼容解析，实际规则请求由当前令牌确定编辑人。 */
func ruleOpenPlatformCommonFlags() []helpFlag {
	flags := openPlatformCommonFlags()
	for index := range flags {
		if flags[index].Name == "--user-id <id>" {
			flags[index].Description = "兼容解析但不发送到矩阵路径；编辑人由当前用户令牌确定"
		}
	}
	return flags
}

func tableIDHelpFlags() []helpFlag {
	return []helpFlag{
		{"--table-id <id>", "必填，规则表 ID"},
	}
}

/*
approvalMatrixImportPlanHelpFlags 返回批量计划特有的目标矩阵参数说明。
入参为空。
返回值（[]helpFlag）为帮助渲染使用的 flag 列表；table-id 未传时命令会返回候选矩阵。
*/
func approvalMatrixImportPlanHelpFlags() []helpFlag {
	return []helpFlag{
		{"--table-id <id>", "目标规则表 ID；未传时返回候选矩阵"},
	}
}

func openPlatformCommonFlags() []helpFlag {
	return []helpFlag{
		{"--profile <name>", "profile 名称；不传使用当前 profile"},
		{"--as <user|app>", "请求身份；不传使用 profile default_identity；兼容旧值 bot"},
		{"--output <json|yaml|table>", "输出格式，默认 json"},
		{"--raw", "原样输出响应 body"},
		{"--user-id-type <type>", "通用 query 参数 user_id_type；不传默认 user_id，传了则覆盖默认值"},
		{"--user-id <id>", "通用 query 参数 user_id；传了就透传，不传就不带"},
	}
}

func jsonBodyFlags() []helpFlag {
	return []helpFlag{
		{"--input-file <path>", "从 JSON 文件读取请求体；与 --data 互斥"},
		{"--data <json>", "内联 JSON 请求体；与 --input-file 互斥"},
	}
}

func pageFlags() []helpFlag {
	return []helpFlag{
		{"--page-size <n>", "分页大小"},
		{"--page-token <token>", "分页 token"},
	}
}

func eventOutboundIPPageFlags() []helpFlag {
	return []helpFlag{
		{"--page-size <n>", "分页大小，10-50"},
		{"--page-token <token>", "分页 token"},
	}
}

func listQueryFlags() []helpFlag {
	return concatHelpFlags([]helpFlag{
		{"--name <name>", "名称或编码查询条件"},
	}, pageFlags())
}

func concatHelpFlags(groups ...[]helpFlag) []helpFlag {
	var total int
	for _, group := range groups {
		total += len(group)
	}
	flags := make([]helpFlag, 0, total)
	for _, group := range groups {
		flags = append(flags, group...)
	}
	return flags
}
