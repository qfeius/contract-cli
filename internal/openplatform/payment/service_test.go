package payment_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/openplatform/payment"
)

func TestServicePaymentEndpoints(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		want             string
		wantBody         string
		call             func(*payment.Service, openplatform.RequestContext) (openplatform.Response, error)
		wantBodyOptional bool
	}{
		{
			name:     "create payment",
			want:     "POST https://dev-open.qtech.cn/open-apis/contract/v1/contracts/contract-1/payments",
			wantBody: `{"apply_amount":100}`,
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.Create(context.Background(), requestContext, "contract-1", []byte(`{"apply_amount":100}`))
			},
		},
		{
			name:     "update payment",
			want:     "PATCH https://dev-open.qtech.cn/open-apis/contract/v1/contracts/contract-1/payments/payment-1",
			wantBody: `{"payment_status_code":9}`,
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.Update(context.Background(), requestContext, "contract-1", "payment-1", []byte(`{"payment_status_code":9}`))
			},
		},
		{
			name: "get payment",
			want: "GET https://dev-open.qtech.cn/open-apis/contract/v1/contracts/contract-1/payments/payment-1",
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.Get(context.Background(), requestContext, "contract-1", "payment-1")
			},
			wantBodyOptional: true,
		},
		{
			name: "list payments",
			want: "GET https://dev-open.qtech.cn/open-apis/contract/v1/contracts/contract-1/payments?page_size=10&page_token=next-1",
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.List(context.Background(), requestContext, "contract-1", payment.ListInput{
					PageSize:  10,
					PageToken: "next-1",
				})
			},
			wantBodyOptional: true,
		},
		{
			name:     "notify payment plan",
			want:     "POST https://dev-open.qtech.cn/open-apis/contract/v1/payment/notify",
			wantBody: `{"finance_number":"FN-1"}`,
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.NotifyPlan(context.Background(), requestContext, []byte(`{"finance_number":"FN-1"}`))
			},
		},
		{
			name:     "search payment plans",
			want:     "POST https://dev-open.qtech.cn/open-apis/contract/v1/payments/search",
			wantBody: `{"page_size":10}`,
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.SearchPlans(context.Background(), requestContext, []byte(`{"page_size":10}`))
			},
		},
		{
			name:     "create payment record",
			want:     "POST https://dev-open.qtech.cn/open-apis/contract/v1/contracts/contract-1/payments/payment-1/payment_records",
			wantBody: `{"transaction_amount":100}`,
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.CreateRecord(context.Background(), requestContext, "contract-1", "payment-1", []byte(`{"transaction_amount":100}`))
			},
		},
		{
			name:     "update payment record",
			want:     "PATCH https://dev-open.qtech.cn/open-apis/contract/v1/contracts/contract-1/payments/payment-1/payment_records/record-1",
			wantBody: `{"transaction_amount":90}`,
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.UpdateRecord(context.Background(), requestContext, "contract-1", "payment-1", "record-1", []byte(`{"transaction_amount":90}`))
			},
		},
		{
			name: "get payment record",
			want: "GET https://dev-open.qtech.cn/open-apis/contract/v1/contracts/contract-1/payments/payment-1/payment_records/record-1",
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.GetRecord(context.Background(), requestContext, "contract-1", "payment-1", "record-1")
			},
			wantBodyOptional: true,
		},
		{
			name: "list payment records by plan",
			want: "GET https://dev-open.qtech.cn/open-apis/contract/v1/contracts/payments/plan-1/payment_records",
			call: func(service *payment.Service, requestContext openplatform.RequestContext) (openplatform.Response, error) {
				return service.ListRecordsByPlan(context.Background(), requestContext, "plan-1")
			},
			wantBodyOptional: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := openplatform.New(openplatform.Options{
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						got := req.Method + " " + req.URL.String()
						if got != tc.want {
							t.Fatalf("request = %q, want %q", got, tc.want)
						}
						if req.Header.Get("Authorization") != "Bearer app-token" {
							t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
						}
						body, err := io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("ReadAll() error = %v", err)
						}
						if tc.wantBody != "" && string(body) != tc.wantBody {
							t.Fatalf("body = %s, want %s", string(body), tc.wantBody)
						}
						if tc.wantBodyOptional && len(body) != 0 {
							t.Fatalf("body = %q, want empty", string(body))
						}
						return jsonResponse(`{"code":0}`), nil
					}),
				},
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			})
			requestContext, err := client.RequestContext(profileWithAppToken(), config.IdentityApp)
			if err != nil {
				t.Fatalf("RequestContext() error = %v", err)
			}

			response, err := tc.call(payment.NewService(client), requestContext)
			if err != nil {
				t.Fatalf("call error = %v", err)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", response.StatusCode)
			}
		})
	}
}

