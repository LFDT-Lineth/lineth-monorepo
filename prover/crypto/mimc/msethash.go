package mimc

import (
	"github.com/consensys/gnark/frontend"
	ghash "github.com/consensys/gnark/std/hash"
	gmimc "github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

const (
	// MSetHashSize has been obtained to guarantee security when the number of
	// inputs do not exceed 2**12. It was obtained using lattice-estimator,
	// guaranteeing at least 128 bits of security.
	//
	// ```
	// 		from estimator import *
	// 		from estimator.sis_parameters import *
	//
	//		// The modulus of the field (approximatively)
	//		q = 2**252
	//		// max_number is the maximal number of parts we can have in the hash
	//		// We are looking for a SIS instance that is secure for an L1 norm
	//		// of at most that. We use the bound L1(x) > L2(x), to reduce that
	//		// to finding a SIS instance that is secure for the L2 norm.
	//		max_number = 2**12
	//		// an arbitrary big number to ensure the BKZ solver will find the
	//		// optimal value to attack.
	//		m = 2**20
	//
	// 		params = SISParameters(n=23, q=q, length_bound=max_number, m=m, norm=2, tag='MSET')
	// 		SIS.estimate(params)
	//		--------------------
	//		>>> lattice  :: rop: ≈2^129.9, red: ≈2^129.9, δ: 1.004315, β: 356, d: 966, tag: euclidean
	// ````
	MSetHashSize = 23
)

// MSetHash represents a multisets hash (LtHash) instantiated using the MiMC
// hash function. The zero value of this type is a valid multisets hash for
// the empty set.
type MSetHash [MSetHashSize]field.Element

// MSetHashGnark is a multisets hash (LtHash) instantiated using the MiMC
// hash function. The zero value of this type is a valid multisets hash for
// the empty set.
type MSetHashGnark struct {
	Inner  [MSetHashSize]frontend.Variable
	hasher ghash.StateStorer
}

// Insert adds the given messages to the multisets hash. The message can be an
// array of field elements of any size. The function panics if given an empty
// msg.
func (m *MSetHash) Insert(msgs ...field.Element) {
	m.update(false, msgs...)
}

// Remove removes the given messages from the multisets hash. The message can be
// an array of field elements of any size. The function panics if given an empty
// msg.
func (m *MSetHash) Remove(msgs ...field.Element) {
	m.update(true, msgs...)
}

// Add combines the two multisets hashes into a single multisets hash.
func (m *MSetHash) Add(other MSetHash) {
	for i := 0; i < MSetHashSize; i++ {
		m[i].Add(&m[i], &other[i])
	}
}

// Sub substracts the multiset "other" from "m"
func (m *MSetHash) Sub(other MSetHash) {
	for i := 0; i < MSetHashSize; i++ {
		m[i].Sub(&m[i], &other[i])
	}
}

// IsEmpty returns true if the MSetHash is empty.
func (m *MSetHash) IsEmpty() bool {
	for i := 0; i < MSetHashSize; i++ {
		if !m[i].IsZero() {
			return false
		}
	}
	return true
}

// update adds or removes an element from the multisets hash.
func (m *MSetHash) update(rem bool, msgs ...field.Element) {

	var state field.Element

	if len(msgs) == 0 {
		panic("got provided an empty message")
	}

	for _, msg := range msgs {
		state = BlockCompression(state, msg)
	}

	for i := 0; i < MSetHashSize; i++ {
		if i > 0 {
			state = BlockCompression(state, field.Zero())
		}

		if rem {
			m[i].Sub(&m[i], &state)
		} else {
			m[i].Add(&m[i], &state)
		}
	}
}

// EmptyMSetHashGnark returns an empty multisets hash pre-initialized with 0s.
// Use that instead of `MSetHashGnark{}`
func EmptyMSetHashGnark(hasher ghash.StateStorer) MSetHashGnark {
	res := MSetHashGnark{
		hasher: hasher,
	}
	for i := range res.Inner {
		res.Inner[i] = 0
	}
	return res
}

// updateGnark updates the multisets hash using the gnark library.
func (m *MSetHashGnark) update(api frontend.API, rem bool, msgs []frontend.Variable) {

	var hasher ghash.StateStorer

	if len(msgs) == 0 {
		panic("got provided an empty message")
	}

	if m.hasher == nil {
		hasherMimc, _ := gmimc.NewMiMC(api)
		hasher = &hasherMimc
	}

	if m.hasher != nil {
		hasher = m.hasher
	}

	hasher.Reset()
	defer hasher.Reset()

	// This populates the hasher's state with the message.
	hasher.Write(msgs...)

	// This squeezes the mset row of the element
	for i := 0; i < MSetHashSize; i++ {

		tmp := hasher.State()[0]
		if rem {
			m.Inner[i] = api.Sub(m.Inner[i], tmp)
		} else {
			m.Inner[i] = api.Add(m.Inner[i], tmp)
		}

		// This updates the state so that we get a different value post-update.
		if i < MSetHashSize-1 {
			hasher.Write(0)
		}
	}
}

// Insert adds the given messages to the multisets hash. The message can be an
// array of field elements of any size. The function panics if given an empty
// msg.
func (m *MSetHashGnark) Insert(api frontend.API, msgs ...frontend.Variable) {
	m.update(api, false, msgs)
}

// Remove removes the given messages from the multisets hash. The message can be
// an array of field elements of any size. The function panics if given an empty
// msg.
func (m *MSetHashGnark) Remove(api frontend.API, msgs ...frontend.Variable) {
	m.update(api, true, msgs)
}

// Add combines the two multisets hashes into a single multisets hash.
func (m *MSetHashGnark) Add(api frontend.API, other MSetHashGnark) {
	for i := 0; i < MSetHashSize; i++ {
		m.Inner[i] = api.Add(m.Inner[i], other.Inner[i])
	}
}

// AddRaw adds in a sequence of value representing a multisets hash
func (m *MSetHashGnark) AddRaw(api frontend.API, other []frontend.Variable) {

	if len(m.Inner) != len(other) {
		panic("MSetHashGnark.AddRaw: lengths of multisets hashes are different")
	}

	for i := 0; i < MSetHashSize; i++ {
		m.Inner[i] = api.Add(m.Inner[i], other[i])
	}
}

// Sub substracts the multiset "other" from "m"
func (m *MSetHashGnark) Sub(api frontend.API, other MSetHashGnark) {
	for i := 0; i < MSetHashSize; i++ {
		m.Inner[i] = api.Sub(m.Inner[i], other.Inner[i])
	}
}

// AssertEqual asserts that the multisets hashes are equal.
func (m *MSetHashGnark) AssertEqual(api frontend.API, other MSetHashGnark) {
	for i := 0; i < MSetHashSize; i++ {
		api.AssertIsEqual(m.Inner[i], other.Inner[i])
	}
}

// AssertEqualRaw asserts that the multisets values are equal to the provided
// array.
func (m *MSetHashGnark) AssertEqualRaw(api frontend.API, other []frontend.Variable) {

	if len(m.Inner) != len(other) {
		panic("MSetHashGnark.AssertEqualRaw: lengths of multisets hashes are different")
	}

	for i := 0; i < MSetHashSize; i++ {
		api.AssertIsEqual(m.Inner[i], other[i])
	}
}

// MsetOfSingletonGnark returns the multiset vector of an entry. nil can be
// passed to the hasher to tell the function to explicitly compute the hash
// in circuit.
func MsetOfSingletonGnark(api frontend.API, hasher ghash.StateStorer, msg ...frontend.Variable) MSetHashGnark {
	m := EmptyMSetHashGnark(hasher)
	m.update(api, false, msg)
	return m
}
