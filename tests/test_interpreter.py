"""Tests for Gwen interpreter."""

import sys
import os
import io
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from gwen.parser import parse
from gwen.interpreter import Interpreter, OkValue, ErrValue


def run(source: str) -> str:
    """Run source and capture stdout."""
    program = parse(source)
    interp = Interpreter()
    old_stdout = sys.stdout
    sys.stdout = io.StringIO()
    try:
        interp.run(program)
        return sys.stdout.getvalue().strip()
    finally:
        sys.stdout = old_stdout


def test_hello():
    out = run('write("Hello, Gwen!")')
    assert out == "Hello, Gwen!"


def test_variables():
    out = run("""x := 42
write(x)""")
    assert out == "42"


def test_typed_var():
    out = run("""x: int := 10
write(x)""")
    assert out == "10"


def test_arithmetic():
    out = run('write(2 + 3 * 4)')
    assert out == "14"


def test_mod():
    out = run('write(10 mod 3)')
    assert out == "1"


def test_string_concat():
    out = run('write("hello" + " " + "world")')
    assert out == "hello world"


def test_comparison():
    out = run('write(3 = 3)')
    assert out == "True"


def test_func():
    out = run("""func double(x: int) -> int
  return x * 2
endfunc
write(double(21))""")
    assert out == "42"


def test_func_auto_main():
    out = run("""func main()
  write("from main")
endfunc""")
    assert out == "from main"


def test_default_param():
    out = run("""func greet(name: string, greeting: string = "Hello")
  write(greeting + ", " + name)
endfunc
greet("Gwen")""")
    assert out == "Hello, Gwen"


def test_if():
    out = run("""x := 10
if x > 5 then
  write("big")
else
  write("small")
endif""")
    assert out == "big"


def test_elif():
    out = run("""x := 0
if x > 0 then
  write("positive")
elif x = 0 then
  write("zero")
else
  write("negative")
endif""")
    assert out == "zero"


def test_while():
    out = run("""x := 0
while x < 5 do
  x := x + 1
endwhile
write(x)""")
    assert out == "5"


def test_for_range():
    out = run("""sum := 0
for i in 1 to 5 do
  sum := sum + i
endfor
write(sum)""")
    assert out == "15"


def test_for_range_reverse():
    out = run("""result := ""
for i in 3 to 1 do
  result := result + str(i)
endfor
write(result)""")
    assert out == "321"


def test_for_range_step():
    out = run("""result := ""
for i in 1 to 10 step 3 do
  result := result + str(i) + " "
endfor
write(result)""")
    assert out == "1 4 7 10"


def test_for_each():
    out = run("""items := [10, 20, 30]
sum := 0
for item in items do
  sum := sum + item
endfor
write(sum)""")
    assert out == "60"


def test_match():
    out = run("""x := 2
match x
  when 1 then
    write("one")
  when 2, 3 then
    write("two or three")
  else
    write("other")
endmatch""")
    assert out == "two or three"


def test_match_range():
    out = run("""x := 5
match x
  when 1 to 3 then
    write("low")
  when 4 to 6 then
    write("mid")
  else
    write("high")
endmatch""")
    assert out == "mid"


def test_ok_err():
    out = run("""func safe_div(a: int, b: int)
  if b = 0 then
    return err("division by zero")
  endif
  return ok(a / b)
endfunc

match safe_div(10, 2)
  when ok(result) then
    write(result)
  when err(e) then
    write(e)
endmatch""")
    assert out == "5"


def test_ok_err_error_case():
    out = run("""func safe_div(a: int, b: int)
  if b = 0 then
    return err("division by zero")
  endif
  return ok(a / b)
endfunc

match safe_div(10, 0)
  when ok(result) then
    write(result)
  when err(e) then
    write(e)
endmatch""")
    assert out == "division by zero"


def test_gcd():
    out = run("""func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc
write(gcd(48, 18))""")
    assert out == "6"


def test_module():
    out = run("""module math_utils
  export func square(x: int) -> int
    return x * x
  endfunc
endmodule

use square from math_utils
write(square(7))""")
    assert out == "49"


def test_nested_if():
    out = run("""x := 10
y := 20
if x > 5 then
  if y > 15 then
    write("both")
  endif
endif""")
    assert out == "both"


def test_lambda():
    out = run("""double := (x: int) => x * 2
write(double(5))""")
    assert out == "10"


def test_list():
    out = run("""nums := [1, 2, 3]
write(len(nums))""")
    assert out == "3"


def test_tag_no_effect():
    out = run("""@setup
x := 42
write(x)""")
    assert out == "42"


def test_multi_assign_swap():
    out = run("""a := 1
b := 2
a, b := b, a
write(a)
write(b)""")
    assert out == "2\n1"


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
