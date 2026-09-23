package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMDMSkillAgentMetadataMatchesAppIdentityRoutes(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "skills")
	tests := []struct {
		name     string
		file     string
		required []string
	}{
		{
			name: "vendor",
			file: filepath.Join(root, "contract-cli-mdm-vendor", "agents", "openai.yaml"),
			required: []string{
				"user or app",
				"vendor candidates",
				"vendor details",
				"create",
				"update",
				"patch",
				"enable",
				"disable",
				"certificate",
			},
		},
		{
			name: "legal",
			file: filepath.Join(root, "contract-cli-mdm-legal", "agents", "openai.yaml"),
			required: []string{
				"user or app",
				"legal entity candidates",
				"legal entity details",
				"create",
				"update",
				"code",
			},
		},
		{
			name: "fields",
			file: filepath.Join(root, "contract-cli-mdm-fields", "agents", "openai.yaml"),
			required: []string{
				"user or app",
				"vendor",
				"legal_entity",
				"vendor_risk",
				"user/MCP",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := readMDMSkillMetadataText(t, tt.file)
			for _, forbidden := range []string{"user-only", "user-authorized"} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s must not keep obsolete identity wording %q: %s", tt.file, forbidden, content)
				}
			}
			for _, required := range tt.required {
				if !strings.Contains(content, required) {
					t.Fatalf("%s missing identity wording %q: %s", tt.file, required, content)
				}
			}
		})
	}
}

func TestVendorMaintenanceSkillDocumentsNewCommandsAndSafetyRules(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "skills", "contract-cli-mdm-vendor")
	checks := map[string][]string{
		filepath.Join(root, "SKILL.md"): {
			"contract-cli mdm vendor patch <vendor-id>",
			"contract-cli mdm vendor enable <vendor-id>",
			"contract-cli mdm vendor disable <vendor-id>",
			"本 Skill 只指导 Agent 执行 `contract-cli`",
			"不会改变或增强远程 MCP Server",
			"字段不确定时先查询字段配置",
			"子项 ID 不确定时先查询交易方详情",
			"存在歧义时先询问用户",
			"简单创建",
			"复杂创建",
			"附件替换",
			"部门查询后写入",
			"只允许选择 `status=1` 的启用部门",
			"`status=0` 的停用部门不得用于创建或修改",
			"个人创建不允许传 `vendorAccounts[].bankId`",
			"个人 PATCH 不允许传 `vendorAccounts[].bankId`",
			"未传字段不修改，`null` 请求清空",
			"`UNKNOWN`",
			"不得直接重试",
		},
		filepath.Join(root, "references", "commands.md"): {
			"contract-cli mdm vendor create --profile contract --as user",
			"contract-cli mdm vendor patch 7003410079584092448 --profile contract --as app",
			"contract-cli mdm vendor patch 7003410079584092448 --profile contract --as user",
			"contract-cli mdm vendor enable 7003410079584092448 --profile contract --as user",
			"contract-cli mdm vendor disable 7003410079584092448 --profile contract --as user",
		},
		filepath.Join(root, "references", "vendor-create-parameters.md"): {
			"POST /open-apis/mdm/v1/vendors",
			"POST /open-apis/contract/v1/mcp/vendors",
			"个人身份不允许传 `vendor`、`status`、风险字段和系统字段",
			"个人创建不允许传 `vendorAccounts[].bankId`",
			"`vendorAccounts[].bankId`（string，仅 App 可选，个人禁止）",
			"官方 App 请求体示例",
		},
		filepath.Join(root, "references", "vendor-patch-parameters.md"): {
			"PATCH /open-apis/mdm/v1/vendors/{vendor_id}",
			"PATCH /open-apis/contract/v1/mcp/vendors/{vendor_id}",
			"`_delete: true`",
			"未列出的记录保留",
			"个人 PATCH 不允许传 `vendorAccounts[].bankId`",
		},
		filepath.Join(root, "references", "vendor-status-parameters.md"): {
			"PUT /open-apis/contract/v1/mcp/vendors/{vendor_id}/status",
			"NO_CHANGE",
			"不发起审批",
		},
	}

	for file, fragments := range checks {
		content := readMDMSkillMetadataText(t, file)
		for _, fragment := range fragments {
			if !strings.Contains(content, fragment) {
				t.Fatalf("%s missing %q", file, fragment)
			}
		}
	}
}

