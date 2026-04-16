"""Gwen interpreter - tree-walk execution of AST."""

from typing import Any, Dict, List, Optional
from . import ast_nodes as ast


class GwenError(Exception):
    def __init__(self, message: str, line: int = 0):
        super().__init__(f"Runtime error at L{line}: {message}" if line else message)
        self.line = line


class ReturnSignal(Exception):
    """Used to unwind the call stack on return."""
    def __init__(self, value: Any):
        self.value = value


class OkValue:
    def __init__(self, value: Any):
        self.value = value
    def __repr__(self):
        return f"ok({self.value!r})"

class ErrValue:
    def __init__(self, value: Any):
        self.value = value
    def __repr__(self):
        return f"err({self.value!r})"


class GwenFunction:
    def __init__(self, node: ast.FuncDef, closure: 'Environment'):
        self.node = node
        self.closure = closure
    def __repr__(self):
        return f"<func {self.node.name}>"


class GwenLambda:
    def __init__(self, node: ast.Lambda, closure: 'Environment'):
        self.node = node
        self.closure = closure
    def __repr__(self):
        return "<lambda>"


class Environment:
    def __init__(self, parent: Optional['Environment'] = None):
        self.vars: Dict[str, Any] = {}
        self.parent = parent

    def get(self, name: str) -> Any:
        if name in self.vars:
            return self.vars[name]
        if self.parent:
            return self.parent.get(name)
        raise GwenError(f"Undefined variable: {name}")

    def set(self, name: str, value: Any):
        self.vars[name] = value

    def update(self, name: str, value: Any):
        """Update existing variable, searching up the scope chain."""
        if name in self.vars:
            self.vars[name] = value
            return
        if self.parent:
            self.parent.update(name, value)
            return
        # If not found, create in current scope
        self.vars[name] = value


