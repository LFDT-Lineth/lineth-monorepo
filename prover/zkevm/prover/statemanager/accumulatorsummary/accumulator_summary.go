package accumulatorsummary

import (
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/accumulator"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

// Inputs collects the inputs of [NewModule]
type Inputs struct {
	Name        string
	Accumulator accumulator.Module
}

// Module represents the statement sent to the accumulator.
type Module struct {
	Inputs Inputs
	common.StateDiff
}

// NewModule returns a new AccumulatorStatement with initialized
// columns that are not constrained.
// Todo: See if the below function needs to return a pointer
func NewModule(comp *wizard.CompiledIOP, inputs Inputs) *Module {
	accSummary := Module{
		StateDiff: common.NewStateDiff(comp, inputs.Accumulator.NumRows(), "ACCUMULATOR_SUMMARY", inputs.Name),
		Inputs:    inputs,
	}

	defineSegmentConstraints(comp, &inputs.Accumulator, accSummary)
	accumulatorDefineHKeyConstraint(comp, &inputs.Accumulator, accSummary)
	accumulatorDefineHValConstraints(comp, &inputs.Accumulator, accSummary)
	accumulatorDefineRootConstraints(comp, &inputs.Accumulator, accSummary)

	return &accSummary
}

// ConnectToStateSummary enriches the current AccumulatorSummary with
// lookup constraints tying it to the
func (as *Module) ConnectToStateSummary(comp *wizard.CompiledIOP, ss *statesummary.Module) *Module {

	var accSummaryTable []ifaces.Column
	accSummaryTable = append(accSummaryTable, as.InitialRoot[:]...)
	accSummaryTable = append(accSummaryTable, as.FinalRoot[:]...)
	accSummaryTable = append(accSummaryTable, as.HKey[:]...)
	accSummaryTable = append(accSummaryTable, as.InitialHVal[:]...)
	accSummaryTable = append(accSummaryTable, as.FinalHVal[:]...)
	accSummaryTable = append(
		accSummaryTable,
		as.Inputs.Accumulator.Cols.IsReadNonZero,
		as.Inputs.Accumulator.Cols.IsReadZero,
		as.Inputs.Accumulator.Cols.IsInsert,
		as.Inputs.Accumulator.Cols.IsUpdate,
		as.Inputs.Accumulator.Cols.IsDelete,
	)

	var stateSummaryTable []ifaces.Column
	stateSummaryTable = append(stateSummaryTable, ss.AccumulatorStatement.StateDiff.InitialRoot[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.AccumulatorStatement.StateDiff.FinalRoot[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.AccumulatorStatement.StateDiff.HKey[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.AccumulatorStatement.StateDiff.InitialHVal[:]...)
	stateSummaryTable = append(stateSummaryTable, ss.AccumulatorStatement.StateDiff.FinalHVal[:]...)
	stateSummaryTable = append(
		stateSummaryTable,
		ss.AccumulatorStatement.IsReadNonZero,
		ss.AccumulatorStatement.IsReadZero,
		ss.AccumulatorStatement.IsInsert,
		ss.AccumulatorStatement.IsUpdate,
		ss.AccumulatorStatement.IsDelete,
	)

	comp.InsertInclusion(0, "LOOKUP_STATE_MGR_ACCUMULATOR_SUMMARY_TO_STATE_SUMMARY", stateSummaryTable, accSummaryTable)
	// Perform the reverse check as well to make sure that some traces are not excluded
	comp.InsertInclusion(0, "LOOKUP_STATE_MGR_ACCUMULATOR_SUMMARY_TO_STATE_SUMMARY_REVERSED", accSummaryTable, stateSummaryTable)

	return as
}
