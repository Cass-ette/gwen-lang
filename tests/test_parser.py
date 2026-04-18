"""Tests for Gwen parser."""

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from gwen.parser import parse
from gwen import ast_nodes as ast


def test_assignment():
    prog = parse('x := 42')
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.Assignment)
    assert stmt.targets == ["x"]


def test_typed_var():
    prog = parse('x: int := 42')
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.VarDecl)
    assert stmt.name == "x"
    assert isinstance(stmt.type_name, ast.TypeName)
    assert stmt.type_name.name == "int"


def test_multi_assignment():
    prog = parse('a, b := 1, 2')
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.Assignment)
    assert stmt.targets == ["a", "b"]
    assert len(stmt.values) == 2


def test_func_def():
    prog = parse("""func gcd(a: int, b: int) -> int
  return a
endfunc""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.FuncDef)
    assert stmt.name == "gcd"
    assert len(stmt.params) == 2
    assert isinstance(stmt.return_type, ast.TypeName)
    assert stmt.return_type.name == "int"


def test_func_def_named_end():
    prog = parse("""func gcd(a: int) -> int
  return a
endfunc gcd""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.FuncDef)
    assert stmt.name == "gcd"


def test_if_stmt():
    prog = parse("""if x > 0 then
  y := 1
endif""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.IfStmt)


def test_if_elif_else():
    prog = parse("""if x > 0 then
  y := 1
elif x = 0 then
  y := 0
else
  y := -1
endif""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.IfStmt)
    assert len(stmt.elifs) == 1
    assert len(stmt.else_body) == 1


def test_while():
    prog = parse("""while b != 0 do
  b := b - 1
endwhile""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.WhileStmt)


def test_for_range():
    prog = parse("""for i in 1 to 10 do
  print(i)
endfor""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.ForRangeStmt)
    assert stmt.var == "i"


def test_for_range_step():
    prog = parse("""for i in 1 to 10 step 2 do
  print(i)
endfor""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.ForRangeStmt)
    assert stmt.step is not None


def test_for_each():
    prog = parse("""for item in list do
  print(item)
endfor""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.ForEachStmt)
    assert stmt.var == "item"


def test_match():
    prog = parse("""match x
  when 1 =>
    do_a()
  when 2, 3 =>
    do_b()
  else
    do_c()
endmatch""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.MatchStmt)
    assert len(stmt.cases) == 2
    assert len(stmt.else_body) == 1


def test_module():
    prog = parse("""module math_utils
  export func gcd(a: int) -> int
    return a
  endfunc
endmodule""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.ModuleDef)
    assert stmt.name == "math_utils"
    func = stmt.body[0]
    assert isinstance(func, ast.FuncDef)
    assert func.exported is True


def test_export_object():
    prog = parse("""module bank
  export object Account
    balance: int
  endobject
endmodule""")
    stmt = prog.statements[0]
    obj = stmt.body[0]
    assert isinstance(obj, ast.ObjectDef)
    assert obj.exported is True


def test_export_type_alias():
    prog = parse("""module ids
  export type UserId = int
endmodule""")
    stmt = prog.statements[0]
    alias = stmt.body[0]
    assert isinstance(alias, ast.TypeAlias)
    assert alias.exported is True


def test_use_module():
    prog = parse('use math_utils')
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.UseStmt)
    assert stmt.module == "math_utils"
    assert stmt.names == []


def test_use_from():
    prog = parse('use gcd, lcm from math_utils')
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.UseStmt)
    assert stmt.module == "math_utils"
    assert stmt.names == ["gcd", "lcm"]


def test_parallel():
    prog = parse("""parallel do
  deploy()
endparallel""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.ParallelStmt)
    assert stmt.allow_fail is False
    assert stmt.result_var is None


def test_parallel_allow_fail_result():
    prog = parse("""parallel allowfail => results do
  check()
endparallel""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.ParallelStmt)
    assert stmt.allow_fail is True
    assert stmt.result_var == "results"


def test_tag():
    prog = parse('@validate')
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.TagStmt)
    assert stmt.name == "validate"


def test_func_call():
    prog = parse('print("hello", 42)')
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.ExprStmt)
    assert isinstance(stmt.expr, ast.FuncCall)
    assert len(stmt.expr.args) == 2


def test_binary_ops():
    prog = parse('x := 1 + 2 * 3')
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.Assignment)
    # Should be 1 + (2 * 3) due to precedence
    val = stmt.values[0]
    assert isinstance(val, ast.BinaryOp)
    assert val.op == "+"


def test_ok_err():
    prog = parse("""match readfile()
  when ok(data) =>
    print(data)
  when err(e) =>
    print(e)
endmatch""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.MatchStmt)


def test_default_param():
    prog = parse("""func connect(host: string, port: int = 3306)
  return host
endfunc""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.FuncDef)
    assert stmt.params[1].default is not None


def test_gcd_full():
    source = """func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc"""
    prog = parse(source)
    func = prog.statements[0]
    assert isinstance(func, ast.FuncDef)
    assert func.name == "gcd"
    assert len(func.body) == 2  # while + return


def test_generic_list():
    prog = parse('x: list[int] := [1, 2, 3]')
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.VarDecl)
    assert isinstance(stmt.type_name, ast.GenericType)
    assert stmt.type_name.base == "list"
    assert len(stmt.type_name.params) == 1
    assert stmt.type_name.params[0].name == "int"


def test_generic_dict():
    prog = parse('x: dict[string, int] := []')
    stmt = prog.statements[0]
    assert isinstance(stmt.type_name, ast.GenericType)
    assert stmt.type_name.base == "dict"
    assert len(stmt.type_name.params) == 2


def test_func_type_param():
    prog = parse('func map(f: (int) -> int, arr: list[int]) -> list[int]\n  return arr\nendfunc')
    func = prog.statements[0]
    assert isinstance(func, ast.FuncDef)
    assert isinstance(func.params[0].type_name, ast.FuncType)
    assert len(func.params[0].type_name.param_types) == 1
    assert isinstance(func.return_type, ast.GenericType)


def test_arena():
    prog = parse("""arena test do
  x := 42
endarena""")
    stmt = prog.statements[0]
    assert isinstance(stmt, ast.ArenaStmt)
    assert stmt.name == "test"
    assert len(stmt.body) == 1


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
