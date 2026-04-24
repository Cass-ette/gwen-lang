package parser

import (
	"fmt"
	"strconv"

	"github.com/Cass-ette/gwen-lang/internal/ast"
	"github.com/Cass-ette/gwen-lang/internal/lexer"
	"github.com/Cass-ette/gwen-lang/internal/token"
)

type Error struct {
	Message string
	Token   token.Token
}

func (e *Error) Error() string {
	return fmt.Sprintf("parse error at L%d:%d: %s", e.Token.Line, e.Token.Column, e.Message)
}

type Parser struct {
	tokens []token.Token
	pos    int
}

func New(tokens []token.Token) *Parser {
	return &Parser{tokens: tokens}
}

func Parse(source string) (*ast.Program, error) {
	tokens, err := lexer.Tokenize(source)
	if err != nil {
		return nil, err
	}
	return New(tokens).Parse()
}

func (p *Parser) Parse() (*ast.Program, error) {
	statements, err := p.parseBlockUntil(token.EOF)
	if err != nil {
		return nil, err
	}
	return &ast.Program{Statements: statements}, nil
}

func (p *Parser) parseBlockUntil(endTypes ...token.Type) ([]any, error) {
	var statements []any
	p.skipNewlines()
	for !p.at(endTypes...) {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			statements = append(statements, stmt)
		}
		p.skipNewlines()
	}
	return statements, nil
}

func (p *Parser) parseStatement() (any, error) {
	tok := p.peek()

	switch tok.Type {
	case token.Func:
		return p.parseFuncDef(false)
	case token.Export:
		p.advance()
		switch {
		case p.at(token.Func):
			return p.parseFuncDef(true)
		case p.at(token.Object):
			return p.parseObjectDef(true)
		case p.at(token.Identifier) && p.peek().Value == "type":
			next := p.peekNext()
			if next != nil && next.Type == token.Identifier {
				return p.parseTypeAlias(true)
			}
		}
		return nil, p.errorf(p.peek(), "expected 'func', 'object', or 'type' after 'export'")
	case token.If:
		return p.parseIf()
	case token.While:
		return p.parseWhile()
	case token.For:
		return p.parseFor()
	case token.Match:
		return p.parseMatch()
	case token.Module:
		return p.parseModule()
	case token.Use:
		return p.parseUse()
	case token.Return:
		return p.parseReturn()
	case token.Parallel:
		return p.parseParallel()
	case token.Global:
		return p.parseGlobal()
	case token.Const:
		return p.parseConst()
	case token.Arena:
		return p.parseArena()
	case token.Var:
		return p.parseVarBlock()
	case token.Object:
		return p.parseObjectDef(false)
	case token.Tag:
		p.advance()
		return &ast.TagStmt{Name: tok.Value, Line: tok.Line}, nil
	case token.Newline:
		p.advance()
		return nil, nil
	}

	if tok.Type == token.Identifier && tok.Value == "type" {
		next := p.peekNext()
		if next != nil && next.Type == token.Identifier {
			return p.parseTypeAlias(false)
		}
	}

	if tok.Type == token.Identifier {
		switch tok.Value {
		case "pass":
			next := p.peekNext()
			if next == nil || next.Type == token.Newline || next.Type == token.EOF {
				return p.parsePass()
			}
		case "leave":
			next := p.peekNext()
			if next != nil && next.Type == token.Identifier {
				return p.parseLeave()
			}
		case "next":
			next := p.peekNext()
			if next != nil && next.Type == token.Identifier {
				return p.parseNext()
			}
		}
	}

	return p.parseAssignmentOrExpr()
}

func (p *Parser) parseAssignmentOrExpr() (any, error) {
	tok := p.peek()
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	if p.at(token.Assign) {
		p.advance()
		target, ok := assignmentTarget(expr)
		if !ok {
			return nil, p.errorf(p.peek(), "expected identifier, index access, or member access in assignment")
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.Assignment{
			Targets: []any{target},
			Values:  []any{value},
			Line:    tok.Line,
		}, nil
	}

	if p.at(token.Comma) {
		firstTarget, ok := assignmentTarget(expr)
		if ok {
			targets := []any{firstTarget}
			for p.at(token.Comma) {
				p.advance()
				nextExpr, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				target, ok := assignmentTarget(nextExpr)
				if !ok {
					return nil, p.errorf(p.peek(), "expected identifier, index access, or member access in multi-assignment")
				}
				targets = append(targets, target)
			}
			if _, err := p.expect(token.Assign); err != nil {
				return nil, err
			}
			values := make([]any, 0, len(targets))
			value, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			values = append(values, value)
			for p.at(token.Comma) {
				p.advance()
				value, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				values = append(values, value)
			}
			return &ast.Assignment{Targets: targets, Values: values, Line: tok.Line}, nil
		}
	}

	if p.at(token.Colon) {
		identifier, ok := expr.(*ast.Identifier)
		if ok {
			p.advance()
			typeNode, err := p.parseType()
			if err != nil {
				return nil, err
			}
			var value any
			isUninit := true
			if p.at(token.Assign) {
				p.advance()
				value, err = p.parseExpr()
				if err != nil {
					return nil, err
				}
				isUninit = false
			}
			return &ast.VarDecl{
				Name:     identifier.Name,
				TypeName: typeNode,
				Value:    value,
				IsUninit: isUninit,
				Line:     tok.Line,
			}, nil
		}
	}

	return &ast.ExprStmt{Expr: expr, Line: tok.Line}, nil
}

func (p *Parser) parseExpr() (any, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (any, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.at(token.Or) {
		op := p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Left: left, Op: op.Value, Right: right, Line: lineOf(left)}
	}
	return left, nil
}

func (p *Parser) parseAnd() (any, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.at(token.And) {
		op := p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Left: left, Op: op.Value, Right: right, Line: lineOf(left)}
	}
	return left, nil
}

