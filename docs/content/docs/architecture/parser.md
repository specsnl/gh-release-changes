---
title: Parser
weight: 4
---

> [!NOTE]
> Not built yet: `internal/changes` lands in v0.1. The rules below are the plan from
> [design.md](https://github.com/specsnl/gh-release-changes/blob/main/docs/design.md#parser-rules).

`internal/changes` turns a `generate-notes` body into items. It is pure: it never imports `internal/github`,
takes the body as a string, and returns plain structs.

## Rules

Strict, matching the `amannn/action-semantic-pull-request` check the target repositories enforce:

- Line format: `* <title> by @<author> in <pr-url>`. Headings, "New Contributors" and the "Full Changelog" line
  are ignored, so the `###` categories a `.github/release.yml` adds make no difference.
- Authors include bots: `@dependabot[bot]` → author `dependabot[bot]`, `bot: true`. The PR URL may be on any
  host (GHES); the PR number comes from the URL.
- Title format: `<type>[(<scopes>)][!]: <subject>`; `!` marks a breaking change.
- Types: `feat fix docs style refactor perf test build ci chore revert`.
- Groups, in order: Features, Bug fixes, Other, Maintenance, Dependencies, Unclassified. Dependencies takes
  precedence over the type. Titles outside the format are Unclassified — never dropped.
- Issue numbers: digit scopes only, comma-separated. No `+`/`&`/`and` separators and no keyword fallback.
- Sorting: by first issue number, then PR number, items without an issue last; Dependencies by PR number.
