package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
	"cn.qfei/contract-cli/internal/openplatform"
)

func preAuditOutcomeFixture(t *testing.T) (*App, *preAuditRun) {
	t.Helper()
	app, _, _ := newDeviceTokenTestApp(t, &config.Token{AccessToken: preAuditTestJWT("1002718663263913046"), Expiry: deviceTokenTestNow().Add(time.Hour)}, func(*http.Request) (*http.Response, error) { t.Fatal("unexpected default HTTP"); return nil, nil })
	run := &preAuditRun{SchemaVersion: 1, RunID: strings.Repeat("a", 32), Profile: "contract", Origin: "https://open.qfei.cn", TenantID: "896684654038353011", UserID: "1002718663263913046", Stage: "awaiting_config", Name: "contract.pdf", SourceFileID: "11", CategoryID: "33", PreAuditID: "22", LatestFileID: "11", AIFileID: "44", FileHash: strings.Repeat("a", 64)}
	if err := app.preAuditStore().save(run); err != nil {
		t.Fatal(err)
	}
	return app, run
}

func TestPreAuditLocalFailureRestoresExactStartState(t *testing.T) {
	for _, stage := range []string{"awaiting_config", "review_failed"} {
		for _, mode := range []string{"prepare", "hook", "save failure", "confirmed save failure", "transport", "after refresh", "refresh status failure"} {
			t.Run(stage+"/"+mode, func(t *testing.T) {
				app, run := preAuditOutcomeFixture(t)
				run.Stage = stage
				if stage == "review_failed" {
					run.TaskID = "55"
					run.Start = map[string]any{"taskId": "55"}
					run.Final = map[string]any{"status": "fail"}
				}
				before := *run
				if err := app.preAuditStore().save(run); err != nil {
					t.Fatal(err)
				}
				calls, hooks, prepares := 0, 0, 0
				failure := errors.New("local boundary failure")
				client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					if r.Method == "GET" {
						return jsonResponse(`{"code":0,"data":{"pre_audit_id":"22","source_file_id":"11","latest_file_id":"11","category_id":"33","business_type_code":24}}`), nil
					}
					calls++
					if mode == "confirmed save failure" {
						response := jsonResponse(`{"detail":"Not authenticated"}`)
						response.StatusCode = http.StatusForbidden
						return response, nil
					}
					if mode == "after refresh" || mode == "refresh status failure" {
						response := jsonResponse(`{"data":{"error_type":"token_expired"}}`)
						response.StatusCode = 401
						response.Header.Set("X-Qfei-Open-Platform-Auth-Error", "token_expired")
						return response, nil
					}
					return nil, failure
				})}, BeforeRequestHooks: []openplatform.BeforeRequestHook{func(_ context.Context, r *http.Request) error {
					if r.Method == "GET" {
						return nil
					}
					hooks++
					if mode == "save failure" || mode == "confirmed save failure" {
						p := filepath.Join(app.preAuditStore().dir, run.RunID+".json")
						if err := os.Rename(p, p+".backup"); err != nil {
							t.Fatal(err)
						}
						if err := os.Mkdir(p, 0700); err != nil {
							t.Fatal(err)
						}
					}
					if mode == "hook" || mode == "save failure" || (mode == "after refresh" && hooks == 2) {
						return failure
					}
					return nil
				}}})
				rc := openplatform.RequestContext{BaseURL: run.Origin, Identity: config.IdentityUser, AccessToken: "fixture", PrepareAccessToken: func(context.Context, string) (string, error) {
					prepares++
					if mode == "prepare" && prepares == 2 {
						return "", failure
					}
					return "fixture", nil
				}, RefreshAccessToken: func(context.Context, string) (string, error) {
					if mode == "refresh status failure" {
						return "", &openplatform.HTTPStatusError{StatusCode: http.StatusUnauthorized}
					}
					return "renewed", nil
				}}
				err := app.startPreAudit(context.Background(), commandOptions{reviewSession: &reviewRequestSession{client: client, rc: rc}}, run, map[string]any{"selectedPosition": "partyA", "selectedAuditRole": "legal", "reviewStrength": "中立", "matchContractTypeRulePackage": true})
				if err == nil {
					t.Fatal("expected failure")
				}
				if mode == "prepare" || mode == "hook" {
					stored, loadErr := app.preAuditStore().load(run.RunID)
					if loadErr != nil || calls != 0 || !reflect.DeepEqual(*run, before) || !reflect.DeepEqual(*stored, before) {
						t.Fatalf("lost previous state: run=%+v stored=%+v err=%v calls=%d", run, stored, loadErr, calls)
					}
				} else if run.Stage != "start_unknown" {
					t.Fatalf("unsafe rollback: %+v %v", run, err)
				}
				if mode == "save failure" && !strings.Contains(err.Error(), "保存失败") {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestPreAuditLocalCreateFailureAllowsSameRunRetry(t *testing.T) {
	app, run := preAuditOutcomeFixture(t)
	run.Stage = "awaiting_category"
	run.CategoryID = ""
	run.PreAuditID = ""
	run.LatestFileID = ""
	run.AIFileID = ""
	run.FileHash = ""
	if err := app.preAuditStore().save(run); err != nil {
		t.Fatal(err)
	}
	before := *run
	run.Stage = "create_pending"
	run.CategoryID = "33"
	if err := app.preAuditStore().save(run); err != nil {
		t.Fatal(err)
	}
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("must not send create"); return nil, nil })}, BeforeRequestHooks: []openplatform.BeforeRequestHook{func(context.Context, *http.Request) error { return errors.New("pre-send hook failure") }}})
	rc := openplatform.RequestContext{BaseURL: run.Origin, Identity: config.IdentityUser, AccessToken: "fixture"}
	err := app.createPreAudit(context.Background(), commandOptions{reviewSession: &reviewRequestSession{client: client, rc: rc}}, run, true)
	stored, loadErr := app.preAuditStore().load(run.RunID)
	if err == nil || loadErr != nil || !reflect.DeepEqual(*run, before) || !reflect.DeepEqual(*stored, before) {
		t.Fatalf("create did not restore: %+v %+v %v %v", run, stored, err, loadErr)
	}
	creates, references := 0, 0
	client = openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == preAuditPath {
			creates++
			return jsonResponse(`{"code":0,"data":{"pre_audit_id":"22","source_file_id":"11","latest_file_id":"11","category_id":"33","business_type_code":24}}`), nil
		}
		references++
		return jsonResponse(`{"code":200,"data":{"fileId":"44","fileHash":"` + strings.Repeat("a", 64) + `","snapshotReady":true}}`), nil
	})}})
	run.Stage, run.CategoryID = "create_pending", "33"
	if err := app.preAuditStore().save(run); err != nil {
		t.Fatal(err)
	}
	err = app.createPreAudit(context.Background(), commandOptions{reviewSession: &reviewRequestSession{client: client, rc: rc}}, run, true)
	if err != nil || creates != 1 || references != 1 || run.Stage != "awaiting_config" || run.RunID != before.RunID {
		t.Fatalf("same-run retry failed: %+v %v creates=%d references=%d", run, err, creates, references)
	}
}

