package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
