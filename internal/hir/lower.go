package hir

import (
	"fmt"

	"github.com/Cass-ette/gwen-lang/internal/ast"
)

type lowerer struct {
	nextLoopID *int
	loops      []loopFrame
}

type loopFrame struct {
	id   int
	name string
}

func LowerProgram(program *ast.Program) (*Program, error) {
	return newLowerer().lowerProgram(program)
}

func newLowerer() *lowerer {
	next := 1
	return &lowerer{nextLoopID: &next}
}

func (l *lowerer) childFunction() *lowerer {
	return &lowerer{nextLoopID: l.nextLoopID}
}

func (l *lowerer) withLoop(name string) *lowerer {
	id := *l.nextLoopID
	*l.nextLoopID = id + 1
	child := &lowerer{
		nextLoopID: l.nextLoopID,
		loops:      append([]loopFrame{}, l.loops...),
	}
	child.loops = append(child.loops, loopFrame{id: id, name: name})
	return child
}

func (l *lowerer) resolveLoopTarget(name string, line int) (int, error) {
	for idx := len(l.loops) - 1; idx >= 0; idx-- {
		loop := l.loops[idx]
		if name == "" || loop.name == name {
			return loop.id, nil
		}
	}
	if name == "" {
		return 0, fmt.Errorf("leave/next at line %d has no enclosing loop", line)
	}
	return 0, fmt.Errorf("leave/next at line %d targets unknown loop %q", line, name)
}

func (l *lowerer) lowerProgram(program *ast.Program) (*Program, error) {
	if program == nil {
		return nil, fmt.Errorf("cannot lower nil program")
	}

	hirProgram := &Program{}
	for _, stmt := range program.Statements {
		switch node := stmt.(type) {
		case *ast.UseStmt:
			hirProgram.Items = append(hirProgram.Items, lowerUse(node))
		case *ast.FuncDef:
			fn, err := l.lowerFunc(node)
			if err != nil {
				return nil, err
			}
			hirProgram.Items = append(hirProgram.Items, fn)
		case *ast.ModuleDef:
			mod, err := l.lowerModule(node)
			if err != nil {
				return nil, err
			}
			hirProgram.Items = append(hirProgram.Items, mod)
		case *ast.ObjectDef:
			obj, err := l.lowerObject(node)
			if err != nil {
				return nil, err
			}
			hirProgram.Items = append(hirProgram.Items, obj)
		case *ast.TypeAlias:
			alias, err := lowerTypeAlias(node)
			if err != nil {
				return nil, err
			}
			hirProgram.Items = append(hirProgram.Items, alias)
		default:
			lowered, err := l.lowerStmt(stmt)
			if err != nil {
				return nil, err
			}
			hirProgram.Items = append(hirProgram.Items, &StmtItem{Stmt: lowered})
		}
	}
	return hirProgram, nil
}

func (l *lowerer) lowerModule(node *ast.ModuleDef) (*Module, error) {
	mod := &Module{
		Name: node.Name,
		Line: node.Line,
	}
	for _, item := range node.Body {
		switch decl := item.(type) {
		case *ast.UseStmt:
			mod.Items = append(mod.Items, lowerUse(decl))
		case *ast.FuncDef:
			fn, err := l.lowerFunc(decl)
			if err != nil {
				return nil, err
			}
			mod.Items = append(mod.Items, fn)
		case *ast.ObjectDef:
			obj, err := l.lowerObject(decl)
			if err != nil {
				return nil, err
			}
			mod.Items = append(mod.Items, obj)
		case *ast.TypeAlias:
			alias, err := lowerTypeAlias(decl)
			if err != nil {
				return nil, err
			}
			mod.Items = append(mod.Items, alias)
		default:
			return nil, fmt.Errorf("unsupported module item %T in module %q", item, node.Name)
		}
	}
	return mod, nil
}

