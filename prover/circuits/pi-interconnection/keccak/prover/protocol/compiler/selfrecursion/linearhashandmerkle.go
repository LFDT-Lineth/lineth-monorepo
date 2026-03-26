package selfrecursion

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/merkle"
	mimcW "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// LinearHashAndMerkle verifies the following things:
// 1. The MiMC hash of the SIS digest for the sis rounds are correctly computed
// 2. The MiMC hash of the selected columns for the non SIS rounds are correctly computed
// 3. The Merkle proofs are correctly verified for both SIS and non SIS rounds
// 4. The leaves of the SIS and non SIS rounds are consistent with the Merkle tree leaves
// used for the Merkle proof verification.
func (ctx *SelfRecursionCtx) LinearHashAndMerkle() {
	roundQ := ctx.Columns.Q.Round()
	// numRound denotes the total number of commitment rounds
	// including SIS and non SIS rounds
	numRound := ctx.VortexCtx.NumCommittedRounds()
	if ctx.VortexCtx.IsNonEmptyPrecomputed() {
		numRound += 1
	}
	// Next we consider the number of rounds for which we apply the SIS hash
	numRoundSis := ctx.VortexCtx.NumCommittedRoundsSis()
	// We increase numRoundSis by 1 if sis is applied to the precomputed
	if ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		numRoundSis += 1
	}
	// The number of non SIS rounds is the difference
	// between the total number of rounds and the number of SIS rounds.
	// It is after considering the precomputed round.
	numRoundNonSis := numRound - numRoundSis

	// The total SIS hash length = size of a single SIS hash *
	// total number of SIS hash per SIS round * number of SIS rounds
	concatDhQSizeUnpadded := ctx.VortexCtx.SisParams.OutputSize() * ctx.VortexCtx.NbColsToOpen() * numRoundSis
	concatDhQSize := utils.NextPowerOfTwo(concatDhQSizeUnpadded)

	// The leaves are computed for both SIS and non SIS rounds
	leavesSizeUnpadded := ctx.VortexCtx.NbColsToOpen() * numRound
	leavesSize := utils.NextPowerOfTwo(leavesSizeUnpadded)

	// The leaves size for SIS rounds
	sisRoundLeavesSizeUnpadded := ctx.VortexCtx.NbColsToOpen() * numRoundSis
	sisRoundLeavesSize := utils.NextPowerOfTwo(sisRoundLeavesSizeUnpadded)

	// The leaves size for non SIS rounds
	nonSisRoundLeavesSizeUnpadded := ctx.VortexCtx.NbColsToOpen() * numRoundNonSis

	ctx.Columns.MerkleProofsLeaves = ctx.Comp.InsertCommit(roundQ, ctx.merkleLeavesName(), leavesSize)
	ctx.Columns.MerkleProofPositions = ctx.Comp.InsertCommit(roundQ, ctx.merklePositionsName(), leavesSize)
	ctx.Columns.MerkleRoots = ctx.Comp.InsertCommit(roundQ, ctx.merkleRootsName(), leavesSize)

	// We commit to the below columns only if SIS is applied to any of the rounds including precomputed
	if ctx.VortexCtx.NumCommittedRoundsSis() > 0 || ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		ctx.Columns.ConcatenatedDhQ = ctx.Comp.InsertCommit(roundQ, ctx.concatenatedDhQ(), concatDhQSize)
		ctx.Columns.SisRoundLeaves = make([]ifaces.Column, 0, numRoundSis)
		for i := 0; i < numRoundSis; i++ {
			// Register the SIS round leaves
			ctx.Columns.SisRoundLeaves = append(ctx.Columns.SisRoundLeaves, ctx.Comp.InsertCommit(
				roundQ, ctx.sisRoundLeavesName(i), utils.NextPowerOfTwo(ctx.VortexCtx.NbColsToOpen())))
		}
	}

	// Register the linear hash columns for the non sis rounds
	var (
		mimcHashColumnSize      int
		mimcPreimageColumnsSize []int
	)
	if numRoundNonSis > 0 {
		// Register the linear hash columns for non sis rounds
		// If SIS is not applied to the precomputed, we consider
		// it to be the first non sis round
		ctx.MIMCMetaData.NonSisLeaves = make([]ifaces.Column, 0, numRoundNonSis)
		ctx.MIMCMetaData.ConcatenatedHashPreimages = make([]ifaces.Column, 0, numRoundNonSis)
		ctx.MIMCMetaData.ToHashSizes = make([]int, 0, numRoundNonSis)
		mimcHashColumnSize, mimcPreimageColumnsSize = ctx.registerMiMCMetaDataForNonSisRounds(numRoundNonSis, roundQ)
	}

	ctx.Comp.RegisterProverAction(roundQ, &LinearHashMerkleProverAction{
		Ctx:                           ctx,
		ConcatDhQSize:                 concatDhQSize,
		LeavesSize:                    leavesSize,
		LeavesSizeUnpadded:            leavesSizeUnpadded,
		SisRoundLeavesSize:            sisRoundLeavesSize,
		SisRoundLeavesSizeUnpadded:    sisRoundLeavesSizeUnpadded,
		NonSisRoundLeavesSizeUnpadded: nonSisRoundLeavesSizeUnpadded,
		NumNonSisRound:                numRoundNonSis,
		NumSisRound:                   numRoundSis,
		HashValuesSize:                mimcHashColumnSize,
		HashPreimagesSize:             mimcPreimageColumnsSize,
	})

	depth := utils.Log2Ceil(ctx.VortexCtx.NumEncodedCols())

	// The Merkle proof verification is for both sis and non sis rounds
	merkle.MerkleProofCheck(ctx.Comp, ctx.merkleProofVerificationName(), depth, leavesSizeUnpadded,
		ctx.Columns.MerkleProofs, ctx.Columns.MerkleRoots, ctx.Columns.MerkleProofsLeaves, ctx.Columns.MerkleProofPositions)

	// The below linear hash verification is for only sis rounds
	if ctx.VortexCtx.NumCommittedRoundsSis() > 0 || ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		cleanSisLeaves := make([]ifaces.Column, 0, numRoundSis)
		for i := 0; i < numRoundSis; i++ {
			cleanSisLeaves = append(cleanSisLeaves, ctx.Columns.SisRoundLeaves[i])
		}
		// We stack the sis round leaves
		stackedSisLeaves := dedicated.StackColumn(
			ctx.Comp,
			cleanSisLeaves,
			dedicated.HandleSourcePaddedColumns(ctx.VortexCtx.NbColsToOpen()))
		// Register the prover action for the stacked column
		if stackedSisLeaves.IsSourceColsArePadded {
			ctx.Comp.RegisterProverAction(roundQ, &dedicated.StackedColumn{
				Column:                stackedSisLeaves.Column,
				Source:                cleanSisLeaves,
				UnpaddedColumn:        stackedSisLeaves.UnpaddedColumn,
				ColumnFilter:          stackedSisLeaves.ColumnFilter,
				UnpaddedColumnFilter:  stackedSisLeaves.UnpaddedColumnFilter,
				UnpaddedSize:          stackedSisLeaves.UnpaddedSize,
				IsSourceColsArePadded: stackedSisLeaves.IsSourceColsArePadded,
			})
		} else {
			ctx.Comp.RegisterProverAction(roundQ, &dedicated.StackedColumn{
				Column: stackedSisLeaves.Column,
				Source: cleanSisLeaves,
			})
		}
		if stackedSisLeaves.IsSourceColsArePadded {
			mimcW.CheckLinearHash(ctx.Comp, ctx.linearHashVerificationName(), ctx.Columns.ConcatenatedDhQ,
				ctx.VortexCtx.SisParams.OutputSize(), sisRoundLeavesSizeUnpadded, *stackedSisLeaves.UnpaddedColumn)
		} else {
			mimcW.CheckLinearHash(ctx.Comp, ctx.linearHashVerificationName(), ctx.Columns.ConcatenatedDhQ,
				ctx.VortexCtx.SisParams.OutputSize(), sisRoundLeavesSizeUnpadded, stackedSisLeaves.Column)
		}
	}

	// Register the linear hash verification for the non sis rounds
	for i := 0; i < numRoundNonSis; i++ {
		mimcW.CheckLinearHash(ctx.Comp, ctx.nonSisRoundLinearHashVerificationName(i), ctx.MIMCMetaData.ConcatenatedHashPreimages[i],
			ctx.MIMCMetaData.ToHashSizes[i], ctx.VortexCtx.NbColsToOpen(), ctx.MIMCMetaData.NonSisLeaves[i])
	}

	// leafConsistency imposes fixed permutation constraints between the sis
	// and non sis rounds leaves with that of the merkle tree leaves.
	ctx.leafConsistency(roundQ)
}

