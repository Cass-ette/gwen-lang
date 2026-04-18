# Gwen Ledger App

Small Gwen test project for shaking out language design decisions with something more realistic than a toy algorithm.

## Run

```bash
python3 -m gwen examples/ledger_app/main.gw
```

It writes a demo ledger file to `/tmp/gwen_ledger_app_demo.txt`, then reloads it and prints the same summary again.

## What This Project Exercises

- Restricted objects: `Entry` and `Ledger`
- Private fields + getter/method based access
- `result[T]`-style file I/O with `match`
- Inline modules in a single Gwen file
- `list`, `dict`, `split`, `join`, `sort`, `keys`
- A small amount of persistence logic instead of pure in-memory demos

## Why This Is A Good Gwen Sandbox

This project is intentionally chosen to pressure the parts of Gwen that are still settling:

- Whether the restricted object system feels too verbose in normal code
- Whether inline modules are enough before real cross-file imports exist
- Whether `result` + `match` remains ergonomic once I/O shows up everywhere
- Which small standard-library helpers are still missing

## GitHub Language Recognition

GitHub language stats are driven by GitHub Linguist, not by a marker inside the `.gw` file itself.

Current practical implications:

- The repo can clearly present itself as Gwen via README, folder layout, examples, and repo topics.
- But GitHub will not show `Gwen` in the language bar until Gwen is added to GitHub Linguist as a supported language.

Relevant references:

- https://github.com/github-linguist/linguist/blob/main/docs/overrides.md
- https://github.com/github-linguist/linguist/blob/main/CONTRIBUTING.md
