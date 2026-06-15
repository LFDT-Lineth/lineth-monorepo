package wioptest_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runRoundActions runs every prover action registered on rt's current round,
// mimicking what RunAndVerify does for the first round.
func runRoundActions(rt wiop.Runtime) {
	for _, a := range rt.CurrentRound().ProverActions {
		a.Run(rt)
	}
}

// TestMutator_ColumnEntry checks that a column-targeted Mutator corrupts only
// the targeted entry, leaving the rest of the assignment intact.
func TestMutator_ColumnEntry(t *testing.T) {
	sys := wiop.NewSystemf("col-mut")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)

	wioptest.Mutator{Column: col, Row: 2, Tweak: wioptest.AddOne}.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, sevenVec(4))
	runRoundActions(rt)

	got := rt.GetColumnAssignment(col).Plain.AsBase()
	assert.Equal(t, elem(8), got[2], "AddOne must bump the targeted entry to 8")
	for i, v := range got {
		if i == 2 {
			continue
		}
		assert.Equal(t, elem(7), v, "non-targeted entries must be unchanged")
	}
}

// TestMutator_Cell checks that a cell-targeted Mutator overrides the cell value.
func TestMutator_Cell(t *testing.T) {
	sys := wiop.NewSystemf("cell-mut")
	r0 := sys.NewRound()
	cell := r0.NewCell(sys.Context.Childf("cell"), false)

	wioptest.Mutator{Cell: cell, Tweak: wioptest.AddOne}.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignCell(cell, field.ElemFromBase(elem(5)))
	runRoundActions(rt)

	assert.Equal(t, field.ElemFromBase(elem(6)), rt.GetCellValue(cell),
		"AddOne must bump the cell value to 6")
}

// TestMutator_CustomTweak checks that a non-default Tweak is honored.
func TestMutator_CustomTweak(t *testing.T) {
	sys := wiop.NewSystemf("tweak-mut")
	r0 := sys.NewRound()
	cell := r0.NewCell(sys.Context.Childf("cell"), false)

	double := func(v field.Gen) field.Gen { return v.Add(v) }
	wioptest.Mutator{Cell: cell, Tweak: double}.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignCell(cell, field.ElemFromBase(elem(5)))
	runRoundActions(rt)

	assert.Equal(t, field.ElemFromBase(elem(10)), rt.GetCellValue(cell),
		"custom doubling tweak must produce 10")
}

// TestMutator_DefaultIsRandom checks that the default (nil) Tweak replaces the
// value with a different one and is reproducible: the same honest value maps to
// the same corrupted value on every run.
func TestMutator_DefaultIsRandom(t *testing.T) {
	mutate := func() field.Gen {
		sys := wiop.NewSystemf("rand-mut")
		r0 := sys.NewRound()
		cell := r0.NewCell(sys.Context.Childf("cell"), false)
		wioptest.Mutator{Cell: cell}.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignCell(cell, field.ElemFromBase(elem(5)))
		runRoundActions(rt)
		return rt.GetCellValue(cell)
	}

	got := mutate()
	assert.NotEqual(t, field.ElemFromBase(elem(5)), got,
		"the random default must change the value")
	assert.Equal(t, got, mutate(),
		"the random default must be reproducible for a given input")
}

// TestMutator_NegativeRow checks that a negative Row indexes from the end of
// the assigned Plain data, so Row == -1 hits the last entry.
func TestMutator_NegativeRow(t *testing.T) {
	sys := wiop.NewSystemf("neg-mut")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)

	wioptest.Mutator{Column: col, Row: -1, Tweak: wioptest.AddOne}.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, sevenVec(4))
	runRoundActions(rt)

	got := rt.GetColumnAssignment(col).Plain.AsBase()
	assert.Equal(t, elem(8), got[3], "Row -1 must hit the last entry")
	assert.Equal(t, elem(7), got[0], "earlier entries must be unchanged")
}

// TestMutator_Padding checks that targeting the padding fill value corrupts the
// padding constant, leaving the Plain data untouched.
func TestMutator_Padding(t *testing.T) {
	sys := wiop.NewSystemf("pad-mut")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)

	wioptest.Mutator{Column: col, Padding: true, Tweak: wioptest.AddOne}.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// Plain holds two entries; the module pads on the right up to size 4 with
	// the padding fill value, here zero.
	rt.AssignColumn(col, &wiop.ConcreteVector{
		Plain:   field.VecFromBase([]field.Element{elem(7), elem(7)}),
		Padding: elem(0),
	})
	runRoundActions(rt)

	got := rt.GetColumnAssignment(col)
	assert.Equal(t, elem(1), got.Padding, "AddOne must bump the padding value to 1")
	assert.Equal(t, []field.Element{elem(7), elem(7)}, got.Plain.AsBase(),
		"Plain data must be untouched when targeting padding")
}

// TestMutator_Compile_Panics covers the target-selection preconditions.
func TestMutator_Compile_Panics(t *testing.T) {
	sys := wiop.NewSystemf("panic-mut")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)

	require.Panics(t, func() { wioptest.Mutator{}.Compile(sys) },
		"a Mutator with no target must panic")
	require.Panics(t, func() { wioptest.Mutator{Column: col, Cell: cell}.Compile(sys) },
		"a Mutator targeting both a column and a cell must panic")
	require.Panics(t, func() { wioptest.Mutator{Column: col}.Compile(nil) },
		"Compile must reject a nil System")
	require.Panics(t, func() { wioptest.Mutator{Cell: cell, Padding: true}.Compile(sys) },
		"Padding must be rejected for a cell target")
}

// elem builds a base field element from a small integer.
func elem(v uint64) field.Element {
	var e field.Element
	e.SetUint64(v)
	return e
}

// sevenVec returns a length-n PaddingDirectionNone ConcreteVector of sevens.
func sevenVec(n int) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i] = elem(7)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}