// registerMiMCMetaDataForNonSisRounds registers the metadata for the
// for linear hash verification for the non SIS rounds
// and return the mimcHashColumnSize
// and the preimage column sizes per non sis round
func (ctx *SelfRecursionCtx) registerMiMCMetaDataForNonSisRounds(
	numRoundNonSis int, round int) (int, []int) {
	// Compute the concatenated hashes and preimages sizes
	var (
		mimcHashColumnSize      = utils.NextPowerOfTwo(ctx.VortexCtx.NbColsToOpen())
		mimcPreimageColumnsSize = make([]int, 0, numRoundNonSis)
	)

	// Consider the precomputed polynomials
	if ctx.VortexCtx.IsNonEmptyPrecomputed() && !ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		precompPreimageSize := utils.NextPowerOfTwo(
			ctx.VortexCtx.NbColsToOpen() *
				len(ctx.VortexCtx.Items.Precomputeds.PrecomputedColums))

		ctx.MIMCMetaData.NonSisLeaves = append(ctx.MIMCMetaData.NonSisLeaves,
			ctx.Comp.InsertCommit(
				round,
				ctx.concatenatedPrecomputedHashes(),
				mimcHashColumnSize,
			))

		ctx.MIMCMetaData.ConcatenatedHashPreimages = append(ctx.MIMCMetaData.ConcatenatedHashPreimages,
			ctx.Comp.InsertCommit(
				round,
				ctx.concatenatedPrecomputedPreimages(),
				precompPreimageSize,
			))
		mimcPreimageColumnsSize = append(mimcPreimageColumnsSize, precompPreimageSize)
		ctx.MIMCMetaData.ToHashSizes = append(ctx.MIMCMetaData.ToHashSizes, len(ctx.VortexCtx.Items.Precomputeds.PrecomputedColums))
	}

	// Next, consider only the non SIS rounds

	// firstNonEmptyRound and isPastFirstNonEmptyRound are used to determine the
	// first non-empty round for Vortex in the wizard. This is needed for
	// uniformity of the columns across different distributed segments. In
	// particular, we want the name of the columns to be the same regardless of
	// the starting round number.
	firstNonEmptyRound := 0
	isPastFirstNonEmptyRound := false

	for i := range ctx.VortexCtx.RoundStatus {

		if !isPastFirstNonEmptyRound && ctx.VortexCtx.RoundStatus[i] == vortex.IsEmpty {
			firstNonEmptyRound = i + 1
			continue
		}

		isPastFirstNonEmptyRound = true

		if ctx.VortexCtx.RoundStatus[i] == vortex.IsOnlyMiMCApplied {

			roundPreimageSize := utils.NextPowerOfTwo(
				ctx.VortexCtx.NbColsToOpen() *
					ctx.VortexCtx.GetNumPolsForNonSisRounds(i))

			ctx.MIMCMetaData.NonSisLeaves = append(
				ctx.MIMCMetaData.NonSisLeaves,
				ctx.Comp.InsertCommit(
					round,
					ctx.nonSisLeaves(i-firstNonEmptyRound),
					mimcHashColumnSize,
				))

			ctx.MIMCMetaData.ConcatenatedHashPreimages = append(ctx.MIMCMetaData.ConcatenatedHashPreimages, ctx.Comp.InsertCommit(
				round,
				ctx.concatenatedMIMCPreimages(i-firstNonEmptyRound),
				roundPreimageSize,
			))

			ctx.MIMCMetaData.ToHashSizes = append(ctx.MIMCMetaData.ToHashSizes, ctx.VortexCtx.GetNumPolsForNonSisRounds(i))
			mimcPreimageColumnsSize = append(mimcPreimageColumnsSize, roundPreimageSize)
		} else {
			continue
		}
	}
	return mimcHashColumnSize, mimcPreimageColumnsSize
}

