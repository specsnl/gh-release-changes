package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// detailsBatch is how many issues and pull requests one GraphQL query asks
// for. Each issue brings up to 100 comments, so the batch is kept well under
// GitHub's node limit and response size.
const detailsBatch = 25

// Page sizes of the connections an issue brings along. Comments beyond the
// first page are counted, not fetched; dump says how many it left out.
const (
	labelsPage   = 50
	commentsPage = 100
	closedByPage = 20
)

// Details fetches the issues of issueRepo (OWNER/REPO, on the client's host)
// and the pull requests of the client's repository, in as few GraphQL queries
// as the batch size allows.
//
// An issue or pull request that cannot be read — it does not exist, the number
// is a pull request rather than an issue, the token may not see it — does not
// fail the call: it lands in an Unreadable map with GitHub's reason. Only a
// failure of the request as a whole is returned as an error.
func (c *Client) Details(ctx context.Context, issueRepo string, issues, pullRequests []int) (changes.Details, error) {
	owner, name, err := splitIssueRepo(issueRepo)
	if err != nil {
		return changes.Details{}, err
	}

	details := newDetails()

	for len(issues) > 0 || len(pullRequests) > 0 {
		var batchIssues, batchPRs []int

		batchIssues, issues = cut(issues, detailsBatch)
		batchPRs, pullRequests = cut(pullRequests, detailsBatch-len(batchIssues))

		q := detailsQuery{
			issueOwner: owner, issueName: name, issues: batchIssues,
			prOwner: c.repo.Owner, prName: c.repo.Name, pullRequests: batchPRs,
		}

		if err := c.fetchDetails(ctx, q, &details); err != nil {
			return changes.Details{}, err
		}
	}

	slog.Debug("details fetched", "issue_repo", issueRepo,
		"issues", len(details.Issues), "pull_requests", len(details.PullRequests),
		"unreadable", len(details.UnreadableIssues)+len(details.UnreadablePullRequests))

	return details, nil
}

// IssueTitles fetches only the titles of the issues of issueRepo (OWNER/REPO,
// on the client's host), one GraphQL query per batch. What cannot be read
// lands in Unreadable, as for [Client.Details].
func (c *Client) IssueTitles(ctx context.Context, issueRepo string, issues []int) (changes.IssueTitles, error) {
	owner, name, err := splitIssueRepo(issueRepo)
	if err != nil {
		return changes.IssueTitles{}, err
	}

	details := newDetails()

	for len(issues) > 0 {
		var batch []int

		batch, issues = cut(issues, detailsBatch)

		q := detailsQuery{issueOwner: owner, issueName: name, issues: batch, titlesOnly: true}

		if err := c.fetchDetails(ctx, q, &details); err != nil {
			return changes.IssueTitles{}, err
		}
	}

	titles := changes.IssueTitles{Titles: map[int]string{}, Unreadable: details.UnreadableIssues}
	for n, issue := range details.Issues {
		titles.Titles[n] = issue.Title
	}

	slog.Debug("issue titles fetched", "issue_repo", issueRepo,
		"titles", len(titles.Titles), "unreadable", len(titles.Unreadable))

	return titles, nil
}

func splitIssueRepo(issueRepo string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(issueRepo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("%w: %s", releasechanges.ErrInvalidRepoRef, issueRepo)
	}

	return owner, name, nil
}

func newDetails() changes.Details {
	return changes.Details{
		Issues:                 map[int]changes.Issue{},
		PullRequests:           map[int]changes.PullRequest{},
		UnreadableIssues:       map[int]string{},
		UnreadablePullRequests: map[int]string{},
	}
}

// cut splits off the first n numbers.
func cut(numbers []int, n int) (head, tail []int) {
	n = min(n, len(numbers))

	return numbers[:n], numbers[n:]
}

// detailsQuery is one batch: issues from one repository, pull requests from
// another (or the same).
type detailsQuery struct {
	issueOwner, issueName string
	issues                []int

	// titlesOnly selects an issue's number and title, and nothing else.
	titlesOnly bool

	prOwner, prName string
	pullRequests    []int
}