func (p *Parser) parseComparison() (any, error) {
	left, err := p.parseAddition()
	if err != nil {
		return nil, err
	}
	for p.at(token.Eq, token.Neq, token.Lt, token.Gt, token.Lte, token.Gte) {
		op := p.advance()
		right, err := p.parseAddition()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Left: left, Op: op.Value, Right: right, Line: lineOf(left)}
	}
	return left, nil
}

func (p *Parser) parseAddition() (any, error) {
	left, err := p.parseMultiplication()
	if err != nil {
		return nil, err
	}
	for p.at(token.Plus, token.Minus) {
		op := p.advance()
		right, err := p.parseMultiplication()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Left: left, Op: op.Value, Right: right, Line: lineOf(left)}
	}
	return left, nil
}

func (p *Parser) parseMultiplication() (any, error) {
	left, err := p.parsePower()
	if err != nil {
		return nil, err
	}
	for p.at(token.Star, token.Slash, token.Mod) {
		op := p.advance()
		right, err := p.parsePower()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Left: left, Op: op.Value, Right: right, Line: lineOf(left)}
	}
	return left, nil
}

func (p *Parser) parsePower() (any, error) {
	base, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	if p.at(token.Caret) {
		op := p.advance()
		exponent, err := p.parsePower()
		if err != nil {
			return nil, err
		}
		return &ast.BinaryOp{Left: base, Op: op.Value, Right: exponent, Line: lineOf(base)}, nil
	}
	return base, nil
}

func (p *Parser) parseUnary() (any, error) {
	if p.at(token.Minus, token.Not) {
		tok := p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryOp{Op: tok.Value, Operand: operand, Line: tok.Line}, nil
	}

	expr, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}

	for p.at(token.As) {
		asTok := p.advance()
		typeTok, err := p.expect(token.Identifier)
		if err != nil {
			return nil, err
		}
		name := typeTok.Value
		if p.at(token.LBracket) {
			p.advance()
			partTok, err := p.expect(token.Identifier)
			if err != nil {
				return nil, err
			}
			parts := []string{partTok.Value}
			for p.at(token.Comma) {
				p.advance()
				partTok, err := p.expect(token.Identifier)
				if err != nil {
					return nil, err
				}
				parts = append(parts, partTok.Value)
			}
			if _, err := p.expect(token.RBracket); err != nil {
				return nil, err
			}
			name = fmt.Sprintf("%s[%s]", name, joinComma(parts))
		}
		expr = &ast.AsExpr{Expr: expr, TypeName: name, Line: asTok.Line}
	}

	return expr, nil
}

func (p *Parser) parsePostfix() (any, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		switch {
		case p.at(token.LParen):
			expr, err = p.parseCall(expr)
			if err != nil {
				return nil, err
			}
		case p.at(token.Dot):
			p.advance()
			var member token.Token
			if p.at(token.New) {
				member = p.advance()
			} else {
				member, err = p.expect(token.Identifier)
				if err != nil {
					return nil, err
				}
			}
			expr = &ast.MemberAccess{Object: expr, Member: member.Value, Line: lineOf(expr)}
		case p.at(token.LBracket):
			p.advance()
			index, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.RBracket); err != nil {
				return nil, err
			}
			expr = &ast.IndexAccess{Object: expr, Index: index, Line: lineOf(expr)}
		default:
			return expr, nil
		}
	}
}

