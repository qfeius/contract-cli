package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/skillsync"
)

func TestSkillInstallGuidanceDoesNotAssumeNoLocationsMeansNewUser(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	var out, stderr bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &out, Stderr: &stderr,
		LookupEnv: func(string) (string, bool) { return "", false }, LocalUserHome: func() (string, error) { return home, nil },
	})
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"no confirmed Skill locations", "not installed or updated", "not necessarily a new user", "user Skill directory", "Do not omit --target", "Do not use a system-managed Skill directory"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "First install: contract-cli skills install --force") {
		t.Fatal("guidance still silently defaults unknown clients to Codex")
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Fatalf("guidance created a client directory: %v", err)
	}
}

func TestSkillInstallGuidanceNeverOffersRepairIntoSystemDirectory(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows system directory only")
	}
	home, state := t.TempDir(), t.TempDir()
	target := filepath.Join(home, ".doubao", "agent_mode", "workspace", ".skills")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	r := skillsync.Registry{Schema: 1, Locations: []skillsync.Location{{Target: target, ID: "stale-system", Status: "failed", Error: "client-managed system Skill directory", Managed: []string{"auth"}}}}
	content, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	writeSkillSyncFixture(t, filepath.Join(state, "skill-locations.json"), string(content))
	var stderr bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &bytes.Buffer{}, Stderr: &stderr,
		LookupEnv: func(string) (string, bool) { return "", false }, LocalUserHome: func() (string, error) { return home, nil },
	})
	if err := a.Run(context.Background(), []string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "--force") || !strings.Contains(stderr.String(), "user Skill directory") {
		t.Fatalf("unsafe pending repair instruction: %s", stderr.String())
	}
}

func TestSkillInstallGuidanceSuggestsConfiguredTargetWithShellSafeCommand(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	configured := filepath.Join(home, "codex $HOME 'quote' spaced")
	var out bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &out, Stderr: &bytes.Buffer{},
		LookupEnv: func(key string) (string, bool) { return configured, key == "CODEX_HOME" }, LocalUserHome: func() (string, error) { return home, nil },
	})
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Codex") || !strings.Contains(out.String(), "CODEX_HOME") || !strings.Contains(out.String(), "confirmation required") {
		t.Fatalf("configured candidate missing: %s", out.String())
	}
	want := strings.ReplaceAll(filepath.Join(configured, "skills"), "'", "'\"'\"'")
	if runtime.GOOS == "windows" {
		want = strings.ReplaceAll(filepath.Join(configured, "skills"), "'", "''")
	}
	if !strings.Contains(out.String(), "--target '"+want+"' --force") {
		t.Fatalf("unsafe/unusable target command: %s", out.String())
	}
	if runtime.GOOS == "windows" && (!strings.Contains(out.String(), "PowerShell:") || !strings.Contains(out.String(), "CMD:")) {
		t.Fatalf("Windows commands must identify both shells: %s", out.String())
	}
	if _, err := os.Stat(configured); !os.IsNotExist(err) {
		t.Fatalf("candidate was installed without confirmation: %v", err)
	}
}

func TestUpgradeCandidatesAreOptional(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &out, Stderr: &bytes.Buffer{}, LookupEnv: func(string) (string, bool) { return "", false }, LocalUserHome: func() (string, error) { return home, nil }})
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", filepath.Join(home, "another-client", "skills"), "--force"}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "可选：在其他客户端首次使用") || strings.Contains(out.String(), "下一步：等待用户确认") {
		t.Fatal(out.String())
	}
}

