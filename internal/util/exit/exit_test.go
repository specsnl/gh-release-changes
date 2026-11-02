package exit_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
	"github.com/specsnl/gh-release-changes/internal/util/exit"
)

// The codes are a public contract: scripts branch on the numbers, so a
// renumbering is a breaking change and has to fail here rather than in a
// pipeline. Each entry is spelled out literally on purpose — asserting
// exit.OK == exit.OK would test nothing.
func TestCodes_AreStable(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  exit.Code
		want exit.Code
	}{
		{"OK", exit.OK, 0},
		{"Error", exit.Error, 1},
		{"Mismatch", exit.Mismatch, 2},
		{"Skipped", exit.Skipped, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("exit.%s = %d, want %d", tc.name, tc.got, tc.want)
			}
		})
	}
}

// The outcome codes have to be single, non-overlapping bits or they cannot be
// OR'd, and none of them may reach Error, or a pipeline masking for failure
// would see one.
func TestOutcomeCodes_AreDisjointSingleBits(t *testing.T) {
	var union exit.Code

	for _, tc := range []struct {
		name string
		code exit.Code
	}{
		{"Mismatch", exit.Mismatch},
		{"Skipped", exit.Skipped},
	} {
		if tc.code&(tc.code-1) != 0 {
			t.Errorf("exit.%s = %d has more than one bit set", tc.name, tc.code)
		}

		if union&tc.code != 0 {
			t.Errorf("exit.%s = %d overlaps an earlier outcome code", tc.name, tc.code)
		}

		union |= tc.code
	}

	if union&exit.Error != 0 {
		t.Errorf("outcome codes %d overlap exit.Error", union)
	}
}

func TestOf(t *testing.T) {
	sentinel := fmt.Errorf("%w: v9.9.9", releasechanges.ErrTagNotFound)

	for _, tc := range []struct {
		name string
		err  error
		want exit.Code
	}{
		{"nil is success", nil, exit.OK},
		{"a plain error is a failure", errors.New("boom"), exit.Error},
		{"a wrapped sentinel is a failure", sentinel, exit.Error},
		{"a carrier reports its code", &exit.Err{Code: exit.Skipped}, exit.Skipped},
		{"outcomes combine", &exit.Err{Code: exit.Mismatch | exit.Skipped}, 6},
		{"a carrier with a failure", &exit.Err{Code: exit.Error, Err: sentinel}, exit.Error},
		// Left unset by mistake: a failed run must not exit zero, because that is
		// the one wrong answer a pipeline cannot detect.
		{"a carrier with no code but a failure", &exit.Err{Err: sentinel}, exit.Error},
		{"a wrapped carrier", fmt.Errorf("dumping: %w", &exit.Err{Code: exit.Skipped}), exit.Skipped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := exit.Of(tc.err); got != tc.want {
				t.Errorf("Of(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// A carrier must not hide the sentinel it wraps, or KindOf would come back empty
// for exactly the failures that carry a code.
func TestErr_UnwrapsToTheSentinel(t *testing.T) {
	err := error(&exit.Err{
		Code: exit.Error,
		Err:  fmt.Errorf("%w: v9.9.9", releasechanges.ErrTagNotFound),
	})

	if !errors.Is(err, releasechanges.ErrTagNotFound) {
		t.Error("errors.Is could not see through the carrier")
	}

	if kind := releasechanges.KindOf(err); kind != "tag_not_found" {
		t.Errorf("KindOf = %q, want %q", kind, "tag_not_found")
	}
}

// A silent carrier still has to satisfy error, because it travels as one. The
// message is a fallback for a caller that formats it directly; main prints
// nothing for this case.
func TestErr_Error(t *testing.T) {
	if got := (&exit.Err{Code: exit.Skipped}).Error(); got != "exit code 4" {
		t.Errorf("Error() = %q, want %q", got, "exit code 4")
	}

	wrapped := &exit.Err{Code: exit.Error, Err: errors.New("boom")}
	if got := wrapped.Error(); got != "boom" {
		t.Errorf("Error() = %q, want %q", got, "boom")
	}
}
