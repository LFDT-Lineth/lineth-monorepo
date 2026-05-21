package vortex

import (
	"fmt"

	poseidon2 "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/ringsis"
	smt "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/smt"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// CheckColumnInclusion checks the merkle proof opening (merkleProofs[i][j], root[i]) for columns[i][j].
// The leaves are poseidon2_bls12377(sis(columns[i][j])).
func CheckColumnInclusion(sis *ringsis.Key, columns [][][]field.Element,
	merkleProofs [][]smt.Proof, commitments []Commitment, WithSis []bool) error {

	if got, want := len(columns), len(commitments); got != want {
		return fmt.Errorf("columns length %d does not match commitments length %d", got, want)
	}
	if got, want := len(merkleProofs), len(commitments); got != want {
		return fmt.Errorf("merkleProofs length %d does not match commitments length %d", got, want)
	}
	if got, want := len(WithSis), len(commitments); got != want {
		return fmt.Errorf("WithSis length %d does not match commitments length %d", got, want)
	}
	for i := range commitments {
		if got, want := len(merkleProofs[i]), len(columns[i]); got != want {
			return fmt.Errorf("merkleProofs[%d] length %d does not match columns[%d] length %d", i, got, i, want)
		}
	}

	sisHash := make([]field.Element, sis.OutputSize())
	h := poseidon2.NewMDHasher()
	for i := 0; i < len(commitments); i++ {

		for j := 0; j < len(columns[i]); j++ {
			var leaf field.Octuplet
			if WithSis[i] {
				if err := sis.SisGnarkCrypto.Hash(columns[i][j], sisHash); err != nil {
					return fmt.Errorf("sis hash failed for commitment %d column %d: %w", i, j, err)
				}
				h.Reset()
				h.WriteElements(sisHash...)
				leaf = h.SumDigest()
			} else {
				// compute leaf = poseidon2_bls12377(columns[i][j]))
				h.Reset()
				h.WriteElements(columns[i][j]...)
				leaf = h.SumDigest()
			}

			// check merkle proof
			err := smt.Verify(&merkleProofs[i][j], leaf, commitments[i])
			if err != nil {
				return err
			}

		}
	}
	return nil
}

// Verify verifies an opening proof against the given commitments and Merkle proofs.
func Verify(params *Params, proof *OpeningProof, vi *VerifierInput, commitments []Commitment,
	merkleProofs [][]smt.Proof, WithSis []bool) error {

	if got, want := len(proof.Columns), len(commitments); got != want {
		return fmt.Errorf("proof.Columns length %d does not match commitments length %d", got, want)
	}
	if got, want := len(merkleProofs), len(commitments); got != want {
		return fmt.Errorf("merkleProofs length %d does not match commitments length %d", got, want)
	}
	if WithSis != nil {
		if got, want := len(WithSis), len(commitments); got != want {
			return fmt.Errorf("WithSis length %d does not match commitments length %d", got, want)
		}
	} else {
		WithSis = make([]bool, len(commitments))
		for i := range WithSis {
			WithSis[i] = true
		}
	}

	if err := VerifyCommon(params, proof, vi); err != nil {
		return err
	}

	return CheckColumnInclusion(params.Key, proof.Columns, merkleProofs, commitments, WithSis)
}

// VerifyCommon performs the common verification steps shared by Verify and
// similar functions. In coefficient mode proof.LinearCombination holds T
// monomial coefficients of U_alpha (not N evaluations), so no separate
// Reed-Solomon codeword check is needed — the codeword property is implicit:
// any T coefficients describe a unique degree-<T polynomial.
func VerifyCommon(params *Params, proof *OpeningProof, vi *VerifierInput) error {
	if got, want := len(proof.LinearCombination), params.RsParams.NbColumns(); got != want {
		return fmt.Errorf("invalid linear combination length: expected %d coefficients, got %d", want, got)
	}

	if len(vi.EntryList) == 0 {
		return fmt.Errorf("empty entry list")
	}
	n := params.RsParams.NbEncodedColumns()
	for j, selectedColID := range vi.EntryList {
		if selectedColID < 0 || selectedColID >= n {
			return fmt.Errorf("entryList[%d] = %d out of bounds [0, %d)", j, selectedColID, n)
		}
	}
	if got, want := len(vi.Ys), len(proof.Columns); got != want {
		return fmt.Errorf("vi.Ys length %d does not match proof.Columns length %d", got, want)
	}
	for i := range proof.Columns {
		if got, want := len(proof.Columns[i]), len(vi.EntryList); got != want {
			return fmt.Errorf("proof.Columns[%d] length %d does not match entryList length %d", i, got, want)
		}
		for j := range proof.Columns[i] {
			if got, want := len(proof.Columns[i][j]), len(vi.Ys[i]); got != want {
				return fmt.Errorf("proof.Columns[%d][%d] length %d does not match vi.Ys[%d] length %d", i, j, got, i, want)
			}
		}
	}

	if err := CheckLinComb(params.RsParams, proof.LinearCombination, vi.EntryList, vi.Alpha, proof.Columns); err != nil {
		return err
	}

	return CheckStatement(proof.LinearCombination, vi.Ys, vi.X, vi.Alpha)
}
