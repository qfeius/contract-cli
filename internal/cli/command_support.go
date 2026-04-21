package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/openplatform"
	"cn.qfei/contract-cli/internal/output"
)

type commandOptions struct {
	profileName string
	identity    string
	output      output.Format
	raw         bool
	inputFile   string
	data        string
	userIDType  string
	userID      string
}

type parsedArgs struct {
	values      map[string][]string
	bools       map[string]bool
	positionals []string
}

func parseArgs(args []string, valueFlags map[string]struct{}, boolFlags map[string]struct{}) (parsedArgs, error) {
	parsed := parsedArgs{
		values:      map[string][]string{},
		bools:       map[string]bool{},
		positionals: []string{},
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			parsed.positionals = append(parsed.positionals, arg)
			continue
		}

		name, inlineValue, hasInlineValue := strings.Cut(arg, "=")
		if _, ok := boolFlags[name]; ok {
			if !hasInlineValue {
				parsed.bools[name] = true
				continue
			}
			value, err := strconv.ParseBool(inlineValue)
			if err != nil {
				return parsedArgs{}, fmt.Errorf("invalid boolean value for %s: %w", name, err)
			}
			parsed.bools[name] = value
			continue
		}
		if _, ok := valueFlags[name]; ok {
			value, err := argValue(args, &i, inlineValue, hasInlineValue)
			if err != nil {
				return parsedArgs{}, err
			}
			parsed.values[name] = append(parsed.values[name], value)
			continue
		}
		return parsedArgs{}, fmt.Errorf("unknown flag %q", name)
	}

	return parsed, nil
}

func argValue(args []string, index *int, inlineValue string, hasInlineValue bool) (string, error) {
	if hasInlineValue {
		return inlineValue, nil
	}
	if *index+1 >= len(args) {
		return "", fmt.Errorf("missing value for %s", args[*index])
	}
	*index = *index + 1
	return args[*index], nil
}

