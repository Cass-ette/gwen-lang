# Gwen Agent Guide

Gwen is a language project for backend and automation work. It is designed around explicit intent, stable semantics, and code that AI agents can inspect without relying on hidden context.

## Current Release Lines

| Line     | Implementation | Status                                                            |
| -------- | -------------- | ----------------------------------------------------------------- |
| `v0.1.0` | Python         | Reference implementation kept in-tree for history and comparison. |
| `v0.2.x` | Go             | Current mainline implementation. Use this for new work.           |

Do not treat the Python implementation as the current mainline unless the task explicitly says Python reference implementation.

## What Gwen Is Today

- `cmd/gwen` is the current CLI.
- `gwen run`, `gwen check`, `gwen repl`, `gwen emit-c`, and `gwen build` exist.
- `gwen run` and `gwen repl` use the Go interpreter path.
- `gwen build` lowers Gwen through frontend/HIR/MIR, emits C, and invokes the host C compiler to produce a native executable.
- The compiler is not self-hosted yet; Go is the bootstrap implementation language.

## First Commands

```bash
go run ./cmd/gwen --version
go run ./cmd/gwen run examples/hello.gw
go run ./cmd/gwen check examples/hello.gw
go run ./cmd/gwen build examples/hello.gw -o /tmp/gwen-hello
/tmp/gwen-hello
```

Run focused tests before claiming success:

```bash
go test ./cmd/gwen
python -m pytest tests/test_cli.py
```

Use `go test ./...` when changing core language behavior, checker/runtime/compiler internals, or examples used by broad test coverage.

## Repository Map

| Path                     | Purpose                                                                    |
| ------------------------ | -------------------------------------------------------------------------- |
| `cmd/gwen/`              | Go CLI entry point.                                                        |
| `internal/frontend/`     | File loading, parsing, checking, module expansion, analysis entry points.  |
| `internal/hir/`          | High-level intermediate representation.                                    |
| `internal/mir/`          | Lower compiler-facing representation.                                      |
| `internal/backend/cgen/` | C emitter and compiled-path runtime support.                               |
| `internal/interpreter/`  | Go interpreter runtime.                                                    |
| `gwen/`                  | Older Python reference implementation.                                     |
| `docs/`                  | Language and implementation documentation.                                 |
| `examples/`              | Gwen programs used for demos and smoke coverage.                           |
| `tests/`                 | Python reference tests and self-checks.                                    |
| `vscode-extension/`      | VSCode syntax/snippet extension.                                           |

## Read These Before Changing Behavior

- `docs/philosophy.en.md` or `docs/philosophy.md` for the design filter.
- `docs/README.en.md` or `docs/README.md` for the documentation map.
- `docs/syntax.md`, `docs/types.md`, `docs/scope.md`, and `docs/stdlib.md` for public language surface.
- `docs/compiler.en.md` or `docs/compiler.md` before touching HIR, MIR, C generation, or `gwen build`.
- `docs/tracking.md` for implementation history. Treat it as a log, not as the stable spec.

## Change Rules For Agents

- Prefer the Go implementation for current-line fixes and features.
- Keep diffs minimal; do not refactor unrelated code while fixing docs or release metadata.
- Do not add a second long-term spelling for an existing language concept just because it is shorter.
- Do not change language behavior without tests covering checker/runtime/compiler impact as applicable.
- Do not claim work is complete without running the smallest relevant validation command.
- If a change affects both interpreted and compiled behavior, verify both paths or clearly state what remains unverified.

## Release Process

For release preparation, read `docs/release.md` before changing versions, packaging the VSCode extension, tagging, or publishing. Do not tag or publish a planned release unless the task explicitly asks for it.

## Current Roadmap Bias

For `v0.2.x`, prioritize documentation, release confidence, run/build parity, error quality, and AI-friendly onboarding. Do not start self-hosting as a drive-by change.
