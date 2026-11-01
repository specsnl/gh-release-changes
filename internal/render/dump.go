package render

import (
	"fmt"
	"slices"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

// Dump renders the issue dump for the writing skills: per group, every issue
// the release references — its state, labels, description, comments, and the
// pull requests that close it — followed by the features and fixes that
// reference no issue, described by their own pull request.
//
// An issue appears once, under the first group that references it. A breaking
// change is marked on the issue or pull request it belongs to. Whatever could
// not be read is noted in place, with GitHub's reason, so the dump still says
// what the release contains.
//
// Issue references are written with refs, so issues from another repository
// read OWNER/REPO#N; pull requests are always the source repository's.
func Dump(release changes.Release, details changes.Details, refs IssueRefs) string {
	d := dumper{release: release, details: details, refs: refs}

	d.header()

	written := false
	seen := map[int]bool{}

	for _, section := range changes.Sections(release.Items) {
		var issues []int

		for _, n := range section.Issues() {
			if !seen[n] {
				seen[n] = true
				issues = append(issues, n)
			}
		}

		untracked := slices.DeleteFunc(slices.Clone(section.Items), func(item changes.Item) bool {
			return !changes.Untracked(item)
		})

		if len(issues) == 0 && len(untracked) == 0 {
			continue
		}

		written = true

		fmt.Fprintf(&d.b, "\n## %s\n", section.Group.Title())

		for _, n := range issues {
			d.issue(n)
		}

		for _, item := range untracked {
			d.pullRequest(item)
		}
	}

	if !written {
		d.b.WriteString("\nNo issues or pull requests to describe.\n")
	}

	return d.b.String()
}

type dumper struct {
	b       strings.Builder
	release changes.Release
	details changes.Details
	refs    IssueRefs
}

func (d *dumper) header() {
	previous := d.release.PreviousTag
	if previous == "" {
		previous = "nothing (first release)"
	}

	fmt.Fprintf(&d.b, "# Release %s\n\n", d.release.Tag)
	fmt.Fprintf(&d.b, "* Repository: %s\n", d.release.Repo)
	fmt.Fprintf(&d.b, "* Compared with: %s\n", previous)
	fmt.Fprintf(&d.b, "* Issues from: %s\n", d.refs.IssueRepo)
}

func (d *dumper) issue(n int) {
	items := d.referencing(n)

	issue, ok := d.details.Issues[n]
	if !ok {
		fmt.Fprintf(&d.b, "\n### Issue %s\n\n", d.refs.Ref(n))
		fmt.Fprintf(&d.b, "_Could not be read: %s_\n\n", sentence(d.details.UnreadableIssues[n]))
		d.breaking(items)
		d.releaseItems(items)

		return
	}

	fmt.Fprintf(&d.b, "\n### Issue %s: %s\n\n", d.refs.Ref(n), oneLine(issue.Title))
	fmt.Fprintf(&d.b, "* State: %s\n", issue.State)
	fmt.Fprintf(&d.b, "* Labels: %s\n", orNone(strings.Join(issue.Labels, ", ")))
	fmt.Fprintf(&d.b, "* URL: %s\n", issue.URL)
	d.breaking(items)
	d.releaseItems(items)

	d.text("Description", issue.Body, "_No description._")

	d.b.WriteString("\n#### Comments\n")

	if len(issue.Comments) == 0 {
		d.b.WriteString("\n_No comments._\n")
	}

	for _, comment := range issue.Comments {
		text := body(comment.Body)
		if text == "" {
			text = "_Empty comment._"
		}

		fmt.Fprintf(&d.b, "\n##### @%s\n\n%s\n", comment.Author, text)
	}

	if more := issue.CommentCount - len(issue.Comments); more > 0 {
		fmt.Fprintf(&d.b, "\n_%d more %s not shown._\n", more, plural(more, "comment", "comments"))
	}

	d.b.WriteString("\n#### Linked pull requests\n")

	if len(issue.ClosedBy) == 0 {
		d.b.WriteString("\n_No linked pull requests._\n")
	}

	for _, pr := range issue.ClosedBy {
		fmt.Fprintf(&d.b, "\n##### %s: %s\n\n", d.prRef(pr), oneLine(pr.Title))
		fmt.Fprintf(&d.b, "* State: %s\n", pr.State)
		fmt.Fprintf(&d.b, "* Author: @%s\n", pr.Author)
		fmt.Fprintf(&d.b, "* URL: %s\n", pr.URL)

		if text := body(pr.Body); text != "" {
			fmt.Fprintf(&d.b, "\n%s\n", text)
		}
	}
}

// pullRequest describes a feature or fix that references no issue. Its title,
// author and URL come from the generated notes; only the description needs the
// pull request itself, so an unreadable one still says what changed.
func (d *dumper) pullRequest(item changes.Item) {
	fmt.Fprintf(&d.b, "\n### Pull request #%d: %s\n\n", item.PR, item.Title)
	fmt.Fprintf(&d.b, "* Author: @%s\n", item.Author)
	fmt.Fprintf(&d.b, "* URL: %s\n", item.URL)
	d.breaking([]changes.Item{item})

	pr, ok := d.details.PullRequests[item.PR]
	if !ok {
		fmt.Fprintf(&d.b, "\n#### Description\n\n_Could not be read: %s_\n",
			sentence(d.details.UnreadablePullRequests[item.PR]))

		return
	}

	d.text("Description", pr.Body, "_No description._")
}

// referencing returns the release's items that reference issue n, in display
// order.
func (d *dumper) referencing(n int) []changes.Item {
	var items []changes.Item

	for _, item := range d.release.Items {
		if slices.Contains(item.Issues, n) {
			items = append(items, item)
		}
	}

	return items
}

// breaking marks the entry when any of its items is a breaking change.
func (d *dumper) breaking(items []changes.Item) {
	if slices.ContainsFunc(items, func(item changes.Item) bool { return item.Breaking }) {
		d.b.WriteString("* **Breaking change**\n")
	}
}

// releaseItems lists the release's pull requests for an issue, in the line
// format of [Sort].
func (d *dumper) releaseItems(items []changes.Item) {
	d.b.WriteString("* Pull requests in this release:\n")

	for _, item := range items {
		d.b.WriteString("  * ")

		if item.Breaking {
			d.b.WriteString(BreakingMarker)
		}

		fmt.Fprintf(&d.b, "%s by @%s in %s\n", item.Title, item.Author, item.URL)
	}
}

func (d *dumper) text(heading, text, empty string) {
	fmt.Fprintf(&d.b, "\n#### %s\n\n", heading)

	if text = body(text); text == "" {
		text = empty
	}

	fmt.Fprintf(&d.b, "%s\n", text)
}

// prRef is #N for a pull request of the source repository, OWNER/REPO#N for
// one elsewhere.
func (d *dumper) prRef(pr changes.PullRequest) string {
	if pr.Repo == "" || strings.EqualFold(pr.Repo, d.release.Repo) {
		return fmt.Sprintf("#%d", pr.Number)
	}

	return fmt.Sprintf("%s#%d", pr.Repo, pr.Number)
}

// body normalises text written on GitHub: CRLF line endings become LF, and
// surrounding blank space goes, so the same issue always renders the same
// bytes.
func body(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")

	return strings.TrimSpace(text)
}

// oneLine keeps a title on its heading line.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// sentence ends reason with a full stop, for an italic note.
func sentence(reason string) string {
	if reason == "" {
		reason = "no reason given"
	}

	reason = strings.TrimSpace(reason)
	if !strings.HasSuffix(reason, ".") {
		reason += "."
	}

	return reason
}

func orNone(text string) string {
	if text == "" {
		return "none"
	}

	return text
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}

	return many
}
