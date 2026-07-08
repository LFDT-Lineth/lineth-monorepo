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