func (p parsedArgs) String(name string) string {
	values := p.values[name]
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func (p parsedArgs) Strings(name string) []string {
	values := p.values[name]
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func (p parsedArgs) Bool(name string) bool {
	return p.bools[name]
}

func (p parsedArgs) Int(name string) (int, error) {
	value := p.String(name)
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid value for %s: %w", name, err)
	}
	return parsed, nil
}

func parseCommandOptions(parsed parsedArgs) commandOptions {
	return commandOptions{
		profileName: parsed.String("--profile"),
		identity:    parsed.String("--as"),
		output:      output.Format(parsed.String("--output")),
		raw:         parsed.Bool("--raw"),
		inputFile:   parsed.String("--input-file"),
		data:        parsed.String("--data"),
		userIDType:  parsed.String("--user-id-type"),
		userID:      parsed.String("--user-id"),
	}
}

func commonValueFlags(extra ...string) map[string]struct{} {
	flags := map[string]struct{}{
		"--profile":    {},
		"--as":         {},
		"--output":     {},
		"--input-file": {},
		"--data":       {},
	}
	for _, flagName := range extra {
		flags[flagName] = struct{}{}
	}
	return flags
}

func structuredValueFlags(extra ...string) map[string]struct{} {
	flags := commonValueFlags(extra...)
	flags["--user-id-type"] = struct{}{}
	flags["--user-id"] = struct{}{}
	return flags
}

func commonBoolFlags(extra ...string) map[string]struct{} {
	flags := map[string]struct{}{
		"--raw": {},
	}
	for _, flagName := range extra {
		flags[flagName] = struct{}{}
	}
	return flags
}

func (a *App) executeOpenPlatformCommand(ctx context.Context, options commandOptions, request openplatform.Request) error {
	client, requestContext, err := a.openPlatformClientAndContextForOptions(options, request.Path, request.IdentityPolicy)
	if err != nil {
		return err
	}

	response, err := client.Do(ctx, requestContext, request)
	if err != nil {
		return err
	}
	return a.renderOpenPlatformResponse(options, response)
}

func (a *App) openPlatformClientAndContextForOptions(options commandOptions, path string, policy openplatform.IdentityPolicy) (*openplatform.Client, openplatform.RequestContext, error) {
	client, requestContext, err := a.openPlatformClientAndContext(options.profileName, options.identity, path, policy)
	if err != nil {
		return nil, openplatform.RequestContext{}, err
	}
	requestContext.CommonQuery = commandCommonQuery(options)
	return client, requestContext, nil
}

func (a *App) openPlatformClientAndContext(profileName, identityArg, path string, policy openplatform.IdentityPolicy) (*openplatform.Client, openplatform.RequestContext, error) {
	a.logger.Info("resolve open platform request context", "profile", emptyFallback(profileName, "<current>"), "path", path, "identity", emptyFallback(identityArg, "<default>"), "policy", policy)

	profile, err := a.store.GetProfile(profileName)
	if err != nil {
		a.logger.Error("load open platform profile failed", "profile", emptyFallback(profileName, "<current>"), "path", path, "error", err.Error())
		return nil, openplatform.RequestContext{}, err
	}

	identity, err := resolveIdentity(profile, identityArg, path, policy)
	if err != nil {
		a.logger.Error("resolve open platform identity failed", "profile", profile.Name, "path", path, "identity", emptyFallback(identityArg, "<default>"), "error", err.Error())
		return nil, openplatform.RequestContext{}, err
	}

	client := openplatform.New(openplatform.Options{
		HTTPClient: a.httpClient,
		Logger:     a.logger,
	})
	requestContext, err := client.RequestContext(profile, identity)
	if err != nil {
		a.logger.Error("build open platform request context failed", "profile", profile.Name, "path", path, "identity", identity, "error", err.Error())
		return nil, openplatform.RequestContext{}, err
	}
	return client, requestContext, nil
}

func (a *App) renderOpenPlatformResponse(options commandOptions, response openplatform.Response) error {
	renderer := output.NewRenderer(a.stdout)
	if options.raw {
		return renderer.RenderRaw(response.Body)
	}
	if len(response.Body) == 0 {
		return nil
	}
	return renderer.Render(options.output, response.Body)
}

func resolveIdentity(profile config.Profile, requested string, path string, policy openplatform.IdentityPolicy) (config.IdentityKind, error) {
	if strings.TrimSpace(requested) == "" {
		if policy == openplatform.IdentityPolicyUserOnly {
			return config.IdentityUser, nil
		}
		identity := defaultIdentity(profile)
		if policy == openplatform.IdentityPolicyBotOnly && identity != config.IdentityBot {
			return "", fmt.Errorf("open platform path %q only supports --as bot", path)
		}
		return identity, nil
	}

	identity, err := config.ParseIdentityKind(requested)
	if err != nil {
		return "", err
	}
	if policy == openplatform.IdentityPolicyUserOnly && identity != config.IdentityUser {
		return "", fmt.Errorf("open platform path %q only supports --as user", path)
	}
	if policy == openplatform.IdentityPolicyBotOnly && identity != config.IdentityBot {
		return "", fmt.Errorf("open platform path %q only supports --as bot", path)
	}
	return identity, nil
}

func resolveRawBody(options commandOptions) ([]byte, error) {
	switch {
	case options.inputFile != "" && options.data != "":
		return nil, fmt.Errorf("only one of --input-file or --data may be provided")
	case options.inputFile != "":
		body, err := os.ReadFile(options.inputFile)
		if err != nil {
			return nil, fmt.Errorf("read command input file: %w", err)
		}
		return body, nil
	case options.data != "":
		return []byte(options.data), nil
	default:
		return nil, nil
	}
}

func commandCommonQuery(options commandOptions) url.Values {
	query := url.Values{}
	if value := strings.TrimSpace(options.userIDType); value != "" {
		query.Set("user_id_type", value)
	}
	if value := strings.TrimSpace(options.userID); value != "" {
		query.Set("user_id", value)
	}
	if len(query) == 0 {
		return nil
	}
	return query
}

func resolveJSONObjectBody(options commandOptions, requireInput bool) (map[string]any, error) {
	body, err := resolveRawBody(options)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		if requireInput {
			return nil, fmt.Errorf("one of --input-file or --data is required")
		}
		return map[string]any{}, nil
	}

	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode command input json: %w", err)
	}
	object, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("command input must be a JSON object")
	}
	return object, nil
}

func marshalJSONObject(value map[string]any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal command json body: %w", err)
	}
	return data, nil
}
