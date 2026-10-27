package poseidon2

import (
	"github.com/consensys/gnark-crypto/hash"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	gnarkposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
)

// BlockSize is the number of field elements in a Poseidon2 compression block.
const BlockSize = gnarkposeidon2.BlockLen

// MDHasher is the Merkle-Damgard hasher using Poseidon2 as compression
// function. An incomplete last block is left-padded with zeros. Call PadBlock
// before SumDigest to commit the padded block to the chaining state.
type MDHasher = gnarkposeidon2.Hasher

// compressor is stateless and safe for concurrent use, so it is shared by
// Compress and the Fiat-Shamir transcripts.
var compressor = gnarkposeidon2.NewCompressor(gnarkposeidon2.NewDefaultPermutation())

// Compressor returns the Poseidon2 compression function.
func Compressor() hash.FieldCompressor[field.Octuplet] { return compressor }

// NewMDHasher creates a new MDHasher with the zero initial state.
func NewMDHasher() *MDHasher {
	return gnarkposeidon2.NewHasher()
}

// Compress calls the compression function of Poseidon2 over state and block.
func Compress(state, block field.Octuplet) field.Octuplet {
	compressor.CompressField(&state, &state, &block)
	return state
}
