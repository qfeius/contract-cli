package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"cn.qfei/contract-cli/internal/build"
	"cn.qfei/contract-cli/internal/skillsync"
)

type skillMetadata struct {
	Dir         string
	Name        string
	Version     string
	Description string
}

var hiddenBundledSkillDirs = map[string]struct{}{
	"contract-cli-api-call": {},
}

func (a *App) runSkills(ctx context.Context, args []string) error {
	a.logger.Info("skills command started", "args", redactCommandArgs(args))

	if len(args) == 0 {
		return errors.New("missing skills subcommand")
	}

	switch args[0] {
	case "list":
		return a.runSkillsList(args[1:])
	case "install":
		return a.runSkillsInstall(ctx, args[1:])
	default:
		return fmt.Errorf("unknown skills subcommand %q", args[0])
	}
}

func (a *App) runSkillsList(args []string) error {
	flags := flag.NewFlagSet("skills list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	format := registerJSONOutputFlag(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	jsonOutput, err := parseJSONOutputFormat(*format)
	if err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("skills list does not accept arguments")
	}

	a.logger.Info("skills list started")
	skills, err := loadBundledSkills(a.skillsFS)
	if err != nil {
		a.logger.Error("skills list failed", "error", err.Error())
		return err
	}

	if jsonOutput {
		return a.renderSkillsJSONList(skills)
	}
	_, _ = fmt.Fprintln(a.stdout, "Built-in skills:")
	cliVersion := build.Current().Version
	for _, skill := range skills {
		_, _ = fmt.Fprintf(a.stdout, "%s\t%s\t%s\n", skill.Name, cliVersion, skill.Description)
	}
	return nil
}

