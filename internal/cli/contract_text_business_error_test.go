package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestContractTextBusinessFailureReturnsNonzeroAndPreservesResponse(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body, code string
	}{
		{"conversion failure", `{"code":10002106,"msg":"合同文本未转换成功","success":false}`, "10002106"},
		{"false with zero code", `{"code":0,"msg":"合同文本未转换成功","success":false}`, "0"},
		{"nonzero overrides true", `{"code":10002106,"msg":"合同文本未转换成功","success":true}`, "10002106"},
		{"nonzero without boolean", `{"code":10002106,"msg":"合同文本未转换成功"}`, "10002106"},
	} {
		for _, raw := range []bool{false, true} {
			t.Run(test.name+map[bool]string{false: "/json", true: "/raw"}[raw], func(t *testing.T) {
				t.Parallel()
				var stdout, stderr bytes.Buffer
				app, headers, calls := contractTextBusinessFixture(t, test.body, &stdout, &stderr)
				args := contractTextBusinessArgs()
				if raw {
					args = append(args, "--raw")
				}
				code := app.Execute(context.Background(), args)
				if code != 1 {
					t.Errorf("HTTP 200 business failure exit=%d, want 1; stdout=%s", code, &stdout)
				}
				if *calls != 1 {
					t.Errorf("business failure was retried: calls=%d", *calls)
				}
				if raw && stdout.String() != test.body {
					t.Errorf("raw business response changed: %s", &stdout)
				}
				var want, actual any
				if json.Unmarshal([]byte(test.body), &want) != nil || json.Unmarshal(stdout.Bytes(), &actual) != nil || !bytes.Equal(contractTextBusinessJSON(t, want), contractTextBusinessJSON(t, actual)) {
					t.Errorf("business details were lost: %s", &stdout)
				}
				var failure struct {
					OK    *bool `json:"ok"`
					Error struct {
						Type, Message string
						TraceID       string `json:"trace_id"`
					}
				}
				lines := bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n"))
				for _, line := range lines {
					if !json.Valid(line) {
						t.Errorf("debug/error record is not JSON: %s", line)
					}
				}
				if len(lines) == 0 || json.Unmarshal(lines[len(lines)-1], &failure) != nil || failure.OK == nil || *failure.OK || failure.Error.Type != "command" {
					t.Errorf("missing final business error JSON: %s", &stderr)
				}
				trace := headers.Get("X-Log-Id")
				if len(trace) != 32 || failure.Error.TraceID != trace || !strings.Contains(headers.Get("traceparent"), "-"+trace+"-") || !strings.Contains(failure.Error.Message, "business code "+test.code) {
					t.Errorf("business error lost code or request Trace: trace=%q error=%+v", trace, failure.Error)
				}
				if strings.Contains(stderr.String(), "合同文本未转换成功") {
					t.Error("server message must remain in the response rather than copied into diagnostic logs")
				}
			})
		}
	}
}

func TestContractTextBusinessSuccessKeepsZeroExit(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"code":0,"success":true,"data":{"text":"合同文本","contract_id":"1195681378344108361"}}`, `{"code":0,"data":{"text":"合同文本"}}`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			app, _, calls := contractTextBusinessFixture(t, body, &stdout, &stderr)
			if code := app.Execute(context.Background(), contractTextBusinessArgs()); code != 0 || *calls != 1 {
				t.Fatalf("successful text exit=%d calls=%d stderr=%s", code, *calls, &stderr)
			}
			if !strings.Contains(stdout.String(), "合同文本") || strings.Contains(stdout.String(), "trace_id") || strings.Contains(stderr.String(), `"ok":false`) {
				t.Fatalf("successful business response changed: stdout=%s stderr=%s", &stdout, &stderr)
			}
		})
	}
}

func TestContractTextBusinessErrorPreservesOutputFailureAndTrace(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("contract text output failed")
	var stderr bytes.Buffer
	app, headers, calls := contractTextBusinessFixture(t, `{"code":10002106,"success":false}`, commandTraceFailWriter{sentinel}, &stderr)
	err := app.Run(context.Background(), contractTextBusinessArgs())
	var traced *openplatform.TraceError
	if !errors.Is(err, sentinel) || !errors.As(err, &traced) || traced.TraceID != headers.Get("X-Log-Id") || !strings.Contains(err.Error(), "business code 10002106") || *calls != 1 {
		t.Fatalf("business/output error chain lost: err=%v calls=%d", err, *calls)
	}
}

func contractTextBusinessArgs() []string {
	return []string{"contract", "text", "1195681378344108361", "--limit", "500", "--profile", "contract", "--as", "user", "--debug", "--output", "json"}
}

func contractTextBusinessFixture(t *testing.T, body string, stdout io.Writer, stderr *bytes.Buffer) (*cli.App, http.Header, *int) {
	t.Helper()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatal(err)
	}
	headers, calls := make(http.Header), 0
	app := cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: stderr,
		LookupEnv:          func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Method != http.MethodGet || request.URL.Path != "/open-apis/contract/v1/mcp/contracts/1195681378344108361/text" || request.URL.Query().Get("limit") != "500" {
				t.Fatalf("text request changed: %s %s", request.Method, request.URL)
			}
			for key, values := range request.Header {
				headers[key] = append([]string(nil), values...)
			}
			return jsonResponse(body), nil
		})},
	})
	return app, headers, &calls
}

func contractTextBusinessJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
