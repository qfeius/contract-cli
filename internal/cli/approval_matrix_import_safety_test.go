package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

/*
importSafetyRow 构造带金额单元格的规则行，供容量、快照和回读用例共享。
入参 id（string）为行 ID，amount（int）为精确整数金额；返回 string 为服务端行 JSON。
*/
func importSafetyRow(id string, amount int) string {
	return fmt.Sprintf(`{"id":%q,"table_cells":[{"table_column_id":"c","table_cell_content_type":"NUMBER","table_cell_content":{"number":%d}}]}`, id, amount)
}

/*
importSafetyColumns 构造固定金额列响应，测试只关注行状态变化。
入参为空；返回 *http.Response 为列头查询响应。
*/
func importSafetyColumns() *http.Response {
	return jsonResponse(`{"code":0,"data":{"columns_headers":[{"id":"c","name":"金额","type":1,"table_cell_content_type":"NUMBER"}]}}`)
}

/*
TestApprovalMatrixImportRowLimit 验证完整分页后的 2000 行边界，且 update 不占新增名额。
入参 t（*testing.T）为测试上下文；返回值为空，断言失败时报告测试错误。
*/
func TestApprovalMatrixImportRowLimit(t *testing.T) {
	for _, tc := range []struct {
		name, input, wantStatus string
		current                 int
	}{
		{"one_available", `{"rows":[{"cells":{"金额":1}}]}`, "needs_confirmation", 1999},
		{"over_limit", `{"rows":[{"cells":{"金额":1}},{"cells":{"金额":2}}]}`, "needs_input", 1999},
		{"update_at_limit", `{"rows":[{"operation":"update","row_id":"r0","cells":{"金额":2}}]}`, "needs_confirmation", 2000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}
			pages := 0
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatal("plan sent a write request")
				}
				if strings.HasSuffix(req.URL.Path, "/column_headers") {
					return importSafetyColumns(), nil
				}
				if !strings.HasSuffix(req.URL.Path, "/table_rows") || req.URL.Query().Get("page_size") != "100" {
					t.Fatalf("unexpected row request: %s", req.URL)
				}
				pages++
				start := 0
				if token := req.URL.Query().Get("page_token"); token != "" {
					var err error
					start, err = strconv.Atoi(token)
					if err != nil {
						t.Fatal(err)
					}
				}
				end := min(start+100, tc.current)
				rows := make([]string, 0, end-start)
				for i := start; i < end; i++ {
					rows = append(rows, importSafetyRow(fmt.Sprintf("r%d", i), 1))
				}
				more := end < tc.current
				return jsonResponse(fmt.Sprintf(`{"code":0,"data":{"table_rows":[%s],"has_more":%t,"page_token":%q}}`, strings.Join(rows, ","), more, strconv.Itoa(end))), nil
			})}})
			err := app.Run(context.Background(), []string{"rule", "table", "import", "plan", "--profile", "contract", "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--data", tc.input})
			if err != nil {
				t.Fatal(err)
			}
			result := decodeApprovalMatrixOutput(t, stdout.Bytes())
			if result["status"] != tc.wantStatus || pages != 20 {
				t.Fatalf("status=%v pages=%d", result["status"], pages)
			}
			if tc.wantStatus == "needs_input" {
				if result["current_rows"] != float64(tc.current) || result["planned_creates"] != float64(2) {
					t.Fatalf("capacity details=%v", result)
				}
			}
		})
	}
}

/*
TestApprovalMatrixImportInvalidatesChangedRows 验证计划后外部修改行内容时写请求被阻断。
入参 t（*testing.T）为测试上下文；返回值为空，断言失败时报告测试错误。
*/
func TestApprovalMatrixImportInvalidatesChangedRows(t *testing.T) {
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	var amount atomic.Int32
	amount.Store(1)
	writes := 0
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			writes++
			return jsonResponse(`{"code":0}`), nil
		}
		if strings.HasSuffix(req.URL.Path, "/column_headers") {
			return importSafetyColumns(), nil
		}
		return jsonResponse(fmt.Sprintf(`{"code":0,"data":{"table_rows":[%s],"has_more":false}}`, importSafetyRow("r0", int(amount.Load())))), nil
	})}})
	if err := app.Run(context.Background(), []string{"rule", "table", "import", "plan", "--profile", "contract", "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--data", `{"rows":[{"operation":"update","row_id":"r0","cells":{"金额":2}}]}`}); err != nil {
		t.Fatal(err)
	}
	id := decodeApprovalMatrixOutput(t, stdout.Bytes())["plan_id"].(string)
	amount.Store(3)
	stdout.Reset()
	if err := app.Run(context.Background(), []string{"rule", "table", "import", "apply", "--plan-id", id, "--as", "app"}); err == nil {
		t.Fatal("changed row did not invalidate plan")
	}
	result := decodeApprovalMatrixOutput(t, stdout.Bytes())
	if result["status"] != "invalidated" || writes != 0 {
		t.Fatalf("result=%v writes=%d", result, writes)
	}
}

