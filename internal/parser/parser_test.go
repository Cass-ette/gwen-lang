package parser_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Cass-ette/gwen-lang/internal/ast"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

func mustParse(t *testing.T, source string) *ast.Program {
	t.Helper()

	prog, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return prog
}

func TestAssignment(t *testing.T) {
	prog := mustParse(t, "x := 42")
	stmt, ok := prog.Statements[0].(*ast.Assignment)
	if !ok {
		t.Fatalf("expected *ast.Assignment, got %T", prog.Statements[0])
	}
	if !reflect.DeepEqual(stmt.Targets, []any{"x"}) {
		t.Fatalf("targets mismatch: got %v want %v", stmt.Targets, []any{"x"})
	}
}

func TestTypedVar(t *testing.T) {
	prog := mustParse(t, "x: int := 42")
	stmt, ok := prog.Statements[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("expected *ast.VarDecl, got %T", prog.Statements[0])
	}
	if stmt.Name != "x" {
		t.Fatalf("name mismatch: got %q want %q", stmt.Name, "x")
	}
	typeName, ok := stmt.TypeName.(*ast.TypeName)
	if !ok {
		t.Fatalf("expected *ast.TypeName, got %T", stmt.TypeName)
	}
	if typeName.Name != "int" {
		t.Fatalf("type mismatch: got %q want %q", typeName.Name, "int")
	}
}

func TestMultiAssignment(t *testing.T) {
	prog := mustParse(t, "a, b := 1, 2")
	stmt, ok := prog.Statements[0].(*ast.Assignment)
	if !ok {
		t.Fatalf("expected *ast.Assignment, got %T", prog.Statements[0])
	}
	if !reflect.DeepEqual(stmt.Targets, []any{"a", "b"}) {
		t.Fatalf("targets mismatch: got %v want %v", stmt.Targets, []any{"a", "b"})
	}
	if len(stmt.Values) != 2 {
		t.Fatalf("values length mismatch: got %d want %d", len(stmt.Values), 2)
	}
}

func TestFuncDef(t *testing.T) {
	prog := mustParse(t, "func gcd(a: int, b: int) -> int\n  return a\nendfunc")
	stmt, ok := prog.Statements[0].(*ast.FuncDef)
	if !ok {
		t.Fatalf("expected *ast.FuncDef, got %T", prog.Statements[0])
	}
	if stmt.Name != "gcd" {
		t.Fatalf("name mismatch: got %q want %q", stmt.Name, "gcd")
	}
	if len(stmt.Params) != 2 {
		t.Fatalf("params length mismatch: got %d want %d", len(stmt.Params), 2)
	}
	returnType, ok := stmt.ReturnType.(*ast.TypeName)
	if !ok {
		t.Fatalf("expected *ast.TypeName, got %T", stmt.ReturnType)
	}
	if returnType.Name != "int" {
		t.Fatalf("return type mismatch: got %q want %q", returnType.Name, "int")
	}
}

func TestFuncDefNamedEnd(t *testing.T) {
	prog := mustParse(t, "func gcd(a: int) -> int\n  return a\nendfunc gcd")
	stmt, ok := prog.Statements[0].(*ast.FuncDef)
	if !ok {
		t.Fatalf("expected *ast.FuncDef, got %T", prog.Statements[0])
	}
	if stmt.Name != "gcd" {
		t.Fatalf("name mismatch: got %q want %q", stmt.Name, "gcd")
	}
}

func TestIfStmt(t *testing.T) {
	prog := mustParse(t, "if x > 0 then\n  y := 1\nendif")
	if _, ok := prog.Statements[0].(*ast.IfStmt); !ok {
		t.Fatalf("expected *ast.IfStmt, got %T", prog.Statements[0])
	}
}

func TestIfElifElse(t *testing.T) {
	prog := mustParse(t, "if x > 0 then\n  y := 1\nelif x = 0 then\n  y := 0\nelse\n  y := -1\nendif")
	stmt, ok := prog.Statements[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected *ast.IfStmt, got %T", prog.Statements[0])
	}
	if len(stmt.Elifs) != 1 {
		t.Fatalf("elif count mismatch: got %d want %d", len(stmt.Elifs), 1)
	}
	if len(stmt.ElseBody) != 1 {
		t.Fatalf("else length mismatch: got %d want %d", len(stmt.ElseBody), 1)
	}
}

