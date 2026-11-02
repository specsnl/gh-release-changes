package changes_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

func TestLinkClosing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		title   string
		closing []int
		issues  []int
		sources changes.IssueSources
	}{
		{
			name:    "closes nothing",
			title:   "fix(12): x",
			issues:  []int{12},
			sources: changes.IssueSources{Title: []int{12}, Closing: []int{}},
		},
		{
			name:    "closes what the title scopes",
			title:   "fix(12): x",
			closing: []int{12},
			issues:  []int{12},
			sources: changes.IssueSources{Title: []int{12}, Closing: []int{12}},
		},
		{
			name:    "closes a second issue, added after the title's",
			title:   "fix(12): x",
			closing: []int{13, 12},
			issues:  []int{12, 13},
			sources: changes.IssueSources{Title: []int{12}, Closing: []int{12, 13}},
		},
		{
			name:    "the title's order is kept, the closing issues are ascending and deduplicated",
			title:   "fix(30,20): x",
			closing: []int{9, 5, 9},
			issues:  []int{30, 20, 5, 9},
			sources: changes.IssueSources{Title: []int{30, 20}, Closing: []int{5, 9}},
		},
		{
			name:    "a title without a scope",
			title:   "docs: update the changelog",
			closing: []int{8},
			issues:  []int{8},
			sources: changes.IssueSources{Title: []int{}, Closing: []int{8}},
		},
		{
			name:    "an unclassified title still takes nothing from the title",
			title:   "fixer-improvement (#12)",
			closing: []int{8},
			issues:  []int{8},
			sources: changes.IssueSources{Title: []int{}, Closing: []int{8}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := changes.Parse("* " + tc.title + " by @octocat in https://github.com/o/r/pull/1\n")
			group := items[0].Group

			changes.LinkClosing(items, map[int][]int{1: tc.closing, 2: {99}})

			item := items[0]
			if !reflect.DeepEqual(item.Issues, tc.issues) {
				t.Errorf("issues = %v, want %v", item.Issues, tc.issues)
			}

			if item.IssueSources == nil || !reflect.DeepEqual(*item.IssueSources, tc.sources) {
				t.Errorf("issue sources = %+v, want %+v", item.IssueSources, tc.sources)
			}

			if item.Group != group {
				t.Errorf("group = %q, want it unchanged from %q", item.Group, group)
			}
		})
	}
}

// An item that now references an issue moves above the items without one.
func TestLinkClosing_SortsAgain(t *testing.T) {
	items := changes.Parse(strings.Join([]string{
		"* fix: no issue by @octocat in https://github.com/o/r/pull/1",
		"* fix(50): issue 50 by @octocat in https://github.com/o/r/pull/2",
		"* fix: closes 7 by @octocat in https://github.com/o/r/pull/3",
	}, "\n"))

	changes.LinkClosing(items, map[int][]int{3: {7}})

	var prs []int
	for _, item := range items {
		prs = append(prs, item.PR)
	}

	if want := []int{3, 2, 1}; !reflect.DeepEqual(prs, want) {
		t.Errorf("order = %v, want %v", prs, want)
	}
}

// The JSON lists both sources, also when empty; without linking the field is
// left out, so the document is unchanged.
func TestIssueSources_JSON(t *testing.T) {
	items := changes.Parse("* fix(12): x by @octocat in https://github.com/o/r/pull/1\n")

	plain, err := json.Marshal(items[0])
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(plain), "issue_sources") {
		t.Errorf("an unlinked item has issue_sources: %s", plain)
	}

	changes.LinkClosing(items, nil)

	linked, err := json.Marshal(items[0])
	if err != nil {
		t.Fatal(err)
	}

	if want := `"issue_sources":{"title":[12],"closing":[]}`; !strings.Contains(string(linked), want) {
		t.Errorf("linked item = %s, want it to contain %s", linked, want)
	}
}
