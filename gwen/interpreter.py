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

BASE_CHECKED_TYPES = {"int", "float", "string", "bool"}


def needs_type_check(type_name: Optional[str]) -> bool:
    """Whether coerce_to_type should validate values against this type."""
    if type_name is None:
        return False
    if type_name in PRECISION_TYPES or type_name in BASE_CHECKED_TYPES:
        return True
    if is_money_type(type_name):
        return True
    return False

# --- Money type ---
MONEY_SCALE = 10_000  # 4 decimal places
MONEY_MIN, MONEY_MAX = INT_RANGES["int64"]  # stored as int64


def is_money_type(type_name: Optional[str]) -> bool:
    return type_name is not None and type_name.startswith("money[") and type_name.endswith("]")


def money_currency(type_name: str) -> str:
    """Extract currency tag from 'money[USD]' -> 'USD'."""
    return type_name[len("money["):-1]


def zero_value(type_name: Optional[str], line: int = 0) -> Any:
    """Return the zero value for a type. Raises for types without well-defined zeros."""
    if type_name is None:
        raise GwenError("Cannot infer zero value without type annotation", line)
    if type_name in ("int", "int8", "int16", "int32", "int64",
                     "uint8", "uint16", "uint32", "uint64"):
        return 0
    if type_name in ("float", "float32", "float64"):
        return 0.0
    if type_name == "string":
        return ""
    if type_name == "bool":
        return False
    if type_name == "list" or type_name.startswith("list["):
        return []  # fresh list per call
    if is_money_type(type_name):
        return MoneyValue(raw=0, currency=money_currency(type_name))
    raise GwenError(f"No default zero value for type '{type_name}'", line)


def coerce_to_type(value: Any, type_name: str, line: int = 0) -> Any:
    """Coerce a value to the specified explicit precision type."""
    if is_money_type(type_name):
        currency = money_currency(type_name)
        if isinstance(value, MoneyValue):
            if value.currency != currency:
                raise GwenError(
                    f"Currency mismatch: cannot assign money[{value.currency}] to money[{currency}]",
                    line,
                )
            return value
        # float / int literal -> MoneyValue
        try:
            raw = round(float(value) * MONEY_SCALE)
        except (TypeError, ValueError):
            raise GwenError(f"Cannot convert {type(value).__name__} to money[{currency}]", line)
        if raw < MONEY_MIN or raw > MONEY_MAX:
            raise GwenError(
                f"Overflow: {value} out of range for money[{currency}] (int64-backed, scale=4)",
                line,
            )
        return MoneyValue(raw=raw, currency=currency)
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
        if isinstance(value, bool) or not isinstance(value, int):
            raise GwenError(
                f"Type mismatch: expected int, got {type(value).__name__}", line
            )
        return value
    elif type_name == "float":
        if isinstance(value, bool) or not isinstance(value, (int, float)):
            raise GwenError(
                f"Type mismatch: expected float, got {type(value).__name__}", line
            )
        return float(value)
    elif type_name == "string":
        if not isinstance(value, str):
            raise GwenError(
                f"Type mismatch: expected string, got {type(value).__name__}", line
            )
        return value
    elif type_name == "bool":
        if not isinstance(value, bool):
            raise GwenError(
                f"Type mismatch: expected bool, got {type(value).__name__}", line
            )
        return value
    else:
        return value  # unknown type, pass through


def resolve_type_name(type_node: Any) -> Optional[str]:
    """Extract the type name string from a type AST node."""
    if type_node is None:
        return None
    if isinstance(type_node, ast.TypeName):
        return type_node.name
    if isinstance(type_node, ast.GenericType):
        # money[USD] -> "money[USD]"; list[int] -> "list"
        if type_node.base == "money" and len(type_node.params) == 1:
            inner = resolve_type_name(type_node.params[0])
            if inner:
                return f"money[{inner}]"
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


class _UninitType:
    """Sentinel for variables declared without value."""
    _instance = None
    def __new__(cls):
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance
    def __repr__(self):
        return "<uninit>"


UNINIT = _UninitType()


class MoneyValue:
    """Money value with currency tag, stored as int64 scaled by MONEY_SCALE."""
    __slots__ = ("raw", "currency")

    def __init__(self, raw: int, currency: str):
        self.raw = raw
        self.currency = currency

    def as_float(self) -> float:
        return self.raw / MONEY_SCALE

    def __repr__(self) -> str:
        return self.__str__()

    def __str__(self) -> str:
        # "19.99 USD" or "20 USD"
        s = f"{self.as_float():.4f}"
        # strip trailing zeros but keep at least 2 decimals
        if "." in s:
            s = s.rstrip("0")
            if s.endswith("."):
                s += "00"
            elif len(s.split(".")[1]) == 1:
                s += "0"
        return f"{s} {self.currency}"


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


class ObjectType:
    """Registered object type (class-like). Holds field defs, constructor, methods."""
    __slots__ = ("name", "fields", "field_types", "constructor", "methods", "closure")

    def __init__(self, name, fields, field_types, constructor, methods, closure):
        self.name = name
        self.fields = fields  # ordered list of field names
        self.field_types = field_types  # dict: name -> resolved type name
        self.constructor = constructor  # ConstructorDef or None
        self.methods = methods  # dict: name -> MethodDef
        self.closure = closure  # Environment where object was defined

    def __repr__(self):
        return f"<object type {self.name}>"


class ObjectValue:
    """An instance of a Gwen object."""
    __slots__ = ("type_name", "fields", "_object_type")

    def __init__(self, type_name, fields, object_type):
        self.type_name = type_name
        self.fields = fields  # dict: name -> value
        self._object_type = object_type

    def __repr__(self):
        return f"<{self.type_name} {self.fields}>"


class _BoundMethod:
    """Method bound to an instance: acc.deposit -> bound method."""
    __slots__ = ("instance", "obj_type", "method_name")
    def __init__(self, instance, obj_type, method_name):
        self.instance = instance
        self.obj_type = obj_type
        self.method_name = method_name


