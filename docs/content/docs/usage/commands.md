---
title: Commands
weight: 2
---

```text
gh release-changes [-R [HOST/]OWNER/REPO]      default: the repository of the current directory
                   [--previous-tag <tag>]      default: GitHub's choice
                   [--since-final]             compare with the previous final release
                   [--debug]                   slog diagnostics on stderr
│
├── items [<tag>]                              JSON contract
├── sort [<tag>]                               grouped "What's Changed" Markdown
├── issues [<tag>]                             plain issue list per group
│     [--scope-issue-repo owner/repo]          default: the source repo
│     [--relative-to owner/repo]               default: the source repo
│     [--full-refs]
│     [--unlinked]                             also the pull requests without an issue
│     [--tasks]                                - [ ] #N instead of * #N
│     [--titles]                               * #N <title>, titles read from GitHub
├── dump [<tag>]                               issue details to write release notes from
│     [--scope-issue-repo owner/repo]
└── version
```

## Global flags

| Flag                   | Default                        | Meaning                                                     |
|------------------------|--------------------------------|-------------------------------------------------------------|
| `-R`, `--repo`         | the current directory's remote | The source repository, as `OWNER/REPO` or `HOST/OWNER/REPO` |
| `--previous-tag <tag>` | GitHub's choice                | Compare with this tag instead                               |
| `--since-final`        | off                            | Compare with the newest final release older than the tag    |
| `--debug`              | off                            | Write `slog` debug diagnostics to stderr                    |

A malformed `-R` fails before anything is fetched, with exit code `1` and an error along the lines of:

```text
error: invalid repository reference: want [HOST/]OWNER/REPO: -R "acme"
```

`--previous-tag` and `--since-final` exclude each other; passing both fails with exit code `1`.

## The tag

Every command that works on a release takes its tag as an argument. Leave it out on a terminal and the tool
offers the repository's tags to pick from:

- newest first by semantic version (`v1.10.0` before `v1.9.0`, `v2.0.0` before `v2.0.0-rc.2`), release
  candidates and other pre-releases included;
- tags that are not a semantic version after them;
- 10 at a time — type to filter over all of them, arrows to move, Enter to pick, Esc or Ctrl+C to cancel.

The picker is drawn on stderr, so `gh release-changes sort > notes.md` still captures only the notes. It is only
shown when stdin and stdout are terminals: without one — in a pipeline, a script or an agent's shell — a missing
tag fails at once with exit code `1` and an error along the lines of:

```text
error: operation requires an interactive terminal: pass the tag, as in `gh release-changes sort <tag>`,
or run it in a terminal to pick one
```

Scripts and agents should always pass the tag.

`--previous-tag` has no picker: GitHub's choice of base fits the release flow, and the flag takes a tag when you
want another one.

## The range

Any tag works as the target: final, release candidate or patch. The base is GitHub's choice, which fits a
release flow without logic of its own:

| Target        | Compared with |
|---------------|---------------|
| `v3.8.0`      | `v3.7.1`      |
| `v3.8.0-rc.1` | `v3.7.1`      |
| `v3.8.0-rc.3` | `v3.8.0-rc.2` |
| `v3.7.1`      | `v3.7.0`      |

A final is compared with the previous final, so it covers the whole release-candidate cycle. `--previous-tag`
overrides the base. Both tags must exist: `generate-notes` silently compares the default branch for an unknown
tag, so the tool checks them first and fails with `tag_not_found`.

### The whole release cycle: `--since-final`

A later release candidate is compared with the one before it, so its list holds only what changed since. A
release ticket covers the whole release, though: it is created at `rc.1` and updated as candidates follow, and
should not shrink to the delta. `--since-final` compares with the newest final release — a semantic version
without a pre-release — older than the tag:

| Target        | GitHub's choice | `--since-final` |
|---------------|-----------------|-----------------|
| `v3.8.0-rc.1` | `v3.7.1`        | `v3.7.1`        |
| `v3.8.0-rc.3` | `v3.8.0-rc.2`   | `v3.7.1`        |
| `v3.8.0`      | `v3.7.1`        | `v3.7.1`        |
| `v3.7.1`      | `v3.7.0`        | `v3.7.0`        |

So every candidate of a release lists the same range its final will. The tags are read from the repository and
ordered by semantic version; tags that are not a semantic version are ignored. When there is no older final
release — the first release, or a tag that is no semantic version — the base is GitHub's choice after all, and
an `info:` line on stderr says so. `--since-final` excludes `--previous-tag`, and works for every command.

## Access

Authentication is `gh`'s own: `gh auth login`, `GH_TOKEN`, and `GH_HOST` or `-R HOST/OWNER/REPO` for GitHub
Enterprise. Generating release notes needs **Contents: write** on the repository, even though nothing is
created; with read access only, a run fails with `forbidden`. A repository that does not exist, or that the
token cannot see, fails with `repo_not_found`.

## `version`

Prints `gh-release-changes version <version>` to stdout.

## `items`

