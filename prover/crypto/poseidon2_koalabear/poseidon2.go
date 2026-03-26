package poseidon2_koalabear

import (
	"fmt"
	"io"
	"slices"

	"github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/gnark/frontend"

	gnarkposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

const BlockSize = 8

// MDHasher Merkle Damgard Hasher using Poseidon2 as compression function
type MDHasher struct {
	hash.StateStorer

	// Sponge construction state
	state field.Octuplet

	// data to hash
	buffer         []field.Element
	bufferPosition int
}

// NewMDHasher creates a new MDHasher with the given options.
func NewMDHasher() *MDHasher {
	h := &MDHasher{
		StateStorer: gnarkposeidon2.NewMerkleDamgardHasher(),
		state:       field.Octuplet{},
	}

	return h
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
func Compress(state, block field.Octuplet) field.Octuplet {
	return vortex.CompressPoseidon2(state, block)
}

// Reset clears the buffer, and reset state to iv
func (d *MDHasher) Reset() {
	d.bufferPosition = 0
	d.state = field.Octuplet{}
}

// SetStateOctuplet modifies the state (??)
func (d *MDHasher) SetStateOctuplet(state field.Octuplet) {
	d.Reset()
	d.buffer = d.buffer[:0]
	d.bufferPosition = 0
	for i := 0; i < 8; i++ {
		d.state[i].Set(&state[i])
	}
}

func (d *MDHasher) GetStateOctuplet() field.Octuplet {
	// State will flush the buffer, take the state and restore the initiat
	// state of the hasher.
	oldState := d.state
	oldBuffer := slices.Clone(d.buffer)
	oldBufferPosition := d.bufferPosition

	_ = d.SumElement() // this flushes the hasher
	res := slices.Clone(d.state[:])

	d.state = oldState
	d.buffer = oldBuffer
	d.bufferPosition = oldBufferPosition

	return field.Octuplet(res)
}

// WriteElements adds a slice of field elements to the running hash.
func (d *MDHasher) WriteElements(elmts ...field.Element) {
	d.buffer = append(d.buffer, elmts[:]...)
}

func (d *MDHasher) SumElement() field.Octuplet {
	for len(d.buffer) != 0 {
		var buf [BlockSize]field.Element

		// in this case we left pad by zeroes
		if len(d.buffer) < BlockSize {
			copy(buf[BlockSize-len(d.buffer):], d.buffer)
			d.buffer = d.buffer[:0]
		} else {
			copy(buf[:], d.buffer)
			d.buffer = d.buffer[BlockSize:]
		}

		d.state = vortex.CompressPoseidon2(d.state, buf)
	}

	return d.state
}

// WriteElements adds a slice of field elements to the running hash.
func (d *MDHasher) Write(p []byte) (int, error) {

	elemByteSize := field.Bytes // 4 bytes = 1 field element

	if len(p)%elemByteSize != 0 {
		return 0, fmt.Errorf("input length is not a multiple of 4 byte size")
	}
	elems := make([]field.Element, 0, len(p)/elemByteSize)

	for start := 0; start < len(p); start += elemByteSize {
		chunk := p[start : start+elemByteSize]
		var elem field.Element
		if err := elem.SetBytesCanonical(chunk); err != nil {
			return 0, err
		}
		elems = append(elems, elem)

	}
	d.WriteElements(elems...)
	return len(p), nil
}
func (h *MDHasher) State() [BlockSize]field.Element {
	return h.GetStateOctuplet()
}

// Sum computes the poseidon2 hash of msg
func (d *MDHasher) Sum(msg []byte) []byte {
	if _, err := d.Write(msg); err != nil {
		panic(err)
	}
	h := d.SumElement()
	return types.KoalaOctuplet(h).ToBytes()
}

// GnarkBlockCompression applies the poseidon permutation to a given block within
// a gnark circuit and mirrors exactly [BlockCompression].
func GnarkBlockCompressionMekle(api frontend.API, oldState, block [BlockSize]koalagnark.Element) (newState [BlockSize]koalagnark.Element) {
	panic("unimplemented")
}

// HashVec hashes a vector of field elements
func HashVec(v ...field.Element) (h field.Octuplet) {
	state := NewMDHasher()
	state.WriteElements(v...)
	return state.SumElement()
}
