package cmd_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// fakeNotes is a NotesSource that answers from memory and records what it was
// asked for.
type fakeNotes struct {
	notes github.Notes
	err   error

	repo, tag, previousTag string
}

func (f *fakeNotes) GenerateNotes(_ context.Context, tag, previousTag string) (github.Notes, error) {
	f.tag, f.previousTag = tag, previousTag

	return f.notes, f.err
}

func (f *fakeNotes) Repo() string {
	if f.repo == "" {
		return "example-org/current"
	}

	return f.repo
}

// appWith returns an App whose NotesSource is fake.
func appWith(fake *fakeNotes) *cmd.App {
	app := cmd.NewApp()
	app.NewNotesSource = func(repo string) (cmd.NotesSource, error) {
		fake.repo = repo

		return fake, nil
	}

	return app
}

func TestNotes_PrintsTheRawBody(t *testing.T) {
	fake := &fakeNotes{notes: github.Notes{Body: "## What's Changed\n* a change", PreviousTag: "v1.1.0"}}

	_, stdout, stderr, err := runApp(t, appWith(fake), nil,
		"-R", "example-org/app", "--previous-tag", "v1.0.0", "notes", "v1.2.0")
	if err != nil {
		t.Fatalf("notes: %v", err)
	}

	if want := "## What's Changed\n* a change\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	if stderr != "" {
		t.Errorf("stderr = %q, want nothing without --debug", stderr)
	}

	for _, tc := range []struct{ name, got, want string }{
		{"repo", fake.repo, "example-org/app"},
		{"tag", fake.tag, "v1.2.0"},
		{"previous tag", fake.previousTag, "v1.0.0"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// A failure keeps its sentinel on the way out, and leaves no product behind.
func TestNotes_Failure(t *testing.T) {
	fake := &fakeNotes{err: fmt.Errorf("%w: v9.9.9", releasechanges.ErrTagNotFound)}

	_, stdout, _, err := runApp(t, appWith(fake), nil, "notes", "v9.9.9")
	if !errors.Is(err, releasechanges.ErrTagNotFound) {
		t.Fatalf("error = %v, want ErrTagNotFound", err)
	}

	if code := exit.Of(err); code != exit.Error {
		t.Errorf("exit code = %s, want %s", code, exit.Error)
	}

	if stdout != "" {
		t.Errorf("a failed run produced a product: %q", stdout)
	}
}

func TestNotes_ClientFailure(t *testing.T) {
	app := cmd.NewApp()
	app.NewNotesSource = func(string) (cmd.NotesSource, error) {
		return nil, errors.New("no repository here")
	}

	_, _, _, err := runApp(t, app, nil, "notes", "v1.2.0")
	if err == nil || !strings.Contains(err.Error(), "no repository here") {
		t.Errorf("error = %v, want the client's", err)
	}
}

func TestNotes_RequiresATag(t *testing.T) {
	_, _, _, err := runApp(t, appWith(&fakeNotes{}), nil, "notes")
	if code := exit.Of(err); code != exit.Error {
		t.Errorf("exit code = %s, want %s", code, exit.Error)
	}
}

// notes is a debugging aid, not part of the documented command set.
func TestNotes_IsHidden(t *testing.T) {
	_, stdout, _, err := run(t, nil, "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}

	if strings.Contains(stdout, "\n  notes ") {
		t.Errorf("--help lists the hidden notes command:\n%s", stdout)
	}
}
