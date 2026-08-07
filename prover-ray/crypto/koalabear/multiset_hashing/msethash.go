// Package multiset_hashing implements a LtHash-style multiset hash over the
// Koalabear field, and exposes a [Hasher] that satisfies the
// [preflight.AdditiveHasher] interface so it can drive the cross-shard
// shared-randomness protocol.
//
// The accumulator [MSetHash] is an array of [MSetHashSize] field elements
// initialised to zero (the empty-set digest). Inserting a message M maps M
// through Poseidon2 in 41 independent 8-element chunks and adds each chunk
// componentwise to the accumulator; removing subtracts instead. Because the
// group operation is componentwise field addition (commutative and
// associative), the accumulator is order-independent: Insert(A); Insert(B)
// equals Insert(B); Insert(A). The security parameters follow the SIS analysis
// in the linea-monorepo reference implementation (≥128 bits for at most 2^16
// insertions/removals).
package multiset_hashing

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

	// MSetHashSize is the number of field elements in the accumulator.
	MSetHashSize = chunkSize * blockSize
)

// MSetHash is a multiset hash accumulator over the Koalabear field. The zero
// value represents the empty set and is ready to use without initialisation.
type MSetHash [MSetHashSize]field.Element

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

// Hasher implements [preflight.AdditiveHasher][MSetHash]. The zero value is
// ready to use.
//
// Hash maps a Merkle root (8 field elements) into an MSetHash accumulator by
// inserting the root elements. Combine adds two accumulators componentwise.
// ToSeed compresses the 328-element accumulator to a single [field.Octuplet]
// by hashing all elements through Poseidon2; the result seeds SetFSState.
type Hasher struct{}

// Hash implements [preflight.AdditiveHasher].
func (Hasher) Hash(root field.Octuplet) MSetHash {
	var m MSetHash
	m.Insert(root[:]...)
	return m
}

// Combine implements [preflight.AdditiveHasher].
func (Hasher) Combine(a, b MSetHash) MSetHash {
	a.Add(b)
	return a
}

// Identity implements [preflight.AdditiveHasher].
func (Hasher) Identity() MSetHash {
	return MSetHash{}
}

// ToSeed implements [preflight.AdditiveHasher]. It hashes all MSetHashSize
// elements of the accumulator through Poseidon2 and returns the resulting
// octuplet, which is suitable for use as a Fiat-Shamir FS state seed.
func (Hasher) ToSeed(p MSetHash) field.Octuplet {
	hsh := poseidon2.NewMDHasher()
	hsh.WriteElements(p[:]...)
	return hsh.SumDigest()
}
