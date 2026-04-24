package hir

type binder struct{}

type valueSymbol struct {
	kind         BindingKind
	id           int
	name         string
	sourceModule string
	objectName   string
	module       *moduleSymbol
	object       *objectSymbol
}

type moduleSymbol struct {
	name           string
	runtimeExports map[string]valueSymbol
}

type objectSymbol struct {
	name           string
	fields         map[string]struct{}
	methods        map[string]struct{}
	hasConstructor bool
}

type bindScope struct {
	parent      *bindScope
	nextValueID *int
	values      map[string]valueSymbol
	modules     map[string]*moduleSymbol
	objects     map[string]*objectSymbol
	function    bool
}

var coreBuiltins = []string{
	"write",
	"read",
	"len",
	"str",
	"int",
	"float",
	"typeof",
}

var stdlibModuleExports = map[string][]string{
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
		"startswith",
		"endswith",
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
		"readdir",
		"writefile",
		"appendfile",
	},
	"path":   {"basename", "dirname", "joinpath"},
	"os":     {"args", "cwd", "getenv"},
	"time":   {"sleep", "nowunix", "nowunixms", "nowrfc3339"},
	"json":   {"parseobject", "parsearray", "stringify", "objectof", "arrayof", "null", "isnull"},
	"http":   {"get", "request", "listen", "addr", "wait", "close", "method", "path", "requestbody", "requestheader", "requestcookie", "status", "responsebody", "responseheader", "query", "route", "text", "html", "json", "redirect", "withheader", "withcookie", "static"},
	"state":  {"cell", "get", "set", "update"},
	"sqlite": {"open", "close", "exec", "query"},
}

func BindProgram(program *Program) error {
	return (&binder{}).bindProgram(program)
}

func newBindScope(parent *bindScope) *bindScope {
	nextValueID := new(int)
	*nextValueID = 1
	if parent != nil {
		nextValueID = parent.nextValueID
	}
	return &bindScope{
		parent:      parent,
		nextValueID: nextValueID,
		values:      map[string]valueSymbol{},
		modules:     map[string]*moduleSymbol{},
		objects:     map[string]*objectSymbol{},
	}
}

func (s *bindScope) child() *bindScope {
	return newBindScope(s)
}

func (s *bindScope) childFunction() *bindScope {
	child := newBindScope(s)
	child.function = true
	return child
}

func (s *bindScope) defineValue(symbol valueSymbol) valueSymbol {
	if symbol.id == 0 && s.nextValueID != nil {
		symbol.id = *s.nextValueID
		*s.nextValueID = *s.nextValueID + 1
	}
	s.values[symbol.name] = symbol
	return symbol
}

func (s *bindScope) resolveValue(name string) (valueSymbol, bool) {
	value, _, ok := s.resolveValueWithDepth(name)
	return value, ok
}

func (s *bindScope) resolveValueWithDepth(name string) (valueSymbol, int, bool) {
	if value, ok := s.values[name]; ok {
		return value, 0, true
	}
	if s.parent != nil {
		value, depth, ok := s.parent.resolveValueWithDepth(name)
		if ok {
			return value, depth + 1, true
		}
	}
	return valueSymbol{}, 0, false
}

func (s *bindScope) resolveValueInFunctionWithDepth(name string) (valueSymbol, int, bool) {
	depth := 0
	for cur := s; cur != nil; cur = cur.parent {
		if value, ok := cur.values[name]; ok {
			return value, depth, true
		}
		if cur.function {
			break
		}
		depth++
	}
	return valueSymbol{}, 0, false
}

func (s *bindScope) resolveOuterValue(name string) (valueSymbol, int, bool) {
	if s.parent == nil {
		return valueSymbol{}, 0, false
	}
	value, depth, ok := s.parent.resolveValueWithDepth(name)
	if !ok {
		return valueSymbol{}, 0, false
	}
	return value, depth + 1, true
}

