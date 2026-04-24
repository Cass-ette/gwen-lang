package interpreter_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Cass-ette/gwen-lang/internal/interpreter"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

func runProgram(t *testing.T, source string) (*interpreter.Interpreter, string) {
	t.Helper()

	program, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	var out bytes.Buffer
	interp.Stdout = &out

	if err := interp.Run(program); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	return interp, strings.TrimSpace(out.String())
}

func startHTTPServer(t *testing.T, source string) (*interpreter.Interpreter, *interpreter.HTTPServerValue) {
	t.Helper()

	interp, _ := runProgram(t, source)
	value, err := interp.GlobalEnv.Get("started")
	if err != nil {
		if startup, startupErr := interp.GlobalEnv.Get("startup_error"); startupErr == nil {
			t.Fatalf("server failed to start: %v", startup)
		}
		t.Fatalf("missing started server handle: %v", err)
	}
	server, ok := value.(*interpreter.HTTPServerValue)
	if !ok {
		t.Fatalf("started has wrong type: %T", value)
	}
	return interp, server
}

func stopHTTPServer(t *testing.T, server *interpreter.HTTPServerValue) {
	t.Helper()

	if err := server.Server.Close(); err != nil && !strings.Contains(err.Error(), "Server closed") {
		t.Fatalf("close failed: %v", err)
	}
	select {
	case <-server.ErrCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server shutdown")
	}
}

func runSource(t *testing.T, source string) string {
	t.Helper()

	_, out := runProgram(t, source)
	return out
}

func requireListItems(t *testing.T, value any) []any {
	t.Helper()

	switch value := value.(type) {
	case *interpreter.ListValue:
		return value.Items
	case []any:
		return value
	default:
		t.Fatalf("list type mismatch: got %T", value)
		return nil
	}
}

func requireRuntimeErrorContains(t *testing.T, source string, want string) {
	t.Helper()

	program, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	err = interpreter.New().Run(program)
	if err == nil {
		t.Fatalf("expected runtime error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error mismatch:\n got: %v\nwant: %s", err, want)
	}
}

func runProgramPath(t *testing.T, path string) string {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	program, err := parser.Parse(string(source))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	var out bytes.Buffer
	interp.Stdout = &out

	if err := interp.RunWithSource(program, path); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	return strings.TrimSpace(out.String())
}

func TestHello(t *testing.T) {
	out := runSource(t, `write("Hello, Gwen!")`)
	if out != "Hello, Gwen!" {
		t.Fatalf("output mismatch: got %q want %q", out, "Hello, Gwen!")
	}
}

func TestVariables(t *testing.T) {
	out := runSource(t, "x := 42\nwrite(x)")
	if out != "42" {
		t.Fatalf("output mismatch: got %q want %q", out, "42")
	}
}

func TestTypedVar(t *testing.T) {
	out := runSource(t, "x: int := 10\nwrite(x)")
	if out != "10" {
		t.Fatalf("output mismatch: got %q want %q", out, "10")
	}
}

func TestArithmetic(t *testing.T) {
	out := runSource(t, "write(2 + 3 * 4)")
	if out != "14" {
		t.Fatalf("output mismatch: got %q want %q", out, "14")
	}
}

func TestMod(t *testing.T) {
	out := runSource(t, "write(10 mod 3)")
	if out != "1" {
		t.Fatalf("output mismatch: got %q want %q", out, "1")
	}
}

func TestStringConcat(t *testing.T) {
	out := runSource(t, `write("hello" + " " + "world")`)
	if out != "hello world" {
		t.Fatalf("output mismatch: got %q want %q", out, "hello world")
	}
}

func TestComparison(t *testing.T) {
	out := runSource(t, "write(3 = 3)")
	if out != "true" {
		t.Fatalf("output mismatch: got %q want %q", out, "true")
	}
}

func TestFunc(t *testing.T) {
	out := runSource(t, "func double(x: int) -> int\n  return x * 2\nendfunc\nwrite(double(21))")
	if out != "42" {
		t.Fatalf("output mismatch: got %q want %q", out, "42")
	}
}

func TestFuncAutoMain(t *testing.T) {
	out := runSource(t, `func main()
  write("from main")
endfunc`)
	if out != "from main" {
		t.Fatalf("output mismatch: got %q want %q", out, "from main")
	}
}

func TestDefaultParam(t *testing.T) {
	out := runSource(t, `func greet(name: string, greeting: string = "Hello")
  write(greeting + ", " + name)
endfunc
greet("Gwen")`)
	if out != "Hello, Gwen" {
		t.Fatalf("output mismatch: got %q want %q", out, "Hello, Gwen")
	}
}

func TestIf(t *testing.T) {
	out := runSource(t, "x := 10\nif x > 5 then\n  write(\"big\")\nelse\n  write(\"small\")\nendif")
	if out != "big" {
		t.Fatalf("output mismatch: got %q want %q", out, "big")
	}
}

func TestElif(t *testing.T) {
	out := runSource(t, "x := 0\nif x > 0 then\n  write(\"positive\")\nelif x = 0 then\n  write(\"zero\")\nelse\n  write(\"negative\")\nendif")
	if out != "zero" {
		t.Fatalf("output mismatch: got %q want %q", out, "zero")
	}
}

func TestWhile(t *testing.T) {
	out := runSource(t, "x := 0\nwhile x < 5 do\n  x := x + 1\nendwhile\nwrite(x)")
	if out != "5" {
		t.Fatalf("output mismatch: got %q want %q", out, "5")
	}
}

func TestPass(t *testing.T) {
	out := runSource(t, "pass\nwrite(\"ok\")")
	if out != "ok" {
		t.Fatalf("output mismatch: got %q want %q", out, "ok")
	}
}

func TestNamedWhileNextAndLeave(t *testing.T) {
	out := runSource(t, `x := 0
while true do scan
  x := x + 1
  if x < 3 then
    next scan
  endif
  leave scan
endwhile scan
write(x)`)
	if out != "3" {
		t.Fatalf("output mismatch: got %q want %q", out, "3")
	}
}

func TestNestedLeaveOuter(t *testing.T) {
	out := runSource(t, `hits := 0
while true do outer
  for i in 1 to 3 do inner
    hits := hits + 1
    leave outer
  endfor inner
  hits := 99
endwhile outer
write(hits)`)
	if out != "1" {
		t.Fatalf("output mismatch: got %q want %q", out, "1")
	}
}

func TestForRange(t *testing.T) {
	out := runSource(t, "sum := 0\nfor i in 1 to 5 do\n  sum := sum + i\nendfor\nwrite(sum)")
	if out != "15" {
		t.Fatalf("output mismatch: got %q want %q", out, "15")
	}
}

func TestForRangeReverse(t *testing.T) {
	out := runSource(t, "result := \"\"\nfor i in 3 to 1 do\n  result := result + str(i)\nendfor\nwrite(result)")
	if out != "321" {
		t.Fatalf("output mismatch: got %q want %q", out, "321")
	}
}

func TestForRangeStep(t *testing.T) {
	out := runSource(t, "result := \"\"\nfor i in 1 to 10 step 3 do\n  result := result + str(i) + \" \"\nendfor\nwrite(result)")
	if out != "1 4 7 10" {
		t.Fatalf("output mismatch: got %q want %q", out, "1 4 7 10")
	}
}

func TestForEach(t *testing.T) {
	out := runSource(t, "items := [10, 20, 30]\nsum := 0\nfor item in items do\n  sum := sum + item\nendfor\nwrite(sum)")
	if out != "60" {
		t.Fatalf("output mismatch: got %q want %q", out, "60")
	}
}

func TestMatch(t *testing.T) {
	out := runSource(t, "x := 2\nmatch x\n  when 1 =>\n    write(\"one\")\n  when 2, 3 =>\n    write(\"two or three\")\n  else\n    write(\"other\")\nendmatch")
	if out != "two or three" {
		t.Fatalf("output mismatch: got %q want %q", out, "two or three")
	}
}

