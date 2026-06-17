package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// newLagrangeSelector builds a single-round system with one sized, unpadded
// module of the given size and returns a LagrangeSelector at position pos
// together with a fresh runtime.
func newLagrangeSelector(t *testing.T, size, pos int) (*wiop.LagrangeSelector, wiop.Runtime) {
	t.Helper()
	sys := wiop.NewSystemf("ls")
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("lsMod"), size, wiop.PaddingDirectionNone)
	ls := wiop.NewLagrangeSelector(mod, pos)
	return ls, wiop.NewRuntime(sys)
}

// TestLagrangeSelector_EvaluateOutOfDomain checks that EvaluateOutOfDomain
// agrees with the barycentric Lagrange evaluation of the selector's own
// materialised vector, for both base-field and extension-field evaluation
// points, across every position of the domain.
func TestLagrangeSelector_EvaluateOutOfDomain(t *testing.T) {
	const size = 8

	// An out-of-domain base point (7 is not an 8-th root of unity) and a
	// random extension point.
	baseX := field.ElemFromBase(field.NewFromString("7"))
	extX := field.RandomElemExt()

	for pos := 0; pos < size; pos++ {
		ls, rt := newLagrangeSelector(t, size, pos)

		// Reference: materialise the selector vector (0,…,1,…,0) and evaluate
		// it in Lagrange basis at the same point.
		vec := ls.EvaluateVector(rt).Plain

		for name, x := range map[string]field.Gen{"base": baseX, "ext": extX} {
			want := polynomials.EvalLagrange(vec, x)
			got := ls.EvaluateOutOfDomain(rt, x)
			diff := got.Sub(want)
			require.Truef(t, diff.IsZero(),
				"pos=%d point=%s: EvaluateOutOfDomain disagrees with EvalLagrange", pos, name)
		}

		// The base result must stay in the base field when the point does.
		require.Truef(t, ls.EvaluateOutOfDomain(rt, baseX).IsBase(),
			"pos=%d: base evaluation point must yield a base result", pos)
	}
}

// TestLagrangeSelector_EvaluateOutOfDomain_ZeroAtOtherRows checks that the
// selector evaluates to 0 at every domain row other than its own (the
// numerator Xⁿ−1 vanishes there).
func TestLagrangeSelector_EvaluateOutOfDomain_ZeroAtOtherRows(t *testing.T) {
	const size = 8
	const pos = 3

	ls, rt := newLagrangeSelector(t, size, pos)
	omega := field.RootOfUnityBy(size)

	for j := 0; j < size; j++ {
		if j == pos {
			continue // denominator vanishes at the selector's own row
		}
		var omegaJ field.Element
		omegaJ.ExpInt64(omega, int64(j))
		got := ls.EvaluateOutOfDomain(rt, field.ElemFromBase(omegaJ))
		require.Truef(t, got.IsZero(), "selector at pos=%d must vanish at domain row %d", pos, j)
	}
}

// TestLagrangeSelector_EvaluateOutOfDomain_PanicsOnOwnRow checks that
// evaluating at ω^Position panics, since the denominator X−ω^Position is zero
// and an in-domain point violates the out-of-domain contract.
func TestLagrangeSelector_EvaluateOutOfDomain_PanicsOnOwnRow(t *testing.T) {
	const size = 8
	const pos = 5

	ls, rt := newLagrangeSelector(t, size, pos)
	var omegaPos field.Element
	omegaPos.ExpInt64(field.RootOfUnityBy(size), int64(pos))

	require.Panics(t, func() {
		ls.EvaluateOutOfDomain(rt, field.ElemFromBase(omegaPos))
	})
}

// TestLagrangeSelector_NegativePosition checks that a negative Position indexes
// from the end of the domain: −1 is the last row, −2 the second-to-last, etc.
// The materialised vector and the out-of-domain evaluation must both agree with
// the equivalent non-negative position size+pos.
func TestLagrangeSelector_NegativePosition(t *testing.T) {
	const size = 8
	baseX := field.ElemFromBase(field.NewFromString("7"))

	for neg := -1; neg >= -size; neg-- {
		absPos := size + neg // -1 -> 7, -8 -> 0

		lsNeg, rtNeg := newLagrangeSelector(t, size, neg)
		lsAbs, rtAbs := newLagrangeSelector(t, size, absPos)

		// The materialised vectors must be identical.
		vecNeg := lsNeg.EvaluateVector(rtNeg).Plain
		vecAbs := lsAbs.EvaluateVector(rtAbs).Plain
		require.Equalf(t, vecAbs.AsBase(), vecNeg.AsBase(),
			"neg=%d: materialised vector must match position %d", neg, absPos)

		// The single 1 must sit at the resolved row.
		require.Truef(t, vecNeg.AsBase()[absPos].IsOne(),
			"neg=%d: expected 1 at resolved row %d", neg, absPos)

		// Out-of-domain evaluation must match the non-negative form.
		gotNeg := lsNeg.EvaluateOutOfDomain(rtNeg, baseX)
		gotAbs := lsAbs.EvaluateOutOfDomain(rtAbs, baseX)
		diff := gotNeg.Sub(gotAbs)
		require.Truef(t, diff.IsZero(),
			"neg=%d: EvaluateOutOfDomain must match position %d", neg, absPos)
	}
}

