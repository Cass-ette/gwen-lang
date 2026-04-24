package mir

import (
	"strings"

	"github.com/Cass-ette/gwen-lang/internal/hir"
)

type objectTypeInfo struct {
	Fields      map[string]hir.Type
	Methods     map[string]*hir.FuncType
	Constructor *hir.FuncType
}

func namedType(name string) hir.Type {
	return &hir.NamedType{Name: name}
}

func genericType(base string, args ...hir.Type) hir.Type {
	return &hir.GenericType{
		Base: base,
		Args: append([]hir.Type(nil), args...),
	}
}

func funcType(params []hir.Type, returns []hir.Type) *hir.FuncType {
	return &hir.FuncType{
		Params:  append([]hir.Type(nil), params...),
		Returns: append([]hir.Type(nil), returns...),
	}
}

func signatureType(params []*hir.Param, returns []hir.Type) *hir.FuncType {
	paramTypes := make([]hir.Type, 0, len(params))
	for _, param := range params {
		paramTypes = append(paramTypes, param.Type)
	}
	return funcType(paramTypes, returns)
}

func dropFirstParamType(fn *hir.FuncType) *hir.FuncType {
	if fn == nil {
		return nil
	}
	params := append([]hir.Type(nil), fn.Params...)
	if len(params) > 0 {
		params = params[1:]
	}
	return funcType(params, fn.Returns)
}

