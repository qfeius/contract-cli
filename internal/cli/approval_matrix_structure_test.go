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
					if tc.command == "column add" && req.Method == http.MethodGet && req.URL.Path == base+"/approve_matrix/rule_tables/table-1/table_columns/column_headers" {
						return jsonResponse(`{"code":0,"data":{"columns_headers":[{"id":"condition","name":"条件","type":1,"table_cell_content_type":"STRING"},{"id":"result","name":"结果","type":2,"table_cell_content_type":"COLLECTION"},{"id":"note","name":"备注","type":3,"table_cell_content_type":"STRING"}]}}`), nil
					}
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
				wantCalls := 1
				if tc.command == "column add" {
					wantCalls = 2
				}
				if calls != wantCalls {
					t.Fatalf("calls=%d, want %d", calls, wantCalls)
				}
			})
		}
	}
}

/*
TestContractRuleCreationScope 验证 contract 产品的规则组创建与跨组建表在发请求前被拦截。
入参 t（*testing.T）为测试上下文；返回值为空，失败通过测试断言报告。
*/
func TestContractRuleCreationScope(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"rule group create", []string{"rule", "group", "create", "--product-id", "contract", "--data", `{"group_id":"hidden","name":"隐藏组"}`}},
		{"approval-matrix group create", []string{"approval-matrix", "group", "create", "--product-id", "contract", "--data", `{"group_id":"hidden","name":"隐藏组"}`}},
		{"rule table create outside fixed group", []string{"rule", "table", "create", "--product-id", "contract", "--group-id", "hidden", "--data", `{"name":"隐藏矩阵"}`}},
		{"approval-matrix table create outside fixed group", []string{"approval-matrix", "table", "create", "--product-id", "contract", "--group-id", "hidden", "--data", `{"name":"隐藏矩阵"}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}
			// 任何创建请求都应停在本地，计数器用于防止别名绕过保护。
			calls := 0
			app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				return jsonResponse(`{"code":0,"data":{}}`), nil
			})}})
			args := append(append([]string{}, tc.args...), "--profile", "contract", "--as", "app")
			err := app.Run(context.Background(), args)
			if err == nil || !strings.Contains(err.Error(), "approve_matrix") {
				t.Fatalf("error = %v, want fixed approve_matrix group error", err)
			}
			if calls != 0 {
				t.Fatalf("HTTP calls = %d, want 0", calls)
			}
		})
	}
}

/*
TestOtherProductRuleGroupCreate 验证固定组限制不影响非 contract 产品的原有创建命令。
入参 t（*testing.T）为测试上下文；返回值为空，失败通过测试断言报告。
*/
func TestOtherProductRuleGroupCreate(t *testing.T) {
	t.Parallel()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	calls := 0
	app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodPost || req.URL.Path != "/open-apis/rule_engine/v1/products/other/groups" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		return jsonResponse(`{"code":0,"data":{}}`), nil
	})}})
	err := app.Run(context.Background(), []string{"rule", "group", "create", "--profile", "contract", "--as", "app", "--product-id", "other", "--data", `{"group_id":"new_group","name":"新规则组"}`})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("HTTP calls = %d, want 1", calls)
	}
}

/*
TestApprovalMatrixColumnAddRejectsLocalColumnLimit 验证新增列前按服务端口径统计条件列、结果列和系统列，并在总数达到 12 时本地阻断。
入参 t（*testing.T）为测试上下文；返回值为空，失败终止当前测试。
*/
func TestApprovalMatrixColumnAddRejectsLocalColumnLimit(t *testing.T) {
	t.Parallel()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	calls := 0
	app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodGet || !strings.HasSuffix(req.URL.Path, "/table_columns/column_headers") {
			t.Fatalf("column limit preflight sent unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return jsonResponse(`{"code":0,"data":{"columns_headers":[
			{"id":"c1","name":"条件1","type":1,"table_cell_content_type":"STRING"},
			{"id":"c2","name":"条件2","type":1,"table_cell_content_type":"STRING"},
			{"id":"c3","name":"条件3","type":1,"table_cell_content_type":"STRING"},
			{"id":"c4","name":"条件4","type":1,"table_cell_content_type":"STRING"},
			{"id":"c5","name":"条件5","type":1,"table_cell_content_type":"STRING"},
			{"id":"c6","name":"条件6","type":1,"table_cell_content_type":"STRING"},
			{"id":"c7","name":"条件7","type":1,"table_cell_content_type":"STRING"},
			{"id":"c8","name":"条件8","type":1,"table_cell_content_type":"STRING"},
			{"id":"r1","name":"结果1","type":2,"table_cell_content_type":"EMPLOYEE_COLLECTION"},
			{"id":"r2","name":"结果2","type":2,"table_cell_content_type":"EMPLOYEE_COLLECTION"},
			{"id":"note","name":"备注","type":3,"table_cell_content_type":"STRING"}
		]}}`), nil
	})}})
	err := app.Run(context.Background(), []string{
		"rule", "table", "column", "add",
		"--profile", "contract", "--as", "app",
		"--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1",
		"--data", `{"base_table_column_id":"c1","direction":1}`,
	})
	if err == nil {
		t.Fatal("column add accepted a thirteenth total column")
	}
	for _, fragment := range []string{"maximum 12", "condition 8", "result 2", "system 2", "priority and remark", "share 10 slots"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("column limit error %q missing %q", err, fragment)
		}
	}
	if calls != 1 {
		t.Fatalf("column limit preflight calls = %d, want one read and no write", calls)
	}
}

/*
TestApprovalMatrixColumnAddHelpExplainsColumnLimit 验证新增列帮助在执行前明确总列数、系统列占用和业务列共享额度。
入参 t（*testing.T）为测试上下文；返回值为空，失败终止当前测试。
*/
func TestApprovalMatrixColumnAddHelpExplainsColumnLimit(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	app := cli.New(cli.Options{Stdout: &output, Stderr: &bytes.Buffer{}, Store: config.NewStore(t.TempDir())})
	if err := app.Run(context.Background(), []string{"help", "rule", "table", "column", "add"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"总列数上限为 12", "优先级列和备注列固定占 2 列", "条件列与结果列共用剩余 10 个名额", "超限写入前本地阻断"} {
		if !strings.Contains(output.String(), fragment) {
			t.Fatalf("column add help missing %q: %s", fragment, output.String())
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
