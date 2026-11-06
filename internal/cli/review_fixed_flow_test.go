package cli_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReviewFixedFlowInfoPreflight(t *testing.T) {
	for _, tc := range []struct {
		name     string
		snapshot string
	}{
		{"running", `{"status":"running"}`},
		{"failed", `{"status":"fail"}`},
		{"missing", `{}`},
		{"unknown", `{"status":"unexpected"}`},
		{"business rejection", `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			app, _ := alignmentApp(t, func(r *http.Request) string {
				paths = append(paths, r.URL.Path)
				if strings.HasSuffix(r.URL.Path, "/status") {
					if tc.snapshot == `null` {
						return `{"code":403,"message":"denied"}`
					}
					return `{"code":200,"data":` + tc.snapshot + `}`
				}
				return `{"code":200,"data":{"status":"success","ruleResult":[]}}`
			})
			err := app.Run(context.Background(), []string{"review", "task", "info", "--task-id", "123", "--business-id", "biz"})
			if err == nil || !reflect.DeepEqual(paths, []string{"/open-apis/contract/v3/review/status"}) {
				t.Fatalf("info must stop at unsuccessful status: err=%v paths=%v", err, paths)
			}
		})
	}
}

func TestReviewFixedFlowInfoRequiresReadyDetails(t *testing.T) {
	for _, tc := range []struct {
		name  string
		data  string
		ready bool
	}{
		{"running", `{"status":"running","ruleResult":[]}`, false},
		{"failed", `{"status":"fail","ruleResult":[]}`, false},
		{"missing status", `{"ruleResult":[]}`, false},
		{"missing risks", `{"status":"success"}`, false},
		{"null risks", `{"status":"success","ruleResult":null}`, false},
		{"invalid risks", `{"status":"success","ruleResult":{}}`, false},
		{"review not ready", `{"status":"success","reviewReady":false,"ruleResult":[]}`, false},
		{"zero risks", `{"status":"success","ruleResult":[]}`, true},
		{"ready risks", `{"status":"success","reviewReady":true,"ruleResult":[{"id":"risk-1"}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			response := `{"code":200,"msg":"success","data":` + tc.data + `}`
			app, out := alignmentApp(t, func(r *http.Request) string {
				paths = append(paths, r.URL.Path)
				query := r.URL.Query()
				if query.Get("taskId") != "123" || query.Get("businessId") != "biz" || query.Get("visibilityScope") != "contractResult" {
					t.Fatalf("info guard lost task context: %s", r.URL)
				}
				if strings.HasSuffix(r.URL.Path, "/status") {
					if query.Has("simplifyReviewResult") {
						t.Fatal("status must not receive the info-only parameter")
					}
					return `{"code":200,"data":{"status":"success"}}`
				}
				if query.Get("simplifyReviewResult") != "true" {
					t.Fatal("info lost simplifyReviewResult")
				}
				return response
			})
			err := app.Run(context.Background(), []string{"review", "task", "info", "--task-id", "123", "--business-id", "biz", "--visibility-scope", "contractResult", "--raw"})
			if (err == nil) != tc.ready {
				t.Errorf("info completion: ready=%t err=%v output=%s", tc.ready, err, out.String())
			}
			if !reflect.DeepEqual(paths, []string{"/open-apis/contract/v3/review/status", "/open-apis/contract/v3/review/info"}) {
				t.Errorf("info must follow status: %v", paths)
			}
			if strings.TrimSpace(out.String()) != response {
				t.Errorf("info must preserve the raw response: %s", out.String())
			}
		})
	}
}

func TestReviewFixedFlowWaitForReadyDetails(t *testing.T) {
	for _, pending := range []string{
		`{"status":"running","ruleResult":[]}`,
		`{"status":"success"}`,
		`{"status":"success","ruleResult":null}`,
		`{"status":"success","reviewReady":false,"ruleResult":[]}`,
	} {
		t.Run(pending, func(t *testing.T) {
			var paths []string
			var deadline time.Time
			traceID := ""
			statusCalls, infoCalls := 0, 0
			app, out := alignmentApp(t, func(r *http.Request) string {
				paths = append(paths, r.URL.Path)
				if r.URL.Query().Get("taskId") != "123" || r.URL.Query().Get("businessId") != "biz" {
					t.Fatalf("waiting changed the original task: %s", r.URL)
				}
				currentDeadline, ok := r.Context().Deadline()
				if !ok {
					t.Fatal("waiting must retain its deadline")
				}
				if deadline.IsZero() {
					deadline = currentDeadline
					traceID = r.Header.Get("X-Log-Id")
				}
				if !currentDeadline.Equal(deadline) || traceID == "" || r.Header.Get("X-Log-Id") != traceID {
					t.Fatal("detail recovery must stay within the original deadline and command trace")
				}
				if strings.HasSuffix(r.URL.Path, "/status") {
					statusCalls++
					if statusCalls == 1 {
						return `{"code":200,"data":{"status":"running"}}`
					}
					return `{"code":200,"data":{"status":"success"}}`
				}
				infoCalls++
				if infoCalls == 1 {
					return `{"code":200,"data":` + pending + `}`
				}
				return `{"code":200,"data":{"status":"success","reviewReady":true,"ruleResult":[]}}`
			})
			err := app.Run(context.Background(), []string{"review", "task", "result", "--task-id", "123", "--business-id", "biz", "--interval", "1ms", "--deadline", "1s"})
			want := []string{"/open-apis/contract/v3/review/status", "/open-apis/contract/v3/review/status", "/open-apis/contract/v3/review/info", "/open-apis/contract/v3/review/status", "/open-apis/contract/v3/review/info"}
			if err != nil || !reflect.DeepEqual(paths, want) {
				t.Fatalf("pending details must resume the fixed sequence: err=%v paths=%v", err, paths)
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["status"] != "success" || result["reviewReady"] != true {
				t.Fatalf("only ready details may complete: %s, %v", out.String(), err)
			}
		})
	}
}

func TestReviewFixedFlowResultNotReadyDeadline(t *testing.T) {
	app, out := alignmentApp(t, func(r *http.Request) string {
		if strings.HasSuffix(r.URL.Path, "/status") {
			return `{"code":200,"data":{"status":"success"}}`
		}
		return `{"code":200,"data":{"status":"running","ruleResult":[]}}`
	})
	err := app.Run(context.Background(), []string{"review", "task", "result", "--task-id", "123", "--business-id", "biz", "--interval", "1s", "--deadline", "25ms"})
	var result map[string]any
	if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if !errors.Is(err, context.DeadlineExceeded) || result["status"] != "running" {
		t.Fatalf("pending details must time out with their last snapshot: err=%v output=%s", err, out.String())
	}
}

func TestReviewFixedFlowResultFailsWhenInfoFails(t *testing.T) {
	var paths []string
	app, out := alignmentApp(t, func(r *http.Request) string {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/status") {
			return `{"code":200,"data":{"status":"success"}}`
		}
		return `{"code":200,"data":{"status":"fail","ruleResult":[]}}`
	})
	err := app.Run(context.Background(), []string{"review", "task", "result", "--task-id", "123", "--business-id", "biz"})
	if err == nil || !strings.Contains(out.String(), `"fail"`) || !reflect.DeepEqual(paths, []string{"/open-apis/contract/v3/review/status", "/open-apis/contract/v3/review/info"}) {
		t.Fatalf("failed details must stop with an error: err=%v paths=%v output=%s", err, paths, out.String())
	}
}

func TestPreAuditFixedFlowDoesNotDeliverPendingDetails(t *testing.T) {
	ready, started := false, false
	pageCalls, startCalls := 0, 0
	h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open-apis/contract/v1/pre_audits/" + preAuditID:
			if started {
				pageCalls++
			}
			return jsonResponse(preAuditCreatedData()), nil
		case "/open-apis/contract/v3/review/startReview":
			started = true
			startCalls++
			return jsonResponse(`{"code":200,"data":{"taskId":"` + preAuditTaskID + `"}}`), nil
		case "/open-apis/contract/v3/review/status":
			return jsonResponse(`{"code":200,"data":{"status":"success"}}`), nil
		case "/open-apis/contract/v3/review/info":
			if ready {
				return jsonResponse(`{"code":200,"data":{"status":"success","reviewReady":true,"ruleResult":[]}}`), nil
			}
			return jsonResponse(`{"code":200,"data":{"status":"running","reviewReady":false,"ruleResult":[]}}`), nil
		default:
			return preAuditPreparationRequest(t, r)
		}
	})
	runID := h.create()
	result, err := h.run("start", "--run-id", runID, "--data", `{"config":`+alignmentConfig+`}`, "--interval", "1s", "--deadline", "500ms")
	if err == nil || result["workflowStage"] != "running" || pageCalls != 0 {
		t.Fatalf("pending review must not deliver a page: err=%v result=%v pages=%d", err, result, pageCalls)
	}
	ready = true
	result, err = h.run("resume", "--run-id", runID, "--interval", "1ms", "--deadline", "1s")
	final, _ := result["final"].(map[string]any)
	if err != nil || result["workflowStage"] != "completed" || final["reviewDetailUrl"] != preAuditURL || startCalls != 1 || pageCalls != 1 {
		t.Fatalf("ready recovery must finish the original task: err=%v result=%v starts=%d pages=%d", err, result, startCalls, pageCalls)
	}
}

