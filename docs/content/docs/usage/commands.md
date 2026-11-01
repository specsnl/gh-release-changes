---
title: Commands
weight: 2
---

```text
gh release-changes [-R [HOST/]OWNER/REPO]      default: the repository of the current directory
                   [--previous-tag <tag>]      default: GitHub's choice
                   [--debug]                   slog diagnostics on stderr
│
├── items [<tag>]                              JSON contract
├── sort [<tag>]                               grouped "What's Changed" Markdown
├── issues [<tag>]                             plain issue list per group
│     [--issue-repo owner/repo]                default: the source repo
│     [--ticket-repo owner/repo]               default: the source repo
│     [--full-refs]
├── dump [<tag>]                               issue details for the writing skills
│     [--issue-repo owner/repo]
└── version
```

## Global flags

| Flag                   | Default                        | Meaning                                                     |
|------------------------|--------------------------------|-------------------------------------------------------------|
| `-R`, `--repo`         | the current directory's remote | The source repository, as `OWNER/REPO` or `HOST/OWNER/REPO` |
| `--previous-tag <tag>` | GitHub's choice                | Compare with this tag instead                               |
| `--debug`              | off                            | Write `slog` debug diagnostics to stderr                    |

A malformed `-R` fails before anything is fetched, with exit code `1` and the kind `invalid_repo_ref`.

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

## Access

Authentication is `gh`'s own: `gh auth login`, `GH_TOKEN`, and `GH_HOST` or `-R HOST/OWNER/REPO` for GitHub
Enterprise. Generating release notes needs **Contents: write** on the repository, even though nothing is
created; with read access only, a run fails with `forbidden`. A repository that does not exist, or that the
token cannot see, fails with `repo_not_found`.

## `version`

Prints `gh-release-changes version <version>` to stdout.

## `items`

```shell
gh release-changes [-R OWNER/REPO] [--previous-tag <tag>] items <tag>
```

Prints the changes of the release as one JSON object on stdout: the contract skills and automation build on.
The fields, the groups and the order are described in [The `items` JSON]({{< ref "./items-json.md" >}}).

Pull requests whose title is outside the `<type>[(<scopes>)][!]: <subject>` format land in the `unclassified`
group and are named in a warning on stderr. They do not change the exit code.

## `sort`

```shell
gh release-changes [-R OWNER/REPO] [--previous-tag <tag>] sort <tag>
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

The output is stable — the same release always renders the same bytes — because the `sort-release-changes`
skill prints it unchanged.

## `issues`

```shell
gh release-changes [-R OWNER/REPO] [--previous-tag <tag>] issues <tag>
                   [--issue-repo OWNER/REPO] [--ticket-repo OWNER/REPO] [--full-refs]
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

| Flag                       | Default               | Meaning                                  |
|----------------------------|-----------------------|------------------------------------------|
| `--issue-repo OWNER/REPO`  | the source repository | The repository the digit scopes point at |
| `--ticket-repo OWNER/REPO` | the source repository | The repository the list is pasted into   |
| `--full-refs`              | off                   | Write every reference as `OWNER/REPO#N`  |

A reference is short (`#N`) when the issue repository equals the ticket repository, compared without regard to
case, and full (`OWNER/REPO#N`) otherwise, so it links to the right issue wherever the list is pasted. Both
flags take `OWNER/REPO` without a host; anything else fails before anything is fetched, with exit code `1` and
the kind `invalid_repo_ref`.

## `notes` (hidden)

```shell
gh release-changes [-R OWNER/REPO] [--previous-tag <tag>] notes <tag>
```

A debugging aid, left out of `--help`: prints the body `generate-notes` returned for the tag, unparsed, to
stdout. With `--debug` the previous tag the comparison used is logged on stderr. It is not part of the
contract and may change or go away.

## `dump`

> [!NOTE]
> Not built yet: lands in v0.2, with the interactive tag picker.
