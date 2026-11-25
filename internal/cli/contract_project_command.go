package cli

import (
	"context"
	"fmt"
	"math"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	contractsvc "cn.qfei/contract-cli/internal/openplatform/contract"
	"cn.qfei/contract-cli/internal/output"
)

func (a *App) runContractProject(ctx context.Context, args []string) (err error) {
	const usage = "usage: contract-cli contract project search [flags]"
	if len(args) == 0 || args[0] != "search" {
		return fmt.Errorf("%s", usage)
	}
	a.logger.Info("search contract projects")
	defer func() {
		if err != nil {
			a.logger.Error("search contract projects failed", "error_type", fmt.Sprintf("%T", err))
		}
	}()

	parsed, err := parseArgs(args[1:], commonValueFlags("--keyword", "--page-index", "--page-size"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("%s", usage)
	}
	if parsed.HasValue("--input-file") || parsed.HasValue("--data") {
		return fmt.Errorf("contract project search does not accept --input-file or --data")
	}

	fields := make(map[string]any)
	if keyword := strings.TrimSpace(parsed.String("--keyword")); keyword != "" {
		fields["keyword"] = keyword
	}
	if parsed.HasValue("--page-index") {
		pageIndex, err := parsed.Int("--page-index")
		if err != nil {
			return err
		}
		if pageIndex < 0 || pageIndex > math.MaxInt32 {
			return fmt.Errorf("--page-index must be between 0 and %d", math.MaxInt32)
		}
		fields["page_index"] = pageIndex
	}
	if parsed.HasValue("--page-size") {
		pageSize, err := parsed.Int("--page-size")
		if err != nil {
			return err
		}
		if pageSize < 1 || pageSize > 200 {
			return fmt.Errorf("--page-size must be between 1 and 200")
		}
		fields["page_size"] = pageSize
	}
	body, err := marshalJSONObject(fields)
	if err != nil {
		return err
	}

	options := parseCommandOptions(parsed)
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options,
		contractOpenAPIPathPrefix+"/projects/search", openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	a.logger.Info("resolved contract project search identity", "identity", requestContext.Identity)
	response, err := contractsvc.NewService(client).SearchProjects(ctx, requestContext, body)
	if err != nil {
		return err
	}
	if !options.raw && options.output == output.FormatTable && validateContractEnvelope("contract MCP", response.Body) == nil {
		if rendered, tableErr := writeContractProjectTable(a.stdout, response.Body); rendered {
			return tableErr
		}
	}
	if requestContext.Identity == config.IdentityUser {
		return a.renderContractMCPResponse(options, response)
	}
	return a.renderOpenPlatformResponse(options, response)
}
