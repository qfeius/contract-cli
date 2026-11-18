package openplatform_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestFileDownloadValidationBoundaries(t *testing.T) {
	const failure = `{"code":110129,"success":false,"msg":"private-file-name"}`
	for _, tc := range []struct {
		name, body, disposition, method string
		limit, length                   int64
		generic, wantError, emptyOutput bool
		readError                       error
	}{
		{name: "business failure", body: failure, length: -1, wantError: true, emptyOutput: true},
		{name: "BOM business failure", body: "\xef\xbb\xbf" + failure, length: -1, wantError: true, emptyOutput: true},
		{name: "generic stream unchanged", body: failure, length: -1, generic: true},
		{name: "JSON stream limit", body: failure, length: -1, limit: 20, wantError: true, emptyOutput: true},
		{name: "named JSON ignores API size limit", body: failure, length: -1, limit: 20, disposition: `attachment; filename="report.json"`},
		{name: "successful unknown length JSON", body: `{"code":0,"data":[]}`, length: -1},
		{name: "JSON trailing newline preserved", body: "{\"notes\":\"file\"}\n", length: -1},
		{name: "non-envelope JSON", body: `{"success":false,"code":"customer-code"}`, length: -1},
		{name: "malformed JSON file", body: `{not JSON`, length: -1},
		{name: "short attachment", body: failure, length: 200, disposition: `attachment; filename="report.json"`, wantError: true},
		{name: "short ordinary JSON", body: `{}`, length: 200, wantError: true},
		{name: "long ordinary JSON", body: `{}`, length: 1, wantError: true},
		{name: "HEAD keeps headers", method: http.MethodHead, length: 200},
		{name: "broken JSON read", body: `{"code":`, length: -1, readError: io.ErrUnexpectedEOF, wantError: true, emptyOutput: true},
		{name: "cancelled JSON read", body: `{"code":`, length: -1, readError: context.Canceled, wantError: true, emptyOutput: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, refreshes := 0, 0
			client := openplatform.New(openplatform.Options{MaxResponseBytes: tc.limit, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				var body io.Reader = strings.NewReader(tc.body)
				if tc.readError != nil {
					body = io.MultiReader(body, streamErrorReader{tc.readError})
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}, "Content-Disposition": {tc.disposition}}, Body: io.NopCloser(body), ContentLength: tc.length}, nil
			})}})
			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			rc := openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityApp, RefreshAccessToken: func(context.Context, string) (string, error) {
				refreshes++
				return "unused", nil
			}}
			var out bytes.Buffer
			response, err := client.DoStream(context.Background(), rc, openplatform.Request{Method: method, Path: "/open-apis/contract/v1/files/0", FileDownload: !tc.generic}, &out)
			if (err != nil) != tc.wantError || calls != 1 || refreshes != 0 {
				t.Fatalf("error=%v calls=%d refreshes=%d", err, calls, refreshes)
			}
			if err != nil {
				var trace *openplatform.TraceError
				if !errors.As(err, &trace) || trace.TraceID == "" || trace.TraceID != response.TraceID || strings.Contains(err.Error(), "private-file-name") {
					t.Errorf("unsafe or missing error trace: %v", err)
				}
			}
			if tc.readError != nil && !errors.Is(err, tc.readError) {
				t.Errorf("read error lost: %v", err)
			}
			want := tc.body
			if tc.emptyOutput {
				want = ""
			}
			if out.String() != want {
				t.Errorf("output=%q want=%q", &out, want)
			}
		})
	}
}
