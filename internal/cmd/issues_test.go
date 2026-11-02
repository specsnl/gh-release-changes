package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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
