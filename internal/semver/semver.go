// Package semver orders tags for the tag picker — semantic versions newest
// first, pre-releases included, and every other tag after them — and finds the
// final release a release cycle starts from, for --since-final, and the one a
// release ticket is found by.
//
// It is pure — tag names in, tag names out. Parsing and precedence are
// Masterminds/semver's, held to the strict Semantic Versioning 2.0.0 grammar,
// with an optional leading v, which is how tags spell versions.
package semver

import (
	"fmt"
	"slices"
	"strings"

	mmsemver "github.com/Masterminds/semver/v3"
)

// Parse parses tag as a semantic version, with or without a leading v. A
// shorthand such as v1.2, which the lenient parser would complete, is not one.
func Parse(tag string) (*mmsemver.Version, bool) {
	v, err := mmsemver.StrictNewVersion(strings.TrimPrefix(tag, "v"))
	if err != nil {
		return nil, false
	}

	return v, true
}

// Sort returns tags in picker order: semantic versions newest first,
// pre-releases included, then every other tag in reverse lexical order. Tags
// of equal precedence — v1.0.0 and 1.0.0, or two build metadata — are ordered
// by name, so the order is total and the same tags always come out the same.
func Sort(tags []string) []string {
	type tag struct {
		name    string
		version *mmsemver.Version
		semver  bool
	}

	parsed := make([]tag, len(tags))
	for i, name := range tags {
		v, ok := Parse(name)
		parsed[i] = tag{name: name, version: v, semver: ok}
	}

	slices.SortFunc(parsed, func(a, b tag) int {
		switch {
		case a.semver && !b.semver:
			return -1
		case !a.semver && b.semver:
			return 1
		case a.semver:
			if c := b.version.Compare(a.version); c != 0 {
				return c
			}
		}

		return strings.Compare(b.name, a.name)
	})

	sorted := make([]string, len(parsed))
	for i, t := range parsed {
		sorted[i] = t.name
	}

	return sorted
}

// PreviousFinal returns the newest final release among tags — a semantic
// version without pre-release identifiers — with lower precedence than target:
// the base that covers a whole release cycle. For v3.9.0-rc.2 that is v3.8.0,
// the base GitHub picks for v3.9.0-rc.1 and for v3.9.0. It reports false when
// target is no semantic version or no older final release is among tags.
func PreviousFinal(tags []string, target string) (string, bool) {
	t, ok := Parse(target)
	if !ok {
		return "", false
	}

	for _, name := range Sort(tags) {
		v, ok := Parse(name)
		if !ok {
			break
		}

		if v.Prerelease() == "" && v.LessThan(t) {
			return name, true
		}
	}

	return "", false
}

// Release returns the version of the final release target belongs to:
// MAJOR.MINOR.PATCH without pre-release or build, so v3.9.0-rc.2 and v3.9.0
// both give 3.9.0. A tag that is no semantic version gives itself without a
// leading v.
func Release(target string) string {
	v, ok := Parse(target)
	if !ok {
		return strings.TrimPrefix(target, "v")
	}

	return fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch())
}

// InTitle reports whether title names version as a whole: 3.9.0 is in
// "Release 3.9.0-rc.1" and "v3.9.0.", but not in "13.9.0" or "3.9.0.1".
func InTitle(title, version string) bool {
	if version == "" {
		return false
	}

	for i := 0; ; {
		j := strings.Index(title[i:], version)
		if j < 0 {
			return false
		}

		start, end := i+j, i+j+len(version)
		if !continuesBefore(title[:start]) && !continuesAfter(title[end:]) {
			return true
		}

		i = start + 1
	}
}

// continuesBefore reports whether the version found after before is part of a
// longer one: a digit or a dot precedes it.
func continuesBefore(before string) bool {
	return before != "" && strings.ContainsAny(before[len(before)-1:], "0123456789.")
}

// continuesAfter reports whether the version found before after is part of a
// longer one: a digit follows it, or a dot and a digit.
func continuesAfter(after string) bool {
	after = strings.TrimPrefix(after, ".")

	return after != "" && after[0] >= '0' && after[0] <= '9'
}
