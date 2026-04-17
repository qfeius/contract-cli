package cli

import (
	"context"
	"fmt"

	"cn.qfei/contract-cli/internal/openplatform"
	schemasvc "cn.qfei/contract-cli/internal/openplatform/schema"
)

func (a *App) runSchema(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing mdm fields subcommand")
	}

	switch args[0] {
	case "list":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--biz-line"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli mdm fields list --biz-line <biz-line> [flags]")
		}

		options := parseCommandOptions(parsed)
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/mdm/v1/config/config_list", openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		response, err := schemasvc.NewService(client).Fields(ctx, requestContext, parsed.String("--biz-line"))
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	default:
		return fmt.Errorf("unknown mdm fields subcommand %q", args[0])
	}
}
