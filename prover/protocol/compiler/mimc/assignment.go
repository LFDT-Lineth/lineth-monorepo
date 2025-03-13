package mimc

import (
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// assign: Assigns the columns to the prover runtime using PaddedCircularWindow
func (ctx *mimcCtx) assign(run *wizard.ProverRuntime) {

	var (
		oldStateSV = ctx.oldStates.GetColAssignment(run)
		blocksSV   = ctx.blocks.GetColAssignment(run)
		totalRows  = oldStateSV.Len()
		numRounds  = len(ctx.intermediateResult)
	)

	offset, windowLen := identifyActiveWindow(oldStateSV, blocksSV, totalRows)
	oldStateWindow, blocksWindow := extractWindowSlices(oldStateSV, blocksSV, offset, windowLen)
	intermediateResWindow, intermediatePow4Window := computeIntermediateValues(numRounds, oldStateWindow, blocksWindow, windowLen)

	var resPad, pow4Pad []field.Element

	// Precompute padding only when `PaddedCircularWindow` is tobe used
	// i.e. Whenever there is sparsity in the (oldState, blocks) pair
	if windowLen != totalRows {
		resPad, pow4Pad = precomputePaddingValues(numRounds)
	}
	assignOptimizedVectors(run, ctx, intermediateResWindow, intermediatePow4Window, resPad, pow4Pad, offset, totalRows)
}

// identifyActiveWindow finds the smallest active window scanning through the oldState and blocks
func identifyActiveWindow(oldStateSV, blocksSV smartvectors.SmartVector, totalRows int) (offset int, windowLen int) {
	// Convert to regular vectors to scan all elements
	var (
		oldState = smartvectors.IntoRegVec(oldStateSV)
		blocks   = smartvectors.IntoRegVec(blocksSV)
	)

	// Initialize firstNonZero and lastNonZero indices to default values
	firstNonZero, lastNonZero := totalRows, -1
	for i := 0; i < totalRows; i++ {
		if !oldState[i].IsZero() || !blocks[i].IsZero() {
			firstNonZero = min(firstNonZero, i)
			lastNonZero = max(lastNonZero, i)
		}
	}

	if firstNonZero <= lastNonZero {
		offset = firstNonZero
		windowLen = lastNonZero - firstNonZero + 1
		return offset, windowLen
	}
	// Default window => Full window
	return 0, totalRows
}

// computeIntermediateValues computes intermediate values for the window
func computeIntermediateValues(numRounds int, oldStateWindow, blocksWindow []field.Element, windowLen int) ([][]field.Element, [][]field.Element) {
	intermediateResWindow := make([][]field.Element, numRounds)
	intermediatePow4Window := make([][]field.Element, numRounds)
	for i := range intermediateResWindow {
		intermediateResWindow[i] = make([]field.Element, windowLen)
		intermediatePow4Window[i] = make([]field.Element, windowLen)
	}

	// Initalize intermediateResWindow to the blocksWindow
	copy(intermediateResWindow[0], blocksWindow)

	// r => round
	for r := 0; r < numRounds; r++ {
		parallel.Execute(windowLen, func(start, stop int) {
			for k := start; k < stop; k++ {
				if r == 0 {
					tmp := intermediateResWindow[0][k]
					tmp.Add(&tmp, &mimc.Constants[0]).Add(&tmp, &oldStateWindow[k])
					intermediatePow4Window[0][k].Square(&tmp).Square(&intermediatePow4Window[0][k])
				} else {
					// For subsequent rounds, compute intermediate values based on previous results
					ark := mimc.Constants[r-1]
					nextArk := mimc.Constants[r]

					tmp := intermediatePow4Window[r-1][k]
					tmp.Square(&tmp).Square(&tmp)

					// Compute intermediate result using previous result and oldState
					intermediateResWindow[r][k] = intermediateResWindow[r-1][k]
					intermediateResWindow[r][k].Add(&intermediateResWindow[r][k], &ark).Add(&intermediateResWindow[r][k], &oldStateWindow[k])
					intermediateResWindow[r][k].Mul(&intermediateResWindow[r][k], &tmp)

					// Compute intermediatePow4
					tmp = intermediateResWindow[r][k]
					tmp.Add(&tmp, &nextArk).Add(&tmp, &oldStateWindow[k])
					intermediatePow4Window[r][k].Square(&tmp).Square(&intermediatePow4Window[r][k])
				}
			}
		})
	}
	return intermediateResWindow, intermediatePow4Window
}

// assignOptimizedVectors assigns optimized vectors to the prover runtime
func assignOptimizedVectors(run *wizard.ProverRuntime, ctx *mimcCtx, intermediateResWindow, intermediatePow4Window [][]field.Element, resPad, pow4Pad []field.Element, offset, totalRows int) {
	for round := range ctx.intermediateResult {
		windowLen := len(intermediateResWindow[round])

		// Full-length window: use Regular vector
		isRegSmartVec := windowLen == totalRows

		// Helper function to assign a column with the appropriate smart vector
		assignColumn := func(colID ifaces.ColID, window []field.Element, padVal field.Element) {
			if isRegSmartVec {
				fullVec := make([]field.Element, totalRows)
				copy(fullVec[offset:offset+windowLen], window)
				run.AssignColumn(colID, smartvectors.NewRegular(fullVec))
			} else {
				// Partial window: use PaddedCircularWindow with lazily evaluated padding
				run.AssignColumn(colID, smartvectors.NewPaddedCircularWindow(window, padVal, offset, totalRows))
			}
		}

		// Determine padding values
		var resPadVal, pow4PadVal field.Element
		if resPad != nil && len(resPad) > round {
			resPadVal = resPad[round]
		}
		if pow4Pad != nil && len(pow4Pad) > round {
			pow4PadVal = pow4Pad[round]
		}

		// Assign intermediateResult (skip round=0 as it is initialized to the blocks)
		if round > 0 {
			assignColumn(ctx.intermediateResult[round].GetColID(), intermediateResWindow[round], resPadVal)
		}

		// Assign intermediatePow4
		assignColumn(ctx.intermediatePow4[round].GetColID(), intermediatePow4Window[round], pow4PadVal)
	}
}

// precomputePaddingValues precomputes padding values for constant regions
func precomputePaddingValues(numRounds int) ([]field.Element, []field.Element) {
	resPad := make([]field.Element, numRounds)
	pow4Pad := make([]field.Element, numRounds)
	resPad[0].SetZero()

	var tmp field.Element
	tmp.Add(&resPad[0], &mimc.Constants[0])
	pow4Pad[0].Square(&tmp).Square(&pow4Pad[0])

	for r := 1; r < numRounds; r++ {
		tmp.Square(&pow4Pad[r-1]).Square(&tmp)
		resPad[r].Add(&resPad[r-1], &mimc.Constants[r-1])
		resPad[r].Mul(&resPad[r], &tmp)
		tmp.Add(&resPad[r], &mimc.Constants[r])
		pow4Pad[r].Square(&tmp).Square(&pow4Pad[r])
	}

	return resPad, pow4Pad
}

// extractWindowSlices extracts window slices from the smart vectors
func extractWindowSlices(oldStateSV, blocksSV smartvectors.SmartVector, l, h int) ([]field.Element, []field.Element) {
	var (
		oldStateWindow = smartvectors.IntoRegVec(oldStateSV)[l : l+h]
		blocksWindow   = smartvectors.IntoRegVec(blocksSV)[l : l+h]
	)
	return oldStateWindow, blocksWindow
}
