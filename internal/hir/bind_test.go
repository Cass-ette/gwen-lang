package hir_test

import (
	"testing"

	"github.com/Cass-ette/gwen-lang/internal/hir"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

func mustLowerAndBind(t *testing.T, source string) *hir.Program {
	t.Helper()

	program, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	lowered, err := hir.LowerProgram(program)
	if err != nil {
		t.Fatalf("lower failed: %v", err)
	}
	if err := hir.BindProgram(lowered); err != nil {
		t.Fatalf("bind failed: %v", err)
	}
	return lowered
}

func TestBindNamesAndMembers(t *testing.T) {
	program := mustLowerAndBind(t, `use abs from math
use math

object Account
  balance: int
  new(balance: int) -> Account
    return Account{balance := balance}
  endnew

  func value(self: Account) -> int
    return self.balance
  endfunc
endobject

func helper(x: int) -> int
  return abs(x)
endfunc

func main(count: int) -> int
  account: Account := Account.new(count)
  return math.abs(account.value())
endfunc`)

	decls := program.Decls()
	helper := decls[1].(*hir.Func)
	helperReturn := helper.Body[0].(*hir.Return)
	helperCall := helperReturn.Values[0].(*hir.Call)
	helperCallee := helperCall.Callee.(*hir.Ident)
	if helperCallee.Binding == nil || helperCallee.Binding.Kind != hir.BindingImported || helperCallee.Binding.SourceModule != "math" {
		t.Fatalf("expected imported math.abs binding, got %+v", helperCallee.Binding)
	}
	helperArg := helperCall.Args[0].(*hir.Ident)
	if helperArg.Binding == nil || helperArg.Binding.Kind != hir.BindingParam {
		t.Fatalf("expected param binding for helper arg, got %+v", helperArg.Binding)
	}

	mainFn := decls[2].(*hir.Func)
	accountDecl := mainFn.Body[0].(*hir.Var)
	ctorCall := accountDecl.Value.(*hir.Call)
	ctorMember := ctorCall.Callee.(*hir.Member)
	if ctorMember.Binding == nil || ctorMember.Binding.Kind != hir.MemberBindingObjectConstructor || ctorMember.Binding.ObjectName != "Account" {
		t.Fatalf("expected Account.new constructor binding, got %+v", ctorMember.Binding)
	}
	ctorBase := ctorMember.Object.(*hir.Ident)
	if ctorBase.Binding == nil || ctorBase.Binding.Kind != hir.BindingObjectType {
		t.Fatalf("expected Account object-type binding, got %+v", ctorBase.Binding)
	}

	mainReturn := mainFn.Body[1].(*hir.Return)
	mainCall := mainReturn.Values[0].(*hir.Call)
	mainModuleMember := mainCall.Callee.(*hir.Member)
	if mainModuleMember.Binding == nil || mainModuleMember.Binding.Kind != hir.MemberBindingModuleValue || mainModuleMember.Binding.OwnerName != "math" {
		t.Fatalf("expected math.abs module binding, got %+v", mainModuleMember.Binding)
	}
	accountMethodCall := mainCall.Args[0].(*hir.Call)
	accountMethod := accountMethodCall.Callee.(*hir.Member)
	if accountMethod.Binding == nil || accountMethod.Binding.Kind != hir.MemberBindingObjectMethod || accountMethod.Binding.ObjectName != "Account" {
		t.Fatalf("expected account.value method binding, got %+v", accountMethod.Binding)
	}
	accountIdent := accountMethod.Object.(*hir.Ident)
	if accountIdent.Binding == nil || accountIdent.Binding.Kind != hir.BindingLocal || accountIdent.Binding.ObjectName != "Account" {
		t.Fatalf("expected typed local binding for account, got %+v", accountIdent.Binding)
	}
}

func TestBindInferredObjectLocalMembers(t *testing.T) {
	program := mustLowerAndBind(t, `object Account
  balance: int

  new(balance: int) -> Account
    return Account{balance := balance}
  endnew

  func value(self: Account) -> int
    return self.balance
  endfunc
endobject

func main() -> int
  account := Account.new(7)
  return account.value()
endfunc`)

	mainFn := program.Decls()[1].(*hir.Func)
	switch stmt := mainFn.Body[0].(type) {
	case *hir.Var:
		if stmt.Binding == nil || stmt.Binding.Kind != hir.BindingLocal || stmt.Binding.ObjectName != "Account" {
			t.Fatalf("expected inferred object binding for account decl, got %+v", stmt.Binding)
		}
	case *hir.Assign:
		accountIdent := stmt.Targets[0].(*hir.Ident)
		if accountIdent.Binding == nil || accountIdent.Binding.Kind != hir.BindingLocal || accountIdent.Binding.ObjectName != "Account" {
			t.Fatalf("expected inferred object binding for account assign, got %+v", accountIdent.Binding)
		}
	default:
		t.Fatalf("unexpected first stmt type %T", stmt)
	}
	mainReturn := mainFn.Body[1].(*hir.Return)
	accountMethodCall := mainReturn.Values[0].(*hir.Call)
	accountMethod := accountMethodCall.Callee.(*hir.Member)
	if accountMethod.Binding == nil || accountMethod.Binding.Kind != hir.MemberBindingObjectMethod || accountMethod.Binding.ObjectName != "Account" {
		t.Fatalf("expected inferred account.value method binding, got %+v", accountMethod.Binding)
	}
	accountIdent := accountMethod.Object.(*hir.Ident)
	if accountIdent.Binding == nil || accountIdent.Binding.Kind != hir.BindingLocal || accountIdent.Binding.ObjectName != "Account" {
		t.Fatalf("expected inferred typed local binding for account, got %+v", accountIdent.Binding)
	}
}

func TestBindMatchPatternIdentifiers(t *testing.T) {
	program := mustLowerAndBind(t, `func main()
  match readfile("notes.txt")
    when ok(data) =>
      write(data)
    when err(reason) =>
      write(reason)
  endmatch
endfunc`)

	fn := program.Decls()[0].(*hir.Func)
	matchStmt := fn.Body[0].(*hir.Match)
	okPattern := matchStmt.Cases[0].Patterns[0].(*hir.Ok)
	data := okPattern.Value.(*hir.Ident)
	if data.Binding == nil || data.Binding.Kind != hir.BindingLocal {
		t.Fatalf("expected local binding for ok(data), got %+v", data.Binding)
	}
	okBody := matchStmt.Cases[0].Body[0].(*hir.ExprStmt)
	okCall := okBody.Expr.(*hir.Call)
	okArg := okCall.Args[0].(*hir.Ident)
	if okArg.Binding == nil || okArg.Binding.Name != "data" {
		t.Fatalf("expected body use to bind to data, got %+v", okArg.Binding)
	}
}

func TestBindGlobalTargetsAndMatchKinds(t *testing.T) {
	program := mustLowerAndBind(t, `func outer(value: result[int, string])
  total: int := 0
  func bump()
    global total := total + 1
  endfunc
  match value
    when ok(data) =>
      write(data)
    when err(reason) =>
      write(reason)
  endmatch
endfunc`)

	outer := program.Decls()[0].(*hir.Func)
	declStmt := outer.Body[1].(*hir.DeclStmt)
	bump := declStmt.Decl.(*hir.Func)
	globalStmt := bump.Body[0].(*hir.Global)
	if globalStmt.Target == nil || globalStmt.Target.Kind != hir.BindingLocal || globalStmt.Target.Name != "total" || globalStmt.Target.ScopeDepth != 1 {
		t.Fatalf("expected global target binding for outer total, got %+v", globalStmt.Target)
	}

	sum := globalStmt.Value.(*hir.Binary)
	totalRef := sum.Left.(*hir.Ident)
	if totalRef.Binding == nil || totalRef.Binding.Name != "total" || totalRef.Binding.ScopeDepth != 1 {
		t.Fatalf("expected RHS total to resolve one scope out, got %+v", totalRef.Binding)
	}

	matchStmt := outer.Body[2].(*hir.Match)
	if matchStmt.Binding == nil || matchStmt.Binding.Kind != hir.MatchBindingResult {
		t.Fatalf("expected result match binding, got %+v", matchStmt.Binding)
	}
	if got := matchStmt.Cases[0].PatternBindings[0].Kind; got != hir.MatchPatternResultOk {
		t.Fatalf("expected first case result_ok binding, got %q", got)
	}
	if got := matchStmt.Cases[1].PatternBindings[0].Kind; got != hir.MatchPatternResultErr {
		t.Fatalf("expected second case result_err binding, got %q", got)
	}
}

func TestBindLoopAssignmentReusesOuterBinding(t *testing.T) {
	program := mustLowerAndBind(t, `func main()
  total: int := 0
  for i in 1 to 3 do
    total := total + i
  endfor
endfunc`)

	fn := program.Decls()[0].(*hir.Func)
	totalDecl := fn.Body[0].(*hir.Var)
	loop := fn.Body[1].(*hir.ForRange)
	assign := loop.Body[0].(*hir.Assign)
	target := assign.Targets[0].(*hir.Ident)
	if target.Binding == nil || target.Binding.Name != "total" {
		t.Fatalf("expected loop assignment target to bind total, got %+v", target.Binding)
	}
	if target.Binding.ID != totalDecl.Binding.ID {
		t.Fatalf("expected loop assignment to reuse outer binding id %d, got %d", totalDecl.Binding.ID, target.Binding.ID)
	}
	if target.Binding.ScopeDepth != 1 {
		t.Fatalf("expected loop assignment to resolve one scope out, got depth %d", target.Binding.ScopeDepth)
	}
}

func TestBindFunctionLocalUseStatement(t *testing.T) {
	program := mustLowerAndBind(t, `func main() -> int
  use abs from math
  return abs(3)
endfunc`)

	fn := program.Decls()[0].(*hir.Func)
	useStmt, ok := fn.Body[0].(*hir.Use)
	if !ok {
		t.Fatalf("expected first stmt *hir.Use, got %T", fn.Body[0])
	}
	if useStmt.Module != "math" {
		t.Fatalf("expected math use stmt, got %+v", useStmt)
	}
	ret := fn.Body[1].(*hir.Return)
	call := ret.Values[0].(*hir.Call)
	callee := call.Callee.(*hir.Ident)
	if callee.Binding == nil || callee.Binding.Kind != hir.BindingImported || callee.Binding.SourceModule != "math" || callee.Binding.Name != "abs" {
		t.Fatalf("expected imported math.abs binding, got %+v", callee.Binding)
	}
}

func TestBindNestedFunctionAssignmentShadowsOuterBinding(t *testing.T) {
	program := mustLowerAndBind(t, `func outer()
  total: int := 0

  func bump()
    total := total + 1
  endfunc
endfunc`)

	outer := program.Decls()[0].(*hir.Func)
	totalDecl := outer.Body[0].(*hir.Var)
	declStmt := outer.Body[1].(*hir.DeclStmt)
	bump := declStmt.Decl.(*hir.Func)
	assign := bump.Body[0].(*hir.Assign)
	target := assign.Targets[0].(*hir.Ident)
	if target.Binding == nil || target.Binding.Kind != hir.BindingLocal || target.Binding.Name != "total" {
		t.Fatalf("expected nested assignment target to declare local total, got %+v", target.Binding)
	}
	if target.Binding.ID == totalDecl.Binding.ID {
		t.Fatalf("expected nested assignment target to shadow outer binding id %d", totalDecl.Binding.ID)
	}
	sum := assign.Values[0].(*hir.Binary)
	left := sum.Left.(*hir.Ident)
	if left.Binding == nil || left.Binding.Name != "total" {
		t.Fatalf("expected RHS total binding, got %+v", left.Binding)
	}
	if left.Binding.ID != totalDecl.Binding.ID {
		t.Fatalf("expected RHS to reuse outer binding id %d, got %d", totalDecl.Binding.ID, left.Binding.ID)
	}
	if left.Binding.ScopeDepth != 1 {
		t.Fatalf("expected RHS total to resolve one scope out, got depth %d", left.Binding.ScopeDepth)
	}
}