func TestMatchRange(t *testing.T) {
	out := runSource(t, "x := 5\nmatch x\n  when 1 to 3 =>\n    write(\"low\")\n  when 4 to 6 =>\n    write(\"mid\")\n  else\n    write(\"high\")\nendmatch")
	if out != "mid" {
		t.Fatalf("output mismatch: got %q want %q", out, "mid")
	}
}

func TestOkErr(t *testing.T) {
	out := runSource(t, `func safe_div(a: int, b: int)
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
endmatch`)
	if out != "5" {
		t.Fatalf("output mismatch: got %q want %q", out, "5")
	}
}

func TestOkErrErrorCase(t *testing.T) {
	out := runSource(t, `func safe_div(a: int, b: int)
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
endmatch`)
	if out != "division by zero" {
		t.Fatalf("output mismatch: got %q want %q", out, "division by zero")
	}
}

func TestMatchResultWithNonOkErrPatternRejected(t *testing.T) {
	requireRuntimeErrorContains(t, `func f() -> result[int]
  return ok(1)
endfunc

match f()
  when 1 =>
    write("one")
  else
    write("other")
endmatch`, "Match on Result type must use ok(x) or err(x) patterns")
}

func TestMatchResultOnlyOkRaisesOnErrValue(t *testing.T) {
	requireRuntimeErrorContains(t, `func f() -> result[int]
  return err("boom")
endfunc

match f()
  when ok(n) =>
    write(n)
endmatch`, "match statement has no matching case and no 'else' branch")
}

func TestMatchResultOnlyErrRaisesOnOkValue(t *testing.T) {
	requireRuntimeErrorContains(t, `func f() -> result[int]
  return ok(42)
endfunc

match f()
  when err(e) =>
    write(e)
endmatch`, "match statement has no matching case and no 'else' branch")
}

func TestMatchResultOkPlusElseCoversErr(t *testing.T) {
	out := runSource(t, `func f() -> result[int]
  return err("boom")
endfunc

match f()
  when ok(n) =>
    write(n)
  else
    write("fallback")
endmatch`)
	if out != "fallback" {
		t.Fatalf("output mismatch: got %q want %q", out, "fallback")
	}
}

func TestGCD(t *testing.T) {
	out := runSource(t, "func gcd(a: int, b: int) -> int\n  while b != 0 do\n    a, b := b, a mod b\n  endwhile\n  return a\nendfunc\nwrite(gcd(48, 18))")
	if out != "6" {
		t.Fatalf("output mismatch: got %q want %q", out, "6")
	}
}

func TestModuleImport(t *testing.T) {
	out := runSource(t, "module math_utils\n  export func square(x: int) -> int\n    return x * x\n  endfunc\nendmodule\n\nuse square from math_utils\nwrite(square(7))")
	if out != "49" {
		t.Fatalf("output mismatch: got %q want %q", out, "49")
	}
}

func TestModuleNamespace(t *testing.T) {
	out := runSource(t, "module math_utils\n  export func square(x: int) -> int\n    return x * x\n  endfunc\nendmodule\n\nuse math_utils\nwrite(math_utils.square(7))")
	if out != "49" {
		t.Fatalf("output mismatch: got %q want %q", out, "49")
	}
}

