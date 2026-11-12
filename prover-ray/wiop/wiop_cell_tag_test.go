package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// TestAssignCell_UsesDeclaredTag checks that a cell value is stored with the
// cell's declared field tag, which fixes how many field elements it absorbs
// into the transcript: a base value is lifted into an extension cell, and an
// extension value is refused by a base cell.
func TestAssignCell_UsesDeclaredTag(t *testing.T) {
	sys := wiop.NewSystemf("cell-tags")
	r0 := sys.NewRound()
	extCell := r0.NewCell(sys.Context.Childf("ext"), true)
	baseCell := r0.NewCell(sys.Context.Childf("base"), false)
	lazyExt := r0.NewLazyCell(sys.Context.Childf("lazy-ext"), true, func(*wiop.Runtime) field.Gen {
		return field.ElemFromBase(field.NewElement(4))
	})

	rt := wiop.NewRuntime(sys)
	rt.AssignCell(extCell, field.ElemFromBase(field.NewElement(3)))
	got := rt.GetCellValue(extCell)
	require.False(t, got.IsBase(), "a base value in an extension cell must be lifted")
	require.Equal(t, field.Lift(field.NewElement(3)), got.AsExt())

	require.False(t, rt.GetCellValue(lazyExt).IsBase(), "a lazy cell must be lifted too")

	require.PanicsWithValue(t,
		`wiop: AssignCell: wiop: cell "cell-tags/base" is declared base-field but holds an extension-field value`,
		func() { rt.AssignCell(baseCell, field.ElemFromExt(field.Uint64ToExt(5))) })
}

// TestQueryResultCellsAreExtension pins the declarations that make cell tags a
// property of the System: a LagrangeEval claim at an extension point is an
// extension cell even for a base column, and a LogDerivativeSum result is
// always one.
func TestQueryResultCellsAreExtension(t *testing.T) {
	sys := wiop.NewSystemf("result-cells")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	require.False(t, col.View().IsExtension(), "fixture: a base column")

	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	require.True(t, le.EvaluationClaims[0].IsExtension(), "a claim at an extension point is an extension value")

	lds := sys.NewLogDerivativeSum(sys.Context.Childf("lds"), []wiop.Fraction{
		{Numerator: wiop.NewConstantField(field.One()), Denominator: col.View()},
	})
	require.True(t, lds.Result.IsExtension(), "a LogDerivativeSum result is always an extension value")
}