func (ctx *SelfRecursionCtx) leafConsistency(round int) {
	// Fixed permutation constraint between the SIS and non SIS leaves
	// and the Merkle leaves
	// cleanLeaves = (nonSisLeaves || sisLeaves) is checked to be identical to
	// the Merkle leaves.
	var cleanLeaves []ifaces.Column
	if len(ctx.MIMCMetaData.NonSisLeaves) > 0 {
		cleanLeaves = append(cleanLeaves, ctx.MIMCMetaData.NonSisLeaves...)
	}
	if ctx.VortexCtx.NumCommittedRoundsSis() > 0 || ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		cleanLeaves = append(cleanLeaves, ctx.Columns.SisRoundLeaves...)
	}
	stackedCleanLeaves := dedicated.StackColumn(ctx.Comp,
		cleanLeaves,
		dedicated.HandleSourcePaddedColumns(ctx.VortexCtx.NbColsToOpen()))

	// Register prover action for the stacked column
	if stackedCleanLeaves.IsSourceColsArePadded {
		ctx.Comp.RegisterProverAction(round, &dedicated.StackedColumn{
			Column:                stackedCleanLeaves.Column,
			Source:                cleanLeaves,
			UnpaddedColumn:        stackedCleanLeaves.UnpaddedColumn,
			ColumnFilter:          stackedCleanLeaves.ColumnFilter,
			UnpaddedColumnFilter:  stackedCleanLeaves.UnpaddedColumnFilter,
			UnpaddedSize:          stackedCleanLeaves.UnpaddedSize,
			IsSourceColsArePadded: stackedCleanLeaves.IsSourceColsArePadded,
		})
	} else {
		ctx.Comp.RegisterProverAction(round, &dedicated.StackedColumn{
			Column: stackedCleanLeaves.Column,
			Source: cleanLeaves,
		})
	}

	// Next we compute the identity permutation
	s := make([]field.Element, stackedCleanLeaves.Column.Size())
	if stackedCleanLeaves.IsSourceColsArePadded {
		s = make([]field.Element, stackedCleanLeaves.UnpaddedColumn.Size())
	}
	for i := range s {
		s[i].SetInt64(int64(i))
	}
	s_smart := smartvectors.NewRegular(s)

	// Insert the fixed permutation constraint.
	if stackedCleanLeaves.IsSourceColsArePadded {
		ctx.Comp.InsertFixedPermutation(
			round,
			ctx.leafConsistencyName(),
			[]smartvectors.SmartVector{s_smart},
			[]ifaces.Column{*stackedCleanLeaves.UnpaddedColumn},
			[]ifaces.Column{ctx.Columns.MerkleProofsLeaves},
		)
	} else {
		ctx.Comp.InsertFixedPermutation(
			round,
			ctx.leafConsistencyName(),
			[]smartvectors.SmartVector{s_smart},
			[]ifaces.Column{stackedCleanLeaves.Column},
			[]ifaces.Column{ctx.Columns.MerkleProofsLeaves},
		)
	}

}

