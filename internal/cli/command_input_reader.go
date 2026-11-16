package cli

import (
	"context"
	"fmt"
	"io"
	"os"
)

const defaultCommandInputLimit int64 = 32 << 20

func readDefaultCommandInput(ctx context.Context, path string) ([]byte, error) {
	body, err := readCommandInputLimited(ctx, path, defaultCommandInputLimit+1)
	return checkDefaultCommandInput(body, err)
}

func checkDefaultCommandInput(body []byte, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > defaultCommandInputLimit {
		return nil, fmt.Errorf("command input exceeds %d bytes; split the input", defaultCommandInputLimit)
	}
	return body, nil
}

func readCommandReader(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
	if file, ok := reader.(*os.File); ok {
		if limit <= 0 {
			body, err := readCommandInputFile(ctx, file, defaultCommandInputLimit+1)
			return checkDefaultCommandInput(body, err)
		}
		return readCommandInputFile(ctx, file, limit)
	}
	return readCommandReaderBytes(ctx, reader, limit)
}

// Injected readers retain ownership and must make their own Read cancellable if
// they can block. The CLI does not start a goroutine that could outlive Read.
func readCommandReaderBytes(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		body, err := io.ReadAll(commandContextReader{ctx: ctx, reader: io.LimitReader(reader, defaultCommandInputLimit+1)})
		return checkDefaultCommandInput(body, err)
	}
	if limit > 0 {
		reader = io.LimitReader(reader, limit)
	}
	return io.ReadAll(commandContextReader{ctx: ctx, reader: reader})
}

type commandContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader commandContextReader) Read(body []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	count, err := reader.reader.Read(body)
	if cancelled := reader.ctx.Err(); cancelled != nil {
		return count, cancelled
	}
	return count, err
}
