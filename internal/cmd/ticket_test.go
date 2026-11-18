package cmd_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/render"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

const fixtureRepo = "example-org/web-components"

// fakeTemplates is a TemplateSource with the files of the default branch.
type fakeTemplates struct {
	files map[string]string

	asked bool
}

func (f *fakeTemplates) TicketTemplates(context.Context, string) (map[string]string, error) {
	f.asked = true

	return f.files, nil
}

func (f *fakeTemplates) Repo() string { return "example-org/app" }

// fakeTarget is a TicketTarget over issues in memory; it records what it was
// asked and what it wrote.
type fakeTarget struct {
	tickets []github.Ticket

	repo     string
	labels   []string
	created  *github.Ticket
	updated  *github.Ticket
	labelled []string
}

func (f *fakeTarget) Tickets(_ context.Context, labels []string) ([]github.Ticket, error) {
	f.labels = labels

	return f.tickets, nil
}

func (f *fakeTarget) CreateTicket(_ context.Context, title, body string, labels []string) (github.Ticket, error) {
	f.created = &github.Ticket{Number: 99, Title: title, Body: body, URL: "https://github.com/example-org/app/issues/99"}
	f.labelled = labels

	return *f.created, nil
}

func (f *fakeTarget) UpdateTicket(_ context.Context, number int, title, body string) (github.Ticket, error) {
	f.updated = &github.Ticket{Number: number, Title: title, Body: body, URL: "https://github.com/example-org/app/issues/7"}

	return *f.updated, nil
}

func (f *fakeTarget) Repo() string { return f.repo }

// ticketFakes are the boundaries a ticket run crosses.
type ticketFakes struct {
	notes     *fakeNotes
	tags      *fakeTags
	templates *fakeTemplates
	target    *fakeTarget
}

// ticketApp returns an App over the final fixture, run from an empty temp
// directory outside any repository, so no local template is found unless a
// test puts one there.
func ticketApp(t *testing.T) (*cmd.App, *ticketFakes) {
	t.Helper()

	notes := fixtureNotes(t, "final")

	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_WORKSPACE", "")
	t.Chdir(t.TempDir())

	fakes := &ticketFakes{
		notes:     notes,
		tags:      &fakeTags{tags: []string{"v1.9.0", "v2.0.0-rc.1", "v2.0.0"}},
		templates: &fakeTemplates{},
		target:    &fakeTarget{},
	}

	app := appWith(fakes.notes)
	app.NewTagSource = func(string) (cmd.TagSource, error) { return fakes.tags, nil }
	app.NewTemplateSource = func(string) (cmd.TemplateSource, error) { return fakes.templates, nil }
	app.NewTicketTarget = func(repo string) (cmd.TicketTarget, error) {
		fakes.target.repo = repo
		if repo == "" {
			fakes.target.repo = "example-org/current"
		}

		return fakes.target, nil
	}

	return app, fakes
}

// renderGoldens is the renderer's ticket golden files, resolved while the
// tests still run in the package directory: most of them change out of it.
var renderGoldens, _ = filepath.Abs(filepath.Join("..", "render", "testdata", "ticket"))

func readRenderGolden(t *testing.T, name string) string {
	t.Helper()

	want, err := os.ReadFile(filepath.Join(renderGoldens, name+".golden"))
	if err != nil {
		t.Fatalf("reading the renderer's golden file: %v", err)
	}

	return string(want)
}

// A first run creates the ticket from the built-in template, with the range
// since the previous final release, and prints its URL.
func TestTicket_Creates(t *testing.T) {
	app, fakes := ticketApp(t)

	_, stdout, stderr, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--yes")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}

	created := fakes.target.created
	if created == nil {
		t.Fatal("no ticket was created")
	}

	if created.Title != "Release 2.0.0" {
		t.Errorf("title = %q, want %q", created.Title, "Release 2.0.0")
	}

	if created.Body != readRenderGolden(t, "default") {
		t.Errorf("body differs from the renderer's default.golden:\n%s", created.Body)
	}

	if !slices.Equal(fakes.target.labelled, []string{"release"}) {
		t.Errorf("labels = %v, want [release]", fakes.target.labelled)
	}

	if fakes.target.repo != fixtureRepo {
		t.Errorf("ticket repo = %q, want the source repo", fakes.target.repo)
	}

	if fakes.notes.previousTag != "v1.9.0" {
		t.Errorf("previous tag = %q, want v1.9.0: --since-final is the default", fakes.notes.previousTag)
	}

	if stdout != created.URL+"\n" {
		t.Errorf("stdout = %q, want the issue URL", stdout)
	}

	for _, want := range []string{"info: template: built in", "info: created " + fixtureRepo + "#99"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	}
}

