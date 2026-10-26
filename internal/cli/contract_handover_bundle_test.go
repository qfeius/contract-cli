package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	contractskills "cn.qfei/contract-cli/skills"
)

func TestHandoverSkillInstalledReferencesAreSelfContained(t *testing.T) {
	target := t.TempDir()
	app := cli.New(cli.Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Store: config.NewStore(t.TempDir()), SkillsFS: contractskills.FS})
	if err := app.Run(context.Background(), []string{"skills", "install", "--target", target}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(target, "contract-handover")
	for _, name := range []string{"identity.md", "contracts.md", "handover.md", "assemblies.md", "transport.md"} {
		if _, err := os.Stat(filepath.Join(root, "references", name)); err != nil {
			t.Error(err)
		}
	}
	assertSearchSkillLinksResolve(t, root)
	if _, err := os.Stat(filepath.Join(root, "references", "api-contract.yaml")); !os.IsNotExist(err) {
		t.Errorf("development contract must not ship as runtime instructions: %v", err)
	}
}

func TestHandoverDocumentedWriteRequestPreservesIdentityAndResourceTypes(t *testing.T) {
	raw, err := contractskills.FS.ReadFile("contract-handover/references/handover.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```").FindAllSubmatch(raw, -1)
	if len(blocks) != 1 {
		t.Fatalf("want one complete write example, got %d", len(blocks))
	}
	var request struct {
		Manual    bool   `json:"manual"`
		Reason    string `json:"reason"`
		Employee  string `json:"targetEmployeeId"`
		Tenant    string `json:"targetTenantId"`
		Resources []struct {
			ID     string `json:"businessId"`
			Type   int    `json:"businessTypeCode"`
			Parent string `json:"parentId"`
		} `json:"resourceList"`
	}
	if err := json.Unmarshal(blocks[0][1], &request); err != nil {
		t.Fatal(err)
	}
	if !request.Manual || request.Reason != "" || request.Employee == "" || request.Tenant == "" || len(request.Resources) != 1 {
		t.Fatalf("incomplete handover request: %+v", request)
	}
	if request.Resources[0].ID != "9223372036854775806" || request.Resources[0].Parent != "9223372036854775805" {
		t.Fatal("Long resource IDs must retain all decimal digits as strings")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(blocks[0][1], &fields); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"handoverByAdmin", "onlyOwner", "onlySubmitter", "operatorId", "token"} {
		if _, ok := fields[forbidden]; ok {
			t.Errorf("write example must not invent %s", forbidden)
		}
	}
}

func TestHandoverParameterReferencesHaveTypedRequiredness(t *testing.T) {
	for _, name := range []string{"identity.md", "contracts.md", "handover.md", "assemblies.md"} {
		raw, err := contractskills.FS.ReadFile("contract-handover/references/" + name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if strings.Contains(text, "| 参数 | 位置 | 必填 |") {
			t.Errorf("%s conflates API requiredness with recipe policy and omits types", name)
		}
		if !strings.Contains(text, "| 参数 | 位置 | 类型 | 本流程要求 | 值、来源与限制 |") {
			t.Errorf("%s has no complete parameter definition", name)
		}
	}
	raw, err := contractskills.FS.ReadFile("contract-handover/references/resource-types.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile(`(?m)^\| ([0-9]+) \|`).FindAllSubmatch(raw, -1)
	codes := map[string]bool{}
	for _, row := range rows {
		codes[string(row[1])] = true
	}
	for _, code := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "99"} {
		if !codes[code] {
			t.Errorf("missing business enum %s", code)
		}
	}
}

func TestHandoverRuntimeReusesSearchWithoutDevelopmentInstructions(t *testing.T) {
	development := regexp.MustCompile(`(?i)\btest\b|测试|验收|样本`)
	err := fs.WalkDir(contractskills.FS, "contract-handover", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := contractskills.FS.ReadFile(path)
		if err != nil {
			return err
		}
		if development.Match(raw) {
			t.Errorf("runtime resource contains development-stage instructions: %s", path)
		}
		if bytes.Contains(raw, []byte("/es/search/contractList")) && path != "contract-handover/references/assembly-lookup.md" {
			t.Errorf("only the bounded assembly lookup recipe may define Web search: %s", path)
		}
		if bytes.Contains(raw, []byte("searchAfter")) {
			t.Errorf("handover must delegate general search pagination: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := contractskills.FS.ReadFile("contract-handover/references/contracts.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("../../contract-cli-contract-search/SKILL.md")) {
		t.Fatal("search delegation reference missing")
	}
	if _, err := contractskills.FS.ReadFile("contract-cli-contract-search/SKILL.md"); err != nil {
		t.Fatal(err)
	}
}

func TestHandoverAssemblyLookupHasBoundedNumberOnlyRequest(t *testing.T) {
	raw, err := contractskills.FS.ReadFile("contract-handover/references/assembly-lookup.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("POST /open-web/web/clm/es/search/contractList")) {
		t.Fatal("verified parent lookup endpoint missing")
	}
	blocks := regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```").FindAllSubmatch(raw, -1)
	if len(blocks) != 1 {
		t.Fatalf("want one complete number-only lookup request, got %d", len(blocks))
	}
	const expected = `{"conditionUnits":[],"filterUnits":[{"searchField":"CONTRACT_NUMBER_FIELD","searchValue":"CG-DEMO-01","unionType":"MUST"}],"order":"DESC","pageIndex":0,"pageSize":10,"searchTabCode":0,"sortType":"SUBMITTED_TIME","withStatistics":false}`
	var actual, wanted any
	if err := json.Unmarshal(blocks[0][1], &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(expected), &wanted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, wanted) {
		t.Fatal("lookup must be a bounded contract-number query without identity overrides")
	}
}
