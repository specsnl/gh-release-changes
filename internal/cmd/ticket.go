package cmd

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/checkout"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/render"
	"github.com/specsnl/gh-release-changes/internal/semver"
	"github.com/specsnl/gh-release-changes/internal/util/output"
)

// templatePath is where a repository keeps its ticket template.
var templatePath = path.Join(checkout.TemplateDir, checkout.TemplateFile)

const ticketLong = `Create or update the release ticket: an issue, rendered from a Go template,
that lists the issues of a release. Run it for every release candidate: the
ticket is found again by its labels and the version in its title (3.9.0 for
v3.9.0-rc.2), and an update rewrites only the part between the
<!-- gh-release-changes:start --> and <!-- gh-release-changes:end --> markers,
so ticked boxes and answers written in the ticket survive. A template without
the markers is generated as a whole.

The range starts at the previous final release, as with --since-final, unless
--previous-tag or --since-final=false is given. The issue list includes the
pull requests without an issue, as with issues --unlinked.

The template is ` + "`.github/release-changes/ticket.md.tmpl`" + `; the first one found wins:

  1. --template <path>, a local file
  2. the root of the repository the current directory is in, uncommitted edits
     included (not with -R, whose repository may be another one)
  3. the default branch of the source repository, through the API
  4. the built-in default

Every *.tmpl file next to it is a part it can include:
{{ template "checklist.md.tmpl" . }}, or {{ include "checklist.md.tmpl" . }},
which returns a string to pipe on. YAML front matter between --- lines sets
the default title (itself a template) and labels:

  ---
  title: "Release {{ .Version }}"
  labels: [release]
  ---

Template data:

  .Repo         the source repository, OWNER/REPO
  .Tag          the tag
  .Version      the tag without a leading v
  .PreviousTag  the base of the comparison
  .Issues       the issue list, as the issues command prints it
  .Items        the pull requests, as in the items JSON
  .Vars         the --var key=value pairs

A missing .Vars key fails the run; read an optional one with
{{ index .Vars "cab" | default "n/a" }}. The functions are Sprout's, without
env and filesystem: https://docs.atom.codes/sprout/registries/list-of-all-registries

On a terminal the ticket is shown and confirmed before it is written; --yes
writes without asking, and --dry-run prints the title, labels and body and
writes nothing. The issue's URL is printed on stdout.

` + "`gh release-changes ticket init`" + ` writes a starter template. A tag named init is
passed as ` + "`ticket -- init`" + `.`

// newTicketCmd creates or updates the release ticket.
func newTicketCmd(app *App) *cobra.Command {
	var (
		scopeIssueRepo, relativeTo, createIn string
		fullRefs, titles                     bool
		list                                 = render.IssueList{Unlinked: true}
		templateFile, title                  string
		labels                               []string
		vars                                 = ticketVars{}
		dryRun, yes                          bool
	)

	cmd := &cobra.Command{
		Use:   "ticket [<tag>]",
		Short: "Create or update the release ticket, an issue rendered from a template",
		Long:  ticketLong,
		Args:  tagArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkIssueRepoFlags(scopeIssueRepo, relativeTo); err != nil {
				return err
			}

			if createIn != "" && !isRepoRef(createIn) {
				return fmt.Errorf("%w: --create-in %q", releasechanges.ErrInvalidRepoRef, createIn)
			}

			tag, err := app.resolveTag(cmd, args)
			if err != nil {
				return err
			}

			tmpl, err := app.ticketTemplate(cmd.Context(), templateFile)
			if err != nil {
				return err
			}

			if !cmd.Flags().Changed(flagPreviousTag) && !cmd.Flags().Changed(flagSinceFinal) {
				app.SinceFinal = true
			}

			release, err := app.loadRelease(cmd.Context(), tag, scopeIssueRepo)
			if err != nil {
				return err
			}

			if relativeTo == "" && createIn != "" {
				relativeTo = ownerRepo(createIn)
			}

			refs := issueRefs(release, scopeIssueRepo, relativeTo)
			refs.Full = fullRefs

			var unreadable map[int]string

			if titles {
				list.Titles, unreadable, err = app.issueTitles(cmd.Context(), release, refs)
				if err != nil {
					return err
				}
			}

			data := render.TicketData{
				Repo:        release.Repo,
				Tag:         release.Tag,
				Version:     strings.TrimPrefix(release.Tag, "v"),
				PreviousTag: release.PreviousTag,
				Issues:      render.Issues(release.Items, refs, list),
				Items:       release.Items,
				Vars:        vars,
			}

			if len(labels) == 0 {
				labels = tmpl.Labels
			}

			if len(labels) == 0 {
				labels = slices.Clone(render.DefaultTicketLabels)
			}

			renderedTitle, body, err := tmpl.Render(cmp.Or(title, tmpl.Title, render.DefaultTicketTitle), data)
			if err != nil {
				return err
			}

			target, err := app.NewTicketTarget(cmp.Or(createIn, app.Repo))
			if err != nil {
				return err
			}

			w := ticketWrite{title: renderedTitle, labels: labels, body: body, target: target}

			if err := app.planTicket(cmd.Context(), &w, semver.Release(tag)); err != nil {
				return err
			}

			if dryRun {
				err = app.printTicket(w)
			} else {
				err = app.writeTicket(cmd.Context(), w, yes)
			}

			if err != nil {
				return err
			}

			return app.warnUnreadableTitles(refs, unreadable)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&createIn, "create-in", "",
		"The [HOST/]OWNER/REPO the ticket is created in (default: the source repository)")
	flags.StringVar(&scopeIssueRepo, flagScopeIssueRepo, "",
		"The OWNER/REPO the digit scopes of the titles point at (default: the source repository)")
	flags.StringVar(&relativeTo, flagRelativeTo, "",
		"Write references relative to this OWNER/REPO: #N for its own issues (default: the --create-in repository)")
	flags.BoolVar(&fullRefs, "full-refs", false, "Write every reference as OWNER/REPO#N")
	flags.BoolVar(&list.Unlinked, "unlinked", true, "Also list the pull requests without an issue, dependency updates aside")
	flags.BoolVar(&list.Tasks, "tasks", false, "Write every line as a task: - [ ] #N")
	flags.BoolVar(&titles, "titles", false, "Follow every issue reference with the issue's title, read from GitHub")
	flags.StringVar(&templateFile, "template", "",
		"Template file (default: "+templatePath+" in the repository root, then on the default branch, then built in)")
	flags.StringVar(&title, "title", "",
		"The title, a template (default: the template's front matter, else \""+render.DefaultTicketTitle+"\")")
	flags.StringSliceVar(&labels, "label", nil,
		"The labels the ticket is created with and found by (default: the template's front matter, else "+
			strings.Join(render.DefaultTicketLabels, ",")+")")
	flags.Var(vars, "var", "A key=value pair for the template, as .Vars.key; repeatable")
	flags.BoolVar(&dryRun, "dry-run", false, "Print the title, labels and body, and write nothing")
	flags.BoolVar(&yes, "yes", false, "Write the ticket without asking")
	cmd.MarkFlagsMutuallyExclusive("dry-run", "yes")

	cmd.AddCommand(newTicketInitCmd(app))

	return cmd
}

