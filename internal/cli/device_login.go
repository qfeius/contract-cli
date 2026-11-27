package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
)

const maxDeviceLoginWait = 5 * time.Minute

// This explicit host-driven flow leaves auth init/complete's single-call API unchanged.
func (a *App) runAuthDeviceLogin(ctx context.Context, args []string) (err error) {
	a.logger.Info("device login started")
	defer func() {
		if err != nil {
			a.logger.Warn("device login stopped without success")
		}
	}()
	flags := flag.NewFlagSet("auth device-login", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	profile := flags.String("profile", "", "profile name")
	timeout := flags.Duration("timeout", maxDeviceLoginWait, "maximum wait, up to 5m")
	restart := flags.Bool("restart", false, "replace pending authorization after explicit user confirmation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *timeout <= 0 || *timeout > maxDeviceLoginWait {
		return errors.New("auth device-login requires a timeout greater than zero and at most 5m, and no positional arguments")
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	return a.deviceLogin(ctx, *profile, *restart)
}

func (a *App) deviceLogin(ctx context.Context, profile string, restart bool) error {
	selected, err := a.loadDeviceAwareProfile(profile)
	if err != nil {
		return err
	}
	profile = selected.Name
	var started deviceAuthOutput
	startCtx, cancel := context.WithTimeout(ctx, 9*time.Second)
	err = a.startDeviceAuthorization(startCtx, profile, restart, func(output deviceAuthOutput) error {
		started = output
		return nil
	})
	cancel()
	if err != nil {
		return err
	}
	if started.Status == "authorized" {
		return a.writeDeviceLoginSuccess()
	}
	if started.Status != "pending" {
		return deviceLoginStateError(started.Status)
	}
	return a.waitForDeviceLogin(ctx, profile, started)
}

func (a *App) waitForDeviceLogin(ctx context.Context, profile string, started deviceAuthOutput) error {
	expires, err := time.Parse(time.RFC3339, started.ExpiresAt)
	if err != nil || !a.now().Before(expires) || started.VerificationURIComplete == "" {
		return errors.New("device authorization link or expiry is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, expires.Sub(a.now()))
	defer cancel()
	if _, err := fmt.Fprintf(a.stdout, "Authorization URL:\n%s\nExpires at: %s\n", started.VerificationURIComplete, started.ExpiresAtDisplay); err != nil {
		return err
	}
	a.logger.Info("device login waiting for user authorization", "profile", profile)
	err = pollDeviceLogin(ctx, started.pollInterval, waitDeviceLoginInterval, func(ctx context.Context) (deviceAuthOutput, error) {
		var checked deviceAuthOutput
		err := a.completeDeviceAuthorization(ctx, profile, started.VerificationURIComplete, func(output deviceAuthOutput) error {
			checked = output
			return nil
		})
		return checked, err
	})
	if err != nil {
		return err
	}
	return a.writeDeviceLoginSuccess()
}

func (a *App) writeDeviceLoginSuccess() error {
	a.logger.Info("device login authorized")
	_, err := fmt.Fprintln(a.stdout, "Authorization: authorized")
	return err
}

func pollDeviceLogin(ctx context.Context, interval time.Duration, wait func(context.Context, time.Duration) error, complete func(context.Context) (deviceAuthOutput, error)) error {
	for {
		if err := wait(ctx, interval); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		checked, err := complete(ctx)
		if err != nil || ctx.Err() != nil {
			return errors.Join(err, ctx.Err())
		}
		if checked.Status == "succeeded" {
			return nil
		}
		if checked.Status != "pending" {
			return deviceLoginStateError(checked.Status)
		}
		if checked.slowDown {
			interval = min(interval+5*time.Second, maxDeviceLoginWait)
		}
	}
}

func deviceLoginStateError(status string) error {
	return fmt.Errorf("device authorization stopped (%s); no automatic retry or restart; check auth status and confirm with the user before restarting", status)
}

func deviceLoginInterval(seconds int64) time.Duration {
	if seconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(min(seconds, int64(maxDeviceLoginWait/time.Second))) * time.Second
}

func waitDeviceLoginInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
