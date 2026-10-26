package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

func reviewWriteCases() [][]string {
	var cases [][]string
	for _, resource := range [][]string{{"checklist"}, {"rule", "group"}, {"rule"}} {
		for _, action := range []string{"create", "update", "delete", "batch-create", "batch-update", "batch-delete"} {
			if len(resource) == 2 && strings.HasPrefix(action, "batch-") {
				continue
			}
			args := append([]string{"review"}, resource...)
			args = append(args, action)
			rule := len(resource) == 1 && resource[0] == "rule"
			if rule {
				args = append(args, "--group-id", "g")
			}
			if action == "update" || action == "delete" {
				idFlag := "--id"
				if rule {
					idFlag = "--rule-id"
				}
				args = append(args, idFlag, "r")
			}
			switch action {
			case "create", "update":
				args = append(args, "--data", `{"name":"n"}`)
			case "batch-create", "batch-update":
				args = append(args, "--data", `[{"id":"r","name":"n"}]`)
			case "batch-delete":
				args = append(args, "--data", `["r"]`, "--yes")
			}
			cases = append(cases, args)
		}
	}
	return cases
}

func TestReviewWritesRemoteBoundary(t *testing.T) {
	commands := reviewWriteCases()
	commands = append(commands, []string{"review", "checklist", "list"}, []string{"review", "rule", "group", "list"}, []string{"review", "rule", "list", "--group-id", "g"}, []string{"review", "task", "status", "--task-id", "1"})
	for _, original := range commands {
		args := append([]string{}, original...)
		if strings.Contains(strings.Join(args, " "), " delete ") {
			args = append(args, "--yes")
		}
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			calls := 0
			app, out := alignmentApp(t, func(r *http.Request) string {
				calls++
				deadline, ok := r.Context().Deadline()
				if !ok || time.Until(deadline) > 30*time.Second || time.Until(deadline) <= 0 {
					t.Errorf("missing 30s request deadline")
				}
				return `{"code":40123,"message":"secret-remote-text"}`
			})
			err := app.Run(context.Background(), args)
			if err == nil || !strings.Contains(err.Error(), "40123") {
				t.Fatalf("business failure reported as success: %v", err)
			}
			if strings.Contains(err.Error(), "secret-remote-text") {
				t.Fatal("unsafe remote text exposed")
			}
			if out.Len() != 0 || calls != 1 {
				t.Fatalf("unexpected output or retry: %q, %d", out.String(), calls)
			}
		})
	}
	t.Run("parent deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		deadline, _ := ctx.Deadline()
		app, _ := alignmentApp(t, func(r *http.Request) string {
			got, ok := r.Context().Deadline()
			if !ok || !got.Equal(deadline) {
				t.Fatal("parent deadline extended")
			}
			return `{"code":200,"data":{}}`
		})
		if err := app.Run(ctx, []string{"review", "checklist", "list"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("write cancellation is uncertain and never replayed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		store := config.NewStore(t.TempDir())
		if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
			t.Fatal(err)
		}
		calls := 0
		app := cli.New(cli.Options{Store: store, Stdout: io.Discard, Stderr: io.Discard, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { calls++; cancel(); return nil, r.Context().Err() })}})
		err := app.Run(ctx, []string{"review", "checklist", "create", "--data", `{}`})
		var uncertain *openplatform.UncertainWriteError
		if !errors.As(err, &uncertain) || !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("write semantics changed: %v, calls=%d", err, calls)
		}
	})
	t.Run("localhost delete contract and empty response", func(t *testing.T) {
		var method, path, auth, body string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		profile := uploadProfile(config.IdentityApp)
		store := config.NewStore(t.TempDir())
		if err := store.UpsertProfile(profile, true); err != nil {
			t.Fatal(err)
		}
		out := new(bytes.Buffer)
		target, _ := url.Parse(server.URL)
		localClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			local := r.Clone(r.Context())
			local.URL.Scheme, local.URL.Host = target.Scheme, target.Host
			return http.DefaultTransport.RoundTrip(local)
		})}
		app := cli.New(cli.Options{Store: store, Stdout: out, Stderr: io.Discard, HTTPClient: localClient})
		if err := app.Run(context.Background(), []string{"review", "rule", "batch-delete", "--group-id", "g", "--id", "a,b", "--yes"}); err != nil {
			t.Fatal(err)
		}
		if method != "DELETE" || path != "/open-apis/contract/v1/review-rules/review-rule-groups/g/rules/batch" || auth != "Bearer user-token" || body != `{"ids":["a","b"]}` || out.Len() != 0 {
			t.Fatalf("contract mismatch %s %s %s", method, path, body)
		}
	})
}

