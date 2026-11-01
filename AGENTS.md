# gh-release-changes

A gh extension written in Go (cobra). The design lives in
[`docs/plans/gh-github-releases.md`](https://github.com/specsnl/agent-skills-public/blob/main/docs/plans/gh-github-releases.md)
in `specsnl/agent-skills-public`.

- Run every command through the Taskfile (`task --list`); the toolchain runs in Docker Compose, not on the host.
- Run `task checkall` before finishing a change; it mirrors CI.
- Asset names in `.goreleaser.yml` are a contract with `gh extension install`: keep
  `gh-release-changes-<os>-<arch>`.
- Authentication goes through `go-gh`; never read or store tokens directly.