```shell
gh release-changes [-R OWNER/REPO] [--previous-tag <tag> | --since-final] items [<tag>]
```

Prints the changes of the release as one JSON object on stdout: the contract skills and automation build on.
The fields, the groups and the order are described in [The `items` JSON]({{< ref "./items-json.md" >}}).

Pull requests whose title is outside the `<type>[(<scopes>)][!]: <subject>` format land in the `unclassified`
group and are named in a warning on stderr. They do not change the exit code.

## `sort`

```shell
gh release-changes [-R OWNER/REPO] [--previous-tag <tag> | --since-final] sort [<tag>]
```

Prints the grouped "What's Changed" list as Markdown on stdout, for developers and ops:

```markdown
## What's Changed

### Features

* **Breaking:** feat(12)!: drop the v1 API by @alice in https://github.com/owner/repo/pull/31
* feat(14): add an export by @bob in https://github.com/owner/repo/pull/30

### Bug fixes

* fix(13): keep the filter after a reload by @carol in https://github.com/owner/repo/pull/29
```

- One `###` heading per group that has items, in the order Features, Bug fixes, Other, Maintenance,
  Dependencies, Unclassified; empty groups are left out. A release without pull requests prints `No changes.`
- Lines keep GitHub's format, `* <title> by @<author> in <pr-url>`, so the links still render where the list is
  pasted. Within a group they are in the [items order]({{< ref "./items-json.md" >}}).
- A breaking change (a `!` before the colon) is prefixed with `**Breaking:**`; the title itself is unchanged.
- Unclassified items are named in a warning on stderr, as for `items`.

The output is stable — the same release always renders the same bytes — so it can be pasted into a release
description, or compared by a script, as is.

## `issues`

```shell
gh release-changes [-R OWNER/REPO] [--previous-tag <tag> | --since-final] issues [<tag>]
                   [--scope-issue-repo OWNER/REPO] [--relative-to OWNER/REPO] [--full-refs] [--unlinked]
                   [--tasks] [--titles]
```

Prints a plain issue list per group as Markdown on stdout, for the release ticket (PO, QA, acceptance):

```markdown
## Issues

### Features

* #14

### Bug fixes

* #12
* #13
```

The issues are the digit scopes of the titles: `fix(12,13): …` references `#12` and `#13`. Per group they are
listed ascending and deduplicated; groups without issues are left out, and a release without any prints
`No issues.`

| Flag                            | Default               | Meaning                                                                            |
|---------------------------------|-----------------------|------------------------------------------------------------------------------------|
| `--scope-issue-repo OWNER/REPO` | the source repository | The repository the digit scopes of the titles point at                             |
| `--relative-to OWNER/REPO`      | the source repository | The repository references are written relative to: the one the list is pasted into |
| `--full-refs`                   | off                   | Write every reference as `OWNER/REPO#N`                                            |
| `--unlinked`                    | off                   | Also list the pull requests without an issue                                       |
| `--tasks`                       | off                   | Write every line as a task: `- [ ] #N`                                             |
| `--titles`                      | off                   | Follow every issue reference with its title                                        |

A reference is short (`#N`) when the `--scope-issue-repo` repository equals the `--relative-to` repository,
compared without regard to case, and full (`OWNER/REPO#N`) otherwise, so it links to the right issue wherever
the list is pasted. Both flags take `OWNER/REPO` without a host; anything else fails before anything is fetched,
with exit code `1` and an error along the lines of:

```text
error: invalid repository reference: want [HOST/]OWNER/REPO: --scope-issue-repo "github.com/acme/web"
(an issue reference takes OWNER/REPO, without a host)
```

