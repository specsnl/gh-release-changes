package cmd

import (
	"context"
	"strconv"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

// loadRelease is the extraction step every rendering command shares: fetch the
// generated notes for tag, parse them, and warn about unclassified items on
// stderr. Unclassified items are shown, never dropped, and do not change the
// exit code; enforcing the title format is the PR-title check's job.
func (a *App) loadRelease(ctx context.Context, tag string) (changes.Release, error) {
	source, err := a.NewNotesSource(a.Repo)
	if err != nil {
		return changes.Release{}, err
	}

	notes, err := source.GenerateNotes(ctx, tag, a.PreviousTag)
	if err != nil {
		return changes.Release{}, err
	}

	release := changes.Release{
		Repo:        source.Repo(),
		Tag:         tag,
		PreviousTag: notes.PreviousTag,
		Items:       changes.Parse(notes.Body),
	}

	a.warnUnclassified(release.Items)

	return release, nil
}

func (a *App) warnUnclassified(items []changes.Item) {
	var prs []string

	for _, item := range items {
		if item.Group == changes.GroupUnclassified {
			prs = append(prs, "#"+strconv.Itoa(item.PR))
		}
	}

	if len(prs) == 0 {
		return
	}

	noun := "pull requests have titles"
	if len(prs) == 1 {
		noun = "pull request has a title"
	}

	a.Out.Warn("%d %s outside the <type>[(<scopes>)][!]: <subject> format, listed as unclassified: %s",
		len(prs), noun, strings.Join(prs, ", "))
}
