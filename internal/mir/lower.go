package mir

import (
	"fmt"

	"github.com/Cass-ette/gwen-lang/internal/hir"
)

type lowerer struct {
	bindingTypes     map[int]hir.Type
	moduleValueTypes map[string]hir.Type
	aliases          map[string]hir.Type
	objectTypes      map[string]*objectTypeInfo
	lambdaDecls      []*Func
	nextLambda       int
}

type bodyBuilder struct {
	lowerer     *lowerer
	body        *Body
	returns     []hir.Type
	loopTargets map[int]loopTarget
	slots       map[int]*Slot
	computed    map[int]map[int]struct{}
}

type loopTarget struct {
	next  int
	leave int
}

type cursor struct {
	block     int
	reachable bool
}

func LowerProgram(program *hir.Program) (*Program, error) {
	return newLowerer(program).lowerProgram(program)
}

func newLowerer(program *hir.Program) *lowerer {
	l := &lowerer{
		bindingTypes:     map[int]hir.Type{},
		moduleValueTypes: map[string]hir.Type{},
		aliases:          map[string]hir.Type{},
		objectTypes:      map[string]*objectTypeInfo{},
	}
	for moduleName, exports := range stdlibCallableTypes {
		for exportName, signature := range exports {
			l.moduleValueTypes[moduleValueKey(moduleName, exportName)] = signature
		}
	}
	if program != nil {
		l.seedProgram(program)
	}
	return l
}

func (l *lowerer) rememberBindingType(binding *hir.NameBinding, typ hir.Type) {
	if l == nil || binding == nil || binding.ID == 0 || typ == nil {
		return
	}
	if existing, ok := l.bindingTypes[binding.ID]; ok && existing != nil {
		return
	}
	l.bindingTypes[binding.ID] = typ
}

func (l *lowerer) bindingType(binding *hir.NameBinding) hir.Type {
	if l == nil || binding == nil || binding.ID == 0 {
		return nil
	}
	if typ := l.bindingTypes[binding.ID]; typ != nil {
		return typ
	}
	typ := l.fallbackBindingType(binding)
	if typ != nil {
		l.rememberBindingType(binding, typ)
	}
	return typ
}

func (l *lowerer) fallbackBindingType(binding *hir.NameBinding) hir.Type {
	if l == nil || binding == nil {
		return nil
	}
	switch binding.Kind {
	case hir.BindingBuiltin:
		return builtinCallableTypes[binding.Name]
	case hir.BindingImported:
		if binding.SourceModule != "" {
			if typ, ok := l.moduleValueTypes[moduleValueKey(binding.SourceModule, binding.Name)]; ok {
				return typ
			}
		}
		if binding.ObjectName != "" {
			return namedType(binding.ObjectName)
		}
	case hir.BindingObjectType:
		if binding.ObjectName != "" {
			return namedType(binding.ObjectName)
		}
		return namedType(binding.Name)
	}
	return nil
}

func (l *lowerer) seedProgram(program *hir.Program) {
	for _, item := range program.Items {
		switch node := item.(type) {
		case hir.Decl:
			l.seedDecl(node)
		case *hir.StmtItem:
			l.seedStmt(node.Stmt)
		}
	}
}

func (l *lowerer) seedDecl(decl hir.Decl) {
	switch node := decl.(type) {
	case *hir.Func:
		l.rememberBindingType(node.Binding, signatureType(node.Params, node.Returns))
		for _, param := range node.Params {
			l.rememberBindingType(param.Binding, param.Type)
		}
		l.seedStmts(node.Body)
	case *hir.Module:
		for _, item := range node.Items {
			switch decl := item.(type) {
			case *hir.Func:
				if decl.Exported {
					l.moduleValueTypes[moduleValueKey(node.Name, decl.Name)] = signatureType(decl.Params, decl.Returns)
				}
			case *hir.Object:
				if decl.Exported {
					l.moduleValueTypes[moduleValueKey(node.Name, decl.Name)] = namedType(decl.Name)
				}
			}
			if decl, ok := item.(hir.Decl); ok {
				l.seedDecl(decl)
			}
		}
	case *hir.Object:
		info := &objectTypeInfo{
			Fields:  map[string]hir.Type{},
			Methods: map[string]*hir.FuncType{},
		}
		for _, field := range node.Fields {
			info.Fields[field.Name] = field.Type
		}
		if node.Constructor != nil {
			info.Constructor = signatureType(node.Constructor.Params, node.Constructor.Returns)
		}
		for _, method := range node.Methods {
			info.Methods[method.Name] = signatureType(method.Params, method.Returns)
		}
		l.objectTypes[node.Name] = info
		if node.Constructor != nil {
			for _, param := range node.Constructor.Params {
				l.rememberBindingType(param.Binding, param.Type)
			}
			l.seedStmts(node.Constructor.Body)
		}
		for _, method := range node.Methods {
			for _, param := range method.Params {
				l.rememberBindingType(param.Binding, param.Type)
			}
			l.seedStmts(method.Body)
		}
	case *hir.TypeAlias:
		l.aliases[node.Name] = node.Target
	}
}

func (l *lowerer) resolveType(typ hir.Type) hir.Type {
	if l == nil || typ == nil {
		return typ
	}
	seen := map[string]struct{}{}
	current := typ
	for {
		named, ok := current.(*hir.NamedType)
		if !ok {
			return current
		}
		target, ok := l.aliases[named.Name]
		if !ok || target == nil {
			return current
		}
		if _, exists := seen[named.Name]; exists {
			return current
		}
		seen[named.Name] = struct{}{}
		current = target
	}
}

func (l *lowerer) seedStmts(stmts []hir.Stmt) {
	for _, stmt := range stmts {
		l.seedStmt(stmt)
	}
}

func (l *lowerer) seedStmt(stmt hir.Stmt) {
	switch node := stmt.(type) {
	case *hir.Use:
		return
	case *hir.Var:
		l.rememberBindingType(node.Binding, node.Type)
	case *hir.VarBlock:
		for _, decl := range node.Decls {
			l.seedStmt(decl)
		}
	case *hir.ForEach:
		if node.IndexBinding != nil {
			l.rememberBindingType(node.IndexBinding, &hir.NamedType{Name: "int", Line: node.Line})
		}
		l.seedStmts(node.Body)
	case *hir.ForRange:
		l.seedStmts(node.Body)
	case *hir.If:
		l.seedStmts(node.Body)
		for _, branch := range node.Elifs {
			l.seedStmts(branch.Body)
		}
		l.seedStmts(node.ElseBody)
	case *hir.While:
		l.seedStmts(node.Body)
	case *hir.Match:
		for _, matchCase := range node.Cases {
			l.seedStmts(matchCase.Body)
		}
		l.seedStmts(node.ElseBody)
	case *hir.Parallel:
		if node.ResultBinding != nil {
			l.rememberBindingType(node.ResultBinding, &hir.NamedType{Name: "list", Line: node.Line})
		}
		l.seedStmts(node.Body)
	case *hir.Arena:
		l.seedStmts(node.Body)
	case *hir.DeclStmt:
		l.seedDecl(node.Decl)
	}
}

func (l *lowerer) lowerProgram(program *hir.Program) (*Program, error) {
	if program == nil {
		return nil, fmt.Errorf("cannot lower nil HIR program")
	}

	out := &Program{}
	var pendingScript []hir.Stmt

	flushScript := func() error {
		if len(pendingScript) == 0 {
			return nil
		}
		body, err := l.lowerBody(pendingScript)
		if err != nil {
			return err
		}
		out.Items = append(out.Items, &Script{
			Body: body,
			Line: pendingScript[0].Pos(),
		})
		pendingScript = nil
		return nil
	}

	for _, item := range program.Items {
		switch node := item.(type) {
		case *hir.Use:
			if err := flushScript(); err != nil {
				return nil, err
			}
			out.Items = append(out.Items, lowerUse(node))
		case hir.Decl:
			if err := flushScript(); err != nil {
				return nil, err
			}
			decl, err := l.lowerDecl(node)
			if err != nil {
				return nil, err
			}
			out.Items = append(out.Items, decl)
		case *hir.StmtItem:
			pendingScript = append(pendingScript, node.Stmt)
		default:
			return nil, fmt.Errorf("unsupported HIR item %T", item)
		}
	}

	if err := flushScript(); err != nil {
		return nil, err
	}
	for _, fn := range l.lambdaDecls {
		out.Items = append(out.Items, fn)
	}
	return out, nil
}

func lowerUse(node *hir.Use) *Use {
	return &Use{
		Module: node.Module,
		Names:  append([]string(nil), node.Names...),
		Line:   node.Line,
	}
}

