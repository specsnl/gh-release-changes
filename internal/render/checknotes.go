package render

import (
	"fmt"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

// CheckNotes renders how release notes compare with the release: the issues of
// the release missing from the notes, and the references in the notes outside
// the release, each section left out when empty. Missing issues are in
// refs.IssueRepo; both are written relative to refs.RelativeTo.
func CheckNotes(coverage changes.Coverage, refs IssueRefs) string {
	var b strings.Builder

	b.WriteString("## Notes check\n")

	if coverage.Covered() {
		b.WriteString("\nThe notes reference every issue of the release, and nothing outside it.\n")

		return b.String()
	}

	if len(coverage.Missing) > 0 {
		b.WriteString("\n### Issues missing from the notes\n\n")

		for _, n := range coverage.Missing {
			fmt.Fprintf(&b, "* %s\n", refs.Ref(n))
		}
	}

	if len(coverage.Unknown) > 0 {
		b.WriteString("\n### References outside the release\n\n")

		for _, ref := range coverage.Unknown {
			fmt.Fprintf(&b, "* %s\n", refs.ref(ref.Repo, ref.Number))
		}
	}

	return b.String()
}
