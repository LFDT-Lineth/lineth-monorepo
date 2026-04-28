package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Soundness ----

func TestPermutation_Soundness_Completeness(t *testing.T) {
	sc := wioptest.NewPermutationScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunHonest(&rt)
	require.NoError(t, sc.Query.Check(rt), "honest witness must pass Check")
}

func TestPermutation_Soundness_InvalidWitness(t *testing.T) {
	sc := wioptest.NewPermutationScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunInvalid(&rt)
	assert.Error(t, sc.Query.Check(rt), "invalid witness must be rejected by Check")
}

func TestInclusion_Soundness_Completeness(t *testing.T) {
	sc := wioptest.NewInclusionScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunHonest(&rt)
	require.NoError(t, sc.Query.Check(rt), "honest witness must pass Check")
}

func TestInclusion_Soundness_InvalidWitness(t *testing.T) {
	sc := wioptest.NewInclusionScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunInvalid(&rt)
	assert.Error(t, sc.Query.Check(rt), "invalid witness must be rejected by Check")
}

func TestNewTable_Basic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("tblCol"), wiop.VisibilityOracle, r0)
	tab := wiop.NewTable(col.View())
	assert.Equal(t, mod, tab.Module())
	assert.Equal(t, r0, tab.Round())
	assert.Equal(t, 1, tab.Width())
	assert.Nil(t, tab.Selector)
}

func TestNewTable_EmptyPanic(t *testing.T) {
	assert.Panics(t, func() { wiop.NewTable() })
}

func TestNewTable_MixedModulePanic(t *testing.T) {
	sys := wiop.NewSystemf("tblMix")
	r := sys.NewRound()
	mod1 := sys.NewSizedModule(sys.Context.Childf("m1"), 4, wiop.PaddingDirectionNone)
	mod2 := sys.NewSizedModule(sys.Context.Childf("m2"), 4, wiop.PaddingDirectionNone)
	c1 := mod1.NewColumn(sys.Context.Childf("c1"), wiop.VisibilityOracle, r)
	c2 := mod2.NewColumn(sys.Context.Childf("c2"), wiop.VisibilityOracle, r)
	assert.Panics(t, func() { wiop.NewTable(c1.View(), c2.View()) })
}

func TestNewFilteredTable_Basic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("ftCol"), wiop.VisibilityOracle, r0)
	sel := mod.NewColumn(sys.Context.Childf("ftSel"), wiop.VisibilityOracle, r0)
	tab := wiop.NewFilteredTable(sel.View(), col.View())
	assert.Equal(t, sel.View().Column, tab.Selector.Column)
}

func TestNewFilteredTable_NilSelectorPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("ftNilCol"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() { wiop.NewFilteredTable(nil, col.View()) })
}

func TestNewFilteredTable_SelectorMixedModulePanic(t *testing.T) {
	sys := wiop.NewSystemf("ftsMix")
	r := sys.NewRound()
	mod1 := sys.NewSizedModule(sys.Context.Childf("m1"), 4, wiop.PaddingDirectionNone)
	mod2 := sys.NewSizedModule(sys.Context.Childf("m2"), 4, wiop.PaddingDirectionNone)
	c := mod1.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r)
	s := mod2.NewColumn(sys.Context.Childf("s"), wiop.VisibilityOracle, r)
	assert.Panics(t, func() { wiop.NewFilteredTable(s.View(), c.View()) })
}

func TestTableRelationKind_String(t *testing.T) {
	assert.Equal(t, "Permutation", wiop.TableRelationPermutation.String())
	assert.Equal(t, "Inclusion", wiop.TableRelationInclusion.String())
	assert.Contains(t, wiop.TableRelationKind(99).String(), "TableRelationKind")
}

// ---- Permutation ----

func TestPermutation_Check_Match(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	colA := mod.NewColumn(sys.Context.Childf("permA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("permB"), wiop.VisibilityOracle, r0)

	// A and B are same constant vector → trivially a permutation of each other
	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	perm := sys.NewPermutation(sys.Context.Childf("perm"), []wiop.Table{tabA}, []wiop.Table{tabB})
	require.NotNil(t, perm)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, baseVec(4, 7))
	rt.AssignColumn(colB, baseVec(4, 7))

	require.NoError(t, perm.Check(rt))
}

func TestPermutation_Check_Mismatch(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	colA := mod.NewColumn(sys.Context.Childf("permMisA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("permMisB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	perm := sys.NewPermutation(sys.Context.Childf("permMis"), []wiop.Table{tabA}, []wiop.Table{tabB})

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, baseVec(4, 1))
	rt.AssignColumn(colB, baseVec(4, 2)) // different values

	err := perm.Check(rt)
	assert.Error(t, err)
}

func TestPermutation_Round(t *testing.T) {
	sys := wiop.NewSystemf("permRound")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod0 := sys.NewSizedModule(sys.Context.Childf("m0"), 4, wiop.PaddingDirectionNone)
	mod1 := sys.NewSizedModule(sys.Context.Childf("m1"), 4, wiop.PaddingDirectionNone)
	cA := mod0.NewColumn(sys.Context.Childf("cA"), wiop.VisibilityOracle, r0)
	cB := mod1.NewColumn(sys.Context.Childf("cB"), wiop.VisibilityOracle, r1)
	tabA := wiop.NewTable(cA.View())
	tabB := wiop.NewTable(cB.View())
	perm := sys.NewPermutation(sys.Context.Childf("perm"), []wiop.Table{tabA}, []wiop.Table{tabB})
	assert.Equal(t, r1, perm.Round())
}

func TestPermutation_NewPermutation_NilCtxPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("p"), wiop.VisibilityOracle, r0)
	tab := wiop.NewTable(col.View())
	assert.Panics(t, func() { sys.NewPermutation(nil, []wiop.Table{tab}, []wiop.Table{tab}) })
}

