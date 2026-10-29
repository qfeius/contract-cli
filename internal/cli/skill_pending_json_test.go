package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/skillsync"
)

func TestPendingSkillWarningPreservesJSONErrorStream(t *testing.T) {
	t.Parallel()
	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending-location", true: "unreadable-registry"}[corrupt], func(t *testing.T) {
			t.Parallel()
			app, stdout, stderr, registry, target := pendingSkillWarningApp(t, corrupt)
			before, err := os.ReadFile(registry)
			if err != nil {
				t.Fatal(err)
			}
			if code := app.Execute(context.Background(), []string{"unsupported-command", "--output", "json"}); code != 1 || stdout.Len() != 0 {
				t.Fatalf("failure changed code or stdout: code=%d stdout=%s", code, stdout)
			}
			lines := bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n"))
			if len(lines) != 2 {
				t.Fatalf("want one JSON warning and one JSON error: %s", stderr)
			}
			var warning struct {
				Level string `json:"level"`
				Msg   string `json:"msg"`
			}
			if err := json.Unmarshal(lines[0], &warning); err != nil || warning.Level != "WARN" || !strings.Contains(warning.Msg, "Official Skill synchronization incomplete:") {
				t.Fatalf("pending warning is not structured JSON: %s", stderr)
			}
			if corrupt {
				if !strings.Contains(warning.Msg, "登记读取失败") || strings.Contains(warning.Msg, "Repair:") {
					t.Fatalf("registry failure guidance changed: %s", warning.Msg)
				}
			} else if !strings.Contains(warning.Msg, "Repair:") || !strings.Contains(warning.Msg, target) || !strings.Contains(warning.Msg, "copy \"failed\"\ncheck target") {
				t.Fatalf("pending location guidance lost: %s", warning.Msg)
			}
			var failure struct {
				OK    *bool `json:"ok"`
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(lines[1], &failure) != nil || failure.OK == nil || *failure.OK || !strings.Contains(failure.Error.Message, "unknown command") {
				t.Fatalf("final JSON error changed: %s", lines[1])
			}
			after, err := os.ReadFile(registry)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("warning modified registration: %v", err)
			}
		})
	}
}

func TestPendingSkillWarningRemainsVisibleWithoutDebug(t *testing.T) {
	t.Parallel()
	app, stdout, stderr, _, target := pendingSkillWarningApp(t, false)
	if err := app.Run(context.Background(), []string{"version"}); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() == 0 || !strings.HasPrefix(stderr.String(), "Official Skill synchronization incomplete:") || !strings.Contains(stderr.String(), target) || !strings.Contains(stderr.String(), "Repair:") || json.Valid(stderr.Bytes()) {
		t.Fatalf("default user guidance changed: stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestPendingSkillWarningDoesNotCreateMissingRegistration(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := cli.New(cli.Options{Store: config.NewStore(state), Secrets: config.NewSecretsStore(state), Stdout: stdout, Stderr: stderr})
	if err := app.Run(context.Background(), []string{"version", "--output", "json"}); err != nil || stderr.Len() != 0 || stdout.Len() == 0 {
		t.Fatalf("empty registration produced warning or changed version: %v stderr=%s", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(state, "skill-locations.json")); !os.IsNotExist(err) {
		t.Fatalf("warning created missing registration: %v", err)
	}
}

func TestSkillInstallParameterFailurePreservesSelectedErrorFormat(t *testing.T) {
	t.Parallel()
	for _, jsonOutput := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "json"}[jsonOutput], func(t *testing.T) {
			t.Parallel()
			state := t.TempDir()
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			app := cli.New(cli.Options{Store: config.NewStore(state), Secrets: config.NewSecretsStore(state), Stdout: stdout, Stderr: stderr,
				LookupEnv: func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
			})
			args := []string{"skills", "install", "--unknown"}
			if jsonOutput {
				args = []string{"skills", "install", "--output", "json", "--unknown"}
			}
			if code := app.Execute(context.Background(), args); code != 1 {
				t.Fatalf("install argument failure exit = %d", code)
			}
			if !jsonOutput && (!strings.Contains(stdout.String(), "CLI 实际可执行文件：") || !strings.Contains(stdout.String(), "Skill 登记文件：")) {
				t.Fatalf("install identity or registration output changed: %s", stdout)
			}
			if jsonOutput && stdout.Len() != 0 {
				t.Fatalf("JSON parameter failure wrote stdout: %s", stdout)
			}
			if jsonOutput {
				if !json.Valid(stderr.Bytes()) || bytes.Count(bytes.TrimSpace(stderr.Bytes()), []byte("\n")) != 0 {
					t.Fatalf("install JSON error mixed human diagnostics: %s", stderr)
				}
			} else if !strings.Contains(stderr.String(), "Skill 安装：失败") || !strings.Contains(stderr.String(), "error: ") {
				t.Fatalf("default install failure guidance changed: %s", stderr)
			}
			if _, err := os.Stat(filepath.Join(state, "skill-locations.json")); !os.IsNotExist(err) {
				t.Fatalf("invalid install arguments changed registration: %v", err)
			}
		})
	}
}

func TestSkillInstallCorruptRegistryJSONErrorKeepsPendingWarning(t *testing.T) {
	t.Parallel()
	app, stdout, stderr, registry, _ := pendingSkillWarningApp(t, true)
	before, err := os.ReadFile(registry)
	if err != nil {
		t.Fatal(err)
	}
	if code := app.Execute(context.Background(), []string{"skills", "install", "--output", "json", "--unknown"}); code != 1 {
		t.Fatalf("install parameter failure exit = %d", code)
	}
	lines := bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n"))
	if len(lines) != 2 || !json.Valid(lines[0]) || !json.Valid(lines[1]) {
		t.Fatalf("want pending JSON warning plus final JSON error: %s", stderr)
	}
	var warning struct {
		Msg string `json:"msg"`
	}
	if json.Unmarshal(lines[0], &warning) != nil || !strings.Contains(warning.Msg, "登记读取失败") {
		t.Fatalf("registry failure notice was discarded: %s", stderr)
	}
	var failure struct {
		OK *bool `json:"ok"`
	}
	if json.Unmarshal(lines[1], &failure) != nil || failure.OK == nil || *failure.OK {
		t.Fatalf("final JSON error missing: %s", stderr)
	}
	if stdout.Len() != 0 {
		t.Fatalf("JSON install failure wrote human identity/registration to stdout: %s", stdout)
	}
	after, err := os.ReadFile(registry)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("parameter failure modified corrupt registration: %v", err)
	}
}

func pendingSkillWarningApp(t *testing.T, corrupt bool) (*cli.App, *bytes.Buffer, *bytes.Buffer, string, string) {
	t.Helper()
	state, target := t.TempDir(), filepath.Join(t.TempDir(), "skills")
	registry := filepath.Join(state, "skill-locations.json")
	body := []byte("broken registry")
	if !corrupt {
		var err error
		body, err = json.Marshal(skillsync.Registry{Schema: 1, Locations: []skillsync.Location{{
			Target: target, ID: "pending-location", Managed: []string{"auth"}, Status: "failed", Error: "copy \"failed\"\ncheck target",
		}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(registry, body, 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := cli.New(cli.Options{Store: config.NewStore(state), Secrets: config.NewSecretsStore(state), Stdout: stdout, Stderr: stderr,
		LookupEnv: func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
	})
	return app, stdout, stderr, registry, target
}