func lowerUse(node *ast.UseStmt) *Use {
	return &Use{
		Module: node.Module,
		Names:  append([]string(nil), node.Names...),
		Line:   node.Line,
	}
}

func (l *lowerer) lowerFunc(node *ast.FuncDef) (*Func, error) {
	params, err := lowerParams(node.Params)
	if err != nil {
		return nil, err
	}
	returns, err := lowerReturnTypes(node.ReturnType)
	if err != nil {
		return nil, err
	}
	body, err := l.childFunction().lowerBlock(node.Body)
	if err != nil {
		return nil, err
	}
	return &Func{
		Name:     node.Name,
		Params:   params,
		Returns:  returns,
		Body:     body,
		Exported: node.Exported,
		Line:     node.Line,
	}, nil
}

func (l *lowerer) lowerObject(node *ast.ObjectDef) (*Object, error) {
	fields := make([]*Field, 0, len(node.Fields))
	for _, field := range node.Fields {
		fieldType, err := lowerType(field.TypeAnnotation)
		if err != nil {
			return nil, err
		}
		fields = append(fields, &Field{
			Name: field.Name,
			Type: fieldType,
			Line: field.Line,
		})
	}

	var constructor *Constructor
	if node.Constructor != nil {
		lowered, err := l.lowerConstructor(node.Name, node.Constructor)
		if err != nil {
			return nil, err
		}
		constructor = lowered
	}

	methods := make([]*Method, 0, len(node.Methods))
	for _, method := range node.Methods {
		lowered, err := l.lowerMethod(method)
		if err != nil {
			return nil, err
		}
		methods = append(methods, lowered)
	}

	return &Object{
		Name:        node.Name,
		Fields:      fields,
		Constructor: constructor,
		Methods:     methods,
		Exported:    node.Exported,
		Line:        node.Line,
	}, nil
}

func (l *lowerer) lowerConstructor(objectName string, node *ast.ConstructorDef) (*Constructor, error) {
	params, err := lowerParams(node.Params)
	if err != nil {
		return nil, err
	}
	body, err := l.childFunction().lowerBlock(node.Body)
	if err != nil {
		return nil, err
	}

	returns := []Type{&NamedType{Name: objectName, Line: node.Line}}
	if node.ReturnType != nil {
		explicitReturns, err := lowerReturnTypes(node.ReturnType)
		if err != nil {
			return nil, err
		}
		if !singleNamedType(explicitReturns, objectName) {
			return nil, fmt.Errorf("constructor %q must return %q", objectName+".new", objectName)
		}
	}

	return &Constructor{
		Name:    objectName,
		Params:  params,
		Returns: returns,
		Body:    body,
		Line:    node.Line,
	}, nil
}

func (l *lowerer) lowerMethod(node *ast.MethodDef) (*Method, error) {
	params, err := lowerParams(node.Params)
	if err != nil {
		return nil, err
	}
	returns, err := lowerReturnTypes(node.ReturnType)
	if err != nil {
		return nil, err
	}
	body, err := l.childFunction().lowerBlock(node.Body)
	if err != nil {
		return nil, err
	}
	return &Method{
		Name:    node.Name,
		Params:  params,
		Returns: returns,
		Body:    body,
		Line:    node.Line,
	}, nil
}

func lowerTypeAlias(node *ast.TypeAlias) (*TypeAlias, error) {
	target, err := lowerType(node.Target)
	if err != nil {
		return nil, err
	}
	return &TypeAlias{
		Name:     node.Name,
		Target:   target,
		Exported: node.Exported,
		Line:     node.Line,
	}, nil
}

func lowerParams(params []*ast.Param) ([]*Param, error) {
	lowered := make([]*Param, 0, len(params))
	for _, param := range params {
		var typeNode Type
		var err error
		if param.TypeName != nil {
			typeNode, err = lowerType(param.TypeName)
			if err != nil {
				return nil, err
			}
		}
		var defaultValue Expr
		if param.Default != nil {
			defaultValue, err = lowerExpr(param.Default)
			if err != nil {
				return nil, err
			}
		}
		lowered = append(lowered, &Param{
			Name:    param.Name,
			Type:    typeNode,
			Default: defaultValue,
			Line:    param.Line,
		})
	}
	return lowered, nil
}

