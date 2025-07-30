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
	HashHi, HashLo          *common.VectorBuilder
}

func newSha2BlockHashingAssignment(sbh *sha2BlockModule) sha2BlockHashingAssignment {
	return sha2BlockHashingAssignment{
		IsActive:                common.NewVectorBuilder(sbh.IsActive),
		IsEffBlock:              common.NewVectorBuilder(sbh.IsEffBlock),
		IsEffFirstLaneOfNewHash: common.NewVectorBuilder(sbh.IsEffFirstLaneOfNewHash),
		IsEffLastLaneOfCurrHash: common.NewVectorBuilder(sbh.IsEffLastLaneOfCurrHash),
		Limbs:                   common.NewVectorBuilder(sbh.Limbs),
		HashHi:                  common.NewVectorBuilder(sbh.HashHi),
		HashLo:                  common.NewVectorBuilder(sbh.HashLo),
	}
}

// Run implements the [wizard.ProverAction] interface.
func (sbh *sha2BlockModule) Run(run *wizard.ProverRuntime) {

	var (
		assi                 = newSha2BlockHashingAssignment(sbh)
		isFirstLaneOfNewHash = sbh.Inputs.IsFirstLaneOfNewHash.GetColAssignment(run).IntoRegVecSaveAlloc()
		packedUint32         = sbh.Inputs.PackedUint32.GetColAssignment(run).IntoRegVecSaveAlloc()
		selector             = sbh.Inputs.Selector.GetColAssignment(run).IntoRegVecSaveAlloc()
		numRowInp            = len(isFirstLaneOfNewHash)
		cursorInp            = 0
	)

	// scanCurrHash starts from the cursor and increments it until it finds
	// a row where "isFirstNewHash" is 1 or reaches the end of the input module.
	scanCurrHash := func() []field.Element {

		var (
			blocks  []field.Element
			isFirst = true
		)

		for ; cursorInp < numRowInp; cursorInp++ {

			// If we cross a new hash, it hits a stopping condition. We don't
			// include in the loop boundary as it features a sanity-check.
			if !isFirst && isFirstLaneOfNewHash[cursorInp].IsOne() {

				if selector[cursorInp].IsZero() {
					utils.Panic("unexpected: at row %v, the selector is zero but isNewHash is one", cursorInp)
				}

				return blocks
			}

			isFirst = false
			if selector[cursorInp].IsZero() {
				continue
			}

			blocks = append(blocks, packedUint32[cursorInp])
		}

		return blocks
	}

	for cursorInp < numRowInp {

		var (
			currBlock    [16]field.Element
			blocks       = scanCurrHash()
			currState    = initializationVector
			isFirstBlock = true
		)

		if len(blocks)%16 != 0 {
			panic("unappropriate number of lanes in the current stream. Has it been padded?")
		}

		for len(blocks) > 0 {

			copy(currBlock[:], blocks)
			blocks = blocks[16:]
			currState = assi.pushBlock(currState, currBlock, isFirstBlock, len(blocks) == 0)
			isFirstBlock = false
		}

		assi.catchUpHashHiLo(currState)
	}

	assi.padAndAssign(run)

	sbh.IsEffFirstLaneOfNewHashShiftMin2.Assign(run)
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
	oldState [2]field.Element,
	block [16]field.Element,
	isFirstBlockOfHash bool,
	isLastBlockOfHash bool,
) (newState [2]field.Element) {

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
		sbha.IsEffLastLaneOfCurrHash.PushBoolean(isLastBlockOfHash && i == 1)
		sbha.Limbs.PushField(newState[i])
	}

	return newState
}

// catchUpHashHiLo pushes over the HashHi and HashLo columns so that their
// heights match the one of the rest of the columns
func (sbha *sha2BlockHashingAssignment) catchUpHashHiLo(finalState [2]field.Element) {

	var (
		heightHash   = sbha.HashHi.Height()
		heightRest   = sbha.IsActive.Height()
		numToCatchUp = heightRest - heightHash
	)

	for i := 0; i < numToCatchUp; i++ {
		sbha.HashHi.PushField(finalState[0])
		sbha.HashLo.PushField(finalState[1])
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
	sbha.HashHi.PadAndAssign(run, field.Zero())
	sbha.HashLo.PadAndAssign(run, field.Zero())
}

// sha2Compress runs the compression function and returns the resulting hasher
// state in the form of two field elements.
func sha2Compress(oldState [2]field.Element, block [16]field.Element) (newState [2]field.Element) {

	var (
		oldStateBytes = [32]byte{}
		blockBytes    = [64]byte{}
	)

	for i := range oldState {
		osI := oldState[i].Bytes()
		copy(oldStateBytes[16*i:], osI[16:])
	}

	for i := range block {
		bI := block[i].Bytes()
		copy(blockBytes[4*i:], bI[32-4:])
	}

	newStateBytes := sha2.Compress(oldStateBytes, blockBytes)

	for i := range newState {
		newState[i].SetBytes(newStateBytes[16*i : 16*i+16])
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
