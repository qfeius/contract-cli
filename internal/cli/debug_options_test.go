package cli

import (
	"reflect"
	"testing"
)

func TestParseCommandLogOptionsPreservesCommandArguments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		args, clean []string
		debug, json bool
	}{
		{"leading flag", []string{"--debug", "approval-matrix", "table", "get"}, []string{"approval-matrix", "table", "get"}, true, false},
		{"JSON", []string{"rule", "--debug", "--output=json"}, []string{"rule", "--output=json"}, true, true},
		{"last output", []string{"rule", "--output", "json", "--debug", "--output", "table"}, []string{"rule", "--output", "json", "--output", "table"}, true, false},
		{"repeated debug", []string{"rule", "--debug=false", "--debug=true"}, []string{"rule"}, true, false},
		{"separate false is positional", []string{"rule", "--debug", "false"}, []string{"rule", "false"}, true, false},
		{"value", []string{"rule", "--data", "--debug"}, []string{"rule", "--data", "--debug"}, false, false},
		{"inline value", []string{"rule", "--data=--debug"}, []string{"rule", "--data=--debug"}, false, false},
		{"profile value", []string{"auth", "status", "--profile", "--debug"}, []string{"auth", "status", "--profile", "--debug"}, false, false},
		{"single dash value", []string{"auth", "status", "-profile", "--debug"}, []string{"auth", "status", "-profile", "--debug"}, false, false},
		{"single dash business ID", []string{"contract", "get", "-legacy", "--debug"}, []string{"contract", "get", "-legacy"}, true, false},
		{"output value", []string{"rule", "--output", "--debug"}, []string{"rule", "--output", "--debug"}, false, false},
		{"terminator", []string{"rule", "--", "--debug", "--output=json"}, []string{"rule", "--", "--debug", "--output=json"}, false, false},
		{"boolean precedes debug", []string{"contract", "--raw", "--debug", "--force=false"}, []string{"contract", "--raw", "--force=false"}, true, false},
		{"update JSON", []string{"update", "check", "--json", "--debug"}, []string{"update", "check", "--json"}, true, true},
		{"update JSON bool shorthand", []string{"update", "check", "--json=t", "--debug"}, []string{"update", "check", "--json=t"}, true, true},
		{"update JSON false", []string{"update", "check", "--json=false", "--debug"}, []string{"update", "check", "--json=false"}, true, false},
		{"review stdin debug", []string{"review", "file", "upload", "--stdin", "--debug"}, []string{"review", "file", "upload", "--stdin"}, true, false},
		{"review dry run JSON", []string{"review", "run", "--dry-run", "--output", "json"}, []string{"review", "run", "--dry-run", "--output", "json"}, false, true},
		{"review print input debug", []string{"review", "run", "--print-input", "--debug"}, []string{"review", "run", "--print-input"}, true, false},
		{"review yes debug", []string{"review", "rule", "delete", "--yes", "--debug"}, []string{"review", "rule", "delete", "--yes"}, true, false},
		{"review enabled debug", []string{"review", "rule", "list", "--enabled", "--debug"}, []string{"review", "rule", "list", "--enabled"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := append([]string{}, tc.args...)
			clean, options, err := parseCommandLogOptions(tc.args)
			if err != nil || !reflect.DeepEqual(clean, tc.clean) || options.debug != tc.debug || options.json != tc.json {
				t.Fatalf("parse = %v, %+v, %v", clean, options, err)
			}
			if !reflect.DeepEqual(original, tc.args) {
				t.Fatal("parser mutated caller arguments")
			}
		})
	}
	if _, _, err := parseCommandLogOptions([]string{"--debug=not-a-bool", "version"}); err == nil {
		t.Fatal("invalid debug value accepted")
	}
}
