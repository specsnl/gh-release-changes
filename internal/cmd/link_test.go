package cmd_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// fakeClosing is a ClosingSource that answers from memory and records every
// request.
type fakeClosing struct {
	closing map[int][]int
	err     error

	repo     string
	requests []closingRequest
}

type closingRequest struct {
	issueRepo    string
	pullRequests []int
}

func (f *fakeClosing) ClosingIssues(_ context.Context, issueRepo string, pullRequests []int) (map[int][]int, error) {
	f.requests = append(f.requests, closingRequest{issueRepo, pullRequests})

	if f.err != nil {
		return nil, f.err
	}

	return only(f.closing, pullRequests), nil
}

// finalClosing is what the final fixture's pull requests close: #1467 closes
// the issue another title scopes and one no title does, #1511 the issue its
// own title scopes, and the unclassified release PR #1565 one more.
var finalClosing = map[int][]int{1467: {1600, 1514}, 1511: {1510}, 1565: {1601}}

// linkApp returns an App over notes whose ClosingSource is closing.
func linkApp(notes *fakeNotes, closing *fakeClosing) *cmd.App {
	app := appWith(notes)
	app.NewClosingSource = func(repo string) (cmd.ClosingSource, error) {
		closing.repo = repo

		return closing, nil
	}

	return app
}

func TestLink_ItemsGolden(t *testing.T) {
	closing := &fakeClosing{closing: finalClosing}

	_, stdout, _, err := runApp(t, linkApp(fixtureNotes(t, "final"), closing), nil,
		"-R", "example-org/web-components", "--link", "closing", "items", "v2.0.0")
	if err != nil {
		t.Fatalf("items: %v", err)
	}

	assertGolden(t, "items/final-link-closing.golden", stdout)

	if len(closing.requests) != 1 {
		t.Fatalf("made %d requests, want 1", len(closing.requests))
	}

	if got := closing.requests[0]; got.issueRepo != "example-org/web-components" || len(got.pullRequests) != 88 {
		t.Errorf("request = %s with %d pull requests, want the source repository and every pull request",
			got.issueRepo, len(got.pullRequests))
	}

	if closing.repo != "example-org/web-components" {
		t.Errorf("client repo = %q, want the -R repository", closing.repo)
	}
}

// Without --link nothing is fetched and the output is today's.
func TestLink_NotGiven(t *testing.T) {
	app := appWith(fixtureNotes(t, "final"))
	app.NewClosingSource = func(string) (cmd.ClosingSource, error) {
		t.Error("a ClosingSource was created without --link")

		return &fakeClosing{}, nil
	}

	_, stdout, _, err := runApp(t, app, nil, "-R", "example-org/app", "items", "v2.0.0")
	if err != nil {
		t.Fatalf("items: %v", err)
	}

	if strings.Contains(stdout, "issue_sources") {
		t.Error("issue_sources is in the output without --link")
	}
}

func TestLink_Flag(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{nil, nil},
		{[]string{"--link", "closing"}, []string{"closing"}},
		{[]string{"--link", "closing", "--link", " closing"}, []string{"closing"}},
		{[]string{"--link=closing,closing"}, []string{"closing"}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			app, _, _, err := run(t, nil, append(tc.args, "version")...)
			if err != nil {
				t.Fatalf("version: %v", err)
			}

			if !reflect.DeepEqual(app.Link, tc.want) {
				t.Errorf("--link = %v, want %v", app.Link, tc.want)
			}
		})
	}
}

func TestLink_RejectsAnUnknownSource(t *testing.T) {
	for _, value := range []string{"branch", "closing,branch", ""} {
		t.Run(value, func(t *testing.T) {
			_, stdout, _, err := runApp(t, linkApp(fixtureNotes(t, "final"), &fakeClosing{}), nil,
				"--link", value, "items", "v2.0.0")
			if err == nil || !strings.Contains(err.Error(), "unknown source") {
				t.Fatalf("error = %v, want an unknown source", err)
			}

			if code := exit.Of(err); code != exit.Error {
				t.Errorf("exit code = %s, want %s", code, exit.Error)
			}

			if stdout != "" {
				t.Errorf("a rejected flag produced a product: %q", stdout)
			}
		})
	}
}

