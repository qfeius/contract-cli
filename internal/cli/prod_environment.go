package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

const (
	productionEnvironment        = "prod"
	productionOpenPlatformURL    = "https://open.qfei.cn"
	productionAccountOrigin      = "https://myaccount.qfei.cn"
	productionOpenPlatformOrigin = "https://open.qfei.cn"
)

// profileOrigins derives trust anchors from the explicitly configured addresses.
func profileOrigins(profile config.Profile) (string, string, bool) {
	open, err := parseProductionURL(profile.OpenPlatformBaseURL)
	if err != nil || strings.TrimSpace(profile.Environment) == "" {
		return "", "", false
	}
	openOrigin := open.Scheme + "://" + open.Host
	if profile.AuthorizationServerMetadataURL == "" {
		return openOrigin, "", true
	}
	account, err := parseProductionURL(profile.AuthorizationServerMetadataURL)
	if err != nil {
		return "", "", false
	}
	return openOrigin, account.Scheme + "://" + account.Host, true
}

/*
validateProductionProfile 校验资源及认证端点与 profile 声明的地址一致。
入参 profile（config.Profile）为配置；返回 error 表示环境或端点不合法。
*/
func validateProductionProfile(profile config.Profile) error {
	openOrigin, accountOrigin, supported := profileOrigins(profile)
	if !supported {
		return productionProfileError(profile.Name)
	}
	if !isExactEnvironmentResource(profile.OpenPlatformBaseURL, openOrigin) || !isExactEnvironmentResource(profile.Resource, openOrigin) {
		return productionProfileError(profile.Name)
	}
	openPlatformURLs := []string{
		profile.AppTokenEndpoint,
		profile.ProtectedResourceMetadataURL,
	}
	for _, rawURL := range openPlatformURLs {
		if !isProductionOriginURL(rawURL, openOrigin, false) {
			return productionProfileError(profile.Name)
		}
	}
	accountURLs := []string{
		profile.AuthorizationServerMetadataURL,
		profile.Identities.User.AuthorizationEndpoint,
		profile.Identities.User.DeviceAuthorizationEndpoint,
		profile.Identities.User.TokenEndpoint,
		profile.Identities.User.RevocationEndpoint,
		profile.Identities.User.RegistrationEndpoint,
	}
	for _, rawURL := range accountURLs {
		if !isProductionOriginURL(rawURL, accountOrigin, false) {
			return productionProfileError(profile.Name)
		}
	}
	return nil
}

// validateProductionDeviceCredential binds Device state to the actual target,
// even when the profile name and environment label have not changed.
func validateProductionDeviceCredential(profile config.Profile, stored credential.DeviceCredential) error {
	if stored.DeviceProfile == nil {
		if stored.Token != nil || stored.Pending != nil {
			return productionProfileError(profile.Name)
		}
		return nil
	}
	snapshot := stored.DeviceProfile
	if snapshot.Name != profile.Name || snapshot.Environment != profile.Environment ||
		snapshot.OpenPlatformBaseURL != profile.OpenPlatformBaseURL || snapshot.Resource != profile.Resource ||
		snapshot.ProtectedResourceMetadataURL != profile.ProtectedResourceMetadataURL ||
		snapshot.AuthorizationServerMetadataURL != profile.AuthorizationServerMetadataURL ||
		snapshot.DeviceAuthorizationEndpoint != profile.Identities.User.DeviceAuthorizationEndpoint ||
		snapshot.TokenEndpoint != profile.Identities.User.TokenEndpoint ||
		snapshot.RevocationEndpoint != profile.Identities.User.RevocationEndpoint ||
		snapshot.DeviceClientID != profile.Identities.User.DeviceClientID || snapshot.DeviceScope != profile.Identities.User.DeviceScope {
		return productionProfileError(profile.Name)
	}
	return validateProductionPendingTransaction(profile, stored.Pending)
}

func validateProductionPendingTransaction(profile config.Profile, pending *credential.PendingTransaction) error {
	_, accountOrigin, valid := profileOrigins(profile)
	if !valid || (pending != nil &&
		(pending.TokenEndpoint != profile.Identities.User.TokenEndpoint ||
			!isProductionOriginURL(pending.VerificationURIComplete, accountOrigin, false))) {
		return productionProfileError(profile.Name)
	}
	return nil
}

/*
isExactEnvironmentResource 拒绝非根路径、查询参数和不同环境的资源地址。
入参 rawURL、origin（string）为待校验地址和环境域名；返回 bool 表示是否匹配。
*/
func isExactEnvironmentResource(rawURL, origin string) bool {
	parsed, err := parseProductionURL(rawURL)
	if err != nil {
		return false
	}
	return parsed.Scheme+"://"+parsed.Host == origin &&
		(parsed.EscapedPath() == "" || parsed.EscapedPath() == "/") && parsed.RawQuery == ""
}

func isProductionOriginURL(rawURL, expectedOrigin string, required bool) bool {
	if strings.TrimSpace(rawURL) == "" {
		return !required
	}
	parsed, err := parseProductionURL(rawURL)
	if err != nil {
		return false
	}
	return parsed.Scheme+"://"+parsed.Host == expectedOrigin
}

func parseProductionURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("invalid production URL")
	}
	if parsed.Hostname() != strings.TrimSuffix(parsed.Hostname(), ".") {
		return nil, errors.New("trailing-dot hosts are not allowed")
	}
	return parsed, nil
}

