package poseidon2

import (
	"fmt"
	"io"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
)

// BlockSize is the number of field elements in a Poseidon2 compression block.
const BlockSize = 8

// MDHasher Merkle Damgard Hasher using Poseidon2 as compression function
type MDHasher struct {
	// Sponge construction state
	state field.Octuplet

	// data to hash
	buffer []field.Element
}

// NewMDHasher creates a new MDHasher with the given options.
func NewMDHasher() *MDHasher {
	return &MDHasher{}
}

// Size returns the digest size in bytes.
func (d *MDHasher) Size() int { return BlockSize * field.Bytes }

// BlockSize returns the size in bytes of a compression block.
func (d *MDHasher) BlockSize() int { return BlockSize * field.Bytes }

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
func Compress(state, block field.Octuplet) field.Octuplet {
	return vortex.CompressPoseidon2(state, block)
}

// Reset clears the buffer, and reset state to iv
func (d *MDHasher) Reset() {
	d.buffer = d.buffer[:0]
	d.state = field.Octuplet{}
}

// SetStateOctuplet modifies the state (??)
func (d *MDHasher) SetStateOctuplet(state field.Octuplet) {
	d.Reset()
	d.buffer = d.buffer[:0]
	for i := 0; i < 8; i++ {
		d.state[i].Set(&state[i])
	}
}

// GetStateOctuplet returns the current hash state as an octuplet without consuming the buffer.
func (d *MDHasher) GetStateOctuplet() field.Octuplet {
	return absorb(d.state, d.buffer)
}

// WriteElements adds a slice of field elements to the running hash.
func (d *MDHasher) WriteElements(elmts ...field.Element) {
	d.buffer = append(d.buffer, elmts[:]...)
}

// SumDigest finalizes the hash and returns the digest as an octuplet.
func (d *MDHasher) SumDigest() field.Octuplet {
	d.state = absorb(d.state, d.buffer)
	// Truncate in place so the backing array is reused by subsequent writes.
	d.buffer = d.buffer[:0]
	return d.state
}

// absorb compresses the elements of buf into state block by block, left
// padding the last incomplete block with zeroes. It does not allocate.
func absorb(state field.Octuplet, buf []field.Element) field.Octuplet {
	for len(buf) > 0 {
		var block [BlockSize]field.Element
		if len(buf) < BlockSize {
			copy(block[BlockSize-len(buf):], buf)
			buf = nil
		} else {
			copy(block[:], buf[:BlockSize])
			buf = buf[BlockSize:]
		}
		state = vortex.CompressPoseidon2(state, block)
	}
	return state
}

// Write adds a byte slice to the running hash. The input length must be a multiple of 4.
func (d *MDHasher) Write(p []byte) (int, error) {

	elemByteSize := field.Bytes // 4 bytes = 1 field element

	if len(p)%elemByteSize != 0 {
		return 0, fmt.Errorf("input length is not a multiple of 4 byte size")
	}
	// Decode directly into the buffer; on error, roll back to the previous
	// length so a failed Write leaves the hasher unchanged.
	oldLen := len(d.buffer)
	d.buffer = slices.Grow(d.buffer, len(p)/elemByteSize)
	for start := 0; start < len(p); start += elemByteSize {
		var elem field.Element
		if err := elem.SetBytesCanonical(p[start : start+elemByteSize]); err != nil {
			d.buffer = d.buffer[:oldLen]
			return 0, err
		}
		d.buffer = append(d.buffer, elem)
	}
	return len(p), nil
}

// State returns the current state of the hasher as a block-sized array.
func (d *MDHasher) State() [BlockSize]field.Element {
	return d.GetStateOctuplet()
}

// Sum computes the poseidon2 hash of msg
func (d *MDHasher) Sum(msg []byte) []byte {
	if _, err := d.Write(msg); err != nil {
		panic(err)
	}
	h := d.SumDigest()
	h32 := field.OctupletToBytes(h)
	return h32[:]
}

// HashVec hashes a vector of field elements
func HashVec(v ...field.Element) (h field.Octuplet) {
	return absorb(field.Octuplet{}, v)
}