func (s *bindScope) resolveLocalValue(name string) (valueSymbol, bool) {
	value, ok := s.values[name]
	return value, ok
}

func (s *bindScope) defineModule(module *moduleSymbol) {
	s.modules[module.name] = module
}

func (s *bindScope) resolveModule(name string) (*moduleSymbol, bool) {
	if module, ok := s.modules[name]; ok {
		return module, true
	}
	if s.parent != nil {
		return s.parent.resolveModule(name)
	}
	return nil, false
}

func (s *bindScope) defineObject(object *objectSymbol) {
	s.objects[object.name] = object
}

func (s *bindScope) resolveObject(name string) (*objectSymbol, bool) {
	if object, ok := s.objects[name]; ok {
		return object, true
	}
	if s.parent != nil {
		return s.parent.resolveObject(name)
	}
	return nil, false
}

func (sym valueSymbol) publicBinding() *NameBinding {
	return sym.publicBindingAtDepth(0)
}

func (sym valueSymbol) publicBindingAtDepth(depth int) *NameBinding {
	return &NameBinding{
		Kind:         sym.kind,
		ID:           sym.id,
		Name:         sym.name,
		SourceModule: sym.sourceModule,
		ObjectName:   sym.objectName,
		ScopeDepth:   depth,
	}
}

func (b *binder) bindProgram(program *Program) error {
	scope := newBindScope(nil)
	b.seedBuiltins(scope)
	b.seedStdlibModules(scope)
	return b.bindItems(program.Items, scope)
}

func (b *binder) seedBuiltins(scope *bindScope) {
	for _, name := range coreBuiltins {
		scope.defineValue(valueSymbol{kind: BindingBuiltin, name: name})
	}
	for _, exports := range stdlibModuleExports {
		for _, name := range exports {
			scope.defineValue(valueSymbol{kind: BindingBuiltin, name: name})
		}
	}
}

func (b *binder) seedStdlibModules(scope *bindScope) {
	for moduleName, exports := range stdlibModuleExports {
		module := &moduleSymbol{
			name:           moduleName,
			runtimeExports: map[string]valueSymbol{},
		}
		for _, name := range exports {
			module.runtimeExports[name] = valueSymbol{
				kind:         BindingImported,
				name:         name,
				sourceModule: moduleName,
			}
		}
		scope.defineModule(module)
	}
}

func (b *binder) bindUse(use *Use, scope *bindScope) {
	if len(use.Names) == 0 {
		module, ok := scope.resolveModule(use.Module)
		if !ok {
			module = &moduleSymbol{name: use.Module, runtimeExports: map[string]valueSymbol{}}
			scope.defineModule(module)
		}
		scope.defineValue(valueSymbol{
			kind:   BindingModule,
			name:   use.Module,
			module: module,
		})
		return
	}

	module, _ := scope.resolveModule(use.Module)
	for _, name := range use.Names {
		symbol := valueSymbol{
			kind:         BindingImported,
			name:         name,
			sourceModule: use.Module,
		}
		if module != nil {
			if exported, ok := module.runtimeExports[name]; ok {
				symbol.objectName = exported.objectName
				symbol.object = exported.object
				if exported.kind == BindingObjectType {
					symbol.kind = BindingObjectType
					symbol.name = exported.name
				}
			}
		}
		scope.defineValue(symbol)
		if symbol.object != nil {
			scope.defineObject(symbol.object)
		}
	}
}

func (b *binder) bindItems(items []Item, scope *bindScope) error {
	var deferred []func() error
	for _, item := range items {
		switch node := item.(type) {
		case *Use:
			b.bindUse(node, scope)
		case Decl:
			b.predeclareDecls([]Decl{node}, scope)
			decl := node
			deferred = append(deferred, func() error {
				return b.bindDecl(decl, scope)
			})
		case *StmtItem:
			if err := b.bindStmtDeferred(node.Stmt, scope, &deferred); err != nil {
				return err
			}
		}
	}
	for _, callback := range deferred {
		if err := callback(); err != nil {
			return err
		}
	}
	return nil
}

