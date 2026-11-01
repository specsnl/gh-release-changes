package output_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/output"
)

// SetupLogger installs a process-wide default logger, so every test here has to
// put it back. Restoring in TestMain as well keeps a failure inside one test
// from leaking a chatty logger into the rest of the package.
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func withRestoredLogger(t *testing.T) {
	t.Helper()

	previous := slog.Default()

	t.Cleanup(func() { slog.SetDefault(previous) })
}

func TestWriter_Levels(t *testing.T) {
	var stderr bytes.Buffer

	w := output.NewWriter(&stderr)
	w.Info("fetching notes for %s", "v1.2.0")
	w.Warn("%d unclassified items", 2)
	w.Error("it %s", "broke")
	w.WriteErr(fmt.Errorf("%w: v9.9.9", releasechanges.ErrTagNotFound))

	want := "info: fetching notes for v1.2.0\n" +
		"warning: 2 unclassified items\n" +
		"error: it broke\n" +
		"error: tag not found: v9.9.9\n"

	if got := stderr.String(); got != want {
		t.Errorf("stderr =\n%q\nwant\n%q", got, want)
	}
}

// A message that happens to contain a verb must come out as written: the
// arguments are formatted once, not twice.
func TestWriter_WriteErrDoesNotReformat(t *testing.T) {
	var stderr bytes.Buffer

	output.NewWriter(&stderr).WriteErr(errors.New("100%s done"))

	if got, want := stderr.String(), "error: 100%s done\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

// The rule the whole package is built around: a normal run says nothing on the
// diagnostic channel. Anything a user should see went through a Writer.
func TestSetupLogger_SilentWithoutDebug(t *testing.T) {
	withRestoredLogger(t)

	var stderr bytes.Buffer

	output.SetupLogger(&stderr, false)

	slog.Debug("resolving repository")
	slog.Info("fetched notes")
	slog.Warn("slow response")
	slog.Error("request failed")

	if stderr.Len() != 0 {
		t.Errorf("logger emitted records without --debug: %q", stderr.String())
	}
}

func TestSetupLogger_DebugEnables(t *testing.T) {
	withRestoredLogger(t)

	var stderr bytes.Buffer

	output.SetupLogger(&stderr, true)

	slog.Debug("fetched notes", "items", 3)

	got := stderr.String()
	if !strings.Contains(got, "fetched notes") || !strings.Contains(got, "items=3") {
		t.Errorf("--debug produced no complete record: %q", got)
	}
}

// Cobra parses persistent flags after the command tree is built, so the level
// has to stay adjustable once the logger is installed.
func TestSetupLogger_LevelVarFlipsLater(t *testing.T) {
	withRestoredLogger(t)

	var stderr bytes.Buffer

	level := output.SetupLogger(&stderr, false)

	slog.Debug("before")

	if stderr.Len() != 0 {
		t.Fatalf("logger was not silent before the flip: %q", stderr.String())
	}

	level.Set(slog.LevelDebug)
	slog.Debug("after")

	if !strings.Contains(stderr.String(), "after") {
		t.Errorf("level.Set(LevelDebug) did not enable the logger: %q", stderr.String())
	}
}

func TestLevelSilent_IsAboveEveryLevel(t *testing.T) {
	for _, level := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
		if output.LevelSilent <= level {
			t.Errorf("LevelSilent (%v) does not suppress %v", slog.Level(output.LevelSilent), level)
		}
	}
}

// Neither a buffer nor a regular file is a terminal. /dev/null is the case that
// matters: it is a character device, and a mode-based check would call it one.
func TestIsTTY_NotATerminal(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatalf("creating a file: %v", err)
	}

	t.Cleanup(func() { _ = file.Close() })

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}

	t.Cleanup(func() { _ = devNull.Close() })

	for name, stream := range map[string]any{
		"buffer":     &bytes.Buffer{},
		"nil":        nil,
		"file":       file,
		os.DevNull:   devNull,
		"string arg": "not a stream",
	} {
		if output.IsTTY(stream) {
			t.Errorf("IsTTY(%s) = true, want false", name)
		}
	}
}