func TestVendorFieldConfigurationRequiredSemanticsAreConsistent(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "skills")
	checks := map[string][]string{
		filepath.Join(root, "contract-cli-mdm-vendor", "agents", "openai.yaml"): {
			"module 0",
			"modules 1-4",
			"module 5",
			"never fabricate defaults",
		},
		filepath.Join(root, "contract-cli-mdm-vendor", "SKILL.md"): {
			"module 0",
			"全局必填",
			"module 1～4",
			"子项集合本身可选",
			"module 5",
			"不属于交易方创建请求",
			"只询问仍缺失的全局必填字段",
			"不得推荐或生成默认业务值",
			"../contract-cli-mdm-fields/references/vendor-field-config-semantics.md",
		},
		filepath.Join(root, "contract-cli-mdm-fields", "agents", "openai.yaml"): {
			"module 0",
			"modules 1-4",
			"module 5",
		},
		filepath.Join(root, "contract-cli-mdm-fields", "SKILL.md"): {
			"module 0",
			"module 1～4",
			"module 5",
			"references/vendor-field-config-semantics.md",
		},
		filepath.Join(root, "contract-cli-mdm-fields", "references", "schema-fields-guide.md"): {
			"vendor-field-config-semantics.md",
			"子项集合本身可选",
			"不得据此要求用户补齐未请求的子项",
		},
		filepath.Join(root, "contract-cli-mdm-fields", "references", "vendor-field-config-semantics.md"): {
			"module 0",
			"始终存在",
			"module 1～4",
			"仅在提交该子项时生效",
			"module 5",
			"不进入交易方创建请求",
			"不得猜测或推荐默认值",
		},
		filepath.Join(root, "contract-cli-mdm-vendor", "references", "vendor-create-parameters.md"): {
			"vendor-field-config-semantics.md",
			"module 1～4 的子项集合本身可选",
			"module 5 不属于交易方创建请求",
		},
	}

	for file, fragments := range checks {
		content := readMDMSkillMetadataText(t, file)
		for _, fragment := range fragments {
			if !strings.Contains(content, fragment) {
				t.Fatalf("%s missing vendor field semantics %q", file, fragment)
			}
		}
		for _, forbidden := range []string{"全部按建议默认", "全部按默认值", "建议默认值"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s must not recommend fabricated defaults %q", file, forbidden)
			}
		}
	}
}

func TestVendorCustomFieldValueContractIsConsistent(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "skills")
	semanticsFile := filepath.Join(root, "contract-cli-mdm-fields", "references", "vendor-field-config-semantics.md")
	semantics := readMDMSkillMetadataText(t, semanticsFile)
	for _, fragment := range []string{
		"0、1",
		"3、5",
		"`fieldValue`",
		"2",
		"`num`",
		"4、6",
		"`options`",
		"7",
		"`date`",
		"8",
		"`rangeDate`",
		"12",
		"`appendix`",
		"14",
		"`employee`",
		"下拉单选",
		"yyyy-MM-dd",
		"每个自定义字段只提交与 fieldType 对应的一个值属性",
	} {
		if !strings.Contains(semantics, fragment) {
			t.Fatalf("%s missing custom field contract %q", semanticsFile, fragment)
		}
	}

	for _, file := range []string{
		filepath.Join(root, "contract-cli-mdm-fields", "SKILL.md"),
		filepath.Join(root, "contract-cli-mdm-vendor", "SKILL.md"),
		filepath.Join(root, "contract-cli-mdm-vendor", "references", "vendor-create-parameters.md"),
		filepath.Join(root, "contract-cli-mdm-vendor", "references", "vendor-patch-parameters.md"),
		filepath.Join(root, "contract-cli-mdm-vendor", "references", "vendor-update-parameters.md"),
	} {
		content := readMDMSkillMetadataText(t, file)
		if !strings.Contains(content, "vendor-field-config-semantics.md") {
			t.Fatalf("%s must reference the canonical vendor custom-field contract", file)
		}
		for _, misleading := range []string{
			"fieldType`（integer，父对象存在时必填）：文件类型",
			"extendInfo[].fieldType | integer | 父对象存在时必填 | 文件类型",
		} {
			if strings.Contains(content, misleading) {
				t.Fatalf("%s keeps misleading custom-field wording %q", file, misleading)
			}
		}
	}
}

