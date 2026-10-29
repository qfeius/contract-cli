package config

import (
	"crypto/sha256"
	"fmt"
)

// CredentialTargetBinding identifies the addresses for which credentials were
// issued. It contains no credentials and does not change on token refresh or
// OAuth client registration. The environment label does not determine routing.
func CredentialTargetBinding(profile Profile) string {
	user := profile.Identities.User
	fields := []string{
		profile.Name, profile.OpenPlatformBaseURL, profile.Resource,
		profile.AppTokenEndpoint, profile.ProtectedResourceMetadataURL,
		profile.AuthorizationServerMetadataURL,
		user.AuthorizationEndpoint, user.DeviceAuthorizationEndpoint,
		user.TokenEndpoint, user.RevocationEndpoint, user.RegistrationEndpoint,
		user.RedirectURL,
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%q", fields))))
}

// ValidateCredentialBinding rejects credentials whose original targets are
// absent or differ from the configured addresses. Old files require fresh
// authorization instead of inferring credential provenance from mutable URLs.
func ValidateCredentialBinding(profile Profile) error {
	if !hasStoredCredentials(profile) {
		return nil
	}
	if profile.CredentialBinding == "" || profile.CredentialBinding != CredentialTargetBinding(profile) {
		return fmt.Errorf("profile %q credentials are not bound to its current addresses; run `contract-cli config add` with the intended addresses and authorize again", profile.Name)
	}
	return nil
}

func hasStoredCredentials(profile Profile) bool {
	return profile.Identities.User.Token != nil || profile.Identities.App.Token != nil ||
		profile.Identities.App.AppID != "" || profile.Identities.App.SecretRef != ""
}