func TestWhile(t *testing.T) {
	prog := mustParse(t, "while b != 0 do\n  b := b - 1\nendwhile")
	if _, ok := prog.Statements[0].(*ast.WhileStmt); !ok {
		t.Fatalf("expected *ast.WhileStmt, got %T", prog.Statements[0])
	}
}

func TestWhileNamed(t *testing.T) {
	prog := mustParse(t, "while b != 0 do scan\n  next scan\nendwhile scan")
	stmt, ok := prog.Statements[0].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("expected *ast.WhileStmt, got %T", prog.Statements[0])
	}
	if stmt.Name != "scan" {
		t.Fatalf("name mismatch: got %q want %q", stmt.Name, "scan")
	}
}

func TestForRange(t *testing.T) {
	prog := mustParse(t, "for i in 1 to 10 do\n  print(i)\nendfor")
	stmt, ok := prog.Statements[0].(*ast.ForRangeStmt)
	if !ok {
		t.Fatalf("expected *ast.ForRangeStmt, got %T", prog.Statements[0])
	}
	if stmt.Var != "i" {
		t.Fatalf("var mismatch: got %q want %q", stmt.Var, "i")
	}
}

func TestForRangeNamed(t *testing.T) {
	prog := mustParse(t, "for i in 1 to 3 do build\n  leave build\nendfor build")
	stmt, ok := prog.Statements[0].(*ast.ForRangeStmt)
	if !ok {
		t.Fatalf("expected *ast.ForRangeStmt, got %T", prog.Statements[0])
	}
	if stmt.Name != "build" {
		t.Fatalf("name mismatch: got %q want %q", stmt.Name, "build")
	}
}

func TestForRangeStep(t *testing.T) {
	prog := mustParse(t, "for i in 1 to 10 step 2 do\n  print(i)\nendfor")
	stmt, ok := prog.Statements[0].(*ast.ForRangeStmt)
	if !ok {
		t.Fatalf("expected *ast.ForRangeStmt, got %T", prog.Statements[0])
	}
	if stmt.Step == nil {
		t.Fatal("expected non-nil step")
	}
}

func TestForEach(t *testing.T) {
	prog := mustParse(t, "for item in list do\n  print(item)\nendfor")
	stmt, ok := prog.Statements[0].(*ast.ForEachStmt)
	if !ok {
		t.Fatalf("expected *ast.ForEachStmt, got %T", prog.Statements[0])
	}
	if stmt.Var != "item" {
		t.Fatalf("var mismatch: got %q want %q", stmt.Var, "item")
	}
}

func TestForEachNamed(t *testing.T) {
	prog := mustParse(t, "for item in list with index idx do render\n  pass\nendfor render")
	stmt, ok := prog.Statements[0].(*ast.ForEachStmt)
	if !ok {
		t.Fatalf("expected *ast.ForEachStmt, got %T", prog.Statements[0])
	}
	if stmt.Name != "render" {
		t.Fatalf("name mismatch: got %q want %q", stmt.Name, "render")
	}
	if stmt.IndexVar != "idx" {
		t.Fatalf("index var mismatch: got %q want %q", stmt.IndexVar, "idx")
	}
}

func TestPassLeaveNext(t *testing.T) {
	prog := mustParse(t, "pass\nleave outer\nnext outer")
	if _, ok := prog.Statements[0].(*ast.PassStmt); !ok {
		t.Fatalf("expected *ast.PassStmt, got %T", prog.Statements[0])
	}
	leaveStmt, ok := prog.Statements[1].(*ast.LeaveStmt)
	if !ok {
		t.Fatalf("expected *ast.LeaveStmt, got %T", prog.Statements[1])
	}
	if leaveStmt.Name != "outer" {
		t.Fatalf("leave name mismatch: got %q want %q", leaveStmt.Name, "outer")
	}
	nextStmt, ok := prog.Statements[2].(*ast.NextStmt)
	if !ok {
		t.Fatalf("expected *ast.NextStmt, got %T", prog.Statements[2])
	}
	if nextStmt.Name != "outer" {
		t.Fatalf("next name mismatch: got %q want %q", nextStmt.Name, "outer")
	}
}

