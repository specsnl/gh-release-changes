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

Planned, and added by the issue that needs them:

| Package             | Purpose                            | Shared with labelsync |
|---------------------|------------------------------------|-----------------------|
| `charm.land/huh/v2` | The tag picker (filterable select) | yes                   |

## Decisions

- **`golang.org/x/term` over `charmbracelet/x/term`.** Both wrap the same ioctl. go-gh already depends on
  `x/term`, so it costs nothing once go-gh lands, and nothing else here needs the charm one.
- **No lipgloss.** labelsync colours its diagnostics; here the product is the output, and a few stderr lines do
  not earn the dependency.
- **go-gh over go-github.** A gh extension should not see a token. go-gh's clients take gh's authentication and
  hosts end to end.
