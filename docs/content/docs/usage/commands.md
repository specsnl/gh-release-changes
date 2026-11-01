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
overrides the base. The tag must exist: `generate-notes` silently compares the default branch for an unknown
tag, so the tool checks it first and fails with `tag_not_found`.

## `version`

Prints `gh-release-changes version <version>` to stdout.

## `items`, `sort`, `issues`

> [!NOTE]
> Not built yet: these land in v0.1. The output formats are specified in
> [design.md](https://github.com/specsnl/gh-release-changes/blob/main/docs/design.md#cli).

## `dump`

> [!NOTE]
> Not built yet: lands in v0.2, with the interactive tag picker.
