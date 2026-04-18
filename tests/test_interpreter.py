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


def test_module_private_func_not_importable():
    import pytest
    with pytest.raises(Exception, match="does not export 'helper'"):
        run("""module math_utils
  export func square(x: int) -> int
    return x * x
  endfunc

  func helper(x: int) -> int
    return x + 1
  endfunc
endmodule

use helper from math_utils
write(helper(7))""")


def test_module_namespace_only_exposes_exports():
    import pytest
    with pytest.raises(Exception, match="Undefined variable: helper"):
        run("""module math_utils
  export func square(x: int) -> int
    return x * x
  endfunc

  func helper(x: int) -> int
    return x + 1
  endfunc
endmodule

use math_utils
write(math_utils.helper(7))""")


def test_module_exported_object_importable():
    out = run("""module bank
  export object Account
    balance: int

    new(balance: int) -> Account
      return Account{balance := balance}
    endnew

    func value(self: Account) -> int
      return self.balance
    endfunc
  endobject
endmodule

use Account from bank
acc := Account.new(7)
write(acc.value())""")
    assert out == "7"


def test_module_namespace_exposes_exported_object():
    out = run("""module bank
  export object Account
    balance: int

    new(balance: int) -> Account
      return Account{balance := balance}
    endnew

    func value(self: Account) -> int
      return self.balance
    endfunc
  endobject
endmodule

use bank
acc := bank.Account.new(11)
write(acc.value())""")
    assert out == "11"


def test_module_private_object_not_importable():
    import pytest
    with pytest.raises(Exception, match="does not export 'Account'"):
        run("""module bank
  object Account
    balance: int

    new(balance: int) -> Account
      return Account{balance := balance}
    endnew
  endobject
endmodule

use Account from bank
acc := Account.new(7)""")


def test_module_namespace_hides_private_object():
    import pytest
    with pytest.raises(Exception, match="Undefined variable: Account"):
        run("""module bank
  object Account
    balance: int

    new(balance: int) -> Account
      return Account{balance := balance}
    endnew
  endobject
endmodule

use bank
acc := bank.Account.new(7)""")


def test_module_private_object_does_not_leak_globally():
    import pytest
    with pytest.raises(Exception, match="Undefined variable: Account"):
        run("""module bank
  object Account
    balance: int

    new(balance: int) -> Account
      return Account{balance := balance}
    endnew
  endobject
endmodule

acc := Account.new(7)""")


def test_module_exported_type_alias_importable():
    out = run("""module ids
  export type UserId = int8
endmodule

use UserId from ids
id: UserId := 42
write(id)""")
    assert out == "42"


def test_module_private_type_alias_not_importable():
    import pytest
    with pytest.raises(Exception, match="does not export 'UserId'"):
        run("""module ids
  type UserId = int
endmodule

use UserId from ids
id: UserId := 42""")


def test_module_private_type_alias_stays_visible_inside_exported_function():
    import pytest
    with pytest.raises(Exception, match="Overflow"):
        run("""module ids
  type TinyId = int8

  export func echo(id: TinyId) -> TinyId
    return id
  endfunc
endmodule

use echo from ids
write(echo(200))""")


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
  write(typeof(total))
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


def test_removeat_basic():
    out = run("""func main()
  lst := [10, 20, 30, 40]
  removed := removeat(lst, 1)
  write(removed)
  write(lst)
endfunc""")
    lines = out.split("\n")
    assert lines[0] == "20"
    assert lines[1] == "[10, 30, 40]"


def test_removeat_negative_index():
    out = run("""func main()
  lst := [10, 20, 30]
  removed := removeat(lst, -1)
  write(removed)
  write(lst)
endfunc""")
    lines = out.split("\n")
    assert lines[0] == "30"
    assert lines[1] == "[10, 20]"


