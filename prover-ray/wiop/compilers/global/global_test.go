package global_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/global"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompile_Completeness verifies that for every vanishing scenario, an
// honest witness satisfies all verifier actions that Compile registers.
func TestCompile_Completeness(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			global.Compile(sc.Sys)
			proof := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof),
				"compiled verifier must accept an honest witness")
		})
	}
}

// TestCompile_Soundness verifies that for every vanishing scenario, an invalid
// witness (one that violates at least one constraint) is rejected by the
// compiled verifier. This is the core soundness property of the compiler: a
// cheating prover cannot produce a quotient that satisfies the identity check.
func TestCompile_Soundness(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			global.Compile(sc.Sys)
			proof := sc.Sys.Prove(sc.AssignInvalid)
			assert.Error(t, sc.Sys.Verify(proof),
				"compiled verifier must reject an invalid witness")
		})
	}
}

// colVec builds a length-n base-field ConcreteVector filled with 9 everywhere
// except valAtPos at row pos.
func colVec(n, pos int, valAtPos uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i].SetUint64(9)
	}
	elems[pos].SetUint64(valAtPos)
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// TestCompile_GlobalConstraintWithLagrangeSelector exercises the global compiler
// directly on a multi-valued Vanishing whose expression contains a
// LagrangeSelector leaf — not one produced by the localvanishing lift, but
// hand-written. This isolates the global layer's selector handling: analytic
// coset evaluation on the prover side and EvaluateOutOfDomain at the verifier's
// point.
//
// The constraint expr · L_pos must vanish on every row, which holds exactly when
// expr is zero at row pos. Both the linear case (ratio == 1) and the quadratic
// case (ratio > 1, so the selector is evaluated on the large coset) are covered.
func TestCompile_GlobalConstraintWithLagrangeSelector(t *testing.T) {
	const size = 8
	const pos = 3

	t.Run("linear", func(t *testing.T) {
		build := func() (*wiop.System, *wiop.Column) {
			sys := wiop.NewSystemf("gl-ls-lin")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
			col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
			ls := wiop.NewLagrangeSelector(mod, pos)
			// col · L_pos == 0 on every row  ⇔  col[pos] == 0.
			mod.NewVanishingManual(sys.Context.Childf("v"), wiop.Mul(col.View(), ls))
			global.Compile(sys)
			return sys, col
		}

		sys, col := build()
		proof := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, colVec(size, pos, 0)) })
		require.NoError(t, sys.Verify(proof), "honest: col[pos]=0 must verify")

		sys, col = build()
		proof = sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, colVec(size, pos, 5)) })
		assert.Error(t, sys.Verify(proof), "invalid: col[pos]=5 must be rejected")
	})

	t.Run("quadratic", func(t *testing.T) {
		// (a · b) · L_pos: degree factor 3 → ratio > 1, so the selector is
		// evaluated on the large coset (N = n·ratio).
		build := func() (*wiop.System, *wiop.Column, *wiop.Column) {
			sys := wiop.NewSystemf("gl-ls-quad")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
			a := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
			b := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
			ls := wiop.NewLagrangeSelector(mod, pos)
			mod.NewVanishingManual(sys.Context.Childf("v"), wiop.Mul(wiop.Mul(a.View(), b.View()), ls))
			global.Compile(sys)
			return sys, a, b
		}

		sys, a, b := build()
		proof := sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(a, colVec(size, pos, 0)) // a[pos]=0 → product 0 at pos
			rt.AssignColumn(b, colVec(size, pos, 5))
		})
		require.NoError(t, sys.Verify(proof), "honest: (a·b)[pos]=0 must verify")

		sys, a, b = build()
		proof = sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(a, colVec(size, pos, 3)) // a[pos]·b[pos]=15 ≠ 0
			rt.AssignColumn(b, colVec(size, pos, 5))
		})
		assert.Error(t, sys.Verify(proof), "invalid: (a·b)[pos]=15 must be rejected")
	})
}
