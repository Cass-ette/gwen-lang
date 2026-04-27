# Changelog

## Unreleased

### v0.2.1 - AI onboarding and release hygiene

Planned patch release focused on documentation and repository confidence:

- add an AI agent guide for first-contact repository navigation
- document the Python `v0.1.0` reference line versus the Go `v0.2.x` mainline
- add release checklist documentation
- improve top-level README onboarding for humans and agents

No language behavior changes are planned for this release.

## v0.2.0 - Go mainline staged maturity release

- marks the Go implementation as the current `v0.2.0` mainline
- keeps the Python implementation as the `v0.1.0` reference line
- exposes the Go CLI version as `0.2.0`
- updates the VSCode extension package to `0.2.0`
- includes a compiled path through HIR/MIR, C emission, and the host C compiler

## v0.1.0 - Python reference implementation

- packages the Python reference implementation
- provides the early CLI surface for `run`, `check`, `repl`, `--version`, and `--help`
- keeps Python `parallel` as a sequential reference behavior
