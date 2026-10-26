package cli_test

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// Execute the distributed Skill example, rather than matching its wording.
// Node is also used by the repository's installer tests.
func TestReviewSkillNavigationExample(t *testing.T) {
	body := reviewSkill(t, "contract-cli-review")
	blocks := regexp.MustCompile("(?s)```javascript\\n(.*?)\\n```").FindAllStringSubmatch(body, -1)
	if len(blocks) != 1 {
		t.Fatalf("want one executable navigation example, got %d", len(blocks))
	}
	for _, tc := range []struct {
		name, input, want string
	}{
		{"no query", "https://example.test/detail", "https://example.test/detail?targetType=ai-review"},
		{"cooperation with empty info URL", "https://dev-contract.qtech.cn/cooperation/detail?cooperationId=1178244026340475224&from=openPlatform", "https://dev-contract.qtech.cn/cooperation/detail?cooperationId=1178244026340475224&from=allCoopList&targetType=ai-review"},
		{"cooperation preserves positioning", "https://example.test/cooperation/detail?cooperationId=123&from=openPlatform&processTaskInstanceId=&targetType=old#file", "https://example.test/cooperation/detail?cooperationId=123&from=allCoopList&processTaskInstanceId=&targetType=ai-review#file"},
		{"cooperation already all list", "https://example.test/cooperation/detail?from=allCoopList", "https://example.test/cooperation/detail?from=allCoopList&targetType=ai-review"},
		{"cooperation other source", "https://example.test/cooperation/detail?from=coopList", "https://example.test/cooperation/detail?from=coopList&targetType=ai-review"},
		{"cooperation without source", "https://example.test/cooperation/detail?cooperationId=123", "https://example.test/cooperation/detail?cooperationId=123&targetType=ai-review"},
		{"task center preserves source", "https://example.test/task-center/todo/task-123?from=openPlatform", "https://example.test/task-center/todo/task-123?from=openPlatform&targetType=ai-review"},
		{"replace duplicates and preserve fragment", "https://example.test/detail?targetType=old&processTaskInstanceId=&targetType=other#from", "https://example.test/detail?targetType=ai-review&processTaskInstanceId=#from"},
		{"invalid", "not-a-url", ""},
		{"non HTTP", "javascript:alert(1)", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Only contract pc_url is supplied: no info URL or AI task ID.
			out, err := exec.Command("node", "-e", blocks[0][1], tc.input).Output()
			if tc.want == "" {
				if err == nil || len(out) != 0 {
					t.Fatalf("invalid source produced a link: %q, %v", out, err)
				}
				return
			}
			if err != nil || strings.TrimSpace(string(out)) != tc.want {
				t.Fatalf("navigation = %q, %v; want %q", out, err, tc.want)
			}
			replayed, err := exec.Command("node", "-e", blocks[0][1], tc.want).Output()
			if err != nil || strings.TrimSpace(string(replayed)) != tc.want {
				t.Fatalf("repeated navigation = %q, %v; want unchanged %q", replayed, err, tc.want)
			}
		})
	}
}
