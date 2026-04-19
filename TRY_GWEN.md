# Try Gwen

Minimal way to let someone try the current Python Gwen reference implementation and inspect a real Gwen project.

## What To Send

Send the whole repository as either:

- a GitHub repo link
- a zip of the repository root

The trial flow below uses the built-in multi-file sample project:

- `examples/ledger_app/main.gw`
- `examples/ledger_app/ledger_store.gw`
- `examples/ledger_app/ledger_report.gw`
- `examples/ledger_app/ledger_queries.gw`
- `examples/ledger_app/ledger_view.gw`
- `examples/ledger_app/ledger_seed.gw`

## One-Click Trial

### macOS / Linux

```bash
./scripts/try_ledger_app.sh
```

### Windows PowerShell

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\try_ledger_app.ps1
```

Both scripts will:

1. create a local virtual environment at `.gwen-demo-venv`
2. install the Gwen package from the current repository
3. run `gwen check` on the sample project
4. run the sample project

## What To Read After It Runs

Open these files in order:

1. `examples/ledger_app/main.gw`
2. `examples/ledger_app/ledger_seed.gw`
3. `examples/ledger_app/ledger_queries.gw`
4. `examples/ledger_app/ledger_report.gw`
5. `examples/ledger_app/ledger_store.gw`
6. `examples/ledger_app/ledger_view.gw`

## Notes

- This is Gwen `v0.1.0`, the Python reference implementation.
- `parallel` syntax exists, but the Python implementation still executes it sequentially.
