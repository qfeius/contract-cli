package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/oauth"
)

const (
	envBotAppID           = "CONTRACT_CLI_BOT_APP_ID"
	envBotAppSecret       = "CONTRACT_CLI_BOT_APP_SECRET"
	legacyEnvBotAppID     = "DEMOCLI_BOT_APP_ID"
	legacyEnvBotAppSecret = "DEMOCLI_BOT_APP_SECRET"
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
	httpClient  *http.Client
	logger      *slog.Logger
	openBrowser func(string) error
}

func (p userAuthProvider) Login(ctx context.Context, profile *config.Profile, options authCommandOptions) (string, error) {
	p.logger.Info("user auth login started", "profile", profile.Name)

	user := &profile.Identities.User
	if user.RegistrationEndpoint == "" || user.AuthorizationEndpoint == "" || user.TokenEndpoint == "" || user.RedirectURL == "" {
		return "", fmt.Errorf("user identity is not configured; run `contract-cli config add` first")
	}

	if user.ClientID == "" {
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
	}

	verifier, err := oauth.NewCodeVerifier(nil)
	if err != nil {
		return "", err
	}
	state, err := oauth.NewState(nil)
	if err != nil {
		return "", err
	}

	callbackServer, err := oauth.StartCallbackServer(user.RedirectURL)
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

	if !options.NoOpenBrowser {
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
	builder.WriteString(fmt.Sprintf("Open this URL and finish authorization:\n%s\n", authURL))
	builder.WriteString(fmt.Sprintf("Authorization succeeded for profile %q.", profile.Name))
	if !token.Expiry.IsZero() {
		builder.WriteString(fmt.Sprintf("\nAccess token expires at: %s", token.Expiry.Format(time.RFC3339)))
	}
	return builder.String(), nil
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

type botAuthProvider struct {
	logger    *slog.Logger
	secrets   *config.SecretsStore
	lookupEnv func(string) (string, bool)
}

type botCredentials struct {
	appID     string
	appSecret string
	source    string
}

func (p botAuthProvider) Login(_ context.Context, profile *config.Profile, options authCommandOptions) (string, error) {
	p.logger.Info("bot auth login started", "profile", profile.Name)

	credentials, err := p.resolveCredentials(*profile, options, true)
	if err != nil {
		return "", err
	}

	secretKey := config.BotSecretKey(profile.Name)
	if err := p.secrets.Set(secretKey, credentials.appSecret); err != nil {
		return "", err
	}

	profile.Identities.Bot = config.BotIdentity{
		AuthMode:     config.BotAuthModeAppCredentials,
		AppID:        credentials.appID,
		SecretRef:    secretKey,
		ConfiguredAt: time.Now().UTC(),
		Token:        nil,
	}
	p.logger.Info("bot credentials saved", "profile", profile.Name, "source", credentials.source)
	return fmt.Sprintf("Bot credentials saved for profile %q; token exchange not implemented yet.", profile.Name), nil
}

func (p botAuthProvider) Status(_ context.Context, profile config.Profile, options authCommandOptions) (authStatusView, error) {
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

	return authStatusView{
		Authorization: authorization,
		Fields: []authStatusField{
			{Label: "Auth Mode", Value: emptyFallback(profile.Identities.Bot.AuthMode, config.BotAuthModeAppCredentials)},
			{Label: "App ID", Value: emptyFallback(credentials.appID, "<not-configured>")},
			{Label: "App Secret", Value: secretState},
			{Label: "Credential Source", Value: credentials.source},
			{Label: "Token Protocol", Value: "not_implemented"},
		},
	}, nil
}

func (p botAuthProvider) Logout(_ context.Context, profile *config.Profile, _ authCommandOptions) (string, error) {
	p.logger.Info("bot auth logout", "profile", profile.Name)

	if profile.Identities.Bot.SecretRef != "" {
		if err := p.secrets.Delete(profile.Identities.Bot.SecretRef); err != nil {
			return "", err
		}
	}
	profile.Identities.Bot = config.BotIdentity{}
	return fmt.Sprintf("Logged out bot identity for profile %q.", profile.Name), nil
}

func (p botAuthProvider) resolveCredentials(profile config.Profile, options authCommandOptions, requireComplete bool) (botCredentials, error) {
	appID, appIDSource := p.resolveBotAppID(profile, options)
	appSecret, appSecretSource, err := p.resolveBotAppSecret(profile, options)
	if err != nil {
		return botCredentials{}, err
	}

	credentials := botCredentials{
		appID:     appID,
		appSecret: appSecret,
		source:    combineCredentialSources(appIDSource, appSecretSource),
	}
	if requireComplete && (credentials.appID == "" || credentials.appSecret == "") {
		return botCredentials{}, fmt.Errorf("bot app credentials are incomplete; provide --app-id/--app-secret or set %s/%s", envBotAppID, envBotAppSecret)
	}
	return credentials, nil
}

func (p botAuthProvider) resolveBotAppID(profile config.Profile, options authCommandOptions) (string, string) {
	switch {
	case options.AppID != "":
		return options.AppID, "flag"
	default:
		if value, ok := lookupEnvAny(p.lookupEnv, envBotAppID, legacyEnvBotAppID); ok {
			return value, "env"
		}
		if profile.Identities.Bot.AppID != "" {
			return profile.Identities.Bot.AppID, "secrets"
		}
		return "", "missing"
	}
}

func (p botAuthProvider) resolveBotAppSecret(profile config.Profile, options authCommandOptions) (string, string, error) {
	switch {
	case options.AppSecret != "":
		return options.AppSecret, "flag", nil
	default:
		if value, ok := lookupEnvAny(p.lookupEnv, envBotAppSecret, legacyEnvBotAppSecret); ok {
			return value, "env", nil
		}
		if profile.Identities.Bot.SecretRef != "" {
			value, ok, err := p.secrets.Get(profile.Identities.Bot.SecretRef)
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

func defaultLookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func lookupEnvAny(lookup func(string) (string, bool), keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := lookup(key); ok && value != "" {
			return value, true
		}
	}
	return "", false
}
