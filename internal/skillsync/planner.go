package skillsync

import (
	"fmt"
	"sort"
	"strings"
)

type SyncPlan struct{ Install, Remove []string }

// unmanagedNames returns incoming names not covered by the existing registration.
func unmanagedNames(current, managed []string) []string {
	known := make(map[string]bool, len(managed))
	for _, name := range managed {
		known[name] = true
	}
	var added []string
	for _, name := range current {
		if !known[name] {
			added = append(added, name)
		}
	}
	return added
}

func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.HasPrefix(name, ".") && !strings.ContainsAny(name, "/\\:\x00")
}

// Plan compares explicit manifests without mutating either input.
func Plan(current, previous []string) (SyncPlan, error) {
	sets := make([]map[string]bool, 2)
	for i, names := range [][]string{current, previous} {
		sets[i] = map[string]bool{}
		for _, name := range names {
			if !validName(name) || sets[i][name] {
				return SyncPlan{}, fmt.Errorf("invalid or duplicate official skill name %q", name)
			}
			sets[i][name] = true
		}
	}
	p := SyncPlan{Install: append([]string{}, current...)}
	for _, name := range previous {
		if !sets[0][name] {
			p.Remove = append(p.Remove, name)
		}
	}
	sort.Strings(p.Install)
	sort.Strings(p.Remove)
	return p, nil
}