// The two top-level fields of the query. Each holds one aliased field per
// number, so a missing number is a null under its own alias, with a GraphQL
// error whose path names it, and the rest of the batch still answers.
const (
	issuesField       = "issues"
	pullRequestsField = "pullRequests"
)

func issueAlias(n int) string { return "i" + strconv.Itoa(n) }

func pullRequestAlias(n int) string { return "p" + strconv.Itoa(n) }

const pullRequestFragment = `fragment pullRequest on PullRequest {
  number title body state url
  author { login }
  repository { nameWithOwner }
}
`

const issueTitleFragment = `fragment issueTitle on Issue {
  number title
}
`

var issueFragment = fmt.Sprintf(`fragment issue on Issue {
  number title body state url
  labels(first: %d) { nodes { name } }
  comments(first: %d) { totalCount nodes { author { login } body } }
  closedByPullRequestsReferences(first: %d, includeClosedPrs: true) { nodes { ...pullRequest } }
}
`, labelsPage, commentsPage, closedByPage)

// text builds the query and its variables. Only what the batch uses is
// declared: GraphQL rejects an unused variable or fragment.
func (q detailsQuery) text() (string, map[string]any) {
	var params, fields []string

	vars := map[string]any{}

	if len(q.issues) > 0 {
		params = append(params, "$issueOwner: String!", "$issueName: String!")
		vars["issueOwner"], vars["issueName"] = q.issueOwner, q.issueName

		fragment := "issue"
		if q.titlesOnly {
			fragment = "issueTitle"
		}

		aliases := make([]string, len(q.issues))
		for i, n := range q.issues {
			aliases[i] = fmt.Sprintf("%s: issue(number: %d) { ...%s }", issueAlias(n), n, fragment)
		}

		fields = append(fields, fmt.Sprintf("%s: repository(owner: $issueOwner, name: $issueName) {\n    %s\n  }",
			issuesField, strings.Join(aliases, "\n    ")))
	}

	if len(q.pullRequests) > 0 {
		params = append(params, "$prOwner: String!", "$prName: String!")
		vars["prOwner"], vars["prName"] = q.prOwner, q.prName

		aliases := make([]string, len(q.pullRequests))
		for i, n := range q.pullRequests {
			aliases[i] = fmt.Sprintf("%s: pullRequest(number: %d) { ...pullRequest }", pullRequestAlias(n), n)
		}

		fields = append(fields, fmt.Sprintf("%s: repository(owner: $prOwner, name: $prName) {\n    %s\n  }",
			pullRequestsField, strings.Join(aliases, "\n    ")))
	}

	var b strings.Builder

	fmt.Fprintf(&b, "query(%s) {\n  %s\n}\n", strings.Join(params, ", "), strings.Join(fields, "\n  "))

	switch {
	case len(q.issues) > 0 && q.titlesOnly:
		b.WriteString(issueTitleFragment)
	case len(q.issues) > 0:
		b.WriteString(issueFragment)
	}

	// The full issue fragment spreads ...pullRequest for its closing pull
	// requests.
	if len(q.pullRequests) > 0 || (len(q.issues) > 0 && !q.titlesOnly) {
		b.WriteString(pullRequestFragment)
	}

	return b.String(), vars
}

type issueNode struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	URL    string `json:"url"`
	Labels struct {
		Nodes []*struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Comments struct {
		TotalCount int `json:"totalCount"`
		Nodes      []*struct {
			Author *actor `json:"author"`
			Body   string `json:"body"`
		} `json:"nodes"`
	} `json:"comments"`
	ClosedBy struct {
		Nodes []*pullRequestNode `json:"nodes"`
	} `json:"closedByPullRequestsReferences"`
}

