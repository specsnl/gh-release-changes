package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/render"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// newIssuesCmd prints the plain issue list per group, for the release ticket.
//
// With --titles an issue whose title cannot be read keeps its bare reference,
// is warned about on stderr, and the run exits 4 (skipped): the list is still
// worth pasting.
func newIssuesCmd(app *App) *cobra.Command {
	var (
		scopeIssueRepo, relativeTo string
		fullRefs                   bool
		list                       render.IssueList
		titles                     bool
	)

	cmd := &cobra.Command{
		Use:   "issues [<tag>]",
		Short: "Print the issues of a release per group, for the release ticket",
		Args:  tagArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkIssueRepoFlags(scopeIssueRepo, relativeTo); err != nil {
				return err
			}

			tag, err := app.resolveTag(cmd, args)
			if err != nil {
				return err
			}

			release, err := app.loadRelease(cmd.Context(), tag, scopeIssueRepo)
			if err != nil {
				return err
			}

			refs := issueRefs(release, scopeIssueRepo, relativeTo)
			refs.Full = fullRefs

			var unreadable map[int]string

			if issues := changes.ReferencedIssues(release.Items); titles && len(issues) > 0 {
				source, err := app.NewTitleSource(app.Repo)
				if err != nil {
					return err
				}

				fetched, err := source.IssueTitles(cmd.Context(), refs.IssueRepo, issues)
				if err != nil {
					return err
				}

				list.Titles, unreadable = fetched.Titles, fetched.Unreadable
			}

			if _, err := io.WriteString(app.Stdout, render.Issues(release.Items, refs, list)); err != nil {
				return err
			}

			for _, n := range sortedKeys(unreadable) {
				app.Out.Warn("the title of issue %s could not be read and is left out: %s", refs.Ref(n), unreadable[n])
			}

			if len(unreadable) > 0 {
				return &exit.Err{Code: exit.Skipped}
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&scopeIssueRepo, flagScopeIssueRepo, "",
		"The OWNER/REPO the digit scopes of the titles point at (default: the source repository)")
	flags.StringVar(&relativeTo, flagRelativeTo, "",
		"Write references relative to this OWNER/REPO: #N for its own issues (default: the source repository)")
	flags.BoolVar(&fullRefs, "full-refs", false, "Write every reference as OWNER/REPO#N")
	flags.BoolVar(&list.Unlinked, "unlinked", false, "Also list the pull requests without an issue, dependency updates aside")
	flags.BoolVar(&list.Tasks, "tasks", false, "Write every line as a task: - [ ] #N")
	flags.BoolVar(&titles, "titles", false, "Follow every issue reference with the issue's title, read from GitHub")
	addRenamedFlags(cmd)

	return cmd
}

// checkIssueRepoFlags rejects a --scope-issue-repo or --relative-to that is
// not OWNER/REPO; an empty value is a flag that was not given.
func checkIssueRepoFlags(scopeIssueRepo, relativeTo string) error {
	for _, f := range []struct{ name, value string }{
		{"--" + flagScopeIssueRepo, scopeIssueRepo},
		{"--" + flagRelativeTo, relativeTo},
	} {
		if f.value != "" && !isOwnerRepo(f.value) {
			return fmt.Errorf("%w: %s %q (an issue reference takes OWNER/REPO, without a host)",
				releasechanges.ErrInvalidRepoRef, f.name, f.value)
		}
	}

	return nil
}

// issueRefs returns the references of release, with --scope-issue-repo and
// --relative-to defaulting to the source repository.
func issueRefs(release changes.Release, scopeIssueRepo, relativeTo string) render.IssueRefs {
	refs := render.IssueRefs{IssueRepo: release.Repo, SourceRepo: release.Repo, RelativeTo: release.Repo}

	if scopeIssueRepo != "" {
		refs.IssueRepo = scopeIssueRepo
	}

	if relativeTo != "" {
		refs.RelativeTo = relativeTo
	}

	return refs
}

// isOwnerRepo reports whether ref is OWNER/REPO: an issue reference has no
// host, so HOST/OWNER/REPO is not accepted here.
func isOwnerRepo(ref string) bool {
	return strings.Count(ref, "/") == 1 && isRepoRef(ref)
}
