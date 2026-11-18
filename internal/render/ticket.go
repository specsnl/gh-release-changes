package render

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"slices"
	"strings"
	"text/template"

	"github.com/go-sprout/sprout"
	"github.com/go-sprout/sprout/registry/checksum"
	"github.com/go-sprout/sprout/registry/conversion"
	"github.com/go-sprout/sprout/registry/crypto"
	"github.com/go-sprout/sprout/registry/encoding"
	sproutmaps "github.com/go-sprout/sprout/registry/maps"
	"github.com/go-sprout/sprout/registry/network"
	"github.com/go-sprout/sprout/registry/numeric"
	"github.com/go-sprout/sprout/registry/random"
	sproutreflect "github.com/go-sprout/sprout/registry/reflect"
	"github.com/go-sprout/sprout/registry/regex"
	sproutsemver "github.com/go-sprout/sprout/registry/semver"
	sproutslices "github.com/go-sprout/sprout/registry/slices"
	"github.com/go-sprout/sprout/registry/std"
	sproutstrings "github.com/go-sprout/sprout/registry/strings"
	sprouttime "github.com/go-sprout/sprout/registry/time"
	"github.com/go-sprout/sprout/registry/uniqueid"
	"go.yaml.in/yaml/v3"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// The markers around the generated part of a ticket. An update rewrites what
// is between them and leaves the rest of the body alone.
const (
	TicketStart = "<!-- gh-release-changes:start -->"
	TicketEnd   = "<!-- gh-release-changes:end -->"
)

// DefaultTicketTitle is the title of a ticket whose template sets none in its
// front matter.
const DefaultTicketTitle = "Release {{ .Version }}"

// DefaultTicketLabels are the labels of a ticket whose template sets none.
var DefaultTicketLabels = []string{"release"}

// maxIncludeDepth bounds nested include calls, so a part that includes itself
// fails instead of overflowing the stack.
const maxIncludeDepth = 100

// TicketData is what a ticket template renders. The fields are public API for
// template authors: added to, never renamed. The init template uses every one
// of them, which a test enforces.
type TicketData struct {
	// Repo is the source repository, OWNER/REPO.
	Repo string

	// Tag is the release's tag; Version is the tag without a leading v.
	Tag     string
	Version string

	// PreviousTag is the base of the comparison, "" for a first release.
	PreviousTag string

	// Issues is the issue list, rendered as the issues command renders it.
	Issues string

	// Items are the release's pull requests, as in the items JSON.
	Items []changes.Item

	// Vars are the --var key=value pairs.
	Vars map[string]string
}

//go:embed templates
var templates embed.FS

// DefaultTicketFiles returns the built-in ticket template set, used when no
// other template is found.
func DefaultTicketFiles() map[string]string {
	return embedded("templates/default")
}

// InitTicketFiles returns the starter template set ticket init writes.
func InitTicketFiles() map[string]string {
	return embedded("templates/init")
}

func embedded(dir string) map[string]string {
	entries, err := fs.ReadDir(templates, dir)
	if err != nil {
		panic(fmt.Sprintf("the embedded %s is missing: %v", dir, err))
	}

	files := map[string]string{}

	for _, e := range entries {
		text, err := fs.ReadFile(templates, path.Join(dir, e.Name()))
		if err != nil {
			panic(fmt.Sprintf("the embedded %s is unreadable: %v", e.Name(), err))
		}

		files[e.Name()] = string(text)
	}

	return files
}

// TicketFuncs returns the functions a ticket template can use: Sprout's
// registries, without env and filesystem — a template from a repository must
// not read the environment or local files, which could put a CI token in an
// issue and would render differently locally — and without backward, Sprig's
// compatibility shims. regex stands in for the deprecated regexp; Sprout
// registers one or the other.
func TicketFuncs() (template.FuncMap, error) {
	handler := sprout.New(sprout.WithLogger(slog.Default()))

	err := handler.AddRegistries(
		checksum.NewRegistry(),
		conversion.NewRegistry(),
		crypto.NewRegistry(),
		encoding.NewRegistry(),
		sproutmaps.NewRegistry(),
		network.NewRegistry(),
		numeric.NewRegistry(),
		random.NewRegistry(),
		sproutreflect.NewRegistry(),
		regex.NewRegistry(),
		sproutsemver.NewRegistry(),
		sproutslices.NewRegistry(),
		std.NewRegistry(),
		sproutstrings.NewRegistry(),
		sprouttime.NewRegistry(),
		uniqueid.NewRegistry(),
	)
	if err != nil {
		return nil, fmt.Errorf("register the template functions: %w", err)
	}

	return handler.Build(), nil
}

// TicketTemplate is a parsed ticket template set: the entry point and every
// part next to it, with the defaults the entry point's front matter sets.
type TicketTemplate struct {
	// Title and Labels are the front matter's, "" and nil when it sets none.
	Title  string
	Labels []string

	set   *template.Template
	entry string
}

type frontMatter struct {
	Title  string   `yaml:"title"`
	Labels []string `yaml:"labels"`
}

