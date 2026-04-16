"""Gwen parser - turns token stream into AST."""

from typing import List, Optional, Any
from .lexer import Token, TokenType, tokenize
from . import ast_nodes as ast


class ParseError(Exception):
    def __init__(self, message: str, token: Token):
        super().__init__(f"Parse error at L{token.line}:{token.column}: {message}")
        self.token = token


class Parser:
    def __init__(self, tokens: List[Token]):
        self.tokens = tokens
        self.pos = 0

    def peek(self) -> Token:
        return self.tokens[self.pos]

    def at(self, *types: TokenType) -> bool:
        return self.peek().type in types

    def advance(self) -> Token:
        tok = self.tokens[self.pos]
        self.pos += 1
        return tok

    def expect(self, token_type: TokenType) -> Token:
        tok = self.peek()
        if tok.type != token_type:
            raise ParseError(f"Expected {token_type.name}, got {tok.type.name} ({tok.value!r})", tok)
        return self.advance()

    def skip_newlines(self):
        while self.at(TokenType.NEWLINE):
            self.advance()

    def parse(self) -> ast.Program:
        stmts = self.parse_block_until(TokenType.EOF)
        return ast.Program(statements=stmts)

    def parse_block_until(self, *end_types: TokenType) -> List[Any]:
        stmts = []
        self.skip_newlines()
        while not self.at(*end_types):
            stmt = self.parse_statement()
            if stmt is not None:
                stmts.append(stmt)
            self.skip_newlines()
        return stmts

    def parse_statement(self) -> Any:
        tok = self.peek()

        if tok.type == TokenType.FUNC:
            return self.parse_func_def(exported=False)
        if tok.type == TokenType.EXPORT:
            self.advance()
            return self.parse_func_def(exported=True)
        if tok.type == TokenType.IF:
            return self.parse_if()
        if tok.type == TokenType.WHILE:
            return self.parse_while()
        if tok.type == TokenType.FOR:
            return self.parse_for()
        if tok.type == TokenType.MATCH:
            return self.parse_match()
        if tok.type == TokenType.MODULE:
            return self.parse_module()
        if tok.type == TokenType.USE:
            return self.parse_use()
        if tok.type == TokenType.RETURN:
            return self.parse_return()
        if tok.type == TokenType.PARALLEL:
            return self.parse_parallel()
        if tok.type == TokenType.TAG:
            name = tok.value
            self.advance()
            return ast.TagStmt(name=name, line=tok.line)
        if tok.type == TokenType.NEWLINE:
            self.advance()
            return None

        return self.parse_assignment_or_expr()

    def parse_assignment_or_expr(self) -> Any:
        """Parse assignment (:=) or expression statement."""
        tok = self.peek()
        expr = self.parse_expr()

        # Check for := assignment
        if self.at(TokenType.ASSIGN):
            self.advance()
            if isinstance(expr, ast.Identifier):
                targets = [expr.name]
            elif isinstance(expr, ast.IndexAccess):
                targets = [expr]
            else:
                raise ParseError("Expected identifier or index access in assignment", self.peek())
            values = [self.parse_expr()]
            return ast.Assignment(targets=targets, values=values, line=tok.line)

        # Check for multi-target: a, b := x, y
        if self.at(TokenType.COMMA) and isinstance(expr, ast.Identifier):
            targets = [expr.name]
            while self.at(TokenType.COMMA):
                self.advance()
                name_tok = self.expect(TokenType.IDENTIFIER)
                targets.append(name_tok.value)
            self.expect(TokenType.ASSIGN)
            values = [self.parse_expr()]
            while self.at(TokenType.COMMA):
                self.advance()
                values.append(self.parse_expr())
            return ast.Assignment(targets=targets, values=values, line=tok.line)

        # Check for typed var decl: x: int := 42
        if self.at(TokenType.COLON) and isinstance(expr, ast.Identifier):
            self.advance()
            type_node = self.parse_type()
            value = None
            if self.at(TokenType.ASSIGN):
                self.advance()
                value = self.parse_expr()
            return ast.VarDecl(name=expr.name, type_name=type_node, value=value, line=tok.line)

        return ast.ExprStmt(expr=expr, line=tok.line)

    def _expr_to_name(self, expr) -> str:
        if isinstance(expr, ast.Identifier):
            return expr.name
        raise ParseError("Expected identifier in assignment", self.peek())

    # --- Expressions (precedence climbing) ---

    def parse_expr(self) -> Any:
        return self.parse_or()

    def parse_or(self) -> Any:
        left = self.parse_and()
        while self.at(TokenType.OR):
            self.advance()
            right = self.parse_and()
            left = ast.BinaryOp(left, "or", right, line=left.line)
        return left

    def parse_and(self) -> Any:
        left = self.parse_comparison()
        while self.at(TokenType.AND):
            self.advance()
            right = self.parse_comparison()
            left = ast.BinaryOp(left, "and", right, line=left.line)
        return left

    def parse_comparison(self) -> Any:
        left = self.parse_addition()
        while self.at(TokenType.EQ, TokenType.NEQ, TokenType.LT, TokenType.GT, TokenType.LTE, TokenType.GTE):
            op = self.advance().value
            right = self.parse_addition()
            left = ast.BinaryOp(left, op, right, line=left.line)
        return left

    def parse_addition(self) -> Any:
        left = self.parse_multiplication()
        while self.at(TokenType.PLUS, TokenType.MINUS):
            op = self.advance().value
            right = self.parse_multiplication()
            left = ast.BinaryOp(left, op, right, line=left.line)
        return left

    def parse_multiplication(self) -> Any:
        left = self.parse_power()
        while self.at(TokenType.STAR, TokenType.SLASH, TokenType.MOD):
            op = self.advance().value
            right = self.parse_power()
            left = ast.BinaryOp(left, op, right, line=left.line)
        return left

    def parse_power(self) -> Any:
        # Right-associative: 2^3^2 = 2^(3^2)
        base = self.parse_unary()
        if self.at(TokenType.CARET):
            self.advance()
            exponent = self.parse_power()  # Right recursion
            return ast.BinaryOp(base, "^", exponent, line=base.line)
        return base

    def parse_unary(self) -> Any:
        if self.at(TokenType.MINUS):
            tok = self.advance()
            operand = self.parse_unary()
            return ast.UnaryOp("-", operand, line=tok.line)
        if self.at(TokenType.NOT):
            tok = self.advance()
            operand = self.parse_unary()
            return ast.UnaryOp("not", operand, line=tok.line)
        expr = self.parse_postfix()
        while self.at(TokenType.AS):
            self.advance()
            type_tok = self.expect(TokenType.IDENTIFIER)
            expr = ast.AsExpr(expr, type_tok.value, line=expr.line)
        return expr

    def parse_postfix(self) -> Any:
        expr = self.parse_primary()
        while True:
            if self.at(TokenType.LPAREN):
                expr = self.parse_call(expr)
            elif self.at(TokenType.DOT):
                self.advance()
                member = self.expect(TokenType.IDENTIFIER)
                expr = ast.MemberAccess(expr, member.value, line=expr.line)
            elif self.at(TokenType.LBRACKET):
                self.advance()
                index = self.parse_expr()
                self.expect(TokenType.RBRACKET)
                expr = ast.IndexAccess(expr, index, line=expr.line)
            else:
                break
        return expr

    def parse_call(self, callee: Any) -> ast.FuncCall:
        self.expect(TokenType.LPAREN)
        args = []
        if not self.at(TokenType.RPAREN):
            args.append(self.parse_expr())
            while self.at(TokenType.COMMA):
                self.advance()
                args.append(self.parse_expr())
        self.expect(TokenType.RPAREN)
        return ast.FuncCall(name=callee, args=args, line=callee.line)

    def parse_primary(self) -> Any:
        tok = self.peek()

        if tok.type == TokenType.INTEGER:
            self.advance()
            return ast.IntLiteral(int(tok.value), line=tok.line)

        if tok.type == TokenType.FLOAT:
            self.advance()
            return ast.FloatLiteral(float(tok.value), line=tok.line)

        if tok.type == TokenType.STRING:
            self.advance()
            return ast.StringLiteral(tok.value, line=tok.line)

        if tok.type == TokenType.TRUE:
            self.advance()
            return ast.BoolLiteral(True, line=tok.line)

        if tok.type == TokenType.FALSE:
            self.advance()
            return ast.BoolLiteral(False, line=tok.line)

        if tok.type == TokenType.OK:
            self.advance()
            self.expect(TokenType.LPAREN)
            value = self.parse_expr()
            self.expect(TokenType.RPAREN)
            return ast.OkExpr(value, line=tok.line)

        if tok.type == TokenType.ERR:
            self.advance()
            self.expect(TokenType.LPAREN)
            value = self.parse_expr()
            self.expect(TokenType.RPAREN)
            return ast.ErrExpr(value, line=tok.line)

        if tok.type in (TokenType.IDENTIFIER, TokenType.INDEX):
            self.advance()
            return ast.Identifier(tok.value, line=tok.line)

        if tok.type == TokenType.LBRACKET:
            return self.parse_list_literal()

        if tok.type == TokenType.LPAREN:
            return self.parse_paren_or_lambda()

        raise ParseError(f"Unexpected token: {tok.type.name} ({tok.value!r})", tok)

    def parse_list_literal(self) -> ast.ListLiteral:
        tok = self.advance()  # [
        elements = []
        if not self.at(TokenType.RBRACKET):
            elements.append(self.parse_expr())
            while self.at(TokenType.COMMA):
                self.advance()
                elements.append(self.parse_expr())
        self.expect(TokenType.RBRACKET)
        return ast.ListLiteral(elements, line=tok.line)

    def parse_paren_or_lambda(self) -> Any:
        """Parse (expr) or (params) => body."""
        # Try to detect lambda: look ahead for =>
        saved = self.pos
        try:
            params = self._try_parse_lambda_params()
            if params is not None and self.at(TokenType.FAT_ARROW):
                self.advance()  # =>
                self.skip_newlines()
                body = []
                if self.at(TokenType.IDENTIFIER, TokenType.INTEGER, TokenType.FLOAT,
                           TokenType.STRING, TokenType.TRUE, TokenType.FALSE,
                           TokenType.LPAREN, TokenType.MINUS, TokenType.NOT,
                           TokenType.OK, TokenType.ERR):
                    # single expression lambda
                    expr = self.parse_expr()
                    body = [ast.ReturnStmt(value=expr, line=expr.line)]
                else:
                    body = self.parse_block_until(TokenType.IDENTIFIER)
                    # expect 'end'
                    end_tok = self.peek()
                    if end_tok.value == "end":
                        self.advance()
                    else:
                        raise ParseError("Expected 'end' for lambda", end_tok)
                return ast.Lambda(params=params, body=body, line=params[0].line if params else self.peek().line)
        except (ParseError, IndexError):
            pass

        # Not a lambda, restore and parse as grouped expression
        self.pos = saved
        self.advance()  # (
        expr = self.parse_expr()
        self.expect(TokenType.RPAREN)
        return expr

    def _try_parse_lambda_params(self) -> Optional[List[ast.Param]]:
        self.expect(TokenType.LPAREN)
        params = []
        if not self.at(TokenType.RPAREN):
            params.append(self._parse_param())
            while self.at(TokenType.COMMA):
                self.advance()
                params.append(self._parse_param())
        self.expect(TokenType.RPAREN)
        return params

    # --- Statements ---

    def parse_func_def(self, exported: bool = False) -> ast.FuncDef:
        tok = self.expect(TokenType.FUNC)
        name = self.expect(TokenType.IDENTIFIER).value
        self.expect(TokenType.LPAREN)
        params = []
        if not self.at(TokenType.RPAREN):
            params.append(self._parse_param())
            while self.at(TokenType.COMMA):
                self.advance()
                params.append(self._parse_param())
        self.expect(TokenType.RPAREN)

        return_type = None
        if self.at(TokenType.ARROW):
            self.advance()
            return_type = self.parse_type()

        body = self.parse_block_until(TokenType.ENDFUNC)
        self.expect(TokenType.ENDFUNC)
        # optional name after endfunc
        if self.at(TokenType.IDENTIFIER):
            self.advance()

        return ast.FuncDef(name=name, params=params, return_type=return_type,
                           body=body, exported=exported, line=tok.line)

    def parse_type(self) -> Any:
        line, col = self.peek().line, self.peek().column

        # Function type: (int, int) -> int
        if self.at(TokenType.LPAREN):
            self.advance()
            param_types = []
            if not self.at(TokenType.RPAREN):
                param_types.append(self.parse_type())
                while self.at(TokenType.COMMA):
                    self.advance()
                    param_types.append(self.parse_type())
            self.expect(TokenType.RPAREN)
            self.expect(TokenType.ARROW)
            return_type = self.parse_type()
            return ast.FuncType(param_types=param_types, return_type=return_type, line=line)

        # Base type or generic type
        base = self.expect(TokenType.IDENTIFIER).value
        if self.at(TokenType.LT):
            self.advance()
            params = [self.parse_type()]
            while self.at(TokenType.COMMA):
                self.advance()
                params.append(self.parse_type())
            self.expect(TokenType.GT)
            return ast.GenericType(base=base, params=params, line=line)
        return ast.TypeName(name=base, line=line)

    def _parse_param(self) -> ast.Param:
        if self.at(TokenType.IDENTIFIER, TokenType.INDEX):
            name_tok = self.advance()
        else:
            raise ParseError("Expected parameter name", self.peek())
        type_name = None
        default = None
        if self.at(TokenType.COLON):
            self.advance()
            type_name = self.parse_type()
        if self.at(TokenType.EQ):
            self.advance()
            default = self.parse_expr()
        return ast.Param(name=name_tok.value, type_name=type_name, default=default, line=name_tok.line)

    def parse_if(self) -> ast.IfStmt:
        tok = self.expect(TokenType.IF)
        condition = self.parse_expr()
        self.expect(TokenType.THEN)
        body = self.parse_block_until(TokenType.ELIF, TokenType.ELSE, TokenType.ENDIF)

        elifs = []
        while self.at(TokenType.ELIF):
            self.advance()
            elif_cond = self.parse_expr()
            self.expect(TokenType.THEN)
            elif_body = self.parse_block_until(TokenType.ELIF, TokenType.ELSE, TokenType.ENDIF)
            elifs.append((elif_cond, elif_body))

        else_body = []
        if self.at(TokenType.ELSE):
            self.advance()
            else_body = self.parse_block_until(TokenType.ENDIF)

        self.expect(TokenType.ENDIF)
        return ast.IfStmt(condition=condition, body=body, elifs=elifs,
                          else_body=else_body, line=tok.line)

    def parse_while(self) -> ast.WhileStmt:
        tok = self.expect(TokenType.WHILE)
        condition = self.parse_expr()
        self.expect(TokenType.DO)
        body = self.parse_block_until(TokenType.ENDWHILE)
        self.expect(TokenType.ENDWHILE)
        return ast.WhileStmt(condition=condition, body=body, line=tok.line)

    def parse_for(self) -> ast.ForRangeStmt:
        tok = self.expect(TokenType.FOR)
        var = self.expect(TokenType.IDENTIFIER).value
        self.expect(TokenType.IN)

        # Check if range (number/identifier followed by 'to') or collection
        start = self.parse_expr()

        if self.at(TokenType.TO):
            # Range for
            self.advance()
            end = self.parse_expr()
            step = None
            if self.at(TokenType.STEP):
                self.advance()
                step = self.parse_expr()
            self.expect(TokenType.DO)
            body = self.parse_block_until(TokenType.ENDFOR)
            self.expect(TokenType.ENDFOR)
            return ast.ForRangeStmt(var=var, start=start, end=end, step=step,
                                     body=body, line=tok.line)
        else:
            # For-each
            self.expect(TokenType.DO) if not self.at(TokenType.WITH) else None
            index_var = None
            if self.at(TokenType.WITH):
                self.advance()
                self.expect(TokenType.INDEX)
                index_var = self.expect(TokenType.IDENTIFIER).value
                self.expect(TokenType.DO)
            else:
                if not self.tokens[self.pos - 1].type == TokenType.DO:
                    self.expect(TokenType.DO)
            body = self.parse_block_until(TokenType.ENDFOR)
            self.expect(TokenType.ENDFOR)
            return ast.ForEachStmt(var=var, iterable=start, index_var=index_var,
                                    body=body, line=tok.line)

    def parse_match(self) -> ast.MatchStmt:
        tok = self.expect(TokenType.MATCH)
        subject = self.parse_expr()
        self.skip_newlines()

        cases = []
        else_body = []
        while self.at(TokenType.WHEN):
            self.advance()
            patterns = [self.parse_match_pattern()]
            while self.at(TokenType.COMMA):
                self.advance()
                patterns.append(self.parse_match_pattern())
            self.expect(TokenType.THEN)
            case_body = self.parse_block_until(TokenType.WHEN, TokenType.ELSE, TokenType.ENDMATCH)
            cases.append(ast.WhenClause(patterns=patterns, body=case_body))

        if self.at(TokenType.ELSE):
            self.advance()
            else_body = self.parse_block_until(TokenType.ENDMATCH)

        self.expect(TokenType.ENDMATCH)
        return ast.MatchStmt(subject=subject, cases=cases, else_body=else_body, line=tok.line)

    def parse_match_pattern(self) -> Any:
        """Parse a pattern: literal, ok(name), err(name), or range."""
        if self.at(TokenType.OK):
            tok = self.advance()
            self.expect(TokenType.LPAREN)
            inner = self.parse_expr()
            self.expect(TokenType.RPAREN)
            return ast.OkExpr(inner, line=tok.line)
        if self.at(TokenType.ERR):
            tok = self.advance()
            self.expect(TokenType.LPAREN)
            inner = self.parse_expr()
            self.expect(TokenType.RPAREN)
            return ast.ErrExpr(inner, line=tok.line)
        expr = self.parse_expr()
        # Check for range pattern: 1 to 10
        if self.at(TokenType.TO):
            self.advance()
            end = self.parse_expr()
            return ast.BinaryOp(expr, "to", end, line=expr.line)
        return expr

    def parse_module(self) -> ast.ModuleDef:
        tok = self.expect(TokenType.MODULE)
        name = self.expect(TokenType.IDENTIFIER).value
        body = self.parse_block_until(TokenType.ENDMODULE)
        self.expect(TokenType.ENDMODULE)
        return ast.ModuleDef(name=name, body=body, line=tok.line)

    def parse_use(self) -> ast.UseStmt:
        tok = self.expect(TokenType.USE)
        first = self.expect(TokenType.IDENTIFIER).value

        # use gcd, lcm from math_utils
        if self.at(TokenType.COMMA) or self.at(TokenType.FROM):
            names = [first]
            while self.at(TokenType.COMMA):
                self.advance()
                names.append(self.expect(TokenType.IDENTIFIER).value)
            self.expect(TokenType.FROM)
            module = self.expect(TokenType.IDENTIFIER).value
            return ast.UseStmt(module=module, names=names, line=tok.line)

        # use math_utils
        return ast.UseStmt(module=first, names=[], line=tok.line)

    def parse_return(self) -> ast.ReturnStmt:
        tok = self.expect(TokenType.RETURN)
        value = None
        if not self.at(TokenType.NEWLINE, TokenType.EOF, TokenType.ENDFUNC,
                       TokenType.ENDIF, TokenType.ENDWHILE, TokenType.ENDFOR,
                       TokenType.ENDMATCH, TokenType.ENDMODULE, TokenType.ENDPARALLEL):
            value = self.parse_expr()
        return ast.ReturnStmt(value=value, line=tok.line)

    def parse_parallel(self) -> ast.ParallelStmt:
        tok = self.expect(TokenType.PARALLEL)
        allow_fail = False
        result_var = None

        if self.at(TokenType.ALLOW_FAIL):
            self.advance()
            allow_fail = True

        if self.at(TokenType.FAT_ARROW):
            self.advance()
            result_var = self.expect(TokenType.IDENTIFIER).value

        self.expect(TokenType.DO)
        body = self.parse_block_until(TokenType.ENDPARALLEL)
        self.expect(TokenType.ENDPARALLEL)
        return ast.ParallelStmt(body=body, result_var=result_var,
                                 allow_fail=allow_fail, line=tok.line)


def parse(source: str) -> ast.Program:
    """Convenience function to parse source code into AST."""
    tokens = tokenize(source)
    return Parser(tokens).parse()
