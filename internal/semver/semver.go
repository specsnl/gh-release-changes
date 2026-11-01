// Package semver orders tags for the tag picker: semantic versions newest
// first, pre-releases included, and every other tag after them.
//
// It is pure — tag names in, tag names out — and implements only the
// precedence rules of Semantic Versioning 2.0.0 the picker needs, with an
// optional leading v, which is how tags spell versions.
package semver

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// versionRe is MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD], with an optional v,
// as the specification spells it: no leading zeros in a number.
var versionRe = regexp.MustCompile(
	`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// Version is a parsed semantic version.
type Version struct {
	Major, Minor, Patch uint64

	// Prerelease is the dot-separated identifiers after -, empty for a release.
	Prerelease []string
}

// Parse parses tag as a semantic version, with or without a leading v.
func Parse(tag string) (Version, bool) {
	m := versionRe.FindStringSubmatch(tag)
	if m == nil {
		return Version{}, false
	}

	var v Version

	for i, field := range []*uint64{&v.Major, &v.Minor, &v.Patch} {
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return Version{}, false
		}

		*field = n
	}

	if m[4] != "" {
		v.Prerelease = strings.Split(m[4], ".")
	}

	return v, true
}

// Compare returns -1, 0 or +1 as a has lower, equal or higher precedence than
// b. Build metadata is not part of a Version, so it never decides.
func Compare(a, b Version) int {
	if c := cmp.Compare(a.Major, b.Major); c != 0 {
		return c
	}

	if c := cmp.Compare(a.Minor, b.Minor); c != 0 {
		return c
	}

	if c := cmp.Compare(a.Patch, b.Patch); c != 0 {
		return c
	}

	return comparePrerelease(a.Prerelease, b.Prerelease)
}

// comparePrerelease applies rule 11: a release outranks its pre-releases;
// identifiers compare left to right, numeric ones numerically and below
// alphanumeric ones; a longer set wins when all before it are equal.
func comparePrerelease(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}

	for i := range min(len(a), len(b)) {
		if c := compareIdentifier(a[i], b[i]); c != 0 {
			return c
		}
	}

	return cmp.Compare(len(a), len(b))
}

func compareIdentifier(a, b string) int {
	an, aErr := strconv.ParseUint(a, 10, 64)
	bn, bErr := strconv.ParseUint(b, 10, 64)

	switch {
	case aErr == nil && bErr == nil:
		return cmp.Compare(an, bn)
	case aErr == nil:
		return -1
	case bErr == nil:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// Sort returns tags in picker order: semantic versions newest first,
// pre-releases included, then every other tag in reverse lexical order. Tags
// of equal precedence — v1.0.0 and 1.0.0, or two build metadata — are ordered
// by name, so the order is total and the same tags always come out the same.
func Sort(tags []string) []string {
	type tag struct {
		name    string
		version Version
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
			if c := Compare(b.version, a.version); c != 0 {
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
