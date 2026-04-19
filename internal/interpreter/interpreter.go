package interpreter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/Cass-ette/gwen-lang/internal/ast"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

var (
	errUnknownStatement  = errors.New("unknown statement type")
	errUnknownExpression = errors.New("unknown expression type")
)

var intRanges = map[string]struct {
	min int64
	max uint64
}{
	"int8":   {min: -1 << 7, max: 1<<7 - 1},
	"int16":  {min: -1 << 15, max: 1<<15 - 1},
	"int32":  {min: -1 << 31, max: 1<<31 - 1},
	"int64":  {min: -1 << 63, max: 1<<63 - 1},
	"uint8":  {min: 0, max: 1<<8 - 1},
	"uint16": {min: 0, max: 1<<16 - 1},
	"uint32": {min: 0, max: 1<<32 - 1},
}

const moneyScale int64 = 10_000

var officialStdlibModules = map[string][]string{
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
}

type RuntimeError struct {
	Message string
	Line    int
}

func (e *RuntimeError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("runtime error at L%d: %s", e.Line, e.Message)
	}
	return e.Message
}

type Builtin func(args []any) (any, error)

type OkValue struct {
	Value any
}

func (v *OkValue) String() string {
	return fmt.Sprintf("ok(%s)", formatValue(v.Value))
}

type ErrValue struct {
	Value any
}

func (v *ErrValue) String() string {
	return fmt.Sprintf("err(%s)", formatValue(v.Value))
}

type MoneyValue struct {
	Raw      int64
	Currency string
}

type Function struct {
	Node    *ast.FuncDef
	Closure *Environment
}

type Lambda struct {
	Node    *ast.Lambda
	Closure *Environment
}

type ObjectType struct {
	Name        string
	FieldOrder  []string
	FieldTypes  map[string]string
	Constructor *ast.ConstructorDef
	Methods     map[string]*ast.MethodDef
	Closure     *Environment
}

type ObjectValue struct {
	TypeName string
	Fields   map[string]any
	Object   *ObjectType
}

type BoundMethod struct {
	Instance *ObjectValue
	Object   *ObjectType
	Method   *ast.MethodDef
}

type StaticMethodRef struct {
	Object *ObjectType
	Method *ast.MethodDef
}

type ConstructorRef struct {
	Object *ObjectType
}

type Environment struct {
	vars    map[string]any
	types   map[string]string
	aliases map[string]string
	consts  map[string]struct{}
	parent  *Environment
	self    *ObjectValue
}

func NewEnvironment(parent *Environment) *Environment {
	return &Environment{
		vars:    map[string]any{},
		types:   map[string]string{},
		aliases: map[string]string{},
		consts:  map[string]struct{}{},
		parent:  parent,
	}
}

func (e *Environment) Get(name string) (any, error) {
	if value, ok := e.vars[name]; ok {
		return value, nil
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return nil, &RuntimeError{Message: fmt.Sprintf("Undefined variable: %s", name)}
}

func (e *Environment) GetLocal(name string) (any, bool) {
	value, ok := e.vars[name]
	return value, ok
}

func (e *Environment) Set(name string, value any) {
	e.vars[name] = value
}

func (e *Environment) Update(name string, value any) {
	e.vars[name] = value
}

func (e *Environment) SetType(name, typeName string) {
	if typeName != "" {
		e.types[name] = typeName
	}
}

func (e *Environment) GetLocalType(name string) string {
	return e.types[name]
}

func (e *Environment) SetAlias(name, target string) {
	e.aliases[name] = target
}

func (e *Environment) GetAlias(name string) (string, bool) {
	if target, ok := e.aliases[name]; ok {
		return target, true
	}
	if e.parent != nil {
		return e.parent.GetAlias(name)
	}
	return "", false
}

func (e *Environment) GetLocalAlias(name string) (string, bool) {
	target, ok := e.aliases[name]
	return target, ok
}

func (e *Environment) MarkConst(name string) {
	e.consts[name] = struct{}{}
}

func (e *Environment) IsConst(name string) bool {
	if _, ok := e.consts[name]; ok {
		return true
	}
	if e.parent != nil {
		return e.parent.IsConst(name)
	}
	return false
}

type returnSignal struct {
	value any
}

type uninitialized struct{}

var uninitializedValue = uninitialized{}

type Interpreter struct {
	GlobalEnv         *Environment
	Modules           map[string]*Environment
	Stdout            io.Writer
	moduleSearchPaths []string
	loadingModules    map[string]struct{}
	stdioMu           *sync.Mutex
}

func New() *Interpreter {
	env := NewEnvironment(nil)
	interp := &Interpreter{
		GlobalEnv:         env,
		Modules:           map[string]*Environment{},
		Stdout:            os.Stdout,
		moduleSearchPaths: []string{},
		loadingModules:    map[string]struct{}{},
		stdioMu:           &sync.Mutex{},
	}
	interp.setupBuiltins()
	interp.setupStdlibModules()
	return interp
}

func (i *Interpreter) Run(program *ast.Program) error {
	return i.RunWithSource(program, "")
}

func (i *Interpreter) Execute(program *ast.Program) error {
	_, err := i.execBlock(program.Statements, i.GlobalEnv)
	return err
}

func (i *Interpreter) RunWithSource(program *ast.Program, sourcePath string) error {
	if sourcePath != "" {
		i.AddModuleSearchPath(filepath.Dir(sourcePath))
	}
	if err := i.Execute(program); err != nil {
		return err
	}

	mainValue, err := i.GlobalEnv.Get("main")
	if err != nil {
		var runtimeErr *RuntimeError
		if errors.As(err, &runtimeErr) && strings.Contains(runtimeErr.Message, "Undefined variable: main") {
			return nil
		}
		return err
	}

	fn, ok := mainValue.(*Function)
	if !ok {
		return nil
	}
	_, err = i.callFunction(fn, nil, fn.Node.Line)
	return err
}

func (i *Interpreter) AddModuleSearchPath(path string) {
	if path == "" {
		return
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}
	for _, existing := range i.moduleSearchPaths {
		if existing == absPath {
			return
		}
	}
	i.moduleSearchPaths = append([]string{absPath}, i.moduleSearchPaths...)
}

func (i *Interpreter) execBlock(statements []any, env *Environment) (*returnSignal, error) {
	for _, stmt := range statements {
		result, err := i.execStmt(stmt, env)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}
	return nil, nil
}

func (i *Interpreter) execStmt(stmt any, env *Environment) (*returnSignal, error) {
	switch node := stmt.(type) {
	case *ast.FuncDef:
		env.Set(node.Name, &Function{Node: node, Closure: env})
		return nil, nil

	case *ast.Assignment:
		values := make([]any, 0, len(node.Values))
		for _, valueExpr := range node.Values {
			value, err := i.evalExpr(valueExpr, env)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}

		targets := node.Targets
		if len(targets) > 1 && len(values) == 1 {
			if unpacked, ok := values[0].([]any); ok {
				values = unpacked
			}
		}
		if len(targets) != len(values) {
			return nil, runtimeErrorf(node.Line, "assignment count mismatch: %d targets, %d values", len(targets), len(values))
		}

		for idx, target := range targets {
			if err := i.assignTarget(target, values[idx], env, node.Line); err != nil {
				return nil, err
			}
		}
		return nil, nil

	case *ast.VarDecl:
		if env.IsConst(node.Name) {
			return nil, runtimeErrorf(node.Line, "Cannot redeclare const variable: %s", node.Name)
		}
		typeName, err := resolveRuntimeType(node.TypeName, env, node.Line)
		if err != nil {
			return nil, err
		}
		if node.IsUninit {
			if node.IsConst {
				return nil, runtimeErrorf(node.Line, "Const variable '%s' must be initialized", node.Name)
			}
			env.Set(node.Name, uninitializedValue)
			env.SetType(node.Name, typeName)
			return nil, nil
		}

		var value any
		if node.Value != nil {
			value, err = i.evalExpr(node.Value, env)
			if err != nil {
				return nil, err
			}
		}
		value, err = coerceIfTyped(value, typeName, node.Line)
		if err != nil {
			return nil, err
		}
		env.Set(node.Name, value)
		env.SetType(node.Name, typeName)
		if node.IsConst {
			env.MarkConst(node.Name)
		}
		return nil, nil

	case *ast.VarBlock:
		var sharedDefault any
		var err error
		if node.DefaultMode == "value" && node.DefaultValue != nil {
			sharedDefault, err = i.evalExpr(node.DefaultValue, env)
			if err != nil {
				return nil, err
			}
		}
		for _, decl := range node.Decls {
			var value any
			typeName, err := resolveRuntimeType(decl.TypeName, env, decl.Line)
			if err != nil {
				return nil, err
			}
			switch {
			case decl.Value != nil:
				value, err = i.evalExpr(decl.Value, env)
				if err != nil {
					return nil, err
				}
			case node.DefaultMode == "value":
				value = sharedDefault
			case node.DefaultMode == "zero":
				value, err = zeroValue(typeName, decl.Line)
				if err != nil {
					return nil, err
				}
			default:
				value = uninitializedValue
			}
			value, err = coerceIfTyped(value, typeName, decl.Line)
			if err != nil {
				return nil, err
			}
			env.Set(decl.Name, value)
			env.SetType(decl.Name, typeName)
		}
		return nil, nil

	case *ast.TypeAlias:
		target, err := resolveRuntimeType(node.Target, env, node.Line)
		if err != nil {
			return nil, err
		}
		env.SetAlias(node.Name, target)
		return nil, nil

	case *ast.ReturnStmt:
		if node.Value == nil {
			return &returnSignal{value: nil}, nil
		}
		if values, ok := node.Value.([]any); ok {
			result := make([]any, 0, len(values))
			for _, valueExpr := range values {
				value, err := i.evalExpr(valueExpr, env)
				if err != nil {
					return nil, err
				}
				result = append(result, value)
			}
			return &returnSignal{value: result}, nil
		}
		value, err := i.evalExpr(node.Value, env)
		if err != nil {
			return nil, err
		}
		return &returnSignal{value: value}, nil

	case *ast.IfStmt:
		condition, err := i.evalExpr(node.Condition, env)
		if err != nil {
			return nil, err
		}
		if err := requireBool(condition, "'if' condition", node.Line); err != nil {
			return nil, err
		}
		if condition.(bool) {
			return i.execBlock(node.Body, env)
		}
		for _, branch := range node.Elifs {
			condition, err := i.evalExpr(branch.Condition, env)
			if err != nil {
				return nil, err
			}
			if err := requireBool(condition, "'elif' condition", node.Line); err != nil {
				return nil, err
			}
			if condition.(bool) {
				return i.execBlock(branch.Body, env)
			}
		}
		if len(node.ElseBody) > 0 {
			return i.execBlock(node.ElseBody, env)
		}
		return nil, nil

	case *ast.WhileStmt:
		for {
			condition, err := i.evalExpr(node.Condition, env)
			if err != nil {
				return nil, err
			}
			if err := requireBool(condition, "'while' condition", node.Line); err != nil {
				return nil, err
			}
			if !condition.(bool) {
				return nil, nil
			}
			result, err := i.execBlock(node.Body, env)
			if err != nil {
				return nil, err
			}
			if result != nil {
				return result, nil
			}
		}

	case *ast.ForRangeStmt:
		start, err := i.evalExpr(node.Start, env)
		if err != nil {
			return nil, err
		}
		end, err := i.evalExpr(node.End, env)
		if err != nil {
			return nil, err
		}
		startInt, ok := start.(int64)
		if !ok {
			return nil, runtimeErrorf(node.Line, "for-range start must be int, got %s", typeNameOf(start))
		}
		endInt, ok := end.(int64)
		if !ok {
			return nil, runtimeErrorf(node.Line, "for-range end must be int, got %s", typeNameOf(end))
		}

		var step int64
		if node.Step != nil {
			stepValue, err := i.evalExpr(node.Step, env)
			if err != nil {
				return nil, err
			}
			var ok bool
			step, ok = stepValue.(int64)
			if !ok {
				return nil, runtimeErrorf(node.Line, "for-range step must be int, got %s", typeNameOf(stepValue))
			}
			if step == 0 {
				return nil, runtimeErrorf(node.Line, "for-range step cannot be 0")
			}
		} else if startInt <= endInt {
			step = 1
		} else {
			step = -1
		}

		switch node.Direction {
		case "asc":
			if startInt > endInt {
				startInt, endInt = endInt, startInt
			}
			if step < 0 {
				step = -step
			}
			if step == 0 {
				step = 1
			}
		case "desc":
			if startInt < endInt {
				startInt, endInt = endInt, startInt
			}
			if step > 0 {
				step = -step
			}
			if step == 0 {
				step = -1
			}
		}

		for current := startInt; compareRange(current, endInt, step); current += step {
			env.Update(node.Var, current)
			result, err := i.execBlock(node.Body, env)
			if err != nil {
				return nil, err
			}
			if result != nil {
				return result, nil
			}
		}
		return nil, nil

	case *ast.ForEachStmt:
		iterable, err := i.evalExpr(node.Iterable, env)
		if err != nil {
			return nil, err
		}
		items, err := toSlice(iterable, node.Line)
		if err != nil {
			return nil, err
		}
		for idx, item := range items {
			env.Update(node.Var, item)
			if node.IndexVar != "" {
				env.Update(node.IndexVar, int64(idx))
			}
			result, err := i.execBlock(node.Body, env)
			if err != nil {
				return nil, err
			}
			if result != nil {
				return result, nil
			}
		}
		return nil, nil

	case *ast.MatchStmt:
		subject, err := i.evalExpr(node.Subject, env)
		if err != nil {
			return nil, err
		}
		for _, clause := range node.Cases {
			matched, bindings, err := i.matchPatterns(subject, clause.Patterns, env)
			if err != nil {
				return nil, err
			}
			if !matched {
				continue
			}
			for name, value := range bindings {
				env.Set(name, value)
			}
			return i.execBlock(clause.Body, env)
		}
		if len(node.ElseBody) > 0 {
			return i.execBlock(node.ElseBody, env)
		}
		return nil, runtimeErrorf(node.Line, "match statement has no matching case and no 'else' branch")

	case *ast.ModuleDef:
		moduleEnv := NewEnvironment(env)
		if _, err := i.execBlock(node.Body, moduleEnv); err != nil {
			return nil, err
		}
		namespace := NewEnvironment(nil)
		for _, inner := range node.Body {
			switch decl := inner.(type) {
			case *ast.FuncDef:
				if decl.Exported {
					value, err := moduleEnv.Get(decl.Name)
					if err != nil {
						return nil, err
					}
					namespace.Set(decl.Name, value)
				}
			case *ast.ObjectDef:
				if decl.Exported {
					value, err := moduleEnv.Get(decl.Name)
					if err != nil {
						return nil, err
					}
					namespace.Set(decl.Name, value)
				}
			case *ast.TypeAlias:
				if decl.Exported {
					target, ok := moduleEnv.GetLocalAlias(decl.Name)
					if !ok {
						return nil, runtimeErrorf(decl.Line, "Exported type alias '%s' was not defined", decl.Name)
					}
					namespace.SetAlias(decl.Name, target)
				}
			}
		}
		i.Modules[node.Name] = namespace
		env.Set(node.Name, namespace)
		return nil, nil

	case *ast.UseStmt:
		namespace, ok := i.Modules[node.Module]
		if !ok {
			if err := i.loadModuleFromFile(node.Module, node.Line); err != nil {
				return nil, err
			}
			namespace, ok = i.Modules[node.Module]
			if !ok {
				return nil, runtimeErrorf(node.Line, "Module not found: %s", node.Module)
			}
		}
		if len(node.Names) == 0 {
			env.Set(node.Module, namespace)
			return nil, nil
		}
		for _, name := range node.Names {
			imported := false
			if value, ok := namespace.GetLocal(name); ok {
				env.Set(name, value)
				imported = true
			}
			if alias, ok := namespace.GetLocalAlias(name); ok {
				env.SetAlias(name, alias)
				imported = true
			}
			if !imported {
				return nil, runtimeErrorf(node.Line, "Module '%s' does not export '%s'", node.Module, name)
			}
		}
		return nil, nil

	case *ast.TagStmt:
		return nil, nil

	case *ast.ObjectDef:
		fieldOrder := make([]string, 0, len(node.Fields))
		fieldTypes := make(map[string]string, len(node.Fields))
		for _, field := range node.Fields {
			typeName, err := resolveRuntimeType(field.TypeAnnotation, env, field.Line)
			if err != nil {
				return nil, err
			}
			fieldOrder = append(fieldOrder, field.Name)
			fieldTypes[field.Name] = typeName
		}
		methods := make(map[string]*ast.MethodDef, len(node.Methods))
		for _, method := range node.Methods {
			methods[method.Name] = method
		}
		env.Set(node.Name, &ObjectType{
			Name:        node.Name,
			FieldOrder:  fieldOrder,
			FieldTypes:  fieldTypes,
			Constructor: node.Constructor,
			Methods:     methods,
			Closure:     env,
		})
		return nil, nil

	case *ast.ArenaStmt:
		return i.execBlock(node.Body, env)

	case *ast.ParallelStmt:
		return i.execParallel(node, env)

	case *ast.ExprStmt:
		_, err := i.evalExpr(node.Expr, env)
		return nil, err

	default:
		return nil, fmt.Errorf("%w: %T", errUnknownStatement, stmt)
	}
}

type parallelTaskResult struct {
	value any
	err   error
}

func (i *Interpreter) execParallel(node *ast.ParallelStmt, env *Environment) (*returnSignal, error) {
	results := make([]parallelTaskResult, len(node.Body))
	var wg sync.WaitGroup

	for idx, inner := range node.Body {
		taskInterp, taskEnv := i.forkForParallel(env)
		wg.Add(1)
		go func(idx int, stmt any, interp *Interpreter, scope *Environment) {
			defer wg.Done()
			value, err := interp.execParallelTask(stmt, scope)
			results[idx] = parallelTaskResult{value: value, err: err}
		}(idx, inner, taskInterp, taskEnv)
	}

	wg.Wait()

	if !node.AllowFail {
		for _, result := range results {
			if result.err != nil {
				return nil, result.err
			}
		}
	}

	if node.ResultVar != "" {
		parallelValues := make([]any, 0, len(results))
		for _, result := range results {
			if result.err != nil {
				parallelValues = append(parallelValues, &ErrValue{Value: result.err.Error()})
				continue
			}
			parallelValues = append(parallelValues, &OkValue{Value: result.value})
		}
		env.Set(node.ResultVar, parallelValues)
	}

	return nil, nil
}

func (i *Interpreter) execParallelTask(stmt any, env *Environment) (any, error) {
	if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
		return i.evalExpr(exprStmt.Expr, env)
	}
	result, err := i.execStmt(stmt, env)
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result.value, nil
	}
	return nil, nil
}

