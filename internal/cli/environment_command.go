package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"cn.qfei/contract-cli/internal/invocation"
)

func (a *App) runEnvironment(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("missing environment subcommand")
	}

	switch args[0] {
	case "inspect":
		return a.runEnvironmentInspect(ctx, args[1:])
	default:
		return fmt.Errorf("unknown environment subcommand %q", args[0])
	}
}

func (a *App) runEnvironmentInspect(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("environment inspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var maxDepth int
	var output string
	var includeProcesses bool
	flags.IntVar(&maxDepth, "depth", invocation.DefaultMaxDepth, "maximum process ancestry depth")
	flags.StringVar(&output, "output", "text", "output format: text|json")
	flags.BoolVar(&includeProcesses, "include-processes", false, "include the collected process ancestry without command-line arguments")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("environment inspect does not accept positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if maxDepth <= 0 || maxDepth > 128 {
		return errors.New("--depth must be between 1 and 128")
	}

	report := a.inspectEnvironment(ctx, maxDepth)
	if err := ctx.Err(); err != nil {
		return err
	}
	if !includeProcesses {
		report.Processes = nil
	}

	switch strings.ToLower(strings.TrimSpace(output)) {
	case "text":
		return writeEnvironmentReport(a.stdout, report, includeProcesses)
	case "json":
		encoder := json.NewEncoder(a.stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	default:
		return fmt.Errorf("unsupported output format %q; expected text or json", output)
	}
}

func writeEnvironmentReport(writer io.Writer, report invocation.Result, includeProcesses bool) error {
	lines := []string{
		"Channel: " + report.ChannelType,
		"Agent Source: " + report.AgentSourceType,
		"Product: " + report.ProductCode,
		"Evidence: " + report.EvidenceType,
		"Confidence: " + report.Confidence,
		"Platform: " + report.Platform,
		"Detector: " + report.DetectorVersion,
	}
	if report.RuleID != "" {
		lines = append(lines, "Rule: "+report.RuleID)
	}
	lines = append(lines, "Reason: "+report.Reason)
	if report.MatchedProcess != nil {
		lines = append(lines, fmt.Sprintf(
			"Matched Process: depth=%d pid=%d ppid=%d name=%s executable=%s",
			report.MatchedProcess.Depth,
			report.MatchedProcess.PID,
			report.MatchedProcess.PPID,
			emptyFallback(report.MatchedProcess.Name, "<unknown>"),
			emptyFallback(report.MatchedProcess.Executable, "<unknown>"),
		))
	}
	if report.Application != nil {
		if report.Application.BundlePath != "" {
			lines = append(lines, fmt.Sprintf(
				"Application: bundle_id=%s team_id=%s version=%s signature_valid=%t path=%s",
				emptyFallback(report.Application.BundleID, "<unknown>"),
				emptyFallback(report.Application.TeamID, "<unknown>"),
				emptyFallback(report.Application.Version, "<unknown>"),
				report.Application.SignatureValid,
				report.Application.BundlePath,
			))
		} else {
			lines = append(lines, fmt.Sprintf(
				"Application: package_family=%s publisher=%s certificate_sha256=%s signature_valid=%t path=%s",
				emptyFallback(report.Application.PackageFamilyName, "<unknown>"),
				emptyFallback(report.Application.Publisher, "<unknown>"),
				emptyFallback(report.Application.CertificateSHA256, "<unknown>"),
				report.Application.SignatureValid,
				report.Application.ExecutablePath,
			))
		}
	}
	for _, warning := range report.Warnings {
		lines = append(lines, "Warning: "+warning)
	}
	if includeProcesses {
		lines = append(lines, "Processes:")
		for _, current := range report.Processes {
			lines = append(lines, fmt.Sprintf(
				"  depth=%d pid=%d ppid=%d name=%s executable=%s",
				current.Depth,
				current.PID,
				current.PPID,
				emptyFallback(current.Name, "<unknown>"),
				emptyFallback(current.Executable, "<unknown>"),
			))
		}
	}

	_, err := fmt.Fprintln(writer, strings.Join(lines, "\n"))
	return err
}
