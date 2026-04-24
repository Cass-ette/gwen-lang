package hir_test

import (
	"testing"

	"github.com/Cass-ette/gwen-lang/internal/hir"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

func mustLower(t *testing.T, source string) *hir.Program {
	t.Helper()

	program, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	lowered, err := hir.LowerProgram(program)
	if err != nil {
		t.Fatalf("lower failed: %v", err)
	}
	return lowered
}

func requireNamedType(t *testing.T, node hir.Type, want string) {
	t.Helper()

	named, ok := node.(*hir.NamedType)
	if !ok {
		t.Fatalf("expected *hir.NamedType, got %T", node)
	}
	if named.Name != want {
		t.Fatalf("type mismatch: got %q want %q", named.Name, want)
	}
}

func TestLowerProgramCollectsUsesDeclsAndStmts(t *testing.T) {
	program := mustLower(t, `use gcd from math_utils
func classify(value: int) -> int, bool
  return value, true
endfunc
write("side effect")`)

	if len(program.Uses()) != 1 {
		t.Fatalf("use count mismatch: got %d want 1", len(program.Uses()))
	}
	if program.Uses()[0].Module != "math_utils" {
		t.Fatalf("module mismatch: got %q want %q", program.Uses()[0].Module, "math_utils")
	}
	if len(program.Decls()) != 1 {
		t.Fatalf("decl count mismatch: got %d want 1", len(program.Decls()))
	}
	fn, ok := program.Decls()[0].(*hir.Func)
	if !ok {
		t.Fatalf("expected *hir.Func, got %T", program.Decls()[0])
	}
	if len(fn.Returns) != 2 {
		t.Fatalf("return count mismatch: got %d want 2", len(fn.Returns))
	}
	requireNamedType(t, fn.Returns[0], "int")
	requireNamedType(t, fn.Returns[1], "bool")
	if len(program.Stmts()) != 1 {
		t.Fatalf("stmt count mismatch: got %d want 1", len(program.Stmts()))
	}
	if _, ok := program.Stmts()[0].(*hir.ExprStmt); !ok {
		t.Fatalf("expected *hir.ExprStmt, got %T", program.Stmts()[0])
	}
}

func TestLowerProgramPreservesTopLevelOrder(t *testing.T) {
	program := mustLower(t, `use math
func first() -> int
  return 1
endfunc
write("mid")
func second() -> int
  return 2
endfunc`)

	if len(program.Items) != 4 {
		t.Fatalf("item count mismatch: got %d want 4", len(program.Items))
	}
	if _, ok := program.Items[0].(*hir.Use); !ok {
		t.Fatalf("expected first item *hir.Use, got %T", program.Items[0])
	}
	first, ok := program.Items[1].(*hir.Func)
	if !ok {
		t.Fatalf("expected second item *hir.Func, got %T", program.Items[1])
	}
	if first.Name != "first" {
		t.Fatalf("first func mismatch: got %q want %q", first.Name, "first")
	}
	if _, ok := program.Items[2].(*hir.StmtItem); !ok {
		t.Fatalf("expected third item *hir.StmtItem, got %T", program.Items[2])
	}
	second, ok := program.Items[3].(*hir.Func)
	if !ok {
		t.Fatalf("expected fourth item *hir.Func, got %T", program.Items[3])
	}
	if second.Name != "second" {
		t.Fatalf("second func mismatch: got %q want %q", second.Name, "second")
	}
}

func TestLowerFunctionBodyUseStatement(t *testing.T) {
	program := mustLower(t, `func main()
  use abs from math
  write(abs(3))
endfunc`)

	fn, ok := program.Decls()[0].(*hir.Func)
	if !ok {
		t.Fatalf("expected *hir.Func, got %T", program.Decls()[0])
	}
	if len(fn.Body) != 2 {
		t.Fatalf("body length mismatch: got %d want 2", len(fn.Body))
	}
	useStmt, ok := fn.Body[0].(*hir.Use)
	if !ok {
		t.Fatalf("expected first stmt *hir.Use, got %T", fn.Body[0])
	}
	if useStmt.Module != "math" || len(useStmt.Names) != 1 || useStmt.Names[0] != "abs" {
		t.Fatalf("unexpected use stmt: %+v", useStmt)
	}
	if _, ok := fn.Body[1].(*hir.ExprStmt); !ok {
		t.Fatalf("expected second stmt *hir.ExprStmt, got %T", fn.Body[1])
	}
}

func TestLowerModuleObjectAndAlias(t *testing.T) {
	program := mustLower(t, `module bank
  use readfile from io
  export type UserId = int
  export object Account
    id: UserId
    balance: int
    new(id: UserId, balance: int) -> Account
      return Account{id := id, balance := balance}
    endnew
    func snapshot(self: Account) -> int
      return self.balance
    endfunc
  endobject
  export func open(id: UserId) -> Account
    return Account.new(id, 0)
  endfunc
endmodule`)

	if len(program.Decls()) != 1 {
		t.Fatalf("decl count mismatch: got %d want 1", len(program.Decls()))
	}
	module, ok := program.Decls()[0].(*hir.Module)
	if !ok {
		t.Fatalf("expected *hir.Module, got %T", program.Decls()[0])
	}
	if module.Name != "bank" {
		t.Fatalf("name mismatch: got %q want %q", module.Name, "bank")
	}
	if len(module.Uses()) != 1 {
		t.Fatalf("module use count mismatch: got %d want 1", len(module.Uses()))
	}
	if len(module.Decls()) != 3 {
		t.Fatalf("module decl count mismatch: got %d want 3", len(module.Decls()))
	}

	alias, ok := module.Decls()[0].(*hir.TypeAlias)
	if !ok {
		t.Fatalf("expected *hir.TypeAlias, got %T", module.Decls()[0])
	}
	if !alias.Exported {
		t.Fatal("expected exported alias")
	}
	requireNamedType(t, alias.Target, "int")

	objectDecl, ok := module.Decls()[1].(*hir.Object)
	if !ok {
		t.Fatalf("expected *hir.Object, got %T", module.Decls()[1])
	}
	if !objectDecl.Exported {
		t.Fatal("expected exported object")
	}
	if len(objectDecl.Fields) != 2 {
		t.Fatalf("field count mismatch: got %d want 2", len(objectDecl.Fields))
	}
	requireNamedType(t, objectDecl.Fields[0].Type, "UserId")
	if objectDecl.Constructor == nil {
		t.Fatal("expected constructor")
	}
	if len(objectDecl.Constructor.Returns) != 1 {
		t.Fatalf("constructor return count mismatch: got %d want 1", len(objectDecl.Constructor.Returns))
	}
	requireNamedType(t, objectDecl.Constructor.Returns[0], "Account")
	if len(objectDecl.Methods) != 1 {
		t.Fatalf("method count mismatch: got %d want 1", len(objectDecl.Methods))
	}
	requireNamedType(t, objectDecl.Methods[0].Returns[0], "int")
	if len(objectDecl.Methods[0].Body) != 1 {
		t.Fatalf("method body length mismatch: got %d want 1", len(objectDecl.Methods[0].Body))
	}

	fn, ok := module.Decls()[2].(*hir.Func)
	if !ok {
		t.Fatalf("expected *hir.Func, got %T", module.Decls()[2])
	}
	if !fn.Exported {
		t.Fatal("expected exported func")
	}
	requireNamedType(t, fn.Returns[0], "Account")
}

func TestLowerStructuredTypes(t *testing.T) {
	program := mustLower(t, `func map_once(f: (int) -> string, xs: list[int]) -> list[string]
  return []
endfunc`)

	fn := program.Decls()[0].(*hir.Func)
	callbackType, ok := fn.Params[0].Type.(*hir.FuncType)
	if !ok {
		t.Fatalf("expected *hir.FuncType, got %T", fn.Params[0].Type)
	}
	requireNamedType(t, callbackType.Params[0], "int")
	requireNamedType(t, callbackType.Returns[0], "string")

	listType, ok := fn.Params[1].Type.(*hir.GenericType)
	if !ok {
		t.Fatalf("expected *hir.GenericType, got %T", fn.Params[1].Type)
	}
	if listType.Base != "list" {
		t.Fatalf("generic base mismatch: got %q want %q", listType.Base, "list")
	}
	requireNamedType(t, listType.Args[0], "int")

	returnType, ok := fn.Returns[0].(*hir.GenericType)
	if !ok {
		t.Fatalf("expected *hir.GenericType, got %T", fn.Returns[0])
	}
	if returnType.Base != "list" {
		t.Fatalf("return generic base mismatch: got %q want %q", returnType.Base, "list")
	}
	requireNamedType(t, returnType.Args[0], "string")
}

func TestLowerStructuredTypesWithMultiReturnFuncType(t *testing.T) {
	program := mustLower(t, `func apply(f: (int) -> string, bool, x: int) -> int
  return x
endfunc`)

	fn := program.Decls()[0].(*hir.Func)
	callbackType, ok := fn.Params[0].Type.(*hir.FuncType)
	if !ok {
		t.Fatalf("expected *hir.FuncType, got %T", fn.Params[0].Type)
	}
	if len(callbackType.Returns) != 2 {
		t.Fatalf("callback return count mismatch: got %d want 2", len(callbackType.Returns))
	}
	requireNamedType(t, callbackType.Returns[0], "string")
	requireNamedType(t, callbackType.Returns[1], "bool")
}

func TestLowerExpressionTree(t *testing.T) {
	program := mustLower(t, `func main(xs: list[int]) -> result[string]
  return ok(service.format(xs[0] as UserId))
endfunc`)

	fn := program.Decls()[0].(*hir.Func)
	okExpr, ok := fn.Body[0].(*hir.Return)
	if !ok {
		t.Fatalf("expected *hir.Return, got %T", fn.Body[0])
	}
	if len(okExpr.Values) != 1 {
		t.Fatalf("return value count mismatch: got %d want 1", len(okExpr.Values))
	}
	wrapped, ok := okExpr.Values[0].(*hir.Ok)
	if !ok {
		t.Fatalf("expected *hir.Ok, got %T", okExpr.Values[0])
	}
	call, ok := wrapped.Value.(*hir.Call)
	if !ok {
		t.Fatalf("expected *hir.Call, got %T", wrapped.Value)
	}
	member, ok := call.Callee.(*hir.Member)
	if !ok {
		t.Fatalf("expected *hir.Member, got %T", call.Callee)
	}
	if member.Member != "format" {
		t.Fatalf("member mismatch: got %q want %q", member.Member, "format")
	}
	cast, ok := call.Args[0].(*hir.Cast)
	if !ok {
		t.Fatalf("expected *hir.Cast, got %T", call.Args[0])
	}
	if cast.TargetName != "UserId" {
		t.Fatalf("cast target mismatch: got %q want %q", cast.TargetName, "UserId")
	}
	index, ok := cast.Value.(*hir.Index)
	if !ok {
		t.Fatalf("expected *hir.Index, got %T", cast.Value)
	}
	if _, ok := index.Object.(*hir.Ident); !ok {
		t.Fatalf("expected *hir.Ident object, got %T", index.Object)
	}
	if _, ok := index.Index.(*hir.IntLiteral); !ok {
		t.Fatalf("expected *hir.IntLiteral index, got %T", index.Index)
	}
}

func TestLowerControlFlowBodies(t *testing.T) {
	program := mustLower(t, `func main()
  var default
    count: int
  endvar
  while count < 3 do scan
    if count = 1 then
      next scan
    endif
    count := count + 1
    if count = 2 then
      leave scan
    endif
  endwhile scan
  match count
    when 3 =>
      write("done")
    else
      pass
  endmatch
  parallel allowfail => results do
    check_a()
    check_b()
  endparallel
endfunc`)

	fn := program.Decls()[0].(*hir.Func)
	if len(fn.Body) != 4 {
		t.Fatalf("body length mismatch: got %d want 4", len(fn.Body))
	}
	if _, ok := fn.Body[0].(*hir.VarBlock); !ok {
		t.Fatalf("expected *hir.VarBlock, got %T", fn.Body[0])
	}
	loop, ok := fn.Body[1].(*hir.While)
	if !ok {
		t.Fatalf("expected *hir.While, got %T", fn.Body[1])
	}
	if loop.Name != "scan" {
		t.Fatalf("loop name mismatch: got %q want %q", loop.Name, "scan")
	}
	if loop.LoopID == 0 {
		t.Fatal("expected non-zero loop id")
	}
	firstIf, ok := loop.Body[0].(*hir.If)
	if !ok {
		t.Fatalf("expected *hir.If, got %T", loop.Body[0])
	}
	nextStmt, ok := firstIf.Body[0].(*hir.Next)
	if !ok {
		t.Fatalf("expected *hir.Next, got %T", firstIf.Body[0])
	}
	if nextStmt.TargetID != loop.LoopID {
		t.Fatalf("next target mismatch: got %d want %d", nextStmt.TargetID, loop.LoopID)
	}
	secondIf, ok := loop.Body[2].(*hir.If)
	if !ok {
		t.Fatalf("expected second *hir.If, got %T", loop.Body[2])
	}
	leaveStmt, ok := secondIf.Body[0].(*hir.Leave)
	if !ok {
		t.Fatalf("expected *hir.Leave, got %T", secondIf.Body[0])
	}
	if leaveStmt.TargetID != loop.LoopID {
		t.Fatalf("leave target mismatch: got %d want %d", leaveStmt.TargetID, loop.LoopID)
	}
	matchStmt, ok := fn.Body[2].(*hir.Match)
	if !ok {
		t.Fatalf("expected *hir.Match, got %T", fn.Body[2])
	}
	if len(matchStmt.Cases) != 1 {
		t.Fatalf("match case count mismatch: got %d want 1", len(matchStmt.Cases))
	}
	parallelStmt, ok := fn.Body[3].(*hir.Parallel)
	if !ok {
		t.Fatalf("expected *hir.Parallel, got %T", fn.Body[3])
	}
	if !parallelStmt.AllowFail {
		t.Fatal("expected allowfail=true")
	}
	if parallelStmt.ResultVar != "results" {
		t.Fatalf("result var mismatch: got %q want %q", parallelStmt.ResultVar, "results")
	}
	if len(parallelStmt.Body) != 2 {
		t.Fatalf("parallel body length mismatch: got %d want 2", len(parallelStmt.Body))
	}
}

func TestLowerNestedFuncAsDeclStmt(t *testing.T) {
	program := mustLower(t, `func outer()
  func inner() -> int
    return 1
  endfunc
  return inner()
endfunc`)

	fn := program.Decls()[0].(*hir.Func)
	if len(fn.Body) != 2 {
		t.Fatalf("body length mismatch: got %d want 2", len(fn.Body))
	}
	declStmt, ok := fn.Body[0].(*hir.DeclStmt)
	if !ok {
		t.Fatalf("expected *hir.DeclStmt, got %T", fn.Body[0])
	}
	inner, ok := declStmt.Decl.(*hir.Func)
	if !ok {
		t.Fatalf("expected nested *hir.Func, got %T", declStmt.Decl)
	}
	if inner.Name != "inner" {
		t.Fatalf("nested func name mismatch: got %q want %q", inner.Name, "inner")
	}
}

func TestLowerRejectsUnboundLeave(t *testing.T) {
	program, err := parser.Parse(`func main()
  leave outer
endfunc`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	_, err = hir.LowerProgram(program)
	if err == nil {
		t.Fatal("expected lower error")
	}
	if got := err.Error(); got != "leave/next at line 2 targets unknown loop \"outer\"" {
		t.Fatalf("error mismatch: got %q", got)
	}
}
