# Gwen Ledger App Tutorial

This tutorial is for first-time Gwen readers.

Goal:

1. run a real Gwen project
2. understand how the files connect
3. make one or two small edits yourself

## 1. Run It First

### Fast path

From the repository root:

```bash
./scripts/try_ledger_app.sh
```

That will:

1. create a local virtual environment
2. install Gwen from this repository
3. run `gwen check`
4. run the ledger app

### Manual path

```bash
python3 -m pip install -e .
gwen check examples/ledger_app/main.gw
gwen examples/ledger_app/main.gw
```

## 2. What The Program Does

The app:

1. creates a ledger
2. seeds demo income and expense entries
3. prints summaries and filtered queries
4. deletes one entry
5. updates one note
6. saves the ledger to a text file
7. reloads the file and prints the data again

This makes it a good sample because it is not just an algorithm demo. It has data, modules, objects, file I/O, and visible program flow.

## 3. Read The Code In This Order

### Step 1: start from the entry point

Read:

- `examples/ledger_app/main.gw`

What to look for:

- `object Entry`
- `object Ledger`
- `func main()`

This file defines the main data model and the top-level runnable flow.

### Step 2: see where the demo data comes from

Read:

- `examples/ledger_app/ledger_seed.gw`

What to look for:

- `seed_demo_data(ledger)`

This is the easiest file to edit first.

### Step 3: see how queries are printed

Read:

- `examples/ledger_app/ledger_queries.gw`
- `examples/ledger_app/ledger_view.gw`

What to look for:

- exported helper functions
- `use ... from ...`
- how `list[string]` is turned into visible output

### Step 4: see how reports are built

Read:

- `examples/ledger_app/ledger_report.gw`

What to look for:

- category totals
- sorting
- reuse of `print_lines`

### Step 5: see persistence

Read:

- `examples/ledger_app/ledger_store.gw`

What to look for:

- `readfile` / `writefile`
- `result[int]`
- `match ok(...) / err(...)`

## 4. First Three Edits To Try

These are the easiest ways to learn the project.

### Edit 1: add one more entry

Open:

- `examples/ledger_app/ledger_seed.gw`

Add one more line such as:

```gwen
ledger.add_expense("2026-04-07", "books", 25.0, "parser book")
```

Run again and see:

- entry count changes
- category totals change
- April report changes

### Edit 2: change a query

Open:

- `examples/ledger_app/main.gw`

Find:

```gwen
print_query_results(ledger, "Filtered expenses in food:", "food", "expense")
```

Change `"food"` to `"software"` or `"refund"` and run again.

### Edit 3: change the updated note text

In `examples/ledger_app/main.gw`, find the note update flow and replace the new note text with your own text.

Then run again and check:

- the "Updated note search" output
- the reloaded output after saving and loading

## 5. What Gwen Features This Project Teaches

This project is a compact tour of current Gwen:

- objects with explicit methods
- private fields accessed through methods
- multi-file modules with `use ... from ...`
- lists and dictionaries
- result-based file I/O
- `match` for visible error handling
- explicit, readable program flow

## 6. A Good Reading Question

After reading the project, ask:

> Which parts feel clear, and which parts feel too verbose or too limited?

That is the real purpose of this sample. It is not only a demo app. It is also a pressure test for Gwen's current language design.
