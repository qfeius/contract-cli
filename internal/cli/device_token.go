package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/oauth"
	"cn.qfei/contract-cli/internal/openplatform"
	"github.com/gofrs/flock"
)

const deviceTokenRefreshSkew = 5 * time.Minute

var ErrDeviceReauthorizationRequired = errors.New("device authorization must be renewed")

func (a *App) deviceRequestContext(profile config.Profile) (openplatform.RequestContext, error) {
	store, err := a.deviceCredentials()
	if err != nil {
		return openplatform.RequestContext{}, err
	}
	stored, err := a.loadProductionDeviceCredential(profile, store)
	if err != nil {
		if errors.Is(err, credential.ErrCredentialNotFound) {
			return openplatform.RequestContext{}, fmt.Errorf("user identity is not authorized; run `contract-cli auth init --profile %s --output json` first", profile.Name)
		}
		return openplatform.RequestContext{}, err
	}
	if stored.Token == nil || strings.TrimSpace(stored.Token.AccessToken) == "" {
		return openplatform.RequestContext{}, fmt.Errorf("user identity is not authorized; run `contract-cli auth init --profile %s --output json` first", profile.Name)
	}
	if strings.TrimSpace(profile.OpenPlatformBaseURL) == "" {
		return openplatform.RequestContext{}, fmt.Errorf("open platform base url is not configured for profile %q", profile.Name)
	}

	return openplatform.RequestContext{
		Profile: profile, Identity: config.IdentityUser,
		BaseURL: strings.TrimRight(profile.OpenPlatformBaseURL, "/"), AccessToken: stored.Token.AccessToken,
		PrepareAccessToken: func(ctx context.Context, currentAccessToken string) (string, error) {
			return a.refreshDeviceToken(ctx, profile, currentAccessToken, false)
		},
		RefreshAccessToken: func(ctx context.Context, currentAccessToken string) (string, error) {
			return a.refreshDeviceToken(ctx, profile, currentAccessToken, true)
		},
	}, nil
}

func (a *App) deviceAuthStatus(profile config.Profile) (authStatusView, error) {
	view := authStatusView{
		Authorization: "unauthorized",
		Fields:        []authStatusField{{Label: "Device Client ID", Value: emptyFallback(profile.Identities.User.DeviceClientID, "<not-configured>")}},
	}
	if runtimeContext, err := a.resolveDeviceRuntime(); err == nil {
		view.Fields = append(view.Fields, authStatusField{Label: "Credential Scope", Value: deviceCredentialScope(runtimeContext)})
	}
	store, err := a.deviceCredentials()
	if err != nil {
		return authStatusView{}, err
	}
	stored, err := a.loadProductionDeviceCredential(profile, store)
	if errors.Is(err, credential.ErrCredentialNotFound) {
		return view, nil
	}
	if err != nil {
		return authStatusView{}, err
	}
	if stored.Token == nil || stored.Token.AccessToken == "" {
		if stored.Pending != nil {
			switch stored.Pending.EffectiveStatus() {
			case credential.PendingStatusPending:
				if a.now().Before(stored.Pending.ExpiresAt) {
					view.Authorization = "pending"
				} else {
					view.Authorization = "expired"
				}
			case credential.PendingStatusChecking, credential.PendingStatusUncertain:
				view.Authorization = "uncertain"
			case credential.PendingStatusDenied:
				view.Authorization = "denied"
			case credential.PendingStatusExpired:
				view.Authorization = "expired"
			case credential.PendingStatusInvalidGrant:
				view.Authorization = "restart_required"
			default:
				return authStatusView{}, fmt.Errorf("unsupported pending device authorization status %q", stored.Pending.Status)
			}
		}
		return view, nil
	}
	view.Authorization = "authorized"
	if !stored.Token.Expiry.IsZero() {
		if !a.now().Before(stored.Token.Expiry) {
			view.Authorization = "expired"
		}
		view.Fields = append(view.Fields, authStatusField{Label: "Expires At", Value: stored.Token.Expiry.Format(time.RFC3339)})
	}
	if stored.Token.Scope != "" {
		view.Fields = append(view.Fields, authStatusField{Label: "Scope", Value: stored.Token.Scope})
	}
	return view, nil
}