func (p *Parser) parseCall(callee any) (any, error) {
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	var args []any
	if !p.at(token.RParen) {
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		for p.at(token.Comma) {
			p.advance()
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	return &ast.FuncCall{Name: callee, Args: args, Line: lineOf(callee)}, nil
}

func (p *Parser) parsePrimary() (any, error) {
	tok := p.peek()

	switch tok.Type {
	case token.Integer:
		p.advance()
		value, err := strconv.ParseInt(tok.Value, 10, 64)
		if err != nil {
			return nil, err
		}
		return &ast.IntLiteral{Value: value, Line: tok.Line}, nil
	case token.Float:
		p.advance()
		value, err := strconv.ParseFloat(tok.Value, 64)
		if err != nil {
			return nil, err
		}
		return &ast.FloatLiteral{Value: value, Line: tok.Line}, nil
	case token.String:
		p.advance()
		return &ast.StringLiteral{Value: tok.Value, Line: tok.Line}, nil
	case token.True:
		p.advance()
		return &ast.BoolLiteral{Value: true, Line: tok.Line}, nil
	case token.False:
		p.advance()
		return &ast.BoolLiteral{Value: false, Line: tok.Line}, nil
	case token.Ok:
		p.advance()
		if _, err := p.expect(token.LParen); err != nil {
			return nil, err
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.RParen); err != nil {
			return nil, err
		}
		return &ast.OkExpr{Value: value, Line: tok.Line}, nil
	case token.Err:
		p.advance()
		if _, err := p.expect(token.LParen); err != nil {
			return nil, err
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.RParen); err != nil {
			return nil, err
		}
		return &ast.ErrExpr{Value: value, Line: tok.Line}, nil
	case token.Identifier:
		if tok.Value == "dict" {
			return p.parseDictLiteral()
		}
		if next := p.peekNext(); next != nil && next.Type == token.LBrace {
			return p.parseObjectLiteral()
		}
		p.advance()
		return &ast.Identifier{Name: tok.Value, Line: tok.Line}, nil
	case token.Index:
		p.advance()
		return &ast.Identifier{Name: tok.Value, Line: tok.Line}, nil
	case token.LBracket:
		return p.parseListLiteral()
	case token.LParen:
		return p.parseParenOrLambda()
	default:
		return nil, p.errorf(tok, "unexpected token: %s (%q)", tok.Type, tok.Value)
	}
}

func (p *Parser) parseListLiteral() (any, error) {
	tok := p.advance()
	var elements []any
	p.skipNewlines()
	if !p.at(token.RBracket) {
		element, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
		for {
			p.skipNewlines()
			if p.at(token.RBracket) {
				break
			}
			if !p.at(token.Comma) {
				break
			}
			p.advance()
			p.skipNewlines()
			if p.at(token.RBracket) {
				break
			}
			element, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			elements = append(elements, element)
		}
	}
	if _, err := p.expect(token.RBracket); err != nil {
		return nil, err
	}
	return &ast.ListLiteral{Elements: elements, Line: tok.Line}, nil
}

func (p *Parser) parseDictLiteral() (any, error) {
	tok := p.advance()
	if _, err := p.expect(token.LBracket); err != nil {
		return nil, err
	}
	keyType, err := p.parseType()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Comma); err != nil {
		return nil, err
	}
	valueType, err := p.parseType()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RBracket); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.LBrace); err != nil {
		return nil, err
	}

	var entries []ast.DictEntry
	p.skipNewlines()
	if !p.at(token.RBrace) {
		key, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Colon); err != nil {
			return nil, err
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		entries = append(entries, ast.DictEntry{Key: key, Value: value})

		for {
			p.skipNewlines()
			if p.at(token.RBrace) {
				break
			}
			if !p.at(token.Comma) {
				break
			}
			p.advance()
			p.skipNewlines()
			if p.at(token.RBrace) {
				break
			}
			key, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.Colon); err != nil {
				return nil, err
			}
			value, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			entries = append(entries, ast.DictEntry{Key: key, Value: value})
		}
	}

	if _, err := p.expect(token.RBrace); err != nil {
		return nil, err
	}
	return &ast.DictLiteral{
		KeyType:   keyType,
		ValueType: valueType,
		Entries:   entries,
		Line:      tok.Line,
	}, nil
}

func (p *Parser) parseParenOrLambda() (any, error) {
	saved := p.pos
	params, err := p.tryParseLambdaParams()
	if err == nil && p.at(token.FatArrow) {
		arrow := p.advance()
		p.skipNewlines()
		if isLambdaExprStart(p.peek().Type) {
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &ast.Lambda{
				Params: params,
				Body:   []any{&ast.ReturnStmt{Value: expr, Line: lineOf(expr)}},
				Line:   arrow.Line,
			}, nil
		}
		body, err := p.parseBlockUntil(token.EndFunc)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.EndFunc); err != nil {
			return nil, err
		}
		return &ast.Lambda{Params: params, Body: body, Line: arrow.Line}, nil
	}

	p.pos = saved
	p.advance()
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	return expr, nil
}

