"""Tests for Gwen lexer."""

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from gwen.lexer import tokenize, TokenType


def test_basic_assignment():
    tokens = tokenize('x := 42')
    types = [t.type for t in tokens if t.type != TokenType.EOF]
    assert types == [TokenType.IDENTIFIER, TokenType.ASSIGN, TokenType.INTEGER]
    assert tokens[0].value == "x"
    assert tokens[2].value == "42"


def test_comparison():
    tokens = tokenize('x = 42')
    types = [t.type for t in tokens if t.type != TokenType.EOF]
    assert types == [TokenType.IDENTIFIER, TokenType.EQ, TokenType.INTEGER]


def test_not_equal():
    tokens = tokenize('x != 0')
    types = [t.type for t in tokens if t.type != TokenType.EOF]
    assert types == [TokenType.IDENTIFIER, TokenType.NEQ, TokenType.INTEGER]


def test_keywords():
    tokens = tokenize('func endfunc if then else endif while do endwhile')
    types = [t.type for t in tokens if t.type not in (TokenType.EOF, TokenType.NEWLINE)]
    assert types == [
        TokenType.FUNC, TokenType.ENDFUNC,
        TokenType.IF, TokenType.THEN, TokenType.ELSE, TokenType.ENDIF,
        TokenType.WHILE, TokenType.DO, TokenType.ENDWHILE,
    ]


def test_for_loop():
    tokens = tokenize('for i in 1 to 10 step 2 do')
    types = [t.type for t in tokens if t.type not in (TokenType.EOF, TokenType.NEWLINE)]
    assert types == [
        TokenType.FOR, TokenType.IDENTIFIER, TokenType.IN,
        TokenType.INTEGER, TokenType.TO, TokenType.INTEGER,
        TokenType.STEP, TokenType.INTEGER, TokenType.DO,
    ]


def test_string():
    tokens = tokenize('"hello world"')
    assert tokens[0].type == TokenType.STRING
    assert tokens[0].value == "hello world"


def test_comment():
    tokens = tokenize('x := 1 // this is a comment\ny := 2')
    types = [t.type for t in tokens if t.type not in (TokenType.EOF, TokenType.NEWLINE)]
    assert types == [
        TokenType.IDENTIFIER, TokenType.ASSIGN, TokenType.INTEGER,
        TokenType.IDENTIFIER, TokenType.ASSIGN, TokenType.INTEGER,
    ]


def test_tag():
    tokens = tokenize('@validate')
    assert tokens[0].type == TokenType.TAG
    assert tokens[0].value == "validate"


def test_arrow():
    tokens = tokenize('func gcd(a: int) -> int')
    types = [t.type for t in tokens if t.type not in (TokenType.EOF, TokenType.NEWLINE)]
    assert types == [
        TokenType.FUNC, TokenType.IDENTIFIER, TokenType.LPAREN,
        TokenType.IDENTIFIER, TokenType.COLON, TokenType.IDENTIFIER,
        TokenType.RPAREN, TokenType.ARROW, TokenType.IDENTIFIER,
    ]


def test_fat_arrow():
    tokens = tokenize('(x: int) => x * 2')
    types = [t.type for t in tokens if t.type not in (TokenType.EOF, TokenType.NEWLINE)]
    assert types == [
        TokenType.LPAREN, TokenType.IDENTIFIER, TokenType.COLON,
        TokenType.IDENTIFIER, TokenType.RPAREN, TokenType.FAT_ARROW,
        TokenType.IDENTIFIER, TokenType.STAR, TokenType.INTEGER,
    ]


def test_match():
    tokens = tokenize('match x\n  when 1 then do_a()\n  else do_b()\nendmatch')
    types = [t.type for t in tokens if t.type not in (TokenType.EOF, TokenType.NEWLINE)]
    assert TokenType.MATCH in types
    assert TokenType.WHEN in types
    assert TokenType.ENDMATCH in types


def test_module():
    tokens = tokenize('module math_utils\nexport func gcd()\nendfunc\nendmodule')
    types = [t.type for t in tokens if t.type not in (TokenType.EOF, TokenType.NEWLINE)]
    assert TokenType.MODULE in types
    assert TokenType.EXPORT in types
    assert TokenType.ENDMODULE in types


def test_parallel():
    tokens = tokenize('parallel allowfail => results do')
    types = [t.type for t in tokens if t.type not in (TokenType.EOF, TokenType.NEWLINE)]
    assert types == [
        TokenType.PARALLEL, TokenType.ALLOWFAIL, TokenType.FAT_ARROW,
        TokenType.IDENTIFIER, TokenType.DO,
    ]


def test_gcd_example():
    source = """func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc"""
    tokens = tokenize(source)
    types = [t.type for t in tokens if t.type not in (TokenType.EOF, TokenType.NEWLINE)]
    assert types[0] == TokenType.FUNC
    assert types[-1] == TokenType.ENDFUNC
    assert TokenType.MOD in types


if __name__ == "__main__":
    tests = [v for k, v in globals().items() if k.startswith("test_")]
    passed = 0
    failed = 0
    for test in tests:
        try:
            test()
            print(f"  PASS  {test.__name__}")
            passed += 1
        except Exception as e:
            print(f"  FAIL  {test.__name__}: {e}")
            failed += 1
    print(f"\n{passed} passed, {failed} failed")
