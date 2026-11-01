package changes

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	// lineRe matches `* <title> by @<author> in <pr-url>`. The title is greedy,
	// so a title that itself contains " by @" still splits at the last one. The
	// URL may be on any host, which is what makes GHES bodies parse.
	lineRe = regexp.MustCompile(`^\* (.+) by @(\S+) in (https?://\S+/pull/(\d+))$`)

	// titleRe matches `<type>[(<scopes>)][!]: <subject>`, the format the
	// PR-title check enforces. Validating the type itself is left to types.
	titleRe = regexp.MustCompile(`^([a-z]+)(?:\(([^()]+)\))?(!)?: \S`)

	issueScopeRe = regexp.MustCompile(`^\d+$`)
)

// types maps every accepted type — the PR-title action's defaults — to its
// group.
var types = map[string]Group{
	"feat":     GroupFeatures,
	"fix":      GroupFixes,
	"docs":     GroupOther,
	"style":    GroupOther,
	"refactor": GroupOther,
	"perf":     GroupOther,
	"test":     GroupOther,
	"build":    GroupOther,
	"ci":       GroupOther,
	"revert":   GroupOther,
	"chore":    GroupMaintenance,
}

// dependabot is the author whose pull requests are dependency updates whatever
// their title says.
const dependabot = "dependabot[bot]"

// Parse turns a generate-notes body into items, sorted into display order.
//
// Only `* <title> by @<author> in <pr-url>` lines become items. Headings — the
// `###` categories a .github/release.yml adds included — are ignored, as are
// the "New Contributors" section and the "Full Changelog" line, so the result
// is the same with or without that file.
func Parse(body string) []Item {
	items := []Item{}
	inNewContributors := false

	for line := range strings.Lines(body) {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "## ") {
			inNewContributors = line == "## New Contributors"

			continue
		}

		if inNewContributors {
			continue
		}

		if item, ok := parseLine(line); ok {
			items = append(items, item)
		}
	}

	Sort(items)

	return items
}

// parseLine turns one list line into an item, or reports false for anything
// else.
func parseLine(line string) (Item, bool) {
	m := lineRe.FindStringSubmatch(line)
	if m == nil {
		return Item{}, false
	}

	pr, err := strconv.Atoi(m[4])
	if err != nil {
		return Item{}, false
	}

	item := Item{
		PR:     pr,
		Title:  m[1],
		Scopes: []string{},
		Issues: []int{},
		Author: m[2],
		Bot:    strings.HasSuffix(m[2], "[bot]"),
		URL:    m[3],
		Group:  GroupUnclassified,
	}

	classify(&item)

	return item, true
}

// classify fills in the type, scopes, breaking marker, issues, and group from
// the title. A title outside the format stays unclassified — unless dependabot
// wrote it, because a dependency update is one whatever its title says.
func classify(item *Item) {
	if item.Author == dependabot {
		item.Group = GroupDependencies
	}

	m := titleRe.FindStringSubmatch(item.Title)
	if m == nil {
		return
	}

	group, ok := types[m[1]]
	if !ok {
		return
	}

	item.Type = m[1]
	item.Breaking = m[3] == "!"

	if m[2] != "" {
		for scope := range strings.SplitSeq(m[2], ",") {
			item.Scopes = append(item.Scopes, strings.TrimSpace(scope))
		}
	}

	for _, scope := range item.Scopes {
		if !issueScopeRe.MatchString(scope) {
			continue
		}

		// The pattern guarantees digits; only an absurdly long scope overflows.
		n, err := strconv.Atoi(scope)
		if err == nil && !slices.Contains(item.Issues, n) {
			item.Issues = append(item.Issues, n)
		}
	}

	if item.Group != GroupDependencies {
		item.Group = group
	}

	if slices.Contains(item.Scopes, "deps") || slices.Contains(item.Scopes, "deps-dev") {
		item.Group = GroupDependencies
	}
}