// Implements the prover action interface
type LinearHashMerkleProverAction struct {
	Ctx                           *SelfRecursionCtx
	ConcatDhQSize                 int
	LeavesSize                    int
	LeavesSizeUnpadded            int
	SisRoundLeavesSize            int
	SisRoundLeavesSizeUnpadded    int
	NonSisRoundLeavesSizeUnpadded int
	NumNonSisRound                int
	NumSisRound                   int
	HashValuesSize                int
	HashPreimagesSize             []int
}

// linearHashMerkleProverActionBuilder builds the assignment parameters
// of the prover action for the linear hash and merkle
type linearHashMerkleProverActionBuilder struct {
	// Contains the concatenated sis hashes of the selected columns
	// for the sis round matrices
	ConcatDhQ []field.Element
	// The leaves of the merkle tree (both sis and non sis)
	MerkleLeaves []field.Element
	// The positions of the leaves in the merkle tree
	MerklePositions []field.Element
	// The roots of the merkle tree
	MerkleRoots []field.Element
	// The merkle proofs are aligned as (non sis, sis).
	// Hence we need to align leaves, position and roots
	// in the same way. Meaning we need to store them separately
	// and append them later
	MerkleSisLeaves    []field.Element
	MerkleSisPositions []field.Element
	MerkleSisRoots     []field.Element
	// Now the non sis round values
	MerkleNonSisLeaves    []field.Element
	MerkleNonSisPositions []field.Element
	MerkleNonSisRoots     []field.Element
	// The leaves of the sis round matrices
	SisLeaves [][]field.Element
	// The leaves of the non sis round matrices.
	// NonSisLeaves[i][j] is leaf of the jth selected column
	// for the ith non sis round matrix
	NonSisLeaves [][]field.Element
	// The MiMC hash pre images of the
	// non sis round matrices, NonSisHashPreimages[i][j]
	// is the jth selected column for the ith non sis round matrix
	NonSisHashPreimages [][]field.Element
	// the size of the sis hash digest
	SisHashSize int
	// the number of opened/selected columns
	// per round
	NumOpenedCol int
	// total number of rounds
	TotalNumRounds int
	// the committed round offset
	CommittedRound int
}

