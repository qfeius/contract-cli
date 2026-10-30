package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

type reviewUploadReadCounter struct{ reads int }

func (r *reviewUploadReadCounter) Read([]byte) (int, error) {
	r.reads++
	return 0, io.EOF
}

func assertReviewURLDisabled(t *testing.T, args []string, required ...string) {
	t.Helper()
	store := config.NewStore(t.TempDir())
	reader := &reviewUploadReadCounter{}
	output, logs := new(bytes.Buffer), new(bytes.Buffer)
	requests := 0
	app := cli.New(cli.Options{
		Store: store, Stdin: reader, Stdout: output, Stderr: io.Discard,
		Logger: slog.New(slog.NewTextHandler(logs, nil)),
		LookupEnv: func(name string) (string, bool) {
			return "1", name == "CONTRACT_CLI_NO_UPDATE_CHECK"
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			return jsonResponse(`{"code":200,"data":{}}`), nil
		})},
	})
	err := app.Run(context.Background(), args)
	if len(required) == 0 {
		required = []string{"URL", "暂不支持", "review pre-audit prepare"}
	}
	for _, want := range required {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want rejection containing %q, got %v", want, err)
		}
	}
	if reader.reads != 0 || requests != 0 || output.Len() != 0 {
		t.Errorf("URL rejection caused side effects: reads=%d requests=%d output=%q", reader.reads, requests, output.String())
	}
	if strings.Contains(logs.String(), "example.com") || strings.Contains(logs.String(), "signed-private-value") {
		t.Error("URL or signature leaked into logs")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(store.Path()), "preaudit-runs")); !os.IsNotExist(err) {
		t.Errorf("URL rejection created run storage: %v", err)
	}
}

func TestReviewURLCommandRemovedBeforeSideEffects(t *testing.T) {
	for _, input := range []struct {
		name string
		args []string
	}{
		{"https", []string{"--file-url", "https://example.com/contract.pdf?signature=signed-private-value", "--name", "contract.pdf"}},
		{"http", []string{"--file-url", "http://example.com/contract.pdf", "--name", "contract.pdf"}},
		{"invalid URL", []string{"--file-url", "invalid", "--name", "contract.pdf"}},
		{"empty URL", []string{"--file-url", "", "--name", "contract.pdf"}},
		{"missing args", nil},
	} {
		for _, mode := range [][]string{nil, {"--dry-run"}, {"--print-input"}} {
			t.Run(input.name+strings.Join(mode, ""), func(t *testing.T) {
				args := append([]string{"review", "file", "upload-url"}, input.args...)
				assertReviewURLDisabled(t, append(args, mode...), `unknown review file subcommand "upload-url"`)
			})
		}
	}
}

func TestReviewRunURLDisabledBeforeSideEffects(t *testing.T) {
	for _, body := range []string{
		`{"source":{"type":"url","fileUrl":"https://example.com/a.pdf?signature=signed-private-value","name":"a.pdf"},"config":` + alignmentConfig + `,"wait":true,"extractSubjects":true}`,
		`{"source":{"type":"url","fileUrl":"http://example.com/a.pdf","name":"a.pdf"},"config":` + alignmentConfig + `,"wait":false}`,
		`{"source":{"type":"url","fileUrl":"https://example.com/a.pdf","name":"a.pdf"}}`,
		`{"source":{"type":"url","fileUrl":"","path":"/nonexistent","name":"a.pdf"}}`,
	} {
		for _, inputFlag := range []string{"--data", "--input", "--input-file"} {
			for _, mode := range [][]string{nil, {"--dry-run"}, {"--print-input"}} {
				t.Run(inputFlag+strings.Join(mode, "")+body, func(t *testing.T) {
					value := body
					if inputFlag != "--data" {
						value = filepath.Join(t.TempDir(), "run.json")
						if err := os.WriteFile(value, []byte(body), 0600); err != nil {
							t.Fatal(err)
						}
					}
					assertReviewURLDisabled(t, append([]string{"review", "run", inputFlag, value}, mode...))
				})
			}
		}
	}
}

