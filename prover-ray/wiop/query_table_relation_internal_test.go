package wiop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateRowLimit_DynamicOverLimit is a white-box test of the verifier
// runtime row-limit check for the one scenario the compile-time static check
// cannot catch: a dynamic module whose runtime domain size exceeds the 2^22
// upper bound the static check assumes for dynamic modules.
//
// At compile time, [TableRelationQuery.PrecheckRowLimit] counts a dynamic
// module as [ColumnSizeMaxSupported] (2^22), so a single-fragment dynamic
// lookup always clears the budget and Compile does not reject it. The domain
// size a verifier actually sees, however, is read from the proof — a dishonest
// prover can claim a far larger dynamic domain. That is exactly what the
// verifier runtime check ([TableRelationQuery.ValidateRowLimit]) guards, and
// what the prover runtime check ([TableRelationQuery.CheckRowLimit]) panics on.
//
// The test is white-box (package wiop) because seeding an oversized dynamic
// size directly into the runtime — as [System.Verify] does from the proof's
// DynamicSizes before the row-limit action runs — needs access to the
// unexported dynamicSizes map; there is no public API to set a runtime dynamic
// size beyond the 2^22 assignment cap.
func TestValidateRowLimit_DynamicOverLimit(t *testing.T) {
	sys := NewSystemf("rowlimit-dyn-verifier")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 2, PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), VisibilityOracle, r0)

	dynMod := sys.NewDynamicModule(sys.Context.Childf("dynS"), PaddingDirectionRight)
	dynCol := dynMod.NewColumn(sys.Context.Childf("S"), VisibilityOracle, r0)

	inc := sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]Table{NewTable(dynCol.View())},
		[]Table{NewTable(colT.View())},
	)

	rt := NewRuntime(sys)
	// Simulate a proof claiming an oversized dynamic A-side domain: 2^30 rows,
	// far beyond the 2^22 the compile-time check assumed. Verify seeds
	// dynamicSizes from proof.DynamicSizes in the same way before running this
	// query's verifier action.
	rt.dynamicSizes[dynMod.index] = 1 << 30

	err := inc.ValidateRowLimit(rt, MaxLookupRows)
	assert.ErrorContains(t, err, "effective per-query row limit",
		"verifier must reject a lookup whose dynamic side claims more rows than the budget")

	assert.Panics(t, func() { inc.CheckRowLimit(rt, MaxLookupRows) },
		"prover must panic on the same over-limit dynamic side")
}

// TestValidateRowLimit_DynamicWithinCap confirms the complementary case: a
// dynamic side whose runtime domain stays within the 2^22 cap (as an honest
// assignment guarantees) passes the verifier runtime check, matching the
// compile-time static assumption.
func TestValidateRowLimit_DynamicWithinCap(t *testing.T) {
	sys := NewSystemf("rowlimit-dyn-ok")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 2, PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), VisibilityOracle, r0)

	dynMod := sys.NewDynamicModule(sys.Context.Childf("dynS"), PaddingDirectionRight)
	dynCol := dynMod.NewColumn(sys.Context.Childf("S"), VisibilityOracle, r0)

	inc := sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]Table{NewTable(dynCol.View())},
		[]Table{NewTable(colT.View())},
	)

	rt := NewRuntime(sys)
	rt.dynamicSizes[dynMod.index] = ColumnSizeMaxSupported // the maximum honest size.

	assert.NoError(t, inc.ValidateRowLimit(rt, MaxLookupRows),
		"a dynamic side at the 2^22 cap is well within the 2^30 budget")
}
