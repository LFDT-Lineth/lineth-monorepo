package vortex_koalabear

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Check the merkle proof opening (merkleProofs[i][j], root[i]) for columns[i][j].
// The leaves are poseidon2_koalabear(columns[i][j])
func GnarkCheckColumnInclusionNoSis(api frontend.API, columns [][][]koalagnark.Element,
	merkleProofs [][]smt_koalabear.GnarkProof, roots []poseidon2_koalabear.GnarkOctuplet) error {

	for i := 0; i < len(roots); i++ {

		h, err := poseidon2_koalabear.NewGnarkMDHasher(api)
		if err != nil {
			return err
		}

		for j := 0; j < len(columns[i]); j++ {

			// compute leaf = poseidon2_koalabear(right-padded columns[i][j])
			// Poseidon2 on koalabear is never emulated, so we can unwrap the variables.
			// Right-pad to colChunksPad*8 to match the commitment hash.
			colSize := len(columns[i][j])
			colChunksPad := utils.NextPowerOfTwo((colSize + 7) / 8)
			paddedSize := colChunksPad * 8
			currentColumnsUnwrapped := make([]frontend.Variable, paddedSize)
			for k := 0; k < colSize; k++ {
				currentColumnsUnwrapped[k] = columns[i][j][k].Native()
			}
			for k := colSize; k < paddedSize; k++ {
				currentColumnsUnwrapped[k] = 0
			}
			h.Write(currentColumnsUnwrapped...)
			leaf := h.Sum()
			h.Reset()

			// check merkle proof
			err = smt_koalabear.GnarkVerifyMerkleProof(api, merkleProofs[i][j], leaf, roots[i])
			if err != nil {
				return err
			}
		}
	}
	return nil
}
