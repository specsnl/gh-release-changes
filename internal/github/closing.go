package github

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// closingBatch is how many pull requests one closing-references query asks
// for. Each brings a page of small nodes, so a release is usually one query.
const closingBatch = 100

// closingPage is how many closing references of one pull request are read.
// More are left out, and logged at debug level.
const closingPage = 25

// closingField is the top-level field of the query: the source repository,
// with one aliased pullRequest field per number under it.
const closingField = "repository"

// ClosingIssues returns, per pull request of the client's repository, the
// issues it closes on GitHub (closingIssuesReferences) that are in issueRepo
// (OWNER/REPO, on the client's host). A pull request closing none is absent.
//
// An issue in another repository cannot be told apart from issueRepo's by its
// number, so it is left out, and logged at debug level.
//
// The pull requests come from the release's generated notes, so each one
// exists: one that cannot be read fails the call.
func (c *Client) ClosingIssues(ctx context.Context, issueRepo string, pullRequests []int) (map[int][]int, error) {
	if _, _, err := splitIssueRepo(issueRepo); err != nil {
		return nil, err
	}

	closing := map[int][]int{}

	for len(pullRequests) > 0 {
		var batch []int

		batch, pullRequests = cut(pullRequests, closingBatch)

		if err := c.fetchClosing(ctx, issueRepo, batch, closing); err != nil {
			return nil, err
		}
	}

	slog.Debug("closing references fetched", "issue_repo", issueRepo, "pull_requests_closing", len(closing))

	return closing, nil
}

type closingNode struct {
	ClosingIssuesReferences struct {
		TotalCount int `json:"totalCount"`
		Nodes      []*struct {
			Number     int `json:"number"`
			Repository struct {
				NameWithOwner string `json:"nameWithOwner"`
			} `json:"repository"`
		} `json:"nodes"`
	} `json:"closingIssuesReferences"`
}

func (c *Client) fetchClosing(ctx context.Context, issueRepo string, batch []int, closing map[int][]int) error {
	aliases := make([]string, len(batch))
	for i, n := range batch {
		aliases[i] = fmt.Sprintf("%s: pullRequest(number: %d) { ...closing }", pullRequestAlias(n), n)
	}

	query := fmt.Sprintf(`query($owner: String!, $name: String!) {
  %s: repository(owner: $owner, name: $name) {
    %s
  }
}
fragment closing on PullRequest {
  closingIssuesReferences(first: %d) { totalCount nodes { number repository { nameWithOwner } } }
}
`, closingField, strings.Join(aliases, "\n    "), closingPage)

	var data struct {
		Repository map[string]*closingNode `json:"repository"`
	}

	err := c.gql.DoWithContext(ctx, query, map[string]any{"owner": c.repo.Owner, "name": c.repo.Name}, &data)

	reasons, err := graphQLReasons(err)
	if err != nil {
		return c.mapError(err, releasechanges.ErrRepoNotFound, c.Repo())
	}

	for _, n := range batch {
		node := data.Repository[pullRequestAlias(n)]
		if node == nil {
			return fmt.Errorf("read the closing references of pull request #%d of %s: %s",
				n, c.Repo(), reasonFor(reasons, closingField, pullRequestAlias(n)))
		}

		refs := node.ClosingIssuesReferences
		if refs.TotalCount > len(refs.Nodes) {
			slog.Debug("closing references left out", "pull_request", n, "total", refs.TotalCount, "read", len(refs.Nodes))
		}

		for _, issue := range refs.Nodes {
			if issue == nil {
				continue
			}

			if !strings.EqualFold(issue.Repository.NameWithOwner, issueRepo) {
				slog.Debug("closing reference outside the issue repository left out",
					"pull_request", n, "issue", issue.Repository.NameWithOwner+fmt.Sprintf("#%d", issue.Number))

				continue
			}

			closing[n] = append(closing[n], issue.Number)
		}
	}

	return nil
}
