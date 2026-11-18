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

const flagDontPrettify = "dont-prettify"

func newVersionCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the gh-release-changes version",
		Long: `Print the gh-release-changes version.

The version is a result, not narration, so it goes to stdout and can be
captured:

  gh release-changes version                  # gh-release-changes version 1.2.3
  gh release-changes version --dont-prettify  # 1.2.3

The root flag "gh release-changes --version" is the same as "--dont-prettify".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			bare, _ := cmd.Flags().GetBool(flagDontPrettify)

			return writeVersion(app, bare)
		},
	}

	cmd.Flags().Bool(flagDontPrettify, false, "Print the bare version string, with nothing around it")

	return cmd
}

// writeVersion writes the version to stdout as the command's product.
//
// bare drops the sentence around the value, for $(gh release-changes version
// --dont-prettify): a shell substitution wants the version, not a phrase
// containing it.
//
// Shared with the root's --version, which is defined to mean exactly
// `version --dont-prettify` — one function so the two cannot drift.
func writeVersion(app *App, bare bool) error {
	format := "gh-release-changes version %s\n"
	if bare {
		format = "%s\n"
	}

	_, err := fmt.Fprintf(app.Stdout, format, Version)

	return err
}