/*
runSkillsInstall 将包内 Skill 安装到目标目录，并可限定只更新一个命名 Skill。
入参 args（[]string）为 install 参数；返回 error 为参数、选择或文件安装错误。
*/
func (a *App) runSkillsInstall(ctx context.Context, args []string) (err error) {
	_, logOptions, _ := parseCommandLogOptions(append([]string{"skills", "install"}, args...))
	jsonOutput := logOptions.outputJSON
	service := a.skillSyncService()
	if !jsonOutput {
		a.printSkillCLIIdentity()
	}
	var target string
	defer func() {
		if jsonOutput {
			if err != nil {
				err = skillJSONFailure(err, service, target)
			}
			return
		}
		if err != nil {
			a.printSkillFailure("Skill 安装", target, err.Error(), skillsync.RecoveryStatus(err))
		}
		a.printRegisteredSkillLocations(service)
	}()
	flags := flag.NewFlagSet("skills install", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var selectedName string
	var force bool
	flags.StringVar(&target, "target", "", "已确认的当前客户端用户 Skill 目录；省略 --target 时默认使用 Codex（CODEX_HOME/skills 或 ~/.codex/skills），不会自动识别当前客户端")
	flags.StringVar(&selectedName, "name", "", "install only the named bundled skill")
	flags.BoolVar(&force, "force", false, "overwrite existing installed skills")
	format := registerJSONOutputFlag(flags)

	if err := flags.Parse(args); err != nil {
		return err
	}
	if _, err := parseJSONOutputFormat(*format); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("skills install does not accept positional arguments")
	}

	resolvedTarget, err := a.resolveSkillsInstallTarget(target)
	if err != nil {
		a.logger.Error("resolve skills install target failed", "target", target, "error", err.Error())
		return err
	}
	target = resolvedTarget
	a.logger.Info("skills install started", "target", resolvedTarget, "name", selectedName, "force", force)

	skills, err := loadBundledSkillsContext(ctx, a.skillsFS)
	if err != nil {
		a.logger.Error("load bundled skills failed", "target", resolvedTarget, "error", err.Error())
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if selectedName != "" {
		// 只接受包内目录名，避免把用户提供的名称直接拼接进目标路径。
		selected := make([]skillMetadata, 0, 1)
		for _, skill := range skills {
			if skill.Dir == selectedName {
				selected = append(selected, skill)
				break
			}
		}
		if len(selected) == 0 {
			return fmt.Errorf("bundled skill %q not found", selectedName)
		}
		skills = selected
	}
	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		names = append(names, skill.Dir)
	}
	result, err := service.InstallContext(ctx, resolvedTarget, names, force, selectedName == "")
	if result.Target != "" {
		resolvedTarget = result.Target
		target = result.Target
	}
	if err != nil {
		return err
	}
	if err := skillInstallationCancellation(ctx, result); err != nil {
		return err
	}
	installed, skipped := len(result.Installed), len(result.Skipped)
	if jsonOutput {
		a.logger.Info("skills install completed", "target", resolvedTarget, "installed", installed, "skipped", skipped)
		return a.renderSkillsJSONInstall(ctx, service, resolvedTarget, result)
	}
	for _, name := range result.Installed {
		_, _ = fmt.Fprintf(a.stdout, "Installed skill: %s\n", name)
	}
	for _, name := range result.Skipped {
		_, _ = fmt.Fprintf(a.stdout, "Skipped existing skill: %s (not synchronized; use --force to overwrite)\n", name)
	}

	_, _ = fmt.Fprintf(a.stdout, "Skills target: %s\n", resolvedTarget)
	_, _ = fmt.Fprintf(a.stdout, "Installed: %d, skipped: %d\n", installed, skipped)
	_, _ = fmt.Fprintf(a.stdout, "Skill 安装位置：%s\nSkill 来源版本：%s\n", resolvedTarget, build.Current().Version)
	if skipped > 0 {
		_, _ = fmt.Fprintln(a.stdout, "Skill 安装：有跳过项；不能认为全部配套 Skill 已更新")
	} else {
		_, _ = fmt.Fprintf(a.stdout, "Skill 安装：成功；安装数量：%d\n", installed)
	}
	if result.Confirmed {
		_, _ = fmt.Fprintln(a.stdout, "本次目录登记：成功；后续全局安装升级将处理此位置")
	} else {
		_, _ = fmt.Fprintln(a.stdout, "本次目录登记：未执行（单个 Skill 或有跳过项不会新增完整登记；已有登记不因此删除）")
	}
	if skipped > 0 {
		_, _ = fmt.Fprintf(a.stdout, "Review local skill changes before synchronizing, then run: contract-cli skills install --target %q --force\n", resolvedTarget)
	}
	a.logger.Info("skills install completed", "target", resolvedTarget, "installed", installed, "skipped", skipped)
	return nil
}

func (a *App) skillSyncService() *skillsync.Service {
	return &skillsync.Service{StateDir: filepath.Dir(a.store.Path()), Source: a.skillsFS, Version: build.Current().Version, Logger: a.logger}
}

