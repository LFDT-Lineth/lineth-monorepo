//go:build ignore

// Experiment: crypto/poseidon2/hasher.go with an allocation-free KoalaBear
// fast path in Compress (generic-asm only). Not compiled; apply
// compress-fastpath.patch from prover-ray/ to try it.

package poseidon2

import (
	"fmt"
	"slices"

	"github.com/consensys/gnark-crypto/field/koalabear"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/generic"
)

// BlockSize is the number of field elements in a compression block (and in
// the digest). The compression permutation has width 2·BlockSize.
const BlockSize = 8

// CompressionWidth is the width of the permutation used by [Compress].
const CompressionWidth = 2 * BlockSize

// Octuplet is a compression block / digest over the field T.
type Octuplet[T any] = [BlockSize]T

// CompressFunc is a 2-to-1 compression function over octuplets.
type CompressFunc[T any] func(state, block Octuplet[T]) Octuplet[T]

// Compress returns P(state ‖ block)[8..16] + block, where P is perm (of width
// 16). This matches gnark-crypto's koalabear vortex.CompressPoseidon2.
func Compress[T any, PT generic.Element[T]](perm *Permutation[T, PT], state, block Octuplet[T]) Octuplet[T] {
	if perm.kbAsm != nil {
		// Concrete KoalaBear types keep x and res on the stack: no indirect
		// calls through the generic dictionary, so nothing escapes.
		var y [CompressionWidth]koalabear.Element
		copy(y[:BlockSize], asKB(state[:]))
		copy(y[BlockSize:], asKB(block[:]))
		if err := perm.kbAsm.Permutation(y[:]); err != nil {
			panic(err)
		}
		res := block
		r := asKB(res[:])
		for i := range r {
			r[i].Add(&r[i], &y[BlockSize+i])
		}
		return res
	}
	var x [CompressionWidth]T
	copy(x[:BlockSize], state[:])
	copy(x[BlockSize:], block[:])
	if err := perm.Permute(x[:]); err != nil {
		panic(err) // can't happen, the width is fixed
	}
	res := block
	for i := range res {
		PT(&res[i]).Add(&res[i], &x[BlockSize+i])
	}
	return res
}

// Compressor returns perm's [Compress] as a [CompressFunc].
func Compressor[T any, PT generic.Element[T]](perm *Permutation[T, PT]) CompressFunc[T] {
	return func(state, block Octuplet[T]) Octuplet[T] { return Compress(perm, state, block) }
}

// MDHasher is a Merkle-Damgård hasher over the field T using a Poseidon2
// compression function. Incomplete trailing blocks are left-padded with zeroes.
type MDHasher[T any, PT generic.Element[T]] struct {
	compress CompressFunc[T]

	state Octuplet[T]
	// data to hash
	buffer []T
}

// NewMDHasher returns a hasher using the given compression function.
func NewMDHasher[T any, PT generic.Element[T]](compress CompressFunc[T]) *MDHasher[T, PT] {
	return &MDHasher[T, PT]{compress: compress}
}

// Reset clears the buffer and resets the state to the zero IV.
func (d *MDHasher[T, PT]) Reset() {
	d.buffer = d.buffer[:0]
	d.state = Octuplet[T]{}
}

// SetStateOctuplet clears the buffer and sets the state.
func (d *MDHasher[T, PT]) SetStateOctuplet(state Octuplet[T]) {
	d.buffer = d.buffer[:0]
	d.state = state
}

// GetStateOctuplet returns the digest of what has been written so far,
// without modifying the hasher.
func (d *MDHasher[T, PT]) GetStateOctuplet() Octuplet[T] {
	oldState := d.state
	oldBuffer := slices.Clone(d.buffer)

	res := d.SumDigest() // this flushes the hasher

	d.state = oldState
	d.buffer = oldBuffer
	return res
}

// State is an alias for [MDHasher.GetStateOctuplet].
func (d *MDHasher[T, PT]) State() Octuplet[T] {
	return d.GetStateOctuplet()
}

// WriteElements adds field elements to the running hash.
func (d *MDHasher[T, PT]) WriteElements(elmts ...T) {
	d.buffer = append(d.buffer, elmts...)
}

// SumDigest flushes the buffer and returns the digest.
func (d *MDHasher[T, PT]) SumDigest() Octuplet[T] {
	for len(d.buffer) != 0 {
		var buf Octuplet[T]
		if len(d.buffer) < BlockSize {
			// left pad by zeroes
			copy(buf[BlockSize-len(d.buffer):], d.buffer)
			d.buffer = d.buffer[:0]
		} else {
			copy(buf[:], d.buffer)
			d.buffer = d.buffer[BlockSize:]
		}
		d.state = d.compress(d.state, buf)
	}
	return d.state
}

// Write adds canonically-encoded field elements to the running hash. The
// input length must be a multiple of the field element encoding size.
func (d *MDHasher[T, PT]) Write(p []byte) (int, error) {
	elemByteSize := generic.ByteSize[T, PT]()
	if len(p)%elemByteSize != 0 {
		return 0, fmt.Errorf("input length is not a multiple of %d bytes", elemByteSize)
	}
	elems := make([]T, len(p)/elemByteSize)
	for i := range elems {
		if err := PT(&elems[i]).SetBytesCanonical(p[i*elemByteSize : (i+1)*elemByteSize]); err != nil {
			return 0, err
		}
	}
	d.WriteElements(elems...)
	return len(p), nil
}

// Sum writes msg, finalizes the hash and returns the encoded digest. Unlike
// [hash.Hash.Sum], msg is hashed rather than used as a prefix of the output.
func (d *MDHasher[T, PT]) Sum(msg []byte) []byte {
	if _, err := d.Write(msg); err != nil {
		panic(err)
	}
	return OctupletToBytes[T, PT](d.SumDigest())
}

// Size returns the size in bytes of the encoded digest.
func (d *MDHasher[T, PT]) Size() int { return BlockSize * generic.ByteSize[T, PT]() }

// BlockSize returns the size in bytes of an encoded compression block.
func (d *MDHasher[T, PT]) BlockSize() int { return d.Size() }

// OctupletToBytes concatenates the canonical encodings of the elements.
func OctupletToBytes[T any, PT generic.Element[T]](o Octuplet[T]) []byte {
	res := make([]byte, 0, BlockSize*generic.ByteSize[T, PT]())
	for i := range o {
		res = append(res, PT(&o[i]).Marshal()...)
	}
	return res
}

// HashVec hashes a vector of field elements with a fresh hasher.
func HashVec[T any, PT generic.Element[T]](compress CompressFunc[T], v ...T) Octuplet[T] {
	h := NewMDHasher[T, PT](compress)
	h.WriteElements(v...)
	return h.SumDigest()
}