func TestReviewWritesBatchIDs(t *testing.T) {
	for _, prefix := range [][]string{{"review", "checklist", "batch-delete"}, {"review", "rule", "batch-delete", "--group-id", "g"}} {
		for _, flags := range [][]string{{"--id", "a,b", "--id", " a "}, {"--data", `[" a ","b","a"]`}} {
			for _, preview := range []string{"--dry-run", "--print-input", ""} {
				t.Run(strings.Join(prefix, " ")+strings.Join(flags, " ")+preview, func(t *testing.T) {
					var sent map[string]any
					app, out := alignmentApp(t, func(r *http.Request) string {
						if preview != "" {
							t.Fatal("preview sent request")
						}
						if r.Method != "DELETE" || !strings.HasSuffix(r.URL.Path, "/batch") {
							t.Fatal(r.URL)
						}
						sent = alignmentBody(t, r)
						return `{"code":200,"data":{}}`
					})
					args := append(append([]string{}, prefix...), flags...)
					if preview != "" {
						args = append(args, preview)
					} else {
						args = append(args, "--yes")
					}
					if err := app.Run(context.Background(), args); err != nil {
						t.Fatal(err)
					}
					if preview != "" {
						if err := json.Unmarshal(out.Bytes(), &sent); err != nil {
							t.Fatal(err)
						}
					}
					if !reflect.DeepEqual(sent["ids"], []any{"a", "b"}) {
						t.Fatalf("wrong IDs: %v", sent)
					}
				})
			}
		}
		for _, flags := range [][]string{{"--id", "a", "--data", `["b"]`}, {"--id", "a,,b"}, {"--data", `["a",""]`}, {"--data", `[]`}, {"--data", `[1]`}, {"--data", `[null]`}, {"--data", `["a"]`, "--input", "unused.json"}} {
			app, _ := alignmentApp(t, nil)
			args := append(append(append([]string{}, prefix...), flags...), "--yes", "--dry-run")
			if err := app.Run(context.Background(), args); err == nil {
				t.Fatalf("invalid IDs accepted: %v", flags)
			}
		}
		app, _ := alignmentApp(t, nil)
		if err := app.Run(context.Background(), append(append([]string{}, prefix...), "--id", "a")); err == nil || !strings.Contains(err.Error(), "--yes") {
			t.Fatalf("confirmation: %v", err)
		}
	}
}

func TestReviewWritesDispatchPreview(t *testing.T) {
	for _, args := range reviewWriteCases() {
		for _, preview := range []string{"--dry-run", "--print-input"} {
			t.Run(strings.Join(args[:3], " ")+strings.Join(args[3:], " ")+preview, func(t *testing.T) {
				app, out := alignmentApp(t, nil)
				if err := app.Run(context.Background(), append(append([]string{}, args...), preview)); err != nil {
					t.Fatal(err)
				}
				if out.Len() == 0 {
					t.Fatal("preview missing")
				}
			})
		}
	}
	for _, action := range []string{"update", "delete"} {
		t.Run("rule "+action, func(t *testing.T) {
			called := false
			app, _ := alignmentApp(t, func(r *http.Request) string {
				called = true
				want := "PUT"
				if action == "delete" {
					want = "DELETE"
				}
				if r.Method != want || r.URL.Path != "/open-apis/contract/v1/review-rules/review-rule-groups/g/rules/r" {
					t.Fatalf("wrong request %s %s", r.Method, r.URL.Path)
				}
				return `{"code":200,"data":{}}`
			})
			args := []string{"review", "rule", action, "--group-id", "g", "--rule-id", "r"}
			if action == "update" {
				args = append(args, "--data", `{"name":"n"}`)
			} else {
				args = append(args, "--yes")
			}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("request missing")
			}
		})
	}
	for _, args := range [][]string{{"review", "rule", "delete", "--group-id", "g", "--rule-id", "r"}, {"review", "checklist", "delete", "--id", "r"}, {"review", "rule", "group", "delete", "--id", "r"}} {
		app, _ := alignmentApp(t, nil)
		if err := app.Run(context.Background(), args); err == nil || !strings.Contains(err.Error(), "--yes") {
			t.Fatalf("missing confirmation: %v", err)
		}
	}
}