func (i *Interpreter) forkForParallel(env *Environment) (*Interpreter, *Environment) {
	child := New()
	child.Stdout = i.Stdout
	child.stdioMu = i.stdioMu
	child.moduleSearchPaths = append([]string{}, i.moduleSearchPaths...)

	cloner := newCloneContext(i, child)
	cloner.cloneEnv(i.GlobalEnv)

	for name, moduleEnv := range i.Modules {
		child.Modules[name] = cloner.cloneEnv(moduleEnv)
	}

	return child, cloner.cloneEnv(env)
}

type cloneContext struct {
	parentBuiltinPtrs map[uintptr]string
	childBuiltins     map[string]Builtin
	envs              map[*Environment]*Environment
	envPopulating     map[*Environment]struct{}
	envPopulated      map[*Environment]struct{}
	functions         map[*Function]*Function
	lambdas           map[*Lambda]*Lambda
	objectTypes       map[*ObjectType]*ObjectType
	objectValues      map[*ObjectValue]*ObjectValue
	boundMethods      map[*BoundMethod]*BoundMethod
	staticMethods     map[*StaticMethodRef]*StaticMethodRef
	constructors      map[*ConstructorRef]*ConstructorRef
	okValues          map[*OkValue]*OkValue
	errValues         map[*ErrValue]*ErrValue
	moneyValues       map[*MoneyValue]*MoneyValue
	slices            map[uintptr][]any
	dicts             map[uintptr]map[any]any
}

func newCloneContext(parent *Interpreter, child *Interpreter) *cloneContext {
	return &cloneContext{
		parentBuiltinPtrs: builtinPointerNames(parent.GlobalEnv),
		childBuiltins:     builtinValues(child.GlobalEnv),
		envs:              map[*Environment]*Environment{parent.GlobalEnv: child.GlobalEnv},
		envPopulating:     map[*Environment]struct{}{},
		envPopulated:      map[*Environment]struct{}{},
		functions:         map[*Function]*Function{},
		lambdas:           map[*Lambda]*Lambda{},
		objectTypes:       map[*ObjectType]*ObjectType{},
		objectValues:      map[*ObjectValue]*ObjectValue{},
		boundMethods:      map[*BoundMethod]*BoundMethod{},
		staticMethods:     map[*StaticMethodRef]*StaticMethodRef{},
		constructors:      map[*ConstructorRef]*ConstructorRef{},
		okValues:          map[*OkValue]*OkValue{},
		errValues:         map[*ErrValue]*ErrValue{},
		moneyValues:       map[*MoneyValue]*MoneyValue{},
		slices:            map[uintptr][]any{},
		dicts:             map[uintptr]map[any]any{},
	}
}

func builtinPointerNames(env *Environment) map[uintptr]string {
	names := map[uintptr]string{}
	for name, value := range env.vars {
		builtin, ok := value.(Builtin)
		if !ok {
			continue
		}
		names[reflect.ValueOf(builtin).Pointer()] = name
	}
	return names
}

func builtinValues(env *Environment) map[string]Builtin {
	values := map[string]Builtin{}
	for name, value := range env.vars {
		builtin, ok := value.(Builtin)
		if !ok {
			continue
		}
		values[name] = builtin
	}
	return values
}

func (c *cloneContext) cloneEnv(src *Environment) *Environment {
	if src == nil {
		return nil
	}
	dst, ok := c.envs[src]
	if !ok {
		dst = NewEnvironment(nil)
		c.envs[src] = dst
	}
	if _, done := c.envPopulated[src]; done {
		return dst
	}
	if _, inProgress := c.envPopulating[src]; inProgress {
		return dst
	}

	c.envPopulating[src] = struct{}{}
	dst.vars = map[string]any{}
	dst.types = map[string]string{}
	dst.aliases = map[string]string{}
	dst.consts = map[string]struct{}{}
	dst.parent = c.cloneEnv(src.parent)
	dst.self = nil

	for name, value := range src.vars {
		dst.vars[name] = c.cloneNamedValue(name, value)
	}
	for name, typeName := range src.types {
		dst.types[name] = typeName
	}
	for name, target := range src.aliases {
		dst.aliases[name] = target
	}
	for name := range src.consts {
		dst.consts[name] = struct{}{}
	}
	if src.self != nil {
		dst.self = c.cloneObjectValue(src.self)
	}

	delete(c.envPopulating, src)
	c.envPopulated[src] = struct{}{}
	return dst
}

