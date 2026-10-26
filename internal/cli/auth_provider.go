package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/oauth"
)

const (
	envAppID              = "CONTRACT_CLI_APP_ID"
	envAppSecret          = "CONTRACT_CLI_APP_SECRET"
	legacyEnvBotAppID     = "DEMOCLI_BOT_APP_ID"
	legacyEnvBotAppSecret = "DEMOCLI_BOT_APP_SECRET"
	legacyEnvAppID        = "CONTRACT_CLI_BOT_APP_ID"
	legacyEnvAppSecret    = "CONTRACT_CLI_BOT_APP_SECRET"
)

type authCommandOptions struct {
	Identity      config.IdentityKind
	ProfileName   string
	Timeout       time.Duration
	NoOpenBrowser bool
	AppID         string
	AppSecret     string
}

type authStatusField struct {
	Label string
	Value string
}

type authStatusView struct {
	Authorization string
	Fields        []authStatusField
}

type authProvider interface {
	Login(context.Context, *config.Profile, authCommandOptions) (string, error)
	Status(context.Context, config.Profile, authCommandOptions) (authStatusView, error)
	Logout(context.Context, *config.Profile, authCommandOptions) (string, error)
}

type userAuthProvider struct {
	httpClient             *http.Client
	logger                 *slog.Logger
	openBrowser            func(string) error
	authorizationURLWriter io.Writer
	startCallbackServer    func(string) (authorizationCallback, error)
}

type authorizationCallback interface {
	Wait(context.Context, string) (string, error)
}

