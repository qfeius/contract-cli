package cli_test

import (
	"strings"
	"testing"
)

// Validate the distributed instruction contract; this does not simulate an LLM
// or claim that a backend creation request is idempotent.
func TestReviewSkillUncertainCreationPolicy(t *testing.T) {
	content := reviewSkill(t, "contract-cli-review")
	for _, rule := range []string{
		"### 发起失败与结果不确定",
		"启动请求超时或连接中断且未取得 taskId",
		"任务可能已创建",
		"无法确认时停止",
		"不自动重发，不触发第二轮",
		"不猜taskId或调用未提供的请求号恢复接口",
		"已取得 taskId：保留原任务及业务上下文",
		"不把轮次清零当作自动第三轮",
		"不绕过CLI状态机",
	} {
		if !strings.Contains(content, rule) {
			t.Errorf("published skill lacks recovery boundary: %s", rule)
		}
	}
	for _, obsolete := range []string{
		"业务失败或网络结果不确定不重复创建",
		"请求超时而未取得 taskId 时报告结果不确定，禁止重发创建",
		"再次超时或连接中断仍需重新告知并由用户决定",
	} {
		if strings.Contains(content, obsolete) {
			t.Errorf("published skill retains conflicting unconditional prohibition: %s", obsolete)
		}
	}
}