func (c *cloneContext) cloneNamedValue(name string, value any) any {
	if builtin, ok := value.(Builtin); ok {
		if childBuiltin, ok := c.remapBuiltin(name, builtin); ok {
			return childBuiltin
		}
	}
	return c.cloneValue(value)
}

func (c *cloneContext) remapBuiltin(name string, builtin Builtin) (Builtin, bool) {
	if name != "" {
		if childBuiltin, ok := c.childBuiltins[name]; ok {
			return childBuiltin, true
		}
	}
	builtinName, ok := c.parentBuiltinPtrs[reflect.ValueOf(builtin).Pointer()]
	if !ok {
		return nil, false
	}
	childBuiltin, ok := c.childBuiltins[builtinName]
	return childBuiltin, ok
}

func (c *cloneContext) cloneValue(value any) any {
	switch value := value.(type) {
	case nil, int64, float64, string, bool, uninitialized:
		return value
	case Builtin:
		if childBuiltin, ok := c.remapBuiltin("", value); ok {
			return childBuiltin
		}
		return value
	case []any:
		ptr := reflect.ValueOf(value).Pointer()
		if ptr != 0 {
			if cloned, ok := c.slices[ptr]; ok {
				return cloned
			}
		}
		cloned := make([]any, len(value))
		if ptr != 0 {
			c.slices[ptr] = cloned
		}
		for idx, item := range value {
			cloned[idx] = c.cloneValue(item)
		}
		return cloned
	case map[any]any:
		ptr := reflect.ValueOf(value).Pointer()
		if ptr != 0 {
			if cloned, ok := c.dicts[ptr]; ok {
				return cloned
			}
		}
		cloned := make(map[any]any, len(value))
		if ptr != 0 {
			c.dicts[ptr] = cloned
		}
		for key, item := range value {
			cloned[c.cloneValue(key)] = c.cloneValue(item)
		}
		return cloned
	case *Environment:
		return c.cloneEnv(value)
	case *Function:
		if cloned, ok := c.functions[value]; ok {
			return cloned
		}
		cloned := &Function{Node: value.Node}
		c.functions[value] = cloned
		cloned.Closure = c.cloneEnv(value.Closure)
		return cloned
	case *Lambda:
		if cloned, ok := c.lambdas[value]; ok {
			return cloned
		}
		cloned := &Lambda{Node: value.Node}
		c.lambdas[value] = cloned
		cloned.Closure = c.cloneEnv(value.Closure)
		return cloned
	case *ObjectType:
		if cloned, ok := c.objectTypes[value]; ok {
			return cloned
		}
		cloned := &ObjectType{
			Name:        value.Name,
			FieldOrder:  append([]string{}, value.FieldOrder...),
			FieldTypes:  map[string]string{},
			Constructor: value.Constructor,
			Methods:     map[string]*ast.MethodDef{},
		}
		c.objectTypes[value] = cloned
		for name, typeName := range value.FieldTypes {
			cloned.FieldTypes[name] = typeName
		}
		for name, method := range value.Methods {
			cloned.Methods[name] = method
		}
		cloned.Closure = c.cloneEnv(value.Closure)
		return cloned
	case *ObjectValue:
		return c.cloneObjectValue(value)
	case *BoundMethod:
		if cloned, ok := c.boundMethods[value]; ok {
			return cloned
		}
		cloned := &BoundMethod{
			Method: value.Method,
		}
		c.boundMethods[value] = cloned
		cloned.Instance = c.cloneObjectValue(value.Instance)
		cloned.Object = c.cloneValue(value.Object).(*ObjectType)
		return cloned
	case *StaticMethodRef:
		if cloned, ok := c.staticMethods[value]; ok {
			return cloned
		}
		cloned := &StaticMethodRef{Method: value.Method}
		c.staticMethods[value] = cloned
		cloned.Object = c.cloneValue(value.Object).(*ObjectType)
		return cloned
	case *ConstructorRef:
		if cloned, ok := c.constructors[value]; ok {
			return cloned
		}
		cloned := &ConstructorRef{}
		c.constructors[value] = cloned
		cloned.Object = c.cloneValue(value.Object).(*ObjectType)
		return cloned
	case *OkValue:
		if cloned, ok := c.okValues[value]; ok {
			return cloned
		}
		cloned := &OkValue{}
		c.okValues[value] = cloned
		cloned.Value = c.cloneValue(value.Value)
		return cloned
	case *ErrValue:
		if cloned, ok := c.errValues[value]; ok {
			return cloned
		}
		cloned := &ErrValue{}
		c.errValues[value] = cloned
		cloned.Value = c.cloneValue(value.Value)
		return cloned
	case *MoneyValue:
		if cloned, ok := c.moneyValues[value]; ok {
			return cloned
		}
		cloned := &MoneyValue{Raw: value.Raw, Currency: value.Currency}
		c.moneyValues[value] = cloned
		return cloned
	default:
		return value
	}
}

func (c *cloneContext) cloneObjectValue(value *ObjectValue) *ObjectValue {
	if value == nil {
		return nil
	}
	if cloned, ok := c.objectValues[value]; ok {
		return cloned
	}
	cloned := &ObjectValue{
		TypeName: value.TypeName,
		Fields:   map[string]any{},
	}
	c.objectValues[value] = cloned
	cloned.Object = c.cloneValue(value.Object).(*ObjectType)
	for name, fieldValue := range value.Fields {
		cloned.Fields[name] = c.cloneValue(fieldValue)
	}
	return cloned
}