func (p userAuthProvider) Login(ctx context.Context, profile *config.Profile, options authCommandOptions) (string, error) {
	p.logger.Info("user auth login started", "profile", profile.Name)

	user := &profile.Identities.User
	user.AuthMode = config.UserAuthModeAuthorizationCode
	if user.RegistrationEndpoint == "" || user.AuthorizationEndpoint == "" || user.TokenEndpoint == "" || user.RedirectURL == "" {
		return "", fmt.Errorf("user identity is not configured; run `contract-cli config add` first")
	}

	p.logger.Info("register oauth client", "profile", profile.Name, "registration_endpoint", user.RegistrationEndpoint)
	registration, err := oauth.RegisterClient(ctx, p.httpClient, p.logger, user.RegistrationEndpoint, oauth.ClientRegistrationRequest{
		ClientName:              profile.ClientName,
		RedirectURIs:            []string{user.RedirectURL},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "",
	})
	if err != nil {
		return "", err
	}
	user.ClientID = registration.ClientID

	verifier, err := oauth.NewCodeVerifier(nil)
	if err != nil {
		return "", err
	}
	state, err := oauth.NewState(nil)
	if err != nil {
		return "", err
	}

	startCallbackServer := p.startCallbackServer
	if startCallbackServer == nil {
		startCallbackServer = func(redirectURL string) (authorizationCallback, error) {
			return oauth.StartCallbackServer(redirectURL)
		}
	}
	callbackServer, err := startCallbackServer(user.RedirectURL)
	if err != nil {
		return "", err
	}

	authURL, err := oauth.BuildAuthorizationURL(oauth.AuthorizationRequest{
		AuthorizationEndpoint: user.AuthorizationEndpoint,
		BusinessType:          profile.BusinessType,
		ClientID:              user.ClientID,
		RedirectURL:           user.RedirectURL,
		Resource:              profile.Resource,
		Scopes:                profile.Scopes,
		State:                 state,
		CodeChallenge:         oauth.S256Challenge(verifier),
	})
	if err != nil {
		return "", err
	}

	p.logger.Info("user auth authorization url prepared", "profile", profile.Name, "no_open_browser", options.NoOpenBrowser)
	if options.NoOpenBrowser {
		if p.authorizationURLWriter != nil {
			_, _ = fmt.Fprint(p.authorizationURLWriter, authorizationURLMessage(authURL))
		}
	} else {
		if err := p.openBrowser(authURL); err != nil {
			p.logger.Warn("open browser failed", "profile", profile.Name, "error", err.Error())
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	code, err := callbackServer.Wait(waitCtx, state)
	if err != nil {
		return "", err
	}

	token, err := oauth.ExchangeAuthorizationCode(ctx, p.httpClient, p.logger, oauth.TokenExchangeRequest{
		TokenEndpoint: user.TokenEndpoint,
		ClientID:      user.ClientID,
		Code:          code,
		CodeVerifier:  verifier,
		RedirectURL:   user.RedirectURL,
		Resource:      profile.Resource,
	})
	if err != nil {
		return "", err
	}

	user.Token = token
	p.logger.Info("user auth login completed", "profile", profile.Name)

	var builder strings.Builder
	_, _ = fmt.Fprintf(&builder, "Authorization succeeded for profile %q.", profile.Name)
	if !token.Expiry.IsZero() {
		_, _ = fmt.Fprintf(&builder, "\nAccess token expires at: %s", token.Expiry.Format(time.RFC3339))
	}
	return builder.String(), nil
}

func authorizationURLMessage(authURL string) string {
	return fmt.Sprintf("Open this URL and finish authorization:\n%s\n", authURL)
}

func (p userAuthProvider) Status(_ context.Context, profile config.Profile, _ authCommandOptions) (authStatusView, error) {
	user := profile.Identities.User
	view := authStatusView{
		Authorization: "unauthorized",
		Fields: []authStatusField{
			{Label: "Client ID", Value: emptyFallback(user.ClientID, "<not-registered>")},
		},
	}
	if user.Token == nil || user.Token.AccessToken == "" {
		return view, nil
	}

	view.Authorization = "authorized"
	if !user.Token.Expiry.IsZero() {
		if time.Now().After(user.Token.Expiry) {
			view.Authorization = "expired"
		}
		view.Fields = append(view.Fields, authStatusField{
			Label: "Expires At",
			Value: user.Token.Expiry.Format(time.RFC3339),
		})
	}
	return view, nil
}

func (p userAuthProvider) Logout(_ context.Context, profile *config.Profile, _ authCommandOptions) (string, error) {
	p.logger.Info("user auth logout", "profile", profile.Name)
	profile.Identities.User.Token = nil
	return fmt.Sprintf("Logged out user identity for profile %q.", profile.Name), nil
}

type appAuthProvider struct {
	httpClient *http.Client
	logger     *slog.Logger
	secrets    *config.SecretsStore
	lookupEnv  func(string) (string, bool)
}

type appCredentials struct {
	appID     string
	appSecret string
	source    string
}

/*
Login 使用所选 profile 的环境端点完成 app 身份登录，并保存本地凭据引用。
入参 ctx（context.Context）为请求上下文，profile（*config.Profile）为目标配置，options（authCommandOptions）为登录参数；返回提示文本（string）和 error。
*/
func (p appAuthProvider) Login(ctx context.Context, profile *config.Profile, options authCommandOptions) (string, error) {
	p.logger.Info("app auth login started", "profile", profile.Name)

	credentials, err := p.resolveCredentials(*profile, options, true)
	if err != nil {
		p.logger.Error("app auth resolve credentials failed", "profile", profile.Name, "error", err.Error())
		return "", err
	}
	if profile.AppTokenEndpoint == "" {
		err = fmt.Errorf("app identity is not configured; run `contract-cli config add --env %s --name %s` first", profile.Environment, profile.Name)
		p.logger.Error("app auth token endpoint missing", "profile", profile.Name, "environment", profile.Environment, "error", err.Error())
		return "", err
	}

	secretKey := config.AppSecretKey(profile.Name)
	if err := p.secrets.Set(secretKey, credentials.appSecret); err != nil {
		p.logger.Error("app auth save secret failed", "profile", profile.Name, "error", err.Error())
		return "", err
	}

	profile.Identities.App = config.AppIdentity{
		AuthMode:     config.AppAuthModeAppCredentials,
		AppID:        credentials.appID,
		SecretRef:    secretKey,
		ConfiguredAt: time.Now().UTC(),
		Token:        nil,
	}
	p.logger.Info("app credentials saved", "profile", profile.Name, "source", credentials.source)

	p.logger.Info("app token exchange started", "profile", profile.Name, "token_endpoint", profile.AppTokenEndpoint)
	token, err := oauth.ExchangeTenantAccessToken(ctx, p.httpClient, p.logger, profile.AppTokenEndpoint, credentials.appID, credentials.appSecret)
	if err != nil {
		p.logger.Error("app token exchange failed", "profile", profile.Name, "token_endpoint", profile.AppTokenEndpoint, "error", err.Error())
		profile.Identities.App.Token = nil
		return "", err
	}

	profile.Identities.App.Token = token
	p.logger.Info("app auth login completed", "profile", profile.Name, "token_endpoint", profile.AppTokenEndpoint, "expires_at", token.Expiry.Format(time.RFC3339))

	var builder strings.Builder
	_, _ = fmt.Fprintf(&builder, "App authorization succeeded for profile %q.", profile.Name)
	if !token.Expiry.IsZero() {
		_, _ = fmt.Fprintf(&builder, "\nAccess token expires at: %s", token.Expiry.Format(time.RFC3339))
	}
	return builder.String(), nil
}

func (p appAuthProvider) Status(_ context.Context, profile config.Profile, options authCommandOptions) (authStatusView, error) {
	credentials, err := p.resolveCredentials(profile, options, false)
	if err != nil {
		return authStatusView{}, err
	}

	authorization := "unconfigured"
	secretState := "missing"
	if credentials.appSecret != "" {
		secretState = "configured"
	}
	if credentials.appID != "" && credentials.appSecret != "" {
		authorization = "configured"
	}
	if token := profile.Identities.App.Token; token != nil && token.AccessToken != "" {
		authorization = "authorized"
		if !token.Expiry.IsZero() && time.Now().After(token.Expiry) {
			authorization = "expired"
		}
	}

	fields := []authStatusField{
		{Label: "Auth Mode", Value: emptyFallback(profile.Identities.App.AuthMode, config.AppAuthModeAppCredentials)},
		{Label: "App ID", Value: emptyFallback(credentials.appID, "<not-configured>")},
		{Label: "App Secret", Value: secretState},
		{Label: "Credential Source", Value: credentials.source},
		{Label: "Token Protocol", Value: "tenant_access_token/internal"},
	}
	if token := profile.Identities.App.Token; token != nil && !token.Expiry.IsZero() {
		fields = append(fields, authStatusField{
			Label: "Expires At",
			Value: token.Expiry.Format(time.RFC3339),
		})
	}

	return authStatusView{
		Authorization: authorization,
		Fields:        fields,
	}, nil
}

func (p appAuthProvider) Logout(_ context.Context, profile *config.Profile, _ authCommandOptions) (string, error) {
	p.logger.Info("app auth logout", "profile", profile.Name, "preserve_credentials", true)
	profile.Identities.App.Token = nil
	p.logger.Info("app auth logout completed", "profile", profile.Name, "preserve_credentials", true)
	return fmt.Sprintf("Logged out app token for profile %q while keeping app credentials.", profile.Name), nil
}

func (p appAuthProvider) resolveCredentials(profile config.Profile, options authCommandOptions, requireComplete bool) (appCredentials, error) {
	appID, appIDSource := p.resolveAppID(profile, options)
	appSecret, appSecretSource, err := p.resolveAppSecret(profile, options)
	if err != nil {
		return appCredentials{}, err
	}

	credentials := appCredentials{
		appID:     appID,
		appSecret: appSecret,
		source:    combineCredentialSources(appIDSource, appSecretSource),
	}
	if requireComplete && (credentials.appID == "" || credentials.appSecret == "") {
		return appCredentials{}, fmt.Errorf("app credentials are incomplete; provide --app-id/--app-secret or set %s/%s", envAppID, envAppSecret)
	}
	return credentials, nil
}

func (p appAuthProvider) resolveAppID(profile config.Profile, options authCommandOptions) (string, string) {
	switch {
	case options.AppID != "":
		return options.AppID, "flag"
	default:
		if value, ok := lookupEnvAny(p.lookupEnv, envAppID, legacyEnvAppID, legacyEnvBotAppID); ok {
			return value, "env"
		}
		if profile.Identities.App.AppID != "" {
			return profile.Identities.App.AppID, "secrets"
		}
		return "", "missing"
	}
}

func (p appAuthProvider) resolveAppSecret(profile config.Profile, options authCommandOptions) (string, string, error) {
	switch {
	case options.AppSecret != "":
		return options.AppSecret, "flag", nil
	default:
		if value, ok := lookupEnvAny(p.lookupEnv, envAppSecret, legacyEnvAppSecret, legacyEnvBotAppSecret); ok {
			return value, "env", nil
		}
		if profile.Identities.App.SecretRef != "" {
			value, ok, err := p.secrets.Get(profile.Identities.App.SecretRef)
			if err != nil {
				return "", "", err
			}
			if ok && value != "" {
				return value, "secrets", nil
			}
		}
		return "", "missing", nil
	}
}

func combineCredentialSources(appIDSource, appSecretSource string) string {
	switch {
	case appIDSource == "missing" && appSecretSource == "missing":
		return "missing"
	case appIDSource == appSecretSource:
		return appIDSource
	case appIDSource == "missing":
		return appSecretSource
	case appSecretSource == "missing":
		return appIDSource
	default:
		return "mixed"
	}
}

func lookupEnvAny(lookup func(string) (string, bool), keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := lookup(key); ok && value != "" {
			return value, true
		}
	}
	return "", false
}