// ParseTicket parses files, keyed by file name, into one template set whose
// entry point is entry. Every file can include every other, through
// {{ template "name" . }} or the include function. A file that does not parse
// fails with ErrTemplate, naming it.
//
// The entry point may start with YAML front matter between --- lines, setting
// title and labels.
func ParseTicket(files map[string]string, entry string) (*TicketTemplate, error) {
	text, ok := files[entry]
	if !ok {
		return nil, fmt.Errorf("%w: %s is missing", releasechanges.ErrTemplate, entry)
	}

	fm, text, err := splitFrontMatter(text)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: front matter: %w", releasechanges.ErrTemplate, entry, err)
	}

	funcs, err := TicketFuncs()
	if err != nil {
		return nil, err
	}

	set := template.New(entry).Option("missingkey=error")

	depth := 0
	funcs["include"] = func(name string, data any) (string, error) {
		if depth >= maxIncludeDepth {
			return "", fmt.Errorf("include %q: nested more than %d deep", name, maxIncludeDepth)
		}

		depth++
		defer func() { depth-- }()

		var b strings.Builder
		if err := set.ExecuteTemplate(&b, name, data); err != nil {
			return "", err
		}

		return b.String(), nil
	}

	set.Funcs(funcs)

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		source := files[name]
		if name == entry {
			source = text
		}

		if _, err := set.New(name).Parse(source); err != nil {
			return nil, fmt.Errorf("%w: %w", releasechanges.ErrTemplate, err)
		}
	}

	return &TicketTemplate{Title: fm.Title, Labels: fm.Labels, set: set, entry: entry}, nil
}

// splitFrontMatter cuts the front matter off text. The lines it took are left
// blank, and a trim action on the closing line removes them and the whitespace
// after it, so the body renders without them and a template error still names
// the line it is on in the file.
func splitFrontMatter(text string) (frontMatter, string, error) {
	lines := strings.SplitAfter(text, "\n")
	if strings.TrimRight(lines[0], "\r\n") != "---" {
		return frontMatter{}, text, nil
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") != "---" {
			continue
		}

		var fm frontMatter

		decoder := yaml.NewDecoder(strings.NewReader(strings.Join(lines[1:i], "")))
		decoder.KnownFields(true)

		if err := decoder.Decode(&fm); err != nil && !errors.Is(err, io.EOF) {
			return frontMatter{}, "", err
		}

		closing := `{{- "" -}}` + lines[i][len("---"):]

		return fm, strings.Repeat("\n", i) + closing + strings.Join(lines[i+1:], ""), nil
	}

	return frontMatter{}, "", errors.New("no closing ---")
}

// Render renders the title template and the body for data. Both run with
// missingkey=error, so a misspelled .Vars key fails rather than rendering
// "<no value>"; a failure is ErrTemplate.
//
// The title is folded onto one line. The body always holds the generated part
// between [TicketStart] and [TicketEnd]: a template without the markers is
// generated as a whole, and one holding them other than once each, start
// first, fails.
func (t *TicketTemplate) Render(title string, data TicketData) (string, string, error) {
	const titleName = "--title"

	if _, err := t.set.New(titleName).Parse(title); err != nil {
		return "", "", fmt.Errorf("%w: the title: %w", releasechanges.ErrTemplate, err)
	}

	var b strings.Builder
	if err := t.set.ExecuteTemplate(&b, titleName, data); err != nil {
		return "", "", fmt.Errorf("%w: the title: %w", releasechanges.ErrTemplate, err)
	}

	renderedTitle := strings.Join(strings.Fields(b.String()), " ")
	if renderedTitle == "" {
		return "", "", fmt.Errorf("%w: the title %q renders empty", releasechanges.ErrTemplate, title)
	}

	b.Reset()

	if err := t.set.ExecuteTemplate(&b, t.entry, data); err != nil {
		return "", "", fmt.Errorf("%w: %w", releasechanges.ErrTemplate, err)
	}

	body, err := markGenerated(b.String())
	if err != nil {
		return "", "", fmt.Errorf("%w: %s: %w", releasechanges.ErrTemplate, t.entry, err)
	}

	return renderedTitle, body, nil
}

func markGenerated(body string) (string, error) {
	starts, ends := strings.Count(body, TicketStart), strings.Count(body, TicketEnd)

	switch {
	case starts == 0 && ends == 0:
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}

		return TicketStart + "\n" + body + TicketEnd + "\n", nil
	case starts == 1 && ends == 1 && strings.Index(body, TicketStart) < strings.Index(body, TicketEnd):
		return body, nil
	default:
		return "", fmt.Errorf("it renders %d %s and %d %s; want each once, start first, or neither",
			starts, TicketStart, ends, TicketEnd)
	}
}

// ReplaceGenerated returns existing with its generated part replaced by the one
// in body, which [TicketTemplate.Render] made. Everything outside the markers
// is kept, so ticked boxes and answers written in the ticket survive.
//
// When existing has no complete marker pair — someone removed one — the
// generated part is appended instead, and replaced reports false.
func ReplaceGenerated(existing, body string) (updated string, replaced bool) {
	start := strings.Index(body, TicketStart)
	end := strings.Index(body, TicketEnd)
	block := body[start : end+len(TicketEnd)]

	if i := strings.Index(existing, TicketStart); i >= 0 {
		if j := strings.Index(existing[i:], TicketEnd); j >= 0 {
			return existing[:i] + block + existing[i+j+len(TicketEnd):], true
		}
	}

	existing = strings.TrimRight(existing, "\r\n")
	if existing == "" {
		return block + "\n", false
	}

	return existing + "\n\n" + block + "\n", false
}
