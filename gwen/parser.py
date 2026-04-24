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

    def _peek_next(self):
        """Look at the token after current, if exists."""
        if self.pos + 1 < len(self.tokens):
            return self.tokens[self.pos + 1]
        return None

    def parse_object_literal(self) -> ast.ObjectLiteral:
        """Parse ObjectName{field1 := value1, field2 := value2, ...}."""
        tok = self.advance()  # ObjectName (IDENTIFIER)
        name = tok.value
        line = tok.line
        self.expect(TokenType.LBRACE)

        fields = []
        self.skip_newlines()
        if not self.at(TokenType.RBRACE):
            # First field: field_name := value
            field_name = self.expect(TokenType.IDENTIFIER).value
            self.expect(TokenType.ASSIGN)  # :=
            value = self.parse_expr()
            fields.append((field_name, value))

            while True:
                self.skip_newlines()
                if self.at(TokenType.RBRACE):
                    break
                if self.at(TokenType.COMMA):
                    self.advance()
                    self.skip_newlines()
                    if self.at(TokenType.RBRACE):  # Trailing comma
                        break
                    field_name = self.expect(TokenType.IDENTIFIER).value
                    self.expect(TokenType.ASSIGN)  # :=
                    value = self.parse_expr()
                    fields.append((field_name, value))
                else:
                    break
        self.expect(TokenType.RBRACE)
        return ast.ObjectLiteral(name=name, fields=fields, line=line)

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
            if self.at(TokenType.FUNC):
                return self.parse_func_def(exported=True)
            if self.at(TokenType.OBJECT):
                return self.parse_object_def(exported=True)
            if self.at(TokenType.IDENTIFIER) and self.peek().value == "type":
                next_tok = self.tokens[self.pos + 1] if self.pos + 1 < len(self.tokens) else None
                if next_tok and next_tok.type == TokenType.IDENTIFIER:
                    return self.parse_type_alias(exported=True)
            raise ParseError("Expected 'func', 'object', or 'type' after 'export'", self.peek())
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
        if tok.type == TokenType.GLOBAL:
            return self.parse_global()
        if tok.type == TokenType.CONST:
            return self.parse_const()
        if tok.type == TokenType.ARENA:
            return self.parse_arena()
        if tok.type == TokenType.VAR:
            return self.parse_var_block()
        if tok.type == TokenType.OBJECT:
            return self.parse_object_def()
        # contextual keyword: type Alias = ExistingType
        if tok.type == TokenType.IDENTIFIER and tok.value == "type":
            next_tok = self.tokens[self.pos + 1] if self.pos + 1 < len(self.tokens) else None
            if next_tok and next_tok.type == TokenType.IDENTIFIER:
                return self.parse_type_alias()
        if tok.type == TokenType.TAG:
            name = tok.value
            self.advance()
            return ast.TagStmt(name=name, line=tok.line)
        if tok.type == TokenType.NEWLINE:
            self.advance()
            return None

        if tok.type == TokenType.IDENTIFIER:
            if tok.value == "pass":
                next_tok = self._peek_next()
                if next_tok is None or next_tok.type in (TokenType.NEWLINE, TokenType.EOF):
                    return self.parse_pass()
            if tok.value == "leave":
                next_tok = self._peek_next()
                if next_tok and next_tok.type == TokenType.IDENTIFIER:
                    return self.parse_leave()
            if tok.value == "next":
                next_tok = self._peek_next()
                if next_tok and next_tok.type == TokenType.IDENTIFIER:
                    return self.parse_next()

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
            elif isinstance(expr, ast.MemberAccess):
                # Support self.field := value (member access assignment)
                targets = [expr]
            else:
                raise ParseError("Expected identifier, index access, or member access in assignment", self.peek())
            values = [self.parse_expr()]
            return ast.Assignment(targets=targets, values=values, line=tok.line)

        # Check for multi-target: a, b := x, y  or  arr[i], arr[j] := x, y
        if self.at(TokenType.COMMA) and isinstance(expr, (ast.Identifier, ast.IndexAccess, ast.MemberAccess)):
            targets = []
            if isinstance(expr, ast.Identifier):
                targets = [expr.name]
            else:  # IndexAccess or MemberAccess
                targets = [expr]
            while self.at(TokenType.COMMA):
                self.advance()
                # Parse next target: identifier, index access, or member access
                target_expr = self.parse_expr()
                if isinstance(target_expr, ast.Identifier):
                    targets.append(target_expr.name)
                elif isinstance(target_expr, (ast.IndexAccess, ast.MemberAccess)):
                    targets.append(target_expr)
                else:
                    raise ParseError("Expected identifier, index access, or member access in multi-assignment", self.peek())
            self.expect(TokenType.ASSIGN)
            values = [self.parse_expr()]
            while self.at(TokenType.COMMA):
                self.advance()
                values.append(self.parse_expr())
            return ast.Assignment(targets=targets, values=values, line=tok.line)

        # Check for typed var decl: x: int := 42  OR  x: int  (uninit)
        if self.at(TokenType.COLON) and isinstance(expr, ast.Identifier):
            self.advance()
            type_node = self.parse_type()
            value = None
            is_uninit = True
            if self.at(TokenType.ASSIGN):
                self.advance()
                value = self.parse_expr()
                is_uninit = False
            return ast.VarDecl(name=expr.name, type_name=type_node, value=value,
                               is_uninit=is_uninit, line=tok.line)

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
            name = type_tok.value
            # Optional generic tag: as money[USD]
            if self.at(TokenType.LBRACKET):
                self.advance()
                parts = [self.expect(TokenType.IDENTIFIER).value]
                while self.at(TokenType.COMMA):
                    self.advance()
                    parts.append(self.expect(TokenType.IDENTIFIER).value)
                self.expect(TokenType.RBRACKET)
                name = f"{name}[{','.join(parts)}]"
            expr = ast.AsExpr(expr, name, line=expr.line)
        return expr

    def parse_postfix(self) -> Any:
        expr = self.parse_primary()
        while True:
            if self.at(TokenType.LPAREN):
                expr = self.parse_call(expr)
            elif self.at(TokenType.DOT):
                self.advance()
                # Allow `new` (keyword) as a member name: e.g. Account.new(...)
                if self.at(TokenType.NEW):
                    member_tok = self.advance()
                else:
                    member_tok = self.expect(TokenType.IDENTIFIER)
                expr = ast.MemberAccess(expr, member_tok.value, line=expr.line)
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

        # Dict literal: dict[string, int]{"a": 1, "b": 2}
        if tok.type == TokenType.IDENTIFIER and tok.value == "dict":
            return self.parse_dict_literal()

        # Object literal: ObjectName{field := value, ...}
        if tok.type == TokenType.IDENTIFIER and self._peek_next() and self._peek_next().type == TokenType.LBRACE:
            return self.parse_object_literal()

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
        self.skip_newlines()  # Allow newline after [
        if not self.at(TokenType.RBRACKET):
            elements.append(self.parse_expr())
            while True:
                self.skip_newlines()  # Allow newlines between elements
                if self.at(TokenType.RBRACKET):
                    break
                if self.at(TokenType.COMMA):
                    self.advance()
                    self.skip_newlines()  # Allow newline after comma
                    if self.at(TokenType.RBRACKET):  # Trailing comma
                        break
                    elements.append(self.parse_expr())
                else:
                    break
        self.expect(TokenType.RBRACKET)
        return ast.ListLiteral(elements, line=tok.line)

    def parse_dict_literal(self) -> ast.DictLiteral:
        """Parse dict[string, int]{"a": 1, "b": 2} syntax."""
        tok = self.advance()  # dict
        # Parse type parameters: [string, int]
        self.expect(TokenType.LBRACKET)
        key_type = self.parse_type()
        self.expect(TokenType.COMMA)
        value_type = self.parse_type()
        self.expect(TokenType.RBRACKET)
        # Parse entries: {"a": 1, "b": 2}
        self.expect(TokenType.LBRACE)
        entries = []
        self.skip_newlines()
        if not self.at(TokenType.RBRACE):
            # First entry
            key = self.parse_expr()
            self.expect(TokenType.COLON)
            value = self.parse_expr()
            entries.append((key, value))
            while True:
                self.skip_newlines()
                if self.at(TokenType.RBRACE):
                    break
                if self.at(TokenType.COMMA):
                    self.advance()
                    self.skip_newlines()
                    if self.at(TokenType.RBRACE):  # Trailing comma
                        break
                    key = self.parse_expr()
                    self.expect(TokenType.COLON)
                    value = self.parse_expr()
                    entries.append((key, value))
                else:
                    break
        self.expect(TokenType.RBRACE)
        return ast.DictLiteral(key_type=key_type, value_type=value_type, entries=entries, line=tok.line)

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
                    body = self.parse_block_until(TokenType.ENDFUNC)
                    self.expect(TokenType.ENDFUNC)
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
            # Support multiple return types: -> int, bool
            types = [self.parse_type()]
            while self.at(TokenType.COMMA):
                self.advance()
                types.append(self.parse_type())
            return_type = types if len(types) > 1 else types[0]

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
        if self.at(TokenType.LBRACKET):
            self.advance()
            params = [self.parse_type()]
            while self.at(TokenType.COMMA):
                self.advance()
                params.append(self.parse_type())
            self.expect(TokenType.RBRACKET)
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
        name = self._parse_optional_line_label()
        body = self.parse_block_until(TokenType.ENDWHILE)
        self.expect(TokenType.ENDWHILE)
        end_name = self._parse_optional_loop_end_name()
        if name and end_name and name != end_name:
            raise ParseError(f"Loop name mismatch: started '{name}', ended '{end_name}'", self.peek())
        return ast.WhileStmt(condition=condition, name=name or end_name or "", body=body, line=tok.line)

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
            direction = "auto"

            # Parse optional modifiers: order, reverse, step
            if self.at(TokenType.ORDER):
                self.advance()
                direction = "asc"
            elif self.at(TokenType.REVERSE):
                self.advance()
                direction = "desc"

            if self.at(TokenType.STEP):
                self.advance()
                step = self.parse_expr()

            self.expect(TokenType.DO)
            loop_name = self._parse_optional_line_label()
            body = self.parse_block_until(TokenType.ENDFOR)
            self.expect(TokenType.ENDFOR)
            end_name = self._parse_optional_loop_end_name()
            if loop_name and end_name and loop_name != end_name:
                raise ParseError(f"Loop name mismatch: started '{loop_name}', ended '{end_name}'", self.peek())
            return ast.ForRangeStmt(var=var, start=start, end=end, step=step,
                                     direction=direction, name=loop_name or end_name or "",
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
            loop_name = self._parse_optional_line_label()
            body = self.parse_block_until(TokenType.ENDFOR)
            self.expect(TokenType.ENDFOR)
            end_name = self._parse_optional_loop_end_name()
            if loop_name and end_name and loop_name != end_name:
                raise ParseError(f"Loop name mismatch: started '{loop_name}', ended '{end_name}'", self.peek())
            return ast.ForEachStmt(var=var, iterable=start, index_var=index_var,
                                    name=loop_name or end_name or "", body=body, line=tok.line)

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
            self.expect(TokenType.FAT_ARROW)
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
        values = []
        if not self.at(TokenType.NEWLINE, TokenType.EOF, TokenType.ENDFUNC,
                       TokenType.ENDIF, TokenType.ENDWHILE, TokenType.ENDFOR,
                       TokenType.ENDMATCH, TokenType.ENDMODULE, TokenType.ENDPARALLEL):
            values.append(self.parse_expr())
            # Support multiple return values: return a, b, c
            while self.at(TokenType.COMMA):
                self.advance()
                values.append(self.parse_expr())
        value = values if len(values) > 1 else (values[0] if values else None)
        return ast.ReturnStmt(value=value, line=tok.line)

    def parse_pass(self) -> ast.PassStmt:
        tok = self.peek()
        if tok.type != TokenType.IDENTIFIER or tok.value != "pass":
            raise ParseError("Expected 'pass'", tok)
        self.advance()
        return ast.PassStmt(line=tok.line)

    def parse_leave(self) -> ast.LeaveStmt:
        tok = self.peek()
        if tok.type != TokenType.IDENTIFIER or tok.value != "leave":
            raise ParseError("Expected 'leave'", tok)
        self.advance()
        name = self.expect(TokenType.IDENTIFIER).value
        return ast.LeaveStmt(name=name, line=tok.line)

    def parse_next(self) -> ast.NextStmt:
        tok = self.peek()
        if tok.type != TokenType.IDENTIFIER or tok.value != "next":
            raise ParseError("Expected 'next'", tok)
        self.advance()
        name = self.expect(TokenType.IDENTIFIER).value
        return ast.NextStmt(name=name, line=tok.line)

    def parse_global(self) -> ast.GlobalStmt:
        """Parse global x := value - force assignment to module/global scope."""
        tok = self.expect(TokenType.GLOBAL)
        name = self.expect(TokenType.IDENTIFIER).value
        self.expect(TokenType.ASSIGN)
        value = self.parse_expr()
        return ast.GlobalStmt(name=name, value=value, line=tok.line)

    def _parse_optional_line_label(self) -> str:
        if not self.at(TokenType.IDENTIFIER):
            return ""
        next_tok = self._peek_next()
        if next_tok is None or next_tok.type not in (TokenType.NEWLINE, TokenType.EOF):
            return ""
        return self.advance().value

    def _parse_optional_loop_end_name(self) -> str:
        if not self.at(TokenType.IDENTIFIER):
            return ""
        next_tok = self._peek_next()
        if next_tok is not None and next_tok.type not in (TokenType.NEWLINE, TokenType.EOF):
            raise ParseError("Expected newline after loop name", self.peek())
        return self.advance().value

    def parse_const(self) -> ast.VarDecl:
        """Parse const NAME [: type] := value - immutable binding."""
        tok = self.expect(TokenType.CONST)
        name = self.expect(TokenType.IDENTIFIER).value
        type_node = None
        if self.at(TokenType.COLON):
            self.advance()
            type_node = self.parse_type()
        self.expect(TokenType.ASSIGN)
        value = self.parse_expr()
        return ast.VarDecl(name=name, type_name=type_node, value=value, is_const=True, line=tok.line)

    def parse_type_alias(self, exported: bool = False) -> ast.TypeAlias:
        """Parse type Alias = ExistingType - transparent type alias."""
        tok = self.advance()  # consume 'type' identifier
        name = self.expect(TokenType.IDENTIFIER).value
        self.expect(TokenType.EQ)
        target = self.parse_type()
        return ast.TypeAlias(name=name, target=target, exported=exported, line=tok.line)

    def parse_var_block(self) -> ast.VarBlock:
        """Parse var [default [<expr>]] NAME : TYPE (newline ...)* endvar."""
        tok = self.expect(TokenType.VAR)
        default_mode = "none"
        default_value = None
        if self.at(TokenType.DEFAULT):
            self.advance()
            default_mode = "zero"
            # Peek: if next non-newline token is an expression (not NEWLINE), parse it
            if not self.at(TokenType.NEWLINE):
                default_mode = "value"
                default_value = self.parse_expr()
        self.skip_newlines()
        decls: List[ast.VarDecl] = []
        while not self.at(TokenType.ENDVAR):
            if self.at(TokenType.EOF):
                raise ParseError("Unexpected EOF in var block", self.peek())
            name_tok = self.expect(TokenType.IDENTIFIER)
            self.expect(TokenType.COLON)
            type_node = self.parse_type()
            value = None
            is_uninit = True
            if self.at(TokenType.ASSIGN):
                self.advance()
                value = self.parse_expr()
                is_uninit = False
            decls.append(ast.VarDecl(name=name_tok.value, type_name=type_node,
                                     value=value, is_uninit=is_uninit,
                                     line=name_tok.line))
            self.skip_newlines()
        self.expect(TokenType.ENDVAR)
        return ast.VarBlock(decls=decls, default_mode=default_mode,
                            default_value=default_value, line=tok.line)

    def parse_parallel(self) -> ast.ParallelStmt:
        tok = self.expect(TokenType.PARALLEL)
        allow_fail = False
        result_var = None

        if self.at(TokenType.ALLOWFAIL):
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


    def parse_arena(self) -> ast.ArenaStmt:
        """Parse arena name do ... endarena."""
        tok = self.expect(TokenType.ARENA)
        name = self.expect(TokenType.IDENTIFIER).value
        self.expect(TokenType.DO)
        body = self.parse_block_until(TokenType.ENDARENA)
        self.expect(TokenType.ENDARENA)
        return ast.ArenaStmt(name=name, body=body, line=tok.line)

    def parse_object_def(self, exported: bool = False) -> ast.ObjectDef:
        """Parse object Name ... endobject."""
        tok = self.expect(TokenType.OBJECT)
        name = self.expect(TokenType.IDENTIFIER).value
        line = tok.line

        fields: List[ast.FieldDef] = []
        constructor: Optional[ast.ConstructorDef] = None
        methods: List[ast.MethodDef] = []

        # Parse fields (name: Type), methods (func name...endfunc), and constructor (new...endnew)
        while not self.at(TokenType.ENDOBJECT):
            # Skip newlines
            while self.at(TokenType.NEWLINE):
                self.advance()
            if self.at(TokenType.ENDOBJECT):
                break
            if self.at(TokenType.IDENTIFIER):
                # Field definition: field_name: Type
                field_name = self.expect(TokenType.IDENTIFIER).value
                self.expect(TokenType.COLON)
                type_node = self.parse_type()
                fields.append(ast.FieldDef(name=field_name, type_annotation=type_node, line=line))
                # Optional newline
                if self.at(TokenType.NEWLINE):
                    self.advance()
            elif self.at(TokenType.FUNC):
                # Method definition
                methods.append(self.parse_method_def())
            elif self.at(TokenType.NEW):
                # Constructor definition
                constructor = self.parse_constructor_def(name)
            else:
                raise ParseError(f"Unexpected token in object definition: {self.peek()}", self.peek())

        self.expect(TokenType.ENDOBJECT)
        return ast.ObjectDef(
            name=name,
            fields=fields,
            constructor=constructor,
            methods=methods,
            exported=exported,
            line=line
        )

    def parse_constructor_def(self, object_name: str) -> ast.ConstructorDef:
        """Parse new(params) -> Type ... endnew (constructor)."""
        tok = self.expect(TokenType.NEW)
        line = tok.line

        self.expect(TokenType.LPAREN)
        params = []
        while not self.at(TokenType.RPAREN):
            params.append(self._parse_param())
            if self.at(TokenType.COMMA):
                self.advance()
        self.expect(TokenType.RPAREN)

        # Optional return type annotation: -> Type
        return_type = None
        if self.at(TokenType.ARROW):
            self.advance()
            return_type = self.parse_type()

        # Body: parse statements until endnew
        body = self.parse_block_until(TokenType.ENDNEW)
        self.expect(TokenType.ENDNEW)

        return ast.ConstructorDef(
            name=object_name,  # same as object name
            params=params,
            return_type=return_type,
            body=body,
            line=line
        )

    def parse_method_def(self) -> ast.MethodDef:
        """Parse func name(params) -> Type ... endfunc (method)."""
        tok = self.expect(TokenType.FUNC)
        line = tok.line

        method_name = self.expect(TokenType.IDENTIFIER).value
        self.expect(TokenType.LPAREN)
        params = []
        while not self.at(TokenType.RPAREN):
            params.append(self._parse_param())
            if self.at(TokenType.COMMA):
                self.advance()
        self.expect(TokenType.RPAREN)

        # Optional return type annotation: -> Type
        return_type = None
        if self.at(TokenType.ARROW):
            self.advance()
            return_type = self.parse_type()

        # Body: parse statements until endfunc
        body = self.parse_block_until(TokenType.ENDFUNC)
        self.expect(TokenType.ENDFUNC)

        return ast.MethodDef(
            name=method_name,
            params=params,
            return_type=return_type,
            body=body,
            line=line
        )


def parse(source: str) -> ast.Program:
    """Convenience function to parse source code into AST."""
    tokens = tokenize(source)
    return Parser(tokens).parse()
