package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/render"
)

// newSortCmd prints the grouped "What's Changed" Markdown, for developers and
// ops.
func newSortCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "sort [<tag>]",
		Short: `Print the grouped "What's Changed" Markdown of a release`,
		Args:  tagArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tag, err := app.resolveTag(cmd, args)
			if err != nil {
				return err
			}

			release, err := app.loadRelease(cmd.Context(), tag, "")
			if err != nil {
				return err
			}

			_, err = io.WriteString(app.Stdout, render.Sort(release.Items))

			return err
		},
	}
}
