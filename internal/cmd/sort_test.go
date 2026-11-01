package cmd_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// The command prints the renderer's output unchanged, so it is compared with
// the renderer's own golden files rather than a copy of them.
func TestSort_PrintsTheRenderedMarkdown(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			_, stdout, _, err := runApp(t, appWith(fixtureNotes(t, name)), nil, "sort", "v2.0.0")
			if err != nil {
				t.Fatalf("sort: %v", err)
			}

			want, err := os.ReadFile(filepath.Join("..", "render", "testdata", "sort", name+".golden"))
			if err != nil {
				t.Fatalf("reading the renderer's golden file: %v", err)
			}

			if stdout != string(want) {
				t.Errorf("stdout differs from render.Sort's golden output:\n%s", stdout)
			}
		})
	}
}

// Unclassified items are rendered under their own heading and warned about on
// stderr, never on stdout, where the Markdown has to stay pasteable.
func TestSort_WarnsAboutUnclassifiedOnStderr(t *testing.T) {
	fake := &fakeNotes{notes: github.Notes{Body: "* Free-form title by @alice in https://github.com/o/r/pull/7\n"}}

	_, stdout, stderr, err := runApp(t, appWith(fake), nil, "sort", "v2")
	if err != nil {
		t.Fatalf("sort: %v", err)
	}

	if !strings.Contains(stdout, "### Unclassified\n\n* Free-form title by @alice") {
		t.Errorf("stdout lacks the unclassified item:\n%s", stdout)
	}

	if strings.Contains(stdout, "warning") || !strings.Contains(stderr, "warning: 1 pull request has a title") {
		t.Errorf("the warning is misplaced:\nstdout %q\nstderr %q", stdout, stderr)
	}
}

func TestSort_Failure(t *testing.T) {
	fake := &fakeNotes{err: fmt.Errorf("%w: example-org/app", releasechanges.ErrForbidden)}

	_, stdout, _, err := runApp(t, appWith(fake), nil, "sort", "v2")
	if !errors.Is(err, releasechanges.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}

	if stdout != "" {
		t.Errorf("a failed run produced a product: %q", stdout)
	}
}
