package render_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/render"
)

const source = "example-org/web-components"

func TestIssues_Golden(t *testing.T) {
	short := render.IssueRefs{IssueRepo: source, SourceRepo: source, RelativeTo: source}

	for _, name := range fixtures {
		t.Run(name+"/short", func(t *testing.T) {
			assertGolden(t, "issues/"+name+"-short.golden", render.Issues(fixtureItems(t, name), short, false))
		})

		t.Run(name+"/unlinked", func(t *testing.T) {
			assertGolden(t, "issues/"+name+"-unlinked.golden", render.Issues(fixtureItems(t, name), short, true))
		})
	}

	for _, tc := range []struct {
		name     string
		refs     render.IssueRefs
		unlinked bool
	}{
		{"full", render.IssueRefs{IssueRepo: source, SourceRepo: source, RelativeTo: "example-org/releases"}, false},
		{"full-refs", render.IssueRefs{IssueRepo: source, SourceRepo: source, RelativeTo: source, Full: true}, false},
		{"unlinked-full", render.IssueRefs{IssueRepo: source, SourceRepo: source, RelativeTo: "example-org/releases"}, true},
	} {
		t.Run("final/"+tc.name, func(t *testing.T) {
			assertGolden(t, "issues/final-"+tc.name+".golden", render.Issues(fixtureItems(t, "final"), tc.refs, tc.unlinked))
		})
	}
}

// Every pull request of all-linked references an issue or updates a
// dependency, so --unlinked adds nothing.
func TestIssues_Golden_AllLinked(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "issues", "all-linked.txt"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	items := changes.Parse(string(body))
	refs := render.IssueRefs{IssueRepo: "example-org/app", SourceRepo: "example-org/app", RelativeTo: "example-org/app"}

	got := render.Issues(items, refs, true)
	assertGolden(t, "issues/all-linked-unlinked.golden", got)

	if without := render.Issues(items, refs, false); got != without {
		t.Errorf("--unlinked changed the output:\n%s\nwithout:\n%s", got, without)
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

// A pull request is in the source repository, whatever the issue repository.
func TestIssueRefs_PR(t *testing.T) {
	for _, tc := range []struct {
		name string
		refs render.IssueRefs
		want string
	}{
		{"same repo", render.IssueRefs{IssueRepo: "o/app", SourceRepo: "o/app", RelativeTo: "o/app"}, "#7"},
		{"other issue repo", render.IssueRefs{IssueRepo: "o/tracker", SourceRepo: "o/app", RelativeTo: "o/app"}, "#7"},
		{"pasted in the issue repo", render.IssueRefs{IssueRepo: "o/tracker", SourceRepo: "o/app", RelativeTo: "o/tracker"}, "o/app#7"},
		{"source repo, other case", render.IssueRefs{IssueRepo: "o/app", SourceRepo: "O/App", RelativeTo: "o/app"}, "#7"},
		{"forced", render.IssueRefs{IssueRepo: "o/app", SourceRepo: "o/app", RelativeTo: "o/app", Full: true}, "o/app#7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.refs.PR(7); got != tc.want {
				t.Errorf("PR(7) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIssues(t *testing.T) {
	refs := render.IssueRefs{IssueRepo: "o/app", SourceRepo: "o/app", RelativeTo: "o/app"}

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
			if got := render.Issues(tc.items, refs, false); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestIssues_Unlinked(t *testing.T) {
	refs := render.IssueRefs{IssueRepo: "o/app", SourceRepo: "o/app", RelativeTo: "o/app"}

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
			name: "only dependency updates without an issue",
			items: []changes.Item{
				{PR: 1, Title: "fix(12): x", Group: changes.GroupFixes, Issues: []int{12}},
				{PR: 2, Title: "chore(deps): bump y", Group: changes.GroupDependencies},
			},
			want: "## Issues\n\n### Bug fixes\n\n* #12\n",
		},
		{
			name: "no issues at all",
			items: []changes.Item{
				{PR: 3, Title: "chore: tidy", Group: changes.GroupMaintenance},
			},
			want: "## Issues\n\nNo issues.\n\n### Pull requests without an issue\n\n* #3 chore: tidy\n",
		},
		{
			name: "every group but dependencies, by PR number",
			items: []changes.Item{
				{PR: 9, Title: "feat: a", Group: changes.GroupFeatures},
				{PR: 4, Title: "fix(12): b", Group: changes.GroupFixes, Issues: []int{12}},
				{PR: 7, Title: "fix: c", Group: changes.GroupFixes},
				{PR: 2, Title: "docs: d", Group: changes.GroupOther},
				{PR: 8, Title: "chore: release/v3.8.0 >> main", Group: changes.GroupMaintenance},
				{PR: 3, Title: "chore(deps): bump e", Group: changes.GroupDependencies},
				{PR: 5, Title: "Release v1", Group: changes.GroupUnclassified},
			},
			want: "## Issues\n\n### Bug fixes\n\n* #12\n\n" +
				"### Pull requests without an issue\n\n" +
				"* #2 docs: d\n* #5 Release v1\n* #7 fix: c\n* #8 chore: release/v3.8.0 >> main\n* #9 feat: a\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := render.Issues(tc.items, refs, true); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}
