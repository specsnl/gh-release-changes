// Package output is the channel for everything a user reads that is not the
// product.
//
// The product of a command — Markdown, JSON — is written to stdout as-is by the
// command. Everything else — progress, warnings, failures — goes through a
// [Writer] on stderr, never fmt.Println, so `gh release-changes sort v1.2.0 >
// notes.md` captures only the notes.
//
// # slog is not this
//
// log/slog is a debug-only diagnostic channel on stderr, silent on a normal run
// and never used for reporting. See [SetupLogger]. If a user is supposed to see
// it, it is a Writer call; if it only helps someone debugging, it is slog.
//
// # Writes are best-effort
//
// Writer has no error channel, and reporting a failed write would itself need a
// working stream. Output calls therefore discard their error, which is why
// .golangci.yml excludes fmt.Fprint and fmt.Fprintln from errcheck.
package output

import (
	"fmt"
	"io"
	"os"
)

// Writer reports diagnostics on stderr, one line per message, each prefixed
// with its level.
//
// It is plain text with no colour and no JSON mode. labelsync has both, because
// it reports on a run; here the product is the output, and the few lines around
// it do not earn a styling dependency.
type Writer struct {
	stderr io.Writer
}

// NewWriter creates a Writer over the given stream. Commands get theirs from the
// App, which builds it over cmd.ErrOrStderr so a test can capture it.
func NewWriter(stderr io.Writer) *Writer {
	return &Writer{stderr: stderr}
}

// NewDefaultWriter creates a Writer over os.Stderr.
func NewDefaultWriter() *Writer {
	return NewWriter(os.Stderr)
}

// Info reports normal progress. Progress is narration, not the result the
// command exists to produce, so it must not land in the file when a user
// redirects stdout.
func (w *Writer) Info(format string, args ...any) {
	w.line("info", format, args...)
}

// Warn reports a recoverable problem — an unclassified item, an issue that
// could not be read.
func (w *Writer) Warn(format string, args ...any) {
	w.line("warning", format, args...)
}

// Error reports a failure.
func (w *Writer) Error(format string, args ...any) {
	w.line("error", format, args...)
}

// WriteErr renders err as an error-level message. The sentinel kind is not
// shown: it exists for machines, and the message already reads well.
func (w *Writer) WriteErr(err error) {
	w.Error("%v", err)
}

func (w *Writer) line(level, format string, args ...any) {
	fmt.Fprintln(w.stderr, level+": "+fmt.Sprintf(format, args...))
}
