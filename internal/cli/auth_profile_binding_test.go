package cli

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"cn.qfei/contract-cli/internal/config"
)

func TestAppLoginBindsSavedSecretEvenWhenTokenExchangeFails(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			profile := validProductionProfile()
			provider := appAuthProvider{
				logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
				secrets:   config.NewSecretsStore(t.TempDir()),
				lookupEnv: func(string) (string, bool) { return "", false },
				httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return productionResponse(status, `{"code":0,"tenant_access_token":"fixture-token","expire":3600}`), nil
				})},
			}
			_, err := provider.Login(context.Background(), &profile, authCommandOptions{AppID: "fixture-app", AppSecret: "fixture-secret"})
			if (err == nil) != (status == http.StatusOK) {
				t.Fatalf("unexpected exchange outcome: %v", err)
			}
			if profile.Identities.App.SecretRef == "" || profileCredentialBinding(t, profile) == "" {
				t.Fatal("persisted app credentials must remain bound even after exchange failure")
			}
		})
	}
}

func profileCredentialBinding(t *testing.T, profile config.Profile) string {
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
	return fields.Binding
}
