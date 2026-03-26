package smt_bls12377

import (
	"errors"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	_ "github.com/consensys/gnark/std/hash/all" // Register all hash function getters
	"github.com/consensys/linea-monorepo/prover/crypto/encoding"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// ------------------------------------------------------------------------------
// Test where the leaves are bls12377 elmts
func getMerkleProof(t *testing.T) ([]Proof, []fr.Element, fr.Element) {

	depth := 40
	tree := NewEmptyTree(depth)

	// populate the tree
	nbLeaves := 10
	var tmp fr.Element
	for i := 0; i < nbLeaves; i++ {
		tmp.SetRandom()
		tree.Update(i, tmp)
	}
	nbProofs := 10
	proofs := make([]Proof, nbProofs)
	leafs := make([]fr.Element, nbProofs)
	for pos := 0; pos < nbProofs; pos++ {

		// Make a valid Bytes32
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
	Proofs []GnarkProof        `gnark:",public"`
	Leafs  []frontend.Variable `gnark:",public"`
	Root   frontend.Variable
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
	witness.Leafs = make([]frontend.Variable, nbProofs)
	for i := 0; i < nbProofs; i++ {
		witness.Proofs[i].Siblings = make([]frontend.Variable, len(proofs[i].Siblings))
		for j := 0; j < len(proofs[i].Siblings); j++ {
			witness.Proofs[i].Siblings[j] = proofs[i].Siblings[j].String()
		}
		witness.Proofs[i].Path = proofs[i].Path
		witness.Leafs[i] = leafs[i].String()
	}
	witness.Root = root.String()

	// compile circuit
	var circuit MerkleProofCircuit
	circuit.Proofs = make([]GnarkProof, nbProofs)
	circuit.Leafs = make([]frontend.Variable, nbProofs)
	for i := 0; i < nbProofs; i++ {
		circuit.Proofs[i].Siblings = make([]frontend.Variable, len(proofs[i].Siblings))
	}
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		t.Fatal(err)
	}

	// solve the circuit
	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}

}

// ------------------------------------------------------------------------------
// Test where the leaves are koalabear octuplet
func getMerkleProofWithEncoding(t *testing.T) ([]Proof, []field.Octuplet, []fr.Element, fr.Element) {

	depth := 40
	tree := NewEmptyTree(depth)

	// populate the tree
	nbLeaves := 10
	var tmpFrElmt []fr.Element
	leavesOctuplet := make([]field.Octuplet, nbLeaves)
	for i := 0; i < nbLeaves; i++ {
		for j := 0; j < 8; j++ {
			leavesOctuplet[i][j].SetRandom()
		}
		tmpFrElmt = encoding.EncodeKoalabearsToFrElement(leavesOctuplet[i][:])
		tree.Update(i, tmpFrElmt[0])
	}
	nbProofs := 10
	proofs := make([]Proof, nbProofs)
	leavesFrElmt := make([]fr.Element, nbProofs)
	for pos := 0; pos < nbProofs; pos++ {
		leavesFrElmt[pos], _ = tree.GetLeaf(pos)
		proofs[pos], _ = tree.Prove(pos)
		err := Verify(&proofs[pos], leavesFrElmt[pos], tree.Root)
		if err != nil {
			t.Fatal(err)
		}
	}

	return proofs, leavesOctuplet, leavesFrElmt, tree.Root
}

type MerkleProofCircuitWithEncoding struct {
	Proofs         []GnarkProof          `gnark:",public"`
	LeavesFrElmt   []frontend.Variable   `gnark:",public"`
	Leavesoctuplet []koalagnark.Octuplet `gnark:",public"`
	Root           frontend.Variable
}

func (circuit *MerkleProofCircuitWithEncoding) Define(api frontend.API) error {

	// check that the encoding is ok
	for i := 0; i < len(circuit.LeavesFrElmt); i++ {
		encodedLeaf := encoding.EncodeWVsToFVs(api, circuit.Leavesoctuplet[i][:])
		if len(encodedLeaf) != 1 {
			return errors.New("encodedLeaf should contain 1 element")
		}
		api.AssertIsEqual(encodedLeaf[0], circuit.LeavesFrElmt[i])
	}

	// verify the merkle proofs
	for i := 0; i < len(circuit.Proofs); i++ {
		GnarkVerifyMerkleProof(api, circuit.Proofs[i], circuit.LeavesFrElmt[i], circuit.Root)
	}
	return nil
}

func TestMerkleProofWithEncodingGnark(t *testing.T) {

	// generate witness
	proofs, leavesOctuplet, leavesFrElmts, root := getMerkleProofWithEncoding(t)
	nbProofs := len(proofs)
	var witness MerkleProofCircuitWithEncoding
	witness.Proofs = make([]GnarkProof, nbProofs)
	witness.LeavesFrElmt = make([]frontend.Variable, nbProofs)
	witness.Leavesoctuplet = make([]koalagnark.Octuplet, nbProofs)
	for i := 0; i < nbProofs; i++ {
		witness.Proofs[i].Siblings = make([]frontend.Variable, len(proofs[i].Siblings))
		for j := 0; j < len(proofs[i].Siblings); j++ {
			witness.Proofs[i].Siblings[j] = proofs[i].Siblings[j].String()
		}
		witness.Proofs[i].Path = proofs[i].Path
		for j := 0; j < 8; j++ {
			witness.Leavesoctuplet[i][j] = koalagnark.NewElementFromKoala(leavesOctuplet[i][j])
		}
		witness.LeavesFrElmt[i] = leavesFrElmts[i]
	}
	witness.Root = root.String()

	// compile circuit
	var circuit MerkleProofCircuitWithEncoding
	circuit.Proofs = make([]GnarkProof, nbProofs)
	circuit.LeavesFrElmt = make([]frontend.Variable, nbProofs)
	circuit.Leavesoctuplet = make([]koalagnark.Octuplet, nbProofs)
	for i := 0; i < nbProofs; i++ {
		circuit.Proofs[i].Siblings = make([]frontend.Variable, len(proofs[i].Siblings))
	}
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		t.Fatal(err)
	}

	// solve the circuit
	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}

}
