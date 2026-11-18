package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// The permissions a 403 on the ticket's calls is reported as needing.
const (
	needsContentsRead = "reading the ticket template needs Contents: read access to the repository"
	needsIssuesWrite  = "writing the release ticket needs Issues: write access to the repository"
)

// ticketsPage is the most issues GitHub returns in one page.
const ticketsPage = 100

// Ticket is an issue of the repository the release ticket is written to.
type Ticket struct {
	Number int
	Title  string
	Body   string
	URL    string
}

type issueResponse struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PullRequest *struct{} `json:"pull_request"`
}

func (r issueResponse) ticket() Ticket {
	return Ticket{Number: r.Number, Title: r.Title, Body: r.Body, URL: r.HTMLURL}
}

// TicketTemplates reads every *.tmpl file of dir (not recursively) from the
// default branch, keyed by file name: one call for the listing, then one per
// file. A dir that does not exist gives nil and no error.
func (c *Client) TicketTemplates(ctx context.Context, dir string) (map[string]string, error) {
	var entries []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}

	err := c.rest.DoWithContext(ctx, http.MethodGet, c.repoPath()+"/contents/"+escapePath(dir), nil, &entries)
	if statusOf(err) == http.StatusNotFound {
		slog.Debug("no ticket templates on the default branch", "repo", c.Repo(), "dir", dir)

		return nil, nil
	}

	if err != nil {
		return nil, c.mapErrorFor(err, releasechanges.ErrRepoNotFound, c.Repo(), needsContentsRead)
	}

	files := map[string]string{}

	for _, entry := range entries {
		if entry.Type != "file" || path.Ext(entry.Name) != ".tmpl" {
			continue
		}

		text, err := c.fileContent(ctx, entry.Path)
		if err != nil {
			return nil, err
		}

		files[entry.Name] = text
	}

	slog.Debug("ticket templates read", "repo", c.Repo(), "dir", dir, "files", len(files))

	return files, nil
}

func (c *Client) fileContent(ctx context.Context, file string) (string, error) {
	var content struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}

	err := c.rest.DoWithContext(ctx, http.MethodGet, c.repoPath()+"/contents/"+escapePath(file), nil, &content)
	if err != nil {
		return "", c.mapErrorFor(err, releasechanges.ErrTemplate, file, needsContentsRead)
	}

	// GitHub sends a file over 1 MB without its content, as encoding "none".
	if content.Encoding != "base64" {
		return "", fmt.Errorf("%w: %s on the default branch of %s is too large to read", releasechanges.ErrTemplate, file, c.Repo())
	}

	text, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return "", fmt.Errorf("%w: %s on the default branch of %s: %w", releasechanges.ErrTemplate, file, c.Repo(), err)
	}

	return string(text), nil
}

// Tickets lists the issues, open and closed, that carry every one of labels.
// Pull requests are left out.
func (c *Client) Tickets(ctx context.Context, labels []string) ([]Ticket, error) {
	query := url.Values{
		"labels":   {strings.Join(labels, ",")},
		"state":    {"all"},
		"per_page": {strconv.Itoa(ticketsPage)},
	}

	var tickets []Ticket

	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))

		var issues []issueResponse

		err := c.rest.DoWithContext(ctx, http.MethodGet, c.repoPath()+"/issues?"+query.Encode(), nil, &issues)
		if err != nil {
			return nil, c.mapErrorFor(err, releasechanges.ErrRepoNotFound, c.Repo(), needsIssuesWrite)
		}

		for _, issue := range issues {
			if issue.PullRequest == nil {
				tickets = append(tickets, issue.ticket())
			}
		}

		if len(issues) < ticketsPage {
			break
		}
	}

	slog.Debug("tickets listed", "repo", c.Repo(), "labels", labels, "tickets", len(tickets))

	return tickets, nil
}

// CreateTicket opens an issue.
func (c *Client) CreateTicket(ctx context.Context, title, body string, labels []string) (Ticket, error) {
	request := struct {
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}{title, body, labels}

	return c.writeTicket(ctx, http.MethodPost, c.repoPath()+"/issues", request)
}

// UpdateTicket rewrites the title and body of issue number. Its labels are
// left alone: it was found by them.
func (c *Client) UpdateTicket(ctx context.Context, number int, title, body string) (Ticket, error) {
	request := struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}{title, body}

	return c.writeTicket(ctx, http.MethodPatch, c.repoPath()+"/issues/"+strconv.Itoa(number), request)
}

func (c *Client) writeTicket(ctx context.Context, method, endpoint string, request any) (Ticket, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return Ticket{}, fmt.Errorf("encode the issue: %w", err)
	}

	var response issueResponse

	if err := c.rest.DoWithContext(ctx, method, endpoint, bytes.NewReader(payload), &response); err != nil {
		return Ticket{}, c.mapErrorFor(err, releasechanges.ErrRepoNotFound, c.Repo(), needsIssuesWrite)
	}

	slog.Debug("ticket written", "repo", c.Repo(), "method", method, "number", response.Number)

	return response.ticket(), nil
}
