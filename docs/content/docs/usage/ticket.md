---
title: The release ticket
weight: 3
---

`ticket` creates the release ticket — an issue that lists the issues of a release — and updates it as release
candidates follow. The issue is rendered from a Go template the repository keeps next to its code.

```shell
gh release-changes [-R [HOST/]OWNER/REPO] [--previous-tag <tag> | --since-final[=false]] [--link closing]
                   ticket [<tag>] [--create-in [HOST/]OWNER/REPO]
                   [--template <path>] [--title <template>] [--label <name>]... [--var key=value]...
                   [--scope-issue-repo OWNER/REPO] [--relative-to OWNER/REPO] [--full-refs]
                   [--unlinked=false] [--tasks] [--titles]
                   [--dry-run | --yes] [--require-markers]
gh release-changes ticket init [--dir <path>] [--force] [--stdout]
```

Start with `ticket init`, edit the template it writes, and try it with `--dry-run`:

```shell
gh release-changes ticket init
gh release-changes ticket v1.4.0-rc.1 --create-in OWNER/tickets --var environment=PROD --dry-run
```

## What a run does

1. **Finds the template** — see [Where the template comes from](#where-the-template-comes-from) — and says
   which one on stderr: `info: template: .github/release-changes/ticket.md.tmpl (local)`.
2. **Reads the release.** The range starts at the previous final release, as with
   [`--since-final`]({{< ref "./commands.md#the-whole-release-cycle---since-final" >}}), so the ticket of
   `rc.2` lists the whole release and not the delta since `rc.1`. `--previous-tag`, or `--since-final=false` for
   GitHub's choice, overrides it.
3. **Renders the issue list** as [`issues`]({{< ref "./commands.md#issues" >}}) does, with the pull requests
   without an issue included (`--unlinked=false` leaves them out). `--tasks`, `--titles`, `--full-refs` and
   `--scope-issue-repo` work as there, and so does `--link closing`, which stays opt-in. References are written
   relative to the `--create-in` repository, so they resolve where the ticket is.
4. **Renders the title and the body** from the template.
5. **Looks for the ticket**: an issue in the `--create-in` repository (default: the source repository), open or
   closed, that carries every label and whose title names the release's version — `1.4.0` for `v1.4.0-rc.2`
   as for `v1.4.0`, so `1.4.0-rc.1` in a title matches but `1.4.01` does not. More than one fails with
   `ticket_ambiguous`, naming them.
6. **Shows the ticket and asks**, on a terminal. `--yes` writes without asking; without a terminal and without
   `--yes` the run fails with `interactive_required` and writes nothing. `--dry-run` prints the title, the labels
   and the body on stdout and writes nothing.
7. **Creates the issue, or updates the one it found**, and prints its URL on stdout. An update rewrites the
   title and the generated part of the body, and leaves the labels alone. A ticket that would not change is not
   written.

Adding the issue to a project board stays manual.

## The generated part

An update rewrites only what is between the markers, and keeps the rest of the body as it is on GitHub:

```markdown
# Release 1.4.0

<!-- gh-release-changes:start -->
The issue list and whatever else the template renders here.
<!-- gh-release-changes:end -->

## Checklist

- [x] Tested on ACC
```

So ticked boxes and answers written in the ticket survive a run for `rc.2`; what the template renders outside
the markers is written once, when the ticket is created. A template without the markers is generated as a whole:
its output is wrapped in them. A template must render the markers once each, start first, or not at all. When
someone removed the markers from the ticket, the generated part is appended to it, with a warning.
`--require-markers` fails the run instead, with `ticket_markers_missing` and exit code `1`, and writes nothing —
with `--dry-run` too. Put the markers back around the part to regenerate, or run once without the flag.

## Where the template comes from

The template is `.github/release-changes/ticket.md.tmpl`. The first one found wins:

1. `--template <path>`, a local file of any name.
2. The local checkout: `.github/release-changes/ticket.md.tmpl` in the root of the repository the current
   directory is in. Uncommitted edits count, so a template can be tried with `--dry-run` before it is committed.
   Not with `-R`: its repository may be another one than the checkout.
3. The source repository's default branch, read through the API, so no checkout is needed.
4. The built-in default: a line naming the release and the range, and the issue list.

The root of the repository is what `git rev-parse --show-toplevel` answers, so worktrees, submodules, `GIT_DIR`
and `GIT_WORK_TREE` work; without git, the nearest directory above the current one that holds a `.git` or a
`.jj`; and in GitHub Actions, when neither finds one, `$GITHUB_WORKSPACE`. A nested `.github` is not a root.
Without a root, step 2 is skipped.

### Parts

Every `*.tmpl` file in the template's directory — not in its subdirectories — is a part, named by its file
name, that any other can use:

```text
{{ template "checklist.md.tmpl" . }}
{{ include "checklist.md.tmpl" . | indent 2 }}
```

`include` is Helm's: it returns the part as a string, so it can be piped. This works for every source: the
directory of `--template`, the local `.github/release-changes/`, and that directory on the default branch. A
part that does not parse fails the run with `template`, naming the file.

### Front matter

The template can start with YAML front matter, which sets the defaults for `--title` and `--label`:

```yaml
---
title: "WOO: release {{ .Version }} naar PROD"
labels:
  - release
---
```

| Setting | Flag                                     | Front matter key | Without either           |
|---------|------------------------------------------|------------------|--------------------------|
| Title   | `--title`                                | `title`          | `Release {{ .Version }}` |
| Labels  | `--label`, repeatable or comma-separated | `labels`         | `release`                |

The title is a template too, over the same data, and is folded onto one line. The labels are what the ticket is
created with and found by. An unknown front matter key fails the run.

## Template data

| Field          | Holds                                                                                                                    |
|----------------|--------------------------------------------------------------------------------------------------------------------------|
| `.Repo`        | The source repository, `OWNER/REPO`                                                                                      |
| `.Tag`         | The tag, `v1.4.0-rc.1`                                                                                                   |
| `.Version`     | The tag without a leading `v`, `1.4.0-rc.1`                                                                              |
| `.PreviousTag` | The base of the comparison, empty for a first release                                                                    |
| `.Issues`      | The issue list, as `issues` prints it                                                                                    |
| `.Items`       | The pull requests, as in [the items JSON]({{< ref "./items-json.md" >}}): `.PR`, `.Title`, `.Group`, `.Issues` and so on |
| `.Vars`        | The `--var key=value` pairs, for what only the repository knows                                                          |

`--var` is repeatable, and the last value of a key wins. Templates run with `missingkey=error`, so a misspelled
`{{ .Vars.enviroment }}` fails the run instead of rendering `<no value>`. Read an optional var with
`{{ index .Vars "cab" | default "n/a" }}`.

## Template functions

Go's own, `include`, and those of [Sprout](https://docs.atom.codes/sprout/registries/list-of-all-registries),
the maintained successor of Sprig: `default`, `trim`, `toUpper`, `join`, `date`, `semverCompare` and the rest.
Sprout renamed some of Sprig's functions (`upper` is `toUpper`); its function list is the reference. Every
registry is there except:

- `env` and `filesystem`: a template from a repository does not read environment variables or local files. In
  CI that could put a token in a public issue, and a local run would render differently from CI. Pass what the
  template needs with `--var`.
- `backward`, Sprig's compatibility shims, and `regexp`, which `regex` replaces.

## `ticket init`

Writes a starter template to `.github/release-changes/` in the repository root, found as above — not in the
current directory:

| File                | Shows                                                                                                                                                                           |
|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ticket.md.tmpl`    | Front matter, the marker block, every template data field, Sprout functions, a required and an optional `.Vars` key, and a comment that does not render with an example command |
| `checklist.md.tmpl` | A part `ticket.md.tmpl` includes, outside the markers                                                                                                                           |

| Flag           | Effect                                                                                                     |
|----------------|------------------------------------------------------------------------------------------------------------|
| `--force`      | Overwrite existing files; without it an existing file fails with `template_exists`, and nothing is written |
| `--dir <path>` | Write somewhere else — also the way out when no root is found (`no_repo_root`)                             |
| `--stdout`     | Print the files instead of writing them                                                                    |

`init` is a subcommand of `ticket`, so a tag literally named `init` is passed as `ticket -- init`.

## Access

Writing the ticket needs **Issues: write** on the `--create-in` repository, on top of the **Contents: write**
generating the notes needs on the source repository. A 403 says which one is missing; the kind is `forbidden`
either way. Reading the template from the default branch needs Contents: read, which the notes already imply.
