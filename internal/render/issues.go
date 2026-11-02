package render

import (
	"fmt"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

// IssueRefs decides how [Issues] writes a reference.
type IssueRefs struct {
	// IssueRepo is the OWNER/REPO the digit scopes point at.
	IssueRepo string

	// RelativeTo is the OWNER/REPO references are written relative to, usually
	// the one the list is pasted into. A reference is short (#N) when it equals
	// IssueRepo, and full (OWNER/REPO#N) otherwise.
	RelativeTo string

	// Full forces the full form everywhere.
	Full bool
}

// Ref formats issue n.
func (r IssueRefs) Ref(n int) string {
	// GitHub resolves owner and repository names case-insensitively.
	if !r.Full && strings.EqualFold(r.IssueRepo, r.RelativeTo) {
		return fmt.Sprintf("#%d", n)
	}

	return fmt.Sprintf("%s#%d", r.IssueRepo, n)
}

// Issues renders the plain issue list for the release ticket: a heading per
// group that references issues, in display order, and its issues ascending and
// deduplicated, one reference per line.
func Issues(items []changes.Item, refs IssueRefs) string {
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

	return b.String()
}
