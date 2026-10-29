package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	envConfigDir       = "CONTRACT_CLI_CONFIG_DIR"
	legacyEnvConfigDir = "DEMOCLI_CONFIG_DIR"
	defaultClientName  = "contract-cli"
	legacyClientName   = "democli"
)

type IdentityKind string

const (
	IdentityUser IdentityKind = "user"
	IdentityApp  IdentityKind = "app"

	legacyIdentityBot IdentityKind = "bot"
)

const AppAuthModeAppCredentials = "app_credentials"

const (
	UserAuthModeAuthorizationCode = "authorization_code"
	UserAuthModeDevice            = "device"
)

type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

type UserIdentity struct {
	AuthMode                    string `json:"auth_mode,omitempty"`
	ClientID                    string `json:"client_id,omitempty"`
	DeviceClientID              string `json:"device_client_id,omitempty"`
	DeviceScope                 string `json:"device_scope,omitempty"`
	AuthorizationEndpoint       string `json:"authorization_endpoint,omitempty"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint,omitempty"`
	TokenEndpoint               string `json:"token_endpoint,omitempty"`
	RevocationEndpoint          string `json:"revocation_endpoint,omitempty"`
	RegistrationEndpoint        string `json:"registration_endpoint,omitempty"`
	RedirectURL                 string `json:"redirect_url,omitempty"`
	Token                       *Token `json:"token,omitempty"`
}

type AppIdentity struct {
	AuthMode     string    `json:"auth_mode,omitempty"`
	AppID        string    `json:"app_id,omitempty"`
	SecretRef    string    `json:"secret_ref,omitempty"`
	ConfiguredAt time.Time `json:"configured_at,omitempty"`
	Token        *Token    `json:"token,omitempty"`
}

type Identities struct {
	User UserIdentity `json:"user"`
	App  AppIdentity  `json:"app"`
}

type Profile struct {
	Name                           string       `json:"name"`
	Environment                    string       `json:"environment"`
	OpenPlatformBaseURL            string       `json:"open_platform_base_url,omitempty"`
	AppTokenEndpoint               string       `json:"app_token_endpoint,omitempty"`
	ProtectedResourceMetadataURL   string       `json:"protected_resource_metadata_url"`
	AuthorizationServerMetadataURL string       `json:"authorization_server_metadata_url"`
	Resource                       string       `json:"resource"`
	Scopes                         []string     `json:"scopes"`
	BusinessType                   string       `json:"business_type"`
	ClientName                     string       `json:"client_name"`
	DefaultIdentity                IdentityKind `json:"default_identity,omitempty"`
	CredentialBinding              string       `json:"credential_binding,omitempty"`
	Identities                     Identities   `json:"identities"`
}

type File struct {
	CurrentProfile string             `json:"current_profile,omitempty"`
	Profiles       map[string]Profile `json:"profiles"`
}

type Store struct {
	dir string
}

type rawFile struct {
	CurrentProfile string                     `json:"current_profile,omitempty"`
	Profiles       map[string]json.RawMessage `json:"profiles"`
}

