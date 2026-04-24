package mir

import "github.com/Cass-ette/gwen-lang/internal/hir"

// Program is Gwen's current block/terminator MIR.
//
// It keeps source order at the top level, lowers executable bodies into basic
// blocks, and is starting to grow an explicit typed value layer while still
// keeping HIR expressions around as a transition aid.
type Program struct {
	Items []Item
}

type Item interface {
	Pos() int
	itemNode()
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

func (p *Program) Scripts() []*Script {
	var scripts []*Script
	for _, item := range p.Items {
		script, ok := item.(*Script)
		if !ok {
			continue
		}
		scripts = append(scripts, script)
	}
	return scripts
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
	Params   []*hir.Param
	Returns  []hir.Type
	Body     *Body
	Binding  *hir.NameBinding
	Exported bool
	Line     int
}

func (*Func) itemNode() {}

func (*Func) declNode() {}

func (f *Func) Pos() int { return f.Line }

type Constructor struct {
	Name    string
	Params  []*hir.Param
	Returns []hir.Type
	Body    *Body
	Line    int
}

type Method struct {
	Name    string
	Params  []*hir.Param
	Returns []hir.Type
	Body    *Body
	Line    int
}

type Object struct {
	Name        string
	Fields      []*hir.Field
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
	Target   hir.Type
	Exported bool
	Line     int
}

func (*TypeAlias) itemNode() {}

func (*TypeAlias) declNode() {}

func (a *TypeAlias) Pos() int { return a.Line }

type Script struct {
	Body *Body
	Line int
}

func (*Script) itemNode() {}

func (s *Script) Pos() int { return s.Line }

type Body struct {
	Entry  int
	Blocks []*Block
	Slots  []*Slot
	Values []*Value
	Places []*Place
}

func (b *Body) Block(id int) *Block {
	if b == nil || id <= 0 || id > len(b.Blocks) {
		return nil
	}
	return b.Blocks[id-1]
}

func (b *Body) SlotByBindingID(bindingID int) *Slot {
	if b == nil {
		return nil
	}
	for _, slot := range b.Slots {
		if slot.BindingID == bindingID {
			return slot
		}
	}
	return nil
}

func (b *Body) Value(id int) *Value {
	if b == nil || id <= 0 || id > len(b.Values) {
		return nil
	}
	return b.Values[id-1]
}

func (b *Body) Place(id int) *Place {
	if b == nil || id <= 0 || id > len(b.Places) {
		return nil
	}
	return b.Places[id-1]
}

type SlotKind string

const (
	SlotParam   SlotKind = "param"
	SlotLocal   SlotKind = "local"
	SlotCapture SlotKind = "capture"
)

type Slot struct {
	ID        int
	Kind      SlotKind
	BindingID int
	Name      string
	Type      hir.Type
}

type Block struct {
	ID    int
	Ops   []Op
	Insts []Inst
	Term  Terminator
}

type Op interface {
	Pos() int
	opNode()
}

type AssignOp struct {
	Targets        []hir.Expr
	TargetPlaceIDs []int
	Values         []hir.Expr
	ValueIDs       []int
	Line           int
}

func (*AssignOp) opNode() {}

func (o *AssignOp) Pos() int { return o.Line }

type VarOp struct {
	Name          string
	Type          hir.Type
	TargetPlaceID int
	Value         hir.Expr
	ValueID       int
	Binding       *hir.NameBinding
	IsConst       bool
	IsUninit      bool
	Line          int
}

func (*VarOp) opNode() {}

func (o *VarOp) Pos() int { return o.Line }

type VarBlockOp struct {
	Decls        []*VarOp
	DefaultMode  string
	DefaultValue hir.Expr
	Line         int
}

func (*VarBlockOp) opNode() {}

func (o *VarBlockOp) Pos() int { return o.Line }

type GlobalOp struct {
	Name          string
	Value         hir.Expr
	ValueID       int
	Target        *hir.NameBinding
	TargetPlaceID int
	Line          int
}

func (*GlobalOp) opNode() {}

func (o *GlobalOp) Pos() int { return o.Line }

type ExprOp struct {
	Expr     hir.Expr
	ValueIDs []int
	Line     int
}

func (*ExprOp) opNode() {}

func (o *ExprOp) Pos() int { return o.Line }

type DeclOp struct {
	Decl Decl
	Line int
}

func (*DeclOp) opNode() {}

func (o *DeclOp) Pos() int { return o.Line }

type ParallelOp struct {
	Branches      []*Body
	ResultVar     string
	ResultBinding *hir.NameBinding
	AllowFail     bool
	Line          int
}

func (*ParallelOp) opNode() {}

func (o *ParallelOp) Pos() int { return o.Line }

type ArenaEnterOp struct {
	Name string
	Line int
}

func (*ArenaEnterOp) opNode() {}

func (o *ArenaEnterOp) Pos() int { return o.Line }

type ArenaExitOp struct {
	Name string
	Line int
}

func (*ArenaExitOp) opNode() {}

func (o *ArenaExitOp) Pos() int { return o.Line }

type Terminator interface {
	Pos() int
	termNode()
}

type JumpTerm struct {
	Target int
	Line   int
}

func (*JumpTerm) termNode() {}

func (t *JumpTerm) Pos() int { return t.Line }

type CondTerm struct {
	Condition      hir.Expr
	ConditionValue int
	Then           int
	Else           int
	Line           int
}

func (*CondTerm) termNode() {}

func (t *CondTerm) Pos() int { return t.Line }

type MatchArm struct {
	Patterns        []hir.Expr
	PatternBindings []*hir.MatchPatternBinding
	Target          int
	Line            int
}

type MatchTerm struct {
	Binding      *hir.MatchBinding
	Subject      hir.Expr
	SubjectValue int
	Cases        []MatchArm
	Else         int
	HasElse      bool
	Line         int
}

func (*MatchTerm) termNode() {}

func (t *MatchTerm) Pos() int { return t.Line }

type ForRangeTerm struct {
	Var        string
	VarBinding *hir.NameBinding
	Start      hir.Expr
	StartValue int
	End        hir.Expr
	EndValue   int
	Step       hir.Expr
	StepValue  int
	Direction  string
	Name       string
	LoopID     int
	Body       int
	Exit       int
	Line       int
}

func (*ForRangeTerm) termNode() {}

func (t *ForRangeTerm) Pos() int { return t.Line }

type ForEachTerm struct {
	Var           string
	VarBinding    *hir.NameBinding
	Iterable      hir.Expr
	IterableValue int
	IndexVar      string
	IndexBinding  *hir.NameBinding
	Name          string
	LoopID        int
	Body          int
	Exit          int
	Line          int
}

func (*ForEachTerm) termNode() {}

func (t *ForEachTerm) Pos() int { return t.Line }

type ReturnTerm struct {
	Values   []hir.Expr
	ValueIDs []int
	Line     int
}

func (*ReturnTerm) termNode() {}

func (t *ReturnTerm) Pos() int { return t.Line }

type StopTerm struct {
	Line int
}

func (*StopTerm) termNode() {}

func (t *StopTerm) Pos() int { return t.Line }