func TestExtendedStdlibModules(t *testing.T) {
	out := runSource(t, `func main()
  use pop, insert, reversed, sort, desc from list
  use replace from string
  use abs, sqrt, floor, ceil from math

  xs := [1, 3]
  insert(xs, 1, 2)
  last := pop(xs)
  write(xs)
  write(last)
  write(reversed(xs))
  write(sort([1, 3, 2], desc))
  write(replace("hello hello", "hello", "hi"))
  write(abs(-3))
  write(sqrt(4.0))
  write(floor(2.9))
  write(ceil(2.1))
endfunc`)
	want := "[1,2]\n3\n[2,1]\n[3,2,1]\nhi hi\n3\n2.0\n2.0\n3.0"
	if out != want {
		t.Fatalf("output mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestStringStartsWithEndsWithModuleImport(t *testing.T) {
	out := runSource(t, `func main()
  use startswith, endswith from string

  write(startswith("gwen-lang", "gwen"))
  write(startswith("gwen-lang", "lang"))
  write(endswith("gwen-lang", "lang"))
  write(endswith("gwen-lang", "gwen"))
endfunc`)
	if out != "true\nfalse\ntrue\nfalse" {
		t.Fatalf("output mismatch: got %q want %q", out, "true\nfalse\ntrue\nfalse")
	}
}

func TestPathModuleHelpers(t *testing.T) {
	out := runSource(t, `func main()
  use basename, dirname, joinpath from path

  write(basename("docs/stdlib.md"))
  write(dirname("docs/stdlib.md"))
  write(joinpath("docs", "stdlib.md"))
endfunc`)
	if out != "stdlib.md\ndocs\ndocs/stdlib.md" {
		t.Fatalf("output mismatch: got %q want %q", out, "stdlib.md\ndocs\ndocs/stdlib.md")
	}
}

func TestLeaveOutsideLoopRejectedAtRuntime(t *testing.T) {
	requireRuntimeErrorContains(t, "leave scan", "leave 'scan' used outside matching loop")
}

func TestNextOutsideLoopRejectedAtRuntime(t *testing.T) {
	requireRuntimeErrorContains(t, "next scan", "next 'scan' used outside matching loop")
}

func TestObjectMethod(t *testing.T) {
	out := runSource(t, "object Account\n  balance: int\n\n  new(balance: int) -> Account\n    return Account{balance := balance}\n  endnew\n\n  func value(self: Account) -> int\n    return self.balance\n  endfunc\nendobject\n\nacc := Account.new(7)\nwrite(acc.value())")
	if out != "7" {
		t.Fatalf("output mismatch: got %q want %q", out, "7")
	}
}

func TestModuleExportedObject(t *testing.T) {
	out := runSource(t, "module bank\n  export object Account\n    balance: int\n\n    new(balance: int) -> Account\n      return Account{balance := balance}\n    endnew\n\n    func value(self: Account) -> int\n      return self.balance\n    endfunc\n  endobject\nendmodule\n\nuse Account from bank\nacc := Account.new(7)\nwrite(acc.value())")
	if out != "7" {
		t.Fatalf("output mismatch: got %q want %q", out, "7")
	}
}

func TestLambda(t *testing.T) {
	out := runSource(t, "double := (x: int) => x * 2\nwrite(double(5))")
	if out != "10" {
		t.Fatalf("output mismatch: got %q want %q", out, "10")
	}
}

func TestListLen(t *testing.T) {
	out := runSource(t, "nums := [1, 2, 3]\nwrite(len(nums))")
	if out != "3" {
		t.Fatalf("output mismatch: got %q want %q", out, "3")
	}
}

func TestTagNoEffect(t *testing.T) {
	out := runSource(t, "@setup\nx := 42\nwrite(x)")
	if out != "42" {
		t.Fatalf("output mismatch: got %q want %q", out, "42")
	}
}

func TestMultiAssignSwap(t *testing.T) {
	out := runSource(t, "a := 1\nb := 2\na, b := b, a\nwrite(a)\nwrite(b)")
	if out != "2\n1" {
		t.Fatalf("output mismatch: got %q want %q", out, "2\n1")
	}
}

func TestTypeAliasBasic(t *testing.T) {
	out := runSource(t, "type UserId = int\nfunc main()\n  id: UserId := 42\n  write(id)\nendfunc")
	if out != "42" {
		t.Fatalf("output mismatch: got %q want %q", out, "42")
	}
}

func TestPrecisionIntNegativeValue(t *testing.T) {
	out := runSource(t, "x: int8 := -1\nwrite(x)")
	if out != "-1" {
		t.Fatalf("output mismatch: got %q want %q", out, "-1")
	}
}

func TestIndexAssignInLoop(t *testing.T) {
	out := runSource(t, "arr := [1, 2, 3, 4, 5]\nfor i in 0 to 4 do\n  arr[i] := arr[i] * 2\nendfor\nfor i in 0 to 4 do\n  write(arr[i])\nendfor")
	if out != "2\n4\n6\n8\n10" {
		t.Fatalf("output mismatch: got %q want %q", out, "2\n4\n6\n8\n10")
	}
}

func TestAppendAndRemoveAtMutateListTarget(t *testing.T) {
	out := runSource(t, "items := []\nappend(items, 1)\nappend(items, 2)\nappend(items, 3)\nremoveat(items, 1)\nwrite(items)")
	if out != "[1,3]" {
		t.Fatalf("output mismatch: got %q want %q", out, "[1,3]")
	}
}

func TestAppendMutatesListAcrossFunctionBoundary(t *testing.T) {
	out := runSource(t, `func add(xs: list, value: string)
  append(xs, value)
endfunc

items := []
add(items, "x")
write(items)`)
	if out != "[\"x\"]" {
		t.Fatalf("output mismatch: got %q want %q", out, "[\"x\"]")
	}
}

func TestInsertAndRemoveAtMutateListAcrossFunctionBoundary(t *testing.T) {
	out := runSource(t, `func reshape(xs: list)
  insert(xs, 1, 2)
  removeat(xs, 0)
endfunc

items := [1, 3]
reshape(items)
write(items)`)
	if out != "[2,3]" {
		t.Fatalf("output mismatch: got %q want %q", out, "[2,3]")
	}
}

func TestAppendRejectsCapturedMultiReturnValue(t *testing.T) {
	requireRuntimeErrorContains(t, `func pair() -> int, int
  return 1, 2
endfunc

values := pair()
append(values, 3)`, "append() requires mutable list, got multi-value result")
}

func TestObjectPrivateFieldAccessForbidden(t *testing.T) {
	program, err := parser.Parse("object Account\n  balance: float\n  owner: string\n\n  new(owner: string, initial: float) -> Account\n    return Account{balance := initial, owner := owner}\n  endnew\n\n  func get_owner(self: Account) -> string\n    return self.owner\n  endfunc\nendobject\n\nacc := Account.new(\"Dave\", 10.0)\nwrite(str(acc.balance))")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	err = interpreter.New().Run(program)
	if err == nil || !strings.Contains(err.Error(), "private field") {
		t.Fatalf("expected private field error, got %v", err)
	}
}

func TestObjectStaticMethodDispatch(t *testing.T) {
	out := runSource(t, "object Counter\n  n: int\n\n  new() -> Counter\n    return Counter{n := 0}\n  endnew\n\n  func inc(self: Counter) -> int\n    self.n := self.n + 1\n    return self.n\n  endfunc\nendobject\n\nc := Counter.new()\nwrite(str(Counter.inc(c)))\nwrite(str(Counter.inc(c)))")
	if out != "1\n2" {
		t.Fatalf("output mismatch: got %q want %q", out, "1\n2")
	}
}

func TestTypeofObjectInstance(t *testing.T) {
	out := runSource(t, "object Box\n  v: int\n  new(x: int) -> Box\n    return Box{v := x}\n  endnew\nendobject\n\nb := Box.new(7)\nwrite(typeof(b))")
	if out != "Box" {
		t.Fatalf("output mismatch: got %q want %q", out, "Box")
	}
}

func TestStdlibListModuleImport(t *testing.T) {
	out := runSource(t, "use map, filter, range, enumerate from list\n\nfunc main()\n  nums := range(1, 5)\n  write(\"Original:\", nums)\n  doubled := map(nums, (x: int) => x * 2)\n  write(\"Doubled:\", doubled)\n  evens := filter(nums, (x: int) => x mod 2 = 0)\n  write(\"Evens:\", evens)\n  indexed := enumerate([\"a\", \"b\", \"c\"])\n  write(\"Indexed:\", indexed)\nendfunc")
	want := "Original: [1,2,3,4,5]\nDoubled: [2,4,6,8,10]\nEvens: [2,4]\nIndexed: [[0,\"a\"],[1,\"b\"],[2,\"c\"]]"
	if out != want {
		t.Fatalf("output mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestUseLoadsModuleFromSameDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "math_utils.gw"), []byte("module math_utils\nexport func square(x: int) -> int\n  return x * x\nendfunc\nendmodule\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	mainPath := filepath.Join(dir, "main.gw")
	if err := os.WriteFile(mainPath, []byte("use square from math_utils\nwrite(square(9))\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	out := runProgramPath(t, mainPath)
	if out != "81" {
		t.Fatalf("output mismatch: got %q want %q", out, "81")
	}
}

func TestUseSupportsTransitiveModuleLoading(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helpers.gw"), []byte("module helpers\nexport func double(x: int) -> int\n  return x * 2\nendfunc\nendmodule\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "math_utils.gw"), []byte("module math_utils\nuse double from helpers\n\nexport func quadruple(x: int) -> int\n  return double(double(x))\nendfunc\nendmodule\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	mainPath := filepath.Join(dir, "main.gw")
	if err := os.WriteFile(mainPath, []byte("use quadruple from math_utils\nwrite(quadruple(7))\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	out := runProgramPath(t, mainPath)
	if out != "28" {
		t.Fatalf("output mismatch: got %q want %q", out, "28")
	}
}

func TestUseRejectsModuleFileWithExtraTopLevelStatements(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "math_utils.gw"), []byte("module math_utils\nexport func square(x: int) -> int\n  return x * x\nendfunc\nendmodule\n\nwrite(\"leak\")\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	mainPath := filepath.Join(dir, "main.gw")
	if err := os.WriteFile(mainPath, []byte("use square from math_utils\nwrite(square(9))\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	source, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	program, err := parser.Parse(string(source))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	err = interpreter.New().RunWithSource(program, mainPath)
	if err == nil || !strings.Contains(err.Error(), "must contain exactly one top-level module definition for 'math_utils'") {
		t.Fatalf("expected module file structure error, got %v", err)
	}
}

func TestDictBuiltins(t *testing.T) {
	out := runSource(t, `func main()
  scores := dict[string, int]{"alice": 90, "bob": 85}
  scores["carol"] := 77
  scores["alice"] := 95
  write("has zoe?", haskey(scores, "zoe"))
  write("zoe default =", get(scores, "zoe", 0))
  total := 0
  for k in keys(scores) do
    total := total + scores[k]
  endfor
  write("total =", total)
endfunc`)
	if !strings.Contains(out, "has zoe? false") {
		t.Fatalf("expected haskey output, got %q", out)
	}
	if !strings.Contains(out, "zoe default = 0") {
		t.Fatalf("expected get default output, got %q", out)
	}
	if !strings.Contains(out, "total = 257") {
		t.Fatalf("expected total output, got %q", out)
	}
}

func TestFileIOBuiltins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.txt")
	source := fmt.Sprintf(`func main()
  path := %q
  match writefile(path, "hello\nworld\n")
    when ok(n) => write("wrote", n, "bytes")
    when err(e) => write("write failed:", e)
  endmatch

  match readfile(path)
    when ok(content) =>
      write(content)
      lines := split(content, "\n")
      write("line count:", len(lines))
    when err(e) =>
      write("read failed:", e)
  endmatch

  match appendfile(path, "appended!\n")
    when ok(n) => write("appended", n, "bytes")
    when err(e) => write("append failed:", e)
  endmatch
endfunc`, path)
	out := runSource(t, source)
	if !strings.Contains(out, "wrote 12 bytes") {
		t.Fatalf("expected writefile output, got %q", out)
	}
	if !strings.Contains(out, "hello\nworld") {
		t.Fatalf("expected readfile content, got %q", out)
	}
	if !strings.Contains(out, "line count: 3") {
		t.Fatalf("expected split/len output, got %q", out)
	}
	if !strings.Contains(out, "appended 10 bytes") {
		t.Fatalf("expected appendfile output, got %q", out)
	}
}

func TestReadDirBuiltin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write fixture failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write fixture failed: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir fixture failed: %v", err)
	}

	source := fmt.Sprintf(`use readdir from io

func main()
  match readdir(%q)
    when ok(entries) => write(entries)
    when err(e) => write("failed:", e)
  endmatch
endfunc`, dir)
	out := runSource(t, source)
	if !strings.Contains(out, "[") || !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.txt") || !strings.Contains(out, "nested") {
		t.Fatalf("expected directory entries, got %q", out)
	}
}

func TestOSModuleImports(t *testing.T) {
	t.Setenv("GWEN_LANG_TEST_ENV", "present")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}

	program, err := parser.Parse(`use args, cwd, getenv from os

func main()
  argv := args()
  write("argc", len(argv))
  write("arg0", argv[0])
  write("arg1", argv[1])
  write("cwd", cwd())
  match getenv("GWEN_LANG_TEST_ENV")
    when ok(value) => write("env", value)
    when err(e) => write("missing", e)
  endmatch
  match getenv("GWEN_LANG_TEST_ENV_MISSING")
    when ok(value) => write("unexpected", value)
    when err(e) => write("missing", e)
  endmatch
endfunc`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	interp.ProgramArgs = []string{"serve", "--port=8080"}
	var out bytes.Buffer
	interp.Stdout = &out

	if err := interp.Run(program); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "argc 2") {
		t.Fatalf("missing argc output: %q", got)
	}
	if !strings.Contains(got, "arg0 serve") || !strings.Contains(got, "arg1 --port=8080") {
		t.Fatalf("missing args output: %q", got)
	}
	if !strings.Contains(got, "cwd "+wd) {
		t.Fatalf("missing cwd output: %q", got)
	}
	if !strings.Contains(got, "env present") {
		t.Fatalf("missing getenv success output: %q", got)
	}
	if !strings.Contains(got, "missing environment variable not found: GWEN_LANG_TEST_ENV_MISSING") {
		t.Fatalf("missing getenv error output: %q", got)
	}
}

func TestTimeModuleImports(t *testing.T) {
	start := time.Now()
	out := runSource(t, `use sleep, nowunix, nowunixms, nowrfc3339 from time

func main()
  before := nowunixms()
  sleep(20)
  after := nowunixms()
  write(after >= before)
  write(nowunix() >= 0)
  stamp := nowrfc3339()
  write(contains(stamp, "T"))
endfunc`)
	if time.Since(start) < 15*time.Millisecond {
		t.Fatalf("sleep() returned too quickly")
	}
	if out != "true\ntrue\ntrue" {
		t.Fatalf("output mismatch: got %q want %q", out, "true\ntrue\ntrue")
	}
}

func TestOSTimeNamespaceImports(t *testing.T) {
	t.Setenv("GWEN_LANG_TEST_ENV", "present")

	out := runSource(t, `use os
use time

func main()
  argv := os.args()
  write("argc", len(argv))
  write("cwd", os.cwd() != "")
  match os.getenv("GWEN_LANG_TEST_ENV")
    when ok(value) => write("env", value)
    when err(e) => write("missing", e)
  endmatch
  before := time.nowunixms()
  time.sleep(5)
  after := time.nowunixms()
  write(after >= before)
  write(time.nowunix() >= 0)
  write(contains(time.nowrfc3339(), "T"))
endfunc`)

	if out != "argc 0\ncwd true\nenv present\ntrue\ntrue\ntrue" {
		t.Fatalf("output mismatch: got %q want %q", out, "argc 0\ncwd true\nenv present\ntrue\ntrue\ntrue")
	}
}

func TestOSTimeModuleOnlyBuiltinsRequireImport(t *testing.T) {
	requireRuntimeErrorContains(t, `func main()
  write(args())
endfunc`, "Undefined variable: args")
}

func TestHTTPModuleGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Trace", "abc123")
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	source := fmt.Sprintf(`use http

func main()
  match http.get(%q)
    when ok(resp) =>
      write(http.status(resp))
      write(http.responseheader(resp, "X-Trace", "missing"))
      write(http.responsebody(resp))
    when err(e) => write("err", e)
  endmatch
endfunc`, server.URL+"/health")

	out := runSource(t, source)
	if out != "200\nabc123\nok" {
		t.Fatalf("output mismatch: got %q want %q", out, "200\nabc123\nok")
	}
}

func TestHTTPModuleGetReturnsResponseOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	source := fmt.Sprintf(`use http

func main()
  match http.get(%q)
    when ok(resp) =>
      write(http.status(resp))
      write(contains(http.responsebody(resp), "bad gateway"))
    when err(e) => write("err", e)
  endmatch
endfunc`, server.URL)

	out := runSource(t, source)
	if out != "502\ntrue" {
		t.Fatalf("expected structured non-2xx response, got %q", out)
	}
}

func TestHTTPModuleRequestWithHeadersAndBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer demo" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content-type: %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if string(body) != "{\"name\":\"Ada\"}" {
			t.Fatalf("unexpected body: %q", string(body))
		}
		w.Header().Set("X-Trace", "req-1")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, "created")
	}))
	defer server.Close()

	source := fmt.Sprintf(`use http

func main()
  headers := dict[string, string]{"Authorization": "Bearer demo", "Content-Type": "application/json"}
  match http.request("POST", %q, "{\"name\":\"Ada\"}", headers)
    when ok(resp) =>
      write(http.status(resp))
      write(http.responseheader(resp, "X-Trace", "missing"))
      write(http.responsebody(resp))
    when err(e) => write("err", e)
  endmatch
endfunc`, server.URL)

	out := runSource(t, source)
	if out != "201\nreq-1\ncreated" {
		t.Fatalf("output mismatch: got %q want %q", out, "201\nreq-1\ncreated")
	}
}