func TestLoopNameMismatchRejected(t *testing.T) {
	_, err := parser.Parse("while true do outer\n  pass\nendwhile inner")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "loop name mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMatch(t *testing.T) {
	prog := mustParse(t, "match x\n  when 1 =>\n    do_a()\n  when 2, 3 =>\n    do_b()\n  else\n    do_c()\nendmatch")
	stmt, ok := prog.Statements[0].(*ast.MatchStmt)
	if !ok {
		t.Fatalf("expected *ast.MatchStmt, got %T", prog.Statements[0])
	}
	if len(stmt.Cases) != 2 {
		t.Fatalf("case count mismatch: got %d want %d", len(stmt.Cases), 2)
	}
	if len(stmt.ElseBody) != 1 {
		t.Fatalf("else length mismatch: got %d want %d", len(stmt.ElseBody), 1)
	}
}

func TestModule(t *testing.T) {
	prog := mustParse(t, "module math_utils\n  export func gcd(a: int) -> int\n    return a\n  endfunc\nendmodule")
	stmt, ok := prog.Statements[0].(*ast.ModuleDef)
	if !ok {
		t.Fatalf("expected *ast.ModuleDef, got %T", prog.Statements[0])
	}
	if stmt.Name != "math_utils" {
		t.Fatalf("name mismatch: got %q want %q", stmt.Name, "math_utils")
	}
	fn, ok := stmt.Body[0].(*ast.FuncDef)
	if !ok {
		t.Fatalf("expected *ast.FuncDef, got %T", stmt.Body[0])
	}
	if !fn.Exported {
		t.Fatal("expected exported func")
	}
}

func TestExportObject(t *testing.T) {
	prog := mustParse(t, "module bank\n  export object Account\n    balance: int\n  endobject\nendmodule")
	stmt := prog.Statements[0].(*ast.ModuleDef)
	obj, ok := stmt.Body[0].(*ast.ObjectDef)
	if !ok {
		t.Fatalf("expected *ast.ObjectDef, got %T", stmt.Body[0])
	}
	if !obj.Exported {
		t.Fatal("expected exported object")
	}
}

func TestExportTypeAlias(t *testing.T) {
	prog := mustParse(t, "module ids\n  export type UserId = int\nendmodule")
	stmt := prog.Statements[0].(*ast.ModuleDef)
	alias, ok := stmt.Body[0].(*ast.TypeAlias)
	if !ok {
		t.Fatalf("expected *ast.TypeAlias, got %T", stmt.Body[0])
	}
	if !alias.Exported {
		t.Fatal("expected exported alias")
	}
}

func TestUseModule(t *testing.T) {
	prog := mustParse(t, "use math_utils")
	stmt, ok := prog.Statements[0].(*ast.UseStmt)
	if !ok {
		t.Fatalf("expected *ast.UseStmt, got %T", prog.Statements[0])
	}
	if stmt.Module != "math_utils" {
		t.Fatalf("module mismatch: got %q want %q", stmt.Module, "math_utils")
	}
	if len(stmt.Names) != 0 {
		t.Fatalf("names length mismatch: got %d want 0", len(stmt.Names))
	}
}

func TestUseFrom(t *testing.T) {
	prog := mustParse(t, "use gcd, lcm from math_utils")
	stmt, ok := prog.Statements[0].(*ast.UseStmt)
	if !ok {
		t.Fatalf("expected *ast.UseStmt, got %T", prog.Statements[0])
	}
	if stmt.Module != "math_utils" {
		t.Fatalf("module mismatch: got %q want %q", stmt.Module, "math_utils")
	}
	if !reflect.DeepEqual(stmt.Names, []string{"gcd", "lcm"}) {
		t.Fatalf("names mismatch: got %v want %v", stmt.Names, []string{"gcd", "lcm"})
	}
}

func TestParallel(t *testing.T) {
	prog := mustParse(t, "parallel do\n  deploy()\nendparallel")
	stmt, ok := prog.Statements[0].(*ast.ParallelStmt)
	if !ok {
		t.Fatalf("expected *ast.ParallelStmt, got %T", prog.Statements[0])
	}
	if stmt.AllowFail {
		t.Fatal("expected allowfail=false")
	}
	if stmt.ResultVar != "" {
		t.Fatalf("result var mismatch: got %q want empty", stmt.ResultVar)
	}
}

