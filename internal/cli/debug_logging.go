package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
)

type commandLogOptions struct {
	debug      bool
	json       bool
	jsonAlias  bool
	outputJSON bool
}

// parseCommandLogOptions 只移除全局调试开关；原命令仍负责业务参数校验。
// 其他带值参数整体跳过，避免把参数值中的 --debug 识别为开关。
func parseCommandLogOptions(args []string) ([]string, commandLogOptions, error) {
	clean := make([]string, 0, len(args))
	options := commandLogOptions{}
	var parseErr error
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			clean = append(clean, args[i:]...)
			break
		}
		name, value, inline := strings.Cut(arg, "=")
		if name == "--debug" {
			on := true
			if inline {
				var err error
				on, err = strconv.ParseBool(value)
				if err != nil {
					if parseErr == nil {
						parseErr = fmt.Errorf("invalid boolean value for --debug: %w", err)
					}
					continue
				}
			}
			options.debug = on
			continue
		}
		clean = append(clean, arg)
		if name == "--json" {
			on, _ := strconv.ParseBool(value) // 原命令继续校验非法值。
			options.json = !inline || on
			options.jsonAlias = options.json
		}
		if !isCommandValueFlag(name, clean) || isCommandBooleanFlag(name) {
			continue
		}
		if !inline && i+1 < len(args) {
			i++
			value = args[i]
			clean = append(clean, value)
		}
		if name == "--output" || name == "-output" {
			options.json = value == "json"
			options.outputJSON = options.json
		}
	}
	return clean, options, parseErr
}

func isCommandValueFlag(name string, args []string) bool {
	if strings.HasPrefix(name, "--") {
		return true
	}
	if !strings.HasPrefix(name, "-") || len(args) == 0 {
		return false
	}
	// 业务解析器将 -legacy 一类 token 视为位置参数；仅标准 flag 命令支持单横线选项。
	switch args[0] {
	case "auth", "config", "skills", "environment":
		return true
	default:
		return false
	}
}

// 单横线形式用于 Go flag 子命令；保留其原有参数元数，不扩展业务解析语义。
func isCommandBooleanFlag(name string) bool {
	switch strings.TrimLeft(name, "-") {
	case "raw", "force", "full-text", "no-open-browser", "restart", "include-processes", "json", "help", "h", "version", "v", "stdin", "dry-run", "print-input", "yes", "enabled":
		return true
	default:
		return false
	}
}

// 各下游共享稳定的 handler；每次 Run 原子替换不可变策略，避免改绑认证和网络客户端。
// App 本身仍按既有约定串行调用 Run；原子指针保证 slog handler 的并发读取安全。
type commandLogPolicy struct {
	current atomic.Pointer[commandLogOptions]
}

type commandLogHandler struct {
	policy     *commandLogPolicy
	text, json slog.Handler
}

func newCommandLogger(writer io.Writer) (*slog.Logger, *commandLogPolicy) {
	policy := &commandLogPolicy{}
	handler := commandLogHandler{policy: policy,
		text: slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug}),
		json: slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug}),
	}
	return slog.New(handler), policy
}

func (h commandLogHandler) selected() slog.Handler {
	options := h.policy.current.Load()
	if options == nil || !options.debug {
		return nil
	}
	if options.json {
		return h.json
	}
	return h.text
}

func (h commandLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	selected := h.selected()
	return selected != nil && selected.Enabled(ctx, level)
}

func (h commandLogHandler) Handle(ctx context.Context, record slog.Record) error {
	if selected := h.selected(); selected != nil {
		return selected.Handle(ctx, record)
	}
	return nil
}

func (h commandLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return commandLogHandler{policy: h.policy, text: h.text.WithAttrs(attrs), json: h.json.WithAttrs(attrs)}
}

func (h commandLogHandler) WithGroup(name string) slog.Handler {
	return commandLogHandler{policy: h.policy, text: h.text.WithGroup(name), json: h.json.WithGroup(name)}
}
