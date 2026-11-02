package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newItemsCmd prints the parsed items as the items JSON: one object, items in
// display order. The field names are the contract skills and automation build
// on.
func newItemsCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "items [<tag>]",
		Short: "Print the changes of a release as JSON",
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

			out, err := json.MarshalIndent(release, "", "  ")
			if err != nil {
				return fmt.Errorf("encode the items: %w", err)
			}

			_, err = fmt.Fprintf(app.Stdout, "%s\n", out)

			return err
		},
	}
}
