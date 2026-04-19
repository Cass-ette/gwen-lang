package checker_test

import (
	"os"
	"path/filepath"
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

func mustWrite(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
