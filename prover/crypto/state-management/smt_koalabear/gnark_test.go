package smt_koalabear

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"

	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

func randomOctuplet() field.Octuplet {
	var res field.Octuplet
	for i := 0; i < 8; i++ {
		res[i].SetRandom()
	}
	return res
}

func getMerkleProof(t *testing.T) ([]Proof, []field.Octuplet, field.Octuplet) {

	tree := NewEmptyTree()

	// populate the tree
	nbLeaves := 10
	var tmp field.Octuplet
	for i := 0; i < nbLeaves; i++ {
		tmp = randomOctuplet()
		tree.Update(i, tmp)
	}
	nbProofs := 10
	proofs := make([]Proof, nbProofs)
	leafs := make([]field.Octuplet, nbProofs)
	for pos := 0; pos < nbProofs; pos++ {

		leafs[pos], _ = tree.GetLeaf(pos)
		proofs[pos], _ = tree.Prove(pos)

		// Directly verify the proof
		err := Verify(&proofs[pos], leafs[pos], tree.Root)
		if err != nil {
			t.Fatal(err)
		}
	}

	return proofs, leafs, tree.Root
}

type MerkleProofCircuit struct {
	Proofs []GnarkProof                        `gnark:",public"`
	Leafs  []poseidon2_koalabear.GnarkOctuplet `gnark:",public"`
	Root   poseidon2_koalabear.GnarkOctuplet
}

func (circuit *MerkleProofCircuit) Define(api frontend.API) error {

	for i := 0; i < len(circuit.Proofs); i++ {
		GnarkVerifyMerkleProof(api, circuit.Proofs[i], circuit.Leafs[i], circuit.Root)
	}
	return nil
}

func TestMerkleProofGnark(t *testing.T) {

	// generate witness
	proofs, leafs, root := getMerkleProof(t)
	nbProofs := len(proofs)
	var witness MerkleProofCircuit
	witness.Proofs = make([]GnarkProof, nbProofs)
	witness.Leafs = make([]poseidon2_koalabear.GnarkOctuplet, nbProofs)
	for i := 0; i < nbProofs; i++ {
		witness.Proofs[i].Path = proofs[i].Path
		witness.Proofs[i].Siblings = make([]poseidon2_koalabear.GnarkOctuplet, len(proofs[i].Siblings))
		for j := 0; j < len(proofs[i].Siblings); j++ {
			for k := 0; k < 8; k++ {
				witness.Proofs[i].Siblings[j][k] = proofs[i].Siblings[j][k].String()
			}
		}
		for k := 0; k < 8; k++ {
			witness.Leafs[i][k] = leafs[i][k].String()
		}
	}

	for k := 0; k < 8; k++ {
		witness.Root[k] = root[k].String()
	}

	// compile circuit
	var circuit MerkleProofCircuit
	circuit.Proofs = make([]GnarkProof, nbProofs)
	circuit.Leafs = make([]poseidon2_koalabear.GnarkOctuplet, nbProofs)
	for i := 0; i < nbProofs; i++ {
		circuit.Proofs[i].Siblings = make([]poseidon2_koalabear.GnarkOctuplet, len(proofs[i].Siblings))
	}
	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		t.Fatal(err)
	}

	// solve the circuit
	twitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}

}
