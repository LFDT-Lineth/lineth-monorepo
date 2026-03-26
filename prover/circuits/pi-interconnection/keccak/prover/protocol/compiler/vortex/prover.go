package vortex

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
	"github.com/sirupsen/logrus"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

type commitmentMode int

const (
	// Denotes the Vortex mode when we don't apply
	// self recursion
	NonSelfRecursion commitmentMode = iota
	// Denotes the Vortex mode when we apply
	// self recursion and commit using SIS
	SelfRecursionSIS
	// Denotes the Vortex mode when we apply
	// self recursion and commit using only MiMC
	SelfRecursionMiMCOnly
)

// ReassignPrecomputedRootAction is a [wizard.ProverAction] that assigns the
// precomputed Merkle root of the Vortex invokation. The action is defined
// for round 0 only and only if the AddPrecomputedMerkleRootToPublicInputsOpt
// is enabled.
type ReassignPrecomputedRootAction struct {
	*Ctx
}

func (r ReassignPrecomputedRootAction) Run(run *wizard.ProverRuntime) {
	run.AssignColumn(
		r.Items.Precomputeds.MerkleRoot.GetColID(),
		smartvectors.NewConstant(r.AddPrecomputedMerkleRootToPublicInputsOpt.PrecomputedValue, 1),
	)
}

// ColumnAssignmentProverAction is a [wizard.ProverAction] that assigns the
// the columns at a given round.
type ColumnAssignmentProverAction struct {
	*Ctx
	Round int
}

// Prover steps of Vortex that is run in place of committing to polynomials
func (ctx *ColumnAssignmentProverAction) Run(run *wizard.ProverRuntime) {

	round := ctx.Round

	// Check if that is a dry round
	if ctx.RoundStatus[round] == IsEmpty {
		// Nothing special to do.
		return
	}

	var (
		committedMatrix  vortex.EncodedMatrix
		tree             *smt.Tree
		sisAndMimcDigest []field.Element
		mimcDigest       []field.Element
	)
	pols := ctx.getPols(run, round)
	// If there are no polynomials to commit to, we don't need to do anything
	if len(pols) == 0 {
		logrus.Infof("Vortex AssignColumn at round %v: No polynomials to commit to", round)
		return
	}
	// We commit to the polynomials with SIS hashing if the number of polynomials
	// is greater than the [ApplyToSISThreshold].
	if ctx.RoundStatus[round] == IsOnlyMiMCApplied {
		committedMatrix, tree, mimcDigest = ctx.VortexParams.CommitMerkleWithoutSIS(pols)
	} else if ctx.RoundStatus[round] == IsSISApplied {
		committedMatrix, tree, sisAndMimcDigest = ctx.VortexParams.CommitMerkleWithSIS(pols)
	}
	run.State.InsertNew(ctx.VortexProverStateName(round), committedMatrix)
	run.State.InsertNew(ctx.MerkleTreeName(round), tree)

	// Only to be read by the self-recursion compiler.
	if ctx.IsSelfrecursed {
		// We need to store the SIS and MiMC digests in the prover state
		// so that we can use them in the self-recursion compiler.
		if ctx.RoundStatus[round] == IsOnlyMiMCApplied {
			run.State.InsertNew(ctx.MIMCHashName(round), mimcDigest)
		} else if ctx.RoundStatus[round] == IsSISApplied {
			run.State.InsertNew(ctx.SisHashName(round), sisAndMimcDigest)
		}
	}

	// And assign the 1-sized column to contain the root
	var root field.Element
	root.SetBytes(tree.Root[:])
	run.AssignColumn(ifaces.ColID(ctx.MerkleRootName(round)), smartvectors.NewConstant(root, 1))
}

type LinearCombinationComputationProverAction struct {
	*Ctx
}

