package cli_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestReviewWorkflowSharesCommandTraceAcrossRequests(t *testing.T) {
	t.Parallel()
	var headers []http.Header
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := newDebugLoggingApp(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		headers = append(headers, req.Header.Clone())
		switch {
		case strings.HasSuffix(req.URL.Path, "/file/uploadByUrl"):
			return jsonResponse(`{"code":200,"data":{"fileId":"123","businessId":"test","fileHash":"` + strings.Repeat("a", 64) + `"}}`), nil
		case strings.HasSuffix(req.URL.Path, "/subjects"):
			return jsonResponse(`{"code":200,"data":{"subjects":[]}}`), nil
		case strings.HasSuffix(req.URL.Path, "/startReview"):
			return jsonResponse(`{"code":200,"data":{"taskId":"456"}}`), nil
		default:
			t.Fatalf("unexpected request %s", req.URL.Path)
			return nil, nil
		}
	}), stdout, stderr)
	body := `{"source":{"type":"url","fileUrl":"https://example.test/contract","name":"contract.pdf"},"config":` + alignmentConfig + `,"extractSubjects":true,"wait":false}`
	if err := app.Run(context.Background(), []string{"review", "run", "--data", body, "--debug", "--output", "json"}); err != nil {
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