// newLinearHashMerkleProverActionBuilder returns an empty
// linearHashMerkleProverActionBuilder
func newLinearHashMerkleProverActionBuilder(a *LinearHashMerkleProverAction) *linearHashMerkleProverActionBuilder {
	lmp := linearHashMerkleProverActionBuilder{}
	lmp.ConcatDhQ = make([]field.Element, a.SisRoundLeavesSizeUnpadded*a.Ctx.VortexCtx.SisParams.OutputSize())
	lmp.MerkleLeaves = make([]field.Element, 0, a.LeavesSizeUnpadded)
	lmp.MerklePositions = make([]field.Element, 0, a.LeavesSizeUnpadded)
	lmp.MerkleRoots = make([]field.Element, 0, a.LeavesSizeUnpadded)
	lmp.MerkleSisLeaves = make([]field.Element, 0, a.SisRoundLeavesSizeUnpadded)
	lmp.MerkleSisPositions = make([]field.Element, 0, a.SisRoundLeavesSizeUnpadded)
	lmp.MerkleSisRoots = make([]field.Element, 0, a.SisRoundLeavesSizeUnpadded)
	lmp.MerkleNonSisLeaves = make([]field.Element, 0, a.NonSisRoundLeavesSizeUnpadded)
	lmp.MerkleNonSisPositions = make([]field.Element, 0, a.NonSisRoundLeavesSizeUnpadded)
	lmp.MerkleNonSisRoots = make([]field.Element, 0, a.NonSisRoundLeavesSizeUnpadded)
	lmp.SisLeaves = make([][]field.Element, 0, a.NumSisRound)
	lmp.NonSisLeaves = make([][]field.Element, 0, a.NumNonSisRound)
	lmp.NonSisHashPreimages = make([][]field.Element, 0, a.NumNonSisRound)
	lmp.SisHashSize = a.Ctx.VortexCtx.SisParams.OutputSize()
	lmp.NumOpenedCol = a.Ctx.VortexCtx.NbColsToOpen()
	lmp.TotalNumRounds = a.Ctx.VortexCtx.MaxCommittedRound
	lmp.CommittedRound = 0
	return &lmp
}

