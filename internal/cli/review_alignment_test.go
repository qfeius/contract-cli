package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
)

const alignmentConfig = `{"selectedPosition":"甲公司","selectedAuditRole":"甲方","reviewStrength":"中立","selectedCheckListIds":["2001"]}`

func TestReviewAlignmentHistoricalInputIsolation(t *testing.T) {
	app, _ := alignmentApp(t, nil)
	err := app.Run(context.Background(), []string{"rule", "table", "list", "--input", "unrelated.json"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("legacy rule table must reject review-only --input: %v", err)
	}
}

func TestReviewAlignmentRejectIrrelevantFlags(t *testing.T) {
	for _, args := range [][]string{
		{"subject", "extract", "--business-id", "biz", "--file-id", "1", "--data", "{}", "--dry-run"},
		{"checklist", "list", "--input", "ignored.json"},
		{"task", "status", "--task-id", "1", "--data", "{}"},
	} {
		app, _ := alignmentApp(t, nil)
		if err := app.Run(context.Background(), append([]string{"review"}, args...)); err == nil || !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("expected unknown flag: %v", err)
		}
	}
}

func TestReviewAlignmentTaskPreviewNormalization(t *testing.T) {
	app, out := alignmentApp(t, nil)
	spec := strings.Replace(alignmentStart(), alignmentConfig, `{"selectedPosition":" 甲公司 ","selectedAuditRole":" 甲方 ","reviewStrength":" 中立 ","selectedCheckListIds":[" 2001 "]}`, 1)
	if err := app.Run(context.Background(), []string{"review", "task", "start", "--data", spec, "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	var value struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(out.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value.Config["selectedPosition"] != "甲公司" || value.Config["reviewStrength"] != float64(1) {
		t.Fatal(value.Config)
	}
}

func alignmentStart() string {
	return `{"businessId":"biz","fileId":1175703921365090648,"fileHash":"` + strings.Repeat("a", 64) + `","config":` + alignmentConfig + `}`
}

func alignmentApp(t *testing.T, handle func(*http.Request) string) (*cli.App, *bytes.Buffer) {
	t.Helper()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	return cli.New(cli.Options{Stdout: out, Stderr: io.Discard, Store: store,
		LookupEnv:          func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if handle == nil {
				t.Fatalf("unexpected HTTP request: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer user-token" {
				t.Fatalf("review must use user token")
			}
			return jsonResponse(handle(r)), nil
		})}}), out
}

func alignmentBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	if err := d.Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestReviewAlignmentUpload(t *testing.T) {
	t.Run("stdin", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "stdin")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = file.Close() }()
		if _, err := file.WriteString("contract bytes"); err != nil {
			t.Fatal(err)
		}
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		old := os.Stdin
		os.Stdin = file
		defer func() { os.Stdin = old }()
		called := false
		app, _ := alignmentApp(t, func(*http.Request) string {
			called = true
			return `{"code":200,"data":{"fileId":"123"}}`
		})
		err = app.Run(context.Background(), []string{"review", "file", "upload", "--stdin", "--name", "contract.pdf"})
		if err == nil || !strings.Contains(err.Error(), "review pre-audit prepare") || called {
			t.Fatalf("stdin must stop before upload: err=%v called=%t", err, called)
		}
	})
	t.Run("url extension validation", func(t *testing.T) {
		app, _ := alignmentApp(t, nil)
		if err := app.Run(context.Background(), []string{"review", "file", "upload-url", "--file-url", "https://example.com/a", "--name", "bad.txt", "--dry-run"}); err == nil {
			t.Fatal("unsupported extension accepted")
		}
	})
	t.Run("url command removed", func(t *testing.T) {
		called := false
		app, _ := alignmentApp(t, func(r *http.Request) string {
			called = true
			return `{"code":200,"data":{}}`
		})
		if err := app.Run(context.Background(), []string{"review", "file", "upload-url", "--file-url", "https://example.com/a", "--name", "合同.pdf"}); err == nil || !strings.Contains(err.Error(), `unknown review file subcommand "upload-url"`) || called {
			t.Fatalf("URL must stop before upload: err=%v called=%t", err, called)
		}
	})
}

