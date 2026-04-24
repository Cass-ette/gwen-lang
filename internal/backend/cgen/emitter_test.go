package cgen_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Cass-ette/gwen-lang/internal/backend/cgen"
	"github.com/Cass-ette/gwen-lang/internal/frontend"
	"github.com/Cass-ette/gwen-lang/internal/interpreter"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

func mustEmitC(t *testing.T, source string) string {
	t.Helper()

	unit, err := frontend.AnalyzeSource(source, "test.gw")
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	out, err := cgen.EmitProgram(unit.MIR)
	if err != nil {
		t.Fatalf("emit failed: %v", err)
	}
	return out
}

func mustEmitCPath(t *testing.T, path string) string {
	t.Helper()

	unit, err := frontend.AnalyzePath(path)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	out, err := cgen.EmitProgram(unit.MIR)
	if err != nil {
		t.Fatalf("emit failed: %v", err)
	}
	return out
}

func emitCError(t *testing.T, source string) error {
	t.Helper()

	unit, err := frontend.AnalyzeSource(source, "test.gw")
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	_, err = cgen.EmitProgram(unit.MIR)
	return err
}

func requireExternalTools(t *testing.T, names ...string) {
	t.Helper()

	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("skipping integration test: missing %s", name)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root")
		}
		dir = parent
	}
}

func buildCompiledProgramFromPath(t *testing.T, sourcePath string) string {
	t.Helper()

	requireExternalTools(t, "cc")
	cSource := mustEmitCPath(t, sourcePath)
	outDir := t.TempDir()
	cPath := filepath.Join(outDir, "program.c")
	binPath := filepath.Join(outDir, "program")
	if err := os.WriteFile(cPath, []byte(cSource), 0o644); err != nil {
		t.Fatalf("write emitted C failed: %v", err)
	}
	cmd := exec.Command("cc", cPath, "-o", binPath, "-pthread")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cc failed: %v\n%s", err, string(output))
	}
	return binPath
}

func buildCompiledProgram(t *testing.T, source string) string {
	t.Helper()

	requireExternalTools(t, "cc")
	cSource := mustEmitC(t, source)
	outDir := t.TempDir()
	cPath := filepath.Join(outDir, "program.c")
	binPath := filepath.Join(outDir, "program")
	if err := os.WriteFile(cPath, []byte(cSource), 0o644); err != nil {
		t.Fatalf("write emitted C failed: %v", err)
	}
	cmd := exec.Command("cc", cPath, "-o", binPath, "-pthread")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cc failed: %v\n%s", err, string(output))
	}
	return binPath
}

func runInterpretedProgramFromPath(t *testing.T, sourcePath string, args ...string) string {
	t.Helper()

	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source failed: %v", err)
	}
	program, err := parser.Parse(string(source))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	interp := interpreter.New()
	interp.ProgramArgs = append([]string(nil), args...)
	var out bytes.Buffer
	interp.Stdout = &out
	if err := interp.RunWithSource(program, sourcePath); err != nil {
		t.Fatalf("interpreter run failed: %v", err)
	}
	return strings.TrimSpace(out.String())
}

var runtimeErrorPrefixPattern = regexp.MustCompile(`runtime error at L\d+: `)

func normalizeComparableOutput(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for idx, line := range lines {
		lines[idx] = normalizeComparableLine(line)
	}
	return strings.Join(lines, "\n")
}

func normalizeComparableLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	line = runtimeErrorPrefixPattern.ReplaceAllString(line, "")
	if strings.HasPrefix(line, "expected err:") {
		lowered := strings.ToLower(line)
		if strings.Contains(lowered, "no such file or directory") {
			return "expected err: no such file or directory"
		}
	}
	if lowered := strings.ToLower(line); strings.Contains(lowered, "no such file or directory") {
		idx := strings.Index(lowered, "no such file or directory")
		line = line[:idx] + "no such file or directory"
	}
	if json.Valid([]byte(line)) {
		var value any
		if err := json.Unmarshal([]byte(line), &value); err == nil {
			if encoded, err := json.Marshal(value); err == nil {
				return string(encoded)
			}
		}
	}
	return line
}

func reserveLocalAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener failed: %v", err)
	}
	return addr
}

func waitForHTTPReady(t *testing.T, url string, waitCh <-chan error, logs *bytes.Buffer) {
	t.Helper()

	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		select {
		case err := <-waitCh:
			t.Fatalf("server exited before becoming ready: %v\n%s", err, logs.String())
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server did not become ready in time\n%s", logs.String())
}

func doHTTP(t *testing.T, client *http.Client, method, url, body, cookie string) (int, string, http.Header) {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return resp.StatusCode, strings.TrimSpace(string(data)), resp.Header
}

func decodeJSONBody[T any](t *testing.T, text string) T {
	t.Helper()

	var out T
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("decode json failed: %v\nbody: %s", err, text)
	}
	return out
}

func cleanupCommand(t *testing.T, cancel context.CancelFunc, waitCh <-chan error, logs *bytes.Buffer) {
	t.Helper()

	cancel()
	select {
	case err := <-waitCh:
		var exitErr *exec.ExitError
		if err != nil && !errors.Is(err, context.Canceled) && !errors.As(err, &exitErr) {
			t.Logf("compiled process exited during cleanup: %v\n%s", err, logs.String())
		}
	case <-time.After(2 * time.Second):
		t.Logf("compiled process did not exit during cleanup\n%s", logs.String())
	}
}

