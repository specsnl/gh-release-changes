package changes_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
)

var update = flag.Bool("update", false, "rewrite the .golden files from the current output")

// fixtures are real generate-notes bodies with the repositories, the people,
// and the titles that would identify them anonymised; the structure is as
// captured. The .txt extension keeps markdownlint off them.
var fixtures = []struct {
	name   string
	source string
}{
	{"final", "a final release: digit scopes, unclassified titles, New Contributors, dependabot"},
	{"rc", "a release candidate: build(deps) from dependabot, free-form titles"},
	{"patch", "a patch release: deps and deps-dev scopes, a fix with a scope"},
	{"dependabot", "a dependency-only release: dependabot deps-dev updates"},
	{"release-yml", "a repository with .github/release.yml: ### categories, CRLF"},
}

// TestParse_Golden parses every captured body and compares the items JSON with
// testdata/<name>.golden. Rewrite the files after an intended change with
//
//	go test ./internal/changes -update
func TestParse_Golden(t *testing.T) {
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("testdata", fx.name+".txt"))
			if err != nil {
				t.Fatalf("reading the fixture (%s): %v", fx.source, err)
			}

			got, err := json.MarshalIndent(changes.Parse(string(body)), "", "  ")
			if err != nil {
				t.Fatalf("marshalling: %v", err)
			}

			got = append(got, '\n')
			path := filepath.Join("testdata", fx.name+".golden")

			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("writing %s: %v", path, err)
				}

				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v (run the tests with -update to create it)", path, err)
			}

			if string(got) != string(want) {
				t.Errorf("items for %s do not match %s\n--- got ---\n%s", fx.source, path, got)
			}
		})
	}
}

// The same body parsed twice gives byte-identical output.
func TestParse_Deterministic(t *testing.T) {
	for _, fx := range fixtures {
		body, err := os.ReadFile(filepath.Join("testdata", fx.name+".txt"))
		if err != nil {
			t.Fatalf("reading %s: %v", fx.name, err)
		}

		first, _ := json.Marshal(changes.Parse(string(body)))
		second, _ := json.Marshal(changes.Parse(string(body)))

		if string(first) != string(second) {
			t.Errorf("%s: two parses differ", fx.name)
		}
	}
}

// The release.yml fixture's ### headings make no difference: stripped of them,
// the body parses to the same items.
func TestParse_ReleaseYmlHeadingsDoNotMatter(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "release-yml.txt"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	var stripped []byte

	for l := range strings.Lines(string(body)) {
		if !strings.HasPrefix(l, "### ") {
			stripped = append(stripped, l...)
		}
	}

	with, _ := json.Marshal(changes.Parse(string(body)))
	without, _ := json.Marshal(changes.Parse(string(stripped)))

	if string(with) != string(without) {
		t.Errorf("headings changed the result:\nwith    %s\nwithout %s", with, without)
	}

	if len(changes.Parse(string(body))) == 0 {
		t.Error("the release.yml fixture parsed to nothing")
	}
}
