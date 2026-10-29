package wioptest

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Scenarios whose tables carry vector constants (see [wiop.TableEntry]) next
// to, or instead of, column views. They are registered in
// [LookupScenarios], [LookupSoundnessScenarios] and [PermutationScenarios].

// constVector returns the vector constant v over m.
func constVector(m *wiop.Module, v uint64) *wiop.Constant {
	return wiop.NewConstantVector(m, field.NewElement(v))
}

// NewLookupConstOnAScenario: (Sx, 20) ⊆ (Tx, Ty), the constant on the A side
// in the last position.
func NewLookupConstOnAScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-const-a")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTableEntries(modS, nil, sx.View(), constVector(modS, 20))},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)

	return &LookupScenario{
		Name: "ConstOnA",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
			rt.AssignColumn(ty, makeVec(10, 20, 20, 40))
			rt.AssignColumn(sx, makeVec(2, 3, 3, 2))
		},
	}
}

// NewLookupConstOnBScenario: (Sx, Sy) ⊆ (5, Tx), the constant on the B side
// in the first position (so the table's module is not derivable from its
// first entry).
func NewLookupConstOnBScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-const-b")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewTableEntries(modT, nil, constVector(modT, 5), tx.View())},
	)

	return &LookupScenario{
		Name: "ConstOnB",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
			rt.AssignColumn(sx, makeVec(5, 5, 5, 5))
			rt.AssignColumn(sy, makeVec(4, 1, 1, 3))
		},
	}
}

// NewLookupConstBothSidesScenario: (9, Sx, 1) ⊆ (9, Tx, 1) with a filter on
// the B side, so the prepend-selector path runs with constant entries.
func NewLookupConstBothSidesScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-const-both")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), r0)
	selT := modT.NewColumn(sys.Context.Childf("selT"), r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTableEntries(modS, nil, constVector(modS, 9), sx.View(), constVector(modS, 1))},
		[]wiop.Table{wiop.NewTableEntries(modT, selT.View(), constVector(modT, 9), tx.View(), constVector(modT, 1))},
	)

	return &LookupScenario{
		Name: "ConstBothSides",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
			rt.AssignColumn(selT, makeVec(1, 0, 1, 1))
			rt.AssignColumn(sx, makeVec(4, 1, 1, 3))
		},
	}
}

// NewLookupConstMultiFragmentBScenario: (Sx, Sy) ⊆ (T1, 1) ∪ (T2, T2y), the
// constant in one B fragment only.
func NewLookupConstMultiFragmentBScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-const-multi-b")
	r0 := sys.NewRound()
	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 4, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 2, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	t1 := modT1.NewColumn(sys.Context.Childf("T1"), r0)
	t2 := modT2.NewColumn(sys.Context.Childf("T2"), r0)
	t2y := modT2.NewColumn(sys.Context.Childf("T2y"), r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{
			wiop.NewTableEntries(modT1, nil, t1.View(), constVector(modT1, 1)),
			wiop.NewTable(t2.View(), t2y.View()),
		},
	)

	return &LookupScenario{
		Name: "ConstMultiFragmentB",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(t1, makeVec(10, 11, 12, 13))
			rt.AssignColumn(t2, makeVec(20, 21))
			rt.AssignColumn(t2y, makeVec(2, 2))
			rt.AssignColumn(sx, makeVec(10, 13, 20, 21))
			rt.AssignColumn(sy, makeVec(1, 1, 2, 2))
		},
	}
}

// NewLookupConstOnlyFilteredAScenario: a filtered A fragment whose single
// entry is a constant, (sel; 7) ⊆ (T). Width 1, so no α is drawn and the
// constant is the whole RLC.
func NewLookupConstOnlyFilteredAScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-const-only-a")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), r0)
	selS := modS.NewColumn(sys.Context.Childf("selS"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTableEntries(modS, selS.View(), constVector(modS, 7))},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "ConstOnlyFilteredA",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(5, 6, 7, 8))
			rt.AssignColumn(selS, makeVec(1, 0, 1, 1))
		},
	}
}

// NewLookupConstPaddedFilteredAScenario mirrors how zkc lowers a call with a
// literal argument: a left-padded caller module whose A fragment is
// (sel; Sx, 1). The selector filters out the padding rows, where the constant
// would otherwise read 1 rather than the column padding value 0.
func NewLookupConstPaddedFilteredAScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-const-padded")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 8, wiop.PaddingDirectionLeft)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), r0)
	selS := modS.NewColumn(sys.Context.Childf("selS"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTableEntries(modS, selS.View(), sx.View(), constVector(modS, 1))},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)

	return &LookupScenario{
		Name: "ConstPaddedFilteredA",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
			rt.AssignColumn(ty, makeVec(1, 1, 0, 1))
			// 5 data rows, 3 left-padding rows (Sx = 0, selS = 0).
			rt.AssignColumn(sx, makeVec(1, 2, 4, 4, 9))
			rt.AssignColumn(selS, makeVec(1, 1, 1, 1, 0))
		},
	}
}

