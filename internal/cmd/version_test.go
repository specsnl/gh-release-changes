package cmd_test

import (
	"bytes"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/cmd"
)

func TestVersion(t *testing.T) {
	var out bytes.Buffer

	root := cmd.NewRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if got, want := out.String(), "gh-release-changes version dev\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
