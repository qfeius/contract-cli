package openplatform

import (
	"context"
	"fmt"
	"io"
	"time"
)

func requestBudget(ctx context.Context, fallback time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, fallback)
}

func (c *Client) readResponseBody(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, c.maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > c.maxResponseBytes {
		return nil, fmt.Errorf("open platform response exceeds %d bytes; narrow the query or paginate", c.maxResponseBytes)
	}
	return body, nil
}
