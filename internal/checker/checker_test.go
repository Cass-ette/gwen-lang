package checker_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Cass-ette/gwen-lang/internal/checker"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

func checkSource(t *testing.T, source string, sourcePath string) error {
	t.Helper()

	program, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return checker.New().CheckProgram(program, sourcePath)
}

func requireOK(t *testing.T, source string) {
	t.Helper()

	if err := checkSource(t, source, ""); err != nil {
		t.Fatalf("check failed: %v", err)
	}
}

func requireErrorContains(t *testing.T, source string, want string) {
	t.Helper()

	err := checkSource(t, source, "")
	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error mismatch:\n got: %v\nwant: %s", err, want)
	}
}

func TestUnknownTypeAnnotationRejected(t *testing.T) {
	requireErrorContains(t, "x: MissingType := 1", "Unknown type: MissingType")
}

func TestUnknownTypeAliasTargetRejected(t *testing.T) {
	requireErrorContains(t, "type UserId = MissingType", "Unknown type: MissingType")
}

func TestMoneyTagDoesNotNeedTypeDeclaration(t *testing.T) {
	requireOK(t, "price: money[USD] := 19.99")
}

func TestGenericParameterTypeIsValidated(t *testing.T) {
	requireErrorContains(t, "items: list[MissingType] := []", "Unknown type: MissingType")
}

func TestObjectMethodMustDeclareSelf(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int

  func value(this: Account) -> int
    return 0
  endfunc
endobject`, "must declare 'self' as first parameter")
}

func TestObjectMethodSelfTypeMustMatchObject(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int

  func value(self: Other) -> int
    return 0
  endfunc
endobject`, "must declare 'self: Account'")
}

func TestConstructorReturnMustMatchObject(t *testing.T) {
	requireErrorContains(t, `object Account
  new() -> Other
  endnew
endobject`, "Constructor 'Account.new' must return 'Account'")
}

func TestModuleBodyAllowsDeclarationsOnly(t *testing.T) {
	requireErrorContains(t, `module config
  x := 1
endmodule`, "top level only allows use/func/object/type declarations, got assignment")
}

func TestModuleUseMustComeBeforeDeclarations(t *testing.T) {
	requireErrorContains(t, `module m
  export func f()
  endfunc

  use list
endmodule`, "must place use statements before func/object/type declarations")
}

func TestDuplicateObjectFieldsRejected(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int
  balance: int
endobject`, "Duplicate field 'balance' in object 'Account'")
}

func TestDuplicateObjectMethodsRejected(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int

  func value(self: Account) -> int
    return 1
  endfunc

  func value(self: Account) -> int
    return 2
  endfunc
endobject`, "Duplicate method 'value' in object 'Account'")
}

func TestUseLoadsExportedTypeAliasFromSiblingFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ids.gw"), `module ids
export type UserId = int
endmodule
`)

	source := `use UserId from ids
id: UserId := 17
`
	if err := checkSource(t, source, filepath.Join(dir, "main.gw")); err != nil {
		t.Fatalf("check failed: %v", err)
	}
}

func TestUseRejectsPrivateTypeAliasFromSiblingFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ids.gw"), `module ids
type UserId = int
endmodule
`)

	source := `use UserId from ids
id: UserId := 17
`
	err := checkSource(t, source, filepath.Join(dir, "main.gw"))
	if err == nil {
		t.Fatal("expected checker error")
	}
	if !strings.Contains(err.Error(), "Module 'ids' does not export 'UserId'") {
		t.Fatalf("error mismatch: %v", err)
	}
}

func TestLoadedModuleCanReferenceHostObjectType(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "helpers.gw"), `module helpers
export func accept(item: Item) -> int
  return 1
endfunc
endmodule
`)

	source := `object Item
  value: int
endobject

func main()
  use accept from helpers
endfunc
`
	if err := checkSource(t, source, filepath.Join(dir, "main.gw")); err != nil {
		t.Fatalf("check failed: %v", err)
	}
}

func TestTooManyFunctionArgumentsRejected(t *testing.T) {
	requireErrorContains(t, `func add_one(x: int) -> int
  return x + 1
endfunc

func main()
  write(add_one(1, 2))
endfunc`, "Too many arguments for 'add_one'")
}

func TestFunctionArgumentTypeMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func greet(name: string)
  write(name)
endfunc

func main()
  greet(42)
endfunc`, "Argument 'name' to 'greet' expects string, got int")
}

func TestReturnTypeMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func bad() -> int
  return "oops"
