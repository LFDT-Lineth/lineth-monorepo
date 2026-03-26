package vortex

import (
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
	"github.com/stretchr/testify/require"
)

func TestLinearCombination(t *testing.T) {

	nPolys := 15
	polySize := 1 << 10
	blowUpFactor := 2

	x := field.NewElement(478)
	randomCoin := field.NewElement(1523)

	params := NewParams(blowUpFactor, polySize, nPolys, ringsis.StdParams)

	// Polynomials to commit to
	polys := make([]smartvectors.SmartVector, nPolys)
	ys := make([]field.Element, nPolys)
	for i := range polys {
		polys[i] = smartvectors.Rand(polySize)
		ys[i] = smartvectors.Interpolate(polys[i], x)
	}

	// Make a linear combination of the poly
	lc := smartvectors.PolyEval(polys, randomCoin)

	// Generate the proof
	proof := params.InitOpeningWithLC(polys, randomCoin)

	// Evaluate the two on a random-ish point. Should
	// yield the same result.
	y0 := smartvectors.Interpolate(lc, x)
	y1 := smartvectors.Interpolate(proof.LinearCombination, x)

	require.Equal(t, y0, y1)
}

// testCaseParameters is a corpus of valid parameters for Vortex
var testCaseParameters = []*Params{
	NewParams(2, 1<<4, 32, ringsis.StdParams),
	NewParams(4, 1<<3, 32, ringsis.StdParams),
}

