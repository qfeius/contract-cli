package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/build"
	updatecheck "cn.qfei/contract-cli/internal/update"
)

func (a *App) runUpdate(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("missing update subcommand")
	}

	switch args[0] {
	case "check":
		return a.runUpdateCheck(ctx, args[1:])
	default:
		return fmt.Errorf("unknown update subcommand %q", args[0])
	}
}

func (a *App) runUpdateCheck(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, map[string]struct{}{
		"--channel": {},
	}, nil)
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("unexpected update check arguments: %s", strings.Join(parsed.positionals, " "))
	}

	result, err := a.checkUpdate(ctx, parsed.String("--channel"))
	if err != nil {
		return err
	}
	if result.Skipped {
		_, _ = fmt.Fprintf(a.stdout, "Update check skipped: %s\n", result.Reason)
		return nil
	}
	if err := updatecheck.SaveCache(a.updateCachePath(), updatecheck.CacheFromResult(result)); err != nil {
		a.logger.Warn("save update check cache failed", "path", a.updateCachePath(), "error", err.Error())
	}

	if result.UpdateAvailable {
		_, _ = fmt.Fprintf(a.stdout, "A new contract-cli version is available: %s -> %s\n", result.CurrentVersion, result.LatestVersion)
		_, _ = fmt.Fprintf(a.stdout, "Run: %s\n", result.InstallCommand)
		return nil
	}

	_, _ = fmt.Fprintf(a.stdout, "contract-cli is up to date: %s (channel %s)\n", result.CurrentVersion, result.Channel)
	return nil
}

func (a *App) maybePrintUpdateNotice(ctx context.Context, args []string) {
	if !a.shouldAutoCheckUpdate(args) {
		return
	}

	currentVersion := a.currentUpdateVersion()
	channel := updatecheck.InferChannel(currentVersion)
	cache, ok, err := updatecheck.LoadCache(a.updateCachePath())
	if err != nil {
		a.logger.Warn("load update check cache failed", "path", a.updateCachePath(), "error", err.Error())
	}
	if err == nil && ok && updatecheck.CacheFresh(cache, a.now(), a.updateInterval, currentVersion, channel) {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	result, err := a.checkUpdateWithLogger(checkCtx, "", nil)
	if err != nil {
		a.logger.Debug("automatic update check failed", "error", err.Error())
		if err := updatecheck.SaveCache(a.updateCachePath(), updatecheck.Cache{
			CheckedAt:      a.now(),
			Channel:        channel,
			CurrentVersion: currentVersion,
		}); err != nil {
			a.logger.Warn("save update check failure cache failed", "path", a.updateCachePath(), "error", err.Error())
		}
		return
	}
	if result.Skipped {
		return
	}
	if err := updatecheck.SaveCache(a.updateCachePath(), updatecheck.CacheFromResult(result)); err != nil {
		a.logger.Warn("save update check cache failed", "path", a.updateCachePath(), "error", err.Error())
	}
	if !result.UpdateAvailable {
		return
	}

	_, _ = fmt.Fprintf(a.stderr, "\nA new contract-cli version is available: %s -> %s\n", result.CurrentVersion, result.LatestVersion)
	_, _ = fmt.Fprintf(a.stderr, "Run: %s\n\n", result.InstallCommand)
}

func (a *App) shouldAutoCheckUpdate(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if isHelpRequest(args) {
		return false
	}
	switch args[0] {
	case "version", "--version", "-version", "-v", "update":
		return false
	}
	if value, ok := a.lookupEnv("CONTRACT_CLI_NO_UPDATE_CHECK"); ok && truthy(value) {
		return false
	}
	return a.isTerminal(a.stderr)
}

func (a *App) checkUpdate(ctx context.Context, channel string) (updatecheck.Result, error) {
	return a.checkUpdateWithLogger(ctx, channel, a.logger)
}

func (a *App) checkUpdateWithLogger(ctx context.Context, channel string, logger *slog.Logger) (updatecheck.Result, error) {
	return updatecheck.Check(ctx, updatecheck.Options{
		HTTPClient:     a.httpClient,
		Logger:         logger,
		RegistryURL:    a.updateURL,
		PackageName:    updatecheck.DefaultPackageName,
		CurrentVersion: a.currentUpdateVersion(),
		Channel:        channel,
		Now:            a.now,
	})
}

func (a *App) currentUpdateVersion() string {
	if strings.TrimSpace(a.updateVersion) != "" {
		return strings.TrimSpace(a.updateVersion)
	}
	return build.Current().Version
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