func TestServicePaymentEndpointsEscapePathValues(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://dev-open.qtech.cn/open-apis/contract/v1/contracts/contract%201/payments/payment%251/payment_records/record%201" {
					t.Fatalf("url = %q", req.URL.String())
				}
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithAppToken(), config.IdentityApp)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	if _, err := payment.NewService(client).GetRecord(context.Background(), requestContext, "contract 1", "payment%1", "record 1"); err != nil {
		t.Fatalf("GetRecord() error = %v", err)
	}
}

func TestServicePaymentEndpointsRejectUnsafePathIDs(t *testing.T) {
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("unsafe ID must not send HTTP")
			return nil, nil
		})},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	rc, err := client.RequestContext(profileWithAppToken(), config.IdentityApp)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{".", "..", "a/b", `a\b`, "a\x00b", "%2e%2e"} {
		for position := range 3 {
			ids := []string{"contract-1", "payment-1", "record-1"}
			ids[position] = id
			if _, err := payment.NewService(client).GetRecord(context.Background(), rc, ids[0], ids[1], ids[2]); err == nil {
				t.Fatalf("accepted unsafe ID %q at position %d", id, position)
			}
		}
	}
}

func TestServicePaymentEndpointsRejectMissingIDs(t *testing.T) {
	t.Parallel()

	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("missing id should not send HTTP")
				return nil, nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithAppToken(), config.IdentityApp)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	testCases := []struct {
		name    string
		call    func() (openplatform.Response, error)
		wantErr string
	}{
		{
			name: "missing contract",
			call: func() (openplatform.Response, error) {
				return payment.NewService(client).Create(context.Background(), requestContext, "", []byte(`{}`))
			},
			wantErr: "contract id is required",
		},
		{
			name: "missing payment",
			call: func() (openplatform.Response, error) {
				return payment.NewService(client).Get(context.Background(), requestContext, "contract-1", "")
			},
			wantErr: "payment id is required",
		},
		{
			name: "missing record",
			call: func() (openplatform.Response, error) {
				return payment.NewService(client).GetRecord(context.Background(), requestContext, "contract-1", "payment-1", "")
			},
			wantErr: "payment record id is required",
		},
		{
			name: "missing plan",
			call: func() (openplatform.Response, error) {
				return payment.NewService(client).ListRecordsByPlan(context.Background(), requestContext, "")
			},
			wantErr: "payment plan uuid is required",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.call()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error: %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestServicePaymentEndpointsRejectUserIdentityBeforeHTTP(t *testing.T) {
	t.Parallel()

	transportUsed := false
	client := openplatform.New(openplatform.Options{
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				transportUsed = true
				return jsonResponse(`{"code":0}`), nil
			}),
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	requestContext, err := client.RequestContext(profileWithUserToken(), config.IdentityUser)
	if err != nil {
		t.Fatalf("RequestContext() error = %v", err)
	}

	_, err = payment.NewService(client).Create(context.Background(), requestContext, "contract-1", []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "only supports --as app") {
		t.Fatalf("unexpected user error: %v", err)
	}
	if transportUsed {
		t.Fatalf("request transport should not be used for rejected app-only action")
	}
}

func profileWithAppToken() config.Profile {
	return config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityApp,
		Identities: config.Identities{
			App: config.AppIdentity{
				Token: &config.Token{
					AccessToken: "app-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}
}

func profileWithUserToken() config.Profile {
	return config.Profile{
		Name:                "contract",
		Environment:         "dev",
		OpenPlatformBaseURL: "https://dev-open.qtech.cn",
		DefaultIdentity:     config.IdentityUser,
		Identities: config.Identities{
			User: config.UserIdentity{
				Token: &config.Token{
					AccessToken: "user-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				},
			},
		},
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(payload string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(payload)),
	}
}