func TestPermutation_NewPermutation_EmptyAPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("pEA"), wiop.VisibilityOracle, r0)
	tab := wiop.NewTable(col.View())
	assert.Panics(t, func() {
		sys.NewPermutation(sys.Context.Childf("q"), nil, []wiop.Table{tab})
	})
}

func TestPermutation_NewPermutation_SelectorPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("pSel"), wiop.VisibilityOracle, r0)
	sel := mod.NewColumn(sys.Context.Childf("pSelSel"), wiop.VisibilityOracle, r0)
	tab := wiop.NewFilteredTable(sel.View(), col.View())
	colB := mod.NewColumn(sys.Context.Childf("pSelB"), wiop.VisibilityOracle, r0)
	tabB := wiop.NewTable(colB.View())
	assert.Panics(t, func() {
		sys.NewPermutation(sys.Context.Childf("q"), []wiop.Table{tab}, []wiop.Table{tabB})
	})
}

func TestPermutation_NewPermutation_EmptyBPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("pEB"), wiop.VisibilityOracle, r0)
	tab := wiop.NewTable(col.View())
	assert.Panics(t, func() {
		sys.NewPermutation(sys.Context.Childf("q"), []wiop.Table{tab}, nil)
	})
}

func TestPermutation_NewPermutation_WidthMismatchPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	c1 := mod.NewColumn(sys.Context.Childf("pW1"), wiop.VisibilityOracle, r0)
	c2 := mod.NewColumn(sys.Context.Childf("pW2"), wiop.VisibilityOracle, r0)
	c3 := mod.NewColumn(sys.Context.Childf("pW3"), wiop.VisibilityOracle, r0)
	tab1 := wiop.NewTable(c1.View())
	tab2 := wiop.NewTable(c2.View(), c3.View()) // width 2 vs 1
	assert.Panics(t, func() {
		sys.NewPermutation(sys.Context.Childf("q"), []wiop.Table{tab1}, []wiop.Table{tab2})
	})
}

// ---- Inclusion ----

func TestInclusion_Check_Match(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	colA := mod.NewColumn(sys.Context.Childf("incA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("incB"), wiop.VisibilityOracle, r0)

	// A ⊆ B: both are same values → trivially included
	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})
	require.NotNil(t, inc)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, baseVec(4, 3))
	rt.AssignColumn(colB, baseVec(4, 3))

	require.NoError(t, inc.Check(rt))
}

func TestInclusion_Check_Mismatch(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	colA := mod.NewColumn(sys.Context.Childf("incMisA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("incMisB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("incMis"), []wiop.Table{tabA}, []wiop.Table{tabB})

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, baseVec(4, 1))
	rt.AssignColumn(colB, baseVec(4, 2)) // A not in B

	err := inc.Check(rt)
	assert.Error(t, err)
}

func TestInclusion_NewInclusion_NilCtxPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("incNilC"), wiop.VisibilityOracle, r0)
	tab := wiop.NewTable(col.View())
	assert.Panics(t, func() { sys.NewInclusion(nil, []wiop.Table{tab}, []wiop.Table{tab}) })
}

func TestInclusion_NewInclusion_EmptyIncludingPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("incEIng"), wiop.VisibilityOracle, r0)
	tab := wiop.NewTable(col.View())
	assert.Panics(t, func() {
		sys.NewInclusion(sys.Context.Childf("q"), []wiop.Table{tab}, nil)
	})
}

func TestInclusion_NewInclusion_WidthMismatchPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	c1 := mod.NewColumn(sys.Context.Childf("incWMc1"), wiop.VisibilityOracle, r0)
	c2 := mod.NewColumn(sys.Context.Childf("incWMc2"), wiop.VisibilityOracle, r0)
	c3 := mod.NewColumn(sys.Context.Childf("incWMc3"), wiop.VisibilityOracle, r0)
	included := wiop.NewTable(c1.View())
	including := wiop.NewTable(c2.View(), c3.View()) // width 2 vs 1
	assert.Panics(t, func() {
		sys.NewInclusion(sys.Context.Childf("q"), []wiop.Table{included}, []wiop.Table{including})
	})
}

