package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestContractTemplateListExitCode(t *testing.T) {
	const path = "/open-apis/contract/v1/templates"
	const rejected = `{"code":110405,"msg":"category not found","success":false,"data":null}`
	list := []string{"contract", "template", "list"}
	for _, tc := range []struct {
		name       string
		args       []string
		path, body string
		exit       int
	}{
		{"app business failure", append(list, "--as", "app"), path, rejected, 1},
		{"default app raw failure", append(list, "--raw"), path, rejected, 1},
		{"false overrides zero", list, path, `{"code":0,"success":false,"msg":"failed"}`, 1},
		{"nonzero without flag", list, path, `{"code":110405,"msg":"category not found"}`, 1},
		{"normal list", append(list, "--category-number", "CAT-1", "--user-id", "ou_app_owner", "--user-id-type", "employee_id"), path, `{"code":0,"success":true,"data":{"template_brief_infos":[{"template_id":"tpl-1"}],"has_more":false}}`, 0},
		{"empty list", list, path, `{"code":0,"data":{"template_brief_infos":[],"has_more":false}}`, 0},
		{"user passthrough", append(list, "--as", "user"), "/open-apis/contract/v1/mcp/templates", rejected, 0},
		{"generic api passthrough", []string{"api", "call", "--channel", "web", "--as", "user", "GET", "/open-web/web/clm/contract/info"}, "/open-web/web/clm/contract/info", rejected, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			calls := 0
			app := newDebugLoggingApp(t, func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != http.MethodGet || req.URL.Path != tc.path {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				return jsonResponse(tc.body), nil
			}, &stdout, &stderr)
			exit := app.Execute(context.Background(), append(tc.args, "--profile", "contract", "--output", "json"))
			if exit != tc.exit || calls != 1 {
				t.Fatalf("exit=%d want %d, calls=%d; stdout=%s stderr=%s", exit, tc.exit, calls, &stdout, &stderr)
			}
			var got, want any
			if json.Unmarshal(stdout.Bytes(), &got) != nil || json.Unmarshal([]byte(tc.body), &want) != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("response changed: %s", &stdout)
			}
			if tc.name == "default app raw failure" && stdout.String() != tc.body {
				t.Fatal("raw response changed")
			}
			if tc.exit == 1 && !strings.Contains(stderr.String(), "business code") {
				t.Fatalf("missing business error: %s", &stderr)
			}
		})
	}
}