func TestReviewAlignmentSubject(t *testing.T) {
	app, out := alignmentApp(t, nil)
	err := app.Run(context.Background(), []string{"review", "subject", "extract", "--business-id", "biz", "--file-id", "1175703921365090648", "--file-hash", "trace", "--dry-run"})
	if err != nil {
		t.Fatal(err)
	}
	var b map[string]any
	if err := json.Unmarshal(out.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b["fileId"] != "1175703921365090648" || b["businessId"] != "biz" || b["fileHash"] != "trace" {
		t.Fatal(b)
	}
}

func TestReviewAlignmentStart(t *testing.T) {
	t.Run("normalized request", func(t *testing.T) {
		called := false
		app, _ := alignmentApp(t, func(r *http.Request) string {
			called = true
			b := alignmentBody(t, r)
			if r.URL.Path != "/open-apis/contract/v3/review/startReview" || r.Method != "POST" {
				t.Fatal(r.URL)
			}
			if b["fileId"] != "1175703921365090648" || b["config"].(map[string]any)["reviewStrength"] != json.Number("1") {
				t.Fatal(b)
			}
			usage, ok := b["usageReportContext"].(map[string]any)
			if !ok || usage["reportBusinessCode"] != "clm_100_openApi_cli" {
				t.Fatal("missing CLM attribution", b)
			}
			return `{"code":200,"data":{"taskId":"55"}}`
		})
		if err := app.Run(context.Background(), []string{"review", "task", "start", "--data", alignmentStart()}); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("not called")
		}
	})
	for _, body := range []string{`[]`, `{}`, strings.Replace(alignmentStart(), `"businessId":"biz"`, `"unknown":true,"businessId":"biz"`, 1), strings.Replace(alignmentStart(), `"selectedCheckListIds":["2001"]`, `"matchContractTypeRulePackage":false`, 1)} {
		t.Run("reject "+body[:2], func(t *testing.T) {
			app, _ := alignmentApp(t, nil)
			if err := app.Run(context.Background(), []string{"review", "task", "start", "--data", body, "--dry-run"}); err == nil {
				t.Fatal("invalid start input accepted")
			}
		})
	}
}

func TestReviewAlignmentRunURLDisabled(t *testing.T) {
	calls := 0
	app, out := alignmentApp(t, func(*http.Request) string {
		calls++
		return `{"code":200,"data":{}}`
	})
	body := `{"source":{"type":"url","fileUrl":"https://example.com/a","name":"合同.pdf"},"config":` + alignmentConfig + `,"extractSubjects":true,"wait":true}`
	err := app.Run(context.Background(), []string{"review", "run", "--data", body, "--interval", "1ms", "--deadline", "1s"})
	if err == nil || !strings.Contains(err.Error(), "review pre-audit prepare") || calls != 0 || out.Len() != 0 {
		t.Fatalf("URL workflow must stop before upload: err=%v calls=%d output=%s", err, calls, out.String())
	}
}

func TestReviewAlignmentQuery(t *testing.T) {
	t.Run("result failure", func(t *testing.T) {
		app, out := alignmentApp(t, func(r *http.Request) string {
			if r.URL.Path != "/open-apis/contract/v3/review/status" {
				t.Fatal("expected polling status", r.URL.Path)
			}
			return `{"code":200,"data":{"status":"fail","message":"failed"}}`
		})
		if err := app.Run(context.Background(), []string{"review", "task", "result", "--task-id", "99", "--interval", "1ms", "--deadline", "1s"}); err == nil {
			t.Fatal("failure must return error")
		}
		if !strings.Contains(out.String(), "fail") {
			t.Fatal(out.String())
		}
	})
	for _, args := range [][]string{{"status", "--task-id", "0"}, {"info", "--task-id", "1", "--visibility-scope", "other"}, {"info", "--task-id", "1", "--visibility-scope", "contractResult"}, {"status", "--task-id", "1", "--app-type", "CLM"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, _ := alignmentApp(t, nil)
			if err := app.Run(context.Background(), append([]string{"review", "task"}, args...)); err == nil {
				t.Fatal("invalid query accepted")
			}
		})
	}
}

