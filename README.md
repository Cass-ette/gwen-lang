[中文 README](README.zh.md)

# Gwen

Gwen is a language project for backend and automation work.

This repository currently has two active tracks:

- a runnable Go bootstrap implementation
- a compiler path that can already produce native executables

## Current State

- `cmd/gwen` is the main CLI
- `gwen run/check/repl/build/emit-c` are available
- the interpreter, checker, frontend, and C emitter all live in this repo
- `examples/` already contains real programs: HTTP, SQLite, docs site, rules app, ledger app
- the older Python reference implementation is still kept in-tree, but the main line is now the Go implementation

## Quick Start

```bash
go run ./cmd/gwen --version
go run ./cmd/gwen run examples/hello.gw
go run ./cmd/gwen check examples/hello.gw
go run ./cmd/gwen repl
```

If you want to go through the compiled path, you need `cc` in `PATH`:

```bash
go run ./cmd/gwen build examples/hello.gw -o /tmp/gwen-hello
/tmp/gwen-hello
```

If you only want to inspect generated C:

```bash
go run ./cmd/gwen emit-c examples/hello.gw
```

## Small Example

```gwen
func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc

func main()
  write(gcd(48, 18))
endfunc
```

Some basic Gwen surface rules:

- blocks close explicitly: `endif/endwhile/endfor/endfunc`
- errors use `result[...]` with `match ok/err`
- scope is local by default; outer mutation must use `global`
- concurrency must be written explicitly as `parallel`

## What To Try First

### Hello

```bash
go run ./cmd/gwen run examples/hello.gw
```

### HTTP Example

```bash
go run ./cmd/gwen run examples/http_server.gw
```

Then open:

- `http://127.0.0.1:8080/`
- `http://127.0.0.1:8080/api/hello/Ada?lang=zh`
- `http://127.0.0.1:8080/assets/app.css`

### Session Notes

```bash
go run ./cmd/gwen run examples/session_notes.gw
```

Then open:

- `http://127.0.0.1:8082/`
- `http://127.0.0.1:8082/login/Ada`
- `http://127.0.0.1:8082/api/me`

### Docs Site Prototype

```bash
go run ./cmd/gwen run examples/docs_site/main.gw
```

Then open:

- `http://127.0.0.1:8090/`
- `http://127.0.0.1:8090/api/health`
- `http://127.0.0.1:8090/api/site/en`

The site reads repository Markdown and Gwen source files directly.

## VSCode

The repository includes a minimal VSCode extension with:

- `.gw` syntax highlighting
- basic snippets
- block-aware indentation and comment config

See [vscode-extension/README.md](vscode-extension/README.md) for installation.

## How To Read This Repo

- [docs/README.en.md](docs/README.en.md)
  docs index for readers who want an English entry page
- [docs/syntax.md](docs/syntax.md)
  language surface
- [docs/types.md](docs/types.md)
  type system
- [docs/stdlib.md](docs/stdlib.md)
  current stdlib boundary
- [docs/compiler.en.md](docs/compiler.en.md)
  compiler path and current backend boundary
- [docs/philosophy.en.md](docs/philosophy.en.md)
  the design filter Gwen uses for new features
- [docs/tracking.md](docs/tracking.md)
  implementation tracking

## Layout

```text
gwen-lang/
├── cmd/gwen/           # CLI
├── internal/           # Go implementation
├── gwen/               # Older Python reference implementation
├── docs/               # Language docs
├── examples/           # Example programs
├── tests/              # Python-side tests and self-checks
└── vscode-extension/   # VSCode extension
```

## Tests

```bash
go test ./...
pytest
```