func lowerReturnTypes(node any) ([]Type, error) {
	if node == nil {
		return nil, nil
	}
	if multi, ok := node.([]any); ok {
		returns := make([]Type, 0, len(multi))
		for _, item := range multi {
			lowered, err := lowerType(item)
			if err != nil {
				return nil, err
			}
			returns = append(returns, lowered)
		}
		return returns, nil
	}
	lowered, err := lowerType(node)
	if err != nil {
		return nil, err
	}
	return []Type{lowered}, nil
}

func lowerType(node any) (Type, error) {
	switch t := node.(type) {
	case *ast.TypeName:
		return &NamedType{Name: t.Name, Line: t.Line}, nil
	case *ast.GenericType:
		args := make([]Type, 0, len(t.Params))
		for _, arg := range t.Params {
			lowered, err := lowerType(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, lowered)
		}
		return &GenericType{Base: t.Base, Args: args, Line: t.Line}, nil
	case *ast.FuncType:
		params := make([]Type, 0, len(t.ParamTypes))
		for _, param := range t.ParamTypes {
			lowered, err := lowerType(param)
			if err != nil {
				return nil, err
			}
			params = append(params, lowered)
		}
		returns, err := lowerReturnTypes(t.ReturnType)
		if err != nil {
			return nil, err
		}
		return &FuncType{
			Params:  params,
			Returns: returns,
			Line:    t.Line,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type node %T", node)
	}
}

func (l *lowerer) lowerBlock(nodes []any) ([]Stmt, error) {
	stmts := make([]Stmt, 0, len(nodes))
	for _, node := range nodes {
		lowered, err := l.lowerStmt(node)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, lowered)
	}
	return stmts, nil
}

