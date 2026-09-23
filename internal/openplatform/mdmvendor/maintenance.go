package mdmvendor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

type MaintenanceError struct {
	Code    int64
	Message string
}

func (e *MaintenanceError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return fmt.Sprintf("交易方维护失败，业务错误码 %d：%s", e.Code, e.Message)
	}
	return fmt.Sprintf("交易方维护失败，业务错误码 %d", e.Code)
}

type MaintenanceUnknownError struct {
	VendorID  string
	Uncertain *openplatform.UncertainWriteError
}

func (e *MaintenanceUnknownError) Error() string {
	return fmt.Sprintf("交易方维护结果不确定（交易方 ID：%s），请先按此 ID 查询确认", e.VendorID)
}

func (e *MaintenanceUnknownError) Unwrap() error {
	return e.Uncertain
}

// Create is the personal creation path; the existing app creation command is unchanged.
func (s *Service) Create(ctx context.Context, requestContext openplatform.RequestContext, body []byte) (openplatform.Response, error) {
	spec, ok := openplatformContractSpec("create-vendor")
	if !ok {
		return openplatform.Response{}, fmt.Errorf("vendor create spec is not configured")
	}
	return s.writeMaintenance(ctx, requestContext, openplatform.Request{
		Method:         spec.Method,
		Path:           spec.Path,
		Query:          spec.Query(nil),
		Body:           body,
		IdentityPolicy: spec.IdentityPolicy,
		OperationKind:  spec.OperationKind,
	}, "")
}

func (s *Service) Patch(ctx context.Context, requestContext openplatform.RequestContext, vendorID string, body []byte) (openplatform.Response, error) {
	if err := validateMaintenanceID(vendorID); err != nil {
		return openplatform.Response{}, err
	}
	request := openplatform.Request{
		Method:         http.MethodPatch,
		Path:           "/open-apis/mdm/v1/vendors/" + vendorID,
		Body:           body,
		IdentityPolicy: openplatform.IdentityPolicyAppOnly,
		OperationKind:  openplatform.OperationWrite,
	}
	if requestContext.Identity == config.IdentityUser {
		spec, ok := openplatformContractSpec("patch-vendor")
		if !ok {
			return openplatform.Response{}, fmt.Errorf("vendor patch spec is not configured")
		}
		request.Method = spec.Method
		request.Path = strings.ReplaceAll(spec.Path, "{vendor_id}", vendorID)
		request.Query = spec.Query(nil)
		request.IdentityPolicy = spec.IdentityPolicy
		request.OperationKind = spec.OperationKind
	}
	return s.writeMaintenance(ctx, requestContext, request, vendorID)
}

func (s *Service) SetStatus(ctx context.Context, requestContext openplatform.RequestContext, vendorID string, target int) (openplatform.Response, error) {
	if err := validateMaintenanceID(vendorID); err != nil {
		return openplatform.Response{}, err
	}
	if target != 0 && target != 1 {
		return openplatform.Response{}, fmt.Errorf("target status must be 0 or 1")
	}
	body, err := json.Marshal(struct {
		TargetStatus int `json:"target_status"`
	}{target})
	if err != nil {
		return openplatform.Response{}, fmt.Errorf("encode vendor status: %w", err)
	}
	spec, ok := openplatformContractSpec("set-vendor-status")
	if !ok {
		return openplatform.Response{}, fmt.Errorf("vendor status spec is not configured")
	}
	return s.writeMaintenance(ctx, requestContext, openplatform.Request{
		Method:         spec.Method,
		Path:           strings.ReplaceAll(spec.Path, "{vendor_id}", vendorID),
		Query:          spec.Query(nil),
		Body:           body,
		IdentityPolicy: spec.IdentityPolicy,
		OperationKind:  spec.OperationKind,
	}, vendorID)
}

func (s *Service) writeMaintenance(ctx context.Context, requestContext openplatform.RequestContext, request openplatform.Request, expectedID string) (openplatform.Response, error) {
	response, err := s.client.Do(ctx, requestContext, request)
	if err != nil {
		return response, err
	}
	return response, validateMaintenanceResponse(response.Body, requestContext.Identity, expectedID)
}

func validateMaintenanceID(id string) error {
	number, err := strconv.ParseInt(id, 10, 64)
	if err != nil || number <= 0 || strconv.FormatInt(number, 10) != id {
		return fmt.Errorf("vendor id must be a positive decimal ID string")
	}
	return nil
}

func validateMaintenanceResponse(body []byte, identity config.IdentityKind, expectedID string) error {
	var envelope struct {
		Code    *int64 `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Data    struct {
			ID      string `json:"id"`
			Outcome string `json:"outcome"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return &openplatform.UncertainWriteError{Cause: fmt.Errorf("invalid vendor maintenance response: %w", err)}
	}
	if envelope.Code == nil {
		return &openplatform.UncertainWriteError{Cause: fmt.Errorf("vendor maintenance outcome is not confirmed")}
	}
	if envelope.Data.Outcome == "UNKNOWN" {
		uncertain := &openplatform.UncertainWriteError{Cause: fmt.Errorf("vendor maintenance outcome is not confirmed")}
		if validateMaintenanceID(envelope.Data.ID) == nil && (expectedID == "" || expectedID == envelope.Data.ID) {
			return &MaintenanceUnknownError{VendorID: envelope.Data.ID, Uncertain: uncertain}
		}
		return uncertain
	}
	if *envelope.Code != 0 {
		message := envelope.Message
		if strings.TrimSpace(message) == "" {
			message = envelope.Msg
		}
		return &MaintenanceError{Code: *envelope.Code, Message: message}
	}
	if err := validateMaintenanceID(envelope.Data.ID); err != nil {
		return &openplatform.UncertainWriteError{Cause: fmt.Errorf("vendor maintenance response has no valid ID")}
	}
	if expectedID != "" && envelope.Data.ID != expectedID {
		return &openplatform.UncertainWriteError{Cause: fmt.Errorf("vendor maintenance response ID does not match the requested vendor")}
	}
	if identity == config.IdentityUser && envelope.Data.Outcome != "APPLIED" && envelope.Data.Outcome != "NO_CHANGE" {
		return &openplatform.UncertainWriteError{Cause: fmt.Errorf("vendor maintenance response has no confirmed outcome")}
	}
	return nil
}
