package token

import "fmt"

type Type int

const (
	Integer Type = iota
	Float
	String
	Identifier
	Tag
	As
	Func
	EndFunc
	If
	Then
	Elif
	Else
	EndIf
	While
	Do
	EndWhile
	For
	In
	To
	Step
	Order
	Reverse
	With
	Index
	EndFor
	Match
	When
	EndMatch
	Module
	EndModule
	Export
	Use
	From
	Return
	Parallel
	EndParallel
	AllowFail
	Ok
	Err
	And
	Or
	Not
	True
	False
	Mod
	Global
	Const
	Arena
	EndArena
	Var
	EndVar
	Default
	Object
	EndObject
	New
	EndNew
	Assign
	Arrow
	FatArrow
	Eq
	Neq
	Lt
	Gt
	Lte
	Gte
	Plus
	Minus
	Star
	Slash
	Caret
	LParen
	RParen
	LBracket
	RBracket
	LBrace
	RBrace
	Comma
	Colon
	Dot
	Newline
	EOF
)

var typeNames = [...]string{
	"INTEGER",
	"FLOAT",
	"STRING",
	"IDENTIFIER",
	"TAG",
	"AS",
	"FUNC",
	"ENDFUNC",
	"IF",
	"THEN",
	"ELIF",
	"ELSE",
	"ENDIF",
	"WHILE",
	"DO",
	"ENDWHILE",
	"FOR",
	"IN",
	"TO",
	"STEP",
	"ORDER",
	"REVERSE",
	"WITH",
	"INDEX",
	"ENDFOR",
	"MATCH",
	"WHEN",
	"ENDMATCH",
	"MODULE",
	"ENDMODULE",
	"EXPORT",
	"USE",
	"FROM",
	"RETURN",
	"PARALLEL",
	"ENDPARALLEL",
	"ALLOWFAIL",
	"OK",
	"ERR",
	"AND",
	"OR",
	"NOT",
	"TRUE",
	"FALSE",
	"MOD",
	"GLOBAL",
	"CONST",
	"ARENA",
	"ENDARENA",
	"VAR",
	"ENDVAR",
	"DEFAULT",
	"OBJECT",
	"ENDOBJECT",
	"NEW",
	"ENDNEW",
	"ASSIGN",
	"ARROW",
	"FAT_ARROW",
	"EQ",
	"NEQ",
	"LT",
	"GT",
	"LTE",
	"GTE",
	"PLUS",
	"MINUS",
	"STAR",
	"SLASH",
	"CARET",
	"LPAREN",
	"RPAREN",
	"LBRACKET",
	"RBRACKET",
	"LBRACE",
	"RBRACE",
	"COMMA",
	"COLON",
	"DOT",
	"NEWLINE",
	"EOF",
}

var keywords = map[string]Type{
	"func":        Func,
	"endfunc":     EndFunc,
	"if":          If,
	"then":        Then,
	"elif":        Elif,
	"else":        Else,
	"endif":       EndIf,
	"while":       While,
	"do":          Do,
	"endwhile":    EndWhile,
	"for":         For,
	"in":          In,
	"to":          To,
	"step":        Step,
	"with":        With,
	"index":       Index,
	"endfor":      EndFor,
	"match":       Match,
	"when":        When,
	"endmatch":    EndMatch,
	"module":      Module,
	"endmodule":   EndModule,
	"export":      Export,
	"use":         Use,
	"from":        From,
	"return":      Return,
	"parallel":    Parallel,
	"endparallel": EndParallel,
	"allowfail":   AllowFail,
	"ok":          Ok,
	"err":         Err,
	"as":          As,
	"and":         And,
	"or":          Or,
	"not":         Not,
	"true":        True,
	"false":       False,
	"mod":         Mod,
	"global":      Global,
	"const":       Const,
	"arena":       Arena,
	"endarena":    EndArena,
	"var":         Var,
	"endvar":      EndVar,
	"default":     Default,
	"order":       Order,
	"reverse":     Reverse,
	"object":      Object,
	"endobject":   EndObject,
	"new":         New,
	"endnew":      EndNew,
}

type Position struct {
	Line   int
	Column int
}

type Token struct {
	Type   Type
	Value  string
	Line   int
	Column int
}

func LookupIdentifier(value string) Type {
	if tokenType, ok := keywords[value]; ok {
		return tokenType
	}
	return Identifier
}

func (t Type) String() string {
	if int(t) < len(typeNames) {
		return typeNames[t]
	}
	return fmt.Sprintf("Type(%d)", t)
}

func (t Token) Pos() Position {
	return Position{Line: t.Line, Column: t.Column}
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%s, %q, L%d:%d)", t.Type, t.Value, t.Line, t.Column)
}
