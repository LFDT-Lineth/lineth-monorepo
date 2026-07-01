package fiatshamir

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/utils"
)

// BitReader is a utility to read a slice of bytes bit by bit. It should be
// initialized via the NewBitReader method. Its main use-case is to help
// converting generate random field elements into a list of bounded integers
// as the list of random column to open in Vortex.
type BitReader struct {
	numBits    int
	curBits    int
	underlying []byte
}

// NewBitReader constructs a [BitReader] based on an underlying array of bytes.
// The underlying parameter is expected to represent a sequence of field elements
// in big endian form.
//
// The BitReader will internally represent the same integers
// in little endian and in reverse order. The reason for this transformation is
// that otherwise it makes it expensive to arithmetize the bit splitting as it
// requires a bit decomposition. In contrast, the little endian form allows
// using look-up table and doing direct limb-decomposition.
//
// In the current version, the BitReader is only instantiated for a single field
// element and in this case, numBytes is set to field.Bytes - 1 (so 31). The
// reason is that the most significant byte cannot be assumed to contain enough
// entropy to properly instantiate a challenge.
//
// numBits corresponds to the designated capacity of the reader: the total
// number of bits we should look for in it.
func NewBitReader(underlying []byte, numBits int) BitReader {
	// Check numBits
	if 8*len(underlying) < numBits {
		utils.Panic("NumBits %v can't be larger than the buffer size %v", numBits, 8*len(underlying))
	}

	return BitReader{
		numBits:    numBits,
		curBits:    0,
		underlying: bigToLittle(underlying),
	}
}

// readLimit characterizes the number of bits that can be read in a single go
// by the [BitReader.ReadInt].
const readLimit int = 30

// ReadInt reads an integer with n bits. It will panic if the number of b
func (r *BitReader) ReadInt(n int) (int, error) {

	if n > readLimit {
		return 0, fmt.Errorf("ReadInt: can't read more than %v, got %v", readLimit, n)
	}

	if n <= 0 {
		return 0, fmt.Errorf("ReadInt: caller passed 0 or negative number of bits, probably a mistake")
	}

	if n+r.curBits > r.numBits {
		return 0, fmt.Errorf("ReadInt: overflown the reader, curBits=%v, nBits=%v and numBits=%v", r.curBits, n, r.numBits)
	}

	res := 0

	// Add the bits one by one
	for i := 0; i < n; i++ {
		curr := r.curBits + i
		selected := r.underlying[curr/8]
		selected >>= curr % 8
		selected &= 1
		res |= int(selected) << i
	}

	r.curBits += n
	return res, nil
}

/*
bigToLittle converts out of place a sequence of bytes from bigToLittle. It works

(i.e) just reverse the order
*/
func bigToLittle(b []byte) []byte {
	res := make([]byte, len(b))
	for i := range res {
		res[i] = b[len(b)-1-i]
	}
	return res
}
