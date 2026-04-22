// Package fiatshamir implements the Fiat-Shamir transcript hashing to build non-interactive protocol.
// Use by instantiating a transcripting, updating it with prover messages and sampling randomness.
package fiatshamir

import (
	"unsafe"

	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
	"github.com/consensys/linea-monorepo/prover-ray/utils/types"
	"golang.org/x/crypto/blake2b"
)

// https://blog.trailofbits.com/2022/04/18/the-frozen-heart-vulnerability-in-plonk/

// FiatShamir accumulates the transcript of a protocol (e.g. the set of messages
// sent between the prover and the verifier). The accumulated transcripts can
// then be used to sample random coins that are obtained by hashing the
// provided transcript. The hashing is incremental and can be updated at any
// moment.
type FiatShamir struct {
	h *poseidon2.MDHasher
}

// NewFiatShamir creates a new FiatShamir transcript accumulator.
func NewFiatShamir() *FiatShamir {
	return &FiatShamir{h: poseidon2.NewMDHasher()}
}

// Update adds base field elements to the transcript.
func (fs *FiatShamir) Update(vec ...field.Element) {
	fs.h.WriteElements(vec...)
}

// UpdateExt adds extension field elements to the transcript.
func (fs *FiatShamir) UpdateExt(vec ...field.Ext) {
	if len(vec) == 0 {
		return
	}
	vElems := unsafe.Slice((*field.Element)(unsafe.Pointer(&vec[0])), 4*len(vec)) //nolint
	fs.h.WriteElements(vElems...)
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

// UpdateVec adds slices of base field elements to the transcript.
func (fs *FiatShamir) UpdateVec(vecs ...[]field.Element) {
	for i := range vecs {
		fs.Update(vecs[i]...)
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

// RandomField samples a random octuplet from the current transcript state.
func (fs *FiatShamir) RandomField() field.Octuplet {
	defer fs.safeguardUpdate()
	return fs.h.SumElement()
}

// RandomFext samples a random extension field element from the transcript.
func (fs *FiatShamir) RandomFext() field.Ext {
	s := fs.RandomField() // already calls safeguardUpdate()
	var res field.Ext
	res.B0.A0 = s[0]
	res.B0.A1 = s[1]
	res.B1.A0 = s[2]
	res.B1.A1 = s[3]

	return res
}

// RandomFieldFromSeed derives a deterministic extension field element from a seed and name.
func (fs *FiatShamir) RandomFieldFromSeed(seed field.Octuplet, name string) field.Ext {

	// The first step encodes the 'name' into a single field element. The
	// field element is obtained by hashing and taking the modulo of the
	// result to fit into a field element.
	nameBytes := []byte(name)
	hasher, _ := blake2b.New256(nil)
	hasher.Write(nameBytes)
	nameBytes = hasher.Sum(nil)
	nameOctuplet := types.BytesToKoalaOctupletLoose(nameBytes)

	// The seed is then obtained by calling the compression function over
	// the seed and the encoded name.
	oldState := fs.State()
	defer fs.SetState(oldState)

	fs.SetState(seed)
	fs.h.WriteElements(nameOctuplet[:]...)
	res := fs.RandomFext()
	return res
}

// RandomManyIntegers samples num integers uniformly in [0, upperBound). The function expects
// upperBound to be a power of two.
func (fs *FiatShamir) RandomManyIntegers(num, upperBound int) []int {

	if !utils.IsPowerOfTwo(upperBound) {
		panic("upperBound must be a power of two")
	}

	var (
		i   = 0
		res = make([]int, num)
	)

	for i < num {
		// take the remainder mod n of each limb
		c := fs.RandomField() // already calls safeguardUpdate()
		for j := 0; j < 8; j++ {
			b := c[j].Bits()
			res[i] = int(b[0]) % upperBound
			i++
			if i >= num {
				break
			}
		}
	}
	return res
}

// SetState replaces the internal hasher state with s.
func (fs *FiatShamir) SetState(s field.Octuplet) {
	fs.h.SetStateOctuplet(s)
}

// State returns the current internal hasher state.
func (fs *FiatShamir) State() field.Octuplet {
	return fs.h.GetStateOctuplet()
}

func (fs *FiatShamir) safeguardUpdate() {
	fs.Update(field.Zero())
}
