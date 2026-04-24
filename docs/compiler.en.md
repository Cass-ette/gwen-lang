[中文版本](./compiler.md)

# Gwen Compiler Path

This document is for people working on Gwen's implementation.

It only answers three things:

- what the compiler path looks like today
- which boundaries are already taking shape
- what still needs real work next

## The Goal

Gwen is not trying to look compiled on the surface. The real goal is:

- a compiled Gwen program should not require Go to be installed
- a compiled Gwen program should not keep running on top of a hidden Go runtime
- Go is only the current bootstrap implementation language

So these routes do not count as the target:

- translate Gwen into wrapped Go code
- ship the Go runtime inside the final result
- hide Go from the user while still depending on Go execution semantics

## The Current Pipeline

Today the main path in the repository is:

```text
source
  -> frontend
  -> HIR
  -> MIR
  -> C emitter
  -> cc
  -> native executable
```

Relevant directories:

- `internal/frontend`
- `internal/hir`
- `internal/mir`
- `internal/backend/cgen`
- `cmd/gwen`

You can drive that path directly from the CLI:

```bash
go run ./cmd/gwen build examples/hello.gw
go run ./cmd/gwen emit-c examples/hello.gw
```

`build` currently produces host-native executables:

- macOS: `Mach-O`
- Linux: `ELF`
- Windows: `PE/.exe`

## What Already Exists

Not planned. Already present in the repository:

- a unified frontend entry point
- HIR as a layer separate from raw AST
- early binding for names, `global`, `leave/next`, and `match` targets
- MIR with blocks, slots, values, and terminators
- a C emitter that is exercised by real examples, not only toy snippets

The compiler path is currently pressured by real programs such as:

- `examples/http_server.gw`
- `examples/docs_site/main.gw`
- `examples/session_notes.gw`
- `examples/sqlite_basics.gw`
- `examples/rules_app/main.gw`
- `examples/ledger_app/main.gw`

That means Gwen is no longer at the stage where IR work happens without backend pressure.

## Current Boundaries

### Frontend

The frontend now owns real compiler-facing responsibilities:

- file loading
- parse
- checker
- module expansion
- producing a unified input for later lowering

This removes the old split between "interpreter setup" and "compiler frontend".

### HIR

HIR is not there to optimize yet. Its job is to reorganize AST surface structure into something the compiler can consume.

Information that has already started to stabilize:

- top-level `use / func / module / object / type`
- structured type annotations
- statement and expression skeletons
- one layer of binding
- outer targets for `global`
- loop targets for `leave/next`
- basic `match` pattern shapes

Things that are not fully frozen yet:

- complete known types for every expression
- fuller definite-assignment information
- a more systematic binding/type combination layer

### MIR

MIR is already beyond a pure control-flow sketch.

It now explicitly carries:

- `Block`
- `Terminator`
- `Slot`
- `Value`
- part of the real instruction sequence

The current MIR is already making backend-relevant structure explicit:

- control-flow edges
- local slots
- call results
- multi-return values
- part of member/index access
- part of declaration and assignment

It is still not fully lowered to the last primitive layer:

- some higher-level Gwen semantics are still preserved
- runtime ABI is not fully frozen
- some value/typing parts are still in the "good enough for now" stage

## Why Runtime, Stdlib, and Real Examples Came First

Because they decide the compiler boundary in practice.

`http`, `json`, `sqlite`, `state`, the docs site, and real backend examples force Gwen to answer questions like:

- which types become official runtime handles
- which semantics must be statically decidable
- which helpers are only bootstrap conveniences
- which capabilities deserve to enter a long-term ABI

If those answers are not pressured early, the backend can move fast and still force language-level rework later.

## What Is Still Open

This phase is now closed enough to move on, but the compiler path is not done. The next real work is:

### 1. tighten the runtime ABI further

At minimum, these boundaries need to become more explicit:

- basic value representation
- `result[...]` representation
- function call ABI
- module initialization order
- the boundary between runtime handles and Gwen values

### 2. keep lowering high-level constructs away

The current MIR is sufficient for the first backend, but it still preserves part of Gwen's own high-level structure.

That should keep moving toward flatter backend-facing form:

- more control flow
- more assignment/value shapes
- less semantics that the backend has to infer again

### 3. keep compiled and interpreted paths aligned

Recent work already closed part of that gap:

- `bool` display
- container display
- part of I/O error wording
- differential regression coverage between interpreter and compiled output

Still remaining:

- details of `json.stringify(dict)`
- more consistency in error text and diagnostic shape

### 4. keep expanding real coverage

The first backend is already real, but it does not yet cover every surface area of the language.

The right next step is:

- use real examples to expand coverage
- use differential tests to catch semantic drift
- then decide which capabilities deserve the next round of freezing

## Why The First Backend Is C

The current C choice is not a statement that Gwen ends at C. It is a practical first bridge.

The reasons are simple:

- it gives native executables
- the toolchain is mature
- the output is inspectable
- it helps settle ABI, lowering, and runtime boundaries before heavier backends

The point of this route is to get a real non-interpreted backend first.

Whether Gwen later adds:

- LLVM
- Wasm
- lower-level native codegen

is a later decision, not a prerequisite for the current bootstrap path.

## Self-Hosting Is Not This Phase

Self-hosting can be a long-term goal.

But the order cannot be reversed:

1. stabilize the bootstrap compiler
2. get Gwen programs off the Go runtime
3. only then move toward Gwen compiling Gwen

Right now, "stop depending on the Go runtime" matters more than "stop being implemented in Go".

## What This Document Does Not Try To Do

This document is not for:

- teaching first-time users how to write Gwen
- selling Gwen as a special language
- recording every small step in the implementation

For those, read:

- `docs/syntax.md` / `docs/types.md` / `docs/stdlib.md`
- `docs/philosophy.en.md`
- `docs/tracking.md`
