"""Tests for restricted object system."""

import sys
import os
import io
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from gwen.parser import parse
from gwen.interpreter import Interpreter, GwenError


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


ACCOUNT_DEF = """
object Account
  balance: float
  owner: string

  new(owner: string, initial: float) -> Account
    return Account{balance := initial, owner := owner}
  endnew

  func deposit(self: Account, amount: float) -> result
    if amount < 0.0 then
      return err("negative")
    endif
    self.balance := self.balance + amount
    return ok(self.balance)
  endfunc

  func get_balance(self: Account) -> float
    return self.balance
  endfunc

  func get_owner(self: Account) -> string
    return self.owner
  endfunc
endobject
"""


def test_object_constructor_and_method():
    out = run(ACCOUNT_DEF + """
acc := Account.new("Alice", 1000.0)
write(acc.get_owner())
write(str(acc.get_balance()))
""")
    assert out == "Alice\n1000.0"


def test_method_mutates_self():
    out = run(ACCOUNT_DEF + """
acc := Account.new("Bob", 100.0)
r := acc.deposit(50.0)
match r
  when ok(b) => write(str(b))
  when err(e) => write("err:" + e)
endmatch
""")
    assert out == "150.0"


def test_static_method_dispatch():
    """Account.deposit(acc, amount) is the desugared form."""
    out = run(ACCOUNT_DEF + """
acc := Account.new("Carol", 0.0)
r := Account.deposit(acc, 25.0)
match r
  when ok(b) => write(str(b))
  when err(e) => write("err:" + e)
endmatch
""")
    assert out == "25.0"


def test_private_field_access_forbidden():
    with pytest.raises(GwenError, match="private field"):
        run(ACCOUNT_DEF + """
acc := Account.new("Dave", 10.0)
write(str(acc.balance))
""")


def test_external_field_assignment_forbidden():
    with pytest.raises(GwenError, match="Cannot assign to field"):
        run(ACCOUNT_DEF + """
acc := Account.new("Eve", 10.0)
acc.balance := 999.0
""")


def test_private_field_access_forbidden_in_free_function_named_self():
    with pytest.raises(GwenError, match="private field"):
        run(ACCOUNT_DEF + """
func leak(self: Account) -> float
  return self.balance
endfunc

acc := Account.new("Mallory", 10.0)
write(str(leak(acc)))
""")


def test_private_field_assignment_forbidden_in_free_function_named_self():
    with pytest.raises(GwenError, match="Cannot assign to field"):
        run(ACCOUNT_DEF + """
func leak(self: Account)
  self.balance := 999.0
endfunc

acc := Account.new("Mallory", 10.0)
leak(acc)
""")


def test_object_literal_missing_field():
    with pytest.raises(GwenError, match="missing fields"):
        run("""
object Pt
  x: int
  y: int
endobject

p := Pt{x := 1}
""")


def test_object_literal_unknown_field():
    with pytest.raises(GwenError, match="has no field"):
        run("""
object Pt
  x: int
  y: int
endobject

p := Pt{x := 1, y := 2, z := 3}
""")


def test_method_returning_value_via_self_field():
    out = run("""
object Counter
  n: int

  new() -> Counter
    return Counter{n := 0}
  endnew

  func inc(self: Counter) -> int
    self.n := self.n + 1
    return self.n
  endfunc
endobject

c := Counter.new()
write(str(c.inc()))
write(str(c.inc()))
write(str(c.inc()))
""")
    assert out == "1\n2\n3"


def test_static_dispatch_wrong_instance_type():
    with pytest.raises(GwenError, match="must be a 'Counter' instance"):
        run("""
object Counter
  n: int
  new() -> Counter
    return Counter{n := 0}
  endnew
  func inc(self: Counter) -> int
    self.n := self.n + 1
    return self.n
  endfunc
endobject

Counter.inc(42)
""")


def test_no_constructor_error():
    with pytest.raises(GwenError, match="no constructor"):
        run("""
object Pt
  x: int
  y: int
endobject

Pt.new(1, 2)
""")


def test_typeof_object_instance():
    out = run("""
object Box
  v: int
  new(x: int) -> Box
    return Box{v := x}
  endnew
endobject

b := Box.new(7)
write(typeof(b))
""")
    assert out == "Box"
