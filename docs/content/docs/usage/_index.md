---
title: Usage
weight: 5
---

How to drive `gh release-changes` from a terminal, a script, or an agent skill. These pages are the reference;
the reasoning behind the choices they describe lives in the [architecture section]({{< ref "../architecture" >}}).

```text
gh release-changes [flags] <command> [<tag>]
```

| Page                                                  | Covers                                                    |
|-------------------------------------------------------|-----------------------------------------------------------|
| [Getting started]({{< ref "./getting-started.md" >}}) | Install, authenticate, first run                          |
| [Commands]({{< ref "./commands.md" >}})               | Every command and flag                                    |
| [The `items` JSON]({{< ref "./items-json.md" >}})     | The JSON contract: fields, groups, and what is stable     |
| [Exit codes]({{< ref "./exit-codes.md" >}})           | `0`, `1` and the outcome bit `4`, and what each one means |

## Output, in one paragraph

**stdout is the product; stderr is everything else.** The Markdown or JSON a command produces goes to stdout
as-is; progress, warnings, errors and `--debug` diagnostics go to stderr, so
`gh release-changes sort v1.2.0 > notes.md` captures only the notes.
