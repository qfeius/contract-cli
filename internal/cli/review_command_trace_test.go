package cli_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestReviewResultSharesCommandTraceAcrossRequests(t *testing.T) {
	t.Parallel()
	var headers []http.Header
	statusCalls := 0
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := newDebugLoggingApp(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		headers = append(headers, req.Header.Clone())
		switch {
		case strings.HasSuffix(req.URL.Path, "/status"):
			statusCalls++
			if statusCalls == 1 {
				return jsonResponse(`{"code":200,"data":{"status":"running"}}`), nil
			}
			return jsonResponse(`{"code":200,"data":{"status":"success"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/info"):
			return jsonResponse(`{"code":200,"data":{"ruleResult":[]}}`), nil
		default:
			t.Fatalf("unexpected request %s", req.URL.Path)
			return nil, nil
		}
	}), stdout, stderr)
	if err := app.Run(context.Background(), []string{"review", "task", "result", "--task-id", "456", "--business-id", "test", "--interval", "1ms", "--debug", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if len(headers) != 3 {
		t.Fatalf("workflow request count = %d", len(headers))
	}
	id := assertDebugTraceHeaders(t, headers)
	if !strings.Contains(stderr.String(), `"msg":"command started"`) || !strings.Contains(stderr.String(), `"trace_id":"`+id+`"`) {
		t.Fatalf("missing command trace diagnostics: %s", stderr)
	}
}