func TestProver(t *testing.T) {

	var (
		x          = field.NewElement(478)
		randomCoin = field.NewElement(1523)
		entryList  = []int{1, 7, 5, 6, 4, 5, 1, 2}
	)

	// the testCases are applied over all those of [testCaseParameters]
	testCases := []struct {
		Explainer             string
		NumPolysPerCommitment []int
		NumOpenedColumns      int
		// ChangeAssignmentSize takes params.NbColumns and returns a possibly
		// different value corresponding to the size of the assignment that
		// the testCase provides to the prover. If nil, then this is equivalent
		// to `f(n) -> n`.
		ChangeAssignmentSize func(int) int
		// Flag denoting if we are committing with SIS+MiMC hash or MiMC hash
		IsSisReplacedByMiMC []bool
		MustPanic           bool
	}{
		{
			Explainer:             "1 matrix commitment with one poly with SIS commitment",
			NumPolysPerCommitment: []int{1},
			IsSisReplacedByMiMC:   []bool{false},
			NumOpenedColumns:      4,
		},
		{
			Explainer:             "1 matrix commitment with several polys without SIS commitment",
			NumPolysPerCommitment: []int{3},
			IsSisReplacedByMiMC:   []bool{true},
			NumOpenedColumns:      4,
		},
		{
			Explainer:             "2 matrix commitment with several polys with SIS commitment",
			NumPolysPerCommitment: []int{3, 3},
			IsSisReplacedByMiMC:   []bool{false, false},
			NumOpenedColumns:      8,
		},
		{
			Explainer:             "1 matrix commitment with several polys with SIS and no SIS commitment",
			NumPolysPerCommitment: []int{1, 15},
			IsSisReplacedByMiMC:   []bool{false, true},
			NumOpenedColumns:      8,
		},
		{
			Explainer:             "too many rows",
			NumPolysPerCommitment: []int{1, 105},
			NumOpenedColumns:      8,
			MustPanic:             true,
		},
		{
			Explainer:             "no commitment",
			NumPolysPerCommitment: []int{},
			NumOpenedColumns:      8,
			MustPanic:             true,
		},
		{
			Explainer:             "1 commitment but zero rows",
			NumPolysPerCommitment: []int{0},
			NumOpenedColumns:      8,
			MustPanic:             true,
		},
		{
			Explainer:             "Several commitment but none have rows",
			NumPolysPerCommitment: []int{0, 0, 0},
			NumOpenedColumns:      8,
			MustPanic:             true,
		},
		{
			Explainer:             "Several commitment but none have rows",
			NumPolysPerCommitment: []int{0, 0, 0},
			NumOpenedColumns:      8,
			MustPanic:             true,
		},
		{
			Explainer:             "Empty entry list",
			NumPolysPerCommitment: []int{5, 6},
			NumOpenedColumns:      0,
			MustPanic:             true,
		},
		{
			Explainer:             "the polys are twice too large",
			NumPolysPerCommitment: []int{3, 3},
			NumOpenedColumns:      8,
			ChangeAssignmentSize:  func(i int) int { return 2 * i },
			MustPanic:             true,
		},
		{
			Explainer:             "the polys are twice too small",
			NumPolysPerCommitment: []int{3, 3},
			NumOpenedColumns:      8,
			ChangeAssignmentSize:  func(i int) int { return i / 2 },
			MustPanic:             true,
		},
		{
			Explainer:             "the polys are twice too small",
			NumPolysPerCommitment: []int{3, 3},
			NumOpenedColumns:      8,
			ChangeAssignmentSize:  func(i int) int { return i + 1 },
			MustPanic:             true,
		},
		{
			Explainer:             "the polys are twice too small",
			NumPolysPerCommitment: []int{3, 3},
			NumOpenedColumns:      8,
			ChangeAssignmentSize:  func(i int) int { return i - 1 },
			MustPanic:             true,
		},
	}

	for i := range testCaseParameters {
		params := testCaseParameters[i]
		for j := range testCases {

			t.Run(fmt.Sprintf("params-%v-case-%v", i, j), func(t *testing.T) {

				testCase := testCases[j]

				t.Logf("params=%++v test-case=%++v", params, testCase)

				var (
					numCommitments      = len(testCase.NumPolysPerCommitment)
					effPolySize         = params.NbColumns
					polyLists           = make([][]smartvectors.SmartVector, numCommitments)
					yLists              = make([][]field.Element, numCommitments)
					roots               = make([]types.Bytes32, numCommitments)
					trees               = make([]*smt.Tree, numCommitments)
					isSisReplacedByMiMC = make([]bool, numCommitments)
				)

				if testCase.ChangeAssignmentSize != nil {
					effPolySize = testCase.ChangeAssignmentSize(effPolySize)
				}
				if testCase.IsSisReplacedByMiMC != nil {
					isSisReplacedByMiMC = testCase.IsSisReplacedByMiMC
				}

				for i := range polyLists {
					// Polynomials to commit to
					polys := make([]smartvectors.SmartVector, testCase.NumPolysPerCommitment[i])
					ys := make([]field.Element, testCase.NumPolysPerCommitment[i])
					for j := range polys {
						polys[j] = smartvectors.Rand(effPolySize)

						// effPolySize is messed with and is not a power of 2,
						// the interpolation algorithm will panic as this counts
						// as invalid inputs.
						if utils.IsPowerOfTwo(effPolySize) {
							ys[j] = smartvectors.Interpolate(polys[j], x)
						} else {
							ys[j].SetRandom()
						}
					}
					polyLists[i] = polys
					yLists[i] = ys
				}

				// Importantly, the defer must be declared right before calling
				// the tested code or it will occult potential mistakes in the
				// test.
				if testCase.MustPanic {
					defer func() {
						if r := recover(); r != nil {
							t.Logf("Panicked with message = %v and stacktraces %v", r, string(debug.Stack()))
							return
						}

						t.Fatalf("The test did not panic")
					}()
				}

				// Commits to it
				committedMatrices := make([]EncodedMatrix, numCommitments)
				for i := range trees {
					if !isSisReplacedByMiMC[i] {
						committedMatrices[i], trees[i], _ = params.CommitMerkleWithSIS(polyLists[i])
					} else {
						committedMatrices[i], trees[i], _ = params.CommitMerkleWithoutSIS(polyLists[i])
					}
					roots[i] = trees[i].Root
				}

				// Generate the proof
				proof := params.InitOpeningWithLC(utils.Join(polyLists...), randomCoin)
				proof.Complete(entryList[:testCase.NumOpenedColumns], committedMatrices, trees)

				// Check the proof
				err := VerifyOpening(
					&VerifierInputs{
						Params:              *params,
						MerkleRoots:         roots,
						X:                   x,
						Ys:                  yLists,
						OpeningProof:        *proof,
						RandomCoin:          randomCoin,
						EntryList:           entryList[:testCase.NumOpenedColumns],
						IsSISReplacedByMiMC: isSisReplacedByMiMC,
					})

				require.NoError(t, err)
			})
		}
	}
}

