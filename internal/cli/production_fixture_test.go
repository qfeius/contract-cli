package cli_test

import (
	"net/url"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

func productionProfileFixture(profile config.Profile) config.Profile {
	profile.Environment = "prod"
	profile.OpenPlatformBaseURL = "https://open.qfei.cn"
	profile.Resource = "https://open.qfei.cn"
	profile.AppTokenEndpoint = productionFixtureURL(profile.AppTokenEndpoint, "https://open.qfei.cn")
	profile.ProtectedResourceMetadataURL = productionFixtureURL(profile.ProtectedResourceMetadataURL, "https://open.qfei.cn")
	profile.AuthorizationServerMetadataURL = productionFixtureURL(profile.AuthorizationServerMetadataURL, "https://myaccount.qfei.cn")
	if profile.AuthorizationServerMetadataURL == "" {
		profile.AuthorizationServerMetadataURL = "https://myaccount.qfei.cn/.well-known/oauth-authorization-server/contract"
	}
	profile.Identities.User.AuthorizationEndpoint = productionFixtureURL(profile.Identities.User.AuthorizationEndpoint, "https://myaccount.qfei.cn")
	profile.Identities.User.DeviceAuthorizationEndpoint = productionFixtureURL(profile.Identities.User.DeviceAuthorizationEndpoint, "https://myaccount.qfei.cn")
	profile.Identities.User.TokenEndpoint = productionFixtureURL(profile.Identities.User.TokenEndpoint, "https://myaccount.qfei.cn")
	profile.Identities.User.RevocationEndpoint = productionFixtureURL(profile.Identities.User.RevocationEndpoint, "https://myaccount.qfei.cn")
	profile.Identities.User.RegistrationEndpoint = productionFixtureURL(profile.Identities.User.RegistrationEndpoint, "https://myaccount.qfei.cn")
	return profile
}

func deviceSnapshotFixture(profile config.Profile) *credential.DeviceProfile {
	user := profile.Identities.User
	return &credential.DeviceProfile{
		Name:                           profile.Name,
		Environment:                    profile.Environment,
		OpenPlatformBaseURL:            profile.OpenPlatformBaseURL,
		ProtectedResourceMetadataURL:   profile.ProtectedResourceMetadataURL,
		AuthorizationServerMetadataURL: profile.AuthorizationServerMetadataURL,
		Resource:                       profile.Resource,
		Scopes:                         append([]string(nil), profile.Scopes...),
		BusinessType:                   profile.BusinessType,
		ClientName:                     profile.ClientName,
		DeviceClientID:                 user.DeviceClientID,
		DeviceScope:                    user.DeviceScope,
		DeviceAuthorizationEndpoint:    user.DeviceAuthorizationEndpoint,
		TokenEndpoint:                  user.TokenEndpoint,
		RevocationEndpoint:             user.RevocationEndpoint,
	}
}

func productionFixtureURL(rawURL, origin string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	productionOrigin, err := url.Parse(origin)
	if err != nil {
		panic(err)
	}
	parsed.Scheme = productionOrigin.Scheme
	parsed.Host = productionOrigin.Host
	return parsed.String()
}
