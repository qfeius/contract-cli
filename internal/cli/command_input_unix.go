//go:build unix

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func readCommandInput(ctx context.Context, path string) ([]byte, error) {
	return readCommandInputLimited(ctx, path, 0)
}

func readCommandInputLimited(ctx context.Context, path string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fd, wasNonblocking, err := openCommandInputDescriptor(path)
	if err != nil {
		return nil, err
	}
	return readOwnedCommandInputDescriptor(ctx, fd, wasNonblocking, path, limit)
}

func readCommandInputFile(ctx context.Context, file *os.File, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fd, wasNonblocking, err := duplicateCommandInputDescriptor(file)
	if err != nil {
		return nil, err
	}
	return readOwnedCommandInputDescriptor(ctx, fd, wasNonblocking, file.Name(), limit)
}

func duplicateCommandInputDescriptor(file *os.File) (int, bool, error) {
	connection, err := file.SyscallConn()
	if err != nil {
		return -1, false, err
	}
	fd := -1
	var duplicateErr error
	if err := connection.Control(func(original uintptr) {
		fd, duplicateErr = unix.FcntlInt(original, unix.F_DUPFD_CLOEXEC, 0)
	}); err != nil {
		return -1, false, err
	}
	if duplicateErr != nil {
		return -1, false, &os.PathError{Op: "duplicate", Path: file.Name(), Err: duplicateErr}
	}
	return prepareCommandInputDescriptor(fd, file.Name())
}

func readOwnedCommandInputDescriptor(ctx context.Context, fd int, wasNonblocking bool, path string, limit int64) (body []byte, err error) {
	defer func() { err = errors.Join(err, closeCommandInputDescriptor(fd, wasNonblocking)) }()
	body, err = readCommandInputDescriptor(ctx, fd, limit)
	if err != nil {
		return nil, &os.PathError{Op: "read", Path: path, Err: err}
	}
	return body, nil
}

func openCommandInputDescriptor(path string) (int, bool, error) {
	// Nonblocking Open also covers a FIFO before a writer has connected.
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return -1, false, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return prepareCommandInputDescriptor(fd, path)
}

func prepareCommandInputDescriptor(fd int, path string) (int, bool, error) {
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err == nil {
		err = unix.SetNonblock(fd, true)
	}
	if err != nil {
		return -1, false, &os.PathError{Op: "set nonblocking", Path: path, Err: errors.Join(err, unix.Close(fd))}
	}
	return fd, flags&unix.O_NONBLOCK != 0, nil
}

func closeCommandInputDescriptor(fd int, wasNonblocking bool) error {
	var restoreErr error
	if !wasNonblocking {
		// Darwin /dev/fd duplicates share status flags with the original stdin.
		restoreErr = unix.SetNonblock(fd, false)
	}
	return errors.Join(restoreErr, unix.Close(fd))
}

func readCommandInputDescriptor(ctx context.Context, fd int, limit int64) ([]byte, error) {
	var body []byte
	buffer := make([]byte, 32*1024)
	for {
		if err := waitCommandInputReady(ctx, fd); err != nil {
			return nil, err
		}
		readSize := len(buffer)
		if limit > 0 {
			readSize = int(min(int64(readSize), limit-int64(len(body))))
		}
		count, err := unix.Read(fd, buffer[:readSize])
		if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
			continue
		}
		if err != nil {
			return nil, err
		}
		body = append(body, buffer[:count]...)
		if count == 0 || (limit > 0 && int64(len(body)) >= limit) {
			return body, ctx.Err()
		}
	}
}

func waitCommandInputReady(ctx context.Context, fd int) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ready, err := pollCommandInputDescriptor(fd)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("wait for command input: %w", err)
		}
		if ready {
			return nil
		}
	}
}
