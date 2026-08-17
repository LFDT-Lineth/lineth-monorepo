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

// NumRows returns the column length for the accumulator module. The +1 keeps at
// least one trailing padding row so the module is never fully packed: the
// distributed prover pads columns to the segment size by repeating the last row,
// which only yields the correct padding when that row is genuine padding. A fully
// packed module otherwise breaks cross-row constraints (e.g. the counter
// increment) on the padded rows.
func (s Settings) NumRows() int {
	return utils.NextPowerOfTwo(s.MaxNumProofs + 1)
}
