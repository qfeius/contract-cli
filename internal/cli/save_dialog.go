package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"
)

func defaultSaveFileDialog(ctx context.Context, suggestedName string) (string, error) {
	command, err := buildSaveDialogCommand(runtime.GOOS, suggestedName, exec.LookPath)
	if err != nil {
		slog.Error("save dialog unavailable", "platform", runtime.GOOS)
		return "", err
	}
	return runSaveDialogCommand(ctx, command)
}

type saveDialogCommand struct {
	name string
	args []string
	env  []string
}

func buildSaveDialogCommand(platform, suggestedName string, lookup func(string) (string, error)) (saveDialogCommand, error) {
	name := safeSuggestedFileName(suggestedName)
	switch platform {
	case "darwin":
		return saveDialogCommand{name: "osascript", args: []string{"-e", "on run argv", "-e", `set targetFile to choose file name with prompt "Save contract file as:" default name (item 1 of argv)`, "-e", "return POSIX path of targetFile", "-e", "end run", "--", name}}, nil
	case "windows":
		script := strings.Join([]string{
			"Add-Type -AssemblyName System.Windows.Forms",
			"$dialog = New-Object System.Windows.Forms.SaveFileDialog",
			"$dialog.FileName = $env:CONTRACT_CLI_SAVE_NAME",
			"if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { $dialog.FileName } else { exit 2 }",
		}, "; ")
		return saveDialogCommand{name: "powershell", args: []string{"-NoProfile", "-NonInteractive", "-STA", "-Command", script}, env: []string{"CONTRACT_CLI_SAVE_NAME=" + name}}, nil
	default:
		if path, err := lookup("zenity"); err == nil {
			return saveDialogCommand{name: path, args: []string{"--file-selection", "--save", "--confirm-overwrite", "--filename=./" + name}}, nil
		}
		if path, err := lookup("kdialog"); err == nil {
			return saveDialogCommand{name: path, args: []string{"--getsavefilename", "./" + name}}, nil
		}
		return saveDialogCommand{}, fmt.Errorf("no supported save dialog command found")
	}
}

func runSaveDialogCommand(ctx context.Context, command saveDialogCommand) (string, error) {
	slog.Debug("save dialog started", "command", command.name)
	if err := ctx.Err(); err != nil {
		slog.Debug("save dialog cancelled before start")
		return "", err
	}
	process := exec.CommandContext(ctx, command.name, command.args...)
	process.Env = append(os.Environ(), command.env...)
	output, err := process.Output()
	if err != nil {
		slog.Debug("save dialog stopped", "command", command.name, "error_type", fmt.Sprintf("%T", err))
		return "", err
	}
	path := strings.TrimSpace(string(output))
	if path == "" {
		slog.Debug("save dialog returned no path", "command", command.name)
		return "", fmt.Errorf("save dialog returned empty path")
	}
	return path, nil
}

func safeSuggestedFileName(value string) string {
	name := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(value))
	name = strings.TrimRight(strings.TrimLeft(name, ".- "), ". ")
	if name == "" {
		return "download"
	}
	return name
}
