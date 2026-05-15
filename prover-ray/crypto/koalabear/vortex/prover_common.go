package vortex

import (
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
	"github.com/consensys/linea-monorepo/prover-ray/utils/parallel"
)

// OpeningProof represents an opening proof for a Vortex commitment
type OpeningProof struct {

	// Columns [i][j][k] returns the k-th entry
	// of the j-th selected column of the i-th commitment
	Columns [][][]field.Element

	// Linear combination of the Reed-Solomon encoded polynomials to open,
	// in coefficient (monomial) form.
	LinearCombination []field.Ext
}

// LinearCombination computes ∑ᵢ randomCoinⁱ · v[i] in evaluation form, then
// converts the result to coefficient form via IFFT over d and stores it in
// proof.LinearCombination. Returning the coefficient form directly keeps
// proof.LinearCombination in a single, well-defined representation.
func LinearCombination(proof *OpeningProof, d *fft.Domain, v [][]field.Element, randomCoin field.Ext) []field.Ext {

	if len(v) == 0 {
		utils.Panic("attempted to open an empty witness")
	}

	nColumns := len(v[0])
	linComb := make([]field.Ext, nColumns)
	parallel.Execute(nColumns, func(start, stop int) {

		var (
			x   = field.OneExt()
			tmp field.Ext
		)

		out := linComb[start:stop]
		for i := range v {
			for j := range out {
				tmp.MulByElement(&x, &v[i][start+j])
				out[j].Add(&out[j], &tmp)
			}
			x.Mul(&x, &randomCoin)
		}
	})

	proof.LinearCombination = polynomials.LCEvalsToCoefficients(d, linComb)

	return proof.LinearCombination
}
