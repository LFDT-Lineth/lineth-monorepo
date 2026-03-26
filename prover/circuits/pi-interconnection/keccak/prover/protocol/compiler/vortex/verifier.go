package vortex

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
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
		noSisRoots = []types.Bytes32{}
		sisRoots   = []types.Bytes32{}
		// Slice of true value of length equal to the number of no SIS round
		// + 1 (if SIS is not applied to precomputed)
		flagForNoSISRounds = []bool{}
		// Slice of false value of length equal to the number of SIS round
		// + 1 (if SIS is applied to precomputed)
		flagForSISRounds = []bool{}
	)

	// Append the precomputed roots and the corresponding flag
	if ctx.IsNonEmptyPrecomputed() {
		precompRootSv := run.GetColumn(ctx.Items.Precomputeds.MerkleRoot.GetColID()) // len 1 smart vector
		precompRootF := precompRootSv.Get(0)                                         // root as a field element

		if ctx.IsSISAppliedToPrecomputed() {
			sisRoots = append(sisRoots, types.Bytes32(precompRootF.Bytes()))
			flagForSISRounds = append(flagForSISRounds, false)
		} else {
			noSisRoots = append(noSisRoots, types.Bytes32(precompRootF.Bytes()))
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

		rootSv := run.GetColumn(ctx.Items.MerkleRoots[round].GetColID()) // len 1 smart vector
		rootF := rootSv.Get(0)                                           // root as field element

		// Append the isSISApplied flag
		if ctx.RoundStatus[round] == IsOnlyMiMCApplied {
			noSisRoots = append(noSisRoots, types.Bytes32(rootF.Bytes()))
			flagForNoSISRounds = append(flagForNoSISRounds, true)
		} else if ctx.RoundStatus[round] == IsSISApplied {
			sisRoots = append(sisRoots, types.Bytes32(rootF.Bytes()))
			flagForSISRounds = append(flagForSISRounds, false)
		}
	}
	// assign the roots and the isSisReplacedByMiMC flags
	roots := append(noSisRoots, sisRoots...)
	isSISReplacedByMiMC := append(flagForNoSISRounds, flagForSISRounds...)

	proof := &vortex.OpeningProof{}
	randomCoin := run.GetRandomCoinField(ctx.LinCombRandCoinName())

	// Collect the linear combination
	proof.LinearCombination = run.GetColumn(ctx.LinCombName())

	// Collect the random entry List and the random coin
	entryList := run.GetRandomCoinIntegerVec(ctx.RandColSelectionName())

	// Collect the opened columns and split them "by-commitment-rounds"
	proof.Columns = ctx.RecoverSelectedColumns(run, entryList)
	x := run.GetUnivariateParams(ctx.Query.QueryID).X

	packedMProofs := run.GetColumn(ctx.MerkleProofName())
	proof.MerkleProofs = ctx.unpackMerkleProofs(packedMProofs, entryList)

	return vortex.VerifyOpening(&vortex.VerifierInputs{
		Params:              *ctx.VortexParams,
		MerkleRoots:         roots,
		X:                   x,
		Ys:                  ctx.getYs(run),
		OpeningProof:        *proof,
		RandomCoin:          randomCoin,
		EntryList:           entryList,
		IsSISReplacedByMiMC: isSISReplacedByMiMC,
	})
}

// returns the number of committed rows for the given round. This takes
// into account the fact that we use shadow columns.
func (ctx *Ctx) getNbCommittedRows(round int) int {
	return ctx.CommitmentsByRounds.LenOf(round)
}

// returns the Ys as a vector
func (ctx *Ctx) getYs(run wizard.Runtime) (ys [][]field.Element) {

	var (
		query   = ctx.Query
		params  = run.GetUnivariateParams(ctx.Query.QueryID)
		ysNoSIS = [][]field.Element{}
		ysSIS   = [][]field.Element{}
	)

	// Build an index table to efficiently lookup an alleged
	// prover evaluation from its colID.
	ysMap := make(map[ifaces.ColID]field.Element, len(params.Ys))
	for i := range query.Pols {
		ysMap[query.Pols[i].GetColID()] = params.Ys[i]
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
		ysMap[shadowID] = field.Zero()
	}

	// add ys for precomputed
	if ctx.IsNonEmptyPrecomputed() {
		names := make([]ifaces.ColID, len(ctx.Items.Precomputeds.PrecomputedColums))
		for i, poly := range ctx.Items.Precomputeds.PrecomputedColums {
			names[i] = poly.GetColID()
		}
		ysPrecomputed := make([]field.Element, len(names))
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
		ysRounds := make([]field.Element, len(names))
		for i, name := range names {
			ysRounds[i] = ysMap[name]
		}
		// conditionally append ysRounds to the SIS or no SIS list
		if ctx.RoundStatus[round] == IsOnlyMiMCApplied {
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
		if ctx.RoundStatus[round] == IsOnlyMiMCApplied {
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

		y := smartvectors.Interpolate(val, params.X)
		if y != params.Ys[i] {
			return fmt.Errorf("inconsistent evaluation")
		}
	}

	return nil
}

type ShadowRowProverAction struct {
	Name ifaces.ColID
	Size int
}

func (a *ShadowRowProverAction) Run(run *wizard.ProverRuntime) {
	run.AssignColumn(a.Name, smartvectors.NewConstant(field.Zero(), a.Size))
}

// A shadow row is a row filled with zeroes that we **may** add at the end of
// the rounds commitment. Its purpose is to ensure the number of "SIS limbs" in
// a row divides the degree of the ring-SIS instance.
func autoAssignedShadowRow(comp *wizard.CompiledIOP, size, round, id int) ifaces.Column {

	name := ifaces.ColIDf("VORTEX_%v_SHADOW_ROUND_%v_ID_%v", comp.SelfRecursionCount, round, id)
	col := comp.InsertCommit(round, name, size)

	comp.RegisterProverAction(round, &ShadowRowProverAction{
		Name: name,
		Size: size,
	})

	return col
}
