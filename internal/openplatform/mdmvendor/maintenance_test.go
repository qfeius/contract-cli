package mdmvendor_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/openplatform/mdmvendor"
)

func TestVendorMaintenanceRoutesAndPreservesPayload(t *testing.T) {
	for _, identity := range []config.IdentityKind{config.IdentityApp, config.IdentityUser} {
		t.Run(string(identity), func(t *testing.T) {
			body := []byte(`{"shortText":null,"vendorContacts":[{"id":"9007199254740993","phone":""}],"extendInfo":[{"fieldCode":"amount","num":1.0000000000000000001}]}`)
			path := "/open-apis/mdm/v1/vendors/9007199254740993"
			payload := `{"code":0,"data":{"id":"9007199254740993"}}`
			if identity == config.IdentityUser {
				path = "/open-apis/contract/v1/mcp/vendors/9007199254740993"
				payload = `{"code":0,"data":{"outcome":"APPLIED","id":"9007199254740993","code":"V1","name":"name","status":0,"changed":["shortText"]}}`
			}
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				actual, err := io.ReadAll(req.Body)
				if err != nil || string(actual) != string(body) || req.Method != http.MethodPatch || req.URL.Path != path {
					t.Fatalf("unexpected request: %s %s %s, error %v", req.Method, req.URL.Path, actual, err)
				}
				wantIDType := "employee_id"
				if identity == config.IdentityUser {
					wantIDType = "user_id"
				}
				if req.URL.Query().Get("user_id_type") != wantIDType {
					t.Fatalf("query = %s", req.URL.RawQuery)
				}
				return jsonResponse(payload), nil
			})}})
			requestContext := maintenanceContext(identity)
			requestContext.CommonQuery.Set("user_id_type", "employee_id")
			_, err := mdmvendor.NewService(client).Patch(context.Background(), requestContext, "9007199254740993", body)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVendorMaintenanceRejectsUnconfirmedSuccess(t *testing.T) {
	for _, payload := range []string{"", "null", `{}`, `{"code":0}`, `{"code":0,"data":null}`, `{"data":{"id":"1","outcome":"APPLIED"}}`,
		`{"code":0,"data":{"id":"1","outcome":"UNKNOWN"}}`, `{"code":0,"data":{"id":"1","outcome":"PENDING"}}`,
		`{"code":0,"data":{"id":9007199254740993,"outcome":"APPLIED"}}`} {
		t.Run(payload, func(t *testing.T) {
			calls := 0
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				return jsonResponse(payload), nil
			})}})
			_, err := mdmvendor.NewService(client).Create(context.Background(), maintenanceContext(config.IdentityUser), []byte(`{"vendorText":"name"}`))
			var unknown *openplatform.UncertainWriteError
			if !errors.As(err, &unknown) || calls != 1 {
				t.Fatalf("error = %v, calls = %d; expected UNKNOWN without retry", err, calls)
			}
		})
	}
}

func TestVendorMaintenanceUnknownOutcomeReportsKnownVendorIDWithoutRetry(t *testing.T) {
	const vendorID = "9007199254740993"
	calls := 0
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(`{"code":0,"data":{"id":"` + vendorID + `","outcome":"UNKNOWN"}}`), nil
	})}})

	_, err := mdmvendor.NewService(client).Create(context.Background(), maintenanceContext(config.IdentityUser), []byte(`{"vendorText":"name"}`))
	var uncertain *openplatform.UncertainWriteError
	if !errors.As(err, &uncertain) {
		t.Fatalf("expected UNKNOWN write error, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), vendorID) {
		t.Fatalf("error %q must tell the caller which vendor to query", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d; UNKNOWN must not be retried", calls)
	}
}

func TestVendorMaintenanceBusinessFailureIsNotSuccess(t *testing.T) {
	for _, payload := range []string{
		`{"code":40001,"message":"permission denied","data":{"outcome":"FAILED"}}`,
		`{"code":40001,"msg":"permission denied","data":{"outcome":"FAILED"}}`,
	} {
		t.Run(payload, func(t *testing.T) {
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(payload), nil
			})}})
			_, err := mdmvendor.NewService(client).Patch(context.Background(), maintenanceContext(config.IdentityApp), "1", []byte(`{"status":0}`))
			var business *mdmvendor.MaintenanceError
			if !errors.As(err, &business) || business.Code != 40001 || business.Message != "permission denied" {
				t.Fatalf("expected business error with message, got %v", err)
			}
			if !strings.Contains(err.Error(), "permission denied") {
				t.Fatalf("error text = %q", err.Error())
			}
		})
	}
}

