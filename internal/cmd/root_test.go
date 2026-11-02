package cmd_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// run builds the tree the way main does, points both streams at buffers, and
// executes. Every assertion below reads those buffers: if a test cannot see the
// output, the streams were taken from os.Stdout/os.Stderr and the wiring is
// wrong, which is exactly what this helper is here to catch.
func run(t *testing.T, extra []*cobra.Command, args ...string) (*cmd.App, string, string, error) {
	t.Helper()

	return runApp(t, cmd.NewApp(), extra, args...)
}

// runApp is run over an App the test already holds, for a leaf that closes over
// it. Every field the persistent flags own is overwritten in PersistentPreRunE.
func runApp(t *testing.T, app *cmd.App, extra []*cobra.Command, args ...string) (*cmd.App, string, string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	root := cmd.NewRootCmd(app)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(&bytes.Buffer{})
	root.SetArgs(args)

	for _, c := range extra {
		root.AddCommand(c)
	}

	err := root.Execute()

	return app, stdout.String(), stderr.String(), err
}

// noopCmd is a leaf that does nothing, so a test can exercise the persistent
// flags without depending on a real command's behaviour.
func noopCmd() *cobra.Command {
	return &cobra.Command{Use: "noop", RunE: func(*cobra.Command, []string) error { return nil }}
}

// failCmd is a leaf that fails with err, for the exit-code paths.
func failCmd(err error) *cobra.Command {
	return &cobra.Command{Use: "boom", RunE: func(*cobra.Command, []string) error { return err }}
}

func TestRoot_FlagDefaults(t *testing.T) {
	app, _, _, err := run(t, []*cobra.Command{noopCmd()}, "noop")
	if err != nil {
		t.Fatalf("noop: %v", err)
	}

	for _, tc := range []struct {
		name      string
		got, want any
	}{
		{"--repo", app.Repo, ""},
		{"--previous-tag", app.PreviousTag, ""},
		{"--since-final", app.SinceFinal, false},
		{"--debug", app.Debug, false},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestRoot_SinceFinalFlag(t *testing.T) {
	app, _, _, err := run(t, []*cobra.Command{noopCmd()}, "--since-final", "noop")
	if err != nil {
		t.Fatalf("noop: %v", err)
	}

	if !app.SinceFinal {
		t.Error("--since-final = false, want true")
	}
}

func TestRoot_FlagParsing(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		repo string
	}{
		{"long", []string{"--repo", "specsnl/labelsync", "--previous-tag", "v1.1.0", "--debug", "noop"}, "specsnl/labelsync"},
		{"short, with a host", []string{"-R", "github.example.com/specsnl/labelsync", "--previous-tag", "v1.1.0", "--debug", "noop"}, "github.example.com/specsnl/labelsync"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _, _, err := run(t, []*cobra.Command{noopCmd()}, tc.args...)
			if err != nil {
				t.Fatalf("noop: %v", err)
			}

			if app.Repo != tc.repo {
				t.Errorf("--repo = %q, want %q", app.Repo, tc.repo)
			}

			if app.PreviousTag != "v1.1.0" {
				t.Errorf("--previous-tag = %q, want %q", app.PreviousTag, "v1.1.0")
			}

			if !app.Debug {
				t.Error("--debug = false, want true")
			}
		})
	}
}

func TestRoot_RejectsMalformedRepo(t *testing.T) {
	for _, repo := range []string{"labelsync", "specsnl/", "/labelsync", "a/b/c/d", "specsnl/label sync"} {
		t.Run(repo, func(t *testing.T) {
			_, stdout, _, err := run(t, []*cobra.Command{noopCmd()}, "-R", repo, "noop")
			if !errors.Is(err, releasechanges.ErrInvalidRepoRef) {
				t.Fatalf("error = %v, want ErrInvalidRepoRef", err)
			}

			if code := exit.Of(err); code != exit.Error {
				t.Errorf("exit code = %s, want %s", code, exit.Error)
			}

			if stdout != "" {
				t.Errorf("a rejected flag produced a product: %q", stdout)
			}
		})
	}
}

