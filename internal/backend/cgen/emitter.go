package cgen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Cass-ette/gwen-lang/internal/hir"
	"github.com/Cass-ette/gwen-lang/internal/mir"
)

func EmitProgram(program *mir.Program) (string, error) {
	if program == nil {
		return "", fmt.Errorf("cannot emit C for nil MIR program")
	}
	emitter := &emitter{
		program:             program,
		typeAliases:         map[string]hir.Type{},
		funcNames:           map[string]string{},
		funcBinding:         map[int]string{},
		funcByBinding:       map[int]*mir.Func{},
		funcByName:          map[string]*mir.Func{},
		moduleFuncs:         map[string]*mir.Func{},
		moduleValues:        map[string]string{},
		objectInfo:          map[string]*objectInfo{},
		globalSlots:         map[int]*mir.Slot{},
		funcTypeByKey:       map[string]string{},
		funcReturns:         map[string]string{},
		funcParams:          map[string][]string{},
		builtinClosures:     map[string]*builtinClosureSpec{},
		builtinClosureByKey: map[string]string{},
		boundMethodClosures: map[string]struct{}{},
		listByKey:           map[string]string{},
		listItems:           map[string]string{},
		listItemHIR:         map[string]hir.Type{},
		dictByKey:           map[string]string{},
		dictKeys:            map[string]string{},
		dictValues:          map[string]string{},
		dictKeyHIR:          map[string]hir.Type{},
		dictValueHIR:        map[string]hir.Type{},
		resultByKey:         map[string]string{},
		resultOK:            map[string]string{},
		resultErr:           map[string]string{},
		resultOKHIR:         map[string]hir.Type{},
		resultErrHIR:        map[string]hir.Type{},
		cellByKey:           map[string]string{},
		cellItems:           map[string]string{},
		cellItemHIR:         map[string]hir.Type{},
		tupleByKey:          map[string]string{},
		tupleFields:         map[string][]string{},
		tupleTypes:          map[string][]hir.Type{},
		scriptNames:         map[*mir.Script]string{},
		parallelByBody:      map[*mir.Body]*parallelBranchInfo{},
	}
	emitter.collectTopLevelGlobals()
	if err := emitter.collectProgram(); err != nil {
		return "", err
	}
	if err := emitter.collectTupleTypes(); err != nil {
		return "", err
	}
	if err := emitter.collectRuntimeTypes(); err != nil {
		return "", err
	}
	if err := emitter.collectBuiltinClosures(); err != nil {
		return "", err
	}
	if err := emitter.emitProgram(); err != nil {
		return "", err
	}
	return emitter.out.String(), nil
}

type emitter struct {
	program *mir.Program
	out     strings.Builder

	typeAliases         map[string]hir.Type
	funcs               []*mir.Func
	funcNames           map[string]string
	funcBinding         map[int]string
	funcByBinding       map[int]*mir.Func
	funcByName          map[string]*mir.Func
	moduleFuncs         map[string]*mir.Func
	moduleValues        map[string]string
	objects             []*mir.Object
	objectInfo          map[string]*objectInfo
	scripts             []*mir.Script
	scriptNames         map[*mir.Script]string
	userMain            *mir.Func
	globalSlots         map[int]*mir.Slot
	globalOrder         []int
	parallelBranches    []*parallelBranchInfo
	parallelByBody      map[*mir.Body]*parallelBranchInfo
	funcTypeByKey       map[string]string
	funcTypeOrder       []string
	funcReturns         map[string]string
	funcParams          map[string][]string
	builtinClosures     map[string]*builtinClosureSpec
	builtinClosureByKey map[string]string
	builtinClosureOrder []string
	boundMethodClosures map[string]struct{}

	listByKey    map[string]string
	listOrder    []string
	listItems    map[string]string
	listItemHIR  map[string]hir.Type
	dictByKey    map[string]string
	dictOrder    []string
	dictKeys     map[string]string
	dictValues   map[string]string
	dictKeyHIR   map[string]hir.Type
	dictValueHIR map[string]hir.Type
	resultByKey  map[string]string
	resultOrder  []string
	resultOK     map[string]string
	resultErr    map[string]string
	resultOKHIR  map[string]hir.Type
	resultErrHIR map[string]hir.Type
	cellByKey    map[string]string
	cellOrder    []string
	cellItems    map[string]string
	cellItemHIR  map[string]hir.Type
	tupleByKey   map[string]string
	tupleOrder   []string
	tupleFields  map[string][]string
	tupleTypes   map[string][]hir.Type
}

type builtinClosureSpec struct {
	ModuleName string
	Name       string
	Wrapper    string
	FuncType   *hir.FuncType
}

type objectInfo struct {
	name            string
	moduleName      string
	typeName        string
	cloneName       string
	constructorName string
	methodNames     map[string]string
	node            *mir.Object
}

type parallelBranchInfo struct {
	name               string
	execName           string
	ctxName            string
	entryName          string
	body               *mir.Body
	captureSlots       []*mir.Slot
	returnsResult      bool
	exprResultValueIDs []int
}

func (e *emitter) collectTopLevelGlobals() {
	var walkItems func(items []mir.Item)
	walkItems = func(items []mir.Item) {
		for _, item := range items {
			switch node := item.(type) {
			case *mir.Script:
				if node.Body == nil {
					continue
				}
				for _, slot := range node.Body.Slots {
					if slot == nil || slot.Kind != mir.SlotLocal || slot.BindingID == 0 {
						continue
					}
					if _, exists := e.globalSlots[slot.BindingID]; exists {
						continue
					}
					copied := *slot
					e.globalSlots[slot.BindingID] = &copied
					e.globalOrder = append(e.globalOrder, slot.BindingID)
				}
			case *mir.Module:
				walkItems(node.Items)
			}
		}
	}
	walkItems(e.program.Items)
	sort.Ints(e.globalOrder)
}

func (e *emitter) collectProgram() error {
	for _, item := range e.program.Items {
		if err := e.collectItem(item, ""); err != nil {
			return err
		}
	}
	if e.userMain != nil && len(e.userMain.Params) != 0 {
		return fmt.Errorf("unsupported entry main with parameters in C emitter")
	}
	return nil
}

func (e *emitter) collectItem(item mir.Item, moduleName string) error {
	switch node := item.(type) {
	case *mir.TypeAlias:
		e.typeAliases[node.Name] = node.Target
	case *mir.Func:
		if err := e.validateTopLevelFunc(node); err != nil {
			return err
		}
		name := "gwen_fn_" + sanitizeName(node.Name)
		if moduleName != "" {
			name = "gwen_mod_" + sanitizeName(moduleName) + "_" + sanitizeName(node.Name)
			e.moduleValues[moduleValueKey(moduleName, node.Name)] = name
			e.moduleFuncs[moduleValueKey(moduleName, node.Name)] = node
		} else {
			e.funcNames[node.Name] = name
			e.funcByName[node.Name] = node
			if node.Name == "main" {
				e.userMain = node
			}
		}
		if node.Binding != nil && node.Binding.ID != 0 {
			e.funcBinding[node.Binding.ID] = name
			e.funcByBinding[node.Binding.ID] = node
		}
		e.funcs = append(e.funcs, node)
		if err := e.collectParallelBranches(node.Body); err != nil {
			return fmt.Errorf("unsupported function %q: %w", node.Name, err)
		}
		if err := e.collectNestedDecls(node.Body, moduleName); err != nil {
			return fmt.Errorf("unsupported function %q: %w", node.Name, err)
		}
	case *mir.Script:
		if moduleName != "" {
			return fmt.Errorf("unsupported script inside module %q", moduleName)
		}
		if err := e.validateBody(node.Body); err != nil {
			return fmt.Errorf("unsupported top-level script at line %d: %w", node.Line, err)
		}
		if err := e.collectParallelBranches(node.Body); err != nil {
			return fmt.Errorf("unsupported top-level script at line %d: %w", node.Line, err)
		}
		e.scripts = append(e.scripts, node)
		e.scriptNames[node] = fmt.Sprintf("gwen_script_%d", len(e.scripts))
		if err := e.collectNestedDecls(node.Body, moduleName); err != nil {
			return fmt.Errorf("unsupported top-level script at line %d: %w", node.Line, err)
		}
	case *mir.Use:
		return nil
	case *mir.Module:
		for _, inner := range node.Items {
			if err := e.collectItem(inner, node.Name); err != nil {
				return err
			}
		}
	case *mir.Object:
		if _, exists := e.objectInfo[node.Name]; exists {
			return fmt.Errorf("duplicate object name %q in C emitter", node.Name)
		}
		typeName := "gwen_object_" + sanitizeName(node.Name)
		if moduleName != "" {
			typeName = "gwen_object_" + sanitizeName(moduleName) + "_" + sanitizeName(node.Name)
		}
		info := &objectInfo{
			name:        node.Name,
			moduleName:  moduleName,
			typeName:    typeName,
			cloneName:   typeName + "_clone",
			node:        node,
			methodNames: map[string]string{},
		}
		if node.Constructor != nil {
			info.constructorName = typeName + "_new"
		}
		for _, method := range node.Methods {
			info.methodNames[method.Name] = typeName + "_" + sanitizeName(method.Name)
		}
		e.objectInfo[node.Name] = info
		if err := e.validateObject(node); err != nil {
			delete(e.objectInfo, node.Name)
			if moduleName != "" {
				return fmt.Errorf("unsupported object %q in module %q in C emitter: %w", node.Name, moduleName, err)
			}
			return fmt.Errorf("unsupported object %q in C emitter: %w", node.Name, err)
		}
		e.objects = append(e.objects, node)
		if node.Constructor != nil {
			if err := e.collectNestedDecls(node.Constructor.Body, moduleName); err != nil {
				return fmt.Errorf("unsupported object %q constructor in C emitter: %w", node.Name, err)
			}
		}
		for _, method := range node.Methods {
			if err := e.collectNestedDecls(method.Body, moduleName); err != nil {
				return fmt.Errorf("unsupported object %q method %q in C emitter: %w", node.Name, method.Name, err)
			}
		}
	default:
		return fmt.Errorf("unsupported top-level MIR item %T", item)
	}
	return nil
}

func (e *emitter) collectNestedDecls(body *mir.Body, moduleName string) error {
	if body == nil {
		return nil
	}
	for _, block := range body.Blocks {
		for _, op := range block.Ops {
			declOp, ok := op.(*mir.DeclOp)
			if !ok || declOp == nil || declOp.Decl == nil {
				continue
			}
			switch decl := declOp.Decl.(type) {
			case *mir.Func:
				if err := e.collectNestedFunc(decl, moduleName); err != nil {
					return err
				}
			case *mir.TypeAlias:
				continue
			default:
				return fmt.Errorf("nested declarations of type %T are not supported yet", declOp.Decl)
			}
		}
	}
	return nil
}

func (e *emitter) collectNestedFunc(fn *mir.Func, moduleName string) error {
	if fn == nil {
		return fmt.Errorf("nil nested function")
	}
	if err := e.validateNestedFunc(fn); err != nil {
		return err
	}
	if fn.Binding == nil || fn.Binding.ID == 0 {
		return fmt.Errorf("nested function %q is missing binding metadata", fn.Name)
	}
	if _, exists := e.funcBinding[fn.Binding.ID]; exists {
		return nil
	}
	name := fmt.Sprintf("gwen_nested_fn_%d_%s", fn.Binding.ID, sanitizeName(fn.Name))
	if moduleName != "" {
		name = fmt.Sprintf("gwen_nested_mod_%s_fn_%d_%s", sanitizeName(moduleName), fn.Binding.ID, sanitizeName(fn.Name))
	}
	e.funcBinding[fn.Binding.ID] = name
	e.funcByBinding[fn.Binding.ID] = fn
	e.funcByName[fn.Name] = fn
	e.funcs = append(e.funcs, fn)
	if err := e.collectParallelBranches(fn.Body); err != nil {
		return fmt.Errorf("unsupported nested function %q: %w", fn.Name, err)
	}
	if err := e.collectNestedDecls(fn.Body, moduleName); err != nil {
		return fmt.Errorf("unsupported nested function %q: %w", fn.Name, err)
	}
	return nil
}

func (e *emitter) validateObject(obj *mir.Object) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	for _, field := range obj.Fields {
		if field == nil {
			continue
		}
		if _, err := e.cType(field.Type); err != nil {
			return fmt.Errorf("unsupported field %q: %w", field.Name, err)
		}
	}
	if obj.Constructor != nil {
		for _, param := range obj.Constructor.Params {
			if _, err := e.cType(param.Type); err != nil {
				return fmt.Errorf("unsupported constructor parameter type for %q: %w", param.Name, err)
			}
		}
		if _, err := e.signatureReturnType(obj.Constructor.Returns); err != nil {
			return fmt.Errorf("unsupported constructor return type: %w", err)
		}
		if err := e.validateBody(obj.Constructor.Body); err != nil {
			return fmt.Errorf("unsupported constructor body: %w", err)
		}
		if err := e.collectParallelBranches(obj.Constructor.Body); err != nil {
			return fmt.Errorf("unsupported constructor body: %w", err)
		}
	}
	for _, method := range obj.Methods {
		for _, param := range method.Params {
			if _, err := e.cType(param.Type); err != nil {
				return fmt.Errorf("unsupported method parameter type for %q: %w", param.Name, err)
			}
		}
		if _, err := e.signatureReturnType(method.Returns); err != nil {
			return fmt.Errorf("unsupported method return type for %q: %w", method.Name, err)
		}
		if err := e.validateBody(method.Body); err != nil {
			return fmt.Errorf("unsupported method %q body: %w", method.Name, err)
		}
		if err := e.collectParallelBranches(method.Body); err != nil {
			return fmt.Errorf("unsupported method %q body: %w", method.Name, err)
		}
	}
	return nil
}

func (e *emitter) validateTopLevelFunc(fn *mir.Func) error {
	if err := e.validateBodyWithOptions(fn.Body, true); err != nil {
		return fmt.Errorf("unsupported function %q: %w", fn.Name, err)
	}
	for _, param := range fn.Params {
		if _, err := e.cType(param.Type); err != nil {
			return fmt.Errorf("unsupported parameter type for %q: %w", fn.Name, err)
		}
	}
	if _, err := e.signatureReturnType(fn.Returns); err != nil {
		return fmt.Errorf("unsupported return type for %q: %w", fn.Name, err)
	}
	return nil
}

func (e *emitter) validateNestedFunc(fn *mir.Func) error {
	if err := e.validateBodyWithOptions(fn.Body, true); err != nil {
		return fmt.Errorf("unsupported function %q: %w", fn.Name, err)
	}
	for _, param := range fn.Params {
		if _, err := e.cType(param.Type); err != nil {
			return fmt.Errorf("unsupported parameter type for %q: %w", fn.Name, err)
		}
	}
	if _, err := e.signatureReturnType(fn.Returns); err != nil {
		return fmt.Errorf("unsupported return type for %q: %w", fn.Name, err)
	}
	return nil
}

func (e *emitter) validateBody(body *mir.Body) error {
	return e.validateBodyWithOptions(body, false)
}

func (e *emitter) validateBodyWithOptions(body *mir.Body, allowCaptures bool) error {
	if body == nil {
		return fmt.Errorf("nil body")
	}
	for _, slot := range body.Slots {
		if !allowCaptures && slot.Kind == mir.SlotCapture && !e.isGlobalBinding(slot.BindingID) {
			return fmt.Errorf("capture slots are not supported yet")
		}
		if slot.Type != nil {
			if _, err := e.cType(slot.Type); err != nil {
				return fmt.Errorf("unsupported slot type %q: %w", slot.Name, err)
			}
		}
	}
	for _, block := range body.Blocks {
		for _, op := range block.Ops {
			switch node := op.(type) {
			case *mir.AssignOp, *mir.VarOp, *mir.VarBlockOp, *mir.ExprOp, *mir.ArenaEnterOp, *mir.ArenaExitOp:
			case *mir.GlobalOp:
				_ = node
			case *mir.DeclOp:
				switch decl := node.Decl.(type) {
				case *mir.Func:
					if err := e.validateNestedFunc(decl); err != nil {
						return err
					}
				case *mir.TypeAlias:
				default:
					return fmt.Errorf("nested declarations of type %T are not supported yet", node.Decl)
				}
			case *mir.ParallelOp:
			default:
				return fmt.Errorf("unsupported MIR op %T", node)
			}
		}
		switch term := block.Term.(type) {
		case nil, *mir.JumpTerm, *mir.CondTerm, *mir.ReturnTerm, *mir.StopTerm, *mir.ForRangeTerm, *mir.ForEachTerm, *mir.MatchTerm:
		default:
			return fmt.Errorf("unsupported MIR terminator %T", term)
		}
	}
	return nil
}

func (e *emitter) collectParallelBranches(body *mir.Body) error {
	if body == nil {
		return nil
	}
	for _, block := range body.Blocks {
		for _, op := range block.Ops {
			parallel, ok := op.(*mir.ParallelOp)
			if ok {
				for _, branch := range parallel.Branches {
					if branch == nil {
						return fmt.Errorf("nil parallel branch")
					}
					if _, ok := e.parallelByBody[branch]; ok {
						continue
					}
					if err := e.validateBodyWithOptions(branch, true); err != nil {
						return fmt.Errorf("parallel branch: %w", err)
					}
					info := &parallelBranchInfo{
						name:               fmt.Sprintf("gwen_parallel_%d", len(e.parallelBranches)+1),
						execName:           fmt.Sprintf("gwen_parallel_exec_%d", len(e.parallelBranches)+1),
						ctxName:            fmt.Sprintf("gwen_parallel_ctx_%d", len(e.parallelBranches)+1),
						entryName:          fmt.Sprintf("gwen_parallel_entry_%d", len(e.parallelBranches)+1),
						body:               branch,
						captureSlots:       parallelCaptureSlots(e, branch),
						returnsResult:      parallel.ResultBinding != nil || parallel.ResultVar != "",
						exprResultValueIDs: parallelExprResultValueIDs(branch),
					}
					e.parallelBranches = append(e.parallelBranches, info)
					e.parallelByBody[branch] = info
					if err := e.collectParallelBranches(branch); err != nil {
						return err
					}
				}
				continue
			}
			declOp, ok := op.(*mir.DeclOp)
			if ok {
				if fn, ok := declOp.Decl.(*mir.Func); ok {
					if err := e.collectParallelBranches(fn.Body); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func captureSlots(body *mir.Body) []*mir.Slot {
	if body == nil {
		return nil
	}
	slots := make([]*mir.Slot, 0)
	for _, slot := range body.Slots {
		if slot != nil && slot.Kind == mir.SlotCapture {
			slots = append(slots, slot)
		}
	}
	return slots
}

func nonGlobalCaptureSlots(e *emitter, body *mir.Body) []*mir.Slot {
	if e == nil {
		return captureSlots(body)
	}
	slots := captureSlots(body)
	if len(slots) == 0 {
		return nil
	}
	filtered := make([]*mir.Slot, 0, len(slots))
	for _, slot := range slots {
		if slot == nil || e.isGlobalBinding(slot.BindingID) {
			continue
		}
		filtered = append(filtered, slot)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func parallelCaptureSlots(e *emitter, body *mir.Body) []*mir.Slot {
	return captureSlots(body)
}

func parallelExprResultValueIDs(body *mir.Body) []int {
	if body == nil || len(body.Blocks) != 1 {
		return nil
	}
	entry := body.Block(body.Entry)
	if entry == nil || len(entry.Ops) != 1 {
		return nil
	}
	expr, ok := entry.Ops[0].(*mir.ExprOp)
	if !ok || len(expr.ValueIDs) == 0 {
		return nil
	}
	return append([]int(nil), expr.ValueIDs...)
}

func functionCaptureSlots(e *emitter, body *mir.Body) []*mir.Slot {
	return nonGlobalCaptureSlots(e, body)
}

func refCaptureBindingIDSet(body *mir.Body) map[int]struct{} {
	out := map[int]struct{}{}
	if body == nil {
		return out
	}
	for _, block := range body.Blocks {
		for _, op := range block.Ops {
			switch node := op.(type) {
			case *mir.GlobalOp:
				if node != nil && node.Target != nil && node.Target.ID != 0 {
					out[node.Target.ID] = struct{}{}
				}
			case *mir.DeclOp:
				if fn, ok := node.Decl.(*mir.Func); ok {
					for bindingID := range refCaptureBindingIDSet(fn.Body) {
						out[bindingID] = struct{}{}
					}
				}
			}
		}
	}
	return out
}

func hasCaptureBinding(bindings map[int]struct{}, bindingID int) bool {
	if bindings == nil {
		return false
	}
	_, ok := bindings[bindingID]
	return ok
}

func captureSlotIDSet(e *emitter, body *mir.Body) map[int]struct{} {
	slots := functionCaptureSlots(e, body)
	if len(slots) == 0 {
		return nil
	}
	out := map[int]struct{}{}
	for _, slot := range slots {
		if slot != nil {
			out[slot.ID] = struct{}{}
		}
	}
	return out
}

func refCaptureSlotIDSet(e *emitter, body *mir.Body) map[int]struct{} {
	slots := functionCaptureSlots(e, body)
	if len(slots) == 0 {
		return nil
	}
	refBindings := refCaptureBindingIDSet(body)
	out := map[int]struct{}{}
	for _, slot := range slots {
		if slot != nil && hasCaptureBinding(refBindings, slot.BindingID) {
			out[slot.ID] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (e *emitter) collectTupleTypes() error {
	for _, fn := range e.funcs {
		if len(fn.Returns) > 1 {
			if _, err := e.tupleTypeName(fn.Returns); err != nil {
				return err
			}
		}
		if err := e.collectBodyTupleTypes(fn.Body); err != nil {
			return fmt.Errorf("function %q: %w", fn.Name, err)
		}
	}
	for _, script := range e.scripts {
		if err := e.collectBodyTupleTypes(script.Body); err != nil {
			return fmt.Errorf("script at line %d: %w", script.Line, err)
		}
	}
	for _, branch := range e.parallelBranches {
		if err := e.collectBodyTupleTypes(branch.body); err != nil {
			return fmt.Errorf("parallel branch %q: %w", branch.name, err)
		}
		if err := e.collectParallelBranchTupleTypes(branch); err != nil {
			return fmt.Errorf("parallel branch %q: %w", branch.name, err)
		}
	}
	for _, obj := range e.objects {
		if obj.Constructor != nil {
			if len(obj.Constructor.Returns) > 1 {
				if _, err := e.tupleTypeName(obj.Constructor.Returns); err != nil {
					return err
				}
			}
			if err := e.collectBodyTupleTypes(obj.Constructor.Body); err != nil {
				return fmt.Errorf("object %q constructor: %w", obj.Name, err)
			}
		}
		for _, method := range obj.Methods {
			if len(method.Returns) > 1 {
				if _, err := e.tupleTypeName(method.Returns); err != nil {
					return err
				}
			}
			if err := e.collectBodyTupleTypes(method.Body); err != nil {
				return fmt.Errorf("object %q method %q: %w", obj.Name, method.Name, err)
			}
		}
	}
	return nil
}

func (e *emitter) collectBodyTupleTypes(body *mir.Body) error {
	if body == nil {
		return nil
	}
	for _, value := range body.Values {
		if value == nil {
			continue
		}
		if value.Kind == mir.ValueCall && len(value.ReturnTypes) > 1 {
			if _, err := e.tupleTypeName(value.ReturnTypes); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *emitter) collectParallelBranchTupleTypes(info *parallelBranchInfo) error {
	if info == nil || !info.returnsResult || info.body == nil {
		return nil
	}
	if err := e.collectTupleForValueIDs(info.body, info.exprResultValueIDs); err != nil {
		return err
	}
	for _, block := range info.body.Blocks {
		ret, ok := block.Term.(*mir.ReturnTerm)
		if !ok {
			continue
		}
		if err := e.collectTupleForValueIDs(info.body, ret.ValueIDs); err != nil {
			return err
		}
	}
	return nil
}

func (e *emitter) collectTupleForValueIDs(body *mir.Body, valueIDs []int) error {
	if body == nil || len(valueIDs) <= 1 {
		return nil
	}
	types := make([]hir.Type, 0, len(valueIDs))
	for _, valueID := range valueIDs {
		value := body.Value(valueID)
		if value == nil {
			return fmt.Errorf("unknown MIR value %d", valueID)
		}
		types = append(types, value.Type)
	}
	_, err := e.tupleTypeName(types)
	return err
}

func (e *emitter) collectRuntimeTypes() error {
	for _, fn := range e.funcs {
		if _, err := e.funcTypeName(signatureType(fn.Params, fn.Returns)); err != nil {
			return err
		}
		for _, param := range fn.Params {
			if _, err := e.cType(param.Type); err != nil {
				return err
			}
		}
		for _, ret := range fn.Returns {
			if _, err := e.cType(ret); err != nil {
				return err
			}
		}
		if err := e.collectBodyTypes(fn.Body); err != nil {
			return fmt.Errorf("function %q: %w", fn.Name, err)
		}
	}
	for _, script := range e.scripts {
		if err := e.collectBodyTypes(script.Body); err != nil {
			return fmt.Errorf("script at line %d: %w", script.Line, err)
		}
	}
	for _, branch := range e.parallelBranches {
		if err := e.collectBodyTypes(branch.body); err != nil {
			return fmt.Errorf("parallel branch %q: %w", branch.name, err)
		}
		for _, slot := range branch.captureSlots {
			if slot == nil || slot.Type == nil {
				continue
			}
			if _, err := e.cType(slot.Type); err != nil {
				return fmt.Errorf("parallel branch %q capture %q: %w", branch.name, slot.Name, err)
			}
		}
	}
	for _, obj := range e.objects {
		for _, field := range obj.Fields {
			if field == nil {
				continue
			}
			if _, err := e.cType(field.Type); err != nil {
				return fmt.Errorf("object %q field %q: %w", obj.Name, field.Name, err)
			}
		}
		if obj.Constructor != nil {
			if _, err := e.funcTypeName(signatureType(obj.Constructor.Params, obj.Constructor.Returns)); err != nil {
				return fmt.Errorf("object %q constructor function type: %w", obj.Name, err)
			}
			for _, param := range obj.Constructor.Params {
				if _, err := e.cType(param.Type); err != nil {
					return fmt.Errorf("object %q constructor param %q: %w", obj.Name, param.Name, err)
				}
			}
			for _, ret := range obj.Constructor.Returns {
				if _, err := e.cType(ret); err != nil {
					return fmt.Errorf("object %q constructor return: %w", obj.Name, err)
				}
			}
			if err := e.collectBodyTypes(obj.Constructor.Body); err != nil {
				return fmt.Errorf("object %q constructor: %w", obj.Name, err)
			}
		}
		for _, method := range obj.Methods {
			if _, err := e.funcTypeName(signatureType(method.Params, method.Returns)); err != nil {
				return fmt.Errorf("object %q method %q function type: %w", obj.Name, method.Name, err)
			}
			for _, param := range method.Params {
				if _, err := e.cType(param.Type); err != nil {
					return fmt.Errorf("object %q method %q param %q: %w", obj.Name, method.Name, param.Name, err)
				}
			}
			for _, ret := range method.Returns {
				if _, err := e.cType(ret); err != nil {
					return fmt.Errorf("object %q method %q return: %w", obj.Name, method.Name, err)
				}
			}
			if err := e.collectBodyTypes(method.Body); err != nil {
				return fmt.Errorf("object %q method %q: %w", obj.Name, method.Name, err)
			}
		}
	}
	return nil
}

func (e *emitter) collectBuiltinClosures() error {
	for _, fn := range e.funcs {
		if err := e.collectBuiltinClosuresInBody(fn.Body); err != nil {
			return fmt.Errorf("function %q builtin closures: %w", fn.Name, err)
		}
	}
	for _, script := range e.scripts {
		if err := e.collectBuiltinClosuresInBody(script.Body); err != nil {
			return fmt.Errorf("script at line %d builtin closures: %w", script.Line, err)
		}
	}
	for _, branch := range e.parallelBranches {
		if err := e.collectBuiltinClosuresInBody(branch.body); err != nil {
			return fmt.Errorf("parallel branch %q builtin closures: %w", branch.name, err)
		}
	}
	for _, obj := range e.objects {
		if obj.Constructor != nil {
			if err := e.collectBuiltinClosuresInBody(obj.Constructor.Body); err != nil {
				return fmt.Errorf("object %q constructor builtin closures: %w", obj.Name, err)
			}
		}
		for _, method := range obj.Methods {
			if err := e.collectBuiltinClosuresInBody(method.Body); err != nil {
				return fmt.Errorf("object %q method %q builtin closures: %w", obj.Name, method.Name, err)
			}
		}
	}
	return nil
}

func (e *emitter) collectBuiltinClosuresInBody(body *mir.Body) error {
	if body == nil {
		return nil
	}
	directCallees := map[int]struct{}{}
	for _, value := range body.Values {
		if value == nil || value.Kind != mir.ValueCall || value.Callee == 0 {
			continue
		}
		callee := body.Value(value.Callee)
		if callee == nil {
			continue
		}
		if _, _, ok := builtinValueIdentity(callee); ok {
			directCallees[value.Callee] = struct{}{}
		}
		if moduleName, callName, ok := builtinValueIdentity(callee); ok && callName == "sort" && (moduleName == "" || moduleName == "list") && len(value.Args) >= 2 {
			directCallees[value.Args[1]] = struct{}{}
		}
	}
	for _, value := range body.Values {
		if value == nil {
			continue
		}
		funcType, ok := e.resolveType(value.Type).(*hir.FuncType)
		if !ok {
			continue
		}
		if _, skip := directCallees[value.ID]; skip {
			continue
		}
		switch value.Kind {
		case mir.ValueBindingRef:
			if value.Binding == nil {
				continue
			}
			switch value.Binding.Kind {
			case hir.BindingBuiltin:
				if _, err := e.ensureBuiltinClosure("", value.Binding.Name, funcType); err != nil {
					return err
				}
			case hir.BindingImported:
				if isStdlibModuleName(value.Binding.SourceModule) {
					if _, err := e.ensureBuiltinClosure(value.Binding.SourceModule, value.Binding.Name, funcType); err != nil {
						return err
					}
				}
			}
		case mir.ValueMember:
			if value.MemberBinding == nil || value.MemberBinding.Kind != hir.MemberBindingModuleValue {
				continue
			}
			if isStdlibModuleName(value.MemberBinding.OwnerName) {
				if _, err := e.ensureBuiltinClosure(value.MemberBinding.OwnerName, value.Member, funcType); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (e *emitter) collectBodyTypes(body *mir.Body) error {
	if body == nil {
		return nil
	}
	for _, slot := range body.Slots {
		if slot == nil || slot.Type == nil {
			continue
		}
		if _, err := e.cType(slot.Type); err != nil {
			return err
		}
	}
	for _, value := range body.Values {
		if value == nil {
			continue
		}
		if err := e.collectValueTypes(body, value); err != nil {
			return err
		}
	}
	return nil
}

func (e *emitter) emitProgram() error {
	e.writeLine("#include <stdbool.h>")
	e.writeLine("#include <arpa/inet.h>")
	e.writeLine("#include <ctype.h>")
	e.writeLine("#include <dirent.h>")
	e.writeLine("#include <errno.h>")
	e.writeLine("#include <limits.h>")
	e.writeLine("#include <math.h>")
	e.writeLine("#include <netdb.h>")
	e.writeLine("#include <netinet/in.h>")
	e.writeLine("#include <pthread.h>")
	e.writeLine("#include <stdint.h>")
	e.writeLine("#include <setjmp.h>")
	e.writeLine("#include <stdlib.h>")
	e.writeLine("#include <stdio.h>")
	e.writeLine("#include <strings.h>")
	e.writeLine("#include <string.h>")
	e.writeLine("#include <sys/socket.h>")
	e.writeLine("#include <sys/time.h>")
	e.writeLine("#include <sys/wait.h>")
	e.writeLine("#include <sys/types.h>")
	e.writeLine("#include <time.h>")
	e.writeLine("#include <unistd.h>")
	e.writeLine("")

	if err := e.emitPrelude(); err != nil {
		return err
	}
	if err := e.emitMoneyHelpers(); err != nil {
		return err
	}
	if err := e.emitObjectForwards(); err != nil {
		return err
	}
	if err := e.emitTupleForwards(); err != nil {
		return err
	}
	if err := e.emitFuncForwards(); err != nil {
		return err
	}
	if err := e.emitAggregateForwards(); err != nil {
		return err
	}
	if err := e.emitFuncTypes(); err != nil {
		return err
	}
	if err := e.emitListTypes(); err != nil {
		return err
	}
	if err := e.emitListHelperPrototypes(); err != nil {
		return err
	}
	if err := e.emitDictTypes(); err != nil {
		return err
	}
	if err := e.emitDictHelperPrototypes(); err != nil {
		return err
	}
	if err := e.emitCellTypes(); err != nil {
		return err
	}
	if err := e.emitCellHelperPrototypes(); err != nil {
		return err
	}
	if err := e.emitResultTypes(); err != nil {
		return err
	}
	if err := e.emitObjectTypes(); err != nil {
		return err
	}
	if err := e.emitIOHelpers(); err != nil {
		return err
	}
	if err := e.emitOSTimeHelpers(); err != nil {
		return err
	}
	if err := e.emitJSONHelpers(); err != nil {
		return err
	}
	if err := e.emitTupleTypes(); err != nil {
		return err
	}
	if err := e.emitFuncHelpers(); err != nil {
		return err
	}
	if err := e.emitListHelpers(); err != nil {
		return err
	}
	if err := e.emitDictHelpers(); err != nil {
		return err
	}
	if err := e.emitCellHelpers(); err != nil {
		return err
	}
	if err := e.emitParallelHelpers(); err != nil {
		return err
	}
	if err := e.emitGlobals(); err != nil {
		return err
	}
	if err := e.emitPrototypes(); err != nil {
		return err
	}
	if err := e.emitClosureHelpers(); err != nil {
		return err
	}
	for _, fn := range e.funcs {
		if err := e.emitFunc(fn); err != nil {
			return err
		}
	}
	for _, obj := range e.objects {
		if err := e.emitObject(obj); err != nil {
			return err
		}
	}
	for _, branch := range e.parallelBranches {
		if err := e.emitParallelBranch(branch); err != nil {
			return err
		}
	}
	for _, script := range e.scripts {
		if err := e.emitScript(script); err != nil {
			return err
		}
	}
	return e.emitEntryPoint()
}

func (e *emitter) emitPrelude() error {
	e.writeLine("typedef struct gwen_error_frame gwen_error_frame;")
	e.writeLine("")
	e.writeLine("#if defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L")
	e.writeLine("#define GWEN_THREAD_LOCAL _Thread_local")
	e.writeLine("#elif defined(__GNUC__) || defined(__clang__)")
	e.writeLine("#define GWEN_THREAD_LOCAL __thread")
	e.writeLine("#else")
	e.writeLine("#define GWEN_THREAD_LOCAL")
	e.writeLine("#endif")
	e.writeLine("")
	e.writeLine("struct gwen_error_frame {")
	e.writeLine("  jmp_buf jump;")
	e.writeLine("  gwen_error_frame *prev;")
	e.writeLine("  const char *message;")
	e.writeLine("};")
	e.writeLine("")
	e.writeLine("static GWEN_THREAD_LOCAL gwen_error_frame *gwen_error_frame_current = NULL;")
	e.writeLine("")
	e.writeLine("static const char *gwen_runtime_error_copy(const char *message) {")
	e.writeLine("  const char *safe = message != NULL ? message : \"runtime error\";")
	e.writeLine("  size_t len = strlen(safe);")
	e.writeLine("  char *copy = (char *)malloc(len + 1U);")
	e.writeLine("  if (copy == NULL) {")
	e.writeLine("    fputs(\"runtime error: out of memory copying error message\", stderr);")
	e.writeLine("    fputc('\\n', stderr);")
	e.writeLine("    exit(1);")
	e.writeLine("  }")
	e.writeLine("  memcpy(copy, safe, len + 1U);")
	e.writeLine("  return copy;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_runtime_error(const char *message) {")
	e.writeLine("  const char *safe = message != NULL ? message : \"runtime error\";")
	e.writeLine("  if (gwen_error_frame_current != NULL) {")
	e.writeLine("    gwen_error_frame_current->message = gwen_runtime_error_copy(safe);")
	e.writeLine("    longjmp(gwen_error_frame_current->jump, 1);")
	e.writeLine("  }")
	e.writeLine("  fputs(safe, stderr);")
	e.writeLine("  fputc('\\n', stderr);")
	e.writeLine("  exit(1);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_pthread_require(int code, const char *action) {")
	e.writeLine("  if (code == 0) return;")
	e.writeLine("  {")
	e.writeLine("    char message[256];")
	e.writeLine("    const char *detail = strerror(code);")
	e.writeLine("    snprintf(message, sizeof(message), \"runtime error: %s failed: %s\", action != NULL ? action : \"pthread call\", detail != NULL ? detail : \"pthread error\");")
	e.writeLine("    gwen_runtime_error(message);")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static int gwen_cli_argc = 0;")
	e.writeLine("static char **gwen_cli_argv = NULL;")
	e.writeLine("")
	e.writeLine("static void gwen_write_int(long long value) {")
	e.writeLine("  printf(\"%lld\", value);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_write_float(double value) {")
	e.writeLine("  printf(\"%.15g\", value);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_write_bool(bool value) {")
	e.writeLine("  fputs(value ? \"true\" : \"false\", stdout);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_write_string(const char *value) {")
	e.writeLine("  fputs(value != NULL ? value : \"\", stdout);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_read_line(const char *prompt) {")
	e.writeLine("  size_t cap = 64U;")
	e.writeLine("  size_t len = 0U;")
	e.writeLine("  char *buffer = NULL;")
	e.writeLine("  int ch = 0;")
	e.writeLine("  if (prompt != NULL) {")
	e.writeLine("    fputs(prompt, stdout);")
	e.writeLine("    fflush(stdout);")
	e.writeLine("  }")
	e.writeLine("  buffer = (char *)malloc(cap);")
	e.writeLine("  if (buffer == NULL) gwen_runtime_error(\"runtime error: out of memory reading input\");")
	e.writeLine("  while ((ch = fgetc(stdin)) != EOF) {")
	e.writeLine("    if (ch == '\\n') break;")
	e.writeLine("    if (ch == '\\r') {")
	e.writeLine("      int next = fgetc(stdin);")
	e.writeLine("      if (next != '\\n' && next != EOF) ungetc(next, stdin);")
	e.writeLine("      break;")
	e.writeLine("    }")
	e.writeLine("    if (len + 1U >= cap) {")
	e.writeLine("      size_t next_cap = cap * 2U;")
	e.writeLine("      char *next_buffer = (char *)realloc(buffer, next_cap);")
	e.writeLine("      if (next_buffer == NULL) {")
	e.writeLine("        free(buffer);")
	e.writeLine("        gwen_runtime_error(\"runtime error: out of memory reading input\");")
	e.writeLine("      }")
	e.writeLine("      buffer = next_buffer;")
	e.writeLine("      cap = next_cap;")
	e.writeLine("    }")
	e.writeLine("    buffer[len++] = (char)ch;")
	e.writeLine("  }")
	e.writeLine("  buffer[len] = '\\0';")
	e.writeLine("  return buffer;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static long long gwen_string_len(const char *value) {")
	e.writeLine("  return (long long)strlen(value != NULL ? value : \"\");")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_string_index(const char *value, long long index) {")
	e.writeLine("  const char *safe = value != NULL ? value : \"\";")
	e.writeLine("  long long len = gwen_string_len(safe);")
	e.writeLine("  char *result = NULL;")
	e.writeLine("  if (index < 0 || index >= len) gwen_runtime_error(\"runtime error: index out of range\");")
	e.writeLine("  result = (char *)malloc(2ULL);")
	e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory allocating string index result\");")
	e.writeLine("  result[0] = safe[index];")
	e.writeLine("  result[1] = '\\0';")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_string_eq(const char *left, const char *right) {")
	e.writeLine("  const char *safe_left = left != NULL ? left : \"\";")
	e.writeLine("  const char *safe_right = right != NULL ? right : \"\";")
	e.writeLine("  return strcmp(safe_left, safe_right) == 0;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_string_ne(const char *left, const char *right) {")
	e.writeLine("  return !gwen_string_eq(left, right);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static int gwen_string_cmp(const char *left, const char *right) {")
	e.writeLine("  const char *safe_left = left != NULL ? left : \"\";")
	e.writeLine("  const char *safe_right = right != NULL ? right : \"\";")
	e.writeLine("  return strcmp(safe_left, safe_right);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static int gwen_string_ptr_cmp(const void *left, const void *right) {")
	e.writeLine("  const char *left_value = *(const char * const *)left;")
	e.writeLine("  const char *right_value = *(const char * const *)right;")
	e.writeLine("  return gwen_string_cmp(left_value, right_value);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_string_dup_len(const char *start, size_t len) {")
	e.writeLine("  char *result = (char *)malloc(len + 1U);")
	e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory allocating string\");")
	e.writeLine("  if (len > 0U) memcpy(result, start, len);")
	e.writeLine("  result[len] = '\\0';")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_string_dup(const char *value) {")
	e.writeLine("  const char *safe = value != NULL ? value : \"\";")
	e.writeLine("  return gwen_string_dup_len(safe, strlen(safe));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_int_to_string(long long value) {")
	e.writeLine("  int len = snprintf(NULL, 0, \"%lld\", value);")
	e.writeLine("  char *result = (char *)malloc((size_t)len + 1U);")
	e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory formatting int\");")
	e.writeLine("  snprintf(result, (size_t)len + 1U, \"%lld\", value);")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_float_to_string(double value) {")
	e.writeLine("  int len = snprintf(NULL, 0, \"%.15g\", value);")
	e.writeLine("  char *result = (char *)malloc((size_t)len + 1U);")
	e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory formatting float\");")
	e.writeLine("  snprintf(result, (size_t)len + 1U, \"%.15g\", value);")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_bool_to_string(bool value) {")
	e.writeLine("  return gwen_string_dup(value ? \"true\" : \"false\");")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_message_with_path(const char *action, const char *path, const char *detail) {")
	e.writeLine("  const char *safe_action = action != NULL ? action : \"open\";")
	e.writeLine("  const char *safe_path = path != NULL ? path : \"\";")
	e.writeLine("  const char *safe_detail = detail != NULL ? detail : \"unknown error\";")
	e.writeLine("  int len = snprintf(NULL, 0, \"%s %s: %s\", safe_action, safe_path, safe_detail);")
	e.writeLine("  char *result = (char *)malloc((size_t)len + 1U);")
	e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory formatting path message\");")
	e.writeLine("  snprintf(result, (size_t)len + 1U, \"%s %s: %s\", safe_action, safe_path, safe_detail);")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_string_concat(const char *left, const char *right) {")
	e.writeLine("  const char *safe_left = left != NULL ? left : \"\";")
	e.writeLine("  const char *safe_right = right != NULL ? right : \"\";")
	e.writeLine("  size_t left_len = strlen(safe_left);")
	e.writeLine("  size_t right_len = strlen(safe_right);")
	e.writeLine("  char *result = (char *)malloc(left_len + right_len + 1U);")
	e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory concatenating string\");")
	e.writeLine("  if (left_len > 0U) memcpy(result, safe_left, left_len);")
	e.writeLine("  if (right_len > 0U) memcpy(result + left_len, safe_right, right_len);")
	e.writeLine("  result[left_len + right_len] = '\\0';")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_string_substring(const char *value, long long start, long long end) {")
	e.writeLine("  const char *safe = value != NULL ? value : \"\";")
	e.writeLine("  long long len = gwen_string_len(safe);")
	e.writeLine("  if (start < 0 || end < start || end >= len) gwen_runtime_error(\"runtime error: substring() bounds out of range\");")
	e.writeLine("  return gwen_string_dup_len(safe + start, (size_t)(end - start + 1));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_string_startswith(const char *value, const char *prefix) {")
	e.writeLine("  const char *safe_value = value != NULL ? value : \"\";")
	e.writeLine("  const char *safe_prefix = prefix != NULL ? prefix : \"\";")
	e.writeLine("  size_t prefix_len = strlen(safe_prefix);")
	e.writeLine("  return strncmp(safe_value, safe_prefix, prefix_len) == 0;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_string_endswith(const char *value, const char *suffix) {")
	e.writeLine("  const char *safe_value = value != NULL ? value : \"\";")
	e.writeLine("  const char *safe_suffix = suffix != NULL ? suffix : \"\";")
	e.writeLine("  size_t value_len = strlen(safe_value);")
	e.writeLine("  size_t suffix_len = strlen(safe_suffix);")
	e.writeLine("  if (suffix_len > value_len) return false;")
	e.writeLine("  return strncmp(safe_value + (value_len - suffix_len), safe_suffix, suffix_len) == 0;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_string_contains(const char *value, const char *substr) {")
	e.writeLine("  const char *safe_value = value != NULL ? value : \"\";")
	e.writeLine("  const char *safe_substr = substr != NULL ? substr : \"\";")
	e.writeLine("  return strstr(safe_value, safe_substr) != NULL;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_string_trim(const char *value) {")
	e.writeLine("  const char *safe = value != NULL ? value : \"\";")
	e.writeLine("  size_t start = 0U;")
	e.writeLine("  size_t end = strlen(safe);")
	e.writeLine("  while (start < end && isspace((unsigned char)safe[start])) start++;")
	e.writeLine("  while (end > start && isspace((unsigned char)safe[end - 1U])) end--;")
	e.writeLine("  return gwen_string_dup_len(safe + start, end - start);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_string_replace(const char *value, const char *old_value, const char *new_value) {")
	e.writeLine("  const char *safe_value = value != NULL ? value : \"\";")
	e.writeLine("  const char *safe_old = old_value != NULL ? old_value : \"\";")
	e.writeLine("  const char *safe_new = new_value != NULL ? new_value : \"\";")
	e.writeLine("  size_t value_len = strlen(safe_value);")
	e.writeLine("  size_t old_len = strlen(safe_old);")
	e.writeLine("  size_t new_len = strlen(safe_new);")
	e.writeLine("  if (old_len == 0U) {")
	e.writeLine("    size_t total = value_len + ((value_len + 1U) * new_len);")
	e.writeLine("    char *result = (char *)malloc(total + 1U);")
	e.writeLine("    size_t pos = 0U;")
	e.writeLine("    if (result == NULL) gwen_runtime_error(\"runtime error: out of memory replacing string\");")
	e.writeLine("    for (size_t i = 0U; i < value_len; ++i) {")
	e.writeLine("      if (new_len > 0U) { memcpy(result + pos, safe_new, new_len); pos += new_len; }")
	e.writeLine("      result[pos++] = safe_value[i];")
	e.writeLine("    }")
	e.writeLine("    if (new_len > 0U) { memcpy(result + pos, safe_new, new_len); pos += new_len; }")
	e.writeLine("    result[pos] = '\\0';")
	e.writeLine("    return result;")
	e.writeLine("  }")
	e.writeLine("  size_t count = 0U;")
	e.writeLine("  const char *cursor = safe_value;")
	e.writeLine("  const char *match = NULL;")
	e.writeLine("  while ((match = strstr(cursor, safe_old)) != NULL) {")
	e.writeLine("    count++;")
	e.writeLine("    cursor = match + old_len;")
	e.writeLine("  }")
	e.writeLine("  if (count == 0U) return gwen_string_dup_len(safe_value, value_len);")
	e.writeLine("  size_t total = value_len;")
	e.writeLine("  if (new_len >= old_len) total += count * (new_len - old_len);")
	e.writeLine("  else total -= count * (old_len - new_len);")
	e.writeLine("  char *result = (char *)malloc(total + 1U);")
	e.writeLine("  size_t pos = 0U;")
	e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory replacing string\");")
	e.writeLine("  cursor = safe_value;")
	e.writeLine("  while ((match = strstr(cursor, safe_old)) != NULL) {")
	e.writeLine("    size_t chunk = (size_t)(match - cursor);")
	e.writeLine("    if (chunk > 0U) { memcpy(result + pos, cursor, chunk); pos += chunk; }")
	e.writeLine("    if (new_len > 0U) { memcpy(result + pos, safe_new, new_len); pos += new_len; }")
	e.writeLine("    cursor = match + old_len;")
	e.writeLine("  }")
	e.writeLine("  if (*cursor != '\\0') {")
	e.writeLine("    size_t tail = strlen(cursor);")
	e.writeLine("    memcpy(result + pos, cursor, tail);")
	e.writeLine("    pos += tail;")
	e.writeLine("  }")
	e.writeLine("  result[pos] = '\\0';")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_path_basename(const char *value) {")
	e.writeLine("  const char *safe = value != NULL ? value : \"\";")
	e.writeLine("  size_t len = strlen(safe);")
	e.writeLine("  size_t start = 0U;")
	e.writeLine("  if (len == 0U) return gwen_string_dup_len(\"\", 0U);")
	e.writeLine("  while (len > 1U && safe[len - 1U] == '/') len--;")
	e.writeLine("  start = len;")
	e.writeLine("  while (start > 0U && safe[start - 1U] != '/') start--;")
	e.writeLine("  return gwen_string_dup_len(safe + start, len - start);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_path_dirname(const char *value) {")
	e.writeLine("  const char *safe = value != NULL ? value : \"\";")
	e.writeLine("  size_t len = strlen(safe);")
	e.writeLine("  if (len == 0U) return gwen_string_dup_len(\"\", 0U);")
	e.writeLine("  while (len > 1U && safe[len - 1U] == '/') len--;")
	e.writeLine("  while (len > 0U && safe[len - 1U] != '/') len--;")
	e.writeLine("  if (len == 0U) return gwen_string_dup_len(\".\", 1U);")
	e.writeLine("  while (len > 1U && safe[len - 1U] == '/') len--;")
	e.writeLine("  return gwen_string_dup_len(safe, len);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_path_join(const char *left, const char *right) {")
	e.writeLine("  const char *safe_left = left != NULL ? left : \"\";")
	e.writeLine("  const char *safe_right = right != NULL ? right : \"\";")
	e.writeLine("  size_t left_len = strlen(safe_left);")
	e.writeLine("  size_t right_len = strlen(safe_right);")
	e.writeLine("  size_t extra = 0U;")
	e.writeLine("  size_t trim_right = 0U;")
	e.writeLine("  char *result = NULL;")
	e.writeLine("  size_t pos = 0U;")
	e.writeLine("  if (left_len == 0U) return gwen_string_dup_len(safe_right, right_len);")
	e.writeLine("  if (right_len == 0U) return gwen_string_dup_len(safe_left, left_len);")
	e.writeLine("  if (safe_left[left_len - 1U] == '/' && safe_right[0] == '/') trim_right = 1U;")
	e.writeLine("  else if (safe_left[left_len - 1U] != '/' && safe_right[0] != '/') extra = 1U;")
	e.writeLine("  result = (char *)malloc(left_len + right_len + extra - trim_right + 1U);")
	e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory joining path\");")
	e.writeLine("  memcpy(result + pos, safe_left, left_len);")
	e.writeLine("  pos += left_len;")
	e.writeLine("  if (extra == 1U) result[pos++] = '/';")
	e.writeLine("  memcpy(result + pos, safe_right + trim_right, right_len - trim_right);")
	e.writeLine("  pos += right_len - trim_right;")
	e.writeLine("  result[pos] = '\\0';")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("typedef enum {")
	e.writeLine("  GWEN_VALUE_NULL = 0,")
	e.writeLine("  GWEN_VALUE_INT = 1,")
	e.writeLine("  GWEN_VALUE_FLOAT = 2,")
	e.writeLine("  GWEN_VALUE_BOOL = 3,")
	e.writeLine("  GWEN_VALUE_STRING = 4,")
	e.writeLine("  GWEN_VALUE_LIST = 5,")
	e.writeLine("  GWEN_VALUE_DICT = 6,")
	e.writeLine("  GWEN_VALUE_RESULT = 7")
	e.writeLine("} gwen_value_kind;")
	e.writeLine("")
	e.writeLine("typedef struct gwen_value gwen_value;")
	e.writeLine("typedef struct gwen_dyn_result gwen_dyn_result;")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  long long len;")
	e.writeLine("  gwen_value *items;")
	e.writeLine("} gwen_dyn_list;")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  long long len;")
	e.writeLine("  gwen_value *keys;")
	e.writeLine("  gwen_value *values;")
	e.writeLine("} gwen_dyn_dict;")
	e.writeLine("")
	e.writeLine("struct gwen_value {")
	e.writeLine("  gwen_value_kind kind;")
	e.writeLine("  long long int_value;")
	e.writeLine("  double float_value;")
	e.writeLine("  bool bool_value;")
	e.writeLine("  const char *string_value;")
	e.writeLine("  gwen_dyn_list *list_value;")
	e.writeLine("  gwen_dyn_dict *dict_value;")
	e.writeLine("  gwen_dyn_result *result_value;")
	e.writeLine("};")
	e.writeLine("")
	e.writeLine("struct gwen_dyn_result {")
	e.writeLine("  bool is_ok;")
	e.writeLine("  gwen_value ok;")
	e.writeLine("  gwen_value err;")
	e.writeLine("};")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_null(void) {")
	e.writeLine("  return (gwen_value){GWEN_VALUE_NULL, 0, 0.0, false, NULL, NULL, NULL, NULL};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_int(long long value) {")
	e.writeLine("  return (gwen_value){GWEN_VALUE_INT, value, 0.0, false, NULL, NULL, NULL, NULL};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_float(double value) {")
	e.writeLine("  return (gwen_value){GWEN_VALUE_FLOAT, 0, value, false, NULL, NULL, NULL, NULL};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_bool(bool value) {")
	e.writeLine("  return (gwen_value){GWEN_VALUE_BOOL, 0, 0.0, value, NULL, NULL, NULL, NULL};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_string(const char *value) {")
	e.writeLine("  return (gwen_value){GWEN_VALUE_STRING, 0, 0.0, false, value, NULL, NULL, NULL};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_dyn_list *gwen_dyn_list_new(long long len) {")
	e.writeLine("  gwen_dyn_list *list = (gwen_dyn_list *)malloc(sizeof(gwen_dyn_list));")
	e.writeLine("  if (list == NULL) gwen_runtime_error(\"runtime error: out of memory allocating dynamic list\");")
	e.writeLine("  list->len = len;")
	e.writeLine("  list->items = NULL;")
	e.writeLine("  if (len > 0) {")
	e.writeLine("    list->items = (gwen_value *)malloc(sizeof(gwen_value) * (size_t)len);")
	e.writeLine("    if (list->items == NULL) gwen_runtime_error(\"runtime error: out of memory allocating dynamic list items\");")
	e.writeLine("  }")
	e.writeLine("  return list;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_dyn_dict *gwen_dyn_dict_new(long long len) {")
	e.writeLine("  gwen_dyn_dict *dict = (gwen_dyn_dict *)malloc(sizeof(gwen_dyn_dict));")
	e.writeLine("  if (dict == NULL) gwen_runtime_error(\"runtime error: out of memory allocating dynamic dict\");")
	e.writeLine("  dict->len = len;")
	e.writeLine("  dict->keys = NULL;")
	e.writeLine("  dict->values = NULL;")
	e.writeLine("  if (len > 0) {")
	e.writeLine("    dict->keys = (gwen_value *)malloc(sizeof(gwen_value) * (size_t)len);")
	e.writeLine("    dict->values = (gwen_value *)malloc(sizeof(gwen_value) * (size_t)len);")
	e.writeLine("    if (dict->keys == NULL || dict->values == NULL) gwen_runtime_error(\"runtime error: out of memory allocating dynamic dict items\");")
	e.writeLine("  }")
	e.writeLine("  return dict;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_list_from_ptr(gwen_dyn_list *list) {")
	e.writeLine("  return (gwen_value){GWEN_VALUE_LIST, 0, 0.0, false, NULL, list, NULL, NULL};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_dict_from_ptr(gwen_dyn_dict *dict) {")
	e.writeLine("  return (gwen_value){GWEN_VALUE_DICT, 0, 0.0, false, NULL, NULL, dict, NULL};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_dyn_result *gwen_dyn_result_new(bool is_ok, gwen_value ok, gwen_value err) {")
	e.writeLine("  gwen_dyn_result *result = (gwen_dyn_result *)malloc(sizeof(gwen_dyn_result));")
	e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory allocating dynamic result\");")
	e.writeLine("  result->is_ok = is_ok;")
	e.writeLine("  result->ok = ok;")
	e.writeLine("  result->err = err;")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_result_from_ptr(gwen_dyn_result *result) {")
	e.writeLine("  return (gwen_value){GWEN_VALUE_RESULT, 0, 0.0, false, NULL, NULL, NULL, result};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_result_ok(gwen_value value) {")
	e.writeLine("  return gwen_value_result_from_ptr(gwen_dyn_result_new(true, value, gwen_value_null()));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_result_err(gwen_value value) {")
	e.writeLine("  return gwen_value_result_from_ptr(gwen_dyn_result_new(false, gwen_value_null(), value));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_value_is_null(gwen_value value) {")
	e.writeLine("  return value.kind == GWEN_VALUE_NULL;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_clone(gwen_value value) {")
	e.writeLine("  switch (value.kind) {")
	e.writeLine("    case GWEN_VALUE_NULL:")
	e.writeLine("    case GWEN_VALUE_INT:")
	e.writeLine("    case GWEN_VALUE_FLOAT:")
	e.writeLine("    case GWEN_VALUE_BOOL:")
	e.writeLine("      return value;")
	e.writeLine("    case GWEN_VALUE_STRING:")
	e.writeLine("      return gwen_value_string(gwen_string_dup(value.string_value != NULL ? value.string_value : \"\"));")
	e.writeLine("    case GWEN_VALUE_LIST: {")
	e.writeLine("      gwen_dyn_list *src = value.list_value;")
	e.writeLine("      gwen_dyn_list *dst = gwen_dyn_list_new(src->len);")
	e.writeLine("      for (long long i = 0; i < src->len; ++i) dst->items[i] = gwen_value_clone(src->items[i]);")
	e.writeLine("      return gwen_value_list_from_ptr(dst);")
	e.writeLine("    }")
	e.writeLine("    case GWEN_VALUE_DICT: {")
	e.writeLine("      gwen_dyn_dict *src = value.dict_value;")
	e.writeLine("      gwen_dyn_dict *dst = gwen_dyn_dict_new(src->len);")
	e.writeLine("      for (long long i = 0; i < src->len; ++i) {")
	e.writeLine("        dst->keys[i] = gwen_value_clone(src->keys[i]);")
	e.writeLine("        dst->values[i] = gwen_value_clone(src->values[i]);")
	e.writeLine("      }")
	e.writeLine("      return gwen_value_dict_from_ptr(dst);")
	e.writeLine("    }")
	e.writeLine("    case GWEN_VALUE_RESULT: {")
	e.writeLine("      gwen_dyn_result *src = value.result_value;")
	e.writeLine("      if (src == NULL) return gwen_value_result_from_ptr(NULL);")
	e.writeLine("      return gwen_value_result_from_ptr(gwen_dyn_result_new(src->is_ok, gwen_value_clone(src->ok), gwen_value_clone(src->err)));")
	e.writeLine("    }")
	e.writeLine("    default:")
	e.writeLine("      return value;")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_value_as_string(gwen_value value) {")
	e.writeLine("  if (value.kind != GWEN_VALUE_STRING) gwen_runtime_error(\"runtime error: expected string value\");")
	e.writeLine("  return value.string_value != NULL ? value.string_value : \"\";")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static long long gwen_value_as_int(gwen_value value) {")
	e.writeLine("  if (value.kind != GWEN_VALUE_INT) gwen_runtime_error(\"runtime error: expected int value\");")
	e.writeLine("  return value.int_value;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static double gwen_value_as_float(gwen_value value) {")
	e.writeLine("  if (value.kind == GWEN_VALUE_FLOAT) return value.float_value;")
	e.writeLine("  if (value.kind == GWEN_VALUE_INT) return (double)value.int_value;")
	e.writeLine("  gwen_runtime_error(\"runtime error: expected float value\");")
	e.writeLine("  return 0.0;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_value_as_bool(gwen_value value) {")
	e.writeLine("  if (value.kind != GWEN_VALUE_BOOL) gwen_runtime_error(\"runtime error: expected bool value\");")
	e.writeLine("  return value.bool_value;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_dyn_list *gwen_value_as_list(gwen_value value) {")
	e.writeLine("  if (value.kind != GWEN_VALUE_LIST || value.list_value == NULL) gwen_runtime_error(\"runtime error: expected list value\");")
	e.writeLine("  return value.list_value;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_dyn_dict *gwen_value_as_dict(gwen_value value) {")
	e.writeLine("  if (value.kind != GWEN_VALUE_DICT || value.dict_value == NULL) gwen_runtime_error(\"runtime error: expected dict value\");")
	e.writeLine("  return value.dict_value;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_value_eq(gwen_value left, gwen_value right) {")
	e.writeLine("  if (left.kind != right.kind) return false;")
	e.writeLine("  switch (left.kind) {")
	e.writeLine("    case GWEN_VALUE_NULL:")
	e.writeLine("      return true;")
	e.writeLine("    case GWEN_VALUE_INT:")
	e.writeLine("      return left.int_value == right.int_value;")
	e.writeLine("    case GWEN_VALUE_FLOAT:")
	e.writeLine("      return left.float_value == right.float_value;")
	e.writeLine("    case GWEN_VALUE_BOOL:")
	e.writeLine("      return left.bool_value == right.bool_value;")
	e.writeLine("    case GWEN_VALUE_STRING:")
	e.writeLine("      return gwen_string_eq(left.string_value, right.string_value);")
	e.writeLine("    case GWEN_VALUE_LIST:")
	e.writeLine("      return left.list_value == right.list_value;")
	e.writeLine("    case GWEN_VALUE_DICT:")
	e.writeLine("      return left.dict_value == right.dict_value;")
	e.writeLine("    case GWEN_VALUE_RESULT:")
	e.writeLine("      if (left.result_value == NULL || right.result_value == NULL) return left.result_value == right.result_value;")
	e.writeLine("      if (left.result_value->is_ok != right.result_value->is_ok) return false;")
	e.writeLine("      if (left.result_value->is_ok) return gwen_value_eq(left.result_value->ok, right.result_value->ok);")
	e.writeLine("      return gwen_value_eq(left.result_value->err, right.result_value->err);")
	e.writeLine("    default:")
	e.writeLine("      return false;")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static long long gwen_value_len(gwen_value value) {")
	e.writeLine("  switch (value.kind) {")
	e.writeLine("    case GWEN_VALUE_LIST:")
	e.writeLine("      return gwen_value_as_list(value)->len;")
	e.writeLine("    case GWEN_VALUE_DICT:")
	e.writeLine("      return gwen_value_as_dict(value)->len;")
	e.writeLine("    case GWEN_VALUE_STRING:")
	e.writeLine("      return gwen_string_len(value.string_value);")
	e.writeLine("    default:")
	e.writeLine("      gwen_runtime_error(\"runtime error: len() expects string, list, or dict\");")
	e.writeLine("      return 0;")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_value_list_append(gwen_value *list_value, gwen_value item) {")
	e.writeLine("  gwen_dyn_list *list = NULL;")
	e.writeLine("  gwen_value *new_items = NULL;")
	e.writeLine("  long long new_len = 0;")
	e.writeLine("  if (list_value == NULL) gwen_runtime_error(\"runtime error: append() requires list target\");")
	e.writeLine("  list = gwen_value_as_list(*list_value);")
	e.writeLine("  new_len = list->len + 1;")
	e.writeLine("  new_items = (gwen_value *)realloc(list->items, sizeof(gwen_value) * (size_t)new_len);")
	e.writeLine("  if (new_items == NULL) gwen_runtime_error(\"runtime error: out of memory growing dynamic list\");")
	e.writeLine("  list->items = new_items;")
	e.writeLine("  list->items[list->len] = item;")
	e.writeLine("  list->len = new_len;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_value_list_removeat(gwen_value *list_value, long long index) {")
	e.writeLine("  gwen_dyn_list *list = NULL;")
	e.writeLine("  gwen_value *new_items = NULL;")
	e.writeLine("  if (list_value == NULL) gwen_runtime_error(\"runtime error: removeat() requires list target\");")
	e.writeLine("  list = gwen_value_as_list(*list_value);")
	e.writeLine("  if (index < 0LL || index >= list->len) gwen_runtime_error(\"runtime error: removeat() index out of range\");")
	e.writeLine("  for (long long i = index + 1LL; i < list->len; ++i) list->items[i - 1LL] = list->items[i];")
	e.writeLine("  list->len--;")
	e.writeLine("  if (list->len <= 0LL) {")
	e.writeLine("    free(list->items);")
	e.writeLine("    list->items = NULL;")
	e.writeLine("    list->len = 0LL;")
	e.writeLine("    return;")
	e.writeLine("  }")
	e.writeLine("  new_items = (gwen_value *)realloc(list->items, sizeof(gwen_value) * (size_t)list->len);")
	e.writeLine("  if (new_items != NULL) list->items = new_items;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_list_concat(gwen_value left_value, gwen_value right_value) {")
	e.writeLine("  gwen_dyn_list *left = gwen_value_as_list(left_value);")
	e.writeLine("  gwen_dyn_list *right = gwen_value_as_list(right_value);")
	e.writeLine("  gwen_dyn_list *result = gwen_dyn_list_new(left->len + right->len);")
	e.writeLine("  for (long long i = 0; i < left->len; ++i) result->items[i] = left->items[i];")
	e.writeLine("  for (long long i = 0; i < right->len; ++i) result->items[left->len + i] = right->items[i];")
	e.writeLine("  return gwen_value_list_from_ptr(result);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_list_index(gwen_value list_value, long long index) {")
	e.writeLine("  gwen_dyn_list *list = gwen_value_as_list(list_value);")
	e.writeLine("  if (index < 0 || index >= list->len) gwen_runtime_error(\"runtime error: index out of range\");")
	e.writeLine("  return list->items[index];")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_value_list_set(gwen_value *list_value, long long index, gwen_value item) {")
	e.writeLine("  gwen_dyn_list *list = NULL;")
	e.writeLine("  if (list_value == NULL) gwen_runtime_error(\"runtime error: list store requires list target\");")
	e.writeLine("  list = gwen_value_as_list(*list_value);")
	e.writeLine("  if (index < 0 || index >= list->len) gwen_runtime_error(\"runtime error: index out of range\");")
	e.writeLine("  list->items[index] = item;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_value_dict_haskey(gwen_value dict_value, gwen_value key) {")
	e.writeLine("  gwen_dyn_dict *dict = gwen_value_as_dict(dict_value);")
	e.writeLine("  for (long long i = dict->len - 1; i >= 0; --i) {")
	e.writeLine("    if (gwen_value_eq(dict->keys[i], key)) return true;")
	e.writeLine("  }")
	e.writeLine("  return false;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_dict_get(gwen_value dict_value, gwen_value key, gwen_value fallback) {")
	e.writeLine("  gwen_dyn_dict *dict = gwen_value_as_dict(dict_value);")
	e.writeLine("  for (long long i = dict->len - 1; i >= 0; --i) {")
	e.writeLine("    if (gwen_value_eq(dict->keys[i], key)) return dict->values[i];")
	e.writeLine("  }")
	e.writeLine("  return fallback;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_dict_index(gwen_value dict_value, gwen_value key) {")
	e.writeLine("  gwen_dyn_dict *dict = gwen_value_as_dict(dict_value);")
	e.writeLine("  for (long long i = dict->len - 1; i >= 0; --i) {")
	e.writeLine("    if (gwen_value_eq(dict->keys[i], key)) return dict->values[i];")
	e.writeLine("  }")
	e.writeLine("  gwen_runtime_error(\"runtime error: key not found\");")
	e.writeLine("  return gwen_value_null();")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_value_dict_set(gwen_value *dict_value, gwen_value key, gwen_value value) {")
	e.writeLine("  gwen_dyn_dict *dict = NULL;")
	e.writeLine("  gwen_value *new_keys = NULL;")
	e.writeLine("  gwen_value *new_values = NULL;")
	e.writeLine("  long long new_len = 0;")
	e.writeLine("  if (dict_value == NULL) gwen_runtime_error(\"runtime error: dict store requires dict target\");")
	e.writeLine("  dict = gwen_value_as_dict(*dict_value);")
	e.writeLine("  for (long long i = dict->len - 1; i >= 0; --i) {")
	e.writeLine("    if (gwen_value_eq(dict->keys[i], key)) {")
	e.writeLine("      dict->values[i] = value;")
	e.writeLine("      return;")
	e.writeLine("    }")
	e.writeLine("  }")
	e.writeLine("  new_len = dict->len + 1;")
	e.writeLine("  new_keys = (gwen_value *)realloc(dict->keys, sizeof(gwen_value) * (size_t)new_len);")
	e.writeLine("  new_values = (gwen_value *)realloc(dict->values, sizeof(gwen_value) * (size_t)new_len);")
	e.writeLine("  if (new_keys == NULL || new_values == NULL) gwen_runtime_error(\"runtime error: out of memory growing dynamic dict\");")
	e.writeLine("  dict->keys = new_keys;")
	e.writeLine("  dict->values = new_values;")
	e.writeLine("  dict->keys[dict->len] = key;")
	e.writeLine("  dict->values[dict->len] = value;")
	e.writeLine("  dict->len = new_len;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_dict_keys(gwen_value dict_value) {")
	e.writeLine("  gwen_dyn_dict *dict = gwen_value_as_dict(dict_value);")
	e.writeLine("  gwen_dyn_list *list = gwen_dyn_list_new(dict->len);")
	e.writeLine("  for (long long i = 0; i < dict->len; ++i) list->items[i] = dict->keys[i];")
	e.writeLine("  return gwen_value_list_from_ptr(list);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_dict_values(gwen_value dict_value) {")
	e.writeLine("  gwen_dyn_dict *dict = gwen_value_as_dict(dict_value);")
	e.writeLine("  gwen_dyn_list *list = gwen_dyn_list_new(dict->len);")
	e.writeLine("  for (long long i = 0; i < dict->len; ++i) list->items[i] = dict->values[i];")
	e.writeLine("  return gwen_value_list_from_ptr(list);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_value gwen_value_index(gwen_value object_value, gwen_value index) {")
	e.writeLine("  switch (object_value.kind) {")
	e.writeLine("    case GWEN_VALUE_LIST:")
	e.writeLine("      return gwen_value_list_index(object_value, gwen_value_as_int(index));")
	e.writeLine("    case GWEN_VALUE_DICT:")
	e.writeLine("      return gwen_value_dict_index(object_value, index);")
	e.writeLine("    case GWEN_VALUE_STRING:")
	e.writeLine("      return gwen_value_string(gwen_string_index(gwen_value_as_string(object_value), gwen_value_as_int(index)));")
	e.writeLine("    default:")
	e.writeLine("      gwen_runtime_error(\"runtime error: unsupported dynamic index target\");")
	e.writeLine("      return gwen_value_null();")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_value_index_set(gwen_value *object_value, gwen_value index, gwen_value value) {")
	e.writeLine("  if (object_value == NULL) gwen_runtime_error(\"runtime error: missing dynamic index target\");")
	e.writeLine("  switch (object_value->kind) {")
	e.writeLine("    case GWEN_VALUE_LIST:")
	e.writeLine("      gwen_value_list_set(object_value, gwen_value_as_int(index), value);")
	e.writeLine("      return;")
	e.writeLine("    case GWEN_VALUE_DICT:")
	e.writeLine("      gwen_value_dict_set(object_value, index, value);")
	e.writeLine("      return;")
	e.writeLine("    default:")
	e.writeLine("      gwen_runtime_error(\"runtime error: unsupported dynamic index store\");")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  long long len;")
	e.writeLine("  const char **keys;")
	e.writeLine("  const char **values;")
	e.writeLine("} gwen_string_pairs;")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  long long status;")
	e.writeLine("  const char *content_type;")
	e.writeLine("  const char *body;")
	e.writeLine("  gwen_string_pairs headers;")
	e.writeLine("} gwen_http_reply;")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  bool is_ok;")
	e.writeLine("  gwen_http_reply ok;")
	e.writeLine("  const char *err;")
	e.writeLine("} gwen_result_http_reply;")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  long long status;")
	e.writeLine("  const char *body;")
	e.writeLine("  gwen_string_pairs headers;")
	e.writeLine("} gwen_http_response;")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  const char *path;")
	e.writeLine("} gwen_sqlite_db;")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  const char *method;")
	e.writeLine("  const char *path;")
	e.writeLine("  const char *body;")
	e.writeLine("  gwen_string_pairs query;")
	e.writeLine("  gwen_string_pairs headers;")
	e.writeLine("  gwen_string_pairs cookies;")
	e.writeLine("} gwen_http_request;")
	e.writeLine("")
	e.writeLine("typedef gwen_result_http_reply (*gwen_http_handler_fn)(void *env, gwen_http_request request);")
	e.writeLine("typedef gwen_http_reply (*gwen_http_direct_handler_fn)(void *env, gwen_http_request request);")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  int fd;")
	e.writeLine("  const char *addr;")
	e.writeLine("  int handler_kind;")
	e.writeLine("  bool closed;")
	e.writeLine("  void *result_env;")
	e.writeLine("  gwen_http_handler_fn result_handler;")
	e.writeLine("  void *direct_env;")
	e.writeLine("  gwen_http_direct_handler_fn direct_handler;")
	e.writeLine("} gwen_http_server;")
	e.writeLine("")
	e.writeLine("typedef struct {")
	e.writeLine("  char *data;")
	e.writeLine("  size_t len;")
	e.writeLine("  size_t cap;")
	e.writeLine("} gwen_strbuf;")
	e.writeLine("")
	e.writeLine("static gwen_string_pairs gwen_string_pairs_empty(void) {")
	e.writeLine("  return (gwen_string_pairs){0, NULL, NULL};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_string_pairs gwen_string_pairs_clone(gwen_string_pairs pairs) {")
	e.writeLine("  gwen_string_pairs result = gwen_string_pairs_empty();")
	e.writeLine("  if (pairs.len == 0) return result;")
	e.writeLine("  result.len = pairs.len;")
	e.writeLine("  result.keys = (const char **)malloc(sizeof(const char *) * (size_t)pairs.len);")
	e.writeLine("  result.values = (const char **)malloc(sizeof(const char *) * (size_t)pairs.len);")
	e.writeLine("  if (result.keys == NULL || result.values == NULL) gwen_runtime_error(\"runtime error: out of memory cloning string pairs\");")
	e.writeLine("  for (long long i = 0; i < pairs.len; ++i) {")
	e.writeLine("    result.keys[i] = pairs.keys[i];")
	e.writeLine("    result.values[i] = pairs.values[i];")
	e.writeLine("  }")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_string_pairs_append_len(gwen_string_pairs *pairs, const char *key, size_t key_len, const char *value, size_t value_len) {")
	e.writeLine("  const char **new_keys = NULL;")
	e.writeLine("  const char **new_values = NULL;")
	e.writeLine("  long long new_len = 0;")
	e.writeLine("  if (pairs == NULL) gwen_runtime_error(\"runtime error: missing string pairs target\");")
	e.writeLine("  new_len = pairs->len + 1;")
	e.writeLine("  new_keys = (const char **)realloc((void *)pairs->keys, sizeof(const char *) * (size_t)new_len);")
	e.writeLine("  new_values = (const char **)realloc((void *)pairs->values, sizeof(const char *) * (size_t)new_len);")
	e.writeLine("  if (new_keys == NULL || new_values == NULL) gwen_runtime_error(\"runtime error: out of memory growing string pairs\");")
	e.writeLine("  pairs->keys = new_keys;")
	e.writeLine("  pairs->values = new_values;")
	e.writeLine("  pairs->keys[pairs->len] = gwen_string_dup_len(key != NULL ? key : \"\", key_len);")
	e.writeLine("  pairs->values[pairs->len] = gwen_string_dup_len(value != NULL ? value : \"\", value_len);")
	e.writeLine("  pairs->len = new_len;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_string_pairs_append(gwen_string_pairs *pairs, const char *key, const char *value) {")
	e.writeLine("  gwen_string_pairs_append_len(pairs, key != NULL ? key : \"\", strlen(key != NULL ? key : \"\"), value != NULL ? value : \"\", strlen(value != NULL ? value : \"\"));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_string_pairs_get(gwen_string_pairs pairs, const char *key, const char *fallback, bool case_insensitive) {")
	e.writeLine("  const char *safe_key = key != NULL ? key : \"\";")
	e.writeLine("  for (long long i = pairs.len - 1; i >= 0; --i) {")
	e.writeLine("    bool match = case_insensitive ? strcasecmp(pairs.keys[i], safe_key) == 0 : gwen_string_eq(pairs.keys[i], safe_key);")
	e.writeLine("    if (match) return pairs.values[i];")
	e.writeLine("  }")
	e.writeLine("  return fallback != NULL ? fallback : \"\";")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_http_reply gwen_http_reply_new(long long status, const char *content_type, const char *body) {")
	e.writeLine("  if (status < 100 || status > 999) gwen_runtime_error(\"runtime error: http reply status must be between 100 and 999\");")
	e.writeLine("  return (gwen_http_reply){status, content_type != NULL ? content_type : \"\", body != NULL ? body : \"\", gwen_string_pairs_empty()};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_http_reply gwen_http_reply_clone(gwen_http_reply reply) {")
	e.writeLine("  gwen_http_reply cloned = reply;")
	e.writeLine("  cloned.headers = gwen_string_pairs_clone(reply.headers);")
	e.writeLine("  return cloned;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_result_http_reply gwen_http_reply_ok_result(gwen_http_reply reply) {")
	e.writeLine("  return (gwen_result_http_reply){true, reply, NULL};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_result_http_reply gwen_http_reply_err_result(const char *message) {")
	e.writeLine("  return (gwen_result_http_reply){false, (gwen_http_reply){0}, message != NULL ? message : \"\"};")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_http_reply gwen_http_with_header(gwen_http_reply reply, const char *key, const char *value) {")
	e.writeLine("  gwen_http_reply cloned = gwen_http_reply_clone(reply);")
	e.writeLine("  const char *safe_key = key != NULL ? key : \"\";")
	e.writeLine("  if (strcasecmp(safe_key, \"Content-Type\") == 0) {")
	e.writeLine("    cloned.content_type = value != NULL ? value : \"\";")
	e.writeLine("    return cloned;")
	e.writeLine("  }")
	e.writeLine("  gwen_string_pairs_append(&cloned.headers, safe_key, value != NULL ? value : \"\");")
	e.writeLine("  return cloned;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_http_reply gwen_http_with_cookie(gwen_http_reply reply, const char *name, const char *value) {")
	e.writeLine("  gwen_http_reply cloned = gwen_http_reply_clone(reply);")
	e.writeLine("  const char *safe_name = name != NULL ? name : \"\";")
	e.writeLine("  const char *safe_value = value != NULL ? value : \"\";")
	e.writeLine("  const char *left = gwen_string_concat(safe_name, \"=\");")
	e.writeLine("  const char *middle = gwen_string_concat(left, safe_value);")
	e.writeLine("  const char *header = gwen_string_concat(middle, \"; Path=/\");")
	e.writeLine("  gwen_string_pairs_append(&cloned.headers, \"Set-Cookie\", header);")
	e.writeLine("  return cloned;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_strbuf_init(gwen_strbuf *buf) {")
	e.writeLine("  buf->data = NULL;")
	e.writeLine("  buf->len = 0U;")
	e.writeLine("  buf->cap = 0U;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_strbuf_reserve(gwen_strbuf *buf, size_t extra) {")
	e.writeLine("  size_t needed = 0U;")
	e.writeLine("  char *new_data = NULL;")
	e.writeLine("  if (buf == NULL) gwen_runtime_error(\"runtime error: missing string buffer\");")
	e.writeLine("  needed = buf->len + extra + 1U;")
	e.writeLine("  if (needed <= buf->cap) return;")
	e.writeLine("  if (buf->cap == 0U) buf->cap = 64U;")
	e.writeLine("  while (buf->cap < needed) buf->cap *= 2U;")
	e.writeLine("  new_data = (char *)realloc(buf->data, buf->cap);")
	e.writeLine("  if (new_data == NULL) gwen_runtime_error(\"runtime error: out of memory growing string buffer\");")
	e.writeLine("  buf->data = new_data;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_strbuf_append_len(gwen_strbuf *buf, const char *text, size_t len) {")
	e.writeLine("  gwen_strbuf_reserve(buf, len);")
	e.writeLine("  if (len > 0U) memcpy(buf->data + buf->len, text, len);")
	e.writeLine("  buf->len += len;")
	e.writeLine("  buf->data[buf->len] = '\\0';")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_strbuf_append(gwen_strbuf *buf, const char *text) {")
	e.writeLine("  const char *safe = text != NULL ? text : \"\";")
	e.writeLine("  gwen_strbuf_append_len(buf, safe, strlen(safe));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_strbuf_append_char(gwen_strbuf *buf, char ch) {")
	e.writeLine("  gwen_strbuf_reserve(buf, 1U);")
	e.writeLine("  buf->data[buf->len++] = ch;")
	e.writeLine("  buf->data[buf->len] = '\\0';")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_strbuf_take(gwen_strbuf *buf) {")
	e.writeLine("  char *data = NULL;")
	e.writeLine("  if (buf == NULL) return gwen_string_dup(\"\");")
	e.writeLine("  if (buf->data == NULL) return gwen_string_dup(\"\");")
	e.writeLine("  data = buf->data;")
	e.writeLine("  buf->data = NULL;")
	e.writeLine("  buf->len = 0U;")
	e.writeLine("  buf->cap = 0U;")
	e.writeLine("  return data;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_json_escape_append(gwen_strbuf *buf, const char *text) {")
	e.writeLine("  const unsigned char *cursor = (const unsigned char *)(text != NULL ? text : \"\");")
	e.writeLine("  gwen_strbuf_append_char(buf, '\"');")
	e.writeLine("  while (*cursor != '\\0') {")
	e.writeLine("    char escaped[7];")
	e.writeLine("    switch (*cursor) {")
	e.writeLine("      case '\\\\': gwen_strbuf_append(buf, \"\\\\\\\\\"); break;")
	e.writeLine("      case '\"': gwen_strbuf_append(buf, \"\\\\\\\"\"); break;")
	e.writeLine("      case '\\n': gwen_strbuf_append(buf, \"\\\\n\"); break;")
	e.writeLine("      case '\\r': gwen_strbuf_append(buf, \"\\\\r\"); break;")
	e.writeLine("      case '\\t': gwen_strbuf_append(buf, \"\\\\t\"); break;")
	e.writeLine("      default:")
	e.writeLine("        if (*cursor < 0x20U) {")
	e.writeLine("          snprintf(escaped, sizeof(escaped), \"\\\\u%04x\", (unsigned int)(*cursor));")
	e.writeLine("          gwen_strbuf_append(buf, escaped);")
	e.writeLine("        } else {")
	e.writeLine("          gwen_strbuf_append_char(buf, (char)(*cursor));")
	e.writeLine("        }")
	e.writeLine("        break;")
	e.writeLine("    }")
	e.writeLine("    cursor++;")
	e.writeLine("  }")
	e.writeLine("  gwen_strbuf_append_char(buf, '\"');")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_json_append_value(gwen_strbuf *buf, gwen_value value, const char **error) {")
	e.writeLine("  switch (value.kind) {")
	e.writeLine("    case GWEN_VALUE_NULL:")
	e.writeLine("      gwen_strbuf_append(buf, \"null\");")
	e.writeLine("      return true;")
	e.writeLine("    case GWEN_VALUE_INT:")
	e.writeLine("      gwen_strbuf_append(buf, gwen_int_to_string(value.int_value));")
	e.writeLine("      return true;")
	e.writeLine("    case GWEN_VALUE_FLOAT:")
	e.writeLine("      gwen_strbuf_append(buf, gwen_float_to_string(value.float_value));")
	e.writeLine("      return true;")
	e.writeLine("    case GWEN_VALUE_BOOL:")
	e.writeLine("      gwen_strbuf_append(buf, value.bool_value ? \"true\" : \"false\");")
	e.writeLine("      return true;")
	e.writeLine("    case GWEN_VALUE_STRING:")
	e.writeLine("      gwen_json_escape_append(buf, value.string_value);")
	e.writeLine("      return true;")
	e.writeLine("    case GWEN_VALUE_LIST: {")
	e.writeLine("      gwen_dyn_list *list = gwen_value_as_list(value);")
	e.writeLine("      gwen_strbuf_append_char(buf, '[');")
	e.writeLine("      for (long long i = 0; i < list->len; ++i) {")
	e.writeLine("        if (i > 0) gwen_strbuf_append_char(buf, ',');")
	e.writeLine("        if (!gwen_json_append_value(buf, list->items[i], error)) return false;")
	e.writeLine("      }")
	e.writeLine("      gwen_strbuf_append_char(buf, ']');")
	e.writeLine("      return true;")
	e.writeLine("    }")
	e.writeLine("    case GWEN_VALUE_DICT: {")
	e.writeLine("      gwen_dyn_dict *dict = gwen_value_as_dict(value);")
	e.writeLine("      gwen_strbuf_append_char(buf, '{');")
	e.writeLine("      for (long long i = 0; i < dict->len; ++i) {")
	e.writeLine("        if (dict->keys[i].kind != GWEN_VALUE_STRING) {")
	e.writeLine("          if (error != NULL) *error = \"http.json() requires string dict keys\";")
	e.writeLine("          return false;")
	e.writeLine("        }")
	e.writeLine("        if (i > 0) gwen_strbuf_append_char(buf, ',');")
	e.writeLine("        gwen_json_escape_append(buf, dict->keys[i].string_value);")
	e.writeLine("        gwen_strbuf_append_char(buf, ':');")
	e.writeLine("        if (!gwen_json_append_value(buf, dict->values[i], error)) return false;")
	e.writeLine("      }")
	e.writeLine("      gwen_strbuf_append_char(buf, '}');")
	e.writeLine("      return true;")
	e.writeLine("    }")
	e.writeLine("    default:")
	e.writeLine("      if (error != NULL) *error = \"http.json() cannot encode value\";")
	e.writeLine("      return false;")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_result_http_reply gwen_http_json_reply(long long status, gwen_value value) {")
	e.writeLine("  gwen_strbuf buf;")
	e.writeLine("  const char *error = NULL;")
	e.writeLine("  gwen_strbuf_init(&buf);")
	e.writeLine("  if (!gwen_json_append_value(&buf, value, &error)) return gwen_http_reply_err_result(error != NULL ? error : \"http.json() failed\");")
	e.writeLine("  return gwen_http_reply_ok_result(gwen_http_reply_new(status, \"application/json; charset=utf-8\", gwen_strbuf_take(&buf)));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_path_has_prefix(const char *request_path, const char *prefix) {")
	e.writeLine("  const char *safe_path = request_path != NULL ? request_path : \"\";")
	e.writeLine("  const char *safe_prefix = prefix != NULL ? prefix : \"\";")
	e.writeLine("  if (gwen_string_eq(safe_prefix, \"/\")) return safe_path[0] == '/';")
	e.writeLine("  return strncmp(safe_path, safe_prefix, strlen(safe_prefix)) == 0;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_route_pairs(const char *request_path, const char *pattern, gwen_string_pairs *params) {")
	e.writeLine("  const char *path_cursor = request_path != NULL ? request_path : \"\";")
	e.writeLine("  const char *pattern_cursor = pattern != NULL ? pattern : \"\";")
	e.writeLine("  while (*path_cursor == '/') path_cursor++;")
	e.writeLine("  while (*pattern_cursor == '/') pattern_cursor++;")
	e.writeLine("  for (;;) {")
	e.writeLine("    const char *path_end = strchr(path_cursor, '/');")
	e.writeLine("    const char *pattern_end = strchr(pattern_cursor, '/');")
	e.writeLine("    size_t path_len = path_end != NULL ? (size_t)(path_end - path_cursor) : strlen(path_cursor);")
	e.writeLine("    size_t pattern_len = pattern_end != NULL ? (size_t)(pattern_end - pattern_cursor) : strlen(pattern_cursor);")
	e.writeLine("    if (path_len == 0U && pattern_len == 0U) return true;")
	e.writeLine("    if ((path_len == 0U) != (pattern_len == 0U)) return false;")
	e.writeLine("    if (pattern_len > 1U && pattern_cursor[0] == ':') {")
	e.writeLine("      gwen_string_pairs_append_len(params, pattern_cursor + 1, pattern_len - 1U, path_cursor, path_len);")
	e.writeLine("    } else if (path_len != pattern_len || strncmp(path_cursor, pattern_cursor, path_len) != 0) {")
	e.writeLine("      return false;")
	e.writeLine("    }")
	e.writeLine("    if (path_end == NULL && pattern_end == NULL) return true;")
	e.writeLine("    if (path_end == NULL || pattern_end == NULL) return false;")
	e.writeLine("    path_cursor = path_end + 1;")
	e.writeLine("    pattern_cursor = pattern_end + 1;")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_http_content_type(const char *path) {")
	e.writeLine("  const char *safe = path != NULL ? path : \"\";")
	e.writeLine("  const char *ext = strrchr(safe, '.');")
	e.writeLine("  if (ext == NULL) return \"application/octet-stream\";")
	e.writeLine("  if (strcasecmp(ext, \".html\") == 0) return \"text/html; charset=utf-8\";")
	e.writeLine("  if (strcasecmp(ext, \".css\") == 0) return \"text/css; charset=utf-8\";")
	e.writeLine("  if (strcasecmp(ext, \".js\") == 0) return \"application/javascript; charset=utf-8\";")
	e.writeLine("  if (strcasecmp(ext, \".json\") == 0) return \"application/json; charset=utf-8\";")
	e.writeLine("  if (strcasecmp(ext, \".svg\") == 0) return \"image/svg+xml\";")
	e.writeLine("  if (strcasecmp(ext, \".ico\") == 0) return \"image/x-icon\";")
	e.writeLine("  if (strcasecmp(ext, \".md\") == 0 || strcasecmp(ext, \".gw\") == 0 || strcasecmp(ext, \".txt\") == 0) return \"text/plain; charset=utf-8\";")
	e.writeLine("  return \"application/octet-stream\";")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_read_file_bytes(const char *path, const char **out, const char **error) {")
	e.writeLine("  FILE *fp = fopen(path, \"rb\");")
	e.writeLine("  char *data = NULL;")
	e.writeLine("  long size = 0;")
	e.writeLine("  size_t read_bytes = 0U;")
	e.writeLine("  if (fp == NULL) {")
	e.writeLine("    if (error != NULL) *error = gwen_message_with_path(\"open\", path, strerror(errno));")
	e.writeLine("    return false;")
	e.writeLine("  }")
	e.writeLine("  if (fseek(fp, 0L, SEEK_END) != 0) { fclose(fp); if (error != NULL) *error = gwen_message_with_path(\"read\", path, strerror(errno)); return false; }")
	e.writeLine("  size = ftell(fp);")
	e.writeLine("  if (size < 0L) { fclose(fp); if (error != NULL) *error = gwen_message_with_path(\"read\", path, strerror(errno)); return false; }")
	e.writeLine("  if (fseek(fp, 0L, SEEK_SET) != 0) { fclose(fp); if (error != NULL) *error = gwen_message_with_path(\"read\", path, strerror(errno)); return false; }")
	e.writeLine("  data = (char *)malloc((size_t)size + 1U);")
	e.writeLine("  if (data == NULL) gwen_runtime_error(\"runtime error: out of memory reading file\");")
	e.writeLine("  if (size > 0L) read_bytes = fread(data, 1U, (size_t)size, fp);")
	e.writeLine("  fclose(fp);")
	e.writeLine("  if (read_bytes != (size_t)size) { if (error != NULL) *error = gwen_message_with_path(\"read\", path, errno != 0 ? strerror(errno) : \"failed to read file\"); return false; }")
	e.writeLine("  data[size] = '\\0';")
	e.writeLine("  if (out != NULL) *out = data;")
	e.writeLine("  return true;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_result_http_reply gwen_http_static_reply(gwen_http_request request, const char *prefix, const char *root, bool *matched) {")
	e.writeLine("  const char *safe_path = request.path != NULL ? request.path : \"\";")
	e.writeLine("  const char *safe_prefix = prefix != NULL ? prefix : \"\";")
	e.writeLine("  const char *safe_root = root != NULL ? root : \"\";")
	e.writeLine("  const char *relative = NULL;")
	e.writeLine("  const char *body = NULL;")
	e.writeLine("  const char *error = NULL;")
	e.writeLine("  const char *full_path = NULL;")
	e.writeLine("  if (matched != NULL) *matched = false;")
	e.writeLine("  if (!gwen_http_path_has_prefix(safe_path, safe_prefix)) return gwen_http_reply_err_result(\"path does not match prefix\");")
	e.writeLine("  if (matched != NULL) *matched = true;")
	e.writeLine("  relative = safe_path + strlen(safe_prefix);")
	e.writeLine("  while (*relative == '/') relative++;")
	e.writeLine("  if (*relative == '\\0') relative = \"index.html\";")
	e.writeLine("  if (strstr(relative, \"..\") != NULL) return gwen_http_reply_err_result(\"invalid static path\");")
	e.writeLine("  full_path = gwen_path_join(safe_root, relative);")
	e.writeLine("  if (!gwen_read_file_bytes(full_path, &body, &error)) return gwen_http_reply_err_result(error != NULL ? error : \"failed to read static file\");")
	e.writeLine("  return gwen_http_reply_ok_result(gwen_http_reply_new(200LL, gwen_http_content_type(full_path), body));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_http_reason_phrase(long long status) {")
	e.writeLine("  switch (status) {")
	e.writeLine("    case 200: return \"OK\";")
	e.writeLine("    case 201: return \"Created\";")
	e.writeLine("    case 204: return \"No Content\";")
	e.writeLine("    case 303: return \"See Other\";")
	e.writeLine("    case 400: return \"Bad Request\";")
	e.writeLine("    case 401: return \"Unauthorized\";")
	e.writeLine("    case 404: return \"Not Found\";")
	e.writeLine("    case 405: return \"Method Not Allowed\";")
	e.writeLine("    case 500: return \"Internal Server Error\";")
	e.writeLine("    default: return \"OK\";")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_send_all(int fd, const char *data, size_t len) {")
	e.writeLine("  size_t sent = 0U;")
	e.writeLine("  while (sent < len) {")
	e.writeLine("    ssize_t wrote = send(fd, data + sent, len - sent, 0);")
	e.writeLine("    if (wrote <= 0) return false;")
	e.writeLine("    sent += (size_t)wrote;")
	e.writeLine("  }")
	e.writeLine("  return true;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_http_parse_query_pairs(const char *query, size_t query_len, gwen_string_pairs *out) {")
	e.writeLine("  const char *cursor = query != NULL ? query : \"\";")
	e.writeLine("  const char *end = cursor + query_len;")
	e.writeLine("  while (cursor < end) {")
	e.writeLine("    const char *next = memchr(cursor, '&', (size_t)(end - cursor));")
	e.writeLine("    size_t part_len = next != NULL ? (size_t)(next - cursor) : (size_t)(end - cursor);")
	e.writeLine("    const char *eq = memchr(cursor, '=', part_len);")
	e.writeLine("    if (eq != NULL) {")
	e.writeLine("      gwen_string_pairs_append_len(out, cursor, (size_t)(eq - cursor), eq + 1, part_len - (size_t)(eq - cursor) - 1U);")
	e.writeLine("    } else if (part_len > 0U) {")
	e.writeLine("      gwen_string_pairs_append_len(out, cursor, part_len, \"\", 0U);")
	e.writeLine("    }")
	e.writeLine("    if (next == NULL) break;")
	e.writeLine("    cursor = next + 1;")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_http_parse_cookie_pairs(const char *cookie_header, size_t cookie_len, gwen_string_pairs *out) {")
	e.writeLine("  const char *cursor = cookie_header != NULL ? cookie_header : \"\";")
	e.writeLine("  const char *end = cursor + cookie_len;")
	e.writeLine("  while (cursor < end) {")
	e.writeLine("    const char *next = memchr(cursor, ';', (size_t)(end - cursor));")
	e.writeLine("    size_t part_len = next != NULL ? (size_t)(next - cursor) : (size_t)(end - cursor);")
	e.writeLine("    const char *eq = memchr(cursor, '=', part_len);")
	e.writeLine("    while (part_len > 0U && (*cursor == ' ' || *cursor == '\\t')) { cursor++; part_len--; }")
	e.writeLine("    while (part_len > 0U && (cursor[part_len - 1U] == ' ' || cursor[part_len - 1U] == '\\t')) part_len--;")
	e.writeLine("    if (eq != NULL && (size_t)(eq - cursor) < part_len) {")
	e.writeLine("      size_t key_len = (size_t)(eq - cursor);")
	e.writeLine("      size_t value_len = part_len - key_len - 1U;")
	e.writeLine("      gwen_string_pairs_append_len(out, cursor, key_len, eq + 1, value_len);")
	e.writeLine("    }")
	e.writeLine("    if (next == NULL) break;")
	e.writeLine("    cursor = next + 1;")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static ssize_t gwen_http_find_header_end(const char *data, size_t len) {")
	e.writeLine("  if (len < 4U) return -1;")
	e.writeLine("  for (size_t i = 3U; i < len; ++i) {")
	e.writeLine("    if (data[i - 3U] == '\\r' && data[i - 2U] == '\\n' && data[i - 1U] == '\\r' && data[i] == '\\n') return (ssize_t)(i - 3U);")
	e.writeLine("  }")
	e.writeLine("  return -1;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static long long gwen_http_content_length_from_headers(const char *data, size_t header_len) {")
	e.writeLine("  const char *cursor = data;")
	e.writeLine("  while (cursor < data + header_len) {")
	e.writeLine("    const char *line_end = strstr(cursor, \"\\r\\n\");")
	e.writeLine("    if (line_end == NULL || line_end > data + header_len) break;")
	e.writeLine("    const char *colon = memchr(cursor, ':', (size_t)(line_end - cursor));")
	e.writeLine("    if (colon != NULL && strncasecmp(cursor, \"Content-Length\", (size_t)(colon - cursor)) == 0) {")
	e.writeLine("      const char *value = colon + 1;")
	e.writeLine("      while (*value == ' ' || *value == '\\t') value++;")
	e.writeLine("      return atoll(value);")
	e.writeLine("    }")
	e.writeLine("    cursor = line_end + 2;")
	e.writeLine("  }")
	e.writeLine("  return 0LL;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_read_request_bytes(int fd, char **out, size_t *out_len, const char **error) {")
	e.writeLine("  size_t cap = 8192U;")
	e.writeLine("  size_t len = 0U;")
	e.writeLine("  char *data = (char *)malloc(cap + 1U);")
	e.writeLine("  ssize_t header_end = -1;")
	e.writeLine("  long long content_length = 0LL;")
	e.writeLine("  if (data == NULL) gwen_runtime_error(\"runtime error: out of memory reading http request\");")
	e.writeLine("  for (;;) {")
	e.writeLine("    ssize_t read_bytes = recv(fd, data + len, cap - len, 0);")
	e.writeLine("    if (read_bytes < 0) { if (error != NULL) *error = strerror(errno); return false; }")
	e.writeLine("    if (read_bytes == 0) break;")
	e.writeLine("    len += (size_t)read_bytes;")
	e.writeLine("    data[len] = '\\0';")
	e.writeLine("    if (header_end < 0) {")
	e.writeLine("      header_end = gwen_http_find_header_end(data, len);")
	e.writeLine("      if (header_end >= 0) content_length = gwen_http_content_length_from_headers(data, (size_t)header_end);")
	e.writeLine("    }")
	e.writeLine("    if (header_end >= 0 && len >= (size_t)header_end + 4U + (size_t)content_length) break;")
	e.writeLine("    if (len == cap) {")
	e.writeLine("      cap *= 2U;")
	e.writeLine("      data = (char *)realloc(data, cap + 1U);")
	e.writeLine("      if (data == NULL) gwen_runtime_error(\"runtime error: out of memory growing http request buffer\");")
	e.writeLine("    }")
	e.writeLine("  }")
	e.writeLine("  if (out != NULL) *out = data;")
	e.writeLine("  if (out_len != NULL) *out_len = len;")
	e.writeLine("  return true;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_parse_request(int fd, gwen_http_request *out, const char **error) {")
	e.writeLine("  char *data = NULL;")
	e.writeLine("  size_t len = 0U;")
	e.writeLine("  ssize_t header_end = -1;")
	e.writeLine("  char *line_end = NULL;")
	e.writeLine("  char *method_end = NULL;")
	e.writeLine("  char *target_end = NULL;")
	e.writeLine("  char *target = NULL;")
	e.writeLine("  char *query_mark = NULL;")
	e.writeLine("  if (out == NULL) return false;")
	e.writeLine("  *out = (gwen_http_request){0};")
	e.writeLine("  out->query = gwen_string_pairs_empty();")
	e.writeLine("  out->headers = gwen_string_pairs_empty();")
	e.writeLine("  out->cookies = gwen_string_pairs_empty();")
	e.writeLine("  if (!gwen_http_read_request_bytes(fd, &data, &len, error)) return false;")
	e.writeLine("  header_end = gwen_http_find_header_end(data, len);")
	e.writeLine("  if (header_end < 0) { if (error != NULL) *error = \"malformed http request\"; return false; }")
	e.writeLine("  line_end = strstr(data, \"\\r\\n\");")
	e.writeLine("  if (line_end == NULL) { if (error != NULL) *error = \"missing http request line\"; return false; }")
	e.writeLine("  method_end = strchr(data, ' ');")
	e.writeLine("  if (method_end == NULL || method_end >= line_end) { if (error != NULL) *error = \"malformed http method\"; return false; }")
	e.writeLine("  target = method_end + 1;")
	e.writeLine("  target_end = strchr(target, ' ');")
	e.writeLine("  if (target_end == NULL || target_end >= line_end) { if (error != NULL) *error = \"malformed http target\"; return false; }")
	e.writeLine("  out->method = gwen_string_dup_len(data, (size_t)(method_end - data));")
	e.writeLine("  query_mark = memchr(target, '?', (size_t)(target_end - target));")
	e.writeLine("  if (query_mark != NULL) {")
	e.writeLine("    out->path = gwen_string_dup_len(target, (size_t)(query_mark - target));")
	e.writeLine("    gwen_http_parse_query_pairs(query_mark + 1, (size_t)(target_end - query_mark - 1), &out->query);")
	e.writeLine("  } else {")
	e.writeLine("    out->path = gwen_string_dup_len(target, (size_t)(target_end - target));")
	e.writeLine("  }")
	e.writeLine("  {")
	e.writeLine("    char *cursor = line_end + 2;")
	e.writeLine("    while (cursor < data + header_end) {")
	e.writeLine("      char *next = strstr(cursor, \"\\r\\n\");")
	e.writeLine("      char *colon = NULL;")
	e.writeLine("      if (next == NULL || next > data + header_end) break;")
	e.writeLine("      colon = memchr(cursor, ':', (size_t)(next - cursor));")
	e.writeLine("      if (colon != NULL) {")
	e.writeLine("        const char *value = colon + 1;")
	e.writeLine("        size_t key_len = (size_t)(colon - cursor);")
	e.writeLine("        size_t value_len = (size_t)(next - value);")
	e.writeLine("        while (value_len > 0U && (*value == ' ' || *value == '\\t')) { value++; value_len--; }")
	e.writeLine("        while (value_len > 0U && (value[value_len - 1U] == ' ' || value[value_len - 1U] == '\\t')) value_len--;")
	e.writeLine("        gwen_string_pairs_append_len(&out->headers, cursor, key_len, value, value_len);")
	e.writeLine("        if (strncasecmp(cursor, \"Cookie\", key_len) == 0) gwen_http_parse_cookie_pairs(value, value_len, &out->cookies);")
	e.writeLine("      }")
	e.writeLine("      cursor = next + 2;")
	e.writeLine("    }")
	e.writeLine("  }")
	e.writeLine("  out->body = gwen_string_dup_len(data + header_end + 4, len - (size_t)header_end - 4U);")
	e.writeLine("  return true;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_write_reply(int fd, gwen_http_reply reply, const char **error) {")
	e.writeLine("  gwen_strbuf buf;")
	e.writeLine("  const char *body = reply.body != NULL ? reply.body : \"\";")
	e.writeLine("  char status_line[128];")
	e.writeLine("  snprintf(status_line, sizeof(status_line), \"HTTP/1.1 %lld %s\\r\\n\", reply.status, gwen_http_reason_phrase(reply.status));")
	e.writeLine("  gwen_strbuf_init(&buf);")
	e.writeLine("  gwen_strbuf_append(&buf, status_line);")
	e.writeLine("  if (reply.content_type != NULL && reply.content_type[0] != '\\0') {")
	e.writeLine("    gwen_strbuf_append(&buf, \"Content-Type: \");")
	e.writeLine("    gwen_strbuf_append(&buf, reply.content_type);")
	e.writeLine("    gwen_strbuf_append(&buf, \"\\r\\n\");")
	e.writeLine("  }")
	e.writeLine("  {")
	e.writeLine("    char content_length[64];")
	e.writeLine("    snprintf(content_length, sizeof(content_length), \"Content-Length: %zu\\r\\n\", strlen(body));")
	e.writeLine("    gwen_strbuf_append(&buf, content_length);")
	e.writeLine("  }")
	e.writeLine("  gwen_strbuf_append(&buf, \"Connection: close\\r\\n\");")
	e.writeLine("  for (long long i = 0; i < reply.headers.len; ++i) {")
	e.writeLine("    gwen_strbuf_append(&buf, reply.headers.keys[i]);")
	e.writeLine("    gwen_strbuf_append(&buf, \": \");")
	e.writeLine("    gwen_strbuf_append(&buf, reply.headers.values[i]);")
	e.writeLine("    gwen_strbuf_append(&buf, \"\\r\\n\");")
	e.writeLine("  }")
	e.writeLine("  gwen_strbuf_append(&buf, \"\\r\\n\");")
	e.writeLine("  if (!gwen_http_send_all(fd, buf.data != NULL ? buf.data : \"\", buf.len)) { if (error != NULL) *error = strerror(errno); return false; }")
	e.writeLine("  if (!gwen_http_send_all(fd, body, strlen(body))) { if (error != NULL) *error = strerror(errno); return false; }")
	e.writeLine("  return true;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_http_reply gwen_http_server_invoke(gwen_http_server server, gwen_http_request request) {")
	e.writeLine("  if (server.handler_kind == 1 && server.result_handler != NULL) {")
	e.writeLine("    gwen_result_http_reply result = server.result_handler(server.result_env, request);")
	e.writeLine("    if (result.is_ok) return result.ok;")
	e.writeLine("    return gwen_http_reply_new(500LL, \"text/plain; charset=utf-8\", result.err);")
	e.writeLine("  }")
	e.writeLine("  if (server.handler_kind == 2 && server.direct_handler != NULL) return server.direct_handler(server.direct_env, request);")
	e.writeLine("  return gwen_http_reply_new(500LL, \"text/plain; charset=utf-8\", \"missing handler\");")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_split_addr(const char *addr, const char **host_out, const char **port_out, const char **error) {")
	e.writeLine("  const char *safe = addr != NULL ? addr : \"\";")
	e.writeLine("  const char *colon = strrchr(safe, ':');")
	e.writeLine("  if (colon == NULL || colon == safe || colon[1] == '\\0') { if (error != NULL) *error = \"address must be host:port\"; return false; }")
	e.writeLine("  if (host_out != NULL) *host_out = gwen_string_dup_len(safe, (size_t)(colon - safe));")
	e.writeLine("  if (port_out != NULL) *port_out = gwen_string_dup(colon + 1);")
	e.writeLine("  return true;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_server_open(gwen_http_server *out, const char *addr, int handler_kind, void *result_env, gwen_http_handler_fn result_handler, void *direct_env, gwen_http_direct_handler_fn direct_handler, const char **error) {")
	e.writeLine("  const char *host = NULL;")
	e.writeLine("  const char *port = NULL;")
	e.writeLine("  struct addrinfo hints;")
	e.writeLine("  struct addrinfo *result = NULL;")
	e.writeLine("  int fd = -1;")
	e.writeLine("  if (!gwen_http_split_addr(addr, &host, &port, error)) return false;")
	e.writeLine("  memset(&hints, 0, sizeof(hints));")
	e.writeLine("  hints.ai_family = AF_INET;")
	e.writeLine("  hints.ai_socktype = SOCK_STREAM;")
	e.writeLine("  hints.ai_flags = AI_NUMERICSERV;")
	e.writeLine("  if (getaddrinfo(host, port, &hints, &result) != 0 || result == NULL) { if (error != NULL) *error = \"failed to resolve listen address\"; return false; }")
	e.writeLine("  fd = socket(result->ai_family, result->ai_socktype, result->ai_protocol);")
	e.writeLine("  if (fd < 0) { if (error != NULL) *error = strerror(errno); freeaddrinfo(result); return false; }")
	e.writeLine("  {")
	e.writeLine("    int reuse = 1;")
	e.writeLine("    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse));")
	e.writeLine("  }")
	e.writeLine("  if (bind(fd, result->ai_addr, result->ai_addrlen) != 0) { if (error != NULL) *error = strerror(errno); close(fd); freeaddrinfo(result); return false; }")
	e.writeLine("  if (listen(fd, 128) != 0) { if (error != NULL) *error = strerror(errno); close(fd); freeaddrinfo(result); return false; }")
	e.writeLine("  if (out != NULL) {")
	e.writeLine("    struct sockaddr_in sin;")
	e.writeLine("    socklen_t sin_len = sizeof(sin);")
	e.writeLine("    char host_buf[INET_ADDRSTRLEN];")
	e.writeLine("    char addr_buf[128];")
	e.writeLine("    memset(&sin, 0, sizeof(sin));")
	e.writeLine("    if (getsockname(fd, (struct sockaddr *)&sin, &sin_len) != 0) { if (error != NULL) *error = strerror(errno); close(fd); freeaddrinfo(result); return false; }")
	e.writeLine("    if (inet_ntop(AF_INET, &sin.sin_addr, host_buf, sizeof(host_buf)) == NULL) { if (error != NULL) *error = strerror(errno); close(fd); freeaddrinfo(result); return false; }")
	e.writeLine("    snprintf(addr_buf, sizeof(addr_buf), \"%s:%d\", host_buf, (int)ntohs(sin.sin_port));")
	e.writeLine("    *out = (gwen_http_server){fd, gwen_string_dup(addr_buf), handler_kind, false, result_env, result_handler, direct_env, direct_handler};")
	e.writeLine("  }")
	e.writeLine("  freeaddrinfo(result);")
	e.writeLine("  return true;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_server_wait_loop(gwen_http_server server, const char **error) {")
	e.writeLine("  for (;;) {")
	e.writeLine("    int client_fd = accept(server.fd, NULL, NULL);")
	e.writeLine("    if (client_fd < 0) {")
	e.writeLine("      if (errno == EBADF || errno == EINVAL) return true;")
	e.writeLine("      if (error != NULL) *error = strerror(errno);")
	e.writeLine("      return false;")
	e.writeLine("    }")
	e.writeLine("    {")
	e.writeLine("      gwen_http_request request;")
	e.writeLine("      const char *request_error = NULL;")
	e.writeLine("      const char *write_error = NULL;")
	e.writeLine("      gwen_http_reply reply;")
	e.writeLine("      if (!gwen_http_parse_request(client_fd, &request, &request_error)) {")
	e.writeLine("        reply = gwen_http_reply_new(400LL, \"text/plain; charset=utf-8\", request_error != NULL ? request_error : \"bad request\");")
	e.writeLine("      } else {")
	e.writeLine("        reply = gwen_http_server_invoke(server, request);")
	e.writeLine("      }")
	e.writeLine("      gwen_http_write_reply(client_fd, reply, &write_error);")
	e.writeLine("    }")
	e.writeLine("    close(client_fd);")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static bool gwen_http_server_close_fd(gwen_http_server server, const char **error) {")
	e.writeLine("  if (close(server.fd) != 0) {")
	e.writeLine("    if (error != NULL) *error = strerror(errno);")
	e.writeLine("    return false;")
	e.writeLine("  }")
	e.writeLine("  return true;")
	e.writeLine("}")
	e.writeLine("")
	e.writeBlock(`static bool gwen_http_make_temp_file(const char *pattern, char **path_out, int *fd_out, const char **error) {
  char *path = (char *)gwen_string_dup(pattern != NULL ? pattern : "/tmp/gwen-http-XXXXXX");
  int fd = mkstemp(path);
  if (fd < 0) {
    if (error != NULL) *error = strerror(errno);
    return false;
  }
  if (path_out != NULL) *path_out = path;
  if (fd_out != NULL) *fd_out = fd;
  return true;
}

static gwen_string_pairs gwen_http_response_headers_from_dump(const char *dump) {
  gwen_string_pairs current = gwen_string_pairs_empty();
  gwen_string_pairs final = gwen_string_pairs_empty();
  const char *cursor = dump != NULL ? dump : "";
  while (*cursor != '\0') {
    const char *line_end = strchr(cursor, '\n');
    size_t line_len = line_end != NULL ? (size_t)(line_end - cursor) : strlen(cursor);
    const char *colon = NULL;
    if (line_len > 0U && cursor[line_len - 1U] == '\r') line_len--;
    if (line_len == 0U) {
      if (current.len > 0) {
        final = current;
        current = gwen_string_pairs_empty();
      }
    } else if (line_len >= 5U && strncmp(cursor, "HTTP/", 5U) == 0) {
      current = gwen_string_pairs_empty();
    } else {
      colon = memchr(cursor, ':', line_len);
      if (colon != NULL) {
        const char *value = colon + 1;
        size_t key_len = (size_t)(colon - cursor);
        size_t value_len = line_len - key_len - 1U;
        while (value_len > 0U && (*value == ' ' || *value == '\t')) {
          value++;
          value_len--;
        }
        while (value_len > 0U && (value[value_len - 1U] == ' ' || value[value_len - 1U] == '\t')) value_len--;
        gwen_string_pairs_append_len(&current, cursor, key_len, value, value_len);
      }
    }
    if (line_end == NULL) break;
    cursor = line_end + 1;
  }
  if (current.len > 0) final = current;
  return final;
}

static bool gwen_http_parse_status_code(const char *text, long long *out) {
  const char *trimmed = gwen_string_trim(text);
  const char *cursor = trimmed;
  long long value = 0LL;
  if (cursor[0] == '\0') return false;
  while (*cursor != '\0') {
    if (!isdigit((unsigned char)*cursor)) return false;
    cursor++;
  }
  value = atoll(trimmed);
  if (value <= 0LL) return false;
  if (out != NULL) *out = value;
  return true;
}

static bool gwen_http_client_request(const char *method, const char *url, const char *body, gwen_string_pairs headers, long long timeout_ms, gwen_http_response *out, const char **error) {
  const char *safe_method = method != NULL ? method : "GET";
  const char *safe_url = url != NULL ? url : "";
  const char *safe_body = body != NULL ? body : "";
  char *body_path = NULL;
  char *header_path = NULL;
  char *status_path = NULL;
  char *stderr_path = NULL;
  int body_fd = -1;
  int header_fd = -1;
  int status_fd = -1;
  int stderr_fd = -1;
  pid_t pid = -1;
  int wait_status = 0;
  const char *status_text = "";
  const char *stderr_text = "";
  const char *body_text = "";
  const char *header_text = "";
  long long status_code = 0LL;
  bool ok = false;
  if (out == NULL) {
    if (error != NULL) *error = "missing http response output";
    return false;
  }
  *out = (gwen_http_response){0};
  if (!gwen_http_make_temp_file("/tmp/gwen-http-body-XXXXXX", &body_path, &body_fd, error)) goto cleanup;
  if (!gwen_http_make_temp_file("/tmp/gwen-http-header-XXXXXX", &header_path, &header_fd, error)) goto cleanup;
  if (!gwen_http_make_temp_file("/tmp/gwen-http-status-XXXXXX", &status_path, &status_fd, error)) goto cleanup;
  if (!gwen_http_make_temp_file("/tmp/gwen-http-stderr-XXXXXX", &stderr_path, &stderr_fd, error)) goto cleanup;
  close(body_fd);
  body_fd = -1;
  close(header_fd);
  header_fd = -1;
  pid = fork();
  if (pid < 0) {
    if (error != NULL) *error = strerror(errno);
    goto cleanup;
  }
  if (pid == 0) {
    size_t arg_cap = 16U + (size_t)headers.len * 2U;
    char **argv = NULL;
    size_t argc = 0U;
    char timeout_buf[32];
    if (timeout_ms > 0LL) arg_cap += 4U;
    if (safe_body[0] != '\0') arg_cap += 2U;
    argv = (char **)malloc(sizeof(char *) * (arg_cap + 1U));
    if (argv == NULL) gwen_runtime_error("runtime error: out of memory building curl argv");
    argv[argc++] = "curl";
    argv[argc++] = "-sS";
    argv[argc++] = "-L";
    argv[argc++] = "-X";
    argv[argc++] = (char *)safe_method;
    argv[argc++] = "-o";
    argv[argc++] = body_path;
    argv[argc++] = "-D";
    argv[argc++] = header_path;
    argv[argc++] = "-w";
    argv[argc++] = "%{http_code}";
    if (timeout_ms > 0LL) {
      snprintf(timeout_buf, sizeof(timeout_buf), "%.3f", ((double)timeout_ms) / 1000.0);
      argv[argc++] = "--max-time";
      argv[argc++] = timeout_buf;
      argv[argc++] = "--connect-timeout";
      argv[argc++] = timeout_buf;
    }
    for (long long i = 0; i < headers.len; ++i) {
      const char *prefix = gwen_string_concat(headers.keys[i], ": ");
      argv[argc++] = "-H";
      argv[argc++] = (char *)gwen_string_concat(prefix, headers.values[i]);
    }
    if (safe_body[0] != '\0') {
      argv[argc++] = "--data-binary";
      argv[argc++] = (char *)safe_body;
    }
    argv[argc++] = (char *)safe_url;
    argv[argc] = NULL;
    if (dup2(status_fd, STDOUT_FILENO) < 0 || dup2(stderr_fd, STDERR_FILENO) < 0) {
      fprintf(stderr, "failed to redirect curl io: %s", strerror(errno));
      _exit(127);
    }
    close(status_fd);
    close(stderr_fd);
    execvp("curl", argv);
    fprintf(stderr, "failed to exec curl: %s", strerror(errno));
    _exit(127);
  }
  close(status_fd);
  status_fd = -1;
  close(stderr_fd);
  stderr_fd = -1;
  if (waitpid(pid, &wait_status, 0) < 0) {
    if (error != NULL) *error = strerror(errno);
    goto cleanup;
  }
  if (!gwen_read_file_bytes(stderr_path, &stderr_text, NULL)) stderr_text = "";
  stderr_text = gwen_string_trim(stderr_text);
  if (!WIFEXITED(wait_status) || WEXITSTATUS(wait_status) != 0) {
    if (error != NULL) {
      *error = stderr_text[0] != '\0' ? stderr_text : "http request failed";
    }
    goto cleanup;
  }
  if (!gwen_read_file_bytes(status_path, &status_text, error)) goto cleanup;
  if (!gwen_http_parse_status_code(status_text, &status_code)) {
    if (error != NULL) *error = "failed to read http status";
    goto cleanup;
  }
  if (!gwen_read_file_bytes(body_path, &body_text, error)) goto cleanup;
  if (!gwen_read_file_bytes(header_path, &header_text, error)) goto cleanup;
  *out = (gwen_http_response){
    status_code,
    body_text,
    gwen_http_response_headers_from_dump(header_text),
  };
  ok = true;
cleanup:
  if (body_fd >= 0) close(body_fd);
  if (header_fd >= 0) close(header_fd);
  if (status_fd >= 0) close(status_fd);
  if (stderr_fd >= 0) close(stderr_fd);
  if (body_path != NULL) unlink(body_path);
  if (header_path != NULL) unlink(header_path);
  if (status_path != NULL) unlink(status_path);
  if (stderr_path != NULL) unlink(stderr_path);
  return ok;
}

typedef struct {
  const char *text;
  size_t len;
  size_t pos;
} gwen_json_parser;

static bool gwen_json_parse_value(gwen_json_parser *parser, gwen_value *out, const char **error);
static bool gwen_json_parse_array(gwen_json_parser *parser, gwen_value *out, const char **error);
static bool gwen_json_parse_object(gwen_json_parser *parser, gwen_value *out, const char **error);

static void gwen_strbuf_append_utf8(gwen_strbuf *buf, unsigned int codepoint) {
  if (codepoint <= 0x7FU) {
    gwen_strbuf_append_char(buf, (char)codepoint);
    return;
  }
  if (codepoint <= 0x7FFU) {
    gwen_strbuf_append_char(buf, (char)(0xC0U | (codepoint >> 6)));
    gwen_strbuf_append_char(buf, (char)(0x80U | (codepoint & 0x3FU)));
    return;
  }
  if (codepoint <= 0xFFFFU) {
    gwen_strbuf_append_char(buf, (char)(0xE0U | (codepoint >> 12)));
    gwen_strbuf_append_char(buf, (char)(0x80U | ((codepoint >> 6) & 0x3FU)));
    gwen_strbuf_append_char(buf, (char)(0x80U | (codepoint & 0x3FU)));
    return;
  }
  gwen_strbuf_append_char(buf, (char)(0xF0U | (codepoint >> 18)));
  gwen_strbuf_append_char(buf, (char)(0x80U | ((codepoint >> 12) & 0x3FU)));
  gwen_strbuf_append_char(buf, (char)(0x80U | ((codepoint >> 6) & 0x3FU)));
  gwen_strbuf_append_char(buf, (char)(0x80U | (codepoint & 0x3FU)));
}

static void gwen_json_skip_ws(gwen_json_parser *parser) {
  while (parser != NULL && parser->pos < parser->len && isspace((unsigned char)parser->text[parser->pos])) parser->pos++;
}

static bool gwen_json_parse_hex4(gwen_json_parser *parser, unsigned int *out, const char **error) {
  unsigned int value = 0U;
  for (int i = 0; i < 4; ++i) {
    unsigned char ch = 0U;
    if (parser == NULL || parser->pos >= parser->len) {
      if (error != NULL) *error = "unterminated unicode escape";
      return false;
    }
    ch = (unsigned char)parser->text[parser->pos++];
    value <<= 4U;
    if (ch >= '0' && ch <= '9') value |= (unsigned int)(ch - '0');
    else if (ch >= 'a' && ch <= 'f') value |= (unsigned int)(10 + ch - 'a');
    else if (ch >= 'A' && ch <= 'F') value |= (unsigned int)(10 + ch - 'A');
    else {
      if (error != NULL) *error = "invalid unicode escape";
      return false;
    }
  }
  if (out != NULL) *out = value;
  return true;
}

static bool gwen_json_parse_string_value(gwen_json_parser *parser, const char **out, const char **error) {
  gwen_strbuf buf;
  gwen_strbuf_init(&buf);
  if (parser == NULL || parser->pos >= parser->len || parser->text[parser->pos] != '"') {
    if (error != NULL) *error = "expected json string";
    return false;
  }
  parser->pos++;
  while (parser->pos < parser->len) {
    unsigned char ch = (unsigned char)parser->text[parser->pos++];
    if (ch == '"') {
      if (out != NULL) *out = gwen_strbuf_take(&buf);
      return true;
    }
    if (ch == '\\') {
      unsigned int codepoint = 0U;
      if (parser->pos >= parser->len) {
        if (error != NULL) *error = "unterminated string escape";
        return false;
      }
      ch = (unsigned char)parser->text[parser->pos++];
      switch (ch) {
        case '"': gwen_strbuf_append_char(&buf, '"'); break;
        case '\\': gwen_strbuf_append_char(&buf, '\\'); break;
        case '/': gwen_strbuf_append_char(&buf, '/'); break;
        case 'b': gwen_strbuf_append_char(&buf, '\b'); break;
        case 'f': gwen_strbuf_append_char(&buf, '\f'); break;
        case 'n': gwen_strbuf_append_char(&buf, '\n'); break;
        case 'r': gwen_strbuf_append_char(&buf, '\r'); break;
        case 't': gwen_strbuf_append_char(&buf, '\t'); break;
        case 'u':
          if (!gwen_json_parse_hex4(parser, &codepoint, error)) return false;
          if (codepoint >= 0xD800U && codepoint <= 0xDBFFU) {
            unsigned int low = 0U;
            if (parser->pos + 1U >= parser->len || parser->text[parser->pos] != '\\' || parser->text[parser->pos + 1U] != 'u') {
              if (error != NULL) *error = "missing low surrogate";
              return false;
            }
            parser->pos += 2U;
            if (!gwen_json_parse_hex4(parser, &low, error)) return false;
            if (low < 0xDC00U || low > 0xDFFFU) {
              if (error != NULL) *error = "invalid low surrogate";
              return false;
            }
            codepoint = 0x10000U + (((codepoint - 0xD800U) << 10U) | (low - 0xDC00U));
          }
          gwen_strbuf_append_utf8(&buf, codepoint);
          break;
        default:
          if (error != NULL) *error = "invalid string escape";
          return false;
      }
      continue;
    }
    if (ch < 0x20U) {
      if (error != NULL) *error = "invalid control character in string";
      return false;
    }
    gwen_strbuf_append_char(&buf, (char)ch);
  }
  if (error != NULL) *error = "unterminated json string";
  return false;
}

static bool gwen_json_parse_number(gwen_json_parser *parser, gwen_value *out, const char **error) {
  size_t start = 0U;
  size_t end = 0U;
  const char *text = NULL;
  char *number_text = NULL;
  bool is_float = false;
  if (parser == NULL || parser->pos >= parser->len) {
    if (error != NULL) *error = "expected json number";
    return false;
  }
  start = parser->pos;
  if (parser->text[parser->pos] == '-') parser->pos++;
  if (parser->pos >= parser->len || !isdigit((unsigned char)parser->text[parser->pos])) {
    if (error != NULL) *error = "invalid json number";
    return false;
  }
  if (parser->text[parser->pos] == '0') parser->pos++;
  else while (parser->pos < parser->len && isdigit((unsigned char)parser->text[parser->pos])) parser->pos++;
  if (parser->pos < parser->len && parser->text[parser->pos] == '.') {
    is_float = true;
    parser->pos++;
    if (parser->pos >= parser->len || !isdigit((unsigned char)parser->text[parser->pos])) {
      if (error != NULL) *error = "invalid json fraction";
      return false;
    }
    while (parser->pos < parser->len && isdigit((unsigned char)parser->text[parser->pos])) parser->pos++;
  }
  if (parser->pos < parser->len && (parser->text[parser->pos] == 'e' || parser->text[parser->pos] == 'E')) {
    is_float = true;
    parser->pos++;
    if (parser->pos < parser->len && (parser->text[parser->pos] == '+' || parser->text[parser->pos] == '-')) parser->pos++;
    if (parser->pos >= parser->len || !isdigit((unsigned char)parser->text[parser->pos])) {
      if (error != NULL) *error = "invalid json exponent";
      return false;
    }
    while (parser->pos < parser->len && isdigit((unsigned char)parser->text[parser->pos])) parser->pos++;
  }
  end = parser->pos;
  text = parser->text + start;
  number_text = (char *)gwen_string_dup_len(text, end - start);
  if (is_float) {
    if (out != NULL) *out = gwen_value_float(strtod(number_text, NULL));
  } else {
    if (out != NULL) *out = gwen_value_int(strtoll(number_text, NULL, 10));
  }
  return true;
}

static bool gwen_json_parse_array(gwen_json_parser *parser, gwen_value *out, const char **error) {
  gwen_value result = gwen_value_list_from_ptr(gwen_dyn_list_new(0));
  if (parser == NULL || parser->pos >= parser->len || parser->text[parser->pos] != '[') {
    if (error != NULL) *error = "expected json array";
    return false;
  }
  parser->pos++;
  gwen_json_skip_ws(parser);
  if (parser->pos < parser->len && parser->text[parser->pos] == ']') {
    parser->pos++;
    if (out != NULL) *out = result;
    return true;
  }
  for (;;) {
    gwen_value item = gwen_value_null();
    if (!gwen_json_parse_value(parser, &item, error)) return false;
    gwen_value_list_append(&result, item);
    gwen_json_skip_ws(parser);
    if (parser->pos >= parser->len) {
      if (error != NULL) *error = "unterminated json array";
      return false;
    }
    if (parser->text[parser->pos] == ',') {
      parser->pos++;
      gwen_json_skip_ws(parser);
      continue;
    }
    if (parser->text[parser->pos] == ']') {
      parser->pos++;
      if (out != NULL) *out = result;
      return true;
    }
    if (error != NULL) *error = "expected ',' or ']'";
    return false;
  }
}

static bool gwen_json_parse_object(gwen_json_parser *parser, gwen_value *out, const char **error) {
  gwen_value result = gwen_value_dict_from_ptr(gwen_dyn_dict_new(0));
  if (parser == NULL || parser->pos >= parser->len || parser->text[parser->pos] != '{') {
    if (error != NULL) *error = "expected json object";
    return false;
  }
  parser->pos++;
  gwen_json_skip_ws(parser);
  if (parser->pos < parser->len && parser->text[parser->pos] == '}') {
    parser->pos++;
    if (out != NULL) *out = result;
    return true;
  }
  for (;;) {
    const char *key = NULL;
    gwen_value item = gwen_value_null();
    if (!gwen_json_parse_string_value(parser, &key, error)) return false;
    gwen_json_skip_ws(parser);
    if (parser->pos >= parser->len || parser->text[parser->pos] != ':') {
      if (error != NULL) *error = "expected ':' after object key";
      return false;
    }
    parser->pos++;
    gwen_json_skip_ws(parser);
    if (!gwen_json_parse_value(parser, &item, error)) return false;
    gwen_value_dict_set(&result, gwen_value_string(key), item);
    gwen_json_skip_ws(parser);
    if (parser->pos >= parser->len) {
      if (error != NULL) *error = "unterminated json object";
      return false;
    }
    if (parser->text[parser->pos] == ',') {
      parser->pos++;
      gwen_json_skip_ws(parser);
      continue;
    }
    if (parser->text[parser->pos] == '}') {
      parser->pos++;
      if (out != NULL) *out = result;
      return true;
    }
    if (error != NULL) *error = "expected ',' or '}'";
    return false;
  }
}

static bool gwen_json_parse_value(gwen_json_parser *parser, gwen_value *out, const char **error) {
  if (parser == NULL) {
    if (error != NULL) *error = "missing json parser";
    return false;
  }
  gwen_json_skip_ws(parser);
  if (parser->pos >= parser->len) {
    if (error != NULL) *error = "unexpected end of json";
    return false;
  }
  switch (parser->text[parser->pos]) {
    case '{':
      return gwen_json_parse_object(parser, out, error);
    case '[':
      return gwen_json_parse_array(parser, out, error);
    case '"': {
      const char *text = NULL;
      if (!gwen_json_parse_string_value(parser, &text, error)) return false;
      if (out != NULL) *out = gwen_value_string(text);
      return true;
    }
    case 't':
      if (parser->pos + 4U <= parser->len && strncmp(parser->text + parser->pos, "true", 4U) == 0) {
        parser->pos += 4U;
        if (out != NULL) *out = gwen_value_bool(true);
        return true;
      }
      break;
    case 'f':
      if (parser->pos + 5U <= parser->len && strncmp(parser->text + parser->pos, "false", 5U) == 0) {
        parser->pos += 5U;
        if (out != NULL) *out = gwen_value_bool(false);
        return true;
      }
      break;
    case 'n':
      if (parser->pos + 4U <= parser->len && strncmp(parser->text + parser->pos, "null", 4U) == 0) {
        parser->pos += 4U;
        if (out != NULL) *out = gwen_value_null();
        return true;
      }
      break;
    default:
      if (parser->text[parser->pos] == '-' || isdigit((unsigned char)parser->text[parser->pos])) {
        return gwen_json_parse_number(parser, out, error);
      }
      break;
  }
  if (error != NULL) *error = "invalid json value";
  return false;
}

static bool gwen_json_parse_root(const char *text, gwen_value *out, const char **error) {
  gwen_json_parser parser;
  if (out == NULL) {
    if (error != NULL) *error = "missing json output";
    return false;
  }
  parser.text = text != NULL ? text : "";
  parser.len = strlen(parser.text);
  parser.pos = 0U;
  if (!gwen_json_parse_value(&parser, out, error)) return false;
  gwen_json_skip_ws(&parser);
  if (parser.pos != parser.len) {
    if (error != NULL) *error = "unexpected trailing json";
    return false;
  }
  return true;
}

static bool gwen_json_stringify_value(gwen_value value, const char **out, const char **error) {
  gwen_strbuf buf;
  gwen_strbuf_init(&buf);
  if (!gwen_json_append_value(&buf, value, error)) return false;
  if (out != NULL) *out = gwen_strbuf_take(&buf);
  return true;
}

static const char *gwen_bool_display_string(bool value) {
  return gwen_bool_to_string(value);
}

static const char *gwen_float_display_string(double value) {
  const char *text = gwen_float_to_string(value);
  if (strchr(text, '.') != NULL || strchr(text, 'e') != NULL || strchr(text, 'E') != NULL) return text;
  return gwen_string_concat(text, ".0");
}

static void gwen_display_append_value(gwen_strbuf *buf, gwen_value value) {
  switch (value.kind) {
    case GWEN_VALUE_NULL:
      gwen_strbuf_append(buf, "null");
      return;
    case GWEN_VALUE_INT:
      gwen_strbuf_append(buf, gwen_int_to_string(value.int_value));
      return;
    case GWEN_VALUE_FLOAT:
      gwen_strbuf_append(buf, gwen_float_to_string(value.float_value));
      return;
    case GWEN_VALUE_BOOL:
      gwen_strbuf_append(buf, value.bool_value ? "true" : "false");
      return;
    case GWEN_VALUE_STRING:
      gwen_json_escape_append(buf, value.string_value);
      return;
    case GWEN_VALUE_LIST: {
      gwen_dyn_list *list = gwen_value_as_list(value);
      gwen_strbuf_append_char(buf, '[');
      for (long long i = 0; i < list->len; ++i) {
        if (i > 0) gwen_strbuf_append_char(buf, ',');
        gwen_display_append_value(buf, list->items[i]);
      }
      gwen_strbuf_append_char(buf, ']');
      return;
    }
    case GWEN_VALUE_DICT: {
      gwen_dyn_dict *dict = gwen_value_as_dict(value);
      gwen_strbuf_append_char(buf, '{');
      for (long long i = 0; i < dict->len; ++i) {
        if (i > 0) gwen_strbuf_append_char(buf, ',');
        if (dict->keys[i].kind == GWEN_VALUE_STRING) gwen_json_escape_append(buf, dict->keys[i].string_value);
        else gwen_display_append_value(buf, dict->keys[i]);
        gwen_strbuf_append_char(buf, ':');
        gwen_display_append_value(buf, dict->values[i]);
      }
      gwen_strbuf_append_char(buf, '}');
      return;
    }
    case GWEN_VALUE_RESULT: {
      gwen_dyn_result *result = value.result_value;
      if (result == NULL) {
        gwen_strbuf_append(buf, "err(\"invalid result\")");
        return;
      }
      gwen_strbuf_append(buf, result->is_ok ? "ok(" : "err(");
      gwen_display_append_value(buf, result->is_ok ? result->ok : result->err);
      gwen_strbuf_append_char(buf, ')');
      return;
    }
    default:
      gwen_strbuf_append(buf, "<value>");
      return;
  }
}

static const char *gwen_value_display_string(gwen_value value) {
  const char *text = NULL;
  const char *error = NULL;
  gwen_strbuf buf;
  switch (value.kind) {
    case GWEN_VALUE_NULL:
      return gwen_string_dup("null");
    case GWEN_VALUE_INT:
      return gwen_int_to_string(value.int_value);
    case GWEN_VALUE_FLOAT:
      return gwen_float_display_string(value.float_value);
    case GWEN_VALUE_BOOL:
      return gwen_bool_display_string(value.bool_value);
    case GWEN_VALUE_STRING:
      return gwen_string_dup(value.string_value != NULL ? value.string_value : "");
    case GWEN_VALUE_LIST:
    case GWEN_VALUE_DICT:
      if (gwen_json_stringify_value(value, &text, &error)) return text;
      gwen_strbuf_init(&buf);
      gwen_display_append_value(&buf, value);
      return gwen_strbuf_take(&buf);
    case GWEN_VALUE_RESULT:
      gwen_strbuf_init(&buf);
      gwen_display_append_value(&buf, value);
      return gwen_strbuf_take(&buf);
    default:
      return gwen_string_dup("<value>");
  }
}

static const char *gwen_value_list_join(gwen_value value, const char *sep) {
  gwen_dyn_list *items = gwen_value_as_list(value);
  const char *safe_sep = sep != NULL ? sep : "";
  size_t sep_len = strlen(safe_sep);
  if (items->len == 0) return gwen_string_dup("");
  const char **parts = (const char **)malloc(sizeof(const char *) * (size_t)items->len);
  if (parts == NULL) gwen_runtime_error("runtime error: out of memory joining list");
  size_t total = 0U;
  for (long long i = 0; i < items->len; ++i) {
    parts[i] = gwen_value_display_string(items->items[i]);
    total += strlen(parts[i]);
  }
  if (items->len > 1 && sep_len > 0U) total += sep_len * (size_t)(items->len - 1);
  char *result = (char *)malloc(total + 1U);
  if (result == NULL) gwen_runtime_error("runtime error: out of memory joining list");
  size_t pos = 0U;
  for (long long i = 0; i < items->len; ++i) {
    size_t part_len = strlen(parts[i]);
    if (part_len > 0U) {
      memcpy(result + pos, parts[i], part_len);
      pos += part_len;
    }
    if (i + 1 < items->len && sep_len > 0U) {
      memcpy(result + pos, safe_sep, sep_len);
      pos += sep_len;
    }
    free((void *)parts[i]);
  }
  free(parts);
  result[pos] = '\0';
  return result;
}

static gwen_value gwen_value_abs(gwen_value value) {
  switch (value.kind) {
    case GWEN_VALUE_INT:
      return gwen_value_int(value.int_value < 0LL ? -value.int_value : value.int_value);
    case GWEN_VALUE_FLOAT:
      return gwen_value_float(fabs(value.float_value));
    default:
      gwen_runtime_error("runtime error: abs() requires numeric value");
      return gwen_value_null();
  }
}

static long long gwen_value_cast_int(gwen_value value) {
  switch (value.kind) {
    case GWEN_VALUE_INT:
      return value.int_value;
    case GWEN_VALUE_FLOAT:
      return (long long)value.float_value;
    case GWEN_VALUE_STRING: {
      char *end = NULL;
      long long result = strtoll(value.string_value != NULL ? value.string_value : "", &end, 10);
      if (end == NULL || *end != '\0') gwen_runtime_error("runtime error: Cannot convert string to int");
      return result;
    }
    default:
      gwen_runtime_error("runtime error: Cannot convert value to int");
      return 0LL;
  }
}

static double gwen_value_cast_float(gwen_value value) {
  switch (value.kind) {
    case GWEN_VALUE_INT:
      return (double)value.int_value;
    case GWEN_VALUE_FLOAT:
      return value.float_value;
    case GWEN_VALUE_STRING: {
      char *end = NULL;
      double result = strtod(value.string_value != NULL ? value.string_value : "", &end);
      if (end == NULL || *end != '\0') gwen_runtime_error("runtime error: Cannot convert string to float");
      return result;
    }
    default:
      gwen_runtime_error("runtime error: Cannot convert value to float");
      return 0.0;
  }
}

static bool gwen_exec_capture(char **argv, const char **stdout_text, const char **stderr_text, int *exit_code, const char **error) {
  char *stdout_path = NULL;
  char *stderr_path = NULL;
  int stdout_fd = -1;
  int stderr_fd = -1;
  pid_t pid = -1;
  int wait_status = 0;
  bool ok = false;
  if (argv == NULL || argv[0] == NULL) {
    if (error != NULL) *error = "missing command";
    return false;
  }
  if (!gwen_http_make_temp_file("/tmp/gwen-cmd-out-XXXXXX", &stdout_path, &stdout_fd, error)) goto cleanup;
  if (!gwen_http_make_temp_file("/tmp/gwen-cmd-err-XXXXXX", &stderr_path, &stderr_fd, error)) goto cleanup;
  pid = fork();
  if (pid < 0) {
    if (error != NULL) *error = strerror(errno);
    goto cleanup;
  }
  if (pid == 0) {
    if (dup2(stdout_fd, STDOUT_FILENO) < 0 || dup2(stderr_fd, STDERR_FILENO) < 0) {
      fprintf(stderr, "failed to redirect command io: %s", strerror(errno));
      _exit(127);
    }
    close(stdout_fd);
    close(stderr_fd);
    execvp(argv[0], argv);
    fprintf(stderr, "failed to exec %s: %s", argv[0], strerror(errno));
    _exit(127);
  }
  close(stdout_fd);
  stdout_fd = -1;
  close(stderr_fd);
  stderr_fd = -1;
  if (waitpid(pid, &wait_status, 0) < 0) {
    if (error != NULL) *error = strerror(errno);
    goto cleanup;
  }
  if (stdout_text != NULL) {
    if (!gwen_read_file_bytes(stdout_path, stdout_text, NULL)) *stdout_text = gwen_string_dup("");
  }
  if (stderr_text != NULL) {
    if (!gwen_read_file_bytes(stderr_path, stderr_text, NULL)) *stderr_text = gwen_string_dup("");
  }
  if (exit_code != NULL) {
    if (WIFEXITED(wait_status)) *exit_code = WEXITSTATUS(wait_status);
    else *exit_code = -1;
  }
  ok = true;
cleanup:
  if (stdout_fd >= 0) close(stdout_fd);
  if (stderr_fd >= 0) close(stderr_fd);
  if (stdout_path != NULL) unlink(stdout_path);
  if (stderr_path != NULL) unlink(stderr_path);
  return ok;
}

static bool gwen_parse_long_long_text(const char *text, long long *out) {
  const char *trimmed = gwen_string_trim(text);
  const char *cursor = trimmed;
  if (cursor[0] == '\0') return false;
  if (*cursor == '-') cursor++;
  if (*cursor == '\0') return false;
  while (*cursor != '\0') {
    if (!isdigit((unsigned char)*cursor)) return false;
    cursor++;
  }
  if (out != NULL) *out = atoll(trimmed);
  return true;
}

static const char *gwen_sqlite_string_literal(const char *text) {
  gwen_strbuf buf;
  const char *cursor = text != NULL ? text : "";
  gwen_strbuf_init(&buf);
  gwen_strbuf_append_char(&buf, '\'');
  while (*cursor != '\0') {
    if (*cursor == '\'') gwen_strbuf_append_char(&buf, '\'');
    gwen_strbuf_append_char(&buf, *cursor);
    cursor++;
  }
  gwen_strbuf_append_char(&buf, '\'');
  return gwen_strbuf_take(&buf);
}

static bool gwen_sqlite_literal_from_value(gwen_value value, const char **literal, const char **error) {
  switch (value.kind) {
    case GWEN_VALUE_NULL:
      if (literal != NULL) *literal = "null";
      return true;
    case GWEN_VALUE_INT:
      if (literal != NULL) *literal = gwen_int_to_string(value.int_value);
      return true;
    case GWEN_VALUE_FLOAT:
      if (literal != NULL) *literal = gwen_float_to_string(value.float_value);
      return true;
    case GWEN_VALUE_BOOL:
      if (literal != NULL) *literal = value.bool_value ? "1" : "0";
      return true;
    case GWEN_VALUE_STRING:
      if (literal != NULL) *literal = gwen_sqlite_string_literal(value.string_value);
      return true;
    default:
      if (error != NULL) *error = "sqlite params only support int/float/string/bool/json.null()";
      return false;
  }
}

static const char *gwen_sqlite_param_command(long long index, const char *literal) {
  const char *prefix = gwen_string_concat(".parameter set ?", gwen_int_to_string(index));
  const char *with_space = gwen_string_concat(prefix, " ");
  return gwen_string_concat(with_space, literal != NULL ? literal : "null");
}

static bool gwen_sqlite_build_argv(const char *path, const char *sql, gwen_value params, bool json_mode, bool include_changes, char ***argv_out, const char **error) {
  gwen_dyn_list *param_list = NULL;
  char **argv = NULL;
  size_t argc = 0U;
  size_t index = 0U;
  const char *safe_path = path != NULL ? path : "";
  const char *safe_sql = sql != NULL ? sql : "";
  const char *command_sql = safe_sql;
  if (safe_path[0] == '\0') {
    if (error != NULL) *error = "sqlite path cannot be empty";
    return false;
  }
  param_list = gwen_value_as_list(params);
  argc = 5U + (size_t)param_list->len;
  if (json_mode) argc += 1U;
  argv = (char **)malloc(sizeof(char *) * (argc + 1U));
  if (argv == NULL) gwen_runtime_error("runtime error: out of memory building sqlite argv");
  argv[index++] = "sqlite3";
  argv[index++] = "-batch";
  argv[index++] = (char *)safe_path;
  argv[index++] = ".parameter init";
  for (long long i = 0; i < param_list->len; ++i) {
    const char *literal = NULL;
    if (!gwen_sqlite_literal_from_value(param_list->items[i], &literal, error)) return false;
    argv[index++] = (char *)gwen_sqlite_param_command(i + 1LL, literal);
  }
  if (json_mode) argv[index++] = ".mode json";
  if (include_changes) command_sql = gwen_string_concat(safe_sql, "; select changes();");
  argv[index++] = (char *)command_sql;
  argv[index] = NULL;
  if (argv_out != NULL) *argv_out = argv;
  return true;
}

static bool gwen_sqlite_open_path(const char *path, const char **error) {
  char *argv[] = {"sqlite3", "-batch", (char *)(path != NULL ? path : ""), "pragma schema_version;", NULL};
  const char *stdout_text = "";
  const char *stderr_text = "";
  int exit_code = 0;
  if (path == NULL || path[0] == '\0') {
    if (error != NULL) *error = "sqlite path cannot be empty";
    return false;
  }
  if (!gwen_exec_capture(argv, &stdout_text, &stderr_text, &exit_code, error)) return false;
  stderr_text = gwen_string_trim(stderr_text);
  if (exit_code != 0) {
    if (error != NULL) *error = stderr_text[0] != '\0' ? stderr_text : "sqlite open failed";
    return false;
  }
  return true;
}

static bool gwen_sqlite_exec_path(const char *path, const char *sql, gwen_value params, long long *affected, const char **error) {
  char **argv = NULL;
  const char *stdout_text = "";
  const char *stderr_text = "";
  long long changes = 0LL;
  int exit_code = 0;
  if (!gwen_sqlite_build_argv(path, sql, params, false, true, &argv, error)) return false;
  if (!gwen_exec_capture(argv, &stdout_text, &stderr_text, &exit_code, error)) return false;
  stderr_text = gwen_string_trim(stderr_text);
  if (exit_code != 0) {
    if (error != NULL) *error = stderr_text[0] != '\0' ? stderr_text : "sqlite exec failed";
    return false;
  }
  if (!gwen_parse_long_long_text(stdout_text, &changes)) {
    if (error != NULL) *error = "failed to read sqlite changes";
    return false;
  }
  if (affected != NULL) *affected = changes;
  return true;
}

static bool gwen_sqlite_query_path(const char *path, const char *sql, gwen_value params, gwen_value *rows, const char **error) {
  char **argv = NULL;
  const char *stdout_text = "";
  const char *stderr_text = "";
  const char *json_text = "";
  int exit_code = 0;
  gwen_value parsed = gwen_value_null();
  if (!gwen_sqlite_build_argv(path, sql, params, true, false, &argv, error)) return false;
  if (!gwen_exec_capture(argv, &stdout_text, &stderr_text, &exit_code, error)) return false;
  json_text = gwen_string_trim(stdout_text);
  stderr_text = gwen_string_trim(stderr_text);
  if (exit_code != 0) {
    if (error != NULL) *error = stderr_text[0] != '\0' ? stderr_text : "sqlite query failed";
    return false;
  }
  if (json_text[0] == '\0') {
    parsed = gwen_value_list_from_ptr(gwen_dyn_list_new(0));
  } else if (!gwen_json_parse_root(json_text, &parsed, error)) {
    return false;
  }
  if (parsed.kind != GWEN_VALUE_LIST) {
    if (error != NULL) *error = "sqlite query expected json array";
    return false;
  }
  if (rows != NULL) *rows = parsed;
  return true;
}

`)
	return nil
}

func (e *emitter) emitListTypes() error {
	for _, key := range e.listOrder {
		typeName := e.listByKey[key]
		itemType := e.listItems[key]
		e.writeLine(fmt.Sprintf("struct %s {", typeName))
		e.writeLine("  long long len;")
		e.writeLine(fmt.Sprintf("  %s *items;", itemType))
		e.writeLine("};")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitListHelperPrototypes() error {
	emitted := 0
	if stringListType, ok := e.listByKey["const char *"]; ok {
		e.writeLine(fmt.Sprintf("static %s gwen_string_split(const char *value, const char *sep);", stringListType))
		emitted++
	}
	for _, key := range e.listOrder {
		typeName := e.listByKey[key]
		itemHIR := e.listItemHIR[key]
		if _, err := e.cloneExpr("items.items[i]", itemHIR); err == nil {
			e.writeLine(fmt.Sprintf("static %s %s(%s items);", typeName, listCloneFuncName(typeName), typeName))
			emitted++
		}
		if _, err := e.dynamicValueExpr("items.items[i]", itemHIR); err == nil {
			e.writeLine(fmt.Sprintf("static gwen_value %s(%s items);", listToValueFuncName(typeName), typeName))
			emitted++
		}
		if _, err := e.coerceDynamicExpr("source->items[i]", itemHIR); err == nil {
			e.writeLine(fmt.Sprintf("static %s %s(gwen_value value);", typeName, listFromValueFuncName(typeName)))
			emitted++
		}
		if _, _, err := e.displayStringHelper(itemHIR); err == nil {
			e.writeLine(fmt.Sprintf("static const char *%s(%s items, const char *sep);", listJoinFuncName(typeName), typeName))
			emitted++
		}
		sortable := isStringType(itemHIR) || isBoolType(itemHIR) || isNumericType(itemHIR)
		if !sortable {
			continue
		}
		for _, descending := range []bool{false, true} {
			e.writeLine(fmt.Sprintf("static %s %s(%s items);", typeName, listSortFuncName(typeName, descending), typeName))
			emitted++
		}
	}
	if emitted > 0 {
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitListHelpers() error {
	if stringListType, ok := e.listByKey["const char *"]; ok {
		e.writeLine(fmt.Sprintf("static %s gwen_string_split(const char *value, const char *sep) {", stringListType))
		e.writeLine(fmt.Sprintf("  %s result;", stringListType))
		e.writeLine("  const char *safe_value = value != NULL ? value : \"\";")
		e.writeLine("  const char *safe_sep = sep != NULL ? sep : \"\";")
		e.writeLine("  size_t value_len = strlen(safe_value);")
		e.writeLine("  size_t sep_len = strlen(safe_sep);")
		e.writeLine("  result.len = 0;")
		e.writeLine("  result.items = NULL;")
		e.writeLine("  if (sep_len == 0U) {")
		e.writeLine("    result.len = (long long)value_len;")
		e.writeLine("    if (value_len == 0U) return result;")
		e.writeLine("    result.items = (const char **)malloc(sizeof(const char *) * value_len);")
		e.writeLine("    if (result.items == NULL) gwen_runtime_error(\"runtime error: out of memory splitting string\");")
		e.writeLine("    for (size_t i = 0U; i < value_len; ++i) {")
		e.writeLine("      result.items[i] = gwen_string_dup_len(safe_value + i, 1U);")
		e.writeLine("    }")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  long long parts = 1;")
		e.writeLine("  const char *cursor = safe_value;")
		e.writeLine("  const char *match = NULL;")
		e.writeLine("  while ((match = strstr(cursor, safe_sep)) != NULL) {")
		e.writeLine("    parts++;")
		e.writeLine("    cursor = match + sep_len;")
		e.writeLine("  }")
		e.writeLine("  result.len = parts;")
		e.writeLine("  result.items = (const char **)malloc(sizeof(const char *) * (size_t)parts);")
		e.writeLine("  if (result.items == NULL) gwen_runtime_error(\"runtime error: out of memory splitting string\");")
		e.writeLine("  cursor = safe_value;")
		e.writeLine("  long long index = 0;")
		e.writeLine("  while ((match = strstr(cursor, safe_sep)) != NULL) {")
		e.writeLine("    result.items[index++] = gwen_string_dup_len(cursor, (size_t)(match - cursor));")
		e.writeLine("    cursor = match + sep_len;")
		e.writeLine("  }")
		e.writeLine("  result.items[index] = gwen_string_dup(cursor);")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	for _, key := range e.listOrder {
		typeName := e.listByKey[key]
		e.writeLine(fmt.Sprintf("static void %s(%s *list, %s item) {", listAppendFuncName(typeName), typeName, key))
		e.writeLine("  if (list == NULL) gwen_runtime_error(\"runtime error: append() requires list target\");")
		e.writeLine("  long long new_len = list->len + 1LL;")
		e.writeLine(fmt.Sprintf("  %s *items = (%s *)realloc(list->items, sizeof(%s) * (size_t)new_len);", key, key, key))
		e.writeLine(fmt.Sprintf("  if (items == NULL) gwen_runtime_error(\"runtime error: out of memory appending %s\");", typeName))
		e.writeLine("  list->items = items;")
		e.writeLine("  list->items[new_len - 1LL] = item;")
		e.writeLine("  list->len = new_len;")
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static void %s(%s *list, long long index) {", listRemoveAtFuncName(typeName), typeName))
		e.writeLine("  if (list == NULL) gwen_runtime_error(\"runtime error: removeat() requires list target\");")
		e.writeLine("  if (index < 0LL || index >= list->len) gwen_runtime_error(\"runtime error: removeat() index out of range\");")
		e.writeLine("  for (long long i = index + 1LL; i < list->len; ++i) list->items[i - 1LL] = list->items[i];")
		e.writeLine("  list->len--;")
		e.writeLine("  if (list->len <= 0LL) {")
		e.writeLine("    free(list->items);")
		e.writeLine("    list->items = NULL;")
		e.writeLine("    list->len = 0LL;")
		e.writeLine("    return;")
		e.writeLine("  }")
		e.writeLine(fmt.Sprintf("  %s *items = (%s *)realloc(list->items, sizeof(%s) * (size_t)list->len);", key, key, key))
		e.writeLine("  if (items != NULL) list->items = items;")
		e.writeLine("}")
		e.writeLine("")
		itemHIR := e.listItemHIR[key]
		itemCloneExpr, err := e.cloneExpr("items.items[i]", itemHIR)
		if err == nil {
			e.writeLine(fmt.Sprintf("static %s %s(%s items) {", typeName, listCloneFuncName(typeName), typeName))
			e.writeLine(fmt.Sprintf("  %s result;", typeName))
			e.writeLine("  result.len = items.len;")
			e.writeLine("  if (items.len == 0) {")
			e.writeLine("    result.items = NULL;")
			e.writeLine("    return result;")
			e.writeLine("  }")
			e.writeLine(fmt.Sprintf("  result.items = (%s *)malloc(sizeof(%s) * (size_t)items.len);", key, key))
			e.writeLine(fmt.Sprintf("  if (result.items == NULL) gwen_runtime_error(\"runtime error: out of memory cloning %s\");", typeName))
			e.writeLine("  for (long long i = 0; i < items.len; ++i) {")
			e.writeLine(fmt.Sprintf("    result.items[i] = %s;", itemCloneExpr))
			e.writeLine("  }")
			e.writeLine("  return result;")
			e.writeLine("}")
			e.writeLine("")
		}
		itemDynamicExpr, err := e.dynamicValueExpr("items.items[i]", itemHIR)
		if err == nil {
			e.writeLine(fmt.Sprintf("static gwen_value %s(%s items) {", listToValueFuncName(typeName), typeName))
			e.writeLine("  gwen_dyn_list *result = gwen_dyn_list_new(items.len);")
			e.writeLine("  for (long long i = 0; i < items.len; ++i) {")
			e.writeLine(fmt.Sprintf("    result->items[i] = %s;", itemDynamicExpr))
			e.writeLine("  }")
			e.writeLine("  return gwen_value_list_from_ptr(result);")
			e.writeLine("}")
			e.writeLine("")
		}
		itemFromDynamicExpr, err := e.coerceDynamicExpr("source->items[i]", itemHIR)
		if err == nil {
			e.writeLine(fmt.Sprintf("static %s %s(gwen_value value) {", typeName, listFromValueFuncName(typeName)))
			e.writeLine(fmt.Sprintf("  %s result;", typeName))
			e.writeLine("  gwen_dyn_list *source = gwen_value_as_list(value);")
			e.writeLine("  result.len = source->len;")
			e.writeLine("  if (source->len == 0) {")
			e.writeLine("    result.items = NULL;")
			e.writeLine("    return result;")
			e.writeLine("  }")
			e.writeLine(fmt.Sprintf("  result.items = (%s *)malloc(sizeof(%s) * (size_t)source->len);", key, key))
			e.writeLine(fmt.Sprintf("  if (result.items == NULL) gwen_runtime_error(\"runtime error: out of memory converting to %s\");", typeName))
			e.writeLine("  for (long long i = 0; i < source->len; ++i) {")
			e.writeLine(fmt.Sprintf("    result.items[i] = %s;", itemFromDynamicExpr))
			e.writeLine("  }")
			e.writeLine("  return result;")
			e.writeLine("}")
			e.writeLine("")
		}
		helperName, castType, err := e.displayStringHelper(itemHIR)
		if err != nil {
			continue
		}
		e.writeLine(fmt.Sprintf("static const char *%s(%s items, const char *sep) {", listJoinFuncName(typeName), typeName))
		e.writeLine("  const char *safe_sep = sep != NULL ? sep : \"\";")
		e.writeLine("  size_t sep_len = strlen(safe_sep);")
		e.writeLine("  if (items.len == 0) return gwen_string_dup(\"\");")
		e.writeLine("  const char **parts = (const char **)malloc(sizeof(const char *) * (size_t)items.len);")
		e.writeLine("  if (parts == NULL) gwen_runtime_error(\"runtime error: out of memory joining list\");")
		e.writeLine("  size_t total = 0U;")
		e.writeLine("  for (long long i = 0; i < items.len; ++i) {")
		itemExpr := fmt.Sprintf("items.items[i]")
		if castType != "" {
			itemExpr = fmt.Sprintf("(%s)(%s)", castType, itemExpr)
		}
		e.writeLine(fmt.Sprintf("    parts[i] = %s(%s);", helperName, itemExpr))
		e.writeLine("    total += strlen(parts[i]);")
		e.writeLine("  }")
		e.writeLine("  if (items.len > 1 && sep_len > 0U) total += sep_len * (size_t)(items.len - 1);")
		e.writeLine("  char *result = (char *)malloc(total + 1U);")
		e.writeLine("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory joining list\");")
		e.writeLine("  size_t pos = 0U;")
		e.writeLine("  for (long long i = 0; i < items.len; ++i) {")
		e.writeLine("    size_t part_len = strlen(parts[i]);")
		e.writeLine("    if (part_len > 0U) { memcpy(result + pos, parts[i], part_len); pos += part_len; }")
		e.writeLine("    if (i + 1 < items.len && sep_len > 0U) { memcpy(result + pos, safe_sep, sep_len); pos += sep_len; }")
		e.writeLine("    free((void *)parts[i]);")
		e.writeLine("  }")
		e.writeLine("  free(parts);")
		e.writeLine("  result[pos] = '\\0';")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
		sortable := isStringType(itemHIR) || isBoolType(itemHIR) || isNumericType(itemHIR)
		if !sortable {
			continue
		}
		cmpAscName := listSortCmpFuncName(typeName, false)
		cmpDescName := listSortCmpFuncName(typeName, true)
		e.writeLine(fmt.Sprintf("static int %s(const void *left, const void *right) {", cmpAscName))
		switch {
		case isStringType(itemHIR):
			e.writeLine("  const char *left_value = *(const char * const *)left;")
			e.writeLine("  const char *right_value = *(const char * const *)right;")
			e.writeLine("  return gwen_string_cmp(left_value, right_value);")
		case isBoolType(itemHIR):
			e.writeLine("  bool left_value = *(const bool *)left;")
			e.writeLine("  bool right_value = *(const bool *)right;")
			e.writeLine("  if (left_value == right_value) return 0;")
			e.writeLine("  return left_value ? 1 : -1;")
		default:
			e.writeLine(fmt.Sprintf("  %s left_value = *(const %s *)left;", key, key))
			e.writeLine(fmt.Sprintf("  %s right_value = *(const %s *)right;", key, key))
			e.writeLine("  if (left_value < right_value) return -1;")
			e.writeLine("  if (left_value > right_value) return 1;")
			e.writeLine("  return 0;")
		}
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static int %s(const void *left, const void *right) {", cmpDescName))
		e.writeLine(fmt.Sprintf("  return %s(right, left);", cmpAscName))
		e.writeLine("}")
		e.writeLine("")
		for _, descending := range []bool{false, true} {
			helper := listSortFuncName(typeName, descending)
			cmp := cmpAscName
			if descending {
				cmp = cmpDescName
			}
			e.writeLine(fmt.Sprintf("static %s %s(%s items) {", typeName, helper, typeName))
			e.writeLine(fmt.Sprintf("  %s result;", typeName))
			e.writeLine("  result.len = items.len;")
			e.writeLine("  if (items.len == 0) {")
			e.writeLine("    result.items = NULL;")
			e.writeLine("    return result;")
			e.writeLine("  }")
			e.writeLine(fmt.Sprintf("  result.items = (%s *)malloc(sizeof(%s) * (size_t)items.len);", key, key))
			e.writeLine(fmt.Sprintf("  if (result.items == NULL) gwen_runtime_error(\"runtime error: out of memory sorting %s\");", typeName))
			e.writeLine("  for (long long i = 0; i < items.len; ++i) result.items[i] = items.items[i];")
			e.writeLine(fmt.Sprintf("  if (result.len > 1) qsort(result.items, (size_t)result.len, sizeof(%s), %s);", key, cmp))
			e.writeLine("  return result;")
			e.writeLine("}")
			e.writeLine("")
		}
	}
	return nil
}

func (e *emitter) emitDictTypes() error {
	for _, key := range e.dictOrder {
		typeName := e.dictByKey[key]
		keyType := e.dictKeys[key]
		valueType := e.dictValues[key]
		e.writeLine(fmt.Sprintf("struct %s {", typeName))
		e.writeLine("  long long len;")
		e.writeLine(fmt.Sprintf("  %s *keys;", keyType))
		e.writeLine(fmt.Sprintf("  %s *values;", valueType))
		e.writeLine("};")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitDictHelperPrototypes() error {
	emitted := 0
	for _, key := range e.dictOrder {
		typeName := e.dictByKey[key]
		keyHIR := e.dictKeyHIR[key]
		valueHIR := e.dictValueHIR[key]
		if _, err := e.cloneExpr("dict.keys[i]", keyHIR); err == nil {
			if _, err := e.cloneExpr("dict.values[i]", valueHIR); err == nil {
				e.writeLine(fmt.Sprintf("static %s %s(%s dict);", typeName, dictCloneFuncName(typeName), typeName))
				emitted++
			}
		}
		if _, err := e.dynamicValueExpr("dict.keys[i]", keyHIR); err == nil {
			if _, err := e.dynamicValueExpr("dict.values[i]", valueHIR); err == nil {
				e.writeLine(fmt.Sprintf("static gwen_value %s(%s dict);", dictToValueFuncName(typeName), typeName))
				emitted++
			}
		}
		if _, err := e.coerceDynamicExpr("source->keys[i]", keyHIR); err == nil {
			if _, err := e.coerceDynamicExpr("source->values[i]", valueHIR); err == nil {
				e.writeLine(fmt.Sprintf("static %s %s(gwen_value value);", typeName, dictFromValueFuncName(typeName)))
				emitted++
			}
		}
	}
	if emitted > 0 {
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitDictHelpers() error {
	for _, key := range e.dictOrder {
		typeName := e.dictByKey[key]
		keyType := e.dictKeys[key]
		valueType := e.dictValues[key]
		keyHIR := e.dictKeyHIR[key]
		valueHIR := e.dictValueHIR[key]
		eqExpr, err := dictKeyEqExpr("dict.keys[i]", "key", keyHIR)
		if err != nil {
			return err
		}
		zeroValue, err := e.zeroExpr(valueHIR)
		if err != nil {
			return err
		}
		keyCloneExpr, keyCloneErr := e.cloneExpr("dict.keys[i]", keyHIR)
		valueCloneExpr, valueCloneErr := e.cloneExpr("dict.values[i]", valueHIR)
		if keyCloneErr == nil && valueCloneErr == nil {
			e.writeLine(fmt.Sprintf("static %s %s(%s dict) {", typeName, dictCloneFuncName(typeName), typeName))
			e.writeLine(fmt.Sprintf("  %s result;", typeName))
			e.writeLine("  result.len = dict.len;")
			e.writeLine("  if (dict.len == 0) {")
			e.writeLine("    result.keys = NULL;")
			e.writeLine("    result.values = NULL;")
			e.writeLine("    return result;")
			e.writeLine("  }")
			e.writeLine(fmt.Sprintf("  result.keys = (%s *)malloc(sizeof(%s) * (size_t)dict.len);", keyType, keyType))
			e.writeLine(fmt.Sprintf("  result.values = (%s *)malloc(sizeof(%s) * (size_t)dict.len);", valueType, valueType))
			e.writeLine(fmt.Sprintf("  if (result.keys == NULL || result.values == NULL) gwen_runtime_error(\"runtime error: out of memory cloning %s\");", typeName))
			e.writeLine("  for (long long i = 0; i < dict.len; ++i) {")
			e.writeLine(fmt.Sprintf("    result.keys[i] = %s;", keyCloneExpr))
			e.writeLine(fmt.Sprintf("    result.values[i] = %s;", valueCloneExpr))
			e.writeLine("  }")
			e.writeLine("  return result;")
			e.writeLine("}")
			e.writeLine("")
		}
		keyDynamicExpr, keyDynamicErr := e.dynamicValueExpr("dict.keys[i]", keyHIR)
		valueDynamicExpr, valueDynamicErr := e.dynamicValueExpr("dict.values[i]", valueHIR)
		if keyDynamicErr == nil && valueDynamicErr == nil {
			e.writeLine(fmt.Sprintf("static gwen_value %s(%s dict) {", dictToValueFuncName(typeName), typeName))
			e.writeLine("  gwen_dyn_dict *result = gwen_dyn_dict_new(dict.len);")
			e.writeLine("  for (long long i = 0; i < dict.len; ++i) {")
			e.writeLine(fmt.Sprintf("    result->keys[i] = %s;", keyDynamicExpr))
			e.writeLine(fmt.Sprintf("    result->values[i] = %s;", valueDynamicExpr))
			e.writeLine("  }")
			e.writeLine("  return gwen_value_dict_from_ptr(result);")
			e.writeLine("}")
			e.writeLine("")
		}
		keyFromDynamicExpr, keyFromDynamicErr := e.coerceDynamicExpr("source->keys[i]", keyHIR)
		valueFromDynamicExpr, valueFromDynamicErr := e.coerceDynamicExpr("source->values[i]", valueHIR)
		if keyFromDynamicErr == nil && valueFromDynamicErr == nil {
			e.writeLine(fmt.Sprintf("static %s %s(gwen_value value) {", typeName, dictFromValueFuncName(typeName)))
			e.writeLine(fmt.Sprintf("  %s result;", typeName))
			e.writeLine("  gwen_dyn_dict *source = gwen_value_as_dict(value);")
			e.writeLine("  result.len = source->len;")
			e.writeLine("  if (source->len == 0) {")
			e.writeLine("    result.keys = NULL;")
			e.writeLine("    result.values = NULL;")
			e.writeLine("    return result;")
			e.writeLine("  }")
			e.writeLine(fmt.Sprintf("  result.keys = (%s *)malloc(sizeof(%s) * (size_t)source->len);", keyType, keyType))
			e.writeLine(fmt.Sprintf("  result.values = (%s *)malloc(sizeof(%s) * (size_t)source->len);", valueType, valueType))
			e.writeLine(fmt.Sprintf("  if (result.keys == NULL || result.values == NULL) gwen_runtime_error(\"runtime error: out of memory converting to %s\");", typeName))
			e.writeLine("  for (long long i = 0; i < source->len; ++i) {")
			e.writeLine(fmt.Sprintf("    result.keys[i] = %s;", keyFromDynamicExpr))
			e.writeLine(fmt.Sprintf("    result.values[i] = %s;", valueFromDynamicExpr))
			e.writeLine("  }")
			e.writeLine("  return result;")
			e.writeLine("}")
			e.writeLine("")
		}
		e.writeLine(fmt.Sprintf("static bool %s(%s dict, %s key) {", dictHasKeyFuncName(typeName), typeName, keyType))
		e.writeLine("  for (long long i = dict.len - 1; i >= 0; --i) {")
		e.writeLine(fmt.Sprintf("    if (%s) return true;", eqExpr))
		e.writeLine("  }")
		e.writeLine("  return false;")
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static %s %s(%s dict, %s key, %s fallback) {", valueType, dictGetFuncName(typeName), typeName, keyType, valueType))
		e.writeLine("  for (long long i = dict.len - 1; i >= 0; --i) {")
		e.writeLine(fmt.Sprintf("    if (%s) return dict.values[i];", eqExpr))
		e.writeLine("  }")
		e.writeLine("  return fallback;")
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static %s %s(%s dict, %s key) {", valueType, dictIndexFuncName(typeName), typeName, keyType))
		e.writeLine("  for (long long i = dict.len - 1; i >= 0; --i) {")
		e.writeLine(fmt.Sprintf("    if (%s) return dict.values[i];", eqExpr))
		e.writeLine("  }")
		e.writeLine("  gwen_runtime_error(\"runtime error: key not found\");")
		e.writeLine(fmt.Sprintf("  return %s;", zeroValue))
		e.writeLine("}")
		e.writeLine("")
		ptrEqExpr, err := dictKeyEqExpr("dict->keys[i]", "key", keyHIR)
		if err != nil {
			return err
		}
		e.writeLine(fmt.Sprintf("static void %s(%s *dict, %s key, %s value) {", dictSetFuncName(typeName), typeName, keyType, valueType))
		e.writeLine("  for (long long i = dict->len - 1; i >= 0; --i) {")
		e.writeLine(fmt.Sprintf("    if (%s) {", ptrEqExpr))
		e.writeLine("      dict->values[i] = value;")
		e.writeLine("      return;")
		e.writeLine("    }")
		e.writeLine("  }")
		e.writeLine("  long long new_len = dict->len + 1;")
		e.writeLine(fmt.Sprintf("  %s *new_keys = (%s *)realloc(dict->keys, sizeof(%s) * (size_t)new_len);", keyType, keyType, keyType))
		e.writeLine(fmt.Sprintf("  %s *new_values = (%s *)realloc(dict->values, sizeof(%s) * (size_t)new_len);", valueType, valueType, valueType))
		e.writeLine("  if (new_keys == NULL || new_values == NULL) gwen_runtime_error(\"runtime error: out of memory growing dict\");")
		e.writeLine("  dict->keys = new_keys;")
		e.writeLine("  dict->values = new_values;")
		e.writeLine("  dict->keys[dict->len] = key;")
		e.writeLine("  dict->values[dict->len] = value;")
		e.writeLine("  dict->len = new_len;")
		e.writeLine("}")
		e.writeLine("")
		keyListType, err := e.listTypeName(&hir.GenericType{Base: "list", Args: []hir.Type{keyHIR}})
		if err != nil {
			return err
		}
		valueListType, err := e.listTypeName(&hir.GenericType{Base: "list", Args: []hir.Type{valueHIR}})
		if err != nil {
			return err
		}
		e.writeLine(fmt.Sprintf("static %s %s(%s dict) {", keyListType, dictKeysFuncName(typeName), typeName))
		e.writeLine(fmt.Sprintf("  %s result;", keyListType))
		e.writeLine("  result.len = dict.len;")
		e.writeLine("  if (dict.len == 0) {")
		e.writeLine("    result.items = NULL;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine(fmt.Sprintf("  result.items = (%s *)malloc(sizeof(%s) * (size_t)dict.len);", keyType, keyType))
		e.writeLine(fmt.Sprintf("  if (result.items == NULL) gwen_runtime_error(\"runtime error: out of memory allocating %s keys\");", typeName))
		e.writeLine("  for (long long i = 0; i < dict.len; ++i) {")
		e.writeLine("    result.items[i] = dict.keys[i];")
		e.writeLine("  }")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static %s %s(%s dict) {", valueListType, dictValuesFuncName(typeName), typeName))
		e.writeLine(fmt.Sprintf("  %s result;", valueListType))
		e.writeLine("  result.len = dict.len;")
		e.writeLine("  if (dict.len == 0) {")
		e.writeLine("    result.items = NULL;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine(fmt.Sprintf("  result.items = (%s *)malloc(sizeof(%s) * (size_t)dict.len);", valueType, valueType))
		e.writeLine(fmt.Sprintf("  if (result.items == NULL) gwen_runtime_error(\"runtime error: out of memory allocating %s values\");", typeName))
		e.writeLine("  for (long long i = 0; i < dict.len; ++i) {")
		e.writeLine("    result.items[i] = dict.values[i];")
		e.writeLine("  }")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitResultTypes() error {
	for _, key := range e.resultOrder {
		typeName := e.resultByKey[key]
		okType := e.resultOK[key]
		errType := e.resultErr[key]
		e.writeLine(fmt.Sprintf("struct %s {", typeName))
		e.writeLine("  bool is_ok;")
		e.writeLine(fmt.Sprintf("  %s ok;", okType))
		e.writeLine(fmt.Sprintf("  %s err;", errType))
		e.writeLine("};")
		e.writeLine("")
		okHIR := e.resultOKHIR[key]
		errHIR := e.resultErrHIR[key]
		okCloneExpr, okErr := e.cloneExpr("value.ok", okHIR)
		errCloneExpr, errErr := e.cloneExpr("value.err", errHIR)
		zeroOKExpr, zeroOKErr := e.zeroExpr(okHIR)
		zeroErrExpr, zeroErrErr := e.zeroExpr(errHIR)
		if okErr == nil && errErr == nil && zeroOKErr == nil && zeroErrErr == nil {
			e.writeLine(fmt.Sprintf("static %s %s(%s value) {", typeName, resultCloneFuncName(typeName), typeName))
			e.writeLine("  if (value.is_ok) {")
			e.writeLine(fmt.Sprintf("    return (%s){true, %s, %s};", typeName, okCloneExpr, zeroErrExpr))
			e.writeLine("  }")
			e.writeLine(fmt.Sprintf("  return (%s){false, %s, %s};", typeName, zeroOKExpr, errCloneExpr))
			e.writeLine("}")
			e.writeLine("")
		}
	}
	return nil
}

func (e *emitter) emitCellTypes() error {
	for _, key := range e.cellOrder {
		typeName := e.cellByKey[key]
		itemType := e.cellItems[key]
		e.writeLine(fmt.Sprintf("struct %s {", typeName))
		e.writeLine("  pthread_mutex_t *mutex;")
		e.writeLine(fmt.Sprintf("  %s *value;", itemType))
		e.writeLine("};")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitCellHelperPrototypes() error {
	emitted := 0
	for _, key := range e.cellOrder {
		typeName := e.cellByKey[key]
		itemHIR := e.cellItemHIR[key]
		if _, err := e.cloneExpr("value", itemHIR); err != nil {
			return err
		}
		if _, err := e.cloneExpr("*(cell.value)", itemHIR); err != nil {
			return err
		}
		e.writeLine(fmt.Sprintf("static %s %s(%s cell);", typeName, cellCloneFuncName(typeName), typeName))
		emitted++
	}
	if emitted > 0 {
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitCellHelpers() error {
	for _, key := range e.cellOrder {
		typeName := e.cellByKey[key]
		itemType := e.cellItems[key]
		itemHIR := e.cellItemHIR[key]
		setCloneExpr, err := e.cloneExpr("value", itemHIR)
		if err != nil {
			return err
		}
		getCloneExpr, err := e.cloneExpr("*(cell.value)", itemHIR)
		if err != nil {
			return err
		}
		e.writeLine(fmt.Sprintf("static %s %s(%s value) {", typeName, cellNewFuncName(typeName), itemType))
		e.writeLine(fmt.Sprintf("  %s cell;", typeName))
		e.writeLine("  cell.mutex = (pthread_mutex_t *)malloc(sizeof(pthread_mutex_t));")
		e.writeLine(fmt.Sprintf("  cell.value = (%s *)malloc(sizeof(%s));", itemType, itemType))
		e.writeLine(fmt.Sprintf("  if (cell.mutex == NULL || cell.value == NULL) gwen_runtime_error(\"runtime error: out of memory allocating %s\");", typeName))
		e.writeLine("  gwen_pthread_require(pthread_mutex_init(cell.mutex, NULL), \"init state cell mutex\");")
		e.writeLine(fmt.Sprintf("  *(cell.value) = %s;", setCloneExpr))
		e.writeLine("  return cell;")
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static %s %s(%s cell) {", itemType, cellGetFuncName(typeName), typeName))
		e.writeLine(fmt.Sprintf("  %s snapshot;", itemType))
		e.writeLine("  if (cell.mutex == NULL || cell.value == NULL) gwen_runtime_error(\"runtime error: state.get() requires initialized cell\");")
		e.writeLine("  gwen_pthread_require(pthread_mutex_lock(cell.mutex), \"lock state cell\");")
		e.writeLine("  snapshot = *(cell.value);")
		e.writeLine("  gwen_pthread_require(pthread_mutex_unlock(cell.mutex), \"unlock state cell\");")
		e.writeLine(fmt.Sprintf("  return %s;", strings.ReplaceAll(getCloneExpr, "*(cell.value)", "snapshot")))
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static %s %s(%s cell, %s value) {", itemType, cellSetFuncName(typeName), typeName, itemType))
		e.writeLine(fmt.Sprintf("  %s stored = %s;", itemType, setCloneExpr))
		e.writeLine("  if (cell.mutex == NULL || cell.value == NULL) gwen_runtime_error(\"runtime error: state.set() requires initialized cell\");")
		e.writeLine("  gwen_pthread_require(pthread_mutex_lock(cell.mutex), \"lock state cell\");")
		e.writeLine("  *(cell.value) = stored;")
		e.writeLine("  gwen_pthread_require(pthread_mutex_unlock(cell.mutex), \"unlock state cell\");")
		e.writeLine(fmt.Sprintf("  return %s;", strings.ReplaceAll(getCloneExpr, "*(cell.value)", "stored")))
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static %s %s(%s cell) {", typeName, cellCloneFuncName(typeName), typeName))
		e.writeLine(fmt.Sprintf("  return %s(%s(cell));", cellNewFuncName(typeName), cellGetFuncName(typeName)))
		e.writeLine("}")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitIOHelpers() error {
	stringResultType, hasStringResult := e.resultByKey["const char *|const char *"]
	intResultType, hasIntResult := e.resultByKey["long long|const char *"]
	stringListType, hasStringList := e.listByKey["const char *"]
	readdirResultType := ""
	if hasStringList {
		readdirResultType = e.resultByKey[stringListType+"|const char *"]
	}
	if hasStringResult {
		e.writeLine(fmt.Sprintf("static %s gwen_io_readfile(const char *path) {", stringResultType))
		e.writeLine(fmt.Sprintf("  %s result;", stringResultType))
		e.writeLine("  const char *safe_path = path != NULL ? path : \"\";")
		e.writeLine("  FILE *file = fopen(safe_path, \"rb\");")
		e.writeLine("  if (file == NULL) {")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = NULL;")
		e.writeLine("    result.err = gwen_message_with_path(\"open\", safe_path, strerror(errno));")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  if (fseek(file, 0, SEEK_END) != 0) {")
		e.writeLine("    const char *message = gwen_message_with_path(\"read\", safe_path, strerror(errno));")
		e.writeLine("    fclose(file);")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = NULL;")
		e.writeLine("    result.err = message;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  long size = ftell(file);")
		e.writeLine("  if (size < 0) {")
		e.writeLine("    const char *message = gwen_message_with_path(\"read\", safe_path, strerror(errno));")
		e.writeLine("    fclose(file);")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = NULL;")
		e.writeLine("    result.err = message;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  rewind(file);")
		e.writeLine("  char *buffer = (char *)malloc((size_t)size + 1U);")
		e.writeLine("  if (buffer == NULL) gwen_runtime_error(\"runtime error: out of memory reading file\");")
		e.writeLine("  size_t read_count = fread(buffer, 1U, (size_t)size, file);")
		e.writeLine("  if (ferror(file)) {")
		e.writeLine("    const char *message = gwen_message_with_path(\"read\", safe_path, strerror(errno));")
		e.writeLine("    fclose(file);")
		e.writeLine("    free(buffer);")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = NULL;")
		e.writeLine("    result.err = message;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  fclose(file);")
		e.writeLine("  buffer[read_count] = '\\0';")
		e.writeLine("  result.is_ok = true;")
		e.writeLine("  result.ok = buffer;")
		e.writeLine("  result.err = NULL;")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	if hasStringList && readdirResultType != "" {
		e.writeLine(fmt.Sprintf("static %s gwen_io_readdir(const char *path) {", readdirResultType))
		e.writeLine(fmt.Sprintf("  %s result;", readdirResultType))
		e.writeLine(fmt.Sprintf("  %s ok_value;", stringListType))
		e.writeLine("  const char *safe_path = path != NULL ? path : \"\";")
		e.writeLine("  DIR *dir = opendir(safe_path);")
		e.writeLine("  ok_value.len = 0;")
		e.writeLine("  ok_value.items = NULL;")
		e.writeLine("  if (dir == NULL) {")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = ok_value;")
		e.writeLine("    result.err = gwen_message_with_path(\"open\", safe_path, strerror(errno));")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  errno = 0;")
		e.writeLine("  for (;;) {")
		e.writeLine("    struct dirent *entry = readdir(dir);")
		e.writeLine("    if (entry == NULL) break;")
		e.writeLine("    if (strcmp(entry->d_name, \".\") == 0 || strcmp(entry->d_name, \"..\") == 0) continue;")
		e.writeLine("    long long new_len = ok_value.len + 1;")
		e.writeLine("    const char **new_items = (const char **)realloc(ok_value.items, sizeof(const char *) * (size_t)new_len);")
		e.writeLine("    if (new_items == NULL) gwen_runtime_error(\"runtime error: out of memory reading directory\");")
		e.writeLine("    ok_value.items = new_items;")
		e.writeLine("    ok_value.items[ok_value.len] = gwen_string_dup(entry->d_name);")
		e.writeLine("    ok_value.len = new_len;")
		e.writeLine("    errno = 0;")
		e.writeLine("  }")
		e.writeLine("  if (errno != 0) {")
		e.writeLine("    const char *message = gwen_message_with_path(\"read\", safe_path, strerror(errno));")
		e.writeLine("    closedir(dir);")
		e.writeLine("    for (long long i = 0; i < ok_value.len; ++i) free((void *)ok_value.items[i]);")
		e.writeLine("    free(ok_value.items);")
		e.writeLine("    ok_value.len = 0;")
		e.writeLine("    ok_value.items = NULL;")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = ok_value;")
		e.writeLine("    result.err = message;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  closedir(dir);")
		e.writeLine("  if (ok_value.len > 1) qsort(ok_value.items, (size_t)ok_value.len, sizeof(const char *), gwen_string_ptr_cmp);")
		e.writeLine("  result.is_ok = true;")
		e.writeLine("  result.ok = ok_value;")
		e.writeLine("  result.err = NULL;")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	if hasIntResult {
		e.writeLine(fmt.Sprintf("static %s gwen_io_writefile(const char *path, const char *content) {", intResultType))
		e.writeLine(fmt.Sprintf("  %s result;", intResultType))
		e.writeLine("  const char *safe_path = path != NULL ? path : \"\";")
		e.writeLine("  const char *safe_content = content != NULL ? content : \"\";")
		e.writeLine("  FILE *file = fopen(safe_path, \"wb\");")
		e.writeLine("  if (file == NULL) {")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = 0LL;")
		e.writeLine("    result.err = gwen_message_with_path(\"open\", safe_path, strerror(errno));")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  size_t content_len = strlen(safe_content);")
		e.writeLine("  size_t written = fwrite(safe_content, 1U, content_len, file);")
		e.writeLine("  int close_err = fclose(file);")
		e.writeLine("  if (written != content_len || close_err != 0) {")
		e.writeLine("    const char *message = gwen_message_with_path(\"write\", safe_path, errno != 0 ? strerror(errno) : \"short write\");")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = 0LL;")
		e.writeLine("    result.err = message;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  result.is_ok = true;")
		e.writeLine("  result.ok = (long long)written;")
		e.writeLine("  result.err = NULL;")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static %s gwen_io_appendfile(const char *path, const char *content) {", intResultType))
		e.writeLine(fmt.Sprintf("  %s result;", intResultType))
		e.writeLine("  const char *safe_path = path != NULL ? path : \"\";")
		e.writeLine("  const char *safe_content = content != NULL ? content : \"\";")
		e.writeLine("  FILE *file = fopen(safe_path, \"ab\");")
		e.writeLine("  if (file == NULL) {")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = 0LL;")
		e.writeLine("    result.err = gwen_message_with_path(\"open\", safe_path, strerror(errno));")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  size_t content_len = strlen(safe_content);")
		e.writeLine("  size_t written = fwrite(safe_content, 1U, content_len, file);")
		e.writeLine("  int close_err = fclose(file);")
		e.writeLine("  if (written != content_len || close_err != 0) {")
		e.writeLine("    const char *message = gwen_message_with_path(\"write\", safe_path, errno != 0 ? strerror(errno) : \"short write\");")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = 0LL;")
		e.writeLine("    result.err = message;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  result.is_ok = true;")
		e.writeLine("  result.ok = (long long)written;")
		e.writeLine("  result.err = NULL;")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitOSTimeHelpers() error {
	stringResultType := e.resultByKey["const char *|const char *"]
	stringListType := e.listByKey["const char *"]
	if stringResultType != "" {
		e.writeLine(fmt.Sprintf("static %s gwen_os_getenv(const char *name) {", stringResultType))
		e.writeLine(fmt.Sprintf("  %s result;", stringResultType))
		e.writeLine("  const char *safe_name = name != NULL ? name : \"\";")
		e.writeLine("  const char *value = getenv(safe_name);")
		e.writeLine("  if (value == NULL) {")
		e.writeLine("    result.is_ok = false;")
		e.writeLine("    result.ok = NULL;")
		e.writeLine("    result.err = gwen_string_concat(\"environment variable not found: \", safe_name);")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  result.is_ok = true;")
		e.writeLine("  result.ok = gwen_string_dup(value);")
		e.writeLine("  result.err = NULL;")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	if stringListType != "" {
		e.writeLine(fmt.Sprintf("static %s gwen_os_args(void) {", stringListType))
		e.writeLine(fmt.Sprintf("  %s result;", stringListType))
		e.writeLine("  result.len = gwen_cli_argc > 1 ? (long long)(gwen_cli_argc - 1) : 0LL;")
		e.writeLine("  result.items = NULL;")
		e.writeLine("  if (result.len == 0) return result;")
		e.writeLine("  result.items = (const char **)malloc(sizeof(const char *) * (size_t)result.len);")
		e.writeLine(fmt.Sprintf("  if (result.items == NULL) gwen_runtime_error(\"runtime error: out of memory allocating %s\");", stringListType))
		e.writeLine("  for (long long i = 1; i < (long long)gwen_cli_argc; ++i) result.items[i - 1] = gwen_cli_argv[i];")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	e.writeLine("static const char *gwen_os_cwd(void) {")
	e.writeLine("  char *buffer = getcwd(NULL, 0U);")
	e.writeLine("  if (buffer == NULL) gwen_runtime_error(\"runtime error: cwd() failed\");")
	e.writeLine("  return buffer;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static long long gwen_time_nowunix(void) {")
	e.writeLine("  return (long long)time(NULL);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static long long gwen_time_nowunixms(void) {")
	e.writeLine("  struct timeval tv;")
	e.writeLine("  if (gettimeofday(&tv, NULL) != 0) gwen_runtime_error(\"runtime error: nowunixms() failed\");")
	e.writeLine("  return ((long long)tv.tv_sec * 1000LL) + ((long long)tv.tv_usec / 1000LL);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_time_sleep(long long ms) {")
	e.writeLine("  struct timespec req;")
	e.writeLine("  if (ms < 0LL) gwen_runtime_error(\"runtime error: sleep() duration must be >= 0\");")
	e.writeLine("  req.tv_sec = (time_t)(ms / 1000LL);")
	e.writeLine("  req.tv_nsec = (long)((ms % 1000LL) * 1000000LL);")
	e.writeLine("  while (nanosleep(&req, &req) != 0) {")
	e.writeLine("    if (errno != EINTR) gwen_runtime_error(\"runtime error: sleep() failed\");")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_time_nowrfc3339(void) {")
	e.writeLine("  time_t now = time(NULL);")
	e.writeLine("  struct tm tm_value;")
	e.writeLine("  char *buffer = (char *)malloc(32U);")
	e.writeLine("  if (buffer == NULL) gwen_runtime_error(\"runtime error: out of memory formatting time\");")
	e.writeLine("  if (gmtime_r(&now, &tm_value) == NULL) gwen_runtime_error(\"runtime error: nowrfc3339() failed\");")
	e.writeLine("  if (strftime(buffer, 32U, \"%Y-%m-%dT%H:%M:%SZ\", &tm_value) == 0U) gwen_runtime_error(\"runtime error: nowrfc3339() failed\");")
	e.writeLine("  return buffer;")
	e.writeLine("}")
	e.writeLine("")
	return nil
}

func (e *emitter) emitJSONHelpers() error {
	dynamicResultType := e.resultByKey["gwen_value|const char *"]
	if dynamicResultType != "" {
		e.writeLine(fmt.Sprintf("static %s gwen_json_parseobject_result(const char *text) {", dynamicResultType))
		e.writeLine(fmt.Sprintf("  %s result;", dynamicResultType))
		e.writeLine("  gwen_value parsed = gwen_value_null();")
		e.writeLine("  const char *error = NULL;")
		e.writeLine("  if (gwen_json_parse_root(text, &parsed, &error) && parsed.kind == GWEN_VALUE_DICT) {")
		e.writeLine("    result.is_ok = true;")
		e.writeLine("    result.ok = parsed;")
		e.writeLine("    result.err = NULL;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  result.is_ok = false;")
		e.writeLine("  result.ok = gwen_value_null();")
		e.writeLine("  result.err = error != NULL ? error : \"json.parseobject() requires top-level object\";")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static %s gwen_json_parsearray_result(const char *text) {", dynamicResultType))
		e.writeLine(fmt.Sprintf("  %s result;", dynamicResultType))
		e.writeLine("  gwen_value parsed = gwen_value_null();")
		e.writeLine("  const char *error = NULL;")
		e.writeLine("  if (gwen_json_parse_root(text, &parsed, &error) && parsed.kind == GWEN_VALUE_LIST) {")
		e.writeLine("    result.is_ok = true;")
		e.writeLine("    result.ok = parsed;")
		e.writeLine("    result.err = NULL;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  result.is_ok = false;")
		e.writeLine("  result.ok = gwen_value_null();")
		e.writeLine("  result.err = error != NULL ? error : \"json.parsearray() requires top-level array\";")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	stringResultType := e.resultByKey["const char *|const char *"]
	if stringResultType != "" {
		e.writeLine(fmt.Sprintf("static %s gwen_json_stringify_result(gwen_value value) {", stringResultType))
		e.writeLine(fmt.Sprintf("  %s result;", stringResultType))
		e.writeLine("  const char *text = NULL;")
		e.writeLine("  const char *error = NULL;")
		e.writeLine("  if (gwen_json_stringify_value(value, &text, &error)) {")
		e.writeLine("    result.is_ok = true;")
		e.writeLine("    result.ok = text;")
		e.writeLine("    result.err = NULL;")
		e.writeLine("    return result;")
		e.writeLine("  }")
		e.writeLine("  result.is_ok = false;")
		e.writeLine("  result.ok = NULL;")
		e.writeLine("  result.err = error != NULL ? error : \"json.stringify() failed\";")
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitTupleTypes() error {
	for _, key := range e.tupleOrder {
		typeName := e.tupleByKey[key]
		fields := e.tupleFields[key]
		hirTypes := e.tupleTypes[key]
		parts := strings.Split(key, "|")
		e.writeLine(fmt.Sprintf("struct %s {", typeName))
		for idx, part := range parts {
			e.writeLine(fmt.Sprintf("  %s %s;", part, fields[idx]))
		}
		e.writeLine("};")
		e.writeLine("")
		if len(hirTypes) != len(fields) {
			return fmt.Errorf("tuple %q metadata mismatch", typeName)
		}
		itemExprs := make([]string, 0, len(fields))
		convertible := true
		for idx, field := range fields {
			itemExpr, err := e.dynamicValueExpr("value."+field, hirTypes[idx])
			if err != nil {
				convertible = false
				break
			}
			itemExprs = append(itemExprs, itemExpr)
		}
		if !convertible {
			continue
		}
		e.writeLine(fmt.Sprintf("static gwen_value %s(%s value) {", tupleToValueFuncName(typeName), typeName))
		e.writeLine(fmt.Sprintf("  gwen_value result = gwen_value_list_from_ptr(gwen_dyn_list_new(%dLL));", len(fields)))
		for idx, itemExpr := range itemExprs {
			e.writeLine(fmt.Sprintf("  result.list_value->items[%d] = %s;", idx, itemExpr))
		}
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitTupleForwards() error {
	for _, key := range e.tupleOrder {
		typeName := e.tupleByKey[key]
		e.writeLine(fmt.Sprintf("typedef struct %s %s;", typeName, typeName))
	}
	if len(e.tupleOrder) > 0 {
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitFuncForwards() error {
	for _, key := range e.funcTypeOrder {
		name := e.funcTypeByKey[key]
		e.writeLine(fmt.Sprintf("typedef struct %s %s;", name, name))
	}
	if len(e.funcTypeOrder) > 0 {
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitAggregateForwards() error {
	emitted := 0
	for _, key := range e.listOrder {
		typeName := e.listByKey[key]
		e.writeLine(fmt.Sprintf("typedef struct %s %s;", typeName, typeName))
		emitted++
	}
	for _, key := range e.dictOrder {
		typeName := e.dictByKey[key]
		e.writeLine(fmt.Sprintf("typedef struct %s %s;", typeName, typeName))
		emitted++
	}
	for _, key := range e.resultOrder {
		typeName := e.resultByKey[key]
		e.writeLine(fmt.Sprintf("typedef struct %s %s;", typeName, typeName))
		emitted++
	}
	for _, key := range e.cellOrder {
		typeName := e.cellByKey[key]
		e.writeLine(fmt.Sprintf("typedef struct %s %s;", typeName, typeName))
		emitted++
	}
	if emitted > 0 {
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitFuncTypes() error {
	for _, key := range e.funcTypeOrder {
		name := e.funcTypeByKey[key]
		returnType := e.funcReturns[key]
		callParams := append([]string{"void *"}, e.funcParams[key]...)
		e.writeLine(fmt.Sprintf("struct %s {", name))
		e.writeLine("  void *env;")
		e.writeLine(fmt.Sprintf("  %s (*call)(%s);", returnType, strings.Join(callParams, ", ")))
		e.writeLine("};")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitFuncHelpers() error {
	for _, key := range e.funcTypeOrder {
		name := e.funcTypeByKey[key]
		returnType := e.funcReturns[key]
		params := append([]string(nil), e.funcParams[key]...)
		helperParams := []string{fmt.Sprintf("%s fn", name)}
		callArgs := []string{"fn.env"}
		for idx, param := range params {
			argName := fmt.Sprintf("arg_%d", idx+1)
			helperParams = append(helperParams, fmt.Sprintf("%s %s", param, argName))
			callArgs = append(callArgs, argName)
		}
		e.writeLine(fmt.Sprintf("static %s %s(%s) {", returnType, funcCallHelperName(name), strings.Join(helperParams, ", ")))
		e.writeLine("  if (fn.call == NULL) gwen_runtime_error(\"runtime error: call to null function\");")
		callExpr := fmt.Sprintf("fn.call(%s)", strings.Join(callArgs, ", "))
		if returnType == "void" {
			e.writeLine("  " + callExpr + ";")
		} else {
			e.writeLine("  return " + callExpr + ";")
		}
		e.writeLine("}")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitParallelHelpers() error {
	if len(e.parallelBranches) == 0 {
		return nil
	}
	e.writeLine("typedef struct {")
	e.writeLine("  bool ok;")
	e.writeLine("  gwen_value value;")
	e.writeLine("  const char *error;")
	e.writeLine("} gwen_parallel_task_result;")
	e.writeLine("")
	e.writeLine("static gwen_parallel_task_result gwen_parallel_task_success(gwen_value value) {")
	e.writeLine("  gwen_parallel_task_result result;")
	e.writeLine("  result.ok = true;")
	e.writeLine("  result.value = value;")
	e.writeLine("  result.error = NULL;")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_parallel_task_result gwen_parallel_task_failure(const char *error) {")
	e.writeLine("  gwen_parallel_task_result result;")
	e.writeLine("  result.ok = false;")
	e.writeLine("  result.value = gwen_value_null();")
	e.writeLine("  result.error = error != NULL ? error : \"runtime error\";")
	e.writeLine("  return result;")
	e.writeLine("}")
	e.writeLine("")
	for _, branch := range e.parallelBranches {
		if branch == nil {
			continue
		}
		e.writeLine("typedef struct {")
		for _, slot := range branch.captureSlots {
			if slot == nil {
				continue
			}
			typeName, err := e.cType(slot.Type)
			if err != nil {
				return fmt.Errorf("parallel capture %q: %w", slot.Name, err)
			}
			e.writeLine(fmt.Sprintf("  %s %s;", typeName, slotName(slot.ID)))
		}
		e.writeLine("  gwen_parallel_task_result result;")
		e.writeLine(fmt.Sprintf("} %s;", branch.ctxName))
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static void *%s(void *data);", branch.entryName))
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitGlobals() error {
	emitted := 0
	for _, bindingID := range e.globalOrder {
		slot := e.globalSlots[bindingID]
		if slot == nil || slot.Type == nil {
			continue
		}
		typeName, err := e.cType(slot.Type)
		if err != nil {
			return fmt.Errorf("global %q: %w", slot.Name, err)
		}
		e.writeLine(fmt.Sprintf("static %s %s;", typeName, e.globalName(bindingID)))
		e.writeLine(fmt.Sprintf("static bool %s = false;", globalInitName(bindingID)))
		emitted++
	}
	if emitted > 0 {
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitClosureHelpers() error {
	for _, fn := range e.funcs {
		if err := e.emitFuncClosureHelper(fn); err != nil {
			return err
		}
	}
	for _, obj := range e.objects {
		if err := e.emitObjectClosureHelpers(obj); err != nil {
			return err
		}
	}
	if err := e.emitBuiltinClosureHelpers(); err != nil {
		return err
	}
	return nil
}

func (e *emitter) emitBuiltinClosureHelpers() error {
	for _, key := range e.builtinClosureOrder {
		spec := e.builtinClosures[key]
		if spec == nil {
			continue
		}
		if err := e.emitBuiltinClosureHelper(spec); err != nil {
			label := spec.Name
			if spec.ModuleName != "" {
				label = spec.ModuleName + "." + spec.Name
			}
			return fmt.Errorf("builtin closure %q: %w", label, err)
		}
	}
	return nil
}

func (e *emitter) emitBuiltinClosureHelper(spec *builtinClosureSpec) error {
	if spec == nil || spec.FuncType == nil {
		return fmt.Errorf("missing builtin closure spec")
	}
	returnType, err := e.signatureReturnType(spec.FuncType.Returns)
	if err != nil {
		return err
	}
	params := []string{"void *env"}
	argNames := make([]string, 0, len(spec.FuncType.Params))
	for idx, paramType := range spec.FuncType.Params {
		typeName, err := e.cType(paramType)
		if err != nil {
			return err
		}
		argName := fmt.Sprintf("arg_%d", idx+1)
		params = append(params, fmt.Sprintf("%s %s", typeName, argName))
		argNames = append(argNames, argName)
	}
	callExpr, isVoid, err := e.builtinClosureExpr(spec.ModuleName, spec.Name, spec.FuncType, argNames)
	if err != nil {
		return err
	}
	e.writeLine(fmt.Sprintf("static %s %s(%s) {", returnType, spec.Wrapper, strings.Join(params, ", ")))
	e.writeLine("  (void)env;")
	if isVoid {
		e.writeLine("  " + callExpr + ";")
	} else {
		e.writeLine("  return " + callExpr + ";")
	}
	e.writeLine("}")
	e.writeLine("")
	return nil
}

func (e *emitter) emitFuncClosureHelper(fn *mir.Func) error {
	if fn == nil {
		return fmt.Errorf("nil function")
	}
	actualName, err := e.funcCName(fn)
	if err != nil {
		return err
	}
	captures := functionCaptureSlots(e, fn.Body)
	if len(captures) == 0 {
		return e.emitDirectClosureAdapter(actualName, fn.Params, fn.Returns)
	}
	funcType, err := e.funcTypeName(signatureType(fn.Params, fn.Returns))
	if err != nil {
		return err
	}
	refCaptures := refCaptureBindingIDSet(fn.Body)
	envType := closureEnvTypeName(actualName)
	e.writeLine("typedef struct {")
	for _, slot := range captures {
		if slot == nil {
			continue
		}
		fieldType, err := e.captureParamType(slot.Type, hasCaptureBinding(refCaptures, slot.BindingID))
		if err != nil {
			return err
		}
		e.writeLine(fmt.Sprintf("  %s %s;", fieldType, closureCaptureFieldName(slot.ID)))
	}
	e.writeLine(fmt.Sprintf("} %s;", envType))
	e.writeLine("")
	returnType, err := e.signatureReturnType(fn.Returns)
	if err != nil {
		return err
	}
	adapterParams := []string{"void *env"}
	callArgs := make([]string, 0, len(captures)+len(fn.Params))
	for _, slot := range captures {
		if slot != nil {
			callArgs = append(callArgs, fmt.Sprintf("captures->%s", closureCaptureFieldName(slot.ID)))
		}
	}
	for idx, param := range fn.Params {
		typeName, err := e.cType(param.Type)
		if err != nil {
			return err
		}
		argName := fmt.Sprintf("arg_%d", idx+1)
		adapterParams = append(adapterParams, fmt.Sprintf("%s %s", typeName, argName))
		callArgs = append(callArgs, argName)
	}
	e.writeLine(fmt.Sprintf("static %s %s(%s) {", returnType, closureAdapterName(actualName), strings.Join(adapterParams, ", ")))
	e.writeLine(fmt.Sprintf("  %s *captures = (%s *)env;", envType, envType))
	callExpr := fmt.Sprintf("%s(%s)", actualName, strings.Join(callArgs, ", "))
	if returnType == "void" {
		e.writeLine("  " + callExpr + ";")
	} else {
		e.writeLine("  return " + callExpr + ";")
	}
	e.writeLine("}")
	e.writeLine("")
	constructorParams := make([]string, 0, len(captures))
	for _, slot := range captures {
		if slot == nil {
			continue
		}
		typeName, err := e.captureParamType(slot.Type, hasCaptureBinding(refCaptures, slot.BindingID))
		if err != nil {
			return err
		}
		constructorParams = append(constructorParams, fmt.Sprintf("%s %s", typeName, slotName(slot.ID)))
	}
	if len(constructorParams) == 0 {
		constructorParams = append(constructorParams, "void")
	}
	e.writeLine(fmt.Sprintf("static %s %s(%s) {", funcType, closureConstructorName(actualName), strings.Join(constructorParams, ", ")))
	e.writeLine(fmt.Sprintf("  %s *env = (%s *)malloc(sizeof(%s));", envType, envType, envType))
	e.writeLine(fmt.Sprintf("  if (env == NULL) gwen_runtime_error(\"runtime error: out of memory creating closure %s\");", actualName))
	for _, slot := range captures {
		if slot != nil {
			e.writeLine(fmt.Sprintf("  env->%s = %s;", closureCaptureFieldName(slot.ID), slotName(slot.ID)))
		}
	}
	e.writeLine(fmt.Sprintf("  return (%s){env, %s};", funcType, closureAdapterName(actualName)))
	e.writeLine("}")
	e.writeLine("")
	return nil
}

func (e *emitter) emitObjectClosureHelpers(obj *mir.Object) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	info := e.objectInfo[obj.Name]
	if info == nil {
		return fmt.Errorf("missing object info for %q", obj.Name)
	}
	if obj.Constructor != nil {
		if err := e.emitDirectClosureAdapter(info.constructorName, obj.Constructor.Params, obj.Constructor.Returns); err != nil {
			return err
		}
	}
	for _, method := range obj.Methods {
		name, ok := info.methodNames[method.Name]
		if !ok {
			return fmt.Errorf("missing method C name for %q.%q", obj.Name, method.Name)
		}
		if err := e.emitDirectClosureAdapter(name, method.Params, method.Returns); err != nil {
			return err
		}
		if _, ok := e.boundMethodClosures[name]; ok {
			if err := e.emitBoundMethodClosureHelper(info, method); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *emitter) emitBoundMethodClosureHelper(info *objectInfo, method *mir.Method) error {
	if info == nil {
		return fmt.Errorf("missing object info")
	}
	if method == nil {
		return fmt.Errorf("nil object method")
	}
	if len(method.Params) == 0 {
		return fmt.Errorf("object method %q.%q is missing receiver parameter", info.name, method.Name)
	}
	actualName, ok := info.methodNames[method.Name]
	if !ok {
		return fmt.Errorf("missing method C name for %q.%q", info.name, method.Name)
	}
	funcType, err := e.funcTypeName(signatureType(method.Params[1:], method.Returns))
	if err != nil {
		return err
	}
	receiverType, err := e.cType(method.Params[0].Type)
	if err != nil {
		return err
	}
	returnType, err := e.signatureReturnType(method.Returns)
	if err != nil {
		return err
	}
	envType := boundMethodClosureEnvTypeName(actualName)
	e.writeLine("typedef struct {")
	e.writeLine(fmt.Sprintf("  %s receiver;", receiverType))
	e.writeLine(fmt.Sprintf("} %s;", envType))
	e.writeLine("")
	adapterParams := []string{"void *env"}
	callArgs := []string{"captures->receiver"}
	for idx, param := range method.Params[1:] {
		typeName, err := e.cType(param.Type)
		if err != nil {
			return err
		}
		argName := fmt.Sprintf("arg_%d", idx+1)
		adapterParams = append(adapterParams, fmt.Sprintf("%s %s", typeName, argName))
		callArgs = append(callArgs, argName)
	}
	e.writeLine(fmt.Sprintf("static %s %s(%s) {", returnType, boundMethodClosureAdapterName(actualName), strings.Join(adapterParams, ", ")))
	e.writeLine(fmt.Sprintf("  %s *captures = (%s *)env;", envType, envType))
	callExpr := fmt.Sprintf("%s(%s)", actualName, strings.Join(callArgs, ", "))
	if returnType == "void" {
		e.writeLine("  " + callExpr + ";")
	} else {
		e.writeLine("  return " + callExpr + ";")
	}
	e.writeLine("}")
	e.writeLine("")
	e.writeLine(fmt.Sprintf("static %s %s(%s receiver) {", funcType, boundMethodClosureConstructorName(actualName), receiverType))
	e.writeLine(fmt.Sprintf("  %s *env = (%s *)malloc(sizeof(%s));", envType, envType, envType))
	e.writeLine(fmt.Sprintf("  if (env == NULL) gwen_runtime_error(\"runtime error: out of memory creating bound method %s\");", actualName))
	e.writeLine("  env->receiver = receiver;")
	e.writeLine(fmt.Sprintf("  return (%s){env, %s};", funcType, boundMethodClosureAdapterName(actualName)))
	e.writeLine("}")
	e.writeLine("")
	return nil
}

func (e *emitter) emitDirectClosureAdapter(actualName string, params []*hir.Param, returns []hir.Type) error {
	returnType, err := e.signatureReturnType(returns)
	if err != nil {
		return err
	}
	adapterParams := []string{"void *env"}
	callArgs := make([]string, 0, len(params))
	for idx, param := range params {
		typeName, err := e.cType(param.Type)
		if err != nil {
			return err
		}
		argName := fmt.Sprintf("arg_%d", idx+1)
		adapterParams = append(adapterParams, fmt.Sprintf("%s %s", typeName, argName))
		callArgs = append(callArgs, argName)
	}
	e.writeLine(fmt.Sprintf("static %s %s(%s) {", returnType, closureAdapterName(actualName), strings.Join(adapterParams, ", ")))
	e.writeLine("  (void)env;")
	callExpr := fmt.Sprintf("%s(%s)", actualName, strings.Join(callArgs, ", "))
	if returnType == "void" {
		e.writeLine("  " + callExpr + ";")
	} else {
		e.writeLine("  return " + callExpr + ";")
	}
	e.writeLine("}")
	e.writeLine("")
	return nil
}

func (e *emitter) ensureBuiltinClosure(moduleName, name string, funcType *hir.FuncType) (string, error) {
	if funcType == nil {
		return "", fmt.Errorf("missing builtin function type")
	}
	if _, err := e.funcTypeName(funcType); err != nil {
		return "", err
	}
	key := builtinClosureKey(moduleName, name, funcType)
	if wrapper, ok := e.builtinClosureByKey[key]; ok {
		return wrapper, nil
	}
	wrapper := fmt.Sprintf("gwen_builtin_closure_%d", len(e.builtinClosureOrder)+1)
	e.builtinClosureByKey[key] = wrapper
	e.builtinClosureOrder = append(e.builtinClosureOrder, key)
	e.builtinClosures[key] = &builtinClosureSpec{
		ModuleName: moduleName,
		Name:       name,
		Wrapper:    wrapper,
		FuncType:   cloneFuncTypeNode(funcType),
	}
	return wrapper, nil
}

func (e *emitter) emitMoneyHelpers() error {
	e.writeLine("typedef struct {")
	e.writeLine("  long long raw;")
	e.writeLine("  const char *currency;")
	e.writeLine("} gwen_money;")
	e.writeLine("")
	e.writeLine("static const char *gwen_money_currency(gwen_money value) {")
	e.writeLine("  return value.currency != NULL ? value.currency : \"\";")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_make(long long raw, const char *currency) {")
	e.writeLine("  gwen_money value;")
	e.writeLine("  value.raw = raw;")
	e.writeLine("  value.currency = currency != NULL ? currency : \"\";")
	e.writeLine("  return value;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_zero(const char *currency) {")
	e.writeLine("  return gwen_money_make(0LL, currency);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_from_int(long long value, const char *currency) {")
	e.writeLine("  return gwen_money_make(value * 10000LL, currency);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_from_float(double value, const char *currency) {")
	e.writeLine("  return gwen_money_make((long long)llround(value * 10000.0), currency);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_money_require_same_currency(gwen_money left, gwen_money right, const char *op) {")
	e.writeLine("  if (strcmp(gwen_money_currency(left), gwen_money_currency(right)) == 0) return;")
	e.writeLine("  char message[256];")
	e.writeLine("  snprintf(message, sizeof(message), \"runtime error: Currency mismatch: money[%s] %s money[%s]\", gwen_money_currency(left), op != NULL ? op : \"?\", gwen_money_currency(right));")
	e.writeLine("  gwen_runtime_error(message);")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_add(gwen_money left, gwen_money right) {")
	e.writeLine("  gwen_money_require_same_currency(left, right, \"+\");")
	e.writeLine("  return gwen_money_make(left.raw + right.raw, gwen_money_currency(left));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_sub(gwen_money left, gwen_money right) {")
	e.writeLine("  gwen_money_require_same_currency(left, right, \"-\");")
	e.writeLine("  return gwen_money_make(left.raw - right.raw, gwen_money_currency(left));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_mul_int(gwen_money value, long long scalar) {")
	e.writeLine("  return gwen_money_make(value.raw * scalar, gwen_money_currency(value));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_mul_float(gwen_money value, double scalar) {")
	e.writeLine("  return gwen_money_make((long long)llround((double)value.raw * scalar), gwen_money_currency(value));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_div_int(gwen_money value, long long scalar) {")
	e.writeLine("  if (scalar == 0LL) gwen_runtime_error(\"runtime error: Division by zero\");")
	e.writeLine("  return gwen_money_make((long long)llround((double)value.raw / (double)scalar), gwen_money_currency(value));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static gwen_money gwen_money_div_float(gwen_money value, double scalar) {")
	e.writeLine("  if (scalar == 0.0) gwen_runtime_error(\"runtime error: Division by zero\");")
	e.writeLine("  return gwen_money_make((long long)llround((double)value.raw / scalar), gwen_money_currency(value));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static double gwen_money_ratio(gwen_money left, gwen_money right) {")
	e.writeLine("  gwen_money_require_same_currency(left, right, \"/\");")
	e.writeLine("  if (right.raw == 0LL) gwen_runtime_error(\"runtime error: Division by zero\");")
	e.writeLine("  return (double)left.raw / (double)right.raw;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static int gwen_money_cmp(gwen_money left, gwen_money right) {")
	e.writeLine("  gwen_money_require_same_currency(left, right, \"cmp\");")
	e.writeLine("  if (left.raw < right.raw) return -1;")
	e.writeLine("  if (left.raw > right.raw) return 1;")
	e.writeLine("  return 0;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static long long gwen_money_to_int(gwen_money value) {")
	e.writeLine("  return value.raw / 10000LL;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static double gwen_money_to_float(gwen_money value) {")
	e.writeLine("  return (double)value.raw / 10000.0;")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_money_to_string(gwen_money value) {")
	e.writeLine("  unsigned long long abs_raw = value.raw < 0 ? (unsigned long long)(-(value.raw + 1LL)) + 1ULL : (unsigned long long)value.raw;")
	e.writeLine("  unsigned long long whole = abs_raw / 10000ULL;")
	e.writeLine("  unsigned long long frac = abs_raw % 10000ULL;")
	e.writeLine("  char frac_buf[5];")
	e.writeLine("  char num_buf[128];")
	e.writeLine("  int frac_len = 4;")
	e.writeLine("  snprintf(frac_buf, sizeof(frac_buf), \"%04llu\", frac);")
	e.writeLine("  while (frac_len > 0 && frac_buf[frac_len-1] == '0') frac_len--;")
	e.writeLine("  if (frac_len == 0) frac_len = 2;")
	e.writeLine("  else if (frac_len == 1) frac_len = 2;")
	e.writeLine("  frac_buf[frac_len] = '\\0';")
	e.writeLine("  if (value.raw < 0) snprintf(num_buf, sizeof(num_buf), \"-%llu.%s\", whole, frac_buf);")
	e.writeLine("  else snprintf(num_buf, sizeof(num_buf), \"%llu.%s\", whole, frac_buf);")
	e.writeLine("  const char *with_space = gwen_string_concat(num_buf, \" \");")
	e.writeLine("  return gwen_string_concat(with_space, gwen_money_currency(value));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static void gwen_write_money(gwen_money value) {")
	e.writeLine("  gwen_write_string(gwen_money_to_string(value));")
	e.writeLine("}")
	e.writeLine("")
	e.writeLine("static const char *gwen_value_typeof(gwen_value value) {")
	e.writeLine("  switch (value.kind) {")
	e.writeLine("    case GWEN_VALUE_NULL: return \"JsonNull\";")
	e.writeLine("    case GWEN_VALUE_INT: return \"int\";")
	e.writeLine("    case GWEN_VALUE_FLOAT: return \"float\";")
	e.writeLine("    case GWEN_VALUE_BOOL: return \"bool\";")
	e.writeLine("    case GWEN_VALUE_STRING: return \"string\";")
	e.writeLine("    case GWEN_VALUE_LIST: return \"list\";")
	e.writeLine("    case GWEN_VALUE_DICT: return \"dict\";")
	e.writeLine("    case GWEN_VALUE_RESULT: return value.result_value != NULL && value.result_value->is_ok ? \"ok\" : \"err\";")
	e.writeLine("    default: return \"dynamic\";")
	e.writeLine("  }")
	e.writeLine("}")
	e.writeLine("")
	return nil
}

func (e *emitter) emitObjectForwards() error {
	if len(e.objects) == 0 {
		return nil
	}
	for _, obj := range e.objects {
		info := e.objectInfo[obj.Name]
		if info == nil {
			return fmt.Errorf("missing object info for %q", obj.Name)
		}
		e.writeLine(fmt.Sprintf("typedef struct %s %s;", info.typeName, info.typeName))
		e.writeLine(fmt.Sprintf("static %s *%s(%s *value);", info.typeName, info.cloneName, info.typeName))
	}
	e.writeLine("")
	return nil
}

func (e *emitter) emitObjectTypes() error {
	if len(e.objects) == 0 {
		return nil
	}
	for _, obj := range e.objects {
		info := e.objectInfo[obj.Name]
		if info == nil {
			return fmt.Errorf("missing object info for %q", obj.Name)
		}
		e.writeLine(fmt.Sprintf("struct %s {", info.typeName))
		for _, field := range obj.Fields {
			if field == nil {
				continue
			}
			typeName, err := e.cType(field.Type)
			if err != nil {
				return fmt.Errorf("object %q field %q: %w", obj.Name, field.Name, err)
			}
			e.writeLine(fmt.Sprintf("  %s %s;", typeName, objectFieldName(field.Name)))
		}
		e.writeLine("};")
		e.writeLine("")
		e.writeLine(fmt.Sprintf("static %s *%s(%s *value) {", info.typeName, info.cloneName, info.typeName))
		e.writeLine("  if (value == NULL) return NULL;")
		e.writeLine(fmt.Sprintf("  %s *result = (%s *)calloc(1U, sizeof(%s));", info.typeName, info.typeName, info.typeName))
		e.writeLine(fmt.Sprintf("  if (result == NULL) gwen_runtime_error(\"runtime error: out of memory cloning %s\");", obj.Name))
		for _, field := range obj.Fields {
			if field == nil {
				continue
			}
			clonedExpr, err := e.cloneExpr("value->"+objectFieldName(field.Name), field.Type)
			if err != nil {
				return fmt.Errorf("object %q field %q clone: %w", obj.Name, field.Name, err)
			}
			e.writeLine(fmt.Sprintf("  result->%s = %s;", objectFieldName(field.Name), clonedExpr))
		}
		e.writeLine("  return result;")
		e.writeLine("}")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitPrototypes() error {
	for _, fn := range e.funcs {
		signature, err := e.funcSignature(fn)
		if err != nil {
			return err
		}
		e.writeLine(signature + ";")
	}
	for _, obj := range e.objects {
		info := e.objectInfo[obj.Name]
		if info == nil {
			return fmt.Errorf("missing object info for %q", obj.Name)
		}
		if obj.Constructor != nil {
			signature, err := e.constructorSignature(info, obj.Constructor)
			if err != nil {
				return err
			}
			e.writeLine(signature + ";")
		}
		for _, method := range obj.Methods {
			signature, err := e.methodSignature(info, method)
			if err != nil {
				return err
			}
			e.writeLine(signature + ";")
		}
	}
	for _, script := range e.scripts {
		e.writeLine(fmt.Sprintf("static void %s(void);", e.scriptNames[script]))
	}
	for _, branch := range e.parallelBranches {
		signature, err := e.parallelBranchSignature(branch)
		if err != nil {
			return err
		}
		e.writeLine(signature + ";")
	}
	if len(e.funcs) > 0 || len(e.objects) > 0 || len(e.scripts) > 0 || len(e.parallelBranches) > 0 {
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitFunc(fn *mir.Func) error {
	signature, err := e.funcSignature(fn)
	if err != nil {
		return err
	}
	e.writeLine(signature + " {")
	if err := e.emitBodyWithParamSlots(fn.Body, fn.Returns, "  ", captureSlotIDSet(e, fn.Body), refCaptureSlotIDSet(e, fn.Body), false, nil); err != nil {
		return fmt.Errorf("function %q: %w", fn.Name, err)
	}
	e.writeLine("}")
	e.writeLine("")
	return nil
}

func (e *emitter) emitObject(obj *mir.Object) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	info := e.objectInfo[obj.Name]
	if info == nil {
		return fmt.Errorf("missing object info for %q", obj.Name)
	}
	if obj.Constructor != nil {
		signature, err := e.constructorSignature(info, obj.Constructor)
		if err != nil {
			return err
		}
		e.writeLine(signature + " {")
		if err := e.emitBody(obj.Constructor.Body, obj.Constructor.Returns, "  "); err != nil {
			return fmt.Errorf("object %q constructor: %w", obj.Name, err)
		}
		e.writeLine("}")
		e.writeLine("")
	}
	for _, method := range obj.Methods {
		signature, err := e.methodSignature(info, method)
		if err != nil {
			return err
		}
		e.writeLine(signature + " {")
		if err := e.emitBody(method.Body, method.Returns, "  "); err != nil {
			return fmt.Errorf("object %q method %q: %w", obj.Name, method.Name, err)
		}
		e.writeLine("}")
		e.writeLine("")
	}
	return nil
}

func (e *emitter) emitScript(script *mir.Script) error {
	e.writeLine(fmt.Sprintf("static void %s(void) {", e.scriptNames[script]))
	if err := e.emitBody(script.Body, nil, "  "); err != nil {
		return fmt.Errorf("script at line %d: %w", script.Line, err)
	}
	e.writeLine("}")
	e.writeLine("")
	return nil
}

func (e *emitter) parallelBranchParams(info *parallelBranchInfo) ([]string, error) {
	if info == nil {
		return nil, fmt.Errorf("nil parallel branch info")
	}
	params := make([]string, 0, len(info.captureSlots))
	for _, slot := range info.captureSlots {
		if slot == nil {
			continue
		}
		typeName, err := e.cType(slot.Type)
		if err != nil {
			return nil, fmt.Errorf("parallel capture %q: %w", slot.Name, err)
		}
		params = append(params, fmt.Sprintf("%s %s", typeName, slotName(slot.ID)))
	}
	if len(params) == 0 {
		params = append(params, "void")
	}
	return params, nil
}

func (e *emitter) parallelBranchSignature(info *parallelBranchInfo) (string, error) {
	params, err := e.parallelBranchParams(info)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("static gwen_parallel_task_result %s(%s)", info.name, strings.Join(params, ", ")), nil
}

func (e *emitter) parallelBranchExecSignature(info *parallelBranchInfo) (string, error) {
	params, err := e.parallelBranchParams(info)
	if err != nil {
		return "", err
	}
	returnType := "void"
	if info != nil && info.returnsResult {
		returnType = "gwen_value"
	}
	return fmt.Sprintf("static %s %s(%s)", returnType, info.execName, strings.Join(params, ", ")), nil
}

func (e *emitter) emitParallelBranch(info *parallelBranchInfo) error {
	signature, err := e.parallelBranchSignature(info)
	if err != nil {
		return err
	}
	execSignature, err := e.parallelBranchExecSignature(info)
	if err != nil {
		return err
	}
	paramSlotIDs := map[int]struct{}{}
	callArgs := make([]string, 0, len(info.captureSlots))
	for _, slot := range info.captureSlots {
		if slot != nil {
			paramSlotIDs[slot.ID] = struct{}{}
			callArgs = append(callArgs, slotName(slot.ID))
		}
	}
	e.writeLine(execSignature + " {")
	if err := e.emitBodyWithParamSlots(info.body, nil, "  ", paramSlotIDs, nil, info.returnsResult, info.exprResultValueIDs); err != nil {
		return fmt.Errorf("parallel branch %q: %w", info.name, err)
	}
	e.writeLine("}")
	e.writeLine("")
	e.writeLine(signature + " {")
	e.writeLine("  gwen_error_frame frame;")
	e.writeLine("  frame.prev = gwen_error_frame_current;")
	e.writeLine("  frame.message = NULL;")
	e.writeLine("  gwen_error_frame_current = &frame;")
	e.writeLine("  if (setjmp(frame.jump) != 0) {")
	e.writeLine("    gwen_error_frame_current = frame.prev;")
	e.writeLine("    return gwen_parallel_task_failure(frame.message);")
	e.writeLine("  }")
	if info.returnsResult {
		e.writeLine(fmt.Sprintf("  gwen_value value = %s(%s);", info.execName, strings.Join(callArgs, ", ")))
		e.writeLine("  gwen_error_frame_current = frame.prev;")
		e.writeLine("  return gwen_parallel_task_success(value);")
	} else {
		e.writeLine(fmt.Sprintf("  %s(%s);", info.execName, strings.Join(callArgs, ", ")))
		e.writeLine("  gwen_error_frame_current = frame.prev;")
		e.writeLine("  return gwen_parallel_task_success(gwen_value_null());")
	}
	e.writeLine("}")
	e.writeLine("")
	e.writeLine(fmt.Sprintf("static void *%s(void *data) {", info.entryName))
	e.writeLine(fmt.Sprintf("  %s *ctx = (%s *)data;", info.ctxName, info.ctxName))
	e.writeLine("  if (ctx == NULL) return NULL;")
	if len(callArgs) == 0 {
		e.writeLine(fmt.Sprintf("  ctx->result = %s();", info.name))
	} else {
		threadArgs := make([]string, 0, len(info.captureSlots))
		for _, slot := range info.captureSlots {
			if slot != nil {
				threadArgs = append(threadArgs, "ctx->"+slotName(slot.ID))
			}
		}
		e.writeLine(fmt.Sprintf("  ctx->result = %s(%s);", info.name, strings.Join(threadArgs, ", ")))
	}
	e.writeLine("  return NULL;")
	e.writeLine("}")
	e.writeLine("")
	return nil
}

func (e *emitter) emitEntryPoint() error {
	e.writeLine("int main(int argc, char **argv) {")
	e.writeLine("  gwen_cli_argc = argc;")
	e.writeLine("  gwen_cli_argv = argv;")
	for _, item := range e.program.Items {
		if script, ok := item.(*mir.Script); ok {
			e.writeLine(fmt.Sprintf("  %s();", e.scriptNames[script]))
		}
	}
	if e.userMain != nil {
		mainName, err := e.funcCName(e.userMain)
		if err != nil {
			return err
		}
		call := fmt.Sprintf("%s()", mainName)
		if len(e.userMain.Returns) == 0 {
			e.writeLine("  " + call + ";")
		} else {
			e.writeLine("  (void)" + call + ";")
		}
	}
	e.writeLine("  return 0;")
	e.writeLine("}")
	return nil
}

func (e *emitter) emitBody(body *mir.Body, returns []hir.Type, indent string) error {
	return e.emitBodyWithParamSlots(body, returns, indent, nil, nil, false, nil)
}

func (e *emitter) emitBodyWithParamSlots(body *mir.Body, returns []hir.Type, indent string, paramSlotIDs map[int]struct{}, refSlotIDs map[int]struct{}, parallelReturnsDynamic bool, parallelResultValueIDs []int) error {
	state := &bodyState{
		emitter:                e,
		body:                   body,
		indent:                 indent,
		tempIDs:                map[int]struct{}{},
		rangeLoops:             map[int]*mir.ForRangeTerm{},
		forEachLoops:           map[int]*mir.ForEachTerm{},
		paramSlotIDs:           paramSlotIDs,
		refSlotIDs:             refSlotIDs,
		parallelReturnsDynamic: parallelReturnsDynamic,
		parallelResultValueIDs: append([]int(nil), parallelResultValueIDs...),
	}
	if err := state.collectTemps(); err != nil {
		return err
	}
	slotDecls := 0
	for _, slot := range body.Slots {
		if e.isGlobalBinding(slot.BindingID) && !state.isParameterSlot(slot.ID) {
			continue
		}
		if slot.Kind == mir.SlotParam || state.isParameterSlot(slot.ID) {
			e.writeLine(indent + fmt.Sprintf("bool %s = true;", slotInitName(slot.ID)))
			slotDecls++
			continue
		}
		typeName, err := e.cType(slot.Type)
		if err != nil {
			return fmt.Errorf("slot %q: %w", slot.Name, err)
		}
		zeroExpr, err := e.zeroExpr(slot.Type)
		if err != nil {
			return fmt.Errorf("slot %q: %w", slot.Name, err)
		}
		e.writeLine(indent + fmt.Sprintf("%s %s = %s;", typeName, slotName(slot.ID), zeroExpr))
		e.writeLine(indent + fmt.Sprintf("bool %s = false;", slotInitName(slot.ID)))
		slotDecls += 2
	}
	if slotDecls > 0 {
		e.writeLine("")
	}
	for _, blockID := range state.sortedRangeLoopIDs() {
		e.writeLine(indent + "bool " + loopStartedName(blockID) + " = false;")
		e.writeLine(indent + "long long " + loopCurrentName(blockID) + " = 0;")
		e.writeLine(indent + "long long " + loopEndName(blockID) + " = 0;")
		e.writeLine(indent + "long long " + loopStepName(blockID) + " = 0;")
	}
	for _, blockID := range state.sortedForEachLoopIDs() {
		e.writeLine(indent + "bool " + loopStartedName(blockID) + " = false;")
		e.writeLine(indent + "long long " + loopIndexName(blockID) + " = 0;")
	}
	if len(state.rangeLoops) > 0 || len(state.forEachLoops) > 0 {
		e.writeLine("")
	}
	for _, valueID := range state.sortedTempIDs() {
		value := body.Value(valueID)
		if value == nil {
			return fmt.Errorf("unknown MIR value %d", valueID)
		}
		typeName, err := e.cTypeForValue(value)
		if err != nil {
			return fmt.Errorf("value %d: %w", valueID, err)
		}
		e.writeLine(indent + typeName + " " + tempName(valueID) + ";")
	}
	if len(state.tempIDs) > 0 {
		e.writeLine("")
	}
	e.writeLine(indent + fmt.Sprintf("goto block_%d;", body.Entry))
	for _, block := range body.Blocks {
		e.writeLine("")
		e.writeLine(indent + fmt.Sprintf("block_%d:", block.ID))
		for _, inst := range block.Insts {
			if err := state.emitInst(inst); err != nil {
				return fmt.Errorf("block %d: %w", block.ID, err)
			}
		}
		if err := state.emitTerm(block.ID, block.Term, returns); err != nil {
			return fmt.Errorf("block %d: %w", block.ID, err)
		}
	}
	return nil
}

type bodyState struct {
	emitter                *emitter
	body                   *mir.Body
	indent                 string
	tempIDs                map[int]struct{}
	rangeLoops             map[int]*mir.ForRangeTerm
	forEachLoops           map[int]*mir.ForEachTerm
	paramSlotIDs           map[int]struct{}
	refSlotIDs             map[int]struct{}
	parallelReturnsDynamic bool
	parallelResultValueIDs []int
	parallelSerial         int
}

type matchClause struct {
	Cond  string
	Binds []string
}

func (s *bodyState) isParameterSlot(slotID int) bool {
	if s == nil || s.paramSlotIDs == nil {
		return false
	}
	_, ok := s.paramSlotIDs[slotID]
	return ok
}

func (s *bodyState) isReferenceSlot(slotID int) bool {
	if s == nil || s.refSlotIDs == nil {
		return false
	}
	_, ok := s.refSlotIDs[slotID]
	return ok
}

func (s *bodyState) slotExpr(slotID int, binding *hir.NameBinding) string {
	if s.isReferenceSlot(slotID) {
		return "(*" + slotName(slotID) + ")"
	}
	if s.isParameterSlot(slotID) {
		return slotName(slotID)
	}
	if binding != nil && s.emitter.isGlobalBinding(binding.ID) {
		return s.emitter.globalName(binding.ID)
	}
	return slotName(slotID)
}

func (s *bodyState) slotInitExpr(slotID int, binding *hir.NameBinding) string {
	if s.isParameterSlot(slotID) {
		return slotInitName(slotID)
	}
	if binding != nil && s.emitter.isGlobalBinding(binding.ID) {
		return globalInitName(binding.ID)
	}
	return slotInitName(slotID)
}

func (s *bodyState) slotDisplayName(slotID int, binding *hir.NameBinding) string {
	if binding != nil && binding.Name != "" {
		return binding.Name
	}
	if slot := s.slotInfo(slotID); slot != nil && slot.Name != "" {
		return slot.Name
	}
	return slotName(slotID)
}

func (s *bodyState) slotInfo(slotID int) *mir.Slot {
	if s.body == nil || slotID <= 0 || slotID > len(s.body.Slots) {
		return nil
	}
	return s.body.Slots[slotID-1]
}

func (s *bodyState) slotAlwaysInitialized(slotID int, binding *hir.NameBinding) bool {
	if s.isParameterSlot(slotID) {
		return true
	}
	if binding != nil && s.emitter.isGlobalBinding(binding.ID) {
		return false
	}
	slot := s.slotInfo(slotID)
	return slot != nil && slot.Kind == mir.SlotParam
}

func (s *bodyState) slotReadExpr(slotID int, binding *hir.NameBinding, typ hir.Type) (string, error) {
	if s.slotAlwaysInitialized(slotID, binding) {
		return s.slotExpr(slotID, binding), nil
	}
	if typ == nil {
		if slot := s.slotInfo(slotID); slot != nil {
			typ = slot.Type
		}
	}
	zeroExpr, err := s.emitter.zeroExpr(typ)
	if err != nil {
		return "", err
	}
	name := s.slotDisplayName(slotID, binding)
	return fmt.Sprintf("(%s ? %s : (gwen_runtime_error(%s), %s))", s.slotInitExpr(slotID, binding), s.slotExpr(slotID, binding), quoteCString(fmt.Sprintf("runtime error: '%s' read before assignment", name)), zeroExpr), nil
}

func (s *bodyState) requireSlotInitialized(slotID int, binding *hir.NameBinding) {
	if s.slotAlwaysInitialized(slotID, binding) {
		return
	}
	name := s.slotDisplayName(slotID, binding)
	s.emitter.writeLine(s.indent + fmt.Sprintf("if (!%s) gwen_runtime_error(%s);", s.slotInitExpr(slotID, binding), quoteCString(fmt.Sprintf("runtime error: '%s' read before assignment", name))))
}

func (s *bodyState) exprForMutableValue(valueID int) (string, error) {
	if valueID == 0 {
		return "", fmt.Errorf("invalid MIR value 0")
	}
	if _, ok := s.tempIDs[valueID]; ok {
		return tempName(valueID), nil
	}
	value := s.body.Value(valueID)
	if value == nil {
		return "", fmt.Errorf("unknown MIR value %d", valueID)
	}
	if value.Kind == mir.ValueSlotRef {
		s.requireSlotInitialized(value.SlotID, value.Binding)
		return s.slotExpr(value.SlotID, value.Binding), nil
	}
	return s.exprForValue(valueID)
}

func (s *bodyState) mutableListTargetExpr(valueID int) (string, error) {
	if valueID == 0 {
		return "", fmt.Errorf("invalid MIR value 0")
	}
	value := s.body.Value(valueID)
	if value == nil {
		return "", fmt.Errorf("unknown MIR value %d", valueID)
	}
	if value.Kind == mir.ValueMember && value.MemberBinding != nil && value.MemberBinding.Kind == hir.MemberBindingObjectField {
		return s.memberExpr(value)
	}
	return s.exprForMutableValue(valueID)
}

func (s *bodyState) collectTemps() error {
	for _, block := range s.body.Blocks {
		for _, inst := range block.Insts {
			switch node := inst.(type) {
			case *mir.ComputeInst:
				value := s.body.Value(node.ValueID)
				if value == nil {
					return fmt.Errorf("unknown MIR value %d", node.ValueID)
				}
				if value.Kind == mir.ValueMember {
					if _, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok {
						continue
					}
				}
				s.tempIDs[node.ValueID] = struct{}{}
			case *mir.CallInst:
				value := s.body.Value(node.ValueID)
				if value == nil {
					return fmt.Errorf("unknown MIR call value %d", node.ValueID)
				}
				if len(value.ReturnTypes) > 0 {
					s.tempIDs[node.ValueID] = struct{}{}
				}
				for _, resultID := range node.ResultIDs {
					s.tempIDs[resultID] = struct{}{}
				}
			}
		}
		if term, ok := block.Term.(*mir.ForRangeTerm); ok {
			s.rangeLoops[block.ID] = term
		}
		if term, ok := block.Term.(*mir.ForEachTerm); ok {
			s.forEachLoops[block.ID] = term
		}
	}
	return nil
}

func (s *bodyState) sortedTempIDs() []int {
	ids := make([]int, 0, len(s.tempIDs))
	for id := range s.tempIDs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func (s *bodyState) sortedRangeLoopIDs() []int {
	ids := make([]int, 0, len(s.rangeLoops))
	for id := range s.rangeLoops {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func (s *bodyState) sortedForEachLoopIDs() []int {
	ids := make([]int, 0, len(s.forEachLoops))
	for id := range s.forEachLoops {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func (s *bodyState) emitInst(inst mir.Inst) error {
	switch node := inst.(type) {
	case *mir.ComputeInst:
		value := s.body.Value(node.ValueID)
		if value == nil {
			return fmt.Errorf("unknown MIR value %d", node.ValueID)
		}
		if value.Kind == mir.ValueCast {
			return s.emitCastValue(node.ValueID, value)
		}
		if value.Kind == mir.ValueMember {
			if _, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok {
				return nil
			}
		}
		if value.Kind == mir.ValueList {
			return s.emitListValue(node.ValueID, value)
		}
		if value.Kind == mir.ValueDict {
			return s.emitDictValue(node.ValueID, value)
		}
		if value.Kind == mir.ValueObjectLiteral {
			return s.emitObjectValue(node.ValueID, value)
		}
		if value.Kind == mir.ValueOk || value.Kind == mir.ValueErr {
			return s.emitResultValue(node.ValueID, value)
		}
		if value.Kind == mir.ValueIndex {
			return s.emitIndexValue(node.ValueID, value)
		}
		expr, err := s.computeExpr(value)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", tempName(node.ValueID), expr))
		return nil

	case *mir.CallInst:
		return s.emitCallInst(node)

	case *mir.StoreInst:
		place := s.body.Place(node.PlaceID)
		if place == nil {
			return fmt.Errorf("unknown MIR place %d", node.PlaceID)
		}
		switch place.Kind {
		case mir.PlaceSlot:
			valueExpr, err := s.exprForExpectedValue(node.ValueID, place.Type)
			if err != nil {
				return err
			}
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", s.slotExpr(place.SlotID, place.Binding), valueExpr))
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s = true;", s.slotInitExpr(place.SlotID, place.Binding)))
			return nil
		case mir.PlaceIndex:
			return s.emitIndexStore(place, node.ValueID)
		case mir.PlaceField:
			return s.emitFieldStore(place, node.ValueID)
		default:
			return fmt.Errorf("unsupported store target kind %q", place.Kind)
		}

	case *mir.DeclareInst:
		place := s.body.Place(node.PlaceID)
		if place == nil {
			return fmt.Errorf("unknown MIR place %d", node.PlaceID)
		}
		if place.Kind != mir.PlaceSlot {
			return fmt.Errorf("unsupported declare target kind %q", place.Kind)
		}
		if node.IsUninit {
			return nil
		}
		if node.ValueID != 0 {
			valueExpr, err := s.exprForExpectedValue(node.ValueID, place.Type)
			if err != nil {
				return err
			}
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", s.slotExpr(place.SlotID, place.Binding), valueExpr))
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s = true;", s.slotInitExpr(place.SlotID, place.Binding)))
		} else {
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s = true;", s.slotInitExpr(place.SlotID, place.Binding)))
		}
		return nil

	case *mir.ParallelInst:
		return s.emitParallelInst(node)

	default:
		return fmt.Errorf("unsupported MIR instruction %T", inst)
	}
}

func (s *bodyState) emitCastValue(valueID int, value *mir.Value) error {
	if value == nil {
		return fmt.Errorf("nil cast value")
	}
	resultType := s.emitter.resolveType(value.Type)
	targetType := resultOKType(resultType)
	errType := resultErrType(resultType)
	if targetType == nil || !isStringType(errType) {
		return fmt.Errorf("unsupported cast result type %s", typeLabel(resultType))
	}
	operandValue := s.body.Value(value.Operand)
	if operandValue == nil {
		return fmt.Errorf("unknown cast operand %d", value.Operand)
	}
	operandExpr, err := s.exprForValue(value.Operand)
	if err != nil {
		return err
	}
	convertedExpr, failCond, failExpr, err := s.castResultExprParts(operandExpr, operandValue.Type, targetType)
	if err != nil {
		return err
	}
	resultCType, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	zeroOK, err := s.emitter.zeroExpr(targetType)
	if err != nil {
		return err
	}
	zeroErr, err := s.emitter.zeroExpr(errType)
	if err != nil {
		return err
	}
	temp := tempName(valueID)
	if failCond == "" {
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s){true, %s, %s};", temp, resultCType, convertedExpr, zeroErr))
		return nil
	}
	if convertedExpr == "" {
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s){false, %s, %s};", temp, resultCType, zeroOK, failExpr))
		return nil
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s) {", failCond))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (%s){false, %s, %s};", temp, resultCType, zeroOK, failExpr))
	s.emitter.writeLine(s.indent + "} else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (%s){true, %s, %s};", temp, resultCType, convertedExpr, zeroErr))
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) castResultExprParts(operandExpr string, actualType hir.Type, targetType hir.Type) (string, string, string, error) {
	actualResolved := s.emitter.resolveType(actualType)
	targetResolved := s.emitter.resolveType(targetType)
	if actualResolved == nil || targetResolved == nil {
		return "", "", "", fmt.Errorf("missing cast types")
	}
	if typeLabel(actualResolved) == typeLabel(targetResolved) {
		return operandExpr, "", "", nil
	}
	targetCType, err := s.emitter.cType(targetResolved)
	if err != nil {
		return "", "", "", err
	}
	if actualMoney := moneyType(actualResolved); actualMoney != nil {
		actualCurrency := moneyCurrencyName(actualMoney)
		if targetMoney := moneyType(targetResolved); targetMoney != nil {
			targetCurrency := moneyCurrencyName(targetMoney)
			if actualCurrency != targetCurrency {
				return "", "true", quoteCString(fmt.Sprintf("Cannot convert money[%s] to money[%s] (explicit exchange rate required)", actualCurrency, targetCurrency)), nil
			}
			return operandExpr, "", "", nil
		}
		if isFloatType(targetResolved) {
			return fmt.Sprintf("((%s)gwen_money_to_float(%s))", targetCType, operandExpr), "", "", nil
		}
		if isIntegerType(targetResolved) {
			return fmt.Sprintf("((%s)gwen_money_to_int(%s))", targetCType, operandExpr), "", "", nil
		}
		return "", "true", quoteCString(fmt.Sprintf("Cannot convert money[%s] to %s", actualCurrency, typeLabel(targetResolved))), nil
	}
	if targetMoney := moneyType(targetResolved); targetMoney != nil {
		currency := moneyCurrencyName(targetMoney)
		if isIntegerType(actualResolved) {
			return fmt.Sprintf("gwen_money_from_int((long long)(%s), %s)", operandExpr, quoteCString(currency)), "", "", nil
		}
		if isFloatType(actualResolved) {
			return fmt.Sprintf("gwen_money_from_float((double)(%s), %s)", operandExpr, quoteCString(currency)), "", "", nil
		}
		return "", "true", quoteCString(fmt.Sprintf("Cannot convert %s to money[%s]", typeLabel(actualResolved), currency)), nil
	}
	if isFloatType(targetResolved) {
		if isIntegerType(actualResolved) || isFloatType(actualResolved) {
			return fmt.Sprintf("((%s)(%s))", targetCType, operandExpr), "", "", nil
		}
		return "", "true", quoteCString(fmt.Sprintf("Cannot convert %s to %s", typeLabel(actualResolved), typeLabel(targetResolved))), nil
	}
	if isIntegerType(targetResolved) {
		targetName := namedTypeName(targetResolved)
		if (targetName == "int" || targetName == "int64") && isIntegerType(actualResolved) {
			return fmt.Sprintf("((%s)(%s))", targetCType, operandExpr), "", "", nil
		}
		if typeLabel(actualResolved) == typeLabel(targetResolved) {
			return operandExpr, "", "", nil
		}
		return "", "true", quoteCString(fmt.Sprintf("Cannot convert %s to %s", typeLabel(actualResolved), typeLabel(targetResolved))), nil
	}
	if isStringType(targetResolved) {
		if isStringType(actualResolved) {
			return operandExpr, "", "", nil
		}
		return "", "true", quoteCString(fmt.Sprintf("Cannot convert %s to string", typeLabel(actualResolved))), nil
	}
	if isBoolType(targetResolved) {
		if isBoolType(actualResolved) {
			return operandExpr, "", "", nil
		}
		return "", "true", quoteCString(fmt.Sprintf("Cannot convert %s to bool", typeLabel(actualResolved))), nil
	}
	return "", "true", quoteCString(fmt.Sprintf("Cannot convert %s to %s", typeLabel(actualResolved), typeLabel(targetResolved))), nil
}

func (s *bodyState) emitParallelInst(inst *mir.ParallelInst) error {
	if inst == nil {
		return fmt.Errorf("nil parallel instruction")
	}
	parallelID := s.parallelSerial
	s.parallelSerial++
	firstErrorName := fmt.Sprintf("gwen_parallel_error_%d", parallelID)
	resultNames := make([]string, 0, len(inst.Branches))
	threadNames := make([]string, 0, len(inst.Branches))
	ctxNames := make([]string, 0, len(inst.Branches))
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("const char *%s = NULL;", firstErrorName))
	for idx, branch := range inst.Branches {
		info := s.emitter.parallelByBody[branch]
		if info == nil {
			return fmt.Errorf("missing parallel branch info")
		}
		threadName := fmt.Sprintf("gwen_parallel_thread_%d_%d", parallelID, idx+1)
		ctxName := fmt.Sprintf("gwen_parallel_ctx_%d_%d", parallelID, idx+1)
		resultName := fmt.Sprintf("gwen_parallel_result_%d_%d", parallelID, idx+1)
		resultNames = append(resultNames, resultName)
		threadNames = append(threadNames, threadName)
		ctxNames = append(ctxNames, ctxName)
		s.emitter.writeLine(s.indent + fmt.Sprintf("pthread_t %s;", threadName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s %s;", info.ctxName, ctxName))
		for _, slot := range info.captureSlots {
			argExpr, err := s.captureArgExpr(slot, false)
			if err != nil {
				return fmt.Errorf("parallel capture %q: %w", slot.Name, err)
			}
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s.%s = %s;", ctxName, slotName(slot.ID), argExpr))
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.result = gwen_parallel_task_failure(\"runtime error: parallel task did not start\");", ctxName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("gwen_pthread_require(pthread_create(&%s, NULL, %s, &%s), \"start parallel task\");", threadName, info.entryName, ctxName))
	}
	for idx := range inst.Branches {
		resultName := resultNames[idx]
		threadName := threadNames[idx]
		ctxName := ctxNames[idx]
		s.emitter.writeLine(s.indent + fmt.Sprintf("gwen_pthread_require(pthread_join(%s, NULL), \"join parallel task\");", threadName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("gwen_parallel_task_result %s = %s.result;", resultName, ctxName))
		if !inst.AllowFail {
			s.emitter.writeLine(s.indent + fmt.Sprintf("if (!%s.ok && %s == NULL) %s = %s.error;", resultName, firstErrorName, firstErrorName, resultName))
		}
	}
	if !inst.AllowFail {
		s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s != NULL) gwen_runtime_error(%s);", firstErrorName, firstErrorName))
	}
	if inst.ResultBinding != nil || inst.ResultVar != "" {
		if inst.ResultBinding == nil {
			return fmt.Errorf("parallel results are missing binding metadata")
		}
		resultSlot := s.body.SlotByBindingID(inst.ResultBinding.ID)
		if resultSlot == nil {
			return fmt.Errorf("missing slot for parallel results %q", inst.ResultVar)
		}
		resultExpr := s.slotExpr(resultSlot.ID, inst.ResultBinding)
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_list_from_ptr(gwen_dyn_list_new(%dLL));", resultExpr, len(resultNames)))
		for idx, resultName := range resultNames {
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s.list_value->items[%d] = (%s.ok ? gwen_value_result_ok(%s.value) : gwen_value_result_err(gwen_value_string(%s.error != NULL ? %s.error : \"\")));", resultExpr, idx, resultName, resultName, resultName, resultName))
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = true;", s.slotInitExpr(resultSlot.ID, inst.ResultBinding)))
	}
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) captureArgExpr(slot *mir.Slot, byRef bool) (string, error) {
	if slot == nil {
		return "", fmt.Errorf("nil capture slot")
	}
	typ := slot.Type
	if outer := s.body.SlotByBindingID(slot.BindingID); outer != nil {
		if typ == nil {
			typ = outer.Type
		}
		outerBinding := (*hir.NameBinding)(nil)
		if s.emitter.isGlobalBinding(outer.BindingID) {
			outerBinding = &hir.NameBinding{ID: outer.BindingID, Name: outer.Name}
		}
		if byRef {
			s.requireSlotInitialized(outer.ID, outerBinding)
			return "&" + s.slotExpr(outer.ID, outerBinding), nil
		}
		readExpr, err := s.slotReadExpr(outer.ID, outerBinding, typ)
		if err != nil {
			return "", err
		}
		if cellItemType(s.emitter.resolveType(typ)) != nil {
			return readExpr, nil
		}
		return s.emitter.cloneExpr(readExpr, typ)
	}
	if global := s.emitter.globalSlots[slot.BindingID]; global != nil {
		if typ == nil {
			typ = global.Type
		}
		binding := &hir.NameBinding{ID: slot.BindingID, Name: global.Name}
		if byRef {
			s.requireSlotInitialized(0, binding)
			return "&" + s.emitter.globalName(slot.BindingID), nil
		}
		readExpr, err := s.slotReadExpr(0, binding, typ)
		if err != nil {
			return "", err
		}
		if cellItemType(s.emitter.resolveType(typ)) != nil {
			return readExpr, nil
		}
		return s.emitter.cloneExpr(readExpr, typ)
	}
	return "", fmt.Errorf("missing outer binding %d", slot.BindingID)
}

func (s *bodyState) funcValueExpr(fn *mir.Func, actualName string, funcType *hir.FuncType) (string, error) {
	if funcType == nil {
		return actualName, nil
	}
	typeName, err := s.emitter.funcTypeName(funcType)
	if err != nil {
		return "", err
	}
	if fn == nil || len(functionCaptureSlots(s.emitter, fn.Body)) == 0 {
		return fmt.Sprintf("(%s){NULL, %s}", typeName, closureAdapterName(actualName)), nil
	}
	refCaptures := refCaptureBindingIDSet(fn.Body)
	args := make([]string, 0, len(functionCaptureSlots(s.emitter, fn.Body)))
	for _, slot := range functionCaptureSlots(s.emitter, fn.Body) {
		argExpr, err := s.captureArgExpr(slot, hasCaptureBinding(refCaptures, slot.BindingID))
		if err != nil {
			return "", err
		}
		args = append(args, argExpr)
	}
	return fmt.Sprintf("%s(%s)", closureConstructorName(actualName), strings.Join(args, ", ")), nil
}

func (s *bodyState) builtinFuncValueExpr(moduleName, name string, funcType *hir.FuncType) (string, error) {
	if funcType == nil {
		return "", fmt.Errorf("missing builtin function type")
	}
	typeName, err := s.emitter.funcTypeName(funcType)
	if err != nil {
		return "", err
	}
	wrapper, err := s.emitter.ensureBuiltinClosure(moduleName, name, funcType)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("(%s){NULL, %s}", typeName, wrapper), nil
}

func (s *bodyState) funcInvokeExpr(funcType *hir.FuncType, calleeExpr string, args []string) (string, error) {
	if funcType == nil {
		return "", fmt.Errorf("missing function type")
	}
	typeName, err := s.emitter.funcTypeName(funcType)
	if err != nil {
		return "", err
	}
	parts := []string{calleeExpr}
	parts = append(parts, args...)
	return fmt.Sprintf("%s(%s)", funcCallHelperName(typeName), strings.Join(parts, ", ")), nil
}

func (s *bodyState) emitListValue(valueID int, value *mir.Value) error {
	if value == nil {
		return fmt.Errorf("nil list value")
	}
	temp := tempName(valueID)
	if isBareListType(s.emitter.resolveType(value.Type)) {
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_list_from_ptr(gwen_dyn_list_new(%dLL));", temp, len(value.Elements)))
		for idx, elementID := range value.Elements {
			elementExpr, err := s.dynamicExprForValue(elementID)
			if err != nil {
				return err
			}
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s.list_value->items[%d] = %s;", temp, idx, elementExpr))
		}
		return nil
	}
	listType, err := s.emitter.cTypeForValue(value)
	if err != nil {
		return err
	}
	_, itemType, err := s.emitter.listTypeKeyAndItemType(value.Type)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s.len = %dLL;", temp, len(value.Elements)))
	if len(value.Elements) == 0 {
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.items = NULL;", temp))
		return nil
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s.items = (%s *)malloc(sizeof(%s) * %dULL);", temp, itemType, itemType, len(value.Elements)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory allocating %s\");", temp, listType))
	for idx, elementID := range value.Elements {
		elementExpr, err := s.exprForValue(elementID)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.items[%d] = %s;", temp, idx, elementExpr))
	}
	return nil
}

func (s *bodyState) emitDictValue(valueID int, value *mir.Value) error {
	if value == nil {
		return fmt.Errorf("nil dict value")
	}
	temp := tempName(valueID)
	if isBareDictType(s.emitter.resolveType(value.Type)) {
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_dict_from_ptr(gwen_dyn_dict_new(%dLL));", temp, len(value.Entries)))
		for idx, entry := range value.Entries {
			keyExpr, err := s.dynamicExprForValue(entry.Key)
			if err != nil {
				return err
			}
			valueExpr, err := s.dynamicExprForValue(entry.Value)
			if err != nil {
				return err
			}
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s.dict_value->keys[%d] = %s;", temp, idx, keyExpr))
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s.dict_value->values[%d] = %s;", temp, idx, valueExpr))
		}
		return nil
	}
	dictType, err := s.emitter.cTypeForValue(value)
	if err != nil {
		return err
	}
	_, keyType, valueType, err := s.emitter.dictTypeKeyAndFieldTypes(value.Type)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s.len = %dLL;", temp, len(value.Entries)))
	if len(value.Entries) == 0 {
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.keys = NULL;", temp))
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.values = NULL;", temp))
		return nil
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s.keys = (%s *)malloc(sizeof(%s) * %dULL);", temp, keyType, keyType, len(value.Entries)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s.values = (%s *)malloc(sizeof(%s) * %dULL);", temp, valueType, valueType, len(value.Entries)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s.keys == NULL || %s.values == NULL) gwen_runtime_error(\"runtime error: out of memory allocating %s\");", temp, temp, dictType))
	for idx, entry := range value.Entries {
		keyExpr, err := s.exprForValue(entry.Key)
		if err != nil {
			return err
		}
		valueExpr, err := s.exprForValue(entry.Value)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.keys[%d] = %s;", temp, idx, keyExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.values[%d] = %s;", temp, idx, valueExpr))
	}
	return nil
}

func (s *bodyState) emitObjectValue(valueID int, value *mir.Value) error {
	if value == nil {
		return fmt.Errorf("nil object value")
	}
	info, err := s.emitter.objectInfoForType(value.Type)
	if err != nil {
		return err
	}
	temp := tempName(valueID)
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s *)calloc(1U, sizeof(%s));", temp, info.typeName, info.typeName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s == NULL) gwen_runtime_error(\"runtime error: out of memory allocating %s\");", temp, info.name))
	for _, field := range value.Fields {
		fieldType, err := s.emitter.objectFieldType(info, field.Name)
		if err != nil {
			return err
		}
		fieldExpr, err := s.exprForExpectedValue(field.Value, fieldType)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s->%s = %s;", temp, objectFieldName(field.Name), fieldExpr))
	}
	return nil
}

func (s *bodyState) emitResultValue(valueID int, value *mir.Value) error {
	if value == nil {
		return fmt.Errorf("nil result value")
	}
	if value.Type == nil {
		return fmt.Errorf("result value %d is missing contextual result type", valueID)
	}
	resultType, err := s.emitter.cType(value.Type)
	if err != nil {
		return err
	}
	payloadExpr, err := s.exprForValue(value.Operand)
	if err != nil {
		return err
	}
	okType := resultOKType(value.Type)
	errType := resultErrType(value.Type)
	if okType == nil || errType == nil {
		return fmt.Errorf("unsupported result payload types %s", typeLabel(value.Type))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	zeroErr, err := s.emitter.zeroExpr(errType)
	if err != nil {
		return err
	}
	temp := tempName(valueID)
	switch value.Kind {
	case mir.ValueOk:
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s){true, %s, %s};", temp, resultType, payloadExpr, zeroErr))
	case mir.ValueErr:
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s){false, %s, %s};", temp, resultType, zeroOK, payloadExpr))
	default:
		return fmt.Errorf("unsupported result value kind %q", value.Kind)
	}
	return nil
}

func (s *bodyState) emitIndexValue(valueID int, value *mir.Value) error {
	if value == nil {
		return fmt.Errorf("nil index value")
	}
	object := s.body.Value(value.Object)
	if object == nil {
		return fmt.Errorf("unknown index object value %d", value.Object)
	}
	indexValue := s.body.Value(value.Index)
	if indexValue == nil {
		return fmt.Errorf("unknown index input value %d", value.Index)
	}

	objectExpr, err := s.exprForValue(value.Object)
	if err != nil {
		return err
	}
	indexExpr, err := s.exprForValue(value.Index)
	if err != nil {
		return err
	}

	temp := tempName(valueID)
	objectType := s.emitter.resolveType(object.Type)
	indexType := s.emitter.resolveType(indexValue.Type)
	switch {
	case isDynamicValueType(objectType):
		keyExpr, err := s.dynamicExprForValue(value.Index)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_index(%s, %s);", temp, objectExpr, keyExpr))
		return nil
	case isStringType(objectType):
		if !isIntegerType(indexType) {
			return fmt.Errorf("unsupported string index type %s", typeLabel(indexType))
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_string_index(%s, (long long)(%s));", temp, objectExpr, indexExpr))
		return nil
	case isBareListType(objectType):
		if !isIntegerType(indexType) && !isDynamicValueType(indexType) {
			return fmt.Errorf("unsupported dynamic list index type %s", typeLabel(indexType))
		}
		indexValueExpr := indexExpr
		if isDynamicValueType(indexType) {
			indexValueExpr = fmt.Sprintf("gwen_value_as_int(%s)", indexExpr)
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_list_index(%s, (long long)(%s));", temp, objectExpr, indexValueExpr))
		return nil
	case listItemType(objectType) != nil:
		if !isIntegerType(indexType) {
			return fmt.Errorf("unsupported list index type %s", typeLabel(indexType))
		}
		itemType, err := s.emitter.cType(value.Type)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("if (((long long)(%s)) < 0 || ((long long)(%s)) >= (%s).len) gwen_runtime_error(\"runtime error: index out of range\");", indexExpr, indexExpr, objectExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s)((%s).items[(long long)(%s)]);", temp, itemType, objectExpr, indexExpr))
		return nil
	case isBareDictType(objectType):
		keyExpr, err := s.dynamicExprForValue(value.Index)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_dict_index(%s, %s);", temp, objectExpr, keyExpr))
		return nil
	case dictValueType(objectType) != nil:
		keyType := dictKeyType(objectType)
		if keyType == nil || typeLabel(s.emitter.resolveType(keyType)) != typeLabel(indexType) {
			return fmt.Errorf("unsupported dict index type %s for %s", typeLabel(indexType), typeLabel(objectType))
		}
		dictType, err := s.emitter.cType(objectType)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s, %s);", temp, dictIndexFuncName(dictType), objectExpr, indexExpr))
		return nil
	default:
		return fmt.Errorf("unsupported index target type %s", typeLabel(objectType))
	}
}

func (s *bodyState) emitIndexStore(place *mir.Place, valueID int) error {
	if place == nil {
		return fmt.Errorf("nil index store place")
	}
	object := s.body.Value(place.Object)
	if object == nil {
		return fmt.Errorf("unknown store index object value %d", place.Object)
	}
	indexValue := s.body.Value(place.Index)
	if indexValue == nil {
		return fmt.Errorf("unknown store index input value %d", place.Index)
	}
	objectExpr, err := s.exprForMutableValue(place.Object)
	if err != nil {
		return err
	}
	indexExpr, err := s.exprForValue(place.Index)
	if err != nil {
		return err
	}
	valueExpr, err := s.exprForValue(valueID)
	if err != nil {
		return err
	}
	objectType := s.emitter.resolveType(object.Type)
	indexType := s.emitter.resolveType(indexValue.Type)
	switch {
	case isDynamicValueType(objectType):
		keyExpr, err := s.dynamicExprForValue(place.Index)
		if err != nil {
			return err
		}
		dynamicValueExpr, err := s.dynamicExprForValue(valueID)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("gwen_value_index_set(&(%s), %s, %s);", objectExpr, keyExpr, dynamicValueExpr))
		return nil
	case isBareListType(objectType):
		indexValueExpr := indexExpr
		if isDynamicValueType(indexType) {
			indexValueExpr = fmt.Sprintf("gwen_value_as_int(%s)", indexExpr)
		}
		dynamicValueExpr, err := s.dynamicExprForValue(valueID)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("gwen_value_list_set(&(%s), (long long)(%s), %s);", objectExpr, indexValueExpr, dynamicValueExpr))
		return nil
	case listItemType(objectType) != nil:
		if !isIntegerType(indexType) {
			return fmt.Errorf("unsupported list index store type %s", typeLabel(indexType))
		}
		itemType, err := s.emitter.cType(place.Type)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("if (((long long)(%s)) < 0 || ((long long)(%s)) >= (%s).len) gwen_runtime_error(\"runtime error: index out of range\");", indexExpr, indexExpr, objectExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("(%s).items[(long long)(%s)] = (%s)(%s);", objectExpr, indexExpr, itemType, valueExpr))
		return nil
	case isBareDictType(objectType):
		keyExpr, err := s.dynamicExprForValue(place.Index)
		if err != nil {
			return err
		}
		dynamicValueExpr, err := s.dynamicExprForValue(valueID)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("gwen_value_dict_set(&(%s), %s, %s);", objectExpr, keyExpr, dynamicValueExpr))
		return nil
	case dictValueType(objectType) != nil:
		keyType := dictKeyType(objectType)
		if keyType == nil || typeLabel(s.emitter.resolveType(keyType)) != typeLabel(indexType) {
			return fmt.Errorf("unsupported dict index store type %s for %s", typeLabel(indexType), typeLabel(objectType))
		}
		dictType, err := s.emitter.cType(objectType)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s(&(%s), %s, %s);", dictSetFuncName(dictType), objectExpr, indexExpr, valueExpr))
		return nil
	default:
		return fmt.Errorf("unsupported index store target type %s", typeLabel(objectType))
	}
}

func (s *bodyState) emitFieldStore(place *mir.Place, valueID int) error {
	if place == nil {
		return fmt.Errorf("nil field store place")
	}
	objectExpr, err := s.exprForMutableValue(place.Object)
	if err != nil {
		return err
	}
	valueExpr, err := s.exprForExpectedValue(valueID, place.Type)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("(%s)->%s = %s;", objectExpr, objectFieldName(place.Member), valueExpr))
	return nil
}

func (s *bodyState) emitCallInst(inst *mir.CallInst) error {
	call := s.body.Value(inst.ValueID)
	if call == nil {
		return fmt.Errorf("unknown MIR call value %d", inst.ValueID)
	}
	if call.Kind != mir.ValueCall {
		return fmt.Errorf("value %d is %q, not call", inst.ValueID, call.Kind)
	}
	if moduleName, callName, ok := s.builtinCallIdentity(call.Callee); ok {
		handled, err := s.emitBuiltinCall(moduleName, callName, call, inst)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
		label := callName
		if moduleName != "" {
			label = moduleName + "." + callName
		}
		return fmt.Errorf("unsupported builtin call %q", label)
	}

	callExpr, err := s.callExpr(call)
	if err != nil {
		return err
	}
	switch {
	case len(inst.ResultIDs) == 0 && len(call.ReturnTypes) == 0:
		s.emitter.writeLine(s.indent + callExpr + ";")
	case len(inst.ResultIDs) == 0:
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", tempName(inst.ValueID), callExpr))
	default:
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", tempName(inst.ValueID), callExpr))
		for idx, resultID := range inst.ResultIDs {
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s.%s;", tempName(resultID), tempName(inst.ValueID), tupleFieldName(idx)))
		}
	}
	return nil
}

func (s *bodyState) emitBuiltinCall(moduleName, callName string, call *mir.Value, inst *mir.CallInst) (bool, error) {
	switch {
	case moduleName == "" && callName == "write":
		return true, s.emitBuiltinWrite(call)
	case moduleName == "" && callName == "read":
		return true, s.emitBuiltinReadCall(call, inst)
	case moduleName == "" && callName == "len":
		return true, s.emitBuiltinLenCall(call, inst)
	case moduleName == "" && callName == "str":
		return true, s.emitBuiltinStrCall(call, inst)
	case moduleName == "" && callName == "typeof":
		return true, s.emitBuiltinTypeofCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "append":
		return true, s.emitBuiltinAppendCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "pop":
		return true, s.emitBuiltinPopCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "insert":
		return true, s.emitBuiltinInsertCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "removeat":
		return true, s.emitBuiltinRemoveAtCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "map":
		return true, s.emitBuiltinMapCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "filter":
		return true, s.emitBuiltinFilterCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "range":
		return true, s.emitBuiltinRangeCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "enumerate":
		return true, s.emitBuiltinEnumerateCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "concat":
		return true, s.emitBuiltinConcatCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "reversed":
		return true, s.emitBuiltinReversedCall(call, inst)
	case (moduleName == "" || moduleName == "list") && callName == "sort":
		return true, s.emitBuiltinSortCall(call, inst)
	case moduleName == "" && (callName == "int" || callName == "float"):
		return true, s.emitBuiltinCastCall(call, callName, inst)
	case (moduleName == "" || moduleName == "string") && callName == "split":
		return true, s.emitBuiltinSplitCall(call, inst)
	case (moduleName == "" || moduleName == "string") && callName == "join":
		return true, s.emitBuiltinJoinCall(call, inst)
	case (moduleName == "" || moduleName == "string") && callName == "substring":
		return true, s.emitBuiltinDirectCall(call, inst, "substring", "gwen_string_substring", 3)
	case (moduleName == "" || moduleName == "string") && (callName == "startswith" || callName == "endswith" || callName == "contains"):
		return true, s.emitBuiltinStringBoolCall(callName, call, inst)
	case (moduleName == "" || moduleName == "string") && callName == "trim":
		return true, s.emitBuiltinDirectCall(call, inst, "trim", "gwen_string_trim", 1)
	case (moduleName == "" || moduleName == "string") && callName == "replace":
		return true, s.emitBuiltinDirectCall(call, inst, "replace", "gwen_string_replace", 3)
	case (moduleName == "" || moduleName == "math") && callName == "abs":
		return true, s.emitBuiltinAbsCall(call, inst)
	case (moduleName == "" || moduleName == "math") && (callName == "min" || callName == "max"):
		return true, s.emitBuiltinMinMaxCall(callName, call, inst)
	case (moduleName == "" || moduleName == "math") && callName == "sqrt":
		return true, s.emitBuiltinUnaryMathCall(call, inst, "sqrt", "sqrt")
	case (moduleName == "" || moduleName == "math") && callName == "floor":
		return true, s.emitBuiltinUnaryMathCall(call, inst, "floor", "floor")
	case (moduleName == "" || moduleName == "math") && callName == "ceil":
		return true, s.emitBuiltinUnaryMathCall(call, inst, "ceil", "ceil")
	case (moduleName == "" || moduleName == "dict") && callName == "haskey":
		return true, s.emitBuiltinHasKeyCall(call, inst)
	case (moduleName == "" || moduleName == "dict") && callName == "get":
		return true, s.emitBuiltinGetCall(call, inst)
	case (moduleName == "" || moduleName == "dict") && callName == "keys":
		return true, s.emitBuiltinKeysCall(call, inst)
	case (moduleName == "" || moduleName == "dict") && callName == "values":
		return true, s.emitBuiltinValuesCall(call, inst)
	case (moduleName == "" || moduleName == "dict") && callName == "items":
		return true, s.emitBuiltinItemsCall(call, inst)
	case moduleName == "json" && callName == "objectof":
		return true, s.emitBuiltinJSONObjectCall(call, inst)
	case moduleName == "json" && callName == "arrayof":
		return true, s.emitBuiltinJSONArrayCall(call, inst)
	case moduleName == "json" && callName == "null":
		return true, s.emitBuiltinJSONNullCall(call, inst)
	case moduleName == "json" && callName == "isnull":
		return true, s.emitBuiltinJSONIsNullCall(call, inst)
	case moduleName == "json" && callName == "parseobject":
		return true, s.emitBuiltinJSONParseObjectCall(call, inst)
	case moduleName == "json" && callName == "parsearray":
		return true, s.emitBuiltinJSONParseArrayCall(call, inst)
	case moduleName == "json" && callName == "stringify":
		return true, s.emitBuiltinJSONStringifyCall(call, inst)
	case moduleName == "http" && callName == "get":
		return true, s.emitBuiltinHTTPGetCall(call, inst)
	case moduleName == "http" && callName == "request":
		return true, s.emitBuiltinHTTPRequestCall(call, inst)
	case moduleName == "http" && (callName == "method" || callName == "path" || callName == "requestbody"):
		return true, s.emitBuiltinHTTPRequestFieldCall(callName, call, inst)
	case moduleName == "http" && callName == "addr":
		return true, s.emitBuiltinHTTPServerAddrCall(call, inst)
	case moduleName == "http" && callName == "query":
		return true, s.emitBuiltinHTTPQueryCall(call, inst)
	case moduleName == "http" && (callName == "status" || callName == "responsebody"):
		return true, s.emitBuiltinHTTPResponseFieldCall(callName, call, inst)
	case moduleName == "http" && callName == "responseheader":
		return true, s.emitBuiltinHTTPResponseHeaderCall(call, inst)
	case moduleName == "http" && callName == "requestheader":
		return true, s.emitBuiltinHTTPRequestHeaderCall(call, inst)
	case moduleName == "http" && callName == "requestcookie":
		return true, s.emitBuiltinHTTPRequestCookieCall(call, inst)
	case moduleName == "http" && callName == "text":
		return true, s.emitBuiltinHTTPTextLikeCall(call, inst, "text/plain; charset=utf-8")
	case moduleName == "http" && callName == "html":
		return true, s.emitBuiltinHTTPTextLikeCall(call, inst, "text/html; charset=utf-8")
	case moduleName == "http" && callName == "redirect":
		return true, s.emitBuiltinHTTPRedirectCall(call, inst)
	case moduleName == "http" && callName == "withheader":
		return true, s.emitBuiltinHTTPWithHeaderCall(call, inst)
	case moduleName == "http" && callName == "withcookie":
		return true, s.emitBuiltinHTTPWithCookieCall(call, inst)
	case moduleName == "http" && callName == "json":
		return true, s.emitBuiltinHTTPJSONCall(call, inst)
	case moduleName == "http" && callName == "route":
		return true, s.emitBuiltinHTTPRouteCall(call, inst)
	case moduleName == "http" && callName == "static":
		return true, s.emitBuiltinHTTPStaticCall(call, inst)
	case moduleName == "http" && callName == "listen":
		return true, s.emitBuiltinHTTPListenCall(call, inst)
	case moduleName == "http" && callName == "wait":
		return true, s.emitBuiltinHTTPWaitCall(call, inst)
	case moduleName == "http" && callName == "close":
		return true, s.emitBuiltinHTTPCloseCall(call, inst)
	case moduleName == "state" && callName == "cell":
		return true, s.emitBuiltinStateCellCall(call, inst)
	case moduleName == "state" && callName == "get":
		return true, s.emitBuiltinStateGetCall(call, inst)
	case moduleName == "state" && callName == "set":
		return true, s.emitBuiltinStateSetCall(call, inst)
	case moduleName == "state" && callName == "update":
		return true, s.emitBuiltinStateUpdateCall(call, inst)
	case moduleName == "sqlite" && callName == "open":
		return true, s.emitBuiltinSQLiteOpenCall(call, inst)
	case moduleName == "sqlite" && callName == "close":
		return true, s.emitBuiltinSQLiteCloseCall(call, inst)
	case moduleName == "sqlite" && callName == "exec":
		return true, s.emitBuiltinSQLiteExecCall(call, inst)
	case moduleName == "sqlite" && callName == "query":
		return true, s.emitBuiltinSQLiteQueryCall(call, inst)
	case moduleName == "os" && callName == "args":
		return true, s.emitBuiltinOSArgsCall(call, inst)
	case moduleName == "os" && callName == "cwd":
		return true, s.emitBuiltinDirectCall(call, inst, "os.cwd", "gwen_os_cwd", 0)
	case moduleName == "os" && callName == "getenv":
		return true, s.emitBuiltinDirectCall(call, inst, "os.getenv", "gwen_os_getenv", 1)
	case moduleName == "time" && callName == "nowunix":
		return true, s.emitBuiltinDirectCall(call, inst, "time.nowunix", "gwen_time_nowunix", 0)
	case moduleName == "time" && callName == "nowunixms":
		return true, s.emitBuiltinDirectCall(call, inst, "time.nowunixms", "gwen_time_nowunixms", 0)
	case moduleName == "time" && callName == "sleep":
		return true, s.emitBuiltinVoidCall(call, "time.sleep", "gwen_time_sleep", 1)
	case moduleName == "time" && callName == "nowrfc3339":
		return true, s.emitBuiltinDirectCall(call, inst, "time.nowrfc3339", "gwen_time_nowrfc3339", 0)
	case (moduleName == "" || moduleName == "io") && callName == "readfile":
		return true, s.emitBuiltinDirectCall(call, inst, "readfile", "gwen_io_readfile", 1)
	case (moduleName == "" || moduleName == "io") && callName == "readdir":
		return true, s.emitBuiltinDirectCall(call, inst, "readdir", "gwen_io_readdir", 1)
	case (moduleName == "" || moduleName == "io") && callName == "writefile":
		return true, s.emitBuiltinDirectCall(call, inst, "writefile", "gwen_io_writefile", 2)
	case (moduleName == "" || moduleName == "io") && callName == "appendfile":
		return true, s.emitBuiltinDirectCall(call, inst, "appendfile", "gwen_io_appendfile", 2)
	case moduleName == "path" && callName == "basename":
		return true, s.emitBuiltinDirectCall(call, inst, "path.basename", "gwen_path_basename", 1)
	case moduleName == "path" && callName == "dirname":
		return true, s.emitBuiltinDirectCall(call, inst, "path.dirname", "gwen_path_dirname", 1)
	case moduleName == "path" && callName == "joinpath":
		return true, s.emitBuiltinDirectCall(call, inst, "path.joinpath", "gwen_path_join", 2)
	default:
		return false, nil
	}
}

func (s *bodyState) emitBuiltinWrite(call *mir.Value) error {
	for idx, argID := range call.Args {
		if idx > 0 {
			s.emitter.writeLine(s.indent + "fputc(' ', stdout);")
		}
		arg := s.body.Value(argID)
		if arg == nil {
			return fmt.Errorf("unknown MIR write arg value %d", argID)
		}
		argExpr, err := s.exprForValue(argID)
		if err != nil {
			return err
		}
		if isDynamicValueType(s.emitter.resolveType(arg.Type)) {
			s.emitter.writeLine(s.indent + fmt.Sprintf("gwen_write_string(gwen_value_display_string(%s));", argExpr))
			continue
		}
		writeFunc, err := s.emitter.writeHelper(arg.Type)
		if err != nil {
			dynamicExpr, dynamicErr := s.emitter.dynamicValueExpr(argExpr, arg.Type)
			if dynamicErr != nil {
				return fmt.Errorf("write arg %d: %w", idx, err)
			}
			s.emitter.writeLine(s.indent + fmt.Sprintf("gwen_write_string(gwen_value_display_string(%s));", dynamicExpr))
			continue
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s(%s);", writeFunc, argExpr))
	}
	s.emitter.writeLine(s.indent + "fputc('\\n', stdout);")
	return nil
}

func (s *bodyState) emitBuiltinReadCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) > 1 {
		return fmt.Errorf("read() expects at most 1 argument, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("read() unexpectedly produced multi-return MIR")
	}
	promptExpr := "NULL"
	if len(call.Args) == 1 {
		expr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "string"})
		if err != nil {
			return err
		}
		promptExpr = expr
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_read_line(%s);", tempName(inst.ValueID), promptExpr))
	return nil
}

func (s *bodyState) emitBuiltinLenCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("len() expects exactly 1 argument, got %d", len(call.Args))
	}
	targetExpr, err := s.builtinLenExpr(call.Args[0])
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("len() unexpectedly produced multi-return MIR")
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", tempName(inst.ValueID), targetExpr))
	return nil
}

func (s *bodyState) emitBuiltinHasKeyCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("haskey() expects exactly 2 arguments, got %d", len(call.Args))
	}
	dictExpr, helperName, err := s.dictHelperTarget(call.Args[0], dictHasKeyFuncName)
	if err != nil {
		return err
	}
	keyExpr, err := s.exprForValue(call.Args[1])
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("haskey() unexpectedly produced multi-return MIR")
	}
	if helperName == "" {
		keyExpr, err = s.dynamicExprForValue(call.Args[1])
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_dict_haskey(%s, %s);", tempName(inst.ValueID), dictExpr, keyExpr))
		return nil
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s, %s);", tempName(inst.ValueID), helperName, dictExpr, keyExpr))
	return nil
}

func (s *bodyState) emitBuiltinGetCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 3 {
		return fmt.Errorf("get() expects exactly 3 arguments, got %d", len(call.Args))
	}
	dictExpr, helperName, err := s.dictHelperTarget(call.Args[0], dictGetFuncName)
	if err != nil {
		return err
	}
	keyExpr, err := s.exprForValue(call.Args[1])
	if err != nil {
		return err
	}
	fallbackExpr, err := s.exprForValue(call.Args[2])
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("get() unexpectedly produced multi-return MIR")
	}
	if helperName == "" {
		keyExpr, err = s.dynamicExprForValue(call.Args[1])
		if err != nil {
			return err
		}
		fallbackExpr, err = s.dynamicExprForValue(call.Args[2])
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_dict_get(%s, %s, %s);", tempName(inst.ValueID), dictExpr, keyExpr, fallbackExpr))
		return nil
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s, %s, %s);", tempName(inst.ValueID), helperName, dictExpr, keyExpr, fallbackExpr))
	return nil
}

func (s *bodyState) emitBuiltinKeysCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("keys() expects exactly 1 argument, got %d", len(call.Args))
	}
	dictExpr, helperName, err := s.dictHelperTarget(call.Args[0], dictKeysFuncName)
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("keys() unexpectedly produced multi-return MIR")
	}
	if helperName == "" {
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_dict_keys(%s);", tempName(inst.ValueID), dictExpr))
		return nil
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s);", tempName(inst.ValueID), helperName, dictExpr))
	return nil
}

func (s *bodyState) emitBuiltinValuesCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("values() expects exactly 1 argument, got %d", len(call.Args))
	}
	dictExpr, helperName, err := s.dictHelperTarget(call.Args[0], dictValuesFuncName)
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("values() unexpectedly produced multi-return MIR")
	}
	if helperName == "" {
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_dict_values(%s);", tempName(inst.ValueID), dictExpr))
		return nil
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s);", tempName(inst.ValueID), helperName, dictExpr))
	return nil
}

func (s *bodyState) emitBuiltinItemsCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("items() expects exactly 1 argument, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("items() unexpectedly produced multi-return MIR")
	}
	if len(call.ReturnTypes) != 1 {
		return fmt.Errorf("items() expects exactly 1 return type, got %d", len(call.ReturnTypes))
	}
	resultType := s.emitter.resolveType(call.ReturnTypes[0])
	resultIsBare := isBareListType(resultType)
	resultItemType := listItemType(resultType)
	resultTypedPairs := resultItemType != nil && isBareListType(s.emitter.resolveType(resultItemType))
	if !resultIsBare && !resultTypedPairs {
		return fmt.Errorf("compiled items() currently returns bare list or list[list], got %s", typeLabel(call.ReturnTypes[0]))
	}
	pairListTypeName := ""
	var err error
	if resultTypedPairs {
		pairListTypeName, err = s.emitter.cType(resultItemType)
		if err != nil {
			return err
		}
	}
	dictValue := s.body.Value(call.Args[0])
	if dictValue == nil {
		return fmt.Errorf("unknown MIR items dict value %d", call.Args[0])
	}
	dictExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	sourceType := s.emitter.resolveType(dictValue.Type)
	sourceName := fmt.Sprintf("gwen_items_source_%d", inst.ValueID)
	pairName := fmt.Sprintf("gwen_items_pair_%d", inst.ValueID)
	switch {
	case isBareDictType(sourceType):
		s.emitter.writeLine(s.indent + "{")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_dyn_dict *%s = gwen_value_as_dict(%s);", sourceName, dictExpr))
		if resultIsBare {
			s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_value_list_from_ptr(gwen_dyn_list_new(%s->len));", tempName(inst.ValueID), sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = 0; i < %s->len; ++i) {", sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    gwen_value %s = gwen_value_list_from_ptr(gwen_dyn_list_new(2));", pairName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[0] = gwen_value_clone(%s->keys[i]);", pairName, sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[1] = gwen_value_clone(%s->values[i]);", pairName, sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[i] = %s;", tempName(inst.ValueID), pairName))
			s.emitter.writeLine(s.indent + "  }")
		} else {
			s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = %s->len;", tempName(inst.ValueID), sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s.len == 0) {", tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = NULL;", tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + "  } else {")
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = (%s *)malloc(sizeof(%s) * (size_t)%s.len);", tempName(inst.ValueID), pairListTypeName, pairListTypeName, tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory in items()\");", tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    for (long long i = 0; i < %s->len; ++i) {", sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      gwen_value %s = gwen_value_list_from_ptr(gwen_dyn_list_new(2));", pairName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.list_value->items[0] = gwen_value_clone(%s->keys[i]);", pairName, sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.list_value->items[1] = gwen_value_clone(%s->values[i]);", pairName, sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items[i] = %s;", tempName(inst.ValueID), pairName))
			s.emitter.writeLine(s.indent + "    }")
			s.emitter.writeLine(s.indent + "  }")
		}
		s.emitter.writeLine(s.indent + "}")
		return nil
	case dictValueType(sourceType) != nil:
		keyType := dictKeyType(sourceType)
		valueType := dictValueType(sourceType)
		keyExpr, err := s.emitter.dynamicValueExpr(sourceName+".keys[i]", keyType)
		if err != nil {
			return err
		}
		valueExpr, err := s.emitter.dynamicValueExpr(sourceName+".values[i]", valueType)
		if err != nil {
			return err
		}
		dictTypeName, err := s.emitter.cType(sourceType)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + "{")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", dictTypeName, sourceName, dictExpr))
		if resultIsBare {
			s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_value_list_from_ptr(gwen_dyn_list_new(%s.len));", tempName(inst.ValueID), sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = 0; i < %s.len; ++i) {", sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    gwen_value %s = gwen_value_list_from_ptr(gwen_dyn_list_new(2));", pairName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[0] = %s;", pairName, keyExpr))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[1] = %s;", pairName, valueExpr))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[i] = %s;", tempName(inst.ValueID), pairName))
			s.emitter.writeLine(s.indent + "  }")
		} else {
			s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = %s.len;", tempName(inst.ValueID), sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s.len == 0) {", tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = NULL;", tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + "  } else {")
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = (%s *)malloc(sizeof(%s) * (size_t)%s.len);", tempName(inst.ValueID), pairListTypeName, pairListTypeName, tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory in items()\");", tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    for (long long i = 0; i < %s.len; ++i) {", sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      gwen_value %s = gwen_value_list_from_ptr(gwen_dyn_list_new(2));", pairName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.list_value->items[0] = %s;", pairName, keyExpr))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.list_value->items[1] = %s;", pairName, valueExpr))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items[i] = %s;", tempName(inst.ValueID), pairName))
			s.emitter.writeLine(s.indent + "    }")
			s.emitter.writeLine(s.indent + "  }")
		}
		s.emitter.writeLine(s.indent + "}")
		return nil
	default:
		return fmt.Errorf("compiled items() currently requires dict input, got %s", typeLabel(dictValue.Type))
	}
}

func (s *bodyState) emitBuiltinAppendCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("append() expects exactly 2 arguments, got %d", len(call.Args))
	}
	listValue := s.body.Value(call.Args[0])
	if listValue == nil {
		return fmt.Errorf("unknown MIR append list value %d", call.Args[0])
	}
	listExpr, err := s.mutableListTargetExpr(call.Args[0])
	if err != nil {
		return err
	}
	if isBareListType(s.emitter.resolveType(listValue.Type)) {
		itemExpr, err := s.dynamicExprForValue(call.Args[1])
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("gwen_value_list_append(&(%s), %s);", listExpr, itemExpr))
		return nil
	}
	itemType := listItemType(s.emitter.resolveType(listValue.Type))
	if itemType != nil {
		itemExpr, err := s.exprForExpectedValue(call.Args[1], itemType)
		if err != nil {
			return err
		}
		listType, err := s.emitter.cType(listValue.Type)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s(&(%s), %s);", listAppendFuncName(listType), listExpr, itemExpr))
		return nil
	}
	return fmt.Errorf("unsupported append() target type %s", typeLabel(listValue.Type))
}

func (s *bodyState) emitBuiltinPopCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("pop() expects exactly 1 argument, got %d", len(call.Args))
	}
	if len(call.ReturnTypes) != 1 {
		return fmt.Errorf("pop() expects exactly 1 return type, got %d", len(call.ReturnTypes))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("pop() unexpectedly produced multi-return MIR")
	}
	listValue := s.body.Value(call.Args[0])
	if listValue == nil {
		return fmt.Errorf("unknown MIR pop list value %d", call.Args[0])
	}
	listExpr, err := s.mutableListTargetExpr(call.Args[0])
	if err != nil {
		return err
	}
	listType := s.emitter.resolveType(listValue.Type)
	if isBareListType(listType) {
		targetName := fmt.Sprintf("gwen_pop_target_%d", inst.ValueID)
		s.emitter.writeLine(s.indent + "{")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_dyn_list *%s = gwen_value_as_list(%s);", targetName, listExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s->len == 0LL) gwen_runtime_error(\"runtime error: pop() from empty list\");", targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = %s->items[%s->len - 1LL];", tempName(inst.ValueID), targetName, targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->len--;", targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s->len <= 0LL) {", targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    free(%s->items);", targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s->items = NULL;", targetName))
		s.emitter.writeLine(s.indent + "  } else {")
		s.emitter.writeLine(s.indent + fmt.Sprintf("    gwen_value *gwen_pop_items_%d = (gwen_value *)realloc(%s->items, sizeof(gwen_value) * (size_t)%s->len);", inst.ValueID, targetName, targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    if (gwen_pop_items_%d != NULL) {", inst.ValueID))
		s.emitter.writeLine(s.indent + fmt.Sprintf("      %s->items = gwen_pop_items_%d;", targetName, inst.ValueID))
		s.emitter.writeLine(s.indent + "    }")
		s.emitter.writeLine(s.indent + "  }")
		s.emitter.writeLine(s.indent + "}")
		return nil
	}
	itemType := listItemType(listType)
	if itemType == nil {
		return fmt.Errorf("unsupported pop() target type %s", typeLabel(listValue.Type))
	}
	listTypeName, err := s.emitter.cType(listType)
	if err != nil {
		return err
	}
	itemCType, err := s.emitter.cType(itemType)
	if err != nil {
		return err
	}
	targetName := fmt.Sprintf("gwen_pop_target_%d", inst.ValueID)
	itemsName := fmt.Sprintf("gwen_pop_items_%d", inst.ValueID)
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s *%s = &(%s);", listTypeName, targetName, listExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s->len == 0LL) gwen_runtime_error(\"runtime error: pop() from empty list\");", targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = %s->items[%s->len - 1LL];", tempName(inst.ValueID), targetName, targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->len--;", targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s->len <= 0LL) {", targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    free(%s->items);", targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s->items = NULL;", targetName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s *%s = (%s *)realloc(%s->items, sizeof(%s) * (size_t)%s->len);", itemCType, itemsName, itemCType, targetName, itemCType, targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s != NULL) {", itemsName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      %s->items = %s;", targetName, itemsName))
	s.emitter.writeLine(s.indent + "    }")
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinInsertCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 3 {
		return fmt.Errorf("insert() expects exactly 3 arguments, got %d", len(call.Args))
	}
	if len(call.ReturnTypes) != 0 {
		return fmt.Errorf("insert() unexpectedly produced return types")
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("insert() unexpectedly produced multi-return MIR")
	}
	listValue := s.body.Value(call.Args[0])
	if listValue == nil {
		return fmt.Errorf("unknown MIR insert list value %d", call.Args[0])
	}
	listExpr, err := s.mutableListTargetExpr(call.Args[0])
	if err != nil {
		return err
	}
	indexExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "int"})
	if err != nil {
		return err
	}
	listType := s.emitter.resolveType(listValue.Type)
	if isBareListType(listType) {
		itemExpr, err := s.dynamicExprForValue(call.Args[2])
		if err != nil {
			return err
		}
		targetName := fmt.Sprintf("gwen_insert_target_%d", inst.ValueID)
		indexName := fmt.Sprintf("gwen_insert_index_%d", inst.ValueID)
		s.emitter.writeLine(s.indent + "{")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_dyn_list *%s = gwen_value_as_list(%s);", targetName, listExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  long long %s = (long long)(%s);", indexName, indexExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s < 0LL || %s > %s->len) gwen_runtime_error(\"runtime error: insert() index out of range\");", indexName, indexName, targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->items = (gwen_value *)realloc(%s->items, sizeof(gwen_value) * (size_t)(%s->len + 1LL));", targetName, targetName, targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s->items == NULL) gwen_runtime_error(\"runtime error: out of memory inserting into dynamic list\");", targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = %s->len; i > %s; --i) {", targetName, indexName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s->items[i] = %s->items[i - 1LL];", targetName, targetName))
		s.emitter.writeLine(s.indent + "  }")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->items[%s] = %s;", targetName, indexName, itemExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->len++;", targetName))
		s.emitter.writeLine(s.indent + "}")
		return nil
	}
	itemType := listItemType(listType)
	if itemType == nil {
		return fmt.Errorf("unsupported insert() target type %s", typeLabel(listValue.Type))
	}
	itemExpr, err := s.exprForExpectedValue(call.Args[2], itemType)
	if err != nil {
		return err
	}
	listTypeName, err := s.emitter.cType(listType)
	if err != nil {
		return err
	}
	itemCType, err := s.emitter.cType(itemType)
	if err != nil {
		return err
	}
	targetName := fmt.Sprintf("gwen_insert_target_%d", inst.ValueID)
	indexName := fmt.Sprintf("gwen_insert_index_%d", inst.ValueID)
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s *%s = &(%s);", listTypeName, targetName, listExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  long long %s = (long long)(%s);", indexName, indexExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s < 0LL || %s > %s->len) gwen_runtime_error(\"runtime error: insert() index out of range\");", indexName, indexName, targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->items = (%s *)realloc(%s->items, sizeof(%s) * (size_t)(%s->len + 1LL));", targetName, itemCType, targetName, itemCType, targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s->items == NULL) gwen_runtime_error(\"runtime error: out of memory inserting into %s\");", targetName, listTypeName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = %s->len; i > %s; --i) {", targetName, indexName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s->items[i] = %s->items[i - 1LL];", targetName, targetName))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->items[%s] = (%s)(%s);", targetName, indexName, itemCType, itemExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->len++;", targetName))
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinRemoveAtCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("removeat() expects exactly 2 arguments, got %d", len(call.Args))
	}
	if len(call.ReturnTypes) != 1 {
		return fmt.Errorf("removeat() expects exactly 1 return type, got %d", len(call.ReturnTypes))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("removeat() unexpectedly produced multi-return MIR")
	}
	listValue := s.body.Value(call.Args[0])
	if listValue == nil {
		return fmt.Errorf("unknown MIR removeat list value %d", call.Args[0])
	}
	listExpr, err := s.mutableListTargetExpr(call.Args[0])
	if err != nil {
		return err
	}
	indexExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "int"})
	if err != nil {
		return err
	}
	listType := s.emitter.resolveType(listValue.Type)
	if isBareListType(listType) {
		targetName := fmt.Sprintf("gwen_removeat_target_%d", inst.ValueID)
		indexName := fmt.Sprintf("gwen_removeat_index_%d", inst.ValueID)
		s.emitter.writeLine(s.indent + "{")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_dyn_list *%s = gwen_value_as_list(%s);", targetName, listExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  long long %s = (long long)(%s);", indexName, indexExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s < 0LL || %s >= %s->len) gwen_runtime_error(\"runtime error: removeat() index out of range\");", indexName, indexName, targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = %s->items[%s];", tempName(inst.ValueID), targetName, indexName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = %s + 1LL; i < %s->len; ++i) {", indexName, targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s->items[i - 1LL] = %s->items[i];", targetName, targetName))
		s.emitter.writeLine(s.indent + "  }")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->len--;", targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s->len <= 0LL) {", targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    free(%s->items);", targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s->items = NULL;", targetName))
		s.emitter.writeLine(s.indent + "  } else {")
		s.emitter.writeLine(s.indent + fmt.Sprintf("    gwen_value *gwen_removeat_items_%d = (gwen_value *)realloc(%s->items, sizeof(gwen_value) * (size_t)%s->len);", inst.ValueID, targetName, targetName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    if (gwen_removeat_items_%d != NULL) {", inst.ValueID))
		s.emitter.writeLine(s.indent + fmt.Sprintf("      %s->items = gwen_removeat_items_%d;", targetName, inst.ValueID))
		s.emitter.writeLine(s.indent + "    }")
		s.emitter.writeLine(s.indent + "  }")
		s.emitter.writeLine(s.indent + "}")
		return nil
	}
	itemType := listItemType(listType)
	if itemType == nil {
		return fmt.Errorf("unsupported removeat() target type %s", typeLabel(listValue.Type))
	}
	listTypeName, err := s.emitter.cType(listValue.Type)
	if err != nil {
		return err
	}
	itemCType, err := s.emitter.cType(itemType)
	if err != nil {
		return err
	}
	targetName := fmt.Sprintf("gwen_removeat_target_%d", inst.ValueID)
	indexName := fmt.Sprintf("gwen_removeat_index_%d", inst.ValueID)
	itemsName := fmt.Sprintf("gwen_removeat_items_%d", inst.ValueID)
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s *%s = &(%s);", listTypeName, targetName, listExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  long long %s = (long long)(%s);", indexName, indexExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s < 0LL || %s >= %s->len) gwen_runtime_error(\"runtime error: removeat() index out of range\");", indexName, indexName, targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = %s->items[%s];", tempName(inst.ValueID), targetName, indexName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = %s + 1LL; i < %s->len; ++i) {", indexName, targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s->items[i - 1LL] = %s->items[i];", targetName, targetName))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s->len--;", targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s->len <= 0LL) {", targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    free(%s->items);", targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s->items = NULL;", targetName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s *%s = (%s *)realloc(%s->items, sizeof(%s) * (size_t)%s->len);", itemCType, itemsName, itemCType, targetName, itemCType, targetName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s != NULL) {", itemsName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      %s->items = %s;", targetName, itemsName))
	s.emitter.writeLine(s.indent + "    }")
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinMapCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("map() expects exactly 2 arguments, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("map() unexpectedly produced multi-return MIR")
	}
	if len(call.ReturnTypes) != 1 {
		return fmt.Errorf("map() expects exactly 1 return type, got %d", len(call.ReturnTypes))
	}
	listValue := s.body.Value(call.Args[0])
	if listValue == nil {
		return fmt.Errorf("unknown MIR map list value %d", call.Args[0])
	}
	callbackValue := s.body.Value(call.Args[1])
	if callbackValue == nil {
		return fmt.Errorf("unknown MIR map callback value %d", call.Args[1])
	}
	listType := s.emitter.resolveType(listValue.Type)
	sourceIsBare := isBareListType(listType)
	itemType := listItemType(listType)
	if !sourceIsBare && itemType == nil {
		return fmt.Errorf("compiled map() currently requires list input, got %s", typeLabel(listValue.Type))
	}
	callbackType, ok := s.emitter.resolveType(callbackValue.Type).(*hir.FuncType)
	if !ok || len(callbackType.Params) != 1 || len(callbackType.Returns) != 1 {
		return fmt.Errorf("unsupported map() callback type %s", typeLabel(callbackValue.Type))
	}
	resultType := s.emitter.resolveType(call.ReturnTypes[0])
	resultIsBare := isBareListType(resultType)
	resultItemType := listItemType(resultType)
	if !resultIsBare && resultItemType == nil {
		return fmt.Errorf("unsupported map() return type %s", typeLabel(call.ReturnTypes[0]))
	}
	listExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	callbackExpr, err := s.exprForValue(call.Args[1])
	if err != nil {
		return err
	}
	sourceName := fmt.Sprintf("gwen_map_source_%d", inst.ValueID)
	sourceTypeName := "gwen_value"
	sourceLenExpr := sourceName + ".list_value->len"
	sourceItemExpr := sourceName + ".list_value->items[i]"
	if !sourceIsBare {
		sourceTypeName, err = s.emitter.cType(listType)
		if err != nil {
			return err
		}
		sourceLenExpr = sourceName + ".len"
		sourceItemExpr = sourceName + ".items[i]"
	}
	argExpr := ""
	if sourceIsBare {
		argExpr, err = s.emitter.coerceDynamicExpr(sourceItemExpr, callbackType.Params[0])
		if err != nil {
			return fmt.Errorf("unsupported map() callback parameter type %s", typeLabel(callbackType.Params[0]))
		}
	} else {
		argExpr, err = s.coerceExpr(sourceItemExpr, itemType, callbackType.Params[0])
		if err != nil {
			return err
		}
	}
	callExpr, err := s.funcInvokeExpr(callbackType, callbackExpr, []string{argExpr})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", sourceTypeName, sourceName, listExpr))
	if resultIsBare {
		resultItemExpr, err := s.emitter.dynamicValueExpr(callExpr, callbackType.Returns[0])
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_value_list_from_ptr(gwen_dyn_list_new(%s));", tempName(inst.ValueID), sourceLenExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = 0; i < %s; ++i) {", sourceLenExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[i] = %s;", tempName(inst.ValueID), resultItemExpr))
		s.emitter.writeLine(s.indent + "  }")
		s.emitter.writeLine(s.indent + "}")
		return nil
	}
	resultItemExpr, err := s.coerceExpr(callExpr, callbackType.Returns[0], resultItemType)
	if err != nil {
		return err
	}
	resultItemCType, err := s.emitter.cType(resultItemType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = %s;", tempName(inst.ValueID), sourceLenExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s == 0) {", sourceLenExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = NULL;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = (%s *)malloc(sizeof(%s) * (size_t)%s);", tempName(inst.ValueID), resultItemCType, resultItemCType, sourceLenExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory in map()\");", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    for (long long i = 0; i < %s; ++i) {", sourceLenExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items[i] = %s;", tempName(inst.ValueID), resultItemExpr))
	s.emitter.writeLine(s.indent + "    }")
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinFilterCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("filter() expects exactly 2 arguments, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("filter() unexpectedly produced multi-return MIR")
	}
	if len(call.ReturnTypes) != 1 {
		return fmt.Errorf("filter() expects exactly 1 return type, got %d", len(call.ReturnTypes))
	}
	listValue := s.body.Value(call.Args[0])
	if listValue == nil {
		return fmt.Errorf("unknown MIR filter list value %d", call.Args[0])
	}
	callbackValue := s.body.Value(call.Args[1])
	if callbackValue == nil {
		return fmt.Errorf("unknown MIR filter callback value %d", call.Args[1])
	}
	listType := s.emitter.resolveType(listValue.Type)
	sourceIsBare := isBareListType(listType)
	itemType := listItemType(listType)
	if !sourceIsBare && itemType == nil {
		return fmt.Errorf("compiled filter() currently requires list input, got %s", typeLabel(listValue.Type))
	}
	callbackType, ok := s.emitter.resolveType(callbackValue.Type).(*hir.FuncType)
	if !ok || len(callbackType.Params) != 1 || len(callbackType.Returns) != 1 {
		return fmt.Errorf("unsupported filter() callback type %s", typeLabel(callbackValue.Type))
	}
	if !isBoolType(s.emitter.resolveType(callbackType.Returns[0])) {
		return fmt.Errorf("filter() callback must return bool, got %s", typeLabel(callbackType.Returns[0]))
	}
	resultType := s.emitter.resolveType(call.ReturnTypes[0])
	resultIsBare := isBareListType(resultType)
	resultItemType := listItemType(resultType)
	if !resultIsBare && resultItemType == nil {
		return fmt.Errorf("unsupported filter() return type %s", typeLabel(call.ReturnTypes[0]))
	}
	listExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	callbackExpr, err := s.exprForValue(call.Args[1])
	if err != nil {
		return err
	}
	sourceName := fmt.Sprintf("gwen_filter_source_%d", inst.ValueID)
	itemsName := fmt.Sprintf("gwen_filter_items_%d", inst.ValueID)
	sourceTypeName := "gwen_value"
	sourceLenExpr := sourceName + ".list_value->len"
	sourceItemExpr := sourceName + ".list_value->items[i]"
	if !sourceIsBare {
		sourceTypeName, err = s.emitter.cType(listType)
		if err != nil {
			return err
		}
		sourceLenExpr = sourceName + ".len"
		sourceItemExpr = sourceName + ".items[i]"
	}
	argExpr := ""
	if sourceIsBare {
		argExpr, err = s.emitter.coerceDynamicExpr(sourceItemExpr, callbackType.Params[0])
		if err != nil {
			return fmt.Errorf("unsupported filter() callback parameter type %s", typeLabel(callbackType.Params[0]))
		}
	} else {
		argExpr, err = s.coerceExpr(sourceItemExpr, itemType, callbackType.Params[0])
		if err != nil {
			return err
		}
	}
	callExpr, err := s.funcInvokeExpr(callbackType, callbackExpr, []string{argExpr})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", sourceTypeName, sourceName, listExpr))
	if resultIsBare {
		itemExpr := sourceItemExpr
		if !sourceIsBare {
			itemExpr, err = s.emitter.dynamicValueExpr(sourceItemExpr, itemType)
			if err != nil {
				return err
			}
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_value_list_from_ptr(gwen_dyn_list_new(0));", tempName(inst.ValueID)))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = 0; i < %s; ++i) {", sourceLenExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s) {", callExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("      gwen_value_list_append(&%s, %s);", tempName(inst.ValueID), itemExpr))
		s.emitter.writeLine(s.indent + "    }")
		s.emitter.writeLine(s.indent + "  }")
		s.emitter.writeLine(s.indent + "}")
		return nil
	}
	itemExpr := sourceItemExpr
	if sourceIsBare {
		itemExpr, err = s.emitter.coerceDynamicExpr(sourceItemExpr, resultItemType)
		if err != nil {
			return err
		}
	} else {
		itemExpr, err = s.coerceExpr(sourceItemExpr, itemType, resultItemType)
		if err != nil {
			return err
		}
	}
	itemCType, err := s.emitter.cType(resultItemType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = 0;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.items = NULL;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s > 0) {", sourceLenExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = (%s *)malloc(sizeof(%s) * (size_t)%s);", tempName(inst.ValueID), itemCType, itemCType, sourceLenExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory in filter()\");", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    for (long long i = 0; i < %s; ++i) {", sourceLenExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      if (%s) {", callExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("        %s.items[%s.len] = %s;", tempName(inst.ValueID), tempName(inst.ValueID), itemExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("        %s.len++;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + "      }")
	s.emitter.writeLine(s.indent + "    }")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.len == 0) {", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      free(%s.items);", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items = NULL;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    } else if (%s.len < %s) {", tempName(inst.ValueID), sourceLenExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      %s *%s = (%s *)realloc(%s.items, sizeof(%s) * (size_t)%s.len);", itemCType, itemsName, itemCType, tempName(inst.ValueID), itemCType, tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      if (%s != NULL) {", itemsName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("        %s.items = %s;", tempName(inst.ValueID), itemsName))
	s.emitter.writeLine(s.indent + "      }")
	s.emitter.writeLine(s.indent + "    }")
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinReversedCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("reversed() expects exactly 1 argument, got %d", len(call.Args))
	}
	if len(call.ReturnTypes) != 1 {
		return fmt.Errorf("reversed() expects exactly 1 return type, got %d", len(call.ReturnTypes))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("reversed() unexpectedly produced multi-return MIR")
	}
	listValue := s.body.Value(call.Args[0])
	if listValue == nil {
		return fmt.Errorf("unknown MIR reversed list value %d", call.Args[0])
	}
	listExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	listType := s.emitter.resolveType(listValue.Type)
	if isBareListType(listType) {
		sourceName := fmt.Sprintf("gwen_reversed_source_%d", inst.ValueID)
		s.emitter.writeLine(s.indent + "{")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_value %s = %s;", sourceName, listExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_value_list_from_ptr(gwen_dyn_list_new(%s.list_value->len));", tempName(inst.ValueID), sourceName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = 0; i < %s.list_value->len; ++i) {", sourceName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[i] = gwen_value_clone(%s.list_value->items[%s.list_value->len - 1LL - i]);", tempName(inst.ValueID), sourceName, sourceName))
		s.emitter.writeLine(s.indent + "  }")
		s.emitter.writeLine(s.indent + "}")
		return nil
	}
	itemType := listItemType(listType)
	if itemType == nil {
		return fmt.Errorf("unsupported reversed() target type %s", typeLabel(listValue.Type))
	}
	listTypeName, err := s.emitter.cType(listType)
	if err != nil {
		return err
	}
	itemCType, err := s.emitter.cType(itemType)
	if err != nil {
		return err
	}
	sourceName := fmt.Sprintf("gwen_reversed_source_%d", inst.ValueID)
	itemExpr, err := s.coerceExpr(sourceName+".items["+sourceName+".len - 1LL - i]", itemType, itemType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", listTypeName, sourceName, listExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = %s.len;", tempName(inst.ValueID), sourceName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s.len == 0) {", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = NULL;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = (%s *)malloc(sizeof(%s) * (size_t)%s.len);", tempName(inst.ValueID), itemCType, itemCType, tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory in reversed()\");", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    for (long long i = 0; i < %s.len; ++i) {", sourceName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items[i] = %s;", tempName(inst.ValueID), itemExpr))
	s.emitter.writeLine(s.indent + "    }")
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinRangeCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) < 2 || len(call.Args) > 3 {
		return fmt.Errorf("range() expects 2 or 3 arguments, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("range() unexpectedly produced multi-return MIR")
	}
	if len(call.ReturnTypes) != 1 {
		return fmt.Errorf("range() expects exactly 1 return type, got %d", len(call.ReturnTypes))
	}
	resultType := s.emitter.resolveType(call.ReturnTypes[0])
	itemType := listItemType(resultType)
	if itemType == nil || !isIntegerType(itemType) {
		return fmt.Errorf("unsupported range() return type %s", typeLabel(call.ReturnTypes[0]))
	}
	startExpr, err := s.exprForExpectedValue(call.Args[0], itemType)
	if err != nil {
		return err
	}
	endExpr, err := s.exprForExpectedValue(call.Args[1], itemType)
	if err != nil {
		return err
	}
	itemCType, err := s.emitter.cType(itemType)
	if err != nil {
		return err
	}
	startName := fmt.Sprintf("gwen_range_start_%d", inst.ValueID)
	endName := fmt.Sprintf("gwen_range_end_%d", inst.ValueID)
	stepName := fmt.Sprintf("gwen_range_step_%d", inst.ValueID)
	countName := fmt.Sprintf("gwen_range_count_%d", inst.ValueID)
	indexName := fmt.Sprintf("gwen_range_index_%d", inst.ValueID)
	currentName := fmt.Sprintf("gwen_range_current_%d", inst.ValueID)
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = (%s)(%s);", itemCType, startName, itemCType, startExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = (%s)(%s);", itemCType, endName, itemCType, endExpr))
	if len(call.Args) == 3 {
		stepExpr, err := s.exprForExpectedValue(call.Args[2], itemType)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = (%s)(%s);", itemCType, stepName, itemCType, stepExpr))
	} else {
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s <= %s ? 1 : -1;", itemCType, stepName, startName, endName))
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s == 0) gwen_runtime_error(\"runtime error: range() step cannot be 0\");", stepName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = 0;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.items = NULL;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  long long %s = 0;", countName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  for (%s %s = %s; ((%s > 0) && (%s <= %s)) || ((%s < 0) && (%s >= %s)); %s = (%s)(%s + %s)) {", itemCType, currentName, startName, stepName, currentName, endName, stepName, currentName, endName, currentName, itemCType, currentName, stepName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s++;", countName))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = %s;", tempName(inst.ValueID), countName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s > 0) {", countName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = (%s *)malloc(sizeof(%s) * (size_t)%s);", tempName(inst.ValueID), itemCType, itemCType, countName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory in range()\");", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    long long %s = 0;", indexName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    for (%s %s = %s; ((%s > 0) && (%s <= %s)) || ((%s < 0) && (%s >= %s)); %s = (%s)(%s + %s)) {", itemCType, currentName, startName, stepName, currentName, endName, stepName, currentName, endName, currentName, itemCType, currentName, stepName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items[%s++] = %s;", tempName(inst.ValueID), indexName, currentName))
	s.emitter.writeLine(s.indent + "    }")
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinEnumerateCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("enumerate() expects exactly 1 argument, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("enumerate() unexpectedly produced multi-return MIR")
	}
	if len(call.ReturnTypes) != 1 {
		return fmt.Errorf("enumerate() expects exactly 1 return type, got %d", len(call.ReturnTypes))
	}
	resultType := s.emitter.resolveType(call.ReturnTypes[0])
	resultIsBare := isBareListType(resultType)
	resultItemType := listItemType(resultType)
	resultTypedPairs := resultItemType != nil && isBareListType(s.emitter.resolveType(resultItemType))
	if !resultIsBare && !resultTypedPairs {
		return fmt.Errorf("compiled enumerate() currently returns bare list or list[list], got %s", typeLabel(call.ReturnTypes[0]))
	}
	listValue := s.body.Value(call.Args[0])
	if listValue == nil {
		return fmt.Errorf("unknown MIR enumerate list value %d", call.Args[0])
	}
	listExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	sourceType := s.emitter.resolveType(listValue.Type)
	sourceName := fmt.Sprintf("gwen_enumerate_source_%d", inst.ValueID)
	pairName := fmt.Sprintf("gwen_enumerate_pair_%d", inst.ValueID)
	pairListTypeName := ""
	if resultTypedPairs {
		pairListTypeName, err = s.emitter.cType(resultItemType)
		if err != nil {
			return err
		}
	}
	itemExpr := ""
	switch {
	case isBareListType(sourceType):
		s.emitter.writeLine(s.indent + "{")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_value %s = %s;", sourceName, listExpr))
		if resultIsBare {
			s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_value_list_from_ptr(gwen_dyn_list_new(%s.list_value->len));", tempName(inst.ValueID), sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = 0; i < %s.list_value->len; ++i) {", sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    gwen_value %s = gwen_value_list_from_ptr(gwen_dyn_list_new(2));", pairName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[0] = gwen_value_int(i);", pairName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[1] = gwen_value_clone(%s.list_value->items[i]);", pairName, sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[i] = %s;", tempName(inst.ValueID), pairName))
			s.emitter.writeLine(s.indent + "  }")
		} else {
			s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = %s.list_value->len;", tempName(inst.ValueID), sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s.len == 0) {", tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = NULL;", tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + "  } else {")
			s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = (%s *)malloc(sizeof(%s) * (size_t)%s.len);", tempName(inst.ValueID), pairListTypeName, pairListTypeName, tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory in enumerate()\");", tempName(inst.ValueID)))
			s.emitter.writeLine(s.indent + fmt.Sprintf("    for (long long i = 0; i < %s.list_value->len; ++i) {", sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      gwen_value %s = gwen_value_list_from_ptr(gwen_dyn_list_new(2));", pairName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.list_value->items[0] = gwen_value_int(i);", pairName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.list_value->items[1] = gwen_value_clone(%s.list_value->items[i]);", pairName, sourceName))
			s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items[i] = %s;", tempName(inst.ValueID), pairName))
			s.emitter.writeLine(s.indent + "    }")
			s.emitter.writeLine(s.indent + "  }")
		}
		s.emitter.writeLine(s.indent + "}")
		return nil
	case listItemType(sourceType) != nil:
		itemExpr, err = s.emitter.dynamicValueExpr(sourceName+".items[i]", listItemType(sourceType))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("compiled enumerate() currently requires list input, got %s", typeLabel(listValue.Type))
	}
	listTypeName, err := s.emitter.cType(sourceType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", listTypeName, sourceName, listExpr))
	if resultIsBare {
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_value_list_from_ptr(gwen_dyn_list_new(%s.len));", tempName(inst.ValueID), sourceName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = 0; i < %s.len; ++i) {", sourceName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    gwen_value %s = gwen_value_list_from_ptr(gwen_dyn_list_new(2));", pairName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[0] = gwen_value_int(i);", pairName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[1] = %s;", pairName, itemExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[i] = %s;", tempName(inst.ValueID), pairName))
		s.emitter.writeLine(s.indent + "  }")
	} else {
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = %s.len;", tempName(inst.ValueID), sourceName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s.len == 0) {", tempName(inst.ValueID)))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = NULL;", tempName(inst.ValueID)))
		s.emitter.writeLine(s.indent + "  } else {")
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = (%s *)malloc(sizeof(%s) * (size_t)%s.len);", tempName(inst.ValueID), pairListTypeName, pairListTypeName, tempName(inst.ValueID)))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory in enumerate()\");", tempName(inst.ValueID)))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    for (long long i = 0; i < %s.len; ++i) {", sourceName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("      gwen_value %s = gwen_value_list_from_ptr(gwen_dyn_list_new(2));", pairName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.list_value->items[0] = gwen_value_int(i);", pairName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.list_value->items[1] = %s;", pairName, itemExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items[i] = %s;", tempName(inst.ValueID), pairName))
		s.emitter.writeLine(s.indent + "    }")
		s.emitter.writeLine(s.indent + "  }")
	}
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinConcatCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("concat() expects exactly 2 arguments, got %d", len(call.Args))
	}
	leftValue := s.body.Value(call.Args[0])
	if leftValue == nil {
		return fmt.Errorf("unknown MIR concat left value %d", call.Args[0])
	}
	rightValue := s.body.Value(call.Args[1])
	if rightValue == nil {
		return fmt.Errorf("unknown MIR concat right value %d", call.Args[1])
	}
	leftExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	rightExpr, err := s.exprForValue(call.Args[1])
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("concat() unexpectedly produced multi-return MIR")
	}
	leftType := s.emitter.resolveType(leftValue.Type)
	if isBareListType(leftType) {
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_list_concat(%s, %s);", tempName(inst.ValueID), leftExpr, rightExpr))
		return nil
	}
	leftItemType := listItemType(leftType)
	rightType := s.emitter.resolveType(rightValue.Type)
	rightItemType := listItemType(rightType)
	if leftItemType == nil || rightItemType == nil || typeLabel(s.emitter.resolveType(leftItemType)) != typeLabel(s.emitter.resolveType(rightItemType)) {
		return fmt.Errorf("unsupported concat() target type %s", typeLabel(leftValue.Type))
	}
	listTypeName, err := s.emitter.cType(leftType)
	if err != nil {
		return err
	}
	itemCType, err := s.emitter.cType(leftItemType)
	if err != nil {
		return err
	}
	resultItemExpr := func(expr string) (string, error) {
		return s.coerceExpr(expr, leftItemType, leftItemType)
	}
	leftItemExpr, err := resultItemExpr("gwen_concat_left.items[i]")
	if err != nil {
		return err
	}
	rightItemExpr, err := s.coerceExpr("gwen_concat_right.items[i]", rightItemType, leftItemType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s gwen_concat_left = %s;", listTypeName, leftExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s gwen_concat_right = %s;", listTypeName, rightExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.len = gwen_concat_left.len + gwen_concat_right.len;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s.len == 0) {", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = NULL;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.items = (%s *)malloc(sizeof(%s) * (size_t)%s.len);", tempName(inst.ValueID), itemCType, itemCType, tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory in concat()\");", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + "    for (long long i = 0; i < gwen_concat_left.len; ++i) {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items[i] = %s;", tempName(inst.ValueID), leftItemExpr))
	s.emitter.writeLine(s.indent + "    }")
	s.emitter.writeLine(s.indent + "    for (long long i = 0; i < gwen_concat_right.len; ++i) {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("      %s.items[gwen_concat_left.len + i] = %s;", tempName(inst.ValueID), rightItemExpr))
	s.emitter.writeLine(s.indent + "    }")
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinSortCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("sort() expects exactly 2 arguments, got %d", len(call.Args))
	}
	listExpr, helperName, err := s.listSortTarget(call.Args[0], call.Args[1])
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("sort() unexpectedly produced multi-return MIR")
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s);", tempName(inst.ValueID), helperName, listExpr))
	return nil
}

func (s *bodyState) emitBuiltinJSONObjectCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args)%2 != 0 {
		return fmt.Errorf("json.objectof() expects even number of arguments, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("json.objectof() unexpectedly produced multi-return MIR")
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_dict_from_ptr(gwen_dyn_dict_new(%dLL));", tempName(inst.ValueID), len(call.Args)/2))
	for idx := 0; idx < len(call.Args); idx += 2 {
		keyExpr, err := s.exprForExpectedValue(call.Args[idx], &hir.NamedType{Name: "string"})
		if err != nil {
			return err
		}
		valueExpr, err := s.dynamicExprForValue(call.Args[idx+1])
		if err != nil {
			return err
		}
		itemIdx := idx / 2
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.dict_value->keys[%d] = gwen_value_string(%s);", tempName(inst.ValueID), itemIdx, keyExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.dict_value->values[%d] = %s;", tempName(inst.ValueID), itemIdx, valueExpr))
	}
	return nil
}

func (s *bodyState) emitBuiltinJSONArrayCall(call *mir.Value, inst *mir.CallInst) error {
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("json.arrayof() unexpectedly produced multi-return MIR")
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_list_from_ptr(gwen_dyn_list_new(%dLL));", tempName(inst.ValueID), len(call.Args)))
	for idx, argID := range call.Args {
		valueExpr, err := s.dynamicExprForValue(argID)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("%s.list_value->items[%d] = %s;", tempName(inst.ValueID), idx, valueExpr))
	}
	return nil
}

func (s *bodyState) emitBuiltinJSONNullCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 0 {
		return fmt.Errorf("json.null() expects exactly 0 arguments, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("json.null() unexpectedly produced multi-return MIR")
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_null();", tempName(inst.ValueID)))
	return nil
}

func (s *bodyState) emitBuiltinJSONIsNullCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("json.isnull() expects exactly 1 argument, got %d", len(call.Args))
	}
	argExpr, err := s.dynamicExprForValue(call.Args[0])
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("json.isnull() unexpectedly produced multi-return MIR")
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_value_is_null(%s);", tempName(inst.ValueID), argExpr))
	return nil
}

func (s *bodyState) emitBuiltinJSONParseObjectCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("json.parseobject() expects exactly 1 argument, got %d", len(call.Args))
	}
	textExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	resultType := call.ReturnTypes[0]
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported json.parseobject() result type %s", typeLabel(resultType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  gwen_value gwen_json_value = gwen_value_null();")
	s.emitter.writeLine(s.indent + "  const char *gwen_json_error = NULL;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (gwen_json_parse_root(%s, &gwen_json_value, &gwen_json_error) && gwen_json_value.kind == GWEN_VALUE_DICT) {", textExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, gwen_json_value, NULL};", tempName(inst.ValueID), typeName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_json_error != NULL ? gwen_json_error : \"json.parseobject() requires top-level object\"};", tempName(inst.ValueID), typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinJSONParseArrayCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("json.parsearray() expects exactly 1 argument, got %d", len(call.Args))
	}
	textExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	resultType := call.ReturnTypes[0]
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported json.parsearray() result type %s", typeLabel(resultType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  gwen_value gwen_json_value = gwen_value_null();")
	s.emitter.writeLine(s.indent + "  const char *gwen_json_error = NULL;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (gwen_json_parse_root(%s, &gwen_json_value, &gwen_json_error) && gwen_json_value.kind == GWEN_VALUE_LIST) {", textExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, gwen_json_value, NULL};", tempName(inst.ValueID), typeName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_json_error != NULL ? gwen_json_error : \"json.parsearray() requires top-level array\"};", tempName(inst.ValueID), typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinJSONStringifyCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("json.stringify() expects exactly 1 argument, got %d", len(call.Args))
	}
	valueExpr, err := s.dynamicExprForValue(call.Args[0])
	if err != nil {
		return err
	}
	resultType := call.ReturnTypes[0]
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported json.stringify() result type %s", typeLabel(resultType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  const char *gwen_json_text = NULL;")
	s.emitter.writeLine(s.indent + "  const char *gwen_json_error = NULL;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (gwen_json_stringify_value(%s, &gwen_json_text, &gwen_json_error)) {", valueExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, gwen_json_text, NULL};", tempName(inst.ValueID), typeName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_json_error != NULL ? gwen_json_error : \"json.stringify() failed\"};", tempName(inst.ValueID), typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinHTTPGetCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 && len(call.Args) != 2 {
		return fmt.Errorf("http.get() expects 1 or 2 arguments, got %d", len(call.Args))
	}
	urlExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	timeoutExpr := "5000LL"
	if len(call.Args) == 2 {
		expr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "int"})
		if err != nil {
			return err
		}
		timeoutExpr = fmt.Sprintf("(long long)(%s)", expr)
	}
	resultType := call.ReturnTypes[0]
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported http.get() result type %s", typeLabel(resultType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  long long gwen_http_timeout_ms = %s;", timeoutExpr))
	s.emitter.writeLine(s.indent + "  gwen_http_response gwen_http_response_value = (gwen_http_response){0};")
	s.emitter.writeLine(s.indent + "  const char *gwen_http_error = NULL;")
	s.emitter.writeLine(s.indent + "  if (gwen_http_timeout_ms < 0) gwen_runtime_error(\"runtime error: http.get() timeoutms must be >= 0\");")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (gwen_http_client_request(\"GET\", %s, \"\", gwen_string_pairs_empty(), gwen_http_timeout_ms, &gwen_http_response_value, &gwen_http_error)) {", urlExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, gwen_http_response_value, NULL};", tempName(inst.ValueID), typeName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_http_error != NULL ? gwen_http_error : \"http.get() failed\"};", tempName(inst.ValueID), typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinHTTPRequestCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 4 && len(call.Args) != 5 {
		return fmt.Errorf("http.request() expects 4 or 5 arguments, got %d", len(call.Args))
	}
	methodExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	urlExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	bodyExpr, err := s.exprForExpectedValue(call.Args[2], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	headerType := &hir.GenericType{Base: "dict", Args: []hir.Type{&hir.NamedType{Name: "string"}, &hir.NamedType{Name: "string"}}}
	headersExpr, err := s.exprForExpectedValue(call.Args[3], headerType)
	if err != nil {
		return err
	}
	timeoutExpr := "5000LL"
	if len(call.Args) == 5 {
		expr, err := s.exprForExpectedValue(call.Args[4], &hir.NamedType{Name: "int"})
		if err != nil {
			return err
		}
		timeoutExpr = fmt.Sprintf("(long long)(%s)", expr)
	}
	resultType := call.ReturnTypes[0]
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported http.request() result type %s", typeLabel(resultType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  long long gwen_http_timeout_ms = %s;", timeoutExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_string_pairs gwen_http_headers = (gwen_string_pairs){(%s).len, (%s).keys, (%s).values};", headersExpr, headersExpr, headersExpr))
	s.emitter.writeLine(s.indent + "  gwen_http_response gwen_http_response_value = (gwen_http_response){0};")
	s.emitter.writeLine(s.indent + "  const char *gwen_http_error = NULL;")
	s.emitter.writeLine(s.indent + "  if (gwen_http_timeout_ms < 0) gwen_runtime_error(\"runtime error: http.request() timeoutms must be >= 0\");")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (gwen_http_client_request(%s, %s, %s, gwen_http_headers, gwen_http_timeout_ms, &gwen_http_response_value, &gwen_http_error)) {", methodExpr, urlExpr, bodyExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, gwen_http_response_value, NULL};", tempName(inst.ValueID), typeName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_http_error != NULL ? gwen_http_error : \"http.request() failed\"};", tempName(inst.ValueID), typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinHTTPRequestFieldCall(callName string, call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("http.%s() expects exactly 1 argument, got %d", callName, len(call.Args))
	}
	requestExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpRequest"})
	if err != nil {
		return err
	}
	field := ""
	switch callName {
	case "method":
		field = "method"
	case "path":
		field = "path"
	case "requestbody":
		field = "body"
	default:
		return fmt.Errorf("unsupported http request field builtin %q", callName)
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s).%s;", tempName(inst.ValueID), requestExpr, field))
	return nil
}

func (s *bodyState) emitBuiltinHTTPServerAddrCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("http.addr() expects exactly 1 argument, got %d", len(call.Args))
	}
	serverExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpServer"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s).addr;", tempName(inst.ValueID), serverExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPQueryCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 3 {
		return fmt.Errorf("http.query() expects exactly 3 arguments, got %d", len(call.Args))
	}
	requestExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpRequest"})
	if err != nil {
		return err
	}
	keyExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	fallbackExpr, err := s.exprForExpectedValue(call.Args[2], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_string_pairs_get((%s).query, %s, %s, false);", tempName(inst.ValueID), requestExpr, keyExpr, fallbackExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPRequestHeaderCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 3 {
		return fmt.Errorf("http.requestheader() expects exactly 3 arguments, got %d", len(call.Args))
	}
	requestExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpRequest"})
	if err != nil {
		return err
	}
	keyExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	fallbackExpr, err := s.exprForExpectedValue(call.Args[2], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_string_pairs_get((%s).headers, %s, %s, true);", tempName(inst.ValueID), requestExpr, keyExpr, fallbackExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPRequestCookieCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 3 {
		return fmt.Errorf("http.requestcookie() expects exactly 3 arguments, got %d", len(call.Args))
	}
	requestExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpRequest"})
	if err != nil {
		return err
	}
	keyExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	fallbackExpr, err := s.exprForExpectedValue(call.Args[2], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_string_pairs_get((%s).cookies, %s, %s, false);", tempName(inst.ValueID), requestExpr, keyExpr, fallbackExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPResponseFieldCall(callName string, call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("http.%s() expects exactly 1 argument, got %d", callName, len(call.Args))
	}
	responseExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpResponse"})
	if err != nil {
		return err
	}
	field := ""
	switch callName {
	case "status":
		field = "status"
	case "responsebody":
		field = "body"
	default:
		return fmt.Errorf("unsupported http response field builtin %q", callName)
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s).%s;", tempName(inst.ValueID), responseExpr, field))
	return nil
}

func (s *bodyState) emitBuiltinHTTPResponseHeaderCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 3 {
		return fmt.Errorf("http.responseheader() expects exactly 3 arguments, got %d", len(call.Args))
	}
	responseExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpResponse"})
	if err != nil {
		return err
	}
	keyExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	fallbackExpr, err := s.exprForExpectedValue(call.Args[2], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_string_pairs_get((%s).headers, %s, %s, true);", tempName(inst.ValueID), responseExpr, keyExpr, fallbackExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPTextLikeCall(call *mir.Value, inst *mir.CallInst, contentType string) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("http text-like builtin expects exactly 2 arguments, got %d", len(call.Args))
	}
	statusExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "int"})
	if err != nil {
		return err
	}
	bodyExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_http_reply_new((long long)(%s), %s, %s);", tempName(inst.ValueID), statusExpr, quoteCString(contentType), bodyExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPRedirectCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("http.redirect() expects exactly 2 arguments, got %d", len(call.Args))
	}
	statusExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "int"})
	if err != nil {
		return err
	}
	locationExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_http_with_header(gwen_http_reply_new((long long)(%s), \"\", \"\"), \"Location\", %s);", tempName(inst.ValueID), statusExpr, locationExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPWithHeaderCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 3 {
		return fmt.Errorf("http.withheader() expects exactly 3 arguments, got %d", len(call.Args))
	}
	replyExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpReply"})
	if err != nil {
		return err
	}
	keyExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	valueExpr, err := s.exprForExpectedValue(call.Args[2], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_http_with_header(%s, %s, %s);", tempName(inst.ValueID), replyExpr, keyExpr, valueExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPWithCookieCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 3 {
		return fmt.Errorf("http.withcookie() expects exactly 3 arguments, got %d", len(call.Args))
	}
	replyExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpReply"})
	if err != nil {
		return err
	}
	nameExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	valueExpr, err := s.exprForExpectedValue(call.Args[2], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_http_with_cookie(%s, %s, %s);", tempName(inst.ValueID), replyExpr, nameExpr, valueExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPJSONCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("http.json() expects exactly 2 arguments, got %d", len(call.Args))
	}
	statusExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "int"})
	if err != nil {
		return err
	}
	valueExpr, err := s.dynamicExprForValue(call.Args[1])
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = gwen_http_json_reply((long long)(%s), %s);", tempName(inst.ValueID), statusExpr, valueExpr))
	return nil
}

func (s *bodyState) emitBuiltinHTTPRouteCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("http.route() expects exactly 2 arguments, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 2 {
		return fmt.Errorf("http.route() expects 2 result ids, got %d", len(inst.ResultIDs))
	}
	requestExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpRequest"})
	if err != nil {
		return err
	}
	patternExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	dictValue := s.body.Value(inst.ResultIDs[1])
	if dictValue == nil {
		return fmt.Errorf("missing http.route() params result value")
	}
	dictType, err := s.emitter.cType(dictValue.Type)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  gwen_string_pairs gwen_route_params = gwen_string_pairs_empty();")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_http_route_pairs((%s).path, %s, &gwen_route_params);", tempName(inst.ResultIDs[0]), requestExpr, patternExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (%s){gwen_route_params.len, gwen_route_params.keys, gwen_route_params.values};", tempName(inst.ResultIDs[1]), dictType))
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinHTTPStaticCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 3 {
		return fmt.Errorf("http.static() expects exactly 3 arguments, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 2 {
		return fmt.Errorf("http.static() expects 2 result ids, got %d", len(inst.ResultIDs))
	}
	requestExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpRequest"})
	if err != nil {
		return err
	}
	prefixExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	rootExpr, err := s.exprForExpectedValue(call.Args[2], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  bool gwen_static_matched = false;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_http_static_reply(%s, %s, %s, &gwen_static_matched);", tempName(inst.ResultIDs[1]), requestExpr, prefixExpr, rootExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_static_matched;", tempName(inst.ResultIDs[0])))
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinHTTPListenCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("http.listen() expects exactly 2 arguments, got %d", len(call.Args))
	}
	addrExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	handlerValue := s.body.Value(call.Args[1])
	if handlerValue == nil {
		return fmt.Errorf("missing http.listen() handler")
	}
	handlerExpr, err := s.exprForValue(call.Args[1])
	if err != nil {
		return err
	}
	handlerType, ok := s.emitter.resolveType(handlerValue.Type).(*hir.FuncType)
	if !ok || len(handlerType.Returns) != 1 {
		return fmt.Errorf("unsupported http.listen() handler type %s", typeLabel(handlerValue.Type))
	}
	handlerTypeName, err := s.emitter.cType(handlerValue.Type)
	if err != nil {
		return err
	}
	handlerVar := fmt.Sprintf("gwen_http_handler_%d", inst.ValueID)
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", handlerTypeName, handlerVar, handlerExpr))
	switch {
	case namedTypeName(handlerType.Returns[0]) == "HttpReply":
		if err := s.emitHTTPServerOpenResult(tempName(inst.ValueID), call.ReturnTypes[0], addrExpr, "2", "NULL", "NULL", handlerVar+".env", fmt.Sprintf("(gwen_http_direct_handler_fn)(%s.call)", handlerVar)); err != nil {
			return err
		}
	case isHTTPReplyResultType(handlerType.Returns[0]):
		if err := s.emitHTTPServerOpenResult(tempName(inst.ValueID), call.ReturnTypes[0], addrExpr, "1", handlerVar+".env", fmt.Sprintf("(gwen_http_handler_fn)(%s.call)", handlerVar), "NULL", "NULL"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported http.listen() handler return type %s", typeLabel(handlerType.Returns[0]))
	}
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinHTTPWaitCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("http.wait() expects exactly 1 argument, got %d", len(call.Args))
	}
	serverExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpServer"})
	if err != nil {
		return err
	}
	return s.emitHTTPServerIntResult(tempName(inst.ValueID), call.ReturnTypes[0], fmt.Sprintf("gwen_http_server_wait_loop(%s, &gwen_http_error)", serverExpr))
}

func (s *bodyState) emitBuiltinHTTPCloseCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("http.close() expects exactly 1 argument, got %d", len(call.Args))
	}
	serverExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "HttpServer"})
	if err != nil {
		return err
	}
	return s.emitHTTPServerIntResult(tempName(inst.ValueID), call.ReturnTypes[0], fmt.Sprintf("gwen_http_server_close_fd(%s, &gwen_http_error)", serverExpr))
}

func (s *bodyState) emitBuiltinStateCellCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("state.cell() expects exactly 1 argument, got %d", len(call.Args))
	}
	if len(call.ReturnTypes) != 1 {
		return fmt.Errorf("state.cell() expects exactly 1 return type, got %d", len(call.ReturnTypes))
	}
	cellType := s.emitter.resolveType(call.ReturnTypes[0])
	itemType := cellItemType(cellType)
	if itemType == nil {
		return fmt.Errorf("unsupported state.cell() return type %s", typeLabel(call.ReturnTypes[0]))
	}
	cellExpr, err := s.emitter.cType(cellType)
	if err != nil {
		return err
	}
	valueExpr, err := s.exprForExpectedValue(call.Args[0], itemType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s);", tempName(inst.ValueID), cellNewFuncName(cellExpr), valueExpr))
	return nil
}

func (s *bodyState) emitBuiltinStateGetCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("state.get() expects exactly 1 argument, got %d", len(call.Args))
	}
	cellValue := s.body.Value(call.Args[0])
	if cellValue == nil {
		return fmt.Errorf("unknown MIR state.get() cell value %d", call.Args[0])
	}
	cellExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	cellType, err := s.emitter.cType(cellValue.Type)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s);", tempName(inst.ValueID), cellGetFuncName(cellType), cellExpr))
	return nil
}

func (s *bodyState) emitBuiltinStateSetCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("state.set() expects exactly 2 arguments, got %d", len(call.Args))
	}
	cellValue := s.body.Value(call.Args[0])
	if cellValue == nil {
		return fmt.Errorf("unknown MIR state.set() cell value %d", call.Args[0])
	}
	itemType := cellItemType(s.emitter.resolveType(cellValue.Type))
	if itemType == nil {
		return fmt.Errorf("unsupported state.set() cell type %s", typeLabel(cellValue.Type))
	}
	cellExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	cellType, err := s.emitter.cType(cellValue.Type)
	if err != nil {
		return err
	}
	valueExpr, err := s.exprForExpectedValue(call.Args[1], itemType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s, %s);", tempName(inst.ValueID), cellSetFuncName(cellType), cellExpr, valueExpr))
	return nil
}

func (s *bodyState) emitBuiltinStateUpdateCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("state.update() expects exactly 2 arguments, got %d", len(call.Args))
	}
	cellValue := s.body.Value(call.Args[0])
	if cellValue == nil {
		return fmt.Errorf("unknown MIR state.update() cell value %d", call.Args[0])
	}
	itemType := cellItemType(s.emitter.resolveType(cellValue.Type))
	if itemType == nil {
		return fmt.Errorf("unsupported state.update() cell type %s", typeLabel(cellValue.Type))
	}
	callbackValue := s.body.Value(call.Args[1])
	if callbackValue == nil {
		return fmt.Errorf("unknown MIR state.update() callback value %d", call.Args[1])
	}
	callbackType, ok := s.emitter.resolveType(callbackValue.Type).(*hir.FuncType)
	if !ok || len(callbackType.Params) != 1 || len(callbackType.Returns) != 1 {
		return fmt.Errorf("unsupported state.update() callback type %s", typeLabel(callbackValue.Type))
	}
	itemLabel := typeLabel(s.emitter.resolveType(itemType))
	if typeLabel(s.emitter.resolveType(callbackType.Params[0])) != itemLabel || typeLabel(s.emitter.resolveType(callbackType.Returns[0])) != itemLabel {
		return fmt.Errorf("state.update() callback must be (%s) -> %s, got %s", itemLabel, itemLabel, typeLabel(callbackValue.Type))
	}
	cellExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	cellType, err := s.emitter.cType(cellValue.Type)
	if err != nil {
		return err
	}
	callbackExpr, err := s.exprForValue(call.Args[1])
	if err != nil {
		return err
	}
	itemCType, err := s.emitter.cType(itemType)
	if err != nil {
		return err
	}
	cellName := fmt.Sprintf("gwen_state_cell_%d", inst.ValueID)
	currentName := fmt.Sprintf("gwen_state_current_%d", inst.ValueID)
	nextName := fmt.Sprintf("gwen_state_next_%d", inst.ValueID)
	storedName := fmt.Sprintf("gwen_state_stored_%d", inst.ValueID)
	frameName := fmt.Sprintf("gwen_state_frame_%d", inst.ValueID)
	updatedExpr, err := s.funcInvokeExpr(callbackType, callbackExpr, []string{currentName})
	if err != nil {
		return err
	}
	currentCloneExpr, err := s.emitter.cloneExpr("*(("+cellName+").value)", itemType)
	if err != nil {
		return err
	}
	storedCloneExpr, err := s.emitter.cloneExpr(nextName, itemType)
	if err != nil {
		return err
	}
	resultCloneExpr, err := s.emitter.cloneExpr(storedName, itemType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", cellType, cellName, cellExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_error_frame %s;", frameName))
	s.emitter.writeLine(s.indent + "  bool gwen_state_locked = false;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s.mutex == NULL || %s.value == NULL) gwen_runtime_error(\"runtime error: state.update() requires initialized cell\");", cellName, cellName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.prev = gwen_error_frame_current;", frameName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.message = NULL;", frameName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_error_frame_current = &%s;", frameName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (setjmp(%s.jump) != 0) {", frameName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    gwen_error_frame_current = %s.prev;", frameName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    if (gwen_state_locked) gwen_pthread_require(pthread_mutex_unlock(%s.mutex), \"unlock state cell\");", cellName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    gwen_runtime_error(%s.message);", frameName))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_pthread_require(pthread_mutex_lock(%s.mutex), \"lock state cell\");", cellName))
	s.emitter.writeLine(s.indent + "  gwen_state_locked = true;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", itemCType, currentName, currentCloneExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", itemCType, nextName, updatedExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s %s = %s;", itemCType, storedName, storedCloneExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  *(%s.value) = %s;", cellName, storedName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_pthread_require(pthread_mutex_unlock(%s.mutex), \"unlock state cell\");", cellName))
	s.emitter.writeLine(s.indent + "  gwen_state_locked = false;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  gwen_error_frame_current = %s.prev;", frameName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = %s;", tempName(inst.ValueID), resultCloneExpr))
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinOSArgsCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 0 {
		return fmt.Errorf("os.args() expects exactly 0 arguments, got %d", len(call.Args))
	}
	listType := ""
	var err error
	if len(call.ReturnTypes) == 0 {
		return fmt.Errorf("os.args() is missing return type")
	}
	listType, err = s.emitter.cType(call.ReturnTypes[0])
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s.len = gwen_cli_argc > 1 ? (long long)(gwen_cli_argc - 1) : 0LL;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s.items = NULL;", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s.len > 0) {", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s.items = (const char **)malloc(sizeof(const char *) * (size_t)%s.len);", tempName(inst.ValueID), tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s.items == NULL) gwen_runtime_error(\"runtime error: out of memory allocating %s\");", tempName(inst.ValueID), listType))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = 1; i < (long long)gwen_cli_argc; ++i) %s.items[i - 1] = gwen_cli_argv[i];", tempName(inst.ValueID)))
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinSQLiteOpenCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("sqlite.open() expects exactly 1 argument, got %d", len(call.Args))
	}
	pathExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	resultType := call.ReturnTypes[0]
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported sqlite.open() result type %s", typeLabel(resultType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  gwen_sqlite_db gwen_sqlite_db_value = (gwen_sqlite_db){0};")
	s.emitter.writeLine(s.indent + "  const char *gwen_sqlite_error = NULL;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (gwen_sqlite_open_path(%s, &gwen_sqlite_error)) {", pathExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    gwen_sqlite_db_value.path = gwen_string_dup(%s);", pathExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, gwen_sqlite_db_value, NULL};", tempName(inst.ValueID), typeName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_sqlite_error != NULL ? gwen_sqlite_error : \"sqlite.open() failed\"};", tempName(inst.ValueID), typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinSQLiteCloseCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("sqlite.close() expects exactly 1 argument, got %d", len(call.Args))
	}
	if _, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "SqliteDB"}); err != nil {
		return err
	}
	return s.emitResultFromBool(tempName(inst.ValueID), call.ReturnTypes[0], "true", "0LL")
}

func (s *bodyState) emitBuiltinSQLiteExecCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 && len(call.Args) != 3 {
		return fmt.Errorf("sqlite.exec() expects 2 or 3 arguments, got %d", len(call.Args))
	}
	dbExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "SqliteDB"})
	if err != nil {
		return err
	}
	sqlExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	resultType := call.ReturnTypes[0]
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported sqlite.exec() result type %s", typeLabel(resultType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  gwen_value gwen_sqlite_params = gwen_value_list_from_ptr(gwen_dyn_list_new(0));")
	if len(call.Args) == 3 {
		if err := s.emitSQLiteParamsValue(call.Args[2], "gwen_sqlite_params"); err != nil {
			return err
		}
	}
	s.emitter.writeLine(s.indent + "  long long gwen_sqlite_changes = 0LL;")
	s.emitter.writeLine(s.indent + "  const char *gwen_sqlite_error = NULL;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (gwen_sqlite_exec_path((%s).path, %s, gwen_sqlite_params, &gwen_sqlite_changes, &gwen_sqlite_error)) {", dbExpr, sqlExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, gwen_sqlite_changes, NULL};", tempName(inst.ValueID), typeName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_sqlite_error != NULL ? gwen_sqlite_error : \"sqlite.exec() failed\"};", tempName(inst.ValueID), typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinSQLiteQueryCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 && len(call.Args) != 3 {
		return fmt.Errorf("sqlite.query() expects 2 or 3 arguments, got %d", len(call.Args))
	}
	dbExpr, err := s.exprForExpectedValue(call.Args[0], &hir.NamedType{Name: "SqliteDB"})
	if err != nil {
		return err
	}
	sqlExpr, err := s.exprForExpectedValue(call.Args[1], &hir.NamedType{Name: "string"})
	if err != nil {
		return err
	}
	resultType := call.ReturnTypes[0]
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported sqlite.query() result type %s", typeLabel(resultType))
	}
	listItem := listItemType(okType)
	if listItem == nil || !isDynamicValueType(s.emitter.resolveType(listItem)) {
		return fmt.Errorf("unsupported sqlite.query() ok payload type %s", typeLabel(okType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	listType, err := s.emitter.cType(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  gwen_value gwen_sqlite_params = gwen_value_list_from_ptr(gwen_dyn_list_new(0));")
	if len(call.Args) == 3 {
		if err := s.emitSQLiteParamsValue(call.Args[2], "gwen_sqlite_params"); err != nil {
			return err
		}
	}
	s.emitter.writeLine(s.indent + "  gwen_value gwen_sqlite_rows = gwen_value_null();")
	s.emitter.writeLine(s.indent + "  const char *gwen_sqlite_error = NULL;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (gwen_sqlite_query_path((%s).path, %s, gwen_sqlite_params, &gwen_sqlite_rows, &gwen_sqlite_error)) {", dbExpr, sqlExpr))
	s.emitter.writeLine(s.indent + "    gwen_dyn_list *gwen_sqlite_rows_list = gwen_value_as_list(gwen_sqlite_rows);")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, (%s){gwen_sqlite_rows_list->len, gwen_sqlite_rows_list->items}, NULL};", tempName(inst.ValueID), typeName, listType))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_sqlite_error != NULL ? gwen_sqlite_error : \"sqlite.query() failed\"};", tempName(inst.ValueID), typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitHTTPServerIntResult(targetTemp string, resultType hir.Type, okExpr string) error {
	return s.emitResultFromBool(targetTemp, resultType, okExpr, "0LL")
}

func (s *bodyState) emitHTTPServerOpenResult(targetTemp string, resultType hir.Type, addrExpr string, handlerKind string, resultHandlerEnvExpr string, resultHandlerExpr string, directHandlerEnvExpr string, directHandlerExpr string) error {
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported listen() result type %s", typeLabel(resultType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  gwen_http_server gwen_http_server_value = (gwen_http_server){0};")
	s.emitter.writeLine(s.indent + "  const char *gwen_http_error = NULL;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (gwen_http_server_open(&gwen_http_server_value, %s, %s, %s, %s, %s, %s, &gwen_http_error)) {", addrExpr, handlerKind, resultHandlerEnvExpr, resultHandlerExpr, directHandlerEnvExpr, directHandlerExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, gwen_http_server_value, NULL};", targetTemp, typeName))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_http_error != NULL ? gwen_http_error : \"listen failed\"};", targetTemp, typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitResultFromBool(targetTemp string, resultType hir.Type, okExpr string, okPayloadExpr string) error {
	typeName, err := s.emitter.cType(resultType)
	if err != nil {
		return err
	}
	okType := resultOKType(resultType)
	if okType == nil || resultErrType(resultType) == nil {
		return fmt.Errorf("unsupported result type %s", typeLabel(resultType))
	}
	zeroOK, err := s.emitter.zeroExpr(okType)
	if err != nil {
		return err
	}
	s.emitter.writeLine(s.indent + "{")
	s.emitter.writeLine(s.indent + "  const char *gwen_http_error = NULL;")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s) {", okExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){true, %s, NULL};", targetTemp, typeName, okPayloadExpr))
	s.emitter.writeLine(s.indent + "  } else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = (%s){false, %s, gwen_http_error != NULL ? gwen_http_error : \"http operation failed\"};", targetTemp, typeName, zeroOK))
	s.emitter.writeLine(s.indent + "  }")
	s.emitter.writeLine(s.indent + "}")
	return nil
}

func (s *bodyState) emitBuiltinCastCall(call *mir.Value, builtinName string, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("%s() expects exactly 1 argument, got %d", builtinName, len(call.Args))
	}
	targetExpr, err := s.builtinCastExpr(builtinName, call.Args[0])
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("%s() unexpectedly produced multi-return MIR", builtinName)
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", tempName(inst.ValueID), targetExpr))
	return nil
}

func (s *bodyState) emitBuiltinTypeofCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("typeof() expects exactly 1 argument, got %d", len(call.Args))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("typeof() unexpectedly produced multi-return MIR")
	}
	arg := s.body.Value(call.Args[0])
	if arg == nil {
		return fmt.Errorf("unknown MIR typeof arg value %d", call.Args[0])
	}
	argExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	resolved := s.emitter.resolveType(arg.Type)
	resultExpr := quoteCString(typeLabel(resolved))
	if isDynamicValueType(resolved) {
		resultExpr = fmt.Sprintf("gwen_value_typeof(%s)", argExpr)
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", tempName(inst.ValueID), resultExpr))
	return nil
}

func (s *bodyState) emitBuiltinDirectCall(call *mir.Value, inst *mir.CallInst, builtinName, helper string, argCount int) error {
	if len(call.Args) != argCount {
		return fmt.Errorf("%s() expects exactly %d argument(s), got %d", builtinName, argCount, len(call.Args))
	}
	expectedTypes := builtinExpectedTypes(builtinName)
	args := make([]string, 0, len(call.Args))
	for idx, argID := range call.Args {
		expectedType := hir.Type(nil)
		if idx < len(expectedTypes) {
			expectedType = expectedTypes[idx]
		}
		argExpr, err := s.exprForExpectedValue(argID, expectedType)
		if err != nil {
			return err
		}
		args = append(args, argExpr)
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("%s() unexpectedly produced multi-return MIR", builtinName)
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s);", tempName(inst.ValueID), helper, strings.Join(args, ", ")))
	return nil
}

func (s *bodyState) emitBuiltinVoidCall(call *mir.Value, builtinName, helper string, argCount int) error {
	if len(call.Args) != argCount {
		return fmt.Errorf("%s() expects exactly %d argument(s), got %d", builtinName, argCount, len(call.Args))
	}
	if len(call.ReturnTypes) != 0 {
		return fmt.Errorf("%s() unexpectedly produced return types", builtinName)
	}
	expectedTypes := builtinExpectedTypes(builtinName)
	args := make([]string, 0, len(call.Args))
	for idx, argID := range call.Args {
		expectedType := hir.Type(nil)
		if idx < len(expectedTypes) {
			expectedType = expectedTypes[idx]
		}
		argExpr, err := s.exprForExpectedValue(argID, expectedType)
		if err != nil {
			return err
		}
		args = append(args, argExpr)
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s(%s);", helper, strings.Join(args, ", ")))
	return nil
}

func (s *bodyState) emitBuiltinStrCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("str() expects exactly 1 argument, got %d", len(call.Args))
	}
	arg := s.body.Value(call.Args[0])
	if arg == nil {
		return fmt.Errorf("unknown MIR str arg value %d", call.Args[0])
	}
	argExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	resolved := s.emitter.resolveType(arg.Type)
	resultExpr := ""
	switch {
	case isStringType(resolved):
		resultExpr = argExpr
	case isBoolType(resolved):
		resultExpr = fmt.Sprintf("gwen_bool_display_string(%s)", argExpr)
	case isIntegerType(resolved):
		resultExpr = fmt.Sprintf("gwen_int_to_string((long long)(%s))", argExpr)
	case isFloatType(resolved):
		resultExpr = fmt.Sprintf("gwen_float_display_string((double)(%s))", argExpr)
	case moneyType(resolved) != nil:
		resultExpr = fmt.Sprintf("gwen_money_to_string(%s)", argExpr)
	case isDynamicValueType(resolved):
		resultExpr = fmt.Sprintf("gwen_value_display_string(%s)", argExpr)
	default:
		return fmt.Errorf("unsupported str() argument type %s", typeLabel(resolved))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("str() unexpectedly produced multi-return MIR")
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", tempName(inst.ValueID), resultExpr))
	return nil
}

func (s *bodyState) emitBuiltinSplitCall(call *mir.Value, inst *mir.CallInst) error {
	return s.emitBuiltinDirectCall(call, inst, "split", "gwen_string_split", 2)
}

func (s *bodyState) emitBuiltinJoinCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("join() expects exactly 2 arguments, got %d", len(call.Args))
	}
	listExpr, helperName, err := s.listJoinTarget(call.Args[0])
	if err != nil {
		return err
	}
	sepExpr, err := s.exprForValue(call.Args[1])
	if err != nil {
		return err
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("join() unexpectedly produced multi-return MIR")
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s(%s, %s);", tempName(inst.ValueID), helperName, listExpr, sepExpr))
	return nil
}

func (s *bodyState) emitBuiltinStringBoolCall(callName string, call *mir.Value, inst *mir.CallInst) error {
	helper := ""
	switch callName {
	case "startswith":
		helper = "gwen_string_startswith"
	case "endswith":
		helper = "gwen_string_endswith"
	case "contains":
		helper = "gwen_string_contains"
	default:
		return fmt.Errorf("unsupported string boolean builtin %q", callName)
	}
	return s.emitBuiltinDirectCall(call, inst, callName, helper, 2)
}

func (s *bodyState) emitBuiltinAbsCall(call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("abs() expects exactly 1 argument, got %d", len(call.Args))
	}
	arg := s.body.Value(call.Args[0])
	if arg == nil {
		return fmt.Errorf("unknown MIR abs arg value %d", call.Args[0])
	}
	argExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	argType := s.emitter.resolveType(arg.Type)
	resultExpr := ""
	switch {
	case isUnsignedIntegerType(argType):
		resultExpr = argExpr
	case isSignedIntegerType(argType):
		resultExpr = fmt.Sprintf("((%s) < 0 ? -(%s) : (%s))", argExpr, argExpr, argExpr)
	case isFloatType(argType):
		if namedTypeName(argType) == "float32" {
			resultExpr = fmt.Sprintf("fabsf(%s)", argExpr)
		} else {
			resultExpr = fmt.Sprintf("fabs(%s)", argExpr)
		}
	default:
		return fmt.Errorf("unsupported abs() argument type %s", typeLabel(argType))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("abs() unexpectedly produced multi-return MIR")
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = %s;", tempName(inst.ValueID), resultExpr))
	return nil
}

func (s *bodyState) emitBuiltinMinMaxCall(callName string, call *mir.Value, inst *mir.CallInst) error {
	if len(call.Args) != 2 {
		return fmt.Errorf("%s() expects exactly 2 arguments, got %d", callName, len(call.Args))
	}
	left := s.body.Value(call.Args[0])
	if left == nil {
		return fmt.Errorf("unknown MIR %s left arg value %d", callName, call.Args[0])
	}
	right := s.body.Value(call.Args[1])
	if right == nil {
		return fmt.Errorf("unknown MIR %s right arg value %d", callName, call.Args[1])
	}
	leftType := s.emitter.resolveType(left.Type)
	rightType := s.emitter.resolveType(right.Type)
	if typeLabel(leftType) != typeLabel(rightType) {
		return fmt.Errorf("%s() requires same-type arguments, got %s and %s", callName, typeLabel(leftType), typeLabel(rightType))
	}
	leftExpr, err := s.exprForValue(call.Args[0])
	if err != nil {
		return err
	}
	rightExpr, err := s.exprForValue(call.Args[1])
	if err != nil {
		return err
	}
	op := "<"
	if callName == "max" {
		op = ">"
	}
	condExpr := ""
	switch {
	case isStringType(leftType):
		condExpr = fmt.Sprintf("gwen_string_cmp(%s, %s) %s 0", leftExpr, rightExpr, op)
	case isBoolType(leftType):
		condExpr = fmt.Sprintf("((%s) ? 1 : 0) %s ((%s) ? 1 : 0)", leftExpr, op, rightExpr)
	case isNumericType(leftType):
		condExpr = fmt.Sprintf("(%s %s %s)", leftExpr, op, rightExpr)
	default:
		return fmt.Errorf("unsupported %s() argument type %s", callName, typeLabel(leftType))
	}
	if len(inst.ResultIDs) != 0 {
		return fmt.Errorf("%s() unexpectedly produced multi-return MIR", callName)
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = (%s ? %s : %s);", tempName(inst.ValueID), condExpr, leftExpr, rightExpr))
	return nil
}

func (s *bodyState) emitBuiltinUnaryMathCall(call *mir.Value, inst *mir.CallInst, builtinName, helper string) error {
	if len(call.Args) != 1 {
		return fmt.Errorf("%s() expects exactly 1 argument, got %d", builtinName, len(call.Args))
	}
	arg := s.body.Value(call.Args[0])
	if arg == nil {
		return fmt.Errorf("unknown MIR %s arg value %d", builtinName, call.Args[0])
	}
	argType := s.emitter.resolveType(arg.Type)
	if !isFloatType(argType) {
		return fmt.Errorf("%s() requires float input, got %s", builtinName, typeLabel(argType))
	}
	return s.emitBuiltinDirectCall(call, inst, builtinName, helper, 1)
}

func (s *bodyState) builtinLenExpr(argID int) (string, error) {
	arg := s.body.Value(argID)
	if arg == nil {
		return "", fmt.Errorf("unknown MIR len arg value %d", argID)
	}
	argExpr, err := s.exprForValue(argID)
	if err != nil {
		return "", err
	}
	resolved := s.emitter.resolveType(arg.Type)
	switch {
	case isDynamicValueType(resolved):
		return "gwen_value_len(" + argExpr + ")", nil
	case isStringType(resolved):
		return "gwen_string_len(" + argExpr + ")", nil
	case listItemType(resolved) != nil:
		return "(" + argExpr + ").len", nil
	case dictValueType(resolved) != nil:
		return "(" + argExpr + ").len", nil
	default:
		return "", fmt.Errorf("unsupported len() argument type %s", typeLabel(resolved))
	}
}

func (s *bodyState) dictHelperTarget(argID int, nameFn func(string) string) (string, string, error) {
	arg := s.body.Value(argID)
	if arg == nil {
		return "", "", fmt.Errorf("unknown MIR dict arg value %d", argID)
	}
	resolved := s.emitter.resolveType(arg.Type)
	if isBareDictType(resolved) {
		dictExpr, err := s.exprForValue(argID)
		if err != nil {
			return "", "", err
		}
		return dictExpr, "", nil
	}
	if dictValueType(resolved) == nil {
		return "", "", fmt.Errorf("unsupported dict builtin argument type %s", typeLabel(resolved))
	}
	dictExpr, err := s.exprForValue(argID)
	if err != nil {
		return "", "", err
	}
	dictType, err := s.emitter.cType(resolved)
	if err != nil {
		return "", "", err
	}
	return dictExpr, nameFn(dictType), nil
}

func (s *bodyState) listJoinTarget(argID int) (string, string, error) {
	arg := s.body.Value(argID)
	if arg == nil {
		return "", "", fmt.Errorf("unknown MIR list arg value %d", argID)
	}
	resolved := s.emitter.resolveType(arg.Type)
	itemType := listItemType(resolved)
	if itemType == nil {
		return "", "", fmt.Errorf("unsupported join() argument type %s", typeLabel(resolved))
	}
	if _, _, err := s.emitter.displayStringHelper(itemType); err != nil {
		return "", "", fmt.Errorf("unsupported join() list item type %s", typeLabel(itemType))
	}
	listExpr, err := s.exprForValue(argID)
	if err != nil {
		return "", "", err
	}
	listType, err := s.emitter.cType(resolved)
	if err != nil {
		return "", "", err
	}
	return listExpr, listJoinFuncName(listType), nil
}

func (s *bodyState) listSortTarget(listArgID int, cmpArgID int) (string, string, error) {
	arg := s.body.Value(listArgID)
	if arg == nil {
		return "", "", fmt.Errorf("unknown MIR sort list value %d", listArgID)
	}
	resolved := s.emitter.resolveType(arg.Type)
	itemType := listItemType(resolved)
	if itemType == nil {
		return "", "", fmt.Errorf("unsupported sort() argument type %s", typeLabel(resolved))
	}
	listExpr, err := s.exprForValue(listArgID)
	if err != nil {
		return "", "", err
	}
	listType, err := s.emitter.cType(resolved)
	if err != nil {
		return "", "", err
	}
	moduleName, callName, ok := s.builtinCallIdentity(cmpArgID)
	if !ok || (moduleName != "" && moduleName != "list") {
		return "", "", fmt.Errorf("unsupported sort() comparator")
	}
	switch callName {
	case "asc":
		return listExpr, listSortFuncName(listType, false), nil
	case "desc":
		return listExpr, listSortFuncName(listType, true), nil
	default:
		return "", "", fmt.Errorf("unsupported sort() comparator %q", callName)
	}
}

func (s *bodyState) emitSQLiteParamsValue(argID int, targetName string) error {
	arg := s.body.Value(argID)
	if arg == nil {
		return fmt.Errorf("unknown MIR sqlite params value %d", argID)
	}
	resolved := s.emitter.resolveType(arg.Type)
	if isBareListType(resolved) {
		paramsExpr, err := s.dynamicExprForValue(argID)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = %s;", targetName, paramsExpr))
		return nil
	}
	itemType := listItemType(resolved)
	if itemType == nil {
		return fmt.Errorf("sqlite params require list input, got %s", typeLabel(resolved))
	}
	listExpr, err := s.exprForValue(argID)
	if err != nil {
		return err
	}
	itemResolved := s.emitter.resolveType(itemType)
	itemExpr := fmt.Sprintf("(%s).items[i]", listExpr)
	valueExpr := ""
	switch {
	case isDynamicValueType(itemResolved):
		valueExpr = itemExpr
	case isStringType(itemResolved):
		valueExpr = fmt.Sprintf("gwen_value_string(%s)", itemExpr)
	case isBoolType(itemResolved):
		valueExpr = fmt.Sprintf("gwen_value_bool(%s)", itemExpr)
	case isIntegerType(itemResolved):
		valueExpr = fmt.Sprintf("gwen_value_int((long long)(%s))", itemExpr)
	case isFloatType(itemResolved):
		valueExpr = fmt.Sprintf("gwen_value_float((double)(%s))", itemExpr)
	default:
		return fmt.Errorf("unsupported sqlite params item type %s", typeLabel(itemResolved))
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = gwen_value_list_from_ptr(gwen_dyn_list_new((%s).len));", targetName, listExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  for (long long i = 0; i < (%s).len; ++i) {", listExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("    %s.list_value->items[i] = %s;", targetName, valueExpr))
	s.emitter.writeLine(s.indent + "  }")
	return nil
}

func (s *bodyState) builtinCastExpr(name string, argID int) (string, error) {
	arg := s.body.Value(argID)
	if arg == nil {
		return "", fmt.Errorf("unknown MIR cast arg value %d", argID)
	}
	argExpr, err := s.exprForValue(argID)
	if err != nil {
		return "", err
	}
	resolved := s.emitter.resolveType(arg.Type)
	switch name {
	case "int":
		if isDynamicValueType(resolved) {
			return "gwen_value_cast_int(" + argExpr + ")", nil
		}
		if isStringType(resolved) {
			return "gwen_value_cast_int(gwen_value_string(" + argExpr + "))", nil
		}
		if !isNumericOrBoolType(resolved) {
			return "", fmt.Errorf("unsupported int() argument type %s", typeLabel(resolved))
		}
		return "(long long)(" + argExpr + ")", nil
	case "float":
		if isDynamicValueType(resolved) {
			return "gwen_value_cast_float(" + argExpr + ")", nil
		}
		if isStringType(resolved) {
			return "gwen_value_cast_float(gwen_value_string(" + argExpr + "))", nil
		}
		if !isNumericOrBoolType(resolved) {
			return "", fmt.Errorf("unsupported float() argument type %s", typeLabel(resolved))
		}
		return "(double)(" + argExpr + ")", nil
	default:
		return "", fmt.Errorf("unsupported builtin cast %q", name)
	}
}

func (s *bodyState) emitTerm(blockID int, term mir.Terminator, returns []hir.Type) error {
	switch node := term.(type) {
	case *mir.JumpTerm:
		s.resetLoopStateForTarget(node.Target)
		s.emitter.writeLine(s.indent + fmt.Sprintf("goto block_%d;", node.Target))
		return nil

	case *mir.CondTerm:
		condExpr, err := s.exprForValue(node.ConditionValue)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s) goto block_%d;", condExpr, node.Then))
		s.emitter.writeLine(s.indent + fmt.Sprintf("goto block_%d;", node.Else))
		return nil

	case *mir.ForRangeTerm:
		return s.emitForRangeTerm(blockID, node)

	case *mir.ForEachTerm:
		return s.emitForEachTerm(blockID, node)

	case *mir.MatchTerm:
		return s.emitMatchTerm(node)

	case *mir.ReturnTerm:
		if s.parallelReturnsDynamic {
			valueExpr, err := s.parallelValueExpr(node.ValueIDs)
			if err != nil {
				return err
			}
			s.emitter.writeLine(s.indent + "return " + valueExpr + ";")
			return nil
		}
		if len(returns) == 0 {
			s.emitter.writeLine(s.indent + "return;")
			return nil
		}
		switch len(node.ValueIDs) {
		case 1:
			valueExpr, err := s.exprForExpectedValue(node.ValueIDs[0], returns[0])
			if err != nil {
				return err
			}
			s.emitter.writeLine(s.indent + "return " + valueExpr + ";")
			return nil
		default:
			tupleType, err := s.emitter.tupleTypeName(returns)
			if err != nil {
				return err
			}
			values := make([]string, 0, len(node.ValueIDs))
			for idx, valueID := range node.ValueIDs {
				valueExpr, err := s.exprForExpectedValue(valueID, returns[idx])
				if err != nil {
					return err
				}
				values = append(values, valueExpr)
			}
			s.emitter.writeLine(s.indent + fmt.Sprintf("return (%s){%s};", tupleType, strings.Join(values, ", ")))
			return nil
		}

	case *mir.StopTerm:
		if s.parallelReturnsDynamic {
			valueExpr, err := s.parallelValueExpr(s.parallelResultValueIDs)
			if err != nil {
				return err
			}
			s.emitter.writeLine(s.indent + "return " + valueExpr + ";")
			return nil
		}
		if len(returns) != 0 {
			zeroReturn, err := s.stopReturnExpr(returns)
			if err != nil {
				return err
			}
			s.emitter.writeLine(s.indent + "gwen_runtime_error(\"runtime error: reached unexpected stop in non-void function\");")
			s.emitter.writeLine(s.indent + "return " + zeroReturn + ";")
			return nil
		}
		s.emitter.writeLine(s.indent + "return;")
		return nil

	case nil:
		return fmt.Errorf("missing terminator")

	default:
		return fmt.Errorf("unsupported MIR terminator %T", term)
	}
}

func (s *bodyState) emitForRangeTerm(blockID int, node *mir.ForRangeTerm) error {
	if node == nil || node.VarBinding == nil {
		return fmt.Errorf("for-range terminator missing loop binding")
	}
	slot := s.body.SlotByBindingID(node.VarBinding.ID)
	if slot == nil {
		return fmt.Errorf("missing slot for for-range variable %q", node.Var)
	}
	resolved := s.emitter.resolveType(slot.Type)
	if !isIntegerType(resolved) {
		return fmt.Errorf("unsupported for-range variable type %s", typeLabel(resolved))
	}
	slotType, err := s.emitter.cType(slot.Type)
	if err != nil {
		return err
	}
	startExpr, err := s.exprForValue(node.StartValue)
	if err != nil {
		return err
	}
	endExpr, err := s.exprForValue(node.EndValue)
	if err != nil {
		return err
	}

	startedName := loopStartedName(blockID)
	currentName := loopCurrentName(blockID)
	endName := loopEndName(blockID)
	stepName := loopStepName(blockID)

	s.emitter.writeLine(s.indent + fmt.Sprintf("if (!%s) {", startedName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (long long)(%s);", currentName, startExpr))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (long long)(%s);", endName, endExpr))
	if node.StepValue != 0 {
		stepExpr, err := s.exprForValue(node.StepValue)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (long long)(%s);", stepName, stepExpr))
	} else {
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = %s <= %s ? 1LL : -1LL;", stepName, currentName, endName))
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s == 0) gwen_runtime_error(\"runtime error: for-range step cannot be 0\");", stepName))
	switch node.Direction {
	case "", "auto":
	case "asc":
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s > %s) {", currentName, endName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    long long gwen_loop_swap_%d = %s;", blockID, currentName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = %s;", currentName, endName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = gwen_loop_swap_%d;", endName, blockID))
		s.emitter.writeLine(s.indent + "  }")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s < 0) %s = -%s;", stepName, stepName, stepName))
	case "desc":
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s < %s) {", currentName, endName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    long long gwen_loop_swap_%d = %s;", blockID, currentName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = %s;", currentName, endName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("    %s = gwen_loop_swap_%d;", endName, blockID))
		s.emitter.writeLine(s.indent + "  }")
		s.emitter.writeLine(s.indent + fmt.Sprintf("  if (%s > 0) %s = -%s;", stepName, stepName, stepName))
	default:
		return fmt.Errorf("unsupported for-range direction %q", node.Direction)
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = true;", startedName))
	s.emitter.writeLine(s.indent + "} else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = %s + %s;", currentName, currentName, stepName))
	s.emitter.writeLine(s.indent + "}")
	s.emitter.writeLine(s.indent + fmt.Sprintf("if (((%s > 0) && (%s <= %s)) || ((%s < 0) && (%s >= %s))) {", stepName, currentName, endName, stepName, currentName, endName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (%s)(%s);", s.slotExpr(slot.ID, node.VarBinding), slotType, currentName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = true;", s.slotInitExpr(slot.ID, node.VarBinding)))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  goto block_%d;", node.Body))
	s.emitter.writeLine(s.indent + "}")
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = false;", startedName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("goto block_%d;", node.Exit))
	return nil
}

func (s *bodyState) emitForEachTerm(blockID int, node *mir.ForEachTerm) error {
	if node == nil || node.VarBinding == nil {
		return fmt.Errorf("for-each terminator missing loop binding")
	}
	itemSlot := s.body.SlotByBindingID(node.VarBinding.ID)
	if itemSlot == nil {
		return fmt.Errorf("missing slot for for-each variable %q", node.Var)
	}
	itemType, err := s.emitter.cType(itemSlot.Type)
	if err != nil {
		return err
	}
	iterableExpr, err := s.exprForValue(node.IterableValue)
	if err != nil {
		return err
	}
	iterableValue := s.body.Value(node.IterableValue)
	if iterableValue == nil {
		return fmt.Errorf("unknown iterable value %d", node.IterableValue)
	}
	if _, _, err := s.emitter.listTypeKeyAndItemType(iterableValue.Type); err != nil {
		if !isBareListType(s.emitter.resolveType(iterableValue.Type)) {
			return fmt.Errorf("unsupported for-each iterable type %s", typeLabel(iterableValue.Type))
		}
	}

	startedName := loopStartedName(blockID)
	indexName := loopIndexName(blockID)
	s.emitter.writeLine(s.indent + fmt.Sprintf("if (!%s) {", startedName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = 0LL;", indexName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = true;", startedName))
	s.emitter.writeLine(s.indent + "} else {")
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = %s + 1LL;", indexName, indexName))
	s.emitter.writeLine(s.indent + "}")
	if isBareListType(s.emitter.resolveType(iterableValue.Type)) {
		s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s < gwen_value_as_list(%s)->len) {", indexName, iterableExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (%s)(gwen_value_as_list(%s)->items[%s]);", s.slotExpr(itemSlot.ID, node.VarBinding), itemType, iterableExpr, indexName))
	} else {
		s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s < %s.len) {", indexName, iterableExpr))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (%s)(%s.items[%s]);", s.slotExpr(itemSlot.ID, node.VarBinding), itemType, iterableExpr, indexName))
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = true;", s.slotInitExpr(itemSlot.ID, node.VarBinding)))
	if node.IndexBinding != nil {
		indexSlot := s.body.SlotByBindingID(node.IndexBinding.ID)
		if indexSlot == nil {
			return fmt.Errorf("missing slot for for-each index %q", node.IndexVar)
		}
		indexType, err := s.emitter.cType(indexSlot.Type)
		if err != nil {
			return err
		}
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = (%s)(%s);", s.slotExpr(indexSlot.ID, node.IndexBinding), indexType, indexName))
		s.emitter.writeLine(s.indent + fmt.Sprintf("  %s = true;", s.slotInitExpr(indexSlot.ID, node.IndexBinding)))
	}
	s.emitter.writeLine(s.indent + fmt.Sprintf("  goto block_%d;", node.Body))
	s.emitter.writeLine(s.indent + "}")
	s.emitter.writeLine(s.indent + fmt.Sprintf("%s = false;", startedName))
	s.emitter.writeLine(s.indent + fmt.Sprintf("goto block_%d;", node.Exit))
	return nil
}

func (s *bodyState) resetLoopStateForTarget(targetBlockID int) {
	for headerBlockID, loop := range s.rangeLoops {
		if loop != nil && loop.Exit == targetBlockID {
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s = false;", loopStartedName(headerBlockID)))
		}
	}
	for headerBlockID, loop := range s.forEachLoops {
		if loop != nil && loop.Exit == targetBlockID {
			s.emitter.writeLine(s.indent + fmt.Sprintf("%s = false;", loopStartedName(headerBlockID)))
		}
	}
}

func (s *bodyState) emitMatchTerm(node *mir.MatchTerm) error {
	if node == nil {
		return fmt.Errorf("nil match terminator")
	}
	if node.Binding == nil {
		return fmt.Errorf("match terminator is missing binding metadata")
	}
	subjectValue := s.body.Value(node.SubjectValue)
	if subjectValue == nil {
		return fmt.Errorf("unknown match subject value %d", node.SubjectValue)
	}
	subjectExpr, err := s.exprForValue(node.SubjectValue)
	if err != nil {
		return err
	}
	subjectType := s.emitter.resolveType(subjectValue.Type)
	for _, arm := range node.Cases {
		clauses, err := s.matchClauses(node.Binding, subjectExpr, subjectType, arm)
		if err != nil {
			return err
		}
		for _, clause := range clauses {
			s.emitter.writeLine(s.indent + fmt.Sprintf("if (%s) {", clause.Cond))
			for _, bind := range clause.Binds {
				s.emitter.writeLine(s.indent + "  " + bind)
			}
			s.emitter.writeLine(s.indent + fmt.Sprintf("  goto block_%d;", arm.Target))
			s.emitter.writeLine(s.indent + "}")
		}
	}
	if node.HasElse {
		s.emitter.writeLine(s.indent + fmt.Sprintf("goto block_%d;", node.Else))
		return nil
	}
	s.emitter.writeLine(s.indent + "gwen_runtime_error(\"runtime error: match statement has no matching case and no 'else' branch (exhaustive match required)\");")
	return nil
}

func (s *bodyState) stopReturnExpr(returns []hir.Type) (string, error) {
	switch len(returns) {
	case 0:
		return "", fmt.Errorf("stopReturnExpr requires non-void returns")
	case 1:
		return s.emitter.zeroExpr(returns[0])
	default:
		tupleType, err := s.emitter.tupleTypeName(returns)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s){0}", tupleType), nil
	}
}

func (s *bodyState) matchClauses(binding *hir.MatchBinding, subjectExpr string, subjectType hir.Type, arm mir.MatchArm) ([]matchClause, error) {
	switch binding.Kind {
	case hir.MatchBindingResult:
		if resultOKType(subjectType) == nil || resultErrType(subjectType) == nil {
			return nil, fmt.Errorf("unsupported match subject type %s", typeLabel(subjectType))
		}
		return s.resultMatchClauses(subjectExpr, arm)
	case hir.MatchBindingValue:
		if !isValueMatchSubjectType(subjectType) {
			return nil, fmt.Errorf("unsupported match subject type %s", typeLabel(subjectType))
		}
		return s.valueMatchClauses(subjectExpr, subjectType, arm)
	default:
		return nil, fmt.Errorf("unsupported match binding kind %q", binding.Kind)
	}
}

func (s *bodyState) resultMatchClauses(subjectExpr string, arm mir.MatchArm) ([]matchClause, error) {
	if len(arm.Patterns) != len(arm.PatternBindings) {
		return nil, fmt.Errorf("result match arm metadata mismatch: %d patterns, %d bindings", len(arm.Patterns), len(arm.PatternBindings))
	}
	clauses := make([]matchClause, 0, len(arm.Patterns))
	for idx, pattern := range arm.Patterns {
		cond, binds, err := s.resultMatchPattern(subjectExpr, pattern, arm.PatternBindings[idx])
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, matchClause{Cond: cond, Binds: binds})
	}
	return clauses, nil
}

func (s *bodyState) resultMatchPattern(subjectExpr string, pattern hir.Expr, binding *hir.MatchPatternBinding) (string, []string, error) {
	if binding == nil {
		return "", nil, fmt.Errorf("result match pattern is missing binding metadata")
	}
	switch binding.Kind {
	case hir.MatchPatternResultOk:
		okExpr, ok := pattern.(*hir.Ok)
		if !ok {
			return "", nil, fmt.Errorf("expected ok(...) pattern, got %T", pattern)
		}
		binds, err := s.resultMatchBinds(okExpr.Value, subjectExpr, "ok")
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("(%s).is_ok", subjectExpr), binds, nil
	case hir.MatchPatternResultErr:
		errExpr, ok := pattern.(*hir.Err)
		if !ok {
			return "", nil, fmt.Errorf("expected err(...) pattern, got %T", pattern)
		}
		binds, err := s.resultMatchBinds(errExpr.Value, subjectExpr, "err")
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("!(%s).is_ok", subjectExpr), binds, nil
	default:
		return "", nil, fmt.Errorf("unsupported result match pattern kind %q", binding.Kind)
	}
}

func (s *bodyState) valueMatchClauses(subjectExpr string, subjectType hir.Type, arm mir.MatchArm) ([]matchClause, error) {
	if len(arm.Patterns) != len(arm.PatternBindings) {
		return nil, fmt.Errorf("value match arm metadata mismatch: %d patterns, %d bindings", len(arm.Patterns), len(arm.PatternBindings))
	}
	clauses := make([]matchClause, 0, len(arm.Patterns))
	for idx, pattern := range arm.Patterns {
		cond, binds, err := s.valueMatchPattern(subjectExpr, subjectType, pattern, arm.PatternBindings[idx])
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, matchClause{Cond: cond, Binds: binds})
	}
	return clauses, nil
}

func (s *bodyState) valueMatchPattern(subjectExpr string, subjectType hir.Type, pattern hir.Expr, binding *hir.MatchPatternBinding) (string, []string, error) {
	if binding == nil {
		return "", nil, fmt.Errorf("value match pattern is missing binding metadata")
	}
	switch binding.Kind {
	case hir.MatchPatternCapture:
		ident, ok := pattern.(*hir.Ident)
		if !ok {
			return "", nil, fmt.Errorf("expected capture pattern, got %T", pattern)
		}
		binds, err := s.valueMatchCaptureBinds(subjectExpr, subjectType, ident)
		if err != nil {
			return "", nil, err
		}
		return "true", binds, nil
	case hir.MatchPatternRange:
		rng, ok := pattern.(*hir.Binary)
		if !ok || rng.Op != "to" {
			return "", nil, fmt.Errorf("expected range pattern, got %T", pattern)
		}
		if !isIntegerType(subjectType) {
			return "", nil, fmt.Errorf("range pattern requires int match subject, got %s", typeLabel(subjectType))
		}
		startExpr, endExpr, err := inlineRangePatternBounds(rng)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s >= %s && %s <= %s", subjectExpr, startExpr, subjectExpr, endExpr), nil, nil
	case hir.MatchPatternValue:
		patternExpr, err := inlineValuePatternExpr(pattern)
		if err != nil {
			return "", nil, err
		}
		cond, err := valueMatchEquals(subjectExpr, patternExpr, subjectType)
		if err != nil {
			return "", nil, err
		}
		return cond, nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported value match pattern kind %q", binding.Kind)
	}
}

func (s *bodyState) resultMatchBinds(value hir.Expr, subjectExpr, field string) ([]string, error) {
	ident, ok := value.(*hir.Ident)
	if !ok {
		return nil, fmt.Errorf("only capture-style result patterns are supported in C emitter")
	}
	if ident.Binding == nil {
		return nil, fmt.Errorf("result pattern capture is missing binding")
	}
	slot := s.body.SlotByBindingID(ident.Binding.ID)
	if slot == nil {
		return nil, fmt.Errorf("missing slot for result pattern capture %q", ident.Name)
	}
	slotType, err := s.emitter.cType(slot.Type)
	if err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("%s = (%s)((%s).%s);", s.slotExpr(slot.ID, ident.Binding), slotType, subjectExpr, field),
		fmt.Sprintf("%s = true;", s.slotInitExpr(slot.ID, ident.Binding)),
	}, nil
}

func (s *bodyState) valueMatchCaptureBinds(subjectExpr string, subjectType hir.Type, ident *hir.Ident) ([]string, error) {
	if ident == nil {
		return nil, fmt.Errorf("nil capture pattern")
	}
	if ident.Binding == nil {
		return nil, fmt.Errorf("value pattern capture is missing binding")
	}
	slot := s.body.SlotByBindingID(ident.Binding.ID)
	if slot == nil {
		return nil, fmt.Errorf("missing slot for value pattern capture %q", ident.Name)
	}
	slotType := slot.Type
	if slotType == nil {
		slotType = subjectType
	}
	typeName, err := s.emitter.cType(slotType)
	if err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("%s = (%s)(%s);", s.slotExpr(slot.ID, ident.Binding), typeName, subjectExpr),
		fmt.Sprintf("%s = true;", s.slotInitExpr(slot.ID, ident.Binding)),
	}, nil
}

func (s *bodyState) exprForValue(valueID int) (string, error) {
	if valueID == 0 {
		return "", fmt.Errorf("invalid MIR value 0")
	}
	if _, ok := s.tempIDs[valueID]; ok {
		return tempName(valueID), nil
	}
	value := s.body.Value(valueID)
	if value == nil {
		return "", fmt.Errorf("unknown MIR value %d", valueID)
	}
	switch value.Kind {
	case mir.ValueIntConst:
		return fmt.Sprintf("%dLL", value.IntValue), nil
	case mir.ValueFloatConst:
		return strconv.FormatFloat(value.FloatValue, 'g', -1, 64), nil
	case mir.ValueStringConst:
		return quoteCString(value.StringValue), nil
	case mir.ValueBoolConst:
		if value.BoolValue {
			return "true", nil
		}
		return "false", nil
	case mir.ValueSlotRef:
		slotType := value.Type
		if slotType == nil && value.SlotID > 0 && value.SlotID <= len(s.body.Slots) {
			slotType = s.body.Slots[value.SlotID-1].Type
		}
		return s.slotReadExpr(value.SlotID, value.Binding, slotType)
	case mir.ValueBindingRef:
		if value.Binding == nil {
			return "", fmt.Errorf("binding_ref without binding")
		}
		switch value.Binding.Kind {
		case hir.BindingFunc:
			fn, name, err := s.emitter.funcForBinding(value.Binding)
			if err != nil {
				return "", err
			}
			if funcType, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok {
				return s.funcValueExpr(fn, name, funcType)
			}
			return name, nil
		case hir.BindingBuiltin:
			if funcType, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok {
				return s.builtinFuncValueExpr("", value.Binding.Name, funcType)
			}
			return value.Binding.Name, nil
		case hir.BindingImported:
			if funcType, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok && isStdlibModuleName(value.Binding.SourceModule) {
				return s.builtinFuncValueExpr(value.Binding.SourceModule, value.Binding.Name, funcType)
			}
			fn, name, err := s.emitter.funcForBinding(value.Binding)
			if err != nil {
				return "", err
			}
			if funcType, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok {
				return s.funcValueExpr(fn, name, funcType)
			}
			return name, nil
		default:
			return "", fmt.Errorf("unsupported binding ref kind %q", value.Binding.Kind)
		}
	case mir.ValueMember:
		if value.MemberBinding == nil {
			return "", fmt.Errorf("member value %q is missing binding", value.Member)
		}
		switch value.MemberBinding.Kind {
		case hir.MemberBindingModuleValue:
			if funcType, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok && isStdlibModuleName(value.MemberBinding.OwnerName) {
				return s.builtinFuncValueExpr(value.MemberBinding.OwnerName, value.Member, funcType)
			}
			name, ok := s.emitter.moduleValues[moduleValueKey(value.MemberBinding.OwnerName, value.Member)]
			if !ok {
				return "", fmt.Errorf("unknown module member %q.%q", value.MemberBinding.OwnerName, value.Member)
			}
			if funcType, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok {
				return s.funcValueExpr(nil, name, funcType)
			}
			return name, nil
		case hir.MemberBindingObjectConstructor:
			name, err := s.emitter.objectMemberCName(s.body, value)
			if err != nil {
				return "", err
			}
			if funcType, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok {
				return s.funcValueExpr(nil, name, funcType)
			}
			return name, nil
		case hir.MemberBindingObjectMethod:
			name, err := s.emitter.objectMemberCName(s.body, value)
			if err != nil {
				return "", err
			}
			if funcType, ok := s.emitter.resolveType(value.Type).(*hir.FuncType); ok {
				if !s.emitter.exprIsObjectTypeValue(s.body, value.Object) {
					info, _, err := s.emitter.objectMemberInfo(s.body, value)
					if err != nil {
						return "", err
					}
					method := s.emitter.objectMethod(info, value.Member)
					if method == nil {
						return "", fmt.Errorf("missing object method %q.%q", info.name, value.Member)
					}
					if len(method.Params) == 0 {
						return "", fmt.Errorf("object method %q.%q is missing receiver parameter", info.name, value.Member)
					}
					receiverExpr, err := s.exprForExpectedValue(value.Object, method.Params[0].Type)
					if err != nil {
						return "", err
					}
					receiverExpr, err = s.emitter.cloneExpr(receiverExpr, method.Params[0].Type)
					if err != nil {
						return "", err
					}
					return fmt.Sprintf("%s(%s)", boundMethodClosureConstructorName(name), receiverExpr), nil
				}
				return s.funcValueExpr(nil, name, funcType)
			}
			return name, nil
		case hir.MemberBindingObjectField:
			return s.memberExpr(value)
		default:
			return "", fmt.Errorf("unsupported member binding %q", value.MemberBinding.Kind)
		}
	case mir.ValueCallResult:
		return "", fmt.Errorf("call result %d was not materialized as temp", valueID)
	default:
		return "", fmt.Errorf("unsupported inline MIR value kind %q", value.Kind)
	}
}

func (s *bodyState) exprForExpectedValue(valueID int, expectedType hir.Type) (string, error) {
	expr, err := s.exprForValue(valueID)
	if err != nil {
		return "", err
	}
	value := s.body.Value(valueID)
	if value == nil {
		return "", fmt.Errorf("unknown MIR value %d", valueID)
	}
	return s.coerceExpr(expr, value.Type, expectedType)
}

func (s *bodyState) dynamicExprForValue(valueID int) (string, error) {
	return s.exprForExpectedValue(valueID, &hir.NamedType{Name: "dynamic"})
}

func (s *bodyState) coerceExpr(expr string, actualType hir.Type, expectedType hir.Type) (string, error) {
	actualResolved := s.emitter.resolveType(actualType)
	expectedResolved := s.emitter.resolveType(expectedType)
	if expectedResolved == nil {
		return expr, nil
	}
	if typeLabel(actualResolved) == typeLabel(expectedResolved) {
		return expr, nil
	}
	if moneyType(expectedResolved) != nil {
		return s.coerceMoneyExpr(expr, actualResolved, expectedResolved)
	}
	if isDynamicValueType(expectedResolved) {
		return s.emitter.dynamicValueExpr(expr, actualResolved)
	}
	if isDynamicValueType(actualResolved) {
		return s.emitter.coerceDynamicExpr(expr, expectedResolved)
	}
	return expr, nil
}

func (s *bodyState) coerceMoneyExpr(expr string, actualType hir.Type, expectedType hir.Type) (string, error) {
	expectedMoney := moneyType(expectedType)
	if expectedMoney == nil {
		return "", fmt.Errorf("unsupported money coercion target %s", typeLabel(expectedType))
	}
	currency := moneyCurrencyName(expectedMoney)
	if actualMoney := moneyType(actualType); actualMoney != nil {
		actualCurrency := moneyCurrencyName(actualMoney)
		if actualCurrency != currency {
			return "", fmt.Errorf("Currency mismatch: cannot assign money[%s] to money[%s]", actualCurrency, currency)
		}
		return expr, nil
	}
	if isIntegerType(actualType) {
		return fmt.Sprintf("gwen_money_from_int((long long)(%s), %s)", expr, quoteCString(currency)), nil
	}
	if isFloatType(actualType) {
		return fmt.Sprintf("gwen_money_from_float((double)(%s), %s)", expr, quoteCString(currency)), nil
	}
	return "", fmt.Errorf("Cannot convert %s to money[%s]", typeLabel(actualType), currency)
}

func (e *emitter) coerceDynamicExpr(expr string, expectedType hir.Type) (string, error) {
	expectedResolved := e.resolveType(expectedType)
	switch {
	case expectedResolved == nil:
		return expr, nil
	case isDynamicValueType(expectedResolved):
		return expr, nil
	case isStringType(expectedResolved):
		return fmt.Sprintf("gwen_value_as_string(%s)", expr), nil
	case isBoolType(expectedResolved):
		return fmt.Sprintf("gwen_value_as_bool(%s)", expr), nil
	case isIntegerType(expectedResolved):
		return fmt.Sprintf("gwen_value_as_int(%s)", expr), nil
	case isFloatType(expectedResolved):
		return fmt.Sprintf("gwen_value_as_float(%s)", expr), nil
	}
	if itemType := listItemType(expectedResolved); itemType != nil {
		listType, err := e.cType(expectedResolved)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(%s)", listFromValueFuncName(listType), expr), nil
	}
	if dictKeyType(expectedResolved) != nil && dictValueType(expectedResolved) != nil {
		dictType, err := e.cType(expectedResolved)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(%s)", dictFromValueFuncName(dictType), expr), nil
	}
	return "", fmt.Errorf("unsupported dynamic coercion to %s", typeLabel(expectedResolved))
}

func (e *emitter) coerceWrapperExpr(expr string, actualType hir.Type, expectedType hir.Type) (string, error) {
	actualResolved := e.resolveType(actualType)
	expectedResolved := e.resolveType(expectedType)
	if expectedResolved == nil || typeLabel(actualResolved) == typeLabel(expectedResolved) {
		return expr, nil
	}
	if isDynamicValueType(expectedResolved) {
		return e.dynamicValueExpr(expr, actualResolved)
	}
	if isDynamicValueType(actualResolved) {
		return e.coerceDynamicExpr(expr, expectedResolved)
	}
	return expr, nil
}

func (e *emitter) builtinClosureExpr(moduleName, name string, funcType *hir.FuncType, argNames []string) (string, bool, error) {
	switch canonicalBuiltinModuleName(moduleName, name) {
	case "":
		switch name {
		case "read":
			return e.builtinDirectClosureExpr("read", "gwen_read_line", funcType, argNames)
		case "len":
			if len(argNames) != 1 || len(funcType.Params) != 1 || len(funcType.Returns) != 1 {
				return "", false, fmt.Errorf("unsupported len() builtin function type %s", typeLabel(funcType))
			}
			expr, err := e.builtinClosureLenExpr(argNames[0], funcType.Params[0])
			return expr, false, err
		case "str":
			if len(argNames) != 1 || len(funcType.Params) != 1 || len(funcType.Returns) != 1 {
				return "", false, fmt.Errorf("unsupported str() builtin function type %s", typeLabel(funcType))
			}
			expr, err := e.builtinClosureStrExpr(argNames[0], funcType.Params[0])
			return expr, false, err
		case "typeof":
			if len(argNames) != 1 || len(funcType.Params) != 1 || len(funcType.Returns) != 1 {
				return "", false, fmt.Errorf("unsupported typeof() builtin function type %s", typeLabel(funcType))
			}
			expr, err := e.builtinClosureTypeofExpr(argNames[0], funcType.Params[0])
			return expr, false, err
		case "int", "float":
			if len(argNames) != 1 || len(funcType.Params) != 1 || len(funcType.Returns) != 1 {
				return "", false, fmt.Errorf("unsupported %s() builtin function type %s", name, typeLabel(funcType))
			}
			expr, err := e.builtinClosureCastExpr(name, argNames[0], funcType.Params[0])
			return expr, false, err
		}
	case "string":
		switch name {
		case "split":
			return e.builtinDirectClosureExpr("split", "gwen_string_split", funcType, argNames)
		case "join":
			if len(argNames) != 2 || len(funcType.Params) != 2 || len(funcType.Returns) != 1 {
				return "", false, fmt.Errorf("unsupported join() builtin function type %s", typeLabel(funcType))
			}
			expr, err := e.builtinClosureJoinExpr(argNames[0], argNames[1], funcType.Params[0])
			return expr, false, err
		case "substring":
			return e.builtinDirectClosureExpr("substring", "gwen_string_substring", funcType, argNames)
		case "trim":
			return e.builtinDirectClosureExpr("trim", "gwen_string_trim", funcType, argNames)
		case "replace":
			return e.builtinDirectClosureExpr("replace", "gwen_string_replace", funcType, argNames)
		case "startswith":
			return e.builtinDirectClosureExpr("startswith", "gwen_string_startswith", funcType, argNames)
		case "endswith":
			return e.builtinDirectClosureExpr("endswith", "gwen_string_endswith", funcType, argNames)
		case "contains":
			return e.builtinDirectClosureExpr("contains", "gwen_string_contains", funcType, argNames)
		}
	case "math":
		switch name {
		case "abs":
			if len(argNames) != 1 || len(funcType.Params) != 1 || len(funcType.Returns) != 1 {
				return "", false, fmt.Errorf("unsupported abs() builtin function type %s", typeLabel(funcType))
			}
			expr, err := e.builtinClosureAbsExpr(argNames[0], funcType.Params[0], funcType.Returns[0])
			return expr, false, err
		case "sqrt":
			return e.builtinDirectClosureExpr("sqrt", "sqrt", funcType, argNames)
		case "floor":
			return e.builtinDirectClosureExpr("floor", "floor", funcType, argNames)
		case "ceil":
			return e.builtinDirectClosureExpr("ceil", "ceil", funcType, argNames)
		}
	case "io":
		switch name {
		case "readfile":
			return e.builtinDirectClosureExpr("readfile", "gwen_io_readfile", funcType, argNames)
		case "readdir":
			return e.builtinDirectClosureExpr("readdir", "gwen_io_readdir", funcType, argNames)
		case "writefile":
			return e.builtinDirectClosureExpr("writefile", "gwen_io_writefile", funcType, argNames)
		case "appendfile":
			return e.builtinDirectClosureExpr("appendfile", "gwen_io_appendfile", funcType, argNames)
		}
	case "path":
		switch name {
		case "basename":
			return e.builtinDirectClosureExpr("path.basename", "gwen_path_basename", funcType, argNames)
		case "dirname":
			return e.builtinDirectClosureExpr("path.dirname", "gwen_path_dirname", funcType, argNames)
		case "joinpath":
			return e.builtinDirectClosureExpr("path.joinpath", "gwen_path_join", funcType, argNames)
		}
	case "os":
		switch name {
		case "args":
			return e.builtinDirectClosureExpr("os.args", "gwen_os_args", funcType, argNames)
		case "cwd":
			return e.builtinDirectClosureExpr("os.cwd", "gwen_os_cwd", funcType, argNames)
		case "getenv":
			return e.builtinDirectClosureExpr("os.getenv", "gwen_os_getenv", funcType, argNames)
		}
	case "time":
		switch name {
		case "sleep":
			return e.builtinDirectClosureExpr("time.sleep", "gwen_time_sleep", funcType, argNames)
		case "nowunix":
			return e.builtinDirectClosureExpr("time.nowunix", "gwen_time_nowunix", funcType, argNames)
		case "nowunixms":
			return e.builtinDirectClosureExpr("time.nowunixms", "gwen_time_nowunixms", funcType, argNames)
		case "nowrfc3339":
			return e.builtinDirectClosureExpr("time.nowrfc3339", "gwen_time_nowrfc3339", funcType, argNames)
		}
	case "json":
		switch name {
		case "parseobject":
			return e.builtinDirectClosureExpr("json.parseobject", "gwen_json_parseobject_result", funcType, argNames)
		case "parsearray":
			return e.builtinDirectClosureExpr("json.parsearray", "gwen_json_parsearray_result", funcType, argNames)
		case "stringify":
			return e.builtinDirectClosureExpr("json.stringify", "gwen_json_stringify_result", funcType, argNames)
		case "isnull":
			return e.builtinDirectClosureExpr("json.isnull", "gwen_value_is_null", funcType, argNames)
		}
	}
	label := name
	if moduleName != "" {
		label = moduleName + "." + name
	}
	return "", false, fmt.Errorf("compiled builtin function values do not support %q yet", label)
}

func (e *emitter) builtinDirectClosureExpr(helperKey, helperName string, funcType *hir.FuncType, argNames []string) (string, bool, error) {
	if funcType == nil {
		return "", false, fmt.Errorf("missing builtin function type")
	}
	if len(argNames) != len(funcType.Params) {
		return "", false, fmt.Errorf("builtin wrapper arg count mismatch: got %d want %d", len(argNames), len(funcType.Params))
	}
	args := make([]string, 0, len(argNames))
	expectedTypes := builtinExpectedTypes(helperKey)
	for idx, argName := range argNames {
		expectedType := hir.Type(nil)
		if idx < len(expectedTypes) {
			expectedType = expectedTypes[idx]
		}
		expr, err := e.coerceWrapperExpr(argName, funcType.Params[idx], expectedType)
		if err != nil {
			return "", false, err
		}
		args = append(args, expr)
	}
	return fmt.Sprintf("%s(%s)", helperName, strings.Join(args, ", ")), len(funcType.Returns) == 0, nil
}

func (e *emitter) builtinClosureLenExpr(argName string, argType hir.Type) (string, error) {
	resolved := e.resolveType(argType)
	switch {
	case isDynamicValueType(resolved):
		return fmt.Sprintf("gwen_value_len(%s)", argName), nil
	case isStringType(resolved):
		return fmt.Sprintf("gwen_string_len(%s)", argName), nil
	case listItemType(resolved) != nil || dictValueType(resolved) != nil:
		return fmt.Sprintf("(%s).len", argName), nil
	default:
		return "", fmt.Errorf("unsupported len() builtin function argument type %s", typeLabel(argType))
	}
}

func (e *emitter) builtinClosureJoinExpr(listArgName, sepArgName string, listType hir.Type) (string, error) {
	resolved := e.resolveType(listType)
	switch namedTypeName(resolved) {
	case "dynamic", "list":
		return fmt.Sprintf("gwen_value_list_join(%s, %s)", listArgName, sepArgName), nil
	}
	itemType := listItemType(resolved)
	if itemType == nil {
		return "", fmt.Errorf("unsupported join() builtin function argument type %s", typeLabel(listType))
	}
	if _, _, err := e.displayStringHelper(itemType); err != nil {
		return "", fmt.Errorf("unsupported join() builtin function list item type %s", typeLabel(itemType))
	}
	listCType, err := e.cType(resolved)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s(%s, %s)", listJoinFuncName(listCType), listArgName, sepArgName), nil
}

func (e *emitter) builtinClosureStrExpr(argName string, argType hir.Type) (string, error) {
	resolved := e.resolveType(argType)
	if isDynamicValueType(resolved) {
		return fmt.Sprintf("gwen_value_display_string(%s)", argName), nil
	}
	if helper, castType, err := e.displayStringHelper(resolved); err == nil {
		if castType != "" {
			return fmt.Sprintf("%s((%s)(%s))", helper, castType, argName), nil
		}
		return fmt.Sprintf("%s(%s)", helper, argName), nil
	}
	dynamicExpr, err := e.dynamicValueExpr(argName, resolved)
	if err != nil {
		return "", fmt.Errorf("unsupported str() builtin function argument type %s", typeLabel(argType))
	}
	return fmt.Sprintf("gwen_value_display_string(%s)", dynamicExpr), nil
}

func (e *emitter) builtinClosureTypeofExpr(argName string, argType hir.Type) (string, error) {
	resolved := e.resolveType(argType)
	if isDynamicValueType(resolved) {
		return fmt.Sprintf("gwen_value_typeof(%s)", argName), nil
	}
	return quoteCString(typeLabel(resolved)), nil
}

func (e *emitter) builtinClosureCastExpr(name, argName string, argType hir.Type) (string, error) {
	resolved := e.resolveType(argType)
	switch name {
	case "int":
		if isDynamicValueType(resolved) {
			return fmt.Sprintf("gwen_value_cast_int(%s)", argName), nil
		}
		if isStringType(resolved) {
			return fmt.Sprintf("gwen_value_cast_int(gwen_value_string(%s))", argName), nil
		}
		if isNumericOrBoolType(resolved) {
			return fmt.Sprintf("(long long)(%s)", argName), nil
		}
		if dynamicExpr, err := e.dynamicValueExpr(argName, resolved); err == nil {
			return fmt.Sprintf("gwen_value_cast_int(%s)", dynamicExpr), nil
		}
		return "", fmt.Errorf("unsupported int() builtin function argument type %s", typeLabel(argType))
	case "float":
		if isDynamicValueType(resolved) {
			return fmt.Sprintf("gwen_value_cast_float(%s)", argName), nil
		}
		if isStringType(resolved) {
			return fmt.Sprintf("gwen_value_cast_float(gwen_value_string(%s))", argName), nil
		}
		if isNumericOrBoolType(resolved) {
			return fmt.Sprintf("(double)(%s)", argName), nil
		}
		if dynamicExpr, err := e.dynamicValueExpr(argName, resolved); err == nil {
			return fmt.Sprintf("gwen_value_cast_float(%s)", dynamicExpr), nil
		}
		return "", fmt.Errorf("unsupported float() builtin function argument type %s", typeLabel(argType))
	default:
		return "", fmt.Errorf("unsupported builtin cast wrapper %q", name)
	}
}

func (e *emitter) builtinClosureAbsExpr(argName string, argType hir.Type, returnType hir.Type) (string, error) {
	argResolved := e.resolveType(argType)
	returnResolved := e.resolveType(returnType)
	resultExpr := ""
	resultType := argResolved
	switch namedTypeName(argResolved) {
	case "dynamic":
		if namedTypeName(returnResolved) != "dynamic" {
			return "", fmt.Errorf("unsupported abs() builtin function return type %s for dynamic input", typeLabel(returnType))
		}
		return fmt.Sprintf("gwen_value_abs(%s)", argName), nil
	}
	switch {
	case isUnsignedIntegerType(argResolved):
		resultExpr = argName
	case isSignedIntegerType(argResolved):
		resultExpr = fmt.Sprintf("((%s) < 0 ? -(%s) : (%s))", argName, argName, argName)
	case isFloatType(argResolved):
		if namedTypeName(argResolved) == "float32" {
			resultExpr = fmt.Sprintf("fabsf(%s)", argName)
		} else {
			resultExpr = fmt.Sprintf("fabs(%s)", argName)
		}
	default:
		return "", fmt.Errorf("unsupported abs() builtin function argument type %s", typeLabel(argType))
	}
	if namedTypeName(returnResolved) == "dynamic" {
		return e.dynamicValueExpr(resultExpr, resultType)
	}
	if typeLabel(resultType) != typeLabel(returnResolved) {
		return "", fmt.Errorf("unsupported abs() builtin function return type %s for %s input", typeLabel(returnType), typeLabel(argType))
	}
	return resultExpr, nil
}

func (e *emitter) dynamicValueExpr(expr string, actualType hir.Type) (string, error) {
	actualResolved := e.resolveType(actualType)
	switch {
	case isDynamicValueType(actualResolved):
		return expr, nil
	case isStringType(actualResolved):
		return fmt.Sprintf("gwen_value_string(%s)", expr), nil
	case isBoolType(actualResolved):
		return fmt.Sprintf("gwen_value_bool(%s)", expr), nil
	case isIntegerType(actualResolved):
		return fmt.Sprintf("gwen_value_int((long long)(%s))", expr), nil
	case isFloatType(actualResolved):
		return fmt.Sprintf("gwen_value_float((double)(%s))", expr), nil
	}
	if okType := resultOKType(actualResolved); okType != nil {
		errType := resultErrType(actualResolved)
		okExpr, err := e.dynamicValueExpr(fmt.Sprintf("(%s).ok", expr), okType)
		if err != nil {
			return "", err
		}
		errExpr, err := e.dynamicValueExpr(fmt.Sprintf("(%s).err", expr), errType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("((%s).is_ok ? gwen_value_result_ok(%s) : gwen_value_result_err(%s))", expr, okExpr, errExpr), nil
	}
	if itemType := listItemType(actualResolved); itemType != nil {
		listType, err := e.cType(actualResolved)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(%s)", listToValueFuncName(listType), expr), nil
	}
	if dictKeyType(actualResolved) != nil && dictValueType(actualResolved) != nil {
		dictType, err := e.cType(actualResolved)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(%s)", dictToValueFuncName(dictType), expr), nil
	}
	return "", fmt.Errorf("unsupported dynamic conversion from %s", typeLabel(actualResolved))
}

func (s *bodyState) parallelValueExpr(valueIDs []int) (string, error) {
	switch len(valueIDs) {
	case 0:
		return "gwen_value_null()", nil
	case 1:
		value := s.body.Value(valueIDs[0])
		if value == nil {
			return "", fmt.Errorf("unknown MIR value %d", valueIDs[0])
		}
		if value.Type == nil {
			return "gwen_value_null()", nil
		}
		expr, err := s.exprForValue(valueIDs[0])
		if err != nil {
			return "", err
		}
		return s.emitter.dynamicValueExpr(expr, value.Type)
	default:
		types := make([]hir.Type, 0, len(valueIDs))
		values := make([]string, 0, len(valueIDs))
		for _, valueID := range valueIDs {
			value := s.body.Value(valueID)
			if value == nil {
				return "", fmt.Errorf("unknown MIR value %d", valueID)
			}
			valueExpr, err := s.exprForValue(valueID)
			if err != nil {
				return "", err
			}
			types = append(types, value.Type)
			values = append(values, valueExpr)
		}
		tupleType, err := s.emitter.tupleTypeName(types)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s((%s){%s})", tupleToValueFuncName(tupleType), tupleType, strings.Join(values, ", ")), nil
	}
}

func (s *bodyState) computeExpr(value *mir.Value) (string, error) {
	switch value.Kind {
	case mir.ValueUnary:
		operand, err := s.exprForValue(value.Operand)
		if err != nil {
			return "", err
		}
		switch value.Op {
		case "-":
			return "(-" + operand + ")", nil
		case "not":
			return "(!" + operand + ")", nil
		default:
			return "", fmt.Errorf("unsupported unary op %q", value.Op)
		}

	case mir.ValueBinary:
		return s.binaryExpr(value)

	case mir.ValueCast:
		return s.castExpr(value)

	case mir.ValueMember:
		return s.memberExpr(value)

	default:
		return "", fmt.Errorf("unsupported computed MIR value kind %q", value.Kind)
	}
}

func (s *bodyState) memberExpr(value *mir.Value) (string, error) {
	_, kind, err := s.emitter.objectMemberInfo(s.body, value)
	if err != nil {
		return "", err
	}
	switch kind {
	case hir.MemberBindingObjectField:
		objectExpr, err := s.exprForValue(value.Object)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s)->%s", objectExpr, objectFieldName(value.Member)), nil
	default:
		return "", fmt.Errorf("unsupported member expression %q", kind)
	}
}

func (s *bodyState) binaryExpr(value *mir.Value) (string, error) {
	left, err := s.exprForValue(value.Left)
	if err != nil {
		return "", err
	}
	right, err := s.exprForValue(value.Right)
	if err != nil {
		return "", err
	}
	leftType := s.emitter.resolveType(s.body.Value(value.Left).Type)
	rightType := s.emitter.resolveType(s.body.Value(value.Right).Type)
	if isDynamicValueType(leftType) && isDynamicValueType(rightType) {
		switch value.Op {
		case "=":
			return fmt.Sprintf("gwen_value_eq(%s, %s)", left, right), nil
		case "!=":
			return fmt.Sprintf("(!gwen_value_eq(%s, %s))", left, right), nil
		default:
			return "", fmt.Errorf("unsupported dynamic binary op %q", value.Op)
		}
	}
	if isDynamicValueType(leftType) && !isDynamicValueType(rightType) {
		left, err = s.coerceExpr(left, leftType, rightType)
		if err != nil {
			return "", err
		}
		leftType = rightType
	}
	if isDynamicValueType(rightType) && !isDynamicValueType(leftType) {
		right, err = s.coerceExpr(right, rightType, leftType)
		if err != nil {
			return "", err
		}
		rightType = leftType
	}
	leftMoney := moneyType(leftType)
	rightMoney := moneyType(rightType)
	if leftMoney != nil || rightMoney != nil {
		switch {
		case leftMoney != nil && rightMoney != nil:
			switch value.Op {
			case "+":
				return fmt.Sprintf("gwen_money_add(%s, %s)", left, right), nil
			case "-":
				return fmt.Sprintf("gwen_money_sub(%s, %s)", left, right), nil
			case "/":
				return fmt.Sprintf("gwen_money_ratio(%s, %s)", left, right), nil
			case "=", "!=", "<", ">", "<=", ">=":
				op := mapBinaryOp(value.Op)
				if op == "" {
					return "", fmt.Errorf("unsupported money comparison %q", value.Op)
				}
				return fmt.Sprintf("(gwen_money_cmp(%s, %s) %s 0)", left, right, op), nil
			default:
				return "", fmt.Errorf("unsupported money binary op %q", value.Op)
			}
		case leftMoney != nil && isIntegerType(rightType):
			switch value.Op {
			case "*":
				return fmt.Sprintf("gwen_money_mul_int(%s, (long long)(%s))", left, right), nil
			case "/":
				return fmt.Sprintf("gwen_money_div_int(%s, (long long)(%s))", left, right), nil
			}
		case leftMoney != nil && isFloatType(rightType):
			switch value.Op {
			case "*":
				return fmt.Sprintf("gwen_money_mul_float(%s, (double)(%s))", left, right), nil
			case "/":
				return fmt.Sprintf("gwen_money_div_float(%s, (double)(%s))", left, right), nil
			}
		case rightMoney != nil && isIntegerType(leftType) && value.Op == "*":
			return fmt.Sprintf("gwen_money_mul_int(%s, (long long)(%s))", right, left), nil
		case rightMoney != nil && isFloatType(leftType) && value.Op == "*":
			return fmt.Sprintf("gwen_money_mul_float(%s, (double)(%s))", right, left), nil
		}
		return "", fmt.Errorf("unsupported money binary op %q for %s and %s", value.Op, typeLabel(leftType), typeLabel(rightType))
	}
	if isStringType(leftType) || isStringType(rightType) {
		switch value.Op {
		case "+":
			if !isStringType(leftType) || !isStringType(rightType) {
				return "", fmt.Errorf("unsupported string concat types %s and %s", typeLabel(leftType), typeLabel(rightType))
			}
			return fmt.Sprintf("gwen_string_concat(%s, %s)", left, right), nil
		case "=":
			return fmt.Sprintf("gwen_string_eq(%s, %s)", left, right), nil
		case "!=":
			return fmt.Sprintf("gwen_string_ne(%s, %s)", left, right), nil
		default:
			return "", fmt.Errorf("unsupported string binary op %q", value.Op)
		}
	}
	op := mapBinaryOp(value.Op)
	if op == "" {
		return "", fmt.Errorf("unsupported binary op %q", value.Op)
	}
	return fmt.Sprintf("(%s %s %s)", left, op, right), nil
}

func (s *bodyState) castExpr(value *mir.Value) (string, error) {
	return "", fmt.Errorf("cast values must be emitted through emitCastValue")
}

func (s *bodyState) callExpr(call *mir.Value) (string, error) {
	args := make([]string, 0, len(call.Args))
	calleeValue := s.body.Value(call.Callee)
	var calleeType *hir.FuncType
	var calleeParams []*hir.Param
	if calleeValue != nil {
		calleeType, _ = s.emitter.resolveType(calleeValue.Type).(*hir.FuncType)
	}
	directCallee := ""
	var err error
	argOffset := 0
	if calleeValue != nil && calleeValue.Kind == mir.ValueMember {
		if info, kind, err := s.emitter.objectMemberInfo(s.body, calleeValue); err == nil && kind == hir.MemberBindingObjectMethod && !s.emitter.exprIsObjectTypeValue(s.body, calleeValue.Object) {
			directCallee, err = s.emitter.objectMemberCName(s.body, calleeValue)
			if err != nil {
				return "", err
			}
			receiverExpr, err := s.exprForExpectedValue(calleeValue.Object, &hir.NamedType{Name: info.name})
			if err != nil {
				return "", err
			}
			args = append(args, receiverExpr)
			if info.node != nil {
				for _, method := range info.node.Methods {
					if method != nil && method.Name == calleeValue.Member {
						calleeType = signatureType(method.Params, method.Returns)
						calleeParams = method.Params
						break
					}
				}
			}
			argOffset = 1
		}
	}
	if directCallee == "" && calleeValue != nil {
		switch calleeValue.Kind {
		case mir.ValueBindingRef:
			if calleeValue.Binding != nil && (calleeValue.Binding.Kind == hir.BindingFunc || calleeValue.Binding.Kind == hir.BindingImported) {
				fn, name, err := s.emitter.funcForBinding(calleeValue.Binding)
				if err != nil {
					return "", err
				}
				directCallee = name
				if fn != nil {
					calleeParams = fn.Params
				}
				if fn != nil && len(functionCaptureSlots(s.emitter, fn.Body)) > 0 {
					refCaptures := refCaptureBindingIDSet(fn.Body)
					for _, slot := range functionCaptureSlots(s.emitter, fn.Body) {
						argExpr, err := s.captureArgExpr(slot, hasCaptureBinding(refCaptures, slot.BindingID))
						if err != nil {
							return "", err
						}
						args = append(args, argExpr)
					}
				}
			}
		case mir.ValueMember:
			if calleeValue.MemberBinding != nil && calleeValue.MemberBinding.Kind == hir.MemberBindingModuleValue {
				name, ok := s.emitter.moduleValues[moduleValueKey(calleeValue.MemberBinding.OwnerName, calleeValue.Member)]
				if !ok {
					return "", fmt.Errorf("unknown module member %q.%q", calleeValue.MemberBinding.OwnerName, calleeValue.Member)
				}
				directCallee = name
				if fn := s.emitter.moduleFuncs[moduleValueKey(calleeValue.MemberBinding.OwnerName, calleeValue.Member)]; fn != nil {
					calleeParams = fn.Params
				}
				break
			}
			info, kind, err := s.emitter.objectMemberInfo(s.body, calleeValue)
			if err == nil && s.emitter.exprIsObjectTypeValue(s.body, calleeValue.Object) {
				switch kind {
				case hir.MemberBindingObjectConstructor:
					directCallee, err = s.emitter.objectMemberCName(s.body, calleeValue)
					if err != nil {
						return "", err
					}
					if info != nil && info.node != nil && info.node.Constructor != nil {
						calleeType = signatureType(info.node.Constructor.Params, info.node.Constructor.Returns)
						calleeParams = info.node.Constructor.Params
					}
				case hir.MemberBindingObjectMethod:
					directCallee, err = s.emitter.objectMemberCName(s.body, calleeValue)
					if err != nil {
						return "", err
					}
					if info != nil && info.node != nil {
						for _, method := range info.node.Methods {
							if method != nil && method.Name == calleeValue.Member {
								calleeType = signatureType(method.Params, method.Returns)
								calleeParams = method.Params
								break
							}
						}
					}
				}
			}
		}
	}
	for idx, argID := range call.Args {
		argExpr := ""
		expectedType := hir.Type(nil)
		if calleeType != nil && idx+argOffset < len(calleeType.Params) {
			expectedType = calleeType.Params[idx+argOffset]
		}
		argExpr, err = s.exprForExpectedValue(argID, expectedType)
		if err != nil {
			return "", err
		}
		args = append(args, argExpr)
	}
	if directCallee != "" {
		if len(calleeParams) > 0 {
			defaultArgs, err := s.defaultCallArgs(calleeParams, argOffset, len(call.Args))
			if err != nil {
				return "", err
			}
			args = append(args, defaultArgs...)
		}
		return fmt.Sprintf("%s(%s)", directCallee, strings.Join(args, ", ")), nil
	}
	callee, err := s.exprForValue(call.Callee)
	if err != nil {
		return "", err
	}
	if calleeType != nil {
		return s.funcInvokeExpr(calleeType, callee, args)
	}
	return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), nil
}

func (s *bodyState) defaultCallArgs(params []*hir.Param, argOffset int, providedArgs int) ([]string, error) {
	if len(params) == 0 {
		return nil, nil
	}
	start := argOffset + providedArgs
	if start < 0 {
		start = 0
	}
	if start >= len(params) {
		return nil, nil
	}
	args := make([]string, 0, len(params)-start)
	for idx := start; idx < len(params); idx++ {
		param := params[idx]
		if param == nil || param.Default == nil {
			return nil, fmt.Errorf("missing required argument %d for direct call", idx+1-argOffset)
		}
		argExpr, err := s.defaultArgExpr(param.Default, param.Type)
		if err != nil {
			return nil, fmt.Errorf("default argument %q: %w", param.Name, err)
		}
		args = append(args, argExpr)
	}
	return args, nil
}

func (s *bodyState) defaultArgExpr(expr hir.Expr, expectedType hir.Type) (string, error) {
	actualExpr, actualType, err := s.inlineDefaultExpr(expr)
	if err != nil {
		return "", err
	}
	if expectedType == nil || actualType == nil {
		return actualExpr, nil
	}
	coerced, err := s.coerceExpr(actualExpr, actualType, expectedType)
	if err != nil {
		return "", err
	}
	return coerced, nil
}

func (s *bodyState) inlineDefaultExpr(expr hir.Expr) (string, hir.Type, error) {
	switch node := expr.(type) {
	case *hir.IntLiteral:
		return fmt.Sprintf("%dLL", node.Value), &hir.NamedType{Name: "int"}, nil
	case *hir.FloatLiteral:
		return strconv.FormatFloat(node.Value, 'g', -1, 64), &hir.NamedType{Name: "float"}, nil
	case *hir.StringLiteral:
		return quoteCString(node.Value), &hir.NamedType{Name: "string"}, nil
	case *hir.BoolLiteral:
		if node.Value {
			return "true", &hir.NamedType{Name: "bool"}, nil
		}
		return "false", &hir.NamedType{Name: "bool"}, nil
	case *hir.Unary:
		operandExpr, operandType, err := s.inlineDefaultExpr(node.Operand)
		if err != nil {
			return "", nil, err
		}
		switch node.Op {
		case "-":
			if !isIntegerType(s.emitter.resolveType(operandType)) && !isFloatType(s.emitter.resolveType(operandType)) {
				return "", nil, fmt.Errorf("unsupported unary default operand %s", typeLabel(operandType))
			}
			return "(-" + operandExpr + ")", operandType, nil
		case "not":
			if !isBoolType(s.emitter.resolveType(operandType)) {
				return "", nil, fmt.Errorf("unsupported unary default operand %s", typeLabel(operandType))
			}
			return "(!" + operandExpr + ")", operandType, nil
		default:
			return "", nil, fmt.Errorf("unsupported unary default op %q", node.Op)
		}
	default:
		return "", nil, fmt.Errorf("unsupported default expression %T", expr)
	}
}

func (s *bodyState) builtinCallIdentity(valueID int) (string, string, bool) {
	value := s.body.Value(valueID)
	if value == nil {
		return "", "", false
	}
	switch value.Kind {
	case mir.ValueBindingRef:
		if value.Binding == nil {
			return "", "", false
		}
		switch value.Binding.Kind {
		case hir.BindingBuiltin:
			return "", value.Binding.Name, true
		case hir.BindingImported:
			if !isStdlibModuleName(value.Binding.SourceModule) {
				return "", "", false
			}
			return value.Binding.SourceModule, value.Binding.Name, true
		default:
			return "", "", false
		}
	case mir.ValueMember:
		if value.MemberBinding == nil || value.MemberBinding.Kind != hir.MemberBindingModuleValue {
			return "", "", false
		}
		if !isStdlibModuleName(value.MemberBinding.OwnerName) {
			return "", "", false
		}
		return value.MemberBinding.OwnerName, value.Member, true
	default:
		return "", "", false
	}
}

func (e *emitter) funcSignature(fn *mir.Func) (string, error) {
	funcName, err := e.funcCName(fn)
	if err != nil {
		return "", err
	}
	return e.signatureForParams(fn.Body, fn.Params, fn.Returns, funcName)
}

func (e *emitter) constructorSignature(info *objectInfo, ctor *mir.Constructor) (string, error) {
	if info == nil {
		return "", fmt.Errorf("missing object info")
	}
	if ctor == nil {
		return "", fmt.Errorf("nil constructor")
	}
	return e.signatureForParams(ctor.Body, ctor.Params, ctor.Returns, info.constructorName)
}

func (e *emitter) methodSignature(info *objectInfo, method *mir.Method) (string, error) {
	if info == nil {
		return "", fmt.Errorf("missing object info")
	}
	if method == nil {
		return "", fmt.Errorf("nil method")
	}
	name, ok := info.methodNames[method.Name]
	if !ok {
		return "", fmt.Errorf("missing C name for object method %q.%q", info.name, method.Name)
	}
	return e.signatureForParams(method.Body, method.Params, method.Returns, name)
}

func (e *emitter) signatureForParams(body *mir.Body, params []*hir.Param, returns []hir.Type, name string) (string, error) {
	returnType, err := e.signatureReturnType(returns)
	if err != nil {
		return "", err
	}
	paramDecls := make([]string, 0, len(params))
	refCaptures := refCaptureBindingIDSet(body)
	for _, slot := range functionCaptureSlots(e, body) {
		if slot == nil {
			continue
		}
		typeName, err := e.captureParamType(slot.Type, hasCaptureBinding(refCaptures, slot.BindingID))
		if err != nil {
			return "", err
		}
		paramDecls = append(paramDecls, fmt.Sprintf("%s %s", typeName, slotName(slot.ID)))
	}
	for _, param := range params {
		slot := body.SlotByBindingID(param.Binding.ID)
		if slot == nil {
			return "", fmt.Errorf("missing MIR slot for parameter %q", param.Name)
		}
		typeName, err := e.cType(param.Type)
		if err != nil {
			return "", err
		}
		paramDecls = append(paramDecls, fmt.Sprintf("%s %s", typeName, slotName(slot.ID)))
	}
	if len(paramDecls) == 0 {
		paramDecls = append(paramDecls, "void")
	}
	return fmt.Sprintf("static %s %s(%s)", returnType, name, strings.Join(paramDecls, ", ")), nil
}

func (e *emitter) captureParamType(typ hir.Type, byRef bool) (string, error) {
	typeName, err := e.cType(typ)
	if err != nil {
		return "", err
	}
	if byRef {
		return typeName + " *", nil
	}
	return typeName, nil
}

func (e *emitter) signatureReturnType(returns []hir.Type) (string, error) {
	switch len(returns) {
	case 0:
		return "void", nil
	case 1:
		return e.cType(returns[0])
	default:
		return e.tupleTypeName(returns)
	}
}

func (e *emitter) cTypeForValue(value *mir.Value) (string, error) {
	if value == nil {
		return "", fmt.Errorf("nil MIR value")
	}
	if value.Kind == mir.ValueExprFallback {
		if _, ok := value.Source.(*hir.Lambda); ok {
			return "", fmt.Errorf("capturing or complex lambda expressions are not supported yet")
		}
	}
	if value.Kind == mir.ValueCall {
		switch len(value.ReturnTypes) {
		case 0:
		case 1:
			return e.cType(value.ReturnTypes[0])
		default:
			return e.tupleTypeName(value.ReturnTypes)
		}
	}
	return e.cType(value.Type)
}

func (e *emitter) collectValueTypes(body *mir.Body, value *mir.Value) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case mir.ValueBindingRef:
		return nil
	case mir.ValueCall:
		switch len(value.ReturnTypes) {
		case 0:
			return nil
		case 1:
			_, err := e.cType(value.ReturnTypes[0])
			return err
		default:
			_, err := e.tupleTypeName(value.ReturnTypes)
			return err
		}
	}
	if value.Type == nil {
		return nil
	}
	if _, ok := e.resolveType(value.Type).(*hir.FuncType); ok {
		if value.Kind == mir.ValueMember && value.MemberBinding != nil && value.MemberBinding.Kind == hir.MemberBindingObjectMethod && !e.exprIsObjectTypeValue(body, value.Object) {
			if _, err := e.cType(value.Type); err != nil {
				return err
			}
			info, _, err := e.objectMemberInfo(body, value)
			if err != nil {
				return err
			}
			name, err := e.objectMemberCName(body, value)
			if err != nil {
				return err
			}
			if method := e.objectMethod(info, value.Member); method == nil || len(method.Params) == 0 {
				return fmt.Errorf("object method %q.%q is missing receiver parameter", info.name, value.Member)
			}
			e.boundMethodClosures[name] = struct{}{}
		}
		return nil
	}
	_, err := e.cTypeForValue(value)
	return err
}

func (e *emitter) zeroExpr(typ hir.Type) (string, error) {
	resolved := e.resolveType(typ)
	if isDynamicValueType(resolved) {
		return "gwen_value_null()", nil
	}
	if money := moneyType(resolved); money != nil {
		return fmt.Sprintf("gwen_money_zero(%s)", quoteCString(moneyCurrencyName(money))), nil
	}
	if _, ok := resolved.(*hir.FuncType); ok {
		typeName, err := e.cType(resolved)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s){NULL, NULL}", typeName), nil
	}
	if _, ok := e.objectInfoForResolvedType(resolved); ok {
		return "NULL", nil
	}
	switch node := resolved.(type) {
	case *hir.NamedType:
		switch node.Name {
		case "int", "int64", "int32", "int16", "int8", "uint32", "uint16", "uint8":
			return "0", nil
		case "float", "float64", "float32":
			return "0.0", nil
		case "bool":
			return "false", nil
		case "string":
			return "NULL", nil
		}
	}
	typeName, err := e.cType(resolved)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("(%s){0}", typeName), nil
}

func (e *emitter) cType(typ hir.Type) (string, error) {
	resolved := e.resolveType(typ)
	if moneyType(resolved) != nil {
		return "gwen_money", nil
	}
	switch node := resolved.(type) {
	case nil:
		return "", fmt.Errorf("missing type")
	case *hir.NamedType:
		if info, ok := e.objectInfoForResolvedType(node); ok {
			return info.typeName + " *", nil
		}
		switch node.Name {
		case "SqliteDB":
			return "gwen_sqlite_db", nil
		case "HttpRequest":
			return "gwen_http_request", nil
		case "HttpReply":
			return "gwen_http_reply", nil
		case "HttpResponse":
			return "gwen_http_response", nil
		case "HttpServer":
			return "gwen_http_server", nil
		case "dynamic", "list", "dict", "JsonNull":
			return "gwen_value", nil
		case "int", "int64":
			return "long long", nil
		case "int32":
			return "int32_t", nil
		case "int16":
			return "int16_t", nil
		case "int8":
			return "int8_t", nil
		case "uint32":
			return "uint32_t", nil
		case "uint16":
			return "uint16_t", nil
		case "uint8":
			return "uint8_t", nil
		case "float", "float64":
			return "double", nil
		case "float32":
			return "float", nil
		case "bool":
			return "bool", nil
		case "string":
			return "const char *", nil
		default:
			return "", fmt.Errorf("unsupported type %q", node.Name)
		}
	case *hir.GenericType:
		if node.Base == "money" {
			return "gwen_money", nil
		}
		if node.Base == "list" {
			return e.listTypeName(node)
		}
		if node.Base == "dict" {
			return e.dictTypeName(node)
		}
		if node.Base == "result" {
			return e.resultTypeName(node)
		}
		if node.Base == "cell" {
			return e.cellTypeName(node)
		}
		return "", fmt.Errorf("unsupported type %s", typeLabel(resolved))
	case *hir.FuncType:
		return e.funcTypeName(node)
	default:
		return "", fmt.Errorf("unsupported type %s", typeLabel(resolved))
	}
}

func (e *emitter) funcTypeName(typ *hir.FuncType) (string, error) {
	if typ == nil {
		return "", fmt.Errorf("missing function type")
	}
	returnType, err := e.signatureReturnType(typ.Returns)
	if err != nil {
		return "", err
	}
	paramTypes := make([]string, 0, len(typ.Params))
	for _, param := range typ.Params {
		name, err := e.cType(param)
		if err != nil {
			return "", err
		}
		paramTypes = append(paramTypes, name)
	}
	key := returnType + "|" + strings.Join(paramTypes, "|")
	if name, ok := e.funcTypeByKey[key]; ok {
		return name, nil
	}
	name := fmt.Sprintf("gwen_func_%d", len(e.funcTypeOrder)+1)
	e.funcTypeByKey[key] = name
	e.funcTypeOrder = append(e.funcTypeOrder, key)
	e.funcReturns[key] = returnType
	e.funcParams[key] = append([]string(nil), paramTypes...)
	return name, nil
}

func (e *emitter) listTypeName(typ *hir.GenericType) (string, error) {
	key, _, err := e.listTypeKeyAndItemType(typ)
	if err != nil {
		return "", err
	}
	if name, ok := e.listByKey[key]; ok {
		return name, nil
	}
	name := fmt.Sprintf("gwen_list_%d", len(e.listOrder)+1)
	e.listByKey[key] = name
	e.listOrder = append(e.listOrder, key)
	return name, nil
}

func (e *emitter) dictTypeName(typ *hir.GenericType) (string, error) {
	key, _, _, err := e.dictTypeKeyAndFieldTypes(typ)
	if err != nil {
		return "", err
	}
	if name, ok := e.dictByKey[key]; ok {
		return name, nil
	}
	name := fmt.Sprintf("gwen_dict_%d", len(e.dictOrder)+1)
	e.dictByKey[key] = name
	e.dictOrder = append(e.dictOrder, key)
	return name, nil
}

func (e *emitter) resultTypeName(typ *hir.GenericType) (string, error) {
	if isHTTPReplyResultType(typ) {
		return "gwen_result_http_reply", nil
	}
	key, _, _, err := e.resultTypeKeyAndFieldTypes(typ)
	if err != nil {
		return "", err
	}
	if name, ok := e.resultByKey[key]; ok {
		return name, nil
	}
	name := fmt.Sprintf("gwen_result_%d", len(e.resultOrder)+1)
	e.resultByKey[key] = name
	e.resultOrder = append(e.resultOrder, key)
	return name, nil
}

func (e *emitter) cellTypeName(typ *hir.GenericType) (string, error) {
	key, _, err := e.cellTypeKeyAndItemType(typ)
	if err != nil {
		return "", err
	}
	if name, ok := e.cellByKey[key]; ok {
		return name, nil
	}
	name := fmt.Sprintf("gwen_cell_%d", len(e.cellOrder)+1)
	e.cellByKey[key] = name
	e.cellOrder = append(e.cellOrder, key)
	return name, nil
}

func (e *emitter) listTypeKeyAndItemType(typ hir.Type) (string, string, error) {
	generic, ok := e.resolveType(typ).(*hir.GenericType)
	if !ok || generic.Base != "list" || len(generic.Args) != 1 {
		return "", "", fmt.Errorf("unsupported list type %s", typeLabel(typ))
	}
	itemHIR := e.resolveType(generic.Args[0])
	itemType, err := e.cType(itemHIR)
	if err != nil {
		return "", "", err
	}
	key := itemType
	if _, ok := e.listItems[key]; !ok {
		e.listItems[key] = itemType
		e.listItemHIR[key] = itemHIR
	}
	return key, itemType, nil
}

func (e *emitter) dictTypeKeyAndFieldTypes(typ hir.Type) (string, string, string, error) {
	generic, ok := e.resolveType(typ).(*hir.GenericType)
	if !ok || generic.Base != "dict" || len(generic.Args) != 2 {
		return "", "", "", fmt.Errorf("unsupported dict type %s", typeLabel(typ))
	}
	keyNamed, ok := e.resolveType(generic.Args[0]).(*hir.NamedType)
	if !ok {
		return "", "", "", fmt.Errorf("unsupported dict key type %s", typeLabel(generic.Args[0]))
	}
	if !isStringType(keyNamed) && !isNumericOrBoolType(keyNamed) {
		return "", "", "", fmt.Errorf("unsupported dict key type %s", typeLabel(keyNamed))
	}
	keyType, err := e.cType(keyNamed)
	if err != nil {
		return "", "", "", err
	}
	valueHIR := e.resolveType(generic.Args[1])
	valueType, err := e.cType(valueHIR)
	if err != nil {
		return "", "", "", err
	}
	key := keyType + "|" + valueType
	if _, ok := e.dictKeys[key]; !ok {
		e.dictKeys[key] = keyType
		e.dictValues[key] = valueType
		e.dictKeyHIR[key] = keyNamed
		e.dictValueHIR[key] = valueHIR
		if _, err := e.listTypeName(&hir.GenericType{Base: "list", Args: []hir.Type{keyNamed}}); err != nil {
			return "", "", "", err
		}
		if _, err := e.listTypeName(&hir.GenericType{Base: "list", Args: []hir.Type{valueHIR}}); err != nil {
			return "", "", "", err
		}
	}
	return key, keyType, valueType, nil
}

func (e *emitter) resultTypeKeyAndFieldTypes(typ hir.Type) (string, string, string, error) {
	generic, ok := e.resolveType(typ).(*hir.GenericType)
	if !ok || generic.Base != "result" || len(generic.Args) == 0 {
		return "", "", "", fmt.Errorf("unsupported result type %s", typeLabel(typ))
	}
	okType, err := e.cType(generic.Args[0])
	if err != nil {
		return "", "", "", err
	}
	errTypeNode := hir.Type(&hir.NamedType{Name: "string"})
	if len(generic.Args) > 1 {
		errTypeNode = generic.Args[1]
		for _, candidate := range generic.Args[2:] {
			if typeLabel(candidate) != typeLabel(errTypeNode) {
				return "", "", "", fmt.Errorf("unsupported result error types %s", typeLabel(typ))
			}
		}
	}
	errType, err := e.cType(errTypeNode)
	if err != nil {
		return "", "", "", err
	}
	key := okType + "|" + errType
	if _, ok := e.resultOK[key]; !ok {
		e.resultOK[key] = okType
		e.resultErr[key] = errType
		e.resultOKHIR[key] = generic.Args[0]
		e.resultErrHIR[key] = errTypeNode
	}
	return key, okType, errType, nil
}

func (e *emitter) cellTypeKeyAndItemType(typ hir.Type) (string, string, error) {
	generic, ok := e.resolveType(typ).(*hir.GenericType)
	if !ok || generic.Base != "cell" || len(generic.Args) != 1 {
		return "", "", fmt.Errorf("unsupported cell type %s", typeLabel(typ))
	}
	itemType := e.resolveType(generic.Args[0])
	cItemType, err := e.cType(itemType)
	if err != nil {
		return "", "", err
	}
	key := cItemType
	if _, ok := e.cellItems[key]; !ok {
		e.cellItems[key] = cItemType
		e.cellItemHIR[key] = itemType
	}
	return key, cItemType, nil
}

func (e *emitter) cloneExpr(expr string, typ hir.Type) (string, error) {
	resolved := e.resolveType(typ)
	if isDynamicValueType(resolved) {
		return fmt.Sprintf("gwen_value_clone(%s)", expr), nil
	}
	if _, ok := resolved.(*hir.FuncType); ok {
		return expr, nil
	}
	if moneyType(resolved) != nil {
		return expr, nil
	}
	switch node := resolved.(type) {
	case *hir.NamedType:
		switch node.Name {
		case "int", "int64", "int32", "int16", "int8", "uint32", "uint16", "uint8", "float", "float64", "float32", "bool":
			return expr, nil
		case "string":
			return fmt.Sprintf("gwen_string_dup(%s)", expr), nil
		default:
			if info, ok := e.objectInfoForResolvedType(node); ok {
				return fmt.Sprintf("%s(%s)", info.cloneName, expr), nil
			}
			return "", fmt.Errorf("unsupported clone type %s", typeLabel(resolved))
		}
	case *hir.GenericType:
		switch node.Base {
		case "list":
			if _, err := e.cloneExpr("gwen_clone_item", listItemType(node)); err != nil {
				return "", err
			}
			listType, err := e.cType(node)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s(%s)", listCloneFuncName(listType), expr), nil
		case "dict":
			if _, err := e.cloneExpr("gwen_clone_key", dictKeyType(node)); err != nil {
				return "", err
			}
			if _, err := e.cloneExpr("gwen_clone_value", dictValueType(node)); err != nil {
				return "", err
			}
			dictType, err := e.cType(node)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s(%s)", dictCloneFuncName(dictType), expr), nil
		case "result":
			if _, err := e.cloneExpr("gwen_clone_ok", resultOKType(node)); err != nil {
				return "", err
			}
			if _, err := e.cloneExpr("gwen_clone_err", resultErrType(node)); err != nil {
				return "", err
			}
			resultType, err := e.cType(node)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s(%s)", resultCloneFuncName(resultType), expr), nil
		case "cell":
			if _, err := e.cloneExpr("gwen_clone_cell", cellItemType(node)); err != nil {
				return "", err
			}
			cellType, err := e.cType(node)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s(%s)", cellCloneFuncName(cellType), expr), nil
		default:
			return "", fmt.Errorf("unsupported clone type %s", typeLabel(resolved))
		}
	default:
		return "", fmt.Errorf("unsupported clone type %s", typeLabel(resolved))
	}
}

func (e *emitter) writeHelper(typ hir.Type) (string, error) {
	resolved := e.resolveType(typ)
	if moneyType(resolved) != nil {
		return "gwen_write_money", nil
	}
	switch node := resolved.(type) {
	case *hir.NamedType:
		switch node.Name {
		case "int", "int64", "int32", "int16", "int8", "uint32", "uint16", "uint8":
			return "gwen_write_int", nil
		case "float", "float64", "float32":
			return "gwen_write_float", nil
		case "bool":
			return "gwen_write_bool", nil
		case "string":
			return "gwen_write_string", nil
		}
	}
	return "", fmt.Errorf("unsupported write type %s", typeLabel(resolved))
}

func (e *emitter) displayStringHelper(typ hir.Type) (string, string, error) {
	resolved := e.resolveType(typ)
	switch {
	case isStringType(resolved):
		return "gwen_string_dup", "", nil
	case isBoolType(resolved):
		return "gwen_bool_to_string", "", nil
	case isIntegerType(resolved):
		return "gwen_int_to_string", "long long", nil
	case isFloatType(resolved):
		return "gwen_float_to_string", "double", nil
	case moneyType(resolved) != nil:
		return "gwen_money_to_string", "", nil
	default:
		return "", "", fmt.Errorf("unsupported display type %s", typeLabel(resolved))
	}
}

func (e *emitter) resolveType(typ hir.Type) hir.Type {
	if _, ok := typ.(*hir.NamedType); !ok {
		return typ
	}
	seen := map[string]struct{}{}
	current := typ
	for {
		named, ok := current.(*hir.NamedType)
		if !ok {
			return current
		}
		next, ok := e.typeAliases[named.Name]
		if !ok {
			return current
		}
		if _, dup := seen[named.Name]; dup {
			return current
		}
		seen[named.Name] = struct{}{}
		current = next
	}
}

func (e *emitter) objectInfoForResolvedType(typ hir.Type) (*objectInfo, bool) {
	named, ok := typ.(*hir.NamedType)
	if !ok {
		return nil, false
	}
	info, ok := e.objectInfo[named.Name]
	return info, ok
}

func (e *emitter) objectInfoForType(typ hir.Type) (*objectInfo, error) {
	resolved := e.resolveType(typ)
	info, ok := e.objectInfoForResolvedType(resolved)
	if !ok {
		return nil, fmt.Errorf("unsupported object type %s", typeLabel(typ))
	}
	return info, nil
}

func (e *emitter) objectFieldType(info *objectInfo, fieldName string) (hir.Type, error) {
	if info == nil || info.node == nil {
		return nil, fmt.Errorf("missing object info")
	}
	for _, field := range info.node.Fields {
		if field != nil && field.Name == fieldName {
			return field.Type, nil
		}
	}
	return nil, fmt.Errorf("object %q has no field %q", info.name, fieldName)
}

func (e *emitter) objectMethod(info *objectInfo, methodName string) *mir.Method {
	if info == nil || info.node == nil {
		return nil
	}
	for _, method := range info.node.Methods {
		if method != nil && method.Name == methodName {
			return method
		}
	}
	return nil
}

func (e *emitter) objectMemberInfo(body *mir.Body, value *mir.Value) (*objectInfo, hir.MemberBindingKind, error) {
	if value == nil {
		return nil, "", fmt.Errorf("nil object member")
	}
	if value.MemberBinding != nil {
		info := e.objectInfo[value.MemberBinding.ObjectName]
		if info == nil {
			return nil, "", fmt.Errorf("unknown object %q", value.MemberBinding.ObjectName)
		}
		return info, value.MemberBinding.Kind, nil
	}
	if body == nil || value.Object == 0 {
		return nil, "", fmt.Errorf("member value %q is missing binding", value.Member)
	}
	objectValue := body.Value(value.Object)
	if objectValue == nil {
		return nil, "", fmt.Errorf("unknown member object value %d", value.Object)
	}
	info, ok := e.objectInfoForResolvedType(e.resolveType(objectValue.Type))
	if !ok {
		return nil, "", fmt.Errorf("member value %q is missing binding", value.Member)
	}
	if e.exprIsObjectTypeValue(body, value.Object) && value.Member == "new" && info.node != nil && info.node.Constructor != nil {
		return info, hir.MemberBindingObjectConstructor, nil
	}
	if _, ok := info.methodNames[value.Member]; ok {
		return info, hir.MemberBindingObjectMethod, nil
	}
	if _, err := e.objectFieldType(info, value.Member); err == nil {
		return info, hir.MemberBindingObjectField, nil
	}
	return nil, "", fmt.Errorf("member value %q is missing binding", value.Member)
}

func (e *emitter) objectMemberCName(body *mir.Body, value *mir.Value) (string, error) {
	info, kind, err := e.objectMemberInfo(body, value)
	if err != nil {
		return "", err
	}
	switch kind {
	case hir.MemberBindingObjectConstructor:
		if info.constructorName == "" {
			return "", fmt.Errorf("object %q has no constructor", info.name)
		}
		return info.constructorName, nil
	case hir.MemberBindingObjectMethod:
		name, ok := info.methodNames[value.Member]
		if !ok {
			return "", fmt.Errorf("object %q has no method %q", info.name, value.Member)
		}
		return name, nil
	default:
		return "", fmt.Errorf("unsupported object member binding %q", kind)
	}
}

func (e *emitter) exprIsObjectTypeValue(body *mir.Body, valueID int) bool {
	if body == nil || valueID == 0 {
		return false
	}
	value := body.Value(valueID)
	if value == nil {
		return false
	}
	if _, ok := e.objectInfoForResolvedType(e.resolveType(value.Type)); !ok {
		return false
	}
	switch value.Kind {
	case mir.ValueBindingRef:
		return value.Binding != nil && value.Binding.Kind == hir.BindingObjectType
	case mir.ValueMember:
		return value.MemberBinding != nil && value.MemberBinding.Kind == hir.MemberBindingObjectConstructor
	default:
		return false
	}
}

func (e *emitter) tupleTypeName(types []hir.Type) (string, error) {
	parts := make([]string, 0, len(types))
	for _, typ := range types {
		typeName, err := e.cType(typ)
		if err != nil {
			return "", err
		}
		parts = append(parts, typeName)
	}
	key := strings.Join(parts, "|")
	if name, ok := e.tupleByKey[key]; ok {
		return name, nil
	}
	name := fmt.Sprintf("gwen_tuple_%d", len(e.tupleOrder)+1)
	e.tupleByKey[key] = name
	e.tupleOrder = append(e.tupleOrder, key)
	fields := make([]string, 0, len(parts))
	for idx := range parts {
		fields = append(fields, tupleFieldName(idx))
	}
	e.tupleFields[key] = fields
	e.tupleTypes[key] = append([]hir.Type(nil), types...)
	return name, nil
}

func (e *emitter) writeLine(line string) {
	e.out.WriteString(line)
	e.out.WriteByte('\n')
}

func (e *emitter) writeBlock(block string) {
	e.out.WriteString(block)
	if block == "" || !strings.HasSuffix(block, "\n") {
		e.out.WriteByte('\n')
	}
}

func slotName(id int) string {
	return fmt.Sprintf("slot_%d", id)
}

func slotInitName(id int) string {
	return fmt.Sprintf("slot_init_%d", id)
}

func (e *emitter) isGlobalBinding(bindingID int) bool {
	_, ok := e.globalSlots[bindingID]
	return ok
}

func (e *emitter) globalName(bindingID int) string {
	return fmt.Sprintf("gwen_global_%d", bindingID)
}

func globalInitName(bindingID int) string {
	return fmt.Sprintf("gwen_global_init_%d", bindingID)
}

func tempName(id int) string {
	return fmt.Sprintf("tmp_%d", id)
}

func loopStartedName(blockID int) string {
	return fmt.Sprintf("gwen_loop_started_%d", blockID)
}

func loopCurrentName(blockID int) string {
	return fmt.Sprintf("gwen_loop_current_%d", blockID)
}

func loopEndName(blockID int) string {
	return fmt.Sprintf("gwen_loop_end_%d", blockID)
}

func loopStepName(blockID int) string {
	return fmt.Sprintf("gwen_loop_step_%d", blockID)
}

func loopIndexName(blockID int) string {
	return fmt.Sprintf("gwen_loop_index_%d", blockID)
}

func tupleFieldName(index int) string {
	return fmt.Sprintf("f%d", index)
}

func tupleToValueFuncName(typeName string) string {
	return typeName + "_to_value"
}

func objectFieldName(name string) string {
	return sanitizeName(name)
}

func moduleValueKey(moduleName, valueName string) string {
	return moduleName + ":" + valueName
}

func funcCallHelperName(typeName string) string {
	return typeName + "_call"
}

func closureAdapterName(actualName string) string {
	return actualName + "_closure_call"
}

func closureConstructorName(actualName string) string {
	return actualName + "_closure_new"
}

func builtinClosureKey(moduleName, name string, funcType *hir.FuncType) string {
	return moduleName + "|" + name + "|" + typeLabel(funcType)
}

func canonicalBuiltinModuleName(moduleName, name string) string {
	if moduleName != "" {
		return moduleName
	}
	switch name {
	case "split", "join", "substring", "startswith", "endswith", "contains", "trim", "replace":
		return "string"
	case "abs", "min", "max", "sqrt", "floor", "ceil":
		return "math"
	default:
		return ""
	}
}

func builtinValueIdentity(value *mir.Value) (string, string, bool) {
	if value == nil {
		return "", "", false
	}
	switch value.Kind {
	case mir.ValueBindingRef:
		if value.Binding == nil {
			return "", "", false
		}
		switch value.Binding.Kind {
		case hir.BindingBuiltin:
			return "", value.Binding.Name, true
		case hir.BindingImported:
			if isStdlibModuleName(value.Binding.SourceModule) {
				return value.Binding.SourceModule, value.Binding.Name, true
			}
		}
	case mir.ValueMember:
		if value.MemberBinding != nil && value.MemberBinding.Kind == hir.MemberBindingModuleValue && isStdlibModuleName(value.MemberBinding.OwnerName) {
			return value.MemberBinding.OwnerName, value.Member, true
		}
	}
	return "", "", false
}

func cloneFuncTypeNode(fn *hir.FuncType) *hir.FuncType {
	if fn == nil {
		return nil
	}
	return &hir.FuncType{
		Params:  append([]hir.Type(nil), fn.Params...),
		Returns: append([]hir.Type(nil), fn.Returns...),
	}
}

func boundMethodClosureAdapterName(actualName string) string {
	return actualName + "_bound_closure_call"
}

func boundMethodClosureConstructorName(actualName string) string {
	return actualName + "_bound_closure_new"
}

func closureEnvTypeName(actualName string) string {
	return actualName + "_closure_env"
}

func boundMethodClosureEnvTypeName(actualName string) string {
	return actualName + "_bound_closure_env"
}

func closureCaptureFieldName(slotID int) string {
	return fmt.Sprintf("capture_%d", slotID)
}

func (e *emitter) funcCName(fn *mir.Func) (string, error) {
	if fn == nil {
		return "", fmt.Errorf("nil function")
	}
	if fn.Binding != nil && fn.Binding.ID != 0 {
		if name, ok := e.funcBinding[fn.Binding.ID]; ok {
			return name, nil
		}
	}
	if name, ok := e.funcNames[fn.Name]; ok {
		return name, nil
	}
	return "", fmt.Errorf("missing C name for function %q", fn.Name)
}

func (e *emitter) funcForBinding(binding *hir.NameBinding) (*mir.Func, string, error) {
	if binding == nil {
		return nil, "", fmt.Errorf("missing binding")
	}
	switch binding.Kind {
	case hir.BindingFunc:
		if binding.ID != 0 {
			if fn := e.funcByBinding[binding.ID]; fn != nil {
				name, err := e.funcCName(fn)
				return fn, name, err
			}
			if name, ok := e.funcBinding[binding.ID]; ok {
				return nil, name, nil
			}
		}
		if fn := e.funcByName[binding.Name]; fn != nil {
			name, err := e.funcCName(fn)
			return fn, name, err
		}
		if name, ok := e.funcNames[binding.Name]; ok {
			return nil, name, nil
		}
		return nil, "", fmt.Errorf("unknown function binding %q", binding.Name)
	case hir.BindingImported:
		key := moduleValueKey(binding.SourceModule, binding.Name)
		name, ok := e.moduleValues[key]
		if !ok {
			return nil, "", fmt.Errorf("unknown imported binding %q from module %q", binding.Name, binding.SourceModule)
		}
		return e.moduleFuncs[key], name, nil
	default:
		return nil, "", fmt.Errorf("unsupported function binding kind %q", binding.Kind)
	}
}

func sanitizeName(name string) string {
	if name == "" {
		return "anon"
	}
	var out strings.Builder
	for idx, ch := range name {
		if idx == 0 {
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
				out.WriteRune(ch)
				continue
			}
			out.WriteByte('_')
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			out.WriteRune(ch)
			continue
		}
		out.WriteByte('_')
	}
	return out.String()
}

func quoteCString(value string) string {
	return strconv.Quote(value)
}

func signatureType(params []*hir.Param, returns []hir.Type) *hir.FuncType {
	paramTypes := make([]hir.Type, 0, len(params))
	for _, param := range params {
		paramTypes = append(paramTypes, param.Type)
	}
	return &hir.FuncType{
		Params:  paramTypes,
		Returns: append([]hir.Type(nil), returns...),
	}
}

func builtinExpectedTypes(name string) []hir.Type {
	switch name {
	case "read":
		return []hir.Type{&hir.NamedType{Name: "string"}}
	case "split":
		return []hir.Type{&hir.NamedType{Name: "string"}, &hir.NamedType{Name: "string"}}
	case "join":
		return []hir.Type{nil, &hir.NamedType{Name: "string"}}
	case "substring":
		return []hir.Type{&hir.NamedType{Name: "string"}, &hir.NamedType{Name: "int"}, &hir.NamedType{Name: "int"}}
	case "trim":
		return []hir.Type{&hir.NamedType{Name: "string"}}
	case "replace":
		return []hir.Type{&hir.NamedType{Name: "string"}, &hir.NamedType{Name: "string"}, &hir.NamedType{Name: "string"}}
	case "readfile", "readdir", "path.basename", "path.dirname":
		return []hir.Type{&hir.NamedType{Name: "string"}}
	case "os.getenv":
		return []hir.Type{&hir.NamedType{Name: "string"}}
	case "time.sleep":
		return []hir.Type{&hir.NamedType{Name: "int"}}
	case "writefile", "appendfile", "path.joinpath":
		return []hir.Type{&hir.NamedType{Name: "string"}, &hir.NamedType{Name: "string"}}
	case "json.parseobject", "json.parsearray":
		return []hir.Type{&hir.NamedType{Name: "string"}}
	case "json.stringify", "json.isnull":
		return []hir.Type{&hir.NamedType{Name: "dynamic"}}
	case "startswith", "endswith", "contains":
		return []hir.Type{&hir.NamedType{Name: "string"}, &hir.NamedType{Name: "string"}}
	default:
		return nil
	}
}

func mapBinaryOp(op string) string {
	switch op {
	case "+":
		return "+"
	case "-":
		return "-"
	case "*":
		return "*"
	case "/":
		return "/"
	case "mod":
		return "%"
	case "=":
		return "=="
	case "!=":
		return "!="
	case "<":
		return "<"
	case ">":
		return ">"
	case "<=":
		return "<="
	case ">=":
		return ">="
	case "and":
		return "&&"
	case "or":
		return "||"
	default:
		return ""
	}
}

func typeLabel(typ hir.Type) string {
	switch node := typ.(type) {
	case nil:
		return "<nil>"
	case *hir.NamedType:
		return node.Name
	case *hir.GenericType:
		parts := make([]string, 0, len(node.Args))
		for _, arg := range node.Args {
			parts = append(parts, typeLabel(arg))
		}
		return node.Base + "[" + strings.Join(parts, ", ") + "]"
	case *hir.FuncType:
		params := make([]string, 0, len(node.Params))
		for _, param := range node.Params {
			params = append(params, typeLabel(param))
		}
		returns := make([]string, 0, len(node.Returns))
		for _, ret := range node.Returns {
			returns = append(returns, typeLabel(ret))
		}
		return "(" + strings.Join(params, ", ") + ") -> " + strings.Join(returns, ", ")
	default:
		return fmt.Sprintf("%T", typ)
	}
}

func namedTypeName(typ hir.Type) string {
	named, ok := typ.(*hir.NamedType)
	if !ok {
		return ""
	}
	return named.Name
}

func isStringType(typ hir.Type) bool {
	return namedTypeName(typ) == "string"
}

func isDynamicValueType(typ hir.Type) bool {
	switch namedTypeName(typ) {
	case "dynamic", "list", "dict", "JsonNull":
		return true
	default:
		return false
	}
}

func isBareListType(typ hir.Type) bool {
	return namedTypeName(typ) == "list"
}

func isBareDictType(typ hir.Type) bool {
	return namedTypeName(typ) == "dict"
}

func isBoolType(typ hir.Type) bool {
	return namedTypeName(typ) == "bool"
}

func isFloatType(typ hir.Type) bool {
	switch namedTypeName(typ) {
	case "float", "float32", "float64":
		return true
	default:
		return false
	}
}

func isSignedIntegerType(typ hir.Type) bool {
	switch namedTypeName(typ) {
	case "int", "int64", "int32", "int16", "int8":
		return true
	default:
		return false
	}
}

func isUnsignedIntegerType(typ hir.Type) bool {
	switch namedTypeName(typ) {
	case "uint32", "uint16", "uint8":
		return true
	default:
		return false
	}
}

func isNumericType(typ hir.Type) bool {
	return isSignedIntegerType(typ) || isUnsignedIntegerType(typ) || isFloatType(typ)
}

func isStdlibModuleName(name string) bool {
	switch name {
	case "list", "string", "math", "dict", "io", "path", "os", "time", "json", "http", "state", "sqlite":
		return true
	default:
		return false
	}
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

func isValueMatchSubjectType(typ hir.Type) bool {
	return isStringType(typ) || isNumericOrBoolType(typ)
}

func listItemType(typ hir.Type) hir.Type {
	generic, ok := typ.(*hir.GenericType)
	if !ok || generic.Base != "list" || len(generic.Args) != 1 {
		return nil
	}
	return generic.Args[0]
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

func moneyCurrencyName(typ *hir.GenericType) string {
	if typ == nil || len(typ.Args) == 0 {
		return ""
	}
	return typeLabel(typ.Args[0])
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
		return &hir.NamedType{Name: "string"}
	}
	candidate := generic.Args[1]
	for _, arg := range generic.Args[2:] {
		if typeLabel(arg) != typeLabel(candidate) {
			return nil
		}
	}
	return candidate
}

func isHTTPReplyResultType(typ hir.Type) bool {
	generic, ok := typ.(*hir.GenericType)
	if !ok || generic.Base != "result" || len(generic.Args) == 0 {
		return false
	}
	if namedTypeName(generic.Args[0]) != "HttpReply" {
		return false
	}
	errType := resultErrType(generic)
	return errType != nil && namedTypeName(errType) == "string"
}

func dictKeyEqExpr(left, right string, keyType hir.Type) (string, error) {
	switch {
	case isStringType(keyType):
		return fmt.Sprintf("gwen_string_eq(%s, %s)", left, right), nil
	case isNumericOrBoolType(keyType):
		return fmt.Sprintf("%s == %s", left, right), nil
	default:
		return "", fmt.Errorf("unsupported dict key comparison type %s", typeLabel(keyType))
	}
}

func isNumericOrBoolType(typ hir.Type) bool {
	return isNumericType(typ) || isBoolType(typ)
}

func isIntegerType(typ hir.Type) bool {
	return isSignedIntegerType(typ) || isUnsignedIntegerType(typ)
}

func valueMatchEquals(subjectExpr, patternExpr string, subjectType hir.Type) (string, error) {
	switch {
	case isStringType(subjectType):
		return fmt.Sprintf("gwen_string_eq(%s, %s)", subjectExpr, patternExpr), nil
	case isNumericOrBoolType(subjectType):
		return fmt.Sprintf("%s == %s", subjectExpr, patternExpr), nil
	default:
		return "", fmt.Errorf("unsupported match equality type %s", typeLabel(subjectType))
	}
}

func inlineRangePatternBounds(pattern *hir.Binary) (string, string, error) {
	if pattern == nil || pattern.Op != "to" {
		return "", "", fmt.Errorf("expected range pattern")
	}
	start, err := inlineIntPatternExpr(pattern.Left)
	if err != nil {
		return "", "", fmt.Errorf("range start: %w", err)
	}
	end, err := inlineIntPatternExpr(pattern.Right)
	if err != nil {
		return "", "", fmt.Errorf("range end: %w", err)
	}
	return start, end, nil
}

func inlineIntPatternExpr(expr hir.Expr) (string, error) {
	switch node := expr.(type) {
	case *hir.IntLiteral:
		return fmt.Sprintf("%dLL", node.Value), nil
	case *hir.Unary:
		if node.Op != "-" {
			return "", fmt.Errorf("unsupported int pattern unary op %q", node.Op)
		}
		operand, err := inlineIntPatternExpr(node.Operand)
		if err != nil {
			return "", err
		}
		return "(-" + operand + ")", nil
	default:
		return "", fmt.Errorf("unsupported int pattern expression %T", expr)
	}
}

func inlineValuePatternExpr(expr hir.Expr) (string, error) {
	switch node := expr.(type) {
	case *hir.IntLiteral:
		return fmt.Sprintf("%dLL", node.Value), nil
	case *hir.FloatLiteral:
		return strconv.FormatFloat(node.Value, 'g', -1, 64), nil
	case *hir.StringLiteral:
		return quoteCString(node.Value), nil
	case *hir.BoolLiteral:
		if node.Value {
			return "true", nil
		}
		return "false", nil
	case *hir.Unary:
		if node.Op != "-" {
			return "", fmt.Errorf("unsupported pattern unary op %q", node.Op)
		}
		operand, err := inlineValuePatternExpr(node.Operand)
		if err != nil {
			return "", err
		}
		return "(-" + operand + ")", nil
	default:
		return "", fmt.Errorf("unsupported value match pattern expression %T", expr)
	}
}

func dictHasKeyFuncName(typeName string) string {
	return typeName + "_haskey"
}

func dictGetFuncName(typeName string) string {
	return typeName + "_get"
}

func dictIndexFuncName(typeName string) string {
	return typeName + "_index"
}

func listJoinFuncName(typeName string) string {
	return typeName + "_join"
}

func listAppendFuncName(typeName string) string {
	return typeName + "_append"
}

func listRemoveAtFuncName(typeName string) string {
	return typeName + "_removeat"
}

func listCloneFuncName(typeName string) string {
	return typeName + "_clone"
}

func listToValueFuncName(typeName string) string {
	return typeName + "_to_value"
}

func listFromValueFuncName(typeName string) string {
	return typeName + "_from_value"
}

func listSortFuncName(typeName string, descending bool) string {
	if descending {
		return typeName + "_sort_desc"
	}
	return typeName + "_sort_asc"
}

func listSortCmpFuncName(typeName string, descending bool) string {
	if descending {
		return typeName + "_sort_cmp_desc"
	}
	return typeName + "_sort_cmp_asc"
}

func dictSetFuncName(typeName string) string {
	return typeName + "_set"
}

func dictCloneFuncName(typeName string) string {
	return typeName + "_clone"
}

func dictToValueFuncName(typeName string) string {
	return typeName + "_to_value"
}

func dictFromValueFuncName(typeName string) string {
	return typeName + "_from_value"
}

func dictKeysFuncName(typeName string) string {
	return typeName + "_keys"
}

func dictValuesFuncName(typeName string) string {
	return typeName + "_values"
}

func resultCloneFuncName(typeName string) string {
	return typeName + "_clone"
}

func cellNewFuncName(typeName string) string {
	return typeName + "_cell_new"
}

func cellGetFuncName(typeName string) string {
	return typeName + "_cell_get"
}

func cellSetFuncName(typeName string) string {
	return typeName + "_cell_set"
}

func cellCloneFuncName(typeName string) string {
	return typeName + "_clone"
}
