package grandproduct_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeVecU64 builds a base-field ConcreteVector from a varargs list.
func makeVecU64(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// newSingleColumnPermutation builds a 1-round system with a single-column
// permutation between two size-4 modules. The grandproduct compiler later adds
// the β coin round and the GrandProduct result round itself.
func newSingleColumnPermutation(t *testing.T) (*wiop.System, *wiop.Column, *wiop.Column) {
	t.Helper()
	sys := wiop.NewSystemf("gp-single")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)
	return sys, colA, colB
}

// TestCompile_WioptestCompleteness drives every permutation scenario through
// the grandproduct pass alone: the prover assigns the grand-product Result and
// the Z columns, and the verifier actions (CheckResultIsOne + FinalProductCheck)
// must accept an honest permutation witness.
func TestCompile_WioptestCompleteness(t *testing.T) {
	for _, build := range wioptest.PermutationScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			grandproduct.Compile(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"compiled verifier must accept an honest permutation witness")
		})
	}
}

// TestCompile_WioptestSoundness drives every permutation scenario's invalid
// path: a non-permutation witness makes the grand product differ from one, so
// CheckResultIsOne (and FinalProductCheck) must reject it.
func TestCompile_WioptestSoundness(t *testing.T) {
	for _, build := range wioptest.PermutationScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			grandproduct.Compile(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignInvalid)
			assert.Error(t, sc.Sys.Verify(proof, pub),
				"compiled verifier must reject a non-permutation witness")
		})
	}
}

// TestCompile_NoQueries: a system without permutation queries compiles to a
// no-op and adds no GrandProduct, columns, or vanishings.
func TestCompile_NoQueries(t *testing.T) {
	sys := wiop.NewSystemf("gp-empty")
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	grandproduct.Compile(sys) // must not panic

	assert.Empty(t, sys.GrandProducts)
	assert.Empty(t, mod.Vanishings)
}

// TestCompile_ReducesPermutationAndAddsGrandProduct: the permutation query is
// marked reduced and exactly one aggregated GrandProduct (also reduced) is
// emitted, with one Z column + three vanishings per module.
func TestCompile_ReducesPermutationAndAddsGrandProduct(t *testing.T) {
	sys, _, _ := newSingleColumnPermutation(t)
	modA, modB := sys.Modules[0], sys.Modules[1]
	aColsBefore, aVansBefore := len(modA.Columns), len(modA.Vanishings)
	bColsBefore, bVansBefore := len(modB.Columns), len(modB.Vanishings)

	grandproduct.Compile(sys)

	assert.True(t, sys.TableRelations[0].IsReduced(),
		"the permutation query must be marked reduced")
	require.Len(t, sys.GrandProducts, 1, "exactly one aggregated GrandProduct")
	assert.True(t, sys.GrandProducts[0].IsReduced(),
		"the GrandProduct must be reduced by phase 2")

	// Each module owns one factor → one Z column, with recurrence + local-init
	// + endpoint-opening vanishings (the latter two scalar).
	assert.Len(t, modA.Columns, aColsBefore+1, "modA gets one Z column")
	assert.Len(t, modB.Columns, bColsBefore+1, "modB gets one Z column")
	assert.Len(t, modA.Vanishings, aVansBefore+3, "modA: recurrence + init + endpoint")
	assert.Len(t, modB.Vanishings, bVansBefore+3, "modB: recurrence + init + endpoint")
}

// TestCompile_Idempotent: a second Compile is a no-op once every query is
// reduced.
func TestCompile_Idempotent(t *testing.T) {
	sys, _, _ := newSingleColumnPermutation(t)
	grandproduct.Compile(sys)

	modA := sys.Modules[0]
	colsAfterFirst := len(modA.Columns)
	vansAfterFirst := len(modA.Vanishings)
	gpAfterFirst := len(sys.GrandProducts)

	grandproduct.Compile(sys)

	assert.Len(t, modA.Columns, colsAfterFirst, "no new Z columns on re-compile")
	assert.Len(t, modA.Vanishings, vansAfterFirst, "no new vanishings on re-compile")
	assert.Len(t, sys.GrandProducts, gpAfterFirst, "no new GrandProduct on re-compile")
}