**Renamed flags.** `--scope-issue-repo` and `--relative-to` were called `--issue-repo` and `--ticket-repo` up
to v0.2. The old names are hidden from the help but still work, with a warning on stderr; they will be removed
in a later release ([#44](https://github.com/specsnl/gh-release-changes/issues/44)). Giving a flag under both
names is an error.

### Line formats

By default a line is a bullet with the bare reference, `* #12`: GitHub expands it with the issue's title and
state where the list is pasted. Two flags change the line for other places, and combine:

| Flags              | Line                | For                                                |
|--------------------|---------------------|----------------------------------------------------|
| —                  | `* #12`             | GitHub                                             |
| `--tasks`          | `- [ ] #12`         | A GitHub release ticket QA ticks issues off in     |
| `--titles`         | `* #12 <title>`     | Places that do not expand references: Zammad, mail |
| `--tasks --titles` | `- [ ] #12 <title>` | Both                                               |

- `--tasks` needs no extra request: GitHub renders a task-list reference with the issue's title and state too.
  With `--unlinked` the pull request lines become tasks as well.
- `--titles` reads the titles from the `--scope-issue-repo` repository, one GraphQL request per 25 issues, and
  only when an issue is referenced. The pull request lines of `--unlinked` already carry their title.
- An issue whose title cannot be read keeps its bare reference, is named in a warning on stderr, and the run
  exits `4`; the list is still printed.
- Grouping and ordering do not change.

### Pull requests without an issue

`--unlinked` adds a section after the issues, so nothing in the release stays invisible in the ticket:

```markdown
### Pull requests without an issue

* #1439 ci: update github actions and enable npm provenance
* #1543 feat: add a project metadata file
* #1565 Release v19.0.0
```

- It lists every pull request without a digit scope, by PR number, with its title as GitHub has it —
  features and fixes, but also Other, Maintenance and Unclassified changes.
- Dependency updates are left out: Dependabot bumps are noise in a ticket. There is no flag to bring them back;
  `sort` lists them.
- A pull request is in the source repository, so its reference is short when the `--relative-to` repository is
  the source repository and full otherwise — also when `--scope-issue-repo` points the issues elsewhere.
  `--full-refs` makes it full too.
- The section is left out when there is nothing to list, and without `--unlinked` the output is unchanged.
- Back-merges of a release branch (`chore: release/v3.8.0 >> main`) are Maintenance, so they are listed. Keep
  them out of the generated notes altogether with an `exclude` in the repository's `.github/release.yml`, which
  takes them out of every command's output, rather than out of this list alone.

## `notes` (hidden)

```shell
gh release-changes [-R OWNER/REPO] [--previous-tag <tag> | --since-final] notes [<tag>]
```

A debugging aid, left out of `--help`: prints the body `generate-notes` returned for the tag, unparsed, to
stdout. With `--debug` the previous tag the comparison used is logged on stderr. It is not part of the
contract and may change or go away.

## `dump`

```shell
gh release-changes [-R OWNER/REPO] [--previous-tag <tag> | --since-final] dump [<tag>]
                   [--scope-issue-repo OWNER/REPO]
```

Prints everything needed to write release notes or a changelog for the release as one Markdown document on
stdout:

- **Every referenced issue** — the digit scopes of the titles, read from `--scope-issue-repo` (default: the
  source repository): title, state, labels, URL, description, comments, and the pull requests that close it
  (GitHub's `closedByPullRequestsReferences`, merged or not, each marked `In this release: yes` or `no`), with
  the release's own pull requests for it. A release pull request whose title references the issue while GitHub
  does not link it — "Related to #12" is no closing keyword — is described under **Unlinked pull requests in
  this release**, with its description.
- **Every feature or fix without an issue** — `feat` and `fix` titles without a digit scope: title, author, URL,
  and the pull request's description. A `fix(deps)` is a dependency update, not a fix, and is left out.
- **Breaking changes** are marked: `* **Breaking change**` on the issue or pull request, and `**Breaking:**` on
  the release's pull request line, as in `sort`.

```markdown
# Release v2.1.0

* Repository: example-org/app
* Compared with: v2.0.0
* Issues from: example-org/app

## Features

### Issue #12: Exports are too slow for large lists

* State: closed
* Labels: bug, export
* URL: https://github.com/example-org/app/issues/12
* **Breaking change**
* Pull requests in this release:
  * **Breaking:** feat(12)!: replace the export API by @alice in https://github.com/example-org/app/pull/40
  * fix(12,13): keep the filter after a reload by @carol in https://github.com/example-org/app/pull/42

#### Description

~~~markdown
…
~~~

#### Comments

##### @bob

~~~markdown
…
~~~

#### Linked pull requests

##### #40: feat(12)!: replace the export API

* State: merged
* Author: @alice
* URL: https://github.com/example-org/app/pull/40
* In this release: yes

~~~markdown
…
~~~

#### Unlinked pull requests in this release

##### #42: fix(12,13): keep the filter after a reload

* Author: @carol
* URL: https://github.com/example-org/app/pull/42

~~~markdown
…
~~~

### Pull request #44: feat: add a dark theme

* Author: @carol
* URL: https://github.com/example-org/app/pull/44

#### Description

~~~markdown
…
~~~
```

- One `##` heading per group, in the order of `sort`. An issue appears once, under the first group that
  references it; the features and fixes without an issue follow the issues of their group. Maintenance,
  dependency and unclassified changes without an issue are left out.
- Descriptions and comments are GitHub's Markdown, with line endings normalised, so the same release always
  renders the same bytes. Each is quoted in a `~~~markdown` fence, so its own headings cannot be read as the
  dump's; the fence is longer than any run of `~` in the text, so nothing inside can close it. Only the first
  100 comments of an issue are shown; the dump says how many it left out.
- An issue or pull request that cannot be read — it does not exist, the number is a pull request rather than an
  issue, the token may not see it — is noted in place with GitHub's reason (`_Could not be read: …_`) and named
  in a warning on stderr. The rest of the dump is still printed, and the run exits `4` rather than `0`.
- `--scope-issue-repo` takes `OWNER/REPO` without a host, as for `issues`, where its old name `--issue-repo`
  also still works; issues from another repository are written as `OWNER/REPO#N`. A malformed value fails
  before anything is fetched, with exit code `1` and the same error as for `issues`.
