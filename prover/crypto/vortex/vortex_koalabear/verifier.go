package vortex_koalabear

import (
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

// Check the merkle proof opening (merkleProofs[i][j], root[i]) for columns[i][j].
// The leaves are poseidon2_bls12377(sis(columns[i][j]))
func CheckColumnInclusion(sis *ringsis.Key, columns [][][]field.Element,
	merkleProofs [][]smt_koalabear.Proof, commitments []Commitment, WithSis []bool) error {

	sisHash := make([]field.Element, sis.OutputSize())
	// var leaf fr.Element
	h := poseidon2_koalabear.NewMDHasher()
	for i := 0; i < len(commitments); i++ {

		for j := 0; j < len(columns[i]); j++ {
			var leaf field.Octuplet
			if WithSis[i] {

				// compute leaf = poseidon2_bls12377(columns[i][j]))
				sis.SisGnarkCrypto.Hash(columns[i][j], sisHash)
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
			err := smt_koalabear.Verify(&merkleProofs[i][j], leaf, commitments[i])
			if err != nil {
				return err
			}

		}
	}
	return nil
}

func Verify(params *Params, proof *vortex.OpeningProof, vi *vortex.VerifierInput, commitments []Commitment, merkleProofs [][]smt_koalabear.Proof, WithSis []bool) error {

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

func VerifyCommon(params *Params, proof *vortex.OpeningProof, vi *vortex.VerifierInput) error {

	err := vortex.CheckIsCodeWord(params.RsParams, proof.LinearCombination)
	if err != nil {
		return err
	}

	err = vortex.CheckLinComb(proof.LinearCombination, vi.EntryList, vi.Alpha, proof.Columns)
	if err != nil {
		return err
	}

	err = vortex.CheckStatement(proof.LinearCombination, vi.Ys, vi.X, vi.Alpha)
	if err != nil {
		return err
	}

	return err

}
