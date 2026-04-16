"""Tests for Gwen index assignment."""

import sys
import os
import io
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from gwen.parser import parse
from gwen.interpreter import Interpreter


def run(source: str) -> str:
    program = parse(source)
    interp = Interpreter()
    old_stdout = sys.stdout
    sys.stdout = io.StringIO()
    try:
        interp.run(program)
        return sys.stdout.getvalue().strip()
    finally:
        sys.stdout = old_stdout


def test_index_assign_basic():
    out = run("""arr := [10, 20, 30]
arr[1] := 99
write(arr[0])
write(arr[1])
write(arr[2])""")
    assert out == "10\n99\n30"


def test_index_assign_in_loop():
    out = run("""arr := [1, 2, 3, 4, 5]
for i in 0 to 4 do
  arr[i] := arr[i] * 2
endfor
for i in 0 to 4 do
  write(arr[i])
endfor""")
    assert out == "2\n4\n6\n8\n10"


if __name__ == "__main__":
    tests = [v for k, v in globals().items() if k.startswith("test_")]
    passed = 0
    failed = 0
    for test in tests:
        try:
            test()
            print(f"  PASS  {test.__name__}")
            passed += 1
        except Exception as e:
            print(f"  FAIL  {test.__name__}: {e}")
            failed += 1
    print(f"\n{passed} passed, {failed} failed")
