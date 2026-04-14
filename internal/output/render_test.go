package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/output"
)

func TestRendererRenderJSON(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	renderer := output.NewRenderer(&stdout)

	if err := renderer.Render(output.FormatJSON, json.RawMessage(`{"code":0,"data":{"vendorId":"123"}}`)); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, `"vendorId": "123"`) {
		t.Fatalf("unexpected json output: %s", got)
	}
}

func TestRendererRenderRaw(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	renderer := output.NewRenderer(&stdout)

	if err := renderer.RenderRaw([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("RenderRaw() error = %v", err)
	}
	if stdout.String() != `{"ok":true}` {
		t.Fatalf("unexpected raw output: %s", stdout.String())
	}
}

func TestRendererRenderTable(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	renderer := output.NewRenderer(&stdout)

	value := []map[string]any{
		{"vendorId": "123", "name": "Acme"},
	}
	if err := renderer.Render(output.FormatTable, value); err != nil {
		t.Fatalf("Render(table) error = %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "vendorId") || !strings.Contains(got, "Acme") {
		t.Fatalf("unexpected table output: %s", got)
	}
}

func TestRendererRenderYAML(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	renderer := output.NewRenderer(&stdout)

	if err := renderer.Render(output.FormatYAML, map[string]any{
		"code": 0,
		"data": map[string]any{"vendorId": "123"},
	}); err != nil {
		t.Fatalf("Render(yaml) error = %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "code: 0") || !strings.Contains(got, "vendorId: \"123\"") {
		t.Fatalf("unexpected yaml output: %s", got)
	}
}
