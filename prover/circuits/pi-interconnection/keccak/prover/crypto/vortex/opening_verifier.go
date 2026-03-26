package vortex

import (
	"errors"
	"fmt"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/hashtypes"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/poly"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
)

var (
	// ErrInvalidVerifierInputs flags a verifier input with invalid dimensions.
	ErrInvalidVerifierInputs = errors.New("invalid verification input")
)

// VerifierInputs represents the statement made by the prover in the opening
// protocol of Vortex. It stands for univariate evaluation as desribed in the
// paper. The struct stands for the input of the checks of the verifier, it
// does not cover the construction of the random coins.
//
// Thus, the caller is responsible for handling the construction of RandomCoin
// and EntryList as they are the random coins. It can be done by sampling them
// at random in the interactive setting or using the Fiat-Shamir heuristic.
//
// In our settings, the caller is a function in a framework managing the random
// coins as Vortex is used a sub-protocol of a larger protocol.
type VerifierInputs struct {
	// Params are the public parameters
	Params Params
	// MerkleRoots are the commitment to verify the opening for
	MerkleRoots []types.Bytes32
	// X is the univariate evaluation point
	X field.Element
	// Ys are the alleged evaluation at point X
	Ys [][]field.Element
	// OpeningProof contains the messages of the prover
	OpeningProof OpeningProof
	// RandomCoin is the random coin sampled by the verifier to be used to
	// construct the linear combination of the columns.
	RandomCoin field.Element
	// EntryList is the random coin representing the columns to open.
	EntryList []int
	// Flag indicating if the SIS hash is replaced for the particular round
	// the default behavior is to use the SIS hash function along with the
	// MiMC hash function
	IsSISReplacedByMiMC []bool
}

// VerifyOpening verifies a Vortex opening proof, see [VerifierInputs] for
// more details. The function returns an error on failure. The function validates
// the dimensions of the items in the proof and returns an error if they are
// inconsistent with the parameters. If the provided parameters are invalid
// themselves the function may panic.
func VerifyOpening(v *VerifierInputs) error {

	var (
		numCommitments = len(v.MerkleRoots)
		numEntries     = len(v.EntryList)
		proof          = v.OpeningProof
	)

	if numCommitments == 0 {
		return ErrInvalidVerifierInputs
	}

	if len(v.Ys) != numCommitments ||
		len(proof.Columns) != numCommitments ||
		len(proof.MerkleProofs) != numCommitments ||
		proof.LinearCombination.Len() != v.Params.NumEncodedCols() {
		return ErrInvalidVerifierInputs
	}

	for i := range v.MerkleRoots {

		if len(proof.Columns[i]) != numEntries ||
			len(proof.MerkleProofs[i]) != numEntries ||
			len(v.Ys[i]) == 0 ||
			len(v.Ys[i]) > v.Params.MaxNbRows {
			return ErrInvalidVerifierInputs
		}

		for j := range v.EntryList {
			if len(proof.Columns[i][j]) != len(v.Ys[i]) ||
				len(proof.Columns) == 0 {
				return ErrInvalidVerifierInputs
			}
		}
	}

	if err := v.Params.isCodeword(v.OpeningProof.LinearCombination); err != nil {
		return err
	}

	if err := v.checkColLinCombination(); err != nil {
		return err
	}

	if err := v.checkStatement(); err != nil {
		return err
	}

	if err := v.checkColumnInclusion(); err != nil {
		return err
	}

	return nil
}

