package render_test

import (
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"text/template/parse"

	"github.com/go-sprout/sprout"
	"github.com/go-sprout/sprout/registry/env"
	"github.com/go-sprout/sprout/registry/filesystem"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/render"
)

const ticketRepo = "example-org/web-components"

// exampleVars are the --var values of the example command in the init
// template's comment, so the golden file renders what a user trying it sees.
func exampleVars(t *testing.T) map[string]string {
	t.Helper()

	vars := map[string]string{}

	for _, m := range regexp.MustCompile(`--var (\w+)=(\S+)`).FindAllStringSubmatch(render.InitTicketFiles()["ticket.md.tmpl"], -1) {
		vars[m[1]] = m[2]
	}

	if len(vars) == 0 {
		t.Fatal("the init template's example command has no --var")
	}

	return vars
}

// ticketData is the final fixture as the ticket command passes it.
func ticketData(t *testing.T, tag string, vars map[string]string) render.TicketData {
	t.Helper()

	items := fixtureItems(t, "final")
	refs := render.IssueRefs{IssueRepo: ticketRepo, SourceRepo: ticketRepo, RelativeTo: ticketRepo}

	return render.TicketData{
		Repo:        ticketRepo,
		Tag:         tag,
		Version:     strings.TrimPrefix(tag, "v"),
		PreviousTag: "v1.9.0",
		Issues:      render.Issues(items, refs, render.IssueList{Unlinked: true}),
		Items:       items,
		Vars:        vars,
	}
}

func renderTicket(t *testing.T, files map[string]string, title string, data render.TicketData) (string, string) {
	t.Helper()

	tmpl, err := render.ParseTicket(files, "ticket.md.tmpl")
	if err != nil {
		t.Fatalf("ParseTicket: %v", err)
	}

	if title == "" {
		title = tmpl.Title
	}

	gotTitle, body, err := tmpl.Render(title, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	return gotTitle, body
}

func TestTicket_Golden(t *testing.T) {
	custom := map[string]string{
		"ticket.md.tmpl": "{{ include \"header.md.tmpl\" . | toUpper }}\n\n" +
			"<!-- gh-release-changes:start -->\n{{ .Issues -}}\n<!-- gh-release-changes:end -->\n\n" +
			"Deploy to {{ .Vars.environment }} ({{ index .Vars \"window\" | default \"any time\" }}).\n" +
			"{{ template \"footer.md.tmpl\" . }}",
		"header.md.tmpl": "release {{ .Version }}",
		"footer.md.tmpl": "{{ len .Items }} pull requests, {{ if semverCompare \">=2.0.0\" .Version }}a major{{ else }}a minor{{ end }} release.\n",
	}

	for _, tc := range []struct {
		name, tag, title string
		files            map[string]string
		vars             map[string]string
		wantTitle        string
	}{
		{"default", "v2.0.0", render.DefaultTicketTitle, render.DefaultTicketFiles(), nil, "Release 2.0.0"},
		{"init", "v1.4.0-rc.1", "", render.InitTicketFiles(), exampleVars(t), "Release 1.4.0-rc.1"},
		{"custom", "v2.0.0", "WOO: release {{ .Version }} naar {{ .Vars.environment }}", custom,
			map[string]string{"environment": "PROD"}, "WOO: release 2.0.0 naar PROD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			title, body := renderTicket(t, tc.files, tc.title, ticketData(t, tc.tag, tc.vars))

			if title != tc.wantTitle {
				t.Errorf("title = %q, want %q", title, tc.wantTitle)
			}

			assertGolden(t, "ticket/"+tc.name+".golden", body)
		})
	}
}

