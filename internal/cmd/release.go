package cmd

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/semver"
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

	previousTag, err := a.previousTag(ctx, tag)
	if err != nil {
		return changes.Release{}, err
	}

	notes, err := source.GenerateNotes(ctx, tag, previousTag)
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

// previousTag is the base to generate the notes against: --previous-tag, the
// previous final release with --since-final, or "" to let GitHub choose. When
// --since-final finds no older final release, GitHub chooses after all, and an
// info line says so.
func (a *App) previousTag(ctx context.Context, tag string) (string, error) {
	if !a.SinceFinal {
		return a.PreviousTag, nil
	}

	source, err := a.NewTagSource(a.Repo)
	if err != nil {
		return "", err
	}

	tags, err := source.Tags(ctx)
	if err != nil {
		return "", err
	}

	if final, ok := semver.PreviousFinal(tags, tag); ok {
		slog.Debug("previous final release", "tag", tag, "previous_tag", final)

		return final, nil
	}

	if _, ok := semver.Parse(tag); !ok {
		a.Out.Info("%s is not a semantic version, so --since-final has no final release to compare with; using the tag GitHub picks", tag)
	} else {
		a.Out.Info("no final release is older than %s; using the tag GitHub picks", tag)
	}

	return "", nil
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
