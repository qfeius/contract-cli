package cli

import (
	"encoding/json"

	"cn.qfei/contract-cli/internal/config"
)

type authJSONResult struct {
	Profile         string              `json:"profile"`
	Environment     string              `json:"environment"`
	Identity        config.IdentityKind `json:"identity"`
	DefaultIdentity config.IdentityKind `json:"default_identity"`
	Authorization   string              `json:"authorization,omitempty"`
	Message         string              `json:"message,omitempty"`
	Fields          []authStatusField   `json:"fields,omitempty"`
}

func authCommandJSONResult(profile config.Profile, identity config.IdentityKind, view authStatusView, message string) authJSONResult {
	return authJSONResult{Profile: profile.Name, Environment: profile.Environment,
		Identity: identity, DefaultIdentity: defaultIdentity(profile),
		Authorization: view.Authorization, Message: message, Fields: view.Fields}
}

func (p userAuthProvider) writeAuthorizationGuidance(authURL string, jsonOutput bool) {
	if jsonOutput {
		if p.authorizationJSONWriter != nil {
			// Like the existing text hint, stderr guidance is best-effort. Keep
			// callback.Wait responsible for cancellation and listener cleanup.
			_ = json.NewEncoder(p.authorizationJSONWriter).Encode(map[string]string{
				"event": "authorization_required", "authorization_url": authURL,
			})
		}
		return
	}
	if p.authorizationURLWriter != nil {
		_, _ = p.authorizationURLWriter.Write([]byte(authorizationURLMessage(authURL)))
	}
}
