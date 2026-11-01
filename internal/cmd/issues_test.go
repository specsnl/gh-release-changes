package cmd_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// The command prints the renderer's output unchanged, so it is compared with
// the renderer's own golden files; the source repository is the fixture's.
func TestIssues_PrintsTheRenderedList(t *testing.T) {
	const source = "example-org/web-components"

	for _, tc := range []struct {
		name   string
		flags  []string
		golden string
	}{
		{"defaults to the source repo", nil, "final-short"},
		{"same repo given explicitly", []string{"--scope-issue-repo", source, "--relative-to", source}, "final-short"},
		{"relative to another repo", []string{"--relative-to", "example-org/releases"}, "final-full"},
		{"--full-refs", []string{"--full-refs"}, "final-full-refs"},
		{"--unlinked", []string{"--unlinked"}, "final-unlinked"},
		{"--tasks", []string{"--tasks"}, "final-tasks"},
		{"--tasks --unlinked", []string{"--tasks", "--unlinked"}, "final-tasks-unlinked"},
		{"--unlinked, relative to another repo", []string{"--unlinked", "--relative-to", "example-org/releases"}, "final-unlinked-full"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-R", source, "issues", "v2.0.0"}, tc.flags...)

			_, stdout, _, err := runApp(t, appWith(fixtureNotes(t, "final")), nil, args...)
			if err != nil {
				t.Fatalf("issues: %v", err)
			}

			want, err := os.ReadFile(filepath.Join("..", "render", "testdata", "issues", tc.golden+".golden"))
			if err != nil {
				t.Fatalf("reading the renderer's golden file: %v", err)
			}

			if stdout != string(want) {
				t.Errorf("stdout differs from %s:\n%s", tc.golden, stdout)
			}
		})
	}
}

// --scope-issue-repo points the references elsewhere: they become full,
// because they are still written relative to the source repo.
func TestIssues_ScopeIssueRepo(t *testing.T) {
	fake := &fakeNotes{notes: github.Notes{Body: "* fix(12): x by @alice in https://github.com/o/app/pull/3\n"}}

	_, stdout, _, err := runApp(t, appWith(fake), nil, "-R", "o/app", "issues", "v2", "--scope-issue-repo", "o/tracker")
	if err != nil {
		t.Fatalf("issues: %v", err)
	}

	if want := "## Issues\n\n### Bug fixes\n\n* o/tracker#12\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// A pull request stays in the source repository when --scope-issue-repo points the
// issue references elsewhere.
func TestIssues_UnlinkedWithScopeIssueRepo(t *testing.T) {
	fake := &fakeNotes{notes: github.Notes{Body: "* fix(12): x by @alice in https://github.com/o/app/pull/3\n" +
		"* fix: y by @alice in https://github.com/o/app/pull/4\n"}}

	_, stdout, _, err := runApp(t, appWith(fake), nil,
		"-R", "o/app", "issues", "v2", "--scope-issue-repo", "o/tracker", "--unlinked")
	if err != nil {
		t.Fatalf("issues: %v", err)
	}

	want := "## Issues\n\n### Bug fixes\n\n* o/tracker#12\n\n### Pull requests without an issue\n\n* #4 fix: y\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// A malformed repository flag fails before anything is fetched.
func TestIssues_RejectsMalformedRepoFlags(t *testing.T) {
	for _, flag := range []string{"--scope-issue-repo", "--relative-to", "--issue-repo", "--ticket-repo"} {
		for _, value := range []string{"app", "o/", "/app", "github.com/o/app", "o/my app"} {
			t.Run(flag+"="+value, func(t *testing.T) {
				fake := fixtureNotes(t, "final")

				_, stdout, _, err := runApp(t, appWith(fake), nil, "issues", "v2", flag, value)
				if !errors.Is(err, releasechanges.ErrInvalidRepoRef) {
					t.Fatalf("error = %v, want ErrInvalidRepoRef", err)
				}

				if code := exit.Of(err); code != exit.Error {
					t.Errorf("exit code = %s, want %s", code, exit.Error)
				}

				if fake.tag != "" {
					t.Error("the notes were fetched before the flag was checked")
				}

				if stdout != "" {
					t.Errorf("a rejected flag produced a product: %q", stdout)
				}
			})
		}
	}
}

// fakeTitles is a TitleSource that answers from memory and records every
// request.
type fakeTitles struct {
	titles     map[int]string
	unreadable map[int]string
	err        error

	repo     string
	requests []detailsRequest
}

func (f *fakeTitles) IssueTitles(_ context.Context, issueRepo string, issues []int) (changes.IssueTitles, error) {
	f.requests = append(f.requests, detailsRequest{issueRepo: issueRepo, issues: issues})

	if f.err != nil {
		return changes.IssueTitles{}, f.err
	}

	return changes.IssueTitles{Titles: only(f.titles, issues), Unreadable: only(f.unreadable, issues)}, nil
}

// titlesApp returns an App over the final fixture, whose titles come from the
// renderer's final-titles.json; #1514 cannot be read.
func titlesApp(t *testing.T, notes *fakeNotes) (*cmd.App, *fakeTitles) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "render", "testdata", "issues", "final-titles.json"))
	if err != nil {
		t.Fatalf("reading the titles: %v", err)
	}

	titles := &fakeTitles{unreadable: map[int]string{1514: "Could not resolve to an Issue with the number of 1514."}}
	if err := json.Unmarshal(raw, &titles.titles); err != nil {
		t.Fatalf("decoding the titles: %v", err)
	}

	app := appWith(notes)
	app.NewTitleSource = func(repo string) (cmd.TitleSource, error) {
		titles.repo = repo

		return titles, nil
	}

	return app, titles
}

