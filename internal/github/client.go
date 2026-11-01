// Package github is the only package that talks to GitHub. It goes through
// go-gh end to end, so authentication, GH_TOKEN, GH_HOST and GitHub Enterprise
// hosts are resolved the way gh resolves them, and a token is never seen here.
//
// Every failure GitHub reports that a user can act on is mapped to a sentinel
// from internal/releasechanges, wrapped with %w. Anything else — a network
// error, a 5xx, a rate limit — is returned as go-gh reported it.
package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// Client is a REST and GraphQL client bound to one repository.
type Client struct {
	rest *api.RESTClient
	gql  *api.GraphQLClient
	repo repository.Repository
}

// New creates a Client for repoRef, in [HOST/]OWNER/REPO form, or for the
// repository of the current directory when repoRef is "".
func New(repoRef string) (*Client, error) {
	return NewWithOptions(repoRef, api.ClientOptions{})
}

// NewWithOptions is [New] with go-gh client options, for a test that points the
// client at an httptest server through opts.Transport. opts.Host is always the
// repository's host.
func NewWithOptions(repoRef string, opts api.ClientOptions) (*Client, error) {
	repo, err := resolveRepo(repoRef)
	if err != nil {
		return nil, err
	}

	opts.Host = repo.Host

	rest, err := api.NewRESTClient(opts)
	if err != nil {
		return nil, fmt.Errorf("create the GitHub client for %s: %w", repo.Host, err)
	}

	gql, err := api.NewGraphQLClient(opts)
	if err != nil {
		return nil, fmt.Errorf("create the GitHub GraphQL client for %s: %w", repo.Host, err)
	}

	return &Client{rest: rest, gql: gql, repo: repo}, nil
}

// Repo returns the repository as OWNER/REPO.
func (c *Client) Repo() string {
	return c.repo.Owner + "/" + c.repo.Name
}

func resolveRepo(repoRef string) (repository.Repository, error) {
	if repoRef == "" {
		repo, err := repository.Current()
		if err != nil {
			return repository.Repository{}, fmt.Errorf("determine the repository of the current directory (pass -R OWNER/REPO): %w", err)
		}

		return repo, nil
	}

	repo, err := repository.Parse(repoRef)
	if err != nil {
		return repository.Repository{}, fmt.Errorf("%w: %s", releasechanges.ErrInvalidRepoRef, repoRef)
	}

	return repo, nil
}

// repoPath is the repos/OWNER/REPO prefix of every endpoint.
func (c *Client) repoPath() string {
	return "repos/" + url.PathEscape(c.repo.Owner) + "/" + url.PathEscape(c.repo.Name)
}

// checkRepo asks GitHub whether the repository is visible, to tell a missing
// repository apart from a missing tag: both are a 404 on the tag lookup.
func (c *Client) checkRepo(ctx context.Context) error {
	var repo struct{}

	err := c.rest.DoWithContext(ctx, http.MethodGet, c.repoPath(), nil, &repo)
	if err != nil {
		return c.mapError(err, releasechanges.ErrRepoNotFound, c.Repo())
	}

	return nil
}

// mapError turns a go-gh HTTP error into a sentinel: a 404 into notFound,
// reported with subject, and a 403 into ErrForbidden. Every other error comes
// back unchanged.
//
// A 403 is also how GitHub reports an exhausted primary rate limit; that one is
// not a permission problem, and is not reported as one.
func (c *Client) mapError(err error, notFound error, subject string) error {
	var httpErr *api.HTTPError
	if !errors.As(err, &httpErr) {
		return err
	}

	slog.Debug("GitHub API error", "status", httpErr.StatusCode, "url", httpErr.RequestURL, "message", httpErr.Message)

	switch httpErr.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", notFound, subject)
	case http.StatusForbidden:
		if httpErr.Headers.Get("X-Ratelimit-Remaining") == "0" {
			return err
		}

		return fmt.Errorf("%w: %s", releasechanges.ErrForbidden, c.Repo())
	default:
		return err
	}
}

// statusOf returns the HTTP status of a go-gh error, or 0 for an error that is
// not an HTTP response.
func statusOf(err error) int {
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}

	return 0
}

// escapePath escapes each /-separated segment of p, keeping the separators: a
// tag such as release/1.0 is a nested ref, not one segment with an encoded
// slash.
func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}

	return strings.Join(segments, "/")
}
