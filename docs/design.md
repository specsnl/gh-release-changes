# Release Changes — Design Plan

> **Repo:** `specsnl/gh-release-changes` · **module:** `github.com/specsnl/gh-release-changes` · **binary:**
> `gh-release-changes` · **docs:** <https://gh-release-changes.specs.dev/>
> Distributed as a `gh` CLI extension (`gh release-changes …`) and as a standalone binary through the Homebrew
> tap — see [Distribution](#distribution).

A Go CLI that reads GitHub's generated notes for a release and renders the same set of changes for each audience
that needs them: a grouped "What's Changed" list, an issue list for the release ticket, and an issue dump for an
LLM to write release notes from. It backs the `github-releases` plugin in `specsnl/agent-skills-public`.

Structure, conventions, and library choices deliberately mirror
[`specsnl/labelsync`](https://github.com/specsnl/labelsync), which in turn mirrors
[`specsnl/specs-cli`](https://github.com/specsnl/specs-cli). Where this design departs from labelsync, it says
so and why.

The design was first discussed in `specsnl/agent-skills-public` (`docs/plans/gh-github-releases.md` on the
`plan-gh-release-changes` branch); this document supersedes it.

---

## Goals

- One extraction step, renderers per audience: every output is derived from the same parsed items, so the
  grouped list, the issue list and the dump can never disagree.
- Deterministic where the work is mechanical (classify, sort, deduplicate); an LLM only for the prose, and only
  in the plugin, never in the CLI.
- Work for any tag: final, release candidate, or patch.
- No token handling of its own: authentication, `GH_TOKEN`, and GHES hosts all come from gh through `go-gh`.
- Scriptable: stable JSON for automation, stable Markdown for skills to print unchanged, meaningful exit codes,
  no prompts without a TTY.

## Non-goals

- Writing prose. The Dutch release notes and changelog are the plugin's job; the CLI hands it the facts.
- Editing releases. The release body is never read and never written.
- Supporting PR-title conventions other than the one the target repos enforce (see
  [Parser rules](#parser-rules)). No keyword fallback for free-form titles.

---

## Source material

- specsnl/agent-skills-public#9 (`releases` branch): a plugin with `/releasenotes` and `/changelog` skills
  that write Dutch release notes and a changelog, backed by `scripts/get-release-issues.sh`. Reuse its writing
  instructions (`release-notes-instructions.md`, `changelog-instructions.md`) and the issue-dump behaviour of
  the script.
- specsnl/agent-skills-public#7: a personal `github-minvws-release-sorter` skill (Python, hardcoded PAT) that
  groups a release's "What's Changed" list and prints the issue numbers per group. Reuse its grouping and
  sorting rules.

Both are source material only and are not updated; specsnl/agent-skills-public#11 supersedes them.

## Problem

After tagging a release, the same set of changes is written up for several audiences:

| Artifact                      | Audience                 | Nature                                 |
|-------------------------------|--------------------------|----------------------------------------|
| Grouped "What's Changed"      | developers / ops         | Deterministic: classify and sort       |
| Issue list for release ticket | PO / QA / acceptance     | Deterministic: extract and deduplicate |
| Dutch changelog               | quick overview           | Judgement: summarise and filter (LLM)  |
| Dutch release notes           | end users / stakeholders | Judgement: translate user impact (LLM) |

All four come from one source.

## Facts about the current process

- A tag is created and GitHub's "auto generate release notes" fills the release body. That generated body
  is the base; versions for other target groups are derived from it.
- PR titles are enforced with `amannn/action-semantic-pull-request@v6`, with scopes `e2e` and `\d+`
  (issue number). Multiple scopes are comma-separated (`feat(6768,6747):`).
- Titles outside the format still get merged (`fixer-improvement` in `v3.8.0` of that repo), so the parser has
  to handle them.
- The release ticket is a GitHub issue, possibly in a different repo. It is created by hand for now;
  automating it is a likely next step.
- Release notes are written for final tags, but it must also work for RC and patch/fix tags.

---

## Decisions

- **Source of truth:** the `generate-notes` API, never the (editable) release body:
  `POST /repos/{owner}/{repo}/releases/generate-notes` with `tag_name` and optional `previous_tag_name`.
- **Tooling:** a dedicated Go CLI in its own repo, built as a gh extension and released with goreleaser. The
  plugin ships no scripts of its own.
- **Plugin name:** `github-releases` (the `gh-` prefix belongs to the CLI repo, not the plugin).
- **Skill names (verb-object):**

  | Skill                  | Claude                  | Copilot                                 | Purpose                               |
  |------------------------|-------------------------|-----------------------------------------|---------------------------------------|
  | `write-release-notes`  | `/write-release-notes`  | `/github-releases:write-release-notes`  | Dutch release notes for end users     |
  | `write-changelog`      | `/write-changelog`      | `/github-releases:write-changelog`      | Dutch changelog                       |
  | `sort-release-changes` | `/sort-release-changes` | `/github-releases:sort-release-changes` | Grouped "What's Changed" + issue list |

- **Issue list format:** plain list per group. Digit scopes point at issues in the issue repo (`--issue-repo`,
  default: the source repo). A reference is short (`#N`) when the issue repo equals the ticket repo
  (`--ticket-repo`, default: the source repo) and full (`owner/repo#N`) otherwise; `--full-refs` forces the full
  form everywhere. Both flags take `OWNER/REPO` without a host — an issue reference has none — and the
  comparison ignores case, as GitHub does. The list has an `## Issues` heading and a `###` heading per group,
  mirroring `sort`, with one `* <ref>` line per issue.
- **Range:** any tag (RC, patch or final) as target. The base is GitHub's choice, which already fits the
  release flow (verified on `minvws/nl-rdo-woo-web-private`): a final is compared with the previous final, so it
  covers the whole RC cycle (`v3.8.0` → `v3.7.1`); a first RC with the previous final (`v3.8.0-rc.1` →
  `v3.7.1`); a later RC with the previous RC (`v3.8.0-rc.3` → `v3.8.0-rc.2`); a patch with the release before it
  (`v3.7.1` → `v3.7.0`). The CLI adds no logic of its own; `--previous-tag` overrides the base.
- **The tag must exist:** `generate-notes` does not fail on an unknown `tag_name` — it silently compares the
  default branch instead (`v9.9.9-doesnotexist` returned a full body) — so the CLI checks the tag (and
  `--previous-tag`, when given) first and fails with `ErrTagNotFound`. The previous tag GitHub chose is read from
  the body's `**Full Changelog**: …/compare/<previous>...<tag>` link.
- **Breaking changes in `sort`:** the line is prefixed with `**Breaking:**` and the title is left as written, so
  the `!` stays and GitHub's line format (`* <title> by @<author> in <pr-url>`) is kept.
- **Unclassified items do not change the exit code.** They are shown in the Unclassified group and reported with
  a warning on stderr; enforcing the title format is the PR-title check's job, not the release tool's.
- **Tag picker:** without a tag argument on an interactive terminal, the CLI offers the tags in semver order
  (newest first, 10 visible) with type-to-filter over all of them. Without a terminal a missing tag is
  `ErrInteractiveRequired`; skills always pass the tag.
- **`.github/release.yml`:** `generate-notes` reads it by default. PRs it excludes are considered not worth
  mentioning and stay out. Its categories only add `###` headings above the same `* … by @… in …` lines, and the
  parser ignores headings, so the CLI works with or without the file. The target repo has none today; a fixture
  with one is added when a repo that uses it comes along.

---

## CLI

```text
gh release-changes [-R owner/repo]              default: the repository of the current directory
                   [--previous-tag <tag>]       default: GitHub's choice
                   [--debug]                    slog diagnostics on stderr
│
├── items [<tag>]                               JSON contract, see below
├── sort [<tag>]                                grouped "What's Changed" Markdown
├── issues [<tag>]                              plain issue list per group
│     [--issue-repo owner/repo]                 default: the source repo
│     [--ticket-repo owner/repo]                default: the source repo
│     [--full-refs]
├── dump [<tag>]                                issue details for the writing skills
│     [--issue-repo owner/repo]
├── ticket [<tag>]                              (later) create/update the release issue
└── version
```

| Command  | Output                                                     | Used by                                  |
|----------|------------------------------------------------------------|------------------------------------------|
| `items`  | JSON: repo, tag, previous tag and the items                | skills, automation (contract)            |
| `sort`   | Grouped "What's Changed" Markdown                          | `sort-release-changes`                   |
| `issues` | Plain issue list per group                                 | `sort-release-changes`                   |
| `dump`   | Issue details, comments and linked PRs for the LLM         | `write-release-notes`, `write-changelog` |
| `ticket` | (later) create/update the release issue in the ticket repo | automation                               |

The product of every command goes to stdout; everything else — progress, warnings, errors — goes to stderr, so
`gh release-changes sort v1.2.0 > notes.md` captures only the notes.

### `items` JSON

An object, so fields can be added without breaking consumers. Field names and group identifiers are public API:
they may be added to, never renamed.

```json
{
  "repo": "owner/repo",
  "tag": "v3.7.0",
  "previous_tag": "v3.6.2",
  "items": [
    {
      "pr": 123, "title": "feat(6768,6747)!: …", "type": "feat", "scopes": ["6768", "6747"],
      "breaking": true, "issues": [6768, 6747], "author": "octocat", "bot": false,
      "url": "https://github.com/owner/repo/pull/123", "group": "features"
    }
  ]
}
```

Group identifiers: `features`, `fixes`, `other`, `maintenance`, `dependencies`, `unclassified`.

### Parser rules

Strict, matching the PR-title action:

- Line format: `* <title> by @<author> in <pr-url>`; ignore headings, "New Contributors" and the
  "Full Changelog" line. Authors include bots (`@dependabot[bot]` → author `dependabot[bot]`, `bot: true`);
  the PR URL may be on any host (GHES).
- Title format: `<type>[(<scopes>)][!]: <subject>`. `!` marks a breaking change (`breaking: true`).
- Types: the action defaults (`feat fix docs style refactor perf test build ci chore revert`).
- Groups and order: Features (`feat`), Bug fixes (`fix`), Other (`docs test ci refactor perf style build`,
  `revert`), Maintenance (`chore`), Dependencies (`deps`/`deps-dev` scope or dependabot author),
  Unclassified (titles that do not match the format, e.g. GitHub's `Revert "…"`; always shown, never dropped).
- Dependencies takes precedence over the type: `fix(deps): …` and `chore(deps): …` both land in Dependencies,
  not in Bug fixes or Maintenance. A dependabot PR is a dependency even when its title is outside the format.
- Issue numbers: digit scopes only (`\d+`; `e2e` and other scopes ignored), comma-separated.
  No `+`/`&`/`and` separators and no keyword fallback for titles without a type.
- Sorting: by first issue number, then PR number (no issue → bottom); Dependencies by PR number.
  Issue list: per group ascending, deduplicated.

### `dump`

Every referenced issue (title, body, labels, state, comments and `closedByPullRequestsReferences`), plus
`feat`/`fix` PRs without an issue, in one Markdown document, per group, with breaking changes marked. Fetched
through GraphQL, one aliased field per number and 25 numbers per query, so a release is one or two round trips.
An issue that cannot be read is noted in the output and reported on stderr; it does not abort the dump, and
the run exits `4`.

### Exit codes

Borrowed from labelsync, which borrowed them from `terraform plan -detailed-exitcode`. The outcome codes are
disjoint bits and combine; `1` stays exclusive.

| Code | Meaning                                                                           |
|------|-----------------------------------------------------------------------------------|
| `0`  | Success                                                                           |
| `1`  | Error (unknown tag, auth failure, unrecoverable API error, interactive guard)     |
| `4`  | Rendered, but something was skipped — an issue `dump` could not read, for example |

Unclassified items are not an outcome: they are rendered and warned about, and the run still exits `0` — see
[Decisions](#decisions).

Bit `2` is unassigned, so the numbers stay those of labelsync (where it means drift). `internal/util/exit` holds
the codes; a command returns an error and never calls `os.Exit`. An outcome that is not a failure travels on an
`*exit.Err` with a nil `Err` field, and `main` maps the error to a code with `exit.Of` and prints it — or nothing,
for a silent carrier — through `output.Writer`.

### Non-interactive guard

A prompt is only ever shown when stdin and stdout are terminals. Without one, a missing tag fails immediately
with `ErrInteractiveRequired` rather than hanging a pipeline or an agent's shell.

---

## Package structure

Mirrors labelsync.

```text
gh-release-changes/
├── main.go                       # cmd.Execute(), exit.Of
├── go.mod
└── internal/
    ├── releasechanges/           # ≈ labelsync's internal/labelsync
    │   └── errors.go             # sentinel errors + KindOf()
    ├── cmd/                      # one file per Cobra command
    │   ├── root.go
    │   ├── app.go                # App struct, persistent flags, the tag argument / picker
    │   ├── items.go
    │   ├── sort.go
    │   ├── issues.go
    │   ├── dump.go
    │   ├── notes.go              # hidden: the raw generated body, for debugging
    │   └── version.go
    ├── github/                   # the only package that talks to GitHub, through go-gh
    │   ├── client.go             # REST + GraphQL clients from go-gh, error mapping to sentinels
    │   ├── notes.go              # tag existence check, generate-notes
    │   ├── tags.go               # tag listing for the picker
    │   └── issues.go             # issue / PR details for dump
    ├── changes/                  # pure, no network
    │   ├── item.go               # Item, Group, Release — the items JSON contract
    │   ├── parse.go              # generated-notes body → []Item (the parser rules)
    │   └── sort.go               # grouping, ordering, issue deduplication
    ├── render/                   # pure: Release → sort Markdown, issue list, dump Markdown
    ├── semver/                   # pure: tag ordering for the picker
    └── util/
        ├── exit/                 # exit codes
        └── output/               # Writer for stderr diagnostics, slog setup, TTY detection
```

### Why `changes`, `render` and `semver` are isolated

None of them imports `internal/github`. They take plain structs (or a string body) and return plain structs or
text. Two consequences, the same as in labelsync:

1. Everything interesting — the parser rules, grouping, ordering, reference formatting, semver ordering — is
   testable with table-driven stdlib tests and golden files, with zero HTTP mocking.
2. A future "render from a saved `items` file" mode is a thin deserialisation shell rather than a refactor.

### Departures from labelsync

- **No own auth chain or HTTP client.** labelsync resolves a token itself and talks to GitHub through
  go-github; it only borrows `go-gh/pkg/auth`. This tool is a gh extension, so it uses go-gh's `api` clients
  end to end and never sees a token.
- **No `--output pretty|json`.** labelsync reports on a run; here the product *is* the output (Markdown or
  JSON by command), so there is no second rendering of it. Diagnostics still go through `output.Writer`.
- **No config file, XDG paths, cache, or rate limiter.** A run is a handful of requests.
- **A plain-text `output.Writer`.** One concrete type, no interface, no colour and no NDJSON mode: it writes
  `info: …`, `warning: …` and `error: …` lines on stderr. labelsync styles its diagnostics with lipgloss because
  it reports on a run; here the few lines around the product do not earn the dependency.

---

## Error handling

Sentinel errors live in `internal/releasechanges/errors.go` and are always wrapped with `%w`, exactly the
labelsync pattern:

```go
return fmt.Errorf("%w: %s", releasechanges.ErrTagNotFound, tag)
```

```go
func KindOf(err error) string // stable kind string, or "" when no known sentinel is wrapped
```

The kind strings are a public contract, for a future machine-readable error output and for skills that match
on them. They may be added to, never renamed. `main` prints the error once, as `error: <message>` on stderr;
Cobra's own error print and the usage block on runtime failures are silenced.

| Sentinel                 | Kind string            |
|--------------------------|------------------------|
| `ErrRepoNotFound`        | `repo_not_found`       |
| `ErrTagNotFound`         | `tag_not_found`        |
| `ErrForbidden`           | `forbidden`            |
| `ErrInteractiveRequired` | `interactive_required` |
| `ErrInvalidRepoRef`      | `invalid_repo_ref`     |

**Adding a sentinel means adding a row here, a `KindOf` case, and an entry in the `allSentinels` test table**,
which parses the package source for exported `Err*` variables so an untabled sentinel fails the build.

## Output

The command's product is written to stdout as-is, through `App.Stdout`. All other user-facing output goes
through `output.Writer` (`Info`, `Warn`, `Error`, `WriteErr`) on stderr. `log/slog` is a **debug-only diagnostic
channel on stderr**, silent unless `--debug`, and never used for reporting.

`cmd.App` holds both streams, the slog level, and the resolved persistent flags (`-R`, `--previous-tag`,
`--debug`). Every command is built with it, and the root's `PersistentPreRunE` rebuilds the streams over
`cmd.OutOrStdout()` / `cmd.ErrOrStderr()`, so a test drives the whole tree with buffers. `-R` is checked for
`[HOST/]OWNER/REPO` shape there (`ErrInvalidRepoRef`); whether the repository exists is GitHub's answer.

`output.IsTTY` answers the non-interactive guard with the isatty ioctl (`golang.org/x/term`), not the file mode:
`/dev/null` is a character device too.

---

## Authentication

gh's own: `gh auth login`, `GH_TOKEN`, and `GH_HOST` for GitHub Enterprise, all resolved by go-gh. The tool
never reads, stores, or prints a token.

`generate-notes` needs repository **Contents: write** (GitHub's fine-grained permissions table), even though it
creates nothing. Users with read-only access cannot use the tool on a repo, and a future CI token needs
`contents: write`. The 403 error (`ErrForbidden`) says so rather than only that the call was forbidden.

---

## Testing strategy

Stdlib `testing` only, as in labelsync.

| Package   | Approach                                                                                            |
|-----------|-----------------------------------------------------------------------------------------------------|
| `changes` | **The core suite.** Table-driven over every parser rule; golden `items` output over captured bodies |
| `render`  | Golden files for `sort`, `issues` (short, full, `--full-refs`), and `dump`                          |
| `semver`  | Table-driven ordering, pre-releases, non-semver tags                                                |
| `github`  | `net/http/httptest` fake: tag check, generate-notes, 403/404 mapping to sentinels, pagination       |
| `cmd`     | End to end against a fake `github` boundary: stdout/stderr split, exit codes, interactive guard     |

The captured `generate-notes` bodies in `internal/changes/testdata/` cover a final release, an RC, a patch
release, dependabot PRs, and a repo with a `.github/release.yml`. They are anonymised — repositories, people and
identifying titles — because the fixtures are published with the code.

Determinism deserves an explicit test: the same body parsed and rendered twice gives byte-identical output.

## Dependencies

| Package                   | Purpose                                     | Shared with labelsync             |
|---------------------------|---------------------------------------------|-----------------------------------|
| `github.com/spf13/cobra`  | Command tree                                | yes                               |
| `github.com/cli/go-gh/v2` | Repo resolution, REST/GraphQL clients, auth | yes (auth only there)             |
| `charm.land/huh/v2`       | The tag picker (filterable select)          | yes                               |
| `golang.org/x/term`       | TTY detection for the non-interactive guard | no (`charmbracelet/x/term` there) |

Anything beyond these is proposed in the issue that needs it.

---

## Distribution

goreleaser, linux/darwin/windows × amd64/arm64, two archive sets:

- `gh-extension` — raw binaries named `gh-release-changes-<os>-<arch>[.exe]`, the names `gh extension install`
  looks for. **These names are a contract with gh; do not change them.**
- `homebrew` — `tar.gz` archives feeding two casks in `specsnl/homebrew-tap/Casks`: `gh-release-changes@rc` on
  every tag, `gh-release-changes` on stable tags only (`skip_upload: auto`), same pattern as labelsync.

### Why a `gh` extension (unlike labelsync)

labelsync deliberately is not one: it manages its own token chain and runs in CI without gh. This tool is
the opposite case — it is read-mostly, used interactively and by agent skills on machines that already have
gh, and leaning on gh's auth removes all token handling. The Homebrew cask stays as a second channel; it puts a
standalone `gh-release-changes` on `PATH`, which gh does not treat as an extension, so brew users run
`gh-release-changes …` and the skills accept either form.

## Documentation site

A Hugo site on the Hextra theme under `docs/`, published to GitHub Pages at
<https://gh-release-changes.specs.dev/>, set up the same way as labelsync's: `docs/content/` is published
(usage and architecture pages), `docs/design.md` sits outside it and is not. Hextra (v0.13) is a Hugo module in its
own `docs/go.mod`; Hugo is pinned to the newest `hugomods/hugo` non-root image (0.165.0, behind Hugo's own latest)
in `compose.yml`, and the `Deploy Documentation` workflow uses the same version. Unlike labelsync's, the site
carries a single override, `layouts/_partials/custom/head-end.html`, for the specs.nl brand colour: the footer
line comes from `params.footer` plus `i18n/en.yaml`, and labelsync's layout CSS and `card.html` fix are not
needed on Hextra v0.13. The DNS record is managed in OpenTofu (specsnl/specsops-opentofu#82).

## Repository scaffolding

Done in the scaffold (2026-11-01): repo (public, topic `gh-extension`), Go module and cobra root with
`version`, Taskfile + Docker Compose, golangci-lint (`wsl_v5`, `whitespace`, `modernize`), markdownlint,
Dependabot, goreleaser, the `HOMEBREW_TAP_GITHUB_TOKEN` org secret, and rulesets (`main` only through a PR with
`Unit tests` and `Markdown gate` green; tags must start with `v`). `task checkall` and `task release:dry-run`
pass.

Learned along the way:

- `go.mod` says `go 1.26.5` on purpose: golangci-lint v2.12 is built with Go 1.26 and refuses a newer `go`
  directive. Do not bump it past what golangci-lint supports.
- GoReleaser v2.19 deprecated `homebrew_casks.hooks.*.install`; CI runs `goreleaser check`, which fails on
  deprecations. The cask hook uses `install_steps` (`on_macos` / `run` / `{{ .StagedPath }}`). Keep the
  `goreleaser` image in `compose.yml` on the same version CI resolves (`~> v2`), currently v2.19.0. The same
  deprecation was filed for the siblings: specsnl/labelsync#133, specsnl/specs-cli#166, specsnl/specsdeployd#7.
- The goreleaser container needs `HOME: /tmp`, otherwise git cannot write its `safe.directory` entry.
- Dependabot's `go_modules` run failed once with a 404 on GitHub's updater image: transient, not config.

---

## Milestones

Tracked as GitHub milestones; GitHub's native "blocked by" links give the order inside them.

| Milestone | Scope                                                                                      |
|-----------|--------------------------------------------------------------------------------------------|
| `v0.1`    | Foundation, fetch, parser, `items`, `sort`, `issues`, the docs site, release `v0.1.0-rc.1` |
| `v0.2`    | `dump` and the interactive tag picker                                                      |
| backlog   | `ticket`                                                                                   |

The `github-releases` plugin follows `dump`, in `specsnl/agent-skills-public` (specsnl/agent-skills-public#11).

### Build order

The critical path is **foundation → fetch + parser → `items` → `sort` / `issues` → release `v0.1.0-rc.1`**.
The parser, the foundation and the docs site need nothing and can start at once. The `generate-notes` spike (#7)
is answered in [Decisions](#decisions) and [Authentication](#authentication).

---

## Plugin design (`specsnl/agent-skills-public`)

Built in that repo, tracked in specsnl/agent-skills-public#11. A new plugin `plugins/github-releases`:

- `plugin.json`, `.claude-plugin/marketplace.json` entry and root `README.md` table row, all with the same
  description covering both sorting and writing.
- `skills/write-release-notes/` and `skills/write-changelog/`: run `gh release-changes dump <tag>`, then
  write the Dutch text following their instruction files (based on the specsnl/agent-skills-public#9
  instructions, cleaned up).
- `skills/sort-release-changes/`: run `gh release-changes sort` and `issues`, and show both blocks
  unchanged in separate code fences.
- Every skill checks that the CLI is available, as `gh release-changes` (gh extension) or `gh-release-changes`
  (Homebrew), and prints install instructions if neither is.
- `README.md`: Claude and Copilot install commands and invocations for every skill; requirements `gh` +
  `gh extension install specsnl/gh-release-changes`.
