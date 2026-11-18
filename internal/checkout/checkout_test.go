package checkout_test

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/checkout"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// isolate keeps the GitHub Actions fallback out of a test, which may itself
// run in GitHub Actions, and returns a fresh temp dir with symlinks resolved —
// git reports the resolved path.
func isolate(t *testing.T) string {
	t.Helper()

	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_WORKSPACE", "")

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()

	dir := filepath.Join(parts...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	return dir
}

func write(t *testing.T, path, text string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A .git that git itself does not accept — an empty directory, a worktree's
// file pointing nowhere — or a .jj is still a root marker, found from a nested
// directory.
func TestRoot_Markers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		marker func(t *testing.T, root string)
	}{
		{".git directory", func(t *testing.T, root string) { mkdir(t, root, ".git") }},
		{".git file (worktree)", func(t *testing.T, root string) {
			write(t, filepath.Join(root, ".git"), "gitdir: /nowhere/.git/worktrees/x\n")
		}},
		{".jj directory", func(t *testing.T, root string) { mkdir(t, root, ".jj") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := isolate(t)
			tc.marker(t, root)
			nested := mkdir(t, root, "a", "b")

			// .github is not a marker: a nested one must not stop the walk.
			mkdir(t, root, "a", ".github")

			got, err := checkout.Root(context.Background(), nested)
			if err != nil {
				t.Fatalf("Root: %v", err)
			}

			if got != root {
				t.Errorf("Root = %q, want %q", got, root)
			}
		})
	}
}

func TestRoot_Git(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := isolate(t)

	if out, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	got, err := checkout.Root(context.Background(), mkdir(t, root, "nested", "deeper"))
	if err != nil {
		t.Fatalf("Root: %v", err)
	}

	if got != root {
		t.Errorf("Root = %q, want %q", got, root)
	}
}

func TestRoot_NoMarker(t *testing.T) {
	dir := isolate(t)

	_, err := checkout.Root(context.Background(), dir)
	if !errors.Is(err, releasechanges.ErrNoRepoRoot) {
		t.Errorf("error = %v, want ErrNoRepoRoot", err)
	}
}

// In GitHub Actions the workspace is the last resort.
func TestRoot_GitHubWorkspace(t *testing.T) {
	dir := isolate(t)
	workspace := t.TempDir()

	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_WORKSPACE", workspace)

	got, err := checkout.Root(context.Background(), dir)
	if err != nil {
		t.Fatalf("Root: %v", err)
	}

	if got != workspace {
		t.Errorf("Root = %q, want %q", got, workspace)
	}
}

func TestReadTemplates(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "ticket.md.tmpl"), "entry")
	write(t, filepath.Join(dir, "checklist.md.tmpl"), "part")
	write(t, filepath.Join(dir, "README.md"), "not a part")
	mkdir(t, dir, "nested.tmpl")
	write(t, filepath.Join(mkdir(t, dir, "sub"), "deep.tmpl"), "not read")

	files, err := checkout.ReadTemplates(filepath.Join(dir, "ticket.md.tmpl"))
	if err != nil {
		t.Fatalf("ReadTemplates: %v", err)
	}

	if got, want := slices.Sorted(maps.Keys(files)), []string{"checklist.md.tmpl", "ticket.md.tmpl"}; !slices.Equal(got, want) {
		t.Errorf("files = %v, want %v", got, want)
	}

	if files["checklist.md.tmpl"] != "part" {
		t.Errorf("checklist.md.tmpl = %q, want %q", files["checklist.md.tmpl"], "part")
	}
}

// An entry point with another extension is read as well.
func TestReadTemplates_EntryWithoutTheExtension(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "release.md"), "entry")
	write(t, filepath.Join(dir, "part.tmpl"), "part")

	files, err := checkout.ReadTemplates(filepath.Join(dir, "release.md"))
	if err != nil {
		t.Fatalf("ReadTemplates: %v", err)
	}

	if got, want := slices.Sorted(maps.Keys(files)), []string{"part.tmpl", "release.md"}; !slices.Equal(got, want) {
		t.Errorf("files = %v, want %v", got, want)
	}
}

func TestReadTemplates_MissingEntry(t *testing.T) {
	_, err := checkout.ReadTemplates(filepath.Join(t.TempDir(), "ticket.md.tmpl"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want fs.ErrNotExist", err)
	}
}