/*
TestApprovalMatrixImportPauseAfterCurrentWrite 验证暂停在当前行回读后生效，恢复后立即输出并持久化可续跑状态。
入参 t（*testing.T）为测试上下文；返回值为空，断言失败时报告测试错误。
*/
func TestApprovalMatrixImportPauseAfterCurrentWrite(t *testing.T) {
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var writes, completed atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/column_headers") {
			return importSafetyColumns(), nil
		}
		if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/table_rows") {
			rows := []string{}
			for i := int32(1); i <= completed.Load(); i++ {
				rows = append(rows, importSafetyRow(fmt.Sprintf("r%d", i), int(i)))
			}
			return jsonResponse(fmt.Sprintf(`{"code":0,"data":{"table_rows":[%s],"has_more":false}}`, strings.Join(rows, ","))), nil
		}
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/table_rows/r") {
			id := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
			value, _ := strconv.Atoi(strings.TrimPrefix(id, "r"))
			return jsonResponse(fmt.Sprintf(`{"code":0,"data":{"table_row":%s}}`, importSafetyRow(id, value))), nil
		}
		if req.Method == http.MethodPost {
			n := writes.Add(1)
			if n == 1 {
				started <- struct{}{}
				<-release
			}
			completed.Store(n)
			return jsonResponse(fmt.Sprintf(`{"code":0,"data":{"table_row_id":"r%d"}}`, n)), nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		return nil, nil
	})}
	planOutput := &bytes.Buffer{}
	planApp := cli.New(cli.Options{Store: store, Stdout: planOutput, Stderr: &bytes.Buffer{}, HTTPClient: client})
	if err := planApp.Run(context.Background(), []string{"rule", "table", "import", "plan", "--profile", "contract", "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--data", `{"rows":[{"cells":{"金额":1}},{"cells":{"金额":2}}]}`}); err != nil {
		t.Fatal(err)
	}
	id := decodeApprovalMatrixOutput(t, planOutput.Bytes())["plan_id"].(string)
	applyOutput := &bytes.Buffer{}
	applyApp := cli.New(cli.Options{Store: store, Stdout: applyOutput, Stderr: &bytes.Buffer{}, HTTPClient: client})
	done := make(chan error, 1)
	go func() {
		done <- applyApp.Run(context.Background(), []string{"rule", "table", "import", "apply", "--plan-id", id, "--profile", "contract", "--as", "app"})
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("first write did not start")
	}
	pauseOutput := &bytes.Buffer{}
	controlApp := cli.New(cli.Options{Store: store, Stdout: pauseOutput, Stderr: &bytes.Buffer{}, HTTPClient: client})
	if err := controlApp.Run(context.Background(), []string{"rule", "table", "import", "pause", "--plan-id", id}); err != nil {
		t.Fatal(err)
	}
	if decodeApprovalMatrixOutput(t, pauseOutput.Bytes())["pause_requested"] != true {
		t.Fatal("pause request was not persisted")
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("apply did not stop after current write")
	}
	if result := decodeApprovalMatrixOutput(t, applyOutput.Bytes()); result["status"] != "paused" || writes.Load() != 1 {
		t.Fatalf("paused result=%v writes=%d", result, writes.Load())
	}
	pauseOutput.Reset()
	if err := controlApp.Run(context.Background(), []string{"rule", "table", "import", "resume", "--plan-id", id}); err != nil {
		t.Fatal(err)
	}
	// 恢复只解除暂停并允许下次 apply，不应保留旧暂停状态或启动第二条写入。
	if result := decodeApprovalMatrixOutput(t, pauseOutput.Bytes()); result["status"] != "ready" || result["reason"] != "" || result["pause_requested"] != false || writes.Load() != 1 {
		t.Fatalf("resume result=%v writes=%d", result, writes.Load())
	}
	pauseOutput.Reset()
	if err := controlApp.Run(context.Background(), []string{"rule", "table", "import", "get", "--plan-id", id}); err != nil {
		t.Fatal(err)
	}
	if result := decodeApprovalMatrixOutput(t, pauseOutput.Bytes()); result["status"] != "ready" || result["reason"] != "" || result["pause_requested"] != false {
		t.Fatalf("persisted resume result=%v", result)
	}
	applyOutput.Reset()
	if err := applyApp.Run(context.Background(), []string{"rule", "table", "import", "apply", "--plan-id", id, "--profile", "contract", "--as", "app"}); err != nil {
		t.Fatal(err)
	}
	if result := decodeApprovalMatrixOutput(t, applyOutput.Bytes()); result["status"] != "success" || writes.Load() != 2 {
		t.Fatalf("resumed result=%v writes=%d", result, writes.Load())
	}
}

