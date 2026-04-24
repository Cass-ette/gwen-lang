package frontend

import (
	"os"
	"path/filepath"

	"github.com/Cass-ette/gwen-lang/internal/ast"
	"github.com/Cass-ette/gwen-lang/internal/checker"
	"github.com/Cass-ette/gwen-lang/internal/hir"
	"github.com/Cass-ette/gwen-lang/internal/mir"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

// Unit is the compiler-facing result of the current Gwen frontend pipeline:
// source loading, parsing, and semantic checking.
//
// The point of this type is not to be "the final compiler IR". It is the
// stable handoff boundary that lets the rest of the toolchain stop depending on
// CLI glue code.
type Unit struct {
	Path    string
	Source  string
	Program *ast.Program
	HIR     *hir.Program
	MIR     *mir.Program
}

func AnalyzePath(path string) (*Unit, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return AnalyzeSource(string(source), path)
}

func AnalyzeSource(source string, sourcePath string) (*Unit, error) {
	program, err := parser.Parse(source)
	if err != nil {
		return nil, err
	}
	if err := checker.New().CheckProgram(program, sourcePath); err != nil {
		return nil, err
	}
	lowerInput, err := expandModules(program, sourcePath)
	if err != nil {
		return nil, err
	}
	lowered, err := hir.LowerProgram(lowerInput)
	if err != nil {
		return nil, err
	}
	if err := hir.BindProgram(lowered); err != nil {
		return nil, err
	}
	loweredMIR, err := mir.LowerProgram(lowered)
	if err != nil {
		return nil, err
	}
	return &Unit{
		Path:    sourcePath,
		Source:  source,
		Program: program,
		HIR:     lowered,
		MIR:     loweredMIR,
	}, nil
}

func ParseSource(source string) (*ast.Program, error) {
	return parser.Parse(source)
}

func expandModules(program *ast.Program, sourcePath string) (*ast.Program, error) {
	if program == nil {
		return nil, nil
	}
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		absPath = sourcePath
	}
	searchPaths := []string{"."}
	if absPath != "" {
		searchPaths = []string{filepath.Dir(absPath)}
	}
	loader := &moduleLoader{
		searchPaths: searchPaths,
		loaded:      map[string]*ast.ModuleDef{},
		loading:     map[string]struct{}{},
	}
	if err := loader.loadProgramModules(program); err != nil {
		return nil, err
	}
	if len(loader.order) == 0 {
		return program, nil
	}
	items := make([]any, 0, len(loader.order)+len(program.Statements))
	for _, name := range loader.order {
		items = append(items, loader.loaded[name])
	}
	items = append(items, program.Statements...)
	return &ast.Program{Statements: items}, nil
}

type moduleLoader struct {
	searchPaths []string
	order       []string
	loaded      map[string]*ast.ModuleDef
	loading     map[string]struct{}
}

func (l *moduleLoader) loadProgramModules(program *ast.Program) error {
	if program == nil {
		return nil
	}
	localModules := map[string]struct{}{}
	for _, stmt := range program.Statements {
		if module, ok := stmt.(*ast.ModuleDef); ok {
			localModules[module.Name] = struct{}{}
		}
	}
	return l.loadUsesInNodes(program.Statements, localModules)
}

func (l *moduleLoader) loadModuleUses(module *ast.ModuleDef) error {
	if module == nil {
		return nil
	}
	return l.loadUsesInNodes(module.Body, nil)
}

func (l *moduleLoader) loadUsesInNodes(nodes []any, localModules map[string]struct{}) error {
	for _, node := range nodes {
		if err := l.loadUsesInNode(node, localModules); err != nil {
			return err
		}
	}
	return nil
}