// A run for a later tag of the same release updates the ticket of an earlier
// one, rewriting only the generated part.
func TestTicket_UpdatesTheGeneratedPart(t *testing.T) {
	app, fakes := ticketApp(t)
	fakes.target.tickets = []github.Ticket{
		{Number: 6, Title: "Release 1.9.0", Body: "older"},
		{Number: 7, Title: "Release 2.0.0-rc.1", Body: "- [x] QA done\n" + render.TicketStart + "\nold list\n" + render.TicketEnd + "\nCAB: yes\n"},
	}

	_, stdout, stderr, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--yes")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}

	updated := fakes.target.updated
	if updated == nil || updated.Number != 7 {
		t.Fatalf("updated = %+v, want #7", updated)
	}

	if fakes.target.created != nil {
		t.Error("a ticket was created as well")
	}

	want := "- [x] QA done\n" + strings.TrimSuffix(readRenderGolden(t, "default"), "\n") + "\nCAB: yes\n"
	if updated.Body != want {
		t.Errorf("body = %q, want %q", updated.Body, want)
	}

	if updated.Title != "Release 2.0.0" {
		t.Errorf("title = %q, want %q", updated.Title, "Release 2.0.0")
	}

	if !strings.Contains(stderr, "info: updated "+fixtureRepo+"#7") || stdout != updated.URL+"\n" {
		t.Errorf("stdout, stderr = %q, %q, want the update reported", stdout, stderr)
	}
}

func TestTicket_UpToDate(t *testing.T) {
	app, fakes := ticketApp(t)
	fakes.target.tickets = []github.Ticket{{Number: 7, Title: "Release 2.0.0", Body: readRenderGolden(t, "default"), URL: "u"}}

	_, stdout, stderr, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}

	if fakes.target.updated != nil {
		t.Error("an unchanged ticket was written")
	}

	if stdout != "u\n" || !strings.Contains(stderr, "is up to date") {
		t.Errorf("stdout, stderr = %q, %q, want the URL and up to date", stdout, stderr)
	}
}

func TestTicket_Ambiguous(t *testing.T) {
	app, fakes := ticketApp(t)
	fakes.target.tickets = []github.Ticket{{Number: 7, Title: "Release 2.0.0-rc.1"}, {Number: 8, Title: "Release 2.0.0"}}

	_, _, _, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--yes")
	if !errors.Is(err, releasechanges.ErrTicketAmbiguous) {
		t.Fatalf("error = %v, want ErrTicketAmbiguous", err)
	}

	if !strings.Contains(err.Error(), "#7, #8") {
		t.Errorf("error = %v, want it to name both", err)
	}

	if fakes.target.created != nil || fakes.target.updated != nil {
		t.Error("a ticket was written")
	}
}

func TestTicket_DryRun(t *testing.T) {
	app, fakes := ticketApp(t)

	_, stdout, stderr, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--dry-run")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}

	if want := "title: Release 2.0.0\nlabels: release\n\n" + readRenderGolden(t, "default"); stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	if !strings.Contains(stderr, "dry run: would create a new issue in "+fixtureRepo) {
		t.Errorf("stderr = %q, want the dry run reported", stderr)
	}

	if fakes.target.created != nil || fakes.target.updated != nil {
		t.Error("a dry run wrote a ticket")
	}
}

// Without a terminal and without --yes nothing is written.
func TestTicket_NeedsYesWithoutATerminal(t *testing.T) {
	app, fakes := ticketApp(t)

	_, stdout, _, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0")
	if !errors.Is(err, releasechanges.ErrInteractiveRequired) {
		t.Fatalf("error = %v, want ErrInteractiveRequired", err)
	}

	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error = %v, want it to mention --yes", err)
	}

	if fakes.target.created != nil || stdout != "" {
		t.Errorf("a ticket was written: %q", stdout)
	}
}

