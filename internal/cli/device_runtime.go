package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/invocation"
)

// Unlike os.UserHomeDir/UserCacheDir, native account lookup on our supported
// desktop targets is independent of an agent's temporary HOME/LOCALAPPDATA.
func currentOSUserHome() (string, error) {
	account, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolve operating system user: %w", err)
	}
	if account.HomeDir == "" || !filepath.IsAbs(account.HomeDir) {
		return "", errors.New("operating system user home must be absolute")
	}
	return account.HomeDir, nil
}

func (a *App) localDeviceDirectory() (string, error) {
	home, err := a.localUserHome()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(home) {
		return "", errors.New("local Device home must be absolute")
	}
	return filepath.Join(home, ".contract-cli", "device-user"), nil
}

// Runtime selection is shared by storage, locks and artifacts. Source attribution
// remains diagnostic; this decision never grants access or replaces OAuth.
func (a *App) resolveDeviceRuntime() (credential.DeviceRuntime, error) {
	if workspace, ok := a.lookupEnv("SKILL_SESSION_WORKSPACE"); ok && strings.TrimSpace(workspace) != "" {
		return credential.ResolveDeviceRuntime(a.lookupEnv)
	}
	if a.runtimeEvidence == nil {
		ctx := a.commandContext
		if ctx == nil {
			ctx = context.Background()
		}
		a.logger.Info("inspect Device credential runtime started")
		report := a.inspectEnvironment(ctx, invocation.DefaultMaxDepth)
		if err := ctx.Err(); err != nil {
			return credential.DeviceRuntime{}, err
		}
		a.runtimeEvidence = &report
		a.logger.Info("Device credential runtime inspected", "source", report.AgentSourceType, "confidence", report.Confidence, "rule_id", report.RuleID)
	}
	resolved, err := credential.ResolveDeviceRuntimeWithEvidence(a.lookupEnv, *a.runtimeEvidence)
	if err == nil && resolved.Kind == credential.DeviceRuntimeDoubaoWorkTask {
		resolved.DataDir = filepath.Join(filepath.Dir(a.store.Path()), "device-sessions-v2", resolved.SessionNamespace)
	}
	if err != nil {
		a.logger.Debug("Device credential runtime unavailable", "error", err.Error())
	}
	return resolved, err
}

func (a *App) writeDeviceAuthOutput(output deviceAuthOutput) error {
	runtimeContext, err := a.resolveDeviceRuntime()
	if err != nil {
		return err
	}
	output.CredentialScope = deviceCredentialScope(runtimeContext)
	return json.NewEncoder(a.stdout).Encode(output)
}

func deviceCredentialScope(runtimeContext credential.DeviceRuntime) string {
	if runtimeContext.Kind == credential.DeviceRuntimeLocalUser {
		return "user"
	}
	return "task"
}

// Called under the same lock as auth complete, refresh and logout.
func (a *App) reuseDeviceAuthorization(ctx context.Context, profile config.Profile, store credential.Store, existing credential.DeviceCredential) (bool, error) {
	if existing.Token == nil || existing.Token.AccessToken == "" {
		return false, nil
	}
	if !existing.Token.Expiry.IsZero() && !a.now().Before(existing.Token.Expiry) && existing.Token.RefreshToken == "" {
		return false, nil
	}
	if _, err := a.refreshDeviceTokenUnderLock(ctx, profile, existing.Token.AccessToken, false, store); err != nil {
		return true, err
	}
	stored, err := a.loadProductionDeviceCredential(profile, store)
	if err != nil {
		return true, err
	}
	profile.Identities.User.AuthMode = config.UserAuthModeDevice
	profile.Identities.User.Token = nil
	profile.DefaultIdentity = config.IdentityUser
	if err := a.store.SaveProfile(profile); err != nil {
		return true, err
	}
	a.logger.Info("reused existing Device authorization", "profile", profile.Name)
	output := deviceAuthOutput{Status: "authorized"}
	if !stored.Token.Expiry.IsZero() {
		output.ExpiresAt = stored.Token.Expiry.Format(time.RFC3339)
		output.ExpiresAtDisplay = formatDeviceAuthorizationExpiry(stored.Token.Expiry)
	}
	return true, a.writeDeviceAuthOutput(output)
}

// A fresh config directory can recover the already-authorized local profile;
// explicit legacy Authorization Code profiles keep their chosen mode.
func (a *App) restoreLocalDeviceIdentity(profile config.Profile) (config.Profile, error) {
	if profile.Identities.User.AuthMode != "" || profile.Identities.User.Token != nil {
		return profile, nil
	}
	runtimeContext, err := a.resolveDeviceRuntime()
	if err != nil || runtimeContext.Kind != credential.DeviceRuntimeLocalUser {
		return profile, nil
	}
	store, err := a.deviceCredentials()
	if err != nil {
		return config.Profile{}, err
	}
	stored, err := a.loadProductionDeviceCredential(profile, store)
	if errors.Is(err, credential.ErrCredentialNotFound) {
		return profile, nil
	}
	if err != nil {
		return config.Profile{}, err
	}
	if stored.DeviceProfile == nil {
		return profile, nil
	}
	restored, err := restoreDeviceProfile(profile.Name, stored.DeviceProfile)
	if err != nil {
		return config.Profile{}, err
	}
	userIdentity := restored.Identities.User
	userIdentity.AuthorizationEndpoint = profile.Identities.User.AuthorizationEndpoint
	userIdentity.RegistrationEndpoint = profile.Identities.User.RegistrationEndpoint
	userIdentity.RedirectURL = profile.Identities.User.RedirectURL
	userIdentity.ClientID = profile.Identities.User.ClientID
	profile.Identities.User = userIdentity
	if err := a.store.SaveProfile(profile); err != nil {
		return config.Profile{}, err
	}
	a.logger.Info("restored local Device identity", "profile", profile.Name)
	return profile, nil
}
