package dedicated

import (
	"strconv"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
)

// AccumulatorInputs stores the inputs for  [AccumulateUpToMax] function.
type DoublyMaxAccumulatorInputs struct {
	// ColA is subject to the acumulation.
	ColA ifaces.Column
	// accumulating should not goes beyond the given maxValue.
	MaxValue int
	MaxAtRow int
}

/*
AccumulateUpToMaxCtx stores the intermediate columns for [AccumulateUpToMax] function.
AccumulateUpToMax accumulates the values of column ColA (upward) up to the given max,

after reaching the max, it restarts the accumulation.

To prevent the accumulation going beyond the maximum,
we force the accumulator to reach the max at the first row.

The column IsMax indicate where the accumulator reaches the max,

	it is a fully-constrained binary column.
*/
type AccumulateUpToDoublyMaxCtx struct {
	Inputs DoublyMaxAccumulatorInputs
	// It is 1 when the accumulator reaches the max value.
	IsMaxVal   ifaces.Column
	IsMaxAtRow ifaces.Column
	IsActive   ifaces.Column
	// the  ProverAction for IsZero()
	PA      wizard.ProverAction
	PaAtRow wizard.ProverAction
	// It accumulate the elements from ColA.
	Accumulator      ifaces.Column
	AccumulatorAtRow ifaces.Column
	// size of the accumulator
	Size  int
	IsMax ifaces.Column // sum of IsMaxVal and IsMaxAtRow
}

func AccumulateUpToDoublyMax(comp *wizard.CompiledIOP, maxValue, MaxAtRow int, colA, isActive ifaces.Column) *AccumulateUpToDoublyMaxCtx {

	if maxValue%MaxAtRow == 0 {
		utils.Panic("maxValue should not be multiple of MaxAtRow, use AccumulateUpToMax instead")
	}
	var (
		uniqueID  = strconv.Itoa(len(comp.ListCommitments()))
		size      = colA.Size()
		createCol = common.CreateColFn(comp, "ACCUMULATE_UP_TO_MAX_"+uniqueID, size, pragmas.RightPadded)
	)

	acc := &AccumulateUpToDoublyMaxCtx{
		Inputs: DoublyMaxAccumulatorInputs{
			MaxValue: maxValue,
			MaxAtRow: MaxAtRow,
			ColA:     colA},
		Accumulator:      createCol("Accumulator"),
		AccumulatorAtRow: createCol("AccumulatorAtRow"),
		IsMax:            createCol("IsMax"),
		IsActive:         isActive,
		Size:             size,
	}

	acc.IsMaxVal, acc.PA = dedicated.IsZero(comp, sym.Sub(maxValue, acc.Accumulator)).GetColumnAndProverAction()

	// Constraints over the accumulator
	// Accumulator[first] =ColA[first]
	comp.InsertLocal(0, ifaces.QueryID("AccCLDLenSpaghetti_Loc_"+uniqueID),
		sym.Sub(
			acc.Accumulator, acc.Inputs.ColA,
		),
	)

	// Accumulator[i] = Accumulator[i-1]*(1-acc.IsMax[i-1]) +ColA[i]; i standing for row-index.
	res := sym.Sub(1, column.Shift(acc.IsMaxVal, -1)) // 1-acc.IsMax[i-1]

	comp.InsertGlobal(0, ifaces.QueryID("AccCLDLenSpaghetti_Glob_"+uniqueID),
		sym.Sub(
			sym.Add(
				sym.Mul(
					column.Shift(acc.Accumulator, -1), res),
				acc.Inputs.ColA),
			acc.Accumulator,
		),
	)

	// IsMax[last-active-row] = 1
	comp.InsertGlobal(0, ifaces.QueryID("IS_1_AT_LAST_ACTIVE_"+uniqueID),
		sym.Mul(
			isActive,
			sym.Sub(1, column.Shift(isActive, 1)),
			sym.Sub(1, acc.IsMaxVal),
		))

	// the case that isActive is full 1,
	comp.InsertLocal(0, ifaces.QueryID("IS_1_AT_FULL_ACTIVE_"+uniqueID),
		sym.Sub(column.Shift(acc.IsMaxVal, -1),
			column.Shift(isActive, -1)),
	)

	// AtRow is upward accumulation up to MaxAtRow (due to big-endian)
	acc.IsMaxAtRow, acc.PaAtRow = dedicated.IsZero(comp, sym.Sub(MaxAtRow, acc.AccumulatorAtRow)).GetColumnAndProverAction()

	// Constraints over the accumulator
	// AccumulatorAtRow[last] =ColA[last]
	comp.InsertLocal(0, ifaces.QueryID("AccRowCLDLenSpaghetti_Loc_"+uniqueID),
		sym.Sub(
			column.Shift(acc.AccumulatorAtRow, -1), column.Shift(acc.Inputs.ColA, -1),
		),
	)

	// AccumulatorAtRow[i] = AccumulatorAtRow[i+1]*(1-acc.IsMaxAtRow[i+1])*(1-acc.IsMax[i+1)]) +ColA[i]; i standing for row-index.
	comp.InsertGlobal(0, ifaces.QueryID("AccRowCLDLenSpaghetti_Glob_"+uniqueID),
		sym.Sub(
			acc.AccumulatorAtRow,
			sym.Add(
				sym.Mul(
					column.Shift(acc.AccumulatorAtRow, 1),
					sym.Sub(1, column.Shift(acc.IsMaxAtRow, 1)),
					sym.Sub(1, column.Shift(acc.IsMaxVal, 1)),
				),
				acc.Inputs.ColA,
			),
		),
	)

	// IsMaxAtRow[0] = 1
	comp.InsertLocal(0, ifaces.QueryID("MaxAtRow_IS_1_AT_POS_0_"+uniqueID),
		sym.Sub(acc.IsMaxAtRow, acc.IsActive),
	)

	// isMax = isMaxVal + isMaxAtRow
	comp.InsertGlobal(0, ifaces.QueryID("IsMax_Sum_"+uniqueID),
		sym.Sub(
			acc.IsMax,
			sym.Add(
				acc.IsMaxVal,
				acc.IsMaxAtRow,
			),
		),
	)
	// isMax is binary
	commonconstraints.MustBeBinary(comp, acc.IsMax)
	return acc

}

