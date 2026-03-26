package smt_koalabear

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
)

// GnarkProof mirrors [Proof] in a gnark circuit.
type GnarkProof struct {
	Path     frontend.Variable
	Siblings []poseidon2_koalabear.GnarkOctuplet
}

// selectOcuplet if b=1, returns l else return r
func selectOcuplet(api frontend.API, b frontend.Variable, l, r poseidon2_koalabear.GnarkOctuplet) poseidon2_koalabear.GnarkOctuplet {
	var res poseidon2_koalabear.GnarkOctuplet
	for i := 0; i < 8; i++ {
		res[i] = api.Select(b, l[i], r[i])
	}
	return res
}

// GnarkRecoverRoot computes the root form the proof and the leaf
func GnarkRecoverRoot(
	api frontend.API,
	proof GnarkProof,
	leaf poseidon2_koalabear.GnarkOctuplet) (poseidon2_koalabear.GnarkOctuplet, error) {

	h, err := poseidon2_koalabear.NewGnarkMDHasher(api)
	if err != nil {
		return poseidon2_koalabear.GnarkOctuplet{}, err
	}

	current := leaf
	nbBits := len(proof.Siblings)
	b := api.ToBinary(proof.Path, nbBits)
	for i := 0; i < len(proof.Siblings); i++ {
		h.Reset()
		left := selectOcuplet(api, b[i], proof.Siblings[i], current)
		right := selectOcuplet(api, b[i], current, proof.Siblings[i])
		h.WriteOctuplet(left, right)
		current = h.Sum()
	}

	return current, nil
}

// GnarkVerifyMerkleProof asserts the validity of a [GnarkProof] against a root.
func GnarkVerifyMerkleProof(
	api frontend.API,
	proof GnarkProof,
	leaf poseidon2_koalabear.GnarkOctuplet,
	root poseidon2_koalabear.GnarkOctuplet) error {

	r, err := GnarkRecoverRoot(api, proof, leaf)
	if err != nil {
		return err
	}

	for i := 0; i < 8; i++ {
		api.AssertIsEqual(r[i], root[i])
	}
	return nil
}
