# Gwen Language Support for VSCode

VSCode extension for the Gwen programming language.

## Features

- **Syntax Highlighting**: Full support for Gwen keywords, types, operators, strings, numbers, and tags
- **Code Snippets**: Quick insertion of common Gwen constructs (functions, loops, conditionals)
- **Auto-indentation**: Smart indentation based on block structure
- **Comment Support**: Toggle line comments with `Cmd+/` (macOS) or `Ctrl+/` (Windows/Linux)

## Installation

### From Source

```bash
cd vscode-extension
npm install -g @vscode/vsce
vsce package
code --install-extension gwen-lang-0.1.0.vsix
```

### Development Mode

1. Open this folder in VSCode
2. Press `F5` to launch Extension Development Host
3. Open a `.gw` file to test

## Snippets

| Prefix | Description |
|--------|-------------|
| `func` | Function definition |
| `if` / `ifelse` | If statement |
| `while` | While loop |
| `for` / `foreach` | For loops |
| `match` | Pattern matching |
| `module` | Module definition |
| `var` | Variable declaration |
| `ok` / `err` | Result types |
| `global` | Global variable |
| `arena` | Memory arena |
| `parallel` | Parallel block |
| `lambda` | Anonymous function |

## Language Features Highlighted

- **Keywords**: `func`, `if`, `while`, `for`, `match`, `module`, `return`, `parallel`
- **Types**: `int`, `float`, `string`, `bool`, `list`, `result`
- **Precision Types**: `int8`, `int16`, `int32`, `int64`, `float32`, `float64`
- **Operators**: `:=`, `=`, `!=`, `<=`, `>=`, `->`, `=>`
- **Built-ins**: `write`, `len`, `append`, `pop`
- **Tags**: `@tagname` for navigation

## Example

```gwen
-- Gwen Hello World
func main()
  write("Hello, Gwen!")
endfunc
```

## Requirements

- VSCode 1.60.0 or higher
- Gwen interpreter installed (for running code)

## Release Notes

### 0.1.0

- Initial release
- Syntax highlighting for Gwen language
- Code snippets for common patterns
- Auto-indentation support
