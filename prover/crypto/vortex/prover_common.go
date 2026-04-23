package vortex

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// OpeningProof represents an opening proof for a Vortex commitment
type OpeningProof struct {

	// Columns [i][j][k] returns the k-th entry
	// of the j-th selected column of the i-th commitment
	Columns [][][]field.Element

	// Linear combination of the Reed-Solomon encoded polynomials to open.
	LinearCombination smartvectors.SmartVector
}

// Let x := randomCoin
// computes ∑ᵢxⁱ * v[i]
// n is the size of each vector v[i]
//
// TODO @thomaspiellard why not use directly smarvectorext.LinComb ??
func LinearCombination(proof *OpeningProof, v []smartvectors.SmartVector, randomCoin fext.Element) {

	if len(v) == 0 {
		utils.Panic("attempted to open an empty witness")
	}

	n := v[0].Len()
	linComb := make([]fext.Element, n)
	parallel.Execute(len(linComb), func(start, stop int) {

		x := fext.One()
		var scratch vectorext.Vector // lazy-allocated only if needed (default case)
		// Accumulate directly into the shared linComb slice — each goroutine
		// owns a disjoint [start:stop) range, so no data race and no copy needed.
		localLinComb := vectorext.Vector(linComb[start:stop])
		for i := range v {
			_sv := v[i]
			// we distinguish the case of a regular vector and constant to avoid
			// unnecessary allocations and copies
			switch _svt := _sv.(type) {
			case *smartvectors.Constant:
				cst := _svt.GetExt(0)
				cst.Mul(&cst, &x)
				for j := range localLinComb {
					localLinComb[j].Add(&localLinComb[j], &cst)
				}
				x.Mul(&x, &randomCoin)
				continue
			case *smartvectors.Regular:
				// MulAccByElement: fused E4×base multiply-accumulate (4 base muls)
				// instead of SetFromBase + ScalarMul (E4×E4, 9 base muls via Karatsuba)
				sv := field.Vector((*_svt)[start:stop])
				localLinComb.MulAccByElement(sv, &x)
				x.Mul(&x, &randomCoin)
				continue
			default:
				if scratch == nil {
					scratch = make(vectorext.Vector, stop-start)
				}
				sv := _svt.SubVector(start, stop)
				sv.WriteInSliceExt(scratch)
			}
			scratch.ScalarMul(scratch, &x)
			localLinComb.Add(localLinComb, scratch)
			x.Mul(&x, &randomCoin)

		}
	})

	proof.LinearCombination = smartvectors.NewRegularExt(linComb)
}
