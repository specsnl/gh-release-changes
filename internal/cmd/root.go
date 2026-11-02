package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/output"
)

// Flag names, in one place because both the definition and the PersistentPreRunE
// lookup have to agree on the string.
const (
	flagRepo        = "repo"
	flagPreviousTag = "previous-tag"
	flagSinceFinal  = "since-final"
	flagLink        = "link"
	flagDebug       = "debug"
)

// Execute builds the command tree and runs it with a background context.
func Execute(app *App) error {
	return ExecuteContext(context.Background(), app)
}

// ExecuteContext builds the command tree and runs it with the given context,
// which reaches the handlers as cmd.Context().
func ExecuteContext(ctx context.Context, app *App) error {
	return NewRootCmd(app).ExecuteContext(ctx)
}

// NewRootCmd builds the root command, with every subcommand attached and every
// persistent flag defined.
//
// Exported because a test drives the tree the way main does — build it, point
// SetOut/SetErr at buffers, execute — and that is the only way to assert that
// the output really is captured.
func NewRootCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh release-changes",
		Short: "Sort and extract the changes of a GitHub release",
		Long: `gh release-changes reads the generated notes of a GitHub release and renders
them for different audiences: a grouped "What's Changed" list, an issue list
for the release ticket, an issue dump to write release notes from, and a check
of release notes against the release.`,

		// Usage helps with a bad flag, not with "tag not found"; on a runtime
		// failure it buries the one line that matters.
		SilenceUsage: true,

		// main owns the single error print, through the Writer. Without this Cobra
		// prints its own copy as well.
		SilenceErrors: true,

		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return app.resolveFlags(cmd)
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringP(flagRepo, "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	flags.String(flagPreviousTag, "", "Compare with this tag instead of the one GitHub picks")
	flags.Bool(flagSinceFinal, false, "Compare with the newest final release older than the tag, covering the whole release cycle")
	flags.Var(&linkSources{}, flagLink, "Also link issues from these sources: closing (the issues a pull request closes on GitHub)")
	flags.Bool(flagDebug, false, "Write debug diagnostics to stderr")
	cmd.MarkFlagsMutuallyExclusive(flagPreviousTag, flagSinceFinal)

	cmd.AddCommand(newItemsCmd(app))
	cmd.AddCommand(newSortCmd(app))
	cmd.AddCommand(newIssuesCmd(app))
	cmd.AddCommand(newDumpCmd(app))
	cmd.AddCommand(newCheckNotesCmd(app))
	cmd.AddCommand(newVersionCmd(app))
	cmd.AddCommand(newNotesCmd(app))

	return cmd
}

// resolveFlags copies the parsed persistent flags onto the App and rebuilds the
// streams and the logger over the command's own.
//
// The streams come from cmd.OutOrStdout / cmd.ErrOrStderr rather than os.Stdout
// and os.Stderr, which are how output silently escapes a test's buffers. In
// production the accessors resolve to exactly the same files.
//
// The writer is installed before the flag values are validated, so a rejected
// -R is still reported over the command's own stderr.
func (a *App) resolveFlags(cmd *cobra.Command) error {
	flags := cmd.Flags()

	// Lookup errors are unreachable: every name here is defined in NewRootCmd as
	// a flag of that type.
	a.Repo, _ = flags.GetString(flagRepo)
	a.PreviousTag, _ = flags.GetString(flagPreviousTag)
	a.SinceFinal, _ = flags.GetBool(flagSinceFinal)
	a.Debug, _ = flags.GetBool(flagDebug)

	if link, ok := flags.Lookup(flagLink).Value.(*linkSources); ok {
		a.Link = link.list()
	}

	a.Stdout = cmd.OutOrStdout()
	a.Stdin = cmd.InOrStdin()
	a.Stderr = cmd.ErrOrStderr()
	a.Out = output.NewWriter(cmd.ErrOrStderr())
	a.LogLevel = output.SetupLogger(cmd.ErrOrStderr(), a.Debug)

	a.warnRenamedFlags(cmd)

	if a.Repo != "" && !isRepoRef(a.Repo) {
		return fmt.Errorf("%w: -R %q", releasechanges.ErrInvalidRepoRef, a.Repo)
	}

	slog.Debug("flags resolved", "repo", a.Repo, "previous_tag", a.PreviousTag, "since_final", a.SinceFinal, "link", a.Link)

	return nil
}

// isRepoRef reports whether ref has the shape gh accepts for -R: OWNER/REPO or
// HOST/OWNER/REPO, with no empty part. Whether the repository exists is a
// question for GitHub.
func isRepoRef(ref string) bool {
	parts := strings.Split(ref, "/")
	if len(parts) != 2 && len(parts) != 3 {
		return false
	}

	for _, part := range parts {
		if part == "" || strings.ContainsAny(part, " \t") {
			return false
		}
	}

	return true
}
