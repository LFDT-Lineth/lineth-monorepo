package poseidon2

import (
	"io"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"

	gp2 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// BlockSize is the number of field elements in a Poseidon2 compression block.
const BlockSize = gp2.BlockSize

// Poseidon2 parameters of the compression permutation over KoalaBear.
const (
	Width           = gp2.CompressionWidth
	NbFullRounds    = 6
	NbPartialRounds = 21
)

// MDHasher Merkle Damgard Hasher using Poseidon2 as compression function. It
// is the KoalaBear instance of the generic [gp2.MDHasher].
type MDHasher = gp2.MDHasher[field.Element, *field.Element]

// nativePermutation is the generic Poseidon2 permutation instantiated over
// KoalaBear with gnark-crypto's default seed. [Compress] uses gnark-crypto's
// vectorized implementation instead; both are tested to agree.
var nativePermutation = gp2.NewPermutation(gp2.NewParameters[field.Element](
	Width, NbFullRounds, NbPartialRounds,
	gp2.DefaultSeed("koalabear", Width, NbFullRounds, NbPartialRounds),
))

// Permutation returns the generic Poseidon2 permutation over KoalaBear backing
// [Compress].
func Permutation() *gp2.Permutation[field.Element, *field.Element] { return nativePermutation }

// NewMDHasher creates a new MDHasher.
func NewMDHasher() *MDHasher {
	return gp2.NewMDHasher[field.Element, *field.Element](Compress)
}

// HashBytes hashes an array of bytes and returns the result. It assumes the
// bytes can be directly converted into blocks of octuplet after zero-padding.
func HashBytes(x []byte) []byte {
	state := NewMDHasher()
	if _, err := state.Write(x); err != nil {
		panic(err)
	}
	return state.Sum(nil)
}

// HashWriterTo hashes a [io.WriterTo] using poseidon koalabear
func HashWriterTo(w io.WriterTo) []byte {
	state := NewMDHasher()
	if _, err := w.WriteTo(state); err != nil {
		panic(err)
	}
	return state.Sum(nil)
}

// Compress calls the compression function of Poseidon2 over state and block.
// It is equal to gp2.Compress(Permutation(), state, block), computed with
// gnark-crypto's optimized permutation.
func Compress(state, block field.Octuplet) field.Octuplet {
	return vortex.CompressPoseidon2(state, block)
}

// HashVec hashes a vector of field elements
func HashVec(v ...field.Element) (h field.Octuplet) {
	return gp2.HashVec[field.Element, *field.Element](Compress, v...)
}