// TestCompile_PacksFactors: four A-fragments sharing one module pack into
// ⌈4/3⌉ = 2 Z columns on that module.
func TestCompile_PacksFactors(t *testing.T) {
	sys := wiop.NewSystemf("gp-pack")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)

	aTables := make([]wiop.Table, 4)
	bTables := make([]wiop.Table, 4)
	for i := range aTables {
		a := modA.NewColumn(sys.Context.Childf("a%d", i), wiop.VisibilityOracle, r0)
		b := modB.NewColumn(sys.Context.Childf("b%d", i), wiop.VisibilityOracle, r0)
		aTables[i] = wiop.NewTable(a.View())
		bTables[i] = wiop.NewTable(b.View())
	}
	sys.NewPermutation(sys.Context.Childf("perm"), aTables, bTables)

	aColsBefore := len(modA.Columns)
	grandproduct.Compile(sys)

	assert.Len(t, modA.Columns, aColsBefore+2,
		"4 numerator factors must pack into ⌈4/3⌉ = 2 Z columns on modA")
}

// TestCompile_TamperResult: corrupting the Result cell before the prover
// assigns it makes both CheckResultIsOne and FinalProductCheck reject.
func TestCompile_TamperResult(t *testing.T) {
	sys, colA, colB := newSingleColumnPermutation(t)
	grandproduct.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVecU64(10, 20, 30, 40))
	rt.AssignColumn(colB, makeVecU64(30, 10, 40, 20))

	// Advance to the GrandProduct result round and pin Result to a wrong value
	// before the prover action runs (it skips an already-assigned cell).
	require.Len(t, sys.GrandProducts, 1)
	result := sys.GrandProducts[0].Result
	for rt.CurrentRound().ID < result.Round().ID {
		runRound(rt)
		rt.AdvanceRound()
	}
	rt.AssignCell(result, field.ElemFromBase(field.NewFromString("12345")))
	runRound(rt)

	assert.Error(t, checkAllVerifierActions(rt),
		"a tampered grand-product Result must be rejected")
}

// ---- Row limit (driven through the real prover/verifier) ----
//
// These tests exceed wiop.MaxPermutationRows (2^58) for real by declaring
// modules of that size. No 2^58 vector is materialised: a static module's size
// is metadata, the row-limit action reads only that size, and it is registered
// on the witness round — which the runtime opens first — ahead of the
// row-walking grand-product prover actions, so the prover fails fast.

// newPermutation builds a single-column permutation A ⊆⊇ B with the given
// module sizes and compiles it. Columns are left unassigned: the row-limit
// action reads only module sizes.
func newPermutation(t *testing.T, aSize, bSize int) *wiop.System {
	t.Helper()
	sys := wiop.NewSystemf("gp-limit")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), aSize, wiop.PaddingDirectionRight)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), bSize, wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)
	grandproduct.Compile(sys)
	return sys
}

// TestCompilePermutation_RowLimit_ProverPanics drives the real prover
// ([wiop.System.Prove]) of a permutation whose A side has 2^58 rows. The
// row-limit prover action — the only action on the witness round, registered
// ahead of the grand-product discharge pass — must panic before any row is
// touched, so the witness callback need not assign anything.
func TestCompilePermutation_RowLimit_ProverPanics(t *testing.T) {
	sys := newPermutation(t, 1<<58, 2) // A side = 2^58 rows (>= bound); B side tiny.
	assert.Panics(t, func() { sys.Prove(func(*wiop.Runtime) {}) },
		"prover must panic when a permutation side reaches the row limit")
}

