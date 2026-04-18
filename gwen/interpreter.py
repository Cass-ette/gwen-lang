"""Gwen interpreter - tree-walk execution of AST."""

import struct
from typing import Any, Dict, List, Optional
from . import ast_nodes as ast


# --- Explicit precision type definitions ---

INT_RANGES = {
    "int8":   (-2**7,       2**7 - 1),
    "int16":  (-2**15,      2**15 - 1),
    "int32":  (-2**31,      2**31 - 1),
    "int64":  (-2**63,      2**63 - 1),
    "uint8":  (0,           2**8 - 1),
    "uint16": (0,           2**16 - 1),
    "uint32": (0,           2**32 - 1),
    "uint64": (0,           2**64 - 1),
}

PRECISION_TYPES = {"float32", "float64", "int8", "int16", "int32", "int64",
                   "uint8", "uint16", "uint32", "uint64"}


def coerce_to_type(value: Any, type_name: str, line: int = 0) -> Any:
    """Coerce a value to the specified explicit precision type."""
    if type_name == "float32":
        # Truncate to IEEE 754 single precision
        f = float(value)
        return struct.unpack('f', struct.pack('f', f))[0]
    elif type_name == "float64":
        return float(value)
    elif type_name in INT_RANGES:
        lo, hi = INT_RANGES[type_name]
        i = int(value)
        if i < lo or i > hi:
            raise GwenError(f"Overflow: {i} out of range for {type_name} [{lo}, {hi}]", line)
        return i
    elif type_name == "int":
        return int(value)
    elif type_name == "float":
        return float(value)
    else:
        return value  # unknown type, pass through


def resolve_type_name(type_node: Any) -> Optional[str]:
    """Extract the type name string from a type AST node."""
    if type_node is None:
        return None
    if isinstance(type_node, ast.TypeName):
        return type_node.name
    if isinstance(type_node, ast.GenericType):
        return type_node.base
    return None


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
    def __init__(self, parent: Optional['Environment'] = None, is_call_frame: bool = False, func_name: Optional[str] = None):
        self.vars: Dict[str, Any] = {}
        self.types: Dict[str, str] = {}  # variable name -> type name (e.g. "int8")
        self.consts: set = set()  # set of variable names that are const (immutable)
        self.parent = parent
        self.is_call_frame = is_call_frame  # True for function call environments
        self.func_name = func_name  # Function name for this call frame

    def get(self, name: str) -> Any:
        if name in self.vars:
            return self.vars[name]
        if self.parent:
            return self.parent.get(name)
        raise GwenError(f"Undefined variable: {name}")

    def get_type(self, name: str) -> Optional[str]:
        """Look up type annotation for variable across scope chain."""
        if name in self.types:
            return self.types[name]
        if self.parent:
            return self.parent.get_type(name)
        return None

    def get_local_type(self, name: str) -> Optional[str]:
        """Look up type annotation for variable in current scope only."""
        return self.types.get(name)

    def set(self, name: str, value: Any):
        """Create new variable in current scope."""
        self.vars[name] = value

    def set_type(self, name: str, type_name: Optional[str]):
        """Record type annotation for a variable."""
        if type_name:
            self.types[name] = type_name

    def mark_const(self, name: str):
        """Mark a variable as const (immutable) in current scope."""
        self.consts.add(name)

    def is_const(self, name: str) -> bool:
        """Check if name is const anywhere in the scope chain."""
        if name in self.consts:
            return True
        if self.parent:
            return self.parent.is_const(name)
        return False

    def update_local(self, name: str, value: Any):
        """Update or create variable in current scope only."""
        self.vars[name] = value

    def update(self, name: str, value: Any, current_func: Optional[str] = None):
        """Local assignment: always create/update in current scope only.
        Use global x := value to modify outer scope explicitly."""
        # Always update/create in current scope (local behavior)
        self.vars[name] = value


