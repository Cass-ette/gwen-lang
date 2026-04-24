package hir

// Program is Gwen's current declaration + expression/statement-structure HIR.
//
// It lowers imports, declarations, type annotations, statements, and
// expression trees into a compiler-facing shape, while still deferring richer
// name/type binding to later passes.
type Program struct {
	Items []Item
}

type Item interface {
	Pos() int
	itemNode()
}

func (p *Program) Uses() []*Use {
	var uses []*Use
	for _, item := range p.Items {
		if use, ok := item.(*Use); ok {
			uses = append(uses, use)
		}
	}
	return uses
}

func (p *Program) Decls() []Decl {
	var decls []Decl
	for _, item := range p.Items {
		if decl, ok := item.(Decl); ok {
			decls = append(decls, decl)
		}
	}
	return decls
}

func (p *Program) Stmts() []Stmt {
	var stmts []Stmt
	for _, item := range p.Items {
		stmtItem, ok := item.(*StmtItem)
		if !ok {
			continue
		}
		stmts = append(stmts, stmtItem.Stmt)
	}
	return stmts
}

type Decl interface {
	Item
	declNode()
}

type Use struct {
	Module string
	Names  []string
	Line   int
}

func (*Use) itemNode() {}

func (*Use) stmtNode() {}

func (u *Use) Pos() int { return u.Line }

type Module struct {
	Name  string
	Items []Item
	Line  int
}

func (*Module) itemNode() {}

func (*Module) declNode() {}

func (m *Module) Pos() int { return m.Line }

func (m *Module) Uses() []*Use {
	var uses []*Use
	for _, item := range m.Items {
		if use, ok := item.(*Use); ok {
			uses = append(uses, use)
		}
	}
	return uses
}

func (m *Module) Decls() []Decl {
	var decls []Decl
	for _, item := range m.Items {
		if decl, ok := item.(Decl); ok {
			decls = append(decls, decl)
		}
	}
	return decls
}

type Func struct {
	Name     string
	Params   []*Param
	Returns  []Type
	Body     []Stmt
	Binding  *NameBinding
	Exported bool
	Line     int
}

func (*Func) itemNode() {}

func (*Func) declNode() {}

func (f *Func) Pos() int { return f.Line }

type Field struct {
	Name string
	Type Type
	Line int
}

type Constructor struct {
	Name    string
	Params  []*Param
	Returns []Type
	Body    []Stmt
	Line    int
}

type Method struct {
	Name    string
	Params  []*Param
	Returns []Type
	Body    []Stmt
	Line    int
}

type Object struct {
	Name        string
	Fields      []*Field
	Constructor *Constructor
	Methods     []*Method
	Exported    bool
	Line        int
}

func (*Object) itemNode() {}

func (*Object) declNode() {}

func (o *Object) Pos() int { return o.Line }

type TypeAlias struct {
	Name     string
	Target   Type
	Exported bool
	Line     int
}

func (*TypeAlias) itemNode() {}

func (*TypeAlias) declNode() {}

func (a *TypeAlias) Pos() int { return a.Line }

type Param struct {
	Name    string
	Type    Type
	Default Expr
	Binding *NameBinding
	Line    int
}

type Stmt interface {
	stmtNode()
	Pos() int
}

type StmtItem struct {
	Stmt Stmt
}

func (*StmtItem) itemNode() {}

func (s *StmtItem) Pos() int {
	if s == nil || s.Stmt == nil {
		return 0
	}
	return s.Stmt.Pos()
}

type DeclStmt struct {
	Decl Decl
}

func (*DeclStmt) stmtNode() {}

func (s *DeclStmt) Pos() int { return s.Decl.Pos() }

type Assign struct {
	Targets []Expr
	Values  []Expr
	Line    int
}

func (*Assign) stmtNode() {}

func (s *Assign) Pos() int { return s.Line }

type Var struct {
	Name     string
	Type     Type
	Value    Expr
	Binding  *NameBinding
	IsConst  bool
	IsUninit bool
	Line     int
}

func (*Var) stmtNode() {}

func (s *Var) Pos() int { return s.Line }

type VarBlock struct {
	Decls        []*Var
	DefaultMode  string
	DefaultValue Expr
	Line         int
}

