package changes_test

import (
	"slices"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

func TestReferencedIssues(t *testing.T) {
	items := []changes.Item{
		{PR: 1, Issues: []int{30, 10}},
		{PR: 2, Issues: []int{10}},
		{PR: 3, Issues: []int{}},
		{PR: 4, Issues: []int{20}},
	}

	if got, want := changes.ReferencedIssues(items), []int{10, 20, 30}; !slices.Equal(got, want) {
		t.Errorf("ReferencedIssues = %v, want %v", got, want)
	}

	if got := changes.ReferencedIssues(nil); len(got) != 0 {
		t.Errorf("ReferencedIssues(nil) = %v, want none", got)
	}
}

func TestUntracked(t *testing.T) {
	for _, tc := range []struct {
		name string
		item changes.Item
		want bool
	}{
		{"feature without an issue", changes.Item{Group: changes.GroupFeatures}, true},
		{"fix without an issue", changes.Item{Group: changes.GroupFixes, Issues: []int{}}, true},
		{"fix with an issue", changes.Item{Group: changes.GroupFixes, Issues: []int{4}}, false},
		{"fix(deps)", changes.Item{Type: "fix", Group: changes.GroupDependencies}, false},
		{"docs", changes.Item{Type: "docs", Group: changes.GroupOther}, false},
		{"unclassified", changes.Item{Group: changes.GroupUnclassified}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := changes.Untracked(tc.item); got != tc.want {
				t.Errorf("Untracked = %v, want %v", got, tc.want)
			}
		})
	}
}
