package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Cass-ette/gwen-lang/internal/ast"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

type SemanticError struct {
	Message string
	Line    int
}

func (e *SemanticError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("semantic error at L%d: %s", e.Line, e.Message)
	}
	return e.Message
}

type Checker struct {
	globalScope       *Scope
	modules           map[string]*ModuleInfo
	moduleSearchPaths []string
	loadingModules    map[string]struct{}
}

type ModuleInfo struct {
	Name           string
	RuntimeExports map[string]*ValueInfo
	TypeExports    map[string]string
}

type ObjectInfo struct {
	Name           string
	Fields         map[string]string
	Methods        map[string]*CallableInfo
	HasConstructor bool
	Constructor    *CallableInfo
}

type Scope struct {
	parent              *Scope
	values              map[string]*ValueInfo
	aliases             map[string]string
	objects             map[string]*ObjectInfo
	expectedReturnTypes []string
	methodSelfType      string
}

type ValueInfo struct {
	Kind           string
	TypeName       string
	MultiTypeNames []string
	ObjectInfo     *ObjectInfo
	ModuleInfo     *ModuleInfo
	CallableInfo   *CallableInfo
}

type CallableInfo struct {
	Label           string
	Params          []ParamInfo
	ReturnTypeNames []string
	Variadic        bool
	DefinitionScope *Scope
}

type ParamInfo struct {
	Name       string
	TypeName   string
	HasDefault bool
	Line       int
}

var baseTypeNames = map[string]struct{}{
	"int":          {},
	"float":        {},
	"string":       {},
	"bool":         {},
	"int8":         {},
	"int16":        {},
	"int32":        {},
	"int64":        {},
	"uint8":        {},
	"uint16":       {},
	"uint32":       {},
	"uint64":       {},
	"float32":      {},
	"float64":      {},
	"list":         {},
	"dict":         {},
	"func":         {},
	"result":       {},
	"HttpResponse": {},
	"JsonNull":     {},
}

type arity struct {
	min int
	max int
}

var genericBaseArity = map[string]arity{
	"list":   {min: 1, max: 1},
	"dict":   {min: 2, max: 2},
	"money":  {min: 1, max: 1},
	"result": {min: 1, max: -1},
}

const (
	resultOkBase  = "result_ok"
	resultErrBase = "result_err"
)

type resultTypeInfo struct {
	bare         bool
	okOnly       bool
	errOnly      bool
	okType       string
	errTypes     []string
	implicitErrs bool
}

var stdlibModules = map[string][]string{
	"list": {
		"append",
		"pop",
		"concat",
		"removeat",
		"insert",
		"sort",
		"asc",
		"desc",
		"reversed",
		"map",
		"filter",
		"range",
		"enumerate",
	},
	"string": {
		"split",
		"join",
		"substring",
		"contains",
		"trim",
		"replace",
	},
	"math": {
		"abs",
		"min",
		"max",
		"sqrt",
		"floor",
		"ceil",
	},
	"dict": {
		"haskey",
		"get",
		"keys",
		"values",
		"items",
	},
	"io": {
		"readfile",
		"writefile",
		"appendfile",
	},
	"os": {
		"args",
		"cwd",
		"getenv",
	},
	"time": {
		"sleep",
		"nowunix",
		"nowunixms",
		"nowrfc3339",
	},
	"json": {},
	"http": {},
}

var moduleOnlyBuiltins = map[string]struct{}{
	"args":       {},
	"cwd":        {},
	"getenv":     {},
	"sleep":      {},
	"nowunix":    {},
	"nowunixms":  {},
	"nowrfc3339": {},
}

func New() *Checker {
	globalScope := newScope(nil)
	checker := &Checker{
		globalScope:       globalScope,
		modules:           map[string]*ModuleInfo{},
		moduleSearchPaths: []string{},
		loadingModules:    map[string]struct{}{},
	}
	checker.setupBuiltins()
	checker.setupStdlibModules()
	checker.hideModuleOnlyBuiltins()
	return checker
}

func (c *Checker) CheckProgram(program *ast.Program, sourcePath string) error {
	if sourcePath != "" {
		c.AddModuleSearchPath(filepath.Dir(sourcePath))
	}

	deferred := []func() error{}
	if err := c.checkBlock(program.Statements, c.globalScope, &deferred); err != nil {
		return err
	}
	return c.runDeferred(&deferred)
}