func TestReviewRemovedURLCommandHelpAndGuidance(t *testing.T) {
	for _, command := range [][]string{{"review"}, {"review", "file"}, {"review", "run"}} {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			app, out := alignmentApp(t, nil)
			if err := app.Run(context.Background(), append(command, "--help")); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "upload-url") || strings.Contains(out.String(), "已禁用") {
				t.Errorf("help still displays a removed command or placeholder: %s", out.String())
			}
			if strings.Contains(out.String(), "--file-url <") || strings.Contains(out.String(), "URL 文件由服务端下载") || strings.Contains(out.String(), "type=url 时提供") {
				t.Error("help still advertises URL upload inputs")
			}
		})
	}
	for _, args := range [][]string{
		{"review", "file", "upload-url", "-h"},
		{"review", "file", "upload-url", "--help"},
		{"help", "review", "file", "upload-url"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			assertReviewURLDisabled(t, args, `unknown help topic "review file upload-url"`)
		})
	}
	for _, name := range []string{"skills/contract-cli-review/SKILL.md", "README.md", "docs/cli-command-reference.md"} {
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("..", "..", name))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(body, []byte("review pre-audit prepare")) {
				t.Error("guidance must direct users to CLM prepare")
			}
			for _, pattern := range []string{`upload-url`, `"source"\s*:\s*\{\s*"type"\s*:\s*"url"`, `URL来源沿用原编排|URL来源继续使用既有命令|URL 来源继续使用`} {
				if regexp.MustCompile(pattern).Match(body) {
					t.Errorf("guidance still enables URL uploads: %s", pattern)
				}
			}
		})
	}
}

func TestReviewUploadRejectsBinaryBeforeSideEffects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contract.pdf")
	if err := os.WriteFile(path, []byte("local contract"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name string
		args []string
	}{
		{"file", []string{"--file", path}},
		{"missing file", []string{"--file", path + ".missing"}},
		{"empty file", []string{"--file", ""}},
		{"inline empty file", []string{"--file="}},
		{"stdin", []string{"--stdin"}},
		{"explicit false stdin", []string{"--stdin=false"}},
		{"both inputs", []string{"--file", path, "--stdin"}},
		{"file with reference", append(referenceArgs()[3:], "--file", path)},
		{"stdin with reference", append(referenceArgs()[3:], "--stdin")},
	} {
		for _, mode := range []struct {
			name string
			args []string
		}{{"execute", nil}, {"dry run", []string{"--dry-run"}}, {"print input", []string{"--print-input"}}} {
			t.Run(input.name+"/"+mode.name, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				reader := &reviewUploadReadCounter{}
				output := new(bytes.Buffer)
				requests := 0
				app := cli.New(cli.Options{
					Store: store, Stdin: reader, Stdout: output, Stderr: io.Discard,
					LookupEnv: func(name string) (string, bool) {
						return "1", name == "CONTRACT_CLI_NO_UPDATE_CHECK"
					},
					HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						requests++
						return jsonResponse(`{"code":200,"data":{}}`), nil
					})},
				})
				args := append([]string{"review", "file", "upload", "--name", "contract.pdf"}, input.args...)
				args = append(args, mode.args...)
				err := app.Run(context.Background(), args)
				if err == nil || !strings.Contains(err.Error(), "review pre-audit prepare") || !strings.Contains(err.Error(), "引用") {
					t.Errorf("want reference-only rejection and pre-audit guidance, got %v", err)
				}
				if reader.reads != 0 || requests != 0 || output.Len() != 0 {
					t.Errorf("rejected upload caused side effects: reads=%d requests=%d output=%q", reader.reads, requests, output.String())
				}
				if _, err := os.Stat(filepath.Join(filepath.Dir(store.Path()), "preaudit-runs")); !os.IsNotExist(err) {
					t.Errorf("rejected upload created run storage: %v", err)
				}
			})
		}
	}
}

func TestReviewReferenceMissingFieldsNoBinaryFallback(t *testing.T) {
	for _, flag := range []string{"--business-id", "--business-file-id", "--business-file-version", "--source-locator"} {
		t.Run(flag, func(t *testing.T) {
			args := referenceArgs()
			for i := range args {
				if args[i] == flag {
					args[i+1] = ""
					break
				}
			}
			app, _ := alignmentApp(t, nil)
			err := app.Run(context.Background(), append(args, "--dry-run"))
			if err == nil || !strings.Contains(err.Error(), flag) {
				t.Fatalf("missing reference field was accepted: %v", err)
			}
			if strings.Contains(err.Error(), "或使用 --file/--stdin") {
				t.Fatalf("validation still recommends binary upload: %v", err)
			}
		})
	}
}