endfunc`, "Return type mismatch: expected int, got string")
}

func TestMultiReturnItemTypeMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func pair() -> int, bool
  return 1, "nope"
endfunc`, "Return value 2 expects bool, got string")
}

func TestResultOkReturnMatchesExplicitErrorType(t *testing.T) {
	requireOK(t, `func good() -> result[int, string]
  return ok(42)
endfunc`)
}

func TestResultErrReturnMatchesExplicitErrorType(t *testing.T) {
	requireOK(t, `func good() -> result[int, string]
  return err("boom")
endfunc`)
}

func TestResultErrReturnRejectsWrongExplicitErrorType(t *testing.T) {
	requireErrorContains(t, `func bad() -> result[int, string]
  return err(42)
endfunc`, "Return type mismatch: expected result[int, string], got err(int)")
}

func TestResultShorthandMatchesExplicitStringError(t *testing.T) {
	requireOK(t, `func helper(flag: bool) -> result[int]
  if flag then
    return ok(1)
  endif
  return err("boom")
endfunc

func main()
  value: result[int, string] := helper(true)
  write(value)
endfunc`)
}

func TestResultBranchMergeCombinesOkAndErr(t *testing.T) {
	requireOK(t, `func main()
  cond := 1 = 1
  if cond then
    value := ok(1)
  else
    value := err("boom")
  endif
  typed: result[int, string] := value
  write(typed)
endfunc`)
}

func TestTypedVarAssignmentMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func main()
  x: int := "oops"
endfunc`, "Cannot assign string to 'x' (int)")
}

func TestTypedReassignmentMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func main()
  x: int := 1
  x := "oops"
endfunc`, "Cannot assign string to 'x' (int)")
}

func TestGlobalRequiresOuterBinding(t *testing.T) {
	requireErrorContains(t, `func main()
  global missing := 1
endfunc`, "global variable 'missing' not found in any outer scope")
}

func TestGlobalAssignmentTypeMismatchRejected(t *testing.T) {
	requireErrorContains(t, `count: int := 0

func bump()
  global count := "oops"
endfunc`, "Cannot assign string to 'count' (int)")
}

func TestGlobalCannotTargetBuiltin(t *testing.T) {
	requireErrorContains(t, `func main()
  global write := 1
endfunc`, "Cannot assign to builtin 'write' with global")
}

func TestFunctionTypeAssignmentMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func greet(name: string) -> int
  return 1
endfunc

func main()
  f: (int) -> int := greet
  write(f(1))
endfunc`, "Cannot assign (string) -> int to 'f' ((int) -> int)")
}

func TestMultiAssignCountMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func one() -> int
  return 1
endfunc

func main()
  a, b := one()
endfunc`, "Assignment count mismatch: 2 targets, 1 values")
}

func TestTypedListLiteralItemMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func main()
  xs: list[int] := ["a", "b"]
endfunc`, "Cannot assign list[string] to 'xs' (list[int])")
}

func TestParallelResultVarIsVisibleAfterBlock(t *testing.T) {
	requireOK(t, `func id(x: int) -> int
  return x
endfunc

func main()
  parallel => results do
    id(1)
  endparallel
  write(results)
endfunc`)
}

func TestIfBindingIsVisibleAfterBlock(t *testing.T) {
	requireOK(t, `func main()
  if true then
    found := 1
  endif
  write(found)
endfunc`)
}

func TestIfBindingMustBeAssignedOnContinuingPaths(t *testing.T) {
	requireErrorContains(t, `func main()
  cond := 1 = 1
  if cond then
    found := 1
  endif
  write(found)
endfunc`, "Undefined variable: found")
}

func TestForLoopVariableIsVisibleAfterBlock(t *testing.T) {
	requireOK(t, `func main()
  for i in 1 to 1 do
  endfor
  write(i)
endfunc`)
}

func TestWhileBindingMustBeAssignedOnContinuingPaths(t *testing.T) {
	requireErrorContains(t, `func main()
  cond := 1 = 1
  while cond do
    x := 1
    cond := false
  endwhile
  write(x)
endfunc`, "Undefined variable: x")
}

func TestWhileKnownTrueBindingIsVisibleAfterBlock(t *testing.T) {
	requireOK(t, `func main()
  while true do
    x := 1
    return
  endwhile
  write(x)
endfunc`)
}

func TestForEachEmptyLiteralBindingDoesNotLeak(t *testing.T) {
	requireErrorContains(t, `func main()
  xs := []
  for item in xs do
    x := item
  endfor
  write(x)
endfunc`, "Undefined variable: x")
}

func TestForEachNonEmptyLiteralBindingIsVisibleAfterBlock(t *testing.T) {
	requireOK(t, `func main()
  for item in [1] do
    x := item
  endfor
  write(x)