class _StaticMethodRef:
    """Type-level method reference: Account.deposit (requires explicit self)."""
    __slots__ = ("obj_type", "method_name")
    def __init__(self, obj_type, method_name):
        self.obj_type = obj_type
        self.method_name = method_name


class _ConstructorRef:
    """Type-level constructor reference: Account.new."""
    __slots__ = ("obj_type",)
    def __init__(self, obj_type):
        self.obj_type = obj_type


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
    def __init__(
        self,
        parent: Optional['Environment'] = None,
        is_call_frame: bool = False,
        func_name: Optional[str] = None,
        method_self: Optional[Any] = None,
    ):
        self.vars: Dict[str, Any] = {}
        self.types: Dict[str, str] = {}  # variable name -> type name (e.g. "int8")
        self.consts: set = set()  # set of variable names that are const (immutable)
        self.parent = parent
        self.is_call_frame = is_call_frame  # True for function call environments
        self.func_name = func_name  # Function name for this call frame
        self.method_self = method_self  # Bound receiver when executing an object method

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

    def get_method_self(self) -> Optional[Any]:
        """Look up the active method receiver across nested scopes."""
        if self.method_self is not None:
            return self.method_self
        if self.parent:
            return self.parent.get_method_self()
        return None


class Interpreter:
    def __init__(self):
        self.global_env = Environment()
        self.modules: Dict[str, Environment] = {}
        self.type_aliases: Dict[str, str] = {}  # alias name -> canonical type name
        self.objects: Dict[str, ObjectType] = {}  # object name -> ObjectType
        self._setup_builtins()

    def _setup_builtins(self):
        self.global_env.set("write", self._builtin_write)
        self.global_env.set("read", self._builtin_read)
        self.global_env.set("len", self._builtin_len)
        self.global_env.set("str", self._builtin_str)
        self.global_env.set("int", self._builtin_int)
        self.global_env.set("float", self._builtin_float)
        self.global_env.set("append", self._builtin_append)
        self.global_env.set("typeof", self._builtin_type)
        self.global_env.set("sort", self._builtin_sort)
        self.global_env.set("asc", self._builtin_asc)
        self.global_env.set("desc", self._builtin_desc)
        self.global_env.set("reversed", self._builtin_reverse)
        self.global_env.set("split", self._builtin_split)
        self.global_env.set("join", self._builtin_join)
        self.global_env.set("pop", self._builtin_pop)
        self.global_env.set("insert", self._builtin_insert)
        self.global_env.set("concat", self._builtin_concat)
        self.global_env.set("substring", self._builtin_substring)
        self.global_env.set("contains", self._builtin_contains)
        self.global_env.set("trim", self._builtin_trim)
        self.global_env.set("replace", self._builtin_replace)
        self.global_env.set("abs", self._builtin_abs)
        self.global_env.set("min", self._builtin_min)
        self.global_env.set("max", self._builtin_max)
        self.global_env.set("sqrt", self._builtin_sqrt)
        self.global_env.set("floor", self._builtin_floor)
        self.global_env.set("ceil", self._builtin_ceil)
        self.global_env.set("haskey", self._builtin_haskey)
        self.global_env.set("get", self._builtin_get)
        self.global_env.set("keys", self._builtin_keys)
        self.global_env.set("values", self._builtin_values)
        self.global_env.set("items", self._builtin_items)
        self.global_env.set("readfile", self._builtin_readfile)
        self.global_env.set("writefile", self._builtin_writefile)
        self.global_env.set("appendfile", self._builtin_appendfile)

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
        if isinstance(obj, dict):
            return "dict"
        if isinstance(obj, MoneyValue):
            return f"money[{obj.currency}]"
        if isinstance(obj, OkValue):
            return "ok"
        if isinstance(obj, ErrValue):
            return "err"
        if isinstance(obj, (GwenFunction, GwenLambda)):
            return "func"
        if isinstance(obj, ObjectValue):
            return obj.type_name
        if isinstance(obj, ObjectType):
            return f"object<{obj.name}>"
        return "unknown"

    def _builtin_sort(self, lst, cmp):
        """Stable sort returning new list. cmp is a Gwen function (a, b) -> bool meaning a < b."""
        import functools
        if not isinstance(lst, list):
            raise GwenError(f"sort() requires a list, got {type(lst).__name__}")
        # cmp is a GwenLambda or GwenFunction that returns bool
        def key_func(a, b):
            # Call the Gwen comparison function
            if isinstance(cmp, GwenLambda):
                return self.call_lambda(cmp, [a, b])
            elif isinstance(cmp, GwenFunction):
                return self.call_function(cmp, [a, b])
            else:
                # cmp is a Python callable (for asc/desc builtins)
                return cmp(a, b)
        # Use cmp_to_key for Python sorted with custom comparator
        # Our cmp returns bool (a < b), convert to -1/0/1 for cmp_to_key
        def py_cmp(a, b):
            if key_func(a, b):  # a < b
                return -1
            if key_func(b, a):  # b < a (a > b)
                return 1
            return 0
        return sorted(lst, key=functools.cmp_to_key(py_cmp))

    def _builtin_asc(self, a, b):
        """Ascending comparison: a < b"""
        # Support for primitive types
        return a < b

    def _builtin_desc(self, a, b):
        """Descending comparison: a > b"""
        return a > b

    def _builtin_reverse(self, lst):
        """Return a new list with elements in reverse order."""
        if not isinstance(lst, list):
            raise GwenError(f"reverse() requires a list, got {type(lst).__name__}")
        return lst[::-1]

    def _builtin_split(self, s, sep):
        """Split string by separator."""
        if not isinstance(s, str):
            raise GwenError(f"split() requires a string, got {type(s).__name__}")
        if not isinstance(sep, str):
            raise GwenError(f"split() separator must be a string, got {type(sep).__name__}")
        if sep == "":
            # Split into characters
            return list(s)
        return s.split(sep)

    def _builtin_join(self, parts, sep):
        """Join list of strings with separator."""
        if not isinstance(parts, list):
            raise GwenError(f"join() requires a list, got {type(parts).__name__}")
        if not isinstance(sep, str):
            raise GwenError(f"join() separator must be a string, got {type(sep).__name__}")
        # Convert all parts to strings
        str_parts = [str(p) for p in parts]
        return sep.join(str_parts)

    def _builtin_pop(self, lst):
        """Remove and return the last element of a list. Modifies the list in place."""
        if not isinstance(lst, list):
            raise GwenError(f"pop() requires a list, got {type(lst).__name__}")
        if len(lst) == 0:
            raise GwenError("pop() from empty list")
        return lst.pop()

    def _builtin_insert(self, lst, idx, item):
        """Insert item at index. Modifies the list in place."""
        if not isinstance(lst, list):
            raise GwenError(f"insert() requires a list, got {type(lst).__name__}")
        if not isinstance(idx, int):
            raise GwenError(f"insert() index must be an integer, got {type(idx).__name__}")
        # Allow negative indices and append behavior (idx == len)
        if idx < 0:
            idx = len(lst) + idx + 1  # Convert to positive: -1 becomes len
        if idx < 0 or idx > len(lst):
            raise GwenError(f"insert() index out of range: {idx}")
        lst.insert(idx, item)
        return None

    def _builtin_concat(self, a, b):
        """Return a new list concatenating a and b. Does not modify inputs."""
        if not isinstance(a, list):
            raise GwenError(f"concat() first argument must be a list, got {type(a).__name__}")
        if not isinstance(b, list):
            raise GwenError(f"concat() second argument must be a list, got {type(b).__name__}")
        return a + b

    def _builtin_substring(self, s, start, end):
        """Extract substring from start to end (both inclusive).

        Gwen uses closed-closed intervals [start, end] for substring —
        both endpoints are included. This is more intuitive than [start, end).

        Bounds are strictly checked: out-of-bounds raises GwenError.
        """
        if not isinstance(s, str):
            raise GwenError(f"substring() requires a string, got {type(s).__name__}")
        if not isinstance(start, int):
            raise GwenError(f"substring() start must be an integer, got {type(start).__name__}")
        if not isinstance(end, int):
            raise GwenError(f"substring() end must be an integer, got {type(end).__name__}")
        length = len(s)
        # Strict bounds checking: no silent clamping
        if start < 0:
            raise GwenError(f"substring() start out of bounds: {start} (string length: {length})")
        # end is inclusive, so max valid end is length-1
        if end >= length:
            raise GwenError(f"substring() end out of bounds: {end} (string length: {length}, max valid: {length-1})")
        if start > end:
            raise GwenError(f"substring() start ({start}) > end ({end})")
        # Closed-closed interval: include end, so slice to end+1
        return s[start:end+1]

    def _builtin_contains(self, s, substr):
        """Check if substring exists in string."""
        if not isinstance(s, str):
            raise GwenError(f"contains() requires a string, got {type(s).__name__}")
        if not isinstance(substr, str):
            raise GwenError(f"contains() substr must be a string, got {type(substr).__name__}")
        return substr in s

    def _builtin_trim(self, s):
        """Remove leading and trailing whitespace."""
        if not isinstance(s, str):
            raise GwenError(f"trim() requires a string, got {type(s).__name__}")
        return s.strip()

    def _builtin_replace(self, s, old, new):
        """Replace all occurrences of old with new."""
        if not isinstance(s, str):
            raise GwenError(f"replace() requires a string, got {type(s).__name__}")
        if not isinstance(old, str):
            raise GwenError(f"replace() old must be a string, got {type(old).__name__}")
        if not isinstance(new, str):
            raise GwenError(f"replace() new must be a string, got {type(new).__name__}")
        return s.replace(old, new)

    def _builtin_abs(self, x):
        """Absolute value for int and float."""
        if isinstance(x, bool):
            raise GwenError("abs() does not accept bool")
        if isinstance(x, int):
            return abs(x)
        if isinstance(x, float):
            return abs(x)
        raise GwenError(f"abs() requires int or float, got {type(x).__name__}")

    def _builtin_min(self, a, b):
        """Minimum of two values (supports int, float, string by < comparison)."""
        if type(a) != type(b):
            raise GwenError(f"min() arguments must be same type, got {type(a).__name__} and {type(b).__name__}")
        if a < b:
            return a
        return b

    def _builtin_max(self, a, b):
        """Maximum of two values (supports int, float, string by > comparison)."""
        if type(a) != type(b):
            raise GwenError(f"max() arguments must be same type, got {type(a).__name__} and {type(b).__name__}")
        if a > b:
            return a
        return b

    def _builtin_sqrt(self, x):
        """Square root for float. Requires explicit float input (no implicit int conversion)."""
        import math
        if isinstance(x, bool):
            raise GwenError("sqrt() does not accept bool")
        if isinstance(x, int):
            raise GwenError("sqrt() requires float, got int; use sqrt(float(x)) for explicit conversion")
        if isinstance(x, float):
            return math.sqrt(x)
        raise GwenError(f"sqrt() requires float, got {type(x).__name__}")

    def _builtin_floor(self, x):
        """Floor for float. Returns float (not int, for type consistency)."""
        import math
        if isinstance(x, bool):
            raise GwenError("floor() does not accept bool")
        if isinstance(x, int):
            raise GwenError("floor() requires float, got int; use floor(float(x)) for explicit conversion")
        if isinstance(x, float):
            return float(math.floor(x))
        raise GwenError(f"floor() requires float, got {type(x).__name__}")

    def _builtin_ceil(self, x):
        """Ceiling for float. Returns float (not int, for type consistency)."""
        import math
        if isinstance(x, bool):
            raise GwenError("ceil() does not accept bool")
        if isinstance(x, int):
            raise GwenError("ceil() requires float, got int; use ceil(float(x)) for explicit conversion")
        if isinstance(x, float):
            return float(math.ceil(x))
        raise GwenError(f"ceil() requires float, got {type(x).__name__}")

    # --- Dict built-in functions ---

    def _builtin_haskey(self, d, key):
        """Check if dict contains key."""
        if not isinstance(d, dict):
            raise GwenError(f"haskey() requires a dict, got {type(d).__name__}")
        return key in d

    def _builtin_get(self, d, key, default):
        """Get value from dict with default if key not found."""
        if not isinstance(d, dict):
            raise GwenError(f"get() requires a dict, got {type(d).__name__}")
        return d.get(key, default)

    def _builtin_keys(self, d):
        """Return list of dict keys."""
        if not isinstance(d, dict):
            raise GwenError(f"keys() requires a dict, got {type(d).__name__}")
        return list(d.keys())

    def _builtin_values(self, d):
        """Return list of dict values."""
        if not isinstance(d, dict):
            raise GwenError(f"values() requires a dict, got {type(d).__name__}")
        return list(d.values())

    def _builtin_items(self, d):
        """Return list of [key, value] pairs from a dict.

        Each pair is a two-element list: ["key", value].
        Use `for pair in items(d) do ... endfor` to iterate dict entries.
        """
        if not isinstance(d, dict):
            raise GwenError(f"items() requires a dict, got {type(d).__name__}")
        return [[k, v] for k, v in d.items()]

    # --- File I/O built-in functions ---
    # 全部返回 result[T]：错误不静默，调用方必须 match 处理。

    def _builtin_readfile(self, path):
        """Read entire file as string. Returns result[string]."""
        if not isinstance(path, str):
            raise GwenError(f"readfile() requires a string path, got {type(path).__name__}")
        try:
            with open(path, "r", encoding="utf-8") as f:
                return OkValue(f.read())
        except OSError as e:
            return ErrValue(str(e))
        except UnicodeDecodeError as e:
            return ErrValue(f"decode error: {e}")

    def _builtin_writefile(self, path, content):
        """Overwrite file with content. Returns result[int] (bytes written)."""
        if not isinstance(path, str):
            raise GwenError(f"writefile() requires a string path, got {type(path).__name__}")
        if not isinstance(content, str):
            raise GwenError(f"writefile() requires string content, got {type(content).__name__}")
        try:
            data = content.encode("utf-8")
            with open(path, "wb") as f:
                f.write(data)
            return OkValue(len(data))
        except OSError as e:
            return ErrValue(str(e))

    def _builtin_appendfile(self, path, content):
        """Append content to file. Returns result[int] (bytes written)."""
        if not isinstance(path, str):
            raise GwenError(f"appendfile() requires a string path, got {type(path).__name__}")
        if not isinstance(content, str):
            raise GwenError(f"appendfile() requires string content, got {type(content).__name__}")
        try:
            data = content.encode("utf-8")
            with open(path, "ab") as f:
                f.write(data)
            return OkValue(len(data))
        except OSError as e:
            return ErrValue(str(e))

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
                    elif isinstance(target, ast.MemberAccess):
                        self._assign_member(target, val, env, stmt.line)
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
                elif isinstance(target, ast.MemberAccess):
                    self._assign_member(target, values[0], env, stmt.line)
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
                    elif isinstance(target, ast.MemberAccess):
                        self._assign_member(target, val, env, stmt.line)
                    else:
                        raise GwenError("Invalid assignment target", stmt.line)
            else:
                raise GwenError(f"Assignment count mismatch: {len(stmt.targets)} targets, {len(values)} values", stmt.line)

        elif isinstance(stmt, ast.VarDecl):
            if env.is_const(stmt.name):
                raise GwenError(f"Cannot redeclare const variable: {stmt.name}", stmt.line)
            type_name = self._resolve_alias(resolve_type_name(stmt.type_name))
            if stmt.is_uninit:
                # Declared without value -> mark uninit; reads will error
                env.set(stmt.name, UNINIT)
                env.set_type(stmt.name, type_name)
                if stmt.is_const:
                    raise GwenError(f"Const variable '{stmt.name}' must be initialized", stmt.line)
            else:
                value = self.eval_expr(stmt.value, env) if stmt.value is not None else None
                if value is not None and needs_type_check(type_name):
                    value = coerce_to_type(value, type_name, stmt.line)
                env.set(stmt.name, value)
                env.set_type(stmt.name, type_name)
                if stmt.is_const:
                    env.mark_const(stmt.name)

        elif isinstance(stmt, ast.VarBlock):
            # Evaluate default_value once (for "value" mode); fresh for each var would
            # surprise users since expression may have side effects.
            shared_value = None
            if stmt.default_mode == "value":
                shared_value = self.eval_expr(stmt.default_value, env)
            for d in stmt.decls:
                if env.is_const(d.name):
                    raise GwenError(f"Cannot redeclare const variable: {d.name}", d.line)
                type_name = self._resolve_alias(resolve_type_name(d.type_name))
                # Per-decl resolution: explicit := wins over block default
                if d.value is not None:
                    v = self.eval_expr(d.value, env)
                    if needs_type_check(type_name):
                        v = coerce_to_type(v, type_name, d.line)
                elif stmt.default_mode == "zero":
                    v = zero_value(type_name, d.line)
                elif stmt.default_mode == "value":
                    v = shared_value
                    if needs_type_check(type_name):
                        try:
                            v = coerce_to_type(v, type_name, d.line)
                        except GwenError:
                            raise GwenError(
                                f"'{d.name}: {type_name}' cannot accept `default` value "
                                f"of type {type(shared_value).__name__}; "
                                f"use `{d.name}: {type_name} := ...` or switch to `var default` (zero values)",
                                d.line,
                            )
                else:
                    v = UNINIT
                env.set(d.name, v)
                env.set_type(d.name, type_name)

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
            cond_val = self.eval_expr(stmt.condition, env)
            self.require_bool(cond_val, "'if' condition", stmt.line)
            if cond_val:
                self.exec_block(stmt.body, env)
            else:
                matched = False
                for cond, body in stmt.elifs:
                    elif_val = self.eval_expr(cond, env)
                    self.require_bool(elif_val, "'elif' condition", stmt.line)
                    if elif_val:
                        self.exec_block(body, env)
                        matched = True
                        break
                if not matched and stmt.else_body:
                    self.exec_block(stmt.else_body, env)

        elif isinstance(stmt, ast.WhileStmt):
            while True:
                cond_val = self.eval_expr(stmt.condition, env)
                self.require_bool(cond_val, "'while' condition", stmt.line)
                if not cond_val:
                    break
                self.exec_block(stmt.body, env)

        elif isinstance(stmt, ast.ForRangeStmt):
            start = self.eval_expr(stmt.start, env)
            end = self.eval_expr(stmt.end, env)
            step = self.eval_expr(stmt.step, env) if stmt.step else None

            # Detect char range mode: both start and end are single-char strings
            is_char_range = (
                isinstance(start, str) and len(start) == 1
                and isinstance(end, str) and len(end) == 1
            )
            if is_char_range:
                # Convert to ord for numeric iteration
                start_ord = ord(start)
                end_ord = ord(end)
                # ASCII safety: both should be in ASCII range for clarity
                if not (0 <= start_ord <= 127 and 0 <= end_ord <= 127):
                    raise GwenError(
                        f"Char range only supports ASCII characters (0-127), "
                        f"got ord(start)={start_ord}, ord(end)={end_ord}",
                        stmt.line
                    )

            # Determine direction based on direction field and auto-detection
            if stmt.direction == "asc":
                # Force ascending: always iterate small -> large
                if start > end:
                    start, end = end, start
                    if is_char_range:
                        start_ord, end_ord = end_ord, start_ord
                step = 1 if step is None else abs(step)
                compare = lambda i, end_val: i <= end_val
            elif stmt.direction == "desc":
                # Force descending: always iterate large -> small
                if start < end:
                    start, end = end, start
                    if is_char_range:
                        start_ord, end_ord = end_ord, start_ord
                step = -1 if step is None else -abs(step)
                compare = lambda i, end_val: i >= end_val
            else:
                # Auto mode: infer from start/end
                if step is None:
                    step = 1 if start <= end else -1
                if step > 0:
                    compare = lambda i, end_val: i <= end_val
                else:
                    compare = lambda i, end_val: i >= end_val

            if is_char_range:
                # Iterate using ord, yield char
                i_ord = start_ord
                while compare(i_ord, end_ord):
                    env.update_local(stmt.var, chr(i_ord))
                    self.exec_block(stmt.body, env)
                    i_ord += step
            else:
                # Integer range (original behavior)
                i = start
                while compare(i, end):
                    env.update_local(stmt.var, i)
                    self.exec_block(stmt.body, env)
                    i += step

        elif isinstance(stmt, ast.ForEachStmt):
            iterable = self.eval_expr(stmt.iterable, env)
            if isinstance(iterable, dict):
                raise GwenError(
                    "Cannot iterate directly over a dict. "
                    "Use 'for k in keys(d) do ... endfor', "
                    "'for v in values(d) do ... endfor', "
                    "or 'for pair in items(d) do ... endfor' instead.",
                    stmt.line
                )
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
                    if needs_type_check(type_name):
                        value = coerce_to_type(value, type_name, stmt.line)
                    search_env.vars[stmt.name] = value
                    found = True
                    break
                if search_env.is_call_frame:
                    # Found a call frame - check if it has the variable
                    if stmt.name in search_env.vars:
                        type_name = search_env.get_type(stmt.name)
                        if needs_type_check(type_name):
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

        elif isinstance(stmt, ast.ObjectDef):
            if stmt.name in self.objects:
                raise GwenError(f"Object '{stmt.name}' already defined", stmt.line)
            field_names = []
            field_types = {}
            for f in stmt.fields:
                if f.name in field_types:
                    raise GwenError(f"Duplicate field '{f.name}' in object '{stmt.name}'", f.line)
                field_names.append(f.name)
                field_types[f.name] = self._resolve_alias(resolve_type_name(f.type_annotation))
            methods_dict = {}
            for m in stmt.methods:
                if m.name in methods_dict:
                    raise GwenError(f"Duplicate method '{m.name}' in object '{stmt.name}'", m.line)
                methods_dict[m.name] = m
            obj_type = ObjectType(stmt.name, field_names, field_types,
                                  stmt.constructor, methods_dict, env)
            self.objects[stmt.name] = obj_type
            env.set(stmt.name, obj_type)

        else:
            raise GwenError(f"Unknown statement type: {type(stmt).__name__}")

    def _coerce_if_typed(self, name: str, value: Any, line: int, env: Environment) -> Any:
        """If variable has a precision type annotation, coerce value to it."""
        type_name = env.get_local_type(name)
        if needs_type_check(type_name):
            return coerce_to_type(value, type_name, line)
        return value

    def _assign_member(self, target: ast.MemberAccess, value: Any, env: Environment, line: int):
        """Assign to a MemberAccess target. Only allowed via 'self.field := ...' inside methods."""
        if not (isinstance(target.obj, ast.Identifier) and target.obj.name == "self"):
            raise GwenError(
                f"Cannot assign to field '{target.member}' from outside; "
                f"use 'self.{target.member} := ...' inside a method",
                line,
            )
        obj = self.eval_expr(target.obj, env)
        method_self = env.get_method_self()
        if method_self is None or obj is not method_self:
            raise GwenError(
                f"Cannot assign to field '{target.member}' from outside; "
                f"use 'self.{target.member} := ...' inside a method",
                line,
            )
        if not isinstance(obj, ObjectValue):
            raise GwenError(f"'self' is not an object instance", line)
        if target.member not in obj.fields:
            raise GwenError(f"Object '{obj.type_name}' has no field '{target.member}'", line)
        ftype = obj._object_type.field_types.get(target.member)
        if needs_type_check(ftype):
            value = coerce_to_type(value, ftype, line)
        obj.fields[target.member] = value

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
            val = env.get(expr.name)
            if val is UNINIT:
                raise GwenError(f"'{expr.name}' read before assignment", expr.line)
            return val
        if isinstance(expr, ast.ListLiteral):
            return [self.eval_expr(e, env) for e in expr.elements]

        if isinstance(expr, ast.BinaryOp):
            # Short-circuit for and/or: only evaluate right side if needed
            if expr.op in ("and", "or"):
                left = self.eval_expr(expr.left, env)
                self.require_bool(left, f"left side of '{expr.op}'", expr.line)
                if expr.op == "and":
                    if not left:
                        return False
                    right = self.eval_expr(expr.right, env)
                    self.require_bool(right, f"right side of 'and'", expr.line)
                    return right
                else:  # or
                    if left:
                        return True
                    right = self.eval_expr(expr.right, env)
                    self.require_bool(right, f"right side of 'or'", expr.line)
                    return right
            left = self.eval_expr(expr.left, env)
            right = self.eval_expr(expr.right, env)
            return self.eval_binary(expr.op, left, right, expr.line)

        if isinstance(expr, ast.UnaryOp):
            operand = self.eval_expr(expr.operand, env)
            if expr.op == "-":
                return -operand
            if expr.op == "not":
                self.require_bool(operand, "'not' operand", expr.line)
                return not operand

        if isinstance(expr, ast.FuncCall):
            return self.eval_call(expr, env)

        if isinstance(expr, ast.MemberAccess):
            # Type-side access: Account.new / Account.method
            if isinstance(expr.obj, ast.Identifier) and expr.obj.name in self.objects:
                obj_type = self.objects[expr.obj.name]
                if expr.member == "new":
                    if obj_type.constructor is None:
                        raise GwenError(f"Object '{obj_type.name}' has no constructor", expr.line)
                    return _ConstructorRef(obj_type)
                if expr.member in obj_type.methods:
                    return _StaticMethodRef(obj_type, expr.member)
                raise GwenError(f"Object '{obj_type.name}' has no member '{expr.member}'", expr.line)

            obj = self.eval_expr(expr.obj, env)
            if isinstance(obj, ObjectValue):
                # Method access: bound method
                if expr.member in obj._object_type.methods:
                    return _BoundMethod(obj, obj._object_type, expr.member)
                # Field access: only allowed via 'self' identifier
                if isinstance(expr.obj, ast.Identifier) and expr.obj.name == "self":
                    method_self = env.get_method_self()
                    if method_self is None or obj is not method_self:
                        raise GwenError(
                            f"Cannot access private field '{expr.member}' of '{obj.type_name}' "
                            f"from outside; use a method instead",
                            expr.line,
                        )
                    if expr.member in obj.fields:
                        return obj.fields[expr.member]
                    raise GwenError(f"Object '{obj.type_name}' has no field '{expr.member}'", expr.line)
                if expr.member in obj.fields:
                    raise GwenError(
                        f"Cannot access private field '{expr.member}' of '{obj.type_name}' "
                        f"from outside; use a method instead",
                        expr.line,
                    )
                raise GwenError(f"Object '{obj.type_name}' has no member '{expr.member}'", expr.line)
            if isinstance(obj, ObjectType):
                if expr.member == "new":
                    if obj.constructor is None:
                        raise GwenError(f"Object '{obj.name}' has no constructor", expr.line)
                    return _ConstructorRef(obj)
                if expr.member in obj.methods:
                    return _StaticMethodRef(obj, expr.member)
                raise GwenError(f"Object '{obj.name}' has no member '{expr.member}'", expr.line)
            if isinstance(obj, Environment):
                return obj.get(expr.member)
            raise GwenError(f"Cannot access member '{expr.member}' on {type(obj)}", expr.line)

        if isinstance(expr, ast.IndexAccess):
            obj = self.eval_expr(expr.obj, env)
            index = self.eval_expr(expr.index, env)
            # Handle dict with proper error on missing key
            if isinstance(obj, dict):
                if index not in obj:
                    raise GwenError(f"Key not found: {index!r}", expr.line)
                return obj[index]
            # Handle list with bounds check
            if isinstance(obj, list):
                if not isinstance(index, int):
                    raise GwenError(f"List index must be an integer, got {type(index).__name__}", expr.line)
                if index < 0 or index >= len(obj):
                    raise GwenError(f"Index out of range: {index} (list length: {len(obj)})", expr.line)
                return obj[index]
            # Handle string with bounds check
            if isinstance(obj, str):
                if not isinstance(index, int):
                    raise GwenError(f"String index must be an integer, got {type(index).__name__}", expr.line)
                if index < 0 or index >= len(obj):
                    raise GwenError(f"Index out of range: {index} (string length: {len(obj)})", expr.line)
                return obj[index]
            raise GwenError(f"Cannot index type {type(obj).__name__}", expr.line)

        if isinstance(expr, ast.Lambda):
            return GwenLambda(expr, env)

        if isinstance(expr, ast.OkExpr):
            return OkValue(self.eval_expr(expr.value, env))

        if isinstance(expr, ast.ErrExpr):
            return ErrValue(self.eval_expr(expr.value, env))

        if isinstance(expr, ast.AsExpr):
            return self.eval_as(expr, env)

        if isinstance(expr, ast.DictLiteral):
            result = {}
            for key_expr, val_expr in expr.entries:
                key = self.eval_expr(key_expr, env)
                val = self.eval_expr(val_expr, env)
                # Validate key type (must be str or int for now)
                if not isinstance(key, (str, int)):
                    raise GwenError(f"Dict keys must be string or int, got {type(key).__name__}", expr.line)
                result[key] = val
            return result

        if isinstance(expr, ast.ObjectLiteral):
            if expr.name not in self.objects:
                raise GwenError(f"Unknown object type: {expr.name}", expr.line)
            obj_type = self.objects[expr.name]
            fields = {}
            provided = set()
            for fname, fexpr in expr.fields:
                if fname in provided:
                    raise GwenError(f"Duplicate field '{fname}' in '{expr.name}' literal", expr.line)
                if fname not in obj_type.field_types:
                    raise GwenError(f"Object '{expr.name}' has no field '{fname}'", expr.line)
                provided.add(fname)
                val = self.eval_expr(fexpr, env)
                ftype = obj_type.field_types[fname]
                if needs_type_check(ftype):
                    val = coerce_to_type(val, ftype, expr.line)
                fields[fname] = val
            missing = [f for f in obj_type.fields if f not in provided]
            if missing:
                raise GwenError(
                    f"Object '{expr.name}' literal missing fields: {', '.join(missing)}",
                    expr.line,
                )
            return ObjectValue(expr.name, fields, obj_type)

        raise GwenError(f"Unknown expression type: {type(expr).__name__}")

    def _eval_money_binary(self, op: str, left: Any, right: Any, line: int) -> Any:
        lm = isinstance(left, MoneyValue)
        rm = isinstance(right, MoneyValue)

        # Comparisons: require same currency
        if op in ("=", "!=", "<", ">", "<=", ">="):
            if not (lm and rm):
                raise GwenError(
                    f"Cannot compare money with non-money value ({op})", line,
                )
            if left.currency != right.currency:
                raise GwenError(
                    f"Currency mismatch: money[{left.currency}] {op} money[{right.currency}]",
                    line,
                )
            a, b = left.raw, right.raw
            return {
                "=": a == b, "!=": a != b,
                "<": a < b, ">": a > b,
                "<=": a <= b, ">=": a >= b,
            }[op]

        if op in ("+", "-"):
            if not (lm and rm):
                raise GwenError(
                    f"Cannot {op} money with non-money value", line,
                )
            if left.currency != right.currency:
                raise GwenError(
                    f"Currency mismatch: money[{left.currency}] {op} money[{right.currency}]",
                    line,
                )
            raw = left.raw + right.raw if op == "+" else left.raw - right.raw
            if raw < MONEY_MIN or raw > MONEY_MAX:
                raise GwenError(f"Money overflow in {op}", line)
            return MoneyValue(raw=raw, currency=left.currency)

        if op == "*":
            if lm and rm:
                raise GwenError("Cannot multiply money by money", line)
            money = left if lm else right
            scalar = right if lm else left
            # Only int allowed — float multiplication on fixed-point loses precision silently
            if not isinstance(scalar, int) or isinstance(scalar, bool):
                raise GwenError(
                    "Cannot multiply money by float. "
                    "Use explicit conversion: 'money as float' for float arithmetic, "
                    "or divide into int steps instead.",
                    line,
                )
            raw = money.raw * scalar
            if raw < MONEY_MIN or raw > MONEY_MAX:
                raise GwenError("Money overflow in *", line)
            return MoneyValue(raw=raw, currency=money.currency)

        if op == "/":
            if lm and rm:
                # money / money -> float ratio (same currency required)
                if left.currency != right.currency:
                    raise GwenError(
                        f"Currency mismatch in division: money[{left.currency}] / money[{right.currency}]",
                        line,
                    )
                if right.raw == 0:
                    raise GwenError("Division by zero", line)
                return left.raw / right.raw
            if rm:
                raise GwenError("Cannot divide non-money by money", line)
            # money / scalar
            if not isinstance(right, (int, float)) or isinstance(right, bool):
                raise GwenError("Money can only be divided by int or float", line)
            if right == 0:
                raise GwenError("Division by zero", line)
            raw = round(left.raw / right)
            if raw < MONEY_MIN or raw > MONEY_MAX:
                raise GwenError("Money overflow in /", line)
            return MoneyValue(raw=raw, currency=left.currency)

        raise GwenError(f"Operator {op} not supported on money values", line)

    def eval_binary(self, op: str, left: Any, right: Any, line: int) -> Any:
        # Money arithmetic dispatch (before numeric promotion)
        if isinstance(left, MoneyValue) or isinstance(right, MoneyValue):
            return self._eval_money_binary(op, left, right, line)

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
        if op in ("<", ">", "<=", ">="):
            # Gwen does not define ordering for composite types
            if isinstance(left, (list, dict)) or isinstance(right, (list, dict)):
                raise GwenError(
                    f"Comparison '{op}' is not defined for {type(left).__name__} or {type(right).__name__}. "
                    f"Use explicit element-wise comparison instead.",
                    line
                )
            return left < right if op == "<" else \
                   left > right if op == ">" else \
                   left <= right if op == "<=" else \
                   left >= right
        # Note: 'and' / 'or' are handled in eval_expr (short-circuit + strict bool)
        raise GwenError(f"Unknown operator: {op}", line)

    def eval_as(self, expr: ast.AsExpr, env: Environment) -> Any:
        value = self.eval_expr(expr.expr, env)
        target = expr.type_name
        # Forbid money -> money[other_currency]
        if isinstance(value, MoneyValue) and is_money_type(target):
            target_currency = money_currency(target)
            if value.currency != target_currency:
                return ErrValue(
                    f"Cannot convert money[{value.currency}] to money[{target_currency}] "
                    f"(explicit exchange rate required)"
                )
            return OkValue(value)
        # Money -> numeric: strip currency, give raw float/int
        if isinstance(value, MoneyValue):
            if target == "float" or target == "float64":
                return OkValue(value.as_float())
            if target == "float32":
                return OkValue(coerce_to_type(value.as_float(), "float32", expr.line))
            if target in INT_RANGES:
                return OkValue(coerce_to_type(round(value.as_float()), target, expr.line))
            if target == "int":
                return OkValue(round(value.as_float()))
        try:
            if target == "int":
                return OkValue(int(value))
            if target == "float":
                return OkValue(float(value))
            if target == "string":
                return OkValue(str(value))
            if target == "bool":
                # Strict: only bool -> bool. No truthiness conversion.
                # Use explicit comparison instead (e.g., 'x != 0', 's != ""').
                if not isinstance(value, bool):
                    raise GwenError(
                        f"Cannot convert {type(value).__name__} to bool. "
                        f"Use explicit comparison instead (e.g., 'x != 0', 's != \"\"', 'len(lst) > 0')",
                        expr.line
                    )
                return OkValue(value)
            if target in PRECISION_TYPES:
                return OkValue(coerce_to_type(value, target, expr.line))
            # Numeric -> money[X]
            if is_money_type(target):
                return OkValue(coerce_to_type(value, target, expr.line))
            return ErrValue(f"Unknown type: {target}")
        except GwenError:
            raise
        except (ValueError, TypeError, OverflowError):
            return ErrValue(f"Cannot convert {type(value).__name__} to {target}")

    def eval_call(self, call: ast.FuncCall, env: Environment) -> Any:
        callee = self.eval_expr(call.name, env)
        args = [self.eval_expr(a, env) for a in call.args]

        if isinstance(callee, _BoundMethod):
            method_def = callee.obj_type.methods[callee.method_name]
            return self._call_method(method_def, callee.instance, args, callee.obj_type, call.line)

        if isinstance(callee, _StaticMethodRef):
            method_def = callee.obj_type.methods[callee.method_name]
            if not args:
                raise GwenError(
                    f"Method '{callee.obj_type.name}.{callee.method_name}' "
                    f"requires explicit self as first argument",
                    call.line,
                )
            instance = args[0]
            if not (isinstance(instance, ObjectValue) and instance.type_name == callee.obj_type.name):
                raise GwenError(
                    f"First argument to '{callee.obj_type.name}.{callee.method_name}' "
                    f"must be a '{callee.obj_type.name}' instance",
                    call.line,
                )
            return self._call_method(method_def, instance, args[1:], callee.obj_type, call.line)

        if isinstance(callee, _ConstructorRef):
            return self._call_constructor(callee.obj_type, args, call.line)

        if callable(callee):
            return callee(*args)

        if isinstance(callee, GwenFunction):
            return self.call_function(callee, args)

        if isinstance(callee, GwenLambda):
            return self.call_lambda(callee, args)

        raise GwenError(f"'{callee}' is not callable", call.line)

    def _call_method(self, method_def, instance, args, obj_type, line):
        call_env = Environment(
            parent=obj_type.closure,
            is_call_frame=True,
            func_name=method_def.name,
            method_self=instance,
        )
        params = method_def.params
        if not params:
            raise GwenError(
                f"Method '{obj_type.name}.{method_def.name}' must declare 'self' as first parameter",
                line,
            )
        # Bind self
        call_env.set(params[0].name, instance)
        # Bind remaining params
        rest = params[1:]
        for i, param in enumerate(rest):
            if i < len(args):
                val = args[i]
                ptype = self._resolve_alias(resolve_type_name(param.type_name))
                if ptype and ptype in PRECISION_TYPES:
                    val = coerce_to_type(val, ptype, param.line)
                call_env.set(param.name, val)
            elif param.default is not None:
                call_env.set(param.name, self.eval_expr(param.default, obj_type.closure))
            else:
                raise GwenError(f"Missing argument: {param.name}", line)
        try:
            self.exec_block(method_def.body, call_env)
        except ReturnSignal as r:
            return r.value
        return None

    def _call_constructor(self, obj_type, args, line):
        ctor = obj_type.constructor
        if ctor is None:
            raise GwenError(f"Object '{obj_type.name}' has no constructor", line)
        call_env = Environment(parent=obj_type.closure, is_call_frame=True, func_name=f"{obj_type.name}.new")
        for i, param in enumerate(ctor.params):
            if i < len(args):
                val = args[i]
                ptype = self._resolve_alias(resolve_type_name(param.type_name))
                if ptype and ptype in PRECISION_TYPES:
                    val = coerce_to_type(val, ptype, param.line)
                call_env.set(param.name, val)
            elif param.default is not None:
                call_env.set(param.name, self.eval_expr(param.default, obj_type.closure))
            else:
                raise GwenError(f"Missing argument: {param.name}", line)
        try:
            self.exec_block(ctor.body, call_env)
        except ReturnSignal as r:
            if not isinstance(r.value, ObjectValue) or r.value.type_name != obj_type.name:
                raise GwenError(
                    f"Constructor '{obj_type.name}.new' must return a '{obj_type.name}' instance",
                    line,
                )
            return r.value
        raise GwenError(f"Constructor '{obj_type.name}.new' did not return a value", line)

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

    def require_bool(self, value: Any, context: str, line: int) -> None:
        """Strict bool check: raise if value is not exactly a bool.

        Gwen follows Go-style strict typing for conditions. No truthiness.
        Use explicit comparisons (x != 0, s != "", len(lst) > 0) instead.

        TODO: When Gwen transitions from interpreted to compiled, this check
        should move to compile-time type checking instead of runtime.
        """
        if not isinstance(value, bool):
            type_name = type(value).__name__
            raise GwenError(
                f"{context} must be bool, got {type_name}. "
                f"Use explicit comparison (e.g., 'x != 0', 's != \"\"', 'len(lst) > 0')",
                line
            )
