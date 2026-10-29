package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestCommandTraceSharesImportPlanRequestsAndIsolatesRuns(t *testing.T) {
	t.Parallel()
	var headers []http.Header
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		headers = append(headers, req.Header.Clone())
		return commandTracePlanResponse(t, req), nil
	})
	stdout, logs := &bytes.Buffer{}, &bytes.Buffer{}
	app := newCommandTraceApp(t, transport, stdout, logs)
	for i := 0; i < 3; i++ {
		if i == 2 {
			app = newCommandTraceApp(t, transport, stdout, logs)
		}
		stdout.Reset()
		if err := app.Run(context.Background(), commandTracePlanArgs()); err != nil {
			t.Fatalf("Run(plan %d) error = %v", i, err)
		}
		if got := decodeApprovalMatrixOutput(t, stdout.Bytes())["status"]; got != "needs_confirmation" {
			t.Fatalf("plan status = %v", got)
		}
	}
	assertCommandTraceGroups(t, headers, 3)
	assertCommandBoundaryTraceLogs(t, logs.Bytes(), headers)
}

func newCommandTraceApp(t *testing.T, transport roundTripFunc, stdout io.Writer, logs *bytes.Buffer) *cli.App {
	t.Helper()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	return cli.New(cli.Options{
		Stdout: stdout, Stderr: logs, Store: store,
		Logger:             slog.New(slog.NewJSONHandler(logs, nil)),
		HTTPClient:         &http.Client{Transport: transport},
		LookupEnv:          func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
	})
}

func TestCommandTracePreservesBusinessOutputAndWriterErrors(t *testing.T) {
	t.Parallel()
	stdout, logs := &bytes.Buffer{}, &bytes.Buffer{}
	var id string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		id = req.Header.Get("X-Log-Id")
		return jsonResponse(`{"code":99,"data":{"unchanged":true}}`), nil
	})
	args := []string{"contract", "search", "--profile", "contract", "--as", "user", "--output", "json"}
	err := newCommandTraceApp(t, transport, stdout, logs).Run(context.Background(), args)
	var traced *openplatform.TraceError
	if !errors.As(err, &traced) || traced.TraceID != id || strings.Count(err.Error(), "trace_id=") != 1 {
		t.Fatalf("business failure trace = %v", err)
	}
	var body map[string]any
	if json.Unmarshal(stdout.Bytes(), &body) != nil || body["code"] != float64(99) || strings.Contains(stdout.String(), "trace_id") {
		t.Fatalf("business stdout changed: %s", stdout.String())
	}
	sentinel := errors.New("output unavailable")
	err = newCommandTraceApp(t, transport, commandTraceFailWriter{sentinel}, logs).Run(context.Background(), args)
	if !errors.Is(err, sentinel) || !errors.As(err, &traced) || traced.TraceID != id || !strings.Contains(err.Error(), "business code 99") {
		t.Fatalf("joined business/writer error changed: %v", err)
	}
}

type commandTraceFailWriter struct{ err error }

func (w commandTraceFailWriter) Write([]byte) (int, error) { return 0, w.err }

func commandTracePlanArgs() []string {
	return []string{"rule", "table", "import", "plan", "--profile", "contract", "--as", "app",
		"--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1",
		"--data", `{"rows":[{"cells":{"column-amount":null}}]}`, "--output", "json"}
}

func commandTracePlanResponse(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	if req.Method != http.MethodGet {
		t.Fatalf("unexpected write: %s %s", req.Method, req.URL.Path)
	}
	if strings.HasSuffix(req.URL.Path, "/column_headers") {
		return jsonResponse(`{"code":0,"data":{"columns_headers":[{"id":"column-amount","name":"Amount","type":1,"table_cell_content_type":"NUMBER"}]}}`)
	}
	if !strings.HasSuffix(req.URL.Path, "/table_rows") {
		t.Fatalf("unexpected path: %s", req.URL.Path)
	}
	if req.URL.Query().Get("page_token") == "" {
		return jsonResponse(`{"code":0,"data":{"table_rows":[{"id":"one","table_cells":[]}],"has_more":true,"page_token":"next"}}`)
	}
	return jsonResponse(`{"code":0,"data":{"table_rows":[{"id":"two","table_cells":[]}],"has_more":false}}`)
}

func assertCommandTraceGroups(t *testing.T, headers []http.Header, groupSize int) {
	t.Helper()
	if len(headers) != groupSize*3 {
		t.Fatalf("request count = %d, want %d", len(headers), groupSize*3)
	}
	pattern := regexp.MustCompile(`^00-([0-9a-f]{32})-([0-9a-f]{16})-01$`)
	seenTraces, seenParents := map[string]bool{}, map[string]bool{}
	for start := 0; start < len(headers); start += groupSize {
		id := headers[start].Get("X-Log-Id")
		if seenTraces[id] {
			t.Fatalf("trace ID leaked across command runs: %s", id)
		}
		seenTraces[id] = true
		for _, h := range headers[start : start+groupSize] {
			parts := pattern.FindStringSubmatch(h.Get("traceparent"))
			if parts == nil || parts[1] != id || h.Get("X-Log-Id") != id {
				t.Fatalf("same command generated different trace IDs: first=%s headers=%v", id, h)
			}
			if seenParents[parts[2]] {
				t.Fatalf("parent ID reused: %s", parts[2])
			}
			seenParents[parts[2]] = true
		}
	}
}

func assertCommandBoundaryTraceLogs(t *testing.T, logs []byte, headers []http.Header) {
	t.Helper()
	started, completed := map[string]bool{}, map[string]bool{}
	for _, line := range bytes.Split(bytes.TrimSpace(logs), []byte("\n")) {
		if bytes.Count(line, []byte(`"trace_id":`)) > 1 {
			t.Fatalf("duplicate trace_id fields: %s", line)
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		id, _ := record["trace_id"].(string)
		switch record["msg"] {
		case "command started":
			started[id] = true
		case "command completed":
			completed[id] = true
		}
	}
	for i := 0; i < len(headers); i += 3 {
		id := headers[i].Get("X-Log-Id")
		if !started[id] || !completed[id] {
			t.Fatalf("command boundary logs missing trace ID %s", id)
		}
	}
}
