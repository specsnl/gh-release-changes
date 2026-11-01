package render_test

import (
	"strconv"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/render"
)

func TestSort_Golden(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			assertGolden(t, "sort/"+name+".golden", render.Sort(fixtureItems(t, name)))
		})
	}
}

func TestSort(t *testing.T) {
	item := func(pr int, title, author string, group changes.Group, breaking bool, issues ...int) changes.Item {
		return changes.Item{
			PR: pr, Title: title, Author: author, Group: group, Breaking: breaking, Issues: issues,
			URL: "https://github.com/o/r/pull/" + strconv.Itoa(pr),
		}
	}

	for _, tc := range []struct {
		name  string
		items []changes.Item
		want  string
	}{
		{
			name: "no items",
			want: "## What's Changed\n\nNo changes.\n",
		},
		{
			name: "empty groups are left out, the rest in display order",
			items: []changes.Item{
				item(3, "Free form", "carol", changes.GroupUnclassified, false),
				item(2, "fix(9): b", "bob", changes.GroupFixes, false, 9),
				item(1, "fix(4): a", "alice", changes.GroupFixes, false, 4),
			},
			want: "## What's Changed\n\n" +
				"### Bug fixes\n\n" +
				"* fix(4): a by @alice in https://github.com/o/r/pull/1\n" +
				"* fix(9): b by @bob in https://github.com/o/r/pull/2\n\n" +
				"### Unclassified\n\n" +
				"* Free form by @carol in https://github.com/o/r/pull/3\n",
		},
		{
			name:  "a breaking change is marked, its title kept",
			items: []changes.Item{item(1, "feat!: drop v1", "alice", changes.GroupFeatures, true)},
			want: "## What's Changed\n\n" +
				"### Features\n\n" +
				"* **Breaking:** feat!: drop v1 by @alice in https://github.com/o/r/pull/1\n",
		},
		{
			name:  "a bot keeps its suffix",
			items: []changes.Item{item(5, "chore(deps): bump x", "dependabot[bot]", changes.GroupDependencies, false)},
			want: "## What's Changed\n\n" +
				"### Dependencies\n\n" +
				"* chore(deps): bump x by @dependabot[bot] in https://github.com/o/r/pull/5\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := render.Sort(tc.items); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

// The input is not reordered in place: rendering is free of side effects.
func TestSort_LeavesTheInputAlone(t *testing.T) {
	items := []changes.Item{
		{PR: 2, Title: "fix: b", Group: changes.GroupFixes},
		{PR: 1, Title: "feat: a", Group: changes.GroupFeatures},
	}

	render.Sort(items)

	if items[0].PR != 2 {
		t.Errorf("Sort reordered its input: %+v", items)
	}
}