func (b *binder) bindUses(uses []*Use, scope *bindScope) {
	for _, use := range uses {
		b.bindUse(use, scope)
	}
}

func (b *binder) bindUsesOldCompat(uses []*Use, scope *bindScope) {
	for _, use := range uses {
		if len(use.Names) == 0 {
			module, ok := scope.resolveModule(use.Module)
			if !ok {
				module = &moduleSymbol{name: use.Module, runtimeExports: map[string]valueSymbol{}}
				scope.defineModule(module)
			}
			scope.defineValue(valueSymbol{
				kind:   BindingModule,
				name:   use.Module,
				module: module,
			})
			continue
		}
		module, _ := scope.resolveModule(use.Module)
		for _, name := range use.Names {
			symbol := valueSymbol{
				kind:         BindingImported,
				name:         name,
				sourceModule: use.Module,
			}
			if module != nil {
				if exported, ok := module.runtimeExports[name]; ok {
					symbol.objectName = exported.objectName
					symbol.object = exported.object
					if exported.kind == BindingObjectType {
						symbol.kind = BindingObjectType
						symbol.name = exported.name
					}
				}
			}
			scope.defineValue(symbol)
			if symbol.object != nil {
				scope.defineObject(symbol.object)
			}
		}
	}
}

func (b *binder) predeclareDecls(decls []Decl, scope *bindScope) {
	for _, decl := range decls {
		switch node := decl.(type) {
		case *Func:
			symbol := scope.defineValue(valueSymbol{kind: BindingFunc, name: node.Name})
			node.Binding = symbol.publicBinding()
		case *Object:
			object := buildObjectSymbol(node)
			scope.defineObject(object)
			scope.defineValue(valueSymbol{
				kind:       BindingObjectType,
				name:       node.Name,
				objectName: node.Name,
				object:     object,
			})
		case *Module:
			module := buildModuleSymbol(node)
			scope.defineModule(module)
			scope.defineValue(valueSymbol{
				kind:   BindingModule,
				name:   node.Name,
				module: module,
			})
		}
	}
}

func buildObjectSymbol(node *Object) *objectSymbol {
	object := &objectSymbol{
		name:           node.Name,
		fields:         map[string]struct{}{},
		methods:        map[string]struct{}{},
		hasConstructor: node.Constructor != nil,
	}
	for _, field := range node.Fields {
		object.fields[field.Name] = struct{}{}
	}
	for _, method := range node.Methods {
		object.methods[method.Name] = struct{}{}
	}
	return object
}

func buildModuleSymbol(node *Module) *moduleSymbol {
	module := &moduleSymbol{
		name:           node.Name,
		runtimeExports: map[string]valueSymbol{},
	}
	for _, decl := range node.Decls() {
		switch item := decl.(type) {
		case *Func:
			if item.Exported {
				module.runtimeExports[item.Name] = valueSymbol{
					kind:         BindingImported,
					name:         item.Name,
					sourceModule: node.Name,
				}
			}
		case *Object:
			if item.Exported {
				object := buildObjectSymbol(item)
				module.runtimeExports[item.Name] = valueSymbol{
					kind:         BindingObjectType,
					name:         item.Name,
					sourceModule: node.Name,
					objectName:   item.Name,
					object:       object,
				}
			}
		}
	}
	return module
}

func (b *binder) bindDecl(decl Decl, scope *bindScope) error {
	switch node := decl.(type) {
	case *Func:
		return b.bindFunc(node, scope)
	case *Object:
		return b.bindObject(node, scope)
	case *Module:
		return b.bindModule(node, scope)
	default:
		return nil
	}
}

func (b *binder) bindFunc(node *Func, scope *bindScope) error {
	funcScope := scope.childFunction()
	for _, param := range node.Params {
		symbol := funcScope.defineValue(paramSymbol(param, scope))
		param.Binding = symbol.publicBinding()
	}
	return b.bindBlock(node.Body, funcScope)
}

