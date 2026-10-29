package config_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
)

func TestStoreBindsNewCredentialsButDoesNotRebindChangedAddresses(t *testing.T) {
	store := config.NewStore(t.TempDir())
	profile := bindingProfile()
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	saved, err := store.GetProfile(profile.Name)
	if err != nil {
		t.Fatal(err)
	}
	binding := storedBinding(t, saved)
	if binding == "" {
		t.Fatal("new credentials must be bound to their profile addresses")
	}
	saved.OpenPlatformBaseURL = "https://test-open.qtech.cn"
	if err := store.SaveProfile(saved); err != nil {
		t.Fatal(err)
	}
	changed, err := store.GetProfile(profile.Name)
	if err != nil || storedBinding(t, changed) != binding {
		t.Fatalf("SaveProfile must preserve the original credential binding: %v", err)
	}
}

func TestCredentialBindingTracksAddressesAndProfileNotTokens(t *testing.T) {
	changes := map[string]func(*config.Profile){
		"profile":           func(p *config.Profile) { p.Name = "other" },
		"business":          func(p *config.Profile) { p.OpenPlatformBaseURL += "/changed" },
		"resource":          func(p *config.Profile) { p.Resource += "/changed" },
		"app endpoint":      func(p *config.Profile) { p.AppTokenEndpoint += "/changed" },
		"resource metadata": func(p *config.Profile) { p.ProtectedResourceMetadataURL += "/changed" },
		"auth metadata":     func(p *config.Profile) { p.AuthorizationServerMetadataURL += "/changed" },
		"authorization":     func(p *config.Profile) { p.Identities.User.AuthorizationEndpoint += "/changed" },
		"device":            func(p *config.Profile) { p.Identities.User.DeviceAuthorizationEndpoint += "/changed" },
		"token endpoint":    func(p *config.Profile) { p.Identities.User.TokenEndpoint += "/changed" },
		"revocation":        func(p *config.Profile) { p.Identities.User.RevocationEndpoint += "/changed" },
		"registration":      func(p *config.Profile) { p.Identities.User.RegistrationEndpoint += "/changed" },
	}
	baseline := savedBinding(t, bindingProfile())
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			profile := bindingProfile()
			change(&profile)
			if savedBinding(t, profile) == baseline {
				t.Fatal("changing credential target did not change its binding")
			}
		})
	}
	profile := bindingProfile()
	profile.Identities.User.Token = &config.Token{AccessToken: "renewed-token"}
	profile.Identities.User.ClientID = "new-registration"
	profile.DefaultIdentity = config.IdentityApp
	if savedBinding(t, profile) != baseline {
		t.Fatal("token renewal, registration or identity selection changed the target binding")
	}
}

func TestLoadingLegacyCredentialsDoesNotInventTargetBinding(t *testing.T) {
	store := config.NewStore(t.TempDir())
	content := `{"current_profile":"contract","profiles":{"contract":{"name":"contract","environment":"prod","identities":{"user":{"token":{"access_token":"legacy-token"}}}}}}`
	if err := os.WriteFile(store.Path(), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	profile, err := store.GetProfile("contract")
	if err != nil || storedBinding(t, profile) != "" {
		t.Fatalf("reading legacy credentials must not sign their current addresses: %v", err)
	}
	if config.ValidateCredentialBinding(profile) == nil {
		t.Fatal("legacy credentials must require authorization before any reuse")
	}
}

func TestValidateCredentialBindingRejectsChangedTargetAndUnboundAppSecret(t *testing.T) {
	profile := bindingProfile()
	profile.CredentialBinding = config.CredentialTargetBinding(profile)
	if err := config.ValidateCredentialBinding(profile); err != nil {
		t.Fatal(err)
	}
	profile.OpenPlatformBaseURL = "https://test-open.qtech.cn"
	if config.ValidateCredentialBinding(profile) == nil {
		t.Fatal("changing the business address must reject the old credential")
	}
	profile = config.Profile{Identities: config.Identities{App: config.AppIdentity{SecretRef: "contract.app.app_secret"}}}
	if config.ValidateCredentialBinding(profile) == nil {
		t.Fatal("a saved app secret must also have a target binding")
	}
	if err := config.ValidateCredentialBinding(config.Profile{}); err != nil {
		t.Fatalf("credential-free profiles must remain usable for initialization: %v", err)
	}
}

func TestSaveTokenEstablishesBindingWithoutOverwritingAnExistingOne(t *testing.T) {
	store := config.NewStore(t.TempDir())
	profile := bindingProfile()
	profile.Identities.User.Token = nil
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveToken(profile.Name, config.IdentityUser, &config.Token{AccessToken: "fresh-token"}); err != nil {
		t.Fatal(err)
	}
	saved, err := store.GetProfile(profile.Name)
	if err != nil || storedBinding(t, saved) == "" {
		t.Fatalf("newly saved token must acquire a target binding: %v", err)
	}
	binding := saved.CredentialBinding
	saved.Resource += "/changed"
	if err := store.SaveProfile(saved); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveToken(profile.Name, config.IdentityUser, &config.Token{AccessToken: "renewed-token"}); err != nil {
		t.Fatal(err)
	}
	saved, err = store.GetProfile(profile.Name)
	if err != nil || saved.CredentialBinding != binding || config.ValidateCredentialBinding(saved) == nil {
		t.Fatalf("token storage must not re-sign modified addresses: %v", err)
	}
}

func savedBinding(t *testing.T, profile config.Profile) string {
	t.Helper()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	saved, err := store.GetProfile(profile.Name)
	if err != nil {
		t.Fatal(err)
	}
	return storedBinding(t, saved)
}

func storedBinding(t *testing.T, profile config.Profile) string {
	t.Helper()
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	var fields struct {
		Binding string `json:"credential_binding"`
	}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fields.Binding, "token") {
		t.Fatal("credential binding must not contain credential values")
	}
	return fields.Binding
}

func bindingProfile() config.Profile {
	return config.Profile{
		Name: "contract", Environment: "prod",
		OpenPlatformBaseURL: "https://open.qfei.cn", Resource: "https://open.qfei.cn",
		AppTokenEndpoint:               "https://open.qfei.cn/app-token",
		ProtectedResourceMetadataURL:   "https://open.qfei.cn/metadata",
		AuthorizationServerMetadataURL: "https://myaccount.qfei.cn/metadata",
		Identities: config.Identities{User: config.UserIdentity{
			AuthorizationEndpoint:       "https://myaccount.qfei.cn/authorize",
			DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device",
			TokenEndpoint:               "https://myaccount.qfei.cn/token",
			RevocationEndpoint:          "https://myaccount.qfei.cn/revoke",
			RegistrationEndpoint:        "https://myaccount.qfei.cn/register",
			Token:                       &config.Token{AccessToken: "fixture-token"},
		}},
	}
}
