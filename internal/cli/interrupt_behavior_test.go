package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestImportCancellationPreservesPersistedOutcomeAndErrorChain(t *testing.T) {
	t.Parallel()
	for _, during := range []string{"write", "readback"} {
		t.Run(during, func(t *testing.T) {
			t.Parallel()
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writes := 0
			var writeTrace string
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case strings.HasSuffix(req.URL.Path, "/column_headers"):
					return importSafetyColumns(), nil
				case req.Method == http.MethodPost:
					writes++
					writeTrace = req.Header.Get("X-Log-Id")
					if during == "write" {
						cancel()
						return nil, req.Context().Err()
					}
					return jsonResponse(`{"code":0,"data":{"table_row_id":"created-row"}}`), nil
				case strings.HasSuffix(req.URL.Path, "/created-row"):
					cancel()
					return nil, req.Context().Err()
				default:
					return jsonResponse(`{"code":0,"data":{"table_rows":[],"has_more":false}}`), nil
				}
			})
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: &bytes.Buffer{},
				HTTPClient:         &http.Client{Transport: transport},
				LookupEnv:          func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
				InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
			})
			args := []string{"rule", "table", "import", "plan", "--profile", "contract", "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--data", `{"rows":[{"cells":{"c":1}},{"cells":{"c":2}}]}`, "--output", "json"}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			planID := decodeApprovalMatrixOutput(t, stdout.Bytes())["plan_id"].(string)
			stdout.Reset()
			err := app.Run(ctx, []string{"rule", "table", "import", "apply", "--profile", "contract", "--as", "app", "--plan-id", planID, "--output", "json"})
			var traced *openplatform.TraceError
			if !errors.Is(err, context.Canceled) || !errors.As(err, &traced) || traced.TraceID != writeTrace {
				t.Fatalf("cancellation/trace error was swallowed: %v", err)
			}
			var uncertain *openplatform.UncertainWriteError
			if errors.As(err, &uncertain) != (during == "write") || writes != 1 {
				t.Fatalf("write uncertainty or retry behavior changed: writes=%d err=%v", writes, err)
			}
			body, readErr := os.ReadFile(filepath.Join(filepath.Dir(store.Path()), "approval-matrix-plans", planID+".json"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			var plan struct {
				Status     string `json:"status"`
				Operations []struct {
					Status string `json:"status"`
				} `json:"operations"`
			}
			if err := json.Unmarshal(body, &plan); err != nil {
				t.Fatal(err)
			}
			state, status := "uncertain", "needs_input"
			if during == "readback" {
				state, status = "unverified", "needs_verification"
			}
			if plan.Status != status || plan.Operations[0].Status != state || plan.Operations[1].Status != "pending" {
				t.Fatalf("unsafe saved plan after cancellation: %s", body)
			}
			if got := decodeApprovalMatrixOutput(t, stdout.Bytes())["status"]; got != status {
				t.Fatalf("business output lost: %v", got)
			}
		})
	}
}
