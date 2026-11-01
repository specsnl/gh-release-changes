package cmd_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

func TestItems_Golden(t *testing.T) {
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			_, stdout, _, err := runApp(t, appWith(fixtureNotes(t, name)), nil,
				"-R", "example-org/app", "items", "v2.0.0")
			if err != nil {
				t.Fatalf("items: %v", err)
			}

			assertGolden(t, "items/"+name+".golden", stdout)
		})
	}
}

// The document is an object carrying the range, so fields can be added; the
// repository is the one the client resolved, also without -R.
func TestItems_Document(t *testing.T) {
	fake := &fakeNotes{notes: github.Notes{
		Body:        "* feat(12)!: a change by @octocat in https://github.com/example-org/current/pull/3\n",
		PreviousTag: "v1.1.0",
	}}

	_, stdout, _, err := runApp(t, appWith(fake), nil, "items", "v1.2.0")
	if err != nil {
		t.Fatalf("items: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not a JSON object: %v\n%s", err, stdout)
	}

	for key, want := range map[string]any{"repo": "example-org/current", "tag": "v1.2.0", "previous_tag": "v1.1.0"} {
		if doc[key] != want {
			t.Errorf("%s = %v, want %v", key, doc[key], want)
		}
	}

	var release changes.Release
	if err := json.Unmarshal([]byte(stdout), &release); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if len(release.Items) != 1 || !release.Items[0].Breaking || release.Items[0].Group != changes.GroupFeatures {
		t.Errorf("items = %+v, want the one breaking feature", release.Items)
	}
}

// A release with no pull requests is still an object, with an empty array.
func TestItems_Empty(t *testing.T) {
	fake := &fakeNotes{notes: github.Notes{Body: "**Full Changelog**: https://github.com/o/r/compare/v1...v2"}}

	_, stdout, _, err := runApp(t, appWith(fake), nil, "items", "v2")
	if err != nil {
		t.Fatalf("items: %v", err)
	}

	if !strings.Contains(stdout, `"items": []`) {
		t.Errorf("stdout = %s, want an empty items array", stdout)
	}
}

// Unclassified items are listed, warned about on stderr, and do not fail the run.
func TestItems_WarnsAboutUnclassified(t *testing.T) {
	fake := &fakeNotes{notes: github.Notes{Body: "" +
		"* Free-form title by @alice in https://github.com/o/r/pull/7\n" +
		"* feat: fine by @alice in https://github.com/o/r/pull/8\n" +
		"* Another one by @bob in https://github.com/o/r/pull/9\n"}}

	_, stdout, stderr, err := runApp(t, appWith(fake), nil, "items", "v2")
	if err != nil {
		t.Fatalf("items: %v", err)
	}

	if !strings.Contains(stdout, `"group": "unclassified"`) {
		t.Errorf("the unclassified items are missing from stdout:\n%s", stdout)
	}

	if !strings.HasPrefix(stderr, "warning: 2 pull requests") || !strings.Contains(stderr, "#7, #9") {
		t.Errorf("stderr = %q, want a warning naming #7 and #9", stderr)
	}
}

func TestItems_Failure(t *testing.T) {
	fake := &fakeNotes{err: fmt.Errorf("%w: v9.9.9", releasechanges.ErrTagNotFound)}

	_, stdout, _, err := runApp(t, appWith(fake), nil, "items", "v9.9.9")
	if !errors.Is(err, releasechanges.ErrTagNotFound) {
		t.Fatalf("error = %v, want ErrTagNotFound", err)
	}

	if stdout != "" {
		t.Errorf("a failed run produced a product: %q", stdout)
	}
}
