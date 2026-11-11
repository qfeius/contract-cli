package cli

import "strings"

const redactedArgumentValue = "[REDACTED]"

var sensitiveArgumentNames = map[string]struct{}{
	"access-token":  {},
	"api-key":       {},
	"app-secret":    {},
	"authorization": {},
	"client-secret": {},
	"comment":       {},
	"cookie":        {},
	"data":          {},
	"file-url":      {},
	"name":          {},
	"department-id": {},
	"page-token":    {},
	"header":        {},
	"password":      {},
	"private-key":   {},
	"query":         {},
	"refresh-token": {},
	"secret":        {},
	"token":         {},
}

func redactCommandArgs(args []string) string {
	redacted := make([]string, 0, len(args))
	redactNext := false

	for _, arg := range args {
		if redactNext {
			redacted = append(redacted, redactedArgumentValue)
			redactNext = false
			continue
		}

		name, _, hasInlineValue := strings.Cut(arg, "=")
		if !isSensitiveArgument(name) {
			redacted = append(redacted, arg)
			continue
		}

		if hasInlineValue {
			redacted = append(redacted, name+"="+redactedArgumentValue)
			continue
		}

		redacted = append(redacted, arg)
		redactNext = true
	}

	return strings.Join(redacted, " ")
}

func isSensitiveArgument(name string) bool {
	name = strings.ToLower(strings.TrimLeft(name, "-"))
	name = strings.ReplaceAll(name, "_", "-")
	_, sensitive := sensitiveArgumentNames[name]
	return sensitive
}