func (i *Interpreter) evalExpr(expr any, env *Environment) (any, error) {
	switch node := expr.(type) {
	case *ast.IntLiteral:
		return node.Value, nil
	case *ast.FloatLiteral:
		return node.Value, nil
	case *ast.StringLiteral:
		return node.Value, nil
	case *ast.BoolLiteral:
		return node.Value, nil
	case *ast.Identifier:
		value, err := env.Get(node.Name)
		if err != nil {
			var runtimeErr *RuntimeError
			if errors.As(err, &runtimeErr) {
				runtimeErr.Line = node.Line
			}
			return nil, err
		}
		if value == uninitializedValue {
			return nil, runtimeErrorf(node.Line, "'%s' read before assignment", node.Name)
		}
		return value, nil
	case *ast.ListLiteral:
		items := make([]any, 0, len(node.Elements))
		for _, element := range node.Elements {
			value, err := i.evalExpr(element, env)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
		}
		return items, nil
	case *ast.DictLiteral:
		items := map[any]any{}
		for _, entry := range node.Entries {
			key, err := i.evalExpr(entry.Key, env)
			if err != nil {
				return nil, err
			}
			value, err := i.evalExpr(entry.Value, env)
			if err != nil {
				return nil, err
			}
			items[key] = value
		}
		return items, nil
	case *ast.BinaryOp:
		if node.Op == "and" || node.Op == "or" {
			left, err := i.evalExpr(node.Left, env)
			if err != nil {
				return nil, err
			}
			if err := requireBool(left, fmt.Sprintf("left side of '%s'", node.Op), node.Line); err != nil {
				return nil, err
			}
			if node.Op == "and" {
				if !left.(bool) {
					return false, nil
				}
				right, err := i.evalExpr(node.Right, env)
				if err != nil {
					return nil, err
				}
				if err := requireBool(right, "right side of 'and'", node.Line); err != nil {
					return nil, err
				}
				return right, nil
			}
			if left.(bool) {
				return true, nil
			}
			right, err := i.evalExpr(node.Right, env)
			if err != nil {
				return nil, err
			}
			if err := requireBool(right, "right side of 'or'", node.Line); err != nil {
				return nil, err
			}
			return right, nil
		}
		left, err := i.evalExpr(node.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := i.evalExpr(node.Right, env)
		if err != nil {
			return nil, err
		}
		return evalBinary(node.Op, left, right, node.Line)
	case *ast.UnaryOp:
		operand, err := i.evalExpr(node.Operand, env)
		if err != nil {
			return nil, err
		}
		switch node.Op {
		case "-":
			switch value := operand.(type) {
			case int64:
				return -value, nil
			case float64:
				return -value, nil
			default:
				return nil, runtimeErrorf(node.Line, "cannot negate %s", typeNameOf(operand))
			}
		case "not":
			if err := requireBool(operand, "'not' operand", node.Line); err != nil {
				return nil, err
			}
			return !operand.(bool), nil
		default:
			return nil, runtimeErrorf(node.Line, "unknown unary operator: %s", node.Op)
		}
	case *ast.FuncCall:
		return i.evalCall(node, env)
	case *ast.IndexAccess:
		object, err := i.evalExpr(node.Object, env)
		if err != nil {
			return nil, err
		}
		index, err := i.evalExpr(node.Index, env)
		if err != nil {
			return nil, err
		}
		return indexValue(object, index, node.Line)
	case *ast.Lambda:
		return &Lambda{Node: node, Closure: env}, nil
	case *ast.OkExpr:
		value, err := i.evalExpr(node.Value, env)
		if err != nil {
			return nil, err
		}
		return &OkValue{Value: value}, nil
	case *ast.ErrExpr:
		value, err := i.evalExpr(node.Value, env)
		if err != nil {
			return nil, err
		}
		return &ErrValue{Value: value}, nil
	case *ast.AsExpr:
		value, err := i.evalExpr(node.Expr, env)
		if err != nil {
			return nil, err
		}
		target, err := resolveAlias(node.TypeName, env, node.Line)
		if err != nil {
			return nil, err
		}
		converted, err := convertAs(value, target, node.Line)
		if err != nil {
			return &ErrValue{Value: err.Error()}, nil
		}
		return &OkValue{Value: converted}, nil
	case *ast.MemberAccess:
		object, err := i.evalExpr(node.Object, env)
		if err != nil {
			return nil, err
		}
		switch value := object.(type) {
		case *ObjectValue:
			if method, ok := value.Object.Methods[node.Member]; ok {
				return &BoundMethod{Instance: value, Object: value.Object, Method: method}, nil
			}
			if identifier, ok := node.Object.(*ast.Identifier); ok && identifier.Name == "self" {
				if env.self == nil || env.self != value {
					return nil, runtimeErrorf(node.Line, "Cannot access private field '%s' of '%s' from outside; use a method instead", node.Member, value.TypeName)
				}
				field, ok := value.Fields[node.Member]
				if !ok {
					return nil, runtimeErrorf(node.Line, "Object '%s' has no field '%s'", value.TypeName, node.Member)
				}
				return field, nil
			}
			return nil, runtimeErrorf(node.Line, "Cannot access private field '%s' of '%s' from outside; use a method instead", node.Member, value.TypeName)
		case *ObjectType:
			if node.Member == "new" {
				if value.Constructor == nil {
					return nil, runtimeErrorf(node.Line, "Object '%s' has no constructor", value.Name)
				}
				return &ConstructorRef{Object: value}, nil
			}
			if method, ok := value.Methods[node.Member]; ok {
				return &StaticMethodRef{Object: value, Method: method}, nil
			}
			return nil, runtimeErrorf(node.Line, "Object '%s' has no member '%s'", value.Name, node.Member)
		case *Environment:
			member, err := value.Get(node.Member)
			if err != nil {
				return nil, err
			}
			return member, nil
		default:
			return nil, runtimeErrorf(node.Line, "member access is not implemented for %s", typeNameOf(object))
		}
	case *ast.ObjectLiteral:
		typeValue, err := env.Get(node.Name)
		if err != nil {
			return nil, runtimeErrorf(node.Line, "Unknown object type: %s", node.Name)
		}
		objectType, ok := typeValue.(*ObjectType)
		if !ok {
			return nil, runtimeErrorf(node.Line, "'%s' is not an object type", node.Name)
		}
		fields := make(map[string]any, len(node.Fields))
		seen := make(map[string]struct{}, len(node.Fields))
		for _, field := range node.Fields {
			if _, exists := seen[field.Name]; exists {
				return nil, runtimeErrorf(node.Line, "Duplicate field '%s' in '%s' literal", field.Name, node.Name)
			}
			seen[field.Name] = struct{}{}
			expectedType, ok := objectType.FieldTypes[field.Name]
			if !ok {
				return nil, runtimeErrorf(node.Line, "Object '%s' has no field '%s'", node.Name, field.Name)
			}
			value, err := i.evalExpr(field.Value, env)
			if err != nil {
				return nil, err
			}
			value, err = coerceIfTyped(value, expectedType, node.Line)
			if err != nil {
				return nil, err
			}
			fields[field.Name] = value
		}
		for _, fieldName := range objectType.FieldOrder {
			if _, ok := fields[fieldName]; !ok {
				return nil, runtimeErrorf(node.Line, "Object '%s' literal missing field '%s'", node.Name, fieldName)
			}
		}
		return &ObjectValue{TypeName: node.Name, Fields: fields, Object: objectType}, nil
	default:
		return nil, fmt.Errorf("%w: %T", errUnknownExpression, expr)
	}
}

func (i *Interpreter) evalCall(call *ast.FuncCall, env *Environment) (any, error) {
	if builtinName, ok := builtinCallName(call.Name); ok {
		switch builtinName {
		case "append":
			return i.evalAppendCall(call, env)
		case "pop":
			return i.evalPopCall(call, env)
		case "removeat":
			return i.evalRemoveAtCall(call, env)
		case "insert":
			return i.evalInsertCall(call, env)
		}
	}

	callee, err := i.evalExpr(call.Name, env)
	if err != nil {
		return nil, err
	}
	args := make([]any, 0, len(call.Args))
	for _, argExpr := range call.Args {
		value, err := i.evalExpr(argExpr, env)
		if err != nil {
			return nil, err
		}
		args = append(args, value)
	}

	switch fn := callee.(type) {
	case Builtin:
		return fn(args)
	case *Function:
		return i.callFunction(fn, args, call.Line)
	case *Lambda:
		return i.callLambda(fn, args, call.Line)
	case *BoundMethod:
		return i.callMethod(fn, args, call.Line)
	case *StaticMethodRef:
		if len(args) == 0 {
			return nil, runtimeErrorf(call.Line, "Method '%s.%s' requires explicit self as first argument", fn.Object.Name, fn.Method.Name)
		}
		instance, ok := args[0].(*ObjectValue)
		if !ok || instance.TypeName != fn.Object.Name {
			return nil, runtimeErrorf(call.Line, "First argument to '%s.%s' must be a '%s' instance", fn.Object.Name, fn.Method.Name, fn.Object.Name)
		}
		return i.callMethod(&BoundMethod{Instance: instance, Object: fn.Object, Method: fn.Method}, args[1:], call.Line)
	case *ConstructorRef:
		return i.callConstructor(fn, args, call.Line)
	default:
		return nil, runtimeErrorf(call.Line, "'%s' is not callable", formatValue(callee))
	}
}

func (i *Interpreter) evalAppendCall(call *ast.FuncCall, env *Environment) (any, error) {
	if len(call.Args) != 2 {
		return nil, runtimeErrorf(call.Line, "append() expects 2 arguments, got %d", len(call.Args))
	}
	target, ok := assignmentTargetExpr(call.Args[0])
	if !ok {
		return nil, runtimeErrorf(call.Line, "append() first argument must be assignable list target")
	}
	listValue, err := i.evalExpr(call.Args[0], env)
	if err != nil {
		return nil, err
	}
	list, ok := listValue.([]any)
	if !ok {
		return nil, runtimeErrorf(call.Line, "append() requires list, got %s", typeNameOf(listValue))
	}
	item, err := i.evalExpr(call.Args[1], env)
	if err != nil {
		return nil, err
	}
	list = append(list, item)
	if err := i.assignTarget(target, list, env, call.Line); err != nil {
		return nil, err
	}
	return list, nil
}

func (i *Interpreter) evalPopCall(call *ast.FuncCall, env *Environment) (any, error) {
	if len(call.Args) != 1 {
		return nil, runtimeErrorf(call.Line, "pop() expects 1 argument, got %d", len(call.Args))
	}
	target, ok := assignmentTargetExpr(call.Args[0])
	if !ok {
		return nil, runtimeErrorf(call.Line, "pop() first argument must be assignable list target")
	}
	listValue, err := i.evalExpr(call.Args[0], env)
	if err != nil {
		return nil, err
	}
	list, ok := listValue.([]any)
	if !ok {
		return nil, runtimeErrorf(call.Line, "pop() requires list, got %s", typeNameOf(listValue))
	}
	if len(list) == 0 {
		return nil, runtimeErrorf(call.Line, "pop() from empty list")
	}
	removed := list[len(list)-1]
	list[len(list)-1] = nil
	list = list[:len(list)-1]
	if err := i.assignTarget(target, list, env, call.Line); err != nil {
		return nil, err
	}
	return removed, nil
}

func (i *Interpreter) evalRemoveAtCall(call *ast.FuncCall, env *Environment) (any, error) {
	if len(call.Args) != 2 {
		return nil, runtimeErrorf(call.Line, "removeat() expects 2 arguments, got %d", len(call.Args))
	}
	target, ok := assignmentTargetExpr(call.Args[0])
	if !ok {
		return nil, runtimeErrorf(call.Line, "removeat() first argument must be assignable list target")
	}
	listValue, err := i.evalExpr(call.Args[0], env)
	if err != nil {
		return nil, err
	}
	list, ok := listValue.([]any)
	if !ok {
		return nil, runtimeErrorf(call.Line, "removeat() requires list, got %s", typeNameOf(listValue))
	}
	index, err := i.evalExpr(call.Args[1], env)
	if err != nil {
		return nil, err
	}
	intIndex, ok := index.(int64)
	if !ok {
		return nil, runtimeErrorf(call.Line, "removeat() index must be int, got %s", typeNameOf(index))
	}
	if intIndex < 0 || intIndex >= int64(len(list)) {
		return nil, runtimeErrorf(call.Line, "removeat() index out of range: %d", intIndex)
	}
	removed := list[intIndex]
	list = append(list[:intIndex], list[intIndex+1:]...)
	if err := i.assignTarget(target, list, env, call.Line); err != nil {
		return nil, err
	}
	return removed, nil
}

func (i *Interpreter) evalInsertCall(call *ast.FuncCall, env *Environment) (any, error) {
	if len(call.Args) != 3 {
		return nil, runtimeErrorf(call.Line, "insert() expects 3 arguments, got %d", len(call.Args))
	}
	target, ok := assignmentTargetExpr(call.Args[0])
	if !ok {
		return nil, runtimeErrorf(call.Line, "insert() first argument must be assignable list target")
	}
	listValue, err := i.evalExpr(call.Args[0], env)
	if err != nil {
		return nil, err
	}
	list, ok := listValue.([]any)
	if !ok {
		return nil, runtimeErrorf(call.Line, "insert() requires list, got %s", typeNameOf(listValue))
	}
	indexValue, err := i.evalExpr(call.Args[1], env)
	if err != nil {
		return nil, err
	}
	index, ok := indexValue.(int64)
	if !ok {
		return nil, runtimeErrorf(call.Line, "insert() index must be int, got %s", typeNameOf(indexValue))
	}
	if index < 0 || index > int64(len(list)) {
		return nil, runtimeErrorf(call.Line, "insert() index out of range: %d", index)
	}
	item, err := i.evalExpr(call.Args[2], env)
	if err != nil {
		return nil, err
	}
	list = append(list, nil)
	copy(list[index+1:], list[index:])
	list[index] = item
	if err := i.assignTarget(target, list, env, call.Line); err != nil {
		return nil, err
	}
	return nil, nil
}

func (i *Interpreter) callFunction(fn *Function, args []any, line int) (any, error) {
	callEnv := NewEnvironment(fn.Closure)
	if err := bindParams(args, fn.Node.Params, callEnv, fn.Closure, line, fn.Node.Name, i); err != nil {
		return nil, err
	}
	result, err := i.execBlock(fn.Node.Body, callEnv)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.value, nil
}

func (i *Interpreter) callLambda(fn *Lambda, args []any, line int) (any, error) {
	callEnv := NewEnvironment(fn.Closure)
	if err := bindParams(args, fn.Node.Params, callEnv, fn.Closure, line, "<lambda>", i); err != nil {
		return nil, err
	}
	result, err := i.execBlock(fn.Node.Body, callEnv)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.value, nil
}

func (i *Interpreter) callMethod(method *BoundMethod, args []any, line int) (any, error) {
	callEnv := NewEnvironment(method.Object.Closure)
	callEnv.self = method.Instance
	if len(method.Method.Params) == 0 {
		return nil, runtimeErrorf(line, "Method '%s.%s' must declare 'self' as first parameter", method.Object.Name, method.Method.Name)
	}
	selfParam := method.Method.Params[0]
	callEnv.Set(selfParam.Name, method.Instance)
	callEnv.SetType(selfParam.Name, method.Object.Name)
	if err := bindParams(args, method.Method.Params[1:], callEnv, method.Object.Closure, line, method.Method.Name, i); err != nil {
		return nil, err
	}
	result, err := i.execBlock(method.Method.Body, callEnv)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.value, nil
}

func (i *Interpreter) callConstructor(constructor *ConstructorRef, args []any, line int) (any, error) {
	callEnv := NewEnvironment(constructor.Object.Closure)
	if constructor.Object.Constructor == nil {
		return nil, runtimeErrorf(line, "Object '%s' has no constructor", constructor.Object.Name)
	}
	if err := bindParams(args, constructor.Object.Constructor.Params, callEnv, constructor.Object.Closure, line, constructor.Object.Name+".new", i); err != nil {
		return nil, err
	}
	result, err := i.execBlock(constructor.Object.Constructor.Body, callEnv)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, runtimeErrorf(line, "Constructor '%s.new' did not return a value", constructor.Object.Name)
	}
	instance, ok := result.value.(*ObjectValue)
	if !ok || instance.TypeName != constructor.Object.Name {
		return nil, runtimeErrorf(line, "Constructor '%s.new' must return a '%s' instance", constructor.Object.Name, constructor.Object.Name)
	}
	return instance, nil
}

func (i *Interpreter) assignTarget(target any, value any, env *Environment, line int) error {
	switch target := target.(type) {
	case string:
		if env.IsConst(target) {
			return runtimeErrorf(line, "Cannot assign to const variable: %s", target)
		}
		coerced, err := coerceIfTyped(value, env.GetLocalType(target), line)
		if err != nil {
			return err
		}
		env.Update(target, coerced)
		return nil
	case *ast.MemberAccess:
		identifier, ok := target.Object.(*ast.Identifier)
		if !ok || identifier.Name != "self" {
			return runtimeErrorf(line, "Cannot assign to field '%s' from outside; use 'self.%s := ...' inside a method", target.Member, target.Member)
		}
		if env.self == nil {
			return runtimeErrorf(line, "Cannot assign to field '%s' from outside; use 'self.%s := ...' inside a method", target.Member, target.Member)
		}
		expectedType, ok := env.self.Object.FieldTypes[target.Member]
		if !ok {
			return runtimeErrorf(line, "Object '%s' has no field '%s'", env.self.TypeName, target.Member)
		}
		coerced, err := coerceIfTyped(value, expectedType, line)
		if err != nil {
			return err
		}
		env.self.Fields[target.Member] = coerced
		return nil
	case *ast.IndexAccess:
		object, err := i.evalExpr(target.Object, env)
		if err != nil {
			return err
		}
		index, err := i.evalExpr(target.Index, env)
		if err != nil {
			return err
		}
		return assignIndex(object, index, value, line)
	default:
		return runtimeErrorf(line, "invalid assignment target")
	}
}

