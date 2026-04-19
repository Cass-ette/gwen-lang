package ast

type TypeName struct {
	Name string
	Line int
}

type GenericType struct {
	Base   string
	Params []any
	Line   int
}

type FuncType struct {
	ParamTypes []any
	ReturnType any
	Line       int
}

type IntLiteral struct {
	Value int64
	Line  int
}

type FloatLiteral struct {
	Value float64
	Line  int
}

type StringLiteral struct {
	Value string
	Line  int
}

type BoolLiteral struct {
	Value bool
	Line  int
}

type Identifier struct {
	Name string
	Line int
}

type BinaryOp struct {
	Left  any
	Op    string
	Right any
	Line  int
}

type UnaryOp struct {
	Op      string
	Operand any
	Line    int
}

type FuncCall struct {
	Name any
	Args []any
	Line int
}

type MemberAccess struct {
	Object any
	Member string
	Line   int
}

type IndexAccess struct {
	Object any
	Index  any
	Line   int
}

type Lambda struct {
	Params []*Param
	Body   []any
	Line   int
}

type OkExpr struct {
	Value any
	Line  int
}

type ErrExpr struct {
	Value any
	Line  int
}

type ListLiteral struct {
	Elements []any
	Line     int
}

type DictEntry struct {
	Key   any
	Value any
}

type DictLiteral struct {
	KeyType   any
	ValueType any
	Entries   []DictEntry
	Line      int
}

type AsExpr struct {
	Expr     any
	TypeName string
	Line     int
}

type ObjectField struct {
	Name  string
	Value any
}

type ObjectLiteral struct {
	Name   string
	Fields []ObjectField
	Line   int
}

type Param struct {
	Name     string
	TypeName any
	Default  any
	Line     int
}

type Assignment struct {
	Targets []any
	Values  []any
	Line    int
}

type VarDecl struct {
	Name     string
	TypeName any
	Value    any
	IsConst  bool
	IsUninit bool
	Line     int
}

type VarBlock struct {
	Decls        []*VarDecl
	DefaultMode  string
	DefaultValue any
	Line         int
}

type ReturnStmt struct {
	Value any
	Line  int
}

type IfBranch struct {
	Condition any
	Body      []any
}

type IfStmt struct {
	Condition any
	Body      []any
	Elifs     []IfBranch
	ElseBody  []any
	Line      int
}

type WhileStmt struct {
	Condition any
	Body      []any
	Line      int
}

type ForRangeStmt struct {
	Var       string
	Start     any
	End       any
	Step      any
	Direction string
	Body      []any
	Line      int
}

type ForEachStmt struct {
	Var      string
	Iterable any
	IndexVar string
	Body     []any
	Line     int
}

type MatchStmt struct {
	Subject  any
	Cases    []*WhenClause
	ElseBody []any
	Line     int
}

type WhenClause struct {
	Patterns []any
	Body     []any
	Line     int
}

type FuncDef struct {
	Name       string
	Params     []*Param
	ReturnType any
	Body       []any
	Exported   bool
	Line       int
}

type ModuleDef struct {
	Name string
	Body []any
	Line int
}

type UseStmt struct {
	Module string
	Names  []string
	Line   int
}

type ParallelStmt struct {
	Body      []any
	ResultVar string
	AllowFail bool
	Line      int
}

type GlobalStmt struct {
	Name  string
	Value any
	Line  int
}

type ArenaStmt struct {
	Name string
	Body []any
	Line int
}

type FieldDef struct {
	Name           string
	TypeAnnotation any
	Line           int
}

type MethodDef struct {
	Name       string
	Params     []*Param
	ReturnType any
	Body       []any
	Line       int
}

type ConstructorDef struct {
	Name       string
	Params     []*Param
	ReturnType any
	Body       []any
	Line       int
}

type ObjectDef struct {
	Name        string
	Fields      []*FieldDef
	Constructor *ConstructorDef
	Methods     []*MethodDef
	Exported    bool
	Line        int
}

type TagStmt struct {
	Name string
	Line int
}

type TypeAlias struct {
	Name     string
	Target   any
	Exported bool
	Line     int
}

type ExprStmt struct {
	Expr any
	Line int
}

type Program struct {
	Statements []any
}