func TestSkillInstallGuidanceDoesNotGuessDoubaoDirectoryWhenAnotherSynced(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows default clients only")
	}
	home, state, local := t.TempDir(), t.TempDir(), t.TempDir()
	codex := filepath.Join(home, ".codex", "skills")
	user := filepath.Join(local, "Doubao", "User Data", "Default", ".doubao", "agent_mode", "workspace", ".user_skills")
	if err := os.MkdirAll(user, 0755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &out, Stderr: &bytes.Buffer{},
		LookupEnv: func(key string) (string, bool) { return local, key == "LOCALAPPDATA" }, LocalUserHome: func() (string, error) { return home, nil },
	})
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", codex, "--force"}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), user) || strings.Contains(out.String(), ".user_skills") {
		t.Fatalf("unverified Doubao directory recommended: %s", out.String())
	}
	if strings.Contains(out.String(), "workspace\\.skills'") {
		t.Fatalf("system target recommended: %s", out.String())
	}
	r := readSkillSyncRegistry(t, filepath.Join(state, "skill-locations.json"))
	if len(r.Locations) != 1 || r.Locations[0].Target != codex {
		t.Fatalf("recommendation implicitly registered or installed: %+v", r)
	}
}

func TestSkillInstallGuidanceDoesNotOfferFirstInstallOnCorruptRegistry(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	writeSkillSyncFixture(t, filepath.Join(state, "skill-locations.json"), "invalid JSON")
	var out bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &out, Stderr: &bytes.Buffer{},
		LookupEnv: func(string) (string, bool) { return "", false }, LocalUserHome: func() (string, error) { return home, nil },
	})
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err == nil {
		t.Fatal("corrupt registry reported as first install")
	}
	if strings.Contains(out.String(), "no confirmed Skill locations") || strings.Contains(out.String(), "First install:") {
		t.Fatalf("registry failure disguised as no locations: %s", out.String())
	}
}

func TestSkillInstallGuidanceWaitsForVerifiedRecommendationBeforeShowingCommands(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	configured := filepath.Join(home, "configured-codex")
	var out bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &out, Stderr: &bytes.Buffer{},
		LookupEnv: func(key string) (string, bool) { return configured, key == "CODEX_HOME" }, LocalUserHome: func() (string, error) { return home, nil },
	})
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"CLI 已安装；配套 Skill 尚未安装或更新", "下一步：等待用户确认", "核实后向用户推荐", "推荐理由", "聊天名称", "无法核实则停止", "文字提示不是交互授权锁"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in %s", want, output)
		}
	}
	if !strings.Contains(output, "当前客户端实际支持安装、实际读取的 Skill 目录") {
		t.Errorf("missing actual directory verification guidance: %s", output)
	}
	confirmation := strings.Index(output, "下一步：等待用户确认")
	command := strings.Index(output, "After confirmation only:")
	if confirmation < 0 || command < confirmation {
		t.Errorf("command appeared before the confirmation warning: %s", output)
	}
	if strings.Contains(output, "推荐：Codex") || strings.Contains(output, "推荐（当前默认）") {
		t.Errorf("CLI blindly recommended the invoking client: %s", output)
	}
	for _, target := range []string{configured, filepath.Join(state, "skill-locations.json")} {
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Errorf("guidance created unconfirmed state at %s: %v", target, err)
		}
	}
}

func TestSkillInstallGuidanceDoesNotSayAllSkillsMissingWhenAnotherLocationSynced(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	configured := filepath.Join(home, "configured-codex")
	registered := filepath.Join(t.TempDir(), "other-client", "skills")
	var out bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &out, Stderr: &bytes.Buffer{},
		LookupEnv: func(key string) (string, bool) { return configured, key == "CODEX_HOME" }, LocalUserHome: func() (string, error) { return home, nil },
	})
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", registered, "--force"}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "Skill location "+registered+": success") || !strings.Contains(output, "可选：在其他客户端首次使用") {
		t.Fatalf("sync result or independent confirmation missing: %s", output)
	}
	if strings.Contains(output, "配套 Skill 尚未安装或更新") {
		t.Fatalf("successful synchronization reported as all Skills missing: %s", output)
	}
	r := readSkillSyncRegistry(t, filepath.Join(state, "skill-locations.json"))
	if len(r.Locations) != 1 || r.Locations[0].Target != registered {
		t.Fatalf("guidance changed confirmed locations: %+v", r)
	}
}
