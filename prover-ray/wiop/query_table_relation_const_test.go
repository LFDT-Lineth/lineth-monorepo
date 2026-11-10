package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Helpers ----

// baseVec builds a ConcreteVector from the given uint64 values.
func baseVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// cst builds a vector constant with value v over m.
func cst(m *wiop.Module, v uint64) *wiop.Constant {
	var e field.Element
	e.SetUint64(v)
	return wiop.NewConstantVector(m, e)
}

// ---- Constructors ----

func TestNewTableEntries_MixedEntries(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	tab := wiop.NewTableEntries(mod, nil, cst(mod, 1), col.View())
	assert.Equal(t, mod, tab.Module())
	assert.Equal(t, r0, tab.Round())
	assert.Equal(t, 2, tab.Width())
	assert.Nil(t, tab.Selector)

	v, ok := wiop.EntryConst(tab.Columns[0])
	require.True(t, ok)
	assert.True(t, v.IsOne())
	_, ok = wiop.EntryView(tab.Columns[0])
	assert.False(t, ok)
	cv, ok := wiop.EntryView(tab.Columns[1])
	require.True(t, ok)
	assert.Equal(t, col, cv.Column)
}

func TestNewTableEntries_ConstantFirstEntryKeepsModule(t *testing.T) {
	// Table.Module must not be derived from the first entry: a constant there
	// is fine because the module is stored explicitly.
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	tab := wiop.NewTableEntries(mod, nil, cst(mod, 0), cst(mod, 1), col.View())
	assert.Equal(t, mod, tab.Module())
}

func TestNewTableEntries_ConstOneSelectorNormalisedToNil(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	tab := wiop.NewTableEntries(mod, cst(mod, 1), col.View())
	assert.Nil(t, tab.Selector, "a constant-1 selector selects every row and must be normalised to nil")
}

func TestNewTableEntries_ColumnSelector(t *testing.T) {
	sys, _, r1, mod := newTestSystem(t)
	sel := mod.NewColumn(sys.Context.Childf("sel"), r1)
	tab := wiop.NewTableEntries(mod, sel.View(), cst(mod, 3))
	assert.Equal(t, sel, tab.Selector.Column)
	assert.Equal(t, r1, tab.Round(), "an all-constant table takes its round from the selector")
}

func TestNewTableEntries_Panics(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	other := sys.NewSizedModule(sys.Context.Childf("other"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	otherCol := other.NewColumn(sys.Context.Childf("otherCol"), r0)

	cases := map[string]func(){
		"nil module":          func() { wiop.NewTableEntries(nil, nil, col.View()) },
		"no entries":          func() { wiop.NewTableEntries(mod, nil) },
		"constant-0 selector": func() { wiop.NewTableEntries(mod, cst(mod, 0), col.View()) },
		"non-binary selector": func() { wiop.NewTableEntries(mod, cst(mod, 2), col.View()) },
		"scalar constant entry": func() {
			wiop.NewTableEntries(mod, nil, col.View(), wiop.NewConstantField(field.One()))
		},
		"constant from other module":  func() { wiop.NewTableEntries(mod, nil, col.View(), cst(other, 1)) },
		"column from other module":    func() { wiop.NewTableEntries(mod, nil, otherCol.View()) },
		"selector from other module":  func() { wiop.NewTableEntries(mod, otherCol.View(), col.View()) },
		"all constants, no selector":  func() { wiop.NewTableEntries(mod, nil, cst(mod, 1), cst(mod, 2)) },
		"all constants, const-1 sel.": func() { wiop.NewTableEntries(mod, cst(mod, 1), cst(mod, 1)) },
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Panics(t, f, "NewTableEntries must reject a table with %s", name)
		})
	}
}

// ---- Inclusion ----

// newConstInclusion builds A = (aCol, aConst?) ⊆ B = (bCol, bConst?) on a
// PaddingDirectionNone module of size 4. A nil const pointer means the
// position is a column (aCst / bCst) instead.
func newConstInclusion(t *testing.T, aConst, bConst *uint64) (sys *wiop.System, inc *wiop.TableRelationQuery, cols map[string]*wiop.Column) {
	t.Helper()
	sys = wiop.NewSystemf("constInc")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	cols = map[string]*wiop.Column{
		"a": mod.NewColumn(sys.Context.Childf("a"), r0),
		"b": mod.NewColumn(sys.Context.Childf("b"), r0),
	}
	entry := func(name string, c *uint64) wiop.TableEntry {
		if c != nil {
			return cst(mod, *c)
		}
		cols[name] = mod.NewColumn(sys.Context.Childf("%s", name), r0)
		return cols[name].View()
	}
	tabA := wiop.NewTableEntries(mod, nil, cols["a"].View(), entry("aCst", aConst))
	tabB := wiop.NewTableEntries(mod, nil, cols["b"].View(), entry("bCst", bConst))
	inc = sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})
	return sys, inc, cols
}

func u64(v uint64) *uint64 { return &v }

