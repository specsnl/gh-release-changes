---
title: The items JSON
linkTitle: The items JSON
weight: 3
---

`gh release-changes items <tag>` prints one JSON object, indented with two spaces, to stdout. It is an object
rather than an array so fields can be added without breaking consumers. **Field names and group identifiers are
public API: they may be added to, never renamed.**

```json
{
  "repo": "owner/repo",
  "tag": "v3.7.0",
  "previous_tag": "v3.6.2",
  "items": [
    {
      "pr": 123,
      "title": "feat(6768,6747)!: …",
      "type": "feat",
      "scopes": ["6768", "6747"],
      "breaking": true,
      "issues": [6768, 6747],
      "author": "octocat",
      "bot": false,
      "url": "https://github.com/owner/repo/pull/123",
      "group": "features"
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

Items are listed in display order: by group as above, then by first issue number and PR number, with items
without an issue last; dependencies by PR number. `scopes` and `issues` are always arrays, never `null`. An
unclassified item has an empty `type`, empty `scopes` and `issues`, and `breaking: false`.

| Field          | Meaning                                                                                    |
|----------------|--------------------------------------------------------------------------------------------|
| `repo`         | The source repository as `OWNER/REPO`: `-R`, or the repository of the current directory    |
| `tag`          | The tag the notes were generated for                                                       |
| `previous_tag` | The base of the comparison: `--previous-tag`, or GitHub's choice; `""` for a first release |
| `items`        | The pull requests, in display order; `[]` when there are none                              |

Unclassified items are listed and reported with a warning on stderr naming their PRs; the run still exits `0`.

How a line becomes an item is described in [Parser]({{< ref "../architecture/parser.md" >}}).
