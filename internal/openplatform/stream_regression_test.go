package openplatform_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/openplatform/contract"
	"cn.qfei/contract-cli/internal/openplatform/payment"
)

func TestStreamPreparesAndRefreshesTrustedTokens(t *testing.T) {
	calls, prepares, refreshes, hooks := 0, 0, 0, 0
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			want := "prepared"
			if calls == 2 {
				want = "refreshed"
			}
			if r.Header.Get("Authorization") != "Bearer "+want {
				t.Fatal("wrong token used")
			}
			if calls == 1 {
				return responseWithAuthenticationError(401, "token_expired", `{"data":{"error_type":"token_expired"}}`), nil
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("file")), ContentLength: 4}, nil
		})},
		BeforeRequestHooks: []openplatform.BeforeRequestHook{func(context.Context, *http.Request) error { hooks++; return nil }},
	})
	rc := openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityUser, AccessToken: "old",
		PrepareAccessToken: func(_ context.Context, token string) (string, error) {
			prepares++
			if token != "old" {
				t.Fatal(token)
			}
			return "prepared", nil
		},
		RefreshAccessToken: func(_ context.Context, token string) (string, error) {
			refreshes++
			if token != "prepared" {
				t.Fatal(token)
			}
			return "refreshed", nil
		},
	}
	var out bytes.Buffer
	_, err := client.DoStream(context.Background(), rc, openplatform.Request{Method: "GET", Path: "/open-apis/contract/v1/files/1"}, &out)
	if err != nil || out.String() != "file" || calls != 2 || prepares != 1 || refreshes != 1 || hooks != 2 || rc.AccessToken != "old" {
		t.Fatalf("err=%v body=%q calls=%d prepare=%d refresh=%d hooks=%d", err, out.String(), calls, prepares, refreshes, hooks)
	}
}

func TestStreamRefreshIsTrustedAndBounded(t *testing.T) {
	for _, tc := range []struct {
		name             string
		status           int
		header, body     string
		refreshes, calls int
	}{
		{"body only", 401, "", `{"data":{"error_type":"token_expired"}}`, 0, 1},
		{"header only", 401, "token_expired", `{}`, 0, 1},
		{"forbidden", 403, "token_expired", `{"data":{"error_type":"token_expired"}}`, 0, 1},
		{"expired twice", 401, "token_expired", `{"data":{"error_type":"token_expired"}}`, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, refreshes := 0, 0
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return responseWithAuthenticationError(tc.status, tc.header, tc.body), nil
			})}})
			var out bytes.Buffer
			_, err := client.DoStream(context.Background(), openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityApp, AccessToken: "old", RefreshAccessToken: func(context.Context, string) (string, error) { refreshes++; return "new", nil }}, openplatform.Request{Method: "GET", Path: "/open-apis/contract/v1/files/1"}, &out)
			var status *openplatform.HTTPStatusError
			if !errors.As(err, &status) || out.Len() != 0 || calls != tc.calls || refreshes != tc.refreshes {
				t.Fatalf("err=%v calls=%d refreshes=%d body=%q", err, calls, refreshes, out.String())
			}
		})
	}
}

func TestStreamPreparationFailureStopsBeforeHTTP(t *testing.T) {
	for _, failure := range []error{nil, errors.New("prepare failed")} {
		client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("must not send HTTP"); return nil, nil })}})
		_, err := client.DoStream(context.Background(), openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityApp, PrepareAccessToken: func(context.Context, string) (string, error) { return "", failure }}, openplatform.Request{Method: "GET", Path: "/open-apis/contract/v1/files/1"}, io.Discard)
		if err == nil {
			t.Fatal("accepted missing prepared token")
		}
	}
}

func TestStreamChecksLengthAndDoesNotReplayPartialOutput(t *testing.T) {
	for _, tc := range []struct {
		name    string
		length  int64
		body    string
		readErr error
		wantErr bool
	}{
		{"complete", 4, "file", nil, false}, {"unknown", -1, "file", nil, false},
		{"short", 8, "file", nil, true}, {"long", 2, "file", nil, true},
		{"empty", 0, "", nil, false}, {"unexpected body", 0, "file", nil, true},
		{"reader failure", -1, "part", io.ErrUnexpectedEOF, true}, {"cancelled", -1, "part", context.Canceled, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				reader := io.Reader(strings.NewReader(tc.body))
				if tc.readErr != nil {
					reader = io.MultiReader(reader, streamErrorReader{tc.readErr})
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(reader), ContentLength: tc.length}, nil
			})}})
			var out bytes.Buffer
			_, err := client.DoStream(context.Background(), openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityApp}, openplatform.Request{Method: "GET", Path: "/open-apis/contract/v1/files/1"}, &out)
			if (err != nil) != tc.wantErr || calls != 1 || out.String() != tc.body {
				t.Fatalf("err=%v calls=%d body=%q", err, calls, out.String())
			}
			if tc.readErr != nil && !errors.Is(err, tc.readErr) {
				t.Fatalf("lost read error: %v", err)
			}
		})
	}
}

type streamErrorReader struct{ err error }

func (r streamErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestStreamDoesNotReplayRequestBodyOnTokenRefresh(t *testing.T) {
	calls := 0
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return responseWithAuthenticationError(401, "token_expired", `{"data":{"error_type":"token_expired"}}`), nil
	})}})
	_, err := client.DoStream(context.Background(), openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityApp, RefreshAccessToken: func(context.Context, string) (string, error) { return "new", nil }}, openplatform.Request{Method: "POST", Path: "/open-apis/files", BodyReader: strings.NewReader("body")}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "cannot be replayed") || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestStreamHeadResponseDoesNotRequireBodyBytes(t *testing.T) {
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: 123, Body: http.NoBody}, nil
	})}})
	_, err := client.DoStream(context.Background(), openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityApp}, openplatform.Request{Method: http.MethodHead, Path: "/open-apis/files/1"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAppSearchPOSTsAreReadsWithoutChangingGenericPOSTs(t *testing.T) {
	for _, kind := range []string{"contract search", "payment plans", "generic write"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != `{"name":"unchanged"}` {
					t.Fatalf("body changed: %q %v", body, err)
				}
				if calls == 1 {
					return nil, temporaryNetworkError{}
				}
				return jsonResponse(`{}`), nil
			})}})
			rc := openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityApp}
			var err error
			switch kind {
			case "contract search":
				_, err = contract.NewService(client).Search(context.Background(), rc, contract.SearchInput{Body: []byte(`{"name":"unchanged"}`)})
			case "payment plans":
				_, err = payment.NewService(client).SearchPlans(context.Background(), rc, []byte(`{"name":"unchanged"}`))
			case "generic write":
				_, err = client.Do(context.Background(), rc, openplatform.Request{Method: "POST", Path: "/open-apis/arbitrary", Body: []byte(`{"name":"unchanged"}`)})
			}
			if kind == "generic write" {
				var uncertain *openplatform.UncertainWriteError
				if !errors.As(err, &uncertain) || calls != 1 {
					t.Fatalf("write retried: %v %d", err, calls)
				}
			} else if err != nil || calls != 2 {
				t.Fatalf("read was not retried: %v %d", err, calls)
			}
		})
	}
}
