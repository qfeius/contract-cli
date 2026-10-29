package invocation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

const (
	// One budget for process discovery AND all identity checks, per HTTP attempt.
	DefaultInspectionTimeout = 5 * time.Second
	inspectionHelperFlag     = "--internal-invocation-inspect"
	maxInspectionOutput      = 1 << 20
	inspectionWaitDelay      = 100 * time.Millisecond
)

// Inspect runs the existing detector in a disposable copy of this executable.
// Native identity checks (notably WinVerifyTrust) cannot be cancelled by a Go
// context. Process isolation bounds them without leaking blocked goroutines.
// Any inspection failure is optional metadata loss, never a business error.
func Inspect(parent context.Context, maxDepth int) Result {
	ctx, cancel := context.WithTimeout(parent, DefaultInspectionTimeout)
	defer cancel()
	if ctx.Err() != nil {
		return unavailableInspection("environment inspection cancelled")
	}
	executable, err := os.Executable()
	if err != nil {
		return unavailableInspection("environment inspection helper unavailable")
	}
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	if maxDepth > 128 {
		maxDepth = 128
	}
	command := exec.CommandContext(ctx, executable, inspectionHelperFlag, strconv.Itoa(maxDepth))
	// The entrypoint owns SIGINT and error output. CommandContext cancels the
	// private process group; Run/Wait reaps it before callers inspect ctx.Err().
	return inspectCommand(ctx, command)
}

func inspectCommand(ctx context.Context, command *exec.Cmd) Result {
	var output inspectionOutput
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = inspectionWaitDelay
	configureInspectionCommand(command)
	err := command.Run() // Wait reaps the helper before any business HTTP call continues.
	// The helper sends a complete preliminary report before signature work.
	// Wait has joined the pipe copier, so reading the buffer here is race-free.
	var result Result
	haveResult := false
	decoder := json.NewDecoder(bytes.NewReader(output.buffer.Bytes()))
	for !output.overflow {
		var next Result
		if decodeErr := decoder.Decode(&next); decodeErr != nil {
			if decodeErr != io.EOF && ctx.Err() == nil && err == nil {
				return unavailableInspection("environment inspection helper returned invalid output")
			}
			break
		}
		if next.ChannelType != "cli" || next.ProductCode != ProductCodeContract ||
			next.DetectorVersion != DetectorVersion || next.AgentSourceType == "" {
			return unavailableInspection("environment inspection helper returned invalid output")
		}
		result, haveResult = next, true
	}
	if output.overflow {
		return unavailableInspection("environment inspection output exceeded limit")
	}
	if !haveResult {
		return unavailableInspection("environment inspection helper failed or timed out before reporting")
	}
	if ctx.Err() != nil || err != nil {
		result.Warnings = append(result.Warnings, "identity enrichment failed or timed out; retained preliminary attribution")
	}

	return result
}

func unavailableInspection(reason string) Result {
	result := Analyze(nil, nil)
	result.Platform = runtime.GOOS
	result.Reason = reason
	return result
}

// RunInspectionHelper must be dispatched by main BEFORE CLI/auth/update setup.
// It only reads local process identity and writes preliminary and enriched JSON reports; it never
// opens a profile, reads tokens, starts another helper or sends a business call.
func RunInspectionHelper(args []string, output io.Writer) (bool, error) {
	if len(args) == 0 || args[0] != inspectionHelperFlag {
		return false, nil
	}
	if len(args) != 2 {
		return true, fmt.Errorf("invalid inspection helper arguments")
	}
	maxDepth, err := strconv.Atoi(args[1])
	if err != nil || maxDepth <= 0 || maxDepth > 128 {
		return true, fmt.Errorf("invalid inspection helper depth")
	}
	encoder := json.NewEncoder(output)
	var writeErr error
	result := inspectProcessWithProgress(int32(os.Getppid()), maxDepth, func(preliminary Result) {
		writeErr = encoder.Encode(preliminary)
	})
	if writeErr != nil {
		return true, writeErr
	}
	return true, encoder.Encode(result)
}

// Bound the helper's output without blocking its pipes if output is oversized.
type inspectionOutput struct {
	buffer   bytes.Buffer
	overflow bool
}

func (output *inspectionOutput) Write(value []byte) (int, error) {
	size := len(value)
	remaining := maxInspectionOutput - output.buffer.Len()
	if size > remaining {
		output.overflow = true
		value = value[:remaining]
	}
	_, _ = output.buffer.Write(value)
	return size, nil
}
