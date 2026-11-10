package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/tracecontext"
)

func TestCommandTraceErrorPreservesSafeWrappersAndJoinedBranches(t *testing.T) {
	t.Parallel()
	id := "11111111111111111111111111111111"
	cause := errors.New("transport stopped")
	traced := &openplatform.TraceError{TraceID: id, Cause: cause}
	safe := &directoryRequestError{cause: fmt.Errorf("private query: %w", traced), message: "safe directory error"}
	joined := errors.Join(&openplatform.TraceError{TraceID: "other", Cause: errors.New("other")}, safe)
	got := withCommandTraceError(id, joined)
	if got != joined || !errors.Is(got, cause) || strings.Contains(got.Error(), "private query") {
		t.Fatalf("safe wrapper or error identity was lost: %v", got)
	}
	if withCommandTraceError(id, nil) != nil {
		t.Fatal("nil command error was wrapped")
	}
	wrapped := withCommandTraceError(id, cause)
	var actual *openplatform.TraceError
	if !errors.As(wrapped, &actual) || actual.TraceID != id || !errors.Is(wrapped, cause) {
		t.Fatalf("command trace error = %v", wrapped)
	}
	if withCommandTraceError(id, fmt.Errorf("outer: %w", wrapped)).Error() != "outer: "+wrapped.Error() {
		t.Fatal("trace ID was appended twice")
	}
}

func TestCommandTraceRestoresContextForAllReturnPaths(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"help"}, {"version"}, {"environment", "inspect"}, {"auth", "status"}, {"contract", "get"}, {"unknown"}} {
		app := New(Options{Store: config.NewStore(t.TempDir()), Stdout: io.Discard, Stderr: io.Discard,
			LookupEnv:          func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
			InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
		})
		old, _, err := tracecontext.NewCommandContext(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		app.commandContext = old
		logger := app.logger
		err = app.Run(context.Background(), args)
		var traced *openplatform.TraceError
		if errors.As(err, &traced) != (args[0] == "contract") {
			t.Fatalf("trace scope for %v: %v", args, err)
		}
		if app.commandContext != old || app.logger != logger {
			t.Fatalf("command state leaked after %v", args)
		}
	}
}

func TestCommandTracePropagatesParentAndPreservesCancellation(t *testing.T) {
	t.Parallel()
	store := config.NewStore(t.TempDir())
	profile := validProductionProfile()
	profile.Identities.App.Token = &config.Token{AccessToken: "test-token", Expiry: time.Now().Add(time.Hour)}
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	parent, inherited, err := tracecontext.NewCommandContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	parent, cancel := context.WithDeadline(parent, time.Now().Add(time.Minute))
	defer cancel()
	var sent string
	var app *App
	transport := productionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		trace, ok := tracecontext.FromContext(req.Context())
		if !ok || trace.TraceID == inherited.TraceID || app.commandContext != req.Context() {
			t.Fatalf("command context not installed before request: %v, %v", trace, ok)
		}
		if _, ok := req.Context().Deadline(); !ok {
			t.Fatal("deadline was lost")
		}
		sent = req.Header.Get("X-Log-Id")
		cancel()
		return nil, req.Context().Err()
	})
	app = New(Options{Store: store, Stdout: io.Discard, Stderr: io.Discard, HTTPClient: &http.Client{Transport: transport},
		LookupEnv:          func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
	})
	err = app.Run(parent, []string{"contract", "get", "123", "--profile", "contract", "--as", "app"})
	var traced *openplatform.TraceError
	if !errors.Is(err, context.Canceled) || !errors.As(err, &traced) || traced.TraceID != sent || sent == "" {
		t.Fatalf("cancellation or trace identity lost: %v", err)
	}
	if strings.Count(err.Error(), "trace_id=") != 1 || app.commandContext != nil {
		t.Fatalf("trace or context leaked: %v", err)
	}
	if app.commandTraceLogger(context.Background()) != app.logger {
		t.Fatal("unscoped logger was unexpectedly changed")
	}
}
