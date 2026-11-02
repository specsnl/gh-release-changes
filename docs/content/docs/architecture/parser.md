---
title: Parser
weight: 4
---

`internal/changes` turns a `generate-notes` body into items. It is pure: it never imports `internal/github`,
takes the body as a string, and returns plain structs, so every rule below is covered by table tests and golden
files without HTTP.

| File       | Holds                                                                                          |
|------------|------------------------------------------------------------------------------------------------|
| `item.go`  | `Item`, `Release` (the [items JSON]({{< ref "../usage/items-json.md" >}})), `Group`, `Section` |
| `parse.go` | `Parse(body) []Item`: line format, title format, classification                                |
| `sort.go`  | `Sort(items)`, `Sections(items)` and `Section.Issues()`: display order, issue deduplication    |

## Lines

Only `* <title> by @<author> in <pr-url>` lines become items. Everything else is ignored:

- headings, including the `###` categories a `.github/release.yml` adds — the result is the same with or without
  the file;
- the `## New Contributors` section, whose lines look like list items;
- the `**Full Changelog**` line and the generated-by comment.

The title is matched greedily, so a title that contains `by @` itself still splits at the last one. The PR URL
may be on any host (GHES); the PR number comes from the URL. Lines are trimmed, so CRLF bodies parse too.
Authors include bots: `@dependabot[bot]` gives author `dependabot[bot]` and `bot: true` — any `[bot]` suffix
sets `bot`.

## Titles

Strict, matching the `amannn/action-semantic-pull-request` check the target repositories enforce:

- Format: `<type>[(<scopes>)][!]: <subject>`, with a lower-case type, a space after the colon, and a non-empty
  subject. `!` sets `breaking`. More spaces before the subject are allowed, as the check allows them:
  `fix(12):  subject` is a fix for `#12`, and its title is kept as written.
- Types: `feat fix docs style refactor perf test build ci chore revert`.
- Scopes are comma-separated and trimmed. **Issue numbers** are the digit scopes (`\d+`), in title order and
  deduplicated; `e2e` and every other scope are ignored. There are no `+`/`&`/`and` separators and no keyword
  fallback.
- A title outside the format — GitHub's `Revert "…"`, a free-form title, an unknown or capitalised type — is
  **Unclassified**, and nothing is taken from it: empty `type`, `scopes` and `issues`, `breaking: false`.

## Groups

| Group          | From                                                          |
|----------------|---------------------------------------------------------------|
| `features`     | `feat`                                                        |
| `fixes`        | `fix`                                                         |
| `other`        | `docs test ci refactor perf style build revert`               |
| `maintenance`  | `chore`                                                       |
| `dependencies` | a `deps` or `deps-dev` scope, or the author `dependabot[bot]` |
| `unclassified` | anything outside the title format                             |

**Dependencies takes precedence over the type**: `fix(deps): …` and `chore(deps): …` are dependencies, not bug
fixes or maintenance. A dependabot pull request is a dependency even when its title is outside the format
(`Bump lodash from … to …`); its `type` stays empty.

## Order

`Parse` returns the items sorted, and the order is total, so the same body always gives the same bytes:

1. By group, in the order of the table above.
2. Within a group, by first issue number, then PR number; items without an issue go to the bottom, by PR number.
3. Dependencies by PR number alone.

`Sections` returns the non-empty groups in that order, each with its items, for renderers that print a heading
per group. `Section.Issues` collects the issue numbers of a group's items, ascending and deduplicated, for the
issue list.

## Fixtures

Real `generate-notes` bodies under `internal/changes/testdata/*.txt`, each with a `.golden` items JSON beside it.
They are **anonymised**: repositories are `example-org/…`, people are pseudonyms, and titles that would identify
the source are reworded. Line structure, PR numbers, bot authors, package names and line endings are kept as
captured, so the bodies still exercise the parser the way real ones do. Rewrite the goldens after an intended
change with `go test ./internal/changes -update` (inside `task dc:shell`).

| Fixture       | Kind of release                      | Covers                                                          |
|---------------|--------------------------------------|-----------------------------------------------------------------|
| `final`       | final                                | digit scopes, unclassified titles, New Contributors, dependabot |
| `rc`          | release candidate                    | `build(deps)` from dependabot, free-form titles                 |
| `patch`       | patch                                | `deps` and `deps-dev` scopes, a fix with a named scope          |
| `dependabot`  | dependency updates only              | dependabot `deps-dev` updates                                   |
| `release-yml` | generated with `.github/release.yml` | `###` category headings, CRLF line endings                      |

A new fixture is anonymised the same way before it is committed.
