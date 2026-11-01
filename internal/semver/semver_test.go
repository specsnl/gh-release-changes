package semver_test

import (
	"slices"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/semver"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		tag  string
		want semver.Version
		ok   bool
	}{
		{"v1.2.3", semver.Version{Major: 1, Minor: 2, Patch: 3}, true},
		{"1.2.3", semver.Version{Major: 1, Minor: 2, Patch: 3}, true},
		{"v0.0.0", semver.Version{}, true},
		{"v2.0.0-rc.1", semver.Version{Major: 2, Prerelease: []string{"rc", "1"}}, true},
		{"v1.0.0-alpha-1.x", semver.Version{Major: 1, Prerelease: []string{"alpha-1", "x"}}, true},
		{"v1.0.0+build.5", semver.Version{Major: 1}, true},
		{"v1.0.0-beta+exp.sha.5114f85", semver.Version{Major: 1, Prerelease: []string{"beta"}}, true},
		{"v1.2", semver.Version{}, false},
		{"v1.2.3.4", semver.Version{}, false},
		{"V1.2.3", semver.Version{}, false},
		{"v01.2.3", semver.Version{}, false},
		{"v1.2.3-01", semver.Version{}, false},
		{"v1.2.3-", semver.Version{}, false},
		{"release/1.0", semver.Version{}, false},
		{"latest", semver.Version{}, false},
		{"", semver.Version{}, false},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			got, ok := semver.Parse(tc.tag)
			if ok != tc.ok {
				t.Fatalf("Parse(%q) ok = %v, want %v", tc.tag, ok, tc.ok)
			}

			if ok && (got.Major != tc.want.Major || got.Minor != tc.want.Minor || got.Patch != tc.want.Patch ||
				!slices.Equal(got.Prerelease, tc.want.Prerelease)) {
				t.Errorf("Parse(%q) = %+v, want %+v", tc.tag, got, tc.want)
			}
		})
	}
}

// The precedence example from the specification, rule 11, lowest first.
func TestCompare_SpecificationOrder(t *testing.T) {
	ordered := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11",
		"1.0.0-rc.1", "1.0.0", "2.0.0", "2.1.0", "2.1.1",
	}

	for i := range len(ordered) - 1 {
		a, _ := semver.Parse(ordered[i])
		b, _ := semver.Parse(ordered[i+1])

		if c := semver.Compare(a, b); c != -1 {
			t.Errorf("Compare(%s, %s) = %d, want -1", ordered[i], ordered[i+1], c)
		}

		if c := semver.Compare(b, a); c != 1 {
			t.Errorf("Compare(%s, %s) = %d, want 1", ordered[i+1], ordered[i], c)
		}
	}
}

func TestCompare_BuildMetadataDoesNotDecide(t *testing.T) {
	a, _ := semver.Parse("v1.0.0+one")
	b, _ := semver.Parse("v1.0.0+two")

	if c := semver.Compare(a, b); c != 0 {
		t.Errorf("Compare = %d, want 0", c)
	}
}

func TestSort(t *testing.T) {
	for _, tc := range []struct {
		name string
		tags []string
		want []string
	}{
		{
			name: "newest first, pre-releases included",
			tags: []string{"v1.0.0", "v2.0.0-rc.1", "v1.10.0", "v1.9.0", "v2.0.0", "v2.0.0-rc.2"},
			want: []string{"v2.0.0", "v2.0.0-rc.2", "v2.0.0-rc.1", "v1.10.0", "v1.9.0", "v1.0.0"},
		},
		{
			name: "numeric pre-release identifiers compare as numbers",
			tags: []string{"v1.0.0-rc.2", "v1.0.0-rc.10", "v1.0.0-rc.1"},
			want: []string{"v1.0.0-rc.10", "v1.0.0-rc.2", "v1.0.0-rc.1"},
		},
		{
			name: "non-semver tags after the semver ones",
			tags: []string{"latest", "v1.0.0", "release-2024-10", "v1.2", "v0.9.0", "release-2024-09"},
			want: []string{"v1.0.0", "v0.9.0", "v1.2", "release-2024-10", "release-2024-09", "latest"},
		},
		{
			name: "equal precedence is ordered by name",
			tags: []string{"1.0.0", "v1.0.0", "v1.0.0+b", "v1.0.0+a"},
			want: []string{"v1.0.0+b", "v1.0.0+a", "v1.0.0", "1.0.0"},
		},
		{
			name: "no tags",
			tags: []string{},
			want: []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := semver.Sort(tc.tags); !slices.Equal(got, tc.want) {
				t.Errorf("Sort = %v, want %v", got, tc.want)
			}
		})
	}
}

// Sorting is free of side effects, and the same tags in any order give the
// same result.
func TestSort_IsTotalAndLeavesTheInputAlone(t *testing.T) {
	tags := []string{"v1.0.0", "latest", "v2.0.0-rc.1", "1.0.0", "v2.0.0"}
	reversed := slices.Clone(tags)
	slices.Reverse(reversed)

	first := semver.Sort(tags)

	if tags[0] != "v1.0.0" {
		t.Errorf("Sort reordered its input: %v", tags)
	}

	if second := semver.Sort(reversed); !slices.Equal(first, second) {
		t.Errorf("Sort depends on the input order: %v and %v", first, second)
	}
}
