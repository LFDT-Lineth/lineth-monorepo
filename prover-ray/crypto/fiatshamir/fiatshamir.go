// Package fiatshamir implements a Fiat-Shamir transcript generically over any
// prime field satisfying [generic.Element], hashing with a Poseidon2
// Merkle-Damgård sponge over the same field.
//
// Field-specific extensions (e.g. sampling extension-field challenges) are
// built on top of it, see crypto/koalabear/fiatshamir.
package fiatshamir

import (
	"golang.org/x/crypto/blake2b"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/generic"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

// https://blog.trailofbits.com/2022/04/18/the-frozen-heart-vulnerability-in-plonk/

// FiatShamir accumulates the transcript of a protocol (e.g. the set of messages
// sent between the prover and the verifier). The accumulated transcripts can
// then be used to sample random coins that are obtained by hashing the
// provided transcript. The hashing is incremental and can be updated at any
// moment.
type FiatShamir[T any, PT generic.Element[T]] struct {
	h *poseidon2.MDHasher[T, PT]
}

// New creates a transcript hashing with the given compression function.
func New[T any, PT generic.Element[T]](compress poseidon2.CompressFunc[T]) *FiatShamir[T, PT] {
	return &FiatShamir[T, PT]{h: poseidon2.NewMDHasher[T, PT](compress)}
}

// Update adds field elements to the transcript.
func (fs *FiatShamir[T, PT]) Update(vec ...T) {
	fs.h.WriteElements(vec...)
}

// UpdateVec adds slices of field elements to the transcript.
func (fs *FiatShamir[T, PT]) UpdateVec(vecs ...[]T) {
	for i := range vecs {
		fs.Update(vecs[i]...)
	}
}

// RandomDigest samples a random octuplet from the current transcript state.
func (fs *FiatShamir[T, PT]) RandomDigest() poseidon2.Octuplet[T] {
	defer fs.safeguardUpdate()
	return fs.h.SumDigest()
}

// RandomDigestFromSeed derives a deterministic octuplet from a seed and a
// name, leaving the transcript state unchanged.
func (fs *FiatShamir[T, PT]) RandomDigestFromSeed(seed poseidon2.Octuplet[T], name string) poseidon2.Octuplet[T] {
	oldState := fs.State()
	defer fs.SetState(oldState)

	fs.SetState(seed)
	nameOctuplet := nameToOctuplet[T, PT](name)
	fs.h.WriteElements(nameOctuplet[:]...)
	return fs.RandomDigest()
}

// nameToOctuplet encodes name into field elements: its blake2b-256 digest is
// split into [poseidon2.BlockSize] big-endian chunks, each reduced mod p.
func nameToOctuplet[T any, PT generic.Element[T]](name string) (res poseidon2.Octuplet[T]) {
	digest := blake2b.Sum256([]byte(name))
	const chunk = len(digest) / poseidon2.BlockSize
	for i := range res {
		PT(&res[i]).SetBytes(digest[chunk*i : chunk*(i+1)])
	}
	return res
}

// RandomManyIntegers samples num integers uniformly in [0, upperBound). The function expects
// upperBound to be a power of two.
func (fs *FiatShamir[T, PT]) RandomManyIntegers(num, upperBound int) []int {
	if !utils.IsPowerOfTwo(upperBound) {
		panic("upperBound must be a power of two")
	}

	var (
		i   = 0
		res = make([]int, num)
	)

	for i < num {
		// take the remainder mod n of each limb
		c := fs.RandomDigest() // already calls safeguardUpdate()
		for j := range c {
			res[i] = int(PT(&c[j]).Uint64() % uint64(upperBound)) //nolint:gosec // upperBound > 0
			i++
			if i >= num {
				break
			}
		}
	}
	return res
}

// SetState replaces the internal hasher state with s.
func (fs *FiatShamir[T, PT]) SetState(s poseidon2.Octuplet[T]) {
	fs.h.SetStateOctuplet(s)
}

// State returns the current internal hasher state.
func (fs *FiatShamir[T, PT]) State() poseidon2.Octuplet[T] {
	return fs.h.GetStateOctuplet()
}

func (fs *FiatShamir[T, PT]) safeguardUpdate() {
	var zero T
	fs.Update(zero)
}
