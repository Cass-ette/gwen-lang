"""Official stdlib module catalog for Gwen.

These modules are currently backed by interpreter/checker builtins.
They provide the future-facing `use ... from list/string/...` import shape
without breaking existing direct builtin access.
"""

OFFICIAL_STDLIB_MODULES = {
    "list": [
        "append",
        "pop",
        "removeat",
        "insert",
        "concat",
        "sort",
        "asc",
        "desc",
        "reversed",
        "map",
        "filter",
        "range",
        "enumerate",
    ],
    "string": [
        "split",
        "join",
        "substring",
        "contains",
        "trim",
        "replace",
    ],
    "math": [
        "abs",
        "min",
        "max",
        "sqrt",
        "floor",
        "ceil",
    ],
    "dict": [
        "haskey",
        "get",
        "keys",
        "values",
        "items",
    ],
    "io": [
        "readfile",
        "writefile",
        "appendfile",
    ],
}