func (b *binder) bindObject(node *Object, scope *bindScope) error {
	if node.Constructor != nil {
		ctorScope := scope.childFunction()
		for _, param := range node.Constructor.Params {
			symbol := ctorScope.defineValue(paramSymbol(param, scope))
			param.Binding = symbol.publicBinding()
		}
		if err := b.bindBlock(node.Constructor.Body, ctorScope); err != nil {
			return err
		}
	}
	for _, method := range node.Methods {
		methodScope := scope.childFunction()
		for _, param := range method.Params {
			symbol := methodScope.defineValue(paramSymbol(param, scope))
			param.Binding = symbol.publicBinding()
		}
		if err := b.bindBlock(method.Body, methodScope); err != nil {
			return err
		}
	}
	return nil
}

func (b *binder) bindModule(node *Module, scope *bindScope) error {
	moduleScope := scope.child()
	return b.bindItems(node.Items, moduleScope)
}

func (b *binder) bindBlock(stmts []Stmt, scope *bindScope) error {
	var deferred []func() error
	for _, stmt := range stmts {
		if err := b.bindStmtDeferred(stmt, scope, &deferred); err != nil {
			return err
		}
	}
	for _, callback := range deferred {
		if err := callback(); err != nil {
			return err
		}
	}
	return nil
}

func (b *binder) bindStmt(stmt Stmt, scope *bindScope) error {
	return b.bindStmtDeferred(stmt, scope, nil)
}

