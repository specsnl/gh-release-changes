package cmd

import (
	"github.com/spf13/cobra"
)

// Execute builds the command tree and runs it.
func Execute() error {
	return NewRootCmd().Execute()
}

// NewRootCmd builds the root command with every subcommand attached.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh release-changes",
		Short: "Sort and extract the changes of a GitHub release",
		Long: `gh release-changes reads the generated notes of a GitHub release and renders
them for different audiences: a grouped "What's Changed" list, an issue list
for the release ticket, and an issue dump to write release notes from.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(newVersionCmd())

	return cmd
}
