package lookuptologderivsum

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildRuntimeLimitGroup builds a single-column inclusion S ⊆ T with the given
// static A and B module sizes, bins it into subgroups, and returns the sole
// subgroup together with a fresh Runtime. Sizes are metadata only (no 2^30
// vector is ever materialised), and RuntimeSize reads a static module's size
// directly, so this drives RowLimitVerifierAction without a witness.
func buildRuntimeLimitGroup(t *testing.T, aSize, bSize int) (*lookupGroup, *wiop.Runtime) {
	t.Helper()
	sys := wiop.NewSystemf("rt-limit")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), bSize, wiop.PaddingDirectionRight)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), aSize, wiop.PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	groups := collectGroups(sys)
	require.Len(t, groups, 1, "one inclusion against one table must bin into one subgroup")
	return groups[0], wiop.NewRuntime(sys)
}

// rowLimitActionFor builds the RowLimitVerifierAction for g the same way
// registerRowLimitChecks does, so tests exercise the exported AModules/BModules
// path rather than a stale group-only construction.
func rowLimitActionFor(g *lookupGroup) *RowLimitVerifierAction {
	aModules := make([]*wiop.Module, len(g.included))
	for i, inc := range g.included {
		aModules[i] = inc.cols[0].Module()
	}
	bModules := make([]*wiop.Module, len(g.includings))
	for i, it := range g.includings {
		bModules[i] = it.module
	}
	return &RowLimitVerifierAction{group: g, AModules: aModules, BModules: bModules}
}

// TestRowLimitVerifierAction_RuntimeRejectsOverLimitASide checks that the runtime
// subgroup row-limit check fires on the A side: as a verifier action it returns
// an error, and as a prover action it panics. The A module is declared with 2^30
// rows so its exact per-run height reaches the budget.
func TestRowLimitVerifierAction_RuntimeRejectsOverLimitASide(t *testing.T) {
	g, rt := buildRuntimeLimitGroup(t, 1<<30, 2) // A reaches the budget; B tiny.
	action := rowLimitActionFor(g)

	err := action.Check(rt)
	require.Error(t, err, "verifier must reject an over-limit subgroup A side")
	require.ErrorContains(t, err, "A side")

	assert.Panics(t, func() { action.Run(rt) },
		"prover must panic on an over-limit subgroup A side")
}

// TestRowLimitVerifierAction_RuntimeRejectsOverLimitBSide is the B-side counterpart:
// the check fails independently on the shared lookup table.
func TestRowLimitVerifierAction_RuntimeRejectsOverLimitBSide(t *testing.T) {
	g, rt := buildRuntimeLimitGroup(t, 2, 1<<30) // B reaches the budget; A tiny.
	action := rowLimitActionFor(g)

	err := action.Check(rt)
	require.Error(t, err, "verifier must reject an over-limit subgroup B side")
	require.ErrorContains(t, err, "B side")

	assert.Panics(t, func() { action.Run(rt) },
		"prover must panic on an over-limit subgroup B side")
}

// TestRowLimitVerifierAction_RuntimeAcceptsWithinLimit confirms the runtime check is
// silent for an in-budget subgroup: Check returns nil and Run does not panic.
func TestRowLimitVerifierAction_RuntimeAcceptsWithinLimit(t *testing.T) {
	g, rt := buildRuntimeLimitGroup(t, 4, 4)
	action := rowLimitActionFor(g)

	require.NoError(t, action.Check(rt), "an in-budget subgroup must pass the verifier check")
	assert.NotPanics(t, func() { action.Run(rt) }, "an in-budget subgroup must not panic the prover")
}