func (l *lowerer) lowerStmt(node any) (Stmt, error) {
	switch stmt := node.(type) {
	case *ast.UseStmt:
		return lowerUse(stmt), nil

	case *ast.Assignment:
		targets, err := lowerExprList(stmt.Targets)
		if err != nil {
			return nil, err
		}
		values, err := lowerExprList(stmt.Values)
		if err != nil {
			return nil, err
		}
		return &Assign{
			Targets: targets,
			Values:  values,
			Line:    stmt.Line,
		}, nil

	case *ast.VarDecl:
		return lowerVar(stmt)

	case *ast.VarBlock:
		decls := make([]*Var, 0, len(stmt.Decls))
		for _, decl := range stmt.Decls {
			lowered, err := lowerVar(decl)
			if err != nil {
				return nil, err
			}
			decls = append(decls, lowered)
		}
		var defaultValue Expr
		var err error
		if stmt.DefaultValue != nil {
			defaultValue, err = lowerExpr(stmt.DefaultValue)
			if err != nil {
				return nil, err
			}
		}
		return &VarBlock{
			Decls:        decls,
			DefaultMode:  stmt.DefaultMode,
			DefaultValue: defaultValue,
			Line:         stmt.Line,
		}, nil

	case *ast.ReturnStmt:
		values, err := lowerExprBundle(stmt.Value)
		if err != nil {
			return nil, err
		}
		return &Return{Values: values, Line: stmt.Line}, nil

	case *ast.PassStmt:
		return &Pass{Line: stmt.Line}, nil

	case *ast.LeaveStmt:
		targetID, err := l.resolveLoopTarget(stmt.Name, stmt.Line)
		if err != nil {
			return nil, err
		}
		return &Leave{Name: stmt.Name, TargetID: targetID, Line: stmt.Line}, nil

	case *ast.NextStmt:
		targetID, err := l.resolveLoopTarget(stmt.Name, stmt.Line)
		if err != nil {
			return nil, err
		}
		return &Next{Name: stmt.Name, TargetID: targetID, Line: stmt.Line}, nil

	case *ast.IfStmt:
		condition, err := lowerExpr(stmt.Condition)
		if err != nil {
			return nil, err
		}
		body, err := l.lowerBlock(stmt.Body)
		if err != nil {
			return nil, err
		}
		elifs := make([]IfBranch, 0, len(stmt.Elifs))
		for _, branch := range stmt.Elifs {
			branchCondition, err := lowerExpr(branch.Condition)
			if err != nil {
				return nil, err
			}
			loweredBody, err := l.lowerBlock(branch.Body)
			if err != nil {
				return nil, err
			}
			elifs = append(elifs, IfBranch{
				Condition: branchCondition,
				Body:      loweredBody,
			})
		}
		elseBody, err := l.lowerBlock(stmt.ElseBody)
		if err != nil {
			return nil, err
		}
		return &If{
			Condition: condition,
			Body:      body,
			Elifs:     elifs,
			ElseBody:  elseBody,
			Line:      stmt.Line,
		}, nil

	case *ast.WhileStmt:
		condition, err := lowerExpr(stmt.Condition)
		if err != nil {
			return nil, err
		}
		loopLowerer := l.withLoop(stmt.Name)
		body, err := loopLowerer.lowerBlock(stmt.Body)
		if err != nil {
			return nil, err
		}
		return &While{
			Condition: condition,
			Name:      stmt.Name,
			LoopID:    loopLowerer.loops[len(loopLowerer.loops)-1].id,
			Body:      body,
			Line:      stmt.Line,
		}, nil

	case *ast.ForRangeStmt:
		start, err := lowerExpr(stmt.Start)
		if err != nil {
			return nil, err
		}
		end, err := lowerExpr(stmt.End)
		if err != nil {
			return nil, err
		}
		var step Expr
		if stmt.Step != nil {
			step, err = lowerExpr(stmt.Step)
			if err != nil {
				return nil, err
			}
		}
		loopLowerer := l.withLoop(stmt.Name)
		body, err := loopLowerer.lowerBlock(stmt.Body)
		if err != nil {
			return nil, err
		}
		return &ForRange{
			Var:       stmt.Var,
			Start:     start,
			End:       end,
			Step:      step,
			Direction: stmt.Direction,
			Name:      stmt.Name,
			LoopID:    loopLowerer.loops[len(loopLowerer.loops)-1].id,
			Body:      body,
			Line:      stmt.Line,
		}, nil

	case *ast.ForEachStmt:
		iterable, err := lowerExpr(stmt.Iterable)
		if err != nil {
			return nil, err
		}
		loopLowerer := l.withLoop(stmt.Name)
		body, err := loopLowerer.lowerBlock(stmt.Body)
		if err != nil {
			return nil, err
		}
		return &ForEach{
			Var:      stmt.Var,
			Iterable: iterable,
			IndexVar: stmt.IndexVar,
			Name:     stmt.Name,
			LoopID:   loopLowerer.loops[len(loopLowerer.loops)-1].id,
			Body:     body,
			Line:     stmt.Line,
		}, nil

	case *ast.MatchStmt:
		subject, err := lowerExpr(stmt.Subject)
		if err != nil {
			return nil, err
		}
		cases := make([]*MatchCase, 0, len(stmt.Cases))
		for _, clause := range stmt.Cases {
			patterns, err := lowerExprList(clause.Patterns)
			if err != nil {
				return nil, err
			}
			body, err := l.lowerBlock(clause.Body)
			if err != nil {
				return nil, err
			}
			cases = append(cases, &MatchCase{
				Patterns: patterns,
				Body:     body,
				Line:     clause.Line,
			})
		}
		elseBody, err := l.lowerBlock(stmt.ElseBody)
		if err != nil {
			return nil, err
		}
		return &Match{
			Subject:  subject,
			Cases:    cases,
			ElseBody: elseBody,
			Line:     stmt.Line,
		}, nil

	case *ast.ParallelStmt:
		body, err := l.lowerBlock(stmt.Body)
		if err != nil {
			return nil, err
		}
		return &Parallel{
			Body:      body,
			ResultVar: stmt.ResultVar,
			AllowFail: stmt.AllowFail,
			Line:      stmt.Line,
		}, nil

	case *ast.GlobalStmt:
		value, err := lowerExpr(stmt.Value)
		if err != nil {
			return nil, err
		}
		return &Global{Name: stmt.Name, Value: value, Line: stmt.Line}, nil

	case *ast.ArenaStmt:
		body, err := l.lowerBlock(stmt.Body)
		if err != nil {
			return nil, err
		}
		return &Arena{Name: stmt.Name, Body: body, Line: stmt.Line}, nil

	case *ast.TagStmt:
		return &Tag{Name: stmt.Name, Line: stmt.Line}, nil

	case *ast.ExprStmt:
		expr, err := lowerExpr(stmt.Expr)
		if err != nil {
			return nil, err
		}
		return &ExprStmt{Expr: expr, Line: stmt.Line}, nil

	case *ast.FuncDef:
		decl, err := l.lowerFunc(stmt)
		if err != nil {
			return nil, err
		}
		return &DeclStmt{Decl: decl}, nil

	case *ast.ObjectDef:
		decl, err := l.lowerObject(stmt)
		if err != nil {
			return nil, err
		}
		return &DeclStmt{Decl: decl}, nil

	case *ast.TypeAlias:
		decl, err := lowerTypeAlias(stmt)
		if err != nil {
			return nil, err
		}
		return &DeclStmt{Decl: decl}, nil

	case *ast.ModuleDef:
		decl, err := l.lowerModule(stmt)
		if err != nil {
			return nil, err
		}
		return &DeclStmt{Decl: decl}, nil

	default:
		return nil, fmt.Errorf("unsupported statement node %T", node)
	}
}

