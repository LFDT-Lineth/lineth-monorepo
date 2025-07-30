package accumulator

import "github.com/consensys/linea-monorepo/prover/utils"

// Settings collects all input parameters to dimension an [Module] during
// its construction.
type Settings struct {
	// MaxNbProof is the maximum number of accumulator proofs that the accumulator
	// can verify.
	MaxNumProofs int
	// Name is a string identifying the accumulator module to construct. It is
	// not used as only one instance per Wizard exists.
	Name string
	// MerkleTreeDepth is the depth of the Merkle tree to use to construct the
	// accumulator. In production, we use a value of 40 and this should not be
	// changed as this would modify the state.
	MerkleTreeDepth int
	// Round denotes the interaction round at which the module should be
	// constructed. In production, this should always be zero.
	Round int
}

// leaveSizes returns the column length for the
func (s Settings) NumRows() int {
	return utils.NextPowerOfTwo(s.MaxNumProofs)
}
