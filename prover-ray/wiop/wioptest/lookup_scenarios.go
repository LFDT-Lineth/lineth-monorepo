package wioptest

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// NewLookupSingleColumnNoFiltersScenario: a single-column inclusion query
// with no selectors on either side. Every S value appears in T.
func NewLookupSingleColumnNoFiltersScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-simple")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "SingleColumnNoFilters",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
			rt.AssignColumn(colS, makeVec(10, 20, 10, 30))
		},
	}
}

// NewLookupFilterOnIncludedScenario: a filter on the A (included) side
// masks bogus rows so that every active A row appears in T.
func NewLookupFilterOnIncludedScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-filterA")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 2, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	filterS := modS.NewColumn(sys.Context.Childf("filterS"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewFilteredTable(filterS.View(), colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "FilterOnIncluded",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 20))
			rt.AssignColumn(colS, makeVec(10, 99, 20, 99))
			rt.AssignColumn(filterS, makeVec(1, 0, 1, 0))
		},
	}
}

// NewLookupFilterOnIncludingScenario: a filter on the B (including) side
// masks rows out of the lookup table. Only un-masked T rows are reachable
// from A.
func NewLookupFilterOnIncludingScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-filterT")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), colT.View())},
	)

	return &LookupScenario{
		Name: "FilterOnIncluding",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 999, 20, 999))
			rt.AssignColumn(filterT, makeVec(1, 0, 1, 0))
			rt.AssignColumn(colS, makeVec(10, 20, 10, 20))
		},
	}
}

// NewLookupDoubleConditionalScenario: both A and B carry selectors; only
// (filterT-active T rows) × (filterS-active S rows) are involved.
func NewLookupDoubleConditionalScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-double")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	filterS := modS.NewColumn(sys.Context.Childf("filterS"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewFilteredTable(filterS.View(), colS.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), colT.View())},
	)

	return &LookupScenario{
		Name: "DoubleConditional",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 999, 20, 999))
			rt.AssignColumn(filterT, makeVec(1, 0, 1, 0))
			rt.AssignColumn(colS, makeVec(10, 0, 20, 7))
			rt.AssignColumn(filterS, makeVec(1, 0, 1, 0))
		},
	}
}

// NewLookupMultiColumnScenario: a width-2 inclusion query. The lookup
// compiler samples α for the multi-column RLC, then γ for denominator
// randomisation.
func NewLookupMultiColumnScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-multi-col")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)

	return &LookupScenario{
		Name: "MultiColumn",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
			rt.AssignColumn(ty, makeVec(10, 20, 30, 40))
			rt.AssignColumn(sx, makeVec(2, 2, 3, 1))
			rt.AssignColumn(sy, makeVec(20, 20, 30, 10))
		},
	}
}

// NewLookupSharedTableScenario: two inclusion queries share the same B
// fragment. The compiler must allocate exactly one M column for both
// queries (otherwise the B-side and A-side terms would not cancel).
func NewLookupSharedTableScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-shared")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 4, wiop.PaddingDirectionNone)
	modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
	colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)
	tabT := wiop.NewTable(colT.View())
	sys.NewInclusion(sys.Context.Childf("inc1"),
		[]wiop.Table{wiop.NewTable(colS1.View())}, []wiop.Table{tabT})
	sys.NewInclusion(sys.Context.Childf("inc2"),
		[]wiop.Table{wiop.NewTable(colS2.View())}, []wiop.Table{tabT})

	return &LookupScenario{
		Name: "SharedTable",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
			rt.AssignColumn(colS1, makeVec(10, 20, 10, 30))
			rt.AssignColumn(colS2, makeVec(40, 30))
		},
	}
}

// NewLookupDistinctTablesScenario: two inclusion queries against distinct
// tables. Each lookup table gets its own M column.
func NewLookupDistinctTablesScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-distinct")
	r0 := sys.NewRound()
	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 4, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 2, wiop.PaddingDirectionNone)
	modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 2, wiop.PaddingDirectionNone)
	modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)
	colT1 := modT1.NewColumn(sys.Context.Childf("T1"), wiop.VisibilityOracle, r0)
	colT2 := modT2.NewColumn(sys.Context.Childf("T2"), wiop.VisibilityOracle, r0)
	colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
	colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(sys.Context.Childf("inc1"),
		[]wiop.Table{wiop.NewTable(colS1.View())},
		[]wiop.Table{wiop.NewTable(colT1.View())})
	sys.NewInclusion(sys.Context.Childf("inc2"),
		[]wiop.Table{wiop.NewTable(colS2.View())},
		[]wiop.Table{wiop.NewTable(colT2.View())})

	return &LookupScenario{
		Name: "DistinctTables",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT1, makeVec(10, 20, 30, 40))
			rt.AssignColumn(colT2, makeVec(100, 200))
			rt.AssignColumn(colS1, makeVec(10, 30))
			rt.AssignColumn(colS2, makeVec(100, 200))
		},
	}
}

