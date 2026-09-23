package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/openplatform/mdmvendor"
)

func (a *App) executeVendorCreate(ctx context.Context, options commandOptions) error {
	const path = "/open-apis/mdm/v1/vendors"
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, path, openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	if err := validateVendorActor(options, requestContext.Identity, "mdm vendor create"); err != nil {
		return err
	}
	if err := validatePersonalVendorDepartmentIDType(options, requestContext.Identity, "mdm vendor create"); err != nil {
		return err
	}
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	if err := validateMDMVendorCreateBody(body); err != nil {
		return err
	}
	if requestContext.Identity == config.IdentityUser {
		if err := validatePersonalVendorBody(body); err != nil {
			return err
		}
		response, err := mdmvendor.NewService(client).Create(ctx, requestContext, body)
		return a.renderVendorMaintenanceResponse(options, response, err)
	}
	response, err := client.Do(ctx, requestContext, openplatform.Request{
		Method: http.MethodPost, Path: path, Body: body, IdentityPolicy: openplatform.IdentityPolicyAppOnly,
	})
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runMDMVendorPatch(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--department-id-type"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli mdm vendor patch <vendor-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	object, err := decodeJSONBodyObject("mdm vendor patch", body)
	if err != nil {
		return err
	}
	if len(object) == 0 {
		return fmt.Errorf("mdm vendor patch requires at least one field")
	}
	if _, exists := object["id"]; exists {
		return fmt.Errorf("mdm vendor patch uses the path ID; body.id is not allowed")
	}
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/mdm/v1/vendors", openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	if err := validateVendorActor(options, requestContext.Identity, "mdm vendor patch"); err != nil {
		return err
	}
	if err := validatePersonalVendorDepartmentIDType(options, requestContext.Identity, "mdm vendor patch"); err != nil {
		return err
	}
	if requestContext.Identity == config.IdentityUser {
		if err := validatePersonalVendorBody(body); err != nil {
			return err
		}
	}
	response, err := mdmvendor.NewService(client).Patch(ctx, requestContext, parsed.positionals[0], body)
	return a.renderVendorMaintenanceResponse(options, response, err)
}

func (a *App) runMDMVendorStatus(ctx context.Context, command string, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli mdm vendor %s <vendor-id> [flags]", command)
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "mdm vendor "+command); err != nil {
		return err
	}
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options,
		"/open-apis/contract/v1/mcp/vendors", openplatform.IdentityPolicyUserOnly)
	if err != nil {
		return err
	}
	if err := validateVendorActor(options, requestContext.Identity, "mdm vendor "+command); err != nil {
		return err
	}
	target := 0
	if command == "enable" {
		target = 1
	}
	response, err := mdmvendor.NewService(client).SetStatus(ctx, requestContext, parsed.positionals[0], target)
	return a.renderVendorMaintenanceResponse(options, response, err)
}

func validateVendorActor(options commandOptions, identity config.IdentityKind, command string) error {
	if identity == config.IdentityApp {
		return requireMDMWriteUserID(command, options)
	}
	if strings.TrimSpace(options.userID) != "" {
		return fmt.Errorf("%s personal identity must not use --user-id; the actor comes from authentication", command)
	}
	return nil
}

func validatePersonalVendorDepartmentIDType(options commandOptions, identity config.IdentityKind, command string) error {
	value := strings.TrimSpace(options.departmentIDType)
	if value == "" {
		return nil
	}
	if identity != config.IdentityUser {
		return fmt.Errorf("%s --department-id-type is only supported with personal identity", command)
	}
	if value != "department_id" && value != "open_department_id" {
		return fmt.Errorf("%s --department-id-type must be department_id or open_department_id", command)
	}
	return nil
}

func validatePersonalVendorBody(body []byte) error {
	object, err := decodeJSONBodyObject("personal vendor maintenance", body)
	if err != nil {
		return err
	}
	for _, field := range []string{"id", "vendor", "status", "isRisked", "riskLevel", "tenantId", "createdBy", "updatedBy"} {
		if _, exists := object[field]; exists {
			return fmt.Errorf("personal vendor maintenance body must not include %s", field)
		}
	}
	if personalVendorAccountsContainBankID(object["vendorAccounts"]) {
		return fmt.Errorf("personal vendor maintenance body must not include vendorAccounts[].bankId")
	}
	return nil
}

func personalVendorAccountsContainBankID(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var accounts []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &accounts); err != nil {
		return false
	}
	for _, account := range accounts {
		if _, exists := account["bankId"]; exists {
			return true
		}
	}
	return false
}

func (a *App) renderVendorMaintenanceResponse(options commandOptions, response openplatform.Response, requestErr error) error {
	if requestErr != nil {
		a.logger.Error("vendor maintenance did not return confirmed success", "http_status", response.StatusCode, "error_type", fmt.Sprintf("%T", requestErr))
		return requestErr
	}
	return a.renderOpenPlatformResponse(options, response)
}