func TestReviewWritesBodyValidation(t *testing.T) {
	for _, prefix := range [][]string{{"review", "checklist"}, {"review", "rule", "group"}, {"review", "rule"}} {
		rule := len(prefix) == 2 && prefix[1] == "rule"
		for _, action := range []string{"create", "update", "batch-create", "batch-update"} {
			if len(prefix) == 3 && strings.HasPrefix(action, "batch-") {
				continue
			}
			base := append(append([]string{}, prefix...), action)
			if rule {
				base = append(base, "--group-id", "g")
			}
			if action == "update" {
				flag := "--id"
				if rule {
					flag = "--rule-id"
				}
				base = append(base, flag, "r")
			}
			bad := []string{`null`, `[]`, `1`, `"x"`, `true`}
			if strings.HasPrefix(action, "batch-") {
				bad = []string{`null`, `[]`, `{}`, `[null]`, `[1]`, `[[]]`}
			}
			if action == "update" {
				bad = append(bad, `{"id":"other"}`, `{"id":1}`, `{"id":null}`)
			}
			for _, body := range bad {
				t.Run(strings.Join(base, " ")+body, func(t *testing.T) {
					app, _ := alignmentApp(t, nil)
					if err := app.Run(context.Background(), append(append([]string{}, base...), "--data", body, "--dry-run")); err == nil {
						t.Fatalf("invalid body accepted: %s", body)
					}
				})
			}
		}
	}
	for _, flags := range [][]string{{"--data", `{}`, "--input", "unused.json"}, {"--input", "one.json", "--input-file", "two.json"}} {
		app, _ := alignmentApp(t, nil)
		err := app.Run(context.Background(), append(append([]string{"review", "checklist", "create"}, flags...), "--dry-run"))
		if err == nil || !strings.Contains(err.Error(), "只能") {
			t.Fatalf("expected mutually exclusive inputs: %v", err)
		}
	}
	called := false
	app, _ := alignmentApp(t, func(r *http.Request) string {
		called = true
		b := alignmentBody(t, r)
		if b["large"] != json.Number("1175703921365090648") || b["id"] != "r" || b["extension"] != "kept" {
			t.Fatalf("body changed: %v", b)
		}
		return `{"code":200,"data":{}}`
	})
	if err := app.Run(context.Background(), []string{"review", "checklist", "update", "--id", "r", "--data", `{"id":"r","large":1175703921365090648,"extension":"kept"}`}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("request missing")
	}
}

func TestReviewWritesHelp(t *testing.T) {
	for _, args := range reviewWriteCases() {
		end := len(args)
		for i, arg := range args {
			if strings.HasPrefix(arg, "--") {
				end = i
				break
			}
		}
		path := args[:end]
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			app, out := alignmentApp(t, nil)
			if err := app.Run(context.Background(), append(append([]string{}, path...), "--help")); err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{"Name:", "Usage:", "Flags:", "Examples:", "Notes:", "--dry-run", "--print-input", "30"} {
				if !strings.Contains(out.String(), required) {
					t.Errorf("missing help %s", required)
				}
			}
			for _, forbidden := range []string{"暂不可用", "不支持 --dry-run", "无法通过", "/open-apis/", "当前不支持 --id"} {
				if strings.Contains(out.String(), forbidden) {
					t.Errorf("stale help %s", forbidden)
				}
			}
			if path[1] == "rule" && (path[2] == "update" || path[2] == "delete") && !strings.Contains(out.String(), "--rule-id <id>") {
				t.Fatal("missing rule ID")
			}
		})
	}
}
