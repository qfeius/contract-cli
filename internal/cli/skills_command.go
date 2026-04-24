package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
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

func (a *App) runSkills(_ context.Context, args []string) error {
	a.logger.Info("skills command started", "args", strings.Join(args, " "))

	if len(args) == 0 {
		return errors.New("missing skills subcommand")
	}

	switch args[0] {
	case "list":
		return a.runSkillsList(args[1:])
	case "install":
		return a.runSkillsInstall(args[1:])
	default:
		return fmt.Errorf("unknown skills subcommand %q", args[0])
	}
}

func (a *App) runSkillsList(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("skills list does not accept arguments")
	}

	a.logger.Info("skills list started")
	skills, err := loadBundledSkills(a.skillsFS)
	if err != nil {
		a.logger.Error("skills list failed", "error", err.Error())
		return err
	}

	_, _ = fmt.Fprintln(a.stdout, "Built-in skills:")
	for _, skill := range skills {
		_, _ = fmt.Fprintf(a.stdout, "%s\t%s\t%s\n", skill.Name, skill.Version, skill.Description)
	}
	return nil
}

func (a *App) runSkillsInstall(args []string) error {
	flags := flag.NewFlagSet("skills install", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var target string
	var force bool
	flags.StringVar(&target, "target", "", "Codex skills target directory")
	flags.BoolVar(&force, "force", false, "overwrite existing installed skills")

	if err := flags.Parse(args); err != nil {
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
	a.logger.Info("skills install started", "target", resolvedTarget, "force", force)

	skills, err := loadBundledSkills(a.skillsFS)
	if err != nil {
		a.logger.Error("load bundled skills failed", "target", resolvedTarget, "error", err.Error())
		return err
	}
	if err := os.MkdirAll(resolvedTarget, 0o755); err != nil {
		a.logger.Error("create skills target failed", "target", resolvedTarget, "error", err.Error())
		return fmt.Errorf("create skills target: %w", err)
	}

	installed := 0
	skipped := 0
	for _, skill := range skills {
		destination := filepath.Join(resolvedTarget, skill.Dir)
		if _, err := os.Stat(destination); err == nil {
			if !force {
				skipped++
				_, _ = fmt.Fprintf(a.stdout, "Skipped existing skill: %s\n", skill.Dir)
				continue
			}
			if err := os.RemoveAll(destination); err != nil {
				a.logger.Error("remove existing skill failed", "skill", skill.Dir, "target", destination, "error", err.Error())
				return fmt.Errorf("remove existing skill %q: %w", skill.Dir, err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			a.logger.Error("stat skill target failed", "skill", skill.Dir, "target", destination, "error", err.Error())
			return fmt.Errorf("stat skill target %q: %w", skill.Dir, err)
		}

		if err := copySkillDir(a.skillsFS, skill.Dir, destination); err != nil {
			a.logger.Error("install skill failed", "skill", skill.Dir, "target", destination, "error", err.Error())
			return err
		}
		installed++
		_, _ = fmt.Fprintf(a.stdout, "Installed skill: %s\n", skill.Dir)
	}

	_, _ = fmt.Fprintf(a.stdout, "Skills target: %s\n", resolvedTarget)
	_, _ = fmt.Fprintf(a.stdout, "Installed: %d, skipped: %d\n", installed, skipped)
	a.logger.Info("skills install completed", "target", resolvedTarget, "installed", installed, "skipped", skipped)
	return nil
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
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, fmt.Errorf("read bundled skills: %w", err)
	}

	var skills []skillMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		if isHiddenBundledSkill(dir) {
			continue
		}
		content, err := fs.ReadFile(source, path.Join(dir, "SKILL.md"))
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

func copySkillDir(source fs.FS, sourceDir string, destination string) error {
	return fs.WalkDir(source, sourceDir, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk bundled skill %q: %w", sourceDir, walkErr)
		}

		relative := "."
		if current != sourceDir {
			relative = strings.TrimPrefix(current, sourceDir+"/")
		}
		target := destination
		if relative != "." {
			target = filepath.Join(destination, filepath.FromSlash(relative))
		}

		if entry.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create skill directory %q: %w", target, err)
			}
			return nil
		}

		content, err := fs.ReadFile(source, current)
		if err != nil {
			return fmt.Errorf("read bundled skill file %q: %w", current, err)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return fmt.Errorf("write skill file %q: %w", target, err)
		}
		return nil
	})
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
