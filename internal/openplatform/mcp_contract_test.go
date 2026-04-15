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
