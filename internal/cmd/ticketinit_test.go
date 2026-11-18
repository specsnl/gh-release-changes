package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/render"
)

// initRepo makes a temp repository root, with a .git directory, and changes
// into a directory nested in it.
func initRepo(t *testing.T) string {
	t.Helper()

	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_WORKSPACE", "")

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	mustMkdir(t, filepath.Join(root, ".git"))
	t.Chdir(mustMkdir(t, filepath.Join(root, "src", "nested")))

	return root
}

func assertInitFiles(t *testing.T, dir string) {
	t.Helper()

	for name, want := range render.InitTicketFiles() {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s was not written: %v", name, err)

			continue
		}

		if string(got) != want {
			t.Errorf("%s differs from the embedded template", name)
		}
	}
}

// From a nested directory, init writes to the repository root.
func TestTicketInit_WritesAtTheRoot(t *testing.T) {
	root := initRepo(t)

	_, stdout, stderr, err := run(t, nil, "ticket", "init")
	if err != nil {
		t.Fatalf("ticket init: %v", err)
	}

	dir := filepath.Join(root, ".github", "release-changes")
	assertInitFiles(t, dir)

	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}

	for _, name := range []string{"ticket.md.tmpl", "checklist.md.tmpl"} {
		if want := "info: wrote " + filepath.Join(dir, name); !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	}
}

func TestTicketInit_RefusesToOverwrite(t *testing.T) {
	root := initRepo(t)
	dir := mustMkdir(t, filepath.Join(root, ".github", "release-changes"))
	checklist := filepath.Join(dir, "checklist.md.tmpl")

	if err := os.WriteFile(checklist, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := run(t, nil, "ticket", "init")
	if !errors.Is(err, releasechanges.ErrTemplateExists) {
		t.Fatalf("error = %v, want ErrTemplateExists", err)
	}

	if !strings.Contains(err.Error(), checklist) || !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want it to name the file and --force", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "ticket.md.tmpl")); !errors.Is(err, os.ErrNotExist) {
		t.Error("ticket.md.tmpl was written although the run failed")
	}

	if _, _, _, err := run(t, nil, "ticket", "init", "--force"); err != nil {
		t.Fatalf("ticket init --force: %v", err)
	}

	assertInitFiles(t, dir)
}

func TestTicketInit_Dir(t *testing.T) {
	initRepo(t)

	dir := filepath.Join(t.TempDir(), "templates")

	if _, _, _, err := run(t, nil, "ticket", "init", "--dir", dir); err != nil {
		t.Fatalf("ticket init --dir: %v", err)
	}

	assertInitFiles(t, dir)
}

func TestTicketInit_Stdout(t *testing.T) {
	root := initRepo(t)

	_, stdout, _, err := run(t, nil, "ticket", "init", "--stdout")
	if err != nil {
		t.Fatalf("ticket init --stdout: %v", err)
	}

	files := render.InitTicketFiles()
	want := "==> ticket.md.tmpl <==\n" + files["ticket.md.tmpl"] + "\n==> checklist.md.tmpl <==\n" + files["checklist.md.tmpl"]

	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	if _, err := os.Stat(filepath.Join(root, ".github")); !errors.Is(err, os.ErrNotExist) {
		t.Error("--stdout wrote files")
	}
}

func TestTicketInit_NoRoot(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_WORKSPACE", "")
	t.Chdir(t.TempDir())

	_, _, _, err := run(t, nil, "ticket", "init")
	if !errors.Is(err, releasechanges.ErrNoRepoRoot) {
		t.Fatalf("error = %v, want ErrNoRepoRoot", err)
	}

	if !strings.Contains(err.Error(), "--dir") {
		t.Errorf("error = %v, want it to point at --dir", err)
	}
}
