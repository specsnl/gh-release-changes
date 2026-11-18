package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// The version is the product of the command, so it belongs on stdout and only
// there: `gh release-changes version > v.txt` must find it in the file, and
// nothing narrated beside it.
func TestVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"prettified by default", []string{"version"}, "gh-release-changes version " + cmd.Version + "\n"},
		{"bare with --dont-prettify", []string{"version", "--dont-prettify"}, cmd.Version + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, stdout, stderr, err := run(t, nil, tc.args...)
			if code := exit.Of(err); code != exit.OK {
				t.Fatalf("exit code = %s, want %s (error: %v)", code, exit.OK, err)
			}

			if stdout != tc.want {
				t.Errorf("stdout = %q, want %q", stdout, tc.want)
			}

			if stderr != "" {
				t.Errorf("version narrated on stderr: %q", stderr)
			}
		})
	}
}

// --version is defined as `version --dont-prettify`, so the test is that the two
// produce the same bytes rather than that each produces something plausible.
func TestVersion_RootFlagMatchesDontPrettify(t *testing.T) {
	_, viaFlag, flagStderr, err := run(t, nil, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}

	_, viaCmd, _, err := run(t, nil, "version", "--dont-prettify")
	if err != nil {
		t.Fatalf("version --dont-prettify: %v", err)
	}

	if viaFlag != viaCmd {
		t.Errorf("--version = %q, version --dont-prettify = %q; they must agree", viaFlag, viaCmd)
	}

	if flagStderr != "" {
		t.Errorf("--version narrated on stderr: %q", flagStderr)
	}
}

// --version answers for the binary only; a subcommand has no such flag.
func TestVersion_RootFlagIsNotPersistent(t *testing.T) {
	_, _, _, err := run(t, nil, "sort", "--version")
	if err == nil {
		t.Fatal("want an error for --version on a subcommand, got none")
	}
}

// Without the flag the root still prints its help, the way a bare
// `gh release-changes` did before the flag gave the root a RunE at all.
func TestRoot_BareInvocationPrintsHelp(t *testing.T) {
	_, stdout, _, err := run(t, nil)
	if err != nil {
		t.Fatalf("bare invocation: %v", err)
	}

	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("stdout = %q, want the help", stdout)
	}
}

// The persistent flags reach every leaf, version included.
func TestVersion_AcceptsPersistentFlags(t *testing.T) {
	_, stdout, _, err := run(t, nil, "-R", "specsnl/labelsync", "--debug", "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	if !strings.HasPrefix(stdout, "gh-release-changes version ") {
		t.Errorf("stdout = %q, want the version line", stdout)
	}
}

func TestVersion_RejectsArguments(t *testing.T) {
	_, stdout, _, err := run(t, nil, "version", "extra")
	if code := exit.Of(err); code != exit.Error {
		t.Errorf("exit code = %s, want %s", code, exit.Error)
	}

	if stdout != "" {
		t.Errorf("a rejected invocation produced a product: %q", stdout)
	}
}

// Nothing injects Version under `go test`, so the fallback is what a build
// without -ldflags ships as.
func TestVersion_DefaultsToDev(t *testing.T) {
	if cmd.Version != "dev" {
		t.Errorf("Version = %q, want %q", cmd.Version, "dev")
	}
}

// The build files name the variable by its full path, in a string the compiler
// never checks. Renaming or moving Version would leave them injecting into
// nothing, and every release would silently ship as "dev".
func TestVersion_BuildFilesInjectThisVariable(t *testing.T) {
	const want = "github.com/specsnl/gh-release-changes/internal/cmd.Version"

	for _, file := range []string{"../../.goreleaser.yml", "../../Taskfile.dist.yml"} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		if !strings.Contains(string(content), want) {
			t.Errorf("%s does not inject %s; a build from it would ship as %q", file, want, "dev")
		}
	}
}
