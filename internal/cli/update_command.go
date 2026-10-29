package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/build"
	"cn.qfei/contract-cli/internal/output"
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

func (a *App) runUpdateCheck(ctx context.Context, args []string) (err error) {
	var outcome *localCommandOutcome
	defer func() { err = withCompletedLocalCommand(ctx, err, outcome) }()
	parsed, err := parseArgs(args, map[string]struct{}{
		"--channel": {},
		"--output":  {},
	}, map[string]struct{}{
		"--json": {},
	})
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("unexpected update check arguments: %s", strings.Join(parsed.positionals, " "))
	}
	jsonOutput, err := parseJSONOutputFormat(parsed.String("--output"))
	if err != nil {
		return err
	}

	result, err := a.checkUpdate(ctx, parsed.String("--channel"))
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !result.Skipped {
		if err := updatecheck.SaveCache(a.updateCachePath(), updatecheck.CacheFromResult(result)); err != nil {
			a.logger.Warn("save update check cache failed", "path", a.updateCachePath(), "error", err.Error())
		} else {
			outcome = &localCommandOutcome{Operation: "update check", Result: map[string]any{"cache_saved": true, "current_version": result.CurrentVersion, "latest_version": result.LatestVersion}}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if jsonOutput || parsed.Bool("--json") {
		return output.NewRenderer(commandResultWriter{ctx: ctx, writer: a.stdout}).Render(output.FormatJSON, updateCheckJSON(result))
	}
	return writeUpdateCheckText(commandResultWriter{ctx: ctx, writer: a.stdout}, result)
}

func updateCheckJSON(result updatecheck.Result) map[string]any {
	action := "already_up_to_date"
	message := fmt.Sprintf("contract-cli %s is already up to date", result.CurrentVersion)
	if result.UpdateAvailable {
		action = "update_available"
		message = fmt.Sprintf("contract-cli %s -> %s available", result.CurrentVersion, result.LatestVersion)
	}
	if result.Skipped {
		action = "skipped"
		message = result.Reason
	}

	data := map[string]any{
		"ok":               true,
		"package":          result.PackageName,
		"previous_version": result.CurrentVersion,
		"current_version":  result.CurrentVersion,
		"latest_version":   result.LatestVersion,
		"channel":          result.Channel,
		"action":           action,
		"message":          message,
	}
	if result.UpdateAvailable {
		data["auto_update"] = false
		data["command"] = result.InstallCommand
	}
	if result.Skipped {
		data["reason"] = result.Reason
	}
	return data
}

func writeUpdateCheckText(w anyWriter, result updatecheck.Result) error {
	if result.Skipped {
		_, err := fmt.Fprintf(w, "Update check skipped: %s\n", result.Reason)
		return err
	}
	if result.UpdateAvailable {
		if _, err := fmt.Fprintf(w, "Update available: contract-cli %s -> %s\n", result.CurrentVersion, result.LatestVersion); err != nil {
			return err
		}
		_, err := fmt.Fprintf(w, "Run: %s\n", result.InstallCommand)
		return err
	}
	_, err := fmt.Fprintf(w, "contract-cli %s is already up to date\n", result.CurrentVersion)
	return err
}

type anyWriter interface {
	Write([]byte) (int, error)
}

func (a *App) maybePrepareUpdateNotice(ctx context.Context, args []string) (err error) {
	var outcome *localCommandOutcome
	defer func() { err = withCompletedLocalCommand(ctx, err, outcome) }()
	if !a.shouldAutoCheckUpdate(args) {
		return nil
	}

	currentVersion := a.currentUpdateVersion()
	channel := updatecheck.InferChannel(currentVersion)
	now := a.now()
	cache, cacheOK, err := updatecheck.LoadCache(a.updateCachePath())
	if err != nil {
		a.logger.Debug("load update check cache failed", "path", a.updateCachePath(), "error", err.Error())
	}
	if cacheOK && strings.TrimSpace(cache.Channel) == channel {
		if updatecheck.CacheFresh(cache, channel, now, updatecheck.CacheTTL) {
			if notice := updatecheck.NoticeFromCache(cache, currentVersion, updatecheck.DefaultPackageName); notice != nil {
				a.updateNotice = map[string]any{"update": notice.Map()}
			}
			return nil
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	result, err := a.checkUpdateWithLogger(checkCtx, "", nil)
	if err != nil {
		a.logger.Debug("automatic update check failed", "error", err.Error())
		if ctx.Err() != nil {
			return withCommandCancellation(ctx, err)
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if result.Skipped {
		return nil
	}
	if err := updatecheck.SaveCache(a.updateCachePath(), updatecheck.CacheFromResult(result)); err != nil {
		a.logger.Warn("save update check cache failed", "path", a.updateCachePath(), "error", err.Error())
	} else {
		outcome = &localCommandOutcome{Operation: "automatic update check", Result: map[string]any{"cache_saved": true, "current_version": result.CurrentVersion, "latest_version": result.LatestVersion}}
	}
	a.updateNotice = nil
	if notice := updatecheck.NoticeFromResult(result); notice != nil {
		a.updateNotice = map[string]any{"update": notice.Map()}
	}
	return ctx.Err()
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
	if isCIUpdateEnv(a.lookupEnv) {
		return false
	}
	return true
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

func isCIUpdateEnv(lookup func(string) (string, bool)) bool {
	for _, key := range []string{"CI", "BUILD_NUMBER", "RUN_ID"} {
		if value, ok := lookup(key); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