func typeEqual(left, right hir.Type) bool {
	switch l := left.(type) {
	case nil:
		return right == nil
	case *hir.NamedType:
		r, ok := right.(*hir.NamedType)
		return ok && l.Name == r.Name
	case *hir.GenericType:
		r, ok := right.(*hir.GenericType)
		if !ok || l.Base != r.Base || len(l.Args) != len(r.Args) {
			return false
		}
		for idx := range l.Args {
			if !typeEqual(l.Args[idx], r.Args[idx]) {
				return false
			}
		}
		return true
	case *hir.FuncType:
		r, ok := right.(*hir.FuncType)
		if !ok || len(l.Params) != len(r.Params) || len(l.Returns) != len(r.Returns) {
			return false
		}
		for idx := range l.Params {
			if !typeEqual(l.Params[idx], r.Params[idx]) {
				return false
			}
		}
		for idx := range l.Returns {
			if !typeEqual(l.Returns[idx], r.Returns[idx]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func namedTypeName(typ hir.Type) string {
	named, ok := typ.(*hir.NamedType)
	if !ok {
		return ""
	}
	return named.Name
}

func isNamedType(typ hir.Type, name string) bool {
	return namedTypeName(typ) == name
}

func isDynamicValueType(typ hir.Type) bool {
	switch namedTypeName(typ) {
	case "dynamic", "list", "dict", "JsonNull":
		return true
	default:
		return false
	}
}

func isIntType(typ hir.Type) bool {
	switch namedTypeName(typ) {
	case "int", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}

func isFloatType(typ hir.Type) bool {
	switch namedTypeName(typ) {
	case "float", "float32", "float64":
		return true
	default:
		return false
	}
}

func isNumericType(typ hir.Type) bool {
	return isIntType(typ) || isFloatType(typ)
}

func listItemType(typ hir.Type) hir.Type {
	generic, ok := typ.(*hir.GenericType)
	if !ok || generic.Base != "list" || len(generic.Args) != 1 {
		return nil
	}
	return generic.Args[0]
}

func dictKeyType(typ hir.Type) hir.Type {
	generic, ok := typ.(*hir.GenericType)
	if !ok || generic.Base != "dict" || len(generic.Args) != 2 {
		return nil
	}
	return generic.Args[0]
}

func dictValueType(typ hir.Type) hir.Type {
	generic, ok := typ.(*hir.GenericType)
	if !ok || generic.Base != "dict" || len(generic.Args) != 2 {
		return nil
	}
	return generic.Args[1]
}

func cellItemType(typ hir.Type) hir.Type {
	generic, ok := typ.(*hir.GenericType)
	if !ok || generic.Base != "cell" || len(generic.Args) != 1 {
		return nil
	}
	return generic.Args[0]
}

func moneyType(typ hir.Type) *hir.GenericType {
	switch node := typ.(type) {
	case *hir.GenericType:
		if node.Base != "money" || len(node.Args) != 1 {
			return nil
		}
		return node
	case *hir.NamedType:
		if !strings.HasPrefix(node.Name, "money[") || !strings.HasSuffix(node.Name, "]") {
			return nil
		}
		currency := strings.TrimSuffix(strings.TrimPrefix(node.Name, "money["), "]")
		return &hir.GenericType{
			Base: "money",
			Args: []hir.Type{&hir.NamedType{Name: currency}},
		}
	default:
		return nil
	}
}

func moduleValueKey(moduleName, valueName string) string {
	return moduleName + "." + valueName
}

var stdlibCallableTypes = map[string]map[string]*hir.FuncType{
	"list": {
		"append":    funcType([]hir.Type{namedType("list"), namedType("dynamic")}, nil),
		"pop":       funcType([]hir.Type{namedType("list")}, []hir.Type{namedType("dynamic")}),
		"concat":    funcType([]hir.Type{namedType("list"), namedType("list")}, []hir.Type{namedType("list")}),
		"removeat":  funcType([]hir.Type{namedType("list"), namedType("int")}, []hir.Type{namedType("dynamic")}),
		"insert":    funcType([]hir.Type{namedType("list"), namedType("int"), namedType("dynamic")}, nil),
		"sort":      funcType([]hir.Type{namedType("list"), namedType("dynamic")}, []hir.Type{namedType("list")}),
		"asc":       funcType([]hir.Type{namedType("dynamic"), namedType("dynamic")}, []hir.Type{namedType("bool")}),
		"desc":      funcType([]hir.Type{namedType("dynamic"), namedType("dynamic")}, []hir.Type{namedType("bool")}),
		"reversed":  funcType([]hir.Type{namedType("list")}, []hir.Type{namedType("list")}),
		"map":       funcType([]hir.Type{namedType("list"), namedType("dynamic")}, []hir.Type{namedType("list")}),
		"filter":    funcType([]hir.Type{namedType("list"), namedType("dynamic")}, []hir.Type{namedType("list")}),
		"range":     funcType([]hir.Type{namedType("int"), namedType("int"), namedType("int")}, []hir.Type{genericType("list", namedType("int"))}),
		"enumerate": funcType([]hir.Type{namedType("list")}, []hir.Type{genericType("list", namedType("list"))}),
	},
	"string": {
		"split":      funcType([]hir.Type{namedType("string"), namedType("string")}, []hir.Type{genericType("list", namedType("string"))}),
		"join":       funcType([]hir.Type{namedType("list"), namedType("string")}, []hir.Type{namedType("string")}),
		"substring":  funcType([]hir.Type{namedType("string"), namedType("int"), namedType("int")}, []hir.Type{namedType("string")}),
		"startswith": funcType([]hir.Type{namedType("string"), namedType("string")}, []hir.Type{namedType("bool")}),
		"endswith":   funcType([]hir.Type{namedType("string"), namedType("string")}, []hir.Type{namedType("bool")}),
		"contains":   funcType([]hir.Type{namedType("string"), namedType("string")}, []hir.Type{namedType("bool")}),
		"trim":       funcType([]hir.Type{namedType("string")}, []hir.Type{namedType("string")}),
		"replace":    funcType([]hir.Type{namedType("string"), namedType("string"), namedType("string")}, []hir.Type{namedType("string")}),
	},
	"math": {
		"abs":   funcType([]hir.Type{namedType("dynamic")}, []hir.Type{namedType("dynamic")}),
		"min":   funcType([]hir.Type{namedType("dynamic"), namedType("dynamic")}, []hir.Type{namedType("dynamic")}),
		"max":   funcType([]hir.Type{namedType("dynamic"), namedType("dynamic")}, []hir.Type{namedType("dynamic")}),
		"sqrt":  funcType([]hir.Type{namedType("float")}, []hir.Type{namedType("float")}),
		"floor": funcType([]hir.Type{namedType("float")}, []hir.Type{namedType("float")}),
		"ceil":  funcType([]hir.Type{namedType("float")}, []hir.Type{namedType("float")}),
	},
	"dict": {
		"haskey": funcType(nil, []hir.Type{namedType("bool")}),
		"get":    funcType(nil, nil),
		"keys":   funcType(nil, []hir.Type{namedType("list")}),
		"values": funcType(nil, []hir.Type{namedType("list")}),
		"items":  funcType(nil, []hir.Type{genericType("list", namedType("list"))}),
	},
	"io": {
		"readfile":   funcType([]hir.Type{namedType("string")}, []hir.Type{genericType("result", namedType("string"))}),
		"readdir":    funcType([]hir.Type{namedType("string")}, []hir.Type{genericType("result", genericType("list", namedType("string")))}),
		"writefile":  funcType([]hir.Type{namedType("string"), namedType("string")}, []hir.Type{genericType("result", namedType("int"))}),
		"appendfile": funcType([]hir.Type{namedType("string"), namedType("string")}, []hir.Type{genericType("result", namedType("int"))}),
	},
	"path": {
		"basename": funcType([]hir.Type{namedType("string")}, []hir.Type{namedType("string")}),
		"dirname":  funcType([]hir.Type{namedType("string")}, []hir.Type{namedType("string")}),
		"joinpath": funcType([]hir.Type{namedType("string"), namedType("string")}, []hir.Type{namedType("string")}),
	},
	"os": {
		"args":   funcType(nil, []hir.Type{genericType("list", namedType("string"))}),
		"cwd":    funcType(nil, []hir.Type{namedType("string")}),
		"getenv": funcType([]hir.Type{namedType("string")}, []hir.Type{genericType("result", namedType("string"))}),
	},
	"time": {
		"sleep":      funcType([]hir.Type{namedType("int")}, nil),
		"nowunix":    funcType(nil, []hir.Type{namedType("int")}),
		"nowunixms":  funcType(nil, []hir.Type{namedType("int")}),
		"nowrfc3339": funcType(nil, []hir.Type{namedType("string")}),
	},
	"json": {
		"parseobject": funcType([]hir.Type{namedType("string")}, []hir.Type{genericType("result", namedType("dict"))}),
		"parsearray":  funcType([]hir.Type{namedType("string")}, []hir.Type{genericType("result", namedType("list"))}),
		"stringify":   funcType([]hir.Type{namedType("dynamic")}, []hir.Type{genericType("result", namedType("string"))}),
		"objectof":    funcType(nil, []hir.Type{namedType("dict")}),
		"arrayof":     funcType(nil, []hir.Type{namedType("list")}),
		"null":        funcType(nil, []hir.Type{namedType("JsonNull")}),
		"isnull":      funcType([]hir.Type{namedType("dynamic")}, []hir.Type{namedType("bool")}),
	},
	"http": {
		"get":            funcType(nil, []hir.Type{genericType("result", namedType("HttpResponse"))}),
		"request":        funcType(nil, []hir.Type{genericType("result", namedType("HttpResponse"))}),
		"listen":         funcType(nil, []hir.Type{genericType("result", namedType("HttpServer"))}),
		"addr":           funcType(nil, []hir.Type{namedType("string")}),
		"wait":           funcType(nil, []hir.Type{genericType("result", namedType("int"))}),
		"close":          funcType(nil, []hir.Type{genericType("result", namedType("int"))}),
		"method":         funcType(nil, []hir.Type{namedType("string")}),
		"path":           funcType(nil, []hir.Type{namedType("string")}),
		"requestbody":    funcType(nil, []hir.Type{namedType("string")}),
		"requestheader":  funcType(nil, []hir.Type{namedType("string")}),
		"requestcookie":  funcType(nil, []hir.Type{namedType("string")}),
		"status":         funcType(nil, []hir.Type{namedType("int")}),
		"responsebody":   funcType(nil, []hir.Type{namedType("string")}),
		"responseheader": funcType(nil, []hir.Type{namedType("string")}),
		"query":          funcType(nil, []hir.Type{namedType("string")}),
		"route":          funcType(nil, []hir.Type{namedType("bool"), genericType("dict", namedType("string"), namedType("string"))}),
		"text":           funcType(nil, []hir.Type{namedType("HttpReply")}),
		"html":           funcType(nil, []hir.Type{namedType("HttpReply")}),
		"json":           funcType(nil, []hir.Type{genericType("result", namedType("HttpReply"))}),
		"redirect":       funcType(nil, []hir.Type{namedType("HttpReply")}),
		"withheader":     funcType(nil, []hir.Type{namedType("HttpReply")}),
		"withcookie":     funcType(nil, []hir.Type{namedType("HttpReply")}),
		"static":         funcType(nil, []hir.Type{namedType("bool"), genericType("result", namedType("HttpReply"))}),
	},
	"state": {
		"cell":   funcType(nil, []hir.Type{namedType("cell")}),
		"get":    funcType(nil, nil),
		"set":    funcType(nil, nil),
		"update": funcType(nil, nil),
	},
	"sqlite": {
		"open":  funcType([]hir.Type{namedType("string")}, []hir.Type{genericType("result", namedType("SqliteDB"))}),
		"close": funcType([]hir.Type{namedType("SqliteDB")}, []hir.Type{genericType("result", namedType("int"))}),
		"exec":  funcType([]hir.Type{namedType("SqliteDB"), namedType("string"), namedType("list")}, []hir.Type{genericType("result", namedType("int"))}),
		"query": funcType([]hir.Type{namedType("SqliteDB"), namedType("string"), namedType("list")}, []hir.Type{genericType("result", genericType("list", namedType("dict")))}),
	},
}

var builtinCallableTypes = func() map[string]*hir.FuncType {
	builtins := map[string]*hir.FuncType{
		"write":  funcType(nil, nil),
		"read":   funcType([]hir.Type{namedType("string")}, []hir.Type{namedType("string")}),
		"len":    funcType([]hir.Type{namedType("dynamic")}, []hir.Type{namedType("int")}),
		"str":    funcType([]hir.Type{namedType("dynamic")}, []hir.Type{namedType("string")}),
		"int":    funcType([]hir.Type{namedType("dynamic")}, []hir.Type{namedType("int")}),
		"float":  funcType([]hir.Type{namedType("dynamic")}, []hir.Type{namedType("float")}),
		"typeof": funcType([]hir.Type{namedType("dynamic")}, []hir.Type{namedType("string")}),
	}
	for _, exports := range stdlibCallableTypes {
		for name, signature := range exports {
			builtins[name] = signature
		}
	}
	return builtins
}()
