package cli_test

import (
	"bytes"
	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/* TestRemovedMatrixSnapshotCommands 验证删除的快照发布命令在本地拒绝、不发网络请求；t 为测试上下文，无返回值。 */
func TestRemovedMatrixSnapshotCommands(t *testing.T) {
	for _, command := range []string{"snapshot", "publication prepare", "publication release"} {
		t.Run(command, func(t *testing.T) {
			app := cli.New(cli.Options{Store: config.NewStore(t.TempDir()), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatal("removed command must not send HTTP requests")
				return nil, nil
			})}})
			args := append([]string{"rule", "table"}, strings.Fields(command)...)
			if err := app.Run(context.Background(), args); err == nil {
				t.Fatal("removed command must fail")
			}
		})
	}
}

/* TestRemovedMatrixCapabilitiesCommand 验证 capabilities 命令及兼容入口已移除且不会发 HTTP；t 为测试上下文，无返回值。 */
func TestRemovedMatrixCapabilitiesCommand(t *testing.T) {
	var output bytes.Buffer
	app := cli.New(cli.Options{Store: config.NewStore(t.TempDir()), Stdout: &output, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("removed capabilities command made HTTP request: %s", req.URL.Path)
		return nil, nil
	})}})
	for _, args := range [][]string{
		{"rule", "capabilities", "get", "--product-id", "contract", "--group-id", "approve_matrix"},
		{"approval-matrix", "capabilities", "get", "--product-id", "contract", "--group-id", "approve_matrix"},
	} {
		if err := app.Run(context.Background(), args); err == nil {
			t.Fatalf("removed capabilities command accepted: %v", args)
		}
	}
	if err := app.Run(context.Background(), []string{"help", "rule"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "rule capabilities") {
		t.Fatalf("removed capabilities command remains in help: %s", output.String())
	}
}

