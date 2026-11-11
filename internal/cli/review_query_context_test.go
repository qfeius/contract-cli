package cli_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestReviewTaskQueryRequiresBusinessID(t *testing.T) {
	for _, action := range []string{"status", "info", "result"} {
		for _, tc := range []struct {
			name string
			args []string
		}{
			{"missing", nil},
			{"empty", []string{"--business-id", ""}},
			{"blank", []string{"--business-id", " \t\n "}},
		} {
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				calls := 0
				app, _ := alignmentApp(t, func(*http.Request) string {
					calls++
					return `{"code":200,"data":{"status":"success","ruleResult":[]}}`
				})
				args := append([]string{"review", "task", action, "--task-id", "123"}, tc.args...)
				err := app.Run(context.Background(), args)
				if err == nil || !strings.Contains(err.Error(), "business-id") || calls != 0 {
					t.Fatalf("missing business context must fail before HTTP: err=%v calls=%d", err, calls)
				}
			})
		}
	}
}

func TestReviewTaskQueryPreservesBusinessID(t *testing.T) {
	for _, action := range []string{"status", "info", "result"} {
		t.Run(action, func(t *testing.T) {
			var paths []string
			statusCalls := 0
			app, _ := alignmentApp(t, func(r *http.Request) string {
				paths = append(paths, r.URL.Path)
				query := r.URL.Query()
				if r.Method != http.MethodGet || query.Get("taskId") != "123" || query.Get("businessId") != "biz-original" || query.Has("visibilityScope") {
					t.Fatalf("original business context changed: %s %s", r.Method, r.URL)
				}
				if strings.HasSuffix(r.URL.Path, "/status") {
					statusCalls++
					if action == "result" && statusCalls == 1 {
						return `{"code":200,"data":{"status":"running"}}`
					}
				}
				return `{"code":200,"data":{"status":"success","ruleResult":[]}}`
			})
			args := []string{"review", "task", action, "--task-id", "123", "--business-id", " \tbiz-original\n "}
			if action == "result" {
				args = append(args, "--interval", "1ms", "--deadline", "1s")
			}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			want := []string{"/open-apis/contract/v3/review/" + action}
			if action == "info" {
				want = []string{"/open-apis/contract/v3/review/status", "/open-apis/contract/v3/review/info"}
			}
			if action == "result" {
				want = []string{"/open-apis/contract/v3/review/status", "/open-apis/contract/v3/review/status", "/open-apis/contract/v3/review/info"}
			}
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("request sequence: got %v, want %v", paths, want)
			}
		})
	}
}

func TestReviewTaskQueryExamples(t *testing.T) {
	for _, action := range []string{"", "status", "info", "result"} {
		name := action
		args := []string{"review"}
		if action != "" {
			args = append(args, "task", action)
		} else {
			name = "root"
		}
		t.Run(name, func(t *testing.T) {
			app, out := alignmentApp(t, nil)
			if err := app.Run(context.Background(), append(args, "--help")); err != nil {
				t.Fatal(err)
			}
			help := out.String()
			if action != "" {
				if !strings.Contains(help, "contract-cli review task "+action+" --task-id <id> --business-id <id> [flags]") {
					t.Error("query usage must require both task-id and business-id")
				}
				required := false
				for _, line := range strings.Split(help, "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "--business-id <id>") && strings.Contains(line, "必填") {
						required = true
					}
				}
				if !required {
					t.Error("query flag must declare business-id required")
				}
			}
			_, examples, ok := strings.Cut(help, "\nExamples:\n")
			if !ok {
				t.Fatal("query help has no examples")
			}
			examples, _, _ = strings.Cut(examples, "\nNotes:")
			exerciseReviewTaskQueryExamples(t, examples)
		})
	}
	t.Run("skill", func(t *testing.T) {
		exerciseReviewTaskQueryExamples(t, reviewSkill(t, "contract-cli-review"))
	})
}

func exerciseReviewTaskQueryExamples(t *testing.T, text string) {
	t.Helper()
	count := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "contract-cli review task ") {
			continue
		}
		line = strings.NewReplacer("<profile>", "contract", "<review-task-id>", "123", "<business-id>", "biz-001").Replace(line)
		fields := strings.Fields(line)
		action := fields[3]
		if action != "status" && action != "info" && action != "result" {
			continue
		}
		count++
		t.Run(action, func(t *testing.T) {
			businessID := ""
			for i := 4; i+1 < len(fields); i++ {
				if fields[i] == "--business-id" {
					businessID = fields[i+1]
				}
			}
			if businessID == "" {
				t.Fatalf("query example must include original business-id: %s", line)
			}
			calls := 0
			app, _ := alignmentApp(t, func(r *http.Request) string {
				calls++
				if r.URL.Query().Get("businessId") != businessID {
					t.Fatalf("example lost business context: %s", r.URL)
				}
				return `{"code":200,"data":{"status":"success","ruleResult":[]}}`
			})
			if err := app.Run(context.Background(), fields[1:]); err != nil {
				t.Fatal(err)
			}
			if calls == 0 {
				t.Fatal("query example did not reach HTTP")
			}
		})
	}
	if count == 0 {
		t.Fatal("no executable query example found")
	}
}
