package changes

import (
	"cmp"
	"slices"
)

// Sort orders items in place for display: by group in [Groups] order, then
// within a group by first issue number and then PR number, with items that
// reference no issue at the bottom. Dependencies are ordered by PR number
// alone — their scopes are never issues.
//
// The order is total, so the same body always renders the same bytes.
func Sort(items []Item) {
	slices.SortStableFunc(items, compare)
}

// Sections returns the groups that have items, in display order, each with its
// items sorted.
func Sections(items []Item) []Section {
	sorted := slices.Clone(items)
	Sort(sorted)

	var sections []Section

	for _, item := range sorted {
		if n := len(sections); n > 0 && sections[n-1].Group == item.Group {
			sections[n-1].Items = append(sections[n-1].Items, item)

			continue
		}

		sections = append(sections, Section{Group: item.Group, Items: []Item{item}})
	}

	return sections
}

// Issues returns the issue numbers the section's items reference, ascending
// and deduplicated; nil when none of them references an issue.
func (s Section) Issues() []int {
	var issues []int

	for _, item := range s.Items {
		issues = append(issues, item.Issues...)
	}

	slices.Sort(issues)

	return slices.Compact(issues)
}

func compare(a, b Item) int {
	if c := cmp.Compare(groupRank(a.Group), groupRank(b.Group)); c != 0 {
		return c
	}

	if a.Group != GroupDependencies {
		if c := compareIssues(a.Issues, b.Issues); c != 0 {
			return c
		}
	}

	return cmp.Compare(a.PR, b.PR)
}

// compareIssues orders by first issue number, with no issue last.
func compareIssues(a, b []int) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	default:
		return cmp.Compare(a[0], b[0])
	}
}

// groupRank is the position of g in [Groups]; an unknown group sorts last.
func groupRank(g Group) int {
	if i := slices.Index(Groups, g); i >= 0 {
		return i
	}

	return len(Groups)
}
