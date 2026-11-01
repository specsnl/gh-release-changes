package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is injected at build time by .goreleaser.yml and the Taskfile:
//
//	-ldflags "-X github.com/specsnl/gh-release-changes/internal/cmd.Version=1.2.3"
//
// Renaming or moving it makes every build silently report "dev".
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the gh-release-changes version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "gh-release-changes version %s\n", Version)

			return err
		},
	}
}
