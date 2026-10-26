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
		"任务可能已创建，重新发起可能产生重复任务",
		"用户未确认、拒绝或未回复时，不重发",
		"用户明确确认本次重发后，允许重新发起一次",
		"原始发起授权不等于重发授权",
		"再次超时或连接中断仍需重新告知并由用户决定",
		"已取得 taskId：继续查询原任务",
		"明确未创建任务：修正失败原因后可以重新发起",
		"复用仍有效的上传结果和已确认配置",
		"不重新执行 review run",
	} {
		if !strings.Contains(content, rule) {
			t.Errorf("published skill lacks recovery boundary: %s", rule)
		}
	}
	for _, obsolete := range []string{
		"业务失败或网络结果不确定不重复创建",
		"请求超时而未取得 taskId 时报告结果不确定，禁止重发创建",
	} {
		if strings.Contains(content, obsolete) {
			t.Errorf("published skill retains conflicting unconditional prohibition: %s", obsolete)
		}
	}
}
