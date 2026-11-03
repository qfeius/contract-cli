package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
)

func TestNonBusinessJSONHelpDocumentsOutputFlag(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"config add", "auth login", "auth status", "auth logout", "auth use", "version", "skills list", "skills install", "update check"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			args := append(strings.Fields(command), "--help")
			if err := cli.New(cli.Options{Stdout: &stdout}).Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), "--output <json>") {
				t.Fatalf("help does not advertise explicit JSON output: %s", stdout.String())
			}
		})
	}
}