func (p *Parser) tryParseLambdaParams() ([]*ast.Param, error) {
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	var params []*ast.Param
	if !p.at(token.RParen) {
		param, err := p.parseParam()
		if err != nil {
			return nil, err
		}
		params = append(params, param)
		for p.at(token.Comma) {
			p.advance()
			param, err := p.parseParam()
			if err != nil {
				return nil, err
			}
			params = append(params, param)
		}
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	return params, nil
}

func (p *Parser) parseObjectLiteral() (any, error) {
	tok := p.advance()
	name := tok.Value
	if _, err := p.expect(token.LBrace); err != nil {
		return nil, err
	}

	var fields []ast.ObjectField
	p.skipNewlines()
	if !p.at(token.RBrace) {
		fieldName, err := p.expect(token.Identifier)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Assign); err != nil {
			return nil, err
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		fields = append(fields, ast.ObjectField{Name: fieldName.Value, Value: value})

		for {
			p.skipNewlines()
			if p.at(token.RBrace) {
				break
			}
			if !p.at(token.Comma) {
				break
			}
			p.advance()
			p.skipNewlines()
			if p.at(token.RBrace) {
				break
			}
			fieldName, err := p.expect(token.Identifier)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.Assign); err != nil {
				return nil, err
			}
			value, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			fields = append(fields, ast.ObjectField{Name: fieldName.Value, Value: value})
		}
	}

	if _, err := p.expect(token.RBrace); err != nil {
		return nil, err
	}
	return &ast.ObjectLiteral{Name: name, Fields: fields, Line: tok.Line}, nil
}

func (p *Parser) parseFuncDef(exported bool) (any, error) {
	tok, err := p.expect(token.Func)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}

	var params []*ast.Param
	if !p.at(token.RParen) {
		param, err := p.parseParam()
		if err != nil {
			return nil, err
		}
		params = append(params, param)
		for p.at(token.Comma) {
			p.advance()
			param, err := p.parseParam()
			if err != nil {
				return nil, err
			}
			params = append(params, param)
		}
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}

	var returnType any
	if p.at(token.Arrow) {
		p.advance()
		returnType, err = p.parseReturnTypeAnnotation()
		if err != nil {
			return nil, err
		}
	}

	body, err := p.parseBlockUntil(token.EndFunc)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.EndFunc); err != nil {
		return nil, err
	}
	if p.at(token.Identifier) {
		p.advance()
	}
	return &ast.FuncDef{
		Name:       nameTok.Value,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		Exported:   exported,
		Line:       tok.Line,
	}, nil
}

func (p *Parser) parseType() (any, error) {
	line := p.peek().Line

	if p.at(token.LParen) {
		p.advance()
		var paramTypes []any
		if !p.at(token.RParen) {
			paramType, err := p.parseType()
			if err != nil {
				return nil, err
			}
			paramTypes = append(paramTypes, paramType)
			for p.at(token.Comma) {
				p.advance()
				paramType, err := p.parseType()
				if err != nil {
					return nil, err
				}
				paramTypes = append(paramTypes, paramType)
			}
		}
		if _, err := p.expect(token.RParen); err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Arrow); err != nil {
			return nil, err
		}
		returnType, err := p.parseReturnTypeAnnotation()
		if err != nil {
			return nil, err
		}
		return &ast.FuncType{ParamTypes: paramTypes, ReturnType: returnType, Line: line}, nil
	}

	base, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	if p.at(token.LBracket) {
		p.advance()
		paramType, err := p.parseType()
		if err != nil {
			return nil, err
		}
		params := []any{paramType}
		for p.at(token.Comma) {
			p.advance()
			paramType, err := p.parseType()
			if err != nil {
				return nil, err
			}
			params = append(params, paramType)
		}
		if _, err := p.expect(token.RBracket); err != nil {
			return nil, err
		}
		return &ast.GenericType{Base: base.Value, Params: params, Line: line}, nil
	}
	return &ast.TypeName{Name: base.Value, Line: line}, nil
}

func (p *Parser) parseParam() (*ast.Param, error) {
	var nameTok token.Token
	var err error
	switch {
	case p.at(token.Identifier), p.at(token.Index):
		nameTok = p.advance()
	default:
		return nil, p.errorf(p.peek(), "expected parameter name")
	}

	var typeName any
	var defaultValue any
	if p.at(token.Colon) {
		p.advance()
		typeName, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}
	if p.at(token.Eq) {
		p.advance()
		defaultValue, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}
	return &ast.Param{Name: nameTok.Value, TypeName: typeName, Default: defaultValue, Line: nameTok.Line}, nil
}

func (p *Parser) parseIf() (any, error) {
	tok, err := p.expect(token.If)
	if err != nil {
		return nil, err
	}
	condition, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Then); err != nil {
		return nil, err
	}
	body, err := p.parseBlockUntil(token.Elif, token.Else, token.EndIf)
	if err != nil {
		return nil, err
	}

	var elifs []ast.IfBranch
	for p.at(token.Elif) {
		p.advance()
		elifCondition, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Then); err != nil {
			return nil, err
		}
		elifBody, err := p.parseBlockUntil(token.Elif, token.Else, token.EndIf)
		if err != nil {
			return nil, err
		}
		elifs = append(elifs, ast.IfBranch{Condition: elifCondition, Body: elifBody})
	}

	var elseBody []any
	if p.at(token.Else) {
		p.advance()
		elseBody, err = p.parseBlockUntil(token.EndIf)
		if err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(token.EndIf); err != nil {
		return nil, err
	}
	return &ast.IfStmt{
		Condition: condition,
		Body:      body,
		Elifs:     elifs,
		ElseBody:  elseBody,
		Line:      tok.Line,
	}, nil
}

