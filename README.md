# Gwen

Gwen is an audit-friendly, math-intuitive programming language designed for backend development and DevOps automation.

Current implementation status: Go bootstrap frontend/runtime for the frozen `v0.1` surface, with the earlier Python reference implementation still kept in-tree.

## Design Philosophy

- **Audit-first** — In the age of AI-generated code, readability and auditability come first
- **Math-intuitive** — Syntax follows mathematical conventions; accessible with math and English basics
- **Explicit over implicit** — Errors must be handled, interfaces must be marked, parallelism must be declared
- **Natural but not verbose** — More concise than Pascal, more natural than C

## Quick Start

```bash
go run ./cmd/gwen --version
go run ./cmd/gwen run examples/hello.gw
go run ./cmd/gwen check examples/hello.gw
go run ./cmd/gwen repl

# Python reference path is still available
python3 -m pip install -e .
gwen examples/hello.gw
```

## Share With A Friend

If you want someone else to try Gwen quickly and read a real project, send them the repo and point them to [TRY_GWEN.md](TRY_GWEN.md).

- macOS / Linux: `./scripts/try_ledger_app.sh`
- Windows PowerShell: `powershell -ExecutionPolicy Bypass -File .\scripts\try_ledger_app.ps1`

### VSCode Extension

Gwen has a VSCode extension for syntax highlighting and snippets:

```bash
cd vscode-extension
./install.sh
```

Features:
- Syntax highlighting for `.gw` files
- Code snippets (func, if, while, for, match, etc.)
- Auto-indentation based on block structure
- Comment toggling with `Cmd+/` / `Ctrl+/`

## Language Features

### Variables & Types

```gwen
x := 42            // inferred
x: int := 42       // explicit type
```

### Functions

```gwen
func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc
```

### Control Flow

```gwen
if x > 0 then
  do_a()
elif x = 0 then
  do_b()
else
  do_c()
endif

while b != 0 do
  b := b - 1
endwhile

for i in 1 to 10 do
  write(i)
endfor

for i in 1 to 10 step 2 do
  write(i)
endfor

for item in list with index i do
  write(i, item)
endfor
```

### Pattern Matching

```gwen
match x
  when 1 => do_a()
  when 2, 3 => do_b()
  when 4 to 10 => do_c()
  else do_d()
endmatch
```

### Error Handling (Result type)

```gwen
match readfile("config.txt")
  when ok(data) =>
    write(data)
  when err(e) =>
    write("Error:", e)
endmatch
```

### Modules

```gwen
module math_utils

export func gcd(a: int, b: int) -> int
  // ...
endfunc

endmodule

use gcd from math_utils
use math_utils
```

### Parallel Syntax

```gwen
parallel do
  deploy(server1)
  deploy(server2)
endparallel

parallel allowfail => results do
  check(server1)
  check(server2)
endparallel
```

In the Go runtime, each top-level statement inside a `parallel` block runs concurrently in its own snapshot of the outer environment. `=> results` preserves source order, expression statements capture their values, and `allowfail` records `err(...)` entries instead of aborting the block. The older Python reference implementation still keeps `parallel` sequential.

### Navigation Tags

```gwen
func deploy(config: Config)

  @validate
  check_config(config)

  @build
  build_project()

  @push
  push_to_server()

endfunc
```

## Implementation

The active bootstrap lives in Go:

- `cmd/gwen` — CLI entrypoint
- `internal/lexer` — Tokenizer
- `internal/parser` — Recursive descent parser
- `internal/checker` — Pre-execution semantic checker
- `internal/interpreter` — Go tree-walk runtime

The earlier Python reference implementation is still kept for comparison and migration support:

- `gwen/lexer.py`
- `gwen/parser.py`
- `gwen/checker.py`
- `gwen/interpreter.py`

## Running Tests

```bash
go test ./...
pytest
```

## File Extension

`.gw`

## Project Structure

```
gwen-lang/
├── gwen/              # Interpreter implementation
│   ├── lexer.py
│   ├── parser.py
│   ├── interpreter.py
│   └── ast_nodes.py
├── docs/              # Language documentation
│   ├── syntax.md
│   ├── types.md
│   ├── scope.md
│   └── ...
├── examples/          # Example programs
├── tests/             # Test suite
└── vscode-extension/  # VSCode extension
```

## Documentation

- [docs/README.md](docs/README.md) — Documentation index
- [docs/syntax.md](docs/syntax.md) — Syntax reference
- [docs/types.md](docs/types.md) — Type system
- [docs/scope.md](docs/scope.md) — Variable scoping
- [docs/modules.md](docs/modules.md) — Module system
- [docs/stdlib.md](docs/stdlib.md) — Stdlib boundary and import shape
- [docs/tracking.md](docs/tracking.md) — Implementation status