func TestVendorMaintenanceRejectsMismatchedPatchAndStatusIDs(t *testing.T) {
	client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"code":0,"data":{"outcome":"APPLIED","id":"2"}}`), nil
	})}})
	service := mdmvendor.NewService(client)

	for _, call := range []func() error{
		func() error {
			_, err := service.Patch(context.Background(), maintenanceContext(config.IdentityApp), "1", []byte(`{"shortText":"name"}`))
			return err
		},
		func() error {
			_, err := service.Patch(context.Background(), maintenanceContext(config.IdentityUser), "1", []byte(`{"shortText":"name"}`))
			return err
		},
		func() error {
			_, err := service.SetStatus(context.Background(), maintenanceContext(config.IdentityUser), "1", 1)
			return err
		},
	} {
		var unknown *openplatform.UncertainWriteError
		if err := call(); !errors.As(err, &unknown) {
			t.Fatalf("expected mismatched ID to be UNKNOWN, got %v", err)
		}
	}
}

func TestVendorStatusAndCreateAreUserOnly(t *testing.T) {
	for _, target := range []int{0, 1} {
		t.Run(string(rune('0'+target)), func(t *testing.T) {
			calls := 0
			client := openplatform.New(openplatform.Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				actual, err := io.ReadAll(req.Body)
				want := `{"target_status":0}`
				if target == 1 {
					want = `{"target_status":1}`
				}
				if err != nil || string(actual) != want || req.Method != http.MethodPut || req.URL.Path != "/open-apis/contract/v1/mcp/vendors/1/status" {
					t.Fatalf("unexpected request %s %s %s", req.Method, req.URL.Path, actual)
				}
				return jsonResponse(`{"code":0,"data":{"outcome":"NO_CHANGE","id":"1","code":"V1","name":"name","status":1}}`), nil
			})}})
			service := mdmvendor.NewService(client)
			if _, err := service.SetStatus(context.Background(), maintenanceContext(config.IdentityUser), "1", target); err != nil {
				t.Fatal(err)
			}
			if _, err := service.SetStatus(context.Background(), maintenanceContext(config.IdentityApp), "1", target); err == nil {
				t.Fatal("app must be rejected")
			}
			if _, err := service.Create(context.Background(), maintenanceContext(config.IdentityApp), []byte(`{}`)); err == nil {
				t.Fatal("personal create must reject app")
			}
			if calls != 1 {
				t.Fatalf("calls = %d", calls)
			}
		})
	}
}

func TestVendorMaintenanceRejectsInvalidResourceIDsBeforeHTTP(t *testing.T) {
	service := mdmvendor.NewService(openplatform.New(openplatform.Options{}))
	for _, id := range []string{"", "0", "-1", "01", "1/../2", "1?x=1", "1.0", "9223372036854775808"} {
		if _, err := service.Patch(context.Background(), maintenanceContext(config.IdentityUser), id, []byte(`{"shortText":null}`)); err == nil {
			t.Fatalf("accepted invalid id %q", id)
		}
	}
}

func maintenanceContext(identity config.IdentityKind) openplatform.RequestContext {
	return openplatform.RequestContext{Identity: identity, BaseURL: "https://unused.invalid", AccessToken: "fixture",
		CommonQuery: url.Values{"user_id_type": {"user_id"}}}
}
