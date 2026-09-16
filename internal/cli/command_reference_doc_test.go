package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/*
TestCommandReferenceDocumentCoversCurrentSupportedCommands 验证命令参考覆盖当前所有公开结构化命令。
入参 t（*testing.T）为 Go 测试上下文。
返回值为空；失败通过 t.Fatalf 报告。
*/
func TestCommandReferenceDocumentCoversCurrentSupportedCommands(t *testing.T) {
	t.Parallel()

	docPath := filepath.Join("..", "..", "docs", "cli-command-reference.md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", docPath, err)
	}

	text := string(content)
	requiredFragments := []string{
		"# contract-cli 命令文档",
		"contract-cli --help",
		"contract-cli help contract upload-file",
		"contract-cli contract search --help",
		"contract-cli config add --env prod --name contract",
		"contract-cli config add",
		"contract-cli auth login",
		"contract-cli auth status",
		"contract-cli auth logout",
		"contract-cli auth use",
		"contract-cli version",
		"contract-cli skills list",
		"contract-cli skills install",
		"npx skills add qfeius/contract-cli -y -g",
		"api call 暂未开放使用",
		"contract-cli update check",
		"contract-cli contract search",
		"contract-cli contract search-v2",
		"contract-cli contract get",
		"contract-cli contract sync-user-groups",
		"contract-cli contract text",
		"contract-cli contract create",
		"contract-cli contract field update",
		"contract-cli contract sign switch-to-paper",
		"contract-cli contract sign-url get",
		"contract-cli contract form attribute list",
		"contract-cli contract authorization grant",
		"contract-cli contract esign personal-auth-url",
		"contract-cli contract esign org-auth-url",
		"contract-cli contract upload-file",
		"contract-cli contract submit",
		"contract-cli contract resubmit",
		"contract-cli contract patch",
		"contract-cli contract download-file",
		"contract-cli contract delete",
		"contract-cli contract print-file",
		"contract-cli contract share get",
		"contract-cli contract share batch-create",
		"contract-cli contract cooperation link get",
		"contract-cli contract cooperation record get",
		"contract-cli contract cooperation search",
		"contract-cli contract cooperation file get",
		"contract-cli contract cooperation file download",
		"contract-cli contract approval start",
		"contract-cli contract approval get",
		"contract-cli contract category list",
		"contract-cli contract template list",
		"contract-cli contract template get",
		"contract-cli contract template instantiate",
		"contract-cli contract enum list",
		"contract-cli payment create",
		"contract-cli payment update",
		"contract-cli payment get",
		"contract-cli payment list",
		"contract-cli payment plan notify",
		"contract-cli payment plan search",
		"contract-cli payment record create",
		"contract-cli payment record update",
		"contract-cli payment record get",
		"contract-cli payment record list",
		"contract-cli mdm vendor list",
		"contract-cli mdm vendor get",
		"contract-cli mdm vendor create",
		"contract-cli mdm vendor update",
		"contract-cli mdm vendor list-all",
		"contract-cli mdm vendor query-by-cert",
		"contract-cli mdm legal list",
		"contract-cli mdm legal get",
		"contract-cli mdm legal get --profile contract --as app --code",
		"contract-cli mdm legal create",
		"contract-cli mdm legal update",
		"contract-cli mdm fields list",
		"contract-cli mdm fixed-exchange-rate get",
		"contract-cli mdm fixed-exchange-rate update",
		"contract-cli mdm file download",
		"contract-cli event outbound-ip list",
		"contract-cli rule table list",
		"contract-cli rule table row create",
		"contract-cli rule table import plan",
		"contract-cli rule table import apply",
		"`contract get`、`contract search`、`contract create`、`contract sync-user-groups`、`contract text`、`contract category list`、`contract template list`、`contract template get`、`contract template instantiate`、`contract upload-file`、`mdm vendor list`、`mdm vendor get`、`mdm legal list`、`mdm legal get`、`mdm fields list` 是当前仅有的十五个同时支持 `user` 与 `app` 的结构化业务命令",
		"`contract search-v2`、`contract field update`、`contract sign switch-to-paper`、`contract sign-url get`",
		"`contract share get/batch-create`、`contract cooperation link/record/search/file`",
		"`mdm vendor create/update/list-all/query-by-cert`、`mdm legal get --code/create/update`、`mdm fixed-exchange-rate get/update`、`mdm file download`、`event outbound-ip list` 当前仅支持 `--as app`",
		"`--user-id-type`",
		"`--user-id`",
		"传了就拼接到 query string",
		"`mdm vendor create/update` 与 `mdm legal create/update`",
		"创建请求体不要包含后端生成的 `vendor` 编码",
		"请求体必须包含后端返回的 `id` 和 camelCase `legalEntity` 编码",
		"CLI flag `--effective-date` 会映射到底层 query 参数 `date`",
		"`--page-size` 可选，传入时必须在 `10` 到 `50` 之间",
		"`auth login --as app`",
		"旧身份值 `--as bot` 仍可使用，运行时等价于 `--as app`",
		"`contract text --as app` 走 `GET /open-apis/contract/v1/contracts/{contract_id}/text?...`",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(text, fragment) {
			t.Fatalf("command reference missing %q", fragment)
		}
	}
	if strings.Contains(text, "contract-cli api call GET") ||
		strings.Contains(text, "contract-cli api call POST") {
		t.Fatalf("command reference should not expose runnable api call examples")
	}
	for _, forbidden := range []string{
		"contract" + "-group",
		"/Users/lyy/",
		"当前只预置 `dev`",
		"`--env`：当前仅支持 `dev`",
		"支持 `prod` 和 `dev`",
		"contract-cli config add --env dev",
		"`contract text --as app` 走 `POST /open-apis/contract/v1/contracts/{contract_id}/text?...`",
		"除上述 app 能力外，当前其他结构化业务命令仍只支持 `--as user`",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("command reference should not contain production-stale fragment %q", forbidden)
		}
	}
}