func TestEmitProgramWithHelperAndMain(t *testing.T) {
	out := mustEmitC(t, `func double(x: int) -> int
  return x * 2
endfunc

func main()
  write(double(21))
endfunc`)

	wants := []string{
		"#include <stdbool.h>",
		"static long long gwen_fn_double(long long slot_1);",
		"static void gwen_fn_main(void);",
		"tmp_3 = (slot_1 * 2LL);",
		"tmp_3 = gwen_fn_double(21LL);",
		"gwen_write_int(tmp_3);",
		"gwen_fn_main();",
		"int main(int argc, char **argv) {",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithTopLevelScriptAndWhile(t *testing.T) {
	out := mustEmitC(t, `x := 0
while x < 3 do
  write(x)
  x := x + 1
endwhile`)

	wants := []string{
		"static void gwen_script_1(void);",
		"static void gwen_script_1(void) {",
		"goto block_1;",
		"block_2:",
		"gwen_script_1();",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithForRange(t *testing.T) {
	out := mustEmitC(t, `func main()
  total := 0
  for i in 1 to 3 do
    total := total + i
  endfor
  write(total)
endfunc`)

	wants := []string{
		"bool gwen_loop_started_2 = false;",
		"long long gwen_loop_current_2 = 0;",
		"gwen_loop_step_2 = gwen_loop_current_2 <= gwen_loop_end_2 ? 1LL : -1LL;",
		"slot_2 = (long long)(gwen_loop_current_2);",
		"goto block_4;",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithForEach(t *testing.T) {
	out := mustEmitC(t, `func main()
  total := 0
  for item in [1, 2, 3] do
    total := total + item
  endfor
  write(total)
endfunc`)

	wants := []string{
		"typedef struct {",
		"long long len;",
		"long long *items;",
		"bool gwen_loop_started_2 = false;",
		"long long gwen_loop_index_2 = 0;",
		"tmp_5.items = (long long *)malloc(sizeof(long long) * 3ULL);",
		"if (gwen_loop_index_2 < tmp_5.len) {",
		"slot_2 = (long long)(tmp_5.items[gwen_loop_index_2]);",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithForEachIndex(t *testing.T) {
	out := mustEmitC(t, `func main()
  total := 0
  for item in [10, 20, 30] with index idx do
    total := total + item + idx
  endfor
  write(total)
endfunc`)

	wants := []string{
		"slot_2 = (long long)(tmp_5.items[gwen_loop_index_2]);",
		"slot_init_2 = true;",
		"slot_3 = (long long)(gwen_loop_index_2);",
		"slot_init_3 = true;",
		"tmp_8 = ((slot_init_1 ? slot_1",
		"+ (slot_init_2 ? slot_2",
		"tmp_10 = (tmp_8 + (slot_init_3 ? slot_3",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithLenAndIndex(t *testing.T) {
	out := mustEmitC(t, `func main()
  items := [4, 5, 6]
  mid := items[1]
  text := "go"
  write(len(items), mid, len(text), text[1])
endfunc`)

	wants := []string{
		"static long long gwen_string_len(const char *value) {",
		"static const char *gwen_string_index(const char *value, long long index) {",
		"if (((long long)(1LL)) < 0 || ((long long)(1LL)) >= ((slot_init_1 ? slot_1",
		"tmp_7 = (long long)(((slot_init_1 ? slot_1",
		"tmp_11 = ((slot_init_1 ? slot_1",
		"tmp_15 = gwen_string_len((slot_init_3 ? slot_3",
		"tmp_18 = gwen_string_index((slot_init_3 ? slot_3",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithDynamicWrite(t *testing.T) {
	out := mustEmitC(t, `use json

func main()
  payload := json.objectof("name", "gwen")
  write(payload["name"])
endfunc`)

	if !strings.Contains(out, "gwen_write_string(gwen_value_display_string(") {
		t.Fatalf("emitted C missing dynamic write helper\n%s", out)
	}
}

func TestEmitProgramWithTypedListAppend(t *testing.T) {
	out := mustEmitC(t, `func main()
  notes: list[string] := []
  append(notes, "hi")
  write(notes[0])
endfunc`)

	wants := []string{
		"static void gwen_list_1_append(",
		"gwen_list_1_append(&(slot_1), \"hi\");",
		".items[(long long)(0LL)]",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithTypedListToDynamicJSON(t *testing.T) {
	out := mustEmitC(t, `use json

func main()
  notes: list[string] := []
  append(notes, "hi")
  payload := json.objectof("notes", notes)
  write(str(payload["notes"]))
endfunc`)

	wants := []string{
		"static gwen_value gwen_list_1_to_value(",
		"gwen_list_1_to_value((slot_init_1 ? slot_1",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithObjectMethods(t *testing.T) {
	out := mustEmitC(t, `object Counter
  n: int

  new() -> Counter
    return Counter{n := 0}
  endnew

  func inc(self: Counter) -> int
    self.n := self.n + 1
    return self.n
  endfunc
endobject

func main()
  c := Counter.new()
  write(c.inc(), Counter.inc(c))
endfunc`)

	wants := []string{
		"typedef struct gwen_object_Counter gwen_object_Counter;",
		"struct gwen_object_Counter {",
		"long long n;",
		"static gwen_object_Counter * gwen_object_Counter_new(void);",
		"static long long gwen_object_Counter_inc(gwen_object_Counter * slot_1);",
		"= (gwen_object_Counter *)calloc(1U, sizeof(gwen_object_Counter));",
		"= gwen_object_Counter_new();",
		"= gwen_object_Counter_inc((slot_init_1 ? slot_1",
		"((slot_init_1 ? slot_1",
		")->n = tmp_",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithFunctionValues(t *testing.T) {
	out := mustEmitC(t, `func double(x: int) -> int
  return x * 2
endfunc

func apply(f: (int) -> int, x: int) -> int
  return f(x)
endfunc

func main()
  op: (int) -> int := double
  write(op(5), apply(double, 6))
endfunc`)

	wants := []string{
		"long long (*call)(void *, long long);",
		"static long long gwen_func_1_call(gwen_func_1 fn, long long arg_1) {",
		"static long long gwen_fn_apply(gwen_func_1 slot_1, long long slot_2);",
		"= (gwen_func_1){NULL, gwen_fn_double_closure_call};",
		"gwen_fn_apply((gwen_func_1){NULL, gwen_fn_double_closure_call}, 6LL);",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithMapAndFilter(t *testing.T) {
	out := mustEmitC(t, `func double(x: int) -> int
  return x * 2
endfunc

func even(x: int) -> bool
  return x mod 2 = 0
endfunc

func main()
  items := [1, 2, 3, 4]
  op: (int) -> int := double
  pred: (int) -> bool := even
  doubled := map(items, op)
  kept := filter(items, pred)
  write(doubled[2], len(kept), kept[1])
endfunc`)

	wants := []string{
		"gwen_map_source_",
		"gwen_filter_source_",
		"(gwen_func_1){NULL, NULL}",
		"(gwen_func_2){NULL, NULL}",
		"= (gwen_func_1){NULL, gwen_fn_double_closure_call};",
		"= (gwen_func_2){NULL, gwen_fn_even_closure_call};",
		"gwen_func_1_call(",
		"gwen_func_2_call(",
		"out of memory in map()",
		"out of memory in filter()",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithRangeAndEnumerate(t *testing.T) {
	out := mustEmitC(t, `func main()
  up := range(2, 6, 2)
  down := range(3, 1)
  indexed := enumerate(["a", "b", "c"])
  write(up[1], down[2], indexed[1][0], indexed[1][1])
endfunc`)

	wants := []string{
		"gwen_range_start_",
		"gwen_range_step_",
		"out of memory in range()",
		"range() step cannot be 0",
		"gwen_enumerate_source_",
		"gwen_enumerate_pair_",
		"gwen_value_int(i)",
		"gwen_value_list_from_ptr(gwen_dyn_list_new(2))",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithMultiReturnFunctionValues(t *testing.T) {
	out := mustEmitC(t, `func pair(x: int) -> int, int
  return x, x + 1
endfunc

func main()
  f: (int) -> int, int := pair
  left, right := f(4)
  write(left, right)
endfunc`)

	wants := []string{
		"typedef struct gwen_tuple_1 gwen_tuple_1;",
		"gwen_tuple_1 (*call)(void *, long long);",
		"gwen_func_1 slot_1 = (gwen_func_1){NULL, NULL};",
		"= (gwen_func_1){NULL, gwen_fn_pair_closure_call};",
		"gwen_func_1_call(",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithBoundInstanceMethodValue(t *testing.T) {
	out := mustEmitC(t, `object Counter
  n: int

  new() -> Counter
    return Counter{n := 0}
  endnew

  func add(self: Counter, delta: int) -> int
    self.n := self.n + delta
    return self.n
  endfunc
endobject

func main()
  c := Counter.new()
  op: (int) -> int := c.add
  write(op(2))
endfunc`)

	wants := []string{
		"gwen_object_Counter_add_bound_closure_env",
		"gwen_object_Counter_add_bound_closure_new",
		"gwen_object_Counter_add_bound_closure_call",
		"return gwen_object_Counter_add(captures->receiver, arg_1);",
		"= gwen_object_Counter_add_bound_closure_new(",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithCapturingLambdaExpression(t *testing.T) {
	out := mustEmitC(t, `func main()
  factor := 2
  apply: (int) -> int := (x: int) => x * factor
  write(apply(4))
endfunc`)

	wants := []string{
		"gwen_fn_lambda_1_closure_env",
		"gwen_fn_lambda_1_closure_new",
		"return gwen_fn_lambda_1(captures->capture_",
		"gwen_func_1_call(",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithResultMatch(t *testing.T) {
	out := mustEmitC(t, `func safe_div(x: int, y: int) -> result[int, string]
  if y = 0 then
    return err("division by zero")
  endif
  return ok(x / y)
endfunc

func main()
  match safe_div(10, 2)
    when ok(value) =>
      write(value)
    when err(reason) =>
      write(reason)
  endmatch
endfunc`)

	wants := []string{
		"typedef struct {",
		"bool is_ok;",
		"long long ok;",
		"const char * err;",
		"tmp_5 = (gwen_result_1){false, 0, \"division by zero\"};",
		"tmp_9 = (gwen_result_1){true, tmp_8, NULL};",
		"return tmp_5;",
		"return tmp_9;",
		"if ((tmp_4).is_ok) {",
		"slot_1 = (long long)((tmp_4).ok);",
		"if (!(tmp_4).is_ok) {",
		"slot_2 = (const char *)((tmp_4).err);",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithValueMatch(t *testing.T) {
	out := mustEmitC(t, `func main()
  score := 5
  match score
    when 1 to 3 =>
      write("low")
    when 4, 5 =>
      write("mid")
    when n =>
      write(n)
  endmatch

  text := "go"
  match text
    when "go" =>
      write("ok")
    when other =>
      write(other)
  endmatch
endfunc`)

	wants := []string{
		"if ((slot_init_1 ? slot_1",
		"<= 3LL) {",
		"== 4LL) {",
		"== 5LL) {",
		"slot_2 = (long long)((slot_init_1 ? slot_1",
		"slot_init_2 = true;",
		"if (gwen_string_eq((slot_init_3 ? slot_3",
		"slot_4 = (const char *)((slot_init_3 ? slot_3",
		"slot_init_4 = true;",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithDictReadPath(t *testing.T) {
	out := mustEmitC(t, `func main()
  scores := dict[string, int]{"alice": 90, "bob": 85}
  users := dict[int, string]{1: "alice", 2: "bob"}
  write(scores["alice"], len(scores), haskey(scores, "zoe"), get(scores, "zoe", 0), users[1], haskey(users, 3), get(users, 3, "none"))
endfunc`)

	wants := []string{
		"typedef struct {",
		"const char * *keys;",
		"long long *values;",
		"static bool gwen_dict_1_haskey(gwen_dict_1 dict, const char * key) {",
		"if (gwen_string_eq(dict.keys[i], key)) return true;",
		"static long long gwen_dict_1_get(gwen_dict_1 dict, const char * key, long long fallback) {",
		"static const char * gwen_dict_2_index(gwen_dict_2 dict, long long key) {",
		".keys = (const char * *)malloc(sizeof(const char *) * 2ULL);",
		"= gwen_dict_1_index((slot_init_1 ? slot_1",
		"= ((slot_init_1 ? slot_1",
		"= gwen_dict_1_haskey((slot_init_1 ? slot_1",
		"= gwen_dict_1_get((slot_init_1 ? slot_1",
		"= gwen_dict_2_index((slot_init_2 ? slot_2",
		"= gwen_dict_2_haskey((slot_init_2 ? slot_2",
		"= gwen_dict_2_get((slot_init_2 ? slot_2",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithIndexStore(t *testing.T) {
	out := mustEmitC(t, `func main()
  items := [1, 2, 3]
  items[1] := 7
  scores := dict[string, int]{"alice": 90}
  scores["bob"] := 85
  scores["alice"] := 95
  write(items[1], scores["alice"], scores["bob"], len(scores))
endfunc`)

	wants := []string{
		"static void gwen_dict_1_set(gwen_dict_1 *dict, const char * key, long long value) {",
		"dict->values[i] = value;",
		"dict->keys[dict->len] = key;",
		"dict->len = new_len;",
		".items[(long long)(1LL)] = (long long)(7LL);",
		"gwen_dict_1_set(&(slot_2), \"bob\", 85LL);",
		"gwen_dict_1_set(&(slot_2), \"alice\", 95LL);",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithDictKeysAndValues(t *testing.T) {
	out := mustEmitC(t, `func main()
  scores := dict[string, int]{"alice": 90, "bob": 85}
  total := 0
  for name in keys(scores) do
    total := total + scores[name]
  endfor
  for score in values(scores) do
    total := total + score
  endfor
  write(total)
endfunc`)

	wants := []string{
		"static gwen_list_1 gwen_dict_1_keys(gwen_dict_1 dict) {",
		"static gwen_list_2 gwen_dict_1_values(gwen_dict_1 dict) {",
		"= gwen_dict_1_keys((slot_init_1 ? slot_1",
		"= gwen_dict_1_values((slot_init_1 ? slot_1",
		"if (gwen_loop_index_2 < tmp_9.len) {",
		"if (gwen_loop_index_5 < tmp_17.len) {",
		"slot_3 = (const char *)(tmp_9.items[gwen_loop_index_2]);",
		"slot_init_3 = true;",
		"slot_4 = (long long)(tmp_17.items[gwen_loop_index_5]);",
		"slot_init_4 = true;",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithImportedStdlibBuiltins(t *testing.T) {
	out := mustEmitC(t, `use keys, get from dict

func main()
  scores := dict[string, int]{"alice": 90, "bob": 85}
  write(get(scores, "zoe", 0), len(keys(scores)))
endfunc`)

	wants := []string{
		"static long long gwen_dict_1_get(gwen_dict_1 dict, const char * key, long long fallback) {",
		"static gwen_list_1 gwen_dict_1_keys(gwen_dict_1 dict) {",
		"= gwen_dict_1_get((slot_init_1 ? slot_1",
		"= gwen_dict_1_keys((slot_init_1 ? slot_1",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithImportedStringAndMathBuiltins(t *testing.T) {
	out := mustEmitC(t, `use replace, trim, contains, startswith, endswith from string
use abs, min, max, sqrt, floor, ceil from math

func main()
  x := -3
  a := 3
  b := 5
  left := "apple"
  right := "banana"
  write(replace("hello hello", "hello", "hi"))
  write(startswith("gwen-lang", "gwen"), endswith("gwen-lang", "lang"), contains("gwen-lang", "-"), trim("  hi  "))
  write(abs(x), min(a, b), max(a, b), min(left, right), max(left, right), sqrt(4.0), floor(2.9), ceil(2.1))
endfunc`)

	wants := []string{
		"static bool gwen_string_startswith(const char *value, const char *prefix) {",
		"static const char *gwen_string_trim(const char *value) {",
		"static const char *gwen_string_replace(const char *value, const char *old_value, const char *new_value) {",
		"static int gwen_string_cmp(const char *left, const char *right) {",
		"= gwen_string_replace(\"hello hello\", \"hello\", \"hi\");",
		"= gwen_string_startswith(\"gwen-lang\", \"gwen\");",
		"= gwen_string_endswith(\"gwen-lang\", \"lang\");",
		"= gwen_string_contains(\"gwen-lang\", \"-\");",
		"= gwen_string_trim(\"  hi  \");",
		"< 0 ? -(",
		"gwen_string_cmp(",
		"= sqrt(",
		"= floor(",
		"= ceil(",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithCompiledStringCollectionHelpers(t *testing.T) {
	out := mustEmitC(t, `use split, join, substring from string

func main()
  parts := split("a,b,c", ",")
  whole := "gw" + "en"
  write(len(parts), parts[1], join(parts, "-"), substring(whole, 1, 2))
endfunc`)

	wants := []string{
		"static const char *gwen_string_concat(const char *left, const char *right) {",
		"static const char *gwen_string_substring(const char *value, long long start, long long end) {",
		"static gwen_list_1 gwen_string_split(const char *value, const char *sep) {",
		"static const char *gwen_list_1_join(gwen_list_1 items, const char *sep) {",
		"= gwen_string_split(\"a,b,c\", \",\");",
		"= gwen_string_concat(\"gw\", \"en\");",
		"= gwen_list_1_join((slot_init_1 ? slot_1",
		"= gwen_string_substring((slot_init_2 ? slot_2",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithImportedIOBuiltins(t *testing.T) {
	out := mustEmitC(t, `use readfile, readdir, writefile, appendfile from io

func main()
  wrote := writefile("/tmp/gwen_cgen_io.txt", "hi")
  appended := appendfile("/tmp/gwen_cgen_io.txt", "\n")
  content := readfile("/tmp/gwen_cgen_io.txt")
  entries := readdir("/tmp")
  match wrote
    when ok(n) =>
      write(n)
    when err(e) =>
      write(e)
  endmatch
  match appended
    when ok(n) =>
      write(n)
    when err(e) =>
      write(e)
  endmatch
  match content
    when ok(text) =>
      write(text)
    when err(reason) =>
      write(reason)
  endmatch
  match entries
    when ok(names) =>
      write(len(names))
    when err(reason) =>
      write(reason)
  endmatch
endfunc`)

	wants := []string{
		"#include <dirent.h>",
		"#include <errno.h>",
		"gwen_io_readfile(const char *path) {",
		"gwen_io_readdir(const char *path) {",
		"gwen_io_writefile(const char *path, const char *content) {",
		"gwen_io_appendfile(const char *path, const char *content) {",
		"= gwen_io_writefile(\"/tmp/gwen_cgen_io.txt\", \"hi\");",
		"= gwen_io_appendfile(\"/tmp/gwen_cgen_io.txt\", \"\\n\");",
		"= gwen_io_readfile(\"/tmp/gwen_cgen_io.txt\");",
		"= gwen_io_readdir(\"/tmp\");",
		"qsort(ok_value.items, (size_t)ok_value.len, sizeof(const char *), gwen_string_ptr_cmp);",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithStdlibModuleMemberBuiltins(t *testing.T) {
	out := mustEmitC(t, `use string
use path

func main()
  write(string.trim("  hi  "))
  write(string.join(string.split("a,b", ","), "/"), string.substring("gwen", 1, 2))
  write(path.basename("docs/stdlib.md"), path.dirname("docs/stdlib.md"), path.joinpath("docs", "stdlib.md"))
endfunc`)

	wants := []string{
		"= gwen_string_trim(\"  hi  \");",
		"= gwen_string_split(\"a,b\", \",\");",
		"= gwen_list_1_join(tmp_",
		"= gwen_string_substring(\"gwen\", 1LL, 2LL);",
		"= gwen_path_basename(\"docs/stdlib.md\");",
		"= gwen_path_dirname(\"docs/stdlib.md\");",
		"= gwen_path_join(\"docs\", \"stdlib.md\");",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithUnusedStdlibUse(t *testing.T) {
	out := mustEmitC(t, `use http

func main()
  write(1)
endfunc`)

	wants := []string{
		"static void gwen_fn_main(void);",
		"gwen_write_int(1LL);",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithUserModuleImports(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.gw")
	modulePath := filepath.Join(dir, "helper.gw")
	if err := os.WriteFile(modulePath, []byte(`module helper

func scale(x: int) -> int
  return x * 3
endfunc

export func triple(x: int) -> int
  return scale(x)
endfunc
endmodule
`), 0o644); err != nil {
		t.Fatalf("write module failed: %v", err)
	}
	if err := os.WriteFile(mainPath, []byte(`use helper
use triple from helper

func main()
  write(helper.triple(7), triple(8))
endfunc
`), 0o644); err != nil {
		t.Fatalf("write main failed: %v", err)
	}

	out := mustEmitCPath(t, mainPath)
	wants := []string{
		"static long long gwen_mod_helper_scale(long long slot_",
		"static long long gwen_mod_helper_triple(long long slot_",
		"tmp_3 = gwen_mod_helper_scale(slot_",
		"gwen_mod_helper_triple(7LL);",
		"gwen_mod_helper_triple(8LL);",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithDynamicJSONAndListFlow(t *testing.T) {
	out := mustEmitC(t, `use json
use append, concat, sort, asc from list

func take(path: string) -> string
  return path
endfunc

func main()
  items := []
  append(items, json.objectof("path", "README.md"))
  other := []
  append(other, json.objectof("path", "docs/README.md"))
  items := concat(items, other)
  names := sort(["b", "a"], asc)
  write(take(items[0]["path"]), names[0], json.isnull(json.null()))
endfunc`)

	wants := []string{
		"static gwen_value gwen_value_index(gwen_value object_value, gwen_value index) {",
		"static void gwen_value_list_append(gwen_value *list_value, gwen_value item) {",
		"static gwen_list_1 gwen_list_1_sort_asc(gwen_list_1 items) {",
		"gwen_value_list_append(&(slot_1), tmp_",
		"= gwen_value_list_concat((slot_init_1 ? slot_1",
		"= gwen_list_1_sort_asc(tmp_",
		"= gwen_value_index((slot_init_1 ? slot_1",
		"gwen_fn_take(gwen_value_as_string(tmp_",
		"= gwen_value_is_null(tmp_",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithTypedConcat(t *testing.T) {
	out := mustEmitC(t, `use concat from list

func main()
  left: list[int] := [1, 2]
  right: list[int] := [3, 4]
  merged := concat(left, right)
  write(merged[2], len(merged))
endfunc`)

	wants := []string{
		"out of memory in concat()",
		"gwen_concat_left",
		"gwen_concat_right",
		"gwen_concat_left.len + gwen_concat_right.len",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithTypedInsertAndReversed(t *testing.T) {
	out := mustEmitC(t, `use insert, reversed from list

func main()
  items: list[int] := [1, 3]
  insert(items, 1, 2)
  flipped := reversed(items)
  write(flipped[0], len(flipped))
endfunc`)

	wants := []string{
		"insert() index out of range",
		"out of memory inserting into",
		"out of memory in reversed()",
		"gwen_reversed_source_",
		"gwen_insert_target_",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithHTTPServerBuiltins(t *testing.T) {
	out := mustEmitC(t, `use http
use json
use args from os

func handle(req: HttpRequest) -> result[HttpReply]
  if http.path(req) = "/health" then
    return ok(http.text(200, "ok"))
  endif

  matched, params := http.route(req, "/hello/:name")
  if matched then
    return http.json(200, json.objectof("name", params["name"], "path", http.path(req)))
  endif

  matched, served := http.static(req, "/", "examples/docs_site/public")
  if matched then
    match served
      when ok(reply) => return ok(reply)
      when err(e) => return ok(http.text(404, e))
    endmatch
  endif

  return ok(http.text(404, "missing"))
endfunc

func main()
  argv := args()
  match http.listen("127.0.0.1:0", handle)
    when ok(server) =>
      write(http.addr(server), len(argv))
      match http.wait(server)
        when ok(code) => write(code)
        when err(e) => write(e)
      endmatch
    when err(e) =>
      write(e)
  endmatch
endfunc`)

	wants := []string{
		"typedef struct {",
		"} gwen_http_reply;",
		"static gwen_result_http_reply gwen_http_json_reply(",
		"static bool gwen_http_route_pairs(",
		"static gwen_result_http_reply gwen_http_static_reply(",
		"static bool gwen_http_server_open(",
		"static bool gwen_http_server_wait_loop(",
		"static int gwen_cli_argc = 0;",
		"gwen_cli_argc = argc;",
		"int main(int argc, char **argv) {",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestEmitProgramWithHTTPClientBuiltins(t *testing.T) {
	out := mustEmitC(t, `use http

func main()
  match http.get("https://example.com")
    when ok(resp) =>
      write(http.status(resp), len(http.responsebody(resp)), http.responseheader(resp, "content-type", "missing"))
    when err(e) =>
      write(e)
  endmatch

  headers := dict[string, string]{"Content-Type": "application/json"}
  match http.request("POST", "https://httpbin.org/post", "{\"lang\":\"gwen\"}", headers)
    when ok(resp) =>
      write(http.status(resp), http.responseheader(resp, "Content-Type", "missing"))
    when err(e) =>
      write(e)
  endmatch
endfunc`)

	wants := []string{
		"static bool gwen_http_client_request(",
		"static gwen_string_pairs gwen_http_response_headers_from_dump(",
		"execvp(\"curl\", argv);",
		"long long gwen_http_timeout_ms = 5000LL;",
		"gwen_http_client_request(\"GET\", \"https://example.com\", \"\", gwen_string_pairs_empty(), gwen_http_timeout_ms, &gwen_http_response_value, &gwen_http_error)",
		"gwen_string_pairs gwen_http_headers = (gwen_string_pairs){",
		"gwen_http_client_request(\"POST\", \"https://httpbin.org/post\", \"{\\\"lang\\\":\\\"gwen\\\"}\", gwen_http_headers, gwen_http_timeout_ms, &gwen_http_response_value, &gwen_http_error)",
		"gwen_string_pairs_get((",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("emitted C missing %q\n%s", want, out)
		}
	}
}

func TestCompiledStateCellBuiltins(t *testing.T) {
	binPath := buildCompiledProgram(t, `use state

func inc(n: int) -> int
  return n + 1
endfunc

func main()
  counter: cell[int] := state.cell(1)
  write(state.get(counter))
  write(state.set(counter, 2))
  write(state.update(counter, inc))
  write(state.get(counter))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled state cell program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), strings.Join([]string{"1", "2", "3", "3"}, "\n"); got != want {
		t.Fatalf("compiled state cell output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledStateCellSnapshots(t *testing.T) {
	binPath := buildCompiledProgram(t, `use state

func main()
  items := [1]
  saved: cell[list[int]] := state.cell(items)
  append(items, 2)

  snapshot := state.get(saved)
  append(snapshot, 3)
  current := state.get(saved)
  write(len(current), current[0])

  next := [7]
  state.set(saved, next)
  append(next, 8)
  latest := state.get(saved)
  write(len(latest), latest[0])
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled state snapshot program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "1 1\n1 7"; got != want {
		t.Fatalf("compiled state snapshot output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledParallelStateCellUpdates(t *testing.T) {
	binPath := buildCompiledProgramFromPath(t, filepath.Join(repoRoot(t), "examples", "state_parallel.gw"))

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled parallel program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "2"; got != want {
		t.Fatalf("compiled parallel output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledDynamicListCoercionToTypedList(t *testing.T) {
	binPath := buildCompiledProgram(t, `func extend(items: list[int]) -> list[int]
  append(items, 3)
  return items
endfunc

func main()
  items := []
  append(items, 1)
  append(items, 2)
  done := extend(items)
  write(done)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled dynamic list coercion program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "[1,2,3]"; got != want {
		t.Fatalf("compiled dynamic list coercion output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledAsResultMatch(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  match 3 as float32
    when ok(v) =>
      write("ok", v)
    else
      write("bad")
  endmatch

  match "nope" as float32
    when ok(v) =>
      write("unexpected", v)
    else
      write("err")
  endmatch
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled as-result program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "ok 3\nerr"; got != want {
		t.Fatalf("compiled as-result output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledMoneyExample(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  price: money[USD] := 19.99
  tax: money[USD] := 1.5
  total := price + tax
  write(total)
  write(typeof(total))
  write(price * 2)
  write(price / 2)
  write(tax / price)

  match price as float64
    when ok(v) =>
      write(v)
    else
      write("bad")
  endmatch

  match price as money[EUR]
    when ok(v) =>
      write(v)
    when err(e) =>
      write(e)
  endmatch
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled money program failed: %v\n%s", err, string(output))
	}
	want := strings.Join([]string{
		"21.49 USD",
		"money[USD]",
		"39.98 USD",
		"9.995 USD",
		"0.0750375187593797",
		"19.99",
		"Cannot convert money[USD] to money[EUR] (explicit exchange rate required)",
	}, "\n")
	if got := strings.TrimSpace(string(output)); got != want {
		t.Fatalf("compiled money output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledStringNumericCasts(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  write(int("42"), float("3.5"))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled string numeric cast program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "42 3.5"; got != want {
		t.Fatalf("compiled string numeric cast output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledDefaultFunctionArgs(t *testing.T) {
	binPath := buildCompiledProgram(t, `func greet(name: string, greeting: string = "Hello") -> string
  return greeting + ", " + name
endfunc

func main()
  write(greet("Gwen"))
  write(greet("Gwen", "Hi"))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled default function args program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "Hello, Gwen\nHi, Gwen"; got != want {
		t.Fatalf("compiled default function args output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledDefaultMethodArgs(t *testing.T) {
	binPath := buildCompiledProgram(t, `object Counter
  n: int

  new(n: int = 0) -> Counter
    return Counter{n := n}
  endnew

  func add(self: Counter, delta: int = 1) -> int
    self.n := self.n + delta
    return self.n
  endfunc
endobject

func main()
  c := Counter.new()
  write(c.add(), c.add(4))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled default method args program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "1 5"; got != want {
		t.Fatalf("compiled default method args output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledDefaultImportedFunctionArgs(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.gw")
	modulePath := filepath.Join(dir, "helper.gw")
	if err := os.WriteFile(modulePath, []byte(`module helper

export func greet(name: string, greeting: string = "Hello") -> string
  return greeting + ", " + name
endfunc

endmodule
`), 0o644); err != nil {
		t.Fatalf("write module failed: %v", err)
	}
	if err := os.WriteFile(mainPath, []byte(`use greet from helper

func main()
  write(greet("Gwen"))
endfunc
`), 0o644); err != nil {
		t.Fatalf("write main failed: %v", err)
	}

	binPath := buildCompiledProgramFromPath(t, mainPath)
	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled imported default args program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "Hello, Gwen"; got != want {
		t.Fatalf("compiled imported default args output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledRemoveAtOnTypedList(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  items: list[int] := []
  append(items, 1)
  append(items, 2)
  append(items, 3)
  removeat(items, 1)
  write(items)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled removeat program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "[1,3]"; got != want {
		t.Fatalf("compiled removeat output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledPopOnTypedList(t *testing.T) {
	binPath := buildCompiledProgram(t, `use pop from list

func main()
  items: list[int] := [1, 2, 3]
  last := pop(items)
  write(last, items)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled pop program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "3 [1,2]"; got != want {
		t.Fatalf("compiled pop output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledRemoveAtReturnsRemovedItem(t *testing.T) {
	binPath := buildCompiledProgram(t, `use removeat from list

func main()
  items: list[int] := [10, 20, 30, 40]
  removed := removeat(items, 1)
  write(removed, items)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled removeat return program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "20 [10,30,40]"; got != want {
		t.Fatalf("compiled removeat return output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledAppendOnObjectListField(t *testing.T) {
	binPath := buildCompiledProgram(t, `object Box
  items: list[int]

  new() -> Box
    items: list[int] := []
    return Box{items := items}
  endnew

  func add(self: Box, n: int)
    append(self.items, n)
  endfunc

  func count(self: Box) -> int
    return len(self.items)
  endfunc
endobject

func main()
  box := Box.new()
  box.add(1)
  box.add(2)
  write(box.count())
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled object list append program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "2"; got != want {
		t.Fatalf("compiled object list append output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledListOfFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func inc(x: int) -> int
  return x + 1
endfunc

func double(x: int) -> int
  return x * 2
endfunc

func main()
  ops: list[(int) -> int] := []
  append(ops, inc)
  append(ops, double)

  total := 0
  for op in ops do
    total := total + op(3)
  endfor
  write(total)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled list of function values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "10"; got != want {
		t.Fatalf("compiled list of function values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledDictOfFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func inc(x: int) -> int
  return x + 1
endfunc

func double(x: int) -> int
  return x * 2
endfunc

func main()
  ops := dict[string, (int) -> int]{}
  ops["inc"] := inc
  ops["double"] := double
  write(ops["inc"](3) + ops["double"](3))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled dict of function values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "10"; got != want {
		t.Fatalf("compiled dict of function values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledCellOfFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `use state

func inc(x: int) -> int
  return x + 1
endfunc

func double(x: int) -> int
  return x * 2
endfunc

func main()
  op: cell[(int) -> int] := state.cell(inc)
  write(state.get(op)(3))
  write(state.set(op, double)(3))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled cell of function values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "4\n6"; got != want {
		t.Fatalf("compiled cell of function values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledListOfResultValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func safe_div(x: int, y: int) -> result[int]
  if y = 0 then
    return err("division by zero")
  endif
  return ok(x / y)
endfunc

func main()
  items: list[result[int]] := []
  append(items, safe_div(8, 2))
  append(items, safe_div(1, 0))

  match items[0]
    when ok(v) =>
      write(v)
    when err(e) =>
      write(e)
  endmatch

  match items[1]
    when ok(v) =>
      write(v)
    when err(e) =>
      write(e)
  endmatch
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled list of result values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "4\ndivision by zero"; got != want {
		t.Fatalf("compiled list of result values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledResultOfFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func inc(x: int) -> int
  return x + 1
endfunc

func choose(okay: bool) -> result[(int) -> int]
  if okay then
    return ok(inc)
  endif
  return err("missing")
endfunc

func main()
  match choose(true)
    when ok(op) =>
      write(op(3))
    when err(e) =>
      write(e)
  endmatch

  match choose(false)
    when ok(op) =>
      write(op(3))
    when err(e) =>
      write(e)
  endmatch
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled result of function values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "4\nmissing"; got != want {
		t.Fatalf("compiled result of function values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledTupleOfFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `type Unary = (int) -> int

func inc(x: int) -> int
  return x + 1
endfunc

func double(x: int) -> int
  return x * 2
endfunc

func pair(seed: int) -> Unary, Unary
  return inc, double
endfunc

func main()
  picker: (int) -> Unary, Unary := pair
  left, right := picker(0)
  a: int := left(3)
  b: int := right(3)
  write(a, b)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled tuple of function values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "4 6"; got != want {
		t.Fatalf("compiled tuple of function values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledAliasContainerBuiltins(t *testing.T) {
	binPath := buildCompiledProgram(t, `use state

type Scores = dict[string, int]
type Numbers = list[int]
type Counter = cell[int]

func main()
  scores: Scores := dict[string, int]{"alice": 1}
  nums: Numbers := [1, 2]
  counter: Counter := state.cell(3)

  append(nums, 3)
  scores["bob"] := 2

  names := keys(scores)
  vals := values(scores)
  current := state.get(counter)
  next := state.set(counter, 7)

  write(len(names), get(scores, "bob", 0), vals[1], nums[2], current, next)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled alias container builtins program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "2 2 2 3 3 7"; got != want {
		t.Fatalf("compiled alias container builtins output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledDirectMultiReturnCall(t *testing.T) {
	binPath := buildCompiledProgram(t, `func pair(x: int) -> int, int
  return x, x + 1
endfunc

func main()
  left, right := pair(4)
  write(left, right)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled direct multi-return call program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "4 5"; got != want {
		t.Fatalf("compiled direct multi-return call output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledNumericAliasOps(t *testing.T) {
	binPath := buildCompiledProgram(t, `type UserId = int

func main()
  a: UserId := 4
  b: UserId := 6
  total := a + b
  neg := -a
  dist := abs(-b)
  low := min(a, b)
  high := max(a, b)
  write(total, neg, dist, low, high)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled numeric alias ops program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "10 -4 6 4 6"; got != want {
		t.Fatalf("compiled numeric alias ops output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledParallelAllowFailContinues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func boom() -> int
  x: int
  return x
endfunc

func main()
  parallel allowfail do
    boom()
  endparallel
  write("done")
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled parallel allowfail program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "done"; got != want {
		t.Fatalf("compiled parallel allowfail output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledParallelResultsCollectValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func noop()
endfunc

func main()
  parallel => results do
    7
    noop()
  endparallel
  write(results)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled parallel results program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), `[ok(7),ok(null)]`; got != want {
		t.Fatalf("compiled parallel results output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledParallelAllowFailResultsCollectErrors(t *testing.T) {
	binPath := buildCompiledProgram(t, `func boom() -> int
  x: int
  return x
endfunc

func main()
  parallel allowfail => results do
    7
    boom()
  endparallel
  write(results)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled parallel allowfail results program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), `[ok(7),err("runtime error: 'x' read before assignment")]`; got != want {
		t.Fatalf("compiled parallel allowfail results output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledParallelRunsTasksConcurrently(t *testing.T) {
	binPath := buildCompiledProgram(t, `use sleep, nowunixms from time

func block(ms: int) -> int
  sleep(ms)
  return ms
endfunc

func main()
  start := nowunixms()
  parallel => results do
    block(200)
    block(200)
  endparallel
  stop := nowunixms()
  write(stop - start)
  write(results)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled parallel concurrency program failed: %v\n%s", err, string(output))
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 {
		t.Fatalf("compiled parallel concurrency output line mismatch\n%s", string(output))
	}
	elapsed, parseErr := time.ParseDuration(lines[0] + "ms")
	if parseErr != nil {
		t.Fatalf("compiled parallel elapsed parse failed: %v\n%s", parseErr, string(output))
	}
	if elapsed >= 350*time.Millisecond {
		t.Fatalf("compiled parallel did not run concurrently: elapsed=%s", elapsed)
	}
	if got, want := lines[1], `[ok(200),ok(200)]`; got != want {
		t.Fatalf("compiled parallel concurrency output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledParallelDoesNotLeakOuterAssignments(t *testing.T) {
	binPath := buildCompiledProgram(t, `x := 1
parallel do
  x := 2
endparallel
write(x)`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled parallel outer assignment program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "1"; got != want {
		t.Fatalf("compiled parallel outer assignment output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledVarBlockBasic(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  var
    a: int := 1
    b: int
  endvar
  b := a + 2
  write(a, b)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled var block program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "1 3"; got != want {
		t.Fatalf("compiled var block output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledVarBlockDefaultModes(t *testing.T) {
	binPath := buildCompiledProgram(t, `counter: int := 0

func nextvalue() -> int
  global counter := counter + 1
  return counter
endfunc

func main()
  var default
    zero_a: int
    zero_b: int
  endvar

  var default nextvalue()
    shared_a: int
    shared_b: int
  endvar

  write(zero_a, zero_b, shared_a, shared_b, counter)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled var block default modes program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "0 0 1 1 1"; got != want {
		t.Fatalf("compiled var block default modes output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledFunctionAssignmentShadowsOuterBinding(t *testing.T) {
	binPath := buildCompiledProgram(t, `call_count: int := 0

func increment() -> int
  call_count := call_count + 1
  return call_count
endfunc

func counter_maker() -> int
  global call_count := call_count + 1
  return call_count
endfunc

func main()
  first := increment()
  second := increment()
  before := call_count
  after := counter_maker()
  write(first, second, before, after, call_count)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled function assignment shadowing program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "1 1 0 1 1"; got != want {
		t.Fatalf("compiled function assignment shadowing output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledReturnCoercesToDeclaredTypes(t *testing.T) {
	binPath := buildCompiledProgram(t, `func pack() -> int, list[int]
  size := 2
  items := []
  append(items, 7)
  append(items, 9)
  return size, items
endfunc

func main()
  n, values := pack()
  write(n, values[0], values[1], len(values))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled return coercion program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "2 7 9 2"; got != want {
		t.Fatalf("compiled return coercion output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledNestedLoopReentersForEach(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  sequence := []
  idx := 0
  while idx < 3 do
    temp := []
    if idx = 0 then
      append(temp, 101)
    endif
    if idx = 1 then
      append(temp, 7)
    endif
    if idx = 2 then
      append(temp, 5)
    endif
    for item in sequence do
      append(temp, item)
    endfor
    sequence := temp
    idx := idx + 1
  endwhile
  write(sequence)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled nested for-each reentry program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "[5,7,101]"; got != want {
		t.Fatalf("compiled nested for-each reentry output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledNestedLoopReentersForRange(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  rounds := 0
  total := 0
  while rounds < 2 do
    for i in 1 to 3 do
      total := total + i
    endfor
    rounds := rounds + 1
  endwhile
  write(total)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled nested for-range reentry program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "12"; got != want {
		t.Fatalf("compiled nested for-range reentry output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledObjectMethods(t *testing.T) {
	binPath := buildCompiledProgram(t, `object Counter
  n: int

  new() -> Counter
    return Counter{n := 0}
  endnew

  func inc(self: Counter) -> int
    self.n := self.n + 1
    return self.n
  endfunc
endobject

func main()
  c := Counter.new()
  write(c.inc(), Counter.inc(c))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled object program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "1 2"; got != want {
		t.Fatalf("compiled object output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func double(x: int) -> int
  return x * 2
endfunc

func apply(f: (int) -> int, x: int) -> int
  return f(x)
endfunc

func main()
  op: (int) -> int := double
  write(op(5), apply(double, 6))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled function value program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "10 12"; got != want {
		t.Fatalf("compiled function value output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledCapturingNestedFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  factor := 2

  func scale(x: int) -> int
    return x * factor
  endfunc

  apply: (int) -> int := scale
  write(apply(4), scale(4))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled capturing nested function value program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "8 8"; got != want {
		t.Fatalf("compiled capturing nested function value output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledCapturingLambdaValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  factor := 2
  apply: (int) -> int := (x: int) => x * factor
  write(apply(4))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled capturing lambda program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "8"; got != want {
		t.Fatalf("compiled capturing lambda output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledTopLevelFunctionValueUsingGlobal(t *testing.T) {
	binPath := buildCompiledProgram(t, `count: int := 41

func read_count() -> int
  return count
endfunc

func apply(f: () -> int) -> int
  return f()
endfunc

func main()
  current: () -> int := read_count
  write(current(), apply(read_count))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled top-level global function value program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "41 41"; got != want {
		t.Fatalf("compiled top-level global function value output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledNestedFunctionValueUsingGlobalOnly(t *testing.T) {
	binPath := buildCompiledProgram(t, `count: int := 41

func apply(f: () -> int) -> int
  return f()
endfunc

func main()
  func read_count() -> int
    return count
  endfunc

  current: () -> int := read_count
  write(current(), apply(read_count))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled nested global-only function value program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "41 41"; got != want {
		t.Fatalf("compiled nested global-only function value output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledNonCapturingLambdaValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func reduce(f: (int, int) -> int, arr: list[int], init: int) -> int
  acc := init
  for item in arr do
    acc := f(acc, item)
  endfor
  return acc
endfunc

func main()
  nums := range(1, 5)
  doubled := map(nums, (x: int) => x * 2)
  kept := filter(nums, (x: int) => x mod 2 = 0)
  sum := reduce((a: int, b: int) => a + b, nums, 0)
  product := reduce((a: int, b: int) => a * b, nums, 1)
  write(sum, product, doubled[2], len(kept))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled lambda program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "15 120 6 2"; got != want {
		t.Fatalf("compiled lambda output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledMapAndFilter(t *testing.T) {
	binPath := buildCompiledProgram(t, `func double(x: int) -> int
  return x * 2
endfunc

func even(x: int) -> bool
  return x mod 2 = 0
endfunc

func main()
  items := [1, 2, 3, 4]
  op: (int) -> int := double
  pred: (int) -> bool := even
  doubled := map(items, op)
  kept := filter(items, pred)
  write(doubled[2], len(kept), kept[1])
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled map/filter program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "6 2 4"; got != want {
		t.Fatalf("compiled map/filter output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledMapAndFilterFromBareList(t *testing.T) {
	binPath := buildCompiledProgram(t, `func double(x: int) -> int
  return x * 2
endfunc

func at_least_three(x: int) -> bool
  return x >= 3
endfunc

func main()
  raw: list := [1, 2, 3, 4]
  numbers: list[int] := map(raw, double)
  kept: list[int] := filter(raw, at_least_three)
  write(len(numbers), numbers[1], len(kept), kept[0])
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled bare-list map/filter program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "4 4 2 3"; got != want {
		t.Fatalf("compiled bare-list map/filter output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledRangeAndEnumerate(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  up := range(2, 6, 2)
  down := range(3, 1)
  indexed: list[list] := enumerate(["a", "b", "c"])
  write(up[1], down[2], len(indexed), indexed[1][0], indexed[1][1])
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled range/enumerate program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "4 1 3 1 b"; got != want {
		t.Fatalf("compiled range/enumerate output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledItems(t *testing.T) {
	binPath := buildCompiledProgram(t, `use json

func main()
  typed := dict[string, int]{"alice": 4, "bob": 5}
  typed_pairs: list[list] := items(typed)

  dyn_pairs: list[list] := items(json.objectof("left", 7, "right", 8))

  typed_total := 0
  for pair in typed_pairs do
    typed_total := typed_total + int(pair[1])
  endfor

  dyn_total := 0
  for pair in dyn_pairs do
    dyn_total := dyn_total + int(pair[1])
  endfor

  write(len(typed_pairs), typed_total, len(dyn_pairs), dyn_total)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled items program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "2 9 2 15"; got != want {
		t.Fatalf("compiled items output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledBuiltinFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `use trim from string

func main()
  nums := [1, 2, 3]
  raw := [" 4 ", "5 "]
  texts: list[string] := map(nums, str)
  trimmed: list[string] := map(raw, trim)
  ints: list[int] := map(trimmed, int)

  show := str
  measure := len
  kindof := typeof

  write(texts[2], ints[0] + ints[1], show(true), measure(texts), kindof(texts[0]))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled builtin function values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "3 9 true 3 string"; got != want {
		t.Fatalf("compiled builtin function values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledDynamicBuiltinFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `use join from string
use abs from math

func main()
  joiner := join
  mag := abs
  write(joiner(["a", "b", "c"], "-"), mag(-7), mag(-1.5))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled dynamic builtin function values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "a-b-c 7 1.5"; got != want {
		t.Fatalf("compiled dynamic builtin function values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledImportedBuiltinFunctionValues(t *testing.T) {
	readPath := filepath.ToSlash(filepath.Join(repoRoot(t), "README.md"))
	binPath := buildCompiledProgram(t, `use readfile from io
use basename, joinpath from path
use join from string
use os
use time

func main()
  loader := readfile
  stem := basename
  makepath := joinpath
  joiner := join
  cwdf := os.cwd
  clock := time.nowunix
  stamp := time.nowrfc3339

  match loader("`+readPath+`")
    when ok(text) =>
      write(stem("docs/README.md"), makepath("docs", "stdlib.md"), joiner(["a", "b"], "-"), len(cwdf()) > 0, clock() > 0, contains(stamp(), "T"), contains(text, "Gwen"))
    when err(e) =>
      write("ERR", e)
  endmatch
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled imported builtin function values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "README.md docs/stdlib.md a-b true true true true"; got != want {
		t.Fatalf("compiled imported builtin function values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledJSONAndOSBuiltinFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `use json
use os

func main()
  encode := json.stringify
  parseobj := json.parseobject
  isnull := json.isnull
  argv := os.args

  args := argv()
  payload := json.objectof("args", args, "missing", json.null())

  match encode(payload)
    when ok(text) =>
      match parseobj(text)
        when ok(decoded) =>
          write(len(args), len(decoded["args"]), isnull(decoded["missing"]))
        when err(e) =>
          write("ERR", e)
      endmatch
    when err(e) =>
      write("ERR", e)
  endmatch
endfunc`)

	cmd := exec.Command(binPath, "one", "two")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled json/os builtin function values program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "2 2 true"; got != want {
		t.Fatalf("compiled json/os builtin function values output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledReadFunctionValue(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  ask := read
  write(ask("name? "))
endfunc`)

	cmd := exec.Command(binPath)
	cmd.Stdin = strings.NewReader("gwen\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled read function value program failed: %v\n%s", err, string(output))
	}
	if got, want := string(output), "name? gwen\n"; got != want {
		t.Fatalf("compiled read function value output mismatch\n got:\n%q\nwant:\n%q", got, want)
	}
}

func TestCompiledRead(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  name := read("name? ")
  lang := read()
  write(name, lang)
endfunc`)

	cmd := exec.Command(binPath)
	cmd.Stdin = strings.NewReader("gwen\nlang\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled read program failed: %v\n%s", err, string(output))
	}
	if got, want := string(output), "name? gwen lang\n"; got != want {
		t.Fatalf("compiled read output mismatch\n got:\n%q\nwant:\n%q", got, want)
	}
}

func TestCompiledTypedConcat(t *testing.T) {
	binPath := buildCompiledProgram(t, `use concat from list

func main()
  left: list[int] := [1, 2]
  right: list[int] := [3, 4]
  merged := concat(left, right)
  write(merged[2], len(merged))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled typed concat program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "3 4"; got != want {
		t.Fatalf("compiled typed concat output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledTypedInsertAndReversed(t *testing.T) {
	binPath := buildCompiledProgram(t, `use insert, reversed from list

func main()
  items: list[int] := [1, 3]
  insert(items, 1, 2)
  flipped := reversed(items)
  write(items[1], flipped[0], len(flipped))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled typed insert/reversed program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "2 3 3"; got != want {
		t.Fatalf("compiled typed insert/reversed output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledMultiReturnFunctionValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `func pair(x: int) -> int, int
  return x, x + 1
endfunc

func main()
  f: (int) -> int, int := pair
  left, right := f(4)
  write(left, right)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled multi-return function value program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "4 5"; got != want {
		t.Fatalf("compiled multi-return function value output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledObjectStaticMethodValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `object Counter
  n: int

  new(n: int) -> Counter
    return Counter{n := n}
  endnew

  func value(self: Counter) -> int
    return self.n
  endfunc
endobject

func apply(f: (Counter) -> int, c: Counter) -> int
  return f(c)
endfunc

func main()
  op: (Counter) -> int := Counter.value
  c := Counter.new(7)
  write(op(c), apply(Counter.value, c))
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled object static method value program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "7 7"; got != want {
		t.Fatalf("compiled object static method value output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledBoundInstanceMethodValues(t *testing.T) {
	binPath := buildCompiledProgram(t, `object Counter
  n: int

  new() -> Counter
    return Counter{n := 0}
  endnew

  func add(self: Counter, delta: int) -> int
    self.n := self.n + delta
    return self.n
  endfunc

  func value(self: Counter) -> int
    return self.n
  endfunc
endobject

func apply(f: (int) -> int, x: int) -> int
  return f(x)
endfunc

func main()
  c := Counter.new()
  op: (int) -> int := c.add
  write(op(2), op(5), apply(c.add, 4), c.value())
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled bound instance method value program failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "2 7 4 0"; got != want {
		t.Fatalf("compiled bound instance method value output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledModuleExportedObject(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.gw")
	modulePath := filepath.Join(dir, "bank.gw")
	if err := os.WriteFile(modulePath, []byte(`module bank

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
`), 0o644); err != nil {
		t.Fatalf("write module failed: %v", err)
	}
	if err := os.WriteFile(mainPath, []byte(`use Account from bank

func main()
  acc := Account.new(7)
  write(acc.value())
endfunc
`), 0o644); err != nil {
		t.Fatalf("write main failed: %v", err)
	}

	binPath := buildCompiledProgramFromPath(t, mainPath)
	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled module object failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "7"; got != want {
		t.Fatalf("compiled module object output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledUninitializedReadErrors(t *testing.T) {
	binPath := buildCompiledProgram(t, `func main()
  x: int
  write(x)
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err == nil {
		t.Fatalf("expected compiled uninitialized read to fail\n%s", string(output))
	}
	if !strings.Contains(string(output), "read before assignment") {
		t.Fatalf("compiled uninitialized read error mismatch\n%s", string(output))
	}
}

func TestCompiledBasicExamplesSmoke(t *testing.T) {
	root := repoRoot(t)
	tests := []struct {
		name     string
		path     string
		args     []string
		env      []string
		setup    func(*testing.T)
		contains []string
	}{
		{
			name:     "hello",
			path:     filepath.Join(root, "examples", "hello.gw"),
			contains: []string{"Hello, Gwen!"},
		},
		{
			name:     "dict_basics",
			path:     filepath.Join(root, "examples", "dict_basics.gw"),
			contains: []string{"alice = 95", "total = 257", "user 1 = alice"},
		},
		{
			name:     "match_strict",
			path:     filepath.Join(root, "examples", "match_strict.gw"),
			contains: []string{"=== Test: ok(x) pattern ===", "=== Test: range pattern (not Result type, allowed) ===", "Match strict semantics: ALL TESTS PASSED"},
		},
		{
			name:     "global_scope",
			path:     filepath.Join(root, "examples", "global_scope.gw"),
			contains: []string{"After increment(): 0", "After 2nd counter_maker(): 2", "SUCCESS: global works correctly!"},
		},
		{
			name:     "var_block",
			path:     filepath.Join(root, "examples", "var_block.gw"),
			contains: []string{"Alice 30 true", "0 0 0", "1 99 1", "42"},
		},
		{
			name:     "money",
			path:     filepath.Join(root, "examples", "money.gw"),
			contains: []string{"21.49 USD", "money[USD]", "correctly rejected:"},
		},
		{
			name:     "lis",
			path:     filepath.Join(root, "examples", "lis.gw"),
			contains: []string{"LIS length: 4", "LIS length: 0", "LIS length: 5", "LIS: ALL TESTS PASSED"},
		},
		{
			name:     "quicksort",
			path:     filepath.Join(root, "examples", "quicksort.gw"),
			contains: []string{"Original: [64,34,25,12,22,11,90]", "Sorted: [11,12,22,25,34,64,90]", "All tests passed!"},
		},
		{
			name:     "runtime_basics",
			path:     filepath.Join(root, "examples", "runtime_basics.gw"),
			args:     []string{"a", "b"},
			env:      []string{"PORT=9876"},
			contains: []string{"cwd: ", `argv: ["a","b"]`, "now: ", "nowms: ", "PORT = 9876"},
		},
		{
			name:     "binary_search",
			path:     filepath.Join(root, "examples", "binary_search.gw"),
			contains: []string{"Binary search: ALL TESTS PASSED"},
		},
		{
			name:     "binary_search_float",
			path:     filepath.Join(root, "examples", "binary_search_float.gw"),
			contains: []string{"Float binary search: ALL TESTS COMPLETED"},
		},
		{
			name:     "explicit_types",
			path:     filepath.Join(root, "examples", "explicit_types.gw"),
			contains: []string{"已实现精度类型：", "IEEE 754"},
		},
		{
			name:     "fibonacci",
			path:     filepath.Join(root, "examples", "fibonacci.gw"),
			contains: []string{"Fibonacci: ALL TESTS PASSED"},
		},
		{
			name: "file_io",
			path: filepath.Join(root, "examples", "file_io.gw"),
			setup: func(t *testing.T) {
				t.Helper()
				_ = os.Remove("/tmp/gwen_demo.txt")
				t.Cleanup(func() { _ = os.Remove("/tmp/gwen_demo.txt") })
			},
			contains: []string{
				"wrote 12 bytes",
				"line count: 3",
				"appended 10 bytes",
				"expected err:",
			},
		},
		{
			name:     "gcd",
			path:     filepath.Join(root, "examples", "gcd.gw"),
			contains: []string{"GCD of 48 and 18 is 6"},
		},
		{
			name:     "json_basics",
			path:     filepath.Join(root, "examples", "json_basics.gw"),
			contains: []string{`"roles":["admin","ops"]`, `"deleted_at":null`},
		},
		{
			name:     "merge_sort",
			path:     filepath.Join(root, "examples", "merge_sort.gw"),
			contains: []string{"Merge sort: ALL TESTS COMPLETED"},
		},
		{
			name:     "nested_scope",
			path:     filepath.Join(root, "examples", "nested_scope.gw"),
			contains: []string{"Nested scope with global works!"},
		},
		{
			name:     "precision_test",
			path:     filepath.Join(root, "examples", "precision_test.gw"),
			contains: []string{"Precision types: ALL TESTS PASSED"},
		},
		{
			name:     "arena",
			path:     filepath.Join(root, "examples", "arena.gw"),
			contains: []string{"All arena tests passed!"},
		},
		{
			name:     "arena_memory",
			path:     filepath.Join(root, "examples", "arena_memory.gw"),
			contains: []string{"区域内存管理尚未实现", "当前 Gwen 使用 Python 的 GC"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			binPath := buildCompiledProgramFromPath(t, tc.path)
			cmd := exec.Command(binPath, tc.args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), tc.env...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("compiled example %s failed: %v\n%s", tc.name, err, string(output))
			}
			text := string(output)
			for _, want := range tc.contains {
				if !strings.Contains(text, want) {
					t.Fatalf("compiled example %s output missing %q\n%s", tc.name, want, text)
				}
			}
		})
	}
}

func TestCompiledHTTPServerExample(t *testing.T) {
	type helloResponse struct {
		Name   string `json:"name"`
		Lang   string `json:"lang"`
		Method string `json:"method"`
	}

	root := repoRoot(t)
	sourcePath := filepath.Join(root, "examples", "http_server.gw")
	binPath := buildCompiledProgramFromPath(t, sourcePath)
	addr := reserveLocalAddr(t)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binPath, addr)
	cmd.Dir = root
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start compiled http_server failed: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()
	defer cleanupCommand(t, cancel, waitCh, &logs)

	baseURL := "http://" + addr
	waitForHTTPReady(t, baseURL+"/", waitCh, &logs)

	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	status, body, _ := doHTTP(t, client, http.MethodGet, baseURL+"/", "", "")
	if status != http.StatusOK || !strings.Contains(body, "Gwen HTTP") || !strings.Contains(body, "session: guest") {
		t.Fatalf("root response mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}

	status, _, headers := doHTTP(t, client, http.MethodGet, baseURL+"/login", "", "")
	if status != http.StatusSeeOther {
		t.Fatalf("login status mismatch: %d\n%s", status, logs.String())
	}
	if location := headers.Get("Location"); location != "/" {
		t.Fatalf("login location mismatch: %q\n%s", location, logs.String())
	}
	cookies := headers.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatalf("login response missing Set-Cookie\n%s", logs.String())
	}
	loginCookie := ""
	for _, raw := range cookies {
		parts := strings.SplitN(raw, ";", 2)
		if strings.HasPrefix(parts[0], "session=") {
			loginCookie = parts[0]
			break
		}
	}
	if loginCookie != "session=demo" {
		t.Fatalf("login cookie mismatch: %q\n%s", loginCookie, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/", "", loginCookie)
	if status != http.StatusOK || !strings.Contains(body, "session: demo") {
		t.Fatalf("cookie root response mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/api/hello/Ada?lang=zh", "", "")
	if status != http.StatusOK {
		t.Fatalf("api hello status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	hello := decodeJSONBody[helloResponse](t, body)
	if hello.Name != "Ada" || hello.Lang != "zh" || hello.Method != "GET" {
		t.Fatalf("api hello body mismatch: %+v\n%s", hello, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/assets/app.css", "", "")
	if status != http.StatusOK || !strings.Contains(body, "font-family: serif") {
		t.Fatalf("static asset mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/missing", "", "")
	if status != http.StatusNotFound || body != "not found" {
		t.Fatalf("missing route mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}
}

func TestCompiledDocsSiteExample(t *testing.T) {
	type siteResponse struct {
		Lang     string           `json:"lang"`
		Brand    string           `json:"brand"`
		Pages    []map[string]any `json:"pages"`
		Examples []map[string]any `json:"examples"`
	}
	type pageResponse struct {
		Lang    string `json:"lang"`
		Kind    string `json:"kind"`
		Slug    string `json:"slug"`
		Format  string `json:"format"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	type exampleResponse struct {
		Lang   string `json:"lang"`
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Format string `json:"format"`
		Path   string `json:"path"`
		Source string `json:"source"`
	}
	type searchResponse struct {
		Lang  string           `json:"lang"`
		Items []map[string]any `json:"items"`
	}

	root := repoRoot(t)
	sourcePath := filepath.Join(root, "examples", "docs_site", "main.gw")
	binPath := buildCompiledProgramFromPath(t, sourcePath)
	addr := reserveLocalAddr(t)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binPath, addr)
	cmd.Dir = root
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start compiled docs_site failed: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()
	defer cleanupCommand(t, cancel, waitCh, &logs)

	baseURL := "http://" + addr
	waitForHTTPReady(t, baseURL+"/api/health", waitCh, &logs)

	client := &http.Client{Timeout: 2 * time.Second}

	status, body, _ := doHTTP(t, client, http.MethodGet, baseURL+"/api/health", "", "")
	if status != http.StatusOK || body != "ok" {
		t.Fatalf("health mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/", "", "")
	if status != http.StatusOK || !strings.Contains(body, "Gwen Field Guide") || !strings.Contains(body, "/brand/gwen-logo.svg") {
		t.Fatalf("index mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/api/site/zh", "", "")
	if status != http.StatusOK {
		t.Fatalf("site api status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	site := decodeJSONBody[siteResponse](t, body)
	if site.Lang != "zh" || site.Brand != "Gwen 学习指南" || len(site.Pages) == 0 || len(site.Examples) == 0 {
		t.Fatalf("site api body mismatch: %+v\n%s", site, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/api/page/en/README", "", "")
	if status != http.StatusOK {
		t.Fatalf("page api status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	page := decodeJSONBody[pageResponse](t, body)
	if page.Lang != "en" || page.Kind != "page" || page.Slug != "README" || page.Format != "markdown" || page.Path != "README.md" || !strings.Contains(page.Content, "# Gwen") {
		t.Fatalf("page api body mismatch: %+v\n%s", page, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/api/page/zh/README", "", "")
	if status != http.StatusOK {
		t.Fatalf("zh page api status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	page = decodeJSONBody[pageResponse](t, body)
	if page.Lang != "zh" || page.Kind != "page" || page.Slug != "README" || page.Format != "markdown" || page.Path != "README.zh.md" || !strings.Contains(page.Content, "Gwen 是一门面向后端与自动化场景的语言") {
		t.Fatalf("zh page api body mismatch: %+v\n%s", page, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/api/example/en/examples--hello", "", "")
	if status != http.StatusOK {
		t.Fatalf("example api status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	example := decodeJSONBody[exampleResponse](t, body)
	if example.Lang != "en" || example.Kind != "example" || example.Name != "examples--hello" || example.Format != "gwen" || example.Path != "examples/hello.gw" || !strings.Contains(example.Source, `write("Hello, Gwen!")`) {
		t.Fatalf("example api body mismatch: %+v\n%s", example, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/api/search/en", "", "")
	if status != http.StatusOK {
		t.Fatalf("search api status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	search := decodeJSONBody[searchResponse](t, body)
	if search.Lang != "en" || len(search.Items) == 0 {
		t.Fatalf("search api body mismatch: %+v\n%s", search, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/repo/docs/README.md", "", "")
	if status != http.StatusOK || !strings.Contains(body, "# Gwen 文档入口") || !strings.Contains(body, "tracking.md") {
		t.Fatalf("repo docs static mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/brand/gwen-logo.svg", "", "")
	if status != http.StatusOK || !strings.Contains(body, "<svg") {
		t.Fatalf("brand static mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}
}

func TestCompiledHTTPClientRuntime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hello":
			if r.Method != http.MethodGet {
				http.Error(w, "wrong method", http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("X-Source", "gwen-test")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "pong")
		case "/echo":
			if r.Method != http.MethodPost {
				http.Error(w, "wrong method", http.StatusMethodNotAllowed)
				return
			}
			if r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "missing content-type", http.StatusBadRequest)
				return
			}
			if r.Header.Get("X-Test") != "yes" {
				http.Error(w, "missing x-test", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	binPath := buildCompiledProgram(t, `use http

func main()
  match http.get("`+server.URL+`/hello")
    when ok(resp) =>
      write("GET", http.status(resp), http.responseheader(resp, "X-Source", "missing"), http.responsebody(resp))
    when err(e) =>
      write("GETERR", e)
  endmatch

  headers := dict[string, string]{"Content-Type": "application/json", "X-Test": "yes"}
  match http.request("POST", "`+server.URL+`/echo", "{\"lang\":\"gwen\"}", headers)
    when ok(resp) =>
      write("POST", http.status(resp), http.responseheader(resp, "Content-Type", "missing"), http.responsebody(resp))
    when err(e) =>
      write("POSTERR", e)
  endmatch
endfunc`)

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled http client runtime failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "GET 200 gwen-test pong\nPOST 201 application/json {\"lang\":\"gwen\"}"; got != want {
		t.Fatalf("compiled http client runtime output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledHTTPClientExamples(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hello":
			if r.Method != http.MethodGet {
				http.Error(w, "wrong method", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "pong")
		case "/echo":
			if r.Method != http.MethodPost {
				http.Error(w, "wrong method", http.StatusMethodNotAllowed)
				return
			}
			if r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "missing content-type", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if string(body) != "{\"lang\":\"gwen\"}" {
				http.Error(w, "unexpected body", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, "{\"ok\":true}")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := repoRoot(t)
	tests := []struct {
		name string
		path string
		arg  string
		want string
	}{
		{
			name: "http_get",
			path: filepath.Join(root, "examples", "http_get.gw"),
			arg:  server.URL + "/hello",
			want: "status: 200\nbytes: 4",
		},
		{
			name: "http_request",
			path: filepath.Join(root, "examples", "http_request.gw"),
			arg:  server.URL + "/echo",
			want: "status: 201\ncontent-type: application/json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binPath := buildCompiledProgramFromPath(t, tc.path)
			cmd := exec.Command(binPath, tc.arg)
			cmd.Dir = root
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("compiled example %s failed: %v\n%s", tc.name, err, string(output))
			}
			if got := strings.TrimSpace(string(output)); got != tc.want {
				t.Fatalf("compiled example %s output mismatch\n got:\n%s\nwant:\n%s", tc.name, got, tc.want)
			}
		})
	}
}

func TestCompiledExamplesMatchInterpreter(t *testing.T) {
	root := repoRoot(t)
	tests := []struct {
		name  string
		path  string
		args  []string
		setup func(*testing.T)
	}{
		{
			name: "global_scope",
			path: filepath.Join(root, "examples", "global_scope.gw"),
		},
		{
			name: "var_block",
			path: filepath.Join(root, "examples", "var_block.gw"),
		},
		{
			name: "money",
			path: filepath.Join(root, "examples", "money.gw"),
		},
		{
			name: "lis",
			path: filepath.Join(root, "examples", "lis.gw"),
		},
		{
			name: "quicksort",
			path: filepath.Join(root, "examples", "quicksort.gw"),
		},
		{
			name: "json_basics",
			path: filepath.Join(root, "examples", "json_basics.gw"),
		},
		{
			name: "nested_scope",
			path: filepath.Join(root, "examples", "nested_scope.gw"),
		},
		{
			name: "file_io",
			path: filepath.Join(root, "examples", "file_io.gw"),
			setup: func(t *testing.T) {
				t.Helper()
				_ = os.Remove("/tmp/gwen_demo.txt")
				t.Cleanup(func() { _ = os.Remove("/tmp/gwen_demo.txt") })
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			want := runInterpretedProgramFromPath(t, tc.path, tc.args...)
			binPath := buildCompiledProgramFromPath(t, tc.path)
			cmd := exec.Command(binPath, tc.args...)
			cmd.Dir = root
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("compiled example %s failed: %v\n%s", tc.name, err, string(output))
			}
			got := strings.TrimSpace(string(output))
			if gotNorm, wantNorm := normalizeComparableOutput(got), normalizeComparableOutput(want); gotNorm != wantNorm {
				t.Fatalf("compiled example %s output mismatch\ncompiled:\n%s\ninterpreter:\n%s\nnormalized compiled:\n%s\nnormalized interpreter:\n%s", tc.name, got, want, gotNorm, wantNorm)
			}
		})
	}
}

func TestCompiledHTTPExamplesMatchInterpreter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hello":
			if r.Method != http.MethodGet {
				http.Error(w, "wrong method", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "pong")
		case "/echo":
			if r.Method != http.MethodPost {
				http.Error(w, "wrong method", http.StatusMethodNotAllowed)
				return
			}
			if r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "missing content-type", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if string(body) != "{\"lang\":\"gwen\"}" {
				http.Error(w, "unexpected body", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, "{\"ok\":true}")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := repoRoot(t)
	tests := []struct {
		name string
		path string
		args []string
	}{
		{
			name: "http_get",
			path: filepath.Join(root, "examples", "http_get.gw"),
			args: []string{server.URL + "/hello"},
		},
		{
			name: "http_request",
			path: filepath.Join(root, "examples", "http_request.gw"),
			args: []string{server.URL + "/echo"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := runInterpretedProgramFromPath(t, tc.path, tc.args...)
			binPath := buildCompiledProgramFromPath(t, tc.path)
			cmd := exec.Command(binPath, tc.args...)
			cmd.Dir = root
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("compiled example %s failed: %v\n%s", tc.name, err, string(output))
			}
			got := strings.TrimSpace(string(output))
			if gotNorm, wantNorm := normalizeComparableOutput(got), normalizeComparableOutput(want); gotNorm != wantNorm {
				t.Fatalf("compiled example %s output mismatch\ncompiled:\n%s\ninterpreter:\n%s\nnormalized compiled:\n%s\nnormalized interpreter:\n%s", tc.name, got, want, gotNorm, wantNorm)
			}
		})
	}
}

func TestCompiledSQLiteBasicsExample(t *testing.T) {
	requireExternalTools(t, "cc", "sqlite3")

	root := repoRoot(t)
	sourcePath := filepath.Join(root, "examples", "sqlite_basics.gw")
	binPath := buildCompiledProgramFromPath(t, sourcePath)
	dbPath := "/tmp/gwen_sqlite_basics.db"
	_ = os.Remove(dbPath)
	t.Cleanup(func() { _ = os.Remove(dbPath) })

	cmd := exec.Command(binPath)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled sqlite basics failed: %v\n%s", err, string(output))
	}

	got := strings.TrimSpace(string(output))
	want := strings.Join([]string{
		"schema rows: 0",
		"inserted rows: 1",
		"latest body: ship it",
		"latest visits: 1",
		"deleted_at is null: true",
		"close: 0",
	}, "\n")
	if got != want {
		t.Fatalf("compiled sqlite basics output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledRulesAppExample(t *testing.T) {
	binPath := buildCompiledProgramFromPath(t, filepath.Join(repoRoot(t), "examples", "rules_app", "main.gw"))

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled rules app failed: %v\n%s", err, string(output))
	}
	text := string(output)
	wants := []string{
		"== Gwen Rules App ==",
		"Decision: approve",
		"Decision: review",
		"Decision: reject",
		"review: amount over manual-review threshold",
		"reject: country blocked: KP",
	}
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("compiled rules app output missing %q\n%s", want, text)
		}
	}
}

func TestCompiledLedgerAppExample(t *testing.T) {
	binPath := buildCompiledProgramFromPath(t, filepath.Join(repoRoot(t), "examples", "ledger_app", "main.gw"))
	outputPath := "/tmp/gwen_ledger_app_demo.txt"
	_ = os.Remove(outputPath)
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled ledger app failed: %v\n%s", err, string(output))
	}
	text := string(output)
	wants := []string{
		"== Gwen Ledger App ==",
		"Entry count: 7",
		"Balance: 4893.51",
		"Deleted entries: 1",
		"Updated notes: 1",
		"Reloaded entries: 6",
		"renewed domain and DNS",
	}
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("compiled ledger app output missing %q\n%s", want, text)
		}
	}
}

func TestCompiledSessionNotesExample(t *testing.T) {
	requireExternalTools(t, "cc", "sqlite3")

	type meResponse struct {
		Session    string `json:"session"`
		LoginCount int    `json:"login_count"`
		NoteCount  int    `json:"note_count"`
	}
	type postNoteResponse struct {
		Session string `json:"session"`
		Count   int    `json:"count"`
		Last    string `json:"last"`
	}
	type listNotesResponse struct {
		Session string   `json:"session"`
		Count   int      `json:"count"`
		Notes   []string `json:"notes"`
	}

	root := repoRoot(t)
	sourcePath := filepath.Join(root, "examples", "session_notes.gw")
	binPath := buildCompiledProgramFromPath(t, sourcePath)
	addr := reserveLocalAddr(t)
	dbPath := filepath.Join(t.TempDir(), "session_notes.db")

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binPath, addr, dbPath)
	cmd.Dir = root
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start compiled session_notes failed: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()
	defer cleanupCommand(t, cancel, waitCh, &logs)

	baseURL := "http://" + addr
	waitForHTTPReady(t, baseURL+"/api/health", waitCh, &logs)

	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	status, body, _ := doHTTP(t, client, http.MethodGet, baseURL+"/api/health", "", "")
	if status != http.StatusOK || body != "ok" {
		t.Fatalf("health mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/", "", "")
	if status != http.StatusOK || !strings.Contains(body, "Session Notes") {
		t.Fatalf("static index mismatch: status=%d body=%q\n%s", status, body, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/api/me", "", "")
	if status != http.StatusOK {
		t.Fatalf("guest api/me status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	guest := decodeJSONBody[meResponse](t, body)
	if guest.Session != "guest" || guest.LoginCount != 0 || guest.NoteCount != 0 {
		t.Fatalf("guest api/me mismatch: %+v\n%s", guest, logs.String())
	}

	status, _, headers := doHTTP(t, client, http.MethodGet, baseURL+"/login/alice", "", "")
	if status != http.StatusSeeOther {
		t.Fatalf("login status mismatch: %d\n%s", status, logs.String())
	}
	cookies := headers.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatalf("login response missing Set-Cookie\n%s", logs.String())
	}
	loginCookie := ""
	for _, raw := range cookies {
		parts := strings.SplitN(raw, ";", 2)
		if strings.HasPrefix(parts[0], "session=") {
			loginCookie = parts[0]
			break
		}
	}
	if loginCookie != "session=alice" {
		t.Fatalf("login cookie mismatch: %q\n%s", loginCookie, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/api/me", "", loginCookie)
	if status != http.StatusOK {
		t.Fatalf("logged-in api/me status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	me := decodeJSONBody[meResponse](t, body)
	if me.Session != "alice" || me.LoginCount != 1 || me.NoteCount != 0 {
		t.Fatalf("logged-in api/me mismatch: %+v\n%s", me, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodPost, baseURL+"/api/notes", `{"text":"ship it"}`, loginCookie)
	if status != http.StatusCreated {
		t.Fatalf("post note status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	posted := decodeJSONBody[postNoteResponse](t, body)
	if posted.Session != "alice" || posted.Count != 1 || posted.Last != "ship it" {
		t.Fatalf("post note body mismatch: %+v\n%s", posted, logs.String())
	}

	status, body, _ = doHTTP(t, client, http.MethodGet, baseURL+"/api/notes", "", loginCookie)
	if status != http.StatusOK {
		t.Fatalf("list notes status mismatch: %d body=%q\n%s", status, body, logs.String())
	}
	notes := decodeJSONBody[listNotesResponse](t, body)
	if notes.Session != "alice" || notes.Count != 1 || len(notes.Notes) != 1 || notes.Notes[0] != "ship it" {
		t.Fatalf("list notes body mismatch: %+v\n%s", notes, logs.String())
	}
}

func TestCompiledHigherOrderExample(t *testing.T) {
	binPath := buildCompiledProgramFromPath(t, filepath.Join(repoRoot(t), "examples", "higher_order.gw"))

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled higher_order example failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "Original: [1,2,3,4,5]\nDoubled: [2,4,6,8,10]\nEvens: [2,4]\nIndexed: [[0,\"a\"],[1,\"b\"],[2,\"c\"]]\nSum: 15\nProduct: 120"; got != want {
		t.Fatalf("compiled higher_order output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCompiledSegmentTreeExample(t *testing.T) {
	binPath := buildCompiledProgramFromPath(t, filepath.Join(repoRoot(t), "examples", "segment_tree.gw"))

	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled segment_tree example failed: %v\n%s", err, string(output))
	}
	want := strings.Join([]string{
		"Original array: [1,3,5,7,9,11]",
		"Segment tree built",
		"",
		"Query tests:",
		"Sum of range [0, 2]: 9",
		"Sum of range [1, 4]: 24",
		"Sum of range [0, 5]: 36",
		"",
		"Update test:",
		"Update index 2 from 5 to 10",
		"New sum of range [0, 5]: 41",
		"",
		"All tests completed!",
	}, "\n")
	if got := strings.TrimSpace(string(output)); got != want {
		t.Fatalf("compiled segment_tree output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}
