package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestDebugLoggingImportPlanModes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, format string
		flags        []string
		prefix       bool
	}{
		{name: "quiet JSON", format: "json"},
		{name: "debug text", flags: []string{"--debug"}, format: "table"},
		{name: "debug default text", flags: []string{"--debug"}},
		{name: "debug JSON", flags: []string{"--debug"}, format: "json"},
		{name: "debug before command", flags: []string{"--debug"}, format: "json", prefix: true},
		{name: "explicit false", flags: []string{"--debug=false"}, format: "json"},
		{name: "last debug wins", flags: []string{"--debug", "--debug=false"}, format: "json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var headers []http.Header
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				headers = append(headers, req.Header.Clone())
				return commandTracePlanResponse(t, req), nil
			})
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			app := newDebugLoggingApp(t, transport, stdout, stderr)
			args := commandTracePlanArgs()[:len(commandTracePlanArgs())-2]
			if tc.format != "" {
				args = append(args, "--output", tc.format)
			}
			if tc.prefix {
				args = append(append([]string{}, tc.flags...), args...)
			} else {
				args = append(args, tc.flags...)
			}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			id := assertDebugTraceHeaders(t, headers)
			debug := len(tc.flags) > 0 && tc.flags[len(tc.flags)-1] != "--debug=false"
			if !debug {
				if stderr.Len() != 0 {
					t.Fatalf("default diagnostics must be quiet: %s", stderr)
				}
				return
			}
			if tc.format == "json" {
				assertDebugJSONLogs(t, stderr.Bytes(), id)
				if !json.Valid(stdout.Bytes()) || bytes.Contains(stdout.Bytes(), []byte(`"trace_id"`)) {
					t.Fatalf("business JSON changed: %s", stdout)
				}
			} else if !strings.Contains(stderr.String(), `msg="open platform request started"`) || !strings.Contains(stderr.String(), "trace_id="+id) {
				t.Fatalf("missing text request diagnostics: %s", stderr)
			}
			for _, secret := range []string{"app-token", "user-token", `"cells"`} {
				if strings.Contains(stderr.String(), secret) {
					t.Fatalf("diagnostics leaked %q: %s", secret, stderr)
				}
			}
		})
	}
}

func TestDebugLoggingRestoresBetweenRunsAndErrors(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := newDebugLoggingApp(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return commandTracePlanResponse(t, req), nil
	}), stdout, stderr)
	for _, flags := range [][]string{{"--debug"}, nil, {"--debug=invalid"}, nil} {
		stdout.Reset()
		stderr.Reset()
		err := app.Run(context.Background(), append(commandTracePlanArgs(), flags...))
		if len(flags) > 0 && flags[0] == "--debug=invalid" {
			if err == nil || !strings.Contains(err.Error(), "--debug") {
				t.Fatalf("invalid debug flag error = %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(flags) == 0 && stderr.Len() != 0 {
			t.Fatalf("debug policy leaked to next run: %s", stderr)
		}
	}
}

func TestDebugLoggingFailurePreservesTraceWithoutDefaultLogs(t *testing.T) {
	t.Parallel()
	for _, debug := range []bool{false, true} {
		var sent string
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		app := newDebugLoggingApp(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sent = req.Header.Get("X-Log-Id")
			response := jsonResponse(`{"code":50001,"msg":"test failure"}`)
			response.StatusCode = http.StatusInternalServerError
			return response, nil
		}), stdout, stderr)
		args := commandTracePlanArgs()
		if debug {
			args = append(args, "--debug")
		}
		err := app.Run(context.Background(), args)
		var traced *openplatform.TraceError
		var status *openplatform.HTTPStatusError
		if !errors.As(err, &traced) || traced.TraceID != sent || !errors.As(err, &status) || status.StatusCode != 500 {
			t.Fatalf("failure lost trace/status: %v", err)
		}
		if stdout.Len() != 0 || strings.Contains(stderr.String(), "command completed") {
			t.Fatalf("failure reported as success: stdout=%s stderr=%s", stdout, stderr)
		}
		if debug {
			assertDebugJSONLogs(t, stderr.Bytes(), sent)
		} else if stderr.Len() != 0 {
			t.Fatalf("quiet failure printed internal diagnostics: %s", stderr)
		}
	}
}

