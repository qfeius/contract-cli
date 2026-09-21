package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"github.com/gofrs/flock"
)

/*
TestApprovalMatrixImportPlanAndRetryableApply 验证计划转换、部分失败续跑和成功计划防重复写入。
入参 t（*testing.T）为 Go 测试上下文。
返回值为空；失败通过 t.Fatalf 报告。
*/
func TestApprovalMatrixImportPlanAndRetryableApply(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}

	columnsPath := "/open-apis/rule_engine/v1/products/contract/groups/approve_matrix/rule_tables/table-1/table_columns/column_headers"
	rowsPath := "/open-apis/rule_engine/v1/products/contract/groups/approve_matrix/rule_tables/table-1/table_rows"
	createCalls := 0
	updateCalls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer app-token" {
			t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
		}
		if req.URL.Query().Get("user_id_type") != "user_id" {
			t.Fatalf("user_id_type = %q", req.URL.Query().Get("user_id_type"))
		}
		if got := req.URL.Query().Get("user_id"); got != "" {
			t.Fatalf("matrix request must use authenticated editor, user_id query = %q", got)
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == columnsPath:
			return jsonResponse(`{
				"code":0,
				"msg":"success",
				"data":{"columns_headers":[
					{"id":"column-amount","name":"合同金额","type":1,"table_cell_content_type":"NUMBER"},
					{"id":"column-approvers","name":"审批人","type":2,"table_cell_content_type":"EMPLOYEE_COLLECTION"},
					{"id":"column-department","name":"部门","type":3,"table_cell_content_type":"DEPARTMENT_COLLECTION"}
				]}
			}`), nil
		case req.Method == http.MethodPost && req.URL.Path == rowsPath:
			createCalls++
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll(create body) error = %v", err)
			}
			bodyText := string(body)
			for _, fragment := range []string{`"table_column_id":"column-amount"`, `"number":1000000`, `"table_column_id":"column-approvers"`, `"employee_collection":[7113921696628736004]`, `"table_column_id":"column-department"`, `"department_collection":["od-1b1b803a7df98989bf457d9ba203c350"]`} {
				if !strings.Contains(bodyText, fragment) {
					t.Fatalf("create body missing %q: %s", fragment, bodyText)
				}
			}
			return jsonResponse(`{"code":0,"msg":"success","data":{"table_row_id":"row-created"}}`), nil
		case req.Method == http.MethodPut && req.URL.Path == rowsPath+"/row-existing":
			updateCalls++
			if updateCalls == 1 {
				return jsonResponse(`{"code":41001,"msg":"temporary business rejection","data":{}}`), nil
			}
			return jsonResponse(`{"code":0,"msg":"success","data":{}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout:     stdout,
		Stderr:     &bytes.Buffer{},
		Store:      store,
		HTTPClient: &http.Client{Transport: transport},
	})

	input := `{
		"rows":[
		{"operation":"create","cells":{"合同金额":1000000,"审批人":["7113921696628736004"],"部门":["od-1b1b803a7df98989bf457d9ba203c350"]}},
			{"operation":"update","row_id":"row-existing","cells":{"column-amount":1200000}}
		]
	}`
	planArgs := []string{
		"rule", "table", "import", "plan",
		"--profile", "contract", "--as", "app",
		"--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1",
		"--data", input, "--user-id", "another-user",
	}
	if err := app.Run(context.Background(), planArgs); err != nil {
		t.Fatalf("Run(plan) error = %v", err)
	}
	planOutput := decodeApprovalMatrixOutput(t, stdout.Bytes())
	if planOutput["status"] != "needs_confirmation" {
		t.Fatalf("plan status = %v", planOutput["status"])
	}
	planID, ok := planOutput["plan_id"].(string)
	if !ok || len(planID) != 32 {
		t.Fatalf("plan_id = %#v", planOutput["plan_id"])
	}
	planPath := filepath.Join(dir, "approval-matrix-plans", planID+".json")
	info, err := os.Stat(planPath)
	if err != nil {
		t.Fatalf("Stat(plan) error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("plan permissions = %o, want 600", info.Mode().Perm())
	}

	stdout.Reset()
	applyArgs := []string{"rule", "table", "import", "apply", "--plan-id", planID, "--profile", "contract", "--as", "app"}
	if err := app.Run(context.Background(), applyArgs); err == nil {
		t.Fatal("Run(first apply) error = nil, want partial_success error")
	}
	firstApplyOutput := decodeApprovalMatrixOutput(t, stdout.Bytes())
	if firstApplyOutput["status"] != "partial_success" {
		t.Fatalf("first apply status = %v, output = %s", firstApplyOutput["status"], stdout.String())
	}
	if createCalls != 1 || updateCalls != 1 {
		t.Fatalf("first apply calls create=%d update=%d", createCalls, updateCalls)
	}

	stdout.Reset()
	if err := app.Run(context.Background(), applyArgs); err != nil {
		t.Fatalf("Run(retry apply) error = %v", err)
	}
	retryOutput := decodeApprovalMatrixOutput(t, stdout.Bytes())
	if retryOutput["status"] != "success" {
		t.Fatalf("retry status = %v, output = %s", retryOutput["status"], stdout.String())
	}
	if createCalls != 1 || updateCalls != 2 {
		t.Fatalf("retry should skip successful create, calls create=%d update=%d", createCalls, updateCalls)
	}

	stdout.Reset()
	if err := app.Run(context.Background(), applyArgs); err != nil {
		t.Fatalf("Run(completed apply) error = %v", err)
	}
	if createCalls != 1 || updateCalls != 2 {
		t.Fatalf("completed plan should not write again, calls create=%d update=%d", createCalls, updateCalls)
	}
}

/*
TestApprovalMatrixImportPlanReturnsNeedsInputWithoutSavingPlan 验证列和类型问题以结构化状态返回且不落无效计划。
入参 t（*testing.T）为 Go 测试上下文。
返回值为空；失败通过 t.Fatalf 报告。
*/
func TestApprovalMatrixImportPlanReturnsNeedsInputWithoutSavingPlan(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	requests := 0
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			return jsonResponse(`{"code":0,"msg":"success","data":{"columns_headers":[{"id":"column-amount","name":"合同金额","type":1,"table_cell_content_type":"NUMBER"}]}}`), nil
		})},
	})

	args := []string{
		"rule", "table", "import", "plan",
		"--profile", "contract", "--as", "app",
		"--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1",
		"--data", `{"rows":[{"cells":{"合同金额":"一百万","不存在的列":true}}]}`,
	}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("Run(plan) error = %v", err)
	}
	output := decodeApprovalMatrixOutput(t, stdout.Bytes())
	if output["status"] != "needs_input" {
		t.Fatalf("status = %v, output = %s", output["status"], stdout.String())
	}
	issues, ok := output["issues"].([]any)
	if !ok || len(issues) != 2 {
		t.Fatalf("issues = %#v", output["issues"])
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want one column request", requests)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "approval-matrix-plans"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir(plans) error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("invalid input saved plan files: %v", entries)
	}
}

/*
TestApprovalMatrixImportPlanWithoutTableReturnsCandidates 验证缺少目标矩阵时返回可供 Agent 追问的候选列表。
入参 t（*testing.T）为 Go 测试上下文。
返回值为空；失败通过 t.Fatalf 报告。
*/
func TestApprovalMatrixImportPlanWithoutTableReturnsCandidates(t *testing.T) {
	t.Parallel()

	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			wantPath := "/open-apis/rule_engine/v1/products/contract/groups/approve_matrix/rule_tables"
			if req.Method != http.MethodGet || req.URL.Path != wantPath {
				t.Fatalf("request = %s %s", req.Method, req.URL.String())
			}
			return jsonResponse(`{"code":0,"msg":"success","data":{"items":[{"id":"table-1","name":"采购审批矩阵"}]}}`), nil
		})},
	})

	err := app.Run(context.Background(), []string{
		"rule", "table", "import", "plan", "--profile", "contract", "--as", "app",
		"--product-id", "contract", "--group-id", "approve_matrix",
		"--data", `{"rows":[{"cells":{"合同金额":1}}]}`,
	})
	if err != nil {
		t.Fatalf("Run(plan without table) error = %v", err)
	}
	output := decodeApprovalMatrixOutput(t, stdout.Bytes())
	if output["status"] != "needs_input" || !strings.Contains(stdout.String(), "采购审批矩阵") {
		t.Fatalf("unexpected candidate output: %s", stdout.String())
	}
}

/*
TestApprovalMatrixImportApplyRejectsConcurrentExecution 验证同一计划的并发执行在发送写请求前被计划锁阻止。
入参 t（*testing.T）为 Go 测试上下文。
返回值为空；失败通过 t.Fatalf 报告。
*/
func TestApprovalMatrixImportApplyRejectsConcurrentExecution(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatalf("UpsertProfile() error = %v", err)
	}
	stdout := &bytes.Buffer{}
	writes := 0
	app := cli.New(cli.Options{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Store:  store,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == http.MethodGet {
				return jsonResponse(`{"code":0,"msg":"success","data":{"columns_headers":[{"id":"column-amount","name":"合同金额","type":1,"table_cell_content_type":"NUMBER"}]}}`), nil
			}
			writes++
			return jsonResponse(`{"code":0,"msg":"success","data":{}}`), nil
		})},
	})
	if err := app.Run(context.Background(), []string{
		"rule", "table", "import", "plan", "--profile", "contract", "--as", "app",
		"--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1",
		"--data", `{"rows":[{"cells":{"合同金额":1}}]}`,
	}); err != nil {
		t.Fatalf("Run(plan) error = %v", err)
	}
	planID := decodeApprovalMatrixOutput(t, stdout.Bytes())["plan_id"].(string)
	planLock := flock.New(filepath.Join(dir, "approval-matrix-plans", planID+".json.lock"))
	locked, err := planLock.TryLock()
	if err != nil || !locked {
		t.Fatalf("TryLock() locked=%v error=%v", locked, err)
	}
	defer planLock.Unlock()

	err = app.Run(context.Background(), []string{"rule", "table", "import", "apply", "--plan-id", planID, "--profile", "contract", "--as", "app"})
	if err == nil || !strings.Contains(err.Error(), "already being applied") {
		t.Fatalf("unexpected concurrent apply error: %v", err)
	}
	if writes != 0 {
		t.Fatalf("concurrent apply sent %d write requests", writes)
	}
}

/*
TestApprovalMatrixImportUserIdentity 验证 user 导入及身份切换保护，计划文件不保存原始凭证。
入参 t 为测试上下文；返回值为空。
*/
func TestApprovalMatrixImportUserIdentity(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"user", "app", "rotated-user"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			store := config.NewStore(dir)
			profile := uploadProfile(config.IdentityUser)
			if err := store.UpsertProfile(profile, true); err != nil {
				t.Fatal(err)
			}
			stdout := &bytes.Buffer{}
			reads, writes := 0, 0
			app := cli.New(cli.Options{Stdout: stdout, Stderr: &bytes.Buffer{}, Store: store,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Header.Get("Authorization") != "Bearer user-token" || req.Header.Get("X-Qfei-Identity") != "user" {
						t.Fatal("wrong user identity")
					}
					if req.Method == http.MethodGet {
						reads++
						return jsonResponse(`{"code":0,"data":{"columns_headers":[{"id":"c","name":"金额","type":1,"table_cell_content_type":"NUMBER"}]}}`), nil
					}
					writes++
					return jsonResponse(`{"code":0,"data":{"table_row_id":"new-row"}}`), nil
				})},
			})
			if err := app.Run(context.Background(), []string{"rule", "table", "import", "plan", "--as", "user", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--data", `{"rows":[{"cells":{"c":1}}]}`}); err != nil {
				t.Fatal(err)
			}
			id := decodeJSONObject(t, stdout.Bytes())["plan_id"].(string)
			data, err := os.ReadFile(filepath.Join(dir, "approval-matrix-plans", id+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "user-token") {
				t.Fatal("raw token persisted")
			}
			identity := mode
			if mode == "rotated-user" {
				identity = "user"
				profile.Identities.User.Token.AccessToken = "different-user-token"
				if err := store.UpsertProfile(profile, true); err != nil {
					t.Fatal(err)
				}
			}
			stdout.Reset()
			err = app.Run(context.Background(), []string{"rule", "table", "import", "apply", "--as", identity, "--plan-id", id})
			if mode == "user" && err != nil {
				t.Fatal(err)
			}
			if mode != "user" && err == nil {
				t.Fatal("cross-identity plan apply error = nil, want invalidated error")
			}
			result := decodeJSONObject(t, stdout.Bytes())
			if mode == "user" {
				if result["status"] != "success" || writes != 1 || reads != 2 {
					t.Fatalf("result=%v writes=%d reads=%d", result, writes, reads)
				}
			} else if result["status"] != "invalidated" || writes != 0 || reads != 1 {
				t.Fatalf("cross-identity plan executed: %v", result)
			}
		})
	}
}

/*
TestApprovalMatrixImportStopsAndInvalidates 验证远端类型变化、不确定写入、限流和局部确认均阻止未确认的后续写入。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestApprovalMatrixImportStopsAndInvalidates(t *testing.T) {
	for _, scenario := range []string{"changed_columns", "uncertain", "malformed", "rate_limit", "partial_confirmation", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}
			stdout := &bytes.Buffer{}
			reads, writes := 0, 0
			app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodGet {
					reads++
					kind := "NUMBER"
					if scenario == "changed_columns" && reads > 1 {
						kind = "STRING"
					}
					return jsonResponse(`{"code":0,"data":{"columns_headers":[{"id":"c","name":"金额","type":1,"table_cell_content_type":"` + kind + `"}]}}`), nil
				}
				writes++
				response := jsonResponse(`{"code":0,"data":{"table_row_id":"r"}}`)
				if scenario == "malformed" {
					response = jsonResponse(`{"message":"missing result"}`)
				}
				if scenario == "uncertain" {
					response.StatusCode = 500
				}
				if scenario == "rate_limit" {
					response.StatusCode = 429
				}
				return response, nil
			})}})
			args := []string{"rule", "table", "import", "plan", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--profile", "contract", "--as", "app", "--data", `{"rows":[{"cells":{"金额":1}},{"cells":{"金额":2}}]}`}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			id := decodeApprovalMatrixOutput(t, stdout.Bytes())["plan_id"].(string)
			apply := []string{"rule", "table", "import", "apply", "--plan-id", id, "--as", "app"}
			if scenario == "partial_confirmation" {
				apply = append(apply, "--rows", "2")
			}
			if scenario == "cancel" {
				if err := app.Run(context.Background(), []string{"rule", "table", "import", "cancel", "--plan-id", id}); err != nil {
					t.Fatal(err)
				}
			}
			stdout.Reset()
			err := app.Run(context.Background(), apply)
			if scenario == "changed_columns" && err == nil {
				t.Fatal("Run(apply) error = nil, want invalidated error")
			}
			if scenario != "changed_columns" && err != nil {
				t.Fatal(err)
			}
			result := decodeApprovalMatrixOutput(t, stdout.Bytes())
			want := map[string]string{"changed_columns": "invalidated", "uncertain": "needs_input", "malformed": "needs_input", "rate_limit": "paused", "partial_confirmation": "paused", "cancel": "cancelled"}[scenario]
			if result["status"] != want {
				t.Fatalf("status=%v want=%s", result["status"], want)
			}
			wantWrites := 1
			if scenario == "changed_columns" || scenario == "cancel" {
				wantWrites = 0
			}
			if writes != wantWrites {
				t.Fatalf("writes=%d want=%d", writes, wantWrites)
			}
			if scenario == "uncertain" || scenario == "malformed" {
				if err := app.Run(context.Background(), apply); err != nil {
					t.Fatal(err)
				}
				if writes != 1 {
					t.Fatal("uncertain request replayed")
				}
			}
		})
	}
}

/*
TestApprovalMatrixImportBackendValueTypes 验证数字范围、资源 ID 精度和 null 清空语义。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestApprovalMatrixImportBackendValueTypes(t *testing.T) {
	for _, tc := range []struct {
		kind, value string
		valid       bool
	}{
		{"NUMBER", "2147483647", true}, {"NUMBER", "2147483648", true}, {"NUMBER", "1.5", true},
		{"NUMBER", "123456789012345678901234567890.12345678", true},
		{"NUMBER", "0.123456789", false}, {"NUMBER", "1e30", false}, {"NUMBER", "1e-9", false},
		{"BOOLEAN", "null", true}, {"STRING", "null", true},
		{"EMPLOYEE_COLLECTION", `["7113921696628736004"]`, true},
		{"DEPARTMENT_COLLECTION", `["od-1b1b803a7df98989bf457d9ba203c350"]`, true},
		{"DEPARTMENT_COLLECTION", `[1018399484738012242]`, false},
		{"DEPARTMENT_COLLECTION", `["1018399484738012242"]`, false},
		{"EMPLOYEE_COLLECTION", `["ou_zhangsan"]`, false},
		{"EMPLOYEE_COLLECTION", `[]`, true},
	} {
		store := config.NewStore(t.TempDir())
		if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
			t.Fatal(err)
		}
		stdout := &bytes.Buffer{}
		app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatal("plan wrote data")
			}
			return jsonResponse(`{"code":0,"data":{"columns_headers":[{"id":"c","name":"c","type":1,"table_cell_content_type":"` + tc.kind + `"}]}}`), nil
		})}})
		if err := app.Run(context.Background(), []string{"rule", "table", "import", "plan", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--as", "app", "--data", `{"rows":[{"cells":{"c":` + tc.value + `}}]}`}); err != nil {
			t.Fatal(err)
		}
		status := decodeApprovalMatrixOutput(t, stdout.Bytes())["status"]
		if (status == "needs_confirmation") != tc.valid {
			t.Fatalf("%s %s: %v", tc.kind, tc.value, status)
		}
	}
}

/*
decodeApprovalMatrixOutput 将命令 JSON 输出解析成测试可断言的对象。
入参 t（*testing.T）为 Go 测试上下文，data（[]byte）为命令输出字节。
返回值为结构化输出对象；解析失败通过 t.Fatalf 报告。
*/
func decodeApprovalMatrixOutput(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var output map[string]any
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("decode output error = %v, data = %s", err, data)
	}
	return output
}
