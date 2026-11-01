---
title: gh release-changes
layout: hextra-home
---

<!-- markdownlint-disable MD013 MD033 -->

<div style="margin-top: 1.5rem; margin-bottom: 1.5rem">
{{< hextra/hero-headline >}}
  One release, every audience
{{< /hextra/hero-headline >}}
</div>

<div style="margin-bottom: 3rem">
{{< hextra/hero-subtitle >}}
  `gh release-changes` reads GitHub's generated notes for a release and renders the same changes for everyone
  who needs them: a grouped "What's Changed", an issue list for the release ticket, and an issue dump to write
  release notes from.
{{< /hextra/hero-subtitle >}}
</div>

<div style="margin-bottom: 4rem">
{{< hextra/hero-button text="Get Started" link="docs/usage/getting-started" >}}
{{< hextra/hero-button text="GitHub" link="https://github.com/specsnl/gh-release-changes" >}}
</div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="One Source of Truth"
    subtitle="Every output comes from the same parsed items, read from the `generate-notes` API — never from the editable release body."
  >}}
  {{< hextra/feature-card
    title="Deterministic"
    subtitle="Classifying, sorting and deduplicating are mechanical, so the CLI does them the same way every time. Prose is left to the skills."
  >}}
  {{< hextra/feature-card
    title="Any Tag"
    subtitle="Final, release candidate or patch: the range is GitHub's choice, and `--previous-tag` overrides it."
  >}}
  {{< hextra/feature-card
    title="No Token Handling"
    subtitle="A gh extension: authentication, `GH_TOKEN` and GitHub Enterprise hosts all come from gh."
  >}}
  {{< hextra/feature-card
    title="Nothing Dropped"
    subtitle="Titles outside the Conventional Commits format land in an Unclassified group and are warned about, never left out."
  >}}
  {{< hextra/feature-card
    title="Scriptable"
    subtitle="The product on stdout, diagnostics on stderr, stable JSON and meaningful exit codes, and no prompts without a terminal."
  >}}
{{< /hextra/feature-grid >}}