// Run implements the prover action for the linear hash and merkle
func (a *LinearHashMerkleProverAction) Run(run *wizard.ProverRuntime) {
	openingIndices := run.GetRandomCoinIntegerVec(a.Ctx.Coins.Q.Name)
	lmp := newLinearHashMerkleProverActionBuilder(a)

	// Handle the precomputed round
	if a.Ctx.VortexCtx.IsNonEmptyPrecomputed() {
		processPrecomputedRound(a, lmp, run, openingIndices)
	}

	// Handle the SIS and non SIS rounds
	processRound(a, lmp, run, openingIndices)

	numCommittedRound := a.Ctx.VortexCtx.NumCommittedRounds()
	if a.Ctx.VortexCtx.IsNonEmptyPrecomputed() {
		numCommittedRound += 1
	}

	if lmp.CommittedRound != numCommittedRound {
		utils.Panic("Committed rounds %v does not match the total number of committed rounds %v", lmp.CommittedRound, numCommittedRound)
	}

	// Append non sis and sis round leaves, roots, and positions
	lmp.MerkleLeaves = append(lmp.MerkleNonSisLeaves, lmp.MerkleSisLeaves...)
	lmp.MerkleRoots = append(lmp.MerkleNonSisRoots, lmp.MerkleSisRoots...)
	lmp.MerklePositions = append(lmp.MerkleNonSisPositions, lmp.MerkleSisPositions...)

	// Assign columns using IDs from ctx.Columns
	run.AssignColumn(a.Ctx.Columns.MerkleProofsLeaves.GetColID(), smartvectors.RightZeroPadded(lmp.MerkleLeaves, a.LeavesSize))
	run.AssignColumn(a.Ctx.Columns.MerkleProofPositions.GetColID(), smartvectors.RightZeroPadded(lmp.MerklePositions, a.LeavesSize))
	run.AssignColumn(a.Ctx.Columns.MerkleRoots.GetColID(), smartvectors.RightZeroPadded(lmp.MerkleRoots, a.LeavesSize))
	// The below assignments are only done if SIS is applied to any of the rounds
	if a.Ctx.VortexCtx.NumCommittedRoundsSis() > 0 || a.Ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		// Assign the concatenated SIS hashes
		run.AssignColumn(a.Ctx.Columns.ConcatenatedDhQ.GetColID(), smartvectors.RightZeroPadded(lmp.ConcatDhQ, a.ConcatDhQSize))
		for i := 0; i < a.NumSisRound; i++ {
			// Assign the SIS round leaves
			run.AssignColumn(a.Ctx.Columns.SisRoundLeaves[i].GetColID(), smartvectors.NewRegular(lmp.SisLeaves[i]))
		}
	}

	// Assign the hash values and preimages for the non SIS rounds
	for i := 0; i < a.NumNonSisRound; i++ {
		run.AssignColumn(a.Ctx.MIMCMetaData.NonSisLeaves[i].GetColID(), smartvectors.RightZeroPadded(lmp.NonSisLeaves[i], a.HashValuesSize))
		run.AssignColumn(a.Ctx.MIMCMetaData.ConcatenatedHashPreimages[i].GetColID(), smartvectors.RightZeroPadded(lmp.NonSisHashPreimages[i], a.HashPreimagesSize[i]))
	}
}