type rawProfile struct {
	Name                           string        `json:"name"`
	Environment                    string        `json:"environment"`
	OpenPlatformBaseURL            string        `json:"open_platform_base_url,omitempty"`
	AppTokenEndpoint               string        `json:"app_token_endpoint,omitempty"`
	LegacyBotTokenEndpoint         string        `json:"bot_token_endpoint,omitempty"`
	ProtectedResourceMetadataURL   string        `json:"protected_resource_metadata_url"`
	AuthorizationServerMetadataURL string        `json:"authorization_server_metadata_url"`
	Resource                       string        `json:"resource"`
	Scopes                         []string      `json:"scopes"`
	BusinessType                   string        `json:"business_type"`
	ClientName                     string        `json:"client_name"`
	DefaultIdentity                IdentityKind  `json:"default_identity,omitempty"`
	CredentialBinding              string        `json:"credential_binding,omitempty"`
	Identities                     rawIdentities `json:"identities"`

	// Legacy flat user-auth fields kept for backward-compatible reads.
	AuthorizationEndpoint string `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string `json:"token_endpoint,omitempty"`
	RegistrationEndpoint  string `json:"registration_endpoint,omitempty"`
	RedirectURL           string `json:"redirect_url,omitempty"`
	ClientID              string `json:"client_id,omitempty"`
	Token                 *Token `json:"token,omitempty"`
}

type rawIdentities struct {
	User UserIdentity `json:"user"`
	App  AppIdentity  `json:"app"`
	Bot  AppIdentity  `json:"bot"`
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func DefaultDir() (string, error) {
	if dir := os.Getenv(envConfigDir); dir != "" {
		return dir, nil
	}
	if dir := os.Getenv(legacyEnvConfigDir); dir != "" {
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}

	defaultDir := filepath.Join(home, ".contract-cli")
	legacyDir := filepath.Join(home, ".democli")
	if _, err := os.Stat(defaultDir); err == nil {
		return defaultDir, nil
	}
	if _, err := os.Stat(legacyDir); err == nil {
		return legacyDir, nil
	}
	return defaultDir, nil
}

func ParseIdentityKind(value string) (IdentityKind, error) {
	switch IdentityKind(strings.ToLower(strings.TrimSpace(value))) {
	case "", IdentityUser:
		return IdentityUser, nil
	case IdentityApp, legacyIdentityBot:
		return IdentityApp, nil
	default:
		return "", fmt.Errorf("unsupported identity %q", value)
	}
}

func (s *Store) Path() string {
	return filepath.Join(s.dir, "config.json")
}

func (s *Store) Load() (*File, error) {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{Profiles: map[string]Profile{}}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var raw rawFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	cfg := &File{
		CurrentProfile: raw.CurrentProfile,
		Profiles:       map[string]Profile{},
	}
	for name, rawProfileData := range raw.Profiles {
		profile, err := decodeProfile(rawProfileData)
		if err != nil {
			return nil, fmt.Errorf("decode profile %q: %w", name, err)
		}
		cfg.Profiles[name] = profile
	}
	return cfg, nil
}

func (s *Store) Save(cfg *File) error {
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	for name, profile := range cfg.Profiles {
		profile.ensureDefaults()
		cfg.Profiles[name] = profile
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.Path(), data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (s *Store) UpsertProfile(profile Profile, makeCurrent bool) error {
	cfg, err := s.Load()
	if err != nil {
		return err
	}

	profile.ensureDefaults()
	// Programmatic credential creation may establish a first binding. Loading a
	// file never does so, and subsequent saves never rebind changed addresses.
	if profile.CredentialBinding == "" && hasStoredCredentials(profile) {
		profile.CredentialBinding = CredentialTargetBinding(profile)
	}
	cfg.Profiles[profile.Name] = profile
	if makeCurrent || cfg.CurrentProfile == "" {
		cfg.CurrentProfile = profile.Name
	}
	return s.Save(cfg)
}

func (s *Store) LookupProfile(name string) (Profile, bool, error) {
	cfg, err := s.Load()
	if err != nil {
		return Profile{}, false, err
	}

	if name == "" {
		name = cfg.CurrentProfile
	}
	profile, ok := cfg.Profiles[name]
	if !ok {
		return Profile{}, false, nil
	}
	profile.ensureDefaults()
	return profile, true, nil
}

func (s *Store) GetProfile(name string) (Profile, error) {
	profile, ok, err := s.LookupProfile(name)
	if err != nil {
		return Profile{}, err
	}
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found", name)
	}
	return profile, nil
}

func (s *Store) SaveProfile(profile Profile) error {
	return s.UpsertProfile(profile, false)
}

func (s *Store) SaveToken(profileName string, identity IdentityKind, token *Token) error {
	cfg, err := s.Load()
	if err != nil {
		return err
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}
	profile.ensureDefaults()
	switch identity {
	case IdentityUser:
		profile.Identities.User.Token = token
	case IdentityApp:
		profile.Identities.App.Token = token
	case legacyIdentityBot:
		return fmt.Errorf("unsupported identity %q", identity)
	default:
		return fmt.Errorf("unsupported identity %q", identity)
	}
	if token != nil && profile.CredentialBinding == "" {
		profile.CredentialBinding = CredentialTargetBinding(profile)
	}
	cfg.Profiles[profileName] = profile
	return s.Save(cfg)
}

func (s *Store) ClearToken(profileName string, identity IdentityKind) error {
	return s.SaveToken(profileName, identity, nil)
}

func AppSecretKey(profileName string) string {
	return fmt.Sprintf("%s.app.app_secret", profileName)
}

func decodeProfile(data []byte) (Profile, error) {
	var raw rawProfile
	if err := json.Unmarshal(data, &raw); err != nil {
		return Profile{}, err
	}

	profile := Profile{
		Name:                           raw.Name,
		Environment:                    raw.Environment,
		OpenPlatformBaseURL:            raw.OpenPlatformBaseURL,
		AppTokenEndpoint:               raw.AppTokenEndpoint,
		ProtectedResourceMetadataURL:   raw.ProtectedResourceMetadataURL,
		AuthorizationServerMetadataURL: raw.AuthorizationServerMetadataURL,
		Resource:                       raw.Resource,
		Scopes:                         raw.Scopes,
		BusinessType:                   raw.BusinessType,
		ClientName:                     raw.ClientName,
		DefaultIdentity:                normalizeStoredIdentity(raw.DefaultIdentity),
		CredentialBinding:              raw.CredentialBinding,
		Identities: Identities{
			User: raw.Identities.User,
			App:  raw.Identities.App,
		},
	}
	if profile.AppTokenEndpoint == "" {
		profile.AppTokenEndpoint = raw.LegacyBotTokenEndpoint
	}
	if !appIdentityConfigured(profile.Identities.App) && appIdentityConfigured(raw.Identities.Bot) {
		profile.Identities.App = raw.Identities.Bot
	}

	if profile.Identities.User.AuthorizationEndpoint == "" {
		profile.Identities.User.AuthorizationEndpoint = raw.AuthorizationEndpoint
	}
	if profile.Identities.User.TokenEndpoint == "" {
		profile.Identities.User.TokenEndpoint = raw.TokenEndpoint
	}
	if profile.Identities.User.RegistrationEndpoint == "" {
		profile.Identities.User.RegistrationEndpoint = raw.RegistrationEndpoint
	}
	if profile.Identities.User.RedirectURL == "" {
		profile.Identities.User.RedirectURL = raw.RedirectURL
	}
	if profile.Identities.User.ClientID == "" {
		profile.Identities.User.ClientID = raw.ClientID
	}
	if profile.Identities.User.Token == nil && raw.Token != nil {
		profile.Identities.User.Token = raw.Token
	}

	profile.ensureDefaults()
	return profile, nil
}

func (p *Profile) ensureDefaults() {
	p.DefaultIdentity = normalizeStoredIdentity(p.DefaultIdentity)
	if p.DefaultIdentity == "" {
		p.DefaultIdentity = IdentityUser
	}
	if p.ClientName == "" || p.ClientName == legacyClientName {
		p.ClientName = defaultClientName
	}
}

func normalizeStoredIdentity(identity IdentityKind) IdentityKind {
	if identity == legacyIdentityBot {
		return IdentityApp
	}
	return identity
}

func appIdentityConfigured(identity AppIdentity) bool {
	return identity.AuthMode != "" ||
		identity.AppID != "" ||
		identity.SecretRef != "" ||
		!identity.ConfiguredAt.IsZero() ||
		identity.Token != nil
}
