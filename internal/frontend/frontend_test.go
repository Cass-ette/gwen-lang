package frontend_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Cass-ette/gwen-lang/internal/frontend"
)

func TestAnalyzeSource(t *testing.T) {
	unit, err := frontend.AnalyzeSource(`func main()
  write("hi")
endfunc`, "hello.gw")
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if unit.Path != "hello.gw" {
		t.Fatalf("path mismatch: got %q want %q", unit.Path, "hello.gw")
	}
	if unit.Source == "" {
		t.Fatal("expected source to be preserved")
	}
	if unit.Program == nil {
		t.Fatal("expected parsed program")
	}
	if unit.HIR == nil {
		t.Fatal("expected lowered HIR")
	}
	if unit.MIR == nil {
		t.Fatal("expected lowered MIR")
	}
	if len(unit.HIR.Decls()) != 1 {
		t.Fatalf("HIR decl count mismatch: got %d want 1", len(unit.HIR.Decls()))
	}
}

func TestAnalyzePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gw")
	source := `func main()
  write("ok")
endfunc
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	unit, err := frontend.AnalyzePath(path)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if unit.Path != path {
		t.Fatalf("path mismatch: got %q want %q", unit.Path, path)
	}
}

func TestAnalyzePathExpandsUserModulesForHIRAndMIR(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.gw")
	modulePath := filepath.Join(dir, "helper.gw")
	if err := os.WriteFile(modulePath, []byte(`module helper

export func triple(x: int) -> int
  return x * 3
endfunc
endmodule
`), 0o644); err != nil {
		t.Fatalf("write module failed: %v", err)
	}
	if err := os.WriteFile(mainPath, []byte(`use helper

func main()
  write(helper.triple(7))
endfunc
`), 0o644); err != nil {
		t.Fatalf("write main failed: %v", err)
	}

	unit, err := frontend.AnalyzePath(mainPath)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if len(unit.HIR.Decls()) != 2 {
		t.Fatalf("HIR decl count mismatch: got %d want 2", len(unit.HIR.Decls()))
	}
	if len(unit.MIR.Items) != 3 {
		t.Fatalf("MIR item count mismatch: got %d want 3", len(unit.MIR.Items))
	}
}

func TestAnalyzePathExpandsModulesUsedInsideFunctionBodies(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.gw")
	modulePath := filepath.Join(dir, "helper.gw")
	if err := os.WriteFile(modulePath, []byte(`module helper

export func triple(x: int) -> int
  return x * 3
endfunc
endmodule
`), 0o644); err != nil {
		t.Fatalf("write module failed: %v", err)
	}
	if err := os.WriteFile(mainPath, []byte(`func main()
  use triple from helper
  write(triple(7))
endfunc
`), 0o644); err != nil {
		t.Fatalf("write main failed: %v", err)
	}

	unit, err := frontend.AnalyzePath(mainPath)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if len(unit.HIR.Decls()) != 2 {
		t.Fatalf("HIR decl count mismatch: got %d want 2", len(unit.HIR.Decls()))
	}
	if len(unit.MIR.Items) != 2 {
		t.Fatalf("MIR item count mismatch: got %d want 2", len(unit.MIR.Items))
	}
}

func TestAnalyzeSourceReportsSemanticErrors(t *testing.T) {
	_, err := frontend.AnalyzeSource(`func main()
  http.get("https://example.com")
endfunc`, "bad.gw")
	if err == nil {
		t.Fatal("expected semantic error")
	}
	if got := err.Error(); got != "semantic error at L2: Undefined variable: http" {
		t.Fatalf("error mismatch: got %q", got)
	}
}
