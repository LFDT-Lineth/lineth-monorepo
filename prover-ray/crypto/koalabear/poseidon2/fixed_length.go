package poseidon2

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	gnarkposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
)

var fixedLengthBatchPoseidon2 = gnarkposeidon2.NewPermutation(16, 6, 21)

// FixedLengthHasher is an IV-elided Poseidon2 compression chain. It is
// collision-resistant only when the caller fixes the exact canonical element
// length before writing; different lengths are not domain-separated. Empty
// input returns the zero digest, and a one-block input is returned directly.
type FixedLengthHasher struct {
	state          field.Octuplet
	buffer         field.Octuplet
	bufferPosition int
	initialized    bool
}

// NewFixedLengthHasher creates an IV-elided hasher. The caller fixes the
// preimage length by choosing the canonical row/table shape before writing.
func NewFixedLengthHasher() *FixedLengthHasher {
	return &FixedLengthHasher{}
}

// Reset clears the state and buffered elements.
func (h *FixedLengthHasher) Reset() {
	h.state = field.Octuplet{}
	h.buffer = field.Octuplet{}
	h.bufferPosition = 0
	h.initialized = false
}

// WriteElements appends field elements to the compression chain.
func (h *FixedLengthHasher) WriteElements(elements ...field.Element) {
	for len(elements) != 0 {
		n := min(len(elements), BlockSize-h.bufferPosition)
		copy(h.buffer[h.bufferPosition:h.bufferPosition+n], elements[:n])
		h.bufferPosition += n
		elements = elements[n:]
		if h.bufferPosition == BlockSize {
			h.absorbBlock()
		}
	}
}

func (h *FixedLengthHasher) absorbBlock() {
	if h.initialized {
		h.state = Compress(h.state, h.buffer)
	} else {
		h.state = h.buffer
		h.initialized = true
	}
	h.buffer = field.Octuplet{}
	h.bufferPosition = 0
}

// SumDigest returns the digest without consuming buffered input.
func (h *FixedLengthHasher) SumDigest() field.Octuplet {
	final := *h
	if final.bufferPosition != 0 {
		n := final.bufferPosition
		copy(final.buffer[BlockSize-n:], final.buffer[:n])
		clear(final.buffer[:BlockSize-n])
		final.absorbBlock()
	}
	return final.state
}

// CompressFixedLengthx16Columns computes 16 independent IV-elided chains from
// a column-major matrix. colSize is the fixed padded stream length in elements
// per lane and must be a positive multiple of BlockSize. state is caller-owned
// scratch space of length 16*BlockSize; it is reused to avoid an allocation per
// SIMD group. The first block becomes each lane's initial state, so this saves
// the permutation that an IV-based chain would spend absorbing that block.
func CompressFixedLengthx16Columns(
	state, matrix []field.Element,
	colSize int,
	result []field.Octuplet,
) {
	const lanes = 16
	if colSize == BlockSize {
		for lane := range lanes {
			for column := range BlockSize {
				result[lane][column] = matrix[column*lanes+lane]
			}
		}
		return
	}

	const firstBlockElements = lanes * BlockSize
	copy(state, matrix[:firstBlockElements])
	fixedLengthBatchPoseidon2.Compressx16ColumnsWithState(
		state,
		matrix[firstBlockElements:],
		colSize-BlockSize,
		result,
	)
}