func (p *Parser) parseWhile() (any, error) {
	tok, err := p.expect(token.While)
	if err != nil {
		return nil, err
	}
	condition, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Do); err != nil {
		return nil, err
	}
	name := p.parseOptionalLineLabel()
	body, err := p.parseBlockUntil(token.EndWhile)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.EndWhile); err != nil {
		return nil, err
	}
	endName, err := p.parseOptionalLoopEndName()
	if err != nil {
		return nil, err
	}
	if name != "" && endName != "" && name != endName {
		return nil, p.errorf(p.peek(), "loop name mismatch: started %q, ended %q", name, endName)
	}
	return &ast.WhileStmt{Condition: condition, Name: firstNonEmpty(name, endName), Body: body, Line: tok.Line}, nil
}

func (p *Parser) parseFor() (any, error) {
	tok, err := p.expect(token.For)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.In); err != nil {
		return nil, err
	}

	start, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	if p.at(token.To) {
		p.advance()
		end, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		var step any
		direction := "auto"
		if p.at(token.Order) {
			p.advance()
			direction = "asc"
		} else if p.at(token.Reverse) {
			p.advance()
			direction = "desc"
		}
		if p.at(token.Step) {
			p.advance()
			step, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
		if _, err := p.expect(token.Do); err != nil {
			return nil, err
		}
		loopName := p.parseOptionalLineLabel()
		body, err := p.parseBlockUntil(token.EndFor)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.EndFor); err != nil {
			return nil, err
		}
		endName, err := p.parseOptionalLoopEndName()
		if err != nil {
			return nil, err
		}
		if loopName != "" && endName != "" && loopName != endName {
			return nil, p.errorf(p.peek(), "loop name mismatch: started %q, ended %q", loopName, endName)
		}
		return &ast.ForRangeStmt{
			Var:       nameTok.Value,
			Start:     start,
			End:       end,
			Step:      step,
			Direction: direction,
			Name:      firstNonEmpty(loopName, endName),
			Body:      body,
			Line:      tok.Line,
		}, nil
	}

	indexVar := ""
	if !p.at(token.With) {
		if _, err := p.expect(token.Do); err != nil {
			return nil, err
		}
	}
	if p.at(token.With) {
		p.advance()
		if _, err := p.expect(token.Index); err != nil {
			return nil, err
		}
		indexTok, err := p.expect(token.Identifier)
		if err != nil {
			return nil, err
		}
		indexVar = indexTok.Value
		if _, err := p.expect(token.Do); err != nil {
			return nil, err
		}
	}
	loopName := p.parseOptionalLineLabel()
	body, err := p.parseBlockUntil(token.EndFor)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.EndFor); err != nil {
		return nil, err
	}
	endName, err := p.parseOptionalLoopEndName()
	if err != nil {
		return nil, err
	}
	if loopName != "" && endName != "" && loopName != endName {
		return nil, p.errorf(p.peek(), "loop name mismatch: started %q, ended %q", loopName, endName)
	}
	return &ast.ForEachStmt{
		Var:      nameTok.Value,
		Iterable: start,
		IndexVar: indexVar,
		Name:     firstNonEmpty(loopName, endName),
		Body:     body,
		Line:     tok.Line,
	}, nil
}

func (p *Parser) parseMatch() (any, error) {
	tok, err := p.expect(token.Match)
	if err != nil {
		return nil, err
	}
	subject, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()

	var cases []*ast.WhenClause
	var elseBody []any
	for p.at(token.When) {
		whenTok := p.advance()
		pattern, err := p.parseMatchPattern()
		if err != nil {
			return nil, err
		}
		patterns := []any{pattern}
		for p.at(token.Comma) {
			p.advance()
			pattern, err := p.parseMatchPattern()
			if err != nil {
				return nil, err
			}
			patterns = append(patterns, pattern)
		}
		if _, err := p.expect(token.FatArrow); err != nil {
			return nil, err
		}
		body, err := p.parseBlockUntil(token.When, token.Else, token.EndMatch)
		if err != nil {
			return nil, err
		}
		cases = append(cases, &ast.WhenClause{Patterns: patterns, Body: body, Line: whenTok.Line})
	}

	if p.at(token.Else) {
		p.advance()
		elseBody, err = p.parseBlockUntil(token.EndMatch)
		if err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(token.EndMatch); err != nil {
		return nil, err
	}
	return &ast.MatchStmt{Subject: subject, Cases: cases, ElseBody: elseBody, Line: tok.Line}, nil
}

func (p *Parser) parseMatchPattern() (any, error) {
	if p.at(token.Ok) {
		tok := p.advance()
		if _, err := p.expect(token.LParen); err != nil {
			return nil, err
		}
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.RParen); err != nil {
			return nil, err
		}
		return &ast.OkExpr{Value: inner, Line: tok.Line}, nil
	}
	if p.at(token.Err) {
		tok := p.advance()
		if _, err := p.expect(token.LParen); err != nil {
			return nil, err
		}
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.RParen); err != nil {
			return nil, err
		}
		return &ast.ErrExpr{Value: inner, Line: tok.Line}, nil
	}

	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.at(token.To) {
		tok := p.advance()
		end, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.BinaryOp{Left: expr, Op: tok.Value, Right: end, Line: lineOf(expr)}, nil
	}
	return expr, nil
}

