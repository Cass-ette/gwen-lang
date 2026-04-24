package mir

import "github.com/Cass-ette/gwen-lang/internal/hir"

type PlaceKind string

const (
	PlaceSlot  PlaceKind = "slot"
	PlaceIndex PlaceKind = "index"
	PlaceField PlaceKind = "field"
)

// Place is MIR's current explicit assignment/storage target node.
//
// Like Value, this stays intentionally broad while lowering is still being
// normalized. The important part is that target shape is now explicit instead
// of hidden in HIR expressions.
type Place struct {
	ID     int
	Kind   PlaceKind
	Type   hir.Type
	Source hir.Expr

	SlotID  int
	Binding *hir.NameBinding

	Object        int
	Index         int
	Member        string
	MemberBinding *hir.MemberBinding
}
