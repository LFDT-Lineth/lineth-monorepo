package multisethashing_koalabear

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
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
	//		q = 2**31 for koalabear
	//		// max_number is the maximal number of parts we can have in the hash
	//		// We are looking for a SIS instance that is secure for an L1 norm
	//		// of at most that. We use the bound L1(x) > L2(x), to reduce that
	//		// to finding a SIS instance that is secure for the L2 norm.
	//		max_number = 2**12 (max number of insert/remove)
	//		// an arbitrary big number to ensure the BKZ solver will find the
	//		// optimal value to attack.
	//		m = 2**20
	//
	// 		params = SISParameters(n=23*8, q=q, length_bound=max_number, m=m, norm=2, tag='MSET')
	// 		SIS.estimate(params)
	//		--------------------
	//		>>> lattice  :: rop: ≈2^127.7, red: ≈2^127.7, δ: 1.004382, β: 348, d: 950, tag: euclidean
	// ````
	//		// q = 2**(31*8)
	//		// max_number = 2**12
	// 		// m = 2**20
	// 		// n = 23
	//
	// 		params = SISParameters(n=23, q=2**(31*8), length_bound=2**12, m=2**20, norm=2, tag='MSET')
	// 		// SIS.estimate(params)
	// 		>>>lattice:: rop: ≈2^127.7, red: ≈2^127.7, δ: 1.004382, β: 348, d: 950, tag: euclidean
	//
	ChunkSize    = 23
	BlockSize    = 8
	MSetHashSize = ChunkSize * BlockSize
)

// MSetHash represents a multisets hash (LtHash) instantiated using the MiMC
// hash function. The zero value of this type is a valid multisets hash for
// the empty set.
type MSetHash [MSetHashSize]field.Element

// MSetHashGnark is a multisets hash (LtHash) instantiated using Poseidon2.
// The zero value of this type is a valid multisets hash for the empty set.
type MSetHashGnark struct {
	Inner  [MSetHashSize]frontend.Variable
	hasher poseidon2_koalabear.GnarkKoalaHasher
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

	var (
		hsh          = poseidon2_koalabear.NewMDHasher()
		zeroOctuplet = field.Octuplet{}
		state        = field.Octuplet{}
	)
	if len(msgs) == 0 {
		panic("got provided an empty message")
	}

	hsh.WriteElements(msgs...)

	for i := 0; i < ChunkSize; i++ {

		state = hsh.SumElement()
		if rem {
			for j := 0; j < BlockSize; j++ {
				m[i*BlockSize+j].Sub(&m[i*BlockSize+j], &state[j])
			}
		} else {
			for j := 0; j < BlockSize; j++ {
				m[i*BlockSize+j].Add(&m[i*BlockSize+j], &state[j])
			}
		}
		if i < ChunkSize-1 {
			hsh.WriteElements(zeroOctuplet[:]...)

		}
	}
}

// EmptyMSetHashGnark returns an empty multisets hash pre-initialized with 0s.
// Use that instead of `MSetHashGnark{}`
func EmptyMSetHashGnark(hasher poseidon2_koalabear.GnarkKoalaHasher) MSetHashGnark {
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

	var hasher poseidon2_koalabear.GnarkKoalaHasher

	if len(msgs) == 0 {
		panic("got provided an empty message")
	}

	if m.hasher == nil {
		hasherPoseidon2, _ := poseidon2_koalabear.NewGnarkMDHasher(api)
		hasher = &hasherPoseidon2
	} else {
		hasher = m.hasher
	}

	hasher.Reset()
	defer hasher.Reset()

	// This populates the hasher's state with the message.
	// Flatten the octuplets into individual variables for Write

	hasher.Write(msgs...)

	// This squeezes the mset row of the element
	for i := 0; i < ChunkSize; i++ {

		tmp := hasher.Sum()
		if rem {
			for j := 0; j < BlockSize; j++ {
				m.Inner[i*BlockSize+j] = api.Sub(m.Inner[i*BlockSize+j], tmp[j])
			}
		} else {
			for j := 0; j < BlockSize; j++ {
				m.Inner[i*BlockSize+j] = api.Add(m.Inner[i*BlockSize+j], tmp[j])
			}
		}

		// This updates the state so that we get a different value post-update.
		// Write 8 zeros to match native version's CompressPoseidon2(state, field.Octuplet{})
		if i < ChunkSize-1 {
			hasher.Write(0, 0, 0, 0, 0, 0, 0, 0)
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
func MsetOfSingletonGnark(api frontend.API, hasher poseidon2_koalabear.GnarkKoalaHasher, msg ...frontend.Variable) MSetHashGnark {
	m := EmptyMSetHashGnark(hasher)
	m.update(api, false, msg)
	return m
}