// processPrecomputedRound processes the precomputed polynomials
// assignments for the linear hash and merkle tree prover action
func processPrecomputedRound(
	a *LinearHashMerkleProverAction,
	lmp *linearHashMerkleProverActionBuilder,
	run *wizard.ProverRuntime,
	openingIndices []int,
) {
	// The merkle root for the precomputed round
	rootPrecomp := a.Ctx.Columns.PrecompRoot.GetColAssignment(run).Get(0)
	if a.Ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		precompColSisHash := a.Ctx.VortexCtx.Items.Precomputeds.DhWithMerkle
		precompSisLeaves := make([]field.Element, 0, len(openingIndices))
		for i, selectedCol := range openingIndices {
			srcStart := selectedCol * lmp.SisHashSize
			destStart := i * lmp.SisHashSize
			sisHash := precompColSisHash[srcStart : srcStart+lmp.SisHashSize]
			copy(lmp.ConcatDhQ[destStart:destStart+lmp.SisHashSize], sisHash)
			leaf := mimc.HashVec(sisHash)
			lmp.MerkleSisLeaves = append(lmp.MerkleSisLeaves, leaf)
			precompSisLeaves = append(precompSisLeaves, leaf)
			lmp.MerkleSisRoots = append(lmp.MerkleSisRoots, rootPrecomp)
			lmp.MerkleSisPositions = append(lmp.MerkleSisPositions, field.NewElement(uint64(selectedCol)))
		}
		// make the size of the precompSisLeaves a power of two
		precompSisLeaves = rightPadWithZero(precompSisLeaves)
		lmp.SisLeaves = append(lmp.SisLeaves, precompSisLeaves)
		lmp.CommittedRound++
		lmp.TotalNumRounds++
	} else {
		precompColMiMCHash := a.Ctx.VortexCtx.Items.Precomputeds.DhWithMerkle
		precompMimcHashValues := make([]field.Element, 0, lmp.NumOpenedCol)
		precompMimcHashPreimages := make([]field.Element, 0, lmp.NumOpenedCol*len(a.Ctx.VortexCtx.Items.Precomputeds.PrecomputedColums))
		for _, selectedCol := range openingIndices {
			srcStart := selectedCol
			// MiMC hash is a single value
			mimcHash := precompColMiMCHash[srcStart : srcStart+1]
			leaf := mimcHash[0]
			mimcPreimage := a.Ctx.VortexCtx.GetPrecomputedSelectedCol(selectedCol)
			// Also compute the leaf from the column
			// to check sanity
			leaf_ := mimc.HashVec(mimcPreimage)
			if leaf != leaf_ {
				utils.Panic("MiMC hash of the precomputed column %v does not match the leaf %v", leaf_, leaf)
			}
			lmp.MerkleNonSisLeaves = append(lmp.MerkleNonSisLeaves, leaf)
			lmp.MerkleNonSisRoots = append(lmp.MerkleNonSisRoots, rootPrecomp)
			lmp.MerkleNonSisPositions = append(lmp.MerkleNonSisPositions, field.NewElement(uint64(selectedCol)))
			precompMimcHashValues = append(precompMimcHashValues, leaf)
			precompMimcHashPreimages = append(precompMimcHashPreimages, mimcPreimage...)
		}
		// Append the hash values and preimages
		lmp.NonSisLeaves = append(lmp.NonSisLeaves, precompMimcHashValues)
		lmp.NonSisHashPreimages = append(lmp.NonSisHashPreimages, precompMimcHashPreimages)
		lmp.CommittedRound++
		lmp.TotalNumRounds++
	}
}

