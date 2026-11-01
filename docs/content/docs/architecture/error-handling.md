---
title: Error Handling
weight: 2
---

Every way a run can fail has a sentinel error in `internal/releasechanges/errors.go`. Sentinels are always
wrapped with `%w`, and each maps to a stable kind string.

## The wrapping rule

A call site with context to add never returns a sentinel bare, and never renders one with `%v` or into a freshly
constructed error — either breaks both `errors.Is` matching and `KindOf`:

```go
return fmt.Errorf("%w: %s", releasechanges.ErrTagNotFound, tag)
```

```go
func KindOf(err error) string // stable kind string, or "" when no known sentinel is wrapped
```

The kind strings are a **public contract**, for a future machine-readable error output and for skills that
match on them. They may be added to, never renamed.

## Sentinels

| Sentinel                 | Kind string            | Raised when                                                                                     |
|--------------------------|------------------------|-------------------------------------------------------------------------------------------------|
| `ErrRepoNotFound`        | `repo_not_found`       | The repository does not exist or is not visible (a 404)                                         |
| `ErrTagNotFound`         | `tag_not_found`        | The tag (or `--previous-tag`) does not exist — checked before `generate-notes`, which would not |
| `ErrForbidden`           | `forbidden`            | GitHub answers 403, other than a rate limit; `generate-notes` needs Contents: write             |
| `ErrInteractiveRequired` | `interactive_required` | A prompt is needed, but stdin and stdout are not terminals                                      |
| `ErrInvalidRepoRef`      | `invalid_repo_ref`     | `-R` is not in `[HOST/]OWNER/REPO` form                                                         |

**Adding a sentinel means adding a `KindOf` case, a row here, and an entry in the `allSentinels` test table.**
That test parses the package source for exported `Err*` variables, so an untabled sentinel fails the build
rather than shipping without a kind.

How `internal/github` maps HTTP responses onto these is on the [GitHub Client]({{< ref "./github.md" >}}) page.

## Where errors are printed

Exactly once, by `main`, as `error: <message>` on stderr through `output.Writer`. The root command sets
`SilenceErrors` so Cobra does not print its own copy, and `SilenceUsage` so a runtime failure is not buried
under the usage block.