func TestMergeConflictResolutionKeepsCommandDocsAndSkillsConsistent(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	docContent, err := os.ReadFile(filepath.Join(root, "docs", "cli-command-reference.md"))
	if err != nil {
		t.Fatalf("read command reference: %v", err)
	}
	docText := string(docContent)
	for _, heading := range []string{
		"#### `contract-cli contract approval start`",
		"#### `contract-cli contract approval get`",
	} {
		if count := strings.Count(docText, heading); count != 1 {
			t.Fatalf("command reference heading %q count = %d, want 1", heading, count)
		}
	}
	if strings.Contains(docText, "`rule table *`、`contract approval start/get` 和 `payment *`") {
		t.Fatal("command reference keeps duplicated app-only capability suffix from merge conflict")
	}

	skillContent, err := os.ReadFile(filepath.Join(root, "skills", "contract-cli-contract", "SKILL.md"))
	if err != nil {
		t.Fatalf("read contract skill: %v", err)
	}
	skillText := string(skillContent)
	approvalDecision := "- 想发起流程审批或查询审批实例：用 `contract approval start|get --as app`"
	if count := strings.Count(skillText, approvalDecision); count != 1 {
		t.Fatalf("contract skill approval decision count = %d, want 1", count)
	}
	for _, forbidden := range []string{
		"若需求是授权：读 [../contract-cli-payment/SKILL.md]",
		"`contract approval start`、`contract approval get`、`contract approval start`、`contract approval get`",
	} {
		if strings.Contains(skillText, forbidden) {
			t.Fatalf("contract skill keeps invalid merge fragment %q", forbidden)
		}
	}
}

