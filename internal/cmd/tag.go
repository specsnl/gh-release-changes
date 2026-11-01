package cmd

import (
	"context"
	"errors"
	"fmt"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/semver"
	"github.com/specsnl/gh-release-changes/internal/util/output"
)

// tagArgs is the Args of every command that takes a tag: at most one, because
// without it the tag is picked.
var tagArgs = cobra.MaximumNArgs(1)

// visibleTags is how many tags the picker shows at once; filtering searches
// all of them.
const visibleTags = 10

// errNoTagPicked is returned when the picker is left without a choice.
var errNoTagPicked = errors.New("no tag picked")

// resolveTag returns the tag argument, or lets the user pick one when it is
// missing.
//
// The guard comes first: without a terminal on stdin and stdout a missing tag
// is ErrInteractiveRequired before anything is requested, rather than a prompt
// nobody can answer in a pipeline or an agent's shell. Callers check their own
// flags before calling this, so a bad flag is still reported first.
func (a *App) resolveTag(cmd *cobra.Command, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	if !a.canPrompt() {
		// Not cmd.CommandPath: it names the root by the first word of its Use,
		// which is "gh".
		return "", fmt.Errorf("%w: pass the tag, as in `%s %s <tag>`, or run it in a terminal to pick one",
			releasechanges.ErrInteractiveRequired, cmd.Root().Use, cmd.Name())
	}

	source, err := a.NewTagSource(a.Repo)
	if err != nil {
		return "", err
	}

	tags, err := source.Tags(cmd.Context())
	if err != nil {
		return "", err
	}

	if len(tags) == 0 {
		return "", fmt.Errorf("%w: the repository has no tags to pick from", releasechanges.ErrTagNotFound)
	}

	pick := a.PickTag
	if pick == nil {
		pick = a.selectTag
	}

	return pick(cmd.Context(), semver.Sort(tags))
}

// canPrompt reports whether this run can ask for a tag: stdin and stdout are
// both terminals, or a test supplied a picker, which stands in for both the
// terminal and the answer.
func (a *App) canPrompt() bool {
	return a.PickTag != nil || (output.IsTTY(a.Stdin) && output.IsTTY(a.Stdout))
}

// selectTag is the real picker: a huh Select with type-to-filter, drawn on
// stderr — stdout is for the product — and reading stdin, which canPrompt has
// established is a terminal.
func (a *App) selectTag(ctx context.Context, tags []string) (string, error) {
	var tag string

	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Tag").
			Options(huh.NewOptions(tags...)...).
			Filtering(true).
			// The height counts the title line as well as the options.
			Height(visibleTags + 1).
			Value(&tag),
	)).WithInput(a.Stdin).WithOutput(a.Stderr)

	if err := form.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", errNoTagPicked
		}

		return "", fmt.Errorf("pick a tag: %w", err)
	}

	return tag, nil
}
