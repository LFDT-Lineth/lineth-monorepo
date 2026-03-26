package fiatshamir

import (
	"math"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash"
	locmimc "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"golang.org/x/crypto/blake2b"
)

// GnarkFiatShamir mirrors [State] in a gnark circuit. It provides analogous
// methods for every of [State]'s method and works over [frontend.Variable]
// instead of [field.Element].
//
// This implementation design eases the task of writing a gnark circuit version
// of the verifier of a protocol calling [State] as it allows having a very
// similar code for both tasks.
type GnarkFiatShamir struct {
	hasher hash.StateStorer
	// pointer to the gnark-API (also passed to the hasher but behind an
	// interface). This is needed to perform bit-decomposition.
	api frontend.API
}

// NewGnarkFiatShamir creates a [GnarkFiatShamir] object. The function accepts
// an optional [gkrmimc.HasherFactory] object as input. This is expected to be
// used in the scope of a [frontend.Define] function.
func NewGnarkFiatShamir(api frontend.API, factory locmimc.HasherFactory) *GnarkFiatShamir {

	if factory == nil {
		factory = &locmimc.BasicHasherFactory{Api: api}
	}

	hasher := factory.NewHasher()

	return &GnarkFiatShamir{
		hasher: hasher,
		api:    api,
	}
}

// SetState mutates the fiat-shamir state of
func (fs *GnarkFiatShamir) SetState(state []frontend.Variable) {

	switch hsh := fs.hasher.(type) {
	case interface {
		SetState([]frontend.Variable) error
	}:
		fs.hasher.Reset()
		if err := hsh.SetState(state); err != nil {
			panic(err)
		}
	default:
		panic("unexpected hasher type")
	}
}

// State mutates returns the state of the fiat-shamir hasher. The
// function will also updates its own state with unprocessed inputs.
func (fs *GnarkFiatShamir) State() []frontend.Variable {

	switch hsh := fs.hasher.(type) {
	case interface {
		State() []frontend.Variable
	}:
		return hsh.State()
	default:
		panic("unexpected hasher type")
	}
}

// Update updates the Fiat-Shamir state with a vector of frontend.Variable
// representing field element each.
func (fs *GnarkFiatShamir) Update(vec ...frontend.Variable) {
	// Safeguard against nil
	for _, x := range vec {
		if x == nil {
			panic("gnark fiat-shamir updated with a nil frontend variable")
		}
	}
	fs.hasher.Write(vec...)
}

// UpdateVec updates the Fiat-Shamir state with a matrix of field element.
func (fs *GnarkFiatShamir) UpdateVec(mat ...[]frontend.Variable) {
	for i := range mat {
		fs.Update(mat[i]...)
	}
}

// RandomField returns a single valued fiat-shamir hash
func (fs *GnarkFiatShamir) RandomField() frontend.Variable {
	defer fs.safeguardUpdate()
	return fs.hasher.Sum()
}

// RandomManyIntegers returns a vector of variable that will contain small integers
func (fs *GnarkFiatShamir) RandomManyIntegers(num, upperBound int) []frontend.Variable {

	// Even `1` would be wierd, there would be only one acceptable coin value.
	if upperBound < 1 {
		utils.Panic("UpperBound was %v", upperBound)
	}

	if !utils.IsPowerOfTwo(upperBound) {
		utils.Panic("Expected a power of two but got %v", upperBound)
	}

	if num == 0 {
		return []frontend.Variable{}
	}

	defer fs.safeguardUpdate()

	var (
		// Compute the number of bytes to generate the challenges in bits
		challsBitSize = int(math.Ceil(math.Log2(float64(upperBound))))
		// Number of challenges computable with one call to hash (implicitly, the
		// the division is rounded-down)
		maxNumChallsPerDigest = (field.Bits - 1) / int(challsBitSize)
		// res stores the function result
		res = make([]frontend.Variable, 0, num)
		// challCount stores the number of generated small integers
		challCount = 0
	)

	for {
		digest := fs.hasher.Sum()
		digestBits := fs.api.ToBinary(digest)

		for i := 0; i < maxNumChallsPerDigest; i++ {
			// Stopping condition, we computed enough challenges
			if challCount >= num {
				return res
			}

			// Drains the first `challsBitSize` of the digestBits into
			// a new challenge to be returned.
			newChall := fs.api.FromBinary(digestBits[:challsBitSize]...)
			digestBits = digestBits[challsBitSize:]
			res = append(res, newChall)
			challCount++
		}

		/*
			This test prevents reupdating fs just before returning
			and thus, safeguardupdating once more because of the defer.
		*/
		if challCount >= num {
			return res
		}

		fs.safeguardUpdate()
	}
}

// RandomFieldFromSeed generates a new field element from the given seed
// and a name. The 'fs' is left unchanged by the call (aside from the
// underlying [frontend.API]).
func (fs *GnarkFiatShamir) RandomFieldFromSeed(seed frontend.Variable, name string) frontend.Variable {

	// The first step encodes the 'name' into a single field element. The
	// field element is obtained by hashing and taking the modulo of the
	// result to fit into a field element.
	nameBytes := []byte(name)
	hasher, _ := blake2b.New256(nil)
	hasher.Write(nameBytes)
	nameBytes = hasher.Sum(nil)
	nameField := new(field.Element).SetBytes(nameBytes)

	// The seed is then obtained by calling the compression function over
	// the seed and the encoded name.
	oldState := fs.State()
	defer fs.SetState(oldState)

	fs.SetState([]frontend.Variable{seed})
	fs.hasher.Write(nameField)

	return fs.hasher.Sum()
}

// safeguardUpdate updates the state as a safeguard by appending a field element
// representing a "0". This is used every time a field element is consumed from
// the hasher to ensure that the next field element will have a different
// value.
func (fs *GnarkFiatShamir) safeguardUpdate() {
	fs.Update(0)
}