// ticketTemplate finds and parses the ticket template, in the order the help
// gives, and says on stderr which one it used.
func (a *App) ticketTemplate(ctx context.Context, file string) (*render.TicketTemplate, error) {
	files, entry, source, err := a.findTicketTemplate(ctx, file)
	if err != nil {
		return nil, err
	}

	a.Out.Info("template: %s", source)

	return render.ParseTicket(files, entry)
}

func (a *App) findTicketTemplate(ctx context.Context, file string) (files map[string]string, entry, source string, err error) {
	if file != "" {
		files, err := checkout.ReadTemplates(file)
		if err != nil {
			return nil, "", "", fmt.Errorf("%w: %w", releasechanges.ErrTemplate, err)
		}

		return files, filepath.Base(file), file + " (--template)", nil
	}

	if a.Repo == "" {
		files, err := localTicketTemplates(ctx)
		if err != nil || files != nil {
			return files, checkout.TemplateFile, templatePath + " (local)", err
		}
	}

	remote, err := a.NewTemplateSource(a.Repo)
	if err != nil {
		return nil, "", "", err
	}

	files, err = remote.TicketTemplates(ctx, checkout.TemplateDir)
	if err != nil {
		return nil, "", "", err
	}

	if _, ok := files[checkout.TemplateFile]; ok {
		return files, checkout.TemplateFile, templatePath + " (" + remote.Repo() + ", default branch)", nil
	}

	return render.DefaultTicketFiles(), checkout.TemplateFile, "built in", nil
}

// localTicketTemplates reads the template set from the root of the repository
// the current directory is in: nil when there is no root or no template.
func localTicketTemplates(ctx context.Context) (map[string]string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	root, err := checkout.Root(ctx, wd)
	if err != nil {
		slog.Debug("no local ticket template", "error", err)

		return nil, nil
	}

	files, err := checkout.ReadTemplates(filepath.Join(root, filepath.FromSlash(templatePath)))
	if errors.Is(err, fs.ErrNotExist) {
		slog.Debug("no local ticket template", "root", root)

		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("%w: %w", releasechanges.ErrTemplate, err)
	}

	return files, nil
}

// ticketWrite is what a run writes: a new issue, or the body of existing with
// its generated part replaced.
type ticketWrite struct {
	title  string
	labels []string
	body   string
	target TicketTarget

	existing *github.Ticket
}

func (w ticketWrite) subject() string {
	if w.existing == nil {
		return "a new issue in " + w.target.Repo()
	}

	return fmt.Sprintf("%s#%d", w.target.Repo(), w.existing.Number)
}

func (w ticketWrite) unchanged() bool {
	return w.existing != nil && w.existing.Title == w.title && w.existing.Body == w.body
}

