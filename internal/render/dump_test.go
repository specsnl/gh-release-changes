package render_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/render"
)

// dumpFixture is a small release built to exercise every part of the dump: a
// breaking feature, an issue referenced from two groups, an unreadable issue,
// features and fixes without an issue (one unreadable), and the kinds of
// change the dump leaves out.
func dumpFixture(t *testing.T) (changes.Release, changes.Details) {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("testdata", "dump", "release.txt"))
	if err != nil {
		t.Fatalf("reading the fixture release: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join("testdata", "dump", "details.json"))
	if err != nil {
		t.Fatalf("reading the fixture details: %v", err)
	}

	var details changes.Details
	if err := json.Unmarshal(raw, &details); err != nil {
		t.Fatalf("decoding the fixture details: %v", err)
	}

	release := changes.Release{
		Repo: "example-org/app", Tag: "v2.1.0", PreviousTag: "v2.0.0", Items: changes.Parse(string(body)),
	}

	return release, details
}

func TestDump_Golden(t *testing.T) {
	release, details := dumpFixture(t)

	got := render.Dump(release, details, render.IssueRefs{IssueRepo: release.Repo, RelativeTo: release.Repo})

	assertGolden(t, "dump/release.golden", got)
}

func TestDump_IsDeterministic(t *testing.T) {
	release, details := dumpFixture(t)
	refs := render.IssueRefs{IssueRepo: release.Repo, RelativeTo: release.Repo}

	if first, second := render.Dump(release, details, refs), render.Dump(release, details, refs); first != second {
		t.Error("the same release rendered two different dumps")
	}
}

// Issues from another repository are referenced in full, so the dump says where
// they live; the release's own pull requests stay short.
func TestDump_OtherIssueRepo(t *testing.T) {
	release, details := dumpFixture(t)

	got := render.Dump(release, details, render.IssueRefs{IssueRepo: "example-org/tracker", RelativeTo: release.Repo})

	for _, want := range []string{
		"* Issues from: example-org/tracker\n",
		"### Issue example-org/tracker#12: Exports are too slow for large lists\n",
		"### Issue example-org/tracker#15\n",
		"##### #40: feat(12)!: replace the export API\n",
		"##### example-org/lib#7: Stream CSV rows\n",
		"### Pull request #44: feat: add a dark theme\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the dump lacks %q", want)
		}
	}
}

func TestDump(t *testing.T) {
	for _, tc := range []struct {
		name    string
		release changes.Release
		details changes.Details
		want    string
	}{
		{
			name:    "nothing to describe",
			release: changes.Release{Repo: "o/app", Tag: "v1.0.0", Items: []changes.Item{{PR: 1, Type: "chore", Group: changes.GroupMaintenance}}},
			want: "# Release v1.0.0\n\n" +
				"* Repository: o/app\n" +
				"* Compared with: nothing (first release)\n" +
				"* Issues from: o/app\n\n" +
				"No issues or pull requests to describe.\n",
		},
		{
			name: "a fix(deps) is a dependency, not an untracked fix",
			release: changes.Release{Repo: "o/app", Tag: "v1.0.1", PreviousTag: "v1.0.0", Items: []changes.Item{
				{PR: 2, Type: "fix", Scopes: []string{"deps"}, Group: changes.GroupDependencies},
			}},
			want: "# Release v1.0.1\n\n" +
				"* Repository: o/app\n" +
				"* Compared with: v1.0.0\n" +
				"* Issues from: o/app\n\n" +
				"No issues or pull requests to describe.\n",
		},
		{
			name: "an untracked breaking feature",
			release: changes.Release{Repo: "o/app", Tag: "v2.0.0", PreviousTag: "v1.0.0", Items: []changes.Item{
				{PR: 3, Title: "feat!: drop v1", Type: "feat", Breaking: true, Author: "alice", URL: "https://github.com/o/app/pull/3", Group: changes.GroupFeatures},
			}},
			details: changes.Details{PullRequests: map[int]changes.PullRequest{3: {Number: 3, Body: "Gone.\r\n"}}},
			want: "# Release v2.0.0\n\n" +
				"* Repository: o/app\n" +
				"* Compared with: v1.0.0\n" +
				"* Issues from: o/app\n\n" +
				"## Features\n\n" +
				"### Pull request #3: feat!: drop v1\n\n" +
				"* Author: @alice\n" +
				"* URL: https://github.com/o/app/pull/3\n" +
				"* **Breaking change**\n\n" +
				"#### Description\n\n" +
				"~~~markdown\nGone.\n~~~\n",
		},
		{
			name: "a description with headings and fences of its own",
			release: changes.Release{Repo: "o/app", Tag: "v1.1.0", PreviousTag: "v1.0.0", Items: []changes.Item{
				{PR: 4, Title: "feat: add a config file", Type: "feat", Author: "bob", URL: "https://github.com/o/app/pull/4", Group: changes.GroupFeatures},
			}},
			details: changes.Details{PullRequests: map[int]changes.PullRequest{4: {
				Number: 4, Body: "## Example\n\n~~~~toml\nkey = 1\n~~~~\n\n```\ngh app\n```",
			}}},
			want: "# Release v1.1.0\n\n" +
				"* Repository: o/app\n" +
				"* Compared with: v1.0.0\n" +
				"* Issues from: o/app\n\n" +
				"## Features\n\n" +
				"### Pull request #4: feat: add a config file\n\n" +
				"* Author: @bob\n" +
				"* URL: https://github.com/o/app/pull/4\n\n" +
				"#### Description\n\n" +
				"~~~~~markdown\n" +
				"## Example\n\n~~~~toml\nkey = 1\n~~~~\n\n```\ngh app\n```\n" +
				"~~~~~\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refs := render.IssueRefs{IssueRepo: tc.release.Repo, RelativeTo: tc.release.Repo}

			if got := render.Dump(tc.release, tc.details, refs); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}
