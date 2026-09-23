package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAPIGapSkillsCoverNewCommands(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	testCases := []struct {
		path      string
		fragments []string
	}{
		{
			path: filepath.Join(root, "skills", "contract-cli-contract", "references", "openapi-gap-commands.md"),
			fragments: []string{
				"contract-cli contract search-v2",
				"contract-cli contract field update",
				"contract-cli contract cooperation file download",
				"--force",
			},
		},
		{
			path: filepath.Join(root, "skills", "contract-cli-mdm-vendor", "SKILL.md"),
			fragments: []string{
				"contract-cli mdm vendor create",
				"contract-cli mdm vendor update <vendor-id>",
				"contract-cli mdm vendor patch <vendor-id>",
				"contract-cli mdm vendor enable <vendor-id>",
				"contract-cli mdm vendor disable <vendor-id>",
				"mdm vendor query-by-cert",
				"app 身份执行 `create/update/patch` 必须传 `--user-id`",
				"create 请求体不要包含后端生成的 `vendor` 编码",
				"update 请求体必须包含后端返回的 `id` 和 `vendor` 编码",
			},
		},
		{
			path: filepath.Join(root, "skills", "contract-cli-mdm-legal", "SKILL.md"),
			fragments: []string{
				"contract-cli mdm legal create",
				"contract-cli mdm legal update <legal-entity-id>",
				"contract-cli mdm legal get --code",
				"mdm fields list --biz-line legal_entity",
				"mdm legal create/update` 必须传 `--user-id`",
				"请求体不要包含后端生成的 `legalEntity` / `legal_entity` 编码",
				"字段名使用 camelCase `legalEntity`",
			},
		},
		{
			path: filepath.Join(root, "skills", "contract-cli-mdm-exchange", "SKILL.md"),
			fragments: []string{
				"contract-cli mdm fixed-exchange-rate get",
				"contract-cli mdm fixed-exchange-rate update",
				"/open-apis/mdm/v1/fixed_exchange_rate",
				"`--effective-date` 会映射到底层 query `date`",
			},
		},
		{
			path: filepath.Join(root, "skills", "contract-cli-mdm-file", "SKILL.md"),
			fragments: []string{
				"contract-cli mdm file download",
				"/open-apis/mdm/v1/file/download/{file_id}",
				"--force",
			},
		},
		{
			path: filepath.Join(root, "skills", "contract-cli-event", "SKILL.md"),
			fragments: []string{
				"contract-cli event outbound-ip list",
				"/open-apis/event/v1/outbound_ip",
				"`--page-size` 必须在 `10` 到 `50` 之间",
			},
		},
		{
			path: filepath.Join(root, "skills", "contract-cli-rule", "SKILL.md"),
			fragments: []string{
				"contract-cli rule table list",
				"contract-cli rule table row create",
				"`--page-size` / `--page-token` 作为 query 参数",
				"/open-apis/rule_engine/v1",
			},
		},
		{
			path: filepath.Join(root, "skills", "contract-cli-shared", "SKILL.md"),
			fragments: []string{
				"contract-cli-mdm-exchange",
				"contract-cli-event",
				"contract-cli-rule",
				"mdm legal list/get/create/update",
				"rule table *",
				"例外：`mdm vendor create/update/patch --as app` 与 `mdm legal create/update` 写接口会本地要求 `--user-id`",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			content := readTextFile(t, tc.path)
			for _, fragment := range tc.fragments {
				if !strings.Contains(content, fragment) {
					t.Fatalf("%s missing %q", tc.path, fragment)
				}
			}
		})
	}
}

func TestMDMSkillsNoLongerMarkCreateUpdateAsMissing(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	for _, path := range []string{
		filepath.Join(root, "skills", "contract-cli-mdm-vendor", "SKILL.md"),
		filepath.Join(root, "skills", "contract-cli-mdm-legal", "SKILL.md"),
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			content := readTextFile(t, path)
			if strings.Contains(content, "create/update` 当成已有结构化命令") ||
				strings.Contains(content, "结构化命令未实现") {
				t.Fatalf("%s still says create/update is missing", path)
			}
		})
	}
}

