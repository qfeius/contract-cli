package skillsync

import (
	"reflect"
	"testing"
)

func TestPlanIncludesOnlyExplicitOfficialNames(t *testing.T) {
	p, err := Plan([]string{"auth", "contract-cli-new"}, []string{"auth", "contract-cli-old"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Install, []string{"auth", "contract-cli-new"}) || !reflect.DeepEqual(p.Remove, []string{"contract-cli-old"}) {
		t.Fatalf("plan = %#v", p)
	}
}

func TestPlanRejectsUnsafeAndDuplicateNames(t *testing.T) {
	for _, names := range [][]string{{"../escape"}, {"a/b"}, {"a\\b"}, {"."}, {""}, {"auth", "auth"}, {".contract-cli-managed.json"}} {
		if _, err := Plan(names, nil); err == nil {
			t.Errorf("accepted unsafe names %q", names)
		}
	}
}

func TestPlanDoesNotMutateInput(t *testing.T) {
	current := []string{"z", "a"}
	old := []string{"x", "a"}
	_, _ = Plan(current, old)
	if !reflect.DeepEqual(current, []string{"z", "a"}) || !reflect.DeepEqual(old, []string{"x", "a"}) {
		t.Fatal("input mutated")
	}
}
