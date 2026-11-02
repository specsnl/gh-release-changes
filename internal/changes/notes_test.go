package changes_test

import (
	"slices"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

func TestNoteRefs(t *testing.T) {
	type ref = changes.Ref

	for _, tc := range []struct {
		name string
		text string
		want []ref
	}{
		{"bare", "fixes #12.", []ref{{"o/app", 12}}},
		{"bare at the start of a line", "#7 and #8", []ref{{"o/app", 7}, {"o/app", 8}}},
		{"bare in a list", "* #3\n- [ ] #4 Title", []ref{{"o/app", 3}, {"o/app", 4}}},
		{"bare in brackets", "(#5), [#6]", []ref{{"o/app", 5}, {"o/app", 6}}},
		{"bare after a comma", "#1,#2", []ref{{"o/app", 1}, {"o/app", 2}}},
		{"full", "see o/tracker#9", []ref{{"o/tracker", 9}}},
		{"full with dots and dashes", "my-org/web.site_v2#3", []ref{{"my-org/web.site_v2", 3}}},
		{"issue URL", "https://github.com/o/tracker/issues/14", []ref{{"o/tracker", 14}}},
		{"pull request URL", "https://github.com/o/app/pull/31", []ref{{"o/app", 31}}},
		{"URL on another host", "https://ghe.example.com/o/app/issues/2", []ref{{"o/app", 2}}},
		{"URL with a fragment", "https://github.com/o/app/issues/2#issuecomment-99", []ref{{"o/app", 2}}},
		{"URL to the files of a pull request", "https://github.com/o/app/pull/3/files", []ref{{"o/app", 3}}},
		{"URL in a Markdown link", "[the bug](https://github.com/o/app/issues/4)", []ref{{"o/app", 4}}},
		{"http URL", "http://github.com/o/app/issues/5", []ref{{"o/app", 5}}},
		{"the same issue three ways", "#12 o/app#12 https://github.com/o/app/issues/12", []ref{{"o/app", 12}}},
		{"repositories compared without case", "O/App#12 #12", []ref{{"O/App", 12}}},
		{"ordered by repo, then number", "z/z#1 #20 #3 a/a#9", []ref{{"a/a", 9}, {"o/app", 3}, {"o/app", 20}, {"z/z", 1}}},
		{"glued to a word", "abc#12 v2#3", nil},
		{"a path with a fragment", "docs/guide/setup.md#12 a/b/c#4", nil},
		{"an HTML entity", "it&#39;s", nil},
		{"followed by letters", "#12abc", nil},
		{"a heading", "## Features\n### 12 things", nil},
		{"zero", "#0", nil},
		{"too large to be a number", "#99999999999999999999", nil},
		{"a URL to something else", "https://github.com/o/app/commit/1234", nil},
		{"nothing", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := changes.NoteRefs(tc.text, "o/app"); !slices.Equal(got, tc.want) {
				t.Errorf("NoteRefs(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestCheckNotes(t *testing.T) {
	type ref = changes.Ref

	release := changes.Release{
		Repo: "o/app",
		Items: []changes.Item{
			{PR: 40, Issues: []int{12}},
			{PR: 41, Issues: []int{12, 13}},
			{PR: 42, Issues: []int{}},
		},
	}

	for _, tc := range []struct {
		name      string
		issueRepo string
		refs      []ref
		want      changes.Coverage
	}{
		{
			name:      "covered",
			issueRepo: "o/app",
			refs:      []ref{{"o/app", 12}, {"o/app", 13}},
			want:      changes.Coverage{},
		},
		{
			name:      "an issue missing",
			issueRepo: "o/app",
			refs:      []ref{{"o/app", 13}},
			want:      changes.Coverage{Missing: []int{12}},
		},
		{
			name:      "an issue outside the release",
			issueRepo: "o/app",
			refs:      []ref{{"o/app", 12}, {"o/app", 13}, {"o/app", 99}},
			want:      changes.Coverage{Unknown: []ref{{"o/app", 99}}},
		},
		{
			name:      "a pull request of the release is in it",
			issueRepo: "o/app",
			refs:      []ref{{"o/app", 12}, {"o/app", 13}, {"o/app", 42}},
			want:      changes.Coverage{},
		},
		{
			name:      "a pull request does not cover its issue",
			issueRepo: "o/app",
			refs:      []ref{{"o/app", 40}, {"o/app", 13}},
			want:      changes.Coverage{Missing: []int{12}},
		},
		{
			name:      "issues in another repository",
			issueRepo: "o/tracker",
			refs:      []ref{{"o/tracker", 12}, {"o/app", 13}, {"o/app", 41}},
			want:      changes.Coverage{Missing: []int{13}, Unknown: []ref{{"o/app", 13}}},
		},
		{
			name:      "a pull request number in the issue repository",
			issueRepo: "o/tracker",
			refs:      []ref{{"o/tracker", 12}, {"o/tracker", 13}, {"o/tracker", 40}},
			want:      changes.Coverage{Unknown: []ref{{"o/tracker", 40}}},
		},
		{
			name:      "repositories compared without case",
			issueRepo: "O/Tracker",
			refs:      []ref{{"o/tracker", 12}, {"o/TRACKER", 13}, {"O/APP", 40}},
			want:      changes.Coverage{},
		},
		{
			name:      "no references",
			issueRepo: "o/app",
			want:      changes.Coverage{Missing: []int{12, 13}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := changes.CheckNotes(release, tc.issueRepo, tc.refs)

			if !slices.Equal(got.Missing, tc.want.Missing) {
				t.Errorf("Missing = %v, want %v", got.Missing, tc.want.Missing)
			}

			if !slices.Equal(got.Unknown, tc.want.Unknown) {
				t.Errorf("Unknown = %v, want %v", got.Unknown, tc.want.Unknown)
			}

			if got.Covered() != tc.want.Covered() {
				t.Errorf("Covered = %v, want %v", got.Covered(), tc.want.Covered())
			}
		})
	}
}

func TestCheckNotes_ReleaseWithoutIssues(t *testing.T) {
	release := changes.Release{Repo: "o/app", Items: []changes.Item{{PR: 1, Issues: []int{}}}}

	if got := changes.CheckNotes(release, "o/app", nil); !got.Covered() {
		t.Errorf("empty notes for a release without issues: %+v, want covered", got)
	}
}