// Prover steps of Vortex that is run when committing to the linear combination
// We stack the No SIS round matrices before the SIS round matrices in the committed matrix stack.
// For the precomputed matrix, we stack it on top of the SIS round matrices if SIS is used on it or
// we stack it on top of the No SIS round matrices if SIS is not used on it.
func (ctx *LinearCombinationComputationProverAction) Run(pr *wizard.ProverRuntime) {
	var (
		committedSVSIS   = []smartvectors.SmartVector{}
		committedSVNoSIS = []smartvectors.SmartVector{}
	)
	// Add the precomputed columns
	if ctx.IsNonEmptyPrecomputed() {
		var precomputedSV = []smartvectors.SmartVector{}
		for _, col := range ctx.Items.Precomputeds.PrecomputedColums {
			precomputedSV = append(precomputedSV, col.GetColAssignment(pr))
		}
		// Add the precomputed columns to commitedSVSIS or commitedSVNoSIS
		if ctx.IsSISAppliedToPrecomputed() {
			committedSVSIS = append(committedSVSIS, precomputedSV...)
		} else {
			committedSVNoSIS = append(committedSVNoSIS, precomputedSV...)
		}
	}

	// Collect all the committed polynomials : round by round
	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		// There are not included in the commitments so there
		// is no need to compute their linear combination.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}
		pols := ctx.getPols(pr, round)
		// Push pols to the right stack
		if ctx.RoundStatus[round] == IsOnlyMiMCApplied {
			committedSVNoSIS = append(committedSVNoSIS, pols...)
		} else if ctx.RoundStatus[round] == IsSISApplied {
			committedSVSIS = append(committedSVSIS, pols...)
		}
	}
	// Construct committedSV by stacking the No SIS round
	// matrices before the SIS round matrices
	committedSV := append(committedSVNoSIS, committedSVSIS...)

	// And get the randomness
	randomCoinLC := pr.GetRandomCoinField(ctx.Items.Alpha.Name)

	// and compute and assign the random linear combination of the rows
	proof := ctx.VortexParams.InitOpeningWithLC(committedSV, randomCoinLC)
	pr.AssignColumn(ctx.Items.Ualpha.GetColID(), proof.LinearCombination)
}

// ComputeLinearCombFromRsMatrix is the same as ComputeLinearComb but uses
// the RS encoded matrix instead of using the basic one. It is slower than
// the later but is recommended.
func (ctx *Ctx) ComputeLinearCombFromRsMatrix(run *wizard.ProverRuntime) {

	var (
		committedSVSIS   = []smartvectors.SmartVector{}
		committedSVNoSIS = []smartvectors.SmartVector{}
	)

	// Add the precomputed columns to commitedSVSIS or commitedSVNoSIS
	if ctx.IsSISAppliedToPrecomputed() {
		committedSVSIS = append(committedSVSIS, ctx.Items.Precomputeds.CommittedMatrix...)
	} else {
		committedSVNoSIS = append(committedSVNoSIS, ctx.Items.Precomputeds.CommittedMatrix...)
	}

	// Collect all the committed polynomials : round by round
	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		// There are not included in the commitments so there
		// is no need to proceed.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}

		committedMatrix := run.State.MustGet(ctx.VortexProverStateName(round)).(vortex.EncodedMatrix)

		// Push pols to the right stack
		if ctx.RoundStatus[round] == IsOnlyMiMCApplied {
			committedSVNoSIS = append(committedSVNoSIS, committedMatrix...)
		} else if ctx.RoundStatus[round] == IsSISApplied {
			committedSVSIS = append(committedSVSIS, committedMatrix...)
		}
	}

	// Construct committedSV by stacking the No SIS round
	// matrices before the SIS round matrices
	committedSV := append(committedSVNoSIS, committedSVSIS...)

	// And get the randomness
	randomCoinLC := run.GetRandomCoinField(ctx.Items.Alpha.Name)

	// and compute and assign the random linear combination of the rows
	proof := ctx.VortexParams.InitOpeningFromAlreadyEncodedLC(committedSV, randomCoinLC)

	run.AssignColumn(ctx.Items.Ualpha.GetColID(), proof.LinearCombination)
}

// Prover steps of Vortex where he opens the columns selected by the verifier
// We stack the no SIS round matrices before the SIS round matrices in the committed matrix stack.
// The same is done for the tree.
type OpenSelectedColumnsProverAction struct {
	*Ctx
}

