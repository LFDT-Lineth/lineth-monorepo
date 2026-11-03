package global_test

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
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
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub),
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
			// The PCS step is needed otherwise, the change in the transcript
			// does not impact the FS state and the transcript alteration attack
			// is not detectable.
			pcs.Compile(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignInvalid)
			assert.Error(t, sc.Sys.Verify(proof, pub),
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
			col := mod.NewColumn(sys.Context.Childf("col"), r0)
			ls := wiop.NewLagrangeSelector(mod, pos)
			// col · L_pos == 0 on every row  ⇔  col[pos] == 0.
			mod.NewVanishingManual(sys.Context.Childf("v"), wiop.Mul(col.View(), ls))
			global.Compile(sys)
			return sys, col
		}

		sys, col := build()
		proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, colVec(size, pos, 0)) })
		require.NoError(t, sys.Verify(proof, pub), "honest: col[pos]=0 must verify")

		sys, col = build()
		proof, pub = sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, colVec(size, pos, 5)) })
		assert.Error(t, sys.Verify(proof, pub), "invalid: col[pos]=5 must be rejected")
	})

	t.Run("quadratic", func(t *testing.T) {
		// (a · b) · L_pos: degree factor 3 → ratio > 1, so the selector is
		// evaluated on the large coset (N = n·ratio).
		build := func() (*wiop.System, *wiop.Column, *wiop.Column) {
			sys := wiop.NewSystemf("gl-ls-quad")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
			a := mod.NewColumn(sys.Context.Childf("a"), r0)
			b := mod.NewColumn(sys.Context.Childf("b"), r0)
			ls := wiop.NewLagrangeSelector(mod, pos)
			mod.NewVanishingManual(sys.Context.Childf("v"), wiop.Mul(wiop.Mul(a.View(), b.View()), ls))
			global.Compile(sys)
			return sys, a, b
		}

		sys, a, b := build()
		proof, pub := sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(a, colVec(size, pos, 0)) // a[pos]=0 → product 0 at pos
			rt.AssignColumn(b, colVec(size, pos, 5))
		})
		require.NoError(t, sys.Verify(proof, pub), "honest: (a·b)[pos]=0 must verify")

		sys, a, b = build()
		proof, pub = sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(a, colVec(size, pos, 3)) // a[pos]·b[pos]=15 ≠ 0
			rt.AssignColumn(b, colVec(size, pos, 5))
		})
		assert.Error(t, sys.Verify(proof, pub), "invalid: (a·b)[pos]=15 must be rejected")
	})
}

// TestCompile_CompletenessWithStalePools reruns the completeness scenarios,
// with the PCS, after filling the field pools with junk: the quotient's coset
// tables and the commitments take their buffers from the pools, which hand
// out stale values, and must not read any before writing it. Every scenario
// is proven twice, so the second proof also reuses the first one's buffers.
func TestCompile_CompletenessWithStalePools(t *testing.T) {
	var junk field.Element
	junk.SetUint64(0xbad)
	for l := range 14 {
		for range 16 {
			b, e := field.BasePool.Get(1<<l), field.ExtPool.Get(1<<l)
			o := field.OctupletPool.Get(1 << l)
			for i := range b {
				b[i], e[i] = junk, field.Lift(junk)
				o[i] = field.Octuplet{junk, junk, junk, junk, junk, junk, junk, junk}
			}
			field.BasePool.Put(b)
			field.ExtPool.Put(e)
			field.OctupletPool.Put(o)
		}
	}
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			global.Compile(sc.Sys)
			pcs.Compile(sc.Sys)
			for range 2 {
				proof, pub := sc.Sys.Prove(sc.AssignHonest)
				require.NoError(t, sc.Sys.Verify(proof, pub))
			}
		})
	}
}

// TestCompile_AllVanishingsReduced checks that a module whose vanishings were
// all reduced by an earlier pass compiles, proves and verifies, its quotient
// having no bucket. Both a static and a dynamic module are covered, the
// dynamic one exercising the runtime path.
func TestCompile_AllVanishingsReduced(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		sys := wiop.NewSystemf("gl-all-reduced")
		r0 := sys.NewRound()
		var mod *wiop.Module
		if dynamic {
			mod = sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
		} else {
			mod = sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
		}
		col := mod.NewColumn(sys.Context.Childf("col"), r0)
		v := mod.NewVanishing(sys.Context.Childf("v"), col.View())
		v.MarkAsReduced()

		require.NotPanics(t, func() { global.Compile(sys) })
		proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, colVec(8, 3, 5)) })
		require.NoError(t, sys.Verify(proof, pub), "dynamic=%v", dynamic)
	}
}

