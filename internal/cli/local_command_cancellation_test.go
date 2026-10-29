package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"cn.qfei/contract-cli/internal/config"
)

func TestCompletedLocalCommandCancellationPreservesCauseAndOutcome(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writeFailure := errors.New("fixture output failure")
	outcome := &localCommandOutcome{Operation: "auth logout", Result: map[string]any{"profile": "fixture", "token_cleared": true}}
	err := withCompletedLocalCommand(ctx, writeFailure, outcome)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, writeFailure) {
		t.Fatalf("cancelled completion lost an error: %v", err)
	}
	failure := describeCommandFailure(err)
	details, ok := failure.Details.(localCommandFailureDetails)
	if !ok || details.State != "committed" || details.Operation != "auth logout" || !failure.Cancelled || failure.Type != "cancelled" {
		t.Fatalf("cancelled completion lost safe outcome: %#v", failure)
	}
}

type cancelledFirstResultWrite struct{ cancel context.CancelFunc }

func (writer cancelledFirstResultWrite) Write([]byte) (int, error) {
	writer.cancel()
	return 0, context.Canceled
}

func TestSkillCommittedCancellationAtResultWriteKeepsRecoveryDetails(t *testing.T) {
	app, _, stderr, _ := skillsJSONFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.stdout = cancelledFirstResultWrite{cancel: cancel}
	code := app.Execute(ctx, []string{"skills", "install", "--target", filepath.Join(t.TempDir(), "skills"), "--output", "json"})
	var failure struct {
		Error struct {
			Details struct {
				Recovery struct {
					Status string `json:"status"`
				} `json:"recovery"`
				Installation struct {
					Confirmed bool `json:"confirmed"`
				} `json:"installation"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &failure); err != nil || code != 130 || failure.Error.Details.Recovery.Status != "committed" || !failure.Error.Details.Installation.Confirmed {
		t.Fatalf("committed Skill result write lost actual outcome: code=%d error=%v stderr=%s", code, err, stderr)
	}
}

type cancellationDuringJSONEncoding struct{ cancel context.CancelFunc }

func (value cancellationDuringJSONEncoding) MarshalJSON() ([]byte, error) {
	value.cancel()
	return []byte(`{"safe":"fixture"}`), nil
}

func TestCommandJSONCancelledDuringEncodingDoesNotStartStdoutWrite(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout bytes.Buffer
	app := New(Options{Store: config.NewStore(t.TempDir()), Stdout: &stdout})
	app.commandContext = ctx
	err := app.renderJSONCommandResult(cancellationDuringJSONEncoding{cancel: cancel})
	if !errors.Is(err, context.Canceled) || stdout.Len() != 0 {
		t.Fatalf("JSON encoding cancellation started successful output: err=%v stdout=%s", err, &stdout)
	}
}

func TestCompletedLocalCommandCancellationKeepsExistingDetails(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	original := jsonContextError{cause: context.Canceled}
	err := withCompletedLocalCommand(ctx, original, &localCommandOutcome{Operation: "outer", Result: "must not replace inner details"})
	failure := describeCommandFailure(err)
	details, ok := failure.Details.(map[string]string)
	if !ok || details["target"] != "/safe/path" {
		t.Fatalf("existing details overwritten: %#v", failure.Details)
	}
}

func TestCompletedLocalCommandWithoutCancellationKeepsOriginalError(t *testing.T) {
	t.Parallel()
	original := errors.New("fixture failure")
	if got := withCompletedLocalCommand(context.Background(), original, &localCommandOutcome{}); got != original {
		t.Fatalf("active command error changed: %v", got)
	}
	if got := withCompletedLocalCommand(nil, original, nil); got != original {
		t.Fatalf("nil command context changed error: %v", got)
	}
}
