package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

const uploadSessionURL = "https://files.example.test/files/upload_sessions/upl_secret/content"

func TestContractUploadMCPAttachmentWorkflow(t *testing.T) {
	for _, fileType := range []string{"reviewAttachment", "approveAttachment"} {
		for _, identity := range []string{"user", "default", "app"} {
			for _, output := range []string{"json", "yaml", "raw"} {
				t.Run(fileType+"/"+identity+"/"+output, func(t *testing.T) {
					runAttachmentUploadScenario(t, fileType, identity, output, "", nil, 0, "", false)
				})
			}
		}
	}
}

func TestContractUploadMCPAttachmentStopsOnInvalidResponse(t *testing.T) {
	for _, tc := range []struct {
		name, phase, body, want string
		status                  int
		uncertain               bool
	}{
		{"prepare business", "prepare", `{"code":110004,"msg":"upl_secret"}`, "110004", 200, false},
		{"prepare missing code", "prepare", `{"data":{}}`, "business code", 200, false},
		{"prepare invalid JSON", "prepare", `upl_secret`, "invalid", 200, false},
		{"prepare HTTP error", "prepare", `upl_secret`, "403", 403, false},
		{"prepare server error", "prepare", `upl_secret`, "不确定", 503, true},
		{"content business", "content", `{"code":110004,"msg":"upl_secret"}`, "110004", 200, false},
		{"content false", "content", `{"code":0,"data":false}`, "content", 200, true},
		{"content invalid JSON", "content", `upl_secret`, "invalid", 200, true},
		{"content invalid success type", "content", `{"code":0,"success":"upl_secret"}`, "invalid", 200, true},
		{"content HTTP error", "content", `upl_secret`, "404", 404, false},
		{"content server error", "content", `upl_secret`, "不确定", 503, true},
		{"content redirect", "content", "", "302", 302, false},
		{"commit business", "commit", `{"code":110004,"msg":"upl_secret"}`, "110004", 200, false},
		{"commit missing file", "commit", `{"code":0,"data":{}}`, "file_id", 200, true},
		{"commit numeric file", "commit", `{"code":0,"data":{"file_id":123}}`, "file_id", 200, true},
		{"commit invalid JSON", "commit", `upl_secret`, "invalid", 200, true},
		{"commit server error", "commit", `upl_secret`, "不确定", 503, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runAttachmentUploadScenario(t, "reviewAttachment", "user", "json", tc.phase, []byte(tc.body), tc.status, tc.want, tc.uncertain)
		})
	}
}

func TestContractUploadMCPAttachmentValidatesPrepareBeforeSendingFile(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   string
		value any
	}{
		{"missing upload id", "upload_id", ""},
		{"insecure URL", "upload_url", "http://files.example.test/upl_secret"},
		{"URL credentials", "upload_url", "https://user:upl_secret@files.example.test/content"},
		{"URL fragment", "upload_url", uploadSessionURL + "#upl_secret"},
		{"unsupported method", "upload_method", "PUT"},
		{"unexpected headers", "upload_headers", map[string]string{"Authorization": "Bearer upl_secret"}},
		{"expired session", "expires_at", "2000-01-01T00:00:00Z"},
		{"invalid expiry", "expires_at", "upl_secret"},
		{"zero size limit", "max_size", 0},
		{"file exceeds size limit", "max_size", 2},
	} {
		// t.TempDir includes the test name; keep response secrets out of the
		// local file path so the unchanged command-argument log cannot cause a false positive.
		t.Run(tc.name, func(t *testing.T) {
			data := validAttachmentPrepareData()
			data[tc.key] = tc.value
			body, err := json.Marshal(map[string]any{"code": 0, "data": data})
			if err != nil {
				t.Fatal(err)
			}
			runAttachmentUploadScenario(t, "approveAttachment", "user", "json", "prepare", body, 200, "upload", false)
		})
	}
}

func validAttachmentPrepareData() map[string]any {
	return map[string]any{
		"upload_id": "upl_secret", "upload_url": uploadSessionURL, "upload_method": "POST",
		"upload_headers": map[string]string{}, "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339Nano),
		"max_size": 1024,
	}
}

