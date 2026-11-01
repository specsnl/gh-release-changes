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

## `items`, `sort`, `issues`

> [!NOTE]
> Not built yet: these land in v0.1. The output formats are specified in
> [design.md](https://github.com/specsnl/gh-release-changes/blob/main/docs/design.md#cli).

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
