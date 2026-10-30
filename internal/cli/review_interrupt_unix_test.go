//go:build unix

package cli_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestReviewInterruptPartialInputCancelsWithoutClosingBorrowedPipe(t *testing.T) {
	for _, source := range []string{"json-file", "pre-audit-config"} {
		t.Run(source, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			fd := reader.Fd() // Match inherited blocking stdin in the real entrypoint.
			before, err := unix.FcntlInt(fd, unix.F_GETFL, 0)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			app := newReviewInterruptApp(t, reader, nil)
			args := []string{"review", "pre-audit", "start", "--run-id", strings.Repeat("a", 32), "--input-file", fmt.Sprintf("/dev/fd/%d", fd), "--dry-run"}
			if source == "json-file" {
				args = []string{"review", "task", "start", "--input-file", fmt.Sprintf("/dev/fd/%d", fd), "--dry-run"}
			}
			done := make(chan error, 1)
			go func() { done <- app.Run(ctx, args) }()
			if _, err := io.WriteString(writer, `{"partial":`); err != nil {
				t.Fatal(err)
			}
			assertReviewInputCancelled(t, cancel, done, func() { _ = writer.Close() })
			if _, err := reader.Stat(); err != nil {
				t.Fatalf("borrowed input was closed: %v", err)
			}
			after, err := unix.FcntlInt(fd, unix.F_GETFL, 0)
			if err != nil || before&unix.O_NONBLOCK != after&unix.O_NONBLOCK {
				t.Fatalf("borrowed input flags changed: before=%d after=%d err=%v", before, after, err)
			}
		})
	}
}

func TestReviewInterruptJSONFIFOCancelsBeforeWriterExists(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "review-input.fifo")
	if err := unix.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := newReviewInterruptApp(t, nil, nil)
	done := make(chan error, 1)
	go func() {
		done <- app.Run(ctx, []string{"review", "task", "start", "--input", fifo, "--dry-run"})
	}()
	assertReviewInputCancelled(t, cancel, done, func() {
		fd, err := unix.Open(fifo, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if err == nil {
			_ = unix.Close(fd)
		}
	})
}

func assertReviewInputCancelled(t *testing.T, cancel context.CancelFunc, done <-chan error, unblock func()) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("partial input returned before cancellation: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("input cancellation lost its error chain: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		unblock()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("failed input reader could not be cleaned up")
		}
		t.Error("partial input ignored cancellation")
	}
}
