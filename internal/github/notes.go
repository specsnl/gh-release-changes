package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// Notes is what generate-notes returns for a tag.
type Notes struct {
	// Body is the generated Markdown, as GitHub wrote it.
	Body string

	// PreviousTag is the base of the comparison: --previous-tag when given,
	// otherwise the one GitHub chose, read from the "Full Changelog" link. It is
	// "" when GitHub compared with nothing, as for a first release.
	PreviousTag string
}

// GenerateNotes checks that tag (and previousTag, when given) exists, then asks
// GitHub to generate the release notes for it. previousTag "" lets GitHub
// choose the base.
//
// The tags are checked first because generate-notes does not fail on an
// unknown tag_name: it generates notes up to the default branch instead, which
// would turn a typo into a silently wrong range.
//
// The release body is never read; it is editable, the generated notes are not.
func (c *Client) GenerateNotes(ctx context.Context, tag, previousTag string) (Notes, error) {
	if err := c.checkTag(ctx, tag); err != nil {
		return Notes{}, err
	}

	if previousTag != "" {
		if err := c.checkTag(ctx, previousTag); err != nil {
			return Notes{}, err
		}
	}

	request := struct {
		TagName         string `json:"tag_name"`
		PreviousTagName string `json:"previous_tag_name,omitempty"`
	}{tag, previousTag}

	payload, err := json.Marshal(request)
	if err != nil {
		return Notes{}, fmt.Errorf("encode the generate-notes request: %w", err)
	}

	var response struct {
		Body string `json:"body"`
	}

	err = c.rest.DoWithContext(ctx, http.MethodPost, c.repoPath()+"/releases/generate-notes", bytes.NewReader(payload), &response)
	if err != nil {
		return Notes{}, c.mapError(err, releasechanges.ErrRepoNotFound, c.Repo())
	}

	if previousTag == "" {
		previousTag = previousTagFrom(response.Body, tag)
	}

	slog.Debug("notes generated", "repo", c.Repo(), "tag", tag, "previous_tag", previousTag)

	return Notes{Body: response.Body, PreviousTag: previousTag}, nil
}

// checkTag fails with ErrTagNotFound when tag is not a tag of the repository,
// or with ErrRepoNotFound when the repository itself is missing.
//
// GitHub answers the ref lookup with a 409 for an empty repository, which has
// no tags either.
func (c *Client) checkTag(ctx context.Context, tag string) error {
	var ref struct{}

	err := c.rest.DoWithContext(ctx, http.MethodGet, c.repoPath()+"/git/ref/tags/"+escapePath(tag), nil, &ref)
	if err == nil {
		return nil
	}

	if status := statusOf(err); status != http.StatusNotFound && status != http.StatusConflict {
		return c.mapError(err, releasechanges.ErrTagNotFound, tag)
	}

	if repoErr := c.checkRepo(ctx); repoErr != nil {
		return repoErr
	}

	return fmt.Errorf("%w: %s", releasechanges.ErrTagNotFound, tag)
}

// previousTagFrom reads the base of the comparison from the "Full Changelog"
// line GitHub ends the body with:
//
//	**Full Changelog**: https://github.com/OWNER/REPO/compare/v1.1.0...v1.2.0
//
// A first release links to commits/<tag> instead and has no base, so the
// result is "".
func previousTagFrom(body, tag string) string {
	for line := range strings.Lines(body) {
		_, link, ok := strings.Cut(strings.TrimSpace(line), "**Full Changelog**: ")
		if !ok {
			continue
		}

		_, compare, ok := strings.Cut(link, "/compare/")
		if !ok {
			return ""
		}

		if unescaped, err := url.PathUnescape(compare); err == nil {
			compare = unescaped
		}

		previous, ok := strings.CutSuffix(compare, "..."+tag)
		if !ok {
			return ""
		}

		return previous
	}

	return ""
}