func TestVendorDateRangeClearContractIsExplicit(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "skills")
	semanticsFile := filepath.Join(root, "contract-cli-mdm-fields", "references", "vendor-field-config-semantics.md")
	semantics := readMDMSkillMetadataText(t, semanticsFile)
	for _, fragment := range []string{
		"`rangeDate` 传 `null` 请求清空",
		"`options`、`appendix`、`employee` 传 `[]` 请求清空",
		"`rangeDate: []` 是非法日期区间",
	} {
		if !strings.Contains(semantics, fragment) {
			t.Fatalf("%s missing date-range clear contract %q", semanticsFile, fragment)
		}
	}
	if strings.Contains(semantics, "数组值属性传 `[]` 请求清空") {
		t.Fatalf("%s must not treat every array-shaped custom field as []-clearable", semanticsFile)
	}

	patchFile := filepath.Join(root, "contract-cli-mdm-vendor", "references", "vendor-patch-parameters.md")
	patch := readMDMSkillMetadataText(t, patchFile)
	for _, fragment := range []string{
		`"rangeDate": ["2026-09-22", "2027-09-22"]`,
		`"rangeDate": null`,
		"不提交该 `fieldCode`",
		"`rangeDate: []` 非法",
	} {
		if !strings.Contains(patch, fragment) {
			t.Fatalf("%s missing date-range PATCH example %q", patchFile, fragment)
		}
	}
}

func TestVendorQueryDocumentationDistinguishesUserAndAppSemantics(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "skills", "contract-cli-mdm-vendor")
	for _, file := range []string{
		filepath.Join(root, "SKILL.md"),
		filepath.Join(root, "references", "vendor-query-guide.md"),
		filepath.Join(root, "references", "vendor-query-parameters.md"),
	} {
		content := readMDMSkillMetadataText(t, file)
		for _, required := range []string{
			"user 身份只支持交易方名称模糊查询",
			"app 身份按交易方编码查询",
		} {
			if !strings.Contains(content, required) {
				t.Fatalf("%s missing identity-specific vendor query wording %q", file, required)
			}
		}
		for _, forbidden := range []string{
			"名称或编码查询条件",
			"支持按名称或编码模糊匹配",
		} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s keeps misleading vendor query wording %q", file, forbidden)
			}
		}
	}
}

func TestMDMSkillReferencesDoNotKeepObsoleteUserOnlyClaims(t *testing.T) {
	t.Parallel()

	files := []string{
		filepath.Join("..", "..", "skills", "contract-cli-mdm-vendor", "references", "vendor-query-parameters.md"),
		filepath.Join("..", "..", "skills", "contract-cli-mdm-legal", "references", "entity-query-parameters.md"),
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()

			content := readMDMSkillMetadataText(t, file)
			for _, forbidden := range []string{
				"mdm legal` 和 `mdm fields` 仍保持 user-only",
				"mdm fields` 仍保持 user-only",
			} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s must not keep obsolete identity wording %q: %s", file, forbidden, content)
				}
			}
		})
	}
}

func readMDMSkillMetadataText(t *testing.T, file string) string {
	t.Helper()

	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", file, err)
	}
	return string(content)
}
