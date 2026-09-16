package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

/*
TestApprovalMatrixStructureEndpoints 验证新结构命令与实际后端路径、方法、请求体及 user/app 双身份一致。
入参 t（*testing.T）为测试上下文；返回值为空，失败终止当前测试。
*/
func TestApprovalMatrixStructureEndpoints(t *testing.T) {
	t.Parallel()
	base := "/open-apis/rule_engine/v1/products/contract/groups"
	for _, identity := range []config.IdentityKind{config.IdentityApp, config.IdentityUser} {
		for _, tc := range []struct{ command, method, suffix, body string }{
			{"employee search", "POST", "/approve_matrix/employees/search", `{"param":"赵少帅"}`},
			{"employee search", "POST", "/approve_matrix/employees/search", `{"param":"赵少帅","group_code":"approve_matrix"}`},
			{"employee search", "POST", "/approve_matrix/employees/search", `{"param":"赵少帅","group_code":null}`},
			{"group get", "GET", "/approve_matrix", ""},
			{"group create", "POST", "", `{"group_id":"test_group","name":"测试组"}`},
			{"table create", "POST", "/approve_matrix/rule_tables", `{"name":"采购","match_policy":0}`},
			{"table get", "GET", "/approve_matrix/rule_tables/table-1", ""},
			{"table update", "PUT", "/approve_matrix/rule_tables/table-1", `{"name":"采购2","match_policy":1}`},
			{"table delete", "DELETE", "/approve_matrix/rule_tables/table-1", ""},
			{"column add", "POST", "/approve_matrix/rule_tables/table-1/table_columns", `{"base_table_column_id":"column-1","direction":-1}`},
			{"column update-condition", "PUT", "/approve_matrix/rule_tables/table-1/table_columns/column-1", `{"table_column_name":"金额","value_type":"NUMBER","symbol":"GT"}`},
			{"column update-result", "PUT", "/approve_matrix/rule_tables/table-1/table_columns/column-1", `{"table_column_name":"审批人","result_type":"EMPLOYEE_COLLECTION","default_value":"7113921696628736004"}`},
			{"column delete", "DELETE", "/approve_matrix/rule_tables/table-1/table_columns/column-1", ""},
		} {
			t.Run(string(identity)+"/"+tc.command, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(identity), true); err != nil {
					t.Fatal(err)
				}
				calls := 0
				app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					if req.Method != tc.method || req.URL.Path != base+tc.suffix {
						t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
					}
					if req.Header.Get("Authorization") != "Bearer "+string(identity)+"-token" {
						t.Fatal("wrong identity")
					}
					wantMarker := ""
					if identity == config.IdentityUser {
						wantMarker = "user"
					}
					if req.Header.Get("X-Qfei-Identity") != wantMarker {
						t.Fatal("wrong identity marker")
					}
					if got := req.URL.Query().Get("user_id"); got != "" {
						t.Fatalf("matrix request must use authenticated editor, user_id query = %q", got)
					}
					if req.Body != nil {
						body, _ := io.ReadAll(req.Body)
						if string(body) != tc.body {
							t.Fatalf("body = %s", body)
						}
					}
					return jsonResponse(`{"code":0,"data":{}}`), nil
				})}})
				args := append([]string{"approval-matrix"}, strings.Fields(tc.command)...)
				args = append(args, "--product-id", "contract", "--group-id", "approve_matrix", "--profile", "contract", "--as", string(identity))
				args = append(args, "--table-id", "table-1", "--column-id", "column-1", "--user-id", "another-user")
				if tc.body != "" {
					args = append(args, "--data", tc.body)
				}
				if err := app.Run(context.Background(), args); err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatalf("calls=%d", calls)
				}
			})
		}
	}
}