func TestTicket_FrontMatter(t *testing.T) {
	tmpl, err := render.ParseTicket(map[string]string{
		"ticket.md.tmpl": "---\ntitle: \"Release {{ .Version }}: {{ .Vars.environment }}\"\nlabels: [release, woo]\n---\nbody\n",
	}, "ticket.md.tmpl")
	if err != nil {
		t.Fatalf("ParseTicket: %v", err)
	}

	if want := "Release {{ .Version }}: {{ .Vars.environment }}"; tmpl.Title != want {
		t.Errorf("Title = %q, want %q", tmpl.Title, want)
	}

	if want := []string{"release", "woo"}; !reflect.DeepEqual(tmpl.Labels, want) {
		t.Errorf("Labels = %v, want %v", tmpl.Labels, want)
	}

	_, body, err := tmpl.Render(tmpl.Title, render.TicketData{Version: "1.0.0", Vars: map[string]string{"environment": "PROD"}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if want := render.TicketStart + "\nbody\n" + render.TicketEnd + "\n"; body != want {
		t.Errorf("body = %q, want %q: the front matter must not render", body, want)
	}
}

func TestTicket_NoFrontMatter(t *testing.T) {
	tmpl, err := render.ParseTicket(map[string]string{"ticket.md.tmpl": "---- not front matter\n"}, "ticket.md.tmpl")
	if err != nil {
		t.Fatalf("ParseTicket: %v", err)
	}

	if tmpl.Title != "" || tmpl.Labels != nil {
		t.Errorf("Title, Labels = %q, %v, want none", tmpl.Title, tmpl.Labels)
	}
}

func TestTicket_ParseErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"unknown front matter key", map[string]string{"ticket.md.tmpl": "---\ntitel: x\n---\n"}, "titel"},
		{"unclosed front matter", map[string]string{"ticket.md.tmpl": "---\ntitle: x\n"}, "no closing ---"},
		{"a part that does not parse", map[string]string{"ticket.md.tmpl": "ok", "checklist.md.tmpl": "{{ .Broken "}, "checklist.md.tmpl"},
		{"an unknown function", map[string]string{"ticket.md.tmpl": "{{ env \"GH_TOKEN\" }}"}, `"env" not defined`},
		{"a missing entry point", map[string]string{"other.md.tmpl": "x"}, "ticket.md.tmpl is missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := render.ParseTicket(tc.files, "ticket.md.tmpl")
			if !errors.Is(err, releasechanges.ErrTemplate) {
				t.Fatalf("error = %v, want ErrTemplate", err)
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// Front matter is cut without moving the lines below it, so an error names
// the line it is on in the file.
func TestTicket_ErrorsKeepTheirLine(t *testing.T) {
	_, err := render.ParseTicket(map[string]string{
		"ticket.md.tmpl": "---\ntitle: x\nlabels: [a]\n---\nline 5\n{{ .Broken ",
	}, "ticket.md.tmpl")
	if err == nil || !strings.Contains(err.Error(), "ticket.md.tmpl:6") {
		t.Errorf("error = %v, want it at ticket.md.tmpl:6", err)
	}
}

func TestTicket_RenderErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, title, want string
	}{
		{"a misspelled var", "{{ .Vars.enviroment }}", "t", `map has no entry for key "enviroment"`},
		{"a field that does not exist", "{{ .Release }}", "t", "Release"},
		{"an empty title", "body", "{{ index .Vars \"none\" }}", "renders empty"},
		{"a title that does not parse", "body", "{{ .Version ", "the title"},
		{"two start markers", render.TicketStart + render.TicketStart + render.TicketEnd, "t", "want each once"},
		{"the end marker first", render.TicketEnd + render.TicketStart, "t", "want each once"},
		{"only a start marker", render.TicketStart, "t", "want each once"},
		{"an include that recurses", `{{ include "ticket.md.tmpl" . }}`, "t", "nested more than"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := render.ParseTicket(map[string]string{"ticket.md.tmpl": tc.body}, "ticket.md.tmpl")
			if err != nil {
				t.Fatalf("ParseTicket: %v", err)
			}

			_, _, err = tmpl.Render(tc.title, render.TicketData{Version: "1.0.0", Vars: map[string]string{}})
			if !errors.Is(err, releasechanges.ErrTemplate) {
				t.Fatalf("error = %v, want ErrTemplate", err)
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// The title is one line, however the template spreads it.
func TestTicket_TitleIsFolded(t *testing.T) {
	tmpl, err := render.ParseTicket(map[string]string{"ticket.md.tmpl": "body"}, "ticket.md.tmpl")
	if err != nil {
		t.Fatalf("ParseTicket: %v", err)
	}

	title, _, err := tmpl.Render("  Release\n{{ .Version }}  \n", render.TicketData{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if title != "Release 1.0.0" {
		t.Errorf("title = %q, want %q", title, "Release 1.0.0")
	}
}

func TestTicket_Markers(t *testing.T) {
	start, end := render.TicketStart, render.TicketEnd

	for _, tc := range []struct {
		name, body, want string
	}{
		{"no markers: the whole body is generated", "list\n", start + "\nlist\n" + end + "\n"},
		{"no markers, no final newline", "list", start + "\nlist\n" + end + "\n"},
		{"one pair", "intro\n" + start + "\nlist\n" + end + "\noutro\n", "intro\n" + start + "\nlist\n" + end + "\noutro\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, body := renderTicket(t, map[string]string{"ticket.md.tmpl": tc.body}, "t", render.TicketData{})

			if body != tc.want {
				t.Errorf("body = %q, want %q", body, tc.want)
			}
		})
	}
}

func TestReplaceGenerated(t *testing.T) {
	start, end := render.TicketStart, render.TicketEnd
	body := "new intro\n" + start + "\nnew list\n" + end + "\nnew checklist\n"
	block := start + "\nnew list\n" + end

	for _, tc := range []struct {
		name, existing, want string
		replaced             bool
	}{
		{"one pair", start + "\nold list\n" + end + "\n", block + "\n", true},
		{
			"text around the markers is kept",
			"# Answers\n\n- [x] ticked\n" + start + "\nold list\n" + end + "\n\nQA: done\n",
			"# Answers\n\n- [x] ticked\n" + block + "\n\nQA: done\n",
			true,
		},
		{"Windows line endings from the web editor", "a\r\n" + start + "\r\nold\r\n" + end + "\r\nb\r\n", "a\r\n" + block + "\r\nb\r\n", true},
		{"no markers: appended", "hand-written\n\n", "hand-written\n\n" + block + "\n", false},
		{"a start without an end: appended", "x " + start + " y", "x " + start + " y\n\n" + block + "\n", false},
		{"an empty body", "", block + "\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, replaced := render.ReplaceGenerated(tc.existing, body)

			if got != tc.want {
				t.Errorf("ReplaceGenerated = %q, want %q", got, tc.want)
			}

			if replaced != tc.replaced {
				t.Errorf("replaced = %v, want %v", replaced, tc.replaced)
			}
		})
	}
}

// A template must not read the environment or local files.
func TestTicketFuncs_LeaveOutEnvAndFilesystem(t *testing.T) {
	funcs, err := render.TicketFuncs()
	if err != nil {
		t.Fatalf("TicketFuncs: %v", err)
	}

	for _, registry := range []sprout.Registry{env.NewRegistry(), filesystem.NewRegistry()} {
		handler := sprout.New()
		if err := handler.AddRegistry(registry); err != nil {
			t.Fatal(err)
		}

		for name := range handler.Build() {
			if _, ok := funcs[name]; ok {
				t.Errorf("%s from the %s registry is in the function map", name, registry.UID())
			}
		}
	}

	for _, name := range []string{"default", "trim", "toUpper", "join", "date", "semverCompare", "indent", "regexReplaceAll"} {
		if _, ok := funcs[name]; !ok {
			t.Errorf("%s is missing from the function map", name)
		}
	}
}

// The init template shows every field a template can use; a field added to
// TicketData without a use in it fails here.
func TestInitTemplate_UsesEveryField(t *testing.T) {
	funcs, err := render.TicketFuncs()
	if err != nil {
		t.Fatalf("TicketFuncs: %v", err)
	}

	funcs["include"] = func(string, any) string { return "" }

	used := map[string]bool{}

	for name, text := range render.InitTicketFiles() {
		set, err := template.New(name).Funcs(funcs).Parse(text)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		for _, tmpl := range set.Templates() {
			collectFields(tmpl.Root, used)
		}
	}

	for field := range reflect.TypeFor[render.TicketData]().Fields() {
		if !used[field.Name] {
			t.Errorf(".%s is not used in the init template", field.Name)
		}
	}
}

// collectFields records the first identifier of every field the tree reads.
func collectFields(node parse.Node, used map[string]bool) {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}

		for _, child := range n.Nodes {
			collectFields(child, used)
		}
	case *parse.ActionNode:
		collectFields(n.Pipe, used)
	case *parse.PipeNode:
		if n == nil {
			return
		}

		for _, cmd := range n.Cmds {
			collectFields(cmd, used)
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			collectFields(arg, used)
		}
	case *parse.FieldNode:
		used[n.Ident[0]] = true
	case *parse.ChainNode:
		collectFields(n.Node, used)
	case *parse.IfNode:
		collectBranch(&n.BranchNode, used)
	case *parse.RangeNode:
		collectBranch(&n.BranchNode, used)
	case *parse.WithNode:
		collectBranch(&n.BranchNode, used)
	case *parse.TemplateNode:
		collectFields(n.Pipe, used)
	}
}

func collectBranch(n *parse.BranchNode, used map[string]bool) {
	collectFields(n.Pipe, used)
	collectFields(n.List, used)
	collectFields(n.ElseList, used)
}
