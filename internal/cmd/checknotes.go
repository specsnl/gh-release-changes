package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/render"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// newCheckNotesCmd checks release notes against the release: every issue of
// the release is referenced, and nothing outside it is.
//
// A difference is the answer to the question asked, not a failure: the report
// is on stdout and the run exits 2 (mismatch).
func newCheckNotesCmd(app *App) *cobra.Command {
	var scopeIssueRepo, relativeTo, file string

	cmd := &cobra.Command{
		Use:   "check-notes [<tag>] --file <path>",
		Short: "Check that release notes reference every issue of a release, and nothing else",
		Args:  tagArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkIssueRepoFlags(scopeIssueRepo, relativeTo); err != nil {
				return err
			}

			tag, err := app.resolveTag(cmd, args)
			if err != nil {
				return err
			}

			notes, err := app.readNotes(file)
			if err != nil {
				return err
			}

			release, err := app.loadRelease(cmd.Context(), tag)
			if err != nil {
				return err
			}

			refs := issueRefs(release, scopeIssueRepo, relativeTo)
			coverage := changes.CheckNotes(release, refs.IssueRepo, changes.NoteRefs(notes, refs.RelativeTo))

			if _, err := io.WriteString(app.Stdout, render.CheckNotes(coverage, refs)); err != nil {
				return err
			}

			if !coverage.Covered() {
				return &exit.Err{Code: exit.Mismatch}
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&file, "file", "", "The release notes to check, or - for stdin")
	flags.StringVar(&scopeIssueRepo, flagScopeIssueRepo, "",
		"The OWNER/REPO the digit scopes of the titles point at (default: the source repository)")
	flags.StringVar(&relativeTo, flagRelativeTo, "",
		"The OWNER/REPO a bare #N in the notes refers to, and the report is written relative to (default: the source repository)")
	// Unreachable error: the flag was defined above.
	_ = cmd.MarkFlagRequired("file")
	addRenamedFlags(cmd)

	return cmd
}

// readNotes reads the file check-notes is given, or stdin for -.
func (a *App) readNotes(file string) (string, error) {
	var (
		notes []byte
		err   error
	)

	if file == "-" {
		notes, err = io.ReadAll(a.Stdin)
	} else {
		notes, err = os.ReadFile(file)
	}

	if err != nil {
		return "", fmt.Errorf("reading the notes: %w", err)
	}

	return string(notes), nil
}