// The closing references are read in the repository the digit scopes point at,
// so both sources number the same issues.
func TestLink_ScopeIssueRepo(t *testing.T) {
	for _, command := range []string{"issues", "dump", "check-notes"} {
		t.Run(command, func(t *testing.T) {
			closing := &fakeClosing{err: errors.New("stop here")}
			args := []string{"-R", "o/app", "--link", "closing", command, "v2", "--scope-issue-repo", "o/tracker"}

			if command == "check-notes" {
				args = append(args, "--file", renderNotes)
			}

			notes := &fakeNotes{notes: github.Notes{Body: "* fix(12): x by @alice in https://github.com/o/app/pull/3\n"}}

			_, _, _, err := runApp(t, linkApp(notes, closing), nil, args...)
			if err == nil {
				t.Fatal("the closing source's error was lost")
			}

			if len(closing.requests) != 1 || closing.requests[0].issueRepo != "o/tracker" {
				t.Errorf("requests = %+v, want one for o/tracker", closing.requests)
			}
		})
	}
}

// The issue a pull request closes is listed, and that pull request no longer
// counts as one without an issue.
func TestLink_IssuesList(t *testing.T) {
	notes := &fakeNotes{notes: github.Notes{Body: "" +
		"* fix(12): scoped by @alice in https://github.com/o/app/pull/3\n" +
		"* docs: update the changelog by @alice in https://github.com/o/app/pull/4\n" +
		"* chore: tidy up by @alice in https://github.com/o/app/pull/5\n"}}
	closing := &fakeClosing{closing: map[int][]int{3: {12, 13}, 4: {20}}}

	_, stdout, _, err := runApp(t, linkApp(notes, closing), nil, "-R", "o/app", "--link", "closing", "issues", "v2", "--unlinked")
	if err != nil {
		t.Fatalf("issues: %v", err)
	}

	want := "## Issues\n\n### Bug fixes\n\n* #12\n* #13\n\n### Other\n\n* #20\n\n" +
		"### Pull requests without an issue\n\n* #5 chore: tidy up\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// An issue only a closing reference brings is in the release for check-notes.
func TestLink_CheckNotes(t *testing.T) {
	notes := &fakeNotes{notes: github.Notes{Body: "* docs: update the changelog by @alice in https://github.com/o/app/pull/4\n"}}

	for _, tc := range []struct {
		link []string
		code exit.Code
	}{
		{nil, exit.Mismatch},
		{[]string{"--link", "closing"}, exit.OK},
	} {
		t.Run(fmt.Sprint(tc.link), func(t *testing.T) {
			closing := &fakeClosing{closing: map[int][]int{4: {20}}}

			args := append(append([]string{"-R", "o/app"}, tc.link...), "check-notes", "v2", "--file", "-")

			_, _, err := runWithStdin(t, linkApp(notes, closing), "Fixes #20.\n", args...)
			if code := exit.Of(err); code != tc.code {
				t.Errorf("exit code = %s, want %s (error: %v)", code, tc.code, err)
			}
		})
	}
}

// A release without pull requests asks GitHub nothing.
func TestLink_NoPullRequests(t *testing.T) {
	closing := &fakeClosing{}
	notes := &fakeNotes{notes: github.Notes{Body: "**Full Changelog**: https://github.com/o/r/compare/v1...v2"}}

	_, _, _, err := runApp(t, linkApp(notes, closing), nil, "--link", "closing", "items", "v2")
	if err != nil {
		t.Fatalf("items: %v", err)
	}

	if len(closing.requests) != 0 {
		t.Errorf("made %d requests, want none", len(closing.requests))
	}
}

// A failure to read the closing references fails the run: the issues would be
// incomplete.
func TestLink_Failure(t *testing.T) {
	closing := &fakeClosing{err: errors.New("read the closing references of pull request #3: gone")}

	_, stdout, _, err := runApp(t, linkApp(fixtureNotes(t, "final"), closing), nil, "--link", "closing", "sort", "v2")
	if code := exit.Of(err); code != exit.Error {
		t.Errorf("exit code = %s, want %s (error: %v)", code, exit.Error, err)
	}

	if stdout != "" {
		t.Errorf("a failed run produced a product: %q", stdout)
	}
}
