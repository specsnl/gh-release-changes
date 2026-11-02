package cmd

import (
	"fmt"
	"slices"
	"strings"
)

// linkClosing is the --link source for the issues a pull request closes on
// GitHub.
const linkClosing = "closing"

// linkSourceNames are the values --link accepts.
var linkSourceNames = []string{linkClosing}

// linkSources is the value of --link: a comma-separated list, repeatable. An
// unknown source is rejected while the flags are parsed, like a malformed
// value of any other flag.
type linkSources struct {
	sources []string
}

func (l *linkSources) Set(value string) error {
	for source := range strings.SplitSeq(value, ",") {
		source = strings.TrimSpace(source)
		if !slices.Contains(linkSourceNames, source) {
			return fmt.Errorf("unknown source %q (want %s)", source, strings.Join(linkSourceNames, ", "))
		}

		if !slices.Contains(l.sources, source) {
			l.sources = append(l.sources, source)
		}
	}

	return nil
}

func (l *linkSources) String() string { return strings.Join(l.sources, ",") }

func (l *linkSources) Type() string { return "sources" }

func (l *linkSources) list() []string { return slices.Clone(l.sources) }
