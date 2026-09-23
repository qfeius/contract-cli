package cli

import (
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

/*
environmentOrigins 只返回线上 prod 环境域名，避免旧配置跨环境复用。
入参 environment（string）为环境名；返回 API 域名、账号域名（string）及是否支持（bool）。
*/
func environmentOrigins(environment string) (string, string, bool) {
	if environment == productionEnvironment {
		return productionOpenPlatformOrigin, productionAccountOrigin, true
	}
	return "", "", false
}

var blockedProductionBuildHosts = map[string]struct{}{
	"dev-open.qtech.cn":       {},
	"dev-myaccount.qtech.cn":  {},
	"test-open.qtech.cn":      {},
	"test-myaccount.qtech.cn": {},
	"open-b.qfei.cn":          {},
	"myaccount-b.qfei.cn":     {},
}

/*
validateProductionProfile 校验配置及认证端点都属于线上 prod 环境。
入参 profile（config.Profile）为配置；返回 error 表示环境或端点不合法。
*/
func validateProductionProfile(profile config.Profile) error {
	openOrigin, accountOrigin, supported := environmentOrigins(strings.TrimSpace(profile.Environment))
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

/*
validateProductionDeviceCredential 校验快照与待授权状态属于当前环境。
入参 profileName（string）为配置名，stored（credential.DeviceCredential）为凭据，environment（可选 string）默认为 prod；返回校验 error。
*/
func validateProductionDeviceCredential(profileName string, stored credential.DeviceCredential, environment ...string) error {
	env := productionEnvironment
	if len(environment) > 0 {
		env = environment[0]
	}
	if stored.DeviceProfile != nil {
		profile, err := restoreDeviceProfile(profileName, stored.DeviceProfile)
		if err != nil || profile.Environment != env || validateProductionProfile(profile) != nil {
			return productionProfileError(profileName)
		}
	}
	return validateProductionPendingTransaction(profileName, stored.Pending, env)
}

/*
validateProductionPendingTransaction 校验授权中的链接和 token 端点，阻止跨环境续用。
入参 profileName（string）为配置名，pending（*credential.PendingTransaction）为待授权状态，environment（可选 string）默认为 prod；返回校验 error。
*/
func validateProductionPendingTransaction(profileName string, pending *credential.PendingTransaction, environment ...string) error {
	env := productionEnvironment
	if len(environment) > 0 {
		env = environment[0]
	}
	_, accountOrigin, supported := environmentOrigins(env)
	if !supported {
		return productionProfileError(profileName)
	}
	if pending != nil &&
		(!isProductionOriginURL(pending.TokenEndpoint, accountOrigin, true) ||
			!isProductionOriginURL(pending.VerificationURIComplete, accountOrigin, false)) {
		return productionProfileError(profileName)
	}
	return nil
}

/*
isExactProductionResource 保持原有生产资源校验入口。
入参 rawURL（string）为资源地址；返回 bool 表示是否为精确生产资源。
*/
func isExactProductionResource(rawURL string) bool {
	return isExactEnvironmentResource(rawURL, productionOpenPlatformURL)
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
productionProfileError 返回非生产配置的统一错误，提示重新初始化 prod profile。
入参 profileName（string）为配置名；返回 error。
*/
func productionProfileError(profileName string) error {
	return fmt.Errorf(
		"profile %q belongs to a non-production environment and is not allowed in this production build; run `contract-cli config add --env prod --name contract` and authorize again",
		profileName,
	)
}

func withProductionNetworkGuard(client *http.Client, logger *slog.Logger) *http.Client {
	guarded := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	guarded.Transport = productionGuardTransport{next: transport, logger: logger}
	return &guarded
}

type productionGuardTransport struct {
	next   http.RoundTripper
	logger *slog.Logger
}

/*
RoundTrip 在传输层拒绝已知 dev/test/blue 域名，避免旧配置绕过生产 profile 校验。
入参 request（*http.Request）为请求；返回 HTTP 响应和 error。
*/
func (transport productionGuardTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	host := strings.ToLower(strings.TrimSuffix(request.URL.Hostname(), "."))
	transport.logger.Debug("production network request", "method", request.Method, "host", host)
	if _, blocked := blockedProductionBuildHosts[host]; blocked {
		transport.logger.Error("production build blocked non-production network request", "method", request.Method, "host", host)
		return nil, fmt.Errorf("production build blocks non-production host %q", host)
	}
	return transport.next.RoundTrip(request)
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
	if err := validateProductionDeviceCredential(profile.Name, stored, profile.Environment); err != nil {
		a.logger.Error("reject non-production Device credential", "profile", profile.Name, "error", err.Error())
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
	if err := validateProductionDeviceCredential(profile.Name, stored, profile.Environment); err != nil {
		a.logger.Error("reject non-production Device credential", "profile", profile.Name, "error", err.Error())
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
	return nil, false, nil
}

/*
productionProfileRequiresReset 判断配置或凭据环境不合法时是否需要重置认证。
入参 profile（config.Profile）为旧配置；返回是否重置（bool）和 error。
*/
func (a *App) productionProfileRequiresReset(profile config.Profile) (bool, error) {
	if validateProductionProfile(profile) != nil {
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
	return validateProductionDeviceCredential(profile.Name, stored, profile.Environment) != nil, nil
}

func (a *App) clearProfileAuthenticationState(profileName string) error {
	a.logger.Info("clear non-production profile authentication state", "profile", profileName)
	if err := a.secrets.Delete(config.AppSecretKey(profileName)); err != nil {
		a.logger.Error("clear app secret failed", "profile", profileName, "error", err.Error())
		return err
	}
	store, available, err := a.deviceCredentialStoreIfAvailable()
	if err != nil {
		a.logger.Error("resolve Device credential store for reset failed", "profile", profileName, "error", err.Error())
		return err
	}
	if available {
		if err := store.Delete(profileName); err != nil {
			a.logger.Error("clear Device credential failed", "profile", profileName, "error", err.Error())
			return err
		}
	}
	return nil
}
