// keccak gives a toy implementation of the keccak hash function.
// It is aimed at helping to test the Wizard implementation of Keccak that
// we are developping by exposing all the underlying functions.
package keccak

import (
	"bytes"
)

const (
	//rate in byte
	Rate = 136
	//output length in byte
	OutputLen = 32
	// domain separator byte of Keccak
	Dsb = byte(0x01)
)

// State represents the Keccak state.
//
// The indexing is not made consistent with the following resource
// https://keccak.team/keccak_specs_summary.html
//
// Namely, State[X][Y] gives the S[X, Y] in the spec.
type State [5][5]uint64

// Block represents a keccak block
type Block = [Rate / 8]uint64

// Digest represents the output of a keccak hash
type Digest = [OutputLen]byte

// PermTraces represents the traces of the keccakf permutation happening when
// hashing a long string
type PermTraces struct {
	KeccakFInps []State
	KeccakFOuts []State
	Blocks      []Block
	// indicates whether the current perm is for the first block of a hash?
	IsNewHash  []bool
	HashOutPut []Digest
}

// Hash computes the keccak hash of a slice of bytes. The user can optionally
// pass a PermTraces to which the function will append the generated permutation
// traces throughout the hashing process.
func Hash(stream []byte, maybeTraces ...*PermTraces) (digest Digest) {

	state := State{}
	var block *Block

	// optionally passes the traces
	var traces *PermTraces
	if len(maybeTraces) > 0 {
		traces = maybeTraces[0]
	}

	isFirstPermInHash := true

	// absorb the stream
	for len(stream) >= Rate {
		block, stream = ExtractBlock(stream)
		state.XorIn(block, traces)
		state.Permute(traces)

		// track whether this is the first permutation of a hash
		if traces != nil {
			traces.IsNewHash = append(traces.IsNewHash, isFirstPermInHash)
			isFirstPermInHash = false
		}
	}

	// absorb the padding block
	block = PaddingBlock(stream)
	state.XorIn(block, traces)
	state.Permute(traces)

	// track whether this is the first permutation of a hash. If the stream is
	// smaller than a block, this may be in fact the first and only permutation
	// in the entire hash.
	if traces != nil {
		traces.IsNewHash = append(traces.IsNewHash, isFirstPermInHash)
		// The assignment below is ineffective since this is in all cases the
		// last permutation for the current hash but we leave it here for the
		// sake of homogeneity.
		// isFirstPermInHash = false
	}

	// extract the digest, no need to squeeze because the output
	// is small enough.
	res := state.ExtractDigest()
	if traces != nil {
		traces.HashOutPut = append(traces.HashOutPut, res)
	}

	return res
}

// PadStream returns the input stream padded following the specification of keccak
func PadStream(stream []byte) []byte {

	var (
		buf      = &bytes.Buffer{}
		nbZeroes = Rate - ((len(stream) + 1) % Rate)
	)

	buf.Write(stream)
	buf.WriteByte(Dsb)
	buf.Write(make([]byte, nbZeroes))
	paddedStream := buf.Bytes()
	paddedStream[len(paddedStream)-1] ^= 0x80

	return paddedStream
}

// ExtractBlock extracts a block from the stream and returns the extracted block.
// remStream is what has not been assigned to a block. Returns an error if stream
// is empty.
func ExtractBlock(stream []byte) (block *Block, remStream []byte) {
	// If the stream is smaller than a block, then we should
	// be using the padding functionality.
	if len(stream) < Rate {
		panic("stream is too small")
	}

	block = bytesAsBlockPtrUnsafe(stream)
	remStream = stream[Rate:]
	return block, remStream
}

// PaddingBlock constructs the padding block from a stream whose length is
// assumed to be smaller then the block length
func PaddingBlock(stream []byte) *Block {
	// Allocate the block to not mutate the input stream
	blockBytes := make([]byte, 0, Rate)
	blockBytes = append(blockBytes, stream...)

	// Applies the domain separator
	blockBytes = append(blockBytes, Dsb)

	// Zero pad on the right
	zeroes := make([]byte, Rate-len(stream)-1)
	blockBytes = append(blockBytes, zeroes...)

	// And xor the final byte
	blockBytes[Rate-1] ^= 0x80 // applies the final bit
	return bytesAsBlockPtrUnsafe(blockBytes)
}

// Xor the input into the state of the hasher
func (state *State) XorIn(block *Block, traces *PermTraces) {

	// Optionally trace the block
	if traces != nil {
		traces.Blocks = append(traces.Blocks, *block)
	}

	// Apply the block over the state
	state[0][0] ^= block[0]
	state[1][0] ^= block[1]
	state[2][0] ^= block[2]
	state[3][0] ^= block[3]
	state[4][0] ^= block[4]
	state[0][1] ^= block[5]
	state[1][1] ^= block[6]
	state[2][1] ^= block[7]
	state[3][1] ^= block[8]
	state[4][1] ^= block[9]
	state[0][2] ^= block[10]
	state[1][2] ^= block[11]
	state[2][2] ^= block[12]
	state[3][2] ^= block[13]
	state[4][2] ^= block[14]
	state[0][3] ^= block[15]
	state[1][3] ^= block[16]
}

// ExtractDigest returns the digest
func (state *State) ExtractDigest() Digest {
	return castDigest(state[0][0], state[1][0], state[2][0], state[3][0])
}

// it generates [PermTraces] from the given stream.
func GenerateTrace(streams [][]byte) (t PermTraces) {
	for _, stream := range streams {
		Hash(stream, &t)
	}
	return t
}
