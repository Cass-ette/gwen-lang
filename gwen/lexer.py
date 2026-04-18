"""Gwen lexer - tokenizes .gw source files."""

from enum import Enum, auto
from dataclasses import dataclass
from typing import List


class TokenType(Enum):
    # Literals
    INTEGER = auto()
    FLOAT = auto()
    STRING = auto()
    IDENTIFIER = auto()
    TAG = auto()           # @name
    AS = auto()            # as

    # Keywords
    FUNC = auto()
    ENDFUNC = auto()
    IF = auto()
    THEN = auto()
    ELIF = auto()
    ELSE = auto()
    ENDIF = auto()
    WHILE = auto()
    DO = auto()
    ENDWHILE = auto()
    FOR = auto()
    IN = auto()
    TO = auto()
    STEP = auto()
    ORDER = auto()       # order (强制正序)
    REVERSE = auto()     # reverse (强制倒序)
    WITH = auto()
    INDEX = auto()
    ENDFOR = auto()
    MATCH = auto()
    WHEN = auto()
    ENDMATCH = auto()
    MODULE = auto()
    ENDMODULE = auto()
    EXPORT = auto()
    USE = auto()
    FROM = auto()
    RETURN = auto()
    PARALLEL = auto()
    ENDPARALLEL = auto()
    ALLOWFAIL = auto()
    OK = auto()
    ERR = auto()
    AND = auto()
    OR = auto()
    NOT = auto()
    TRUE = auto()
    FALSE = auto()
    MOD = auto()
    GLOBAL = auto()        # global
    CONST = auto()         # const
    ARENA = auto()         # arena
    ENDARENA = auto()      # endarena
    VAR = auto()           # var
    ENDVAR = auto()        # endvar
    DEFAULT = auto()       # default

    # Operators
    ASSIGN = auto()        # :=
    ARROW = auto()         # ->
    FAT_ARROW = auto()     # =>
    EQ = auto()            # =
    NEQ = auto()           # !=
    LT = auto()            # <
    GT = auto()            # >
    LTE = auto()           # <=
    GTE = auto()           # >=
    PLUS = auto()          # +
    MINUS = auto()         # -
    STAR = auto()          # *
    SLASH = auto()         # /
    CARET = auto()          # ^

    # Delimiters
    LPAREN = auto()        # (
    RPAREN = auto()        # )
    LBRACKET = auto()      # [
    RBRACKET = auto()      # ]
    COMMA = auto()         # ,
    COLON = auto()         # :
    DOT = auto()           # .
    NEWLINE = auto()

    # Special
    EOF = auto()


KEYWORDS = {
    "func": TokenType.FUNC,
    "endfunc": TokenType.ENDFUNC,
    "if": TokenType.IF,
    "then": TokenType.THEN,
    "elif": TokenType.ELIF,
    "else": TokenType.ELSE,
    "endif": TokenType.ENDIF,
    "while": TokenType.WHILE,
    "do": TokenType.DO,
    "endwhile": TokenType.ENDWHILE,
    "for": TokenType.FOR,
    "in": TokenType.IN,
    "to": TokenType.TO,
    "step": TokenType.STEP,
    "with": TokenType.WITH,
    "index": TokenType.INDEX,
    "endfor": TokenType.ENDFOR,
    "match": TokenType.MATCH,
    "when": TokenType.WHEN,
    "endmatch": TokenType.ENDMATCH,
    "module": TokenType.MODULE,
    "endmodule": TokenType.ENDMODULE,
    "export": TokenType.EXPORT,
    "use": TokenType.USE,
    "from": TokenType.FROM,
    "return": TokenType.RETURN,
    "parallel": TokenType.PARALLEL,
    "endparallel": TokenType.ENDPARALLEL,
    "allowfail": TokenType.ALLOWFAIL,
    "ok": TokenType.OK,
    "err": TokenType.ERR,
    "as": TokenType.AS,
    "and": TokenType.AND,
    "or": TokenType.OR,
    "not": TokenType.NOT,
    "true": TokenType.TRUE,
    "false": TokenType.FALSE,
    "mod": TokenType.MOD,
    "global": TokenType.GLOBAL,
    "const": TokenType.CONST,
    "arena": TokenType.ARENA,
    "endarena": TokenType.ENDARENA,
    "var": TokenType.VAR,
    "endvar": TokenType.ENDVAR,
    "default": TokenType.DEFAULT,
    "order": TokenType.ORDER,
    "reverse": TokenType.REVERSE,
}


@dataclass
class Token:
    type: TokenType
    value: str
    line: int
    column: int

    def __repr__(self):
        return f"Token({self.type.name}, {self.value!r}, L{self.line}:{self.column})"


class LexerError(Exception):
    def __init__(self, message: str, line: int, column: int):
        super().__init__(f"Lexer error at L{line}:{column}: {message}")
        self.line = line
        self.column = column


