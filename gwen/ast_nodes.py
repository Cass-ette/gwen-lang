"""Gwen AST node definitions."""

from dataclasses import dataclass, field
from typing import List, Optional, Any


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
class AsExpr:
    expr: Any
    type_name: str
    line: int = 0


# --- Statements ---

@dataclass
class Param:
    name: str
    type_name: Optional[str] = None
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
    type_name: Optional[str] = None
    value: Any = None
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
    return_type: Optional[str] = None
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
class TagStmt:
    name: str = ""
    line: int = 0

@dataclass
class ExprStmt:
    expr: Any = None
    line: int = 0

@dataclass
class Program:
    statements: List[Any] = field(default_factory=list)
