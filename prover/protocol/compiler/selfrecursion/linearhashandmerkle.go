package selfrecursion

import (
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/merkle"
	poseidon2W "github.com/consensys/linea-monorepo/prover/protocol/dedicated/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// LinearHashAndMerkle verifies the following things:
// 1. The Poseidon2 hash of the SIS digest for the sis rounds are correctly computed
// 2. The Poseidon2 hash of the selected columns for the non SIS rounds are correctly computed
// 3. The Merkle proofs are correctly verified for both SIS and non SIS rounds
// 4. The leaves of the SIS and non SIS rounds are consistent with the Merkle tree leaves
// used for the Merkle proof verification.
func (ctx *SelfRecursionCtx) LinearHashAndMerkle() {
	roundQ := ctx.Columns.Q.Round()

	// Calculate round parameters
	//
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
	// between the total number of rounds and the number of SIS rounds
	// It is after considering the precomputed round
	numRoundNonSis := numRound - numRoundSis

	// CalculateSISParameters computes SIS-related parameters
	sisLeavesSizeUnpadded := ctx.VortexCtx.NbColsToOpen() * numRoundSis
	sisLeavesSize := utils.NextPowerOfTwo(sisLeavesSizeUnpadded)
	// sis hash is also the preimage of poseidon2 hash, poseidon2 hash takes every 8 elements as input at a time
	// sisHashColChunks stores the number of poseidon2 hashes computed for one sis hash column
	sisHashColSize := ctx.VortexCtx.VortexKoalaParams.Key.OutputSize()
	sisHashColChunks := (sisHashColSize + blockSize - 1) / blockSize
	sisHashTotalChunksUnpadded := sisHashColChunks * sisLeavesSizeUnpadded
	sisHashTotalChunks := utils.NextPowerOfTwo(sisHashTotalChunksUnpadded)
	// concated sis hash
	concatSisHashQSizeUnpadded := sisHashColSize * sisLeavesSizeUnpadded
	concatSisHashQSize := utils.NextPowerOfTwo(concatSisHashQSizeUnpadded)

	// CalculateMerkleParameters computes Merkle tree parameters
	// The leaves are computed for both SIS and non SIS rounds
	leavesSizeUnpadded := ctx.VortexCtx.NbColsToOpen() * numRound
	leavesSize := utils.NextPowerOfTwo(leavesSizeUnpadded)

	// The leaves size for non SIS rounds
	nonSisRoundLeavesSizeUnpadded := ctx.VortexCtx.NbColsToOpen() * numRoundNonSis

	// Register Merkle-related columns
	ctx.Columns.MerkleProofPositions = ctx.Comp.InsertCommit(roundQ, ctx.merklePositionsName(), leavesSize, true)
	for i := 0; i < blockSize; i++ {
		ctx.Columns.MerkleProofsLeaves[i] = ctx.Comp.InsertCommit(roundQ, ctx.merkleLeavesName(i), leavesSize, true)
		ctx.Columns.MerkleRoots[i] = ctx.Comp.InsertCommit(roundQ, ctx.merkleRootsName(i), leavesSize, true)
	}

	// Register SIS-related columns if needed
	// We commit to the below columns only if SIS is applied to any of the rounds including precomputed
	if ctx.VortexCtx.NumCommittedRoundsSis() > 0 || ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		ctx.Columns.ConcatenatedSisHashQ = ctx.Comp.InsertCommit(roundQ, ctx.ConcatenatedSisHashQ(), concatSisHashQSize, true)
		for j := 0; j < blockSize; j++ {
			ctx.Columns.SisHashToHash[j] = ctx.Comp.InsertCommit(roundQ, ctx.SisHashToHash(j), sisHashTotalChunks, true)
			ctx.Columns.SisRoundLeaves[j] = make([]ifaces.Column, 0, numRoundSis)
			for i := 0; i < numRoundSis; i++ {
				// Register the SIS round leaves
				ctx.Columns.SisRoundLeaves[j] = append(ctx.Columns.SisRoundLeaves[j], ctx.Comp.InsertCommit(
					roundQ, ctx.sisRoundLeavesName(i, j), utils.NextPowerOfTwo(ctx.VortexCtx.NbColsToOpen()), true))
			}
		}
	}

	// Register the linear hash columns for the non sis rounds
	var (
		nonSisLeavesColumnSize    int
		nonSisPreimageColumnsSize []int
	)
	if numRoundNonSis > 0 {
		// Register the linear hash columns for non sis rounds
		// If SIS is not applied to the precomputed, we consider
		// it to be the first non sis round
		for j := 0; j < blockSize; j++ {
			ctx.NonSisMetaData.NonSisLeaves[j] = make([]ifaces.Column, 0, numRoundNonSis)
		}
		ctx.NonSisMetaData.NonHashPreimages = make([][blockSize]ifaces.Column, 0, numRoundNonSis)
		ctx.NonSisMetaData.ColChunks = make([]int, 0, numRoundNonSis)
		ctx.NonSisMetaData.ToHashSizes = make([]int, 0, numRoundNonSis)
		nonSisLeavesColumnSize, nonSisPreimageColumnsSize = ctx.registerNonSisMetaDataForNonSisRounds(numRoundNonSis, roundQ)
	}

	ctx.Comp.RegisterProverAction(roundQ, &LinearHashMerkleProverAction{
		Ctx:                           ctx,
		LeavesSize:                    leavesSize,
		LeavesSizeUnpadded:            leavesSizeUnpadded,
		ConcatSisHashQSize:            concatSisHashQSize,
		SisLeavesSize:                 sisLeavesSize,
		SisLeavesSizeUnpadded:         sisLeavesSizeUnpadded,
		SisHashTotalChunksUnpadded:    sisHashTotalChunksUnpadded,
		SisHashTotalChunks:            sisHashTotalChunks,
		NumOpenedCol:                  ctx.VortexCtx.NbColsToOpen(),
		NonSisRoundLeavesSizeUnpadded: nonSisRoundLeavesSizeUnpadded,
		NumRoundNonSis:                numRoundNonSis,
		NumRoundSis:                   numRoundSis,
		NonSisLeavesColumnSize:        nonSisLeavesColumnSize,
		NonSisPreimageColumnsSize:     nonSisPreimageColumnsSize,
	})

	depth := utils.Log2Ceil(ctx.VortexCtx.NumEncodedCols())

	// The Merkle proof verification is for both sis and non sis rounds
	merkle.MerkleProofCheck(ctx.Comp, ctx.merkleProofVerificationName(), depth, leavesSizeUnpadded, ctx.Columns.MerkleProofPositions,
		ctx.Columns.MerkleProofs, ctx.Columns.MerkleRoots, ctx.Columns.MerkleProofsLeaves)

	// The below linear hash verification is for only sis rounds
	if ctx.VortexCtx.NumCommittedRoundsSis() > 0 || ctx.VortexCtx.IsSISAppliedToPrecomputed() {

		var cleanSisLeaves [blockSize][]ifaces.Column
		var stackedSisLeaves [blockSize]*dedicated.StackedColumn
		var expectedHash [blockSize]ifaces.Column

		for j := 0; j < blockSize; j++ {
			cleanSisLeaves[j] = make([]ifaces.Column, 0, numRoundSis)
			for i := 0; i < numRoundSis; i++ {
				cleanSisLeaves[j] = append(cleanSisLeaves[j], ctx.Columns.SisRoundLeaves[j][i])
			}
			// We stack the sis round leaves
			stackedSisLeaves[j] = dedicated.StackColumn(ctx.Comp, cleanSisLeaves[j], dedicated.HandleSourcePaddedColumns(ctx.VortexCtx.NbColsToOpen()))
			// Register the prover action for the stacked column
			if stackedSisLeaves[j].AreSourceColsPadded {
				ctx.Comp.RegisterProverAction(roundQ, &dedicated.StackedColumn{
					Column:               stackedSisLeaves[j].Column,
					Source:               cleanSisLeaves[j],
					UnpaddedColumn:       stackedSisLeaves[j].UnpaddedColumn,
					ColumnFilter:         stackedSisLeaves[j].ColumnFilter,
					UnpaddedColumnFilter: stackedSisLeaves[j].UnpaddedColumnFilter,
					UnpaddedSize:         stackedSisLeaves[j].UnpaddedSize,
					AreSourceColsPadded:  stackedSisLeaves[j].AreSourceColsPadded,
				})
				// expected hash should be the unpadded column when source cols are padded
				expectedHash[j] = *stackedSisLeaves[j].UnpaddedColumn
			} else {
				ctx.Comp.RegisterProverAction(roundQ, &dedicated.StackedColumn{
					Column: stackedSisLeaves[j].Column,
					Source: cleanSisLeaves[j],
				})
				expectedHash[j] = stackedSisLeaves[j].Column
			}

		}
		// SisLeaves have the same size of preimages each rounds for linear hash (sisHashColSize), so we can concatenate them together and check once
		poseidon2W.CheckLinearHash(ctx.Comp, ctx.linearHashVerificationName(), sisHashColChunks, ctx.Columns.SisHashToHash,
			sisLeavesSizeUnpadded, expectedHash)
	}

	// Register the linear hash verification for the non sis rounds
	for i := 0; i < numRoundNonSis; i++ {
		var expectedHash [blockSize]ifaces.Column
		for j := 0; j < blockSize; j++ {
			expectedHash[j] = ctx.NonSisMetaData.NonSisLeaves[j][i]
		}

		// NonSisLeaves may have different size of preimages each rounds for linear hash, so we need to check them round by round
		poseidon2W.CheckLinearHash(ctx.Comp, ctx.nonSisRoundLinearHashVerificationName(i), ctx.NonSisMetaData.ColChunks[i], ctx.NonSisMetaData.NonHashPreimages[i],
			ctx.VortexCtx.NbColsToOpen(), expectedHash)
	}

	// leafConsistency imposes fixed permutation constraints between the sis
	// and non sis rounds leaves with that of the merkle tree leaves.
	ctx.leafConsistency(roundQ)
}

