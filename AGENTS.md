# gh-release-changes

- **Binary:** `gh-release-changes` (run as `gh release-changes` when installed as a gh extension)
- **Module:** `github.com/specsnl/gh-release-changes`

A gh extension written in Go (cobra) that reads GitHub's generated release notes and renders the changes for
different audiences. Structure, conventions, and library choices deliberately mirror
[`specsnl/labelsync`](https://github.com/specsnl/labelsync). The design record is
[`docs/design.md`](docs/design.md); the work is tracked as GitHub issues in milestones.

## Commands

**Every command runs through [Task](https://taskfile.dev).** Never invoke `docker compose`, `go`, or `npx`
directly — the Taskfile wraps Docker Compose services that pin the Go, golangci-lint, goreleaser, and Node
versions. Running the underlying tool directly uses whatever happens to be installed locally, which is not what
CI runs.

Every task routed through `docker compose` removes the `volume-init` container it pulled in when it finishes;
the teardown in the output is not an error.

| Command                | What it does                                                                          |
|------------------------|---------------------------------------------------------------------------------------|
| `task checkall`        | Every check CI runs: tidy check, golangci-lint, tests, goreleaser check, markdownlint |
| `task tidy:check`      | `go mod tidy -diff` — fails if `go.mod`/`go.sum` are untidy                           |
| `task tidy`            | `go mod tidy`                                                                         |
| `task lint`            | `golangci-lint run`                                                                   |
| `task lint:fix`        | `golangci-lint run --fix`                                                             |
| `task test`            | The Go tests                                                                          |
| `task md:check`        | markdownlint over every Markdown file                                                 |
| `task md:fix`          | Align Markdown tables, then apply autofixable rules                                   |
| `task build`           | Build `./gh-release-changes` for this machine                                         |
| `task install`         | Build and install this checkout as the gh extension                                   |
| `task release:check`   | Validate `.goreleaser.yml`                                                            |
| `task release:dry-run` | Local goreleaser snapshot into `dist/`, no publishing                                 |
| `task docs:serve`      | Hugo dev server with live reload at <http://localhost:1313>                           |
| `task docs:preview`    | Build the static site and serve it over nginx at <http://localhost:8080>              |
| `task docs:build`      | Build the static site into `docs/public/`                                             |
| `task docs:mod:tidy`   | Tidy the Hextra Hugo module in `docs/go.mod`                                          |
| `task dc:shell`        | Shell into the Go builder                                                             |

Run `task checkall` before pushing; it mirrors CI.

## Workflow

- **Always work on a branch off `main`.** Implementing an issue starts with
  `git checkout -b <type>/<short-slug> main` — never commit to `main` directly. Changes reach `main` through a
  pull request.
- **One issue, one branch, one pull request.** Reference the issue in the PR description so it closes on merge.
- **"Implement milestone X" means "implement every remaining open issue in milestone X".** List them with
  `gh issue list --milestone "<X>" --state open`; closed issues are done and are not redone.
- **Work out the order before writing any code, and state it.** Read every open issue in full, then:
  1. **Blocked issues come after what blocks them.** Use GitHub's native "blocked by" links (and anything the
     issue body says). A cycle or an ambiguous dependency is a question for the user, not a guess.
  2. **A spike or research issue goes first**, and its result must be *merged* before the issues it informs are
     implemented. If the spike changes an issue's scope, say so rather than implementing the issue as written.
  3. **Otherwise, order for the smallest reviewable diffs** — foundations before what is built on them.
- **The rest of a milestone is delivered as a stack of pull requests** with the `gh-stack` skill (`gh stack`):
  one branch and PR per issue, each based on the branch below it, ordered bottom-to-top by the dependency order.
  Never collapse a milestone into a single branch or PR. `gh stack push` after each rebase or new layer.
- **Every branch in a stack passes `task checkall` on its own.**
- **Close the loop against the issue.** Before opening the PR, re-read the issue's Scope and Done-when
  sections, tick what the change satisfies, and say plainly in the PR what is left unticked and why.

## Conventions

- **Code changes require tests and docs.** Whenever code is added, removed, or updated, in the *same* change:
  - **Tests** — add or update `*_test.go` files covering the changed behaviour.
  - **Architecture docs** — update `docs/content/docs/architecture/` if the change affects package structure,
    data flow, error handling, or a documented design decision. Mandatory, even for "internal" fixes.
  - **User docs** — update `docs/content/docs/usage/` if the change affects anything user-facing: commands,
    flags, output formats, exit codes.
  - **README** — only when what the tool *is*, or how it is installed, changes.

  Pages that describe something not built yet say so; the issue that builds it rewrites them. Keep
  `docs/design.md` in step where a decision it records changes.

- **Tests default to stdlib `testing`.** Table-driven tests, golden files under `testdata/` (rewritten from the
  current output when a change is intended), and `net/http/httptest` for the GitHub client.

- **Sentinel errors are always wrapped with `%w`.** Every way a run can fail has a sentinel in
  `internal/releasechanges/errors.go`. A call site never returns a sentinel bare when it has context to add,
  and never renders one with `%v`:

  ```go
  return fmt.Errorf("%w: %s", releasechanges.ErrTagNotFound, tag)
  ```

  Kind strings are a public contract: added to, never renamed. **Adding a sentinel means adding a `KindOf`
  case, a row in the error table in [`docs/design.md`](docs/design.md#error-handling), and an entry in the
  `allSentinels` test table.**

- **The product goes to stdout; everything else goes through `output.Writer` on stderr.** Never
  `fmt.Println` for diagnostics. `log/slog` is a debug-only channel on stderr, silent without `--debug`.

- **`internal/changes`, `internal/render` and `internal/semver` never import `internal/github`.** They take
  plain structs and return plain structs or text. This keeps the parser and renderers testable without HTTP.

- **The `items` JSON fields and group identifiers are public API**: added to, never renamed.

- **Authentication goes through `go-gh`**; never read, store, or print tokens.

- **Asset names in `.goreleaser.yml` are a contract with `gh extension install`**: keep
  `gh-release-changes-<os>-<arch>`.

- **Do not bump the `go` directive in `go.mod` past what golangci-lint supports** (see
  [`docs/design.md`](docs/design.md#repository-scaffolding)).

## Package layout

```text
gh-release-changes/
├── main.go                       # cmd.Execute(), exit.Of
└── internal/
    ├── releasechanges/           # errors.go (sentinels + KindOf)
    ├── cmd/                      # one file per Cobra command; app.go holds App and the persistent flags
    ├── github/                   # go-gh clients: tag check, generate-notes, tags, issue details
    ├── changes/                  # Item/Release (the items contract), Parse(), grouping and sorting — pure
    ├── render/                   # sort Markdown, issue list, dump Markdown — pure
    ├── semver/                   # tag ordering for the picker — pure
    └── util/                     # exit/, output/
```

The full structure is in [`docs/design.md`](docs/design.md#package-structure). `cmd`, `releasechanges`,
`github`, `changes` and `util/` exist so far; the rest lands with the v0.1 issues.

## Documentation

`docs/design.md` is the *plan*: goals, decisions, CLI, package structure, milestones. It is not published.

`docs/` also holds a [Hugo](https://gohugo.io) site on the [Hextra](https://github.com/imfing/hextra) theme,
published at <https://gh-release-changes.specs.dev/>, set up like labelsync's: everything under `docs/content/`
is published, the theme is a Hugo module in its own `docs/go.mod` (so the root `go mod tidy -diff` never sees
it), and links between content pages go through `{{< ref >}}`; links to `design.md` use its absolute
github.com URL. As subsystems land, their behaviour moves from `design.md` into the architecture and usage
pages. The `Deploy Documentation` workflow publishes the site on pushes to `main` that touch `docs/`.

Keep the `hugomods/hugo` tag in `compose.yml` and `hugo-version` in `.github/workflows/docs.yml` on the same
version.
