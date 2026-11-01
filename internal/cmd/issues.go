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
		issueRepo, ticketRepo string
		fullRefs              bool
	)

	cmd := &cobra.Command{
		Use:   "issues [<tag>]",
		Short: "Print the issues of a release per group, for the release ticket",
		Args:  tagArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, f := range []struct{ name, value string }{{"--issue-repo", issueRepo}, {"--ticket-repo", ticketRepo}} {
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

			refs := render.IssueRefs{IssueRepo: release.Repo, TicketRepo: release.Repo, Full: fullRefs}
			if issueRepo != "" {
				refs.IssueRepo = issueRepo
			}

			if ticketRepo != "" {
				refs.TicketRepo = ticketRepo
			}

			_, err = io.WriteString(app.Stdout, render.Issues(release.Items, refs))

			return err
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&issueRepo, "issue-repo", "", "The OWNER/REPO digit scopes point at (default: the source repository)")
	flags.StringVar(&ticketRepo, "ticket-repo", "", "The OWNER/REPO the list is pasted into (default: the source repository)")
	flags.BoolVar(&fullRefs, "full-refs", false, "Write every reference as OWNER/REPO#N")

	return cmd
}

// isOwnerRepo reports whether ref is OWNER/REPO: an issue reference has no
// host, so HOST/OWNER/REPO is not accepted here.
func isOwnerRepo(ref string) bool {
	return strings.Count(ref, "/") == 1 && isRepoRef(ref)
}