// planTicket looks for the existing ticket — an issue carrying every label
// whose title names version — and, when there is one, makes w update it.
func (a *App) planTicket(ctx context.Context, w *ticketWrite, version string) error {
	tickets, err := w.target.Tickets(ctx, w.labels)
	if err != nil {
		return err
	}

	var matches []github.Ticket

	for _, t := range tickets {
		if semver.InTitle(t.Title, version) {
			matches = append(matches, t)
		}
	}

	switch len(matches) {
	case 0:
		return nil
	case 1:
	default:
		refs := make([]string, len(matches))
		for i, t := range matches {
			refs[i] = fmt.Sprintf("#%d", t.Number)
		}

		return fmt.Errorf("%w: %s in %s carry the labels %s and %s in their title; close or relabel all but one",
			releasechanges.ErrTicketAmbiguous, strings.Join(refs, ", "), w.target.Repo(), strings.Join(w.labels, ", "), version)
	}

	body, replaced := render.ReplaceGenerated(matches[0].Body, w.body)
	if !replaced {
		a.Out.Warn("%s#%d has no %s … %s block; the generated part is appended",
			w.target.Repo(), matches[0].Number, render.TicketStart, render.TicketEnd)
	}

	w.existing, w.body = &matches[0], body

	return nil
}

// printTicket is --dry-run: the ticket on stdout, what would happen to it on
// stderr.
func (a *App) printTicket(w ticketWrite) error {
	switch {
	case w.unchanged():
		a.Out.Info("dry run: %s is up to date", w.subject())
	case w.existing != nil:
		a.Out.Info("dry run: would update %s", w.subject())
	default:
		a.Out.Info("dry run: would create %s", w.subject())
	}

	_, err := fmt.Fprintf(a.Stdout, "title: %s\nlabels: %s\n\n%s", w.title, strings.Join(w.labels, ", "), w.body)

	return err
}

// writeTicket creates or updates the ticket, after asking unless yes, and
// prints its URL.
func (a *App) writeTicket(ctx context.Context, w ticketWrite, yes bool) error {
	if w.unchanged() {
		a.Out.Info("%s is up to date", w.subject())

		_, err := fmt.Fprintln(a.Stdout, w.existing.URL)

		return err
	}

	verb, done := "Create", "created"
	if w.existing != nil {
		verb, done = "Update", "updated"
	}

	if !yes {
		if !a.canConfirm() {
			return fmt.Errorf("%w: pass --yes to write the ticket without asking, or --dry-run to print it",
				releasechanges.ErrInteractiveRequired)
		}

		if _, err := fmt.Fprintf(a.Stderr, "title: %s\nlabels: %s\n\n%s\n", w.title, strings.Join(w.labels, ", "), w.body); err != nil {
			return err
		}

		confirm := a.Confirm
		if confirm == nil {
			confirm = a.confirm
		}

		ok, err := confirm(ctx, verb+" "+w.subject()+"?")
		if err != nil {
			return err
		}

		if !ok {
			a.Out.Info("nothing written")

			return nil
		}
	}

	var (
		ticket github.Ticket
		err    error
	)

	if w.existing == nil {
		ticket, err = w.target.CreateTicket(ctx, w.title, w.body, w.labels)
	} else {
		ticket, err = w.target.UpdateTicket(ctx, w.existing.Number, w.title, w.body)
	}

	if err != nil {
		return err
	}

	a.Out.Info("%s %s#%d", done, w.target.Repo(), ticket.Number)

	_, err = fmt.Fprintln(a.Stdout, ticket.URL)

	return err
}

// canConfirm reports whether this run can ask, as [App.canPrompt] does for the
// tag picker.
func (a *App) canConfirm() bool {
	return a.Confirm != nil || (output.IsTTY(a.Stdin) && output.IsTTY(a.Stdout))
}

// confirm is the real question: a huh Confirm on stderr, reading stdin.
func (a *App) confirm(ctx context.Context, question string) (bool, error) {
	var ok bool

	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(question).Value(&ok),
	)).WithInput(a.Stdin).WithOutput(a.Stderr)

	if err := form.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return false, nil
		}

		return false, fmt.Errorf("confirm: %w", err)
	}

	return ok, nil
}

// ticketVars is the value of --var: key=value pairs, repeatable; the last
// value of a key wins.
type ticketVars map[string]string

func (v ticketVars) Set(value string) error {
	key, val, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return fmt.Errorf("%q is not key=value", value)
	}

	v[key] = val

	return nil
}

func (v ticketVars) String() string {
	pairs := make([]string, 0, len(v))
	for _, key := range slices.Sorted(maps.Keys(v)) {
		pairs = append(pairs, key+"="+v[key])
	}

	return strings.Join(pairs, ",")
}

func (v ticketVars) Type() string { return "key=value" }

// ownerRepo drops the host from a [HOST/]OWNER/REPO reference.
func ownerRepo(ref string) string {
	parts := strings.Split(ref, "/")

	return strings.Join(parts[len(parts)-2:], "/")
}
