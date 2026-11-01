package cmd_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// fakeTags is a TagSource that answers from memory.
type fakeTags struct {
	tags []string
	err  error

	listed bool
}

func (f *fakeTags) Tags(context.Context) ([]string, error) {
	f.listed = true

	return f.tags, f.err
}

// fakePicker records the tags it was offered and picks one.
type fakePicker struct {
	pick string
	err  error

	offered []string
}

func (p *fakePicker) Pick(_ context.Context, tags []string) (string, error) {
	p.offered = tags

	return p.pick, p.err
}

// pickerApp returns an App over fake notes and tags, with picker as the prompt
// when it is not nil.
func pickerApp(notes *fakeNotes, tags *fakeTags, picker *fakePicker) *cmd.App {
	app := appWith(notes)
	app.NewTagSource = func(string) (cmd.TagSource, error) { return tags, nil }
	app.NewDetailsSource = func(string) (cmd.DetailsSource, error) { return &fakeDetails{}, nil }

	if picker != nil {
		app.PickTag = picker.Pick
	}

	return app
}

var tagCommands = []string{"items", "sort", "issues", "dump", "notes"}

// Without a terminal a missing tag fails before anything is requested, and
// says how to pass it.
func TestTag_MissingWithoutATerminal(t *testing.T) {
	for _, name := range tagCommands {
		t.Run(name, func(t *testing.T) {
			notes, tags := fixtureNotes(t, "final"), &fakeTags{tags: []string{"v1.0.0"}}

			_, stdout, _, err := runApp(t, pickerApp(notes, tags, nil), nil, name)
			if !errors.Is(err, releasechanges.ErrInteractiveRequired) {
				t.Fatalf("error = %v, want ErrInteractiveRequired", err)
			}

			if code := exit.Of(err); code != exit.Error {
				t.Errorf("exit code = %s, want %s", code, exit.Error)
			}

			if want := "`gh release-changes " + name + " <tag>`"; !strings.Contains(err.Error(), want) {
				t.Errorf("error = %q, want the hint %s", err, want)
			}

			if tags.listed || notes.tag != "" {
				t.Error("something was requested before the guard")
			}

			if stdout != "" {
				t.Errorf("a refused run produced a product: %q", stdout)
			}
		})
	}
}

// A bad flag is reported before the missing tag, as it would be with one.
func TestTag_FlagsAreCheckedFirst(t *testing.T) {
	_, _, _, err := runApp(t, pickerApp(fixtureNotes(t, "final"), &fakeTags{}, nil), nil, "issues", "--issue-repo", "app")
	if !errors.Is(err, releasechanges.ErrInvalidRepoRef) {
		t.Errorf("error = %v, want ErrInvalidRepoRef", err)
	}
}

// The picker is offered every tag in semver order, and the run uses the pick.
func TestTag_Picked(t *testing.T) {
	for _, name := range tagCommands {
		t.Run(name, func(t *testing.T) {
			notes := fixtureNotes(t, "final")
			tags := &fakeTags{tags: []string{"v1.9.0", "latest", "v2.0.0-rc.1", "v2.0.0", "v1.10.0"}}
			picker := &fakePicker{pick: "v2.0.0"}

			_, stdout, _, err := runApp(t, pickerApp(notes, tags, picker), nil, name)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}

			if want := []string{"v2.0.0", "v2.0.0-rc.1", "v1.10.0", "v1.9.0", "latest"}; !slices.Equal(picker.offered, want) {
				t.Errorf("offered %v, want %v", picker.offered, want)
			}

			if notes.tag != "v2.0.0" {
				t.Errorf("notes generated for %q, want the picked tag", notes.tag)
			}

			if stdout == "" {
				t.Error("the picked tag produced no product")
			}
		})
	}
}

// With a tag argument nothing is listed and nobody is asked.
func TestTag_GivenIsNotPicked(t *testing.T) {
	notes, tags, picker := fixtureNotes(t, "final"), &fakeTags{tags: []string{"v1.0.0"}}, &fakePicker{pick: "v1.0.0"}

	if _, _, _, err := runApp(t, pickerApp(notes, tags, picker), nil, "sort", "v2.0.0"); err != nil {
		t.Fatalf("sort: %v", err)
	}

	if tags.listed || picker.offered != nil {
		t.Error("the tags were listed or offered although a tag was given")
	}

	if notes.tag != "v2.0.0" {
		t.Errorf("notes generated for %q, want %q", notes.tag, "v2.0.0")
	}
}

func TestTag_Failures(t *testing.T) {
	listFailure := errors.New("listing broke")
	pickFailure := errors.New("picker broke")

	for _, tc := range []struct {
		name   string
		tags   *fakeTags
		picker *fakePicker
		want   error
	}{
		{"no tags", &fakeTags{tags: []string{}}, &fakePicker{}, releasechanges.ErrTagNotFound},
		{"listing fails", &fakeTags{err: listFailure}, &fakePicker{}, listFailure},
		{"picker fails", &fakeTags{tags: []string{"v1.0.0"}}, &fakePicker{err: pickFailure}, pickFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notes := fixtureNotes(t, "final")

			_, stdout, _, err := runApp(t, pickerApp(notes, tc.tags, tc.picker), nil, "sort")
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}

			if notes.tag != "" || stdout != "" {
				t.Error("the run went on without a tag")
			}
		})
	}
}

func TestTag_AtMostOne(t *testing.T) {
	_, _, _, err := runApp(t, pickerApp(fixtureNotes(t, "final"), &fakeTags{}, nil), nil, "sort", "v1", "v2")
	if code := exit.Of(err); code != exit.Error {
		t.Errorf("exit code = %s, want %s", code, exit.Error)
	}
}