func (*VarBlock) stmtNode() {}

func (s *VarBlock) Pos() int { return s.Line }

type Return struct {
	Values []Expr
	Line   int
}

func (*Return) stmtNode() {}

func (s *Return) Pos() int { return s.Line }

type Pass struct {
	Line int
}

func (*Pass) stmtNode() {}

func (s *Pass) Pos() int { return s.Line }

type Leave struct {
	Name     string
	TargetID int
	Line     int
}

func (*Leave) stmtNode() {}

func (s *Leave) Pos() int { return s.Line }

type Next struct {
	Name     string
	TargetID int
	Line     int
}

func (*Next) stmtNode() {}

func (s *Next) Pos() int { return s.Line }

type IfBranch struct {
	Condition Expr
	Body      []Stmt
}

type If struct {
	Condition Expr
	Body      []Stmt
	Elifs     []IfBranch
	ElseBody  []Stmt
	Line      int
}

func (*If) stmtNode() {}

func (s *If) Pos() int { return s.Line }

type While struct {
	Condition Expr
	Name      string
	LoopID    int
	Body      []Stmt
	Line      int
}

func (*While) stmtNode() {}

func (s *While) Pos() int { return s.Line }

type ForRange struct {
	Var        string
	VarBinding *NameBinding
	Start      Expr
	End        Expr
	Step       Expr
	Direction  string
	Name       string
	LoopID     int
	Body       []Stmt
	Line       int
}

func (*ForRange) stmtNode() {}

func (s *ForRange) Pos() int { return s.Line }

type ForEach struct {
	Var          string
	VarBinding   *NameBinding
	Iterable     Expr
	IndexVar     string
	IndexBinding *NameBinding
	Name         string
	LoopID       int
	Body         []Stmt
	Line         int
}

func (*ForEach) stmtNode() {}

func (s *ForEach) Pos() int { return s.Line }

type MatchCase struct {
	Patterns        []Expr
	PatternBindings []*MatchPatternBinding
	Body            []Stmt
	Line            int
}

type Match struct {
	Binding  *MatchBinding
	Subject  Expr
	Cases    []*MatchCase
	ElseBody []Stmt
	Line     int
}

func (*Match) stmtNode() {}

func (s *Match) Pos() int { return s.Line }

type Parallel struct {
	Body          []Stmt
	ResultVar     string
	ResultBinding *NameBinding
	AllowFail     bool
	Line          int
}

func (*Parallel) stmtNode() {}

func (s *Parallel) Pos() int { return s.Line }

type Global struct {
	Name   string
	Value  Expr
	Target *NameBinding
	Line   int
}

func (*Global) stmtNode() {}

func (s *Global) Pos() int { return s.Line }

type Arena struct {
	Name string
	Body []Stmt
	Line int
}

func (*Arena) stmtNode() {}

func (s *Arena) Pos() int { return s.Line }

type Tag struct {
	Name string
	Line int
}

func (*Tag) stmtNode() {}

func (s *Tag) Pos() int { return s.Line }

type ExprStmt struct {
	Expr Expr
	Line int
}

func (*ExprStmt) stmtNode() {}

func (s *ExprStmt) Pos() int { return s.Line }

type Expr interface {
	exprNode()
	Pos() int
}

type BindingKind string

const (
	BindingLocal      BindingKind = "local"
	BindingParam      BindingKind = "param"
	BindingFunc       BindingKind = "func"
	BindingModule     BindingKind = "module"
	BindingImported   BindingKind = "imported"
	BindingBuiltin    BindingKind = "builtin"
	BindingObjectType BindingKind = "object_type"
)

type NameBinding struct {
	Kind         BindingKind
	ID           int
	Name         string
	SourceModule string
	ObjectName   string
	ScopeDepth   int
}

type MatchBindingKind string

const (
	MatchBindingValue  MatchBindingKind = "value"
	MatchBindingResult MatchBindingKind = "result"
)

type MatchBinding struct {
	Kind MatchBindingKind
}

type MatchPatternKind string

const (
	MatchPatternValue     MatchPatternKind = "value"
	MatchPatternCapture   MatchPatternKind = "capture"
	MatchPatternRange     MatchPatternKind = "range"
	MatchPatternResultOk  MatchPatternKind = "result_ok"
	MatchPatternResultErr MatchPatternKind = "result_err"
)