func TestHTTPModuleGetInsideParallel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "parallel-ok")
	}))
	defer server.Close()

	source := fmt.Sprintf(`use http

func main()
  parallel do
    match http.get(%q)
      when ok(resp) => write(http.responsebody(resp))
      when err(e) => write("err", e)
    endmatch
  endparallel
endfunc`, server.URL)

	out := runSource(t, source)
	if out != "parallel-ok" {
		t.Fatalf("output mismatch: got %q want %q", out, "parallel-ok")
	}
}

func TestHTTPModuleGetReturnsErrOnTransportFailure(t *testing.T) {
	out := runSource(t, `use http

func main()
  match http.get("://bad")
    when ok(resp) => write(http.status(resp))
    when err(e) => write(contains(e, "missing protocol scheme"))
  endmatch
endfunc`)

	if out != "true" {
		t.Fatalf("output mismatch: got %q want %q", out, "true")
	}
}

func TestHTTPServerLifecycleBuiltins(t *testing.T) {
	out := runSource(t, `use http

func handle(req: HttpRequest) -> HttpReply
  return http.text(200, "ok")
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    write(contains(http.addr(server), ":"))
    match http.close(server)
      when ok(code) => write(code = 0)
      when err(e) => write(e)
    endmatch
    match http.wait(server)
      when ok(code) => write(code = 0)
      when err(e) => write(e)
    endmatch
  when err(e) =>
    write(e)
endmatch`)

	if out != "true\ntrue\ntrue" {
		t.Fatalf("output mismatch: got %q want %q", out, "true\ntrue\ntrue")
	}
}