func TestParallelAllowFailResult(t *testing.T) {
	prog := mustParse(t, "parallel allowfail => results do\n  check()\nendparallel")
	stmt, ok := prog.Statements[0].(*ast.ParallelStmt)
	if !ok {
		t.Fatalf("expected *ast.ParallelStmt, got %T", prog.Statements[0])
	}
	if !stmt.AllowFail {
		t.Fatal("expected allowfail=true")
	}
	if stmt.ResultVar != "results" {
		t.Fatalf("result var mismatch: got %q want %q", stmt.ResultVar, "results")
	}
}

func TestTag(t *testing.T) {
	prog := mustParse(t, "@validate")
	stmt, ok := prog.Statements[0].(*ast.TagStmt)
	if !ok {
		t.Fatalf("expected *ast.TagStmt, got %T", prog.Statements[0])
	}
	if stmt.Name != "validate" {
		t.Fatalf("name mismatch: got %q want %q", stmt.Name, "validate")
	}
}

func TestFuncCall(t *testing.T) {
	prog := mustParse(t, `print("hello", 42)`)
	stmt, ok := prog.Statements[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected *ast.ExprStmt, got %T", prog.Statements[0])
	}
	call, ok := stmt.Expr.(*ast.FuncCall)
	if !ok {
		t.Fatalf("expected *ast.FuncCall, got %T", stmt.Expr)
	}
	if len(call.Args) != 2 {
		t.Fatalf("arg length mismatch: got %d want %d", len(call.Args), 2)
	}
}

func TestBinaryOps(t *testing.T) {
	prog := mustParse(t, "x := 1 + 2 * 3")
	stmt, ok := prog.Statements[0].(*ast.Assignment)
	if !ok {
		t.Fatalf("expected *ast.Assignment, got %T", prog.Statements[0])
	}
	value, ok := stmt.Values[0].(*ast.BinaryOp)
	if !ok {
		t.Fatalf("expected *ast.BinaryOp, got %T", stmt.Values[0])
	}
	if value.Op != "+" {
		t.Fatalf("op mismatch: got %q want %q", value.Op, "+")
	}
}

func TestOkErr(t *testing.T) {
	prog := mustParse(t, "match readfile()\n  when ok(data) =>\n    print(data)\n  when err(e) =>\n    print(e)\nendmatch")
	if _, ok := prog.Statements[0].(*ast.MatchStmt); !ok {
		t.Fatalf("expected *ast.MatchStmt, got %T", prog.Statements[0])
	}
}

func TestDefaultParam(t *testing.T) {
	prog := mustParse(t, "func connect(host: string, port: int = 3306)\n  return host\nendfunc")
	stmt, ok := prog.Statements[0].(*ast.FuncDef)
	if !ok {
		t.Fatalf("expected *ast.FuncDef, got %T", prog.Statements[0])
	}
	if stmt.Params[1].Default == nil {
		t.Fatal("expected default parameter value")
	}
}

func TestGCDFull(t *testing.T) {
	source := "func gcd(a: int, b: int) -> int\n  while b != 0 do\n    a, b := b, a mod b\n  endwhile\n  return a\nendfunc"
	prog := mustParse(t, source)
	fn, ok := prog.Statements[0].(*ast.FuncDef)
	if !ok {
		t.Fatalf("expected *ast.FuncDef, got %T", prog.Statements[0])
	}
	if fn.Name != "gcd" {
		t.Fatalf("name mismatch: got %q want %q", fn.Name, "gcd")
	}
	if len(fn.Body) != 2 {
		t.Fatalf("body length mismatch: got %d want %d", len(fn.Body), 2)
	}
}