type MatchPatternBinding struct {
	Kind MatchPatternKind
}

type MemberBindingKind string

const (
	MemberBindingModuleValue       MemberBindingKind = "module_value"
	MemberBindingObjectMethod      MemberBindingKind = "object_method"
	MemberBindingObjectConstructor MemberBindingKind = "object_constructor"
	MemberBindingObjectField       MemberBindingKind = "object_field"
)

type MemberBinding struct {
	Kind       MemberBindingKind
	OwnerName  string
	ObjectName string
}

type IntLiteral struct {
	Value int64
	Line  int
}

func (*IntLiteral) exprNode() {}

func (e *IntLiteral) Pos() int { return e.Line }

type FloatLiteral struct {
	Value float64
	Line  int
}

func (*FloatLiteral) exprNode() {}

func (e *FloatLiteral) Pos() int { return e.Line }

type StringLiteral struct {
	Value string
	Line  int
}

func (*StringLiteral) exprNode() {}

func (e *StringLiteral) Pos() int { return e.Line }

type BoolLiteral struct {
	Value bool
	Line  int
}

func (*BoolLiteral) exprNode() {}

func (e *BoolLiteral) Pos() int { return e.Line }

type Ident struct {
	Name    string
	Binding *NameBinding
	Line    int
}

func (*Ident) exprNode() {}

func (e *Ident) Pos() int { return e.Line }

type Binary struct {
	Left  Expr
	Op    string
	Right Expr
	Line  int
}

func (*Binary) exprNode() {}

func (e *Binary) Pos() int { return e.Line }

type Unary struct {
	Op      string
	Operand Expr
	Line    int
}

func (*Unary) exprNode() {}

func (e *Unary) Pos() int { return e.Line }

type Call struct {
	Callee Expr
	Args   []Expr
	Line   int
}

func (*Call) exprNode() {}

func (e *Call) Pos() int { return e.Line }

type Member struct {
	Object  Expr
	Member  string
	Binding *MemberBinding
	Line    int
}

func (*Member) exprNode() {}

func (e *Member) Pos() int { return e.Line }

type Index struct {
	Object Expr
	Index  Expr
	Line   int
}

func (*Index) exprNode() {}

func (e *Index) Pos() int { return e.Line }

type Lambda struct {
	Params []*Param
	Body   []Stmt
	Line   int
}

func (*Lambda) exprNode() {}

func (e *Lambda) Pos() int { return e.Line }

type Ok struct {
	Value Expr
	Line  int
}

func (*Ok) exprNode() {}

func (e *Ok) Pos() int { return e.Line }

type Err struct {
	Value Expr
	Line  int
}

func (*Err) exprNode() {}

func (e *Err) Pos() int { return e.Line }

type List struct {
	Elements []Expr
	Line     int
}

func (*List) exprNode() {}

func (e *List) Pos() int { return e.Line }

type DictEntry struct {
	Key   Expr
	Value Expr
}

type Dict struct {
	KeyType   Type
	ValueType Type
	Entries   []DictEntry
	Line      int
}

func (*Dict) exprNode() {}

func (e *Dict) Pos() int { return e.Line }

type Cast struct {
	Value      Expr
	TargetName string
	Line       int
}

func (*Cast) exprNode() {}

func (e *Cast) Pos() int { return e.Line }

type ObjectField struct {
	Name  string
	Value Expr
}

type ObjectLiteral struct {
	Name   string
	Fields []ObjectField
	Line   int
}

func (*ObjectLiteral) exprNode() {}

func (e *ObjectLiteral) Pos() int { return e.Line }

type Type interface {
	typeNode()
	Pos() int
}

type NamedType struct {
	Name string
	Line int
}

func (*NamedType) typeNode() {}

func (t *NamedType) Pos() int { return t.Line }

type GenericType struct {
	Base string
	Args []Type
	Line int
}

func (*GenericType) typeNode() {}

func (t *GenericType) Pos() int { return t.Line }

type FuncType struct {
	Params  []Type
	Returns []Type
	Line    int
}

func (*FuncType) typeNode() {}

func (t *FuncType) Pos() int { return t.Line }
