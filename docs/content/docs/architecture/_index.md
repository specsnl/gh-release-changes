---
title: Architecture
weight: 10
---

Internal design documents covering the architecture, key decisions and internals of `gh release-changes`. For
how to *drive* the tool, see [Usage]({{< ref "../usage" >}}).

These pages describe **what has been built**. The forward-looking plan — goals, decisions, the CLI, milestones —
lives in [design.md](https://github.com/specsnl/gh-release-changes/blob/main/docs/design.md). As each subsystem
lands, its behaviour moves from the design plan into this section.

| Page                                                      | Covers                                                                 |
|-----------------------------------------------------------|------------------------------------------------------------------------|
| [Overview]({{< ref "./overview.md" >}})                   | Package structure, the command tree, the data flow                     |
| [Error Handling]({{< ref "./error-handling.md" >}})       | Sentinel errors, the `%w` wrapping rule, and the kind strings          |
| [Output & Exit Codes]({{< ref "./output.md" >}})          | `App`, `output.Writer`, the `slog` boundary, TTY detection, exit codes |
| [Parser]({{< ref "./parser.md" >}})                       | From a `generate-notes` body to classified, sorted items, fixtures     |
| [Distribution]({{< ref "./distribution.md" >}})           | The release pipeline, the gh extension and the Homebrew casks          |
| [Library Decisions]({{< ref "./library-decisions.md" >}}) | Every direct dependency, and what it was chosen over                   |
