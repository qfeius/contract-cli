package cli

import (
	"context"
	"encoding/json"
	"testing"
)

func TestParseJSONOutputFormat(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value string
		json  bool
		err   bool
	}{{"", false, false}, {"json", true, false}, {"yaml", false, true}, {"table", false, true}, {"JSON", false, true}} {
		t.Run(test.value, func(t *testing.T) {
			got, err := parseJSONOutputFormat(test.value)
			if got != test.json || (err != nil) != test.err {
				t.Fatalf("format=%q json=%t error=%v", test.value, got, err)
			}
		})
	}
}

func TestCommandRequestsJSONKeepsUpdateAliasScoped(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		args []string
		want bool
	}{
		{[]string{"version", "--output", "json"}, true},
		{[]string{"update", "check", "--json"}, true},
		{[]string{"update", "check", "--json=false"}, false},
		{[]string{"update", "check", "--output=json", "--json=false"}, true},
		{[]string{"update", "check", "--json", "--output", ""}, true},
		{[]string{"update", "check", "--output=", "--json"}, true},
		{[]string{"update", "check", "--json", "--output=", "--json=false"}, false},
		{[]string{"unknown", "--json"}, false},
		{[]string{"update", "check", "--channel", "--json"}, false},
	} {
		clean, options, err := parseCommandLogOptions(test.args)
		if err != nil {
			t.Fatal(err)
		}
		if got := commandRequestsJSON(clean, options); got != test.want {
			t.Fatalf("args=%v json=%t want=%t", test.args, got, test.want)
		}
	}
}

type jsonContextError struct{ cause error }

func (err jsonContextError) Error() string { return "operation failed" }
func (err jsonContextError) Unwrap() error { return err.cause }
func (err jsonContextError) commandFailureDetails() any {
	return map[string]string{"target": "/safe/path"}
}

func TestDescribeCommandFailurePreservesStructuredContextAndCancellation(t *testing.T) {
	t.Parallel()
	failure := describeCommandFailure(jsonContextError{cause: context.Canceled})
	body, err := json.Marshal(failure)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	details, ok := result["details"].(map[string]any)
	if !ok || details["target"] != "/safe/path" || failure.Type != "cancelled" || !failure.Cancelled {
		t.Fatalf("structured failure context or cancellation missing: %s", body)
	}
}
