---
title: Guides
weight: 7
---

Recipes that put `gh release-changes` to work in a release. They build on
[the release flow]({{< ref "../usage/release-flow.md" >}}): a release branch, release candidates, and a final
tagged on the accepted candidate.

| Guide                                                                          | Covers                                                                  |
|--------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| [The release from your laptop]({{< ref "./from-your-laptop.md" >}})            | Every step by hand from a terminal: the tag picker, the prompt, aliases |
| [The release ticket from GitHub Actions]({{< ref "./ticket-in-actions.md" >}}) | A workflow that creates and updates the ticket on every pushed tag      |
| [Release notes with an agent]({{< ref "./release-notes-with-an-agent.md" >}})  | `dump` for an agent to write from, `check-notes` until the notes agree  |
| [The notes check in CI]({{< ref "./notes-check-in-ci.md" >}})                  | A pull request check that fails when the notes and the release differ   |

Every guide uses `--link closing` throughout: the closing references belong in all of a release's lists or in
none of them. Only on a terminal can the tag be left out to pick it; the guides for CI and agents always pass it.
