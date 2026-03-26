package execution_data_collector

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// ExecutionDataCollectorVectors is a helper struct used to instantiate the ExecutionDataCollector's columns
type ExecutionDataCollectorVectors struct {
	BlockID, AbsTxID, AbsTxIDMax []field.Element
	Limbs                        [common.NbLimbU128][]field.Element
	NoBytes                      []field.Element
	TotalNoTxBlock               []field.Element

	IsActive                                                                       []field.Element
	IsNoTx, IsBlockHashHi, IsBlockHashLo, IsTimestamp, IsTxRLP, IsAddrHi, IsAddrLo []field.Element

	Ct []field.Element

	FirstAbsTxIDBlock, LastAbsTxIDBlock []field.Element

	EndOfRlpSegment        []field.Element
	TotalBytesCounter      []field.Element
	FinalTotalBytesCounter field.Element
}

// NewExecutionDataCollectorVectors creates a new ExecutionDataCollectorVectors.
func NewExecutionDataCollectorVectors(size int) *ExecutionDataCollectorVectors {
	res := &ExecutionDataCollectorVectors{
		BlockID:                make([]field.Element, size),
		AbsTxID:                make([]field.Element, size),
		AbsTxIDMax:             make([]field.Element, size),
		NoBytes:                make([]field.Element, size),
		TotalNoTxBlock:         make([]field.Element, size),
		IsActive:               make([]field.Element, size),
		IsNoTx:                 make([]field.Element, size),
		IsBlockHashHi:          make([]field.Element, size),
		IsBlockHashLo:          make([]field.Element, size),
		IsTimestamp:            make([]field.Element, size),
		IsTxRLP:                make([]field.Element, size),
		IsAddrHi:               make([]field.Element, size),
		IsAddrLo:               make([]field.Element, size),
		Ct:                     make([]field.Element, size),
		FirstAbsTxIDBlock:      make([]field.Element, size),
		LastAbsTxIDBlock:       make([]field.Element, size),
		EndOfRlpSegment:        make([]field.Element, size),
		TotalBytesCounter:      make([]field.Element, size),
		FinalTotalBytesCounter: field.Zero(),
	}

	for i := range res.Limbs {
		res.Limbs[i] = make([]field.Element, size)
	}

	return res
}

// SetCounters assigns the counter values
func (vect *ExecutionDataCollectorVectors) SetCounters(totalCt, blockCt, absTxCt, absTxIdMax int) {
	vect.Ct[totalCt].SetInt64(int64(totalCt))
	vect.BlockID[totalCt].SetInt64(int64(blockCt + 1))
	vect.AbsTxID[totalCt].SetInt64(int64(absTxCt))
	vect.AbsTxIDMax[totalCt].SetInt64(int64(absTxIdMax))
	if totalCt > 0 {
		// add the current bytes to the total
		vect.TotalBytesCounter[totalCt].Add(&vect.NoBytes[totalCt], &vect.TotalBytesCounter[totalCt-1])
	} else {
		// or if we are at the beginning, initialize the current bytes as the bytes that are loaded.
		vect.TotalBytesCounter[totalCt].Set(&vect.NoBytes[totalCt])
	}
}

// SetBlockMetadata assigns block metadata values. This function will be called in a manner
// which makes the block metadata remain constant for the entire block segment.
func (vect *ExecutionDataCollectorVectors) SetBlockMetadata(totalCt int, totalTxBlock, firstAbsTxIDBlock, lastAbsTxIDBlock field.Element) {
	vect.TotalNoTxBlock[totalCt].Set(&totalTxBlock)
	vect.FirstAbsTxIDBlock[totalCt].Set(&firstAbsTxIDBlock)
	vect.LastAbsTxIDBlock[totalCt].Set(&lastAbsTxIDBlock)
}
