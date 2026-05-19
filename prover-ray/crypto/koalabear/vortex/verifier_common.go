package vortex

import (
	"errors"
	"fmt"
	"slices"

	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/reedsolomon"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/polynomials"
)

var (
	// ErrInvalidVerifierInputs flags a verifier input with invalid dimensions.
	ErrInvalidVerifierInputs = errors.New("invalid verification input")
)

// VerifierInput holds the public inputs needed by the verifier.
type VerifierInput struct {
	// RsParams *reedsolomon.RsParams

	// alpha random coin used for the linear combination
	Alpha field.Ext

	// X is the univariate evaluation point
	X field.Ext

	// Ys are the alleged evaluation at point X
	Ys [][]field.Ext

	// EntryList is the random coin representing the columns to open.
	EntryList []int
}

// CheckStatement verifies that the linear combination U_alpha — given as T
// monomial coefficients — is consistent with the claimed evaluations ys at
// point x. The check is U_alpha(x) == ∑ᵢ αⁱ · yJoinedᵢ where U_alpha is
// evaluated by Horner on its coefficients.
func CheckStatement(coefficients []field.Ext, ys [][]field.Ext, x, alpha field.Ext) error {

	yJoined := slices.Concat(ys...)
	alphaY := polynomials.EvalCanonical(field.VecFromExt(coefficients), field.ElemFromExt(x)).AsExt()
	alphaYPrime := polynomials.EvalCanonical(field.VecFromExt(yJoined), field.ElemFromExt(alpha)).AsExt()

	if !alphaY.Equal(&alphaYPrime) {
		return fmt.Errorf("RowLincomb and Y are inconsistent")
	}

	return nil
}

// CheckLinComb verifies that U_alpha — given as T monomial coefficients —
// agrees with the alpha-Horner combination of the opened columns at every
// selected position. The verifier expands the coefficients to all N domain
// evaluations once and then looks up the relevant U_alpha(ω^selectedColID).
func CheckLinComb(
	rsParams *reedsolomon.RsParams,
	coefficients []field.Ext,
	entryList []int,
	alpha field.Ext,
	columns [][][]field.Element,
) (err error) {

	evals := rsParams.EncodeFromMonomials(coefficients)
	numCommitments := len(columns)

	for j, selectedColID := range entryList {
		// Will carry the concatenation of the columns for the same entry j
		fullCol := []field.Element{}

		for i := range numCommitments {
			// Entries of the selected columns #j contained in the commitment #i.
			fullCol = append(fullCol, columns[i][j]...)
		}

		// U_alpha(ω^selectedColID) must equal α-Horner of the opened column
		y := polynomials.EvalCanonical(field.VecFromBase(fullCol), field.ElemFromExt(alpha)).AsExt()
		other := evals[selectedColID]

		if !y.Equal(&other) {
			return fmt.Errorf("the linear combination is inconsistent %v : %v", y.String(), other.String())
		}
	}

	return nil
}