/* TestMatrixHelpDeclaresFeatureBaseline 验证审批矩阵帮助明确声明命令集的版本基线；t 为测试上下文，无返回值。 */
func TestMatrixHelpDeclaresFeatureBaseline(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	app := cli.New(cli.Options{Store: config.NewStore(t.TempDir()), Stdout: &output, Stderr: &bytes.Buffer{}})
	if err := app.Run(context.Background(), []string{"help", "rule"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "feature baseline approval-matrix-extensions since 1.8.3-test.13 (commit 61d8aa6)") {
		t.Fatalf("matrix help must declare feature baseline: %s", output.String())
	}
}

/* TestMatrixSymbolQueryUsesBuiltinMapping 验证运算符查询读取 CLI 内置映射且不发网络请求；t 为上下文，返回 void。 */
func TestMatrixSymbolQueryUsesBuiltinMapping(t *testing.T) {
	cases := []struct {
		kind    string
		symbols []string
	}{
		{"STRING", []string{"=", "!=", "in", "notIn"}},
		{"NUMBER", []string{"=", ">", ">=", "<", "<=", "!=", "in", "notIn"}},
		{"COLLECTION", []string{"contain", "notContain", "=", "in", "notIn", "isNull", "isNotNull"}},
		{"EMPLOYEE_COLLECTION", []string{"contain", "notContain", "=", "in", "notIn", "isNull", "isNotNull"}},
		{"DEPARTMENT_COLLECTION", []string{"contain", "notContain", "=", "in", "notIn", "isNull", "isNotNull"}},
		{"BOOLEAN", []string{"=", "!="}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			var output bytes.Buffer
			app := cli.New(cli.Options{Store: config.NewStore(t.TempDir()), Stdout: &output, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("builtin symbol query made HTTP request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			})}})
			args := []string{"rule", "symbol", "query", "--raw", "--data", `{"value_type":"` + tc.kind + `"}`}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			var response struct {
				Code int `json:"code"`
				Data []struct {
					Symbol string `json:"symbol"`
				} `json:"data"`
			}
			if err := json.Unmarshal(output.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Code != 0 || len(response.Data) != len(tc.symbols) {
				t.Fatalf("unexpected response: %s", output.String())
			}
			for i, want := range tc.symbols {
				if response.Data[i].Symbol != want {
					t.Fatalf("symbol[%d]=%q, want %q", i, response.Data[i].Symbol, want)
				}
			}
		})
	}
}

/* TestRemovedMatrixMutationCommandsRejectBeforeHTTP 验证已移除入口不触发网络请求；t 为测试上下文，返回 void。 */
func TestRemovedMatrixMutationCommandsRejectBeforeHTTP(t *testing.T) {
	app := cli.New(cli.Options{Store: config.NewStore(t.TempDir()), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("removed command made HTTP request: %s", req.URL.Path)
		return nil, nil
	})}})
	for _, args := range [][]string{
		{"rule", "table", "mutation", "apply"},
		{"approval-matrix", "table", "mutation", "get"},
		{"rule", "table", "import", "plan", "--guarded"},
	} {
		if err := app.Run(context.Background(), args); err == nil {
			t.Fatalf("removed command accepted: %v", args)
		}
	}
}

/* TestRetiredGuardedPlanDoesNotFallBackToRowWrites 验证旧保护计划停用后不降级写入；入参 t（*testing.T）为测试上下文，返回值为空。 */
func TestRetiredGuardedPlanDoesNotFallBackToRowWrites(t *testing.T) {
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	output := &bytes.Buffer{}
	calls := 0
	app := cli.New(cli.Options{Store: store, Stdout: output, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != "GET" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		switch {
		case calls == 1 && strings.HasSuffix(req.URL.Path, "/column_headers"):
			return jsonResponse(`{"code":0,"data":{"columns_headers":[{"id":"c","name":"金额","type":1,"table_cell_content_type":"NUMBER"}]}}`), nil
		case calls == 2 && strings.HasSuffix(req.URL.Path, "/table_rows"):
			return jsonResponse(`{"code":0,"data":{"table_rows":[],"has_more":false}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})}})
	if err := app.Run(context.Background(), []string{"rule", "table", "import", "plan", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--as", "app", "--data", `{"rows":[{"cells":{"c":1000.50}}]}`}); err != nil {
		t.Fatal(err)
	}
	id := decodeApprovalMatrixOutput(t, output.Bytes())["plan_id"].(string)
	// 构造上一个版本保存的本地文件，不调用已移除的服务端接口。
	path := filepath.Join(filepath.Dir(store.Path()), "approval-matrix-plans", id+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan map[string]json.RawMessage
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	plan["guarded"] = json.RawMessage("true")
	raw, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := app.Run(context.Background(), []string{"rule", "table", "import", "apply", "--plan-id", id, "--as", "app"}); err == nil {
		t.Fatal("invalidated plan apply error = nil")
	}
	if decodeApprovalMatrixOutput(t, output.Bytes())["status"] != "invalidated" || calls != 2 {
		t.Fatalf("legacy plan executed: %s", output.String())
	}
}

/* TestMatrixExtensionRoutes 验证新增命令的路径、原始精度与双身份；t 为测试上下文，返回 void。 */
func TestMatrixExtensionRoutes(t *testing.T) {
	cases := []struct{ command, method, suffix, body string }{
		{"employee batch-get", "POST", "employees/batch_get", `{"ids":["1069134472156283974"],"group_code":"approve_matrix"}`},
		{"department search", "POST", "departments/search", `{"param":"法务"}`},
		{"department batch-get", "POST", "departments/batch_get", `{"ids":["123"]}`},
		{"role search", "POST", "roles/search", `{"param":"法务审批"}`},
		{"role batch-get", "POST", "roles/batch_get", `{"ids":["123"]}`},
		{"loop-function query", "POST", "loop_functions/query", `{"value_type":"DEPARTMENT_COLLECTION"}`},
		{"table column patch", "PATCH", "rule_tables/t/table_columns/c", `{"table_column_name":"合同金额"}`},
		{"table column preview", "POST", "rule_tables/t/table_columns/c/preview", `{"value_type":"NUMBER"}`},
	}
	for _, identity := range []config.IdentityKind{config.IdentityUser, config.IdentityApp} {
		for _, tc := range cases {
			t.Run(string(identity)+"/"+tc.command, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(identity), true); err != nil {
					t.Fatal(err)
				}
				calls := 0
				app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					if req.Method != tc.method || req.URL.Path != "/open-apis/rule_engine/v1/products/contract/groups/approve_matrix/"+tc.suffix {
						t.Fatalf("unexpected %s %s", req.Method, req.URL.Path)
					}
					if tc.command == "department batch-get" && req.URL.Query().Get("department_id_type") != "tenant_department_id" {
						t.Fatalf("default department_id_type = %q", req.URL.Query().Get("department_id_type"))
					}
					if req.Header.Get("Authorization") != "Bearer "+string(identity)+"-token" {
						t.Fatal("wrong identity")
					}
					if got := req.URL.Query().Get("user_id"); got != "" {
						t.Fatalf("matrix request must use authenticated editor, user_id query = %q", got)
					}
					if req.Body != nil {
						raw, _ := io.ReadAll(req.Body)
						if string(raw) != tc.body {
							t.Fatalf("body changed: %s", raw)
						}
					}
					return jsonResponse(`{"code":0,"data":{}}`), nil
				})}})
				args := append([]string{"approval-matrix"}, strings.Fields(tc.command)...)
				args = append(args, "--product-id", "contract", "--group-id", "approve_matrix", "--as", string(identity), "--user-id", "another-user")
				if strings.HasPrefix(tc.command, "table ") {
					args = append(args, "--table-id", "t")
				}
				if strings.Contains(tc.command, " column ") {
					args = append(args, "--column-id", "c")
				}
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
TestDepartmentDirectoryIDContract 验证部门目录返回的 SaaS department_id 可直接用于规则行。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestDepartmentDirectoryIDContract(t *testing.T) {
	t.Parallel()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Store:  store,
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/open-apis/rule_engine/v1/products/contract/groups/approve_matrix/departments/search" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			if req.URL.Query().Get("department_id_type") != "tenant_department_id" {
				t.Fatalf("department_id_type = %q", req.URL.Query().Get("department_id_type"))
			}
			return jsonResponse(`{"code":0,"msg":"success","data":{"items":[{"department_id":"1018399484738012242","open_department_id":"od-1b1b803a7df98989bf457d9ba203c350","name":"产品研发部","selectable":true}],"missing_ids":[]}}`), nil
		})},
	})

	err := app.Run(context.Background(), []string{
		"rule", "department", "search", "--profile", "contract", "--as", "app",
		"--product-id", "contract", "--group-id", "approve_matrix", "--data", `{"param":"产品研发部"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := decodeJSONObject(t, stdout.Bytes())
	data, ok := result["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", result["data"])
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %#v", data["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("item = %#v", items[0])
	}
	if item["department_id"] != "1018399484738012242" || item["open_department_id"] != "od-1b1b803a7df98989bf457d9ba203c350" || item["selectable"] != true {
		t.Fatalf("department candidate = %#v", item)
	}
}

/*
TestDepartmentBatchGetRejectsWritableIDAsLookupID 验证默认模式只接收 SaaS 部门 id。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestDepartmentBatchGetRejectsWritableIDAsLookupID(t *testing.T) {
	t.Parallel()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	requests := 0
	app := cli.New(cli.Options{
		Store:  store,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			return jsonResponse(`{"code":0,"msg":"unexpected","data":{}}`), nil
		})},
	})
	err := app.Run(context.Background(), []string{
		"rule", "department", "batch-get", "--profile", "contract", "--as", "app",
		"--product-id", "contract", "--group-id", "approve_matrix",
		"--data", `{"ids":["od-1b1b803a7df98989bf457d9ba203c350"]}`,
	})
	if err == nil || !strings.Contains(err.Error(), "positive numeric SaaS department id") {
		t.Fatalf("error = %v, want SaaS department ID guidance", err)
	}
	if requests != 0 {
		t.Fatalf("invalid batch-get request reached backend: %d", requests)
	}
}

/*
TestDepartmentBatchGetAcceptsExplicitExternalIDs 验证显式兼容类型可用目录外部 ID 发出请求。
入参 t（*testing.T）为测试上下文；无返回值，逐项核对 user/app 身份、查询参数、请求体和回包。
*/
func TestDepartmentBatchGetAcceptsExplicitExternalIDs(t *testing.T) {
	cases := []struct{ idType, id string }{
		{"department_id", "4155855a78e8cged"},
		{"open_department_id", "od-1b1b803a7df98989bf457d9ba203c350"},
	}
	for _, identity := range []config.IdentityKind{config.IdentityUser, config.IdentityApp} {
		for _, tc := range cases {
			t.Run(string(identity)+"/"+tc.idType, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(identity), true); err != nil {
					t.Fatal(err)
				}
				body := `{"ids":["` + tc.id + `"]}`
				input := filepath.Join(t.TempDir(), "department.json")
				if err := os.WriteFile(input, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				var stdout bytes.Buffer
				requests := 0
				app := cli.New(cli.Options{Store: store, Stdout: &stdout, Stderr: &bytes.Buffer{},
					HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						requests++
						if req.URL.Path != "/open-apis/rule_engine/v1/products/contract/groups/approve_matrix/departments/batch_get" ||
							req.URL.Query().Get("department_id_type") != tc.idType {
							t.Fatalf("unexpected request URL: %s", req.URL.String())
						}
						if req.Header.Get("Authorization") != "Bearer "+string(identity)+"-token" {
							t.Fatal("wrong identity")
						}
						got, err := io.ReadAll(req.Body)
						if err != nil || string(got) != body {
							t.Fatalf("body = %s, read error = %v", got, err)
						}
						return jsonResponse(`{"code":0,"data":{"items":[{"department_id":"` + tc.id + `"}],"missing_ids":[]}}`), nil
					})}})
				err := app.Run(context.Background(), []string{"rule", "department", "batch-get", "--as", string(identity),
					"--profile", "contract", "--product-id", "contract", "--group-id", "approve_matrix",
					"--department-id-type", tc.idType, "--input-file", input})
				if err != nil || requests != 1 {
					t.Fatalf("error = %v, requests = %d", err, requests)
				}
				data := decodeJSONObject(t, stdout.Bytes())["data"].(map[string]any)
				if data["items"].([]any)[0].(map[string]any)["department_id"] != tc.id {
					t.Fatalf("response data = %#v", data)
				}
			})
		}
	}
}

/*
TestDepartmentBatchGetRejectsIDsForWrongType 验证显式类型与 ID 不匹配时在发请求前拒绝。
入参 t（*testing.T）为测试上下文；无返回值，逐项核对错误和零 HTTP 请求。
*/
func TestDepartmentBatchGetRejectsIDsForWrongType(t *testing.T) {
	cases := []struct{ idType, id string }{
		{"tenant_department_id", "4155855a78e8cged"},
		{"open_department_id", "4155855a78e8cged"},
		{"department_id", " "},
		{"unsupported_type", "1018399484738012242"},
	}
	for _, tc := range cases {
		t.Run(tc.idType, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}
			requests := 0
			app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					return jsonResponse(`{"code":0}`), nil
				})}})
			err := app.Run(context.Background(), []string{"rule", "department", "batch-get", "--as", "app",
				"--profile", "contract", "--product-id", "contract", "--group-id", "approve_matrix",
				"--department-id-type", tc.idType, "--data", `{"ids":["` + tc.id + `"]}`})
			if err == nil || requests != 0 {
				t.Fatalf("error = %v, requests = %d", err, requests)
			}
		})
	}
}

/*
TestDepartmentHelpExplainsIDContract 验证命令帮助说明目录 id 可直接用于规则行。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestDepartmentHelpExplainsIDContract(t *testing.T) {
	t.Parallel()
	for _, command := range [][]string{{"help", "rule", "department", "search"}, {"help", "rule", "department", "batch-get"}, {"help", "rule", "table", "import", "plan"}} {
		var stdout bytes.Buffer
		app := cli.New(cli.Options{Store: config.NewStore(t.TempDir()), Stdout: &stdout, Stderr: &bytes.Buffer{}})
		if err := app.Run(context.Background(), command); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout.String(), "SaaS") {
			t.Fatalf("help %v does not explain writable department ID: %s", command, stdout.String())
		}
		if len(command) == 4 && command[3] == "batch-get" && !strings.Contains(stdout.String(), "对应目录字段") {
			t.Fatalf("help %v does not explain explicit department ID types: %s", command, stdout.String())
		}
	}
}