func (a *App) deviceAuthLogout(ctx context.Context, profile config.Profile) (message string, err error) {
	var outcome *localCommandOutcome
	defer func() { err = withCompletedLocalCommand(ctx, err, outcome) }()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	store, err := a.deviceCredentials()
	if err != nil {
		return "", err
	}
	a.logger.Info("Device logout started", "profile", profile.Name)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	release, acquired, err := a.tryDeviceAuthorizationOperation(profile.Name)
	if err != nil {
		return "", err
	}
	if !acquired {
		return "", errors.New("device credential operation is busy; retry logout after the active operation completes")
	}
	defer release()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	stored, err := a.loadProductionDeviceCredential(profile, store)
	if errors.Is(err, credential.ErrCredentialNotFound) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return fmt.Sprintf("Device authorization is already cleared for profile %q.", profile.Name), nil
	}
	if err != nil {
		return "", err
	}
	revoked := false
	if stored.Token != nil && strings.TrimSpace(stored.Token.RefreshToken) != "" {
		user := profile.Identities.User
		if strings.TrimSpace(user.RevocationEndpoint) == "" || strings.TrimSpace(user.DeviceClientID) == "" {
			return "", errors.New("device revocation endpoint or client id is not configured")
		}
		a.logger.Info("device authorization revoke started", "profile", profile.Name)
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := oauth.RevokeDeviceToken(ctx, a.httpClient, oauth.DeviceRevokeRequest{
			Endpoint: user.RevocationEndpoint, ClientID: user.DeviceClientID, RefreshToken: stored.Token.RefreshToken,
		}); err != nil {
			a.logger.Error("device authorization revoke failed", "profile", profile.Name, "error", err.Error())
			return "", err
		}
		revoked = true
		outcome = &localCommandOutcome{Operation: "auth logout", Result: map[string]any{
			"profile": profile.Name, "device_token_revoked": true, "device_credentials_cleared": false,
		}}
	}
	if !revoked {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	// A confirmed remote revocation invalidates the stored token. Finish local
	// cleanup under the existing operation lock even if cancellation arrived late.
	if err := store.Delete(profile.Name); err != nil {
		a.logger.Error("clear Device logout credential failed", "profile", profile.Name, "error", err.Error())
		return "", err
	}
	outcome = &localCommandOutcome{Operation: "auth logout", Result: map[string]any{
		"profile": profile.Name, "device_token_revoked": revoked, "device_credentials_cleared": true,
	}}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Logged out device user identity for profile %q.", profile.Name), nil
}

func (a *App) refreshDeviceToken(ctx context.Context, profile config.Profile, expectedAccessToken string, force bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	store, err := a.deviceCredentials()
	if err != nil {
		return "", err
	}
	beforeLock, err := a.loadProductionDeviceCredential(profile, store)
	if err != nil {
		return "", err
	}
	if beforeLock.Token == nil || strings.TrimSpace(beforeLock.Token.AccessToken) == "" {
		return "", errors.New("device access token is not available")
	}
	if beforeLock.Token.AccessToken != expectedAccessToken {
		return beforeLock.Token.AccessToken, nil
	}
	if !force && (beforeLock.Token.Expiry.IsZero() || beforeLock.Token.Expiry.After(a.now().Add(deviceTokenRefreshSkew))) {
		return beforeLock.Token.AccessToken, nil
	}

	lock, err := a.deviceRefreshLock(profile.Name)
	if err != nil {
		return "", err
	}
	locked, err := lock.TryLock()
	if err != nil {
		return "", fmt.Errorf("acquire device token refresh lock: %w", err)
	}
	if !locked {
		return "", errors.New("device token refresh is already in progress; retry the request")
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			a.logger.Error("release device token refresh lock failed", "profile", profile.Name, "error", unlockErr.Error())
		}
	}()

	return a.refreshDeviceTokenUnderLock(ctx, profile, expectedAccessToken, force, store)
}

