package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Soundness (scenario-driven) ----

func TestInclusion_Soundness_Completeness(t *testing.T) {
	sc := wioptest.NewInclusionScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunHonest(rt)
	require.NoError(t, sc.Query.Check(rt), "honest witness must pass Check")
}

func TestInclusion_Soundness_InvalidWitness(t *testing.T) {
	sc := wioptest.NewInclusionScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunInvalid(rt)
	require.Error(t, sc.Query.Check(rt), "invalid witness must be rejected by Check")
}

// ---- Table constructors ----

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

// ---- Inclusion ----

func TestInclusion_Check_Match(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	colA := mod.NewColumn(sys.Context.Childf("incA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("incB"), wiop.VisibilityOracle, r0)

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
	require.Error(t, err)
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

// ---- Inclusion under padding ----

func TestInclusion_PaddingLeft_Match(t *testing.T) {
	sys := wiop.NewSystemf("incPadL")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionLeft)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	tabA := wiop.NewTable(colA.View())
	tabB := wiop.NewTable(colB.View())
	inc := sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})

	rt := wiop.NewRuntime(sys)
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
	require.Error(t, err)
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
	require.Error(t, err)
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
	require.Error(t, err)
}

// ---- Row limit (runtime + compile-time static forms) ----

// newRowLimitInclusion builds a single-column inclusion S ⊆ T whose A and B
// sides live on static modules of the given sizes, and returns the query and a
// fresh runtime. Static modules report their size via RuntimeSize without any
// assignment, so the runtime is usable by the row-limit checks as-is.
func newRowLimitInclusion(t *testing.T, aSize, bSize int) (*wiop.TableRelationQuery, *wiop.Runtime) {
	t.Helper()
	sys := wiop.NewSystemf("rowlimit")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), bSize, wiop.PaddingDirectionRight)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), aSize, wiop.PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	inc := sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)
	return inc, wiop.NewRuntime(sys)
}

// TestValidateRowLimit_UnderLimit confirms the runtime check accepts a lookup
// whose per-side totals stay strictly below the limit.
func TestValidateRowLimit_UnderLimit(t *testing.T) {
	inc, rt := newRowLimitInclusion(t, 4, 8)
	require.NoError(t, inc.ValidateRowLimit(rt, wiop.MaxLookupRows))
	assert.NotPanics(t, func() { inc.CheckRowLimit(rt, wiop.MaxLookupRows) })
}

// TestValidateRowLimit_OverLimit confirms the runtime check rejects (error) and
// CheckRowLimit panics when a side reaches the limit.
func TestValidateRowLimit_OverLimit(t *testing.T) {
	incA, rtA := newRowLimitInclusion(t, 1<<30, 2) // A side reaches the 2^30 bound.
	require.ErrorContains(t, incA.ValidateRowLimit(rtA, wiop.MaxLookupRows), "effective per-query row limit")
	assert.Panics(t, func() { incA.CheckRowLimit(rtA, wiop.MaxLookupRows) })

	incB, rtB := newRowLimitInclusion(t, 2, 1<<30) // B side reaches the bound.
	require.ErrorContains(t, incB.ValidateRowLimit(rtB, wiop.MaxLookupRows), "effective per-query row limit")
}

// TestPrevalidateRowLimit_SizedModules confirms the compile-time check matches
// the runtime form for static modules (their static size == runtime size).
func TestPrevalidateRowLimit_SizedModules(t *testing.T) {
	incOK, _ := newRowLimitInclusion(t, 4, 8)
	require.NoError(t, incOK.PrevalidateRowLimit(wiop.MaxLookupRows))
	assert.NotPanics(t, func() { incOK.PrecheckRowLimit(wiop.MaxLookupRows) })

	incBad, _ := newRowLimitInclusion(t, 1<<30, 2)
	require.ErrorContains(t, incBad.PrevalidateRowLimit(wiop.MaxLookupRows), "effective per-query row limit")
	assert.Panics(t, func() { incBad.PrecheckRowLimit(wiop.MaxLookupRows) })
}

// TestPrevalidateRowLimit_DynamicCountsAsMax confirms a dynamic module is
// counted as its maximum height ColumnSizeMaxSupported (2^22) by the
// compile-time check, so a single dynamic fragment stays under MaxLookupRows
// (2^30) while enough dynamic fragments to exceed it are rejected at compile
// time even though no runtime sizes are known.
func TestPrevalidateRowLimit_DynamicCountsAsMax(t *testing.T) {
	sys := wiop.NewSystemf("rowlimit-dyn")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 2, wiop.PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)

	// A single dynamic A fragment: counted as 2^22, well under 2^30 → passes.
	dynMod := sys.NewDynamicModule(sys.Context.Childf("dynS"), wiop.PaddingDirectionRight)
	dynCol := dynMod.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	inc := sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(dynCol.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)
	require.NoError(t, inc.PrevalidateRowLimit(wiop.MaxLookupRows),
		"one dynamic fragment (counted as 2^22) is under the 2^30 budget")

	// 2^30 / 2^22 = 256 dynamic fragments on the A side reach the budget.
	aFrags := make([]wiop.Table, 256)
	for i := range aFrags {
		m := sys.NewDynamicModule(sys.Context.Childf("dynA%d", i), wiop.PaddingDirectionRight)
		c := m.NewColumn(sys.Context.Childf("A%d", i), wiop.VisibilityOracle, r0)
		aFrags[i] = wiop.NewTable(c.View())
	}
	incMany := sys.NewInclusion(
		sys.Context.Childf("incMany"),
		aFrags,
		[]wiop.Table{wiop.NewTable(colT.View())},
	)
	require.ErrorContains(t, incMany.PrevalidateRowLimit(wiop.MaxLookupRows), "effective per-query row limit",
		"256 dynamic fragments counted as 2^22 each reach the 2^30 budget")
}