func TestHTTPServerRouteQueryAndJSON(t *testing.T) {
	source := `use http
use json

func handle(req: HttpRequest) -> result[HttpReply]
  matched, params := http.route(req, "/hello/:name")
  if matched then
    return http.json(200, json.objectof("method", http.method(req), "name", params["name"], "lang", http.query(req, "lang", "en"), "body", http.requestbody(req)))
  endif
  return ok(http.text(404, "missing"))
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    started := server
  when err(e) =>
    startup_error := e
endmatch`

	_, server := startHTTPServer(t, source)
	defer stopHTTPServer(t, server)

	req, err := http.NewRequest(http.MethodPost, "http://"+server.Addr+"/hello/Ada?lang=zh", strings.NewReader("nihao"))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("unexpected content-type: %q", got)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if payload["method"] != "POST" || payload["name"] != "Ada" || payload["lang"] != "zh" || payload["body"] != "nihao" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestHTTPServerRequestAndResponseHeaders(t *testing.T) {
	source := `use http

func handle(req: HttpRequest) -> HttpReply
  token := http.requestheader(req, "X-Token", "missing")
  return http.withheader(http.text(200, token), "X-Server", "gwen")
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    started := server
  when err(e) =>
    startup_error := e
endmatch`

	_, server := startHTTPServer(t, source)
	defer stopHTTPServer(t, server)

	req, err := http.NewRequest(http.MethodGet, "http://"+server.Addr+"/", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("X-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(body) != "secret" {
		t.Fatalf("unexpected body: %q", string(body))
	}
	if got := resp.Header.Get("X-Server"); got != "gwen" {
		t.Fatalf("unexpected X-Server header: %q", got)
	}
}

func TestHTTPServerRedirectAndCookies(t *testing.T) {
	source := `use http

func handle(req: HttpRequest) -> HttpReply
  if http.path(req) = "/login" then
    reply := http.redirect(303, "/home")
    reply := http.withcookie(reply, "session", "abc")
    reply := http.withcookie(reply, "theme", "light")
    return reply
  endif
  session := http.requestcookie(req, "session", "guest")
  theme := http.requestcookie(req, "theme", "plain")
  return http.text(200, session + "/" + theme)
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    started := server
  when err(e) =>
    startup_error := e
endmatch`

	_, server := startHTTPServer(t, source)
	defer stopHTTPServer(t, server)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get("http://" + server.Addr + "/login")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("unexpected redirect status: got %d want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if got := resp.Header.Get("Location"); got != "/home" {
		t.Fatalf("unexpected location header: %q", got)
	}
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("unexpected Set-Cookie count: %#v", cookies)
	}
	if !slices.Contains(cookies, "session=abc; Path=/") {
		t.Fatalf("missing session cookie: %#v", cookies)
	}
	if !slices.Contains(cookies, "theme=light; Path=/") {
		t.Fatalf("missing theme cookie: %#v", cookies)
	}

	req, err := http.NewRequest(http.MethodGet, "http://"+server.Addr+"/home", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Add("Cookie", "session=abc")
	req.Header.Add("Cookie", "theme=light")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(body) != "abc/light" {
		t.Fatalf("unexpected cookie echo body: %q", string(body))
	}
}

func TestHTTPSessionNotesFlow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "session_notes.db")
	source := fmt.Sprintf(`use http
use json
use sqlite

notes_db: SqliteDB

func current_session(req: HttpRequest) -> string
  return http.requestcookie(req, "session", "guest")
endfunc

func login_count(session: string) -> result[int]
  if session = "guest" then
    return ok(0)
  endif

  match sqlite.query(notes_db, "select login_count from sessions where session_name = ? limit 1", [session])
    when ok(rows) =>
      if len(rows) = 0 then
        return ok(0)
      endif
      return ok(int(rows[0]["login_count"]))
    when err(e) =>
      return err(e)
  endmatch
endfunc

func note_count(session: string) -> result[int]
  if session = "guest" then
    return ok(0)
  endif

  match sqlite.query(notes_db, "select count(*) as total from notes where session_name = ?", [session])
    when ok(rows) =>
      if len(rows) = 0 then
        return ok(0)
      endif
      return ok(int(rows[0]["total"]))
    when err(e) =>
      return err(e)
  endmatch
endfunc

func notes_for(session: string) -> result[list[string]]
  if session = "guest" then
    return ok([])
  endif

  match sqlite.query(notes_db, "select body from notes where session_name = ? order by id", [session])
    when ok(rows) =>
      notes: list[string] := []
      for row in rows do
        append(notes, str(row["body"]))
      endfor
      return ok(notes)
    when err(e) =>
      return err(e)
  endmatch
endfunc

func bump_login_count(name: string) -> result[int]
  match sqlite.exec(notes_db, "insert into sessions(session_name, login_count) values(?, 1) on conflict(session_name) do update set login_count = login_count + 1", [name])
    when ok(rows) =>
      updated_rows := rows
    when err(e) =>
      return err(e)
  endmatch
  return login_count(name)
endfunc

func append_note(session: string, text: string) -> result[int]
  match sqlite.exec(notes_db, "insert into notes(session_name, body) values(?, ?)", [session, text])
    when ok(rows) =>
      inserted_rows := rows
    when err(e) =>
      return err(e)
  endmatch
  return note_count(session)
endfunc

func api_me(session: string) -> result[HttpReply]
  match login_count(session)
    when ok(logins) =>
      match note_count(session)
        when ok(notes_total) =>
          return http.json(200, json.objectof("session", session, "login_count", logins, "note_count", notes_total))
        when err(e) =>
          return err(e)
      endmatch
    when err(e) =>
      return err(e)
  endmatch
endfunc

func api_notes(req: HttpRequest, session: string) -> result[HttpReply]
  if session = "guest" then
    return ok(http.text(401, "login required"))
  endif

  if http.method(req) = "GET" then
    match notes_for(session)
      when ok(notes) =>
        return http.json(200, json.objectof("session", session, "count", len(notes), "notes", notes))
      when err(e) =>
        return err(e)
    endmatch
  endif

  if http.method(req) = "POST" then
    match json.parseobject(http.requestbody(req))
      when ok(payload) =>
        if not haskey(payload, "text") then
          return ok(http.text(400, "missing text"))
        endif
        text := trim(str(payload["text"]))
        if text = "" then
          return ok(http.text(400, "missing text"))
        endif
        match append_note(session, text)
          when ok(total) =>
            return http.json(201, json.objectof("session", session, "count", total, "last", text))
          when err(e) =>
            return err(e)
        endmatch
      when err(e) =>
        return ok(http.text(400, "bad json"))
    endmatch
  endif

  reply := http.withheader(http.text(405, "method not allowed"), "Allow", "GET, POST")
  return ok(reply)
endfunc

func handle(req: HttpRequest) -> result[HttpReply]
  session := current_session(req)

  matched, params := http.route(req, "/login/:name")
  if matched then
    name := trim(params["name"])
    match bump_login_count(name)
      when ok(total) =>
        login_total := total
      when err(e) =>
        return err(e)
    endmatch
    reply := http.redirect(303, "/")
    reply := http.withcookie(reply, "session", name)
    return ok(reply)
  endif

  if http.path(req) = "/api/me" then
    return api_me(session)
  endif

  if http.path(req) = "/api/notes" then
    return api_notes(req, session)
  endif

  return ok(http.text(404, "not found"))
endfunc

match sqlite.open(%q)
  when ok(db) =>
    notes_db := db
    match sqlite.exec(notes_db, "create table if not exists sessions(session_name text primary key, login_count integer not null)", [])
      when ok(rows) =>
        schema_sessions_rows := rows
      when err(e) =>
        startup_error := e
    endmatch
    match sqlite.exec(notes_db, "create table if not exists notes(id integer primary key autoincrement, session_name text not null, body text not null)", [])
      when ok(rows) =>
        schema_notes_rows := rows
        match http.listen("127.0.0.1:0", handle)
          when ok(server) =>
            started := server
          when err(e) =>
            startup_error := e
        endmatch
      when err(e) =>
        startup_error := e
    endmatch
  when err(e) =>
    startup_error := e
endmatch`, dbPath)

	interp, server := startHTTPServer(t, source)
	defer stopHTTPServer(t, server)
	if value, err := interp.GlobalEnv.Get("notes_db"); err == nil {
		if db, ok := value.(*interpreter.SqliteDBValue); ok {
			defer db.DB.Close()
		}
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get("http://" + server.Addr + "/login/Ada")
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("unexpected login status: got %d want %d", resp.StatusCode, http.StatusSeeOther)
	}
	cookies := resp.Cookies()
	if len(cookies) == 0 || cookies[0].Name != "session" || cookies[0].Value != "Ada" {
		t.Fatalf("unexpected login cookies: %#v", cookies)
	}

	req, err := http.NewRequest(http.MethodPost, "http://"+server.Addr+"/api/notes", strings.NewReader("{\"text\":\"ship it\"}"))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookies[0])
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("note create request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected note create status: got %d want %d", resp.StatusCode, http.StatusCreated)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	created := map[string]any{}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if created["session"] != "Ada" || created["last"] != "ship it" || created["count"] != float64(1) {
		t.Fatalf("unexpected note create payload: %#v", created)
	}

	req, err = http.NewRequest(http.MethodGet, "http://"+server.Addr+"/api/me", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.AddCookie(cookies[0])
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("me request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	me := map[string]any{}
	if err := json.Unmarshal(body, &me); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if me["session"] != "Ada" || me["login_count"] != float64(1) || me["note_count"] != float64(1) {
		t.Fatalf("unexpected me payload: %#v", me)
	}

	req, err = http.NewRequest(http.MethodGet, "http://"+server.Addr+"/api/notes", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.AddCookie(cookies[0])
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("notes request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	notes := map[string]any{}
	if err := json.Unmarshal(body, &notes); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	items, ok := notes["notes"].([]any)
	if !ok || len(items) != 1 || items[0] != "ship it" {
		t.Fatalf("unexpected notes payload: %#v", notes)
	}
}

func TestHTTPWithCookieDefaultsToRootPath(t *testing.T) {
	source := `use http

func handle(req: HttpRequest) -> HttpReply
  if http.path(req) = "/login" then
    reply := http.redirect(303, "/")
    reply := http.withcookie(reply, "session", "Ada")
    return reply
  endif

  return http.text(200, http.requestcookie(req, "session", "guest"))
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    started := server
  when err(e) =>
    startup_error := e
endmatch`

	_, server := startHTTPServer(t, source)
	defer stopHTTPServer(t, server)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar init failed: %v", err)
	}
	client := &http.Client{Jar: jar}

	resp, err := client.Get("http://" + server.Addr + "/login")
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(body) != "Ada" {
		t.Fatalf("cookie did not reach root path: got %q want %q", string(body), "Ada")
	}
}

func TestHTTPServerHandlesRequestsConcurrently(t *testing.T) {
	program, err := parser.Parse(`use http

func handle(req: HttpRequest) -> HttpReply
  return http.text(200, block(http.path(req)))
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    started := server
  when err(e) =>
    startup_error := e
endmatch`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	startedPaths := make(chan string, 2)
	release := make(chan struct{}, 2)
	interp.GlobalEnv.Set("block", interpreter.Builtin(func(args []any) (any, error) {
		path, ok := args[0].(string)
		if !ok {
			t.Fatalf("expected string arg, got %T", args[0])
		}
		startedPaths <- path
		<-release
		return path, nil
	}))

	if err := interp.Run(program); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	value, err := interp.GlobalEnv.Get("started")
	if err != nil {
		t.Fatalf("missing started server handle: %v", err)
	}
	server, ok := value.(*interpreter.HTTPServerValue)
	if !ok {
		t.Fatalf("started has wrong type: %T", value)
	}
	defer stopHTTPServer(t, server)

	type fetchResult struct {
		body string
		err  error
	}
	results := make(chan fetchResult, 2)
	fetch := func(path string) {
		resp, err := http.Get("http://" + server.Addr + path)
		if err != nil {
			results <- fetchResult{err: err}
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			results <- fetchResult{err: err}
			return
		}
		if resp.StatusCode != http.StatusOK {
			results <- fetchResult{err: fmt.Errorf("unexpected status for %s: %d", path, resp.StatusCode)}
			return
		}
		results <- fetchResult{body: string(body)}
	}

	go fetch("/one")

	var first string
	select {
	case first = <-startedPaths:
	case <-time.After(2 * time.Second):
		t.Fatal("first handler did not start")
	}

	go fetch("/two")

	var second string
	select {
	case second = <-startedPaths:
		if second == first {
			t.Fatalf("expected distinct handler paths, got %q twice", first)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second handler did not start while first request was still blocked")
	}

	release <- struct{}{}
	release <- struct{}{}

	seenBodies := map[string]struct{}{}
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("request failed: %v", result.err)
		}
		seenBodies[result.body] = struct{}{}
	}

	if len(seenBodies) != 2 {
		t.Fatalf("unexpected response bodies: %#v", seenBodies)
	}
	if _, ok := seenBodies[first]; !ok {
		t.Fatalf("missing first response body %q in %#v", first, seenBodies)
	}
	if _, ok := seenBodies[second]; !ok {
		t.Fatalf("missing second response body %q in %#v", second, seenBodies)
	}
}

func TestHTTPServerRequestSnapshotsIsolateModuleState(t *testing.T) {
	source := `use http

hits := dict[string, int]{}

func handle(req: HttpRequest) -> HttpReply
  path := http.path(req)
  if not haskey(hits, path) then
    hits[path] := 0
  endif
  hits[path] := hits[path] + 1
  return http.text(200, str(hits[path]))
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    started := server
  when err(e) =>
    startup_error := e
endmatch`

	_, server := startHTTPServer(t, source)
	defer stopHTTPServer(t, server)

	for idx := range 2 {
		resp, err := http.Get("http://" + server.Addr + "/hits")
		if err != nil {
			t.Fatalf("request %d failed: %v", idx+1, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read %d failed: %v", idx+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected status on request %d: got %d want %d", idx+1, resp.StatusCode, http.StatusOK)
		}
		if string(body) != "1" {
			t.Fatalf("unexpected body on request %d: got %q want %q", idx+1, string(body), "1")
		}
	}
}

func TestHTTPServerCanShareStateCellAcrossRequests(t *testing.T) {
	source := `use http
use state

hits: cell[int] := state.cell(0)

func handle(req: HttpRequest) -> HttpReply
  total := state.update(hits, (n: int) => n + 1)
  return http.text(200, str(total))
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    started := server
  when err(e) =>
    startup_error := e
endmatch`

	_, server := startHTTPServer(t, source)
	defer stopHTTPServer(t, server)

	wantBodies := []string{"1", "2"}
	for idx, want := range wantBodies {
		resp, err := http.Get("http://" + server.Addr + "/hits")
		if err != nil {
			t.Fatalf("request %d failed: %v", idx+1, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read %d failed: %v", idx+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected status on request %d: got %d want %d", idx+1, resp.StatusCode, http.StatusOK)
		}
		if string(body) != want {
			t.Fatalf("unexpected body on request %d: got %q want %q", idx+1, string(body), want)
		}
	}
}

func TestHTTPServerStaticFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.css"), []byte("body { color: teal; }\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	source := fmt.Sprintf(`use http

func handle(req: HttpRequest) -> result[HttpReply]
  matched, served := http.static(req, "/assets/", %q)
  if matched then
    match served
      when ok(reply) => return ok(reply)
      when err(e) => return ok(http.text(404, "asset missing"))
    endmatch
  endif
  return ok(http.text(404, "route miss"))
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    started := server
  when err(e) =>
    startup_error := e
endmatch`, dir)

	_, server := startHTTPServer(t, source)
	defer stopHTTPServer(t, server)

	resp, err := http.Get("http://" + server.Addr + "/assets/app.css")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("unexpected content-type: %q", got)
	}
	if string(body) != "body { color: teal; }\n" {
		t.Fatalf("unexpected body: %q", string(body))
	}

	resp, err = http.Get("http://" + server.Addr + "/other")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected route-miss status: got %d want %d", resp.StatusCode, http.StatusNotFound)
	}
	if string(body) != "route miss" {
		t.Fatalf("unexpected route-miss body: %q", string(body))
	}

	resp, err = http.Get("http://" + server.Addr + "/assets/missing.css")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected asset-miss status: got %d want %d", resp.StatusCode, http.StatusNotFound)
	}
	if string(body) != "asset missing" {
		t.Fatalf("unexpected asset-miss body: %q", string(body))
	}
}

