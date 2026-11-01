---
title: The items JSON
linkTitle: The items JSON
weight: 3
---

> [!NOTE]
> Not built yet: the `items` command lands in v0.1. This is the contract it will keep.

`gh release-changes items <tag>` prints one JSON object. It is an object rather than an array so fields can be
added without breaking consumers. **Field names and group identifiers are public API: they may be added to,
never renamed.**

```json
{
  "repo": "owner/repo",
  "tag": "v3.7.0",
  "previous_tag": "v3.6.2",
  "items": [
    {
      "pr": 123, "title": "feat(6768,6747)!: …", "type": "feat", "scopes": ["6768", "6747"],
      "breaking": true, "issues": [6768, 6747], "author": "octocat", "bot": false,
      "url": "https://github.com/owner/repo/pull/123", "group": "features"
    }
  ]
}
```

## Groups

In display order:

| Identifier     | Name         | What lands there                                                            |
|----------------|--------------|-----------------------------------------------------------------------------|
| `features`     | Features     | `feat`                                                                      |
| `fixes`        | Bug fixes    | `fix`                                                                       |
| `other`        | Other        | `docs test ci refactor perf style build revert`                             |
| `maintenance`  | Maintenance  | `chore`                                                                     |
| `dependencies` | Dependencies | a `deps` or `deps-dev` scope, or a dependabot author — whatever the type    |
| `unclassified` | Unclassified | titles outside the `<type>[(<scopes>)][!]: <subject>` format, never dropped |

How a line becomes an item is described in [Parser]({{< ref "../architecture/parser.md" >}}).
