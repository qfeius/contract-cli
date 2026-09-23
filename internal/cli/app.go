package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/build"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/oauth"
	contractskills "cn.qfei/contract-cli/skills"
)

const defaultProfileName = "contract"

var errAPICommandUnavailable = errors.New("api call 暂未开放使用，请使用已开放的结构化命令")

type Options struct {
	Stdout          io.Writer
	Stderr          io.Writer
	Logger          *slog.Logger
	Store           *config.Store
	Secrets         *config.SecretsStore
	HTTPClient      *http.Client
	OpenBrowser     func(string) error
	SaveFileDialog  func(context.Context, string) (string, error)
	LookupEnv       func(string) (string, bool)
	SkillsFS        fs.FS
	CredentialStore credential.Store

	UpdateRegistryURL    string
	UpdateCurrentVersion string
	Now                  func() time.Time
}

type App struct {
	stdout          io.Writer
	stderr          io.Writer
	logger          *slog.Logger
	store           *config.Store
	secrets         *config.SecretsStore
	httpClient      *http.Client
	openBrowser     func(string) error
	saveFileDialog  func(context.Context, string) (string, error)
	lookupEnv       func(string) (string, bool)
	skillsFS        fs.FS
	credentialStore credential.Store
	updateURL       string
	updateVersion   string
	updateNotice    map[string]any
	now             func() time.Time
	userProvider    authProvider
	appProvider     authProvider
}

type environmentPreset struct {
	OpenPlatformBaseURL            string
	AppTokenEndpoint               string
	ProtectedResourceMetadataURL   string
	AuthorizationServerMetadataURL string
	Resource                       string
	RedirectURL                    string
	Scopes                         []string
	BusinessType                   string
	ClientName                     string
	DeviceClientID                 string
	DeviceScope                    string
}

func New(options Options) *App {
	store := options.Store
	if store == nil {
		dir, err := config.DefaultDir()
		if err != nil {
			panic(err)
		}
		store = config.NewStore(dir)
	}

	secrets := options.Secrets
	if secrets == nil {
		dir, err := config.DefaultDir()
		if err != nil {
			panic(err)
		}
		secrets = config.NewSecretsStore(dir)
	}

	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := options.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{}))
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	httpClient = withProductionNetworkGuard(httpClient, logger)
	opener := options.OpenBrowser
	if opener == nil {
		opener = oauth.OpenBrowser
	}
	saveFileDialog := options.SaveFileDialog
	if saveFileDialog == nil {
		saveFileDialog = defaultSaveFileDialog
	}
	lookupEnv := options.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	skillsFS := options.SkillsFS
	if skillsFS == nil {
		skillsFS = contractskills.FS
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}

	app := &App{
		stdout:          stdout,
		stderr:          stderr,
		logger:          logger,
		store:           store,
		secrets:         secrets,
		httpClient:      httpClient,
		openBrowser:     opener,
		saveFileDialog:  saveFileDialog,
		lookupEnv:       lookupEnv,
		skillsFS:        skillsFS,
		credentialStore: options.CredentialStore,
		updateURL:       options.UpdateRegistryURL,
		updateVersion:   options.UpdateCurrentVersion,
		now:             now,
	}
	app.userProvider = userAuthProvider{
		httpClient:             httpClient,
		logger:                 logger,
		openBrowser:            opener,
		authorizationURLWriter: stdout,
		startCallbackServer: func(redirectURL string) (authorizationCallback, error) {
			return oauth.StartCallbackServer(redirectURL)
		},
	}
	app.appProvider = appAuthProvider{
		httpClient: httpClient,
		logger:     logger,
		secrets:    secrets,
		lookupEnv:  lookupEnv,
	}
	return app
}

