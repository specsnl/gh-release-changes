package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// tagsPage is the most refs GitHub returns in one page.
const tagsPage = 100

const tagsQuery = `query($owner: String!, $name: String!, $first: Int!, $after: String) {
  repository(owner: $owner, name: $name) {
    refs(refPrefix: "refs/tags/", first: $first, after: $after) {
      nodes { name }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

// Tags lists every tag of the repository, in GitHub's order, following the
// pages through GraphQL. Ordering them is the caller's business.
func (c *Client) Tags(ctx context.Context) ([]string, error) {
	tags := []string{}

	var after *string

	for {
		var data struct {
			Repository *struct {
				Refs struct {
					Nodes []struct {
						Name string `json:"name"`
					} `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"refs"`
			} `json:"repository"`
		}

		vars := map[string]any{"owner": c.repo.Owner, "name": c.repo.Name, "first": tagsPage, "after": after}

		if err := c.gql.DoWithContext(ctx, tagsQuery, vars, &data); err != nil {
			return nil, c.mapTagsError(err)
		}

		if data.Repository == nil {
			return nil, fmt.Errorf("%w: %s", releasechanges.ErrRepoNotFound, c.Repo())
		}

		for _, node := range data.Repository.Refs.Nodes {
			tags = append(tags, node.Name)
		}

		page := data.Repository.Refs.PageInfo
		if !page.HasNextPage {
			break
		}

		after = &page.EndCursor
	}

	slog.Debug("tags listed", "repo", c.Repo(), "tags", len(tags))

	return tags, nil
}

// mapTagsError maps a missing repository — a NOT_FOUND on the repository
// field, as GraphQL reports it — to ErrRepoNotFound, and an HTTP error as the
// REST calls do.
func (c *Client) mapTagsError(err error) error {
	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) && gqlErr.Match("NOT_FOUND", "repository") {
		return fmt.Errorf("%w: %s", releasechanges.ErrRepoNotFound, c.Repo())
	}

	return c.mapError(err, releasechanges.ErrRepoNotFound, c.Repo())
}
