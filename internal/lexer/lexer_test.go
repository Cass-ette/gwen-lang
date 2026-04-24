package lexer_test

import (
	"reflect"
	"testing"

	"github.com/Cass-ette/gwen-lang/internal/lexer"
	"github.com/Cass-ette/gwen-lang/internal/token"
)

func mustTokenize(t *testing.T, source string) []token.Token {
	t.Helper()

	tokens, err := lexer.Tokenize(source)
	if err != nil {
		t.Fatalf("tokenize failed: %v", err)
	}
	return tokens
}

func filteredTypes(tokens []token.Token, excluded ...token.Type) []token.Type {
	skip := make(map[token.Type]struct{}, len(excluded))
	for _, tokenType := range excluded {
		skip[tokenType] = struct{}{}
	}

	var types []token.Type
	for _, tok := range tokens {
		if _, ok := skip[tok.Type]; ok {
			continue
		}
		types = append(types, tok.Type)
	}
	return types
}

func TestBasicAssignment(t *testing.T) {
	tokens := mustTokenize(t, "x := 42")
	types := filteredTypes(tokens, token.EOF)
	want := []token.Type{token.Identifier, token.Assign, token.Integer}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("token types mismatch: got %v want %v", types, want)
	}
	if tokens[0].Value != "x" {
		t.Fatalf("identifier mismatch: got %q want %q", tokens[0].Value, "x")
	}
	if tokens[2].Value != "42" {
		t.Fatalf("integer mismatch: got %q want %q", tokens[2].Value, "42")
	}
}

func TestComparison(t *testing.T) {
	tokens := mustTokenize(t, "x = 42")
	types := filteredTypes(tokens, token.EOF)
	want := []token.Type{token.Identifier, token.Eq, token.Integer}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("token types mismatch: got %v want %v", types, want)
	}
}

func TestNotEqual(t *testing.T) {
	tokens := mustTokenize(t, "x != 0")
	types := filteredTypes(tokens, token.EOF)
	want := []token.Type{token.Identifier, token.Neq, token.Integer}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("token types mismatch: got %v want %v", types, want)
	}
}

func TestKeywords(t *testing.T) {
	tokens := mustTokenize(t, "func endfunc if then else endif while do endwhile")
	types := filteredTypes(tokens, token.EOF, token.Newline)
	want := []token.Type{
		token.Func, token.EndFunc,
		token.If, token.Then, token.Else, token.EndIf,
		token.While, token.Do, token.EndWhile,
	}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("token types mismatch: got %v want %v", types, want)
	}
}

func TestForLoop(t *testing.T) {
	tokens := mustTokenize(t, "for i in 1 to 10 step 2 do")
	types := filteredTypes(tokens, token.EOF, token.Newline)
	want := []token.Type{
		token.For, token.Identifier, token.In,
		token.Integer, token.To, token.Integer,
		token.Step, token.Integer, token.Do,
	}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("token types mismatch: got %v want %v", types, want)
	}
}

func TestString(t *testing.T) {
	tokens := mustTokenize(t, "\"hello world\"")
	if tokens[0].Type != token.String {
		t.Fatalf("token type mismatch: got %v want %v", tokens[0].Type, token.String)
	}
	if tokens[0].Value != "hello world" {
		t.Fatalf("string mismatch: got %q want %q", tokens[0].Value, "hello world")
	}
}

func TestComment(t *testing.T) {
	tokens := mustTokenize(t, "x := 1 // this is a comment\ny := 2")
	types := filteredTypes(tokens, token.EOF, token.Newline)
	want := []token.Type{
		token.Identifier, token.Assign, token.Integer,
		token.Identifier, token.Assign, token.Integer,
	}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("token types mismatch: got %v want %v", types, want)
	}
}

func TestTag(t *testing.T) {
	tokens := mustTokenize(t, "@validate")
	if tokens[0].Type != token.Tag {
		t.Fatalf("token type mismatch: got %v want %v", tokens[0].Type, token.Tag)
	}
	if tokens[0].Value != "validate" {
		t.Fatalf("tag mismatch: got %q want %q", tokens[0].Value, "validate")
	}
}

func TestArrow(t *testing.T) {
	tokens := mustTokenize(t, "func gcd(a: int) -> int")
	types := filteredTypes(tokens, token.EOF, token.Newline)
	want := []token.Type{
		token.Func, token.Identifier, token.LParen,
		token.Identifier, token.Colon, token.Identifier,
		token.RParen, token.Arrow, token.Identifier,
	}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("token types mismatch: got %v want %v", types, want)
	}
}

func TestFatArrow(t *testing.T) {
	tokens := mustTokenize(t, "(x: int) => x * 2")
	types := filteredTypes(tokens, token.EOF, token.Newline)
	want := []token.Type{
		token.LParen, token.Identifier, token.Colon,
		token.Identifier, token.RParen, token.FatArrow,
		token.Identifier, token.Star, token.Integer,
	}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("token types mismatch: got %v want %v", types, want)
	}
}

func TestMatch(t *testing.T) {
	tokens := mustTokenize(t, "match x\n  when 1 => do_a()\n  else do_b()\nendmatch")
	types := filteredTypes(tokens, token.EOF, token.Newline)
	assertContains(t, types, token.Match)
	assertContains(t, types, token.When)
	assertContains(t, types, token.EndMatch)
}

func TestModule(t *testing.T) {
	tokens := mustTokenize(t, "module math_utils\nexport func gcd()\nendfunc\nendmodule")
	types := filteredTypes(tokens, token.EOF, token.Newline)
	assertContains(t, types, token.Module)
	assertContains(t, types, token.Export)
	assertContains(t, types, token.EndModule)
}

func TestParallel(t *testing.T) {
	tokens := mustTokenize(t, "parallel allowfail => results do")
	types := filteredTypes(tokens, token.EOF, token.Newline)
	want := []token.Type{
		token.Parallel, token.AllowFail, token.FatArrow,
		token.Identifier, token.Do,
	}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("token types mismatch: got %v want %v", types, want)
	}
}

func TestGCDExample(t *testing.T) {
	source := `func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc`
	tokens := mustTokenize(t, source)
	types := filteredTypes(tokens, token.EOF, token.Newline)
	if types[0] != token.Func {
		t.Fatalf("first token mismatch: got %v want %v", types[0], token.Func)
	}
	if types[len(types)-1] != token.EndFunc {
		t.Fatalf("last token mismatch: got %v want %v", types[len(types)-1], token.EndFunc)
	}
	assertContains(t, types, token.Mod)
}

func assertContains(t *testing.T, types []token.Type, want token.Type) {
	t.Helper()

	for _, tokenType := range types {
		if tokenType == want {
			return
		}
	}
	t.Fatalf("token types %v do not contain %v", types, want)
}
