package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/render"
)

// newIssuesCmd prints the plain issue list per group, for the release ticket.
func newIssuesCmd(app *App) *cobra.Command {
	var (
		scopeIssueRepo, relativeTo string
		fullRefs, unlinked         bool
	)

	cmd := &cobra.Command{
		Use:   "issues [<tag>]",
		Short: "Print the issues of a release per group, for the release ticket",
		Args:  tagArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, f := range []struct{ name, value string }{
				{"--" + flagScopeIssueRepo, scopeIssueRepo},
				{"--" + flagRelativeTo, relativeTo},
			} {
				if f.value != "" && !isOwnerRepo(f.value) {
					return fmt.Errorf("%w: %s %q (an issue reference takes OWNER/REPO, without a host)",
						releasechanges.ErrInvalidRepoRef, f.name, f.value)
				}
			}

			tag, err := app.resolveTag(cmd, args)
			if err != nil {
				return err
			}

			release, err := app.loadRelease(cmd.Context(), tag)
			if err != nil {
				return err
			}

			refs := render.IssueRefs{
				IssueRepo:  release.Repo,
				SourceRepo: release.Repo,
				RelativeTo: release.Repo,
				Full:       fullRefs,
			}
			if scopeIssueRepo != "" {
				refs.IssueRepo = scopeIssueRepo
			}

			if relativeTo != "" {
				refs.RelativeTo = relativeTo
			}

			_, err = io.WriteString(app.Stdout, render.Issues(release.Items, refs, unlinked))

			return err
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&scopeIssueRepo, flagScopeIssueRepo, "",
		"The OWNER/REPO the digit scopes of the titles point at (default: the source repository)")
	flags.StringVar(&relativeTo, flagRelativeTo, "",
		"Write references relative to this OWNER/REPO: #N for its own issues (default: the source repository)")
	flags.BoolVar(&fullRefs, "full-refs", false, "Write every reference as OWNER/REPO#N")
	flags.BoolVar(&unlinked, "unlinked", false, "Also list the pull requests without an issue, dependency updates aside")
	addRenamedFlags(cmd)

	return cmd
}

// isOwnerRepo reports whether ref is OWNER/REPO: an issue reference has no
// host, so HOST/OWNER/REPO is not accepted here.
func isOwnerRepo(ref string) bool {
	return strings.Count(ref, "/") == 1 && isRepoRef(ref)
}
