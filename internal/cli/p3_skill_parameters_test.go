package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

type p3ParameterReference struct {
	skill      string
	file       string
	command    string
	methodPath string
	hasBody    bool
}

func TestP3CommandsHaveDedicatedParameterReferences(t *testing.T) {
	t.Parallel()

	references := p3ParameterReferences()
	if len(references) != 35 {
		t.Fatalf("P3 parameter reference count = %d, want 35", len(references))
	}

	root := filepath.Join("..", "..", "skills")
	seenFiles := make(map[string]struct{}, len(references))
	for _, reference := range references {
		key := reference.skill + "/" + reference.file
		if _, exists := seenFiles[key]; exists {
			t.Fatalf("duplicate parameter reference %s", key)
		}
		seenFiles[key] = struct{}{}

		reference := reference
		t.Run(reference.command, func(t *testing.T) {
			t.Parallel()

			skillContent := readTextFile(t, filepath.Join(root, reference.skill, "SKILL.md"))
			link := "references/" + reference.file
			if !strings.Contains(skillContent, link) {
				t.Fatalf("%s must link %s", reference.skill, link)
			}

			content := readTextFile(t, filepath.Join(root, reference.skill, "references", reference.file))
			for _, fragment := range []string{
				reference.command,
				reference.methodPath,
				"## CLI 参数映射",
				"| 参数 | 请求位置 | 类型 | 必填性 |",
				"## 枚举与约束",
				"## 示例",
				"官方 OpenAPI",
			} {
				if !strings.Contains(content, fragment) {
					t.Fatalf("%s missing %q", reference.file, fragment)
				}
			}
			if reference.hasBody && !strings.Contains(content, "## 请求体字段") {
				t.Fatalf("%s must document request body fields", reference.file)
			}
			for _, forbidden := range []string{"| unknown |", "| array<> |", "Authorization:", "Bearer ", "app_secret"} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s contains incomplete or sensitive content %q", reference.file, forbidden)
				}
			}
			if strings.Count(content, "\n") > 100 && !strings.Contains(content, "## 目录") {
				t.Fatalf("%s must provide navigation for a long parameter reference", reference.file)
			}
		})
	}
}