func TestInclusion_NewInclusion_EmptyIncludedPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("incEI"), wiop.VisibilityOracle, r0)
	tab := wiop.NewTable(col.View())
	assert.Panics(t, func() {
		sys.NewInclusion(sys.Context.Childf("q"), nil, []wiop.Table{tab})
	})
}

// ---- Permutation with padding (covers padAnchorRow, tableHasZeroShift) ----

func TestPermutation_Check_PaddingRight(t *testing.T) {
	sys := wiop.NewSystemf("permPad")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionRight)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	perm := sys.NewPermutation(sys.Context.Childf("perm"), []wiop.Table{tabA}, []wiop.Table{tabB})

	// 2-element data padded to 4 with zeros
	var e field.Element
	e.SetUint64(5)
	data := []field.Element{e, e}
	vecA := &wiop.ConcreteVector{Plain: field.VecFromBase(data)}
	vecB := &wiop.ConcreteVector{Plain: field.VecFromBase(data)}

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, vecA)
	rt.AssignColumn(colB, vecB)

	require.NoError(t, perm.Check(rt))
}

// ---- Inclusion with selector (covers inclusionBuildSet/inclusionCheckSet selector branch) ----

func TestInclusion_Check_WithSelector(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	colA := mod.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("selB"), wiop.VisibilityOracle, r0)
	selA := mod.NewColumn(sys.Context.Childf("selAsel"), wiop.VisibilityOracle, r0)

	// selA = [1,0,0,0] → only first row of A is selected
	// A[0] = 1, B is all-1 → first row of A is in B
	tabA := wiop.NewFilteredTable(selA.View(), colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("incSel"), []wiop.Table{tabA}, []wiop.Table{tabB})

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, baseVec(4, 1))
	rt.AssignColumn(colB, baseVec(4, 1))

	// sel = [1,0,0,0]
	var one, zero field.Element
	one.SetUint64(1)
	selVec := &wiop.ConcreteVector{Plain: field.VecFromBase([]field.Element{one, zero, zero, zero})}
	rt.AssignColumn(selA, selVec)

	require.NoError(t, inc.Check(rt))
}

func TestInclusion_PaddingLeft_Match(t *testing.T) {
	sys := wiop.NewSystemf("incPadL")
	r0 := sys.NewRound()
	// 4-element module with left padding
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionLeft)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})

	rt := wiop.NewRuntime(sys)
	// Both have 3-element assignments padded left to 4 with zero
	var v field.Element
	v.SetUint64(7)
	elems := []field.Element{v, v, v}
	vA := &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
	vB := &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
	rt.AssignColumn(colA, vA)
	rt.AssignColumn(colB, vB)

	require.NoError(t, inc.Check(rt))
}

func TestInclusion_PaddingRight_Match(t *testing.T) {
	sys := wiop.NewSystemf("incPadR")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionRight)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})

	rt := wiop.NewRuntime(sys)
	var v field.Element
	v.SetUint64(5)
	elems := []field.Element{v, v, v}
	vA := &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
	vB := &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
	rt.AssignColumn(colA, vA)
	rt.AssignColumn(colB, vB)

	require.NoError(t, inc.Check(rt))
}

func TestInclusion_PaddingLeft_Mismatch(t *testing.T) {
	sys := wiop.NewSystemf("incPadMis")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionLeft)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})

	rt := wiop.NewRuntime(sys)
	var v1, v2 field.Element
	v1.SetUint64(1)
	v2.SetUint64(9)
	elemsA := []field.Element{v1, v1, v1}
	elemsB := []field.Element{v2, v2, v2}
	vA := &wiop.ConcreteVector{Plain: field.VecFromBase(elemsA)}
	vB := &wiop.ConcreteVector{Plain: field.VecFromBase(elemsB)}
	rt.AssignColumn(colA, vA)
	rt.AssignColumn(colB, vB)

	err := inc.Check(rt)
	assert.Error(t, err)
}

// ---- Inclusion with selector + padding (covers the selector path in inclusionBuildSet) ----

func TestInclusion_PaddingRight_WithSelector_Match(t *testing.T) {
	sys := wiop.NewSystemf("incPadSel")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionRight)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	selA := mod.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)

	// Selector selects only data rows (index 0,1), not padding rows.
	var one, zero field.Element
	one.SetUint64(1)
	selElems := []field.Element{one, one}
	selVec := &wiop.ConcreteVector{Plain: field.VecFromBase(selElems)}

	var v field.Element
	v.SetUint64(3)
	dataElems := []field.Element{v, v}
	dataVec := &wiop.ConcreteVector{Plain: field.VecFromBase(dataElems)}

	tabA := wiop.NewFilteredTable(selA.View(), colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, dataVec)
	rt.AssignColumn(colB, dataVec)
	rt.AssignColumn(selA, selVec)

	require.NoError(t, inc.Check(rt))
	_ = zero
}

