package render_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/render"
)

// final-notes.md covers #1510 and #437 of the final fixture, but not #1514; it
// references #1565, a pull request of the release, and two issues outside it.
// Relative to another repository its bare references are in that one, as
// check-notes --relative-to reads them.
func TestCheckNotes_Golden(t *testing.T) {
	notes, err := os.ReadFile(filepath.Join("testdata", "check-notes", "final-notes.md"))
	if err != nil {
		t.Fatalf("reading the notes: %v", err)
	}

	release := changes.Release{Repo: source, Items: fixtureItems(t, "final")}

	for _, tc := range []struct {
		name       string
		relativeTo string
	}{
		{"short", source},
		{"relative-to", "example-org/releases"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refs := render.IssueRefs{IssueRepo: source, SourceRepo: source, RelativeTo: tc.relativeTo}
			coverage := changes.CheckNotes(release, source, changes.NoteRefs(string(notes), tc.relativeTo))

			assertGolden(t, "check-notes/final-"+tc.name+".golden", render.CheckNotes(coverage, refs))
		})
	}
}

func TestCheckNotes_Covered(t *testing.T) {
	refs := render.IssueRefs{IssueRepo: source, SourceRepo: source, RelativeTo: source}

	want := "## Notes check\n\nThe notes reference every issue of the release, and nothing outside it.\n"
	if got := render.CheckNotes(changes.Coverage{}, refs); got != want {
		t.Errorf("CheckNotes = %q, want %q", got, want)
	}
}

func TestCheckNotes_OneSectionOnly(t *testing.T) {
	refs := render.IssueRefs{IssueRepo: "o/tracker", SourceRepo: "o/app", RelativeTo: "o/app"}

	for _, tc := range []struct {
		name     string
		coverage changes.Coverage
		want     string
	}{
		{
			"missing only",
			changes.Coverage{Missing: []int{3}},
			"## Notes check\n\n### Issues missing from the notes\n\n* o/tracker#3\n",
		},
		{
			"outside only",
			changes.Coverage{Unknown: []changes.Ref{{Repo: "o/app", Number: 5}, {Repo: "O/Tracker", Number: 6}}},
			"## Notes check\n\n### References outside the release\n\n* #5\n* O/Tracker#6\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := render.CheckNotes(tc.coverage, refs); got != tc.want {
				t.Errorf("CheckNotes = %q, want %q", got, tc.want)
			}
		})
	}
}
