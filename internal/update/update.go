package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultPackageName   = "@qfeius/contract-cli"
	DefaultRegistryURL   = "https://registry.npmjs.org/@qfeius%2fcontract-cli"
	DefaultRegistryHost  = "https://registry.npmjs.org"
	DefaultCheckInterval = 30 * time.Minute
)

type Options struct {
	HTTPClient     *http.Client
	Logger         *slog.Logger
	RegistryURL    string
	PackageName    string
	CurrentVersion string
	Channel        string
	Now            func() time.Time
}

type Result struct {
	CheckedAt       time.Time
	PackageName     string
	CurrentVersion  string
	LatestVersion   string
	Channel         string
	UpdateAvailable bool
	InstallCommand  string
	Skipped         bool
	Reason          string
}

type Cache struct {
	CheckedAt       time.Time `json:"checked_at"`
	Channel         string    `json:"channel"`
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	InstallCommand  string    `json:"install_command,omitempty"`
}

func Check(ctx context.Context, options Options) (Result, error) {
	now := optionNow(options.Now)
	packageName := defaultString(options.PackageName, DefaultPackageName)
	currentVersion := strings.TrimSpace(options.CurrentVersion)
	channel := defaultString(options.Channel, InferChannel(currentVersion))
	result := Result{
		CheckedAt:      now,
		PackageName:    packageName,
		CurrentVersion: currentVersion,
		Channel:        channel,
	}

	logger := options.Logger
	if logger != nil {
		logger.Info("update check started", "package", packageName, "current_version", emptyFallback(currentVersion, "<empty>"), "channel", channel)
	}

	if shouldSkipVersion(currentVersion) {
		result.Skipped = true
		result.Reason = "current version is dev/unknown; skip remote update check"
		if logger != nil {
			logger.Info("update check skipped", "reason", result.Reason)
		}
		return result, nil
	}
	if _, err := parseSemver(currentVersion); err != nil {
		result.Skipped = true
		result.Reason = "current version is not semantic version; skip remote update check"
		if logger != nil {
			logger.Info("update check skipped", "reason", result.Reason, "current_version", currentVersion)
		}
		return result, nil
	}

	registryURL := defaultString(options.RegistryURL, DefaultRegistryURL)
	client := options.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL, nil)
	if err != nil {
		if logger != nil {
			logger.Error("build update check request failed", "registry_url", registryURL, "error", err.Error())
		}
		return Result{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Error("send update check request failed", "registry_url", registryURL, "error", err.Error())
		}
		return Result{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		if logger != nil {
			logger.Error("read update check response failed", "status_code", resp.StatusCode, "error", err.Error())
		}
		return Result{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("npm registry returned status %d: %s", resp.StatusCode, responseSummary(body))
		if logger != nil {
			logger.Error("update check registry status failed", "status_code", resp.StatusCode, "error", err.Error())
		}
		return Result{}, err
	}

	var packument struct {
		DistTags map[string]string `json:"dist-tags"`
	}
	if err := json.Unmarshal(body, &packument); err != nil {
		if logger != nil {
			logger.Error("decode update check response failed", "error", err.Error())
		}
		return Result{}, err
	}
	latestVersion := strings.TrimSpace(packument.DistTags[channel])
	if latestVersion == "" {
		err := fmt.Errorf("npm dist-tag %q not found for %s", channel, packageName)
		if logger != nil {
			logger.Error("update check dist-tag missing", "package", packageName, "channel", channel, "error", err.Error())
		}
		return Result{}, err
	}

	compare, err := CompareSemver(currentVersion, latestVersion)
	if err != nil {
		if logger != nil {
			logger.Error("compare update versions failed", "current_version", currentVersion, "latest_version", latestVersion, "error", err.Error())
		}
		return Result{}, err
	}

	result.LatestVersion = latestVersion
	result.UpdateAvailable = compare < 0
	result.InstallCommand = fmt.Sprintf("npm install -g %s@%s --registry %s", packageName, channel, DefaultRegistryHost)
	if logger != nil {
		logger.Info("update check completed", "package", packageName, "current_version", currentVersion, "latest_version", latestVersion, "channel", channel, "update_available", result.UpdateAvailable)
	}
	return result, nil
}

func InferChannel(version string) string {
	normalized := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if strings.Contains(normalized, "-") {
		return "beta"
	}
	return "latest"
}

func CompareSemver(a, b string) (int, error) {
	left, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	right, err := parseSemver(b)
	if err != nil {
		return 0, err
	}

	for i := range left.core {
		if left.core[i] < right.core[i] {
			return -1, nil
		}
		if left.core[i] > right.core[i] {
			return 1, nil
		}
	}
	return comparePrerelease(left.prerelease, right.prerelease), nil
}

func CacheFresh(cache Cache, now time.Time, interval time.Duration, currentVersion, channel string) bool {
	if interval <= 0 {
		return false
	}
	if cache.CurrentVersion != currentVersion || cache.Channel != channel {
		return false
	}
	if cache.CheckedAt.IsZero() {
		return false
	}
	elapsed := now.Sub(cache.CheckedAt)
	return elapsed >= 0 && elapsed < interval
}

func LoadCache(path string) (Cache, bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Cache{}, false, nil
	}
	if err != nil {
		return Cache{}, false, err
	}

	var cache Cache
	if err := json.Unmarshal(content, &cache); err != nil {
		return Cache{}, false, err
	}
	return cache, true, nil
}

