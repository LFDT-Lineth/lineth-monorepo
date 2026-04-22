package vortex

import (
	poseidon2 "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/ringsis"
	smt "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/smt"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// CheckColumnInclusion checks the merkle proof opening (merkleProofs[i][j], root[i]) for columns[i][j].
// The leaves are poseidon2_bls12377(sis(columns[i][j])).
func CheckColumnInclusion(sis *ringsis.Key, columns [][][]field.Element,
	merkleProofs [][]smt.Proof, commitments []Commitment, WithSis []bool) error {

	sisHash := make([]field.Element, sis.OutputSize())
	// var leaf fr.Element
	h := poseidon2.NewMDHasher()
	for i := 0; i < len(commitments); i++ {

		for j := 0; j < len(columns[i]); j++ {
			var leaf field.Octuplet
			if WithSis[i] {
				// compute leaf = poseidon2_bls12377(columns[i][j]))
				if err := sis.SisGnarkCrypto.Hash(columns[i][j], sisHash); err != nil {
					panic(err)
				}
				h.Reset()
				h.WriteElements(sisHash...)
				leaf = h.SumElement()
			} else {
				// compute leaf = poseidon2_bls12377(columns[i][j]))
				h.Reset()
				h.WriteElements(columns[i][j]...)
				leaf = h.SumElement()
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

	// If WithSis is not assigned, we assign them with default true values
	if WithSis == nil {
		WithSis = make([]bool, len(merkleProofs))
		for i := range WithSis {
			WithSis[i] = true
		}
	}

	err := VerifyCommon(params, proof, vi)
	if err != nil {
		return err
	}

	err = CheckColumnInclusion(params.Key, proof.Columns,
		merkleProofs, commitments, WithSis)

	return err
}

// VerifyCommon performs the common verification steps shared by Verify and similar functions.
func VerifyCommon(params *Params, proof *OpeningProof, vi *VerifierInput) error {

	err := CheckIsCodeWord(params.RsParams, proof.LinearCombination)
	if err != nil {
		return err
	}

	err = CheckLinComb(proof.LinearCombination, vi.EntryList, vi.Alpha, proof.Columns)
	if err != nil {
		return err
	}

	err = CheckStatement(proof.LinearCombination, vi.Ys, vi.X, vi.Alpha)
	if err != nil {
		return err
	}

	return err

}
