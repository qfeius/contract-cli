package cli_test

import (
	"context"
	"strings"
	"testing"
)

func TestPreAuditPublicHelpAndSkillUseRealPreAuditFlow(t *testing.T) {
	for _, action := range []string{"prepare", "create", "start", "resume"} {
		app, out := alignmentApp(t, nil)
		if err := app.Run(context.Background(), []string{"review", "pre-audit", action, "--help"}); err != nil {
			t.Fatal("public preaudit help missing", action, err)
		}
		if !strings.Contains(out.String(), "review pre-audit "+action) || !strings.Contains(out.String(), "--profile") {
			t.Fatal("public help missing syntax/common flags", out.String())
		}
	}
	content := reviewSkill(t, "contract-cli-review")
	for _, rule := range []string{"review pre-audit prepare", "review pre-audit create", "review pre-audit start", "review pre-audit resume", "SMART_PRE_AUDIT", "awaiting_category", "final.reviewDetailUrl", "链接失败", "启动结果未知", "不自动重发", "category-id"} {
		if !strings.Contains(content, rule) {
			t.Errorf("bundled Skill lacks preaudit contract: %s", rule)
		}
	}
	for _, obsolete := range []string{"不要求先创建合同、预审单或协商", "分类仅用于清单筛选和结果展示", "独立预审无平台页面"} {
		if strings.Contains(content, obsolete) {
			t.Errorf("bundled Skill retains obsolete local file flow: %s", obsolete)
		}
	}
	prepare := strings.Index(content, "contract-cli review pre-audit prepare")
	category := strings.Index(content, "contract-cli contract category list")
	create := strings.Index(content, "contract-cli review pre-audit create")
	if prepare < 0 || category <= prepare || create <= category {
		t.Fatal("Skill must upload first, then choose category and create preaudit")
	}
}

func TestPreAuditCreateHelpExplainsCategoryReselectionAndUnknownOutcome(t *testing.T) {
	app, out := alignmentApp(t, nil)
	if err := app.Run(context.Background(), []string{"review", "pre-audit", "create", "--help"}); err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{out.String(), reviewSkill(t, "contract-cli-review")} {
		if strings.Contains(content, "request_id") || strings.Contains(content, "恢复原CLM创建") {
			t.Error("help/Skill must not advertise withdrawn creation idempotency or recovery")
		}
		for _, rule := range []string{"首次创建明确拒绝", "awaiting_category", "创建结果未知", "不支持重试恢复", "人工核实"} {
			if !strings.Contains(content, rule) {
				t.Errorf("create help/Skill must explain category recovery: %s", rule)
			}
		}
	}
}
