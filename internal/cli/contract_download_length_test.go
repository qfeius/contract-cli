package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestContractDownloadUsesActualHTTPContentLength(t *testing.T) {
	for _, tc := range []struct {
		name, body, length   string
		metadataSize, status int
		chunked, wantError   bool
	}{
		{"stale encrypted size", "plaintext", "9", 1024, 200, false, false},
		{"stale zero size", "plaintext", "9", 0, 200, false, false},
		{"actual empty file", "", "0", 1024, 200, false, false},
		{"chunked fallback matches", "plaintext", "", 9, 200, true, false},
		{"chunked fallback mismatch", "plaintext", "", 20, 200, true, true},
		{"truncated actual response", "plaintext", "20", 9, 200, false, true},
		{"expired presigned URL", "denied", "6", 6, 403, false, true},
		{"storage error", "denied", "6", 6, 503, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("download leaked Authorization")
				}
				if tc.length != "" {
					w.Header().Set("Content-Length", tc.length)
				}
				w.WriteHeader(tc.status)
				if tc.chunked {
					w.(http.Flusher).Flush()
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			dir := t.TempDir()
			store := config.NewStore(dir)
			if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(dir, "download.pdf")
			if err := os.WriteFile(outputPath, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			client := server.Client()
			baseTransport := client.Transport
			client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.HasPrefix(req.URL.Path, "/open-apis/") {
					return jsonResponse(fmt.Sprintf(`{"code":0,"data":{"file_id":"1","file_size":%d,"download_url":%q}}`, tc.metadataSize, "https://storage.example.test/tenant/contract.pdf?X-Tos-Expires=300&X-Tos-Signature=test")), nil
				}
				// Route only this injected storage fixture to the local TLS server.
				local, parseErr := url.Parse(server.URL)
				if parseErr != nil {
					return nil, parseErr
				}
				request := req.Clone(req.Context())
				request.URL.Scheme, request.URL.Host = local.Scheme, local.Host
				return baseTransport.RoundTrip(request)
			})
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{Stdout: stdout, Stderr: io.Discard, Store: store, HTTPClient: client})
			err := app.Run(context.Background(), []string{"contract", "download-file", "1", "--contract", "1", "--as", "user", "--output-file", outputPath, "--force"})
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError %v", err, tc.wantError)
			}
			want := tc.body
			if tc.wantError {
				want = "original"
				if stdout.Len() != 0 {
					t.Fatal("failed download reported success")
				}
			}
			content, err := os.ReadFile(outputPath)
			if err != nil || string(content) != want {
				t.Fatalf("content = %q, err = %v", content, err)
			}
			if paths, err := filepath.Glob(outputPath + ".part-*"); err != nil || len(paths) != 0 {
				t.Fatalf("temporary files = %v: %v", paths, err)
			}
		})
	}
}
