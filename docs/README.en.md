[中文版本](./README.md)

# Gwen Docs Index

This page is the entry point for the documentation set.

The docs currently serve three different readers:

- people who want to write Gwen
- people who want to understand Gwen's design choices
- people who want to keep building Gwen

If you only want to start writing Gwen, read these first:

- [syntax.md](./syntax.md)
- [types.md](./types.md)
- [scope.md](./scope.md)
- [stdlib.md](./stdlib.md)

If you want the design rationale, read:

- [philosophy.en.md](./philosophy.en.md)
- [modules.md](./modules.md)
- [concurrency.md](./concurrency.md)
- [oop.md](./oop.md)

If you care about the implementation and compiler path, read:

- [compiler.en.md](./compiler.en.md)
- [tracking.md](./tracking.md)
- [release.md](./release.md)
  release checklist for maintainers and agents

## What Each File Is For

| File | Purpose |
|------|---------|
| [syntax.md](./syntax.md) | core syntax and control flow |
| [types.md](./types.md) | type system, explicit precision numerics, `money[...]` |
| [scope.md](./scope.md) | scope rules, `global`, nested functions |
| [modules.md](./modules.md) | modules, imports, visibility |
| [stdlib.md](./stdlib.md) | current stdlib surface and boundary |
| [concurrency.md](./concurrency.md) | `parallel`, shared state, current concurrency semantics |
| [memory.md](./memory.md) | current memory model and arena direction |
| [oop.md](./oop.md) | restricted object system |
| [appendix.md](./appendix.md) | keywords, operators, appendix material |
| [philosophy.en.md](./philosophy.en.md) | the design filter Gwen uses for new features |
| [compiler.en.md](./compiler.en.md) | frontend, HIR, MIR, C emitter, compiler path |
| [tracking.md](./tracking.md) | implementation tracking |

## Read With These Rules In Mind

- `tracking.md` is an implementation log, not a language manual.
- `compiler.en.md` is for implementation work, not for first-time Gwen users.
- If something only appears in `tracking.md` and has not entered the main syntax/types/stdlib docs, do not treat it as stable public surface yet.
- Most detailed reference docs are still written in Chinese today; the English pages here cover the entry points and implementation-facing summaries.
