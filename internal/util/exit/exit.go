// Package exit holds the process exit codes gh-release-changes returns, and the
// error type that carries one out of a command.
//
// The scheme is borrowed from labelsync, which borrowed it from `terraform plan
// -detailed-exitcode`: a run that rendered its product but had to leave
// something out exits non-zero without that meaning "the tool broke".
//
// # Outcomes combine; failure does not
//
// The outcome codes are disjoint bits that OR together, so a run that satisfies
// more than one outcome reports all of them rather than the most alarming one.
// Today there is one outcome, [Skipped]. Bit 2 is left unassigned so the numbers
// stay those of labelsync, where it means drift.
//
// [Error] is deliberately not part of that bit space. It is the classic Unix
// generic failure and it is exclusive: when a run fails, nothing about its
// product can be trusted, so a failure exits 1 and nothing else.
//
// The numbers are a public contract — scripts and skills branch on them — so
// bits may be added, never reassigned, and Error stays 1 forever.
package exit

import (
	"errors"
	"strconv"
)

// Code is a process exit code. The outcome codes are single bits and combine
// with |; see the package doc.
type Code int

const (
	// OK means the run succeeded and its product is complete. Unclassified items
	// do not change this: they are rendered and warned about, not skipped.
	OK Code = 0

	// Error means the run failed: an unknown tag, an authentication failure, an
	// unrecoverable API error, or the interactive guard. Nothing can be inferred
	// about the product from this code, which is why it is exclusive rather than
	// a bit combined with the outcomes below.
	Error Code = 1

	// Skipped means the product was rendered, but something had to be left out —
	// an issue `dump` could not read, for example. Those failures are reported on
	// stderr rather than aborting the run; this code is how the caller learns the
	// product is not the whole set.
	Skipped Code = 1 << 2
)

// Err carries a code out of a command, because RunE returns an error and
// nothing else.
//
// A nil Err field means silent: the command has already reported everything the
// user needs through the output.Writer, and there is no failure to print. A dump
// that skipped an unreadable issue is not an error — the dump is on stdout and
// the skip was warned about — but it still has to exit 4.
//
// An ordinary failure needs no carrier at all: [Of] maps every other error to
// [Error].
type Err struct {
	// Code is the code the process should exit with. Combine outcome bits with |.
	Code Code

	// Err is the underlying failure, or nil when the non-zero code reports an
	// outcome rather than a failure.
	Err error
}

// Error implements error. The message is only ever read by a caller that
// formats the carrier itself — main prints the wrapped error, or nothing.
func (e *Err) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}

	return "exit code " + e.Code.String()
}

// Unwrap exposes the underlying failure to errors.Is and errors.As, so putting
// an error in a carrier does not hide its sentinel from releasechanges.KindOf.
func (e *Err) Unwrap() error { return e.Err }

// Of returns the code the process should exit with: OK for a nil error, the
// carried code for an [Err], and Error for anything else.
//
// A carrier holding a real failure never reports success, even when it was
// built with Code left unset — a zero exit on a failed run is the one mistake
// here that a pipeline cannot detect.
func Of(err error) Code {
	if err == nil {
		return OK
	}

	var carrier *Err
	if errors.As(err, &carrier) && carrier.Code != OK {
		return carrier.Code
	}

	return Error
}

// String renders a code for a debug log or a test failure. A combined code
// renders as its sum, which is what the shell sees.
func (c Code) String() string { return strconv.Itoa(int(c)) }
