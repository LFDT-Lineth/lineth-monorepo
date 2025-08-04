package statesummary

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	smCommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/common"
)

var (
	// emptyStorageRoot is the root of the empty tree.
	emptyStorageRoot = field.NewFromString("3433815185717552096229990069654548485149817391158829673291533636212160492517")
)

// AccumulatorStatement represents the statement sent to the accumulator.
type AccumulatorStatement struct {
	// IsReadZero, IsReadNonZero, IsInsert, IsUpdate, IsDelete are binary
	// flags indicating the type of traces being deferred to the accumulator
	// module.
	IsReadZero, IsReadNonZero, IsInsert, IsUpdate, IsDelete ifaces.Column
	// SameTypeAsBefore is an indicator column indicating whether the current
	// row has the same type of shomei operation as the previous one.
	SameTypeAsBefore    ifaces.Column
	CptSameTypeAsBefore wizard.ProverAction

	// StateDiff contains the relevant accumulator values
	smCommon.StateDiff

	// InitialAndFinalHValAreEqual is a constrained column that will contain 1s on the positions where InitialHVal = FinalHVal
	InitialAndFinalHValAreEqual     ifaces.Column
	ComputeInitialAndFinalHValEqual wizard.ProverAction

	// FinalHValIsZero is a constrained column that will contain 1s on the positions where FinalHValIsZero = 0
	FinalHValIsZero        ifaces.Column
	ComputeFinalHValIsZero wizard.ProverAction
}

// newAccumulatorStatement returns a new AccumulatorStatement with initialized
// columns that are not constrained.
func newAccumulatorStatement(comp *wizard.CompiledIOP, size int, name string) AccumulatorStatement {

	createCol := func(subName string) ifaces.Column {
		return comp.InsertCommit(
			0,
			ifaces.ColIDf("STATE_SUMMARY_%v_%v", name, subName),
			size,
		)
	}

	res := AccumulatorStatement{
		IsReadZero:    createCol("IS_READ_ZERO"),
		IsReadNonZero: createCol("IS_READ_NON_ZERO"),
		IsInsert:      createCol("IS_INSERT"),
		IsUpdate:      createCol("IS_UPDATE"),
		IsDelete:      createCol("IS_DELETE"),
		StateDiff:     smCommon.NewStateDiff(comp, size, "STATE_SUMMARY", "ACC_STATEMENT"),
	}

	res.InitialAndFinalHValAreEqual, res.ComputeInitialAndFinalHValEqual = dedicated.IsZero(
		comp,
		sym.Sub(res.StateDiff.InitialHVal, res.StateDiff.FinalHVal),
	).GetColumnAndProverAction()

	res.FinalHValIsZero, res.ComputeFinalHValIsZero = dedicated.IsZero(
		comp,
		sym.Sub(res.StateDiff.FinalHVal, field.Zero()),
	).GetColumnAndProverAction()

	res.SameTypeAsBefore, res.CptSameTypeAsBefore = dedicated.IsZero(
		comp,
		sym.Sub(
			sym.Add(
				res.IsInsert,
				res.IsUpdate,
				res.IsDelete,
			),
			sym.Add(
				column.Shift(res.IsInsert, -1),
				column.Shift(res.IsUpdate, -1),
				column.Shift(res.IsDelete, -1),
			),
		),
	).GetColumnAndProverAction()

	return res
}

// AccumulatorStatementAssignmentBuilder is a convenience structure storing
// the column builders relating to an AccumulatorStatement.
type AccumulatorStatementAssignmentBuilder struct {
	IsReadZero, IsReadNonZero, IsInsert, IsUpdate, IsDelete *common.VectorBuilder
	SummaryBuilder                                          smCommon.StateDiffAssignmentBuilder
}

// newAccumulatorStatementAssignmentBuilder initializes a fresh
// [AccumulatorStatementAssignmentBuilder]
func newAccumulatorStatementAssignmentBuilder(as *AccumulatorStatement) AccumulatorStatementAssignmentBuilder {
	return AccumulatorStatementAssignmentBuilder{
		IsReadZero:     common.NewVectorBuilder(as.IsReadZero),
		IsReadNonZero:  common.NewVectorBuilder(as.IsReadNonZero),
		IsInsert:       common.NewVectorBuilder(as.IsInsert),
		IsUpdate:       common.NewVectorBuilder(as.IsUpdate),
		IsDelete:       common.NewVectorBuilder(as.IsDelete),
		SummaryBuilder: smCommon.NewStateDiffAssignmentBuilder(as.StateDiff),
	}
}

// PushReadZero pushes the relevant row when a ReadZero occurs on the
// accumulator side.
func (as *AccumulatorStatementAssignmentBuilder) PushReadZero(root, hkey types.Bytes32) {
	as.IsReadZero.PushOne()
	as.IsReadNonZero.PushZero()
	as.IsInsert.PushZero()
	as.IsUpdate.PushZero()
	as.IsDelete.PushZero()
	as.SummaryBuilder.PushReadZero(root, hkey)
}

// PushReadNonZero pushes a row onto `as` for a read-non-zero operation.
func (as *AccumulatorStatementAssignmentBuilder) PushReadNonZero(root, hKey, hVal types.Bytes32) {
	as.IsReadZero.PushZero()
	as.IsReadNonZero.PushOne()
	as.IsInsert.PushZero()
	as.IsUpdate.PushZero()
	as.IsDelete.PushZero()
	as.SummaryBuilder.PushReadNonZero(root, hKey, hVal)
}

// PushInsert pushes a row representing an insertion onto `as`.
func (as *AccumulatorStatementAssignmentBuilder) PushInsert(oldRoot, newRoot, hKey, newHVal types.Bytes32) {
	as.IsReadZero.PushZero()
	as.IsReadNonZero.PushZero()
	as.IsInsert.PushOne()
	as.IsUpdate.PushZero()
	as.IsDelete.PushZero()
	as.SummaryBuilder.PushInsert(oldRoot, newRoot, hKey, newHVal)
}

// PushUpdate pushes a row representing an update onto `as`.
func (as *AccumulatorStatementAssignmentBuilder) PushUpdate(oldRoot, newRoot, hKey, oldHVal, newHVal types.Bytes32) {
	as.IsReadZero.PushZero()
	as.IsReadNonZero.PushZero()
	as.IsInsert.PushZero()
	as.IsUpdate.PushOne()
	as.IsDelete.PushZero()
	as.SummaryBuilder.PushUpdate(oldRoot, newRoot, hKey, oldHVal, newHVal)
}

// PushDelete pushes a row representing a deletion onto `as`.
func (as *AccumulatorStatementAssignmentBuilder) PushDelete(oldRoot, newRoot, hKey, oldHVal types.Bytes32) {
	as.IsReadZero.PushZero()
	as.IsReadNonZero.PushZero()
	as.IsInsert.PushZero()
	as.IsUpdate.PushZero()
	as.IsDelete.PushOne()
	as.SummaryBuilder.PushDelete(oldRoot, newRoot, hKey, oldHVal)
}

// PadAndAssign pads all the column in `as` and assign them into `run`
func (as *AccumulatorStatementAssignmentBuilder) PadAndAssign(run *wizard.ProverRuntime) {
	as.IsReadZero.PadAndAssign(run)
	as.IsReadNonZero.PadAndAssign(run)
	as.IsInsert.PadAndAssign(run)
	as.IsUpdate.PadAndAssign(run)
	as.IsDelete.PadAndAssign(run)
	as.SummaryBuilder.PadAndAssign(run)
}

// Type returns a code to identify the type of trace as a symbolic expression
//
