---
title: GitHub Client
weight: 5
---

`internal/github` is the only package that talks to GitHub. It goes through
[go-gh](https://github.com/cli/go-gh) end to end, so authentication, `GH_TOKEN`, `GH_HOST` and GitHub Enterprise
hosts are resolved the way `gh` resolves them, and the tool never sees a token.

| File        | Holds                                                                                |
|-------------|--------------------------------------------------------------------------------------|
| `client.go` | `Client`: repository resolution, the go-gh REST and GraphQL clients, error mapping   |
| `notes.go`  | `GenerateNotes`: the tag check, `generate-notes`, the previous tag from the response |
| `tags.go`   | `Tags`: every tag of the repository, for the tag picker, through GraphQL             |
| `issues.go` | `Details`: issues and pull requests for `dump`, through GraphQL                      |

## The repository

`-R` is parsed with go-gh's `repository.Parse`, so `OWNER/REPO` takes gh's default host and `HOST/OWNER/REPO`
an explicit one. Without `-R` the repository comes from the current directory's git remotes
(`repository.Current()`). The REST client is created for the repository's host, so a GitHub Enterprise `-R`
reaches that host's API.

## Generating the notes

`GenerateNotes(ctx, tag, previousTag)` makes up to four requests:

1. `GET /repos/{owner}/{repo}/git/ref/tags/{tag}` — the tag must exist. `generate-notes` does not fail on an
   unknown `tag_name`: it generates notes up to the default branch instead, so a typo would silently give the
   wrong range.
2. The same check for `previousTag`, when `--previous-tag` is given.
3. `POST /repos/{owner}/{repo}/releases/generate-notes` with `tag_name`, and `previous_tag_name` only when
   given — otherwise GitHub chooses the base.
4. Only after a failed tag check: `GET /repos/{owner}/{repo}`, to tell a missing repository from a missing tag,
   which are both a 404 on the ref lookup.

The release body is never read: it is editable, the generated notes are not.

The result is a `Notes{Body, PreviousTag}`. `PreviousTag` is `--previous-tag` when given, and otherwise read from
the `**Full Changelog**: …/compare/<previous>...<tag>` line GitHub ends the body with. A first release links to
`…/commits/<tag>` instead and has no previous tag (`""`).

## Tags

`Tags(ctx)` lists every tag through the repository's `refs(refPrefix: "refs/tags/")` connection, 100 per page,
following `endCursor` until `hasNextPage` is false. The tags come back in GitHub's order; `internal/semver` orders
them for the picker. A missing repository (`NOT_FOUND` on the `repository` field, or a `null` repository) is
`ErrRepoNotFound`; HTTP errors are mapped as below.

## Issue and pull request details

`Details(ctx, issueRepo, issues, pullRequests)` reads what `dump` describes in as few round trips as possible:
one GraphQL query per batch of 25 numbers, with one aliased field per number —

```graphql
query($issueOwner: String!, $issueName: String!, $prOwner: String!, $prName: String!) {
  issues: repository(owner: $issueOwner, name: $issueName) {
    i12: issue(number: 12) { ...issue }
  }
  pullRequests: repository(owner: $prOwner, name: $prName) {
    p44: pullRequest(number: 44) { ...pullRequest }
  }
}
```

— where an issue brings its labels, its first 100 comments and their total count, and the pull requests that
close it (`closedByPullRequestsReferences`, closed ones included). Issues come from `issueRepo`, pull requests
from the client's repository, both on the client's host. Only what a batch needs is declared: GraphQL rejects an
unused variable or fragment.

GitHub answers a number it cannot resolve with `null` under its alias and an error whose `path` names it
(`["issues", "i15"]`), and still answers the rest. Such a number lands in `UnreadableIssues` or
`UnreadablePullRequests` with GitHub's message; a missing issue repository makes every issue in it unreadable,
with the repository's message. Only a failure of the request as a whole is an error: an HTTP error (mapped as
below), or a GraphQL error without a path.

The result is a `changes.Details` of plain structs, so `internal/render` renders it without importing this
package.

## Error mapping

| Response                                  | Error                                      |
|-------------------------------------------|--------------------------------------------|
| 404 or 409 (empty repository) on a tag    | `ErrTagNotFound`, or `ErrRepoNotFound`     |
| 404 on `generate-notes` or the repository | `ErrRepoNotFound`                          |
| 403                                       | `ErrForbidden`: needs Contents: write      |
| 403 with `X-RateLimit-Remaining: 0`       | go-gh's error, unmapped — not a permission |
| anything else (5xx, network)              | go-gh's error, unmapped                    |

Sentinels are wrapped with `%w` and the tag or repository they are about; the raw response is logged with
`--debug`. See [Error Handling]({{< ref "./error-handling.md" >}}).

## The boundary for commands

Commands do not use `*github.Client` directly. `cmd.App` holds a `NewNotesSource` factory returning the
`NotesSource` interface — `GenerateNotes`, and `Repo` for the resolved `OWNER/REPO` — and a `NewDetailsSource`
factory returning the `DetailsSource` interface (`Details`), and a `NewTagSource` factory returning the
`TagSource` interface (`Tags`), so a command test swaps in fakes and runs without HTTP. The client itself is
tested against a `net/http/httptest` server: `NewWithOptions` takes go-gh `ClientOptions`, and a test passes a
token and a `Transport` that redirects every request to the server. The GraphQL fakes answer from the query the
way GitHub does — the details fake from its aliases, failing a test on an unused variable or fragment, and the
tags fake page by page.
