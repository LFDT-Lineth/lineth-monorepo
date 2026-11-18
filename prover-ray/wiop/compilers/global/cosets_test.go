package global

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/stretchr/testify/require"
)

// serialCancellationCoset is the reference evaluation of the cancellation
// polynomial on the coset: a single pass stepping x by ω_N.
func serialCancellationCoset(cancelled []int, n, N int) []field.Element {
	omega := field.RootOfUnityBy(n)
	roots := make([]field.Element, len(cancelled))
	for i, pos := range cancelled {
		k := pos
		if k < 0 {
			k = n + pos
		}
		field.ExpToInt(&roots[i], omega, k)
	}
	omegaN := field.RootOfUnityBy(N)
	var x field.Element
	x.SetUint64(field.MultiplicativeGen)
	out := make([]field.Element, N)
	for j := range N {
		out[j].SetOne()
		for _, root := range roots {
			var diff field.Element
			diff.Sub(&x, &root)
			out[j].Mul(&out[j], &diff)
		}
		x.Mul(&x, &omegaN)
	}
	return out
}

// serialLagrangeSelectorCoset is the reference evaluation of L_p on the coset,
// one point at a time with a field inversion per point.
func serialLagrangeSelectorCoset(position, n, N int) []field.Element {
	p := ((position % n) + n) % n
	var omegaP, nInv, numCoef, x, one field.Element
	field.ExpToInt(&omegaP, field.RootOfUnityBy(n), p)
	nInv.SetUint64(uint64(n))
	nInv.Inverse(&nInv)
	numCoef.Mul(&omegaP, &nInv)
	one.SetOne()
	omegaN := field.RootOfUnityBy(N)
	x.SetUint64(field.MultiplicativeGen)
	out := make([]field.Element, N)
	for j := range N {
		var xn, num, den field.Element
		field.ExpToInt(&xn, x, n)
		num.Sub(&xn, &one)
		den.Sub(&x, &omegaP)
		den.Inverse(&den)
		out[j].Mul(&numCoef, &num)
		out[j].Mul(&out[j], &den)
		x.Mul(&x, &omegaN)
	}
	return out
}

// toCosetMajor reorders a large-coset vector from the natural order (index j,
// point g·ω_N^j) into the coset-major layout of [cosetShifts] (index k·n + i,
// point g·ω_N^(i·ratio+k)).
func toCosetMajor(v []field.Element, n int) []field.Element {
	ratio := len(v) / n
	out := make([]field.Element, len(v))
	for k := range ratio {
		for i := range n {
			out[k*n+i] = v[i*ratio+k]
		}
	}
	return out
}

// The parallel coset evaluations split the points into chunks that each start
// from their own point; they must agree with a single serial pass over the
// large coset, reordered coset-major, for domains small enough to give one
// point per worker and large enough to give many points per worker.
func TestCosetsMatchSerialReference(t *testing.T) {
	for _, sz := range []struct{ n, ratio int }{{1, 1}, {8, 4}, {64, 2}, {1024, 4}, {1 << 14, 2}} {
		n, N := sz.n, sz.n*sz.ratio
		t.Run(fmt.Sprintf("n=%d/N=%d", n, N), func(t *testing.T) {
			for _, cancelled := range [][]int{{0}, {-1}, {0, -1}, {1, 2, -2}} {
				require.Equal(t, toCosetMajor(serialCancellationCoset(cancelled, n, N), n), computeCancellationCoset(cancelled, n, N),
					"cancelled %v", cancelled)
			}
			for _, pos := range []int{0, 1, -1, n - 1, 3*n + 2} {
				require.Equal(t, toCosetMajor(serialLagrangeSelectorCoset(pos, n, N), n), computeLagrangeSelectorCoset(pos, n, N),
					"position %d", pos)
			}
		})
	}
}