func TestREADMECoversBundledSkillsAndNewAppOnlyCommands(t *testing.T) {
	t.Parallel()

	readmePath := filepath.Join("..", "..", "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", readmePath, err)
	}

	text := string(content)
	requiredFragments := []string{
		"`contract-cli-payment`",
		"`contract-cli-mdm-exchange`",
		"`contract-cli-mdm-file`",
		"`contract-cli-event`",
		"`contract-cli-rule`",
		"`contract search-v2`",
		"`contract authorization grant`",
		"`mdm vendor create/update/list-all/query-by-cert`",
		"`mdm legal get --code/create/update`",
		"`mdm fixed-exchange-rate get/update`",
		"`mdm file download`",
		"`event outbound-ip list`",
		"`mdm vendor create/update` 和 `mdm legal create/update` 会要求 `--user-id`",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(text, fragment) {
			t.Fatalf("README missing %q", fragment)
		}
	}
	for _, forbidden := range []string{
		"交易方候选列表与详情查询",
		"法人主体候选列表与详情查询",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("README keeps stale skill description %q", forbidden)
		}
	}
}

func TestP2UserFacingDocsUseAppIdentityNaming(t *testing.T) {
	t.Parallel()

	checks := map[string][]string{
		filepath.Join("..", "..", "docs", "cli-command-reference.md"): {
			"`bot` 目前已经支持登录、状态查看、登出、默认身份切换",
			"用途：bot 身份发起流程审批。",
			"用途：bot 身份查询审批实例详情。",
			"contract-cli contract approval start <process-instance-id> --profile contract --as bot",
			"contract-cli contract approval get <process-instance-id> --profile contract --as bot",
			"`payment` 这一组命令当前全部仅支持 `--as bot`",
			"contract-cli payment create --contract <contract-id> --profile contract --as bot",
			"contract-cli payment update <payment-id> --contract <contract-id> --profile contract --as bot",
			"contract-cli payment get <payment-id> --contract <contract-id> --profile contract --as bot",
			"contract-cli payment list --contract <contract-id> --profile contract --as bot",
			"contract-cli payment plan notify --profile contract --as bot",
			"contract-cli payment plan search --profile contract --as bot",
			"contract-cli payment record create --contract <contract-id> --payment <payment-id> --profile contract --as bot",
			"contract-cli payment record update <payment-record-id> --contract <contract-id> --payment <payment-id> --profile contract --as bot",
			"contract-cli payment record get <payment-record-id> --contract <contract-id> --payment <payment-id> --profile contract --as bot",
			"contract-cli payment record list --plan <payment-plan-uuid> --profile contract --as bot",
			"当前仅支持 `--as bot`。",
		},
		filepath.Join("..", "..", "docs", "cli-test-plan.md"): {
			"当前这一组命令均为 bot-only",
			"contract-cli payment create --contract \"$CONTRACT_ID\" --profile \"$PROFILE\" --as bot",
			"contract-cli payment plan notify --profile \"$PROFILE\" --as bot",
			"contract-cli contract approval start \"$PROCESS_INSTANCE_ID\" --profile \"$PROFILE\" --as bot",
			"only supports --as bot",
		},
		filepath.Join("..", "..", "docs", "cli-p2-提示词.md"): {
			"新增命令默认按 bot 身份开放",
			"bot-only",
		},
		filepath.Join("..", "..", "skills", "contract-cli-payment", "SKILL.md"): {
			"bot 身份",
			"`--as bot`",
			"bot 登录",
		},
		filepath.Join("..", "..", "skills", "contract-cli-payment", "agents", "openai.yaml"): {
			"bot-only",
			"bot-authorized",
		},
		filepath.Join("..", "..", "skills", "contract-cli-payment", "references", "commands.md"): {
			"--as bot",
		},
		filepath.Join("..", "..", "skills", "contract-cli-contract", "references", "commands.md"): {
			"--as bot",
			"bot 身份",
		},
		filepath.Join("..", "..", "skills", "contract-cli-shared", "SKILL.md"): {
			"payment ... --as bot",
		},
	}

	for path, forbiddenFragments := range checks {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		text := string(content)
		for _, forbidden := range forbiddenFragments {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s should use app identity naming, found %q", path, forbidden)
			}
		}
	}
}