// The titles are read for every referenced issue, in one request, from the
// issue repository; the one that cannot be read keeps its bare reference, is
// warned about, and the run exits 4.
func TestIssues_Titles(t *testing.T) {
	for _, tc := range []struct {
		flags  []string
		golden string
	}{
		{[]string{"--titles"}, "final-titles"},
		{[]string{"--titles", "--tasks"}, "final-tasks-titles"},
		{[]string{"--titles", "--unlinked"}, "final-titles-unlinked"},
		{[]string{"--titles", "--tasks", "--unlinked"}, "final-tasks-titles-unlinked"},
	} {
		t.Run(fmt.Sprint(tc.flags), func(t *testing.T) {
			const source = "example-org/web-components"

			app, titles := titlesApp(t, fixtureNotes(t, "final"))

			args := append([]string{"-R", source, "issues", "v2.0.0"}, tc.flags...)

			_, stdout, stderr, err := runApp(t, app, nil, args...)
			if code := exit.Of(err); code != exit.Skipped {
				t.Fatalf("exit code = %s, want %s (error: %v)", code, exit.Skipped, err)
			}

			want, err := os.ReadFile(filepath.Join("..", "render", "testdata", "issues", tc.golden+".golden"))
			if err != nil {
				t.Fatalf("reading the renderer's golden file: %v", err)
			}

			if stdout != string(want) {
				t.Errorf("stdout differs from %s:\n%s", tc.golden, stdout)
			}

			wantStderr := "warning: 1 pull request has a title outside the <type>[(<scopes>)][!]: <subject> format, listed as unclassified: #1565\n" +
				"warning: the title of issue #1514 could not be read and is left out: Could not resolve to an Issue with the number of 1514.\n"
			if stderr != wantStderr {
				t.Errorf("stderr =\n%s\nwant\n%s", stderr, wantStderr)
			}

			if titles.repo != source {
				t.Errorf("title source repo = %q, want %q", titles.repo, source)
			}

			wantRequests := fmt.Sprint([]detailsRequest{{issueRepo: source, issues: []int{437, 1510, 1514}}})
			if got := fmt.Sprint(titles.requests); got != wantRequests {
				t.Errorf("requests = %s, want %s", got, wantRequests)
			}
		})
	}
}

// The titles are read from --scope-issue-repo; when every one is read the run
// succeeds.
func TestIssues_TitlesFromTheIssueRepo(t *testing.T) {
	fake := &fakeNotes{notes: github.Notes{Body: "* fix(12): x by @alice in https://github.com/o/app/pull/3\n"}}
	app, titles := titlesApp(t, fake)
	titles.titles = map[int]string{12: "Exports are slow"}

	_, stdout, _, err := runApp(t, app, nil, "-R", "o/app", "issues", "v2", "--scope-issue-repo", "o/tracker", "--titles")
	if err != nil {
		t.Fatalf("issues: %v", err)
	}

	if want := "## Issues\n\n### Bug fixes\n\n* o/tracker#12 Exports are slow\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	if len(titles.requests) != 1 || titles.requests[0].issueRepo != "o/tracker" {
		t.Errorf("requests = %v, want one to o/tracker", titles.requests)
	}
}

// Nothing is fetched without --titles, or when no issue is referenced.
func TestIssues_TitlesOnlyFetchedWhenNeeded(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		args []string
	}{
		{"without --titles", "* fix(12): x by @alice in https://github.com/o/app/pull/3\n", nil},
		{"no issues", "* fix: x by @alice in https://github.com/o/app/pull/3\n", []string{"--titles"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, titles := titlesApp(t, &fakeNotes{notes: github.Notes{Body: tc.body}})

			args := append([]string{"-R", "o/app", "issues", "v2"}, tc.args...)
			if _, _, _, err := runApp(t, app, nil, args...); err != nil {
				t.Fatalf("issues: %v", err)
			}

			if len(titles.requests) != 0 {
				t.Errorf("requests = %v, want none", titles.requests)
			}
		})
	}
}

// A failure of the titles request as a whole fails the run before anything is
// printed.
func TestIssues_TitlesFailure(t *testing.T) {
	app, titles := titlesApp(t, fixtureNotes(t, "final"))
	titles.err = releasechanges.ErrForbidden

	_, stdout, _, err := runApp(t, app, nil, "issues", "v2.0.0", "--titles")
	if !errors.Is(err, releasechanges.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}

	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
}
