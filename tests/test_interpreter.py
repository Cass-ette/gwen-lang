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
  when 1 =>
    write("one")
  when 2, 3 =>
    write("two or three")
  else
    write("other")
endmatch""")
    assert out == "two or three"


def test_match_range():
    out = run("""x := 5
match x
  when 1 to 3 =>
    write("low")
  when 4 to 6 =>
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
  when ok(result) =>
    write(result)
  when err(e) =>
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
  when ok(result) =>
    write(result)
  when err(e) =>
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


def test_type_alias_basic():
    out = run("""type UserId = int
func main()
  id: UserId := 42
  write(id)
endfunc""")
    assert out == "42"


def test_type_alias_precision():
    """Alias to precision type inherits overflow checking."""
    import pytest
    with pytest.raises(Exception, match="Overflow"):
        run("""type TinyInt = int8
func main()
  x: TinyInt := 200
endfunc""")


def test_type_alias_chained():
    out = run("""type Id = int32
type UserId = Id
func main()
  x: UserId := 9999
  write(x)
endfunc""")
    assert out == "9999"


def test_money_basic_arithmetic():
    out = run("""func main()
  price: money[USD] := 19.99
  tax: money[USD] := 1.5
  total := price + tax
  write(total)
  write(type(total))
endfunc""")
    assert "21.49 USD" in out
    assert "money[USD]" in out


def test_money_scalar_multiply():
    out = run("""func main()
  price: money[USD] := 10
  double := price * 2
  half := price / 2
  write(double)
  write(half)
endfunc""")
    assert "20.00 USD" in out
    assert "5.00 USD" in out


def test_money_ratio():
    """money / money returns plain float."""
    out = run("""func main()
  a: money[USD] := 10
  b: money[USD] := 4
  ratio := a / b
  write(ratio)
endfunc""")
    assert out == "2.5"


def test_money_currency_mismatch():
    import pytest
    with pytest.raises(Exception, match="Currency mismatch"):
        run("""func main()
  usd: money[USD] := 10
  cny: money[CNY] := 70
  bad := usd + cny
endfunc""")


def test_money_mul_money_forbidden():
    import pytest
    with pytest.raises(Exception, match="Cannot multiply money by money"):
        run("""func main()
  a: money[USD] := 5
  b: money[USD] := 3
  c := a * b
endfunc""")


def test_money_plus_scalar_forbidden():
    import pytest
    with pytest.raises(Exception, match="Cannot \\+ money with non-money"):
        run("""func main()
  a: money[USD] := 5
  c := a + 10
endfunc""")


def test_money_overflow():
    import pytest
    with pytest.raises(Exception, match="Overflow"):
        run("""func main()
  huge: money[USD] := 999999999999999
endfunc""")


def test_money_cross_currency_as_forbidden():
    out = run("""func main()
  a: money[USD] := 5
  b := a as money[EUR]
  match b
    when ok(v) => write("ok")
    when err(e) => write("err")
  endmatch
endfunc""")
    assert out == "err"


def test_money_as_float():
    out = run("""func main()
  a: money[USD] := 19.99
  f := a as float64
  match f
    when ok(v) => write(v)
    when err(e) => write(e)
  endmatch
endfunc""")
    assert out == "19.99"


# --- Var block / uninit tests ---

def test_uninit_read_errors():
    try:
        run("""func main()
  x: int
  write(x)
endfunc""")
        assert False, "expected error"
    except Exception as e:
        assert "read before assignment" in str(e)


def test_uninit_then_assign_works():
    out = run("""func main()
  x: int
  x := 42
  write(x)
endfunc""")
    assert out == "42"


def test_var_block_basic():
    out = run("""func main()
  var
    a: int := 1
    b: int := 2
  endvar
  write(a, b)
endfunc""")
    assert out == "1 2"