func TestPreAuditStartClassifierIsConservative(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   bool
	}{
		{403, `{"detail":"Not authenticated"}`, true},
		{401, `{"detail":"Not authenticated"}`, true},
		{408, `{"code":408}`, false}, {422, `{"code":422}`, false},
		{200, `{"code":999999}`, false}, {403, `{"code":999999}`, false},
		{403, `{"detail":"Not authenticated","data":{"taskId":"55"}}`, false},
		{401, `{"detail":"Not authenticated","taskId":"55"}`, false},
		{401, `invalid`, false},
		{422, `{"code":422,"msg":"参数错误，config不能为空","data":["config不能为空"]}`, true},
		{401, `{"code":401000001,"msg":"无效的认证信息","data":null}`, true},
	} {
		response := openplatform.Response{StatusCode: tc.status, Body: []byte(tc.body)}
		err := &openplatform.HTTPStatusError{StatusCode: tc.status, Body: response.Body}
		if got := preAuditStartRejected(response, err); got != tc.want {
			t.Fatalf("status=%d body=%s got=%v", tc.status, tc.body, got)
		}
	}
}

func TestPreAuditReferencePreSendFailureKeepsCreatedObject(t *testing.T) {
	app, run := preAuditOutcomeFixture(t)
	run.Stage = "pre_audit_created"
	run.AIFileID, run.FileHash = "", ""
	if err := app.preAuditStore().save(run); err != nil {
		t.Fatal(err)
	}
	before := *run
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("reference must not send"); return nil, nil })}, BeforeRequestHooks: []openplatform.BeforeRequestHook{func(context.Context, *http.Request) error { return errors.New("hook stopped") }}})
	err := app.createPreAudit(context.Background(), commandOptions{reviewSession: &reviewRequestSession{client: client, rc: openplatform.RequestContext{BaseURL: run.Origin, Identity: config.IdentityUser, AccessToken: "fixture"}}}, run, false)
	stored, loadErr := app.preAuditStore().load(run.RunID)
	if err == nil || loadErr != nil || !reflect.DeepEqual(*stored, before) || !reflect.DeepEqual(*run, before) {
		t.Fatalf("reference state lost: %+v %v", run, err)
	}
}

func TestPreAuditUnsentUploadDoesNotLeaveUnknownRun(t *testing.T) {
	app, _ := preAuditOutcomeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.inspectEnvironment = func(context.Context, int) invocation.Result { cancel(); return invocation.Result{} }
	file := filepath.Join(t.TempDir(), "contract.pdf")
	if err := os.WriteFile(file, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	run, err := app.preparePreAudit(ctx, commandOptions{}, file, "contract.pdf")
	if err == nil || run != nil || !strings.Contains(err.Error(), "未发送") {
		t.Fatalf("unsent upload became unknown: %+v %v", run, err)
	}
	files, globErr := filepath.Glob(filepath.Join(app.preAuditStore().dir, "*.json"))
	if globErr != nil || len(files) != 1 {
		t.Fatalf("left orphan recovery record: %v %v", files, globErr)
	}
}

func TestPreAuditUploadDeadlineRunAlsoKeepsLongBudget(t *testing.T) {
	app, _ := preAuditOutcomeFixture(t)
	app.stdout = io.Discard
	app.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		d, ok := r.Context().Deadline()
		if !ok || time.Until(d) < time.Minute {
			t.Fatal("upload budget truncated")
		}
		_, _ = io.Copy(io.Discard, r.Body)
		return jsonResponse(`{"code":0,"data":{"file_id":"11"}}`), nil
	})}
	file := filepath.Join(t.TempDir(), "contract.pdf")
	if err := os.WriteFile(file, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"source": map[string]string{"type": "file", "path": file, "name": "contract.pdf"}})
	_ = app.Run(context.Background(), []string{"review", "run", "--data", string(body), "--deadline", "2m"})
}