// NewLookupConstWrongValueScenario: (Sx, 8) ⊆ (Tx, Ty) where Ty never holds 8,
// so no A row has a match in B.
func NewLookupConstWrongValueScenario() *LookupSoundnessScenario {
	sys := wiop.NewSystemf("lk-const-wrong")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 1, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTableEntries(modS, nil, sx.View(), constVector(modS, 8))},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)

	return &LookupSoundnessScenario{
		Name: "ConstWrongValue",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
			rt.AssignColumn(ty, makeVec(7, 7, 7, 7))
			rt.AssignColumn(sx, makeVec(2))
		},
	}
}

// NewLookupConstMissingTupleScenario: (Sx, Sy) ⊆ (Tx, 5) where one A row has
// Sy = 6, a tuple B cannot contain.
func NewLookupConstMissingTupleScenario() *LookupSoundnessScenario {
	sys := wiop.NewSystemf("lk-const-missing")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 2, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewTableEntries(modT, nil, tx.View(), constVector(modT, 5))},
	)

	return &LookupSoundnessScenario{
		Name: "ConstMissingTuple",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
			rt.AssignColumn(sx, makeVec(1, 2))
			rt.AssignColumn(sy, makeVec(5, 6))
		},
	}
}

// NewLookupConstPaddingUnfilteredAScenario: the padding pitfall. An
// unfiltered, left-padded A fragment (Sx, 1) has padding rows (0, 1) — the
// constant is 1 on every row — and B = (Tx, Ty) contains every A data row and
// (0, 0), but not (0, 1). A committed column fixed to 1 on the data rows would
// have padded with (0, 0) and passed.
func NewLookupConstPaddingUnfilteredAScenario() *LookupSoundnessScenario {
	sys := wiop.NewSystemf("lk-const-padding")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionLeft)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTableEntries(modS, nil, sx.View(), constVector(modS, 1))},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)

	return &LookupSoundnessScenario{
		Name: "ConstPaddingUnfilteredA",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(5, 6, 0, 7))
			rt.AssignColumn(ty, makeVec(1, 1, 0, 1))
			rt.AssignColumn(sx, makeVec(5, 6)) // 2 padding rows read (0, 1)
		},
	}
}

// NewPermutationConstScenario: (A0, 3) is a permutation of (B0, B1). The
// invalid witness puts a 2 in B1, which A's constant can never produce.
func NewPermutationConstScenario() *PermutationScenario {
	sys := wiop.NewSystemf("perm-const")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	a0 := modA.NewColumn(sys.Context.Childf("A0"), r0)
	b0 := modB.NewColumn(sys.Context.Childf("B0"), r0)
	b1 := modB.NewColumn(sys.Context.Childf("B1"), r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTableEntries(modA, nil, a0.View(), constVector(modA, 3))},
		[]wiop.Table{wiop.NewTable(b0.View(), b1.View())},
	)

	return &PermutationScenario{
		Name: "Const",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(a0, makeVec(1, 2, 3, 4))
			rt.AssignColumn(b0, makeVec(4, 3, 2, 1))
			rt.AssignColumn(b1, makeVec(3, 3, 3, 3))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(a0, makeVec(1, 2, 3, 4))
			rt.AssignColumn(b0, makeVec(4, 3, 2, 1))
			rt.AssignColumn(b1, makeVec(3, 3, 3, 2))
		},
	}
}

// NewPermutationConstOnlyFilteredScenario: a conditional single-width
// permutation whose A fragment is a lone constant under a selector,
// (selA; 5) ≡ (selB; B). Every fragment has width 1, so no α is drawn.
func NewPermutationConstOnlyFilteredScenario() *PermutationScenario {
	sys := wiop.NewSystemf("perm-const-only")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	selA := modA.NewColumn(sys.Context.Childf("selA"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	selB := modB.NewColumn(sys.Context.Childf("selB"), r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTableEntries(modA, selA.View(), constVector(modA, 5))},
		[]wiop.Table{wiop.NewFilteredTable(selB.View(), colB.View())},
	)

	return &PermutationScenario{
		Name: "ConstOnlyFiltered",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(selA, makeVec(1, 0, 1, 0))
			rt.AssignColumn(colB, makeVec(5, 9, 9, 5))
			rt.AssignColumn(selB, makeVec(1, 0, 0, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(selA, makeVec(1, 0, 1, 0))
			rt.AssignColumn(colB, makeVec(5, 9, 9, 5))
			rt.AssignColumn(selB, makeVec(1, 1, 0, 1))
		},
	}
}
