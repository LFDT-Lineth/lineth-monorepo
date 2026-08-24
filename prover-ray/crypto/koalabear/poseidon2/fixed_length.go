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

// NewFixedLengthHasher creates an IV-elided hasher.
func NewFixedLengthHasher() *FixedLengthHasher {
	return &FixedLengthHasher{}
}

// Reset clears the state and buffered elements.
func (h *FixedLengthHasher) Reset() {
	*h = FixedLengthHasher{}
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

// SumDigest finalizes and returns the digest.
func (h *FixedLengthHasher) SumDigest() field.Octuplet {
	if h.bufferPosition != 0 {
		n := h.bufferPosition
		copy(h.buffer[BlockSize-n:], h.buffer[:n])
		clear(h.buffer[:BlockSize-n])
		h.absorbBlock()
	}
	return h.state
}

// CompressFixedLengthx16Columns computes 16 independent IV-elided chains from
// a column-major matrix. colSize is the fixed padded stream length in elements
// per lane and must be a positive multiple of BlockSize. The first block
// becomes each lane's initial state, so this saves the permutation that an
// IV-based chain would spend absorbing that block.
func CompressFixedLengthx16Columns(
	matrix []field.Element,
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
	fixedLengthBatchPoseidon2.Compressx16ColumnsWithState(
		matrix[:firstBlockElements],
		matrix[firstBlockElements:],
		colSize-BlockSize,
		result,
	)
}