/*
productionProfileError 返回环境配置不合法的统一错误，提示重新初始化 profile。
入参 profileName（string）为配置名；返回 error。
*/
func productionProfileError(profileName string) error {
	return fmt.Errorf(
		"profile %q has unsupported or mismatched environment endpoints; re-run `contract-cli config add` with the intended addresses and authorize again",
		profileName,
	)
}

// withProductionNetworkGuard preserves the configured target across redirects.
// Download/upload clients retain their existing, separate signed-URL policies.
func withProductionNetworkGuard(client *http.Client, logger *slog.Logger) *http.Client {
	guarded := *client
	logger.Debug("configure same-origin network guard")
	guarded.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		logger.Debug("check request redirect", "host", request.URL.Hostname())
		if len(via) > 0 && (request.URL.Scheme != via[0].URL.Scheme || !strings.EqualFold(request.URL.Host, via[0].URL.Host)) {
			logger.Error("reject cross-origin redirect", "host", request.URL.Hostname())
			return errors.New("profile request cannot redirect to a different origin")
		}
		if client.CheckRedirect != nil {
			return client.CheckRedirect(request, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &guarded
}

/*
loadProductionDeviceCredential 加载并按配置环境校验凭据。
入参 profile（config.Profile）为配置、store（credential.Store）为凭据库；返回凭据和 error。
*/
func (a *App) loadProductionDeviceCredential(profile config.Profile, store credential.Store) (credential.DeviceCredential, error) {
	stored, err := store.Load(profile.Name)
	if err != nil {
		return credential.DeviceCredential{}, err
	}
	if err := validateProductionDeviceCredential(profile, stored); err != nil {
		a.logger.Error("reject mismatched Device credential", "profile", profile.Name, "error", err.Error())
		return credential.DeviceCredential{}, err
	}
	return stored, nil
}

/*
validateProductionDeviceCredentialIfAvailable 在凭据库可用时校验环境一致性。
入参 profile（config.Profile）为配置；返回 error。
*/
func (a *App) validateProductionDeviceCredentialIfAvailable(profile config.Profile) error {
	store, available, err := a.deviceCredentialStoreIfAvailable()
	if err != nil || !available {
		return err
	}
	stored, err := store.Load(profile.Name)
	if errors.Is(err, credential.ErrCredentialNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := validateProductionDeviceCredential(profile, stored); err != nil {
		a.logger.Error("reject mismatched Device credential", "profile", profile.Name, "error", err.Error())
		return err
	}
	return nil
}

func (a *App) deviceCredentialStoreIfAvailable() (credential.Store, bool, error) {
	if a.credentialStore != nil {
		return a.credentialStore, true, nil
	}
	for _, name := range []string{"SKILL_SESSION_WORKSPACE", "CODEBUDDY_SESSION_ID", "SESSION_ID"} {
		if value, ok := a.lookupEnv(name); ok && strings.TrimSpace(value) != "" {
			store, err := a.deviceCredentials()
			return store, err == nil, err
		}
	}
	// Native desktop clients can use the user keyring without session variables.
	// Unknown runtimes still support config commands without a Device store.
	runtimeContext, err := a.resolveDeviceRuntime()
	if err == nil && runtimeContext.Kind == credential.DeviceRuntimeLocalUser {
		store, err := a.deviceCredentials()
		return store, err == nil, err
	}
	return nil, false, nil
}

/*
productionProfileRequiresReset 判断配置或凭据环境不合法时是否需要重置认证。
入参 profile（config.Profile）为旧配置；返回是否重置（bool）和 error。
*/
func (a *App) productionProfileRequiresReset(profile config.Profile) (bool, error) {
	if validateProductionProfile(profile) != nil || config.ValidateCredentialBinding(profile) != nil {
		return true, nil
	}
	store, available, err := a.deviceCredentialStoreIfAvailable()
	if err != nil || !available {
		return false, err
	}
	stored, err := store.Load(profile.Name)
	if errors.Is(err, credential.ErrCredentialNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return validateProductionDeviceCredential(profile, stored) != nil, nil
}

func (a *App) clearProfileAuthenticationState(ctx context.Context, profileName string) (err error) {
	var outcome *localCommandOutcome
	defer func() { err = withCompletedLocalCommand(ctx, err, outcome) }()
	a.logger.Info("clear changed profile authentication state", "profile", profileName)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := a.secrets.Delete(config.AppSecretKey(profileName)); err != nil {
		a.logger.Error("clear app secret failed", "profile", profileName, "error", err.Error())
		return err
	}
	outcome = &localCommandOutcome{Operation: "config add", Result: map[string]any{"profile": profileName, "app_secret_cleared": true, "profile_saved": false}}
	if err := ctx.Err(); err != nil {
		return err
	}
	store, available, err := a.deviceCredentialStoreIfAvailable()
	if err != nil {
		a.logger.Error("resolve Device credential store for reset failed", "profile", profileName, "error", err.Error())
		return err
	}
	if available {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := store.Delete(profileName); err != nil {
			a.logger.Error("clear Device credential failed", "profile", profileName, "error", err.Error())
			return err
		}
		outcome = &localCommandOutcome{Operation: "config add", Result: map[string]any{"profile": profileName, "app_secret_cleared": true, "device_credentials_cleared": true, "profile_saved": false}}
	}
	return ctx.Err()
}
