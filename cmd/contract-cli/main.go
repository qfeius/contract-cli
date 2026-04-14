package main

import (
	"context"
	"log/slog"
	"os"

	"cn.qfei/contract-cli/internal/cli"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))
	app := cli.New(cli.Options{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Logger: logger,
	})

	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		logger.Error("command failed", "error", err.Error())
		os.Exit(1)
	}
}