func TestReviewReferenceUploadFileTypes(t *testing.T) {
	for _, kind := range []string{"0", "1", "2"} {
		t.Run(kind, func(t *testing.T) {
			args := referenceArgs()
			if kind != "1" {
				args[len(args)-1] = `{"provider":"clm","sourceFileType":` + kind + `}`
			}
			locator := args[len(args)-1]
			requests := 0
			app, _ := alignmentApp(t, func(r *http.Request) string {
				requests++
				if r.Method != http.MethodPost || r.URL.Path != "/open-apis/contract/v3/review/file/upload" {
					t.Fatalf("unexpected reference endpoint: %s %s", r.Method, r.URL.Path)
				}
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Fatal(err)
				}
				defer func() { _ = r.MultipartForm.RemoveAll() }()
				if len(r.MultipartForm.File) != 0 {
					t.Fatal("reference request contains binary parts")
				}
				for field, want := range map[string]string{"businessId": "1178244026676019544", "businessFileId": "1178244026390806872", "businessFileVersion": "V1.0", "name": "合同.docx", "sourceLocator": locator} {
					if got := r.FormValue(field); got != want {
						t.Errorf("%s = %q, want %q", field, got, want)
					}
				}
				if _, exists := r.MultipartForm.Value["file"]; exists {
					t.Fatal("reference request contains a file field")
				}
				return `{"code":200,"data":{"fileId":"1006610","fileHash":"` + strings.Repeat("a", 64) + `","snapshotReady":true}}`
			})
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			if requests != 1 {
				t.Fatalf("reference requests = %d, want 1", requests)
			}
		})
	}
}

func TestPreAuditPrepareAfterBinaryRejection(t *testing.T) {
	var requests []string
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.Path)
		if r.URL.Path == "/open-apis/contract/v3/review/file/upload" {
			return jsonResponse(`{"code":200,"data":{"fileId":"` + preAuditAIID + `"}}`), nil
		}
		return preAuditUploadResponse(t, r)
	})
	err := h.app.Run(context.Background(), []string{"review", "file", "upload", "--file", h.file, "--name", "合同.pdf"})
	if err == nil || !strings.Contains(err.Error(), "review pre-audit prepare") || len(requests) != 0 {
		t.Fatalf("binary input must redirect before upload: err=%v requests=%v", err, requests)
	}
	result, err := h.run("prepare", "--file", h.file, "--name", "合同.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0] != "/open-apis/contract/v1/files/upload" || result["sourceFileId"] != preAuditSourceID || result["workflowStage"] != "awaiting_category" {
		t.Fatalf("same file must remain uploadable through CLM: requests=%v result=%v", requests, result)
	}
}

func TestPreAuditPrepareAfterURLRejection(t *testing.T) {
	var requests []string
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/file/uploadByUrl") {
			return jsonResponse(`{"code":200,"data":{"fileId":"` + preAuditAIID + `"}}`), nil
		}
		return preAuditUploadResponse(t, r)
	})
	err := h.app.Run(context.Background(), []string{"review", "file", "upload-url", "--file-url", "https://example.com/contract.pdf", "--name", "合同.pdf"})
	if err == nil || !strings.Contains(err.Error(), `unknown review file subcommand "upload-url"`) || len(requests) != 0 {
		t.Fatalf("URL input must stop before upload: err=%v requests=%v", err, requests)
	}
	result, err := h.run("prepare", "--file", h.file, "--name", "合同.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0] != "/open-apis/contract/v1/files/upload" || result["sourceFileId"] != preAuditSourceID || result["workflowStage"] != "awaiting_category" {
		t.Fatalf("local CLM preparation changed: requests=%v result=%v", requests, result)
	}
}

func TestReviewReferenceOnlyHelpAndGuidance(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		app, out := alignmentApp(t, nil)
		if err := app.Run(context.Background(), []string{"review", "file", "upload", "--help"}); err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{"引用", "--business-id", "--business-file-id", "--business-file-version", "--source-locator", "review pre-audit prepare"} {
			if !strings.Contains(out.String(), required) {
				t.Errorf("upload help missing %q", required)
			}
		}
		if regexp.MustCompile(`(?m)^\s+--(file(?:\s|$)|stdin)`).MatchString(out.String()) {
			t.Error("reference-only help advertises binary input flags")
		}
	})
	for _, name := range []string{"skills/contract-cli-review/SKILL.md", "README.md", "docs/cli-command-reference.md"} {
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("..", "..", name))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{"review pre-audit prepare", "review file upload"} {
				if !bytes.Contains(body, []byte(required)) {
					t.Errorf("%s missing %q", name, required)
				}
			}
			if regexp.MustCompile("contract-cli review file upload[^\n`]*--(?:file|stdin)(?:[\\s=<]|$)").Match(body) {
				t.Error("guidance still contains a direct binary upload command")
			}
			if bytes.Contains(body, []byte("低层字节上传保留给")) {
				t.Error("guidance still reserves a legacy binary upload path")
			}
		})
	}
	// Keep a preview assertion at the public command boundary as a preservation check.
	t.Run("reference preview", func(t *testing.T) {
		app, out := alignmentApp(t, nil)
		if err := app.Run(context.Background(), append(referenceArgs(), "--dry-run")); err != nil {
			t.Fatal(err)
		}
		var preview map[string]string
		if err := json.Unmarshal(out.Bytes(), &preview); err != nil || preview["businessFileId"] != "1178244026390806872" {
			t.Fatalf("reference preview changed: %s, %v", out.Bytes(), err)
		}
	})
}
