package cmd_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// fakeDetails is a DetailsSource that answers from memory and records what it
// was asked for.
type fakeDetails struct {
	details changes.Details
	err     error

	called               bool
	repo, issueRepo      string
	issues, pullRequests []int
}

func (f *fakeDetails) Details(_ context.Context, issueRepo string, issues, pullRequests []int) (changes.Details, error) {
	f.called = true
	f.issueRepo, f.issues, f.pullRequests = issueRepo, issues, pullRequests

	return f.details, f.err
}

// dumpApp returns an App over the render package's dump fixture: its release
// as the generated notes, its details as what GitHub answers.
func dumpApp(t *testing.T) (*cmd.App, *fakeNotes, *fakeDetails) {
	t.Helper()

	dir := filepath.Join("..", "render", "testdata", "dump")

	body, err := os.ReadFile(filepath.Join(dir, "release.txt"))
	if err != nil {
		t.Fatalf("reading the fixture release: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "details.json"))
	if err != nil {
		t.Fatalf("reading the fixture details: %v", err)
	}

	details := &fakeDetails{}
	if err := json.Unmarshal(raw, &details.details); err != nil {
		t.Fatalf("decoding the fixture details: %v", err)
	}

	notes := &fakeNotes{notes: github.Notes{Body: string(body), PreviousTag: "v2.0.0"}}

	app := appWith(notes)
	app.NewDetailsSource = func(repo string) (cmd.DetailsSource, error) {
		details.repo = repo

		return details, nil
	}

	return app, notes, details
}

// The command prints the renderer's output unchanged, so it is compared with
// the renderer's own golden file. The fixture has an unreadable issue and pull
// request: both are warned about, and the run exits 4.
func TestDump_PrintsTheRenderedDump(t *testing.T) {
	app, _, details := dumpApp(t)

	_, stdout, stderr, err := runApp(t, app, nil, "-R", "example-org/app", "dump", "v2.1.0")
	if code := exit.Of(err); code != exit.Skipped {
		t.Fatalf("exit code = %s, want %s (error: %v)", code, exit.Skipped, err)
	}

	want, err := os.ReadFile(filepath.Join("..", "render", "testdata", "dump", "release.golden"))
	if err != nil {
		t.Fatalf("reading the renderer's golden file: %v", err)
	}

	if stdout != string(want) {
		t.Errorf("stdout differs from render.Dump's golden output:\n%s", stdout)
	}

	wantStderr := "warning: 1 pull request has a title outside the <type>[(<scopes>)][!]: <subject> format, listed as unclassified: #49\n" +
		"warning: issue #15 could not be read and is noted in the dump: Could not resolve to an Issue with the number of 15.\n" +
		"warning: pull request #45 could not be read and is noted in the dump: Could not resolve to a PullRequest with the number of 45\n"
	if stderr != wantStderr {
		t.Errorf("stderr =\n%s\nwant\n%s", stderr, wantStderr)
	}

	for _, tc := range []struct {
		name      string
		got, want any
	}{
		{"repo", details.repo, "example-org/app"},
		{"issue repo", details.issueRepo, "example-org/app"},
		{"issues", details.issues, []int{12, 13, 14, 15, 16}},
		{"pull requests", details.pullRequests, []int{44, 45}},
	} {
		if fmt.Sprint(tc.got) != fmt.Sprint(tc.want) {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestDump_EverythingReadable(t *testing.T) {
	app, _, details := dumpApp(t)
	details.details.UnreadableIssues = nil
	details.details.UnreadablePullRequests = nil

	_, _, stderr, err := runApp(t, app, nil, "dump", "v2.1.0")
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	if strings.Contains(stderr, "could not be read") {
		t.Errorf("stderr warns about an unreadable issue: %q", stderr)
	}
}

func TestDump_IssueRepo(t *testing.T) {
	app, _, details := dumpApp(t)

	_, stdout, _, _ := runApp(t, app, nil, "-R", "example-org/app", "dump", "v2.1.0", "--issue-repo", "example-org/tracker")

	if details.issueRepo != "example-org/tracker" {
		t.Errorf("issue repo = %q, want %q", details.issueRepo, "example-org/tracker")
	}

	if !strings.Contains(stdout, "### Issue example-org/tracker#12: ") {
		t.Errorf("the issues are not referenced in full:\n%s", stdout)
	}
}

func TestDump_RejectsAMalformedIssueRepo(t *testing.T) {
	for _, value := range []string{"app", "o/", "github.com/o/app"} {
		t.Run(value, func(t *testing.T) {
			app, notes, details := dumpApp(t)

			_, stdout, _, err := runApp(t, app, nil, "dump", "v2.1.0", "--issue-repo", value)
			if !errors.Is(err, releasechanges.ErrInvalidRepoRef) {
				t.Fatalf("error = %v, want ErrInvalidRepoRef", err)
			}

			if notes.tag != "" || details.called {
				t.Error("something was fetched before the flag was checked")
			}

			if stdout != "" {
				t.Errorf("a rejected flag produced a product: %q", stdout)
			}
		})
	}
}

// A failure of the details request as a whole fails the run, with nothing on
// stdout: a dump without its issues is not worth writing from.
func TestDump_DetailsFailure(t *testing.T) {
	app, _, details := dumpApp(t)
	details.err = fmt.Errorf("%w: example-org/app", releasechanges.ErrForbidden)

	_, stdout, _, err := runApp(t, app, nil, "dump", "v2.1.0")
	if !errors.Is(err, releasechanges.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}

	if code := exit.Of(err); code != exit.Error {
		t.Errorf("exit code = %s, want %s", code, exit.Error)
	}

	if stdout != "" {
		t.Errorf("a failed run produced a product: %q", stdout)
	}
}

func TestDump_NotesFailure(t *testing.T) {
	app, notes, details := dumpApp(t)
	notes.err = fmt.Errorf("%w: v9.9.9", releasechanges.ErrTagNotFound)

	_, _, _, err := runApp(t, app, nil, "dump", "v9.9.9")
	if !errors.Is(err, releasechanges.ErrTagNotFound) {
		t.Fatalf("error = %v, want ErrTagNotFound", err)
	}

	if details.called {
		t.Error("details were fetched for a tag that does not exist")
	}
}

// The details source is created for the same -R as the notes, so issues and
// pull requests come from the same host.
func TestDump_UsesTheSourceRepo(t *testing.T) {
	app, _, details := dumpApp(t)

	_, _, _, _ = runApp(t, app, nil, "-R", "github.example.com/example-org/app", "dump", "v2.1.0")

	if details.repo != "github.example.com/example-org/app" {
		t.Errorf("details source repo = %q, want the -R value", details.repo)
	}
}