func TestTicket_Confirm(t *testing.T) {
	for _, answer := range []bool{true, false} {
		t.Run(map[bool]string{true: "yes", false: "no"}[answer], func(t *testing.T) {
			app, fakes := ticketApp(t)

			var asked string

			app.Confirm = func(_ context.Context, question string) (bool, error) {
				asked = question

				return answer, nil
			}

			_, _, stderr, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0")
			if err != nil {
				t.Fatalf("ticket: %v", err)
			}

			if want := "Create a new issue in " + fixtureRepo + "?"; asked != want {
				t.Errorf("question = %q, want %q", asked, want)
			}

			if !strings.Contains(stderr, "title: Release 2.0.0\nlabels: release\n\n"+render.TicketStart) {
				t.Errorf("stderr = %q, want the ticket shown before the question", stderr)
			}

			if written := fakes.target.created != nil; written != answer {
				t.Errorf("written = %v, want %v", written, answer)
			}
		})
	}
}

// --previous-tag, or --since-final=false, replaces the --since-final default.
func TestTicket_Range(t *testing.T) {
	for _, tc := range []struct {
		name   string
		flags  []string
		want   string
		listed bool
	}{
		{"default", nil, "v1.9.0", true},
		{"--previous-tag", []string{"--previous-tag", "v2.0.0-rc.1"}, "v2.0.0-rc.1", false},
		{"--since-final=false", []string{"--since-final=false"}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, fakes := ticketApp(t)

			args := append([]string{"-R", fixtureRepo, "ticket", "v2.0.0", "--dry-run"}, tc.flags...)
			if _, _, _, err := runApp(t, app, nil, args...); err != nil {
				t.Fatalf("ticket: %v", err)
			}

			if fakes.notes.previousTag != tc.want {
				t.Errorf("previous tag = %q, want %q", fakes.notes.previousTag, tc.want)
			}

			if fakes.tags.listed != tc.listed {
				t.Errorf("tags listed = %v, want %v", fakes.tags.listed, tc.listed)
			}
		})
	}
}

// --create-in names the ticket repository, which references are written
// relative to.
func TestTicket_CreateIn(t *testing.T) {
	for _, ref := range []string{"example-org/tickets", "github.example.com/example-org/tickets"} {
		t.Run(ref, func(t *testing.T) {
			app, fakes := ticketApp(t)

			_, stdout, _, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--dry-run", "--create-in", ref)
			if err != nil {
				t.Fatalf("ticket: %v", err)
			}

			if fakes.target.repo != ref {
				t.Errorf("ticket repo = %q, want %q", fakes.target.repo, ref)
			}

			if !strings.Contains(stdout, "* "+fixtureRepo+"#1510\n") {
				t.Errorf("stdout = %q, want full references", stdout)
			}
		})
	}
}

func TestTicket_RejectsMalformedFlags(t *testing.T) {
	for _, tc := range []struct {
		flags []string
		want  error
	}{
		{[]string{"--create-in", "tickets"}, releasechanges.ErrInvalidRepoRef},
		{[]string{"--relative-to", "github.com/o/r"}, releasechanges.ErrInvalidRepoRef},
		{[]string{"--var", "no-value"}, nil},
		{[]string{"--var", "=x"}, nil},
		{[]string{"--dry-run", "--yes"}, nil},
	} {
		t.Run(strings.Join(tc.flags, " "), func(t *testing.T) {
			app, fakes := ticketApp(t)

			_, _, _, err := runApp(t, app, nil, append([]string{"ticket", "v2.0.0"}, tc.flags...)...)
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}

			if fakes.notes.tag != "" {
				t.Error("the notes were fetched before the flags were checked")
			}
		})
	}
}

// --title, --label and --var override the template; the last --var of a key
// wins.
func TestTicket_TitleLabelsVars(t *testing.T) {
	app, fakes := ticketApp(t)
	fakes.templates.files = map[string]string{
		"ticket.md.tmpl": "---\ntitle: from the front matter\nlabels: [front]\n---\n{{ .Vars.env }}\n",
	}

	_, stdout, _, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--dry-run",
		"--title", "WOO {{ .Version }} {{ .Vars.env }}", "--label", "a", "--label", "b",
		"--var", "env=TEST", "--var", "env=PROD")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}

	want := "title: WOO 2.0.0 PROD\nlabels: a, b\n\n" + render.TicketStart + "\nPROD\n" + render.TicketEnd + "\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	if !slices.Equal(fakes.target.labels, []string{"a", "b"}) {
		t.Errorf("found by labels %v, want [a b]", fakes.target.labels)
	}
}