// TestCompile_MixedRatiosShareColumns runs one module whose vanishings fall in
// buckets of ratio 1, 2 and 4, all reading the same column at shifts -1, 0
// and +1: every bucket reads its own subset of the module's cosets of that
// column, at offsets that wrap within a coset. The witness col[i] = i is not
// constant, so every bucket's quotient is nonzero and depends on reading the
// right coset at the right shift; the test checks that before checking that
// the argument accepts the witness and rejects one wrong row.
func TestCompile_MixedRatiosShareColumns(t *testing.T) {
	const size = 16
	build := func() (*wiop.System, *wiop.Column) {
		sys := wiop.NewSystemf("gl-mixed-ratios")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
		col := mod.NewColumn(sys.Context.Childf("col"), r0)
		c := func(shift int) wiop.Expression { return col.View().Shift(shift) }
		one := wiop.NewConstantField(field.One())
		// step(s) = col[i+s+1] - col[i+s] - 1, zero on every row where the
		// shift does not wrap.
		step := func(s int) wiop.Expression { return wiop.Sub(wiop.Sub(c(s+1), c(s)), one) }
		sq := wiop.Mul(c(0), c(0))
		// Degree 1 with one cancelled row: ratio 1.
		mod.NewVanishing(sys.Context.Childf("r1"), step(-1))
		// Degree 3 with one cancelled row: ratio 2.
		mod.NewVanishing(sys.Context.Childf("r2"), wiop.Mul(step(-1), sq))
		// Degree 5 with one cancelled row: ratio 4.
		mod.NewVanishing(sys.Context.Childf("r4"), wiop.Mul(step(0), wiop.Mul(sq, sq)))
		global.Compile(sys)
		pcs.Compile(sys)
		return sys, col
	}
	witness := func(bad bool) *wiop.ConcreteVector {
		v := colVec(size, 0, 0)
		elems := v.Plain.AsBase()
		for i := range elems {
			elems[i].SetUint64(uint64(i))
		}
		if bad {
			elems[5].SetUint64(99)
		}
		return v
	}

	sys, col := build()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, witness(false)) })

	// The claimed evaluations of the quotient shares, by bucket ratio: a
	// bucket whose shares all evaluate to zero has a zero quotient.
	nonZero := map[int]bool{}
	for _, r := range sys.Rounds {
		for _, cell := range r.Cells {
			var ratio, share int
			if _, err := fmt.Sscanf(cell.Context.Label, "q-claim-r%d-s%d", &ratio, &share); err != nil {
				continue
			}
			v := proof.Cells[cell.Context.ID].AsExt()
			nonZero[ratio] = nonZero[ratio] || !v.IsZero()
		}
	}
	require.Equal(t, map[int]bool{1: true, 2: true, 4: true}, nonZero,
		"each ratio bucket must exist and have a nonzero quotient")
	require.NoError(t, sys.Verify(proof, pub), "honest witness must verify")

	sys, col = build()
	proof, pub = sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, witness(true)) })
	assert.Error(t, sys.Verify(proof, pub), "a wrong row must be rejected")
}

// TestCompile_SharedSubexpressionsAreLinear runs the quotient argument on a
// vanishing whose expression is a depth-64 DAG of shared subexpressions:
// x_{i+1} = x_i + x_i over x_0 = col · L_pos, a tree of 2^64 leaves but only
// 65 distinct nodes. Compilation, proving and verification must all walk it
// in time linear in the distinct nodes, and the argument must still accept
// exactly the witnesses with col[pos] == 0.
func TestCompile_SharedSubexpressionsAreLinear(t *testing.T) {
	const size = 8
	const pos = 3

	build := func() (*wiop.System, *wiop.Column) {
		sys := wiop.NewSystemf("gl-shared-dag")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
		col := mod.NewColumn(sys.Context.Childf("col"), r0)
		x := wiop.Mul(col.View(), wiop.NewLagrangeSelector(mod, pos))
		for range 64 {
			x = wiop.Add(x, x)
		}
		mod.NewVanishingManual(sys.Context.Childf("v"), x)
		global.Compile(sys)
		return sys, col
	}

	sys, col := build()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, colVec(size, pos, 0)) })
	require.NoError(t, sys.Verify(proof, pub), "honest: col[pos]=0 must verify")

	sys, col = build()
	proof, pub = sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, colVec(size, pos, 5)) })
	assert.Error(t, sys.Verify(proof, pub), "invalid: col[pos]=5 must be rejected")
}
