package mir

import "github.com/Cass-ette/gwen-lang/internal/hir"

type Inst interface {
	Pos() int
	instNode()
}

type ComputeInst struct {
	ValueID int
	Line    int
}

func (*ComputeInst) instNode() {}

func (i *ComputeInst) Pos() int { return i.Line }

type CallInst struct {
	ValueID   int
	ResultIDs []int
	Line      int
}

func (*CallInst) instNode() {}

func (i *CallInst) Pos() int { return i.Line }

type StoreInst struct {
	PlaceID int
	ValueID int
	Line    int
}

func (*StoreInst) instNode() {}

func (i *StoreInst) Pos() int { return i.Line }

type DeclareInst struct {
	PlaceID  int
	ValueID  int
	IsConst  bool
	IsUninit bool
	Line     int
}

func (*DeclareInst) instNode() {}

func (i *DeclareInst) Pos() int { return i.Line }

type ParallelInst struct {
	Branches      []*Body
	ResultVar     string
	ResultBinding *hir.NameBinding
	AllowFail     bool
	Line          int
}

func (*ParallelInst) instNode() {}

func (i *ParallelInst) Pos() int { return i.Line }
