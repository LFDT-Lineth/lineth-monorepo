// Package fiatshamir implements the Fiat-Shamir transcript hashing to build non-interactive protocol.
// Use by instantiating a transcripting, updating it with prover messages and sampling randomness.
package fiatshamir

import (
	"unsafe"

	gfs "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/fiatshamir"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// https://blog.trailofbits.com/2022/04/18/the-frozen-heart-vulnerability-in-plonk/

// FiatShamir is the KoalaBear instance of the generic transcript, extended
// with methods absorbing and sampling degree-6 extension field elements.
// Update, UpdateVec, RandomDigest, RandomManyIntegers, State and SetState are
// promoted from the generic transcript.
type FiatShamir struct {
	*gfs.FiatShamir[field.Element, *field.Element]
}

// NewFiatShamir creates a new FiatShamir transcript accumulator.
func NewFiatShamir() *FiatShamir {
	return &FiatShamir{gfs.New[field.Element, *field.Element](poseidon2.Compress)}
}

// UpdateExt adds extension field elements to the transcript.
func (fs *FiatShamir) UpdateExt(vec ...field.Ext) {
	if len(vec) == 0 {
		return
	}
	vElems := unsafe.Slice((*field.Element)(unsafe.Pointer(&vec[0])), field.ExtensionDegree*len(vec)) //nolint
	fs.Update(vElems...)
}

// UpdateGeneric adds generic field elements to the transcript, dispatching on their type.
func (fs *FiatShamir) UpdateGeneric(vec ...field.Gen) {
	if len(vec) == 0 {
		return
	}

	// Marshal the elements in a vector of bytes
	for _, f := range vec {
		if f.IsBase() {
			fs.Update(f.AsBase())
		} else {
			fs.UpdateExt(f.Ext)
		}
	}
}

// UpdateVecExt updates the Fiat-Shamir state by passing one or more slices of
// extension field elements.
func (fs *FiatShamir) UpdateVecExt(vecs ...[]field.Ext) {
	for i := range vecs {
		fs.UpdateExt(vecs[i]...)
	}
}

// UpdateSV updates the FS state with a smart-vector. No-op if the smart-vector
// has a length of zero.
func (fs *FiatShamir) UpdateSV(sv field.Vec) {
	if sv.Len() == 0 {
		return
	}

	if sv.IsBase() {
		vec := sv.AsBase()
		fs.Update(vec...)
	} else {
		vec := sv.AsExt()
		fs.UpdateExt(vec...)
	}
}

// RandomFext samples a random extension field element from the transcript.
// Uses the first ExtensionDegree (=6) of the 8 octuplet coordinates; the
// remaining two are discarded.
func (fs *FiatShamir) RandomFext() field.Ext {
	return octupletToExt(fs.RandomDigest())
}

// octupletToExt keeps the first ExtensionDegree coordinates of s.
func octupletToExt(s field.Octuplet) field.Ext {
	var res field.Ext
	res.B0.A0 = s[0]
	res.B0.A1 = s[1]
	res.B1.A0 = s[2]
	res.B1.A1 = s[3]
	res.B2.A0 = s[4]
	res.B2.A1 = s[5]
	return res
}

// RandomFextFromSeed derives a deterministic extension field element from a seed and name.
func (fs *FiatShamir) RandomFextFromSeed(seed field.Octuplet, name string) field.Ext {
	return octupletToExt(fs.RandomDigestFromSeed(seed, name))
}
