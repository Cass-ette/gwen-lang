package interpreter_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cass-ette/gwen-lang/internal/interpreter"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

func runSource(t *testing.T, source string) string {
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
	return strings.TrimSpace(out.String())
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
	if out != "True" {
		t.Fatalf("output mismatch: got %q want %q", out, "True")
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
	if out != "[1, 3]" {
		t.Fatalf("output mismatch: got %q want %q", out, "[1, 3]")
	}
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
	want := "Original: [1, 2, 3, 4, 5]\nDoubled: [2, 4, 6, 8, 10]\nEvens: [2, 4]\nIndexed: [[0, 'a'], [1, 'b'], [2, 'c']]"
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
	if !strings.Contains(out, "has zoe? False") {
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