func runAttachmentUploadScenario(t *testing.T, fileType, identity, output, failPhase string, failBody []byte, failStatus int, wantError string, uncertain bool) {
	t.Helper()
	dir := t.TempDir()
	uploadPath := filepath.Join(dir, "local.pdf")
	if err := os.WriteFile(uploadPath, []byte("pdf bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(uploadSessionURL)
	jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "cookie-secret"}})
	var phases []string
	var requestHeaders []http.Header
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		phase := filepath.Base(req.URL.Path)
		phases = append(phases, phase)
		requestHeaders = append(requestHeaders, req.Header.Clone())
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s", req.Method)
		}
		if identity == "app" {
			if req.URL.Path != "/open-apis/contract/v1/files/upload" || req.Header.Get("Authorization") != "Bearer app-token" {
				t.Fatal("app route or token changed")
			}
			assertUploadMultipart(t, req, "附件.pdf", fileType, "pdf bytes")
			return jsonResponse(`{"code":0,"data":{"file_id":"9223372036854775806"}}`), nil
		}
		if phase != "content" {
			if req.URL.Path != "/open-apis/contract/v1/mcp/files/upload_sessions/"+phase || req.URL.RawQuery != "" || req.Header.Get("Authorization") != "Bearer user-token" {
				t.Fatal("invalid authenticated session request")
			}
		}
		var body string
		switch phase {
		case "prepare":
			var input map[string]any
			if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input["file_name"] != "附件.pdf" || input["file_type"] != fileType {
				t.Fatalf("prepare input = %v", input)
			}
			data, err := json.Marshal(map[string]any{"code": 0, "data": validAttachmentPrepareData()})
			if err != nil {
				t.Fatal(err)
			}
			body = string(data)
		case "content":
			if req.URL.String() != uploadSessionURL {
				t.Fatal("wrong content URL")
			}
			for _, h := range []string{"Authorization", "Cookie", "Referer"} {
				if req.Header.Get(h) != "" {
					t.Fatalf("content leaked %s", h)
				}
			}
			if stdout.Len() != 0 {
				t.Fatal("output before commit")
			}
			reader, err := req.MultipartReader()
			if err != nil {
				t.Fatal(err)
			}
			part, err := reader.NextPart()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatal(err)
			}
			if part.FormName() != "file" || part.FileName() != "附件.pdf" || string(data) != "pdf bytes" {
				t.Fatal("wrong content multipart")
			}
			if _, err := reader.NextPart(); err != io.EOF {
				t.Fatalf("unexpected multipart field: %v", err)
			}
			body = `{"code":0,"data":true}`
		case "commit":
			var input map[string]string
			if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if len(input) != 1 || input["upload_id"] != "upl_secret" {
				t.Fatal("wrong commit input")
			}
			if stdout.Len() != 0 {
				t.Fatal("output before commit")
			}
			body = `{"code":0,"data":{"file_id":"9223372036854775806"}}`
		default:
			t.Fatalf("unexpected upload path %s", req.URL.Path)
		}
		response := jsonResponse(body)
		if phase == failPhase {
			response.Body = io.NopCloser(bytes.NewReader(failBody))
			response.StatusCode = failStatus
			response.Header.Set("Location", "https://other.example.test/upl_secret")
		}
		return response, nil
	})
	app := cli.New(cli.Options{Stdout: stdout, Stderr: stderr, Logger: slog.New(slog.NewTextHandler(stderr, nil)), Store: store, HTTPClient: &http.Client{Transport: transport, Jar: jar}})
	args := []string{"contract", "upload-file", "--file", uploadPath, "--file-type", fileType, "--file-name", "附件.pdf", "--user-id", "ignored-user", "--user-id-type", "employee_id"}
	if identity != "default" {
		args = append(args, "--as", identity)
	}
	if output == "raw" {
		args = append(args, "--raw")
	} else {
		args = append(args, "--output", output)
	}
	err = app.Run(context.Background(), args)
	wantPhases := "prepare,content,commit"
	if identity == "app" {
		wantPhases = "upload"
	}
	if failPhase != "" {
		wantPhases = strings.Split(wantPhases, failPhase)[0] + failPhase
	}
	if strings.Join(phases, ",") != wantPhases {
		t.Fatalf("phases = %v, want %s", phases, wantPhases)
	}
	if wantError == "" {
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout.String(), "9223372036854775806") {
			t.Fatalf("missing file_id: %s", stdout)
		}
		traceID := requestHeaders[0].Get("X-Log-Id")
		parents := make(map[string]bool)
		for i, headers := range requestHeaders {
			parts := strings.Split(headers.Get("traceparent"), "-")
			if len(traceID) != 32 || len(parts) != 4 || parts[1] != traceID || headers.Get("X-Log-Id") != traceID {
				t.Fatalf("phase %s did not share the command trace: %v", phases[i], headers)
			}
			if len(parts[2]) != 16 || parents[parts[2]] {
				t.Fatalf("phase %s reused an HTTP attempt parent-id", phases[i])
			}
			parents[parts[2]] = true
		}
		if identity != "app" {
			for _, line := range strings.Split(stderr.String(), "\n") {
				if strings.Contains(line, "msg=\"user attachment") && strings.Count(line, "trace_id="+traceID) != 1 {
					t.Fatalf("attachment log must have one command trace field: %s", line)
				}
			}
		}
	} else {
		if err == nil || !strings.Contains(err.Error(), wantError) {
			t.Fatalf("error = %v, want %s", err, wantError)
		}
		var writeErr *openplatform.UncertainWriteError
		if errors.As(err, &writeErr) != uncertain {
			t.Fatalf("uncertain = %v, want %v: %v", writeErr != nil, uncertain, err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("failed upload emitted output: %s", stdout)
		}
	}
	for _, secret := range []string{"upl_secret", "user-token", "cookie-secret"} {
		if strings.Contains(fmt.Sprint(err)+stdout.String()+stderr.String(), secret) {
			t.Fatalf("upload leaked %s", secret)
		}
	}
}