func (p *Parser) parseModule() (any, error) {
	tok, err := p.expect(token.Module)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlockUntil(token.EndModule)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.EndModule); err != nil {
		return nil, err
	}
	return &ast.ModuleDef{Name: nameTok.Value, Body: body, Line: tok.Line}, nil
}

func (p *Parser) parseUse() (any, error) {
	tok, err := p.expect(token.Use)
	if err != nil {
		return nil, err
	}
	first, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}

	if p.at(token.Comma, token.From) {
		names := []string{first.Value}
		for p.at(token.Comma) {
			p.advance()
			nameTok, err := p.expect(token.Identifier)
			if err != nil {
				return nil, err
			}
			names = append(names, nameTok.Value)
		}
		if _, err := p.expect(token.From); err != nil {
			return nil, err
		}
		moduleTok, err := p.expect(token.Identifier)
		if err != nil {
			return nil, err
		}
		return &ast.UseStmt{Module: moduleTok.Value, Names: names, Line: tok.Line}, nil
	}

	return &ast.UseStmt{Module: first.Value, Names: nil, Line: tok.Line}, nil
}

func (p *Parser) parseReturn() (any, error) {
	tok, err := p.expect(token.Return)
	if err != nil {
		return nil, err
	}
	var values []any
	if !p.at(token.Newline, token.EOF, token.EndFunc, token.EndIf, token.EndWhile, token.EndFor, token.EndMatch, token.EndModule, token.EndParallel, token.EndArena, token.EndNew, token.EndObject) {
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		for p.at(token.Comma) {
			p.advance()
			value, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
	}

	var value any
	switch len(values) {
	case 0:
		value = nil
	case 1:
		value = values[0]
	default:
		value = values
	}
	return &ast.ReturnStmt{Value: value, Line: tok.Line}, nil
}

func (p *Parser) parsePass() (any, error) {
	tok := p.peek()
	if tok.Type == token.Pass {
		p.advance()
	} else if tok.Type == token.Identifier && tok.Value == "pass" {
		p.advance()
	} else {
		return nil, p.errorf(tok, "expected 'pass'")
	}
	return &ast.PassStmt{Line: tok.Line}, nil
}

func (p *Parser) parseLeave() (any, error) {
	tok := p.peek()
	if tok.Type == token.Leave {
		p.advance()
	} else if tok.Type == token.Identifier && tok.Value == "leave" {
		p.advance()
	} else {
		return nil, p.errorf(tok, "expected 'leave'")
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	return &ast.LeaveStmt{Name: nameTok.Value, Line: tok.Line}, nil
}

func (p *Parser) parseNext() (any, error) {
	tok := p.peek()
	if tok.Type == token.Next {
		p.advance()
	} else if tok.Type == token.Identifier && tok.Value == "next" {
		p.advance()
	} else {
		return nil, p.errorf(tok, "expected 'next'")
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	return &ast.NextStmt{Name: nameTok.Value, Line: tok.Line}, nil
}

func (p *Parser) parseGlobal() (any, error) {
	tok, err := p.expect(token.Global)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Assign); err != nil {
		return nil, err
	}
	value, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.GlobalStmt{Name: nameTok.Value, Value: value, Line: tok.Line}, nil
}

func (p *Parser) parseConst() (any, error) {
	tok, err := p.expect(token.Const)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	var typeNode any
	if p.at(token.Colon) {
		p.advance()
		typeNode, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(token.Assign); err != nil {
		return nil, err
	}
	value, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.VarDecl{
		Name:     nameTok.Value,
		TypeName: typeNode,
		Value:    value,
		IsConst:  true,
		Line:     tok.Line,
	}, nil
}

func (p *Parser) parseTypeAlias(exported bool) (any, error) {
	tok := p.advance()
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Eq); err != nil {
		return nil, err
	}
	target, err := p.parseType()
	if err != nil {
		return nil, err
	}
	return &ast.TypeAlias{Name: nameTok.Value, Target: target, Exported: exported, Line: tok.Line}, nil
}

func (p *Parser) parseVarBlock() (any, error) {
	tok, err := p.expect(token.Var)
	if err != nil {
		return nil, err
	}
	defaultMode := "none"
	var defaultValue any
	if p.at(token.Default) {
		p.advance()
		defaultMode = "zero"
		if !p.at(token.Newline) {
			defaultMode = "value"
			defaultValue, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
	}
	p.skipNewlines()

	var decls []*ast.VarDecl
	for !p.at(token.EndVar) {
		if p.at(token.EOF) {
			return nil, p.errorf(p.peek(), "unexpected EOF in var block")
		}
		nameTok, err := p.expect(token.Identifier)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Colon); err != nil {
			return nil, err
		}
		typeNode, err := p.parseType()
		if err != nil {
			return nil, err
		}
		var value any
		isUninit := true
		if p.at(token.Assign) {
			p.advance()
			value, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
			isUninit = false
		}
		decls = append(decls, &ast.VarDecl{
			Name:     nameTok.Value,
			TypeName: typeNode,
			Value:    value,
			IsUninit: isUninit,
			Line:     nameTok.Line,
		})
		p.skipNewlines()
	}
	if _, err := p.expect(token.EndVar); err != nil {
		return nil, err
	}
	return &ast.VarBlock{
		Decls:        decls,
		DefaultMode:  defaultMode,
		DefaultValue: defaultValue,
		Line:         tok.Line,
	}, nil
}

func (p *Parser) parseParallel() (any, error) {
	tok, err := p.expect(token.Parallel)
	if err != nil {
		return nil, err
	}
	allowFail := false
	resultVar := ""
	if p.at(token.AllowFail) {
		p.advance()
		allowFail = true
	}
	if p.at(token.FatArrow) {
		p.advance()
		resultTok, err := p.expect(token.Identifier)
		if err != nil {
			return nil, err
		}
		resultVar = resultTok.Value
	}
	if _, err := p.expect(token.Do); err != nil {
		return nil, err
	}
	body, err := p.parseBlockUntil(token.EndParallel)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.EndParallel); err != nil {
		return nil, err
	}
	return &ast.ParallelStmt{Body: body, ResultVar: resultVar, AllowFail: allowFail, Line: tok.Line}, nil
}

func (p *Parser) parseArena() (any, error) {
	tok, err := p.expect(token.Arena)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Do); err != nil {
		return nil, err
	}
	body, err := p.parseBlockUntil(token.EndArena)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.EndArena); err != nil {
		return nil, err
	}
	return &ast.ArenaStmt{Name: nameTok.Value, Body: body, Line: tok.Line}, nil
}

