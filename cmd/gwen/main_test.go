package main

import (
	"io"
	"os"
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
	os.Stdout = writer
	defer func() {
		os.Stdout = old
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	return strings.TrimSpace(string(data))
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
