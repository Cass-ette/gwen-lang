package lexer

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Cass-ette/gwen-lang/internal/token"
)

const eofRune = rune(0)

type Error struct {
	Message string
	Line    int
	Column  int
}

func (e *Error) Error() string {
	return fmt.Sprintf("lexer error at L%d:%d: %s", e.Line, e.Column, e.Message)
}

type Lexer struct {
	source []rune
	pos    int
	line   int
	column int
	tokens []token.Token
}

func New(source string) *Lexer {
	return &Lexer{
		source: []rune(source),
		line:   1,
		column: 1,
	}
}

func Tokenize(source string) ([]token.Token, error) {
	return New(source).Tokenize()
}

func (l *Lexer) Tokenize() ([]token.Token, error) {
	for l.pos < len(l.source) {
		ch := l.peek()

		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.skipWhitespace()
			continue
		}

		if ch == '\n' {
			line, column := l.line, l.column
			l.advance()
			if len(l.tokens) == 0 || l.tokens[len(l.tokens)-1].Type != token.Newline {
				l.addToken(token.Newline, "\n", line, column)
			}
			continue
		}

		if ch == '/' && l.peekNext() == '/' {
			l.skipLineComment()
			continue
		}

		if ch == '/' && l.peekNext() == '*' {
			if err := l.skipBlockComment(); err != nil {
				return nil, err
			}
			continue
		}

		if ch == '"' || ch == '\'' {
			if err := l.readString(); err != nil {
				return nil, err
			}
			continue
		}

		if unicode.IsDigit(ch) {
			l.readNumber()
			continue
		}

		if unicode.IsLetter(ch) || ch == '_' {
			l.readIdentifierOrKeyword()
			continue
		}

		if ch == '@' {
			if err := l.readTag(); err != nil {
				return nil, err
			}
			continue
		}

		line, column := l.line, l.column

		switch {
		case ch == ':' && l.peekNext() == '=':
			l.advance()
			l.advance()
			l.addToken(token.Assign, ":=", line, column)
			continue
		case ch == '-' && l.peekNext() == '>':
			l.advance()
			l.advance()
			l.addToken(token.Arrow, "->", line, column)
			continue
		case ch == '=' && l.peekNext() == '>':
			l.advance()
			l.advance()
			l.addToken(token.FatArrow, "=>", line, column)
			continue
		case ch == '!' && l.peekNext() == '=':
			l.advance()
			l.advance()
			l.addToken(token.Neq, "!=", line, column)
			continue
		case ch == '<' && l.peekNext() == '=':
			l.advance()
			l.advance()
			l.addToken(token.Lte, "<=", line, column)
			continue
		case ch == '>' && l.peekNext() == '=':
			l.advance()
			l.advance()
			l.addToken(token.Gte, ">=", line, column)
			continue
		}

		if tokenType, ok := singleCharTokens[ch]; ok {
			l.advance()
			l.addToken(tokenType, string(ch), line, column)
			continue
		}

		return nil, &Error{
			Message: fmt.Sprintf("unexpected character: %q", ch),
			Line:    l.line,
			Column:  l.column,
		}
	}

	l.addToken(token.EOF, "", l.line, l.column)
	return l.tokens, nil
}

var singleCharTokens = map[rune]token.Type{
	'=': token.Eq,
	'<': token.Lt,
	'>': token.Gt,
	'+': token.Plus,
	'-': token.Minus,
	'*': token.Star,
	'/': token.Slash,
	'^': token.Caret,
	'(': token.LParen,
	')': token.RParen,
	'[': token.LBracket,
	']': token.RBracket,
	'{': token.LBrace,
	'}': token.RBrace,
	',': token.Comma,
	':': token.Colon,
	'.': token.Dot,
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.source) {
		return eofRune
	}
	return l.source[l.pos]
}

func (l *Lexer) peekNext() rune {
	if l.pos+1 >= len(l.source) {
		return eofRune
	}
	return l.source[l.pos+1]
}

func (l *Lexer) advance() rune {
	ch := l.source[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return ch
}

func (l *Lexer) addToken(tokenType token.Type, value string, line, column int) {
	l.tokens = append(l.tokens, token.Token{
		Type:   tokenType,
		Value:  value,
		Line:   line,
		Column: column,
	})
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.source) {
		ch := l.peek()
		if ch != ' ' && ch != '\t' && ch != '\r' {
			return
		}
		l.advance()
	}
}

func (l *Lexer) skipLineComment() {
	for l.pos < len(l.source) && l.peek() != '\n' {
		l.advance()
	}
}

func (l *Lexer) skipBlockComment() error {
	startLine, startColumn := l.line, l.column
	l.advance()
	l.advance()

	for l.pos < len(l.source) {
		if l.peek() == '*' && l.peekNext() == '/' {
			l.advance()
			l.advance()
			return nil
		}
		l.advance()
	}

	return &Error{
		Message: "unterminated block comment",
		Line:    startLine,
		Column:  startColumn,
	}
}

func (l *Lexer) readString() error {
	line, column := l.line, l.column
	quote := l.advance()
	var builder strings.Builder

	for l.pos < len(l.source) && l.peek() != quote {
		if l.peek() == '\\' {
			l.advance()
			if l.pos >= len(l.source) {
				return &Error{
					Message: "unterminated string",
					Line:    line,
					Column:  column,
				}
			}

			ch := l.advance()
			switch ch {
			case 'n':
				builder.WriteRune('\n')
			case 't':
				builder.WriteRune('\t')
			case '\\':
				builder.WriteRune('\\')
			default:
				if ch == quote {
					builder.WriteRune(quote)
				} else {
					builder.WriteRune('\\')
					builder.WriteRune(ch)
				}
			}
			continue
		}

		builder.WriteRune(l.advance())
	}

	if l.pos >= len(l.source) {
		return &Error{
			Message: "unterminated string",
			Line:    line,
			Column:  column,
		}
	}

	l.advance()
	l.addToken(token.String, builder.String(), line, column)
	return nil
}

func (l *Lexer) readNumber() {
	line, column := l.line, l.column
	var builder strings.Builder
	isFloat := false

	for l.pos < len(l.source) {
		ch := l.peek()
		if !unicode.IsDigit(ch) && ch != '.' {
			break
		}
		if ch == '.' {
			if isFloat {
				break
			}
			isFloat = true
		}
		builder.WriteRune(l.advance())
	}

	tokenType := token.Integer
	if isFloat {
		tokenType = token.Float
	}
	l.addToken(tokenType, builder.String(), line, column)
}

func (l *Lexer) readIdentifierOrKeyword() {
	line, column := l.line, l.column
	var builder strings.Builder

	for l.pos < len(l.source) {
		ch := l.peek()
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			break
		}
		builder.WriteRune(l.advance())
	}

	value := builder.String()
	l.addToken(token.LookupIdentifier(value), value, line, column)
}

func (l *Lexer) readTag() error {
	line, column := l.line, l.column
	l.advance()

	var builder strings.Builder
	for l.pos < len(l.source) {
		ch := l.peek()
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			break
		}
		builder.WriteRune(l.advance())
	}

	if builder.Len() == 0 {
		return &Error{
			Message: "expected tag name after @",
			Line:    line,
			Column:  column,
		}
	}

	l.addToken(token.Tag, builder.String(), line, column)
	return nil
}
