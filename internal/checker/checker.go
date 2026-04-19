package checker

import (
	"fmt"
	"os"
	"path/filepath"
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
	RuntimeExports map[string]struct{}
	ObjectExports  map[string]*ObjectInfo
	TypeExports    map[string]string
}

type ObjectInfo struct {
	Name    string
	Fields  map[string]string
	Methods map[string]struct{}
}

type Scope struct {
	parent  *Scope
	aliases map[string]string
	objects map[string]*ObjectInfo
}

var baseTypeNames = map[string]struct{}{
	"int":     {},
	"float":   {},
	"string":  {},
	"bool":    {},
	"int8":    {},
	"int16":   {},
	"int32":   {},
	"int64":   {},
	"uint8":   {},
	"uint16":  {},
	"uint32":  {},
	"uint64":  {},
	"float32": {},
	"float64": {},
	"list":    {},
	"dict":    {},
	"func":    {},
	"result":  {},
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

var stdlibModules = map[string][]string{
	"list": {
		"append",
		"concat",
		"removeat",
		"sort",
		"asc",
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
}

func New() *Checker {
	globalScope := newScope(nil)
	checker := &Checker{
		globalScope:       globalScope,
		modules:           map[string]*ModuleInfo{},
		moduleSearchPaths: []string{},
		loadingModules:    map[string]struct{}{},
	}
	checker.setupStdlibModules()
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

func (c *Checker) setupStdlibModules() {
	for moduleName, names := range stdlibModules {
		module := newModuleInfo(moduleName)
		for _, name := range names {
			module.RuntimeExports[name] = struct{}{}
		}
		c.modules[moduleName] = module
	}
}

func newScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		aliases: map[string]string{},
		objects: map[string]*ObjectInfo{},
	}
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
		RuntimeExports: map[string]struct{}{},
		ObjectExports:  map[string]*ObjectInfo{},
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
		deferredScope := scope
		*deferred = append(*deferred, func() error {
			return c.checkFuncBody(node, deferredScope)
		})
		return nil

	case *ast.Assignment:
		for _, value := range node.Values {
			if err := c.checkExpr(value, scope, deferred); err != nil {
				return err
			}
		}
		for _, target := range node.Targets {
			if err := c.checkAssignmentTarget(target, scope, deferred, node.Line); err != nil {
				return err
			}
		}
		return nil

	case *ast.VarDecl:
		if node.TypeName != nil {
			if _, err := c.resolveTypeNode(node.TypeName, scope, node.Line); err != nil {
				return err
			}
		}
		if node.Value != nil {
			return c.checkExpr(node.Value, scope, deferred)
		}
		return nil

	case *ast.VarBlock:
		if node.DefaultValue != nil {
			if err := c.checkExpr(node.DefaultValue, scope, deferred); err != nil {
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
		return c.checkReturnValue(node.Value, scope, deferred)

	case *ast.IfStmt:
		if err := c.checkExpr(node.Condition, scope, deferred); err != nil {
			return err
		}
		if err := c.checkBlock(node.Body, newScope(scope), deferred); err != nil {
			return err
		}
		for _, branch := range node.Elifs {
			if err := c.checkExpr(branch.Condition, scope, deferred); err != nil {
				return err
			}
			if err := c.checkBlock(branch.Body, newScope(scope), deferred); err != nil {
				return err
			}
		}
		return c.checkBlock(node.ElseBody, newScope(scope), deferred)

	case *ast.WhileStmt:
		if err := c.checkExpr(node.Condition, scope, deferred); err != nil {
			return err
		}
		return c.checkBlock(node.Body, newScope(scope), deferred)

	case *ast.ForRangeStmt:
		if err := c.checkExpr(node.Start, scope, deferred); err != nil {
			return err
		}
		if err := c.checkExpr(node.End, scope, deferred); err != nil {
			return err
		}
		if node.Step != nil {
			if err := c.checkExpr(node.Step, scope, deferred); err != nil {
				return err
			}
		}
		return c.checkBlock(node.Body, newScope(scope), deferred)

	case *ast.ForEachStmt:
		if err := c.checkExpr(node.Iterable, scope, deferred); err != nil {
			return err
		}
		return c.checkBlock(node.Body, newScope(scope), deferred)

	case *ast.MatchStmt:
		if err := c.checkExpr(node.Subject, scope, deferred); err != nil {
			return err
		}
		for _, clause := range node.Cases {
			caseScope := newScope(scope)
			for _, pattern := range clause.Patterns {
				if err := c.checkExpr(pattern, caseScope, deferred); err != nil {
					return err
				}
			}
			if err := c.checkBlock(clause.Body, caseScope, deferred); err != nil {
				return err
			}
		}
		return c.checkBlock(node.ElseBody, newScope(scope), deferred)

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
		return nil

	case *ast.UseStmt:
		return c.checkUse(node, scope, deferred)

	case *ast.ParallelStmt:
		return c.checkBlock(node.Body, newScope(scope), deferred)

	case *ast.GlobalStmt:
		if node.Value == nil {
			return nil
		}
		return c.checkExpr(node.Value, scope, deferred)

	case *ast.ArenaStmt:
		return c.checkBlock(node.Body, newScope(scope), deferred)

	case *ast.TagStmt:
		return nil

	case *ast.ObjectDef:
		return c.checkObjectDef(node, scope, deferred)

	case *ast.ExprStmt:
		return c.checkExpr(node.Expr, scope, deferred)

	default:
		return semanticErrorf(lineOf(stmt), "Unknown statement type: %T", stmt)
	}
}

func (c *Checker) checkReturnValue(value any, scope *Scope, deferred *[]func() error) error {
	switch values := value.(type) {
	case nil:
		return nil
	case []any:
		for _, value := range values {
			if err := c.checkExpr(value, scope, deferred); err != nil {
				return err
			}
		}
		return nil
	default:
		return c.checkExpr(value, scope, deferred)
	}
}

func (c *Checker) checkAssignmentTarget(target any, scope *Scope, deferred *[]func() error, line int) error {
	switch node := target.(type) {
	case string:
		return nil
	case *ast.MemberAccess:
		return c.checkExpr(node.Object, scope, deferred)
	case *ast.IndexAccess:
		if err := c.checkExpr(node.Object, scope, deferred); err != nil {
			return err
		}
		return c.checkExpr(node.Index, scope, deferred)
	default:
		return semanticErrorf(line, "Invalid assignment target: %T", target)
	}
}

func (c *Checker) checkFuncBody(fn *ast.FuncDef, closureScope *Scope) error {
	if err := c.validateReturnAnnotation(fn.ReturnType, closureScope, fn.Line); err != nil {
		return err
	}
	bodyScope := newScope(closureScope)
	for _, param := range fn.Params {
		if err := c.checkParam(param, closureScope); err != nil {
			return err
		}
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
	bodyScope := newScope(closureScope)
	for _, param := range ctor.Params {
		if err := c.checkParam(param, closureScope); err != nil {
			return err
		}
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
	bodyScope := newScope(closureScope)
	for _, param := range method.Params {
		if err := c.checkParam(param, closureScope); err != nil {
			return err
		}
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
		if err := c.checkExpr(param.Default, scope, &deferred); err != nil {
			return err
		}
		return c.runDeferred(&deferred)
	}
	return nil
}

func (c *Checker) checkObjectDef(objDef *ast.ObjectDef, scope *Scope, deferred *[]func() error) error {
	obj := &ObjectInfo{
		Name:    objDef.Name,
		Fields:  map[string]string{},
		Methods: map[string]struct{}{},
	}
	scope.defineObject(obj)

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
		obj.Methods[method.Name] = struct{}{}
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

func (c *Checker) checkExpr(expr any, scope *Scope, deferred *[]func() error) error {
	switch node := expr.(type) {
	case nil:
		return nil
	case *ast.IntLiteral, *ast.FloatLiteral, *ast.StringLiteral, *ast.BoolLiteral, *ast.Identifier:
		return nil
	case *ast.BinaryOp:
		if err := c.checkExpr(node.Left, scope, deferred); err != nil {
			return err
		}
		return c.checkExpr(node.Right, scope, deferred)
	case *ast.UnaryOp:
		return c.checkExpr(node.Operand, scope, deferred)
	case *ast.FuncCall:
		if err := c.checkExpr(node.Name, scope, deferred); err != nil {
			return err
		}
		for _, arg := range node.Args {
			if err := c.checkExpr(arg, scope, deferred); err != nil {
				return err
			}
		}
		return nil
	case *ast.MemberAccess:
		return c.checkExpr(node.Object, scope, deferred)
	case *ast.IndexAccess:
		if err := c.checkExpr(node.Object, scope, deferred); err != nil {
			return err
		}
		return c.checkExpr(node.Index, scope, deferred)
	case *ast.Lambda:
		lambdaScope := newScope(scope)
		for _, param := range node.Params {
			if err := c.checkParam(param, scope); err != nil {
				return err
			}
		}
		return c.checkBlock(node.Body, lambdaScope, deferred)
	case *ast.OkExpr:
		return c.checkExpr(node.Value, scope, deferred)
	case *ast.ErrExpr:
		return c.checkExpr(node.Value, scope, deferred)
	case *ast.ListLiteral:
		for _, element := range node.Elements {
			if err := c.checkExpr(element, scope, deferred); err != nil {
				return err
			}
		}
		return nil
	case *ast.DictLiteral:
		if _, err := c.resolveTypeNode(node.KeyType, scope, node.Line); err != nil {
			return err
		}
		if _, err := c.resolveTypeNode(node.ValueType, scope, node.Line); err != nil {
			return err
		}
		for _, entry := range node.Entries {
			if err := c.checkExpr(entry.Key, scope, deferred); err != nil {
				return err
			}
			if err := c.checkExpr(entry.Value, scope, deferred); err != nil {
				return err
			}
		}
		return nil
	case *ast.AsExpr:
		if err := c.checkExpr(node.Expr, scope, deferred); err != nil {
			return err
		}
		_, _, err := c.resolveTypeNameString(node.TypeName, scope, node.Line, true)
		return err
	case *ast.ObjectLiteral:
		return c.checkObjectLiteral(node, scope, deferred)
	default:
		return semanticErrorf(lineOf(expr), "Unknown expression type: %T", expr)
	}
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
		if obj.Fields != nil {
			if _, exists := obj.Fields[field.Name]; !exists {
				return semanticErrorf(literal.Line, "Object '%s' has no field '%s'", literal.Name, field.Name)
			}
		}
		if err := c.checkExpr(field.Value, scope, deferred); err != nil {
			return err
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
			if err := addRuntimeExport(module, moduleDef.Name, node.Name, node.Line); err != nil {
				return nil, err
			}
		case *ast.ObjectDef:
			if !node.Exported {
				continue
			}
			if err := addRuntimeExport(module, moduleDef.Name, node.Name, node.Line); err != nil {
				return nil, err
			}
			obj, _ := moduleScope.resolveObject(node.Name)
			if obj == nil {
				obj = &ObjectInfo{Name: node.Name}
			}
			module.ObjectExports[node.Name] = obj
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

func addRuntimeExport(module *ModuleInfo, moduleName, exportName string, line int) error {
	if _, exists := module.RuntimeExports[exportName]; exists {
		return semanticErrorf(line, "Module '%s' exports runtime name '%s' more than once", moduleName, exportName)
	}
	module.RuntimeExports[exportName] = struct{}{}
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
		return nil
	}

	for _, name := range use.Names {
		imported := false
		if _, ok := module.RuntimeExports[name]; ok {
			imported = true
		}
		if obj, ok := module.ObjectExports[name]; ok {
			scope.defineObject(obj)
			imported = true
		}
		if target, ok := module.TypeExports[name]; ok {
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
		c.AddModuleSearchPath(filepath.Dir(candidate))
		err = c.checkStmt(moduleDef, c.globalScope, deferred)
		delete(c.loadingModules, moduleName)
		return err
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
		for _, param := range node.Params {
			if _, err := c.resolveTypeNode(param, scope, node.Line); err != nil {
				return "", err
			}
		}
		rendered, _ := renderType(node)
		return rendered, nil

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
