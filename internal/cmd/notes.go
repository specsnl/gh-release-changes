package cmd

import (
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// newNotesCmd is a hidden debugging aid: it prints the generated body for a tag
// exactly as GitHub returned it, before the parser sees it. With --debug the
// resolved previous tag is logged on stderr.
func newNotesCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:    "notes <tag>",
		Short:  "Print the raw generated release notes for a tag",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := app.NewNotesSource(app.Repo)
			if err != nil {
				return err
			}

			notes, err := source.GenerateNotes(cmd.Context(), args[0], app.PreviousTag)
			if err != nil {
				return err
			}

			body := notes.Body
			if !strings.HasSuffix(body, "\n") {
				body += "\n"
			}

			_, err = io.WriteString(app.Stdout, body)

			return err
		},
	}
}
