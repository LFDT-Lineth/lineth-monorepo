package vortex

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
	"github.com/consensys/linea-monorepo/prover-ray/utils/parallel"
)

// OpeningProof represents an opening proof for a Vortex commitment
type OpeningProof struct {

	// Columns [i][j][k] returns the k-th entry
	// of the j-th selected column of the i-th commitment
	Columns [][][]field.Element

	// Linear combination of the Reed-Solomon encoded polynomials to open.
	LinearCombination []field.Ext
}

// LinearCombination computes the linear combination of the vectors v[i]
// Let x := randomCoin
// computes ∑ᵢxⁱ * v[i]
// n is the size of each vector v[i]
func LinearCombination(proof *OpeningProof, v [][]field.Element, randomCoin field.Ext) []field.Ext {

	if len(v) == 0 {
		utils.Panic("attempted to open an empty witness")
	}

	nColumns := len(v[0])
	linComb := make([]field.Ext, nColumns)
	parallel.Execute(nColumns, func(start, stop int) {

		var (
			x            = field.OneExt()
			localLinComb = make([]field.Ext, stop-start)
			tmp          field.Ext
		)

		for i := range v {
			// we distinguish the case of a regular vector and constant to avoid
			// unnecessary allocations and copies
			for j := range localLinComb {
				tmp.MulByElement(&x, &v[i][start+j])
				localLinComb[j].Add(&localLinComb[j], &tmp)
			}

			x.Mul(&x, &randomCoin)
		}

		copy(linComb[start:stop], localLinComb)
	})

	proof.LinearCombination = linComb

	return linComb
}