// NewLookupMultiColumnFilterOnIncludingScenario: the cross-product of
// multi-column lookup and an IsFilteredOnIncluding prepend. The compiler
// adds a constant-1 head to A and the B selector head to B before α-RLC.
func NewLookupMultiColumnFilterOnIncludingScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-multi-filterT")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), tx.View(), ty.View())},
	)

	return &LookupScenario{
		Name: "MultiColumnFilterOnIncluding",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 99, 2, 3))
			rt.AssignColumn(ty, makeVec(10, 99, 20, 30))
			rt.AssignColumn(filterT, makeVec(1, 0, 1, 1))
			// The fourth S row repeats (3, 30) so the table is still
			// over a power-of-two-sized module (size 3 is invalid for the
			// global compiler's FFT path).
			rt.AssignColumn(sx, makeVec(1, 2, 3, 3))
			rt.AssignColumn(sy, makeVec(10, 20, 30, 30))
		},
	}
}

// NewLookupRepeatedValueInTableScenario: T contains repeated values; M
// must reflect the per-row multiplicity, not just whether the value exists.
// This stresses the M-assignment task on a non-injective lookup table.
//
// S references value 10 (which appears at T rows 0 and 2) and value 20.
// Honest M: each T row that holds 10 is touched whenever S references 10;
// the M-assignment task may attribute repeated S values to any matching T
// row, but its choice must add up across A to the B side. We only assert
// that the pipeline accepts the witness — the precise M values are an
// implementation detail.
func NewLookupRepeatedValueInTableScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-repeated")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "RepeatedValueInTable",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			// T has 10 at two distinct positions.
			rt.AssignColumn(colT, makeVec(10, 20, 10, 30))
			rt.AssignColumn(colS, makeVec(10, 20, 10, 30))
		},
	}
}

// NewLookupShiftedAColumnScenario: the A side is read with a +1 shift,
// exercising the shift-aware row hashing in the M-assignment task and in
// downstream evaluation. The A-table rows after the cyclic shift must still
// all appear in T.
func NewLookupShiftedAColumnScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-shift-a")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View().Shift(1))}, // A reads col_S<<+1
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "ShiftedAColumn",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
			// After +1 shift, A logically reads [colS[1], colS[2], colS[3], colS[0]].
			rt.AssignColumn(colS, makeVec(40, 10, 20, 30)) // shifted = [10, 20, 30, 40]
		},
	}
}

// NewLookupShiftedBColumnScenario: the B (lookup table) side is read with
// a back-shift. The set of table rows is cyclically rotated; A must still
// match.
func NewLookupShiftedBColumnScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-shift-b")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View().Shift(-1))},
	)

	return &LookupScenario{
		Name: "ShiftedBColumn",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			// After -1 shift, B logically holds [colT[n-1], colT[0], colT[1], colT[2]].
			rt.AssignColumn(colT, makeVec(20, 30, 40, 10)) // shifted = [10, 20, 30, 40]
			rt.AssignColumn(colS, makeVec(10, 20, 30, 40))
		},
	}
}

// NewLookupMultipleAFragmentsScenario: a single inclusion query whose A
// side is the union of two fragments living on different modules. Each
// fragment contributes its own A-side fraction to the aggregated
// LogDerivativeSum, but only one M column is created on the shared B.
func NewLookupMultipleAFragmentsScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-multi-A")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 4, wiop.PaddingDirectionNone)
	modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
	colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{
			wiop.NewTable(colS1.View()),
			wiop.NewTable(colS2.View()),
		},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "MultipleAFragments",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
			rt.AssignColumn(colS1, makeVec(10, 10, 20, 30)) // all ∈ T
			rt.AssignColumn(colS2, makeVec(30, 40))         // all ∈ T
		},
	}
}

// NewLookupWidthThreeScenario: a width-3 inclusion query. The α-RLC has
// three symbolic terms, and the M-assignment task must hash three columns
// per row.
func NewLookupWidthThreeScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-w3")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 2, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	tz := modT.NewColumn(sys.Context.Childf("Tz"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)
	sz := modS.NewColumn(sys.Context.Childf("Sz"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View(), sz.View())},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View(), tz.View())},
	)

	return &LookupScenario{
		Name: "WidthThree",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
			rt.AssignColumn(ty, makeVec(10, 20, 30, 40))
			rt.AssignColumn(tz, makeVec(100, 200, 300, 400))
			rt.AssignColumn(sx, makeVec(2, 3))
			rt.AssignColumn(sy, makeVec(20, 30))
			rt.AssignColumn(sz, makeVec(200, 300))
		},
	}
}

