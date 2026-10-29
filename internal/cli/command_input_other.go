//go:build !unix

package cli

import (
	"context"
	"os"
)

func openCommandInput(ctx context.Context, path string) (*os.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.Open(path)
}