func (p *Parser) parseObjectDef(exported bool) (any, error) {
	tok, err := p.expect(token.Object)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}

	var fields []*ast.FieldDef
	var constructor *ast.ConstructorDef
	var methods []*ast.MethodDef

	for !p.at(token.EndObject) {
		for p.at(token.Newline) {
			p.advance()
		}
		if p.at(token.EndObject) {
			break
		}
		switch {
		case p.at(token.Identifier):
			fieldName, err := p.expect(token.Identifier)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.Colon); err != nil {
				return nil, err
			}
			typeNode, err := p.parseType()
			if err != nil {
				return nil, err
			}
			fields = append(fields, &ast.FieldDef{
				Name:           fieldName.Value,
				TypeAnnotation: typeNode,
				Line:           fieldName.Line,
			})
			if p.at(token.Newline) {
				p.advance()
			}
		case p.at(token.Func):
			methodNode, err := p.parseMethodDef()
			if err != nil {
				return nil, err
			}
			methods = append(methods, methodNode)
		case p.at(token.New):
			constructor, err = p.parseConstructorDef(nameTok.Value)
			if err != nil {
				return nil, err
			}
		default:
			return nil, p.errorf(p.peek(), "unexpected token in object definition: %s", p.peek().String())
		}
	}
	if _, err := p.expect(token.EndObject); err != nil {
		return nil, err
	}
	return &ast.ObjectDef{
		Name:        nameTok.Value,
		Fields:      fields,
		Constructor: constructor,
		Methods:     methods,
		Exported:    exported,
		Line:        tok.Line,
	}, nil
}

func (p *Parser) parseConstructorDef(objectName string) (*ast.ConstructorDef, error) {
	tok, err := p.expect(token.New)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	var params []*ast.Param
	for !p.at(token.RParen) {
		param, err := p.parseParam()
		if err != nil {
			return nil, err
		}
		params = append(params, param)
		if p.at(token.Comma) {
			p.advance()
		}
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	var returnType any
	if p.at(token.Arrow) {
		p.advance()
		returnType, err = p.parseReturnTypeAnnotation()
		if err != nil {
			return nil, err
		}
	}
	body, err := p.parseBlockUntil(token.EndNew)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.EndNew); err != nil {
		return nil, err
	}
	return &ast.ConstructorDef{
		Name:       objectName,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		Line:       tok.Line,
	}, nil
}