/*
Run 执行完整 CLI 参数并兼容审批矩阵交互文档别名。
入参 ctx（context.Context）为运行上下文，args（[]string）为命令参数；返回 error 为执行错误。
*/
func (a *App) Run(ctx context.Context, args []string) error {
	args = normalizeApprovalMatrixArgs(args)
	a.updateNotice = nil
	if len(args) == 0 {
		a.printUsage()
		return nil
	}

	if args[0] == "api" {
		return errAPICommandUnavailable
	}

	if isHelpRequest(args) {
		topic, err := resolveHelpTopic(args)
		if err != nil {
			return err
		}
		return renderHelp(a.stdout, topic)
	}

	switch args[0] {
	case "version", "--version", "-version", "-v":
		a.printVersion()
		return nil
	}

	a.logger.Info("run command", "args", redactCommandArgs(args))
	a.maybePrepareUpdateNotice(ctx, args)

	switch args[0] {
	case "config":
		return a.runConfig(ctx, args[1:])
	case "auth":
		return a.runAuth(ctx, args[1:])
	case "skills":
		return a.runSkills(ctx, args[1:])
	case "update":
		return a.runUpdate(ctx, args[1:])
	case "api":
		return a.runAPI(ctx, args[1:])
	case "contract":
		return a.runContract(ctx, args[1:])
	case "payment":
		return a.runPayment(ctx, args[1:])
	case "mdm":
		return a.runMDM(ctx, args[1:])
	case "event":
		return a.runEvent(ctx, args[1:])
	case "rule":
		return a.runRule(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a *App) printUsage() {
	_ = renderHelp(a.stdout, helpRegistry()["contract-cli"])
}

func (a *App) printVersion() {
	_, _ = fmt.Fprintln(a.stdout, build.Current().String())
}

func (a *App) updateCachePath() string {
	return filepath.Join(filepath.Dir(a.store.Path()), "update-check.json")
}

func (a *App) runConfig(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("missing config subcommand")
	}

	switch args[0] {
	case "add":
		return a.runConfigAdd(ctx, args[1:])
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

/*
runConfigAdd 按指定环境发现并保存配置，切换环境时清理原有认证以免串用。
入参 ctx（context.Context）为请求上下文，args（[]string）为命令参数；返回 error。
*/
func (a *App) runConfigAdd(ctx context.Context, args []string) error {
	a.logger.Info("config add started")

	flags := flag.NewFlagSet("config add", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var env string
	var profileName string
	var protectedResourceURL string
	var redirectURL string
	var scopes string

	flags.StringVar(&env, "env", "prod", "environment preset")
	flags.StringVar(&profileName, "name", defaultProfileName, "profile name")
	flags.StringVar(&protectedResourceURL, "resource-metadata-url", "", "override protected resource metadata URL")
	flags.StringVar(&redirectURL, "redirect-url", "", "OAuth redirect URL")
	flags.StringVar(&scopes, "scope", "", "space-separated OAuth scopes")

	if err := flags.Parse(args); err != nil {
		return err
	}

	preset, err := resolveEnvironment(env)
	if err != nil {
		a.logger.Error("resolve environment failed", "environment", env, "error", err.Error())
		return err
	}

	existing, found, err := a.store.LookupProfile(profileName)
	if err != nil {
		return err
	}
	resetAuthentication := false
	if found {
		resetAuthentication, err = a.productionProfileRequiresReset(existing)
		if err != nil {
			return err
		}
		// 同名配置跨环境切换必须重新认证；独立 profile 不受影响。
		resetAuthentication = resetAuthentication || existing.Environment != env
	}

	if protectedResourceURL == "" {
		protectedResourceURL = preset.ProtectedResourceMetadataURL
	}
	openOrigin, _, _ := environmentOrigins(env)
	if protectedResourceURL != "" && !isProductionOriginURL(protectedResourceURL, openOrigin, true) {
		err := productionProfileError(profileName)
		a.logger.Error("config add rejected non-production metadata url", "profile", profileName, "error", err.Error())
		return err
	}
	if redirectURL == "" {
		redirectURL = preset.RedirectURL
	}
	scopeList := preset.Scopes
	if scopes != "" {
		scopeList = strings.Fields(scopes)
	}

	var discovery *oauth.DiscoveryResult
	switch {
	case protectedResourceURL != "":
		discovery, err = oauth.Discover(ctx, a.httpClient, a.logger, protectedResourceURL)
		if err != nil {
			a.logger.Error("config add discover failed", "profile", profileName, "protected_resource_url", protectedResourceURL, "error", err.Error())
			return err
		}
	case preset.AuthorizationServerMetadataURL != "":
		discovery, err = oauth.DiscoverFromAuthorizationServer(ctx, a.httpClient, a.logger, preset.AuthorizationServerMetadataURL, preset.Resource)
		if err != nil {
			a.logger.Error("config add discover from authorization server failed", "profile", profileName, "authorization_server_metadata_url", preset.AuthorizationServerMetadataURL, "error", err.Error())
			return err
		}
	default:
		return fmt.Errorf("environment %q is missing oauth discovery defaults", env)
	}

	profile := config.Profile{
		Name:                           profileName,
		Environment:                    env,
		OpenPlatformBaseURL:            preset.OpenPlatformBaseURL,
		AppTokenEndpoint:               preset.AppTokenEndpoint,
		ProtectedResourceMetadataURL:   protectedResourceURL,
		AuthorizationServerMetadataURL: discovery.AuthorizationServerMetadataURL,
		Resource:                       discovery.ProtectedResource.Resource,
		Scopes:                         scopeList,
		BusinessType:                   preset.BusinessType,
		ClientName:                     preset.ClientName,
		DefaultIdentity:                config.IdentityUser,
	}
	if found && !resetAuthentication {
		profile.Identities = existing.Identities
		profile.DefaultIdentity = defaultIdentity(existing)
	}

	profile.Identities.User.AuthorizationEndpoint = discovery.AuthorizationServer.AuthorizationEndpoint
	profile.Identities.User.DeviceAuthorizationEndpoint = discovery.AuthorizationServer.DeviceAuthorizationEndpoint
	profile.Identities.User.TokenEndpoint = discovery.AuthorizationServer.TokenEndpoint
	profile.Identities.User.RevocationEndpoint = discovery.AuthorizationServer.RevocationEndpoint
	profile.Identities.User.RegistrationEndpoint = discovery.AuthorizationServer.RegistrationEndpoint
	profile.Identities.User.DeviceClientID = preset.DeviceClientID
	profile.Identities.User.DeviceScope = preset.DeviceScope
	profile.Identities.User.RedirectURL = redirectURL
	if err := validateProductionProfile(profile); err != nil {
		a.logger.Error("reject discovered non-production profile", "profile", profileName, "error", err.Error())
		return err
	}
	if resetAuthentication {
		if err := a.clearProfileAuthenticationState(profileName); err != nil {
			return fmt.Errorf("clear non-production authentication state for profile %q: %w", profileName, err)
		}
	}

	a.logger.Info("save profile", "profile", profileName, "environment", env)
	if err := a.store.UpsertProfile(profile, true); err != nil {
		a.logger.Error("save profile failed", "profile", profileName, "error", err.Error())
		return err
	}

	_, _ = fmt.Fprintf(a.stdout, "Profile %q saved for %s.\n", profileName, env)
	return nil
}

func (a *App) runAuth(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("missing auth subcommand")
	}

	switch args[0] {
	case "login":
		return a.runAuthLogin(ctx, args[1:])
	case "init":
		return a.runAuthDeviceInit(ctx, args[1:])
	case "complete":
		return a.runAuthDeviceComplete(ctx, args[1:])
	case "status":
		return a.runAuthStatus(ctx, args[1:])
	case "logout":
		return a.runAuthLogout(ctx, args[1:])
	case "use":
		return a.runAuthUse(args[1:])
	default:
		return fmt.Errorf("unknown auth subcommand %q", args[0])
	}
}

func (a *App) runAuthLogin(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("auth login", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var profileName string
	var timeout time.Duration
	var noOpenBrowser bool
	var as string
	var appID string
	var appSecret string

	flags.StringVar(&profileName, "profile", "", "profile name")
	flags.DurationVar(&timeout, "timeout", 3*time.Minute, "authorization timeout")
	flags.BoolVar(&noOpenBrowser, "no-open-browser", false, "print authorization URL without auto-opening the browser")
	flags.StringVar(&as, "as", string(config.IdentityUser), "identity to use: user|app")
	flags.StringVar(&appID, "app-id", "", "app id")
	flags.StringVar(&appSecret, "app-secret", "", "app secret")

	if err := flags.Parse(args); err != nil {
		return err
	}

	identity, err := config.ParseIdentityKind(as)
	if err != nil {
		return err
	}
	profile, err := a.loadDeviceAwareProfile(profileName)
	if err != nil {
		return err
	}

	a.logger.Info("auth login started", "profile", profile.Name, "identity", identity)
	message, err := a.providerFor(identity).Login(ctx, &profile, authCommandOptions{
		Identity:      identity,
		ProfileName:   profile.Name,
		Timeout:       timeout,
		NoOpenBrowser: noOpenBrowser,
		AppID:         appID,
		AppSecret:     appSecret,
	})
	if err != nil {
		if identity == config.IdentityApp {
			if saveErr := a.store.SaveProfile(profile); saveErr != nil {
				a.logger.Error("auth login failed while saving app profile", "profile", profile.Name, "identity", identity, "error", saveErr.Error())
				return saveErr
			}
		}
		a.logger.Error("auth login failed", "profile", profile.Name, "identity", identity, "error", err.Error())
		return err
	}

	profile.DefaultIdentity = identity
	if err := a.store.SaveProfile(profile); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(a.stdout, message)
	return nil
}

func (a *App) runAuthStatus(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("auth status", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var profileName string
	var as string
	flags.StringVar(&profileName, "profile", "", "profile name")
	flags.StringVar(&as, "as", string(config.IdentityUser), "identity to inspect: user|app")

	if err := flags.Parse(args); err != nil {
		return err
	}

	identity, err := config.ParseIdentityKind(as)
	if err != nil {
		return err
	}
	profile, err := a.loadDeviceAwareProfile(profileName)
	if err != nil {
		return err
	}

	var view authStatusView
	if identity == config.IdentityUser && profile.Identities.User.AuthMode == config.UserAuthModeDevice {
		view, err = a.deviceAuthStatus(profile)
	} else {
		view, err = a.providerFor(identity).Status(ctx, profile, authCommandOptions{
			Identity:    identity,
			ProfileName: profile.Name,
		})
	}
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(a.stdout, "Profile: %s\n", profile.Name)
	_, _ = fmt.Fprintf(a.stdout, "Environment: %s\n", profile.Environment)
	_, _ = fmt.Fprintf(a.stdout, "Default Identity: %s\n", defaultIdentity(profile))
	_, _ = fmt.Fprintf(a.stdout, "Identity: %s\n", identity)
	for _, field := range view.Fields {
		_, _ = fmt.Fprintf(a.stdout, "%s: %s\n", field.Label, field.Value)
	}
	_, _ = fmt.Fprintf(a.stdout, "Authorization: %s\n", view.Authorization)
	return nil
}

func (a *App) runAuthLogout(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("auth logout", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var profileName string
	var as string
	flags.StringVar(&profileName, "profile", "", "profile name")
	flags.StringVar(&as, "as", string(config.IdentityUser), "identity to logout: user|app")

	if err := flags.Parse(args); err != nil {
		return err
	}

	identity, err := config.ParseIdentityKind(as)
	if err != nil {
		return err
	}
	profile, err := a.loadDeviceAwareProfile(profileName)
	if err != nil {
		return err
	}

	var message string
	if identity == config.IdentityUser && profile.Identities.User.AuthMode == config.UserAuthModeDevice {
		message, err = a.deviceAuthLogout(ctx, profile)
	} else {
		message, err = a.providerFor(identity).Logout(ctx, &profile, authCommandOptions{
			Identity:    identity,
			ProfileName: profile.Name,
		})
	}
	if err != nil {
		return err
	}
	if err := a.store.SaveProfile(profile); err != nil {
		return err
	}
	a.logger.Info("auth logout completed", "profile", profile.Name, "identity", identity)
	_, _ = fmt.Fprintln(a.stdout, message)
	return nil
}

func (a *App) runAuthUse(args []string) error {
	flags := flag.NewFlagSet("auth use", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var profileName string
	var as string
	flags.StringVar(&profileName, "profile", "", "profile name")
	flags.StringVar(&as, "as", string(config.IdentityUser), "default business identity: user|app")

	if err := flags.Parse(args); err != nil {
		return err
	}

	identity, err := config.ParseIdentityKind(as)
	if err != nil {
		return err
	}
	profile, err := a.loadDeviceAwareProfile(profileName)
	if err != nil {
		return err
	}
	profile.DefaultIdentity = identity
	if err := a.store.SaveProfile(profile); err != nil {
		return err
	}

	a.logger.Info("default identity switched", "profile", profile.Name, "identity", identity)
	_, _ = fmt.Fprintf(a.stdout, "Default identity for profile %q set to %s.\n", profile.Name, identity)
	return nil
}

func (a *App) providerFor(identity config.IdentityKind) authProvider {
	switch identity {
	case config.IdentityApp:
		return a.appProvider
	case config.IdentityUser:
		fallthrough
	default:
		return a.userProvider
	}
}

/*
resolveEnvironment 只提供线上 prod 环境预设，非生产环境在任何网络请求前被拒绝。
入参 name（string）为环境名；返回 environmentPreset 和 error。
*/
func resolveEnvironment(name string) (environmentPreset, error) {
	switch name {
	case "prod":
		return environmentPreset{
			OpenPlatformBaseURL:            "https://open.qfei.cn",
			AppTokenEndpoint:               "https://open.qfei.cn/open-apis/auth/v3/tenant_access_token/internal",
			ProtectedResourceMetadataURL:   "",
			AuthorizationServerMetadataURL: "https://myaccount.qfei.cn/.well-known/oauth-authorization-server/contract",
			Resource:                       "https://open.qfei.cn",
			RedirectURL:                    "http://127.0.0.1:8000/callback",
			Scopes:                         []string{"cli:tools", "cli:resources"},
			BusinessType:                   "contract",
			ClientName:                     "contract-cli",
			DeviceClientID:                 "zscli_892efdadc11a3f53",
			DeviceScope:                    "contract:full contract-review:full",
		}, nil
	default:
		return environmentPreset{}, fmt.Errorf("unsupported environment %q; supported environments: prod", name)
	}
}

func defaultIdentity(profile config.Profile) config.IdentityKind {
	if profile.DefaultIdentity == "" {
		return config.IdentityUser
	}
	return profile.DefaultIdentity
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
