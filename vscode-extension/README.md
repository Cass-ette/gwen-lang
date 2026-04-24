# Gwen Language Support for VSCode

VSCode extension for the current Gwen `v0.1` language surface.

Gwen itself is currently in this state:

- the main implementation is written in Go
- `gwen run/check/repl` use the Go frontend and runtime
- `gwen build` lowers Gwen through HIR and MIR, emits C, then calls the host C compiler

This extension does not change that pipeline. It only provides editor support.

## Features

- Syntax highlighting for current Gwen keywords and block forms:
  `object/endobject`, `new/endnew`, `module/endmodule`,
  `use ... from ...`, `type`, `const`, `var/endvar`,
  `parallel`, `allowfail`, `arena`
- Comment support for `//` and `/* ... */`
- Highlighting for builtins and official stdlib import names:
  `list`, `string`, `math`, `dict`, `io`
- Snippets for the constructs you actually write today:
  functions, objects, modules, imports, `match ok/err`, `var default`,
  `parallel`, `arena`
- Auto-indentation for Gwen `endxxx` block syntax

## Installation

### From Source

```bash
cd vscode-extension
npm install -g @vscode/vsce
vsce package
code --install-extension gwen-lang-0.1.0.vsix
```

### Development Mode

1. Open `vscode-extension/` in VSCode
2. Press `F5` to launch an Extension Development Host
3. Open any `.gw` file and verify highlighting/snippets

## Running Gwen Files

The extension itself does not contribute a debugger or runtime adapter.

In this repository, the checked-in workspace config exposes these commands in VSCode:

- `Run Gwen File`
- `Check Gwen File`
- `Build Gwen File`

They currently run the Go CLI:

```bash
go run ./cmd/gwen run <file>
go run ./cmd/gwen check <file>
go run ./cmd/gwen build <file>
```

That repository-level `.vscode/` setup is separate from the extension package.

## Snippets

| Prefix | Description |
|--------|-------------|
| `func` / `funcr` | Function definition |
| `if` / `ifelse` | Conditional blocks |
| `while`, `for`, `foreach` | Loop blocks |
| `match`, `matchr` | Generic match / `result` match |
| `module`, `usefrom`, `usemod` | Module definitions and imports |
| `object`, `expobject` | Object definitions |
| `type`, `const`, `var`, `vardefault` | Type alias and binding templates |
| `ok`, `err` | Result constructors |
| `global`, `arena`, `parallel`, `parallelr` | Runtime-oriented block forms |
| `lambda`, `write`, `tag` | Common utility snippets |

## Example

```gwen
use range from list

object Counter
  values: list[int]

  new() -> Counter
    return Counter{
      values := range(1, 3)
    }
  endnew

  func print(self: Counter)
    for value in self.values do
      write(value)
    endfor
  endfunc
endobject

func main()
  counter := Counter.new()
  counter.print()
endfunc
```

## Notes

- This extension currently provides syntax highlighting, comments, snippets, and indentation rules.
- It does **not** ship a language server, formatter, or debugger.
- It does **not** mean Gwen is already a pure C implementation. The current shipped toolchain is Go frontend/runtime plus a C-emitting build path.
- Gwen still allows many stdlib-style builtins to be used directly, but the extension also highlights the recommended `use ... from list/string/math/dict/io` style.
