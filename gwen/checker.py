"""Semantic checker for Gwen.

The checker runs before execution and catches a focused set of errors early:
- undefined names in definitely-executed code paths
- invalid module imports / exports
- unknown type names and broken alias chains
- invalid object member access and private-field misuse
- object method receiver shape (`self: ObjectName`)
"""

import os
from dataclasses import dataclass, field
from typing import Any, Callable, Dict, List, Optional

from . import ast_nodes as ast
from .stdlib_catalog import OFFICIAL_STDLIB_MODULES


BUILTIN_RUNTIME_RETURNS = {
    "len": "int",
    "str": "string",
    "int": "int",
    "float": "float",
    "typeof": "string",
    "asc": "bool",
    "desc": "bool",
    "map": "list",
    "filter": "list",
    "range": "list[int]",
    "enumerate": "list",
    "contains": "bool",
    "haskey": "bool",
    "abs": "float",
    "min": None,
    "max": None,
    "sqrt": "float",
    "floor": "float",
    "ceil": "float",
}

BUILTIN_RUNTIME_NAMES = {
    "write", "read", "len", "str", "int", "float", "append", "typeof",
    "sort", "asc", "desc", "reversed", "map", "filter", "range", "enumerate",
    "split", "join", "pop", "removeat",
    "insert", "concat", "substring", "contains", "trim", "replace", "abs",
    "min", "max", "sqrt", "floor", "ceil", "haskey", "get", "keys",
    "values", "items", "readfile", "writefile", "appendfile",
}


BASE_TYPE_NAMES = {
    "int", "float", "string", "bool",
    "int8", "int16", "int32", "int64",
    "uint8", "uint16", "uint32", "uint64",
    "float32", "float64",
    "list", "dict", "result",
}

GENERIC_BASE_ARITY = {
    "list": (1, 1),
    "dict": (2, 2),
    "money": (1, 1),
    "result": (1, None),
}


class SemanticError(Exception):
    def __init__(self, message: str, line: int = 0):
        self.raw_message = message
        super().__init__(f"Semantic error at L{line}: {message}" if line else message)
        self.line = line


@dataclass
class ObjectInfo:
    name: str
    fields: Dict[str, Optional[str]] = field(default_factory=dict)
    methods: Dict[str, "CallableInfo"] = field(default_factory=dict)
    has_constructor: bool = False
    constructor: Optional["CallableInfo"] = None


@dataclass
class ModuleInfo:
    name: str
    runtime_exports: Dict[str, "ValueInfo"] = field(default_factory=dict)
    type_exports: Dict[str, str] = field(default_factory=dict)


@dataclass
class ValueInfo:
    kind: str
    type_name: Optional[str] = None
    multi_type_names: Optional[List[str]] = None
    return_type_name: Optional[str] = None
    object_info: Optional[ObjectInfo] = None
    module_info: Optional[ModuleInfo] = None
    callable_info: Optional["CallableInfo"] = None


@dataclass
class ParamInfo:
    name: str
    type_name: Optional[str] = None
    has_default: bool = False
    line: int = 0


@dataclass
class CallableInfo:
    label: str
    params: List[ParamInfo] = field(default_factory=list)
    return_type_name: Optional[str] = None
    return_type_names: Optional[List[str]] = None
    variadic: bool = False
    definition_scope: Optional["Scope"] = field(default=None, repr=False)


BUILTIN_SIGNATURES = {
    "write": CallableInfo(label="write", variadic=True),
    "read": CallableInfo(label="read", params=[ParamInfo(name="prompt", has_default=True)]),
    "len": CallableInfo(label="len", params=[ParamInfo(name="obj")], return_type_name="int"),
    "str": CallableInfo(label="str", params=[ParamInfo(name="obj")], return_type_name="string"),
    "int": CallableInfo(label="int", params=[ParamInfo(name="obj")], return_type_name="int"),
    "float": CallableInfo(label="float", params=[ParamInfo(name="obj")], return_type_name="float"),
    "append": CallableInfo(label="append", params=[ParamInfo(name="lst"), ParamInfo(name="item")]),
    "typeof": CallableInfo(label="typeof", params=[ParamInfo(name="obj")], return_type_name="string"),
    "sort": CallableInfo(label="sort", params=[ParamInfo(name="lst"), ParamInfo(name="cmp")]),
    "asc": CallableInfo(label="asc", params=[ParamInfo(name="a"), ParamInfo(name="b")], return_type_name="bool"),
    "desc": CallableInfo(label="desc", params=[ParamInfo(name="a"), ParamInfo(name="b")], return_type_name="bool"),
    "reversed": CallableInfo(label="reversed", params=[ParamInfo(name="lst")]),
    "map": CallableInfo(label="map", params=[ParamInfo(name="lst", type_name="list"), ParamInfo(name="f")], return_type_name="list"),
    "filter": CallableInfo(label="filter", params=[ParamInfo(name="lst", type_name="list"), ParamInfo(name="pred")], return_type_name="list"),
    "range": CallableInfo(label="range", params=[ParamInfo(name="start", type_name="int"), ParamInfo(name="end", type_name="int"), ParamInfo(name="step", type_name="int", has_default=True)], return_type_name="list[int]"),
    "enumerate": CallableInfo(label="enumerate", params=[ParamInfo(name="lst", type_name="list")], return_type_name="list"),
    "split": CallableInfo(label="split", params=[ParamInfo(name="s"), ParamInfo(name="sep")]),
    "join": CallableInfo(label="join", params=[ParamInfo(name="parts"), ParamInfo(name="sep")]),
    "pop": CallableInfo(label="pop", params=[ParamInfo(name="lst")]),
    "removeat": CallableInfo(label="removeat", params=[ParamInfo(name="lst"), ParamInfo(name="idx")]),
    "insert": CallableInfo(label="insert", params=[ParamInfo(name="lst"), ParamInfo(name="idx"), ParamInfo(name="item")]),
    "concat": CallableInfo(label="concat", params=[ParamInfo(name="a"), ParamInfo(name="b")]),
    "substring": CallableInfo(label="substring", params=[ParamInfo(name="s"), ParamInfo(name="start"), ParamInfo(name="end")]),
    "contains": CallableInfo(label="contains", params=[ParamInfo(name="s"), ParamInfo(name="substr")], return_type_name="bool"),
    "trim": CallableInfo(label="trim", params=[ParamInfo(name="s")]),
    "replace": CallableInfo(label="replace", params=[ParamInfo(name="s"), ParamInfo(name="old"), ParamInfo(name="new")]),
    "abs": CallableInfo(label="abs", params=[ParamInfo(name="x")]),
    "min": CallableInfo(label="min", params=[ParamInfo(name="a"), ParamInfo(name="b")]),
    "max": CallableInfo(label="max", params=[ParamInfo(name="a"), ParamInfo(name="b")]),
    "sqrt": CallableInfo(label="sqrt", params=[ParamInfo(name="x")], return_type_name="float"),
    "floor": CallableInfo(label="floor", params=[ParamInfo(name="x")], return_type_name="float"),
    "ceil": CallableInfo(label="ceil", params=[ParamInfo(name="x")], return_type_name="float"),
    "haskey": CallableInfo(label="haskey", params=[ParamInfo(name="d"), ParamInfo(name="key")], return_type_name="bool"),
    "get": CallableInfo(label="get", params=[ParamInfo(name="d"), ParamInfo(name="key"), ParamInfo(name="default")]),
    "keys": CallableInfo(label="keys", params=[ParamInfo(name="d")]),
    "values": CallableInfo(label="values", params=[ParamInfo(name="d")]),
    "items": CallableInfo(label="items", params=[ParamInfo(name="d")]),
    "readfile": CallableInfo(label="readfile", params=[ParamInfo(name="path")]),
    "writefile": CallableInfo(label="writefile", params=[ParamInfo(name="path"), ParamInfo(name="content")]),
    "appendfile": CallableInfo(label="appendfile", params=[ParamInfo(name="path"), ParamInfo(name="content")]),
}


UNKNOWN_VALUE = ValueInfo(kind="unknown")


MODULE_DECLARATION_STMT_TYPES = (ast.UseStmt, ast.FuncDef, ast.ObjectDef, ast.TypeAlias)


