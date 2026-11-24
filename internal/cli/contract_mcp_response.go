package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"cn.qfei/contract-cli/internal/openplatform"
)

func (a *App) renderContractMCPResponse(options commandOptions, response openplatform.Response) error {
	return a.renderCheckedContractResponse("contract MCP", options, response)
}

func (a *App) renderCheckedContractResponse(operation string, options commandOptions, response openplatform.Response) error {
	responseErr := validateContractEnvelope(operation, response.Body)
	if responseErr != nil {
		a.logger.Error(operation+" response rejected", "error", responseErr.Error())
	}
	// Preserve the response for scripts even when the command must exit with an error.
	outputErr := a.renderOpenPlatformResponse(options, response)
	return errors.Join(responseErr, outputErr)
}

func validateContractEnvelope(operation string, body []byte) error {
	var envelope struct {
		Code    *int64 `json:"code"`
		Success *bool  `json:"success"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("invalid %s response envelope", operation)
	}
	if envelope.Code == nil {
		return fmt.Errorf("%s response is missing a business code", operation)
	}
	if *envelope.Code != 0 || (envelope.Success != nil && !*envelope.Success) {
		return fmt.Errorf("%s request failed with business code %d; see response output for details", operation, *envelope.Code)
	}
	return nil
}
