---
title: Distribution
weight: 6
---

goreleaser builds linux, darwin and windows × amd64 and arm64 on every `v*` tag, into two archive sets.

The `Release` workflow sets `GORELEASER_CURRENT_TAG` to the pushed tag. A final is usually tagged on the commit
of its last release candidate, and with two tags on one commit goreleaser would otherwise release the RC again.

## The gh extension

Raw binaries named `gh-release-changes-<os>-<arch>[.exe]`, the names `gh extension install` looks for. **These
names are a contract with gh; do not change them.**

```sh
gh extension install specsnl/gh-release-changes
```

### Why a gh extension (unlike labelsync)

labelsync deliberately is not one: it manages its own token chain and runs in CI without gh. This tool is the
opposite case — read-mostly, used interactively and by agent skills on machines that already have gh — and
leaning on gh's authentication removes all token handling.

## Homebrew

`tar.gz` archives feed two casks in [`specsnl/homebrew-tap`](https://github.com/specsnl/homebrew-tap):
`gh-release-changes@rc` on every tag, `gh-release-changes` on stable tags only. The cask puts a standalone
`gh-release-changes` on `PATH`, which gh does not treat as an extension, so Homebrew users run
`gh-release-changes …` and the skills accept either form. The tap is written with the
`HOMEBREW_TAP_GITHUB_TOKEN` organisation secret.

## Version injection

`internal/cmd.Version` is set with `-ldflags -X` by `.goreleaser.yml` and the Taskfile's `build`. A test asserts
both files name that exact path, because renaming the variable would make every build silently report `dev`.

## This site

Hugo with the [Hextra](https://github.com/imfing/hextra) theme under `docs/`, as a Hugo module in its own
`docs/go.mod` so the root `go mod tidy -diff` never sees it. The `Deploy Documentation` workflow builds it on
pushes to `main` that touch `docs/`, and deploys it to GitHub Pages at <https://gh-release-changes.specs.dev/>.
`docs/design.md` sits outside `docs/content/` and is not published.
