package fiatshamir

import (
	"math"

	"github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// State holds a Fiat-Shamir state. The Fiat-Shamir state can be updated by
// providing field elements and can be consumed to generate either field
// elements or sequences of small integers. The Fiat-Shamir instantiation relies
// on the MiMC hash function and uses the following strategy for generating
// random public coins:
//
//   - The messages are appended to the hasher as it is received by the Fiat-
//     Shamir state, field element by field element
//   - When requested by the caller, the State can generates a field element by
//     hashing the transcript. Recall that we are using a SNARK-friendly hash
//     function and that its output should be interpreted as a field element.
//   - Everytime a field element has been generated, the hasher is updated by
//     appending an artificial message: field.Element(0). This is to ensure
//     that all generated field elements are independants.
//
// To be safely used, the Fiat-Shamir State should initialized by passing an
// initial field element summarizing the protocol in its full-extent. This is
// to prevent rogue protocol attack as in the Frozen Heart vulnerability.
//
// https://blog.trailofbits.com/2022/04/18/the-frozen-heart-vulnerability-in-plonk/
type State struct {
	hasher           hash.StateStorer
	TranscriptSize   int
	NumCoinGenerated int
}

// NewMiMCFiatShamir constructs a fresh and empty Fiat-Shamir state.
func NewMiMCFiatShamir() *State {
	return &State{
		hasher: mimc.NewMiMC().(hash.StateStorer),
	}
}

// State returns the internal state of the Fiat-Shamir hasher. Only works for
// MiMC.
func (s *State) State() []field.Element {
	_ = s.hasher.Sum(nil)
	b := s.hasher.State()
	f := new(field.Element).SetBytes(b)
	return []field.Element{*f}
}

// SetState sets the fiat-shamir state to the requested value
func (s *State) SetState(f []field.Element) {
	_ = s.hasher.Sum(nil)
	b := f[0].Bytes()
	s.hasher.SetState(b[:])
}

// Update the Fiat-Shamir state with a one or more of field elements. The
// function as no-op if the caller supplies no field elements.
func (fs *State) Update(vec ...field.Element) {
	if len(vec) == 0 {
		return
	}

	// Marshal the elements in a vector of bytes
	for _, f := range vec {
		bytes := f.Bytes()
		_, err := fs.hasher.Write(bytes[:])
		if err != nil {
			// This normally happens if the bytes that we provide do not represent
			// a field element. In our case, the bytes are computed by ourselves
			// from the caller's field element so the error is not possible. Hence,
			// the assertion.
			panic("Hashing is not supposed to fail")
		}
	}

	// Increase the transcript counter
	fs.TranscriptSize += len(vec)
}

// UpdateVec updates the Fiat-Shamir state by passing one of more slices of
// field elements.
func (fs *State) UpdateVec(vecs ...[]field.Element) {
	if len(vecs) == 0 {
		return
	}

	for i := range vecs {
		fs.Update(vecs[i]...)
	}
}

// UpdateSV updates the FS state with a smart-vector. No-op if the smart-vector
// has a length of zero.
func (fs *State) UpdateSV(sv smartvectors.SmartVector) {
	if sv.Len() == 0 {
		return
	}

	vec := make([]field.Element, sv.Len())
	sv.WriteInSlice(vec)
	fs.Update(vec...)
}

// RandomField generates and returns a single field element from the Fiat-Shamir
// transcript.
func (fs *State) RandomField() field.Element {
	defer fs.safeguardUpdate()
	challBytes := fs.hasher.Sum(nil)
	var res field.Element
	res.SetBytes(challBytes)

	// increase the counter by one
	fs.NumCoinGenerated++
	return res
}

// RandomManyIntegers returns a list of challenge small integers. That is, a
// list of positive integer bounded by `upperBound`. The upperBound is strict
// and is restricted to being only be a power of two.
//
// The function will panic if the coin cannot be generated. We motivate this
// behaviour by the fact that if it happens, this will always be for
// "deterministic" reasons pertaining to the description of the user's protocol
// and never because of the values that are in the transcript itself.
//
// The function is implemented by, first generating random field elements, and
// then breaking each of them down separately into several small integers. The
// "remainder" bits (nameely, the bits of the generated field element that we
// could not pack into a small integer) are thrown away.
//
// If the caller provides num=0, the function no-ops after doing its
// sanity-checks although the call makes no possible sense.
func (fs *State) RandomManyIntegers(num, upperBound int) []int {

	// Even `1` would be wierd, there would be only one acceptable coin value.
	if upperBound < 1 {
		utils.Panic("UpperBound was %v", upperBound)
	}

	if !utils.IsPowerOfTwo(upperBound) {
		utils.Panic("Expected a power of two but got %v", upperBound)
	}

	if num == 0 {
		return []int{}
	}

	defer fs.safeguardUpdate()

	var (
		// challsBitSize stores the number of bits required instantiate each
		// small integer.
		challsBitSize = math.Ceil(math.Log2(float64(upperBound)))
		// Number of challenges computable with one call to hash (implicitly,
		// the division is rounded down). The "-1" corresponds to the fact that
		// the most significant bit of a field element cannot be assumed to
		// contain exactly 1 bit of entropy since the modulus of the field is
		// never a power of 2.
		maxNumChallsPerDigest = (field.Bits - 1) / int(challsBitSize)
		// res stores the preallocated result slice, to which all generated
		// small integers will be appended.
		res = make([]int, 0, num)
	)

	for {
		digest := fs.hasher.Sum(nil)
		buffer := NewBitReader(digest, field.Bits-1)

		// Increase the counter
		fs.NumCoinGenerated++

		for i := 0; i < maxNumChallsPerDigest; i++ {
			// Stopping condition, we computed enough challenges
			if len(res) >= num {
				return res
			}

			newChall, err := buffer.ReadInt(int(challsBitSize))
			if err != nil {
				utils.Panic("could not instantiate the buffer for a single field element")
			}
			res = append(res, int(newChall)%upperBound)
		}

		// This is guarded by the condition to prevent the [State] updating
		// twice in a row when exiting the function. Recall that we have a
		// defer of the safeguard update in the function. This handles the
		// edge-case where the number of requested field elements is a multiple
		// of the number of challenges we can generate with a single field
		// element.
		if len(res) >= num {
			return res
		}

		// This updates ensures that for the next iterations of the loop and the
		// next randomness comsumption uses a fresh randomness.
		fs.safeguardUpdate()
	}
}

// safeguardUpdate updates the state as a safeguard. This way, we are guaranteed
// that successive random oracle queries will yield a different, independant
// result.
//
// This is implemented by adding a 0 in the transcript.
func (fs *State) safeguardUpdate() {
	fs.Update(field.NewElement(0))
}
