package vortex

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// Final circuit - commitment using Merkle trees
type VerifyOpeningCircuitMerkleTree struct {
	Proof      GProof                `gnark:",public"`
	Roots      []frontend.Variable   `gnark:",public"`
	X          frontend.Variable     `gnark:",public"`
	RandomCoin frontend.Variable     `gnark:",public"`
	Ys         [][]frontend.Variable `gnark:",public"`
	EntryList  []frontend.Variable   `gnark:",public"`
	Params     GParams
}

// allocate the variables for the verification circuit with Merkle trees
func AllocateCircuitVariablesWithMerkleTree(
	verifyCircuit *VerifyOpeningCircuitMerkleTree,
	proof OpeningProof,
	ys [][]field.Element,
	entryList []int,
	roots []types.Bytes32) {

	verifyCircuit.Proof.LinearCombination = make([]frontend.Variable, proof.LinearCombination.Len())

	verifyCircuit.Proof.Columns = make([][][]frontend.Variable, len(proof.Columns))
	for i := 0; i < len(proof.Columns); i++ {
		verifyCircuit.Proof.Columns[i] = make([][]frontend.Variable, len(proof.Columns[i]))
		for j := 0; j < len(proof.Columns[i]); j++ {
			verifyCircuit.Proof.Columns[i][j] = make([]frontend.Variable, len(proof.Columns[i][j]))
		}
	}

	verifyCircuit.Proof.MerkleProofs = make([][]smt.GnarkProof, len(proof.MerkleProofs))
	for i := 0; i < len(proof.MerkleProofs); i++ {
		verifyCircuit.Proof.MerkleProofs[i] = make([]smt.GnarkProof, len(proof.MerkleProofs[i]))
		for j := 0; j < len(proof.MerkleProofs[i]); j++ {
			verifyCircuit.Proof.MerkleProofs[i][j].Siblings = make([]frontend.Variable, len(proof.MerkleProofs[i][j].Siblings))
		}
	}

	verifyCircuit.EntryList = make([]frontend.Variable, len(entryList))

	verifyCircuit.Ys = make([][]frontend.Variable, len(ys))
	for i := 0; i < len(ys); i++ {
		verifyCircuit.Ys[i] = make([]frontend.Variable, len(ys[i]))
	}

	verifyCircuit.Roots = make([]frontend.Variable, len(roots))

}

// AssignCicuitVariablesWithMerkleTree assign the variables for the verification circuit with Merkle trees
func AssignCicuitVariablesWithMerkleTree(
	verifyCircuit *VerifyOpeningCircuitMerkleTree,
	proof OpeningProof,
	ys [][]field.Element,
	entryList []int,
	roots []types.Bytes32) {

	frLinComb := make([]fr.Element, proof.LinearCombination.Len())
	proof.LinearCombination.WriteInSlice(frLinComb)
	for i := 0; i < proof.LinearCombination.Len(); i++ {
		verifyCircuit.Proof.LinearCombination[i] = frLinComb[i].String()
	}

	for i := 0; i < len(proof.Columns); i++ {
		for j := 0; j < len(proof.Columns[i]); j++ {
			for k := 0; k < len(proof.Columns[i][j]); k++ {
				verifyCircuit.Proof.Columns[i][j][k] = proof.Columns[i][j][k].String()
			}
		}
	}

	var buf fr.Element
	for i := 0; i < len(proof.MerkleProofs); i++ {
		for j := 0; j < len(proof.MerkleProofs[i]); j++ {
			verifyCircuit.Proof.MerkleProofs[i][j].Path = proof.MerkleProofs[i][j].Path
			for k := 0; k < len(proof.MerkleProofs[i][j].Siblings); k++ {
				buf.SetBytes(proof.MerkleProofs[i][j].Siblings[k][:])
				verifyCircuit.Proof.MerkleProofs[i][j].Siblings[k] = buf.String()
			}
		}
	}

	for i := 0; i < len(entryList); i++ {
		verifyCircuit.EntryList[i] = entryList[i]
	}

	for i := 0; i < len(ys); i++ {
		for j := 0; j < len(ys[i]); j++ {
			verifyCircuit.Ys[i][j] = ys[i][j].String()
		}
	}

	for i := 0; i < len(roots); i++ {
		buf.SetBytes(roots[i][:])
		verifyCircuit.Roots[i] = buf.String()
	}

}

func (circuit *VerifyOpeningCircuitMerkleTree) Define(api frontend.API) error {

	// Generic checks
	err := GnarkVerifyOpeningWithMerkleProof(
		api,
		circuit.Params,
		circuit.Roots,
		circuit.Proof,
		circuit.X,
		circuit.Ys,
		circuit.RandomCoin,
		circuit.EntryList,
	)
	return err
}

func GnarkVerifyOpeningWithMerkleProof(
	api frontend.API,
	params GParams,
	roots []frontend.Variable,
	proof GProof,
	x frontend.Variable,
	ys [][]frontend.Variable,
	randomCoin frontend.Variable,
	entryList []frontend.Variable,
) error {

	if !params.HasNoSisHasher() {
		utils.Panic("the verifier circuit can only be instantiated using a NoSisHasher")
	}

	// Generic checks
	selectedColsHashes, err := GnarkVerifyCommon(
		api,
		params,
		proof.GProofWoMerkle,
		x,
		ys,
		randomCoin,
		entryList,
	)
	if err != nil {
		return err
	}

	hasher, _ := params.HasherFunc(api)
	hasher.Reset()

	for i, root := range roots {
		for j, entry := range entryList {

			// Hash the SIS hash
			var leaf = selectedColsHashes[i][j]

			// Check the Merkle-proof for the obtained leaf
			smt.GnarkVerifyMerkleProof(api, proof.MerkleProofs[i][j], leaf, root, hasher)

			// And check that the Merkle proof is related to the correct entry
			api.AssertIsEqual(proof.MerkleProofs[i][j].Path, entry)
		}
	}

	return nil
}
