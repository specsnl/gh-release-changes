---
title: Exit codes
weight: 4
---

Borrowed from labelsync, which borrowed them from `terraform plan -detailed-exitcode`. The outcome codes are
disjoint bits and combine; `1` stays exclusive.

| Code | Meaning                                                                                                           |
|------|-------------------------------------------------------------------------------------------------------------------|
| `0`  | Success                                                                                                           |
| `1`  | Error (unknown tag, auth failure, unrecoverable API error, interactive guard)                                     |
| `4`  | Rendered, but something was skipped — an issue `dump` could not read, or a title `issues --titles` could not read |

Test bits, not equality, when you branch on an outcome: `(( code & 4 ))`. Bit `2` is unassigned.

Unclassified items are **not** an outcome: they are rendered in the Unclassified group and warned about on
stderr, and the run still exits `0`. Enforcing the title format is the PR-title check's job, not the release
tool's.

On a failure, stderr ends in one `error: <message>` line. The kind behind it — `tag_not_found`, `forbidden` and
so on — is listed in [Error Handling]({{< ref "../architecture/error-handling.md" >}}).
