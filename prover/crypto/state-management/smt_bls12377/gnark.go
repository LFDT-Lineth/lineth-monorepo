package smt_bls12377

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_bls12377"
)

// GnarkProof mirrors [Proof] in a gnark circuit.
type GnarkProof struct {
	Path     frontend.Variable
	Siblings []frontend.Variable
}

// GnarkRecoverRoot is as [RecoverRoot] in a gnark circuit. The provided
// position is range-checked against the number of siblings in the Merkle proof
// object.
func GnarkRecoverRoot(
	api frontend.API,
	proof GnarkProof,
	leaf frontend.Variable) (frontend.Variable, error) {

	h, err := poseidon2_bls12377.NewGnarkMDHasher(api)
	if err != nil {
		return nil, err
	}

	current := leaf
	nbBits := len(proof.Siblings)

	b := api.ToBinary(proof.Path, nbBits)
	for i := 0; i < len(proof.Siblings); i++ {
		h.Reset()
		left := api.Select(b[i], proof.Siblings[i], current)
		right := api.Select(b[i], current, proof.Siblings[i])

		h.Write(left, right)
		current = h.Sum()
	}

	return current, nil
}

// GnarkVerifyMerkleProof asserts the validity of a [GnarkProof] against a root.
func GnarkVerifyMerkleProof(
	api frontend.API,
	proof GnarkProof,
	leaf frontend.Variable,
	root frontend.Variable) error {

	r, err := GnarkRecoverRoot(api, proof, leaf)
	if err != nil {
		return err
	}

	api.AssertIsEqual(root, r)
	return nil
}