func TestHTTPServerHandlerRuntimeErrorBecomes500(t *testing.T) {
	source := `use http

func handle(req: HttpRequest) -> HttpReply
  return missing()
endfunc

match http.listen("127.0.0.1:0", handle)
  when ok(server) =>
    started := server
  when err(e) =>
    startup_error := e
endmatch`

	_, server := startHTTPServer(t, source)
	defer stopHTTPServer(t, server)

	resp, err := http.Get("http://" + server.Addr + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected status: got %d want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if !strings.Contains(string(body), "Undefined variable: missing") {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestJSONModuleRoundTrip(t *testing.T) {
	out := runSource(t, `use json

func main()
  payload := json.objectof("name", "Ada", "roles", json.arrayof("admin", "ops"), "active", true, "deleted_at", json.null())

  match json.stringify(payload)
    when ok(text) =>
      write(text)
      match json.parseobject(text)
        when ok(obj) =>
          write(obj["name"])
          write(len(obj["roles"]))
          write(obj["active"])
          write(json.isnull(obj["deleted_at"]))
        when err(e) => write("parse failed", e)
      endmatch
    when err(e) => write("stringify failed", e)
  endmatch
endfunc`)

	if out != "{\"active\":true,\"deleted_at\":null,\"name\":\"Ada\",\"roles\":[\"admin\",\"ops\"]}\nAda\n2\ntrue\ntrue" {
		t.Fatalf("output mismatch: got %q", out)
	}
}

func TestJSONParseArray(t *testing.T) {
	out := runSource(t, `use json

func main()
  match json.parsearray("[1, 2, null, [4]]")
    when ok(items) =>
      write(len(items))
      write(items[0] + items[1])
      write(json.isnull(items[2]))
      write(len(items[3]))
    when err(e) => write("parse failed", e)
  endmatch
endfunc`)

	if out != "4\n3\ntrue\n1" {
		t.Fatalf("output mismatch: got %q want %q", out, "4\n3\ntrue\n1")
	}
}

func TestJSONParseObjectRejectsTopLevelArray(t *testing.T) {
	out := runSource(t, `use json

func main()
  match json.parseobject("[1, 2]")
    when ok(obj) => write("ok", len(obj))
    when err(e) => write(e)
  endmatch
endfunc`)

	if out != "json.parseobject() requires top-level object" {
		t.Fatalf("output mismatch: got %q", out)
	}
}

func TestJSONStringifyRejectsNonStringKeys(t *testing.T) {
	out := runSource(t, `use json

func main()
  bad := dict[int, string]{1: "x"}
  match json.stringify(bad)
    when ok(text) => write(text)
    when err(e) => write(e)
  endmatch
endfunc`)

	if out != "json.stringify() requires string dict keys, got int" {
		t.Fatalf("output mismatch: got %q", out)
	}
}

func TestJSONObjectRequiresStringKeys(t *testing.T) {
	requireRuntimeErrorContains(t, `use json

func main()
  json.objectof(1, "x")
endfunc`, "json.objectof() key 1 must be string, got int")
}

func TestSqliteModuleCRUD(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "notes.db")
	out := runSource(t, fmt.Sprintf(`use sqlite
use json

func main()
  match sqlite.open(%q)
    when ok(db) =>
      match sqlite.exec(db, "create table notes(id integer primary key, body text, visits integer, deleted_at text)", [])
        when ok(created) => write(created)
        when err(e) => write("create failed", e)
      endmatch

      match sqlite.exec(db, "insert into notes(body, visits, deleted_at) values(?, ?, ?)", ["ship it", 3, json.null()])
        when ok(inserted) => write(inserted)
        when err(e) => write("insert failed", e)
      endmatch

      match sqlite.query(db, "select body, visits, deleted_at from notes order by id", [])
        when ok(rows) =>
          first := rows[0]
          write(first["body"])
          write(first["visits"])
          write(typeof(first["deleted_at"]))
          write(json.isnull(first["deleted_at"]))
        when err(e) =>
          write("query failed", e)
      endmatch

      match sqlite.close(db)
        when ok(code) => write(code)
        when err(e) => write("close failed", e)
      endmatch
    when err(e) =>
      write("open failed", e)
  endmatch
endfunc`, dbPath))

	if out != "0\n1\nship it\n3\nJsonNull\ntrue\n0" {
		t.Fatalf("output mismatch: got %q", out)
	}
}

func TestSqliteOpenReturnsErrOnMissingParentDir(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing", "notes.db")
	out := runSource(t, fmt.Sprintf(`use sqlite

func main()
  match sqlite.open(%q)
    when ok(db) =>
      write("opened", db)
    when err(e) =>
      write("err")
  endmatch
endfunc`, dbPath))

	if out != "err" {
		t.Fatalf("output mismatch: got %q want %q", out, "err")
	}
}

func TestSqliteExecRejectsUnsupportedParamTypeAtRuntime(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "notes.db")
	requireRuntimeErrorContains(t, fmt.Sprintf(`use sqlite

func main()
  match sqlite.open(%q)
    when ok(db) =>
      sqlite.exec(db, "select ?", [dict[string, int]{"x": 1}])
    when err(e) =>
      write(e)
  endmatch
endfunc`, dbPath), "sqlite.exec() param 1 only supports int/float/string/bool/json.null(), got dict")
}

func TestHTTPModuleGetRejectsNegativeTimeout(t *testing.T) {
	requireRuntimeErrorContains(t, `use http

func main()
  http.get("https://example.com", -1)
endfunc`, "http.get() timeoutms must be >= 0")
}

func TestHTTPRedirectRejectsNonRedirectStatus(t *testing.T) {
	requireRuntimeErrorContains(t, `use http

func main()
  http.redirect(200, "/home")
endfunc`, "http.redirect() status must be between 300 and 399")
}

func TestSleepRejectsNegativeDuration(t *testing.T) {
	requireRuntimeErrorContains(t, `use sleep from time

func main()
  sleep(-1)
endfunc`, "sleep() duration must be >= 0")
}

func TestMoneyBasics(t *testing.T) {
	out := runSource(t, `func main()
  price: money[USD] := 19.99
  tax: money[USD] := 1.5
  total := price + tax
  write(total)
  write(typeof(total))
  doubled := price * 2
  half := price / 2
  write(doubled)
  write(half)
  ratio := tax / price
  write(ratio)
endfunc`)
	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Fatalf("unexpected output: %q", out)
	}
	if lines[0] != "21.49 USD" {
		t.Fatalf("money add mismatch: got %q", lines[0])
	}
	if lines[1] != "money[USD]" {
		t.Fatalf("typeof mismatch: got %q", lines[1])
	}
	if lines[2] != "39.98 USD" {
		t.Fatalf("money mul mismatch: got %q", lines[2])
	}
	if lines[3] != "9.995 USD" {
		t.Fatalf("money div mismatch: got %q", lines[3])
	}
	if !strings.HasPrefix(lines[4], "0.075") {
		t.Fatalf("money ratio mismatch: got %q", lines[4])
	}
}

