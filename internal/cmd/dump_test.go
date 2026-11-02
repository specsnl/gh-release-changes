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

// fakeDetails is a DetailsSource that answers from memory, with only what it
// was asked for, and records every request.
type fakeDetails struct {
	details changes.Details
	err     error

	repo     string
	requests []detailsRequest
}

type detailsRequest struct {
	issueRepo            string
	issues, pullRequests []int
}

func (f *fakeDetails) Details(_ context.Context, issueRepo string, issues, pullRequests []int) (changes.Details, error) {
	f.requests = append(f.requests, detailsRequest{issueRepo, issues, pullRequests})

	if f.err != nil {
		return changes.Details{}, f.err
	}

	return changes.Details{
		Issues:                 only(f.details.Issues, issues),
		PullRequests:           only(f.details.PullRequests, pullRequests),
		UnreadableIssues:       only(f.details.UnreadableIssues, issues),
		UnreadablePullRequests: only(f.details.UnreadablePullRequests, pullRequests),
	}, nil
}

func only[V any](m map[int]V, numbers []int) map[int]V {
	out := map[int]V{}

	for _, n := range numbers {
		if v, ok := m[n]; ok {
			out[n] = v
		}
	}

	return out
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
		"warning: pull request #45 could not be read and is noted in the dump: Could not resolve to a PullRequest with the number of 45\n" +
		"warning: pull request #47 could not be read and is noted in the dump: Could not resolve to a PullRequest with the number of 47\n"
	if stderr != wantStderr {
		t.Errorf("stderr =\n%s\nwant\n%s", stderr, wantStderr)
	}

	if details.repo != "example-org/app" {
		t.Errorf("details source repo = %q, want %q", details.repo, "example-org/app")
	}

	// The issues and the features and fixes without one first; then the
	// release's pull requests the issues turned out not to link.
	wantRequests := fmt.Sprint([]detailsRequest{
		{"example-org/app", []int{12, 13, 14, 15, 16}, []int{44, 45}},
		{"example-org/app", nil, []int{41, 42, 47}},
	})
	if got := fmt.Sprint(details.requests); got != wantRequests {
		t.Errorf("requests = %s, want %s", got, wantRequests)
	}
}

// When every issue links the release's pull requests for it, the details are
// one request.
func TestDump_NoUnlinkedPullRequests(t *testing.T) {
	app, _, details := dumpApp(t)

	for _, n := range []int{12, 13, 14, 16} {
		issue := details.details.Issues[n]
		for _, pr := range []int{40, 41, 42, 47} {
			issue.ClosedBy = append(issue.ClosedBy, changes.PullRequest{Number: pr, Repo: "example-org/app"})
		}

		details.details.Issues[n] = issue
	}

	_, stdout, _, _ := runApp(t, app, nil, "-R", "example-org/app", "dump", "v2.1.0")

	if len(details.requests) != 1 {
		t.Errorf("requests = %v, want one", details.requests)
	}

	if strings.Contains(stdout, "#### Unlinked pull requests in this release") {
		t.Errorf("the dump lists unlinked pull requests:\n%s", stdout)
	}
}

// A failure of the second request fails the run like the first.
func TestDump_UnlinkedPullRequestsFailure(t *testing.T) {
	app, _, details := dumpApp(t)
	source := &failSecond{fakeDetails: details}

	app.NewDetailsSource = func(string) (cmd.DetailsSource, error) { return source, nil }

	_, stdout, _, err := runApp(t, app, nil, "dump", "v2.1.0")
	if !errors.Is(err, releasechanges.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}

	if stdout != "" {
		t.Errorf("a failed run produced a product: %q", stdout)
	}
}

// failSecond answers the first request and fails every later one.
type failSecond struct {
	*fakeDetails
}

func (f *failSecond) Details(ctx context.Context, issueRepo string, issues, pullRequests []int) (changes.Details, error) {
	if len(f.requests) > 0 {
		return changes.Details{}, fmt.Errorf("%w: example-org/app", releasechanges.ErrForbidden)
	}

	return f.fakeDetails.Details(ctx, issueRepo, issues, pullRequests)
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

func TestDump_ScopeIssueRepo(t *testing.T) {
	app, _, details := dumpApp(t)

	_, stdout, _, _ := runApp(t, app, nil, "-R", "example-org/app", "dump", "v2.1.0", "--scope-issue-repo", "example-org/tracker")

	if len(details.requests) == 0 || details.requests[0].issueRepo != "example-org/tracker" {
		t.Errorf("requests = %v, want issues from %q", details.requests, "example-org/tracker")
	}

	if !strings.Contains(stdout, "### Issue example-org/tracker#12: ") {
		t.Errorf("the issues are not referenced in full:\n%s", stdout)
	}
}

func TestDump_RejectsAMalformedScopeIssueRepo(t *testing.T) {
	for _, value := range []string{"app", "o/", "github.com/o/app"} {
		t.Run(value, func(t *testing.T) {
			app, notes, details := dumpApp(t)

			_, stdout, _, err := runApp(t, app, nil, "dump", "v2.1.0", "--scope-issue-repo", value)
			if !errors.Is(err, releasechanges.ErrInvalidRepoRef) {
				t.Fatalf("error = %v, want ErrInvalidRepoRef", err)
			}

			if notes.tag != "" || len(details.requests) > 0 {
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

	if len(details.requests) > 0 {
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
