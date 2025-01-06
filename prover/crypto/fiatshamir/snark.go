package fiatshamir

import (
	"math"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc/gkrmimc"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
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
func NewGnarkFiatShamir(api frontend.API, factory *gkrmimc.HasherFactory) *GnarkFiatShamir {

	var hasher hash.StateStorer
	if factory != nil {
		h := factory.NewHasher()
		hasher = h
	} else {
		h, err := mimc.NewMiMC(api)
		if err != nil {
			// There is no real case where this can happen. The only case I
			// can think of is when the function is called outside of the scope
			// of a Define function and `api == nil` but then, there is no way
			// the user can do anything useful with this function anyway.
			panic(err)
		}
		hasher = &h
	}

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
		if err := hsh.SetState(state); err != nil {
			panic(err)
		}
	default:
		panic("unexpected hasher type")
	}
}

// State mutates the fiat-shamir state of
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

// safeguardUpdate updates the state as a safeguard by appending a field element
// representing a "0". This is used every time a field element is consumed from
// the hasher to ensure that the next field element will have a different
// value.
func (fs *GnarkFiatShamir) safeguardUpdate() {
	fs.Update(0)
}
