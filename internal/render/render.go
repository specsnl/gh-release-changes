// Package render turns parsed changes into the text a command prints: the
// grouped "What's Changed" Markdown, the issue list per group, and the issue
// dump for the writing skills.
//
// It is pure: it never imports internal/github, takes plain structs from
// internal/changes, and returns text, so every output is golden-tested without
// HTTP. Output is deterministic — the same items always render the same bytes —
// because skills print it unchanged.
package render