def test_removeat_out_of_range():
    import pytest
    with pytest.raises(Exception, match="removeat\\(\\) index out of range"):
        run("""func main()
  lst := [1, 2]
  removeat(lst, 5)
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
    """substring uses closed-closed interval [start, end] — both inclusive."""
    out = run("""s := "hello world"
// [0, 4] = "hello" (indices 0,1,2,3,4)
write(substring(s, 0, 4))
// [6, 10] = "world" (indices 6,7,8,9,10)
write(substring(s, 6, 10))
""")
    lines = out.split("\n")
    assert lines[0] == "hello"
    assert lines[1] == "world"


def test_substring_valid_cases():
    """substring() valid cases: closed-closed interval [start, end]."""
    out = run("""s := "abc"
// start == end returns single char at that index
b := substring(s, 1, 1)
// [0, 2] = "abc" (indices 0,1,2)
a := substring(s, 0, 2)
write(a)
write(b)
// [1, 2] = "bc" (indices 1,2)
write(substring(s, 1, 2))
""")
    assert out == "abc\nb\nbc"


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


# --- Dict tests ---

def test_dict_basic():
    out = run('''d := dict[string, int]{"a": 1, "b": 2}
write(d["a"])
write(d["b"])''')
    assert out == "1\n2"


def test_dict_len():
    out = run('''d := dict[string, int]{"x": 1, "y": 2, "z": 3}
write(len(d))''')
    assert out == "3"


def test_dict_empty():
    out = run('''d := dict[string, int]{}
write(len(d))''')
    assert out == "0"


def test_dict_write_new_key():
    out = run('''d := dict[string, int]{"a": 1}
d["b"] := 2
write(d["a"])
write(d["b"])
write(len(d))''')
    assert out == "1\n2\n2"


def test_dict_overwrite_key():
    out = run('''d := dict[string, int]{"a": 1}
d["a"] := 99
write(d["a"])''')
    assert out == "99"


def test_dict_missing_key_errors():
    import pytest
    from gwen.interpreter import GwenError
    with pytest.raises(GwenError, match="Key not found"):
        run('''d := dict[string, int]{"a": 1}
write(d["missing"])''')


def test_dict_haskey():
    out = run('''d := dict[string, int]{"a": 1}
write(haskey(d, "a"))
write(haskey(d, "z"))''')
    assert out == "True\nFalse"


def test_dict_get_with_default():
    out = run('''d := dict[string, int]{"a": 1}
write(get(d, "a", 0))
write(get(d, "missing", 99))''')
    assert out == "1\n99"


def test_dict_keys_values():
    out = run('''d := dict[string, int]{"a": 1, "b": 2}
write(keys(d))
write(values(d))''')
    assert out == "['a', 'b']\n[1, 2]"


def test_dict_int_keys():
    out = run('''d := dict[int, string]{1: "one", 2: "two"}
write(d[1])
write(d[2])''')
    assert out == "one\ntwo"


def test_dict_trailing_comma():
    out = run('''d := dict[string, int]{
  "a": 1,
  "b": 2,
}
write(len(d))''')
    assert out == "2"


def test_dict_type():
    out = run('''d := dict[string, int]{"a": 1}
write(typeof(d))''')
    assert out == "dict"


def test_dict_iteration_via_keys():
    out = run('''d := dict[string, int]{"a": 1, "b": 2}
total := 0
for k in keys(d) do
  total := total + d[k]
endfor
write(total)''')
    assert out == "3"


# --- File I/O tests ---

import tempfile

def test_file_write_and_read():
    with tempfile.NamedTemporaryFile(delete=False, suffix=".gw_test") as f:
        path = f.name
    try:
        out = run(f'''match writefile("{path}", "hello gwen")
  when ok(n) => write("wrote", n)
  when err(e) => write("fail:", e)
endmatch
match readfile("{path}")
  when ok(s) => write("read:", s)
  when err(e) => write("fail:", e)
endmatch''')
        assert out == "wrote 10\nread: hello gwen"
    finally:
        os.unlink(path)


def test_file_append():
    with tempfile.NamedTemporaryFile(delete=False, suffix=".gw_test") as f:
        path = f.name
    try:
        out = run(f'''match writefile("{path}", "abc")
  when ok(n) => n
  when err(e) => write("fail")
endmatch
match appendfile("{path}", "def")
  when ok(n) => write("app", n)
  when err(e) => write("fail")
endmatch
match readfile("{path}")
  when ok(s) => write(s)
  when err(e) => write("fail")
endmatch''')
        assert out == "app 3\nabcdef"
    finally:
        os.unlink(path)


def test_file_read_missing_returns_err():
    out = run('''match readfile("/tmp/gwen_definitely_missing_xxxyyyzzz_9999.txt")
  when ok(s) => write("unexpected")
  when err(e) => write("got err")
endmatch''')
    assert out == "got err"


def test_file_write_bytes_count_utf8():
    """writefile returns ok with UTF-8 byte count, not char count."""
    with tempfile.NamedTemporaryFile(delete=False, suffix=".gw_test") as f:
        path = f.name
    try:
        out = run(f'''match writefile("{path}", "你好")
  when ok(n) => write(n)
  when err(e) => write("fail")
endmatch''')
        # 你好 = 6 bytes in UTF-8 (3 bytes each)
        assert out == "6"
    finally:
        os.unlink(path)


def test_file_read_utf8():
    with tempfile.NamedTemporaryFile(delete=False, suffix=".gw_test") as f:
        path = f.name
    try:
        out = run(f'''match writefile("{path}", "中文测试")
  when ok(n) => n
  when err(e) => write("fail")
endmatch
match readfile("{path}")
  when ok(s) => write(s)
  when err(e) => write("fail")
endmatch''')
        assert out == "中文测试"
    finally:
        os.unlink(path)


def test_file_write_overwrites():
    """writefile overwrites, not appends."""
    with tempfile.NamedTemporaryFile(delete=False, suffix=".gw_test") as f:
        path = f.name
    try:
        out = run(f'''match writefile("{path}", "first")
  when ok(n) => n
  when err(e) => write("fail")
endmatch
match writefile("{path}", "second")
  when ok(n) => n
  when err(e) => write("fail")
endmatch
match readfile("{path}")
  when ok(s) => write(s)
  when err(e) => write("fail")
endmatch''')
        assert out == "second"
    finally:
        os.unlink(path)


def test_file_write_to_bad_path_returns_err():
    out = run('''match writefile("/this/path/does/not/exist/at/all.txt", "x")
  when ok(n) => write("unexpected")
  when err(e) => write("got err")
endmatch''')
    assert out == "got err"


# ---------- match exhaustiveness on Result ----------

def test_match_result_with_non_ok_err_pattern_rejected():
    """match on Result must use ok(x)/err(x), not literals or ranges."""
    import pytest
    with pytest.raises(Exception, match="Match on Result type must use ok"):
        run('''func f() -> result[int]
  return ok(1)
endfunc

match f()
  when 1 => write("one")
  else write("other")
endmatch''')


def test_match_result_only_ok_raises_on_err_value():
    """match with only when ok(x) raises at runtime if value is err."""
    import pytest
    with pytest.raises(Exception, match="no matching case"):
        run('''func f() -> result[int]
  return err("boom")
endfunc

match f()
  when ok(n) => write(n)
endmatch''')


def test_match_result_only_err_raises_on_ok_value():
    """match with only when err(e) raises at runtime if value is ok."""
    import pytest
    with pytest.raises(Exception, match="no matching case"):
        run('''func f() -> result[int]
  return ok(42)
endfunc

match f()
  when err(e) => write(e)
endmatch''')


def test_match_result_ok_plus_else_covers_err():
    """match with when ok(x) + else is exhaustive (else covers err)."""
    out = run('''func f() -> result[int]
  return err("boom")
endfunc

match f()
  when ok(n) => write(n)
  else write("fallback")
endmatch''')
    assert out == "fallback"


# ---------- Strict bool conditions (Go-style, no truthiness) ----------

def test_if_non_bool_int_rejected():
    """if condition must be bool, not int."""
    import pytest
    with pytest.raises(Exception, match="'if' condition must be bool, got int"):
        run('''x := 1
if x then write("yes") endif''')


def test_if_non_bool_string_rejected():
    """if condition must be bool, not string."""
    import pytest
    with pytest.raises(Exception, match="'if' condition must be bool, got str"):
        run('''s := "hi"
if s then write("yes") endif''')


def test_if_non_bool_list_rejected():
    """if condition must be bool, not list."""
    import pytest
    with pytest.raises(Exception, match="'if' condition must be bool, got list"):
        run('''lst := [1, 2, 3]
if lst then write("yes") endif''')


def test_elif_non_bool_rejected():
    """elif condition must be bool too."""
    import pytest
    with pytest.raises(Exception, match="'elif' condition must be bool"):
        run('''x := 5
if x = 0 then
  write("zero")
elif x then
  write("nonzero")
endif''')


def test_while_non_bool_rejected():
    """while condition must be bool."""
    import pytest
    with pytest.raises(Exception, match="'while' condition must be bool, got int"):
        run('''n := 3
while n do
  n := n - 1
endwhile''')


def test_not_non_bool_rejected():
    """'not' operand must be bool."""
    import pytest
    with pytest.raises(Exception, match="'not' operand must be bool, got int"):
        run('''x := 0
if not x then write("hi") endif''')


def test_and_non_bool_left_rejected():
    """'and' left side must be bool."""
    import pytest
    with pytest.raises(Exception, match="left side of 'and' must be bool"):
        run('''x := 1
if x and true then write("hi") endif''')


def test_and_non_bool_right_rejected():
    """'and' right side must be bool when left is true."""
    import pytest
    with pytest.raises(Exception, match="right side of 'and' must be bool"):
        run('''x := 1
if true and x then write("hi") endif''')


def test_or_non_bool_left_rejected():
    """'or' left side must be bool."""
    import pytest
    with pytest.raises(Exception, match="left side of 'or' must be bool"):
        run('''x := 0
if x or true then write("hi") endif''')


def test_and_short_circuit_skips_right():
    """'and' must short-circuit: right not evaluated when left is false."""
    # If right side were evaluated, division by zero would crash.
    out = run('''x := 0
if x != 0 and 100 / x > 5 then
  write("never")
else
  write("safe")
endif''')
    assert out == "safe"


def test_or_short_circuit_skips_right():
    """'or' must short-circuit: right not evaluated when left is true."""
    out = run('''x := 0
if x = 0 or 100 / x > 5 then
  write("safe")
else
  write("never")
endif''')
    assert out == "safe"


def test_as_bool_from_int_rejected():
    """'as bool' rejects non-bool (no truthiness conversion)."""
    import pytest
    with pytest.raises(Exception, match="Cannot convert int to bool"):
        run('''x := 1
b := x as bool
write(b)''')


def test_as_bool_from_bool_works():
    """'as bool' on a bool is a no-op (returns ok(bool))."""
    out = run('''match true as bool
  when ok(b) => write(b)
  when err(e) => write("err")
endmatch''')
    assert out == "True"


# ---------- for-in iterable rules ----------

def test_for_in_string_iterates_chars():
    """for c in string iterates each character (1-char string)."""
    out = run('''for c in "abc" do
  write(c)
endfor''')
    assert out == "a\nb\nc"


def test_for_in_dict_rejected():
    """Cannot iterate directly over a dict; must use keys/values/items."""
    import pytest
    with pytest.raises(Exception, match="Cannot iterate directly over a dict"):
        run('''d := dict[string, int]{"a": 1, "b": 2}
for x in d do write(x) endfor''')


def test_for_in_keys_works():
    """Iterate dict via keys() is the recommended way."""
    out = run('''d := dict[string, int]{"x": 10, "y": 20}
for k in keys(d) do
  write(k)
endfor''')
    # Note: dict iteration order not guaranteed; check both present
    assert set(out.split("\n")) == {"x", "y"}


def test_for_in_values_works():
    """Iterate dict via values()."""
    out = run('''d := dict[string, int]{"x": 10, "y": 20}
for v in values(d) do
  write(v)
endfor''')
    assert set(out.split("\n")) == {"10", "20"}


def test_for_in_items_works():
    """Iterate dict via items() — each pair is a 2-element list."""
    out = run('''d := dict[string, int]{"x": 10, "y": 20}
for pair in items(d) do
  write(pair[0], pair[1])
endfor''')
    lines = set(out.split("\n"))
    assert lines == {"x 10", "y 20"}


def test_for_in_string_with_index():
    """for c in string with index gives char and index."""
    out = run('''for c in "hi" with index i do
  write(i, c)
endfor''')
    assert out == "0 h\n1 i"


def test_items_type_error():
    """items() on a non-dict raises."""
    import pytest
    with pytest.raises(Exception, match=r"items\(\) requires a dict"):
        run('''x := items([1, 2, 3])''')


# ---------- char range for (ASCII single-char strings) ----------

def test_char_range_basic():
    """for c in "a" to "f" iterates ASCII chars."""
    out = run('''for c in "a" to "e" do
  write(c)
endfor''')
    assert out == "a\nb\nc\nd\ne"


def test_char_range_auto_reverse():
    """Char range auto-detects direction (end < start means reverse)."""
    out = run('''for c in "e" to "a" do
  write(c)
endfor''')
    assert out == "e\nd\nc\nb\na"


def test_char_range_explicit_order():
    """Char range with explicit 'order' forces ascending regardless of bounds."""
    out = run('''for c in "e" to "a" order do
  write(c)
endfor''')
    assert out == "a\nb\nc\nd\ne"


def test_char_range_explicit_reverse():
    """Char range with explicit 'reverse' forces descending regardless of bounds."""
    out = run('''for c in "a" to "e" reverse do
  write(c)
endfor''')
    assert out == "e\nd\nc\nb\na"


def test_char_range_digits():
    """Char range works for digit chars too."""
    out = run('''for c in "0" to "5" do
  write(c)
endfor''')
    assert out == "0\n1\n2\n3\n4\n5"


def test_char_range_rejects_non_ascii():
    """Char range rejects non-ASCII chars (Unicode not supported)."""
    import pytest
    with pytest.raises(Exception, match="Char range only supports ASCII"):
        run('''for c in "\u00e9" to "\u00f0" do write(c) endfor''')


# ---------- strict bounds checking for index operations ----------

def test_string_index_basic():
    """string[i] returns the character at index."""
    out = run('''s := "abc"
write(s[0])
write(s[1])
write(s[2])''')
    assert out == "a\nb\nc"


def test_string_index_out_of_bounds():
    """string[i] out of bounds raises GwenError."""
    import pytest
    with pytest.raises(Exception, match="Index out of range"):
        run('''s := "hi"
write(s[10])''')


def test_string_index_negative():
    """string[i] with negative index raises GwenError (no Python-style negative)."""
    import pytest
    with pytest.raises(Exception, match="Index out of range"):
        run('''s := "hi"
write(s[-1])''')


def test_list_index_out_of_bounds():
    """list[i] out of bounds raises GwenError."""
    import pytest
    with pytest.raises(Exception, match="Index out of range"):
        run('''lst := [1, 2, 3]
write(lst[10])''')


def test_substring_bounds_start_negative():
    """substring() with negative start raises GwenError."""
    import pytest
    with pytest.raises(Exception, match=r"substring\(\) start out of bounds"):
        run('''s := "hello"
write(substring(s, -1, 3))''')


def test_substring_bounds_end_over():
    """substring() with end >= length raises GwenError (end is inclusive)."""
    import pytest
    # "hi" has length 2, max valid end is 1 (index of 'i')
    with pytest.raises(Exception, match=r"substring\(\) end out of bounds"):
        run('''s := "hi"
write(substring(s, 0, 2))''')


def test_substring_bounds_start_greater_than_end():
    """substring() with start > end raises GwenError."""
    import pytest
    with pytest.raises(Exception, match=r"substring\(\) start.*> end"):
        run('''s := "hello"
write(substring(s, 4, 2))''')


def test_substring_valid():
    """substring() works normally within bounds (closed-closed interval)."""
    out = run('''s := "hello"
// [1, 4] = indices 1,2,3,4 = "ello"
write(substring(s, 1, 4))''')
    assert out == "ello"


# ---------- list + operator (concatenation) ----------

def test_list_plus_concatenation():
    """list + list concatenates and returns new list."""
    out = run('''a := [1, 2]
b := [3, 4]
c := a + b
write(len(c))
write(c[0])
write(c[3])''')
    assert out == "4\n1\n4"


def test_list_plus_preserves_originals():
    """list + list does not modify original lists (returns new list)."""
    out = run('''a := [1, 2]
b := [3, 4]
c := a + b
// originals unchanged
write(len(a))
write(len(b))''')
    assert out == "2\n2"


# ---------- money * float rejection ----------

def test_money_times_float_rejected():
    """money[T] * float is rejected — use explicit as float instead."""
    import pytest
    with pytest.raises(Exception, match="Cannot multiply money by float"):
        run('''match 10.50 as money[USD]
  when ok(price) =>
    result := price * 1.5
    write(result)
  when err(e) => write("err")
endmatch''')


def test_money_times_int_allowed():
    """money[T] * int is allowed."""
    out = run('''match 10.00 as money[USD]
  when ok(price) =>
    result := price * 3
    write(result)
  when err(e) => write("err")
endmatch''')
    assert out == "30.00 USD"


def test_money_divided_by_float_allowed():
    """money[T] / float is allowed (returns money[T])."""
    out = run('''match 10.00 as money[USD]
  when ok(m) =>
    result := m / 2.5
    write(result)
  when err(e) => write("err")
endmatch''')
    assert out == "4.00 USD"


# ---------- list/dict comparison rejection ----------

def test_list_less_than_list_rejected():
    """list < list is not defined — use explicit element-wise comparison."""
    import pytest
    with pytest.raises(Exception, match="Comparison '<' is not defined for list"):
        run('''a := [1, 2]
b := [1, 3]
write(a < b)''')


def test_dict_comparison_rejected():
    """dict comparisons (<, >, <=, >=) are not defined."""
    import pytest
    with pytest.raises(Exception, match="Comparison.*is not defined for dict"):
        run('''d1 := dict[string, int]{"a": 1}
d2 := dict[string, int]{"b": 2}
write(d1 > d2)''')


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
