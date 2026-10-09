package logderivativesum

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// The running sum must match a row-by-row evaluation of the fractions, and be
// independent of the number of workers: one
// worker processes the column as a single chunk, which is the serial
// algorithm, and any other split must reproduce it exactly -- including rows
// masked by a filter whose denominator is zero, and chunk counts that do not
// divide the column.
func TestComputeFilteredPrefixSum_IndependentOfWorkers(t *testing.T) {
	for _, n := range []int{1, 8, 1 << 12, 1 << 14} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(uint64(n), 7))
			sys := wiop.NewSystemf("prefix-sum")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)
			newCol := func(name string) *wiop.Column { return mod.NewColumn(sys.Context.Childf("%s", name), r0) }
			num1, den1, num2, den2, filter := newCol("n1"), newCol("d1"), newCol("n2"), newCol("d2"), newCol("f")
			ext := mod.NewExtensionColumn(sys.Context.Childf("e"), r0)

			rt := wiop.NewRuntime(sys)
			assign := func(c *wiop.Column, gen func(i int) uint64) {
				v := make([]field.Element, n)
				for i := range v {
					v[i].SetUint64(gen(i))
				}
				rt.AssignColumn(c, &wiop.ConcreteVector{Plain: field.VecFromBase(v)})
			}
			mask := make([]bool, n)
			for i := range mask {
				mask[i] = rng.IntN(4) != 0
			}
			assign(num1, func(int) uint64 { return rng.Uint64() })
			assign(den1, func(int) uint64 { return 1 + rng.Uint64N(1<<30) })
			assign(num2, func(int) uint64 { return rng.Uint64() })
			// Zero denominators only where the filter masks the row.
			assign(den2, func(i int) uint64 {
				if !mask[i] {
					return 0
				}
				return 1 + rng.Uint64N(1<<30)
			})
			assign(filter, func(i int) uint64 {
				if mask[i] {
					return 1
				}
				return 0
			})
			e := make([]field.Ext, n)
			for i := range e {
				e[i].B0.A0.SetUint64(rng.Uint64())
				e[i].B1.A1.SetUint64(rng.Uint64())
			}
			rt.AssignColumn(ext, &wiop.ConcreteVector{Plain: field.VecFromExt(e)})

			packed := []wiop.Fraction{
				{Numerator: num1.View(), Denominator: wiop.Add(den1.View(), ext.View())},
				{Numerator: num2.View(), Denominator: den2.View(), Filter: filter.View()},
				// Shifted views, an extension product and a subexpression
				// shared with the first fraction.
				{
					Numerator:   wiop.Mul(num2.View().Shift(-1), ext.View()),
					Denominator: wiop.Add(wiop.Mul(ext.View(), den1.View().Shift(2)), wiop.Add(den1.View(), ext.View())),
				},
			}
			serial := computeFilteredPrefixSum(rt, packed, n, 1)
			require.Equal(t, naiveFilteredPrefixSum(rt, packed, n), serial)
			for _, workers := range []int{2, 3, 4, 7, 192} {
				require.Equal(t, serial, computeFilteredPrefixSum(rt, packed, n, workers), "workers=%d", workers)
			}
		})
	}
}

// A zero denominator on a row the filter does not mask is malformed input,
// whichever chunk the row falls in.
func TestComputeFilteredPrefixSum_PanicsOnUnmaskedZeroDenominator(t *testing.T) {
	const n = 1 << 14
	sys := wiop.NewSystemf("prefix-sum-panic")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("n"), r0)
	den := mod.NewColumn(sys.Context.Childf("d"), r0)
	rt := wiop.NewRuntime(sys)
	ones := make([]field.Element, n)
	for i := range ones {
		ones[i].SetOne()
	}
	rt.AssignColumn(num, &wiop.ConcreteVector{Plain: field.VecFromBase(ones)})
	d := append([]field.Element(nil), ones...)
	d[n-3] = field.Element{}
	rt.AssignColumn(den, &wiop.ConcreteVector{Plain: field.VecFromBase(d)})

	packed := []wiop.Fraction{{Numerator: num.View(), Denominator: den.View()}}
	for _, workers := range []int{1, 4} {
		require.Panics(t, func() { computeFilteredPrefixSum(rt, packed, n, workers) }, "workers=%d", workers)
	}
}

// naiveFilteredPrefixSum evaluates every fraction's vectors in full and sums
// them row by row.
func naiveFilteredPrefixSum(rt *wiop.Runtime, packed []wiop.Fraction, n int) []field.Ext {
	z := make([]field.Ext, n)
	var running field.Ext
	nums, dens, filters := make([][]field.Ext, len(packed)), make([][]field.Ext, len(packed)), make([][]field.Ext, len(packed))
	for j, p := range packed {
		nums[j] = wiop.EvaluateAsExtVec(rt, p.Numerator, n)
		dens[j] = wiop.EvaluateAsExtVec(rt, p.Denominator, n)
		if p.Filter != nil {
			filters[j] = wiop.EvaluateAsExtVec(rt, p.Filter, n)
		}
	}
	for i := range n {
		for j := range packed {
			if filters[j] != nil && filters[j][i].IsZero() {
				continue
			}
			var term field.Ext
			term.Inverse(&dens[j][i])
			term.Mul(&term, &nums[j][i])
			if filters[j] != nil {
				term.Mul(&term, &filters[j][i])
			}
			running.Add(&running, &term)
		}
		z[i] = running
	}
	return z
}

// Columns of padded modules are read in place, padding included, and the
// running sum must still match a row-by-row evaluation.
func TestComputeFilteredPrefixSum_PaddedModules(t *testing.T) {
	const n = 1 << 12
	for _, pd := range []wiop.PaddingDirection{wiop.PaddingDirectionLeft, wiop.PaddingDirectionRight} {
		t.Run(fmt.Sprintf("pd=%v", pd), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(uint64(pd), 9))
			sys := wiop.NewSystemf("prefix-sum-padded")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, pd)
			num := mod.NewColumn(sys.Context.Childf("n"), r0)
			den := mod.NewColumn(sys.Context.Childf("d"), r0)
			ext := mod.NewExtensionColumn(sys.Context.Childf("e"), r0)
			rt := wiop.NewRuntime(sys)
			short := n - 1000
			d := field.VecPseudoRandBase(rng, short)
			for i := range d {
				d[i].SetUint64(1 + rng.Uint64N(1<<30))
			}
			var padding field.Element
			padding.SetUint64(7)
			rt.AssignColumn(num, &wiop.ConcreteVector{Plain: field.VecFromBase(field.VecPseudoRandBase(rng, short)), Padding: padding})
			rt.AssignColumn(den, &wiop.ConcreteVector{Plain: field.VecFromBase(d), Padding: padding})
			rt.AssignColumn(ext, &wiop.ConcreteVector{Plain: field.VecFromExt(field.VecPseudoRandExt(rng, short)), Padding: padding})

			packed := []wiop.Fraction{
				{Numerator: num.View().Shift(5), Denominator: wiop.Add(den.View(), ext.View().Shift(-2))},
				{Numerator: ext.View(), Denominator: den.View().Shift(1)},
			}
			require.Equal(t, naiveFilteredPrefixSum(rt, packed, n), computeFilteredPrefixSum(rt, packed, n, 4))
		})
	}
}
