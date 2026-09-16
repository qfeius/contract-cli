package cli

import (
	"errors"
	"fmt"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/credential"
)

/*
loadDeviceAwareProfile 加载配置或从加密快照恢复，并校验待授权状态的环境。
入参 profileName（string）为配置名；返回 config.Profile 和 error。
*/
func (a *App) loadDeviceAwareProfile(profileName string) (config.Profile, error) {
	profile, found, err := a.store.LookupProfile(profileName)
	if err != nil {
		return config.Profile{}, err
	}
	if found {
		if err := validateProductionProfile(profile); err != nil {
			a.logger.Error("reject non-production profile", "profile", profile.Name, "error", err.Error())
			return config.Profile{}, err
		}
		if err := a.validateProductionDeviceCredentialIfAvailable(profile); err != nil {
			return config.Profile{}, err
		}
		return profile, nil
	}

	normalizedProfileName := strings.TrimSpace(profileName)
	if normalizedProfileName == "" {
		return config.Profile{}, profileNotFoundError(profileName)
	}
	agentKitWorkspace, inAgentKit := a.lookupEnv("SKILL_SESSION_WORKSPACE")
	workTaskSession, inDoubaoWorkTask := a.lookupEnv("SESSION_ID")
	hasAgentKitWorkspace := inAgentKit && strings.TrimSpace(agentKitWorkspace) != ""
	hasDoubaoWorkTask := inDoubaoWorkTask && strings.TrimSpace(workTaskSession) != ""
	if !hasAgentKitWorkspace && !hasDoubaoWorkTask {
		return config.Profile{}, profileNotFoundError(profileName)
	}
	runtimeContext, err := credential.ResolveDeviceRuntime(a.lookupEnv)
	if err != nil {
		return config.Profile{}, err
	}
	if runtimeContext.Kind != credential.DeviceRuntimeDoubaoCloud && runtimeContext.Kind != credential.DeviceRuntimeDoubaoWorkTask {
		return config.Profile{}, profileNotFoundError(profileName)
	}

	a.logger.Info("restore Device profile from encrypted credential", "profile", normalizedProfileName)
	store, err := a.deviceCredentials()
	if err != nil {
		a.logger.Error("initialize Device credential store for profile restore failed", "profile", normalizedProfileName, "error", err.Error())
		return config.Profile{}, err
	}
	stored, err := store.Load(normalizedProfileName)
	if err != nil {
		if errors.Is(err, credential.ErrCredentialNotFound) {
			return config.Profile{}, deviceProfileRecoveryError(normalizedProfileName, "encrypted Device credential is unavailable")
		}
		a.logger.Error("load encrypted Device profile failed", "profile", normalizedProfileName, "error", err.Error())
		return config.Profile{}, fmt.Errorf("load encrypted Device profile for %q: %w", normalizedProfileName, err)
	}
	profile, err = restoreDeviceProfile(normalizedProfileName, stored.DeviceProfile)
	if err != nil {
		a.logger.Error("validate encrypted Device profile failed", "profile", normalizedProfileName, "error", err.Error())
		return config.Profile{}, err
	}
	if err := validateProductionProfile(profile); err != nil {
		a.logger.Error("reject non-production Device profile snapshot", "profile", normalizedProfileName, "error", err.Error())
		return config.Profile{}, err
	}
	if err := validateProductionPendingTransaction(normalizedProfileName, stored.Pending, profile.Environment); err != nil {
		a.logger.Error("reject non-production Device pending state during profile restore", "profile", normalizedProfileName, "error", err.Error())
		return config.Profile{}, err
	}
	if err := a.store.SaveProfile(profile); err != nil {
		a.logger.Error("save restored Device profile failed", "profile", normalizedProfileName, "error", err.Error())
		return config.Profile{}, fmt.Errorf("save restored Device profile %q: %w", normalizedProfileName, err)
	}
	a.logger.Info("Device profile restored from encrypted credential", "profile", normalizedProfileName)
	return profile, nil
}

func snapshotDeviceProfile(profile config.Profile) *credential.DeviceProfile {
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

func restoreDeviceProfile(profileName string, snapshot *credential.DeviceProfile) (config.Profile, error) {
	if snapshot == nil {
		return config.Profile{}, deviceProfileRecoveryError(profileName, "encrypted Device profile snapshot is unavailable")
	}
	required := map[string]string{
		"name":                          snapshot.Name,
		"environment":                   snapshot.Environment,
		"open_platform_base_url":        snapshot.OpenPlatformBaseURL,
		"resource":                      snapshot.Resource,
		"business_type":                 snapshot.BusinessType,
		"device_client_id":              snapshot.DeviceClientID,
		"device_scope":                  snapshot.DeviceScope,
		"device_authorization_endpoint": snapshot.DeviceAuthorizationEndpoint,
		"token_endpoint":                snapshot.TokenEndpoint,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return config.Profile{}, deviceProfileRecoveryError(profileName, field+" is missing")
		}
	}
	if snapshot.Name != profileName {
		return config.Profile{}, deviceProfileRecoveryError(profileName, "profile name does not match encrypted snapshot")
	}

	return config.Profile{
		Name:                           snapshot.Name,
		Environment:                    snapshot.Environment,
		OpenPlatformBaseURL:            snapshot.OpenPlatformBaseURL,
		ProtectedResourceMetadataURL:   snapshot.ProtectedResourceMetadataURL,
		AuthorizationServerMetadataURL: snapshot.AuthorizationServerMetadataURL,
		Resource:                       snapshot.Resource,
		Scopes:                         append([]string(nil), snapshot.Scopes...),
		BusinessType:                   snapshot.BusinessType,
		ClientName:                     snapshot.ClientName,
		DefaultIdentity:                config.IdentityUser,
		Identities: config.Identities{User: config.UserIdentity{
			AuthMode:                    config.UserAuthModeDevice,
			DeviceClientID:              snapshot.DeviceClientID,
			DeviceScope:                 snapshot.DeviceScope,
			DeviceAuthorizationEndpoint: snapshot.DeviceAuthorizationEndpoint,
			TokenEndpoint:               snapshot.TokenEndpoint,
			RevocationEndpoint:          snapshot.RevocationEndpoint,
		}},
	}, nil
}

func profileNotFoundError(profileName string) error {
	return fmt.Errorf("profile %q not found", profileName)
}

func deviceProfileRecoveryError(profileName, reason string) error {
	return fmt.Errorf(
		"cannot restore Device profile %q after sandbox rebuild: %s; run `contract-cli config add --env prod --name %s` and `contract-cli auth init --profile %s --output json` again",
		profileName,
		reason,
		profileName,
		profileName,
	)
}