func (ctx *OpenSelectedColumnsProverAction) Run(run *wizard.ProverRuntime) {

	var (
		committedMatricesSIS   = []vortex.EncodedMatrix{}
		committedMatricesNoSIS = []vortex.EncodedMatrix{}
		treesSIS               = []*smt.Tree{}
		treesNoSIS             = []*smt.Tree{}
		// We need them to assign the opened sis and non sis columns
		// to be used in the self-recursion compiler
		sisProof    = vortex.OpeningProof{}
		nonSisProof = vortex.OpeningProof{}
	)

	// Append the precomputed committedMatrices and trees to the SIS or no SIS matrices
	// or trees as per the number of precomputed columns are more than the [ApplyToSISThreshold]
	if ctx.IsNonEmptyPrecomputed() {
		if ctx.IsSISAppliedToPrecomputed() {
			committedMatricesSIS = append(committedMatricesSIS, ctx.Items.Precomputeds.CommittedMatrix)
			treesSIS = append(treesSIS, ctx.Items.Precomputeds.Tree)
		} else {
			committedMatricesNoSIS = append(committedMatricesNoSIS, ctx.Items.Precomputeds.CommittedMatrix)
			treesNoSIS = append(treesNoSIS, ctx.Items.Precomputeds.Tree)
		}
	}

	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		// There are not included in the commitments so there
		// is no need to proceed.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}
		// Fetch it from the state
		committedMatrix := run.State.MustGet(ctx.VortexProverStateName(round)).(vortex.EncodedMatrix)
		// and delete it because it won't be needed anymore and its very heavy
		run.State.Del(ctx.VortexProverStateName(round))

		// Also fetches the trees from the prover state
		tree := run.State.MustGet(ctx.MerkleTreeName(round)).(*smt.Tree)

		// conditionally stack the matrix and tree
		// to SIS or no SIS matrices and trees
		if ctx.RoundStatus[round] == IsOnlyMiMCApplied {
			committedMatricesNoSIS = append(committedMatricesNoSIS, committedMatrix)
			treesNoSIS = append(treesNoSIS, tree)
		} else if ctx.RoundStatus[round] == IsSISApplied {
			committedMatricesSIS = append(committedMatricesSIS, committedMatrix)
			treesSIS = append(treesSIS, tree)
		}
	}

	// Stack the no SIS matrices and trees before the SIS matrices and trees
	committedMatrices := append(committedMatricesNoSIS, committedMatricesSIS...)
	trees := append(treesNoSIS, treesSIS...)

	entryList := run.GetRandomCoinIntegerVec(ctx.Items.Q.Name)
	proof := vortex.OpeningProof{}

	// Amend the Vortex proof with the Merkle proofs and registers
	// the Merkle proofs in the prover runtime
	proof.Complete(entryList, committedMatrices, trees)

	selectedCols := proof.Columns

	// Assign the opened columns
	ctx.assignOpenedColumns(run, entryList, selectedCols, NonSelfRecursion)

	packedMProofs := ctx.packMerkleProofs(proof.MerkleProofs)
	run.AssignColumn(ctx.Items.MerkleProofs.GetColID(), packedMProofs)
	// Assign the SIS and non SIS selected columns.
	// They are not used in the Vortex compilers,
	// but are used in the self-recursion compilers.
	// But we need to assign them anyway as the self-recursion
	// compiler always runs after running the Vortex compiler

	// Handle SIS round
	if len(committedMatricesSIS) > 0 {
		sisProof.Complete(entryList, committedMatricesSIS, treesSIS)
		sisSelectedCols := sisProof.Columns
		// Assign the opened columns
		ctx.assignOpenedColumns(run, entryList, sisSelectedCols, SelfRecursionSIS)
	}
	// Handle non SIS round
	if len(committedMatricesNoSIS) > 0 {
		nonSisProof.Complete(entryList, committedMatricesNoSIS, treesNoSIS)
		nonSisSelectedCols := nonSisProof.Columns
		ctx.assignOpenedColumns(run, entryList, nonSisSelectedCols, SelfRecursionMiMCOnly)
		// Store the selected columns for the non sis round
		//  in the prover state
		ctx.storeSelectedColumnsForNonSisRounds(run, nonSisSelectedCols)
	}
}

// returns the list of all committed smartvectors for the given round
// so that we can commit to them
func (ctx *Ctx) getPols(run *wizard.ProverRuntime, round int) (pols []smartvectors.SmartVector) {
	names := ctx.CommitmentsByRounds.MustGet(round)
	pols = make([]smartvectors.SmartVector, len(names))
	for i := range names {
		pols[i] = run.Columns.MustGet(names[i])
	}
	return pols
}

// pack a list of merkle-proofs in a vector as in
func (ctx *Ctx) packMerkleProofs(proofs [][]smt.Proof) smartvectors.SmartVector {

	depth := len(proofs[0][0].Siblings) // depth of the Merkle-tree
	res := make([]field.Element, ctx.MerkleProofSize())
	numProofWritten := 0

	// Sanity-checks

	if depth != utils.Log2Ceil(ctx.NumEncodedCols()) {
		utils.Panic(
			"expected depth to be equal to Log2(NumEncodedCols()), got %v, %v",
			depth, utils.Log2Ceil(ctx.NumEncodedCols()),
		)
	}

	// When we commit to the precomputeds, len(proofs) = ctx.NumCommittedRounds + 1,
	// otherwise len(proofs) = ctx.NumCommittedRounds
	if len(proofs) != ctx.NumCommittedRounds() && !ctx.IsNonEmptyPrecomputed() {
		utils.Panic(
			"inconsitent proofs length %v, %v",
			len(proofs), ctx.NumCommittedRounds(),
		)
	}

	if len(proofs) != (ctx.NumCommittedRounds()+1) && ctx.IsNonEmptyPrecomputed() {
		utils.Panic(
			"inconsitent proofs length %v, %v",
			len(proofs), ctx.NumCommittedRounds()+1,
		)
	}

	if len(proofs[0]) != ctx.NbColsToOpen() {
		utils.Panic(
			"expected proofs[0] and NbColsToOpen to be equal: %v, %v",
			len(proofs[0]), ctx.NbColsToOpen(),
		)
	}

	for i := range proofs {
		for j := range proofs[i] {
			p := proofs[i][j]
			for k := range p.Siblings {
				// The proof stores the sibling bottom-up but
				// we want to pack the proof in top-down order.
				res[numProofWritten*depth+k].SetBytes(p.Siblings[depth-1-k][:])
			}
			numProofWritten++
		}
	}

	return smartvectors.NewRegular(res)
}