func TestPreAuditFixedFlowRechecksUnreadyCompletionCache(t *testing.T) {
	for _, tc := range []struct {
		name  string
		stage string
		final string
	}{
		{"completed running", "completed", `{"status":"running","ruleResult":[]}`},
		{"completed missing risks", "completed", `{"status":"success"}`},
		{"review succeeded running", "review_succeeded", `{"status":"running","ruleResult":[]}`},
		{"review succeeded not ready", "review_succeeded", `{"status":"success","reviewReady":false,"ruleResult":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recovering, ready := false, false
			pageCalls, infoCalls := 0, 0
			h := newPreAuditHarness(t, func(r *http.Request) (*http.Response, error) {
				if !recovering {
					return preAuditPreparationRequest(t, r)
				}
				if r.Method != http.MethodGet {
					t.Fatalf("cache recovery must not restart or upload: %s %s", r.Method, r.URL)
				}
				switch r.URL.Path {
				case "/open-apis/contract/v1/pre_audits/" + preAuditID:
					pageCalls++
					return jsonResponse(preAuditCreatedData()), nil
				case "/open-apis/contract/v3/review/status":
					if r.URL.Query().Get("taskId") != preAuditTaskID || r.URL.Query().Get("businessId") != preAuditID {
						t.Fatal("cache recovery changed the original task", r.URL)
					}
					if ready {
						return jsonResponse(`{"code":200,"data":{"status":"success"}}`), nil
					}
					return jsonResponse(`{"code":200,"data":{"status":"running"}}`), nil
				case "/open-apis/contract/v3/review/info":
					infoCalls++
					return jsonResponse(`{"code":200,"data":{"status":"success","reviewReady":true,"ruleResult":[]}}`), nil
				default:
					t.Fatalf("unexpected cache recovery request: %s", r.URL)
					return nil, nil
				}
			})
			runID := h.create()
			// Construct a historical record only in this test's isolated store.
			body, err := os.ReadFile(h.recordPath(runID))
			if err != nil {
				t.Fatal(err)
			}
			var record, final map[string]any
			if err := json.Unmarshal(body, &record); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.final), &final); err != nil {
				t.Fatal(err)
			}
			record["workflowStage"], record["taskId"], record["final"] = tc.stage, preAuditTaskID, final
			body, err = json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.recordPath(runID), body, 0600); err != nil {
				t.Fatal(err)
			}
			recovering = true
			result, err := h.run("resume", "--run-id", runID, "--interval", "1s", "--deadline", "500ms")
			if err == nil || result["workflowStage"] != "running" || result["taskId"] != preAuditTaskID || pageCalls != 0 || infoCalls != 0 {
				t.Fatalf("unready cache must recheck status: err=%v result=%v pages=%d info=%d", err, result, pageCalls, infoCalls)
			}
			ready = true
			result, err = h.run("resume", "--run-id", runID, "--interval", "1ms", "--deadline", "1s")
			if err != nil || result["workflowStage"] != "completed" || pageCalls != 1 || infoCalls != 1 {
				t.Fatalf("cache recovery must wait for real success: err=%v result=%v pages=%d info=%d", err, result, pageCalls, infoCalls)
			}
		})
	}
}

func TestReviewFixedFlowHelpAndSkill(t *testing.T) {
	for _, tc := range []struct {
		action string
		rules  []string
	}{
		{"info", []string{"先查询 status", "success", "未就绪", "非零"}},
		{"result", []string{"详情未就绪", "继续查询原任务", "截止时间"}},
	} {
		t.Run(tc.action, func(t *testing.T) {
			app, out := alignmentApp(t, nil)
			if err := app.Run(context.Background(), []string{"review", "task", tc.action, "--help"}); err != nil {
				t.Fatal(err)
			}
			for _, rule := range tc.rules {
				if !strings.Contains(out.String(), rule) {
					t.Errorf("%s help lacks fixed-flow rule %q", tc.action, rule)
				}
			}
		})
	}
	t.Run("skill", func(t *testing.T) {
		content := reviewSkill(t, "contract-cli-review")
		for _, rule := range []string{
			"### CLI 固定审查流程",
			"由 CLI 强制执行",
			"`status=running` 时只轮询状态",
			"`status=success` 后才请求 `info`",
			"禁止并行或提前单独调用 `review task info`",
			"`info.status=running`",
			"`ruleResult` 缺失或为 null",
			"不返回 `completed`",
			"合法空数组",
		} {
			if !strings.Contains(content, rule) {
				t.Errorf("review Skill lacks fixed-flow rule %q", rule)
			}
		}
		if strings.Contains(content, "status/info 各查询一次") {
			t.Error("Skill must not imply independent status/info calls have the same prerequisites")
		}
	})
}
