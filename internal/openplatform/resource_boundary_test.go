package openplatform_test

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

func TestClientResponseLimitStopsOversizedAndUnboundedErrorBodies(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, status := range []int{200, 500} {
			if stream && status == 200 {
				continue
			}
			t.Run(strconv.Itoa(status)+map[bool]string{false: "/body", true: "/stream-error"}[stream], func(t *testing.T) {
				client := openplatform.New(openplatform.Options{MaxResponseBytes: 16, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 17)))}, nil
				})}})
				rc := openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityUser, AccessToken: "fixture"}
				request := openplatform.Request{Method: "GET", Path: "/open-apis/example"}
				var err error
				if stream {
					_, err = client.DoStream(context.Background(), rc, request, io.Discard)
				} else {
					_, err = client.Do(context.Background(), rc, request)
				}
				if err == nil || !strings.Contains(err.Error(), "response exceeds") {
					t.Fatalf("response was not bounded: %v", err)
				}
			})
		}
	}
}

func TestClientMultipartDefaultKeepsLongUploadBudget(t *testing.T) {
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) < 5*time.Minute || time.Until(deadline) > 11*time.Minute {
			t.Fatal("default multipart budget is too short")
		}
		_, _ = io.Copy(io.Discard, r.Body)
		return jsonResponse(`{}`), nil
	})}})
	_, err := client.Do(context.Background(), openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityUser, AccessToken: "fixture"}, openplatform.Request{Method: "POST", Path: "/open-apis/example", BodyReader: strings.NewReader("multipart fixture")})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientDefaultDeadlineDoesNotShortenExplicitUploadBudget(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			deadline, ok := r.Context().Deadline()
			remaining := time.Until(deadline)
			if !ok || (explicit && remaining < time.Minute) || (!explicit && remaining > 31*time.Second) {
				t.Fatalf("wrong request budget: %v %v", ok, remaining)
			}
			return jsonResponse(`{}`), nil
		})}})
		ctx := context.Background()
		if explicit {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
		}
		_, err := client.Do(ctx, openplatform.RequestContext{BaseURL: "https://example.test", Identity: config.IdentityUser, AccessToken: "fixture"}, openplatform.Request{Method: "POST", Path: "/open-apis/example"})
		if err != nil {
			t.Fatal(err)
		}
	}
}
