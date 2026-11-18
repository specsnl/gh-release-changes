package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/checkout"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/render"
)

const ticketInitLong = `Write a starter ticket template to .github/release-changes/ in the root of
the repository the current directory is in — found with git, else as the
nearest directory above it with a .git or .jj — or to --dir:

  ticket.md.tmpl     the entry point: front matter with the title and labels,
                     the marker block, every template data field, and an
                     example command in a comment that does not render
  checklist.md.tmpl  a part ticket.md.tmpl includes

An existing file is left alone, and the run fails, unless --force is given.`

// newTicketInitCmd writes the starter template set.
func newTicketInitCmd(app *App) *cobra.Command {
	var (
		dir           string
		force, stdout bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter ticket template to .github/release-changes/",
		Long:  ticketInitLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			files := render.InitTicketFiles()

			names := slices.Sorted(func(yield func(string) bool) {
				for name := range files {
					if name != checkout.TemplateFile && !yield(name) {
						return
					}
				}
			})
			names = append([]string{checkout.TemplateFile}, names...)

			if stdout {
				for i, name := range names {
					if i > 0 {
						fmt.Fprintln(app.Stdout)
					}

					if _, err := fmt.Fprintf(app.Stdout, "==> %s <==\n%s", name, files[name]); err != nil {
						return err
					}
				}

				return nil
			}

			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			if dir == "" {
				root, err := checkout.Root(cmd.Context(), wd)
				if err != nil {
					return fmt.Errorf("%w; pass --dir to write the templates elsewhere", err)
				}

				dir = filepath.Join(root, filepath.FromSlash(checkout.TemplateDir))
			}

			paths := make([]string, len(names))
			for i, name := range names {
				paths[i] = filepath.Join(dir, name)
			}

			if !force {
				for _, p := range paths {
					_, err := os.Lstat(p)
					if err == nil {
						return fmt.Errorf("%w: %s (pass --force to overwrite it)", releasechanges.ErrTemplateExists, shortPath(wd, p))
					}

					if !errors.Is(err, fs.ErrNotExist) {
						return err
					}
				}
			}

			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}

			for i, p := range paths {
				if err := os.WriteFile(p, []byte(files[names[i]]), 0o644); err != nil {
					return err
				}

				app.Out.Info("wrote %s", shortPath(wd, p))
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&dir, "dir", "", "Write the templates to this directory instead (default: .github/release-changes/ in the repository root)")
	flags.BoolVar(&force, "force", false, "Overwrite existing files")
	flags.BoolVar(&stdout, "stdout", false, "Print the templates instead of writing them")
	cmd.MarkFlagsMutuallyExclusive("stdout", "dir")
	cmd.MarkFlagsMutuallyExclusive("stdout", "force")

	return cmd
}

// shortPath is p relative to wd when it is below it, else p as it is.
func shortPath(wd, p string) string {
	rel, err := filepath.Rel(wd, p)
	if err != nil || !filepath.IsLocal(rel) {
		return p
	}

	return rel
}
