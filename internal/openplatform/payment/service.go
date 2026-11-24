package payment

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
)

type Service struct {
	client *openplatform.Client
}

type ListInput struct {
	PageSize  int
	PageToken string
}

func NewService(client *openplatform.Client) *Service {
	return &Service{client: client}
}

func (s *Service) Create(ctx context.Context, requestContext openplatform.RequestContext, contractID string, body []byte) (openplatform.Response, error) {
	contractID, err := requireID(contractID, "contract id")
	if err != nil {
		return openplatform.Response{}, err
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/payments",
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) Update(ctx context.Context, requestContext openplatform.RequestContext, contractID string, paymentID string, body []byte) (openplatform.Response, error) {
	contractID, paymentID, err := requireContractPaymentIDs(contractID, paymentID)
	if err != nil {
		return openplatform.Response{}, err
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPatch,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/payments/" + url.PathEscape(paymentID),
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) Get(ctx context.Context, requestContext openplatform.RequestContext, contractID string, paymentID string) (openplatform.Response, error) {
	contractID, paymentID, err := requireContractPaymentIDs(contractID, paymentID)
	if err != nil {
		return openplatform.Response{}, err
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/payments/" + url.PathEscape(paymentID),
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) List(ctx context.Context, requestContext openplatform.RequestContext, contractID string, input ListInput) (openplatform.Response, error) {
	contractID, err := requireID(contractID, "contract id")
	if err != nil {
		return openplatform.Response{}, err
	}
	query := url.Values{}
	if input.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(input.PageSize))
	}
	if strings.TrimSpace(input.PageToken) != "" {
		query.Set("page_token", strings.TrimSpace(input.PageToken))
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/payments",
		Query:          query,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) NotifyPlan(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/payment/notify",
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) SearchPlans(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/payments/search",
		OperationKind:  openplatform.OperationRead,
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) CreateRecord(ctx context.Context, requestContext openplatform.RequestContext, contractID string, paymentID string, body []byte) (openplatform.Response, error) {
	contractID, paymentID, err := requireContractPaymentIDs(contractID, paymentID)
	if err != nil {
		return openplatform.Response{}, err
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPost,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/payments/" + url.PathEscape(paymentID) + "/payment_records",
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) UpdateRecord(ctx context.Context, requestContext openplatform.RequestContext, contractID string, paymentID string, paymentRecordID string, body []byte) (openplatform.Response, error) {
	contractID, paymentID, paymentRecordID, err := requirePaymentRecordIDs(contractID, paymentID, paymentRecordID)
	if err != nil {
		return openplatform.Response{}, err
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodPatch,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/payments/" + url.PathEscape(paymentID) + "/payment_records/" + url.PathEscape(paymentRecordID),
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) GetRecord(ctx context.Context, requestContext openplatform.RequestContext, contractID string, paymentID string, paymentRecordID string) (openplatform.Response, error) {
	contractID, paymentID, paymentRecordID, err := requirePaymentRecordIDs(contractID, paymentID, paymentRecordID)
	if err != nil {
		return openplatform.Response{}, err
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/contracts/" + url.PathEscape(contractID) + "/payments/" + url.PathEscape(paymentID) + "/payment_records/" + url.PathEscape(paymentRecordID),
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func (s *Service) ListRecordsByPlan(ctx context.Context, requestContext openplatform.RequestContext, paymentPlanUUID string) (openplatform.Response, error) {
	paymentPlanUUID, err := requireID(paymentPlanUUID, "payment plan uuid")
	if err != nil {
		return openplatform.Response{}, err
	}
	return s.client.Do(ctx, requestContext, openplatform.Request{
		Method:         http.MethodGet,
		Path:           "/open-apis/contract/v1/contracts/payments/" + url.PathEscape(paymentPlanUUID) + "/payment_records",
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
}

func requireContractPaymentIDs(contractID string, paymentID string) (string, string, error) {
	contractID, err := requireID(contractID, "contract id")
	if err != nil {
		return "", "", err
	}
	paymentID, err = requireID(paymentID, "payment id")
	if err != nil {
		return "", "", err
	}
	return contractID, paymentID, nil
}

func requirePaymentRecordIDs(contractID string, paymentID string, paymentRecordID string) (string, string, string, error) {
	contractID, paymentID, err := requireContractPaymentIDs(contractID, paymentID)
	if err != nil {
		return "", "", "", err
	}
	paymentRecordID, err = requireID(paymentRecordID, "payment record id")
	if err != nil {
		return "", "", "", err
	}
	return contractID, paymentID, paymentRecordID, nil
}

func requireID(value string, name string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}
