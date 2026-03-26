package vortex_bls12377

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_bls12377"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// Check the merkle proof opening (merkleProofs[i][j], root[i]) for columns[i][j].
// The leaves are poseidon2_bls12377(columns[i][j])
func GnarkCheckColumnInclusionNoSis(api frontend.API, columns [][][]koalagnark.Element,
	merkleProofs [][]smt_bls12377.GnarkProof, roots []frontend.Variable) error {

	for i := 0; i < len(roots); i++ {

		h, err := poseidon2_bls12377.NewGnarkMDHasher(api)
		if err != nil {
			return err
		}

		for j := 0; j < len(columns[i]); j++ {

			// compute leaf = poseidon2_bls12377(columns[i][j]))
			h.WriteWVs(columns[i][j]...)
			leaf := h.Sum()
			h.Reset()

			err = smt_bls12377.GnarkVerifyMerkleProof(api, merkleProofs[i][j], leaf, roots[i])
			if err != nil {
				return err
			}
		}
	}
	return nil
}