// processRound processes the round assignements
// for the linear hash and merkle tree prover action
func processRound(
	a *LinearHashMerkleProverAction,
	lmp *linearHashMerkleProverActionBuilder,
	run *wizard.ProverRuntime,
	openingIndices []int,
) {
	// If there are non SIS rounds, we need to fetch the
	// non SIS opened columns
	var (
		nonSisOpenedCols     [][][]field.Element
		nonSisOpenedColsName string
		nonSisRoundCount     = 0
		sisRoundCount        = 0
	)
	if a.NumNonSisRound > 0 {
		nonSisOpenedColsName = a.Ctx.VortexCtx.SelectedColumnNonSISName()
		nonSisOpenedColsSV, found := run.State.TryGet(nonSisOpenedColsName)
		if !found {
			utils.Panic("nonSisOpenedColsName %v not found", nonSisOpenedColsName)
		}
		nonSisOpenedCols = nonSisOpenedColsSV.([][][]field.Element)
		// Note nonSisOpenedCols contains the precomputed columns also if
		// SIS is not applied to the precomputed.
		// However, we already have it at the time of processing
		// the precomputed round, so we need to exclude it
		if a.Ctx.VortexCtx.IsNonEmptyPrecomputed() && !a.Ctx.VortexCtx.IsSISAppliedToPrecomputed() {
			nonSisOpenedCols = nonSisOpenedCols[1:]
		}
	}
	// If SIS is applied to the precomputed, we need to
	// increase the sisRoundCount by 1
	if a.Ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		sisRoundCount++
	}

	// The SIS and non SIS rounds are processed
	numRound := lmp.TotalNumRounds
	// We need to decrease the number of rounds by 1
	// as precomputed round is considered seperately
	if a.Ctx.VortexCtx.IsNonEmptyPrecomputed() {
		numRound -= 1
	}
	for round := 0; round <= numRound; round++ {
		if a.Ctx.VortexCtx.RoundStatus[round] == vortex.IsSISApplied {
			colSisHashName := a.Ctx.VortexCtx.SisHashName(round)
			colSisHashSV, found := run.State.TryGet(colSisHashName)
			if !found {
				utils.Panic("colSisHashName %v not found", colSisHashName)
			}

			rooth := a.Ctx.Columns.Rooth[round].GetColAssignment(run).Get(0)
			colSisHash := colSisHashSV.([]field.Element)

			sisRoundLeaves := make([]field.Element, 0, lmp.NumOpenedCol)
			for i, selectedCol := range openingIndices {
				srcStart := selectedCol * lmp.SisHashSize
				destStart := sisRoundCount*lmp.NumOpenedCol*lmp.SisHashSize + i*lmp.SisHashSize
				sisHash := colSisHash[srcStart : srcStart+lmp.SisHashSize]
				copy(lmp.ConcatDhQ[destStart:destStart+lmp.SisHashSize], sisHash)
				leaf := mimc.HashVec(sisHash)
				lmp.MerkleSisLeaves = append(lmp.MerkleSisLeaves, leaf)
				sisRoundLeaves = append(sisRoundLeaves, leaf)
				lmp.MerkleSisRoots = append(lmp.MerkleSisRoots, rooth)
				lmp.MerkleSisPositions = append(lmp.MerkleSisPositions, field.NewElement(uint64(selectedCol)))
			}
			// Make the size of the sisRoundLeaves a power of two
			sisRoundLeaves = rightPadWithZero(sisRoundLeaves)
			// Append the sis leaves
			lmp.SisLeaves = append(lmp.SisLeaves, sisRoundLeaves)
			sisRoundCount++
			run.State.TryDel(colSisHashName)
			lmp.CommittedRound++
		} else if a.Ctx.VortexCtx.RoundStatus[round] == vortex.IsOnlyMiMCApplied {
			// Fetch the MiMC hash values
			colMimcHashName := a.Ctx.VortexCtx.MIMCHashName(round)
			colMimcHashSV, found := run.State.TryGet(colMimcHashName)
			if !found {
				utils.Panic("colMimcHashName %v not found", colMimcHashName)
			}
			colMimcHash := colMimcHashSV.([]field.Element)

			// Fetch the root for the round
			rooth := a.Ctx.Columns.Rooth[round].GetColAssignment(run).Get(0)
			mimcHashValues := make([]field.Element, 0, lmp.NumOpenedCol)
			mimcHashPreimages := make([]field.Element, 0, lmp.NumOpenedCol*a.Ctx.VortexCtx.GetNumPolsForNonSisRounds(round))
			for i, selectedCol := range openingIndices {
				srcStart := selectedCol
				// MiMC hash is a single value
				mimcHash := colMimcHash[srcStart : srcStart+1]
				mimcPreimage := nonSisOpenedCols[nonSisRoundCount][i]
				leaf := mimcHash[0]
				// Also compute the leaf from the column
				// to check sanity
				leaf_ := mimc.HashVec(mimcPreimage)
				if leaf != leaf_ {
					utils.Panic("MiMC hash of the non SIS column %v does not match the leaf %v", leaf_, leaf)
				}
				lmp.MerkleNonSisLeaves = append(lmp.MerkleNonSisLeaves, leaf)
				lmp.MerkleNonSisRoots = append(lmp.MerkleNonSisRoots, rooth)
				lmp.MerkleNonSisPositions = append(lmp.MerkleNonSisPositions, field.NewElement(uint64(selectedCol)))
				mimcHashValues = append(mimcHashValues, leaf)
				mimcHashPreimages = append(mimcHashPreimages, mimcPreimage...)
			}
			// Append the hash values and preimages
			lmp.NonSisLeaves = append(lmp.NonSisLeaves, mimcHashValues)
			lmp.NonSisHashPreimages = append(lmp.NonSisHashPreimages, mimcHashPreimages)
			run.State.TryDel(colMimcHashName)
			lmp.CommittedRound++
			nonSisRoundCount++
		} else if a.Ctx.VortexCtx.RoundStatus[round] == vortex.IsEmpty {
			continue
		}
	}
	// We can delete the non SIS opened columns now
	run.State.TryDel(nonSisOpenedColsName)
}

// rightPadWithZero pads the input slice with zeroes
// to make its size a power of two
func rightPadWithZero(input []field.Element) []field.Element {
	size := len(input)
	if utils.IsPowerOfTwo(size) {
		return input
	}
	paddedSize := utils.NextPowerOfTwo(size)
	padded := make([]field.Element, paddedSize)
	copy(padded, input)
	return padded
}
