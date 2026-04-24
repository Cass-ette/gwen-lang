[中文版本](./philosophy.md)

# Gwen Design Filter

This document is not a slogan page. It only answers one question:

When should a new syntax rule, language feature, or standard-library capability enter Gwen, and when should it stay out?

## The Four Questions

Every candidate design should go through these four checks first:

1. Does it turn hidden context into surface syntax?
2. Does it create two long-lived "main" ways to write the same thing?
3. Does it make checker/compiler reasoning easier instead of harder?
4. In real code, is it shorter and more stable than patch-style workarounds?

If the first two fail, it usually should not enter the language.

If the third fails, it usually means the design is only convenient for the interpreter.

If the fourth fails, it is usually just a syntax shortcut that looks nice in isolation.

## What Gwen Prefers

- explicit targets
- one main spelling
- auditable control flow
- clear scope boundaries
- errors and side effects written on the surface

Gwen is willing to pay a small local cost in exchange for:

- less drift during refactoring
- less confusion under nesting
- clearer checker diagnostics
- less guessing inside the compiler

## What Gwen Is Wary Of

- saving a few characters while hiding semantics
- giving the same capability two or three "main" spellings
- designs that only make sense if readers rely on habits or defaults
- semantics that break down once nesting gets deeper
- features the interpreter can handle but the checker/compiler cannot explain cleanly

## Dot Rules

Gwen does not reject `.`, but it does reject turning `.` into a universal escape hatch.

Today `A.B` should mainly mean a small number of stable things:

- module namespace calls: `http.listen`, `json.stringify`
- user object methods: `order.describe()`
- a limited set of stable member accesses

Gwen does not want long-term dual tracks like:

- `http.path(req)` and `req.path` both treated as main surface
- switching the same capability back and forth between `module.name(...)` and instance-property skins
- long chains that hide side effects, state changes, or implicit conversions

Default rules for dot-style surface:

- module capabilities should prefer `module.name`
- user object behavior can use `obj.method()`
- runtime handles should prefer explicit helpers over large property skins
- the same capability should not keep two main spellings alive

## Loop Control As An Example

Gwen chooses named loops with `leave <name>` / `next <name>` instead of bare `break` / `continue`.

```gwen
while running do scan
  if bad_line then
    next scan
  endif

  if done then
    leave scan
  endif
endwhile scan
```

The tradeoff is explicit:

- the control target is written on the surface
- readers do not need to count loop nesting
- code does not need `done` or `should_break` patch variables
- checker and compiler can validate the target directly

This may be longer in a tiny single-loop example. Gwen does not optimize for the shortest isolated line. It optimizes for code that still reads cleanly once it grows.

## Before A Design Enters The Language

At minimum, it should come with:

- one real example showing that it removes patch-style code
- one counterexample showing that it does not make old code harder to read
- checker errors that remain clear
- no second special rule stack for the compiler/runtime to maintain

If that bar is not met, it should stay out for now.

## One-Line Standard

Gwen's standard is not "does this look like other languages?" but "is the intent written directly on the surface?"
