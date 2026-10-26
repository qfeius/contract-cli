package skillsync

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SkillHint describes a candidate, not a detected caller or a confirmed installation.
type SkillHint struct {
	Client     string
	Root       string
	Target     string
	Configured bool
	Basis      string
}

// ClientSkillHints only plans known paths. It never reads private client settings
// or treats a default path as proof of the client's current configuration.
func ClientSkillHints(platform, home, codexHome, _ string) []SkillHint {
	var hints []SkillHint
	if strings.TrimSpace(codexHome) != "" {
		if filepath.IsAbs(codexHome) {
			hints = append(hints, SkillHint{Client: "Codex", Root: codexHome, Target: filepath.Join(codexHome, "skills"), Configured: true, Basis: "CODEX_HOME"})
		}
	} else if filepath.IsAbs(home) {
		root := filepath.Join(home, ".codex")
		hints = append(hints, SkillHint{Client: "Codex", Root: root, Target: filepath.Join(root, "skills"), Basis: "known default; verify current client configuration"})
	}
	if platform != "windows" {
		return hints
	}
	if filepath.IsAbs(home) {
		root := filepath.Join(home, ".workbuddy")
		hints = append(hints, SkillHint{Client: "WorkBuddy", Root: root, Target: filepath.Join(root, "skills"), Basis: "Windows default; verify current client configuration"})
	}
	// Doubao's actual installable directory must be verified by the client;
	// a profile path or an existing .user_skills directory is not configuration evidence.
	return hints
}

// IsClientManagedSkillTarget identifies the Windows Doubao SYSTEM directory
// verified by the client test. Arbitrary directories named .skills are not blocked.
func IsClientManagedSkillTarget(platform, target string) bool {
	if platform != "windows" {
		return false
	}
	p := strings.ToLower(strings.ReplaceAll(filepath.Clean(target), "\\", "/"))
	return strings.HasSuffix(p, "/.doubao/agent_mode/workspace/.skills")
}

// Recommendations checks candidates without creating directories or changing
// registration. The caller must still confirm ownership, backup and client loading.
func (s *Service) Recommendations(hints []SkillHint, processed []Result) []SkillHint {
	s.log().Info("Skill directory recommendation started", "candidates", len(hints))
	seen := map[string]bool{}
	for _, result := range processed {
		if target, err := canonical(result.Target); err == nil {
			seen[targetKey(target)] = true
		}
	}
	var result []SkillHint
	for _, hint := range hints {
		if !filepath.IsAbs(hint.Target) {
			s.log().Warn("Skill directory candidate rejected", "client", hint.Client, "reason", "not absolute")
			continue
		}
		target, err := s.target(hint.Target)
		if err != nil {
			s.log().Warn("Skill directory candidate rejected", "client", hint.Client, "location", targetKey(hint.Target), "reason", "unsafe or unavailable target")
			continue
		}
		key := targetKey(target)
		if seen[key] {
			continue
		}
		if !hint.Configured {
			info, err := os.Stat(hint.Root)
			if err != nil || !info.IsDir() {
				s.log().Debug("Default Skill directory candidate omitted", "client", hint.Client, "reason", "client root unavailable")
				continue
			}
		}
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			s.log().Warn("Skill directory candidate rejected", "client", hint.Client, "reason", "target is not a directory")
			continue
		} else if err != nil && !os.IsNotExist(err) {
			s.log().Warn("Skill directory candidate rejected", "client", hint.Client, "reason", "target unavailable")
			continue
		}
		hint.Target = target
		result = append(result, hint)
		seen[key] = true
	}
	s.log().Info("Skill directory recommendation completed", "candidates", len(result), "platform", runtime.GOOS)
	return result
}
