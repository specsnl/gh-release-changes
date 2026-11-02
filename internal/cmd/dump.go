package cmd

import (
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/render"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// newDumpCmd prints everything the writing skills need in one Markdown
// document: the referenced issues with their comments and linked pull
// requests, and the features and fixes without an issue.
//
// The release's pull requests an issue does not link are only known once the
// issues are read, so they are fetched in a second request, when there are
// any.
//
// An issue or pull request that cannot be read is noted in the dump and warned
// about on stderr, and the run exits 4 (skipped) rather than failing: the rest
// of the dump is still worth writing from.
func newDumpCmd(app *App) *cobra.Command {
	var issueRepo string

	cmd := &cobra.Command{
		Use:   "dump [<tag>]",
		Short: "Print the issues and pull requests of a release in detail, for the writing skills",
		Args:  tagArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if issueRepo != "" && !isOwnerRepo(issueRepo) {
				return fmt.Errorf("%w: --%s %q (an issue reference takes OWNER/REPO, without a host)",
					releasechanges.ErrInvalidRepoRef, flagScopeIssueRepo, issueRepo)
			}

			tag, err := app.resolveTag(cmd, args)
			if err != nil {
				return err
			}

			release, err := app.loadRelease(cmd.Context(), tag)
			if err != nil {
				return err
			}

			if issueRepo == "" {
				issueRepo = release.Repo
			}

			var untracked []int

			for _, item := range release.Items {
				if changes.Untracked(item) {
					untracked = append(untracked, item.PR)
				}
			}

			source, err := app.NewDetailsSource(app.Repo)
			if err != nil {
				return err
			}

			details, err := source.Details(cmd.Context(), issueRepo, changes.ReferencedIssues(release.Items), untracked)
			if err != nil {
				return err
			}

			if unlinked := changes.UnlinkedPullRequests(release, details.Issues); len(unlinked) > 0 {
				more, err := source.Details(cmd.Context(), issueRepo, nil, unlinked)
				if err != nil {
					return err
				}

				details.Merge(more)
			}

			refs := render.IssueRefs{IssueRepo: issueRepo, RelativeTo: release.Repo}

			if _, err := io.WriteString(app.Stdout, render.Dump(release, details, refs)); err != nil {
				return err
			}

			return app.warnUnreadable(details, refs)
		},
	}

	cmd.Flags().StringVar(&issueRepo, flagScopeIssueRepo, "",
		"The OWNER/REPO the digit scopes of the titles point at (default: the source repository)")
	addRenamedFlags(cmd)

	return cmd
}

// warnUnreadable names every issue and pull request the dump could not read,
// and turns them into the skipped outcome.
func (a *App) warnUnreadable(details changes.Details, refs render.IssueRefs) error {
	skipped := false

	for _, n := range sortedKeys(details.UnreadableIssues) {
		skipped = true

		a.Out.Warn("issue %s could not be read and is noted in the dump: %s", refs.Ref(n), details.UnreadableIssues[n])
	}

	for _, n := range sortedKeys(details.UnreadablePullRequests) {
		skipped = true

		a.Out.Warn("pull request #%d could not be read and is noted in the dump: %s", n, details.UnreadablePullRequests[n])
	}

	if skipped {
		return &exit.Err{Code: exit.Skipped}
	}

	return nil
}

func sortedKeys(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return keys
}
