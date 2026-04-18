# Gwen Ledger App

Small Gwen test project for shaking out language design decisions with something more realistic than a toy algorithm.

## Run

```bash
python3 -m gwen examples/ledger_app/main.gw
```

It seeds a demo ledger, runs several filtered queries, deletes one entry by index, updates one note, prints month-specific reports, writes the updated ledger to `/tmp/gwen_ledger_app_demo.txt`, then reloads it and verifies the persisted state again.

## What This Project Exercises

- Restricted objects: `Entry` and `Ledger`
- Private fields + getter/method based access
- `result[T]`-style file I/O with `match`
- Multi-file modules loaded via `use ... from ...`
- `list`, `dict`, `split`, `join`, `sort`, `keys`
- Query-style filtering via `list_entries(category, kind, month, note_keyword)`
- Deletion by index via `delete_entry(index)`
- Field mutation through object methods via `update_entry_note(index, note)`
- Month-filtered aggregation via `print_month_report(ledger, month)`
- A small amount of persistence logic instead of pure in-memory demos

## File Layout

- `main.gw`: core objects plus the runnable flow
- `ledger_store.gw`: persistence
- `ledger_report.gw`: summary + month reports
- `ledger_queries.gw`: filtered query printing
- `ledger_view.gw`: shared presentation helper
- `ledger_seed.gw`: demo data seeding

## Why This Is A Good Gwen Sandbox

This project is intentionally chosen to pressure the parts of Gwen that are still settling:

- Whether the restricted object system feels too verbose in normal code
- Whether inline modules are enough before real cross-file imports exist
- Whether `result` + `match` remains ergonomic once I/O shows up everywhere
- Which small standard-library helpers are still missing

Concrete pressure points now visible in the example:

- `delete_entry()` 现在依赖 `removeat()`；这次真实项目把这个列表缺口逼出来之后才补进解释器。
- `update_entry_note()` cannot write `entry.note` directly from outside `Entry`; it has to go through an object method because fields are private.
- The app only became a real multi-file project after the interpreter learned to resolve `use` imports from neighboring `.gw` files.
- `export` 现在是真正的可见性边界，不再只是文档约定。

## GitHub Language Recognition

GitHub language stats are driven by GitHub Linguist, not by a marker inside the `.gw` file itself.

Current practical implications:

- The repo can clearly present itself as Gwen via README, folder layout, examples, and repo topics.
- But GitHub will not show `Gwen` in the language bar until Gwen is added to GitHub Linguist as a supported language.

Relevant references:

- https://github.com/github-linguist/linguist/blob/main/docs/overrides.md
- https://github.com/github-linguist/linguist/blob/main/CONTRIBUTING.md