// ---- Permutation with padding + PaddingDirectionLeft (padAnchorRow Left branch) ----

func TestPermutation_PaddingLeft_Match(t *testing.T) {
	sys := wiop.NewSystemf("permPadL")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionLeft)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	perm := sys.NewPermutation(sys.Context.Childf("perm"), []wiop.Table{tabA}, []wiop.Table{tabB})

	var v field.Element
	v.SetUint64(4)
	elems := []field.Element{v, v, v}
	vA := &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
	vB := &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, vA)
	rt.AssignColumn(colB, vB)

	require.NoError(t, perm.Check(rt))
}

// TestPermutation_Check_PaddingLeft_Mismatch exercises the padding-aware
// batching code in permTableCountInto when the data rows differ: the counts
// map ends up with non-zero entries and Check must return an error.
func TestPermutation_Check_PaddingLeft_Mismatch(t *testing.T) {
	sys := wiop.NewSystemf("permPadLMis")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionLeft)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	perm := sys.NewPermutation(sys.Context.Childf("perm"), []wiop.Table{tabA}, []wiop.Table{tabB})

	var v1, v2 field.Element
	v1.SetUint64(3)
	v2.SetUint64(7)
	elemsA := []field.Element{v1, v1, v1}
	elemsB := []field.Element{v2, v2, v2}
	vA := &wiop.ConcreteVector{Plain: field.VecFromBase(elemsA)}
	vB := &wiop.ConcreteVector{Plain: field.VecFromBase(elemsB)}

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, vA)
	rt.AssignColumn(colB, vB)

	err := perm.Check(rt)
	assert.Error(t, err)
}

// TestPermutation_Check_PaddingRight_Mismatch is the right-padding analogue.
func TestPermutation_Check_PaddingRight_Mismatch(t *testing.T) {
	sys := wiop.NewSystemf("permPadRMis")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionRight)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	perm := sys.NewPermutation(sys.Context.Childf("perm"), []wiop.Table{tabA}, []wiop.Table{tabB})

	var v1, v2 field.Element
	v1.SetUint64(2)
	v2.SetUint64(5)
	elemsA := []field.Element{v1, v1, v1}
	elemsB := []field.Element{v2, v2, v2}
	vA := &wiop.ConcreteVector{Plain: field.VecFromBase(elemsA)}
	vB := &wiop.ConcreteVector{Plain: field.VecFromBase(elemsB)}

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, vA)
	rt.AssignColumn(colB, vB)

	err := perm.Check(rt)
	assert.Error(t, err)
}

// TestInclusion_PaddingRight_Mismatch exercises inclusionCheckSet under right
// padding: A has a data row value not present in B's selected rows.
func TestInclusion_PaddingRight_Mismatch(t *testing.T) {
	sys := wiop.NewSystemf("incPadRMis")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionRight)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})

	var v1, v2 field.Element
	v1.SetUint64(1)
	v2.SetUint64(9)
	elemsA := []field.Element{v1, v1, v1}
	elemsB := []field.Element{v2, v2, v2}
	vA := &wiop.ConcreteVector{Plain: field.VecFromBase(elemsA)}
	vB := &wiop.ConcreteVector{Plain: field.VecFromBase(elemsB)}
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, vA)
	rt.AssignColumn(colB, vB)

	err := inc.Check(rt)
	assert.Error(t, err)
}

// TestInclusion_PaddingRight_WithSelector_Mismatch exercises the selector path
// in inclusionCheckSet under right padding: the selected A row value is absent
// from B.
func TestInclusion_PaddingRight_WithSelector_Mismatch(t *testing.T) {
	sys := wiop.NewSystemf("incPadSelMis")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionRight)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	selA := mod.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewFilteredTable(selA.View(), colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})

	// A data = [1,1], B data = [9,9] → selected A rows are absent from B.
	var one field.Element
	one.SetUint64(1)
	var nine field.Element
	nine.SetUint64(9)

	selVec := &wiop.ConcreteVector{Plain: field.VecFromBase([]field.Element{one, one})}
	vA := &wiop.ConcreteVector{Plain: field.VecFromBase([]field.Element{one, one})}
	vB := &wiop.ConcreteVector{Plain: field.VecFromBase([]field.Element{nine, nine})}

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, vA)
	rt.AssignColumn(colB, vB)
	rt.AssignColumn(selA, selVec)

	err := inc.Check(rt)
	assert.Error(t, err)
}
