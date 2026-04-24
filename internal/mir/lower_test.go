package mir_test

import (
	"testing"

	"github.com/Cass-ette/gwen-lang/internal/hir"
	"github.com/Cass-ette/gwen-lang/internal/mir"
	"github.com/Cass-ette/gwen-lang/internal/parser"
)

func requireNamedType(t *testing.T, typ hir.Type, want string) {
	t.Helper()

	named, ok := typ.(*hir.NamedType)
	if !ok {
		t.Fatalf("expected *hir.NamedType, got %T", typ)
	}
	if named.Name != want {
		t.Fatalf("type mismatch: got %q want %q", named.Name, want)
	}
}

func requireGenericType(t *testing.T, typ hir.Type, wantBase string, wantArity int) *hir.GenericType {
	t.Helper()

	generic, ok := typ.(*hir.GenericType)
	if !ok {
		t.Fatalf("expected *hir.GenericType, got %T", typ)
	}
	if generic.Base != wantBase {
		t.Fatalf("generic base mismatch: got %q want %q", generic.Base, wantBase)
	}
	if len(generic.Args) != wantArity {
		t.Fatalf("generic arity mismatch: got %d want %d", len(generic.Args), wantArity)
	}
	return generic
}

func requireFuncType(t *testing.T, typ hir.Type, wantParams int, wantReturns int) *hir.FuncType {
	t.Helper()

	fn, ok := typ.(*hir.FuncType)
	if !ok {
		t.Fatalf("expected *hir.FuncType, got %T", typ)
	}
	if len(fn.Params) != wantParams {
		t.Fatalf("func param count mismatch: got %d want %d", len(fn.Params), wantParams)
	}
	if len(fn.Returns) != wantReturns {
		t.Fatalf("func return count mismatch: got %d want %d", len(fn.Returns), wantReturns)
	}
	return fn
}

func requireValue(t *testing.T, body *mir.Body, id int, want mir.ValueKind) *mir.Value {
	t.Helper()

	value := body.Value(id)
	if value == nil {
		t.Fatalf("expected value id %d", id)
	}
	if value.Kind != want {
		t.Fatalf("value kind mismatch: got %q want %q", value.Kind, want)
	}
	return value
}

func requirePlace(t *testing.T, body *mir.Body, id int, want mir.PlaceKind) *mir.Place {
	t.Helper()

	place := body.Place(id)
	if place == nil {
		t.Fatalf("expected place id %d", id)
	}
	if place.Kind != want {
		t.Fatalf("place kind mismatch: got %q want %q", place.Kind, want)
	}
	return place
}

func mustLowerMIR(t *testing.T, source string) *mir.Program {
	t.Helper()

	program, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	loweredHIR, err := hir.LowerProgram(program)
	if err != nil {
		t.Fatalf("HIR lower failed: %v", err)
	}
	if err := hir.BindProgram(loweredHIR); err != nil {
		t.Fatalf("HIR bind failed: %v", err)
	}
	loweredMIR, err := mir.LowerProgram(loweredHIR)
	if err != nil {
		t.Fatalf("MIR lower failed: %v", err)
	}
	return loweredMIR
}

func TestLowerProgramPreservesOrderWithScripts(t *testing.T) {
	program := mustLowerMIR(t, `use math
func inc(n: int) -> int
  return n
endfunc
write("a")
type Count = int
write("b")`)

	if len(program.Items) != 5 {
		t.Fatalf("item count mismatch: got %d want 5", len(program.Items))
	}
	if _, ok := program.Items[0].(*mir.Use); !ok {
		t.Fatalf("expected first item *mir.Use, got %T", program.Items[0])
	}
	if _, ok := program.Items[1].(*mir.Func); !ok {
		t.Fatalf("expected second item *mir.Func, got %T", program.Items[1])
	}
	if _, ok := program.Items[2].(*mir.Script); !ok {
		t.Fatalf("expected third item *mir.Script, got %T", program.Items[2])
	}
	if _, ok := program.Items[3].(*mir.TypeAlias); !ok {
		t.Fatalf("expected fourth item *mir.TypeAlias, got %T", program.Items[3])
	}
	if _, ok := program.Items[4].(*mir.Script); !ok {
		t.Fatalf("expected fifth item *mir.Script, got %T", program.Items[4])
	}
}