def _module_stmt_label(stmt: Any) -> str:
    if isinstance(stmt, ast.UseStmt):
        return "use"
    if isinstance(stmt, ast.FuncDef):
        return "func"
    if isinstance(stmt, ast.ObjectDef):
        return "object"
    if isinstance(stmt, ast.TypeAlias):
        return "type"
    if isinstance(stmt, ast.Assignment):
        return "assignment"
    if isinstance(stmt, ast.VarDecl):
        return "var"
    if isinstance(stmt, ast.VarBlock):
        return "var block"
    if isinstance(stmt, ast.IfStmt):
        return "if"
    if isinstance(stmt, ast.WhileStmt):
        return "while"
    if isinstance(stmt, (ast.ForRangeStmt, ast.ForEachStmt)):
        return "for"
    if isinstance(stmt, ast.MatchStmt):
        return "match"
    if isinstance(stmt, ast.ParallelStmt):
        return "parallel"
    if isinstance(stmt, ast.GlobalStmt):
        return "global"
    if isinstance(stmt, ast.ArenaStmt):
        return "arena"
    if isinstance(stmt, ast.TagStmt):
        return "@tag"
    if isinstance(stmt, ast.ExprStmt):
        return "expression"
    if isinstance(stmt, ast.ModuleDef):
        return "module"
    return type(stmt).__name__


class Scope:
    def __init__(
        self,
        parent: Optional["Scope"] = None,
        method_self_type: Optional[str] = None,
        expected_return_types: Optional[List[str]] = None,
    ):
        self.parent = parent
        self.values: Dict[str, ValueInfo] = {}
        self.aliases: Dict[str, str] = {}
        if method_self_type is None and parent is not None:
            method_self_type = parent.method_self_type
        self.method_self_type = method_self_type
        if expected_return_types is None and parent is not None:
            expected_return_types = parent.expected_return_types
        self.expected_return_types = expected_return_types

    def define_value(self, name: str, value: ValueInfo):
        self.values[name] = value

    def resolve_value(self, name: str) -> Optional[ValueInfo]:
        if name in self.values:
            return self.values[name]
        if self.parent:
            return self.parent.resolve_value(name)
        return None

    def resolve_local_value(self, name: str) -> Optional[ValueInfo]:
        return self.values.get(name)

    def define_alias(self, name: str, target: str):
        self.aliases[name] = target

    def resolve_alias_raw(self, name: str) -> Optional[str]:
        if name in self.aliases:
            return self.aliases[name]
        if self.parent:
            return self.parent.resolve_alias_raw(name)
        return None

    def resolve_local_alias(self, name: str) -> Optional[str]:
        return self.aliases.get(name)


def _split_top_level(text: str, sep: str) -> List[str]:
    parts: List[str] = []
    depth_paren = 0
    depth_bracket = 0
    start = 0
    i = 0
    while i < len(text):
        ch = text[i]
        if ch == "(":
            depth_paren += 1
        elif ch == ")":
            depth_paren -= 1
        elif ch == "[":
            depth_bracket += 1
        elif ch == "]":
            depth_bracket -= 1
        elif (
            ch == sep
            and depth_paren == 0
            and depth_bracket == 0
        ):
            parts.append(text[start:i].strip())
            start = i + 1
        i += 1
    parts.append(text[start:].strip())
    return [part for part in parts if part]


def _generic_type_params(type_name: Optional[str], base: str) -> Optional[List[str]]:
    if type_name is None:
        return None
    prefix = f"{base}["
    if not type_name.startswith(prefix) or not type_name.endswith("]"):
        return None
    inner = type_name[len(prefix):-1].strip()
    return _split_top_level(inner, ",")


def _single_return_type_name(return_type: Any) -> Optional[str]:
    if return_type is None or isinstance(return_type, list):
        return None
    return _render_type(return_type)


def _render_type(type_node: Any) -> Optional[str]:
    if type_node is None:
        return None
    if isinstance(type_node, ast.TypeName):
        return type_node.name
    if isinstance(type_node, ast.GenericType):
        params = [_render_type(p) or "?" for p in type_node.params]
        return f"{type_node.base}[{', '.join(params)}]"
    if isinstance(type_node, ast.FuncType):
        params = ", ".join(_render_type(p) or "?" for p in type_node.param_types)
        ret = _render_type(type_node.return_type) or "void"
        return f"({params}) -> {ret}"
    return None


def _return_type_names(return_type: Any) -> Optional[List[str]]:
    if return_type is None:
        return None
    if isinstance(return_type, list):
        return [_render_type(node) for node in return_type if _render_type(node) is not None]
    rendered = _render_type(return_type)
    return [rendered] if rendered is not None else []