// registerNonSisMetaDataForNonSisRounds registers the metadata for the
// for linear hash verification for the non SIS rounds
// and return the numLeavesColumnSize
// and the preimage column sizes per non sis round
func (ctx *SelfRecursionCtx) registerNonSisMetaDataForNonSisRounds(
	numRoundNonSis int, round int) (int, []int) {
	// Compute the concatenated hashes and preimages sizes
	var (
		numLeavesUnpadded = ctx.VortexCtx.NbColsToOpen()
		numLeaves         = utils.NextPowerOfTwo(numLeavesUnpadded)
		preimageSize      = make([]int, 0, numRoundNonSis)
	)

	// Consider the precomputed polynomials
	if ctx.VortexCtx.IsNonEmptyPrecomputed() && !ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		colSize := len(ctx.VortexCtx.Items.Precomputeds.PrecomputedColums)
		colChunks := (colSize + blockSize - 1) / blockSize
		precompPreimageChunksSizeUnpadded := colChunks * numLeavesUnpadded
		precompPreimageChunksSize := utils.NextPowerOfTwo(precompPreimageChunksSizeUnpadded)

		// Leaves are the expected hashes
		for j := 0; j < blockSize; j++ {
			ctx.NonSisMetaData.NonSisLeaves[j] = append(ctx.NonSisMetaData.NonSisLeaves[j],
				ctx.Comp.InsertCommit(
					round,
					ctx.nonSisPrecomputedLeaves(j),
					numLeaves,
					true,
				))
		}

		var hashPreimages [blockSize]ifaces.Column
		for j := 0; j < blockSize; j++ {
			hashPreimages[j] = ctx.Comp.InsertCommit(
				round,
				ctx.nonSisPrecomputedPreimages(j),
				precompPreimageChunksSize,
				true,
			)
		}
		ctx.NonSisMetaData.NonHashPreimages = append(ctx.NonSisMetaData.NonHashPreimages,
			hashPreimages)
		preimageSize = append(preimageSize, precompPreimageChunksSize)
		ctx.NonSisMetaData.ColChunks = append(ctx.NonSisMetaData.ColChunks, colChunks)
		ctx.NonSisMetaData.ToHashSizes = append(ctx.NonSisMetaData.ToHashSizes, colSize)

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

		if ctx.VortexCtx.RoundStatus[i] == vortex.IsNoSis {
			colSize := ctx.VortexCtx.GetNumPolsForNonSisRounds(i)
			colChunks := (colSize + blockSize - 1) / blockSize
			preimageChunksSizeUnpadded := colChunks * numLeavesUnpadded
			preimageChunksSize := utils.NextPowerOfTwo(preimageChunksSizeUnpadded)

			for j := 0; j < blockSize; j++ {
				ctx.NonSisMetaData.NonSisLeaves[j] = append(
					ctx.NonSisMetaData.NonSisLeaves[j],
					ctx.Comp.InsertCommit(
						round,
						ctx.nonSisLeaves(i-firstNonEmptyRound, j),
						numLeaves,
						true,
					))

			}

			var hashPreimages [blockSize]ifaces.Column
			for j := 0; j < blockSize; j++ {
				hashPreimages[j] = ctx.Comp.InsertCommit(
					round,
					ctx.nonSisPreimages(i-firstNonEmptyRound, j),
					preimageChunksSize,
					true,
				)
			}
			ctx.NonSisMetaData.NonHashPreimages = append(ctx.NonSisMetaData.NonHashPreimages, hashPreimages)
			ctx.NonSisMetaData.ColChunks = append(ctx.NonSisMetaData.ColChunks, colChunks)
			ctx.NonSisMetaData.ToHashSizes = append(ctx.NonSisMetaData.ToHashSizes, colSize)

			preimageSize = append(preimageSize, preimageChunksSize)
		} else {
			continue
		}
	}

	return numLeaves, preimageSize
}