func p3ParameterReferences() []p3ParameterReference {
	return []p3ParameterReference{
		{"contract-cli-contract-search", "search-v2-parameters.md", "contract-cli contract search-v2", "POST /open-apis/contract/v1/contracts/searchV2", true},
		{"contract-cli-contract", "field-update-parameters.md", "contract-cli contract field update", "PUT /open-apis/contract/v1/attribute_definition", true},
		{"contract-cli-contract", "sign-switch-to-paper-parameters.md", "contract-cli contract sign switch-to-paper", "POST /open-apis/contract/v1/contracts/signType/switchToPaper", false},
		{"contract-cli-contract", "sign-url-get-parameters.md", "contract-cli contract sign-url get", "GET /open-apis/contract/v1/contracts/{contract_id}/sign_url", false},
		{"contract-cli-contract", "form-attribute-list-parameters.md", "contract-cli contract form attribute list", "GET /open-apis/contract/v1/form_definition/attribute", false},
		{"contract-cli-contract", "authorization-grant-parameters.md", "contract-cli contract authorization grant", "POST /open-apis/contract/v1/authorizations", true},
		{"contract-cli-contract", "esign-personal-auth-url-parameters.md", "contract-cli contract esign personal-auth-url", "POST /open-apis/esign/auth/psnAuthUrl", true},
		{"contract-cli-contract", "esign-org-auth-url-parameters.md", "contract-cli contract esign org-auth-url", "POST /open-apis/esign/auth/orgAuthUrl", true},
		{"contract-cli-contract", "share-batch-create-parameters.md", "contract-cli contract share batch-create", "POST /open-apis/contract/v1/contracts/contract/batch_share", true},
		{"contract-cli-contract", "cooperation-search-parameters.md", "contract-cli contract cooperation search", "POST /open-apis/contract/v1/cooperation/search", true},
		{"contract-cli-contract", "cooperation-file-get-parameters.md", "contract-cli contract cooperation file get", "GET /open-apis/contract/v1/contracts/{contract_id}/cooperation/file_info", false},
		{"contract-cli-contract", "cooperation-file-download-parameters.md", "contract-cli contract cooperation file download", "GET /open-apis/contract/v1/contracts/cooperation/{file_id}/download_file", false},
		{"contract-cli-mdm-exchange", "fixed-exchange-rate-get-parameters.md", "contract-cli mdm fixed-exchange-rate get", "GET /open-apis/mdm/v1/fixed_exchange_rate", false},
		{"contract-cli-mdm-exchange", "fixed-exchange-rate-update-parameters.md", "contract-cli mdm fixed-exchange-rate update", "PUT /open-apis/mdm/v1/fixed_exchange_rate", true},
		{"contract-cli-mdm-vendor", "vendor-create-parameters.md", "contract-cli mdm vendor create", "POST /open-apis/mdm/v1/vendors", true},
		{"contract-cli-mdm-vendor", "vendor-update-parameters.md", "contract-cli mdm vendor update", "PUT /open-apis/mdm/v1/vendors/{vendor_id}", true},
		{"contract-cli-mdm-vendor", "vendor-patch-parameters.md", "contract-cli mdm vendor patch", "PATCH /open-apis/mdm/v1/vendors/{vendor_id}", true},
		{"contract-cli-mdm-vendor", "vendor-status-parameters.md", "contract-cli mdm vendor enable", "PUT /open-apis/contract/v1/mcp/vendors/{vendor_id}/status", false},
		{"contract-cli-mdm-vendor", "vendor-list-all-parameters.md", "contract-cli mdm vendor list-all", "GET /open-apis/mdm/v1/vendors/list_all", false},
		{"contract-cli-mdm-vendor", "vendor-query-by-cert-parameters.md", "contract-cli mdm vendor query-by-cert", "GET /open-apis/mdm/v1/vendors/query_vendors", false},
		{"contract-cli-mdm-legal", "legal-create-parameters.md", "contract-cli mdm legal create", "POST /open-apis/mdm/v1/legal_entities", true},
		{"contract-cli-mdm-legal", "legal-update-parameters.md", "contract-cli mdm legal update", "PUT /open-apis/mdm/v1/legal_entities/{legal_entity_id}", true},
		{"contract-cli-mdm-legal", "legal-get-by-code-parameters.md", "contract-cli mdm legal get --code", "GET /open-apis/mdm/v1/legal_entities", false},
		{"contract-cli-mdm-file", "file-download-parameters.md", "contract-cli mdm file download", "GET /open-apis/mdm/v1/file/download/{file_id}", false},
		{"contract-cli-event", "outbound-ip-list-parameters.md", "contract-cli event outbound-ip list", "GET /open-apis/event/v1/outbound_ip", false},
		{"contract-cli-rule", "table-list-parameters.md", "contract-cli rule table list", "GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables", false},
		{"contract-cli-rule", "table-pre-release-parameters.md", "contract-cli rule table pre-release", "PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/pre_release", false},
		{"contract-cli-rule", "table-release-parameters.md", "contract-cli rule table release", "PATCH /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/release", false},
		{"contract-cli-rule", "column-headers-list-parameters.md", "contract-cli rule table column-headers list", "GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_columns/column_headers", false},
		{"contract-cli-rule", "row-create-parameters.md", "contract-cli rule table row create", "POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows", true},
		{"contract-cli-rule", "row-get-parameters.md", "contract-cli rule table row get", "GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/{row_id}", false},
		{"contract-cli-rule", "row-list-parameters.md", "contract-cli rule table row list", "GET /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows", false},
		{"contract-cli-rule", "row-search-parameters.md", "contract-cli rule table row search", "POST /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/search", true},
		{"contract-cli-rule", "row-update-parameters.md", "contract-cli rule table row update", "PUT /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/{row_id}", true},
		{"contract-cli-rule", "row-delete-parameters.md", "contract-cli rule table row delete", "DELETE /open-apis/rule_engine/v1/products/{product_id}/groups/{group_id}/rule_tables/{table_id}/table_rows/{row_id}", false},
	}
}