func TestContractUploadMCPAttachmentNetworkFailureAndCancellation(t *testing.T) {
	for _, phase := range []string{"prepare", "content", "commit"} {
		for _, cancelRequest := range []bool{false, true} {
			t.Run(fmt.Sprint(phase, "/cancel=", cancelRequest), func(t *testing.T) {
				dir := t.TempDir()
				uploadPath := filepath.Join(dir, "file.pdf")
				if err := os.WriteFile(uploadPath, []byte("pdf"), 0o600); err != nil {
					t.Fatal(err)
				}
				store := config.NewStore(dir)
				if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				stdout, logs := &bytes.Buffer{}, &bytes.Buffer{}
				var phases []string
				app := cli.New(cli.Options{Stdout: stdout, Stderr: logs, Logger: slog.New(slog.NewTextHandler(logs, nil)), Store: store,
					HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						current := filepath.Base(req.URL.Path)
						phases = append(phases, current)
						if current == phase {
							if cancelRequest {
								cancel()
								return nil, req.Context().Err()
							}
							return nil, errors.New("connection lost: " + uploadSessionURL)
						}
						if current == "prepare" {
							body, err := json.Marshal(map[string]any{"code": 0, "data": validAttachmentPrepareData()})
							if err != nil {
								t.Fatal(err)
							}
							return jsonResponse(string(body)), nil
						}
						if current == "content" {
							if _, err := io.Copy(io.Discard, req.Body); err != nil {
								t.Fatal(err)
							}
							return jsonResponse(`{"code":0,"data":true}`), nil
						}
						t.Fatalf("unexpected phase %s", current)
						return nil, nil
					})},
				})
				err := app.Run(ctx, []string{"contract", "upload-file", "--as", "user", "--file", uploadPath, "--file-type", "reviewAttachment"})
				var uncertain *openplatform.UncertainWriteError
				if !errors.As(err, &uncertain) {
					t.Fatalf("want uncertain write: %v", err)
				}
				if cancelRequest && !errors.Is(err, context.Canceled) {
					t.Fatalf("lost cancellation cause: %v", err)
				}
				wantPhases := strings.Split("prepare,content,commit", phase)[0] + phase
				if strings.Join(phases, ",") != wantPhases {
					t.Fatalf("requests = %v, want %s", phases, wantPhases)
				}
				if stdout.Len() != 0 || strings.Contains(fmt.Sprint(err)+logs.String(), "upl_secret") {
					t.Fatalf("output or secret leaked: %s, %v", stdout, err)
				}
			})
		}
	}
}

func TestContractUploadMCPAttachmentRejectsEmptyFileBeforeHTTP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.pdf")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(dir)
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatal(err)
	}
	app := cli.New(cli.Options{Stdout: io.Discard, Stderr: io.Discard, Store: store, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("empty attachment triggered HTTP")
		return nil, nil
	})}})
	err := app.Run(context.Background(), []string{"contract", "upload-file", "--as", "user", "--file", path, "--file-type", "approveAttachment"})
	if err == nil || !strings.Contains(err.Error(), "non-empty") {
		t.Fatalf("error = %v", err)
	}
}
