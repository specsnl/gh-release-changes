---
title: Output & Exit Codes
weight: 3
---

## stdout is the product

A command writes its product — Markdown or JSON — to `App.Stdout` as-is. Everything else goes through
`output.Writer` on stderr:

| Method     | Line                   |
|------------|------------------------|
| `Info`     | `info: <message>`      |
| `Warn`     | `warning: <message>`   |
| `Error`    | `error: <message>`     |
| `WriteErr` | `error: <err.Error()>` |

The writer is one concrete type with no colour and no JSON mode. labelsync has both because it reports *on* a
run; here the product is the output, and the few lines around it do not earn a styling dependency. Writes are
best-effort: the writer has no error channel, which is why `.golangci.yml` excludes `fmt.Fprint` and
`fmt.Fprintln` from errcheck.

## The `slog` boundary

`log/slog` is a debug-only channel on stderr, silent unless `--debug`, and never used for reporting. The default
level is `output.LevelSilent`, above every level slog defines. If a user is supposed to see it, it is a Writer
call; if it only helps someone debugging, it is slog.

## `App`

`cmd.App` holds `Stdout`, the writer, the slog level, and the resolved persistent flags (`Repo`, `PreviousTag`,
`Debug`). Every command is built with it and closes over it. The root's `PersistentPreRunE` rebuilds the streams
over `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` — never `os.Stdout` / `os.Stderr` — so a test drives the whole
tree with buffers and asserts on the stdout/stderr split directly. `-R` is checked for shape there; whether the
repository exists is GitHub's answer.

## TTY detection

`output.IsTTY(stream)` backs the non-interactive guard: the tag picker is only shown when stdin and stdout are
terminals, and otherwise a missing tag fails with `ErrInteractiveRequired`. The picker itself draws on stderr
(`App.Stderr`) and reads `App.Stdin`, both rebuilt from the command's own streams like `Stdout`. It uses the isatty ioctl
(`golang.org/x/term`), not the file mode — `/dev/null` is a character device too.

## Exit codes

`internal/util/exit` holds the codes; see [Usage › Exit codes]({{< ref "../usage/exit-codes.md" >}}) for the
table.

- Commands never call `os.Exit`, which skips deferred cleanup. They return an error; `main` maps it with
  `exit.Of` — `0` for nil, the carried code for an `*exit.Err`, `1` for anything else.
- An outcome that is not a failure (`4`, skipped) travels on an `*exit.Err` with a nil `Err` field. `main` prints
  nothing for it: the command already warned about what it skipped.
- A carrier holding a real failure never exits `0`, even with its code left unset.
- `1` is exclusive and never combined with an outcome bit. Bit `2` is unassigned, so the numbers stay those of
  labelsync.