func TestInclusion_ConstOnA(t *testing.T) {
	check := func(aConst uint64) error {
		sys, inc, cols := newConstInclusion(t, u64(aConst), nil)
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(cols["a"], baseVec(1, 2, 1, 2))
		rt.AssignColumn(cols["b"], baseVec(1, 2, 3, 4))
		rt.AssignColumn(cols["bCst"], baseVec(7, 7, 0, 0))
		return inc.Check(rt)
	}
	t.Run("match", func(t *testing.T) {
		require.NoError(t, check(7), "(1, 7) and (2, 7) are rows of B")
	})
	t.Run("wrong constant", func(t *testing.T) {
		require.Error(t, check(8), "A's constant 8 is not in B's second column")
	})
}

func TestInclusion_ConstOnB(t *testing.T) {
	check := func(aSecond *wiop.ConcreteVector) error {
		sys, inc, cols := newConstInclusion(t, nil, u64(5))
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(cols["a"], baseVec(3, 3, 4, 4))
		rt.AssignColumn(cols["aCst"], aSecond)
		rt.AssignColumn(cols["b"], baseVec(1, 2, 3, 4))
		return inc.Check(rt)
	}
	t.Run("match", func(t *testing.T) {
		require.NoError(t, check(baseVec(5, 5, 5, 5)), "every A row carries B's constant 5")
	})
	t.Run("missing tuple", func(t *testing.T) {
		require.Error(t, check(baseVec(5, 5, 5, 6)), "B cannot contain (4, 6): its second position is always 5")
	})
}

func TestInclusion_ConstOnBothSides(t *testing.T) {
	check := func(aConst, bConst uint64) error {
		sys, inc, cols := newConstInclusion(t, u64(aConst), u64(bConst))
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(cols["a"], baseVec(4, 3, 2, 1))
		rt.AssignColumn(cols["b"], baseVec(1, 2, 3, 4))
		return inc.Check(rt)
	}
	t.Run("equal constants", func(t *testing.T) {
		require.NoError(t, check(9, 9), "A is a reordering of B with the same constant")
	})
	t.Run("distinct constants", func(t *testing.T) {
		require.Error(t, check(9, 10), "distinct constants on the two sides never match")
	})
}

func TestInclusion_ConstMultiFragmentB(t *testing.T) {
	// B = F1 ∪ F2 where F1 = (b1, 1) and F2 = (b2, b2Tag). A = (a, aTag).
	sys := wiop.NewSystemf("constMultiB")
	r0 := sys.NewRound()
	m1 := sys.NewSizedModule(sys.Context.Childf("m1"), 4, wiop.PaddingDirectionNone)
	m2 := sys.NewSizedModule(sys.Context.Childf("m2"), 2, wiop.PaddingDirectionNone)
	mA := sys.NewSizedModule(sys.Context.Childf("mA"), 4, wiop.PaddingDirectionNone)
	b1 := m1.NewColumn(sys.Context.Childf("b1"), r0)
	b2 := m2.NewColumn(sys.Context.Childf("b2"), r0)
	b2Tag := m2.NewColumn(sys.Context.Childf("b2Tag"), r0)
	a := mA.NewColumn(sys.Context.Childf("a"), r0)
	aTag := mA.NewColumn(sys.Context.Childf("aTag"), r0)

	inc := sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(a.View(), aTag.View())},
		[]wiop.Table{
			wiop.NewTableEntries(m1, nil, b1.View(), cst(m1, 1)),
			wiop.NewTable(b2.View(), b2Tag.View()),
		},
	)

	assign := func(aTagVals *wiop.ConcreteVector) *wiop.Runtime {
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(b1, baseVec(10, 11, 12, 13))
		rt.AssignColumn(b2, baseVec(20, 21))
		rt.AssignColumn(b2Tag, baseVec(2, 2))
		rt.AssignColumn(a, baseVec(10, 13, 20, 21))
		rt.AssignColumn(aTag, aTagVals)
		return rt
	}
	t.Run("match", func(t *testing.T) {
		require.NoError(t, inc.Check(assign(baseVec(1, 1, 2, 2))), "each A row is in one of the fragments")
	})
	t.Run("in neither fragment", func(t *testing.T) {
		require.Error(t, inc.Check(assign(baseVec(1, 2, 2, 2))), "(13, 2) is in neither fragment")
	})
}

func TestInclusion_ConstWithColumnSelectorOnB(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	b := mod.NewColumn(sys.Context.Childf("b"), r0)
	sel := mod.NewColumn(sys.Context.Childf("sel"), r0)
	inc := sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTableEntries(mod, nil, cst(mod, 4), a.View())},
		[]wiop.Table{wiop.NewTableEntries(mod, sel.View(), cst(mod, 4), b.View())},
	)
	check := func(aVals *wiop.ConcreteVector) error {
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(a, aVals)
		rt.AssignColumn(b, baseVec(1, 2, 3, 4))
		rt.AssignColumn(sel, baseVec(1, 1, 0, 0))
		return inc.Check(rt)
	}
	t.Run("match", func(t *testing.T) {
		require.NoError(t, check(baseVec(1, 1, 2, 2)), "(4, 1) and (4, 2) sit on selected B rows")
	})
	t.Run("unselected row", func(t *testing.T) {
		require.Error(t, check(baseVec(1, 1, 2, 3)), "(4, 3) sits on an unselected B row")
	})
}