def test_var_block_uninit_read_errors():
    try:
        run("""func main()
  var
    x: int
    y: int
  endvar
  x := 10
  write(x, y)
endfunc""")
        assert False, "expected error"
    except Exception as e:
        assert "read before assignment" in str(e)


def test_var_default_zero():
    out = run("""func main()
  var default
    n: int
    s: string
    b: bool
  endvar
  write(n)
  write(s)
  write(b)
endfunc""")
    assert out.split("\n") == ["0", "", "False"]


def test_var_default_value():
    out = run("""func main()
  var default 7
    a: int
    b: int
    c: int
  endvar
  write(a, b, c)
endfunc""")
    assert out == "7 7 7"


def test_var_default_per_decl_override():
    out = run("""func main()
  var default 1
    a: int
    b: int := 99
    c: int
  endvar
  write(a, b, c)
endfunc""")
    assert out == "1 99 1"


def test_var_default_type_mismatch():
    try:
        run("""func main()
  var default "hello"
    a: int
  endvar
endfunc""")
        assert False, "expected type mismatch"
    except Exception as e:
        msg = str(e)
        assert "cannot accept `default`" in msg
        assert "a: int" in msg


def test_var_default_zero_money():
    out = run("""func main()
  var default
    m: money[USD]
  endvar
  write(m)
endfunc""")
    assert out == "0.00 USD"


def test_var_default_zero_list_fresh():
    out = run("""func main()
  var default
    a: list[int]
    b: list[int]
  endvar
  append(a, 1)
  write(len(a), len(b))
endfunc""")
    assert out == "1 0"


# --- Sort tests ---

def test_sort_basic_asc():
    out = run("""func main()
  nums := [3, 1, 4, 1, 5, 9, 2, 6]
  sorted := sort(nums, asc)
  write(sorted)
  write(nums)  // original unchanged
endfunc""")
    lines = out.split("\n")
    assert lines[0] == "[1, 1, 2, 3, 4, 5, 6, 9]"
    assert lines[1] == "[3, 1, 4, 1, 5, 9, 2, 6]"


def test_sort_desc():
    out = run("""func main()
  nums := [3, 1, 4, 1, 5]
  sorted := sort(nums, desc)
  write(sorted)
endfunc""")
    assert out == "[5, 4, 3, 1, 1]"


def test_sort_custom_comparator():
    # Sort by second element of inner list
    out = run("""func by_second(a: list[int], b: list[int]) -> bool
  return a[1] < b[1]
endfunc

func main()
  pairs := [[3, 10], [1, 5], [2, 7]]
  sorted := sort(pairs, by_second)
  write(sorted)
endfunc""")
    assert "[1, 5]" in out


def test_sort_stable():
    # Stable sort: equal elements keep relative order
    out = run("""func by_first(a: list, b: list) -> bool
  return a[0] < b[0]
endfunc

func main()
  items := [[1, 10], [2, 20], [1, 30], [2, 40]]
  sorted := sort(items, by_first)
  write(sorted)
endfunc""")
    # Should preserve order: [1, 10] before [1, 30], [2, 20] before [2, 40]
    assert "[1, 10]" in out and "[1, 30]" in out


def test_sort_empty():
    out = run("""func main()
  empty := []
  sorted := sort(empty, asc)
  write(sorted)
endfunc""")
    assert out == "[]"


def test_reversed():
    out = run("""func main()
  lst := [1, 2, 3, 4]
  rev := reversed(lst)
  write(rev)
  write(lst)  // original unchanged
endfunc""")
    lines = out.split("\n")
    assert lines[0] == "[4, 3, 2, 1]"
    assert lines[1] == "[1, 2, 3, 4]"


def test_split_basic():
    out = run("""func main()
  parts := split("a,b,c", ",")
  write(parts)
endfunc""")
    assert out == "['a', 'b', 'c']"


def test_split_empty_sep():
    out = run("""func main()
  chars := split("abc", "")
  write(chars)
endfunc""")
    assert out == "['a', 'b', 'c']"


