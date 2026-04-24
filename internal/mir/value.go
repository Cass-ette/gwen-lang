package mir

import "github.com/Cass-ette/gwen-lang/internal/hir"

type ValueKind string

const (
	ValueExprFallback  ValueKind = "expr"
	ValueIntConst      ValueKind = "int_const"
	ValueFloatConst    ValueKind = "float_const"
	ValueStringConst   ValueKind = "string_const"
	ValueBoolConst     ValueKind = "bool_const"
	ValueSlotRef       ValueKind = "slot_ref"
	ValueBindingRef    ValueKind = "binding_ref"
	ValueUnary         ValueKind = "unary"
	ValueBinary        ValueKind = "binary"
	ValueCall          ValueKind = "call"
	ValueCallResult    ValueKind = "call_result"
	ValueMember        ValueKind = "member"
	ValueIndex         ValueKind = "index"
	ValueCast          ValueKind = "cast"
	ValueList          ValueKind = "list"
	ValueDict          ValueKind = "dict"
	ValueObjectLiteral ValueKind = "object_literal"
	ValueOk            ValueKind = "ok"
	ValueErr           ValueKind = "err"
)

type DictEntryValue struct {
	Key   int
	Value int
}

type ObjectFieldValue struct {
	Name  string
	Value int
}

// Value is MIR's current explicit value node.
//
// It intentionally keeps a broad struct surface while the value layer is still
// stabilizing. The goal is to give later lowering/codegen passes stable ids and
// types now, before the final instruction set is frozen.
type Value struct {
	ID     int
	Kind   ValueKind
	Type   hir.Type
	Source hir.Expr

	SlotID  int
	Binding *hir.NameBinding

	IntValue    int64
	FloatValue  float64
	StringValue string
	BoolValue   bool

	Op      string
	Operand int
	Left    int
	Right   int

	Callee      int
	Args        []int
	ReturnTypes []hir.Type
	ResultIDs   []int
	CallID      int
	ResultIndex int

	Object        int
	Member        string
	MemberBinding *hir.MemberBinding
	Index         int

	Elements []int
	Entries  []DictEntryValue
	Fields   []ObjectFieldValue
}

func valueNeedsInst(value *Value) bool {
	if value == nil {
		return false
	}
	switch value.Kind {
	case ValueExprFallback,
		ValueUnary,
		ValueBinary,
		ValueCall,
		ValueMember,
		ValueIndex,
		ValueCast,
		ValueList,
		ValueDict,
		ValueObjectLiteral,
		ValueOk,
		ValueErr:
		return true
	default:
		return false
	}
}
