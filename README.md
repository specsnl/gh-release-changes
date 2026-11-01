# gh-release-changes

A [gh](https://cli.github.com) extension that sorts and extracts the changes of a GitHub release for its
different audiences. It reads GitHub's generated release notes (`POST /repos/{owner}/{repo}/releases/generate-notes`)
and renders them as a grouped "What's Changed" list, an issue list for the release ticket, or an issue dump to
write release notes from.

It backs the `github-releases` plugin in
[specsnl/agent-skills-public](https://github.com/specsnl/agent-skills-public).

Documentation: <https://gh-release-changes.specs.dev/>

## Groups

Every change lands in one group. The identifiers appear in the `items` JSON and are public API: they may be
added to, never renamed.

| Identifier     | What lands there                                                            |
|----------------|-----------------------------------------------------------------------------|
| `features`     | `feat`                                                                      |
| `fixes`        | `fix`                                                                       |
| `other`        | `docs test ci refactor perf style build revert`                             |
| `maintenance`  | `chore`                                                                     |
| `dependencies` | a `deps` or `deps-dev` scope, or a dependabot author — whatever the type    |
| `unclassified` | titles outside the `<type>[(<scopes>)][!]: <subject>` format, never dropped |

## Install

As a gh extension (recommended):

```bash
gh extension install specsnl/gh-release-changes
gh release-changes version
```

Or with Homebrew, which puts a standalone `gh-release-changes` binary on your `PATH`:

```bash
brew install specsnl/tap/gh-release-changes
gh-release-changes version
```

`specsnl/tap/gh-release-changes@rc` tracks the newest tag, release candidates included.

Authentication is gh's own: `gh auth login`, `GH_TOKEN`, or `GH_HOST` for GitHub Enterprise.

## Usage

Run it inside a clone of the repository, or pass `-R owner/repo`:

```bash
gh release-changes sort v1.2.0            # grouped "What's Changed" Markdown
gh release-changes issues v1.2.0          # plain issue list per group
gh release-changes dump v1.2.0            # issue details to write release notes from
gh release-changes items v1.2.0 | jq .    # the JSON every other output is rendered from
```

Leave out the tag in a terminal to pick one from the repository's tags. The base of the comparison is GitHub's
choice; `--previous-tag <tag>` overrides it. Generating release notes
needs **Contents: write** on the repository. Every command and flag is described in
[Commands](https://gh-release-changes.specs.dev/docs/usage/commands/).

## Development

Everything runs in containers through [Task](https://taskfile.dev) and Docker Compose:

| Task                   | What it does                                        |
|------------------------|-----------------------------------------------------|
| `task build`           | Build `./gh-release-changes` for this machine       |
| `task install`         | Build and install this checkout as the gh extension |
| `task test`            | Run the Go tests                                    |
| `task lint`            | Run golangci-lint                                   |
| `task checkall`        | Run every check CI runs                             |
| `task release:dry-run` | Build a snapshot release into `dist/`               |
| `task docs:serve`      | Serve the docs site with live reload on port 1313   |
| `task docs:preview`    | Build the docs site and serve it on port 8080       |

## Releasing

Push a `v*` tag. The `Release` workflow runs goreleaser, which:

- creates the GitHub release, with GitHub's generated notes;
- uploads a raw `gh-release-changes-<os>-<arch>` binary per platform, the asset names `gh extension install`
  looks for, plus `tar.gz` archives and `checksums.txt`;
- commits `Casks/gh-release-changes@rc.rb` to [specsnl/homebrew-tap](https://github.com/specsnl/homebrew-tap)
  for every tag, and `Casks/gh-release-changes.rb` for stable tags only.

The tap is written with the `HOMEBREW_TAP_GITHUB_TOKEN` organisation secret, which this repository must have
access to.
