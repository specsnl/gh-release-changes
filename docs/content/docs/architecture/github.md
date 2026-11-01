---
title: GitHub Client
weight: 5
---

`internal/github` is the only package that talks to GitHub. It goes through
[go-gh](https://github.com/cli/go-gh) end to end, so authentication, `GH_TOKEN`, `GH_HOST` and GitHub Enterprise
hosts are resolved the way `gh` resolves them, and the tool never sees a token.

| File        | Holds                                                                                |
|-------------|--------------------------------------------------------------------------------------|
| `client.go` | `Client`: repository resolution, the go-gh REST client, error mapping to sentinels   |
| `notes.go`  | `GenerateNotes`: the tag check, `generate-notes`, the previous tag from the response |

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
`NotesSource` interface — `GenerateNotes` alone — so a command test swaps in a fake and runs without HTTP. The
client itself is tested against a `net/http/httptest` server: `NewWithOptions` takes go-gh `ClientOptions`, and a
test passes a token and a `Transport` that redirects every request to the server.
