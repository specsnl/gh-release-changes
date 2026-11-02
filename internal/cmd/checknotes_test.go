package cmd_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// renderNotes is the renderer's notes for the final fixture, which leave out
// #1514 and reference two issues outside the release.
var renderNotes = filepath.Join("..", "render", "testdata", "check-notes", "final-notes.md")

// runWithStdin is runApp with stdin reading from in.
func runWithStdin(t *testing.T, app *cmd.App, in string, args ...string) (string, string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	root := cmd.NewRootCmd(app)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(in))
	root.SetArgs(args)

	err := root.Execute()

	return stdout.String(), stderr.String(), err
}

// The command prints the renderer's output unchanged, so it is compared with
// the renderer's own golden files; a difference exits 2.
func TestCheckNotes_PrintsTheReport(t *testing.T) {
	const source = "example-org/web-components"

	for _, tc := range []struct {
		name   string
		flags  []string
		golden string
	}{
		{"defaults to the source repo", nil, "final-short"},
		{"relative to another repo", []string{"--relative-to", "example-org/releases"}, "final-relative-to"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-R", source, "check-notes", "v2.0.0", "--file", renderNotes}, tc.flags...)

			_, stdout, _, err := runApp(t, appWith(fixtureNotes(t, "final")), nil, args...)
			if code := exit.Of(err); code != exit.Mismatch {
				t.Fatalf("exit code = %s, want %s (error: %v)", code, exit.Mismatch, err)
			}

			want, err := os.ReadFile(filepath.Join("..", "render", "testdata", "check-notes", tc.golden+".golden"))
			if err != nil {
				t.Fatalf("reading the renderer's golden file: %v", err)
			}

			if stdout != string(want) {
				t.Errorf("stdout differs from %s:\n%s", tc.golden, stdout)
			}
		})
	}
}

// Notes that cover the release exit 0; - reads them from stdin.
func TestCheckNotes_CoveredFromStdin(t *testing.T) {
	fake := &fakeNotes{notes: github.Notes{Body: "* fix(12): x by @alice in https://github.com/o/app/pull/3\n"}}

	stdout, stderr, err := runWithStdin(t, appWith(fake), "Fixed #12 in #3.\n", "-R", "o/app", "check-notes", "v2", "--file", "-")
	if err != nil {
		t.Fatalf("check-notes: %v", err)
	}

	if want := "## Notes check\n\nThe notes reference every issue of the release, and nothing outside it.\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
}

// --scope-issue-repo moves the issues of the release; --relative-to is what a
// bare #N in the notes refers to.
func TestCheckNotes_IssueRepoFlags(t *testing.T) {
	body := "* fix(12): x by @alice in https://github.com/o/app/pull/3\n"

	for _, tc := range []struct {
		name  string
		notes string
		flags []string
		want  string
		code  exit.Code
	}{
		{
			"issues elsewhere, referenced in full",
			"o/tracker#12 and #3",
			[]string{"--scope-issue-repo", "o/tracker"},
			"## Notes check\n\nThe notes reference every issue of the release, and nothing outside it.\n",
			exit.OK,
		},
		{
			"issues elsewhere, a bare #N is in the source repo",
			"#12",
			[]string{"--scope-issue-repo", "o/tracker"},
			"## Notes check\n\n### Issues missing from the notes\n\n* o/tracker#12\n\n" +
				"### References outside the release\n\n* #12\n",
			exit.Mismatch,
		},
		{
			"relative to the issue repo",
			"#12",
			[]string{"--scope-issue-repo", "o/tracker", "--relative-to", "o/tracker"},
			"## Notes check\n\nThe notes reference every issue of the release, and nothing outside it.\n",
			exit.OK,
		},
		{
			"under the old flag names",
			"#12",
			[]string{"--issue-repo", "o/tracker", "--ticket-repo", "o/tracker"},
			"## Notes check\n\nThe notes reference every issue of the release, and nothing outside it.\n",
			exit.OK,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeNotes{notes: github.Notes{Body: body}}

			args := append([]string{"-R", "o/app", "check-notes", "v2", "--file", "-"}, tc.flags...)

			stdout, _, err := runWithStdin(t, appWith(fake), tc.notes, args...)
			if code := exit.Of(err); code != tc.code {
				t.Fatalf("exit code = %s, want %s (error: %v)", code, tc.code, err)
			}

			if stdout != tc.want {
				t.Errorf("stdout = %q, want %q", stdout, tc.want)
			}
		})
	}
}

// The range flags reach the notes request as for every command.
func TestCheckNotes_PreviousTag(t *testing.T) {
	fake := fixtureNotes(t, "final")

	_, _, _, err := runApp(t, appWith(fake), nil,
		"-R", "example-org/web-components", "--previous-tag", "v1.0.0", "check-notes", "v2.0.0", "--file", renderNotes)
	if code := exit.Of(err); code != exit.Mismatch {
		t.Fatalf("exit code = %s, want %s (error: %v)", code, exit.Mismatch, err)
	}

	if fake.tag != "v2.0.0" || fake.previousTag != "v1.0.0" {
		t.Errorf("requested %q against %q, want v2.0.0 against v1.0.0", fake.tag, fake.previousTag)
	}
}

// Without the notes nothing is fetched, and the run fails.
func TestCheckNotes_NeedsTheNotes(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no --file", nil, `required flag(s) "file" not set`},
		{"a file that does not exist", []string{"--file", filepath.Join(t.TempDir(), "missing.md")}, "reading the notes: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := fixtureNotes(t, "final")

			_, stdout, _, err := runApp(t, appWith(fake), nil, append([]string{"check-notes", "v2"}, tc.args...)...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one containing %q", err, tc.want)
			}

			if code := exit.Of(err); code != exit.Error {
				t.Errorf("exit code = %s, want %s", code, exit.Error)
			}

			if fake.tag != "" {
				t.Error("the notes of the release were fetched")
			}

			if stdout != "" {
				t.Errorf("a failed run produced a product: %q", stdout)
			}
		})
	}
}

// A malformed repository flag fails before anything is read or fetched.
func TestCheckNotes_RejectsMalformedRepoFlags(t *testing.T) {
	for _, flag := range []string{"--scope-issue-repo", "--relative-to"} {
		t.Run(flag, func(t *testing.T) {
			fake := fixtureNotes(t, "final")

			_, _, _, err := runApp(t, appWith(fake), nil, "check-notes", "v2", "--file", renderNotes, flag, "github.com/o/app")
			if !errors.Is(err, releasechanges.ErrInvalidRepoRef) {
				t.Fatalf("error = %v, want ErrInvalidRepoRef", err)
			}

			if fake.tag != "" {
				t.Error("the notes were fetched before the flag was checked")
			}
		})
	}
}

// Without a tag and a terminal the guard fails the run, as for every command.
func TestCheckNotes_InteractiveGuard(t *testing.T) {
	_, _, _, err := runApp(t, appWith(fixtureNotes(t, "final")), nil, "check-notes", "--file", renderNotes)
	if !errors.Is(err, releasechanges.ErrInteractiveRequired) {
		t.Fatalf("error = %v, want ErrInteractiveRequired", err)
	}
}
