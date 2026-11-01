package changes_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

// A body whose lines arrive in the wrong order for every rule at once.
var unsortedBody = strings.Join([]string{
	line("Revert \"feat(1): x\"", "octocat", "90"),
	line("chore(deps): bump b", "dependabot[bot]", "31"),
	line("chore(deps): bump a", "dependabot[bot]", "30"),
	line("fix: no issue, high PR", "octocat", "50"),
	line("fix(200): later issue", "octocat", "10"),
	line("fix: no issue, low PR", "octocat", "40"),
	line("fix(100,300): earlier issue", "octocat", "20"),
	line("fix(100): same issue, lower PR", "octocat", "15"),
	line("chore: tidy", "octocat", "60"),
	line("docs(7): explain", "octocat", "70"),
	line("feat(500): add", "octocat", "80"),
}, "\n")

func TestParse_Sorts(t *testing.T) {
	got := changes.Parse(unsortedBody)

	var prs []int
	for _, item := range got {
		prs = append(prs, item.PR)
	}

	want := []int{
		80,                 // features
		15, 20, 10, 40, 50, // fixes: by first issue, then PR; no issue last, by PR
		70,     // other
		60,     // maintenance
		30, 31, // dependencies: by PR
		90, // unclassified
	}

	if !reflect.DeepEqual(prs, want) {
		t.Errorf("order = %v, want %v", prs, want)
	}
}

// Dependencies ignore digit scopes when sorting: a deps item that happens to
// carry one is still ordered by PR number.
func TestSort_DependenciesByPRNumber(t *testing.T) {
	items := changes.Parse(line("chore(deps,5): bump b", "octocat", "2") + "\n" +
		line("chore(deps,1): bump a", "octocat", "3") + "\n" +
		line("chore(deps): bump c", "octocat", "1"))

	var prs []int
	for _, item := range items {
		prs = append(prs, item.PR)
	}

	if want := []int{1, 2, 3}; !reflect.DeepEqual(prs, want) {
		t.Errorf("order = %v, want %v", prs, want)
	}
}

func TestSections(t *testing.T) {
	sections := changes.Sections(changes.Parse(unsortedBody))

	var got []changes.Group
	for _, s := range sections {
		got = append(got, s.Group)
	}

	if !reflect.DeepEqual(got, changes.Groups) {
		t.Errorf("groups = %v, want %v", got, changes.Groups)
	}

	if n := len(sections[1].Items); n != 5 {
		t.Errorf("fixes has %d items, want 5", n)
	}
}

// Empty groups are left out, and the order is still the display order.
func TestSections_SkipsEmptyGroups(t *testing.T) {
	sections := changes.Sections(changes.Parse(line("Odd title", "octocat", "2") + "\n" + line("feat: x", "octocat", "1")))

	if len(sections) != 2 || sections[0].Group != changes.GroupFeatures || sections[1].Group != changes.GroupUnclassified {
		t.Errorf("sections = %+v", sections)
	}
}

func TestGroups_IdentifiersAndTitles(t *testing.T) {
	// The identifiers are public API; spelled out so a rename fails here.
	want := map[changes.Group]string{
		"features":     "Features",
		"fixes":        "Bug fixes",
		"other":        "Other",
		"maintenance":  "Maintenance",
		"dependencies": "Dependencies",
		"unclassified": "Unclassified",
	}

	if len(changes.Groups) != len(want) {
		t.Fatalf("%d groups, want %d", len(changes.Groups), len(want))
	}

	for _, g := range changes.Groups {
		title, ok := want[g]
		if !ok {
			t.Errorf("unexpected group %q", g)

			continue
		}

		if g.Title() != title {
			t.Errorf("%q.Title() = %q, want %q", g, g.Title(), title)
		}
	}
}
