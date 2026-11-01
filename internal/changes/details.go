package changes

import "slices"

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
