# Gwen Rules App

Second Gwen test project focused on rule composition rather than CRUD.

## Run

```bash
python3 -m gwen examples/rules_app/main.gw
```

## What This Project Exercises

- Multi-file module loading with `use ... from ...`
- First-class functions as rule units inside a list
- `result[T]` propagation when a rule fails
- `match`-based reporting over final decision status
- Restricted objects for `Request` and `Decision`

## File Layout

- `main.gw`: object definitions plus the runnable batch flow
- `rules_builtin.gw`: built-in rule implementations
- `rules_engine.gw`: rule pipeline assembly and execution
- `rules_report.gw`: decision and batch summary printing
- `rules_seed.gw`: demo requests

## Why This Complements ledger_app

`ledger_app` pressures persistence, filtering, and object mutation.

`rules_app` pressures a different slice:

- Rule composition through function values
- Explicit error propagation in the middle of a pipeline
- `match` as visible control flow instead of hidden polymorphism
- Module reuse in a workflow that is not just data storage

## Expected Outcome

The sample batch intentionally contains:

- one approved request
- two review requests
- one rejected request
- one rule error caused by malformed input