// TestLagrangeSelector_OutOfRangePosition checks construction-time bounds on a
// statically sized module: positions in [−size, size) are accepted; anything
// outside that range panics.
func TestLagrangeSelector_OutOfRangePosition(t *testing.T) {
	const size = 8
	sys := wiop.NewSystemf("ls-bounds")
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)

	// Boundaries inside the range must not panic.
	require.NotPanics(t, func() { wiop.NewLagrangeSelector(mod, 0) })
	require.NotPanics(t, func() { wiop.NewLagrangeSelector(mod, size-1) })
	require.NotPanics(t, func() { wiop.NewLagrangeSelector(mod, -1) })
	require.NotPanics(t, func() { wiop.NewLagrangeSelector(mod, -size) })

	// Out-of-range on either side must panic.
	require.Panics(t, func() { wiop.NewLagrangeSelector(mod, size) })
	require.Panics(t, func() { wiop.NewLagrangeSelector(mod, -size-1) })
}

// TestLagrangeSelector_DynamicModule checks that a LagrangeSelector built on a
// dynamic-size module with a negative (end-relative) Position resolves against
// the per-Runtime size: the same selector is 1 at the last row whether the
// runtime size is 4 or 8, and its out-of-domain evaluation tracks the size.
func TestLagrangeSelector_DynamicModule(t *testing.T) {
	baseX := field.ElemFromBase(field.NewFromString("7"))

	sys := wiop.NewSystemf("ls-dyn")
	r0 := sys.NewRound()
	dyn := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)
	col := dyn.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)

	// Position −1: always the last row, regardless of runtime size.
	ls := wiop.NewLagrangeSelector(dyn, -1)

	for _, n := range []int{4, 8} {
		rt := wiop.NewRuntime(sys)
		// Assigning a column fixes the dynamic module's runtime size to n.
		rt.AssignColumn(col, makeVec(n, 1))

		vec := ls.EvaluateVector(rt).Plain
		require.Equalf(t, n, vec.Len(), "n=%d: vector length", n)
		for i := 0; i < n; i++ {
			want := i == n-1
			require.Equalf(t, want, vec.AsBase()[i].IsOne(),
				"n=%d row=%d: selector value", n, i)
		}

		// The out-of-domain evaluation must match an equivalent static
		// selector of size n at the last row (n−1).
		lsStatic, rtStatic := newLagrangeSelector(t, n, n-1)
		gotDyn := ls.EvaluateOutOfDomain(rt, baseX)
		gotStatic := lsStatic.EvaluateOutOfDomain(rtStatic, baseX)
		diff := gotDyn.Sub(gotStatic)
		require.Truef(t, diff.IsZero(),
			"n=%d: dynamic EvaluateOutOfDomain must match static size-%d selector", n, n)
	}
}

// TestLagrangeSelector_InExpression checks that a LagrangeSelector composes as
// an ordinary leaf inside an arithmetic expression and is evaluated correctly
// by the expression compiler on the runtime (in-domain) path. The product
// col · L_pos must equal col[pos] at row pos and 0 at every other row.
func TestLagrangeSelector_InExpression(t *testing.T) {
	const size = 8
	const pos = 5

	sys := wiop.NewSystemf("ls-expr")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	ls := wiop.NewLagrangeSelector(mod, pos)

	expr := wiop.Mul(col.View(), ls)

	rt := wiop.NewRuntime(sys)
	elems := make([]field.Element, size)
	for i := range elems {
		elems[i].SetUint64(uint64(i + 1)) // col[i] = i+1, so col[pos] = pos+1
	}
	rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(elems)})

	out := expr.EvaluateVector(rt).Plain
	require.Equal(t, size, out.Len())
	for i := 0; i < size; i++ {
		got := out.AsBase()[i]
		var want field.Element
		if i == pos {
			want.SetUint64(uint64(pos + 1))
		}
		require.Truef(t, got.Equal(&want), "row %d: got %v want %v", i, got, want)
	}
}

// TestLagrangeSelector_InVanishing checks that a LagrangeSelector works inside a
// Vanishing constraint, mirroring the shape produced by the localvanishing lift
// (predicate · L_pos). The constraint holds across the whole domain exactly when
// the pinned predicate holds at row pos.
func TestLagrangeSelector_InVanishing(t *testing.T) {
	const size = 8
	const pos = 3

	build := func(rowPosVal uint64) (*wiop.Vanishing, wiop.Runtime) {
		sys := wiop.NewSystemf("ls-vanish")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
		col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
		ls := wiop.NewLagrangeSelector(mod, pos)

		// Lifted local predicate "col == 0 at row pos": col · L_pos must vanish.
		v := mod.NewVanishingManual(sys.Context.Childf("v"), wiop.Mul(col.View(), ls))

		rt := wiop.NewRuntime(sys)
		elems := make([]field.Element, size)
		for i := range elems {
			elems[i].SetUint64(7) // non-zero everywhere by default
		}
		elems[pos].SetUint64(rowPosVal)
		rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(elems)})
		return v, rt
	}

	// Honest: col[pos] = 0 → the product vanishes on every row.
	v, rt := build(0)
	require.NoError(t, v.Check(rt))

	// Invalid: col[pos] = 5 → non-zero at row pos.
	v, rt = build(5)
	require.Error(t, v.Check(rt))
}