// The front matter sets the title and labels when the flags do not.
func TestTicket_FrontMatterDefaults(t *testing.T) {
	app, fakes := ticketApp(t)
	fakes.templates.files = map[string]string{
		"ticket.md.tmpl": "---\ntitle: \"WOO: release {{ .Version }} naar PROD\"\nlabels: [release, woo]\n---\nbody\n",
	}

	_, stdout, _, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0-rc.1", "--dry-run")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}

	if !strings.HasPrefix(stdout, "title: WOO: release 2.0.0-rc.1 naar PROD\nlabels: release, woo\n\n") {
		t.Errorf("stdout = %q, want the front matter's title and labels", stdout)
	}
}

// A template that does not parse fails before the release is fetched.
func TestTicket_BadTemplateFailsFirst(t *testing.T) {
	app, fakes := ticketApp(t)
	fakes.templates.files = map[string]string{"ticket.md.tmpl": "{{ .Broken "}

	_, _, _, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--yes")
	if !errors.Is(err, releasechanges.ErrTemplate) {
		t.Fatalf("error = %v, want ErrTemplate", err)
	}

	if fakes.notes.tag != "" {
		t.Error("the notes were fetched for a template that does not parse")
	}
}

func TestTicket_MissingVarFails(t *testing.T) {
	app, fakes := ticketApp(t)
	fakes.templates.files = map[string]string{"ticket.md.tmpl": "{{ .Vars.environment }}"}

	_, _, _, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--yes")
	if !errors.Is(err, releasechanges.ErrTemplate) || !strings.Contains(err.Error(), "environment") {
		t.Fatalf("error = %v, want ErrTemplate naming the key", err)
	}

	if fakes.target.created != nil {
		t.Error("a ticket was written")
	}
}

