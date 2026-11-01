package cmd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

var releaseTags = []string{"v3.7.0", "v3.8.0-rc.1", "v3.8.0", "v3.9.0-rc.1", "v3.9.0-rc.2", "latest"}

// sinceFinalApp returns an App over fake notes and the given tags.
func sinceFinalApp(tags *fakeTags) (*cmd.App, *fakeNotes) {
	notes := &fakeNotes{notes: github.Notes{Body: "* fix(12): x by @alice in https://github.com/o/app/pull/3\n"}}

	app := appWith(notes)
	app.NewTagSource = func(string) (cmd.TagSource, error) { return tags, nil }

	return app, notes
}

// Every command that generates notes compares with the previous final release.
func TestSinceFinal_ComparesWithThePreviousFinal(t *testing.T) {
	for _, command := range []string{"items", "sort", "issues", "notes"} {
		t.Run(command, func(t *testing.T) {
			tags := &fakeTags{tags: releaseTags}
			app, notes := sinceFinalApp(tags)

			_, _, stderr, err := runApp(t, app, nil, "-R", "o/app", "--since-final", command, "v3.9.0-rc.2")
			if err != nil {
				t.Fatalf("%s: %v", command, err)
			}

			if notes.previousTag != "v3.8.0" {
				t.Errorf("previous tag = %q, want v3.8.0", notes.previousTag)
			}

			if stderr != "" {
				t.Errorf("stderr = %q, want nothing", stderr)
			}
		})
	}
}

// Without an older final release GitHub chooses, and an info line says so.
func TestSinceFinal_FallsBackToGitHubsChoice(t *testing.T) {
	for _, tc := range []struct {
		name, tag, want string
	}{
		{"no older final", "v3.7.0", "info: no final release is older than v3.7.0; using the tag GitHub picks\n"},
		{"non-semver tag", "latest", "info: latest is not a semantic version, so --since-final has no final release to compare with; using the tag GitHub picks\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, notes := sinceFinalApp(&fakeTags{tags: releaseTags})

			_, _, stderr, err := runApp(t, app, nil, "-R", "o/app", "--since-final", "sort", tc.tag)
			if err != nil {
				t.Fatalf("sort: %v", err)
			}

			if notes.tag != tc.tag || notes.previousTag != "" {
				t.Errorf("generated %q against %q, want %q against GitHub's choice", notes.tag, notes.previousTag, tc.tag)
			}

			if stderr != tc.want {
				t.Errorf("stderr = %q, want %q", stderr, tc.want)
			}
		})
	}
}

// The tags are only listed with --since-final.
func TestSinceFinal_TagsOnlyListedWhenAsked(t *testing.T) {
	tags := &fakeTags{tags: releaseTags}
	app, notes := sinceFinalApp(tags)

	if _, _, _, err := runApp(t, app, nil, "-R", "o/app", "sort", "v3.9.0-rc.2"); err != nil {
		t.Fatalf("sort: %v", err)
	}

	if tags.listed {
		t.Error("the tags were listed without --since-final")
	}

	if notes.previousTag != "" {
		t.Errorf("previous tag = %q, want GitHub's choice", notes.previousTag)
	}
}

func TestSinceFinal_ExcludesPreviousTag(t *testing.T) {
	app, notes := sinceFinalApp(&fakeTags{tags: releaseTags})

	_, _, _, err := runApp(t, app, nil, "--since-final", "--previous-tag", "v3.7.0", "sort", "v3.9.0-rc.2")
	if err == nil || !strings.Contains(err.Error(), "since-final") || !strings.Contains(err.Error(), "previous-tag") {
		t.Fatalf("error = %v, want one naming both flags", err)
	}

	if code := exit.Of(err); code != exit.Error {
		t.Errorf("exit code = %s, want %s", code, exit.Error)
	}

	if notes.tag != "" {
		t.Error("the notes were generated")
	}
}

// A failure to list the tags fails the run before the notes are generated.
func TestSinceFinal_TagsFailure(t *testing.T) {
	app, notes := sinceFinalApp(&fakeTags{err: releasechanges.ErrRepoNotFound})

	_, stdout, _, err := runApp(t, app, nil, "-R", "o/app", "--since-final", "sort", "v3.9.0-rc.2")
	if !errors.Is(err, releasechanges.ErrRepoNotFound) {
		t.Fatalf("error = %v, want ErrRepoNotFound", err)
	}

	if notes.tag != "" || stdout != "" {
		t.Errorf("generated notes for %q and printed %q, want neither", notes.tag, stdout)
	}
}