func (l *lowerer) lowerDecl(node hir.Decl) (Decl, error) {
	switch decl := node.(type) {
	case *hir.Func:
		return l.lowerFunc(decl)
	case *hir.Module:
		return l.lowerModule(decl)
	case *hir.Object:
		return l.lowerObject(decl)
	case *hir.TypeAlias:
		return &TypeAlias{
			Name:     decl.Name,
			Target:   decl.Target,
			Exported: decl.Exported,
			Line:     decl.Line,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported HIR decl %T", node)
	}
}

func (l *lowerer) lowerFunc(node *hir.Func) (*Func, error) {
	body, err := l.lowerBodyWithParams(node.Body, node.Params, node.Returns)
	if err != nil {
		return nil, err
	}
	return &Func{
		Name:     node.Name,
		Params:   append([]*hir.Param(nil), node.Params...),
		Returns:  append([]hir.Type(nil), node.Returns...),
		Body:     body,
		Binding:  node.Binding,
		Exported: node.Exported,
		Line:     node.Line,
	}, nil
}

func (l *lowerer) lowerModule(node *hir.Module) (*Module, error) {
	out := &Module{
		Name: node.Name,
		Line: node.Line,
	}
	for _, item := range node.Items {
		switch inner := item.(type) {
		case *hir.Use:
			out.Items = append(out.Items, lowerUse(inner))
		case hir.Decl:
			decl, err := l.lowerDecl(inner)
			if err != nil {
				return nil, err
			}
			out.Items = append(out.Items, decl)
		default:
			return nil, fmt.Errorf("unsupported HIR module item %T in module %q", item, node.Name)
		}
	}
	return out, nil
}

func (l *lowerer) lowerObject(node *hir.Object) (*Object, error) {
	out := &Object{
		Name:     node.Name,
		Fields:   append([]*hir.Field(nil), node.Fields...),
		Exported: node.Exported,
		Line:     node.Line,
	}

	if node.Constructor != nil {
		body, err := l.lowerBodyWithParams(node.Constructor.Body, node.Constructor.Params, node.Constructor.Returns)
		if err != nil {
			return nil, err
		}
		out.Constructor = &Constructor{
			Name:    node.Constructor.Name,
			Params:  append([]*hir.Param(nil), node.Constructor.Params...),
			Returns: append([]hir.Type(nil), node.Constructor.Returns...),
			Body:    body,
			Line:    node.Constructor.Line,
		}
	}

	out.Methods = make([]*Method, 0, len(node.Methods))
	for _, method := range node.Methods {
		body, err := l.lowerBodyWithParams(method.Body, method.Params, method.Returns)
		if err != nil {
			return nil, err
		}
		out.Methods = append(out.Methods, &Method{
			Name:    method.Name,
			Params:  append([]*hir.Param(nil), method.Params...),
			Returns: append([]hir.Type(nil), method.Returns...),
			Body:    body,
			Line:    method.Line,
		})
	}

	return out, nil
}

func (l *lowerer) lowerLambdaFunc(node *hir.Lambda) (*Func, *hir.FuncType, bool) {
	if l == nil || node == nil {
		return nil, nil, false
	}
	signature := l.inferLambdaType(node)
	if signature == nil {
		return nil, nil, false
	}
	body, err := l.lowerBodyWithParams(node.Body, node.Params, signature.Returns)
	if err != nil {
		return nil, nil, false
	}
	l.nextLambda++
	name := fmt.Sprintf("lambda_%d", l.nextLambda)
	binding := &hir.NameBinding{
		Kind: hir.BindingFunc,
		Name: name,
	}
	fn := &Func{
		Name:    name,
		Params:  append([]*hir.Param(nil), node.Params...),
		Returns: append([]hir.Type(nil), signature.Returns...),
		Body:    body,
		Binding: binding,
		Line:    node.Line,
	}
	l.lambdaDecls = append(l.lambdaDecls, fn)
	return fn, signature, true
}

func (l *lowerer) inferLambdaType(node *hir.Lambda) *hir.FuncType {
	if l == nil || node == nil || len(node.Body) != 1 {
		return nil
	}
	ret, ok := node.Body[0].(*hir.Return)
	if !ok || len(ret.Values) != 1 {
		return nil
	}
	paramTypes := make([]hir.Type, 0, len(node.Params))
	for _, param := range node.Params {
		if param == nil || param.Type == nil || param.Binding == nil || param.Default != nil {
			return nil
		}
		paramTypes = append(paramTypes, param.Type)
		l.rememberBindingType(param.Binding, param.Type)
	}
	returnBuilder := &bodyBuilder{lowerer: l}
	returnType := returnBuilder.inferExprType(ret.Values[0])
	if returnType == nil {
		return nil
	}
	return &hir.FuncType{
		Params:  append([]hir.Type(nil), paramTypes...),
		Returns: []hir.Type{returnType},
		Line:    node.Line,
	}
}

func (l *lowerer) lowerBody(stmts []hir.Stmt) (*Body, error) {
	return l.lowerBodyWithParams(stmts, nil, nil)
}

func (l *lowerer) lowerBodyWithParams(stmts []hir.Stmt, params []*hir.Param, returns []hir.Type) (*Body, error) {
	builder := newBodyBuilder(l, params, returns)
	cur, err := builder.lowerStmts(stmts, cursor{
		block:     builder.body.Entry,
		reachable: true,
	})
	if err != nil {
		return nil, err
	}
	if cur.reachable {
		if err := builder.setTerm(cur.block, &StopTerm{}); err != nil {
			return nil, err
		}
	}
	return builder.body, nil
}

func newBodyBuilder(l *lowerer, params []*hir.Param, returns []hir.Type) *bodyBuilder {
	body := &Body{}
	builder := &bodyBuilder{
		lowerer:     l,
		body:        body,
		returns:     append([]hir.Type(nil), returns...),
		loopTargets: map[int]loopTarget{},
		slots:       map[int]*Slot{},
		computed:    map[int]map[int]struct{}{},
	}
	entry := builder.newBlock().ID
	body.Entry = entry
	for _, param := range params {
		builder.ensureSlot(param.Binding, SlotParam, param.Type)
	}
	return builder
}

func (b *bodyBuilder) newBlock() *Block {
	block := &Block{ID: len(b.body.Blocks) + 1}
	b.body.Blocks = append(b.body.Blocks, block)
	return block
}

func (b *bodyBuilder) block(id int) (*Block, error) {
	block := b.body.Block(id)
	if block == nil {
		return nil, fmt.Errorf("unknown MIR block %d", id)
	}
	return block, nil
}

func (b *bodyBuilder) addOp(blockID int, op Op) error {
	block, err := b.block(blockID)
	if err != nil {
		return err
	}
	block.Ops = append(block.Ops, op)
	return nil
}

func (b *bodyBuilder) addInst(blockID int, inst Inst) error {
	block, err := b.block(blockID)
	if err != nil {
		return err
	}
	block.Insts = append(block.Insts, inst)
	return nil
}

func (b *bodyBuilder) setTerm(blockID int, term Terminator) error {
	block, err := b.block(blockID)
	if err != nil {
		return err
	}
	if block.Term != nil {
		return fmt.Errorf("block %d already has terminator %T", blockID, block.Term)
	}
	block.Term = term
	return nil
}

func (b *bodyBuilder) appendValue(value *Value) int {
	if b == nil || b.body == nil || value == nil {
		return 0
	}
	value.ID = len(b.body.Values) + 1
	b.body.Values = append(b.body.Values, value)
	return value.ID
}

func (b *bodyBuilder) appendPlace(place *Place) int {
	if b == nil || b.body == nil || place == nil {
		return 0
	}
	place.ID = len(b.body.Places) + 1
	b.body.Places = append(b.body.Places, place)
	return place.ID
}

func (b *bodyBuilder) blockComputed(blockID int) map[int]struct{} {
	if b.computed == nil {
		b.computed = map[int]map[int]struct{}{}
	}
	if b.computed[blockID] == nil {
		b.computed[blockID] = map[int]struct{}{}
	}
	return b.computed[blockID]
}

func (b *bodyBuilder) newValue(kind ValueKind, typ hir.Type, source hir.Expr) int {
	return b.appendValue(&Value{
		Kind:   kind,
		Type:   typ,
		Source: source,
	})
}

func (b *bodyBuilder) ensureSlot(binding *hir.NameBinding, kind SlotKind, typ hir.Type) *Slot {
	if binding == nil || binding.ID == 0 {
		return nil
	}
	if typ == nil {
		typ = b.bindingType(binding)
	}
	if slot, ok := b.slots[binding.ID]; ok {
		if slot.Type == nil && typ != nil {
			slot.Type = typ
		}
		return slot
	}
	slot := &Slot{
		ID:        len(b.body.Slots) + 1,
		Kind:      kind,
		BindingID: binding.ID,
		Name:      binding.Name,
		Type:      typ,
	}
	b.body.Slots = append(b.body.Slots, slot)
	b.slots[binding.ID] = slot
	return slot
}

func (b *bodyBuilder) bindingType(binding *hir.NameBinding) hir.Type {
	if binding == nil {
		return nil
	}
	if slot := b.body.SlotByBindingID(binding.ID); slot != nil && slot.Type != nil {
		return slot.Type
	}
	if b.lowerer == nil {
		return nil
	}
	return b.lowerer.bindingType(binding)
}

func (b *bodyBuilder) rememberBindingType(binding *hir.NameBinding, typ hir.Type) {
	if binding == nil || typ == nil {
		return
	}
	if slot := b.body.SlotByBindingID(binding.ID); slot != nil && slot.Type == nil {
		slot.Type = typ
	}
	if b.lowerer != nil {
		b.lowerer.rememberBindingType(binding, typ)
	}
}

func (b *bodyBuilder) ensureCaptureSlot(binding *hir.NameBinding) *Slot {
	if binding == nil {
		return nil
	}
	if binding.Kind != hir.BindingLocal && binding.Kind != hir.BindingParam {
		return nil
	}
	if binding.ScopeDepth == 0 {
		return nil
	}
	return b.ensureSlot(binding, SlotCapture, nil)
}

func (b *bodyBuilder) collectCaptureBindingsExpr(expr hir.Expr) {
	switch node := expr.(type) {
	case nil, *hir.IntLiteral, *hir.FloatLiteral, *hir.StringLiteral, *hir.BoolLiteral:
		return
	case *hir.Ident:
		b.ensureCaptureSlot(node.Binding)
	case *hir.Binary:
		b.collectCaptureBindingsExpr(node.Left)
		b.collectCaptureBindingsExpr(node.Right)
	case *hir.Unary:
		b.collectCaptureBindingsExpr(node.Operand)
	case *hir.Call:
		b.collectCaptureBindingsExpr(node.Callee)
		for _, arg := range node.Args {
			b.collectCaptureBindingsExpr(arg)
		}
	case *hir.Member:
		b.collectCaptureBindingsExpr(node.Object)
	case *hir.Index:
		b.collectCaptureBindingsExpr(node.Object)
		b.collectCaptureBindingsExpr(node.Index)
	case *hir.Lambda:
		for _, param := range node.Params {
			if param.Default != nil {
				b.collectCaptureBindingsExpr(param.Default)
			}
		}
	case *hir.Ok:
		b.collectCaptureBindingsExpr(node.Value)
	case *hir.Err:
		b.collectCaptureBindingsExpr(node.Value)
	case *hir.List:
		for _, element := range node.Elements {
			b.collectCaptureBindingsExpr(element)
		}
	case *hir.Dict:
		for _, entry := range node.Entries {
			b.collectCaptureBindingsExpr(entry.Key)
			b.collectCaptureBindingsExpr(entry.Value)
		}
	case *hir.Cast:
		b.collectCaptureBindingsExpr(node.Value)
	case *hir.ObjectLiteral:
		for _, field := range node.Fields {
			b.collectCaptureBindingsExpr(field.Value)
		}
	}
}

func (b *bodyBuilder) ensureAssignSlots(targets []hir.Expr) {
	for _, target := range targets {
		ident, ok := target.(*hir.Ident)
		if !ok || ident.Binding == nil {
			b.collectCaptureBindingsExpr(target)
			continue
		}
		switch ident.Binding.Kind {
		case hir.BindingParam:
			b.ensureSlot(ident.Binding, SlotParam, nil)
		case hir.BindingLocal:
			if ident.Binding.ScopeDepth == 0 {
				b.ensureSlot(ident.Binding, SlotLocal, nil)
			} else {
				b.ensureCaptureSlot(ident.Binding)
			}
		default:
			b.collectCaptureBindingsExpr(target)
		}
	}
}

func (b *bodyBuilder) ensurePatternSlots(patterns []hir.Expr, bindings []*hir.MatchPatternBinding) {
	for idx, pattern := range patterns {
		if idx < len(bindings) {
			switch bindings[idx].Kind {
			case hir.MatchPatternCapture:
				if ident, ok := pattern.(*hir.Ident); ok {
					b.ensureSlot(ident.Binding, SlotLocal, nil)
				}
			case hir.MatchPatternResultOk:
				if okExpr, ok := pattern.(*hir.Ok); ok {
					if ident, ok := okExpr.Value.(*hir.Ident); ok {
						b.ensureSlot(ident.Binding, SlotLocal, nil)
					}
				}
			case hir.MatchPatternResultErr:
				if errExpr, ok := pattern.(*hir.Err); ok {
					if ident, ok := errExpr.Value.(*hir.Ident); ok {
						b.ensureSlot(ident.Binding, SlotLocal, nil)
					}
				}
			}
		}
		b.collectCaptureBindingsExpr(pattern)
	}
}

func (b *bodyBuilder) lowerExprList(exprs []hir.Expr) []int {
	var ids []int
	for _, expr := range exprs {
		ids = append(ids, b.lowerExprValues(expr)...)
	}
	return ids
}

func (b *bodyBuilder) lowerTargetPlaceList(targets []hir.Expr) []int {
	var ids []int
	for _, target := range targets {
		ids = append(ids, b.lowerTargetPlace(target))
	}
	return ids
}

func (b *bodyBuilder) lowerExprValues(expr hir.Expr) []int {
	if expr == nil {
		return nil
	}
	if call, ok := expr.(*hir.Call); ok {
		return b.lowerCallValues(call)
	}
	return []int{b.lowerExprValue(expr)}
}

func (b *bodyBuilder) lowerTargetPlace(target hir.Expr) int {
	switch node := target.(type) {
	case nil:
		return 0
	case *hir.Ident:
		slot := b.slotForBinding(node.Binding)
		if slot == nil {
			return 0
		}
		return b.appendPlace(&Place{
			Kind:    PlaceSlot,
			Type:    slot.Type,
			Source:  node,
			SlotID:  slot.ID,
			Binding: node.Binding,
		})
	case *hir.Index:
		return b.appendPlace(&Place{
			Kind:   PlaceIndex,
			Type:   b.inferIndexType(node),
			Source: node,
			Object: b.lowerExprSingle(node.Object),
			Index:  b.lowerExprSingle(node.Index),
		})
	case *hir.Member:
		return b.appendPlace(&Place{
			Kind:          PlaceField,
			Type:          b.inferMemberType(node),
			Source:        node,
			Object:        b.lowerExprSingle(node.Object),
			Member:        node.Member,
			MemberBinding: node.Binding,
		})
	default:
		return 0
	}
}

func (b *bodyBuilder) emitValue(blockID int, valueID int) error {
	if valueID == 0 {
		return nil
	}
	value := b.body.Value(valueID)
	if value == nil {
		return fmt.Errorf("unknown MIR value %d", valueID)
	}
	seen := b.blockComputed(blockID)
	if _, ok := seen[valueID]; ok {
		return nil
	}
	for _, dep := range b.valueDeps(value) {
		if err := b.emitValue(blockID, dep); err != nil {
			return err
		}
	}
	if !valueNeedsInst(value) {
		seen[valueID] = struct{}{}
		return nil
	}
	var inst Inst
	switch value.Kind {
	case ValueCall:
		inst = &CallInst{
			ValueID:   valueID,
			ResultIDs: append([]int(nil), value.ResultIDs...),
			Line:      value.Source.Pos(),
		}
	default:
		inst = &ComputeInst{
			ValueID: valueID,
			Line:    value.Source.Pos(),
		}
	}
	if err := b.addInst(blockID, inst); err != nil {
		return err
	}
	seen[valueID] = struct{}{}
	return nil
}

func (b *bodyBuilder) emitValues(blockID int, valueIDs []int) error {
	for _, valueID := range valueIDs {
		if err := b.emitValue(blockID, valueID); err != nil {
			return err
		}
	}
	return nil
}

func (b *bodyBuilder) emitPlace(blockID int, placeID int) error {
	if placeID == 0 {
		return nil
	}
	place := b.body.Place(placeID)
	if place == nil {
		return fmt.Errorf("unknown MIR place %d", placeID)
	}
	switch place.Kind {
	case PlaceIndex:
		if err := b.emitValue(blockID, place.Object); err != nil {
			return err
		}
		if err := b.emitValue(blockID, place.Index); err != nil {
			return err
		}
	case PlaceField:
		if err := b.emitValue(blockID, place.Object); err != nil {
			return err
		}
	}
	return nil
}

func (b *bodyBuilder) emitStore(blockID, placeID, valueID, line int) error {
	if placeID == 0 {
		return nil
	}
	if err := b.emitPlace(blockID, placeID); err != nil {
		return err
	}
	if err := b.emitValue(blockID, valueID); err != nil {
		return err
	}
	return b.addInst(blockID, &StoreInst{
		PlaceID: placeID,
		ValueID: valueID,
		Line:    line,
	})
}

func (b *bodyBuilder) emitDeclare(blockID, placeID, valueID int, isConst, isUninit bool, line int) error {
	if placeID == 0 {
		return nil
	}
	if valueID != 0 {
		if err := b.emitValue(blockID, valueID); err != nil {
			return err
		}
	}
	return b.addInst(blockID, &DeclareInst{
		PlaceID:  placeID,
		ValueID:  valueID,
		IsConst:  isConst,
		IsUninit: isUninit,
		Line:     line,
	})
}

func (b *bodyBuilder) valueDeps(value *Value) []int {
	if value == nil {
		return nil
	}
	var deps []int
	add := func(id int) {
		if id != 0 {
			deps = append(deps, id)
		}
	}
	add(value.Operand)
	add(value.Left)
	add(value.Right)
	add(value.CallID)
	add(value.Callee)
	for _, arg := range value.Args {
		add(arg)
	}
	add(value.Object)
	add(value.Index)
	for _, element := range value.Elements {
		add(element)
	}
	for _, entry := range value.Entries {
		add(entry.Key)
		add(entry.Value)
	}
	for _, field := range value.Fields {
		add(field.Value)
	}
	return deps
}

func (b *bodyBuilder) lowerExprSingle(expr hir.Expr) int {
	if expr == nil {
		return 0
	}
	ids := b.lowerExprValues(expr)
	if len(ids) == 1 {
		return ids[0]
	}
	return b.newValue(ValueExprFallback, b.inferExprType(expr), expr)
}

func (b *bodyBuilder) lowerExprValue(expr hir.Expr) int {
	switch node := expr.(type) {
	case nil:
		return 0
	case *hir.IntLiteral:
		return b.appendValue(&Value{
			Kind:     ValueIntConst,
			Type:     namedType("int"),
			Source:   node,
			IntValue: node.Value,
		})
	case *hir.FloatLiteral:
		return b.appendValue(&Value{
			Kind:       ValueFloatConst,
			Type:       namedType("float"),
			Source:     node,
			FloatValue: node.Value,
		})
	case *hir.StringLiteral:
		return b.appendValue(&Value{
			Kind:        ValueStringConst,
			Type:        namedType("string"),
			Source:      node,
			StringValue: node.Value,
		})
	case *hir.BoolLiteral:
		return b.appendValue(&Value{
			Kind:      ValueBoolConst,
			Type:      namedType("bool"),
			Source:    node,
			BoolValue: node.Value,
		})
	case *hir.Ident:
		return b.lowerIdentValue(node)
	case *hir.Binary:
		return b.appendValue(&Value{
			Kind:   ValueBinary,
			Type:   b.inferBinaryType(node),
			Source: node,
			Op:     node.Op,
			Left:   b.lowerExprSingle(node.Left),
			Right:  b.lowerExprSingle(node.Right),
		})
	case *hir.Unary:
		return b.appendValue(&Value{
			Kind:    ValueUnary,
			Type:    b.inferUnaryType(node),
			Source:  node,
			Op:      node.Op,
			Operand: b.lowerExprSingle(node.Operand),
		})
	case *hir.Member:
		return b.appendValue(&Value{
			Kind:          ValueMember,
			Type:          b.inferMemberType(node),
			Source:        node,
			Object:        b.lowerExprSingle(node.Object),
			Member:        node.Member,
			MemberBinding: node.Binding,
		})
	case *hir.Index:
		return b.appendValue(&Value{
			Kind:   ValueIndex,
			Type:   b.inferIndexType(node),
			Source: node,
			Object: b.lowerExprSingle(node.Object),
			Index:  b.lowerExprSingle(node.Index),
		})
	case *hir.Cast:
		return b.appendValue(&Value{
			Kind:    ValueCast,
			Type:    b.inferExprType(node),
			Source:  node,
			Operand: b.lowerExprSingle(node.Value),
		})
	case *hir.List:
		elements := make([]int, 0, len(node.Elements))
		for _, element := range node.Elements {
			elements = append(elements, b.lowerExprSingle(element))
		}
		return b.appendValue(&Value{
			Kind:     ValueList,
			Type:     b.inferExprType(node),
			Source:   node,
			Elements: elements,
		})
	case *hir.Dict:
		entries := make([]DictEntryValue, 0, len(node.Entries))
		for _, entry := range node.Entries {
			entries = append(entries, DictEntryValue{
				Key:   b.lowerExprSingle(entry.Key),
				Value: b.lowerExprSingle(entry.Value),
			})
		}
		return b.appendValue(&Value{
			Kind:    ValueDict,
			Type:    b.inferExprType(node),
			Source:  node,
			Entries: entries,
		})
	case *hir.ObjectLiteral:
		fields := make([]ObjectFieldValue, 0, len(node.Fields))
		for _, field := range node.Fields {
			fields = append(fields, ObjectFieldValue{
				Name:  field.Name,
				Value: b.lowerExprSingle(field.Value),
			})
		}
		return b.appendValue(&Value{
			Kind:   ValueObjectLiteral,
			Type:   namedType(node.Name),
			Source: node,
			Fields: fields,
		})
	case *hir.Ok:
		return b.appendValue(&Value{
			Kind:    ValueOk,
			Type:    b.inferExprType(node),
			Source:  node,
			Operand: b.lowerExprSingle(node.Value),
		})
	case *hir.Err:
		return b.appendValue(&Value{
			Kind:    ValueErr,
			Type:    b.inferExprType(node),
			Source:  node,
			Operand: b.lowerExprSingle(node.Value),
		})
	case *hir.Lambda:
		if fn, signature, ok := b.lowerer.lowerLambdaFunc(node); ok {
			return b.appendValue(&Value{
				Kind:    ValueBindingRef,
				Type:    signature,
				Source:  node,
				Binding: fn.Binding,
			})
		}
		return b.newValue(ValueExprFallback, b.inferExprType(expr), expr)
	default:
		return b.newValue(ValueExprFallback, b.inferExprType(expr), expr)
	}
}

func (b *bodyBuilder) lowerCallValues(node *hir.Call) []int {
	if node == nil {
		return nil
	}
	args := make([]int, 0, len(node.Args))
	for _, arg := range node.Args {
		args = append(args, b.lowerExprSingle(arg))
	}
	returnTypes := b.callReturnTypes(node)
	callType := hir.Type(nil)
	if len(returnTypes) == 1 {
		callType = returnTypes[0]
	}
	callID := b.appendValue(&Value{
		Kind:        ValueCall,
		Type:        callType,
		Source:      node,
		Callee:      b.lowerExprSingle(node.Callee),
		Args:        args,
		ReturnTypes: append([]hir.Type(nil), returnTypes...),
	})
	if len(returnTypes) <= 1 {
		return []int{callID}
	}
	resultIDs := make([]int, 0, len(returnTypes))
	for idx, typ := range returnTypes {
		resultIDs = append(resultIDs, b.appendValue(&Value{
			Kind:        ValueCallResult,
			Type:        typ,
			Source:      node,
			CallID:      callID,
			ResultIndex: idx,
		}))
	}
	if call := b.body.Value(callID); call != nil {
		call.ResultIDs = append([]int(nil), resultIDs...)
	}
	return resultIDs
}

func (b *bodyBuilder) lowerIdentValue(node *hir.Ident) int {
	if node == nil {
		return 0
	}
	slot := b.slotForBinding(node.Binding)
	if slot != nil {
		return b.appendValue(&Value{
			Kind:    ValueSlotRef,
			Type:    slot.Type,
			Source:  node,
			SlotID:  slot.ID,
			Binding: node.Binding,
		})
	}
	return b.appendValue(&Value{
		Kind:    ValueBindingRef,
		Type:    b.bindingType(node.Binding),
		Source:  node,
		Binding: node.Binding,
	})
}

func (b *bodyBuilder) slotForBinding(binding *hir.NameBinding) *Slot {
	if binding == nil {
		return nil
	}
	switch binding.Kind {
	case hir.BindingParam:
		return b.ensureSlot(binding, SlotParam, nil)
	case hir.BindingLocal:
		if binding.ScopeDepth == 0 {
			return b.ensureSlot(binding, SlotLocal, nil)
		}
		return b.ensureCaptureSlot(binding)
	default:
		return nil
	}
}

func (b *bodyBuilder) inferExprType(expr hir.Expr) hir.Type {
	switch node := expr.(type) {
	case nil:
		return nil
	case *hir.IntLiteral:
		return namedType("int")
	case *hir.FloatLiteral:
		return namedType("float")
	case *hir.StringLiteral:
		return namedType("string")
	case *hir.BoolLiteral:
		return namedType("bool")
	case *hir.Ident:
		return b.bindingType(node.Binding)
	case *hir.Binary:
		return b.inferBinaryType(node)
	case *hir.Unary:
		return b.inferUnaryType(node)
	case *hir.Cast:
		return genericType("result", namedType(node.TargetName))
	case *hir.ObjectLiteral:
		return namedType(node.Name)
	case *hir.Member:
		return b.inferMemberType(node)
	case *hir.Index:
		return b.inferIndexType(node)
	case *hir.Dict:
		if node.KeyType != nil && node.ValueType != nil {
			return genericType("dict", node.KeyType, node.ValueType)
		}
		return namedType("dict")
	case *hir.List:
		if len(node.Elements) == 0 {
			return namedType("list")
		}
		itemType := b.inferExprType(node.Elements[0])
		if itemType == nil {
			return namedType("list")
		}
		for _, element := range node.Elements[1:] {
			if !typeEqual(itemType, b.inferExprType(element)) {
				return namedType("list")
			}
		}
		return genericType("list", itemType)
	case *hir.Ok:
		valueType := b.inferExprType(node.Value)
		if valueType == nil {
			valueType = namedType("dynamic")
		}
		return genericType("result", valueType)
	case *hir.Err:
		errType := b.inferExprType(node.Value)
		if errType == nil {
			errType = namedType("string")
		}
		return genericType("result", namedType("dynamic"), errType)
	case *hir.Lambda:
		if b.lowerer != nil {
			if signature := b.lowerer.inferLambdaType(node); signature != nil {
				return signature
			}
		}
		return nil
	case *hir.Call:
		returnTypes := b.callReturnTypes(node)
		if len(returnTypes) == 1 {
			return returnTypes[0]
		}
		return nil
	default:
		return nil
	}
}

func (b *bodyBuilder) inferExprTypes(expr hir.Expr) []hir.Type {
	if call, ok := expr.(*hir.Call); ok {
		return b.callReturnTypes(call)
	}
	typ := b.inferExprType(expr)
	if typ == nil {
		return nil
	}
	return []hir.Type{typ}
}

func (b *bodyBuilder) resolvedExprType(expr hir.Expr) hir.Type {
	if b == nil || b.lowerer == nil {
		return b.inferExprType(expr)
	}
	return b.lowerer.resolveType(b.inferExprType(expr))
}

func (b *bodyBuilder) inferBinaryType(node *hir.Binary) hir.Type {
	if node == nil {
		return nil
	}
	switch node.Op {
	case "=", "!=", "<", ">", "<=", ">=", "and", "or", "to":
		return namedType("bool")
	}

	leftType := b.resolvedExprType(node.Left)
	rightType := b.resolvedExprType(node.Right)
	if leftType == nil || rightType == nil {
		return nil
	}
	if isDynamicValueType(leftType) && !isDynamicValueType(rightType) {
		leftType = rightType
	}
	if isDynamicValueType(rightType) && !isDynamicValueType(leftType) {
		rightType = leftType
	}

	if node.Op == "+" && isNamedType(leftType, "string") && isNamedType(rightType, "string") {
		return namedType("string")
	}
	if node.Op == "+" && typeEqual(leftType, rightType) && listItemType(leftType) != nil {
		return leftType
	}
	if moneyType(leftType) != nil && moneyType(rightType) != nil && (node.Op == "+" || node.Op == "-") && typeEqual(leftType, rightType) {
		return leftType
	}
	if moneyType(leftType) != nil && moneyType(rightType) != nil && node.Op == "/" && typeEqual(leftType, rightType) {
		return namedType("float")
	}
	if moneyType(leftType) != nil && isNumericType(rightType) && (node.Op == "*" || node.Op == "/") {
		return leftType
	}
	if moneyType(rightType) != nil && isNumericType(leftType) && node.Op == "*" {
		return rightType
	}
	if isIntType(leftType) && isIntType(rightType) {
		return namedType("int")
	}
	if isNumericType(leftType) && isNumericType(rightType) {
		return namedType("float")
	}
	return nil
}

func (b *bodyBuilder) inferUnaryType(node *hir.Unary) hir.Type {
	if node == nil {
		return nil
	}
	switch node.Op {
	case "not":
		return namedType("bool")
	case "-":
		operandType := b.resolvedExprType(node.Operand)
		if isIntType(operandType) {
			return namedType("int")
		}
		if isNumericType(operandType) {
			return namedType("float")
		}
	}
	return nil
}

func (b *bodyBuilder) inferMemberType(node *hir.Member) hir.Type {
	if node == nil || b.lowerer == nil {
		return nil
	}
	if node.Binding != nil {
		switch node.Binding.Kind {
		case hir.MemberBindingModuleValue:
			return b.lowerer.moduleValueTypes[moduleValueKey(node.Binding.OwnerName, node.Member)]
		case hir.MemberBindingObjectConstructor:
			if info := b.lowerer.objectTypes[node.Binding.ObjectName]; info != nil {
				return info.Constructor
			}
		case hir.MemberBindingObjectMethod:
			info := b.lowerer.objectTypes[node.Binding.ObjectName]
			if info == nil {
				return nil
			}
			method := info.Methods[node.Member]
			if method == nil {
				return nil
			}
			if b.exprIsObjectType(node.Object) {
				return method
			}
			return dropFirstParamType(method)
		case hir.MemberBindingObjectField:
			if info := b.lowerer.objectTypes[node.Binding.ObjectName]; info != nil {
				return info.Fields[node.Member]
			}
		}
	}

	named, ok := b.resolvedExprType(node.Object).(*hir.NamedType)
	if !ok {
		return nil
	}
	info := b.lowerer.objectTypes[named.Name]
	if info == nil {
		return nil
	}
	if node.Member == "new" && b.exprIsObjectType(node.Object) {
		return info.Constructor
	}
	if method := info.Methods[node.Member]; method != nil {
		if b.exprIsObjectType(node.Object) {
			return method
		}
		return dropFirstParamType(method)
	}
	if fieldType := info.Fields[node.Member]; fieldType != nil {
		return fieldType
	}
	return nil
}

func (b *bodyBuilder) inferIndexType(node *hir.Index) hir.Type {
	if node == nil {
		return nil
	}
	objectType := b.resolvedExprType(node.Object)
	if itemType := listItemType(objectType); itemType != nil {
		return itemType
	}
	if valueType := dictValueType(objectType); valueType != nil {
		return valueType
	}
	if isNamedType(objectType, "dynamic") {
		return namedType("dynamic")
	}
	if isNamedType(objectType, "list") || isNamedType(objectType, "dict") {
		return namedType("dynamic")
	}
	if isNamedType(objectType, "string") {
		return namedType("string")
	}
	return nil
}

func (b *bodyBuilder) exprIsObjectType(expr hir.Expr) bool {
	switch node := expr.(type) {
	case *hir.Ident:
		return node.Binding != nil && node.Binding.Kind == hir.BindingObjectType
	case *hir.Member:
		named, ok := b.resolvedExprType(node).(*hir.NamedType)
		if !ok || b.lowerer == nil {
			return false
		}
		_, ok = b.lowerer.objectTypes[named.Name]
		return ok
	default:
		return false
	}
}

func (b *bodyBuilder) callReturnTypes(node *hir.Call) []hir.Type {
	if node == nil {
		return nil
	}
	if types := b.specialCallReturnTypes(node); len(types) > 0 {
		return types
	}
	calleeType, ok := b.resolvedExprType(node.Callee).(*hir.FuncType)
	if !ok || len(calleeType.Returns) == 0 {
		return nil
	}
	return append([]hir.Type(nil), calleeType.Returns...)
}

func (b *bodyBuilder) specialCallReturnTypes(node *hir.Call) []hir.Type {
	moduleName, callName, ok := specialCallIdentity(node.Callee)
	if !ok {
		return nil
	}
	switch {
	case callName == "map":
		if len(node.Args) >= 2 {
			if callbackType, ok := b.resolvedExprType(node.Args[1]).(*hir.FuncType); ok && len(callbackType.Returns) == 1 {
				return []hir.Type{genericType("list", callbackType.Returns[0])}
			}
		}
	case callName == "filter" || callName == "concat" || callName == "reversed" || callName == "sort":
		if len(node.Args) >= 1 {
			if typ := b.resolvedExprType(node.Args[0]); typ != nil {
				return []hir.Type{typ}
			}
		}
	case callName == "pop":
		if len(node.Args) >= 1 {
			if itemType := listItemType(b.resolvedExprType(node.Args[0])); itemType != nil {
				return []hir.Type{itemType}
			}
			if isNamedType(b.resolvedExprType(node.Args[0]), "list") {
				return []hir.Type{namedType("dynamic")}
			}
		}
	case callName == "removeat":
		if len(node.Args) >= 1 {
			if itemType := listItemType(b.resolvedExprType(node.Args[0])); itemType != nil {
				return []hir.Type{itemType}
			}
			if isNamedType(b.resolvedExprType(node.Args[0]), "list") {
				return []hir.Type{namedType("dynamic")}
			}
		}
	case callName == "enumerate":
		return []hir.Type{genericType("list", namedType("list"))}
	case callName == "keys":
		if len(node.Args) >= 1 {
			if keyType := dictKeyType(b.resolvedExprType(node.Args[0])); keyType != nil {
				return []hir.Type{genericType("list", keyType)}
			}
		}
	case callName == "values":
		if len(node.Args) >= 1 {
			if valueType := dictValueType(b.resolvedExprType(node.Args[0])); valueType != nil {
				return []hir.Type{genericType("list", valueType)}
			}
		}
	case callName == "items":
		return []hir.Type{genericType("list", namedType("list"))}
	case callName == "get":
		if len(node.Args) >= 1 {
			if moduleName == "state" {
				if itemType := cellItemType(b.resolvedExprType(node.Args[0])); itemType != nil {
					return []hir.Type{itemType}
				}
				return nil
			}
			if valueType := dictValueType(b.resolvedExprType(node.Args[0])); valueType != nil {
				return []hir.Type{valueType}
			}
			if itemType := cellItemType(b.resolvedExprType(node.Args[0])); itemType != nil {
				return []hir.Type{itemType}
			}
		}
	case moduleName == "state" && callName == "cell":
		if len(node.Args) >= 1 {
			if valueType := b.resolvedExprType(node.Args[0]); valueType != nil {
				return []hir.Type{genericType("cell", valueType)}
			}
		}
		return []hir.Type{namedType("cell")}
	case moduleName == "state" && (callName == "set" || callName == "update"):
		if len(node.Args) >= 1 {
			if itemType := cellItemType(b.resolvedExprType(node.Args[0])); itemType != nil {
				return []hir.Type{itemType}
			}
		}
	case callName == "abs":
		if len(node.Args) >= 1 {
			if argType := b.resolvedExprType(node.Args[0]); isNumericType(argType) {
				return []hir.Type{argType}
			}
		}
	case callName == "min" || callName == "max":
		if len(node.Args) >= 2 {
			leftType := b.resolvedExprType(node.Args[0])
			rightType := b.resolvedExprType(node.Args[1])
			if leftType != nil && rightType != nil && typeEqual(leftType, rightType) {
				if isNumericType(leftType) || isNamedType(leftType, "string") || isNamedType(leftType, "bool") {
					return []hir.Type{leftType}
				}
			}
		}
	}
	return nil
}

func specialCallIdentity(expr hir.Expr) (string, string, bool) {
	switch node := expr.(type) {
	case *hir.Ident:
		if node.Binding == nil {
			return "", "", false
		}
		switch node.Binding.Kind {
		case hir.BindingBuiltin:
			return "", node.Name, true
		case hir.BindingImported:
			if _, ok := stdlibCallableTypes[node.Binding.SourceModule]; ok {
				return node.Binding.SourceModule, node.Name, true
			}
		}
	case *hir.Member:
		if node.Binding == nil || node.Binding.Kind != hir.MemberBindingModuleValue {
			return "", "", false
		}
		if _, ok := stdlibCallableTypes[node.Binding.OwnerName]; ok {
			return node.Binding.OwnerName, node.Member, true
		}
	}
	return "", "", false
}

func callIdentity(expr hir.Expr) (string, string) {
	switch node := expr.(type) {
	case *hir.Ident:
		if node.Binding == nil {
			return "", ""
		}
		switch node.Binding.Kind {
		case hir.BindingImported:
			return node.Binding.SourceModule, node.Name
		case hir.BindingBuiltin, hir.BindingFunc:
			return "", node.Name
		}
	case *hir.Member:
		if node.Binding == nil {
			return "", ""
		}
		switch node.Binding.Kind {
		case hir.MemberBindingModuleValue:
			return node.Binding.OwnerName, node.Member
		case hir.MemberBindingObjectMethod:
			return node.Binding.ObjectName, node.Member
		case hir.MemberBindingObjectConstructor:
			return node.Binding.ObjectName, "new"
		}
	}
	return "", ""
}

func (b *bodyBuilder) iterableItemType(iterable hir.Expr) hir.Type {
	iterableType := b.resolvedExprType(iterable)
	if itemType := listItemType(iterableType); itemType != nil {
		return itemType
	}
	if isNamedType(iterableType, "list") {
		return namedType("dynamic")
	}
	return nil
}

func (b *bodyBuilder) rangeItemType(node *hir.ForRange) hir.Type {
	startType := b.resolvedExprType(node.Start)
	endType := b.resolvedExprType(node.End)
	if typeEqual(startType, endType) {
		return startType
	}
	return nil
}

func resultOKType(typ hir.Type) hir.Type {
	generic, ok := typ.(*hir.GenericType)
	if !ok || generic.Base != "result" || len(generic.Args) == 0 {
		return nil
	}
	return generic.Args[0]
}

func resultErrType(typ hir.Type) hir.Type {
	generic, ok := typ.(*hir.GenericType)
	if !ok || generic.Base != "result" || len(generic.Args) == 0 {
		return nil
	}
	if len(generic.Args) == 1 {
		return namedType("string")
	}
	candidate := generic.Args[1]
	for _, arg := range generic.Args[2:] {
		if !typeEqual(candidate, arg) {
			return nil
		}
	}
	return candidate
}

func (b *bodyBuilder) matchPatternType(subject hir.Expr, binding *hir.MatchPatternBinding) hir.Type {
	if binding == nil {
		return nil
	}
	switch binding.Kind {
	case hir.MatchPatternCapture:
		return b.inferExprType(subject)
	case hir.MatchPatternResultOk:
		return resultOKType(b.inferExprType(subject))
	case hir.MatchPatternResultErr:
		return resultErrType(b.inferExprType(subject))
	default:
		return nil
	}
}

func (b *bodyBuilder) lowerStmts(stmts []hir.Stmt, cur cursor) (cursor, error) {
	for _, stmt := range stmts {
		if !cur.reachable {
			cur = cursor{
				block:     b.newBlock().ID,
				reachable: true,
			}
		}
		var err error
		cur, err = b.lowerStmt(stmt, cur)
		if err != nil {
			return cursor{}, err
		}
	}
	return cur, nil
}

func (b *bodyBuilder) lowerStmt(stmt hir.Stmt, cur cursor) (cursor, error) {
	switch node := stmt.(type) {
	case *hir.Use:
		return cur, nil

	case *hir.Assign:
		b.ensureAssignSlots(node.Targets)
		for _, value := range node.Values {
			b.collectCaptureBindingsExpr(value)
		}
		targetPlaceIDs := b.lowerTargetPlaceList(node.Targets)
		valueIDs := b.lowerExprList(node.Values)
		b.contextualizeAssignValues(node.Targets, node.Values, valueIDs)
		switch {
		case len(node.Values) == 1:
			for idx, typ := range b.inferExprTypes(node.Values[0]) {
				if idx >= len(node.Targets) {
					break
				}
				b.rememberAssignTargetType(node.Targets[idx], typ)
			}
		case len(node.Targets) == len(node.Values):
			for idx, target := range node.Targets {
				inferred := b.inferExprTypes(node.Values[idx])
				if len(inferred) == 1 {
					b.rememberAssignTargetType(target, inferred[0])
				}
			}
		}
		if err := b.emitValues(cur.block, valueIDs); err != nil {
			return cursor{}, err
		}
		storeCount := len(targetPlaceIDs)
		if len(valueIDs) < storeCount {
			storeCount = len(valueIDs)
		}
		for idx := 0; idx < storeCount; idx++ {
			if err := b.emitStore(cur.block, targetPlaceIDs[idx], valueIDs[idx], node.Line); err != nil {
				return cursor{}, err
			}
		}
		if err := b.addOp(cur.block, &AssignOp{
			Targets:        append([]hir.Expr(nil), node.Targets...),
			TargetPlaceIDs: append([]int(nil), targetPlaceIDs...),
			Values:         append([]hir.Expr(nil), node.Values...),
			ValueIDs:       append([]int(nil), valueIDs...),
			Line:           node.Line,
		}); err != nil {
			return cursor{}, err
		}
		return cur, nil

	case *hir.Var:
		inferredType := node.Type
		if inferredType == nil && node.Value != nil {
			inferred := b.inferExprTypes(node.Value)
			if len(inferred) == 1 {
				inferredType = inferred[0]
			}
		}
		b.ensureSlot(node.Binding, SlotLocal, inferredType)
		b.rememberBindingType(node.Binding, inferredType)
		if node.Value != nil {
			b.collectCaptureBindingsExpr(node.Value)
		}
		varOp := lowerVar(b, node, inferredType)
		b.contextualizeValueType(varOp.ValueID, inferredType)
		if err := b.emitDeclare(cur.block, varOp.TargetPlaceID, varOp.ValueID, varOp.IsConst, varOp.IsUninit, node.Line); err != nil {
			return cursor{}, err
		}
		if err := b.addOp(cur.block, varOp); err != nil {
			return cursor{}, err
		}
		return cur, nil

	case *hir.VarBlock:
		if node.DefaultValue != nil {
			b.collectCaptureBindingsExpr(node.DefaultValue)
		}
		defaultValueID := 0
		if node.DefaultMode == "value" && node.DefaultValue != nil {
			defaultValueID = b.lowerExprSingle(node.DefaultValue)
		}
		decls := make([]*VarOp, 0, len(node.Decls))
		for _, decl := range node.Decls {
			inferredType := decl.Type
			if inferredType == nil && decl.Value != nil {
				inferred := b.inferExprTypes(decl.Value)
				if len(inferred) == 1 {
					inferredType = inferred[0]
				}
			}
			b.ensureSlot(decl.Binding, SlotLocal, inferredType)
			b.rememberBindingType(decl.Binding, inferredType)
			if decl.Value != nil {
				b.collectCaptureBindingsExpr(decl.Value)
			}
			varOp := lowerVar(b, decl, inferredType)
			if decl.Value == nil {
				switch node.DefaultMode {
				case "value":
					varOp.Value = node.DefaultValue
					varOp.ValueID = defaultValueID
					varOp.IsUninit = false
				case "zero":
					varOp.IsUninit = false
				}
			}
			if varOp.ValueID != 0 {
				b.contextualizeValueType(varOp.ValueID, inferredType)
			}
			if err := b.emitDeclare(cur.block, varOp.TargetPlaceID, varOp.ValueID, varOp.IsConst, varOp.IsUninit, decl.Line); err != nil {
				return cursor{}, err
			}
			decls = append(decls, varOp)
		}
		if err := b.addOp(cur.block, &VarBlockOp{
			Decls:        decls,
			DefaultMode:  node.DefaultMode,
			DefaultValue: node.DefaultValue,
			Line:         node.Line,
		}); err != nil {
			return cursor{}, err
		}
		return cur, nil

	case *hir.Return:
		for _, value := range node.Values {
			b.collectCaptureBindingsExpr(value)
		}
		valueIDs := b.lowerExprList(node.Values)
		b.contextualizeReturnValues(node.Values, valueIDs)
		if err := b.emitValues(cur.block, valueIDs); err != nil {
			return cursor{}, err
		}
		if err := b.setTerm(cur.block, &ReturnTerm{
			Values:   append([]hir.Expr(nil), node.Values...),
			ValueIDs: append([]int(nil), valueIDs...),
			Line:     node.Line,
		}); err != nil {
			return cursor{}, err
		}
		return cursor{reachable: false}, nil

	case *hir.Pass:
		return cur, nil

	case *hir.Leave:
		targets, ok := b.loopTargets[node.TargetID]
		if !ok {
			return cursor{}, fmt.Errorf("leave at line %d targets unknown loop id %d", node.Line, node.TargetID)
		}
		if err := b.setTerm(cur.block, &JumpTerm{Target: targets.leave, Line: node.Line}); err != nil {
			return cursor{}, err
		}
		return cursor{reachable: false}, nil

	case *hir.Next:
		targets, ok := b.loopTargets[node.TargetID]
		if !ok {
			return cursor{}, fmt.Errorf("next at line %d targets unknown loop id %d", node.Line, node.TargetID)
		}
		if err := b.setTerm(cur.block, &JumpTerm{Target: targets.next, Line: node.Line}); err != nil {
			return cursor{}, err
		}
		return cursor{reachable: false}, nil

	case *hir.If:
		b.collectCaptureBindingsExpr(node.Condition)
		for _, branch := range node.Elifs {
			b.collectCaptureBindingsExpr(branch.Condition)
		}
		return b.lowerIf(node, cur)

	case *hir.While:
		b.collectCaptureBindingsExpr(node.Condition)
		return b.lowerWhile(node, cur)

	case *hir.ForRange:
		b.ensureSlot(node.VarBinding, SlotLocal, b.rangeItemType(node))
		b.collectCaptureBindingsExpr(node.Start)
		b.collectCaptureBindingsExpr(node.End)
		if node.Step != nil {
			b.collectCaptureBindingsExpr(node.Step)
		}
		return b.lowerForRange(node, cur)

	case *hir.ForEach:
		b.ensureSlot(node.VarBinding, SlotLocal, b.iterableItemType(node.Iterable))
		if node.IndexBinding != nil {
			b.ensureSlot(node.IndexBinding, SlotLocal, &hir.NamedType{Name: "int", Line: node.Line})
		}
		b.collectCaptureBindingsExpr(node.Iterable)
		return b.lowerForEach(node, cur)

	case *hir.Match:
		b.collectCaptureBindingsExpr(node.Subject)
		for _, matchCase := range node.Cases {
			for idx, pattern := range matchCase.Patterns {
				if idx < len(matchCase.PatternBindings) {
					switch matchCase.PatternBindings[idx].Kind {
					case hir.MatchPatternCapture:
						if ident, ok := pattern.(*hir.Ident); ok {
							b.rememberBindingType(ident.Binding, b.matchPatternType(node.Subject, matchCase.PatternBindings[idx]))
						}
					case hir.MatchPatternResultOk:
						if okExpr, ok := pattern.(*hir.Ok); ok {
							if ident, ok := okExpr.Value.(*hir.Ident); ok {
								b.rememberBindingType(ident.Binding, b.matchPatternType(node.Subject, matchCase.PatternBindings[idx]))
							}
						}
					case hir.MatchPatternResultErr:
						if errExpr, ok := pattern.(*hir.Err); ok {
							if ident, ok := errExpr.Value.(*hir.Ident); ok {
								b.rememberBindingType(ident.Binding, b.matchPatternType(node.Subject, matchCase.PatternBindings[idx]))
							}
						}
					}
				}
			}
			b.ensurePatternSlots(matchCase.Patterns, matchCase.PatternBindings)
		}
		return b.lowerMatch(node, cur)

	case *hir.Parallel:
		branches := make([]*Body, 0, len(node.Body))
		for _, stmt := range node.Body {
			body, err := b.lowerer.lowerBody([]hir.Stmt{stmt})
			if err != nil {
				return cursor{}, err
			}
			branches = append(branches, body)
		}
		if node.ResultBinding != nil {
			b.ensureSlot(node.ResultBinding, SlotLocal, &hir.NamedType{Name: "list", Line: node.Line})
		}
		if err := b.addOp(cur.block, &ParallelOp{
			Branches:      branches,
			ResultVar:     node.ResultVar,
			ResultBinding: node.ResultBinding,
			AllowFail:     node.AllowFail,
			Line:          node.Line,
		}); err != nil {
			return cursor{}, err
		}
		if err := b.addInst(cur.block, &ParallelInst{
			Branches:      branches,
			ResultVar:     node.ResultVar,
			ResultBinding: node.ResultBinding,
			AllowFail:     node.AllowFail,
			Line:          node.Line,
		}); err != nil {
			return cursor{}, err
		}
		return cur, nil

	case *hir.Global:
		b.collectCaptureBindingsExpr(node.Value)
		targetPlaceID := b.lowerTargetPlace(&hir.Ident{
			Name:    node.Name,
			Binding: node.Target,
			Line:    node.Line,
		})
		valueID := b.lowerExprSingle(node.Value)
		if err := b.emitStore(cur.block, targetPlaceID, valueID, node.Line); err != nil {
			return cursor{}, err
		}
		if err := b.addOp(cur.block, &GlobalOp{
			Name:          node.Name,
			Value:         node.Value,
			ValueID:       valueID,
			Target:        node.Target,
			TargetPlaceID: targetPlaceID,
			Line:          node.Line,
		}); err != nil {
			return cursor{}, err
		}
		return cur, nil

	case *hir.Arena:
		if err := b.addOp(cur.block, &ArenaEnterOp{Name: node.Name, Line: node.Line}); err != nil {
			return cursor{}, err
		}
		next, err := b.lowerStmts(node.Body, cur)
		if err != nil {
			return cursor{}, err
		}
		if next.reachable {
			if err := b.addOp(next.block, &ArenaExitOp{Name: node.Name, Line: node.Line}); err != nil {
				return cursor{}, err
			}
		}
		return next, nil

	case *hir.Tag:
		return cur, nil

	case *hir.ExprStmt:
		b.collectCaptureBindingsExpr(node.Expr)
		valueIDs := b.lowerExprValues(node.Expr)
		if err := b.emitValues(cur.block, valueIDs); err != nil {
			return cursor{}, err
		}
		if err := b.addOp(cur.block, &ExprOp{
			Expr:     node.Expr,
			ValueIDs: append([]int(nil), valueIDs...),
			Line:     node.Line,
		}); err != nil {
			return cursor{}, err
		}
		return cur, nil

	case *hir.DeclStmt:
		decl, err := b.lowerer.lowerDecl(node.Decl)
		if err != nil {
			return cursor{}, err
		}
		if err := b.addOp(cur.block, &DeclOp{
			Decl: decl,
			Line: node.Decl.Pos(),
		}); err != nil {
			return cursor{}, err
		}
		return cur, nil

	default:
		return cursor{}, fmt.Errorf("unsupported HIR stmt %T", stmt)
	}
}

func (b *bodyBuilder) rememberAssignTargetType(target hir.Expr, typ hir.Type) {
	ident, ok := target.(*hir.Ident)
	if !ok || ident.Binding == nil {
		return
	}
	switch ident.Binding.Kind {
	case hir.BindingLocal, hir.BindingParam:
		b.rememberBindingType(ident.Binding, typ)
	}
}

func (b *bodyBuilder) contextualizeAssignValues(targets []hir.Expr, values []hir.Expr, valueIDs []int) {
	if len(values) == 1 && len(valueIDs) == 1 && len(targets) == 1 {
		b.contextualizeValueType(valueIDs[0], b.assignTargetType(targets[0]))
		return
	}
	if len(targets) != len(values) || len(values) != len(valueIDs) {
		return
	}
	for idx, target := range targets {
		b.contextualizeValueType(valueIDs[idx], b.assignTargetType(target))
	}
}

func (b *bodyBuilder) contextualizeReturnValues(_ []hir.Expr, valueIDs []int) {
	if len(b.returns) == 0 || len(b.returns) != len(valueIDs) {
		return
	}
	for idx, valueID := range valueIDs {
		b.contextualizeValueType(valueID, b.returns[idx])
	}
}

func (b *bodyBuilder) assignTargetType(target hir.Expr) hir.Type {
	ident, ok := target.(*hir.Ident)
	if !ok || ident.Binding == nil {
		return nil
	}
	return b.bindingType(ident.Binding)
}

func (b *bodyBuilder) contextualizeValueType(valueID int, want hir.Type) {
	if valueID == 0 || want == nil || b == nil || b.body == nil {
		return
	}
	value := b.body.Value(valueID)
	if value == nil {
		return
	}
	switch value.Kind {
	case ValueList:
		itemType := listItemType(want)
		if itemType == nil {
			return
		}
		for _, elementID := range value.Elements {
			b.contextualizeValueType(elementID, itemType)
		}
		value.Type = want
	case ValueDict:
		keyType := dictKeyType(want)
		valueType := dictValueType(want)
		if keyType == nil || valueType == nil {
			return
		}
		for _, entry := range value.Entries {
			b.contextualizeValueType(entry.Key, keyType)
			b.contextualizeValueType(entry.Value, valueType)
		}
		value.Type = want
	case ValueOk:
		expected := resultOKType(want)
		if expected == nil {
			return
		}
		b.contextualizeValueType(value.Operand, expected)
		operand := b.body.Value(value.Operand)
		if operand == nil {
			return
		}
		value.Type = want
	case ValueErr:
		expected := resultErrType(want)
		if expected == nil {
			return
		}
		b.contextualizeValueType(value.Operand, expected)
		operand := b.body.Value(value.Operand)
		if operand == nil {
			return
		}
		value.Type = want
	}
}

func lowerVar(b *bodyBuilder, node *hir.Var, inferredType hir.Type) *VarOp {
	if inferredType == nil {
		inferredType = node.Type
	}
	valueID := 0
	if b != nil && node.Value != nil {
		valueID = b.lowerExprSingle(node.Value)
	}
	targetPlaceID := 0
	if b != nil {
		targetPlaceID = b.lowerTargetPlace(&hir.Ident{
			Name:    node.Name,
			Binding: node.Binding,
			Line:    node.Line,
		})
	}
	return &VarOp{
		Name:          node.Name,
		Type:          inferredType,
		TargetPlaceID: targetPlaceID,
		Value:         node.Value,
		ValueID:       valueID,
		Binding:       node.Binding,
		IsConst:       node.IsConst,
		IsUninit:      node.IsUninit,
		Line:          node.Line,
	}
}

func (b *bodyBuilder) lowerIf(node *hir.If, cur cursor) (cursor, error) {
	join := b.newBlock().ID
	thenBlock := b.newBlock().ID

	elseBlock := join
	if len(node.Elifs) > 0 || len(node.ElseBody) > 0 {
		elseBlock = b.newBlock().ID
	}

	conditionValue := b.lowerExprSingle(node.Condition)
	if err := b.emitValue(cur.block, conditionValue); err != nil {
		return cursor{}, err
	}
	if err := b.setTerm(cur.block, &CondTerm{
		Condition:      node.Condition,
		ConditionValue: conditionValue,
		Then:           thenBlock,
		Else:           elseBlock,
		Line:           node.Line,
	}); err != nil {
		return cursor{}, err
	}

	thenCur, err := b.lowerStmts(node.Body, cursor{block: thenBlock, reachable: true})
	if err != nil {
		return cursor{}, err
	}
	if thenCur.reachable {
		if err := b.setTerm(thenCur.block, &JumpTerm{Target: join, Line: node.Line}); err != nil {
			return cursor{}, err
		}
	}

	nextCond := elseBlock
	for idx, branch := range node.Elifs {
		bodyBlock := b.newBlock().ID
		nextElse := join
		if idx < len(node.Elifs)-1 || len(node.ElseBody) > 0 {
			nextElse = b.newBlock().ID
		}

		conditionValue := b.lowerExprSingle(branch.Condition)
		if err := b.emitValue(nextCond, conditionValue); err != nil {
			return cursor{}, err
		}
		if err := b.setTerm(nextCond, &CondTerm{
			Condition:      branch.Condition,
			ConditionValue: conditionValue,
			Then:           bodyBlock,
			Else:           nextElse,
			Line:           branch.Condition.Pos(),
		}); err != nil {
			return cursor{}, err
		}

		bodyCur, err := b.lowerStmts(branch.Body, cursor{block: bodyBlock, reachable: true})
		if err != nil {
			return cursor{}, err
		}
		if bodyCur.reachable {
			if err := b.setTerm(bodyCur.block, &JumpTerm{Target: join, Line: branch.Condition.Pos()}); err != nil {
				return cursor{}, err
			}
		}
		nextCond = nextElse
	}

	if len(node.ElseBody) > 0 {
		elseCur, err := b.lowerStmts(node.ElseBody, cursor{block: nextCond, reachable: true})
		if err != nil {
			return cursor{}, err
		}
		if elseCur.reachable {
			if err := b.setTerm(elseCur.block, &JumpTerm{Target: join, Line: node.Line}); err != nil {
				return cursor{}, err
			}
		}
	}

	return cursor{block: join, reachable: true}, nil
}

func (b *bodyBuilder) lowerWhile(node *hir.While, cur cursor) (cursor, error) {
	condBlock := b.newBlock().ID
	exitBlock := b.newBlock().ID
	bodyBlock := b.newBlock().ID

	if err := b.setTerm(cur.block, &JumpTerm{Target: condBlock, Line: node.Line}); err != nil {
		return cursor{}, err
	}
	conditionValue := b.lowerExprSingle(node.Condition)
	if err := b.emitValue(condBlock, conditionValue); err != nil {
		return cursor{}, err
	}
	if err := b.setTerm(condBlock, &CondTerm{
		Condition:      node.Condition,
		ConditionValue: conditionValue,
		Then:           bodyBlock,
		Else:           exitBlock,
		Line:           node.Line,
	}); err != nil {
		return cursor{}, err
	}

	prev, hadPrev := b.loopTargets[node.LoopID]
	b.loopTargets[node.LoopID] = loopTarget{next: condBlock, leave: exitBlock}
	bodyCur, err := b.lowerStmts(node.Body, cursor{block: bodyBlock, reachable: true})
	if hadPrev {
		b.loopTargets[node.LoopID] = prev
	} else {
		delete(b.loopTargets, node.LoopID)
	}
	if err != nil {
		return cursor{}, err
	}

	if bodyCur.reachable {
		if err := b.setTerm(bodyCur.block, &JumpTerm{Target: condBlock, Line: node.Line}); err != nil {
			return cursor{}, err
		}
	}

	return cursor{block: exitBlock, reachable: true}, nil
}

func (b *bodyBuilder) lowerForRange(node *hir.ForRange, cur cursor) (cursor, error) {
	headerBlock := b.newBlock().ID
	exitBlock := b.newBlock().ID
	bodyBlock := b.newBlock().ID

	startValue := b.lowerExprSingle(node.Start)
	endValue := b.lowerExprSingle(node.End)
	stepValue := b.lowerExprSingle(node.Step)
	if err := b.emitValues(cur.block, []int{startValue, endValue, stepValue}); err != nil {
		return cursor{}, err
	}
	if err := b.setTerm(cur.block, &JumpTerm{Target: headerBlock, Line: node.Line}); err != nil {
		return cursor{}, err
	}
	if err := b.setTerm(headerBlock, &ForRangeTerm{
		Var:        node.Var,
		VarBinding: node.VarBinding,
		Start:      node.Start,
		StartValue: startValue,
		End:        node.End,
		EndValue:   endValue,
		Step:       node.Step,
		StepValue:  stepValue,
		Direction:  node.Direction,
		Name:       node.Name,
		LoopID:     node.LoopID,
		Body:       bodyBlock,
		Exit:       exitBlock,
		Line:       node.Line,
	}); err != nil {
		return cursor{}, err
	}

	prev, hadPrev := b.loopTargets[node.LoopID]
	b.loopTargets[node.LoopID] = loopTarget{next: headerBlock, leave: exitBlock}
	bodyCur, err := b.lowerStmts(node.Body, cursor{block: bodyBlock, reachable: true})
	if hadPrev {
		b.loopTargets[node.LoopID] = prev
	} else {
		delete(b.loopTargets, node.LoopID)
	}
	if err != nil {
		return cursor{}, err
	}

	if bodyCur.reachable {
		if err := b.setTerm(bodyCur.block, &JumpTerm{Target: headerBlock, Line: node.Line}); err != nil {
			return cursor{}, err
		}
	}

	return cursor{block: exitBlock, reachable: true}, nil
}

func (b *bodyBuilder) lowerForEach(node *hir.ForEach, cur cursor) (cursor, error) {
	headerBlock := b.newBlock().ID
	exitBlock := b.newBlock().ID
	bodyBlock := b.newBlock().ID

	iterableValue := b.lowerExprSingle(node.Iterable)
	if err := b.emitValue(cur.block, iterableValue); err != nil {
		return cursor{}, err
	}
	if err := b.setTerm(cur.block, &JumpTerm{Target: headerBlock, Line: node.Line}); err != nil {
		return cursor{}, err
	}
	if err := b.setTerm(headerBlock, &ForEachTerm{
		Var:           node.Var,
		VarBinding:    node.VarBinding,
		Iterable:      node.Iterable,
		IterableValue: iterableValue,
		IndexVar:      node.IndexVar,
		IndexBinding:  node.IndexBinding,
		Name:          node.Name,
		LoopID:        node.LoopID,
		Body:          bodyBlock,
		Exit:          exitBlock,
		Line:          node.Line,
	}); err != nil {
		return cursor{}, err
	}

	prev, hadPrev := b.loopTargets[node.LoopID]
	b.loopTargets[node.LoopID] = loopTarget{next: headerBlock, leave: exitBlock}
	bodyCur, err := b.lowerStmts(node.Body, cursor{block: bodyBlock, reachable: true})
	if hadPrev {
		b.loopTargets[node.LoopID] = prev
	} else {
		delete(b.loopTargets, node.LoopID)
	}
	if err != nil {
		return cursor{}, err
	}

	if bodyCur.reachable {
		if err := b.setTerm(bodyCur.block, &JumpTerm{Target: headerBlock, Line: node.Line}); err != nil {
			return cursor{}, err
		}
	}

	return cursor{block: exitBlock, reachable: true}, nil
}

func (b *bodyBuilder) lowerMatch(node *hir.Match, cur cursor) (cursor, error) {
	join := b.newBlock().ID
	arms := make([]MatchArm, 0, len(node.Cases))
	for _, matchCase := range node.Cases {
		arms = append(arms, MatchArm{
			Patterns:        append([]hir.Expr(nil), matchCase.Patterns...),
			PatternBindings: append([]*hir.MatchPatternBinding(nil), matchCase.PatternBindings...),
			Target:          b.newBlock().ID,
			Line:            matchCase.Line,
		})
	}

	elseBlock := join
	if len(node.ElseBody) > 0 {
		elseBlock = b.newBlock().ID
	}

	subjectValue := b.lowerExprSingle(node.Subject)
	if err := b.emitValue(cur.block, subjectValue); err != nil {
		return cursor{}, err
	}
	if err := b.setTerm(cur.block, &MatchTerm{
		Binding:      node.Binding,
		Subject:      node.Subject,
		SubjectValue: subjectValue,
		Cases:        arms,
		Else:         elseBlock,
		HasElse:      len(node.ElseBody) > 0,
		Line:         node.Line,
	}); err != nil {
		return cursor{}, err
	}

	for idx, matchCase := range node.Cases {
		caseCur, err := b.lowerStmts(matchCase.Body, cursor{block: arms[idx].Target, reachable: true})
		if err != nil {
			return cursor{}, err
		}
		if caseCur.reachable {
			if err := b.setTerm(caseCur.block, &JumpTerm{Target: join, Line: matchCase.Line}); err != nil {
				return cursor{}, err
			}
		}
	}

	if len(node.ElseBody) > 0 {
		elseCur, err := b.lowerStmts(node.ElseBody, cursor{block: elseBlock, reachable: true})
		if err != nil {
			return cursor{}, err
		}
		if elseCur.reachable {
			if err := b.setTerm(elseCur.block, &JumpTerm{Target: join, Line: node.Line}); err != nil {
				return cursor{}, err
			}
		}
	}

	return cursor{block: join, reachable: true}, nil
}
