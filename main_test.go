package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
	"github.com/specsnl/gh-release-changes/internal/util/output"
)

func TestReport(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"success prints nothing", nil, ""},
		{"a failure prints one error line", fmt.Errorf("%w: v9.9.9", releasechanges.ErrTagNotFound), "error: tag not found: v9.9.9\n"},
		{"a carrier prints what it carries", &exit.Err{Code: exit.Error, Err: errors.New("boom")}, "error: boom\n"},
		// An outcome is not a failure: an error line under a complete dump would
		// claim the run broke.
		{"a silent carrier prints nothing", &exit.Err{Code: exit.Skipped}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer

			report(output.NewWriter(&stderr), tc.err)

			if got := stderr.String(); got != tc.want {
				t.Errorf("stderr = %q, want %q", got, tc.want)
			}
		})
	}
}