// evalColumnsOnCosets must evaluate each padded column at every point of the
// cosets it is needed on, in the coset-major layout, checked against a
// barycentric evaluation per point.
func TestEvalColumnsOnCosets(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for _, n := range []int{4, 64} {
		for _, ratio := range []int{1, 2, 4} {
			for _, pd := range []wiop.PaddingDirection{wiop.PaddingDirectionNone, wiop.PaddingDirectionLeft, wiop.PaddingDirectionRight} {
				t.Run(fmt.Sprintf("n=%d/ratio=%d/pd=%v", n, ratio, pd), func(t *testing.T) {
					plainLen := n
					var pad field.Element
					if pd != wiop.PaddingDirectionNone {
						plainLen = n - 3
						pad = field.VecPseudoRandBase(rng, 1)[0]
					}
					sys := wiop.NewSystemf("cosets")
					r0 := sys.NewRound()
					mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, pd)
					base := mod.NewColumn(sys.Context.Childf("base"), r0)
					ext := mod.NewExtensionColumn(sys.Context.Childf("ext"), r0)
					baseCV := &wiop.ConcreteVector{Plain: field.VecFromBase(field.VecPseudoRandBase(rng, plainLen)), Padding: pad}
					extCV := &wiop.ConcreteVector{Plain: field.VecFromExt(field.VecPseudoRandExt(rng, plainLen)), Padding: pad}
					rt := wiop.NewRuntime(sys)
					rt.AssignColumn(base, baseCV)
					rt.AssignColumn(ext, extCV)

					// The extension column is only read by a bucket of half the
					// ratio: only every other coset is evaluated for it.
					extRatio := max(1, ratio/2)
					gotBase, gotExt := evalColumnsOnCosets(rt, mod, []*wiop.Column{base, ext}, []int{ratio, extRatio},
						fft.NewDomain(uint64(n)), newCosetDomains(n, ratio))

					paddedBase, paddedExt := make([]field.Element, n), make([]field.Ext, n)
					for i := range n {
						paddedBase[i] = baseCV.ElementAtN(pd, n, i).AsBase()
						paddedExt[i] = extCV.ElementAtN(pd, n, i).AsExt()
					}
					shifts := cosetShifts(n, ratio)
					pt := newCosetWalk(shifts, n, 0)
					for tt := range n * ratio {
						x := field.ElemFromBase(pt.x)
						wantBase := polynomials.EvalLagrange(field.VecFromBase(paddedBase), x)
						wantExt := polynomials.EvalLagrange(field.VecFromExt(paddedExt), x)
						require.Equal(t, wantBase.AsBase(), gotBase[base.Context.ID][tt], "base, point %d", tt)
						if (tt/n)%(ratio/extRatio) == 0 {
							require.Equal(t, wantExt.AsExt(), gotExt[ext.Context.ID][tt], "ext, point %d", tt)
						}
						pt.next()
					}
				})
			}
		}
	}
}

// cosetsToShares must recover the shares of a polynomial of degree < n·ratio
// from its evaluations on the coset-major layout: share m in Lagrange form
// holds the m-th chunk of n coefficients, checked by naive evaluation.
func TestCosetsToShares(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	const n = 8
	for _, ratio := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("ratio=%d", ratio), func(t *testing.T) {
			N := n * ratio
			coeffs := field.VecPseudoRandExt(rng, N)
			agg := make([]field.Ext, N)
			pt := newCosetWalk(cosetShifts(n, ratio), n, 0)
			for tt := range N {
				agg[tt] = hornerExt(coeffs, pt.x)
				pt.next()
			}

			shares := cosetsToShares(agg, fft.NewDomain(n), newCosetDomains(n, ratio))

			omega := field.RootOfUnityBy(n)
			for m := range ratio {
				var x field.Element
				x.SetOne()
				for i := range n {
					require.Equal(t, hornerExt(coeffs[m*n:(m+1)*n], x), shares[m][i], "share %d, row %d", m, i)
					x.Mul(&x, &omega)
				}
			}
		})
	}
}

// hornerExt evaluates Σ c_i·x^i.
func hornerExt(c []field.Ext, x field.Element) field.Ext {
	var acc field.Ext
	for i := len(c) - 1; i >= 0; i-- {
		acc.MulByElement(&acc, &x)
		acc.Add(&acc, &c[i])
	}
	return acc
}
