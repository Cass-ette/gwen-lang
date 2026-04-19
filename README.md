# Gwen

Gwen is an audit-friendly, math-intuitive programming language designed for backend development and DevOps automation.

Current implementation status: Python reference implementation for the frozen `v0.1` front-end surface.

## Design Philosophy

- **Audit-first** — In the age of AI-generated code, readability and auditability come first
- **Math-intuitive** — Syntax follows mathematical conventions; accessible with math and English basics
- **Explicit over implicit** — Errors must be handled, interfaces must be marked, parallelism must be declared
- **Natural but not verbose** — More concise than Pascal, more natural than C

## Quick Start

```bash
python3 -m pip install -e .

gwen --version
gwen examples/hello.gw
gwen check examples/hello.gw
gwen repl

# legacy invocation still works
python3 -m gwen examples/hello.gw
```

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
  when 1 then do_a()
  when 2, 3 then do_b()
  when 4 to 10 then do_c()
  else do_d()
endmatch
```

### Error Handling (Result type)

```gwen
match readfile("config.txt")
  when ok(data) then
    write(data)
  when err(e) then
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

In the current Python reference implementation, `parallel` syntax and failure handling are frozen, but execution is still sequential. Real parallel runtime behavior is a later compiler/runtime-stage feature.

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

Gwen is implemented in Python as a tree-walk interpreter:

- `gwen/lexer.py` — Tokenizer
- `gwen/ast_nodes.py` — AST node definitions
- `gwen/parser.py` — Recursive descent parser
- `gwen/checker.py` — Pre-execution semantic checker
- `gwen/interpreter.py` — Tree-walk interpreter
- `gwen/stdlib_catalog.py` — Official stdlib module surface

## Running Tests

```bash
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
