package vortex

import (
	"fmt"

	bls12377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/crypto/encoding"

	"github.com/consensys/linea-monorepo/prover/crypto/vortex"
	vortex_bls12377 "github.com/consensys/linea-monorepo/prover/crypto/vortex/vortex_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex/vortex_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// ExplicitPolynomialEval is a [wizard.VerifierAction] that evaluates the
// public polynomial.
type ExplicitPolynomialEval struct {
	*Ctx
}

// VortexVerifierAction is a [wizard.VerifierAction] that runs the verifier of
// the Vortex protocol.
type VortexVerifierAction struct {
	*Ctx
}

func (a *ExplicitPolynomialEval) Run(run wizard.Runtime) error {
	return a.explicitPublicEvaluation(run) // Adjust based on context; see note below
}

func (ctx *VortexVerifierAction) Run(run wizard.Runtime) error {

	if ctx.IsBLS {
		return ctx.runBLS(run)
	} else {
		return ctx.runKoala(run)
	}
}

func (ctx *VortexVerifierAction) runKoala(run wizard.Runtime) error {

	// The skip verification flag may be on, if the current vortex
	// context get self-recursed. In this case, the verifier does
	// not need to do anything
	if ctx.IsSelfrecursed {
		return nil
	}

	var (
		// The roots of the merkle trees. We stack the no SIS round
		// roots before the SIS roots. The precomputed root is the
		// first root of the SIS roots if SIS hash is applied on the
		// precomputed. Otherwise, it is the first root of the no SIS roots.
		noSisRoots = []field.Octuplet{}
		sisRoots   = []field.Octuplet{}
		// Slice of true value of length equal to the number of no SIS round
		// + 1 (if SIS is not applied to precomputed)
		flagForNoSISRounds = []bool{}
		// Slice of false value of length equal to the number of SIS round
		// + 1 (if SIS is applied to precomputed)
		flagForSISRounds = []bool{}
	)

	// Append the precomputed roots and the corresponding flag
	if ctx.IsNonEmptyPrecomputed() {
		var precompRootF field.Octuplet
		for i := 0; i < blockSize; i++ {
			precompRootSv := run.GetColumn(ctx.Items.Precomputeds.MerkleRoot[i].GetColID())
			precompRootF[i] = precompRootSv.IntoRegVecSaveAlloc()[0]
		}

		if ctx.IsSISAppliedToPrecomputed() {
			sisRoots = append(sisRoots, precompRootF)
			flagForSISRounds = append(flagForSISRounds, false)
		} else {
			noSisRoots = append(noSisRoots, precompRootF)
			flagForNoSISRounds = append(flagForNoSISRounds, true)
		}
	}

	// Collect all the roots: rounds by rounds
	// and append them to the sis or no sis roots
	for round := 0; round <= ctx.MaxCommittedRound; round++ {

		// If the round is empty (i.e. the wizard does not have any committed
		// columns associated to this round), then the rootSv and rootF will not
		// be defined so this case cannot be handled as a "switch-case" as in
		// the "if" clause below.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}

		var precompRootF field.Octuplet
		for i := 0; i < blockSize; i++ {
			rootSv := run.GetColumn(ctx.Items.MerkleRoots[round][i].GetColID())
			precompRootF[i] = rootSv.IntoRegVecSaveAlloc()[0]
		}

		switch ctx.RoundStatus[round] {
		// IsOnlyPoseidon2Applied is equivalent to No SIS hashing applied
		case IsNoSis:
			noSisRoots = append(noSisRoots, precompRootF)
			flagForNoSISRounds = append(flagForNoSISRounds, true)
		case IsSISApplied:
			sisRoots = append(sisRoots, precompRootF)
			flagForSISRounds = append(flagForSISRounds, false)
		default:
			utils.Panic("Unexpected round status: %v", ctx.RoundStatus[round])
		}
	}

	// assign the roots and the WithSis flags
	roots := append(noSisRoots, sisRoots...)
	IsSISReplacedByPoseidon2 := append(flagForNoSISRounds, flagForSISRounds...)

	WithSis := make([]bool, len(IsSISReplacedByPoseidon2))
	for i := range IsSISReplacedByPoseidon2 {
		WithSis[i] = !IsSISReplacedByPoseidon2[i]
	}

	proof := &vortex.OpeningProof{}
	randomCoin := run.GetRandomCoinFieldExt(ctx.LinCombRandCoinName())

	// Collect the linear combination
	proof.LinearCombination = run.GetColumn(ctx.LinCombName())

	// Collect the random entry List and the random coin
	entryList := run.GetRandomCoinIntegerVec(ctx.RandColSelectionName())

	// Collect the opened columns and split them "by-commitment-rounds"
	proof.Columns = ctx.RecoverSelectedColumns(run, entryList)
	x := run.GetUnivariateParams(ctx.Query.QueryID).ExtX

	packedMProofs := [8]smartvectors.SmartVector{}
	for i := range packedMProofs {
		packedMProofs[i] = run.GetColumn(ctx.MerkleProofName(i))
	}

	merkleProofs := ctx.unpackMerkleProofs(packedMProofs, entryList)

	var vi vortex.VerifierInput
	vi.X = x
	vi.Alpha = randomCoin
	vi.EntryList = entryList
	vi.Ys = ctx.getYs(run)

	return vortex_koalabear.Verify(ctx.VortexKoalaParams, proof, &vi, roots, merkleProofs, WithSis)
}

func (ctx *VortexVerifierAction) runBLS(run wizard.Runtime) error {

	// The skip verification flag may be on, if the current vortex
	// context get self-recursed. In this case, the verifier does
	// not need to do anything
	if ctx.IsSelfrecursed {
		return nil
	}

	var (
		blsNoSisRoots = []bls12377.Element{}

		// Slice of true value of length equal to the number of no SIS round
		// + 1 (if SIS is not applied to precomputed)
		WithSis = []bool{}
	)

	// Append the precomputed roots and the corresponding flag
	if ctx.IsNonEmptyPrecomputed() {

		var precompRootF [encoding.KoalabearChunks]field.Element
		for i := 0; i < encoding.KoalabearChunks; i++ {
			precompRootSv := run.GetColumn(ctx.Items.Precomputeds.BLSMerkleRoot[i].GetColID())
			precompRootF[i] = precompRootSv.IntoRegVecSaveAlloc()[0]
		}

		blsNoSisRoots = append(blsNoSisRoots, encoding.DecodeKoalabearToBLS12Root(precompRootF))
		WithSis = append(WithSis, false)

	}

	// Collect all the roots: rounds by rounds
	// and append them to the sis or no sis roots
	for round := 0; round <= ctx.MaxCommittedRound; round++ {

		// If the round is empty (i.e. the wizard does not have any committed
		// columns associated to this round), then the rootSv and rootF will not
		// be defined so this case cannot be handled as a "switch-case" as in
		// the "if" clause below.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}

		var precompRootF [encoding.KoalabearChunks]field.Element
		for i := 0; i < encoding.KoalabearChunks; i++ {
			rootSv := run.GetColumn(ctx.Items.BLSMerkleRoots[round][i].GetColID())
			precompRootF[i] = rootSv.IntoRegVecSaveAlloc()[0]
		}

		switch ctx.RoundStatus[round] {
		case IsNoSis:
			blsNoSisRoots = append(blsNoSisRoots, encoding.DecodeKoalabearToBLS12Root(precompRootF))
			WithSis = append(WithSis, false)
		default:
			utils.Panic("Unexpected round status: %v", ctx.RoundStatus[round])
		}
	}

	proof := &vortex.OpeningProof{}
	randomCoin := run.GetRandomCoinFieldExt(ctx.LinCombRandCoinName())

	// Collect the linear combination
	proof.LinearCombination = run.GetColumn(ctx.LinCombName())

	// Collect the random entry List and the random coin
	entryList := run.GetRandomCoinIntegerVec(ctx.RandColSelectionName())

	// Collect the opened columns and split them "by-commitment-rounds"
	proof.Columns = ctx.RecoverSelectedColumns(run, entryList)
	x := run.GetUnivariateParams(ctx.Query.QueryID).ExtX

	packedMProofs := [encoding.KoalabearChunks]smartvectors.SmartVector{}
	for i := range packedMProofs {
		packedMProofs[i] = run.GetColumn(ctx.MerkleProofName(i))
	}

	merkleProofs := ctx.unpackBLSMerkleProofs(packedMProofs, entryList)

	var vi vortex.VerifierInput
	vi.X = x
	vi.Alpha = randomCoin
	vi.EntryList = entryList
	vi.Ys = ctx.getYs(run)

	return vortex_bls12377.Verify(ctx.VortexBLSParams, proof, &vi, blsNoSisRoots, merkleProofs, WithSis)
}

// returns the number of committed rows for the given round. This takes
// into account the fact that we use shadow columns.
func (ctx *Ctx) getNbCommittedRows(round int) int {
	return ctx.CommitmentsByRounds.LenOf(round)
}

// returns the Ys as a vector
func (ctx *Ctx) getYs(run wizard.Runtime) (ys [][]fext.Element) {

	var (
		query   = ctx.Query
		params  = run.GetUnivariateParams(ctx.Query.QueryID)
		ysNoSIS = [][]fext.Element{}
		ysSIS   = [][]fext.Element{}
	)

	// Build an index table to efficiently lookup an alleged
	// prover evaluation from its colID.
	ysMap := make(map[ifaces.ColID]fext.Element, len(params.ExtYs))
	for i := range query.Pols {
		ysMap[query.Pols[i].GetColID()] = params.ExtYs[i]
	}

	// Also add the shadow evaluations into ysMap. Since the shadow columns
	// are full-zeroes. We know that the evaluation will also always be zero
	//
	// The sorting is necessary to ensure that the iteration below happens in
	// deterministic order over the [ShadowCols] map.
	shadowIDs := utils.SortedKeysOf(ctx.ShadowCols, func(a, b ifaces.ColID) bool {
		return a < b
	})

	for _, shadowID := range shadowIDs {
		ysMap[shadowID] = fext.Zero()
	}

	// add ys for precomputed
	if ctx.IsNonEmptyPrecomputed() {
		names := make([]ifaces.ColID, len(ctx.Items.Precomputeds.PrecomputedColums))
		for i, poly := range ctx.Items.Precomputeds.PrecomputedColums {
			names[i] = poly.GetColID()
		}
		ysPrecomputed := make([]fext.Element, len(names))
		for i, name := range names {
			ysPrecomputed[i] = ysMap[name]
		}
		// conditionally append the ysPrecomputed to the SIS or no SIS list
		if ctx.IsSISAppliedToPrecomputed() {
			ysSIS = append(ysSIS, ysPrecomputed)
		} else {
			ysNoSIS = append(ysNoSIS, ysPrecomputed)
		}
	}

	// Get the list of the polynomials rounds by rounds
	// and append them to the sis or no sis lists
	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		// If the round is empty (i.e. the wizard does not have any committed
		// columns associated to this round), then the ysRounds will not
		// be defined so this case cannot be handled as a "switch-case" as in
		// the "if" clause below.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}
		names := ctx.CommitmentsByRounds.MustGet(round)
		ysRounds := make([]fext.Element, len(names))
		for i, name := range names {
			ysRounds[i] = ysMap[name]
		}
		// conditionally append ysRounds to the SIS or no SIS list
		if ctx.RoundStatus[round] == IsNoSis {
			ysNoSIS = append(ysNoSIS, ysRounds)
		} else if ctx.RoundStatus[round] == IsSISApplied {
			ysSIS = append(ysSIS, ysRounds)
		}
	}
	// Append the ysNoSIS and ysSIS
	// We compute ysFull = (ysNoSIS, ysSIS)
	ysFull := append(ysNoSIS, ysSIS...)

	return ysFull
}