func TestReviewAlignmentLists(t *testing.T) {
	t.Run("checklist CSV", func(t *testing.T) {
		app, _ := alignmentApp(t, func(r *http.Request) string {
			q := r.URL.Query()
			if !reflect.DeepEqual(q["reviewStages"], []string{"1", "2", "3"}) || !reflect.DeepEqual(q["sort"], []string{"name,desc"}) || q.Get("enabled") != "false" || q.Has("pageIndex") {
				t.Fatal(q)
			}
			return `{"code":200,"data":[]}`
		})
		if err := app.Run(context.Background(), []string{"review", "checklist", "list", "--review-stage", "1,2", "--review-stage", "3", "--sort", `"name,desc"`, "--enabled=false", "--page-index", "0"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("rule CSV", func(t *testing.T) {
		app, _ := alignmentApp(t, func(r *http.Request) string {
			if !reflect.DeepEqual(r.URL.Query()["sort"], []string{"name", "desc"}) || r.URL.Path != "/open-apis/contract/v1/review-rules/review-rule-groups/g/rules" {
				t.Fatal(r.URL)
			}
			return `{"code":200,"data":[]}`
		})
		if err := app.Run(context.Background(), []string{"review", "rule", "list", "--group-id", "g", "--sort", "name,desc", "--page-size", "10"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, args := range [][]string{{"checklist", "list", "--page-size", "-1"}, {"checklist", "list", "--start-time", "20", "--end-time", "10"}, {"rule", "group", "list", "--name", "ignored"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, _ := alignmentApp(t, nil)
			if err := app.Run(context.Background(), append([]string{"review"}, args...)); err == nil {
				t.Fatal("invalid list accepted")
			}
		})
	}
}

func TestReviewAlignmentHelp(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		flags []string
	}{
		{[]string{"review", "run"}, []string{"--interval", "--deadline", "--input"}},
		{[]string{"review", "subject", "extract"}, []string{"--business-id", "--file-id", "--file-hash"}},
		{[]string{"review", "file", "upload"}, []string{"--business-file-id", "--as <user>"}},
		{[]string{"review", "task", "result"}, []string{"--visibility-scope", "--interval", "--deadline"}},
		{[]string{"review", "checklist", "list"}, []string{"--enabled", "--review-stage", "--page-size"}},
		{[]string{"review", "rule", "list"}, []string{"--group-id", "--sort", "--page-size"}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			app, out := alignmentApp(t, nil)
			if err := app.Run(context.Background(), append(tc.args, "--help")); err != nil {
				t.Fatal(err)
			}
			for _, f := range tc.flags {
				if !strings.Contains(out.String(), f) {
					t.Errorf("missing flag %s", f)
				}
			}
		})
	}
}

func TestReviewAlignmentBoundaries(t *testing.T) {
	t.Run("binary file upload disabled", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "contract.pdf")
		if err := os.WriteFile(path, []byte("local contract"), 0600); err != nil {
			t.Fatal(err)
		}
		called := false
		app, _ := alignmentApp(t, func(*http.Request) string {
			called = true
			return `{"code":200,"data":{"fileId":"1"}}`
		})
		err := app.Run(context.Background(), []string{"review", "file", "upload", "--file", path, "--name", "contract.PDF"})
		if err == nil || !strings.Contains(err.Error(), "review pre-audit prepare") || called {
			t.Fatalf("file must stop before upload: err=%v called=%t", err, called)
		}
	})
	t.Run("user only", func(t *testing.T) {
		for _, args := range [][]string{{"task", "start", "--data", alignmentStart()}, {"task", "status", "--task-id", "1"}, {"checklist", "list"}, {"rule", "list", "--group-id", "g"}} {
			app, _ := alignmentApp(t, nil)
			err := app.Run(context.Background(), append(append([]string{"review"}, args...), "--as", "app"))
			if err == nil || !strings.Contains(err.Error(), "user") {
				t.Fatalf("expected user policy error: %v", err)
			}
		}
	})
	t.Run("result timeout preserves snapshot", func(t *testing.T) {
		app, out := alignmentApp(t, func(r *http.Request) string {
			if !strings.HasSuffix(r.URL.Path, "/status") {
				t.Fatal(r.URL)
			}
			return `{"code":200,"data":{"status":"running","taskId":"99"}}`
		})
		start := time.Now()
		err := app.Run(context.Background(), []string{"review", "task", "result", "--task-id", "99", "--interval", "1s", "--deadline", "20ms"})
		if err == nil || time.Since(start) > time.Second || !strings.Contains(out.String(), "running") {
			t.Fatalf("timeout %v, %s", err, out.String())
		}
	})
	t.Run("query context and positive numeric ID", func(t *testing.T) {
		app, _ := alignmentApp(t, func(r *http.Request) string {
			q := r.URL.Query()
			if q.Get("businessId") != "biz" || q.Get("visibilityScope") != "contractResult" || q.Get("taskId") != "1175703921365090648" || q.Has("appType") {
				t.Fatal(q)
			}
			return `{"code":200,"data":{"status":"success"}}`
		})
		if err := app.Run(context.Background(), []string{"review", "task", "status", "--task-id", "1175703921365090648", "--business-id", "biz", "--visibility-scope", "contractResult"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("strict config and aliases", func(t *testing.T) {
		app, _ := alignmentApp(t, nil)
		path := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(path, []byte(alignmentStart()), 0600); err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"--input", "--input-file"} {
			if err := app.Run(context.Background(), []string{"review", "task", "start", flag, path, "--dry-run"}); err != nil {
				t.Fatal(err)
			}
		}
		for _, args := range [][]string{
			{"--input", path, "--input-file", path},
			{"--data", strings.Replace(alignmentStart(), `"selectedPosition"`, `"unknown":true,"selectedPosition"`, 1)},
			{"--data", strings.Replace(alignmentStart(), strings.Repeat("a", 64), "bad", 1)},
			{"--data", strings.Replace(alignmentStart(), `1175703921365090648`, `"1175703921365090648"`, 1)},
		} {
			if err := app.Run(context.Background(), append(append([]string{"review", "task", "start"}, args...), "--dry-run")); err == nil {
				t.Fatal("invalid input accepted", args)
			}
		}
	})
	t.Run("business error stops task start", func(t *testing.T) {
		calls := 0
		app, out := alignmentApp(t, func(r *http.Request) string { calls++; return `{"code":400,"message":"rejected"}` })
		err := app.Run(context.Background(), []string{"review", "task", "start", "--data", alignmentStart()})
		if err == nil || calls != 1 {
			t.Fatalf("err=%v calls=%d output=%s", err, calls, out.String())
		}
	})
	t.Run("URL run without wait disabled", func(t *testing.T) {
		calls := 0
		app, out := alignmentApp(t, func(r *http.Request) string {
			calls++
			return `{"code":200,"data":{}}`
		})
		spec := `{"source":{"type":"url","fileUrl":"https://example.com/a.pdf","name":"a.pdf"},"config":` + alignmentConfig + `,"wait":false}`
		if err := app.Run(context.Background(), []string{"review", "run", "--data", spec}); err == nil || !strings.Contains(err.Error(), "review pre-audit prepare") {
			t.Fatalf("expected URL-disabled guidance: %v", err)
		}
		if calls != 0 || out.Len() != 0 {
			t.Fatal(out.String())
		}
	})
}

func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestReviewAlignmentUsageOverrideRejected(t *testing.T) {
	app, _ := alignmentApp(t, nil)
	payload := strings.Replace(alignmentStart(), `"businessId":"biz"`, `"usageReportContext":{"reportBusinessCode":"other"},"businessId":"biz"`, 1)
	if err := app.Run(context.Background(), []string{"review", "task", "start", "--data", payload, "--dry-run"}); err == nil {
		t.Fatal("attribution override accepted")
	}
	spec := `{"source":{"type":"url","fileUrl":"https://example.com/a","name":"a.pdf"},"config":` + alignmentConfig + `,"usageReportContext":{"reportBusinessCode":"other"}}`
	if err := app.Run(context.Background(), []string{"review", "run", "--data", spec, "--dry-run"}); err == nil {
		t.Fatal("run attribution override accepted")
	}
}
