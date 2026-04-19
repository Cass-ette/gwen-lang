"""Smoke tests for runnable Gwen examples."""

import io
import os
import sys
from pathlib import Path

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from gwen.interpreter import Interpreter
from gwen.parser import parse


def run_file(path: Path) -> str:
    program = parse(path.read_text())
    interp = Interpreter()
    old_stdout = sys.stdout
    sys.stdout = io.StringIO()
    try:
        interp.run(program, source_path=str(path))
        return sys.stdout.getvalue().strip()
    finally:
        sys.stdout = old_stdout


def test_ledger_app_example_runs():
    tmp_path = Path("/tmp/gwen_ledger_app_demo.txt")
    if tmp_path.exists():
        tmp_path.unlink()

    out = run_file(Path("examples/ledger_app/main.gw"))

    assert "== Gwen Ledger App ==" in out
    assert "Entry count: 7" in out
    assert "Filtered expenses in food:" in out
    assert "2 | 2026-04-02 | expense | food | 32.5 | lunch with team" in out
    assert "4 | 2026-04-04 | expense | food | 18.0 | coffee beans" in out
    assert "6 | 2026-05-01 | expense | food | 40.0 | next month groceries" in out
    assert "April food expenses:" in out
    assert "Entries mentioning domain:" in out
    assert "Deleted entries: 1" in out
    assert "Updated notes: 1" in out
    assert "Summary after delete:" in out
    assert "Entry count: 6" in out
    assert "April report:" in out
    assert "Month report: 2026-04" in out
    assert "May report:" in out
    assert "Month report: 2026-05" in out
    assert "Saved bytes:" in out
    assert "Reloaded entries: 6" in out
    assert "Balance: 4893.51" in out
    assert "Balance: 4901.51" in out
    assert "Balance: 4941.51" in out
    assert "Balance: -40.0" in out
    assert "Updated note search for renewed:" in out
    assert "Reloaded software expenses:" in out
    assert "Reloaded note search for renewed:" in out
    assert "renewed domain and DNS" in out

    if tmp_path.exists():
        tmp_path.unlink()


def test_rules_app_example_runs():
    out = run_file(Path("examples/rules_app/main.gw"))

    assert "== Gwen Rules App ==" in out
    assert "Request: REQ-100 | Alice | SG | trusted | books | 2500.0" in out
    assert "Decision: approve" in out
    assert "Decision: review" in out
    assert "Decision: reject" in out
    assert "Decision: error" in out
    assert "rule[0] failed for REQ-104: category missing" in out
    assert "note: vip customer" not in out
    assert "review: amount over manual-review threshold" in out
    assert "review: missing documents" in out
    assert "review: new customer" in out
    assert "reject: country blocked: KP" in out
    assert "Summary:" in out
    assert " approve = 1" in out
    assert " review = 2" in out
    assert " reject = 1" in out
    assert " error = 1" in out


def test_higher_order_example_runs():
    out = run_file(Path("examples/higher_order.gw"))

    assert "Original: [1, 2, 3, 4, 5]" in out
    assert "Doubled: [2, 4, 6, 8, 10]" in out
    assert "Evens: [2, 4]" in out
    assert "Indexed: [[0, 'a'], [1, 'b'], [2, 'c']]" in out
    assert "Sum: 15" in out
    assert "Product: 120" in out
