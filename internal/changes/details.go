package changes

import (
	"maps"
	"slices"
	"strings"
)

// Issue is what dump shows of a referenced issue. It is a plain struct, filled
// in by internal/github and rendered by internal/render, which never meet.
type Issue struct {
	Number int
	Title  string
	Body   string

	// State is GitHub's, lowercased: open or closed.
	State  string
	URL    string
	Labels []string

	// Comments are the first page of the issue's comments, oldest first;
	// CommentCount is how many there are in total.
	Comments     []Comment
	CommentCount int

	// ClosedBy are the pull requests that close the issue through a closing
	// keyword or a manual link, merged or not.
	ClosedBy []PullRequest
}

// Links reports whether pull request pr of repo is one the issue lists in
// ClosedBy. A pull request without a repository is taken to be repo's.
func (i Issue) Links(repo string, pr int) bool {
	return slices.ContainsFunc(i.ClosedBy, func(p PullRequest) bool {
		return p.Number == pr && (p.Repo == "" || strings.EqualFold(p.Repo, repo))
	})
}

// Comment is one comment on an issue.
type Comment struct {
	Author string
	Body   string
}

// PullRequest is what dump shows of a pull request.
type PullRequest struct {
	Number int

	// Repo is the OWNER/REPO the pull request lives in: a pull request that
	// closes an issue may live elsewhere.
	Repo   string
	Title  string
	Body   string
	Author string

	// State is GitHub's, lowercased: open, closed, or merged.
	State string
	URL   string
}

// Details are the issues and pull requests dump fetched for a release, keyed
// by number. A number that could not be read is in an Unreadable map instead,
// with the reason, so the dump can say so in place rather than leave it out.
type Details struct {
	Issues       map[int]Issue
	PullRequests map[int]PullRequest

	UnreadableIssues       map[int]string
	UnreadablePullRequests map[int]string
}

// Merge adds the issues and pull requests of other to d.
func (d *Details) Merge(other Details) {
	d.Issues = merged(d.Issues, other.Issues)
	d.PullRequests = merged(d.PullRequests, other.PullRequests)
	d.UnreadableIssues = merged(d.UnreadableIssues, other.UnreadableIssues)
	d.UnreadablePullRequests = merged(d.UnreadablePullRequests, other.UnreadablePullRequests)
}

func merged[V any](dst, src map[int]V) map[int]V {
	if len(src) == 0 {
		return dst
	}

	if dst == nil {
		dst = map[int]V{}
	}

	maps.Copy(dst, src)

	return dst
}

// ReferencedIssues returns every issue number the items reference, ascending
// and deduplicated: the issues dump describes.
func ReferencedIssues(items []Item) []int {
	var issues []int

	for _, item := range items {
		issues = append(issues, item.Issues...)
	}

	slices.Sort(issues)

	return slices.Compact(issues)
}

// Untracked reports whether item is a feature or a fix that references no
// issue. dump describes such a pull request itself, since there is no issue
// to describe it; a dependency update is neither, whatever its type.
func Untracked(item Item) bool {
	return (item.Group == GroupFeatures || item.Group == GroupFixes) && len(item.Issues) == 0
}

// UnlinkedPullRequests returns the release's pull requests that reference one
// of issues by their title while that issue does not link them on GitHub —
// "Related to #N" is no closing keyword — ascending and deduplicated. dump
// fetches them to describe them under the issue, since its linked pull
// requests do not. An issue missing from issues was not read and is skipped.
func UnlinkedPullRequests(release Release, issues map[int]Issue) []int {
	var prs []int

	for _, item := range release.Items {
		for _, n := range item.Issues {
			if issue, ok := issues[n]; ok && !issue.Links(release.Repo, item.PR) {
				prs = append(prs, item.PR)
			}
		}
	}

	slices.Sort(prs)

	return slices.Compact(prs)
}
