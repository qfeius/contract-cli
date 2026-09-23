package build_test

import (
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/build"
)

func TestInfoStringIncludesBuildMetadata(t *testing.T) {
	t.Parallel()

	info := build.Info{
		Name:    "contract-cli",
		Version: "v1.2.3",
		Commit:  "abc1234",
		Date:    "2026-04-15T16:30:00+08:00",
	}

	text := info.String()
	for _, fragment := range []string{
		"contract-cli",
		"version v1.2.3",
		"commit abc1234",
		"built 2026-04-15T16:30:00+08:00",
		"feature baseline approval-matrix-extensions since 1.8.3-test.13 (commit 61d8aa6)",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("Info.String() missing %q in %q", fragment, text)
		}
	}
}
