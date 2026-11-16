package openplatform_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestRequestNotSentEvidenceCoversEveryAttempt(t *testing.T) {
	for _, mode := range []string{"path", "prepare", "hook", "cancel", "transport", "refresh", "second hook", "success"} {
		t.Run(mode, func(t *testing.T) {
			calls, hooks := 0, 0
			failure := errors.New("fixture local failure")
			client := openplatform.New(openplatform.Options{
				HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					calls++
					if mode == "transport" {
						return nil, failure
					}
					if mode == "refresh" || mode == "second hook" {
						return responseWithAuthenticationError(401, "token_expired", `{"data":{"error_type":"token_expired"}}`), nil
					}
					return jsonResponse(`{}`), nil
				})},
				BeforeRequestHooks: []openplatform.BeforeRequestHook{func(context.Context, *http.Request) error {
					hooks++
					if mode == "hook" || (mode == "second hook" && hooks == 2) {
						return failure
					}
					return nil
				}},
			})
			rc := openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityUser, AccessToken: "old",
				PrepareAccessToken: func(context.Context, string) (string, error) {
					if mode == "prepare" {
						return "", failure
					}
					return "old", nil
				},
				RefreshAccessToken: func(context.Context, string) (string, error) {
					if mode == "refresh" {
						return "", failure
					}
					return "new", nil
				},
			}
			request := openplatform.Request{Method: "POST", Path: "/open-apis/contracts", Body: []byte(`{}`)}
			if mode == "path" {
				request.Path = "/open-apis/../contracts"
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				cancel()
			}
			_, err := client.Do(ctx, rc, request)
			wantNotSent := mode == "path" || mode == "prepare" || mode == "hook" || mode == "cancel"
			if openplatform.RequestWasNotSent(err) != wantNotSent || (calls == 0) != wantNotSent {
				t.Fatalf("mode=%s err=%v HTTP=%d", mode, err, calls)
			}
			if mode != "path" && mode != "success" && mode != "cancel" && !errors.Is(err, failure) {
				t.Fatalf("lost error cause: %v", err)
			}
		})
	}
	if openplatform.RequestWasNotSent(errors.New("prepare access token failed")) {
		t.Fatal("error text is not dispatch evidence")
	}
}
