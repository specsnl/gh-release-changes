// Package releasechanges holds the values every other package depends on: the
// sentinel errors that describe how a run can fail. It has no behaviour of its
// own and imports nothing from the rest of the tree, so any package may import
// it.
//
// # Wrapping rule
//
// Sentinels are never returned bare from a call site that has context to add.
// Always wrap with %w so the caller keeps both a readable message and a
// machine-comparable identity:
//
//	return fmt.Errorf("%w: %s", releasechanges.ErrTagNotFound, tag)
//
// Callers match with errors.Is; KindOf turns the sentinel into its stable kind
// string. Returning a sentinel through %v or a freshly constructed error breaks
// both.
package releasechanges

import "errors"

// Sentinel errors for every way a run can fail. Each one maps to a stable kind
// string in KindOf — see the error table in docs/design.md.
var (
	// ErrRepoNotFound is returned when the repository does not exist, or is not
	// visible to the authenticated user — GitHub answers both with a 404.
	ErrRepoNotFound = errors.New("repository not found")

	// ErrTagNotFound is returned when the tag does not exist in the repository.
	// generate-notes does not fail on an unknown tag — it silently compares the
	// default branch instead — so the tag is checked before the call.
	ErrTagNotFound = errors.New("tag not found")

	// ErrForbidden is returned when GitHub refuses the call with a 403. The call
	// site adds the permission it needs: generate-notes needs Contents: write
	// even though it creates nothing, so a user with read access hits this, and
	// writing the release ticket needs Issues: write.
	ErrForbidden = errors.New("forbidden")

	// ErrInteractiveRequired is returned when a run needs a prompt — the tag
	// picker, without a tag argument — but stdin and stdout are not terminals.
	ErrInteractiveRequired = errors.New("operation requires an interactive terminal")

	// ErrInvalidRepoRef is returned when a repository reference is not in
	// [HOST/]OWNER/REPO form.
	ErrInvalidRepoRef = errors.New("invalid repository reference: want [HOST/]OWNER/REPO")

	// ErrTemplate is returned when the ticket template cannot be read, parsed or
	// rendered. The message names the file.
	ErrTemplate = errors.New("ticket template")

	// ErrTemplateExists is returned when ticket init would overwrite a file
	// without --force.
	ErrTemplateExists = errors.New("template already exists")

	// ErrNoRepoRoot is returned when ticket init finds no repository root above
	// the current directory to write to, and --dir is not given.
	ErrNoRepoRoot = errors.New("no repository root found")

	// ErrTicketAmbiguous is returned when more than one issue matches the
	// release ticket, so it is unclear which one to update.
	ErrTicketAmbiguous = errors.New("more than one issue matches the release ticket")

	// ErrTicketMarkersMissing is returned under --require-markers when the
	// existing ticket has no complete marker pair to replace, where the run would
	// otherwise append the generated part.
	ErrTicketMarkersMissing = errors.New("the release ticket has no generated-part markers")
)

// KindOf returns a stable, machine-readable string identifier for the sentinel
// wrapped in err, or "" when err wraps no known sentinel.
// The returned strings are a public contract — for a future machine-readable
// error output, and for skills that match on them — so they may be added to but
// never renamed.
func KindOf(err error) string {
	switch {
	case errors.Is(err, ErrRepoNotFound):
		return "repo_not_found"
	case errors.Is(err, ErrTagNotFound):
		return "tag_not_found"
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	case errors.Is(err, ErrInteractiveRequired):
		return "interactive_required"
	case errors.Is(err, ErrInvalidRepoRef):
		return "invalid_repo_ref"
	case errors.Is(err, ErrTemplate):
		return "template"
	case errors.Is(err, ErrTemplateExists):
		return "template_exists"
	case errors.Is(err, ErrNoRepoRoot):
		return "no_repo_root"
	case errors.Is(err, ErrTicketAmbiguous):
		return "ticket_ambiguous"
	case errors.Is(err, ErrTicketMarkersMissing):
		return "ticket_markers_missing"
	default:
		return ""
	}
}
