package cli

import (
	"context"
	"io"
	"os"
)

func readCommandReader(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
	if file, ok := reader.(*os.File); ok {
		return readCommandInputFile(ctx, file, limit)
	}
	return readCommandReaderBytes(ctx, reader, limit)
}

// Injected readers retain ownership and must make their own Read cancellable if
// they can block. The CLI does not start a goroutine that could outlive Read.
func readCommandReaderBytes(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
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