func (a *App) runSkillSyncOnInstall(args []string) error {
	if len(args) != 1 || args[0] != "skill-sync-on-install" {
		return errors.New("unknown internal command")
	}
	service := a.skillSyncService()
	a.printSkillCLIIdentity()
	defer a.printRegisteredSkillLocations(service)
	skills, err := loadBundledSkills(a.skillsFS)
	if err != nil {
		a.printSkillFailure("Skill 自动同步", "", err.Error(), skillsync.RecoveryStatus(err))
		return err
	}
	for _, skill := range skills {
		service.Names = append(service.Names, skill.Dir)
	}
	// Only include verified client locations; CODEX_HOME changes Codex, not WorkBuddy.
	var history []string
	home, homeErr := a.localUserHome()
	if codexHome, ok := a.lookupEnv("CODEX_HOME"); ok && strings.TrimSpace(codexHome) != "" {
		history = append(history, filepath.Join(codexHome, "skills"))
	} else if homeErr == nil {
		history = append(history, filepath.Join(home, ".codex", "skills"))
	}
	if runtime.GOOS == "windows" && homeErr == nil {
		// Verified against an installed Windows WorkBuddy 5.7.6 historical bundle.
		history = append(history, filepath.Join(home, ".workbuddy", "skills"))
	}
	if homeErr != nil {
		a.logger.Warn("historical Skill discovery could not resolve operating system user home", "error", homeErr.Error())
		_, _ = fmt.Fprintln(a.stderr, "Cannot locate default historical Skill directories (including historical WorkBuddy on Windows); confirmed locations and an explicit CODEX_HOME will still be synchronized.")
	}
	results, syncErr := service.Sync(history)
	_, _ = fmt.Fprintf(a.stdout, "CLI version: %s; official Skill version: %s\n", service.Version, service.Version)
	if len(results) == 0 && syncErr == nil {
		_, _ = fmt.Fprintln(a.stdout, "CLI installed; no confirmed Skill locations. Skill files were not installed or updated. This is not necessarily a new user: an old installation may be in an unrecognized custom directory.")
		_, _ = fmt.Fprintln(a.stdout, "安装状态：CLI 已安装；配套 Skill 尚未安装或更新。未找到可自动同步的位置，可能是首次安装，也可能是未识别的历史自定义位置，不能据此判定新老用户。")
	}
	failed := false
	succeeded := 0
	unverified := 0
	for _, result := range results {
		_, _ = fmt.Fprintf(a.stdout, "Skill location %s: %s\n", result.Target, result.Status)
		if result.Status == "unverified" {
			unverified++
			_, _ = fmt.Fprintf(a.stdout, "历史 Skill 未自动接管：%s\n原因：%s\nCLI 已安装；该位置未覆盖、未新增登记。请核实当前客户端实际读取的 skills 目录、覆盖范围及备份后，明确执行：%s\n", result.Target, result.Error, skillInstallCommand(result.Target))
			continue
		}
		if result.Status != "success" {
			if result.Status == "invalid" {
				_, _ = fmt.Fprintln(a.stdout, "该登记位置已失效：若仍需使用，请先核实实际目录和备份，再完整安装；若该位置已停用，不要重新安装。未自动删除登记，请联系维护人员核实处理旧记录；其他有效位置不受影响。")
			}
			_, _ = fmt.Fprintf(a.stdout, "同步失败：%s\n", result.Target)
			a.printSkillFailure("Skill 同步", result.Target, result.Error, result.Recovery)
			if skillsync.IsClientManagedSkillTarget(runtime.GOOS, result.Target) {
				_, _ = fmt.Fprintf(a.stderr, "%s; do not reinstall into this SYSTEM directory. Confirm the client's user Skill directory; the old registration is retained and needs explicit review.\n", result.Error)
			} else {
				_, _ = fmt.Fprintf(a.stderr, "%s; repair: %s\n", result.Error, skillInstallCommand(result.Target))
			}
			failed = true
		} else {
			succeeded++
			_, _ = fmt.Fprintf(a.stdout, "同步成功：%s；版本：%s\n", result.Target, service.Version)
		}
	}
	switch {
	case syncErr != nil:
		a.printSkillFailure("Skill 自动同步", "", syncErr.Error(), skillsync.RecoveryStatus(syncErr))
	case failed && succeeded > 0:
		_, _ = fmt.Fprintf(a.stdout, "Skill 自动同步：部分失败；成功：%d；失败：%d；待确认：%d\n", succeeded, len(results)-succeeded-unverified, unverified)
	case failed:
		_, _ = fmt.Fprintln(a.stdout, "Skill 自动同步：失败")
	case unverified > 0:
		_, _ = fmt.Fprintf(a.stdout, "Skill 自动同步：需要确认历史位置；成功：%d；未接管：%d。不能报告全部 Skill 已同步。\n", succeeded, unverified)
	case len(results) == 0:
		_, _ = fmt.Fprintln(a.stdout, "Skill 自动同步：未执行（没有可同步位置）")
	default:
		_, _ = fmt.Fprintf(a.stdout, "Skill 自动同步：成功；位置数量：%d\n", succeeded)
	}
	if succeeded > 0 {
		_, _ = fmt.Fprintln(a.stdout, "Skill files updated where successful; refresh, restart or open a new AI client session to verify loading.")
	}
	if syncErr != nil {
		return syncErr
	}
	codexHome, _ := a.lookupEnv("CODEX_HOME")
	localAppData, _ := a.lookupEnv("LOCALAPPDATA")
	hints := skillsync.ClientSkillHints(runtime.GOOS, home, codexHome, localAppData)
	recommendations := service.Recommendations(hints, results)
	if len(results) == 0 || len(recommendations) > 0 {
		a.printSkillInstallGuidance(recommendations, len(results) > 0)
	}
	if failed {
		return errors.New("CLI installed; some Skill locations did not synchronize")
	}
	return nil
}

