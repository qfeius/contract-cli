//go:build linux || darwin

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Build the real entrypoint; the overlay stalls only native identity inspection.
// The child sleep inherits the helper process group, matching codesign/plutil.
func TestInterruptPreservesInputAndInspectionCleanup(t *testing.T) {
	directory := t.TempDir()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	source := "identity_other.go"
	if runtime.GOOS == "darwin" {
		source = "identity_darwin.go"
	}
	replacement := filepath.Join(directory, source)
	const stalledIdentity = `package invocation
import ("fmt"; "os"; "os/exec"; "time")
func platformApplicationIdentities([]Process) ([]ApplicationIdentity, []string) {
    child := exec.Command("sleep", "60")
    if err := child.Start(); err != nil { panic(err) }
    body := []byte(fmt.Sprintf("%d %d", os.Getpid(), child.Process.Pid))
    if err := os.WriteFile(os.Getenv("QFEI_INSPECTION_TEST_PID"), body, 0600); err != nil { panic(err) }
    time.Sleep(time.Minute)
    return nil, nil
}`
	if err := os.WriteFile(replacement, []byte(stalledIdentity), 0600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{
		filepath.Join(root, "internal/invocation", source): replacement,
	}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(directory, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	binary := filepath.Join(directory, "contract-cli")
	build := exec.CommandContext(ctx, "go", "build", "-overlay", overlayPath, "-o", binary, "./cmd/contract-cli")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	t.Setenv("CONTRACT_CLI_CONFIG_DIR", filepath.Join(directory, "profile"))
	t.Setenv("CONTRACT_CLI_NO_UPDATE_CHECK", "1")

	for _, tc := range []struct {
		name  string
		flags []string
		json  bool
	}{
		{"default", nil, false},
		{"json-alias", []string{"--json"}, false},
		{"explicit-json", []string{"--output", "json"}, true},
		{"inline-json", []string{"--output=json"}, true},
		{"yaml", []string{"--output", "yaml"}, false},
		{"body-contains-output", []string{"--data", `{"example":"--output json"}`}, false},
		{"flag-value-is-output", []string{"--label", "--output", "json"}, false},
		{"after-terminator", []string{"--", "--output", "json"}, false},
	} {
		t.Run("error-output-format/"+tc.name, func(t *testing.T) {
			verifyProcessFailureFormat(t, ctx, binary, tc.flags, tc.json)
		})
	}
	for _, mode := range []string{"text", "json"} {
		for _, input := range []string{"stdin-pipe", "stdin-partial", "fifo-without-writer", "stalled-inspection"} {
			t.Run(mode+"/"+input, func(t *testing.T) {
				verifyInterruptedProcess(t, ctx, binary, input, mode)
			})
		}
	}
}

func verifyInterruptedProcess(t *testing.T, ctx context.Context, binary, input, mode string) {
	t.Helper()
	directory := t.TempDir()
	pidFile := filepath.Join(directory, "helper.pid")
	t.Setenv("QFEI_INSPECTION_TEST_PID", pidFile)
	args := []string{"contract", "search", "--input-file", "/dev/stdin", "--debug"}
	if input == "stalled-inspection" {
		args = []string{"environment", "inspect", "--debug"}
	}
	if input == "fifo-without-writer" {
		fifo := filepath.Join(directory, "input.fifo")
		if err := syscall.Mkfifo(fifo, 0600); err != nil {
			t.Fatal(err)
		}
		args[3] = fifo
	}
	if mode == "json" {
		args = append(args, "--output", "json")
	}
	command := exec.CommandContext(ctx, binary, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdoutPath, stderrPath := filepath.Join(directory, "stdout"), filepath.Join(directory, "stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.Create(stderrPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	command.Stdout, command.Stderr = stdout, stderr
	var inputWriter io.WriteCloser
	if strings.HasPrefix(input, "stdin-") {
		inputWriter, err = command.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		defer inputWriter.Close() // Keep the writer open until the child exits.
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	exited, helperPID, descendantPID := false, 0, 0
	defer func() {
		if !exited {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-done
		}
		if helperPID > 0 {
			_ = syscall.Kill(-helperPID, syscall.SIGKILL)
		}
	}()
	waitInterruptCondition(t, 4*time.Second, func() bool {
		if input != "stalled-inspection" {
			body, _ := os.ReadFile(stderrPath)
			// This existing boundary log is emitted after main has installed its
			// handler and immediately before parsing/reading command input.
			return strings.Contains(string(body), "search contracts")
		}
		body, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		_, _ = fmt.Sscanf(string(body), "%d %d", &helperPID, &descendantPID)
		return helperPID > 0 && descendantPID > 0
	}, "command did not reach its input/inspection boundary")
	if strings.HasPrefix(input, "stdin-") {
		waitInterruptCondition(t, 3*time.Second, func() bool { return interruptInputPipeOpened(command.Process.Pid) }, "command did not open its JSON input pipe")
	}
	if input == "stdin-partial" {
		if _, err := inputWriter.Write([]byte(`{"partial":`)); err != nil {
			t.Fatal(err)
		}
		// Poll can now return data; let the reader consume the partial JSON and
		// block waiting for more input while the writer deliberately stays open.
		time.Sleep(100 * time.Millisecond)
	}
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		exited = true
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 130 {
			t.Errorf("first interrupt should cancel and exit 130: %v, status=%v", err, command.ProcessState)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first interrupt did not terminate the blocked command within two seconds")
	}
	output, err := os.ReadFile(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 0 {
		t.Errorf("interrupted command emitted business output: %s", output)
	}
	diagnostic, err := os.ReadFile(stderrPath)
	if err != nil {
		t.Fatal(err)
	}
	assertInterruptedDiagnostic(t, diagnostic, mode, input != "stalled-inspection")
	if strings.Contains(string(diagnostic), "open platform request started") || strings.Contains(string(diagnostic), "resolved contract search identity") {
		t.Errorf("interrupted input continued into the business call: %s", diagnostic)
	}
	if input != "stalled-inspection" {
		if _, err := os.Stat(pidFile); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("input interruption unexpectedly ran identity inspection: %v", err)
		}
		return
	}
	waitInterruptCondition(t, time.Second, func() bool { return errors.Is(syscall.Kill(helperPID, 0), syscall.ESRCH) }, "inspection helper was not reaped")
	waitInterruptCondition(t, time.Second, func() bool { return !interruptProcessRunning(descendantPID) }, "inspection subprocess group still has a running descendant")
	helperPID = 0
}

func assertInterruptedDiagnostic(t *testing.T, diagnostic []byte, mode string, business bool) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(string(diagnostic)), "\n")
	last := lines[len(lines)-1]
	if mode != "json" {
		if !strings.HasPrefix(last, "error:") || (!strings.Contains(last, "canceled") && !strings.Contains(last, "cancelled")) {
			t.Errorf("text interrupt must report a cancellation error: %s", diagnostic)
		}
		return
	}
	var envelope struct {
		OK    *bool `json:"ok"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			TraceID string `json:"trace_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(last), &envelope); err != nil {
		t.Errorf("JSON interrupt must end with a machine-readable error: %s", diagnostic)
		return
	}
	if envelope.OK == nil || *envelope.OK || envelope.Error.Type != "cancelled" || envelope.Error.Message == "" {
		t.Errorf("unexpected JSON cancellation envelope: %s", last)
	}
	if business && len(envelope.Error.TraceID) != 32 {
		t.Errorf("business cancellation lost command trace: %s", last)
	}
}

func waitInterruptCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

func interruptInputPipeOpened(pid int) bool {
	if runtime.GOOS == "linux" {
		fds := filepath.Join("/proc", strconv.Itoa(pid), "fd")
		stdin, err := os.Readlink(filepath.Join(fds, "0"))
		if err != nil {
			return false
		}
		entries, _ := os.ReadDir(fds)
		for _, entry := range entries {
			fd, _ := strconv.Atoi(entry.Name())
			if target, err := os.Readlink(filepath.Join(fds, entry.Name())); fd > 2 && err == nil && target == stdin {
				return true
			}
		}
		return false
	}
	output, err := exec.Command("/usr/sbin/lsof", "-a", "-p", strconv.Itoa(pid), "-Fnft").Output()
	if err != nil {
		return false
	}
	current, stdin := -1, ""
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "f") {
			current = -1
			if fd, err := strconv.Atoi(line[1:]); err == nil {
				current = fd
			}
		}
		if strings.HasPrefix(line, "n") && current == 0 {
			stdin = line[1:]
		}
		if strings.HasPrefix(line, "n") && current > 2 && stdin != "" && line[1:] == stdin {
			return true
		}
	}
	return false
}

func interruptProcessRunning(pid int) bool {
	output, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	state := strings.TrimSpace(string(output))
	return state != "" && !strings.HasPrefix(state, "Z")
}

func TestSecondInterruptRestoresDefaultTermination(t *testing.T) {
	if os.Getenv("QFEI_SECOND_INTERRUPT_HELPER") == "1" {
		ctx, stop := newInterruptContext(context.Background())
		defer stop()
		fmt.Fprintln(os.Stdout, "ready")
		<-ctx.Done()
		// Simulate cleanup work while the asynchronous signal reset completes.
		time.Sleep(50 * time.Millisecond)
		fmt.Fprintln(os.Stdout, "cancelled")
		for {
			time.Sleep(time.Second)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSecondInterruptRestoresDefaultTermination$")
	command.Env = append(os.Environ(), "QFEI_SECOND_INTERRUPT_HELPER=1")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	directory := t.TempDir()
	markerPath := filepath.Join(directory, "markers")
	markers, err := os.Create(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	defer markers.Close()
	command.Stdout, command.Stderr = markers, markers
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	exited := false
	defer func() {
		if !exited {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-done
		}
	}()
	for _, marker := range []string{"ready", "cancelled"} {
		waitInterruptCondition(t, 2*time.Second, func() bool {
			body, _ := os.ReadFile(markerPath)
			return strings.Contains(string(body), marker)
		}, "signal context did not report "+marker)
		if err := syscall.Kill(-command.Process.Pid, syscall.SIGINT); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-done:
		exited = true
		status := command.ProcessState.Sys().(syscall.WaitStatus)
		if err == nil || !status.Signaled() || status.Signal() != syscall.SIGINT {
			t.Fatalf("second interrupt should use default signal termination: %v, status=%v", err, command.ProcessState)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second interrupt did not force termination during cleanup")
	}
}

func verifyProcessFailureFormat(t *testing.T, ctx context.Context, binary string, flags []string, jsonOutput bool) {
	t.Helper()
	command := exec.CommandContext(ctx, binary, append([]string{"unsupported-command"}, flags...)...)
	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	command.Stdout, command.Stderr = stdout, stderr
	err := command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("ordinary failure exit = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("ordinary error wrote stdout: %s", stdout)
	}
	diagnostic := strings.TrimSpace(stderr.String())
	if !jsonOutput {
		if !strings.HasPrefix(diagnostic, "error:") || json.Valid([]byte(diagnostic)) {
			t.Errorf("error should remain plain text without explicit JSON output: %s", diagnostic)
		}
		return
	}
	var envelope struct {
		OK    *bool `json:"ok"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if strings.Contains(diagnostic, "\n") || json.Unmarshal([]byte(diagnostic), &envelope) != nil {
		t.Fatalf("explicit JSON output must produce one JSON error on stderr: %s", diagnostic)
	}
	if envelope.OK == nil || *envelope.OK || envelope.Error.Type == "" || envelope.Error.Message == "" {
		t.Errorf("unexpected JSON failure envelope: %s", diagnostic)
	}
}