// TestCompilePermutation_RowLimit_VerifierRejects runs the real verifier
// ([wiop.System.Verify]) against a maliciously inflated proof. An honest prover
// cannot produce a proof for a 2^58-row permutation (it would panic, and the
// row walk is infeasible anyway), so we prove an honest permutation over
// dynamic modules at a small size, then rewrite the B-side module's declared
// size in the proof to 2^58. The row-limit verifier action — the first verifier
// action, on the witness round — must reject before any other check runs.
func TestCompilePermutation_RowLimit_VerifierRejects(t *testing.T) {
	sys := wiop.NewSystemf("gp-limit-verify")
	r0 := sys.NewRound()
	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)
	grandproduct.Compile(sys)

	// Honest, small permutation witness: B is a reordering of A.
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, makeVecU64(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVecU64(30, 10, 40, 20))
	})
	require.NoError(t, sys.Verify(proof, pub), "sanity: the honest witness must verify")

	// Malicious inflation: claim the B module spans 2^58 rows. DynamicSizes is
	// keyed by the module's position in sys.Modules.
	bIdx := -1
	for i, m := range sys.Modules {
		if m == modB {
			bIdx = i
		}
	}
	require.GreaterOrEqual(t, bIdx, 0, "modB must be registered in the system")
	proof.DynamicSizes[bIdx] = 1 << 58

	err := sys.Verify(proof, pub)
	assert.ErrorContains(t, err, "effective per-query row limit",
		"verifier must reject a proof when a permutation side reaches the row limit")
}

// TestCompilePermutation_RowLimit_ManyPermsDoNotTightenBound builds several
// honest permutations and drives the whole system through the real prover and
// verifier. The row-by-row accumulators are per-module Z columns — not shared
// across permutations — so the effective per-query row limit is
// MaxPermutationRows/packingArity regardless of how many permutations exist.
// The row-limit verifier action fires naturally as part of Verify (it is one of
// the registered verifier actions) and, together with the grand-product result
// and final-product checks, must accept the honest multi-permutation witness.
//
// (The bound cannot be probed for real end-to-end: a module large enough to be
// discriminating would need >= 2^54 materialised rows. That the limit is NOT
// divided by the permutation count is pinned by the ProverPanics/VerifierRejects
// tests above, which fix it at MaxPermutationRows/packingArity.)
func TestCompilePermutation_RowLimit_ManyPermsDoNotTightenBound(t *testing.T) {
	sys := wiop.NewSystemf("gp-limit-many")
	r0 := sys.NewRound()

	const nPerms = 4
	aCols := make([]*wiop.Column, nPerms)
	bCols := make([]*wiop.Column, nPerms)
	for i := 0; i < nPerms; i++ {
		modA := sys.NewSizedModule(sys.Context.Childf("modA%d", i), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB%d", i), 4, wiop.PaddingDirectionNone)
		aCols[i] = modA.NewColumn(sys.Context.Childf("A%d", i), wiop.VisibilityOracle, r0)
		bCols[i] = modB.NewColumn(sys.Context.Childf("B%d", i), wiop.VisibilityOracle, r0)
		sys.NewPermutation(
			sys.Context.Childf("perm%d", i),
			[]wiop.Table{wiop.NewTable(aCols[i].View())},
			[]wiop.Table{wiop.NewTable(bCols[i].View())},
		)
	}
	grandproduct.Compile(sys)

	// B is a genuine reordering of A on every permutation, so each side is a
	// valid witness and all four are well under the per-query row limit.
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		for i := 0; i < nPerms; i++ {
			rt.AssignColumn(aCols[i], makeVecU64(10, 20, 30, 40))
			rt.AssignColumn(bCols[i], makeVecU64(30, 10, 40, 20))
		}
	})
	require.NoError(t, sys.Verify(proof, pub),
		"an honest multi-permutation witness under the row limit must be accepted regardless of how many permutations exist")
}

func runRound(rt *wiop.Runtime) {
	for _, a := range rt.CurrentRound().ProverActions {
		a.Run(rt)
	}
}

func checkAllVerifierActions(rt *wiop.Runtime) error {
	for _, r := range rt.System.Rounds {
		for _, va := range r.VerifierActions {
			if err := va.Check(rt); err != nil {
				return err
			}
		}
	}
	return nil
}