// NewLookupSizeOneScenario: degenerate but valid — both A and B are
// size-1 modules. Exercises the row-iteration loop's lower bound.
func NewLookupSizeOneScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-size1")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 1, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 1, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "SizeOne",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(42))
			rt.AssignColumn(colS, makeVec(42))
		},
	}
}

// NewLookupPrecomputedTableScenario: the B side is a precomputed column
// (as produced by rangecheck.Compile). Exercises the precomputed-round
// fix in [lookupGroup.updateWitnessRound] that ensures M is committed to
// an interactive round rather than the PrecomputedRound.
func NewLookupPrecomputedTableScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-precomp")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewPrecomputedColumn(
		sys.Context.Childf("T"),
		wiop.VisibilityOracle,
		makeVec(10, 20, 30, 40),
	)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "PrecomputedTable",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			// colT is precomputed; nothing to assign for it here.
			rt.AssignColumn(colS, makeVec(10, 20, 30, 40))
		},
	}
}

// NewLookupRepeatedSValuesScenario: S references the same T row many
// times. The honest M for that T row should be |{i : S[i] = T[row]}|. This
// exercises the multiplicity counting on a non-trivial M value.
func NewLookupRepeatedSValuesScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-rep-s")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "RepeatedSValues",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
			// All four S rows pick the same T value → honest M = [4, 0, 0, 0].
			rt.AssignColumn(colS, makeVec(10, 10, 10, 10))
		},
	}
}

// NewLookupEmptySelectedScenario: every A-side filter is zero, so no A rows
// contribute. M must be all-zero; the aggregate is trivially zero. This
// exercises the "no active row" boundary case of the M-assignment task.
func NewLookupEmptySelectedScenario() *LookupScenario {
	sys := wiop.NewSystemf("lk-empty")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	filterS := modS.NewColumn(sys.Context.Childf("filterS"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewFilteredTable(filterS.View(), colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupScenario{
		Name: "EmptySelected",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
			// Random S values are fine because filterS masks all of them.
			rt.AssignColumn(colS, makeVec(7, 99, 0, 42))
			rt.AssignColumn(filterS, makeVec(0, 0, 0, 0))
		},
	}
}

// --- Soundness scenarios (M-assignment panics) -------------------------------

// NewLookupSingleColumnUnmatchedScenario: an active A row has no match in
// B. The M-assignment task panics.
func NewLookupSingleColumnUnmatchedScenario() *LookupSoundnessScenario {
	sys := wiop.NewSystemf("lk-unmatched")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupSoundnessScenario{
		Name: "SingleColumnUnmatched",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
			rt.AssignColumn(colS, makeVec(10, 99, 10, 30)) // 99 not in T
		},
	}
}

// NewLookupMultiColumnPartialMatchScenario: each column individually
// appears in T, but the tuple (Sx, Sy) does not appear pair-wise. The
// M-assignment task must panic.
func NewLookupMultiColumnPartialMatchScenario() *LookupSoundnessScenario {
	sys := wiop.NewSystemf("lk-partial")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 1, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)

	return &LookupSoundnessScenario{
		Name: "MultiColumnPartialMatch",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
			rt.AssignColumn(ty, makeVec(10, 20, 30, 40))
			// (1, 20) — never a row of T.
			rt.AssignColumn(sx, makeVec(1))
			rt.AssignColumn(sy, makeVec(20))
		},
	}
}

// NewLookupMaskedRowMatchScenario: an active A row hashes to a B row that
// is masked out by the B-side selector. The IsFilteredOnIncluding prepend
// trick makes their hashes diverge so the M-assignment task reports no
// match.
func NewLookupMaskedRowMatchScenario() *LookupSoundnessScenario {
	sys := wiop.NewSystemf("lk-masked")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 1, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), colT.View())},
	)

	return &LookupSoundnessScenario{
		Name: "MaskedRowMatch",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 99, 20, 30))
			rt.AssignColumn(filterT, makeVec(1, 0, 1, 1)) // row 1 masked
			rt.AssignColumn(colS, makeVec(99))            // tries to hit masked row
		},
	}
}

// NewLookupNonBinaryFilterScenario: the A-side selector carries a value
// other than 0 or 1. The M-assignment task rejects.
func NewLookupNonBinaryFilterScenario() *LookupSoundnessScenario {
	sys := wiop.NewSystemf("lk-nonbin")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	filterS := modS.NewColumn(sys.Context.Childf("filterS"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewFilteredTable(filterS.View(), colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	return &LookupSoundnessScenario{
		Name: "NonBinaryFilter",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
			rt.AssignColumn(colS, makeVec(10, 20, 30, 40))
			rt.AssignColumn(filterS, makeVec(1, 7, 1, 1)) // 7 is non-binary
		},
	}
}
