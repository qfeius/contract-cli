package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func summaryApp(t *testing.T) (*cli.App, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	state, home := t.TempDir(), t.TempDir()
	var out, stderr bytes.Buffer
	a := cli.New(cli.Options{Store: config.NewStore(state), SkillsFS: testSkillsFS(), Stdout: &out, Stderr: &stderr,
		LookupEnv: func(string) (string, bool) { return "", false }, LocalUserHome: func() (string, error) { return home, nil },
	})
	return a, &out, &stderr, state
}

func requireSummary(t *testing.T, output string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in %s", want, output)
		}
	}
}

func TestSkillResultSummaryNoLocationIsNotFailure(t *testing.T) {
	a, out, _, state := summaryApp(t)
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	requireSummary(t, out.String(), "CLI 实际可执行文件："+executable, "Skill 自动同步：未执行（没有可同步位置）", "已登记的 Skill 目录：0", "文件更新不代表客户端已加载")
	if strings.Contains(out.String(), "Skill 自动同步：失败") {
		t.Fatal("missing target reported as failure")
	}
	if _, err := os.Stat(filepath.Join(state, "skill-locations.json")); !os.IsNotExist(err) {
		t.Fatalf("summary created registry: %v", err)
	}
}

func TestSkillResultSummaryInstallAndAllRegisteredLocations(t *testing.T) {
	a, out, _, _ := summaryApp(t)
	first, second, selected := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b"), filepath.Join(t.TempDir(), "selected")
	for _, target := range []string{first, second} {
		out.Reset()
		if err := a.Run(context.Background(), []string{"skills", "install", "--target", target, "--force"}); err != nil {
			t.Fatal(err)
		}
		requireSummary(t, out.String(), "Skill 安装：成功", "Skill 安装位置："+target, "本次目录登记：成功")
	}
	requireSummary(t, out.String(), "已登记的 Skill 目录：2", first, second, "记录状态：success")
	out.Reset()
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", selected, "--name", "auth", "--force"}); err != nil {
		t.Fatal(err)
	}
	requireSummary(t, out.String(), "本次目录登记：未执行", "已登记的 Skill 目录：2")
	if strings.Contains(out.String(), "本次目录登记：成功") {
		t.Fatal("single Skill claimed full registration")
	}
	// Existing skip semantics must remain unchanged.
	out.Reset()
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", first}); err != nil {
		t.Fatal(err)
	}
	requireSummary(t, out.String(), "Skill 安装：有跳过项", "本次目录登记：未执行", "已登记的 Skill 目录：2")
}

func TestSkillResultSummaryMixedSyncDoesNotHideInvalidTarget(t *testing.T) {
	a, out, stderr, _ := summaryApp(t)
	good, missing := filepath.Join(t.TempDir(), "good"), filepath.Join(t.TempDir(), "missing")
	for _, target := range []string{good, missing} {
		if err := a.Run(context.Background(), []string{"skills", "install", "--target", target, "--force"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(missing, missing+"-backup"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err == nil {
		t.Fatal("partial failure must be reported")
	}
	requireSummary(t, out.String(), "Skill 自动同步：部分失败", "同步成功："+good, "同步失败："+missing, "已登记的 Skill 目录：2", "记录状态：invalid")
	requireSummary(t, stderr.String(), "失败原因：", "恢复结果：", "处理建议：")
	requireSummary(t, out.String(), "若该位置已停用，不要重新安装", "未自动删除登记")
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("summary recreated missing target: %v", err)
	}
}

func TestSkillResultSummaryCorruptRegistryIsNotEmptyOrSuccessful(t *testing.T) {
	a, out, stderr, state := summaryApp(t)
	writeSkillSyncFixture(t, filepath.Join(state, "skill-locations.json"), "bad JSON")
	if err := a.Run(context.Background(), []string{"__internal", "skill-sync-on-install"}); err == nil {
		t.Fatal("expected corrupt registry failure")
	}
	requireSummary(t, out.String()+stderr.String(), "Skill 自动同步：失败", "已登记的 Skill 目录：读取失败", "失败原因：")
	if strings.Contains(stderr.String(), "--target ''") {
		t.Fatal("unreadable registry suggested empty target repair")
	}
	if strings.Contains(out.String(), "已登记的 Skill 目录：0") {
		t.Fatal("unreadable registry reported empty")
	}
	out.Reset()
	stderr.Reset()
	target := filepath.Join(t.TempDir(), "skills")
	if err := a.Run(context.Background(), []string{"skills", "install", "--target", target, "--force"}); err == nil {
		t.Fatal("expected installation failure")
	}
	requireSummary(t, stderr.String(), "Skill 安装：失败", "Skill 安装位置："+target, "失败原因：", "恢复结果：", "处理建议：")
	if strings.Contains(stderr.String(), "--target ''") {
		t.Fatal("unreadable registry suggested empty target repair before install")
	}
	if strings.Contains(out.String(), "本次目录登记：成功") {
		t.Fatal("failed install claimed registration")
	}
}