// unpack a list of merkle proofs from a vector as in
func (ctx *Ctx) unpackMerkleProofs(sv smartvectors.SmartVector, entryList []int) (proofs [][]smt.Proof) {

	depth := utils.Log2Ceil(ctx.NumEncodedCols()) // depth of the Merkle-tree
	numComs := ctx.NumCommittedRounds()
	if ctx.IsNonEmptyPrecomputed() {
		numComs = ctx.NumCommittedRounds() + 1 // Need to consider the precomputed commitments
	}
	numEntries := len(entryList)

	proofs = make([][]smt.Proof, numComs)
	curr := 0 // tracks the position in sv that we are parsing.
	for i := range proofs {
		proofs[i] = make([]smt.Proof, numEntries)
		for j := range proofs[i] {
			// initialize the proof that we are parsing
			proof := smt.Proof{
				Path:     entryList[j],
				Siblings: make([]types.Bytes32, depth),
			}

			// parse the siblings accounting for the fact that we
			// are inversing the order.
			for k := range proof.Siblings {
				v := sv.Get(curr)
				proof.Siblings[depth-k-1] = types.Bytes32(v.Bytes())
				curr++
			}

			proofs[i][j] = proof
		}
	}
	return proofs
}

// assignOpenedColumns assign the opened columns for
// both normal and self-recursion compilers
func (ctx *Ctx) assignOpenedColumns(
	pr *wizard.ProverRuntime,
	entryList []int,
	selectedCols [][][]field.Element,
	mode commitmentMode) {
	// The columns are split by commitment round. So we need to
	// restick them when we commit them.
	for j := range entryList {
		fullCol := []field.Element{}
		for i := range selectedCols {
			fullCol = append(fullCol, selectedCols[i][j]...)
		}

		// Converts it into a smart-vector and zero-pad it if necessary
		var assignable smartvectors.SmartVector = smartvectors.NewRegular(fullCol)
		if assignable.Len() < utils.NextPowerOfTwo(len(fullCol)) {
			assignable = smartvectors.RightZeroPadded(fullCol, utils.NextPowerOfTwo(len(fullCol)))
		}
		if mode == NonSelfRecursion {
			pr.AssignColumn(ctx.Items.OpenedColumns[j].GetColID(), assignable)
		} else if mode == SelfRecursionSIS {
			pr.AssignColumn(ctx.Items.OpenedSISColumns[j].GetColID(), assignable)
		} else if mode == SelfRecursionMiMCOnly {
			pr.AssignColumn(ctx.Items.OpenedNonSISColumns[j].GetColID(), assignable)
		}
	}

}

// storeSelectedColumnsForNonSisRound stores the selected columns in the prover state
// for the non SIS rounds which is to be used in the self-recursion compilers
func (ctx *Ctx) storeSelectedColumnsForNonSisRounds(
	pr *wizard.ProverRuntime,
	selectedCols [][][]field.Element) {
	numNonSisRound := ctx.NumCommittedRoundsNoSis()
	if ctx.IsNonEmptyPrecomputed() && !ctx.IsSISAppliedToPrecomputed() {
		numNonSisRound++
	}
	// selectedColsQ[i][j][k] stores the jth selected
	// column of the ith non SIS round
	selectedColsQ := make([][][]field.Element, numNonSisRound)
	// Sanity check
	if len(selectedCols) != numNonSisRound {
		utils.Panic(
			"expected selectedCols to be of length %v, got %v",
			numNonSisRound, len(selectedCols),
		)
	}
	for i := range selectedCols {
		// Sanity check
		if len(selectedCols[i]) != ctx.NbColsToOpen() {
			utils.Panic(
				"expected selectedCols[%v] to be of length %v, got %v",
				i, ctx.NbColsToOpen(), len(selectedCols[i]),
			)
		}
		selectedColsQ[i] = make([][]field.Element, ctx.NbColsToOpen())
		for j := range selectedCols[i] {
			selectedColsQ[i][j] = make([]field.Element, len(selectedCols[i][j]))
			copy(selectedColsQ[i][j], selectedCols[i][j])
		}
	}
	// Store the selected columns in the prover state
	pr.State.InsertNew(
		ctx.SelectedColumnNonSISName(),
		selectedColsQ)
}
