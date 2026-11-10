package tracecontext

import (
	"context"
	"fmt"
)

type commandTraceKey struct{}

// NewCommandContext starts a fresh command scope while retaining cancellation,
// deadlines and other parent values. A nested command does not reuse its parent ID.
func NewCommandContext(parent context.Context) (context.Context, RequestTrace, error) {
	if parent == nil {
		return nil, RequestTrace{}, fmt.Errorf("command parent context is required")
	}
	trace, err := NewRequestTrace()
	if err != nil {
		return parent, RequestTrace{}, err
	}
	return context.WithValue(parent, commandTraceKey{}, trace), trace, nil
}

// FromContext reads an existing command trace without creating or changing it.
func FromContext(ctx context.Context) (RequestTrace, bool) {
	if ctx == nil {
		return RequestTrace{}, false
	}
	trace, ok := ctx.Value(commandTraceKey{}).(RequestTrace)
	return trace, ok
}

// ForRequest reuses the command trace, retaining request-level IDs for callers
// that use the client without entering a CLI command scope.
func ForRequest(ctx context.Context) (RequestTrace, error) {
	if trace, ok := FromContext(ctx); ok {
		return trace, nil
	}
	return NewRequestTrace()
}