class Interpreter:
    def __init__(self):
        self.global_env = Environment()
        self.modules: Dict[str, Environment] = {}
        self._setup_builtins()

    def _setup_builtins(self):
        self.global_env.set("print", self._builtin_print)
        self.global_env.set("len", self._builtin_len)
        self.global_env.set("str", self._builtin_str)
        self.global_env.set("int", self._builtin_int)
        self.global_env.set("float", self._builtin_float)
        self.global_env.set("append", self._builtin_append)
        self.global_env.set("type", self._builtin_type)

    def _builtin_print(self, *args):
        print(*args)
        return None

    def _builtin_len(self, obj):
        return len(obj)

    def _builtin_str(self, obj):
        return str(obj)

    def _builtin_int(self, obj):
        return int(obj)

    def _builtin_float(self, obj):
        return float(obj)

    def _builtin_append(self, lst, item):
        lst.append(item)
        return lst

    def _builtin_type(self, obj):
        if isinstance(obj, bool):
            return "bool"
        if isinstance(obj, int):
            return "int"
        if isinstance(obj, float):
            return "float"
        if isinstance(obj, str):
            return "string"
        if isinstance(obj, list):
            return "list"
        if isinstance(obj, OkValue):
            return "ok"
        if isinstance(obj, ErrValue):
            return "err"
        if isinstance(obj, (GwenFunction, GwenLambda)):
            return "func"
        return "unknown"

    def run(self, program: ast.Program):
        self.exec_block(program.statements, self.global_env)
        # Auto-call main() if it exists
        try:
            main_fn = self.global_env.get("main")
            if isinstance(main_fn, GwenFunction):
                self.call_function(main_fn, [])
        except GwenError:
            pass

    def exec_block(self, stmts: List[Any], env: Environment):
        for stmt in stmts:
            self.exec_stmt(stmt, env)

    def exec_stmt(self, stmt: Any, env: Environment):
        if isinstance(stmt, ast.FuncDef):
            fn = GwenFunction(stmt, env)
            env.set(stmt.name, fn)

        elif isinstance(stmt, ast.Assignment):
            values = [self.eval_expr(v, env) for v in stmt.values]
            if len(stmt.targets) == 1 and len(values) == 1:
                env.update(stmt.targets[0], values[0])
            elif len(stmt.targets) == len(values):
                # Evaluate all values first (for swap: a, b := b, a)
                for name, val in zip(stmt.targets, values):
                    env.update(name, val)
            else:
                raise GwenError(f"Assignment count mismatch: {len(stmt.targets)} targets, {len(values)} values", stmt.line)

        elif isinstance(stmt, ast.VarDecl):
            value = self.eval_expr(stmt.value, env) if stmt.value else None
            env.set(stmt.name, value)

        elif isinstance(stmt, ast.ReturnStmt):
            value = self.eval_expr(stmt.value, env) if stmt.value else None
            raise ReturnSignal(value)

        elif isinstance(stmt, ast.IfStmt):
            if self.is_truthy(self.eval_expr(stmt.condition, env)):
                self.exec_block(stmt.body, env)
            else:
                matched = False
                for cond, body in stmt.elifs:
                    if self.is_truthy(self.eval_expr(cond, env)):
                        self.exec_block(body, env)
                        matched = True
                        break
                if not matched and stmt.else_body:
                    self.exec_block(stmt.else_body, env)

        elif isinstance(stmt, ast.WhileStmt):
            while self.is_truthy(self.eval_expr(stmt.condition, env)):
                self.exec_block(stmt.body, env)

        elif isinstance(stmt, ast.ForRangeStmt):
            start = self.eval_expr(stmt.start, env)
            end = self.eval_expr(stmt.end, env)
            step = self.eval_expr(stmt.step, env) if stmt.step else None
            if step is None:
                step = 1 if start <= end else -1
            i = start
            while (step > 0 and i <= end) or (step < 0 and i >= end):
                env.update(stmt.var, i)
                self.exec_block(stmt.body, env)
                i += step

        elif isinstance(stmt, ast.ForEachStmt):
            iterable = self.eval_expr(stmt.iterable, env)
            for idx, item in enumerate(iterable):
                env.update(stmt.var, item)
                if stmt.index_var:
                    env.update(stmt.index_var, idx)
                self.exec_block(stmt.body, env)

        elif isinstance(stmt, ast.MatchStmt):
            subject = self.eval_expr(stmt.subject, env)
            matched = False
            for case in stmt.cases:
                case_env = Environment(parent=env)
                if self.match_patterns(subject, case.patterns, case_env):
                    self.exec_block(case.body, case_env)
                    matched = True
                    break
            if not matched and stmt.else_body:
                self.exec_block(stmt.else_body, env)

        elif isinstance(stmt, ast.ModuleDef):
            mod_env = Environment(parent=env)
            self.exec_block(stmt.body, mod_env)
            # Collect exported names
            module_ns = Environment()
            for s in stmt.body:
                if isinstance(s, ast.FuncDef) and s.exported:
                    module_ns.set(s.name, mod_env.get(s.name))
            self.modules[stmt.name] = mod_env
            env.set(stmt.name, module_ns)

        elif isinstance(stmt, ast.UseStmt):
            if stmt.module in self.modules:
                mod_env = self.modules[stmt.module]
                if stmt.names:
                    for name in stmt.names:
                        env.set(name, mod_env.get(name))
                else:
                    # Import module namespace
                    mod_ns = Environment()
                    for key, val in mod_env.vars.items():
                        mod_ns.set(key, val)
                    env.set(stmt.module, mod_ns)
            else:
                raise GwenError(f"Module not found: {stmt.module}", stmt.line)

        elif isinstance(stmt, ast.ParallelStmt):
            # In the interpreter, run sequentially (true parallelism needs async runtime)
            results = []
            for s in stmt.body:
                try:
                    self.exec_stmt(s, env)
                    if isinstance(s, ast.ExprStmt):
                        val = self.eval_expr(s.expr, env)
                        results.append(OkValue(val))
                    else:
                        results.append(OkValue(None))
                except GwenError as e:
                    if stmt.allow_fail:
                        results.append(ErrValue(str(e)))
                    else:
                        raise
            if stmt.result_var:
                env.update(stmt.result_var, results)

        elif isinstance(stmt, ast.TagStmt):
            pass  # Tags are decorative, no runtime effect

        elif isinstance(stmt, ast.ExprStmt):
            self.eval_expr(stmt.expr, env)

        else:
            raise GwenError(f"Unknown statement type: {type(stmt).__name__}")

    def eval_expr(self, expr: Any, env: Environment) -> Any:
        if isinstance(expr, ast.IntLiteral):
            return expr.value
        if isinstance(expr, ast.FloatLiteral):
            return expr.value
        if isinstance(expr, ast.StringLiteral):
            return expr.value
        if isinstance(expr, ast.BoolLiteral):
            return expr.value
        if isinstance(expr, ast.Identifier):
            return env.get(expr.name)
        if isinstance(expr, ast.ListLiteral):
            return [self.eval_expr(e, env) for e in expr.elements]

        if isinstance(expr, ast.BinaryOp):
            left = self.eval_expr(expr.left, env)
            right = self.eval_expr(expr.right, env)
            return self.eval_binary(expr.op, left, right, expr.line)

        if isinstance(expr, ast.UnaryOp):
            operand = self.eval_expr(expr.operand, env)
            if expr.op == "-":
                return -operand
            if expr.op == "not":
                return not self.is_truthy(operand)

        if isinstance(expr, ast.FuncCall):
            return self.eval_call(expr, env)

        if isinstance(expr, ast.MemberAccess):
            obj = self.eval_expr(expr.obj, env)
            if isinstance(obj, Environment):
                return obj.get(expr.member)
            raise GwenError(f"Cannot access member '{expr.member}' on {type(obj)}", expr.line)

        if isinstance(expr, ast.IndexAccess):
            obj = self.eval_expr(expr.obj, env)
            index = self.eval_expr(expr.index, env)
            return obj[index]

        if isinstance(expr, ast.Lambda):
            return GwenLambda(expr, env)

        if isinstance(expr, ast.OkExpr):
            return OkValue(self.eval_expr(expr.value, env))

        if isinstance(expr, ast.ErrExpr):
            return ErrValue(self.eval_expr(expr.value, env))

        raise GwenError(f"Unknown expression type: {type(expr).__name__}")

    def eval_binary(self, op: str, left: Any, right: Any, line: int) -> Any:
        if op == "+":
            return left + right
        if op == "-":
            return left - right
        if op == "*":
            return left * right
        if op == "/":
            if right == 0:
                raise GwenError("Division by zero", line)
            return left / right
        if op == "mod":
            return left % right
        if op == "=":
            return left == right
        if op == "!=":
            return left != right
        if op == "<":
            return left < right
        if op == ">":
            return left > right
        if op == "<=":
            return left <= right
        if op == ">=":
            return left >= right
        if op == "and":
            return self.is_truthy(left) and self.is_truthy(right)
        if op == "or":
            return self.is_truthy(left) or self.is_truthy(right)
        raise GwenError(f"Unknown operator: {op}", line)

    def eval_call(self, call: ast.FuncCall, env: Environment) -> Any:
        callee = self.eval_expr(call.name, env)
        args = [self.eval_expr(a, env) for a in call.args]

        if callable(callee):
            return callee(*args)

        if isinstance(callee, GwenFunction):
            return self.call_function(callee, args)

        if isinstance(callee, GwenLambda):
            return self.call_lambda(callee, args)

        raise GwenError(f"'{callee}' is not callable", call.line)

    def call_function(self, fn: GwenFunction, args: List[Any]) -> Any:
        call_env = Environment(parent=fn.closure)
        params = fn.node.params
        for i, param in enumerate(params):
            if i < len(args):
                call_env.set(param.name, args[i])
            elif param.default is not None:
                call_env.set(param.name, self.eval_expr(param.default, fn.closure))
            else:
                raise GwenError(f"Missing argument: {param.name}")
        try:
            self.exec_block(fn.node.body, call_env)
        except ReturnSignal as r:
            return r.value
        return None

    def call_lambda(self, lam: GwenLambda, args: List[Any]) -> Any:
        call_env = Environment(parent=lam.closure)
        for i, param in enumerate(lam.node.params):
            if i < len(args):
                call_env.set(param.name, args[i])
        try:
            self.exec_block(lam.node.body, call_env)
        except ReturnSignal as r:
            return r.value
        return None

    def match_patterns(self, subject: Any, patterns: List[Any], env: Environment) -> bool:
        for pattern in patterns:
            if self.match_single(subject, pattern, env):
                return True
        return False

    def match_single(self, subject: Any, pattern: Any, env: Environment) -> bool:
        # ok/err pattern matching - check type first before evaluating inner
        if isinstance(pattern, ast.OkExpr):
            if not isinstance(subject, OkValue):
                return False
            if isinstance(pattern.value, ast.Identifier):
                env.set(pattern.value.name, subject.value)
                return True
            return subject.value == self.eval_expr(pattern.value, env)

        if isinstance(pattern, ast.ErrExpr):
            if not isinstance(subject, ErrValue):
                return False
            if isinstance(pattern.value, ast.Identifier):
                env.set(pattern.value.name, subject.value)
                return True
            return subject.value == self.eval_expr(pattern.value, env)
            return subject.value == self.eval_expr(pattern.value, env)

        # Range pattern
        if isinstance(pattern, ast.BinaryOp) and pattern.op == "to":
            start = self.eval_expr(pattern.left, env)
            end = self.eval_expr(pattern.right, env)
            return start <= subject <= end

        # Literal match
        val = self.eval_expr(pattern, env)
        return subject == val

    def is_truthy(self, value: Any) -> bool:
        if value is None:
            return False
        if isinstance(value, bool):
            return value
        if isinstance(value, int):
            return value != 0
        if isinstance(value, str):
            return len(value) > 0
        if isinstance(value, list):
            return len(value) > 0
        return True
