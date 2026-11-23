package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppDownloadBusinessResponses(t *testing.T) {
	const rejected = `{"code":110129,"success":false,"msg":"private-file-name","data":null}`
	for _, tc := range []struct {
		name, body, contentType, disposition string
		failure                              bool
	}{
		{"business failure", rejected, "application/json", "", true},
		{"charset", rejected, "application/json; charset=UTF-8", "", true},
		{"json suffix", rejected, "application/problem+json", "", true},
		{"missing type", rejected, "", "", true},
		{"BOM without type", "\xef\xbb\xbf" + rejected, "", "", true},
		{"generic type", rejected, "application/octet-stream", "", true},
		{"bare disposition", rejected, "application/json", "attachment", true},
		{"malformed disposition", rejected, "application/json", "attachment; filename=", true},
		{"nonzero without boolean", `{"code":110129,"msg":"private-file-name"}`, "application/json", "", true},
		{"false with zero", `{"code":0,"success":false,"msg":"private-file-name"}`, "application/json", "", true},
		{"false without message", `{"code":110129,"success":false}`, "application/json", "", true},
		{"successful envelope", `{"code":0,"success":true,"data":{"name":"document"}}`, "application/json", "", false},
		{"ordinary JSON", `{"code":110129,"name":"document","success":true}`, "application/json", "", false},
		{"BOM JSON file", "\xef\xbb\xbf{\"notes\":\"document\"}\n", "application/json", "", false},
		{"JSON array", `[{"code":110129,"success":false}]`, "application/json", "", false},
		{"text file", "plain text attachment", "text/plain", "", false},
		{"PDF", "%PDF-1.4\nbody", "application/pdf", "", false},
		{"JSON attachment", rejected, "application/json", `attachment; filename="report.json"`, false},
		{"inline JSON file", rejected, "application/json", `inline; filename="report.json"`, false},
		{"encoded filename", rejected, "application/json", "attachment; filename*=UTF-8''report%20data.json", false},
		{"binary JSON attachment", rejected, "application/octet-stream", `attachment; filename="report.json"`, false},
	} {
		for _, command := range [][]string{{"contract", "download-file", "0"}, {"mdm", "file", "download", "0"}, {"contract", "cooperation", "file", "download", "0"}} {
			for _, mode := range []string{"new", "replace", "raw"} {
				t.Run(tc.name+"/"+strings.Join(command, "-")+"/"+mode, func(t *testing.T) {
					target := filepath.Join(t.TempDir(), "saved.pdf")
					if mode == "replace" {
						if err := os.WriteFile(target, []byte("original PDF"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					var stdout, stderr bytes.Buffer
					calls, trace := 0, ""
					app := newDebugLoggingApp(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
						calls++
						trace = r.Header.Get("X-Log-Id")
						headers := make(http.Header)
						headers.Set("Content-Type", tc.contentType)
						headers.Set("Content-Disposition", tc.disposition)
						return &http.Response{StatusCode: 200, Header: headers, ContentLength: int64(len(tc.body)), Body: io.NopCloser(strings.NewReader(tc.body))}, nil
					}), &stdout, &stderr)
					args := append(append([]string{}, command...), "--as", "app")
					if mode == "raw" {
						args = append(args, "--raw")
					} else {
						args = append(args, "--output-file", target, "--output", "json", "--debug")
						if mode == "replace" {
							args = append(args, "--force")
						}
					}
					status := app.Execute(context.Background(), args)
					if (status != 0) != tc.failure || calls != 1 {
						t.Errorf("exit=%d calls=%d failure=%v stdout=%s stderr=%s", status, calls, tc.failure, &stdout, &stderr)
					}
					if tc.failure {
						if stdout.Len() != 0 || !strings.Contains(stderr.String(), "business code") || !strings.Contains(stderr.String(), trace) || trace == "" {
							t.Errorf("failure output/trace missing: stdout=%s stderr=%s", &stdout, &stderr)
						}
						if strings.Contains(stderr.String(), "private-file-name") {
							t.Error("business response leaked into diagnostics")
						}
						if mode != "raw" {
							for _, line := range bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n")) {
								if !json.Valid(line) {
									t.Errorf("invalid JSON diagnostic: %s", line)
								}
							}
						}
					}
					if mode == "raw" {
						if !tc.failure && stdout.String() != tc.body {
							t.Error("raw file content changed")
						}
						return
					}
					body, err := os.ReadFile(target)
					if tc.failure && mode == "new" {
						if !errors.Is(err, os.ErrNotExist) {
							t.Errorf("failed download created a file: %q %v", body, err)
						}
					} else {
						want := tc.body
						if tc.failure {
							want = "original PDF"
						}
						if err != nil || string(body) != want {
							t.Errorf("file content=%q want=%q err=%v", body, want, err)
						}
					}
					parts, err := filepath.Glob(target + ".part-*")
					if err != nil || len(parts) != 0 {
						t.Errorf("temporary files left: %v %v", parts, err)
					}
				})
			}
		}
	}
}
