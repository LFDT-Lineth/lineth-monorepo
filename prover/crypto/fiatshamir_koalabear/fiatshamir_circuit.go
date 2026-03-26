package fiatshamir_koalabear

import (
	"math/bits"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/hasherfactory_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"golang.org/x/crypto/blake2b"
)

type GnarkFSWV struct {
	hasher poseidon2_koalabear.GnarkKoalaHasher
	// pointer to the gnark-API (also passed to the hasher but behind an
	// interface). This is needed to perform bit-decomposition.
	api frontend.API
}

func NewGnarkFSWV(api frontend.API) *GnarkFSWV {
	hasher, _ := poseidon2_koalabear.NewGnarkMDHasher(api)
	return &GnarkFSWV{
		hasher: &hasher,
		api:    api,
	}
}

func NewGnarkFSFromFactory(api frontend.API, factory hasherfactory_koalabear.HasherFactory) *GnarkFSWV {
	hasher := factory.NewHasher()
	return &GnarkFSWV{
		hasher: hasher,
		api:    api,
	}
}

// Update updates the Fiat-Shamir state with a vector of frontend.Variable
// representing field element each.
func (fs *GnarkFSWV) Update(vec ...koalagnark.Element) {
	_vec := wvTofv(vec)
	fs.hasher.Write(_vec...)
}

func (fs *GnarkFSWV) UpdateExt(vec ...koalagnark.Ext) {
	for i := 0; i < len(vec); i++ {
		fs.hasher.Write(vec[i].B0.A0.Native())
		fs.hasher.Write(vec[i].B0.A1.Native())
		fs.hasher.Write(vec[i].B1.A0.Native())
		fs.hasher.Write(vec[i].B1.A1.Native())
	}
}

func wvTofv(v []koalagnark.Element) []frontend.Variable {
	buf := make([]frontend.Variable, len(v))
	for i := 0; i < len(v); i++ {
		buf[i] = v[i].Native()
	}
	return buf
}

func octupletToZkoctuplet(v poseidon2_koalabear.GnarkOctuplet) koalagnark.Octuplet {
	var res koalagnark.Octuplet
	for i := 0; i < 8; i++ {
		res[i] = koalagnark.WrapFrontendVariable(v[i])
	}
	return res
}

func zkoctupletTooctuplet(v koalagnark.Octuplet) poseidon2_koalabear.GnarkOctuplet {
	var res poseidon2_koalabear.GnarkOctuplet
	for i := 0; i < 8; i++ {
		res[i] = v[i].Native()
	}
	return res
}

// UpdateVec updates the Fiat-Shamir state with a matrix of field element.
func (fs *GnarkFSWV) UpdateVec(mat ...[]koalagnark.Element) {
	for i := range mat {
		fs.Update(mat[i]...)
	}
}

func (fs *GnarkFSWV) RandomField() koalagnark.Octuplet {
	res := fs.hasher.Sum()
	defer fs.safeguardUpdate()
	return octupletToZkoctuplet(res)
}

func (fs *GnarkFSWV) randomFieldNative() poseidon2_koalabear.GnarkOctuplet {
	defer fs.safeguardUpdate()
	return fs.hasher.Sum()
}

// RandomField returns a single valued fiat-shamir hash
func (fs *GnarkFSWV) RandomFieldExt() koalagnark.Ext {
	s := fs.RandomField() // already calls safeguardUpdate()
	var res koalagnark.Ext
	res.B0.A0 = s[0]
	res.B0.A1 = s[1]
	res.B1.A0 = s[2]
	res.B1.A1 = s[3]

	return res
}

func (fs *GnarkFSWV) RandomManyIntegers(num, upperBound int) []frontend.Variable {
	n := utils.NextPowerOfTwo(upperBound)
	nbBits := bits.TrailingZeros(uint(n))
	i := 0
	res := make([]frontend.Variable, num)
	for i < num {
		// thake the remainder mod n of each limb
		c := fs.randomFieldNative() // already calls safeguardUpdate()
		for j := 0; j < 8; j++ {
			b := fs.api.ToBinary(c[j])
			res[i] = fs.api.FromBinary(b[:nbBits]...)
			i++
			if i >= num {
				break
			}
		}
	}
	return res
}

func (fs *GnarkFSWV) RandomFieldFromSeed(seed koalagnark.Octuplet, name string) koalagnark.Ext {

	// The first step encodes the 'name' into a single field element. The
	// field element is obtained by hashing and taking the modulo of the
	// result to fit into a field element.
	nameBytes := []byte(name)
	hasher, _ := blake2b.New256(nil)
	hasher.Write(nameBytes)
	nameBytes = hasher.Sum(nil)
	nameOctuplet := types.BytesToKoalaOctupletLoose(nameBytes)
	var nameGnarkOctuplet [8]frontend.Variable
	for i := 0; i < 8; i++ {
		nameGnarkOctuplet[i] = nameOctuplet[i]
	}

	// The seed is then obtained by calling the compression function over
	// the seed and the encoded name.
	oldState := fs.State()
	defer fs.SetState(oldState)

	fs.SetState(seed)
	fs.hasher.Write(nameGnarkOctuplet[:]...)

	res := fs.RandomFieldExt()
	return res
}

// SetState mutates the fiat-shamir state of
func (fs *GnarkFSWV) SetState(state koalagnark.Octuplet) {
	fs.hasher.Reset()
	_state := zkoctupletTooctuplet(state)
	fs.hasher.SetState(_state)
}

// State mutates returns the state of the fiat-shamir hasher. The
// function will also updates its own state with unprocessed inputs.
func (fs *GnarkFSWV) State() koalagnark.Octuplet {
	return octupletToZkoctuplet(fs.hasher.State())
}

// safeguardUpdate updates the state as a safeguard by appending a field element
// representing a "0". This is used every time a field element is consumed from
// the hasher to ensure that the next field element will have a different
// value.
func (fs *GnarkFSWV) safeguardUpdate() {
	fs.hasher.Write(0)
}
