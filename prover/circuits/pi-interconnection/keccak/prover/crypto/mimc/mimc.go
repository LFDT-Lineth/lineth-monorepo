package mimc

import (
	"hash"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/mimc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
)

// NewMiMC wraps [mimc.NewMiMC], this is used to limit the number of gnark-crypto imports.
func NewMiMC() hash.Hash {
	return mimc.NewMiMC()
}

// Constants collects the MiMC constants parsed as field elements
var Constants []field.Element = func() []field.Element {
	bigConsts := mimc.GetConstants()
	res := make([]field.Element, len(bigConsts))
	for i := range res {
		res[i].SetBigInt(&bigConsts[i])
	}
	return res
}()

// BlockCompression applies the MiMC block compression function to a given block
// over a given state. This what is run under the hood by the MiMC hash function
// in Miyaguchi-Preneel mode.
func BlockCompression(oldState, block field.Element) (newState field.Element) {

	res := block
	var tmp field.Element

	// s <- (s + old + c)^17
	for i := range Constants {
		// We don't use the loop value of Constant to explictly
		// show the linter that we are not mutating the loop value.
		c := Constants[i]
		res.Add(&res, &c)
		res.Add(&res, &oldState)
		tmp.Square(&res)
		tmp.Square(&tmp)
		tmp.Square(&tmp)
		tmp.Square(&tmp)
		res.Mul(&tmp, &res)
	}

	res.Add(&res, &oldState).Add(&res, &oldState).Add(&res, &block)
	return res
}

// GnarkBlockCompression applies the MiMC permutation to a given block within
// a gnark circuit and mirrors exactly [BlockCompression].
func GnarkBlockCompression(api frontend.API, oldState, block frontend.Variable) (newState frontend.Variable) {

	res := block
	var tmp frontend.Variable

	// s <- (s + old + c)^17
	for _, c := range Constants {
		res = api.Add(res, c)
		res = api.Add(res, oldState)
		tmp = api.Mul(res, res)
		tmp = api.Mul(tmp, tmp)
		tmp = api.Mul(tmp, tmp)
		tmp = api.Mul(tmp, tmp)
		res = api.Mul(tmp, res)
	}

	res = api.Add(res, oldState)
	res = api.Add(res, oldState)
	res = api.Add(res, block)

	return res
}

// HashVec hashes a vector of field elements
func HashVec(v []field.Element) (h field.Element) {
	state := NewMiMC()
	for i := range v {
		vBytes := v[i].Bytes()
		state.Write(vBytes[:])
	}
	h.SetBytes(state.Sum(nil))
	return
}

// GnarkHashVec is the gnark equivalent of [HashVec]
func GnarkHashVec(api frontend.API, v []frontend.Variable) (h frontend.Variable) {
	state := frontend.Variable(0)
	for i := range v {
		state = GnarkBlockCompression(api, state, v[i])
	}
	return state
}
