package changes_test

import (
	"reflect"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

const prURL = "https://github.com/owner/repo/pull/"

// line builds one generate-notes list line.
func line(title, author string, pr string) string {
	return "* " + title + " by @" + author + " in " + prURL + pr
}

// TestParse_Rules covers every parser rule with one line each. The full item is
// compared, so a rule that leaks into a field it should not touch fails too.
func TestParse_Rules(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want changes.Item
	}{
		{
			name: "feat with one issue scope",
			line: line("feat(6768): add the export", "octocat", "12"),
			want: changes.Item{
				PR: 12, Title: "feat(6768): add the export", Type: "feat", Scopes: []string{"6768"},
				Issues: []int{6768}, Author: "octocat", URL: prURL + "12", Group: changes.GroupFeatures,
			},
		},
		{
			name: "multi-scope title with a breaking marker",
			line: line("feat(6768,6747)!: drop the old API", "octocat", "123"),
			want: changes.Item{
				PR: 123, Title: "feat(6768,6747)!: drop the old API", Type: "feat", Scopes: []string{"6768", "6747"},
				Breaking: true, Issues: []int{6768, 6747}, Author: "octocat", URL: prURL + "123",
				Group: changes.GroupFeatures,
			},
		},
		{
			name: "spaces after the comma are trimmed",
			line: line("fix(12, 34): handle both", "octocat", "5"),
			want: changes.Item{
				PR: 5, Title: "fix(12, 34): handle both", Type: "fix", Scopes: []string{"12", "34"},
				Issues: []int{12, 34}, Author: "octocat", URL: prURL + "5", Group: changes.GroupFixes,
			},
		},
		{
			name: "breaking marker without scopes",
			line: line("refactor!: rename the package", "octocat", "7"),
			want: changes.Item{
				PR: 7, Title: "refactor!: rename the package", Type: "refactor", Scopes: []string{},
				Breaking: true, Issues: []int{}, Author: "octocat", URL: prURL + "7", Group: changes.GroupOther,
			},
		},
		{
			name: "e2e and other non-digit scopes are not issues",
			line: line("test(e2e,1510,sidemenu): cover the menu", "octocat", "8"),
			want: changes.Item{
				PR: 8, Title: "test(e2e,1510,sidemenu): cover the menu", Type: "test",
				Scopes: []string{"e2e", "1510", "sidemenu"}, Issues: []int{1510}, Author: "octocat", URL: prURL + "8",
				Group: changes.GroupOther,
			},
		},
		{
			name: "a repeated issue scope is listed once",
			line: line("fix(9,9): twice", "octocat", "10"),
			want: changes.Item{
				PR: 10, Title: "fix(9,9): twice", Type: "fix", Scopes: []string{"9", "9"}, Issues: []int{9},
				Author: "octocat", URL: prURL + "10", Group: changes.GroupFixes,
			},
		},
		{
			name: "chore is maintenance",
			line: line("chore: update the changelog", "octocat", "11"),
			want: changes.Item{
				PR: 11, Title: "chore: update the changelog", Type: "chore", Scopes: []string{}, Issues: []int{},
				Author: "octocat", URL: prURL + "11", Group: changes.GroupMaintenance,
			},
		},
		{
			name: "revert is other",
			line: line("revert: undo the export", "octocat", "13"),
			want: changes.Item{
				PR: 13, Title: "revert: undo the export", Type: "revert", Scopes: []string{}, Issues: []int{},
				Author: "octocat", URL: prURL + "13", Group: changes.GroupOther,
			},
		},
		{
			name: "dependabot author, with its bot suffix",
			line: line("build(deps): bump golang from 1.26.5 to 1.27.0", "dependabot[bot]", "113"),
			want: changes.Item{
				PR: 113, Title: "build(deps): bump golang from 1.26.5 to 1.27.0", Type: "build", Scopes: []string{"deps"},
				Issues: []int{}, Author: "dependabot[bot]", Bot: true, URL: prURL + "113",
				Group: changes.GroupDependencies,
			},
		},
		{
			name: "deps scope takes precedence over fix",
			line: line("fix(deps): pin playwright", "octocat", "14"),
			want: changes.Item{
				PR: 14, Title: "fix(deps): pin playwright", Type: "fix", Scopes: []string{"deps"}, Issues: []int{},
				Author: "octocat", URL: prURL + "14", Group: changes.GroupDependencies,
			},
		},
		{
			name: "deps-dev scope takes precedence over chore",
			line: line("chore(deps-dev): bump vitest", "octocat", "15"),
			want: changes.Item{
				PR: 15, Title: "chore(deps-dev): bump vitest", Type: "chore", Scopes: []string{"deps-dev"},
				Issues: []int{}, Author: "octocat", URL: prURL + "15", Group: changes.GroupDependencies,
			},
		},
		{
			name: "dependabot author takes precedence over feat",
			line: line("feat: bump everything", "dependabot[bot]", "16"),
			want: changes.Item{
				PR: 16, Title: "feat: bump everything", Type: "feat", Scopes: []string{}, Issues: []int{},
				Author: "dependabot[bot]", Bot: true, URL: prURL + "16", Group: changes.GroupDependencies,
			},
		},
		{
			name: "dependabot outside the format is still a dependency",
			line: line("Bump lodash from 4.17.20 to 4.17.21", "dependabot[bot]", "17"),
			want: changes.Item{
				PR: 17, Title: "Bump lodash from 4.17.20 to 4.17.21", Scopes: []string{}, Issues: []int{},
				Author: "dependabot[bot]", Bot: true, URL: prURL + "17", Group: changes.GroupDependencies,
			},
		},
		{
			name: "another bot is a bot, not a dependency",
			line: line("chore: sync the labels", "github-actions[bot]", "18"),
			want: changes.Item{
				PR: 18, Title: "chore: sync the labels", Type: "chore", Scopes: []string{}, Issues: []int{},
				Author: "github-actions[bot]", Bot: true, URL: prURL + "18", Group: changes.GroupMaintenance,
			},
		},
		{
			name: "GitHub's revert title is unclassified",
			line: line(`Revert "feat(12): add the export"`, "octocat", "19"),
			want: changes.Item{
				PR: 19, Title: `Revert "feat(12): add the export"`, Scopes: []string{}, Issues: []int{},
				Author: "octocat", URL: prURL + "19", Group: changes.GroupUnclassified,
			},
		},
		{
			name: "a free-form title is unclassified",
			line: line("fixer-improvement", "octocat", "20"),
			want: changes.Item{
				PR: 20, Title: "fixer-improvement", Scopes: []string{}, Issues: []int{}, Author: "octocat",
				URL: prURL + "20", Group: changes.GroupUnclassified,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := changes.Parse(tc.line + "\n")
			if len(got) != 1 {
				t.Fatalf("Parse returned %d items, want 1: %+v", len(got), got)
			}

			if !reflect.DeepEqual(got[0], tc.want) {
				t.Errorf("Parse(%q)\n got %+v\nwant %+v", tc.line, got[0], tc.want)
			}
		})
	}
}