func assignmentTargetExpr(expr any) (any, bool) {
	switch node := expr.(type) {
	case *ast.Identifier:
		return node.Name, true
	case *ast.IndexAccess:
		return node, true
	case *ast.MemberAccess:
		return node, true
	default:
		return nil, false
	}
}

func builtinCallName(expr any) (string, bool) {
	switch node := expr.(type) {
	case *ast.Identifier:
		return node.Name, true
	case *ast.MemberAccess:
		return node.Member, true
	default:
		return "", false
	}
}

func (i *Interpreter) matchPatterns(subject any, patterns []any, env *Environment) (bool, map[string]any, error) {
	for _, pattern := range patterns {
		matched, bindings, err := i.matchSingle(subject, pattern, env)
		if err != nil {
			return false, nil, err
		}
		if matched {
			return true, bindings, nil
		}
	}
	return false, nil, nil
}

func (i *Interpreter) matchSingle(subject any, pattern any, env *Environment) (bool, map[string]any, error) {
	switch pattern := pattern.(type) {
	case *ast.OkExpr:
		okValue, ok := subject.(*OkValue)
		if !ok {
			return false, nil, nil
		}
		if identifier, ok := pattern.Value.(*ast.Identifier); ok {
			return true, map[string]any{identifier.Name: okValue.Value}, nil
		}
		value, err := i.evalExpr(pattern.Value, env)
		if err != nil {
			return false, nil, err
		}
		return valuesEqual(okValue.Value, value), nil, nil
	case *ast.ErrExpr:
		errValue, ok := subject.(*ErrValue)
		if !ok {
			return false, nil, nil
		}
		if identifier, ok := pattern.Value.(*ast.Identifier); ok {
			return true, map[string]any{identifier.Name: errValue.Value}, nil
		}
		value, err := i.evalExpr(pattern.Value, env)
		if err != nil {
			return false, nil, err
		}
		return valuesEqual(errValue.Value, value), nil, nil
	case *ast.BinaryOp:
		if pattern.Op == "to" {
			start, err := i.evalExpr(pattern.Left, env)
			if err != nil {
				return false, nil, err
			}
			end, err := i.evalExpr(pattern.Right, env)
			if err != nil {
				return false, nil, err
			}
			startInt, startOK := start.(int64)
			endInt, endOK := end.(int64)
			subjectInt, subjectOK := subject.(int64)
			if !startOK || !endOK || !subjectOK {
				return false, nil, runtimeErrorf(pattern.Line, "range pattern requires int values")
			}
			return startInt <= subjectInt && subjectInt <= endInt, nil, nil
		}
	}
	value, err := i.evalExpr(pattern, env)
	if err != nil {
		return false, nil, err
	}
	return valuesEqual(subject, value), nil, nil
}

