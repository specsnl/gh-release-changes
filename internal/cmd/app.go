// Package cmd is the gh-release-changes command tree.
//
// One file per command, all of them leaves on the root built in [NewRootCmd].
// The wiring that every command inherits — the stdout stream, the diagnostic
// writer, the debug logger, and the persistent flags — is resolved once in the
// root's PersistentPreRunE and reaches the leaves through the [App] they were
// built with.
//
// # No os.Exit here
//
// Commands return errors; main turns them into exit codes with exit.Of. os.Exit
// skips deferred cleanup, so calling it inside a command would leak whatever the
// command had open. A non-zero code that is not a failure travels on an
// *exit.Err with a nil Err field, which main prints nothing for.
package cmd

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/util/output"
)

// NotesSource fetches GitHub's generated release notes for a tag.
// *github.Client is the implementation; a test substitutes a fake so a command
// runs without HTTP.
type NotesSource interface {
	GenerateNotes(ctx context.Context, tag, previousTag string) (github.Notes, error)

	// Repo is the resolved source repository as OWNER/REPO — also when -R was
	// not given and it came from the current directory.
	Repo() string
}

// DetailsSource fetches the issues and pull requests dump describes.
// *github.Client is the implementation; a test substitutes a fake.
type DetailsSource interface {
	// Details reads issues from issueRepo (OWNER/REPO) and pull requests from
	// the source repository. What cannot be read is reported in the result's
	// Unreadable maps, not as an error.
	Details(ctx context.Context, issueRepo string, issues, pullRequests []int) (changes.Details, error)
}

// App holds everything a command needs that is not its own flags: the output
// streams, the handle on the debug log level, and the resolved values of the
// persistent flags.
//
// Every command is built with the App and closes over it, so a test constructs
// one App, points the root at buffers, and drives the tree. The zero-argument
// [NewApp] is what main uses.
type App struct {
	// Stdout is where a command writes its product — the Markdown or the JSON —
	// as-is. Nothing else goes here.
	Stdout io.Writer

	// Out is the channel for everything that is not the product: progress,
	// warnings, and the final error line, all on stderr.
	Out *output.Writer

	// LogLevel gates the debug logger. Cobra parses flags after the tree is
	// built, so the level is held here and raised once --debug is known.
	LogLevel *slog.LevelVar

	// Repo is -R/--repo: the source repository as [HOST/]OWNER/REPO, or "" for
	// the repository of the current directory.
	Repo string

	// PreviousTag is --previous-tag: the base of the comparison, or "" to let
	// GitHub choose it.
	PreviousTag string

	// Debug is --debug: raises the slog level to Debug on stderr.
	Debug bool

	// NewNotesSource creates the NotesSource for a repository reference in the
	// form of Repo. Commands call it once flags are resolved; a test replaces it.
	NewNotesSource func(repo string) (NotesSource, error)

	// NewDetailsSource creates the DetailsSource for a repository reference in
	// the form of Repo; a test replaces it.
	NewDetailsSource func(repo string) (DetailsSource, error)
}

// NewApp creates an App over the process streams with a silent logger.
//
// Stdout, Out, and LogLevel are placeholders for the window before flags are
// parsed — a failure during parsing still has somewhere to go — and all three
// are replaced in PersistentPreRunE by equivalents over the command's own
// streams. A command that reads them is therefore always reading the test's
// buffers under test.
func NewApp() *App {
	return &App{
		Stdout:   os.Stdout,
		Out:      output.NewDefaultWriter(),
		LogLevel: output.SetupDefaultLogger(false),
		NewNotesSource: func(repo string) (NotesSource, error) {
			return github.New(repo)
		},
		NewDetailsSource: func(repo string) (DetailsSource, error) {
			return github.New(repo)
		},
	}
}
