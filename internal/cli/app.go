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

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
	"cn.qfei/contract-cli/internal/invocation"
	"cn.qfei/contract-cli/internal/oauth"
	contractskills "cn.qfei/contract-cli/skills"
)

const defaultProfileName = "contract"

type Options struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Logger 显式注入时由调用方控制格式和级别；CLI 默认日志由 --debug 管理。
	Logger             *slog.Logger
	Store              *config.Store
	Secrets            *config.SecretsStore
	HTTPClient         *http.Client
	OpenBrowser        func(string) error
	SaveFileDialog     func(context.Context, string) (string, error)
	LookupEnv          func(string) (string, bool)
	SkillsFS           fs.FS
	CredentialStore    credential.Store
	CredentialKeyring  credential.Keyring
	LocalUserHome      func() (string, error)
	InspectEnvironment func(context.Context, int) invocation.Result

	UpdateRegistryURL    string
	UpdateCurrentVersion string
	Now                  func() time.Time
}

type App struct {
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
	logger             *slog.Logger
	logPolicy          *commandLogPolicy
	store              *config.Store
	secrets            *config.SecretsStore
	httpClient         *http.Client
	fileHTTPClient     *http.Client
	openBrowser        func(string) error
	saveFileDialog     func(context.Context, string) (string, error)
	lookupEnv          func(string) (string, bool)
	skillsFS           fs.FS
	credentialStore    credential.Store
	credentialKeyring  credential.Keyring
	localUserHome      func() (string, error)
	runtimeEvidence    *invocation.Result
	commandContext     context.Context
	inspectEnvironment func(context.Context, int) invocation.Result
	updateURL          string
	updateVersion      string
	updateNotice       map[string]any
	now                func() time.Time
	userProvider       authProvider
	appProvider        authProvider
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

	stdin := options.Stdin
	if stdin == nil {
		stdin = os.Stdin
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
	var logPolicy *commandLogPolicy
	if logger == nil {
		logger, logPolicy = newCommandLogger(stderr)
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	fileHTTPClient := httpClient
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
	inspectEnvironment := options.InspectEnvironment
	if inspectEnvironment == nil {
		inspectEnvironment = invocation.Inspect
	}
	localUserHome := options.LocalUserHome
	if localUserHome == nil {
		localUserHome = currentOSUserHome
	}

	app := &App{
		stdin:              stdin,
		stdout:             stdout,
		stderr:             stderr,
		logger:             logger,
		logPolicy:          logPolicy,
		store:              store,
		secrets:            secrets,
		httpClient:         httpClient,
		fileHTTPClient:     fileHTTPClient,
		openBrowser:        opener,
		saveFileDialog:     saveFileDialog,
		lookupEnv:          lookupEnv,
		skillsFS:           skillsFS,
		credentialStore:    options.CredentialStore,
		credentialKeyring:  options.CredentialKeyring,
		localUserHome:      localUserHome,
		inspectEnvironment: inspectEnvironment,
		updateURL:          options.UpdateRegistryURL,
		updateVersion:      options.UpdateCurrentVersion,
		now:                now,
	}
	app.userProvider = userAuthProvider{
		httpClient:              httpClient,
		logger:                  logger,
		openBrowser:             opener,
		authorizationURLWriter:  stdout,
		authorizationJSONWriter: stderr,
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
	a.updateNotice = nil
	if err := ctx.Err(); err != nil {
		return err
	}
	args, logOptions, err := parseCommandLogOptions(args)
	if err != nil {
		return err
	}
	logOptions.outputJSON = commandRequestsJSON(args, logOptions)
	if logOptions.outputJSON {
		logOptions.json = true
	}
	if a.logPolicy != nil {
		previous := a.logPolicy.current.Swap(&logOptions)
		defer a.logPolicy.current.Store(previous)
	}
	args = normalizeApprovalMatrixArgs(args)
	previousContext := a.commandContext
	defer func() { a.commandContext = previousContext }()
	a.commandContext = ctx
	a.runtimeEvidence = nil
	if len(args) > 0 && args[0] == "__internal" {
		return withCommandCancellation(ctx, a.runSkillSyncOnInstall(args[1:]))
	}
	a.warnPendingSkillSync(logOptions.outputJSON)
	if len(args) == 0 {
		a.printUsage()
		return nil
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
		return withCommandCancellation(ctx, a.runVersion(ctx, args[1:]))
	}

	if isBusinessCommand(args[0]) {
		return a.runWithCommandTrace(ctx, args)
	}
	return withCommandCancellation(ctx, a.runCommand(ctx, args))
}

func (a *App) runCommand(ctx context.Context, args []string) error {
	a.commandTraceLogger(ctx).Info("run command", "args", redactCommandArgs(args))
	// 本地来源诊断不访问更新服务，也不读取或刷新更新缓存。
	if args[0] == "environment" {
		return a.runEnvironment(ctx, args[1:])
	}
	if err := a.maybePrepareUpdateNotice(ctx, args); err != nil {
		return err
	}
	// 更新检查可失败降级；命令本身已取消时，不再开始后续认证或本地写入。
	if err := ctx.Err(); err != nil {
		return err
	}

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
	case "employee", "department":
		return a.runDirectory(ctx, args[0], args[1:])
	case "review":
		return a.runReview(ctx, args[1:])
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
func (a *App) runConfigAdd(ctx context.Context, args []string) (err error) {
	var outcome *localCommandOutcome
	defer func() { err = withCompletedLocalCommand(ctx, err, outcome) }()
	a.logger.Info("config add started")

	flags := flag.NewFlagSet("config add", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	outputFormat := registerJSONOutputFlag(flags)

	var env string
	var profileName string
	var protectedResourceURL string
	var redirectURL string
	var scopes string
	var openPlatformURL, authorizationMetadataURL string

	flags.StringVar(&env, "env", "prod", "environment preset")
	flags.StringVar(&profileName, "name", defaultProfileName, "profile name")
	flags.StringVar(&protectedResourceURL, "resource-metadata-url", "", "override protected resource metadata URL")
	flags.StringVar(&redirectURL, "redirect-url", "", "OAuth redirect URL")
	flags.StringVar(&scopes, "scope", "", "space-separated OAuth scopes")
	flags.StringVar(&openPlatformURL, "open-platform-base-url", "", "override API base URL and Resource")
	flags.StringVar(&authorizationMetadataURL, "authorization-server-metadata-url", "", "override trusted authorization metadata URL")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("config add does not accept positional arguments")
	}
	jsonOutput, err := parseJSONOutputFormat(*outputFormat)
	if err != nil {
		return err
	}

	preset, err := resolveEnvironment(env)
	if err != nil {
		a.logger.Error("resolve environment failed", "environment", env, "error", err.Error())
		return err
	}
	if openPlatformURL != "" {
		preset.OpenPlatformBaseURL = strings.TrimRight(strings.TrimSpace(openPlatformURL), "/")
		preset.Resource = preset.OpenPlatformBaseURL
		preset.AppTokenEndpoint = preset.OpenPlatformBaseURL + "/open-apis/auth/v3/tenant_access_token/internal"
	}
	if authorizationMetadataURL != "" {
		preset.AuthorizationServerMetadataURL = strings.TrimSpace(authorizationMetadataURL)
	}
	openOrigin, accountOrigin, valid := profileOrigins(config.Profile{Environment: env,
		OpenPlatformBaseURL: preset.OpenPlatformBaseURL, AuthorizationServerMetadataURL: preset.AuthorizationServerMetadataURL})
	if !valid || !isExactEnvironmentResource(preset.OpenPlatformBaseURL, openOrigin) {
		a.logger.Error("config add rejected invalid target addresses", "profile", profileName)
		return productionProfileError(profileName)
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
	if protectedResourceURL != "" && !isProductionOriginURL(protectedResourceURL, openOrigin, true) {
		err := productionProfileError(profileName)
		a.logger.Error("config add rejected mismatched metadata url", "profile", profileName, "error", err.Error())
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
		discovery, err = oauth.Discover(ctx, a.httpClient, a.logger, protectedResourceURL, preset.AuthorizationServerMetadataURL)
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
	if err := ctx.Err(); err != nil {
		return err
	}
	if !isProductionOriginURL(discovery.AuthorizationServerMetadataURL, accountOrigin, true) {
		return productionProfileError(profileName)
	}

	profile := config.Profile{
		Name:                           profileName,
		Environment:                    env,
		OpenPlatformBaseURL:            preset.OpenPlatformBaseURL,
		AppTokenEndpoint:               preset.AppTokenEndpoint,
		ProtectedResourceMetadataURL:   protectedResourceURL,
		AuthorizationServerMetadataURL: preset.AuthorizationServerMetadataURL,
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
	if found {
		resetAuthentication = resetAuthentication || config.CredentialTargetBinding(existing) != config.CredentialTargetBinding(profile)
	}
	if err := validateProductionProfile(profile); err != nil {
		a.logger.Error("reject inconsistent discovered profile", "profile", profileName, "error", err.Error())
		return err
	}
	a.logger.Info("save profile", "profile", profileName, "environment", env)
	if err := ctx.Err(); err != nil {
		return err
	}
	if resetAuthentication {
		profile.Identities.User.Token = nil
		profile.Identities.User.ClientID = ""
		profile.Identities.User.AuthMode = ""
		profile.Identities.App = config.AppIdentity{}
		profile.DefaultIdentity = config.IdentityUser
		if err := a.clearProfileAuthenticationState(ctx, profileName); err != nil {
			return fmt.Errorf("clear authentication state for changed profile %q: %w", profileName, err)
		}
		outcome = &localCommandOutcome{Operation: "config add", Result: map[string]any{"profile": profileName, "authentication_reset": true, "profile_saved": false}}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := a.store.UpsertProfile(profile, true); err != nil {
		a.logger.Error("save profile failed", "profile", profileName, "error", err.Error())
		return err
	}
	outcome = &localCommandOutcome{Operation: "config add", Result: map[string]any{"profile": profileName, "environment": env, "authentication_reset": resetAuthentication, "profile_saved": true}}
	if err := ctx.Err(); err != nil {
		return err
	}

	if jsonOutput {
		return a.renderJSONCommandResult(map[string]any{
			"profile": profile.Name, "environment": profile.Environment,
			"default_identity": defaultIdentity(profile), "resource": profile.Resource,
			"authentication_reset": resetAuthentication,
		})
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
		return a.runAuthUse(ctx, args[1:])
	default:
		return fmt.Errorf("unknown auth subcommand %q", args[0])
	}
}

func (a *App) runAuthLogin(ctx context.Context, args []string) (err error) {
	var outcome *localCommandOutcome
	defer func() { err = withCompletedLocalCommand(ctx, err, outcome) }()
	flags := flag.NewFlagSet("auth login", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	outputFormat := registerJSONOutputFlag(flags)

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
	if flags.NArg() != 0 {
		return fmt.Errorf("auth login does not accept positional arguments")
	}
	jsonOutput, err := parseJSONOutputFormat(*outputFormat)
	if err != nil {
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
	if err := ctx.Err(); err != nil {
		return err
	}
	message, err := a.providerFor(identity).Login(ctx, &profile, authCommandOptions{
		Identity:      identity,
		ProfileName:   profile.Name,
		Timeout:       timeout,
		NoOpenBrowser: noOpenBrowser,
		AppID:         appID,
		AppSecret:     appSecret,
		JSONOutput:    jsonOutput,
	})
	var committed *appCredentialsCommittedError
	credentialsSaved := identity == config.IdentityApp && (err == nil || errors.As(err, &committed))
	if credentialsSaved {
		outcome = &localCommandOutcome{Operation: "auth login", Result: map[string]any{"profile": profile.Name, "identity": identity, "credentials_saved": true, "profile_saved": false}}
	}
	if err != nil {
		if identity == config.IdentityApp && (credentialsSaved || ctx.Err() == nil) {
			// A successful Secret write starts the local credential commit.
			// Finish its matching profile even if cancellation arrived meanwhile.
			if saveErr := a.store.SaveProfile(profile); saveErr != nil {
				a.logger.Error("auth login failed while saving app profile", "profile", profile.Name, "identity", identity, "error", saveErr.Error())
				return saveErr
			}
			outcome = &localCommandOutcome{Operation: "auth login", Result: map[string]any{"profile": profile.Name, "identity": identity, "credentials_saved": credentialsSaved, "profile_saved": true, "default_identity": defaultIdentity(profile)}}
		}
		a.logger.Error("auth login failed", "profile", profile.Name, "identity", identity, "error", err.Error())
		return withCommandCancellation(ctx, err)
	}

	if !credentialsSaved {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if ctx.Err() == nil {
		profile.DefaultIdentity = identity
	}
	if err := a.store.SaveProfile(profile); err != nil {
		return err
	}
	outcome = &localCommandOutcome{Operation: "auth login", Result: map[string]any{"profile": profile.Name, "identity": identity, "credentials_saved": credentialsSaved, "profile_saved": true, "default_identity": defaultIdentity(profile)}}
	if err := ctx.Err(); err != nil {
		return err
	}

	if jsonOutput {
		return a.renderJSONCommandResult(authCommandJSONResult(profile, identity, authStatusView{Authorization: "authorized"}, message))
	}
	_, _ = fmt.Fprintln(a.stdout, message)
	return nil
}

func (a *App) runAuthStatus(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("auth status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	outputFormat := registerJSONOutputFlag(flags)

	var profileName string
	var as string
	flags.StringVar(&profileName, "profile", "", "profile name")
	flags.StringVar(&as, "as", string(config.IdentityUser), "identity to inspect: user|app")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("auth status does not accept positional arguments")
	}
	jsonOutput, err := parseJSONOutputFormat(*outputFormat)
	if err != nil {
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

	if jsonOutput {
		return a.renderJSONCommandResult(authCommandJSONResult(profile, identity, view, ""))
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

func (a *App) runAuthLogout(ctx context.Context, args []string) (err error) {
	var outcome *localCommandOutcome
	defer func() { err = withCompletedLocalCommand(ctx, err, outcome) }()
	flags := flag.NewFlagSet("auth logout", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	outputFormat := registerJSONOutputFlag(flags)

	var profileName string
	var as string
	flags.StringVar(&profileName, "profile", "", "profile name")
	flags.StringVar(&as, "as", string(config.IdentityUser), "identity to logout: user|app")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("auth logout does not accept positional arguments")
	}
	jsonOutput, err := parseJSONOutputFormat(*outputFormat)
	if err != nil {
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
		if err == nil {
			outcome = &localCommandOutcome{Operation: "auth logout", Result: map[string]any{"profile": profile.Name, "identity": identity, "device_credentials_cleared": true}}
		}
	} else {
		message, err = a.providerFor(identity).Logout(ctx, &profile, authCommandOptions{
			Identity:    identity,
			ProfileName: profile.Name,
		})
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := a.store.SaveProfile(profile); err != nil {
		return err
	}
	outcome = &localCommandOutcome{Operation: "auth logout", Result: map[string]any{"profile": profile.Name, "identity": identity, "token_cleared": true, "profile_saved": true}}
	a.logger.Info("auth logout completed", "profile", profile.Name, "identity", identity)
	if err := ctx.Err(); err != nil {
		return err
	}
	if jsonOutput {
		return a.renderJSONCommandResult(authCommandJSONResult(profile, identity, authStatusView{}, message))
	}
	_, _ = fmt.Fprintln(a.stdout, message)
	return nil
}

func (a *App) runAuthUse(ctx context.Context, args []string) (err error) {
	var outcome *localCommandOutcome
	defer func() { err = withCompletedLocalCommand(ctx, err, outcome) }()
	flags := flag.NewFlagSet("auth use", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	outputFormat := registerJSONOutputFlag(flags)

	var profileName string
	var as string
	flags.StringVar(&profileName, "profile", "", "profile name")
	flags.StringVar(&as, "as", string(config.IdentityUser), "default business identity: user|app")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("auth use does not accept positional arguments")
	}
	jsonOutput, err := parseJSONOutputFormat(*outputFormat)
	if err != nil {
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
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := a.store.SaveProfile(profile); err != nil {
		return err
	}
	outcome = &localCommandOutcome{Operation: "auth use", Result: map[string]any{"profile": profile.Name, "default_identity": identity, "profile_saved": true}}

	a.logger.Info("default identity switched", "profile", profile.Name, "identity", identity)
	if err := ctx.Err(); err != nil {
		return err
	}
	if jsonOutput {
		message := fmt.Sprintf("Default identity for profile %q set to %s.", profile.Name, identity)
		return a.renderJSONCommandResult(authCommandJSONResult(profile, identity, authStatusView{}, message))
	}
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

func resolveEnvironment(name string) (environmentPreset, error) {
	if name == "test" {
		preset, err := resolveEnvironment(productionEnvironment)
		if err != nil {
			return environmentPreset{}, err
		}
		preset.OpenPlatformBaseURL = "https://test-open.qtech.cn"
		preset.Resource = preset.OpenPlatformBaseURL
		preset.AppTokenEndpoint = preset.OpenPlatformBaseURL + "/open-apis/auth/v3/tenant_access_token/internal"
		preset.AuthorizationServerMetadataURL = "https://test-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract"
		return preset, nil
	}
	switch name {
	case productionEnvironment:
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
		return environmentPreset{}, fmt.Errorf("unsupported environment %q; supported environments: prod, test", name)
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
