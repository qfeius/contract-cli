package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
)

func TestSaveDialogFailureRespectsCommandJSONPolicy(t *testing.T) {
	for _, debug := range []bool{false, true} {
		t.Run(fmt.Sprintf("debug=%t", debug), func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
			t.Cleanup(func() { slog.SetDefault(previous) })
			store := config.NewStore(t.TempDir())
			profile := validProductionProfile()
			profile.Identities.User.AuthMode = ""
			profile.Identities.User.Token = &config.Token{AccessToken: "fixture-access"}
			if err := store.UpsertProfile(profile, true); err != nil {
				t.Fatal(err)
			}
			app := New(Options{
				Store: store, Secrets: config.NewSecretsStore(t.TempDir()), Stdout: stdout, Stderr: stderr,
				LookupEnv: func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
				HTTPClient: &http.Client{Transport: deviceRoundTripFunc(func(*http.Request) (*http.Response, error) {
					return jsonResponse(`{"code":0,"data":{"file_id":"123","file_name":"private-name.pdf","download_url":"https://cdn.example.test/file?secret=fixture"}}`), nil
				})},
				SaveFileDialog: func(ctx context.Context, name string) (string, error) {
					cancelled, cancel := context.WithCancel(ctx)
					cancel()
					return defaultSaveFileDialog(cancelled, name)
				},
			})
			args := []string{"contract", "download-file", "123", "--contract", "456", "--as", "user", "--output", "json"}
			if debug {
				args = append(args, "--debug")
			}
			if code := app.Execute(context.Background(), args); code == 0 || stdout.Len() != 0 {
				t.Fatalf("failed dialog reported success: exit=%d stdout=%s", code, stdout)
			}
			lines := bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n"))
			if !debug && len(lines) != 1 {
				t.Fatalf("quiet failure contains extra diagnostics: %s", stderr)
			}
			for _, line := range lines {
				if !json.Valid(line) {
					t.Fatalf("non-JSON diagnostics: %s", stderr)
				}
			}
			if !bytes.Contains(stderr.Bytes(), []byte("--output-file")) || bytes.Contains(stderr.Bytes(), []byte("private-name.pdf")) || bytes.Contains(stderr.Bytes(), []byte("secret=fixture")) {
				t.Fatalf("missing guidance or leaked file data: %s", stderr)
			}
		})
	}
}

func TestSaveDialogCommandPassesNamesAsData(t *testing.T) {
	for _, platform := range []string{"darwin", "windows", "linux"} {
		t.Run(platform, func(t *testing.T) {
			lookup := func(name string) (string, error) { return name, nil }
			first, err := buildSaveDialogCommand(platform, "contract.pdf", lookup)
			if err != nil {
				t.Fatal(err)
			}
			name := "合同’ & (1 + 1) --.pdf"
			second, err := buildSaveDialogCommand(platform, name, lookup)
			if err != nil {
				t.Fatal(err)
			}
			if platform == "windows" {
				if strings.Join(first.args, "\x00") != strings.Join(second.args, "\x00") || len(second.env) != 1 || second.env[0] != "CONTRACT_CLI_SAVE_NAME="+name {
					t.Fatal("Windows name must be child environment data, never script source")
				}
			} else if platform == "darwin" {
				if strings.Join(first.args[:len(first.args)-1], "\x00") != strings.Join(second.args[:len(second.args)-1], "\x00") || second.args[len(second.args)-2] != "--" || second.args[len(second.args)-1] != name {
					t.Fatal("AppleScript source must be fixed and read the name from argv")
				}
			} else if second.args[len(second.args)-1] != "--filename=./"+name {
				t.Fatal("zenity name must be an unambiguous filename argument")
			}
		})
	}
}

func TestSaveDialogNamesCannotBecomePathsOrOptions(t *testing.T) {
	for _, name := range []string{"", "..", "--help", "../x\\y:bad\nname.pdf", "\\\" & (1 + 1) --"} {
		command, err := buildSaveDialogCommand("darwin", name, nil)
		if err != nil {
			t.Fatal(err)
		}
		got := command.args[len(command.args)-1]
		if got == "" || strings.ContainsAny(got, "/\\:\r\n\x00") || strings.HasPrefix(got, "-") || strings.HasPrefix(got, ".") {
			t.Fatalf("unsafe suggested name: %q", got)
		}
	}
}

func TestSaveDialogKDialogFallbackAndCancellation(t *testing.T) {
	command, err := buildSaveDialogCommand("linux", "--help", func(name string) (string, error) {
		if name == "zenity" {
			return "", errors.New("not installed")
		}
		return name, nil
	})
	if err != nil || command.name != "kdialog" || command.args[1] != "./help" {
		t.Fatalf("fallback = %#v, %v", command, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runSaveDialogCommand(ctx, command); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestSaveDialogChildEnvironmentAndSelectedPath(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const name = "合同’; & (1 + 1).pdf"
	t.Setenv("CONTRACT_CLI_SAVE_NAME", "parent value")
	selected := filepath.Join(t.TempDir(), "chosen (1).pdf")
	command := saveDialogCommand{
		name: executable,
		args: []string{"-test.run=^TestSaveDialogHelperProcess$"},
		env:  []string{"CONTRACT_CLI_DIALOG_TEST=1", "CONTRACT_CLI_SAVE_NAME=" + name, "CONTRACT_CLI_DIALOG_SELECTED=" + selected},
	}
	got, err := runSaveDialogCommand(context.Background(), command)
	if err != nil || got != selected {
		t.Fatalf("selected path changed: %q, %v", got, err)
	}
	if os.Getenv("CONTRACT_CLI_SAVE_NAME") != "parent value" {
		t.Fatal("child environment changed the parent")
	}
}

func TestSaveDialogHelperProcess(t *testing.T) {
	if os.Getenv("CONTRACT_CLI_DIALOG_TEST") != "1" {
		return
	}
	if os.Getenv("CONTRACT_CLI_SAVE_NAME") != "合同’; & (1 + 1).pdf" {
		os.Exit(2)
	}
	if _, err := fmt.Fprintln(os.Stdout, os.Getenv("CONTRACT_CLI_DIALOG_SELECTED")); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}
