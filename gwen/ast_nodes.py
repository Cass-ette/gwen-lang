"""Gwen AST node definitions."""

from dataclasses import dataclass, field
from typing import List, Optional, Any, Union


# --- Types ---

TypeNode = Union["TypeName", "GenericType", "FuncType"]

@dataclass
class TypeName:
    name: str
    line: int = 0

@dataclass
class GenericType:
    base: str
    params: List[TypeNode] = field(default_factory=list)
    line: int = 0

@dataclass
class FuncType:
    param_types: List[TypeNode] = field(default_factory=list)
    return_type: Optional[TypeNode] = None
    line: int = 0


# --- Expressions ---

@dataclass
class IntLiteral:
    value: int
    line: int = 0

@dataclass
class FloatLiteral:
    value: float
    line: int = 0

@dataclass
class StringLiteral:
    value: str
    line: int = 0

@dataclass
class BoolLiteral:
    value: bool
    line: int = 0

@dataclass
class Identifier:
    name: str
    line: int = 0

@dataclass
class BinaryOp:
    left: Any
    op: str
    right: Any
    line: int = 0

@dataclass
class UnaryOp:
    op: str
    operand: Any
    line: int = 0

@dataclass
class FuncCall:
    name: Any  # can be Identifier or MemberAccess
    args: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class MemberAccess:
    obj: Any
    member: str
    line: int = 0

@dataclass
class IndexAccess:
    obj: Any
    index: Any
    line: int = 0

@dataclass
class Lambda:
    params: List[Any]  # list of Param
    body: List[Any]
    line: int = 0

@dataclass
class OkExpr:
    value: Any
    line: int = 0

@dataclass
class ErrExpr:
    value: Any
    line: int = 0

@dataclass
class ListLiteral:
    elements: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class DictLiteral:
    key_type: Any
    value_type: Any
    entries: List[Any] = field(default_factory=list)  # List of (key_expr, value_expr)
    line: int = 0

@dataclass
class AsExpr:
    expr: Any
    type_name: str
    line: int = 0


# --- Statements ---

@dataclass
class Param:
    name: str
    type_name: Optional[Any] = None
    default: Any = None
    line: int = 0

@dataclass
class Assignment:
    targets: List[Any]  # str or IndexAccess
    values: List[Any]
    line: int = 0

@dataclass
class VarDecl:
    name: str
    type_name: Optional[Any] = None
    value: Any = None
    is_const: bool = False
    is_uninit: bool = False  # declared without value, reads before assign error
    line: int = 0

@dataclass
class VarBlock:
    """var [default [<expr>]] ... endvar - batch declarations."""
    decls: List[Any] = field(default_factory=list)  # list of VarDecl
    default_mode: str = "none"  # "none" | "zero" | "value"
    default_value: Any = None   # expr when mode == "value"
    line: int = 0

@dataclass
class ReturnStmt:
    value: Any = None
    line: int = 0

@dataclass
class IfStmt:
    condition: Any = None
    body: List[Any] = field(default_factory=list)
    elifs: List[Any] = field(default_factory=list)  # list of (condition, body)
    else_body: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class WhileStmt:
    condition: Any = None
    body: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class ForRangeStmt:
    var: str = ""
    start: Any = None
    end: Any = None
    step: Any = None
    direction: str = "auto"  # "auto", "asc", "desc"
    body: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class ForEachStmt:
    var: str = ""
    iterable: Any = None
    index_var: Optional[str] = None
    body: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class MatchStmt:
    subject: Any = None
    cases: List[Any] = field(default_factory=list)  # list of WhenClause
    else_body: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class WhenClause:
    patterns: List[Any] = field(default_factory=list)
    body: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class FuncDef:
    name: str = ""
    params: List[Param] = field(default_factory=list)
    return_type: Any = None  # Single type or List of types for multiple returns
    body: List[Any] = field(default_factory=list)
    exported: bool = False
    line: int = 0

@dataclass
class ModuleDef:
    name: str = ""
    body: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class UseStmt:
    module: str = ""
    names: List[str] = field(default_factory=list)  # empty = import whole module
    line: int = 0

@dataclass
class ParallelStmt:
    body: List[Any] = field(default_factory=list)
    result_var: Optional[str] = None
    allow_fail: bool = False
    line: int = 0

@dataclass
class GlobalStmt:
    """global x := value - force assignment to module/global scope."""
    name: str = ""
    value: Any = None
    line: int = 0

@dataclass
class ArenaStmt:
    """arena name do ... endarena - explicit memory region."""
    name: str = ""
    body: List[Any] = field(default_factory=list)
    line: int = 0

@dataclass
class TagStmt:
    name: str = ""
    line: int = 0

@dataclass
class TypeAlias:
    name: str = ""
    target: Any = None  # TypeNode
    line: int = 0

@dataclass
class ExprStmt:
    expr: Any = None
    line: int = 0

@dataclass
class Program:
    statements: List[Any] = field(default_factory=list)