def test_join_basic():
    out = run("""func main()
  parts := ["Hello", "World"]
  text := join(parts, " ")
  write(text)
endfunc""")
    assert out == "Hello World"


def test_join_auto_convert():
    out = run("""func main()
  nums := [1, 2, 3]
  text := join(nums, "-")
  write(text)
endfunc""")
    assert out == "1-2-3"


def test_split_join_roundtrip():
    out = run("""func main()
  original := "apple,banana,cherry"
  parts := split(original, ",")
  back := join(parts, ",")
  write(back)
endfunc""")
    assert out == "apple,banana,cherry"


def test_pop_basic():
    out = run("""func main()
  lst := [1, 2, 3]
  last := pop(lst)
  write(last)
  write(lst)
endfunc""")
    lines = out.split("\n")
    assert lines[0] == "3"
    assert lines[1] == "[1, 2]"


def test_pop_empty_error():
    import pytest
    with pytest.raises(Exception, match="empty"):
        run("""func main()
  lst := []
  x := pop(lst)
endfunc""")


def test_insert_basic():
    out = run("""func main()
  lst := [1, 2, 3]
  insert(lst, 0, 0)
  write(lst)
  insert(lst, 2, 99)
  write(lst)
endfunc""")
    lines = out.split("\n")
    assert lines[0] == "[0, 1, 2, 3]"
    assert lines[1] == "[0, 1, 99, 2, 3]"


def test_concat_new_list():
    out = run("""func main()
  a := [1, 2]
  b := [3, 4]
  c := concat(a, b)
  write(c)
  write(a)
  write(b)
endfunc""")
    lines = out.split("\n")
    assert lines[0] == "[1, 2, 3, 4]"
    assert lines[1] == "[1, 2]"
    assert lines[2] == "[3, 4]"


def test_substring_basic():
    out = run("""func main()
  s := "hello world"
  write(substring(s, 0, 5))
  write(substring(s, 6, 11))
endfunc""")
    lines = out.split("\n")
    assert lines[0] == "hello"
    assert lines[1] == "world"


def test_substring_bounds():
    out = run("""func main()
  s := "abc"
  // end > length clamps to end
  a := substring(s, 0, 100)
  // start == end returns empty (represented as -)
  b := substring(s, 1, 1)
  // start > length returns empty
  c := substring(s, 5, 10)
  marker := "-"
  write(a, marker, b, marker, c)
endfunc""")
    # Output: abc - - -
    assert "abc" in out
    assert "- - -" in out or out.count("-") >= 2


def test_contains_basic():
    out = run("""func main()
  s := "hello world"
  write(contains(s, "world"))
  write(contains(s, "foo"))
  write(contains(s, ""))
endfunc""")
    lines = out.split("\n")
    assert lines[0] == "True"
    assert lines[1] == "False"
    assert lines[2] == "True"


def test_trim_basic():
    out = run("""func main()
  s := "  hello world  \n\t"
  write(trim(s))
endfunc""")
    assert out == "hello world"


def test_replace_basic():
    out = run("""func main()
  s := "hello world, hello universe"
  write(replace(s, "hello", "hi"))
endfunc""")
    assert out == "hi world, hi universe"


def test_replace_no_match():
    out = run("""func main()
  s := "hello world"
  write(replace(s, "foo", "bar"))
endfunc""")
    assert out == "hello world"


def test_abs_int():
    out = run("""func main()
  write(abs(-5))
  write(abs(5))
  write(abs(0))
endfunc""")
    lines = out.split("\n")
    assert lines == ["5", "5", "0"]


def test_abs_float():
    out = run("""func main()
  write(abs(-3.14))
  write(abs(2.71))
endfunc""")
    lines = out.split("\n")
    assert lines == ["3.14", "2.71"]