func (ctx *SelfRecursionCtx) leafConsistency(round int) {
	// Fixed permutation constraint between the SIS and non SIS leaves
	// and the Merkle leaves
	// cleanLeaves = (nonSisLeaves || sisLeaves) is checked to be identical to
	// the Merkle leaves.
	var cleanLeaves [blockSize][]ifaces.Column
	var stackedCleanLeaves [blockSize]*dedicated.StackedColumn
	var s [blockSize][]field.Element
	for i := 0; i < blockSize; i++ {
		if len(ctx.NonSisMetaData.NonSisLeaves[i]) > 0 {
			cleanLeaves[i] = append(cleanLeaves[i], ctx.NonSisMetaData.NonSisLeaves[i]...)
		}
		if ctx.VortexCtx.NumCommittedRoundsSis() > 0 || ctx.VortexCtx.IsSISAppliedToPrecomputed() {
			cleanLeaves[i] = append(cleanLeaves[i], ctx.Columns.SisRoundLeaves[i]...)
		}
		// Handle possibly non-power-of-two number of opened columns by
		// informing the stacker that source columns are padded
		stackedCleanLeaves[i] = dedicated.StackColumn(ctx.Comp, cleanLeaves[i], dedicated.HandleSourcePaddedColumns(ctx.VortexCtx.NbColsToOpen()))

		// Register prover action for the stacked column with appropriate fields
		if stackedCleanLeaves[i].AreSourceColsPadded {
			ctx.Comp.RegisterProverAction(round, &dedicated.StackedColumn{
				Column:               stackedCleanLeaves[i].Column,
				Source:               cleanLeaves[i],
				UnpaddedColumn:       stackedCleanLeaves[i].UnpaddedColumn,
				ColumnFilter:         stackedCleanLeaves[i].ColumnFilter,
				UnpaddedColumnFilter: stackedCleanLeaves[i].UnpaddedColumnFilter,
				UnpaddedSize:         stackedCleanLeaves[i].UnpaddedSize,
				AreSourceColsPadded:  stackedCleanLeaves[i].AreSourceColsPadded,
			})
		} else {
			ctx.Comp.RegisterProverAction(round, &dedicated.StackedColumn{
				Column: stackedCleanLeaves[i].Column,
				Source: cleanLeaves[i],
			})
		}

		// Next we compute the identity permutation
		if stackedCleanLeaves[i].AreSourceColsPadded {
			s[i] = make([]field.Element, stackedCleanLeaves[i].UnpaddedColumn.Size())
		} else {
			s[i] = make([]field.Element, stackedCleanLeaves[i].Column.Size())
		}
		for j := range s[i] {
			s[i][j].SetInt64(int64(j))
		}
		s_smart := smartvectors.NewRegular(s[i])

		// Insert the fixed permutation constraint using unpadded column if available
		if stackedCleanLeaves[i].AreSourceColsPadded {
			ctx.Comp.InsertFixedPermutation(
				round,
				ctx.leafConsistencyName(i),
				[]smartvectors.SmartVector{s_smart},
				[]ifaces.Column{*stackedCleanLeaves[i].UnpaddedColumn},
				[]ifaces.Column{ctx.Columns.MerkleProofsLeaves[i]},
			)
		} else {
			ctx.Comp.InsertFixedPermutation(
				round,
				ctx.leafConsistencyName(i),
				[]smartvectors.SmartVector{s_smart},
				[]ifaces.Column{stackedCleanLeaves[i].Column},
				[]ifaces.Column{ctx.Columns.MerkleProofsLeaves[i]},
			)
		}
	}

}

