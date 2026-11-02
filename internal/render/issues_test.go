package render_test

import (
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/render"
)

func TestIssues_Golden(t *testing.T) {
	short := render.IssueRefs{IssueRepo: "example-org/web-components", RelativeTo: "example-org/web-components"}

	for _, name := range fixtures {
		t.Run(name+"/short", func(t *testing.T) {
			assertGolden(t, "issues/"+name+"-short.golden", render.Issues(fixtureItems(t, name), short))
		})
	}

	for _, tc := range []struct {
		name string
		refs render.IssueRefs
	}{
		{"full", render.IssueRefs{IssueRepo: "example-org/web-components", RelativeTo: "example-org/releases"}},
		{"full-refs", render.IssueRefs{IssueRepo: "example-org/web-components", RelativeTo: "example-org/web-components", Full: true}},
	} {
		t.Run("final/"+tc.name, func(t *testing.T) {
			assertGolden(t, "issues/final-"+tc.name+".golden", render.Issues(fixtureItems(t, "final"), tc.refs))
		})
	}
}

func TestIssueRefs_Ref(t *testing.T) {
	for _, tc := range []struct {
		name string
		refs render.IssueRefs
		want string
	}{
		{"same repo", render.IssueRefs{IssueRepo: "o/app", RelativeTo: "o/app"}, "#42"},
		{"same repo, other case", render.IssueRefs{IssueRepo: "O/App", RelativeTo: "o/app"}, "#42"},
		{"other ticket repo", render.IssueRefs{IssueRepo: "o/app", RelativeTo: "o/tickets"}, "o/app#42"},
		{"forced", render.IssueRefs{IssueRepo: "o/app", RelativeTo: "o/app", Full: true}, "o/app#42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.refs.Ref(42); got != tc.want {
				t.Errorf("Ref(42) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIssues(t *testing.T) {
	refs := render.IssueRefs{IssueRepo: "o/app", RelativeTo: "o/app"}

	for _, tc := range []struct {
		name  string
		items []changes.Item
		want  string
	}{
		{
			name: "no items",
			want: "## Issues\n\nNo issues.\n",
		},
		{
			name: "no item references an issue",
			items: []changes.Item{
				{PR: 1, Group: changes.GroupFeatures},
				{PR: 2, Group: changes.GroupDependencies, Scopes: []string{"deps"}},
			},
			want: "## Issues\n\nNo issues.\n",
		},
		{
			name: "groups without issues are left out; per group ascending and deduplicated",
			items: []changes.Item{
				{PR: 1, Group: changes.GroupFixes, Issues: []int{30, 10}},
				{PR: 2, Group: changes.GroupFixes, Issues: []int{10}},
				{PR: 3, Group: changes.GroupMaintenance},
				{PR: 4, Group: changes.GroupFeatures, Issues: []int{20}},
				{PR: 5, Group: changes.GroupOther, Issues: []int{10}},
			},
			want: "## Issues\n\n" +
				"### Features\n\n* #20\n\n" +
				"### Bug fixes\n\n* #10\n* #30\n\n" +
				"### Other\n\n* #10\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := render.Issues(tc.items, refs); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}
