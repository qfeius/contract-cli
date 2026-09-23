package openplatform_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/openplatform"
)

func TestContractMCPToolSpecsStayAlignedWithMCPYAML(t *testing.T) {
	t.Parallel()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller() failed")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	data, err := os.ReadFile(filepath.Join(repoRoot, "mcp.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("mcp.yaml not found; skipping local MCP contract alignment test")
		}
		t.Fatalf("ReadFile(mcp.yaml) error = %v", err)
	}
	tools := parseMCPTools(string(data))
	specs := openplatform.ContractMCPToolSpecs()

	for _, spec := range specs {
		tool, ok := tools[spec.ToolName]
		if !ok {
			t.Fatalf("tool %q not found in mcp.yaml", spec.ToolName)
		}
		if tool.Method != spec.Method {
			t.Fatalf("tool %q method = %q, want %q", spec.ToolName, tool.Method, spec.Method)
		}
		expectedPath := spec.Path
		if index := strings.Index(expectedPath, "{"); index >= 0 {
			expectedPath = expectedPath[:index]
		}
		if !strings.Contains(tool.URL, expectedPath) {
			t.Fatalf("tool %q url = %q, want path prefix %q", spec.ToolName, tool.URL, expectedPath)
		}
		for key, values := range spec.FixedQuery {
			if len(values) == 0 {
				continue
			}
			if !strings.Contains(tool.URL, key+"=") {
				t.Fatalf("tool %q url = %q, missing query key %q", spec.ToolName, tool.URL, key)
			}
		}
	}
}

func TestContractMCPToolSpecsDeclareReadWriteSemantics(t *testing.T) {
	t.Parallel()

	writes := map[string]bool{
		"sync-user-groups":         true,
		"create-contracts":         true,
		"create-template-instance": true,
		"create-vendor":            true,
		"patch-vendor":             true,
		"set-vendor-status":        true,
		"create-process-comment":   true,
		"process-approval-task":    true,
	}
	for _, spec := range openplatform.ContractMCPToolSpecs() {
		want := openplatform.OperationRead
		if writes[spec.ToolName] {
			want = openplatform.OperationWrite
		}
		if spec.OperationKind != want {
			t.Fatalf("tool %q operation = %q, want %q", spec.ToolName, spec.OperationKind, want)
		}
	}
}

func TestVendorMaintenanceMCPToolSpecs(t *testing.T) {
	t.Parallel()

	wants := map[string]struct {
		method string
		path   string
	}{
		"create-vendor": {
			method: "POST",
			path:   "/open-apis/contract/v1/mcp/vendors",
		},
		"patch-vendor": {
			method: "PATCH",
			path:   "/open-apis/contract/v1/mcp/vendors/{vendor_id}",
		},
		"set-vendor-status": {
			method: "PUT",
			path:   "/open-apis/contract/v1/mcp/vendors/{vendor_id}/status",
		},
	}

	for name, want := range wants {
		spec, ok := openplatform.ContractMCPToolSpec(name)
		if !ok {
			t.Fatalf("tool %q is not configured", name)
		}
		if spec.Method != want.method || spec.Path != want.path {
			t.Fatalf("tool %q = %s %s, want %s %s", name, spec.Method, spec.Path, want.method, want.path)
		}
		if spec.IdentityPolicy != openplatform.IdentityPolicyUserOnly {
			t.Fatalf("tool %q identity policy = %q, want user_only", name, spec.IdentityPolicy)
		}
		if spec.OperationKind != openplatform.OperationWrite {
			t.Fatalf("tool %q operation = %q, want write", name, spec.OperationKind)
		}
		if got := spec.FixedQuery.Get("user_id_type"); got != "user_id" {
			t.Fatalf("tool %q user_id_type = %q, want user_id", name, got)
		}
	}
}

func TestContractMCPToolSpecsIncludeApprovalWorkflow(t *testing.T) {
	t.Parallel()

	want := map[string]struct {
		method    string
		path      string
		operation openplatform.OperationKind
	}{
		"list-process-comments": {
			method:    "GET",
			path:      "/open-apis/contract/v1/mcp/process_instances/{process_instance_id}/comments",
			operation: openplatform.OperationRead,
		},
		"download-contract-file": {
			method:    "GET",
			path:      "/open-apis/contract/v1/mcp/contracts/{contract_id}/files/{file_id}/download",
			operation: openplatform.OperationRead,
		},
		"get-process-instance": {
			method:    "GET",
			path:      "/open-apis/contract/v1/mcp/process_instances/{process_instance_id}",
			operation: openplatform.OperationRead,
		},
		"create-process-comment": {
			method:    "POST",
			path:      "/open-apis/contract/v1/mcp/process_instances/{process_instance_id}/comments",
			operation: openplatform.OperationWrite,
		},
		"list-personal-tasks": {
			method:    "POST",
			path:      "/open-apis/contract/v1/mcp/tasks",
			operation: openplatform.OperationRead,
		},
		"process-approval-task": {
			method:    "POST",
			path:      "/open-apis/contract/v1/mcp/tasks/{task_instance_id}/approval",
			operation: openplatform.OperationWrite,
		},
	}

	for name, expected := range want {
		spec, ok := openplatform.ContractMCPToolSpec(name)
		if !ok {
			t.Errorf("ContractMCPToolSpec(%q) was not found", name)
			continue
		}
		if spec.Method != expected.method || spec.Path != expected.path || spec.OperationKind != expected.operation {
			t.Errorf("spec %q = (%s, %s, %s), want (%s, %s, %s)", name, spec.Method, spec.Path, spec.OperationKind, expected.method, expected.path, expected.operation)
		}
		wantQuery := ""
		if name == "get-process-instance" || name == "list-personal-tasks" || name == "list-process-comments" {
			wantQuery = "user_id_type=user_id"
		}
		if spec.FixedQuery.Encode() != wantQuery {
			t.Errorf("spec %q fixed query = %v, want %q", name, spec.FixedQuery, wantQuery)
		}
	}
}

type mcpTool struct {
	Method string
	URL    string
}

func parseMCPTools(content string) map[string]mcpTool {
	lines := strings.Split(content, "\n")
	tools := map[string]mcpTool{}

	currentName := ""
	inRequestTemplate := false
	current := mcpTool{}

	flush := func() {
		if currentName == "" {
			return
		}
		tools[currentName] = current
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "- args:"):
			flush()
			currentName = ""
			current = mcpTool{}
			inRequestTemplate = false
		case strings.HasPrefix(line, "  name: "):
			currentName = unquoteYAMLString(strings.TrimPrefix(line, "  name: "))
		case strings.HasPrefix(line, "  requestTemplate:"):
			inRequestTemplate = true
		case inRequestTemplate && strings.HasPrefix(line, "    method: "):
			current.Method = unquoteYAMLString(strings.TrimPrefix(line, "    method: "))
		case inRequestTemplate && strings.HasPrefix(line, "    url: "):
			value := strings.TrimPrefix(line, "    url: ")
			for !strings.HasSuffix(strings.TrimSpace(value), `"`) && i+1 < len(lines) {
				i++
				value += strings.TrimSpace(lines[i])
			}
			current.URL = unquoteYAMLString(value)
		case inRequestTemplate && strings.HasPrefix(line, "  responseTemplate:"):
			inRequestTemplate = false
		}
	}
	flush()

	return tools
}

func unquoteYAMLString(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, `"`)
	value = strings.TrimSuffix(value, `"`)
	return value
}
