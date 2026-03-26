package smartvectors_mixed

import (
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// BatchEvaluateLagrange - Extended version supporting both base and extension fields
func BatchEvaluateLagrange(vs []sv.SmartVector, x fext.Element) []fext.Element {
	results := make([]fext.Element, len(vs))
	if len(vs) == 0 {
		return results
	}

	n := vs[0].Len()
	vLagrangeBasis := computeLagrangeBasisAtX(n, x)

	// Parallel processing - classification and polynomial extraction
	parallel.Execute(len(vs), func(start, stop int) {
		var scratchBase koalabear.Vector
		var scratchExt extensions.Vector

		for i := start; i < stop; i++ {
			// Check if it's base field or extension field
			if IsBase(vs[i]) {
				switch v := vs[i].(type) {
				case *sv.Constant:
					results[i] = v.GetExt(0)
					continue
				case *sv.Regular:
					// no need to copy here.
					results[i] = vLagrangeBasis.InnerProductByElement(koalabear.Vector(*v))
				default:
					if scratchBase == nil {
						scratchBase = make(koalabear.Vector, n)
					}
					v.WriteInSlice(scratchBase)
					results[i] = vLagrangeBasis.InnerProductByElement(koalabear.Vector(scratchBase))
				}

			} else {
				switch v := vs[i].(type) {
				case *sv.ConstantExt:
					results[i] = v.Value
					continue
				case *sv.RegularExt:
					// no need to copy here.
					results[i] = vLagrangeBasis.InnerProduct(extensions.Vector(*v))
				default:
					if scratchExt == nil {
						scratchExt = make(extensions.Vector, n)
					}
					v.WriteInSliceExt(scratchExt)
					results[i] = vLagrangeBasis.InnerProduct(scratchExt)
				}

			}
		}
	})

	return results
}

// TODO taken from gnark-crypto, tuned and adapted for our use case
// computeLagrangeBasisAtX computes (Lᵢ(x))_{i<n} and numerator for Lagrange basis evaluation
func computeLagrangeBasisAtX(n int, x fext.Element) extensions.Vector {

	// 1. Compute common factor: (x^n - 1) / n
	var commonFactor fext.Element
	one := koalabear.One()

	commonFactor.ExpInt64(x, int64(n))
	commonFactor.B0.A0.Sub(&commonFactor.B0.A0, &one)

	// Check for root of unity
	if commonFactor.IsZero() {
		// x is a root of unity.
		// Find k such that x = w^k.
		res := make(extensions.Vector, n)
		ch := make(chan int, 1)

		generator, _ := fft.Generator(uint64(n))
		var generatorInv koalabear.Element
		generatorInv.Inverse(&generator)

		parallel.Execute(n, func(start, stop int) {
			var wInvStart koalabear.Element
			wInvStart.ExpInt64(generatorInv, int64(start))
			curr := wInvStart

			for i := start; i < stop; i++ {
				// Check x * w^{-i} == 1
				var tmp fext.Element
				tmp.MulByElement(&x, &curr)
				tmp.B0.A0.Sub(&tmp.B0.A0, &one)
				if tmp.IsZero() {
					select {
					case ch <- i:
					default:
					}
					return
				}
				curr.Mul(&curr, &generatorInv)
			}
		})
		k := <-ch
		res[k].SetOne()
		return res
	}

	cardInv := koalabear.NewElement(uint64(n))
	cardInv.Inverse(&cardInv)
	commonFactor.MulByElement(&commonFactor, &cardInv)

	// 2. Prepare result vector and fill it in parallel
	// res[i] = x * w^{-i} - 1
	res := make(extensions.Vector, n)

	generator, _ := fft.Generator(uint64(n))
	var generatorInv koalabear.Element
	generatorInv.Inverse(&generator)

	parallel.Execute(n, func(start, stop int) {
		var wInvStart koalabear.Element
		wInvStart.ExpInt64(generatorInv, int64(start))

		currentWInv := wInvStart

		for i := start; i < stop; i++ {
			// res[i] = x * w^{-i}
			res[i].MulByElement(&x, &currentWInv)
			// res[i] = res[i] - 1
			res[i].B0.A0.Sub(&res[i].B0.A0, &one)

			currentWInv.Mul(&currentWInv, &generatorInv)
		}
	})

	// 3. Invert: res[i] = 1 / (x * w^{-i} - 1)
	res = fext.ParBatchInvert(res, 0)

	// 4. Multiply by commonFactor: res[i] *= (x^n - 1)/n
	parallel.Execute(n, func(start, stop int) {
		res := res[start:stop]
		res.ScalarMul(res, &commonFactor)
	})

	return res
}
