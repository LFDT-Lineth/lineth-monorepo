package sha2

import (
	"sync"

	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/linea-monorepo/prover/crypto/sha2"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// sha2BlockHashingAssignment is a collection of column builder used to construct
// the assignment to a [sha2BlockHashing].
type sha2BlockHashingAssignment struct {
	IsActive                *common.VectorBuilder
	IsEffBlock              *common.VectorBuilder
	IsEffFirstLaneOfNewHash *common.VectorBuilder
	IsEffLastLaneOfCurrHash *common.VectorBuilder
	Limbs                   *common.VectorBuilder
	Hash                    [numLimbsPerState]*common.VectorBuilder
}

func newSha2BlockHashingAssignment(sbh *sha2BlockModule) sha2BlockHashingAssignment {
	res := sha2BlockHashingAssignment{
		IsActive:                common.NewVectorBuilder(sbh.IsActive),
		IsEffBlock:              common.NewVectorBuilder(sbh.IsEffBlock),
		IsEffFirstLaneOfNewHash: common.NewVectorBuilder(sbh.IsEffFirstLaneOfNewHash),
		IsEffLastLaneOfCurrHash: common.NewVectorBuilder(sbh.IsEffLastLaneOfCurrHash),
		Limbs:                   common.NewVectorBuilder(sbh.Limbs),
	}

	for i := range res.Hash {
		res.Hash[i] = common.NewVectorBuilder(sbh.Hash[i])
	}

	return res
}

// Run implements the [wizard.ProverAction] interface.
func (sbh *sha2BlockModule) Run(run *wizard.ProverRuntime) {

	var (
		assi                 = newSha2BlockHashingAssignment(sbh)
		isFirstLaneOfNewHash = sbh.Inputs.IsFirstLaneOfNewHash.GetColAssignment(run).IntoRegVecSaveAlloc()
		packedUint16         = sbh.Inputs.PackedUint16.GetColAssignment(run).IntoRegVecSaveAlloc()
		selector             = sbh.Inputs.Selector.GetColAssignment(run).IntoRegVecSaveAlloc()
		numRowInp            = len(isFirstLaneOfNewHash)
		cursorInp            = 0
	)

	// scanCurrHash starts from the cursor and increments it until it finds
	// a row where "isFirstNewHash" is 1 or reaches the end of the input module.
	scanCurrHash := func() []field.Element {

		var (
			blocks []field.Element
		)

		for ; cursorInp < numRowInp; cursorInp++ {

			if selector[cursorInp].IsZero() {
				continue
			}

			blocks = append(blocks, packedUint16[cursorInp])

			// If we cross a new hash, it hits a stopping condition. We don't
			// include in the loop boundary as it features a sanity-check.
			if isFirstLaneOfNewHash[cursorInp].IsZero() && isFirstLaneOfNewHash[cursorInp+1].IsOne() {

				if selector[cursorInp].IsZero() {
					utils.Panic("unexpected: at row %v, the selector is zero but isNewHash is one", cursorInp)
				}

				return blocks
			}
		}

		return blocks
	}

	for cursorInp < numRowInp {

		var (
			currBlock    [numLimbsPerBlock]field.Element
			blocks       = scanCurrHash()
			currState    = initializationVector
			isFirstBlock = true
		)

		if len(blocks)%numLimbsPerBlock != 0 {
			utils.Panic("unappropriate number of lanes in the current stream %d. Has it been padded?", len(blocks))
		}

		for len(blocks) > 0 {

			copy(currBlock[:], blocks)
			blocks = blocks[numLimbsPerBlock:]
			currState = assi.pushBlock(currState, currBlock, isFirstBlock, len(blocks) == 0)
			isFirstBlock = false
		}

		assi.catchUpHashHiLo(currState)
	}

	assi.padAndAssign(run)

	sbh.IsEffFirstLaneOfNewHashShiftMin16.Assign(run)
	sbh.CanBeBeginningOfInstance.Assign(run)
	sbh.CanBeBlockOfInstance.Assign(run)
	sbh.CanBeEndOfInstance.Assign(run)

	for i := range sbh.ProverActions {
		sbh.ProverActions[i].Run(run)
	}

	if sbh.HasCircuit {
		// this is guarded by a once, so it is safe to call multiple times
		registerGnarkHint()
		sbh.GnarkCircuitConnector.Assign(run)
	}
}

// pushBlock pushes the first block of a hash
func (sbha *sha2BlockHashingAssignment) pushBlock(
	oldState [numLimbsPerState]field.Element,
	block [numLimbsPerBlock]field.Element,
	isFirstBlockOfHash bool,
	isLastBlockOfHash bool,
) (newState [numLimbsPerState]field.Element) {

	newState = sha2Compress(oldState, block)

	for i := range oldState {
		sbha.IsActive.PushOne()
		sbha.IsEffBlock.PushZero()
		sbha.IsEffFirstLaneOfNewHash.PushBoolean(isFirstBlockOfHash && i == 0)
		sbha.IsEffLastLaneOfCurrHash.PushZero()
		sbha.Limbs.PushField(oldState[i])
	}

	for i := range block {
		sbha.IsActive.PushOne()
		sbha.IsEffBlock.PushOne()
		sbha.IsEffFirstLaneOfNewHash.PushZero()
		sbha.IsEffLastLaneOfCurrHash.PushZero()
		sbha.Limbs.PushField(block[i])
	}

	for i := range newState {
		sbha.IsActive.PushOne()
		sbha.IsEffBlock.PushZero()
		sbha.IsEffFirstLaneOfNewHash.PushZero()
		sbha.IsEffLastLaneOfCurrHash.PushBoolean(isLastBlockOfHash && i == numLimbsPerState-1)
		sbha.Limbs.PushField(newState[i])
	}

	return newState
}

// catchUpHashHiLo pushes over the HashHi and HashLo columns so that their
// heights match the one of the rest of the columns
func (sbha *sha2BlockHashingAssignment) catchUpHashHiLo(finalState [numLimbsPerState]field.Element) {

	var (
		heightHash   = sbha.Hash[0].Height()
		heightRest   = sbha.IsActive.Height()
		numToCatchUp = heightRest - heightHash
	)

	for i := 0; i < numToCatchUp; i++ {
		for j := range sbha.Hash {
			sbha.Hash[j].PushField(finalState[j])
		}
	}
}

// padAndAssign concludes the building by effectively assign what has been
// accumulated so far.
func (sbha *sha2BlockHashingAssignment) padAndAssign(run *wizard.ProverRuntime) {
	sbha.IsActive.PadAndAssign(run, field.Zero())
	sbha.IsEffFirstLaneOfNewHash.PadAndAssign(run, field.Zero())
	sbha.IsEffLastLaneOfCurrHash.PadAndAssign(run, field.Zero())
	sbha.IsEffBlock.PadAndAssign(run, field.Zero())
	sbha.Limbs.PadAndAssign(run, field.Zero())

	for i := range sbha.Hash {
		sbha.Hash[i].PadAndAssign(run, field.Zero())
	}
}

// sha2Compress runs the compression function and returns the resulting hasher
// state in the form of two field elements.
func sha2Compress(
	oldState [numLimbsPerState]field.Element,
	block [numLimbsPerBlock]field.Element,
) (newState [numLimbsPerState]field.Element) {

	var (
		oldStateBytes = [stateSizeBytes]byte{}
		blockBytes    = [blockSizeBytes]byte{}
	)

	for i := range oldState {
		osI := oldState[i].Bytes()
		copy(oldStateBytes[numLimbBytes*i:], osI[limbBytesStart:])
	}

	for i := range block {
		bI := block[i].Bytes()
		copy(blockBytes[numLimbBytes*i:], bI[limbBytesStart:])
	}

	newStateBytes := sha2.Compress(oldStateBytes, blockBytes)

	for i := range newState {
		newState[i].SetBytes(newStateBytes[numLimbBytes*i : numLimbBytes*i+numLimbBytes])
	}

	return newState
}

var onceRegisterGnarkHint = sync.Once{}

// registerGnarkHint registers the circuit specific hint needed to assign to
// the circuit
func registerGnarkHint() {
	onceRegisterGnarkHint.Do(func() {
		solver.RegisterHint(decomposeIntoBytesHint)
	})
}