class Lexer:
    def __init__(self, source: str):
        self.source = source
        self.pos = 0
        self.line = 1
        self.column = 1
        self.tokens: List[Token] = []

    def peek(self) -> str:
        if self.pos >= len(self.source):
            return "\0"
        return self.source[self.pos]

    def peek_next(self) -> str:
        if self.pos + 1 >= len(self.source):
            return "\0"
        return self.source[self.pos + 1]

    def advance(self) -> str:
        ch = self.source[self.pos]
        self.pos += 1
        if ch == "\n":
            self.line += 1
            self.column = 1
        else:
            self.column += 1
        return ch

    def add_token(self, token_type: TokenType, value: str, line: int, column: int):
        self.tokens.append(Token(token_type, value, line, column))

    def skip_whitespace(self):
        while self.pos < len(self.source) and self.peek() in (" ", "\t", "\r"):
            self.advance()

    def skip_line_comment(self):
        # // single line comment - consume until newline
        while self.pos < len(self.source) and self.peek() != "\n":
            self.advance()

    def skip_block_comment(self):
        # /* ... */ block comment (not nested)
        start_line, start_col = self.line, self.column
        self.advance()  # consume /
        self.advance()  # consume *
        while self.pos < len(self.source):
            if self.peek() == "*" and self.peek_next() == "/":
                self.advance()  # consume *
                self.advance()  # consume /
                return
            self.advance()
        raise LexerError("Unterminated block comment", start_line, start_col)

    def read_string(self):
        line, col = self.line, self.column
        quote = self.advance()  # consume opening quote
        result = ""
        while self.pos < len(self.source) and self.peek() != quote:
            if self.peek() == "\\":
                self.advance()
                ch = self.advance()
                if ch == "n":
                    result += "\n"
                elif ch == "t":
                    result += "\t"
                elif ch == "\\":
                    result += "\\"
                elif ch == quote:
                    result += quote
                else:
                    result += "\\" + ch
            else:
                result += self.advance()
        if self.pos >= len(self.source):
            raise LexerError("Unterminated string", line, col)
        self.advance()  # consume closing quote
        self.add_token(TokenType.STRING, result, line, col)

    def read_number(self):
        line, col = self.line, self.column
        result = ""
        is_float = False
        while self.pos < len(self.source) and (self.peek().isdigit() or self.peek() == "."):
            if self.peek() == ".":
                if is_float:
                    break
                is_float = True
            result += self.advance()
        token_type = TokenType.FLOAT if is_float else TokenType.INTEGER
        self.add_token(token_type, result, line, col)

    def read_identifier_or_keyword(self):
        line, col = self.line, self.column
        result = ""
        while self.pos < len(self.source) and (self.peek().isalnum() or self.peek() == "_"):
            result += self.advance()
        token_type = KEYWORDS.get(result, TokenType.IDENTIFIER)
        self.add_token(token_type, result, line, col)

    def read_tag(self):
        line, col = self.line, self.column
        self.advance()  # consume @
        name = ""
        while self.pos < len(self.source) and (self.peek().isalnum() or self.peek() == "_"):
            name += self.advance()
        if not name:
            raise LexerError("Expected tag name after @", line, col)
        self.add_token(TokenType.TAG, name, line, col)

    def tokenize(self) -> List[Token]:
        while self.pos < len(self.source):
            ch = self.peek()

            # Whitespace (not newline)
            if ch in (" ", "\t", "\r"):
                self.skip_whitespace()
                continue

            # Newline
            if ch == "\n":
                line, col = self.line, self.column
                self.advance()
                # Collapse multiple newlines
                if not self.tokens or self.tokens[-1].type != TokenType.NEWLINE:
                    self.add_token(TokenType.NEWLINE, "\\n", line, col)
                continue

            # Comment
            # Line comment
            if ch == "/" and self.peek_next() == "/":
                self.skip_line_comment()
                continue

            # Block comment
            if ch == "/" and self.peek_next() == "*":
                self.skip_block_comment()
                continue

            # String
            if ch in ('"', "'"):
                self.read_string()
                continue

            # Number
            if ch.isdigit():
                self.read_number()
                continue

            # Identifier or keyword
            if ch.isalpha() or ch == "_":
                self.read_identifier_or_keyword()
                continue

            # Tag
            if ch == "@":
                self.read_tag()
                continue

            # Two-character operators
            line, col = self.line, self.column
            if ch == ":" and self.peek_next() == "=":
                self.advance()
                self.advance()
                self.add_token(TokenType.ASSIGN, ":=", line, col)
                continue
            if ch == "-" and self.peek_next() == ">":
                self.advance()
                self.advance()
                self.add_token(TokenType.ARROW, "->", line, col)
                continue
            if ch == "=" and self.peek_next() == ">":
                self.advance()
                self.advance()
                self.add_token(TokenType.FAT_ARROW, "=>", line, col)
                continue
            if ch == "!" and self.peek_next() == "=":
                self.advance()
                self.advance()
                self.add_token(TokenType.NEQ, "!=", line, col)
                continue
            if ch == "<" and self.peek_next() == "=":
                self.advance()
                self.advance()
                self.add_token(TokenType.LTE, "<=", line, col)
                continue
            if ch == ">" and self.peek_next() == "=":
                self.advance()
                self.advance()
                self.add_token(TokenType.GTE, ">=", line, col)
                continue


            # Single-character operators/delimiters
            single_chars = {
                "=": TokenType.EQ,
                "<": TokenType.LT,
                ">": TokenType.GT,
                "+": TokenType.PLUS,
                "-": TokenType.MINUS,
                "*": TokenType.STAR,
                "/": TokenType.SLASH,
                "^": TokenType.CARET,
                "(": TokenType.LPAREN,
                ")": TokenType.RPAREN,
                "[": TokenType.LBRACKET,
                "]": TokenType.RBRACKET,
                ",": TokenType.COMMA,
                ":": TokenType.COLON,
                ".": TokenType.DOT,
            }

            if ch in single_chars:
                self.advance()
                self.add_token(single_chars[ch], ch, line, col)
                continue

            raise LexerError(f"Unexpected character: {ch!r}", self.line, self.column)

        self.add_token(TokenType.EOF, "", self.line, self.column)
        return self.tokens


def tokenize(source: str) -> List[Token]:
    """Convenience function to tokenize source code."""
    return Lexer(source).tokenize()
