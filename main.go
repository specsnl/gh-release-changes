// Command gh-release-changes is a gh extension that sorts and extracts the
// changes of a GitHub release for its different audiences.
package main

import (
	"os"

	"github.com/specsnl/gh-release-changes/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
