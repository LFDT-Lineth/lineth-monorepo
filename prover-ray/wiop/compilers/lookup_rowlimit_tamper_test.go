package compilers_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rowLimitVec builds a ConcreteVector holding vals in row order.
func rowLimitVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// TestFullPipeline_LookupRowLimit_TamperedDynamicSize is the end-to-end
// soundness companion to the compile-time and unit-level row-limit checks: it
// drives a real lookup through the full pipeline (including PCS), produces an
// honest proof, then forges the proof's claimed dynamic-module size to exceed
// the lookup row budget. The lookup pass's registered verifier action
// ([wiop.TableRelationQuery.ValidateRowLimit], wired by the lookuptologderivsum
// pass) must reject the forged proof.
//
// This is the one scenario the compile-time static check cannot catch: at
// compile time a dynamic module counts as [wiop.ColumnSizeMaxSupported] (2^22),
// so the single-fragment lookup clears the budget and compilation succeeds. The
// domain size a verifier sees, however, comes from the proof — and PCS hides the
// oracle A-side column, so nothing during transcript replay pins the dynamic
// module's size to its honest value. A dishonest prover can therefore claim a
// far larger domain, and only the verifier's runtime row-limit action stands in
// the way.
//
// The row-limit action is registered on the lookup's (early) witness round, so
// it runs ahead of the coin-dependent PCS / LogDerivativeSum-result checks that
// the forged size would also break; the row-limit error is the one surfaced.
func TestFullPipeline_LookupRowLimit_TamperedDynamicSize(t *testing.T) {
	sys := wiop.NewSystemf("ll-tamper")
	r0 := sys.NewRound()

	// B (including) side: a tiny static lookup table {7, 7}.
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 2, wiop.PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)

	// A (included) side: a dynamic module. At compile time it is assumed to be
	// at most 2^22 rows, so the lookup passes the compile-time budget check.
	dynMod := sys.NewDynamicModule(sys.Context.Childf("dynS"), wiop.PaddingDirectionRight)
	colS := dynMod.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	// Locate the dynamic module's index (its position in sys.Modules, which is
	// the key used in Proof.DynamicSizes) before compilation adds more modules.
	dynIdx := -1
	for i, m := range sys.Modules {
		if m == dynMod {
			dynIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, dynIdx, 0, "dynamic module must be registered in sys.Modules")

	compileFullPipeline(sys)

	// Honest witness: every A row (7) is in the lookup table {7, 7}.
	assign := func(rt *wiop.Runtime) {
		rt.AssignColumn(colT, rowLimitVec(7, 7))
		rt.AssignColumn(colS, rowLimitVec(7, 7)) // dynamic module → honest size 2.
	}

	proof, pub := sys.Prove(assign)
	require.NoError(t, sys.Verify(proof, pub),
		"sanity: the honest lookup proof must verify")

	// Forge the claimed dynamic domain to 2^30 rows (a power of two, so it clears
	// the domain-shape checks) — at or above the lookup row budget MaxLookupRows.
	require.Contains(t, proof.DynamicSizes, dynIdx, "proof must carry the dynamic module size")
	proof.DynamicSizes[dynIdx] = 1 << 30

	err := sys.Verify(proof, pub)
	assert.ErrorContains(t, err, "effective per-query row limit",
		"the verifier row-limit action must reject a proof claiming an over-budget dynamic domain")
}