func TestDebugLoggingExplicitObserverRemainsCallerControlled(t *testing.T) {
	t.Parallel()
	observer := &bytes.Buffer{}
	app := cli.New(cli.Options{Store: config.NewStore(t.TempDir()), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		Logger:    slog.New(slog.NewJSONHandler(observer, nil)),
		LookupEnv: func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
	})
	_ = app.Run(context.Background(), []string{"unknown-command"})
	if !strings.Contains(observer.String(), `"msg":"run command"`) {
		t.Fatalf("explicit observer no longer receives diagnostics: %s", observer)
	}
}

func TestDebugFlagAvailableInRootAndCommandHelp(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--debug", "help"}, {"rule", "table", "import", "plan", "--help", "--debug"}} {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		app := cli.New(cli.Options{Stdout: stdout, Stderr: stderr, Store: config.NewStore(t.TempDir())})
		if err := app.Run(context.Background(), args); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout.String(), "--debug") || stderr.Len() != 0 {
			t.Fatalf("global flag missing or help emitted diagnostics: stdout=%s stderr=%s", stdout, stderr)
		}
	}
}

func TestDebugLoggingPreservesSingleDashBusinessID(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	var path, trace string
	app := newDebugLoggingApp(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		path, trace = req.URL.Path, req.Header.Get("X-Log-Id")
		return jsonResponse(`{"code":0,"data":{}}`), nil
	}), stdout, stderr)
	err := app.Run(context.Background(), []string{"contract", "get", "-legacy", "--debug", "--profile", "contract", "--as", "app", "--output", "json"})
	if err != nil || !strings.HasSuffix(path, "/-legacy") || trace == "" {
		t.Fatalf("business ID or debug flag was misparsed: path=%s trace=%s error=%v", path, trace, err)
	}
	assertDebugJSONLogs(t, stderr.Bytes(), trace)
}

func newDebugLoggingApp(t *testing.T, transport roundTripFunc, stdout, stderr *bytes.Buffer) *cli.App {
	t.Helper()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
		t.Fatal(err)
	}
	return cli.New(cli.Options{Store: store, Stdout: stdout, Stderr: stderr,
		HTTPClient:         &http.Client{Transport: transport},
		LookupEnv:          func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
	})
}

func assertDebugTraceHeaders(t *testing.T, headers []http.Header) string {
	t.Helper()
	if len(headers) != 3 {
		t.Fatalf("request count = %d, want columns plus two row pages", len(headers))
	}
	id := headers[0].Get("X-Log-Id")
	pattern := regexp.MustCompile(`^00-([0-9a-f]{32})-([0-9a-f]{16})-01$`)
	parents := map[string]bool{}
	for _, header := range headers {
		parts := pattern.FindStringSubmatch(header.Get("traceparent"))
		if parts == nil || parts[1] != id || header.Get("X-Log-Id") != id || parents[parts[2]] {
			t.Fatalf("trace changed with logging policy: %v", header)
		}
		parents[parts[2]] = true
	}
	return id
}

func assertDebugJSONLogs(t *testing.T, logs []byte, id string) {
	t.Helper()
	var request, boundary bool
	for _, line := range bytes.Split(bytes.TrimSpace(logs), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("debug record is not JSON: %s", line)
		}
		if bytes.Count(line, []byte(`"trace_id":`)) > 1 {
			t.Fatalf("duplicate trace_id fields: %s", line)
		}
		switch record["msg"] {
		case "open platform request started":
			request = record["trace_id"] == id
		case "command completed", "command failed":
			boundary = record["trace_id"] == id
		}
	}
	if !request || !boundary {
		t.Fatalf("request or command logs bypassed JSON policy: %s", logs)
	}
}