func TestVerifierNegative(t *testing.T) {

	var (
		numPolysPerCommitmentCorpus = [][]int{
			{1},
			{1, 3},
			{3, 1, 15},
		}
		params = []*Params{
			NewParams(2, 8, 17, ringsis.StdParams),
			NewParams(2, 8, 17, ringsis.StdParams),
		}

		statementMutatorCorpus = []struct {
			Explainer string
			Func      func(*VerifierInputs) bool
		}{
			{
				Explainer: "Increment the first y",
				Func: func(v *VerifierInputs) bool {
					one := field.One()
					v.Ys[0][0].Add(&v.Ys[0][0], &one)
					return true
				},
			},
			{
				Explainer: "Swap the two first y in the first slice",
				Func: func(v *VerifierInputs) bool {
					if len(v.Ys[0]) < 2 {
						return false
					}
					v.Ys[0][1], v.Ys[0][0] = v.Ys[0][0], v.Ys[0][1]
					return true
				},
			},
			{
				Explainer: "Swap the two slices in Y",
				Func: func(v *VerifierInputs) bool {
					if len(v.Ys) < 2 {
						return false
					}
					v.Ys[0], v.Ys[1] = v.Ys[1], v.Ys[0]
					return true
				},
			},
			{
				Explainer: "Move the last entry of Ys[0] to the beginning of Ys[1]",
				Func: func(v *VerifierInputs) bool {
					if len(v.Ys) < 2 {
						return false
					}
					y := v.Ys[0][len(v.Ys[0])-1]
					v.Ys[0] = v.Ys[0][:len(v.Ys[0])-1]
					v.Ys[1] = append([]field.Element{y}, v.Ys[1]...)
					return true
				},
			},
			{
				Explainer: "Bump the X value",
				Func: func(v *VerifierInputs) bool {
					one := field.One()
					v.X.Add(&v.X, &one)
					return true
				},
			},
			{
				Explainer: "Pop the first Y",
				Func: func(v *VerifierInputs) bool {
					v.Ys[0] = v.Ys[0][1:]
					return true
				},
			},
		}

		proofMutatorCorpus = []struct {
			Explainer string
			Func      func(v *VerifierInputs) bool
		}{
			{
				Explainer: "Swap two first entryLists",
				Func: func(v *VerifierInputs) bool {
					v.EntryList[0], v.EntryList[1] = v.EntryList[1], v.EntryList[0]
					return true
				},
			},
			{
				Explainer: "Cut the first entry",
				Func: func(v *VerifierInputs) bool {
					v.EntryList = v.EntryList[1:]
					return true
				},
			},
			{
				Explainer: "Cut the last entry",
				Func: func(v *VerifierInputs) bool {
					v.EntryList = v.EntryList[:len(v.EntryList)-1]
					return true
				},
			},
			{
				Explainer: "Add an extra entry",
				Func: func(v *VerifierInputs) bool {
					v.EntryList = append(v.EntryList, 0)
					return true
				},
			},
			{
				Explainer: "Swap two roots",
				Func: func(v *VerifierInputs) bool {
					if len(v.MerkleRoots) < 2 {
						return false
					}
					v.MerkleRoots[0], v.MerkleRoots[1] = v.MerkleRoots[1], v.MerkleRoots[0]
					return true
				},
			},
			{
				Explainer: "Remove the first root",
				Func: func(v *VerifierInputs) bool {
					if len(v.MerkleRoots) < 1 {
						return false
					}
					v.MerkleRoots = v.MerkleRoots[1:]
					return true
				},
			},
			{
				Explainer: "Add an extra root",
				Func: func(v *VerifierInputs) bool {
					if len(v.MerkleRoots) < 1 {
						return false
					}
					v.MerkleRoots = append(v.MerkleRoots, v.MerkleRoots[0])
					return true
				},
			},
			{
				Explainer: "Swap two positions in the linear combination",
				Func: func(v *VerifierInputs) bool {
					lc := v.OpeningProof.LinearCombination.IntoRegVecSaveAlloc()
					lc[0], lc[1] = lc[1], lc[0]
					lc_ := smartvectors.Regular(lc)
					v.OpeningProof.LinearCombination = &lc_
					return true
				},
			},
			{
				Explainer: "Overwrite a position in the linear combination",
				Func: func(v *VerifierInputs) bool {
					lc := v.OpeningProof.LinearCombination.IntoRegVecSaveAlloc()
					lc[0] = lc[1]
					lc_ := smartvectors.Regular(lc)
					v.OpeningProof.LinearCombination = &lc_
					return true
				},
			},
			{
				Explainer: "Swap two Merkle proofs",
				Func: func(v *VerifierInputs) bool {
					mps := v.OpeningProof.MerkleProofs
					mps[0][0], mps[0][1] = mps[0][1], mps[0][0]
					v.OpeningProof.MerkleProofs = mps
					return true
				},
			},
			{
				Explainer: "Set the first entry to a very large number",
				Func: func(v *VerifierInputs) bool {
					v.EntryList[0] = 10000
					return true
				},
			},
			{
				Explainer: "Mess with a Merkle proof path",
				Func: func(v *VerifierInputs) bool {
					mps := v.OpeningProof.MerkleProofs
					mps[0][0].Path = 5
					return true
				},
			},
		}

		generateVerifierInputs = func(
			params *Params,
			numPolyPerCommitment []int,
		) *VerifierInputs {

			var (
				x              = field.NewElement(43)
				randomCoin     = field.NewElement(393280)
				entryList      = []int{1, 2, 3, 4, 5, 6, 7, 8}
				numCommitments = len(numPolyPerCommitment)
				effPolySize    = params.NbColumns
				polyLists      = make([][]smartvectors.SmartVector, numCommitments)
				yLists         = make([][]field.Element, numCommitments)
				roots          = make([]types.Bytes32, numCommitments)
				trees          = make([]*smt.Tree, numCommitments)
			)

			for i := range polyLists {
				// Polynomials to commit to
				polys := make([]smartvectors.SmartVector, numPolyPerCommitment[i])
				ys := make([]field.Element, numPolyPerCommitment[i])
				for j := range polys {
					polys[j] = smartvectors.Rand(effPolySize)

					// effPolySize is messed with and is not a power of 2,
					// the interpolation algorithm will panic as this counts
					// as invalid inputs.
					if utils.IsPowerOfTwo(effPolySize) {
						ys[j] = smartvectors.Interpolate(polys[j], x)
					} else {
						ys[j].SetRandom()
					}
				}
				polyLists[i] = polys
				yLists[i] = ys
			}

			// Commits to it
			committedMatrices := make([]EncodedMatrix, numCommitments)
			for i := range trees {
				committedMatrices[i], trees[i], _ = params.CommitMerkleWithSIS(polyLists[i])
				roots[i] = trees[i].Root
			}

			// Generate the proof
			proof := params.InitOpeningWithLC(utils.Join(polyLists...), randomCoin)
			proof.Complete(entryList, committedMatrices, trees)

			return &VerifierInputs{
				Params:       *params,
				MerkleRoots:  roots,
				X:            x,
				Ys:           yLists,
				OpeningProof: *proof,
				RandomCoin:   randomCoin,
				EntryList:    entryList,
			}
		}
	)

	for iParams := range params {
		for iNumPoly := range numPolysPerCommitmentCorpus {
			for iMut := range statementMutatorCorpus {
				t.Run(
					fmt.Sprintf("statement-mutation-%v-%v_%v", iParams, iNumPoly, iMut),
					func(t *testing.T) {

						// It's important to regenerate the entry every time as
						// they will be mutated every time.
						v := generateVerifierInputs(
							params[iParams],
							numPolysPerCommitmentCorpus[iNumPoly],
						)

						ok := statementMutatorCorpus[iMut].Func(v)
						if !ok {
							return
						}

						// Check the proof
						err := VerifyOpening(v)
						require.Error(t, err)
					},
				)
			}
		}
	}

	for iParams := range params {
		for iNumPoly := range numPolysPerCommitmentCorpus {
			for iMut := range proofMutatorCorpus {
				t.Run(
					fmt.Sprintf("proof-mutation-%v-%v_%v", iParams, iNumPoly, iMut),
					func(t *testing.T) {

						// It's important to regenerate the entry every time as
						// they will be mutated every time.
						v := generateVerifierInputs(
							params[iParams],
							numPolysPerCommitmentCorpus[iNumPoly],
						)

						ok := proofMutatorCorpus[iMut].Func(v)
						if !ok {
							return
						}

						// Check the proof
						err := VerifyOpening(v)
						require.Error(t, err)
					},
				)
			}
		}
	}
}

func BenchmarkCommitWithSIS(b *testing.B) {
	const (
		blowUpFactor = 2
		polySize     = 1 << 15
		nPolys       = 1 << 10
	)

	params := NewParams(blowUpFactor, polySize, nPolys, ringsis.StdParams)
	// func (p *Params) CommitMerkleWithSIS(ps []smartvectors.SmartVector) (encodedMatrix EncodedMatrix, tree *smt.Tree, colHashes []field.Element) {

	ps := make([]smartvectors.SmartVector, nPolys)
	for i := range ps {
		if i%15 == 0 {
			// sprinkle some constants
			ps[i] = smartvectors.NewConstant(field.NewElement(uint64(i+1)*42), polySize)
			continue
		}
		ps[i] = smartvectors.Rand(polySize)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = params.CommitMerkleWithSIS(ps)
	}
}
