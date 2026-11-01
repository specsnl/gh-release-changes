package render

import (
	"fmt"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

// BreakingMarker prefixes the line of a breaking change in [Sort], so it stands
// out in a long list while the title itself stays as written.
const BreakingMarker = "**Breaking:** "

// Sort renders the grouped "What's Changed" list: a heading per non-empty group
// in display order, and the items in GitHub's own line format,
//
//	* <title> by @<author> in <pr-url>
//
// so the links still render where the list is pasted.
func Sort(items []changes.Item) string {
	var b strings.Builder

	b.WriteString("## What's Changed\n")

	sections := changes.Sections(items)
	if len(sections) == 0 {
		b.WriteString("\nNo changes.\n")

		return b.String()
	}

	for _, section := range sections {
		fmt.Fprintf(&b, "\n### %s\n\n", section.Group.Title())

		for _, item := range section.Items {
			b.WriteString("* ")

			if item.Breaking {
				b.WriteString(BreakingMarker)
			}

			fmt.Fprintf(&b, "%s by @%s in %s\n", item.Title, item.Author, item.URL)
		}
	}

	return b.String()
}