func (i *Interpreter) setupBuiltins() {
	i.GlobalEnv.Set("write", Builtin(func(args []any) (any, error) {
		parts := make([]string, 0, len(args))
		for _, arg := range args {
			parts = append(parts, formatDisplayValue(arg))
		}
		i.stdioMu.Lock()
		defer i.stdioMu.Unlock()
		if _, err := fmt.Fprintln(i.Stdout, strings.Join(parts, " ")); err != nil {
			return nil, err
		}
		return nil, nil
	}))
	i.GlobalEnv.Set("read", Builtin(func(args []any) (any, error) {
		if len(args) > 1 {
			return nil, runtimeErrorf(0, "read() expects 0 or 1 arguments, got %d", len(args))
		}
		if len(args) == 1 {
			prompt, ok := args[0].(string)
			if !ok {
				return nil, runtimeErrorf(0, "read() prompt must be string, got %s", typeNameOf(args[0]))
			}
			i.stdioMu.Lock()
			if _, err := fmt.Fprint(i.Stdout, prompt); err != nil {
				i.stdioMu.Unlock()
				return nil, err
			}
		} else {
			i.stdioMu.Lock()
		}
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		i.stdioMu.Unlock()
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}))
	i.GlobalEnv.Set("len", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "len() expects 1 argument, got %d", len(args))
		}
		switch value := args[0].(type) {
		case []any:
			return int64(len(value)), nil
		case string:
			return int64(len(value)), nil
		case map[any]any:
			return int64(len(value)), nil
		default:
			return nil, runtimeErrorf(0, "len() requires list, string, or dict, got %s", typeNameOf(args[0]))
		}
	}))
	i.GlobalEnv.Set("str", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "str() expects 1 argument, got %d", len(args))
		}
		return formatDisplayValue(args[0]), nil
	}))
	i.GlobalEnv.Set("int", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "int() expects 1 argument, got %d", len(args))
		}
		return toInt64(args[0], 0)
	}))
	i.GlobalEnv.Set("float", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "float() expects 1 argument, got %d", len(args))
		}
		switch value := args[0].(type) {
		case int64:
			return float64(value), nil
		case float64:
			return value, nil
		case string:
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, runtimeErrorf(0, "Cannot convert string to float: %q", value)
			}
			return parsed, nil
		default:
			return nil, runtimeErrorf(0, "Cannot convert %s to float", typeNameOf(args[0]))
		}
	}))
	i.GlobalEnv.Set("typeof", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "typeof() expects 1 argument, got %d", len(args))
		}
		return typeNameOf(args[0]), nil
	}))
	i.GlobalEnv.Set("range", Builtin(func(args []any) (any, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, runtimeErrorf(0, "range() expects 2 or 3 arguments, got %d", len(args))
		}
		start, err := toInt64(args[0], 0)
		if err != nil {
			return nil, err
		}
		end, err := toInt64(args[1], 0)
		if err != nil {
			return nil, err
		}
		var step int64
		if len(args) == 3 {
			step, err = toInt64(args[2], 0)
			if err != nil {
				return nil, err
			}
			if step == 0 {
				return nil, runtimeErrorf(0, "range() step cannot be 0")
			}
		} else if start <= end {
			step = 1
		} else {
			step = -1
		}

		items := make([]any, 0)
		for current := start; compareRange(current, end, step); current += step {
			items = append(items, current)
		}
		return items, nil
	}))
	i.GlobalEnv.Set("enumerate", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "enumerate() expects 1 argument, got %d", len(args))
		}
		items, err := toSlice(args[0], 0)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(items))
		for idx, item := range items {
			result = append(result, []any{int64(idx), item})
		}
		return result, nil
	}))
	i.GlobalEnv.Set("map", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "map() expects 2 arguments, got %d", len(args))
		}
		items, err := toSlice(args[0], 0)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(items))
		for _, item := range items {
			mapped, err := i.callCallable(args[1], []any{item}, 0)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		}
		return result, nil
	}))
	i.GlobalEnv.Set("filter", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "filter() expects 2 arguments, got %d", len(args))
		}
		items, err := toSlice(args[0], 0)
		if err != nil {
			return nil, err
		}
		result := make([]any, 0, len(items))
		for _, item := range items {
			keep, err := i.callCallable(args[1], []any{item}, 0)
			if err != nil {
				return nil, err
			}
			if err := requireBool(keep, "filter() predicate", 0); err != nil {
				return nil, err
			}
			if keep.(bool) {
				result = append(result, item)
			}
		}
		return result, nil
	}))
	i.GlobalEnv.Set("append", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "append() expects 2 arguments, got %d", len(args))
		}
		list, ok := args[0].([]any)
		if !ok {
			return nil, runtimeErrorf(0, "append() requires list, got %s", typeNameOf(args[0]))
		}
		list = append(list, args[1])
		return list, nil
	}))
	i.GlobalEnv.Set("pop", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "pop() expects 1 argument, got %d", len(args))
		}
		list, ok := args[0].([]any)
		if !ok {
			return nil, runtimeErrorf(0, "pop() requires list, got %s", typeNameOf(args[0]))
		}
		if len(list) == 0 {
			return nil, runtimeErrorf(0, "pop() from empty list")
		}
		return list[len(list)-1], nil
	}))
	i.GlobalEnv.Set("concat", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "concat() expects 2 arguments, got %d", len(args))
		}
		left, ok := args[0].([]any)
		if !ok {
			return nil, runtimeErrorf(0, "concat() requires list as first argument, got %s", typeNameOf(args[0]))
		}
		right, ok := args[1].([]any)
		if !ok {
			return nil, runtimeErrorf(0, "concat() requires list as second argument, got %s", typeNameOf(args[1]))
		}
		result := append([]any{}, left...)
		result = append(result, right...)
		return result, nil
	}))
	i.GlobalEnv.Set("removeat", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "removeat() expects 2 arguments, got %d", len(args))
		}
		list, ok := args[0].([]any)
		if !ok {
			return nil, runtimeErrorf(0, "removeat() requires list, got %s", typeNameOf(args[0]))
		}
		index, err := toInt64(args[1], 0)
		if err != nil {
			return nil, err
		}
		if index < 0 || index >= int64(len(list)) {
			return nil, runtimeErrorf(0, "removeat() index out of range: %d", index)
		}
		removed := list[index]
		copy(list[index:], list[index+1:])
		list[len(list)-1] = nil
		list = list[:len(list)-1]
		return removed, nil
	}))
	i.GlobalEnv.Set("insert", Builtin(func(args []any) (any, error) {
		if len(args) != 3 {
			return nil, runtimeErrorf(0, "insert() expects 3 arguments, got %d", len(args))
		}
		list, ok := args[0].([]any)
		if !ok {
			return nil, runtimeErrorf(0, "insert() requires list, got %s", typeNameOf(args[0]))
		}
		index, err := toInt64(args[1], 0)
		if err != nil {
			return nil, err
		}
		if index < 0 || index > int64(len(list)) {
			return nil, runtimeErrorf(0, "insert() index out of range: %d", index)
		}
		list = append(list, nil)
		copy(list[index+1:], list[index:])
		list[index] = args[2]
		return list, nil
	}))
	i.GlobalEnv.Set("asc", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "asc() expects 2 arguments, got %d", len(args))
		}
		return compareValues("<", args[0], args[1], 0)
	}))
	i.GlobalEnv.Set("desc", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "desc() expects 2 arguments, got %d", len(args))
		}
		return compareValues(">", args[0], args[1], 0)
	}))
	i.GlobalEnv.Set("reversed", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "reversed() expects 1 argument, got %d", len(args))
		}
		items, ok := args[0].([]any)
		if !ok {
			return nil, runtimeErrorf(0, "reversed() requires list, got %s", typeNameOf(args[0]))
		}
		result := make([]any, 0, len(items))
		for idx := len(items) - 1; idx >= 0; idx-- {
			result = append(result, items[idx])
		}
		return result, nil
	}))
	i.GlobalEnv.Set("sort", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "sort() expects 2 arguments, got %d", len(args))
		}
		items, ok := args[0].([]any)
		if !ok {
			return nil, runtimeErrorf(0, "sort() requires list, got %s", typeNameOf(args[0]))
		}
		result := append([]any{}, items...)
		for leftIdx := 0; leftIdx < len(result); leftIdx++ {
			for rightIdx := leftIdx + 1; rightIdx < len(result); rightIdx++ {
				less, err := i.callCallable(args[1], []any{result[rightIdx], result[leftIdx]}, 0)
				if err != nil {
					return nil, err
				}
				if err := requireBool(less, "sort() comparator", 0); err != nil {
					return nil, err
				}
				if less.(bool) {
					result[leftIdx], result[rightIdx] = result[rightIdx], result[leftIdx]
				}
			}
		}
		return result, nil
	}))
	i.GlobalEnv.Set("split", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "split() expects 2 arguments, got %d", len(args))
		}
		value, ok := args[0].(string)
		if !ok {
			return nil, runtimeErrorf(0, "split() requires string input, got %s", typeNameOf(args[0]))
		}
		sep, ok := args[1].(string)
		if !ok {
			return nil, runtimeErrorf(0, "split() separator must be string, got %s", typeNameOf(args[1]))
		}
		parts := make([]any, 0)
		if sep == "" {
			for _, ch := range value {
				parts = append(parts, string(ch))
			}
			return parts, nil
		}
		for _, part := range strings.Split(value, sep) {
			parts = append(parts, part)
		}
		return parts, nil
	}))
	i.GlobalEnv.Set("join", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "join() expects 2 arguments, got %d", len(args))
		}
		items, err := toSlice(args[0], 0)
		if err != nil {
			return nil, err
		}
		sep, ok := args[1].(string)
		if !ok {
			return nil, runtimeErrorf(0, "join() separator must be string, got %s", typeNameOf(args[1]))
		}
		parts := make([]string, 0, len(items))
		for _, item := range items {
			parts = append(parts, formatDisplayValue(item))
		}
		return strings.Join(parts, sep), nil
	}))
	i.GlobalEnv.Set("substring", Builtin(func(args []any) (any, error) {
		if len(args) != 3 {
			return nil, runtimeErrorf(0, "substring() expects 3 arguments, got %d", len(args))
		}
		value, ok := args[0].(string)
		if !ok {
			return nil, runtimeErrorf(0, "substring() requires string input, got %s", typeNameOf(args[0]))
		}
		start, err := toInt64(args[1], 0)
		if err != nil {
			return nil, err
		}
		end, err := toInt64(args[2], 0)
		if err != nil {
			return nil, err
		}
		if start < 0 || end < start || end >= int64(len(value)) {
			return nil, runtimeErrorf(0, "substring() bounds out of range")
		}
		return value[start : end+1], nil
	}))
	i.GlobalEnv.Set("contains", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "contains() expects 2 arguments, got %d", len(args))
		}
		value, ok := args[0].(string)
		if !ok {
			return nil, runtimeErrorf(0, "contains() requires string input, got %s", typeNameOf(args[0]))
		}
		substr, ok := args[1].(string)
		if !ok {
			return nil, runtimeErrorf(0, "contains() requires string substring, got %s", typeNameOf(args[1]))
		}
		return strings.Contains(value, substr), nil
	}))
	i.GlobalEnv.Set("trim", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "trim() expects 1 argument, got %d", len(args))
		}
		value, ok := args[0].(string)
		if !ok {
			return nil, runtimeErrorf(0, "trim() requires string input, got %s", typeNameOf(args[0]))
		}
		return strings.TrimSpace(value), nil
	}))
	i.GlobalEnv.Set("replace", Builtin(func(args []any) (any, error) {
		if len(args) != 3 {
			return nil, runtimeErrorf(0, "replace() expects 3 arguments, got %d", len(args))
		}
		value, ok := args[0].(string)
		if !ok {
			return nil, runtimeErrorf(0, "replace() requires string input, got %s", typeNameOf(args[0]))
		}
		oldValue, ok := args[1].(string)
		if !ok {
			return nil, runtimeErrorf(0, "replace() old must be string, got %s", typeNameOf(args[1]))
		}
		newValue, ok := args[2].(string)
		if !ok {
			return nil, runtimeErrorf(0, "replace() new must be string, got %s", typeNameOf(args[2]))
		}
		return strings.ReplaceAll(value, oldValue, newValue), nil
	}))
	i.GlobalEnv.Set("abs", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "abs() expects 1 argument, got %d", len(args))
		}
		switch value := args[0].(type) {
		case int64:
			if value < 0 {
				return -value, nil
			}
			return value, nil
		case float64:
			return math.Abs(value), nil
		default:
			return nil, runtimeErrorf(0, "abs() requires int or float, got %s", typeNameOf(args[0]))
		}
	}))
	i.GlobalEnv.Set("min", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "min() expects 2 arguments, got %d", len(args))
		}
		less, err := compareValues("<", args[0], args[1], 0)
		if err != nil {
			return nil, err
		}
		if less {
			return args[0], nil
		}
		return args[1], nil
	}))
	i.GlobalEnv.Set("max", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "max() expects 2 arguments, got %d", len(args))
		}
		greater, err := compareValues(">", args[0], args[1], 0)
		if err != nil {
			return nil, err
		}
		if greater {
			return args[0], nil
		}
		return args[1], nil
	}))
	i.GlobalEnv.Set("sqrt", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "sqrt() expects 1 argument, got %d", len(args))
		}
		value, ok := args[0].(float64)
		if !ok {
			return nil, runtimeErrorf(0, "sqrt() requires float, got %s", typeNameOf(args[0]))
		}
		return math.Sqrt(value), nil
	}))
	i.GlobalEnv.Set("floor", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "floor() expects 1 argument, got %d", len(args))
		}
		value, ok := args[0].(float64)
		if !ok {
			return nil, runtimeErrorf(0, "floor() requires float, got %s", typeNameOf(args[0]))
		}
		return math.Floor(value), nil
	}))
	i.GlobalEnv.Set("ceil", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "ceil() expects 1 argument, got %d", len(args))
		}
		value, ok := args[0].(float64)
		if !ok {
			return nil, runtimeErrorf(0, "ceil() requires float, got %s", typeNameOf(args[0]))
		}
		return math.Ceil(value), nil
	}))
	i.GlobalEnv.Set("haskey", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "haskey() expects 2 arguments, got %d", len(args))
		}
		dict, ok := args[0].(map[any]any)
		if !ok {
			return nil, runtimeErrorf(0, "haskey() requires dict, got %s", typeNameOf(args[0]))
		}
		_, exists := dict[args[1]]
		return exists, nil
	}))
	i.GlobalEnv.Set("get", Builtin(func(args []any) (any, error) {
		if len(args) != 3 {
			return nil, runtimeErrorf(0, "get() expects 3 arguments, got %d", len(args))
		}
		dict, ok := args[0].(map[any]any)
		if !ok {
			return nil, runtimeErrorf(0, "get() requires dict, got %s", typeNameOf(args[0]))
		}
		if value, ok := dict[args[1]]; ok {
			return value, nil
		}
		return args[2], nil
	}))
	i.GlobalEnv.Set("keys", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "keys() expects 1 argument, got %d", len(args))
		}
		dict, ok := args[0].(map[any]any)
		if !ok {
			return nil, runtimeErrorf(0, "keys() requires dict, got %s", typeNameOf(args[0]))
		}
		result := make([]any, 0, len(dict))
		for key := range dict {
			result = append(result, key)
		}
		return result, nil
	}))
	i.GlobalEnv.Set("values", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "values() expects 1 argument, got %d", len(args))
		}
		dict, ok := args[0].(map[any]any)
		if !ok {
			return nil, runtimeErrorf(0, "values() requires dict, got %s", typeNameOf(args[0]))
		}
		result := make([]any, 0, len(dict))
		for _, value := range dict {
			result = append(result, value)
		}
		return result, nil
	}))
	i.GlobalEnv.Set("items", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "items() expects 1 argument, got %d", len(args))
		}
		dict, ok := args[0].(map[any]any)
		if !ok {
			return nil, runtimeErrorf(0, "items() requires dict, got %s", typeNameOf(args[0]))
		}
		result := make([]any, 0, len(dict))
		for key, value := range dict {
			result = append(result, []any{key, value})
		}
		return result, nil
	}))
	i.GlobalEnv.Set("readfile", Builtin(func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, runtimeErrorf(0, "readfile() expects 1 argument, got %d", len(args))
		}
		path, ok := args[0].(string)
		if !ok {
			return nil, runtimeErrorf(0, "readfile() requires string path, got %s", typeNameOf(args[0]))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return &ErrValue{Value: err.Error()}, nil
		}
		return &OkValue{Value: string(data)}, nil
	}))
	i.GlobalEnv.Set("writefile", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "writefile() expects 2 arguments, got %d", len(args))
		}
		path, ok := args[0].(string)
		if !ok {
			return nil, runtimeErrorf(0, "writefile() requires string path, got %s", typeNameOf(args[0]))
		}
		content, ok := args[1].(string)
		if !ok {
			return nil, runtimeErrorf(0, "writefile() requires string content, got %s", typeNameOf(args[1]))
		}
		data := []byte(content)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return &ErrValue{Value: err.Error()}, nil
		}
		return &OkValue{Value: int64(len(data))}, nil
	}))
	i.GlobalEnv.Set("appendfile", Builtin(func(args []any) (any, error) {
		if len(args) != 2 {
			return nil, runtimeErrorf(0, "appendfile() expects 2 arguments, got %d", len(args))
		}
		path, ok := args[0].(string)
		if !ok {
			return nil, runtimeErrorf(0, "appendfile() requires string path, got %s", typeNameOf(args[0]))
		}
		content, ok := args[1].(string)
		if !ok {
			return nil, runtimeErrorf(0, "appendfile() requires string content, got %s", typeNameOf(args[1]))
		}
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return &ErrValue{Value: err.Error()}, nil
		}
		defer file.Close()
		written, err := file.WriteString(content)
		if err != nil {
			return &ErrValue{Value: err.Error()}, nil
		}
		return &OkValue{Value: int64(written)}, nil
	}))
}

func (i *Interpreter) setupStdlibModules() {
	for moduleName, names := range officialStdlibModules {
		moduleEnv := NewEnvironment(nil)
		for _, name := range names {
			value, err := i.GlobalEnv.Get(name)
			if err != nil {
				continue
			}
			moduleEnv.Set(name, value)
		}
		i.Modules[moduleName] = moduleEnv
	}
}

func (i *Interpreter) callCallable(callee any, args []any, line int) (any, error) {
	switch fn := callee.(type) {
	case Builtin:
		return fn(args)
	case *Function:
		return i.callFunction(fn, args, line)
	case *Lambda:
		return i.callLambda(fn, args, line)
	case *BoundMethod:
		return i.callMethod(fn, args, line)
	default:
		return nil, runtimeErrorf(line, "%s is not callable", formatDisplayValue(callee))
	}
}

func (i *Interpreter) moduleCandidatePaths(moduleName string) []string {
	searchPaths := i.moduleSearchPaths
	if len(searchPaths) == 0 {
		searchPaths = []string{"."}
	}
	seen := map[string]struct{}{}
	paths := make([]string, 0, len(searchPaths)*2)
	for _, base := range searchPaths {
		candidates := []string{
			filepath.Join(base, moduleName+".gw"),
			filepath.Join(base, moduleName, "main.gw"),
		}
		for _, candidate := range candidates {
			absPath, err := filepath.Abs(candidate)
			if err != nil {
				continue
			}
			if _, ok := seen[absPath]; ok {
				continue
			}
			seen[absPath] = struct{}{}
			paths = append(paths, absPath)
		}
	}
	return paths
}

func (i *Interpreter) loadModuleFromFile(moduleName string, line int) error {
	if _, ok := i.Modules[moduleName]; ok {
		return nil
	}
	if _, loading := i.loadingModules[moduleName]; loading {
		return runtimeErrorf(line, "Cyclic module import detected while loading '%s'", moduleName)
	}

	for _, candidate := range i.moduleCandidatePaths(moduleName) {
		source, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		program, err := parser.Parse(string(source))
		if err != nil {
			return err
		}
		if len(program.Statements) != 1 {
			return runtimeErrorf(line, "Module file '%s' must contain exactly one top-level module definition for '%s'", candidate, moduleName)
		}
		moduleDef, ok := program.Statements[0].(*ast.ModuleDef)
		if !ok || moduleDef.Name != moduleName {
			return runtimeErrorf(line, "Module file '%s' must contain exactly one top-level module definition for '%s'", candidate, moduleName)
		}

		i.loadingModules[moduleName] = struct{}{}
		i.AddModuleSearchPath(filepath.Dir(candidate))
		_, execErr := i.execStmt(moduleDef, i.GlobalEnv)
		delete(i.loadingModules, moduleName)
		return execErr
	}

	return runtimeErrorf(line, "Module not found: %s", moduleName)
}