// Returns the opened columns from the messages. The returned columns are
// split "by-commitment-round".
func (ctx *Ctx) RecoverSelectedColumns(run wizard.Runtime, entryList []int) [][][]field.Element {
	var (
		openedSubColumns = [][][]field.Element{}
		// slice containing the number of rows per SIS round
		numRowsPerSisRound = []int{}
		// slice containing the number of rows per non SIS round
		numRowsPerNonSisRound = []int{}
		// the running offset of rows count
		roundStartAt = 0
	)
	// Collect the columns : first extract the full columns
	// Bear in mind that the prover messages are zero-padded
	fullSelectedCols := make([][]field.Element, len(entryList))
	for j := range entryList {
		fullSelectedCol := run.GetColumn(ctx.SelectedColName(j))
		fullSelectedCols[j] = smartvectors.IntoRegVec(fullSelectedCol)
	}

	// Next we compute numRowsPerRound
	// Process precomputed
	if ctx.IsNonEmptyPrecomputed() {
		numPrecomputeds := len(ctx.Items.Precomputeds.PrecomputedColums)
		// conditionally append numPrecomputeds
		// to the SIS or no SIS list
		if ctx.IsSISAppliedToPrecomputed() {
			numRowsPerSisRound = append(numRowsPerSisRound, numPrecomputeds)
		} else {
			numRowsPerNonSisRound = append(numRowsPerNonSisRound, numPrecomputeds)
		}
	}

	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		// If the round is empty (i.e. the wizard does not have any committed
		// columns associated to this round), then the openedSubColumnsForRound
		// will not be defined so this case cannot be handled as a "switch-case" as in
		// the "if" clause below.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}
		numRowsForRound := ctx.getNbCommittedRows(round)
		// conditionally append the numRowsForRound
		// to the SIS or no SIS list
		if ctx.RoundStatus[round] == IsNoSis {
			numRowsPerNonSisRound = append(numRowsPerNonSisRound, numRowsForRound)
		} else if ctx.RoundStatus[round] == IsSISApplied {
			numRowsPerSisRound = append(numRowsPerSisRound, numRowsForRound)
		}
	}
	// Append the no SIS and SIS rows counts
	// numRowsPerRound = (numRowsPerNonSisRound, numRowsPerSisRound)
	numRowsPerRound := append(numRowsPerNonSisRound, numRowsPerSisRound...)

	// Next compute the openedSubColumns
	for _, numRows := range numRowsPerRound {
		openedSubColumnsForRound := make([][]field.Element, len(entryList))
		for j := range entryList {
			openedSubColumnsForRound[j] = fullSelectedCols[j][roundStartAt : roundStartAt+numRows]
		}

		// update the start counter to ensure we do not pass twice the same row
		roundStartAt += numRows
		openedSubColumns = append(openedSubColumns, openedSubColumnsForRound)
	}

	// sanity-check : make sure we have not forgotten any column
	// We need to treat the precomputed separately if they are committed
	if roundStartAt != ctx.CommittedRowsCount {
		utils.Panic("we have a mistmatch in the row count : %v != %v", roundStartAt, ctx.CommittedRowsCount)
	}

	return openedSubColumns
}

