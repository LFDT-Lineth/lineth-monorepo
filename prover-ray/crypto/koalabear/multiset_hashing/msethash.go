// Package multiset_hashing implements a LtHash-style multiset hash over the
// Koalabear field. On top of the [MSetHash] accumulator it exposes an additive
// group API ([Hash], [Combine], [Identity], [ToSeed]) that drives the
// cross-shard shared-randomness protocol in
// [github.com/LFDT-Lineth/lineth-monorepo/prover-ray/preflight].
//
// The accumulator [MSetHash] is an array of [MSetHashSizeNumFieldElement] field elements
// initialised to zero (the empty-set digest). Inserting a message M maps M
// through Poseidon2 in 41 independent 8-element chunks and adds each chunk
// componentwise to the accumulator; removing subtracts instead. Because the
// group operation is componentwise field addition (commutative and
// associative), the accumulator is order-independent: Insert(A); Insert(B)
// equals Insert(B); Insert(A). The security parameters follow the SIS analysis
// in the linea-monorepo reference implementation (≥128 bits for at most 2^16
// insertions/removals).
package multisethashing

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

const (
	// chunkSize is the number of independent Poseidon2 output blocks used to
	// represent one message. Chosen so that MSetHashSize = chunkSize * blockSize
	// satisfies the SIS security bound (≥128 bits, ≤2^16 insertions/removals).
	chunkSize = 41
	// blockSize matches poseidon2.BlockSize (8 field elements per compression
	// output). Defined locally to avoid importing poseidon2 just for the
	// constant; the actual hashing still uses poseidon2.NewMDHasher.
	blockSize = poseidon2.BlockSize

	// MSetHashSizeNumFieldElement is the number of field elements in the accumulator.
	MSetHashSizeNumFieldElement = chunkSize * blockSize
)

// MSetHash is a multiset hash accumulator over the Koalabear field. The zero
// value represents the empty set and is ready to use without initialisation.
type MSetHash [MSetHashSizeNumFieldElement]field.Element

// Insert adds msg to the accumulator. Panics on an empty msg.
func (m *MSetHash) Insert(msg ...field.Element) {
	m.update(false, msg...)
}

// Remove removes msg from the accumulator. Panics on an empty msg.
func (m *MSetHash) Remove(msg ...field.Element) {
	m.update(true, msg...)
}

// Add combines two accumulators in place: m += other.
func (m *MSetHash) Add(other MSetHash) {
	for i := range m {
		m[i].Add(&m[i], &other[i])
	}
}

// IsEmpty reports whether the accumulator represents the empty set.
func (m *MSetHash) IsEmpty() bool {
	for i := range m {
		if !m[i].IsZero() {
			return false
		}
	}
	return true
}

// update adds (rem=false) or removes (rem=true) msg from the accumulator.
func (m *MSetHash) update(rem bool, msg ...field.Element) {
	if len(msg) == 0 {
		panic("multiset_hashing: Insert/Remove requires a non-empty message")
	}

	var zeros [blockSize]field.Element // zero octuplet for state-advance steps
	hsh := poseidon2.NewMDHasher()
	hsh.WriteElements(msg...)

	for i := 0; i < chunkSize; i++ {
		chunk := hsh.SumDigest()
		if rem {
			for j := 0; j < blockSize; j++ {
				m[i*blockSize+j].Sub(&m[i*blockSize+j], &chunk[j])
			}
		} else {
			for j := 0; j < blockSize; j++ {
				m[i*blockSize+j].Add(&m[i*blockSize+j], &chunk[j])
			}
		}
		if i < chunkSize-1 {
			hsh.WriteElements(zeros[:]...)
		}
	}
}

// The functions below present the multiset hash as an additive group over
// [MSetHash]: [Hash] maps a value in, [Combine] is the group operation,
// [Identity] is the neutral element, and [ToSeed] compresses a group element
// back to a single octuplet.

// Hash maps a Merkle root (8 field elements) into a fresh accumulator by
// inserting the root elements.
func Hash(root field.Octuplet) MSetHash {
	var m MSetHash
	m.Insert(root[:]...)
	return m
}

// Combine is the group operation: componentwise field addition, hence
// commutative and associative.
func Combine(a, b MSetHash) MSetHash {
	a.Add(b)
	return a
}

// Identity returns the neutral element of the group, i.e. the empty-set digest.
func Identity() MSetHash {
	return MSetHash{}
}

// ToSeed hashes all MSetHashSize elements of the accumulator through Poseidon2
// and returns the resulting octuplet, which is suitable for use as a
// Fiat-Shamir FS state seed.
func ToSeed(p MSetHash) field.Octuplet {
	hsh := poseidon2.NewMDHasher()
	hsh.WriteElements(p[:]...)
	return hsh.SumDigest()
}