func lowerVar(node *ast.VarDecl) (*Var, error) {
	var typeNode Type
	var err error
	if node.TypeName != nil {
		typeNode, err = lowerType(node.TypeName)
		if err != nil {
			return nil, err
		}
	}
	var value Expr
	if node.Value != nil {
		value, err = lowerExpr(node.Value)
		if err != nil {
			return nil, err
		}
	}
	return &Var{
		Name:     node.Name,
		Type:     typeNode,
		Value:    value,
		IsConst:  node.IsConst,
		IsUninit: node.IsUninit,
		Line:     node.Line,
	}, nil
}

func lowerExprList(nodes []any) ([]Expr, error) {
	lowered := make([]Expr, 0, len(nodes))
	for _, node := range nodes {
		expr, err := lowerExpr(node)
		if err != nil {
			return nil, err
		}
		lowered = append(lowered, expr)
	}
	return lowered, nil
}

func lowerExprBundle(node any) ([]Expr, error) {
	if node == nil {
		return nil, nil
	}
	if multi, ok := node.([]any); ok {
		return lowerExprList(multi)
	}
	expr, err := lowerExpr(node)
	if err != nil {
		return nil, err
	}
	return []Expr{expr}, nil
}

func lowerExpr(node any) (Expr, error) {
	switch expr := node.(type) {
	case *ast.IntLiteral:
		return &IntLiteral{Value: expr.Value, Line: expr.Line}, nil
	case *ast.FloatLiteral:
		return &FloatLiteral{Value: expr.Value, Line: expr.Line}, nil
	case *ast.StringLiteral:
		return &StringLiteral{Value: expr.Value, Line: expr.Line}, nil
	case *ast.BoolLiteral:
		return &BoolLiteral{Value: expr.Value, Line: expr.Line}, nil
	case *ast.Identifier:
		return &Ident{Name: expr.Name, Line: expr.Line}, nil
	case string:
		return &Ident{Name: expr}, nil
	case *ast.BinaryOp:
		left, err := lowerExpr(expr.Left)
		if err != nil {
			return nil, err
		}
		right, err := lowerExpr(expr.Right)
		if err != nil {
			return nil, err
		}
		return &Binary{
			Left:  left,
			Op:    expr.Op,
			Right: right,
			Line:  expr.Line,
		}, nil
	case *ast.UnaryOp:
		operand, err := lowerExpr(expr.Operand)
		if err != nil {
			return nil, err
		}
		return &Unary{Op: expr.Op, Operand: operand, Line: expr.Line}, nil
	case *ast.FuncCall:
		callee, err := lowerExpr(expr.Name)
		if err != nil {
			return nil, err
		}
		args, err := lowerExprList(expr.Args)
		if err != nil {
			return nil, err
		}
		return &Call{Callee: callee, Args: args, Line: expr.Line}, nil
	case *ast.MemberAccess:
		object, err := lowerExpr(expr.Object)
		if err != nil {
			return nil, err
		}
		return &Member{Object: object, Member: expr.Member, Line: expr.Line}, nil
	case *ast.IndexAccess:
		object, err := lowerExpr(expr.Object)
		if err != nil {
			return nil, err
		}
		index, err := lowerExpr(expr.Index)
		if err != nil {
			return nil, err
		}
		return &Index{Object: object, Index: index, Line: expr.Line}, nil
	case *ast.Lambda:
		params, err := lowerParams(expr.Params)
		if err != nil {
			return nil, err
		}
		body, err := newLowerer().childFunction().lowerBlock(expr.Body)
		if err != nil {
			return nil, err
		}
		return &Lambda{Params: params, Body: body, Line: expr.Line}, nil
	case *ast.OkExpr:
		value, err := lowerExpr(expr.Value)
		if err != nil {
			return nil, err
		}
		return &Ok{Value: value, Line: expr.Line}, nil
	case *ast.ErrExpr:
		value, err := lowerExpr(expr.Value)
		if err != nil {
			return nil, err
		}
		return &Err{Value: value, Line: expr.Line}, nil
	case *ast.ListLiteral:
		elements, err := lowerExprList(expr.Elements)
		if err != nil {
			return nil, err
		}
		return &List{Elements: elements, Line: expr.Line}, nil
	case *ast.DictLiteral:
		keyType, err := lowerType(expr.KeyType)
		if err != nil {
			return nil, err
		}
		valueType, err := lowerType(expr.ValueType)
		if err != nil {
			return nil, err
		}
		entries := make([]DictEntry, 0, len(expr.Entries))
		for _, entry := range expr.Entries {
			key, err := lowerExpr(entry.Key)
			if err != nil {
				return nil, err
			}
			value, err := lowerExpr(entry.Value)
			if err != nil {
				return nil, err
			}
			entries = append(entries, DictEntry{Key: key, Value: value})
		}
		return &Dict{
			KeyType:   keyType,
			ValueType: valueType,
			Entries:   entries,
			Line:      expr.Line,
		}, nil
	case *ast.AsExpr:
		value, err := lowerExpr(expr.Expr)
		if err != nil {
			return nil, err
		}
		return &Cast{Value: value, TargetName: expr.TypeName, Line: expr.Line}, nil
	case *ast.ObjectLiteral:
		fields := make([]ObjectField, 0, len(expr.Fields))
		for _, field := range expr.Fields {
			value, err := lowerExpr(field.Value)
			if err != nil {
				return nil, err
			}
			fields = append(fields, ObjectField{Name: field.Name, Value: value})
		}
		return &ObjectLiteral{Name: expr.Name, Fields: fields, Line: expr.Line}, nil
	default:
		return nil, fmt.Errorf("unsupported expression node %T", node)
	}
}

func singleNamedType(types []Type, name string) bool {
	if len(types) != 1 {
		return false
	}
	named, ok := types[0].(*NamedType)
	return ok && named.Name == name
}
