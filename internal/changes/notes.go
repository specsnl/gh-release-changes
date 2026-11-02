package changes

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Ref is a reference to an issue or pull request: a number in an OWNER/REPO.
type Ref struct {
	Repo   string
	Number int
}

// refPattern matches, leftmost first, an issue or pull request URL, an
// OWNER/REPO#N reference, and a bare #N. Go's regexp has no lookbehind, so
// what may precede a reference is checked in [NoteRefs].
var refPattern = regexp.MustCompile(
	`https?://[^/\s]+/([A-Za-z0-9-]+)/([A-Za-z0-9._-]+)/(?:issues|pull)/(\d+)\b` +
		`|([A-Za-z0-9-]+)/([A-Za-z0-9._-]+)#(\d+)\b` +
		`|#(\d+)\b`)

// NoteRefs returns every reference in text — #N, OWNER/REPO#N, and issue and
// pull request URLs on any host — deduplicated and ordered by repository, then
// number. A bare #N is in relativeTo.
//
// A reference glued to what precedes it is not one, as GitHub does not link it
// either: a#1, a/b/c#1 (a path with a fragment) and &#39; (an HTML entity).
// Repositories are compared without regard to case; the first spelling wins.
func NoteRefs(text, relativeTo string) []Ref {
	var refs []Ref

	seen := map[Ref]bool{}

	for _, m := range refPattern.FindAllStringSubmatchIndex(text, -1) {
		if m[0] > 0 && gluedTo(text[m[0]-1]) {
			continue
		}

		group := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}

			return text[m[2*i]:m[2*i+1]]
		}

		var repo, number string

		switch {
		case group(1) != "":
			repo, number = group(1)+"/"+group(2), group(3)
		case group(4) != "":
			repo, number = group(4)+"/"+group(5), group(6)
		default:
			repo, number = relativeTo, group(7)
		}

		n, err := strconv.Atoi(number)
		if err != nil || n == 0 {
			continue
		}

		ref := Ref{Repo: repo, Number: n}

		key := Ref{Repo: strings.ToLower(ref.Repo), Number: ref.Number}
		if seen[key] {
			continue
		}

		seen[key] = true

		refs = append(refs, ref)
	}

	slices.SortFunc(refs, compareRefs)

	return refs
}

func gluedTo(c byte) bool {
	return c == '/' || c == '&' || c == '.' || c == '-' || c == '_' ||
		'0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

func compareRefs(a, b Ref) int {
	return cmp.Or(cmp.Compare(strings.ToLower(a.Repo), strings.ToLower(b.Repo)), cmp.Compare(a.Number, b.Number))
}

// Coverage is how release notes compare with the release they describe.
type Coverage struct {
	// Missing are the issues of the release the notes do not reference,
	// ascending.
	Missing []int

	// Unknown are the references in the notes to neither an issue nor a pull
	// request of the release, in [NoteRefs] order.
	Unknown []Ref
}

// Covered reports whether the notes reference every issue of the release and
// nothing outside it.
func (c Coverage) Covered() bool {
	return len(c.Missing) == 0 && len(c.Unknown) == 0
}

// CheckNotes compares the references of release notes with the release: its
// issues ([ReferencedIssues]) are in issueRepo, its pull requests in
// release.Repo. A reference to one of the release's pull requests is in the
// release, but does not cover an issue.
func CheckNotes(release Release, issueRepo string, refs []Ref) Coverage {
	inNotes := map[int]bool{}

	var coverage Coverage

	for _, ref := range refs {
		isIssue := strings.EqualFold(ref.Repo, issueRepo) && referencesIssue(release.Items, ref.Number)
		isPR := strings.EqualFold(ref.Repo, release.Repo) && hasPR(release.Items, ref.Number)

		if isIssue {
			inNotes[ref.Number] = true
		}

		if !isIssue && !isPR {
			coverage.Unknown = append(coverage.Unknown, ref)
		}
	}

	for _, n := range ReferencedIssues(release.Items) {
		if !inNotes[n] {
			coverage.Missing = append(coverage.Missing, n)
		}
	}

	return coverage
}

func referencesIssue(items []Item, n int) bool {
	return slices.ContainsFunc(items, func(item Item) bool { return slices.Contains(item.Issues, n) })
}

func hasPR(items []Item, n int) bool {
	return slices.ContainsFunc(items, func(item Item) bool { return item.PR == n })
}