// writeTemplates writes a template set under dir/.github/release-changes.
func writeTemplates(t *testing.T, dir, text string) {
	t.Helper()

	templates := filepath.Join(dir, ".github", "release-changes")
	if err := os.MkdirAll(templates, 0o755); err != nil {
		t.Fatal(err)
	}

	for name, text := range map[string]string{"ticket.md.tmpl": text, "part.md.tmpl": "{{ .Tag }}"} {
		if err := os.WriteFile(filepath.Join(templates, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTicket_TemplateLookup(t *testing.T) {
	for _, tc := range []struct {
		name string

		// setup prepares the current directory (a temp dir), and returns the
		// extra flags.
		setup        func(t *testing.T, wd string) []string
		remote       map[string]string
		source       string
		body         string
		remoteAsked  bool
	}{
		{
			name: "--template",
			setup: func(t *testing.T, wd string) []string {
				writeTemplates(t, wd, "flag {{ template \"part.md.tmpl\" . }}")
				// A local template is there too, and loses.
				return []string{"--template", filepath.Join(".github", "release-changes", "ticket.md.tmpl")}
			},
			source: filepath.Join(".github", "release-changes", "ticket.md.tmpl") + " (--template)",
			body:   "flag v2.0.0",
		},
		{
			name: "local, from a nested directory",
			setup: func(t *testing.T, wd string) []string {
				writeTemplates(t, wd, "local {{ include \"part.md.tmpl\" . }}")
				mustMkdir(t, filepath.Join(wd, ".git"))
				t.Chdir(mustMkdir(t, filepath.Join(wd, "a", "b")))

				return nil
			},
			source: ".github/release-changes/ticket.md.tmpl (local)",
			body:   "local v2.0.0",
		},
		{
			name: "local is not read with -R",
			setup: func(t *testing.T, wd string) []string {
				writeTemplates(t, wd, "local")
				mustMkdir(t, filepath.Join(wd, ".git"))

				return []string{"-R", fixtureRepo}
			},
			remote:      map[string]string{"ticket.md.tmpl": "remote"},
			source:      ".github/release-changes/ticket.md.tmpl (example-org/app, default branch)",
			body:        "remote",
			remoteAsked: true,
		},
		{
			name:        "the default branch",
			setup:       func(*testing.T, string) []string { return nil },
			remote:      map[string]string{"ticket.md.tmpl": "remote {{ template \"part.md.tmpl\" . }}", "part.md.tmpl": "part"},
			source:      ".github/release-changes/ticket.md.tmpl (example-org/app, default branch)",
			body:        "remote part",
			remoteAsked: true,
		},
		{
			name:        "built in, when the default branch has parts but no entry point",
			setup:       func(*testing.T, string) []string { return nil },
			remote:      map[string]string{"part.md.tmpl": "part"},
			source:      "built in",
			body:        "Release v2.0.0 of example-org/current",
			remoteAsked: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, fakes := ticketApp(t)
			fakes.templates.files = tc.remote

			wd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			args := append([]string{"ticket", "v2.0.0", "--dry-run"}, tc.setup(t, wd)...)

			_, stdout, stderr, err := runApp(t, app, nil, args...)
			if err != nil {
				t.Fatalf("ticket: %v", err)
			}

			if want := "info: template: " + tc.source + "\n"; !strings.Contains(stderr, want) {
				t.Errorf("stderr = %q, want %q", stderr, want)
			}

			if !strings.Contains(stdout, tc.body) {
				t.Errorf("stdout = %q, want %q in it", stdout, tc.body)
			}

			if fakes.templates.asked != tc.remoteAsked {
				t.Errorf("default branch asked = %v, want %v", fakes.templates.asked, tc.remoteAsked)
			}
		})
	}
}

func TestTicket_MissingTemplateFlag(t *testing.T) {
	app, _ := ticketApp(t)

	_, _, _, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--template", "nope.md.tmpl")
	if !errors.Is(err, releasechanges.ErrTemplate) {
		t.Errorf("error = %v, want ErrTemplate", err)
	}
}

// An unreadable title is warned about and exits 4, after the ticket is
// written.
func TestTicket_UnreadableTitles(t *testing.T) {
	app, fakes := ticketApp(t)
	app.NewTitleSource = func(string) (cmd.TitleSource, error) {
		return &fakeTitles{titles: map[int]string{1510: "one"}, unreadable: map[int]string{1514: "gone"}}, nil
	}

	_, _, stderr, err := runApp(t, app, nil, "-R", fixtureRepo, "ticket", "v2.0.0", "--yes", "--titles")
	if code := exit.Of(err); code != exit.Skipped {
		t.Fatalf("exit code = %s, want %s (error: %v)", code, exit.Skipped, err)
	}

	if fakes.target.created == nil || !strings.Contains(fakes.target.created.Body, "* #1510 one\n") {
		t.Errorf("the ticket was not written with the titles: %+v", fakes.target.created)
	}

	if !strings.Contains(stderr, "the title of issue #1514 could not be read") {
		t.Errorf("stderr = %q, want the warning", stderr)
	}
}

func TestTicket_Help(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{
			[]string{"ticket", "--help"},
			[]string{".github/release-changes/ticket.md.tmpl", "ticket init", ".Vars", "docs.atom.codes/sprout",
				"Template file (default: .github/release-changes/ticket.md.tmpl in the repository root, then on the default branch, then built in)"},
		},
		{[]string{"ticket", "init", "--help"}, []string{"ticket.md.tmpl", "checklist.md.tmpl", ".github/release-changes/"}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			_, stdout, _, err := run(t, nil, tc.args...)
			if err != nil {
				t.Fatalf("help: %v", err)
			}

			for _, want := range tc.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("help does not mention %q:\n%s", want, stdout)
				}
			}
		})
	}
}

// A tag named init is passed after --.
func TestTicket_TagNamedInit(t *testing.T) {
	app, fakes := ticketApp(t)

	if _, _, _, err := runApp(t, app, nil, "-R", fixtureRepo, "--previous-tag", "v1.0.0", "ticket", "--dry-run", "--", "init"); err != nil {
		t.Fatalf("ticket -- init: %v", err)
	}

	if fakes.notes.tag != "init" {
		t.Errorf("tag = %q, want init", fakes.notes.tag)
	}
}

func mustMkdir(t *testing.T, dir string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	return dir
}