func (la *AccumulateUpToDoublyMaxCtx) Run(run *wizard.ProverRuntime) {

	var (
		column   = la.Inputs.ColA.GetColAssignment(run).IntoRegVecSaveAlloc()
		maxVal   = la.Inputs.MaxValue
		MaxAtRow = la.Inputs.MaxAtRow
		acc      = make([]field.Element, len(column))
		accAtRow = make([]field.Element, len(column))
	)

	// populate Accumulator
	s := uint64(0)
	for j := range acc {
		s = s + column[j].Uint64()
		if s > uint64(maxVal) {
			utils.Panic("Should not reach a value larger than target value at position %v, target-value=%v s=%v:", j, maxVal, s)
		}
		if s == uint64(maxVal) {
			acc[j] = field.NewElement(s)
			s = 0
		} else {
			acc[j] = field.NewElement(s)
		}

	}

	run.AssignColumn(la.Accumulator.GetColID(), smartvectors.RightZeroPadded(acc, la.Size))
	// assign IsMax
	la.PA.Run(run)
	isMaxVal := la.IsMaxVal.GetColAssignment(run).IntoRegVecSaveAlloc()

	// papulate AccumulatorAtRow
	s = uint64(0)
	for j := len(column) - 1; j >= 0; j-- {

		if j != len(column)-1 {
			s = s*(1-isMaxVal[j+1].Uint64()) + column[j].Uint64()
		} else {
			s = s + column[j].Uint64()
		}

		// sanity check
		if s > uint64(MaxAtRow) {
			utils.Panic("Should not reach a value larger than target value at position %v, target-value=%v s=%v:", j, MaxAtRow, s)
		}
		accAtRow[j] = field.NewElement(s)
		if s == uint64(MaxAtRow) {
			s = 0
		}

	}

	run.AssignColumn(la.AccumulatorAtRow.GetColID(), smartvectors.RightZeroPadded(accAtRow, la.Size))
	// assign IsMaxAtRow
	la.PaAtRow.Run(run)
	isMaxRow := la.IsMaxAtRow.GetColAssignment(run).IntoRegVecSaveAlloc()
	// assign IsMax
	isMax := make([]field.Element, len(column))
	vector.Add(isMax, isMaxVal, isMaxRow)
	run.AssignColumn(la.IsMax.GetColID(), smartvectors.RightZeroPadded(isMax, la.Size))

}
