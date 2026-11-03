package cli

import (
	"context"
	"fmt"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
	paymentsvc "cn.qfei/contract-cli/internal/openplatform/payment"
)

func (a *App) runPayment(ctx context.Context, args []string) error {
	a.logger.Info("payment command started")
	if len(args) == 0 {
		a.logger.Error("payment command missing subcommand")
		return fmt.Errorf("missing payment subcommand")
	}

	switch args[0] {
	case "create":
		return a.runPaymentCreate(ctx, args[1:])
	case "update":
		return a.runPaymentUpdate(ctx, args[1:])
	case "get":
		return a.runPaymentGet(ctx, args[1:])
	case "list":
		return a.runPaymentList(ctx, args[1:])
	case "plan":
		return a.runPaymentPlan(ctx, args[1:])
	case "record":
		return a.runPaymentRecord(ctx, args[1:])
	default:
		a.logger.Error("unknown payment subcommand", "subcommand", args[0])
		return fmt.Errorf("unknown payment subcommand %q", args[0])
	}
}

func (a *App) runPaymentCreate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--contract"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli payment create --contract <contract-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	contractID, err := requiredParsedValue(parsed, "--contract")
	if err != nil {
		return err
	}
	body, err := resolveRequiredRawBody(ctx, options)
	if err != nil {
		return err
	}

	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/payments", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).Create(ctx, requestContext, contractID, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runPaymentUpdate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--contract"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli payment update <payment-id> --contract <contract-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	contractID, err := requiredParsedValue(parsed, "--contract")
	if err != nil {
		return err
	}
	body, err := resolveRequiredRawBody(ctx, options)
	if err != nil {
		return err
	}

	paymentID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/payments/"+paymentID, openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).Update(ctx, requestContext, contractID, paymentID, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runPaymentGet(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--contract"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli payment get <payment-id> --contract <contract-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "payment get"); err != nil {
		return err
	}
	contractID, err := requiredParsedValue(parsed, "--contract")
	if err != nil {
		return err
	}

	paymentID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/payments/"+paymentID, openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).Get(ctx, requestContext, contractID, paymentID)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runPaymentList(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--contract", "--page-size", "--page-token"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli payment list --contract <contract-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "payment list"); err != nil {
		return err
	}
	contractID, err := requiredParsedValue(parsed, "--contract")
	if err != nil {
		return err
	}
	pageSize, err := parsed.Int("--page-size")
	if err != nil {
		return err
	}

	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/payments", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).List(ctx, requestContext, contractID, paymentsvc.ListInput{
		PageSize:  pageSize,
		PageToken: parsed.String("--page-token"),
	})
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runPaymentPlan(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing payment plan subcommand")
	}
	switch args[0] {
	case "notify":
		return a.runPaymentPlanNotify(ctx, args[1:])
	case "search":
		return a.runPaymentPlanSearch(ctx, args[1:])
	default:
		return fmt.Errorf("unknown payment plan subcommand %q", args[0])
	}
}

func (a *App) runPaymentPlanNotify(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli payment plan notify --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(ctx, options)
	if err != nil {
		return err
	}

	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/payment/notify", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).NotifyPlan(ctx, requestContext, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runPaymentPlanSearch(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags(), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli payment plan search --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	body, err := resolveRequiredRawBody(ctx, options)
	if err != nil {
		return err
	}

	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/payments/search", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).SearchPlans(ctx, requestContext, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runPaymentRecord(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing payment record subcommand")
	}
	switch args[0] {
	case "create":
		return a.runPaymentRecordCreate(ctx, args[1:])
	case "update":
		return a.runPaymentRecordUpdate(ctx, args[1:])
	case "get":
		return a.runPaymentRecordGet(ctx, args[1:])
	case "list":
		return a.runPaymentRecordList(ctx, args[1:])
	default:
		return fmt.Errorf("unknown payment record subcommand %q", args[0])
	}
}

func (a *App) runPaymentRecordCreate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--contract", "--payment"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli payment record create --contract <contract-id> --payment <payment-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	contractID, paymentID, err := requiredContractPaymentFlags(parsed)
	if err != nil {
		return err
	}
	body, err := resolveRequiredRawBody(ctx, options)
	if err != nil {
		return err
	}

	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/payments/"+paymentID+"/payment_records", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).CreateRecord(ctx, requestContext, contractID, paymentID, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runPaymentRecordUpdate(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--contract", "--payment"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli payment record update <payment-record-id> --contract <contract-id> --payment <payment-id> --input-file <path>|--data <json> [flags]")
	}
	options := parseCommandOptions(parsed)
	contractID, paymentID, err := requiredContractPaymentFlags(parsed)
	if err != nil {
		return err
	}
	body, err := resolveRequiredRawBody(ctx, options)
	if err != nil {
		return err
	}

	paymentRecordID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/payments/"+paymentID+"/payment_records/"+paymentRecordID, openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).UpdateRecord(ctx, requestContext, contractID, paymentID, paymentRecordID, body)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runPaymentRecordGet(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--contract", "--payment"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 1 {
		return fmt.Errorf("usage: contract-cli payment record get <payment-record-id> --contract <contract-id> --payment <payment-id> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "payment record get"); err != nil {
		return err
	}
	contractID, paymentID, err := requiredContractPaymentFlags(parsed)
	if err != nil {
		return err
	}

	paymentRecordID := parsed.positionals[0]
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/"+contractID+"/payments/"+paymentID+"/payment_records/"+paymentRecordID, openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).GetRecord(ctx, requestContext, contractID, paymentID, paymentRecordID)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) runPaymentRecordList(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args, structuredValueFlags("--plan"), commonBoolFlags())
	if err != nil {
		return err
	}
	if len(parsed.positionals) != 0 {
		return fmt.Errorf("usage: contract-cli payment record list --plan <payment-plan-uuid> [flags]")
	}
	options := parseCommandOptions(parsed)
	if err := rejectRawBody(options, "payment record list"); err != nil {
		return err
	}
	paymentPlanUUID, err := requiredParsedValue(parsed, "--plan")
	if err != nil {
		return err
	}

	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, contractOpenAPIPathPrefix+"/contracts/payments/"+paymentPlanUUID+"/payment_records", openplatform.IdentityPolicyAppOnly)
	if err != nil {
		return err
	}
	response, err := paymentsvc.NewService(client).ListRecordsByPlan(ctx, requestContext, paymentPlanUUID)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func requiredContractPaymentFlags(parsed parsedArgs) (string, string, error) {
	contractID, err := requiredParsedValue(parsed, "--contract")
	if err != nil {
		return "", "", err
	}
	paymentID, err := requiredParsedValue(parsed, "--payment")
	if err != nil {
		return "", "", err
	}
	return contractID, paymentID, nil
}

func requiredParsedValue(parsed parsedArgs, name string) (string, error) {
	value := strings.TrimSpace(parsed.String(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func rejectRawBody(options commandOptions, commandName string) error {
	if options.inputFile != "" || options.data != "" {
		return fmt.Errorf("%s does not accept --input-file or --data", commandName)
	}
	return nil
}
