package grandproduct

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// The chunked running product must match a row-by-row product of the
// evaluated factors, for any number of workers, including chunk counts that
// do not divide the column.
func TestComputePrefixProduct_MatchesSerial(t *testing.T) {
	for _, n := range []int{1, 8, 3*prefixProductMinChunk + 5} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(uint64(n), 3))
			sys := wiop.NewSystemf("prefix-product")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)
			a := mod.NewColumn(sys.Context.Childf("a"), r0)
			d := mod.NewColumn(sys.Context.Childf("d"), r0)
			e := mod.NewExtensionColumn(sys.Context.Childf("e"), r0)
			rt := wiop.NewRuntime(sys)
			rt.AssignColumn(a, &wiop.ConcreteVector{Plain: field.VecFromBase(field.VecPseudoRandBase(rng, n))})
			dv := field.VecPseudoRandBase(rng, n)
			for i := range dv {
				if dv[i].IsZero() {
					dv[i].SetOne()
				}
			}
			rt.AssignColumn(d, &wiop.ConcreteVector{Plain: field.VecFromBase(dv)})
			rt.AssignColumn(e, &wiop.ConcreteVector{Plain: field.VecFromExt(field.VecPseudoRandExt(rng, n))})

			zNum := wiop.Mul(a.View().Shift(1), e.View())
			zDen := wiop.Add(d.View(), wiop.Mul(e.View(), e.View().Shift(-1)))
			num := wiop.EvaluateAsExtVec(rt, zNum, n)
			den := wiop.EvaluateAsExtVec(rt, zDen, n)
			want := make([]field.Ext, n)
			var running field.Ext
			running.SetOne()
			for i := range n {
				var term field.Ext
				term.Inverse(&den[i])
				term.Mul(&term, &num[i])
				running.Mul(&running, &term)
				want[i] = running
			}
			for _, workers := range []int{1, 2, 3, 7, 192} {
				require.Equal(t, want, computePrefixProduct(rt, zNum, zDen, n, workers), "workers=%d", workers)
			}
		})
	}
}

// A zero denominator is malformed input, whichever chunk the row falls in.
func TestComputePrefixProduct_PanicsOnZeroDenominator(t *testing.T) {
	const n = 3 * prefixProductMinChunk
	sys := wiop.NewSystemf("prefix-product-panic")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("n"), r0)
	den := mod.NewColumn(sys.Context.Childf("d"), r0)
	rt := wiop.NewRuntime(sys)
	ones := field.VecRepeatBase(field.One(), n)
	rt.AssignColumn(num, &wiop.ConcreteVector{Plain: field.VecFromBase(ones)})
	d := field.VecRepeatBase(field.One(), n)
	d[n-3] = field.Element{}
	rt.AssignColumn(den, &wiop.ConcreteVector{Plain: field.VecFromBase(d)})
	for _, workers := range []int{1, 4} {
		require.Panics(t, func() { computePrefixProduct(rt, num.View(), den.View(), n, workers) }, "workers=%d", workers)
	}
}
