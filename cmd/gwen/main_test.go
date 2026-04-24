package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	dataCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		data, readErr := io.ReadAll(reader)
		if readErr != nil {
			errCh <- readErr
			return
		}
		dataCh <- strings.TrimSpace(string(data))
	}()
	os.Stdout = writer
	defer func() {
		os.Stdout = old
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatalf("read failed: %v", err)
	case data := <-dataCh:
		return data
	}
	return ""
}

func requireExternalTool(t *testing.T, name string) {
	t.Helper()

	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("skipping integration test: missing %s", name)
	}
}

func TestRunPassesProgramArgs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gw")
	source := `use args from os

func main()
  argv := args()
  write(len(argv))
  write(argv[0])
  write(argv[1])
endfunc
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out := captureStdout(t, func() {
		if err := run([]string{"run", path, "serve", "--port=8080"}); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if out != "2\nserve\n--port=8080" {
		t.Fatalf("output mismatch: got %q want %q", out, "2\nserve\n--port=8080")
	}
}

func TestEmitCPrintsGeneratedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gw")
	source := `func main()
  write(42)
endfunc
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out := captureStdout(t, func() {
		if err := run([]string{"emit-c", path}); err != nil {
			t.Fatalf("emit-c failed: %v", err)
		}
	})

	wants := []string{
		"#include <stdio.h>",
		"static void gwen_fn_main(void);",
		"gwen_write_int(42LL);",
		"int main(int argc, char **argv) {",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("generated C missing %q\n%s", want, out)
		}
	}
}

func TestBuildProducesExecutable(t *testing.T) {
	requireExternalTool(t, "cc")

	path := filepath.Join(t.TempDir(), "main.gw")
	source := `func main()
  write(42)
endfunc
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	binPath := strings.TrimSuffix(path, filepath.Ext(path))
	out := captureStdout(t, func() {
		if err := run([]string{"build", path}); err != nil {
			t.Fatalf("build failed: %v", err)
		}
	})

	format, err := detectedBinaryFormat(binPath)
	if err != nil {
		t.Fatalf("detect binary format failed: %v", err)
	}
	if out != binPath+" ("+format+")" {
		t.Fatalf("build path mismatch: got %q want %q", out, binPath+" ("+format+")")
	}
	output, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run built binary failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "42"; got != want {
		t.Fatalf("built binary output mismatch: got %q want %q", got, want)
	}
}

func TestBuildSupportsOutputFlag(t *testing.T) {
	requireExternalTool(t, "cc")

	dir := t.TempDir()
	path := filepath.Join(dir, "main.gw")
	outPath := filepath.Join(dir, "bin", "hello")
	source := `func main()
  write("ok")
endfunc
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out := captureStdout(t, func() {
		if err := run([]string{"build", path, "-o", outPath}); err != nil {
			t.Fatalf("build failed: %v", err)
		}
	})

	format, err := detectedBinaryFormat(outPath)
	if err != nil {
		t.Fatalf("detect binary format failed: %v", err)
	}
	if out != outPath+" ("+format+")" {
		t.Fatalf("build output path mismatch: got %q want %q", out, outPath+" ("+format+")")
	}
	output, err := exec.Command(outPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run built binary failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "ok"; got != want {
		t.Fatalf("built binary output mismatch: got %q want %q", got, want)
	}
}

func TestBuildRejectsMissingSourcePath(t *testing.T) {
	if err := run([]string{"build"}); err == nil || err.Error() != "usage: gwen build <path> [-o output]" {
		t.Fatalf("unexpected build usage error: %v", err)
	}
}

func TestDefaultBuildOutputPathForGOOS(t *testing.T) {
	got := defaultBuildOutputPathForGOOS("/tmp/hello.gw", "darwin")
	if got != "/tmp/hello" {
		t.Fatalf("darwin output path mismatch: got %q want %q", got, "/tmp/hello")
	}

	got = defaultBuildOutputPathForGOOS("hello.gw", "windows")
	if got != "hello.exe" {
		t.Fatalf("windows output path mismatch: got %q want %q", got, "hello.exe")
	}
}

func TestBinaryFormatFromHeader(t *testing.T) {
	cases := []struct {
		name   string
		header []byte
		want   string
	}{
		{name: "elf", header: []byte{0x7f, 'E', 'L', 'F'}, want: "ELF"},
		{name: "macho", header: []byte{0xfe, 0xed, 0xfa, 0xcf}, want: "Mach-O"},
		{name: "pe", header: []byte{'M', 'Z', 0x90, 0x00}, want: "PE"},
		{name: "unknown", header: []byte{0x00, 0x01, 0x02, 0x03}, want: ""},
	}

	for _, tc := range cases {
		if got := binaryFormatFromHeader(tc.header); got != tc.want {
			t.Fatalf("%s format mismatch: got %q want %q", tc.name, got, tc.want)
		}
	}
}