func (b *binder) bindStmtDeferred(stmt Stmt, scope *bindScope, deferred *[]func() error) error {
	switch node := stmt.(type) {
	case *Use:
		b.bindUse(node, scope)
		return nil

	case *DeclStmt:
		b.predeclareDecls([]Decl{node.Decl}, scope)
		if deferred != nil {
			decl := node.Decl
			*deferred = append(*deferred, func() error {
				return b.bindDecl(decl, scope)
			})
			return nil
		}
		return b.bindDecl(node.Decl, scope)

	case *Assign:
		for _, value := range node.Values {
			if err := b.bindExpr(value, scope); err != nil {
				return err
			}
		}
		for idx, target := range node.Targets {
			ident, ok := target.(*Ident)
			if ok {
				if symbol, exists := scope.resolveLocalValue(ident.Name); exists {
					if symbol.kind == BindingBuiltin {
						symbol = scope.defineValue(valueSymbol{
							kind:       BindingLocal,
							name:       ident.Name,
							objectName: objectNameFromValueIndex(node.Values, idx, scope),
							object:     objectFromValueIndex(node.Values, idx, scope),
						})
					}
					ident.Binding = symbol.publicBinding()
				} else if symbol, depth, exists := scope.resolveValueInFunctionWithDepth(ident.Name); exists {
					if symbol.kind == BindingBuiltin {
						symbol = scope.defineValue(valueSymbol{
							kind:       BindingLocal,
							name:       ident.Name,
							objectName: objectNameFromValueIndex(node.Values, idx, scope),
							object:     objectFromValueIndex(node.Values, idx, scope),
						})
						ident.Binding = symbol.publicBinding()
					} else {
						ident.Binding = symbol.publicBindingAtDepth(depth)
					}
				} else {
					symbol := scope.defineValue(valueSymbol{
						kind:       BindingLocal,
						name:       ident.Name,
						objectName: objectNameFromValueIndex(node.Values, idx, scope),
						object:     objectFromValueIndex(node.Values, idx, scope),
					})
					ident.Binding = symbol.publicBinding()
				}
				continue
			}
			if err := b.bindExpr(target, scope); err != nil {
				return err
			}
		}
		return nil

	case *Var:
		if node.Value != nil {
			if err := b.bindExpr(node.Value, scope); err != nil {
				return err
			}
		}
		symbol := scope.defineValue(valueSymbol{
			kind: BindingLocal,
			name: node.Name,
			objectName: firstNonEmpty(
				objectNameFromType(node.Type, scope),
				objectNameFromExpr(node.Value, scope),
			),
			object: firstObject(
				objectFromType(node.Type, scope),
				objectFromExpr(node.Value, scope),
			),
		})
		node.Binding = symbol.publicBinding()
		return nil

	case *VarBlock:
		if node.DefaultValue != nil {
			if err := b.bindExpr(node.DefaultValue, scope); err != nil {
				return err
			}
		}
		for _, decl := range node.Decls {
			if err := b.bindStmt(decl, scope); err != nil {
				return err
			}
		}
		return nil

	case *Return:
		for _, value := range node.Values {
			if err := b.bindExpr(value, scope); err != nil {
				return err
			}
		}
		return nil

	case *If:
		if err := b.bindExpr(node.Condition, scope); err != nil {
			return err
		}
		if err := b.bindBlock(node.Body, scope.child()); err != nil {
			return err
		}
		for _, branch := range node.Elifs {
			if err := b.bindExpr(branch.Condition, scope); err != nil {
				return err
			}
			if err := b.bindBlock(branch.Body, scope.child()); err != nil {
				return err
			}
		}
		return b.bindBlock(node.ElseBody, scope.child())

	case *While:
		if err := b.bindExpr(node.Condition, scope); err != nil {
			return err
		}
		return b.bindBlock(node.Body, scope.child())

	case *ForRange:
		if err := b.bindExpr(node.Start, scope); err != nil {
			return err
		}
		if err := b.bindExpr(node.End, scope); err != nil {
			return err
		}
		if node.Step != nil {
			if err := b.bindExpr(node.Step, scope); err != nil {
				return err
			}
		}
		loopScope := scope.child()
		symbol := loopScope.defineValue(valueSymbol{kind: BindingLocal, name: node.Var})
		node.VarBinding = symbol.publicBinding()
		return b.bindBlock(node.Body, loopScope)

	case *ForEach:
		if err := b.bindExpr(node.Iterable, scope); err != nil {
			return err
		}
		loopScope := scope.child()
		symbol := loopScope.defineValue(valueSymbol{kind: BindingLocal, name: node.Var})
		node.VarBinding = symbol.publicBinding()
		if node.IndexVar != "" {
			indexSymbol := loopScope.defineValue(valueSymbol{kind: BindingLocal, name: node.IndexVar})
			node.IndexBinding = indexSymbol.publicBinding()
		}
		return b.bindBlock(node.Body, loopScope)

	case *Match:
		if err := b.bindExpr(node.Subject, scope); err != nil {
			return err
		}
		node.Binding = &MatchBinding{Kind: classifyMatchBinding(node)}
		for _, matchCase := range node.Cases {
			matchCase.PatternBindings = classifyMatchPatterns(matchCase.Patterns)
			caseScope := scope.child()
			for _, pattern := range matchCase.Patterns {
				if err := b.bindPattern(pattern, caseScope); err != nil {
					return err
				}
			}
			if err := b.bindBlock(matchCase.Body, caseScope); err != nil {
				return err
			}
		}
		return b.bindBlock(node.ElseBody, scope.child())

	case *Parallel:
		if err := b.bindBlock(node.Body, scope.child()); err != nil {
			return err
		}
		if node.ResultVar != "" {
			symbol := scope.defineValue(valueSymbol{kind: BindingLocal, name: node.ResultVar})
			node.ResultBinding = symbol.publicBinding()
		}
		return nil

	case *Global:
		if err := b.bindExpr(node.Value, scope); err != nil {
			return err
		}
		if symbol, depth, ok := scope.resolveOuterValue(node.Name); ok {
			node.Target = symbol.publicBindingAtDepth(depth)
		}
		return nil

	case *Arena:
		return b.bindBlock(node.Body, scope.child())

	case *ExprStmt:
		return b.bindExpr(node.Expr, scope)

	default:
		return nil
	}
}