endfunc`)
}

func TestForRangeZeroIterationBindingDoesNotLeak(t *testing.T) {
	requireErrorContains(t, `func main()
  for i in 1 to 0 step 1 do
    x := i
  endfor
  write(x)
endfunc`, "Undefined variable: x")
}

func TestMatchBindingIsVisibleAfterBlock(t *testing.T) {
	requireOK(t, `func main()
  match ok(42)
    when ok(v) =>
      write(v)
  endmatch
  write(v)
endfunc`)
}

func TestMatchResultRejectsNonOkErrPattern(t *testing.T) {
	requireErrorContains(t, `func f() -> result[int]
  return ok(1)
endfunc

func main()
  match f()
    when 1 =>
      write("one")
    else
      write("other")
  endmatch
endfunc`, "Match on Result type must use ok(x) or err(x) patterns")
}

func TestMatchOkPatternBindsPayloadType(t *testing.T) {
	requireErrorContains(t, `func main()
  match ok(42)
    when ok(v) =>
      bad: string := v
  endmatch
endfunc`, "Cannot assign int to 'bad' (string)")
}

func TestMatchErrPatternBindsPayloadType(t *testing.T) {
	requireErrorContains(t, `func main()
  match err("boom")
    when err(e) =>
      bad: int := e
  endmatch
endfunc`, "Cannot assign string to 'bad' (int)")
}

func TestMatchOkLiteralPatternTypeChecked(t *testing.T) {
	requireErrorContains(t, `func main()
  match ok(42)
    when ok("boom") =>
      write("bad")
    else
      write("other")
  endmatch
endfunc`, "ok pattern expects int, got string")
}

func TestMatchBindingDoesNotLeakFromOnlyOneBranch(t *testing.T) {
	requireErrorContains(t, `func main()
  match ok(42)
    when ok(v) =>
      write(v)
    when err(e) =>
      write(e)
  endmatch
  write(v)
endfunc`, "Undefined variable: v")
}

func TestIfKnownTrueElifBranchStillLeaksBinding(t *testing.T) {
	requireOK(t, `func main()
  if false then
    never := 0
  elif true then
    x := 1
  endif
  write(x)
endfunc`)
}

func TestArenaBindingIsVisibleAfterBlock(t *testing.T) {
	requireOK(t, `func main()
  arena scratch do
    x := 1
  endarena
  write(x)
endfunc`)
}

func TestIfRejectsInconsistentBranchTypes(t *testing.T) {
	requireErrorContains(t, `func main()
  cond := 1 = 1
  if cond then
    x := 1
  else
    x := "s"
  endif
  write(x)
endfunc`, "Variable 'x' has inconsistent types across if branches: int vs string")
}

func TestMatchRejectsInconsistentBranchTypes(t *testing.T) {
	requireErrorContains(t, `func main()
  match ok(1)
    when ok(v) =>
      x := 1
    else
      x := "s"
  endmatch
  write(x)
endfunc`, "Variable 'x' has inconsistent types across match branches: int vs string")
}

func TestIfNumericBranchesMergeToFloat(t *testing.T) {
	requireOK(t, `func main()
  cond := 1 = 1
  if cond then
    x := 1
  else
    x := 2.5
  endif
  y: float := x
  write(y)
endfunc`)
}

func TestDictLiteralValueMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func main()
  scores := dict[string, int]{"alice": "high"}
endfunc`, "Dict value expects int, got string")
}

func TestAppendItemTypeMismatchRejected(t *testing.T) {
	requireErrorContains(t, `func main()
  xs: list[int] := [1, 2]
  append(xs, "bad")
endfunc`, "Argument 'item' to 'append' expects int, got string")
}

func TestHigherOrderParameterSignatureRejected(t *testing.T) {
	requireErrorContains(t, `func apply(f: (int) -> int) -> int
  return f("bad")
endfunc`, "Argument 'arg1' to 'f' expects int, got string")
}

func TestExampleEntrypointsCheck(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	var paths []string
	for _, pattern := range []string{
		filepath.Join(repoRoot, "examples", "*.gw"),
		filepath.Join(repoRoot, "examples", "*", "main.gw"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob failed for %s: %v", pattern, err)
		}
		paths = append(paths, matches...)
	}
	sort.Strings(paths)

	for _, path := range paths {
		path := path
		name := strings.TrimPrefix(path, repoRoot+string(os.PathSeparator))
		t.Run(filepath.ToSlash(name), func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read failed: %v", err)
			}
			program, err := parser.Parse(string(source))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if err := checker.New().CheckProgram(program, path); err != nil {
				t.Fatalf("check failed: %v", err)
			}
		})
	}
}

