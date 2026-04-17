# Gwen

Gwen is an audit-friendly, math-intuitive programming language designed for backend development and DevOps automation.

## Design Philosophy

- **Audit-first** — In the age of AI-generated code, readability and auditability come first
- **Math-intuitive** — Syntax follows mathematical conventions; accessible with math and English basics
- **Explicit over implicit** — Errors must be handled, interfaces must be marked, parallelism must be declared
- **Natural but not verbose** — More concise than Pascal, more natural than C

## Quick Start

```bash
python3 -m gwen examples/hello.gw
python3 -m gwen  # starts the REPL
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

```
x := 42            // inferred
x: int := 42       // explicit type
```

### Functions

```
func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc gcd
```

### Control Flow

```
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

```
match x
  when 1 then do_a()
  when 2, 3 then do_b()
  when 4 to 10 then do_c()
  else do_d()
endmatch
```

### Error Handling (Result type)

```
func read_file(path: string)
  if file_exists(path) then
    return ok(content)
  else
    return err("file not found")
  endif
endfunc

match read_file("/etc/config")
  when ok(data) then
    write(data)
  when err(e) then
    write("Error:", e)
endmatch
```

### Modules

```
module math_utils

export func gcd(a: int, b: int) -> int
  // ...
endfunc

endmodule

use gcd from math_utils
use math_utils
```

### Parallel Execution

```
parallel do
  deploy(server1)
  deploy(server2)
endparallel

parallel allowfail => results do
  check(server1)
  check(server2)
endparallel
```

### Navigation Tags

```
func deploy(config: Config)

  @validate
  check_config(config)

  @build
  build_project()

  @push
  push_to_server()

endfunc deploy
```

## Implementation

Gwen is implemented in Python as a tree-walk interpreter:

- `gwen/lexer.py` — Tokenizer
- `gwen/ast_nodes.py` — AST node definitions
- `gwen/parser.py` — Recursive descent parser
- `gwen/interpreter.py` — Tree-walk interpreter

## Running Tests

```bash
python3 tests/test_lexer.py
python3 tests/test_parser.py
python3 tests/test_interpreter.py
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
- [docs/tracking.md](docs/tracking.md) — Implementation status