func (b *binder) bindPattern(expr Expr, scope *bindScope) error {
	switch node := expr.(type) {
	case *Ident:
		symbol := scope.defineValue(valueSymbol{kind: BindingLocal, name: node.Name})
		node.Binding = symbol.publicBinding()
		return nil
	case *Ok:
		if ident, ok := node.Value.(*Ident); ok {
			symbol := scope.defineValue(valueSymbol{kind: BindingLocal, name: ident.Name})
			ident.Binding = symbol.publicBinding()
			return nil
		}
		return b.bindExpr(node.Value, scope)
	case *Err:
		if ident, ok := node.Value.(*Ident); ok {
			symbol := scope.defineValue(valueSymbol{kind: BindingLocal, name: ident.Name})
			ident.Binding = symbol.publicBinding()
			return nil
		}
		return b.bindExpr(node.Value, scope)
	default:
		return b.bindExpr(expr, scope)
	}
}

func (b *binder) bindExpr(expr Expr, scope *bindScope) error {
	switch node := expr.(type) {
	case nil, *IntLiteral, *FloatLiteral, *StringLiteral, *BoolLiteral:
		return nil
	case *Ident:
		if symbol, depth, ok := scope.resolveValueWithDepth(node.Name); ok {
			node.Binding = symbol.publicBindingAtDepth(depth)
		}
		return nil
	case *Binary:
		if err := b.bindExpr(node.Left, scope); err != nil {
			return err
		}
		return b.bindExpr(node.Right, scope)
	case *Unary:
		return b.bindExpr(node.Operand, scope)
	case *Call:
		if err := b.bindExpr(node.Callee, scope); err != nil {
			return err
		}
		for _, arg := range node.Args {
			if err := b.bindExpr(arg, scope); err != nil {
				return err
			}
		}
		return nil
	case *Member:
		if err := b.bindExpr(node.Object, scope); err != nil {
			return err
		}
		if module := moduleOfExpr(node.Object, scope); module != nil {
			if _, ok := module.runtimeExports[node.Member]; ok {
				node.Binding = &MemberBinding{
					Kind:      MemberBindingModuleValue,
					OwnerName: module.name,
				}
				return nil
			}
		}
		if object := objectOfExpr(node.Object, scope); object != nil {
			if isObjectTypeExpr(node.Object) && node.Member == "new" && object.hasConstructor {
				node.Binding = &MemberBinding{
					Kind:       MemberBindingObjectConstructor,
					OwnerName:  object.name,
					ObjectName: object.name,
				}
				return nil
			}
			if _, ok := object.methods[node.Member]; ok {
				node.Binding = &MemberBinding{
					Kind:       MemberBindingObjectMethod,
					OwnerName:  object.name,
					ObjectName: object.name,
				}
				return nil
			}
			if _, ok := object.fields[node.Member]; ok {
				node.Binding = &MemberBinding{
					Kind:       MemberBindingObjectField,
					OwnerName:  object.name,
					ObjectName: object.name,
				}
			}
		}
		return nil
	case *Index:
		if err := b.bindExpr(node.Object, scope); err != nil {
			return err
		}
		return b.bindExpr(node.Index, scope)
	case *Lambda:
		lambdaScope := scope.childFunction()
		for _, param := range node.Params {
			symbol := lambdaScope.defineValue(paramSymbol(param, scope))
			param.Binding = symbol.publicBinding()
		}
		return b.bindBlock(node.Body, lambdaScope)
	case *Ok:
		return b.bindExpr(node.Value, scope)
	case *Err:
		return b.bindExpr(node.Value, scope)
	case *List:
		for _, element := range node.Elements {
			if err := b.bindExpr(element, scope); err != nil {
				return err
			}
		}
		return nil
	case *Dict:
		for _, entry := range node.Entries {
			if err := b.bindExpr(entry.Key, scope); err != nil {
				return err
			}
			if err := b.bindExpr(entry.Value, scope); err != nil {
				return err
			}
		}
		return nil
	case *Cast:
		return b.bindExpr(node.Value, scope)
	case *ObjectLiteral:
		for _, field := range node.Fields {
			if err := b.bindExpr(field.Value, scope); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func paramSymbol(param *Param, scope *bindScope) valueSymbol {
	return valueSymbol{
		kind:       BindingParam,
		name:       param.Name,
		objectName: objectNameFromType(param.Type, scope),
		object:     objectFromType(param.Type, scope),
	}
}

func classifyMatchBinding(node *Match) MatchBindingKind {
	if len(node.Cases) == 0 {
		return MatchBindingValue
	}
	for _, matchCase := range node.Cases {
		for _, pattern := range matchCase.Patterns {
			switch pattern.(type) {
			case *Ok, *Err:
				continue
			default:
				return MatchBindingValue
			}
		}
	}
	return MatchBindingResult
}

func classifyMatchPatterns(patterns []Expr) []*MatchPatternBinding {
	bindings := make([]*MatchPatternBinding, 0, len(patterns))
	for _, pattern := range patterns {
		bindings = append(bindings, &MatchPatternBinding{
			Kind: classifyMatchPattern(pattern),
		})
	}
	return bindings
}

func classifyMatchPattern(pattern Expr) MatchPatternKind {
	switch pattern.(type) {
	case *Ident:
		return MatchPatternCapture
	case *Ok:
		return MatchPatternResultOk
	case *Err:
		return MatchPatternResultErr
	case *Binary:
		return MatchPatternRange
	default:
		return MatchPatternValue
	}
}

func objectNameFromType(node Type, scope *bindScope) string {
	if named, ok := node.(*NamedType); ok {
		if _, exists := scope.resolveObject(named.Name); exists {
			return named.Name
		}
	}
	return ""
}

func objectNameFromExpr(expr Expr, scope *bindScope) string {
	if object := objectOfExpr(expr, scope); object != nil {
		return object.name
	}
	return ""
}

func objectNameFromValueIndex(values []Expr, idx int, scope *bindScope) string {
	if idx < 0 || idx >= len(values) {
		return ""
	}
	return objectNameFromExpr(values[idx], scope)
}

func objectFromType(node Type, scope *bindScope) *objectSymbol {
	if named, ok := node.(*NamedType); ok {
		object, _ := scope.resolveObject(named.Name)
		return object
	}
	return nil
}

func objectFromExpr(expr Expr, scope *bindScope) *objectSymbol {
	return objectOfExpr(expr, scope)
}

func objectFromValueIndex(values []Expr, idx int, scope *bindScope) *objectSymbol {
	if idx < 0 || idx >= len(values) {
		return nil
	}
	return objectFromExpr(values[idx], scope)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstObject(values ...*objectSymbol) *objectSymbol {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func moduleOfExpr(expr Expr, scope *bindScope) *moduleSymbol {
	switch node := expr.(type) {
	case *Ident:
		if node.Binding == nil || node.Binding.Kind != BindingModule {
			return nil
		}
		module, _ := scope.resolveModule(node.Binding.Name)
		return module
	default:
		return nil
	}
}

func objectOfExpr(expr Expr, scope *bindScope) *objectSymbol {
	switch node := expr.(type) {
	case *Ident:
		if node.Binding == nil {
			return nil
		}
		if node.Binding.ObjectName == "" {
			return nil
		}
		object, _ := scope.resolveObject(node.Binding.ObjectName)
		return object
	case *ObjectLiteral:
		object, _ := scope.resolveObject(node.Name)
		return object
	case *Call:
		member, ok := node.Callee.(*Member)
		if !ok || member.Binding == nil || member.Binding.Kind != MemberBindingObjectConstructor {
			return nil
		}
		object, _ := scope.resolveObject(member.Binding.ObjectName)
		return object
	default:
		return nil
	}
}

func isObjectTypeExpr(expr Expr) bool {
	ident, ok := expr.(*Ident)
	return ok && ident.Binding != nil && ident.Binding.Kind == BindingObjectType
}
