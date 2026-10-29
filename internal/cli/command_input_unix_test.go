//go:build unix

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestCommandInputNamedPipeCanCancelBeforeWriterExists(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "input.fifo")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := readCommandInput(ctx, path); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("FIFO returned before a writer or cancellation: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel error = %v", err)
		}
	case <-time.After(time.Second):
		// Unblock a broken implementation so the test does not leak its goroutine.
		fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if err == nil {
			_ = unix.Close(fd)
		}
		t.Fatal("FIFO open swallowed cancellation")
	}
}

func TestCommandInputPreservesFileBytesAndCancellation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "input.json")
	const body = "{\"amount\":12345678901234567890}\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := readCommandInput(context.Background(), path)
	if err != nil || string(data) != body {
		t.Fatalf("file changed: %q %v", data, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readCommandInput(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled input = %v", err)
	}
}

func TestCommandInputNamedPipeWaitsForAndReadsWriter(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "input.fifo")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	writer := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			writer <- ctx.Err()
			return
		case <-time.After(50 * time.Millisecond):
		}
		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err == nil {
			_, err = io.WriteString(file, `{"rows":[]}`)
			_ = file.Close()
		}
		writer <- err
	}()
	body, err := readCommandInput(ctx, path)
	if err != nil || string(body) != `{"rows":[]}` {
		t.Fatalf("FIFO bytes changed: %q %v", body, err)
	}
	if err := <-writer; err != nil {
		t.Fatal(err)
	}
}

func TestCommandInputCancelsConnectedPipeWithoutWaitingForEOF(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "input.fifo")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	defer close(release)
	wrote := make(chan error, 1)
	go func() {
		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			wrote <- err
			return
		}
		defer file.Close()
		_, err = io.WriteString(file, `{"partial":`)
		wrote <- err
		<-release
	}()
	done := make(chan error, 1)
	go func() { _, err := readCommandInput(ctx, path); done <- err }()
	if err := <-wrote; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		t.Fatalf("input returned before writer EOF or cancellation: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("connected pipe cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("connected read did not wake on cancellation")
	}
}

func TestCommandInputInheritedPipeCancelsAfterPartialBytes(t *testing.T) {
	t.Parallel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	// Fd exposes a blocking descriptor, as inherited terminal/stdin pipes are.
	fd := reader.Fd()
	path := fmt.Sprintf("/dev/fd/%d", fd)
	before, err := unix.FcntlInt(fd, unix.F_GETFL, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := readCommandInput(ctx, path); done <- err }()
	if _, err := io.WriteString(writer, `{"partial":`); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		t.Fatalf("inherited pipe returned before EOF/cancel: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("inherited pipe cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("partial inherited stdin read swallowed cancellation")
	}
	after, err := unix.FcntlInt(fd, unix.F_GETFL, 0)
	if err != nil || before&unix.O_NONBLOCK != after&unix.O_NONBLOCK {
		t.Fatalf("inherited stdin flags changed: before=%x after=%x error=%v", before, after, err)
	}
}

func TestCommandInputNamedPipeReturnsEOFWhenIdleWriterCloses(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "input.fifo")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		data []byte
		err  error
	}
	done := make(chan outcome, 1)
	go func() { data, err := readCommandInput(ctx, path); done <- outcome{data, err} }()
	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	const body = `{"complete":true}`
	if _, err := io.WriteString(writer, body); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		t.Fatalf("read returned before writer closed: %q %v", result.data, result.err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil || string(result.data) != body {
			t.Fatalf("FIFO normal EOF changed data: %q %v", result.data, result.err)
		}
	case <-time.After(500 * time.Millisecond):
		cancel()
		t.Fatal("FIFO did not observe normal EOF after last writer closed")
	}
}

func TestCommandInputNamedPipeReturnsEmptyWriterEOF(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "empty.fifo")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		body, err := readCommandInput(ctx, path)
		if len(body) != 0 {
			err = fmt.Errorf("unexpected empty writer content: %q", body)
		}
		done <- err
	}()
	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("empty FIFO writer EOF: %v", err)
	}
}