func TestObjectMemberMissingRejectedInMethodBody(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int

  func broken(self: Account) -> int
    return self.missing()
  endfunc
endobject`, "Object 'Account' has no member 'missing'")
}

func TestModuleTypeAliasIsNotRuntimeMember(t *testing.T) {
	requireErrorContains(t, `module ids
  export type UserId = int
endmodule

use ids

func main()
  write(ids.UserId)
endfunc`, "not a runtime member of module 'ids'")
}

func TestPrivateFieldReadRejectedOutsideMethod(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int

  new(balance: int) -> Account
    return Account{balance := balance}
  endnew
endobject

func main()
  acc := Account.new(1)
  write(acc.balance)
endfunc`, "Cannot access private field 'balance' of 'Account' from outside")
}

func TestPrivateFieldReadRejectedInFreeFunctionNamedSelf(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int
endobject

func leak(self: Account) -> int
  return self.balance
endfunc`, "Cannot access private field 'balance' of 'Account' from outside")
}

func TestPrivateFieldAssignmentRejectedOutsideMethod(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int

  new(balance: int) -> Account
    return Account{balance := balance}
  endnew
endobject

func main()
  acc := Account.new(1)
  acc.balance := 2
endfunc`, "Cannot access private field 'balance' of 'Account' from outside")
}

func TestPrivateFieldAssignmentAllowedInsideMethod(t *testing.T) {
	requireOK(t, `object Account
  balance: int

  func set_balance(self: Account, value: int)
    self.balance := value
  endfunc
endobject`)
}

func TestObjectLiteralFieldTypeMismatchRejected(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int

  new() -> Account
    return Account{balance := "bad"}
  endnew
endobject`, "Field 'Account.balance' expects int, got string")
}

func TestStaticMethodSelfArgumentTypeRejected(t *testing.T) {
	requireErrorContains(t, `object Account
  balance: int

  func value(self: Account) -> int
    return self.balance
  endfunc
endobject

func main()
  write(Account.value(1))
endfunc`, "Argument 'self' to 'Account.value' expects Account, got int")
}

func TestUseRejectsTypeImportConflict(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ids.gw"), `module ids
export type UserId = int8
endmodule
`)

	source := `type UserId = int
use UserId from ids
id: UserId := 17
`
	err := checkSource(t, source, filepath.Join(dir, "main.gw"))
	if err == nil {
		t.Fatal("expected checker error")
	}
	if !strings.Contains(err.Error(), "Cannot import type 'UserId' from module 'ids': type name already defined in current scope") {
		t.Fatalf("error mismatch: %v", err)
	}
}

func TestUseRejectsModuleNamespaceConflict(t *testing.T) {
	requireErrorContains(t, `math := 1
use math`, "Cannot import module 'math': name already defined in current scope")
}

func TestUseRejectsModuleFileWithExtraTopLevelStatements(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "math_utils.gw"), `module math_utils
export func square(x: int) -> int
  return x * x
endfunc
endmodule

write("leak")
`)

	source := `use square from math_utils
write(square(9))
`
	err := checkSource(t, source, filepath.Join(dir, "main.gw"))
	if err == nil {
		t.Fatal("expected checker error")
	}
	if !strings.Contains(err.Error(), "must contain exactly one top-level module definition for 'math_utils'") {
		t.Fatalf("error mismatch: %v", err)
	}
}

func TestUseRejectsCyclicFileModuleImports(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "alpha.gw"), `module alpha
use beta_value from beta

export func alpha_value() -> int
  return beta_value()
endfunc
endmodule
`)
	mustWrite(t, filepath.Join(dir, "beta.gw"), `module beta
use alpha_value from alpha

export func beta_value() -> int
  return alpha_value()
endfunc
endmodule
`)

	source := `use alpha_value from alpha
write(alpha_value())
`
	err := checkSource(t, source, filepath.Join(dir, "main.gw"))
	if err == nil {
		t.Fatal("expected checker error")
	}
	if !strings.Contains(err.Error(), "Cyclic module import detected while loading 'alpha'") {
		t.Fatalf("error mismatch: %v", err)
	}
}

func TestUseLoadsExportedObjectFromSiblingFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "bank.gw"), `module bank
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
`)

	source := `use Account from bank
acc := Account.new(13)
write(acc.value())
`
	if err := checkSource(t, source, filepath.Join(dir, "main.gw")); err != nil {
		t.Fatalf("check failed: %v", err)
	}
}

func mustWrite(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