func (a *App) printSkillInstallGuidance(hints []skillsync.SkillHint, optional bool) {
	if optional {
		_, _ = fmt.Fprintln(a.stdout, "可选：在其他客户端首次使用。本次自动同步已结束，结果以上述逐目录记录为准；以下候选不是本次升级的必做步骤，也不证明当前客户端缺少 Skill。仅需新增安装位置时继续阅读。")
	} else {
		_, _ = fmt.Fprintln(a.stdout, "下一步：等待用户确认。请 AI 先暂停后续 Skill 安装，不要直接执行下列命令。")
		_, _ = fmt.Fprintln(a.stdout, "WAITING FOR USER CONFIRMATION: pause further Skill installation; do not execute candidate commands yet.")
	}
	_, _ = fmt.Fprintln(a.stdout, "请核实当前客户端实际支持安装、实际读取的 Skill 目录，核实后向用户推荐具体目标，说明推荐理由、全部官方 Skill 的覆盖范围及备份结果；得到用户明确确认后再完整安装。")
	_, _ = fmt.Fprintln(a.stdout, "不能仅凭聊天名称、目录名、候选顺序或 CODEX_HOME 存在推断当前客户端；无法核实则停止，并向用户说明还需要哪些信息。")
	_, _ = fmt.Fprintln(a.stdout, "AI: verify the actual client and its installable user Skill directory before recommending a target. Explain the evidence, overwrite scope and verified backup, then wait for explicit user approval. If verification is unavailable, stop and ask; there is no default recommended client.")
	_, _ = fmt.Fprintln(a.stdout, "Skill directory candidates (not synchronized; confirmation required). These do not identify the AI client executing this installation.")
	for _, hint := range hints {
		_, _ = fmt.Fprintf(a.stdout, "  %s: %s [%s]\n  After confirmation only: %s\n", hint.Client, hint.Target, hint.Basis, skillInstallCommand(hint.Target))
	}
	if len(hints) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "No verified candidate was found. Ask the current client to confirm its configured user Skill directory; do not invent a path.")
	}
	_, _ = fmt.Fprintln(a.stdout, "Do not omit --target for an unidentified client (the existing default is Codex). Do not use a system-managed Skill directory. Do not run every candidate command.")
	_, _ = fmt.Fprintln(a.stdout, "候选仅供确认，不会自动安装或登记；不要逐个执行全部候选。目录为空或不存在时应记录证据，不能把 CLI 安装成功直接报告为当前客户端 Skill 已安装。")
	_, _ = fmt.Fprintln(a.stdout, "此处的文字提示不是交互授权锁，CLI 无法核实聊天中的同意；执行方负责等待确认。这次 CLI 安装已结束，尚未确认的 Skill 安装不能报告为完成。")
	_, _ = fmt.Fprintln(a.stdout, "After a successful complete installation, the location is registered for future global upgrades. Refresh or open a new client session to verify actual Skill loading; manual installation is not evidence that the original automatic synchronization succeeded.")
}