func (p *Parser) parseMethodDef() (*ast.MethodDef, error) {
	tok, err := p.expect(token.Func)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expect(token.Identifier)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	var params []*ast.Param
	for !p.at(token.RParen) {
		param, err := p.parseParam()
		if err != nil {
			return nil, err
		}
		params = append(params, param)
		if p.at(token.Comma) {
			p.advance()
		}
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	var returnType any
	if p.at(token.Arrow) {
		p.advance()
		returnType, err = p.parseReturnTypeAnnotation()
		if err != nil {
			return nil, err
		}
	}
	body, err := p.parseBlockUntil(token.EndFunc)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.EndFunc); err != nil {
		return nil, err
	}
	return &ast.MethodDef{
		Name:       nameTok.Value,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		Line:       tok.Line,
	}, nil
}

func (p *Parser) parseReturnTypeAnnotation() (any, error) {
	typeNode, err := p.parseType()
	if err != nil {
		return nil, err
	}
	typeNodes := []any{typeNode}
	for p.at(token.Comma) {
		if p.pos+2 < len(p.tokens) && p.tokens[p.pos+1].Type == token.Identifier && p.tokens[p.pos+2].Type == token.Colon {
			break
		}
		p.advance()
		typeNode, err := p.parseType()
		if err != nil {
			return nil, err
		}
		typeNodes = append(typeNodes, typeNode)
	}
	if len(typeNodes) == 1 {
		return typeNodes[0], nil
	}
	return typeNodes, nil
}

func (p *Parser) peek() token.Token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekNext() *token.Token {
	if p.pos+1 >= len(p.tokens) {
		return nil
	}
	return &p.tokens[p.pos+1]
}

func (p *Parser) at(types ...token.Type) bool {
	current := p.peek().Type
	for _, tokenType := range types {
		if current == tokenType {
			return true
		}
	}
	return false
}

func (p *Parser) advance() token.Token {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *Parser) expect(tokenType token.Type) (token.Token, error) {
	tok := p.peek()
	if tok.Type != tokenType {
		return token.Token{}, p.errorf(tok, "expected %s, got %s (%q)", tokenType, tok.Type, tok.Value)
	}
	return p.advance(), nil
}

func (p *Parser) skipNewlines() {
	for p.at(token.Newline) {
		p.advance()
	}
}

func (p *Parser) parseOptionalLineLabel() string {
	if !p.at(token.Identifier) {
		return ""
	}
	next := p.peekNext()
	if next == nil {
		return ""
	}
	if next.Type != token.Newline && next.Type != token.EOF {
		return ""
	}
	return p.advance().Value
}

func (p *Parser) parseOptionalLoopEndName() (string, error) {
	if !p.at(token.Identifier) {
		return "", nil
	}
	next := p.peekNext()
	if next != nil && next.Type != token.Newline && next.Type != token.EOF {
		return "", p.errorf(p.peek(), "expected newline after loop name")
	}
	return p.advance().Value, nil
}

func (p *Parser) errorf(tok token.Token, format string, args ...any) error {
	return &Error{
		Message: fmt.Sprintf(format, args...),
		Token:   tok,
	}
}

func assignmentTarget(expr any) (any, bool) {
	switch node := expr.(type) {
	case *ast.Identifier:
		return node.Name, true
	case *ast.IndexAccess:
		return node, true
	case *ast.MemberAccess:
		return node, true
	default:
		return nil, false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func lineOf(node any) int {
	switch n := node.(type) {
	case *ast.IntLiteral:
		return n.Line
	case *ast.FloatLiteral:
		return n.Line
	case *ast.StringLiteral:
		return n.Line
	case *ast.BoolLiteral:
		return n.Line
	case *ast.Identifier:
		return n.Line
	case *ast.BinaryOp:
		return n.Line
	case *ast.UnaryOp:
		return n.Line
	case *ast.FuncCall:
		return n.Line
	case *ast.MemberAccess:
		return n.Line
	case *ast.IndexAccess:
		return n.Line
	case *ast.Lambda:
		return n.Line
	case *ast.OkExpr:
		return n.Line
	case *ast.ErrExpr:
		return n.Line
	case *ast.ListLiteral:
		return n.Line
	case *ast.DictLiteral:
		return n.Line
	case *ast.AsExpr:
		return n.Line
	case *ast.ObjectLiteral:
		return n.Line
	default:
		return 0
	}
}

func isLambdaExprStart(tokenType token.Type) bool {
	switch tokenType {
	case token.Identifier, token.Integer, token.Float, token.String, token.True, token.False, token.LParen, token.Minus, token.Not, token.Ok, token.Err:
		return true
	default:
		return false
	}
}

func joinComma(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ","
		}
		result += part
	}
	return result
}