func (c *Checker) AddModuleSearchPath(path string) {
	if path == "" {
		return
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	for _, existing := range c.moduleSearchPaths {
		if existing == absPath {
			return
		}
	}
	c.moduleSearchPaths = append([]string{absPath}, c.moduleSearchPaths...)
}

func (c *Checker) setupBuiltins() {
	for name, signature := range builtinSignatures() {
		if signature.DefinitionScope == nil {
			signature.DefinitionScope = c.globalScope
		}
		c.globalScope.defineValue(name, &ValueInfo{
			Kind:         "builtin",
			TypeName:     signature.typeName(),
			CallableInfo: signature,
		})
	}
}

func (c *Checker) setupStdlibModules() {
	for moduleName, names := range stdlibModules {
		module := c.ensureStdlibModule(moduleName)
		for _, name := range names {
			if value, ok := c.globalScope.resolveLocalValue(name); ok {
				module.RuntimeExports[name] = value
			}
		}
	}
	c.addStdlibModuleCallable("http", "get", &CallableInfo{
		Label: "get",
		Params: []ParamInfo{
			{Name: "url", TypeName: "string"},
			{Name: "timeoutms", TypeName: "int", HasDefault: true},
		},
		ReturnTypeNames: []string{"result[HttpResponse]"},
	})
	c.addStdlibModuleCallable("http", "status", &CallableInfo{
		Label: "status",
		Params: []ParamInfo{
			{Name: "response", TypeName: "HttpResponse"},
		},
		ReturnTypeNames: []string{"int"},
	})
	c.addStdlibModuleCallable("http", "body", &CallableInfo{
		Label: "body",
		Params: []ParamInfo{
			{Name: "response", TypeName: "HttpResponse"},
		},
		ReturnTypeNames: []string{"string"},
	})
	c.addStdlibModuleCallable("json", "parseobject", &CallableInfo{
		Label: "parseobject",
		Params: []ParamInfo{
			{Name: "text", TypeName: "string"},
		},
		ReturnTypeNames: []string{"result[dict]"},
	})
	c.addStdlibModuleCallable("json", "parsearray", &CallableInfo{
		Label: "parsearray",
		Params: []ParamInfo{
			{Name: "text", TypeName: "string"},
		},
		ReturnTypeNames: []string{"result[list]"},
	})
	c.addStdlibModuleCallable("json", "stringify", &CallableInfo{
		Label: "stringify",
		Params: []ParamInfo{
			{Name: "value"},
		},
		ReturnTypeNames: []string{"result[string]"},
	})
	c.addStdlibModuleCallable("json", "objectof", &CallableInfo{
		Label:           "objectof",
		Params:          []ParamInfo{{Name: "items"}},
		ReturnTypeNames: []string{"dict"},
		Variadic:        true,
	})
	c.addStdlibModuleCallable("json", "arrayof", &CallableInfo{
		Label:           "arrayof",
		Params:          []ParamInfo{{Name: "items"}},
		ReturnTypeNames: []string{"list"},
		Variadic:        true,
	})
	c.addStdlibModuleCallable("json", "null", &CallableInfo{
		Label:           "null",
		ReturnTypeNames: []string{"JsonNull"},
	})
	c.addStdlibModuleCallable("json", "isnull", &CallableInfo{
		Label: "isnull",
		Params: []ParamInfo{
			{Name: "value"},
		},
		ReturnTypeNames: []string{"bool"},
	})
}

func (c *Checker) hideModuleOnlyBuiltins() {
	for name := range moduleOnlyBuiltins {
		delete(c.globalScope.values, name)
	}
}

func (c *Checker) ensureStdlibModule(name string) *ModuleInfo {
	if module, ok := c.modules[name]; ok {
		return module
	}
	module := newModuleInfo(name)
	c.modules[name] = module
	return module
}

func (c *Checker) addStdlibModuleCallable(moduleName, exportName string, signature *CallableInfo) {
	if signature.DefinitionScope == nil {
		signature.DefinitionScope = c.globalScope
	}
	c.ensureStdlibModule(moduleName).RuntimeExports[exportName] = &ValueInfo{
		Kind:         "builtin",
		TypeName:     signature.typeName(),
		CallableInfo: signature,
	}
}

func builtinSignatures() map[string]*CallableInfo {
	return map[string]*CallableInfo{
		"write":      {Label: "write", Variadic: true},
		"read":       {Label: "read", Params: []ParamInfo{{Name: "prompt", HasDefault: true}}, ReturnTypeNames: []string{"string"}},
		"len":        {Label: "len", Params: []ParamInfo{{Name: "obj"}}, ReturnTypeNames: []string{"int"}},
		"str":        {Label: "str", Params: []ParamInfo{{Name: "obj"}}, ReturnTypeNames: []string{"string"}},
		"int":        {Label: "int", Params: []ParamInfo{{Name: "obj"}}, ReturnTypeNames: []string{"int"}},
		"float":      {Label: "float", Params: []ParamInfo{{Name: "obj"}}, ReturnTypeNames: []string{"float"}},
		"append":     {Label: "append", Params: []ParamInfo{{Name: "lst", TypeName: "list"}, {Name: "item"}}},
		"typeof":     {Label: "typeof", Params: []ParamInfo{{Name: "obj"}}, ReturnTypeNames: []string{"string"}},
		"sort":       {Label: "sort", Params: []ParamInfo{{Name: "lst"}, {Name: "cmp"}}},
		"asc":        {Label: "asc", Params: []ParamInfo{{Name: "a"}, {Name: "b"}}, ReturnTypeNames: []string{"bool"}},
		"desc":       {Label: "desc", Params: []ParamInfo{{Name: "a"}, {Name: "b"}}, ReturnTypeNames: []string{"bool"}},
		"reversed":   {Label: "reversed", Params: []ParamInfo{{Name: "lst"}}},
		"map":        {Label: "map", Params: []ParamInfo{{Name: "lst", TypeName: "list"}, {Name: "f"}}, ReturnTypeNames: []string{"list"}},
		"filter":     {Label: "filter", Params: []ParamInfo{{Name: "lst", TypeName: "list"}, {Name: "pred"}}, ReturnTypeNames: []string{"list"}},
		"range":      {Label: "range", Params: []ParamInfo{{Name: "start", TypeName: "int"}, {Name: "end", TypeName: "int"}, {Name: "step", TypeName: "int", HasDefault: true}}, ReturnTypeNames: []string{"list[int]"}},
		"enumerate":  {Label: "enumerate", Params: []ParamInfo{{Name: "lst", TypeName: "list"}}, ReturnTypeNames: []string{"list"}},
		"split":      {Label: "split", Params: []ParamInfo{{Name: "s", TypeName: "string"}, {Name: "sep", TypeName: "string"}}, ReturnTypeNames: []string{"list[string]"}},
		"join":       {Label: "join", Params: []ParamInfo{{Name: "parts", TypeName: "list"}, {Name: "sep", TypeName: "string"}}, ReturnTypeNames: []string{"string"}},
		"pop":        {Label: "pop", Params: []ParamInfo{{Name: "lst"}}},
		"removeat":   {Label: "removeat", Params: []ParamInfo{{Name: "lst"}, {Name: "idx", TypeName: "int"}}},
		"insert":     {Label: "insert", Params: []ParamInfo{{Name: "lst"}, {Name: "idx", TypeName: "int"}, {Name: "item"}}},
		"concat":     {Label: "concat", Params: []ParamInfo{{Name: "a", TypeName: "list"}, {Name: "b", TypeName: "list"}}, ReturnTypeNames: []string{"list"}},
		"substring":  {Label: "substring", Params: []ParamInfo{{Name: "s", TypeName: "string"}, {Name: "start", TypeName: "int"}, {Name: "end", TypeName: "int"}}, ReturnTypeNames: []string{"string"}},
		"contains":   {Label: "contains", Params: []ParamInfo{{Name: "s"}, {Name: "substr"}}, ReturnTypeNames: []string{"bool"}},
		"trim":       {Label: "trim", Params: []ParamInfo{{Name: "s", TypeName: "string"}}, ReturnTypeNames: []string{"string"}},
		"replace":    {Label: "replace", Params: []ParamInfo{{Name: "s", TypeName: "string"}, {Name: "old", TypeName: "string"}, {Name: "new", TypeName: "string"}}, ReturnTypeNames: []string{"string"}},
		"abs":        {Label: "abs", Params: []ParamInfo{{Name: "x"}}},
		"min":        {Label: "min", Params: []ParamInfo{{Name: "a"}, {Name: "b"}}},
		"max":        {Label: "max", Params: []ParamInfo{{Name: "a"}, {Name: "b"}}},
		"sqrt":       {Label: "sqrt", Params: []ParamInfo{{Name: "x"}}, ReturnTypeNames: []string{"float"}},
		"floor":      {Label: "floor", Params: []ParamInfo{{Name: "x"}}, ReturnTypeNames: []string{"float"}},
		"ceil":       {Label: "ceil", Params: []ParamInfo{{Name: "x"}}, ReturnTypeNames: []string{"float"}},
		"haskey":     {Label: "haskey", Params: []ParamInfo{{Name: "d", TypeName: "dict"}, {Name: "key"}}, ReturnTypeNames: []string{"bool"}},
		"get":        {Label: "get", Params: []ParamInfo{{Name: "d", TypeName: "dict"}, {Name: "key"}, {Name: "default"}}},
		"keys":       {Label: "keys", Params: []ParamInfo{{Name: "d", TypeName: "dict"}}, ReturnTypeNames: []string{"list"}},
		"values":     {Label: "values", Params: []ParamInfo{{Name: "d", TypeName: "dict"}}, ReturnTypeNames: []string{"list"}},
		"items":      {Label: "items", Params: []ParamInfo{{Name: "d", TypeName: "dict"}}, ReturnTypeNames: []string{"list"}},
		"readfile":   {Label: "readfile", Params: []ParamInfo{{Name: "path", TypeName: "string"}}, ReturnTypeNames: []string{"result[string]"}},
		"writefile":  {Label: "writefile", Params: []ParamInfo{{Name: "path", TypeName: "string"}, {Name: "content", TypeName: "string"}}, ReturnTypeNames: []string{"result[int]"}},
		"appendfile": {Label: "appendfile", Params: []ParamInfo{{Name: "path", TypeName: "string"}, {Name: "content", TypeName: "string"}}, ReturnTypeNames: []string{"result[int]"}},
		"args":       {Label: "args", ReturnTypeNames: []string{"list[string]"}},
		"cwd":        {Label: "cwd", ReturnTypeNames: []string{"string"}},
		"getenv":     {Label: "getenv", Params: []ParamInfo{{Name: "name", TypeName: "string"}}, ReturnTypeNames: []string{"result[string]"}},
		"sleep":      {Label: "sleep", Params: []ParamInfo{{Name: "ms", TypeName: "int"}}},
		"nowunix":    {Label: "nowunix", ReturnTypeNames: []string{"int"}},
		"nowunixms":  {Label: "nowunixms", ReturnTypeNames: []string{"int"}},
		"nowrfc3339": {Label: "nowrfc3339", ReturnTypeNames: []string{"string"}},
	}
}

func newScope(parent *Scope) *Scope {
	var expectedReturnTypes []string
	methodSelfType := ""
	if parent != nil {
		expectedReturnTypes = parent.expectedReturnTypes
		methodSelfType = parent.methodSelfType
	}
	return &Scope{
		parent:              parent,
		values:              map[string]*ValueInfo{},
		aliases:             map[string]string{},
		objects:             map[string]*ObjectInfo{},
		expectedReturnTypes: expectedReturnTypes,
		methodSelfType:      methodSelfType,
	}
}

func newReturnScope(parent *Scope, expectedReturnTypes []string) *Scope {
	methodSelfType := ""
	if parent != nil {
		methodSelfType = parent.methodSelfType
	}
	return &Scope{
		parent:              parent,
		values:              map[string]*ValueInfo{},
		aliases:             map[string]string{},
		objects:             map[string]*ObjectInfo{},
		expectedReturnTypes: expectedReturnTypes,
		methodSelfType:      methodSelfType,
	}
}

func newMethodScope(parent *Scope, expectedReturnTypes []string, methodSelfType string) *Scope {
	return &Scope{
		parent:              parent,
		values:              map[string]*ValueInfo{},
		aliases:             map[string]string{},
		objects:             map[string]*ObjectInfo{},
		expectedReturnTypes: expectedReturnTypes,
		methodSelfType:      methodSelfType,
	}
}

func cloneScope(base *Scope) *Scope {
	cloned := &Scope{
		parent:              base.parent,
		values:              map[string]*ValueInfo{},
		aliases:             map[string]string{},
		objects:             map[string]*ObjectInfo{},
		expectedReturnTypes: base.expectedReturnTypes,
		methodSelfType:      base.methodSelfType,
	}
	for name, value := range base.values {
		cloned.values[name] = value
	}
	for name, target := range base.aliases {
		cloned.aliases[name] = target
	}
	for name, info := range base.objects {
		cloned.objects[name] = info
	}
	return cloned
}

func (s *Scope) defineValue(name string, value *ValueInfo) {
	s.values[name] = value
}

func (s *Scope) resolveValue(name string) (*ValueInfo, bool) {
	if value, ok := s.values[name]; ok {
		return value, true
	}
	if s.parent != nil {
		return s.parent.resolveValue(name)
	}
	return nil, false
}

func (s *Scope) resolveLocalValue(name string) (*ValueInfo, bool) {
	value, ok := s.values[name]
	return value, ok
}

func (s *Scope) resolveOuterValue(name string) (*ValueInfo, bool) {
	for current := s.parent; current != nil; current = current.parent {
		if value, ok := current.values[name]; ok {
			return value, true
		}
	}
	return nil, false
}

func (s *Scope) defineAlias(name, target string) {
	s.aliases[name] = target
}

func (s *Scope) resolveAliasRaw(name string) (string, bool) {
	if target, ok := s.aliases[name]; ok {
		return target, true
	}
	if s.parent != nil {
		return s.parent.resolveAliasRaw(name)
	}
	return "", false
}

func (s *Scope) resolveLocalAlias(name string) (string, bool) {
	target, ok := s.aliases[name]
	return target, ok
}

func (s *Scope) defineObject(info *ObjectInfo) {
	s.objects[info.Name] = info
}

func (s *Scope) resolveObject(name string) (*ObjectInfo, bool) {
	if info, ok := s.objects[name]; ok {
		return info, true
	}
	if s.parent != nil {
		return s.parent.resolveObject(name)
	}
	return nil, false
}

func newModuleInfo(name string) *ModuleInfo {
	return &ModuleInfo{
		Name:           name,
		RuntimeExports: map[string]*ValueInfo{},
		TypeExports:    map[string]string{},
	}
}

func (c *Checker) runDeferred(deferred *[]func() error) error {
	for len(*deferred) > 0 {
		callback := (*deferred)[0]
		*deferred = (*deferred)[1:]
		if err := callback(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Checker) applyClonedScope(dst *Scope, src *Scope) {
	dst.values = map[string]*ValueInfo{}
	for name, value := range src.values {
		dst.values[name] = value
	}
	dst.aliases = map[string]string{}
	for name, target := range src.aliases {
		dst.aliases[name] = target
	}
	dst.objects = map[string]*ObjectInfo{}
	for name, info := range src.objects {
		dst.objects[name] = info
	}
}

func (c *Checker) mergeAlternativeScopes(base *Scope, dst *Scope, branches []*Scope, context string, line int) error {
	if len(branches) == 0 {
		c.applyClonedScope(dst, base)
		return nil
	}

	mergedValues := map[string]*ValueInfo{}
	mergedAliases := map[string]string{}
	mergedObjects := map[string]*ObjectInfo{}

	for name, value := range base.values {
		merged := value
		for _, branch := range branches {
			branchValue, ok := branch.values[name]
			if !ok {
				branchValue = value
			}
			next, err := c.mergeValueInfo(dst, name, merged, branchValue, context, line)
			if err != nil {
				return err
			}
			merged = next
		}
		mergedValues[name] = merged
	}

	for name := range intersectNewValueNames(base, branches) {
		merged := branches[0].values[name]
		for _, branch := range branches[1:] {
			next, err := c.mergeValueInfo(dst, name, merged, branch.values[name], context, line)
			if err != nil {
				return err
			}
			merged = next
		}
		mergedValues[name] = merged
	}

	for name, target := range base.aliases {
		for _, branch := range branches {
			if branchTarget, ok := branch.aliases[name]; ok && branchTarget != target {
				return semanticErrorf(line, "Type alias '%s' has inconsistent targets across %s branches: %s vs %s", name, context, target, branchTarget)
			}
		}
		mergedAliases[name] = target
	}
	for name := range intersectNewAliasNames(base, branches) {
		target := branches[0].aliases[name]
		for _, branch := range branches[1:] {
			if branch.aliases[name] != target {
				return semanticErrorf(line, "Type alias '%s' has inconsistent targets across %s branches: %s vs %s", name, context, target, branch.aliases[name])
			}
		}
		mergedAliases[name] = target
	}

	for name, info := range base.objects {
		for _, branch := range branches {
			if branchInfo, ok := branch.objects[name]; ok && !reflect.DeepEqual(branchInfo, info) {
				return semanticErrorf(line, "Object '%s' has inconsistent definitions across %s branches", name, context)
			}
		}
		mergedObjects[name] = info
	}
	for name := range intersectNewObjectNames(base, branches) {
		info := branches[0].objects[name]
		for _, branch := range branches[1:] {
			if !reflect.DeepEqual(branch.objects[name], info) {
				return semanticErrorf(line, "Object '%s' has inconsistent definitions across %s branches", name, context)
			}
		}
		mergedObjects[name] = info
	}

	dst.values = mergedValues
	dst.aliases = mergedAliases
	dst.objects = mergedObjects
	return nil
}

func intersectNewValueNames(base *Scope, branches []*Scope) map[string]struct{} {
	result := map[string]struct{}{}
	for name := range branches[0].values {
		if _, exists := base.values[name]; exists {
			continue
		}
		result[name] = struct{}{}
	}
	for _, branch := range branches[1:] {
		for name := range result {
			if _, ok := branch.values[name]; !ok {
				delete(result, name)
			}
		}
	}
	return result
}

func intersectNewAliasNames(base *Scope, branches []*Scope) map[string]struct{} {
	result := map[string]struct{}{}
	for name := range branches[0].aliases {
		if _, exists := base.aliases[name]; exists {
			continue
		}
		result[name] = struct{}{}
	}
	for _, branch := range branches[1:] {
		for name := range result {
			if _, ok := branch.aliases[name]; !ok {
				delete(result, name)
			}
		}
	}
	return result
}

func intersectNewObjectNames(base *Scope, branches []*Scope) map[string]struct{} {
	result := map[string]struct{}{}
	for name := range branches[0].objects {
		if _, exists := base.objects[name]; exists {
			continue
		}
		result[name] = struct{}{}
	}
	for _, branch := range branches[1:] {
		for name := range result {
			if _, ok := branch.objects[name]; !ok {
				delete(result, name)
			}
		}
	}
	return result
}

func staticBool(expr any) (bool, bool) {
	switch node := expr.(type) {
	case *ast.BoolLiteral:
		return node.Value, true
	case *ast.UnaryOp:
		if node.Op == "not" {
			value, ok := staticBool(node.Operand)
			if ok {
				return !value, true
			}
		}
	case *ast.BinaryOp:
		if node.Op == "and" || node.Op == "or" {
			left, leftOK := staticBool(node.Left)
			right, rightOK := staticBool(node.Right)
			if leftOK && rightOK {
				if node.Op == "and" {
					return left && right, true
				}
				return left || right, true
			}
		}
	}
	return false, false
}

func staticInt(expr any) (int64, bool) {
	switch node := expr.(type) {
	case *ast.IntLiteral:
		return node.Value, true
	case *ast.UnaryOp:
		if node.Op == "-" {
			value, ok := staticInt(node.Operand)
			if ok {
				return -value, true
			}
		}
	}
	return 0, false
}

func rangeRunsAtLeastOnce(node *ast.ForRangeStmt) bool {
	if node.Direction == "asc" || node.Direction == "desc" || node.Step == nil {
		return true
	}
	start, startOK := staticInt(node.Start)
	end, endOK := staticInt(node.End)
	step, stepOK := staticInt(node.Step)
	if !startOK || !endOK || !stepOK || step == 0 {
		return false
	}
	return compareRange(start, end, step)
}

func compareRange(current, end, step int64) bool {
	if step > 0 {
		return current <= end
	}
	return current >= end
}

func forEachRunsAtLeastOnce(node *ast.ForEachStmt) bool {
	switch iterable := node.Iterable.(type) {
	case *ast.ListLiteral:
		return len(iterable.Elements) > 0
	case *ast.StringLiteral:
		return iterable.Value != ""
	default:
		return false
	}
}

func (c *Checker) mergeValueInfo(scope *Scope, name string, left *ValueInfo, right *ValueInfo, context string, line int) (*ValueInfo, error) {
	if left == nil {
		return right, nil
	}
	if right == nil {
		return left, nil
	}

	leftType := c.valueTypeName(left, scope)
	rightType := c.valueTypeName(right, scope)
	if leftType != "" && leftType == rightType {
		return left, nil
	}
	if mergedType, ok := c.mergeTypeNames(leftType, rightType); ok {
		return c.valueFromDeclaredType(mergedType, scope, name), nil
	}
	if reflect.DeepEqual(left.CallableInfo, right.CallableInfo) &&
		reflect.DeepEqual(left.ObjectInfo, right.ObjectInfo) &&
		reflect.DeepEqual(left.ModuleInfo, right.ModuleInfo) &&
		left.Kind == right.Kind &&
		left.TypeName == right.TypeName &&
		reflect.DeepEqual(left.MultiTypeNames, right.MultiTypeNames) {
		return left, nil
	}
	if leftType != "" && rightType != "" {
		return nil, semanticErrorf(line, "Variable '%s' has inconsistent types across %s branches: %s vs %s", name, context, displayTypeName(leftType), displayTypeName(rightType))
	}
	return unknownValue(), nil
}

func (c *Checker) mergeTypeNames(leftType string, rightType string) (string, bool) {
	switch {
	case leftType == "" || rightType == "":
		return "", false
	case leftType == rightType:
		return leftType, true
	case isFloatType(leftType) && (isFloatType(rightType) || isIntType(rightType)):
		return "float", true
	case isFloatType(rightType) && (isFloatType(leftType) || isIntType(leftType)):
		return "float", true
	case isIntType(leftType) && isIntType(rightType):
		return "int", true
	}

	if merged, ok := c.mergeResultTypeNames(leftType, rightType); ok {
		return merged, true
	}

	leftBase := typeBase(leftType)
	rightBase := typeBase(rightType)
	if leftBase == rightBase && (leftBase == "list" || leftBase == "dict" || leftBase == "result") {
		if leftType == leftBase || rightType == rightBase {
			return leftBase, true
		}
	}
	if c.typesCompatible(leftType, rightType) {
		return leftType, true
	}
	if c.typesCompatible(rightType, leftType) {
		return rightType, true
	}
	return "", false
}

func (c *Checker) mergeResultTypeNames(leftType string, rightType string) (string, bool) {
	left, ok := parseResultTypeInfo(leftType)
	if !ok {
		return "", false
	}
	right, ok := parseResultTypeInfo(rightType)
	if !ok {
		return "", false
	}

	if left.bare {
		return renderResultTypeInfo(right), true
	}
	if right.bare {
		return renderResultTypeInfo(left), true
	}

	okType, ok := c.mergeOptionalTypeNames(left.okType, right.okType)
	if !ok {
		return "", false
	}

	errTypes, ok := c.mergeResultErrTypes(left.errTypes, right.errTypes)
	if !ok {
		return "", false
	}

	return renderResultTypeInfo(resultTypeInfo{
		okType:       okType,
		errTypes:     errTypes,
		implicitErrs: okType != "" && len(errTypes) == 1 && errTypes[0] == "string",
	}), true
}

func (c *Checker) mergeOptionalTypeNames(leftType string, rightType string) (string, bool) {
	switch {
	case leftType == "":
		return rightType, true
	case rightType == "":
		return leftType, true
	case leftType == rightType:
		return leftType, true
	default:
		return c.mergeTypeNames(leftType, rightType)
	}
}

func (c *Checker) mergeResultErrTypes(leftErrTypes []string, rightErrTypes []string) ([]string, bool) {
	if len(leftErrTypes) == 0 && len(rightErrTypes) == 0 {
		return nil, true
	}
	if len(leftErrTypes) == 0 {
		return append([]string{}, rightErrTypes...), true
	}
	if len(rightErrTypes) == 0 {
		return append([]string{}, leftErrTypes...), true
	}

	merged := append([]string{}, leftErrTypes...)
	for _, current := range rightErrTypes {
		matched := false
		for idx, existing := range merged {
			next, ok := c.mergeOptionalTypeNames(existing, current)
			if ok {
				merged[idx] = next
				matched = true
				break
			}
		}
		if !matched {
			merged = append(merged, current)
		}
	}
	return merged, true
}

func (c *Checker) callableFromParams(label string, params []*ast.Param, returnType any, scope *Scope) (*CallableInfo, error) {
	callable := &CallableInfo{
		Label:           label,
		Params:          make([]ParamInfo, 0, len(params)),
		ReturnTypeNames: renderReturnTypes(returnType),
		DefinitionScope: scope,
	}
	for _, param := range params {
		typeName := ""
		if param.TypeName != nil {
			rendered, ok := renderType(param.TypeName)
			if ok {
				typeName = rendered
			}
		}
		callable.Params = append(callable.Params, ParamInfo{
			Name:       param.Name,
			TypeName:   typeName,
			HasDefault: param.Default != nil,
			Line:       param.Line,
		})
	}
	return callable, nil
}

func (c *Checker) valueFromDeclaredType(typeName string, scope *Scope, label string) *ValueInfo {
	value := &ValueInfo{
		Kind:     "variable",
		TypeName: typeName,
	}
	if obj, ok := c.tryResolveObjectInfo(typeName, scope); ok {
		value.ObjectInfo = obj
	}
	if callable := callableFromTypeName(typeName, label, scope); callable != nil {
		value.CallableInfo = callable
	}
	return value
}

func (c *Checker) copyValue(value *ValueInfo, scope *Scope) *ValueInfo {
	if value == nil {
		return unknownValue()
	}
	copied := *value
	if copied.TypeName != "" {
		if obj, ok := c.tryResolveObjectInfo(copied.TypeName, scope); ok {
			copied.ObjectInfo = obj
		}
		if copied.CallableInfo == nil {
			copied.CallableInfo = callableFromTypeName(copied.TypeName, copied.TypeName, scope)
		}
	}
	return &copied
}

func unknownValue() *ValueInfo {
	return &ValueInfo{Kind: "unknown"}
}

func literalValue(typeName string) *ValueInfo {
	return &ValueInfo{Kind: "literal", TypeName: typeName}
}

func (c *Checker) valueTypeName(value *ValueInfo, scope *Scope) string {
	if value == nil {
		return ""
	}
	if value.TypeName == "" {
		return ""
	}
	resolved, ok, err := c.resolveTypeNameString(value.TypeName, scope, 0, false)
	if err == nil && ok {
		return resolved
	}
	return value.TypeName
}

func (c *Checker) tryResolveObjectInfo(name string, scope *Scope) (*ObjectInfo, bool) {
	if name == "" {
		return nil, false
	}
	resolved, ok, err := c.resolveTypeNameString(name, scope, 0, false)
	if err == nil && ok {
		name = resolved
	}
	return scope.resolveObject(name)
}

func (ci *CallableInfo) typeName() string {
	if ci == nil {
		return ""
	}
	params := make([]string, 0, len(ci.Params))
	for _, param := range ci.Params {
		if param.TypeName == "" {
			return ""
		}
		params = append(params, param.TypeName)
	}
	returnType := "void"
	if len(ci.ReturnTypeNames) == 1 {
		returnType = ci.ReturnTypeNames[0]
	} else if len(ci.ReturnTypeNames) > 1 {
		returnType = strings.Join(ci.ReturnTypeNames, ", ")
	}
	return fmt.Sprintf("(%s) -> %s", strings.Join(params, ", "), returnType)
}

func callableFromTypeName(typeName string, label string, scope *Scope) *CallableInfo {
	if !strings.HasPrefix(typeName, "(") || !strings.Contains(typeName, "->") {
		return nil
	}
	paramText, returnText, ok := splitFuncType(typeName)
	if !ok {
		return nil
	}
	callable := &CallableInfo{Label: label, DefinitionScope: scope}
	if strings.TrimSpace(paramText) != "" {
		for idx, part := range splitTopLevel(paramText, ',') {
			callable.Params = append(callable.Params, ParamInfo{
				Name:     fmt.Sprintf("arg%d", idx+1),
				TypeName: strings.TrimSpace(part),
			})
		}
	}
	if strings.TrimSpace(returnText) != "" && strings.TrimSpace(returnText) != "void" {
		callable.ReturnTypeNames = []string{strings.TrimSpace(returnText)}
	}
	return callable
}

func splitFuncType(typeName string) (string, string, bool) {
	typeName = strings.TrimSpace(typeName)
	if !strings.HasPrefix(typeName, "(") {
		return "", "", false
	}
	depthParen := 0
	depthBracket := 0
	for i := 0; i < len(typeName)-1; i++ {
		switch typeName[i] {
		case '(':
			depthParen++
		case ')':
			depthParen--
		case '[':
			depthBracket++
		case ']':
			depthBracket--
		case '-':
			if typeName[i+1] == '>' && depthParen == 0 && depthBracket == 0 {
				params := strings.TrimSpace(typeName[:i])
				if len(params) < 2 || params[0] != '(' || params[len(params)-1] != ')' {
					return "", "", false
				}
				return params[1 : len(params)-1], strings.TrimSpace(typeName[i+2:]), true
			}
		}
	}
	return "", "", false
}

func splitTopLevel(text string, sep byte) []string {
	var parts []string
	depthParen := 0
	depthBracket := 0
	start := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(':
			depthParen++
		case ')':
			depthParen--
		case '[':
			depthBracket++
		case ']':
			depthBracket--
		default:
			if text[i] == sep && depthParen == 0 && depthBracket == 0 {
				part := strings.TrimSpace(text[start:i])
				if part != "" {
					parts = append(parts, part)
				}
				start = i + 1
			}
		}
	}
	part := strings.TrimSpace(text[start:])
	if part != "" {
		parts = append(parts, part)
	}
	return parts
}

func renderReturnTypes(returnType any) []string {
	switch node := returnType.(type) {
	case nil:
		return nil
	case []any:
		types := make([]string, 0, len(node))
		for _, item := range node {
			if rendered, ok := renderType(item); ok {
				types = append(types, rendered)
			}
		}
		return types
	default:
		if rendered, ok := renderType(node); ok {
			return []string{rendered}
		}
		return nil
	}
}

func (c *Checker) checkBlock(stmts []any, scope *Scope, deferred *[]func() error) error {
	for _, stmt := range stmts {
		if err := c.checkStmt(stmt, scope, deferred); err != nil {
			return err
		}
	}
	return nil
}

func (c *Checker) checkStmt(stmt any, scope *Scope, deferred *[]func() error) error {
	switch node := stmt.(type) {
	case *ast.FuncDef:
		signature, err := c.callableFromParams(node.Name, node.Params, node.ReturnType, scope)
		if err != nil {
			return err
		}
		scope.defineValue(node.Name, &ValueInfo{
			Kind:         "function",
			TypeName:     signature.typeName(),
			CallableInfo: signature,
		})
		deferredScope := scope
		*deferred = append(*deferred, func() error {
			return c.checkFuncBody(node, deferredScope)
		})
		return nil

	case *ast.Assignment:
		values := make([]*ValueInfo, 0, len(node.Values))
		for _, valueExpr := range node.Values {
			value, err := c.checkExpr(valueExpr, scope, deferred)
			if err != nil {
				return err
			}
			values = append(values, value)
		}
		assignedValues := values
		if len(node.Targets) > 1 && len(values) == 1 && len(values[0].MultiTypeNames) > 0 {
			assignedValues = make([]*ValueInfo, 0, len(values[0].MultiTypeNames))
			for _, typeName := range values[0].MultiTypeNames {
				assignedValues = append(assignedValues, c.valueFromDeclaredType(typeName, scope, typeName))
			}
		}
		if len(node.Targets) != len(assignedValues) {
			return semanticErrorf(node.Line, "Assignment count mismatch: %d targets, %d values", len(node.Targets), len(assignedValues))
		}
		for idx, target := range node.Targets {
			actualValue := assignedValues[idx]
			if name, ok := target.(string); ok {
				if existing, exists := scope.resolveLocalValue(name); exists {
					if existing.Kind == "builtin" {
						scope.defineValue(name, c.copyValue(actualValue, scope))
						continue
					}
					if err := c.validateValueAssignment(name, existing, actualValue, scope, node.Line); err != nil {
						return err
					}
					continue
				}
				scope.defineValue(name, c.copyValue(actualValue, scope))
				continue
			}
			if err := c.checkAssignmentTarget(target, scope, deferred, node.Line); err != nil {
				return err
			}
		}
		return nil

	case *ast.VarDecl:
		declaredType := ""
		if node.TypeName != nil {
			resolved, err := c.resolveTypeNode(node.TypeName, scope, node.Line)
			if err != nil {
				return err
			}
			declaredType = resolved
		}
		var inferred *ValueInfo
		if node.Value != nil {
			var err error
			inferred, err = c.checkExpr(node.Value, scope, deferred)
			if err != nil {
				return err
			}
		}
		value, err := c.bindDeclaredValue(node.Name, declaredType, inferred, scope, node.Line)
		if err != nil {
			return err
		}
		scope.defineValue(node.Name, value)
		return nil

	case *ast.VarBlock:
		if node.DefaultValue != nil {
			if _, err := c.checkExpr(node.DefaultValue, scope, deferred); err != nil {
				return err
			}
		}
		for _, decl := range node.Decls {
			if err := c.checkStmt(decl, scope, deferred); err != nil {
				return err
			}
		}
		return nil

	case *ast.TypeAlias:
		target, ok := renderType(node.Target)
		if !ok {
			return semanticErrorf(node.Line, "Invalid type alias target for '%s'", node.Name)
		}
		scope.defineAlias(node.Name, target)
		deferredScope := scope
		*deferred = append(*deferred, func() error {
			_, err := c.resolveTypeNode(node.Target, deferredScope, node.Line)
			return err
		})
		return nil

	case *ast.ReturnStmt:
		return c.checkReturnValue(node.Value, scope, deferred, node.Line)

	case *ast.IfStmt:
		if _, err := c.checkExpr(node.Condition, scope, deferred); err != nil {
			return err
		}
		branches := make([]*Scope, 0, len(node.Elifs)+2)
		remainderReachable := true
		if value, known := staticBool(node.Condition); !known || value {
			thenScope := cloneScope(scope)
			if err := c.checkBlock(node.Body, thenScope, deferred); err != nil {
				return err
			}
			branches = append(branches, thenScope)
			if known && value {
				remainderReachable = false
			}
		}
		for _, branch := range node.Elifs {
			if !remainderReachable {
				break
			}
			if _, err := c.checkExpr(branch.Condition, scope, deferred); err != nil {
				return err
			}
			if value, known := staticBool(branch.Condition); known && !value {
				continue
			}
			elifScope := cloneScope(scope)
			if err := c.checkBlock(branch.Body, elifScope, deferred); err != nil {
				return err
			}
			branches = append(branches, elifScope)
			if value, known := staticBool(branch.Condition); known && value {
				remainderReachable = false
			}
		}
		if remainderReachable && len(node.ElseBody) > 0 {
			elseScope := cloneScope(scope)
			if err := c.checkBlock(node.ElseBody, elseScope, deferred); err != nil {
				return err
			}
			branches = append(branches, elseScope)
			remainderReachable = false
		}
		if remainderReachable {
			branches = append(branches, cloneScope(scope))
		}
		return c.mergeAlternativeScopes(scope, scope, branches, "if", node.Line)

	case *ast.WhileStmt:
		if _, err := c.checkExpr(node.Condition, scope, deferred); err != nil {
			return err
		}
		loopScope := cloneScope(scope)
		if err := c.checkBlock(node.Body, loopScope, deferred); err != nil {
			return err
		}
		if value, known := staticBool(node.Condition); known {
			if value {
				c.applyClonedScope(scope, loopScope)
			}
			return nil
		}
		return c.mergeAlternativeScopes(scope, scope, []*Scope{loopScope, cloneScope(scope)}, "while", node.Line)

	case *ast.ForRangeStmt:
		if _, err := c.checkExpr(node.Start, scope, deferred); err != nil {
			return err
		}
		if _, err := c.checkExpr(node.End, scope, deferred); err != nil {
			return err
		}
		if node.Step != nil {
			if _, err := c.checkExpr(node.Step, scope, deferred); err != nil {
				return err
			}
		}
		loopScope := cloneScope(scope)
		loopScope.defineValue(node.Var, c.valueFromDeclaredType("int", loopScope, node.Var))
		if err := c.checkBlock(node.Body, loopScope, deferred); err != nil {
			return err
		}
		if rangeRunsAtLeastOnce(node) {
			c.applyClonedScope(scope, loopScope)
			return nil
		}
		return c.mergeAlternativeScopes(scope, scope, []*Scope{loopScope, cloneScope(scope)}, "for", node.Line)

	case *ast.ForEachStmt:
		iterable, err := c.checkExpr(node.Iterable, scope, deferred)
		if err != nil {
			return err
		}
		loopScope := cloneScope(scope)
		itemType := ""
		if params := genericTypeParams(c.valueTypeName(iterable, scope), "list"); len(params) == 1 {
			itemType = params[0]
		}
		loopScope.defineValue(node.Var, c.valueFromDeclaredType(itemType, loopScope, node.Var))
		if node.IndexVar != "" {
			loopScope.defineValue(node.IndexVar, c.valueFromDeclaredType("int", loopScope, node.IndexVar))
		}
		if err := c.checkBlock(node.Body, loopScope, deferred); err != nil {
			return err
		}
		if forEachRunsAtLeastOnce(node) {
			c.applyClonedScope(scope, loopScope)
			return nil
		}
		return c.mergeAlternativeScopes(scope, scope, []*Scope{loopScope, cloneScope(scope)}, "for", node.Line)

	case *ast.MatchStmt:
		subject, err := c.checkExpr(node.Subject, scope, deferred)
		if err != nil {
			return err
		}
		if subjectType := c.valueTypeName(subject, scope); subjectType != "" {
			if _, ok := parseResultTypeInfo(subjectType); ok {
				if err := c.validateResultMatchPatterns(node); err != nil {
					return err
				}
			}
		}
		branches := make([]*Scope, 0, len(node.Cases)+1)
		for _, clause := range node.Cases {
			caseScope := cloneScope(scope)
			for _, pattern := range clause.Patterns {
				if err := c.checkPattern(pattern, subject, caseScope, deferred); err != nil {
					return err
				}
			}
			if err := c.checkBlock(clause.Body, caseScope, deferred); err != nil {
				return err
			}
			branches = append(branches, caseScope)
		}
		if len(node.ElseBody) > 0 {
			elseScope := cloneScope(scope)
			if err := c.checkBlock(node.ElseBody, elseScope, deferred); err != nil {
				return err
			}
			branches = append(branches, elseScope)
		}
		return c.mergeAlternativeScopes(scope, scope, branches, "match", node.Line)

	case *ast.ModuleDef:
		if err := c.validateModuleBody(node); err != nil {
			return err
		}
		moduleScope := newScope(scope)
		if err := c.checkBlock(node.Body, moduleScope, deferred); err != nil {
			return err
		}
		module, err := c.collectModuleExports(node, moduleScope)
		if err != nil {
			return err
		}
		c.modules[node.Name] = module
		scope.defineValue(node.Name, &ValueInfo{Kind: "module", ModuleInfo: module})
		return nil

	case *ast.UseStmt:
		return c.checkUse(node, scope, deferred)

	case *ast.ParallelStmt:
		if err := c.checkBlock(node.Body, newScope(scope), deferred); err != nil {
			return err
		}
		if node.ResultVar != "" {
			scope.defineValue(node.ResultVar, c.valueFromDeclaredType("list", scope, node.ResultVar))
		}
		return nil

	case *ast.GlobalStmt:
		value, err := c.checkExpr(node.Value, scope, deferred)
		if err != nil {
			return err
		}
		target, ok := scope.resolveOuterValue(node.Name)
		if !ok {
			return semanticErrorf(node.Line, "global variable '%s' not found in any outer scope", node.Name)
		}
		if target.Kind == "builtin" {
			return semanticErrorf(node.Line, "Cannot assign to builtin '%s' with global", node.Name)
		}
		return c.validateValueAssignment(node.Name, target, value, scope, node.Line)

	case *ast.ArenaStmt:
		arenaScope := cloneScope(scope)
		if err := c.checkBlock(node.Body, arenaScope, deferred); err != nil {
			return err
		}
		c.applyClonedScope(scope, arenaScope)
		return nil

	case *ast.TagStmt:
		return nil

	case *ast.ObjectDef:
		return c.checkObjectDef(node, scope, deferred)

	case *ast.ExprStmt:
		_, err := c.checkExpr(node.Expr, scope, deferred)
		return err

	default:
		return semanticErrorf(lineOf(stmt), "Unknown statement type: %T", stmt)
	}
}

func (c *Checker) checkReturnValue(value any, scope *Scope, deferred *[]func() error, line int) error {
	var infos []*ValueInfo
	switch exprs := value.(type) {
	case nil:
		return c.validateReturnValues(nil, scope, line)
	case []any:
		for _, expr := range exprs {
			info, err := c.checkExpr(expr, scope, deferred)
			if err != nil {
				return err
			}
			infos = append(infos, info)
		}
		return c.validateReturnValues(infos, scope, line)
	default:
		info, err := c.checkExpr(value, scope, deferred)
		if err != nil {
			return err
		}
		infos = append(infos, info)
		return c.validateReturnValues(infos, scope, line)
	}
}

func (c *Checker) checkAssignmentTarget(target any, scope *Scope, deferred *[]func() error, line int) error {
	switch node := target.(type) {
	case string:
		return nil
	case *ast.MemberAccess:
		object, err := c.checkExpr(node.Object, scope, deferred)
		if err != nil {
			return err
		}
		_, err = c.checkMemberAccess(object, node, scope)
		return err
	case *ast.IndexAccess:
		if _, err := c.checkExpr(node.Object, scope, deferred); err != nil {
			return err
		}
		_, err := c.checkExpr(node.Index, scope, deferred)
		return err
	default:
		return semanticErrorf(line, "Invalid assignment target: %T", target)
	}
}

func (c *Checker) checkFuncBody(fn *ast.FuncDef, closureScope *Scope) error {
	if err := c.validateReturnAnnotation(fn.ReturnType, closureScope, fn.Line); err != nil {
		return err
	}
	bodyScope := newReturnScope(closureScope, c.resolveReturnTypeNames(fn.ReturnType, closureScope, fn.Line))
	for _, param := range fn.Params {
		if err := c.checkParam(param, closureScope); err != nil {
			return err
		}
		bodyScope.defineParamValue(c, param, closureScope)
	}

	deferred := []func() error{}
	if err := c.checkBlock(fn.Body, bodyScope, &deferred); err != nil {
		return err
	}
	return c.runDeferred(&deferred)
}

func (c *Checker) checkConstructorBody(ctor *ast.ConstructorDef, obj *ObjectInfo, closureScope *Scope) error {
	if ctor.ReturnType != nil {
		if _, err := c.resolveTypeNode(ctor.ReturnType, closureScope, ctor.Line); err != nil {
			return err
		}
	}
	bodyScope := newReturnScope(closureScope, []string{obj.Name})
	for _, param := range ctor.Params {
		if err := c.checkParam(param, closureScope); err != nil {
			return err
		}
		bodyScope.defineParamValue(c, param, closureScope)
	}

	deferred := []func() error{}
	if err := c.checkBlock(ctor.Body, bodyScope, &deferred); err != nil {
		return err
	}
	return c.runDeferred(&deferred)
}

func (c *Checker) checkMethodBody(method *ast.MethodDef, obj *ObjectInfo, closureScope *Scope) error {
	if err := c.validateMethodReceiver(method, obj); err != nil {
		return err
	}
	if err := c.validateReturnAnnotation(method.ReturnType, closureScope, method.Line); err != nil {
		return err
	}
	bodyScope := newMethodScope(closureScope, c.resolveReturnTypeNames(method.ReturnType, closureScope, method.Line), obj.Name)
	for _, param := range method.Params {
		if err := c.checkParam(param, closureScope); err != nil {
			return err
		}
		bodyScope.defineParamValue(c, param, closureScope)
	}

	deferred := []func() error{}
	if err := c.checkBlock(method.Body, bodyScope, &deferred); err != nil {
		return err
	}
	return c.runDeferred(&deferred)
}

func (c *Checker) checkParam(param *ast.Param, scope *Scope) error {
	if param.TypeName != nil {
		if _, err := c.resolveTypeNode(param.TypeName, scope, param.Line); err != nil {
			return err
		}
	}
	if param.Default != nil {
		deferred := []func() error{}
		if _, err := c.checkExpr(param.Default, scope, &deferred); err != nil {
			return err
		}
		return c.runDeferred(&deferred)
	}
	return nil
}

func (s *Scope) defineParamValue(c *Checker, param *ast.Param, typeScope *Scope) {
	typeName := ""
	if param.TypeName != nil {
		if resolved, err := c.resolveTypeNode(param.TypeName, typeScope, param.Line); err == nil {
			typeName = resolved
		} else if rendered, ok := renderType(param.TypeName); ok {
			typeName = rendered
		}
	}
	s.defineValue(param.Name, c.valueFromDeclaredType(typeName, typeScope, param.Name))
}

func (c *Checker) bindDeclaredValue(name string, declaredType string, inferred *ValueInfo, scope *Scope, line int) (*ValueInfo, error) {
	if declaredType != "" {
		value := c.valueFromDeclaredType(declaredType, scope, name)
		if inferred != nil {
			if err := c.validateValueAssignment(name, value, inferred, scope, line); err != nil {
				return nil, err
			}
		}
		return value, nil
	}
	if inferred != nil {
		return c.copyValue(inferred, scope), nil
	}
	return c.valueFromDeclaredType("", scope, name), nil
}

func (c *Checker) validateValueAssignment(name string, expected *ValueInfo, actual *ValueInfo, scope *Scope, line int) error {
	expectedType := c.valueTypeName(expected, scope)
	actualType := c.valueTypeName(actual, scope)
	if expectedType == "" || actualType == "" {
		return nil
	}
	if c.typesCompatible(expectedType, actualType) {
		return nil
	}
	return semanticErrorf(line, "Cannot assign %s to '%s' (%s)", displayTypeName(actualType), name, displayTypeName(expectedType))
}

func (c *Checker) validateReturnValues(values []*ValueInfo, scope *Scope, line int) error {
	expected := scope.expectedReturnTypes
	if expected == nil {
		return nil
	}
	if len(values) == 1 && len(values[0].MultiTypeNames) > 0 {
		expanded := make([]*ValueInfo, 0, len(values[0].MultiTypeNames))
		for _, typeName := range values[0].MultiTypeNames {
			expanded = append(expanded, c.valueFromDeclaredType(typeName, scope, "return"))
		}
		values = expanded
	}
	if len(expected) != len(values) {
		return semanticErrorf(line, "Return value count mismatch: expected %d, got %d", len(expected), len(values))
	}
	for idx, expectedType := range expected {
		actualType := c.valueTypeName(values[idx], scope)
		if expectedType == "" || actualType == "" {
			continue
		}
		if !c.typesCompatible(expectedType, actualType) {
			if len(expected) == 1 {
				return semanticErrorf(line, "Return type mismatch: expected %s, got %s", displayTypeName(expectedType), displayTypeName(actualType))
			}
			return semanticErrorf(line, "Return value %d expects %s, got %s", idx+1, displayTypeName(expectedType), displayTypeName(actualType))
		}
	}
	return nil
}

func (c *Checker) resolveReturnTypeNames(returnType any, scope *Scope, line int) []string {
	rendered := renderReturnTypes(returnType)
	if rendered == nil {
		return nil
	}
	resolved := make([]string, 0, len(rendered))
	for _, typeName := range rendered {
		if name, ok, err := c.resolveTypeNameString(typeName, scope, line, false); err == nil && ok {
			resolved = append(resolved, name)
			continue
		}
		resolved = append(resolved, typeName)
	}
	return resolved
}

func (c *Checker) typesCompatible(expected string, actual string) bool {
	if expected == actual {
		return true
	}
	expectedBase := typeBase(expected)
	actualBase := typeBase(actual)
	if isIntType(expected) && isIntType(actual) {
		return true
	}
	if isFloatType(expected) && (isIntType(actual) || isFloatType(actual)) {
		return true
	}
	if strings.HasPrefix(expected, "money[") {
		if strings.HasPrefix(actual, "money[") {
			return expected == actual
		}
		return isIntType(actual) || isFloatType(actual)
	}
	if expectedResult, ok := parseResultTypeInfo(expected); ok {
		actualResult, actualOK := parseResultTypeInfo(actual)
		if !actualOK {
			return false
		}
		return c.resultTypesCompatible(expectedResult, actualResult)
	}
	if _, ok := parseResultTypeInfo(actual); ok {
		return false
	}
	if expectedBase == actualBase && (expectedBase == "list" || expectedBase == "dict" || expectedBase == "result") {
		if expected == expectedBase || actual == actualBase {
			return true
		}
		return expected == actual
	}
	if strings.HasPrefix(expected, "(") && strings.Contains(expected, "->") {
		return expected == actual
	}
	return false
}

func (c *Checker) resultTypesCompatible(expected resultTypeInfo, actual resultTypeInfo) bool {
	if expected.bare || actual.bare {
		return true
	}

	if expected.okOnly {
		if actual.errOnly || actual.okType == "" {
			return false
		}
		return c.resultPayloadCompatible(expected.okType, actual.okType)
	}
	if expected.errOnly {
		if actual.okOnly || (!actual.errOnly && actual.okType != "") {
			return false
		}
		return c.resultErrTypesCompatible(expected.errTypes, actual.errTypes)
	}

	if actual.okOnly {
		return c.resultPayloadCompatible(expected.okType, actual.okType)
	}
	if actual.errOnly {
		return c.resultErrTypesCompatible(expected.errTypes, actual.errTypes)
	}
	if !c.resultPayloadCompatible(expected.okType, actual.okType) {
		return false
	}
	return c.resultErrTypesCompatible(expected.errTypes, actual.errTypes)
}

func (c *Checker) resultPayloadCompatible(expected string, actual string) bool {
	if expected == "" || actual == "" {
		return true
	}
	return c.typesCompatible(expected, actual)
}

func (c *Checker) resultErrTypesCompatible(expectedErrTypes []string, actualErrTypes []string) bool {
	if len(actualErrTypes) == 0 {
		return true
	}
	if len(expectedErrTypes) == 0 {
		return true
	}
	for _, actual := range actualErrTypes {
		matched := false
		for _, expected := range expectedErrTypes {
			if c.typesCompatible(expected, actual) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func isIntType(typeName string) bool {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}

func isFloatType(typeName string) bool {
	switch typeName {
	case "float", "float32", "float64":
		return true
	default:
		return false
	}
}

func typeBase(typeName string) string {
	if idx := strings.Index(typeName, "["); idx > 0 {
		return typeName[:idx]
	}
	return typeName
}

func genericTypeParams(typeName string, base string) []string {
	prefix := base + "["
	if !strings.HasPrefix(typeName, prefix) || !strings.HasSuffix(typeName, "]") {
		return nil
	}
	inner := strings.TrimSpace(typeName[len(prefix) : len(typeName)-1])
	if inner == "" {
		return nil
	}
	return splitTopLevel(inner, ',')
}

func parseResultTypeInfo(typeName string) (resultTypeInfo, bool) {
	switch typeName {
	case "result":
		return resultTypeInfo{bare: true}, true
	case resultOkBase:
		return resultTypeInfo{okOnly: true}, true
	case resultErrBase:
		return resultTypeInfo{errOnly: true}, true
	}

	if params := genericTypeParams(typeName, "result"); len(params) >= 1 {
		info := resultTypeInfo{okType: params[0]}
		if len(params) == 1 {
			info.errTypes = []string{"string"}
			info.implicitErrs = true
		} else {
			info.errTypes = append([]string{}, params[1:]...)
		}
		return info, true
	}
	if params := genericTypeParams(typeName, resultOkBase); len(params) >= 1 {
		return resultTypeInfo{okOnly: true, okType: params[0]}, true
	}
	if params := genericTypeParams(typeName, resultErrBase); len(params) >= 1 {
		return resultTypeInfo{errOnly: true, errTypes: append([]string{}, params...)}, true
	}
	return resultTypeInfo{}, false
}

func makeResultOkType(typeName string) string {
	if typeName == "" {
		return resultOkBase
	}
	return fmt.Sprintf("%s[%s]", resultOkBase, typeName)
}

func makeResultErrType(typeName string) string {
	if typeName == "" {
		return resultErrBase
	}
	return fmt.Sprintf("%s[%s]", resultErrBase, typeName)
}

func renderResultTypeInfo(info resultTypeInfo) string {
	switch {
	case info.bare:
		return "result"
	case info.okOnly:
		if info.okType == "" {
			return resultOkBase
		}
		return fmt.Sprintf("%s[%s]", resultOkBase, info.okType)
	case info.errOnly:
		if len(info.errTypes) == 0 {
			return resultErrBase
		}
		return fmt.Sprintf("%s[%s]", resultErrBase, strings.Join(info.errTypes, ", "))
	case info.okType == "":
		if len(info.errTypes) == 0 {
			return "result"
		}
		return renderResultTypeInfo(resultTypeInfo{errOnly: true, errTypes: info.errTypes})
	case info.implicitErrs && len(info.errTypes) == 1 && info.errTypes[0] == "string":
		return fmt.Sprintf("result[%s]", info.okType)
	default:
		params := append([]string{info.okType}, info.errTypes...)
		return fmt.Sprintf("result[%s]", strings.Join(params, ", "))
	}
}

func displayTypeName(typeName string) string {
	info, ok := parseResultTypeInfo(typeName)
	if !ok {
		return typeName
	}
	switch {
	case info.okOnly:
		if info.okType == "" {
			return "ok(?)"
		}
		return fmt.Sprintf("ok(%s)", info.okType)
	case info.errOnly:
		if len(info.errTypes) == 0 {
			return "err(?)"
		}
		return fmt.Sprintf("err(%s)", strings.Join(info.errTypes, " | "))
	default:
		return renderResultTypeInfo(info)
	}
}

func (c *Checker) checkObjectDef(objDef *ast.ObjectDef, scope *Scope, deferred *[]func() error) error {
	obj := &ObjectInfo{
		Name:    objDef.Name,
		Fields:  map[string]string{},
		Methods: map[string]*CallableInfo{},
	}
	scope.defineObject(obj)
	scope.defineValue(objDef.Name, &ValueInfo{
		Kind:       "object_type",
		TypeName:   objDef.Name,
		ObjectInfo: obj,
	})

	for _, field := range objDef.Fields {
		if _, exists := obj.Fields[field.Name]; exists {
			return semanticErrorf(field.Line, "Duplicate field '%s' in object '%s'", field.Name, objDef.Name)
		}
		fieldType, err := c.resolveTypeNode(field.TypeAnnotation, scope, field.Line)
		if err != nil {
			return err
		}
		obj.Fields[field.Name] = fieldType
	}

	if objDef.Constructor != nil {
		ctorReturn, ok := renderType(objDef.Constructor.ReturnType)
		if ok && ctorReturn != objDef.Name {
			return semanticErrorf(objDef.Constructor.Line, "Constructor '%s.new' must return '%s'", objDef.Name, objDef.Name)
		}
		for _, param := range objDef.Constructor.Params {
			if err := c.checkParam(param, scope); err != nil {
				return err
			}
		}
		signature, err := c.callableFromParams(objDef.Name+".new", objDef.Constructor.Params, objDef.Constructor.ReturnType, scope)
		if err != nil {
			return err
		}
		signature.ReturnTypeNames = []string{objDef.Name}
		obj.HasConstructor = true
		obj.Constructor = signature
		ctor := objDef.Constructor
		*deferred = append(*deferred, func() error {
			return c.checkConstructorBody(ctor, obj, scope)
		})
	}

	for _, method := range objDef.Methods {
		if _, exists := obj.Methods[method.Name]; exists {
			return semanticErrorf(method.Line, "Duplicate method '%s' in object '%s'", method.Name, objDef.Name)
		}
		if err := c.validateMethodReceiver(method, obj); err != nil {
			return err
		}
		for _, param := range method.Params {
			if err := c.checkParam(param, scope); err != nil {
				return err
			}
		}
		if err := c.validateReturnAnnotation(method.ReturnType, scope, method.Line); err != nil {
			return err
		}
		signature, err := c.callableFromParams(objDef.Name+"."+method.Name, method.Params, method.ReturnType, scope)
		if err != nil {
			return err
		}
		obj.Methods[method.Name] = signature
		method := method
		*deferred = append(*deferred, func() error {
			return c.checkMethodBody(method, obj, scope)
		})
	}
	return nil
}

func (c *Checker) validateMethodReceiver(method *ast.MethodDef, obj *ObjectInfo) error {
	if len(method.Params) == 0 {
		return semanticErrorf(method.Line, "Method '%s.%s' must declare 'self' as first parameter", obj.Name, method.Name)
	}
	first := method.Params[0]
	if first.Name != "self" {
		return semanticErrorf(first.Line, "Method '%s.%s' must declare 'self' as first parameter", obj.Name, method.Name)
	}
	selfType, ok := renderType(first.TypeName)
	if !ok || selfType != obj.Name {
		return semanticErrorf(first.Line, "Method '%s.%s' must declare 'self: %s'", obj.Name, method.Name, obj.Name)
	}
	return nil
}

func (c *Checker) validateReturnAnnotation(returnType any, scope *Scope, line int) error {
	switch node := returnType.(type) {
	case nil:
		return nil
	case []any:
		for _, item := range node {
			if _, err := c.resolveTypeNode(item, scope, line); err != nil {
				return err
			}
		}
		return nil
	default:
		_, err := c.resolveTypeNode(node, scope, line)
		return err
	}
}

func (c *Checker) checkExpr(expr any, scope *Scope, deferred *[]func() error) (*ValueInfo, error) {
	switch node := expr.(type) {
	case nil:
		return unknownValue(), nil
	case *ast.IntLiteral:
		return literalValue("int"), nil
	case *ast.FloatLiteral:
		return literalValue("float"), nil
	case *ast.StringLiteral:
		return literalValue("string"), nil
	case *ast.BoolLiteral:
		return literalValue("bool"), nil
	case *ast.Identifier:
		if value, ok := scope.resolveValue(node.Name); ok {
			return value, nil
		}
		return nil, semanticErrorf(node.Line, "Undefined variable: %s", node.Name)
	case *ast.BinaryOp:
		left, err := c.checkExpr(node.Left, scope, deferred)
		if err != nil {
			return nil, err
		}
		right, err := c.checkExpr(node.Right, scope, deferred)
		if err != nil {
			return nil, err
		}
		return c.inferBinaryOp(node, left, right, scope), nil
	case *ast.UnaryOp:
		if _, err := c.checkExpr(node.Operand, scope, deferred); err != nil {
			return nil, err
		}
		if node.Op == "not" {
			return literalValue("bool"), nil
		}
		return unknownValue(), nil
	case *ast.FuncCall:
		callee, err := c.checkExpr(node.Name, scope, deferred)
		if err != nil {
			return nil, err
		}
		args := make([]*ValueInfo, 0, len(node.Args))
		for _, arg := range node.Args {
			argInfo, err := c.checkExpr(arg, scope, deferred)
			if err != nil {
				return nil, err
			}
			args = append(args, argInfo)
		}
		if err := c.validateCall(callee, args, scope, node.Line); err != nil {
			return nil, err
		}
		if inferred := c.inferBuiltinReturn(callee, args, scope); inferred != nil {
			return inferred, nil
		}
		if callee.Kind == "constructor" {
			return &ValueInfo{
				Kind:       "variable",
				TypeName:   callee.TypeName,
				ObjectInfo: callee.ObjectInfo,
			}, nil
		}
		if callee.CallableInfo != nil && len(callee.CallableInfo.ReturnTypeNames) > 0 {
			if len(callee.CallableInfo.ReturnTypeNames) > 1 {
				return &ValueInfo{Kind: "multi", MultiTypeNames: callee.CallableInfo.ReturnTypeNames}, nil
			}
			return c.valueFromDeclaredType(callee.CallableInfo.ReturnTypeNames[0], scope, callee.CallableInfo.Label), nil
		}
		return unknownValue(), nil
	case *ast.MemberAccess:
		object, err := c.checkExpr(node.Object, scope, deferred)
		if err != nil {
			return nil, err
		}
		return c.checkMemberAccess(object, node, scope)
	case *ast.IndexAccess:
		object, err := c.checkExpr(node.Object, scope, deferred)
		if err != nil {
			return nil, err
		}
		if _, err := c.checkExpr(node.Index, scope, deferred); err != nil {
			return nil, err
		}
		if params := genericTypeParams(c.valueTypeName(object, scope), "list"); len(params) == 1 {
			return c.valueFromDeclaredType(params[0], scope, "index"), nil
		}
		if params := genericTypeParams(c.valueTypeName(object, scope), "dict"); len(params) == 2 {
			return c.valueFromDeclaredType(params[1], scope, "index"), nil
		}
		return unknownValue(), nil
	case *ast.Lambda:
		lambdaScope := newScope(scope)
		for _, param := range node.Params {
			if err := c.checkParam(param, scope); err != nil {
				return nil, err
			}
			lambdaScope.defineParamValue(c, param, scope)
		}
		returnType := c.inferLambdaReturnType(node, lambdaScope, deferred)
		callable, err := c.callableFromParams("<lambda>", node.Params, returnType, scope)
		if err != nil {
			return nil, err
		}
		if err := c.checkBlock(node.Body, lambdaScope, deferred); err != nil {
			return nil, err
		}
		return &ValueInfo{Kind: "lambda", TypeName: callable.typeName(), CallableInfo: callable}, nil
	case *ast.OkExpr:
		value, err := c.checkExpr(node.Value, scope, deferred)
		if err != nil {
			return nil, err
		}
		typeName := c.valueTypeName(value, scope)
		return literalValue(makeResultOkType(typeName)), nil
	case *ast.ErrExpr:
		value, err := c.checkExpr(node.Value, scope, deferred)
		if err != nil {
			return nil, err
		}
		return literalValue(makeResultErrType(c.valueTypeName(value, scope))), nil
	case *ast.ListLiteral:
		elementType := ""
		homogeneous := true
		for _, element := range node.Elements {
			info, err := c.checkExpr(element, scope, deferred)
			if err != nil {
				return nil, err
			}
			current := c.valueTypeName(info, scope)
			if current == "" {
				homogeneous = false
				continue
			}
			if elementType == "" {
				elementType = current
				continue
			}
			if elementType != current {
				homogeneous = false
			}
		}
		if homogeneous && elementType != "" {
			return literalValue(fmt.Sprintf("list[%s]", elementType)), nil
		}
		return literalValue("list"), nil
	case *ast.DictLiteral:
		keyType, err := c.resolveTypeNode(node.KeyType, scope, node.Line)
		if err != nil {
			return nil, err
		}
		valueType, err := c.resolveTypeNode(node.ValueType, scope, node.Line)
		if err != nil {
			return nil, err
		}
		for _, entry := range node.Entries {
			keyInfo, err := c.checkExpr(entry.Key, scope, deferred)
			if err != nil {
				return nil, err
			}
			valueInfo, err := c.checkExpr(entry.Value, scope, deferred)
			if err != nil {
				return nil, err
			}
			actualKey := c.valueTypeName(keyInfo, scope)
			if actualKey != "" && !c.typesCompatible(keyType, actualKey) {
				return nil, semanticErrorf(node.Line, "Dict key expects %s, got %s", displayTypeName(keyType), displayTypeName(actualKey))
			}
			actualValue := c.valueTypeName(valueInfo, scope)
			if actualValue != "" && !c.typesCompatible(valueType, actualValue) {
				return nil, semanticErrorf(node.Line, "Dict value expects %s, got %s", displayTypeName(valueType), displayTypeName(actualValue))
			}
		}
		return literalValue(fmt.Sprintf("dict[%s, %s]", keyType, valueType)), nil
	case *ast.AsExpr:
		if _, err := c.checkExpr(node.Expr, scope, deferred); err != nil {
			return nil, err
		}
		resolvedType, _, err := c.resolveTypeNameString(node.TypeName, scope, node.Line, true)
		if err != nil {
			return nil, err
		}
		return literalValue(fmt.Sprintf("result[%s]", resolvedType)), nil
	case *ast.ObjectLiteral:
		if err := c.checkObjectLiteral(node, scope, deferred); err != nil {
			return nil, err
		}
		obj, _ := scope.resolveObject(node.Name)
		return &ValueInfo{Kind: "variable", TypeName: node.Name, ObjectInfo: obj}, nil
	default:
		return nil, semanticErrorf(lineOf(expr), "Unknown expression type: %T", expr)
	}
}

func (c *Checker) inferBinaryOp(node *ast.BinaryOp, left *ValueInfo, right *ValueInfo, scope *Scope) *ValueInfo {
	if node.Op == "=" || node.Op == "!=" || node.Op == "<" || node.Op == ">" || node.Op == "<=" || node.Op == ">=" || node.Op == "and" || node.Op == "or" || node.Op == "to" {
		return literalValue("bool")
	}
	leftType := c.valueTypeName(left, scope)
	rightType := c.valueTypeName(right, scope)
	if leftType == "" || rightType == "" {
		return unknownValue()
	}
	if node.Op == "+" && leftType == "string" && rightType == "string" {
		return literalValue("string")
	}
	if node.Op == "+" && strings.HasPrefix(leftType, "list") && strings.HasPrefix(rightType, "list") {
		return literalValue(leftType)
	}
	if strings.HasPrefix(leftType, "money[") && strings.HasPrefix(rightType, "money[") && (node.Op == "+" || node.Op == "-") {
		return literalValue(leftType)
	}
	if strings.HasPrefix(leftType, "money[") && (isIntType(rightType) || isFloatType(rightType)) {
		return literalValue(leftType)
	}
	if isIntType(leftType) && isIntType(rightType) {
		return literalValue("int")
	}
	if (isIntType(leftType) || isFloatType(leftType)) && (isIntType(rightType) || isFloatType(rightType)) {
		return literalValue("float")
	}
	return unknownValue()
}

func (c *Checker) checkPattern(pattern any, subject *ValueInfo, scope *Scope, deferred *[]func() error) error {
	switch node := pattern.(type) {
	case *ast.OkExpr:
		expectedType := c.resultPatternValueType(subject, scope, true)
		if err := c.checkResultPatternValue(node.Value, expectedType, scope, deferred, "ok", node.Line); err != nil {
			return err
		}
		return nil
	case *ast.ErrExpr:
		expectedType := c.resultPatternValueType(subject, scope, false)
		if err := c.checkResultPatternValue(node.Value, expectedType, scope, deferred, "err", node.Line); err != nil {
			return err
		}
		return nil
	case *ast.Identifier:
		scope.defineValue(node.Name, unknownValue())
		return nil
	case *ast.BinaryOp:
		if _, err := c.checkExpr(node.Left, scope, deferred); err != nil {
			return err
		}
		_, err := c.checkExpr(node.Right, scope, deferred)
		return err
	default:
		_, err := c.checkExpr(pattern, scope, deferred)
		return err
	}
}

func (c *Checker) resultPatternValueType(subject *ValueInfo, scope *Scope, okPattern bool) string {
	subjectType := c.valueTypeName(subject, scope)
	info, ok := parseResultTypeInfo(subjectType)
	if !ok {
		return ""
	}
	if okPattern {
		return info.okType
	}
	if len(info.errTypes) == 0 {
		return ""
	}
	merged := info.errTypes[0]
	for _, current := range info.errTypes[1:] {
		next, ok := c.mergeTypeNames(merged, current)
		if !ok {
			return ""
		}
		merged = next
	}
	return merged
}

func (c *Checker) checkResultPatternValue(pattern any, expectedType string, scope *Scope, deferred *[]func() error, label string, line int) error {
	if identifier, ok := pattern.(*ast.Identifier); ok {
		c.bindPatternValue(identifier, scope, expectedType)
		return nil
	}
	value, err := c.checkExpr(pattern, scope, deferred)
	if err != nil {
		return err
	}
	actualType := c.valueTypeName(value, scope)
	if expectedType != "" && actualType != "" && !c.typesCompatible(expectedType, actualType) {
		return semanticErrorf(line, "%s pattern expects %s, got %s", label, displayTypeName(expectedType), displayTypeName(actualType))
	}
	return nil
}

func (c *Checker) validateResultMatchPatterns(stmt *ast.MatchStmt) error {
	for _, clause := range stmt.Cases {
		for _, pattern := range clause.Patterns {
			if isResultMatchPattern(pattern) {
				continue
			}
			return semanticErrorf(lineOf(pattern), "Match on Result type must use ok(x) or err(x) patterns, not %s", describeMatchPattern(pattern))
		}
	}
	return nil
}

func isResultMatchPattern(pattern any) bool {
	switch pattern.(type) {
	case *ast.OkExpr, *ast.ErrExpr:
		return true
	default:
		return false
	}
}

func describeMatchPattern(pattern any) string {
	switch pattern.(type) {
	case *ast.IntLiteral:
		return "int literal"
	case *ast.FloatLiteral:
		return "float literal"
	case *ast.StringLiteral:
		return "string literal"
	case *ast.BoolLiteral:
		return "bool literal"
	case *ast.Identifier:
		return "identifier pattern"
	case *ast.BinaryOp:
		return "range pattern"
	default:
		return "this pattern"
	}
}

func (c *Checker) bindPatternValue(pattern any, scope *Scope, typeName string) {
	switch node := pattern.(type) {
	case *ast.Identifier:
		if typeName == "" {
			scope.defineValue(node.Name, unknownValue())
			return
		}
		scope.defineValue(node.Name, c.valueFromDeclaredType(typeName, scope, node.Name))
	}
}

func (c *Checker) inferLambdaReturnType(lambda *ast.Lambda, scope *Scope, deferred *[]func() error) any {
	if len(lambda.Body) != 1 {
		return nil
	}
	ret, ok := lambda.Body[0].(*ast.ReturnStmt)
	if !ok || ret.Value == nil {
		return nil
	}
	if _, ok := ret.Value.([]any); ok {
		return nil
	}
	info, err := c.checkExpr(ret.Value, scope, deferred)
	if err != nil {
		return nil
	}
	typeName := c.valueTypeName(info, scope)
	if typeName == "" {
		return nil
	}
	return &ast.TypeName{Name: typeName, Line: lambda.Line}
}

func (c *Checker) validateCall(callee *ValueInfo, args []*ValueInfo, scope *Scope, line int) error {
	if callee == nil || callee.CallableInfo == nil {
		return nil
	}
	signature := callee.CallableInfo
	if !signature.Variadic {
		required := 0
		for _, param := range signature.Params {
			if !param.HasDefault {
				required++
			}
		}
		if len(args) < required {
			for _, param := range signature.Params[len(args):] {
				if !param.HasDefault {
					return semanticErrorf(line, "Missing argument: %s", param.Name)
				}
			}
		}
		if len(args) > len(signature.Params) {
			return semanticErrorf(line, "Too many arguments for '%s': expected at most %d, got %d", signature.Label, len(signature.Params), len(args))
		}
	}
	limit := len(args)
	if limit > len(signature.Params) {
		limit = len(signature.Params)
	}
	for idx := 0; idx < limit; idx++ {
		param := signature.Params[idx]
		if param.TypeName == "" {
			continue
		}
		expected := c.resolveCallableTypeName(param.TypeName, signature, scope)
		actual := c.valueTypeName(args[idx], scope)
		if expected != "" && actual != "" && !c.typesCompatible(expected, actual) {
			return semanticErrorf(line, "Argument '%s' to '%s' expects %s, got %s", param.Name, signature.Label, displayTypeName(expected), displayTypeName(actual))
		}
	}
	return c.validateBuiltinContainerTypes(signature, args, scope, line)
}

func (c *Checker) resolveCallableTypeName(typeName string, signature *CallableInfo, fallbackScope *Scope) string {
	typeScope := fallbackScope
	if signature != nil && signature.DefinitionScope != nil {
		typeScope = signature.DefinitionScope
	}
	if resolved, ok, err := c.resolveTypeNameString(typeName, typeScope, 0, false); err == nil && ok {
		return resolved
	}
	return typeName
}

func (c *Checker) validateBuiltinContainerTypes(signature *CallableInfo, args []*ValueInfo, scope *Scope, line int) error {
	switch signature.Label {
	case "append":
		if len(args) >= 2 {
			return c.validateListItemArg(signature, args[0], args[1], "item", scope, line)
		}
	case "insert":
		if len(args) >= 3 {
			return c.validateListItemArg(signature, args[0], args[2], "item", scope, line)
		}
	case "get":
		if len(args) >= 3 {
			params := genericTypeParams(c.valueTypeName(args[0], scope), "dict")
			if len(params) == 2 {
				keyType := c.valueTypeName(args[1], scope)
				if keyType != "" && !c.typesCompatible(params[0], keyType) {
					return semanticErrorf(line, "Argument 'key' to 'get' expects %s, got %s", displayTypeName(params[0]), displayTypeName(keyType))
				}
				defaultType := c.valueTypeName(args[2], scope)
				if defaultType != "" && !c.typesCompatible(params[1], defaultType) {
					return semanticErrorf(line, "Argument 'default' to 'get' expects %s, got %s", displayTypeName(params[1]), displayTypeName(defaultType))
				}
			}
		}
	case "map":
		if len(args) >= 2 {
			params := genericTypeParams(c.valueTypeName(args[0], scope), "list")
			callback := args[1]
			if len(params) == 1 && callback.CallableInfo != nil && len(callback.CallableInfo.Params) > 0 {
				callbackParam := c.resolveCallableTypeName(callback.CallableInfo.Params[0].TypeName, callback.CallableInfo, scope)
				if callbackParam != "" && !c.typesCompatible(callbackParam, params[0]) {
					return semanticErrorf(line, "Argument 'f' to 'map' expects (%s) -> ..., got %s", displayTypeName(params[0]), displayTypeName(c.valueTypeName(callback, scope)))
				}
			}
		}
	case "filter":
		if len(args) >= 2 {
			params := genericTypeParams(c.valueTypeName(args[0], scope), "list")
			callback := args[1]
			if len(params) == 1 && callback.CallableInfo != nil && len(callback.CallableInfo.Params) > 0 {
				callbackParam := c.resolveCallableTypeName(callback.CallableInfo.Params[0].TypeName, callback.CallableInfo, scope)
				if callbackParam != "" && !c.typesCompatible(callbackParam, params[0]) {
					return semanticErrorf(line, "Argument 'pred' to 'filter' expects (%s) -> bool, got %s", displayTypeName(params[0]), displayTypeName(c.valueTypeName(callback, scope)))
				}
				if len(callback.CallableInfo.ReturnTypeNames) == 1 {
					callbackReturn := c.resolveCallableTypeName(callback.CallableInfo.ReturnTypeNames[0], callback.CallableInfo, scope)
					if callbackReturn != "" && callbackReturn != "bool" {
						return semanticErrorf(line, "Argument 'pred' to 'filter' must return bool, got %s", displayTypeName(callbackReturn))
					}
				}
			}
		}
	}
	return nil
}

func (c *Checker) validateListItemArg(signature *CallableInfo, listArg *ValueInfo, itemArg *ValueInfo, paramName string, scope *Scope, line int) error {
	params := genericTypeParams(c.valueTypeName(listArg, scope), "list")
	if len(params) != 1 {
		return nil
	}
	itemType := c.valueTypeName(itemArg, scope)
	if itemType != "" && !c.typesCompatible(params[0], itemType) {
		return semanticErrorf(line, "Argument '%s' to '%s' expects %s, got %s", paramName, signature.Label, displayTypeName(params[0]), displayTypeName(itemType))
	}
	return nil
}

func (c *Checker) inferBuiltinReturn(callee *ValueInfo, args []*ValueInfo, scope *Scope) *ValueInfo {
	if callee == nil || callee.CallableInfo == nil {
		return nil
	}
	switch callee.CallableInfo.Label {
	case "map":
		if len(args) >= 2 && args[1].CallableInfo != nil && len(args[1].CallableInfo.ReturnTypeNames) == 1 {
			return literalValue(fmt.Sprintf("list[%s]", args[1].CallableInfo.ReturnTypeNames[0]))
		}
		return literalValue("list")
	case "filter":
		if len(args) >= 1 {
			listType := c.valueTypeName(args[0], scope)
			if listType != "" {
				return literalValue(listType)
			}
		}
		return literalValue("list")
	case "concat":
		if len(args) >= 1 {
			listType := c.valueTypeName(args[0], scope)
			if listType != "" {
				return literalValue(listType)
			}
		}
	case "get":
		if len(args) >= 1 {
			params := genericTypeParams(c.valueTypeName(args[0], scope), "dict")
			if len(params) == 2 {
				return c.valueFromDeclaredType(params[1], scope, "get")
			}
		}
	}
	return nil
}

func (c *Checker) checkMemberAccess(object *ValueInfo, expr *ast.MemberAccess, scope *Scope) (*ValueInfo, error) {
	if object == nil {
		return unknownValue(), nil
	}
	if object.Kind == "module" && object.ModuleInfo != nil {
		if value, ok := object.ModuleInfo.RuntimeExports[expr.Member]; ok {
			return value, nil
		}
		if _, ok := object.ModuleInfo.TypeExports[expr.Member]; ok {
			return nil, semanticErrorf(expr.Line, "Type alias '%s' is not a runtime member of module '%s'; use 'use %s from %s' instead", expr.Member, memberBaseName(expr.Object), expr.Member, memberBaseName(expr.Object))
		}
		return nil, semanticErrorf(expr.Line, "Undefined variable: %s", expr.Member)
	}
	if object.Kind == "object_type" && object.ObjectInfo != nil {
		if expr.Member == "new" {
			if !object.ObjectInfo.HasConstructor {
				return nil, semanticErrorf(expr.Line, "Object '%s' has no constructor", object.ObjectInfo.Name)
			}
			return &ValueInfo{
				Kind:         "constructor",
				TypeName:     object.ObjectInfo.Name,
				ObjectInfo:   object.ObjectInfo,
				CallableInfo: object.ObjectInfo.Constructor,
			}, nil
		}
		if method, ok := object.ObjectInfo.Methods[expr.Member]; ok {
			return &ValueInfo{Kind: "method", TypeName: method.typeName(), CallableInfo: method}, nil
		}
		return nil, semanticErrorf(expr.Line, "Object '%s' has no member '%s'", object.ObjectInfo.Name, expr.Member)
	}
	if object.ObjectInfo != nil {
		if method, ok := object.ObjectInfo.Methods[expr.Member]; ok {
			return &ValueInfo{Kind: "method", TypeName: method.dropFirstParam().typeName(), CallableInfo: method.dropFirstParam()}, nil
		}
		if fieldType, ok := object.ObjectInfo.Fields[expr.Member]; ok {
			if ident, ok := expr.Object.(*ast.Identifier); ok && ident.Name == "self" && scope.methodSelfType == object.ObjectInfo.Name {
				return c.valueFromDeclaredType(fieldType, scope, expr.Member), nil
			}
			return nil, semanticErrorf(expr.Line, "Cannot access private field '%s' of '%s' from outside; use a method instead", expr.Member, object.ObjectInfo.Name)
		}
		return nil, semanticErrorf(expr.Line, "Object '%s' has no member '%s'", object.ObjectInfo.Name, expr.Member)
	}
	return unknownValue(), nil
}

func (ci *CallableInfo) dropFirstParam() *CallableInfo {
	if ci == nil {
		return nil
	}
	copied := *ci
	if len(copied.Params) > 0 {
		copied.Params = append([]ParamInfo{}, copied.Params[1:]...)
	}
	return &copied
}

func memberBaseName(expr any) string {
	if ident, ok := expr.(*ast.Identifier); ok {
		return ident.Name
	}
	return "module"
}

func (c *Checker) checkObjectLiteral(literal *ast.ObjectLiteral, scope *Scope, deferred *[]func() error) error {
	obj, ok := scope.resolveObject(literal.Name)
	if !ok {
		return semanticErrorf(literal.Line, "Unknown object type: %s", literal.Name)
	}

	provided := map[string]struct{}{}
	for _, field := range literal.Fields {
		if _, exists := provided[field.Name]; exists {
			return semanticErrorf(literal.Line, "Duplicate field '%s' in '%s' literal", field.Name, literal.Name)
		}
		provided[field.Name] = struct{}{}
		expectedType := ""
		if obj.Fields != nil {
			var exists bool
			expectedType, exists = obj.Fields[field.Name]
			if !exists {
				return semanticErrorf(literal.Line, "Object '%s' has no field '%s'", literal.Name, field.Name)
			}
		}
		value, err := c.checkExpr(field.Value, scope, deferred)
		if err != nil {
			return err
		}
		actualType := c.valueTypeName(value, scope)
		if expectedType != "" && actualType != "" && !c.typesCompatible(expectedType, actualType) {
			return semanticErrorf(literal.Line, "Field '%s.%s' expects %s, got %s", literal.Name, field.Name, displayTypeName(expectedType), displayTypeName(actualType))
		}
	}

	if obj.Fields != nil {
		var missing []string
		for name := range obj.Fields {
			if _, exists := provided[name]; !exists {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return semanticErrorf(literal.Line, "Object '%s' literal missing fields: %s", literal.Name, strings.Join(missing, ", "))
		}
	}
	return nil
}

func (c *Checker) validateModuleBody(module *ast.ModuleDef) error {
	seenDeclaration := false
	for _, stmt := range module.Body {
		switch stmt.(type) {
		case *ast.UseStmt:
			if seenDeclaration {
				return semanticErrorf(
					lineOf(stmt),
					"Module '%s' must place use statements before func/object/type declarations",
					module.Name,
				)
			}
		case *ast.FuncDef, *ast.ObjectDef, *ast.TypeAlias:
			seenDeclaration = true
		default:
			return semanticErrorf(
				lineOf(stmt),
				"Module '%s' top level only allows use/func/object/type declarations, got %s",
				module.Name,
				moduleStmtLabel(stmt),
			)
		}
	}
	return nil
}

func (c *Checker) collectModuleExports(moduleDef *ast.ModuleDef, moduleScope *Scope) (*ModuleInfo, error) {
	module := newModuleInfo(moduleDef.Name)
	for _, stmt := range moduleDef.Body {
		switch node := stmt.(type) {
		case *ast.FuncDef:
			if !node.Exported {
				continue
			}
			value, ok := moduleScope.resolveLocalValue(node.Name)
			if !ok {
				value = unknownValue()
			}
			if err := addRuntimeExport(module, moduleDef.Name, node.Name, value, node.Line); err != nil {
				return nil, err
			}
		case *ast.ObjectDef:
			if !node.Exported {
				continue
			}
			value, ok := moduleScope.resolveLocalValue(node.Name)
			if !ok {
				obj, _ := moduleScope.resolveObject(node.Name)
				value = &ValueInfo{Kind: "object_type", TypeName: node.Name, ObjectInfo: obj}
			}
			if err := addRuntimeExport(module, moduleDef.Name, node.Name, value, node.Line); err != nil {
				return nil, err
			}
		case *ast.TypeAlias:
			if !node.Exported {
				continue
			}
			target, err := c.resolveAliasName(node.Name, moduleScope, node.Line)
			if err != nil {
				return nil, err
			}
			if _, exists := module.TypeExports[node.Name]; exists {
				return nil, semanticErrorf(node.Line, "Module '%s' exports type name '%s' more than once", moduleDef.Name, node.Name)
			}
			module.TypeExports[node.Name] = target
		}
	}
	return module, nil
}

func addRuntimeExport(module *ModuleInfo, moduleName, exportName string, value *ValueInfo, line int) error {
	if _, exists := module.RuntimeExports[exportName]; exists {
		return semanticErrorf(line, "Module '%s' exports runtime name '%s' more than once", moduleName, exportName)
	}
	module.RuntimeExports[exportName] = value
	return nil
}

func (c *Checker) checkUse(use *ast.UseStmt, scope *Scope, deferred *[]func() error) error {
	module, ok := c.modules[use.Module]
	if !ok {
		if err := c.loadModuleFromFile(use.Module, use.Line, deferred); err != nil {
			return err
		}
		module, ok = c.modules[use.Module]
		if !ok {
			return semanticErrorf(use.Line, "Module not found: %s", use.Module)
		}
	}

	if len(use.Names) == 0 {
		if existing, exists := scope.resolveLocalValue(use.Module); exists && (existing.Kind != "module" || existing.ModuleInfo != module) {
			return semanticErrorf(use.Line, "Cannot import module '%s': name already defined in current scope", use.Module)
		}
		scope.defineValue(use.Module, &ValueInfo{Kind: "module", ModuleInfo: module})
		return nil
	}

	for _, name := range use.Names {
		imported := false
		if value, ok := module.RuntimeExports[name]; ok {
			if existing, exists := scope.resolveLocalValue(name); exists && existing != value {
				return semanticErrorf(use.Line, "Cannot import '%s' from module '%s': name already defined in current scope", name, use.Module)
			}
			scope.defineValue(name, value)
			if value.ObjectInfo != nil {
				scope.defineObject(value.ObjectInfo)
			}
			imported = true
		}
		if target, ok := module.TypeExports[name]; ok {
			if existing, exists := scope.resolveLocalAlias(name); exists && existing != target {
				return semanticErrorf(use.Line, "Cannot import type '%s' from module '%s': type name already defined in current scope", name, use.Module)
			}
			scope.defineAlias(name, target)
			imported = true
		}
		if !imported {
			return semanticErrorf(use.Line, "Module '%s' does not export '%s'", use.Module, name)
		}
	}
	return nil
}

func (c *Checker) loadModuleFromFile(moduleName string, line int, deferred *[]func() error) error {
	if _, ok := c.modules[moduleName]; ok {
		return nil
	}
	if _, ok := c.loadingModules[moduleName]; ok {
		return semanticErrorf(line, "Cyclic module import detected while loading '%s'", moduleName)
	}

	for _, candidate := range c.moduleCandidatePaths(moduleName) {
		source, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		program, err := parser.Parse(string(source))
		if err != nil {
			return err
		}
		if len(program.Statements) != 1 {
			return semanticErrorf(line, "Module file '%s' must contain exactly one top-level module definition for '%s'", candidate, moduleName)
		}
		moduleDef, ok := program.Statements[0].(*ast.ModuleDef)
		if !ok || moduleDef.Name != moduleName {
			return semanticErrorf(line, "Module file '%s' must contain exactly one top-level module definition for '%s'", candidate, moduleName)
		}

		c.loadingModules[moduleName] = struct{}{}
		defer delete(c.loadingModules, moduleName)
		c.AddModuleSearchPath(filepath.Dir(candidate))
		return c.checkStmt(moduleDef, c.globalScope, deferred)
	}

	return semanticErrorf(line, "Module not found: %s", moduleName)
}

func (c *Checker) moduleCandidatePaths(moduleName string) []string {
	searchPaths := c.moduleSearchPaths
	if len(searchPaths) == 0 {
		searchPaths = []string{"."}
	}

	seen := map[string]struct{}{}
	var candidates []string
	for _, base := range searchPaths {
		for _, candidate := range []string{
			filepath.Join(base, moduleName+".gw"),
			filepath.Join(base, moduleName, "main.gw"),
		} {
			absCandidate, err := filepath.Abs(candidate)
			if err != nil {
				absCandidate = candidate
			}
			if _, exists := seen[absCandidate]; exists {
				continue
			}
			seen[absCandidate] = struct{}{}
			candidates = append(candidates, absCandidate)
		}
	}
	return candidates
}

func (c *Checker) resolveAliasName(aliasName string, scope *Scope, line int) (string, error) {
	resolved, ok, err := c.resolveTypeNameString(aliasName, scope, line, true)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", semanticErrorf(line, "Unknown type: %s", aliasName)
	}
	return resolved, nil
}

func (c *Checker) resolveTypeNode(typeNode any, scope *Scope, line int) (string, error) {
	switch node := typeNode.(type) {
	case *ast.TypeName:
		resolved, ok, err := c.resolveTypeNameString(node.Name, scope, node.Line, true)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", semanticErrorf(node.Line, "Unknown type: %s", node.Name)
		}
		return resolved, nil

	case *ast.GenericType:
		expected, ok := genericBaseArity[node.Base]
		if !ok {
			return "", semanticErrorf(node.Line, "Unknown generic type: %s", node.Base)
		}
		count := len(node.Params)
		if count < expected.min || (expected.max >= 0 && count > expected.max) {
			if expected.max < 0 {
				return "", semanticErrorf(node.Line, "Generic type '%s' expects at least %d parameter(s)", node.Base, expected.min)
			}
			return "", semanticErrorf(node.Line, "Generic type '%s' expects %d parameter(s)", node.Base, expected.min)
		}
		if node.Base == "money" {
			rendered, _ := renderType(node)
			return rendered, nil
		}
		resolvedParams := make([]string, 0, len(node.Params))
		for _, param := range node.Params {
			resolved, err := c.resolveTypeNode(param, scope, node.Line)
			if err != nil {
				return "", err
			}
			resolvedParams = append(resolvedParams, resolved)
		}
		return fmt.Sprintf("%s[%s]", node.Base, strings.Join(resolvedParams, ", ")), nil

	case *ast.FuncType:
		for _, param := range node.ParamTypes {
			if _, err := c.resolveTypeNode(param, scope, node.Line); err != nil {
				return "", err
			}
		}
		if node.ReturnType != nil {
			if _, err := c.resolveTypeNode(node.ReturnType, scope, node.Line); err != nil {
				return "", err
			}
		}
		rendered, _ := renderType(node)
		return rendered, nil

	default:
		return "", semanticErrorf(line, "Unknown type node: %T", typeNode)
	}
}

func (c *Checker) resolveTypeNameString(name string, scope *Scope, line int, strict bool) (string, bool, error) {
	if isDirectTypeName(name) {
		return name, true, nil
	}

	seen := map[string]struct{}{}
	current := name
	for {
		if isDirectTypeName(current) {
			return current, true, nil
		}
		if _, ok := scope.resolveObject(current); ok {
			return current, true, nil
		}
		if _, exists := seen[current]; exists {
			if strict {
				return "", false, semanticErrorf(line, "Cyclic type alias detected at '%s'", current)
			}
			return "", false, nil
		}
		seen[current] = struct{}{}

		alias, ok := scope.resolveAliasRaw(current)
		if !ok {
			if strict {
				return "", false, semanticErrorf(line, "Unknown type: %s", current)
			}
			return "", false, nil
		}
		current = alias
	}
}

func isDirectTypeName(name string) bool {
	if _, ok := baseTypeNames[name]; ok {
		return true
	}
	if strings.HasPrefix(name, "money[") && strings.HasSuffix(name, "]") {
		return true
	}
	if strings.HasPrefix(name, "(") && strings.Contains(name, "->") {
		return true
	}
	if bracket := strings.Index(name, "["); bracket > 0 && strings.HasSuffix(name, "]") {
		base := name[:bracket]
		_, ok := genericBaseArity[base]
		return ok
	}
	return false
}

func renderType(typeNode any) (string, bool) {
	switch node := typeNode.(type) {
	case nil:
		return "", false
	case *ast.TypeName:
		return node.Name, true
	case *ast.GenericType:
		params := make([]string, 0, len(node.Params))
		for _, param := range node.Params {
			rendered, ok := renderType(param)
			if !ok {
				rendered = "?"
			}
			params = append(params, rendered)
		}
		return fmt.Sprintf("%s[%s]", node.Base, strings.Join(params, ", ")), true
	case *ast.FuncType:
		params := make([]string, 0, len(node.ParamTypes))
		for _, param := range node.ParamTypes {
			rendered, ok := renderType(param)
			if !ok {
				rendered = "?"
			}
			params = append(params, rendered)
		}
		returnType, ok := renderType(node.ReturnType)
		if !ok {
			returnType = "void"
		}
		return fmt.Sprintf("(%s) -> %s", strings.Join(params, ", "), returnType), true
	default:
		return "", false
	}
}

func semanticErrorf(line int, format string, args ...any) error {
	return &SemanticError{Message: fmt.Sprintf(format, args...), Line: line}
}

func moduleStmtLabel(stmt any) string {
	switch stmt.(type) {
	case *ast.UseStmt:
		return "use"
	case *ast.FuncDef:
		return "func"
	case *ast.ObjectDef:
		return "object"
	case *ast.TypeAlias:
		return "type"
	case *ast.Assignment:
		return "assignment"
	case *ast.VarDecl:
		return "var"
	case *ast.VarBlock:
		return "var block"
	case *ast.IfStmt:
		return "if"
	case *ast.WhileStmt:
		return "while"
	case *ast.ForRangeStmt, *ast.ForEachStmt:
		return "for"
	case *ast.MatchStmt:
		return "match"
	case *ast.ParallelStmt:
		return "parallel"
	case *ast.GlobalStmt:
		return "global"
	case *ast.ArenaStmt:
		return "arena"
	case *ast.TagStmt:
		return "@tag"
	case *ast.ExprStmt:
		return "expression"
	case *ast.ModuleDef:
		return "module"
	default:
		return fmt.Sprintf("%T", stmt)
	}
}

func lineOf(node any) int {
	switch n := node.(type) {
	case *ast.TypeName:
		return n.Line
	case *ast.GenericType:
		return n.Line
	case *ast.FuncType:
		return n.Line
	case *ast.IntLiteral:
		return n.Line
	case *ast.FloatLiteral:
		return n.Line
	case *ast.StringLiteral:
		return n.Line
	case *ast.BoolLiteral:
		return n.Line
	case *ast.Identifier:
		return n.Line
	case *ast.BinaryOp:
		return n.Line
	case *ast.UnaryOp:
		return n.Line
	case *ast.FuncCall:
		return n.Line
	case *ast.MemberAccess:
		return n.Line
	case *ast.IndexAccess:
		return n.Line
	case *ast.Lambda:
		return n.Line
	case *ast.OkExpr:
		return n.Line
	case *ast.ErrExpr:
		return n.Line
	case *ast.ListLiteral:
		return n.Line
	case *ast.DictLiteral:
		return n.Line
	case *ast.AsExpr:
		return n.Line
	case *ast.ObjectLiteral:
		return n.Line
	case *ast.Assignment:
		return n.Line
	case *ast.VarDecl:
		return n.Line
	case *ast.VarBlock:
		return n.Line
	case *ast.ReturnStmt:
		return n.Line
	case *ast.IfStmt:
		return n.Line
	case *ast.WhileStmt:
		return n.Line
	case *ast.ForRangeStmt:
		return n.Line
	case *ast.ForEachStmt:
		return n.Line
	case *ast.MatchStmt:
		return n.Line
	case *ast.WhenClause:
		return n.Line
	case *ast.FuncDef:
		return n.Line
	case *ast.ModuleDef:
		return n.Line
	case *ast.UseStmt:
		return n.Line
	case *ast.ParallelStmt:
		return n.Line
	case *ast.GlobalStmt:
		return n.Line
	case *ast.ArenaStmt:
		return n.Line
	case *ast.FieldDef:
		return n.Line
	case *ast.MethodDef:
		return n.Line
	case *ast.ConstructorDef:
		return n.Line
	case *ast.ObjectDef:
		return n.Line
	case *ast.TagStmt:
		return n.Line
	case *ast.TypeAlias:
		return n.Line
	case *ast.ExprStmt:
		return n.Line
	default:
		return 0
	}
}