func skillInstallCommand(target string) string {
	executable, err := os.Executable()
	if err != nil {
		executable = "contract-cli"
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	prefix := ""
	if runtime.GOOS == "windows" {
		quote = func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
		prefix = "& "
		powershell := "PowerShell: " + prefix + quote(executable) + " skills install --target " + quote(target) + " --force"
		// CMD expands percent variables and may expand exclamation marks even inside quotes.
		if strings.ContainsAny(executable+target, "%!\"\r\n") {
			return powershell + "\nCMD: 路径含特殊字符，请切换到 PowerShell 使用上面的命令。"
		}
		return powershell + "\nCMD: \"" + executable + "\" skills install --target \"" + target + "\" --force"
	}
	return prefix + quote(executable) + " skills install --target " + quote(target) + " --force"
}

func (a *App) warnPendingSkillSync(jsonOutput ...bool) {
	for _, location := range a.skillSyncService().Pending() {
		var message string
		if location.Target == "" {
			message = fmt.Sprintf("Official Skill synchronization incomplete: %s. 登记读取失败，请核实登记文件和权限；无法生成具体目录修复命令。", location.Error)
		} else if skillsync.IsClientManagedSkillTarget(runtime.GOOS, location.Target) {
			message = fmt.Sprintf("Official Skill synchronization incomplete: %s. Do not reinstall into this SYSTEM directory; confirm the client's user Skill directory and explicitly review the old registration.", location.Error)
		} else {
			message = fmt.Sprintf("Official Skill synchronization incomplete: %s. Repair: %s", location.Error, skillInstallCommand(location.Target))
		}
		if len(jsonOutput) > 0 && jsonOutput[0] {
			encoder := json.NewEncoder(a.stderr)
			encoder.SetEscapeHTML(false)
			_ = encoder.Encode(map[string]string{"level": "WARN", "msg": message})
			continue
		}
		_, _ = fmt.Fprintln(a.stderr, message)
	}
}

func (a *App) resolveSkillsInstallTarget(target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		if codexHome, ok := a.lookupEnv("CODEX_HOME"); ok && strings.TrimSpace(codexHome) != "" {
			return filepath.Join(codexHome, "skills"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		return filepath.Join(home, ".codex", "skills"), nil
	}
	expanded, err := expandHomePath(target)
	if err != nil {
		return "", err
	}
	return expanded, nil
}

func loadBundledSkills(source fs.FS) ([]skillMetadata, error) {
	return loadBundledSkillsContext(context.Background(), source)
}

func loadBundledSkillsContext(ctx context.Context, source fs.FS) ([]skillMetadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(source, ".")
	if ctx.Err() != nil {
		return nil, errors.Join(err, ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("read bundled skills: %w", err)
	}

	var skills []skillMetadata
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		if isHiddenBundledSkill(dir) {
			continue
		}
		content, err := fs.ReadFile(source, path.Join(dir, "SKILL.md"))
		if ctx.Err() != nil {
			return nil, errors.Join(err, ctx.Err())
		}
		if err != nil {
			continue
		}
		metadata := parseSkillMetadata(dir, string(content))
		skills = append(skills, metadata)
	}
	if len(skills) == 0 {
		return nil, fmt.Errorf("no bundled skills found")
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

func isHiddenBundledSkill(dir string) bool {
	_, hidden := hiddenBundledSkillDirs[dir]
	return hidden
}

func parseSkillMetadata(dir string, content string) skillMetadata {
	metadata := skillMetadata{
		Dir:     dir,
		Name:    dir,
		Version: "unknown",
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return metadata
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			if value != "" {
				metadata.Name = value
			}
		case "version":
			if value != "" {
				metadata.Version = value
			}
		case "description":
			metadata.Description = value
		}
	}
	return metadata
}

func expandHomePath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		if trimmed == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(trimmed, "~/")), nil
	}
	return trimmed, nil
}