// Evaluates explicitly the public polynomials (proof, vk, public inputs)
func (ctx *Ctx) explicitPublicEvaluation(run wizard.Runtime) error {

	params := run.GetUnivariateParams(ctx.Query.QueryID)

	for i, pol := range ctx.Query.Pols {

		// If the column is a VerifierDefined column, then it is
		// directly concerned by direct verification but we can
		// access its witness or status so we need a specific check.
		if _, ok := pol.(verifiercol.VerifierCol); !ok {
			status := ctx.Comp.Columns.Status(pol.GetColID())
			if !status.IsPublic() {
				// then, its not concerned by direct evaluation
				continue
			}
		}

		val := pol.GetColAssignment(run)

		y := smartvectors.EvaluateFextPolyLagrange(val, params.ExtX)
		if y != params.ExtYs[i] {
			return fmt.Errorf("inconsistent evaluation")
		}
	}

	return nil
}

type ShadowRowProverAction struct {
	Name ifaces.ColID
	Size int
}

// Run assigns the column to a constant column of zeros, which is a shadow row.
func (a *ShadowRowProverAction) Run(run *wizard.ProverRuntime) {
	run.AssignColumn(a.Name, smartvectors.NewConstant(field.Zero(), a.Size))
}

// A shadow row is a row filled with zeroes that we **may** add at the end of
// the rounds commitment. Its purpose is to ensure the number of "SIS limbs" in
// a row divides the degree of the ring-SIS instance.
func autoAssignedShadowRow(comp *wizard.CompiledIOP, size, round, id int) ifaces.Column {

	name := ifaces.ColIDf("VORTEX_%v_SHADOW_ROUND_%v_ID_%v", comp.SelfRecursionCount, round, id)
	col := comp.InsertCommit(round, name, size, true)

	comp.RegisterProverAction(round, &ShadowRowProverAction{
		Name: name,
		Size: size,
	})

	return col
}