func (a *App) refreshDeviceTokenUnderLock(ctx context.Context, profile config.Profile, expectedAccessToken string, force bool, store credential.Store) (string, error) {
	stored, err := a.loadProductionDeviceCredential(profile, store)
	if err != nil {
		return "", err
	}
	if stored.Token == nil || strings.TrimSpace(stored.Token.AccessToken) == "" {
		return "", errors.New("device access token is not available")
	}
	current := stored.Token
	if current.AccessToken != expectedAccessToken {
		return current.AccessToken, nil
	}
	if !force && (current.Expiry.IsZero() || current.Expiry.After(a.now().Add(deviceTokenRefreshSkew))) {
		return current.AccessToken, nil
	}
	if strings.TrimSpace(current.RefreshToken) == "" {
		return "", errors.New("device refresh token is not available; authorize again")
	}
	user := profile.Identities.User
	if strings.TrimSpace(user.TokenEndpoint) == "" || strings.TrimSpace(user.DeviceClientID) == "" {
		return "", errors.New("device token endpoint or client id is not configured")
	}

	a.logger.Info("device token refresh started", "profile", profile.Name, "forced", force)
	refreshed, err := oauth.RefreshDeviceToken(ctx, a.httpClient, oauth.DeviceRefreshRequest{
		Endpoint: user.TokenEndpoint, ClientID: user.DeviceClientID, RefreshToken: current.RefreshToken,
	})
	if err != nil {
		a.logger.Error("device token refresh failed", "profile", profile.Name, "error", err.Error())
		if oauth.IsDeviceGrantError(err, "invalid_grant") {
			stored.Token = nil
			if saveErr := store.Save(profile.Name, stored); saveErr != nil {
				return "", fmt.Errorf("clear rejected device credential for profile %q: %w", profile.Name, saveErr)
			}
			return "", fmt.Errorf(
				"%w; user confirmation is required before checking authorization status and starting or restarting Device authorization for profile %q",
				ErrDeviceReauthorizationRequired,
				profile.Name,
			)
		}
		return "", err
	}
	stored.Token = refreshed
	if err := store.Save(profile.Name, stored); err != nil {
		return "", fmt.Errorf(
			"save refreshed device credential for profile %q: authorization state may be invalid; authorize again before retrying: %w",
			profile.Name,
			err,
		)
	}
	a.logger.Info("device token refresh completed", "profile", profile.Name, "expires_at", refreshed.Expiry.Format(time.RFC3339))
	return refreshed.AccessToken, nil
}

func (a *App) deviceRefreshLock(profileName string) (*flock.Flock, error) {
	return a.deviceCredentialOperationLock(profileName)
}

func (a *App) deviceAuthorizationLock(profileName string) (*flock.Flock, error) {
	return a.deviceCredentialOperationLock(profileName)
}

func (a *App) deviceCredentialOperationLock(profileName string) (*flock.Flock, error) {
	runtimeContext, err := a.resolveDeviceRuntime()
	if err != nil {
		return nil, err
	}
	baseDir := ""
	namespace := profileName
	switch runtimeContext.Kind {
	case credential.DeviceRuntimeDoubaoCloud:
		baseDir = filepath.Join(runtimeContext.Workspace, ".contract-cli", "locks")
		namespace = runtimeContext.Workspace + ":" + namespace
	case credential.DeviceRuntimeDoubaoWorkTask:
		baseDir = filepath.Join(runtimeContext.DataDir, "locks")
		namespace = runtimeContext.SessionNamespace + ":" + namespace
	case credential.DeviceRuntimeLocalUser:
		directory, err := a.localDeviceDirectory()
		if err != nil {
			return nil, err
		}
		baseDir = filepath.Join(directory, "locks")
		namespace = "local-user-v1:" + profileName
	case credential.DeviceRuntimeWorkBuddy:
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("resolve device credential operation lock directory: %w", err)
		}
		baseDir = filepath.Join(cacheDir, "contract-cli", "locks")
		// Keep the existing WorkBuddy namespace stable for backward compatibility.
		namespace = runtimeContext.SessionID + ":" + namespace
	default:
		return nil, fmt.Errorf("unsupported Device runtime %q", runtimeContext.Kind)
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("create device operation lock directory: %w", err)
	}
	digest := sha256.Sum256([]byte(namespace))
	return flock.New(filepath.Join(baseDir, hex.EncodeToString(digest[:])+".lock")), nil
}