func (l *moduleLoader) loadUsesInNode(node any, localModules map[string]struct{}) error {
	switch item := node.(type) {
	case nil:
		return nil
	case *ast.UseStmt:
		if isStdlibModule(item.Module) {
			return nil
		}
		if localModules != nil {
			if _, ok := localModules[item.Module]; ok {
				return nil
			}
		}
		return l.loadModule(item.Module)
	case *ast.ModuleDef:
		return l.loadModuleUses(item)
	case *ast.FuncDef:
		if err := l.loadUsesInParams(item.Params, localModules); err != nil {
			return err
		}
		return l.loadUsesInNodes(item.Body, localModules)
	case *ast.ObjectDef:
		if item.Constructor != nil {
			if err := l.loadUsesInParams(item.Constructor.Params, localModules); err != nil {
				return err
			}
			if err := l.loadUsesInNodes(item.Constructor.Body, localModules); err != nil {
				return err
			}
		}
		for _, method := range item.Methods {
			if err := l.loadUsesInParams(method.Params, localModules); err != nil {
				return err
			}
			if err := l.loadUsesInNodes(method.Body, localModules); err != nil {
				return err
			}
		}
		return nil
	case *ast.VarDecl:
		return l.loadUsesInNode(item.Value, localModules)
	case *ast.VarBlock:
		for _, decl := range item.Decls {
			if err := l.loadUsesInNode(decl, localModules); err != nil {
				return err
			}
		}
		return l.loadUsesInNode(item.DefaultValue, localModules)
	case *ast.Assignment:
		for _, target := range item.Targets {
			if err := l.loadUsesInNode(target, localModules); err != nil {
				return err
			}
		}
		for _, value := range item.Values {
			if err := l.loadUsesInNode(value, localModules); err != nil {
				return err
			}
		}
		return nil
	case *ast.ReturnStmt:
		return l.loadUsesInNode(item.Value, localModules)
	case *ast.IfStmt:
		if err := l.loadUsesInNode(item.Condition, localModules); err != nil {
			return err
		}
		if err := l.loadUsesInNodes(item.Body, localModules); err != nil {
			return err
		}
		for _, branch := range item.Elifs {
			if err := l.loadUsesInNode(branch.Condition, localModules); err != nil {
				return err
			}
			if err := l.loadUsesInNodes(branch.Body, localModules); err != nil {
				return err
			}
		}
		return l.loadUsesInNodes(item.ElseBody, localModules)
	case *ast.WhileStmt:
		if err := l.loadUsesInNode(item.Condition, localModules); err != nil {
			return err
		}
		return l.loadUsesInNodes(item.Body, localModules)
	case *ast.ForRangeStmt:
		if err := l.loadUsesInNode(item.Start, localModules); err != nil {
			return err
		}
		if err := l.loadUsesInNode(item.End, localModules); err != nil {
			return err
		}
		if err := l.loadUsesInNode(item.Step, localModules); err != nil {
			return err
		}
		return l.loadUsesInNodes(item.Body, localModules)
	case *ast.ForEachStmt:
		if err := l.loadUsesInNode(item.Iterable, localModules); err != nil {
			return err
		}
		return l.loadUsesInNodes(item.Body, localModules)
	case *ast.MatchStmt:
		if err := l.loadUsesInNode(item.Subject, localModules); err != nil {
			return err
		}
		for _, clause := range item.Cases {
			for _, pattern := range clause.Patterns {
				if err := l.loadUsesInNode(pattern, localModules); err != nil {
					return err
				}
			}
			if err := l.loadUsesInNodes(clause.Body, localModules); err != nil {
				return err
			}
		}
		return l.loadUsesInNodes(item.ElseBody, localModules)
	case *ast.ParallelStmt:
		return l.loadUsesInNodes(item.Body, localModules)
	case *ast.GlobalStmt:
		return l.loadUsesInNode(item.Value, localModules)
	case *ast.ArenaStmt:
		return l.loadUsesInNodes(item.Body, localModules)
	case *ast.ExprStmt:
		return l.loadUsesInNode(item.Expr, localModules)
	case *ast.Lambda:
		if err := l.loadUsesInParams(item.Params, localModules); err != nil {
			return err
		}
		return l.loadUsesInNodes(item.Body, localModules)
	case *ast.BinaryOp:
		if err := l.loadUsesInNode(item.Left, localModules); err != nil {
			return err
		}
		return l.loadUsesInNode(item.Right, localModules)
	case *ast.UnaryOp:
		return l.loadUsesInNode(item.Operand, localModules)
	case *ast.FuncCall:
		if err := l.loadUsesInNode(item.Name, localModules); err != nil {
			return err
		}
		for _, arg := range item.Args {
			if err := l.loadUsesInNode(arg, localModules); err != nil {
				return err
			}
		}
		return nil
	case *ast.MemberAccess:
		return l.loadUsesInNode(item.Object, localModules)
	case *ast.IndexAccess:
		if err := l.loadUsesInNode(item.Object, localModules); err != nil {
			return err
		}
		return l.loadUsesInNode(item.Index, localModules)
	case *ast.OkExpr:
		return l.loadUsesInNode(item.Value, localModules)
	case *ast.ErrExpr:
		return l.loadUsesInNode(item.Value, localModules)
	case *ast.ListLiteral:
		for _, element := range item.Elements {
			if err := l.loadUsesInNode(element, localModules); err != nil {
				return err
			}
		}
		return nil
	case *ast.DictLiteral:
		if err := l.loadUsesInNode(item.KeyType, localModules); err != nil {
			return err
		}
		if err := l.loadUsesInNode(item.ValueType, localModules); err != nil {
			return err
		}
		for _, entry := range item.Entries {
			if err := l.loadUsesInNode(entry.Key, localModules); err != nil {
				return err
			}
			if err := l.loadUsesInNode(entry.Value, localModules); err != nil {
				return err
			}
		}
		return nil
	case *ast.AsExpr:
		return l.loadUsesInNode(item.Expr, localModules)
	case *ast.ObjectLiteral:
		for _, field := range item.Fields {
			if err := l.loadUsesInNode(field.Value, localModules); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func (l *moduleLoader) loadUsesInParams(params []*ast.Param, localModules map[string]struct{}) error {
	for _, param := range params {
		if err := l.loadUsesInNode(param.Default, localModules); err != nil {
			return err
		}
	}
	return nil
}

func (l *moduleLoader) loadModule(name string) error {
	if _, ok := l.loaded[name]; ok {
		return nil
	}
	if _, ok := l.loading[name]; ok {
		return nil
	}
	l.loading[name] = struct{}{}
	defer delete(l.loading, name)
	for _, candidate := range l.moduleCandidatePaths(name) {
		source, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		program, err := parser.Parse(string(source))
		if err != nil {
			return err
		}
		if len(program.Statements) != 1 {
			return nil
		}
		moduleDef, ok := program.Statements[0].(*ast.ModuleDef)
		if !ok || moduleDef.Name != name {
			return nil
		}
		prevSearch := l.searchPaths
		l.searchPaths = prependUnique(filepath.Dir(candidate), l.searchPaths)
		if err := l.loadModuleUses(moduleDef); err != nil {
			l.searchPaths = prevSearch
			return err
		}
		l.searchPaths = prevSearch
		l.loaded[name] = moduleDef
		l.order = append(l.order, name)
		return nil
	}
	return nil
}

func (l *moduleLoader) moduleCandidatePaths(moduleName string) []string {
	seen := map[string]struct{}{}
	var candidates []string
	for _, base := range l.searchPaths {
		for _, candidate := range []string{
			filepath.Join(base, moduleName+".gw"),
			filepath.Join(base, moduleName, "main.gw"),
		} {
			absCandidate, err := filepath.Abs(candidate)
			if err != nil {
				absCandidate = candidate
			}
			if _, ok := seen[absCandidate]; ok {
				continue
			}
			seen[absCandidate] = struct{}{}
			candidates = append(candidates, absCandidate)
		}
	}
	return candidates
}

func prependUnique(value string, items []string) []string {
	if value == "" {
		return append([]string{}, items...)
	}
	out := []string{value}
	for _, item := range items {
		if item == value {
			continue
		}
		out = append(out, item)
	}
	return out
}

func isStdlibModule(name string) bool {
	switch name {
	case "list", "string", "math", "dict", "io", "path", "os", "time", "json", "http", "state", "sqlite":
		return true
	default:
		return false
	}
}
