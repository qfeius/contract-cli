package cli

import (
	"flag"
	"fmt"

	"cn.qfei/contract-cli/internal/output"
)

func registerJSONOutputFlag(flags *flag.FlagSet) *string {
	return flags.String("output", "", "output format: json; omitted keeps the default format")
}

func parseJSONOutputFormat(value string) (bool, error) {
	switch value {
	case "":
		return false, nil
	case "json":
		return true, nil
	default:
		return false, fmt.Errorf("unsupported output format %q; use json", value)
	}
}

// --json remains a JSON-output alias only for update check.
func commandRequestsJSON(args []string, options commandLogOptions) bool {
	return options.outputJSON || (len(args) >= 2 && args[0] == "update" && args[1] == "check" && options.jsonAlias)
}

func (a *App) renderJSONCommandResult(data any) error {
	if a.commandContext != nil {
		if err := a.commandContext.Err(); err != nil {
			return err
		}
	}
	return output.NewRenderer(commandResultWriter{ctx: a.commandContext, writer: a.stdout}).WithNotice(a.updateNotice).Render(output.FormatJSON, map[string]any{
		"ok": true, "data": data,
	})
}

// Optional safe, structured context on command failures; the original error
// remains available through Unwrap for cancellation and recovery checks.
type commandFailureDetailer interface {
	commandFailureDetails() any
}