func TestVarDefaultMoneyZero(t *testing.T) {
	out := runSource(t, `func main()
  var default
    m: money[USD]
  endvar
  write(m)
endfunc`)
	if out != "0.00 USD" {
		t.Fatalf("output mismatch: got %q want %q", out, "0.00 USD")
	}
}

func TestGlobalUpdatesModuleScope(t *testing.T) {
	out := runSource(t, `count: int := 0

func bump()
  global count := count + 1
endfunc

func main()
  bump()
  bump()
  write(count)
endfunc`)
	if out != "2" {
		t.Fatalf("output mismatch: got %q want %q", out, "2")
	}
}

func TestGlobalUpdatesOuterFunctionScope(t *testing.T) {
	out := runSource(t, `func outer() -> int
  value: int := 10

  func inner()
    global value := value + 5
  endfunc

  inner()
  return value
endfunc

write(outer())`)
	if out != "15" {
		t.Fatalf("output mismatch: got %q want %q", out, "15")
	}
}

func TestGlobalMissingOuterBindingFailsAtRuntime(t *testing.T) {
	program, err := parser.Parse(`func main()
  global missing := 1
endfunc`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	err = interp.Run(program)
	if err == nil {
		t.Fatal("expected runtime error")
	}
	if err.Error() != "runtime error at L2: global variable 'missing' not found in any outer scope" {
		t.Fatalf("error mismatch: got %q", err.Error())
	}
}

func TestGlobalCannotMutateBuiltin(t *testing.T) {
	program, err := parser.Parse(`func main()
  global write := 1
endfunc`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	err = interp.Run(program)
	if err == nil {
		t.Fatal("expected runtime error")
	}
	if err.Error() != "runtime error at L2: Cannot assign to builtin 'write' with global" {
		t.Fatalf("error mismatch: got %q", err.Error())
	}
}

func TestStateCellGetSetAndUpdate(t *testing.T) {
	out := runSource(t, `use state

func main()
  counter: cell[int] := state.cell(1)
  write(state.get(counter))
  write(state.set(counter, 2))
  write(state.get(counter))
  write(state.update(counter, (n: int) => n + 3))
  write(state.get(counter))
endfunc`)
	if out != "1\n2\n2\n5\n5" {
		t.Fatalf("output mismatch: got %q want %q", out, "1\n2\n2\n5\n5")
	}
}

func TestStateCellReturnsSnapshots(t *testing.T) {
	out := runSource(t, `use state

func main()
  items := [1]
  saved: cell[list[int]] := state.cell(items)
  append(items, 2)

  snapshot := state.get(saved)
  append(snapshot, 3)
  write(state.get(saved))

  next := [7]
  state.set(saved, next)
  append(next, 8)
  write(state.get(saved))
endfunc`)
	if out != "[1]\n[7]" {
		t.Fatalf("output mismatch: got %q want %q", out, "[1]\n[7]")
	}
}

func TestParallelCanShareStateCellExplicitly(t *testing.T) {
	out := runSource(t, `use state

func main()
  counter: cell[int] := state.cell(0)
  parallel do
    state.update(counter, (n: int) => n + 1)
    state.update(counter, (n: int) => n + 1)
  endparallel
  write(state.get(counter))
endfunc`)
	if out != "2" {
		t.Fatalf("output mismatch: got %q want %q", out, "2")
	}
}

func TestStateCellRejectsCallablePayloads(t *testing.T) {
	requireRuntimeErrorContains(t, `use state

func main()
  bad := state.cell((x: int) => x + 1)
  write(bad)
endfunc`, "state.cell() only supports data values, got func")
}

func TestParallelRunsTasksConcurrently(t *testing.T) {
	program, err := parser.Parse(`parallel do
  block(1)
  block(2)
endparallel`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	started := make(chan int64, 2)
	release := make(chan struct{})
	done := make(chan error, 1)

	interp.GlobalEnv.Set("block", interpreter.Builtin(func(args []any) (any, error) {
		id, ok := args[0].(int64)
		if !ok {
			t.Fatalf("expected int64 arg, got %T", args[0])
		}
		started <- id
		<-release
		return id, nil
	}))

	go func() {
		done <- interp.Run(program)
	}()

	deadline := time.After(300 * time.Millisecond)
	seen := map[int64]struct{}{}
	for len(seen) < 2 {
		select {
		case id := <-started:
			seen[id] = struct{}{}
		case <-deadline:
			t.Fatal("parallel block did not start both tasks concurrently")
		}
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestParallelResultOrderUsesExpressionValues(t *testing.T) {
	interp, _ := runProgram(t, `func identity(x: int) -> int
  return x
endfunc

parallel => results do
  identity(1)
  identity(2)
endparallel`)

	value, err := interp.GlobalEnv.Get("results")
	if err != nil {
		t.Fatalf("missing results: %v", err)
	}
	results := requireListItems(t, value)
	if len(results) != 2 {
		t.Fatalf("results length mismatch: got %d want 2", len(results))
	}
	for idx, want := range []int64{1, 2} {
		okValue, ok := results[idx].(*interpreter.OkValue)
		if !ok {
			t.Fatalf("result %d type mismatch: got %T", idx, results[idx])
		}
		got, ok := okValue.Value.(int64)
		if !ok || got != want {
			t.Fatalf("result %d mismatch: got %#v want %d", idx, okValue.Value, want)
		}
	}
}

func TestParallelAllowFailPreservesSourceOrder(t *testing.T) {
	program, err := parser.Parse(`parallel allowfail => results do
  task(1)
  task(2)
  task(3)
endparallel`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	interp.GlobalEnv.Set("task", interpreter.Builtin(func(args []any) (any, error) {
		id := args[0].(int64)
		switch id {
		case 1:
			time.Sleep(60 * time.Millisecond)
			return int64(10), nil
		case 2:
			time.Sleep(10 * time.Millisecond)
			return nil, fmt.Errorf("boom %d", id)
		default:
			time.Sleep(20 * time.Millisecond)
			return int64(30), nil
		}
	}))

	if err := interp.Run(program); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	value, err := interp.GlobalEnv.Get("results")
	if err != nil {
		t.Fatalf("missing results: %v", err)
	}
	results := requireListItems(t, value)
	if len(results) != 3 {
		t.Fatalf("results length mismatch: got %d want 3", len(results))
	}

	first, ok := results[0].(*interpreter.OkValue)
	if !ok || first.Value != int64(10) {
		t.Fatalf("result 0 mismatch: got %#v", results[0])
	}
	second, ok := results[1].(*interpreter.ErrValue)
	if !ok || second.Value != "boom 2" {
		t.Fatalf("result 1 mismatch: got %#v", results[1])
	}
	third, ok := results[2].(*interpreter.OkValue)
	if !ok || third.Value != int64(30) {
		t.Fatalf("result 2 mismatch: got %#v", results[2])
	}
}

func TestParallelReturnsFirstSourceOrderError(t *testing.T) {
	program, err := parser.Parse(`parallel do
  fail(1)
  fail(2)
endparallel`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	interp.GlobalEnv.Set("fail", interpreter.Builtin(func(args []any) (any, error) {
		id := args[0].(int64)
		if id == 1 {
			time.Sleep(80 * time.Millisecond)
		} else {
			time.Sleep(10 * time.Millisecond)
		}
		return nil, fmt.Errorf("boom %d", id)
	}))

	err = interp.Run(program)
	if err == nil {
		t.Fatal("expected parallel block to fail")
	}
	if err.Error() != "boom 1" {
		t.Fatalf("error mismatch: got %q want %q", err.Error(), "boom 1")
	}
}

func TestParallelDoesNotLeakOuterAssignments(t *testing.T) {
	out := runSource(t, `x := 1
parallel do
  x := 2
endparallel
write(x)`)
	if out != "1" {
		t.Fatalf("output mismatch: got %q want %q", out, "1")
	}
}
