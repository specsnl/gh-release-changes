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

func TestIssueless(t *testing.T) {
	for _, tc := range []struct {
		name string
		item changes.Item
		want bool
	}{
		{"feature without an issue", changes.Item{Group: changes.GroupFeatures}, true},
		{"fix with an issue", changes.Item{Group: changes.GroupFixes, Issues: []int{4}}, false},
		{"docs without an issue", changes.Item{Type: "docs", Group: changes.GroupOther, Issues: []int{}}, true},
		{"chore without an issue", changes.Item{Type: "chore", Group: changes.GroupMaintenance}, true},
		{"unclassified", changes.Item{Group: changes.GroupUnclassified}, true},
		{"dependency update", changes.Item{Type: "chore", Group: changes.GroupDependencies}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := changes.Issueless(tc.item); got != tc.want {
				t.Errorf("Issueless = %v, want %v", got, tc.want)
			}
		})
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

func TestIssueLinks(t *testing.T) {
	issue := changes.Issue{ClosedBy: []changes.PullRequest{
		{Number: 1, Repo: "o/app"},
		{Number: 2, Repo: "o/lib"},
		{Number: 3},
	}}

	for _, tc := range []struct {
		name string
		repo string
		pr   int
		want bool
	}{
		{"linked", "o/app", 1, true},
		{"linked, repository in another case", "O/App", 1, true},
		{"same number in another repository", "o/app", 2, false},
		{"no repository", "o/app", 3, true},
		{"not linked", "o/app", 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := issue.Links(tc.repo, tc.pr); got != tc.want {
				t.Errorf("Links(%q, %d) = %v, want %v", tc.repo, tc.pr, got, tc.want)
			}
		})
	}
}

func TestUnlinkedPullRequests(t *testing.T) {
	release := changes.Release{Repo: "o/app", Items: []changes.Item{
		{PR: 5, Issues: []int{1}},
		{PR: 3, Issues: []int{1, 2}},
		{PR: 4, Issues: []int{2}},
		{PR: 6, Issues: []int{9}},
		{PR: 7},
	}}
	issues := map[int]changes.Issue{
		1: {ClosedBy: []changes.PullRequest{{Number: 5, Repo: "o/app"}}},
		2: {ClosedBy: []changes.PullRequest{{Number: 4, Repo: "o/lib"}}},
	}

	if got, want := changes.UnlinkedPullRequests(release, issues), []int{3, 4}; !slices.Equal(got, want) {
		t.Errorf("UnlinkedPullRequests = %v, want %v", got, want)
	}
}

func TestDetailsMerge(t *testing.T) {
	d := changes.Details{Issues: map[int]changes.Issue{1: {Number: 1}}}

	d.Merge(changes.Details{
		PullRequests:           map[int]changes.PullRequest{2: {Number: 2}},
		UnreadablePullRequests: map[int]string{3: "gone"},
	})

	if _, ok := d.Issues[1]; !ok {
		t.Error("Merge dropped an issue")
	}

	if _, ok := d.PullRequests[2]; !ok {
		t.Error("Merge did not add the pull request")
	}

	if d.UnreadablePullRequests[3] != "gone" {
		t.Error("Merge did not add the unreadable pull request")
	}

	if d.UnreadableIssues != nil {
		t.Error("Merge created a map for nothing")
	}
}