func TestGenericList(t *testing.T) {
	prog := mustParse(t, "x: list[int] := [1, 2, 3]")
	stmt, ok := prog.Statements[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("expected *ast.VarDecl, got %T", prog.Statements[0])
	}
	typeName, ok := stmt.TypeName.(*ast.GenericType)
	if !ok {
		t.Fatalf("expected *ast.GenericType, got %T", stmt.TypeName)
	}
	if typeName.Base != "list" {
		t.Fatalf("base mismatch: got %q want %q", typeName.Base, "list")
	}
	if len(typeName.Params) != 1 {
		t.Fatalf("param length mismatch: got %d want %d", len(typeName.Params), 1)
	}
	paramType, ok := typeName.Params[0].(*ast.TypeName)
	if !ok {
		t.Fatalf("expected *ast.TypeName, got %T", typeName.Params[0])
	}
	if paramType.Name != "int" {
		t.Fatalf("type mismatch: got %q want %q", paramType.Name, "int")
	}
}

func TestGenericDict(t *testing.T) {
	prog := mustParse(t, "x: dict[string, int] := []")
	stmt := prog.Statements[0].(*ast.VarDecl)
	typeName, ok := stmt.TypeName.(*ast.GenericType)
	if !ok {
		t.Fatalf("expected *ast.GenericType, got %T", stmt.TypeName)
	}
	if typeName.Base != "dict" {
		t.Fatalf("base mismatch: got %q want %q", typeName.Base, "dict")
	}
	if len(typeName.Params) != 2 {
		t.Fatalf("param length mismatch: got %d want %d", len(typeName.Params), 2)
	}
}

func TestFuncTypeParam(t *testing.T) {
	prog := mustParse(t, "func map(f: (int) -> int, arr: list[int]) -> list[int]\n  return arr\nendfunc")
	fn, ok := prog.Statements[0].(*ast.FuncDef)
	if !ok {
		t.Fatalf("expected *ast.FuncDef, got %T", prog.Statements[0])
	}
	typeName, ok := fn.Params[0].TypeName.(*ast.FuncType)
	if !ok {
		t.Fatalf("expected *ast.FuncType, got %T", fn.Params[0].TypeName)
	}
	if len(typeName.ParamTypes) != 1 {
		t.Fatalf("param type count mismatch: got %d want %d", len(typeName.ParamTypes), 1)
	}
	if _, ok := fn.ReturnType.(*ast.GenericType); !ok {
		t.Fatalf("expected *ast.GenericType, got %T", fn.ReturnType)
	}
}

func TestFuncTypeParamWithMultiReturn(t *testing.T) {
	prog := mustParse(t, "func apply(f: (int) -> int, string, x: int) -> int\n  return x\nendfunc")
	fn, ok := prog.Statements[0].(*ast.FuncDef)
	if !ok {
		t.Fatalf("expected *ast.FuncDef, got %T", prog.Statements[0])
	}
	typeName, ok := fn.Params[0].TypeName.(*ast.FuncType)
	if !ok {
		t.Fatalf("expected *ast.FuncType, got %T", fn.Params[0].TypeName)
	}
	returnTypes, ok := typeName.ReturnType.([]any)
	if !ok {
		t.Fatalf("expected []any return types, got %T", typeName.ReturnType)
	}
	if len(returnTypes) != 2 {
		t.Fatalf("return type count mismatch: got %d want 2", len(returnTypes))
	}
	first, ok := returnTypes[0].(*ast.TypeName)
	if !ok {
		t.Fatalf("expected first return *ast.TypeName, got %T", returnTypes[0])
	}
	second, ok := returnTypes[1].(*ast.TypeName)
	if !ok {
		t.Fatalf("expected second return *ast.TypeName, got %T", returnTypes[1])
	}
	if first.Name != "int" || second.Name != "string" {
		t.Fatalf("return type mismatch: got %q, %q want int, string", first.Name, second.Name)
	}
}

func TestArena(t *testing.T) {
	prog := mustParse(t, "arena test do\n  x := 42\nendarena")
	stmt, ok := prog.Statements[0].(*ast.ArenaStmt)
	if !ok {
		t.Fatalf("expected *ast.ArenaStmt, got %T", prog.Statements[0])
	}
	if stmt.Name != "test" {
		t.Fatalf("name mismatch: got %q want %q", stmt.Name, "test")
	}
	if len(stmt.Body) != 1 {
		t.Fatalf("body length mismatch: got %d want %d", len(stmt.Body), 1)
	}
}
