// Package changes turns the body GitHub's generate-notes endpoint returns into
// classified, ordered items — the single extraction step every output is
// rendered from.
//
// It is pure: it never imports internal/github, takes the body as a string, and
// returns plain structs. The parser rules, the grouping, and the ordering are
// therefore testable with table tests and golden files, without HTTP.
package changes

// Group is the identifier of the group an item lands in. The identifiers are
// public API — they appear in the items JSON — so they may be added to, never
// renamed.
type Group string

const (
	// GroupFeatures holds feat items.
	GroupFeatures Group = "features"

	// GroupFixes holds fix items.
	GroupFixes Group = "fixes"

	// GroupOther holds docs, test, ci, refactor, perf, style, build, and revert
	// items.
	GroupOther Group = "other"

	// GroupMaintenance holds chore items.
	GroupMaintenance Group = "maintenance"

	// GroupDependencies holds dependency updates: a deps or deps-dev scope, or a
	// dependabot author, whatever the type.
	GroupDependencies Group = "dependencies"

	// GroupUnclassified holds titles outside the <type>[(<scopes>)][!]: <subject>
	// format. They are shown, never dropped.
	GroupUnclassified Group = "unclassified"
)

// Groups lists every group in display order.
var Groups = []Group{
	GroupFeatures,
	GroupFixes,
	GroupOther,
	GroupMaintenance,
	GroupDependencies,
	GroupUnclassified,
}

// Title is the heading a renderer prints for the group. Unlike the identifier
// it is prose, and may be reworded.
func (g Group) Title() string {
	switch g {
	case GroupFeatures:
		return "Features"
	case GroupFixes:
		return "Bug fixes"
	case GroupOther:
		return "Other"
	case GroupMaintenance:
		return "Maintenance"
	case GroupDependencies:
		return "Dependencies"
	case GroupUnclassified:
		return "Unclassified"
	default:
		return string(g)
	}
}

// Item is one pull request from the generated notes. The json tags are the
// items contract: fields may be added, never renamed.
//
// Scopes and Issues are never nil, so they marshal as [] rather than null. For
// an unclassified item Type is "", Scopes and Issues are empty, and Breaking is
// false: nothing of a title outside the format is trusted.
type Item struct {
	PR       int      `json:"pr"`
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	Scopes   []string `json:"scopes"`
	Breaking bool     `json:"breaking"`
	Issues   []int    `json:"issues"`
	Author   string   `json:"author"`
	Bot      bool     `json:"bot"`
	URL      string   `json:"url"`
	Group    Group    `json:"group"`
}

// Release is the items JSON document: the repository and range the items were
// generated for, and the items in display order.
type Release struct {
	Repo        string `json:"repo"`
	Tag         string `json:"tag"`
	PreviousTag string `json:"previous_tag"`
	Items       []Item `json:"items"`
}

// Section is one group with its items, for a renderer that prints a heading per
// group.
type Section struct {
	Group Group
	Items []Item
}
