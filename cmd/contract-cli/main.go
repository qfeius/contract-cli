package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/invocation"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if handled, err := invocation.RunInspectionHelper(args, os.Stdout); handled {
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	ctx, stop := newInterruptContext(context.Background())
	defer stop()
	app := cli.New(cli.Options{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})

	return app.Execute(ctx, args)
}

func newInterruptContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt)
	// Restore the default SIGINT action after the first cancellation, so a
	// second Ctrl+C can force termination while cooperative cleanup is pending.
	reset := context.AfterFunc(ctx, stop)
	return ctx, func() { reset(); stop() }
}