class Interpreter:
    def __init__(self):
        self.global_env = Environment()
        self.modules: Dict[str, Environment] = {}
        self.type_aliases: Dict[str, str] = {}  # alias name -> canonical type name
        self._setup_builtins()

    def _setup_builtins(self):
        self.global_env.set("write", self._builtin_write)
        self.global_env.set("read", self._builtin_read)
        self.global_env.set("len", self._builtin_len)
        self.global_env.set("str", self._builtin_str)
        self.global_env.set("int", self._builtin_int)
        self.global_env.set("float", self._builtin_float)
        self.global_env.set("append", self._builtin_append)
        self.global_env.set("type", self._builtin_type)

    def _resolve_alias(self, type_name: Optional[str]) -> Optional[str]:
        """Follow type alias chain to canonical type name."""
        seen = set()
        while type_name and type_name in self.type_aliases:
            if type_name in seen:
                break  # circular alias guard
            seen.add(type_name)
            type_name = self.type_aliases[type_name]
        return type_name

    def _builtin_write(self, *args):
        print(*args)
        return None

    def _builtin_read(self, prompt: str = ""):
        if prompt:
            return input(prompt)
        return input()

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
        except GwenError:
            # No main() defined, that's fine for scripts without func main
            return
        if isinstance(main_fn, GwenFunction):
            self.call_function(main_fn, [])

    def exec_block(self, stmts: List[Any], env: Environment):
        for stmt in stmts:
            self.exec_stmt(stmt, env)

    def exec_stmt(self, stmt: Any, env: Environment):
        if isinstance(stmt, ast.FuncDef):
            fn = GwenFunction(stmt, env)
            env.set(stmt.name, fn)

        elif isinstance(stmt, ast.Assignment):
            # Block reassignment to const bindings before evaluating RHS
            for target in stmt.targets:
                if isinstance(target, str) and env.is_const(target):
                    raise GwenError(f"Cannot assign to const variable: {target}", stmt.line)
            values = [self.eval_expr(v, env) for v in stmt.values]
            current_func = env.func_name
            # Check for multi-value unpacking from function return
            if len(stmt.targets) > 1 and len(values) == 1 and isinstance(values[0], list):
                # Unpack function return values: a, b := func() where func returns [x, y]
                unpacked = values[0]
                if len(stmt.targets) != len(unpacked):
                    raise GwenError(f"Assignment count mismatch: {len(stmt.targets)} targets, {len(unpacked)} values", stmt.line)
                for target, val in zip(stmt.targets, unpacked):
                    if isinstance(target, str):
                        env.update(target, self._coerce_if_typed(target, val, stmt.line, env), current_func)
                    elif isinstance(target, ast.IndexAccess):
                        obj = self.eval_expr(target.obj, env)
                        index = self.eval_expr(target.index, env)
                        obj[index] = val
                    else:
                        raise GwenError("Invalid assignment target", stmt.line)
            elif len(stmt.targets) == 1 and len(values) == 1:
                target = stmt.targets[0]
                if isinstance(target, str):
                    env.update(target, self._coerce_if_typed(target, values[0], stmt.line, env), current_func)
                elif isinstance(target, ast.IndexAccess):
                    obj = self.eval_expr(target.obj, env)
                    index = self.eval_expr(target.index, env)
                    obj[index] = values[0]
                else:
                    raise GwenError("Invalid assignment target", stmt.line)
            elif len(stmt.targets) == len(values):
                # Multi-assignment: a, b := x, y
                for target, val in zip(stmt.targets, values):
                    if isinstance(target, str):
                        env.update(target, self._coerce_if_typed(target, val, stmt.line, env), current_func)
                    elif isinstance(target, ast.IndexAccess):
                        obj = self.eval_expr(target.obj, env)
                        index = self.eval_expr(target.index, env)
                        obj[index] = val
                    else:
                        raise GwenError("Invalid assignment target", stmt.line)
            else:
                raise GwenError(f"Assignment count mismatch: {len(stmt.targets)} targets, {len(values)} values", stmt.line)

        elif isinstance(stmt, ast.VarDecl):
            if env.is_const(stmt.name):
                raise GwenError(f"Cannot redeclare const variable: {stmt.name}", stmt.line)
            value = self.eval_expr(stmt.value, env) if stmt.value else None
            type_name = self._resolve_alias(resolve_type_name(stmt.type_name))
            if value is not None and type_name and type_name in PRECISION_TYPES:
                value = coerce_to_type(value, type_name, stmt.line)
            env.set(stmt.name, value)
            env.set_type(stmt.name, type_name)
            if stmt.is_const:
                env.mark_const(stmt.name)

        elif isinstance(stmt, ast.TypeAlias):
            target_name = self._resolve_alias(resolve_type_name(stmt.target))
            if target_name is None:
                raise GwenError(f"Invalid type in alias '{stmt.name}'", stmt.line)
            self.type_aliases[stmt.name] = target_name

        elif isinstance(stmt, ast.ReturnStmt):
            if stmt.value is None:
                raise ReturnSignal(None)
            # Support multiple return values
            if isinstance(stmt.value, list):
                values = [self.eval_expr(v, env) for v in stmt.value]
                raise ReturnSignal(values)
            else:
                value = self.eval_expr(stmt.value, env)
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

            # Determine direction based on direction field and auto-detection
            if stmt.direction == "asc":
                # Force ascending: always iterate small -> large
                if start > end:
                    start, end = end, start
                step = 1 if step is None else abs(step)
                compare = lambda i, end: i <= end
            elif stmt.direction == "desc":
                # Force descending: always iterate large -> small
                if start < end:
                    start, end = end, start
                step = -1 if step is None else -abs(step)
                compare = lambda i, end: i >= end
            else:
                # Auto mode: infer from start/end
                if step is None:
                    step = 1 if start <= end else -1
                if step > 0:
                    compare = lambda i, end: i <= end
                else:
                    compare = lambda i, end: i >= end

            i = start
            while compare(i, end):
                env.update_local(stmt.var, i)
                self.exec_block(stmt.body, env)
                i += step

        elif isinstance(stmt, ast.ForEachStmt):
            iterable = self.eval_expr(stmt.iterable, env)
            for idx, item in enumerate(iterable):
                env.update_local(stmt.var, item)
                if stmt.index_var:
                    env.update_local(stmt.index_var, idx)
                self.exec_block(stmt.body, env)

        elif isinstance(stmt, ast.MatchStmt):
            subject = self.eval_expr(stmt.subject, env)

            # [方案 A] 如果 subject 是 Result 类型，强制使用 ok(x)/err(x) 模式
            is_result = isinstance(subject, (OkValue, ErrValue))
            if is_result:
                for case in stmt.cases:
                    for pat in case.patterns:
                        if not isinstance(pat, (ast.OkExpr, ast.ErrExpr)):
                            raise GwenError(
                                f"Match on Result type must use ok(x) or err(x) patterns, not '{type(pat).__name__}' "
                                f"(line {pat.line if hasattr(pat, 'line') else '?'}). "
                                f"Use 'when ok(val) then ...' or 'when err(msg) then ...'",
                                stmt.line
                            )

            matched = False
            for case in stmt.cases:
                case_env = Environment(parent=env)
                if self.match_patterns(subject, case.patterns, case_env, stmt.line):
                    # Inject pattern-bound variables into parent scope
                    for name, value in case_env.vars.items():
                        env.set(name, value)
                    self.exec_block(case.body, env)
                    matched = True
                    break
            if not matched:
                if not stmt.else_body:
                    # [方案 B] 没有匹配且没有 else 分支
                    raise GwenError(
                        f"Match statement has no matching case and no 'else' branch (exhaustive match required)",
                        stmt.line
                    )
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

        elif isinstance(stmt, ast.GlobalStmt):
            # global x := value - force assignment to outer (non-local) scope
            # Searches: 1) current call frame's parent (module/closure), 2) call stack
            if env.is_const(stmt.name):
                raise GwenError(f"Cannot assign to const variable: {stmt.name}", stmt.line)
            value = self.eval_expr(stmt.value, env)

            # First try: search up env chain (module/closures)
            search_env = env.parent if env.is_call_frame else env
            found = False
            while search_env:
                if stmt.name in search_env.vars:
                    # Check type annotation and coerce if needed
                    type_name = search_env.get_type(stmt.name)
                    if type_name and type_name in PRECISION_TYPES:
                        value = coerce_to_type(value, type_name, stmt.line)
                    search_env.vars[stmt.name] = value
                    found = True
                    break
                if search_env.is_call_frame:
                    # Found a call frame - check if it has the variable
                    if stmt.name in search_env.vars:
                        type_name = search_env.get_type(stmt.name)
                        if type_name and type_name in PRECISION_TYPES:
                            value = coerce_to_type(value, type_name, stmt.line)
                        search_env.vars[stmt.name] = value
                        found = True
                        break
                    # Otherwise continue up
                search_env = search_env.parent

            if not found:
                # Variable doesn't exist in any accessible outer scope
                raise GwenError(f"global variable '{stmt.name}' not found in any outer scope", stmt.line)

        elif isinstance(stmt, ast.ArenaStmt):
            # arena name do ... endarena - explicit memory region
            # Current implementation: just execute the block (GC handles memory)
            # Future: track arena allocations for batch release
            self.exec_block(stmt.body, env)

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
                env.update_local(stmt.result_var, results)

        elif isinstance(stmt, ast.TagStmt):
            pass  # Tags are decorative, no runtime effect

        elif isinstance(stmt, ast.ExprStmt):
            self.eval_expr(stmt.expr, env)

        else:
            raise GwenError(f"Unknown statement type: {type(stmt).__name__}")

    def _coerce_if_typed(self, name: str, value: Any, line: int, env: Environment) -> Any:
        """If variable has a precision type annotation, coerce value to it."""
        type_name = env.get_local_type(name)
        if type_name and type_name in PRECISION_TYPES:
            return coerce_to_type(value, type_name, line)
        return value

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

        if isinstance(expr, ast.AsExpr):
            return self.eval_as(expr, env)

        raise GwenError(f"Unknown expression type: {type(expr).__name__}")

    def eval_binary(self, op: str, left: Any, right: Any, line: int) -> Any:
        # Type promotion: if one side is float, promote the other to float
        if op in ("+", "-", "*", "/"):
            if isinstance(left, int) and isinstance(right, float):
                left = float(left)
            elif isinstance(left, float) and isinstance(right, int):
                right = float(right)

        if op == "+":
            return left + right
        if op == "-":
            return left - right
        if op == "*":
            return left * right
        if op == "/":
            if right == 0:
                raise GwenError("Division by zero", line)
            if isinstance(left, int) and isinstance(right, int):
                return int(left / right)
            return left / right
        if op == "mod":
            return left % right
        if op == "^":
            result = left ** right
            if isinstance(left, int) and isinstance(right, int) and isinstance(result, int):
                return result
            return float(result)
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

    def eval_as(self, expr: ast.AsExpr, env: Environment) -> Any:
        value = self.eval_expr(expr.expr, env)
        target = expr.type_name
        try:
            if target == "int":
                return OkValue(int(value))
            if target == "float":
                return OkValue(float(value))
            if target == "string":
                return OkValue(str(value))
            if target == "bool":
                return OkValue(self.is_truthy(value))
            if target in PRECISION_TYPES:
                return OkValue(coerce_to_type(value, target, expr.line))
            return ErrValue(f"Unknown type: {target}")
        except GwenError:
            raise
        except (ValueError, TypeError, OverflowError):
            return ErrValue(f"Cannot convert {type(value).__name__} to {target}")

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
        call_env = Environment(parent=fn.closure, is_call_frame=True, func_name=fn.node.name)
        params = fn.node.params
        for i, param in enumerate(params):
            if i < len(args):
                val = args[i]
                ptype = resolve_type_name(param.type_name)
                if ptype and ptype in PRECISION_TYPES:
                    val = coerce_to_type(val, ptype, param.line)
                call_env.set(param.name, val)
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
        call_env = Environment(parent=lam.closure, is_call_frame=True, func_name=None)
        for i, param in enumerate(lam.node.params):
            if i < len(args):
                call_env.set(param.name, args[i])
        try:
            self.exec_block(lam.node.body, call_env)
        except ReturnSignal as r:
            return r.value
        return None

    def match_patterns(self, subject: Any, patterns: List[Any], env: Environment, line: int = 0) -> bool:
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
        if isinstance(value, float):
            return value != 0.0
        if isinstance(value, str):
            return len(value) > 0
        if isinstance(value, list):
            return len(value) > 0
        return True
