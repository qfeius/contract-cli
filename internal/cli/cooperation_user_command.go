package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
)

const cooperationMCPPath = "/open-apis/contract/v1/mcp/cooperations"

func (a *App) runCooperationMCPSearch(ctx context.Context, options commandOptions) error {
	body, err := cooperationSearchBody(options)
	if err != nil {
		return err
	}
	request := openplatform.Request{Method: http.MethodPost, Path: cooperationMCPPath + "/search", Query: cooperationUserQuery(), Body: body, IdentityPolicy: openplatform.IdentityPolicyUserOnly, OperationKind: openplatform.OperationRead}
	return a.executeUserCooperation(ctx, options, request)
}

func cooperationDetailRequest(id string) (openplatform.Request, error) {
	number, err := strconv.ParseInt(id, 10, 64)
	if err != nil || number <= 0 {
		return openplatform.Request{}, fmt.Errorf("--cooperation-id 必须为正 int64 ID")
	}
	return openplatform.Request{Method: http.MethodGet, Path: cooperationMCPPath + "/" + strconv.FormatInt(number, 10), Query: cooperationUserQuery(), IdentityPolicy: openplatform.IdentityPolicyUserOnly, OperationKind: openplatform.OperationRead}, nil
}

func cooperationUserQuery() url.Values {
	return url.Values{"user_id_type": {"user_id"}, "department_id_type": {"open_department_id"}}
}

func (a *App) runUserCooperationGet(ctx context.Context, args []string) error {
	parsed, err := parseReviewArgs(args, []string{"--cooperation-id"}, nil)
	if err != nil {
		return err
	}
	request, err := cooperationDetailRequest(parsed.String("--cooperation-id"))
	if err != nil {
		return err
	}
	return a.executeUserCooperation(ctx, parseCommandOptions(parsed), request)
}

func (a *App) runUserCooperationHistory(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "list" {
		return fmt.Errorf("expected contract cooperation history-files list")
	}
	parsed, err := parseReviewArgs(args[1:], []string{"--cooperation-id"}, []string{"--include-current-version"})
	if err != nil {
		return err
	}
	request, err := cooperationDetailRequest(parsed.String("--cooperation-id"))
	if err != nil {
		return err
	}
	include := true
	if _, found := parsed.bools["--include-current-version"]; found {
		include = parsed.Bool("--include-current-version")
	}
	request.Path += "/history_files"
	request.Query.Set("include_current_version", strconv.FormatBool(include))
	return a.executeUserCooperation(ctx, parseCommandOptions(parsed), request)
}

func (a *App) executeUserCooperation(ctx context.Context, options commandOptions, request openplatform.Request) (err error) {
	a.logger.Info("user cooperation request started", "method", request.Method, "path", request.Path)
	defer func() {
		if err != nil {
			a.logger.Error("user cooperation request failed", "path", request.Path, "error_type", fmt.Sprintf("%T", err))
		}
	}()
	client, rc, err := a.openPlatformClientAndContextForOptions(options, request.Path, openplatform.IdentityPolicyUserOnly)
	if err != nil {
		return err
	}
	rc.CommonQuery = nil // ID types are fixed; the authenticated user cannot be overridden.
	response, err := client.Do(ctx, rc, request)
	if err != nil {
		return err
	}
	outputErr := a.renderOpenPlatformResponse(options, response)
	if err := cooperationResponseError(response.Body); err != nil {
		return errors.Join(err, outputErr)
	}
	a.logger.Info("user cooperation request completed", "path", request.Path)
	return outputErr
}

func cooperationResponseError(body []byte) error {
	var envelope struct {
		Code    json.Number `json:"code"`
		Success *bool       `json:"success"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("invalid cooperation response: %w", err)
	}
	if (envelope.Success != nil && !*envelope.Success) || (envelope.Code != "" && envelope.Code != "0" && envelope.Code != "200") {
		return fmt.Errorf("cooperation request failed with business code %s; see response output for details", envelope.Code)
	}
	return nil
}

func (a *App) cooperationSearchIdentity(options commandOptions) (config.IdentityKind, error) {
	profile, err := a.loadDeviceAwareProfile(options.profileName)
	if err != nil {
		return "", err
	}
	return resolveIdentity(profile, options.identity, contractOpenAPIPathPrefix+"/cooperation/search", openplatform.IdentityPolicyAny)
}

func (a *App) executeCooperationSearch(ctx context.Context, options commandOptions) error {
	identity, err := a.cooperationSearchIdentity(options)
	if err != nil {
		return err
	}
	a.logger.Info("cooperation search identity selected", "identity", identity)
	if identity == config.IdentityUser {
		return a.runCooperationMCPSearch(ctx, options)
	}
	body, err := resolveRequiredRawBody(options)
	if err != nil {
		return err
	}
	return a.executeAppOpenPlatformRequest(ctx, options, http.MethodPost, contractOpenAPIPathPrefix+"/cooperation/search", nil, body)
}