// An error wrapping a sentinel keeps its identity all the way out of the tree,
// so main can map it to a code and a future error output to its kind.
func TestRoot_ErrorCarriesKindAndCode(t *testing.T) {
	failure := fmt.Errorf("%w: v9.9.9", releasechanges.ErrTagNotFound)

	_, _, _, err := run(t, []*cobra.Command{failCmd(failure)}, "boom")
	if !errors.Is(err, releasechanges.ErrTagNotFound) {
		t.Fatalf("error lost its sentinel: %v", err)
	}

	if code := exit.Of(err); code != exit.Error {
		t.Errorf("exit code = %s, want %s", code, exit.Error)
	}

	if kind := releasechanges.KindOf(err); kind != "tag_not_found" {
		t.Errorf("kind = %q, want %q", kind, "tag_not_found")
	}
}

// An outcome is not a failure: the code travels, the error does not.
func TestRoot_OutcomeCodeTravelsOnASilentCarrier(t *testing.T) {
	_, _, _, err := run(t, []*cobra.Command{failCmd(&exit.Err{Code: exit.Skipped})}, "boom")

	if code := exit.Of(err); code != exit.Skipped {
		t.Errorf("exit code = %s, want %s", code, exit.Skipped)
	}

	var carrier *exit.Err
	if !errors.As(err, &carrier) || carrier.Err != nil {
		t.Errorf("carrier is not silent: %#v", carrier)
	}
}

// Without SilenceUsage every runtime failure drags the usage block along;
// without SilenceErrors Cobra prints its own copy next to main's.
func TestRoot_SilencesUsageAndErrors(t *testing.T) {
	_, stdout, stderr, err := run(t, []*cobra.Command{failCmd(errors.New("it broke"))}, "boom")
	if err == nil {
		t.Fatal("want an error, got none")
	}

	for name, got := range map[string]string{"stdout": stdout, "stderr": stderr} {
		if strings.Contains(got, "Usage:") {
			t.Errorf("%s carries a usage block after a runtime failure: %q", name, got)
		}

		if strings.Contains(got, "it broke") {
			t.Errorf("%s carries Cobra's own error line; main owns the single print: %q", name, got)
		}
	}
}

func TestRoot_UnknownCommandFails(t *testing.T) {
	_, _, _, err := run(t, nil, "bogus")
	if code := exit.Of(err); code != exit.Error {
		t.Errorf("exit code = %s, want %s", code, exit.Error)
	}
}

// slog is silent without --debug, and lands on the command's stderr with it —
// never on stdout, and never on os.Stderr where a test could not see it.
func TestRoot_DebugFlagGatesTheLogger(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantDebug bool
	}{
		{"silent by default", []string{"noop"}, false},
		{"debug on stderr", []string{"--debug", "noop"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logging := &cobra.Command{Use: "noop", RunE: func(*cobra.Command, []string) error {
				slog.Debug("hello from the command")

				return nil
			}}

			_, stdout, stderr, err := run(t, []*cobra.Command{logging}, tc.args...)
			if err != nil {
				t.Fatalf("noop: %v", err)
			}

			if got := strings.Contains(stderr, "hello from the command"); got != tc.wantDebug {
				t.Errorf("debug record on stderr = %v, want %v (stderr: %q)", got, tc.wantDebug, stderr)
			}

			if strings.Contains(stdout, "hello from the command") {
				t.Errorf("a debug record reached stdout: %q", stdout)
			}
		})
	}
}

// Diagnostics a command writes through app.Out land on the command's stderr,
// and its product on app.Stdout — the split every command relies on.
func TestRoot_StdoutStderrSplit(t *testing.T) {
	app := cmd.NewApp()

	split := &cobra.Command{Use: "split", RunE: func(*cobra.Command, []string) error {
		app.Out.Warn("narration")

		_, err := fmt.Fprintln(app.Stdout, "product")

		return err
	}}

	_, stdout, stderr, err := runApp(t, app, []*cobra.Command{split}, "split")
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	if stdout != "product\n" {
		t.Errorf("stdout = %q, want only the product", stdout)
	}

	if stderr != "warning: narration\n" {
		t.Errorf("stderr = %q, want only the narration", stderr)
	}
}
