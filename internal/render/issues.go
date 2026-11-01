package render

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

// IssueRefs decides how [Issues] writes a reference.
type IssueRefs struct {
	// IssueRepo is the OWNER/REPO the digit scopes point at.
	IssueRepo string

	// SourceRepo is the OWNER/REPO the pull requests are in.
	SourceRepo string

	// RelativeTo is the OWNER/REPO references are written relative to, usually
	// the one the list is pasted into. A reference is short (#N) when its
	// repository equals RelativeTo, and full (OWNER/REPO#N) otherwise.
	RelativeTo string

	// Full forces the full form everywhere.
	Full bool
}

// Ref formats issue n.
func (r IssueRefs) Ref(n int) string {
	return r.ref(r.IssueRepo, n)
}

// PR formats pull request n.
func (r IssueRefs) PR(n int) string {
	return r.ref(r.SourceRepo, n)
}

func (r IssueRefs) ref(repo string, n int) string {
	// GitHub resolves owner and repository names case-insensitively.
	if !r.Full && strings.EqualFold(repo, r.RelativeTo) {
		return fmt.Sprintf("#%d", n)
	}

	return fmt.Sprintf("%s#%d", repo, n)
}

// Issues renders the plain issue list for the release ticket: a heading per
// group that references issues, in display order, and its issues ascending and
// deduplicated, one reference per line. With unlinked, the [changes.Issueless]
// pull requests follow in a section of their own, by PR number; it is left out
// when there are none.
func Issues(items []changes.Item, refs IssueRefs, unlinked bool) string {
	var b strings.Builder

	b.WriteString("## Issues\n")

	listed := false

	for _, section := range changes.Sections(items) {
		issues := section.Issues()
		if len(issues) == 0 {
			continue
		}

		listed = true

		fmt.Fprintf(&b, "\n### %s\n\n", section.Group.Title())

		for _, n := range issues {
			fmt.Fprintf(&b, "* %s\n", refs.Ref(n))
		}
	}

	if !listed {
		b.WriteString("\nNo issues.\n")
	}

	if unlinked {
		writeIssueless(&b, items, refs)
	}

	return b.String()
}

func writeIssueless(b *strings.Builder, items []changes.Item, refs IssueRefs) {
	var issueless []changes.Item

	for _, item := range items {
		if changes.Issueless(item) {
			issueless = append(issueless, item)
		}
	}

	if len(issueless) == 0 {
		return
	}

	slices.SortFunc(issueless, func(a, b changes.Item) int { return cmp.Compare(a.PR, b.PR) })

	b.WriteString("\n### Pull requests without an issue\n\n")

	for _, item := range issueless {
		fmt.Fprintf(b, "* %s %s\n", refs.PR(item.PR), item.Title)
	}
}
