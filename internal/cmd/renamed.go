package cmd

import (
	"github.com/spf13/cobra"
)

const (
	flagScopeIssueRepo = "scope-issue-repo"
	flagRelativeTo     = "relative-to"
)

// renamedFlags lists the deprecated flag names and the names that replaced
// them. The old names shipped in a release, so they keep working, hidden and
// with a warning, until they are removed.
var renamedFlags = []struct{ old, current string }{
	{"issue-repo", flagScopeIssueRepo},
	{"ticket-repo", flagRelativeTo},
}

// addRenamedFlags defines, on cmd, a hidden alias for every renamed flag whose
// new name cmd has. Both names write the same value, so a command reads only
// the new one; giving both is an error.
func addRenamedFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	for _, f := range renamedFlags {
		current := flags.Lookup(f.current)
		if current == nil {
			continue
		}

		flags.Var(current.Value, f.old, "Deprecated: use --"+f.current)
		// Unreachable error: the flag was defined on the line above.
		_ = flags.MarkHidden(f.old)

		cmd.MarkFlagsMutuallyExclusive(f.current, f.old)
	}
}

// warnRenamedFlags warns about every deprecated flag name the run was given.
func (a *App) warnRenamedFlags(cmd *cobra.Command) {
	for _, f := range renamedFlags {
		if old := cmd.Flags().Lookup(f.old); old != nil && old.Changed {
			a.Out.Warn("--%s is deprecated, use --%s", f.old, f.current)
		}
	}
}
