package smtkoalabear

import (
	"github.com/consensys/gnark/frontend"
	poseidon2 "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poseidon2"
)

// GnarkProof mirrors [Proof] in a gnark circuit.
type GnarkProof struct {
	Path     frontend.Variable
	Siblings []poseidon2.GnarkOctuplet
}

// selectOcuplet if b=1, returns l else return r
func selectOcuplet(api frontend.API, b frontend.Variable, l, r poseidon2.GnarkOctuplet) poseidon2.GnarkOctuplet {
	var res poseidon2.GnarkOctuplet
	for i := 0; i < 8; i++ {
		res[i] = api.Select(b, l[i], r[i])
	}
	return res
}

// GnarkRecoverRoot computes the root form the proof and the leaf
func GnarkRecoverRoot(
	api frontend.API,
	proof GnarkProof,
	leaf poseidon2.GnarkOctuplet) (poseidon2.GnarkOctuplet, error) {

	h, err := poseidon2.NewGnarkMDHasher(api)
	if err != nil {
		return poseidon2.GnarkOctuplet{}, err
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
	leaf poseidon2.GnarkOctuplet,
	root poseidon2.GnarkOctuplet) error {

	r, err := GnarkRecoverRoot(api, proof, leaf)
	if err != nil {
		return err
	}

	for i := 0; i < 8; i++ {
		api.AssertIsEqual(r[i], root[i])
	}
	return nil
}
