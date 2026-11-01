// Command gh-release-changes is a gh extension that sorts and extracts the
// changes of a GitHub release for its different audiences.
package main

import (
	"errors"
	"os"

	"github.com/specsnl/gh-release-changes/internal/cmd"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
	"github.com/specsnl/gh-release-changes/internal/util/output"
)

// main is the only place that calls os.Exit. It skips deferred cleanup, so
// commands return an error and the code is derived from it here.
func main() {
	app := cmd.NewApp()

	err := cmd.Execute(app)

	report(app.Out, err)

	os.Exit(int(exit.Of(err)))
}

// report prints err through the Writer, unless there is nothing to print: a
// carrier with a nil Err field reports an outcome rather than a failure, and the
// command has already said what it skipped.
//
// It is a function rather than inline so that a test can drive it; the os.Exit
// above is the part that cannot be.
func report(w *output.Writer, err error) {
	if err == nil {
		return
	}

	var carrier *exit.Err
	if errors.As(err, &carrier) && carrier.Err == nil {
		return
	}

	w.WriteErr(err)
}
