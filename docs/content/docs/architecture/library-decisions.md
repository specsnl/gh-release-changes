---
title: Library Decisions
weight: 7
---

Every direct dependency, what it does, and what it was chosen over. Two rules decide most of it:

1. **Prefer what [`specsnl/labelsync`](https://github.com/specsnl/labelsync) already uses.** One set of idioms
   to know, one upgrade to do.
2. **Prefer the standard library.** `log/slog`, `net/http/httptest` and `testing` cover logging, the client
   tests and the whole suite. No test framework, no assertion library.

## The direct dependencies

| Package                   | Purpose                                     | Shared with labelsync             |
|---------------------------|---------------------------------------------|-----------------------------------|
| `github.com/spf13/cobra`  | The command tree                            | yes                               |
| `github.com/cli/go-gh/v2` | Repo resolution, REST/GraphQL clients, auth | yes (auth only there)             |
| `golang.org/x/term`       | TTY detection for the non-interactive guard | no — `charmbracelet/x/term` there |
| `charm.land/huh/v2`       | The tag picker (filterable select)          | yes                               |

## Decisions

- **`golang.org/x/term` over `charmbracelet/x/term`.** Both wrap the same ioctl. go-gh already depends on
  `x/term`, so it costs nothing once go-gh lands, and nothing else here needs the charm one.
- **huh over go-gh's `prompter`.** go-gh's prompter would cost nothing extra, but labelsync already prompts with
  huh, and its `Select` filters and scrolls a fixed height out of the box. huh pulls in bubbletea and lipgloss as
  transitive dependencies; nothing here uses them directly.
- **No lipgloss.** labelsync colours its diagnostics; here the product is the output, and a few stderr lines do
  not earn the dependency.
- **go-gh over go-github.** A gh extension should not see a token. go-gh's clients take gh's authentication and
  hosts end to end.