/*
TestRuleElementSearchRemoved 验证已移除的元素查询命令及兼容入口在本地拒绝且不发 HTTP。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestRuleElementSearchRemoved(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	app := cli.New(cli.Options{
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Store:  config.NewStore(t.TempDir()),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("removed element search made HTTP request: %s", req.URL.Path)
			return nil, nil
		})},
	})
	for _, args := range [][]string{
		{"rule", "element", "search", "--product-id", "contract", "--group-id", "approve_matrix", "--data", `{}`},
		{"approval-matrix", "element", "search", "--product-id", "contract", "--group-id", "approve_matrix", "--data", `{}`},
	} {
		if err := app.Run(context.Background(), args); err == nil {
			t.Fatalf("removed element search accepted: %v", args)
		}
	}
	if err := app.Run(context.Background(), []string{"help", "rule"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "rule element") {
		t.Fatalf("removed element command remains in help: %s", stdout.String())
	}
}

/*
TestApprovalMatrixStructureRejectsInvalidWrites 验证 DTO 明确不接受的写入在发 HTTP 前失败。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestApprovalMatrixStructureRejectsInvalidWrites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ command, body string }{
		{"employee search", `{}`},
		{"employee search", `{"param":"  "}`},
		{"employee search", `{"param":100}`},
		{"employee search", `{"param":"赵少帅","tenant_id":"other"}`},
		{"employee search", `{"param":"赵少帅","group_code":"other"}`},
		{"employee search", `{"param":"赵少帅","group_code":""}`},
		{"employee search", `{"param":"赵少帅","group_code":"  "}`},
		{"employee search", `{"param":"赵少帅","group_code":123}`},
		{"employee search", `{"param":"赵少帅","group_code":"approve_matrix","tenant_id":999}`},
		{"employee search", `{"param":"` + strings.Repeat("赵", 101) + `"}`},
		{"table create", `{}`},
		{"table update", `{"name":"采购","rule_table_id":"other"}`},
		{"table update", `{"name":"采购","match_policy":3}`},
		{"column add", `{"base_table_column_id":"1","direction":0}`},
		{"column update-condition", `{"table_column_name":"金额"}`},
		{"column update-result", `{"table_column_name":"审批人"}`},
	} {
		app := cli.New(cli.Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Store: config.NewStore(t.TempDir()), HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) { t.Fatal("invalid request sent"); return nil, nil })}})
		args := append([]string{"approval-matrix"}, strings.Fields(tc.command)...)
		args = append(args, "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1", "--column-id", "column-1", "--data", tc.body)
		if err := app.Run(context.Background(), args); err == nil {
			t.Fatalf("accepted %s %s", tc.command, tc.body)
		}
	}
}

/*
TestRuleEmployeeSearchFileAndIDMode 验证正式命令的文件输入、大整数 ID 输出与不支持的 ID 模式本地拒绝。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestRuleEmployeeSearchFileAndIDMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	input := filepath.Join(dir, "search.json")
	if err := os.WriteFile(input, []byte(`{"param":"赵少帅","group_code":"approve_matrix"}`), 0600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	calls := 0
	app := cli.New(cli.Options{Store: store, Stdout: &output, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(req.Body)
		if req.Method != "POST" || req.URL.Path != "/open-apis/rule_engine/v1/products/contract/groups/approve_matrix/employees/search" || string(body) != `{"param":"赵少帅","group_code":"approve_matrix"}` {
			t.Fatalf("unexpected search request: %s %s %s", req.Method, req.URL.Path, body)
		}
		if req.URL.Query().Get("user_id_type") != "user_id" {
			t.Fatal("unexpected ID type")
		}
		return jsonResponse(`{"code":0,"data":[{"employee_id":"7113921696628736004","name":"赵少帅","selectable":true}]}`), nil
	})}})
	args := []string{"rule", "employee", "search", "--profile", "contract", "--as", "user", "--product-id", "contract", "--group-id", "approve_matrix", "--input-file", input}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"employee_id": "7113921696628736004"`) {
		t.Fatalf("external ID changed: %s", output.String())
	}
	if err := app.Run(context.Background(), append(args, "--user-id-type", "open_id")); err == nil {
		t.Fatal("accepted incompatible ID mode")
	}
	if calls != 1 {
		t.Fatalf("calls=%d, invalid request reached server", calls)
	}
}