func bindParams(args []any, params []*ast.Param, callEnv, defaultEnv *Environment, line int, label string, interp *Interpreter) error {
	required := 0
	for _, param := range params {
		if param.Default == nil {
			required++
		}
	}
	if len(args) < required {
		for idx, param := range params {
			if idx >= len(args) && param.Default == nil {
				return runtimeErrorf(line, "Missing argument: %s", param.Name)
			}
		}
	}
	if len(args) > len(params) {
		return runtimeErrorf(line, "Too many arguments for '%s': expected at most %d, got %d", label, len(params), len(args))
	}

	for idx, param := range params {
		typeName, err := resolveRuntimeType(param.TypeName, callEnv, param.Line)
		if err != nil {
			return err
		}
		var value any
		switch {
		case idx < len(args):
			value = args[idx]
		case param.Default != nil:
			value, err = interp.evalExpr(param.Default, defaultEnv)
			if err != nil {
				return err
			}
		default:
			return runtimeErrorf(line, "Missing argument: %s", param.Name)
		}
		value, err = coerceIfTyped(value, typeName, param.Line)
		if err != nil {
			return err
		}
		callEnv.Set(param.Name, value)
		callEnv.SetType(param.Name, typeName)
	}
	return nil
}

func resolveRuntimeType(typeNode any, env *Environment, line int) (string, error) {
	switch node := typeNode.(type) {
	case nil:
		return "", nil
	case *ast.TypeName:
		return resolveAlias(node.Name, env, line)
	case *ast.GenericType:
		if node.Base == "money" && len(node.Params) == 1 {
			switch param := node.Params[0].(type) {
			case *ast.TypeName:
				return "money[" + param.Name + "]", nil
			default:
				inner, err := resolveRuntimeType(node.Params[0], env, line)
				if err != nil {
					return "", err
				}
				return "money[" + inner + "]", nil
			}
		}
		_, err := resolveAlias(node.Base, env, line)
		if err != nil {
			return "", err
		}
		return node.Base, nil
	case *ast.FuncType:
		return "func", nil
	default:
		return "", runtimeErrorf(line, "Invalid type annotation")
	}
}

func resolveAlias(typeName string, env *Environment, line int) (string, error) {
	if typeName == "" {
		return "", nil
	}
	seen := map[string]struct{}{}
	current := typeName
	for {
		if _, ok := seen[current]; ok {
			break
		}
		seen[current] = struct{}{}
		next, ok := env.GetAlias(current)
		if !ok {
			break
		}
		current = next
	}
	if !isKnownType(current) {
		if value, err := env.Get(current); err == nil {
			if _, ok := value.(*ObjectType); ok {
				return current, nil
			}
		}
		return "", runtimeErrorf(line, "Unknown type: %s", current)
	}
	return current, nil
}

func isKnownType(typeName string) bool {
	if typeName == "" {
		return true
	}
	switch typeName {
	case "int", "float", "string", "bool", "list", "dict", "func", "result", "float32", "float64":
		return true
	}
	if strings.HasPrefix(typeName, "money[") && strings.HasSuffix(typeName, "]") {
		return true
	}
	_, ok := intRanges[typeName]
	return ok
}

func coerceIfTyped(value any, typeName string, line int) (any, error) {
	if typeName == "" || value == uninitializedValue {
		return value, nil
	}
	switch typeName {
	case "int":
		if _, ok := value.(int64); ok {
			return value, nil
		}
		return nil, runtimeErrorf(line, "Type mismatch: expected int, got %s", typeNameOf(value))
	case "float":
		switch value := value.(type) {
		case int64:
			return float64(value), nil
		case float64:
			return value, nil
		default:
			return nil, runtimeErrorf(line, "Type mismatch: expected float, got %s", typeNameOf(value))
		}
	case "float32", "float64":
		switch value := value.(type) {
		case int64:
			return float64(value), nil
		case float64:
			return value, nil
		default:
			return nil, runtimeErrorf(line, "Type mismatch: expected %s, got %s", typeName, typeNameOf(value))
		}
	case "string":
		if _, ok := value.(string); ok {
			return value, nil
		}
		return nil, runtimeErrorf(line, "Type mismatch: expected string, got %s", typeNameOf(value))
	case "bool":
		if _, ok := value.(bool); ok {
			return value, nil
		}
		return nil, runtimeErrorf(line, "Type mismatch: expected bool, got %s", typeNameOf(value))
	case "list", "dict", "func", "result":
		return value, nil
	default:
		if objectValue, ok := value.(*ObjectValue); ok && objectValue.TypeName == typeName {
			return value, nil
		}
		if isMoneyType(typeName) {
			return coerceMoney(value, typeName, line)
		}
		if _, ok := intRanges[typeName]; ok {
			return coerceIntRange(value, typeName, line)
		}
		return nil, runtimeErrorf(line, "Unknown type: %s", typeName)
	}
}

func coerceIntRange(value any, typeName string, line int) (any, error) {
	intValue, err := toInt64(value, line)
	if err != nil {
		return nil, runtimeErrorf(line, "Type mismatch: expected %s, got %s", typeName, typeNameOf(value))
	}
	info := intRanges[typeName]
	if info.min == 0 {
		if intValue < 0 || uint64(intValue) > info.max {
			return nil, runtimeErrorf(line, "Overflow: %d out of range for %s", intValue, typeName)
		}
		return intValue, nil
	}
	if intValue < info.min || intValue > int64(info.max) {
		return nil, runtimeErrorf(line, "Overflow: %d out of range for %s", intValue, typeName)
	}
	return intValue, nil
}

func zeroValue(typeName string, line int) (any, error) {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32":
		return int64(0), nil
	case "float", "float32", "float64":
		return float64(0), nil
	case "string":
		return "", nil
	case "bool":
		return false, nil
	case "list":
		return []any{}, nil
	case "dict":
		return map[any]any{}, nil
	default:
		if isMoneyType(typeName) {
			return &MoneyValue{Raw: 0, Currency: moneyCurrency(typeName)}, nil
		}
		return nil, runtimeErrorf(line, "No default zero value for type '%s'", typeName)
	}
}

func evalBinary(op string, left, right any, line int) (any, error) {
	if isMoneyValue(left) || isMoneyValue(right) {
		return evalMoneyBinary(op, left, right, line)
	}
	switch op {
	case "+":
		switch {
		case isNumber(left) && isNumber(right):
			return addNumbers(left, right), nil
		case isString(left) && isString(right):
			return left.(string) + right.(string), nil
		default:
			return nil, runtimeErrorf(line, "operator + is not defined for %s and %s", typeNameOf(left), typeNameOf(right))
		}
	case "-":
		return arithmeticNumbers(left, right, line, func(a, b float64) float64 { return a - b }, func(a, b int64) int64 { return a - b })
	case "*":
		return arithmeticNumbers(left, right, line, func(a, b float64) float64 { return a * b }, func(a, b int64) int64 { return a * b })
	case "/":
		if isZero(right) {
			return nil, runtimeErrorf(line, "Division by zero")
		}
		if leftInt, ok := left.(int64); ok {
			if rightInt, ok := right.(int64); ok {
				return leftInt / rightInt, nil
			}
		}
		leftFloat, rightFloat, ok := promoteNumbers(left, right)
		if !ok {
			return nil, runtimeErrorf(line, "operator / is not defined for %s and %s", typeNameOf(left), typeNameOf(right))
		}
		return leftFloat / rightFloat, nil
	case "mod":
		leftInt, leftOK := left.(int64)
		rightInt, rightOK := right.(int64)
		if !leftOK || !rightOK {
			return nil, runtimeErrorf(line, "operator mod requires int operands")
		}
		if rightInt == 0 {
			return nil, runtimeErrorf(line, "Division by zero")
		}
		return leftInt % rightInt, nil
	case "^":
		leftFloat, rightFloat, ok := promoteNumbers(left, right)
		if !ok {
			return nil, runtimeErrorf(line, "operator ^ is not defined for %s and %s", typeNameOf(left), typeNameOf(right))
		}
		if leftInt, okLeft := left.(int64); okLeft {
			if rightInt, okRight := right.(int64); okRight && rightInt >= 0 {
				result := int64(1)
				for count := int64(0); count < rightInt; count++ {
					result *= leftInt
				}
				return result, nil
			}
		}
		return math.Pow(leftFloat, rightFloat), nil
	case "=":
		return valuesEqual(left, right), nil
	case "!=":
		return !valuesEqual(left, right), nil
	case "<", ">", "<=", ">=":
		return compareValues(op, left, right, line)
	default:
		return nil, runtimeErrorf(line, "Unknown operator: %s", op)
	}
}

func arithmeticNumbers(left, right any, line int, floatOp func(float64, float64) float64, intOp func(int64, int64) int64) (any, error) {
	if leftInt, ok := left.(int64); ok {
		if rightInt, ok := right.(int64); ok {
			return intOp(leftInt, rightInt), nil
		}
	}
	leftFloat, rightFloat, ok := promoteNumbers(left, right)
	if !ok {
		return nil, runtimeErrorf(line, "operator is not defined for %s and %s", typeNameOf(left), typeNameOf(right))
	}
	return floatOp(leftFloat, rightFloat), nil
}

func compareValues(op string, left, right any, line int) (bool, error) {
	if leftFloat, rightFloat, ok := promoteNumbers(left, right); ok {
		switch op {
		case "<":
			return leftFloat < rightFloat, nil
		case ">":
			return leftFloat > rightFloat, nil
		case "<=":
			return leftFloat <= rightFloat, nil
		case ">=":
			return leftFloat >= rightFloat, nil
		}
	}
	switch left := left.(type) {
	case string:
		rightString, ok := right.(string)
		if !ok {
			return false, runtimeErrorf(line, "comparison '%s' is not defined for %s and %s", op, typeNameOf(left), typeNameOf(right))
		}
		switch op {
		case "<":
			return left < rightString, nil
		case ">":
			return left > rightString, nil
		case "<=":
			return left <= rightString, nil
		case ">=":
			return left >= rightString, nil
		}
	case bool:
		rightBool, ok := right.(bool)
		if !ok {
			return false, runtimeErrorf(line, "comparison '%s' is not defined for %s and %s", op, typeNameOf(left), typeNameOf(right))
		}
		leftInt := 0
		if left {
			leftInt = 1
		}
		rightInt := 0
		if rightBool {
			rightInt = 1
		}
		switch op {
		case "<":
			return leftInt < rightInt, nil
		case ">":
			return leftInt > rightInt, nil
		case "<=":
			return leftInt <= rightInt, nil
		case ">=":
			return leftInt >= rightInt, nil
		}
	}
	return false, runtimeErrorf(line, "comparison '%s' is not defined for %s and %s", op, typeNameOf(left), typeNameOf(right))
}

