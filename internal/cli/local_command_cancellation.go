package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Result contains only a safe command summary, never a Profile, Token or Secret.
type localCommandOutcome struct {
	Operation string
	Result    any
}

// Gate the actual stdout write too: JSON normalization may finish after a
// cancellation that arrived while the result was being encoded.
type commandResultWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (writer commandResultWriter) Write(body []byte) (int, error) {
	if writer.ctx != nil {
		if err := writer.ctx.Err(); err != nil {
			return 0, err
		}
	}
	return writer.writer.Write(body)
}

type localCommandFailureDetails struct {
	State     string `json:"state"`
	Operation string `json:"operation"`
	Result    any    `json:"result"`
}

type commandCommittedCancellationError struct {
	cause   error
	outcome localCommandOutcome
}

func (err *commandCommittedCancellationError) Error() string {
	return fmt.Sprintf("%s committed changes before cancellation; completed steps were retained: %v", err.outcome.Operation, err.cause)
}

func (err *commandCommittedCancellationError) Unwrap() error { return err.cause }
func (err *commandCommittedCancellationError) commandFailureDetails() any {
	return localCommandFailureDetails{State: "committed", Operation: err.outcome.Operation, Result: err.outcome.Result}
}

func withCompletedLocalCommand(ctx context.Context, err error, outcome *localCommandOutcome) error {
	if ctx == nil || ctx.Err() == nil {
		return err
	}
	cause := withCommandCancellation(ctx, err)
	var existing commandFailureDetailer
	if outcome == nil || errors.As(cause, &existing) {
		return cause
	}
	return &commandCommittedCancellationError{cause: cause, outcome: *outcome}
}
