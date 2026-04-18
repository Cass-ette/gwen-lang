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
        interp.run(program)
        return sys.stdout.getvalue().strip()
    finally:
        sys.stdout = old_stdout


def test_ledger_app_example_runs():
    tmp_path = Path("/tmp/gwen_ledger_app_demo.txt")
    if tmp_path.exists():
        tmp_path.unlink()

    out = run_file(Path("examples/ledger_app/main.gw"))

    assert "== Gwen Ledger App ==" in out
    assert "Entry count: 6" in out
    assert "Saved bytes:" in out
    assert "Reloaded entries: 6" in out
    assert "Balance: 4933.51" in out

    if tmp_path.exists():
        tmp_path.unlink()