func indexValue(object, index any, line int) (any, error) {
	switch object := object.(type) {
	case []any:
		intIndex, ok := index.(int64)
		if !ok {
			return nil, runtimeErrorf(line, "List index must be int, got %s", typeNameOf(index))
		}
		if intIndex < 0 || intIndex >= int64(len(object)) {
			return nil, runtimeErrorf(line, "Index out of range: %d", intIndex)
		}
		return object[intIndex], nil
	case string:
		intIndex, ok := index.(int64)
		if !ok {
			return nil, runtimeErrorf(line, "String index must be int, got %s", typeNameOf(index))
		}
		if intIndex < 0 || intIndex >= int64(len(object)) {
			return nil, runtimeErrorf(line, "Index out of range: %d", intIndex)
		}
		return string(object[intIndex]), nil
	case map[any]any:
		value, ok := object[index]
		if !ok {
			return nil, runtimeErrorf(line, "Key not found: %v", index)
		}
		return value, nil
	default:
		return nil, runtimeErrorf(line, "Cannot index type %s", typeNameOf(object))
	}
}

func assignIndex(object, index, value any, line int) error {
	switch object := object.(type) {
	case []any:
		intIndex, ok := index.(int64)
		if !ok {
			return runtimeErrorf(line, "List index must be int, got %s", typeNameOf(index))
		}
		if intIndex < 0 || intIndex >= int64(len(object)) {
			return runtimeErrorf(line, "Index out of range: %d", intIndex)
		}
		object[intIndex] = value
		return nil
	case map[any]any:
		object[index] = value
		return nil
	default:
		return runtimeErrorf(line, "Cannot assign into type %s", typeNameOf(object))
	}
}

func toSlice(value any, line int) ([]any, error) {
	switch value := value.(type) {
	case []any:
		return value, nil
	case string:
		items := make([]any, 0, len(value))
		for _, ch := range value {
			items = append(items, string(ch))
		}
		return items, nil
	default:
		return nil, runtimeErrorf(line, "Cannot iterate over %s", typeNameOf(value))
	}
}

func compareRange(current, end, step int64) bool {
	if step > 0 {
		return current <= end
	}
	return current >= end
}

func requireBool(value any, context string, line int) error {
	if _, ok := value.(bool); !ok {
		return runtimeErrorf(line, "%s must be bool, got %s", context, typeNameOf(value))
	}
	return nil
}

func runtimeErrorf(line int, format string, args ...any) error {
	return &RuntimeError{
		Message: fmt.Sprintf(format, args...),
		Line:    line,
	}
}

func typeNameOf(value any) string {
	switch value := value.(type) {
	case int64:
		return "int"
	case float64:
		return "float"
	case string:
		return "string"
	case bool:
		return "bool"
	case []any:
		return "list"
	case map[any]any:
		return "dict"
	case *OkValue:
		return "ok"
	case *ErrValue:
		return "err"
	case *MoneyValue:
		return "money[" + value.Currency + "]"
	case Builtin, *Function, *Lambda:
		return "func"
	case *Environment:
		return "module"
	case *ObjectType:
		return "object"
	case *ObjectValue:
		return value.TypeName
	case *BoundMethod, *StaticMethodRef, *ConstructorRef:
		return "func"
	case nil:
		return "nil"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func formatValue(value any) string {
	switch value := value.(type) {
	case nil:
		return "None"
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return formatFloat(value)
	case string:
		return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
	case bool:
		if value {
			return "True"
		}
		return "False"
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			parts = append(parts, formatValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[any]any:
		parts := make([]string, 0, len(value))
		for key, item := range value {
			parts = append(parts, fmt.Sprintf("%s: %s", formatValue(key), formatValue(item)))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *OkValue:
		return value.String()
	case *ErrValue:
		return value.String()
	case *MoneyValue:
		return formatMoney(value)
	case *Function:
		return "<func " + value.Node.Name + ">"
	case *Lambda:
		return "<lambda>"
	case *ObjectType:
		return "<object " + value.Name + ">"
	case *ObjectValue:
		return "<" + value.TypeName + ">"
	default:
		return fmt.Sprint(value)
	}
}

func formatDisplayValue(value any) string {
	if str, ok := value.(string); ok {
		return str
	}
	return formatValue(value)
}

func formatFloat(value float64) string {
	text := strconv.FormatFloat(value, 'f', -1, 64)
	if !strings.ContainsAny(text, ".eE") {
		return text + ".0"
	}
	return text
}

func isString(value any) bool {
	_, ok := value.(string)
	return ok
}

func isNumber(value any) bool {
	switch value.(type) {
	case int64, float64:
		return true
	default:
		return false
	}
}

func promoteNumbers(left, right any) (float64, float64, bool) {
	switch left := left.(type) {
	case int64:
		switch right := right.(type) {
		case int64:
			return float64(left), float64(right), true
		case float64:
			return float64(left), right, true
		}
	case float64:
		switch right := right.(type) {
		case int64:
			return left, float64(right), true
		case float64:
			return left, right, true
		}
	}
	return 0, 0, false
}

func addNumbers(left, right any) any {
	if leftInt, ok := left.(int64); ok {
		if rightInt, ok := right.(int64); ok {
			return leftInt + rightInt
		}
	}
	leftFloat, rightFloat, _ := promoteNumbers(left, right)
	return leftFloat + rightFloat
}

func isZero(value any) bool {
	switch value := value.(type) {
	case int64:
		return value == 0
	case float64:
		return value == 0
	default:
		return false
	}
}

func valuesEqual(left, right any) bool {
	leftMoney, leftIsMoney := left.(*MoneyValue)
	rightMoney, rightIsMoney := right.(*MoneyValue)
	if leftIsMoney || rightIsMoney {
		return leftIsMoney && rightIsMoney && leftMoney.Raw == rightMoney.Raw && leftMoney.Currency == rightMoney.Currency
	}
	if leftFloat, rightFloat, ok := promoteNumbers(left, right); ok {
		return leftFloat == rightFloat
	}
	return reflect.DeepEqual(left, right)
}

func toInt64(value any, line int) (int64, error) {
	switch value := value.(type) {
	case int64:
		return value, nil
	case float64:
		return int64(value), nil
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, runtimeErrorf(line, "Cannot convert string to int: %q", value)
		}
		return parsed, nil
	default:
		return 0, runtimeErrorf(line, "Cannot convert %s to int", typeNameOf(value))
	}
}

func isMoneyType(typeName string) bool {
	return strings.HasPrefix(typeName, "money[") && strings.HasSuffix(typeName, "]")
}

func moneyCurrency(typeName string) string {
	return strings.TrimSuffix(strings.TrimPrefix(typeName, "money["), "]")
}

func isMoneyValue(value any) bool {
	_, ok := value.(*MoneyValue)
	return ok
}

func coerceMoney(value any, typeName string, line int) (any, error) {
	currency := moneyCurrency(typeName)
	switch value := value.(type) {
	case *MoneyValue:
		if value.Currency != currency {
			return nil, runtimeErrorf(line, "Currency mismatch: cannot assign money[%s] to money[%s]", value.Currency, currency)
		}
		return value, nil
	case int64:
		return &MoneyValue{Raw: value * moneyScale, Currency: currency}, nil
	case float64:
		return &MoneyValue{Raw: int64(math.Round(value * float64(moneyScale))), Currency: currency}, nil
	default:
		return nil, runtimeErrorf(line, "Cannot convert %s to money[%s]", typeNameOf(value), currency)
	}
}

func convertAs(value any, target string, line int) (any, error) {
	if moneyValue, ok := value.(*MoneyValue); ok {
		switch {
		case target == "float" || target == "float32" || target == "float64":
			return float64(moneyValue.Raw) / float64(moneyScale), nil
		case target == "int":
			return moneyValue.Raw / moneyScale, nil
		case isMoneyType(target):
			if moneyValue.Currency != moneyCurrency(target) {
				return nil, runtimeErrorf(line, "Cannot convert money[%s] to money[%s] (explicit exchange rate required)", moneyValue.Currency, moneyCurrency(target))
			}
			return moneyValue, nil
		}
	}
	return coerceIfTyped(value, target, line)
}

func evalMoneyBinary(op string, left, right any, line int) (any, error) {
	leftMoney, leftIsMoney := left.(*MoneyValue)
	rightMoney, rightIsMoney := right.(*MoneyValue)

	switch op {
	case "+", "-":
		if !leftIsMoney || !rightIsMoney {
			return nil, runtimeErrorf(line, "Cannot %s money with non-money value", op)
		}
		if leftMoney.Currency != rightMoney.Currency {
			return nil, runtimeErrorf(line, "Currency mismatch: money[%s] %s money[%s]", leftMoney.Currency, op, rightMoney.Currency)
		}
		raw := leftMoney.Raw + rightMoney.Raw
		if op == "-" {
			raw = leftMoney.Raw - rightMoney.Raw
		}
		return &MoneyValue{Raw: raw, Currency: leftMoney.Currency}, nil
	case "*":
		if leftIsMoney == rightIsMoney {
			return nil, runtimeErrorf(line, "Cannot multiply money by money")
		}
		money := leftMoney
		scalar := right
		if !leftIsMoney {
			money = rightMoney
			scalar = left
		}
		switch scalar := scalar.(type) {
		case int64:
			return &MoneyValue{Raw: money.Raw * scalar, Currency: money.Currency}, nil
		default:
			return nil, runtimeErrorf(line, "Cannot multiply money by %s", typeNameOf(scalar))
		}
	case "/":
		if leftIsMoney && rightIsMoney {
			if leftMoney.Currency != rightMoney.Currency {
				return nil, runtimeErrorf(line, "Currency mismatch in division: money[%s] / money[%s]", leftMoney.Currency, rightMoney.Currency)
			}
			if rightMoney.Raw == 0 {
				return nil, runtimeErrorf(line, "Division by zero")
			}
			return float64(leftMoney.Raw) / float64(rightMoney.Raw), nil
		}
		if !leftIsMoney || rightIsMoney {
			return nil, runtimeErrorf(line, "Cannot divide non-money by money")
		}
		switch scalar := right.(type) {
		case int64:
			if scalar == 0 {
				return nil, runtimeErrorf(line, "Division by zero")
			}
			return &MoneyValue{Raw: int64(math.Round(float64(leftMoney.Raw) / float64(scalar))), Currency: leftMoney.Currency}, nil
		case float64:
			if scalar == 0 {
				return nil, runtimeErrorf(line, "Division by zero")
			}
			return &MoneyValue{Raw: int64(math.Round(float64(leftMoney.Raw) / scalar)), Currency: leftMoney.Currency}, nil
		default:
			return nil, runtimeErrorf(line, "Money can only be divided by int or float")
		}
	case "=", "!=", "<", ">", "<=", ">=":
		if !leftIsMoney || !rightIsMoney {
			return nil, runtimeErrorf(line, "Cannot compare money with non-money value (%s)", op)
		}
		if leftMoney.Currency != rightMoney.Currency {
			return nil, runtimeErrorf(line, "Currency mismatch: money[%s] %s money[%s]", leftMoney.Currency, op, rightMoney.Currency)
		}
		switch op {
		case "=":
			return leftMoney.Raw == rightMoney.Raw, nil
		case "!=":
			return leftMoney.Raw != rightMoney.Raw, nil
		case "<":
			return leftMoney.Raw < rightMoney.Raw, nil
		case ">":
			return leftMoney.Raw > rightMoney.Raw, nil
		case "<=":
			return leftMoney.Raw <= rightMoney.Raw, nil
		case ">=":
			return leftMoney.Raw >= rightMoney.Raw, nil
		}
	}
	return nil, runtimeErrorf(line, "Operator %s not supported on money values", op)
}

func formatMoney(value *MoneyValue) string {
	text := strconv.FormatFloat(float64(value.Raw)/float64(moneyScale), 'f', 4, 64)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		if strings.HasSuffix(text, ".") {
			text += "00"
		}
		if parts := strings.Split(text, "."); len(parts) == 2 && len(parts[1]) == 1 {
			text += "0"
		}
	}
	return text + " " + value.Currency
}