func TestOpenAPIGapSkillsDoNotKeepObsoleteCoverageClaims(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	testCases := []struct {
		path      string
		forbidden []string
		required  []string
	}{
		{
			path: filepath.Join(root, "skills", "contract-cli-shared", "SKILL.md"),
			forbidden: []string{
				"当前结构化命令里只有",
			},
			required: []string{
				"同时支持 `user` 与 `app` 的结构化业务命令",
				"app-only 命令包括",
				"`contract search-v2`",
				"`contract authorization grant`",
				"`mdm vendor update/list-all/query-by-cert`",
				"`mdm legal get --code/create/update`",
				"`mdm fixed-exchange-rate get/update`",
				"`mdm file download`",
				"`event outbound-ip list` 和 `rule table *`",
				"例外：`mdm vendor create/update/patch --as app` 与 `mdm legal create/update` 写接口会本地要求 `--user-id`",
			},
		},
		{
			path: filepath.Join(root, "skills", "contract-cli-contract", "SKILL.md"),
			forbidden: []string{
				"若需求是审批、授权、付款：当前 skill 不覆盖",
				"若需求是授权：当前 skill 不覆盖",
			},
			required: []string{
				"想发起旧版流程审批：用 `contract approval start --as app`",
				"想查询审批实例：用 `contract approval get --as user|app`",
				"想授予合同权限：用 `contract authorization grant --as app`",
			},
		},
		{
			path: filepath.Join(root, "skills", "contract-cli-mdm-fields", "SKILL.md"),
			forbidden: []string{
				"等待对应结构化写命令开放",
			},
			required: []string{
				"需要写交易方时，改读",
				"需要写法人实体时，改读",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			content := readTextFile(t, tc.path)
			for _, forbidden := range tc.forbidden {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s keeps obsolete claim %q", tc.path, forbidden)
				}
			}
			for _, required := range tc.required {
				if !strings.Contains(content, required) {
					t.Fatalf("%s missing updated claim %q", tc.path, required)
				}
			}
		})
	}
}

func TestContractSkillReferencesStayAlignedWithOpenAPIGapCommands(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	commands := readTextFile(t, filepath.Join(root, "skills", "contract-cli-contract", "references", "commands.md"))
	for _, required := range []string{
		"contract-cli contract search-v2 --profile contract --as app --input-file search-v2.json",
		"contract-cli contract field update --profile contract --as app --input-file field-update.json",
		"contract-cli contract authorization grant --profile contract --as app --input-file authorization.json",
		"contract-cli contract esign personal-auth-url --profile contract --as app --input-file psn-auth-url.json",
		"contract-cli contract share batch-create --profile contract --as app --input-file batch-share.json",
		"contract-cli contract cooperation search --profile contract --as app --input-file cooperation-search.json",
		"contract-cli contract cooperation file get <contract-id> --profile contract --as app",
		"contract-cli contract cooperation file download <file-id> --profile contract --as app --output-file ./cooperation.docx",
		"`contract search-v2`、`contract field update`、`contract sign switch-to-paper`、`contract sign-url get`",
		"`contract share get`、`contract share batch-create`、`contract cooperation link get`、`contract cooperation record get`、`contract cooperation search`、`contract cooperation file get/download`",
	} {
		if !strings.Contains(commands, required) {
			t.Fatalf("contract commands reference missing current openapi-gap fragment %q", required)
		}
	}
}

func TestContractSkillRoutingDoesNotCollapseCooperationCommandsToGet(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	skill := readTextFile(t, filepath.Join(root, "skills", "contract-cli-contract", "SKILL.md"))
	for _, forbidden := range []string{
		"协商列表/链接/记录/文件：用 `contract share ...` 或 `contract cooperation ... get --as app`",
		"`contract cooperation ... get --as app`",
	} {
		if strings.Contains(skill, forbidden) {
			t.Fatalf("contract skill still collapses cooperation commands to get: %q", forbidden)
		}
	}
	for _, required := range []string{
		"想查分享、批量分享、协商链接/记录：用 `contract share ...`、`contract share batch-create`、`contract cooperation link get` 或 `contract cooperation record get --as app`",
		"想查协商列表或协商文件：用 `contract cooperation search`、`contract cooperation file get` 或 `contract cooperation file download --as app`",
	} {
		if !strings.Contains(skill, required) {
			t.Fatalf("contract skill missing precise cooperation routing %q", required)
		}
	}
}

func TestSharedSkillRoutesAllCurrentContractCapabilities(t *testing.T) {
	t.Parallel()

	content := readTextFile(t, filepath.Join("..", "..", "skills", "contract-cli-shared", "SKILL.md"))
	for _, destination := range []string{"contract-cli-contract-search", "contract-cli-contract"} {
		if !strings.Contains(content, "](../"+destination+"/SKILL.md)") {
			t.Errorf("shared skill must route to %s", destination)
		}
	}
}

func TestDisabledAPICallAgentMetadataDoesNotSuggestInvocation(t *testing.T) {
	t.Parallel()

	content := readTextFile(t, filepath.Join("..", "..", "skills", "contract-cli-api-call", "agents", "openai.yaml"))
	for _, forbidden := range []string{
		"Use $contract-cli-api-call",
		"allow_implicit_invocation: true",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("disabled api-call agent metadata should not suggest invocation: %q", forbidden)
		}
	}
}
