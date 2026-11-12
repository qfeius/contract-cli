package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestLogRedactionRecognizesFlagsNotCommandsOrValues(t *testing.T) {
	for _, args := range [][]string{
		{"contract", "summary", "query", "--data", "PRIVATE_BODY"},
		{"contract", "approval", "comment", "--data", "PRIVATE_BODY"},
		{"--profile", "secret", "--app-secret", "PRIVATE_KEY"},
		{"--data", "--app-secret", "PRIVATE_KEY"},
		{"--data", "--app-secret=PRIVATE_KEY"},
		{"--data", "--query", "PRIVATE_QUERY"},
		{"--app-secret", "-PRIVATE_KEY"},
		{"--certification-id", "PRIVATE_ID", "--contract-name=PRIVATE_NAME", "--contract-number", "PRIVATE_NUMBER", "--user-id", "PRIVATE_USER"},
	} {
		before := append([]string(nil), args...)
		got := redactCommandArgs(args)
		if strings.Contains(got, "PRIVATE_") || !reflect.DeepEqual(args, before) {
			t.Fatalf("leaked value or mutated arguments: %s", got)
		}
	}
	for _, name := range []string{"query", "comment", "secret", "---", "--"} {
		if isSensitiveArgument(name) {
			t.Fatalf("not a sensitive flag: %s", name)
		}
	}
	if got := redactCommandArgs([]string{"contract", "summary", "query", "--profile", "secret", "--limit", "10"}); got != "contract summary query --profile secret --limit 10" {
		t.Fatal(got)
	}
}