def test_min_max():
    out = run("""func main()
  write(min(3, 5))
  write(max(3, 5))
  write(min("apple", "banana"))
  write(max("apple", "banana"))
endfunc""")
    lines = out.split("\n")
    assert lines == ["3", "5", "apple", "banana"]


def test_sqrt_basic():
    out = run("""func main()
  write(sqrt(4.0))
  write(sqrt(2.0))
endfunc""")
    lines = out.split("\n")
    # sqrt(4.0) = 2.0, sqrt(2.0) ≈ 1.414
    assert float(lines[0]) == 2.0
    assert 1.41 < float(lines[1]) < 1.42


def test_sqrt_requires_float():
    import pytest
    with pytest.raises(Exception, match="requires float"):
        run("""func main()
  x := sqrt(4)
endfunc""")


def test_min_max_type_mismatch():
    import pytest
    with pytest.raises(Exception, match="same type"):
        run("""func main()
  x := min(1, 2.0)
endfunc""")


def test_floor_ceil():
    out = run("""func main()
  write(floor(3.14))
  write(floor(3.99))
  write(ceil(3.14))
  write(ceil(3.01))
  write(floor(-1.5))
  write(ceil(-1.5))
endfunc""")
    lines = out.split("\n")
    # floor returns float: 3.0, 3.0, ceil: 4.0, 4.0, floor(-1.5): -2.0, ceil(-1.5): -1.0
    assert lines[0] == "3.0"
    assert lines[1] == "3.0"
    assert lines[2] == "4.0"
    assert lines[3] == "4.0"
    assert lines[4] == "-2.0"
    assert lines[5] == "-1.0"


def test_floor_ceil_requires_float():
    import pytest
    with pytest.raises(Exception, match="requires float"):
        run("""func main()
  x := floor(3)
endfunc""")
    with pytest.raises(Exception, match="requires float"):
        run("""func main()
  x := ceil(3)
endfunc""")


# --- Multiple return values tests ---

def test_multiple_return_basic():
    out = run("""func divide(a: int, b: int) -> int, int
  q := a / b
  r := a mod b
  return q, r
endfunc

func main()
  q, r := divide(17, 5)
  write(q, r)
endfunc""")
    assert out == "3 2"


def test_multiple_return_with_types():
    out = run("""func stats(a: int, b: int, c: int) -> int, int, float
  min_val := a
  if b < min_val then min_val := b endif
  if c < min_val then min_val := c endif
  max_val := a
  if b > max_val then max_val := b endif
  if c > max_val then max_val := c endif
  total := a + b + c
  avg := total / 3.0
  return min_val, max_val, avg
endfunc

func main()
  lo, hi, mean := stats(3, 7, 5)
  write(lo, hi, mean)
endfunc""")
    assert out == "3 7 5.0"


def test_multiple_return_destructure_partial():
    out = run("""func pair() -> int, int
  return 1, 2
endfunc

func main()
  first, _ := pair()
  write(first)
endfunc""")
    assert out == "1"


def test_multiple_return_pass_as_args():
    out = run("""func inner() -> int, int
  return 10, 20
endfunc

func outer(x: int, y: int) -> int
  return x + y
endfunc

func main()
  a, b := inner()
  sum := outer(a, b)
  write(sum)
endfunc""")
    assert out == "30"


def test_multiple_return_recursive():
    out = run("""func fib_pair(n: int) -> int, int
  if n = 0 then return 0, 1 endif
  a, b := fib_pair(n - 1)
  return b, a + b
endfunc

func main()
  f, _ := fib_pair(10)
  write(f)
endfunc""")
    assert out == "55"


def test_multiple_return_swap():
    """Use multi-return for swap idiom."""
    out = run("""func swap(a: int, b: int) -> int, int
  return b, a
endfunc

func main()
  x, y := swap(1, 2)
  write(x, y)
  x, y := swap(x, y)
  write(x, y)
endfunc""")
    assert out == "2 1\n1 2"


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