class SemanticChecker:
    def __init__(self, module_search_paths: Optional[List[str]] = None):
        self.global_scope = Scope()
        self.modules: Dict[str, ModuleInfo] = {}
        self.module_search_paths: List[str] = []
        self._loading_modules: set[str] = set()
        self._setup_builtins()
        self._setup_stdlib_modules()
        if module_search_paths:
            for path in module_search_paths:
                self.add_module_search_path(path)

    def _setup_builtins(self):
        for name in BUILTIN_RUNTIME_NAMES:
            self.global_scope.define_value(
                name,
                ValueInfo(
                    kind="builtin",
                    return_type_name=BUILTIN_RUNTIME_RETURNS.get(name),
                    callable_info=BUILTIN_SIGNATURES.get(name),
                ),
            )

    def _setup_stdlib_modules(self):
        for module_name, exports in OFFICIAL_STDLIB_MODULES.items():
            module_info = ModuleInfo(name=module_name)
            for export_name in exports:
                value = self.global_scope.resolve_local_value(export_name)
                if value is not None:
                    module_info.runtime_exports[export_name] = value
            self.modules[module_name] = module_info

    def add_module_search_path(self, path: Optional[str]):
        if not path:
            return
        abs_path = os.path.abspath(path)
        if abs_path in self.module_search_paths:
            return
        self.module_search_paths.insert(0, abs_path)

    def _module_candidate_paths(self, module_name: str):
        seen = set()
        search_paths = self.module_search_paths or [os.getcwd()]
        for base in search_paths:
            for candidate in (
                os.path.join(base, f"{module_name}.gw"),
                os.path.join(base, module_name, "main.gw"),
            ):
                abs_candidate = os.path.abspath(candidate)
                if abs_candidate not in seen:
                    seen.add(abs_candidate)
                    yield abs_candidate

    def _extract_file_module_def(
        self,
        program: ast.Program,
        module_name: str,
        candidate: str,
        line: int,
    ) -> ast.ModuleDef:
        """Require module files to contain exactly one top-level matching module definition."""
        if len(program.statements) != 1 or not isinstance(program.statements[0], ast.ModuleDef):
            raise SemanticError(
                f"Module file '{candidate}' must contain exactly one top-level module definition for '{module_name}'",
                line,
            )
        module_stmt = program.statements[0]
        if module_stmt.name != module_name:
            raise SemanticError(
                f"Module file '{candidate}' did not define module '{module_name}'",
                line,
            )
        return module_stmt

    def _validate_module_body(self, stmt: ast.ModuleDef):
        seen_non_use_decl = False
        for inner in stmt.body:
            if isinstance(inner, MODULE_DECLARATION_STMT_TYPES):
                if isinstance(inner, ast.UseStmt):
                    if seen_non_use_decl:
                        raise SemanticError(
                            f"Module '{stmt.name}' must place use statements before func/object/type declarations",
                            inner.line,
                        )
                    continue
                seen_non_use_decl = True
                continue
            raise SemanticError(
                f"Module '{stmt.name}' top level only allows use/func/object/type declarations, got {_module_stmt_label(inner)}",
                getattr(inner, "line", stmt.line),
            )

    def _add_module_runtime_export(
        self,
        module_info: ModuleInfo,
        module_name: str,
        export_name: str,
        value: "ValueInfo",
        export_kind: str,
        line: int,
    ):
        if export_name in module_info.runtime_exports:
            raise SemanticError(
                f"Module '{module_name}' exports runtime name '{export_name}' more than once",
                line,
            )
        module_info.runtime_exports[export_name] = value

    def _add_module_type_export(
        self,
        module_info: ModuleInfo,
        module_name: str,
        export_name: str,
        target: str,
        line: int,
    ):
        if export_name in module_info.type_exports:
            raise SemanticError(
                f"Module '{module_name}' exports type name '{export_name}' more than once",
                line,
            )
        module_info.type_exports[export_name] = target

    def _import_runtime_name(
        self,
        scope: "Scope",
        module_name: str,
        name: str,
        value: "ValueInfo",
        line: int,
    ):
        existing = scope.resolve_local_value(name)
        if existing is not None and existing is not value:
            raise SemanticError(
                f"Cannot import '{name}' from module '{module_name}': name already defined in current scope",
                line,
            )
        scope.define_value(name, value)

    def _import_type_name(
        self,
        scope: "Scope",
        module_name: str,
        name: str,
        target: str,
        line: int,
    ):
        existing = scope.resolve_local_alias(name)
        if existing is not None and existing != target:
            raise SemanticError(
                f"Cannot import type '{name}' from module '{module_name}': type name already defined in current scope",
                line,
            )
        scope.define_alias(name, target)

    def _import_module_namespace(
        self,
        scope: "Scope",
        module_name: str,
        module: ModuleInfo,
        line: int,
    ):
        existing = scope.resolve_local_value(module_name)
        if existing is not None:
            if existing.kind != "module" or existing.module_info is not module:
                raise SemanticError(
                    f"Cannot import module '{module_name}': name already defined in current scope",
                    line,
                )
        scope.define_value(
            module_name,
            ValueInfo(kind="module", module_info=module),
        )

    def _load_module_from_file(
        self,
        module_name: str,
        line: int,
        deferred: List[Callable[[], None]],
    ):
        if module_name in self.modules:
            return
        if module_name in self._loading_modules:
            raise SemanticError(
                f"Cyclic module import detected while loading '{module_name}'",
                line,
            )

        for candidate in self._module_candidate_paths(module_name):
            if not os.path.isfile(candidate):
                continue

            from .parser import parse

            self._loading_modules.add(module_name)
            self.add_module_search_path(os.path.dirname(candidate))
            try:
                with open(candidate, encoding="utf-8") as f:
                    source = f.read()
                program = parse(source)
                module_stmt = self._extract_file_module_def(program, module_name, candidate, line)
                self._check_stmt(module_stmt, self.global_scope, deferred)
            finally:
                self._loading_modules.remove(module_name)

            if module_name not in self.modules:
                raise SemanticError(
                    f"Module file '{candidate}' did not define module '{module_name}'",
                    line,
                )
            return

        raise SemanticError(f"Module not found: {module_name}", line)

    def check_program(self, program: ast.Program, source_path: Optional[str] = None):
        if source_path:
            self.add_module_search_path(os.path.dirname(os.path.abspath(source_path)))
        deferred: List[Callable[[], None]] = []
        self._check_block(program.statements, self.global_scope, deferred)
        self._run_deferred(deferred)

    def _run_deferred(self, deferred: List[Callable[[], None]]):
        while deferred:
            callback = deferred.pop(0)
            callback()

    def _variable_value(
        self,
        type_name: Optional[str],
        scope: Scope,
        callable_label: str = "<func-type>",
    ) -> ValueInfo:
        object_info = None
        callable_info = None
        if type_name is not None:
            object_info = self._try_resolve_object_info(type_name, scope)
            callable_info = self._callable_from_type_name(
                type_name,
                label=callable_label,
                definition_scope=scope,
            )
        return ValueInfo(
            kind="variable",
            type_name=type_name,
            object_info=object_info,
            callable_info=callable_info,
        )

    def _copy_value_info(self, value: ValueInfo, scope: Scope) -> ValueInfo:
        copied = self._variable_value(value.type_name, scope)
        copied.kind = value.kind
        copied.multi_type_names = value.multi_type_names
        copied.return_type_name = value.return_type_name
        copied.object_info = value.object_info
        copied.module_info = value.module_info
        copied.callable_info = value.callable_info or copied.callable_info
        return copied

    def _callable_from_params(
        self,
        label: str,
        params: List[ast.Param],
        return_type_name: Optional[str],
        definition_scope: Optional["Scope"] = None,
    ) -> CallableInfo:
        return CallableInfo(
            label=label,
            params=[
                ParamInfo(
                    name=param.name,
                    type_name=_render_type(param.type_name),
                    has_default=param.default is not None,
                    line=param.line,
                )
                for param in params
            ],
            return_type_name=return_type_name,
            return_type_names=[return_type_name] if return_type_name is not None else None,
            definition_scope=definition_scope,
        )

    def _callable_from_type_name(
        self,
        type_name: Optional[str],
        label: str = "<func-type>",
        definition_scope: Optional["Scope"] = None,
    ) -> Optional[CallableInfo]:
        if type_name is None:
            return None
        stripped = type_name.strip()
        if not stripped.startswith("("):
            return None
        depth_paren = 0
        depth_bracket = 0
        arrow_index = -1
        i = 0
        while i < len(stripped) - 1:
            ch = stripped[i]
            if ch == "(":
                depth_paren += 1
            elif ch == ")":
                depth_paren -= 1
            elif ch == "[":
                depth_bracket += 1
            elif ch == "]":
                depth_bracket -= 1
            elif (
                ch == "-"
                and stripped[i + 1] == ">"
                and depth_paren == 0
                and depth_bracket == 0
            ):
                arrow_index = i
                break
            i += 1
        if arrow_index == -1:
            return None
        params_part = stripped[:arrow_index].strip()
        return_part = stripped[arrow_index + 2:].strip()
        if not params_part.startswith("(") or not params_part.endswith(")"):
            return None
        inner = params_part[1:-1].strip()
        param_types = _split_top_level(inner, ",") if inner else []
        return CallableInfo(
            label=label,
            params=[
                ParamInfo(name=f"arg{idx + 1}", type_name=param_type)
                for idx, param_type in enumerate(param_types)
            ],
            return_type_name=return_part or None,
            return_type_names=[return_part] if return_part else None,
            definition_scope=definition_scope,
        )

    def _callable_from_return_annotation(
        self,
        label: str,
        params: List[ast.Param],
        return_type: Any,
        definition_scope: Optional["Scope"] = None,
    ) -> CallableInfo:
        return_type_names = _return_type_names(return_type) or None
        return_type_name = return_type_names[0] if return_type_names and len(return_type_names) == 1 else None
        info = self._callable_from_params(
            label,
            params,
            return_type_name,
            definition_scope=definition_scope,
        )
        info.return_type_names = return_type_names
        return info

    def _infer_lambda_callable_info(
        self,
        expr: ast.Lambda,
        scope: Scope,
        deferred: List[Callable[[], None]],
    ) -> CallableInfo:
        callable_info = self._callable_from_params(
            "<lambda>",
            expr.params,
            None,
            definition_scope=scope,
        )
        if len(expr.body) != 1:
            return callable_info
        only_stmt = expr.body[0]
        if not isinstance(only_stmt, ast.ReturnStmt) or only_stmt.value is None or isinstance(only_stmt.value, list):
            return callable_info
        lambda_scope = Scope(parent=scope)
        for param in expr.params:
            rendered = _render_type(param.type_name)
            if rendered is not None:
                rendered = self._resolve_type_name_string(rendered, scope, param.line)
            lambda_scope.define_value(
                param.name,
                self._variable_value(rendered, scope, callable_label=param.name),
            )
        inferred = self._check_expr(only_stmt.value, lambda_scope, deferred)
        inferred_type = self._value_type_name(inferred, lambda_scope)
        if inferred_type is not None:
            callable_info.return_type_name = inferred_type
            callable_info.return_type_names = [inferred_type]
        return callable_info

    def _bind_declared_value(
        self,
        name: str,
        declared_type: Optional[str],
        inferred: Optional[ValueInfo],
        scope: Scope,
        line: int,
    ) -> ValueInfo:
        if declared_type is not None:
            value = self._variable_value(declared_type, scope, callable_label=name)
            if inferred is not None:
                self._validate_value_assignment(
                    target_name=name,
                    expected_value=value,
                    actual_value=inferred,
                    scope=scope,
                    line=line,
                )
                if inferred.object_info is not None and value.object_info is None:
                    value.object_info = inferred.object_info
                if inferred.callable_info is not None and value.callable_info is None:
                    value.callable_info = inferred.callable_info
            return value
        if inferred is not None:
            return self._copy_value_info(inferred, scope)
        return self._variable_value(None, scope)

    def _validate_value_assignment(
        self,
        target_name: str,
        expected_value: ValueInfo,
        actual_value: ValueInfo,
        scope: Scope,
        line: int,
    ):
        expected_type = self._value_type_name(expected_value, scope)
        actual_type = self._value_type_name(actual_value, scope)
        if expected_type is None or actual_type is None:
            return
        if self._types_compatible(expected_type, actual_type):
            return
        raise SemanticError(
            f"Cannot assign {actual_type} to '{target_name}' ({expected_type})",
            line,
        )

    def _check_block(
        self,
        stmts: List[Any],
        scope: Scope,
        deferred: List[Callable[[], None]],
    ):
        for stmt in stmts:
            self._check_stmt(stmt, scope, deferred)

    def _check_stmt(
        self,
        stmt: Any,
        scope: Scope,
        deferred: List[Callable[[], None]],
    ):
        if isinstance(stmt, ast.FuncDef):
            signature = self._callable_from_return_annotation(
                stmt.name,
                stmt.params,
                stmt.return_type,
                definition_scope=scope,
            )
            scope.define_value(
                stmt.name,
                ValueInfo(
                    kind="function",
                    type_name=(
                        self._value_type_name(
                            ValueInfo(callable_info=signature, kind="function"),
                            scope,
                        )
                        if not signature.return_type_names or len(signature.return_type_names) == 1
                        else None
                    ),
                    return_type_name=signature.return_type_name,
                    multi_type_names=signature.return_type_names if signature.return_type_names and len(signature.return_type_names) > 1 else None,
                    callable_info=signature,
                ),
            )
            deferred.append(lambda stmt=stmt, scope=scope: self._check_function_body(stmt, scope))
            return

        if isinstance(stmt, ast.Assignment):
            values = [self._check_expr(v, scope, deferred) for v in stmt.values]
            assigned_values = values
            if len(stmt.targets) > 1 and len(values) == 1 and values[0].multi_type_names is not None:
                assigned_values = [
                    self._variable_value(type_name, scope, callable_label=f"return{idx + 1}")
                    for idx, type_name in enumerate(values[0].multi_type_names)
                ]
            if len(stmt.targets) != len(assigned_values):
                raise SemanticError(
                    f"Assignment count mismatch: {len(stmt.targets)} targets, {len(assigned_values)} values",
                    stmt.line,
                )
            for target, actual_value in zip(stmt.targets, assigned_values):
                if isinstance(target, str):
                    existing = scope.resolve_local_value(target)
                    if existing is not None:
                        if existing.kind == "builtin":
                            scope.define_value(target, self._copy_value_info(actual_value, scope))
                            continue
                        self._validate_value_assignment(target, existing, actual_value, scope, stmt.line)
                        continue
                    scope.define_value(target, self._copy_value_info(actual_value, scope))
                else:
                    self._check_assignment_target(target, scope, deferred, stmt.line)
            return

        if isinstance(stmt, ast.VarDecl):
            if stmt.type_name is not None:
                self._resolve_type_node(stmt.type_name, scope, stmt.line)
            inferred = None
            if stmt.value is not None:
                inferred = self._check_expr(stmt.value, scope, deferred)
            declared_type = _render_type(stmt.type_name)
            if declared_type:
                declared_type = self._resolve_type_name_string(declared_type, scope, stmt.line)
            value = self._bind_declared_value(stmt.name, declared_type, inferred, scope, stmt.line)
            scope.define_value(stmt.name, value)
            return

        if isinstance(stmt, ast.VarBlock):
            if stmt.default_value is not None:
                self._check_expr(stmt.default_value, scope, deferred)
            for decl in stmt.decls:
                if decl.type_name is not None:
                    self._resolve_type_node(decl.type_name, scope, decl.line)
                inferred = None
                if decl.value is not None:
                    inferred = self._check_expr(decl.value, scope, deferred)
                declared_type = _render_type(decl.type_name)
                if declared_type:
                    declared_type = self._resolve_type_name_string(declared_type, scope, decl.line)
                value = self._bind_declared_value(decl.name, declared_type, inferred, scope, decl.line)
                scope.define_value(decl.name, value)
            return

        if isinstance(stmt, ast.TypeAlias):
            target = _render_type(stmt.target)
            if target is None:
                raise SemanticError(f"Invalid type alias target for '{stmt.name}'", stmt.line)
            scope.define_alias(stmt.name, target)
            deferred.append(lambda stmt=stmt, scope=scope: self._validate_alias(stmt, scope))
            return

        if isinstance(stmt, ast.ReturnStmt):
            if stmt.value is None:
                self._validate_return_stmt([], scope, stmt.line)
                return
            if isinstance(stmt.value, list):
                values = [self._check_expr(value, scope, deferred) for value in stmt.value]
                self._validate_return_stmt(values, scope, stmt.line)
            else:
                value = self._check_expr(stmt.value, scope, deferred)
                self._validate_return_stmt([value], scope, stmt.line)
            return

        if isinstance(stmt, ast.IfStmt):
            self._check_expr(stmt.condition, scope, deferred)
            then_scope = Scope(parent=scope)
            self._check_block(stmt.body, then_scope, deferred)
            for cond, body in stmt.elifs:
                self._check_expr(cond, scope, deferred)
                elif_scope = Scope(parent=scope)
                self._check_block(body, elif_scope, deferred)
            if stmt.else_body:
                else_scope = Scope(parent=scope)
                self._check_block(stmt.else_body, else_scope, deferred)
            return

        if isinstance(stmt, ast.WhileStmt):
            self._check_expr(stmt.condition, scope, deferred)
            loop_scope = Scope(parent=scope)
            self._check_block(stmt.body, loop_scope, deferred)
            return

        if isinstance(stmt, ast.ForRangeStmt):
            self._check_expr(stmt.start, scope, deferred)
            self._check_expr(stmt.end, scope, deferred)
            if stmt.step is not None:
                self._check_expr(stmt.step, scope, deferred)
            loop_scope = Scope(parent=scope)
            loop_scope.define_value(stmt.var, self._variable_value(None, loop_scope))
            self._check_block(stmt.body, loop_scope, deferred)
            return

        if isinstance(stmt, ast.ForEachStmt):
            self._check_expr(stmt.iterable, scope, deferred)
            loop_scope = Scope(parent=scope)
            loop_scope.define_value(stmt.var, self._variable_value(None, loop_scope))
            if stmt.index_var:
                loop_scope.define_value(stmt.index_var, self._variable_value("int", loop_scope))
            self._check_block(stmt.body, loop_scope, deferred)
            return

        if isinstance(stmt, ast.MatchStmt):
            self._check_expr(stmt.subject, scope, deferred)
            for case in stmt.cases:
                case_scope = Scope(parent=scope)
                for pattern in case.patterns:
                    self._check_pattern(pattern, case_scope, deferred)
                self._check_block(case.body, case_scope, deferred)
            if stmt.else_body:
                else_scope = Scope(parent=scope)
                self._check_block(stmt.else_body, else_scope, deferred)
            return

        if isinstance(stmt, ast.ModuleDef):
            self._validate_module_body(stmt)
            module_scope = Scope(parent=scope)
            self._check_block(stmt.body, module_scope, deferred)
            module_info = ModuleInfo(name=stmt.name)
            for inner in stmt.body:
                if isinstance(inner, ast.FuncDef) and inner.exported:
                    value = module_scope.resolve_local_value(inner.name)
                    if value is not None:
                        self._add_module_runtime_export(
                            module_info,
                            stmt.name,
                            inner.name,
                            value,
                            "func",
                            inner.line,
                        )
                elif isinstance(inner, ast.ObjectDef) and inner.exported:
                    value = module_scope.resolve_local_value(inner.name)
                    if value is not None:
                        self._add_module_runtime_export(
                            module_info,
                            stmt.name,
                            inner.name,
                            value,
                            "object",
                            inner.line,
                        )
                elif isinstance(inner, ast.TypeAlias) and inner.exported:
                    self._add_module_type_export(
                        module_info,
                        stmt.name,
                        inner.name,
                        self._resolve_alias_name(inner.name, module_scope, inner.line),
                        inner.line,
                    )
            info = ValueInfo(kind="module", module_info=module_info)
            scope.define_value(stmt.name, info)
            self.modules[stmt.name] = module_info
            return

        if isinstance(stmt, ast.UseStmt):
            if stmt.module not in self.modules:
                self._load_module_from_file(stmt.module, stmt.line, deferred)
            module = self.modules[stmt.module]
            if stmt.names:
                for name in stmt.names:
                    imported = False
                    if name in module.runtime_exports:
                        self._import_runtime_name(scope, stmt.module, name, module.runtime_exports[name], stmt.line)
                        imported = True
                    if name in module.type_exports:
                        self._import_type_name(scope, stmt.module, name, module.type_exports[name], stmt.line)
                        imported = True
                    if not imported:
                        raise SemanticError(
                            f"Module '{stmt.module}' does not export '{name}'",
                            stmt.line,
                        )
            else:
                self._import_module_namespace(scope, stmt.module, module, stmt.line)
            return

        if isinstance(stmt, ast.GlobalStmt):
            self._check_expr(stmt.value, scope, deferred)
            outer = scope.parent
            while outer:
                if outer.resolve_local_value(stmt.name) is not None:
                    return
                outer = outer.parent
            raise SemanticError(
                f"global variable '{stmt.name}' not found in any outer scope",
                stmt.line,
            )

        if isinstance(stmt, ast.ArenaStmt):
            arena_scope = Scope(parent=scope)
            self._check_block(stmt.body, arena_scope, deferred)
            return

        if isinstance(stmt, ast.ParallelStmt):
            parallel_scope = Scope(parent=scope)
            self._check_block(stmt.body, parallel_scope, deferred)
            if stmt.result_var:
                scope.define_value(stmt.result_var, self._variable_value("list", scope))
            return

        if isinstance(stmt, ast.TagStmt):
            return

        if isinstance(stmt, ast.ExprStmt):
            self._check_expr(stmt.expr, scope, deferred)
            return

        if isinstance(stmt, ast.ObjectDef):
            obj_info = ObjectInfo(name=stmt.name, has_constructor=stmt.constructor is not None)
            scope.define_value(
                stmt.name,
                ValueInfo(kind="object", type_name=stmt.name, object_info=obj_info),
            )

            for field in stmt.fields:
                field_type = None
                if field.type_annotation is not None:
                    self._resolve_type_node(field.type_annotation, scope, field.line)
                    rendered = _render_type(field.type_annotation)
                    if rendered is not None:
                        field_type = self._resolve_type_name_string(rendered, scope, field.line)
                obj_info.fields[field.name] = field_type

            if stmt.constructor is not None:
                ctor_return = _render_type(stmt.constructor.return_type) if stmt.constructor.return_type is not None else None
                if ctor_return is not None and ctor_return != stmt.name:
                    raise SemanticError(
                        f"Constructor '{stmt.name}.new' must return '{stmt.name}'",
                        stmt.constructor.line,
                    )
                obj_info.constructor = self._callable_from_params(
                    f"{stmt.name}.new",
                    stmt.constructor.params,
                    stmt.name,
                    definition_scope=scope,
                )
                deferred.append(
                    lambda ctor=stmt.constructor, scope=scope, obj_info=obj_info:
                    self._check_constructor_body(ctor, obj_info, scope)
                )

            for method in stmt.methods:
                obj_info.methods[method.name] = self._callable_from_return_annotation(
                    f"{obj_info.name}.{method.name}",
                    method.params,
                    method.return_type,
                    definition_scope=scope,
                )
                deferred.append(
                    lambda method=method, scope=scope, obj_info=obj_info:
                    self._check_method_body(method, obj_info, scope)
                )
            return

        raise SemanticError(f"Unknown statement type: {type(stmt).__name__}", getattr(stmt, "line", 0))

    def _validate_alias(self, stmt: ast.TypeAlias, scope: Scope):
        self._resolve_alias_name(stmt.name, scope, stmt.line)

    def _check_function_body(self, stmt: ast.FuncDef, closure_scope: Scope):
        self._validate_return_annotation(stmt.return_type, closure_scope, stmt.line)
        expected_return_types = self._resolve_return_type_names(stmt.return_type, closure_scope, stmt.line)
        body_scope = Scope(parent=closure_scope, expected_return_types=expected_return_types)
        for param in stmt.params:
            if param.type_name is not None:
                self._resolve_type_node(param.type_name, closure_scope, param.line)
            if param.default is not None:
                self._check_expr(param.default, closure_scope, [])
            rendered = _render_type(param.type_name)
            if rendered is not None:
                rendered = self._resolve_type_name_string(rendered, closure_scope, param.line)
            body_scope.define_value(
                param.name,
                self._variable_value(rendered, closure_scope, callable_label=param.name),
            )
        body_deferred: List[Callable[[], None]] = []
        self._check_block(stmt.body, body_scope, body_deferred)
        self._run_deferred(body_deferred)

    def _check_method_body(self, method: ast.MethodDef, obj_info: ObjectInfo, closure_scope: Scope):
        if not method.params:
            raise SemanticError(
                f"Method '{obj_info.name}.{method.name}' must declare 'self' as first parameter",
                method.line,
            )
        first = method.params[0]
        if first.name != "self":
            raise SemanticError(
                f"Method '{obj_info.name}.{method.name}' must declare 'self' as first parameter",
                first.line,
            )
        if _render_type(first.type_name) != obj_info.name:
            raise SemanticError(
                f"Method '{obj_info.name}.{method.name}' must declare 'self: {obj_info.name}'",
                first.line,
            )
        self._validate_return_annotation(method.return_type, closure_scope, method.line)
        expected_return_types = self._resolve_return_type_names(method.return_type, closure_scope, method.line)
        body_scope = Scope(
            parent=closure_scope,
            method_self_type=obj_info.name,
            expected_return_types=expected_return_types,
        )
        body_scope.define_value("self", self._variable_value(obj_info.name, closure_scope, callable_label="self"))
        for param in method.params[1:]:
            if param.type_name is not None:
                self._resolve_type_node(param.type_name, closure_scope, param.line)
            if param.default is not None:
                self._check_expr(param.default, closure_scope, [])
            rendered = _render_type(param.type_name)
            if rendered is not None:
                rendered = self._resolve_type_name_string(rendered, closure_scope, param.line)
            body_scope.define_value(
                param.name,
                self._variable_value(rendered, closure_scope, callable_label=param.name),
            )
        body_deferred: List[Callable[[], None]] = []
        self._check_block(method.body, body_scope, body_deferred)
        self._run_deferred(body_deferred)

    def _check_constructor_body(self, ctor: ast.ConstructorDef, obj_info: ObjectInfo, closure_scope: Scope):
        if ctor.return_type is not None:
            self._resolve_type_node(ctor.return_type, closure_scope, ctor.line)
        expected_return_types = [obj_info.name]
        body_scope = Scope(parent=closure_scope, expected_return_types=expected_return_types)
        for param in ctor.params:
            if param.type_name is not None:
                self._resolve_type_node(param.type_name, closure_scope, param.line)
            if param.default is not None:
                self._check_expr(param.default, closure_scope, [])
            rendered = _render_type(param.type_name)
            if rendered is not None:
                rendered = self._resolve_type_name_string(rendered, closure_scope, param.line)
            body_scope.define_value(
                param.name,
                self._variable_value(rendered, closure_scope, callable_label=param.name),
            )
        body_deferred: List[Callable[[], None]] = []
        self._check_block(ctor.body, body_scope, body_deferred)
        self._run_deferred(body_deferred)

    def _validate_return_annotation(self, return_type: Any, scope: Scope, line: int):
        if return_type is None:
            return
        if isinstance(return_type, list):
            for node in return_type:
                self._resolve_type_node(node, scope, line)
            return
        self._resolve_type_node(return_type, scope, line)

    def _resolve_return_type_names(
        self,
        return_type: Any,
        scope: Scope,
        line: int,
    ) -> Optional[List[str]]:
        rendered = _return_type_names(return_type)
        if rendered is None:
            return None
        return [
            self._resolve_type_name_string(name, scope, line)
            for name in rendered
        ]

    def _value_type_name(self, value: ValueInfo, scope: Scope) -> Optional[str]:
        if value.type_name is not None:
            return self._normalize_type_name(value.type_name, scope)
        if value.callable_info is not None:
            params = ", ".join(
                self._resolve_callable_type_name(param.type_name, value.callable_info) or "?"
                for param in value.callable_info.params
            )
            ret = self._resolve_callable_type_name(
                value.callable_info.return_type_name,
                value.callable_info,
            ) or "void"
            return f"({params}) -> {ret}"
        return None

    def _validate_return_stmt(
        self,
        values: List[ValueInfo],
        scope: Scope,
        line: int,
    ):
        expected = scope.expected_return_types
        if expected is None:
            return
        if len(values) != len(expected):
            raise SemanticError(
                f"Return value count mismatch: expected {len(expected)}, got {len(values)}",
                line,
            )
        for idx, (value, expected_type) in enumerate(zip(values, expected), start=1):
            actual_type = self._value_type_name(value, scope)
            if actual_type is None:
                continue
            if self._types_compatible(expected_type, actual_type):
                continue
            if len(expected) == 1:
                raise SemanticError(
                    f"Return type mismatch: expected {expected_type}, got {actual_type}",
                    line,
                )
            raise SemanticError(
                f"Return value {idx} expects {expected_type}, got {actual_type}",
                line,
            )

    def _check_pattern(
        self,
        pattern: Any,
        scope: Scope,
        deferred: List[Callable[[], None]],
    ):
        if isinstance(pattern, ast.OkExpr):
            if isinstance(pattern.value, ast.Identifier):
                scope.define_value(pattern.value.name, self._variable_value(None, scope))
            else:
                self._check_expr(pattern.value, scope, deferred)
            return
        if isinstance(pattern, ast.ErrExpr):
            if isinstance(pattern.value, ast.Identifier):
                scope.define_value(pattern.value.name, self._variable_value(None, scope))
            else:
                self._check_expr(pattern.value, scope, deferred)
            return
        self._check_expr(pattern, scope, deferred)

    def _check_assignment_target(
        self,
        target: Any,
        scope: Scope,
        deferred: List[Callable[[], None]],
        line: int,
    ):
        if isinstance(target, ast.IndexAccess):
            self._check_expr(target.obj, scope, deferred)
            self._check_expr(target.index, scope, deferred)
            return
        if isinstance(target, ast.MemberAccess):
            obj = self._check_expr(target.obj, scope, deferred)
            self._check_member_access(obj, target, scope, allow_field_write=True)
            return
        raise SemanticError("Invalid assignment target", line)

    def _validate_call_signature(
        self,
        callee: ValueInfo,
        args: List[ValueInfo],
        scope: Scope,
        line: int,
    ):
        signature = callee.callable_info
        if signature is None:
            return

        params = signature.params
        args_to_check = args
        params_to_check = params

        if callee.kind == "bound_method":
            params_to_check = params[1:]
        elif callee.kind == "static_method":
            if not args:
                raise SemanticError(
                    f"Method '{signature.label}' requires explicit self as first argument",
                    line,
                )
            receiver_param = params[0] if params else None
            if receiver_param is not None and receiver_param.type_name is not None:
                receiver_type = self._resolve_callable_type_name(receiver_param.type_name, signature)
                actual_type = self._normalize_type_name(args[0].type_name, scope)
                if receiver_type is not None and actual_type is not None and actual_type != receiver_type:
                    raise SemanticError(
                        f"First argument to '{signature.label}' must be a '{receiver_type}' instance",
                        line,
                    )
            args_to_check = args[1:]
            params_to_check = params[1:]

        if not signature.variadic:
            min_args = sum(1 for param in params_to_check if not param.has_default)
            max_args = len(params_to_check)
            if len(args_to_check) < min_args:
                consumed = len(args_to_check)
                for param in params_to_check:
                    if consumed > 0:
                        consumed -= 1
                        continue
                    if param.has_default:
                        continue
                    raise SemanticError(f"Missing argument: {param.name}", line)
            if len(args_to_check) > max_args:
                raise SemanticError(
                    f"Too many arguments for '{signature.label}': expected at most {max_args}, got {len(args_to_check)}",
                    line,
                )

        for arg, param in zip(args_to_check, params_to_check):
            self._validate_argument_type(signature, param, arg, scope, line)
        self._validate_builtin_container_types(signature, args_to_check, line, scope)

    def _validate_argument_type(
        self,
        signature: CallableInfo,
        param: ParamInfo,
        arg: ValueInfo,
        scope: Scope,
        line: int,
    ):
        if param.type_name is None or arg.type_name is None:
            return
        expected = self._resolve_callable_type_name(param.type_name, signature)
        actual = self._normalize_type_name(arg.type_name, scope)
        if expected is None or actual is None:
            return
        if self._types_compatible(expected, actual):
            return
        raise SemanticError(
            f"Argument '{param.name}' to '{signature.label}' expects {expected}, got {actual}",
            line,
        )

    def _resolve_callable_type_name(
        self,
        type_name: Optional[str],
        signature: CallableInfo,
    ) -> Optional[str]:
        if type_name is None:
            return None
        type_scope = signature.definition_scope or self.global_scope
        return self._resolve_type_name_string(type_name, type_scope, 0, strict=False)

    def _normalize_type_name(self, type_name: Optional[str], scope: Scope) -> Optional[str]:
        if type_name is None:
            return None
        return self._resolve_type_name_string(type_name, scope, 0, strict=False) or type_name

    def _validate_builtin_container_types(
        self,
        signature: CallableInfo,
        args: List[ValueInfo],
        line: int,
        scope: Scope,
    ):
        label = signature.label
        if label in ("append", "insert") and args:
            list_type = self._value_type_name(args[0], scope)
            list_params = _generic_type_params(list_type, "list")
            if list_params and len(list_params) == 1:
                item_arg = args[1] if label == "append" else args[2]
                item_type = self._value_type_name(item_arg, scope)
                if item_type is not None and not self._types_compatible(list_params[0], item_type):
                    raise SemanticError(
                        f"Argument '{signature.params[-1].name}' to '{label}' expects {list_params[0]}, got {item_type}",
                        line,
                    )
        elif label in ("haskey", "get") and args:
            dict_type = self._value_type_name(args[0], scope)
            dict_params = _generic_type_params(dict_type, "dict")
            if dict_params and len(dict_params) == 2:
                key_type = self._value_type_name(args[1], scope) if len(args) > 1 else None
                if key_type is not None and not self._types_compatible(dict_params[0], key_type):
                    raise SemanticError(
                        f"Argument '{signature.params[1].name}' to '{label}' expects {dict_params[0]}, got {key_type}",
                        line,
                    )
                if label == "get" and len(args) > 2:
                    default_type = self._value_type_name(args[2], scope)
                    if default_type is not None and not self._types_compatible(dict_params[1], default_type):
                        raise SemanticError(
                            f"Argument '{signature.params[2].name}' to '{label}' expects {dict_params[1]}, got {default_type}",
                            line,
                        )
        elif label in ("map", "filter") and len(args) == 2:
            list_type = self._value_type_name(args[0], scope)
            list_params = _generic_type_params(list_type, "list")
            callback = args[1]
            callback_info = callback.callable_info
            if callback_info is None:
                return
            if len(callback_info.params) != 1:
                raise SemanticError(
                    f"Argument '{signature.params[1].name}' to '{label}' must accept exactly 1 argument",
                    line,
                )
            if list_params and len(list_params) == 1:
                callback_param_type = self._resolve_callable_type_name(
                    callback_info.params[0].type_name,
                    callback_info,
                )
                if (
                    callback_param_type is not None
                    and not self._types_compatible(callback_param_type, list_params[0])
                ):
                    raise SemanticError(
                        f"Argument '{signature.params[1].name}' to '{label}' expects ({list_params[0]}) -> ..., got {self._value_type_name(callback, scope)}",
                        line,
                    )
            if label == "filter":
                callback_return = None
                if callback_info.return_type_names:
                    callback_return = callback_info.return_type_names[0] if len(callback_info.return_type_names) == 1 else None
                elif callback_info.return_type_name is not None:
                    callback_return = callback_info.return_type_name
                callback_return = self._resolve_callable_type_name(callback_return, callback_info)
                if callback_return is not None and callback_return != "bool":
                    raise SemanticError(
                        f"Argument '{signature.params[1].name}' to 'filter' must return bool, got {callback_return}",
                        line,
                    )

    def _resolve_call_return_type_names(
        self,
        callee: ValueInfo,
        scope: Scope,
    ) -> Optional[List[str]]:
        if callee.callable_info is not None and callee.callable_info.return_type_names is not None:
            return [
                self._resolve_callable_type_name(type_name, callee.callable_info) or type_name
                for type_name in callee.callable_info.return_type_names
            ]
        if callee.return_type_name is None:
            return None
        resolved = self._resolve_type_name_string(callee.return_type_name, scope, 0, strict=False)
        return [resolved or callee.return_type_name]

    def _infer_builtin_return_value(
        self,
        callee: ValueInfo,
        args: List[ValueInfo],
        scope: Scope,
    ) -> Optional[ValueInfo]:
        if callee.kind != "builtin" or callee.callable_info is None:
            return None
        label = callee.callable_info.label
        if label == "range":
            return ValueInfo(kind="literal", type_name="list[int]")
        if label == "filter" and args:
            list_type = self._value_type_name(args[0], scope)
            if list_type is not None:
                return ValueInfo(kind="literal", type_name=list_type)
            return ValueInfo(kind="literal", type_name="list")
        if label == "map" and len(args) == 2:
            callback = args[1]
            if callback.callable_info is not None:
                callback_return = None
                if callback.callable_info.return_type_names:
                    callback_return = callback.callable_info.return_type_names[0] if len(callback.callable_info.return_type_names) == 1 else None
                elif callback.callable_info.return_type_name is not None:
                    callback_return = callback.callable_info.return_type_name
                callback_return = self._resolve_callable_type_name(callback_return, callback.callable_info)
                if callback_return is not None:
                    return ValueInfo(kind="literal", type_name=f"list[{callback_return}]")
            return ValueInfo(kind="literal", type_name="list")
        if label == "enumerate":
            return ValueInfo(kind="literal", type_name="list")
        return None

    def _types_compatible(self, expected: str, actual: str) -> bool:
        if expected == actual:
            return True

        expected_base = expected.split("[", 1)[0] if "[" in expected else expected
        actual_base = actual.split("[", 1)[0] if "[" in actual else actual

        int_types = {"int", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64"}
        float_types = {"float", "float32", "float64"}

        if expected in int_types and actual in int_types:
            return True
        if expected in float_types and (actual in int_types or actual in float_types):
            return True
        if expected.startswith("money["):
            if actual.startswith("money["):
                return expected == actual
            return actual in int_types or actual in float_types
        if expected_base in ("list", "dict", "result") and actual_base == expected_base:
            if "[" in expected or "[" in actual:
                if expected == actual_base or actual == expected_base:
                    return True
                return expected == actual
            return True
        if expected.startswith("(") and "->" in expected:
            return expected == actual
        return False

    def _check_expr(
        self,
        expr: Any,
        scope: Scope,
        deferred: List[Callable[[], None]],
    ) -> ValueInfo:
        if isinstance(expr, ast.IntLiteral):
            return ValueInfo(kind="literal", type_name="int")
        if isinstance(expr, ast.FloatLiteral):
            return ValueInfo(kind="literal", type_name="float")
        if isinstance(expr, ast.StringLiteral):
            return ValueInfo(kind="literal", type_name="string")
        if isinstance(expr, ast.BoolLiteral):
            return ValueInfo(kind="literal", type_name="bool")

        if isinstance(expr, ast.Identifier):
            value = scope.resolve_value(expr.name)
            if value is None:
                raise SemanticError(f"Undefined variable: {expr.name}", expr.line)
            return value

        if isinstance(expr, ast.ListLiteral):
            element_infos = [self._check_expr(element, scope, deferred) for element in expr.elements]
            element_type = None
            homogeneous = True
            for info in element_infos:
                current = self._value_type_name(info, scope)
                if current is None:
                    homogeneous = False
                    break
                if element_type is None:
                    element_type = current
                    continue
                if current != element_type:
                    homogeneous = False
                    break
            if homogeneous and element_type is not None:
                return ValueInfo(kind="literal", type_name=f"list[{element_type}]")
            return ValueInfo(kind="literal", type_name="list")

        if isinstance(expr, ast.DictLiteral):
            key_type = self._resolve_type_node(expr.key_type, scope, expr.line)
            value_type = self._resolve_type_node(expr.value_type, scope, expr.line)
            for key_expr, val_expr in expr.entries:
                key_info = self._check_expr(key_expr, scope, deferred)
                val_info = self._check_expr(val_expr, scope, deferred)
                actual_key_type = self._value_type_name(key_info, scope)
                actual_value_type = self._value_type_name(val_info, scope)
                if actual_key_type is not None and not self._types_compatible(key_type, actual_key_type):
                    raise SemanticError(
                        f"Dict key expects {key_type}, got {actual_key_type}",
                        expr.line,
                    )
                if actual_value_type is not None and not self._types_compatible(value_type, actual_value_type):
                    raise SemanticError(
                        f"Dict value expects {value_type}, got {actual_value_type}",
                        expr.line,
                    )
            return ValueInfo(kind="literal", type_name=f"dict[{key_type}, {value_type}]")

        if isinstance(expr, ast.BinaryOp):
            left = self._check_expr(expr.left, scope, deferred)
            right = self._check_expr(expr.right, scope, deferred)
            if expr.op in ("=", "!=", "<", ">", "<=", ">=", "and", "or"):
                return ValueInfo(kind="literal", type_name="bool")
            left_type = self._value_type_name(left, scope)
            right_type = self._value_type_name(right, scope)
            if left_type is not None and right_type is not None:
                int_types = {"int", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64"}
                float_types = {"float", "float32", "float64"}
                if expr.op in ("+", "-", "*", "/", "mod", "^"):
                    if expr.op == "+" and left_type == right_type == "string":
                        return ValueInfo(kind="literal", type_name="string")
                    if expr.op == "+" and left_type.startswith("list") and right_type.startswith("list"):
                        return ValueInfo(kind="literal", type_name=left_type)
                    if left_type.startswith("money[") and right_type.startswith("money[") and expr.op in ("+", "-"):
                        return ValueInfo(kind="literal", type_name=left_type)
                    if left_type.startswith("money[") and right_type in int_types.union(float_types):
                        return ValueInfo(kind="literal", type_name=left_type)
                    if left_type in int_types and right_type in int_types:
                        return ValueInfo(kind="literal", type_name="int")
                    if (
                        (left_type in int_types or left_type in float_types)
                        and (right_type in int_types or right_type in float_types)
                    ):
                        return ValueInfo(kind="literal", type_name="float")
            return UNKNOWN_VALUE

        if isinstance(expr, ast.UnaryOp):
            self._check_expr(expr.operand, scope, deferred)
            if expr.op == "not":
                return ValueInfo(kind="literal", type_name="bool")
            return UNKNOWN_VALUE

        if isinstance(expr, ast.FuncCall):
            callee = self._check_expr(expr.name, scope, deferred)
            arg_infos = [self._check_expr(arg, scope, deferred) for arg in expr.args]
            self._validate_call_signature(callee, arg_infos, scope, expr.line)
            if callee.kind == "constructor":
                return ValueInfo(
                    kind="variable",
                    type_name=callee.return_type_name,
                    object_info=callee.object_info,
                    callable_info=callee.callable_info,
                )
            inferred_builtin = self._infer_builtin_return_value(callee, arg_infos, scope)
            if inferred_builtin is not None:
                return inferred_builtin
            return_type_names = self._resolve_call_return_type_names(callee, scope)
            if return_type_names is not None and len(return_type_names) > 1:
                return ValueInfo(kind="multi", multi_type_names=return_type_names)
            return_type_name = return_type_names[0] if return_type_names else None
            if return_type_name is not None:
                return self._variable_value(return_type_name, scope, callable_label=callee.callable_info.label if callee.callable_info is not None else "<func-type>")
            return UNKNOWN_VALUE

        if isinstance(expr, ast.MemberAccess):
            obj = self._check_expr(expr.obj, scope, deferred)
            return self._check_member_access(obj, expr, scope, allow_field_write=False)

        if isinstance(expr, ast.IndexAccess):
            self._check_expr(expr.obj, scope, deferred)
            self._check_expr(expr.index, scope, deferred)
            return UNKNOWN_VALUE

        if isinstance(expr, ast.Lambda):
            for param in expr.params:
                if param.type_name is not None:
                    self._resolve_type_node(param.type_name, scope, param.line)
            callable_info = self._infer_lambda_callable_info(expr, scope, deferred)
            deferred.append(lambda expr=expr, scope=scope: self._check_lambda_body(expr, scope))
            return ValueInfo(
                kind="lambda",
                callable_info=callable_info,
            )

        if isinstance(expr, ast.OkExpr):
            self._check_expr(expr.value, scope, deferred)
            return ValueInfo(kind="literal", type_name="result")

        if isinstance(expr, ast.ErrExpr):
            self._check_expr(expr.value, scope, deferred)
            return ValueInfo(kind="literal", type_name="result")

        if isinstance(expr, ast.AsExpr):
            self._check_expr(expr.expr, scope, deferred)
            self._resolve_type_name_string(expr.type_name, scope, expr.line)
            return ValueInfo(kind="literal", type_name="result")

        if isinstance(expr, ast.ObjectLiteral):
            obj_info = self._resolve_object_info(expr.name, scope, expr.line)
            provided = set()
            for field_name, field_expr in expr.fields:
                if field_name in provided:
                    raise SemanticError(
                        f"Duplicate field '{field_name}' in '{expr.name}' literal",
                        expr.line,
                    )
                if field_name not in obj_info.fields:
                    raise SemanticError(
                        f"Object '{expr.name}' has no field '{field_name}'",
                        expr.line,
                    )
                provided.add(field_name)
                self._check_expr(field_expr, scope, deferred)
            missing = [name for name in obj_info.fields if name not in provided]
            if missing:
                raise SemanticError(
                    f"Object '{expr.name}' literal missing fields: {', '.join(missing)}",
                    expr.line,
                )
            return ValueInfo(kind="variable", type_name=obj_info.name, object_info=obj_info)

        raise SemanticError(f"Unknown expression type: {type(expr).__name__}", getattr(expr, "line", 0))

    def _check_lambda_body(self, expr: ast.Lambda, closure_scope: Scope):
        body_scope = Scope(parent=closure_scope)
        body_scope.expected_return_types = None
        for param in expr.params:
            if param.type_name is not None:
                self._resolve_type_node(param.type_name, closure_scope, param.line)
            rendered = _render_type(param.type_name)
            if rendered is not None:
                rendered = self._resolve_type_name_string(rendered, closure_scope, param.line)
            body_scope.define_value(
                param.name,
                self._variable_value(rendered, closure_scope, callable_label=param.name),
            )
        body_deferred: List[Callable[[], None]] = []
        self._check_block(expr.body, body_scope, body_deferred)
        self._run_deferred(body_deferred)

    def _check_member_access(
        self,
        obj: ValueInfo,
        expr: ast.MemberAccess,
        scope: Scope,
        allow_field_write: bool,
    ) -> ValueInfo:
        if obj.kind == "module" and obj.module_info is not None:
            if expr.member in obj.module_info.runtime_exports:
                return obj.module_info.runtime_exports[expr.member]
            if expr.member in obj.module_info.type_exports:
                raise SemanticError(
                    f"Type alias '{expr.member}' is not a runtime member of module '{self._member_base_name(expr.obj)}'; "
                    f"use 'use {expr.member} from {self._member_base_name(expr.obj)}' instead",
                    expr.line,
                )
            raise SemanticError(f"Undefined variable: {expr.member}", expr.line)

        obj_info = None
        obj_type_name = None
        if obj.kind == "object" and obj.object_info is not None:
            obj_info = obj.object_info
            obj_type_name = obj.object_info.name
        elif obj.object_info is not None:
            obj_info = obj.object_info
            obj_type_name = obj.object_info.name
        elif obj.type_name is not None:
            obj_info = self._try_resolve_object_info(obj.type_name, scope)
            obj_type_name = obj_info.name if obj_info is not None else None

        if obj_info is None:
            if obj.type_name is not None:
                raise SemanticError(
                    f"Cannot access member '{expr.member}' on value of type '{obj.type_name}'",
                    expr.line,
                )
            return UNKNOWN_VALUE

        if expr.member == "new" and obj.kind == "object":
            if not obj_info.has_constructor:
                raise SemanticError(f"Object '{obj_info.name}' has no constructor", expr.line)
            return ValueInfo(
                kind="constructor",
                return_type_name=obj_info.name,
                object_info=obj_info,
                callable_info=obj_info.constructor,
            )

        if expr.member in obj_info.methods:
            method_info = obj_info.methods[expr.member]
            return ValueInfo(
                kind="static_method" if obj.kind == "object" else "bound_method",
                return_type_name=method_info.return_type_name,
                object_info=obj_info,
                callable_info=method_info,
            )

        if expr.member in obj_info.fields:
            is_self = (
                isinstance(expr.obj, ast.Identifier)
                and expr.obj.name == "self"
                and scope.method_self_type == obj_type_name
            )
            if allow_field_write and is_self:
                return self._variable_value(obj_info.fields[expr.member], scope)
            if is_self:
                return self._variable_value(obj_info.fields[expr.member], scope)
            if allow_field_write:
                raise SemanticError(
                    f"Cannot assign to field '{expr.member}' from outside; "
                    f"use 'self.{expr.member} := ...' inside a method",
                    expr.line,
                )
            raise SemanticError(
                f"Cannot access private field '{expr.member}' of '{obj_info.name}' from outside; "
                f"use a method instead",
                expr.line,
            )

        raise SemanticError(f"Object '{obj_info.name}' has no member '{expr.member}'", expr.line)

    def _resolve_object_info(self, name: str, scope: Scope, line: int) -> ObjectInfo:
        obj = self._try_resolve_object_info(name, scope)
        if obj is None:
            raise SemanticError(f"Unknown object type: {name}", line)
        return obj

    def _try_resolve_object_info(self, name: str, scope: Scope) -> Optional[ObjectInfo]:
        resolved = self._resolve_type_name_string(name, scope, 0, strict=False)
        if resolved is None:
            return None
        value = scope.resolve_value(resolved)
        if value is None or value.kind != "object" or value.object_info is None:
            return None
        return value.object_info

    def _resolve_type_node(self, type_node: Any, scope: Scope, line: int) -> str:
        if isinstance(type_node, ast.TypeName):
            resolved = self._resolve_type_name_string(type_node.name, scope, line)
            if resolved is None:
                raise SemanticError(f"Unknown type: {type_node.name}", line)
            return resolved
        if isinstance(type_node, ast.GenericType):
            if type_node.base not in GENERIC_BASE_ARITY:
                raise SemanticError(f"Unknown generic type: {type_node.base}", line)
            min_arity, max_arity = GENERIC_BASE_ARITY[type_node.base]
            count = len(type_node.params)
            if count < min_arity or (max_arity is not None and count > max_arity):
                if max_arity is None:
                    raise SemanticError(
                        f"Generic type '{type_node.base}' expects at least {min_arity} parameter(s)",
                        line,
                    )
                raise SemanticError(
                    f"Generic type '{type_node.base}' expects {min_arity} parameter(s)",
                    line,
                )
            if type_node.base == "money":
                return _render_type(type_node) or "money"
            for param in type_node.params:
                self._resolve_type_node(param, scope, line)
            return _render_type(type_node) or type_node.base
        if isinstance(type_node, ast.FuncType):
            for param in type_node.param_types:
                self._resolve_type_node(param, scope, line)
            if type_node.return_type is not None:
                self._resolve_type_node(type_node.return_type, scope, line)
            return _render_type(type_node) or "func"
        raise SemanticError(f"Unknown type node: {type(type_node).__name__}", line)

    def _resolve_alias_name(self, alias_name: str, scope: Scope, line: int) -> str:
        resolved = self._resolve_type_name_string(alias_name, scope, line)
        if resolved is None:
            raise SemanticError(f"Unknown type: {alias_name}", line)
        return resolved

    def _resolve_type_name_string(
        self,
        name: str,
        scope: Scope,
        line: int,
        strict: bool = True,
    ) -> Optional[str]:
        if name in BASE_TYPE_NAMES:
            return name
        if name.startswith("money[") and name.endswith("]"):
            return name
        if "[" in name and name.endswith("]"):
            base = name.split("[", 1)[0]
            if base in GENERIC_BASE_ARITY:
                return name
        if name.startswith("(") and "->" in name:
            return name
        seen = set()
        current = name
        while True:
            if current in BASE_TYPE_NAMES:
                return current
            if current.startswith("money[") and current.endswith("]"):
                return current
            if "[" in current and current.endswith("]"):
                base = current.split("[", 1)[0]
                if base in GENERIC_BASE_ARITY:
                    return current
            if current.startswith("(") and "->" in current:
                return current
            if current in seen:
                if strict:
                    raise SemanticError(f"Cyclic type alias detected at '{current}'", line)
                return None
            seen.add(current)
            value = scope.resolve_value(current)
            if value is not None and value.kind == "object":
                return current
            alias = scope.resolve_alias_raw(current)
            if alias is None:
                if strict:
                    raise SemanticError(f"Unknown type: {current}", line)
                return None
            current = alias

    def _member_base_name(self, expr: Any) -> str:
        if isinstance(expr, ast.Identifier):
            return expr.name
        return "<expr>"
