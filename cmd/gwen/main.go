package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Cass-ette/gwen-lang/internal/checker"
	"github.com/Cass-ette/gwen-lang/internal/interpreter"
	"github.com/Cass-ette/gwen-lang/internal/lexer"
	"github.com/Cass-ette/gwen-lang/internal/parser"
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
		if len(args) != 2 {
			return errors.New("usage: gwen run <path>")
		}
		return runFile(args[1])
	case "check":
		if len(args) != 2 {
			return errors.New("usage: gwen check <path>")
		}
		return checkFile(args[1])
	case "repl":
		return repl()
	default:
		if len(args) == 1 {
			return runFile(args[0])
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

func runFile(path string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	program, err := parser.Parse(string(source))
	if err != nil {
		return err
	}
	return interpreter.New().RunWithSource(program, path)
}

func checkFile(path string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	program, err := parser.Parse(string(source))
	if err != nil {
		return err
	}
	if err := checker.New().CheckProgram(program, path); err != nil {
		return err
	}
	fmt.Println("OK")
	return nil
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
		program, err := parser.Parse(line)
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
	fmt.Println("  gwen run <path>")
	fmt.Println("  gwen check <path>")
	fmt.Println("  gwen repl")
	fmt.Println("  gwen lex <path>")
	fmt.Println("  gwen <path>")
	fmt.Println("  gwen --version")
}
