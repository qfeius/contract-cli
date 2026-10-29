package tracecontext_test

import (
	"context"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/tracecontext"
)

func TestCommandContextPreservesParentAndIsolatesCommands(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(time.Minute)
	parent, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	ctx, first, err := tracecontext.NewCommandContext(parent)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := tracecontext.FromContext(ctx); !ok || got != first {
		t.Fatalf("FromContext() = %v, %v", got, ok)
	}
	if got, ok := ctx.Deadline(); !ok || !got.Equal(deadline) {
		t.Fatalf("deadline = %v, %v", got, ok)
	}
	assertFreshCommandContext(t, ctx, first.TraceID)
	cancel()
	if ctx.Err() != context.Canceled {
		t.Fatalf("command context cancellation = %v", ctx.Err())
	}
}

func assertFreshCommandContext(t *testing.T, parent context.Context, prior string) {
	t.Helper()
	ctx, next, err := tracecontext.NewCommandContext(parent)
	if err != nil {
		t.Fatal(err)
	}
	if next.TraceID == prior {
		t.Fatal("new command reused its parent's trace ID")
	}
	for range 2 {
		got, err := tracecontext.ForRequest(ctx)
		if err != nil || got != next {
			t.Fatalf("ForRequest() = %v, %v, want %v", got, err, next)
		}
	}
}

func TestRequestContextFallbackDoesNotCreateCommandScope(t *testing.T) {
	t.Parallel()
	for _, ctx := range []context.Context{nil, context.Background()} {
		if _, ok := tracecontext.FromContext(ctx); ok {
			t.Fatal("empty context has a command trace")
		}
		first, err := tracecontext.ForRequest(ctx)
		if err != nil {
			t.Fatal(err)
		}
		second, err := tracecontext.ForRequest(ctx)
		if err != nil || first == second {
			t.Fatalf("fallback request traces = %v, %v, error = %v", first, second, err)
		}
	}
	if _, _, err := tracecontext.NewCommandContext(nil); err == nil {
		t.Fatal("nil parent must return an error")
	}
}
