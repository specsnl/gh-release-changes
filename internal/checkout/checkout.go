// Package checkout reads from the local checkout: it finds the root of the
// repository the current directory is in, and reads the ticket templates
// stored there. It never talks to GitHub, so it is tested in a temp directory.
package checkout

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// TemplateDir is where a repository keeps its ticket templates, relative to
// its root — locally and on the default branch alike.
const TemplateDir = ".github/release-changes"

// TemplateFile is the entry point of the ticket templates in [TemplateDir].
const TemplateFile = "ticket.md.tmpl"

// TemplateExt is the extension of every file in the template set.
const TemplateExt = ".tmpl"

// rootMarkers are the entries that make a directory the root of a repository:
// .git is a directory, or a file in a worktree or a submodule, and .jj is a
// Jujutsu repository without a colocated git one. .github is not one: GitHub
// only reads it at the root, and a vendored copy would be a false hit.
var rootMarkers = []string{".git", ".jj"}

// Root returns the root of the repository dir is in:
//
//  1. what `git rev-parse --show-toplevel` answers, which handles worktrees,
//     submodules, GIT_DIR and GIT_WORK_TREE;
//  2. otherwise the nearest directory at or above dir with a .git or a .jj;
//  3. otherwise, in GitHub Actions, $GITHUB_WORKSPACE.
//
// It fails with ErrNoRepoRoot when none of them finds one.
func Root(ctx context.Context, dir string) (string, error) {
	root, err := gitRoot(ctx, dir)
	if err == nil {
		return root, nil
	}

	slog.Debug("git found no repository root", "dir", dir, "error", err)

	if root, ok := markedRoot(dir); ok {
		return root, nil
	}

	if workspace := os.Getenv("GITHUB_WORKSPACE"); workspace != "" && os.Getenv("GITHUB_ACTIONS") == "true" {
		return workspace, nil
	}

	return "", fmt.Errorf("%w: %s and the directories above it", releasechanges.ErrNoRepoRoot, dir)
}

func gitRoot(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", errors.New("git printed no root")
	}

	return filepath.FromSlash(root), nil
}

// markedRoot walks up from dir to the first directory holding a root marker,
// stopping at the filesystem root.
func markedRoot(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}

	for {
		for _, marker := range rootMarkers {
			if _, err := os.Lstat(filepath.Join(dir, marker)); err == nil {
				return dir, true
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}

		dir = parent
	}
}

// ReadTemplates reads the template set entry is the entry point of: entry
// itself, whatever its extension, and every *.tmpl file next to it, keyed by
// file name. Subdirectories are not read.
//
// A missing entry is an error wrapping fs.ErrNotExist, so a caller can tell
// "no template here" apart from a template that cannot be read.
func ReadTemplates(entry string) (map[string]string, error) {
	text, err := os.ReadFile(entry)
	if err != nil {
		return nil, err
	}

	files := map[string]string{filepath.Base(entry): string(text)}

	dir := filepath.Dir(entry)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		name := e.Name()
		if _, seen := files[name]; seen || !strings.HasSuffix(name, TemplateExt) || e.IsDir() {
			continue
		}

		text, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}

		files[name] = string(text)
	}

	return files, nil
}
