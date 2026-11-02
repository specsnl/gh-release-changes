package changes

import "slices"

// LinkClosing adds to every item the issues its pull request closes on GitHub
// — closing maps a PR number to those issue numbers — after the issues of its
// title, and records both sources in IssueSources. Every item gets
// IssueSources, also one closing nothing.
//
// The items are sorted again: an item that referenced no issue may now, and
// the order is by first issue.
func LinkClosing(items []Item, closing map[int][]int) {
	for i := range items {
		item := &items[i]

		closes := append([]int{}, closing[item.PR]...)
		slices.Sort(closes)
		closes = slices.Compact(closes)

		item.IssueSources = &IssueSources{Title: append([]int{}, item.Issues...), Closing: closes}

		for _, n := range closes {
			if !slices.Contains(item.Issues, n) {
				item.Issues = append(item.Issues, n)
			}
		}
	}

	Sort(items)
}
