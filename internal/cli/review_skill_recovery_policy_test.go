package cli_test

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func reviewRecoveryPolicy(t *testing.T) string {
	t.Helper()
	content := reviewSkill(t, "contract-cli-review")
	start := strings.Index(content, "### 两轮审查与请求重试")
	if start < 0 {
		t.Fatal("published review skill has no two-round recovery policy")
	}
	end := strings.Index(content[start:], "### 结果输出前置准备与连续输出规则")
	if end < 0 {
		t.Fatal("recovery policy must preserve the existing result output section")
	}
	return content[start : start+end]
}

func recoveryPolicyRows(section string) map[string]string {
	rows := make(map[string]string)
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 3 {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows[cells[0]+"/"+cells[1]] = cells[2]
	}
	return rows
}

// 决策表是发布给Agent的流程契约；验证轮次/动作，不测试运行时自动调度器。
func TestReviewSkillRecoveryRoundDecisions(t *testing.T) {
	policy := reviewRecoveryPolicy(t)
	rows := recoveryPolicyRows(policy)
	for _, tc := range []struct {
		condition string
		actions   []string
	}{
		{"第一轮/success 且详情完整", []string{"输出", "结束"}},
		{"第一轮/running", []string{"第二轮", "原 taskId", "续查"}},
		{"第一轮/success 但详情未就绪", []string{"第二轮", "原 taskId", "详情", "不重审"}},
		{"第一轮/fail 或其他明确审查状态", []string{"第二轮", "原业务对象", "重新发起一次"}},
		{"第二轮/任意状态、结果或错误", []string{"实际", "结束", "不触发第三轮"}},
	} {
		t.Run(tc.condition, func(t *testing.T) {
			for _, action := range tc.actions {
				if !strings.Contains(rows[tc.condition], action) {
					t.Errorf("decision %s lacks action %q: %q", tc.condition, action, rows[tc.condition])
				}
			}
		})
	}
	for _, invariant := range []string{"外部文件预审", "合同 ID", "最多两轮", "业务异常优先于", "不重新上传", "不重新创建预审"} {
		if !strings.Contains(policy, invariant) {
			t.Errorf("recovery policy lacks boundary %q", invariant)
		}
	}
	content := reviewSkill(t, "contract-cli-review")
	for _, stale := range []string{
		"已知taskId只续查原任务。",
		"明确失败或取消终态则停止",
		"只有用户明确要求再次发起才使用原run执行start",
		"已取得 taskId：继续查询原任务，不因查询超时、连接中断或链接缺失重新创建。",
	} {
		if strings.Contains(content, stale) {
			t.Errorf("old recovery directive conflicts with the two-round policy: %s", stale)
		}
	}
}

func TestReviewSkillRecoveryRequestFailuresStayWithinRound(t *testing.T) {
	policy := reviewRecoveryPolicy(t)
	rows := recoveryPolicyRows(policy)
	for _, tc := range []struct {
		condition string
		actions   []string
	}{
		{"请求异常/HTTP 429", []string{"Retry-After", "增大", "同轮", "不触发第二轮"}},
		{"请求异常/查询网络超时", []string{"等待", "原请求", "同轮", "不触发第二轮"}},
		{"请求异常/鉴权、权限、参数等业务异常", []string{"停止", "原因", "不触发第二轮"}},
		{"请求异常/启动超时且没有 taskId", []string{"未知", "核实", "不自动重发", "不触发第二轮"}},
	} {
		t.Run(tc.condition, func(t *testing.T) {
			for _, action := range tc.actions {
				if !strings.Contains(rows[tc.condition], action) {
					t.Errorf("request failure %s lacks action %q: %q", tc.condition, action, rows[tc.condition])
				}
			}
		})
	}
	for _, invariant := range []string{"绝对截止时间", "不重置", "10s", "60s", "completed", "只读补查", "合法空数组"} {
		if !strings.Contains(policy, invariant) {
			t.Errorf("recovery policy lacks retry/result invariant %q", invariant)
		}
	}
}

// 执行Skill中的公开只读补查示例，证明补查不写任务且保留原业务上下文。
func TestReviewSkillRecoveryQueryExamples(t *testing.T) {
	policy := reviewRecoveryPolicy(t)
	lines := regexp.MustCompile(`(?m)^contract-cli review task (status|info) .*$`).FindAllString(policy, -1)
	if len(lines) != 2 {
		t.Fatalf("want status and info recovery examples, got %d", len(lines))
	}
	infoCalls, statusCalls := 0, 0
	app, out := alignmentApp(t, func(r *http.Request) string {
		if r.Method != http.MethodGet || r.URL.Query().Get("taskId") != preAuditTaskID || r.URL.Query().Get("businessId") != preAuditID {
			t.Fatalf("recovery changed task/business identity or made a write: %s %s", r.Method, r.URL)
		}
		switch r.URL.Path {
		case "/open-apis/contract/v3/review/status":
			statusCalls++
			return `{"code":200,"data":{"status":"success"}}`
		case "/open-apis/contract/v3/review/info":
			if r.URL.Query().Get("simplifyReviewResult") != "true" {
				t.Fatal("recovery detail query lost simplifyReviewResult")
			}
			infoCalls++
			if infoCalls == 1 {
				return `{"code":200,"data":{"status":"running","ruleResult":[]}}`
			}
			return `{"code":200,"data":{"status":"success","ruleResult":[]}}`
		default:
			t.Fatalf("unexpected recovery request %s", r.URL.Path)
			return ""
		}
	})
	runExample := func(line string, wantStatus string, wantError bool) {
		t.Helper()
		line = strings.NewReplacer("<profile>", "contract", "<review-task-id>", preAuditTaskID, "<business-id>", preAuditID).Replace(line)
		out.Reset()
		if err := app.Run(context.Background(), strings.Fields(line)[1:]); (err != nil) != wantError {
			t.Fatalf("recovery error=%v, wantError=%t", err, wantError)
		}
		var response struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.Data.Status != wantStatus {
			t.Fatalf("recovery response = %s, %v; want %s", out.String(), err, wantStatus)
		}
	}
	runExample(lines[0], "success", false)
	runExample(lines[1], "running", true)
	runExample(lines[1], "success", false)
	if statusCalls != 3 || infoCalls != 2 {
		t.Fatalf("recovery requests changed: status=%d info=%d", statusCalls, infoCalls)
	}
}