// Implements the prover action interface
type LinearHashMerkleProverAction struct {
	Ctx                           *SelfRecursionCtx
	ConcatSisHashQSize            int
	SisLeavesSize                 int
	SisHashTotalChunksUnpadded    int
	LeavesSize                    int
	LeavesSizeUnpadded            int
	SisHashTotalChunks            int
	SisLeavesSizeUnpadded         int
	NumOpenedCol                  int
	NonSisRoundLeavesSizeUnpadded int
	NumRoundNonSis                int
	NumRoundSis                   int
	NonSisLeavesColumnSize        int
	NonSisPreimageColumnsSize     []int
}

// linearHashMerkleProverActionBuilder builds the assignment parameters
// of the prover action for the linear hash and merkle
type linearHashMerkleProverActionBuilder struct {
	// Contains the concatenated sis hashes of the selected columns
	// for the sis round matrices
	ConcatSisHashQ []field.Element
	// The sis hash to be hashed by poseidon2, store in 8 columns as input of poseidon2
	SisHashToHash [blockSize][]field.Element
	// The leaves of the merkle tree (both sis and non sis)
	MerkleLeaves [blockSize][]field.Element
	// The positions of the leaves in the merkle tree
	MerklePositions []field.Element
	// The roots of the merkle tree
	MerkleRoots [blockSize][]field.Element
	// The merkle proofs are aligned as (non sis, sis).
	// Hence we need to align leaves, position and roots
	// in the same way. Meaning we need to store them separately
	// and append them later
	MerkleSisLeaves    [blockSize][]field.Element
	MerkleSisPositions []field.Element
	MerkleSisRoots     [blockSize][]field.Element
	// Now the non sis round values
	MerkleNonSisLeaves    [blockSize][]field.Element
	MerkleNonSisPositions []field.Element
	MerkleNonSisRoots     [blockSize][]field.Element
	// The leaves of the sis round matrices
	SisLeaves [][blockSize][]field.Element
	// The leaves of the non sis round matrices.
	// NonSisLeaves[i][j] is leaf of the jth selected column
	// for the ith non sis round matrix
	NonSisLeaves [][blockSize][]field.Element
	// The Poseidon2 hash pre images of the
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
	SisTotalChunks int
	// the committed round offset
	CommittedRound int
}