/*
TestApprovalMatrixImportReadbackRecovery 验证回读失败或不符时 resume 不绕过核验、不重放写入，并可只读核验恢复。
入参 t（*testing.T）为测试上下文；返回值为空，断言失败时报告测试错误。
*/
func TestApprovalMatrixImportReadbackRecovery(t *testing.T) {
	for _, mode := range []string{"read_error", "mismatch"} {
		t.Run(mode, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}
			var recoverRead atomic.Bool
			writes := 0
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.HasSuffix(req.URL.Path, "/column_headers") {
					return importSafetyColumns(), nil
				}
				if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/table_rows") {
					return jsonResponse(`{"code":0,"data":{"table_rows":[],"has_more":false}}`), nil
				}
				if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/table_rows/r1") {
					if !recoverRead.Load() && mode == "read_error" {
						response := jsonResponse(`{"code":500,"msg":"temporary read error"}`)
						response.StatusCode = http.StatusInternalServerError
						return response, nil
					}
					value := 1
					if !recoverRead.Load() {
						value = 2
					}
					return jsonResponse(fmt.Sprintf(`{"code":0,"data":{"table_row":%s}}`, importSafetyRow("r1", value))), nil
				}
				if req.Method == http.MethodPost {
					writes++
					return jsonResponse(`{"code":0,"data":{"table_row_id":"r1"}}`), nil
				}
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				return nil, nil
			})}})
			if err := app.Run(context.Background(), []string{"rule", "table", "import", "plan", "--profile", "contract", "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--data", `{"rows":[{"cells":{"金额":1}}]}`}); err != nil {
				t.Fatal(err)
			}
			id := decodeApprovalMatrixOutput(t, stdout.Bytes())["plan_id"].(string)
			apply := []string{"rule", "table", "import", "apply", "--plan-id", id, "--as", "app"}
			stdout.Reset()
			if err := app.Run(context.Background(), apply); err != nil {
				t.Fatal(err)
			}
			result := decodeApprovalMatrixOutput(t, stdout.Bytes())
			if result["status"] != "needs_verification" || writes != 1 {
				t.Fatalf("unverified result=%v writes=%d", result, writes)
			}
			stdout.Reset()
			if err := app.Run(context.Background(), []string{"rule", "table", "import", "resume", "--plan-id", id, "--as", "app"}); err != nil {
				t.Fatal(err)
			}
			if result := decodeApprovalMatrixOutput(t, stdout.Bytes()); result["status"] != "needs_verification" || writes != 1 {
				t.Fatalf("resume bypassed verification: result=%v writes=%d", result, writes)
			}
			stdout.Reset()
			if err := app.Run(context.Background(), apply); err != nil {
				t.Fatal(err)
			}
			if writes != 1 {
				t.Fatal("unverified write was replayed")
			}
			recoverRead.Store(true)
			stdout.Reset()
			if err := app.Run(context.Background(), []string{"rule", "table", "import", "verify", "--plan-id", id, "--as", "app"}); err != nil {
				t.Fatal(err)
			}
			if result := decodeApprovalMatrixOutput(t, stdout.Bytes()); result["status"] != "success" || writes != 1 {
				t.Fatalf("verified result=%v writes=%d", result, writes)
			}
		})
	}
}

/*
TestApprovalMatrixImportServerRowLimitStopsBatch 验证服务端 20302 容量兜底使计划失效且阻断后续写入。
入参 t（*testing.T）为测试上下文；返回值为空，断言失败时报告测试错误。
*/
func TestApprovalMatrixImportServerRowLimitStopsBatch(t *testing.T) {
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	writes := 0
	stdout := &bytes.Buffer{}
	app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/column_headers") {
			return importSafetyColumns(), nil
		}
		if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/table_rows") {
			return jsonResponse(`{"code":0,"data":{"table_rows":[],"has_more":false}}`), nil
		}
		if req.Method == http.MethodPost {
			writes++
			return jsonResponse(`{"code":20302,"msg":"row count exceeds threshold"}`), nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		return nil, nil
	})}})
	if err := app.Run(context.Background(), []string{"rule", "table", "import", "plan", "--profile", "contract", "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--data", `{"rows":[{"cells":{"金额":1}},{"cells":{"金额":2}}]}`}); err != nil {
		t.Fatal(err)
	}
	id := decodeApprovalMatrixOutput(t, stdout.Bytes())["plan_id"].(string)
	stdout.Reset()
	if err := app.Run(context.Background(), []string{"rule", "table", "import", "apply", "--plan-id", id, "--as", "app"}); err == nil {
		t.Fatal("server row limit did not invalidate plan")
	}
	if result := decodeApprovalMatrixOutput(t, stdout.Bytes()); result["status"] != "invalidated" || writes != 1 {
		t.Fatalf("result=%v writes=%d", result, writes)
	}
}
