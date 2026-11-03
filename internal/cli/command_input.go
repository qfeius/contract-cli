//go:build !unix

package cli

import (
	"context"
	"os"
)

func readCommandInput(ctx context.Context, path string) ([]byte, error) {
	return readCommandInputLimited(ctx, path, 0)
}

func readCommandInputLimited(ctx context.Context, path string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := openCommandInput(ctx, path)
	if err != nil {
		return nil, err
	}
	return readOwnedCommandInputFile(ctx, file, limit)
}

func readCommandInputFile(ctx context.Context, file *os.File, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owned, err := duplicateCommandInputFile(file)
	if err != nil {
		return nil, err
	}
	return readOwnedCommandInputFile(ctx, owned, limit)
}

func readOwnedCommandInputFile(ctx context.Context, file *os.File, limit int64) ([]byte, error) {
	closed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = file.Close(); close(closed) })
	defer func() {
		if !stop() {
			<-closed
		}
		_ = file.Close()
	}()
	body, err := readCommandReaderBytes(ctx, file, limit)
	if cancelled := ctx.Err(); cancelled != nil {
		return nil, cancelled
	}
	return body, err
}