// newLinearHashMerkleProverActionBuilder returns an empty
// linearHashMerkleProverActionBuilder
func newLinearHashMerkleProverActionBuilder(a *LinearHashMerkleProverAction) *linearHashMerkleProverActionBuilder {
	lmp := linearHashMerkleProverActionBuilder{}
	lmp.ConcatSisHashQ = make([]field.Element, a.SisLeavesSizeUnpadded*a.Ctx.VortexCtx.VortexKoalaParams.Key.OutputSize())
	lmp.MerklePositions = make([]field.Element, 0, a.LeavesSizeUnpadded)
	lmp.MerkleSisPositions = make([]field.Element, 0, a.SisLeavesSizeUnpadded)
	lmp.MerkleNonSisPositions = make([]field.Element, 0, a.NonSisRoundLeavesSizeUnpadded)
	lmp.NonSisHashPreimages = make([][]field.Element, 0, a.NumRoundNonSis)
	lmp.SisHashSize = a.Ctx.VortexCtx.VortexKoalaParams.Key.OutputSize()
	lmp.NumOpenedCol = a.Ctx.VortexCtx.NbColsToOpen()
	lmp.TotalNumRounds = a.Ctx.VortexCtx.MaxCommittedRound
	lmp.CommittedRound = 0
	lmp.SisTotalChunks = a.SisHashTotalChunksUnpadded

	for i := 0; i < blockSize; i++ {
		lmp.SisHashToHash[i] = make([]field.Element, a.SisHashTotalChunksUnpadded)
		lmp.MerkleLeaves[i] = make([]field.Element, 0, a.LeavesSizeUnpadded)
		lmp.MerkleRoots[i] = make([]field.Element, 0, a.LeavesSizeUnpadded)
		lmp.MerkleSisLeaves[i] = make([]field.Element, 0, a.SisLeavesSizeUnpadded)
		lmp.MerkleSisRoots[i] = make([]field.Element, 0, a.SisLeavesSizeUnpadded)
		lmp.MerkleNonSisLeaves[i] = make([]field.Element, 0, a.NonSisRoundLeavesSizeUnpadded)
		lmp.MerkleNonSisRoots[i] = make([]field.Element, 0, a.NonSisRoundLeavesSizeUnpadded)
	}
	lmp.SisLeaves = make([][blockSize][]field.Element, 0, a.NumRoundSis)
	lmp.NonSisLeaves = make([][blockSize][]field.Element, 0, a.NumRoundNonSis)
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
	for i := 0; i < blockSize; i++ {
		lmp.MerkleLeaves[i] = append(lmp.MerkleNonSisLeaves[i], lmp.MerkleSisLeaves[i]...)
		lmp.MerkleRoots[i] = append(lmp.MerkleNonSisRoots[i], lmp.MerkleSisRoots[i]...)
	}
	lmp.MerklePositions = append(lmp.MerkleNonSisPositions, lmp.MerkleSisPositions...)

	// Assign columns using IDs from ctx.Columns
	run.AssignColumn(a.Ctx.Columns.MerkleProofPositions.GetColID(), smartvectors.RightZeroPadded(lmp.MerklePositions, a.LeavesSize))

	for i := 0; i < blockSize; i++ {
		run.AssignColumn(a.Ctx.Columns.MerkleProofsLeaves[i].GetColID(), smartvectors.RightZeroPadded(lmp.MerkleLeaves[i], a.LeavesSize))
		run.AssignColumn(a.Ctx.Columns.MerkleRoots[i].GetColID(), smartvectors.RightZeroPadded(lmp.MerkleRoots[i], a.LeavesSize))
		// The below assignments are only done if SIS is applied to any of the rounds
		if a.Ctx.VortexCtx.NumCommittedRoundsSis() > 0 || a.Ctx.VortexCtx.IsSISAppliedToPrecomputed() {
			// Assign the concatenated SIS hashes
			run.AssignColumn(a.Ctx.Columns.SisHashToHash[i].GetColID(), smartvectors.RightZeroPadded(lmp.SisHashToHash[i], a.SisHashTotalChunks))
			for j := 0; j < a.NumRoundSis; j++ {
				// Assign the SIS round leaves
				run.AssignColumn(a.Ctx.Columns.SisRoundLeaves[i][j].GetColID(), smartvectors.NewRegular(lmp.SisLeaves[j][i]))
			}
		}
	}
	if a.Ctx.VortexCtx.NumCommittedRoundsSis() > 0 || a.Ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		run.AssignColumn(a.Ctx.Columns.ConcatenatedSisHashQ.GetColID(), smartvectors.RightZeroPadded(lmp.ConcatSisHashQ, a.ConcatSisHashQSize))

	}

	// Assign the hash values and preimages for the non SIS rounds
	for i := 0; i < a.NumRoundNonSis; i++ {
		var th [blockSize][]field.Element
		colSize := len(lmp.NonSisHashPreimages[i]) / a.NumOpenedCol
		colChunks := (colSize + blockSize - 1) / blockSize
		for j := 0; j < blockSize; j++ {
			th[j] = make([]field.Element, colChunks*a.NumOpenedCol)
		}
		for k := 0; k < a.NumOpenedCol; k++ {
			srcStart := k * colSize
			nonsisHash := lmp.NonSisHashPreimages[i][srcStart : srcStart+colSize]
			th = poseidon2W.PrepareToHashWitness(th, nonsisHash, k*colChunks)
		}

		for j := 0; j < blockSize; j++ {
			run.AssignColumn(a.Ctx.NonSisMetaData.NonSisLeaves[j][i].GetColID(), smartvectors.RightZeroPadded(lmp.NonSisLeaves[i][j], a.NonSisLeavesColumnSize))
			run.AssignColumn(a.Ctx.NonSisMetaData.NonHashPreimages[i][j].GetColID(), smartvectors.RightZeroPadded(th[j], a.NonSisPreimageColumnsSize[i]))

		}
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
	var rootPrecomp [blockSize]field.Element
	for i := 0; i < blockSize; i++ {
		precompRootSv := run.GetColumn(a.Ctx.Columns.PrecompRoot[i].GetColID())
		rootPrecomp[i] = precompRootSv.IntoRegVecSaveAlloc()[0]
	}
	if a.Ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		precompColSisHash := a.Ctx.VortexCtx.Items.Precomputeds.DhWithMerkle // ColHash of SIS should be hashed by poseidon2 to get leaf
		var precompSisLeaves [blockSize][]field.Element
		for j := 0; j < blockSize; j++ {
			precompSisLeaves[j] = make([]field.Element, 0, len(openingIndices))
		}
		for i, selectedCol := range openingIndices {
			srcStart := selectedCol * lmp.SisHashSize
			if srcStart+lmp.SisHashSize > len(precompColSisHash) {
				utils.Panic("processPrecomputedRound SIS: DhWithMerkle bounds exceeded: need [%d:%d] but len=%d (selectedCol=%d, SisHashSize=%d)",
					srcStart, srcStart+lmp.SisHashSize, len(precompColSisHash), selectedCol, lmp.SisHashSize)
			}
			sisHash := precompColSisHash[srcStart : srcStart+lmp.SisHashSize]
			destStart := i * lmp.SisHashSize
			// Allocate sisHash to SisHash columns

			chunks := (lmp.SisHashSize + blockSize - 1) / blockSize
			preimageDestStart := i * chunks
			// SisHash is the hash of the SIS preimages and also the preimages to poseidon2 hash,
			// We store them in two ways:
			// - ConcatSisHashQ, concatenated as input of the linear combination verification
			// - SisHashToHash, in block of 8 as input of the poseidon2 hash, for merkle proof verification
			lmp.SisHashToHash = poseidon2W.PrepareToHashWitness(lmp.SisHashToHash, sisHash, preimageDestStart)
			copy(lmp.ConcatSisHashQ[destStart:destStart+lmp.SisHashSize], sisHash)

			hasher := poseidon2_koalabear.NewMDHasher()
			hasher.WriteElements(sisHash...)
			leaf := hasher.SumElement()
			for j := 0; j < blockSize; j++ {
				lmp.MerkleSisLeaves[j] = append(lmp.MerkleSisLeaves[j], leaf[j])
				precompSisLeaves[j] = append(precompSisLeaves[j], leaf[j])
				lmp.MerkleSisRoots[j] = append(lmp.MerkleSisRoots[j], rootPrecomp[j])
			}
			lmp.MerkleSisPositions = append(lmp.MerkleSisPositions, field.NewElement(uint64(selectedCol)))

		}
		// make the size of the precompSisLeaves a power of two per block
		for j := 0; j < blockSize; j++ {
			precompSisLeaves[j] = rightPadWithZero(precompSisLeaves[j])
		}
		lmp.SisLeaves = append(lmp.SisLeaves, precompSisLeaves)
		lmp.CommittedRound++
		lmp.TotalNumRounds++
	} else {
		numhash := lmp.NumOpenedCol
		precompColNonSisLeaves := a.Ctx.VortexCtx.Items.Precomputeds.DhWithMerkle // ColHash of NonSIS directly is a leaf
		var precompNonSisLeaves [blockSize][]field.Element
		for j := 0; j < blockSize; j++ {
			precompNonSisLeaves[j] = make([]field.Element, 0, numhash)
		}
		precompNonSisPreimage := make([]field.Element, 0, lmp.NumOpenedCol*len(a.Ctx.VortexCtx.Items.Precomputeds.PrecomputedColums))
		for _, selectedCol := range openingIndices {
			srcStart := selectedCol * blockSize
			if srcStart+blockSize > len(precompColNonSisLeaves) {
				utils.Panic("processPrecomputedRound non-SIS: DhWithMerkle bounds exceeded: need [%d:%d] but len=%d (selectedCol=%d, blockSize=%d)",
					srcStart, srcStart+blockSize, len(precompColNonSisLeaves), selectedCol, blockSize)
			}
			// Poseidon2 hash is [8]field.Element
			var leaf [blockSize]field.Element
			nonSisLeaf := precompColNonSisLeaves[srcStart : srcStart+blockSize]
			copy(leaf[:], nonSisLeaf)

			nonSisPreimage := a.Ctx.VortexCtx.GetPrecomputedSelectedCol(selectedCol)
			// Also compute the leaf from the column
			// to check sanity
			hasher := poseidon2_koalabear.NewMDHasher()
			hasher.WriteElements(nonSisPreimage...)
			leaf_ := hasher.SumElement()
			// Sanity check
			// The leaf computed from the precomputed column
			if leaf != leaf_ {
				utils.Panic("Poseidon2 hash of the precomputed column %v does not match the leaf %v", leaf_, leaf)
			}
			for j := 0; j < blockSize; j++ {

				lmp.MerkleNonSisLeaves[j] = append(lmp.MerkleNonSisLeaves[j], leaf[j])
				lmp.MerkleNonSisRoots[j] = append(lmp.MerkleNonSisRoots[j], rootPrecomp[j])
				precompNonSisLeaves[j] = append(precompNonSisLeaves[j], leaf[j])

			}
			lmp.MerkleNonSisPositions = append(lmp.MerkleNonSisPositions, field.NewElement(uint64(selectedCol)))
			precompNonSisPreimage = append(precompNonSisPreimage, nonSisPreimage...)
		}
		// Append the hash values and preimages
		lmp.NonSisLeaves = append(lmp.NonSisLeaves, precompNonSisLeaves)
		lmp.NonSisHashPreimages = append(lmp.NonSisHashPreimages, precompNonSisPreimage)

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
	if a.NumRoundNonSis > 0 {
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
			colSisHash := colSisHashSV.([]field.Element)

			var rooth field.Octuplet
			for j := 0; j < blockSize; j++ {
				precompRootSv := run.GetColumn(a.Ctx.Columns.Rooth[round][j].GetColID())
				rooth[j] = precompRootSv.IntoRegVecSaveAlloc()[0]
			}

			var sisRoundLeaves [blockSize][]field.Element
			for j := 0; j < blockSize; j++ {
				sisRoundLeaves[j] = make([]field.Element, 0, lmp.NumOpenedCol)
			}

			for i, selectedCol := range openingIndices {
				srcStart := selectedCol * lmp.SisHashSize
				if srcStart+lmp.SisHashSize > len(colSisHash) {
					utils.Panic("processRound SIS: colSisHash bounds exceeded: need [%d:%d] but len=%d (selectedCol=%d, SisHashSize=%d, round=%d)",
						srcStart, srcStart+lmp.SisHashSize, len(colSisHash), selectedCol, lmp.SisHashSize, round)
				}
				sisHash := colSisHash[srcStart : srcStart+lmp.SisHashSize]
				destStart := sisRoundCount*lmp.NumOpenedCol*lmp.SisHashSize + i*lmp.SisHashSize
				copy(lmp.ConcatSisHashQ[destStart:destStart+lmp.SisHashSize], sisHash)

				chunks := (lmp.SisHashSize + blockSize - 1) / blockSize
				tohashDestStart := sisRoundCount*lmp.NumOpenedCol*chunks + i*chunks
				lmp.SisHashToHash = poseidon2W.PrepareToHashWitness(lmp.SisHashToHash, sisHash, tohashDestStart)

				hasher := poseidon2_koalabear.NewMDHasher()
				hasher.WriteElements(sisHash...)
				leaf := hasher.SumElement()

				for j := 0; j < blockSize; j++ {
					lmp.MerkleSisLeaves[j] = append(lmp.MerkleSisLeaves[j], leaf[j])
					sisRoundLeaves[j] = append(sisRoundLeaves[j], leaf[j])
					lmp.MerkleSisRoots[j] = append(lmp.MerkleSisRoots[j], rooth[j])
				}
				lmp.MerkleSisPositions = append(lmp.MerkleSisPositions, field.NewElement(uint64(selectedCol)))
			}
			// Make the size of the sisRoundLeaves a power of two per block
			for j := 0; j < blockSize; j++ {
				sisRoundLeaves[j] = rightPadWithZero(sisRoundLeaves[j])
			}
			// Append the sis leaves
			lmp.SisLeaves = append(lmp.SisLeaves, sisRoundLeaves)
			sisRoundCount++
			run.State.TryDel(colSisHashName)
			lmp.CommittedRound++

		} else if a.Ctx.VortexCtx.RoundStatus[round] == vortex.IsNoSis {
			// Fetch the Poseidon2 hash values
			colNoSisHashName := a.Ctx.VortexCtx.NoSisHashName(round)
			colNonSisLeavesSV, found := run.State.TryGet(colNoSisHashName)
			if !found {
				utils.Panic("colNoSisHashName %v not found", colNoSisHashName)
			}
			colNonSisLeaves := colNonSisLeavesSV.([]field.Element)

			// Fetch the root for the round
			var rooth field.Octuplet
			for j := 0; j < blockSize; j++ {
				precompRootSv := run.GetColumn(a.Ctx.Columns.Rooth[round][j].GetColID())
				rooth[j] = precompRootSv.IntoRegVecSaveAlloc()[0]
			}

			var nonSisRoundLeaves [blockSize][]field.Element
			for j := 0; j < blockSize; j++ {
				nonSisRoundLeaves[j] = make([]field.Element, 0, lmp.NumOpenedCol)
			}

			colSize := a.Ctx.VortexCtx.GetNumPolsForNonSisRounds(round)
			poseidon2HashPreimages := make([]field.Element, 0, lmp.NumOpenedCol*colSize)
			for i, selectedCol := range openingIndices {
				destStart := selectedCol * blockSize
				if destStart+blockSize > len(colNonSisLeaves) {
					utils.Panic("processRound non-SIS: colNonSisLeaves bounds exceeded: need [%d:%d] but len=%d (selectedCol=%d, blockSize=%d, round=%d)",
						destStart, destStart+blockSize, len(colNonSisLeaves), selectedCol, blockSize, round)
				}
				// Poseidon2 hash is [8]field.Element
				var leaf [blockSize]field.Element
				nonSisLeaf := colNonSisLeaves[destStart : destStart+blockSize]
				copy(leaf[:], nonSisLeaf)
				poseidon2Preimage := nonSisOpenedCols[nonSisRoundCount][i]
				// Also compute the leaf from the column
				// to check sanity

				hasher := poseidon2_koalabear.NewMDHasher()
				hasher.WriteElements(poseidon2Preimage...)
				leaf_ := hasher.SumElement()

				if leaf != leaf_ {
					utils.Panic("Poseidon2 hash of the non SIS column %v does not match the leaf %v", leaf_, leaf)
				}
				for j := 0; j < blockSize; j++ {
					lmp.MerkleNonSisLeaves[j] = append(lmp.MerkleNonSisLeaves[j], leaf[j])
					nonSisRoundLeaves[j] = append(nonSisRoundLeaves[j], leaf[j])
					lmp.MerkleNonSisRoots[j] = append(lmp.MerkleNonSisRoots[j], rooth[j])
				}
				lmp.MerkleNonSisPositions = append(lmp.MerkleNonSisPositions, field.NewElement(uint64(selectedCol)))
				poseidon2HashPreimages = append(poseidon2HashPreimages, poseidon2Preimage...)
			}
			// Append the hash values and preimages
			lmp.NonSisLeaves = append(lmp.NonSisLeaves, nonSisRoundLeaves)
			lmp.NonSisHashPreimages = append(lmp.NonSisHashPreimages, poseidon2HashPreimages)
			run.State.TryDel(colNoSisHashName)
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
