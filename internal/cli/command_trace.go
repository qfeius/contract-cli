package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/tracecontext"
)

func isBusinessCommand(name string) bool {
	switch name {
	case "api", "contract", "employee", "department", "payment", "mdm", "event", "rule", "review":
		return true
	default:
		return false
	}
}

func (a *App) runWithCommandTrace(parent context.Context, args []string) error {
	ctx, trace, err := tracecontext.NewCommandContext(parent)
	if err != nil {
		a.logger.Error("command trace creation failed", "command", args[0], "error_type", fmt.Sprintf("%T", err))
		return fmt.Errorf("create command trace: %w", err)
	}
	a.commandContext = ctx
	logger := a.commandTraceLogger(ctx)
	logger.Info("command started", "command", args[0])
	err = a.runCommand(ctx, args)
	err = withCommandCancellation(ctx, err)
	if err != nil {
		logger.Error("command failed", "command", args[0], "error_type", fmt.Sprintf("%T", err))
		return withCommandTraceError(trace.TraceID, err)
	}
	logger.Info("command completed", "command", args[0])
	return nil
}

func withCommandCancellation(ctx context.Context, err error) error {
	if ctx.Err() != nil && !errors.Is(err, ctx.Err()) {
		return errors.Join(err, ctx.Err())
	}
	return err
}

func (a *App) commandTraceLogger(ctx context.Context) *slog.Logger {
	if trace, ok := tracecontext.FromContext(ctx); ok {
		return a.logger.With("trace_id", trace.TraceID)
	}
	return a.logger
}

// Add correlation only after command-specific sanitization, keeping the exact
// safe wrapper and error chain if the same ID is already represented.
func withCommandTraceError(id string, err error) error {
	if err == nil || hasCommandTraceError(err, id) {
		return err
	}
	return &openplatform.TraceError{TraceID: id, Cause: err}
}

func hasCommandTraceError(err error, id string) bool {
	if err == nil {
		return false
	}
	if traced, ok := err.(*openplatform.TraceError); ok && traced.TraceID == id {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return slices.ContainsFunc(joined.Unwrap(), func(cause error) bool { return hasCommandTraceError(cause, id) })
	}
	return hasCommandTraceError(errors.Unwrap(err), id)
}