type pullRequestNode struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	State      string `json:"state"`
	URL        string `json:"url"`
	Author     *actor `json:"author"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

// actor is a GitHub user or bot; null for a deleted account.
type actor struct {
	Login string `json:"login"`
}

// login is the actor's login, or "ghost" — GitHub's name for a deleted
// account, whose author field is null.
func (a *actor) login() string {
	if a == nil {
		return "ghost"
	}

	return a.Login
}

func (c *Client) fetchDetails(ctx context.Context, q detailsQuery, details *changes.Details) error {
	query, vars := q.text()

	var data struct {
		Issues       map[string]*issueNode       `json:"issues"`
		PullRequests map[string]*pullRequestNode `json:"pullRequests"`
	}

	err := c.gql.DoWithContext(ctx, query, vars, &data)

	reasons, err := graphQLReasons(err)
	if err != nil {
		return c.mapError(err, releasechanges.ErrRepoNotFound, c.Repo())
	}

	for _, n := range q.issues {
		node := data.Issues[issueAlias(n)]
		if node == nil {
			details.UnreadableIssues[n] = reasonFor(reasons, issuesField, issueAlias(n))

			continue
		}

		details.Issues[n] = node.issue()
	}

	for _, n := range q.pullRequests {
		node := data.PullRequests[pullRequestAlias(n)]
		if node == nil {
			details.UnreadablePullRequests[n] = reasonFor(reasons, pullRequestsField, pullRequestAlias(n))

			continue
		}

		details.PullRequests[n] = node.pullRequest()
	}

	return nil
}

// graphQLReasons splits a query's error into the per-field reasons GitHub gave
// — keyed by the dotted path of the field — and an error that fails the query
// as a whole. A GraphQL error without a path, or any other error, is the
// latter.
func graphQLReasons(err error) (map[string]string, error) {
	if err == nil {
		return nil, nil
	}

	var gqlErr *api.GraphQLError
	if !errors.As(err, &gqlErr) {
		return nil, err
	}

	reasons := map[string]string{}

	for _, item := range gqlErr.Errors {
		if len(item.Path) == 0 {
			return nil, err
		}

		slog.Debug("GraphQL error", "type", item.Type, "path", item.Path, "message", item.Message)

		parts := make([]string, 0, 2)
		for _, p := range item.Path[:min(2, len(item.Path))] {
			parts = append(parts, fmt.Sprint(p))
		}

		key := strings.Join(parts, ".")
		if _, seen := reasons[key]; !seen {
			reasons[key] = item.Message
		}
	}

	return reasons, nil
}

// reasonFor is GitHub's reason for a null field: the error on the field itself,
// or on the repository above it.
func reasonFor(reasons map[string]string, field, alias string) string {
	if reason, ok := reasons[field+"."+alias]; ok {
		return reason
	}

	if reason, ok := reasons[field]; ok {
		return reason
	}

	return "not returned by GitHub"
}

func (n *issueNode) issue() changes.Issue {
	issue := changes.Issue{
		Number:       n.Number,
		Title:        n.Title,
		Body:         n.Body,
		State:        strings.ToLower(n.State),
		URL:          n.URL,
		Labels:       []string{},
		Comments:     []changes.Comment{},
		CommentCount: n.Comments.TotalCount,
		ClosedBy:     []changes.PullRequest{},
	}

	for _, label := range n.Labels.Nodes {
		if label != nil {
			issue.Labels = append(issue.Labels, label.Name)
		}
	}

	for _, comment := range n.Comments.Nodes {
		if comment != nil {
			issue.Comments = append(issue.Comments, changes.Comment{Author: comment.Author.login(), Body: comment.Body})
		}
	}

	for _, pr := range n.ClosedBy.Nodes {
		if pr != nil {
			issue.ClosedBy = append(issue.ClosedBy, pr.pullRequest())
		}
	}

	return issue
}

func (n *pullRequestNode) pullRequest() changes.PullRequest {
	return changes.PullRequest{
		Number: n.Number,
		Repo:   n.Repository.NameWithOwner,
		Title:  n.Title,
		Body:   n.Body,
		Author: n.Author.login(),
		State:  strings.ToLower(n.State),
		URL:    n.URL,
	}
}
