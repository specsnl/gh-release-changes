package cmd_test

import (
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/github"
)

// The deprecated names still set the value of the flag that replaced them, and
// say so on stderr.
func TestRenamedFlags_StillWork(t *testing.T) {
	notes := "* fix(12): x by @alice in https://github.com/o/app/pull/3\n"

	for _, tc := range []struct {
		name    string
		flags   []string
		want    string
		warning string
	}{
		{
			"--issue-repo", []string{"--issue-repo", "o/tracker"},
			"* o/tracker#12\n", "warning: --issue-repo is deprecated, use --scope-issue-repo\n",
		},
		{
			"--ticket-repo", []string{"--ticket-repo", "o/releases"},
			"* o/app#12\n", "warning: --ticket-repo is deprecated, use --relative-to\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeNotes{notes: github.Notes{Body: notes}}

			args := append([]string{"-R", "o/app", "issues", "v2"}, tc.flags...)

			_, stdout, stderr, err := runApp(t, appWith(fake), nil, args...)
			if err != nil {
				t.Fatalf("issues: %v", err)
			}

			if !strings.HasSuffix(stdout, tc.want) {
				t.Errorf("stdout = %q, want it to end in %q", stdout, tc.want)
			}

			if stderr != tc.warning {
				t.Errorf("stderr = %q, want %q", stderr, tc.warning)
			}
		})
	}
}

func TestRenamedFlags_DumpIssueRepo(t *testing.T) {
	app, _, details := dumpApp(t)

	_, _, stderr, _ := runApp(t, app, nil, "-R", "example-org/app", "dump", "v2.1.0", "--issue-repo", "example-org/tracker")

	if len(details.requests) == 0 || details.requests[0].issueRepo != "example-org/tracker" {
		t.Errorf("requests = %v, want issues from %q", details.requests, "example-org/tracker")
	}

	if want := "warning: --issue-repo is deprecated, use --scope-issue-repo\n"; !strings.HasPrefix(stderr, want) {
		t.Errorf("stderr = %q, want it to start with %q", stderr, want)
	}
}

// Both names of one flag is a conflict, not a silent pick of either value.
func TestRenamedFlags_BothNamesConflict(t *testing.T) {
	fake := fixtureNotes(t, "final")

	_, _, _, err := runApp(t, appWith(fake), nil,
		"issues", "v2", "--scope-issue-repo", "o/a", "--issue-repo", "o/b")
	if err == nil {
		t.Fatal("issues accepted --scope-issue-repo and --issue-repo together")
	}

	if fake.tag != "" {
		t.Error("something was fetched despite the conflict")
	}
}

// The help shows only the new names.
func TestRenamedFlags_HiddenFromHelp(t *testing.T) {
	for _, command := range []string{"issues", "dump"} {
		t.Run(command, func(t *testing.T) {
			_, stdout, _, err := runApp(t, appWith(&fakeNotes{}), nil, command, "--help")
			if err != nil {
				t.Fatalf("%s --help: %v", command, err)
			}

			if !strings.Contains(stdout, "--scope-issue-repo") {
				t.Errorf("help lacks --scope-issue-repo:\n%s", stdout)
			}

			for _, old := range []string{"--issue-repo", "--ticket-repo"} {
				if strings.Contains(stdout, old) {
					t.Errorf("help shows the deprecated %s:\n%s", old, stdout)
				}
			}
		})
	}
}
