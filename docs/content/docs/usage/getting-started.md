---
title: Getting started
weight: 1
---

## 1. Install

As a gh extension (recommended):

```sh
gh extension install specsnl/gh-release-changes
gh release-changes version
```

Or with Homebrew, which puts a standalone `gh-release-changes` binary on your `PATH`:

```sh
brew install specsnl/tap/gh-release-changes
gh-release-changes version
```

`specsnl/tap/gh-release-changes@rc` tracks the newest tag, release candidates included. gh does not treat the
Homebrew binary as an extension, so Homebrew users run `gh-release-changes …` where these pages say
`gh release-changes …`.

## 2. Authenticate

Authentication is gh's own: `gh auth login`, `GH_TOKEN`, or `GH_HOST` for GitHub Enterprise. The tool never
reads, stores or prints a token.

Reading the generated notes goes through `POST /repos/{owner}/{repo}/releases/generate-notes`, which GitHub
guards with **Contents: write** even though it creates nothing. A user with read-only access to a repository
cannot use the tool on it.

## 3. Render a release

```sh
gh release-changes sort v1.2.0            # grouped "What's Changed" Markdown
gh release-changes issues v1.2.0          # plain issue list per group
gh release-changes items v1.2.0 | jq .    # the JSON every other output is rendered from
```

Run it inside a clone of the repository, or pass `-R owner/repo`. The base of the comparison is GitHub's
choice — see [Commands]({{< ref "./commands.md#the-range" >}}).
