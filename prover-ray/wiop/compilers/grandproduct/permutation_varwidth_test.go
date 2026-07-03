package grandproduct_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompile_Permutation_MixedWidth_Balanced covers a permutation whose A and
// B sides mix a width-2 fragment and a width-1 fragment. Each width sub-group is
// a reordering of its counterpart, so — with the α^w length sentinel keeping the
// widths from interfering — the grand product is one and the verifier accepts.
func TestCompile_Permutation_MixedWidth_Balanced(t *testing.T) {
	sys := wiop.NewSystemf("gp-mixed-width")
	r0 := sys.NewRound()

	// Width-2 sub-group.
	modA2 := sys.NewSizedModule(sys.Context.Childf("modA2"), 2, wiop.PaddingDirectionNone)
	modB2 := sys.NewSizedModule(sys.Context.Childf("modB2"), 2, wiop.PaddingDirectionNone)
	keyA := modA2.NewColumn(sys.Context.Childf("keyA"), wiop.VisibilityOracle, r0)
	valA := modA2.NewColumn(sys.Context.Childf("valA"), wiop.VisibilityOracle, r0)
	keyB := modB2.NewColumn(sys.Context.Childf("keyB"), wiop.VisibilityOracle, r0)
	valB := modB2.NewColumn(sys.Context.Childf("valB"), wiop.VisibilityOracle, r0)

	// Width-1 sub-group.
	modA1 := sys.NewSizedModule(sys.Context.Childf("modA1"), 2, wiop.PaddingDirectionNone)
	modB1 := sys.NewSizedModule(sys.Context.Childf("modB1"), 2, wiop.PaddingDirectionNone)
	colA1 := modA1.NewColumn(sys.Context.Childf("A1"), wiop.VisibilityOracle, r0)
	colB1 := modB1.NewColumn(sys.Context.Childf("B1"), wiop.VisibilityOracle, r0)

	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{
			wiop.NewTable(keyA.View(), valA.View()), // width 2
			wiop.NewTable(colA1.View()),             // width 1
		},
		[]wiop.Table{
			wiop.NewTable(keyB.View(), valB.View()), // width 2
			wiop.NewTable(colB1.View()),             // width 1
		},
	)

	grandproduct.Compile(sys)

	proof := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(keyA, makeVecU64(1, 2))
		rt.AssignColumn(valA, makeVecU64(10, 20))
		rt.AssignColumn(keyB, makeVecU64(2, 1)) // reordering of the width-2 rows
		rt.AssignColumn(valB, makeVecU64(20, 10))
		rt.AssignColumn(colA1, makeVecU64(5, 6))
		rt.AssignColumn(colB1, makeVecU64(6, 5)) // reordering of the width-1 rows
	})
	require.NoError(t, sys.Verify(proof),
		"a mixed-width permutation whose per-width sub-groups match must be accepted")
}

// TestCompile_Permutation_MixedWidth_SentinelPreventsAliasing is the soundness
// counterpart: side A holds the width-1 rows (5),(6); side B tries to match them
// with the width-2 rows (0,5),(0,6). Without the α^w sentinel both fold to the
// same value and the grand product would spuriously equal one; the sentinel
// gives the width-1 rows the α^1 term and the width-2 rows the α^2 term, so the
// product differs from one and the verifier rejects.
func TestCompile_Permutation_MixedWidth_SentinelPreventsAliasing(t *testing.T) {
	sys := wiop.NewSystemf("gp-alias")
	r0 := sys.NewRound()

	modA1 := sys.NewSizedModule(sys.Context.Childf("modA1"), 2, wiop.PaddingDirectionNone)
	modB2 := sys.NewSizedModule(sys.Context.Childf("modB2"), 2, wiop.PaddingDirectionNone)
	colA1 := modA1.NewColumn(sys.Context.Childf("A1"), wiop.VisibilityOracle, r0)
	hiB := modB2.NewColumn(sys.Context.Childf("hiB"), wiop.VisibilityOracle, r0)
	loB := modB2.NewColumn(sys.Context.Childf("loB"), wiop.VisibilityOracle, r0)

	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA1.View())},           // width 1
		[]wiop.Table{wiop.NewTable(hiB.View(), loB.View())}, // width 2, zero-padded
	)

	grandproduct.Compile(sys)

	proof := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA1, makeVecU64(5, 6))
		rt.AssignColumn(hiB, makeVecU64(0, 0))
		rt.AssignColumn(loB, makeVecU64(5, 6))
	})
	assert.Error(t, sys.Verify(proof),
		"the length sentinel must stop a width-2 (0,v) row from aliasing a width-1 v row")
}