// ---- Padding semantics ----
//
// A constant is c on every row, padding included, whereas a committed column
// pads with its padding value (0 here). On a padded module an unfiltered
// fragment with a constant entry therefore has padding rows (0, c), not (0, 0).

// newPaddedConstInclusion builds an unfiltered inclusion A ⊆ B where one side
// lives on a left-padded module of size 4 with 2 data rows and carries a
// constant 1 in its second position, and the other side is a plain 2-column
// table on an unpadded module of size 4.
//
// If constOnA, A = (x, 1) [padded] and B = (u, v); otherwise A = (u, v) and
// B = (x, 1) [padded].
func newPaddedConstInclusion(t *testing.T, constOnA bool) (*wiop.System, *wiop.TableRelationQuery, map[string]*wiop.Column) {
	t.Helper()
	sys := wiop.NewSystemf("constPad")
	r0 := sys.NewRound()
	padded := sys.NewSizedModule(sys.Context.Childf("padded"), 4, wiop.PaddingDirectionLeft)
	plain := sys.NewSizedModule(sys.Context.Childf("plain"), 4, wiop.PaddingDirectionNone)
	cols := map[string]*wiop.Column{
		"x": padded.NewColumn(sys.Context.Childf("x"), r0),
		"u": plain.NewColumn(sys.Context.Childf("u"), r0),
		"v": plain.NewColumn(sys.Context.Childf("v"), r0),
	}
	constTab := wiop.NewTableEntries(padded, nil, cols["x"].View(), cst(padded, 1))
	plainTab := wiop.NewTable(cols["u"].View(), cols["v"].View())
	tabA, tabB := plainTab, constTab
	if constOnA {
		tabA, tabB = constTab, plainTab
	}
	inc := sys.NewInclusion(sys.Context.Childf("inc"), []wiop.Table{tabA}, []wiop.Table{tabB})
	return sys, inc, cols
}

func TestInclusion_ConstPadding_UnfilteredA(t *testing.T) {
	sys, inc, cols := newPaddedConstInclusion(t, true)
	check := func(u, v *wiop.ConcreteVector) error {
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(cols["x"], baseVec(5, 6)) // A = {(0,1), (0,1), (5,1), (6,1)}
		rt.AssignColumn(cols["u"], u)
		rt.AssignColumn(cols["v"], v)
		return inc.Check(rt)
	}
	t.Run("padding row absent from B", func(t *testing.T) {
		// B contains every A data row and (0, 0), which is what a committed
		// column fixed to 1 on data rows would pad with, but not (0, 1).
		require.Error(t, check(baseVec(5, 6, 0, 7), baseVec(1, 1, 0, 1)),
			"A's padding row (0, 1) is absent from B")
	})
	t.Run("padding row present in B", func(t *testing.T) {
		require.NoError(t, check(baseVec(5, 6, 0, 7), baseVec(1, 1, 1, 1)),
			"adding (0, 1) to B covers A's padding rows")
	})
}

func TestInclusion_ConstPadding_UnfilteredB(t *testing.T) {
	sys, inc, cols := newPaddedConstInclusion(t, false)
	check := func(u, v *wiop.ConcreteVector) error {
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(cols["x"], baseVec(5, 6)) // B = {(0,1), (0,1), (5,1), (6,1)}
		rt.AssignColumn(cols["u"], u)
		rt.AssignColumn(cols["v"], v)
		return inc.Check(rt)
	}
	t.Run("matched by padding", func(t *testing.T) {
		require.NoError(t, check(baseVec(0, 5, 6, 0), baseVec(1, 1, 1, 1)),
			"(0, 1) is matched by B's padding rows only")
	})
	t.Run("zero row not in padding", func(t *testing.T) {
		// (0, 0) would match a committed column's padding, but not a constant's.
		require.Error(t, check(baseVec(0, 5, 6, 0), baseVec(0, 1, 1, 1)),
			"B's padding row is (0, 1), not (0, 0)")
	})
}

// ---- Permutation ----

func TestPermutation_Constants(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	b := mod.NewColumn(sys.Context.Childf("b"), r0)
	bTag := mod.NewColumn(sys.Context.Childf("bTag"), r0)
	perm := sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTableEntries(mod, nil, a.View(), cst(mod, 3))},
		[]wiop.Table{wiop.NewTable(b.View(), bTag.View())},
	)

	check := func(tag *wiop.ConcreteVector) error {
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(a, baseVec(1, 2, 3, 4))
		rt.AssignColumn(b, baseVec(4, 3, 2, 1))
		rt.AssignColumn(bTag, tag)
		return perm.Check(rt)
	}
	t.Run("permutation", func(t *testing.T) {
		require.NoError(t, check(baseVec(3, 3, 3, 3)), "B is a reordering of (A, 3)")
	})
	t.Run("not a permutation", func(t *testing.T) {
		require.Error(t, check(baseVec(3, 3, 3, 2)), "(1, 2) has no counterpart on A")
	})
}
