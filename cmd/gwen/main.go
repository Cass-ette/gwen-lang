package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Cass-ette/gwen-lang/internal/backend/cgen"
	"github.com/Cass-ette/gwen-lang/internal/frontend"
	"github.com/Cass-ette/gwen-lang/internal/interpreter"
	"github.com/Cass-ette/gwen-lang/internal/lexer"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage()
		return nil
	case "-v", "--version", "version":
		fmt.Println(version)
		return nil
	case "lex":
		if len(args) != 2 {
			return errors.New("usage: gwen lex <path>")
		}
		return lexFile(args[1])
	case "run":
		if len(args) < 2 {
			return errors.New("usage: gwen run <path> [args...]")
		}
		return runFile(args[1], args[2:])
	case "check":
		if len(args) != 2 {
			return errors.New("usage: gwen check <path>")
		}
		return checkFile(args[1])
	case "emit-c":
		if len(args) != 2 {
			return errors.New("usage: gwen emit-c <path>")
		}
		return emitCFile(args[1])
	case "build":
		sourcePath, outputPath, err := parseBuildArgs(args[1:])
		if err != nil {
			return err
		}
		return buildFile(sourcePath, outputPath)
	case "repl":
		return repl()
	default:
		if len(args) >= 1 {
			return runFile(args[0], args[1:])
		}
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func lexFile(path string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	tokens, err := lexer.Tokenize(string(source))
	if err != nil {
		return err
	}

	for _, tok := range tokens {
		fmt.Println(tok)
	}
	return nil
}

func runFile(path string, programArgs []string) error {
	unit, err := frontend.AnalyzePath(path)
	if err != nil {
		return err
	}
	interp := interpreter.New()
	interp.ProgramArgs = append([]string{}, programArgs...)
	return interp.RunWithSource(unit.Program, unit.Path)
}

func checkFile(path string) error {
	if _, err := frontend.AnalyzePath(path); err != nil {
		return err
	}
	fmt.Println("OK")
	return nil
}

func emitCSource(path string) (string, error) {
	unit, err := frontend.AnalyzePath(path)
	if err != nil {
		return "", err
	}
	source, err := cgen.EmitProgram(unit.MIR)
	if err != nil {
		return "", err
	}
	return source, nil
}

func emitCFile(path string) error {
	source, err := emitCSource(path)
	if err != nil {
		return err
	}
	fmt.Print(source)
	return nil
}

func parseBuildArgs(args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", errors.New("usage: gwen build <path> [-o output]")
	}
	sourcePath := ""
	outputPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 >= len(args) {
				return "", "", errors.New("usage: gwen build <path> [-o output]")
			}
			if outputPath != "" {
				return "", "", errors.New("usage: gwen build <path> [-o output]")
			}
			i++
			outputPath = args[i]
		default:
			if sourcePath != "" {
				return "", "", errors.New("usage: gwen build <path> [-o output]")
			}
			sourcePath = args[i]
		}
	}
	if sourcePath == "" {
		return "", "", errors.New("usage: gwen build <path> [-o output]")
	}
	return sourcePath, outputPath, nil
}

func buildFile(path, outputPath string) error {
	cSource, err := emitCSource(path)
	if err != nil {
		return err
	}
	if outputPath == "" {
		outputPath = defaultBuildOutputPath(path)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "gwen-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	cPath := filepath.Join(tmpDir, "program.c")
	if err := os.WriteFile(cPath, []byte(cSource), 0o644); err != nil {
		return err
	}
	compiler, compilerArgs, err := cCompilerCommand()
	if err != nil {
		return err
	}
	args := append(compilerArgs, cPath, "-o", outputPath, "-pthread")
	cmd := exec.Command(compiler, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if text == "" {
			return fmt.Errorf("build failed: %w", err)
		}
		return fmt.Errorf("build failed: %w\n%s", err, text)
	}
	if format, err := detectedBinaryFormat(outputPath); err == nil && format != "" {
		fmt.Printf("%s (%s)\n", outputPath, format)
	} else {
		fmt.Println(outputPath)
	}
	return nil
}

func defaultBuildOutputPath(path string) string {
	return defaultBuildOutputPathForGOOS(path, runtime.GOOS)
}

func defaultBuildOutputPathForGOOS(path, goos string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = "a.out"
	}
	name += defaultBinaryExtensionForGOOS(goos, name)
	return filepath.Join(filepath.Dir(path), name)
}

func defaultBinaryExtensionForGOOS(goos, name string) string {
	if goos != "windows" {
		return ""
	}
	if filepath.Ext(name) != "" {
		return ""
	}
	return ".exe"
}

func detectedBinaryFormat(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) < 2 {
		return "", errors.New("binary too small to detect format")
	}
	return binaryFormatFromHeader(data[:min(len(data), 4)]), nil
}

func binaryFormatFromHeader(header []byte) string {
	if len(header) >= 4 {
		switch {
		case header[0] == 0x7f && header[1] == 'E' && header[2] == 'L' && header[3] == 'F':
			return "ELF"
		case hasMagic(header, 0xfe, 0xed, 0xfa, 0xce),
			hasMagic(header, 0xce, 0xfa, 0xed, 0xfe),
			hasMagic(header, 0xfe, 0xed, 0xfa, 0xcf),
			hasMagic(header, 0xcf, 0xfa, 0xed, 0xfe),
			hasMagic(header, 0xca, 0xfe, 0xba, 0xbe),
			hasMagic(header, 0xbe, 0xba, 0xfe, 0xca),
			hasMagic(header, 0xca, 0xfe, 0xba, 0xbf),
			hasMagic(header, 0xbf, 0xba, 0xfe, 0xca):
			return "Mach-O"
		}
	}
	if len(header) >= 2 && header[0] == 'M' && header[1] == 'Z' {
		return "PE"
	}
	return ""
}

func hasMagic(header []byte, b0, b1, b2, b3 byte) bool {
	return len(header) >= 4 && header[0] == b0 && header[1] == b1 && header[2] == b2 && header[3] == b3
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func cCompilerCommand() (string, []string, error) {
	raw := os.Getenv("CC")
	if strings.TrimSpace(raw) == "" {
		if _, err := exec.LookPath("cc"); err != nil {
			return "", nil, errors.New("build requires a C compiler named 'cc' in PATH")
		}
		return "cc", nil, nil
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return "", nil, errors.New("build requires a valid CC value")
	}
	if _, err := exec.LookPath(parts[0]); err != nil {
		return "", nil, fmt.Errorf("build compiler %q not found in PATH", parts[0])
	}
	return parts[0], parts[1:], nil
}

func repl() error {
	fmt.Println("Gwen REPL (type 'exit' to quit)")

	interp := interpreter.New()
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("gwen> ")
		if !scanner.Scan() {
			fmt.Println()
			return scanner.Err()
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "exit" {
			return nil
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		program, err := frontend.ParseSource(line)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if err := interp.Execute(program); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func printUsage() {
	fmt.Println("Gwen Go frontend")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gwen run <path> [args...]")
	fmt.Println("  gwen check <path>")
	fmt.Println("  gwen emit-c <path>")
	fmt.Println("  gwen build <path> [-o output]")
	fmt.Println("  gwen repl")
	fmt.Println("  gwen lex <path>")
	fmt.Println("  gwen <path> [args...]")
	fmt.Println("  gwen --version")
}
