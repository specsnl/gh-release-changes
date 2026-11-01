package output

import (
	"io"
	"log/slog"
	"os"
)

// LevelSilent is above every level slog defines, so a logger set to it emits
// nothing at all.
//
// It is the default, and that is the whole point of the boundary this package
// draws: slog is a diagnostic channel for someone debugging gh-release-changes,
// not a reporting channel for someone using it. Without a silent default, warn-
// and error-level diagnostics would leak onto stderr and interleave with the
// real output.
const LevelSilent = slog.LevelError + 1

// SetupLogger installs the process-wide default slog logger on stderr and
// returns the [slog.LevelVar] gating it. debug selects [slog.LevelDebug];
// otherwise the level is [LevelSilent] and nothing is written.
//
// Cobra parses persistent flags after the root command is built, so the level is
// returned rather than baked in: the App installs a silent logger at
// construction and replaces it from PersistentPreRunE once --debug is known.
//
// Stderr, never stdout: `gh release-changes items v1.2.0 | jq` must not have
// debug lines spliced into the JSON.
func SetupLogger(stderr io.Writer, debug bool) *slog.LevelVar {
	level := new(slog.LevelVar)
	level.Set(LevelSilent)

	if debug {
		level.Set(slog.LevelDebug)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: level})))

	return level
}

// SetupDefaultLogger installs the default slog logger on os.Stderr.
func SetupDefaultLogger(debug bool) *slog.LevelVar {
	return SetupLogger(os.Stderr, debug)
}