func TestLowerWhileToExplicitBlocks(t *testing.T) {
	program := mustLowerMIR(t, `func main()
  while true do scan
    if true then
      next scan
    endif
    leave scan
  endwhile scan
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	if fn.Body.Entry != 1 {
		t.Fatalf("entry mismatch: got %d want 1", fn.Body.Entry)
	}
	if len(fn.Body.Blocks) != 6 {
		t.Fatalf("block count mismatch: got %d want 6", len(fn.Body.Blocks))
	}

	entryTerm, ok := fn.Body.Block(1).Term.(*mir.JumpTerm)
	if !ok || entryTerm.Target != 2 {
		t.Fatalf("expected block 1 jump to 2, got %T %+v", fn.Body.Block(1).Term, fn.Body.Block(1).Term)
	}
	loopTerm, ok := fn.Body.Block(2).Term.(*mir.CondTerm)
	if !ok || loopTerm.Then != 4 || loopTerm.Else != 3 {
		t.Fatalf("expected loop cond then=4 else=3, got %T %+v", fn.Body.Block(2).Term, fn.Body.Block(2).Term)
	}
	if _, ok := fn.Body.Block(3).Term.(*mir.StopTerm); !ok {
		t.Fatalf("expected exit block stop term, got %T", fn.Body.Block(3).Term)
	}
	ifTerm, ok := fn.Body.Block(4).Term.(*mir.CondTerm)
	if !ok || ifTerm.Then != 6 || ifTerm.Else != 5 {
		t.Fatalf("expected inner if then=6 else=5, got %T %+v", fn.Body.Block(4).Term, fn.Body.Block(4).Term)
	}
	leaveTerm, ok := fn.Body.Block(5).Term.(*mir.JumpTerm)
	if !ok || leaveTerm.Target != 3 {
		t.Fatalf("expected leave block jump to 3, got %T %+v", fn.Body.Block(5).Term, fn.Body.Block(5).Term)
	}
	nextTerm, ok := fn.Body.Block(6).Term.(*mir.JumpTerm)
	if !ok || nextTerm.Target != 2 {
		t.Fatalf("expected next block jump to 2, got %T %+v", fn.Body.Block(6).Term, fn.Body.Block(6).Term)
	}
}

func TestLowerParallelToIndependentBranchBodies(t *testing.T) {
	program := mustLowerMIR(t, `func main()
  parallel allowfail => results do
    write("a")
    return 1
  endparallel
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	if len(fn.Body.Blocks) != 1 {
		t.Fatalf("body block count mismatch: got %d want 1", len(fn.Body.Blocks))
	}
	entry := fn.Body.Block(1)
	if len(entry.Ops) != 1 {
		t.Fatalf("entry op count mismatch: got %d want 1", len(entry.Ops))
	}
	if len(entry.Insts) != 1 {
		t.Fatalf("entry instruction count mismatch: got %d want 1", len(entry.Insts))
	}
	parallelOp, ok := entry.Ops[0].(*mir.ParallelOp)
	if !ok {
		t.Fatalf("expected *mir.ParallelOp, got %T", entry.Ops[0])
	}
	parallelInst, ok := entry.Insts[0].(*mir.ParallelInst)
	if !ok {
		t.Fatalf("expected *mir.ParallelInst, got %T", entry.Insts[0])
	}
	if !parallelOp.AllowFail || parallelOp.ResultVar != "results" {
		t.Fatalf("parallel metadata mismatch: %+v", parallelOp)
	}
	if !parallelInst.AllowFail || parallelInst.ResultVar != "results" {
		t.Fatalf("parallel instruction metadata mismatch: %+v", parallelInst)
	}
	if len(parallelOp.Branches) != 2 {
		t.Fatalf("parallel branch count mismatch: got %d want 2", len(parallelOp.Branches))
	}
	if len(parallelInst.Branches) != 2 {
		t.Fatalf("parallel instruction branch count mismatch: got %d want 2", len(parallelInst.Branches))
	}

	firstBranch := parallelOp.Branches[0]
	if len(firstBranch.Blocks) != 1 {
		t.Fatalf("first branch block count mismatch: got %d want 1", len(firstBranch.Blocks))
	}
	if len(firstBranch.Block(1).Ops) != 1 {
		t.Fatalf("first branch op count mismatch: got %d want 1", len(firstBranch.Block(1).Ops))
	}
	if _, ok := firstBranch.Block(1).Ops[0].(*mir.ExprOp); !ok {
		t.Fatalf("expected first branch expr op, got %T", firstBranch.Block(1).Ops[0])
	}
	if _, ok := firstBranch.Block(1).Term.(*mir.StopTerm); !ok {
		t.Fatalf("expected first branch stop term, got %T", firstBranch.Block(1).Term)
	}

	secondBranch := parallelOp.Branches[1]
	if len(secondBranch.Blocks) != 1 {
		t.Fatalf("second branch block count mismatch: got %d want 1", len(secondBranch.Blocks))
	}
	if len(secondBranch.Block(1).Ops) != 0 {
		t.Fatalf("second branch op count mismatch: got %d want 0", len(secondBranch.Block(1).Ops))
	}
	if _, ok := secondBranch.Block(1).Term.(*mir.ReturnTerm); !ok {
		t.Fatalf("expected second branch return term, got %T", secondBranch.Block(1).Term)
	}
	if len(fn.Body.Slots) != 1 {
		t.Fatalf("expected one slot for parallel results, got %d", len(fn.Body.Slots))
	}
	if slot := fn.Body.Slots[0]; slot.Kind != mir.SlotLocal || slot.Name != "results" {
		t.Fatalf("unexpected parallel result slot: %+v", slot)
	}
	requireNamedType(t, fn.Body.Slots[0].Type, "list")
}

func TestLowerCarriesGlobalAndMatchBindingMetadata(t *testing.T) {
	program := mustLowerMIR(t, `func outer(value: result[int, string])
  total: int := 0
  func bump()
    global total := total + 1
  endfunc
  match value
    when ok(data) =>
      write(data)
    when err(reason) =>
      write(reason)
  endmatch
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	entry := fn.Body.Block(1)
	if len(entry.Ops) != 2 {
		t.Fatalf("entry op count mismatch: got %d want 2", len(entry.Ops))
	}

	declOp, ok := entry.Ops[1].(*mir.DeclOp)
	if !ok {
		t.Fatalf("expected nested decl op, got %T", entry.Ops[1])
	}
	bump := declOp.Decl.(*mir.Func)
	globalOp, ok := bump.Body.Block(1).Ops[0].(*mir.GlobalOp)
	if !ok {
		t.Fatalf("expected nested global op, got %T", bump.Body.Block(1).Ops[0])
	}
	if globalOp.Target == nil || globalOp.Target.Name != "total" || globalOp.Target.ScopeDepth != 1 {
		t.Fatalf("expected global op target for outer total, got %+v", globalOp.Target)
	}
	if globalOp.TargetPlaceID == 0 {
		t.Fatal("expected explicit capture place target for global op")
	}
	globalPlace := requirePlace(t, bump.Body, globalOp.TargetPlaceID, mir.PlaceSlot)
	if globalPlace.Binding == nil || globalPlace.Binding.Name != "total" || globalPlace.Binding.ScopeDepth != 1 {
		t.Fatalf("unexpected global target place binding: %+v", globalPlace.Binding)
	}
	if slot := bump.Body.SlotByBindingID(globalOp.Target.ID); slot != nil && slot.ID != globalPlace.SlotID {
		t.Fatalf("global target slot mismatch: place=%d slot=%d", globalPlace.SlotID, slot.ID)
	}
	if len(bump.Body.Block(1).Insts) != 2 {
		t.Fatalf("expected 2 instructions for nested global write, got %d", len(bump.Body.Block(1).Insts))
	}
	computeInst, ok := bump.Body.Block(1).Insts[0].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected first nested global instruction *mir.ComputeInst, got %T", bump.Body.Block(1).Insts[0])
	}
	storeInst, ok := bump.Body.Block(1).Insts[1].(*mir.StoreInst)
	if !ok {
		t.Fatalf("expected second nested global instruction *mir.StoreInst, got %T", bump.Body.Block(1).Insts[1])
	}
	if computeInst.ValueID != globalOp.ValueID {
		t.Fatalf("global compute value mismatch: got %d want %d", computeInst.ValueID, globalOp.ValueID)
	}
	if storeInst.PlaceID != globalOp.TargetPlaceID || storeInst.ValueID != globalOp.ValueID {
		t.Fatalf("global store mismatch: %+v", storeInst)
	}

	matchTerm, ok := entry.Term.(*mir.MatchTerm)
	if !ok {
		t.Fatalf("expected match terminator, got %T", entry.Term)
	}
	if matchTerm.Binding == nil || matchTerm.Binding.Kind != hir.MatchBindingResult {
		t.Fatalf("expected result match binding, got %+v", matchTerm.Binding)
	}
	if got := matchTerm.Cases[0].PatternBindings[0].Kind; got != hir.MatchPatternResultOk {
		t.Fatalf("expected first MIR arm result_ok binding, got %q", got)
	}
	if got := matchTerm.Cases[1].PatternBindings[0].Kind; got != hir.MatchPatternResultErr {
		t.Fatalf("expected second MIR arm result_err binding, got %q", got)
	}
}

func TestLowerBodyAllocatesStableSlots(t *testing.T) {
	program := mustLowerMIR(t, `func outer(value: int)
  total: int := value
  func bump(delta: int)
    global total := total + delta
    item := delta
    write(value, total, item)
  endfunc
  for item in [1, 2] with index idx do
    write(item, idx)
  endfor
endfunc`)

	outer := program.Decls()[0].(*mir.Func)
	if len(outer.Body.Slots) != 4 {
		t.Fatalf("outer slot count mismatch: got %d want 4", len(outer.Body.Slots))
	}
	if slot := outer.Body.SlotByBindingID(outer.Params[0].Binding.ID); slot == nil || slot.Kind != mir.SlotParam || slot.Name != "value" {
		t.Fatalf("expected param slot for value, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}
	totalDecl := outer.Body.Block(1).Ops[0].(*mir.VarOp)
	if totalDecl.TargetPlaceID == 0 {
		t.Fatal("expected explicit place for total declaration")
	}
	totalPlace := requirePlace(t, outer.Body, totalDecl.TargetPlaceID, mir.PlaceSlot)
	if totalPlace.Binding == nil || totalPlace.Binding.Name != "total" {
		t.Fatalf("unexpected total place binding: %+v", totalPlace.Binding)
	}
	if slot := outer.Body.SlotByBindingID(totalDecl.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "total" {
		t.Fatalf("expected local slot for total, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}
	loopTerm := outer.Body.Block(2).Term.(*mir.ForEachTerm)
	if slot := outer.Body.SlotByBindingID(loopTerm.VarBinding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "item" {
		t.Fatalf("expected local slot for loop item, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}
	if slot := outer.Body.SlotByBindingID(loopTerm.IndexBinding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "idx" {
		t.Fatalf("expected local slot for loop idx, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}

	declOp := outer.Body.Block(1).Ops[1].(*mir.DeclOp)
	bump := declOp.Decl.(*mir.Func)
	if len(bump.Body.Slots) != 4 {
		t.Fatalf("nested slot count mismatch: got %d want 4", len(bump.Body.Slots))
	}
	if slot := bump.Body.SlotByBindingID(bump.Params[0].Binding.ID); slot == nil || slot.Kind != mir.SlotParam || slot.Name != "delta" {
		t.Fatalf("expected param slot for delta, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}
	if slot := bump.Body.SlotByBindingID(outer.Params[0].Binding.ID); slot == nil || slot.Kind != mir.SlotCapture || slot.Name != "value" {
		t.Fatalf("expected capture slot for outer value, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}
	totalSlot := bump.Body.SlotByBindingID(totalDecl.Binding.ID)
	if totalSlot == nil || totalSlot.Kind != mir.SlotCapture || totalSlot.Name != "total" {
		t.Fatalf("expected capture slot for total, got %+v", totalSlot)
	}
	requireNamedType(t, totalSlot.Type, "int")
	itemAssign := bump.Body.Block(1).Ops[1].(*mir.AssignOp)
	itemIdent := itemAssign.Targets[0].(*hir.Ident)
	if slot := bump.Body.SlotByBindingID(itemIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "item" {
		t.Fatalf("expected local slot for inner item, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}
}

func TestLowerInfersResultMatchCaptureTypes(t *testing.T) {
	program := mustLowerMIR(t, `func outer(value: result[int, string])
  match value
    when ok(data) =>
      write(data)
    when err(reason) =>
      write(reason)
  endmatch
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	matchTerm := fn.Body.Block(1).Term.(*mir.MatchTerm)
	okExpr := matchTerm.Cases[0].Patterns[0].(*hir.Ok)
	data := okExpr.Value.(*hir.Ident)
	if slot := fn.Body.SlotByBindingID(data.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "data" {
		t.Fatalf("expected local slot for ok(data), got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}
	errExpr := matchTerm.Cases[1].Patterns[0].(*hir.Err)
	reason := errExpr.Value.(*hir.Ident)
	if slot := fn.Body.SlotByBindingID(reason.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "reason" {
		t.Fatalf("expected local slot for err(reason), got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "string")
	}
}

func TestLowerInfersListLiteralTypes(t *testing.T) {
	program := mustLowerMIR(t, `func main()
  items := [1, 2, 3]
  for item in items do
    write(item)
  endfor
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	assign := fn.Body.Block(1).Ops[0].(*mir.AssignOp)
	items := assign.Targets[0].(*hir.Ident)
	itemsSlot := fn.Body.SlotByBindingID(items.Binding.ID)
	if itemsSlot == nil {
		t.Fatal("expected slot for items")
	}
	listType := requireGenericType(t, itemsSlot.Type, "list", 1)
	requireNamedType(t, listType.Args[0], "int")

	loopTerm := fn.Body.Block(2).Term.(*mir.ForEachTerm)
	itemSlot := fn.Body.SlotByBindingID(loopTerm.VarBinding.ID)
	if itemSlot == nil {
		t.Fatal("expected slot for loop item")
	}
	requireNamedType(t, itemSlot.Type, "int")
}

func TestLowerContextualizesEmptyListTypes(t *testing.T) {
	program := mustLowerMIR(t, `func guest() -> result[list[string]]
  return ok([])
endfunc

func build() -> list[string]
  items: list[string] := []
  return items
endfunc`)

	guest := program.Decls()[0].(*mir.Func)
	guestReturn := guest.Body.Block(1).Term.(*mir.ReturnTerm)
	okValue := requireValue(t, guest.Body, guestReturn.ValueIDs[0], mir.ValueOk)
	okType := requireGenericType(t, okValue.Type, "result", 1)
	guestListType := requireGenericType(t, okType.Args[0], "list", 1)
	requireNamedType(t, guestListType.Args[0], "string")

	okList := requireValue(t, guest.Body, okValue.Operand, mir.ValueList)
	okListType := requireGenericType(t, okList.Type, "list", 1)
	requireNamedType(t, okListType.Args[0], "string")

	build := program.Decls()[1].(*mir.Func)
	buildDecl := build.Body.Block(1).Ops[0].(*mir.VarOp)
	buildList := requireValue(t, build.Body, buildDecl.ValueID, mir.ValueList)
	buildListType := requireGenericType(t, buildList.Type, "list", 1)
	requireNamedType(t, buildListType.Args[0], "string")
}

func TestLowerInfersMultiReturnAssignmentFromLocalFunc(t *testing.T) {
	program := mustLowerMIR(t, `func pair() -> int, string
  return 7, "ok"
endfunc

func main()
  count, label := pair()
endfunc`)

	fn := program.Decls()[1].(*mir.Func)
	assign := fn.Body.Block(1).Ops[0].(*mir.AssignOp)

	count := assign.Targets[0].(*hir.Ident)
	if slot := fn.Body.SlotByBindingID(count.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "count" {
		t.Fatalf("expected slot for count, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}

	label := assign.Targets[1].(*hir.Ident)
	if slot := fn.Body.SlotByBindingID(label.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "label" {
		t.Fatalf("expected slot for label, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "string")
	}
}

func TestLowerInfersMemberIndexAndExpressionTypes(t *testing.T) {
	program := mustLowerMIR(t, `object Account
  balance: int
  new(balance: int) -> Account
    return Account{balance := balance}
  endnew
  func snapshot(self: Account) -> int
    current := self.balance
    return current
  endfunc
endobject

func main()
  account := Account.new(1)
  total := 1 + 2
  value := account.snapshot()
  parts := split("go", "")
  first := parts[0]
endfunc`)

	accountObj := program.Decls()[0].(*mir.Object)
	snapshot := accountObj.Methods[0]
	currentAssign := snapshot.Body.Block(1).Ops[0].(*mir.AssignOp)
	current := currentAssign.Targets[0].(*hir.Ident)
	if slot := snapshot.Body.SlotByBindingID(current.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "current" {
		t.Fatalf("expected slot for current, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}

	mainFn := program.Decls()[1].(*mir.Func)
	accountAssign := mainFn.Body.Block(1).Ops[0].(*mir.AssignOp)
	account := accountAssign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(account.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "account" {
		t.Fatalf("expected slot for account, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "Account")
	}

	totalAssign := mainFn.Body.Block(1).Ops[1].(*mir.AssignOp)
	total := totalAssign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(total.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "total" {
		t.Fatalf("expected slot for total, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}

	valueAssign := mainFn.Body.Block(1).Ops[2].(*mir.AssignOp)
	value := valueAssign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(value.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "value" {
		t.Fatalf("expected slot for value, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}

	partsAssign := mainFn.Body.Block(1).Ops[3].(*mir.AssignOp)
	parts := partsAssign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(parts.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "parts" {
		t.Fatalf("expected slot for parts, got %+v", slot)
	} else {
		listType := requireGenericType(t, slot.Type, "list", 1)
		requireNamedType(t, listType.Args[0], "string")
	}

	firstAssign := mainFn.Body.Block(1).Ops[4].(*mir.AssignOp)
	first := firstAssign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(first.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "first" {
		t.Fatalf("expected slot for first, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "string")
	}
}

func TestLowerInfersStdlibCallTypes(t *testing.T) {
	program := mustLowerMIR(t, `use http
use state

func handle(req: HttpRequest)
  matched, params := http.route(req, "/hello/:name")
  served, reply := http.static(req, "/assets/", "examples/http_server_public")
endfunc

func main()
  counter := state.cell(1)
  current := state.get(counter)
  updated := state.set(counter, current + 1)
endfunc`)

	handle := program.Decls()[0].(*mir.Func)
	firstAssign := handle.Body.Block(1).Ops[0].(*mir.AssignOp)
	matched := firstAssign.Targets[0].(*hir.Ident)
	if slot := handle.Body.SlotByBindingID(matched.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "matched" {
		t.Fatalf("expected slot for matched, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "bool")
	}

	params := firstAssign.Targets[1].(*hir.Ident)
	if slot := handle.Body.SlotByBindingID(params.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "params" {
		t.Fatalf("expected slot for params, got %+v", slot)
	} else {
		dictType := requireGenericType(t, slot.Type, "dict", 2)
		requireNamedType(t, dictType.Args[0], "string")
		requireNamedType(t, dictType.Args[1], "string")
	}

	secondAssign := handle.Body.Block(1).Ops[1].(*mir.AssignOp)
	served := secondAssign.Targets[0].(*hir.Ident)
	if slot := handle.Body.SlotByBindingID(served.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "served" {
		t.Fatalf("expected slot for served, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "bool")
	}

	reply := secondAssign.Targets[1].(*hir.Ident)
	if slot := handle.Body.SlotByBindingID(reply.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "reply" {
		t.Fatalf("expected slot for reply, got %+v", slot)
	} else {
		resultType := requireGenericType(t, slot.Type, "result", 1)
		requireNamedType(t, resultType.Args[0], "HttpReply")
	}

	mainFn := program.Decls()[1].(*mir.Func)
	counterAssign := mainFn.Body.Block(1).Ops[0].(*mir.AssignOp)
	counter := counterAssign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(counter.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "counter" {
		t.Fatalf("expected slot for counter, got %+v", slot)
	} else {
		cellType := requireGenericType(t, slot.Type, "cell", 1)
		requireNamedType(t, cellType.Args[0], "int")
	}

	currentAssign := mainFn.Body.Block(1).Ops[1].(*mir.AssignOp)
	current := currentAssign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(current.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "current" {
		t.Fatalf("expected slot for current, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}

	updatedAssign := mainFn.Body.Block(1).Ops[2].(*mir.AssignOp)
	updated := updatedAssign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(updated.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "updated" {
		t.Fatalf("expected slot for updated, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}
}

func TestLowerInfersEnumerateAsListOfLists(t *testing.T) {
	program := mustLowerMIR(t, `use enumerate from list

func main()
  indexed: list[list] := enumerate(["a", "b", "c"])
  write(indexed[1][0], indexed[1][1])
endfunc`)

	mainFn := program.Decls()[0].(*mir.Func)
	decl := mainFn.Body.Block(1).Ops[0].(*mir.VarOp)
	if decl.Binding == nil {
		t.Fatalf("expected binding for indexed declaration")
	}
	if slot := mainFn.Body.SlotByBindingID(decl.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "indexed" {
		t.Fatalf("expected slot for indexed, got %+v", slot)
	} else {
		listType := requireGenericType(t, slot.Type, "list", 1)
		requireNamedType(t, listType.Args[0], "list")
	}

	call := requireValue(t, mainFn.Body, decl.ValueID, mir.ValueCall)
	if len(call.ReturnTypes) != 1 {
		t.Fatalf("call return count mismatch: got %d want 1", len(call.ReturnTypes))
	}
	listType := requireGenericType(t, call.ReturnTypes[0], "list", 1)
	requireNamedType(t, listType.Args[0], "list")
}

func TestLowerInfersItemsAsListOfLists(t *testing.T) {
	program := mustLowerMIR(t, `use items from dict

func main()
  scores := dict[string, int]{"alice": 1, "bob": 2}
  pairs: list[list] := items(scores)
  write(len(pairs))
endfunc`)

	mainFn := program.Decls()[0].(*mir.Func)
	decl := mainFn.Body.Block(1).Ops[1].(*mir.VarOp)
	if decl.Binding == nil {
		t.Fatalf("expected binding for pairs declaration")
	}
	if slot := mainFn.Body.SlotByBindingID(decl.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "pairs" {
		t.Fatalf("expected slot for pairs, got %+v", slot)
	} else {
		listType := requireGenericType(t, slot.Type, "list", 1)
		requireNamedType(t, listType.Args[0], "list")
	}

	call := requireValue(t, mainFn.Body, decl.ValueID, mir.ValueCall)
	if len(call.ReturnTypes) != 1 {
		t.Fatalf("call return count mismatch: got %d want 1", len(call.ReturnTypes))
	}
	listType := requireGenericType(t, call.ReturnTypes[0], "list", 1)
	requireNamedType(t, listType.Args[0], "list")
}

func TestLowerBuiltinFunctionValuesKeepCallableParams(t *testing.T) {
	program := mustLowerMIR(t, `use trim from string

func main()
  formatter := str
  cleaner := trim
  measurer := len
endfunc`)

	mainFn := program.Decls()[0].(*mir.Func)

	formatter := mainFn.Body.Block(1).Ops[0].(*mir.AssignOp)
	formatterIdent := formatter.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(formatterIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "formatter" {
		t.Fatalf("expected slot for formatter, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 1, 1)
		requireNamedType(t, fn.Params[0], "dynamic")
		requireNamedType(t, fn.Returns[0], "string")
	}

	cleaner := mainFn.Body.Block(1).Ops[1].(*mir.AssignOp)
	cleanerIdent := cleaner.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(cleanerIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "cleaner" {
		t.Fatalf("expected slot for cleaner, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 1, 1)
		requireNamedType(t, fn.Params[0], "string")
		requireNamedType(t, fn.Returns[0], "string")
	}

	measurer := mainFn.Body.Block(1).Ops[2].(*mir.AssignOp)
	measurerIdent := measurer.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(measurerIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "measurer" {
		t.Fatalf("expected slot for measurer, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 1, 1)
		requireNamedType(t, fn.Params[0], "dynamic")
		requireNamedType(t, fn.Returns[0], "int")
	}
}

func TestLowerImportedBuiltinFunctionValuesKeepCallableParams(t *testing.T) {
	program := mustLowerMIR(t, `use readfile from io
use basename from path
use join from string
use json
use os
use time

func main()
  loader := readfile
  stem := basename
  joiner := join
  encode := json.stringify
  parseobj := json.parseobject
  nothing := json.isnull
  argv := os.args
  cwdf := os.cwd
  clock := time.nowunix
endfunc`)

	mainFn := program.Decls()[0].(*mir.Func)

	loader := mainFn.Body.Block(1).Ops[0].(*mir.AssignOp)
	loaderIdent := loader.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(loaderIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "loader" {
		t.Fatalf("expected slot for loader, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 1, 1)
		requireNamedType(t, fn.Params[0], "string")
		resultType := requireGenericType(t, fn.Returns[0], "result", 1)
		requireNamedType(t, resultType.Args[0], "string")
	}

	stem := mainFn.Body.Block(1).Ops[1].(*mir.AssignOp)
	stemIdent := stem.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(stemIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "stem" {
		t.Fatalf("expected slot for stem, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 1, 1)
		requireNamedType(t, fn.Params[0], "string")
		requireNamedType(t, fn.Returns[0], "string")
	}

	joiner := mainFn.Body.Block(1).Ops[2].(*mir.AssignOp)
	joinerIdent := joiner.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(joinerIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "joiner" {
		t.Fatalf("expected slot for joiner, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 2, 1)
		requireNamedType(t, fn.Params[0], "list")
		requireNamedType(t, fn.Params[1], "string")
		requireNamedType(t, fn.Returns[0], "string")
	}

	encode := mainFn.Body.Block(1).Ops[3].(*mir.AssignOp)
	encodeIdent := encode.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(encodeIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "encode" {
		t.Fatalf("expected slot for encode, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 1, 1)
		requireNamedType(t, fn.Params[0], "dynamic")
		resultType := requireGenericType(t, fn.Returns[0], "result", 1)
		requireNamedType(t, resultType.Args[0], "string")
	}

	parseobj := mainFn.Body.Block(1).Ops[4].(*mir.AssignOp)
	parseobjIdent := parseobj.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(parseobjIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "parseobj" {
		t.Fatalf("expected slot for parseobj, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 1, 1)
		requireNamedType(t, fn.Params[0], "string")
		resultType := requireGenericType(t, fn.Returns[0], "result", 1)
		requireNamedType(t, resultType.Args[0], "dict")
	}

	nothing := mainFn.Body.Block(1).Ops[5].(*mir.AssignOp)
	nothingIdent := nothing.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(nothingIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "nothing" {
		t.Fatalf("expected slot for nothing, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 1, 1)
		requireNamedType(t, fn.Params[0], "dynamic")
		requireNamedType(t, fn.Returns[0], "bool")
	}

	argv := mainFn.Body.Block(1).Ops[6].(*mir.AssignOp)
	argvIdent := argv.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(argvIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "argv" {
		t.Fatalf("expected slot for argv, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 0, 1)
		listType := requireGenericType(t, fn.Returns[0], "list", 1)
		requireNamedType(t, listType.Args[0], "string")
	}

	cwdf := mainFn.Body.Block(1).Ops[7].(*mir.AssignOp)
	cwdfIdent := cwdf.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(cwdfIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "cwdf" {
		t.Fatalf("expected slot for cwdf, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 0, 1)
		requireNamedType(t, fn.Returns[0], "string")
	}

	clock := mainFn.Body.Block(1).Ops[8].(*mir.AssignOp)
	clockIdent := clock.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(clockIdent.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "clock" {
		t.Fatalf("expected slot for clock, got %+v", slot)
	} else {
		fn := requireFuncType(t, slot.Type, 0, 1)
		requireNamedType(t, fn.Returns[0], "int")
	}
}

func TestLowerInfersPopItemType(t *testing.T) {
	program := mustLowerMIR(t, `use pop from list

func main()
  items: list[int] := [1, 2, 3]
  last := pop(items)
  write(last)
endfunc`)

	mainFn := program.Decls()[0].(*mir.Func)
	assign := mainFn.Body.Block(1).Ops[1].(*mir.AssignOp)
	last := assign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(last.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "last" {
		t.Fatalf("expected slot for last, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}

	call := requireValue(t, mainFn.Body, assign.ValueIDs[0], mir.ValueCall)
	if len(call.ReturnTypes) != 1 {
		t.Fatalf("call return count mismatch: got %d want 1", len(call.ReturnTypes))
	}
	requireNamedType(t, call.ReturnTypes[0], "int")
}

func TestLowerInfersRemoveAtItemType(t *testing.T) {
	program := mustLowerMIR(t, `use removeat from list

func main()
  items: list[int] := [10, 20, 30]
  removed := removeat(items, 1)
  write(removed)
endfunc`)

	mainFn := program.Decls()[0].(*mir.Func)
	assign := mainFn.Body.Block(1).Ops[1].(*mir.AssignOp)
	removed := assign.Targets[0].(*hir.Ident)
	if slot := mainFn.Body.SlotByBindingID(removed.Binding.ID); slot == nil || slot.Kind != mir.SlotLocal || slot.Name != "removed" {
		t.Fatalf("expected slot for removed, got %+v", slot)
	} else {
		requireNamedType(t, slot.Type, "int")
	}

	call := requireValue(t, mainFn.Body, assign.ValueIDs[0], mir.ValueCall)
	if len(call.ReturnTypes) != 1 {
		t.Fatalf("call return count mismatch: got %d want 1", len(call.ReturnTypes))
	}
	requireNamedType(t, call.ReturnTypes[0], "int")
}

func TestLowerBuildsExplicitValuesForMultiReturnCall(t *testing.T) {
	program := mustLowerMIR(t, `func pair(n: int) -> int, string
  return n, str(n)
endfunc

func main(x: int) -> int, string
  y, label := pair(x + 1)
  return y, label
endfunc`)

	fn := program.Decls()[1].(*mir.Func)
	assign := fn.Body.Block(1).Ops[0].(*mir.AssignOp)
	if len(assign.ValueIDs) != 2 {
		t.Fatalf("assign value id count mismatch: got %d want 2", len(assign.ValueIDs))
	}

	firstResult := requireValue(t, fn.Body, assign.ValueIDs[0], mir.ValueCallResult)
	requireNamedType(t, firstResult.Type, "int")
	secondResult := requireValue(t, fn.Body, assign.ValueIDs[1], mir.ValueCallResult)
	requireNamedType(t, secondResult.Type, "string")
	if firstResult.CallID != secondResult.CallID {
		t.Fatalf("expected both call results to reference same call, got %d and %d", firstResult.CallID, secondResult.CallID)
	}

	call := requireValue(t, fn.Body, firstResult.CallID, mir.ValueCall)
	if len(call.ReturnTypes) != 2 {
		t.Fatalf("call return count mismatch: got %d want 2", len(call.ReturnTypes))
	}
	requireNamedType(t, call.ReturnTypes[0], "int")
	requireNamedType(t, call.ReturnTypes[1], "string")

	callee := requireValue(t, fn.Body, call.Callee, mir.ValueBindingRef)
	if callee.Binding == nil || callee.Binding.Name != "pair" || callee.Binding.Kind != hir.BindingFunc {
		t.Fatalf("unexpected call callee binding: %+v", callee.Binding)
	}

	if len(call.Args) != 1 {
		t.Fatalf("call arg count mismatch: got %d want 1", len(call.Args))
	}
	arg := requireValue(t, fn.Body, call.Args[0], mir.ValueBinary)
	requireNamedType(t, arg.Type, "int")
	if arg.Op != "+" {
		t.Fatalf("binary op mismatch: got %q want +", arg.Op)
	}
	left := requireValue(t, fn.Body, arg.Left, mir.ValueSlotRef)
	if left.Binding == nil || left.Binding.Name != "x" {
		t.Fatalf("unexpected left operand binding: %+v", left.Binding)
	}
	right := requireValue(t, fn.Body, arg.Right, mir.ValueIntConst)
	if right.IntValue != 1 {
		t.Fatalf("expected int const 1, got %d", right.IntValue)
	}

	ret := fn.Body.Block(1).Term.(*mir.ReturnTerm)
	if len(ret.ValueIDs) != 2 {
		t.Fatalf("return value id count mismatch: got %d want 2", len(ret.ValueIDs))
	}
	retY := requireValue(t, fn.Body, ret.ValueIDs[0], mir.ValueSlotRef)
	if retY.Binding == nil || retY.Binding.Name != "y" {
		t.Fatalf("unexpected return y binding: %+v", retY.Binding)
	}
	retLabel := requireValue(t, fn.Body, ret.ValueIDs[1], mir.ValueSlotRef)
	if retLabel.Binding == nil || retLabel.Binding.Name != "label" {
		t.Fatalf("unexpected return label binding: %+v", retLabel.Binding)
	}

	if len(fn.Body.Block(1).Insts) != 4 {
		t.Fatalf("instruction count mismatch: got %d want 4", len(fn.Body.Block(1).Insts))
	}
	argInst, ok := fn.Body.Block(1).Insts[0].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected first instruction *mir.ComputeInst, got %T", fn.Body.Block(1).Insts[0])
	}
	if argInst.ValueID != call.Args[0] {
		t.Fatalf("argument compute mismatch: got %d want %d", argInst.ValueID, call.Args[0])
	}
	callInst, ok := fn.Body.Block(1).Insts[1].(*mir.CallInst)
	if !ok {
		t.Fatalf("expected second instruction *mir.CallInst, got %T", fn.Body.Block(1).Insts[1])
	}
	if callInst.ValueID != call.ID {
		t.Fatalf("call inst value mismatch: got %d want %d", callInst.ValueID, call.ID)
	}
	if len(callInst.ResultIDs) != len(assign.ValueIDs) {
		t.Fatalf("call result count mismatch: got %d want %d", len(callInst.ResultIDs), len(assign.ValueIDs))
	}
	for idx, resultID := range callInst.ResultIDs {
		if resultID != assign.ValueIDs[idx] {
			t.Fatalf("call result id mismatch at %d: got %d want %d", idx, resultID, assign.ValueIDs[idx])
		}
	}
	firstStore, ok := fn.Body.Block(1).Insts[2].(*mir.StoreInst)
	if !ok {
		t.Fatalf("expected third instruction *mir.StoreInst, got %T", fn.Body.Block(1).Insts[2])
	}
	if firstStore.PlaceID != assign.TargetPlaceIDs[0] || firstStore.ValueID != assign.ValueIDs[0] {
		t.Fatalf("first store mismatch: %+v", firstStore)
	}
	secondStore, ok := fn.Body.Block(1).Insts[3].(*mir.StoreInst)
	if !ok {
		t.Fatalf("expected fourth instruction *mir.StoreInst, got %T", fn.Body.Block(1).Insts[3])
	}
	if secondStore.PlaceID != assign.TargetPlaceIDs[1] || secondStore.ValueID != assign.ValueIDs[1] {
		t.Fatalf("second store mismatch: %+v", secondStore)
	}
}

func TestLowerBuildsExplicitValuesForModuleMemberCallAndCondition(t *testing.T) {
	program := mustLowerMIR(t, `use http

func handle(req: HttpRequest)
  matched, params := http.route(req, "/hello/:name")
  if matched then
    write(params["name"])
  endif
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	assign := fn.Body.Block(1).Ops[0].(*mir.AssignOp)
	if len(assign.ValueIDs) != 2 {
		t.Fatalf("assign value id count mismatch: got %d want 2", len(assign.ValueIDs))
	}

	matched := requireValue(t, fn.Body, assign.ValueIDs[0], mir.ValueCallResult)
	requireNamedType(t, matched.Type, "bool")
	params := requireValue(t, fn.Body, assign.ValueIDs[1], mir.ValueCallResult)
	dictType := requireGenericType(t, params.Type, "dict", 2)
	requireNamedType(t, dictType.Args[0], "string")
	requireNamedType(t, dictType.Args[1], "string")

	call := requireValue(t, fn.Body, matched.CallID, mir.ValueCall)
	if len(call.ReturnTypes) != 2 {
		t.Fatalf("route return count mismatch: got %d want 2", len(call.ReturnTypes))
	}
	member := requireValue(t, fn.Body, call.Callee, mir.ValueMember)
	if member.Member != "route" {
		t.Fatalf("member name mismatch: got %q want route", member.Member)
	}
	if member.MemberBinding == nil || member.MemberBinding.Kind != hir.MemberBindingModuleValue || member.MemberBinding.OwnerName != "http" {
		t.Fatalf("unexpected member binding: %+v", member.MemberBinding)
	}
	mod := requireValue(t, fn.Body, member.Object, mir.ValueBindingRef)
	if mod.Binding == nil || mod.Binding.Name != "http" || mod.Binding.Kind != hir.BindingModule {
		t.Fatalf("unexpected module binding ref: %+v", mod.Binding)
	}

	cond := fn.Body.Block(1).Term.(*mir.CondTerm)
	condValue := requireValue(t, fn.Body, cond.ConditionValue, mir.ValueSlotRef)
	if condValue.Binding == nil || condValue.Binding.Name != "matched" {
		t.Fatalf("unexpected condition binding: %+v", condValue.Binding)
	}
	requireNamedType(t, condValue.Type, "bool")
}

func TestLowerBuildsExplicitPlacesForIndexAssignment(t *testing.T) {
	program := mustLowerMIR(t, `func main()
  items := [1, 2, 3]
  items[1] := items[0] + 10
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	assign := fn.Body.Block(1).Ops[1].(*mir.AssignOp)
	if len(assign.TargetPlaceIDs) != 1 {
		t.Fatalf("assign target place count mismatch: got %d want 1", len(assign.TargetPlaceIDs))
	}
	place := requirePlace(t, fn.Body, assign.TargetPlaceIDs[0], mir.PlaceIndex)
	requireNamedType(t, place.Type, "int")

	object := requireValue(t, fn.Body, place.Object, mir.ValueSlotRef)
	if object.Binding == nil || object.Binding.Name != "items" {
		t.Fatalf("unexpected place object binding: %+v", object.Binding)
	}
	index := requireValue(t, fn.Body, place.Index, mir.ValueIntConst)
	if index.IntValue != 1 {
		t.Fatalf("expected place index 1, got %d", index.IntValue)
	}

	if len(assign.ValueIDs) != 1 {
		t.Fatalf("assign value id count mismatch: got %d want 1", len(assign.ValueIDs))
	}
	value := requireValue(t, fn.Body, assign.ValueIDs[0], mir.ValueBinary)
	requireNamedType(t, value.Type, "int")

	if len(fn.Body.Block(1).Insts) != 5 {
		t.Fatalf("instruction count mismatch: got %d want 5", len(fn.Body.Block(1).Insts))
	}
	indexInst, ok := fn.Body.Block(1).Insts[2].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected third instruction *mir.ComputeInst, got %T", fn.Body.Block(1).Insts[2])
	}
	indexValue := requireValue(t, fn.Body, indexInst.ValueID, mir.ValueIndex)
	if indexValue.Index == 0 {
		t.Fatalf("expected explicit index input for assignment value, got %+v", indexValue)
	}
	binaryInst, ok := fn.Body.Block(1).Insts[3].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected fourth instruction *mir.ComputeInst, got %T", fn.Body.Block(1).Insts[3])
	}
	if binaryInst.ValueID != assign.ValueIDs[0] {
		t.Fatalf("binary compute mismatch: got %d want %d", binaryInst.ValueID, assign.ValueIDs[0])
	}
	storeInst, ok := fn.Body.Block(1).Insts[4].(*mir.StoreInst)
	if !ok {
		t.Fatalf("expected fifth instruction *mir.StoreInst, got %T", fn.Body.Block(1).Insts[4])
	}
	if storeInst.PlaceID != assign.TargetPlaceIDs[0] || storeInst.ValueID != assign.ValueIDs[0] {
		t.Fatalf("index store mismatch: %+v", storeInst)
	}
}

func TestLowerBuildsExplicitPlacesForFieldAssignment(t *testing.T) {
	program := mustLowerMIR(t, `object Account
  balance: int
  new(balance: int) -> Account
    return Account{balance := balance}
  endnew
  func deposit(self: Account, amount: int)
    self.balance := self.balance + amount
  endfunc
endobject`)

	obj := program.Decls()[0].(*mir.Object)
	method := obj.Methods[0]
	assign := method.Body.Block(1).Ops[0].(*mir.AssignOp)
	if len(assign.TargetPlaceIDs) != 1 {
		t.Fatalf("assign target place count mismatch: got %d want 1", len(assign.TargetPlaceIDs))
	}
	place := requirePlace(t, method.Body, assign.TargetPlaceIDs[0], mir.PlaceField)
	requireNamedType(t, place.Type, "int")
	if place.Member != "balance" {
		t.Fatalf("place member mismatch: got %q want balance", place.Member)
	}
	if place.MemberBinding == nil || place.MemberBinding.Kind != hir.MemberBindingObjectField {
		t.Fatalf("unexpected field binding: %+v", place.MemberBinding)
	}

	object := requireValue(t, method.Body, place.Object, mir.ValueSlotRef)
	if object.Binding == nil || object.Binding.Name != "self" {
		t.Fatalf("unexpected field object binding: %+v", object.Binding)
	}

	value := requireValue(t, method.Body, assign.ValueIDs[0], mir.ValueBinary)
	if value.Op != "+" {
		t.Fatalf("binary op mismatch: got %q want +", value.Op)
	}
	requireNamedType(t, value.Type, "int")

	if len(method.Body.Block(1).Insts) != 3 {
		t.Fatalf("instruction count mismatch: got %d want 3", len(method.Body.Block(1).Insts))
	}
	memberInst, ok := method.Body.Block(1).Insts[0].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected first instruction *mir.ComputeInst, got %T", method.Body.Block(1).Insts[0])
	}
	memberValue := requireValue(t, method.Body, memberInst.ValueID, mir.ValueMember)
	if memberValue.Member != "balance" {
		t.Fatalf("member compute mismatch: got %q want balance", memberValue.Member)
	}
	binaryInst, ok := method.Body.Block(1).Insts[1].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected second instruction *mir.ComputeInst, got %T", method.Body.Block(1).Insts[1])
	}
	if binaryInst.ValueID != assign.ValueIDs[0] {
		t.Fatalf("binary compute mismatch: got %d want %d", binaryInst.ValueID, assign.ValueIDs[0])
	}
	storeInst, ok := method.Body.Block(1).Insts[2].(*mir.StoreInst)
	if !ok {
		t.Fatalf("expected third instruction *mir.StoreInst, got %T", method.Body.Block(1).Insts[2])
	}
	if storeInst.PlaceID != assign.TargetPlaceIDs[0] || storeInst.ValueID != assign.ValueIDs[0] {
		t.Fatalf("field store mismatch: %+v", storeInst)
	}
}

func TestLowerEmitsDeclareAndConditionInstructions(t *testing.T) {
	program := mustLowerMIR(t, `func main(x: int)
  total: int := x + 1
  if total > 2 then
    write(total)
  endif
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	entry := fn.Body.Block(1)
	decl := entry.Ops[0].(*mir.VarOp)
	cond := entry.Term.(*mir.CondTerm)

	if len(entry.Insts) != 3 {
		t.Fatalf("instruction count mismatch: got %d want 3", len(entry.Insts))
	}
	declValue := requireValue(t, fn.Body, decl.ValueID, mir.ValueBinary)
	if declValue.Op != "+" {
		t.Fatalf("declare value op mismatch: got %q want +", declValue.Op)
	}
	declCompute, ok := entry.Insts[0].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected first instruction *mir.ComputeInst, got %T", entry.Insts[0])
	}
	if declCompute.ValueID != decl.ValueID {
		t.Fatalf("declare compute mismatch: got %d want %d", declCompute.ValueID, decl.ValueID)
	}
	declareInst, ok := entry.Insts[1].(*mir.DeclareInst)
	if !ok {
		t.Fatalf("expected second instruction *mir.DeclareInst, got %T", entry.Insts[1])
	}
	if declareInst.PlaceID != decl.TargetPlaceID || declareInst.ValueID != decl.ValueID {
		t.Fatalf("declare inst mismatch: %+v", declareInst)
	}
	condValue := requireValue(t, fn.Body, cond.ConditionValue, mir.ValueBinary)
	if condValue.Op != ">" {
		t.Fatalf("condition value op mismatch: got %q want >", condValue.Op)
	}
	condCompute, ok := entry.Insts[2].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected third instruction *mir.ComputeInst, got %T", entry.Insts[2])
	}
	if condCompute.ValueID != cond.ConditionValue {
		t.Fatalf("condition compute mismatch: got %d want %d", condCompute.ValueID, cond.ConditionValue)
	}
}

func TestLowerHoistsForRangeInputsIntoPreheader(t *testing.T) {
	program := mustLowerMIR(t, `func main(x: int)
  for i in x + 1 to 3 do
    pass
  endfor
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	if len(fn.Body.Block(1).Insts) != 1 {
		t.Fatalf("preheader instruction count mismatch: got %d want 1", len(fn.Body.Block(1).Insts))
	}
	computeInst, ok := fn.Body.Block(1).Insts[0].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected preheader instruction *mir.ComputeInst, got %T", fn.Body.Block(1).Insts[0])
	}
	value := requireValue(t, fn.Body, computeInst.ValueID, mir.ValueBinary)
	if value.Op != "+" {
		t.Fatalf("preheader loop input op mismatch: got %q want +", value.Op)
	}
	if len(fn.Body.Block(2).Insts) != 0 {
		t.Fatalf("expected loop header to reuse hoisted inputs, got %d instructions", len(fn.Body.Block(2).Insts))
	}
}

func TestLowerHoistsForEachIterableIntoPreheader(t *testing.T) {
	program := mustLowerMIR(t, `func main()
  for item in [1, 2] do
    pass
  endfor
endfunc`)

	fn := program.Decls()[0].(*mir.Func)
	if len(fn.Body.Block(1).Insts) != 1 {
		t.Fatalf("preheader instruction count mismatch: got %d want 1", len(fn.Body.Block(1).Insts))
	}
	computeInst, ok := fn.Body.Block(1).Insts[0].(*mir.ComputeInst)
	if !ok {
		t.Fatalf("expected preheader instruction *mir.ComputeInst, got %T", fn.Body.Block(1).Insts[0])
	}
	value := requireValue(t, fn.Body, computeInst.ValueID, mir.ValueList)
	if len(value.Elements) != 2 {
		t.Fatalf("iterable element count mismatch: got %d want 2", len(value.Elements))
	}
	if len(fn.Body.Block(2).Insts) != 0 {
		t.Fatalf("expected foreach header to reuse hoisted iterable, got %d instructions", len(fn.Body.Block(2).Insts))
	}
}