// Titles that look almost right are unclassified, and trust nothing from the
// title: no type, no scopes, no issues, no breaking marker.
func TestParse_Unclassified(t *testing.T) {
	for _, title := range []string{
		"feature(12): not a type",
		"Feat(12): types are lower case",
		"feat(12):no space after the colon",
		"feat(12): ",
		"feat(): empty scopes",
		"feat(12) : space before the colon",
		"feat(12)!:missing space",
		"feat(12+13): a plus is not a separator, but the title is still a title",
		"Closes #12 and #13",
		"WIP",
	} {
		t.Run(title, func(t *testing.T) {
			got := changes.Parse(line(title, "octocat", "1"))
			if len(got) != 1 {
				t.Fatalf("Parse returned %d items, want 1", len(got))
			}

			item := got[0]
			if item.Group == changes.GroupUnclassified {
				if item.Type != "" || len(item.Scopes) != 0 || len(item.Issues) != 0 || item.Breaking {
					t.Errorf("an unclassified item kept parts of its title: %+v", item)
				}

				return
			}

			// The one title above that is in the format: its scope is not an issue.
			if title != "feat(12+13): a plus is not a separator, but the title is still a title" {
				t.Errorf("%q landed in %q, want unclassified", title, item.Group)
			}

			if len(item.Issues) != 0 {
				t.Errorf("%q: issues = %v, want none — + is not a separator", title, item.Issues)
			}
		})
	}
}

// The parts of a body that are not pull request lines never become items.
func TestParse_IgnoresEverythingElse(t *testing.T) {
	body := "<!-- Release notes generated using configuration in .github/release.yml at v1.2.0 -->\n" +
		"\n" +
		"## What's Changed\n" +
		"### 🚀 Features\n" +
		line("feat(1): one", "octocat", "1") + "\n" +
		"### Other Changes\n" +
		line("chore: two", "octocat", "2") + "\n" +
		"\n" +
		"## New Contributors\n" +
		"* @newcomer made their first contribution in " + prURL + "2\n" +
		"* @dependabot[bot] made their first contribution in " + prURL + "3\n" +
		"\n" +
		"**Full Changelog**: https://github.com/owner/repo/compare/v1.1.0...v1.2.0\n"

	got := changes.Parse(body)

	var prs []int
	for _, item := range got {
		prs = append(prs, item.PR)
	}

	if want := []int{1, 2}; !reflect.DeepEqual(prs, want) {
		t.Errorf("PRs = %v, want %v", prs, want)
	}
}

// The edge cases of the line format itself.
func TestParse_LineFormat(t *testing.T) {
	t.Run("a title containing ' by @' splits at the last one", func(t *testing.T) {
		got := changes.Parse(line("docs: credit the fix by @someone else", "octocat", "4"))
		if len(got) != 1 || got[0].Title != "docs: credit the fix by @someone else" || got[0].Author != "octocat" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a GHES host", func(t *testing.T) {
		got := changes.Parse("* fix(3): on prem by @octocat in https://github.example.com/owner/repo/pull/42")
		if len(got) != 1 || got[0].PR != 42 || got[0].URL != "https://github.example.com/owner/repo/pull/42" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		got := changes.Parse("## What's Changed\r\n" + line("fix(3): crlf", "octocat", "5") + "\r\n")
		if len(got) != 1 || got[0].PR != 5 || got[0].URL != prURL+"5" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a link that is not a pull request", func(t *testing.T) {
		got := changes.Parse("* fix(3): not a PR by @octocat in https://github.com/owner/repo/issues/5")
		if len(got) != 0 {
			t.Errorf("got %+v, want nothing", got)
		}
	})

	t.Run("an empty body", func(t *testing.T) {
		got := changes.Parse("")
		if got == nil || len(got) != 0 {
			t.Errorf("Parse(\"\") = %#v, want an empty, non-nil slice", got)
		}
	})
}
