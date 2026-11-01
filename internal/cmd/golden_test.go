package cmd_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/github"
)

var update = flag.Bool("update", false, "rewrite the .golden files from the current output")

// fixtures are the captured generate-notes bodies the parser is tested on, in
// internal/changes/testdata. The commands are golden-tested over the same ones.
var fixtures = []string{"final", "rc", "patch", "dependabot", "release-yml"}

// fixtureNotes returns a fake NotesSource answering with the named fixture.
func fixtureNotes(t *testing.T, name string) *fakeNotes {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "changes", "testdata", name+".txt"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	return &fakeNotes{notes: github.Notes{Body: string(body), PreviousTag: "v1.9.0"}}
}

// assertGolden compares got with testdata/<path>. Rewrite the files after an
// intended change with
//
//	go test ./internal/cmd -update
func assertGolden(t *testing.T, path, got string) {
	t.Helper()

	path = filepath.Join("testdata", path)

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(path), err)
		}

		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v (run the tests with -update to create it)", path, err)
	}

	if got != string(want) {
		t.Errorf("output does not match %s\n--- got ---\n%s", path, got)
	}
}
