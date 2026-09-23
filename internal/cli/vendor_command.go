package cli

import (
	"context"
	"fmt"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
	entitysvc "cn.qfei/contract-cli/internal/openplatform/entity"
	vendorsvc "cn.qfei/contract-cli/internal/openplatform/mdmvendor"
)

func (a *App) runMDM(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing mdm resource")
	}

	switch args[0] {
	case "vendor":
		return a.runMDMVendor(ctx, args[1:])
	case "legal":
		return a.runMDMLegal(ctx, args[1:])
	case "fields":
		return a.runSchema(ctx, args[1:])
	case "fixed-exchange-rate":
		return a.runMDMFixedExchangeRate(ctx, args[1:])
	case "file":
		return a.runMDMFile(ctx, args[1:])
	default:
		return fmt.Errorf("unknown mdm resource %q", args[0])
	}
}

func (a *App) runMDMVendor(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing mdm vendor subcommand")
	}

	switch args[0] {
	case "create":
		return a.runMDMVendorCreate(ctx, args[1:])
	case "update":
		return a.runMDMVendorUpdate(ctx, args[1:])
	case "patch":
		return a.runMDMVendorPatch(ctx, args[1:])
	case "enable", "disable":
		return a.runMDMVendorStatus(ctx, args[0], args[1:])
	case "list-all":
		return a.runMDMVendorListAll(ctx, args[1:])
	case "query-by-cert":
		return a.runMDMVendorQueryByCert(ctx, args[1:])
	case "list":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--name", "--page-size", "--page-token"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli mdm vendor list [flags]")
		}
		options := parseCommandOptions(parsed)
		pageSize, err := parsed.Int("--page-size")
		if err != nil {
			return err
		}
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/mdm/v1/vendors", openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		response, err := vendorsvc.NewService(client).List(ctx, requestContext, vendorsvc.ListInput{
			Name:      parsed.String("--name"),
			PageSize:  pageSize,
			PageToken: parsed.String("--page-token"),
		})
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	case "get":
		parsed, err := parseArgs(args[1:], structuredValueFlags(), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 1 {
			return fmt.Errorf("usage: contract-cli mdm vendor get <vendor-id> [flags]")
		}
		options := parseCommandOptions(parsed)
		vendorID := parsed.positionals[0]
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/mdm/v1/vendors/"+vendorID, openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		response, err := vendorsvc.NewService(client).Get(ctx, requestContext, vendorID)
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	default:
		return fmt.Errorf("unknown mdm vendor subcommand %q", args[0])
	}
}

func (a *App) runMDMLegal(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing mdm legal subcommand")
	}

	switch args[0] {
	case "create":
		return a.runMDMLegalCreate(ctx, args[1:])
	case "update":
		return a.runMDMLegalUpdate(ctx, args[1:])
	case "list":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--name", "--page-size", "--page-token"), commonBoolFlags())
		if err != nil {
			return err
		}
		if len(parsed.positionals) != 0 {
			return fmt.Errorf("usage: contract-cli mdm legal list [flags]")
		}
		options := parseCommandOptions(parsed)
		pageSize, err := parsed.Int("--page-size")
		if err != nil {
			return err
		}
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/mdm/v1/legal_entities/list_all", openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		response, err := entitysvc.NewService(client).List(ctx, requestContext, entitysvc.ListInput{
			Name:      parsed.String("--name"),
			PageSize:  pageSize,
			PageToken: parsed.String("--page-token"),
		})
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	case "get":
		parsed, err := parseArgs(args[1:], structuredValueFlags("--code", "--page-size", "--page-token"), commonBoolFlags())
		if err != nil {
			return err
		}
		if strings.TrimSpace(parsed.String("--code")) != "" {
			return a.runMDMLegalGetByCode(ctx, parsed)
		}
		if len(parsed.positionals) != 1 {
			return fmt.Errorf("usage: contract-cli mdm legal get <legal-entity-id> [flags]")
		}
		if parsed.HasValue("--page-size") || parsed.HasValue("--page-token") {
			return fmt.Errorf("usage: contract-cli mdm legal get <legal-entity-id> [flags]")
		}
		options := parseCommandOptions(parsed)
		entityID := parsed.positionals[0]
		client, requestContext, err := a.openPlatformClientAndContextForOptions(options, "/open-apis/mdm/v1/legal_entities/"+entityID, openplatform.IdentityPolicyAny)
		if err != nil {
			return err
		}
		response, err := entitysvc.NewService(client).Get(ctx, requestContext, entityID)
		if err != nil {
			return err
		}
		return a.renderOpenPlatformResponse(options, response)
	default:
		return fmt.Errorf("unknown mdm legal subcommand %q", args[0])
	}
}
