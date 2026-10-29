package cli

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestCommandLogHandlerDerivedLoggersShareRestorablePolicy(t *testing.T) {
	t.Parallel()
	buffer := &bytes.Buffer{}
	logger, policy := newCommandLogger(buffer)
	child := logger.WithGroup("request").With("trace_id", "test-id")
	child.Info("quiet")
	if buffer.Len() != 0 {
		t.Fatalf("derived logger bypassed quiet policy: %s", buffer)
	}
	previous := policy.current.Swap(&commandLogOptions{debug: true, json: true})
	child.Debug("enabled", slog.Int("attempt", 1))
	var record map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	request, ok := record["request"].(map[string]any)
	if !ok || request["trace_id"] != "test-id" || request["attempt"] != float64(1) {
		t.Fatalf("attrs/group lost: %s", buffer)
	}
	policy.current.Store(previous)
	before := buffer.Len()
	child.Error("quiet again")
	if buffer.Len() != before {
		t.Fatalf("derived logger retained debug policy: %s", buffer)
	}
}