func SaveCache(path string, cache Cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	content, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(content, '\n'), 0o600)
}

func CacheFromResult(result Result) Cache {
	return Cache{
		CheckedAt:       result.CheckedAt,
		Channel:         result.Channel,
		CurrentVersion:  result.CurrentVersion,
		LatestVersion:   result.LatestVersion,
		UpdateAvailable: result.UpdateAvailable,
		InstallCommand:  result.InstallCommand,
	}
}

type semver struct {
	core       [3]int
	prerelease []string
}

func parseSemver(version string) (semver, error) {
	normalized := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if normalized == "" {
		return semver{}, errors.New("empty semantic version")
	}
	if before, _, ok := strings.Cut(normalized, "+"); ok {
		normalized = before
	}

	corePart := normalized
	var prerelease []string
	if before, after, ok := strings.Cut(normalized, "-"); ok {
		corePart = before
		prerelease = strings.Split(after, ".")
	}

	parts := strings.Split(corePart, ".")
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("invalid semantic version %q", version)
	}

	var parsed semver
	parsed.prerelease = prerelease
	for i, part := range parts {
		if part == "" {
			return semver{}, fmt.Errorf("invalid semantic version %q", version)
		}
		number, err := strconv.Atoi(part)
		if err != nil {
			return semver{}, fmt.Errorf("invalid semantic version %q: %w", version, err)
		}
		parsed.core[i] = number
	}
	return parsed, nil
}

func comparePrerelease(a, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}

	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		result := comparePrereleaseIdentifier(a[i], b[i])
		if result != 0 {
			return result
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func comparePrereleaseIdentifier(a, b string) int {
	aNumber, aErr := strconv.Atoi(a)
	bNumber, bErr := strconv.Atoi(b)
	aIsNumber := aErr == nil
	bIsNumber := bErr == nil

	switch {
	case aIsNumber && bIsNumber:
		if aNumber < bNumber {
			return -1
		}
		if aNumber > bNumber {
			return 1
		}
		return 0
	case aIsNumber:
		return -1
	case bIsNumber:
		return 1
	default:
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
}

func shouldSkipVersion(version string) bool {
	switch strings.ToLower(strings.TrimSpace(version)) {
	case "", "dev", "unknown":
		return true
	default:
		return false
	}
}

func optionNow(now func() time.Time) time.Time {
	if now != nil {
		return now()
	}
	return time.Now()
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func responseSummary(body []byte) string {
	const limit = 512
	summary := strings.TrimSpace(string(body))
	if len(summary) > limit {
		return summary[:limit] + "..."
	}
	return summary
}