// checkColLinCombination checks that the inner-product of the opened column
// (concatenated together) matches the requested positions of the
// RowLinearCombination.
func (v *VerifierInputs) checkColLinCombination() (err error) {

	linearCombination := v.OpeningProof.LinearCombination

	for j, selectedColID := range v.EntryList {
		// Will carry the concatenation of the columns for the same entry j
		fullCol := []field.Element{}

		for i := range v.MerkleRoots {
			// Entries of the selected columns #j contained in the commitment #i.
			selectedSubCol := v.OpeningProof.Columns[i][j]
			fullCol = append(fullCol, selectedSubCol...)
		}

		// Check the linear combination is consistent with the opened column
		y := poly.EvalUnivariate(fullCol, v.RandomCoin)

		if selectedColID > linearCombination.Len() {
			return fmt.Errorf("entry overflows the size of the linear combination")
		}

		if y != linearCombination.Get(selectedColID) {
			other := linearCombination.Get(selectedColID)
			return fmt.Errorf("the linear combination is inconsistent %v : %v", y.String(), other.String())
		}
	}

	return nil
}

// checkStatement checks that the row linear combination is consistent
// with the statement. The function returns an error if the check fails.
func (v *VerifierInputs) checkStatement() (err error) {

	// Check the consistency of Ys and proof.Linear combination
	var (
		Yjoined     = utils.Join(v.Ys...)
		alphaY      = smartvectors.Interpolate(v.OpeningProof.LinearCombination, v.X)
		alphaYProme = poly.EvalUnivariate(Yjoined, v.RandomCoin)
	)

	if alphaY != alphaYProme {
		return fmt.Errorf("RowLincomb and Y are inconsistent")
	}

	return nil
}

// checkColumnInclusion checks the inclusion of the opened column in their
// respective Merkle trees at the positions requested by the verifier. Returns
// an error if a proof does not pass.
func (v *VerifierInputs) checkColumnInclusion() error {

	var (
		mTreeHashConfig = &smt.Config{
			HashFunc: func() hashtypes.Hasher {
				return hashtypes.Hasher{Hash: mimc.NewMiMC()}
			},
			Depth: utils.Log2Ceil(v.Params.NumEncodedCols()),
		}
	)

	// If IsSISReplacedByMiMC is not assigned, we assign them with default false values
	if v.IsSISReplacedByMiMC == nil {
		v.IsSISReplacedByMiMC = make([]bool, len(v.MerkleRoots))
	}

	for i := 0; i < len(v.MerkleRoots); i++ {
		for j := 0; j < len(v.EntryList); j++ {

			var (
				// Selected columns #j contained in the commitment #i.
				selectedSubCol = v.OpeningProof.Columns[i][j]
				leaf           types.Bytes32
				entry          = v.EntryList[j]
				root           = v.MerkleRoots[i]
				mProof         = v.OpeningProof.MerkleProofs[i][j]
			)
			// We verify the SIS hash of the current sub-column
			// only if the SIS hash is applied for the current round.
			if !v.IsSISReplacedByMiMC[i] {
				var (
					// SIS hash of the current sub-column
					sisHash = v.Params.Key.Hash(selectedSubCol)
					// hasher used to hash the SIS hash (and thus not a hasher
					// based on SIS)
					hasher = mimc.NewMiMC()
				)

				hasher.Reset()
				for _, x := range sisHash {
					xBytes := x.Bytes()
					hasher.Write(xBytes[:])
				}
				copy(leaf[:], hasher.Sum(nil))

			} else {
				// We assume that HashFunc (to be used for Merkle Tree) and NoSisHashFunc()
				// (to be used for in place of SIS hash) are the same i.e. the MiMC hash function
				hasher := mimc.NewFieldHasher()
				hasher.Reset()
				s := hasher.SumElements(selectedSubCol)
				leaf = s.Bytes()
			}

			// Check the Merkle-proof for the obtained leaf
			ok := mProof.Verify(mTreeHashConfig, leaf, root)
			if !ok {
				return fmt.Errorf("merkle proof failed for com #%v and entry %v (mProof.path=%v)", i, j, mProof.Path)
			}

			// And check that the Merkle proof is related to the correct entry
			if mProof.Path != entry {
				return fmt.Errorf("expected the Merkle proof to hold for position %v but was %v", entry, mProof.Path)
			}
		}
	}

	return nil
}
